package lacp_base_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/input_cisco"
)

const (
	input_file = "interface.yaml"
)

var ()
var (
	testInput = ipb.LoadInput(input_file)
	device1   = "dut"
	observer  = fptest.
			NewObserver("LACP").
			AddCsvRecorder()
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
