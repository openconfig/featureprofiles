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
	outerSrcIP1     = "199.51.100.1"
	outerDstIP2     = "111.111.111.111"
	outerSrcIP2     = "222.222.222.222"
	innerDstIP1     = "98.51.100.1"
	innerSrcIP1     = "99.51.100.1"
	VIP1            = "100.1.1.1"
	VIP2            = "101.1.1.1"
	vrf1            = "VRF-A"
	vrf2            = "VRF-B"
	vrf3            = "VRF-C"
	NH1ID           = 1
	NHG1ID          = 1
	NH2ID           = 2
	NHG2ID          = 2
	NH100ID         = 100
	NHG100ID        = 100
	NH101ID         = 101
	NHG101ID        = 101
	NH102ID         = 102
	NHG102ID        = 102
	NH103ID         = 103
	NHG103ID        = 103
	NH104ID         = 104
	NHG104ID        = 104
	baseFlowFilter  = "454" // last seven bits of src and first eight bits of dst
	encapFlowFilter = "24175"
	decapFlowFliter = "354"
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

	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name()))

	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name()))

	p3 := dut.Port(t, "port3")
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name()))

	p4 := dut.Port(t, "port4")
	gnmi.Replace(t, dut, d.Interface(p4.Name()).Config(), dutPort4.NewOCInterface(p4.Name()))

	if *deviations.ExplicitPortSpeed {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
		fptest.SetPortSpeed(t, p4)
	}
	if *deviations.ExplicitInterfaceInDefaultVRF {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p4.Name(), *deviations.DefaultNetworkInstance, 0)
	}

}

// addStaticRoute configures static route.
func addStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	s := &oc.Root{}
	static := s.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName)
	ipv4Nh := static.GetOrCreateStatic(innerDstIP1 + "/" + mask).GetOrCreateNextHop("0")
	ipv4Nh.NextHop, _ = ipv4Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(atePort4.IPv4)
	ipv4Nh := static.GetOrCreateStatic(innerdstPfx + "/" + mask).GetOrCreateNextHop("0")
	ipv4Nh.NextHop, _ = ipv4Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(atePort3.IPv4)
	gnmi.Update(t, dut, d.NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName).Config(), static)
}

// configureNetworkInstance configures vrfs vrf1,vrf2,vrf3 and adds port1 to  vrf1
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	c := &oc.Root{}
	vrfs := []string{vrf1, vrf2, vrf3}
	for _, vrf := range vrfs {
		ni := c.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		if vrf == vrf1 {
			p1 := dut.Port(t, "port1")
			niIntf := ni.GetOrCreateInterface(p1.Name())
			niIntf.Subinterface = ygot.Uint32(0)
			niIntf.Interface = ygot.String(p1.Name())
		}
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}
	if *deviations.ExplicitGRIBIUnderNetworkInstance {
		for _, vrf := range []string{vrf1, vrf2, vrf3, *deviations.DefaultNetworkInstance} {
			fptest.EnableGRIBIUnderNetworkInstance(t, dut, vrf)
		}
	}

	ni := c.GetOrCreateNetworkInstance(vrfName)
	ni.Description = ygot.String("Non Default routing instance created for testing")
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	p1 := dut.Port(t, "port1")
	niIntf := ni.GetOrCreateInterface(p1.Name())
	niIntf.Subinterface = ygot.Uint32(0)
	niIntf.Interface = ygot.String(p1.Name())
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), ni)
	if *deviations.ExplicitGRIBIUnderNetworkInstance {
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, vrfName)
		fptest.EnableGRIBIUnderNetworkInstance(t, dut, *deviations.DefaultNetworkInstance)
	}
}

