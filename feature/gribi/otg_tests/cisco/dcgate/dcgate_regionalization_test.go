package dcgate_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
}

var bgpNbr1 = bgpNeighbor{
	as:         ateAS,
	neighborip: otgPort1.IPv4,
	isV4:       true,
}
var bgpNbr2 = bgpNeighbor{
	as:         dutAS,
	neighborip: otgPort2.IPv4,
	isV4:       true,
}

func bgpCreateNbr(t *testing.T, localAs uint32, dut *ondatra.DUTDevice, nbrs []*bgpNeighbor) *oc.NetworkInstance_Protocol {
	// t.Helper()
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutPort2.IPv4)
	global.As = ygot.Uint32(localAs)

	for _, nbr := range nbrs {
		nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
		nv4.PeerAs = ygot.Uint32(nbr.as)
		nv4.Enabled = ygot.Bool(true)

		global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

		if nbr.isV4 == true {
			af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			rpl := af4.GetOrCreateApplyPolicy()
			rpl.ImportPolicy = []string{rplName}
			rpl.ExportPolicy = []string{rplName}
		} else {
			af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			rpl := af6.GetOrCreateApplyPolicy()
			rpl.ImportPolicy = []string{rplName}
			rpl.ExportPolicy = []string{rplName}
		}
	}
	return niProto
}

var (
	otgPort1V4Peer = dutPort1.IPv4
	otgPort1V6Peer = dutPort1.IPv6
	otgPort2V4Peer = dutPort2.IPv4
	otgPort2V6Peer = dutPort2.IPv6
	ateAS          = uint32(65001)
	dutAS          = uint32(65000)
)

func configureOTGBgp(t *testing.T, otg *otg.OTG, topo gosnappi.Config, otgPeerList []string) {
	otgBgpRtr := make(map[string]gosnappi.DeviceBgpRouter)
	for _, d := range topo.Devices().Items() {
		fmt.Println(" device item :", d.Name())
		switch d.Name() {
		case otgPort1.Name:
			otgBgpRtr[otgPort1.Name] = d.Bgp().SetRouterId(otgPort1.IPv4)
		case otgPort2.Name:
			otgBgpRtr[otgPort2.Name] = d.Bgp().SetRouterId(otgPort2.IPv4)
		}
	}
	// BGP seesion
	for _, peer := range otgPeerList {
		switch peer {
		case otgPort1V4Peer:
			iDut1Bgp4Peer := otgBgpRtr[otgPort1.Name].Ipv4Interfaces().Add().SetIpv4Name(otgPort1.Name + ".IPv4").Peers().Add().SetName(otgPort1V4Peer)
			iDut1Bgp4Peer.SetPeerAddress(dutPort1.IPv4).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
			iDut1Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
			configureBGPv4Routes(iDut1Bgp4Peer, otgPort1.IPv4, bgp4Routes1, startPrefix1, 10)
		case otgPort1V6Peer:
			iDut1Bgp6Peer := otgBgpRtr[otgPort1.Name].Ipv6Interfaces().Add().SetIpv6Name(otgPort1.Name + ".IPv6").Peers().Add().SetName(otgPort1V6Peer)
			iDut1Bgp6Peer.SetPeerAddress(dutPort1.IPv6).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
			iDut1Bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			iDut1Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
		case otgPort2V4Peer:
			iDut2Bgp4Peer := otgBgpRtr[otgPort2.Name].Ipv4Interfaces().Add().SetIpv4Name(otgPort2.Name + ".IPv4").Peers().Add().SetName(otgPort2V4Peer)
			iDut2Bgp4Peer.SetPeerAddress(dutPort2.IPv4).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
			iDut2Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
			configureBGPv4Routes(iDut2Bgp4Peer, otgPort2.IPv4, bgp4Routes2, startPrefix2, 10)
		case otgPort2V6Peer:
			iDut2Bgp6Peer := otgBgpRtr[otgPort2.Name].Ipv6Interfaces().Add().SetIpv6Name(otgPort2.Name + ".IPv6").Peers().Add().SetName(otgPort2V6Peer)
			iDut2Bgp6Peer.SetPeerAddress(dutPort2.IPv6).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
			iDut2Bgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			iDut2Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
		}
	}

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, topo)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
}

var (
	advertisedRoutesv4Prefix = uint32(32)
	rplName                  = "ALLOW"
	startPrefix1             = "201.0.0.1"
	startPrefix2             = "202.0.0.1"
	bgp4Routes1              = "BGPv4_1"
	bgp4Routes2              = "BGPv4_2"
	routeCount               = 10
	installedRoutes          = uint32(routeCount)
)

func configureBGPv4Routes(peer gosnappi.BgpV4Peer, ipv4 string, name string, prefix string, count uint32) {
	routes := peer.V4Routes().Add().SetName(name)
	routes.SetNextHopIpv4Address(ipv4).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	routes.Addresses().Add().
		SetAddress(prefix).
		SetPrefix(advertisedRoutesv4Prefix).
		SetCount(count)
}

