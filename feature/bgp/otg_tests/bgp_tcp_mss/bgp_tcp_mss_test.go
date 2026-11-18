// Copyright 2026 Google LLC
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

// Package bgp_tcp_mss_test implements RT-1.22: BGP TCP MSS. The procedure is
// described in README.md in this directory.
package bgp_tcp_mss_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
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
)

// TestMain is the entry point for the go test runner. Environment setup needs
// a *testing.T and a reserved testbed, so it is performed by setupEnvironment
// at the start of TestBGPTCPMSS.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// AS numbers, names, and addresses from the README "DUT and OTG properties"
// table and Initial Setup.
const (
	dutAS  = 64500
	ebgpAS = 64501 // AS of the ATE eBGP peers on port-1.

	ebgpPeerGroup = "EBGP-PEERS"
	ibgpPeerGroup = "IBGP-PEERS"

	bgpInstance = "BGP"   // BGP protocol name from the README Canonical OC.
	allowPolicy = "ALLOW" // Routing policy that accepts all routes.
	dutRouterID = "192.0.2.1"

	plenIPv4 = 30
	plenIPv6 = 126
)

// MTU and TCP MSS values of RT-1.22.1.
const (
	// jumboMTU is the MTU of the DUT and ATE ports in the neighbor-level and
	// peer-group-level steps. It fits TCP segments of mssNeighbor bytes over
	// IPv4 and IPv6.
	jumboMTU uint16 = 5040
	// defaultMTU is the Ethernet MTU: the MTU of the ATE ports outside the
	// jumboMTU steps, and the MTU assumed for a DUT port that reports none.
	defaultMTU = 1500
	// ethernetHeader is the size of the Ethernet header, which the
	// OpenConfig interface MTU includes.
	ethernetHeader = 14

	mssNeighbor  uint16 = 4096 // MSS of the neighbor-level step.
	mssPeerGroup uint16 = 4000 // MSS of the peer-group-level step.
	mssOverride  uint16 = 1400 // Neighbor MSS that overrides mssPeerGroup.

	// A TCP MSS that the DUT reports before an MSS is configured must be at
	// least the TCP default MSS of the address family, and at most the MTU
	// of the DUT port minus the IP and TCP headers.
	minDefaultMSSIPv4 = 536
	minDefaultMSSIPv6 = 1220
	headersIPv4       = 40 // IPv4 and TCP headers without options.
	headersIPv6       = 60 // IPv6 and TCP headers without options.
)

// routeCount is the number of prefixes per address family that the ATE eBGP
// peers advertise in the neighbor-level step. All have the same path
// attributes, so the UPDATE messages add up to several times mssNeighbor.
const routeCount = 2000

// bgpPort is the TCP port of BGP.
const bgpPort = layers.TCPPort(179)

// maxSnaplen is the largest snap length that the pcap reader accepts.
const maxSnaplen = 1 << 18

// Upper bounds for the gNMI watches.
const (
	stateTimeout   = time.Minute
	sessionTimeout = 3 * time.Minute
	routeTimeout   = 3 * time.Minute
	// holdTimeout covers the expiry of the 240-second OTG hold time, in case
	// a stopped ATE peer does not close its session.
	holdTimeout = 5 * time.Minute
	// monitorTimeout bounds the background watch of the session state while
	// the prefixes are advertised; it exceeds the prefix checks it covers.
	monitorTimeout = 20 * time.Minute
	// resetWindow is how long after the MSS change the test watches for a
	// session reset, as in the README.
	resetWindow = 30 * time.Second
)

// family is the address family of a BGP session and of its routes.
type family string

const (
	ipv4 family = "IPv4"
	ipv6 family = "IPv6"
)

// afiSafi returns the unicast AFI-SAFI of the address family.
func (f family) afiSafi() oc.E_BgpTypes_AFI_SAFI_TYPE {
	if f == ipv6 {
		return oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
	}
	return oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
}

// link is one DUT-ATE link of the README topology, with the IPv4 and IPv6 BGP
// peers on it.
type link struct {
	port      string // Testbed port ID, the same on the DUT and the ATE.
	peerGroup string // DUT peer group of the two neighbors on the link.
	peerAS    uint32 // AS of the ATE BGP peers.
	dutAttr   attrs.Attributes
	ateAttr   attrs.Attributes
}

// ibgp reports whether the BGP sessions on the link are iBGP sessions.
func (l *link) ibgp() bool {
	return l.peerAS == dutAS
}

