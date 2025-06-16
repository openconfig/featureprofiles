// Copyright 2022 Google LLC
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

// Package match_dscp_indirect_next_hop_test implements PF-1.1: IPv4/IPv6 policy-forwarding to indirect NH matching DSCP/TC.
package match_dscp_indirect_next_hop_test

import (
	"context"
	"fmt"
	"math"
	"net"
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
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	advertisedRoutesv4Prefix = 32
	advertisedRoutesv6Prefix = 128
	dutAS                    = 65500
	plenIPv4                 = 30
	plenIPv6                 = 126
	rplType                  = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	rplName                  = "ALLOW"
	peerGrpNamev4            = "BGP-PEER-GROUP-V4"
	peerGrpNamev6            = "BGP-PEER-GROUP-V6"
	RouteCount               = uint32(1)
	ateAS                    = 65501
	ipvNHv4                  = "202.0.113.1"
	ipvNHv6                  = "2001:FFFF::1234:5678"
	ipv4Dst                  = "100.100.100.0"
	ipv6Dst                  = "2001:2:1::0"
	lossTolerance            = 1
	bgpV4RouteName           = "BGP4-PEER-v4ROUTE"
	bgpV6RouteName           = "BGP6-PEER-v6ROUTE"
	policyMatchFlowV4        = "PF-MATCH-FLOW-V4"
	policyMatchFlowV6        = "PF-MATCH-FLOW-V6"
	policyNoMatchFlowV4      = "PF-NO-MATCH-FLOW-V4"
	policyNoMatchFlowV6      = "PF-NO-MATCH-FLOW-V6"
	defaultFlow              = "DEFAULT-FLOW"
	defaultFlowV6            = "DEFAULT-FLOW-V6"
	trafficPolicyName        = "BG_PBR_TRAFFIC_POLICY"
	trafficDuration          = 30 * time.Second
	timeout                  = 1 * time.Minute
	interval                 = 20 * time.Second
	iterationCount           = 2
)

