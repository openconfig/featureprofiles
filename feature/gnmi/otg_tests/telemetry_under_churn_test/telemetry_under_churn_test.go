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

// Package telemetry_under_churn_test implements gNMI-1.30: gNMI Telemetry
// Performance under Dynamic Control Plane Churn.
package telemetry_under_churn_test

import (
	"context"
	"fmt"
	"math"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126

	dutAS       = 65001
	ateAS       = 65002
	peerGroupV4 = "BGP-PEER-GROUP-V4"
	peerGroupV6 = "BGP-PEER-GROUP-V6"
	allowPolicy = "ALLOW"

	baselineRouteCount = 100
	ipv4RouteStart     = "198.51.100.1"
	ipv6RouteStart     = "2001:db8:100::1"
	// Covering routes advertised by ATE port2 over BGP, so that prefixes
	// transiently removed by gRIBI add/delete churn still forward to port2.
	ipv4CoverAddr = "198.51.100.0"
	ipv4CoverLen  = 24
	ipv6CoverAddr = "2001:db8:100::"
	ipv6CoverLen  = 64

	nhIDv4  = 1001
	nhIDv6  = 1002
	nhgV4A  = 2001
	nhgV4B  = 2002
	nhgV6A  = 3001
	nhgV6B  = 3002
	nhgBase = 1

	addDeleteSubset = 10

	sampleInterval    = 10 * time.Second
	cadenceTolerance  = 2500 * time.Millisecond
	deliveryTolerance = 3 * time.Second
	churnDuration     = 2 * time.Minute
	counterSettleTime = 2*sampleInterval + 5*time.Second

	trafficPPS          = 5000
	frameSize           = 512
	counterTolerancePct = 1.0

	flowIPv4 = "flowIPv4"
	flowIPv6 = "flowIPv6"

	inUnicastLeaf  = "in-unicast-pkts"
	outUnicastLeaf = "out-unicast-pkts"

	operStatusTimeout = 2 * time.Minute
	bgpTimeout        = 3 * time.Minute
	aftTimeout        = 2 * time.Minute
	syncTimeout       = 2 * time.Minute
	gribiTimeout      = 2 * time.Minute
	flowStatsTimeout  = time.Minute
)

// gNMI-1.30.2 scale and churn parameters.
const (
	scaledRouteCount     = 10000
	scaledIPv4RouteStart = "198.18.0.1"
	scaledIPv6RouteStart = "2001:db8:200::1"
	scaledIPv4CoverAddr  = "198.18.0.0"
	scaledIPv4CoverLen   = 16
	scaledIPv6CoverAddr  = "2001:db8:200::"
	scaledIPv6CoverLen   = 64

	ecmpNHBaseV4 = 1101
	ecmpNHBaseV6 = 1201
	nhgV4All     = 2101
	nhgV4Alt     = 2102
	nhgV6All     = 3101
	nhgV6Alt     = 3102

	churnBatchSize     = 1000
	targetChurnRate    = 1000
	scaledGRIBITimeout = 5 * time.Minute
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE link 1",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:1::2",
		IPv6Len: ipv6PrefixLen,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:1::1",
		IPv6Len: ipv6PrefixLen,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE link 2",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:2::2",
		IPv6Len: ipv6PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:2::1",
		IPv6Len: ipv6PrefixLen,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "DUT to ATE link 3",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:3::2",
		IPv6Len: ipv6PrefixLen,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:3::1",
		IPv6Len: ipv6PrefixLen,
	}
	dutPort4 = attrs.Attributes{
		Desc:    "DUT to ATE link 4",
		IPv4:    "192.0.2.14",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:4::2",
		IPv6Len: ipv6PrefixLen,
	}
	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		MAC:     "02:00:04:01:01:01",
		IPv4:    "192.0.2.13",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:4::1",
		IPv6Len: ipv6PrefixLen,
	}

	// egressATEs are the ECMP next-hops of gNMI-1.30.2, in NH index order.
	egressATEs = []*attrs.Attributes{&atePort2, &atePort3, &atePort4}

	baselineCover = routeCover{ipv4CoverAddr, ipv4CoverLen, ipv6CoverAddr, ipv6CoverLen}
	scaledCover   = routeCover{scaledIPv4CoverAddr, scaledIPv4CoverLen, scaledIPv6CoverAddr, scaledIPv6CoverLen}
)

