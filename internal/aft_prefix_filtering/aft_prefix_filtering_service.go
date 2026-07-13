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
	AFTFilterSubscriptionWait     = 3 * time.Minute
	AFTFilterStaticRouteIndex     = 100
	AFTFilterPolicyMatchAll       = "POLICY-MATCH-ALL"
	AFTFilterDefaultStatementName = "10"
	AFTFilterPfxMode              = "exact"
	AFTFilterBulkV4BaseAddr       = "80.0.0.1"
	AFTFilterBulkV4RouteCount     = 1500000
	AFTFilterBulkV4PrefixLen      = 32
	AFTFilterBulkV6BaseAddr       = "3000::1"
	AFTFilterBulkV6RouteCount     = 500000
	AFTFilterBulkV6PrefixLen      = 128

	// Local constants for the AFT prefix-filtering service.
	aftFilterDUTAS              = 65001
	aftFilterATEAS              = 65002
	aftFilterBGPV4PeerGroup     = "BGP-BULK-V4-PEER-GROUP"
	aftFilterBGPV6PeerGroup     = "BGP-BULK-V6-PEER-GROUP"
	aftFilterBGPSessionTimeout  = 2 * time.Minute
	aftFilterBGPConvergenceWait = 10 * time.Minute
	aristaPersistConfig         = "management api gnmi\ntransport grpc default\noperation set persistence"
	aristaNoPersistConfig       = "management api gnmi\ntransport grpc default\nno operation set persistence"
)

var (
	// Package-level variables shared by the AFT prefix-filtering tests
	AFTFilterDUTPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port 1",
		MAC:     "02:00:02:02:02:02",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:1::1",
		IPv6Len: 64,
	}
	AFTFilterATEPort1 = attrs.Attributes{
		Name:    "atePort1",
		Desc:    "ATE to DUT Port 1",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:1::2",
		IPv6Len: 64,
	}
	AFTFilterDUTPort2 = attrs.Attributes{
		Desc:    "DUT to ATE Port 2",
		MAC:     "02:00:04:02:02:02",
		IPv4:    "192.0.3.1",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:2::1",
		IPv6Len: 64,
	}
	AFTFilterATEPort2 = attrs.Attributes{
		Name:    "atePort2",
		Desc:    "ATE to DUT Port 2",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.3.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:0:2::2",
		IPv6Len: 64,
	}

	// Local variables for the AFT prefix-filtering service
	debugNotifications  = flag.Bool("debug_notifications", true, "Enable full AFT notification recording")
	aftFilterGNMIOnce   sync.Once
	aftFilterGNMIClient gpb.GNMIClient
	aftFilterGNMIErr    error
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

// RunCollectorParams contains the parameters required to execute an AFT collector until the supplied stopping condition is satisfied.
type RunCollectorParams struct {
	Ctx       context.Context
	Collector *aftcache.AFTStreamSession
	Stop      aftcache.PeriodicHook
	Timeout   time.Duration
}

