package basetest

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
)

const (
	input_file = "interface.yaml"
)

var ()
var (
	testInput = ipb.LoadInput(input_file)
	device1   = "dut"
	observer  = fptest.NewObserver().AddCsvRecorder("ocreport").
			AddCsvRecorder("LACP")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
