package testbed_test

import (
	"context"
	"flag"
	"os"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"
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
		"run cat /etc/build-info.txt",
		"show version",
		"show install fixes active",
		"show platform",
		"show context",
		"show running-config",
	}

	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	sshClient := dut.RawAPIs().CLI(t)
	defer sshClient.Close()

	content := ""
	for _, cmd := range commands {
		testt.CaptureFatal(t, func(t testing.TB) {
			if result, err := sshClient.SendCommand(ctx, cmd); err == nil {
				content += ">" + cmd + "\n"
				content += result
				content += "\n"
			} else {
				content += ">" + cmd + "\n"
				content += err.Error()
				content += "\n"
			}
		})
	}

	os.WriteFile(*outFile, []byte(content), 0644)
}
