// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package strict_fib_precedence_blackhole_test implements TE-1.22: Traffic Blackhole on
// gRIBI Invalid Next-Hop (Strict FIB Precedence).
package strict_fib_precedence_blackhole_test

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	spb "github.com/openconfig/gribi/v1/proto/service"
)

const (
	numRoutes = 1000

	dutAS  = 65001
	ate1AS = 65002
	ate2AS = 65003

	// Traffic profile: every flow cycles through all 1000 destinations.
	trafficPkts = 20000
	trafficPPS  = 5000
	pktSize     = 128

	lossTolerancePct = 0.1
	// ctrlPlaneAllowance is the egress tolerance on DUT port1/port2 during blackhole runs
	// (0.5% of a flow). Interface out-pkts counters also count DUT-originated control-plane
	// packets (BGP keepalives on the live eBGP sessions, LLDP, IPv6 ND), so requiring an
	// exact zero would fail even when no data traffic leaks; observed leakage-free runs
	// show a handful of such packets. A real data-plane leak sends ~trafficPkts packets,
	// far above this threshold.
	ctrlPlaneAllowance = 100

	bgpTimeout  = 2 * time.Minute
	aftTimeout  = 3 * time.Minute
	linkTimeout = time.Minute
	// Upper bounds for event-driven waits (subscriptions), not delays.
	rxSettleTimeout = 5 * time.Second
	neighTimeout    = 30 * time.Second
	counterTimeout  = 90 * time.Second
	// Upper bound for a fixed-packet flow to finish transmitting.
	flowTxTimeout = 2 * time.Minute
	// Overall test deadline.
	testTimeout = 20 * time.Minute

	nhIndexV4, nhgIDV4 = 1, 1
	nhIndexV6, nhgIDV6 = 2, 2
	// staticNHIndex is the IPv6 next-hop static route's next-hop index, unrelated to the gRIBI indices.
	staticNHIndex       = "0"
	aristaIPv6NHAddress = "2001:db8:2::22"
	aristaIPv6NHMAC     = "02:00:00:00:00:01"

	v4RoutesName = "ate1-v4-routes"
	v6RoutesName = "ate1-v6-routes"
)

var (
	dutPort1 = attrs.Attributes{Desc: "dutPort1", IPv4: "192.0.2.1", IPv4Len: 30, IPv6: "2001:db8:1::1", IPv6Len: 126}
	dutPort2 = attrs.Attributes{Desc: "dutPort2", IPv4: "192.0.2.5", IPv4Len: 30, IPv6: "2001:db8:2::1", IPv6Len: 126}
	dutPort3 = attrs.Attributes{Desc: "dutPort3", IPv4: "192.0.2.9", IPv4Len: 30, IPv6: "2001:db8:3::1", IPv6Len: 126}

	atePort1 = attrs.Attributes{Name: "atePort1", MAC: "02:00:01:01:01:01", IPv4: "192.0.2.2", IPv4Len: 30, IPv6: "2001:db8:1::2", IPv6Len: 126}
	atePort2 = attrs.Attributes{Name: "atePort2", MAC: "02:00:02:01:01:01", IPv4: "192.0.2.6", IPv4Len: 30, IPv6: "2001:db8:2::2", IPv6Len: 126}
	atePort3 = attrs.Attributes{Name: "atePort3", MAC: "02:00:03:01:01:01", IPv4: "192.0.2.10", IPv4Len: 30, IPv6: "2001:db8:3::2", IPv6Len: 126}

	// DUT BGP peers (address, peer AS, IPv6 flag).
	bgpPeers = []bgpPeer{
		{addr: atePort1.IPv4, as: ate1AS, v6: false},
		{addr: atePort1.IPv6, as: ate1AS, v6: true},
		{addr: atePort2.IPv4, as: ate2AS, v6: false},
		{addr: atePort2.IPv6, as: ate2AS, v6: true},
	}

	// Every traffic run gets its own fixed-packet flow so counters are absolute
	// and the OTG config never needs to be re-pushed (which would flap BGP).
	flowV4Base      = flowSpec{name: "v4-base", v6: false, rx: []string{"port1"}}
	flowV4Gribi     = flowSpec{name: "v4-gribi", v6: false, rx: []string{"port2"}}
	flowV4Blackhole = flowSpec{name: "v4-blackhole", v6: false, rx: []string{"port1", "port2"}}

	flowV6Base                  = flowSpec{name: "v6-base", v6: true, rx: []string{"port1"}}
	flowV6Gribi                 = flowSpec{name: "v6-gribi", v6: true, rx: []string{"port2"}}
	flowV6Blackhole             = flowSpec{name: "v6-blackhole", v6: true, rx: []string{"port1", "port2"}}
	flowV6Withdrawn             = flowSpec{name: "v6-gribi-withdrawn", v6: true, rx: []string{"port1"}}
	flowV6BlackholeReprogrammed = flowSpec{name: "v6-blackhole-reprogrammed", v6: true, rx: []string{"port1", "port2"}}
	flowV6BlackholeBGPWithdrawn = flowSpec{name: "v6-blackhole-bgp-withdrawn", v6: true, rx: []string{"port1", "port2"}}
	flowV6Recovered             = flowSpec{name: "v6-recovered", v6: true, rx: []string{"port2"}}
	allFlowSpecs                = []flowSpec{flowV4Base, flowV4Gribi, flowV4Blackhole, flowV6Base, flowV6Gribi, flowV6Blackhole, flowV6Withdrawn, flowV6BlackholeReprogrammed, flowV6BlackholeBGPWithdrawn, flowV6Recovered}
	ateDevByPort                = map[string]*attrs.Attributes{"port1": &atePort1, "port2": &atePort2}

	// sentPkts is the running total of packets transmitted by all flows so far.
	sentPkts uint64
)

