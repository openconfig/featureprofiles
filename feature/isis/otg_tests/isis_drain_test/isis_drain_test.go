// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package isis_drain_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstisis"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	plen4        = 30
	plen6        = 126
	isisInstance = "DEFAULT"
	bgpName      = "BGP"
	peerGrpName  = "BGP-PEER-GROUP"
	dutAS        = 64496
	areaAddress  = "49.0001"
	sysID        = "1920.0000.2001"

	v4Route             = "100.64.0.0"
	v4RoutePlen         = 24
	v4IP                = "100.64.0.1"
	v6Route             = "2001:db8:2::"
	v6RoutePlen         = 64
	v6IP                = "2001:db8:2::1"
	routeCount          = 1000
	baseMetric          = 10
	drainMetric         = 1000
	lag2MAC             = "02:aa:bb:02:00:02"
	lag3MAC             = "02:aa:bb:03:00:02"
	otgPort1sysID       = "640000000001"
	otgLAG2sysID        = "640000000002"
	otgLAG3sysID        = "640000000003"
	maxEcmpPaths  uint8 = 16

	v4PrefixStep = "0.0.1.0"
	v6PrefixStep = "0:0:0:1::"

	port1ID = "port1"
	port2ID = "port2"
	port3ID = "port3"

	ateLag2 = "lag2"
	ateLag3 = "lag3"

	// Traffic flow parameters.
	flowSizeBytes   = 300
	tcpDstPortStart = 12345
	tcpDstPortCount = 200

	trafficPPS      = 1000
	localTrafficPPS = 100
	runTrafficTime  = 10 * time.Second

	trafficTolerance        = 0.99
	loadBalanceTolerancePct = 10.0

	// Traffic and telemetry timing.
	watchTimeout       = 30 * time.Second
	stabilityWindow    = 30 * time.Second
	stateWatchTimeout  = 1 * time.Minute
	trafficWaitTimeout = 6 * runTrafficTime
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT port 1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	atePort1 = attrs.Attributes{
		Name:    "ATEport1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plen4,
		IPv6Len: plen6,
		MAC:     "02:aa:bb:01:00:01",
	}

	dutPort2 = attrs.Attributes{
		Desc:    "DUT port 2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	atePort2 = attrs.Attributes{
		Name:    "ATEport2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
		MAC:     "02:aa:bb:02:00:01",
	}

	dutPort3 = attrs.Attributes{
		Desc:    "DUT port 3",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::9",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	atePort3 = attrs.Attributes{
		Name:    "ATEport3",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::a",
		IPv4Len: plen4,
		IPv6Len: plen6,
		MAC:     "02:aa:bb:03:00:01",
	}

	dutLoopback = attrs.Attributes{
		Desc:    "DUT Loopback",
		IPv4:    "192.0.2.21",
		IPv6:    "2001:db8::15",
		IPv4Len: 32,
		IPv6Len: 128,
	}

	dutPort1Name, agg2ID, agg3ID, loopbackIntfName string

	v4RouteCIDR = fmt.Sprintf("%s/%d", v4Route, v4RoutePlen)
	v6RouteCIDR = fmt.Sprintf("%s/%d", v6Route, v6RoutePlen)
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	// configure port 1
	p1 := dut.Port(t, port1ID)
	dutPort1Name = p1.Name()

	intfBatch := &gnmi.SetBatch{}
	i1 := dutPort1.NewOCInterface(p1.Name(), dut)
	gnmi.BatchReplace(intfBatch, gnmi.OC().Interface(p1.Name()).Config(), i1)

	// configure trunks 2 and 3 (LAGs)
	trunks := []struct {
		portID   string
		dutAttrs *attrs.Attributes
		aggID    *string
	}{
		{port2ID, &dutPort2, &agg2ID},
		{port3ID, &dutPort3, &agg3ID},
	}

	aggIDs, err := cfgplugins.NextAggregates(t, dut, len(trunks))
	if err != nil {
		t.Fatalf("Failed to get aggregate IDs: %v", err)
	}
	for index, tr := range trunks {
		p := dut.Port(t, tr.portID)
		*tr.aggID = aggIDs[index]
		t.Logf("Adding port: %s to Aggregate: %s", p.Name(), *tr.aggID)

		cfgplugins.NewAggregateInterface(t, dut, intfBatch, &cfgplugins.DUTAggData{
			Attributes:   *tr.dutAttrs,
			OndatraPorts: []*ondatra.Port{p},
			LagName:      *tr.aggID,
			AggType:      oc.IfAggregate_AggregationType_STATIC,
		})
	}

	// configure the loopback that is advertised into IS-IS and used to source iBGP.
	loopbackIntfName = netutil.LoopbackInterface(t, dut, 0)
	loop1 := dutLoopback.NewOCInterface(loopbackIntfName, dut)
	loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
	gnmi.BatchReplace(intfBatch, gnmi.OC().Interface(loopbackIntfName).Config(), loop1)
	t.Logf("Created loopback interface %s with IPv4=%s IPv6=%s", loopbackIntfName, dutLoopback.IPv4, dutLoopback.IPv6)
	intfBatch.Set(t, dut)

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, agg2ID, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, agg3ID, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, loopbackIntfName, deviations.DefaultNetworkInstance(dut), 0)
	}

	if deviations.ExplicitPortSpeed(dut) {
		for _, port := range dut.Ports() {
			fptest.SetPortSpeed(t, port)
		}
	}

	configBatch := &gnmi.SetBatch{}
	// configure ISIS
	cfgplugins.NewISISWithMetric(t, dut, configBatch, cfgplugins.ISISMetricParams{
		InstanceName: isisInstance,
		AreaAddress:  areaAddress,
		SystemID:     sysID,
		MaxEcmpPaths: maxEcmpPaths,
		BaseMetric:   baseMetric,
		Interfaces:   []string{p1.Name(), agg2ID, agg3ID},
		LoopbackIntf: loopbackIntfName,
		LspOverload:  false,
	})

	// configure the iBGP session towards ATE port-1 to emulate control plane traffic
	configureBGPDUT(t, dut, configBatch)
	configBatch.Set(t, dut)
}

