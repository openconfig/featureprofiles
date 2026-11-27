// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package bgp_extended_nexthop_test implements RT-1.112: BGP Extended Next-Hop
// Encoding (RFC 8950). The procedure is described in README.md in this
// directory.
package bgp_extended_nexthop_test

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstbgp"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

// TestMain initializes the testbed and runs the test suite. Device reservation
// happens inside fptest.RunTests, so TestBGPExtendedNextHopEncoding sets up
// the shared test environment after reservation.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	dutAS       = 64500
	dutRouterID = "198.18.0.1"

	bgpName         = "BGP" // BGP protocol name from the README Canonical OC.
	acceptAllPolicy = "ACCEPT-ALL"
	maxPaths        = 2 // eBGP multipath paths of RT-1.112.5.

	ipv4PrefixLen = 30
	ipv6PrefixLen = 126

	// Link bandwidths of RT-1.112.5 and RT-1.112.6 in bytes per second, the
	// unit of the link bandwidth extended community: 20 Mbps and 10 Mbps.
	bandwidth20M = 2500000
	bandwidth10M = 1250000
)

// Traffic profile from the README: every flow is sent from ATE port-4 for
// trafficSeconds at trafficPPS and is counted per receiving ATE port.
const (
	trafficSeconds = 30
	trafficPPS     = 1000
	trafficPackets = trafficSeconds * trafficPPS
	frameSize      = 512
	maxLossPct     = 2.0  // Every flow must lose less than this.
	minRxPct       = 98.0 // Share of the sent packets that a receiving ATE port must count.
	ecmpTolerance  = 5.0  // Percentage points.

	// The ECMP flow has random UDP source and destination ports in each
	// packet, taken from lists of co-prime lengths, so that the port pairs
	// don't repeat in lockstep.
	udpSrcPorts = 1000
	udpDstPorts = 997
)

// Upper bounds for the gNMI watches.
const (
	stateTimeout   = 2 * time.Minute
	sessionTimeout = 3 * time.Minute
	routeTimeout   = 2 * time.Minute
	flowTimeout    = trafficSeconds*time.Second + time.Minute
	counterTimeout = time.Minute
	// rxSettleTimeout bounds the wait for the received packet count of a flow
	// to reach its sent packet count, which it never does if packets are lost.
	rxSettleTimeout = 15 * time.Second
	// optionalSessionTimeout bounds the wait for the RT-1.112.7 session with
	// extended next-hop encoding disabled, which the README allows to stay
	// down.
	optionalSessionTimeout = time.Minute
	// absenceWindow is how long RT-1.112.7 watches for an installed prefix
	// or an advertised route that must not appear.
	absenceWindow = 30 * time.Second
)

// atePeer is an ATE port of the README table with its BGP peer toward the DUT.
type atePeer struct {
	desc     string // README name, for example "ATE port-1".
	port     string // Testbed port ID, the same on the DUT and the ATE.
	asn      uint32
	routerID string
	ibgp     bool
	// ipv4Session is set for IPv4 BGP sessions (ATE port-4 and RT-1.112.8 on
	// ATE port-3). The other sessions run over IPv6.
	ipv4Session bool
	// enableENH enables extended next-hop encoding for IPV4_UNICAST on the
	// session.
	enableENH bool
	peerName  string // Optional OTG BGP peer name override.
	pg        string // Optional DUT BGP peer-group name override.
	dut       attrs.Attributes
	ate       attrs.Attributes
}

func (p *atePeer) String() string {
	return p.desc
}

// neighbor returns the ATE address that is the DUT BGP neighbor address.
func (p *atePeer) neighbor() string {
	if p.ipv4Session {
		return p.ate.IPv4
	}
	return p.ate.IPv6
}

// bgpPeerName returns the name of the OTG BGP peer.
func (p *atePeer) bgpPeerName() string {
	if p.peerName != "" {
		return p.peerName
	}
	return p.ate.Name + ".BGP"
}

// peerGroup returns the DUT BGP peer-group name of p.
func (p *atePeer) peerGroup() string {
	if p.pg != "" {
		return p.pg
	}
	return "BGP-PEER-GROUP-" + strings.ToUpper(p.port)
}

var (
	atePort1 = &atePeer{
		desc:      "ATE port-1",
		port:      "port1",
		asn:       64501,
		routerID:  "198.18.0.2",
		enableENH: true,
		dut:       attrs.Attributes{Desc: "DUT port-1 to ATE port-1", IPv6: "2001:db8::1", IPv6Len: ipv6PrefixLen},
		ate:       attrs.Attributes{Name: "atePort1", MAC: "02:00:01:01:01:01", IPv6: "2001:db8::2", IPv6Len: ipv6PrefixLen},
	}
	atePort2 = &atePeer{
		desc:      "ATE port-2",
		port:      "port2",
		asn:       64502,
		routerID:  "198.18.0.3",
		enableENH: true,
		dut:       attrs.Attributes{Desc: "DUT port-2 to ATE port-2", IPv6: "2001:db8::5", IPv6Len: ipv6PrefixLen},
		ate:       attrs.Attributes{Name: "atePort2", MAC: "02:00:02:01:01:01", IPv6: "2001:db8::6", IPv6Len: ipv6PrefixLen},
	}
	atePort3 = &atePeer{
		desc:      "ATE port-3",
		port:      "port3",
		asn:       dutAS,
		routerID:  "198.18.0.4",
		ibgp:      true,
		enableENH: true,
		dut: attrs.Attributes{
			Desc:    "DUT port-3 to ATE port-3",
			IPv4:    "192.0.2.9",
			IPv4Len: ipv4PrefixLen,
			IPv6:    "2001:db8::9",
			IPv6Len: ipv6PrefixLen,
		},
		ate: attrs.Attributes{
			Name:    "atePort3",
			MAC:     "02:00:03:01:01:01",
			IPv4:    "192.0.2.10",
			IPv4Len: ipv4PrefixLen,
			IPv6:    "2001:db8::a",
			IPv6Len: ipv6PrefixLen,
		},
	}
	atePort4 = &atePeer{
		desc:        "ATE port-4",
		port:        "port4",
		asn:         64503,
		routerID:    "198.18.0.5",
		ipv4Session: true,
		dut: attrs.Attributes{
			Desc:    "DUT port-4 to ATE port-4",
			IPv4:    "192.0.2.13",
			IPv4Len: ipv4PrefixLen,
			IPv6:    "2001:db8::d",
			IPv6Len: ipv6PrefixLen,
		},
		ate: attrs.Attributes{
			Name:    "atePort4",
			MAC:     "02:00:04:01:01:01",
			IPv4:    "192.0.2.14",
			IPv4Len: ipv4PrefixLen,
			IPv6:    "2001:db8::e",
			IPv6Len: ipv6PrefixLen,
		},
	}
	// atePort3IPv4 is the IPv4 iBGP peer added on ATE port-3 in RT-1.112.8. It
	// is excluded from atePeers so interface setup and cleanup visit each
	// physical port once.
	atePort3IPv4 = &atePeer{
		desc:        "ATE port-3 IPv4",
		port:        "port3",
		asn:         dutAS,
		routerID:    atePort3.routerID,
		ibgp:        true,
		ipv4Session: true,
		enableENH:   true,
		peerName:    "atePort3.BGP4",
		pg:          "BGP-PEER-GROUP-PORT3-V4",
		dut:         atePort3.dut,
		ate:         atePort3.ate,
	}
	atePeers = []*atePeer{atePort1, atePort2, atePort3, atePort4}
)

// advertisedRoute is a route of the README route table, advertised by one ATE
// port with the global IPv6 address of the port as its only next-hop.
type advertisedRoute struct {
	name   string // OTG route range name.
	prefix string
	peer   *atePeer
}

var (
	port1IPv4Route = &advertisedRoute{name: "atePort1.routes.ipv4", prefix: "198.51.100.0/25", peer: atePort1}
	port1IPv6Route = &advertisedRoute{name: "atePort1.routes.ipv6", prefix: "2001:db8:100::/64", peer: atePort1}
	port3IPv4Route = &advertisedRoute{name: "atePort3.routes.ipv4", prefix: "203.0.113.0/24", peer: atePort3}
	port1ECMPRoute = &advertisedRoute{name: "atePort1.routes.ecmp", prefix: "198.51.100.128/25", peer: atePort1}
	port2ECMPRoute = &advertisedRoute{name: "atePort2.routes.ecmp", prefix: "198.51.100.128/25", peer: atePort2}
)

// trafficFlow is a README traffic flow from ATE port-4.
type trafficFlow struct {
	name string
	dst  string
	// routes are the routes to dst. The ATE ports that advertise them receive
	// the flow.
	routes []*advertisedRoute
	// randomUDP sets random UDP source and destination ports in each packet.
	randomUDP bool
}

var (
	port1IPv4Flow = &trafficFlow{name: "RT-1.112.1-IPv4", dst: "198.51.100.1", routes: []*advertisedRoute{port1IPv4Route}}
	port1IPv6Flow = &trafficFlow{name: "RT-1.112.1-IPv6", dst: "2001:db8:100::1", routes: []*advertisedRoute{port1IPv6Route}}
	port3Flow     = &trafficFlow{name: "RT-1.112.3-IPv4", dst: "203.0.113.1", routes: []*advertisedRoute{port3IPv4Route}}
	ecmpFlow      = &trafficFlow{
		name:      "RT-1.112.5-IPv4-ECMP",
		dst:       "198.51.100.129",
		routes:    []*advertisedRoute{port1ECMPRoute, port2ECMPRoute},
		randomUDP: true,
	}
)

// ecmpExpectation is the expected result of the RT-1.112.5 checks.
type ecmpExpectation struct {
	minRatio, maxRatio float64 // AFT weight ratio of the ATE port-1 to the ATE port-2 next-hop.
	port1Pct, port2Pct float64 // Traffic share of ATE port-1 and ATE port-2.
}

var (
	// ecmp2To1 is the expectation of RT-1.112.5, with link bandwidths 20M and 10M.
	ecmp2To1 = ecmpExpectation{minRatio: 1.9, maxRatio: 2.1, port1Pct: 66.7, port2Pct: 33.3}
	// ecmp1To1 is the expectation of RT-1.112.6, with link bandwidths 10M and 10M.
	ecmp1To1 = ecmpExpectation{minRatio: 0.95, maxRatio: 1.05, port1Pct: 50, port2Pct: 50}
)

// testEnv holds the devices and the ATE configuration that the subtests share.
type testEnv struct {
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	dutPort map[*atePeer]string // DUT port name per ATE port.
	atePort map[*atePeer]string // ATE port ID per ATE port.
	otgCfg  gosnappi.Config
	// peers are the active ATE BGP peers whose DUT sessions startATEProtocols
	// waits to be ESTABLISHED.
	peers []*atePeer
	// atePort3BGP is the OTG BGP configuration of ATE port-3, to which
	// RT-1.112.8 adds an IPv4 iBGP peer.
	atePort3BGP gosnappi.DeviceBgpRouter
	// bgpPeers are the IPv6 BGP peers of ATE port-1 to port-3, to which the
	// subtests add their routes.
	bgpPeers map[*atePeer]gosnappi.BgpV6Peer
	// port1LBW is the link bandwidth extended community of the RT-1.112.5
	// route of ATE port-1, which RT-1.112.6 changes.
	port1LBW gosnappi.BgpExtendedCommunityNonTransitive2OctetAsTypeLinkBandwidth
	// multipathCLICleanup holds CLI negation commands for multipath/WECMP CLI
	// pushed in enableMultipath, executed before deleting BGP in cleanup.
	multipathCLICleanup []string
	// juniperLocalIPv4Peers tracks BGP peers where Junos local-ipv4-address is
	// currently configured via CLI.
	juniperLocalIPv4Peers map[*atePeer]bool
	// juniperCLIBGPPeers tracks every BGP peer touched by Junos CLI under
	// protocols bgp so removeDUTConfig deletes its CLI neighbor stanza before
	// deleting the OpenConfig BGP protocol.
	juniperCLIBGPPeers map[*atePeer]bool
	// intfCLICleanup holds CLI negation commands for interface CLI pushed in
	// configureDUTInterfaces, executed before restoring interfaces in cleanup.
	intfCLICleanup []string
}

// TestBGPExtendedNextHopEncoding verifies RFC 8950 BGP extended next-hop
// encoding across eBGP, iBGP, weighted ECMP, and per-peer capability disable
// subtests (RT-1.112.1 through RT-1.112.8).
func TestBGPExtendedNextHopEncoding(t *testing.T) {
	env := setupEnvironment(t, ondatra.DUT(t, "dut"), ondatra.ATE(t, "ate"))

	// The subtests run in README order and share one environment. Each
	// advertises the routes that it is the first to use.
	t.Run("RT-1.112.1: eBGP: receive IPv4 routes with an IPv6 next-hop", func(t *testing.T) {
		testEBGPReceive(t, env)
	})
	t.Run("RT-1.112.2: eBGP to iBGP: next-hop unchanged", func(t *testing.T) {
		testEBGPToIBGP(t, env)
	})
	t.Run("RT-1.112.3: iBGP to eBGP: DUT IPv6 address as next-hop", func(t *testing.T) {
		testIBGPToEBGP(t, env)
	})
	t.Run("RT-1.112.4: iBGP to eBGP over IPv4: DUT IPv4 address as next-hop", func(t *testing.T) {
		testIBGPToEBGPOverIPv4(t, env)
	})
	t.Run("RT-1.112.5: Weighted ECMP over IPv6 next-hops (2:1)", func(t *testing.T) {
		testWeightedECMP(t, env)
	})
	t.Run("RT-1.112.6: Weighted ECMP with equal link bandwidth (1:1)", func(t *testing.T) {
		testEqualLinkBandwidth(t, env)
	})
	t.Run("RT-1.112.7: Extended next-hop encoding disabled for one peer", func(t *testing.T) {
		testEncodingDisabled(t, env)
	})
	t.Run("RT-1.112.8: eBGP to iBGP over IPv4: extended next-hop encoding", func(t *testing.T) {
		testEBGPToIBGPOverIPv4ENH(t, env)
	})
}