var (
	dutP1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateP1 = attrs.Attributes{
		Name:    "ateP1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	dutP2 = attrs.Attributes{
		Name:    "port2",
		Desc:    "DUT to ATE destination-2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	ateP2 = attrs.Attributes{
		Name:    "ateP2",
		MAC:     "02:00:02:01:01:02",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	dutP3 = attrs.Attributes{
		Name:    "port3",
		Desc:    "DUT to ATE destination-3",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::9",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	ateP3 = &attrs.Attributes{
		Name:    "ateP3",
		MAC:     "02:00:02:01:01:03",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::a",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	pfMatchingDscpValues = []uint32{3, 11, 19, 27, 35, 43, 51, 59}
	otherDscpValues      = []uint32{0, 8, 16, 24, 32, 40, 48, 56}
)

type ipAddr struct {
	address string
	prefix  uint32
}

// configureDUT configures all the interfaces and BGP on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()

	for _, portAttr := range []attrs.Attributes{dutP1, dutP2, dutP3} {
		p := dut.Port(t, portAttr.Name).Name()
		i := portAttr.NewOCInterface(p, dut)
		gnmi.Replace(t, dut, dc.Interface(p).Config(), i)
	}

	t.Log("Configure Default Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	configureRoutePolicy(t, dut, rplName, rplType)

	dutConfPath := dc.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	dutConf := createBGPNeighbor(dutAS, ateAS, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)

	t.Log("Configure Static Routes IPV4-DST1/IPV6-DST1 towards ATE port 3")
	configureDUTStaticRoutes(t, dut)

	t.Log("PF action is to redirect to BGP-announced next-hops (IPV-NH-V4/IPV-NH-V6)")
	configTrafficPolicy(t, dut, trafficPolicyName)
}

func configTrafficPolicy(t *testing.T, dut *ondatra.DUTDevice, name string) {

	interfaceName := dut.Port(t, "port1").Name()

	if deviations.PolicyForwardingToNextHopOcUnsupported(dut) {
		gnmiClient := dut.RawAPIs().GNMI(t)
		config := trafficPolicyConf(dut, interfaceName)
		if config == "" {
			t.Fatalf("Unsupported vendor %s for deviation 'PolicyForwardingToNextHopOcUnsupported'", dut.Vendor())
		}
		t.Logf("Push the CLI config:%s", dut.Vendor())
		gpbSetRequest, err := helpers.BuildCliConfigRequest(config)
		if err != nil {
			t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
		}
		if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	} else {
		d := &oc.Root{}
		ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
		npf := ni.GetOrCreatePolicyForwarding()
		np := npf.GetOrCreatePolicy(name)
		np.SetPolicyId(name)
		np.Type = oc.Policy_Type_PBR_POLICY

		for i, dscp := range pfMatchingDscpValues {
			npRule := np.GetOrCreateRule(uint32(i + 1))
			ip := npRule.GetOrCreateIpv4()
			ip.SetSourceAddress(fmt.Sprintf("%s/32", ateP1.IPv4))
			ip.SetDestinationAddress(fmt.Sprintf("%s/32", ipv4Dst))
			ip.SetDscp(uint8(dscp))
			npRuleAction := npRule.GetOrCreateAction()
			npRuleAction.SetNextHop(ipvNHv4)
		}

		for i, dscp := range pfMatchingDscpValues {
			npRule := np.GetOrCreateRule(uint32(i+1) + uint32(len(pfMatchingDscpValues)))
			ip := npRule.GetOrCreateIpv6()
			ip.SetSourceAddress(fmt.Sprintf("%s/128", ateP1.IPv6))
			ip.SetDestinationAddress(fmt.Sprintf("%s/128", ipv6Dst))
			ip.SetDscp(uint8(dscp))
			npRuleAction := npRule.GetOrCreateAction()
			npRuleAction.SetNextHop(ipvNHv6)
		}

		interfaceName := dut.Port(t, "port1").Name()
		npi := npf.GetOrCreateInterface(interfaceName)
		npi.ApplyForwardingPolicy = np.PolicyId
		gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Config(), ni)

	}
}

func (ip *ipAddr) cidr(t *testing.T) string {
	_, net, err := net.ParseCIDR(fmt.Sprintf("%s/%d", ip.address, ip.prefix))
	if err != nil {
		t.Fatal(err)
	}
	return net.String()
}

func configureDUTStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	prefix := ipAddr{address: ipv4Dst, prefix: 24}
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          prefix.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(ateP3.IPv4),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}

	prefixV6 := ipAddr{address: ipv6Dst, prefix: 48}
	sV6 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          prefixV6.cidr(t),
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(ateP3.IPv6),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV6, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, dut)
}

func trafficPolicyConf(dut *ondatra.DUTDevice, interfaceName string) string {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		var v4MatchRules, v6MatchRules string
		// Create MatchingRules And Action
		for _, dscp := range pfMatchingDscpValues {
			v4MatchRules += fmt.Sprintf(`
			match v4-dscp%d ipv4
			dscp %d
			!
			actions
			count
			redirect next-hop group NH_V4
			!
			`, dscp, dscp)

			v6MatchRules += fmt.Sprintf(`
			match v6-dscp%d ipv6
			dscp %d
			!
			actions
			count
			redirect next-hop group NH_V6
			!
			`, dscp, dscp)
		}

		// Apply Policy on the interface
		trafficPolicyConfig := fmt.Sprintf(`
			traffic-policies
			traffic-policy %s
			%s
			%s
			match ipv4-all-default ipv4
			actions
			count
			!
			match ipv6-all-default ipv6
			actions
			count
			!
			nexthop-group NH_V4 type ip
			fec hierarchical
			entry 0 nexthop %s
			!
			nexthop-group NH_V6 type ip
			fec hierarchical
			entry 0 nexthop %s
			!
			interface %s
			
			traffic-policy input BG_PBR_TRAFFIC_POLICY
			`, trafficPolicyName, v4MatchRules, v6MatchRules, ipvNHv4, ipvNHv6, interfaceName)
		return trafficPolicyConfig
	default:
		return ""
	}
}

type BGPNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
}