func configureBGPDUT(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
	prot.Enabled = ygot.Bool(true)
	bgp := prot.GetOrCreateBgp()

	cfgplugins.ConfigureGlobal(bgp, dut,
		cfgplugins.WithAS(dutAS),
		cfgplugins.WithRouterID(dutLoopback.IPv4),
		cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, true),
	)

	localAddress := dutLoopback.IPv4
	if deviations.UseInterfaceNameForIBGPNeighborTransportIpv4LocalAddress(dut) {
		localAddress = loopbackIntfName
	}
	pg := bgp.GetOrCreatePeerGroup(peerGrpName)
	cfgplugins.ConfigurePeerGroup(pg, dut,
		cfgplugins.WithPeerAS(dutAS),
		cfgplugins.WithPGAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, true, false),
	)

	nbr := bgp.GetOrCreateNeighbor(atePort1.IPv4)
	cfgplugins.ConfigurePeer(nbr, dut,
		cfgplugins.WithPeerGroup(peerGrpName, dutAS, "", "", false),
		cfgplugins.WithPeerAfiSafiEnabled(true, "", "", false),
		cfgplugins.WithPeerTransport(localAddress),
	)

	gnmi.BatchUpdate(batch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Config(), prot)
}

func configureATE() gosnappi.Config {
	cfg := gosnappi.NewConfig()
	p1 := cfg.Ports().Add().SetName(port1ID)
	p2 := cfg.Ports().Add().SetName(port2ID)
	p3 := cfg.Ports().Add().SetName(port3ID)

	// configure port 1 - src
	i1 := cfg.Devices().Add().SetName(atePort1.Name)
	i1Eth := i1.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	i1Eth.Connection().SetPortName(p1.Name())
	i1IPv4 := i1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + cfgplugins.IPv4)
	i1IPv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(plen4)
	i1IPv6 := i1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + cfgplugins.IPv6)
	i1IPv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(plen6)

	// configure ISIS on port 1 so that the ATE learns the DUT loopback
	p1Isis := i1.Isis().SetSystemId(otgPort1sysID).SetName("port1-isis")
	p1Isis.Basic().SetHostname(p1Isis.Name()).SetLearnedLspFilter(true)
	p1Isis.Advanced().SetAreaAddresses([]string{strings.Replace(areaAddress, ".", "", -1)})

	p1IsisInt := p1Isis.Interfaces().Add().
		SetEthName(i1Eth.Name()).SetName("port1IsisInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(baseMetric)
	p1IsisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	// configure the iBGP session from port 1 to the DUT loopback
	p1Bgp := i1.Bgp().SetRouterId(atePort1.IPv4)
	p1BgpPeer := p1Bgp.Ipv4Interfaces().Add().SetIpv4Name(i1IPv4.Name()).
		Peers().Add().SetName(atePort1.Name + ".BGP4.peer")
	p1BgpPeer.SetPeerAddress(dutLoopback.IPv4).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)

	// configure trunks (LAGs) 2 & 3 - dst
	trunks := []struct {
		lagID   uint32
		lagName string
		port    gosnappi.Port
		rxMAC   string
		ateAttr attrs.Attributes
		dutAttr attrs.Attributes
		sysID   string
	}{
		{2, ateLag2, p2, lag2MAC, atePort2, dutPort2, otgLAG2sysID},
		{3, ateLag3, p3, lag3MAC, atePort3, dutPort3, otgLAG3sysID},
	}
	for _, tr := range trunks {
		lag := cfg.Lags().Add().SetName(tr.lagName)
		lag.Protocol().Static().SetLagId(tr.lagID)
		lag.Ports().Add().SetPortName(tr.port.Name()).Ethernet().SetMac(tr.rxMAC).SetName(fmt.Sprintf("LAGRx-%d", tr.lagID))

		lagDev := cfg.Devices().Add().SetName(lag.Name() + ".Dev")
		lagEth := lagDev.Ethernets().Add().SetName(tr.ateAttr.Name + ".Eth").SetMac(tr.ateAttr.MAC)
		lagEth.Connection().SetLagName(lag.Name())
		lagIPv4 := lagEth.Ipv4Addresses().Add().SetName(tr.ateAttr.Name + cfgplugins.IPv4)
		lagIPv4.SetAddress(tr.ateAttr.IPv4).SetGateway(tr.dutAttr.IPv4).SetPrefix(plen4)
		lagIPv6 := lagEth.Ipv6Addresses().Add().SetName(tr.ateAttr.Name + cfgplugins.IPv6)
		lagIPv6.SetAddress(tr.ateAttr.IPv6).SetGateway(tr.dutAttr.IPv6).SetPrefix(plen6)

		// configure ISIS on the trunk
		lagIsis := lagDev.Isis().SetSystemId(tr.sysID).SetName(tr.lagName + "-isis")
		lagIsis.Basic().SetHostname(lagIsis.Name())
		lagIsis.Advanced().SetAreaAddresses([]string{strings.Replace(areaAddress, ".", "", -1)})

		lagIsisInt := lagIsis.Interfaces().Add().
			SetEthName(lagDev.Ethernets().Items()[0].Name()).SetName(tr.lagName + "IsisInt").
			SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
			SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
			SetMetric(baseMetric)
		lagIsisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

		// configure emulated network params
		netV4 := lagDev.Isis().V4Routes().Add().SetName("v4-isisNet-" + tr.lagName).SetLinkMetric(baseMetric)
		netV4.Addresses().Add().SetAddress(v4Route).SetPrefix(v4RoutePlen).SetCount(routeCount)
		netV6 := lagDev.Isis().V6Routes().Add().SetName("v6-isisNet-" + tr.lagName).SetLinkMetric(baseMetric)
		netV6.Addresses().Add().SetAddress(v6Route).SetPrefix(v6RoutePlen).SetCount(routeCount)
	}

	return cfg
}

