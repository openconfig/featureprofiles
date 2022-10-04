//  Package monitor contains utolity api for monitoring telemetry paths in background while running tests
//  A monitor pushes all event to the an event consumer that should provide process method.
//  A monitor can monitor multipe paths, however provided paths should be disjoint.

package monitor

import (
	"container/ring"
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/gnmiutil"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"
	"github.com/patrickmn/go-cache"
)

// GNMIMonior provides access to Monitoring Paths.
//
// Usage:
//
// eventConsumer := NewCachedConsumer(5 * time.Minute)
//
//	monitor := GNMIMonior{
//		Paths: []ygot.PathStruct{
//			dut.Telemetry().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Afts(),
//			dut.Telemetry().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Afts(),
//		},
//		Consumer: eventConsumer,
//		DUT:      dut,
//	}
//
// ctx, cancelMonitors := context.WithCancel(context.Background())
// monitor.Start(ctx, t, true, gpb.SubscriptionList_STREAM)
// start tests
type GNMIMonior struct {
	Paths    []ygot.PathStruct
	Consumer gnmiutil.Consumer
	DUT      *ondatra.DUTDevice
}

// Datasource is used to store the telemtry events.
// Any datastore providing Get functionality can be used as the datasoiurce
type Datasource interface {
	Get(key string) (interface{}, bool)
}

// Start function starts the telemetry steraming for paths specefied when defining monitors.
// The received events passed to an event consumer for processing.
func (monitor *GNMIMonior) Start(context context.Context, t *testing.T, shareStub bool, mode gnmi.SubscriptionList_Mode) {
	t.Helper()
	for _, ygotPath := range monitor.Paths {
		{
			path, _, err := gnmiutil.ResolvePath(ygotPath)
			if err != nil {
				t.Fatalf("Could not start the monitor for path %v", ygotPath)
			}
			watcher, path, err := gnmiutil.Watch(context, t, monitor.DUT, ygotPath, []*gnmi.Path{path}, false, monitor.Consumer, mode)
			if err != nil {
				t.Fatalf("Could not start the watcher for path %v", path)
			}
			// this need to be run as part of monitor group
			go watcher.Await(t)
		}
	}
}

// CachedConsumer is a datasource and event consumer that use go-chache (local caching) to store the events.
// It uses a cirrculat buffer to only store the last n event.
// It also implements process function, so it can be used as the event processor dicretly.
type CachedConsumer struct {
	Cache      *cache.Cache
	bufferSize int
}

// NewCachedConsumer Initialize the CachedConsumer. The windowPeriod specefies the expiration time of the events that will be cached.
// bufferSize specefies the number of events that will be kept for each path
func NewCachedConsumer(windowPeriod time.Duration, bufferSize int) *CachedConsumer {
	return &CachedConsumer{
		Cache:      cache.New(windowPeriod, windowPeriod+2),
		bufferSize: bufferSize,
	}
}

// Process recives the event and saves them in the cach
func (consumer *CachedConsumer) Process(datapoints []*gnmiutil.DataPoint) {
	for _, data := range datapoints {
		pathStr, _ := ygot.PathToString(data.Path)
		preValue, found := consumer.Cache.Get(pathStr)
		if found {
			ring := preValue.(*ring.Ring)
			ring = ring.Next()
			ring.Value = data
			consumer.Cache.Set(pathStr, ring, 0)
			return
		}
		ring := ring.New(consumer.bufferSize) // let keep only last 10 values(5 minutes)
		ring.Value = data
		consumer.Cache.Set(pathStr, ring, 0)
	}
}

// Get return the event related to a oc path specefid as the key
func (consumer *CachedConsumer) Get(key string) (interface{}, bool) {
	return consumer.Cache.Get(key)
}
