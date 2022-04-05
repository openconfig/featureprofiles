package cli

import (
	"fmt"
	"testing"
	"context"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCLI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cli := dut.RawAPIs().CLI(t)
	resp, err := cli.SendCommand(context.Background(), "show ver")
	if err!=nil {
		fmt.Println(resp)
	}
}
