// Package config contains cisco specefic binding APIs to config a router using oc and text and cli.
package config

import (
	"container/ring"
	"context"
	"fmt"
	"testing"
	"time"

	ciscoFlag "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestLoad(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	eventConsumer:= NewCachedConsumer(5*time.Minute)
	monitor := GNMIMonior{
		Paths: []ygot.PathStruct{dut.Telemetry().Interface(dut.Port(t, "port1").Name()),
			dut.Telemetry().System(),
			dut.Telemetry().ComponentAny(),
			dut.Telemetry().InterfaceAny(),
			dut.Telemetry().NetworkInstance(*ciscoFlag.DefaultNetworkInstance).Afts(),
		},
		Consumer: eventConsumer,
		DUT:      dut,
	}
	ctx, cancelMonitors := context.WithCancel(context.Background())
	//go monitor.Start(ctx, t, true, gpb.SubscriptionList_ONCE)
	monitor.Start(ctx, t, true, gpb.SubscriptionList_STREAM)
	//runBackground(gribi, ) // add entries
	//runBackground(gribi, ) // add entries

	// write other tests here rather. The monitor will recive and process all telemtry streams while the test is running

	//
	time.Sleep(70 * time.Second)
	cancelMonitors()
	for key, val := range eventConsumer.Cache.Items() {
		ring := val.Object.(*ring.Ring)
		if ring.Prev().Value!=nil {
			fmt.Printf("%s:%d\n",key,ring.Len())
		}
	}
	time.Sleep(10 * time.Second)

}

/*writePath func write(path *gnmi.Path) {
	pathStr, err := ygot.PathToString(path)
	if err != nil {
		pathStr = prototext.Format(path)
	}
	fmt.Fprintf(&buf, "%s\n", pathStr)
}

writeVal := func(val *gnmi.TypedValue) {
	switch v := val.Value.(type) {
	case *gnmi.TypedValue_JsonIetfVal:
		fmt.Fprintf(&buf, "%s\n", v.JsonIetfVal)
	default:
		fmt.Fprintf(&buf, "%s\n", prototext.Format(val))
	}
}*/
