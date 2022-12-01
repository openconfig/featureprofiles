package backup_nhg_action_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
)

const (
	ipv4PrefixLen        = 30
	ipv6PrefixLen        = 126
	innerdstPfx          = "201.1.0.1"
	dstPfxMin            = "203.0.113.0"
	dstPfxMask           = "24"
	mask                 = "32"
	primaryTunnelDstIP   = "198.51.100.1"
	secondaryTunnelDstIP = "10.1.0.1"
	secondaryTunnelSrcIP = "222.222.222.222"
	vrfName              = "VRF-1"
	NH1ID                = 1
	NH2ID                = 2
	NH3ID                = 3
	innersrcPfx          = "5.5.5.5"
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
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureATE configures ports on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)
	i1.IPv6().
		WithAddress(atePort1.IPv6CIDR()).
		WithDefaultGateway(dutPort1.IPv6)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)
	i2.IPv6().
		WithAddress(atePort2.IPv6CIDR()).
		WithDefaultGateway(dutPort2.IPv6)

	p3 := ate.Port(t, "port3")
	i3 := top.AddInterface(atePort3.Name).WithPort(p3)
	i3.IPv4().
		WithAddress(atePort3.IPv4CIDR()).
		WithDefaultGateway(dutPort3.IPv4)
	i3.IPv6().
		WithAddress(atePort3.IPv6CIDR()).
		WithDefaultGateway(dutPort3.IPv6)
	return top
}

// configureDUT configures port1, port2, port3 and port4 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	d.Interface(p1.Name()).Update(t, dutPort1.NewInterface(p1.Name()))

	p2 := dut.Port(t, "port2")
	d.Interface(p2.Name()).Replace(t, dutPort2.NewInterface(p2.Name()))

	p3 := dut.Port(t, "port3")
	d.Interface(p3.Name()).Replace(t, dutPort3.NewInterface(p3.Name()))

}

// Add static route
func addStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	s := &telemetry.Device{}
	static := s.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance).GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName)
	ipv4Nh := static.GetOrCreateStatic(innerdstPfx + "/" + mask).GetOrCreateNextHop(atePort3.IPv4)
	ipv4Nh.NextHop, _ = ipv4Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(atePort3.IPv4)
	dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName).Update(t, static)
}

// Add vrf VRF-1
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	c := &oc.Root{}
	ni := c.GetOrCreateNetworkInstance(vrfName)
	ni.Description = ygot.String("Non Default routing instance created for testing")
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	ni.Enabled = ygot.Bool(true)
	ni.EnabledAddressFamilies = []oc.E_Types_ADDRESS_FAMILY{oc.Types_ADDRESS_FAMILY_IPV4, oc.Types_ADDRESS_FAMILY_IPV6}
	p1 := dut.Port(t, "port1")
	niIntf := ni.GetOrCreateInterface(p1.Name())
	niIntf.Subinterface = ygot.Uint32(0)
	niIntf.Interface = ygot.String(p1.Name())
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), ni)
}

