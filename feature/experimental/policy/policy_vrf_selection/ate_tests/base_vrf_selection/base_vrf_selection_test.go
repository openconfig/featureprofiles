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

package base_vrf_selection_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	trafficDuration   = 1 * time.Minute
	sleepOnChange     = 10 * time.Second
	plen4             = 30
	plen6             = 126
	vlan10            = 10
	vlan20            = 20
	ipipProtocol      = 4
	ipv6ipProtocol    = 41
	ipv4Address       = "198.18.0.1/32"
	ateDestIPv4VLAN10 = "203.0.113.0/30"
	ateDestIPv4VLAN20 = "203.0.113.4/30"
	ateDestIPv6       = "2001:DB8:2::/64"
	defaultNHv4       = "192.0.2.10"
	defaultNHv6       = "2001:db8::a"
	vrfNH             = "192.0.2.6"
)

var (
	ateSrc = attrs.Attributes{
		Name:    "ATE to Ingress Source",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
	dutSrc = attrs.Attributes{
		Desc:    "Ingress to ATE Source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
	ateDst = attrs.Attributes{
		Name:    "ATE to Egress VLAN 10",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
	dutDst = attrs.Attributes{
		Desc:    "Egress VLAN 10 to ATE",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
	ateDst2 = attrs.Attributes{
		Name:    "ATE to Egress VLAN 20",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::a",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
	dutDst2 = attrs.Attributes{
		Desc:    "Egress VLAN 20 to ATE",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::9",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, p1 *ondatra.Port, p2 *ondatra.Port) {
	d := gnmi.OC()

	// Configure ingress interface
	t.Logf("*** Configuring interfaces on DUT ...")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutSrc, 0, 0))

	// Configure egress interface
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst, 10, vlan10))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst2, 20, vlan20))

	if *deviations.ExplicitPortSpeed {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}

	// Configure network instance
	t.Logf("*** Configuring network instance on DUT ... ")
	niConfPath := gnmi.OC().NetworkInstance("VRF-10")
	intfName := *i2.Name + ".10"
	niConf := configNetworkInstance("VRF-10", intfName, "10")
	gnmi.Replace(t, dut, niConfPath.Config(), niConf)

	// Configure default NI and forwarding policy
	t.Logf("*** Configuring default instance forwarding policy on DUT ...")
	dutConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance)
	gnmi.Replace(t, dut, dutConfPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	policyDutConf := configForwardingPolicy()
	gnmi.Replace(t, dut, dutConfPath.PolicyForwarding().Config(), policyDutConf)

}

func configInterfaceDUT(i *oc.Interface, me *attrs.Attributes, subintfindex uint32, vlan uint16) *oc.Interface {
	i.Description = ygot.String(me.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	// Create subinterface.
	s := i.GetOrCreateSubinterface(subintfindex)

	if vlan != 0 {
		// Add VLANs.
		singletag := s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged()
		singletag.VlanId = ygot.Uint16(vlan)
	}
	// Add IPv4 stack.
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled && !*deviations.IPv4MissingEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	a := s4.GetOrCreateAddress(me.IPv4)
	a.PrefixLength = ygot.Uint8(plen4)

	// Add IPv6 stack.
	s6 := s.GetOrCreateIpv6()
	if *deviations.InterfaceEnabled {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(me.IPv6).PrefixLength = ygot.Uint8(plen6)

	return i
}

// Configure Network instance on the DUT
func configNetworkInstance(name string, intf string, id string) *oc.NetworkInstance {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(name)

	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	niIntf := ni.GetOrCreateInterface(intf)
	niIntf.Id = ygot.String(id)
	niIntf.Interface = ygot.String(intf)

	return ni
}

// configDefaultRoute configures a static route in DEFAULT network-instance.
func configDefaultRoute(t *testing.T, dut *ondatra.DUTDevice, v4prefix, v4nexthop, v6prefix, v6nexthop string) {
	t.Logf("*** Configuring static route in DEFAULT network-instance ...")
	ni := oc.NetworkInstance{Name: deviations.DefaultNetworkInstance}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName)
	sr := static.GetOrCreateStatic(v4prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v4nexthop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName).Config(), static)
	sr = static.GetOrCreateStatic(v6prefix)
	nh = sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v6nexthop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName).Config(), static)
}

// configvrfRoute configures a static route.
func configvrfRoute(t *testing.T, dut *ondatra.DUTDevice, v4prefix string, v4nexthop string) {
	t.Logf("*** Configuring static route in VRF-10 network-instance ...")
	ni := oc.NetworkInstance{Name: ygot.String("VRF-10")}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName)
	sr := static.GetOrCreateStatic(v4prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v4nexthop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance("VRF-10").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName).Config(), static)
}