// routeCover is the pair of covering prefixes an ATE port advertises over BGP.
type routeCover struct {
	v4Addr string
	v4Len  uint32
	v6Addr string
	v6Len  uint32
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestTelemetryUnderChurn implements gNMI-1.30.
func TestTelemetryUnderChurn(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run("gNMI-1.30.1 Baseline gNMI Telemetry under gRIBI Churn (Single Link)", func(t *testing.T) {
		testBaselineTelemetryUnderChurn(t, dut, ate)
	})
	t.Run("gNMI-1.30.2 Scaled gNMI Telemetry under High gRIBI Churn (4 Links)", func(t *testing.T) {
		testScaledTelemetryUnderChurn(t, dut, ate)
	})
}

func testBaselineTelemetryUnderChurn(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	ctx := context.Background()
	ni := deviations.DefaultNetworkInstance(dut)
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	t.Log("Step 1/2 - Generate DUT configuration and push it using gNMI Set REPLACE.")
	configureDUT(t, dut, map[*ondatra.Port]*attrs.Attributes{p1: &dutPort1, p2: &dutPort2})
	configureDUTBGP(t, dut, &atePort1, &atePort2)

	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	defer ate.OTG().StopProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	t.Log("Step 3 - Wait for interface oper-status UP and BGP sessions ESTABLISHED.")
	awaitOperStatusUp(t, dut, p1, p2)
	awaitBGPEstablished(t, dut, atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6)

	t.Logf("Step 4 - Establish SAMPLE subscriptions (%v) for interface counters and AFTs.", sampleInterval)
	counterMon := startMonitor(t, dut, "interface-counters", counterSubscriptions(p1.Name(), p2.Name()))
	defer counterMon.stop()
	aftMon := startMonitor(t, dut, "afts", aftSubscriptions(ni))
	defer aftMon.stop()
	for _, m := range []*subscriptionMonitor{counterMon, aftMon} {
		if !m.awaitSync(syncTimeout) {
			t.Fatalf("%s subscription did not receive sync_response within %v: %v", m.name, syncTimeout, m.err())
		}
	}

	t.Logf("Step 5 - Inject %d IPv4 and %d IPv6 routes via gRIBI towards port2.", baselineRouteCount, baselineRouteCount)
	gc := newGRIBIChurner(ctx, t, dut)
	defer gc.close(t)
	gc.programBaseline(t)

	t.Log("Step 6 - Wait for programmed prefixes to be present in the AFT.")
	awaitAFTPrefixes(t, dut, ni, gc.v4Prefixes, gc.v6Prefixes)

	t.Log("Step 7 - Start ATE traffic from port1 to port2 matching the gRIBI routes.")
	ingress := []counterLeaf{{p1.Name(), inUnicastLeaf}}
	egress := []counterLeaf{{p2.Name(), outUnicastLeaf}}
	leaves := []counterLeaf{ingress[0], egress[0]}
	base := snapshotCounters(t, counterMon, leaves)
	aftNotifsBefore := aftMon.notifications()
	trafficStart := time.Now()
	ate.OTG().StartTraffic(t)

	t.Logf("Step 8 - Continuous gRIBI churn for %v.", churnDuration)
	gc.churn(t, churnDuration)
	gc.verifyNoFailures(t)

	ate.OTG().StopTraffic(t)
	trafficStop := time.Now()
	t.Logf("Waiting %v for final counter samples.", counterSettleTime)
	time.Sleep(counterSettleTime)

	t.Log("Step 9 - Validate telemetry and traffic.")
	validateSubscriptionStability(t, counterMon, aftMon, aftNotifsBefore)
	validateOnTimeDelivery(t, counterMon, leaves, trafficStart, trafficStop)
	totalTx, totalRx := validateLossless(t, ate, top)
	validateDataAccuracy(t, counterMon, base, ingress, egress, totalTx, totalRx, trafficStop)
}

// testScaledTelemetryUnderChurn implements gNMI-1.30.2.
func testScaledTelemetryUnderChurn(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	ctx := context.Background()
	ni := deviations.DefaultNetworkInstance(dut)
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")

	t.Log("Step 1/2 - Generate DUT configuration for 4 links and push it using gNMI Set REPLACE.")
	configureDUT(t, dut, map[*ondatra.Port]*attrs.Attributes{p1: &dutPort1, p2: &dutPort2, p3: &dutPort3, p4: &dutPort4})
	configureDUTBGP(t, dut, &atePort1, &atePort2, &atePort3, &atePort4)

	top := configureATEScaled(t, ate)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	defer ate.OTG().StopProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	t.Log("Step 3 - Wait for interface oper-status UP and BGP sessions ESTABLISHED on all 4 links.")
	awaitOperStatusUp(t, dut, p1, p2, p3, p4)
	awaitBGPEstablished(t, dut,
		atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6,
		atePort3.IPv4, atePort3.IPv6, atePort4.IPv4, atePort4.IPv6)

	t.Logf("Step 4 - Establish SAMPLE subscriptions (%v) for interface counters and AFTs.", sampleInterval)
	counterMon := startMonitor(t, dut, "interface-counters", counterSubscriptions(p1.Name(), p2.Name(), p3.Name(), p4.Name()))
	defer counterMon.stop()
	aftMon := startMonitor(t, dut, "afts", aftSubscriptions(ni))
	defer aftMon.stop()
	for _, m := range []*subscriptionMonitor{counterMon, aftMon} {
		if !m.awaitSync(syncTimeout) {
			t.Fatalf("%s subscription did not receive sync_response within %v: %v", m.name, syncTimeout, m.err())
		}
	}

	t.Logf("Step 5 - Program %d IPv4 and %d IPv6 routes via gRIBI with ECMP across port2, port3 and port4.", scaledRouteCount, scaledRouteCount)
	gc := newECMPChurner(ctx, t, dut)
	defer gc.close(t)
	gc.programBaseline(t)

	t.Log("Step 6 - Wait for the scaled prefixes to be present in the AFT.")
	awaitAFTPrefixes(t, dut, ni, gc.v4Prefixes, gc.v6Prefixes)

	t.Log("Step 7 - Start ATE traffic from port1 load-balanced to port2, port3 and port4.")
	ingress := []counterLeaf{{p1.Name(), inUnicastLeaf}}
	egress := []counterLeaf{{p2.Name(), outUnicastLeaf}, {p3.Name(), outUnicastLeaf}, {p4.Name(), outUnicastLeaf}}
	leaves := []counterLeaf{ingress[0], egress[0], egress[1], egress[2]}
	base := snapshotCounters(t, counterMon, leaves)
	aftNotifsBefore := aftMon.notifications()
	trafficStart := time.Now()
	ate.OTG().StartTraffic(t)

	t.Logf("Step 8 - High-rate gRIBI ECMP churn for %v.", churnDuration)
	gc.churn(t, churnDuration)
	gc.verifyNoFailures(t)

	ate.OTG().StopTraffic(t)
	trafficStop := time.Now()
	t.Logf("Waiting %v for final counter samples.", counterSettleTime)
	time.Sleep(counterSettleTime)

	t.Log("Step 9 - Validate telemetry and traffic.")
	validateSubscriptionStability(t, counterMon, aftMon, aftNotifsBefore)
	validateOnTimeDelivery(t, counterMon, leaves, trafficStart, trafficStop)
	totalTx, totalRx := validateLossless(t, ate, top)
	validateDataAccuracy(t, counterMon, base, ingress, egress, totalTx, totalRx, trafficStop)
}

// configureDUT replaces the interface configuration of the given ports.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, ports map[*ondatra.Port]*attrs.Attributes) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	b := &gnmi.SetBatch{}
	for p, a := range ports {
		gnmi.BatchReplace(b, gnmi.OC().Interface(p.Name()).Config(), a.NewOCInterface(p.Name(), dut))
	}
	b.Set(t, dut)

	for p := range ports {
		// Interface REPLACE resets EOS ports back to switchport mode.
		if dut.Vendor() == ondatra.ARISTA {
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s\n no switchport\n", p.Name()))
		}
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, p)
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}
}