func createBGPNeighbor(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nbrs := []*BGPNeighbor{
		{as: peerAs, neighborip: ateP2.IPv4, isV4: true},
		{as: peerAs, neighborip: ateP2.IPv6, isV4: false},
	}

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.SetAs(localAs)
	global.SetRouterId(dutP2.IPv4)

	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pgv4 := bgp.GetOrCreatePeerGroup(peerGrpNamev4)
	pgv4.SetPeerAs(peerAs)
	pgv4.SetPeerGroupName(peerGrpNamev4)
	pgv6 := bgp.GetOrCreatePeerGroup(peerGrpNamev6)
	pgv6.SetPeerAs(peerAs)
	pgv6.SetPeerGroupName(peerGrpNamev6)

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.SetPeerAs(nbr.as)
			nv4.SetEnabled(true)
			nv4.SetPeerGroup(peerGrpNamev4)
			afisafi := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			afisafi.SetEnabled(true)
			nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(false)
			pgafv4 := pgv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			pgafv4.SetEnabled(true)
			rpl := pgafv4.GetOrCreateApplyPolicy()
			rpl.ImportPolicy = []string{rplName}
			rpl.ExportPolicy = []string{rplName}
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.SetPeerAs(nbr.as)
			nv6.SetEnabled(true)
			nv6.SetPeerGroup(peerGrpNamev6)
			afisafi6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			afisafi6.SetEnabled(true)
			nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(false)
			pgafv6 := pgv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			pgafv6.SetEnabled(true)
			rpl := pgafv6.GetOrCreateApplyPolicy()
			rpl.ImportPolicy = []string{rplName}
			rpl.ExportPolicy = []string{rplName}
		}
	}
	return niProto
}

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func waitForBGPSession(t *testing.T, dut *ondatra.DUTDevice, wantEstablished bool) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateP2.IPv4)
	nbrPathv6 := statePath.Neighbor(ateP2.IPv6)
	compare := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if ok {
			if wantEstablished {
				return state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			}
			return state == oc.Bgp_Neighbor_SessionState_IDLE
		}
		return false
	}

	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, compare).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		if wantEstablished {
			t.Fatal("No BGP neighbor formed...")
		} else {
			t.Fatal("BGPv4 session didn't teardown.")
		}
	}
	_, ok = gnmi.Watch(t, dut, nbrPathv6.SessionState().State(), 2*time.Minute, compare).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, dut, nbrPathv6.State()))
		if wantEstablished {
			t.Fatal("No BGPv6 neighbor formed...")
		} else {
			t.Fatal("BGPv6 session didn't teardown.")
		}
	}
}

func verifyPrefixesTelemetryV4(t *testing.T, dut *ondatra.DUTDevice, wantInstalled uint32) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv4 := statePath.Neighbor(ateP2.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()

	if gotInstalled, ok := gnmi.Watch(t, dut, prefixesv4.Installed().State(), time.Minute, func(v *ygnmi.Value[uint32]) bool {
		got, ok := v.Val()
		return ok && got == wantInstalled
	}).Await(t); !ok {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}
}