// setupEnvironment applies the README Test environment setup and registers
// the README Cleanup. Each Cleanup step is registered before the
// configuration that it reverts is pushed, so that it also runs if a push
// fails part-way. The Cleanups run in reverse order: the ATE protocols are
// stopped before the DUT configuration is removed.
func setupEnvironment(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) *testEnv {
	t.Helper()
	env := &testEnv{
		dut:                   dut,
		ate:                   ate,
		dutPort:               make(map[*atePeer]string),
		atePort:               make(map[*atePeer]string),
		peers:                 slices.Clone(atePeers),
		juniperLocalIPv4Peers: make(map[*atePeer]bool),
		juniperCLIBGPPeers:    make(map[*atePeer]bool),
	}
	var dutPorts []string
	for _, p := range atePeers {
		env.dutPort[p] = dut.Port(t, p.port).Name()
		env.atePort[p] = ate.Port(t, p.port).ID()
		dutPorts = append(dutPorts, env.dutPort[p])
	}

	snapshot := cfgplugins.SnapshotInterfacesConfig(t, dut, dutPorts)
	detach := portsToDetach(t, dut, dutPorts)
	t.Cleanup(func() { removeDUTConfig(t, env, snapshot, detach) })

	t.Log("Configure the DUT ports as in the README table, and enable IPv4 on DUT port-1 and DUT port-2 without an IPv4 address.")
	configureDUTInterfaces(t, env, dutPorts, snapshot)

	t.Logf("Configure BGP on the DUT with AS %d and router ID %s, IPv6 neighbors for ATE port-1 to port-3 and an IPv4 eBGP neighbor for ATE port-4, and apply the %s import and export policy to all neighbors.", dutAS, dutRouterID, acceptAllPolicy)
	configureDUTBGP(t, dut)

	t.Log("Configure the ATE ports as in the README table: ATE port-1 to port-3 each run one BGP peer toward the DUT over IPv6, and ATE port-4 one over IPv4. No route is advertised yet.")
	env.otgCfg, env.bgpPeers = configureATE(t, env)
	otg := ate.OTG()
	t.Cleanup(func() {
		t.Log("Cleanup: stop the ATE traffic and protocols.")
		otg.StopTraffic(t)
		otg.StopProtocols(t)
	})
	pushAndStartATE(t, env, env.peers, func() {
		awaitDUTInterfaces(t, env)
	})
	return env
}

// configureDUTInterfaces configures the DUT ports from the README table.
func configureDUTInterfaces(t *testing.T, env *testEnv, dutPorts []string, snapshot cfgplugins.InterfaceConfigSnapshot) {
	t.Helper()
	dut := env.dut
	// The DEFAULT network instance always exists; this idempotent update only
	// sets its type, so it is not reverted.
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	b := &gnmi.SetBatch{}
	for _, p := range atePeers {
		name := env.dutPort[p]
		intf := p.dut.NewOCInterface(name, dut)
		// NewOCInterface allocates an empty intf.Ethernet; copy the sanitized
		// snapshot Ethernet when p.dut sets no Ethernet fields so BatchReplace
		// preserves baseline PortSpeed/AutoNegotiate/DuplexMode.
		if snap := snapshot[name]; snap != nil && snap.GetEthernet() != nil && p.dut.MAC == "" && p.dut.Duplex == "" {
			intf.Ethernet = snap.GetEthernet()
		}
		if p.dut.IPv4 == "" {
			// attrs adds no ipv4 container without an IPv4 address. As in the
			// Canonical OC, IPv4 is enabled on DUT port-1 and port-2 without
			// an address, so that they forward IPv4 traffic to the IPv6
			// next-hops of IPv4 routes.
			sub := intf.GetOrCreateSubinterface(0)
			if !deviations.IPv4MissingEnabled(dut) {
				sub.GetOrCreateIpv4().SetEnabled(true)
			}
			if deviations.InterfaceEnabled(dut) {
				sub.GetOrCreateIpv6().SetEnabled(true)
			}
		}
		gnmi.BatchReplace(b, gnmi.OC().Interface(name).Config(), intf)
	}
	cfgplugins.AssignInterfacesToDefaultNetworkInstance(t, dut, b, cfgplugins.DefaultNetworkInstanceInterfacesConfig{Interfaces: dutPorts})
	b.Set(t, dut)

	if deviations.IPv4MissingEnabled(dut) {
		switch dut.Vendor() {
		case ondatra.CISCO:
			var setCLI, clearCLI strings.Builder
			for _, p := range atePeers {
				if p.dut.IPv4 == "" {
					fmt.Fprintf(&setCLI, "interface %s\n ipv4 forwarding-enable\n", env.dutPort[p])
					fmt.Fprintf(&clearCLI, "interface %s\n no ipv4 forwarding-enable\n", env.dutPort[p])
				}
			}
			if setCLI.Len() > 0 {
				helpers.GnmiCLIConfig(t, dut, setCLI.String())
				env.intfCLICleanup = append(env.intfCLICleanup, clearCLI.String())
			}
		}
	}

	if deviations.ExplicitPortSpeed(dut) {
		for _, p := range atePeers {
			fptest.SetPortSpeed(t, dut.Port(t, p.port))
		}
	}
}

// awaitDUTInterfaces waits until every DUT port is up and reports the
// addresses of the README table, which the BGP sessions need. Each leaf is
// watched individually rather than via gnmi.OCBatch so multi-leaf STATE
// subscriptions do not stall on Junos EVO.
func awaitDUTInterfaces(t *testing.T, env *testEnv) {
	t.Helper()
	deadline := time.Now().Add(stateTimeout)
	var problems []string
	for _, p := range atePeers {
		name := env.dutPort[p]
		intf := gnmi.OC().Interface(name)
		if s, ok := awaitStateValue(t, env.dut, intf.OperStatus().State(), deadline, oc.Interface_OperStatus_UP); !ok {
			problems = append(problems, fmt.Sprintf("%s oper-status %v, want UP", name, s))
		}
		sub := intf.Subinterface(0)
		if got, ok := awaitStateValue(t, env.dut, sub.Ipv6().Address(p.dut.IPv6).PrefixLength().State(), deadline, p.dut.IPv6Len); !ok {
			problems = append(problems, fmt.Sprintf("%s %s prefix-length %d, want %d", name, p.dut.IPv6, got, p.dut.IPv6Len))
		}
		if p.dut.IPv4 == "" {
			continue
		}
		if got, ok := awaitStateValue(t, env.dut, sub.Ipv4().Address(p.dut.IPv4).PrefixLength().State(), deadline, p.dut.IPv4Len); !ok {
			problems = append(problems, fmt.Sprintf("%s %s prefix-length %d, want %d", name, p.dut.IPv4, got, p.dut.IPv4Len))
		}
	}
	if len(problems) > 0 {
		t.Fatalf("DUT ports not ready: %s", strings.Join(problems, "; "))
	}
}

// configureDUTBGP configures the accept-all policy and BGP from the README
// Test environment setup. Neither extended next-hop encoding nor multipath is
// configured globally, and no next-hop-self is configured.
func configureDUTBGP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	b := &gnmi.SetBatch{}
	if _, err := cfgplugins.ConfigureBGPRoutePolicy(t, b, cfgplugins.BGPPolicyConfig{PolicyName: acceptAllPolicy, StatementID: "accept-all"}); err != nil {
		t.Fatalf("ConfigureBGPRoutePolicy(%q) failed: %v", acceptAllPolicy, err)
	}
	// ConfigureDUTBGP adds to b the global AS and router ID, with IPV4_UNICAST
	// and IPV6_UNICAST enabled. The neighbors are added to the protocol that
	// it returns before b is applied.
	bgp := cfgplugins.ConfigureDUTBGP(t, dut, b, cfgplugins.BGPConfig{DutAS: dutAS, RouterID: dutRouterID}).GetOrCreateBgp()
	for _, p := range atePeers {
		addNeighbor(dut, bgp, p)
	}
	b.Set(t, dut)
	gnmi.Await(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(acceptAllPolicy).Name().State(), stateTimeout, acceptAllPolicy)

	// Some devices stream the BGP Loc-RIB of RT-1.112.1 and RT-1.112.3 only
	// after a native telemetry setting. The setting only enables telemetry
	// and is often part of the device baseline, so it is not reverted.
	if deviations.BgpRibStreamingConfigRequired(dut) {
		cfgplugins.DeviationBgpRibStreamingConfigRequired(t, dut)
	}
}

// addNeighbor adds the DUT BGP neighbor of the ATE peer p to bgp. IPv6
// neighbors enable IPV4_UNICAST and IPV6_UNICAST; IPv4 neighbors enable only
// IPV4_UNICAST. Extended next-hop encoding is enabled for IPV4_UNICAST when
// p.enableENH is set. The accept-all policy is applied for import and export
// to each AFI-SAFI, or to the neighbor on devices without AFI-SAFI policies.
func addNeighbor(dut *ondatra.DUTDevice, bgp *oc.NetworkInstance_Protocol_Bgp, p *atePeer) {
	// OpenConfig requires a referenced peer-group to exist, and some platforms
	// require every neighbor to belong to one.
	pgName := p.peerGroup()
	pg := bgp.GetOrCreatePeerGroup(pgName)
	pg.SetPeerGroupName(pgName)
	pg.SetPeerAs(p.asn)

	nbr := bgp.GetOrCreateNeighbor(p.neighbor())
	nbr.SetPeerGroup(pgName)
	nbr.SetPeerAs(p.asn)
	nbr.SetEnabled(true)
	afiSafis := []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST}
	if !p.ipv4Session {
		afiSafis = append(afiSafis, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	}
	for _, afiSafi := range afiSafis {
		af := nbr.GetOrCreateAfiSafi(afiSafi)
		af.SetEnabled(true)
		if !deviations.RoutePolicyUnderAFIUnsupported(dut) {
			policy := af.GetOrCreateApplyPolicy()
			policy.SetImportPolicy([]string{acceptAllPolicy})
			policy.SetExportPolicy([]string{acceptAllPolicy})
		}
	}
	if p.enableENH && !deviations.BGPGlobalExtendedNextHopEncodingUnsupported(dut) {
		nbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().SetExtendedNextHopEncoding(true)
	}
	// With RoutePolicyUnderAFIUnsupported, this applies the policies to the
	// neighbor; otherwise it does nothing.
	cfgplugins.ConfigurePeer(nbr, dut, cfgplugins.ApplyPeerPerAfiSafiRoutingPolicy(true, acceptAllPolicy, acceptAllPolicy, false))
}

// configureATE returns the ATE configuration of the README table: each ATE
// port with its interface and one BGP peer toward the DUT, without routes or
// flows. It also returns the IPv6 BGP peers, to which the subtests add their
// routes.
func configureATE(t *testing.T, env *testEnv) (gosnappi.Config, map[*atePeer]gosnappi.BgpV6Peer) {
	t.Helper()
	cfg := gosnappi.NewConfig()
	peers := make(map[*atePeer]gosnappi.BgpV6Peer)
	for _, p := range atePeers {
		// ATE port-1 and port-2 have no IPv4 address, so they get only an
		// IPv6 interface.
		dev := p.ate.AddToOTG(cfg, env.ate.Port(t, p.port), &p.dut)
		bgp := dev.Bgp().SetRouterId(p.routerID)
		if p == atePort3 {
			env.atePort3BGP = bgp
		}
		if p.ipv4Session {
			addATEBGPv4Peer(bgp, p)
			continue
		}
		asType := gosnappi.BgpV6PeerAsType.EBGP
		if p.ibgp {
			asType = gosnappi.BgpV6PeerAsType.IBGP
		}
		peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(p.ate.Name + ".IPv6").Peers().Add().
			SetName(p.bgpPeerName()).
			SetPeerAddress(p.dut.IPv6).
			SetAsNumber(p.asn).
			SetAsType(asType)
		peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true).SetExtendedNextHopEncoding(true)
		filter := peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
		if !p.ibgp {
			filter.SetUnicastIpv6Prefix(true)
		}
		peers[p] = peer
	}
	return cfg, peers
}

// addATEBGPv4Peer adds an IPv4 BGP peer for p to bgp with IPV4_UNICAST
// enabled, IPV6_UNICAST disabled, and extended next-hop encoding set from
// p.enableENH.
func addATEBGPv4Peer(bgp gosnappi.DeviceBgpRouter, p *atePeer) {
	asType := gosnappi.BgpV4PeerAsType.EBGP
	if p.ibgp {
		asType = gosnappi.BgpV4PeerAsType.IBGP
	}
	peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(p.ate.Name + ".IPv4").Peers().Add().
		SetName(p.bgpPeerName()).
		SetPeerAddress(p.dut.IPv4).
		SetAsNumber(p.asn).
		SetAsType(asType)
	// gosnappi enables the IPV6_UNICAST capability by default, so it is
	// disabled explicitly.
	peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(false).SetExtendedNextHopEncoding(p.enableENH)
	peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
}

