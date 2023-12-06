// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package programming_with_reload_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen  = 30
	ipv6PrefixLen  = 126
	mask           = "32"
	outerDstIP1    = "198.51.100.1"
	outerSrcIP1    = "198.51.100.2"
	outerDstIP2    = "203.0.113.1"
	outerSrcIP2    = "203.0.113.2"
	innerDstIP1    = "198.18.0.1"
	innerSrcIP1    = "198.18.0.255"
	vip1           = "198.18.1.1"
	vip2           = "198.18.1.2"
	vrfA           = "VRF-A"
	vrfB           = "VRF-B"
	nh1ID          = 1
	nhg1ID         = 1
	nh2ID          = 2
	nhg2ID         = 2
	nh100ID        = 100
	nhg100ID       = 100
	nh101ID        = 101
	nhg101ID       = 101
	nh102ID        = 102
	nhg102ID       = 102
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	// Maximum reboot time is 900 seconds (15 minutes).
	maxRebootTime = 900
	// Maximum gribi connection time is 180 seconds following reload
	maxGribiConnectTime = 180
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    gosnappi.Config
	ctx    context.Context
	client *gribi.Client
}

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:9",
		IPv6Len: ipv6PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:a",
		IPv6Len: ipv6PrefixLen,
	}

	atePorts = map[string]attrs.Attributes{
		"port1": atePort1,
		"port2": atePort2,
		"port3": atePort3,
	}
	dutPorts = map[string]attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
		"port3": dutPort3,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureATE configures ports on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()
	for p, ap := range atePorts {
		p1 := ate.Port(t, p)
		dp := dutPorts[p]
		ap.AddToOTG(top, p1, &dp)
	}
	return top
}

// configureDUT configures port1, port2, port3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	for p, dp := range dutPorts {
		p1 := dut.Port(t, p)
		gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dp.NewOCInterface(p1.Name(), dut))
	}

}

// configureNetworkInstance configures vrfs vrfA, vrfB and add port1 to vrfA
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	c := &oc.Root{}
	vrfs := []string{vrfA, vrfB}
	for _, vrf := range vrfs {
		ni := c.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		if vrf == vrfA {
			p1 := dut.Port(t, "port1")
			niIntf := ni.GetOrCreateInterface(p1.Name())
			niIntf.Subinterface = ygot.Uint32(0)
			niIntf.Interface = ygot.String(p1.Name())
		}
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}
}