// configureDUTBGP replaces the ALLOW policy and BGP configuration with eBGP
// IPv4 and IPv6 sessions towards the given ATE peers.
func configureDUTBGP(t *testing.T, dut *ondatra.DUTDevice, peers ...*attrs.Attributes) {
	t.Helper()
	root := &oc.Root{}

	pd := root.GetOrCreateRoutingPolicy().GetOrCreatePolicyDefinition(allowPolicy)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatalf("AppendNewStatement() failed: %v", err)
	}
	st.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	ni := deviations.DefaultNetworkInstance(dut)
	bgpName := deviations.DefaultBgpInstanceName(dut)
	proto := root.GetOrCreateNetworkInstance(ni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
	bgp := proto.GetOrCreateBgp()
	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAS)
	g.RouterId = ygot.String(dutPort1.IPv4)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	for pgName, afi := range map[string]oc.E_BgpTypes_AFI_SAFI_TYPE{
		peerGroupV4: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
		peerGroupV6: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
	} {
		pg := bgp.GetOrCreatePeerGroup(pgName)
		pg.PeerAs = ygot.Uint32(ateAS)
		if deviations.RoutePolicyUnderAFIUnsupported(dut) {
			ap := pg.GetOrCreateApplyPolicy()
			ap.SetImportPolicy([]string{allowPolicy})
			ap.SetExportPolicy([]string{allowPolicy})
			continue
		}
		af := pg.GetOrCreateAfiSafi(afi)
		af.Enabled = ygot.Bool(true)
		ap := af.GetOrCreateApplyPolicy()
		ap.SetImportPolicy([]string{allowPolicy})
		ap.SetExportPolicy([]string{allowPolicy})
	}

	type bgpNbr struct {
		addr string
		pg   string
		afi  oc.E_BgpTypes_AFI_SAFI_TYPE
	}
	var nbrs []bgpNbr
	for _, p := range peers {
		nbrs = append(nbrs,
			bgpNbr{p.IPv4, peerGroupV4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST},
			bgpNbr{p.IPv6, peerGroupV6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST})
	}
	for _, n := range nbrs {
		nbr := bgp.GetOrCreateNeighbor(n.addr)
		nbr.PeerAs = ygot.Uint32(ateAS)
		nbr.Enabled = ygot.Bool(true)
		nbr.PeerGroup = ygot.String(n.pg)
		nbr.GetOrCreateAfiSafi(n.afi).Enabled = ygot.Bool(true)
	}

	b := &gnmi.SetBatch{}
	gnmi.BatchReplace(b, gnmi.OC().RoutingPolicy().PolicyDefinition(allowPolicy).Config(), pd)
	gnmi.BatchReplace(b, gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Config(), proto)
	b.Set(t, dut)
}

// configureATE builds the OTG config: port1/port2 devices with eBGP peers,
// covering routes advertised from port2 and IPv4/IPv6 flows port1->port2.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	d1 := atePort1.AddToOTG(top, ate.Port(t, "port1"), &dutPort1)
	d2 := atePort2.AddToOTG(top, ate.Port(t, "port2"), &dutPort2)
	addATEBGP(d1, &atePort1, &dutPort1, nil)
	addATEBGP(d2, &atePort2, &dutPort2, &baselineCover)

	v4 := addFlow(top, flowIPv4, atePort1.Name+".IPv4", []string{atePort2.Name + ".IPv4"})
	ip4 := v4.Packet().Add().Ipv4()
	ip4.Src().SetValue(atePort1.IPv4)
	ip4.Dst().Increment().SetStart(ipv4RouteStart).SetStep("0.0.0.1").SetCount(baselineRouteCount)

	v6 := addFlow(top, flowIPv6, atePort1.Name+".IPv6", []string{atePort2.Name + ".IPv6"})
	ip6 := v6.Packet().Add().Ipv6()
	ip6.Src().SetValue(atePort1.IPv6)
	ip6.Dst().Increment().SetStart(ipv6RouteStart).SetStep("::1").SetCount(baselineRouteCount)
	return top
}

// configureATEScaled builds the OTG config for gNMI-1.30.2: 4 devices with
// eBGP peers, covering routes advertised from port2-4 and IPv4/IPv6 flows
// from port1 to the scaled routes, received on port2-4.
func configureATEScaled(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	d1 := atePort1.AddToOTG(top, ate.Port(t, "port1"), &dutPort1)
	addATEBGP(d1, &atePort1, &dutPort1, nil)
	var rxV4, rxV6 []string
	for _, p := range []struct {
		id       string
		ate, dut *attrs.Attributes
	}{
		{"port2", &atePort2, &dutPort2},
		{"port3", &atePort3, &dutPort3},
		{"port4", &atePort4, &dutPort4},
	} {
		d := p.ate.AddToOTG(top, ate.Port(t, p.id), p.dut)
		addATEBGP(d, p.ate, p.dut, &scaledCover)
		rxV4 = append(rxV4, p.ate.Name+".IPv4")
		rxV6 = append(rxV6, p.ate.Name+".IPv6")
	}

	v4 := addFlow(top, flowIPv4, atePort1.Name+".IPv4", rxV4)
	ip4 := v4.Packet().Add().Ipv4()
	ip4.Src().SetValue(atePort1.IPv4)
	ip4.Dst().Increment().SetStart(scaledIPv4RouteStart).SetStep("0.0.0.1").SetCount(scaledRouteCount)

	v6 := addFlow(top, flowIPv6, atePort1.Name+".IPv6", rxV6)
	ip6 := v6.Packet().Add().Ipv6()
	ip6.Src().SetValue(atePort1.IPv6)
	ip6.Dst().Increment().SetStart(scaledIPv6RouteStart).SetStep("::1").SetCount(scaledRouteCount)
	return top
}

