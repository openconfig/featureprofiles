package basetest

import (
	"flag"
	"testing"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

const (
	RP           = "0/RP0/CPU0"
	linecardType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD
)

var (
	device1  = "dut"
	observer = fptest.NewObserver("Platform").AddCsvRecorder("ocreport").
			AddCsvRecorder("Platform")
	PlatformSF = PlatformInfo{
		Chassis:            "Rack 0",
		Linecard:           "0/0/CPU0",
		OpticsModule:       "HundredGigE0/0/0/9",
		FanTray:            "0/FT0",
		PowerSupply:        "0/PT2-PM0",
		TempSensor:         "0/0/CPU0-TEMP_FET1_DX",
		BiosFirmware:       "0/0/CPU0-Bios",
		Transceiver:        "0/0/CPU0-QSFP_DD Optics Port 20",
		SWVersionComponent: "0/0/CPU0-Broadwell-DE (D-1530)",
		FabricCard:         "0/FC0",
		SubComponent:       "Rack 0-Line Card Slot 0",
		SwPackage:          "IOSXR-PKG/2 xr-8000-qos-ea-7.8.1.14Iv1.0.0-1",
	}
	Platform = PlatformSF
)
var (
	ControllerOptics      = flag.String("controller_optics", "0/0/0/20", "ControllerOptics")
	ControllerOpticsSpeed = flag.String("controller_optics_speed", "4x10", "ControllerOpticsSpeed")
	qspfdString           = flag.String("QSFP_DD_Optics", "-QSFP_DD Optics Port 20", "qspfdString")
)

// to hold platform info
// To do: get this dynamiclly from device
type PlatformInfo struct {
	Chassis            string
	Linecard           string
	OpticsModule       string
	FanTray            string
	PowerSupply        string
	TempSensor         string
	BiosFirmware       string
	Transceiver        string
	SWVersionComponent string
	FabricCard         string
	SubComponent       string
	SwPackage          string
}

var componentName string

func portComponentName(t *testing.T, dut *ondatra.DUTDevice) {

	lcs := components.FindComponentsByType(t, dut, linecardType)
	if got := len(lcs); got == 0 {
		componentName = "0/RP0/CPU0" + *qspfdString
		t.Logf("The choosen component name: %v", componentName)
	} else {
		for _, lc := range lcs {
			componentName = lc + *qspfdString
			t.Logf("The choosen component name: %v", componentName)
			break
		}
	}
}
func verifyBreakout(index uint8, numBreakoutsWant uint8, numBreakoutsGot uint8, breakoutSpeedWant string, breakoutSpeedGot string, t *testing.T) {

	if index != uint8(1) {
		t.Errorf("Index: got %v, want 1", index)
	}
	if numBreakoutsGot != numBreakoutsWant {
		t.Errorf("Number of breakouts configured : got %v, want %v", numBreakoutsGot, numBreakoutsWant)
	}
	if breakoutSpeedGot != breakoutSpeedWant {
		t.Errorf("Breakout speed configured : got %v, want %v", breakoutSpeedGot, breakoutSpeedWant)
	}
}
func verifyDelete(t *testing.T, dut *ondatra.DUTDevice) {
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.GetConfig(t, dut, gnmi.OC().Component(componentName).Port().BreakoutMode().Group(1).Index().Config()) //catch the error  as it is expected and absorb the panic.
	}); errMsg != nil {
		t.Log("Expected failure ")
	} else {
		t.Errorf("This get on empty config should have failed : %s", *errMsg)
	}
}
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
