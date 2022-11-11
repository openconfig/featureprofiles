package basetest

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestPlatformCPUState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe//components/component/cpu/utilization/state/avg", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().Avg()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform CPU Avg value")
		} else {
			t.Errorf("Platform CPU Avg: got %d, want > %d", val, 0)

		}
	})
	t.Run("Subscribe//components/component/cpu/utilization/state/min", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().Min()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform CPU  Min value")
		} else {
			t.Errorf("Platform CPU  Min: got %d, want >%d", val, 0)

		}
	})
	t.Run("Subscribe//components/component/cpu/utilization/state/max", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().Max()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform  CPU Max value")
		} else {
			t.Errorf("Platform  CPU Max: got %d, want >%d", val, 0)

		}
	})
	t.Run("Subscribe//components/component/cpu/utilization/state/instant", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform  CPU Instant value")
		} else {
			t.Errorf("Platform  CPU Instant: got %d, want >%d", val, 0)

		}
	})
	t.Run("Subscribe//components/component/cpu/utilization/state/max-time", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().MaxTime()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform  CPU MaxTime value")
		} else {
			t.Errorf("Platform  CPU MaxTime: got %d, want >%d", val, 0)

		}
	})
	t.Run("Subscribe//components/component/cpu/utilization/state/min-time", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().MinTime()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform  CPU MinTime value")
		} else {
			t.Errorf("Platform  CPU MinTime: got %d, want >%d", val, 0)

		}
	})
	t.Run("Subscribe//components/component/cpu/utilization/state/Interval", func(t *testing.T) {
		state := dut.Telemetry().Component(RP).Cpu().Utilization().Interval()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform CPU interval value")
		} else {
			t.Errorf("Platform CPU interval: got %d, want >%d", val, 0)

		}
	})
}

func TestPlatformFanTrayState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FanTray).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform FanTray SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FanTray).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform FanTray HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FanTray).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform FanTray  OperStatus: got %s, want > %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
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
	t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Chassis).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform Chassis SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Chassis).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform Chassis HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Chassis).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform Chassis  OperStatus: got %s, want > %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
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
	t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.PowerSupply).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform PowerSupply SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.PowerSupply).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform PowerSupply HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.PowerSupply).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform PowerSupply  OperStatus: got %s, want > %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.PowerSupply).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Power") {
			t.Errorf("Platform PowerSupply Description: got %s, should contain %s", val, "Power")

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
	t.Run("Subscribe//components/component/transceiver/state/form-factor", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Transceiver).Transceiver().FormFactor()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_SFP {
			t.Errorf("Platform Transceiverchannel FormFactor: got %s, want != %s", val, oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_SFP)

		}
	})
	t.Run("Subscribe//components/component/transceiver/physical-channels/channel/state/input-power/instant", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Transceiver).Transceiver().Channel(1).InputPower().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != float64(0) {
			t.Errorf("Platform Transceiverchannel Channel InputPower Instant: got %v, want != %v", val, float64(0))

		}
	})
	t.Run("Subscribe//components/component/transceiver/physical-channels/channel/state/output-power/instant", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Transceiver).Transceiver().Channel(1).OutputPower().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != float64(0) {
			t.Errorf("Platform Transceiverchannel Channel OutputPower Instant: got %v, want != %v", val, float64(0))

		}
	})
	t.Run("Subscribe//components/component/transceiver/physical-channels/channel/state/laser-bias-current/instant", func(t *testing.T) {
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
	t.Run("Subscribe//components/component/state/temperature/instant", func(t *testing.T) {
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
	t.Run("Subscribe//components/component/state/firmware-version", func(t *testing.T) {
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
	t.Run("Subscribe//components/component/state/software-version", func(t *testing.T) {
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
	t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform Fabric SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
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
	t.Run("Subscribe//components/component/subcomponents/subcomponent/state/name", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Chassis).Subcomponent(Platform.SubComponent).Name()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform Subcomponent Name: got %s, want != %s", val, "''")

		}
	})
}

func TestPlatformTransceiverState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	cliHandle := dut.RawAPIs().CLI(t)
	resp, err := cliHandle.SendCommand(context.Background(), "show version")
	t.Logf(resp)
	if err != nil {
		t.Error(err)
	}
	if strings.Contains(resp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}

	t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.OpticsModule).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform OpticsModule SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.OpticsModule).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform OpticsModule HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.OpticsModule).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform OpticsModule  OperStatus: got %s, want > %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.OpticsModule).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Optics") {
			t.Errorf("Platform OpticsModule Description: got %s, should contain %s", val, "Optics")

		}
	})
	t.Run("Subscribe//components/component/state/type", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.OpticsModule).Type()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER {
			t.Errorf("Platform OpticsModule  OperStatus: got %s, want > %s", val, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER)

		}
	})

}

