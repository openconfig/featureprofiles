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

// Package aft_prefix_filtering_service provides the shared topology, BGP
// scale, gNMI, and AFT validation helpers for the AFT prefix-filtering
// feature tests (AFT-6.x). It brings up the common two-port DUT/ATE BGP
// topology, dials a reusable raw gNMI client for the streamed AFT
// subscription, and validates the streamed AFT cache contents.
package aft_prefix_filtering_service

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	// Package-level constants shared by the AFT prefix-filtering tests.
	AFTSubscriptionWait  = 3 * time.Minute
	StaticRouteIndex     = 100
	PolicyMatchAll       = "POLICY-MATCH-ALL"
	DefaultStatementName = "10"
	PfxMode              = "exact"
	BulkV4BaseAddr       = "80.0.0.1"
	BulkV4RouteCount     = 1500000
	BulkV4PrefixLen      = 32
	BulkV6BaseAddr       = "3000::1"
	BulkV6RouteCount     = 500000
	BulkV6PrefixLen      = 128

	// Local constants for the AFT prefix-filtering service.
	dutAS                 = 65001
	ateAS                 = 65002
	bgpV4PeerGroup        = "BGP-BULK-V4-PEER-GROUP"
	bgpV6PeerGroup        = "BGP-BULK-V6-PEER-GROUP"
	bgpSessionTimeout     = 2 * time.Minute
	bgpConvergenceWait    = 10 * time.Minute
	aristaPersistConfig   = "management api gnmi\ntransport grpc default\noperation set persistence"
	aristaNoPersistConfig = "management api gnmi\ntransport grpc default\nno operation set persistence"
)

var (
	// Package-level variables shared by the AFT prefix-filtering tests
	DUTPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port 1",
		MAC:     "02:00:02:02:02:02",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:1::1",
		IPv6Len: 64,
	}
	ATEPort1 = attrs.Attributes{
		Name:    "atePort1",
		Desc:    "ATE to DUT Port 1",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:1::2",
		IPv6Len: 64,
	}
	DUTPort2 = attrs.Attributes{
		Desc:    "DUT to ATE Port 2",
		MAC:     "02:00:04:02:02:02",
		IPv4:    "192.0.3.1",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:2::1",
		IPv6Len: 64,
	}
	ATEPort2 = attrs.Attributes{
		Name:    "atePort2",
		Desc:    "ATE to DUT Port 2",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.3.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:2::2",
		IPv6Len: 64,
	}

	// Local variables for the AFT prefix-filtering service
	debugNotifications = flag.Bool("debug_notifications", true, "Enable full AFT notification recording")
	gnmiOnce           sync.Once
	gnmiClient         gpb.GNMIClient
	gnmiErr            error
)

// PrefixesParams contains the prefixes expected to be present in the AFT cache.
type PrefixesParams struct {
	InfoAFT  *aftcache.AFTData
	Prefixes []string
	Ctx      context.Context
}

// GnmiClientSession get the GNMI client session.
func GnmiClientSession(t *testing.T, dut *ondatra.DUTDevice, cfg PrefixesParams) gpb.GNMIClient {
	t.Helper()
	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(cfg.Ctx)
	if err != nil {
		t.Fatalf("Failed to dial GNMI client: %v", err)
	}
	return gnmiClient
}

// VerifyPrefixesPresent validates expected prefixes exist.
func VerifyPrefixesPresent(t *testing.T, cfg PrefixesParams) {
	t.Helper()
	for _, pfx := range cfg.Prefixes {
		if _, ok := cfg.InfoAFT.Prefixes[pfx]; !ok {
			t.Fatalf("Expected prefix missing: %s", pfx)
		}
	}
}

// VerifyPrefixesAbsent validates prefixes do not exist.
func VerifyPrefixesAbsent(t *testing.T, cfg PrefixesParams) {
	t.Helper()
	for _, pfx := range cfg.Prefixes {
		if _, ok := cfg.InfoAFT.Prefixes[pfx]; ok {
			t.Fatalf("Unexpected prefix present: %s", pfx)
		}
	}
}

// RunCollectorParams contains the parameters required to execute an AFT collector.
// It runs until the supplied stopping condition is satisfied.
type RunCollectorParams struct {
	Ctx       context.Context
	Collector *aftcache.AFTStreamSession
	Stop      aftcache.PeriodicHook
	Timeout   time.Duration
}

// RunCollector starts the AFT stream collector.
// It blocks until the supplied stopping condition is satisfied or the collector times out.
func RunCollector(t *testing.T, cfg RunCollectorParams) {
	t.Helper()
	cfg.Collector.ListenUntil(cfg.Ctx, t, cfg.Timeout, cfg.Stop)
}