// pushAndStartATE pushes env.otgCfg, optionally runs afterPush before starting
// ATE protocols (for example waiting for DUT interfaces once ATE L1 config
// is pushed), starts protocols, waits for ARP/NDP, and waits until waitPeers
// reach ESTABLISHED.
func pushAndStartATE(t *testing.T, env *testEnv, waitPeers []*atePeer, afterPush func()) {
	t.Helper()
	otg := env.ate.OTG()
	otg.PushConfig(t, env.otgCfg)
	if afterPush != nil {
		afterPush()
	}
	otg.StartProtocols(t)
	// WaitForARP skips the interfaces without an address of the family, such
	// as ATE port-1 and port-2 for IPv4.
	otgutils.WaitForARP(t, otg, env.otgCfg, "IPv4")
	otgutils.WaitForARP(t, otg, env.otgCfg, "IPv6")
	awaitSessionsEstablished(t, env.dut, waitPeers)
}

// startATEProtocols pushes the ATE configuration, starts the ATE protocols,
// and waits for ARP and IPv6 neighbor discovery and for all DUT BGP sessions
// in env.peers to be ESTABLISHED. A push replaces the whole ATE configuration
// and restarts all ATE BGP sessions, so the wait for the sessions is a
// precondition of the checks that follow, not a README check.
func startATEProtocols(t *testing.T, env *testEnv) {
	t.Helper()
	pushAndStartATE(t, env, env.peers, nil)
}

// addIPv4Route adds the IPv4 route r to the BGP peer of its ATE port, with
// the global IPv6 address of the port as the only next-hop, so that no
// link-local next-hop is advertised.
func addIPv4Route(t *testing.T, env *testEnv, r *advertisedRoute) gosnappi.BgpV4RouteRange {
	t.Helper()
	prefix := mustParsePrefix(t, r.prefix)
	routes := env.bgpPeers[r.peer].V4Routes().Add().SetName(r.name).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV6).
		SetNextHopIpv6Address(r.peer.ate.IPv6)
	routes.Addresses().Add().SetAddress(prefix.Addr().String()).SetPrefix(uint32(prefix.Bits()))
	return routes
}

// addIPv6Route adds the IPv6 route r to the BGP peer of its ATE port, with
// the global IPv6 address of the port as the only next-hop.
func addIPv6Route(t *testing.T, env *testEnv, r *advertisedRoute) {
	t.Helper()
	prefix := mustParsePrefix(t, r.prefix)
	routes := env.bgpPeers[r.peer].V6Routes().Add().SetName(r.name).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopIpv6Address(r.peer.ate.IPv6)
	routes.Addresses().Add().SetAddress(prefix.Addr().String()).SetPrefix(uint32(prefix.Bits()))
}

// addECMPRoute adds the RT-1.112.5 route r with ORIGIN IGP, an AS_PATH of
// only the AS of its ATE port, so that both ECMP routes have the same ORIGIN
// and AS-path length, and a link bandwidth extended community with that AS
// and the bandwidth in bytes per second. It returns the community.
func addECMPRoute(t *testing.T, env *testEnv, r *advertisedRoute, bandwidth float32) gosnappi.BgpExtendedCommunityNonTransitive2OctetAsTypeLinkBandwidth {
	t.Helper()
	routes := addIPv4Route(t, env, r)
	routes.Advanced().SetIncludeOrigin(true).SetOrigin(gosnappi.BgpRouteAdvancedOrigin.IGP)
	routes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SEQ)
	return routes.ExtendedCommunities().Add().NonTransitive2OctetAsType().LinkBandwidthSubtype().
		SetGlobal2ByteAs(r.peer.asn).
		SetBandwidth(bandwidth)
}

// addFlow adds the flow f to the ATE configuration: trafficPackets packets at
// trafficPPS from ATE port-4 to the destination of f, counted by the ATE
// ports that advertise the routes of f.
func addFlow(env *testEnv, f *trafficFlow) {
	ipv6 := isIPv6Addr(f.dst)
	src, tx := atePort4.ate.IPv4, atePort4.ate.Name+".IPv4"
	if ipv6 {
		src, tx = atePort4.ate.IPv6, atePort4.ate.Name+".IPv6"
	}
	var rx []string
	for _, r := range f.routes {
		rx = append(rx, r.name)
	}
	flow := env.otgCfg.Flows().Add().SetName(f.name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{tx}).SetRxNames(rx)
	flow.Rate().SetPps(trafficPPS)
	flow.Duration().FixedPackets().SetPackets(trafficPackets)
	flow.Size().SetFixed(frameSize)
	flow.Packet().Add().Ethernet().Src().SetValue(atePort4.ate.MAC)
	if ipv6 {
		ip := flow.Packet().Add().Ipv6()
		ip.Src().SetValue(src)
		ip.Dst().SetValue(f.dst)
	} else {
		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue(src)
		ip.Dst().SetValue(f.dst)
	}
	if f.randomUDP {
		udp := flow.Packet().Add().Udp()
		udp.SrcPort().SetValues(randomPorts(udpSrcPorts))
		udp.DstPort().SetValues(randomPorts(udpDstPorts))
	}
}

// randomPorts returns n random ports of the dynamic port range (RFC 6335).
func randomPorts(n int) []uint32 {
	ports := make([]uint32, n)
	for i := range ports {
		ports[i] = uint32(49152 + rand.IntN(16384))
	}
	return ports
}

// testEBGPReceive implements RT-1.112.1.
func testEBGPReceive(t *testing.T, env *testEnv) {
	t.Logf("%v advertises %s and %s.", atePort1, port1IPv4Route.prefix, port1IPv6Route.prefix)
	addIPv4Route(t, env, port1IPv4Route)
	addIPv6Route(t, env, port1IPv6Route)
	addFlow(env, port1IPv4Flow)
	addFlow(env, port1IPv6Flow)
	startATEProtocols(t, env)

	t.Log("Verify that the sessions to ATE port-1, port-2 and port-3 are ESTABLISHED, that IPV4_UNICAST and IPV6_UNICAST are active, and that extended next-hop encoding is enabled and EXTENDED_NEXTHOP_ENCODING is a negotiated capability.")
	verifyENHSessions(t, env.dut, atePort1, atePort2, atePort3)

	if deviations.BgpExtendedNexthopInstalledPrefixesUnsupported(env.dut) {
		t.Logf("Verify that the DUT reports 1 received IPv4 prefix from %v (skipping prefixes/installed check because BgpExtendedNexthopInstalledPrefixesUnsupported is set).", atePort1)
		verifyIPv4Prefixes(t, env.dut, atePort1, "received 1", func(c *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Prefixes) bool {
			return c.GetReceived() == 1
		})
	} else {
		t.Logf("Verify that the DUT reports 1 received and 1 installed IPv4 prefix from %v.", atePort1)
		verifyIPv4Prefixes(t, env.dut, atePort1, "received 1, installed 1", func(c *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Prefixes) bool {
			return c.GetReceived() == 1 && c.GetInstalled() == 1
		})
	}

	t.Logf("Verify that the DUT Loc-RIB and AFT have %s, and the AFT has %s, each with the single next-hop %s via DUT port-1.", port1IPv4Route.prefix, port1IPv6Route.prefix, atePort1.ate.IPv6)
	verifyLocRIB(t, env.dut, port1IPv4Route, true)
	verifyAFTNextHop(t, env, port1IPv4Route, true)
	verifyAFTNextHop(t, env, port1IPv6Route, true)

	t.Logf("Send IPv4 traffic to %s and IPv6 traffic to %s; verify that %v receives both flows.", port1IPv4Flow.dst, port1IPv6Flow.dst, atePort1)
	rx, tx := sendTraffic(t, env, []*trafficFlow{port1IPv4Flow, port1IPv6Flow}, atePort1)
	verifyReceived(t, atePort1, rx, tx)
}

// testEBGPToIBGP implements RT-1.112.2 with the route that RT-1.112.1
// advertises.
func testEBGPToIBGP(t *testing.T, env *testEnv) {
	t.Logf("Verify that the DUT reports at least 1 IPv4 prefix sent to %v, and that %v receives %s with next-hop %s. The next-hop is not changed toward an iBGP peer (RFC 4271).", atePort3, atePort3, port1IPv4Route.prefix, atePort1.ate.IPv6)
	verifyIPv4Prefixes(t, env.dut, atePort3, "sent at least 1", func(c *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Prefixes) bool {
		return c.GetSent() >= 1
	})
	verifyATERoute(t, env, atePort3, port1IPv4Route.prefix, atePort1.ate.IPv6)
}

// testIBGPToEBGP implements RT-1.112.3.
func testIBGPToEBGP(t *testing.T, env *testEnv) {
	t.Logf("%v advertises %s.", atePort3, port3IPv4Route.prefix)
	addIPv4Route(t, env, port3IPv4Route)
	addFlow(env, port3Flow)
	startATEProtocols(t, env)

	t.Logf("Verify that the DUT Loc-RIB and AFT have %s with next-hop %s via DUT port-3.", port3IPv4Route.prefix, atePort3.ate.IPv6)
	verifyLocRIB(t, env.dut, port3IPv4Route, false)
	verifyAFTNextHop(t, env, port3IPv4Route, false)

	t.Logf("Verify that %v receives %s with next-hop %s and %v with %s; an additional link-local next-hop is allowed. Toward an eBGP peer the DUT uses its own address on the link (RFC 4271).", atePort1, port3IPv4Route.prefix, atePort1.dut.IPv6, atePort2, atePort2.dut.IPv6)
	verifyATERoute(t, env, atePort1, port3IPv4Route.prefix, atePort1.dut.IPv6)
	verifyATERoute(t, env, atePort2, port3IPv4Route.prefix, atePort2.dut.IPv6)

	t.Logf("Send IPv4 traffic to %s; verify that %v receives it.", port3Flow.dst, atePort3)
	rx, tx := sendTraffic(t, env, []*trafficFlow{port3Flow}, atePort3)
	verifyReceived(t, atePort3, rx, tx)
}

// testIBGPToEBGPOverIPv4 implements RT-1.112.4 with the route that RT-1.112.3
// advertises.
func testIBGPToEBGPOverIPv4(t *testing.T, env *testEnv) {
	t.Logf("Verify that the session to %v is ESTABLISHED, with IPV4_UNICAST active and extended next-hop encoding not negotiated.", atePort4)
	n, ok := watchNeighbor(t, env.dut, atePort4, sessionTimeout, func(n *oc.NetworkInstance_Protocol_Bgp_Neighbor) bool {
		return n.GetSessionState() == oc.Bgp_Neighbor_SessionState_ESTABLISHED &&
			n.GetAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetActive() &&
			len(n.GetSupportedCapabilities()) > 0
	})
	switch caps := n.GetSupportedCapabilities(); {
	case !ok:
		t.Errorf("DUT neighbor %s (%v): session-state %v, IPV4_UNICAST active %t, supported-capabilities %v; want ESTABLISHED, active and the negotiated capabilities",
			atePort4.neighbor(), atePort4, n.GetSessionState(), n.GetAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetActive(), caps)
	case slices.Contains(caps, oc.BgpTypes_BGP_CAPABILITY_EXTENDED_NEXTHOP_ENCODING):
		t.Errorf("DUT neighbor %s (%v): supported-capabilities %v, want no EXTENDED_NEXTHOP_ENCODING", atePort4.neighbor(), atePort4, caps)
	}

	t.Logf("Verify that %v receives %s with next-hop %s, the DUT IPv4 address on the link. Without extended next-hop encoding on this session, the DUT must use its own IPv4 address.", atePort4, port3IPv4Route.prefix, atePort4.dut.IPv4)
	verifyATERoute(t, env, atePort4, port3IPv4Route.prefix, atePort4.dut.IPv4)
}

// testWeightedECMP implements RT-1.112.5.
func testWeightedECMP(t *testing.T, env *testEnv) {
	t.Logf("On the DUT, enable eBGP multipath for IPV4_UNICAST with up to %d paths from different ASes, and link-bandwidth based load balancing, as in the Canonical OC.", maxPaths)
	enableMultipath(t, env)

	t.Logf("%v and %v advertise %s with the same ORIGIN and AS-path length, and with link-bandwidth:%d:20M and link-bandwidth:%d:10M.", atePort1, atePort2, port1ECMPRoute.prefix, atePort1.asn, atePort2.asn)
	env.port1LBW = addECMPRoute(t, env, port1ECMPRoute, bandwidth20M)
	addECMPRoute(t, env, port2ECMPRoute, bandwidth10M)
	addFlow(env, ecmpFlow)
	startATEProtocols(t, env)

	verifyWeightedECMP(t, env, ecmp2To1)
}

// testEqualLinkBandwidth implements RT-1.112.6.
func testEqualLinkBandwidth(t *testing.T, env *testEnv) {
	if env.port1LBW == nil {
		t.Fatalf("RT-1.112.5 did not add the %v route of %s", atePort1, port1ECMPRoute.prefix)
	}
	t.Logf("%v re-advertises %s with link-bandwidth:%d:10M.", atePort1, port1ECMPRoute.prefix, atePort1.asn)
	env.port1LBW.SetBandwidth(bandwidth10M)
	startATEProtocols(t, env)

	t.Logf("Repeat the checks of RT-1.112.5, expecting a weight ratio between %.2f and %.2f and %.0f%% of the traffic on each port.", ecmp1To1.minRatio, ecmp1To1.maxRatio, ecmp1To1.port1Pct)
	verifyWeightedECMP(t, env, ecmp1To1)
}