// RunCollector starts the AFT stream collector and blocks until the supplied stopping condition is satisfied or the collector times out.
func RunCollector(t *testing.T, cfg RunCollectorParams) {
	t.Helper()
	cfg.Collector.ListenUntil(cfg.Ctx, t, cfg.Timeout, cfg.Stop)
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

// NewCollector creates and returns a new AFT stream session. If debug_notifications is enabled, all received gNMI notifications are recorded in memory for later inspection and troubleshooting.
func NewCollector(t *testing.T, dut *ondatra.DUTDevice, cfg NewCollectorParams) *aftcache.AFTStreamSession {
	t.Helper()
	c := aftcache.NewAFTStreamSession(cfg.Context, t, cfg.Client, dut)
	if *debugNotifications {
		c.WithDebug()
		t.Log("DEBUG MODE ENABLED: Recording all gNMI notifications to memory.")
	}
	return c
}

// AFTFilterConfigureDUT configures the DUT with basic port1/port2 interfaces in the DEFAULT network instance.
func AFTFilterConfigureDUT(t *testing.T, dut *ondatra.DUTDevice) *gnmi.SetBatch {
	t.Helper()
	batch := &gnmi.SetBatch{}
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	aftFilterConfigureDUTInterface(t, dut, batch, &AFTFilterDUTPort1, p1)
	aftFilterConfigureDUTInterface(t, dut, batch, &AFTFilterDUTPort2, p2)
	batch.Set(t, dut)
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

// aftFilterConfigureDUTInterface configures an interface on the DUT.
func aftFilterConfigureDUTInterface(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, a *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	ocPath := gnmi.OC()
	i := a.NewOCInterface(p.Name(), dut)
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	i.GetOrCreateEthernet()
	i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	i4.Enabled = ygot.Bool(true)
	av4 := i4.GetOrCreateAddress(a.IPv4)
	av4.PrefixLength = ygot.Uint8(a.IPv4Len)
	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	i6.Enabled = ygot.Bool(true)
	av6 := i6.GetOrCreateAddress(a.IPv6)
	av6.PrefixLength = ygot.Uint8(a.IPv6Len)
	gnmi.BatchUpdate(batch, ocPath.Interface(p.Name()).Config(), i)
}

// AFTFilterConfigureATE configures the ATE ports and returns the topology along with the list of configured device (interface) names.
func AFTFilterConfigureATE(t *testing.T, ate *ondatra.ATEDevice) (gosnappi.Config, []string) {
	t.Helper()
	interfaceNamesList := []string{}
	topo := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	AFTFilterATEPort1.AddToOTG(topo, p1, &AFTFilterDUTPort1)
	AFTFilterATEPort2.AddToOTG(topo, p2, &AFTFilterDUTPort2)
	for _, dev := range topo.Devices().Items() {
		interfaceNamesList = append(interfaceNamesList, dev.Name())
	}
	return topo, interfaceNamesList
}

// AFTFilterConfigureBGP configures two eBGP neighbors in the DEFAULT network instance, one per port, carrying the bulk background routes (1.5M IPv4, 500k IPv6).
func AFTFilterConfigureBGP(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, ni *oc.NetworkInstance) {
	t.Helper()
	aftFilterConfigureBGPInstance(t, ni, AFTFilterDUTPort1.IPv4, []aftFilterBGPNeighborSpec{
		{address: AFTFilterATEPort1.IPv4, peerGroup: aftFilterBGPV4PeerGroup, v4: true},
		{address: AFTFilterATEPort2.IPv6, peerGroup: aftFilterBGPV6PeerGroup, v4: false},
	})
	gnmi.BatchUpdate(batch, gnmi.OC().NetworkInstance(ni.GetName()).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP"))
}

// AFTFilterApplyBGPMaxPrefixes applies the Arista max-prefixes deviation to the
// DEFAULT-instance eBGP neighbors.
func AFTFilterApplyBGPMaxPrefixes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if !deviations.BGPMissingOCMaxPrefixesConfiguration(dut) {
		return
	}
	for _, addr := range []string{AFTFilterATEPort1.IPv4, AFTFilterATEPort2.IPv6} {
		cfgplugins.DeviationAristaBGPNeighborMaxPrefixes(t, dut, addr, 0)
	}
}

// AFTFilterBGPParams parameterizes a per-network-instance eBGP peering that advertises scaled IPv4/IPv6 background routes over a single connected port.
type AFTFilterBGPParams struct {
	NetworkInstance *oc.NetworkInstance
	RouterID        string
	V4Neighbor      string
	V6Neighbor      string
}

// AFTFilterConfigureScaleBGP configures a dual-AFI eBGP peering in the given network instance over a single connected port, so that scaled IPv4 and IPv6 background routes can be advertised into either the default or a VRF network instance.
func AFTFilterConfigureScaleBGP(t *testing.T, dut *ondatra.DUTDevice, cfg AFTFilterBGPParams) {
	t.Helper()
	v4PeerGroup := fmt.Sprintf("%s-%s", aftFilterBGPV4PeerGroup, cfg.NetworkInstance.GetName())
	v6PeerGroup := fmt.Sprintf("%s-%s", aftFilterBGPV6PeerGroup, cfg.NetworkInstance.GetName())
	aftFilterConfigureBGPInstance(t, cfg.NetworkInstance, cfg.RouterID, []aftFilterBGPNeighborSpec{
		{address: cfg.V4Neighbor, peerGroup: v4PeerGroup, v4: true},
		{address: cfg.V6Neighbor, peerGroup: v6PeerGroup, v4: false},
	})
	if deviations.BGPMissingOCMaxPrefixesConfiguration(dut) {
		cfgplugins.DeviationAristaBGPNeighborMaxPrefixes(t, dut, cfg.V4Neighbor, 0)
		cfgplugins.DeviationAristaBGPNeighborMaxPrefixes(t, dut, cfg.V6Neighbor, 0)
	}
}

// aftFilterBGPNeighborSpec describes a single-AFI eBGP neighbor: when v4 is
// true the IPv4 unicast AFI is enabled (IPv6 unicast disabled), and vice versa.
type aftFilterBGPNeighborSpec struct {
	address   string
	peerGroup string
	v4        bool
}

// aftFilterConfigureBGPInstance configures a complete eBGP protocol instance in the given network instance with the supplied router-id and single-AFI neighbors, applying the accept-all import/export policy directly to each neighbor.
func aftFilterConfigureBGPInstance(t *testing.T, ni *oc.NetworkInstance, routerID string, neighbors []aftFilterBGPNeighborSpec) {
	t.Helper()
	if ni == nil {
		t.Fatalf("Network Instance is not configured")
	}
	niProto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	niProto.Enabled = ygot.Bool(true)
	bgp := niProto.GetOrCreateBgp()
	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(aftFilterDUTAS)
	g.RouterId = ygot.String(routerID)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	for _, n := range neighbors {
		nb := bgp.GetOrCreateNeighbor(n.address)
		nb.PeerAs = ygot.Uint32(aftFilterATEAS)
		nb.Enabled = ygot.Bool(true)
		nb.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(n.v4)
		nb.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(!n.v4)
		ap := nb.GetOrCreateApplyPolicy()
		ap.ImportPolicy = []string{AFTFilterPolicyMatchAll}
		ap.ExportPolicy = []string{AFTFilterPolicyMatchAll}
	}
}

// AFTFilterConfigureATEBGP attaches BGPv4 (port1) and BGPv6 (port2) peers advertising the bulk IPv4/IPv6 background route ranges to the ATE topology.
func AFTFilterConfigureATEBGP(t *testing.T, topo gosnappi.Config) {
	t.Helper()
	dev1 := topo.Devices().Items()[0]
	ipv4Name1 := fmt.Sprintf("%s.IPv4", AFTFilterATEPort1.Name)
	bgp1 := dev1.Bgp().SetRouterId(AFTFilterATEPort1.IPv4)
	peer1 := bgp1.Ipv4Interfaces().Add().SetIpv4Name(ipv4Name1).Peers().Add().SetName("atePort1.BGP4.peer")
	peer1.SetPeerAddress(AFTFilterDUTPort1.IPv4).SetAsNumber(aftFilterATEAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	v4Routes := peer1.V4Routes().Add().SetName("bulk-ipv4-routes")
	v4Routes.SetNextHopIpv4Address(AFTFilterATEPort1.IPv4).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	v4Routes.Addresses().Add().SetAddress(AFTFilterBulkV4BaseAddr).SetPrefix(AFTFilterBulkV4PrefixLen).SetCount(AFTFilterBulkV4RouteCount).SetStep(1)
	dev2 := topo.Devices().Items()[1]
	ipv6Name2 := fmt.Sprintf("%s.IPv6", AFTFilterATEPort2.Name)
	bgp2 := dev2.Bgp().SetRouterId(AFTFilterATEPort2.IPv4)
	peer2 := bgp2.Ipv6Interfaces().Add().SetIpv6Name(ipv6Name2).Peers().Add().SetName("atePort2.BGP6.peer")
	peer2.SetPeerAddress(AFTFilterDUTPort2.IPv6).SetAsNumber(aftFilterATEAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	v6Routes := peer2.V6Routes().Add().SetName("bulk-ipv6-routes")
	v6Routes.SetNextHopIpv6Address(AFTFilterATEPort2.IPv6).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	v6Routes.Addresses().Add().SetAddress(AFTFilterBulkV6BaseAddr).SetPrefix(AFTFilterBulkV6PrefixLen).SetCount(AFTFilterBulkV6RouteCount).SetStep(1)
}

// AFTFilterATEBGPParams parameterizes ATE-side eBGP peers advertising scaled
// IPv4/IPv6 background routes over a single connected port.
type AFTFilterATEBGPParams struct {
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

// AFTFilterConfigureATEScaleBGP attaches dual-AFI eBGP peers to the supplied
// ATE device, advertising the parameterized IPv4/IPv6 background route ranges.
func AFTFilterConfigureATEScaleBGP(t *testing.T, dev gosnappi.Device, cfg AFTFilterATEBGPParams) {
	t.Helper()
	bgp := dev.Bgp().SetRouterId(cfg.ATEPort.IPv4)
	ipv4Name := fmt.Sprintf("%s.IPv4", cfg.ATEPort.Name)
	peer4 := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4Name).Peers().Add().SetName(fmt.Sprintf("%s.BGP4.peer", cfg.NamePrefix))
	peer4.SetPeerAddress(cfg.DUTPort.IPv4).SetAsNumber(aftFilterATEAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	v4Routes := peer4.V4Routes().Add().SetName(fmt.Sprintf("%s-ipv4-routes", cfg.NamePrefix))
	v4Routes.SetNextHopIpv4Address(cfg.ATEPort.IPv4).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	v4Routes.Addresses().Add().SetAddress(cfg.V4BaseAddr).SetPrefix(cfg.V4PrefixLen).SetCount(cfg.V4RouteCount).SetStep(1)
	ipv6Name := fmt.Sprintf("%s.IPv6", cfg.ATEPort.Name)
	peer6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6Name).Peers().Add().SetName(fmt.Sprintf("%s.BGP6.peer", cfg.NamePrefix))
	peer6.SetPeerAddress(cfg.DUTPort.IPv6).SetAsNumber(aftFilterATEAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	v6Routes := peer6.V6Routes().Add().SetName(fmt.Sprintf("%s-ipv6-routes", cfg.NamePrefix))
	v6Routes.SetNextHopIpv6Address(cfg.ATEPort.IPv6).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	v6Routes.Addresses().Add().SetAddress(cfg.V6BaseAddr).SetPrefix(cfg.V6PrefixLen).SetCount(cfg.V6RouteCount).SetStep(1)
}

// AFTFilterConfigureBaseRoutesParams defines the parameters for configuring the baseline static routes.
type AFTFilterConfigureBaseRoutesParams struct {
	V4Prefixes []string
	V6Prefixes []string
}

// AFTFilterConfigureBaseRoutes installs the baseline static routes into the DEFAULT network instance. IPv4 prefixes use the ATE port1 IPv4 next-hop and IPv6 prefixes use the ATE port1 IPv6 next-hop.
func AFTFilterConfigureBaseRoutes(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, p AFTFilterConfigureBaseRoutesParams) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	for idx, prefix := range p.V4Prefixes {
		cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: ni, Prefix: prefix, Index: fmt.Sprintf("%d", AFTFilterStaticRouteIndex+idx), NextHop: AFTFilterATEPort1.IPv4})
	}
	for idx, prefix := range p.V6Prefixes {
		cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{NetworkInstance: ni, Prefix: prefix, Index: fmt.Sprintf("%d", AFTFilterStaticRouteIndex+100+idx), NextHop: AFTFilterATEPort1.IPv6})
	}
	batch.Set(t, dut)
}

// AFTFilterAwaitBGPConvergence waits for both eBGP sessions to establish and for the expected bulk prefix counts to install, recording failures with t.Errorf so the caller can continue.
func AFTFilterAwaitBGPConvergence(t *testing.T, dut *ondatra.DUTDevice, niName string) {
	t.Helper()
	AFTFilterAwaitScaleBGPConvergence(t, dut, AFTFilterBGPConvergenceParams{
		NetworkInstance: niName,
		V4Neighbor:      AFTFilterATEPort1.IPv4,
		V6Neighbor:      AFTFilterATEPort2.IPv6,
		V4RouteCount:    AFTFilterBulkV4RouteCount,
		V6RouteCount:    AFTFilterBulkV6RouteCount,
	})
}

// AFTFilterBGPConvergenceParams parameterizes the BGP convergence check for a
// dual-AFI peering advertising scaled routes over a single connected port.
type AFTFilterBGPConvergenceParams struct {
	NetworkInstance string
	V4Neighbor      string
	V6Neighbor      string
	V4RouteCount    uint32
	V6RouteCount    uint32
}

// AFTFilterAwaitScaleBGPConvergence waits for both eBGP sessions to establish and for the expected IPv4/IPv6 prefix counts to install, recording failures with t.Errorf so the caller can continue.
func AFTFilterAwaitScaleBGPConvergence(t *testing.T, dut *ondatra.DUTDevice, cfg AFTFilterBGPConvergenceParams) {
	t.Helper()
	if err := aftFilterAwaitBGPEstablished(t, dut, cfg.NetworkInstance, cfg.V4Neighbor); err != nil {
		t.Errorf("%v", err)
	}
	if err := aftFilterAwaitBGPEstablished(t, dut, cfg.NetworkInstance, cfg.V6Neighbor); err != nil {
		t.Errorf("%v", err)
	}
	if err := aftFilterAwaitBGPPrefixCount(t, dut, cfg.NetworkInstance, cfg.V4Neighbor, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, cfg.V4RouteCount); err != nil {
		t.Errorf("%v", err)
	}
	if err := aftFilterAwaitBGPPrefixCount(t, dut, cfg.NetworkInstance, cfg.V6Neighbor, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, cfg.V6RouteCount); err != nil {
		t.Errorf("%v", err)
	}
}

// aftFilterAwaitBGPEstablished waits for a BGP neighbor session to reach ESTABLISHED, returning an error on timeout.
func aftFilterAwaitBGPEstablished(t *testing.T, dut *ondatra.DUTDevice, niName, neighbor string) error {
	t.Helper()
	path := gnmi.OC().NetworkInstance(niName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(neighbor).SessionState().State()
	val, ok := gnmi.Watch(t, dut, path, aftFilterBGPSessionTimeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := v.Val()
		return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		got, _ := val.Val()
		return fmt.Errorf("BGP session with %s did not reach ESTABLISHED within %v (last observed state: %v)", neighbor, aftFilterBGPSessionTimeout, got)
	}
	t.Logf("BGP session with %s is ESTABLISHED", neighbor)
	return nil
}

// aftFilterAwaitBGPPrefixCount waits for a BGP neighbor to install the expected number of prefixes for the given AFI-SAFI, returning an error on timeout.
func aftFilterAwaitBGPPrefixCount(t *testing.T, dut *ondatra.DUTDevice, niName, neighbor string, afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE, wantCount uint32) error {
	t.Helper()
	path := gnmi.OC().NetworkInstance(niName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(neighbor).AfiSafi(afiSafi).Prefixes().Installed().State()
	val, ok := gnmi.Watch(t, dut, path, aftFilterBGPConvergenceWait, func(v *ygnmi.Value[uint32]) bool {
		got, present := v.Val()
		return present && got >= wantCount
	}).Await(t)
	if !ok {
		got, _ := val.Val()
		return fmt.Errorf("BGP neighbor %s only installed %d of %d wanted prefixes for %v within %v", neighbor, got, wantCount, afiSafi, aftFilterBGPConvergenceWait)
	}
	t.Logf("BGP neighbor %s installed >= %d prefixes for %v", neighbor, wantCount, afiSafi)
	return nil
}

// AFTFilterDialGNMI returns a shared raw gNMI client, dialing it once and caching it for reuse. Needed for raw paths absent from the OC tree.
func AFTFilterDialGNMI(t *testing.T, dut *ondatra.DUTDevice) (gpb.GNMIClient, error) {
	t.Helper()
	aftFilterGNMIOnce.Do(func() {
		client, err := dut.RawAPIs().BindingDUT().DialGNMI(context.Background())
		if err != nil {
			aftFilterGNMIErr = fmt.Errorf("failed to dial GNMI: %w", err)
			return
		}
		aftFilterGNMIClient = client
	})
	return aftFilterGNMIClient, aftFilterGNMIErr
}

// AFTFilterDeleteGlobalFilter deletes both global-filter policy leaves for the given network-instance.
func AFTFilterDeleteGlobalFilter(t *testing.T, dut *ondatra.DUTDevice, niName string) error {
	t.Helper()
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Log("Skipping AFT global-filter deletion: unsupported on EOS")
			return nil
		}
	}
	// For vendors that support the OpenConfig afts/global-filter augment
	// (openconfig-aft-global-filter.yang, added in openconfig/public models
	// 3.3.0), delete the IPv4/IPv6 filter policy leaves through the typed OC
	// path API. The generated ondatra `oc` bindings do not yet contain the
	// GlobalFilter container, so the calls below are commented out; uncomment
	// them once the bindings are regenerated against openconfig/public >= 3.3.0.
	// batch := &gnmi.SetBatch{}
	// gnmi.BatchDelete(batch, gnmi.OC().NetworkInstance(niName).Afts().GlobalFilter().Ipv4Policy().Config())
	// gnmi.BatchDelete(batch, gnmi.OC().NetworkInstance(niName).Afts().GlobalFilter().Ipv6Policy().Config())
	// batch.Set(t, dut)
	// return nil
	return fmt.Errorf("AFT global filter deletion is expected to be supported on %s, but no OpenConfig implementation is available", dut.Vendor())
}

// ConfigureToStoreRunningGNMIConfig configures the DUT to persist gNMI configuration changes to the running configuration, if supported.
func ConfigureToStoreRunningGNMIConfig(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	if dut.Vendor() == ondatra.ARISTA {
		dut.Config().New().WithAristaText(aristaPersistConfig).Append(t)
		t.Logf("Applied Arista config to persist gNMI running config: %q", aristaPersistConfig)
	}
	return nil
}

// UnconfigureToStoreRunningGNMIConfig removes the configuration that persists gNMI configuration changes to the running configuration, if supported.
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
