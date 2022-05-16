package bgp_base_test

import (
	"testing"

	"github.com/openconfig/ondatra"
)

func TestInterfaceCfgs(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	input_obj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	iut := input_obj.Device(dut).GetInterface("Bundle-Ether120")

}
