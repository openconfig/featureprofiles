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

package bgp_tcp_mss_path_mtu_test

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstbgp"
	"github.com/openconfig/ondatra/gnmi/oc/networkinstance"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// The testbed is TESTBED_DUT_1_LOOP_2_ATE:
//
//	ATE port-1 <-> DUT port-1 [TRANSIT] DUT port-2 <-> DUT port-3 [DEFAULT] DUT port-4 <-> ATE port-2
//
// BGP runs in the DEFAULT network instance. The L3VRF network instance TRANSIT
// routes between ATE port-1 and the loop link, so it is a router between the
// DUT BGP speaker (DUT port-3 addresses) and the iBGP peers on ATE port-1.
// That router is needed to test PMTUD with a single DUT.

const (
	transitNI   = "TRANSIT"
	bgpName     = "BGP"
	ebgpPG      = "EBGP-PEERS"
	ibgpPG      = "IBGP-PEERS"
	policyName  = "ACCEPT-ALL"
	dutAS       = 64500
	ateEBGPAS   = 64501
	plenIPv4    = 30
	plenIPv6    = 126
	mtuDefault  = 1500
	mtuJumbo    = 5040
	mtuPMTUD    = 1400
	mssNeighbor = uint16(4096)
	mssGroup    = uint16(4000)
	mssOverride = uint16(1400)
	// TCP default MSS values (RFC 9293 and RFC 8200).
	mssMinV4 = uint16(536)
	mssMinV6 = uint16(1220)
	// Header overhead of an IPv4 or IPv6 TCP segment without options.
	tcpIPv4Overhead = 40
	tcpIPv6Overhead = 60
	// The capture must show DUT packets larger than this, so the DUT uses the
	// 1400-byte path MTU and not a smaller default.
	minPMTUDPacket = 1280
	routeCount     = 2000
	v4RouteStart   = "100.64.0.0"
	v4RouteLen     = 24
	v6RouteStart   = "2001:db8:1::"
	v6RouteLen     = 64
	v4RouteName    = "ateP2.v4.routes"
	v6RouteName    = "ateP2.v6.routes"

	protocolTimeout = 3 * time.Minute
	resetWindow     = 30 * time.Second
)

