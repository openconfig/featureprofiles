package backup_nhg_action_test

import (
	"context"
	"strings"
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
	"github.com/openconfig/gribigo/client"
)

const (
	ipv4PrefixLen      = 30
	ipv6PrefixLen      = 126
	mask               = "32"
	outerDstIP1        = "198.51.100.1"
	outerSrcIP1        = "198.51.100.2"
	outerDstIP2        = "203.0.113.1"
	outerSrcIP2        = "203.0.113.2"
	innerDstIP1        = "198.18.0.1"
	innerSrcIP1        = "198.18.0.255"
	vip1               = "198.18.1.1"
	vip2               = "198.18.1.2"
	vrfA               = "VRF-A"
	vrfB               = "VRF-B"
	vrfC               = "VRF-C"
	nh1ID              = 1
	nhg1ID             = 1
	nh2ID              = 2
	nhg2ID             = 2
	nh100ID            = 100
	nhg100ID           = 100
	nh101ID            = 101
	nhg101ID           = 101
	nh102ID            = 102
	nhg102ID           = 102
	nh103ID            = 103
	nhg103ID           = 103
	nh104ID            = 104
	nhg104ID           = 104
	baseSrcFlowFilter  = "0x02" // hexadecimal value of last 6 bits of src 198.51.100.2
	baseDstFlowFilter  = "0x31" // hexadecimal value of first 6 bits of dst 198.51.100.1
	encapSrcFlowFilter = "0x02" // hexadecimal value of last 6 bits of src 203.0.113.2
	encapDstFlowFilter = "0x32" // hexadecimal value of first 6 bits of dst 203.0.113.1
	decapSrcFlowFliter = "0x3f" // hexadecimal value of last 6 bits of src 198.18.0.255
	decapDstFlowFliter = "0x31" // hexadecimal value of first 6 bits of dst 198.18.0.1
	ethernetCsmacd     = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	policyID           = "match-ipip"
	ipOverIPProtocol   = 4
	srcTrackingName    = "ipSrcTracking"
	dstTrackingName    = "ipDstTracking"
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

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.13",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:d",
		IPv6Len: ipv6PrefixLen,
	}

	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		MAC:     "02:00:04:01:01:01",
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
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	top := gosnappi.NewConfig()
	for p, ap := range atePorts {
		p1 := ate.Port(t, p)
		dp := dutPorts[p]
		ap.AddToOTG(top, p1, &dp)
	}
	return top
}

// configureDUT configures port1, port2, port3, port4 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	for p, dp := range dutPorts {
		p1 := dut.Port(t, p)
		gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dp.NewOCInterface(p1.Name(), dut))

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
}

