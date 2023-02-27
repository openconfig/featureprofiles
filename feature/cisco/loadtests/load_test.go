// Package config contains cisco specefic binding APIs to config a router using oc and text and cli.
package load_test

import (
	"context"
	"flag"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/gnmiutil"
	"github.com/openconfig/featureprofiles/internal/cisco/gribi"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/confgen"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/monitor"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/runner"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	tpb "github.com/openconfig/gnoi/types"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	pingThreadScale          = 5    // # of parallel ping request to send
	pingScale                = 2000 // # of ping message to send
	AFTTelemtryUpdateTimeout = 120  // second
)

var (
	configFilePath = flag.String("gnmi_config_file", "", "Path for gNMI config file")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func testGNMISet(t *testing.T, event *monitor.CachedConsumer, args ...interface{}) {
	dut := args[0].(*ondatra.DUTDevice)

	// TODO: The below code is not tested yet
	if *configFilePath == "" {
		return
	}
	ports := dut.Ports()
	bundles := []confgen.Bundle{
		{
			ID:                121,
			Interfaces:        []string{ports[0].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			ID:                122,
			Interfaces:        []string{ports[1].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			ID:                123,
			Interfaces:        []string{ports[2].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			ID:                124,
			Interfaces:        []string{ports[3].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			ID:                125,
			Interfaces:        []string{ports[4].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			ID:                126,
			Interfaces:        []string{ports[5].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			ID:                127,
			Interfaces:        []string{ports[7].Name()},
			SubInterfaceRange: []int{100, 196},
		},
		{
			ID: 128,
		},
	}
	generatedConf := confgen.GenerateConfig(bundles, *configFilePath)
	configRoot := &oc.Root{}
	if err := oc.Unmarshal([]byte(generatedConf), configRoot); err != nil {
		t.Fatalf(err.Error())
	}
	gnmi.Replace(t, dut, gnmi.OC().Config(), configRoot)
}

func testPing(t *testing.T, event *monitor.CachedConsumer, args ...interface{}) {
	t.Helper()

	dut := args[0].(*ondatra.DUTDevice)
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
	lbIntf := netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(lbIntf).Subinterface(0)
	ipv4Addrs := gnmi.GetAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.GetAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 {
		t.Fatalf("Failed to get a valid IPv4 loopback address: %+v", ipv4Addrs)
	}
	if len(ipv6Addrs) == 0 {
		t.Fatalf("Failed to get a valid IPv6 loopback address: %+v", ipv6Addrs)
	}

	gnoiClient := dut.RawAPIs().GNOI().Default(t)
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

func testBatchADDReplaceDeleteIPV4(t *testing.T, events *monitor.CachedConsumer, args ...interface{}) {
	t.Helper()
	dut := args[0].(*ondatra.DUTDevice)

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
		DUT:                  dut,
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

	// check to make sure we get at least one update
out:
	for {
		for _, prefix := range prefixes {
			path := gnmi.OC().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Afts().Ipv4Entry(prefix).Prefix()
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
	// check to make sure we have update for all prefixes
	for _, prefix := range prefixes {
		path := gnmi.OC().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Afts().Ipv4Entry(prefix).Prefix()
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

func configVRFS(t *testing.T, dut *ondatra.DUTDevice) {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(*ciscoFlags.NonDefaultNetworkInstance)
	ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "default")
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Config(), ni1)
	ni2 := d.GetOrCreateNetworkInstance(*ciscoFlags.NonDefaultNetworkInstance)
	ni2.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "default")
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Config(), ni2)
}

func TestLoad(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	resp := config.CMDViaGNMI(context.Background(), t, dut, "show version")
	t.Logf(resp)
	if strings.Contains(resp, "VXR") {
		t.Logf("Skipping since platfrom is VXR")
		t.Skip()
	}
	// ate := ondatra.ATE(t, "ate")
	configVRFS(t, dut)

	eventConsumer := monitor.NewCachedConsumer(5*time.Minute, /*expiration time for events in the cache*/
		10 /*number of events for keep for each leaf*/)
	monitor := monitor.GNMIMonior{
		Paths: []ygnmi.PathStruct{
			gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Afts(),
			gnmi.OC().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Afts(),
			// other paths can be added here
		},
		Consumer: eventConsumer,
		DUT:      dut,
	}
	ctx, cancelMonitors := context.WithCancel(context.Background())
	monitor.Start(ctx, t, true, gpb.SubscriptionList_STREAM)
	// start tests
	testGroup := &sync.WaitGroup{}
	// start reset/apply config
	runner.RunTestInBackground(ctx, t, time.NewTimer(10*time.Millisecond), testGroup, eventConsumer, testGNMISet)
	// start gribi test writter
	runner.RunTestInBackground(ctx, t, time.NewTimer(1*time.Second), testGroup, eventConsumer, testBatchADDReplaceDeleteIPV4)
	// start gnoi  ping
	runner.RunTestInBackground(ctx, t, time.NewTimer(10*time.Second), testGroup, eventConsumer, testPing)

	// start p4rt test

	time.Sleep(11 * time.Second) // wait until all the tests start (the timer period + 1)
	testGroup.Wait()
	cancelMonitors()
	time.Sleep(60 * time.Second)

	/* sample code to read from cache for the last events
	for key, val := range eventConsumer.Cache.Items() {
		ring := val.Object.(*ring.Ring)
		fmt.Printf("%s:%d\n", key, ring.Len())

	}
	fmt.Println(len(eventConsumer.Cache.Items()))
	*/

}
