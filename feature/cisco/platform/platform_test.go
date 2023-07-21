package basetest

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	transceiverType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER
)

func TestPlatformCPUState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe//components/component/cpu/utilization/state/avg", func(t *testing.T) {
		state := gnmi.OC().Component(RP).Cpu().Utilization().Avg()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform CPU Avg value")
		} else {
			t.Errorf("Platform CPU Avg: got %d, want > %d", val, 0)

		}
	})
	t.Run("Subscribe//components/component/cpu/utilization/state/min", func(t *testing.T) {
		state := gnmi.OC().Component(RP).Cpu().Utilization().Min()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform CPU  Min value")
		} else {
			t.Errorf("Platform CPU  Min: got %d, want >%d", val, 0)

		}
	})
	t.Run("Subscribe//components/component/cpu/utilization/state/max", func(t *testing.T) {
		state := gnmi.OC().Component(RP).Cpu().Utilization().Max()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform  CPU Max value")
		} else {
			t.Errorf("Platform  CPU Max: got %d, want >%d", val, 0)

		}
	})
	t.Run("Subscribe//components/component/cpu/utilization/state/instant", func(t *testing.T) {
		state := gnmi.OC().Component(RP).Cpu().Utilization().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform  CPU Instant value")
		} else {
			t.Errorf("Platform  CPU Instant: got %d, want >%d", val, 0)

		}
	})
	t.Run("Subscribe//components/component/cpu/utilization/state/max-time", func(t *testing.T) {
		state := gnmi.OC().Component(RP).Cpu().Utilization().MaxTime()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform  CPU MaxTime value")
		} else {
			t.Errorf("Platform  CPU MaxTime: got %d, want >%d", val, 0)

		}
	})
	t.Run("Subscribe//components/component/cpu/utilization/state/min-time", func(t *testing.T) {
		state := gnmi.OC().Component(RP).Cpu().Utilization().MinTime()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == 0 || val > 0 {
			t.Logf("Got correct Platform  CPU MinTime value")
		} else {
			t.Errorf("Platform  CPU MinTime: got %d, want >%d", val, 0)

		}
	})
	t.Run("Subscribe//components/component/cpu/utilization/state/Interval", func(t *testing.T) {
		state := gnmi.OC().Component(RP).Cpu().Utilization().Interval()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
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
		state := gnmi.OC().Component(Platform.FanTray).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform FanTray SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FanTray).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform FanTray HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FanTray).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform FanTray  OperStatus: got %s, want > %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FanTray).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if !strings.Contains(val, "Fan") {
			t.Errorf("Platform FanTray Description: got %s, should contain %s", val, "Fan")

		}
	})

}

func TestPlatformChassisState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Chassis).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform Chassis SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Chassis).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform Chassis HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Chassis).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform Chassis  OperStatus: got %s, want > %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Chassis).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if !strings.Contains(val, "Chassis") {
			t.Errorf("Platform Chassis Description: got %s, should contain %s", val, "Chassis")

		}
	})

}
func TestPlatformPSUState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.PowerSupply).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform PowerSupply SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.PowerSupply).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform PowerSupply HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.PowerSupply).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform PowerSupply  OperStatus: got %s, want > %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.PowerSupply).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
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
	t.Skip()
	// Failure due to CSCwb72703
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe//components/component/transceiver/state/form-factor", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Transceiver).Transceiver().FormFactor()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_SFP {
			t.Errorf("Platform Transceiverchannel FormFactor: got %s, want != %s", val, oc.TransportTypes_TRANSCEIVER_FORM_FACTOR_TYPE_SFP)

		}
	})
	t.Run("Subscribe//components/component/transceiver/physical-channels/channel/state/input-power/instant", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Transceiver).Transceiver().Channel(1).InputPower().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != float64(0) {
			t.Errorf("Platform Transceiverchannel Channel InputPower Instant: got %v, want != %v", val, float64(0))

		}
	})
	t.Run("Subscribe//components/component/transceiver/physical-channels/channel/state/output-power/instant", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Transceiver).Transceiver().Channel(1).OutputPower().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != float64(0) {
			t.Errorf("Platform Transceiverchannel Channel OutputPower Instant: got %v, want != %v", val, float64(0))

		}
	})
	t.Run("Subscribe//components/component/transceiver/physical-channels/channel/state/laser-bias-current/instant", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Transceiver).Transceiver().Channel(1).LaserBiasCurrent().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != float64(0) {
			t.Errorf("Platform Transceiverchannel Channel LaserBiasCurrent Instant: got %v, want != %v", val, float64(0))

		}
	})

}