var (
	// link1 carries the eBGP sessions, whose ATE peers advertise the routes.
	link1 = &link{
		port:      "port1",
		peerGroup: ebgpPeerGroup,
		peerAS:    ebgpAS,
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
	// link2 carries the iBGP sessions, whose ATE peers receive the routes
	// from the DUT.
	link2 = &link{
		port:      "port2",
		peerGroup: ibgpPeerGroup,
		peerAS:    dutAS,
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
	links = []*link{link1, link2}
)

// session is one of the four BGP sessions: the IPv4 or the IPv6 session on a
// link.
type session struct {
	link *link
	f    family
}

// sessions are the four BGP sessions of the README.
var sessions = []session{{link1, ipv4}, {link1, ipv6}, {link2, ipv4}, {link2, ipv6}}

// neighbor returns the ATE address of the session, which is the DUT neighbor
// address.
func (s session) neighbor() string {
	if s.f == ipv6 {
		return s.link.ateAttr.IPv6
	}
	return s.link.ateAttr.IPv4
}

// local returns the DUT address of the session.
func (s session) local() string {
	if s.f == ipv6 {
		return s.link.dutAttr.IPv6
	}
	return s.link.dutAttr.IPv4
}

// otgInterface returns the name of the OTG IP interface of the session, as
// created by attrs.AddToOTG.
func (s session) otgInterface() string {
	return s.link.ateAttr.Name + "." + string(s.f)
}

// otgPeer returns the name of the OTG BGP peer of the session.
func (s session) otgPeer() string {
	return s.link.ateAttr.Name + ".BGP." + string(s.f)
}

// String describes the session in logs, for example "eBGP IPv4 neighbor
// 192.0.2.2".
func (s session) String() string {
	kind := "eBGP"
	if s.link.ibgp() {
		kind = "iBGP"
	}
	return fmt.Sprintf("%s %s neighbor %s", kind, s.f, s.neighbor())
}

// neighbors returns the DUT neighbor addresses of the sessions.
func neighbors(ss []session) []string {
	var addrs []string
	for _, s := range ss {
		addrs = append(addrs, s.neighbor())
	}
	return addrs
}

// overrideSessions returns the sessions whose neighbors override the
// peer-group MSS in the README: the IPv4 neighbor of each peer group,
// 192.0.2.2 and 192.0.2.6.
func overrideSessions() []session {
	var ss []session
	for _, s := range sessions {
		if s.f == ipv4 {
			ss = append(ss, s)
		}
	}
	return ss
}

// routeRange is a README route range that an ATE eBGP peer on port-1
// advertises: routeCount prefixes from start onwards.
type routeRange struct {
	name  string // OTG route range name.
	f     family
	start netip.Prefix
}

var (
	v4Routes    = routeRange{name: "atePort1.routes.IPv4", f: ipv4, start: netip.MustParsePrefix("100.64.0.0/24")}
	v6Routes    = routeRange{name: "atePort1.routes.IPv6", f: ipv6, start: netip.MustParsePrefix("2001:db8:1::/64")}
	routeRanges = []routeRange{v4Routes, v6Routes}
)

// prefixes returns the routeCount prefixes of the route range. An OTG route
// range with the default step of 1 advertises consecutive prefixes of the
// same length.
func (r routeRange) prefixes() map[netip.Prefix]bool {
	all := make(map[netip.Prefix]bool, routeCount)
	bits := r.start.Addr().BitLen()
	addr := new(big.Int).SetBytes(r.start.Masked().Addr().AsSlice())
	step := new(big.Int).Lsh(big.NewInt(1), uint(bits-r.start.Bits()))
	for range routeCount {
		a, _ := netip.AddrFromSlice(addr.FillBytes(make([]byte, bits/8)))
		all[netip.PrefixFrom(a, r.start.Bits())] = true
		addr.Add(addr, step)
	}
	return all
}

// testEnv holds the devices and the OTG configuration that the steps of the
// test share.
type testEnv struct {
	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	otg *otg.OTG
	top gosnappi.Config
	// eths are the OTG Ethernet interfaces of ATE port-1 and port-2, whose
	// MTU the test changes.
	eths []gosnappi.DeviceEthernet
}

// TestBGPTCPMSS implements RT-1.22.1: TCP MSS at the neighbor and peer-group
// levels, on the environment of the README Initial Setup.
func TestBGPTCPMSS(t *testing.T) {
	env := setupEnvironment(t)

	t.Run("RT-1.22.1: TCP MSS at the neighbor and peer-group levels", func(t *testing.T) {
		t.Run("Default TCP MSS", func(t *testing.T) {
			if deviations.SkipTCPNegotiatedMSSCheck(env.dut) {
				t.Skip("The DUT does not report the TCP MSS in use before an MSS is configured (deviation skip_tcp_negotiated_mss_check)")
			}
			verifyDefaultTCPMSS(t, env)
		})

		// The neighbor-level and peer-group-level steps run at jumboMTU. The
		// last README step of RT-1.22.1 removes the MTU configuration again; it
		// is registered with t.Cleanup before the MTU is set, so that it also
		// runs when a later step fails.
		t.Cleanup(func() {
			t.Log("Remove the MTU configuration from the four ports so that they return to the default MTU")
			removeJumboMTU(t, env)
		})
		t.Logf("Set the MTU of DUT port-1, DUT port-2, ATE port-1 and ATE port-2 to %d bytes", jumboMTU)
		setJumboMTU(t, env)
		t.Log("Verify that all four sessions are ESTABLISHED")
		if !verifySessions(t, env.dut) {
			t.Fatalf("Not all four BGP sessions are ESTABLISHED at MTU %d; the neighbor-level and peer-group-level steps cannot run", jumboMTU)
		}

		t.Run("Neighbor level", func(t *testing.T) {
			testNeighborLevel(t, env)
		})
		t.Run("Peer-group level", func(t *testing.T) {
			testPeerGroupLevel(t, env)
		})
	})
}

// setupEnvironment applies the README Initial Setup and verifies that the four
// BGP sessions are ESTABLISHED. Each README Cleanup step is registered with
// t.Cleanup before the configuration that it reverts is pushed, so that it
// also runs when a push fails part-way.
func setupEnvironment(t *testing.T) *testEnv {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	env := &testEnv{dut: dut, ate: ate, otg: ate.OTG()}

	t.Log("Initial Setup: connect ATE port-1 to DUT port-1, and ATE port-2 to DUT port-2")
	for _, l := range links {
		t.Logf("DUT port %s is connected to ATE port %s", dut.Port(t, l.port).Name(), ate.Port(t, l.port).ID())
	}

	t.Log("Initial Setup: configure the DUT interfaces per the README table")
	configureDUTInterfaces(t, dut)

	t.Logf("Initial Setup: configure the routing policy %s that accepts all routes, and BGP AS %d with the peer groups %s and %s", allowPolicy, dutAS, ebgpPeerGroup, ibgpPeerGroup)
	configureDUTBGP(t, dut)

	t.Log("Initial Setup: configure the ATE interfaces and the matching BGP peers, and start the OTG protocols")
	env.top, env.eths = configureOTG(t, ate)
	t.Cleanup(func() {
		t.Log("Cleanup: stop the OTG protocols")
		env.otg.StopProtocols(t)
	})
	startOTG(t, env)

	t.Log("Initial Setup: verify that all four sessions are ESTABLISHED")
	if !verifySessions(t, dut) {
		t.Fatal("Not all four BGP sessions are ESTABLISHED after the Initial Setup")
	}
	return env
}

// configureDUTInterfaces configures the DUT ports from the README table and
// waits until their state matches the configuration. The README Cleanup
// restores the baseline state, so the pre-test interface configuration is read
// first and restored in Cleanup.
func configureDUTInterfaces(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	var names []string
	for _, l := range links {
		names = append(names, dut.Port(t, l.port).Name())
	}
	snapshot := cfgplugins.SnapshotInterfacesConfig(t, dut, names)
	restore := cfgplugins.RestoreInterfacesConfig(t, dut, &gnmi.SetBatch{}, snapshot)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, name := range names {
			// fptest.AssignToNetworkInstance uses the ID "<interface>.<subinterface>".
			niIntf := gnmi.OC().NetworkInstance(ni).Interface(name + ".0")
			if _, ok := gnmi.Lookup(t, dut, niIntf.Config()).Val(); !ok {
				gnmi.BatchDelete(restore, niIntf.Config())
			}
		}
	}
	t.Cleanup(func() {
		t.Log("Cleanup: restore the pre-test DUT interface configuration")
		restore.Set(t, dut)
	})
	batch := &gnmi.SetBatch{}
	for _, l := range links {
		p := dut.Port(t, l.port)
		intf := l.dutAttr.NewOCInterface(p.Name(), dut)
		if deviations.FrBreakoutFix(dut) && p.PMD() == ondatra.PMD100GBASEFR {
			// These devices bring up a 100GBASE-FR port only with a fixed
			// speed and duplex mode, and auto-negotiation disabled.
			e := intf.GetOrCreateEthernet()
			e.SetAutoNegotiate(false)
			e.SetDuplexMode(oc.Ethernet_DuplexMode_FULL)
			e.SetPortSpeed(oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB)
		}
		gnmi.BatchReplace(batch, gnmi.OC().Interface(p.Name()).Config(), intf)
	}
	batch.Set(t, dut)

	// The DEFAULT network instance always exists; this idempotent update only
	// sets its type, so it is not reverted.
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	for _, l := range links {
		p := dut.Port(t, l.port)
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, p)
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, p.Name(), ni, 0)
		}
	}
	for _, l := range links {
		awaitInterfaceState(t, dut, dut.Port(t, l.port).Name(), &l.dutAttr)
	}
}

// awaitInterfaceState waits until the DUT port is up and reports the
// configured IPv4 and IPv6 addresses.
func awaitInterfaceState(t *testing.T, dut *ondatra.DUTDevice, name string, a *attrs.Attributes) {
	t.Helper()
	intf := gnmi.OC().Interface(name)
	gnmi.Await(t, dut, intf.OperStatus().State(), stateTimeout, oc.Interface_OperStatus_UP)
	sub := intf.Subinterface(0)
	gnmi.Await(t, dut, sub.Ipv4().Address(a.IPv4).PrefixLength().State(), stateTimeout, a.IPv4Len)
	gnmi.Await(t, dut, sub.Ipv6().Address(a.IPv6).PrefixLength().State(), stateTimeout, a.IPv6Len)
}

