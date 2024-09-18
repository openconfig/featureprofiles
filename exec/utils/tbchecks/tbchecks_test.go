package checktb_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestTBChecks(t *testing.T) {
	for _, dut := range ondatra.DUTs(t) {
		t.Logf("Checking gNMI connection on dut %v", dut.ID())
		gnmi.Get(t, dut, gnmi.OC().System().SoftwareVersion().State())
	}
}