// testEncodingDisabled implements RT-1.112.7.
func testEncodingDisabled(t *testing.T, env *testEnv) {
	if env.port1LBW == nil {
		t.Fatalf("RT-1.112.5 did not add the routes and the flow of %s", port1ECMPRoute.prefix)
	}
	dut := env.dut
	nbr := bgpPath(dut).Neighbor(atePort1.neighbor())

	t.Logf("%v is not changed and still advertises the extended next-hop encoding capability.", atePort1)
	t.Logf("On the DUT, disable extended next-hop encoding for IPV4_UNICAST of the %v neighbor, then reset the session.", atePort1)
	setPeerENH(t, env, atePort1, false)
	resetSession(t, dut, atePort1)

	t.Log("Verify that the encoding is reported as disabled for the neighbor.")
	verifyENHDisabled(t, dut, atePort1)

	t.Log("The session may stay down or reset; if it is ESTABLISHED, verify that the capability is not negotiated. A capability is used only when both peers advertise it (RFC 5492).")
	n, established := watchNeighbor(t, dut, atePort1, optionalSessionTimeout, func(n *oc.NetworkInstance_Protocol_Bgp_Neighbor) bool {
		return n.GetSessionState() == oc.Bgp_Neighbor_SessionState_ESTABLISHED && len(n.GetSupportedCapabilities()) > 0
	})
	switch caps := n.GetSupportedCapabilities(); {
	case established && slices.Contains(caps, oc.BgpTypes_BGP_CAPABILITY_EXTENDED_NEXTHOP_ENCODING):
		t.Errorf("DUT neighbor %s (%v): supported-capabilities %v, want no EXTENDED_NEXTHOP_ENCODING", atePort1.neighbor(), atePort1, caps)
	case established:
		t.Logf("The session to %v is ESTABLISHED without EXTENDED_NEXTHOP_ENCODING (supported-capabilities %v).", atePort1, caps)
	case n.GetSessionState() == oc.Bgp_Neighbor_SessionState_ESTABLISHED:
		t.Errorf("DUT neighbor %s (%v): ESTABLISHED without supported-capabilities, want them reported without EXTENDED_NEXTHOP_ENCODING", atePort1.neighbor(), atePort1)
	default:
		t.Logf("The session to %v is %v after %v, which the README allows.", atePort1, n.GetSessionState(), optionalSessionTimeout)
	}

	t.Logf("Verify that the DUT installs no IPv4 route from %v: %s is not in the AFT, %s has only the next-hop %s, and the installed IPv4 prefix count for the neighbor is 0 or not reported.", atePort1, port1IPv4Route.prefix, port1ECMPRoute.prefix, atePort2.ate.IPv6)
	awaitAFT(t, dut, []string{port1IPv4Route.prefix, port1ECMPRoute.prefix}, func(afts *oc.NetworkInstance_Afts) error {
		if _, ok := entryNextHops(afts, port1IPv4Route.prefix); ok {
			return fmt.Errorf("%s is in the AFT, want it absent", port1IPv4Route.prefix)
		}
		return checkNextHop(dut, afts, port1ECMPRoute.prefix, atePort2.ate.IPv6, env.dutPort[atePort2], true)
	})
	t.Logf("Verify that %v receives no IPv4 route with an IPv6 next-hop, including an IPv4-mapped one. Routes with an IPv4 next-hop are allowed.", atePort1)
	// The AFT check above already holds once the session goes down. These
	// watches run at the same time for absenceWindow and report an ATE
	// port-1 route that the DUT installs or a route that ATE port-1 receives
	// later, for example once the session is ESTABLISHED again.
	ni := deviations.DefaultNetworkInstance(dut)
	aftRoutes := gnmi.Watch(t, dut, aftQuery(dut, port1IPv4Route.prefix, port1ECMPRoute.prefix), absenceWindow, func(v *ygnmi.Value[*oc.Root]) bool {
		root, _ := v.Val()
		return port1RouteInAFT(env, root.GetNetworkInstance(ni).GetAfts()) != ""
	})
	installed := gnmi.Watch(t, dut, nbr.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes().Installed().State(), absenceWindow, func(v *ygnmi.Value[uint32]) bool {
		count, present := v.Val()
		return present && count > 0
	})
	ateRoutes := gnmi.WatchAll(t, env.ate.OTG(), gnmi.OTG().BgpPeer(atePort1.bgpPeerName()).UnicastIpv4PrefixAny().State(), absenceWindow, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
		r, present := v.Val()
		return present && hasIPv6NextHop(r)
	})
	if v, found := aftRoutes.Await(t); found {
		root, _ := v.Val()
		t.Errorf("DUT AFT: %s, want no IPv4 route from %v", port1RouteInAFT(env, root.GetNetworkInstance(ni).GetAfts()), atePort1)
	} else {
		awaitAFT(t, dut, []string{port1IPv4Route.prefix, port1ECMPRoute.prefix}, func(afts *oc.NetworkInstance_Afts) error {
			if _, ok := entryNextHops(afts, port1IPv4Route.prefix); ok {
				return fmt.Errorf("%s appeared in the AFT after %v, want it absent", port1IPv4Route.prefix, absenceWindow)
			}
			return checkNextHop(dut, afts, port1ECMPRoute.prefix, atePort2.ate.IPv6, env.dutPort[atePort2], true)
		})
	}
	if v, found := installed.Await(t); found {
		count, _ := v.Val()
		t.Errorf("DUT neighbor %s (%v): %d installed IPv4 prefixes, want 0 or not reported", atePort1.neighbor(), atePort1, count)
	}
	if v, found := ateRoutes.Await(t); found {
		r, _ := v.Val()
		t.Errorf("%v received %s/%d with next-hop %s, want no IPv4 route with an IPv6 next-hop", atePort1, r.GetAddress(), r.GetPrefixLength(), r.GetNextHopIpv6Address())
	}

	t.Logf("Send the flow of RT-1.112.5; verify that %v receives at least %.0f%% of it.", atePort2, minRxPct)
	pushAndStartATE(t, env, []*atePeer{atePort2, atePort3, atePort4}, nil)
	awaitAFT(t, dut, []string{port1IPv4Route.prefix, port1ECMPRoute.prefix}, func(afts *oc.NetworkInstance_Afts) error {
		if _, ok := entryNextHops(afts, port1IPv4Route.prefix); ok {
			return fmt.Errorf("%s is in the AFT, want it absent", port1IPv4Route.prefix)
		}
		return checkNextHop(dut, afts, port1ECMPRoute.prefix, atePort2.ate.IPv6, env.dutPort[atePort2], true)
	})
	rx, tx := sendTraffic(t, env, []*trafficFlow{ecmpFlow}, atePort1, atePort2)
	verifyReceived(t, atePort2, rx, tx)

	t.Log("Re-enable the encoding and reset the session in the same way.")
	setPeerENH(t, env, atePort1, true)
	resetSession(t, dut, atePort1)

	t.Logf("Verify that the session and AFT checks of RT-1.112.1 for %v and %s pass again.", atePort1, port1IPv4Route.prefix)
	verifyENHSessions(t, dut, atePort1)
	verifyAFTNextHop(t, env, port1IPv4Route, true)

	t.Logf("Verify that the %v check of RT-1.112.3 passes again: it receives %s with next-hop %s.", atePort1, port3IPv4Route.prefix, atePort1.dut.IPv6)
	verifyATERoute(t, env, atePort1, port3IPv4Route.prefix, atePort1.dut.IPv6)

	t.Log("Verify that RT-1.112.6 passes again.")
	startATEProtocols(t, env)
	verifyWeightedECMP(t, env, ecmp1To1)
}

// testEBGPToIBGPOverIPv4ENH implements RT-1.112.8 with the route that
// RT-1.112.1 advertises. It is scoped to Arista DUTs.
func testEBGPToIBGPOverIPv4ENH(t *testing.T, env *testEnv) {
	dut := env.dut
	if dut.Vendor() != ondatra.ARISTA {
		t.Skipf("Skipping RT-1.112.8: IPv4 iBGP extended next-hop is only in scope for Arista DUTs (got %s).", dut.Vendor())
	}
	if env.atePort3BGP == nil {
		t.Fatalf("setupEnvironment did not initialize %v OTG BGP configuration", atePort3)
	}

	t.Logf("On the DUT, add an IPv4 iBGP neighbor for %v (%s, AS %d) with IPV4_UNICAST and extended next-hop encoding enabled, and the %s import and export policies applied.", atePort3IPv4, atePort3IPv4.neighbor(), atePort3IPv4.asn, acceptAllPolicy)
	bgp := &oc.NetworkInstance_Protocol_Bgp{}
	addNeighbor(dut, bgp, atePort3IPv4)
	b := &gnmi.SetBatch{}
	gnmi.BatchUpdate(b, bgpPath(dut).PeerGroup(atePort3IPv4.peerGroup()).Config(), bgp.GetPeerGroup(atePort3IPv4.peerGroup()))
	gnmi.BatchUpdate(b, bgpPath(dut).Neighbor(atePort3IPv4.neighbor()).Config(), bgp.GetNeighbor(atePort3IPv4.neighbor()))
	b.Set(t, dut)

	t.Logf("On %v, add a second BGP peer toward the DUT over IPv4 (%s, AS %d, iBGP) with IPV4_UNICAST and the extended next-hop encoding capability enabled.", atePort3, atePort3IPv4.dut.IPv4, atePort3IPv4.asn)
	if !slices.Contains(env.peers, atePort3IPv4) {
		env.peers = append(env.peers, atePort3IPv4)
	}
	addATEBGPv4Peer(env.atePort3BGP, atePort3IPv4)
	startATEProtocols(t, env)

	t.Logf("Verify that the IPv4 iBGP session %s is ESTABLISHED, IPV4_UNICAST is active, extended next-hop encoding is enabled, and EXTENDED_NEXTHOP_ENCODING is a negotiated capability.", atePort3IPv4.neighbor())
	verifyENHSessions(t, dut, atePort3IPv4)

	t.Logf("Verify that the DUT reports at least 1 IPv4 prefix sent to %s, and that %v receives %s with next-hop %s unchanged.", atePort3IPv4.neighbor(), atePort3IPv4, port1IPv4Route.prefix, atePort1.ate.IPv6)
	verifyIPv4Prefixes(t, dut, atePort3IPv4, "sent at least 1", func(c *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Prefixes) bool {
		return c.GetSent() >= 1
	})
	verifyATERoute(t, env, atePort3IPv4, port1IPv4Route.prefix, atePort1.ate.IPv6)
}

// enableMultipath enables eBGP multipath for IPV4_UNICAST with up to
// maxPaths paths from different ASes, and link-bandwidth based load
// balancing, at the paths of the README Canonical OC (or their platform
// deviation equivalents).
func enableMultipath(t *testing.T, env *testEnv) {
	t.Helper()
	dut := env.dut
	if deviations.MultipathUnsupportedNeighborOrAfisafi(dut) {
		bgp := &oc.NetworkInstance_Protocol_Bgp{}
		b := &gnmi.SetBatch{}
		for _, p := range []*atePeer{atePort1, atePort2} {
			mp := bgp.GetOrCreatePeerGroup(p.peerGroup()).GetOrCreateUseMultiplePaths()
			mp.SetEnabled(true)
			if !deviations.SkipSettingAllowMultipleAS(dut) {
				mp.GetOrCreateEbgp().SetAllowMultipleAs(true)
			}
			gnmi.BatchUpdate(b, bgpPath(dut).PeerGroup(p.peerGroup()).UseMultiplePaths().Config(), mp)
		}
		b.Set(t, dut)
		return
	}

	ni := deviations.DefaultNetworkInstance(dut)
	multipath := (&oc.Root{}).GetOrCreateNetworkInstance(ni).
		GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).
		GetOrCreateBgp().
		GetOrCreateGlobal().
		GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).
		GetOrCreateUseMultiplePaths()
	if deviations.EnableMultipathUnderAfiSafi(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA, ondatra.NOKIA:
			// Arista exposes config/enabled only at /global/use-multiple-paths
			// (not under afi-safi), and Nokia SR Linux lacks
			// use-multiple-paths/config (multipath enables via ebgp/maximum-paths).
		default:
			multipath.SetEnabled(true)
		}
	} else {
		multipath.SetEnabled(true)
	}

	ebgp := multipath.GetOrCreateEbgp()
	if !deviations.BgpMaxMultipathPathsUnsupported(dut) {
		ebgp.SetMaximumPaths(maxPaths)
	}
	if !deviations.RibWecmp(dut) && !deviations.BgpExplicitExtendedCommunityEnable(dut) {
		ebgp.GetOrCreateLinkBandwidthExtCommunity().SetEnabled(true)
	}
	if !deviations.SkipSettingAllowMultipleAS(dut) && !deviations.SkipAfiSafiPathForBgpMultipleAs(dut) {
		ebgp.SetAllowMultipleAs(true)
	}
	gnmi.Update(t, dut, bgpPath(dut).Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).UseMultiplePaths().Config(), multipath)
	if !deviations.SkipSettingAllowMultipleAS(dut) && deviations.SkipAfiSafiPathForBgpMultipleAs(dut) {
		gnmi.Update(t, dut, bgpPath(dut).Global().UseMultiplePaths().Ebgp().AllowMultipleAs().Config(), true)
	}

	if deviations.SkipSettingAllowMultipleAS(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("router bgp %d\n bgp bestpath as-path multipath-relax\n", dutAS))
			env.multipathCLICleanup = append(env.multipathCLICleanup, fmt.Sprintf("router bgp %d\n no bgp bestpath as-path multipath-relax\n", dutAS))
		case ondatra.CISCO:
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("router bgp %d instance %s\n bgp bestpath as-path multipath-relax\n", dutAS, bgpName))
			env.multipathCLICleanup = append(env.multipathCLICleanup, fmt.Sprintf("router bgp %d instance %s\n no bgp bestpath as-path multipath-relax\n", dutAS, bgpName))
		default:
			t.Fatalf("Unsupported vendor %s for deviation 'SkipSettingAllowMultipleAS'", dut.Vendor())
		}
	}
	if deviations.RibWecmp(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			// Enabling link-bandwidth-ext-community on only IPV4_UNICAST causes
			// Arista session gnmi sync to reject it as mismatching IPV6_UNICAST;
			// ucmp mode 1 in CLI enables it directly.
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("router bgp %d\n ucmp mode 1\n", dutAS))
			env.multipathCLICleanup = append(env.multipathCLICleanup, fmt.Sprintf("router bgp %d\n no ucmp mode\n", dutAS))
		default:
			t.Fatalf("Unsupported vendor %s for deviation 'RibWecmp'", dut.Vendor())
		}
	}
	if deviations.BgpExplicitExtendedCommunityEnable(dut) {
		switch dut.Vendor() {
		case ondatra.CISCO:
			var setCLI, clearCLI strings.Builder
			fmt.Fprintf(&setCLI, "router bgp %d instance %s\n", dutAS, bgpName)
			fmt.Fprintf(&clearCLI, "router bgp %d instance %s\n", dutAS, bgpName)
			for _, p := range []*atePeer{atePort1, atePort2} {
				pg := p.peerGroup()
				fmt.Fprintf(&setCLI, " neighbor-group %s\n  dmz-link-bandwidth\n  ebgp-recv-extcommunity-dmz\n  ebgp-send-extcommunity-dmz\n  address-family ipv4 unicast\n   multipath\n", pg)
				fmt.Fprintf(&clearCLI, " neighbor-group %s\n  no dmz-link-bandwidth\n  no ebgp-recv-extcommunity-dmz\n  no ebgp-send-extcommunity-dmz\n  address-family ipv4 unicast\n   no multipath\n", pg)
			}
			helpers.GnmiCLIConfig(t, dut, setCLI.String())
			env.multipathCLICleanup = append(env.multipathCLICleanup, clearCLI.String())
		default:
			t.Fatalf("Unsupported vendor %s for deviation 'BgpExplicitExtendedCommunityEnable'", dut.Vendor())
		}
	}
}