func configForwardingPolicy() *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance)
	// Match policy.
	policyFwding := ni.GetOrCreatePolicyForwarding()

	fwdPolicy1 := policyFwding.GetOrCreatePolicy("match-ipip")
	fwdPolicy1.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	fwdPolicy1.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = oc.UnionUint8(ipipProtocol)
	fwdPolicy1.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("VRF-10")

	fwdPolicy2 := policyFwding.GetOrCreatePolicy("match-ipip-src")
	fwdPolicy2.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = oc.UnionUint8(ipipProtocol)
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateIpv4().SourceAddress = ygot.String(ipv4Address)
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("VRF-10")

	fwdPolicy3 := policyFwding.GetOrCreatePolicy("match-ipv6inipv4")
	fwdPolicy3.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	fwdPolicy3.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = oc.UnionUint8(ipv6ipProtocol)
	fwdPolicy3.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("VRF-10")

	fwdPolicy4 := policyFwding.GetOrCreatePolicy("match-ipv6inipv4-src")
	fwdPolicy4.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	fwdPolicy4.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = oc.UnionUint8(ipv6ipProtocol)
	fwdPolicy4.GetOrCreateRule(1).GetOrCreateIpv4().SourceAddress = ygot.String(ipv4Address)
	fwdPolicy4.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("VRF-10")

	return policyFwding
}

// Apply forwarding policy on the interface.
func applyForwardingPolicy(t *testing.T, ate *ondatra.ATEDevice, ingressPort string, matchType string) {
	t.Logf("*** Applying forwarding policy %v on interface %v ... ", matchType, ingressPort)

	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	pfpath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).PolicyForwarding().Interface(ingressPort)
	gnmi.Delete(t, dut, pfpath.Config())

	intf := d.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance).GetOrCreatePolicyForwarding().GetOrCreateInterface(ingressPort)
	intf.ApplyVrfSelectionPolicy = ygot.String(matchType)
	if *deviations.ExplicitInterfaceRefDefinition {
		intf.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
		intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	}
	// Configure default NI and forwarding policy.
	intfConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).PolicyForwarding().Interface(ingressPort)
	gnmi.Replace(t, dut, intfConfPath.Config(), intf)

	// Restart Protocols after policy change.
	ate.Topology().New().StopProtocols(t)
	time.Sleep(sleepOnChange)
	ate.Topology().New().StartProtocols(t)
	time.Sleep(sleepOnChange)

}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) {
	t.Logf("*** Configuring ATE interfaces ...")
	topo := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	i1 := topo.AddInterface(ateSrc.Name).WithPort(p1)
	i1.IPv4().WithAddress(ateSrc.IPv4CIDR()).WithDefaultGateway(dutSrc.IPv4)
	i1.IPv6().WithAddress(ateSrc.IPv6CIDR()).WithDefaultGateway(dutSrc.IPv6)

	i2 := topo.AddInterface(ateDst.Name).WithPort(p2)
	i2.Ethernet().WithVLANID(vlan10)
	i2.IPv4().WithAddress(ateDst.IPv4CIDR()).WithDefaultGateway(dutDst.IPv4)
	i2.IPv6().WithAddress(ateDst.IPv6CIDR()).WithDefaultGateway(dutDst.IPv6)

	i3 := topo.AddInterface(ateDst2.Name).WithPort(p2)
	i3.Ethernet().WithVLANID(vlan20)
	i3.IPv4().WithAddress(ateDst2.IPv4CIDR()).WithDefaultGateway(dutDst2.IPv4)
	i3.IPv6().WithAddress(ateDst2.IPv6CIDR()).WithDefaultGateway(dutDst2.IPv6)

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)
}

type trafficFlows struct {
	ipInIPFlow1, ipInIPFlow2, ipInIPFlow3, ipInIPFlow4         *ondatra.Flow
	ipv6InIPFlow5, ipv6InIPFlow6, ipv6InIPFlow7, ipv6InIPFlow8 *ondatra.Flow
	nativeIPv4, nativeIPv6                                     *ondatra.Flow
}

