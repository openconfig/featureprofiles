package testbed_test

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

	commands := []string{
		"show version",
		"show platform",
		"show install fixes active",
		"show running-config",
	}

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	content := ""
	for _, cmd := range commands {
		if result, err := dut.RawAPIs().CLI(t).SendCommand(ctx, cmd); err == nil {
			content += ">" + cmd + "\n"
			content += result
			content += "\n"
		}
	}

	os.WriteFile(*outFile, []byte(content), 0644)
}
