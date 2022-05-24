package basetest

import (
	"sort"
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
	observer  = fptest.NewObserver("Interface").AddCsvRecorder("ocreport").
			AddCsvRecorder("Interface")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
func sliceEqual(a, b []string) bool {
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
