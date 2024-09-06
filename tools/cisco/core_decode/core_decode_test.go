package basetest

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCoreFileDecode(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("create core file", func(t *testing.T) {
		dut.CLI().RunResult(t, "dumpcore running vlan_ea  location 0/RP0/CPU0\n")
	})
}