// CollectAndVerify runs the collector per cfg, converts the resulting
// notifications into an AFT snapshot, and verifies that the given prefixes
// are present/absent in that snapshot (either slice may be nil/empty to skip
// that check). It returns the resulting snapshot (nil on ToAFT failure, in
// which case a t.Errorf has already been logged) so callers can layer on any
// additional, case-specific checks.
func CollectAndVerify(t *testing.T, dut *ondatra.DUTDevice, cfg RunCollectorParams,
	present, absent []string) *aftcache.AFTData {
	t.Helper()
	RunCollector(t, cfg)
	aft, err := cfg.Collector.ToAFT(t, dut)
	if err != nil {
		t.Errorf("ToAFT failed: %v", err)
		return nil
	}
	if len(present) > 0 {
		VerifyPrefixesPresent(t, PrefixesParams{InfoAFT: aft, Prefixes: present})
	}
	if len(absent) > 0 {
		VerifyPrefixesAbsent(t, PrefixesParams{InfoAFT: aft, Prefixes: absent})
	}
	return aft
}

// RemovePrefixFromPrefixSetParams contains the parameters required to remove a prefix entry from a routing policy prefix set.
type RemovePrefixFromPrefixSetParams struct {
	PrefixSetName string
	Prefix        string
	MaskRange     string
}

// RemovePrefixFromPrefixSet removes the specified prefix entry from the given routing-policy prefix-set on the DUT.
func RemovePrefixFromPrefixSet(t *testing.T, dut *ondatra.DUTDevice, cfg RemovePrefixFromPrefixSetParams) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	gnmi.BatchDelete(batch, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(cfg.PrefixSetName).Prefix(cfg.Prefix, cfg.MaskRange).Config())
	batch.Set(t, dut)
}

// NewCollectorParams contains the parameters required to create a new AFT stream session.
type NewCollectorParams struct {
	Context context.Context
	Client  gpb.GNMIClient
}

// NewCollector creates and returns a new AFT stream session.
// If debug_notifications is enabled, all received gNMI notifications are recorded in memory
// for later inspection and troubleshooting.
func NewCollector(t *testing.T, dut *ondatra.DUTDevice, cfg NewCollectorParams) *aftcache.AFTStreamSession {
	t.Helper()
	c := aftcache.NewAFTStreamSession(cfg.Context, t, cfg.Client, dut)
	if *debugNotifications {
		c.WithDebug()
		t.Log("DEBUG MODE ENABLED: Recording all gNMI notifications to memory.")
	}
	return c
}

// ConfigureDUT configures the DUT with basic port1/port2 interfaces in the DEFAULT network instance.
func ConfigureDUT(t *testing.T, dut *ondatra.DUTDevice) *gnmi.SetBatch {
	t.Helper()
	batch := &gnmi.SetBatch{}
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	p1 := dut.Port(t, "port1")
	intf1 := &oc.Interface{Name: ygot.String(p1.Name())}
	DUTPort1.ConfigOCInterface(intf1, dut)
	gnmi.BatchUpdate(batch, gnmi.OC().Interface(p1.Name()).Config(), intf1)

	p2 := dut.Port(t, "port2")
	intf2 := &oc.Interface{Name: ygot.String(p2.Name())}
	DUTPort2.ConfigOCInterface(intf2, dut)
	gnmi.BatchUpdate(batch, gnmi.OC().Interface(p2.Name()).Config(), intf2)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		ni := deviations.DefaultNetworkInstance(dut)
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), ni, 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), ni, 0)
	}
	return batch
}

// ConfigureATE configures the ATE ports and returns the topology along with the list of configured device (interface) names.
func ConfigureATE(t *testing.T, ate *ondatra.ATEDevice) (gosnappi.Config, []string) {
	t.Helper()
	interfaceNamesList := []string{}
	topo := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	ATEPort1.AddToOTG(topo, p1, &DUTPort1)
	ATEPort2.AddToOTG(topo, p2, &DUTPort2)
	for _, dev := range topo.Devices().Items() {
		interfaceNamesList = append(interfaceNamesList, dev.Name())
	}
	return topo, interfaceNamesList
}