// addATEBGP adds eBGP IPv4/IPv6 peers to dev; a non-nil cover is advertised.
func addATEBGP(dev gosnappi.Device, ate, dut *attrs.Attributes, cover *routeCover) {
	bgp := dev.Bgp().SetRouterId(ate.IPv4)
	v4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ate.Name + ".IPv4").Peers().Add().SetName(ate.Name + ".BGP4.peer")
	v4Peer.SetPeerAddress(dut.IPv4).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	v6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(ate.Name + ".IPv6").Peers().Add().SetName(ate.Name + ".BGP6.peer")
	v6Peer.SetPeerAddress(dut.IPv6).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	if cover == nil {
		return
	}
	v4Peer.V4Routes().Add().SetName(ate.Name + ".BGP4.routes").
		SetNextHopIpv4Address(ate.IPv4).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).
		Addresses().Add().SetAddress(cover.v4Addr).SetPrefix(cover.v4Len).SetCount(1)
	v6Peer.V6Routes().Add().SetName(ate.Name + ".BGP6.routes").
		SetNextHopIpv6Address(ate.IPv6).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).
		Addresses().Add().SetAddress(cover.v6Addr).SetPrefix(cover.v6Len).SetCount(1)
}

func addFlow(top gosnappi.Config, name, tx string, rx []string) gosnappi.Flow {
	f := top.Flows().Add().SetName(name)
	f.Metrics().SetEnable(true)
	f.TxRx().Device().SetTxNames([]string{tx}).SetRxNames(rx)
	f.Size().SetFixed(frameSize)
	f.Rate().SetPps(trafficPPS)
	f.Duration().Continuous()
	f.Packet().Add().Ethernet().Src().SetValue(atePort1.MAC)
	return f
}

func awaitOperStatusUp(t *testing.T, dut *ondatra.DUTDevice, ports ...*ondatra.Port) {
	t.Helper()
	for _, p := range ports {
		if _, ok := gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), operStatusTimeout, oc.Interface_OperStatus_UP).Val(); !ok {
			t.Fatalf("Interface %s oper-status did not become UP within %v", p.Name(), operStatusTimeout)
		}
	}
}

func awaitBGPEstablished(t *testing.T, dut *ondatra.DUTDevice, neighbors ...string) {
	t.Helper()
	bgp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Bgp()
	for _, n := range neighbors {
		if _, ok := gnmi.Await(t, dut, bgp.Neighbor(n).SessionState().State(), bgpTimeout, oc.Bgp_Neighbor_SessionState_ESTABLISHED).Val(); !ok {
			t.Fatalf("BGP neighbor %s did not reach ESTABLISHED within %v", n, bgpTimeout)
		}
	}
}

// awaitAFTPrefixes waits for the first and last prefix of each family.
func awaitAFTPrefixes(t *testing.T, dut *ondatra.DUTDevice, ni string, v4, v6 []string) {
	t.Helper()
	afts := gnmi.OC().NetworkInstance(ni).Afts()
	for _, p := range []string{v4[0], v4[len(v4)-1]} {
		_, ok := gnmi.Watch(t, dut, afts.Ipv4Entry(p).State(), aftTimeout, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			e, present := v.Val()
			return present && e.GetPrefix() == p
		}).Await(t)
		if !ok {
			t.Fatalf("IPv4 prefix %s not present in AFT within %v", p, aftTimeout)
		}
	}
	for _, p := range []string{v6[0], v6[len(v6)-1]} {
		_, ok := gnmi.Watch(t, dut, afts.Ipv6Entry(p).State(), aftTimeout, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			e, present := v.Val()
			return present && e.GetPrefix() == p
		}).Await(t)
		if !ok {
			t.Fatalf("IPv6 prefix %s not present in AFT within %v", p, aftTimeout)
		}
	}
}

// buildPrefixes returns count consecutive host prefixes starting at start.
func buildPrefixes(t *testing.T, start string, count int) []string {
	t.Helper()
	addr, err := netip.ParseAddr(start)
	if err != nil {
		t.Fatalf("netip.ParseAddr(%q) failed: %v", start, err)
	}
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, netip.PrefixFrom(addr, addr.BitLen()).String())
		addr = addr.Next()
	}
	return out
}

func checkCounterDelta(t *testing.T, desc string, before, after, want uint64) {
	t.Helper()
	if after < before {
		t.Errorf("%s: counter decreased from %d to %d", desc, before, after)
		return
	}
	if want == 0 {
		t.Errorf("%s: ATE reference packet count is 0", desc)
		return
	}
	got := after - before
	diffPct := math.Abs(float64(got)-float64(want)) * 100 / float64(want)
	if diffPct > counterTolerancePct {
		t.Errorf("%s: gNMI delta %d differs from ATE count %d by %.3f%%, want <= %.1f%%", desc, got, want, diffPct, counterTolerancePct)
		return
	}
	t.Logf("%s: gNMI delta %d vs ATE count %d (diff %.3f%%)", desc, got, want, diffPct)
}

