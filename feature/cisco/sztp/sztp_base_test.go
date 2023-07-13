package sztp_base_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	fpb "github.com/openconfig/gnoi/file"
	"github.com/openconfig/gnoi/system"
	gnps "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	filesCreated      = []string{}
	fileCreateDevRand = "bash  dd if=/dev/urandom of=%s bs=1M count=2"
	checkFileExists   = "bash [ -f \"%s\" ] && echo \"YES_exists\""
	fileExists        = "YES_exists"
	fileCreate        = "bash fallocate -l %dM %s"
	remoteHashAfter   = ""
	localHashBefore   = ""
	content           = []byte{}
	backupFile        = ""
	bootzUrl          = `option bootz_bootstrap_servers "bootz://%v:50052/grpc"`
	sztpUrl           = `option bootstrap_servers "https://%v:5000/restconf/operations/ietf-sztp-bootstrap-server:get-bootstrap-data"`
	bootzFiles        = []string{"/misc/config/grpc/gnsi/authz.json", "/misc/config/grpc/gnsi/certz.json", "/misc/config/grpc/gnsi/pathz.json", "/misc/config/grpc/gnsi/credentialz.json", "/misc/config/bootz/credentialz.cfg", "/misc/config/bootz/vendor_config.cfg", "/misc/config/bootz/oc_config.json"}
	sjcdhcpdEntry     = `		host aaa-19-01-RP0 {
		hardware ethernet 78:bc:1a:c1:71:f8;	
		fixed-address 1.1.16.3;
			%v;
	   }
	host aaa-19-01-RP1 {
		hardware ethernet b0:26:80:21:3a:0c;
		fixed-address 1.1.16.4;
			%v;
	
	   }`
	bgldhcpEntry = `host SF-1 {
		hardware ethernet b0:c5:3c:e0:b0:8e;
		fixed-address 5.38.9.39;
		%v;
		}`
	bootzImageJson = `{
			"name": "mini_image.iso",
			"version": "%v",
			"url": "http://%v/%v.iso",
			"osImageHash": "%v",
			"hashAlgorithm": "sha256"
		}
		`
	sztpImageJson = `{
			"ietf-sztp-conveyed-info:onboarding-information" : {
			  "boot-image" : {
				"os-name" : "exr",
				"os-version" : "%v",
				"download-uri" : [ "http://%v/%v.iso" ],
				"image-verification" : [
				  {
					"hash-algorithm" : "ietf-sztp-conveyed-info:sha-256",
					"hash-value" : "%v"
				  }
				]
			  },
			  "configuration-handling" : "merge",
			  "pre-configuration-script-file" : "test/data/pre_config_script.py",
			  "configuration-file" : "test/data/configs.cfg",
			  "post-configuration-script-file" : "test/data/post_config_script.py"
			}
		  }
		  `
)

const maxRebootTime = 40 // 40 mins wait time for the factory reset and sztp to kick in
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// creating files before factory reset
func createFiles(t *testing.T, dut *ondatra.DUTDevice, devicePaths []string) {
	cli := dut.RawAPIs().CLI(t)
	for _, folderPath := range devicePaths {
		fPath := path.Join(folderPath, "devrandom.log")
		_, err := cli.SendCommand(context.Background(), fmt.Sprintf(fileCreateDevRand, fPath))
		if err != nil {
			t.Fatalf("Failed to create file devrandom.log in the path %v, Error: %v ", folderPath, err)
		}
		t.Log("Check if the file is created")
		time.Sleep(30 * time.Second)
		filesCreated = append(filesCreated, fPath)
		fPath = path.Join(folderPath, ".devrandom.log")
		_, err = cli.SendCommand(context.Background(), fmt.Sprintf(fileCreateDevRand, fPath))
		if err != nil {
			t.Fatalf("Failed to create file .devrandom.log in the path %v, Error: %v", folderPath, err)

		}

		filesCreated = append(filesCreated, fPath)
		fPath = path.Join(folderPath, "largeFile.log")
		_, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), fmt.Sprintf(fileCreate, 100, fPath))
		if err != nil {
			t.Fatalf("Failed to create file largeFile.log in the path %v, Error: %v", folderPath, err)
		}

		filesCreated = append(filesCreated, fPath)
	}
	for _, fP := range filesCreated {
		resp, err := cli.SendCommand(context.Background(), fmt.Sprintf(checkFileExists, fP))
		if err != nil {
			t.Fatalf("Failed to send command %s on the device, Error: %v", fmt.Sprintf(checkFileExists, fP), err)
		}
		t.Logf("%v", resp)
		if !strings.Contains(resp, fileExists) {
			t.Fatalf("Unable to Create a file object %s in device %s", fP, dut.Name())
		}
	}

}