// TE11.3 backup nhg action tests.
func TestBackupNHGAction(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	// Configure DUT
	if !*deviations.InterfaceConfigVrfBeforeAddress {
		configureDUT(t, dut)
	}

	dutConfNIPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance)
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	configureNetworkInstance(t, dut)

	// For interface configuration, Arista prefers config Vrf first then the IP address
	if *deviations.InterfaceConfigVrfBeforeAddress {
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
			desc: "Usecase3 with 2 NHOP Groups - Backup Pointing to Decap",
			fn:   testBackupDecap,
		},
		{
			name: "testDecapEncap",
			desc: "Usecase3 with 3 NHOP Groups - Redirect pointing to back up DecapEncap and its Backup Pointing to Decap",
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

	t.Logf("Adding VIP %v/32 with NHG %d NH %d and  atePort2 via gRIBI", VIP1, NHG1ID, NH1ID)
	args.client.AddNH(t, NH1ID, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddNHG(t, NHG1ID, map[uint64]uint64{NH1ID: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddIPv4(t, VIP1+"/"+mask, NHG1ID, *deviations.DefaultNetworkInstance, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Adding NHG %d with NH %d as decap and DEFAULT vrf lookup via gRIBI", NHG100ID, NH100ID)
	args.client.AddNH(t, NH100ID, "Decap", *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHOptions{VrfName: *deviations.DefaultNetworkInstance})
	args.client.AddNHG(t, NHG100ID, map[uint64]uint64{NH100ID: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Adding NHG %d NH %d and  NH as %v  and backup NHG %d via gRIBI", NHG101ID, NH101ID, VIP1, NHG100ID)
	args.client.AddNH(t, NH101ID, VIP1, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddNHG(t, NHG101ID, map[uint64]uint64{NH101ID: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: NHG100ID})

	t.Logf("Adding an IPv4Entry for %s in VRF %s with primary atePort2, backup as Decap via gRIBI", outerDstIP1, vrf1)
	args.client.AddIPv4(t, outerDstIP1+"/"+mask, NHG101ID, vrf1, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Create flows with dst %s", outerDstIP1)
	baseFlow := createFlow(t, args.ate, args.top, "BaseFlow", &atePort2)
	decapFlow := createFlow(t, args.ate, args.top, "DecapFlow", &atePort4)
	t.Run("ValidatePrimaryPath", func(t *testing.T) {
		t.Log("Validate primary path traffic passes through Port2")
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
		t.Log("Validate traffic passes through ATE port4 after decap")
		validateTrafficFlows(t, args.ate, []*ondatra.Flow{decapFlow}, []*ondatra.Flow{baseFlow}, decapFlowFliter)
	})
}

// TE11.3 - case 2: new tunnel viability triggers decap in the backup NHG.
func testDecapEncap(ctx context.Context, t *testing.T, args *testArgs) {

	t.Logf("Adding VIP %v/32 with NHG %d NH %d and  atePort2 via gRIBI", VIP1, NHG1ID, NH1ID)
	args.client.AddNH(t, NH1ID, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddNHG(t, NHG1ID, map[uint64]uint64{NH1ID: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddIPv4(t, VIP1+"/"+mask, NHG1ID, *deviations.DefaultNetworkInstance, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Adding VIP %v/32 with NHG %d , NH %d and  atePort3 via gRIBI", VIP2, NHG2ID, NH2ID)
	args.client.AddNH(t, NH2ID, atePort3.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddNHG(t, NHG2ID, map[uint64]uint64{NH2ID: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddIPv4(t, VIP2+"/"+mask, NHG2ID, *deviations.DefaultNetworkInstance, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Adding NHG %d with NH %d as decap and DEFAULT vrf lookup via gRIBI", NHG100ID, NH100ID)
	args.client.AddNH(t, NH100ID, "Vrf", *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrf2})
	args.client.AddNHG(t, NHG100ID, map[uint64]uint64{NH100ID: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Adding NHG %d NH %d with  %v  and backup NHG %d via gRIBI", NHG101ID, NH101ID, VIP1, NHG100ID)
	args.client.AddNH(t, NH101ID, VIP1, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddNHG(t, NHG101ID, map[uint64]uint64{NH101ID: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: NHG100ID})

	t.Logf("Adding an IPv4Entry for %s in VRF %s with primary VIP1, backup as VRF %s  via gRIBI", outerDstIP1, vrf1, vrf2)
	args.client.AddIPv4(t, outerDstIP1+"/"+mask, NHG101ID, vrf1, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Adding NHG %d with NH %d as decap and DEFAULT vrf lookup via gRIBI", NHG103ID, NH103ID)
	args.client.AddNH(t, NH103ID, "Decap", *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHOptions{VrfName: *deviations.DefaultNetworkInstance})
	args.client.AddNHG(t, NHG103ID, map[uint64]uint64{NH103ID: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Adding NHG %d NH %d and  NH as decap and encap with destination vrf as %v and backup NHG %d via gRIBI", NHG102ID, NH102ID, vrf3, NHG103ID)
	args.client.AddNH(t, NH102ID, "DecapEncap", *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHOptions{Src: outerSrcIP2, Dest: outerDstIP2, VrfName: vrf3})
	args.client.AddNHG(t, NHG102ID, map[uint64]uint64{NH102ID: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: NHG103ID})

	t.Logf("Adding an IPv4Entry for %s in vrf %s with decap and encap destiantion  in  %s via gRIBI", outerDstIP1, vrf2, vrf3)
	args.client.AddIPv4(t, outerDstIP1+"/"+mask, NHG102ID, vrf2, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Adding NHG %d NH %d and  NH as %v  and backup NHG %d via gRIBI", NHG104ID, NH104ID, VIP2, NHG103ID)
	args.client.AddNH(t, NH104ID, VIP2, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddNHG(t, NHG104ID, map[uint64]uint64{NH104ID: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: NHG103ID})

	t.Logf("Adding an IPv4Entry for %s in vrf %s with NHG %d via gRIBI", outerDstIP2, vrf3, NHG104ID)
	args.client.AddIPv4(t, outerDstIP2+"/"+mask, NHG104ID, vrf3, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

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
		t.Logf("Validate Primary path traffic passes through port2")
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
		t.Log("Validate traffic passes through port3 with encap header")
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
		t.Log("Validate traffic passes through port4 with decap and inner header lookup")
		validateTrafficFlows(t, args.ate, []*ondatra.Flow{decapFLow}, []*ondatra.Flow{baseFlow, encapFLow}, decapFlowFliter)
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
