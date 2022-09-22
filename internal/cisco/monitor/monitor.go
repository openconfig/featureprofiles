// Package config contains cisco specefic binding APIs to config a router using oc and text and cli.
package config

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/gnmiutil"
	"github.com/openconfig/gnmi/proto/gnmi"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"
)

type EventType int

const (
	Update EventType = iota + 1
	Replace
	Delete
)

type GNMIMonior struct {
	Paths  []ygot.PathStruct
	Consumer  gnmiutil.Consumer
	DUT 	*ondatra.DUTDevice
	//events map[string][]gnmiutil.DataPoint
}

func (monitor *GNMIMonior) Start(context context.Context, t *testing.T, shareStub bool, mode gpb.SubscriptionList_Mode) {
	t.Helper()
	for _, ygotPath := range monitor.Paths {
		{
			path, _, err := gnmiutil.ResolvePath(ygotPath)
			if err != nil {
				t.Fatalf("Could not start the monitor for path %v", ygotPath)
			}
			watcher, path, err := gnmiutil.Watch(context,t,monitor.DUT, ygotPath, []*gnmi.Path{path}, false, monitor.Consumer, mode)
			if err != nil {
				t.Fatalf("Could not start the watcher for path %v", path)
			}
			// this need to be run as part of monitor group
			go watcher.Await(t)
		} 
	}
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