func changeISISMetric(t *testing.T, dut *ondatra.DUTDevice, intf string, metric uint32) {
	t.Helper()
	b := &gnmi.SetBatch{}
	cfgplugins.ChangeISISMetric(t, dut, b, cfgplugins.ISISInterfaceMetricParams{
		InstanceName: isisInstance,
		Interface:    intf,
		Metric:       metric,
	})
	b.Set(t, dut)
}

// flowParams configures an ATE traffic flow created by createFlow.
type flowParams struct {
	name     string
	isV6     bool
	isLocal  bool
	dstMAC   string
	dstPorts []string
}

func createFlow(t *testing.T, ateTopo gosnappi.Config, p flowParams) gosnappi.Flow {
	t.Helper()

	flow := ateTopo.Flows().Add()
	flow.SetName(p.name)
	flow.Size().SetFixed(flowSizeBytes)
	flow.Duration().Continuous()
	flow.Metrics().SetEnable(true)

	if p.isLocal {
		flow.Rate().SetPps(localTrafficPPS)
		flow.TxRx().Port().SetTxName(port1ID).SetRxNames([]string{port1ID})
	} else {
		flow.Rate().SetPps(trafficPPS)
		srcName := atePort1.Name + cfgplugins.IPv4
		if p.isV6 {
			srcName = atePort1.Name + cfgplugins.IPv6
		}
		flow.TxRx().Device().SetTxNames([]string{srcName}).SetRxNames(p.dstPorts)
	}

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(atePort1.MAC)
	if p.isLocal {
		eth.Dst().SetValue(p.dstMAC)
	}

	if p.isV6 {
		ip := flow.Packet().Add().Ipv6()
		ip.Src().SetValue(atePort1.IPv6)
		if p.isLocal {
			ip.Dst().SetValue(dutLoopback.IPv6)
			flow.Packet().Add().Icmpv6().SetEcho(gosnappi.NewFlowIcmpv6Echo())
		} else {
			ip.Dst().Increment().SetStart(v6IP).SetStep(v6PrefixStep).SetCount(routeCount)
		}
	} else {
		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue(atePort1.IPv4)
		if p.isLocal {
			ip.Dst().SetValue(dutLoopback.IPv4)
			flow.Packet().Add().Icmp().SetEcho(gosnappi.NewFlowIcmpEcho())
		} else {
			ip.Dst().Increment().SetStart(v4IP).SetStep(v4PrefixStep).SetCount(routeCount)
		}
	}

	if !p.isLocal {
		tcp := flow.Packet().Add().Tcp()
		tcp.DstPort().Increment().SetStart(tcpDstPortStart).SetCount(tcpDstPortCount)
	}
	return flow
}

