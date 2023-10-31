package bootz

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	bootzServer "github.com/openconfig/bootz/server"
	entityAPI "github.com/openconfig/bootz/server/entitymanager"
	"github.com/openconfig/bootz/server/service"
	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	frpb "github.com/openconfig/gnoi/factory_reset"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"google.golang.org/protobuf/encoding/prototext"
)



var (
	backupFileName    = "cisco_bkp.cfg"
	controlcardType   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	activeController  = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController = oc.Platform_ComponentRedundantRole_SECONDARY
)

var (
	invalidISO     = flag.String("invalid_iso", "/var/www/html/invalid.iso", "Provide the path for invalid ISO eg: /var/www/html/invalid.iso")
	validISO       = flag.String("valid_iso", "/var/www/html/valid.iso", "Provide the path for invalid ISO eg: /var/www/html/valid.iso")
	versionUpgrade = flag.String("version_upgrade", "24.1.1.12I", "Provide the version to be upgraded to")
	dhcpInterface  = flag.String("dhcp_interface", "ens224", "Provide the interface on which dhcp runs on your server eg: ens224")
	dhcpIp         = flag.String("dhcp_ip", "2.2.2.2", "Provide the IP on which dhcp runs on your server eg: 2.2.2.2")
	baseconfig     string
)

var showrunAfter string

type execCommands struct {
	TerminalLength string
	ShowZtpLog     string
	ShowRun        string
	ZtpClean       string
}

var ciscoExecuteCommands execCommands

type ztpStatus struct {
	Success    string
	Terminated string
}

var ciscoZtpStatus ztpStatus

type consoleCommands struct {
	EnterUserName      string
	EnterPassword      string
	EnterPasswordAgain string
}

var ciscoConsoleRegexCommands consoleCommands
var ztpLog string

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func findActiveRPActiveSerialNumber(t *testing.T, dut *ondatra.DUTDevice) (string, string) {
	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)
	var rpStandbyBeforeSwitch, rpActiveBeforeSwitch string
	if *args.NumControllerCards >= 0 && len(controllerCards) != *args.NumControllerCards {
		t.Errorf("Incorrect number of controller cards: got %v, want exactly %v (specified by flag)", len(controllerCards), *args.NumControllerCards)
	}

	if got, want := len(controllerCards), 2; got < want {
		rpStandbyBeforeSwitch, rpActiveBeforeSwitch = "", controllerCards[0]

	} else {
		rpStandbyBeforeSwitch, rpActiveBeforeSwitch = components.FindStandbyRP(t, dut, controllerCards)

	}

	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)
	return rpStandbyBeforeSwitch, rpActiveBeforeSwitch
}
func versionOnBox(t *testing.T, dut *ondatra.DUTDevice) string {
	_, compName := findActiveRPActiveSerialNumber(t, dut)
	state := gnmi.OC().Component(compName).SoftwareVersion()
	versiononbox := gnmi.Get(t, dut, state.State())
	return versiononbox
}
func initiateBootzOnDevice(t *testing.T, dut *ondatra.DUTDevice, console *fptest.ConsoleIO) {
	console.WaitForPrompt()
	console.Execute(ciscoExecuteCommands.ZtpClean)
	time.Sleep(30 * time.Second)
	gnoiClient := dut.RawAPIs().GNOI(t)
	facRe, err := gnoiClient.FactoryReset().Start(context.Background(), &frpb.StartRequest{FactoryOs: false, ZeroFill: false})
	if err != nil {
		t.Fatalf("Failed to initiate Factory Reset on the device, Error : %v ", err)
	}

	t.Logf("facRe.GetResponse: %v\n", facRe.GetResponse())
	switch v := facRe.GetResponse().(type) {
	case *frpb.StartResponse_ResetError:
		actErr := facRe.GetResetError()
		t.Fatalf("Error during Factory Reset %v: %v", actErr.GetOther(), actErr.GetDetail())
	case *frpb.StartResponse_ResetSuccess:
		t.Logf("FACTORY RESET COMMAND SENT SUCCESSFULLY")
	default:
		t.Fatalf("Expected ResetSuccess following Start: got %v", v)

	}
	t.Logf("facRe.Response: %v\n", facRe.Response)
}
func verifyBaseConfig(t *testing.T, show_run string) {
	t.Logf("SHOW RUN FROM BOX : %v", show_run)
	file, err := os.Open("testdata/cisco.cfg")
	if err != nil {
		t.Fatalf("Could not open file: %v", err)
	}
	defer file.Close()
	cisco_cfg, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("Could not read file data: %v", err)
	}
	str1 := string(show_run)
	str2 := string(cisco_cfg)
	startIndex := strings.Index(str1, "hostname")
	if startIndex == -1 {
		t.Logf("Hostname not found in the configuration.")
		return
	}

	// Remove everything before "hostname."
	str1 = str1[startIndex:]

	// Replace "!" and newline characters.
	str1 = strings.ReplaceAll(str1, "!", "")
	str1 = strings.ReplaceAll(str1, "\n", "")

	// Print the modified configuration.
	t.Logf(str1)

	startIndex = strings.Index(str2, "hostname")
	if startIndex == -1 {
		t.Logf("Hostname not found in the configuration.")
		return
	}

	// Remove everything before "hostname."
	str2 = str2[startIndex:]

	// Replace "!" and newline characters.
	str2 = strings.ReplaceAll(str2, "!", "")
	str2 = strings.ReplaceAll(str2, "\n", "")
	str2 = strings.ReplaceAll(str2, "no shutdown", "")

	// Print the modified configuration.
	t.Logf(str2)
	pattern := regexp.MustCompile(`\s+`)
	showRun := pattern.ReplaceAllString(str1, "")
	ciscoBase := pattern.ReplaceAllString(str2, "")

	//check if show run and testdata/cisco.cfg are same
	if showRun != ciscoBase {
		t.Errorf("Base config verification for bootz failed")
	}
}
func verifyVersionChange(t *testing.T, versionBefore, versionAfter string, versionChange bool) {
	if (versionChange) && (versionBefore == versionAfter) {
		t.Error("Image Change expected")
	}
	if (!versionChange) && (versionBefore != versionAfter) {
		t.Error("Image change not expected")
	}

}
func deviceBootStatus(t *testing.T, dut *ondatra.DUTDevice, maxRebootTime uint64) {
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(3 * time.Minute)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Logf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
			break
		}
	}
	t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
}
func usernamepassword(t *testing.T) (string, string) {
	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatalf("unable to read binding file")
	}
	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		t.Fatalf("unable to parse binding file")
	}
	username := b.Duts[0].Options.GetUsername()
	password := b.Duts[0].Options.GetPassword()
	return username, password

}
func baseConfig(t *testing.T, console *fptest.ConsoleIO) {
	t.Logf("executing new line and wait for prompt")
	console.Writeln("")

	userRe := regexp.MustCompile(ciscoConsoleRegexCommands.EnterUserName)
	passwordRe := regexp.MustCompile(ciscoConsoleRegexCommands.EnterPassword)
	confirmRe := regexp.MustCompile(ciscoConsoleRegexCommands.EnterPasswordAgain)
	username, password := usernamepassword(t)
	console.ReadUntilPrompt(func(data []byte) bool {
		if console.FindInputPrompt(userRe, username, data) {
			return false
		}
		if console.FindInputPrompt(passwordRe, password, data) {
			return false
		}
		return console.FindInputPrompt(confirmRe, password, data)
	})

	console.WaitForPrompt()
	time.Sleep(30 * time.Second)
	out, err := console.Execute(baseconfig)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	t.Logf(string(out))
	time.Sleep(30 * time.Second)
	for i := 0; i < 5; i++ {
		console.Writeln("")
	}
	console.WaitForPrompt()

}

