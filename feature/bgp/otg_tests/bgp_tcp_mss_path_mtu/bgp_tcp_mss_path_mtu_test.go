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

// Package bgp_tcp_mss_path_mtu_test implements RT-1.21: BGP TCP MSS and PMTUD.
// The procedure is described in README.md in this directory.
package bgp_tcp_mss_path_mtu_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstbgp"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// AS numbers and names from the README "DUT and OTG properties" table and
// Initial Setup.
const (
	dutAS  = 64500 // DUT AS, which is also the AS of the iBGP peers.
	ebgpAS = 64501 // AS of the eBGP peers on ATE port-1.

	ebgpPeerGroup = "EBGP-PEERS"
	ibgpPeerGroup = "IBGP-PEERS"

	dutRouterID = "192.0.2.1"
	bgpInstance = "BGP" // BGP protocol name from the README Canonical OC.

	plenIPv4 = 30
	plenIPv6 = 126
)

// MTU and MSS values of RT-1.21.1.
const (
	largeMTU = 5040 // MTU of the four ports while the MSS is tested.

	neighborMSS = 4096 // MSS set on the four neighbors.
	// peerGroupMSS differs from neighborMSS, so a value left over from the
	// neighbor-level step fails the peer-group-level check.
	peerGroupMSS = 4000
	overrideMSS  = 1400 // MSS that overrides the peer-group MSS on one neighbor.

	// defaultIPMTU is the Ethernet IP MTU. The default MSS check uses it when
	// the DUT does not report the IP MTU of a port.
	defaultIPMTU = 1500
)

// Upper bounds for the gNMI watches, and the window in which the test records
// whether applying the MSS resets the sessions.
const (
	stateTimeout   = time.Minute
	sessionTimeout = 3 * time.Minute
	resetWindow    = 30 * time.Second
)

// family is the address family of a BGP session.
type family string

const (
	ipv4 family = "IPv4"
	ipv6 family = "IPv6"
)

// families are the address families of the BGP sessions on each ATE port.
var families = []family{ipv4, ipv6}

// afiSafi returns the BGP unicast AFI-SAFI of the address family.
func (f family) afiSafi() oc.E_BgpTypes_AFI_SAFI_TYPE {
	if f == ipv4 {
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
	}
	return oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
}

// minMSS returns the TCP default MSS of the address family: 536 bytes for IPv4
// (RFC 9293) and 1220 bytes for IPv6 (RFC 8200).
func (f family) minMSS() int {
	if f == ipv4 {
		return 536
	}
	return 1220
}

// headerLen returns the length of the IP and TCP headers, which the MSS
// excludes from the IP MTU: 40 bytes for IPv4 and 60 bytes for IPv6.
func (f family) headerLen() int {
	if f == ipv4 {
		return 40
	}
	return 60
}

// ateRouter is an ATE port with its IPv4 and IPv6 BGP peers, together with the
// DUT port and the DUT peer group of those peers.
type ateRouter struct {
	name      string // eBGP or iBGP.
	port      string // Testbed port ID, the same on the DUT and the ATE.
	asn       uint32
	ibgp      bool
	peerGroup string
	dutAttr   attrs.Attributes
	ateAttr   attrs.Attributes
}