var (
	atePort1 = attrs.Attributes{
		Name:    "ateP1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort1 = attrs.Attributes{
		Desc:    "DUT port-1 to ATE port-1 (TRANSIT)",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT port-2 loop to DUT port-3 (TRANSIT)",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::192:0:2:9",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "DUT port-3 loop to DUT port-2 (DEFAULT)",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::192:0:2:a",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort4 = attrs.Attributes{
		Desc:    "DUT port-4 to ATE port-2 (DEFAULT)",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort2 = attrs.Attributes{
		Name:    "ateP2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	// ATE port-1 subnets, routed by DEFAULT through the loop link.
	ateP1PrefixV4 = "192.0.2.0/30"
	ateP1PrefixV6 = "2001:db8::192:0:2:0/126"

	otgBGPPeers = []string{otgPeerName(atePort1, "v4"), otgPeerName(atePort1, "v6"), otgPeerName(atePort2, "v4"), otgPeerName(atePort2, "v6")}
)

// bgpNeighbor describes one DUT BGP neighbor.
type bgpNeighbor struct {
	address   string
	peerGroup string
	isV4      bool
}

var (
	ibgpV4 = bgpNeighbor{address: atePort1.IPv4, peerGroup: ibgpPG, isV4: true}
	ibgpV6 = bgpNeighbor{address: atePort1.IPv6, peerGroup: ibgpPG}
	ebgpV4 = bgpNeighbor{address: atePort2.IPv4, peerGroup: ebgpPG, isV4: true}
	ebgpV6 = bgpNeighbor{address: atePort2.IPv6, peerGroup: ebgpPG}

	allNeighbors = []bgpNeighbor{ibgpV4, ibgpV6, ebgpV4, ebgpV6}
	// The neighbor-level override steps use the IPv4 neighbor of each peer group.
	overrideNeighbors = []bgpNeighbor{ibgpV4, ebgpV4}
	inheritNeighbors  = []bgpNeighbor{ibgpV6, ebgpV6}
	peerGroups        = []string{ibgpPG, ebgpPG}
)

func otgPeerName(a attrs.Attributes, af string) string {
	return a.Name + "." + af + ".bgp"
}

func bgpPath(dut *ondatra.DUTDevice) *netinstbgp.NetworkInstance_Protocol_BgpPath {
	return gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp()
}

// dutPorts returns the DUT ports with their attributes and network instance.
func dutPorts(t *testing.T, dut *ondatra.DUTDevice) []struct {
	port *ondatra.Port
	a    *attrs.Attributes
	ni   string
} {
	t.Helper()
	def := deviations.DefaultNetworkInstance(dut)
	return []struct {
		port *ondatra.Port
		a    *attrs.Attributes
		ni   string
	}{
		{dut.Port(t, "port1"), &dutPort1, transitNI},
		{dut.Port(t, "port2"), &dutPort2, transitNI},
		{dut.Port(t, "port3"), &dutPort3, def},
		{dut.Port(t, "port4"), &dutPort4, def},
	}
}

// configureDUTInterfaces configures the four DUT ports, the TRANSIT network
// instance on port-1 and port-2, and the DEFAULT network instance on port-3
// and port-4, in one gNMI Set.
func configureDUTInterfaces(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	b := &gnmi.SetBatch{}
	transit := &oc.NetworkInstance{
		Name: ygot.String(transitNI),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
	}
	for _, p := range dutPorts(t, dut) {
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, p.port)
		}
		intf := p.a.NewOCInterface(p.port.Name(), dut)
		if deviations.FrBreakoutFix(dut) && p.port.PMD() == ondatra.PMD100GBASEFR {
			e := intf.GetOrCreateEthernet()
			e.SetAutoNegotiate(false)
			e.SetDuplexMode(oc.Ethernet_DuplexMode_FULL)
			e.SetPortSpeed(oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB)
		}
		gnmi.BatchReplace(b, gnmi.OC().Interface(p.port.Name()).Config(), intf)
		if p.ni == transitNI {
			ni := transit.GetOrCreateInterface(p.port.Name() + ".0")
			ni.Interface = ygot.String(p.port.Name())
			ni.Subinterface = ygot.Uint32(0)
		}
	}
	gnmi.BatchReplace(b, gnmi.OC().NetworkInstance(transitNI).Config(), transit)
	b.Set(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, p := range dutPorts(t, dut) {
			if p.ni != transitNI {
				fptest.AssignToNetworkInstance(t, dut, p.port.Name(), p.ni, 0)
			}
		}
	}
}

// configureStaticRoutes routes the ATE port-1 subnets from DEFAULT to the
// TRANSIT addresses on DUT port-2, so the packets cross the loop link.
func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	static := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		Name:       ygot.String(deviations.StaticProtocolName(dut)),
	}
	for prefix, nh := range map[string]string{ateP1PrefixV4: dutPort2.IPv4, ateP1PrefixV6: dutPort2.IPv6} {
		static.GetOrCreateStatic(prefix).GetOrCreateNextHop("0").NextHop = oc.UnionString(nh)
	}
	gnmi.Update(t, dut, staticPath(dut).Config(), static)
}

func staticPath(dut *ondatra.DUTDevice) *networkinstance.NetworkInstance_ProtocolPath {
	return gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
}

// configureRoutingPolicy configures a policy that accepts all routes.
func configureRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	rp := &oc.RoutingPolicy{}
	stmt, err := rp.GetOrCreatePolicyDefinition(policyName).AppendNewStatement("accept")
	if err != nil {
		t.Fatalf("AppendNewStatement(%q) failed: %v", "accept", err)
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(policyName).Config(), rp.GetPolicyDefinition(policyName))
}