// configureDUTBGP configures the accept-all routing policy and BGP with the
// two README peer groups and their neighbors. The peer groups carry the peer
// AS and the import and export policy; each neighbor only joins its peer group
// and enables the unicast AFI-SAFI of its session.
func configureDUTBGP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	if _, err := cfgplugins.ConfigureBGPRoutePolicy(t, batch, cfgplugins.BGPPolicyConfig{PolicyName: allowPolicy, StatementID: "accept-all"}); err != nil {
		t.Fatalf("ConfigureBGPRoutePolicy(%q) failed: %v", allowPolicy, err)
	}
	bgp := cfgplugins.ConfigureDUTBGP(t, dut, batch, cfgplugins.BGPConfig{DutAS: dutAS, RouterID: dutRouterID}).GetOrCreateBgp()
	for _, l := range links {
		opts := []cfgplugins.PeerGroupOption{
			cfgplugins.WithPeerAS(l.peerAS),
			cfgplugins.ApplyPGRoutingPolicy(allowPolicy, allowPolicy, false),
		}
		// Each peer group holds an IPv4 and an IPv6 neighbor, so it enables
		// both unicast AFI-SAFIs.
		for _, f := range []family{ipv4, ipv6} {
			opts = append(opts,
				cfgplugins.WithPGAfiSafiEnabled(f.afiSafi(), true, false),
				withPGAfiSafiPolicy(f.afiSafi(), allowPolicy),
			)
		}
		cfgplugins.ConfigurePeerGroup(bgp.GetOrCreatePeerGroup(l.peerGroup), dut, opts...)
	}
	for _, s := range sessions {
		// Without policy names, the AFI-SAFI option only enables the unicast
		// AFI-SAFI of the session and disables the other one, so the neighbor
		// uses the policy of its peer group.
		cfgplugins.ConfigurePeer(bgp.GetOrCreateNeighbor(s.neighbor()), dut,
			cfgplugins.WithPeerGroup(s.link.peerGroup, s.link.peerAS, "", "", false),
			cfgplugins.WithPeerAfiSafiEnabled(s.f == ipv4, "", "", false),
		)
	}
	// Cleanups run in reverse order: BGP is deleted before the policy it uses.
	t.Cleanup(func() {
		t.Logf("Cleanup: delete the DUT routing policy %s", allowPolicy)
		gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(allowPolicy).Config())
	})
	t.Cleanup(func() {
		t.Log("Cleanup: delete the DUT BGP configuration")
		cfgplugins.BGPClearConfig(t, dut)
	})
	batch.Set(t, dut)
}

// withPGAfiSafiPolicy applies the import and export policy to an AFI-SAFI of
// the peer group. Devices that do not support policies under the AFI-SAFI get
// them at the peer-group level from cfgplugins.ApplyPGRoutingPolicy instead.
func withPGAfiSafiPolicy(afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE, policy string) cfgplugins.PeerGroupOption {
	return func(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup, dut *ondatra.DUTDevice) {
		if deviations.RoutePolicyUnderAFIUnsupported(dut) {
			return
		}
		ap := pg.GetOrCreateAfiSafi(afiSafi).GetOrCreateApplyPolicy()
		ap.SetImportPolicy([]string{policy})
		ap.SetExportPolicy([]string{policy})
	}
}

// configureOTG builds the OTG configuration: on each ATE port, an IPv4 and an
// IPv6 BGP peer that match the DUT neighbors, the README route ranges on the
// eBGP peers of port-1, and a packet capture of the port.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) (gosnappi.Config, []gosnappi.DeviceEthernet) {
	t.Helper()
	top := gosnappi.NewConfig()
	var eths []gosnappi.DeviceEthernet
	for _, l := range links {
		ap := ate.Port(t, l.port)
		dev := l.ateAttr.AddToOTG(top, ap, &l.dutAttr)
		eths = append(eths, dev.Ethernets().Items()[0])
		peerType := otgconfighelpers.WithBGPEBGP()
		if l.ibgp() {
			peerType = otgconfighelpers.WithBGPIBGP()
		}
		v4, v6 := session{l, ipv4}, session{l, ipv6}
		peer4 := otgconfighelpers.AddBGPV4Peer(dev, v4.otgInterface(),
			otgconfighelpers.WithBGPName(v4.otgPeer()),
			otgconfighelpers.WithBGPPeerAddress(v4.local()),
			otgconfighelpers.WithBGPASNumber(l.peerAS),
			otgconfighelpers.WithBGPRouterID(l.ateAttr.IPv4),
			otgconfighelpers.WithoutBGPGR(),
			otgconfighelpers.WithBGPLearnedV4Pfx(true),
			peerType,
		)
		peer6 := otgconfighelpers.AddBGPV6Peer(dev, v6.otgInterface(),
			otgconfighelpers.WithBGPName(v6.otgPeer()),
			otgconfighelpers.WithBGPPeerAddress(v6.local()),
			otgconfighelpers.WithBGPASNumber(l.peerAS),
			otgconfighelpers.WithBGPRouterID(l.ateAttr.IPv4),
			otgconfighelpers.WithoutBGPGR(),
			otgconfighelpers.WithBGPLearnedV6Pfx(true),
			peerType,
		)
		if !l.ibgp() {
			// One route range per address family gives all prefixes the same
			// path attributes.
			otgconfighelpers.AddBGPV4Routes(peer4, v4Routes.name, []string{v4Routes.start.String()},
				otgconfighelpers.WithBGPRouteAddressCount(routeCount),
				otgconfighelpers.WithBGPRouteNextHopIPv4(l.ateAttr.IPv4),
			)
			otgconfighelpers.AddBGPV6Routes(peer6, v6Routes.name, []string{v6Routes.start.String()},
				otgconfighelpers.WithBGPRouteAddressCount(routeCount),
				otgconfighelpers.WithBGPRouteNextHopIPv6(l.ateAttr.IPv6),
			)
		}
		top.Captures().Add().SetName(ap.ID() + ".capture").SetPortNames([]string{ap.ID()}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}
	return top, eths
}

// startOTG pushes the OTG configuration, starts the protocols, and waits for
// IPv4 and IPv6 neighbor resolution. The route ranges are withdrawn right
// away, because the README advertises them only in the neighbor-level step.
func startOTG(t *testing.T, env *testEnv) {
	t.Helper()
	env.otg.PushConfig(t, env.top)
	env.otg.StartProtocols(t)
	controlRoutes(t, env.otg, gosnappi.StateProtocolRouteState.WITHDRAW)
	otgutils.WaitForARP(t, env.otg, env.top, "IPv4")
	otgutils.WaitForARP(t, env.otg, env.top, "IPv6")
}

// controlRoutes advertises or withdraws the README route ranges on the ATE.
func controlRoutes(t *testing.T, otg *otg.OTG, state gosnappi.StateProtocolRouteStateEnum) {
	t.Helper()
	var names []string
	for _, r := range routeRanges {
		names = append(names, r.name)
	}
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(names).SetState(state)
	otg.SetControlState(t, cs)
}

// setATEPeersState starts or stops the four ATE BGP peers.
func setATEPeersState(t *testing.T, otg *otg.OTG, state gosnappi.StateProtocolBgpPeersStateEnum) {
	t.Helper()
	var names []string
	for _, s := range sessions {
		names = append(names, s.otgPeer())
	}
	cs := gosnappi.NewControlState()
	cs.Protocol().Bgp().Peers().SetPeerNames(names).SetState(state)
	otg.SetControlState(t, cs)
}