var (
	ebgpRouter = &ateRouter{
		name:      "eBGP",
		port:      "port1",
		asn:       ebgpAS,
		peerGroup: ebgpPeerGroup,
		dutAttr: attrs.Attributes{
			Desc:    "DUT to ATE port-1 (eBGP)",
			IPv4:    "192.0.2.1",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::192:0:2:1",
			IPv6Len: plenIPv6,
		},
		ateAttr: attrs.Attributes{
			Name:    "atePort1",
			MAC:     "02:00:01:01:01:01",
			IPv4:    "192.0.2.2",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::192:0:2:2",
			IPv6Len: plenIPv6,
		},
	}
	ibgpRouter = &ateRouter{
		name:      "iBGP",
		port:      "port2",
		asn:       dutAS,
		ibgp:      true,
		peerGroup: ibgpPeerGroup,
		dutAttr: attrs.Attributes{
			Desc:    "DUT to ATE port-2 (iBGP)",
			IPv4:    "192.0.2.5",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::192:0:2:5",
			IPv6Len: plenIPv6,
		},
		ateAttr: attrs.Attributes{
			Name:    "atePort2",
			MAC:     "02:00:02:01:01:01",
			IPv4:    "192.0.2.6",
			IPv4Len: plenIPv4,
			IPv6:    "2001:db8::192:0:2:6",
			IPv6Len: plenIPv6,
		},
	}
	routers = []*ateRouter{ebgpRouter, ibgpRouter}
)

// neighbor is a DUT BGP neighbor: the address of an ATE router in one family.
type neighbor struct {
	router *ateRouter
	family family
}

// neighbors returns the DUT neighbors of the given families on both ATE ports.
func neighbors(fs ...family) []neighbor {
	var nbrs []neighbor
	for _, r := range routers {
		for _, f := range fs {
			nbrs = append(nbrs, neighbor{router: r, family: f})
		}
	}
	return nbrs
}

// addr returns the neighbor address.
func (n neighbor) addr() string {
	if n.family == ipv4 {
		return n.router.ateAttr.IPv4
	}
	return n.router.ateAttr.IPv6
}

// otgPeer returns the name of the OTG BGP peer of the neighbor.
func (n neighbor) otgPeer() string {
	return n.router.ateAttr.Name + ".BGP." + string(n.family)
}

func (n neighbor) String() string {
	return fmt.Sprintf("%s %s neighbor %s", n.router.name, n.family, n.addr())
}

// leaf is a leaf path with config and state queries of type T.
type leaf[T any] interface {
	Config() ygnmi.ConfigQuery[T]
	State() ygnmi.SingletonQuery[T]
}

// transportLeaf is a BGP transport leaf that the test sets at the neighbor and
// peer-group levels: tcp-mss in RT-1.21.1 and mtu-discovery in RT-1.21.2.
type transportLeaf[T comparable] struct {
	name      string
	neighbor  func(*netinstbgp.NetworkInstance_Protocol_Bgp_Neighbor_TransportPath) leaf[T]
	peerGroup func(*netinstbgp.NetworkInstance_Protocol_Bgp_PeerGroup_TransportPath) leaf[T]
}

var (
	tcpMSS = transportLeaf[uint16]{
		name: "tcp-mss",
		neighbor: func(p *netinstbgp.NetworkInstance_Protocol_Bgp_Neighbor_TransportPath) leaf[uint16] {
			return p.TcpMss()
		},
		peerGroup: func(p *netinstbgp.NetworkInstance_Protocol_Bgp_PeerGroup_TransportPath) leaf[uint16] {
			return p.TcpMss()
		},
	}
	mtuDiscovery = transportLeaf[bool]{
		name: "mtu-discovery",
		neighbor: func(p *netinstbgp.NetworkInstance_Protocol_Bgp_Neighbor_TransportPath) leaf[bool] {
			return p.MtuDiscovery()
		},
		peerGroup: func(p *netinstbgp.NetworkInstance_Protocol_Bgp_PeerGroup_TransportPath) leaf[bool] {
			return p.MtuDiscovery()
		},
	}
)

// onNeighbors returns the leaf of each of the neighbors.
func (l transportLeaf[T]) onNeighbors(dut *ondatra.DUTDevice, nbrs []neighbor) []leaf[T] {
	var leaves []leaf[T]
	for _, n := range nbrs {
		leaves = append(leaves, l.neighbor(bgpPath(dut).Neighbor(n.addr()).Transport()))
	}
	return leaves
}