// checkFiles check if the files created are deleted from the device after factory reset
func checkFiles(t *testing.T, dut *ondatra.DUTDevice, filesList []string, factReset bool) {
	for _, fP := range filesList {
		resp, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), fmt.Sprintf(checkFileExists, fP))
		if err != nil {
			t.Fatalf("Failed to send command %s on the device, Error: %v", fmt.Sprintf(checkFileExists, fP), err)
		}
		t.Logf(resp)
		if factReset == true {
			if strings.Contains(resp, fileExists) == true {
				t.Fatalf("File %s not cleared by system Reset, in device %s", fP, dut.Name())
			}
		} else {
			if strings.Contains(resp, fileExists) == false {
				t.Fatalf("File %s not created after bootz succeeded, in device %s", fP, dut.Name())
			}
		}
	}
}

func deviceBootStatus(t *testing.T, dut *ondatra.DUTDevice) {
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
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
}

func connect_remote_server(t *testing.T, username string, password string, serverIp string) *ssh.Client {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	// connect ot ssh server
	conn, err := ssh.Dial("tcp", serverIp+":22", config)
	if err != nil {
		t.Error(err)
	}
	return conn
}

func ssh_session(t *testing.T, conn *ssh.Client, cmd string) {
	session, err := conn.NewSession()
	if err != nil {
		t.Error(err)
	}
	// var stdoutBuf bytes.Buffer
	// session.Stdout = &stdoutBuf
	var stdout bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = os.Stderr

	t.Log(cmd)
	if strings.Contains(cmd, "pkill") {
		err := session.Run(cmd)
		if err != nil {
			exitError, ok := err.(*ssh.ExitError)
			if ok {
				// Check the exit status of pgrep
				exitStatus := exitError.ExitStatus()
				switch exitStatus {
				case 1:
					// Process not found
					t.Logf("process '%s' is not running", cmd)
				default:
					// Other exit status, return the original error
					t.Error(err)
				}
			}
		}
	} else {
		err = session.Run(cmd)
		if err != nil {
			t.Error(err)
		}
	}
	// t.Logf("result of smd %v", stdout.String())
	defer session.Close()
}

