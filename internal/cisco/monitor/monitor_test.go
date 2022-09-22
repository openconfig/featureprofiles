// Package config contains cisco specefic binding APIs to config a router using oc and text and cli.
package config

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/gnmiutil"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/protobuf/encoding/prototext"
)

type simpleConsumer struct {
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func (consumer *simpleConsumer) Process(datapoints []*gnmiutil.DataPoint) {
	//fmt.Println("Processing events")
	for _, data := range datapoints {
		pathStr, _ := ygot.PathToString(data.Path)
		fmt.Printf("Processing events at %v for path %s :  %v ", data.RecvTimestamp, pathStr, prototext.Format(data.Value))
		//fmt.Println(data.String())
	}
}

func TestStart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	simpleConsumer := &simpleConsumer{}
	monitor := GNMIMonior{
		Paths: []ygot.PathStruct{dut.Telemetry().Interface(dut.Port(t, "port1").Name()),
			dut.Telemetry().Interface(dut.Port(t, "port2").Name()),
			dut.Telemetry().Interface(dut.Port(t, "port3").Name()),
			dut.Telemetry().System(),
			dut.Telemetry().ComponentAny(),
		},
		Consumer: simpleConsumer,
		DUT:      dut,
	}
	ctx, cancelMonitors := context.WithCancel(context.Background())
	//go monitor.Start(ctx, t, true, gpb.SubscriptionList_ONCE)
	monitor.Start(ctx, t, true, gpb.SubscriptionList_STREAM)
	// write other tests here rather. The monitor will recive and process all telemtry streams while the test is running
	time.Sleep(60 * time.Second)
	cancelMonitors()
	time.Sleep(60 * time.Second)

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