// onPeerGroups returns the leaf of each of the two peer groups.
func (l transportLeaf[T]) onPeerGroups(dut *ondatra.DUTDevice) []leaf[T] {
	var leaves []leaf[T]
	for _, r := range routers {
		leaves = append(leaves, l.peerGroup(bgpPath(dut).PeerGroup(r.peerGroup).Transport()))
	}
	return leaves
}

// verifyNeighbors verifies that the leaf state of each of the neighbors is
// want.
func (l transportLeaf[T]) verifyNeighbors(t *testing.T, dut *ondatra.DUTDevice, nbrs []neighbor, want T) {
	t.Helper()
	for i, lf := range l.onNeighbors(dut, nbrs) {
		awaitState(t, dut, fmt.Sprintf("%v transport/state/%s", nbrs[i], l.name), lf.State(), want)
	}
}

// verifyPeerGroups verifies that the leaf state of both peer groups is want.
func (l transportLeaf[T]) verifyPeerGroups(t *testing.T, dut *ondatra.DUTDevice, want T) {
	t.Helper()
	for i, lf := range l.onPeerGroups(dut) {
		awaitState(t, dut, fmt.Sprintf("Peer group %s transport/state/%s", routers[i].peerGroup, l.name), lf.State(), want)
	}
}

// TestTcpMssPathMtu verifies the TCP MSS and PMTUD configuration and state of
// eBGP and iBGP sessions at the neighbor and peer-group levels.
func TestTcpMssPathMtu(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	setupEnvironment(t, dut, ate)
	verifySessions(t, dut, ate.OTG())

	// The subtests run in README order and share one environment.
	t.Run("RT-1.21.1: TCP MSS at the neighbor and peer-group levels", func(t *testing.T) {
		testTCPMSS(t, dut, ate)
	})
	t.Run("RT-1.21.2: PMTUD at the neighbor and peer-group levels", func(t *testing.T) {
		testPMTUD(t, dut, ate.OTG())
	})
}

// testTCPMSS runs RT-1.21.1.
func testTCPMSS(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	t.Run("Default MSS", func(t *testing.T) {
		if deviations.SkipTCPNegotiatedMSSCheck(dut) {
			t.Skip("The DUT does not report the MSS in use when no MSS is configured")
		}
		verifyDefaultMSS(t, dut)
	})

	t.Logf("Set the MTU of DUT port-1, DUT port-2, ATE port-1 and ATE port-2 to %d bytes", largeMTU)
	t.Cleanup(func() {
		t.Log("Remove the MTU configuration from the four ports so that they return to the default MTU")
		setPortMTU(t, dut, ate, 0)
	})
	setPortMTU(t, dut, ate, largeMTU)
	verifySessions(t, dut, otg)

	t.Run("Neighbor level", func(t *testing.T) {
		nbrs := neighbors(families...)
		before := establishedTransitions(t, dut)
		t.Logf("Recorded state/established-transitions of the four neighbors: %v", before)
		t.Cleanup(func() {
			t.Log("Remove transport/config/tcp-mss from the four neighbors")
			deleteAll(t, dut, tcpMSS.onNeighbors(dut, nbrs))
		})
		t.Logf("Configure transport/config/tcp-mss to %d on all four neighbors", neighborMSS)
		replaceAll(t, dut, tcpMSS.onNeighbors(dut, nbrs), neighborMSS)
		// The README reports whether the change is disruptive, but does not
		// make it a pass/fail criterion.
		t.Logf("Applying the MSS reset the sessions within %v: %t", resetWindow, sessionsReset(t, dut, before))
		reestablishSessions(t, dut, otg)
		tcpMSS.verifyNeighbors(t, dut, nbrs, neighborMSS)
	})

	t.Run("Peer-group level", func(t *testing.T) {
		testPeerGroupLevel(t, dut, otg, tcpMSS, peerGroupMSS, overrideMSS)
	})
}