func createTrafficFlows(t *testing.T, ate *ondatra.ATEDevice) *trafficFlows {
	topo := ate.Topology().New()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	i1 := topo.AddInterface(ateSrc.Name).WithPort(p1)
	i2 := topo.AddInterface(ateDst.Name).WithPort(p2)
	i3 := topo.AddInterface(ateDst2.Name).WithPort(p2)
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipipHeader := ondatra.NewIPv4Header().WithSrcAddress("198.18.0.1").WithDstAddress("203.0.113.0")
	ipipHeader2 := ondatra.NewIPv4Header().WithSrcAddress("198.51.100.1").WithDstAddress("203.0.113.0")
	ipipHeader4 := ondatra.NewIPv4Header().WithSrcAddress("198.18.0.1").WithDstAddress("203.0.113.4")
	ipipHeader3 := ondatra.NewIPv4Header().WithSrcAddress("198.51.100.1").WithDstAddress("203.0.113.4")
	ipv6Header := ondatra.NewIPv6Header()

	// Create traffic flows.
	t.Logf("*** Configuring ATE flows ...")

	allFlows := new(trafficFlows)
	allFlows.ipInIPFlow1 = ate.Traffic().NewFlow("ipInIPFlow1").WithSrcEndpoints(i1).WithHeaders(ethHeader, ipipHeader2, ipv4Header).WithFrameSize(512).WithFrameRatePct(5).WithDstEndpoints(i2)
	allFlows.ipInIPFlow2 = ate.Traffic().NewFlow("ipInIPFlow2").WithSrcEndpoints(i1).WithHeaders(ethHeader, ipipHeader3, ipv4Header).WithFrameSize(512).WithFrameRatePct(5).WithDstEndpoints(i3)
	allFlows.ipInIPFlow3 = ate.Traffic().NewFlow("ipInIPFlow3").WithSrcEndpoints(i1).WithHeaders(ethHeader, ipipHeader, ipv4Header).WithFrameSize(512).WithFrameRatePct(5).WithDstEndpoints(i2)
	allFlows.ipInIPFlow4 = ate.Traffic().NewFlow("ipInIPFlow4").WithSrcEndpoints(i1).WithHeaders(ethHeader, ipipHeader4, ipv4Header).WithFrameSize(512).WithFrameRatePct(5).WithDstEndpoints(i3)
	allFlows.ipv6InIPFlow5 = ate.Traffic().NewFlow("ipv6InIPFlow5").WithSrcEndpoints(i1).WithHeaders(ethHeader, ipipHeader2, ipv6Header).WithFrameSize(512).WithFrameRatePct(5).WithDstEndpoints(i2)
	allFlows.ipv6InIPFlow6 = ate.Traffic().NewFlow("ipv6InIPFlow6").WithSrcEndpoints(i1).WithHeaders(ethHeader, ipipHeader3, ipv6Header).WithFrameSize(512).WithFrameRatePct(5).WithDstEndpoints(i3)
	allFlows.ipv6InIPFlow7 = ate.Traffic().NewFlow("ipv6InIPFlow7").WithSrcEndpoints(i1).WithHeaders(ethHeader, ipipHeader, ipv6Header).WithFrameSize(512).WithFrameRatePct(5).WithDstEndpoints(i2)
	allFlows.ipv6InIPFlow8 = ate.Traffic().NewFlow("ipv6InIPFlow8").WithSrcEndpoints(i1).WithHeaders(ethHeader, ipipHeader4, ipv6Header).WithFrameSize(512).WithFrameRatePct(5).WithDstEndpoints(i3)
	allFlows.nativeIPv4 = ate.Traffic().NewFlow("nativeIPv4").WithSrcEndpoints(i1).WithDstEndpoints(i3).WithHeaders(ethHeader, ipv4Header).WithFrameSize(512).WithFrameRatePct(5)
	allFlows.nativeIPv6 = ate.Traffic().NewFlow("nativeIPv6").WithSrcEndpoints(i1).WithDstEndpoints(i3).WithHeaders(ethHeader, ipv6Header).WithFrameSize(512).WithFrameRatePct(5)

	return allFlows
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow) {
	t.Logf("*** Sending traffic from ATE ...")
	t.Logf("*** Starting traffic ...")
	ate.Traffic().Start(t, allFlows...)
	time.Sleep(trafficDuration)
	t.Logf("*** Stop traffic ...")
	ate.Traffic().Stop(t)
}

func captureTrafficStats(t *testing.T, ate *ondatra.ATEDevice, flowName string, wantLoss bool) {
	afc := gnmi.OC().Flow(flowName).Counters()
	outPkts := gnmi.Get(t, ate, afc.OutPkts().State())
	t.Logf("ate:Flow out counters %v %v", flowName, outPkts)
	fptest.LogQuery(t, "ate:Flow counters", afc.State(), gnmi.Get(t, ate, afc.State()))

	inPkts := gnmi.Get(t, ate, afc.InPkts().State())
	t.Logf("ate:Flow in counte rs %v %v", flowName, inPkts)

	lostPkts := inPkts - outPkts
	t.Logf("Sent Packets: %d, received Packets: %d", outPkts, inPkts)
	if !wantLoss {
		if lostPkts > 0 {
			t.Logf("Packets received not matching packets sent. Sent: %v, Received: %d", outPkts, inPkts)
		} else {
			t.Logf("Traffic Test Passed! Sent: %d, Received: %d", outPkts, inPkts)
		}
	}
}

