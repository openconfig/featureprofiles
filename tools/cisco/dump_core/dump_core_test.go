package basetest

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

type targetInfo struct {
	dut string
}

type Targets struct {
	targetInfo map[string]targetInfo
}

// NewTargets initializes a new Targets instance and sets up SSH information for each target
func NewTargets(t *testing.T) *Targets {
	t.Helper()
	nt := Targets{
		targetInfo: make(map[string]targetInfo),
	}
	return &nt
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCoreFileDecode(t *testing.T) {
	processList := []string{
		"bundlemgr_checker", "ifmgr", "netio", "pkt_trace_agent",
	}
	targets := NewTargets(t)
	for dutID, _ := range targets.targetInfo {
		dut := ondatra.DUT(t, dutID)
		t.Logf("Start dumping core for device : %s", dutID)
		for _, process := range processList {
			t.Run("create core file", func(t *testing.T) {
				t.Logf("Dumping core for device : %s , process name : %s", dutID, process)
				dut.CLI().RunResult(t, fmt.Sprintf("dumpcore running %s location 0/RP0/CPU0\n", process))
			})
			t.Logf("Finished dumping core for device : %s", dutID)
		}
	}
}
