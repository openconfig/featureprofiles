package static_bgp_nexthop_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	bgpNHv4         = "203.0.200.1"
	bgpNHv6         = "2001:db8:128:200::1"
	mplsLabelV4     = 10004 // Arista not suppoting the range mentioned in readme
	mplsLabelV6     = 10006 // Arista not suppoting the range mentioned in readme
	ipv4Dst         = "203.0.113.0/24"
	ipv6Dst         = "2001:db8:128:128::/64"
	dutAS           = 65001
	ateAS           = 65002
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	rplType         = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	rplName         = "ALLOW"
	trafficPps      = 1000
	trafficSize     = 1500
	bgpV4PeerName   = "BGP-PEER-GROUP-V4-P2"
	bgpV6PeerName   = "BGP-PEER-GROUP-V6-P2"
	lspV4Name       = "lsp-egress-v4"
	lspV6Name       = "lsp-egress-v6"
	ipv4Flow        = "Ipv4_Mpls"
	ipv6Flow        = "Ipv6_Mpls"
	lossTolerance   = 2
	trafficDuration = 30
)

var (
	atePort1 = attrs.Attributes{
		Name:    "ateP1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "ateP2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort3 = attrs.Attributes{
		Name:    "ateP3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::10",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE destination-2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "DUT to ATE destination-3",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::9",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	timeout  = 1 * time.Minute
	interval = 20 * time.Second
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test cases:
//  1) Verify IPv4 MPLS forwarding.
//		- Push the above DUT configuration.
//		- Start traffic flow with MPLS[lbl-1000004] and IPv4 destined to IPV4-DST.
//		- Verify that traffic arrives to ATE Port 2.
//  2) Verify IPv6 MPLS forwarding.
// 	`	- Push the above DUT configuration.
// 		- Start traffic flow with MPLS[lbl-1000006] and IPv4 destined to IPV6-DST.
// 		- Verify that traffic arrives to ATE Port 2.
// 	3) Verify IPv4 traffic discard when BGP-NH is not available.
// 		- Withdraw BGP-NH-V4 advertisement.
// 		- Push the above DUT configuration.
// 		- Start traffic flow with MPLS[lbl-1000004] and IPv4 destination set to IPV4-DST.
// 		- Verify that traffic is discarded.
// 	4) Verify IPv6 traffic discard when BGP-NH is not available.
// 		- Withdraw BGP-NH-V6 advertisement.
// 		- Push the above DUT configuration.
// 		- Start traffic flow with MPLS[lbl-1000006] and IPv6 destination set to IPV6-DST.
// 		- Verify that traffic is discarded.
//  Details: https://github.com/openconfig/featureprofiles/blob/main/feature/mpls/otg_tests/static_bgp_nexthop/README.md
//

// TestMplsStaticLspBGPNextHop runs the verification steps
func TestMplsStaticLspBGPNextHop(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	configureMplsStaticLsps(t, dut)
	topo := configureATE(t, ate)
	verifyPortsUp(t, dut.Device)
	checkBgpStatus(t, dut)
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration * time.Second)
	ate.OTG().StopTraffic(t)
	t.Run("Verify IPv4 MPLS forwarding", func(t *testing.T) {
		if verifyFlowTraffic(t, ate, topo, ipv4Flow) {
			t.Log("IPv4 Traffic MPLS forwarding Passed")
		} else {
			t.Error("IPv4 Traffic MPLS forwarding Failed")
		}
	})
	t.Run("Verify IPv6 MPLS forwarding", func(t *testing.T) {
		if verifyFlowTraffic(t, ate, topo, ipv6Flow) {
			t.Log("IPv6 Traffic MPLS forwarding Passed")
		} else {
			t.Error("IPv6 Traffic MPLS forwarding Failed")
		}
	})
	t.Run("Verify IPv4 traffic discard when BGP-NH-V4 is not available.", func(t *testing.T) {
		withdrawBGPRoutes(t, []string{"BGP4.peer2.Route"})
		verifyIpv4TrafficDiscard(t, ate, topo)
	})
	t.Run("Verify IPv6 traffic discard when BGP-NH-V6 is not available.", func(t *testing.T) {
		withdrawBGPRoutes(t, []string{"BGP6.peer2.Route"})
		verifyIpv6TrafficDiscard(t, ate, topo)
	})
}

// configureDUT sets up the DUT interfaces, static LSPs, and BGP neighbors.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()
	// Configure interfaces
	p1 := dut.Port(t, "port1").Name()
	i1 := dutPort1.NewOCInterface(p1, dut)
	gnmi.Replace(t, dut, d.Interface(p1).Config(), i1)

	p2 := dut.Port(t, "port2").Name()
	i2 := dutPort2.NewOCInterface(p2, dut)
	gnmi.Replace(t, dut, d.Interface(p2).Config(), i2)

	p3 := dut.Port(t, "port3").Name()
	i3 := dutPort3.NewOCInterface(p3, dut)
	gnmi.Replace(t, dut, d.Interface(p3).Config(), i3)
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	configureRoutePolicy(t, dut, rplName, rplType)
	dutConfPath := d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutConf := createBGPNeighborPort2(dutAS, ateAS, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
}

// configureATE sets up the ATE interfaces and BGP configurations.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	topo := gosnappi.NewConfig()
	t.Log("Configure ATE interface")
	port1 := topo.Ports().Add().SetName("port1")
	port2 := topo.Ports().Add().SetName("port2")
	port3 := topo.Ports().Add().SetName("port3")

	port1Dev := topo.Devices().Add().SetName(atePort1.Name + ".dev")
	port1Eth := port1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	port1Eth.Connection().SetPortName(port1.Name())
	port1Ipv4 := port1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	port1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	port1Ipv6 := port1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	port1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	port2Dev := topo.Devices().Add().SetName(atePort2.Name + ".dev")
	port2Eth := port2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	port2Eth.Connection().SetPortName(port2.Name())
	port2Ipv4 := port2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	port2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	port2Ipv6 := port2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	port2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	port3Dev := topo.Devices().Add().SetName(atePort3.Name + ".dev")
	port3Eth := port3Dev.Ethernets().Add().SetName(atePort3.Name + ".Eth").SetMac(atePort3.MAC)
	port3Eth.Connection().SetPortName(port3.Name())
	port3Ipv4 := port3Eth.Ipv4Addresses().Add().SetName(atePort3.Name + ".IPv4")
	port3Ipv4.SetAddress(atePort3.IPv4).SetGateway(dutPort3.IPv4).SetPrefix(uint32(atePort3.IPv4Len))
	port3Ipv6 := port3Eth.Ipv6Addresses().Add().SetName(atePort3.Name + ".IPv6")
	port3Ipv6.SetAddress(atePort3.IPv6).SetGateway(dutPort3.IPv6).SetPrefix(uint32(atePort3.IPv6Len))
	// Configure BGP on ATE Port 2 to advertise next-hop prefixes
	bgproute := port2Dev.Bgp().SetRouterId(atePort2.IPv4)
	bgpPeerv4 := bgproute.Ipv4Interfaces().Add().SetIpv4Name(port2Ipv4.Name()).Peers().Add().SetName("BGP4.peer2")
	bgpPeerv4.SetPeerAddress(port2Ipv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	bgpPeerv6 := bgproute.Ipv6Interfaces().Add().SetIpv6Name(port2Ipv6.Name()).Peers().Add().SetName("BGP6.peer2")
	bgpPeerv6.SetPeerAddress(port2Ipv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	bgpPeerv4Routes := bgpPeerv4.V4Routes().Add().SetName("BGP4.peer2.Route")
	bgpPeerv4Routes.Addresses().Add().SetAddress(bgpNHv4).SetPrefix(24)

	bgpPeerv6Routes := bgpPeerv6.V6Routes().Add().SetName("BGP6.peer2.Route")
	bgpPeerv6Routes.Addresses().Add().SetAddress(bgpNHv6).SetPrefix(64)
	iPv4MPLSForwarding(t, topo, ipv4Flow)
	iPv6MPLSForwarding(t, topo, ipv6Flow)

	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	return topo
}

func withdrawBGPRoutes(t *testing.T, routeNames []string) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Failed to withdraw BGP routes: %v", r)
		}
	}()

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(routeNames).SetState(gosnappi.StateProtocolRouteState.WITHDRAW)
	otg.SetControlState(t, cs)
	t.Log("BGP routes withdrawn successfully")
}

