package cli

import (
	"testing"
	"context"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/topologies/binding/cisco"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCLI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ciscoCLI := cisco.CiscoCLI {
		Handle: dut.RawAPIs().CLI(t),
	}
	resp, err := ciscoCLI.Config(t, context.Background(), "config \n hostname test \n commit \n",30*time.Second)
	if err!=nil {
		t.Fatalf("CLI config is failed")
	}
	t.Log(resp)
}
