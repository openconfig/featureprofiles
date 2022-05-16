package bgp_base_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	ipb "github.com/openconfig/featureprofiles/tools/input_cisco"
)

const (
	input_file = "bgp.yaml"
)

var ()
var (
	testInput = ipb.LoadInput(input_file)
	device1   = "dut"
	device2   = "peer"
	observer  = fptest.
			NewObserver("BGP").
			AddCsvRecorder()
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