// Add IPv4 MPLS Forwarding Flow
func iPv4MPLSForwarding(t *testing.T, config gosnappi.Config, flowname string) {
	dut := ondatra.DUT(t, "dut")
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	flow := config.Flows().Add()
	flow.SetName(flowname)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName(config.Ports().Items()[0].Name()).SetRxNames([]string{config.Ports().Items()[1].Name()})
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(atePort1.MAC)
	eth.Dst().SetValue(macAddress)

	mpls := flow.Packet().Add().Mpls()
	mpls.Label().SetValue(mplsLabelV4)

	ip4 := flow.Packet().Add().Ipv4()
	ip4.Src().SetValue(atePort1.IPv4)
	ip4.Dst().SetValue(bgpNHv4)

	flow.Size().SetFixed(trafficSize)
	flow.Rate().SetPps(trafficPps)
}

// Add IPv6 MPLS Forwarding Flow
func iPv6MPLSForwarding(t *testing.T, config gosnappi.Config, flowname string) {
	dut := ondatra.DUT(t, "dut")
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	flow := config.Flows().Add()
	flow.SetName(flowname)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName(config.Ports().Items()[0].Name()).SetRxNames([]string{config.Ports().Items()[1].Name()})
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(atePort1.MAC)
	eth.Dst().SetValue(macAddress)
	mpls := flow.Packet().Add().Mpls()
	mpls.Label().SetValue(mplsLabelV6)
	ip6 := flow.Packet().Add().Ipv6()
	ip6.Src().SetValue(atePort1.IPv6)
	ip6.Dst().SetValue(bgpNHv6)
	ip6.Version().SetValue(4)
	flow.Size().SetFixed(trafficSize)
	flow.Rate().SetPps(trafficPps)
}