func TestBackupNHGAction(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	//configure DUT
	configureDUT(t, dut)
	configureNetworkInstance(t, dut)
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
			name: "testbackupDecap",
			desc: "Usecase3 with 2 NHOP Groups - Backup Pointing to Decap",
			fn:   testbackupDecap,
		},
		{
			name: "testDecapEncap",
			desc: "Usecase3 with 2 NHOP Groups - Primary DecapEncap, Backup Pointing to Decap",
			fn:   testDecapEncap,
		},
	}
	// Configure the gRIBI client client
	client := gribi.Client{
		DUT:         dut,
		FIBACK:      false,
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

func testbackupDecap(ctx context.Context, t *testing.T, args *testArgs) {

	t.Logf("Adding NH %d with ATE port-2 via gRIBI", NH1ID)
	args.client.AddNH(t, NH1ID, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	t.Logf("Adding NH %d as decap and NHGs %d, %d via gRIBI", NH2ID, NH1ID, NH2ID)
	args.client.AddNH(t, NH2ID, "Decap", *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddNHG(t, NH2ID, map[uint64]uint64{NH2ID: 100}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)
	args.client.AddNHG(t, NH1ID, map[uint64]uint64{NH1ID: 100}, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: NH2ID})
	t.Logf("Adding an IPv4Entry for %s with primary atePort2, backup as Decap via gRIBI", primaryTunnelDstIP)
	args.client.AddIPv4(t, primaryTunnelDstIP+"/"+mask, NH1ID, vrfName, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Create flow with dst %s", primaryTunnelDstIP)
	BaseFlow := createFlow(t, args.ate, args.top, "BaseFlow", primaryTunnelDstIP)
	t.Log("Validate traffic passes")
	validateTrafficFlows(t, args.ate, BaseFlow, false, []string{"port2"})

	t.Log("Shutdown Port2")
	ateP := args.ate.Port(t, "port2")
	args.ate.Actions().NewSetPortState().WithPort(ateP).WithEnabled(false).Send(t)
	defer args.ate.Actions().NewSetPortState().WithPort(ateP).WithEnabled(true).Send(t)
	t.Log("Validate traffic passes through port3")
	validateTrafficFlows(t, args.ate, BaseFlow, false, []string{"port3"})
}

func testDecapEncap(ctx context.Context, t *testing.T, args *testArgs) {

	t.Logf("Adding NH %d with ATE port-2 via gRIBI", NH3ID)
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
	t.Logf("an IPv4Entry for %s with primary DecapEncap & backup decap via gRIBI", primaryTunnelDstIP)
	args.client.AddIPv4(t, primaryTunnelDstIP+"/"+mask, NH1ID, vrfName, *deviations.DefaultNetworkInstance, fluent.InstalledInFIB)

	t.Logf("Create base flow with dst %s", primaryTunnelDstIP)
	BaseFlow := createFlow(t, args.ate, args.top, "Baseflow", primaryTunnelDstIP)
	t.Logf("Validate traffic passes through port2")
	validateTrafficFlows(t, args.ate, BaseFlow, false, []string{"port2"})

	t.Log("Shutdown Port2")
	ateP := args.ate.Port(t, "port2")
	args.ate.Actions().NewSetPortState().WithPort(ateP).WithEnabled(false).Send(t)
	defer args.ate.Actions().NewSetPortState().WithPort(ateP).WithEnabled(true).Send(t)

	t.Log("Validate traffic passes through port3")
	validateTrafficFlows(t, args.ate, BaseFlow, false, []string{"port3"})
}

// createFlow returns a flow from atePort1 to the dstPfx
func createFlow(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, name string, dstPfx string) *ondatra.Flow {
	srcEndPoint := top.Interfaces()[atePort1.Name]
	dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range top.Interfaces() {
		if intf != "atePort1" {
			dstEndPoint = append(dstEndPoint, intf_data)
		}
	}
	hdr := ondatra.NewIPv4Header()
	hdr.WithSrcAddress(dutPort1.IPv4).DstAddressRange().WithMin(dstPfx).WithCount(1)
	innerIpv4Header := ondatra.NewIPv4Header()
	innerIpv4Header.WithSrcAddress(innersrcPfx)
	innerIpv4Header.DstAddressRange().WithMin(innerdstPfx).WithCount(1)
	flow := ate.Traffic().NewFlow(name).
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...).
		WithHeaders(ondatra.NewEthernetHeader(), hdr, innerIpv4Header).WithFrameSize(300)
	return flow
}

// validateTrafficFlows verifies that the flow on ATE and check interface counters on DUT
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, flow *ondatra.Flow, drop bool, d_port []string) {
	ate.Traffic().Start(t, flow)
	time.Sleep(60 * time.Second)
	ate.Traffic().Stop(t)
	flowPath := ate.Telemetry().Flow(flow.Name())
	got := flowPath.LossPct().Get(t)
	if drop {
		if got != 100 {
			t.Fatalf("Traffic passing for flow %s got %f, want 100 percent loss", flow.Name(), got)
		}
	} else {
		if got > 0 {
			t.Fatalf("LossPct for flow %s got %f, want 0", flow.Name(), got)
		}
	}
}