func contains(item string, items []string) bool {
	flag := false
	for _, j := range items {
		if item == j {
			return true
		}
	}
	return flag
}

// verifyTraffic confirms that every traffic flow has the expected amount of loss (0% or 100%).
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, flows []*ondatra.Flow, passFlows []string) {
	for _, flow := range flows {
		t.Logf("*** Verifying %v traffic on ATE ... ", flow.Name())
		captureTrafficStats(t, ate, flow.Name(), false)
		lossPct := gnmi.Get(t, ate, gnmi.OC().Flow(flow.Name()).LossPct().State())
		if contains(flow.Name(), passFlows) {
			if lossPct > 0 {
				t.Errorf("Traffic Loss Pct for Flow: %s got %v, want 0", flow.Name(), lossPct)
			} else {
				t.Logf("Traffic Test Passed!")
			}
		} else {
			if lossPct < 100 {
				t.Errorf("Traffic is expected to fail %s got %v, want 100%% failure", flow.Name(), lossPct)
			} else {
				t.Logf("Traffic Test Passed!")
			}
		}
	}
}

func TestVrfPolicy(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	// Configure DUT interfaces and forwarding policy.
	configureDUT(t, dut, p1, p2)
	configDefaultRoute(t, dut, ateDestIPv4VLAN20, defaultNHv4, ateDestIPv6, defaultNHv6)
	configvrfRoute(t, dut, ateDestIPv4VLAN10, vrfNH)

	// Configure ATE.
	ate := ondatra.ATE(t, "ate")
	configureATE(t, ate)

	allFlows := createTrafficFlows(t, ate)

	tcs := []struct {
		desc        string
		policy      string
		endpoints   []string
		streamNames []string
		streams     []*ondatra.Flow
	}{
		{
			desc:   "Match IP in IP.",
			policy: "match-ipip",
			streams: []*ondatra.Flow{allFlows.ipInIPFlow1, allFlows.ipInIPFlow3, allFlows.ipv6InIPFlow6,
				allFlows.ipv6InIPFlow8, allFlows.nativeIPv4, allFlows.nativeIPv6},
			streamNames: []string{"ipInIPFlow1", "ipInIPFlow3", "ipv6InIPFlow6", "ipv6InIPFlow8", "nativeIPv4", "nativeIPv6"},
		},
		{
			desc:   "Match IPinIP with Source IP.",
			policy: "match-ipip-src",
			streams: []*ondatra.Flow{allFlows.ipInIPFlow2, allFlows.ipInIPFlow3, allFlows.ipv6InIPFlow6,
				allFlows.ipv6InIPFlow8, allFlows.nativeIPv4, allFlows.nativeIPv6},
			streamNames: []string{"ipInIPFlow2", "ipInIPFlow3", "ipv6InIPFlow6", "ipv6InIPFlow8", "nativeIPv4", "nativeIPv6"},
		},
		{
			desc:   "Match IPv6 in IP.",
			policy: "match-ipv6inipv4",
			streams: []*ondatra.Flow{allFlows.ipInIPFlow2, allFlows.ipInIPFlow4, allFlows.ipv6InIPFlow5,
				allFlows.ipv6InIPFlow7, allFlows.nativeIPv4, allFlows.nativeIPv6},
			streamNames: []string{"ipInIPFlow2", "ipInIPFlow4", "ipv6InIPFlow5", "ipv6InIPFlow7", "nativeIPv4", "nativeIPv6"},
		},
		{
			desc:   "Match IPv6 in IP with Source IP.",
			policy: "match-ipv6inipv4-src",
			streams: []*ondatra.Flow{allFlows.ipInIPFlow2, allFlows.ipInIPFlow4, allFlows.ipv6InIPFlow6,
				allFlows.ipv6InIPFlow7, allFlows.nativeIPv4, allFlows.nativeIPv6},
			streamNames: []string{"ipInIPFlow2", "ipInIPFlow4", "ipv6InIPFlow6", "ipv6InIPFlow7", "nativeIPv4", "nativeIPv6"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			applyForwardingPolicy(t, ate, p1.Name(), tc.policy)
			sendTraffic(t, ate, tc.streams)
			verifyTraffic(t, ate, tc.streams, tc.streamNames)
		})
	}
}