const (
	juniperSetLocalIPv4Tmpl = `protocols {
    bgp {
        group %s {
            neighbor %s {
                family {
                    inet {
                        unicast {
                            local-ipv4-address %s;
                        }
                    }
                }
            }
        }
    }
}`
	juniperDeleteLocalIPv4Tmpl = `protocols {
    bgp {
        group %s {
            neighbor %s {
                family {
                    inet {
                        unicast {
                            extended-nexthop;
                            delete: local-ipv4-address;
                        }
                    }
                }
            }
        }
    }
}`
	juniperDeleteNeighborTmpl = `protocols {
    bgp {
        group %s {
            delete: neighbor %s;
        }
    }
}`
)

// juniperLocalIPv4 returns a non-loopback IPv4 address configured on dut for
// Junos local-ipv4-address, preferring loopback0 and falling back to the first
// DUT interface with an IPv4 address.
func juniperLocalIPv4(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	lb := netutil.LoopbackInterface(t, dut, 0)
	var addrs []*ygnmi.Value[*oc.Interface_Subinterface_Ipv4_Address]
	if msg := testt.CaptureFatal(t, func(tb testing.TB) {
		addrs = gnmi.LookupAll(tb, dut, gnmi.OC().Interface(lb).Subinterface(0).Ipv4().AddressAny().State())
	}); msg != nil {
		t.Logf("juniperLocalIPv4(%s): %s", lb, *msg)
	}
	for _, v := range addrs {
		a, ok := v.Val()
		if !ok || a == nil {
			continue
		}
		if addr, err := netip.ParseAddr(a.GetIp()); err == nil && addr.Is4() && !addr.IsLoopback() {
			return addr.String()
		}
	}
	for _, p := range atePeers {
		if p.dut.IPv4 != "" {
			return p.dut.IPv4
		}
	}
	t.Fatalf("DUT %s: no non-loopback IPv4 address found for Juniper local-ipv4-address", dut.Name())
	return ""
}

// setPeerENH sets extended next-hop encoding for IPV4_UNICAST of the neighbor
// of p to enabled.
func setPeerENH(t *testing.T, env *testEnv, p *atePeer, enabled bool) {
	t.Helper()
	dut := env.dut
	if deviations.BGPGlobalExtendedNextHopEncodingUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.CISCO:
			cmd := "capability suppress extended-nexthop-encoding"
			if enabled {
				cmd = "no " + cmd
			}
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("router bgp %d instance %s\n neighbor %s\n  %s\n", dutAS, bgpName, p.neighbor(), cmd))
		default:
			t.Fatalf("Unsupported vendor %s for deviation 'BGPGlobalExtendedNextHopEncodingUnsupported'", dut.Vendor())
		}
		return
	}
	enh := bgpPath(dut).Neighbor(p.neighbor()).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().ExtendedNextHopEncoding()
	if dut.Vendor() == ondatra.JUNIPER && !enabled {
		// Junos rpd rejects disabling extended-nexthop on an IPv6 neighbor while
		// family inet unicast remains active unless local-ipv4-address is set.
		localIPv4 := juniperLocalIPv4(t, dut)
		env.juniperCLIBGPPeers[p] = true
		env.juniperLocalIPv4Peers[p] = true
		helpers.GnmiCLIConfig(t, dut, fmt.Sprintf(juniperSetLocalIPv4Tmpl, p.peerGroup(), p.neighbor(), localIPv4))
	}
	if dut.Vendor() == ondatra.JUNIPER && enabled && env.juniperLocalIPv4Peers[p] {
		helpers.GnmiCLIConfig(t, dut, fmt.Sprintf(juniperDeleteLocalIPv4Tmpl, p.peerGroup(), p.neighbor()))
		delete(env.juniperLocalIPv4Peers, p)
	}
	gnmi.Replace(t, dut, enh.Config(), enabled)
}

// resetSession resets the DUT BGP session to the ATE port p as the README
// describes: it disables the neighbor, waits until the session is not
// ESTABLISHED, and enables the neighbor again.
func resetSession(t *testing.T, dut *ondatra.DUTDevice, p *atePeer) {
	t.Helper()
	t.Logf("Reset the session to %v: disable the neighbor, wait until the session is not ESTABLISHED, and enable it again.", p)
	nbr := bgpPath(dut).Neighbor(p.neighbor())
	gnmi.Replace(t, dut, nbr.Enabled().Config(), false)
	// The neighbor is enabled again even if the wait fails, so that the
	// remaining steps run with the README configuration.
	defer gnmi.Replace(t, dut, nbr.Enabled().Config(), true)
	_, ok := gnmi.Watch(t, dut, nbr.SessionState().State(), stateTimeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := v.Val()
		return !present || state != oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		t.Errorf("DUT neighbor %s (%v): session still ESTABLISHED %v after the neighbor was disabled", p.neighbor(), p, stateTimeout)
	}
}

// verifyENHDisabled waits until the DUT reports extended next-hop encoding as
// disabled for IPV4_UNICAST of the neighbor of the ATE port p.
func verifyENHDisabled(t *testing.T, dut *ondatra.DUTDevice, p *atePeer) {
	t.Helper()
	if deviations.BGPGlobalExtendedNextHopEncodingUnsupported(dut) {
		t.Logf("Skipping extended-next-hop-encoding state leaf check on %v because BGPGlobalExtendedNextHopEncodingUnsupported is set.", p)
		return
	}
	// Devices with the MissingValueForDefaults deviation report no value for
	// a leaf with its default value, which is false for this leaf.
	missingIsDefault := deviations.MissingValueForDefaults(dut)
	q := bgpPath(dut).Neighbor(p.neighbor()).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().ExtendedNextHopEncoding().State()
	got, ok := gnmi.Watch(t, dut, q, stateTimeout, func(v *ygnmi.Value[bool]) bool {
		enabled, present := v.Val()
		if !present {
			return missingIsDefault
		}
		return !enabled
	}).Await(t)
	if ok {
		return
	}
	desc := "not reported"
	if got != nil {
		if enabled, present := got.Val(); present {
			desc = fmt.Sprint(enabled)
		}
	}
	t.Errorf("DUT neighbor %s (%v): IPV4_UNICAST extended-next-hop-encoding %s, want false", p.neighbor(), p, desc)
}

// verifyWeightedECMP implements the checks of RT-1.112.5, which RT-1.112.6
// and RT-1.112.7 repeat with other expectations: the AFT next-hops of
// 198.51.100.128/25 with their weight ratio, and the traffic share of ATE
// port-1 and ATE port-2.
func verifyWeightedECMP(t *testing.T, env *testEnv, want ecmpExpectation) {
	t.Helper()
	prefix := port1ECMPRoute.prefix
	t.Logf("Verify that the DUT AFT has %s with exactly two next-hops, %s via DUT port-1 and %s via DUT port-2, with a weight ratio between %.2f and %.2f.", prefix, atePort1.ate.IPv6, atePort2.ate.IPv6, want.minRatio, want.maxRatio)
	awaitAFT(t, env.dut, []string{prefix}, func(afts *oc.NetworkInstance_Afts) error {
		ratio, err := ecmpWeightRatio(env, afts)
		if err != nil {
			return err
		}
		if ratio < want.minRatio || ratio > want.maxRatio {
			return fmt.Errorf("%s weight ratio of the ATE port-1 to the ATE port-2 next-hop is %.3f, want between %.2f and %.2f", prefix, ratio, want.minRatio, want.maxRatio)
		}
		return nil
	})

	t.Logf("Send IPv4 traffic to %s with random UDP source and destination ports in each packet; verify that %v receives %.1f%% and %v %.1f%% of it, each within %.0f percentage points.", ecmpFlow.dst, atePort1, want.port1Pct, atePort2, want.port2Pct, ecmpTolerance)
	rx, _ := sendTraffic(t, env, []*trafficFlow{ecmpFlow}, atePort1, atePort2)
	total := rx[atePort1] + rx[atePort2]
	if total == 0 {
		t.Errorf("%v and %v received no frames of flow %s", atePort1, atePort2, ecmpFlow.name)
		return
	}
	for _, s := range []struct {
		peer *atePeer
		want float64
	}{
		{peer: atePort1, want: want.port1Pct},
		{peer: atePort2, want: want.port2Pct},
	} {
		got := 100 * float64(rx[s.peer]) / float64(total)
		if math.Abs(got-s.want) > ecmpTolerance {
			t.Errorf("%v received %.1f%% (%d of %d frames), want %.1f%% +/- %.0f percentage points", s.peer, got, rx[s.peer], total, s.want, ecmpTolerance)
			continue
		}
		t.Logf("%v received %.1f%% (%d of %d frames)", s.peer, got, rx[s.peer], total)
	}
}

// ecmpWeightRatio returns the weight ratio of the ATE port-1 to the ATE
// port-2 next-hop of the RT-1.112.5 prefix in afts, or an error unless the
// AFT entry has exactly these two next-hops, each via the DUT port that
// connects to its ATE port (unless SkipInterfaceNameCheck is set), with
// non-zero weights.
func ecmpWeightRatio(env *testEnv, afts *oc.NetworkInstance_Afts) (float64, error) {
	prefix := port1ECMPRoute.prefix
	nhs, ok := entryNextHops(afts, prefix)
	if !ok {
		return 0, fmt.Errorf("no entry for %s", prefix)
	}
	skipIntf := deviations.SkipInterfaceNameCheck(env.dut)
	weights := make(map[*atePeer]uint64)
	for _, nh := range nhs {
		for _, p := range []*atePeer{atePort1, atePort2} {
			if sameAddr(nh.ip, p.ate.IPv6) && (skipIntf || nh.intf == env.dutPort[p]) {
				weights[p] = nh.weight
			}
		}
	}
	if len(nhs) != 2 || len(weights) != 2 {
		if skipIntf {
			return 0, fmt.Errorf("%s has next-hops %v, want %s and %s", prefix, nhs, atePort1.ate.IPv6, atePort2.ate.IPv6)
		}
		return 0, fmt.Errorf("%s has next-hops %v, want %s via %s and %s via %s", prefix, nhs, atePort1.ate.IPv6, env.dutPort[atePort1], atePort2.ate.IPv6, env.dutPort[atePort2])
	}
	if weights[atePort1] == 0 || weights[atePort2] == 0 {
		return 0, fmt.Errorf("%s has next-hops %v, want non-zero weights", prefix, nhs)
	}
	return float64(weights[atePort1]) / float64(weights[atePort2]), nil
}

// bgpPath returns the path of the DUT BGP instance.
func bgpPath(dut *ondatra.DUTDevice) *netinstbgp.NetworkInstance_Protocol_BgpPath {
	return gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp()
}

