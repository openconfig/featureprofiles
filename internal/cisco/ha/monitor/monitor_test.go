// Package config contains cisco specefic binding APIs to config a router using oc and text and cli.
package config

import (
	//"container/ring"
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gnmiutil"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/confgen"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

const (
	pingThreadScale          = 5    // # of parallel ping request to send
	pingScale                = 2000 // # of ping message to send
	AFTTelemtryUpdateTimeout = 120   // second
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func testGNMISet(t *testing.T, args *TestArgs, event *CachedConsumer) {
	ports := args.DUT.Ports()
	bundles := []confgen.Bundle{
		{
			Id:                121,
			Interfaces:        []string{ports[0].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			Id:                122,
			Interfaces:        []string{ports[1].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			Id:                123,
			Interfaces:        []string{ports[2].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			Id:                124,
			Interfaces:        []string{ports[3].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			Id:                125,
			Interfaces:        []string{ports[4].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			Id:                126,
			Interfaces:        []string{ports[5].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			Id:                127,
			Interfaces:        []string{ports[7].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			Id: 128,
		},
	}
	generatedConf := confgen.GenerateConfig(bundles,"/Users/mbagherz/git/test_ws/src/featureprofiles/internal/cisco/ha/confgen/templates/gnmi.jsonnet")
	configRoot := &telemetry.Device{}
	if err := telemetry.Unmarshal([]byte(generatedConf), configRoot); err != nil {
		t.Fatalf(err.Error())
	}
	args.DUT.Config().Replace(t, configRoot)

}

func testPing(t *testing.T, args *TestArgs, event *CachedConsumer) {
	t.Helper()
	startTime := time.Now()
	t.Logf("Ping test is started at: %v", startTime)
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
	for i := 0; i <= pingThreadScale; i++ {
		go func(t *testing.T) {
			for i := 0; i < pingScale; i++ {
				pingClient, err := gnoiClient.System().Ping(context.Background(), pingRequest)
				if err != nil {
					t.Errorf("Failed to query gnoi endpoint: %v", err)
				}
				responses, err := fetchResponses(pingClient)
				if err != nil {
					t.Errorf("Failed to handle gnoi ping client stream: %v", err)
				}
				if len(responses) == 0 {
					t.Errorf("Number of responses to %v: got 0, want > 0", pingRequest.Destination)
				}
			}
		}(t)
	}

	endTime := time.Now()
	t.Logf("Ping test is completed by doing ping %d times  at %v, (The completion time is %v)", pingScale*pingThreadScale, endTime, time.Since(startTime))
}

func testBatchADDReplaceDeleteIPV4(t *testing.T, args *TestArgs, events *CachedConsumer) {
	t.Helper()
	startTime := time.Now()
	t.Logf("Gribi test is started at: %v", startTime)
	ciscoFlags.GRIBIFIBCheck = ygot.Bool(true)
	scale := uint(1500)
	ciscoFlags.GRIBIScale = &scale
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
	defer gribiC.FlushServer(t)
	// 192.0.2.42/32  Next-Site
	t.Log("Gribi Test: Adding next site")
	weights := map[uint64]uint64{41: 40}
	gribiC.AddNH(t, 41, "100.129.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks) // Not connected
	gribiC.AddNHG(t, 100, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	gribiC.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	// 11.11.11.0/32 Self-Site
	t.Log("Gribi Test: Self-site")
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.11", i, "32"))
	}
	weights = map[uint64]uint64{20: 99}
	gribiC.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	gribiC.AddNHG(t, 1, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// Add
	gribiC.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	waitTime := 0
	// check for receving update
out:
	for {
		for _, prefix := range prefixes {
			path := args.DUT.Telemetry().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Afts().Ipv4Entry(prefix).Prefix()
			strpath := gnmiutil.PathStructToString(path)
			_, found := events.Cache.Get(strpath)
			if found {
				break out
			}
		}
		if waitTime > AFTTelemtryUpdateTimeout {
			t.Fatalf("The Telemtry Update for AFT entries added by gribi is not recieved ontime, waittime: %d seconds", AFTTelemtryUpdateTimeout)
		}
		time.Sleep(10 * time.Second)
		waitTime += 10
	}
	// check to make sure we have update for all prefix
	for _, prefix := range prefixes {
		path := args.DUT.Telemetry().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Afts().Ipv4Entry(prefix).Prefix()
		strpath := gnmiutil.PathStructToString(path)
		for {
			_, found := events.Cache.Get(strpath)
			if found {
				break
			}
			if waitTime > AFTTelemtryUpdateTimeout {
				t.Fatalf("The Telemtry Update for AFT entry %s added by gribi is not recieved", prefix)
			}
			time.Sleep(10 * time.Second)
			waitTime += 10
		}
	}
	endTime := time.Now()
	t.Logf("Gribi test is completed by adding %d entries at %v, (The completion time is %v)", len(prefixes), endTime, time.Since(startTime))
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
	confifVRFS(t, dut)
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
			//dut.Telemetry().System().Memory(),
			//dut.Telemetry().System().CpuAny(),
			dut.Telemetry().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Afts(),
			dut.Telemetry().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Afts(),
		},
		Consumer: eventConsumer,
		DUT:      dut,
	}
	ctx, cancelMonitors := context.WithCancel(context.Background())
	//go monitor.Start(ctx, t, true, gpb.SubscriptionList_ONCE)
	monitor.Start(ctx, t, true, gpb.SubscriptionList_STREAM)
	// start tests
	testGroup := &sync.WaitGroup{}
	// start reset/apply config
	//BackgroundFunc(ctx, t, time.NewTimer(10*time.Millisecond), testArgs, eventConsumer, testGNMISet, testGroup)

	// start gribi test writter
	BackgroundFunc(ctx, t, time.NewTimer(1*time.Second), testArgs, eventConsumer, testBatchADDReplaceDeleteIPV4, testGroup)
	// start gribi tests getter

	// start gnoi  ping
	BackgroundFunc(ctx, t, time.NewTimer(10*time.Second), testArgs, eventConsumer, testPing, testGroup)
	// start p4rt
	//runBackground(gribi, )
	//BackgroundFunc(ctx, t, time.NewTimer(10*time.Second), testArgs, eventConsumer, testPing, testGroup)

	time.Sleep(11 * time.Second) // wait until the test start (the timer period + 1)
	testGroup.Wait()
	cancelMonitors()
	time.Sleep(60*time.Second)

	/*for key, val := range eventConsumer.Cache.Items() {
		ring := val.Object.(*ring.Ring)
		// just for debugging, will be removed later
		fmt.Printf("%s:%d\n", key, ring.Len())
		/*if ring.Prev().Value != nil {
			fmt.Printf("%s:%d\n", key, ring.Len())
		}*/
	//}
	fmt.Println(len(eventConsumer.Cache.Items()))

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
