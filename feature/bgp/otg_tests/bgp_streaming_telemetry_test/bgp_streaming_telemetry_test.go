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

// Package bgp_streaming_telemetry_test implements RT-1.109: BGP Streaming
// Telemetry and Operational State Verification.
package bgp_streaming_telemetry_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstbgp"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	vrfName               = "ce1"
	dutAS                 = 64496
	ceAS                  = 64497
	rrAS                  = dutAS
	plenIPv4              = 30
	plenIPv6              = 126
	bgpName               = "BGP"
	allowPolicy           = "ALLOW"
	exportPolicy          = "EXPORT-TO-CE"
	exportPfxSet          = "DUT-ORIGINATED"
	maxPrefixes           = 5000
	warnPct               = 80
	stateLatency          = 10 * time.Second
	sampleInterval        = 5 * time.Second
	convergeWait          = 3 * time.Minute
	dutStaticPfx          = "203.0.113.0"
	dutStaticCount        = 10
	baseV4Routes          = 2500
	addV4Routes1          = 1000
	addV4Routes2          = 750
	addV4Routes3          = 800
	baseV6Routes          = 25
	routeV4Base           = "ceV4Base"
	routeV4Add1           = "ceV4Add1"
	routeV4Add2           = "ceV4Add2"
	routeV4Add3           = "ceV4Add3"
	routeV6Base           = "ceV6Base"
	ceV4PeerName          = "ateCE.BGP4.peer"
	ceV6PeerName          = "ateCE.BGP6.peer"
	rrV4PeerName          = "ateRR.BGP4.peer"
	v6RoutePrefix         = "2001:db8:100::"
	dutStaticPrefixFormat = "203.0.113.%d/32"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE CE (VRF ce1)",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1 = attrs.Attributes{
		Name:    "ateCE",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE RR (core)",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort2 = attrs.Attributes{
		Name:    "ateRR",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	// v4 route blocks inside 198.18.0.0/15, non-overlapping.
	v4RouteBlocks = []struct {
		name  string
		start string
		count uint32
	}{
		{routeV4Base, "198.18.0.0", baseV4Routes},
		{routeV4Add1, "198.18.16.0", addV4Routes1},
		{routeV4Add2, "198.18.32.0", addV4Routes2},
		{routeV4Add3, "198.18.48.0", addV4Routes3},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// --------------------------- DUT configuration ---------------------------

// bgpPath returns the OpenConfig BGP path under a network instance.
func bgpPath(ni string) *netinstbgp.NetworkInstance_Protocol_BgpPath {
	return gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp()
}

// configureDUT programs interfaces, VRF, policies, and BGP on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()
	batch := &gnmi.SetBatch{}
	p1 := dut.Port(t, "port1").Name()
	p2 := dut.Port(t, "port2").Name()
	defNI := &cfgplugins.NetworkInstanceParams{Default: true}
	ceNI := &cfgplugins.NetworkInstanceParams{Name: vrfName, Default: false}
	// Configure network-instances first.
	cfgplugins.NewNetworkInstance(t, dut, batch, defNI)
	cfgplugins.NewNetworkInstance(t, dut, batch, ceNI)
	// Configure VRF binding before interface addressing on platforms that require it.
	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		cfgplugins.AssignInterfaceToNetworkInstance(t, batch, dut, p1, ceNI, 0)
	}
	i1 := dutPort1.NewOCInterface(p1, dut)
	i2 := dutPort2.NewOCInterface(p2, dut)
	gnmi.BatchReplace(batch, d.Interface(p1).Config(), i1)
	gnmi.BatchReplace(batch, d.Interface(p2).Config(), i2)
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		cfgplugins.AssignInterfaceToNetworkInstance(t, batch, dut, p2, defNI, 0)
	}
	if !deviations.InterfaceConfigVRFBeforeAddress(dut) {
		cfgplugins.AssignInterfaceToNetworkInstance(t, batch, dut, p1, ceNI, 0)
	}
	configureRoutingPolicy(t, dut, batch)
	configureStaticRoutes(t, dut, batch)
	// Default VRF: global BGP + iBGP to core RR.
	defaultName := deviations.DefaultNetworkInstance(dut)
	gnmi.BatchReplace(batch, d.NetworkInstance(defaultName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Config(), buildCoreBGP(dut))
	// VRF ce1: eBGP to CE.
	gnmi.BatchReplace(batch, d.NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Config(), buildVRFBGP(dut))
	configureStaticRedistribution(t, dut, batch)
	batch.Set(t, dut)
	// Configure max-prefix after base OC config is committed so vendor CLI
	// fallback paths can reference an already-created VRF/BGP context.
	if maxPrefixBatch := cfgplugins.ConfigureBGPMaxPrefix(t, dut, &gnmi.SetBatch{}, cfgplugins.BGPMaxPrefixConfig{
		DUTAS:               dutAS,
		VRFName:             vrfName,
		NeighborAddress:     atePort1.IPv4,
		MaxPrefixes:         maxPrefixes,
		WarningThresholdPct: warnPct,
	}); maxPrefixBatch != nil {
		maxPrefixBatch.Set(t, dut)
	}
}