// ConfigureBGP configures two eBGP neighbors in the DEFAULT network instance.
// It assigns one neighbor per port and carries the bulk background routes
// (1.5M IPv4 and 500k IPv6).
func ConfigureBGP(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, ni *oc.NetworkInstance) {
	t.Helper()
	configureBGPInstance(t, ni, DUTPort1.IPv4, []bgpNeighborSpec{
		{address: ATEPort1.IPv4, peerGroup: bgpV4PeerGroup, v4: true},
		{address: ATEPort2.IPv6, peerGroup: bgpV6PeerGroup, v4: false},
	})
	gnmi.BatchUpdate(batch, gnmi.OC().NetworkInstance(ni.GetName()).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Config(), ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)))
}

type BGPPrefixParams struct {
	V4Prefix        string
	V6Prefix        string
	NetworkInstance *oc.NetworkInstance
}

// ApplyBGPMaxPrefixes applies the Arista max-prefixes deviation to the DEFAULT-instance eBGP neighbors.
func ApplyBGPMaxPrefixes(t *testing.T, dut *ondatra.DUTDevice, cfg BGPPrefixParams) {
	t.Helper()
	if !deviations.BGPMissingOCMaxPrefixesConfiguration(dut) {
		return
	}
	for _, addr := range []string{cfg.V4Prefix, cfg.V6Prefix} {
		cfgplugins.DeviationAristaBGPNeighborMaxPrefixes(t, dut, addr, 0)
	}
}

// BGPParams parameterizes a per-network-instance eBGP peering.
// It advertises scaled IPv4 and IPv6 background routes over a single connected port.
type BGPParams struct {
	NetworkInstance *oc.NetworkInstance
	RouterID        string
	V4Neighbor      string
	V6Neighbor      string
}

// ConfigureScaleBGP configures a dual-AFI eBGP peering in the given network instance.
// It uses a single connected port to advertise scaled IPv4 and IPv6 background routes
// into either the default or a VRF network instance.
func ConfigureScaleBGP(t *testing.T, dut *ondatra.DUTDevice, cfg BGPParams) {
	t.Helper()
	v4PeerGroup := fmt.Sprintf("%s-%s", bgpV4PeerGroup, cfg.NetworkInstance.GetName())
	v6PeerGroup := fmt.Sprintf("%s-%s", bgpV6PeerGroup, cfg.NetworkInstance.GetName())
	configureBGPInstance(t, cfg.NetworkInstance, cfg.RouterID, []bgpNeighborSpec{
		{address: cfg.V4Neighbor, peerGroup: v4PeerGroup, v4: true},
		{address: cfg.V6Neighbor, peerGroup: v6PeerGroup, v4: false},
	})
	if deviations.BGPMissingOCMaxPrefixesConfiguration(dut) {
		cfgplugins.DeviationAristaBGPNeighborMaxPrefixes(t, dut, cfg.V4Neighbor, 0)
		cfgplugins.DeviationAristaBGPNeighborMaxPrefixes(t, dut, cfg.V6Neighbor, 0)
	}
}

// bgpNeighborSpec describes a single-AFI eBGP neighbor: when v4 is
// true the IPv4 unicast AFI is enabled (IPv6 unicast disabled), and vice versa.
type bgpNeighborSpec struct {
	address   string
	peerGroup string
	v4        bool
}

// configureBGPInstance configures a complete eBGP protocol instance in the given network instance.
// It applies the supplied router ID and single-AFI neighbors with the accept-all import/export policy.
func configureBGPInstance(t *testing.T, ni *oc.NetworkInstance, routerID string, neighbors []bgpNeighborSpec) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	if ni == nil {
		t.Fatalf("Network Instance is not configured")
	}
	niProto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut))
	niProto.Enabled = ygot.Bool(true)
	bgp := niProto.GetOrCreateBgp()
	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAS)
	g.RouterId = ygot.String(routerID)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	for _, n := range neighbors {
		nb := bgp.GetOrCreateNeighbor(n.address)
		nb.PeerAs = ygot.Uint32(ateAS)
		nb.Enabled = ygot.Bool(true)
		afiSafiType := oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
		if !n.v4 {
			afiSafiType = oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
		}

		afiSafi := nb.GetOrCreateAfiSafi(afiSafiType)
		afiSafi.Enabled = ygot.Bool(true)

		ap := afiSafi.GetOrCreateApplyPolicy()
		ap.ImportPolicy = []string{PolicyMatchAll}
		ap.ExportPolicy = []string{PolicyMatchAll}
	}
}