func modify_dhcp_entry(t *testing.T, conn *ssh.Client, hostEntry string, bootz bool) {
	sftpClient, err := sftp.NewClient(conn)
	if err != nil {
		t.Errorf("Failed to create SFTP session: %v", err)
	}
	defer sftpClient.Close()
	filePath := "/etc/dhcp/dhcpd.conf"
	remoteFile, err := sftpClient.Open(filePath)
	if err != nil {
		t.Errorf("Failed to open file: %v", err)
	}
	defer remoteFile.Close()
	contents, err := ioutil.ReadAll(remoteFile)
	if err != nil {
		t.Error(err)
	}
	dhcpFileContents := string(contents)
	hostIndex := strings.Index(dhcpFileContents, "host SF-1")
	if *sjcSetup == true {
		hostIndex = strings.Index(dhcpFileContents, "host aaa-19-01-RP0")
	}
	var previousStr string
	t.Log(hostEntry)
	if hostIndex != -1 {
		previousStr = dhcpFileContents[:hostIndex]
	}
	dhcpEntry := previousStr + hostEntry
	// fmt.Print(dhcpEntry)
	newFile, err := sftpClient.Create(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer newFile.Close()
	_, err = newFile.Write([]byte(dhcpEntry))
	if err != nil {
		t.Error(err)
	}
}

func scp_local_remote(t *testing.T, serverConn *ssh.Client, remoteFile string, localFile string, jsonContent string) {
	var err error
	if jsonContent != "" {
		content = []byte(jsonContent)
	} else {
		content, err = ioutil.ReadFile(localFile)
		if err != nil {
			t.Errorf("Error reading local file: %v", err)

		}
		// Calculate the SHA256 hash of the local file
		localHashBefore, err = calculateFileHash(localFile)
		if err != nil {
			t.Errorf("Error calculating local file hash before transfer: %v", err)

		}
	}
	// Create a new SFTP client session
	sftpClient, err := sftp.NewClient(serverConn)
	if err != nil {
		t.Errorf("Error creating SFTP client: %v", err)

	}
	defer sftpClient.Close()
	remoteFileWriter, err := sftpClient.Create(remoteFile)
	if err != nil {
		t.Errorf("Error creating remote file: %v", err)

	}
	defer remoteFileWriter.Close()
	// Write the local file content to the remote file
	_, err = remoteFileWriter.Write(content)
	if err != nil {
		t.Errorf("Error writing to remote file: %v", err)

	}
	// Calculate the SHA256 hash of the remote file
	remoteHashAfter, err = calculateRemoteFileHash(sftpClient, remoteFile)
	if err != nil {
		t.Errorf("Error calculating remote file hash after transfer: %v", err)

	}
	// Verify if the hash values are the same
	if jsonContent == "" {
		t.Log("File copied successfully")
		t.Logf("Local hash before transfer:%v", localHashBefore)
		t.Logf("Remote hash after transfer:%v", remoteHashAfter)
		if localHashBefore != remoteHashAfter {
			t.Errorf("File transfer failed: Hash mismatch")

		}
	}

}

// calculateFileHash calculates the SHA256 hash of a file
func calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	hashBytes := hash.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return hashString, nil
}

// calculateRemoteFileHash calculates the SHA256 hash of a remote file
func calculateRemoteFileHash(sftpClient *sftp.Client, remoteFilePath string) (string, error) {
	remoteFile, err := sftpClient.Open(remoteFilePath)
	if err != nil {
		return "", err
	}
	defer remoteFile.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, remoteFile); err != nil {
		return "", err
	}

	hashBytes := hash.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return hashString, nil
}

// splitIntoSegments splits a string into segments of a specified size
func splitIntoSegments(str string, segmentSize int) []string {
	var segments []string
	for i := 0; i < len(str); i += segmentSize {
		end := i + segmentSize
		if end > len(str) {
			end = len(str)
		}
		segments = append(segments, str[i:end])
	}
	return segments
}

func update_onboarding(t *testing.T, image string, hashValue string, bootz bool) string {
	var jsonContent string
	hashString := strings.Join(splitIntoSegments(hashValue, 2), ":")
	if bootz == true {
		jsonContent = fmt.Sprintf(bootzImageJson, image, sjcSztpServerIP, image, hashValue)
	} else {
		jsonContent = fmt.Sprintf(sztpImageJson, image, sjcSztpServerIP, image, hashString)
	}
	if *sjcSetup == true {
		if bootz == true {
			jsonContent = fmt.Sprintf(bootzImageJson, image, sjcSztpServerIP, image, hashValue)
		} else {
			jsonContent = fmt.Sprintf(sztpImageJson, image, sjcSztpServerIP, image, hashString)
		}
	}
	t.Log(jsonContent)
	return jsonContent
}

func create_backup_dhcpd_file(t *testing.T, dhcpServerConn *ssh.Client) {
	currentTime := time.Now()
	formattedTime := currentTime.Format("2023_01_02_15_04_05")
	backupFile = "/etc/dhcp/dhcpd.conf_" + formattedTime
	ssh_session(t, dhcpServerConn, fmt.Sprintf("yes | cp -rf /etc/dhcp/dhcpd.conf %s", backupFile))
}

