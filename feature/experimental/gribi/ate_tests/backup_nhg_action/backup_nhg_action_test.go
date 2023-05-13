package backup_nhg_action_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
)

const (
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	mask            = "32"
	outerDstIP1     = "198.51.100.1"
	outerSrcIP1     = "198.51.100.2"
	outerDstIP2     = "203.0.113.1"
	outerSrcIP2     = "203.0.113.2"
	innerDstIP1     = "198.18.0.1"
	innerSrcIP1     = "198.18.0.255"
	vip1            = "198.18.1.1"
	vip2            = "198.18.1.2"
	vrfA            = "VRF-A"
	vrfB            = "VRF-B"
	vrfC            = "VRF-C"
	nh1ID           = 1
	nhg1ID          = 1
	nh2ID           = 2
	nhg2ID          = 2
	nh100ID         = 100
	nhg100ID        = 100
	nh101ID         = 101
	nhg101ID        = 101
	nh102ID         = 102
	nhg102ID        = 102
	nh103ID         = 103
	nhg103ID        = 103
	nh104ID         = 104
	nhg104ID        = 104
	baseFlowFilter  = "710" // decimal value of last seven bits of src and first eight bits of dst
	encapFlowFilter = "715"
	decapFlowFliter = "32710"
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    *ondatra.ATETopology
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
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:a",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.13",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:d",
		IPv6Len: ipv6PrefixLen,
	}

	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		IPv4:    "192.0.2.14",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:e",
		IPv6Len: ipv6PrefixLen,
	}
	atePorts = map[string]attrs.Attributes{
		"port1": atePort1,
		"port2": atePort2,
		"port3": atePort3,
		"port4": atePort4,
	}
	dutPorts = map[string]attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
		"port3": dutPort3,
		"port4": dutPort4,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureATE configures ports on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()
	for p, ap := range atePorts {
		p1 := ate.Port(t, p)
		i1 := top.AddInterface(ap.Name).WithPort(p1)
		i1.IPv4().
			WithAddress(ap.IPv4CIDR()).
			WithDefaultGateway(dutPorts[p].IPv4)
		i1.IPv6().
			WithAddress(ap.IPv6CIDR()).
			WithDefaultGateway(dutPorts[p].IPv6)
	}
	return top
}

// configureDUT configures port1, port2, port3, port4 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	for p, dp := range dutPorts {
		p1 := dut.Port(t, p)
		gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dp.NewOCInterface(p1.Name()))

		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, p1)
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) && p != "port1" {
			fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}

}

// addStaticRoute configures static route.
func addStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	s := &oc.Root{}
	static := s.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	ipv4Nh := static.GetOrCreateStatic(innerDstIP1 + "/" + mask).GetOrCreateNextHop("0")
	ipv4Nh.NextHop, _ = ipv4Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(atePort4.IPv4)
	gnmi.Update(t, dut, d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// configureNetworkInstance configures vrfs vrfA,vrfB,vrfC and adds port1 to  vrfA
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	c := &oc.Root{}
	vrfs := []string{vrfA, vrfB, vrfC}
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
	if deviations.ExplicitGRIBIUnderNetworkInstance(dut) {
		for _, vrf := range []string{vrfA, vrfB, vrfC, deviations.DefaultNetworkInstance(dut)} {
			fptest.EnableGRIBIUnderNetworkInstance(t, dut, vrf)
		}
	}
}

// TE11.3 backup nhg action tests.
func TestBackupNHGAction(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	// Configure DUT
	if !deviations.InterfaceConfigVRFBeforeAddress(dut) {
		configureDUT(t, dut)
	}

	dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	configureNetworkInstance(t, dut)

	// For interface configuration, Arista prefers config Vrf first then the IP address
	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		configureDUT(t, dut)
	}

	addStaticRoute(t, dut)

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	test := []struct {
		name string
		desc string
		fn   func(ctx context.Context, t *testing.T, args *testArgs)
	}{
		{
			name: "testBackupDecap",
			desc: "Usecase with 2 NHOP Groups - Backup Pointing to Decap",
			fn:   testBackupDecap,
		},
		{
			name: "testDecapEncap",
			desc: "Usecase with 3 NHOP Groups - Redirect pointing to back up DecapEncap and its Backup Pointing to Decap",
			fn:   testDecapEncap,
		},
	}
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
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)
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
			tt.fn(ctx, t, tcArgs)
		})
	}
}