// ConfigureATEBGP attaches BGPv4 and BGPv6 peers to the ATE topology.
// It advertises the bulk IPv4 and IPv6 background route ranges over separate ports.
func ConfigureATEBGP(t *testing.T, topo gosnappi.Config) {
	t.Helper()
	dev1 := topo.Devices().Items()[0]
	ipv4Name1 := fmt.Sprintf("%s.IPv4", ATEPort1.Name)
	bgp1 := dev1.Bgp().SetRouterId(ATEPort1.IPv4)
	peer1 := bgp1.Ipv4Interfaces().Add().SetIpv4Name(ipv4Name1).Peers().Add().SetName("atePort1.BGP4.peer")
	peer1.SetPeerAddress(DUTPort1.IPv4).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	v4Routes := peer1.V4Routes().Add().SetName("bulk-ipv4-routes")
	v4Routes.SetNextHopIpv4Address(ATEPort1.IPv4).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	v4Routes.Addresses().Add().SetAddress(BulkV4BaseAddr).SetPrefix(BulkV4PrefixLen).SetCount(BulkV4RouteCount).SetStep(1)
	dev2 := topo.Devices().Items()[1]
	ipv6Name2 := fmt.Sprintf("%s.IPv6", ATEPort2.Name)
	bgp2 := dev2.Bgp().SetRouterId(ATEPort2.IPv4)
	peer2 := bgp2.Ipv6Interfaces().Add().SetIpv6Name(ipv6Name2).Peers().Add().SetName("atePort2.BGP6.peer")
	peer2.SetPeerAddress(DUTPort2.IPv6).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	v6Routes := peer2.V6Routes().Add().SetName("bulk-ipv6-routes")
	v6Routes.SetNextHopIpv6Address(ATEPort2.IPv6).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	v6Routes.Addresses().Add().SetAddress(BulkV6BaseAddr).SetPrefix(BulkV6PrefixLen).SetCount(BulkV6RouteCount).SetStep(1)
}

// ATEBGPParams parameterizes ATE-side eBGP peers advertising scaled
// IPv4/IPv6 background routes over a single connected port.
type ATEBGPParams struct {
	DUTPort      attrs.Attributes
	ATEPort      attrs.Attributes
	NamePrefix   string
	V4RouteCount uint32
	V4BaseAddr   string
	V4PrefixLen  uint32
	V6RouteCount uint32
	V6BaseAddr   string
	V6PrefixLen  uint32
}

// ConfigureATEScaleBGP attaches dual-AFI eBGP peers to the supplied
// ATE device, advertising the parameterized IPv4/IPv6 background route ranges.
func ConfigureATEScaleBGP(t *testing.T, dev gosnappi.Device, cfg ATEBGPParams) {
	t.Helper()
	bgp := dev.Bgp().SetRouterId(cfg.ATEPort.IPv4)
	ipv4Name := fmt.Sprintf("%s.IPv4", cfg.ATEPort.Name)
	peer4 := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4Name).Peers().Add().SetName(fmt.Sprintf("%s.BGP4.peer", cfg.NamePrefix))
	peer4.SetPeerAddress(cfg.DUTPort.IPv4).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	v4Routes := peer4.V4Routes().Add().SetName(fmt.Sprintf("%s-ipv4-routes", cfg.NamePrefix))
	v4Routes.SetNextHopIpv4Address(cfg.ATEPort.IPv4).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	v4Routes.Addresses().Add().SetAddress(cfg.V4BaseAddr).SetPrefix(cfg.V4PrefixLen).SetCount(cfg.V4RouteCount).SetStep(1)
	ipv6Name := fmt.Sprintf("%s.IPv6", cfg.ATEPort.Name)
	peer6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6Name).Peers().Add().SetName(fmt.Sprintf("%s.BGP6.peer", cfg.NamePrefix))
	peer6.SetPeerAddress(cfg.DUTPort.IPv6).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	v6Routes := peer6.V6Routes().Add().SetName(fmt.Sprintf("%s-ipv6-routes", cfg.NamePrefix))
	v6Routes.SetNextHopIpv6Address(cfg.ATEPort.IPv6).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	v6Routes.Addresses().Add().SetAddress(cfg.V6BaseAddr).SetPrefix(cfg.V6PrefixLen).SetCount(cfg.V6RouteCount).SetStep(1)
}

// ConfigureBaseRoutesParams defines the parameters for configuring the baseline static routes.
type ConfigureBaseRoutesParams struct {
	V4Prefixes []string
	V6Prefixes []string
}

// ConfigureBaseRoutes installs the baseline static routes into the DEFAULT
// network instance. IPv4 prefixes use the ATE port1 IPv4 next-hop, and IPv6
// prefixes use the ATE port1 IPv6 next-hop.
func ConfigureBaseRoutes(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, p ConfigureBaseRoutesParams) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	for idx, prefix := range p.V4Prefixes {
		cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: ni, Prefix: prefix, Index: fmt.Sprintf("%d", StaticRouteIndex+idx), NextHop: ATEPort1.IPv4})
	}
	for idx, prefix := range p.V6Prefixes {
		cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: ni, Prefix: prefix, Index: fmt.Sprintf("%d", StaticRouteIndex+100+idx), NextHop: ATEPort1.IPv6})
	}
	batch.Set(t, dut)
}

