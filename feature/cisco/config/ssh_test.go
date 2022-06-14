package config

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/topologies/binding/cisco/config"
	"github.com/openconfig/ondatra"
)

func TestCLIConfigViaSSH(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	oldHostName := dut.Telemetry().System().Hostname().Get(t)
	newHostname := oldHostName + "new"
	config.TextWithSSH(context.Background(), t, dut, fmt.Sprintf("config \n hostname %s \n commit \n", newHostname), 30*time.Second)
	defer config.TextWithSSH(context.Background(), t, dut, fmt.Sprintf("config \n hostname %s \n commit \n", oldHostName), 30*time.Second)
	if got := dut.Telemetry().System().Hostname().Get(t); got != newHostname {
		t.Fatalf("Expected the host name to be %s, got %s", newHostname, got)
	}
}