type bgpPeer struct {
	addr string
	as   uint32
	v6   bool
}

type flowSpec struct {
	name string
	v6   bool
	rx   []string
}

type gribiCtx struct {
	c       *fluent.GRIBIClient
	dut     *ondatra.DUTDevice
	results int
	port2   string
}

// TestMain runs the featureprofiles test harness.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// v4BasePrefix returns the i-th BGP-advertised IPv4 /24 (10.0.0.0/24 upward).
func v4BasePrefix(i int) string { return fmt.Sprintf("10.%d.%d.0/24", i/256, i%256) }

// v4GribiPrefix returns the more-specific gRIBI IPv4 /25 for index i.
func v4GribiPrefix(i int) string { return fmt.Sprintf("10.%d.%d.0/25", i/256, i%256) }

// v4Dst returns a traffic destination inside the i-th gRIBI IPv4 /25.
func v4Dst(i int) string { return fmt.Sprintf("10.%d.%d.1", i/256, i%256) }

// v6BasePrefix returns the i-th BGP-advertised IPv6 /48 (2001:db8:1000::/48 upward).
func v6BasePrefix(i int) string { return fmt.Sprintf("2001:db8:%x::/48", 0x1000+i) }

// v6GribiPrefix returns the more-specific gRIBI IPv6 /64 for index i.
func v6GribiPrefix(i int) string { return fmt.Sprintf("2001:db8:%x:1::/64", 0x1000+i) }

// v6Dst returns a destination inside the i-th gRIBI IPv6 /64 (2001:db8:x:1::1); it is
// also covered by the i-th BGP /48.
func v6Dst(i int) string { return fmt.Sprintf("2001:db8:%x:1::1", 0x1000+i) }

// gen returns numRoutes strings produced by applying f to each index.
func gen(f func(int) string) []string {
	out := make([]string, numRoutes)
	for i := range out {
		out[i] = f(i)
	}
	return out
}

// configureDUT configures port1-3, the accept-all policy and BGP, registering a
// t.Cleanup for each item it adds.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	for _, p := range []struct {
		port string
		a    *attrs.Attributes
	}{{"port1", &dutPort1}, {"port2", &dutPort2}, {"port3", &dutPort3}} {
		dp := dut.Port(t, p.port)
		ifName := dp.Name()
		orig := gnmi.Lookup(t, dut, gnmi.OC().Interface(ifName).Config())
		gnmi.Replace(t, dut, gnmi.OC().Interface(ifName).Config(), p.a.NewOCInterface(ifName, dut))
		t.Cleanup(func() {
			if cfg, ok := orig.Val(); ok {
				gnmi.Replace(t, dut, gnmi.OC().Interface(ifName).Config(), cfg)
			} else {
				gnmi.Delete(t, dut, gnmi.OC().Interface(ifName).Config())
			}
		})
		if deviations.ExplicitPortSpeed(dut) {
			fptest.SetPortSpeed(t, dp)
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, dp.Name(), ni, 0)
		}
	}

	root := &oc.Root{}

	rp := root.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition("PERMIT-ALL")
	st, err := pd.AppendNewStatement("10")
	if err != nil {
		t.Fatalf("AppendNewStatement: %v", err)
	}
	st.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
	t.Cleanup(func() { gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition("PERMIT-ALL").Config()) })

	proto := root.GetOrCreateNetworkInstance(ni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut))
	bgp := proto.GetOrCreateBgp()
	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAS)
	g.RouterId = ygot.String(dutPort1.IPv4)
	for _, afi := range []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST} {
		g.GetOrCreateAfiSafi(afi).Enabled = ygot.Bool(true)
	}
	for _, p := range bgpPeers {
		n := bgp.GetOrCreateNeighbor(p.addr)
		n.PeerAs = ygot.Uint32(p.as)
		n.Enabled = ygot.Bool(true)
		afi := oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
		if p.v6 {
			afi = oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
		}
		af := n.GetOrCreateAfiSafi(afi)
		af.Enabled = ygot.Bool(true)
		ap := af.GetOrCreateApplyPolicy()
		ap.SetImportPolicy([]string{"PERMIT-ALL"})
		ap.SetExportPolicy([]string{"PERMIT-ALL"})
	}
	bgpCfg := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Config()
	gnmi.Replace(t, dut, bgpCfg, proto)
	t.Cleanup(func() { gnmi.Delete(t, dut, bgpCfg) })
}