// testPMTUD runs RT-1.21.2.
func testPMTUD(t *testing.T, dut *ondatra.DUTDevice, otg *otg.OTG) {
	t.Log("No MSS is configured in this subtest, and all four ports use the default MTU")
	t.Run("Neighbor level", func(t *testing.T) {
		nbrs := neighbors(families...)
		t.Cleanup(func() {
			t.Log("Remove transport/config/mtu-discovery from the four neighbors")
			deleteAll(t, dut, mtuDiscovery.onNeighbors(dut, nbrs))
		})
		t.Log("Set transport/config/mtu-discovery to true on all four neighbors")
		replaceAll(t, dut, mtuDiscovery.onNeighbors(dut, nbrs), true)
		reestablishSessions(t, dut, otg)
		mtuDiscovery.verifyNeighbors(t, dut, nbrs, true)
	})

	t.Run("Peer-group level", func(t *testing.T) {
		testPeerGroupLevel(t, dut, otg, mtuDiscovery, true, false)
	})
}

// testPeerGroupLevel runs the peer-group level of a subtest. It sets the leaf
// to pgValue on both peer groups and verifies that the four neighbors inherit
// it. Then it overrides the leaf with overrideValue on the IPv4 neighbor of
// each peer group, and verifies that only those two neighbors change.
func testPeerGroupLevel[T comparable](t *testing.T, dut *ondatra.DUTDevice, otg *otg.OTG, l transportLeaf[T], pgValue, overrideValue T) {
	t.Cleanup(func() {
		t.Logf("Remove transport/config/%s from both peer groups", l.name)
		deleteAll(t, dut, l.onPeerGroups(dut))
	})
	t.Logf("Set transport/config/%s to %v on the peer groups %s and %s, with no value configured on the neighbors", l.name, pgValue, ebgpPeerGroup, ibgpPeerGroup)
	replaceAll(t, dut, l.onPeerGroups(dut), pgValue)
	reestablishSessions(t, dut, otg)
	l.verifyPeerGroups(t, dut, pgValue)
	t.Log("Verify that all four neighbors inherit the peer-group value")
	l.verifyNeighbors(t, dut, neighbors(families...), pgValue)

	t.Run("Neighbor-level override", func(t *testing.T) {
		overridden := neighbors(ipv4)
		t.Cleanup(func() {
			t.Logf("Remove transport/config/%s from the two IPv4 neighbors", l.name)
			deleteAll(t, dut, l.onNeighbors(dut, overridden))
		})
		t.Logf("Keep the peer-group value, and set transport/config/%s to %v on the IPv4 neighbor of each peer group", l.name, overrideValue)
		replaceAll(t, dut, l.onNeighbors(dut, overridden), overrideValue)
		reestablishSessions(t, dut, otg)
		l.verifyNeighbors(t, dut, overridden, overrideValue)
		t.Log("Verify that the IPv6 neighbors and both peer groups keep the peer-group value")
		l.verifyNeighbors(t, dut, neighbors(ipv6), pgValue)
		l.verifyPeerGroups(t, dut, pgValue)
	})
}

// setupEnvironment applies the README Initial Setup. Each README Cleanup step
// is registered with t.Cleanup before the configuration that it reverts is
// pushed, so that it also runs when a push fails part-way.
func setupEnvironment(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	t.Helper()
	t.Log("Initial Setup: connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2")
	for _, r := range routers {
		t.Logf("%s peers: DUT port %s, ATE port %s", r.name, dut.Port(t, r.port).Name(), ate.Port(t, r.port).ID())
	}

	t.Log("Initial Setup: configure the DUT interfaces, and BGP with the peer groups EBGP-PEERS and IBGP-PEERS")
	configureDUTInterfaces(t, dut)
	configureDUTBGP(t, dut)

	t.Log("Initial Setup: configure the ATE interfaces and BGP peers, and start the OTG protocols")
	otg := ate.OTG()
	t.Cleanup(func() {
		t.Log("Cleanup: stop the OTG protocols")
		otg.StopProtocols(t)
	})
	startOTG(t, ate, 0)
}

