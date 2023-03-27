package backup_nhg_action_test

import (
	"context"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
)

const (
	ipv4PrefixLen        = 30
	ipv6PrefixLen        = 126
	innerdstPfx          = "203.0.113.55"
	mask                 = "32"
	primaryTunnelDstIP   = "203.0.113.1"
	secondaryTunnelDstIP = "203.0.113.70"
	secondaryTunnelSrcIP = "198.51.100.222"
	vrfName              = "VRF-1"
	NH1ID                = 1
	NH2ID                = 2
	NH3ID                = 3
	innersrcPfx          = "198.51.100.1"
	ethernetCsmacd       = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
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
		Name:    "port1",
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
		Name:    "port2",
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
		Name:    "port3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:a",
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureATE configures ports on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := ate.OTG().NewConfig(t)

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort2.AddToOTG(top, p2, &dutPort2)
	atePort3.AddToOTG(top, p3, &dutPort3)

	return top
}

// configureDUT configures port1, port2, port3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name()))

	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name()))

	p3 := dut.Port(t, "port3")
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutPort3.NewOCInterface(p3.Name()))

	if *deviations.ExplicitPortSpeed {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
	}
	if *deviations.ExplicitInterfaceInDefaultVRF {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), *deviations.DefaultNetworkInstance, 0)
	}
}

// addStaticRoute configures static route.
func addStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	s := &oc.Root{}
	static := s.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName)
	ipv4Nh := static.GetOrCreateStatic(innerdstPfx + "/" + mask).GetOrCreateNextHop("0")
	ipv4Nh.NextHop, _ = ipv4Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(atePort3.IPv4)
	gnmi.Update(t, dut, d.NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName).Config(), static)
}