// operStatus returns the DUT interface oper-status, or "<absent>" for diagnostics.
func operStatus(t *testing.T, dut *ondatra.DUTDevice, name string) string {
	t.Helper()
	if v, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(name).OperStatus().State()).Val(); ok {
		return v.String()
	}
	return "<absent>"
}

// awaitLinksUp waits via subscription for DUT port1-3 to be oper-up, so link
// problems are not misreported as BGP timeouts.
func awaitLinksUp(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	for _, pn := range []string{"port1", "port2", "port3"} {
		name := dut.Port(t, pn).Name()
		if _, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(name).OperStatus().State(), linkTimeout,
			func(v *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
				s, present := v.Val()
				return present && s == oc.Interface_OperStatus_UP
			}).Await(t); !ok {
			t.Fatalf("DUT %s (%s) oper-status is %s after %v, want UP: check port speed / cabling", pn, name, operStatus(t, dut, name), linkTimeout)
		}
	}
}

// awaitBGP waits via subscriptions for all DUT BGP sessions to be ESTABLISHED and
// reports the last observed state on failure.
func awaitBGP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	bgpPath := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, deviations.DefaultBgpInstanceName(dut)).Bgp()
	for _, p := range bgpPeers {
		val, ok := gnmi.Watch(t, dut, bgpPath.Neighbor(p.addr).SessionState().State(), bgpTimeout,
			func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
				s, present := v.Val()
				return present && s == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			}).Await(t)
		if !ok {
			got, _ := val.Val()
			for _, pn := range []string{"port1", "port2", "port3"} {
				name := dut.Port(t, pn).Name()
				t.Logf("DUT %s (%s): oper-status %s", pn, name, operStatus(t, dut, name))
			}
			t.Fatalf("BGP session to %s did not reach ESTABLISHED within %v (last state: %v)", p.addr, bgpTimeout, got)
		}
	}
}

// setPortEnabled toggles a DUT port administratively and waits for oper-status.
func setPortEnabled(t *testing.T, dut *ondatra.DUTDevice, name string, enabled bool) {
	t.Helper()
	gnmi.Replace(t, dut, gnmi.OC().Interface(name).Enabled().Config(), enabled)
	want := oc.Interface_OperStatus_DOWN
	if enabled {
		want = oc.Interface_OperStatus_UP
	}
	if _, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(name).OperStatus().State(), linkTimeout,
		func(v *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			s, present := v.Val()
			return present && s == want
		}).Await(t); !ok {
		t.Fatalf("interface %s oper-status did not reach %v", name, want)
	}
}

// awaitNeighborResolved waits for the DUT to report a neighbor entry for addr on the port.
func awaitNeighborResolved(t *testing.T, dut *ondatra.DUTDevice, port, addr string, v6 bool) bool {
	t.Helper()
	sub := gnmi.OC().Interface(port).Subinterface(0)
	present := func(v *ygnmi.Value[string]) bool { _, ok := v.Val(); return ok }
	neigh := sub.Ipv4().Neighbor(addr).LinkLayerAddress().State()
	if v6 {
		neigh = sub.Ipv6().Neighbor(addr).LinkLayerAddress().State()
	}
	if _, ok := gnmi.Watch(t, dut, neigh, neighTimeout, present).Await(t); !ok {
		t.Logf("neighbor %s on %s not reported within %v; continuing", addr, port, neighTimeout)
		return false
	}
	return true
}

func configureAristaIPv6NextHop(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if !deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		return
	}

	port := dut.Port(t, "port2")
	ni := deviations.DefaultNetworkInstance(dut)
	staticName := deviations.StaticProtocolName(dut)
	prefix := aristaIPv6NHAddress + "/128"
	nhIndex := staticNHIndex
	static := &oc.NetworkInstance_Protocol_Static{
		Prefix: ygot.String(prefix),
		NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
			nhIndex: {
				Index: ygot.String(nhIndex),
				InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
					Interface: ygot.String(port.Name()),
				},
			},
		},
	}
	proto := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		Name:       ygot.String(staticName),
		Static:     map[string]*oc.NetworkInstance_Protocol_Static{prefix: static},
	}
	staticPath := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, staticName)
	gnmi.Update(t, dut, staticPath.Config(), proto)

	intf := &oc.Interface{Name: ygot.String(port.Name()), Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd}
	intf.GetOrCreateSubinterface(0).GetOrCreateIpv6().GetOrCreateNeighbor(aristaIPv6NHAddress).LinkLayerAddress = ygot.String(aristaIPv6NHMAC)
	gnmi.Update(t, dut, gnmi.OC().Interface(port.Name()).Config(), intf)
	t.Cleanup(func() {
		gnmi.Delete(t, dut, staticPath.Static(prefix).Config())
		gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Subinterface(0).Ipv6().Neighbor(aristaIPv6NHAddress).Config())
	})
}