// configureRoutingPolicy installs routing policies used by this test.
func configureRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdAllow := rp.GetOrCreatePolicyDefinition(allowPolicy)
	st, err := pdAllow.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	ps := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(exportPfxSet)
	ps.Mode = oc.PrefixSet_Mode_IPV4
	ps.GetOrCreatePrefix(dutStaticPfx+"/24", "24..32")
	pdExp := rp.GetOrCreatePolicyDefinition(exportPolicy)
	st1, err := pdExp.AppendNewStatement("match-dut-originated")
	if err != nil {
		t.Fatal(err)
	}
	st1.GetOrCreateConditions().GetOrCreateMatchPrefixSet().PrefixSet = ygot.String(exportPfxSet)
	st1.GetOrCreateConditions().GetOrCreateMatchPrefixSet().MatchSetOptions = oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY
	st1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	st2, err := pdExp.AppendNewStatement("reject-rest")
	if err != nil {
		t.Fatal(err)
	}
	st2.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)
}

// configureStaticRoutes installs static routes that are exported to the CE peer.
func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(vrfName)
	sp := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	for i := 1; i <= dutStaticCount; i++ {
		pfx := fmt.Sprintf(dutStaticPrefixFormat, i)
		sr := sp.GetOrCreateStatic(pfx)
		nh := sr.GetOrCreateNextHop("0")
		nh.NextHop = oc.LocalRouting_LOCAL_DEFINED_NEXT_HOP_DROP
	}
	gnmi.BatchUpdate(batch, gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), sp)
}

// configureStaticRedistribution enables static-to-BGP redistribution in VRF ce1.
func configureStaticRedistribution(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(vrfName)
	tc := ni.GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.Types_ADDRESS_FAMILY_IPV4)
	tc.ImportPolicy = []string{allowPolicy}
	if !deviations.SkipSettingDisableMetricPropagation(dut) {
		tc.DisableMetricPropagation = ygot.Bool(false)
	}
	gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(vrfName).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.Types_ADDRESS_FAMILY_IPV4).Config(), tc)
}

// applyPolicy applies import/export policy either at neighbor or AFI-SAFI level.
func applyPolicy(dut *ondatra.DUTDevice, nbr *oc.NetworkInstance_Protocol_Bgp_Neighbor, afi oc.E_BgpTypes_AFI_SAFI_TYPE, imp, exp string) {
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		ap := nbr.GetOrCreateApplyPolicy()
		ap.ImportPolicy = []string{imp}
		ap.ExportPolicy = []string{exp}
		return
	}
	ap := nbr.GetOrCreateAfiSafi(afi).GetOrCreateApplyPolicy()
	ap.ImportPolicy = []string{imp}
	ap.ExportPolicy = []string{exp}
}

// buildCoreBGP returns the default-VRF BGP with the iBGP core (RR) peer.
func buildCoreBGP(dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	root := &oc.Root{}
	proto := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
	bgp := proto.GetOrCreateBgp()
	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAS)
	g.RouterId = ygot.String(dutPort2.IPv4)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	n := bgp.GetOrCreateNeighbor(atePort2.IPv4)
	n.PeerAs = ygot.Uint32(rrAS)
	n.Enabled = ygot.Bool(true)
	n.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	applyPolicy(dut, n, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, allowPolicy, allowPolicy)
	return proto
}

