package helpers

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

// AwaitDUTReboot waits for the DUT to boot by monitoring the System BootTime state.
// It returns the new boot time once it is confirmed to be strictly greater than the baseline.
func AwaitDUTReboot(t *testing.T, dut *ondatra.DUTDevice, lastBootTime uint64) uint64 {
	t.Helper()
	t.Logf("Waiting for DUT to boot up by monitoring System BootTime state (max 30 minutes)...")

	// Use gnmi.Watch to block until the BootTime is strictly greater than the pre-reboot BootTime.
	// We wait up to 30 minutes for the chassis to fully reboot and gNMI to come back online.
	var newTime uint64
	var ok bool
	timeout := 30 * time.Minute
	start := time.Now()
	for time.Since(start) < timeout {
		if val, present := gnmi.Lookup(t, dut, gnmi.OC().System().BootTime().State()).Val(); present {
			if val > lastBootTime || lastBootTime == 0 {
				newTime = val
				ok = true
				break
			}
		}
		time.Sleep(10 * time.Second)
	}

	if !ok {
		t.Fatalf("Timeout waiting for device boot time to update. Expected BootTime > %d", lastBootTime)
	}

	t.Logf("Device rebooted successfully. New boot time: %d", newTime)
	return newTime
}