// setJumboMTU sets the MTU of DUT port-1, DUT port-2, ATE port-1, and ATE
// port-2 to jumboMTU.
func setJumboMTU(t *testing.T, env *testEnv) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	for _, l := range links {
		cfgplugins.AddInterfaceMTUOps(batch, env.dut, env.dut.Port(t, l.port).Name(), jumboMTU, false)
	}
	batch.Set(t, env.dut)
	for _, l := range links {
		awaitDUTMTU(t, env.dut, env.dut.Port(t, l.port).Name(), fmt.Sprint(jumboMTU), func(mtu uint32, present bool) bool {
			return present && mtu == uint32(jumboMTU)
		})
	}

	// The OTG applies a new MTU only with a new configuration, which
	// restarts its protocols and with them the four BGP sessions.
	env.otg.StopProtocols(t)
	for _, eth := range env.eths {
		eth.SetMtu(uint32(jumboMTU))
	}
	startOTG(t, env)
}

// removeJumboMTU removes the MTU configuration from the DUT ports, and returns
// the ATE ports to the default MTU.
func removeJumboMTU(t *testing.T, env *testEnv) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	for _, l := range links {
		cfgplugins.AddInterfaceMTUOps(batch, env.dut, env.dut.Port(t, l.port).Name(), jumboMTU, true)
	}
	batch.Set(t, env.dut)
	for _, l := range links {
		awaitDUTMTU(t, env.dut, env.dut.Port(t, l.port).Name(), "the default MTU", func(mtu uint32, present bool) bool {
			return !present || mtu != uint32(jumboMTU)
		})
	}

	env.otg.StopProtocols(t)
	for _, eth := range env.eths {
		eth.SetMtu(defaultMTU)
	}
	env.otg.PushConfig(t, env.top)
}

// awaitDUTMTU waits until the MTU state of a DUT port, at the leaves that
// cfgplugins.AddInterfaceMTUOps configures, satisfies match. want describes
// the expected MTU in error messages.
func awaitDUTMTU(t *testing.T, dut *ondatra.DUTDevice, port, want string, match func(mtu uint32, present bool) bool) {
	t.Helper()
	intf := gnmi.OC().Interface(port)
	if deviations.OmitL2MTU(dut) {
		v4 := gnmi.Watch(t, dut, intf.Subinterface(0).Ipv4().Mtu().State(), stateTimeout, func(v *ygnmi.Value[uint16]) bool {
			mtu, ok := v.Val()
			return match(uint32(mtu), ok)
		})
		v6 := gnmi.Watch(t, dut, intf.Subinterface(0).Ipv6().Mtu().State(), stateTimeout, func(v *ygnmi.Value[uint32]) bool {
			mtu, ok := v.Val()
			return match(mtu, ok)
		})
		if got, ok := v4.Await(t); !ok {
			t.Errorf("DUT port %s: subinterface 0 IPv4 MTU state got %s, want %s", port, valueString(got), want)
		}
		if got, ok := v6.Await(t); !ok {
			t.Errorf("DUT port %s: subinterface 0 IPv6 MTU state got %s, want %s", port, valueString(got), want)
		}
		return
	}
	w := gnmi.Watch(t, dut, intf.Mtu().State(), stateTimeout, func(v *ygnmi.Value[uint16]) bool {
		mtu, ok := v.Val()
		return match(uint32(mtu), ok)
	})
	if got, ok := w.Await(t); !ok {
		t.Errorf("DUT port %s: MTU state got %s, want %s", port, valueString(got), want)
	}
}

// verifyDefaultTCPMSS implements the optional first step of RT-1.22.1. If the
// DUT reports transport/state/tcp-mss for a neighbor before an MSS is
// configured, the value is the MSS in use on the TCP session, which must be at
// least the TCP default MSS and at most the MTU of the DUT port minus the IP
// and TCP headers.
func verifyDefaultTCPMSS(t *testing.T, env *testEnv) {
	t.Helper()
	t.Logf("If the DUT reports transport/state/tcp-mss for the neighbors before an MSS is configured, verify that it is at least %d for IPv4 and %d for IPv6, and at most the DUT port MTU minus %d bytes for IPv4 or %d bytes for IPv6", minDefaultMSSIPv4, minDefaultMSSIPv6, headersIPv4, headersIPv6)
	for _, s := range sessions {
		mss, ok := gnmi.Lookup(t, env.dut, bgpPath(env.dut).Neighbor(s.neighbor()).Transport().TcpMss().State()).Val()
		if !ok {
			t.Logf("DUT %v: transport/state/tcp-mss is not reported before an MSS is configured, so it is not checked", s)
			continue
		}
		port := env.dut.Port(t, s.link.port).Name()
		mtu := dutPortMTU(t, env.dut, port, s.f)
		minMSS, maxMSS := minDefaultMSSIPv4, int(mtu)-headersIPv4
		if s.f == ipv6 {
			minMSS, maxMSS = minDefaultMSSIPv6, int(mtu)-headersIPv6
		}
		if int(mss) < minMSS || int(mss) > maxMSS {
			t.Errorf("DUT %v: transport/state/tcp-mss before an MSS is configured got %d, want between %d and %d (port %s MTU %d)", s, mss, minMSS, maxMSS, port, mtu)
			continue
		}
		t.Logf("DUT %v: transport/state/tcp-mss before an MSS is configured is %d, between %d and %d (port %s MTU %d)", s, mss, minMSS, maxMSS, port, mtu)
	}
}

// dutPortMTU returns the IP MTU of a DUT port for the address family: the MTU
// of subinterface 0 for the family if the DUT reports it, else the IP MTU
// derived from the interface MTU, else the Ethernet default.
func dutPortMTU(t *testing.T, dut *ondatra.DUTDevice, port string, f family) uint32 {
	t.Helper()
	intf := gnmi.OC().Interface(port)
	switch f {
	case ipv4:
		if mtu, ok := gnmi.Lookup(t, dut, intf.Subinterface(0).Ipv4().Mtu().State()).Val(); ok && mtu > 0 {
			return uint32(mtu)
		}
	case ipv6:
		if mtu, ok := gnmi.Lookup(t, dut, intf.Subinterface(0).Ipv6().Mtu().State()).Val(); ok && mtu > 0 {
			return mtu
		}
	}
	if mtu, ok := gnmi.Lookup(t, dut, intf.Mtu().State()).Val(); ok && mtu > 0 {
		// In OpenConfig the interface MTU includes the Ethernet header, for
		// example 1514 for the default IP MTU of 1500. An interface MTU of at
		// most 1500 is taken as an IP MTU.
		ipMTU := uint32(mtu)
		if ipMTU > defaultMTU {
			ipMTU = max(ipMTU-ethernetHeader, defaultMTU)
		}
		return ipMTU
	}
	return defaultMTU
}