// buildVRFBGP returns VRF ce1 BGP with the eBGP CE peers (IPv4 and IPv6).
func buildVRFBGP(dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	root := &oc.Root{}
	proto := root.GetOrCreateNetworkInstance(vrfName).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
	bgp := proto.GetOrCreateBgp()
	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAS)
	g.RouterId = ygot.String(dutPort1.IPv4)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	n4 := bgp.GetOrCreateNeighbor(atePort1.IPv4)
	n4.PeerAs = ygot.Uint32(ceAS)
	n4.Enabled = ygot.Bool(true)
	af4 := n4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	af4.Enabled = ygot.Bool(true)
	n4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(false)
	applyPolicy(dut, n4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, allowPolicy, exportPolicy)
	n6 := bgp.GetOrCreateNeighbor(atePort1.IPv6)
	n6.PeerAs = ygot.Uint32(ceAS)
	n6.Enabled = ygot.Bool(true)
	n6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	n6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(false)
	applyPolicy(dut, n6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, allowPolicy, allowPolicy)
	return proto
}

// clearCENeighborOnDUT performs a vendor CLI clear for the CE IPv4 BGP
// neighbor in VRF ce1, used after a max-prefix teardown.
func clearCENeighborOnDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var cli string
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli = fmt.Sprintf("clear ip bgp vrf %s reason %s\n", vrfName, "Shut down communication reset message")
	case ondatra.CISCO:
		cli = fmt.Sprintf("clear ip bgp vrf %s %s\n", vrfName, atePort1.IPv4)
	default:
		t.Logf("Skipping explicit DUT BGP clear for unsupported vendor: %v", dut.Vendor())
		return
	}
	helpers.GnmiCLIConfig(t, dut, cli)
}

// configureATE builds and pushes OTG topology, interfaces, peers, and routes.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	p1 := top.Ports().Add().SetName("port1")
	ce := top.Devices().Add().SetName(atePort1.Name)
	ceEth := ce.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	ceEth.Connection().SetPortName(p1.Name())
	ceV4 := ceEth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	ceV4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	ceV6 := ceEth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	ceV6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))
	ceBgp := ce.Bgp().SetRouterId(atePort1.IPv4)
	ce4 := ceBgp.Ipv4Interfaces().Add().SetIpv4Name(ceV4.Name()).Peers().Add().SetName(ceV4PeerName)
	ce4.SetPeerAddress(dutPort1.IPv4).SetAsNumber(ceAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	ce6 := ceBgp.Ipv6Interfaces().Add().SetIpv6Name(ceV6.Name()).Peers().Add().SetName(ceV6PeerName)
	ce6.SetPeerAddress(dutPort1.IPv6).SetAsNumber(ceAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	for _, b := range v4RouteBlocks {
		r := ce4.V4Routes().Add().SetName(b.name)
		r.SetNextHopIpv4Address(atePort1.IPv4).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		r.Addresses().Add().SetAddress(b.start).SetPrefix(32).SetCount(b.count)
	}
	r6 := ce6.V6Routes().Add().SetName(routeV6Base)
	r6.SetNextHopIpv6Address(atePort1.IPv6).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	r6.Addresses().Add().SetAddress(v6RoutePrefix).SetPrefix(64).SetCount(baseV6Routes)
	p2 := top.Ports().Add().SetName("port2")
	rr := top.Devices().Add().SetName(atePort2.Name)
	rrEth := rr.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	rrEth.Connection().SetPortName(p2.Name())
	rrV4 := rrEth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	rrV4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	rrBgp := rr.Bgp().SetRouterId(atePort2.IPv4)
	rrBgp.Ipv4Interfaces().Add().SetIpv4Name(rrV4.Name()).Peers().Add().SetName(rrV4PeerName).SetPeerAddress(dutPort2.IPv4).SetAsNumber(rrAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	ate.OTG().PushConfig(t, top)
	return top
}

// setRouteState advertises or withdraws the specified route ranges on the ATE.
func setRouteState(t *testing.T, ate *ondatra.ATEDevice, names []string, st gosnappi.StateProtocolRouteStateEnum) {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(names).SetState(st)
	ate.OTG().SetControlState(t, cs)
}

// setCENeighborEnabled administratively enables or disables the CE BGP neighbor on the DUT.
func setCENeighborEnabled(t *testing.T, dut *ondatra.DUTDevice, enabled bool) {
	t.Helper()
	gnmi.Update(t, dut, ceNbr(dut).Enabled().Config(), enabled)
}

// setPeerState administratively changes state of named BGP peers on the ATE.
func setPeerState(t *testing.T, ate *ondatra.ATEDevice, names []string, st gosnappi.StateProtocolBgpPeersStateEnum) {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Protocol().Bgp().Peers().SetPeerNames(names).SetState(st)
	ate.OTG().SetControlState(t, cs)
}

// onChangeSubscriptionOpts returns DUT options for a gNMI STREAM / ON_CHANGE subscription.
func onChangeSubscriptionOpts(dut *ondatra.DUTDevice) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_ON_CHANGE))
}