func TestTempSensor(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe//components/component/state/temperature/instant", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.TempSensor).Temperature().Instant()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == float64(0) {
			t.Errorf("Platform Temperature Instant: got %v, want != %v", val, float64(0))

		}
	})
}

func TestFirmware(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe//components/component/state/firmware-version", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.BiosFirmware).FirmwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform FirmwareVersion: got %s, want != %s", val, "''")

		}
	})
}

func TestSWVersion(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe//components/component/state/software-version", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.SWVersionComponent).SoftwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform SoftwareVersion : got %s, want != %s", val, "''")

		}
	})
}

func TestFabric(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform Fabric SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if !strings.Contains(val, "Fabric") {
			t.Errorf("Platform Fabric Description: got %s, should contain %s", val, "Fabric")

		}
	})
}

func TestSubComponent(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe//components/component/subcomponents/subcomponent/state/name", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Chassis).Subcomponent(Platform.SubComponent).Name()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform Subcomponent Name: got %s, want != %s", val, "''")

		}
	})
}

func TestPlatformTransceiverState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	cards := components.FindComponentsByType(t, dut, transceiverType)
	t.Logf("Found Card list: %v", cards)

	for _, card := range cards {
		t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
			state := gnmi.OC().Component(card).SerialNo()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if val == "" {
				t.Errorf("Platform OpticsModule SerialNo: got %s, want != %s", val, "''")

			}
		})
		t.Run("Subscribe//components/component/state/oper-status/hardware-version", func(t *testing.T) {
			state := gnmi.OC().Component(card).HardwareVersion()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if val == "" {
				t.Errorf("Platform OpticsModule HardwareVersion: got %s, want != %s", val, "''")

			}
		})
		t.Run("Subscribe//components/component/state/oper-status", func(t *testing.T) {
			state := gnmi.OC().Component(card).OperStatus()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
				t.Errorf("Platform OpticsModule  OperStatus: got %s, want > %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

			}
		})
		t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
			state := gnmi.OC().Component(card).Description()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if !strings.Contains(val, "Optics") {
				t.Errorf("Platform OpticsModule Description: got %s, should contain %s", val, "Optics")

			}
		})
		t.Run("Subscribe//components/component/state/type", func(t *testing.T) {
			state := gnmi.OC().Component(card).Type()
			defer observer.RecordYgot(t, "SUBSCRIBE", state)
			val := gnmi.Get(t, dut, state.State())
			if val != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER {
				t.Errorf("Platform OpticsModule  OperStatus: got %s, want > %s", val, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_TRANSCEIVER)

			}
		})
	}

}

func TestSubComponentSwmodule(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe///components/component/software-module/state/oc-sw-module:module-type", func(t *testing.T) {
		components := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().Name().State())
		regexpPattern := ".*xr-8000-qos-ea.*"
		r, _ := regexp.Compile(regexpPattern)
		var s1 []string
		for _, c := range components {
			if len(r.FindString(c)) > 0 {
				s1 = append(s1, c)
			}
		}
		s2 := "IOSXR-PKG/2 " + s1[0]
		state := gnmi.OC().Component(s2).SoftwareModule().ModuleType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())

		if val != oc.PlatformSoftware_SOFTWARE_MODULE_TYPE_USERSPACE_PACKAGE {
			t.Errorf("Platform ModuleType: got %s,want %s", val, oc.PlatformSoftware_SOFTWARE_MODULE_TYPE_USERSPACE_PACKAGE)

		}
	})
}