// testNeighborLevel implements the neighbor-level steps of RT-1.22.1. The
// README ends them by withdrawing the prefixes and removing the neighbor MSS.
// These two steps are registered with t.Cleanup, so that the peer-group-level
// steps start without the neighbor MSS also when a step here fails.
func testNeighborLevel(t *testing.T, env *testEnv) {
	dut, otg := env.dut, env.otg
	allNeighbors := neighbors(sessions)

	t.Log("Record state/established-transitions for all four neighbors")
	before := establishedTransitions(t, dut)
	for _, s := range sessions {
		if n, ok := before[s.neighbor()]; ok {
			t.Logf("DUT %v: state/established-transitions is %d", s, n)
		} else {
			t.Logf("DUT %v: state/established-transitions is not reported", s)
		}
	}

	t.Logf("Configure transport/config/tcp-mss to %d on all four neighbors", mssNeighbor)
	t.Cleanup(func() {
		t.Log("Remove transport/config/tcp-mss from the four neighbors")
		removeTCPMSS(t, dut, allNeighbors, nil)
	})
	setTCPMSS(t, dut, mssNeighbor, allNeighbors, nil)
	reportSessionResets(t, dut, before)

	t.Log("Start a packet capture on ATE port-1 and ATE port-2")
	cs := otgutils.StartCapture(t, otg)
	defer otgutils.StopCapture(t, otg, cs)

	reestablishSessions(t, env)
	t.Run("Neighbor TCP MSS state", func(t *testing.T) {
		verifyTCPMSS(t, dut, neighborChecks(dut, mssNeighbor, nil))
	})
	established := establishedTransitions(t, dut)
	stopMonitor := monitorSessions(t, dut)

	t.Logf("Advertise %d IPv4 prefixes (%v onwards) and %d IPv6 prefixes (%v onwards) from the eBGP peers on ATE port-1", routeCount, v4Routes.start, routeCount, v6Routes.start)
	t.Cleanup(func() {
		t.Log("Withdraw the prefixes")
		controlRoutes(t, otg, gosnappi.StateProtocolRouteState.WITHDRAW)
	})
	// An OTG can advertise its route ranges again when its peers restart, so
	// the prefixes may already be advertised on the new sessions. The UPDATE
	// messages are then in the capture all the same, which started before the
	// sessions were re-established.
	controlRoutes(t, otg, gosnappi.StateProtocolRouteState.ADVERTISE)
	t.Run("Received prefixes", func(t *testing.T) {
		verifyDUTPrefixesReceived(t, dut)
		verifyATEPrefixesReceived(t, otg)
		verifySessionsStayedEstablished(t, dut, established, stopMonitor(t))
	})

	t.Log("Stop the capture")
	otgutils.StopCapture(t, otg, cs)
	t.Run("Capture", func(t *testing.T) {
		verifyCapture(t, env)
	})
}

// testPeerGroupLevel implements the peer-group-level steps of RT-1.22.1,
// including the neighbor-level override. The removal of the override and the
// last README step, which removes the peer-group MSS, are registered with
// t.Cleanup, so that they also run when a step fails.
func testPeerGroupLevel(t *testing.T, env *testEnv) {
	dut := env.dut
	peerGroups := []string{ebgpPeerGroup, ibgpPeerGroup}
	overrides := overrideSessions()
	overrideMSS := map[session]uint16{}
	for _, s := range overrides {
		overrideMSS[s] = mssOverride
	}

	t.Logf("Set transport/config/tcp-mss to %d on the peer groups %s and %s, with no MSS configured on the neighbors", mssPeerGroup, ebgpPeerGroup, ibgpPeerGroup)
	t.Cleanup(func() {
		t.Log("Remove transport/config/tcp-mss from both peer groups")
		removeTCPMSS(t, dut, nil, peerGroups)
	})
	setTCPMSS(t, dut, mssPeerGroup, nil, peerGroups)
	reestablishSessions(t, env)
	t.Run("Neighbors inherit the peer-group TCP MSS", func(t *testing.T) {
		verifyPeerGroupLevelMSS(t, dut, nil)
	})

	t.Logf("Neighbor-level override: keep the peer-group MSS, and set transport/config/tcp-mss to %d on one neighbor in each peer group (the IPv4 neighbors %s)", mssOverride, strings.Join(neighbors(overrides), " and "))
	// The README step below removes the override again. This cleanup removes
	// it only if a step fails before that.
	overrideRemoved := false
	t.Cleanup(func() {
		if !overrideRemoved {
			t.Logf("Cleanup: remove transport/config/tcp-mss from the IPv4 neighbors %s", strings.Join(neighbors(overrides), " and "))
			removeTCPMSS(t, dut, neighbors(overrides), nil)
		}
	})
	setTCPMSS(t, dut, mssOverride, neighbors(overrides), nil)
	reestablishSessions(t, env)
	t.Run("Neighbor TCP MSS overrides the peer-group TCP MSS", func(t *testing.T) {
		verifyPeerGroupLevelMSS(t, dut, overrideMSS)
	})

	t.Logf("Remove transport/config/tcp-mss from the IPv4 neighbors %s, and keep it on the peer groups", strings.Join(neighbors(overrides), " and "))
	removeTCPMSS(t, dut, neighbors(overrides), nil)
	overrideRemoved = true
	reestablishSessions(t, env)
	t.Run("Neighbors return to the peer-group TCP MSS", func(t *testing.T) {
		verifyPeerGroupLevelMSS(t, dut, nil)
	})
}

// verifyPeerGroupLevelMSS verifies transport/state/tcp-mss at the peer-group
// level: mssPeerGroup for both peer groups, and for the four neighbors the MSS
// in override or else the inherited mssPeerGroup. The peer groups and the
// neighbors are checked in separate subtests, so that a DUT that cannot stream
// one of the two paths still has the other one checked. The peer-group state
// is also read once with a gNMI ONCE subscription, only for the log.
func verifyPeerGroupLevelMSS(t *testing.T, dut *ondatra.DUTDevice, override map[session]uint16) {
	t.Helper()
	t.Run("Peer groups", func(t *testing.T) {
		checks := peerGroupChecks(dut, mssPeerGroup)
		logTCPMSSOnce(t, dut, checks)
		verifyTCPMSS(t, dut, checks)
	})
	t.Run("Neighbors", func(t *testing.T) {
		verifyTCPMSS(t, dut, neighborChecks(dut, mssPeerGroup, override))
	})
}

// logTCPMSSOnce reads transport/state/tcp-mss of each check with a gNMI ONCE
// subscription and logs the result, without failing the test. When a DUT
// rejects the ON_CHANGE subscription that verifyTCPMSS uses, the log then
// shows whether the DUT reports the value at all.
func logTCPMSSOnce(t *testing.T, dut *ondatra.DUTDevice, checks []mssCheck) {
	t.Helper()
	client, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.Name()))
	if err != nil {
		t.Logf("Cannot create a gNMI client for a ONCE subscription: %v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), stateTimeout)
	defer cancel()
	for _, c := range checks {
		v, err := ygnmi.Lookup(ctx, client, c.query)
		if err != nil {
			t.Logf("gNMI ONCE subscription to transport/state/tcp-mss of the %s failed: %v", c.desc, err)
			continue
		}
		t.Logf("gNMI ONCE subscription to transport/state/tcp-mss of the %s returned %s", c.desc, valueString(v))
	}
}

// setTCPMSS replaces transport/config/tcp-mss with mss on the given neighbors
// and peer groups in one gNMI Set. cfgplugins.ConfigureDUTBGPMaxSegmentSize is
// not used: it also sets the interface MTU, skips Arista devices, and has no
// peer-group option.
func setTCPMSS(t *testing.T, dut *ondatra.DUTDevice, mss uint16, nbrs, peerGroups []string) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	for _, n := range nbrs {
		gnmi.BatchReplace(batch, bgpPath(dut).Neighbor(n).Transport().TcpMss().Config(), mss)
	}
	for _, pg := range peerGroups {
		gnmi.BatchReplace(batch, bgpPath(dut).PeerGroup(pg).Transport().TcpMss().Config(), mss)
	}
	batch.Set(t, dut)
}

// removeTCPMSS deletes transport/config/tcp-mss from the given neighbors and
// peer groups in one gNMI Set.
func removeTCPMSS(t *testing.T, dut *ondatra.DUTDevice, nbrs, peerGroups []string) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	for _, n := range nbrs {
		gnmi.BatchDelete(batch, bgpPath(dut).Neighbor(n).Transport().TcpMss().Config())
	}
	for _, pg := range peerGroups {
		gnmi.BatchDelete(batch, bgpPath(dut).PeerGroup(pg).Transport().TcpMss().Config())
	}
	batch.Set(t, dut)
}