// addBGP adds eBGP peers to an ATE device, advertising the underlay routes when
// advertise is set.
func addBGP(dev gosnappi.Device, a, dutA *attrs.Attributes, as uint32, advertise bool) {
	peerOpts := func(name, addr string) []otgconfighelpers.BGPPeerOption {
		return []otgconfighelpers.BGPPeerOption{
			otgconfighelpers.WithBGPName(name),
			otgconfighelpers.WithBGPPeerAddress(addr),
			otgconfighelpers.WithBGPASNumber(as),
			otgconfighelpers.WithBGPEBGP(),
			otgconfighelpers.WithBGPRouterID(a.IPv4),
			otgconfighelpers.WithoutBGPGR(),
			otgconfighelpers.WithBGPTimers(90, 30),
		}
	}
	p4 := otgconfighelpers.AddBGPV4Peer(dev, a.Name+".IPv4", peerOpts(a.Name+".BGP4.peer", dutA.IPv4)...)
	p6 := otgconfighelpers.AddBGPV6Peer(dev, a.Name+".IPv6", peerOpts(a.Name+".BGP6.peer", dutA.IPv6)...)

	if !advertise {
		return
	}
	otgconfighelpers.AddBGPV4Routes(p4, v4RoutesName, []string{"10.0.0.0/24"},
		otgconfighelpers.WithBGPRouteAddressCount(numRoutes),
		otgconfighelpers.WithBGPRouteNextHopIPv4(a.IPv4),
		otgconfighelpers.WithBGPRouteNextHopMode("MANUAL"))
	otgconfighelpers.AddBGPV6Routes(p6, v6RoutesName, []string{"2001:db8:1000::/48"},
		otgconfighelpers.WithBGPRouteAddressCount(numRoutes),
		otgconfighelpers.WithBGPRouteNextHopIPv6(a.IPv6),
		otgconfighelpers.WithBGPRouteNextHopMode("MANUAL"))
}

// addFlow adds a fixed-packet flow from ATE port3 cycling through all destinations.
func addFlow(top gosnappi.Config, fs flowSpec) {
	suffix := ".IPv4"
	if fs.v6 {
		suffix = ".IPv6"
	}
	rx := make([]string, 0, len(fs.rx))
	for _, p := range fs.rx {
		rx = append(rx, ateDevByPort[p].Name+suffix)
	}
	f := top.Flows().Add().SetName(fs.name)
	f.Metrics().SetEnable(true)
	f.TxRx().Device().SetTxNames([]string{atePort3.Name + suffix}).SetRxNames(rx)
	f.Size().SetFixed(pktSize)
	f.Rate().SetPps(trafficPPS)
	f.Duration().FixedPackets().SetPackets(trafficPkts)

	eth := f.Packet().Add().Ethernet()
	eth.Src().SetValue(atePort3.MAC)
	eth.Dst().Auto()

	if fs.v6 {
		ip := f.Packet().Add().Ipv6()
		ip.Src().SetValue(atePort3.IPv6)
		ip.Dst().SetValues(gen(v6Dst))
	} else {
		ip := f.Packet().Add().Ipv4()
		ip.Src().SetValue(atePort3.IPv4)
		ip.Dst().SetValues(gen(v4Dst))
	}
}

// configureATE builds the OTG topology: three ports, BGP on ports 1-2 and all flows.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	ap1, ap2, ap3 := ate.Port(t, "port1"), ate.Port(t, "port2"), ate.Port(t, "port3")

	d1 := atePort1.AddToOTG(top, ap1, &dutPort1)
	d2 := atePort2.AddToOTG(top, ap2, &dutPort2)
	atePort3.AddToOTG(top, ap3, &dutPort3)

	addBGP(d1, &atePort1, &dutPort1, ate1AS, true)
	addBGP(d2, &atePort2, &dutPort2, ate2AS, false)

	for _, fs := range allFlowSpecs {
		addFlow(top, fs)
	}
	return top
}

// setRoutes advertises or withdraws the named ATE BGP route range.
func setRoutes(t *testing.T, ate *ondatra.ATEDevice, name string, advertise bool) {
	t.Helper()
	state := gosnappi.StateProtocolRouteState.WITHDRAW
	if advertise {
		state = gosnappi.StateProtocolRouteState.ADVERTISE
	}
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames([]string{name}).SetState(state)
	ate.OTG().SetControlState(t, cs)
}