// bgpConfig returns the DUT BGP configuration: two peer groups that use the
// accept-all policy, and the four neighbors.
func bgpConfig(dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	proto := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
	bgp := proto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(dutAS)
	global.RouterId = ygot.String(dutPort3.IPv4)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	for name, peerAS := range map[string]uint32{ebgpPG: ateEBGPAS, ibgpPG: dutAS} {
		pg := bgp.GetOrCreatePeerGroup(name)
		pg.PeerAs = ygot.Uint32(peerAS)
		if deviations.RoutePolicyUnderAFIUnsupported(dut) {
			ap := pg.GetOrCreateApplyPolicy()
			ap.ImportPolicy = []string{policyName}
			ap.ExportPolicy = []string{policyName}
			continue
		}
		for _, afi := range []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST} {
			af := pg.GetOrCreateAfiSafi(afi)
			af.Enabled = ygot.Bool(true)
			ap := af.GetOrCreateApplyPolicy()
			ap.ImportPolicy = []string{policyName}
			ap.ExportPolicy = []string{policyName}
		}
	}

	for _, n := range allNeighbors {
		nbr := bgp.GetOrCreateNeighbor(n.address)
		nbr.PeerGroup = ygot.String(n.peerGroup)
		nbr.Enabled = ygot.Bool(true)
		afi := oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
		if n.isV4 {
			afi = oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
		}
		nbr.GetOrCreateAfiSafi(afi).Enabled = ygot.Bool(true)
		if n.peerGroup == ibgpPG {
			// The iBGP sessions run from the DUT port-3 addresses through TRANSIT.
			local := dutPort3.IPv6
			if n.isV4 {
				local = dutPort3.IPv4
			}
			nbr.GetOrCreateTransport().LocalAddress = ygot.String(local)
		}
	}
	return proto
}

// otgOptions selects the optional parts of the OTG configuration.
type otgOptions struct {
	mtu          uint32
	routes       bool
	capturePort1 bool
}

// otgConfig returns the ATE configuration: one device per port with IPv4 and
// IPv6 BGP peers. The ATE port-1 peers are iBGP peers of the DUT port-3
// addresses, reached through DUT port-1.
func otgConfig(t *testing.T, ate *ondatra.ATEDevice, o otgOptions) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	for _, p := range []struct {
		ate, gw, peer *attrs.Attributes
		port          string
		asn           uint32
		asType4       gosnappi.BgpV4PeerAsTypeEnum
		asType6       gosnappi.BgpV6PeerAsTypeEnum
	}{
		{&atePort1, &dutPort1, &dutPort3, "port1", dutAS, gosnappi.BgpV4PeerAsType.IBGP, gosnappi.BgpV6PeerAsType.IBGP},
		{&atePort2, &dutPort4, &dutPort4, "port2", ateEBGPAS, gosnappi.BgpV4PeerAsType.EBGP, gosnappi.BgpV6PeerAsType.EBGP},
	} {
		port := config.Ports().Add().SetName(ate.Port(t, p.port).ID())
		dev := config.Devices().Add().SetName(p.ate.Name)
		eth := dev.Ethernets().Add().SetName(p.ate.Name + ".Eth").SetMac(p.ate.MAC).SetMtu(o.mtu)
		eth.Connection().SetPortName(port.Name())
		ip4 := eth.Ipv4Addresses().Add().SetName(p.ate.Name + ".IPv4").SetAddress(p.ate.IPv4).SetGateway(p.gw.IPv4).SetPrefix(uint32(p.ate.IPv4Len))
		ip6 := eth.Ipv6Addresses().Add().SetName(p.ate.Name + ".IPv6").SetAddress(p.ate.IPv6).SetGateway(p.gw.IPv6).SetPrefix(uint32(p.ate.IPv6Len))

		bgp := dev.Bgp().SetRouterId(p.ate.IPv4)
		peer4 := bgp.Ipv4Interfaces().Add().SetIpv4Name(ip4.Name()).Peers().Add().SetName(otgPeerName(*p.ate, "v4"))
		peer4.SetPeerAddress(p.peer.IPv4).SetAsNumber(p.asn).SetAsType(p.asType4)
		peer6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ip6.Name()).Peers().Add().SetName(otgPeerName(*p.ate, "v6"))
		peer6.SetPeerAddress(p.peer.IPv6).SetAsNumber(p.asn).SetAsType(p.asType6)

		if o.routes && p.ate == &atePort2 {
			r4 := peer4.V4Routes().Add().SetName(v4RouteName).SetNextHopIpv4Address(p.ate.IPv4).
				SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
				SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
			r4.Addresses().Add().SetAddress(v4RouteStart).SetPrefix(v4RouteLen).SetCount(routeCount)
			r6 := peer6.V6Routes().Add().SetName(v6RouteName).SetNextHopIpv6Address(p.ate.IPv6).
				SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
				SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
			r6.Addresses().Add().SetAddress(v6RouteStart).SetPrefix(v6RouteLen).SetCount(routeCount)
		}
	}
	if o.capturePort1 {
		config.Captures().Add().SetName("port1").SetPortNames([]string{ate.Port(t, "port1").ID()}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}
	return config
}