// mssCheck is the expected transport/state/tcp-mss of a neighbor or a peer
// group.
type mssCheck struct {
	desc  string
	query ygnmi.SingletonQuery[uint16]
	want  uint16
}

// neighborChecks returns the expected MSS of the four neighbors: the MSS in
// override for the sessions listed there, and want for the others.
func neighborChecks(dut *ondatra.DUTDevice, want uint16, override map[session]uint16) []mssCheck {
	var checks []mssCheck
	for _, s := range sessions {
		mss := want
		if m, ok := override[s]; ok {
			mss = m
		}
		checks = append(checks, mssCheck{desc: s.String(), query: bgpPath(dut).Neighbor(s.neighbor()).Transport().TcpMss().State(), want: mss})
	}
	return checks
}

// peerGroupChecks returns the expected MSS, want, of the two peer groups.
func peerGroupChecks(dut *ondatra.DUTDevice, want uint16) []mssCheck {
	var checks []mssCheck
	for _, l := range links {
		checks = append(checks, mssCheck{desc: "peer group " + l.peerGroup, query: bgpPath(dut).PeerGroup(l.peerGroup).Transport().TcpMss().State(), want: want})
	}
	return checks
}

// verifyTCPMSS waits until transport/state/tcp-mss of each neighbor and peer
// group in checks has the expected value, which is the configured MSS or the
// one inherited from the peer group. The values are watched concurrently.
func verifyTCPMSS(t *testing.T, dut *ondatra.DUTDevice, checks []mssCheck) {
	t.Helper()
	watchers := make([]*gnmi.Watcher[uint16], len(checks))
	for i, c := range checks {
		t.Logf("Verify that transport/state/tcp-mss of the %s is %d", c.desc, c.want)
		watchers[i] = gnmi.Watch(t, onChange(dut), c.query, stateTimeout, func(v *ygnmi.Value[uint16]) bool {
			mss, ok := v.Val()
			return ok && mss == c.want
		})
	}
	for i, w := range watchers {
		if got, ok := w.Await(t); !ok {
			t.Errorf("DUT %s: transport/state/tcp-mss got %s, want %d", checks[i].desc, valueString(got), checks[i].want)
		}
	}
}

// bgpPath returns the path of the DUT BGP instance in the DEFAULT network
// instance.
func bgpPath(dut *ondatra.DUTDevice) *netinstbgp.NetworkInstance_Protocol_BgpPath {
	return gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
}

// onChange returns DUT gNMI options for ON_CHANGE subscriptions, which the
// README lists in its RPC coverage. They are used for the session state and
// the TCP MSS state, which change only on events.
func onChange(dut *ondatra.DUTDevice) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE))
}

// verifySessions waits until the four DUT BGP sessions are ESTABLISHED, and
// reports whether they all are. Each neighbor is watched on its own, instead
// of with cfgplugins.VerifyDUTBGPEstablished, whose watch of all neighbors
// succeeds as soon as any one of them is ESTABLISHED.
func verifySessions(t *testing.T, dut *ondatra.DUTDevice) bool {
	t.Helper()
	watchers := make([]*gnmi.Watcher[oc.E_Bgp_Neighbor_SessionState], len(sessions))
	for i, s := range sessions {
		watchers[i] = gnmi.Watch(t, onChange(dut), bgpPath(dut).Neighbor(s.neighbor()).SessionState().State(), sessionTimeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := v.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		})
	}
	established := true
	for i, w := range watchers {
		if got, ok := w.Await(t); !ok {
			established = false
			t.Errorf("DUT %v: session-state got %s, want ESTABLISHED", sessions[i], valueString(got))
		}
	}
	return established
}

// reestablishSessions re-establishes the four sessions by restarting the ATE
// BGP peers, so that the DUT uses new TCP connections, which apply the MSS
// configured at that point.
func reestablishSessions(t *testing.T, env *testEnv) {
	t.Helper()
	t.Log("Re-establish the four sessions by restarting the ATE BGP peers")
	setATEPeersState(t, env.otg, gosnappi.StateProtocolBgpPeersState.DOWN)
	awaitSessionsDown(t, env)
	setATEPeersState(t, env.otg, gosnappi.StateProtocolBgpPeersState.UP)
	t.Log("Verify that all four sessions are ESTABLISHED")
	if !verifySessions(t, env.dut) {
		t.Fatal("Not all four BGP sessions are ESTABLISHED after they were re-established")
	}
}

// awaitSessionsDown waits until the stopped ATE BGP peers and the DUT sessions
// have left ESTABLISHED, so that the sessions brought up next use new TCP
// connections. A stopped OTG peer does not always report IDLE: it can report
// CONNECT while the DUT tries to reconnect.
func awaitSessionsDown(t *testing.T, env *testEnv) {
	t.Helper()
	ateWatchers := make([]*gnmi.Watcher[otgtelemetry.E_BgpPeer_SessionState], len(sessions))
	dutWatchers := make([]*gnmi.Watcher[oc.E_Bgp_Neighbor_SessionState], len(sessions))
	for i, s := range sessions {
		ateWatchers[i] = gnmi.Watch(t, env.otg, gnmi.OTG().BgpPeer(s.otgPeer()).SessionState().State(), holdTimeout, func(v *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
			state, ok := v.Val()
			return !ok || state != otgtelemetry.BgpPeer_SessionState_ESTABLISHED
		})
		dutWatchers[i] = gnmi.Watch(t, onChange(env.dut), bgpPath(env.dut).Neighbor(s.neighbor()).SessionState().State(), holdTimeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := v.Val()
			return ok && state != oc.Bgp_Neighbor_SessionState_ESTABLISHED
		})
	}
	for i, s := range sessions {
		if got, ok := ateWatchers[i].Await(t); !ok {
			t.Errorf("ATE BGP peer %s: session-state got %s after the peer was stopped, want a state other than ESTABLISHED", s.otgPeer(), valueString(got))
		}
		if got, ok := dutWatchers[i].Await(t); !ok {
			t.Errorf("DUT %v: session-state got %s after the ATE peer was stopped, want a state other than ESTABLISHED", s, valueString(got))
		}
	}
}

// establishedTransitions reads state/established-transitions of the four
// neighbors in one request, keyed by neighbor address. Neighbors that do not
// report it are absent from the result.
func establishedTransitions(t *testing.T, dut *ondatra.DUTDevice) map[string]uint64 {
	t.Helper()
	transitions := map[string]uint64{}
	for _, v := range gnmi.LookupAll(t, dut, bgpPath(dut).NeighborAny().EstablishedTransitions().State()) {
		n, ok := v.Val()
		if !ok {
			continue
		}
		if s, ok := sessionOf(v.Path); ok {
			transitions[s.neighbor()] = n
		}
	}
	return transitions
}

// sessionOf returns the session of the neighbor in a path that a NeighborAny
// query returned. The neighbor address is compared as an IP address, because
// the DUT may render it differently from the README.
func sessionOf(p *gpb.Path) (session, bool) {
	for _, e := range p.GetElem() {
		if e.GetName() != "neighbor" {
			continue
		}
		addr, err := netip.ParseAddr(e.GetKey()["neighbor-address"])
		if err != nil {
			return session{}, false
		}
		for _, s := range sessions {
			if netip.MustParseAddr(s.neighbor()) == addr {
				return s, true
			}
		}
	}
	return session{}, false
}