// bgpState returns the DUT BGP instance in root, the result of a batch query.
func bgpState(root *oc.Root, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol_Bgp {
	return root.GetNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetBgp()
}

// awaitStateValue watches q until it reports want or deadline expires (using a
// short probe window if deadline has already passed so healthy leaves are not
// reported as unset), returning the last observed value and whether want was
// seen.
func awaitStateValue[T comparable](t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[T], deadline time.Time, want T) (T, bool) {
	t.Helper()
	var last T
	wait := max(time.Until(deadline), 2*time.Second)
	got, ok := gnmi.Watch(t, dut, q, wait, func(v *ygnmi.Value[T]) bool {
		val, present := v.Val()
		if present {
			last = val
		}
		return present && val == want
	}).Await(t)
	if !ok && got != nil {
		if val, present := got.Val(); present {
			last = val
		}
	}
	return last, ok
}

// awaitSessionsEstablished waits until the DUT BGP sessions to all peers are
// ESTABLISHED, and fails the test otherwise. Each peer is watched individually
// rather than via gnmi.OCBatch because multi-neighbor STATE batches on Junos
// EVO can report unset SessionState across all neighbors.
func awaitSessionsEstablished(t *testing.T, dut *ondatra.DUTDevice, peers []*atePeer) {
	t.Helper()
	deadline := time.Now().Add(sessionTimeout)
	var notEstablished []string
	for _, p := range peers {
		q := bgpPath(dut).Neighbor(p.neighbor()).SessionState().State()
		if s, ok := awaitStateValue(t, dut, q, deadline, oc.Bgp_Neighbor_SessionState_ESTABLISHED); !ok {
			notEstablished = append(notEstablished, fmt.Sprintf("%s (%v) %v", p.neighbor(), p, s))
		}
	}
	if len(notEstablished) > 0 {
		t.Fatalf("DUT BGP sessions not ESTABLISHED within %v: %s", sessionTimeout, strings.Join(notEstablished, "; "))
	}
}

// lookupLeaf fetches q on the main test goroutine using unary gNMI Get first
// (except on Juniper, where EVO rejects gNMI Get on operational STATE paths),
// falling back to gNMI Subscribe ONCE for queries where Get fails with an RPC
// error. Errors are logged at most once per mode and leaf in loggedErrs.
func lookupLeaf[T any](
	t *testing.T,
	dut *ondatra.DUTDevice,
	q ygnmi.SingletonQuery[T],
	getUnsupported map[string]bool,
	loggedErrs map[string]bool,
) (T, bool) {
	t.Helper()
	var zero T
	key := fmt.Sprint(q)
	if dut.Vendor() != ondatra.JUNIPER && !getUnsupported[key] {
		var got *ygnmi.Value[T]
		msg := testt.CaptureFatal(t, func(tb testing.TB) {
			got = gnmi.Lookup(tb, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithUseGet()), q)
		})
		if msg == nil {
			if got == nil {
				return zero, false
			}
			return got.Val()
		}
		getUnsupported[key] = true
		errKey := "get:" + key
		if !loggedErrs[errKey] {
			loggedErrs[errKey] = true
			t.Logf("watchNeighbor Get(%v) failed, falling back to Subscribe ONCE: %s", q, *msg)
		}
	}
	var got *ygnmi.Value[T]
	if msg := testt.CaptureFatal(t, func(tb testing.TB) {
		got = gnmi.Lookup(tb, dut, q)
	}); msg != nil {
		errKey := "once:" + key
		if !loggedErrs[errKey] {
			loggedErrs[errKey] = true
			t.Logf("watchNeighbor Lookup(%v): %s", q, *msg)
		}
		return zero, false
	}
	if got == nil {
		return zero, false
	}
	return got.Val()
}

// watchNeighbor waits up to timeout until ready accepts the DUT state of the
// neighbor of the ATE port p: its session state, its negotiated capabilities,
// whether its AFI-SAFIs are active, and its extended next-hop encoding for
// IPV4_UNICAST. It waits once for SessionState == ESTABLISHED with a single
// gNMI STREAM subscription, then polls the required operational leaves via
// unary Get (with Subscribe ONCE fallback) so repeated watches neither exhaust
// device subscription IDs nor cut off slow initial syncs at 5s. It returns the
// last non-nil state received and whether ready accepted it.
func watchNeighbor(t *testing.T, dut *ondatra.DUTDevice, p *atePeer, timeout time.Duration, ready func(*oc.NetworkInstance_Protocol_Bgp_Neighbor) bool) (*oc.NetworkInstance_Protocol_Bgp_Neighbor, bool) {
	t.Helper()
	last := &oc.NetworkInstance_Protocol_Bgp_Neighbor{}
	if timeout <= 0 {
		return last, false
	}
	deadline := time.Now().Add(timeout)
	n := bgpPath(dut).Neighbor(p.neighbor())
	ipv4 := n.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	sessionStateQ := n.SessionState().State()
	capsQ := n.SupportedCapabilities().State()
	ipv4ActiveQ := ipv4.Active().State()
	checkENH := !deviations.BGPGlobalExtendedNextHopEncodingUnsupported(dut)
	enhQ := ipv4.Ipv4Unicast().ExtendedNextHopEncoding().State()
	ipv6ActiveQ := n.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Active().State()

	// Stage 1: wait once up to deadline for SessionState == ESTABLISHED using
	// at most a single STREAM subscription.
	var established bool
	watchErr := false
	if wait := time.Until(deadline); wait > 0 {
		if msg := testt.CaptureFatal(t, func(tb testing.TB) {
			got, ok := gnmi.Watch(tb, dut, sessionStateQ, wait, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
				state, present := v.Val()
				if present {
					last.SessionState = state
				}
				return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			}).Await(tb)
			if got != nil {
				if state, present := got.Val(); present {
					last.SessionState = state
				}
			}
			established = ok
		}); msg != nil {
			watchErr = true
			t.Logf("watchNeighbor Watch(%v): %s", sessionStateQ, *msg)
		}
	}
	if !established && (!watchErr || time.Now().After(deadline)) {
		return last, ready(last)
	}

	// Stage 2: poll operational leaves without opening STREAM subscriptions.
	getUnsupported := make(map[string]bool)
	loggedErrs := make(map[string]bool)
	const pollInterval = 5 * time.Second
	for attempt := 0; ; attempt++ {
		if attempt > 0 && time.Now().After(deadline) {
			return last, false
		}
		cur := &oc.NetworkInstance_Protocol_Bgp_Neighbor{}
		if v, ok := lookupLeaf(t, dut, sessionStateQ, getUnsupported, loggedErrs); ok {
			cur.SessionState = v
		}
		if v, ok := lookupLeaf(t, dut, capsQ, getUnsupported, loggedErrs); ok {
			cur.SupportedCapabilities = v
		}
		if ready(cur) {
			return cur, true
		}
		if v, ok := lookupLeaf(t, dut, ipv4ActiveQ, getUnsupported, loggedErrs); ok {
			cur.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Active = &v
		}
		if checkENH {
			if v, ok := lookupLeaf(t, dut, enhQ, getUnsupported, loggedErrs); ok {
				cur.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().SetExtendedNextHopEncoding(v)
			}
		}
		if !p.ipv4Session {
			if v, ok := lookupLeaf(t, dut, ipv6ActiveQ, getUnsupported, loggedErrs); ok {
				cur.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Active = &v
			}
		}
		last = cur
		if ready(cur) {
			return last, true
		}
		rem := time.Until(deadline)
		if rem <= 0 {
			return last, false
		}
		delay := pollInterval
		if attempt == 0 {
			delay = 2 * time.Second
		}
		time.Sleep(min(delay, rem))
	}
}

// verifyENHSessions verifies the extended next-hop session checks for peers:
// the DUT session is ESTABLISHED, IPV4_UNICAST (and IPV6_UNICAST on IPv6
// sessions) is active, extended next-hop encoding is enabled for IPV4_UNICAST,
// and EXTENDED_NEXTHOP_ENCODING is a negotiated capability.
func verifyENHSessions(t *testing.T, dut *ondatra.DUTDevice, peers ...*atePeer) {
	t.Helper()
	if deviations.BgpExtendedNexthopSupportedCapabilitiesUnsupported(dut) {
		t.Log("Skipping EXTENDED_NEXTHOP_ENCODING supported-capabilities check because BgpExtendedNexthopSupportedCapabilitiesUnsupported is set.")
	}
	for _, p := range peers {
		n, ok := watchNeighbor(t, dut, p, sessionTimeout, func(n *oc.NetworkInstance_Protocol_Bgp_Neighbor) bool {
			return len(enhSessionProblems(dut, p, n)) == 0
		})
		if !ok {
			t.Errorf("DUT neighbor %s (%v): %s", p.neighbor(), p, strings.Join(enhSessionProblems(dut, p, n), "; "))
		}
	}
}

// enhSessionProblems returns the extended next-hop session checks for p that
// the DUT neighbor state n fails.
func enhSessionProblems(dut *ondatra.DUTDevice, p *atePeer, n *oc.NetworkInstance_Protocol_Bgp_Neighbor) []string {
	var problems []string
	if s := n.GetSessionState(); s != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
		problems = append(problems, fmt.Sprintf("session-state %v, want ESTABLISHED", s))
	}
	afiSafis := []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST}
	if !p.ipv4Session {
		afiSafis = append(afiSafis, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	}
	for _, afiSafi := range afiSafis {
		if !n.GetAfiSafi(afiSafi).GetActive() {
			problems = append(problems, fmt.Sprintf("%v not active", afiSafi))
		}
	}
	if !deviations.BGPGlobalExtendedNextHopEncodingUnsupported(dut) && !n.GetAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetIpv4Unicast().GetExtendedNextHopEncoding() {
		problems = append(problems, "IPV4_UNICAST extended-next-hop-encoding not reported as true")
	}
	if !deviations.BgpExtendedNexthopSupportedCapabilitiesUnsupported(dut) {
		if caps := n.GetSupportedCapabilities(); !slices.Contains(caps, oc.BgpTypes_BGP_CAPABILITY_EXTENDED_NEXTHOP_ENCODING) {
			problems = append(problems, fmt.Sprintf("supported-capabilities %v, want EXTENDED_NEXTHOP_ENCODING", caps))
		}
	}
	return problems
}

// verifyIPv4Prefixes waits until ok accepts the IPv4 prefix counters of the
// DUT neighbor of the ATE port p, which want describes, and reports an error
// otherwise.
func verifyIPv4Prefixes(t *testing.T, dut *ondatra.DUTDevice, p *atePeer, want string, ok func(*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Prefixes) bool) {
	t.Helper()
	q := bgpPath(dut).Neighbor(p.neighbor()).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes().State()
	got := "no prefix counters"
	_, done := gnmi.Watch(t, dut, q, routeTimeout, func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_Prefixes]) bool {
		c, present := v.Val()
		if !present {
			return false
		}
		got = fmt.Sprintf("received %d, installed %d, sent %d", c.GetReceived(), c.GetInstalled(), c.GetSent())
		return ok(c)
	}).Await(t)
	if !done {
		t.Errorf("DUT neighbor %s (%v) IPv4 prefixes: got %s, want %s", p.neighbor(), p, got, want)
	}
}

// verifyLocRIB waits until the DUT Loc-RIB has the IPv4 prefix of r with the
// next-hop of r, the global IPv6 address of its ATE port. With single, every
// Loc-RIB route of the prefix must have this next-hop.
func verifyLocRIB(t *testing.T, dut *ondatra.DUTDevice, r *advertisedRoute, single bool) {
	t.Helper()
	if deviations.BGPRibOcPathUnsupported(dut) {
		t.Logf("Skipping BGP Loc-RIB OC path verification for %s because BGPRibOcPathUnsupported is set.", r.prefix)
		return
	}
	rib := bgpPath(dut).Rib()
	b := gnmi.OCBatch().AddPaths(rib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().LocRib(), rib.AttrSetAny())
	var got []string
	_, ok := gnmi.Watch(t, dut, b.State(), routeTimeout, func(v *ygnmi.Value[*oc.Root]) bool {
		root, _ := v.Val()
		got = locRIBNextHops(bgpState(root, dut).GetRib(), r.prefix)
		isWant := func(nh string) bool { return sameAddr(nh, r.peer.ate.IPv6) }
		if single {
			return len(got) > 0 && !slices.ContainsFunc(got, func(nh string) bool { return !isWant(nh) })
		}
		return slices.ContainsFunc(got, isWant)
	}).Await(t)
	if !ok {
		t.Errorf("DUT Loc-RIB %s: next-hops %v, want %s", r.prefix, got, r.peer.ate.IPv6)
	}
}

// locRIBNextHops returns the next-hop of each IPv4 Loc-RIB route of the prefix
// in rib.
func locRIBNextHops(rib *oc.NetworkInstance_Protocol_Bgp_Rib, prefix string) []string {
	locRIB := rib.GetAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetIpv4Unicast().GetLocRib()
	if locRIB == nil {
		return nil
	}
	var nhs []string
	for _, route := range locRIB.Route {
		if samePrefix(route.GetPrefix(), prefix) {
			nhs = append(nhs, rib.GetAttrSet(route.GetAttrIndex()).GetNextHop())
		}
	}
	slices.Sort(nhs)
	return nhs
}

// nextHop is a next-hop of a DUT AFT entry.
type nextHop struct {
	ip     string
	intf   string
	weight uint64
}

func (n nextHop) String() string {
	return fmt.Sprintf("%s via %s (weight %d)", n.ip, n.intf, n.weight)
}

// verifyAFTNextHop waits until the DUT AFT entry of the prefix of r has the
// next-hop of r, the global IPv6 address of its ATE port, via the DUT port
// that connects to that ATE port (unless SkipInterfaceNameCheck is set). With
// single, it must be the only next-hop.
func verifyAFTNextHop(t *testing.T, env *testEnv, r *advertisedRoute, single bool) {
	t.Helper()
	awaitAFT(t, env.dut, []string{r.prefix}, func(afts *oc.NetworkInstance_Afts) error {
		return checkNextHop(env.dut, afts, r.prefix, r.peer.ate.IPv6, env.dutPort[r.peer], single)
	})
}

