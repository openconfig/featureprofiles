package basetest

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func testNTPEnableConfig(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	config := gnmi.OC().System().Ntp().Enabled()
	t.Run("Replace//system/ntp/config/enabled", func(t *testing.T) {
		defer observer.RecordYgot(t, "REPLACE", config)
		gnmi.Replace(t, dut, config.Config(), true)
		gnmi.Replace(t, dut, config.Config(), false)
	})
	t.Run("Update//system/ntp/config/enabled", func(t *testing.T) {
		defer observer.RecordYgot(t, "UPDATE", config)
		gnmi.Update(t, dut, config.Config(), true)
		gnmi.Update(t, dut, config.Config(), false)
	})
	t.Run("Delete//system/ntp/config/enabled", func(t *testing.T) {
		defer observer.RecordYgot(t, "DELETE", config)
		gnmi.Update(t, dut, config.Config(), true)
		gnmi.Delete(t, dut, config.Config())
	})
}

func testNTPEnableState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	config := gnmi.OC().System().Ntp()
	model := &oc.System_Ntp{}
	model.Enabled = ygot.Bool(true)
	gnmi.Replace(t, dut, config.Config(), model)
	defer gnmi.Delete(t, dut, config.Config())
	telemetry := gnmi.OC().System().Ntp().Enabled()
	t.Run("Subscribe//system/ntp/config/enabled", func(t *testing.T) {
		defer observer.RecordYgot(t, "SUBSCRIBE", config)
		enabled := gnmi.Get(t, dut, telemetry.State())
		if enabled != true {
			t.Errorf("Ntp Enabled: got %t, want %t", enabled, true)
		}

	})
}