// runFlow transmits the flow's fixed packet count and returns the tx and rx counters;
// the transmitted count is added to sentPkts.
func runFlow(t *testing.T, ate *ondatra.ATEDevice, fs flowSpec) (tx, rx uint64) {
	t.Helper()
	otg := ate.OTG()

	cs := gosnappi.NewControlState()
	cs.Traffic().FlowTransmit().SetFlowNames([]string{fs.name}).SetState(gosnappi.StateTrafficFlowTransmitState.START)
	otg.SetControlState(t, cs)
	defer func() {
		stop := gosnappi.NewControlState()
		stop.Traffic().FlowTransmit().SetFlowNames([]string{fs.name}).SetState(gosnappi.StateTrafficFlowTransmitState.STOP)
		otg.SetControlState(t, stop)
	}()

	txVal, ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(fs.name).Counters().OutPkts().State(), flowTxTimeout,
		func(v *ygnmi.Value[uint64]) bool { n, ok := v.Val(); return ok && n >= trafficPkts }).Await(t)
	tx, _ = txVal.Val()
	if !ok {
		t.Fatalf("flow %s: transmitted %d of %d packets within %v", fs.name, tx, trafficPkts, flowTxTimeout)
	}

	rxVal, _ := gnmi.Watch(t, otg, gnmi.OTG().Flow(fs.name).Counters().InPkts().State(), rxSettleTimeout,
		func(v *ygnmi.Value[uint64]) bool { n, ok := v.Val(); return ok && n >= tx }).Await(t)
	rx, _ = rxVal.Val()

	if tx == 0 {
		t.Fatalf("flow %s: no packets transmitted", fs.name)
	}
	sentPkts += tx
	return tx, rx
}

// requireNoLoss runs the flow and fails if loss exceeds lossTolerancePct.
func requireNoLoss(t *testing.T, ate *ondatra.ATEDevice, fs flowSpec) {
	t.Helper()
	tx, rx := runFlow(t, ate, fs)
	loss := float64(int64(tx)-int64(rx)) / float64(tx) * 100
	t.Logf("flow %s (rx on %v): tx=%d rx=%d loss=%.3f%%", fs.name, fs.rx, tx, rx, loss)
	if loss > lossTolerancePct {
		t.Errorf("flow %s: loss %.3f%% exceeds tolerance %.1f%%", fs.name, loss, lossTolerancePct)
	}
}

// aftPrefixes returns the normalized IPv4 or IPv6 prefixes currently in the DUT AFT.
func aftPrefixes(t *testing.T, dut *ondatra.DUTDevice, v6 bool) map[string]bool {
	t.Helper()
	ni := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	list := make([]string, 0)
	if v6 {
		for _, entry := range gnmi.LookupAll(t, dut, ni.Afts().Ipv6EntryAny().State()) {
			if e, ok := entry.Val(); ok && e != nil {
				list = append(list, e.GetPrefix())
			}
		}
	} else {
		for _, entry := range gnmi.LookupAll(t, dut, ni.Afts().Ipv4EntryAny().State()) {
			if e, ok := entry.Val(); ok && e != nil {
				list = append(list, e.GetPrefix())
			}
		}
	}
	m := make(map[string]bool, len(list))
	for _, p := range list {
		if pf, err := netip.ParsePrefix(p); err == nil {
			m[pf.String()] = true
		}
	}
	return m
}

// aftEvent returns the prefix key and presence for an AFT entry notification, taking
// the key from the path for deletions.
func aftEvent[T interface{ GetPrefix() string }](v *ygnmi.Value[T]) (string, bool) {
	if e, ok := v.Val(); ok {
		return e.GetPrefix(), true
	}
	if v.Path != nil {
		for _, el := range v.Path.GetElem() {
			if p, ok := el.GetKey()["prefix"]; ok {
				return p, false
			}
		}
	}
	return "", false
}

// awaitAFT waits via a streaming subscription until all prefixes are present or
// absent in the DUT AFT.
func awaitAFT(t *testing.T, dut *ondatra.DUTDevice, prefixes []string, v6, present bool) {
	t.Helper()
	got := aftPrefixes(t, dut, v6)
	wrong := func() int {
		n := 0
		for _, p := range prefixes {
			if pf, err := netip.ParsePrefix(p); err == nil && got[pf.String()] != present {
				n++
			}
		}
		return n
	}
	if wrong() == 0 {
		return
	}
	apply := func(pfx string, isPresent bool) bool {
		if pf, err := netip.ParsePrefix(pfx); err == nil {
			if isPresent {
				got[pf.String()] = true
			} else {
				delete(got, pf.String())
			}
		}
		return wrong() == 0
	}
	afts := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts()
	if v6 {
		gnmi.WatchAll(t, dut, afts.Ipv6EntryAny().State(), aftTimeout,
			func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool { return apply(aftEvent(v)) }).Await(t)
	} else {
		gnmi.WatchAll(t, dut, afts.Ipv4EntryAny().State(), aftTimeout,
			func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool { return apply(aftEvent(v)) }).Await(t)
	}
	if n := wrong(); n != 0 {
		got = aftPrefixes(t, dut, v6)
		if n = wrong(); n != 0 {
			t.Fatalf("AFT: %d/%d prefixes not %s after %v", n, len(prefixes), map[bool]string{true: "present", false: "removed"}[present], aftTimeout)
		}
	}
}