// AwaitBGPConvergence waits for both eBGP sessions to establish and for the
// expected bulk prefix counts to install. It records failures with t.Errorf so
// the caller can continue.
func AwaitBGPConvergence(t *testing.T, dut *ondatra.DUTDevice, niName string) {
	t.Helper()
	AwaitScaleBGPConvergence(t, dut, BGPConvergenceParams{
		NetworkInstance: niName,
		V4Neighbor:      ATEPort1.IPv4,
		V6Neighbor:      ATEPort2.IPv6,
		V4RouteCount:    BulkV4RouteCount,
		V6RouteCount:    BulkV6RouteCount,
	})
}

// BGPConvergenceParams parameterizes the BGP convergence check for a
// dual-AFI peering advertising scaled routes over a single connected port.
type BGPConvergenceParams struct {
	NetworkInstance string
	V4Neighbor      string
	V6Neighbor      string
	V4RouteCount    uint32
	V6RouteCount    uint32
}

// AwaitScaleBGPConvergence waits for both eBGP sessions to establish and for
// the expected IPv4 and IPv6 prefix counts to be installed. It records
// failures with t.Errorf so the caller can continue.
func AwaitScaleBGPConvergence(t *testing.T, dut *ondatra.DUTDevice, cfg BGPConvergenceParams) {
	t.Helper()
	if err := awaitBGPEstablished(t, dut, cfg.NetworkInstance, cfg.V4Neighbor); err != nil {
		t.Errorf("%v", err)
	}
	if err := awaitBGPEstablished(t, dut, cfg.NetworkInstance, cfg.V6Neighbor); err != nil {
		t.Errorf("%v", err)
	}
	if err := awaitBGPPrefixCount(t, dut, cfg.NetworkInstance, cfg.V4Neighbor, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, cfg.V4RouteCount); err != nil {
		t.Errorf("%v", err)
	}
	if err := awaitBGPPrefixCount(t, dut, cfg.NetworkInstance, cfg.V6Neighbor, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, cfg.V6RouteCount); err != nil {
		t.Errorf("%v", err)
	}
}

