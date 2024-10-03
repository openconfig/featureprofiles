package basetest

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func testSSHServerEnableConfig(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	config := gnmi.OC().System().SshServer().Enable()
	enable := true
	defer gnmi.Replace(t, dut, gnmi.OC().System().SshServer().Enable().Config(), true)
	t.Run("Replace//system/ssh-server/config/enable", func(t *testing.T) {
		defer observer.RecordYgot(t, "REPLACE", config)

		gnmi.Replace(t, dut, config.Config(), enable)
		enable = false
		gnmi.Replace(t, dut, config.Config(), enable)
	})
	t.Run("Update//system/ssh-server/config/enable", func(t *testing.T) {
		defer observer.RecordYgot(t, "UPDATE", config)
		enable = true
		gnmi.Update(t, dut, config.Config(), enable)
		enable = false
		gnmi.Update(t, dut, config.Config(), enable)
	})
	t.Run("Delete//system/ssh-server/config/enable", func(t *testing.T) {
		defer observer.RecordYgot(t, "DELETE", config)
		enable = true
		gnmi.Update(t, dut, config.Config(), enable)
		gnmi.Delete(t, dut, config.Config())
	})
}

func testSSHEnableState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	defer gnmi.Replace(t, dut, gnmi.OC().System().SshServer().Enable().Config(), true)
	config := gnmi.OC().System().SshServer().Enable()
	gnmi.Replace(t, dut, config.Config(), true)
	defer gnmi.Delete(t, dut, config.Config())
	telemetry := gnmi.OC().System().SshServer().Enable()
	t.Run("Subscribe//system/ssh-server/config/enable", func(t *testing.T) {
		defer observer.RecordYgot(t, "SUBSCRIBE", config)
		enabled := gnmi.Get(t, dut, telemetry.State())
		if enabled != true {
			t.Errorf("SSH not Enabled: got %t, want %t", enabled, true)
		}

	})
}