// startOTG pushes the configuration, optionally starts the capture, and starts
// the protocols.
func startOTG(t *testing.T, ate *ondatra.ATEDevice, o otgOptions) gosnappi.Config {
	t.Helper()
	config := otgConfig(t, ate, o)
	ate.OTG().StopProtocols(t)
	ate.OTG().PushConfig(t, config)
	if o.capturePort1 {
		cs := gosnappi.NewControlState()
		cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
		ate.OTG().SetControlState(t, cs)
	}
	ate.OTG().StartProtocols(t)
	return config
}

// restartOTGBGPPeers restarts the four ATE BGP peers to re-establish the
// sessions.
func restartOTGBGPPeers(t *testing.T, o *otg.OTG) {
	t.Helper()
	for _, state := range []gosnappi.StateProtocolBgpPeersStateEnum{gosnappi.StateProtocolBgpPeersState.DOWN, gosnappi.StateProtocolBgpPeersState.UP} {
		cs := gosnappi.NewControlState()
		cs.Protocol().Bgp().Peers().SetPeerNames(otgBGPPeers).SetState(state)
		o.SetControlState(t, cs)
		if state == gosnappi.StateProtocolBgpPeersState.DOWN {
			for _, p := range otgBGPPeers {
				gnmi.Watch(t, o, gnmi.OTG().BgpPeer(p).SessionState().State(), protocolTimeout, func(v *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
					s, ok := v.Val()
					return !ok || s != otgtelemetry.BgpPeer_SessionState_ESTABLISHED
				}).Await(t)
			}
		}
	}
}

// verifyEstablished verifies that the four sessions are ESTABLISHED on the
// ATE and on the DUT.
func verifyEstablished(t *testing.T, dut *ondatra.DUTDevice, o *otg.OTG) {
	t.Helper()
	for _, p := range otgBGPPeers {
		_, ok := gnmi.Watch(t, o, gnmi.OTG().BgpPeer(p).SessionState().State(), protocolTimeout, func(v *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
			s, ok := v.Val()
			return ok && s == otgtelemetry.BgpPeer_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			t.Fatalf("OTG BGP peer %s did not reach ESTABLISHED within %v", p, protocolTimeout)
		}
	}
	for _, n := range allNeighbors {
		path := bgpPath(dut).Neighbor(n.address).SessionState().State()
		_, ok := gnmi.Watch(t, dut, path, protocolTimeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			s, ok := v.Val()
			return ok && s == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			t.Fatalf("DUT BGP neighbor %s did not reach ESTABLISHED within %v", n.address, protocolTimeout)
		}
	}
}

// reportSessionReset logs whether a configuration change reset the sessions.
// It is reported only and is not a pass/fail criterion.
func reportSessionReset(t *testing.T, dut *ondatra.DUTDevice, before map[string]uint64) {
	t.Helper()
	deadline := time.Now().Add(resetWindow)
	reset := map[string]bool{}
	for time.Now().Before(deadline) {
		for _, n := range allNeighbors {
			nbr := bgpPath(dut).Neighbor(n.address)
			transitions, _ := gnmi.Lookup(t, dut, nbr.EstablishedTransitions().State()).Val()
			state, ok := gnmi.Lookup(t, dut, nbr.SessionState().State()).Val()
			if transitions > before[n.address] || !ok || state != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
				reset[n.address] = true
			}
		}
		time.Sleep(5 * time.Second)
	}
	for _, n := range allNeighbors {
		t.Logf("Neighbor %s: applying the configuration reset the session: %t", n.address, reset[n.address])
	}
}

func establishedTransitions(t *testing.T, dut *ondatra.DUTDevice) map[string]uint64 {
	t.Helper()
	m := map[string]uint64{}
	for _, n := range allNeighbors {
		m[n.address], _ = gnmi.Lookup(t, dut, bgpPath(dut).Neighbor(n.address).EstablishedTransitions().State()).Val()
	}
	return m
}

