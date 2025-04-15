package static_route_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestStaticRoute(t *testing.T) {

	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	ate := ondatra.ATE(t, "ate")

	configureDUT(t, dut1)
	configureDUT(t, dut2)
	topo := configureATE(t, ate)
	t.Log("ATE CONFIG: ", topo)

}