// checkNextHop returns an error unless the AFT entry of the prefix has the
// next-hop ip (or a link-local IPv6 next-hop when LinkLocalInsteadOfNh is set)
// via the interface intf (unless SkipInterfaceNameCheck is set), and with
// single, no other next-hop.
func checkNextHop(dut *ondatra.DUTDevice, afts *oc.NetworkInstance_Afts, prefix, ip, intf string, single bool) error {
	nhs, ok := entryNextHops(afts, prefix)
	if !ok {
		return fmt.Errorf("no entry for %s", prefix)
	}
	skipIntf := deviations.SkipInterfaceNameCheck(dut)
	linkLocal := deviations.LinkLocalInsteadOfNh(dut)
	isWant := func(nh nextHop) bool {
		ipOK := sameAddr(nh.ip, ip) || (linkLocal && isLinkLocalIPv6(nh.ip))
		return ipOK && (skipIntf || nh.intf == intf)
	}
	wantDesc := ip + " via " + intf
	if skipIntf {
		wantDesc = ip
	}
	switch {
	case single && (len(nhs) != 1 || !isWant(nhs[0])):
		return fmt.Errorf("%s has next-hops %v, want only %s", prefix, nhs, wantDesc)
	case !slices.ContainsFunc(nhs, isWant):
		return fmt.Errorf("%s has next-hops %v, want %s", prefix, nhs, wantDesc)
	}
	return nil
}

// awaitAFT waits until check accepts the DUT AFT, read with aftQuery, and
// reports the last error of check otherwise.
func awaitAFT(t *testing.T, dut *ondatra.DUTDevice, prefixes []string, check func(*oc.NetworkInstance_Afts) error) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	err := errors.New("no AFT state received")
	_, ok := gnmi.Watch(t, dut, aftQuery(dut, prefixes...), routeTimeout, func(v *ygnmi.Value[*oc.Root]) bool {
		root, _ := v.Val()
		err = check(root.GetNetworkInstance(ni).GetAfts())
		return err == nil
	}).Await(t)
	if !ok {
		t.Errorf("DUT AFT: %v", err)
	}
}

// aftQuery returns a batch query for the DUT AFT entries of the prefixes and
// all next-hop groups and next-hops.
func aftQuery(dut *ondatra.DUTDevice, prefixes ...string) ygnmi.SingletonQuery[*oc.Root] {
	afts := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts()
	b := gnmi.OCBatch()
	for _, prefix := range prefixes {
		if isIPv6Prefix(prefix) {
			b.AddPaths(afts.Ipv6Entry(prefix))
		} else {
			b.AddPaths(afts.Ipv4Entry(prefix))
		}
	}
	b.AddPaths(afts.NextHopGroupAny(), afts.NextHopAny())
	return b.State()
}

// port1RouteInAFT describes the IPv4 route from ATE port-1 in afts, which
// RT-1.112.7 does not allow while extended next-hop encoding is disabled for
// it: an entry for the RT-1.112.1 prefix, or an ATE port-1 next-hop (global
// IPv6 or over DUT port-1) for the RT-1.112.5 prefix. It returns "" if afts
// has no such route.
func port1RouteInAFT(env *testEnv, afts *oc.NetworkInstance_Afts) string {
	if _, ok := entryNextHops(afts, port1IPv4Route.prefix); ok {
		return fmt.Sprintf("%s is in the AFT", port1IPv4Route.prefix)
	}
	skipIntf := deviations.SkipInterfaceNameCheck(env.dut)
	nhs, _ := entryNextHops(afts, port1ECMPRoute.prefix)
	if slices.ContainsFunc(nhs, func(nh nextHop) bool {
		return sameAddr(nh.ip, atePort1.ate.IPv6) || (!skipIntf && nh.intf == env.dutPort[atePort1])
	}) {
		return fmt.Sprintf("%s has next-hops %v", port1ECMPRoute.prefix, nhs)
	}
	return ""
}

// entryNextHops returns the next-hops of the AFT entry of the prefix, sorted
// by IP address, and whether the entry exists.
func entryNextHops(afts *oc.NetworkInstance_Afts, prefix string) ([]nextHop, bool) {
	var nhgID uint64
	if isIPv6Prefix(prefix) {
		entry := afts.GetIpv6Entry(prefix)
		if entry == nil {
			return nil, false
		}
		nhgID = entry.GetNextHopGroup()
	} else {
		entry := afts.GetIpv4Entry(prefix)
		if entry == nil {
			return nil, false
		}
		nhgID = entry.GetNextHopGroup()
	}
	nhg := afts.GetNextHopGroup(nhgID)
	if nhg == nil {
		return nil, true
	}
	var nhs []nextHop
	for index, member := range nhg.NextHop {
		nh := afts.GetNextHop(index)
		nhs = append(nhs, nextHop{ip: nh.GetIpAddress(), intf: nh.GetInterfaceRef().GetInterface(), weight: member.GetWeight()})
	}
	slices.SortFunc(nhs, func(a, b nextHop) int { return strings.Compare(a.ip, b.ip) })
	return nhs, true
}

// verifyATERoute waits until the ATE port p has learned the IPv4 prefix with
// the next-hop want. An IPv4 want is compared with the IPv4 next-hop of the
// route, an IPv6 want with its IPv6 next-hop. The OTG reports one IPv6
// next-hop per route, so an additional link-local next-hop is not checked.
func verifyATERoute(t *testing.T, env *testEnv, p *atePeer, prefix, want string) {
	t.Helper()
	wantIPv4 := !isIPv6Addr(want)
	got := "prefix not received"
	q := gnmi.OTG().BgpPeer(p.bgpPeerName()).UnicastIpv4PrefixAny().State()
	_, ok := gnmi.WatchAll(t, env.ate.OTG(), q, routeTimeout, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
		r, present := v.Val()
		if !present || !samePrefix(fmt.Sprintf("%s/%d", r.GetAddress(), r.GetPrefixLength()), prefix) {
			return false
		}
		got = fmt.Sprintf("next-hop IPv4 %q, IPv6 %q", r.GetNextHopIpv4Address(), r.GetNextHopIpv6Address())
		if wantIPv4 {
			return sameAddr(r.GetNextHopIpv4Address(), want) && !hasIPv6NextHop(r)
		}
		return sameAddr(r.GetNextHopIpv6Address(), want) && !hasIPv4NextHop(r)
	}).Await(t)
	if !ok {
		t.Errorf("%v %s: got %s, want next-hop %s", p, prefix, got, want)
	}
}

// hasIPv4NextHop reports whether the route learned by the ATE has a
// non-unspecified IPv4 next-hop. Physical Ixia (KENG) reports "0.0.0.0" when
// an IPv4 route carries only an IPv6 next-hop.
func hasIPv4NextHop(r *otgtelemetry.BgpPeer_UnicastIpv4Prefix) bool {
	addr, err := netip.ParseAddr(r.GetNextHopIpv4Address())
	return err == nil && addr.Is4() && !addr.IsUnspecified()
}

// hasIPv6NextHop reports whether the route learned by the ATE has an IPv6
// next-hop, including an IPv4-mapped one.
func hasIPv6NextHop(r *otgtelemetry.BgpPeer_UnicastIpv4Prefix) bool {
	addr, err := netip.ParseAddr(r.GetNextHopIpv6Address())
	return err == nil && addr.Is6() && !addr.IsUnspecified()
}

// sendTraffic sends the flows from ATE port-4 at the same time and verifies
// that each loses less than maxLossPct of its packets. It returns the frames
// that the ATE ports of rxPeers received meanwhile, and the packets that the
// flows sent in total.
func sendTraffic(t *testing.T, env *testEnv, flows []*trafficFlow, rxPeers ...*atePeer) (map[*atePeer]uint64, uint64) {
	t.Helper()
	otg := env.ate.OTG()
	var ports []string
	for _, p := range rxPeers {
		ports = append(ports, env.atePort[p])
	}
	before := inFrames(t, env.ate)
	for _, port := range ports {
		// A missing counter would read as 0 and inflate the increase.
		if _, ok := before[port]; !ok {
			t.Fatalf("ATE port %s is missing from the OTG port counters", port)
		}
	}
	beforeFlows := flowCounters(t, env.ate, flows)
	// Start OutPkts stream watches before starting traffic so any counter reset
	// on start (for example 30000 -> 0 -> 30000 while trafficBaseline waits)
	// is observed continuously from t=0 without spawning test goroutines.
	outWatches := make(map[string]*flowOutWatch, len(flows))
	var names []string
	for _, f := range flows {
		w := startFlowOutWatch(t, env.ate, f.name, beforeFlows[f.name])
		defer w.watcher.Cancel()
		outWatches[f.name] = w
		names = append(names, f.name)
	}
	cs := gosnappi.NewControlState()
	cs.Traffic().FlowTransmit().SetFlowNames(names).SetState(gosnappi.StateTrafficFlowTransmitState.START)
	otg.SetControlState(t, cs)
	stopped := false
	defer func() {
		if !stopped {
			if msg := testt.CaptureFatal(t, func(tb testing.TB) {
				stopFlows(tb, env.ate, names)
			}); msg != nil {
				t.Logf("Cleanup stopFlows(%v): %s", names, *msg)
			}
		}
	}()
	baseline := trafficBaseline(t, env.ate, ports, before)

	var tx, rx uint64
	logCfg := gosnappi.NewConfig()
	for _, f := range flows {
		flowTx, flowRx := flowStats(t, env.ate, f.name, outWatches[f.name])
		verifyLoss(t, f.name, flowTx, flowRx)
		tx += flowTx
		rx += flowRx
		logCfg.Flows().Add().SetName(f.name)
	}
	otgutils.LogFlowMetrics(t, otg, logCfg)

	// The port counters are read once the frames that the flows delivered
	// show up in them.
	increase := awaitInFramesIncrease(t, env.ate, ports, baseline, rx)
	stopFlows(t, env.ate, names)
	stopped = true
	received := make(map[*atePeer]uint64)
	for _, p := range rxPeers {
		received[p] = increase[env.atePort[p]]
	}
	return received, tx
}

// stopFlows sets FlowTransmit state to STOP for names and waits for each flow
// to report Transmit == false, so physical Ixia (KENG) accepts a subsequent
// START on the same flows without re-pushing the OTG configuration.
func stopFlows(t testing.TB, ate *ondatra.ATEDevice, names []string) {
	t.Helper()
	otg := ate.OTG()
	stopCS := gosnappi.NewControlState()
	stopCS.Traffic().FlowTransmit().SetFlowNames(names).SetState(gosnappi.StateTrafficFlowTransmitState.STOP)
	otg.SetControlState(t, stopCS)
	for _, name := range names {
		if _, ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(name).Transmit().State(), counterTimeout, func(v *ygnmi.Value[bool]) bool {
			tx, present := v.Val()
			return present && !tx
		}).Await(t); !ok {
			t.Logf("Flow %s transmit state did not become false within %v after STOP", name, counterTimeout)
		}
	}
}

// flowCounter holds the packet counters of an OTG flow before a traffic run.
type flowCounter struct {
	outPkts uint64
	inPkts  uint64
}

// flowOutWatch holds a pre-started OutPkts stream watcher and its baseline.
type flowOutWatch struct {
	before     flowCounter
	outCleared bool
	watcher    *gnmi.Watcher[uint64]
}

// flowCounters returns the current packet counters of each flow, or zero
// counters for a flow whose state has not been reported yet.
func flowCounters(t *testing.T, ate *ondatra.ATEDevice, flows []*trafficFlow) map[string]flowCounter {
	t.Helper()
	otg := ate.OTG()
	counters := make(map[string]flowCounter, len(flows))
	for _, f := range flows {
		if c, ok := gnmi.Lookup(t, otg, gnmi.OTG().Flow(f.name).Counters().State()).Val(); ok {
			counters[f.name] = flowCounter{outPkts: c.GetOutPkts(), inPkts: c.GetInPkts()}
		}
	}
	return counters
}

// startFlowOutWatch starts an asynchronous STREAM watch on OutPkts of flow
// before traffic is started.
func startFlowOutWatch(t *testing.T, ate *ondatra.ATEDevice, flow string, before flowCounter) *flowOutWatch {
	t.Helper()
	w := &flowOutWatch{before: before}
	w.watcher = gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(flow).Counters().OutPkts().State(), 2*flowTimeout, func(v *ygnmi.Value[uint64]) bool {
		count, present := v.Val()
		if !present {
			return false
		}
		if before.outPkts > 0 && count < before.outPkts {
			w.outCleared = true
		}
		baseOut := before.outPkts
		if w.outCleared {
			baseOut = 0
		}
		return count >= baseOut && count-baseOut >= trafficPackets
	})
	return w
}

// flowStats awaits the pre-started OutPkts watch until the flow has sent
// trafficPackets packets during this run, waits for transmit to stop, and
// returns the packets sent and received during this run, whether the ATE
// clears flow counters on start or accumulates them.
func flowStats(t *testing.T, ate *ondatra.ATEDevice, flow string, outWatch *flowOutWatch) (tx, rx uint64) {
	t.Helper()
	otg := ate.OTG()
	f := gnmi.OTG().Flow(flow)
	before := outWatch.before
	sent, ok := outWatch.watcher.Await(t)
	outCleared := outWatch.outCleared
	if sent != nil {
		if rawTx, present := sent.Val(); present {
			if rawTx < before.outPkts {
				outCleared = true
			}
			baseOut := before.outPkts
			if outCleared {
				baseOut = 0
			}
			if rawTx >= baseOut {
				tx = rawTx - baseOut
			}
		}
	}
	if !ok {
		t.Errorf("Flow %s sent %d packets within %v, want %d", flow, tx, flowTimeout, trafficPackets)
	}
	// The flow is watched until it stops only after it has sent its packets,
	// so that a transmit state from before the start isn't taken for a stop.
	if _, ok := gnmi.Watch(t, otg, f.Transmit().State(), counterTimeout, func(v *ygnmi.Value[bool]) bool {
		transmit, present := v.Val()
		return present && !transmit
	}).Await(t); !ok {
		t.Errorf("Flow %s did not stop within %v", flow, counterTimeout)
	}
	inCleared := outCleared
	received, _ := gnmi.Watch(t, otg, f.Counters().InPkts().State(), rxSettleTimeout, func(v *ygnmi.Value[uint64]) bool {
		count, present := v.Val()
		if !present {
			return false
		}
		if before.inPkts > 0 && count < before.inPkts {
			inCleared = true
		}
		baseIn := before.inPkts
		if inCleared {
			baseIn = 0
		}
		return count >= baseIn && count-baseIn >= tx
	}).Await(t)
	if received != nil {
		if rawRx, present := received.Val(); present {
			baseIn := before.inPkts
			if inCleared || rawRx < baseIn {
				baseIn = 0
			}
			rx = rawRx - baseIn
		}
	}
	return tx, rx
}