// setPortMTU sets the MTU of a DUT port, or deletes it when mtu is 0.
func setPortMTU(t *testing.T, b *gnmi.SetBatch, dut *ondatra.DUTDevice, port *ondatra.Port, mtu uint16) {
	t.Helper()
	intf := gnmi.OC().Interface(port.Name())
	if deviations.OmitL2MTU(dut) {
		setL3MTU(b, port, mtu)
		return
	}
	if mtu == 0 {
		gnmi.BatchDelete(b, intf.Mtu().Config())
		return
	}
	gnmi.BatchReplace(b, intf.Mtu().Config(), mtu)
}

// setL3MTU sets the IPv4 and IPv6 MTU of a DUT port, or deletes them when mtu
// is 0.
func setL3MTU(b *gnmi.SetBatch, port *ondatra.Port, mtu uint16) {
	sub := gnmi.OC().Interface(port.Name()).Subinterface(0)
	if mtu == 0 {
		gnmi.BatchDelete(b, sub.Ipv4().Mtu().Config())
		gnmi.BatchDelete(b, sub.Ipv6().Mtu().Config())
		return
	}
	gnmi.BatchReplace(b, sub.Ipv4().Mtu().Config(), mtu)
	gnmi.BatchReplace(b, sub.Ipv6().Mtu().Config(), uint32(mtu))
}

// portMTU returns the MTU of a DUT port that a TCP segment must fit in.
func portMTU(t *testing.T, dut *ondatra.DUTDevice, port *ondatra.Port) uint16 {
	t.Helper()
	if deviations.OmitL2MTU(dut) {
		if v, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(port.Name()).Subinterface(0).Ipv4().Mtu().State()).Val(); ok {
			return v
		}
		return mtuDefault
	}
	if v, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(port.Name()).Mtu().State()).Val(); ok {
		return v
	}
	return mtuDefault
}

func verifyNeighborMSS(t *testing.T, dut *ondatra.DUTDevice, nbrs []bgpNeighbor, want uint16) {
	t.Helper()
	for _, n := range nbrs {
		if got := gnmi.Get(t, dut, bgpPath(dut).Neighbor(n.address).Transport().TcpMss().State()); got != want {
			t.Errorf("Neighbor %s transport/state/tcp-mss = %d, want %d", n.address, got, want)
		}
	}
}

func verifyPeerGroupMSS(t *testing.T, dut *ondatra.DUTDevice, want uint16) {
	t.Helper()
	for _, pg := range peerGroups {
		if got := gnmi.Get(t, dut, bgpPath(dut).PeerGroup(pg).Transport().TcpMss().State()); got != want {
			t.Errorf("Peer group %s transport/state/tcp-mss = %d, want %d", pg, got, want)
		}
	}
}

func verifyNeighborPMTUD(t *testing.T, dut *ondatra.DUTDevice, nbrs []bgpNeighbor, want bool) {
	t.Helper()
	for _, n := range nbrs {
		if got := gnmi.Get(t, dut, bgpPath(dut).Neighbor(n.address).Transport().MtuDiscovery().State()); got != want {
			t.Errorf("Neighbor %s transport/state/mtu-discovery = %t, want %t", n.address, got, want)
		}
	}
}

func verifyPeerGroupPMTUD(t *testing.T, dut *ondatra.DUTDevice, want bool) {
	t.Helper()
	for _, pg := range peerGroups {
		if got := gnmi.Get(t, dut, bgpPath(dut).PeerGroup(pg).Transport().MtuDiscovery().State()); got != want {
			t.Errorf("Peer group %s transport/state/mtu-discovery = %t, want %t", pg, got, want)
		}
	}
}

// verifyReceivedPrefixes waits until the ATE port-1 iBGP peers have received
// all the prefixes advertised by the ATE port-2 eBGP peers.
func verifyReceivedPrefixes(t *testing.T, o *otg.OTG) {
	t.Helper()
	peer := gnmi.OTG().BgpPeer
	counts := func() (int, int) {
		v4 := gnmi.LookupAll(t, o, peer(otgPeerName(atePort1, "v4")).UnicastIpv4PrefixAny().State())
		v6 := gnmi.LookupAll(t, o, peer(otgPeerName(atePort1, "v6")).UnicastIpv6PrefixAny().State())
		return len(v4), len(v6)
	}
	deadline := time.Now().Add(protocolTimeout)
	for {
		v4, v6 := counts()
		if v4 >= routeCount && v6 >= routeCount {
			t.Logf("ATE port-1 iBGP peers received %d IPv4 and %d IPv6 prefixes", v4, v6)
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("ATE port-1 iBGP peers received %d IPv4 and %d IPv6 prefixes within %v, want %d of each", v4, v6, protocolTimeout, routeCount)
			return
		}
		time.Sleep(10 * time.Second)
	}
}