func startProtocolsAndAwait(t *testing.T, dut *ondatra.DUTDevice, otg *otg.OTG, top gosnappi.Config) {
	t.Helper()
	t.Log("Pushing OTG configuration")
	otg.PushConfig(t, top)
	t.Logf("Starting protocols and awaiting for ARP & IS-IS adjacencies")
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, top, "IPv4")
	otgutils.WaitForARP(t, otg, top, "IPv6")
	awaitAdjacency(t, dut, dutPort1Name)
	awaitAdjacency(t, dut, agg2ID)
	awaitAdjacency(t, dut, agg3ID)
}

func lagInFrames(t *testing.T, otg *otg.OTG, lagID string) uint64 {
	t.Helper()
	return gnmi.Get(t, otg, gnmi.OTG().Lag(lagID).State()).GetCounters().GetInFrames()
}

func portOutFrames(t *testing.T, otg *otg.OTG, portID string) uint64 {
	t.Helper()
	return gnmi.Get(t, otg, gnmi.OTG().Port(portID).State()).GetCounters().GetOutFrames()
}

// waitForPortFrames blocks until portID has transmitted at least wantFrames beyond before, or
// timeout elapses, giving a deterministic end to the traffic window instead of a fixed sleep.
func waitForPortFrames(t *testing.T, otg *otg.OTG, portID string, before, wantFrames uint64, timeout time.Duration) error {
	t.Helper()
	want := before + wantFrames
	var got uint64
	if _, ok := gnmi.Watch(t, otg, gnmi.OTG().Port(portID).Counters().OutFrames().State(), timeout, func(val *ygnmi.Value[uint64]) bool {
		v, present := val.Val()
		if !present {
			return false
		}
		got = v
		return got >= want
	}).Await(t); !ok {
		return fmt.Errorf("waitForPortFrames: port %s transmitted %d frames within %v, want >=%d (before=%d, frames=%d)", portID, got, timeout, want, before, wantFrames)
	}
	t.Logf("port %s transmitted %d frames", portID, got-before)
	return nil
}

// validateTrunkTraffic checks that received trunk traffic matches the expected next-hop count:
// nhCount==2 requires traffic on both trunk-2 and trunk-3, load-balanced within
// loadBalanceTolerancePct of an even split; nhCount==1 requires all traffic on trunk-3 and none on trunk-2
func validateTrunkTraffic(t *testing.T, otg *otg.OTG, before map[string]uint64, nhCount int) error {
	t.Helper()
	if nhCount == 0 {
		return nil
	}
	var errs []error
	delta := map[string]uint64{}
	for _, lagID := range []string{ateLag2, ateLag3} {
		got := lagInFrames(t, otg, lagID)
		if got < before[lagID] {
			delta[lagID] = got
			continue
		}
		delta[lagID] = got - before[lagID]
	}

	switch nhCount {
	case 2:
		for _, lagID := range []string{ateLag2, ateLag3} {
			if delta[lagID] == 0 {
				errs = append(errs, fmt.Errorf("validateTrunkTraffic: lag %s received no traffic, before=%d delta=%d", lagID, before[lagID], delta[lagID]))
			}
		}
		if err := validateLoadBalance(delta[ateLag2], delta[ateLag3], loadBalanceTolerancePct); err != nil {
			errs = append(errs, err)
		}
	case 1:
		if delta[ateLag3] == 0 {
			errs = append(errs, fmt.Errorf("validateTrunkTraffic: lag %s received no traffic, before=%d delta=%d", ateLag3, before[ateLag3], delta[ateLag3]))
		}
		total := delta[ateLag2] + delta[ateLag3]
		if total > 0 {
			if drainedPct := float64(delta[ateLag2]) / float64(total) * 100; drainedPct > trafficTolerance {
				errs = append(errs, fmt.Errorf("validateTrunkTraffic: lag %s (drained trunk) received %.2f%% of trunk traffic (before=%d delta=%d), want <=%.2f%% (residual traffic only)", ateLag2, drainedPct, before[ateLag2], delta[ateLag2], trafficTolerance))
			}
		}
	}
	return errors.Join(errs...)
}