// awaitBGPEstablished waits for a BGP neighbor session to reach ESTABLISHED, returning an error on timeout.
func awaitBGPEstablished(t *testing.T, dut *ondatra.DUTDevice, niName, neighbor string) error {
	t.Helper()
	path := gnmi.OC().NetworkInstance(niName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Bgp().Neighbor(neighbor).SessionState().State()
	val, ok := gnmi.Watch(t, dut, path, bgpSessionTimeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := v.Val()
		return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		got, _ := val.Val()
		return fmt.Errorf("BGP session with %s did not reach ESTABLISHED within %v (last observed state: %v)", neighbor, bgpSessionTimeout, got)
	}
	t.Logf("BGP session with %s is ESTABLISHED", neighbor)
	return nil
}

// awaitBGPPrefixCount waits for a BGP neighbor to install the expected number
// of prefixes for the given AFI-SAFI. It returns an error if the operation
// times out.
func awaitBGPPrefixCount(t *testing.T, dut *ondatra.DUTDevice, niName, neighbor string, afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE, wantCount uint32) error {
	t.Helper()
	path := gnmi.OC().NetworkInstance(niName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Bgp().Neighbor(neighbor).AfiSafi(afiSafi).Prefixes().Installed().State()
	val, ok := gnmi.Watch(t, dut, path, bgpConvergenceWait, func(v *ygnmi.Value[uint32]) bool {
		got, present := v.Val()
		return present && got >= wantCount
	}).Await(t)
	if !ok {
		got, _ := val.Val()
		return fmt.Errorf("BGP neighbor %s only installed %d of %d wanted prefixes for %v within %v", neighbor, got, wantCount, afiSafi, bgpConvergenceWait)
	}
	t.Logf("BGP neighbor %s installed >= %d prefixes for %v", neighbor, wantCount, afiSafi)
	return nil
}

// DialGNMI returns a shared raw gNMI client, dialing it once and caching it
// for reuse. It is needed for raw paths that are not present in the OC tree.
func DialGNMI(t *testing.T, dut *ondatra.DUTDevice) (gpb.GNMIClient, error) {
	t.Helper()
	gnmiOnce.Do(func() {
		client, err := dut.RawAPIs().BindingDUT().DialGNMI(context.Background())
		if err != nil {
			gnmiErr = fmt.Errorf("failed to dial GNMI: %w", err)
			return
		}
		gnmiClient = client
	})
	return gnmiClient, gnmiErr
}

// globalFilterPolicyPath builds the raw gNMI path for an AFT global-filter
// policy leaf ("ipv4-policy" or "ipv6-policy") on the given network-instance.
// The afts/global-filter augment (openconfig-aft-global-filter.yang, added in
// openconfig/public >= 3.3.0) is not yet present in the ondatra `oc` bindings,
// so the path is constructed manually.
func globalFilterPolicyPath(niName, leaf string) *gpb.Path {
	return &gpb.Path{
		Origin: "openconfig",
		Elem: []*gpb.PathElem{
			{Name: "network-instances"},
			{Name: "network-instance", Key: map[string]string{"name": niName}},
			{Name: "afts"},
			{Name: "global-filter"},
			{Name: "config"},
			{Name: leaf},
		},
	}
}

// AFTDeleteGlobalFilter deletes both global-filter policy leaves for the given network-instance.
func AFTDeleteGlobalFilter(t *testing.T, dut *ondatra.DUTDevice, niName string) error {
	t.Helper()
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Log("Skipping AFT global-filter deletion: unsupported on EOS")
			return nil
		}
	}
	// The ondatra `oc` bindings do not yet contain the afts/global-filter
	// augment, so delete the IPv4/IPv6 filter policy leaves via raw gNMI paths.
	gnmiClient, err := DialGNMI(t, dut)
	if err != nil {
		return fmt.Errorf("failed to dial gNMI client: %w", err)
	}
	req := &gpb.SetRequest{
		Delete: []*gpb.Path{
			globalFilterPolicyPath(niName, "ipv4-policy"),
			globalFilterPolicyPath(niName, "ipv6-policy"),
		},
	}
	if _, err := gnmiClient.Set(context.Background(), req); err != nil {
		return fmt.Errorf("failed to delete AFT global-filter policies on %s: %w", dut.Vendor(), err)
	}
	return nil
}

// ConfigureToStoreRunningGNMIConfig configures the DUT to persist gNMI
// configuration changes to the running configuration, if supported.
func ConfigureToStoreRunningGNMIConfig(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	if dut.Vendor() == ondatra.ARISTA {
		dut.Config().New().WithAristaText(aristaPersistConfig).Append(t)
		t.Logf("Applied Arista config to persist gNMI running config: %q", aristaPersistConfig)
	}
	return nil
}

// UnconfigureToStoreRunningGNMIConfig removes the configuration that persists
// gNMI configuration changes to the running configuration, when supported.
func UnconfigureToStoreRunningGNMIConfig(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	if dut.Vendor() == ondatra.ARISTA {
		dut.Config().New().WithAristaText(aristaNoPersistConfig).Append(t)
		t.Logf("Applied Arista config to remove gNMI running config persistence: %q", aristaNoPersistConfig)
	}
	return nil
}

// GeneratePrefixesParams defines the parameters for generating a set of IPv4 and IPv6 prefixes.
type GeneratePrefixesParams struct {
	V4Prefixes []string
	V6Prefixes []string
	PfxCount   int
}

// GeneratePrefixes returns all generated IPv4 and IPv6 prefixes as a lookup map.
// Empty IPv4 or IPv6 prefix lists are ignored.
func GeneratePrefixes(t *testing.T, pfx GeneratePrefixesParams) map[string]bool {
	t.Helper()
	wantPrefixes := make(map[string]bool)
	for _, v4Prefix := range pfx.V4Prefixes {
		for prefix := range netutil.GenCIDRs(t, v4Prefix, pfx.PfxCount) {
			wantPrefixes[prefix] = true
		}
	}
	for _, v6Prefix := range pfx.V6Prefixes {
		for prefix := range netutil.GenCIDRs(t, v6Prefix, pfx.PfxCount) {
			wantPrefixes[prefix] = true
		}
	}
	return wantPrefixes
}

// PrefixSetPolicyParams has the parameters to configure prefix set policy.
type PrefixSetPolicyParams struct {
	PolicyName     string
	StatementNames []string
	PrefixSetNames []string
	MatchPrefixSet bool
	MatchSetOption oc.E_RoutingPolicy_MatchSetOptionsRestrictedType
	SetTag         bool
	PrefixList     []string
	PrefixMode     string
	PrefixDeny     bool
	PolicyResult   oc.E_RoutingPolicy_PolicyResultType
}