func restore_backup_dhcpd_file(t *testing.T, dhcpServerConn *ssh.Client, backupFile string) {
	defer ssh_session(t, dhcpServerConn, fmt.Sprintf("yes | cp -rf %s /etc/dhcp/dhcpd.conf", backupFile))
}
func find_hwMac_address(t *testing.T, dut *ondatra.DUTDevice, interfaceName string) string {
	state := gnmi.OC().Interface(interfaceName).Ethernet().HwMacAddress()
	hwMac := gnmi.Get(t, dut, state.State())
	t.Logf(hwMac)
	return hwMac
}
func create_dhcpEntry(t *testing.T, dut *ondatra.DUTDevice, bootz bool) string {
	var dhcpEntry string
	if *sjcSetup == true {
		if bootz == true {
			bootzUrl = fmt.Sprintf(bootzUrl, sjcSztpServerIP)
			dhcpEntry = fmt.Sprintf(sjcdhcpdEntry, bootzUrl, bootzUrl)
			t.Log(dhcpEntry)
		} else {
			sztpUrl = fmt.Sprintf(sztpUrl, sjcSztpServerIP)
			dhcpEntry = fmt.Sprintf(sjcdhcpdEntry, sztpUrl, sztpUrl)
			t.Log(dhcpEntry)
		}
	} else {
		if bootz == true {
			bootzUrl = fmt.Sprintf(bootzUrl, sztpServerIP)
			dhcpEntry = fmt.Sprintf(bgldhcpEntry, bootzUrl)
			t.Log(dhcpEntry)
		} else {
			sztpUrl = fmt.Sprintf(sztpUrl, sztpServerIP)
			dhcpEntry = fmt.Sprintf(bgldhcpEntry, sztpUrl)
			t.Log(dhcpEntry)
		}

	}
	return dhcpEntry
}
func create_dhcpd_setup(t *testing.T, dhcpServerConn *ssh.Client, sztpServerIP string, dut *ondatra.DUTDevice, bootz bool) {
	t.Logf("Check if entry for host is present and modify the contents")

	hostEntry := create_dhcpEntry(t, dut, bootz)
	t.Log("HOST ENTRY DETAILS ")
	t.Log(hostEntry)

	modify_dhcp_entry(t, dhcpServerConn, hostEntry, bootz)
	t.Logf("Restart dhcp service after adding an entry")
	ssh_session(t, dhcpServerConn, "service dhcpd restart")
}
func start_sztp_server(t *testing.T, sztpServerConn *ssh.Client) {
	t.Logf("Starting sztp server")
	ssh_session(t, sztpServerConn, "tmux new-session -d -s script_session ' source /root/sztp-server/venv/bin/activate && cd /root/sztp-server/ && python3 /root/sztp-server/app.py'")

}
func stop_sztp_server(t *testing.T, sztpServerConn *ssh.Client) {
	t.Log("Killing any previous sztp sessions")
	ssh_session(t, sztpServerConn, "pkill -f app.py")
}
func stop_bootz_server(t *testing.T, sztpServerConn *ssh.Client) {
	t.Log("Killing any existing bootz sessions")
	ssh_session(t, sztpServerConn, "pkill -f bootz")
}
func findActiveRPActiveSerialNumber(t *testing.T, dut *ondatra.DUTDevice) (string, string, string) {
	var supervisors []string
	active_state := gnmi.OC().Component("0/RP0/CPU0").Name().State()
	active := gnmi.Get(t, dut, active_state)
	standby_state := gnmi.OC().Component("0/RP1/CPU0").Name().State()
	standby := gnmi.Get(t, dut, standby_state)
	supervisors = append(supervisors, active, standby)
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, dut, supervisors)
	t.Logf("ACTIVE RP %v", rpActiveBeforeSwitch)
	activeSerialNumber := gnmi.Get(t, dut, gnmi.OC().Component(rpActiveBeforeSwitch).SerialNo().State())
	t.Log(activeSerialNumber)
	return rpStandbyBeforeSwitch, rpActiveBeforeSwitch, activeSerialNumber
}
func updateVoucher(t *testing.T, dut *ondatra.DUTDevice, sztpServerConn *ssh.Client) {
	_, _, activeSerialNumber := findActiveRPActiveSerialNumber(t, dut)
	voucherContent := fmt.Sprintf(`
		{
			"certificates": {
				"private_key": "configuration/server/certificate/local_cert/ztp-server.com.key",
				"ca_cert": "configuration/server/certificate/local_cert/ztp-chain.pem",
				"certificate_chain": "configuration/server/certificate/local_cert/ztp-server.com.crt"
			},
			"security": {
				"sha512_hash": "true",
				"enable_tls": "true"
			},
			"OVOC": {
				"intermediate_cert": "certificates/ownercerts/3tiercert/intermediate.cert",
				"owner_cert": "certificates/ownercerts/3tiercert/owner.cert",
				"pdc_cert": "certificates/ownercerts/3tiercert/pdc.cert",
				"ov": "certificates/vouchers/%v/%v.vcj"
			},
			"name": "xr-config-bootz",
			"port": "50052",
			"serverIp": "1.1.1.103"
		}
	`, activeSerialNumber, activeSerialNumber)
	t.Log(voucherContent)
	scp_local_remote(t, sztpServerConn, "/root/bootz/sahubbal/bootz_server_exec/configuration/server/config_server/bootz_server_cfg.json", "", voucherContent)

}
func start_bootz_server(t *testing.T, sztpServerConn *ssh.Client, dut *ondatra.DUTDevice) {
	if *sjcSetup == true {
		updateVoucher(t, dut, sztpServerConn)
	}
	t.Log("Starting bootz server")
	ssh_session(t, sztpServerConn, "tmux new-session -d -s script_session 'cd /root/bootz/sahubbal/bootz_server_exec/ && ./dist/bootz_server_libc17'")
}

