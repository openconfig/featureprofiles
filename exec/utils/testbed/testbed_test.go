package testbed_test

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

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
		"terminal length 0\nshow running-config",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	dut := ondatra.DUT(t, "dut")
	sshClient := dut.RawAPIs().CLI(t)

	content := ""
	for _, cmd := range commands {
		testt.CaptureFatal(t, func(t testing.TB) {
			if result, err := sshClient.RunCommand(ctx, cmd); err == nil {
				content += ">" + cmd + "\n"
				content += result.Output()
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
