package basetest

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
)

const (
	input_file = "bgp.yaml"
)

var (
	testInput = ipb.LoadInput(input_file)
	device1   = "dut"
	device2   = "peer"
	ate1      = "ate"
	observer  = fptest.NewObserver().AddCsvRecorder("ocreport").
			AddCsvRecorder("BGP")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
