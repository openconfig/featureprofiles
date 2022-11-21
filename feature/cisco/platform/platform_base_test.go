package basetest

import (
	"flag"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
)

const (
	RP = "0/RP0/CPU0"
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

func verifyBreakout(index uint8, numBreakouts uint8, breakoutSpeed string, t *testing.T) {

	if index != uint8(1) {
		t.Errorf("Index: got %v, want 1", index)
	}
	if numBreakouts != uint8(4) {
		t.Errorf("Number of breakouts configured : got %v, want 4", numBreakouts)
	}
	if breakoutSpeed != "SPEED_10GB" {
		t.Errorf("Breakout speed configured : got %v, want 10GB", breakoutSpeed)
	}
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