// sampleSubscriptionOpts returns DUT options for a gNMI STREAM / SAMPLE subscription.
func sampleSubscriptionOpts(dut *ondatra.DUTDevice) *gnmi.Opts {
	return dut.GNMIOpts().WithYGNMIOpts(
		ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE),
		ygnmi.WithSampleInterval(sampleInterval),
	)
}

// ceNbr returns the CE IPv4 neighbor path in VRF ce1.
func ceNbr(dut *ondatra.DUTDevice) *netinstbgp.NetworkInstance_Protocol_Bgp_NeighborPath {
	return bgpPath(vrfName).Neighbor(atePort1.IPv4)
}

// awaitSessionState waits for a streamed session-state matching any wanted value.
func awaitSessionState(t *testing.T, dut *ondatra.DUTDevice, nbr *netinstbgp.NetworkInstance_Protocol_Bgp_NeighborPath, timeout time.Duration, want ...oc.E_Bgp_Neighbor_SessionState) (oc.E_Bgp_Neighbor_SessionState, time.Duration, bool) {
	t.Helper()
	start := time.Now()
	val, ok := gnmi.Watch(t, onChangeSubscriptionOpts(dut), nbr.SessionState().State(), timeout, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		s, present := v.Val()
		if !present {
			return false
		}
		for _, w := range want {
			if s == w {
				return true
			}
		}
		return false
	}).Await(t)
	got, _ := val.Val()
	return got, time.Since(start), ok
}

// awaitPrefixCount waits for a sampled prefix counter to reach an expected value.
func awaitPrefixCount(t *testing.T, dut *ondatra.DUTDevice, q ygnmi.SingletonQuery[uint32], want uint32, timeout time.Duration) (uint32, bool) {
	t.Helper()
	val, ok := gnmi.Watch(t, sampleSubscriptionOpts(dut), q, timeout, func(v *ygnmi.Value[uint32]) bool {
		c, present := v.Val()
		return present && c == want
	}).Await(t)
	got, _ := val.Val()
	return got, ok
}

// verifyV4Prefixes validates received/installed/sent IPv4 prefix telemetry.
func verifyV4Prefixes(t *testing.T, dut *ondatra.DUTDevice, wantRx, wantInst uint32, wantSent *uint32) {
	t.Helper()
	valid := true
	pfx := ceNbr(dut).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	if got, ok := awaitPrefixCount(t, dut, pfx.Received().State(), wantRx, convergeWait); !ok {
		t.Errorf("prefixes/received: got %d, want %d", got, wantRx)
		valid = false
	}
	if got, ok := awaitPrefixCount(t, dut, pfx.Installed().State(), wantInst, convergeWait); !ok {
		t.Errorf("prefixes/installed: got %d, want %d", got, wantInst)
		valid = false
	}
	if wantSent != nil {
		if got, ok := awaitPrefixCount(t, dut, pfx.Sent().State(), *wantSent, convergeWait); !ok {
			t.Errorf("prefixes/sent: got %d, want %d", got, *wantSent)
			valid = false
		}
	}
	if !valid {
		return
	}
	sent := "not checked"
	if wantSent != nil {
		sent = fmt.Sprintf("%d", *wantSent)
	}
	t.Logf("IPv4 prefix validation completed successfully (received=%d, installed=%d, sent=%s)", wantRx, wantInst, sent)
}

// prefixLimitExceeded returns the proper prefix-limit-exceeded query for DUT deviations.
func prefixLimitExceeded(dut *ondatra.DUTDevice) ygnmi.SingletonQuery[bool] {
	afi := ceNbr(dut).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast()
	if deviations.BGPExplicitPrefixLimitReceived(dut) {
		return afi.PrefixLimitReceived().PrefixLimitExceeded().State()
	}
	return afi.PrefixLimit().PrefixLimitExceeded().State()
}

