package basetest

import (
	"strings"
	"testing"

	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestPlatformCPUState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("state//components/component/cpu/utilization/state/avg", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().Avg()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val < 1 {
			t.Errorf("Platform CPU Avg: got %d, want > %d", val, 0)

		}
	})
	t.Run("state//components/component/cpu/utilization/state/min", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().Min()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val < 1 {
			t.Errorf("Platform CPU  Min: got %d, want >%d", val, 0)

		}
	})
	t.Run("state//components/component/cpu/utilization/state/max", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().Max()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val < 1 {
			t.Errorf("Platform  CPU Max: got %d, want >%d", val, 0)

		}
	})
	t.Run("state//components/component/cpu/utilization/state/instant", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val < 1 {
			t.Errorf("Platform  CPU Instant: got %d, want >%d", val, 0)

		}
	})
	t.Run("state//components/component/cpu/utilization/state/max-time", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().MaxTime()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val < 1 {
			t.Errorf("Platform  CPU MaxTime: got %d, want >%d", val, 0)

		}
	})
	t.Run("state//components/component/cpu/utilization/state/min-time", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().MinTime()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val < 1 {
			t.Errorf("Platform  CPU MinTime: got %d, want >%d", val, 0)

		}
	})
	t.Run("state//components/component/cpu/utilization/state/Interval", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().Interval()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val < 1 {
			t.Errorf("Platform CPU interval: got %d, want >%d", val, 0)

		}
	})
}

func TestPlatformFanTrayState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("state//components/component/state/serial-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FanTray).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform FanTray SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("state//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FanTray).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform FanTray HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("state//components/component/state/oper-status", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FanTray).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform FanTray  OperStatus: got %s, want > %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("state//components/component/state/description", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FanTray).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Fan") {
			t.Errorf("Platform FanTray Description: got %s, should contain %s", val, "Fan")

		}
	})

}

func TestPlatformChassisState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("state//components/component/state/serial-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Chassis).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform Chassis SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("state//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Chassis).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform Chassis HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("state//components/component/state/oper-status", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Chassis).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform Chassis  OperStatus: got %s, want > %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("state//components/component/state/description", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Chassis).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Chassis") {
			t.Errorf("Platform Chassis Description: got %s, should contain %s", val, "Chassis")

		}
	})

}
func TestPlatformPSUState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("state//components/component/state/serial-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.PowerSupply).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform PowerSupply SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("state//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.PowerSupply).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform PowerSupply HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("state//components/component/state/oper-status", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.PowerSupply).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform PowerSupply  OperStatus: got %s, want > %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("state//components/component/state/description", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.PowerSupply).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Power") {
			t.Errorf("Platform PowerSupply Description: got %s, should contain %s", val, "Power")

		}
	})

}

func TestPlatformTransceiverState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("state//components/component/state/serial-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.OpticsModule).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform OpticsModule SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("state//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.OpticsModule).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform OpticsModule HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("state//components/component/state/oper-status", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.OpticsModule).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform OpticsModule  OperStatus: got %s, want > %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("state//components/component/state/description", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.OpticsModule).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Optics") {
			t.Errorf("Platform OpticsModule Description: got %s, should contain %s", val, "Optics")

		}
	})
	t.Run("state//components/component/state/type", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.OpticsModule).Type()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER {
			t.Errorf("Platform OpticsModule  OperStatus: got %s, want > %s", val, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER)

		}
	})

}

func TestPlatformPSUIOState(t *testing.T) {
	// dut := ondatra.DUT(t, device1)
	//PSU model (https://github.com/openconfig/public/blob/master/release/models/platform/openconfig-platform-psu.yang)  not included  in  ondatra
}

func TestTransceiverchannel(t *testing.T) {
	// Failure due to CSCwb72703
	dut := ondatra.DUT(t, device1)
	t.Run("state//components/component/transceiver/state/form-factor", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Transceiver).Transceiver().FormFactor()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_SFP {
			t.Errorf("Platform Transceiverchannel FormFactor: got %s, want != %s", val, oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_SFP)

		}
	})
	t.Run("state//components/component/transceiver/physical-channels/channel/state/input-power/instant", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Transceiver).Transceiver().Channel(1).InputPower().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != float64(0) {
			t.Errorf("Platform Transceiverchannel Channel InputPower Instant: got %v, want != %v", val, float64(0))

		}
	})
	t.Run("state//components/component/transceiver/physical-channels/channel/state/output-power/instant", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Transceiver).Transceiver().Channel(1).OutputPower().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != float64(0) {
			t.Errorf("Platform Transceiverchannel Channel OutputPower Instant: got %v, want != %v", val, float64(0))

		}
	})
	t.Run("state//components/component/transceiver/physical-channels/channel/state/laser-bias-current/instant", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Transceiver).Transceiver().Channel(1).LaserBiasCurrent().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != float64(0) {
			t.Errorf("Platform Transceiverchannel Channel LaserBiasCurrent Instant: got %v, want != %v", val, float64(0))

		}
	})

}

func TestTempSensor(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("state//components/component/state/temperature/instant", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.TempSensor).Temperature().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == float64(0) {
			t.Errorf("Platform Temperature Instant: got %v, want != %v", val, float64(0))

		}
	})
}

func TestFirmware(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("state//components/component/state/firmware-version", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.BiosFirmware).FirmwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform FirmwareVersion: got %s, want != %s", val, "''")

		}
	})
}

func TestSWVersion(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("state//components/component/state/software-version", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.SWVersionComponent).SoftwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform SoftwareVersion : got %s, want != %s", val, "''")

		}
	})
}

func TestFabric(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("state//components/component/state/serial-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform Fabric SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("state//components/component/state/description", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Fabric") {
			t.Errorf("Platform Fabric Description: got %s, should contain %s", val, "Fabric")

		}
	})
}

func TestSubComponent(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("state//components/component/subcomponents/subcomponent/state/name", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Chassis).Subcomponent(Platform.SubComponent).Name()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform Subcomponent Name: got %s, want != %s", val, "''")

		}
	})
}