// validateLoadBalance checks that two trunk traffic counts are each within tolerancePct of an
// even (50/50) split of their combined total.
func validateLoadBalance(count2, count3 uint64, tolerancePct float64) error {
	total := count2 + count3
	if total == 0 {
		return fmt.Errorf("validateLoadBalance: no traffic received on trunk-2 or trunk-3")
	}
	got2Pct := float64(count2) / float64(total) * 100
	got3Pct := float64(count3) / float64(total) * 100
	if got2Pct < 50-tolerancePct || got2Pct > 50+tolerancePct {
		return fmt.Errorf("validateLoadBalance: trunk-2 received %.2f%% of traffic (count=%d), trunk-3 received %.2f%% (count=%d), want each within %.0f%% of an even split", got2Pct, count2, got3Pct, count3, tolerancePct)
	}
	return nil
}

func validateTrafficLoss(t *testing.T, otg *otg.OTG, flowName string, minLossPct, maxLossPct float64) error {
	t.Helper()
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		otgutils.ExpectedTrafficLoss(t, otg, flowName, minLossPct, maxLossPct)
	}); errMsg != nil {
		t.Logf("validateTrafficLoss: captured error message: %s", *errMsg)
		return fmt.Errorf("validateTrafficLoss: unexpected traffic loss on flow %s: %s", flowName, *errMsg)
	}
	return nil
}

// runTrafficFlows validates one traffic window over the already-configured flows, without an ATE config push.
func runTrafficFlows(t *testing.T, dut *ondatra.DUTDevice, otg *otg.OTG, flows []gosnappi.Flow, nhCount int) error {
	t.Helper()
	var errs []error
	if nhCount > 0 {
		if err := aftCheck(t, dut, nhCount); err != nil {
			errs = append(errs, err)
		}
	}

	top := otg.GetConfig(t)

	packetsBefore := map[string]uint64{
		ateLag2: lagInFrames(t, otg, ateLag2),
		ateLag3: lagInFrames(t, otg, ateLag3),
	}
	port1FramesBefore := portOutFrames(t, otg, port1ID)
	var wantPort1Frames uint64
	for _, flow := range flows {
		wantPort1Frames += flow.Rate().Pps() * uint64(runTrafficTime.Seconds())
	}

	otg.StartTraffic(t)
	if err := waitForPortFrames(t, otg, port1ID, port1FramesBefore, wantPort1Frames, trafficWaitTimeout); err != nil {
		errs = append(errs, err)
	}
	otg.StopTraffic(t)
	otgutils.LogFlowMetrics(t, otg, top)
	otgutils.LogPortMetrics(t, otg, top)
	if err := validateTrunkTraffic(t, otg, packetsBefore, nhCount); err != nil {
		errs = append(errs, err)
	}

	for _, flow := range flows {
		if err := validateTrafficLoss(t, otg, flow.Name(), 0, trafficTolerance); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func awaitAdjacency(t *testing.T, dut *ondatra.DUTDevice, intfName string) {
	t.Helper()
	path := isisPath(dut)
	intfName = cfgplugins.InterfaceRefID(dut, intfName)
	intf := path.Interface(intfName)

	query := intf.LevelAny().AdjacencyAny().AdjacencyState().State()
	_, ok := gnmi.WatchAll(t, dut, query, stateWatchTimeout, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		v, ok := val.Val()
		return v == oc.Isis_IsisInterfaceAdjState_UP && ok
	}).Await(t)

	if !ok {
		t.Fatalf("IS-IS adjacency was not formed on interface %v", intfName)
	}
}

func aftCheck(t *testing.T, dut *ondatra.DUTDevice, nhCount int) error {
	aftsPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts()

	_, ok := gnmi.Watch(t, dut, aftsPath.Ipv4Entry(v4RouteCIDR).State(), stateWatchTimeout, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		ipv4Entry, present := val.Val()
		if !present {
			return false
		}
		hopGroup, present := gnmi.Lookup(t, dut, aftsPath.NextHopGroup(ipv4Entry.GetNextHopGroup()).State()).Val()
		if !present {
			return false
		}
		got := len(hopGroup.NextHop)
		want := nhCount
		t.Logf("Aft check for %s: Got %d nexthop,want %d", ipv4Entry.GetPrefix(), got, want)
		return got == want

	}).Await(t)

	if !ok {
		return fmt.Errorf("aftCheck: AFT check failed for %s", v4RouteCIDR)
	}

	_, ok = gnmi.Watch(t, dut, aftsPath.Ipv6Entry(v6RouteCIDR).State(), stateWatchTimeout, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
		ipv6Entry, present := val.Val()
		if !present {
			return false
		}
		hopGroup, present := gnmi.Lookup(t, dut, aftsPath.NextHopGroup(ipv6Entry.GetNextHopGroup()).State()).Val()
		if !present {
			return false
		}
		got := len(hopGroup.NextHop)
		want := nhCount
		t.Logf("Aft check for %s: Got %d nexthop,want %d", ipv6Entry.GetPrefix(), got, want)
		return got == want

	}).Await(t)

	if !ok {
		return fmt.Errorf("aftCheck: AFT check failed for %s", v6RouteCIDR)
	}
	return nil
}

