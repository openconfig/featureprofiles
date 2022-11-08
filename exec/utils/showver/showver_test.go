package showver_test

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

var (
	outFile = flag.String("outFile", "", "Output file for version information")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestShowVersion(t *testing.T) {
	if *outFile == "" {
		t.Fatal("Missing outFile arg")
	}

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	if result, err := dut.RawAPIs().CLI(t).SendCommand(ctx, "show version"); err == nil {
		os.WriteFile(*outFile, []byte(result), 0644)
	}
}