// captureStats are the DUT packets seen in the ATE port-1 capture.
type captureStats struct {
	maxV4, maxV6 int
	v4Fragments  int
}

// parseCapture reads a PCAP capture and returns the sizes of the IPv4 and IPv6
// packets sent by the DUT port-3 addresses, and the number of IPv4 fragments.
func parseCapture(pcap []byte) (captureStats, error) {
	var s captureStats
	r, err := pcapgo.NewReader(bytes.NewReader(pcap))
	if err != nil {
		return s, err
	}
	for {
		data, _, err := r.ReadPacketData()
		if errors.Is(err, io.EOF) {
			return s, nil
		}
		if err != nil {
			return s, err
		}
		pkt := gopacket.NewPacket(data, r.LinkType(), gopacket.Lazy)
		if l, ok := pkt.Layer(layers.LayerTypeIPv4).(*layers.IPv4); ok && l.SrcIP.String() == dutPort3.IPv4 {
			s.maxV4 = max(s.maxV4, int(l.Length))
			if l.Flags&layers.IPv4MoreFragments != 0 || l.FragOffset != 0 {
				s.v4Fragments++
			}
		}
		if l, ok := pkt.Layer(layers.LayerTypeIPv6).(*layers.IPv6); ok && l.SrcIP.Equal(netIP(dutPort3.IPv6)) {
			s.maxV6 = max(s.maxV6, int(l.Length)+40)
		}
	}
}

func netIP(s string) net.IP {
	return net.ParseIP(s)
}

func bgpProtoPath(dut *ondatra.DUTDevice) *networkinstance.NetworkInstance_ProtocolPath {
	return gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
}

// setNeighborMSS sets transport/config/tcp-mss on the neighbors, or deletes it
// when mss is 0.
func setNeighborMSS(t *testing.T, dut *ondatra.DUTDevice, nbrs []bgpNeighbor, mss uint16) {
	t.Helper()
	b := &gnmi.SetBatch{}
	for _, n := range nbrs {
		p := bgpPath(dut).Neighbor(n.address).Transport().TcpMss().Config()
		if mss == 0 {
			gnmi.BatchDelete(b, p)
		} else {
			gnmi.BatchReplace(b, p, mss)
		}
	}
	b.Set(t, dut)
}

// setPeerGroupMSS sets transport/config/tcp-mss on both peer groups, or
// deletes it when mss is 0.
func setPeerGroupMSS(t *testing.T, dut *ondatra.DUTDevice, mss uint16) {
	t.Helper()
	b := &gnmi.SetBatch{}
	for _, pg := range peerGroups {
		p := bgpPath(dut).PeerGroup(pg).Transport().TcpMss().Config()
		if mss == 0 {
			gnmi.BatchDelete(b, p)
		} else {
			gnmi.BatchReplace(b, p, mss)
		}
	}
	b.Set(t, dut)
}

// setNeighborPMTUD sets transport/config/mtu-discovery on the neighbors, or
// deletes it when v is nil.
func setNeighborPMTUD(t *testing.T, dut *ondatra.DUTDevice, nbrs []bgpNeighbor, v *bool) {
	t.Helper()
	b := &gnmi.SetBatch{}
	for _, n := range nbrs {
		p := bgpPath(dut).Neighbor(n.address).Transport().MtuDiscovery().Config()
		if v == nil {
			gnmi.BatchDelete(b, p)
		} else {
			gnmi.BatchReplace(b, p, *v)
		}
	}
	b.Set(t, dut)
}

// setPeerGroupPMTUD sets transport/config/mtu-discovery on both peer groups,
// or deletes it when v is nil.
func setPeerGroupPMTUD(t *testing.T, dut *ondatra.DUTDevice, v *bool) {
	t.Helper()
	b := &gnmi.SetBatch{}
	for _, pg := range peerGroups {
		p := bgpPath(dut).PeerGroup(pg).Transport().MtuDiscovery().Config()
		if v == nil {
			gnmi.BatchDelete(b, p)
		} else {
			gnmi.BatchReplace(b, p, *v)
		}
	}
	b.Set(t, dut)
}

