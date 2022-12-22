package basetest

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
)

const (
	inputFile = "testdata/lldp.yaml"
)

var (
	testInput = ipb.LoadInput(inputFile)
	device1   = "dut"
	device2   = "peer"
	observer  = fptest.NewObserver("LLDP").AddCsvRecorder("ocreport").
			AddCsvRecorder("LLDP")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
