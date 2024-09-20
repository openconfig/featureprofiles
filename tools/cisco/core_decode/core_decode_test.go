package basetest

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCoreFileDecode(t *testing.T) {
	dut := ondatra.DUT(t, "dut1")
	peer := ondatra.DUT(t, "dut2")

	processList := []string{
		"bundlemgr_checker", "ifmgr", "netio", "pkt_trace_agent",
	}

	for _, process := range processList {
		t.Run("create core file", func(t *testing.T) {
			dut.CLI().RunResult(t, fmt.Sprintf("dumpcore running %s location 0/RP0/CPU0\n", process))
			peer.CLI().RunResult(t, fmt.Sprintf("dumpcore running %s location 0/RP0/CPU0\n", process))
		})
	}
}