func isisPath(dut *ondatra.DUTDevice) *netinstisis.NetworkInstance_Protocol_IsisPath {
	return gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()
}

func setOverloadBit(t *testing.T, dut *ondatra.DUTDevice, set bool) {
	t.Helper()
	t.Logf("Setting the IS-IS overload bit to %v", set)
	gnmi.Update(t, dut, isisPath(dut).Global().LspBit().OverloadBit().SetBit().Config(), set)
}

// verifyOverloadBit confirms the overload bit telemetry reflects the configured value on DUT.
func verifyOverloadBit(t *testing.T, dut *ondatra.DUTDevice, want bool) error {
	t.Helper()
	setBit := isisPath(dut).Global().LspBit().OverloadBit().SetBit().State()
	_, ok := gnmi.Watch(t, dut, setBit, watchTimeout, func(val *ygnmi.Value[bool]) bool {
		got, present := val.Val()
		if !present {
			return !want && deviations.MissingValueForDefaults(dut)
		}
		return got == want
	}).Await(t)
	if !ok {
		return fmt.Errorf("verifyOverloadBit: IS-IS overload-bit set-bit state on %v: want %v", dut.Name(), want)
	}
	return nil
}

// verifyOverloadBitAdvertised confirms the ATE learns the overload flag in a DUT LSP.
func verifyOverloadBitAdvertised(t *testing.T, otg *otg.OTG) error {
	t.Helper()
	query := gnmi.OTG().IsisRouter("port1-isis").LinkStateDatabase().LspsAny().Flags().State()
	_, ok := gnmi.WatchAll(t, otg, query, stateWatchTimeout, func(val *ygnmi.Value[[]otgtelemetry.E_Lsps_Flags]) bool {
		flags, present := val.Val()
		if !present {
			return false
		}
		for _, flag := range flags {
			if flag == otgtelemetry.Lsps_Flags_OVERLOAD {
				return true
			}
		}
		return false
	}).Await(t)
	if !ok {
		return fmt.Errorf("verifyOverloadBitAdvertised: ATE did not learn an IS-IS LSP with the overload flag")
	}
	return nil
}

// verifyAdjacencyStability confirms no IS-IS adjacency leaves the UP state during the window.
func verifyAdjacencyStability(t *testing.T, dut *ondatra.DUTDevice, window time.Duration) error {
	t.Helper()
	t.Logf("Verifying IS-IS adjacencies remain UP over %v", window)
	query := isisPath(dut).InterfaceAny().LevelAny().AdjacencyAny().AdjacencyState().State()
	samples := gnmi.CollectAll(t, dut, query, window).Await(t)
	if len(samples) == 0 {
		return fmt.Errorf("verifyAdjacencyStability: got no IS-IS adjacency state samples, want adjacencies reporting UP")
	}
	var errs []error
	for _, sample := range samples {
		state, present := sample.Val()
		if !present || state != oc.Isis_IsisInterfaceAdjState_UP {
			errs = append(errs, fmt.Errorf("verifyAdjacencyStability: IS-IS adjacency %v: got %v, want UP", sample.Path, state))
		}
	}
	return errors.Join(errs...)
}

// verifyBGPEstablished confirms the iBGP session to the DUT loopback is up.
func verifyBGPEstablished(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	nbrPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv4)
	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), stateWatchTimeout, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := val.Val()
		return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		return fmt.Errorf("verifyBGPEstablished: iBGP session to %v: got not ESTABLISHED, want ESTABLISHED", atePort1.IPv4)
	}
	return nil
}

