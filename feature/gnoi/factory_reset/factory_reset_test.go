package factory_reset_base 

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	frpb "github.com/openconfig/gnoi/factory_reset"
	"github.com/openconfig/ondatra"
)

var (
	rmknownhosts      = fmt.Sprintf("%s/.ssh/known_hosts", os.Getenv("HOME"))
	filesCreated      = []string{}
	fileCreateDevRand = "bash  dd if=/dev/urandom of=%s bs=1M count=2"
	checkFileExists   = "bash [ -f \"%s\" ] && echo \"YES_exists\""
	fileExists        = "YES_exists"
	fileCreate        = "bash fallocate -l %dM %s"
)

// performs factory reset
func factoryReset(t *testing.T, dut *ondatra.DUTDevice, devicePaths []string) {
	createFiles(t, dut, devicePaths)
	gnoiClient := dut.RawAPIs().GNOI().New(t)
	facRe, err := gnoiClient.FactoryReset().Start(context.Background(), &frpb.StartRequest{FactoryOs: false, ZeroFill: false})
	t.Logf("Factory reset Response %v ", facRe)
	if err != nil {
		t.Error(err)
	}
	t.Log("Sleep for 8 mins after factory reset and try connecting to box\n")
	time.Sleep(2 * time.Minute)
	DeviceBootStatus(t, dut)
	cmd := exec.Command("bash", "-c", fmt.Sprintf("rm -rf %s", rmknownhosts)).Run()
	if cmd != nil {
		t.Error(cmd)
	}
	cmd1 := exec.Command("bash", "-c", fmt.Sprintf("sshpass -p %s ssh -o StrictHostKeyChecking=no %s@%s -p %s", *sshPass, *sshUser, *sshIP, *sshPort)).Run()
	if cmd1 != nil {
		t.Error(cmd1)
	}
	checkFiles(t, dut)
}

func TestFactoryReset(t *testing.T) {
	if *sshIP == "" {
		t.Fatal("--ssh_ip flag must be set.")
	}

	dut := ondatra.DUT(t, "dut")

	type encryptionCommands struct {
		EncrytionStatus      string
		EncryptionActivate   string
		EncryptionDeactivate string
		DevicePaths          []string
	}
	var enCiscoCommands encryptionCommands

	switch dut.Vendor() {
	case ondatra.CISCO:
		var enCiscoCommands = encryptionCommands{EncrytionStatus: "show disk-encryption status", EncryptionActivate: "disk-encryption activate", EncryptionDeactivate: "disk-encryption deactivate", DevicePaths: []string{"/misc/disk1"}}
		t.Logf("Cisco commands for disk encryption %v ", enCiscoCommands)
	default:
		t.Fatalf("Disk Encryption commands for is missing for %v ", dut.Vendor().String())
	}

	cliHandle := dut.RawAPIs().CLI(t)

	showDiskEncryptionStatus, err := cliHandle.SendCommand(context.Background(), enCiscoCommands.EncrytionStatus)
	if err != nil {
		t.Error(err)
	}
	t.Logf("%v", (showDiskEncryptionStatus))
	if strings.Contains(showDiskEncryptionStatus, "Not Encrypted") {
		t.Log("Performing Factory reset without Encryption\n")
		factoryReset(t, dut, enCiscoCommands.DevicePaths)
		t.Log("Stablise after factory reset\n")
		time.Sleep(5 * time.Minute)
		t.Log("Activate Encryption\n")
		encrypt, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), enCiscoCommands.EncryptionActivate)
		t.Logf("Sleep for 5 mins after disk-encryption activate")
		time.Sleep(5 * time.Minute)
		t.Logf("%v", encrypt)
		if err != nil {
			t.Error(err)

		}
		DeviceBootStatus(t, dut)
		encrypt, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), enCiscoCommands.EncrytionStatus)
		t.Logf("%v", encrypt)
		if err != nil {
			t.Error(err)

		}
		t.Log("Wait for the system to stabalise\n")
		time.Sleep(5 * time.Minute)
		factoryReset(t, dut, enCiscoCommands.DevicePaths)
	} else {
		t.Log("Performing Factory reset with Encryption\n")
		factoryReset(t, dut, enCiscoCommands.DevicePaths)
		t.Log("Stablise after factory reset\n")
		time.Sleep(5 * time.Minute)
		t.Log("Deactivate Encryption\n")
		encrypt, err := dut.RawAPIs().CLI(t).SendCommand(context.Background(), enCiscoCommands.EncryptionDeactivate)
		t.Logf("Sleep for 5 mins after disk-encryption deactivate")
		time.Sleep(5 * time.Minute)
		t.Logf("%v", encrypt)
		if err != nil {
			t.Error(err)

		}
		DeviceBootStatus(t, dut)
		encrypt, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), enCiscoCommands.EncrytionStatus)
		t.Logf("%v", encrypt)
		if err != nil {
			t.Error(err)

		}
		t.Logf("Wait for the system to stabalise\n")
		time.Sleep(5 * time.Minute)
		factoryReset(t, dut, enCiscoCommands.DevicePaths)
	}

}
