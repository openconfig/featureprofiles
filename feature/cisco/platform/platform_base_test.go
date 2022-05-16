package platform_base_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
)

const (
	RP = "0/RP0/CPU0"
)

var (
	device1  = "dut"
	observer = fptest.
			NewObserver("Platform").
			AddCsvRecorder()
	PlatformSF = PlatformInfo{
		Chassis:      "Rack 0",
		Linecard:     "0/0/CPU0",
		OpticsModule: "HundredGigE0/0/0/0",
		FanTray:      "0/FT0",
		PowerSupply:  "0/PT0-PM0",
		TempSensor:   "0/0/CPU0-TEMP_FET1_DX",
		BiosFirmware: "0/0/CPU0-Bios",
	}
	Platform = PlatformSF
)

//to hold platform info
//To do: get this dynamiclly from device
type PlatformInfo struct {
	Chassis      string
	Linecard     string
	OpticsModule string
	FanTray      string
	PowerSupply  string
	TempSensor   string
	BiosFirmware string
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