// verifyLoss verifies that the flow lost less than maxLossPct of its packets
// and did not receive more packets than it transmitted.
func verifyLoss(t *testing.T, flow string, tx, rx uint64) {
	t.Helper()
	if tx == 0 {
		t.Errorf("Flow %s sent no packets", flow)
		return
	}
	if rx > tx {
		t.Errorf("Flow %q received more packets than transmitted: tx=%d rx=%d", flow, tx, rx)
		return
	}
	lossPct := 100 * (float64(tx) - float64(rx)) / float64(tx)
	if lossPct >= maxLossPct {
		t.Errorf("Flow %s loss %.2f%% (sent %d, received %d), want less than %.0f%%", flow, lossPct, tx, rx, maxLossPct)
		return
	}
	t.Logf("Flow %s loss %.2f%% (sent %d, received %d)", flow, lossPct, tx, rx)
}

// verifyReceived verifies that the ATE port p received at least minRxPct of
// the tx packets that the flows sent.
func verifyReceived(t *testing.T, p *atePeer, rx map[*atePeer]uint64, tx uint64) {
	t.Helper()
	if tx == 0 {
		t.Errorf("No packets were sent toward %v", p)
		return
	}
	got := 100 * float64(rx[p]) / float64(tx)
	if got < minRxPct {
		t.Errorf("%v received %d frames, %.2f%% of the %d packets sent, want at least %.0f%%", p, rx[p], got, tx, minRxPct)
		return
	}
	t.Logf("%v received %d frames, %.2f%% of the %d packets sent", p, rx[p], got, tx)
}

// inFramesQuery returns the wildcard query of the in-frames counters of all
// ATE ports, which reads them together in one request.
func inFramesQuery() ygnmi.WildcardQuery[uint64] {
	return gnmi.OTG().PortAny().Counters().InFrames().State()
}

// counterPort returns the ATE port in the path of an in-frames counter value,
// /ports/port[name=<port>]/state/counters/in-frames.
func counterPort(v *ygnmi.Value[uint64]) string {
	for _, e := range v.Path.GetElem() {
		if e.GetName() == "port" {
			return e.GetKey()["name"]
		}
	}
	return ""
}

// inFrames returns the in-frames counter of every ATE port.
func inFrames(t *testing.T, ate *ondatra.ATEDevice) map[string]uint64 {
	t.Helper()
	counters := make(map[string]uint64)
	for _, v := range gnmi.LookupAll(t, ate.OTG(), inFramesQuery()) {
		if count, present := v.Val(); present {
			counters[counterPort(v)] = count
		}
	}
	return counters
}

// trafficBaseline returns the in-frames counters of the ATE ports from which
// the frames of the flows that were just started are counted. An ATE/OTG may
// either keep accumulating port and flow counters across starts or clear them
// when traffic starts, and a read right after the start may still return the
// counters from before the clearing. Each port in ports is tracked
// independently until all ports have been observed and either every port with
// a non-zero pre-run counter has dropped below before or the total increase
// over each port's effective baseline reaches trafficPPS frames.
func trafficBaseline(t *testing.T, ate *ondatra.ATEDevice, ports []string, before map[string]uint64) map[string]uint64 {
	t.Helper()
	if len(ports) == 0 {
		return make(map[string]uint64)
	}
	now := make(map[string]uint64, len(ports))
	seen := make(map[string]bool, len(ports))
	cleared := make(map[string]bool, len(ports))
	_, ok := gnmi.WatchAll(t, ate.OTG(), inFramesQuery(), flowTimeout, func(v *ygnmi.Value[uint64]) bool {
		count, present := v.Val()
		port := counterPort(v)
		if !present || !slices.Contains(ports, port) {
			return false
		}
		seen[port] = true
		if count < before[port] {
			cleared[port] = true
		}
		now[port] = count
		if len(seen) < len(ports) {
			return false
		}
		allNonZeroCleared := true
		hasNonZeroBefore := false
		var total uint64
		for _, p := range ports {
			if before[p] > 0 {
				hasNonZeroBefore = true
				if !cleared[p] {
					allNonZeroCleared = false
				}
			}
			base := before[p]
			if cleared[p] {
				base = 0
			}
			if now[p] > base {
				total += now[p] - base
			}
		}
		return (hasNonZeroBefore && allNonZeroCleared) || total >= trafficPPS
	}).Await(t)
	baseline := make(map[string]uint64, len(ports))
	var clearedPorts []string
	for _, p := range ports {
		if cleared[p] {
			clearedPorts = append(clearedPorts, p)
			continue
		}
		baseline[p] = before[p]
	}
	if len(clearedPorts) > 0 {
		t.Logf("ATE ports %v cleared their port counters when traffic started, so their received frames are counted from 0.", clearedPorts)
	} else if !ok {
		t.Errorf("ATE ports %v: in-frames counters did not grow by %d within %v after the traffic started", ports, trafficPPS, flowTimeout)
	}
	return baseline
}

// awaitInFramesIncrease waits up to rxSettleTimeout for the in-frames counters
// of the ATE ports to settle at want in total since baseline, and returns the
// increase of each port. It waits for a value of every port so a port isn't
// counted as unchanged before its counter is read, and accepts a settled total
// of at least minRxPct of want to tolerate minor port/flow counter skew.
func awaitInFramesIncrease(t *testing.T, ate *ondatra.ATEDevice, ports []string, baseline map[string]uint64, want uint64) map[string]uint64 {
	t.Helper()
	now := make(map[string]uint64)
	increase := make(map[string]uint64)
	var total uint64
	allRead := false
	_, ok := gnmi.WatchAll(t, ate.OTG(), inFramesQuery(), rxSettleTimeout, func(v *ygnmi.Value[uint64]) bool {
		count, present := v.Val()
		if !present {
			return false
		}
		now[counterPort(v)] = count
		total = 0
		allRead = false
		for _, port := range ports {
			current, read := now[port]
			if !read {
				return false
			}
			increase[port] = 0
			if current > baseline[port] {
				increase[port] = current - baseline[port]
			}
			total += increase[port]
		}
		allRead = true
		return total >= want
	}).Await(t)
	if ok {
		return increase
	}
	var minWant uint64
	if want > 0 {
		minWant = uint64(math.Ceil(float64(want) * minRxPct / 100))
	}
	if !allRead || total < minWant {
		t.Errorf("ATE ports %v: in-frames increase %v (%d total), want at least %d in total", ports, increase, total, minWant)
		return increase
	}
	t.Logf("ATE ports %v: in-frames increase %v (%d total, flow received %d)", ports, increase, total, want)
	return increase
}

// portsToDetach returns the DUT ports whose subinterface 0 the test attaches
// to the default network instance, so that the Cleanup detaches them again:
// on devices with the ExplicitInterfaceInDefaultVRF deviation, the ports that
// are not attached before the test. An attachment of the DUT baseline is
// kept. The attachment ID is "<port>.0", as
// cfgplugins.AssignInterfacesToDefaultNetworkInstance creates it.
func portsToDetach(t *testing.T, dut *ondatra.DUTDevice, ports []string) []string {
	t.Helper()
	if !deviations.ExplicitInterfaceInDefaultVRF(dut) {
		return nil
	}
	ni := deviations.DefaultNetworkInstance(dut)
	b := gnmi.OCBatch()
	for _, port := range ports {
		b.AddPaths(gnmi.OC().NetworkInstance(ni).Interface(port + ".0"))
	}
	root, _ := gnmi.Lookup(t, dut, b.Config()).Val()
	var detach []string
	for _, port := range ports {
		if root.GetNetworkInstance(ni).GetInterface(port+".0") == nil {
			detach = append(detach, port)
		}
	}
	return detach
}

// removeDUTConfig is the DUT part of the README Cleanup. It removes the test
// configuration in isolated steps: (1) any multipath CLI pushed during the
// test is negated, (2) BGP, the accept-all policy, and the BGP tables on
// devices that need their explicit deletion are deleted, (3) the default
// network instance attachments added by the test are detached, and (4) the
// DUT ports get their pre-test configuration back. A failed step is reported
// and the next one still runs, so that the ports are always restored.
func removeDUTConfig(t *testing.T, env *testEnv, snapshot cfgplugins.InterfaceConfigSnapshot, detach []string) {
	t.Helper()
	dut := env.dut
	for p := range env.juniperCLIBGPPeers {
		cliForCleanup(t, dut, fmt.Sprintf(juniperDeleteNeighborTmpl, p.peerGroup(), p.neighbor()), "delete juniper CLI BGP neighbor")
		delete(env.juniperCLIBGPPeers, p)
		delete(env.juniperLocalIPv4Peers, p)
	}
	for i := len(env.multipathCLICleanup) - 1; i >= 0; i-- {
		cliForCleanup(t, dut, env.multipathCLICleanup[i], "negate multipath CLI")
	}

	t.Log("Cleanup: delete the DUT BGP configuration and the accept-all policy.")
	ni := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	b := &gnmi.SetBatch{}
	gnmi.BatchDelete(b, ni.Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Config())
	if deviations.NetworkInstanceTableDeletionRequired(dut) {
		for _, af := range []oc.E_Types_ADDRESS_FAMILY{oc.Types_ADDRESS_FAMILY_IPV4, oc.Types_ADDRESS_FAMILY_IPV6} {
			gnmi.BatchDelete(b, ni.Table(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, af).Config())
		}
	}
	setForCleanup(t, dut, b, "delete the BGP configuration")

	b = &gnmi.SetBatch{}
	gnmi.BatchDelete(b, gnmi.OC().RoutingPolicy().PolicyDefinition(acceptAllPolicy).Config())
	setForCleanup(t, dut, b, "delete the accept-all policy")

	if len(detach) > 0 {
		t.Log("Cleanup: detach the DUT ports that the test attached to the default network instance.")
		b = &gnmi.SetBatch{}
		cfgplugins.AssignInterfacesToDefaultNetworkInstance(t, dut, b, cfgplugins.DefaultNetworkInstanceInterfacesConfig{Interfaces: detach, Delete: true})
		setForCleanup(t, dut, b, "detach the ports from the default network instance")
	}

	for i := len(env.intfCLICleanup) - 1; i >= 0; i-- {
		cliForCleanup(t, dut, env.intfCLICleanup[i], "negate interface CLI")
	}

	t.Log("Cleanup: restore the pre-test configuration of the DUT ports.")
	b = &gnmi.SetBatch{}
	cfgplugins.RestoreInterfacesConfig(t, dut, b, snapshot)
	setForCleanup(t, dut, b, "restore the DUT ports")
}

// setForCleanup applies b and reports a failure with t.Errorf instead of
// t.Fatalf, so that the remaining cleanup steps still run.
func setForCleanup(t *testing.T, dut *ondatra.DUTDevice, b *gnmi.SetBatch, step string) {
	t.Helper()
	if msg := testt.CaptureFatal(t, func(t testing.TB) { b.Set(t, dut) }); msg != nil {
		t.Errorf("Cleanup: %s failed: %s", step, *msg)
	}
}

// cliForCleanup applies cli and reports a failure with t.Errorf instead of
// t.Fatalf, so that the remaining cleanup steps still run.
func cliForCleanup(t *testing.T, dut *ondatra.DUTDevice, cli, step string) {
	t.Helper()
	if msg := testt.CaptureFatal(t, func(t testing.TB) { helpers.GnmiCLIConfig(t, dut, cli) }); msg != nil {
		t.Errorf("Cleanup: %s failed: %s", step, *msg)
	}
}

// samePrefix reports whether two CIDR strings denote the same prefix,
// independent of how the address is written.
func samePrefix(a, b string) bool {
	pa, errA := netip.ParsePrefix(a)
	pb, errB := netip.ParsePrefix(b)
	return errA == nil && errB == nil && pa.Masked() == pb.Masked()
}

// sameAddr reports whether two strings denote the same IP address,
// independent of how it is written.
func sameAddr(a, b string) bool {
	pa, errA := netip.ParseAddr(a)
	pb, errB := netip.ParseAddr(b)
	return errA == nil && errB == nil && pa == pb
}

// isIPv6Prefix reports whether the CIDR string is an IPv6 prefix.
func isIPv6Prefix(prefix string) bool {
	p, err := netip.ParsePrefix(prefix)
	return err == nil && p.Addr().Is6()
}

// isIPv6Addr reports whether the string is an IPv6 address.
func isIPv6Addr(addr string) bool {
	a, err := netip.ParseAddr(addr)
	return err == nil && a.Is6()
}

// isLinkLocalIPv6 reports whether s is a link-local unicast IPv6 address.
func isLinkLocalIPv6(s string) bool {
	addr, err := netip.ParseAddr(s)
	return err == nil && addr.Is6() && addr.IsLinkLocalUnicast()
}

// mustParsePrefix parses the CIDR string, and fails the test if it is not a
// prefix.
func mustParsePrefix(t *testing.T, prefix string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		t.Fatalf("netip.ParsePrefix(%q) failed: %v", prefix, err)
	}
	return p
}