// awaitPrefixLimitExceededPresence verifies the prefix-limit-exceeded leaf is
// streamed and returns the streamed value.
func awaitPrefixLimitExceededPresence(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration) (bool, bool) {
	t.Helper()
	val, ok := gnmi.Watch(t, sampleSubscriptionOpts(dut), prefixLimitExceeded(dut), timeout, func(v *ygnmi.Value[bool]) bool {
		_, present := v.Val()
		return present
	}).Await(t)
	got, present := val.Val()
	return got, ok && present
}

// TestBGPStreamingTelemetry validates session and prefix streaming telemetry behavior.
func TestBGPStreamingTelemetry(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	t.Cleanup(func() {
		gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(allowPolicy).Config())
		gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(exportPolicy).Config())
		gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(exportPfxSet).Config())
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(vrfName).Config())
		defaultNI := deviations.DefaultNetworkInstance(dut)
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Config())
	})
	t.Log("Configuring DUT")
	configureDUT(t, dut)
	t.Log("Configuring ATE")
	top := configureATE(t, ate)
	// Only the baseline route ranges are advertised at start.
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
	setRouteState(t, ate, []string{routeV4Add1, routeV4Add2, routeV4Add3}, gosnappi.StateProtocolRouteState.WITHDRAW)
	nbr := ceNbr(dut)
	// Recover to a clean baseline to avoid stale max-prefix hold-down inherited
	// from prior runs.
	recoverCEBaselineSession(t, dut, ate, nbr)
	t.Run("RT-1.109.1 Session operational state and transition counters", func(t *testing.T) {
		if _, _, ok := awaitSessionState(t, dut, nbr, convergeWait, oc.Bgp_Neighbor_SessionState_ESTABLISHED); !ok {
			fptest.LogQuery(t, "CE neighbor", nbr.State(), gnmi.Get(t, dut, nbr.State()))
			t.Fatalf("Initial session-state for %s is not ESTABLISHED", atePort1.IPv4)
		}
		transBefore := gnmi.Get(t, dut, nbr.EstablishedTransitions().State())
		lastEstBefore := gnmi.Get(t, dut, nbr.LastEstablished().State())
		t.Logf("Before flap: established-transitions=%d last-established=%d", transBefore, lastEstBefore)
		t.Log("Starting ON_CHANGE watch for BGP session-state transition")
		// Disabling the local neighbor administratively drives the DUT FSM to
		// IDLE. A remote ATE peer shutdown can leave the DUT in CONNECT/ACTIVE
		// while it attempts to reconnect, as seen on the target device.
		t.Cleanup(func() {
			setCENeighborEnabled(t, dut, true)
			if _, _, ok := awaitSessionState(t, dut, nbr, convergeWait, oc.Bgp_Neighbor_SessionState_ESTABLISHED); !ok {
				fptest.LogQuery(t, "CE neighbor after cleanup", nbr.State(), gnmi.Get(t, dut, nbr.State()))
				t.Errorf("CE BGP neighbor did not recover to ESTABLISHED during cleanup")
			}
		})
		// Start STREAM/ON_CHANGE before disabling the DUT neighbor.
		sessionStateWatch := gnmi.Watch(t, onChangeSubscriptionOpts(dut),
			nbr.SessionState().State(), convergeWait,
			func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
				state, present := v.Val()
				return present && state == oc.Bgp_Neighbor_SessionState_IDLE
			},
		)
		t.Log("Administratively disabling the DUT CE BGP neighbor")
		shutdownStart := time.Now()
		setCENeighborEnabled(t, dut, false)
		// Await an ON_CHANGE update showing that the session is no longer up.
		stateValue, ok := sessionStateWatch.Await(t)
		elapsed := time.Since(shutdownStart)
		if !ok {
			fptest.LogQuery(t, "CE neighbor", nbr.State(), gnmi.Get(t, dut, nbr.State()))
			t.Fatalf("session-state did not transition to IDLE after disabling the DUT neighbor")
		}
		got, present := stateValue.Val()
		if !present {
			t.Fatalf("session-state value is not present after disabling the DUT neighbor")
		}
		if got == oc.Bgp_Neighbor_SessionState_ESTABLISHED {
			t.Fatalf("session-state remained ESTABLISHED after disabling the DUT neighbor")
		}
		t.Logf("ON_CHANGE session-state after disabling the DUT neighbor: %v", got)
		if elapsed > stateLatency {
			t.Errorf("ON_CHANGE session-state update latency: got %v, want < %v",
				elapsed, stateLatency)
		} else {
			t.Logf("ON_CHANGE session-state %v received in %v", got, elapsed)
		}
		t.Log("Re-enabling the DUT CE BGP neighbor")
		setCENeighborEnabled(t, dut, true)

		if _, _, ok := awaitSessionState(t, dut, nbr, convergeWait, oc.Bgp_Neighbor_SessionState_ESTABLISHED); !ok {
			t.Fatalf("session did not re-establish after peer flap")
		}
		transAfter := gnmi.Get(t, dut, nbr.EstablishedTransitions().State())
		lastEstAfter := gnmi.Get(t, dut, nbr.LastEstablished().State())
		t.Logf("After flap: established-transitions=%d last-established=%d", transAfter, lastEstAfter)
		if transAfter <= transBefore {
			t.Errorf("established-transitions: got %d, want > %d", transAfter, transBefore)
		}
		if lastEstAfter <= lastEstBefore {
			t.Errorf("last-established: got %d, want > %d", lastEstAfter, lastEstBefore)
		}
	})

	t.Run("RT-1.109.2 Prefix counters streaming", func(t *testing.T) {
		// Do not interpret zero prefix counters as a telemetry failure if the
		// preceding session-state test left the BGP session down.
		if _, _, ok := awaitSessionState(t, dut, nbr, convergeWait, oc.Bgp_Neighbor_SessionState_ESTABLISHED); !ok {
			fptest.LogQuery(t, "CE neighbor before prefix validation", nbr.State(), gnmi.Get(t, dut, nbr.State()))
			t.Fatalf("CE BGP session is not ESTABLISHED before prefix validation")
		}
		sent := uint32(dutStaticCount)
		verifyV4Prefixes(t, dut, baseV4Routes, baseV4Routes, &sent)
		v6Rx := bgpPath(vrfName).Neighbor(atePort1.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes().Received().State()
		if got, ok := awaitPrefixCount(t, dut, v6Rx, baseV6Routes, convergeWait); !ok {
			t.Errorf("IPv6 prefixes/received: got %d, want %d", got, baseV6Routes)
		}
		t.Logf("Advertising %d additional IPv4 routes", addV4Routes1)
		setRouteState(t, ate, []string{routeV4Add1}, gosnappi.StateProtocolRouteState.ADVERTISE)
		total := uint32(baseV4Routes + addV4Routes1)
		verifyV4Prefixes(t, dut, total, total, nil)
	})

	t.Run("RT-1.109.3 Max-prefix warning telemetry", func(t *testing.T) {
		// prefix-limit-exceeded reports the hard max-prefix limit being exceeded.
		// It is not the warning-threshold notification. At 4,250 routes, the
		// configured 80% warning threshold (4,000) has been crossed, but the
		// hard limit (5,000) has not.
		total := uint32(baseV4Routes + addV4Routes1 + addV4Routes2)
		warningThreshold := uint32(maxPrefixes * warnPct / 100)
		if total <= warningThreshold || total >= maxPrefixes {
			t.Fatalf("invalid RT-1.109.3 route count: got %d, want > warning threshold %d and < max-prefix %d", total, warningThreshold, maxPrefixes)
		}
		t.Logf("Advertising %d additional IPv4 routes (crosses warning threshold %d, below max-prefix %d)", addV4Routes2, warningThreshold, maxPrefixes)
		setRouteState(t, ate, []string{routeV4Add2}, gosnappi.StateProtocolRouteState.ADVERTISE)
		verifyV4Prefixes(t, dut, total, total, nil)

		if got := gnmi.Get(t, dut, nbr.SessionState().State()); got != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
			t.Fatalf("session-state after warning threshold: got %v, want ESTABLISHED", got)
		}
		// Validate the configured max-prefix and warning threshold through
		// OpenConfig when the DUT supports configuring these leaves over OC.
		// Platforms using vendor CLI configuration cannot be checked through
		// these OpenConfig config/state paths.
		if !deviations.BGPMissingOCMaxPrefixesConfiguration(dut) {
			afi := nbr.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast()
			var gotMax uint32
			var gotWarningPct uint8
			if deviations.BGPExplicitPrefixLimitReceived(dut) {
				gotMax = gnmi.Get(t, dut, afi.PrefixLimitReceived().MaxPrefixes().State())
				gotWarningPct = gnmi.Get(t, dut, afi.PrefixLimitReceived().WarningThresholdPct().State())
			} else {
				gotMax = gnmi.Get(t, dut, afi.PrefixLimit().MaxPrefixes().State())
				gotWarningPct = gnmi.Get(t, dut, afi.PrefixLimit().WarningThresholdPct().State())
			}
			if gotMax != maxPrefixes {
				t.Errorf("max-prefix configuration: got %d, want %d", gotMax, maxPrefixes)
			}
			if gotWarningPct != warnPct {
				t.Errorf("warning-threshold-pct: got %d, want %d", gotWarningPct, warnPct)
			}
		} else {
			t.Log("max-prefix is configured through vendor CLI; OpenConfig configuration-state validation is not applicable")
		}
		// The standard prefix-limit-exceeded leaf may remain false until the
		// hard limit is reached. Do not incorrectly require it to signal the
		// warning threshold. A vendor-specific warning-event subscription is
		// required to validate the actual warning notification on platforms
		// that expose one.
		if deviations.PrefixLimitExceededTelemetryUnsupported(dut) {
			t.Log("prefix-limit-exceeded telemetry is unsupported; validated prefix count, configured warning threshold, and established session")
		} else {
			// README Step 3: the leaf must be streamed to the telemetry client
			// over a gNMI STREAM subscription (not a one-shot Get).
			exceeded, ok := awaitPrefixLimitExceededPresence(t, dut, convergeWait)
			switch {
			case !ok:
				t.Errorf("prefix-limit-exceeded was not streamed to the telemetry client after crossing the warning threshold")
			case exceeded:
				t.Log("Streamed prefix-limit-exceeded=true at the warning threshold; DUT reports the limit condition early")
			default:
				t.Log("Streamed prefix-limit-exceeded=false below the hard max-prefix limit, as expected")
			}
		}
	})

	t.Run("RT-1.109.4 Max-prefix teardown telemetry", func(t *testing.T) {
		before, _ := gnmi.Lookup(t, dut, nbr.LastPrefixLimitExceeded().State()).Val()
		// Start all event-driven subscriptions before advertising the routes that
		// cross the hard prefix limit, so the teardown events cannot be missed.
		idleWatch := gnmi.Watch(t, onChangeSubscriptionOpts(dut), nbr.SessionState().State(), convergeWait, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, present := v.Val()
			return present && state == oc.Bgp_Neighbor_SessionState_IDLE
		})
		// The last-notification fields retain the most recent historical
		// notification. RT-1.109.1 administratively disabled this neighbor, so
		// the current subcode may still be ADMINISTRATIVE_SHUTDOWN here. Wait
		// for the specific max-prefix notification rather than accepting any
		// stale CEASE code as evidence of this teardown.
		lastSent := gnmi.Get(t, dut, nbr.Messages().Sent().State())
		t.Logf("Before max-prefix teardown: last-notification-error-code=%v last-notification-error-subcode=%v", lastSent.GetLastNotificationErrorCode(), lastSent.GetLastNotificationErrorSubcode())
		ceaseWatch := gnmi.Watch(t, onChangeSubscriptionOpts(dut), nbr.Messages().Sent().State(), convergeWait, func(v *ygnmi.Value[*oc.NetworkInstance_Protocol_Bgp_Neighbor_Messages_Sent]) bool {
			s, present := v.Val()
			return present &&
				s.GetLastNotificationErrorCode() == oc.BgpTypes_BGP_ERROR_CODE_CEASE &&
				s.GetLastNotificationErrorSubcode() == oc.BgpTypes_BGP_ERROR_SUBCODE_MAX_NUM_PREFIXES_REACHED
		})
		lastPrefixWatch := gnmi.Watch(t, onChangeSubscriptionOpts(dut), nbr.LastPrefixLimitExceeded().State(), convergeWait, func(v *ygnmi.Value[uint64]) bool {
			ts, present := v.Val()
			return present && ts > before
		})
		t.Logf("Advertising %d additional IPv4 routes (exceeds max-prefix %d)", addV4Routes3, maxPrefixes)
		setRouteState(t, ate, []string{routeV4Add3}, gosnappi.StateProtocolRouteState.ADVERTISE)
		if _, ok := idleWatch.Await(t); !ok {
			fptest.LogQuery(t, "CE neighbor", nbr.State(), gnmi.Get(t, dut, nbr.State()))
			t.Errorf("session-state did not stream IDLE after exceeding max-prefix")
		}
		sentVal, ok := ceaseWatch.Await(t)
		if !ok {
			current := gnmi.Get(t, dut, nbr.Messages().Sent().State())
			t.Errorf("max-prefix notification not observed: got code=%v subcode=%v, want CEASE/MAX_NUM_PREFIXES_REACHED", current.GetLastNotificationErrorCode(), current.GetLastNotificationErrorSubcode())
		} else if sent, present := sentVal.Val(); !present {
			t.Errorf("BGP sent-message telemetry value is not present after max-prefix teardown")
		} else {
			t.Logf("Observed max-prefix notification: code=%v subcode=%v", sent.GetLastNotificationErrorCode(), sent.GetLastNotificationErrorSubcode())
		}
		if _, ok := lastPrefixWatch.Await(t); !ok {
			t.Errorf("last-prefix-limit-exceeded timestamp did not update (before=%d)", before)
		}
		t.Log("Withdrawing excess routes and restoring baseline")
		setRouteState(t, ate, []string{routeV4Add1, routeV4Add2, routeV4Add3}, gosnappi.StateProtocolRouteState.WITHDRAW)
		// Reset the session from the ATE side in case the DUT holds it down.
		setPeerState(t, ate, []string{ceV4PeerName, ceV6PeerName}, gosnappi.StateProtocolBgpPeersState.DOWN)
		waitForSessionToLeaveEstablished(t, dut, nbr, time.Minute)
		setPeerState(t, ate, []string{ceV4PeerName, ceV6PeerName}, gosnappi.StateProtocolBgpPeersState.UP)
		// Some platforms keep the max-prefix neighbor in hold-down until an
		// explicit clear is issued.
		clearCENeighborOnDUT(t, dut)
		if _, _, ok := awaitSessionState(t, dut, nbr, 5*time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED); !ok {
			t.Fatalf("session did not return to ESTABLISHED after withdrawing excess routes")
		}
		verifyV4Prefixes(t, dut, baseV4Routes, baseV4Routes, nil)
	})
}