// TE11.3 backup nhg action tests.
func TestBackupNHGAction(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)

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
	if deviations.BackupNHGRequiresVrfWithDecap(dut) {
		d := &oc.Root{}
		ni := d.GetOrCreateNetworkInstance(vrfA)
		pf := ni.GetOrCreatePolicyForwarding()
		fp1 := pf.GetOrCreatePolicy(policyID)
		fp1.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
		fp1.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = oc.UnionUint8(ipOverIPProtocol)
		fp1.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String(vrfA)
		p1 := dut.Port(t, "port1")
		intf := pf.GetOrCreateInterface(p1.Name())
		intf.ApplyVrfSelectionPolicy = ygot.String(policyID)
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfA).PolicyForwarding().Config(), pf)
	}

	addStaticRoute(t, dut)

	ate.OTG().StartProtocols(t)

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
	nh, nhOpResult := gribi.NHEntry(nh1ID, atePort2.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, nhgOpResult := gribi.NHGEntry(nhg1ID, map[uint64]uint64{nh1ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg},
		[]*client.OpResult{nhOpResult, nhgOpResult})
	args.client.AddIPv4(t, vip1+"/"+mask, nhg1ID, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding NHG %d with NH %d as decap and DEFAULT vrf lookup via gRIBI", nhg100ID, nh100ID)
	nh, nhOpResult = gribi.NHEntry(nh100ID, "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB,
		&gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	nhg, nhgOpResult = gribi.NHGEntry(nhg100ID, map[uint64]uint64{nhg100ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg},
		[]*client.OpResult{nhOpResult, nhgOpResult})

	t.Logf("Adding NHG %d NH %d and  NH as %v  and backup NHG %d via gRIBI", nhg101ID, nh101ID, vip1, nhg100ID)
	nh, nhOpResult = gribi.NHEntry(nh101ID, vip1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, nhgOpResult = gribi.NHGEntry(nhg101ID, map[uint64]uint64{nh101ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg100ID})
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg},
		[]*client.OpResult{nhOpResult, nhgOpResult})
	t.Logf("Adding an IPv4Entry for %s in VRF %s with primary atePort2, backup as Decap via gRIBI", outerDstIP1, vrfA)
	args.client.AddIPv4(t, outerDstIP1+"/"+mask, nhg101ID, vrfA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Create flows with dst %s for each path", outerDstIP1)
	baseFlow := createFlow(t, args.ate, args.top, "BaseFlow", &atePort2)
	decapFlow := createFlow(t, args.ate, args.top, "DecapFlow", &atePort4)
	t.Run("ValidatePrimaryPath", func(t *testing.T) {
		t.Log("Validate primary path traffic recieved ate port2 and no traffic on decap flow/port4")
		validateTrafficFlows(t, args.ate, []gosnappi.Flow{baseFlow}, []gosnappi.Flow{decapFlow}, baseSrcFlowFilter, baseDstFlowFilter)
	})
	t.Log("Shutdown Port2")
	p2 := args.dut.Port(t, "port2")
	if deviations.ATEPortLinkStateOperationsUnsupported(args.ate) {
		setDUTInterfaceWithState(t, args.dut, &dutPort2, p2, false)
		defer setDUTInterfaceWithState(t, args.dut, &dutPort2, p2, true)
	} else {
		portStateAction := gosnappi.NewControlState()
		linkState := portStateAction.Port().Link().SetPortNames([]string{"port2"}).SetState(gosnappi.StatePortLinkState.DOWN)
		args.ate.OTG().SetControlState(t, portStateAction)
		// Restore port state at end of test case.
		linkState.SetState(gosnappi.StatePortLinkState.UP)
		defer args.ate.OTG().SetControlState(t, portStateAction)
	}

	t.Log("Capture port2 status if down")
	gnmi.Await(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_DOWN)
	operStatus := gnmi.Get(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_DOWN; operStatus != want {
		t.Errorf("Get(DUT port2 oper status): got %v, want %v", operStatus, want)
	}
	t.Run("ValidateDecapPath", func(t *testing.T) {
		t.Log("Validate Decap traffic recieved port 4 and no traffic on primary flow/port 2")
		validateTrafficFlows(t, args.ate, []gosnappi.Flow{decapFlow}, []gosnappi.Flow{baseFlow}, decapSrcFlowFliter, decapDstFlowFliter)
	})
}

// TE11.3 - case 2: new tunnel viability triggers decap in the backup NHG.
func testDecapEncap(ctx context.Context, t *testing.T, args *testArgs) {

	t.Logf("Adding VIP1 %v/32 with NHG %d NH %d and  atePort2 via gRIBI", vip1, nhg1ID, nh1ID)
	nh, nhOpResult := gribi.NHEntry(nh1ID, atePort2.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, nhgOpResult := gribi.NHGEntry(nhg1ID, map[uint64]uint64{nh1ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg},
		[]*client.OpResult{nhOpResult, nhgOpResult})
	args.client.AddIPv4(t, vip1+"/"+mask, nhg1ID, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding VIP2 %v/32 with NHG %d , NH %d and  atePort3 via gRIBI", vip2, nhg2ID, nh2ID)
	nh, nhOpResult = gribi.NHEntry(nh2ID, atePort3.IPv4, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, nhgOpResult = gribi.NHGEntry(nhg2ID, map[uint64]uint64{nh2ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg},
		[]*client.OpResult{nhOpResult, nhgOpResult})
	args.client.AddIPv4(t, vip2+"/"+mask, nhg2ID, deviations.DefaultNetworkInstance(args.dut), deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding NHG %d with NH %d as redirect to vrfB via gRIBI", nhg100ID, nh100ID)
	nh, nhOpResult = gribi.NHEntry(nh100ID, "VRFOnly", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfB})
	nhg, nhgOpResult = gribi.NHGEntry(nhg100ID, map[uint64]uint64{nh100ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg},
		[]*client.OpResult{nhOpResult, nhgOpResult})

	t.Logf("Adding NHG %d NH %d with  %v  and backup NHG %d via gRIBI", nhg101ID, nh101ID, vip1, nhg100ID)
	nh, nhOpResult = gribi.NHEntry(nh101ID, vip1, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, nhgOpResult = gribi.NHGEntry(nhg101ID, map[uint64]uint64{nh101ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg100ID})
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg},
		[]*client.OpResult{nhOpResult, nhgOpResult})

	t.Logf("Adding an IPv4Entry for %s in VRF %s with primary VIP1, backup as VRF %s  via gRIBI", outerDstIP1, vrfA, vrfB)
	args.client.AddIPv4(t, outerDstIP1+"/"+mask, nhg101ID, vrfA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding NHG %d with NH %d as decap and DEFAULT vrf lookup via gRIBI", nhg103ID, nh103ID)
	nh, nhOpResult = gribi.NHEntry(nh103ID, "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	nhg, nhgOpResult = gribi.NHGEntry(nhg103ID, map[uint64]uint64{nh103ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg},
		[]*client.OpResult{nhOpResult, nhgOpResult})

	t.Logf("Adding NHG %d NH %d and  NH as %v  and backup NHG %d via gRIBI", nhg104ID, nh104ID, vip2, nhg103ID)
	nh, nhOpResult = gribi.NHEntry(nh104ID, vip2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	nhg, nhgOpResult = gribi.NHGEntry(nhg104ID, map[uint64]uint64{nh104ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg103ID})
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg},
		[]*client.OpResult{nhOpResult, nhgOpResult})

	t.Logf("Adding an IPv4Entry for %s in vrf %s with NHG %d via gRIBI", outerDstIP2, vrfC, nhg104ID)
	args.client.AddIPv4(t, outerDstIP2+"/"+mask, nhg104ID, vrfC, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	t.Logf("Adding NHG %d NH %d and  NH as decap and encap with destination vrf as %v and backup NHG %d via gRIBI", nhg102ID, nh102ID, vrfC, nhg103ID)
	nh, nhOpResult = gribi.NHEntry(nh102ID, "DecapEncap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: outerSrcIP2, Dest: outerDstIP2, VrfName: vrfC})
	nhg, nhgOpResult = gribi.NHGEntry(nhg102ID, map[uint64]uint64{nh102ID: 1}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: nhg103ID})
	args.client.AddEntries(t, []fluent.GRIBIEntry{nh, nhg},
		[]*client.OpResult{nhOpResult, nhgOpResult})

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
		validateTrafficFlows(t, args.ate, []gosnappi.Flow{baseFlow}, []gosnappi.Flow{encapFLow, decapFLow}, baseSrcFlowFilter, baseDstFlowFilter)
	})

	t.Log("Shutdown Port2")
	dutP2 := args.dut.Port(t, "port2")
	if deviations.ATEPortLinkStateOperationsUnsupported(args.ate) {
		setDUTInterfaceWithState(t, args.dut, &dutPort2, dutP2, false)
		defer setDUTInterfaceWithState(t, args.dut, &dutPort2, dutP2, true)
	} else {
		portStateAction := gosnappi.NewControlState()
		linkState := portStateAction.Port().Link().SetPortNames([]string{"port2"}).SetState(gosnappi.StatePortLinkState.DOWN)
		args.ate.OTG().SetControlState(t, portStateAction)
		// Restore port state at end of test case.
		linkState.SetState(gosnappi.StatePortLinkState.UP)
		defer args.ate.OTG().SetControlState(t, portStateAction)
	}

	t.Log("Capture port2 status if down")
	gnmi.Await(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_DOWN)
	operStatusAfterShut := gnmi.Get(t, args.dut, gnmi.OC().Interface(p2.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_DOWN; operStatusAfterShut != want {
		t.Errorf("Get(DUT port2 oper status): got %v, want %v", operStatus, want)
	}
	t.Run("ValidateDecapEncapPath", func(t *testing.T) {
		t.Log("Validate traffic with encap header recieved on port 3 and no traffic on other flows/ate ports")
		validateTrafficFlows(t, args.ate, []gosnappi.Flow{encapFLow}, []gosnappi.Flow{baseFlow, decapFLow}, encapSrcFlowFilter, encapDstFlowFilter)
	})

	t.Log("Shutdown Port3")
	dutP3 := args.dut.Port(t, "port3")
	if deviations.ATEPortLinkStateOperationsUnsupported(args.ate) {
		setDUTInterfaceWithState(t, args.dut, &dutPort3, dutP3, false)
		defer setDUTInterfaceWithState(t, args.dut, &dutPort3, dutP3, true)
	} else {
		portStateAction := gosnappi.NewControlState()
		linkState := portStateAction.Port().Link().SetPortNames([]string{"port3"}).SetState(gosnappi.StatePortLinkState.DOWN)
		args.ate.OTG().SetControlState(t, portStateAction)
		// Restore port state at end of test case.
		linkState.SetState(gosnappi.StatePortLinkState.UP)
		defer args.ate.OTG().SetControlState(t, portStateAction)
	}

	t.Log("Capture port3 status if down")
	p3 := args.dut.Port(t, "port3")
	gnmi.Await(t, args.dut, gnmi.OC().Interface(p3.Name()).OperStatus().State(), 1*time.Minute, oc.Interface_OperStatus_DOWN)
	operStatusAfterShut = gnmi.Get(t, args.dut, gnmi.OC().Interface(p3.Name()).OperStatus().State())
	if want := oc.Interface_OperStatus_DOWN; operStatusAfterShut != want {
		t.Errorf("Get(DUT port3 oper status): got %v, want %v", operStatus, want)
	}
	t.Run("ValidateDecapPath", func(t *testing.T) {
		t.Log("Validate traffic after decap is recieved on port4 and no traffic on other flows/ate ports")
		validateTrafficFlows(t, args.ate, []gosnappi.Flow{decapFLow}, []gosnappi.Flow{baseFlow, encapFLow}, decapSrcFlowFliter, decapDstFlowFliter)
	})
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
	ipTracking := flow.EgressPacket().Add().Ipv4()
	ipSrcTracking := ipTracking.Src().MetricTags().Add()
	ipSrcTracking.SetName(srcTrackingName).SetOffset(26).SetLength(6)
	ipDstTracking := ipTracking.Dst().MetricTags().Add()
	ipDstTracking.SetName(dstTrackingName).SetOffset(0).SetLength(6)

	return flow
}

// TODO: Egress Tracking to verify the correctness of packet after decap or encap needs to be added
// validateTrafficFlows verifies that the flow on ATE, traffic should pass for good flow and fail for bad flow.
func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, good []gosnappi.Flow, bad []gosnappi.Flow, srcFlowFilter string, dstFlowFilter string) {
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
		etPath := gnmi.OTG().Flow(flow.Name()).TaggedMetricAny()
		ets := gnmi.GetAll(t, ate.OTG(), etPath.State())
		if got := len(ets); got != 1 {
			t.Errorf("EgressTracking got %d items, want %d", got, 1)
			return
		}
		for _, et := range ets {
			tags := et.Tags
			for _, tag := range tags {
				if tag.GetTagName() == srcTrackingName {
					if got := tag.GetTagValue().GetValueAsHex(); !strings.EqualFold(got, srcFlowFilter) {
						t.Errorf("EgressTracking filter got %q, want %q", got, srcFlowFilter)
					}
				}
				if tag.GetTagName() == dstTrackingName {
					if got := tag.GetTagValue().GetValueAsHex(); !strings.EqualFold(got, dstFlowFilter) {
						t.Errorf("EgressTracking filter got %q, want %q", got, dstFlowFilter)
					}
				}
			}
		}
		if got := ets[0].GetCounters().GetInPkts(); got != uint64(inPkts) {
			t.Errorf("EgressTracking counter in-pkts got %d, want %d", got, uint64(inPkts))
		} else {
			t.Logf("Received %d packets with %s as the last 6 bits of the src IP and %s as first 6 bits of dst IP ", got, srcFlowFilter, dstFlowFilter)
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
func setDUTInterfaceWithState(t testing.TB, dut *ondatra.DUTDevice, dutPort *attrs.Attributes, p *ondatra.Port, state bool) {
	dc := gnmi.OC()
	i := &oc.Interface{}
	i.Enabled = ygot.Bool(state)
	i.Type = ethernetCsmacd
	i.Name = ygot.String(p.Name())
	gnmi.Update(t, dut, dc.Interface(p.Name()).Config(), i)
}