// newGRIBI starts an elected gRIBI client and flushes stale entries.
func newGRIBI(t *testing.T, dut *ondatra.DUTDevice) *gribiCtx {
	t.Helper()
	c := &gribi.Client{DUT: dut, FIBACK: true, Persistence: true}
	t.Cleanup(func() { c.Close(t) })
	if err := c.Start(t); err != nil {
		t.Fatalf("gRIBI client did not become ready: %v", err)
	}
	c.BecomeLeader(t)
	c.FlushAll(t)
	fc := c.Fluent(t)
	return &gribiCtx{c: fc, dut: dut, port2: dut.Port(t, "port2").Name(), results: len(fc.Results(t))}
}

// flush removes all gRIBI entries in all network instances using election override.
func (g *gribiCtx) flush(t *testing.T) {
	t.Helper()
	if err := gribi.FlushAll(g.c); err != nil {
		t.Errorf("gRIBI flush: %v", err)
	}
}

// nhEntries returns the next-hop and next-hop-group entries pointing at ATE port2.
func (g *gribiCtx) nhEntries(v6 bool) []fluent.GRIBIEntry {
	ni := deviations.DefaultNetworkInstance(g.dut)
	idx, id, ip := uint64(nhIndexV4), uint64(nhgIDV4), atePort2.IPv4
	if v6 {
		idx, id, ip = nhIndexV6, nhgIDV6, atePort2.IPv6
	}
	nh := fluent.NextHopEntry().WithNetworkInstance(ni).WithIndex(idx)
	if v6 && deviations.GRIBIMACOverrideStaticARPStaticRoute(g.dut) {
		nh.WithInterfaceRef(g.port2).WithIPAddress(aristaIPv6NHAddress).WithMacAddress(aristaIPv6NHMAC)
	} else {
		nh.WithIPAddress(ip)
	}
	return []fluent.GRIBIEntry{
		nh,
		fluent.NextHopGroupEntry().WithNetworkInstance(ni).WithID(id).AddNextHop(idx, 1),
	}
}

// routeEntries returns the numRoutes more-specific gRIBI route entries.
func (g *gribiCtx) routeEntries(v6 bool) []fluent.GRIBIEntry {
	ni := deviations.DefaultNetworkInstance(g.dut)
	out := make([]fluent.GRIBIEntry, 0, numRoutes)
	for i := range numRoutes {
		if v6 {
			out = append(out, fluent.IPv6Entry().WithNetworkInstance(ni).WithPrefix(v6GribiPrefix(i)).
				WithNextHopGroup(nhgIDV6).WithNextHopGroupNetworkInstance(ni))
		} else {
			out = append(out, fluent.IPv4Entry().WithNetworkInstance(ni).WithPrefix(v4GribiPrefix(i)).
				WithNextHopGroup(nhgIDV4).WithNextHopGroupNetworkInstance(ni))
		}
	}
	return out
}

// await waits for pending gRIBI ops and requires route entries to reach FIB_PROGRAMMED
// (or RIB_PROGRAMMED when ribOK);
// next-hop results are informational and FAILED is always fatal.
func (g *gribiCtx) await(ctx context.Context, t *testing.T, ribOK bool) {
	t.Helper()
	if err := g.c.Await(ctx, t); err != nil {
		t.Fatalf("gRIBI Await: %v", err)
	}
	res := g.c.Results(t)
	batch := res[g.results:]
	g.results = len(res)
	latest := map[string]spb.AFTResult_Status{}
	isRoute := map[string]bool{}
	for _, r := range batch {
		key := fmt.Sprintf("%d", r.OperationID)
		route := false
		if d := r.Details; d != nil {
			route = d.IPv4Prefix != "" || d.IPv6Prefix != ""
			key = fmt.Sprintf("%v/nh%d/nhg%d/%s/%s", d.Type, d.NextHopIndex, d.NextHopGroupID, d.IPv4Prefix, d.IPv6Prefix)
		}
		latest[key] = r.ProgrammingResult
		isRoute[key] = route
	}
	for k, st := range latest {
		switch {
		case st == spb.AFTResult_FIB_PROGRAMMED, ribOK && st == spb.AFTResult_RIB_PROGRAMMED:
		case !isRoute[k] && st != spb.AFTResult_FAILED:
			if st != spb.AFTResult_RIB_PROGRAMMED {
				t.Logf("gRIBI non-route op %s status %v (informational)", k, st)
			}
		default:
			for _, r := range batch {
				t.Logf("gRIBI result: %v", r)
			}
			t.Fatalf("gRIBI op %s not FIB_PROGRAMMED: status %v", k, st)
		}
	}
}