// recoverCEBaselineSession forces the CE neighbor back to ESTABLISHED before
// running RT steps.
func recoverCEBaselineSession(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, nbr *netinstbgp.NetworkInstance_Protocol_Bgp_NeighborPath) {
	t.Helper()
	for attempt := 1; attempt <= 3; attempt++ {
		t.Logf("Recovering CE baseline session (attempt %d/3)", attempt)
		setPeerState(t, ate, []string{ceV4PeerName, ceV6PeerName}, gosnappi.StateProtocolBgpPeersState.DOWN)
		_, _, _ = awaitSessionState(t, dut, nbr, 45*time.Second, oc.Bgp_Neighbor_SessionState_IDLE, oc.Bgp_Neighbor_SessionState_ACTIVE, oc.Bgp_Neighbor_SessionState_CONNECT)
		clearCENeighborOnDUT(t, dut)
		setPeerState(t, ate, []string{ceV4PeerName, ceV6PeerName}, gosnappi.StateProtocolBgpPeersState.UP)
		if _, _, ok := awaitSessionState(t, dut, nbr, 2*time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED); ok {
			return
		}
	}
	fptest.LogQuery(t, "CE neighbor", nbr.State(), gnmi.Get(t, dut, nbr.State()))
	t.Fatalf("failed to recover CE neighbor to ESTABLISHED baseline")
}

// waitForSessionToLeaveEstablished waits until the BGP session transitions
// away from ESTABLISHED after the peer is administratively shut down.
func waitForSessionToLeaveEstablished(t *testing.T, dut *ondatra.DUTDevice, nbr *netinstbgp.NetworkInstance_Protocol_Bgp_NeighborPath, timeout time.Duration) {
	t.Helper()
	_, ok := gnmi.Watch(t, onChangeSubscriptionOpts(dut), nbr.SessionState().State(), timeout,
		func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, present := v.Val()
			return present && state != oc.Bgp_Neighbor_SessionState_ESTABLISHED
		},
	).Await(t)

	if !ok {
		t.Fatalf("BGP session did not leave ESTABLISHED state within %v after peer shutdown", timeout)
	}
}
