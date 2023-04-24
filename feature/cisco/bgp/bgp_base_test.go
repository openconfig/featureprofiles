package basetest

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	yc "github.com/openconfig/featureprofiles/tools/cisco/yang_coverage"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
)

const (
	inputFile = "testdata/bgp.yaml"
)

var (
	testInput = ipb.LoadInput(inputFile)
	device1   = "dut"
	ate1      = "ate"
	observer  = fptest.NewObserver("BGP").AddCsvRecorder("ocreport").
			AddCsvRecorder("BGP")
)

func TestMain(m *testing.M) {
	yc.Init("")
	fptest.RunTests(m)
}
