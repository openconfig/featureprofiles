package interface_base_test

import (
	"sort"
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
			NewObserver("Interface").AddAdditionalCsvRecorder("ocreport").
			AddCsvRecorder()
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
func SliceEqual(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