// Verify Ipv4 Traffic Discard
func verifyIpv4TrafficDiscard(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) {
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration * time.Second)
	ate.OTG().StopTraffic(t)
	if verifyFlowTraffic(t, ate, config, ipv4Flow) {
		t.Error("Receiving IPv4 packets, RX should be zero")
	} else {
		t.Log("IPv4 packets dropped, Test Passed")
	}
}

// Verify Ipv6 Traffic Discard
func verifyIpv6TrafficDiscard(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) {
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration * time.Second)
	ate.OTG().StopTraffic(t)
	if verifyFlowTraffic(t, ate, config, ipv6Flow) {
		t.Error("Receiving IPv6 packets, RX should be zero")
	} else {
		t.Log("IPv6 packets dropped, Test Passed")
	}
}

func verifyFlowTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flowName string) bool {
	t.Log("Verify Flow Traffic")
	startTime := time.Now()
	count := 0
	var got float64
	for time.Since(startTime) < timeout {

		otgutils.LogFlowMetrics(t, ate.OTG(), config)
		countersPath := gnmi.OTG().Flow(flowName).Counters()
		framesRx := gnmi.Get(t, ate.OTG(), countersPath.InPkts().State())
		framesTx := gnmi.Get(t, ate.OTG(), countersPath.OutPkts().State())

		if got = (math.Abs(float64(framesTx)-float64(framesRx)) * 100) / float64(framesTx); got <= lossTolerance {
			return true
		} else {
			time.Sleep(interval)
			count += 1
		}
	}

	if count >= 2 {
		t.Logf("Packet loss percentage for flow: got %v, want %v", got, lossTolerance)
		return false
	}
	return true
}