func show_ztp_state(t *testing.T, dut *ondatra.DUTDevice, console *fptest.ConsoleIO) string {
	// console.WaitForPrompt()
	var ztpstate string
	var getRequest *gnmipb.GetRequest
	if dut.Vendor() == ondatra.CISCO {
		getRequest = &gnmipb.GetRequest{
			Path: []*gnmipb.Path{
				{Origin: "openconfig", Elem: []*gnmipb.PathElem{
					{Name: "Cisco-IOS-XR-ztp-oper:ztp"},
					{Name: "status"},
					{Name: "state"},
				}},
			},
			Type:     gnmipb.GetRequest_ALL,
			Encoding: gnmipb.Encoding_JSON_IETF,
		}

		res, err := dut.RawAPIs().GNMI(t).Get(context.Background(), getRequest)
		if err != nil {
			t.Fatal("There is error when getting configuration: ", err)

		}

		t.Logf("VAL:  %v ", res)
		for _, n := range res.Notification {
			for _, u := range n.Update {
				if u.GetVal().GetJsonIetfVal() == nil {
					t.Fatalf("got an update with a non JSON_IETF schema, got: %s", u)
				} else {
					t.Logf("Get value %v ", string(u.GetVal().GetJsonIetfVal()))
					ztpstate = string(u.GetVal().GetJsonIetfVal())
				}
			}
		}
	}
	return strings.Trim(ztpstate, "")
}

func show_ztp_log(t *testing.T, console *fptest.ConsoleIO, substring string) bool {
	console.WaitForPrompt()
	state, err := console.Execute(ciscoExecuteCommands.ShowZtpLog)
	if err != nil {
		t.Error("Error getting the status of ztp")
	}
	t.Logf("SHOW ZTP LOG: %v", string(state))
	if strings.Contains(string(state), substring) {
		return true
	}
	return false
}

func readFile(t *testing.T, filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Could not open file: %v", err)
	}
	defer file.Close()
	filecontent, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("Could not read file data: %v", err)
	}
	return (string(filecontent))

}
func modifyFile(t *testing.T, content string, filePath string) {
	inFile, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Could not open Inventory local %v ", err)
	}
	defer inFile.Close()

	_, err = inFile.Write([]byte(content))
	if err != nil {
		t.Fatalf("Could not modify the contents of inventory local %v ", err)
	}
}