func TestSubComponentSwmodule(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe///components/component/software-module/state/oc-sw-module:module-type", func(t *testing.T) {
		components := dut.Telemetry().ComponentAny().Name().Get(t)
		regexpPattern := ".*xr-8000-qos-ea.*"
		r, _ := regexp.Compile(regexpPattern)
		var s1 []string
		for _, c := range components {
			if len(r.FindString(c)) > 0 {
				s1 = append(s1, c)
			}
		}
		s2 := "IOSXR-PKG/2 " + s1[0]
		state := dut.Telemetry().Component(s2).SoftwareModule().ModuleType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)

		if val != oc.PlatformSoftware_SOFTWARE_MODULE_TYPE_USERSPACE_PACKAGE {
			t.Errorf("Platform ModuleType: got %s,want %s", val, oc.PlatformSoftware_SOFTWARE_MODULE_TYPE_USERSPACE_PACKAGE)

		}
	})
}

func TestSubComponentSwmoduleWildCard(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe///components/component/software-module/state/oc-sw-module:module-type", func(t *testing.T) {
		state := dut.Telemetry().Component("IOSXR-PKG/2 xr-8000-qos-ea.*").SoftwareModule().ModuleType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformSoftware_SOFTWARE_MODULE_TYPE_USERSPACE_PACKAGE {
			t.Errorf("Platform ModuleType: got %s,want %s", val, oc.PlatformSoftware_SOFTWARE_MODULE_TYPE_USERSPACE_PACKAGE)

		}
	})
}

func TestSubComponentSwmoduleStream(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe///components/component/software-module/state/oc-sw-module:module-type", func(t *testing.T) {
		components := dut.Telemetry().ComponentAny().Name().Get(t)
		regexpPattern := ".*xr-8000-qos-ea.*"
		r, _ := regexp.Compile(regexpPattern)
		var s1 []string
		for _, c := range components {
			if len(r.FindString(c)) > 0 {
				s1 = append(s1, c)
			}
		}
		s2 := "IOSXR-PKG/2 " + s1[0]
		state := dut.Telemetry().Component(s2).SoftwareModule().ModuleType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		got := state.Collect(t, 35*time.Second).Await(t)
		time.Sleep(35 * time.Second)
		t.Logf("Collected samples: %v", got)
		gotEntries := len(got)
		if got[gotEntries-1].Val(t) != oc.PlatformSoftware_SOFTWARE_MODULE_TYPE_USERSPACE_PACKAGE {
			t.Errorf("Platform ModuleType: got %s, want %s", got[gotEntries-1].Val(t), oc.PlatformSoftware_SOFTWARE_MODULE_TYPE_USERSPACE_PACKAGE)
		}
	})
}

