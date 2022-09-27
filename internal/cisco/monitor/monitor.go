// Package config contains cisco specefic binding APIs to config a router using oc and text and cli.
package config

import (
	"container/ring"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/gnmiutil"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygot/ygot"
	"github.com/patrickmn/go-cache"
)

type EventType int

type TestArgs struct {
	DUT *ondatra.DUTDevice
	// ATE lock should be aquired before using ATE. Only one test can use the ATE at a time.
	ATELock sync.Mutex
	ATE     *ondatra.ATEDevice
}

const (
	Update EventType = iota + 1
	Replace
	Delete
)

type GNMIMonior struct {
	Paths     []ygot.PathStruct
	Consumer  gnmiutil.Consumer
	DUT       *ondatra.DUTDevice
	Verifiers []interface{}
	//StringVerifier Verifier[string]
	//IntVerifier Verifier[int]

}

type Verfier interface {
	Verify(t *testing.T)
}

type Datasource interface {
	Get(key string) (interface{}, bool)
}

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

type CachedConsumer struct {
	Cache *cache.Cache
}

func NewCachedConsumer(windowPeriod time.Duration) *CachedConsumer {
	return &CachedConsumer{
		Cache: cache.New(windowPeriod, windowPeriod+2),
	}
}
func (consumer *CachedConsumer) Process(datapoints []*gnmiutil.DataPoint) {
	//fmt.Println("Processing events")
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
		ring := ring.New(10) // let keep only last 10 values(5 minutes)
		ring.Value = data
		consumer.Cache.Set(pathStr, ring, 0)
	}
}

func (consumer *CachedConsumer) Get(key string) (interface{}, bool) {
	return consumer.Cache.Get(key)
}

type backGroundFunc func(t *testing.T, args *TestArgs, events *CachedConsumer)

// BackgroundFunc runs a testing function in the background. The period can be ticker or simple timer. With simple timer the function only run once
// Eeven refers to gnmi evelenet collected with streaming telemtry.
func BackgroundFunc(ctx context.Context, t *testing.T, period interface{}, args *TestArgs, events *CachedConsumer, function backGroundFunc) {
	t.Helper()
	timer, ok := period.(*time.Timer)
	if ok {
		go func() {
			<-timer.C
			function(t, args, events)
		}()
	}

	ticker, ok := period.(*time.Ticker)
	if ok {
		go func() {
			for {
				<-ticker.C
				function(t, args, events)
			}
		}()
	}
}

/*type VerifierType int
const (
	Once  VerifierType = iota + 1
	Always
	Ends
	Flips
	Starts
	Regex
	Sequence
)

type SignlePathVerfier struct{
	datesource  Datasource
	period time.Ticker
	key string
	expected []interface{}
	typee VerifierType
	window int
	journal ring.Ring
}
 func NewSignlePathVerfier(t *testing.T, datasource Datasource, period time.Ticker, expected ) *SignlePathVerfier {
	return & SignlePathVerfier{
		datesource: datasource,
		period: period,
		expected:  ,
	}
 }

func (v *SignlePathVerfier) Verify(t *testing.T) {
	switch (v.Type) {
	case Once:
	if ! v.state {
		if val==v.Value[0]  {
			v.state = true
		}
	}
	case Always:
		if val!=v.Value[0]  {
			t.Fatalf("%s is execpted to be %v all the times", v.Key, v.Value)
		}
	}

}

func (v *SignlePathVerfier) verificationLoop(t *testing.T) {
    for {
    	 <-v.Period.C
		 val := v.Datesource.Get(v.Key)
		 switch (v.Type) {
			case Once:

	}
}*/