// Enable MPLS Forwarding
func configureMplsStaticLsps(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.StaticMplsLspUnsupported(dut) {
		helpers.GnmiCLIConfig(t, dut, "mpls ip")
		setStaticMplsIpv4 := fmt.Sprintf("mpls static top-label %v %s pop payload-type ipv4", mplsLabelV4, bgpNHv4)
		helpers.GnmiCLIConfig(t, dut, setStaticMplsIpv4)
		setStaticMplsIpv6 := fmt.Sprintf("mpls static top-label %v %s pop payload-type ipv6", mplsLabelV6, bgpNHv6)
		helpers.GnmiCLIConfig(t, dut, setStaticMplsIpv6)
	} else {
		configureMplsStaticLsp(t, dut, lspV4Name, mplsLabelV4, bgpNHv4)
		configureMplsStaticLsp(t, dut, lspV6Name, mplsLabelV6, bgpNHv6)
	}
}

// configreRoutePolicy adds route-policy config
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

// Configure BGP Neighbors on DUT
func createBGPNeighborPort2(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(localAs)
	global.RouterId = ygot.String(dutPort2.IPv4)

	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	pgv4 := bgp.GetOrCreatePeerGroup(bgpV4PeerName)
	pgv4.PeerAs = ygot.Uint32(peerAs)
	pgv4.PeerGroupName = ygot.String(bgpV4PeerName)
	pgv6 := bgp.GetOrCreatePeerGroup(bgpV6PeerName)
	pgv6.PeerAs = ygot.Uint32(peerAs)
	pgv6.PeerGroupName = ygot.String(bgpV6PeerName)
	nv4 := bgp.GetOrCreateNeighbor(atePort2.IPv4)
	nv4.PeerAs = ygot.Uint32(peerAs)
	nv4.Enabled = ygot.Bool(true)
	nv4.PeerGroup = ygot.String(bgpV4PeerName)
	afisafi := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afisafi.Enabled = ygot.Bool(true)
	nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(false)
	pgafv4 := pgv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	pgafv4.Enabled = ygot.Bool(true)
	rpl4 := pgafv4.GetOrCreateApplyPolicy()
	rpl4.ImportPolicy = []string{rplName}
	rpl4.ExportPolicy = []string{rplName}
	nv6 := bgp.GetOrCreateNeighbor(atePort2.IPv6)
	nv6.PeerAs = ygot.Uint32(peerAs)
	nv6.Enabled = ygot.Bool(true)
	nv6.PeerGroup = ygot.String(bgpV6PeerName)
	afisafi6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	afisafi6.Enabled = ygot.Bool(true)
	nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(false)
	pgafv6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	pgafv6.Enabled = ygot.Bool(true)
	rpl6 := pgafv6.GetOrCreateApplyPolicy()
	rpl6.ImportPolicy = []string{rplName}
	rpl6.ExportPolicy = []string{rplName}

	return niProto
}

// Validate BGP neighbor established
func checkBgpStatus(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP state")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(atePort2.IPv4)
	nbrPathv6 := statePath.Neighbor(atePort2.IPv6)

	// Get BGP adjacency state
	t.Log("Waiting for BGP neighbor to establish...")
	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		currState, ok := val.Val()
		return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		t.Fatal("BGP session not Established as expected...")
	}

	// Get BGPv6 adjacency state
	t.Log("Waiting for BGPv6 neighbor to establish...")
	_, ok = gnmi.Watch(t, dut, nbrPathv6.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		currState, ok := val.Val()
		return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, dut, nbrPathv6.State()))
		t.Fatal("BGP session not Established as expected...")
	}
}

// Verify ports status
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	t.Log("Verifying port status")
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

// Configure Static Lsp
func configureMplsStaticLsp(t *testing.T, dut *ondatra.DUTDevice, lspName string, incomingLabel uint32, nextHopIP string) {
	d := &oc.Root{}
	mplsCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateMpls()
	staticMplsCfg := mplsCfg.GetOrCreateLsps().GetOrCreateStaticLsp(lspName)

	staticMplsCfg.GetOrCreateEgress().SetIncomingLabel(oc.UnionUint32(incomingLabel))
	staticMplsCfg.GetOrCreateEgress().SetNextHop(nextHopIP)
	staticMplsCfg.GetOrCreateEgress().SetPushLabel(oc.Egress_PushLabel_IMPLICIT_NULL)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Config(), mplsCfg)
}
