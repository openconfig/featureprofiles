package sztp_base_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	frpb "github.com/openconfig/gnoi/factory_reset"
	"github.com/openconfig/ondatra"
)

type encryptionCommands struct {
	EncrytionStatus      string
	EncryptionActivate   string
	EncryptionDeactivate string
	DevicePaths          []string
}

var enCiscoCommands encryptionCommands

// performs factory reset
func factoryReset(t *testing.T, dut *ondatra.DUTDevice, devicePaths []string) {
	createFiles(t, dut, devicePaths)
	gnoiClient := dut.RawAPIs().GNOI().New(t)
	facRe, err := gnoiClient.FactoryReset().Start(context.Background(), &frpb.StartRequest{FactoryOs: false, ZeroFill: false})
	if err != nil {
		t.Fatalf("Failed to initiate Factory Reset on the device, Error : %v ", err)
	}
	t.Logf("Factory reset Response %v ", facRe)
	time.Sleep(2 * time.Minute)
	deviceBootStatus(t, dut)
	dutNew := ondatra.DUT(t, "dut")
	checkFiles(t, dutNew, filesCreated, true)
	t.Log("Factory reset successfull")
}

func TestFactoryReset(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	resp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
	t.Logf(resp)
	if strings.Contains(resp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}
	switch dut.Vendor() {
	case ondatra.CISCO:
		enCiscoCommands = encryptionCommands{EncrytionStatus: "show disk-encryption status", EncryptionActivate: "disk-encryption activate", EncryptionDeactivate: "disk-encryption deactivate", DevicePaths: []string{"/misc/disk1"}}
		t.Logf("Cisco commands for disk encryption %v ", enCiscoCommands)
	default:
		t.Fatalf("Disk Encryption commands is missing for %v ", dut.Vendor().String())
	}

	cli := dut.RawAPIs().CLI(t)

	showDiskEncryptionStatus, err := cli.SendCommand(context.Background(), enCiscoCommands.EncrytionStatus)
	if err != nil {
		t.Fatalf("Failed to send command %v on the device, Error: %v ", enCiscoCommands.EncrytionStatus, err)
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
		encrypt, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), enCiscoCommands.EncrytionStatus)
		if err != nil {
			t.Fatalf("Failed to send command %v on the router, Error : %v ", enCiscoCommands.EncrytionStatus, err)

		}
		t.Logf("Show device encryption status: %v", encrypt)
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
		if err != nil {
			t.Fatalf("Failed send command %v on the device, Error : %v ", enCiscoCommands.EncryptionDeactivate, err)

		}
		t.Logf("Device encrytion deactivate: %v", encrypt)
		t.Logf("Sleep for 5 mins after disk-encryption deactivate")
		time.Sleep(5 * time.Minute)
		deviceBootStatus(t, dut)
		encrypt, err = dut.RawAPIs().CLI(t).SendCommand(context.Background(), enCiscoCommands.EncrytionStatus)
		if err != nil {
			t.Fatalf("Failed to send command %v on the router, Error : %v ", enCiscoCommands.EncrytionStatus, err)

		}
		t.Logf("Show device encrytion status: %v", encrypt)
		t.Logf("Wait for the system to stabalise\n")
		time.Sleep(5 * time.Minute)
		factoryReset(t, dut, enCiscoCommands.DevicePaths)
	}

}