// validateCadence checks that SAMPLE updates received in [from, to] are
// timestamped exactly one interval apart and were not delayed at the client.
func validateCadence(t *testing.T, intf string, samples []leafSample, from, to time.Time) {
	t.Helper()
	var window []leafSample
	var failed bool
	for _, s := range samples {
		if !s.received.Before(from) && !s.received.After(to) {
			window = append(window, s)
		}
	}
	if want := int(to.Sub(from)/sampleInterval) - 1; len(window) < want {
		t.Errorf("%s: received %d samples in %v, want >= %d (telemetry starvation)", intf, len(window), to.Sub(from).Round(time.Second), want)
		failed = true
	}
	for i := 1; i < len(window); i++ {
		gap := window[i].timestamp.Sub(window[i-1].timestamp)
		if gap < sampleInterval-cadenceTolerance || gap > sampleInterval+cadenceTolerance {
			t.Errorf("%s: sample timestamp gap %v at %v, want %v +/- %v", intf, gap, window[i].timestamp, sampleInterval, cadenceTolerance)
			failed = true
		}
		if rgap := window[i].received.Sub(window[i-1].received); rgap > sampleInterval+deliveryTolerance {
			t.Errorf("%s: sample delivery gap %v at %v, want <= %v", intf, rgap, window[i].received, sampleInterval+deliveryTolerance)
			failed = true
		}
	}
	t.Logf("%s: validated cadence of %d samples", intf, len(window))
	if failed {
		logSampleSequence(t, intf, window)
	}
}

// logSampleSequence prints every sample with DUT timestamp, receive time and value.
func logSampleSequence(t *testing.T, intf string, samples []leafSample) {
	t.Helper()
	const tsFmt = "15:04:05.000"
	var b strings.Builder
	fmt.Fprintf(&b, "%s sample sequence:\n  %-3s %-14s %-14s %-8s %-8s %s\n", intf, "#", "DUT timestamp", "received at", "ts gap", "rx gap", "value")
	for i, s := range samples {
		tsGap, rxGap := "-", "-"
		if i > 0 {
			tsGap = s.timestamp.Sub(samples[i-1].timestamp).Round(time.Millisecond).String()
			rxGap = s.received.Sub(samples[i-1].received).Round(time.Millisecond).String()
		}
		fmt.Fprintf(&b, "  %-3d %-14s %-14s %-8s %-8s %d\n", i+1, s.timestamp.UTC().Format(tsFmt), s.received.UTC().Format(tsFmt), tsGap, rxGap, s.value)
	}
	t.Log(b.String())
}

// counterLeaf identifies one streamed counter leaf of an interface.
type counterLeaf struct {
	intf string
	leaf string
}

// snapshotCounters returns the latest streamed sample of each leaf.
func snapshotCounters(t *testing.T, mon *subscriptionMonitor, leaves []counterLeaf) map[counterLeaf]leafSample {
	t.Helper()
	out := map[counterLeaf]leafSample{}
	for _, l := range leaves {
		s, ok := mon.latest(l.intf, l.leaf)
		if !ok {
			t.Fatalf("No baseline %s sample received for %s", l.leaf, l.intf)
		}
		out[l] = s
	}
	return out
}

func validateSubscriptionStability(t *testing.T, counterMon, aftMon *subscriptionMonitor, aftNotifsBefore int) {
	t.Helper()
	t.Log("Validating Subscription Stability.")
	for _, m := range []*subscriptionMonitor{counterMon, aftMon} {
		if err := m.err(); err != nil {
			t.Errorf("Subscription Stability: %s subscription was disrupted during churn: %v", m.name, err)
		}
		if got := m.syncs(); got != 1 {
			t.Errorf("Subscription Stability: %s subscription received %d sync_responses, want exactly 1", m.name, got)
		}
	}
	if aftMon.notifications()-aftNotifsBefore == 0 {
		t.Errorf("Subscription Stability: AFT subscription received no notifications during gRIBI churn")
	}
}

func validateOnTimeDelivery(t *testing.T, mon *subscriptionMonitor, leaves []counterLeaf, from, to time.Time) {
	t.Helper()
	t.Log("Validating On-time Delivery.")
	for _, l := range leaves {
		validateCadence(t, l.intf+" "+l.leaf, mon.samplesFor(l.intf, l.leaf), from, to)
	}
}

// validateLossless checks Tx == Rx per flow and returns the ATE totals.
func validateLossless(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) (totalTx, totalRx uint64) {
	t.Helper()
	t.Log("Validating Lossless Validation.")
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	for _, f := range []string{flowIPv4, flowIPv6} {
		tx, rx := otgutils.GetFlowStats(t, ate.OTG(), f, flowStatsTimeout)
		totalTx += tx
		totalRx += rx
		if tx == 0 {
			t.Errorf("Lossless Validation: flow %s transmitted no packets", f)
			continue
		}
		if tx != rx {
			t.Errorf("Lossless Validation: flow %s tx=%d rx=%d (loss %.4f%%)", f, tx, rx, float64(tx-rx)*100/float64(tx))
		}
	}
	return totalTx, totalRx
}

// validateDataAccuracy compares the summed ingress and egress counter deltas
// with the ATE Tx and Rx totals.
func validateDataAccuracy(t *testing.T, mon *subscriptionMonitor, base map[counterLeaf]leafSample, ingress, egress []counterLeaf, totalTx, totalRx uint64, trafficStop time.Time) {
	t.Helper()
	t.Log("Validating Data Accuracy.")
	final := map[counterLeaf]leafSample{}
	for _, l := range append(append([]counterLeaf{}, ingress...), egress...) {
		s, ok := mon.latest(l.intf, l.leaf)
		if !ok || !s.received.After(trafficStop) {
			t.Errorf("Data Accuracy: no counter sample received after traffic stopped")
			return
		}
		final[l] = s
	}
	for _, g := range []struct {
		leaves []counterLeaf
		want   uint64
	}{{ingress, totalTx}, {egress, totalRx}} {
		var names []string
		var before, after uint64
		for _, l := range g.leaves {
			names = append(names, l.intf)
			before += base[l].value
			after += final[l].value
		}
		checkCounterDelta(t, fmt.Sprintf("Data Accuracy: %s %s", strings.Join(names, "+"), g.leaves[0].leaf), before, after, g.want)
	}
}