func TestSubComponentSwmoduleWildCard(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe///components/component/software-module/state/oc-sw-module:module-type", func(t *testing.T) {
		state := gnmi.OC().Component("IOSXR-PKG/2 xr-8000-qos-ea.*").SoftwareModule().ModuleType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.PlatformSoftware_SOFTWARE_MODULE_TYPE_USERSPACE_PACKAGE {
			t.Errorf("Platform ModuleType: got %s,want %s", val, oc.PlatformSoftware_SOFTWARE_MODULE_TYPE_USERSPACE_PACKAGE)

		}
	})
}

func TestSubComponentSwmoduleStream(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	t.Run("Subscribe///components/component/software-module/state/oc-sw-module:module-type", func(t *testing.T) {
		components := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().Name().State())
		regexpPattern := ".*xr-8000-qos-ea.*"
		r, _ := regexp.Compile(regexpPattern)
		var s1 []string
		for _, c := range components {
			if len(r.FindString(c)) > 0 {
				s1 = append(s1, c)
			}
		}
		s2 := "IOSXR-PKG/2 " + s1[0]
		state := gnmi.OC().Component(s2).SoftwareModule().ModuleType()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		got := gnmi.Collect(t, dut, state.State(), 35*time.Second).Await(t)
		time.Sleep(35 * time.Second)
		t.Logf("Collected samples: %v", got)
		gotEntries := len(got)
		value, _ := got[gotEntries-1].Val()
		if value != oc.PlatformSoftware_SOFTWARE_MODULE_TYPE_USERSPACE_PACKAGE {
			t.Errorf("Platform ModuleType: got %s, want %s", value, oc.PlatformSoftware_SOFTWARE_MODULE_TYPE_USERSPACE_PACKAGE)
		}
	})
}

func TestPlatformFabricCard(t *testing.T) {
	dut := ondatra.DUT(t, device1)

	t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform  HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform  OperStatus: got %s, want %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if !strings.Contains(val, "Fabric Card") {
			t.Errorf("Platform Description: got %s, should contain Fabric Card", val)

		}
	})
	t.Run("Subscribe//components/component/state/name", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).Name()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != Platform.FabricCard {
			t.Errorf("Platform Name: got %s, should contain %s", val, Platform.FabricCard)

		}
	})
	t.Run("Subscribe//components/component/state/type", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).Type()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC {
			t.Errorf("Platform Type: got %s, want %s", val, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_FABRIC)

		}
	})
	t.Run("Subscribe//components/component/state/id", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).Id()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform Id: got %s, want not null string", val)

		}
	})
	t.Run("Subscribe//components/component/state/location", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).Location()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != Platform.FabricCard {
			t.Errorf("Platform Location: got %s, want %s ", val, Platform.FabricCard)

		}
	})
	t.Run("Subscribe//components/component/state/mfg-name", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).MfgName()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if !strings.Contains(val, "Cisco") {
			t.Errorf("Platform Mfg-name: got %s, want Should contain Cisco", val)

		}
	})
	t.Run("Subscribe//components/component/state/part-no", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).PartNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform Part-no: got %s, want not null string", val)

		}
	})
	t.Run("Subscribe//components/component/state/removable", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).Removable()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != true {
			t.Errorf("Platform removable: got %v, want %v", val, true)

		}
	})
	t.Run("Subscribe//components/component/state/empty", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).Empty()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("Platform Empty: got %v, want %v", val, false)

		}
	})
	t.Run("Subscribe//components/component/state/parent", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).Parent()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if !strings.Contains(val, "Fabric Card") {
			t.Errorf("Platform Parent: got %v, want Contain Fabric Card", val)

		}
	})
	t.Run("Subscribe//components/component/state/allocated-power", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.FabricCard).AllocatedPower()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == uint32(0) {
			t.Errorf("Platform allocated-power: got %v, want not non-zero value", val)

		}
	})

}