// add sends the entries to the DUT and validates their programming results.
func (g *gribiCtx) add(ctx context.Context, t *testing.T, e []fluent.GRIBIEntry) {
	t.Helper()
	g.c.Modify().AddEntry(t, e...)
	g.await(ctx, t, false)
}

// addRIBOK is add but also accepts RIB_PROGRAMMED for routes, for entries whose next-hop is down.
func (g *gribiCtx) addRIBOK(ctx context.Context, t *testing.T, e []fluent.GRIBIEntry) {
	t.Helper()
	g.c.Modify().AddEntry(t, e...)
	g.await(ctx, t, true)
}

// del deletes the entries from the DUT and validates the programming results.
func (g *gribiCtx) del(ctx context.Context, t *testing.T, e []fluent.GRIBIEntry) {
	t.Helper()
	g.c.Modify().DeleteEntry(t, e...)
	g.await(ctx, t, false)
}

// awaitInPkts waits up to counterTimeout for the DUT port in-pkts counter to reach want.
func awaitInPkts(t *testing.T, dut *ondatra.DUTDevice, port string, want uint64) {
	t.Helper()
	_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(port).Counters().InPkts().State(), counterTimeout,
		func(v *ygnmi.Value[uint64]) bool {
			n, present := v.Val()
			return present && n >= want
		}).Await(t)
	if !ok {
		t.Logf("DUT %s in-pkts did not reach %d within %v", port, want, counterTimeout)
	}
}

// verifyBlackhole checks 100% loss, no port1/port2 egress, port3 ingress and that the gRIBI
// routes remain in the AFT; in3Start is the DUT port3 in-pkts counter before any traffic.
func verifyBlackhole(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, fs flowSpec, gribiPrefixes []string, v6 bool, in3Start uint64) {
	t.Helper()
	p1, p2, p3 := dut.Port(t, "port1").Name(), dut.Port(t, "port2").Name(), dut.Port(t, "port3").Name()
	counters := func() *oc.Root {
		b := gnmi.OCBatch()
		for _, n := range []string{p1, p2, p3} {
			b.AddPaths(gnmi.OC().Interface(n).Counters())
		}
		return gnmi.Get(t, dut, b.State())
	}
	out := func(r *oc.Root, n string) uint64 { return r.GetInterface(n).GetCounters().GetOutPkts() }
	in := func(r *oc.Root, n string) uint64 { return r.GetInterface(n).GetCounters().GetInPkts() }

	awaitInPkts(t, dut, p3, in3Start+sentPkts)
	before := counters()
	tx, rx := runFlow(t, ate, fs)
	awaitInPkts(t, dut, p3, in(before, p3)+tx)
	after := counters()

	t.Logf("flow %s: tx=%d rx=%d", fs.name, tx, rx)
	if rx != 0 {
		t.Errorf("flow %s: ATE received %d packets, want 0 (100%% blackhole)", fs.name, rx)
	}
	d1, d2, d3 := out(after, p1)-out(before, p1), out(after, p2)-out(before, p2), in(after, p3)-in(before, p3)
	t.Logf("flow %s: DUT out-pkts delta %s=%d %s=%d, in-pkts delta %s=%d", fs.name, p1, d1, p2, d2, p3, d3)
	if d1 > ctrlPlaneAllowance {
		t.Errorf("DUT %s out-pkts grew by %d during blackhole run (>%d): traffic leaked to BGP underlay", p1, d1, ctrlPlaneAllowance)
	}
	if d2 > ctrlPlaneAllowance {
		t.Errorf("DUT %s out-pkts grew by %d during blackhole run (>%d)", p2, d2, ctrlPlaneAllowance)
	}
	if d3 < tx {
		t.Errorf("DUT %s in-pkts grew by %d, want >= %d (traffic must enter the hardware)", p3, d3, tx)
	}
	awaitAFT(t, dut, gribiPrefixes, v6, true)
}

// resetState re-enables port2, flushes gRIBI and waits for BGP to restore the baseline.
func resetState(t *testing.T, dut *ondatra.DUTDevice, g *gribiCtx) {
	t.Helper()
	setPortEnabled(t, dut, dut.Port(t, "port2").Name(), true)
	g.flush(t)
	awaitBGP(t, dut)
}