// gribiSession is a SINGLE_PRIMARY, PRESERVE gRIBI session that is leader
// and starts from an empty RIB.
type gribiSession struct {
	ctx     context.Context
	c       *fluent.GRIBIClient
	ni      string
	timeout time.Duration
}

func newGRIBISession(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration) *gribiSession {
	t.Helper()
	c := fluent.NewClient()
	conn := c.Connection().WithStub(dut.RawAPIs().GRIBI(t)).
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithPersistence().
		WithInitialElectionID(1, 0)
	if !deviations.GRIBIRIBAckOnly(dut) {
		conn.WithFIBACK()
	}
	c.Start(ctx, t)
	c.StartSending(ctx, t)
	s := &gribiSession{ctx: ctx, c: c, ni: deviations.DefaultNetworkInstance(dut), timeout: timeout}
	s.await(t)
	gribi.BecomeLeader(t, c)
	if err := gribi.FlushAll(c); err != nil {
		t.Fatalf("gRIBI FlushAll failed: %v", err)
	}
	return s
}

func (s *gribiSession) close(t *testing.T) {
	t.Helper()
	if err := gribi.FlushAll(s.c); err != nil {
		t.Errorf("gRIBI FlushAll failed: %v", err)
	}
	s.c.Stop(t)
}

func (s *gribiSession) await(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()
	if err := s.c.Await(ctx, t); err != nil {
		t.Fatalf("gRIBI Await failed: %v", err)
	}
}

func (s *gribiSession) verifyNoFailures(t *testing.T) {
	t.Helper()
	var failed int
	for _, r := range s.c.Results(t) {
		if r.ProgrammingResult == spb.AFTResult_FAILED {
			failed++
			if failed <= 5 {
				t.Errorf("gRIBI operation failed: %s", r)
			}
		}
	}
	if failed > 0 {
		t.Errorf("%d gRIBI operations failed", failed)
	}
}

// ecmpChurner churns the gNMI-1.30.2 scaled routes over ECMP next-hops on port2-4.
type ecmpChurner struct {
	*gribiSession
	v4Prefixes []string
	v6Prefixes []string
	blockAlt   []bool // per churnBatchSize prefix block: routed via the ALT NHG
	trimmed    bool   // ALL NHG currently without its last member
}

func newECMPChurner(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) *ecmpChurner {
	t.Helper()
	return &ecmpChurner{
		gribiSession: newGRIBISession(ctx, t, dut, scaledGRIBITimeout),
		v4Prefixes:   buildPrefixes(t, scaledIPv4RouteStart, scaledRouteCount),
		v6Prefixes:   buildPrefixes(t, scaledIPv6RouteStart, scaledRouteCount),
		blockAlt:     make([]bool, scaledRouteCount/churnBatchSize),
	}
}

func (gc *ecmpChurner) nhEntries() []fluent.GRIBIEntry {
	var es []fluent.GRIBIEntry
	for i, a := range egressATEs {
		es = append(es,
			fluent.NextHopEntry().WithNetworkInstance(gc.ni).WithIndex(ecmpNHBaseV4+uint64(i)).WithIPAddress(a.IPv4),
			fluent.NextHopEntry().WithNetworkInstance(gc.ni).WithIndex(ecmpNHBaseV6+uint64(i)).WithIPAddress(a.IPv6))
	}
	return es
}

// nhgEntries returns, per family, ALL = {p2,p3,p4} (or {p2,p3} when trimmed)
// and ALT = {p3,p4}.
func (gc *ecmpChurner) nhgEntries() []fluent.GRIBIEntry {
	allMembers := len(egressATEs)
	if gc.trimmed {
		allMembers--
	}
	var es []fluent.GRIBIEntry
	for _, f := range []struct{ all, alt, nhBase uint64 }{
		{nhgV4All, nhgV4Alt, ecmpNHBaseV4},
		{nhgV6All, nhgV6Alt, ecmpNHBaseV6},
	} {
		all := fluent.NextHopGroupEntry().WithNetworkInstance(gc.ni).WithID(f.all)
		for i := 0; i < allMembers; i++ {
			all.AddNextHop(f.nhBase+uint64(i), 1)
		}
		alt := fluent.NextHopGroupEntry().WithNetworkInstance(gc.ni).WithID(f.alt)
		for i := 1; i < len(egressATEs); i++ {
			alt.AddNextHop(f.nhBase+uint64(i), 1)
		}
		es = append(es, all, alt)
	}
	return es
}

// blockEntries returns the IPv4 and IPv6 entries of prefix block b.
func (gc *ecmpChurner) blockEntries(b int) []fluent.GRIBIEntry {
	v4NHG, v6NHG := uint64(nhgV4All), uint64(nhgV6All)
	if gc.blockAlt[b] {
		v4NHG, v6NHG = nhgV4Alt, nhgV6Alt
	}
	lo, hi := b*churnBatchSize, (b+1)*churnBatchSize
	var es []fluent.GRIBIEntry
	for _, p := range gc.v4Prefixes[lo:hi] {
		es = append(es, fluent.IPv4Entry().WithNetworkInstance(gc.ni).WithPrefix(p).
			WithNextHopGroup(v4NHG).WithNextHopGroupNetworkInstance(gc.ni))
	}
	for _, p := range gc.v6Prefixes[lo:hi] {
		es = append(es, fluent.IPv6Entry().WithNetworkInstance(gc.ni).WithPrefix(p).
			WithNextHopGroup(v6NHG).WithNextHopGroupNetworkInstance(gc.ni))
	}
	return es
}

