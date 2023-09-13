package factoryreset

import (
	"context"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	frpb "github.com/openconfig/gnoi/factory_reset"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
)

var (
	filesCreated      = []string{}
	fileCreateDevRand = "bash  dd if=/dev/urandom of=%s bs=1M count=2"
	checkFileExists   = "bash [ -f \"%s\" ] && echo \"YES_exists\""
	fileExists        = "YES_exists"
	fileCreate        = "bash fallocate -l %dM %s"
)

const maxRebootTime = 40 // 40 mins wait time for the factory reset and sztp to kick in
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type encryptionCommands struct {
	EncryptionStatus     string
	EncryptionActivate   string
	EncryptionDeactivate string
	DevicePaths          []string
}

var enCiscoCommands encryptionCommands

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
	for _, f := range filesCreated {
		resp, err := cli.SendCommand(context.Background(), fmt.Sprintf(checkFileExists, f))
		if err != nil {
			t.Fatalf("Failed to send command %s on the device, Error: %v", fmt.Sprintf(checkFileExists, f), err)
		}
		t.Logf("%v", resp)
		if !strings.Contains(resp, fileExists) {
			t.Fatalf("Unable to Create a file object %s in device %s", f, dut.Name())
		}
	}

}

// checkFiles check if the files created are deleted from the device after factory reset
func checkFiles(t *testing.T, dut *ondatra.DUTDevice) {
	for _, f := range filesCreated {

		resp, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), fmt.Sprintf(checkFileExists, f))
		if err != nil {
			t.Fatalf("Failed to send command %s on the device, Error: %v", fmt.Sprintf(checkFileExists, f), err)
		}
		t.Logf(resp)
		if strings.Contains(resp, fileExists) == true {
			t.Fatalf("File %s not cleared by system Reset, in device %s", f, dut.Name())
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

// performs factory reset
func factoryReset(t *testing.T, dut *ondatra.DUTDevice, devicePaths []string) {
	createFiles(t, dut, devicePaths)
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	facRe, err := gnoiClient.FactoryReset().Start(context.Background(), &frpb.StartRequest{FactoryOs: false, ZeroFill: false})
	if err != nil {
		t.Fatalf("Failed to initiate Factory Reset on the device, Error : %v ", err)
	}
	t.Logf("Factory reset Response %v ", facRe)
	time.Sleep(2 * time.Minute)
	deviceBootStatus(t, dut)
	dutNew := ondatra.DUT(t, "dut")
	checkFiles(t, dutNew)
	t.Log("Factory reset successfull")
}

func TestFactoryReset(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	switch dut.Vendor() {
	case ondatra.CISCO:
		enCiscoCommands = encryptionCommands{EncryptionStatus: "show disk-encryption status", EncryptionActivate: "disk-encryption activate", EncryptionDeactivate: "disk-encryption deactivate", DevicePaths: []string{"/misc/disk1"}}
		t.Logf("Cisco commands for disk encryption %v ", enCiscoCommands)
	default:
		t.Fatalf("Disk Encryption commands is missing for %v ", dut.Vendor().String())
	}

	cli := dut.RawAPIs().CLI(t)

	showDiskEncryptionStatus, err := cli.SendCommand(context.Background(), enCiscoCommands.EncryptionStatus)
	if err != nil {
		t.Fatalf("Failed to send command %v on the device, Error: %v ", enCiscoCommands.EncryptionStatus, err)
	}
	t.Logf("Disk encryption status %v", showDiskEncryptionStatus)

	if strings.Contains(showDiskEncryptionStatus, "Not Encrypted") {
		t.Log("Performing Factory reset without Encryption\n")
		factoryReset(t, dut, enCiscoCommands.DevicePaths)
		t.Log("Stablise after factory reset\n")
		time.Sleep(5 * time.Minute)
		t.Log("Activate Encryption\n")
		encrypt, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), enCiscoCommands.EncryptionActivate)
		t.Logf("Sleep for 5 mins after disk-encryption activate")
		time.Sleep(5 * time.Minute)
		if err != nil {
			t.Fatalf("Failed to send command %v on the device, Error : %v ", enCiscoCommands.EncryptionActivate, err)

		}
		t.Logf("Device encryption acrivare: %v", encrypt)
		deviceBootStatus(t, dut)
		encrypt, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), enCiscoCommands.EncryptionStatus)
		if err != nil {
			t.Fatalf("Failed to send command %v on the router, Error : %v ", enCiscoCommands.EncryptionStatus, err)

		}
		t.Logf("Show device encryption status: %v", encrypt)
		t.Log("Wait for the system to stabilize\n")
		time.Sleep(5 * time.Minute)
		factoryReset(t, dut, enCiscoCommands.DevicePaths)
	} else {
		t.Log("Performing Factory reset with Encryption\n")
		factoryReset(t, dut, enCiscoCommands.DevicePaths)
		t.Log("Stablise after factory reset\n")
		time.Sleep(5 * time.Minute)
		t.Log("Deactivate Encryption\n")
		encrypt, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), enCiscoCommands.EncryptionDeactivate)
		if err != nil {
			t.Fatalf("Failed send command %v on the device, Error : %v ", enCiscoCommands.EncryptionDeactivate, err)

		}
		t.Logf("Device encrytion deactivate: %v", encrypt)
		t.Logf("Sleep for 5 mins after disk-encryption deactivate")
		time.Sleep(5 * time.Minute)
		deviceBootStatus(t, dut)
		encrypt, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), enCiscoCommands.EncryptionStatus)
		if err != nil {
			t.Fatalf("Failed to send command %v on the router, Error : %v ", enCiscoCommands.EncryptionStatus, err)

		}
		t.Logf("Show device encrytion status: %v", encrypt)
		t.Logf("Wait for the system to stabilize\n")
		time.Sleep(5 * time.Minute)
		factoryReset(t, dut, enCiscoCommands.DevicePaths)
	}
}