// TE11.4 Gribi Programming with Reload.
func TestProgrammingWithReload(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)

	// Configure DUT
	configureDUT(t, dut)

	configureNetworkInstance(t, dut)

	addStaticRoute(t, dut, atePort3.IPv4)

	ate.OTG().StartProtocols(t)

	test := []struct {
		name string
		desc string
		fn   func(ctx context.Context, t *testing.T, args *testArgs)
	}{
		{
			name: "testGribiChain1",
			desc: "Usecase with DecapEncap in backup path",
			fn:   testGribiChain1,
		},
		{
			name: "testGribiChain2",
			desc: "Usecase with DecapEncap in primary path",
			fn:   testGribiChain2,
		},
	}
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			// Configure the gRIBI client
			client := gribi.Client{
				DUT:         dut,
				FIBACK:      true,
				Persistence: true,
			}
			defer client.Close(t)
			defer client.FlushAll(t)
			if err := client.Start(t); err != nil {
				t.Fatalf("gRIBI Connection can not be established")
			}

			// Flush past entries before running the tc
			client.BecomeLeader(t)
			client.FlushAll(t)
			tcArgs := &testArgs{
				ctx:    ctx,
				client: &client,
				dut:    dut,
				ate:    ate,
				top:    top,
			}

			// gribi programming for the case
			t.Run("Configure initial gribi programming for the case", func(t *testing.T) {
				tt.fn(ctx, t, tcArgs)
			})

			// validate traffic over primary path
			t.Run("Validate traffic over Primary Path", func(t *testing.T) {
				baseFlow := createFlow(t, tcArgs.ate, tcArgs.top, "BaseFlow", &atePort2)
				decapFlow := createFlow(t, tcArgs.ate, tcArgs.top, "DecapFlow", &atePort3)
				t.Log("Validate primary path traffic received ate port2 and no traffic on decap flow/port3")
				validateTrafficFlows(t, tcArgs.ate, []gosnappi.Flow{baseFlow}, []gosnappi.Flow{decapFlow})
			})

			// perform chassis reload and validate gribi connection is established
			t.Run("Perform chassis reload and validate gribi connection is established", func(t *testing.T) {
				tcArgs.reloadChassis(t)
			})

			// perform same gribi programming chain
			t.Run("Reprogram gribi ", func(t *testing.T) {
				tt.fn(ctx, t, tcArgs)
			})

			//shutdown primary interface
			t.Logf("Shutdown primary path")
			tcArgs.setDUTInterfaceWithState(t, tcArgs.dut.Port(t, "port2"), false)
			defer tcArgs.setDUTInterfaceWithState(t, tcArgs.dut.Port(t, "port2"), true)

			// validate traffic over backup path
			t.Run("Validate traffic over Backup Path", func(t *testing.T) {
				baseFlow := createFlow(t, tcArgs.ate, tcArgs.top, "BaseFlow", &atePort2)
				decapFlow := createFlow(t, tcArgs.ate, tcArgs.top, "DecapFlow", &atePort3)
				t.Log("Validate primary path traffic received ate port2 and no traffic on decap flow/port3")
				validateTrafficFlows(t, tcArgs.ate, []gosnappi.Flow{decapFlow}, []gosnappi.Flow{baseFlow})
			})
		})
	}
}