// programBaseline installs the ECMP next-hops, NHGs and all prefix blocks.
func (gc *ecmpChurner) programBaseline(t *testing.T) {
	t.Helper()
	gc.c.Modify().AddEntry(t, gc.nhEntries()...)
	gc.await(t)
	gc.c.Modify().AddEntry(t, gc.nhgEntries()...)
	gc.await(t)
	for b := range gc.blockAlt {
		gc.c.Modify().AddEntry(t, gc.blockEntries(b)...)
		gc.await(t)
	}
	gc.verifyNoFailures(t)
}

// churn rotates NHG reassignment of a prefix block, ECMP member removal and
// re-add, and delete/re-add of a prefix block until d elapses.
func (gc *ecmpChurner) churn(t *testing.T, d time.Duration) {
	t.Helper()
	ops := map[string]int{}
	var entries, b int
	start := time.Now()
	for i := 0; time.Since(start) < d; i++ {
		switch i % 3 {
		case 0:
			gc.blockAlt[b] = !gc.blockAlt[b]
			es := gc.blockEntries(b)
			gc.c.Modify().AddEntry(t, es...)
			gc.await(t)
			entries += len(es)
			b = (b + 1) % len(gc.blockAlt)
			ops["nhg-shift"]++
		case 1:
			gc.trimmed = !gc.trimmed
			es := gc.nhgEntries()
			gc.c.Modify().AddEntry(t, es...)
			gc.await(t)
			entries += len(es)
			ops["nhg-member"]++
		case 2:
			es := gc.blockEntries(b)
			gc.c.Modify().DeleteEntry(t, es...)
			gc.await(t)
			gc.c.Modify().AddEntry(t, es...)
			gc.await(t)
			entries += 2 * len(es)
			b = (b + 1) % len(gc.blockAlt)
			ops["add-delete"]++
		}
	}
	elapsed := time.Since(start)
	t.Logf("gRIBI churn completed: %v, %d entry operations in %v (%.0f ops/s, target %d ops/s)",
		ops, entries, elapsed.Round(time.Second), float64(entries)/elapsed.Seconds(), targetChurnRate)
}

// gribiChurner churns the gNMI-1.30.1 baseline routes over a single next-hop on port2.
type gribiChurner struct {
	*gribiSession
	v4Prefixes []string
	v6Prefixes []string
	v4NHG      uint64
	v6NHG      uint64
	weight     uint64
}

func newGRIBIChurner(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) *gribiChurner {
	t.Helper()
	return &gribiChurner{
		gribiSession: newGRIBISession(ctx, t, dut, gribiTimeout),
		v4Prefixes:   buildPrefixes(t, ipv4RouteStart, baselineRouteCount),
		v6Prefixes:   buildPrefixes(t, ipv6RouteStart, baselineRouteCount),
		v4NHG:        nhgV4A,
		v6NHG:        nhgV6A,
		weight:       nhgBase,
	}
}

func (gc *gribiChurner) nhgEntries() []fluent.GRIBIEntry {
	var es []fluent.GRIBIEntry
	for _, n := range []struct{ nhg, nh uint64 }{
		{nhgV4A, nhIDv4}, {nhgV4B, nhIDv4}, {nhgV6A, nhIDv6}, {nhgV6B, nhIDv6},
	} {
		es = append(es, fluent.NextHopGroupEntry().WithNetworkInstance(gc.ni).WithID(n.nhg).AddNextHop(n.nh, gc.weight))
	}
	return es
}

func (gc *gribiChurner) routeEntries(v4, v6 []string) []fluent.GRIBIEntry {
	var es []fluent.GRIBIEntry
	for _, p := range v4 {
		es = append(es, fluent.IPv4Entry().WithNetworkInstance(gc.ni).WithPrefix(p).
			WithNextHopGroup(gc.v4NHG).WithNextHopGroupNetworkInstance(gc.ni))
	}
	for _, p := range v6 {
		es = append(es, fluent.IPv6Entry().WithNetworkInstance(gc.ni).WithPrefix(p).
			WithNextHopGroup(gc.v6NHG).WithNextHopGroupNetworkInstance(gc.ni))
	}
	return es
}

// programBaseline installs the next-hops, both NHG pairs and all routes.
func (gc *gribiChurner) programBaseline(t *testing.T) {
	t.Helper()
	gc.c.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(gc.ni).WithIndex(nhIDv4).WithIPAddress(atePort2.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(gc.ni).WithIndex(nhIDv6).WithIPAddress(atePort2.IPv6),
	)
	gc.c.Modify().AddEntry(t, gc.nhgEntries()...)
	gc.c.Modify().AddEntry(t, gc.routeEntries(gc.v4Prefixes, gc.v6Prefixes)...)
	gc.await(t)
	gc.verifyNoFailures(t)
}

// churn cycles NHG weight updates, entry swaps between pre-programmed NHGs
// and add/delete of a prefix subset until d elapses.
func (gc *gribiChurner) churn(t *testing.T, d time.Duration) {
	t.Helper()
	ops := map[string]int{}
	for i, deadline := 0, time.Now().Add(d); time.Now().Before(deadline); i++ {
		switch i % 3 {
		case 0:
			// ADD of an existing NHG is an implicit replace in gRIBI.
			gc.weight = gc.weight%2 + 1
			gc.c.Modify().AddEntry(t, gc.nhgEntries()...)
			gc.await(t)
			ops["nhg-update"]++
		case 1:
			gc.v4NHG = swapNHG(gc.v4NHG, nhgV4A, nhgV4B)
			gc.v6NHG = swapNHG(gc.v6NHG, nhgV6A, nhgV6B)
			gc.c.Modify().AddEntry(t, gc.routeEntries(gc.v4Prefixes, gc.v6Prefixes)...)
			gc.await(t)
			ops["entry-swap"]++
		case 2:
			subset := gc.routeEntries(gc.v4Prefixes[:addDeleteSubset], gc.v6Prefixes[:addDeleteSubset])
			gc.c.Modify().DeleteEntry(t, subset...)
			gc.await(t)
			gc.c.Modify().AddEntry(t, subset...)
			gc.await(t)
			ops["add-delete"]++
		}
	}
	t.Logf("gRIBI churn completed: %v", ops)
}