// AddPrefixSetPolicy creates a routing policy statement.
// It matches the specified prefix set and applies the supplied policy result.
func AddPrefixSetPolicy(t *testing.T, rp *oc.RoutingPolicy, cfg PrefixSetPolicyParams) {
	t.Helper()
	// Create prefix-set if prefixes are provided.
	if len(cfg.PrefixSetNames) > 0 && len(cfg.PrefixList) > 0 {
		ps := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(cfg.PrefixSetNames[0])
		for _, prefix := range cfg.PrefixList {
			addPrefix(t, ps, prefix, cfg.PrefixMode)
		}
	}
	pd := rp.GetOrCreatePolicyDefinition(cfg.PolicyName)
	for stIndex, stName := range cfg.StatementNames {
		stmt, err := pd.AppendNewStatement(stName)
		if err != nil {
			t.Fatalf("AppendNewStatement failed: %v", err)
		}
		if cfg.MatchPrefixSet {
			if cfg.SetTag {
				stmt.GetOrCreateConditions().
					GetOrCreateMatchTagSet().
					TagSet = ygot.String("999")
			} else if stIndex < len(cfg.PrefixSetNames) && cfg.PrefixSetNames[stIndex] != "" {
				match := stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet()
				match.PrefixSet = ygot.String(cfg.PrefixSetNames[stIndex])

				if cfg.MatchSetOption != 0 {
					match.MatchSetOptions = cfg.MatchSetOption
				}
			}
		}
		if cfg.PrefixDeny && stIndex == 0 {
			stmt.GetOrCreateActions().PolicyResult =
				oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
		} else {
			stmt.GetOrCreateActions().PolicyResult = cfg.PolicyResult
		}
	}
}

// addPrefix adds prefix-set entry.
func addPrefix(t *testing.T, ps *oc.RoutingPolicy_DefinedSets_PrefixSet, prefix, maskRange string) {
	t.Helper()
	p := ps.GetOrCreatePrefix(prefix, maskRange)

	p.IpPrefix = ygot.String(prefix)
	p.MasklengthRange = ygot.String(maskRange)
}

// VerifyGlobalFilterPoliciesCLI verifies that the configured global AFT filter policies are attached on the DUT.
// It uses the vendor CLI to validate the policy configuration.
func VerifyGlobalFilterPoliciesCLI(t *testing.T, dut *ondatra.DUTDevice, cfg ConfigureGlobalFilterPoliciesParams) {
	t.Helper()
	out, err := dut.RawAPIs().CLI(t).RunCommand(context.Background(), "show running-config all")
	if err != nil {
		t.Fatalf("show running-config all failed: %v", err)
	}
	output := out.Output()
	if cfg.V4Policy != "" && !strings.Contains(output, cfg.V4Policy) {
		t.Fatalf("IPv4 global filter policy %q not configured", cfg.V4Policy)
	}
	if cfg.V6Policy != "" && !strings.Contains(output, cfg.V6Policy) {
		t.Fatalf("IPv6 global filter policy %q not configured", cfg.V6Policy)
	}
}

// ConfigureGlobalFilterPoliciesParams contains the policy attachment parameters for configuring AFT global filter policies.
type ConfigureGlobalFilterPoliciesParams struct {
	V4Policy string
	V6Policy string
	VRFName  string
}

// ConfigureGlobalFilterPolicies configures AFT global-filter policies for the specified network-instance.
func ConfigureGlobalFilterPolicies(t *testing.T, dut *ondatra.DUTDevice, cfg ConfigureGlobalFilterPoliciesParams) {
	t.Helper()
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Log("Skipping AFT global-filter attachment: unsupported on EOS")
			return
		}
	}

	// The ondatra `oc` bindings do not yet contain the afts/global-filter
	// augment, so attach the IPv4/IPv6 filter policies via raw gNMI paths.
	var updates []*gpb.Update
	if cfg.V4Policy != "" {
		updates = append(updates, &gpb.Update{
			Path: globalFilterPolicyPath(cfg.VRFName, "ipv4-policy"),
			Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: []byte(fmt.Sprintf("%q", cfg.V4Policy))}},
		})
	}
	if cfg.V6Policy != "" {
		updates = append(updates, &gpb.Update{
			Path: globalFilterPolicyPath(cfg.VRFName, "ipv6-policy"),
			Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: []byte(fmt.Sprintf("%q", cfg.V6Policy))}},
		})
	}
	if len(updates) == 0 {
		return
	}
	gnmiClient, err := DialGNMI(t, dut)
	if err != nil {
		t.Fatalf("Failed to dial gNMI client: %v", err)
	}
	if _, err := gnmiClient.Set(context.Background(), &gpb.SetRequest{Update: updates}); err != nil {
		t.Fatalf("Failed to set AFT global-filter policies on %s: %v", dut.Vendor(), err)
	}
}