// TE11.4 - case 1
func testGribiChain1(ctx context.Context, t *testing.T, args *testArgs) {

	t.Logf("an IPv4Entry for VIP1 %s in DEFAULT pointing to ATE port-2 via gRIBI with NHG %d and NH %d", vip1, nhg1ID, nh1ID)
	nh, nhOpResult := gribi.NHEntry(nh1ID, atePort2.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, nhgOpResult := gribi.NHGEntry(nhg1ID, map[uint64]uint64{nh1ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{nhOpResult, nhgOpResult})
	args.client.AddIPv4(t, vip1+"/"+mask, nhg1ID, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("an IPv4Entry for prefix %s in %s pointing to VIP1 via gRIBI with NHG %d and NH %d", outerDstIP2, vrfB, nhg2ID, nh2ID)
	nh, nhOpResult = gribi.NHEntry(nh2ID, vip1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, nhgOpResult = gribi.NHGEntry(nhg2ID, map[uint64]uint64{nh2ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{nhOpResult, nhgOpResult})
	args.client.AddIPv4(t, outerDstIP2+"/"+mask, nhg2ID, vrfB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding NHG %d with NH %d as decap and DEFAULT vrf lookup via gRIBI for backup path", nhg101ID, nh101ID)
	nh, nhOpResult = gribi.NHEntry(nh101ID, "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	nhg, nhgOpResult = gribi.NHGEntry(nhg101ID, map[uint64]uint64{nh101ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{nhOpResult, nhgOpResult})

	t.Logf("Program a decap-encap pointing to NI %s as primary path for prefix %s via gRIBI", vrfB, outerDstIP1)
	nh, nhOpResult = gribi.NHEntry(nh100ID, "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: outerSrcIP2, Dest: outerDstIP2, VrfName: vrfB})
	nhg, nhgOpResult = gribi.NHGEntry(nhg100ID, map[uint64]uint64{nh100ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg101ID})
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{nhOpResult, nhgOpResult})
	args.client.AddIPv4(t, outerDstIP1+"/"+mask, nhg100ID, vrfA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

// TE11.4 - case 2
func testGribiChain2(ctx context.Context, t *testing.T, args *testArgs) {

	t.Logf("an IPv4Entry for VIP1 %s in DEFAULT pointing to ATE port-2 via gRIBI with NHG %d and NH %d", vip1, nhg1ID, nh1ID)
	nh, nhOpResult := gribi.NHEntry(nh1ID, atePort2.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, nhgOpResult := gribi.NHGEntry(nhg1ID, map[uint64]uint64{nh1ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{nhOpResult, nhgOpResult})
	args.client.AddIPv4(t, vip1+"/"+mask, nhg1ID, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Program a decap-encap pointing to NI %s via gRIBI with NHG %d and NH %d", vrfB, nhg101ID, nh101ID)
	nh, nhOpResult = gribi.NHEntry(nh101ID, "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: outerSrcIP2, Dest: outerDstIP2, VrfName: vrfB})
	nhg, nhgOpResult = gribi.NHGEntry(nhg101ID, map[uint64]uint64{nh101ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{nhOpResult, nhgOpResult})

	t.Logf("an IPv4Entry for %s in NI %s pointing to VIP %s via gRIBI with NHG %d and NH %d", outerDstIP1, vrfA, vip1, nhg100ID, nh100ID)
	nh, nhOpResult = gribi.NHEntry(nh100ID, vip1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, nhgOpResult = gribi.NHGEntry(nhg100ID, map[uint64]uint64{nh100ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg101ID})
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{nhOpResult, nhgOpResult})
	args.client.AddIPv4(t, outerDstIP1+"/"+mask, nhg100ID, vrfA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding VIP2 %v/32 with NHG %d , NH %d and  atePort3 via gRIBI", vip2, nhg2ID, nh2ID)
	nh, nhOpResult = gribi.NHEntry(nh2ID, atePort3.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, nhgOpResult = gribi.NHGEntry(nhg2ID, map[uint64]uint64{nh2ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg},
		[]*client.OpResult{nhOpResult, nhgOpResult})
	args.client.AddIPv4(t, vip2+"/"+mask, nhg2ID, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("an IPv4Entry for %s in %s pointing to %s via gRIBI with NHG %d and NH %d", outerDstIP2, vrfB, vip2, nhg102ID, nh102ID)
	nh, nhOpResult = gribi.NHEntry(nh102ID, vip2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, nhgOpResult = gribi.NHGEntry(nhg102ID, map[uint64]uint64{nh102ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{nhOpResult, nhgOpResult})
	args.client.AddIPv4(t, outerDstIP2+"/"+mask, nhg102ID, vrfB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
}

// createFlow returns a flow name from atePort1 to the dstPfx.
func createFlow(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, name string, dst *attrs.Attributes) gosnappi.Flow {
	flow := gosnappi.NewFlow().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{dst.Name + ".IPv4"})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	outerIPHeader := flow.Packet().Add().Ipv4()
	outerIPHeader.Src().SetValue(outerSrcIP1)
	outerIPHeader.Dst().Increment().SetStart(outerDstIP1).SetCount(1)
	innerIPHeader := flow.Packet().Add().Ipv4()
	innerIPHeader.Src().SetValue(innerSrcIP1)
	innerIPHeader.Dst().Increment().SetStart(innerDstIP1).SetCount(1)
	flow.EgressPacket().Add().Ethernet()
	return flow
}

// validateTrafficFlows verifies that the flow on ATE, traffic should pass for good flow and fail for bad flow.
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, good []gosnappi.Flow, bad []gosnappi.Flow) {
	top := ate.OTG().FetchConfig(t)
	top.Flows().Clear()
	for _, flow := range append(good, bad...) {
		top.Flows().Append(flow)
	}
	ate.OTG().PushConfig(t, top)

	ate.OTG().StartProtocols(t)
	ate.OTG().StartTraffic(t)

	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)
	time.Sleep(10 * time.Second)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)

	for _, flow := range good {
		outPkts := float32(gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))
		if outPkts == 0 {
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow)
		}
		if got := ((outPkts - inPkts) * 100) / outPkts; got > 0 {
			t.Fatalf("LossPct for flow %s: got %v, want 0", flow.Name(), got)
		}
	}

	for _, flow := range bad {
		outPkts := float32(gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))
		if outPkts == 0 {
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow)
		}
		if got := ((outPkts - inPkts) * 100) / outPkts; got < 100 {
			t.Fatalf("LossPct for flow %s: got %v, want 100", flow.Name(), got)
		}
	}
}

// setDUTInterfaceState sets the admin state on the dut interface
func (args *testArgs) setDUTInterfaceWithState(t testing.TB, p *ondatra.Port, state bool) {
	dc := gnmi.OC()
	i := &oc.Interface{}
	i.Enabled = ygot.Bool(state)
	i.Type = ethernetCsmacd
	i.Name = ygot.String(p.Name())
	// Following reload try gNMI call twice, Unavaiable error code is returned for the 1st time
	for {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Update(t, args.dut, dc.Interface(p.Name()).Config(), i)
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, retrying ...", *errMsg)
		} else {
			break
		}
	}
}

// addStaticRoute configures static route needed for decap path
func addStaticRoute(t *testing.T, dut *ondatra.DUTDevice, ip string) {
	d := gnmi.OC()
	s := &oc.Root{}
	static := s.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	ipv4Nh := static.GetOrCreateStatic(innerDstIP1 + "/" + mask).GetOrCreateNextHop("0")
	ipv4Nh.NextHop, _ = ipv4Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(ip)
	gnmi.Update(t, dut, d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// Reload chassis
func (args *testArgs) reloadChassis(t *testing.T) {

	gnoiClient, err := args.dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	bootTimeBeforeReboot := gnmi.Get(t, args.dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)

	rebootRequest := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis with delay",
		Force:   true,
	}

	t.Logf("Send reboot request: %v", rebootRequest)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	t.Logf("Got reboot response: %v, err: %v", rebootResponse, err)
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}

	// adding sleep time for reload to begin
	time.Sleep(1 * time.Minute)

	// establish new gribi programming following reload
	err_msg := args.gribi_reconnect(t)
	if err_msg != nil {
		t.Fatalf("Failure: %d", err_msg)
	}
}

// gribi reconnect following reload
func (args *testArgs) gribi_reconnect(t *testing.T) error {

	reboot_timer := time.Now()
	client := gribi.Client{
		DUT:         args.dut,
		FIBACK:      true,
		Persistence: true,
	}

	for {
		// Attempt starting gribi client session and inspect the error code while checking session is established within maxRebootTime
		err := client.Start(t)

		// set gribi connection timer variable to current value everytime client session is tried to established, so when return code is UNAVAILABLE
		// we can track 3 mins following the start time
		gribi_connection_timer := time.Now()

		// if error code is context deadline exceeded DUT is still rebooting and try again
		if errors.Is(err, context.DeadlineExceeded) && time.Since(reboot_timer).Seconds() < maxRebootTime {
			t.Logf("gRIBI Connection could not be established because device is still rebooting: %v\nRetrying...", err)
			time.Sleep(1 * time.Second)
		} else if strings.Contains(strings.ToLower(err.Error()), "unavailable") && time.Since(reboot_timer).Seconds() < maxRebootTime {
			// if error is UNAVAIABLE, try creating gribi session within maxGribiConnectTime
			for {
				if err := client.Start(t); err != nil {
					t.Logf("gRIBI Connection could not be established following reboot: %v\nRetrying...", err)
					time.Sleep(1 * time.Second)
				} else {
					t.Logf("New gRIBI Connection established after reload in %v seconds", time.Since(gribi_connection_timer).Seconds())
					args.client = &client
					args.client.BecomeLeader(t)
					return nil
				}
				if time.Since(gribi_connection_timer).Seconds() > maxGribiConnectTime {
					return fmt.Errorf("Router is up but gRIBI Connection could not be established within max timeout value")
				}
			}
		} else {
			return fmt.Errorf("gRIBI connection failed with error: %v after trying for %v minutes", err, time.Since(reboot_timer).Minutes())
		}
	}
}