// configureDUTInterfaces configures the DUT ports from the README table and
// waits until their state matches the configuration. The README Cleanup
// restores the baseline state, so the pre-test interface configuration is
// captured first and restored in Cleanup.
func configureDUTInterfaces(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	restore := &gnmi.SetBatch{}
	batch := &gnmi.SetBatch{}
	for _, r := range routers {
		p := dut.Port(t, r.port)
		intf := gnmi.OC().Interface(p.Name())
		if orig, ok := gnmi.Lookup(t, dut, intf.Config()).Val(); ok {
			gnmi.BatchReplace(restore, intf.Config(), orig)
		} else {
			gnmi.BatchDelete(restore, intf.Config())
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			// fptest.AssignToNetworkInstance uses the ID "<interface>.<subinterface>".
			niIntf := gnmi.OC().NetworkInstance(ni).Interface(p.Name() + ".0")
			if _, ok := gnmi.Lookup(t, dut, niIntf.Config()).Val(); !ok {
				gnmi.BatchDelete(restore, niIntf.Config())
			}
		}
		ocIntf := r.dutAttr.NewOCInterface(p.Name(), dut)
		if deviations.FrBreakoutFix(dut) && p.PMD() == ondatra.PMD100GBASEFR {
			e := ocIntf.GetOrCreateEthernet()
			e.AutoNegotiate = ygot.Bool(false)
			e.DuplexMode = oc.Ethernet_DuplexMode_FULL
			e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
		}
		gnmi.BatchReplace(batch, intf.Config(), ocIntf)
	}
	t.Cleanup(func() {
		t.Log("Cleanup: restore the pre-test DUT interface configuration")
		restore.Set(t, dut)
	})
	batch.Set(t, dut)

	// The DEFAULT network instance always exists; this idempotent update only
	// sets its type, so it is not reverted.
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	for _, r := range routers {
		p := dut.Port(t, r.port)
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, p)
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, p.Name(), ni, 0)
		}
	}
	for _, r := range routers {
		awaitInterfaceState(t, dut, dut.Port(t, r.port).Name(), &r.dutAttr)
	}
}

// awaitInterfaceState waits until the DUT port is up and reports the configured
// IPv4 and IPv6 addresses.
func awaitInterfaceState(t *testing.T, dut *ondatra.DUTDevice, name string, a *attrs.Attributes) {
	t.Helper()
	intf := gnmi.OC().Interface(name)
	gnmi.Await(t, dut, intf.OperStatus().State(), stateTimeout, oc.Interface_OperStatus_UP)
	sub := intf.Subinterface(0)
	gnmi.Await(t, dut, sub.Ipv4().Address(a.IPv4).PrefixLength().State(), stateTimeout, a.IPv4Len)
	gnmi.Await(t, dut, sub.Ipv6().Address(a.IPv6).PrefixLength().State(), stateTimeout, a.IPv6Len)
}

// configureDUTBGP configures BGP with AS 64500 and the peer groups EBGP-PEERS
// and IBGP-PEERS, each with the IPv4 and IPv6 neighbors of one ATE port. The
// neighbors carry no MSS or PMTUD configuration, which the subtests add.
func configureDUTBGP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Start from a DUT without BGP configuration")
	cfgplugins.BGPClearConfig(t, dut)

	batch := &gnmi.SetBatch{}
	proto := cfgplugins.ConfigureDUTBGP(t, dut, batch, cfgplugins.BGPConfig{DutAS: dutAS, RouterID: dutRouterID})
	bgp := proto.GetOrCreateBgp()
	for _, r := range routers {
		cfgplugins.ConfigurePeerGroup(bgp.GetOrCreatePeerGroup(r.peerGroup), dut, cfgplugins.WithPeerAS(r.asn))
		for _, f := range families {
			nbr := bgp.GetOrCreateNeighbor(neighbor{router: r, family: f}.addr())
			// WithPeerGroup sets the peer group and the peer AS, and enables
			// the neighbor. Each neighbor carries the unicast AFI-SAFI of its
			// own address family.
			cfgplugins.ConfigurePeer(nbr, dut, cfgplugins.WithPeerGroup(r.peerGroup, r.asn, "", "", false))
			nbr.GetOrCreateAfiSafi(f.afiSafi()).Enabled = ygot.Bool(true)
		}
	}
	t.Cleanup(func() {
		t.Log("Cleanup: delete the DUT BGP configuration")
		cfgplugins.BGPClearConfig(t, dut)
	})
	batch.Set(t, dut)
}

