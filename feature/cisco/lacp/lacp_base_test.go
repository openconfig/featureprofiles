package basetest

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/inputcisco"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	inputFile = "testdata/interface.yaml"
)

var (
	testInput = ipb.LoadInput(inputFile)
	device1   = "dut"
	observer  = fptest.NewObserver("LACP").AddCsvRecorder("ocreport").
			AddCsvRecorder("LACP")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