// setAllPortsMTU sets the MTU of the four DUT ports, or deletes it when mtu is
// 0.
func setAllPortsMTU(t *testing.T, dut *ondatra.DUTDevice, mtu uint16) {
	t.Helper()
	b := &gnmi.SetBatch{}
	for _, p := range dutPorts(t, dut) {
		setPortMTU(t, b, dut, p.port, mtu)
	}
	b.Set(t, dut)
}

// verifyDefaultMSS checks an MSS that the DUT reports before one is
// configured: at least the TCP default MSS, and at most what fits in the MTU
// of the DUT port towards the neighbor.
func verifyDefaultMSS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	port3, port4 := dut.Port(t, "port3"), dut.Port(t, "port4")
	for _, n := range allNeighbors {
		got, ok := gnmi.Lookup(t, dut, bgpPath(dut).Neighbor(n.address).Transport().TcpMss().State()).Val()
		if !ok || got == 0 {
			t.Logf("Neighbor %s does not report transport/state/tcp-mss before an MSS is configured", n.address)
			continue
		}
		port := port4
		if n.peerGroup == ibgpPG {
			port = port3
		}
		minMSS, overhead := mssMinV6, uint16(tcpIPv6Overhead)
		if n.isV4 {
			minMSS, overhead = mssMinV4, uint16(tcpIPv4Overhead)
		}
		maxMSS := portMTU(t, dut, port) - overhead
		if got < minMSS || got > maxMSS {
			t.Errorf("Neighbor %s transport/state/tcp-mss before configuration = %d, want between %d and %d", n.address, got, minMSS, maxMSS)
		}
	}
}