// startOTG pushes the OTG configuration with the given MTU on both ATE ports,
// or with the OTG default MTU if mtu is 0, and starts the OTG protocols.
func startOTG(t *testing.T, ate *ondatra.ATEDevice, mtu uint16) {
	t.Helper()
	top := gosnappi.NewConfig()
	for _, r := range routers {
		ateAttr := r.ateAttr
		ateAttr.MTU = mtu
		dev := ateAttr.AddToOTG(top, ate.Port(t, r.port), &r.dutAttr)
		sessionType := otgconfighelpers.WithBGPEBGP()
		if r.ibgp {
			sessionType = otgconfighelpers.WithBGPIBGP()
		}
		otgconfighelpers.AddBGPV4Peer(dev, dev.Name()+".IPv4",
			otgconfighelpers.WithBGPName(neighbor{router: r, family: ipv4}.otgPeer()),
			otgconfighelpers.WithBGPPeerAddress(r.dutAttr.IPv4),
			otgconfighelpers.WithBGPASNumber(r.asn),
			otgconfighelpers.WithBGPRouterID(r.ateAttr.IPv4),
			otgconfighelpers.WithoutBGPGR(),
			sessionType,
		)
		otgconfighelpers.AddBGPV6Peer(dev, dev.Name()+".IPv6",
			otgconfighelpers.WithBGPName(neighbor{router: r, family: ipv6}.otgPeer()),
			otgconfighelpers.WithBGPPeerAddress(r.dutAttr.IPv6),
			otgconfighelpers.WithBGPASNumber(r.asn),
			otgconfighelpers.WithBGPRouterID(r.ateAttr.IPv4),
			otgconfighelpers.WithoutBGPGR(),
			sessionType,
		)
	}
	otg := ate.OTG()
	otg.PushConfig(t, top)
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, top, "IPv4")
	otgutils.WaitForARP(t, otg, top, "IPv6")
}

// setPortMTU sets the MTU of the two DUT ports and the two ATE ports to mtu.
// If mtu is 0, it removes the DUT MTU configuration and returns the ATE ports
// to the OTG default MTU. The ATE MTU is part of the OTG configuration, so
// changing it restarts the OTG protocols.
func setPortMTU(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, mtu uint16) {
	t.Helper()
	b := &gnmi.SetBatch{}
	for _, r := range routers {
		cfgplugins.AddInterfaceMTUOps(b, dut, dut.Port(t, r.port).Name(), mtu, mtu == 0)
	}
	b.Set(t, dut)
	startOTG(t, ate, mtu)
}

// verifyDefaultMSS checks transport/state/tcp-mss before an MSS is configured.
// The DUT may not report the leaf then. If it does, the value must be at least
// the TCP default MSS, and at most the IP MTU of the DUT port minus the IP and
// TCP headers.
func verifyDefaultMSS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	for _, n := range neighbors(families...) {
		mss, present := gnmi.Lookup(t, dut, bgpPath(dut).Neighbor(n.addr()).Transport().TcpMss().State()).Val()
		if !present {
			t.Logf("%v does not report transport/state/tcp-mss before an MSS is configured", n)
			continue
		}
		minMSS := n.family.minMSS()
		maxMSS := ipMTU(t, dut, dut.Port(t, n.router.port).Name(), n.family) - n.family.headerLen()
		if got := int(mss); got < minMSS || got > maxMSS {
			t.Errorf("%v transport/state/tcp-mss before an MSS is configured got %d, want %d to %d", n, got, minMSS, maxMSS)
			continue
		}
		t.Logf("%v reports transport/state/tcp-mss %d before an MSS is configured", n, mss)
	}
}

