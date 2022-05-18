package basetest

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
)

const (
	inputFile = "testdata/interface.yaml"
)

var (
	testInput = ipb.LoadInput(inputFile)
	device1   = "dut"
	observer  = fptest.NewObserver().AddCsvRecorder("ocreport").
			AddCsvRecorder("LACP")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