// TestTCPMSSPathMTU validates the BGP TCP MSS and PMTUD configuration and
// state, and that PMTUD lowers the segment size through the TRANSIT router.
func TestTCPMSSPathMTU(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	o := ate.OTG()

	t.Cleanup(func() {
		o.StopProtocols(t)
		gnmi.Delete(t, dut, bgpProtoPath(dut).Config())
		gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(policyName).Config())
		b := &gnmi.SetBatch{}
		gnmi.BatchDelete(b, staticPath(dut).Static(ateP1PrefixV4).Config())
		gnmi.BatchDelete(b, staticPath(dut).Static(ateP1PrefixV6).Config())
		gnmi.BatchDelete(b, gnmi.OC().NetworkInstance(transitNI).Config())
		b.Set(t, dut)
	})

	if !t.Run("Initial setup", func(t *testing.T) {
		configureDUTInterfaces(t, dut)
		configureStaticRoutes(t, dut)
		configureRoutingPolicy(t, dut)
		gnmi.Replace(t, dut, bgpProtoPath(dut).Config(), bgpConfig(dut))
		startOTG(t, ate, otgOptions{mtu: mtuDefault})
		verifyEstablished(t, dut, o)
	}) {
		return
	}

	t.Run("RT-1.21.1: TCP MSS at the neighbor and peer-group levels", func(t *testing.T) {
		if !deviations.SkipTCPNegotiatedMSSCheck(dut) {
			t.Run("MSS reported before configuration", func(t *testing.T) {
				verifyDefaultMSS(t, dut)
			})
		}

		t.Run("Set the MTU to 5040 bytes", func(t *testing.T) {
			setAllPortsMTU(t, dut, mtuJumbo)
			startOTG(t, ate, otgOptions{mtu: mtuJumbo})
			verifyEstablished(t, dut, o)
		})
		t.Cleanup(func() {
			setAllPortsMTU(t, dut, 0)
			startOTG(t, ate, otgOptions{mtu: mtuDefault})
		})

		t.Run("Neighbor level", func(t *testing.T) {
			before := establishedTransitions(t, dut)
			setNeighborMSS(t, dut, allNeighbors, mssNeighbor)
			reportSessionReset(t, dut, before)
			restartOTGBGPPeers(t, o)
			verifyEstablished(t, dut, o)
			verifyNeighborMSS(t, dut, allNeighbors, mssNeighbor)
			setNeighborMSS(t, dut, allNeighbors, 0)
		})

		t.Run("Peer-group level", func(t *testing.T) {
			setPeerGroupMSS(t, dut, mssGroup)
			restartOTGBGPPeers(t, o)
			verifyEstablished(t, dut, o)
			verifyPeerGroupMSS(t, dut, mssGroup)
			verifyNeighborMSS(t, dut, allNeighbors, mssGroup)

			t.Run("Neighbor-level override", func(t *testing.T) {
				setNeighborMSS(t, dut, overrideNeighbors, mssOverride)
				restartOTGBGPPeers(t, o)
				verifyEstablished(t, dut, o)
				verifyNeighborMSS(t, dut, overrideNeighbors, mssOverride)
				verifyNeighborMSS(t, dut, inheritNeighbors, mssGroup)
				verifyPeerGroupMSS(t, dut, mssGroup)
			})

			setNeighborMSS(t, dut, overrideNeighbors, 0)
			setPeerGroupMSS(t, dut, 0)
		})
	})

	t.Run("RT-1.21.2: PMTUD at the neighbor and peer-group levels", func(t *testing.T) {
		enabled, disabled := ygot.Bool(true), ygot.Bool(false)
		port1 := dut.Port(t, "port1")

		t.Run("Neighbor level", func(t *testing.T) {
			b := &gnmi.SetBatch{}
			setL3MTU(b, port1, mtuPMTUD)
			b.Set(t, dut)
			defer func() {
				b := &gnmi.SetBatch{}
				setL3MTU(b, port1, 0)
				b.Set(t, dut)
			}()

			setNeighborPMTUD(t, dut, allNeighbors, enabled)
			defer setNeighborPMTUD(t, dut, allNeighbors, nil)
			restartOTGBGPPeers(t, o)
			verifyEstablished(t, dut, o)
			verifyNeighborPMTUD(t, dut, allNeighbors, true)

			// Restart the ATE with the capture on port-1 and the prefixes on
			// port-2. The DUT then sends large UPDATE messages to the iBGP
			// peers through TRANSIT, whose link to ATE port-1 has a 1400-byte
			// MTU.
			startOTG(t, ate, otgOptions{mtu: mtuDefault, routes: true, capturePort1: true})
			verifyEstablished(t, dut, o)
			verifyReceivedPrefixes(t, o)
			verifyEstablished(t, dut, o)

			cs := gosnappi.NewControlState()
			cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
			o.SetControlState(t, cs)
			pcap := o.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(ate.Port(t, "port1").ID()))
			s, err := parseCapture(pcap)
			if err != nil {
				t.Fatalf("parseCapture() failed: %v", err)
			}
			t.Logf("Largest DUT packet in the capture: IPv4 %d bytes, IPv6 %d bytes; IPv4 fragments: %d", s.maxV4, s.maxV6, s.v4Fragments)
			if s.maxV4 <= minPMTUDPacket {
				t.Errorf("Largest IPv4 packet from %s = %d bytes, want more than %d", dutPort3.IPv4, s.maxV4, minPMTUDPacket)
			}
			if s.maxV6 <= minPMTUDPacket {
				t.Errorf("Largest IPv6 packet from %s = %d bytes, want more than %d", dutPort3.IPv6, s.maxV6, minPMTUDPacket)
			}
			if s.v4Fragments != 0 {
				t.Errorf("IPv4 fragments from %s = %d, want 0", dutPort3.IPv4, s.v4Fragments)
			}

			// Withdraw the prefixes and remove the capture.
			startOTG(t, ate, otgOptions{mtu: mtuDefault})
			verifyEstablished(t, dut, o)
		})

		t.Run("Peer-group level", func(t *testing.T) {
			setPeerGroupPMTUD(t, dut, enabled)
			defer setPeerGroupPMTUD(t, dut, nil)
			restartOTGBGPPeers(t, o)
			verifyEstablished(t, dut, o)
			verifyPeerGroupPMTUD(t, dut, true)
			verifyNeighborPMTUD(t, dut, allNeighbors, true)

			t.Run("Neighbor-level override", func(t *testing.T) {
				setNeighborPMTUD(t, dut, overrideNeighbors, disabled)
				defer setNeighborPMTUD(t, dut, overrideNeighbors, nil)
				restartOTGBGPPeers(t, o)
				verifyEstablished(t, dut, o)
				verifyNeighborPMTUD(t, dut, overrideNeighbors, false)
				verifyNeighborPMTUD(t, dut, inheritNeighbors, true)
				verifyPeerGroupPMTUD(t, dut, true)
			})
		})
	})
}
