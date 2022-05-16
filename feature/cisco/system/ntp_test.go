package system_base_test

import (
	"testing"

	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestNTPEnableConfig(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	config := dut.Config().System().Ntp().Enabled()
	t.Run("configreplace//system/ntp/config/enabled", func(t *testing.T) {
		defer observer.RecordYgot(t, "REPLACE", config)
		config.Replace(t, true)
		config.Replace(t, false)
	})
	t.Run("configupdate//system/ntp/config/enabled", func(t *testing.T) {
		defer observer.RecordYgot(t, "UPDATE", config)
		config.Update(t, true)
		config.Update(t, false)
	})
	t.Run("configdelete//system/ntp/config/enabled", func(t *testing.T) {
		defer observer.RecordYgot(t, "DELETE", config)
		config.Update(t, true)
		config.Delete(t)
	})
}

func TestNTPEnableState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	config := dut.Config().System().Ntp()
	model := &oc.System_Ntp{}
	model.Enabled = ygot.Bool(true)
	config.Replace(t, model)
	defer config.Delete(t)
	telemetry := dut.Telemetry().System().Ntp().Enabled()
	t.Run("state//system/ntp/config/enabled", func(t *testing.T) {
		defer observer.RecordYgot(t, "SUBSCRIBE", config)
		enabled := telemetry.Get(t)
		if enabled != true {
			t.Errorf("Ntp Enabled: got %t, want %t", enabled, true)
		}

	})
}
