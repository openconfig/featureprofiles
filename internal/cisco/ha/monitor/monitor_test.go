// Package config contains cisco specefic binding APIs to config a router using oc and text and cli.
package monitor

import (
	"context"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestMonitor(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	eventConsumer := NewCachedConsumer(5*time.Minute, /*expiration time for events in the cache*/
		10 /*number of events for keep for each leaf*/)
	monitor := GNMIMonior{
		Paths: []ygot.PathStruct{
			dut.Telemetry().System().Memory(),
			dut.Telemetry().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Afts(),
		},
		Consumer: eventConsumer,
		DUT:      dut,
	}
	ctx, cancelMonitors := context.WithCancel(context.Background())
	defer cancelMonitors()
	monitor.Start(ctx, t, true, gpb.SubscriptionList_STREAM)

	time.Sleep(31 * time.Second)
	if len(eventConsumer.Cache.Items()) == 0 {
		t.Fatal("NO Telemtry Event is Recieved: Expected at least one event for /system/memory")
	}
}
