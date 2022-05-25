package basetest

import (
	"testing"

	"github.com/openconfig/ondatra"
)

func TestSSSHServerEnableConfig(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	config := dut.Config().System().SshServer().Enable()
	enable := true
	t.Run("Replace//system/ssh-server/config/enable", func(t *testing.T) {
		defer observer.RecordYgot(t, "REPLACE", config)

		config.Replace(t, enable)
		enable = false
		config.Replace(t, enable)
	})
	t.Run("Update//system/ssh-server/config/enable", func(t *testing.T) {
		defer observer.RecordYgot(t, "UPDATE", config)
		enable = true
		config.Update(t, enable)
		enable = false
		config.Update(t, enable)
	})
	t.Run("Delete//system/ssh-server/config/enable", func(t *testing.T) {
		defer observer.RecordYgot(t, "DELETE", config)
		enable = true
		config.Update(t, enable)
		config.Delete(t)
	})
}

func TestSSHEnableState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	config := dut.Config().System().SshServer().Enable()
	config.Replace(t, true)
	defer config.Delete(t)
	telemetry := dut.Telemetry().System().SshServer().Enable()
	t.Run("Subscribe//system/ssh-server/config/enable", func(t *testing.T) {
		defer observer.RecordYgot(t, "SUBSCRIBE", config)
		enabled := telemetry.Get(t)
		if enabled != true {
			t.Errorf("Ntp Enabled: got %t, want %t", enabled, true)
		}

	})
}