func advertiseBGPRoutes(t *testing.T, conf gosnappi.Config, routeNames []string) {

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(routeNames).SetState(gosnappi.StateProtocolRouteState.ADVERTISE)
	otg.SetControlState(t, cs)

}

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func verifyRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Waiting for BGPv4 neighbor to establish...")

	compare := func(val *ygnmi.Value[uint32]) bool {
		c, ok := val.Val()
		return ok && c == installedRoutes
	}
	t.Log("Verifying BGP state")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range []string{otgPort1.IPv4, otgPort2.IPv4} {
		prefixes := statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
		if got, ok := gnmi.Watch(t, dut, prefixes.Received().State(), 2*time.Minute, compare).Await(t); !ok {
			t.Errorf("Received prefixes v4 mismatch: got %v, want %v", got, installedRoutes)
		}
		if got, ok := gnmi.Watch(t, dut, prefixes.Installed().State(), 2*time.Minute, compare).Await(t); !ok {
			t.Errorf("Installed prefixes v4 mismatch: got %v, want %v", got, installedRoutes)
		}
	}
}
func TestRegionalization(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut, true)
	defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config())

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	topo := configureOTG(t, ate)

	otg := ate.OTG()

	c := &oc.Root{}
	ni := c.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Config(), ni)

	configureRoutePolicy(t, dut, rplName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutConf := bgpCreateNbr(t, dutAS, dut, []*bgpNeighbor{&bgpNbr1, &bgpNbr2})
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)

	configureOTGBgp(t, otg, topo, []string{otgPort1V4Peer, otgPort2V4Peer})
	advertiseBGPRoutes(t, topo, []string{bgp4Routes1, bgp4Routes2})
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(otgPort1V4Peer)
	compare := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if ok {
			return state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}
		return false
	}
	gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, compare).Await(t)
	nbrPath = statePath.Neighbor(otgPort2V4Peer)
	gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, compare).Await(t)
	verifyRoutes(t, dut)
	// additional sleep time
	time.Sleep(time.Minute)
	// configure gRIBI client
	client := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}

	if err := client.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	defer client.Close(t)
	client.BecomeLeader(t)

	// Flush all existing AFT entries on the router
	client.FlushAll(t)

	args := &testArgs{
		client: &client,
		dut:    dut,
		ate:    ate,
		topo:   topo,
	}

	oSrcIp := faTransit.src
	oDstIp := faTransit.dst
	faTransit.src = ipv4OuterSrc222
	faTransit.dst = tunnelDstIP1
	defer func() { faTransit.src = oSrcIp; faTransit.dst = oDstIp }()

	// match in decap vrf, decap traffic and schedule to match in encap vrf
	args.client.AddNH(t, decapNH(1), "Decap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: vrfEncapA})
	args.client.AddNHG(t, decapNHG(1), map[uint64]uint64{decapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(tunnelDstIP1, 32), decapNHG(1), vrfDecap, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// encap start
	args.client.AddNH(t, encapNH(1), "Encap", deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{Src: ipv4OuterSrc111, Dest: tunnelDstIP2, VrfName: vrfTransit})
	args.client.AddNHG(t, encapNHG(1), map[uint64]uint64{encapNH(1): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// inner hdr prefixes
	args.client.AddIPv4(t, cidr(innerV4DstIP, 32), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(InnerV6DstIP, 128), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	args.client.AddIPv4(t, cidr(ipv4EntryPrefix, ipv4EntryPrefixLen), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	args.client.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), encapNHG(1), vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

	// // configure repair path with backup
	// configureVIP3NHGWithRepairTunnelHavingBackupDecapAction(t, args)

	// transit path
	configureVIP2BGPPrefix(t, args, startPrefix2)
	args.client.AddNH(t, vipNH(2), vipIP2, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHOptions{VrfName: deviations.DefaultNetworkInstance(args.dut)})
	args.client.AddNHG(t, vipNHG(2), map[uint64]uint64{vipNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)
	// args.client.AddNHG(t, vipNHG(2), map[uint64]uint64{vipNH(2): 100}, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB, &gribi.NHGOptions{BackupNHG: tunNHG(3)})
	args.client.AddIPv4(t, cidr(tunnelDstIP2, 32), vipNHG(2), vrfTransit, deviations.DefaultNetworkInstance(args.dut), fluent.InstalledInFIB)

	// verify traffic passes through primary NHG
	weights := []float64{1, 0, 0, 0}
	testTransitTrafficWithDscp(t, args, weights, dscpEncapA1, true)

	// check cluster traffic also passes -TERS_03
	testTraffic(t, args, weights, true)
}
