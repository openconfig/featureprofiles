// Package config contains cisco specefic binding APIs to config a router using oc and text and cli.
package config

import (
	"container/ring"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gnmiutil"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/ondatra/telemetry"

)

const (
	pingScale = 5 // # of parallel ping request to send
	AFTTelemtryUpdateTimeout = 60 // second
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func testPing(t *testing.T, args *TestArgs, event *CachedConsumer) {
	t.Logf("Ping test is started ....")
	fetchResponses := func(c spb.System_PingClient) ([]*spb.PingResponse, error) {
		pingResp := []*spb.PingResponse{}
		for {
			resp, err := c.Recv()
			switch {
			case err == io.EOF:
				return pingResp, nil
			case err != nil:
				return nil, err
			default:
				pingResp = append(pingResp, resp)
			}
		}
	}
	lbIntf := netutil.LoopbackInterface(t, args.DUT, 0)
	lo0 := args.DUT.Telemetry().Interface(lbIntf).Subinterface(0)
	ipv4Addrs := lo0.Ipv4().AddressAny().Get(t)
	ipv6Addrs := lo0.Ipv6().AddressAny().Get(t)
	if len(ipv4Addrs) == 0 {
		t.Fatalf("Failed to get a valid IPv4 loopback address: %+v", ipv4Addrs)
	}
	if len(ipv6Addrs) == 0 {
		t.Fatalf("Failed to get a valid IPv6 loopback address: %+v", ipv6Addrs)
	}

	gnoiClient := args.DUT.RawAPIs().GNOI().Default(t)
	pingRequest := &spb.PingRequest{
		Destination: ipv4Addrs[0].GetIp(),
		Source:      ipv4Addrs[0].GetIp(),
		L3Protocol:  tpb.L3Protocol_IPV4,
	}
	for i := 0; i <= pingScale; i++ {
		go func() {
			pingClient, err := gnoiClient.System().Ping(context.Background(), pingRequest)
			if err != nil {
				t.Fatalf("Failed to query gnoi endpoint: %v", err)
			}
			responses, err := fetchResponses(pingClient)
			if err != nil {
				t.Fatalf("Failed to handle gnoi ping client stream: %v", err)
			}
			if len(responses) == 0 {
				t.Errorf("Number of responses to %v: got 0, want > 0", pingRequest.Destination)
			}
		}()
	}
	t.Logf("Ping test is completed sucessfully")

	//StdDevZero := true
	//pingTime := responses[len(responses)-1].Time
}


func testBatchADDReplaceDeleteIPV4(t *testing.T, args *TestArgs, events *CachedConsumer, done chan bool)   {
	t.Helper()
	t.Logf("Gribi test is started ....")
	ciscoFlags.GRIBIFIBCheck = ygot.Bool(true)
	//scale := uint(20000)
	//ciscoFlags.GRIBIScale = & scale
	ciscoFlags.GRIBIChecks.AFTChainCheck = false
	ciscoFlags.GRIBIChecks.AFTCheck = false
	ciscoFlags.GRIBIChecks.FIBACK = true
	ciscoFlags.GRIBIChecks.RIBACK = true
	gribiC := gribi.Client{
		DUT:                  args.DUT,
		FibACK:               true,
		Persistence:          true,
		InitialElectionIDLow: 100,
	}
	defer gribiC.Close(t)
	if err := gribiC.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}
	gribiC.BecomeLeader(t)
	gribiC.FlushServer(t)
	// 192.0.2.42/32  Next-Site
	weights := map[uint64]uint64{41: 40}
	gribiC.AddNH(t, 41, "100.129.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks) // Not connected
	gribiC.AddNHG(t, 100, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	gribiC.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32 Self-Site
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.11", i, "32"))
	}
	weights = map[uint64]uint64{20: 99}
	gribiC.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	gribiC.AddNHG(t, 1, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// Add
	gribiC.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	waitTime:= 0
	// check for receving update
	out:
	for ;; {
		for _, prefix := range prefixes {
			path := args.DUT.Telemetry().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Afts().Ipv4Entry(prefix).Prefix()
			strpath := gnmiutil.PathStructToString(path)
			_,found := events.Cache.Get(strpath); if found {
				break out
			}
		}
		if waitTime > AFTTelemtryUpdateTimeout {
			t.Fatalf("The Telemtry Update for AFT entries added by gribi is not recieved ontime, waittime: %d seconds", AFTTelemtryUpdateTimeout)
		}
		time.Sleep(10*time.Second)
		waitTime += 10
	}
	// check to make sure we have update for all prefix 
	for _, prefix := range prefixes { 
		path := args.DUT.Telemetry().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Afts().Ipv4Entry(prefix).Prefix()
		strpath := gnmiutil.PathStructToString(path)
		_,found := events.Cache.Get(strpath); if ! found {
			t.Fatalf("The Telemtry Update for AFT entry %s added by gribi is not recieved",prefix )
		}
	}
	t.Logf("Gribi test is completed succefully")
	done <- true
}

func confifVRFS(t *testing.T, dut *ondatra.DUTDevice) {
	d := &telemetry.Device{}
	ni1 := d.GetOrCreateNetworkInstance(*ciscoFlags.NonDefaultNetworkInstance)
	ni1.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "default")
	dut.Config().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Replace(t, ni1)
	ni2 := d.GetOrCreateNetworkInstance(*ciscoFlags.NonDefaultNetworkInstance)
	ni2.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "default")
	dut.Config().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Replace(t, ni2)
}

func TestLoad(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	confifVRFS(t,dut)
	testArgs := &TestArgs{
		DUT:     dut,
		ATE:     ate,
		ATELock: sync.Mutex{},
	}

	eventConsumer := NewCachedConsumer(5 * time.Minute)
	monitor := GNMIMonior{
		Paths: []ygot.PathStruct{
			//dut.Telemetry().Interface(dut.Port(t, "port1").Name()),
			//dut.Telemetry().System(),
			//dut.Telemetry().ComponentAny(),
			//dut.Telemetry().InterfaceAny(),
			dut.Telemetry().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Afts(),
			dut.Telemetry().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Afts(),
		},
		Consumer: eventConsumer,
		DUT:      dut,
	}
	ctx, cancelMonitors := context.WithCancel(context.Background())
	//go monitor.Start(ctx, t, true, gpb.SubscriptionList_ONCE)
	monitor.Start(ctx, t, true, gpb.SubscriptionList_STREAM)
	// start reset/apply config
	// start gribi test writter
	BackgroundFunc(ctx, t, time.NewTimer(1 * time.Second), testArgs, eventConsumer, testBatchADDReplaceDeleteIPV4)
	// start gribi tests getter
	// start gnoi  ping
	BackgroundFunc(ctx, t, time.NewTimer(10 * time.Second), testArgs, eventConsumer, testPing)
	// start p4rt
	//runBackground(gribi, ) // add entries

	// write other tests here rather. The monitor will recive and process all telemtry streams while the test is running

	//
	time.Sleep(80 * time.Second)
	cancelMonitors()
	for key, val := range eventConsumer.Cache.Items() {
		ring := val.Object.(*ring.Ring)
		fmt.Printf("%s:%d\n", key, ring.Len())
		/*if ring.Prev().Value != nil {
			fmt.Printf("%s:%d\n", key, ring.Len())
		}*/
	}
	//time.Sleep(10 * time.Second)

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