func swapNHG(cur, a, b uint64) uint64 {
	if cur == a {
		return b
	}
	return a
}

// leafSample is one streamed value of a single counter leaf.
type leafSample struct {
	timestamp time.Time
	received  time.Time
	value     uint64
}

// subscriptionMonitor runs a raw gNMI STREAM subscription in the background,
// so per-notification timestamps and sync_responses can be inspected.
type subscriptionMonitor struct {
	name   string
	cancel context.CancelFunc
	done   chan struct{}

	mu        sync.Mutex
	syncCount int
	notifs    int
	streamErr error
	samples   map[string][]leafSample // keyed by leafKey(intf, leaf)
}

func counterSubscriptions(intfs ...string) []*gpb.Subscription {
	var subs []*gpb.Subscription
	for _, intf := range intfs {
		subs = append(subs, &gpb.Subscription{
			Path: &gpb.Path{Elem: []*gpb.PathElem{
				{Name: "interfaces"},
				{Name: "interface", Key: map[string]string{"name": intf}},
				{Name: "state"},
				{Name: "counters"},
			}},
			Mode:           gpb.SubscriptionMode_SAMPLE,
			SampleInterval: uint64(sampleInterval.Nanoseconds()),
		})
	}
	return subs
}

func aftSubscriptions(ni string) []*gpb.Subscription {
	return []*gpb.Subscription{{
		Path: &gpb.Path{Elem: []*gpb.PathElem{
			{Name: "network-instances"},
			{Name: "network-instance", Key: map[string]string{"name": ni}},
			{Name: "afts"},
		}},
		Mode:           gpb.SubscriptionMode_SAMPLE,
		SampleInterval: uint64(sampleInterval.Nanoseconds()),
	}}
}

func startMonitor(t *testing.T, dut *ondatra.DUTDevice, name string, subs []*gpb.Subscription) *subscriptionMonitor {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := dut.RawAPIs().GNMI(t).Subscribe(ctx)
	if err != nil {
		cancel()
		t.Fatalf("%s: Subscribe() failed: %v", name, err)
	}
	req := &gpb.SubscribeRequest{Request: &gpb.SubscribeRequest_Subscribe{Subscribe: &gpb.SubscriptionList{
		Prefix:       &gpb.Path{Origin: "openconfig", Target: dut.Name()},
		Mode:         gpb.SubscriptionList_STREAM,
		Encoding:     gpb.Encoding_PROTO,
		Subscription: subs,
	}}}
	if err := stream.Send(req); err != nil {
		cancel()
		t.Fatalf("%s: sending SubscribeRequest failed: %v", name, err)
	}
	m := &subscriptionMonitor{
		name:    name,
		cancel:  cancel,
		done:    make(chan struct{}),
		samples: map[string][]leafSample{},
	}
	go func() {
		defer close(m.done)
		for {
			resp, err := stream.Recv()
			if err != nil {
				if ctx.Err() == nil {
					m.mu.Lock()
					m.streamErr = fmt.Errorf("stream terminated: %w", err)
					m.mu.Unlock()
				}
				return
			}
			if resp.GetSyncResponse() {
				m.mu.Lock()
				m.syncCount++
				m.mu.Unlock()
				continue
			}
			if n := resp.GetUpdate(); n != nil {
				m.record(n, time.Now())
			}
		}
	}()
	return m
}

func (m *subscriptionMonitor) stop() {
	m.cancel()
	<-m.done
}

func (m *subscriptionMonitor) record(n *gpb.Notification, recv time.Time) {
	ts := time.Unix(0, n.GetTimestamp())
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifs++
	for _, u := range n.GetUpdate() {
		elems := append(append([]*gpb.PathElem{}, n.GetPrefix().GetElem()...), u.GetPath().GetElem()...)
		if len(elems) == 0 {
			continue
		}
		leaf := elems[len(elems)-1].GetName()
		if leaf != inUnicastLeaf && leaf != outUnicastLeaf {
			continue
		}
		intf := interfaceKey(elems)
		if intf == "" {
			continue
		}
		k := leafKey(intf, leaf)
		m.samples[k] = append(m.samples[k], leafSample{timestamp: ts, received: recv, value: u.GetVal().GetUintVal()})
	}
}

func leafKey(intf, leaf string) string {
	return intf + "/" + leaf
}

func interfaceKey(elems []*gpb.PathElem) string {
	for _, e := range elems {
		if e.GetName() == "interface" {
			return e.GetKey()["name"]
		}
	}
	return ""
}

func (m *subscriptionMonitor) awaitSync(timeout time.Duration) bool {
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(time.Second) {
		if m.syncs() > 0 {
			return true
		}
		if m.err() != nil {
			return false
		}
	}
	return false
}

func (m *subscriptionMonitor) syncs() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.syncCount
}

func (m *subscriptionMonitor) notifications() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.notifs
}

func (m *subscriptionMonitor) err() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.streamErr
}

func (m *subscriptionMonitor) samplesFor(intf, leaf string) []leafSample {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]leafSample(nil), m.samples[leafKey(intf, leaf)]...)
}

func (m *subscriptionMonitor) latest(intf, leaf string) (leafSample, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.samples[leafKey(intf, leaf)]
	if len(s) == 0 {
		return leafSample{}, false
	}
	return s[len(s)-1], true
}