// configureNetworkInstance configures vrf VRF-1 and adds the vrf to port1.
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	c := &oc.Root{}
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
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

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
			desc: "Usecase3 with 2 NHOP Groups - Primary DecapEncap, Backup Pointing to Decap",
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

	t.Logf("Adding NH %d with atePort2 via gRIBI", NH1ID)
	args.client.AddNH(t, NH1ID, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	t.Logf("Adding NH %d as decap and NHGs %d, %d via gRIBI", NH2ID, NH1ID, NH2ID)
	args.client.AddNH(t, NH2ID, "Decap", *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddNHG(t, NH2ID, map[uint64]uint64{NH2ID: 100}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddNHG(t, NH1ID, map[uint64]uint64{NH1ID: 100}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: NH2ID})
	t.Logf("Adding an IPv4Entry for %s with primary atePort2, backup as Decap via gRIBI", primaryTunnelDstIP)
	args.client.AddIPv4(t, primaryTunnelDstIP+"/"+mask, NH1ID, vrfName, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Create flows with dst %s", primaryTunnelDstIP)
	baselineFlow := createFlow(args.ate, "BaseFlow", primaryTunnelDstIP, &atePort2)
	backupFlow := createFlow(args.ate, "BackupFlow", primaryTunnelDstIP, &atePort3)
	t.Log("Validate traffic passes")
	validateTrafficFlows(t, args.ate, args.top, baselineFlow, backupFlow)

	// Setting admin state down on the DUT interface.
	// Setting the otg interface down has no effect on kne and is not yet supported in otg
	t.Log("Shutdown Port2")
	p2 := args.dut.Port(t, "port2")
	setDUTInterfaceWithState(t, args.dut, &dutPort2, p2, false)
	defer setDUTInterfaceWithState(t, args.dut, &dutPort2, p2, true)

	t.Log("Capture port2 status if down")
	gnmi.Await(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_DOWN)
	operStatus := gnmi.Get(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_DOWN; operStatus != want {
		t.Errorf("Get(DUT port2 oper status): got %v, want %v", operStatus, want)
	}

	t.Log("Validate traffic passes through port3")
	validateTrafficFlows(t, args.ate, args.top, backupFlow, baselineFlow)
}

// TE11.3 - case 2: new tunnel viability triggers decap in the backup NHG.
func testDecapEncap(ctx context.Context, t *testing.T, args *testArgs) {

	t.Logf("Adding NH %d with atePort2 via gRIBI", NH3ID)
	args.client.AddNH(t, NH3ID, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	t.Logf("Adding NHG %d via gRIBI", NH3ID)
	args.client.AddNHG(t, NH3ID, map[uint64]uint64{NH3ID: 100}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	t.Logf("Adding an IPv4Entry for %s pointing to atePort2 via gRIBI", secondaryTunnelDstIP)
	args.client.AddIPv4(t, secondaryTunnelDstIP+"/"+mask, NH3ID, vrfName, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Adding NH %d as decap and NHG %d via gRIBI", NH2ID, NH2ID)
	args.client.AddNH(t, NH2ID, "Decap", *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddNHG(t, NH2ID, map[uint64]uint64{NH2ID: 100}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Adding NH %d as DecapEncap and NHG %d via gRIBI", NH1ID, NH1ID)
	args.client.AddNH(t, NH1ID, "DecapEncap", *deviations.DefaultNetworkInstance, fluent.InstalledInRIB, &gribi.NHOptions{Src: secondaryTunnelSrcIP, Dest: secondaryTunnelDstIP, VrfName: vrfName})
	args.client.AddNHG(t, NH1ID, map[uint64]uint64{NH1ID: 100}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: NH2ID})
	t.Logf("Adding an IPv4Entry for %s with primary DecapEncap & backup decap via gRIBI", primaryTunnelDstIP)
	args.client.AddIPv4(t, primaryTunnelDstIP+"/"+mask, NH1ID, vrfName, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	p2 := args.dut.Port(t, "port2")
	t.Log("Capture port2 status if Up")
	gnmi.Await(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_UP)
	operStatus := gnmi.Get(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_UP; operStatus != want {
		t.Errorf("Get(DUT port2 oper status): got %v, want %v", operStatus, want)
	}

	t.Logf("Create flows with dst %s", primaryTunnelDstIP)
	baselineFlow := createFlow(args.ate, "BaseFlow", primaryTunnelDstIP, &atePort2)
	backupFlow := createFlow(args.ate, "BackupFlow", primaryTunnelDstIP, &atePort3)
	t.Logf("Validate traffic passes through port2")
	validateTrafficFlows(t, args.ate, args.top, baselineFlow, backupFlow)

	t.Log("Shutdown Port2")
	// Setting admin state down on the DUT interface.
	// Setting the otg interface down has no effect on kne and is not yet supported in otg
	dutP2 := args.dut.Port(t, "port2")
	setDUTInterfaceWithState(t, args.dut, &dutPort2, dutP2, false)
	defer setDUTInterfaceWithState(t, args.dut, &dutPort2, dutP2, true)

	t.Log("Capture port2 status if down")
	gnmi.Await(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_DOWN)
	operStatusAfterShut := gnmi.Get(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_DOWN; operStatusAfterShut != want {
		t.Errorf("Get(DUT port2 oper status): got %v, want %v", operStatus, want)
	}

	t.Log("Validate traffic passes through port3")
	validateTrafficFlows(t, args.ate, args.top, backupFlow, baselineFlow)
}

// createFlow returns a flow from atePort1 to the dstPfx.
func createFlow(ate *ondatra.ATEDevice, name string, dstPfx string, dst *attrs.Attributes) gosnappi.Flow {

	flow := gosnappi.NewFlow().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{dst.Name + ".IPv4"})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	outerIPHeader := flow.Packet().Add().Ipv4()
	outerIPHeader.Src().SetValue(atePort1.IPv4)
	outerIPHeader.Dst().Increment().SetStart(dstPfx).SetCount(1)
	innerIPHeader := flow.Packet().Add().Ipv4()
	innerIPHeader.Src().SetValue(innersrcPfx)
	innerIPHeader.Dst().Increment().SetStart(innerdstPfx).SetCount(1)
	flow.Size().SetFixed(300)

	return flow
}

// validateTrafficFlows verifies that the flow on ATE, traffic should pass for good flow and fail for bad flow.
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, good, bad gosnappi.Flow) {
	top.Flows().Clear()
	for _, flow := range []gosnappi.Flow{good, bad} {
		top.Flows().Append(flow)
	}
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.LogPortMetrics(t, ate.OTG(), top)
	outPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(good.Name()).Counters().OutPkts().State())
	inPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(good.Name()).Counters().InPkts().State())
	if outPkts == 0 {
		t.Fatalf("OutPkts for flow %s is 0, want >0.", good.Name())
	}

	if got := ((outPkts - inPkts) * 100) / outPkts; got > 0 {
		t.Fatalf("LossPct for flow %s: got %v, want 0", good.Name(), got)
	}

	outPkts = gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(bad.Name()).Counters().OutPkts().State())
	inPkts = gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(bad.Name()).Counters().InPkts().State())
	if outPkts == 0 {
		t.Fatalf("OutPkts for flow %s is 0, want >0.", bad.Name())
	}

	if got := ((outPkts - inPkts) * 100) / outPkts; got < 100 {
		t.Fatalf("LossPct for flow %s: got %v, want 100", bad.Name(), got)
	}
}

// setDUTInterfaceState sets the admin state on the dut interface
func setDUTInterfaceWithState(t testing.TB, dut *ondatra.DUTDevice, dutPort *attrs.Attributes, p *ondatra.Port, state bool) {
	dc := gnmi.OC()
	i := &oc.Interface{}
	i.Enabled = ygot.Bool(state)
	i.Type = ethernetCsmacd
	i.Name = ygot.String(p.Name())
	gnmi.Update(t, dut, dc.Interface(p.Name()).Config(), i)
}