func configureATE(t *testing.T) gosnappi.Config {
	dut := ondatra.DUT(t, "dut")

	config := gosnappi.NewConfig()
	ate := ondatra.ATE(t, "ate")

	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	ap3 := ate.Port(t, "port3")

	ateP1.AddToOTG(config, ap1, &dutP1)

	d2 := ateP2.AddToOTG(config, ap2, &dutP2)

	d2ipv41 := d2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	d2ipv61 := d2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	configureBGPDev(d2, d2ipv41, ateAS, ipvNHv4)
	configureBGPV6Dev(d2, d2ipv61, ateAS, ipvNHv6)

	ateP3.AddToOTG(config, ap3, &dutP3)

	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())

	configureFlow(config, policyMatchFlowV4, ap1.ID(),
		[]string{ap2.ID()},
		macAddress,
		ateP1.IPv4,
		ipv4Dst, "ipv4", 1, pfMatchingDscpValues)

	configureFlow(config, policyNoMatchFlowV4, ap1.ID(),
		[]string{ap3.ID()},
		macAddress,
		ateP1.IPv4,
		ipv4Dst, "ipv4", 1, otherDscpValues)

	configureFlow(config, defaultFlow, ap1.ID(),
		[]string{ap3.ID()},
		macAddress,
		ateP1.IPv4,
		ipv4Dst, "ipv4", 1, append(pfMatchingDscpValues, otherDscpValues...))

	configureFlow(config, policyMatchFlowV6, ap1.ID(),
		[]string{ap2.ID()},
		macAddress,
		ateP1.IPv6,
		ipv6Dst, "ipv6", 1, pfMatchingDscpValues)

	configureFlow(config, policyNoMatchFlowV6, ap1.ID(),
		[]string{ap3.ID()},
		macAddress,
		ateP1.IPv6,
		ipv6Dst, "ipv6", 1, otherDscpValues)

	configureFlow(config, defaultFlowV6, ap1.ID(),
		[]string{ap3.ID()},
		macAddress,
		ateP1.IPv6,
		ipv6Dst, "ipv6", 1, append(pfMatchingDscpValues, otherDscpValues...))

	return config
}

// configureBGPDev configures the BGP on the OTG dev
func configureBGPDev(dev gosnappi.Device, Ipv4 gosnappi.DeviceIpv4, as int, bgpRoute string) {

	Bgp := dev.Bgp().SetRouterId(Ipv4.Address())
	Bgp4Peer := Bgp.Ipv4Interfaces().Add().SetIpv4Name(Ipv4.Name()).Peers().Add().SetName(dev.Name() + ".BGP4.peer")
	Bgp4Peer.SetPeerAddress(dutP2.IPv4).SetAsNumber(uint32(as)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	routes := Bgp4Peer.V4Routes().Add().SetName(bgpV4RouteName)
	routes.Addresses().Add().
		SetAddress(bgpRoute).
		SetPrefix(advertisedRoutesv4Prefix).
		SetCount(RouteCount)

}

func configureBGPV6Dev(dev gosnappi.Device, Ipv6 gosnappi.DeviceIpv6, as int, bgpRoutev6 string) {

	// BGP Router Id is always ipv4
	Bgp := dev.Bgp().SetRouterId(ateP2.IPv4)
	Bgp6Peer := Bgp.Ipv6Interfaces().Add().SetIpv6Name(Ipv6.Name()).Peers().Add().SetName(dev.Name() + ".BGP6.peer")
	Bgp6Peer.SetPeerAddress(dutP2.IPv6).SetAsNumber(uint32(as)).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	routes := Bgp6Peer.V6Routes().Add().SetName(bgpV6RouteName)
	routes.Addresses().Add().
		SetAddress(bgpRoutev6).
		SetPrefix(advertisedRoutesv6Prefix).
		SetCount(RouteCount)

}

func configureFlow(topo gosnappi.Config, name, src string, dst []string, dstMac, srcIp, dstIp, iptype string, routeCount uint32, dscp []uint32) {
	flow := topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().
		SetTxName(src).
		SetRxNames(dst)
	flow.Size().SetFixed(1500)
	flow.Rate().SetPps(100)
	e := flow.Packet().Add().Ethernet()
	e.Dst().SetValue(dstMac)
	if iptype == "ipv4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(srcIp)
		v4.Dst().Increment().SetStart(dstIp).SetCount(routeCount)
		v4.Priority().Dscp().Phb().SetValues(dscp)
	} else {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(srcIp)
		v6.Dst().Increment().SetStart(dstIp).SetCount(routeCount)
		var newValue []uint32
		// IPv6 TC value as per test README is 4*IPv4 DSCP
		// Ex: for DSCP [0, 8, 16, 24, 32, 40, 48, 56] IPv6 flows should use TC 8-bit values [0, 32 , 64( , 96, 128, 160, 192, 224])
		for _, i := range dscp {
			newValue = append(newValue, i<<2)
		}
		v6.TrafficClass().SetValues(newValue)
	}
}

func verifyFlowTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flowName string) bool {
	t.Log("Verify Flow Traffic")
	startTime := time.Now()
	count := 0
	var got float64
	for time.Since(startTime) < timeout {

		otgutils.LogFlowMetrics(t, ate.OTG(), config)
		countersPath := gnmi.OTG().Flow(flowName).Counters()
		framesRx := gnmi.Get(t, ate.OTG(), countersPath.InPkts().State())
		framesTx := gnmi.Get(t, ate.OTG(), countersPath.OutPkts().State())

		if got = (math.Abs(float64(framesTx)-float64(framesRx)) * 100) / float64(framesTx); got <= lossTolerance {
			return true
		} else {
			time.Sleep(interval)
			count += 1
		}
	}

	if count >= iterationCount {
		t.Logf("Packet loss percentage for flow: got %v, want %v", got, lossTolerance)
		return false
	}
	return true
}

func TestPolicyForwardingIndirectNextHop(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	// DUT Configuration
	t.Log("Start DUT interface Config")
	configureDUT(t, dut)

	// ATE Configuration.
	t.Log("Start ATE Config")
	config := configureATE(t)

	otg.PushConfig(t, config)

	otgutils.WaitForARP(t, otg, config, "ipv4")
	otgutils.WaitForARP(t, otg, config, "ipv6")

	otg.StartProtocols(t)

	t.Log("Verify BGP Session formed with Port 2")
	waitForBGPSession(t, dut, true)

	t.Log("Verifying the Next-Hop for PF is advertised by BGP on Port2")
	verifyPrefixesTelemetryV4(t, dut, 1)

	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	otg.StopTraffic(t)

	type flowTest struct {
		tcName string
		ipType string
		flow   string
	}

	tests := []flowTest{
		{
			tcName: "PF next-hop",
			ipType: "IPv4",
			flow:   policyMatchFlowV4,
		},
		{
			tcName: "PF next-hop",
			ipType: "IPv6",
			flow:   policyMatchFlowV6,
		}}
	for _, tc := range tests {
		t.Run("VerifyPFNext_hopAction", func(t *testing.T) {
			if verifyFlowTraffic(t, ate, config, tc.flow) {
				t.Logf("%s action Passed for %s", tc.tcName, tc.ipType)
			} else {
				t.Errorf("%s action Failed for %s", tc.tcName, tc.ipType)
			}
		})
	}

	tests = []flowTest{
		{
			tcName: "PF no-match",
			ipType: "IPv4",
			flow:   policyMatchFlowV4,
		},
		{
			tcName: "PF no-match",
			ipType: "IPv6",
			flow:   policyNoMatchFlowV6,
		}}
	for _, tc := range tests {
		t.Run("VerifyPFNo_matchAction", func(t *testing.T) {
			if verifyFlowTraffic(t, ate, config, tc.flow) {
				t.Logf("%s action Passed for %s", tc.tcName, tc.ipType)
			} else {
				t.Errorf("%s action Failed for %s", tc.tcName, tc.ipType)
			}
		})
	}

	t.Log("PF-1.1.3: Verify PF without NH present Validation in progress")
	t.Log("Withdraw next-hop prefixes from BGP Announcement")
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames([]string{bgpV4RouteName, bgpV6RouteName}).SetState(gosnappi.StateProtocolRouteState.WITHDRAW)
	otg.SetControlState(t, cs)

	verifyPrefixesTelemetryV4(t, dut, 0)

	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	otg.StopTraffic(t)

	tests = []flowTest{
		{
			tcName: "PF without NH",
			ipType: "IPv4",
			flow:   defaultFlow,
		},
		{
			tcName: "PF without NH",
			ipType: "IPv6",
			flow:   defaultFlowV6,
		}}
	for _, tc := range tests {
		t.Run("VerifyPFWithoutNHPresent", func(t *testing.T) {
			if verifyFlowTraffic(t, ate, config, tc.flow) {
				t.Logf("%s action Passed for %s", tc.tcName, tc.ipType)
			} else {
				t.Errorf("%s action Failed for %s", tc.tcName, tc.ipType)
			}
		})
	}

}
