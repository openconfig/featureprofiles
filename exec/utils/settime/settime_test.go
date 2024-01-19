package settime_test

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

var (
	timezone = flag.String("timezone", "America/Toronto", "Timezone")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSetTime(t *testing.T) {
	loc, err := time.LoadLocation(*timezone)
	if err != nil {
		t.Fatalf("Unknown timezone %s", *timezone)
	}

	tn := time.Now().In(loc)
	t.Logf("Time in %s: %v", *timezone, tn)

	dut := ondatra.DUT(t, "dut")
	sshClient := dut.RawAPIs().CLI(t)

	cmd := fmt.Sprintf("clock set %s", tn.Format("15:04:05 Jan 02 2006"))
	t.Logf("Sending command %s", cmd)

	if resp, err := sshClient.RunCommand(context.Background(), cmd); err != nil {
		t.Logf("Error executing command: %v", err)
	} else {
		t.Log(resp.Output())
	}
}
