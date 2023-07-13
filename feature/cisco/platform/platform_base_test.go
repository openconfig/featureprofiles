package basetest

import (
	"fmt"
	"strings"
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

var componentNameList []string
var qsfpType string
var hundredGigE string
var hundredGigEComponentName string
var fourhundredGigEComponentName string

func qsfptype(t *testing.T, dut *ondatra.DUTDevice, intf string) string {
	qsfptype := strings.Fields(gnmi.Lookup(t, dut, gnmi.OC().Component(intf).Description().State()).String())[1]
	if strings.Contains(qsfptype, "28") {
		qsfpType = "-QSFP28 Optics Port " + strings.Split(intf, "/")[3]
	} else {
		qsfpType = "-QSFP_DD Optics Port " + strings.Split(intf, "/")[3]
	}
	return qsfpType
}
func checkHundredGigE(t *testing.T, dut *ondatra.DUTDevice) {
	intfs := gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().Name().State())
	for _, intf := range intfs {
		intfName, _ := gnmi.Lookup(t, dut, gnmi.OC().Interface(intf).Name().State()).Val()
		if strings.HasPrefix(intfName, "HundredGigE") {
			if strings.Contains(gnmi.Lookup(t, dut, gnmi.OC().Component(intf).Description().State()).String(), "QSFP") {
				hundredGigE = intfName
				t.Logf("HundredGige interface %v ", hundredGigE)
			}
			break
		}
	}
}
func checklc(t *testing.T, dut *ondatra.DUTDevice) bool {
	lcs := components.FindComponentsByType(t, dut, linecardType)
	if got := len(lcs); got == 0 {
		return true
	}
	return false
}

func portName(t *testing.T, dut *ondatra.DUTDevice) {
	checkHundredGigE(t, dut)
	lccheck := checklc(t, dut)
	if hundredGigE != "" {
		if lccheck {
			hundredGigEComponentName = "0/RP0/CPU0" + qsfptype(t, dut, hundredGigE)
			t.Logf("HundredGigE component name %v ", hundredGigEComponentName)
		} else {
			hundredGigEComponentName = fmt.Sprintf("0/%v/CPU0", strings.Split(hundredGigE, "/")[1]) + qsfptype(t, dut, hundredGigE)
			t.Logf("HundredGigE component name %v ", hundredGigEComponentName)
		}
	}
	if lccheck {
		fourhundredGigEComponentName = "0/RP0/CPU0" + qsfptype(t, dut, dut.Port(t, "port1").Name())
		t.Logf("FourHundredGigE component name %v ", fourhundredGigEComponentName)
	} else {
		fourhundredGigEComponentName = fmt.Sprintf("0/%v/CPU0", strings.Split(dut.Port(t, "port1").Name(), "/")[1]) + qsfptype(t, dut, dut.Port(t, "port1").Name())
		t.Logf("FourHundredGigE component name %v ", fourhundredGigEComponentName)
	}

}

func verifyBreakout(index uint8, numBreakoutsWant uint8, numBreakoutsGot uint8, breakoutSpeedWant string, breakoutSpeedGot string, t *testing.T) {

	if index != uint8(0) {
		t.Errorf("Index: got %v, want 1", index)
	}
	if numBreakoutsGot != numBreakoutsWant {
		t.Errorf("Number of breakouts configured : got %v, want %v", numBreakoutsGot, numBreakoutsWant)
	}
	if breakoutSpeedGot != breakoutSpeedWant {
		t.Errorf("Breakout speed configured : got %v, want %v", breakoutSpeedGot, breakoutSpeedWant)
	}
}
func verifyDelete(t *testing.T, dut *ondatra.DUTDevice, compname string) {
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		gnmi.GetConfig(t, dut, gnmi.OC().Component(compname).Port().BreakoutMode().Group(1).Index().Config()) //catch the error  as it is expected and absorb the panic.
	}); errMsg != nil {
		t.Log("Expected failure ")
	} else {
		t.Errorf("This get on empty config should have failed : %s", *errMsg)
	}
}
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