func remove_known_hosts(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Failed to get home directory:", err)
		return
	}

	// Construct the path to the known_hosts file
	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")

	// Check if the known_hosts file exists
	_, err = os.Stat(knownHostsPath)
	if os.IsNotExist(err) {
		fmt.Println("The known_hosts file does not exist.")
		return
	}

	// Remove the known_hosts file
	err = os.Remove(knownHostsPath)
	if err != nil {
		fmt.Println("Failed to remove the known_hosts file:", err)
		return
	}

	fmt.Println("The known_hosts file has been successfully removed.")
}
func ztp_initiate(t *testing.T, dut *ondatra.DUTDevice) {

	cli_handle := dut.RawAPIs().CLI(t)
	ztp_resp, err := cli_handle.SendCommand(context.Background(), "ztp clean noprompt")
	t.Log(ztp_resp)
	time.Sleep(60 * time.Second)
	//only for testing purpose
	ztp_resp, err = cli_handle.SendCommand(context.Background(), "run rm -rf /pkg/etc/giso_ztp.ini ")
	t.Log(ztp_resp)
	ztp_resp, err = cli_handle.SendCommand(context.Background(), "run rm -rf /var/log/ztp.log \n ztp initiate management dhcp4 noprompt")
	if err != nil {
		t.Error(err)
	}
	t.Logf("%v\n", ztp_resp)

	deviceBootStatus(t, dut)
	remove_known_hosts(t)
	dutNew := ondatra.DUT(t, "dut")
	version := config.CMDViaGNMI(context.Background(), t, dutNew, "show version")
	if !strings.Contains(version, bootVersion) {
		t.Error("Image upgrade unsucessfull")
	}
	for i := 1; i <= 10; i++ {
		cli_handle_new := dutNew.RawAPIs().CLI(t)
		ztp_logs, err := cli_handle_new.SendCommand(context.Background(), "show ztp log | utility tail -n 60")
		if err != nil {
			t.Error(err)
		}
		t.Logf("%v\n", ztp_logs)
		if strings.Contains(ztp_logs, "ZTP completed successfully") {
			return
		}
		time.Sleep(3 * time.Minute)
	}

	err = fmt.Errorf("ZTP Failed")
	t.Fatal(err)

}
func version_check(t *testing.T, dut *ondatra.DUTDevice, sztpServerConn *ssh.Client, dhcpServerConn *ssh.Client, upgradeImage bool, bootz bool) {
	if upgradeImage == false {
		state := gnmi.OC().Component("0/RP0/CPU0").SoftwareVersion()
		bootVersion = gnmi.Get(t, dut, state.State())
		t.Logf("Version on the box %v ", bootVersion)
		t.Logf("Not changing the image")
		jsonContent := update_onboarding(t, bootVersion, "", bootz)
		if bootz == true {
			scp_local_remote(t, sztpServerConn, "/root/bootz/sahubbal/bootz_server_exec/configuration/image/image_cfg_input.json", "", jsonContent)
			stop_bootz_server(t, sztpServerConn)

		} else {
			scp_local_remote(t, sztpServerConn, "/root/sztp-server/ztp/onboarding_information.json", "", jsonContent)
			stop_sztp_server(t, sztpServerConn)
		}

	} else {
		re := regexp.MustCompile(`\d+\.\d+\.\d+\.\d+[A-Za-z]`)
		bootVersion = re.FindString(*imagePath)
		scp_local_remote(t, dhcpServerConn, fmt.Sprintf("/var/www/html/%v.iso", bootVersion), *imagePath, "")
		jsonContent := update_onboarding(t, bootVersion, remoteHashAfter, bootz)
		if bootz == true {
			scp_local_remote(t, sztpServerConn, "/root/bootz/sahubbal/bootz_server_exec/configuration/image/image_cfg_input.json", "", jsonContent)
			stop_bootz_server(t, sztpServerConn)
		} else {
			scp_local_remote(t, sztpServerConn, "/root/sztp-server/ztp/onboarding_information.json", "", jsonContent)
			stop_sztp_server(t, sztpServerConn)
		}

	}
}
func decodeBase64(t *testing.T, encodedString string) string {
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedString)
	if err != nil {
		t.Errorf("Error decoding Base64: %v", err)
	}

	decodedString := string(decodedBytes)
	t.Logf("Decoded string: %v", decodedString)
	return decodedString
}
func rpSwitchOver(t *testing.T, dut *ondatra.DUTDevice) {
	// find active and standby RP
	rpStandbyBeforeSwitch, rpActiveBeforeSwitch, _ := findActiveRPActiveSerialNumber(t, dut)
	t.Logf("Detected activeRP: %v, standbyRP: %v", rpActiveBeforeSwitch, rpStandbyBeforeSwitch)

	// make sure standby RP is reach
	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}
	gnoiClient := dut.RawAPIs().GNOI().New(t)
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &gnps.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)
	deviceBootStatus(t, dut)

}
func verify_bootz(t *testing.T, dut *ondatra.DUTDevice) {
	cli_handle_new := dut.RawAPIs().CLI(t)
	ztp_logs, err := cli_handle_new.SendCommand(context.Background(), fmt.Sprint(`run [ -d "/misc/config/grpc/gnsi/" ] && echo "yes" || echo "no"`))
	if err != nil {
		t.Fatalf("Failed to send command %s on the device, Error: %v", fmt.Sprint(`run [ -d "/misc/config/grpc/gnsi/" ] && echo "yes" || echo "no"`), err)
	}
	if strings.Contains(ztp_logs, "no") {
		t.Errorf("Bootz was successfull but gnsi directory not created")
	}
	//checking if authz certz pathz and credz json files are in location /misc/config/grpc/gnsi and if credentailz.cfg vendor.json and oc_config.json is present in /misc/config/bootz
	checkFiles(t, dut, bootzFiles, false)
	//gNMI query after bootz
	t.Log(gnmi.Get(t, dut, gnmi.OC().System().Hostname().State()))
	gnoiClient := dut.RawAPIs().GNOI().New(t)
	in := &fpb.StatRequest{
		Path: "/misc/config/grpc/gnsi",
	}
	//gNOI query after bootz
	validResponse, err := gnoiClient.File().Stat(context.Background(), in)
	if err != nil {
		t.Errorf("Unable to stat path %v  on DUT, %v", "/misc/config/grpc/gnsi", err)
	}
	t.Log(validResponse)
	gribiClient := gribi.Client{
		DUT:                   dut,
		FibACK:                false,
		Persistence:           true,
		InitialElectionIDLow:  10,
		InitialElectionIDHigh: 0,
	}
	defer gribiClient.Close(t)
	//gRIBI query after bootz
	if err := gribiClient.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	//process restart emsd
	killResponse, err := dut.RawAPIs().GNOI().Default(t).System().KillProcess(context.Background(), &system.KillProcessRequest{Name: "emsd", Restart: true, Signal: system.KillProcessRequest_SIGNAL_TERM})
	t.Logf("Got kill process response: %v\n\n", killResponse)
	if err != nil {
		t.Fatalf("Failed to execute gNOI Kill Process, error received: %v", err)
	}
	time.Sleep(60 * time.Second)
	//check if gnmi is reachable post emsd restart
	t.Log(gnmi.Get(t, dut, gnmi.OC().System().Hostname().State()))

}