// reportSessionResets records whether applying the MSS reset the sessions:
// whether state/session-state left ESTABLISHED, or
// state/established-transitions increased from before, within resetWindow of
// the change. It shows whether the change is disruptive; as in the README, it
// is reported only and is not a pass/fail criterion.
func reportSessionResets(t *testing.T, dut *ondatra.DUTDevice, before map[string]uint64) {
	t.Helper()
	t.Logf("Record whether applying the MSS reset the sessions within %v of the change", resetWindow)
	left := map[string]bool{}
	gnmi.WatchAll(t, onChange(dut), bgpPath(dut).NeighborAny().SessionState().State(), resetWindow, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		if state, ok := v.Val(); ok && state != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
			if s, ok := sessionOf(v.Path); ok {
				left[s.neighbor()] = true
			}
		}
		return len(left) == len(sessions)
	}).Await(t)
	after := establishedTransitions(t, dut)
	for _, s := range sessions {
		n := s.neighbor()
		transitions, increased := "not reported", false
		if b, ok := before[n]; ok {
			if a, ok := after[n]; ok {
				transitions, increased = fmt.Sprintf("%d before, %d after", b, a), a > b
			}
		}
		result := "did not reset"
		if left[n] || increased {
			result = "reset"
		}
		t.Logf("DUT %v: applying the MSS %s the session (session-state left ESTABLISHED: %t; established-transitions: %s)", s, result, left[n], transitions)
	}
}

// verifyDUTPrefixesReceived waits until the DUT reports routeCount prefixes in
// afi-safis/afi-safi/state/prefixes/received for the IPv4 and the IPv6 eBGP
// neighbors.
func verifyDUTPrefixesReceived(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	for _, r := range routeRanges {
		s := session{link1, r.f}
		t.Logf("Verify that the DUT reports %d prefixes received from the %v", routeCount, s)
		q := bgpPath(dut).Neighbor(s.neighbor()).AfiSafi(r.f.afiSafi()).Prefixes().Received().State()
		got, ok := gnmi.Watch(t, dut, q, routeTimeout, func(v *ygnmi.Value[uint32]) bool {
			n, ok := v.Val()
			return ok && n == routeCount
		}).Await(t)
		if !ok {
			t.Errorf("DUT %v: prefixes/received got %s, want %d", s, valueString(got), routeCount)
		}
	}
}

// verifyATEPrefixesReceived waits until the ATE iBGP peers on port-2 have
// learned all routeCount IPv4 and IPv6 prefixes from the DUT.
func verifyATEPrefixesReceived(t *testing.T, otg *otg.OTG) {
	t.Helper()
	for _, r := range routeRanges {
		s := session{link2, r.f}
		t.Logf("Verify that the ATE peer of the %v receives all %d %s prefixes from the DUT", s, routeCount, r.f)
		want := r.prefixes()
		learned := map[netip.Prefix]bool{}
		// learn records a prefix that the ATE peer has learned, and reports
		// whether it has learned all prefixes of the route range.
		learn := func(addr string, length uint32) bool {
			if p, err := netip.ParsePrefix(fmt.Sprintf("%s/%d", addr, length)); err == nil && want[p.Masked()] {
				learned[p.Masked()] = true
			}
			return len(learned) == len(want)
		}
		peer := gnmi.OTG().BgpPeer(s.otgPeer())
		var ok bool
		switch r.f {
		case ipv4:
			_, ok = gnmi.WatchAll(t, otg, peer.UnicastIpv4PrefixAny().State(), routeTimeout, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
				p, present := v.Val()
				return present && learn(p.GetAddress(), p.GetPrefixLength())
			}).Await(t)
		case ipv6:
			_, ok = gnmi.WatchAll(t, otg, peer.UnicastIpv6PrefixAny().State(), routeTimeout, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
				p, present := v.Val()
				return present && learn(p.GetAddress(), p.GetPrefixLength())
			}).Await(t)
		}
		if !ok {
			t.Errorf("ATE peer %s: learned %d of the %d %s prefixes from %v onwards, want all", s.otgPeer(), len(learned), routeCount, r.f, r.start)
		}
	}
}

// monitorSessions watches the session state of the four neighbors in the
// background. The returned function stops the watch and returns the sessions
// that left ESTABLISHED while it ran, so that a session reset is detected also
// on a DUT that does not report state/established-transitions. It takes the
// *testing.T of the (sub)test that calls it.
func monitorSessions(t *testing.T, dut *ondatra.DUTDevice) func(*testing.T) []session {
	t.Helper()
	left := map[session]bool{}
	w := gnmi.WatchAll(t, onChange(dut), bgpPath(dut).NeighborAny().SessionState().State(), monitorTimeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		if state, ok := v.Val(); ok && state != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
			if s, ok := sessionOf(v.Path); ok {
				left[s] = true
			}
		}
		// Keep watching until the returned function stops the watch.
		return false
	})
	t.Cleanup(w.Cancel)
	return func(t *testing.T) []session {
		t.Helper()
		w.Cancel()
		w.Await(t)
		var ss []session
		for _, s := range sessions {
			if left[s] {
				ss = append(ss, s)
			}
		}
		return ss
	}
}

// verifySessionsStayedEstablished verifies that the four sessions are still
// ESTABLISHED and that none was reset while the prefixes were advertised:
// none is in left, the sessions that monitorSessions saw leave ESTABLISHED,
// and none has a higher state/established-transitions than in before.
func verifySessionsStayedEstablished(t *testing.T, dut *ondatra.DUTDevice, before map[string]uint64, left []session) {
	t.Helper()
	t.Log("Verify that all four sessions are still ESTABLISHED")
	for _, s := range left {
		t.Errorf("DUT %v: session-state left ESTABLISHED while the prefixes were advertised, want the session to stay ESTABLISHED", s)
	}
	verifySessions(t, dut)
	after := establishedTransitions(t, dut)
	for _, s := range sessions {
		b, okBefore := before[s.neighbor()]
		a, okAfter := after[s.neighbor()]
		if okBefore && okAfter && a != b {
			t.Errorf("DUT %v: state/established-transitions went from %d to %d while the prefixes were advertised, want the session to stay ESTABLISHED", s, b, a)
		}
	}
}

// bgpSegment is a BGP TCP segment between the DUT and an ATE peer of the four
// sessions, read from a packet capture.
type bgpSegment struct {
	s       session
	fromDUT bool
	syn     bool   // The segment is a SYN or a SYN-ACK.
	ack     bool   // The segment has the ACK flag: with syn, it is a SYN-ACK.
	mss     uint16 // MSS option of the segment, if hasMSS.
	hasMSS  bool
	payload int    // Number of bytes of TCP payload.
	desc    string // Addresses, ports, and sequence number of the segment.
}

// synKind returns "SYN" for a SYN, which the DUT sends to open a connection
// itself, and "SYN-ACK" for a SYN-ACK, which it sends when the ATE peer opens
// the connection. Vendors may set the MSS option differently on the two.
func (seg bgpSegment) synKind() string {
	if seg.ack {
		return "SYN-ACK"
	}
	return "SYN"
}