func runMetricDrain(t *testing.T, dut *ondatra.DUTDevice, otg *otg.OTG, ateCfg gosnappi.Config) error {
	t.Helper()
	t.Log("Description: IS-IS Metric Drain (Trunk Interfaces)")
	var errs []error

	ecmpFlows := []gosnappi.Flow{
		createFlow(t, ateCfg, flowParams{name: "ecmp-flow-v4", dstPorts: []string{atePort2.Name + cfgplugins.IPv4, atePort3.Name + cfgplugins.IPv4}}),
		createFlow(t, ateCfg, flowParams{name: "ecmp-flow-v6", isV6: true, dstPorts: []string{atePort2.Name + cfgplugins.IPv6, atePort3.Name + cfgplugins.IPv6}}),
	}
	startProtocolsAndAwait(t, dut, otg, ateCfg)

	// Step 1: Advertise 1,000 IPv4 and 1,000 IPv6 prefixes from ATE connected to trunk-2 and trunk-3 (configured in configureATE).
	// Step 2: Send continuous IPv4 and IPv6 traffic flows from ATE Port-1 to the 2,000 advertised prefixes (ecmpFlows).
	// Step 3: Wait for IS-IS convergence (up to 30s). Validate that steady-state traffic loss is 0% and traffic transits via trunk-2 and trunk-3.
	t.Log("Step 1-3: Validating steady-state traffic is load-balanced over trunk-2 and trunk-3 with 0% loss")
	if err := runTrafficFlows(t, dut, otg, ecmpFlows, 2); err != nil {
		errs = append(errs, err)
	}

	// Step 4: Change the ISIS metric of trunk-2 to 1000 via gNMI Set.
	t.Logf("Step 4: Changing ISIS metric on trunk-2 (%s) to %d", agg2ID, drainMetric)
	changeISISMetric(t, dut, agg2ID, drainMetric)

	// Step 5: Validate that 100% of the traffic transits via trunk-3 only. Verify steady-state traffic loss is 0%.
	t.Log("Step 5: Validating 100% of traffic transits trunk-3 only with 0% steady-state loss")
	if err := runTrafficFlows(t, dut, otg, ecmpFlows, 1); err != nil {
		errs = append(errs, err)
	}

	// Step 6: Revert the ISIS metric on trunk-2 back to original value. Validate traffic recovers and is load-balanced over trunk-2 and trunk-3.
	t.Logf("Step 6: Reverting ISIS metric on trunk-2 (%s) back to %d and verifying traffic recovery", agg2ID, baseMetric)
	changeISISMetric(t, dut, agg2ID, baseMetric)
	if err := runTrafficFlows(t, dut, otg, ecmpFlows, 2); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func runOverloadBitDrain(t *testing.T, dut *ondatra.DUTDevice, otg *otg.OTG, ateCfg gosnappi.Config) error {
	t.Helper()
	t.Log("Description: IS-IS Overload Bit Drain")
	var errs []error

	ateCfg.Flows().Clear()
	p1 := dut.Port(t, port1ID)
	dutPort1MAC := gnmi.Get(t, dut, gnmi.OC().Interface(p1.Name()).Ethernet().MacAddress().State())
	ecmpFlows := []gosnappi.Flow{
		createFlow(t, ateCfg, flowParams{name: "ecmp-flow-v4", dstPorts: []string{atePort2.Name + cfgplugins.IPv4, atePort3.Name + cfgplugins.IPv4}}),
		createFlow(t, ateCfg, flowParams{name: "ecmp-flow-v6", isV6: true, dstPorts: []string{atePort2.Name + cfgplugins.IPv6, atePort3.Name + cfgplugins.IPv6}}),
	}
	localFlows := []gosnappi.Flow{
		createFlow(t, ateCfg, flowParams{name: "loopback-flow-v4", isLocal: true, dstMAC: dutPort1MAC}),
		createFlow(t, ateCfg, flowParams{name: "loopback-flow-v6", isV6: true, isLocal: true, dstMAC: dutPort1MAC}),
	}
	startProtocolsAndAwait(t, dut, otg, ateCfg)

	allFlows := make([]gosnappi.Flow, 0, len(ecmpFlows)+len(localFlows))
	allFlows = append(allFlows, ecmpFlows...)
	allFlows = append(allFlows, localFlows...)

	// Step 1: Ensure ATE Port-2 advertises transit networks (configured in configureATE).
	// Step 2: Establish an iBGP session from ATE Port-1 to DUT Loopback interface to simulate control plane traffic.
	// Step 3: Send transit traffic (ATE Port-1 -> DUT -> ATE Port-2) and local traffic (ATE Port-1 -> DUT Loopback).
	t.Log("Step 1-3: Validating steady-state transit traffic and iBGP session before draining. Validating local traffic to the DUT loopback before draining")
	if err := runTrafficFlows(t, dut, otg, allFlows, 2); err != nil {
		errs = append(errs, err)
	}
	if err := verifyBGPEstablished(t, dut); err != nil {
		errs = append(errs, err)
	}

	// Step 4: Set the ISIS Overload bit to true via gNMI Set.
	t.Log("Step 4: Setting the IS-IS Overload bit to true")
	setOverloadBit(t, dut, true)

	// Step 5: Use gNMI Get/Watch to verify telemetry state reflects true.
	t.Log("Step 5: Verifying telemetry state set-bit is true")
	if err := verifyOverloadBit(t, dut, true); err != nil {
		errs = append(errs, err)
	}

	// Step 6: Wait for convergence. Verify that ate learns the IS-IS overload flag from the DUT LSP.
	t.Log("Step 6: Verifying the ATE learns the IS-IS overload flag in the DUT LSP")
	if err := verifyOverloadBitAdvertised(t, otg); err != nil {
		errs = append(errs, err)
	}

	// Step 7: Local loopback traffic (and iBGP session) experiences 0% loss.
	// Step 8: Verify that toggling overload-bit config does not flap or reset existing IS-IS adjacencies (Adjacency Stability).
	t.Log("Step 7-8: Verifying IS-IS adjacencies remain UP without flapping")
	if err := verifyAdjacencyStability(t, dut, stabilityWindow); err != nil {
		errs = append(errs, err)
	}
	t.Log("Verifying local traffic to the DUT loopback experiences 0% loss")
	if err := runTrafficFlows(t, dut, otg, localFlows, 0); err != nil {
		errs = append(errs, err)
	}
	if err := verifyBGPEstablished(t, dut); err != nil {
		errs = append(errs, err)
	}

	// Step 9: Clear the ISIS Overload bit by setting it to false.
	t.Log("Step 9: Clearing the IS-IS Overload bit to false")
	setOverloadBit(t, dut, false)

	// Step 10: Verify the telemetry state reflects the change to false.
	t.Log("Step 10: Verifying telemetry state set-bit is false")
	if err := verifyOverloadBit(t, dut, false); err != nil {
		errs = append(errs, err)
	}

	// Step 11: Verify that transit IPv4 and IPv6 traffic recovers and is forwarded through DUT with 0% steady-state loss.
	t.Log("Step 11: Verifying transit traffic recovers after clearing the overload bit")
	if err := runTrafficFlows(t, dut, otg, allFlows, 2); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func TestDrain(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	t.Cleanup(func() {
		t.Log("Test cleanup")
		p1 := dut.Port(t, port1ID)
		defaultNI := deviations.DefaultNetworkInstance(dut)
		if agg2ID != "" {
			cfgplugins.DeleteAggregate(t, dut, agg2ID, []*ondatra.Port{dut.Port(t, port2ID)})
		}
		if agg3ID != "" {
			cfgplugins.DeleteAggregate(t, dut, agg3ID, []*ondatra.Port{dut.Port(t, port3ID)})
		}
		cleanupBatch := &gnmi.SetBatch{}
		gnmi.BatchDelete(cleanupBatch, gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Config())
		gnmi.BatchDelete(cleanupBatch, gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Config())
		for _, intfName := range []string{p1.Name(), agg2ID, agg3ID, loopbackIntfName} {
			if intfName == "" {
				continue
			}
			gnmi.BatchDelete(cleanupBatch, gnmi.OC().Interface(intfName).Config())
			gnmi.BatchDelete(cleanupBatch, gnmi.OC().NetworkInstance(defaultNI).Interface(intfName).Config())
		}
		cleanupBatch.Set(t, dut)
		t.Log("Test cleanup completed")
	})

	configureDUT(t, dut)
	ateCfg := configureATE()

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "RT-2.14.1",
			run: func(t *testing.T) error {
				return runMetricDrain(t, dut, otg, ateCfg)
			},
		},
		{
			name: "RT-2.14.2",
			run: func(t *testing.T) error {
				return runOverloadBitDrain(t, dut, otg, ateCfg)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() {
				t.Log("Subtest cleanup")
				changeISISMetric(t, dut, agg2ID, baseMetric)
				setOverloadBit(t, dut, false)
			})
			if err := tc.run(t); err != nil {
				t.Error(err)
			}
		})
	}
}
