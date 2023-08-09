package gnmi_scale_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestGNMIScale(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	beforeTime := time.Now()
	for i := 0; i <= 100; i++ {
		gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test"+strconv.Itoa(i))
	}
	t.Logf("Time to do 100 gnmi uodate is %s", time.Since(beforeTime).String())
	if int(time.Since(beforeTime).Seconds()) >= 180 {
		t.Fatalf("GNMI Scale Took too long")
	}
}