func TestPlatformLC(t *testing.T) {
	dut := ondatra.DUT(t, device1)

	t.Run("Subscribe//components/component/state/serial-no", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).SerialNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform SerialNo: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status/hardware-version", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).HardwareVersion()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform  HardwareVersion: got %s, want != %s", val, "''")

		}
	})
	t.Run("Subscribe//components/component/state/oper-status", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).OperStatus()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
			t.Errorf("Platform  OperStatus: got %s, want %s", val, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)

		}
	})
	t.Run("Subscribe//components/component/state/description", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).Description()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if !strings.Contains(val, "Line Card") {
			t.Errorf("Platform Description: got %s, should contain Line Card", val)

		}
	})
	t.Run("Subscribe//components/component/state/name", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).Name()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != Platform.Linecard {
			t.Errorf("Platform Name: got %s, should contain %s", val, Platform.Linecard)

		}
	})
	t.Run("Subscribe//components/component/state/type", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).Type()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD {
			t.Errorf("Platform Type: got %s, want %s", val, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD)

		}
	})
	t.Run("Subscribe//components/component/state/id", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).Id()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform Id: got %s, want not null string", val)

		}
	})
	t.Run("Subscribe//components/component/state/location", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).Location()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != Platform.Linecard {
			t.Errorf("Platform Location: got %s, want %s ", val, Platform.Linecard)

		}
	})
	t.Run("Subscribe//components/component/state/mfg-name", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).MfgName()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if !strings.Contains(val, "Cisco") {
			t.Errorf("Platform Mfg-name: got %s, want Should contain Cisco", val)

		}
	})
	t.Run("Subscribe//components/component/state/part-no", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).PartNo()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == "" {
			t.Errorf("Platform Part-no: got %s, want not null string", val)

		}
	})
	t.Run("Subscribe//components/component/state/removable", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).Removable()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != true {
			t.Errorf("Platform removable: got %v, want %v", val, true)

		}
	})
	t.Run("Subscribe//components/component/state/empty", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).Empty()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val != false {
			t.Errorf("Platform Empty: got %v, want %v", val, false)

		}
	})
	t.Run("Subscribe//components/component/state/parent", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).Parent()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if !strings.Contains(val, "Line Card") {
			t.Errorf("Platform Parent: got %v, want Contain Line Card", val)

		}
	})
	t.Run("Subscribe//components/component/state/allocated-power", func(t *testing.T) {
		state := gnmi.OC().Component(Platform.Linecard).AllocatedPower()
		defer observer.RecordYgot(t, "SUBSCRIBE", state)
		val := gnmi.Get(t, dut, state.State())
		if val == uint32(0) {
			t.Errorf("Platform allocated-power: got %v, want not non-zero value", val)

		}
	})

}

func TestPlatformBreakoutConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	portName(t, dut)
	port1 := strings.Replace(dut.Port(t, "port1").Name(), "FourHundredGigE", "Port", 1)
	if count := strings.Count(port1, "/"); count == 4 {
		port1 = port1[:strings.LastIndex(port1, "/")]
	}
	componentNameList = []string{port1}
	if hundredGigEComponentName != "" {
		port2 := strings.Replace(hundredGigE, "HundredGigE", "Port", 1)
		if count := strings.Count(port2, "/"); count == 4 {
			port2 = port1[:strings.LastIndex(port2, "/")]
		}
		componentNameList = append(componentNameList, port2)
	}

	for _, element := range componentNameList {
		fmt.Print(element)
	}

	cases := []struct {
		numbreakouts  uint8
		breakoutspeed oc.E_IfEthernet_ETHERNET_SPEED
	}{
		{
			numbreakouts:  4,
			breakoutspeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_10GB,
		},
		{
			numbreakouts:  4,
			breakoutspeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
		},
		{
			numbreakouts:  3,
			breakoutspeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
		},
		{
			numbreakouts:  2,
			breakoutspeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
		},
		{
			numbreakouts:  1,
			breakoutspeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB,
		},
		{
			numbreakouts:  4,
			breakoutspeed: oc.IfEthernet_ETHERNET_SPEED_SPEED_25GB,
		},
	}
	for _, tc := range cases {
		for _, componentName := range componentNameList {

			configContainer := &oc.Component_Port_BreakoutMode_Group{
				Index:         ygot.Uint8(0),
				NumBreakouts:  ygot.Uint8(tc.numbreakouts),
				BreakoutSpeed: oc.E_IfEthernet_ETHERNET_SPEED(tc.breakoutspeed),
			}
			groupContainer := &oc.Component_Port_BreakoutMode{Group: map[uint8]*oc.Component_Port_BreakoutMode_Group{1: configContainer}}
			breakoutContainer := &oc.Component_Port{BreakoutMode: groupContainer}
			portContainer := &oc.Component{Port: breakoutContainer, Name: ygot.String(componentName)}
			fmt.Printf("COMBO : %v*%v ", tc.numbreakouts, tc.breakoutspeed)

			gnmi.Delete(t, dut, gnmi.OC().Component(componentName).Port().BreakoutMode().Group(1).Config())
			t.Run(fmt.Sprintf("Update//component[%v]/config/port/breakout-mode/group[0]/config", componentName), func(t *testing.T) {
				fmt.Printf("The component name inside test: %v", componentName)
				path := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(0)
				defer observer.RecordYgot(t, "UPDATE", path)
				gnmi.Update(t, dut, path.Config(), configContainer)
			})

			t.Run(fmt.Sprintf("Subscribe//component[%v]/config/port/breakout-mode/group[0]", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(0)
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				groupDetails := gnmi.GetConfig(t, dut, state.Config())
				index := *groupDetails.Index
				numBreakouts := *groupDetails.NumBreakouts
				breakoutSpeed := groupDetails.BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)
			})

			t.Run(fmt.Sprintf("Subscribe//component[%v]/state/port/breakout-mode/group[0]", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(0)
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				groupDetails := gnmi.Get(t, dut, state.State())
				index := *groupDetails.Index
				numBreakouts := *groupDetails.NumBreakouts
				breakoutSpeed := groupDetails.BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)
			})

			t.Run(fmt.Sprintf("Subscribe//component[%v]/config/port/breakout-mode/group[0]/config/index", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(0).Index()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				index := gnmi.GetConfig(t, dut, state.Config())
				if index != uint8(0) {
					t.Errorf("Number of Index does not match configured value : got %v, want 0", index)
				}
			})

			t.Run(fmt.Sprintf("Subscribe//component[%v]/state/port/breakout-mode/group[0]/config/index", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(0).Index()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				index := gnmi.Get(t, dut, state.State())
				if index != uint8(0) {
					t.Errorf("Number of Index does not match configured value : got %v, want 0", index)
				}
			})

			t.Run(fmt.Sprintf("Subscribe//component[%v]/config/port/breakout-mode/group[0]/config/num-breakouts", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(0).NumBreakouts()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				numBreakouts := gnmi.GetConfig(t, dut, state.Config())
				if numBreakouts != tc.numbreakouts {
					t.Errorf("Number of breakouts does not match configured value : got %v, want %v", numBreakouts, tc.numbreakouts)
				}
			})
			t.Run(fmt.Sprintf("Subscribe//component[%v]/state/port/breakout-mode/group[0]/config/num-breakouts", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(0).NumBreakouts()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				numBreakouts := gnmi.Get(t, dut, state.State())
				if numBreakouts != tc.numbreakouts {
					t.Errorf("Number of breakouts does not match configured value : got %v, want %v", numBreakouts, tc.numbreakouts)
				}
			})
			t.Run(fmt.Sprintf("Subscribe//component[%v]/config/port/breakout-mode/group[0]/config/breakout-speed", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(0).BreakoutSpeed()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				breakoutSpeed := gnmi.GetConfig(t, dut, state.Config()).String()
				if breakoutSpeed != tc.breakoutspeed.String() {
					t.Errorf("Breakout-Speed does not match configured value : got %v, want %v", breakoutSpeed, tc.breakoutspeed.String())
				}
			})

			t.Run(fmt.Sprintf("Subscribe//component[%v]/state/port/breakout-mode/group[0]/config/breakout-speed", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(0).BreakoutSpeed()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				breakoutSpeed := gnmi.Get(t, dut, state.State()).String()
				if breakoutSpeed != tc.breakoutspeed.String() {
					t.Errorf("Breakout-Speed does not match configured value : got %v, want %v", breakoutSpeed, tc.breakoutspeed.String())
				}
			})

			t.Run(fmt.Sprintf("Delete//component[%v]/config/port/breakout-mode/group[0]/config", componentName), func(t *testing.T) {
				path := gnmi.OC().Component(componentName).Port().BreakoutMode().Group(0)
				defer observer.RecordYgot(t, "UPDATE", path)
				gnmi.Delete(t, dut, path.Config())
				verifyDelete(t, dut, componentName)
			})

			t.Run(fmt.Sprintf("Update//component[%v]/config/port/breakout-mode/group[0]", componentName), func(t *testing.T) {
				path := gnmi.OC().Component(componentName).Port().BreakoutMode()
				defer observer.RecordYgot(t, "UPDATE", path)
				gnmi.Update(t, dut, path.Config(), groupContainer)
			})
			t.Run(fmt.Sprintf("Subscribe//component[%v]/config/port/breakout-mode", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				breakoutDetails := gnmi.GetConfig(t, dut, state.Config())
				index := *breakoutDetails.GetGroup(0).Index
				numBreakouts := *breakoutDetails.GetGroup(0).NumBreakouts
				breakoutSpeed := breakoutDetails.GetGroup(0).BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)
			})
			t.Run(fmt.Sprintf("Subscribe//component[%v]/state/port/breakout-mode", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				breakoutDetails := gnmi.Get(t, dut, state.State())
				index := *breakoutDetails.GetGroup(0).Index
				numBreakouts := *breakoutDetails.GetGroup(0).NumBreakouts
				breakoutSpeed := breakoutDetails.GetGroup(0).BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)
			})

			t.Run(fmt.Sprintf("Replace//component[%v]/config/port/breakout-mode/group[0]", componentName), func(t *testing.T) {
				path := gnmi.OC().Component(componentName).Port().BreakoutMode()
				defer observer.RecordYgot(t, "UPDATE", path)
				gnmi.Replace(t, dut, path.Config(), groupContainer)
			})
			t.Run(fmt.Sprintf("Subscribe//component[%v]/config/port/breakout-mode", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				breakoutDetails := gnmi.GetConfig(t, dut, state.Config())
				index := *breakoutDetails.GetGroup(0).Index
				numBreakouts := *breakoutDetails.GetGroup(0).NumBreakouts
				breakoutSpeed := breakoutDetails.GetGroup(0).BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)
			})
			t.Run(fmt.Sprintf("Subscribe//component[%v]/state/port/breakout-mode", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port().BreakoutMode()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				breakoutDetails := gnmi.Get(t, dut, state.State())
				index := *breakoutDetails.GetGroup(0).Index
				numBreakouts := *breakoutDetails.GetGroup(0).NumBreakouts
				breakoutSpeed := breakoutDetails.GetGroup(0).BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)
			})
			t.Run(fmt.Sprintf("Delete//component[%v]/config/port/breakout-mode/group[0]", componentName), func(t *testing.T) {
				path := gnmi.OC().Component(componentName).Port().BreakoutMode()
				defer observer.RecordYgot(t, "UPDATE", path)
				gnmi.Delete(t, dut, path.Config())
				verifyDelete(t, dut, componentName)
			})

			t.Run(fmt.Sprintf("Update//component[%v]/config/port/breakout-mode/", componentName), func(t *testing.T) {
				path := gnmi.OC().Component(componentName).Port()
				defer observer.RecordYgot(t, "UPDATE", path)
				gnmi.Update(t, dut, path.Config(), breakoutContainer)
			})
			t.Run(fmt.Sprintf("Subscribe//component[%v]/config/port", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				portDetails := gnmi.GetConfig(t, dut, state.Config())
				index := *portDetails.BreakoutMode.GetGroup(0).Index
				numBreakouts := *portDetails.BreakoutMode.GetGroup(0).NumBreakouts
				breakoutSpeed := portDetails.BreakoutMode.GetGroup(0).BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)
			})
			t.Run(fmt.Sprintf("Subscribe//component[%v]/state/port", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				portDetails := gnmi.Get(t, dut, state.State())
				index := *portDetails.BreakoutMode.GetGroup(0).Index
				numBreakouts := *portDetails.BreakoutMode.GetGroup(0).NumBreakouts
				breakoutSpeed := portDetails.BreakoutMode.GetGroup(0).BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)
			})
			t.Run(fmt.Sprintf("Replace//component[%v]/config/port/breakout-mode/", componentName), func(t *testing.T) {
				path := gnmi.OC().Component(componentName).Port()
				defer observer.RecordYgot(t, "UPDATE", path)
				gnmi.Replace(t, dut, path.Config(), breakoutContainer)
			})
			t.Run(fmt.Sprintf("Subscribe//component[%v]/config/port", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				portDetails := gnmi.GetConfig(t, dut, state.Config())
				index := *portDetails.BreakoutMode.GetGroup(0).Index
				numBreakouts := *portDetails.BreakoutMode.GetGroup(0).NumBreakouts
				breakoutSpeed := portDetails.BreakoutMode.GetGroup(0).BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)
			})
			t.Run(fmt.Sprintf("Subscribe//component[%v]/state/port", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName).Port()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				portDetails := gnmi.Get(t, dut, state.State())
				index := *portDetails.BreakoutMode.GetGroup(0).Index
				numBreakouts := *portDetails.BreakoutMode.GetGroup(0).NumBreakouts
				breakoutSpeed := portDetails.BreakoutMode.GetGroup(0).BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)
			})

			t.Run(fmt.Sprintf("Delete//component[%v]/config/port/breakout-mode/", componentName), func(t *testing.T) {
				path := gnmi.OC().Component(componentName).Port()
				defer observer.RecordYgot(t, "UPDATE", path)
				gnmi.Delete(t, dut, path.Config())
				verifyDelete(t, dut, componentName)
			})

			t.Run(fmt.Sprintf("Update//component[%v]/config/port/", componentName), func(t *testing.T) {
				path := gnmi.OC().Component(componentName)
				defer observer.RecordYgot(t, "UPDATE", path)
				gnmi.Update(t, dut, path.Config(), portContainer)
			})

			t.Run(fmt.Sprintf("Subscribe//component[%v]/config", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName)
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				componentDetails := gnmi.GetConfig(t, dut, state.Config())
				index := *componentDetails.Port.BreakoutMode.GetGroup(0).Index
				numBreakouts := *componentDetails.Port.BreakoutMode.GetGroup(0).NumBreakouts
				breakoutSpeed := componentDetails.Port.BreakoutMode.GetGroup(0).BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)

			})

			t.Run(fmt.Sprintf("Subscribe//component[%v]/state", componentName), func(t *testing.T) {
				state := gnmi.OC().Component(componentName)
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				componentDetails := gnmi.Get(t, dut, state.State())
				index := *componentDetails.Port.BreakoutMode.GetGroup(0).Index
				numBreakouts := *componentDetails.Port.BreakoutMode.GetGroup(0).NumBreakouts
				breakoutSpeed := componentDetails.Port.BreakoutMode.GetGroup(0).BreakoutSpeed
				verifyBreakout(index, tc.numbreakouts, numBreakouts, tc.breakoutspeed.String(), breakoutSpeed.String(), t)
			})
			t.Run(fmt.Sprintf("Replace//component[%v]/config/port/", componentName), func(t *testing.T) {
				path := gnmi.OC().Component(componentName)
				defer observer.RecordYgot(t, "UPDATE", path)
				gnmi.Replace(t, dut, path.Config(), portContainer)
			})

			t.Run(fmt.Sprintf("Delete//component[%v]/config/port/breakout-mode/", componentName), func(t *testing.T) {
				path := gnmi.OC().Component(componentName)
				defer observer.RecordYgot(t, "UPDATE", path)
				gnmi.Delete(t, dut, path.Config())
				verifyDelete(t, dut, componentName)
			})

		}
	}

}