// TE11.3 - case 1: next-hop viability triggers decap in backup NHG.
func testBackupDecap(ctx context.Context, t *testing.T, args *testArgs) {

	t.Logf("Adding VIP %v/32 with NHG %d NH %d and  atePort2 via gRIBI", vip1, nhg1ID, nh1ID)
	args.client.AddNH(t, nh1ID, atePort2.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, nhg1ID, map[uint64]uint64{nh1ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, vip1+"/"+mask, nhg1ID, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding NHG %d with NH %d as decap and DEFAULT vrf lookup via gRIBI", nhg100ID, nh100ID)
	args.client.AddNH(t, nh100ID, "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, nhg100ID, map[uint64]uint64{nh100ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding NHG %d NH %d and  NH as %v  and backup NHG %d via gRIBI", nhg101ID, nh101ID, vip1, nhg100ID)
	args.client.AddNH(t, nh101ID, vip1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, nhg101ID, map[uint64]uint64{nh101ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg100ID})

	t.Logf("Adding an IPv4Entry for %s in VRF %s with primary atePort2, backup as Decap via gRIBI", outerDstIP1, vrfA)
	args.client.AddIPv4(t, outerDstIP1+"/"+mask, nhg101ID, vrfA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Create flows with dst %s for each path", outerDstIP1)
	baseFlow := createFlow(t, args.ate, args.top, "BaseFlow", &atePort2)
	decapFlow := createFlow(t, args.ate, args.top, "DecapFlow", &atePort4)
	t.Run("ValidatePrimaryPath", func(t *testing.T) {
		t.Log("Validate primary path traffic recieved ate port2 and no traffic on decap flow/port4")
		validateTrafficFlows(t, args.ate, []*ondatra.Flow{baseFlow}, []*ondatra.Flow{decapFlow}, baseFlowFilter)
	})
	t.Log("Shutdown Port2")
	ateP := args.ate.Port(t, "port2")
	args.ate.Actions().NewSetPortState().WithPort(ateP).WithEnabled(false).Send(t)
	defer args.ate.Actions().NewSetPortState().WithPort(ateP).WithEnabled(true).Send(t)

	p2 := args.dut.Port(t, "port2")
	t.Log("Capture port2 status if down")
	gnmi.Await(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_DOWN)
	operStatus := gnmi.Get(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_DOWN; operStatus != want {
		t.Errorf("Get(DUT port2 oper status): got %v, want %v", operStatus, want)
	}
	t.Run("ValidateDecapPath", func(t *testing.T) {
		t.Log("Validate Decap traffic recieved port 4 and no traffic on primary flow/port 2")
		validateTrafficFlows(t, args.ate, []*ondatra.Flow{decapFlow}, []*ondatra.Flow{baseFlow}, decapFlowFliter)
	})
}

// TE11.3 - case 2: new tunnel viability triggers decap in the backup NHG.
func testDecapEncap(ctx context.Context, t *testing.T, args *testArgs) {

	t.Logf("Adding VIP1 %v/32 with NHG %d NH %d and  atePort2 via gRIBI", vip1, nhg1ID, nh1ID)
	args.client.AddNH(t, nh1ID, atePort2.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, nhg1ID, map[uint64]uint64{nh1ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, vip1+"/"+mask, nhg1ID, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding VIP2 %v/32 with NHG %d , NH %d and  atePort3 via gRIBI", vip2, nhg2ID, nh2ID)
	args.client.AddNH(t, nh2ID, atePort3.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, nhg2ID, map[uint64]uint64{nh2ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, vip2+"/"+mask, nhg2ID, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding NHG %d with NH %d as redirect to vrfB via gRIBI", nhg100ID, nh100ID)
	args.client.AddNH(t, nh100ID, "VRFOnly", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfB})
	args.client.AddNHG(t, nhg100ID, map[uint64]uint64{nh100ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding NHG %d NH %d with  %v  and backup NHG %d via gRIBI", nhg101ID, nh101ID, vip1, nhg100ID)
	args.client.AddNH(t, nh101ID, vip1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, nhg101ID, map[uint64]uint64{nh101ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg100ID})

	t.Logf("Adding an IPv4Entry for %s in VRF %s with primary VIP1, backup as VRF %s  via gRIBI", outerDstIP1, vrfA, vrfB)
	args.client.AddIPv4(t, outerDstIP1+"/"+mask, nhg101ID, vrfA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding NHG %d with NH %d as decap and DEFAULT vrf lookup via gRIBI", nhg103ID, nh103ID)
	args.client.AddNH(t, nh103ID, "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, nhg103ID, map[uint64]uint64{nh103ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding NHG %d NH %d and  NH as %v  and backup NHG %d via gRIBI", nhg104ID, nh104ID, vip2, nhg103ID)
	args.client.AddNH(t, nh104ID, vip2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddNHG(t, nhg104ID, map[uint64]uint64{nh104ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg103ID})

	t.Logf("Adding an IPv4Entry for %s in vrf %s with NHG %d via gRIBI", outerDstIP2, vrfC, nhg104ID)
	args.client.AddIPv4(t, outerDstIP2+"/"+mask, nhg104ID, vrfC, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding NHG %d NH %d and  NH as decap and encap with destination vrf as %v and backup NHG %d via gRIBI", nhg102ID, nh102ID, vrfC, nhg103ID)
	args.client.AddNH(t, nh102ID, "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: outerSrcIP2, Dest: outerDstIP2, VrfName: vrfC})
	args.client.AddNHG(t, nhg102ID, map[uint64]uint64{nh102ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg103ID})

	t.Logf("Adding an IPv4Entry for %s in vrf %s with decap and encap destiantion  in  %s via gRIBI", outerDstIP1, vrfB, vrfC)
	args.client.AddIPv4(t, outerDstIP1+"/"+mask, nhg102ID, vrfB, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	p2 := args.dut.Port(t, "port2")
	t.Log("Capture port2 status if Up")
	gnmi.Await(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
	operStatus := gnmi.Get(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_UP; operStatus != want {
		t.Errorf("Get(DUT port2 oper status): got %v, want %v", operStatus, want)
	}

	t.Logf("Create flows with dst %s", outerDstIP1)
	baseFlow := createFlow(t, args.ate, args.top, "BaseFlow", &atePort2)
	encapFLow := createFlow(t, args.ate, args.top, "DecapEncapFlow", &atePort3)
	decapFLow := createFlow(t, args.ate, args.top, "DecapFlow", &atePort4)

	t.Run("ValidatePrimaryPath", func(t *testing.T) {
		t.Logf("Validate Primary path traffic recieved on port 2 and no traffic on other flows/ate ports")
		validateTrafficFlows(t, args.ate, []*ondatra.Flow{baseFlow}, []*ondatra.Flow{encapFLow, decapFLow}, baseFlowFilter)
	})

	t.Log("Shutdown Port2")
	ateP := args.ate.Port(t, "port2")
	args.ate.Actions().NewSetPortState().WithPort(ateP).WithEnabled(false).Send(t)
	defer args.ate.Actions().NewSetPortState().WithPort(ateP).WithEnabled(true).Send(t)

	t.Log("Capture port2 status if down")
	gnmi.Await(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_DOWN)
	operStatusAfterShut := gnmi.Get(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_DOWN; operStatusAfterShut != want {
		t.Errorf("Get(DUT port2 oper status): got %v, want %v", operStatus, want)
	}
	t.Run("ValidateDecapEncapPath", func(t *testing.T) {
		t.Log("Validate traffic with encap header recieved on port 3 and no traffic on other flows/ate ports")
		validateTrafficFlows(t, args.ate, []*ondatra.Flow{encapFLow}, []*ondatra.Flow{baseFlow, decapFLow}, encapFlowFilter)
	})
	t.Log("Shutdown Port3")
	ateP3 := args.ate.Port(t, "port3")
	args.ate.Actions().NewSetPortState().WithPort(ateP3).WithEnabled(false).Send(t)
	defer args.ate.Actions().NewSetPortState().WithPort(ateP3).WithEnabled(true).Send(t)

	t.Log("Capture port3 status if down")
	p3 := args.dut.Port(t, "port3")
	gnmi.Await(t, args.dut, gnmi.OC().Interface(p3.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_DOWN)
	operStatusAfterShut = gnmi.Get(t, args.dut, gnmi.OC().Interface(p3.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_DOWN; operStatusAfterShut != want {
		t.Errorf("Get(DUT port3 oper status): got %v, want %v", operStatus, want)
	}
	t.Run("ValidateDecapPath", func(t *testing.T) {
		t.Log("Validate traffic after decap is recieved on port4 and no traffic on other flows/ate ports")
		if !deviations.SecondaryBackupPathTrafficFailover(args.dut) {
			validateTrafficFlows(t, args.ate, []*ondatra.Flow{decapFLow}, []*ondatra.Flow{baseFlow, encapFLow}, decapFlowFliter)
		}
	})
}

// createFlow returns a flow from atePort1 to the dstPfx.
func createFlow(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, name string, dst *attrs.Attributes) *ondatra.Flow {
	srcEndPoint := top.Interfaces()[atePort1.Name]
	dstEndPoint := top.Interfaces()[dst.Name]
	outerIpv4Header := ondatra.NewIPv4Header()
	outerIpv4Header.WithSrcAddress(outerSrcIP1).DstAddressRange().WithMin(outerDstIP1).WithCount(1)
	innerIpv4Header := ondatra.NewIPv4Header()
	innerIpv4Header.WithSrcAddress(innerSrcIP1)
	innerIpv4Header.DstAddressRange().WithMin(innerDstIP1).WithCount(1)
	flow := ate.Traffic().NewFlow(name).
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint).
		WithHeaders(ondatra.NewEthernetHeader(), outerIpv4Header, innerIpv4Header).WithFrameSize(300)
	flow.EgressTracking().WithOffset(233).WithWidth(15)
	return flow
}

// validateTrafficFlows verifies that the flow on ATE, traffic should pass for good flow and fail for bad flow.
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, good []*ondatra.Flow, bad []*ondatra.Flow, flowFilter string) {
	flows := append(good, bad...)
	ate.Traffic().Start(t, flows...)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	for _, flow := range good {
		flowPath := gnmi.OC().Flow(flow.Name())
		if got := gnmi.Get(t, ate, flowPath.LossPct().State()); got > 0 {
			t.Fatalf("LossPct for flow %s: got %g, want 0", flow.Name(), got)
		}
		etPath := flowPath.EgressTrackingAny()
		ets := gnmi.GetAll(t, ate, etPath.State())
		if got := len(ets); got != 1 {
			t.Errorf("EgressTracking got %d items, want %d", got, 1)
			return
		}
		if got := ets[0].GetFilter(); got != flowFilter {
			t.Errorf("EgressTracking filter got %q, want %q", got, flowFilter)
		}
		inPkts := gnmi.Get(t, ate, flowPath.State()).GetCounters().GetInPkts()
		if got := ets[0].GetCounters().GetInPkts(); got != inPkts {
			t.Errorf("EgressTracking counter in-pkts got %d, want %d", got, inPkts)
		}
	}

	for _, flow := range bad {
		if got := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).LossPct().State()); got < 100 {
			t.Fatalf("LossPct for flow %s: got %g, want 100", flow.Name(), got)
		}
	}
}
