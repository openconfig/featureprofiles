package helpers

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
)

// AwaitDUTReboot waits for the DUT to boot by monitoring the System BootTime state.
// It returns the new boot time once it is confirmed to be strictly greater than the baseline.
func AwaitDUTReboot(t *testing.T, dut *ondatra.DUTDevice, lastBootTime uint64) uint64 {
	t.Helper()
	t.Logf("Waiting for DUT to boot up by monitoring System BootTime state (max 30 minutes)...")

	// Use gnmi.Watch to block until the BootTime is strictly greater than the pre-reboot BootTime.
	// We wait up to 30 minutes for the chassis to fully reboot and gNMI to come back online.
	val, ok := gnmi.Watch(t, dut, gnmi.OC().System().BootTime().State(), 30*time.Minute, func(val *ygnmi.Value[uint64]) bool {
		newTime, present := val.Val()
		if !present {
			return false
		}
		// Consider the reboot complete only when the system registers a completely new epoch boot time.
		if newTime > lastBootTime || lastBootTime == 0 {
			return true
		}
		return false
	}).Await(t)

	if !ok {
		t.Fatalf("Timeout waiting for device boot time to update. Expected BootTime > %d", lastBootTime)
	}

	newTime, _ := val.Val()
	t.Logf("Device rebooted successfully. New boot time: %d", newTime)
	return newTime
}