// verifyCapture verifies the capture of ATE port-1 and port-2 from the
// neighbor-level step: the MSS option of every TCP SYN or SYN-ACK that the DUT
// sends to each of the four ATE peers is mssNeighbor, and no TCP segment
// between the DUT and the ATE, in either direction, has more than mssNeighbor
// bytes of payload.
func verifyCapture(t *testing.T, env *testEnv) {
	t.Helper()
	t.Logf("Verify in the capture that the MSS option in the TCP SYN or SYN-ACK that the DUT sends to each of the four ATE peers is %d, and that no TCP segment between the DUT and the ATE, in either direction, has more than %d bytes of payload", mssNeighbor, mssNeighbor)
	var segments []bgpSegment
	for _, l := range links {
		segments = append(segments, readBGPSegments(t, env.otg, env.ate.Port(t, l.port).ID())...)
	}

	// synSegments are the TCP SYN and SYN-ACK segments that the DUT sends on
	// each session, in capture order.
	synSegments := map[session][]bgpSegment{}
	// withPayload and largest count the segments that carry payload, and
	// their largest payload, from the DUT (true) and to the DUT (false).
	withPayload := map[bool]int{}
	largest := map[bool]int{}
	// sentBytes is the payload that the DUT sends on each session.
	sentBytes := map[session]int{}
	var oversized []string
	for _, seg := range segments {
		if seg.syn && seg.fromDUT {
			synSegments[seg.s] = append(synSegments[seg.s], seg)
		}
		if seg.payload > 0 {
			withPayload[seg.fromDUT]++
			largest[seg.fromDUT] = max(largest[seg.fromDUT], seg.payload)
		}
		if seg.fromDUT {
			sentBytes[seg.s] += seg.payload
		}
		if seg.payload > int(mssNeighbor) {
			oversized = append(oversized, fmt.Sprintf("%s with %d bytes", seg.desc, seg.payload))
		}
	}

	for _, s := range sessions {
		got := synSegments[s]
		if len(got) == 0 {
			t.Errorf("Capture: no TCP SYN or SYN-ACK from the DUT to the ATE peer of the %v, want one with MSS option %d", s, mssNeighbor)
			continue
		}
		var options []string
		for _, seg := range got {
			if !seg.hasMSS {
				options = append(options, seg.synKind()+" without MSS option")
				t.Errorf("Capture: TCP %s from the DUT to the ATE peer of the %v (%s) has no MSS option, want %d", seg.synKind(), s, seg.desc, mssNeighbor)
				continue
			}
			options = append(options, fmt.Sprintf("%s %d", seg.synKind(), seg.mss))
			if seg.mss != mssNeighbor {
				t.Errorf("Capture: TCP %s from the DUT to the ATE peer of the %v (%s) has MSS option %d, want %d", seg.synKind(), s, seg.desc, seg.mss, mssNeighbor)
			}
		}
		t.Logf("Capture: MSS options of the TCP SYN and SYN-ACK segments from the DUT to the ATE peer of the %v, in capture order: %s", s, strings.Join(options, ", "))
	}

	t.Logf("Capture: %d TCP segments with payload from the DUT, the largest with %d bytes; %d to the DUT, the largest with %d bytes", withPayload[true], largest[true], withPayload[false], largest[false])
	// The UPDATE messages that the DUT sends to the iBGP peers on ATE port-2
	// hold at least the NLRI of all prefixes: a length byte and the prefix
	// bytes for each. Less payload means that the capture misses the UPDATE
	// messages, and the payload check below would not cover them.
	for _, r := range routeRanges {
		s := session{link2, r.f}
		nlri := routeCount * (1 + (r.start.Bits()+7)/8)
		if sentBytes[s] < nlri {
			t.Errorf("Capture: the TCP segments from the DUT to the ATE peer of the %v carry %d bytes of payload, want at least the %d bytes of NLRI of the %d prefixes", s, sentBytes[s], nlri, routeCount)
		}
	}
	if withPayload[false] == 0 {
		t.Log("Capture: no TCP segment with payload to the DUT; an ATE port capture can hold only the frames that the port receives, so only the segments from the DUT are checked")
	}
	if len(oversized) > 0 {
		t.Errorf("Capture: %d TCP segments have more than %d bytes of payload, want none; for example %s", len(oversized), mssNeighbor, strings.Join(oversized[:min(len(oversized), 5)], "; "))
	}
}

// readBGPSegments returns the BGP TCP segments between the DUT and the ATE
// peers in the capture of an ATE port.
func readBGPSegments(t *testing.T, otg *otg.OTG, port string) []bgpSegment {
	t.Helper()
	data := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	r, err := pcapgo.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Errorf("ATE port %s: cannot read the capture of %d bytes: %v", port, len(data), err)
		return nil
	}
	// A capture writer can put a snap length smaller than the packets it
	// writes in the file header, and the reader rejects longer packets. The
	// largest snap length that the reader accepts keeps the jumbo segments.
	r.SetSnaplen(maxSnaplen)
	var segments []bgpSegment
	for {
		raw, _, err := r.ReadPacketData()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Errorf("ATE port %s: cannot read a packet of the capture: %v", port, err)
			break
		}
		if seg, ok := bgpSegmentOf(gopacket.NewPacket(raw, r.LinkType(), gopacket.Default)); ok {
			segments = append(segments, seg)
		}
	}
	t.Logf("ATE port %s: the capture has %d BGP TCP segments between the DUT and the ATE", port, len(segments))
	return segments
}

// bgpSegmentOf returns the BGP TCP segment in a captured packet, if the packet
// is one between the DUT and an ATE peer of the four sessions.
func bgpSegmentOf(pkt gopacket.Packet) (bgpSegment, bool) {
	tcp, ok := pkt.Layer(layers.LayerTypeTCP).(*layers.TCP)
	if !ok || (tcp.SrcPort != bgpPort && tcp.DstPort != bgpPort) {
		return bgpSegment{}, false
	}
	// The payload length is taken from the IP header, so that it is right
	// also for a packet that the capture truncated.
	header := int(tcp.DataOffset) * 4
	payload := len(tcp.Payload)
	var src, dst netip.Addr
	switch ip := pkt.NetworkLayer().(type) {
	case *layers.IPv4:
		src, _ = netip.AddrFromSlice(ip.SrcIP)
		dst, _ = netip.AddrFromSlice(ip.DstIP)
		payload = int(ip.Length) - int(ip.IHL)*4 - header
	case *layers.IPv6:
		src, _ = netip.AddrFromSlice(ip.SrcIP)
		dst, _ = netip.AddrFromSlice(ip.DstIP)
		if ip.NextHeader == layers.IPProtocolTCP {
			payload = int(ip.Length) - header
		}
	default:
		return bgpSegment{}, false
	}
	src, dst = src.Unmap(), dst.Unmap()
	for _, s := range sessions {
		local, peer := netip.MustParseAddr(s.local()), netip.MustParseAddr(s.neighbor())
		fromDUT := src == local && dst == peer
		if !fromDUT && (src != peer || dst != local) {
			continue
		}
		seg := bgpSegment{
			s:       s,
			fromDUT: fromDUT,
			syn:     tcp.SYN,
			ack:     tcp.ACK,
			payload: payload,
			desc:    fmt.Sprintf("%v port %d to %v port %d, sequence number %d", src, uint16(tcp.SrcPort), dst, uint16(tcp.DstPort), tcp.Seq),
		}
		for _, o := range tcp.Options {
			if o.OptionType == layers.TCPOptionKindMSS && len(o.OptionData) == 2 {
				seg.mss, seg.hasMSS = binary.BigEndian.Uint16(o.OptionData), true
			}
		}
		return seg, true
	}
	return bgpSegment{}, false
}

// valueString renders the last value that a watch received, for error
// messages.
func valueString[T any](v *ygnmi.Value[T]) string {
	if v == nil {
		return "no value"
	}
	if got, ok := v.Val(); ok {
		return fmt.Sprint(got)
	}
	return "no value"
}