// ipMTU returns the IPv4 or IPv6 MTU of a DUT port, or the Ethernet default of
// 1500 bytes if the DUT does not report it.
func ipMTU(t *testing.T, dut *ondatra.DUTDevice, port string, f family) int {
	t.Helper()
	sub := gnmi.OC().Interface(port).Subinterface(0)
	if f == ipv4 {
		if mtu, ok := gnmi.Lookup(t, dut, sub.Ipv4().Mtu().State()).Val(); ok && mtu > 0 {
			return int(mtu)
		}
		return defaultIPMTU
	}
	if mtu, ok := gnmi.Lookup(t, dut, sub.Ipv6().Mtu().State()).Val(); ok && mtu > 0 {
		return int(mtu)
	}
	return defaultIPMTU
}

// establishedTransitions returns state/established-transitions of each DUT
// neighbor by address, read in one gNMI Get.
func establishedTransitions(t *testing.T, dut *ondatra.DUTDevice) map[string]uint64 {
	t.Helper()
	transitions := make(map[string]uint64)
	for _, nbr := range gnmi.GetAll(t, dut, bgpPath(dut).NeighborAny().State()) {
		transitions[nbr.GetNeighborAddress()] = nbr.GetEstablishedTransitions()
	}
	return transitions
}

// sessionsReset reports whether a BGP session was reset within resetWindow:
// whether a session left ESTABLISHED, or the established-transitions counter
// of a neighbor increased from before.
func sessionsReset(t *testing.T, dut *ondatra.DUTDevice, before map[string]uint64) bool {
	t.Helper()
	_, left := gnmi.WatchAll(t, onChange(dut), bgpPath(dut).NeighborAny().SessionState().State(), resetWindow, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		s, present := v.Val()
		return !present || s != oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if left {
		return true
	}
	for addr, n := range establishedTransitions(t, dut) {
		if n > before[addr] {
			return true
		}
	}
	return false
}

// reestablishSessions re-establishes the four BGP sessions by stopping and
// starting the ATE BGP peers, and verifies that they are ESTABLISHED again.
func reestablishSessions(t *testing.T, dut *ondatra.DUTDevice, otg *otg.OTG) {
	t.Helper()
	t.Log("Re-establish the four sessions by restarting the ATE BGP peers")
	setOTGPeers(t, otg, gosnappi.StateProtocolBgpPeersState.DOWN)
	// Wait until the DUT reports each session down, so that the check below
	// sees the new sessions and not the ones that were stopped.
	for _, n := range neighbors(families...) {
		_, down := gnmi.Watch(t, onChange(dut), bgpPath(dut).Neighbor(n.addr()).SessionState().State(), sessionTimeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			s, present := v.Val()
			return !present || s != oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !down {
			t.Fatalf("%v is still ESTABLISHED after its ATE BGP peer was stopped", n)
		}
	}
	setOTGPeers(t, otg, gosnappi.StateProtocolBgpPeersState.UP)
	verifySessions(t, dut, otg)
}

// setOTGPeers sets the state of the four OTG BGP peers.
func setOTGPeers(t *testing.T, otg *otg.OTG, state gosnappi.StateProtocolBgpPeersStateEnum) {
	t.Helper()
	var names []string
	for _, n := range neighbors(families...) {
		names = append(names, n.otgPeer())
	}
	cs := gosnappi.NewControlState()
	cs.Protocol().Bgp().Peers().SetPeerNames(names).SetState(state)
	otg.SetControlState(t, cs)
}

// verifySessions verifies that the four BGP sessions are ESTABLISHED, and
// stops the test if any is not, because every later step needs all four.
// Each session is watched on its own, so that an error names each session that
// is not ESTABLISHED. The ATE peers are checked as well as the DUT neighbors:
// after the ATE restarts its peers, its state shows only the new sessions,
// while the DUT can still report a session that the restart closed.
func verifySessions(t *testing.T, dut *ondatra.DUTDevice, otg *otg.OTG) {
	t.Helper()
	t.Log("Verify that all four BGP sessions are ESTABLISHED")
	nbrs := neighbors(families...)
	var ateWatches []*gnmi.Watcher[otgtelemetry.E_BgpPeer_SessionState]
	var dutWatches []*gnmi.Watcher[oc.E_Bgp_Neighbor_SessionState]
	for _, n := range nbrs {
		ateWatches = append(ateWatches, gnmi.Watch(t, otg, gnmi.OTG().BgpPeer(n.otgPeer()).SessionState().State(), sessionTimeout, func(v *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
			s, present := v.Val()
			return present && s == otgtelemetry.BgpPeer_SessionState_ESTABLISHED
		}))
		dutWatches = append(dutWatches, gnmi.Watch(t, onChange(dut), bgpPath(dut).Neighbor(n.addr()).SessionState().State(), sessionTimeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			s, present := v.Val()
			return present && s == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}))
	}
	established := true
	for i, n := range nbrs {
		if last, ok := ateWatches[i].Await(t); !ok {
			t.Errorf("ATE BGP peer %s session-state got %s, want ESTABLISHED", n.otgPeer(), lastValue(last))
			established = false
		}
		if last, ok := dutWatches[i].Await(t); !ok {
			t.Errorf("DUT %v session-state got %s, want ESTABLISHED", n, lastValue(last))
			established = false
		}
	}
	if !established {
		t.Fatal("Not all four BGP sessions are ESTABLISHED")
	}
}

// awaitState waits until a DUT state leaf reports want. It reports an error
// with the last received value if the leaf does not report want within
// stateTimeout.
func awaitState[T comparable](t *testing.T, dut *ondatra.DUTDevice, desc string, q ygnmi.SingletonQuery[T], want T) {
	t.Helper()
	last, ok := gnmi.Watch(t, onChange(dut), q, stateTimeout, func(v *ygnmi.Value[T]) bool {
		got, present := v.Val()
		return present && got == want
	}).Await(t)
	if !ok {
		t.Errorf("%s got %s, want %v", desc, lastValue(last), want)
		return
	}
	t.Logf("%s is %v", desc, want)
}

// lastValue returns the last value that a watch received, for error messages.
func lastValue[T any](v *ygnmi.Value[T]) string {
	if v == nil {
		return "no value"
	}
	got, present := v.Val()
	if !present {
		return "no value"
	}
	return fmt.Sprint(got)
}

// replaceAll replaces the configuration of each leaf with v in one gNMI Set.
func replaceAll[T any](t *testing.T, dut *ondatra.DUTDevice, leaves []leaf[T], v T) {
	t.Helper()
	b := &gnmi.SetBatch{}
	for _, l := range leaves {
		gnmi.BatchReplace(b, l.Config(), v)
	}
	b.Set(t, dut)
}

// deleteAll deletes the configuration of each leaf in one gNMI Set.
func deleteAll[T any](t *testing.T, dut *ondatra.DUTDevice, leaves []leaf[T]) {
	t.Helper()
	b := &gnmi.SetBatch{}
	for _, l := range leaves {
		gnmi.BatchDelete(b, l.Config())
	}
	b.Set(t, dut)
}

// onChange returns DUT gNMI options for ON_CHANGE subscriptions, which the
// README lists in its RPC coverage.
func onChange(dut *ondatra.DUTDevice) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE))
}

// bgpPath returns the path of the DUT BGP instance in the DEFAULT network
// instance.
func bgpPath(dut *ondatra.DUTDevice) *netinstbgp.NetworkInstance_Protocol_BgpPath {
	return gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
}