func TestPlatformFabricCard(t *testing.T) {
	dut := ondatra.DUT(t, device1)

	t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform  HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform  OperStatus: got %s, want %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Fabric Card") {
			t.Errorf("Platform Description: got %s, should contain Fabric Card", val)

		}
	})
	t.Run("Subscribe//components/component/state/name", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).Name()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != Platform.FabricCard {
			t.Errorf("Platform Name: got %s, should contain %s", val, Platform.FabricCard)

		}
	})
	t.Run("Subscribe//components/component/state/type", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).Type()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FRU {
			t.Errorf("Platform Type: got %s, want %s", val, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FRU)

		}
	})
	t.Run("Subscribe//components/component/state/id", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).Id()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform Id: got %s, want not null string", val)

		}
	})
	t.Run("Subscribe//components/component/state/location", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).Location()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != Platform.FabricCard {
			t.Errorf("Platform Location: got %s, want %s ", val, Platform.FabricCard)

		}
	})
	t.Run("Subscribe//components/component/state/mfg-name", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).MfgName()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Cisco") {
			t.Errorf("Platform Mfg-name: got %s, want Should contain Cisco", val)

		}
	})
	t.Run("Subscribe//components/component/state/part-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).PartNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform Part-no: got %s, want not null string", val)

		}
	})
	t.Run("Subscribe//components/component/state/removable", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).Removable()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != true {
			t.Errorf("Platform removable: got %v, want %v", val, true)

		}
	})
	t.Run("Subscribe//components/component/state/empty", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).Empty()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != false {
			t.Errorf("Platform Empty: got %v, want %v", val, false)

		}
	})
	t.Run("Subscribe//components/component/state/parent", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).Parent()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Fabric Card") {
			t.Errorf("Platform Parent: got %v, want Contain Fabric Card", val)

		}
	})
	t.Run("Subscribe//components/component/state/allocated-power", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.FabricCard).AllocatedPower()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == uint32(0) {
			t.Errorf("Platform allocated-power: got %v, want not non-zero value", val)

		}
	})

}

func TestPlatformLC(t *testing.T) {
	dut := ondatra.DUT(t, device1)

	t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform  HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform  OperStatus: got %s, want %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Line Card") {
			t.Errorf("Platform Description: got %s, should contain Line Card", val)

		}
	})
	t.Run("Subscribe//components/component/state/name", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).Name()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != Platform.Linecard {
			t.Errorf("Platform Name: got %s, should contain %s", val, Platform.Linecard)

		}
	})
	t.Run("Subscribe//components/component/state/type", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).Type()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD {
			t.Errorf("Platform Type: got %s, want %s", val, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)

		}
	})
	t.Run("Subscribe//components/component/state/id", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).Id()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform Id: got %s, want not null string", val)

		}
	})
	t.Run("Subscribe//components/component/state/location", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).Location()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != Platform.Linecard {
			t.Errorf("Platform Location: got %s, want %s ", val, Platform.Linecard)

		}
	})
	t.Run("Subscribe//components/component/state/mfg-name", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).MfgName()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Cisco") {
			t.Errorf("Platform Mfg-name: got %s, want Should contain Cisco", val)

		}
	})
	t.Run("Subscribe//components/component/state/part-no", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).PartNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == "" {
			t.Errorf("Platform Part-no: got %s, want not null string", val)

		}
	})
	t.Run("Subscribe//components/component/state/removable", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).Removable()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != true {
			t.Errorf("Platform removable: got %v, want %v", val, true)

		}
	})
	t.Run("Subscribe//components/component/state/empty", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).Empty()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val != false {
			t.Errorf("Platform Empty: got %v, want %v", val, false)

		}
	})
	t.Run("Subscribe//components/component/state/parent", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).Parent()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if !strings.Contains(val, "Line Card") {
			t.Errorf("Platform Parent: got %v, want Contain Line Card", val)

		}
	})
	t.Run("Subscribe//components/component/state/allocated-power", func(t *testing.T) {
		state := dut.Telemetry().Component(Platform.Linecard).AllocatedPower()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		if val == uint32(0) {
			t.Errorf("Platform allocated-power: got %v, want not non-zero value", val)

		}
	})

}

func TestPlatformBreakoutConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	configContainer := &oc.Component_Port_BreakoutMode_Group{
		Index:         ygot.Uint8(1),
		NumBreakouts:  ygot.Uint8(4),
		BreakoutSpeed: oc.E_IfEthernet_ETHERNET_SPEED(oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB),
	}
	groupContainer := &oc.Component_Port_BreakoutMode{Group: map[uint8]*oc.Component_Port_BreakoutMode_Group{1: configContainer}}
	breakoutContainer := &oc.Component_Port{BreakoutMode: groupContainer}
	portContainer := &oc.Component{Port: breakoutContainer}

	t.Run("Update//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]/config", func(t *testing.T) {
		path := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1)
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, configContainer)
	})

	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]", func(t *testing.T) {
		state := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1)
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		groupDetails := state.Get(t)
		index := *groupDetails.Index
		numBreakouts := *groupDetails.NumBreakouts
		breakoutSpeed := groupDetails.BreakoutSpeed
		verifyBreakout(index, numBreakouts, breakoutSpeed.String(), t)
	})

	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]/config/index", func(t *testing.T) {
		state := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1).Index()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		index := state.Get(t)
		if index != uint8(1) {
			t.Errorf("Number of Index does not match configured value : got %v, want 1", index)
		}
	})
	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]/config/num-breakouts", func(t *testing.T) {
		state := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1).NumBreakouts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		numBreakouts := state.Get(t)
		if numBreakouts != uint8(4) {
			t.Errorf("Number of breakouts does not match configured value : got %v, want 4", numBreakouts)
		}
	})
	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]/config/breakout-speed", func(t *testing.T) {
		state := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1).BreakoutSpeed()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		breakoutSpeed := state.Get(t).String()
		if breakoutSpeed != "SPEED_10GB" {
			t.Errorf("Breakout-Speed does not match configured value : got %v, want 10GB", breakoutSpeed)
		}
	})

	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]/config/num-physical-channels", func(t *testing.T) {
		state := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1).NumPhysicalChannels()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		numPhysicalChannels := state.Get(t)
		if numPhysicalChannels != uint8(1) {
			t.Errorf("Number physical channels does not match configured value : got %v, want 1", numPhysicalChannels)
		}
	})

	t.Run("Delete//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]/config", func(t *testing.T) {
		path := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1)
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Delete(t)
		got := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1).Index().Get(t)
		if got == *ygot.Uint8(1) {
			t.Error("Delete has not been successfull on config container level")
		}
	})

	t.Run("Update//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]", func(t *testing.T) {
		path := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, groupContainer)
	})
	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode", func(t *testing.T) {
		state := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		breakoutDetails := state.Get(t)
		index := *breakoutDetails.Group[1].Index
		numBreakouts := *breakoutDetails.Group[1].NumBreakouts
		breakoutSpeed := breakoutDetails.Group[1].BreakoutSpeed
		verifyBreakout(index, numBreakouts, breakoutSpeed.String(), t)
	})
	t.Run("Delete//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]", func(t *testing.T) {
		path := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Delete(t)
		got := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1).Index().Get(t)
		if got == *ygot.Uint8(1) {
			t.Error("Delete has not been successfull on group container level")
		}
	})

	t.Run("Update//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/", func(t *testing.T) {
		path := dut.Config().Component(PlatformSF.Transceiver).Port()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, breakoutContainer)
	})

	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port", func(t *testing.T) {
		state := dut.Config().Component(PlatformSF.Transceiver).Port()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		portDetails := state.Get(t)
		index := *portDetails.BreakoutMode.Group[1].Index
		numBreakouts := *portDetails.BreakoutMode.Group[1].NumBreakouts
		breakoutSpeed := portDetails.BreakoutMode.Group[1].BreakoutSpeed
		verifyBreakout(index, numBreakouts, breakoutSpeed.String(), t)
	})

	t.Run("Delete//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/", func(t *testing.T) {
		path := dut.Config().Component(PlatformSF.Transceiver).Port()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Delete(t)
		got := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1).Index().Get(t)
		if got == *ygot.Uint8(1) {
			t.Error("Delete has not been successfull on breakout-mode container level")
		}
	})

	t.Run("Update//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/", func(t *testing.T) {
		path := dut.Config().Component(PlatformSF.Transceiver)
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, portContainer)
	})
	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/config", func(t *testing.T) {
		state := dut.Config().Component(PlatformSF.Transceiver)
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		componentDetails := state.Get(t)
		index := *componentDetails.Port.BreakoutMode.Group[1].Index
		numBreakouts := *componentDetails.Port.BreakoutMode.Group[1].NumBreakouts
		breakoutSpeed := componentDetails.Port.BreakoutMode.Group[1].BreakoutSpeed
		verifyBreakout(index, numBreakouts, breakoutSpeed.String(), t)
	})
	t.Run("Delete//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/", func(t *testing.T) {
		path := dut.Config().Component(PlatformSF.Transceiver)
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Delete(t)
		got := dut.Config().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1).Index().Get(t)
		if got == *ygot.Uint8(1) {
			t.Error("Delete has not been successfull on port container level")
		}
	})

}