func removeDecorators(baseconfig string) string {
	lines := strings.Split(baseconfig, "\n")

	// Find the line that starts with "hostname"
	var startIndex int
	for i, line := range lines {
		if strings.HasPrefix(line, "hostname") {
			startIndex = i
			break
		}
	}
	// Extract the configuration starting from the "hostname" line
	configLines := lines[startIndex:]

	// Join the configuration lines to form the new configuration
	newConfig := strings.Join(configLines, "\n")
	newConfig = "configure" + "\n" + newConfig
	return newConfig

}
func TestBootzFP(t *testing.T) {

	ciscoCfg := readFile(t, filepath.Join("testdata/cisco.cfg"))
	t.Logf("Cisco Cfg provided  %v", ciscoCfg)
	baseconfig = removeDecorators(ciscoCfg)
	inventoryLocal := readFile(t, filepath.Join("testdata/inventory_local.prototxt"))
	t.Logf("Inventory Local %v", inventoryLocal)

	em, _ := entityAPI.New("testdata/inventory_local.prototxt")
	chassisInventoryOriginal := em.GetChassisInventory()
	chassisInventory := em.GetChassisInventory()

	bServer := bootzServer.New()
	status, err := bServer.Start("5.38.4.124:8009", bootzServer.ServerConfig{
		DhcpIntf:          "ens224",
		ArtifactDirectory: "testdata/",
		InventoryConfig:   "testdata/inventory_local.prototxt",
	})
	t.Logf("%v", status)
	if err != nil {
		t.Fatalf("ERR: %v", err)
	}
	if status != bootzServer.BootzServerStatus_RUNNING {
		t.Errorf("Expected: %s, Received: %s", bootzServer.BootzServerStatus_RUNNING, status)
	}
	retries := 5
	interval := 3 * time.Minute
	for i := 1; i <= retries; i++ {
		if bServer.IsChassisConnected(service.EntityLookup{Manufacturer: "Cisco", SerialNumber: "FOC2503NLRY"}) {
			t.Log("CONNECTED")
			break
		}

		if i == retries {
			t.Log("Condition still false after 5 retries. Exiting.")
			break
		}

		t.Logf("Attempt %d: Condition is still false. Retrying in %s...\n", i, interval)
		time.Sleep(interval)
	}

	// if bServer.IsChassisConnected(service.EntityLookup{Manufacturer: "Cisco", SerialNumber: "FOC2503NLRY"}) {
	// }

	bootStatus, err := bServer.GetBootStatus("FOC2503NLRY")
	// validate boot status and ...
	t.Logf("%v", bootStatus)
	if err != nil {

		t.Fatalf("could not get get bootz status ")
	}
	t.Logf("Bootz status err %v", err)
	//Assuming that dhcp and bootz server is up

	dut := ondatra.DUT(t, "dut")

	cio := dut.RawAPIs().Console(t)
	defer cio.Close()
	console := fptest.NewConsoleIO(t, dut, cio)
	if err := console.Writeln(""); err != nil {
		t.Fatalf("error: %v", err)
	}
	switch dut.Vendor() {
	case ondatra.CISCO:
		ciscoExecuteCommands = execCommands{TerminalLength: "terminal length 0", ShowZtpLog: "show ztp log", ShowRun: "show run", ZtpClean: "ztp clean noprompt"}
		ciscoZtpStatus = ztpStatus{Success: "success", Terminated: "terminated"}
		ciscoConsoleRegexCommands = consoleCommands{EnterUserName: "Enter root-system username:",
			EnterPassword: "Enter secret:", EnterPasswordAgain: "Enter secret again:"}
	default:
		t.Fatalf("Exec, ZTP status and console commands not present for %v ", dut.Vendor().String())
	}

	t.Run("Bootz-1.1: Missing config", func(t *testing.T) {
		//giving an empty cisco.cfg file
		if *invalidISO == "" || *validISO == "" || *versionUpgrade == "" || *dhcpInterface == "" || *dhcpIp == "" {
			t.Fatalf("One or more of the flags is empty")
		}
		modifyFile(t, "", "testdata/empty.cfg")
		versionBefore := versionOnBox(t, dut)

		chassisInventory = chassisInventoryOriginal
		for _, ch := range chassisInventory {
			ch.Config.BootConfig.VendorConfigFile = "testdata/empty.cfg"
			ch.SoftwareImage.Version = versionBefore
			em.ReplaceDevice(&service.EntityLookup{Manufacturer: ch.GetManufacturer(), SerialNumber: ch.GetSerialNumber()}, ch)
		}

		t.Logf("CHASSIS INV AFTER %v ", chassisInventory)
		t.Log("Changed the contents fo the file cisco.cfg ")
		time.Sleep(30 * time.Second)
		initiateBootzOnDevice(t, dut, console)
		deviceBootStatus(t, dut, 20)
		baseConfig(t, console)
		deviceBootStatus(t, dut, 6)
		dutAfter := ondatra.DUT(t, "dut")
		versionAfter := versionOnBox(t, dutAfter)
		verifyVersionChange(t, versionBefore, versionAfter, false)
		ztpStatus := show_ztp_state(t, dutAfter, console)
		if !strings.Contains(ztpStatus, ciscoZtpStatus.Terminated) {
			t.Errorf("ZTP state %v : Expecting Failure", ztpStatus)
		}
		sshClient := dut.RawAPIs().CLI(t)
		if _, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.TerminalLength); err != nil {
			t.Errorf(" Error executing> %s", ciscoExecuteCommands.TerminalLength)
		}
		if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowZtpLog); err == nil {
			t.Logf("%s > %s", ciscoExecuteCommands.ShowZtpLog, result)
			ztpLog = result
		} else {
			t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowZtpLog, err)
		}
		if (!strings.Contains(string(ztpLog), "Provisioning failed")) && (!strings.Contains(string(ztpLog), "ZTP failed to complete")) {
			t.Errorf("Expected ztp to fail with : Provisioning failed and ZTP failed to complete errors ")
		}
	})
	t.Run("Bootz-1.2: Invalid config", func(t *testing.T) {
		versionBefore := versionOnBox(t, dut)
		//create cfg file with random string
		randomString := fmt.Sprintf("%d", rand.Int63())
		t.Log(randomString)
		modifyFile(t, randomString, "testdata/invalid.cfg")
		chassisInventory = chassisInventoryOriginal
		for _, ch := range chassisInventory {
			ch.Config.BootConfig.VendorConfigFile = "testdata/invalid.cfg"
			ch.SoftwareImage.Version = versionBefore
			em.ReplaceDevice(&service.EntityLookup{Manufacturer: ch.GetManufacturer(), SerialNumber: ch.GetSerialNumber()}, ch)
		}
		initiateBootzOnDevice(t, dut, console)
		deviceBootStatus(t, dut, 20)
		baseConfig(t, console)
		deviceBootStatus(t, dut, 6)
		dutAfter := ondatra.DUT(t, "dut")
		versionAfter := versionOnBox(t, dutAfter)
		verifyVersionChange(t, versionBefore, versionAfter, false)
		ztpStatus := show_ztp_state(t, dutAfter, console)
		if !strings.Contains(ztpStatus, ciscoZtpStatus.Terminated) {
			t.Errorf("ZTP state %v : Expecting Failure", ztpStatus)
		}
		sshClient := dut.RawAPIs().CLI(t)
		if _, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.TerminalLength); err != nil {
			t.Errorf(" Error executing> %s", ciscoExecuteCommands.TerminalLength)
		}
		if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowZtpLog); err == nil {
			t.Logf("%s > %s", ciscoExecuteCommands.ShowZtpLog, result)
			ztpLog = result
		} else {
			t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowZtpLog, err)
		}
		if (!strings.Contains(string(ztpLog), "Provisioning failed")) && (!strings.Contains(string(ztpLog), "ZTP failed to complete")) {
			t.Errorf("Expected ztp to fail with : Provisioning failed and ZTP failed to complete errors ")
		}

	})

	t.Run("Bootz-1.3: Valid configuration", func(t *testing.T) {
		versionBefore := versionOnBox(t, dut)
		//create cfg file with valid string
		modifyFile(t, ciscoCfg, "testdata/cisco.cfg")
		chassisInventory = chassisInventoryOriginal
		for _, ch := range chassisInventory {
			ch.Config.BootConfig.VendorConfigFile = "testdata/cisco.cfg"
			ch.SoftwareImage.Version = versionBefore
			em.ReplaceDevice(&service.EntityLookup{Manufacturer: dut.Vendor().String(), SerialNumber: ch.GetSerialNumber()}, ch)
		}
		initiateBootzOnDevice(t, dut, console)
		deviceBootStatus(t, dut, 20)
		dutAfter := ondatra.DUT(t, "dut")
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Get(t, dutAfter, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Expected base config to be present on box ztp failed and got testt.CaptureFatal errMsg : %s", *errMsg)
			baseConfig(t, console)
			deviceBootStatus(t, dut, 6)
		} else {
			versionAfter := versionOnBox(t, dutAfter)
			ztpStatus := show_ztp_state(t, dutAfter, console)
			if strings.Contains(strings.Trim(ztpStatus, "success"), ciscoZtpStatus.Success) {
				sshClient := dut.RawAPIs().CLI(t)
				verifyVersionChange(t, versionBefore, versionAfter, false)
				if _, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.TerminalLength); err != nil {
					t.Errorf(" Error executing> %s", ciscoExecuteCommands.TerminalLength)
				}
				if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowRun); err == nil {
					t.Logf("%s > %s", ciscoExecuteCommands.ShowRun, result)
					ztpLog = result
				} else {
					t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowRun, err)
				}

				verifyBaseConfig(t, ztpLog)
				if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowZtpLog); err == nil {
					t.Logf("%s > %s", ciscoExecuteCommands.ShowZtpLog, result)
					ztpLog = result
				} else {
					t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowZtpLog, err)
				}
				if (strings.Contains(string(ztpLog), "Provisioning complete")) && (strings.Contains(string(ztpLog), "ZTP completed successfully")) {
					t.Errorf("Expected ztp to fail with : Provisioning failed and ZTP failed to complete errors ")
				}
			} else {
				t.Errorf("ZTP state want: %v got:%v", "success", ztpStatus)
			}
		}
	})

	t.Run("Bootz-2.1 Software version is different", func(t *testing.T) {
		chassisInventory = chassisInventoryOriginal
		for _, ch := range chassisInventory {
			theImageUrl := ch.GetSoftwareImage().Url
			parts := strings.Split(theImageUrl, "/")
			parts[len(parts)-1] = *validISO
			fmt.Printf("%vsum %v", ch.SoftwareImage.HashAlgorithm, *validISO)
			cmd := exec.Command(fmt.Sprintf("%vsum", ch.SoftwareImage.HashAlgorithm), *validISO)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("Failed to get the hash of valid image : Error: %v", err)
			}
			invalidImage := strings.Join(parts, "/")
			ch.SoftwareImage.Url = invalidImage
			ch.SoftwareImage.OsImageHash = strings.Fields(string(output))[0]
			ch.SoftwareImage.Version = *versionUpgrade
			em.ReplaceDevice(&service.EntityLookup{Manufacturer: ch.GetManufacturer(), SerialNumber: ch.GetSerialNumber()}, ch)

		}
		versionBefore := versionOnBox(t, dut)
		versionProvided := *versionUpgrade
		initiateBootzOnDevice(t, dut, console)
		deviceBootStatus(t, dut, 20)
		dutAfter := ondatra.DUT(t, "dut")
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Get(t, dutAfter, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Expected base config to be present on box ztp failed and got testt.CaptureFatal errMsg : %s", *errMsg)
			baseConfig(t, console)
			console.WaitForPrompt()
			deviceBootStatus(t, dut, 6)
		} else {
			verifyVersionChange(t, versionBefore, versionProvided, true)
			ztpStatus := show_ztp_state(t, dutAfter, console)
			if strings.Contains(strings.Trim(ztpStatus, "success"), ciscoZtpStatus.Success) {
				sshClient := dut.RawAPIs().CLI(t)
				if _, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.TerminalLength); err != nil {
					t.Errorf(" Error executing> %s", ciscoExecuteCommands.TerminalLength)
				}
				if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowRun); err == nil {
					t.Logf("%s > %s", ciscoExecuteCommands.ShowRun, result)
					showrunAfter = result
				} else {
					t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowRun, err)
				}

				verifyBaseConfig(t, showrunAfter)
				if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowZtpLog); err == nil {
					t.Logf("%s > %s", ciscoExecuteCommands.ShowZtpLog, result)
					ztpLog = result
				} else {
					t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowZtpLog, err)
				}
				if (!strings.Contains(string(ztpLog), "Upgrade successfull")) && (!strings.Contains(string(ztpLog), "ZTP completed successfully")) {
					t.Errorf("Expected ztp to fail with : Upgrade successfull")
				}

			} else {
				t.Errorf("ZTP state want: %v got:%v", "success", ztpStatus)
			}
		}

	})
	t.Run("Bootz-2.2: Invalid software image", func(t *testing.T) {
		//TODO: access the url and change the name of the imgae
		chassisInventory = chassisInventoryOriginal
		dut = ondatra.DUT(t, "dut")
		versionBefore := versionOnBox(t, dut)
		for _, ch := range chassisInventory {
			theImageUrl := ch.GetSoftwareImage().Url
			parts := strings.Split(theImageUrl, "/")
			parts[len(parts)-1] = *invalidISO
			fmt.Printf("%vsum %v", ch.SoftwareImage.HashAlgorithm, *invalidISO)
			cmd := exec.Command(fmt.Sprintf("%vsum", ch.SoftwareImage.HashAlgorithm), *invalidISO)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("Failed to get the hash of invalid image : Error: %v", err)
				return
			}
			invalidImage := strings.Join(parts, "/")
			ch.SoftwareImage.Url = invalidImage
			ch.SoftwareImage.OsImageHash = strings.Fields(string(output))[0]
			ch.SoftwareImage.Version = versionBefore + "I"
			em.ReplaceDevice(&service.EntityLookup{Manufacturer: ch.GetManufacturer(), SerialNumber: ch.GetSerialNumber()}, ch)
		}
		t.Logf("CHASSIS INV %v", chassisInventory)
		initiateBootzOnDevice(t, dut, console)
		deviceBootStatus(t, dut, 20)
		baseConfig(t, console)
		deviceBootStatus(t, dut, 6)
		dutAfter := ondatra.DUT(t, "dut")
		versionAfter := versionOnBox(t, dutAfter)
		verifyVersionChange(t, versionBefore, versionAfter, false)
		ztpStatus := show_ztp_state(t, dutAfter, console)
		if !strings.Contains(ztpStatus, ciscoZtpStatus.Terminated) {
			t.Errorf("ZTP state %v : Expecting Failure", ztpStatus)
		}
		sshClient := dut.RawAPIs().CLI(t)
		if _, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.TerminalLength); err != nil {
			t.Errorf(" Error executing> %s", ciscoExecuteCommands.TerminalLength)
		}
		if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowZtpLog); err == nil {
			t.Logf("%s > %s", ciscoExecuteCommands.ShowZtpLog, result)
			ztpLog = result
		} else {
			t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowZtpLog, err)
		}
		// ztpLog := show_ztp_log(t, console, "Provisioning failed")
		if (!strings.Contains(string(ztpLog), "The ISO is invalid")) && (!strings.Contains(string(ztpLog), "ZTP failed to complete")) {
			t.Errorf("Expected ztp to fail with : The ISO is invalid")
		}

	})

	t.Run("Bootz-3.1 No ownership voucher", func(t *testing.T) {
		dut = ondatra.DUT(t, "dut")
		versionBefore := versionOnBox(t, dut)
		chassisInventory = chassisInventoryOriginal
		for _, ch := range chassisInventory {
			for _, cc := range ch.ControllerCards {
				cc.OwnershipVoucher = ""
			}
			ch.SoftwareImage.Version = versionBefore
			em.ReplaceDevice(&service.EntityLookup{Manufacturer: ch.GetManufacturer(), SerialNumber: ch.GetSerialNumber()}, ch)

		}
		t.Logf("CHASSIS INV %v", chassisInventory)

		initiateBootzOnDevice(t, dut, console)
		deviceBootStatus(t, dut, 20)
		baseConfig(t, console)
		deviceBootStatus(t, dut, 6)
		dutAfter := ondatra.DUT(t, "dut")
		versionAfter := versionOnBox(t, dutAfter)
		verifyVersionChange(t, versionBefore, versionAfter, false)
		ztpStatus := show_ztp_state(t, dutAfter, console)
		if !strings.Contains(ztpStatus, ciscoZtpStatus.Terminated) {
			t.Errorf("ZTP state %v : Expecting Failure", ztpStatus)
		}
		sshClient := dut.RawAPIs().CLI(t)
		if _, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.TerminalLength); err != nil {
			t.Errorf(" Error executing> %s", ciscoExecuteCommands.TerminalLength)
		}
		if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowZtpLog); err == nil {
			t.Logf("%s > %s", ciscoExecuteCommands.ShowZtpLog, result)
			ztpLog = result
		} else {
			t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowZtpLog, err)
		}
		// ztpLog := show_ztp_log(t, console, "Provisioning failed")
		if (!strings.Contains(string(ztpLog), "Empty OV or OC present in Bootstrap Request")) && (!strings.Contains(string(ztpLog), "ZTP failed to complete")) {
			t.Errorf("Expected ztp to fail with :  Empty OV or OC present in Bootstrap Request ")
		}
	})
	t.Run("Bootz-3.2 Invalid OV", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")
		versionBefore := versionOnBox(t, dut)
		chassisInventory = chassisInventoryOriginal
		for _, ch := range chassisInventory {
			for _, cc := range ch.ControllerCards {
				cc.OwnershipVoucher = "XYZ"
			}
			ch.SoftwareImage.Version = versionBefore
			em.ReplaceDevice(&service.EntityLookup{Manufacturer: ch.GetManufacturer(), SerialNumber: ch.GetSerialNumber()}, ch)

		}
		t.Logf("CHASSIS INV %v", chassisInventory)
		initiateBootzOnDevice(t, dut, console)
		deviceBootStatus(t, dut, 20)
		baseConfig(t, console)
		deviceBootStatus(t, dut, 6)
		dutAfter := ondatra.DUT(t, "dut")
		versionAfter := versionOnBox(t, dutAfter)
		verifyVersionChange(t, versionBefore, versionAfter, false)
		ztpStatus := show_ztp_state(t, dutAfter, console)
		if !strings.Contains(ztpStatus, ciscoZtpStatus.Terminated) {
			t.Errorf("ZTP state %v : Expecting Failure", ztpStatus)
		}
		sshClient := dut.RawAPIs().CLI(t)
		if _, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.TerminalLength); err != nil {
			t.Errorf(" Error executing> %s", ciscoExecuteCommands.TerminalLength)
		}
		if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowZtpLog); err == nil {
			t.Logf("%s > %s", ciscoExecuteCommands.ShowZtpLog, result)
			ztpLog = result
		} else {
			t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowZtpLog, err)
		}
		if (!strings.Contains(string(ztpLog), "artifact-validation-failed")) && (!strings.Contains(string(ztpLog), "ZTP failed to complete")) {
			t.Errorf("Expected ztp to fail with :  artifact-validation-failed ")
		}
	})
	t.Run("Bootz-3.3 OV Fails", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")
		versionBefore := versionOnBox(t, dut)
		chassisInventory = chassisInventoryOriginal
		for _, ch := range chassisInventory {
			for _, cc := range ch.ControllerCards {
				cc.SerialNumber = "XYZ"
			}
			ch.SoftwareImage.Version = versionBefore
			em.ReplaceDevice(&service.EntityLookup{Manufacturer: ch.GetManufacturer(), SerialNumber: ch.GetSerialNumber()}, ch)

		}
		initiateBootzOnDevice(t, dut, console)
		deviceBootStatus(t, dut, 20)
		baseConfig(t, console)
		deviceBootStatus(t, dut, 6)
		dutAfter := ondatra.DUT(t, "dut")
		versionAfter := versionOnBox(t, dutAfter)
		verifyVersionChange(t, versionBefore, versionAfter, false)
		ztpStatus := show_ztp_state(t, dutAfter, console)
		if !strings.Contains(ztpStatus, ciscoZtpStatus.Terminated) {
			t.Errorf("ZTP state %v : Expecting Failure", ztpStatus)
		}
		sshClient := dut.RawAPIs().CLI(t)
		if _, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.TerminalLength); err != nil {
			t.Errorf(" Error executing> %s", ciscoExecuteCommands.TerminalLength)
		}
		if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowZtpLog); err == nil {
			t.Logf("%s > %s", ciscoExecuteCommands.ShowZtpLog, result)
			ztpLog = result
		} else {
			t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowZtpLog, err)
		}
		if (!strings.Contains(string(ztpLog), "could not find controller card with serial")) && (!strings.Contains(string(ztpLog), "ZTP failed to complete")) {
			t.Errorf("Expected ztp to fail with :  Could not find controller card with serial number ")
		}
	})
	t.Run("Bootz-3.4 OV valid", func(t *testing.T) {
		//TODO: Get the serial number of device and replace with valid OV
		chassisInventory = chassisInventoryOriginal
		dut := ondatra.DUT(t, "dut")
		versionBefore := versionOnBox(t, dut)
		for _, ch := range chassisInventory {
			ch.SoftwareImage.Version = versionBefore
			em.ReplaceDevice(&service.EntityLookup{Manufacturer: ch.GetManufacturer(), SerialNumber: ch.GetSerialNumber()}, ch)
		}
		initiateBootzOnDevice(t, dut, console)
		deviceBootStatus(t, dut, 20)
		baseConfig(t, console)
		dutAfter := ondatra.DUT(t, "dut")
		versionAfter := versionOnBox(t, dutAfter)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Get(t, dutAfter, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Expected base config to be present on box ztp failed and got testt.CaptureFatal errMsg : %s", *errMsg)
			baseConfig(t, console)
			console.WaitForPrompt()
			deviceBootStatus(t, dut, 6)
		} else {
			verifyVersionChange(t, versionBefore, versionAfter, false)
			ztpStatus := show_ztp_state(t, dutAfter, console)
			if strings.Contains(strings.Trim(ztpStatus, "success"), ciscoZtpStatus.Success) {
				sshClient := dut.RawAPIs().CLI(t)
				if _, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.TerminalLength); err != nil {
					t.Errorf(" Error executing> %s", ciscoExecuteCommands.TerminalLength)
				}
				if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowRun); err == nil {
					t.Logf("%s > %s", ciscoExecuteCommands.ShowRun, result)
					showrunAfter = result
				} else {
					t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowRun, err)
				}

				verifyBaseConfig(t, showrunAfter)
				if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowZtpLog); err == nil {
					t.Logf("%s > %s", ciscoExecuteCommands.ShowZtpLog, result)
					ztpLog = result
				} else {
					t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowZtpLog, err)
				}
				if (!strings.Contains(string(ztpLog), "OV OC Validation completed with Success")) && (!strings.Contains(string(ztpLog), "ZTP completed successfully")) {
					t.Errorf("Expected ztp to fail with : OV OC Validation completed with Success")
				}

			} else {
				t.Errorf("ZTP state want: %v got:%v", "success", ztpStatus)
			}
		}
	})

	t.Run("Bootz-4.1 no OS provided", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")
		versionBefore := versionOnBox(t, dut)
		chassisInventory = chassisInventoryOriginal
		for _, ch := range chassisInventory {
			ch.SoftwareImage.Version = versionBefore + "I"
			ch.SoftwareImage.OsImageHash = ""
			ch.SoftwareImage.HashAlgorithm = ""
			ch.SoftwareImage.Url = ""
			em.ReplaceDevice(&service.EntityLookup{Manufacturer: ch.GetManufacturer(), SerialNumber: ch.GetSerialNumber()}, ch)
		}

		initiateBootzOnDevice(t, dut, console)
		deviceBootStatus(t, dut, 20)
		baseConfig(t, console)
		deviceBootStatus(t, dut, 6)
		dutAfter := ondatra.DUT(t, "dut")
		versionAfter := versionOnBox(t, dutAfter)
		verifyVersionChange(t, versionBefore, versionAfter, false)
		ztpStatus := show_ztp_state(t, dutAfter, console)
		if !strings.Contains(ztpStatus, ciscoZtpStatus.Terminated) {
			t.Errorf("ZTP state %v : Expecting Failure", ztpStatus)
		}
		sshClient := dut.RawAPIs().CLI(t)
		if _, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.TerminalLength); err != nil {
			t.Errorf(" Error executing> %s", ciscoExecuteCommands.TerminalLength)
		}
		if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowZtpLog); err == nil {
			t.Logf("%s > %s", ciscoExecuteCommands.ShowZtpLog, result)
			ztpLog = result
		} else {
			t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowZtpLog, err)
		}
		if (!strings.Contains(string(ztpLog), "Image URL or destination not mentioned")) && (!strings.Contains(string(ztpLog), "ZTP failed to complete")) {
			t.Errorf("Expected ztp to fail with :  Image URL or destination not mentioned ")
		}
	})
	t.Run("Bootz-4.2 failed to fetch image from remote URL	", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")
		versionBefore := versionOnBox(t, dut)
		chassisInventory = chassisInventoryOriginal
		for _, ch := range chassisInventory {
			imageUrl := ch.SoftwareImage.Url
			ch.SoftwareImage.Version = versionBefore + "I"
			ch.SoftwareImage.Url = imageUrl + "XYZ"
			em.ReplaceDevice(&service.EntityLookup{Manufacturer: ch.GetManufacturer(), SerialNumber: ch.GetSerialNumber()}, ch)
		}
		initiateBootzOnDevice(t, dut, console)
		deviceBootStatus(t, dut, 20)
		baseConfig(t, console)
		deviceBootStatus(t, dut, 6)
		dutAfter := ondatra.DUT(t, "dut")
		versionAfter := versionOnBox(t, dutAfter)
		verifyVersionChange(t, versionBefore, versionAfter, false)
		ztpStatus := show_ztp_state(t, dutAfter, console)
		if !strings.Contains(ztpStatus, ciscoZtpStatus.Terminated) {
			t.Errorf("ZTP state %v : Expecting Failure", ztpStatus)
		}
		sshClient := dut.RawAPIs().CLI(t)
		if _, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.TerminalLength); err != nil {
			t.Errorf(" Error executing> %s", ciscoExecuteCommands.TerminalLength)
		}
		if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowZtpLog); err == nil {
			t.Logf("%s > %s", ciscoExecuteCommands.ShowZtpLog, result)
			ztpLog = result
		} else {
			t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowZtpLog, err)
		}
		if (!strings.Contains(string(ztpLog), "http-get-request-failed")) && (!strings.Contains(string(ztpLog), "ZTP failed to complete")) {
			t.Errorf("Expected ztp to fail with :  http-get-request-failed ")
		}
	})

	t.Run("Bootz-4.2 OS checksum doesn't match	", func(t *testing.T) {
		dut := ondatra.DUT(t, "dut")
		versionBefore := versionOnBox(t, dut)
		chassisInventory = chassisInventoryOriginal
		for _, ch := range chassisInventory {
			ch.SoftwareImage.Version = versionBefore + "I"
			ch.SoftwareImage.OsImageHash = "XYZ"
			em.ReplaceDevice(&service.EntityLookup{Manufacturer: ch.GetManufacturer(), SerialNumber: ch.GetSerialNumber()}, ch)
		}
		initiateBootzOnDevice(t, dut, console)
		deviceBootStatus(t, dut, 20)
		baseConfig(t, console)
		deviceBootStatus(t, dut, 6)
		dutAfter := ondatra.DUT(t, "dut")
		versionAfter := versionOnBox(t, dutAfter)
		verifyVersionChange(t, versionBefore, versionAfter, false)
		ztpStatus := show_ztp_state(t, dutAfter, console)
		if !strings.Contains(ztpStatus, ciscoZtpStatus.Terminated) {
			t.Errorf("ZTP state %v : Expecting Failure", ztpStatus)
		}
		sshClient := dut.RawAPIs().CLI(t)
		if _, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.TerminalLength); err != nil {
			t.Errorf(" Error executing> %s", ciscoExecuteCommands.TerminalLength)
		}
		if result, err := sshClient.SendCommand(context.Background(), ciscoExecuteCommands.ShowZtpLog); err == nil {
			t.Logf("%s > %s", ciscoExecuteCommands.ShowZtpLog, result)
			ztpLog = result
		} else {
			t.Errorf("Error executing %s > %s", ciscoExecuteCommands.ShowZtpLog, err)
		}
		if (!strings.Contains(string(ztpLog), "Failed to verify Image Hash")) && (!strings.Contains(string(ztpLog), "ZTP failed to complete")) {
			t.Errorf("Expected ztp to fail with :  Failed to verify Image Hash")
		}
	})

}