// TestStrictFIBPrecedence implements TE-1.22.1 through TE-1.22.4.
func TestStrictFIBPrecedence(t *testing.T) {
	// t.Context() is cancelled before t.Cleanup functions run, which would break the
	// gRIBI flush in cleanup; WithoutCancel keeps its values but not its cancellation.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), testTimeout)
	t.Cleanup(cancel)

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	port2 := dut.Port(t, "port2").Name()

	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)
	configureDUT(t, dut)
	ate.OTG().StartProtocols(t)
	awaitLinksUp(t, dut)
	awaitBGP(t, dut)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
	configureAristaIPv6NextHop(t, dut)

	in3Start := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port3").Name()).Counters().InPkts().State())

	g := newGRIBI(t, dut)
	t.Cleanup(func() {
		setPortEnabled(t, dut, port2, true)
		g.flush(t)
		setRoutes(t, ate, v6RoutesName, true)
	})

	v4Base, v4Gribi := gen(v4BasePrefix), gen(v4GribiPrefix)
	v6Base, v6Gribi := gen(v6BasePrefix), gen(v6GribiPrefix)

	t.Run("TE_1_22_1_IPv4_StrictFIBPrecedence", func(t *testing.T) {
		t.Cleanup(func() { resetState(t, dut, g) })

		t.Log("Step 1: base BGP routes + traffic to ATE port1")
		awaitAFT(t, dut, v4Base, false, true)
		requireNoLoss(t, ate, flowV4Base)

		t.Log("Step 2: program gRIBI more-specific routes -> ATE port2")
		g.add(ctx, t, g.nhEntries(false))
		g.add(ctx, t, g.routeEntries(false))
		awaitAFT(t, dut, v4Gribi, false, true)

		t.Log("Step 3: traffic shifts to ATE port2")
		requireNoLoss(t, ate, flowV4Gribi)

		t.Log("Step 4: take DUT port2 down")
		setPortEnabled(t, dut, port2, false)

		t.Log("Step 5: expect 100% blackhole")
		verifyBlackhole(t, dut, ate, flowV4Blackhole, v4Gribi, false, in3Start)
	})

	ok := t.Run("TE_1_22_2_IPv6_StrictFIBPrecedence", func(t *testing.T) {
		t.Log("Step 1: base BGP routes + traffic to ATE port1")
		awaitAFT(t, dut, v6Base, true, true)
		if !awaitNeighborResolved(t, dut, dut.Port(t, "port1").Name(), atePort1.IPv6, true) {
			t.Fatalf("IPv6 next-hop neighbor %s on port1 was not resolved", atePort1.IPv6)
		}
		requireNoLoss(t, ate, flowV6Base)

		t.Log("Step 2: program gRIBI more-specific routes -> ATE port2")
		g.add(ctx, t, g.nhEntries(true))
		g.add(ctx, t, g.routeEntries(true))
		awaitAFT(t, dut, v6Gribi, true, true)

		t.Log("Step 3: traffic shifts to ATE port2")
		requireNoLoss(t, ate, flowV6Gribi)

		t.Log("Step 4: take DUT port2 down")
		setPortEnabled(t, dut, port2, false)

		t.Log("Step 5: expect 100% blackhole")
		verifyBlackhole(t, dut, ate, flowV6Blackhole, v6Gribi, true, in3Start)
	})
	if !ok {
		t.Fatal("TE-1.22.3/4 depend on the final state of TE-1.22.2, which failed")
	}

	t.Run("TE_1_22_3_WithdrawGRIBIDuringBlackhole", func(t *testing.T) {
		t.Log("Step 2: delete gRIBI IPv6 routes")
		g.del(ctx, t, g.routeEntries(true))
		awaitAFT(t, dut, v6Gribi, true, false)

		t.Log("Step 3: traffic resumes via BGP underlay on DUT port1")
		requireNoLoss(t, ate, flowV6Withdrawn)
	})

	t.Run("TE_1_22_4_WithdrawBGPAndPortRecovery", func(t *testing.T) {
		t.Log("Step 1: re-program gRIBI routes with port2 still down")
		g.addRIBOK(ctx, t, g.routeEntries(true))
		awaitAFT(t, dut, v6Gribi, true, true)
		verifyBlackhole(t, dut, ate, flowV6BlackholeReprogrammed, v6Gribi, true, in3Start)

		t.Log("Step 2: withdraw BGP underlay routes from ATE port1")
		setRoutes(t, ate, v6RoutesName, false)
		awaitAFT(t, dut, v6Base, true, false)

		t.Log("Step 3: traffic must still be blackholed by gRIBI routes")
		verifyBlackhole(t, dut, ate, flowV6BlackholeBGPWithdrawn, v6Gribi, true, in3Start)

		t.Log("Step 4: bring DUT port2 back up")
		setPortEnabled(t, dut, port2, true)
		awaitNeighborResolved(t, dut, port2, atePort2.IPv6, true)
		awaitBGP(t, dut)

		t.Log("Step 5: gRIBI routes recover independently of BGP underlay")
		awaitAFT(t, dut, v6Gribi, true, true)
		requireNoLoss(t, ate, flowV6Recovered)
	})
}