func TestPlatformBreakoutState(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	config.TextWithGNMI(context.Background(), t, dut, "controller optics 0/0/0/20\n breakout 4x10\n")
	defer config.TextWithGNMI(context.Background(), t, dut, "no controller optics 0/0/0/20\n breakout 4x10\n")
	t.Run("Subscribe//components/component[0/0/CPU0-QSFP_DD Optics Port 20]/state", func(t *testing.T) {
		state := dut.Telemetry().Component(PlatformSF.Transceiver)
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := state.Get(t)
		index := *val.Port.BreakoutMode.Group[1].Index
		numBreakouts := *val.Port.BreakoutMode.Group[1].NumBreakouts
		breakoutSpeed := val.Port.BreakoutMode.Group[1].BreakoutSpeed
		verifyBreakout(index, numBreakouts, breakoutSpeed.String(), t)

	})
	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/state/port", func(t *testing.T) {
		state := dut.Telemetry().Component(PlatformSF.Transceiver).Port()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		portDetails := state.Get(t)
		index := *portDetails.BreakoutMode.Group[1].Index
		numBreakouts := *portDetails.BreakoutMode.Group[1].NumBreakouts
		breakoutSpeed := portDetails.BreakoutMode.Group[1].BreakoutSpeed
		verifyBreakout(index, numBreakouts, breakoutSpeed.String(), t)
	})
	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/state/port/breakout-mode", func(t *testing.T) {
		state := dut.Telemetry().Component(PlatformSF.Transceiver).Port().BreakoutMode()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		breakoutDetails := state.Get(t)
		index := *breakoutDetails.Group[1].Index
		numBreakouts := *breakoutDetails.Group[1].NumBreakouts
		breakoutSpeed := breakoutDetails.Group[1].BreakoutSpeed
		verifyBreakout(index, numBreakouts, breakoutSpeed.String(), t)
	})
	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/state/port/breakout-mode/groups", func(t *testing.T) {
		state := dut.Telemetry().Component(PlatformSF.Transceiver).Port().BreakoutMode().GroupAny()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		groupDetails := state.Get(t)
		index := *groupDetails[0].Index
		numBreakouts := *groupDetails[0].NumBreakouts
		breakoutSpeed := groupDetails[0].BreakoutSpeed
		verifyBreakout(index, numBreakouts, breakoutSpeed.String(), t)
	})
	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/state/port/breakout-mode/group[1]", func(t *testing.T) {
		state := dut.Telemetry().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1)
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		groupDetails := state.Get(t)
		index := *groupDetails.Index
		numBreakouts := *groupDetails.NumBreakouts
		breakoutSpeed := groupDetails.BreakoutSpeed
		verifyBreakout(index, numBreakouts, breakoutSpeed.String(), t)
	})

	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]/index", func(t *testing.T) {
		state := dut.Telemetry().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1).Index()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		index := state.Get(t)
		if index != uint8(1) {
			t.Errorf("Number of Index does not match configured value : got %v, want 1", index)
		}
	})
	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]/num-breakouts", func(t *testing.T) {
		state := dut.Telemetry().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1).NumBreakouts()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		numBreakouts := state.Get(t)
		if numBreakouts != uint8(4) {
			t.Errorf("Number of breakouts does not match configured value : got %v, want 4", numBreakouts)
		}
	})
	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]/breakout-speed", func(t *testing.T) {
		state := dut.Telemetry().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1).BreakoutSpeed()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		breakoutSpeed := state.Get(t).String()
		if breakoutSpeed != "SPEED_10GB" {
			t.Errorf("Breakout-Speed does not match configured value : got %v, want 10GB", breakoutSpeed)
		}
	})

	t.Run("Subscribe//component[0/0/CPU0-QSFP_DD Optics Port 20]/config/port/breakout-mode/group[1]/num-physical-channels", func(t *testing.T) {
		state := dut.Telemetry().Component(PlatformSF.Transceiver).Port().BreakoutMode().Group(1).NumPhysicalChannels()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		numPhysicalChannels := state.Get(t)
		if numPhysicalChannels != uint8(1) {
			t.Errorf("Number physical channels does not match configured value : got %v, want 1", numPhysicalChannels)
		}
	})
}