// NetworkInstanceStaticRouteParams holds the parameters required to
// configure static routes in the default and non-default network instances.
type NetworkInstanceStaticRouteParams struct {
	DefaultPrefixes     []string
	VRFPrefixes         []string
	DefaultNextHop      string
	VRFNextHop          string
	StartIndex          int
	VRFInstanceName     string
	DefaultInstanceName string
}

// ConfigureNetworkInstanceStaticRoute configures IPv4 and IPv6 static routes in the DEFAULT
// and Non-Default network instances using the specified next-hop addresses.
func ConfigureNetworkInstanceStaticRoute(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, cfg NetworkInstanceStaticRouteParams) {
	t.Helper()
	for idx, prefix := range cfg.DefaultPrefixes {
		cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: cfg.DefaultInstanceName, Prefix: prefix, Index: fmt.Sprintf("%d", idx+cfg.StartIndex), NextHop: cfg.DefaultNextHop})
	}
	// VRF-A IPv4 and IPv6 routes
	for idx, prefix := range cfg.VRFPrefixes {
		cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: cfg.VRFInstanceName, Prefix: prefix, Index: fmt.Sprintf("%d", idx+cfg.StartIndex+200), NextHop: cfg.VRFNextHop})
	}
	batch.Set(t, dut)
}

// AddStaticRouteParams holds the parameters for adding a static route.
type AddStaticRouteParams struct {
	NetworkInstanceName string
	Prefix              string
	Index               string
	NextHop             string
}

// AddSingleStaticRoute adds one static route.
func AddSingleStaticRoute(t *testing.T, dut *ondatra.DUTDevice, cfg AddStaticRouteParams) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: cfg.NetworkInstanceName, Prefix: cfg.Prefix, Index: cfg.Index, NextHop: cfg.NextHop})
	batch.Set(t, dut)
}

type CheckForDefaultRoutesParams struct {
	DUT             *ondatra.DUTDevice
	NetworkInstance string
	IPv4Route       string
	IPv6Route       string
}

// CheckForDefaultRoutes checks for the presence of IPv4 and IPv6 default routes
// in the DUT's static routing table.
func CheckForDefaultRoutes(t *testing.T, cfg *CheckForDefaultRoutesParams) {
	t.Helper()

	staticProtoName := deviations.StaticProtocolName(cfg.DUT)
	staticProto := gnmi.OC().
		NetworkInstance(cfg.NetworkInstance).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, staticProtoName)

	v4Present := gnmi.Lookup(t, cfg.DUT, staticProto.Static(cfg.IPv4Route).State()).IsPresent()
	v6Present := gnmi.Lookup(t, cfg.DUT, staticProto.Static(cfg.IPv6Route).State()).IsPresent()

	if v4Present && v6Present {
		t.Logf("AFT-6.2: IPv4 and IPv6 default routes are already present in network-instance %s's static routing table.",
			cfg.NetworkInstance)
		return
	}

	t.Logf("WARNING: Precondition failed: default route(s) missing from network-instance %s's static routing table "+
		"(IPv4 present: %v, IPv6 present: %v). Please preconfigure %s and %s as data-plane static routes. "+
		"Otherwise check for InitialSyncStoppingCondition will fail.",
		cfg.NetworkInstance, v4Present, v6Present, cfg.IPv4Route, cfg.IPv6Route)
}

// AddPrefixToPrefixSetParams defines parameters for adding a prefix to a prefix-set.
type AddPrefixToPrefixSetParams struct {
	PrefixSetName string
	Prefix        string
	MaskRange     string
}

// AddPrefixToPrefixSet adds a prefix entry to an existing routing policy prefix-set.
func AddPrefixToPrefixSet(t *testing.T, dut *ondatra.DUTDevice, cfg AddPrefixToPrefixSetParams) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	path := gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(cfg.PrefixSetName).Prefix(cfg.Prefix, cfg.MaskRange)
	entry := &oc.RoutingPolicy_DefinedSets_PrefixSet_Prefix{
		IpPrefix:        ygot.String(cfg.Prefix),
		MasklengthRange: ygot.String(cfg.MaskRange),
	}
	gnmi.BatchReplace(batch, path.Config(), entry)
	batch.Set(t, dut)
}
