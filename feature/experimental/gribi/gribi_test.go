package gribi

import (
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"testing"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSampleGRIBI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gribiC := NewGRIBIFluent(t, dut, true, false)
	defer gribiC.Close(t)
	gribiC.BeMaster(t)
	gribiC.AddNH(t, 1, "192.168.1.1", "DEFAULT")
	gribiC.AddNHG(t, 1, map[uint64]uint64{1: 1}, "DEFAULT")
	gribiC.AddIPV4Entry(t, 1, "192.168.1.0/24", "DEFAULT")
}
