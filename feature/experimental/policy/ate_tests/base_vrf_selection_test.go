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
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.  IxNetwork flow requires both source and destination
// networks be configured on the ATE.  It is not possible to send
// packets to the ether.
//
// The testbed consists of ate:port1 -> dut:port1 and
// dut:port2 -> ate:port2.  The first pair is called the "source"
// pair, and the second the "destination" pair.
//
//   * Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30 2001:db8::0/126
//   * Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30 2001:db8::4/126
//
// Note that the first (.0, .4) and last (.3, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses.  This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.
//
// A traffic flow is configured from ate:port1 as the source interface
// and ate:port2 as the destination interface.  The traffic should
// flow as expected both when using dynamic or static ARP since the
// Ixia interfaces are promiscuous.  However, using custom egress
// filter, we can tell if the static ARP is honored or not.
//
// Synthesized static MAC addresses have the form 02:1a:WW:XX:YY:ZZ
// where WW:XX:YY:ZZ are the four octets of the IPv4 in hex.  The 0x02
// means the MAC address is locally administered.
const (
	trafficDuration = 1 * time.Minute
	sleepOnChange   = 10 * time.Second
	plen4           = 30
	plen6           = 126
	vlan1           = 10
	vlan2           = 20
	tolerance       = 0
	tolerancePct    = 0
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
		Desc:    "Ingress to ATE source",
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
	d := dut.Config()

	//Configure ingress interface
	fmt.Println("*** Configuring interfaces on DUT ...")
	i1 := &telemetry.Interface{Name: ygot.String(p1.Name())}
	d.Interface(p1.Name()).Replace(t, configInterfaceDUT(i1, &dutSrc, &ateSrc, 0, 0))

	//Configure egress interface
	i2 := &telemetry.Interface{Name: ygot.String(p2.Name())}
	d.Interface(p2.Name()).Replace(t, configInterfaceDUT(i2, &dutDst, &ateDst, 0, vlan1))
	d.Interface(p2.Name()).Replace(t, configInterfaceDUT(i2, &dutDst2, &ateDst2, 1, vlan2))

	//Configure network instance
	fmt.Println("*** Configuring network instance on DUT ... ")
	niConfPath := dut.Config().NetworkInstance("10")
	niConf := configNetworkInstance("10", &ateDst)
	niConfPath.Replace(t, niConf)
	niConfPath = dut.Config().NetworkInstance("20")
	niConf = configNetworkInstance("20", &ateDst2)
	niConfPath.Replace(t, niConf)

	//Configure default NI and forwarding policy
	fmt.Println("*** Configuring default instance forwarding policy on DUT ...")
	policyConfPath := dut.Config().NetworkInstance("default").PolicyForwarding()
	policyDutConf := configForwardingPolicy(p1.Name(), "default", &ateSrc)
	policyConfPath.Replace(t, policyDutConf)
}

func configInterfaceDUT(i *telemetry.Interface, me, peer *attrs.Attributes, subintfindex uint32, vlan uint16) *telemetry.Interface {
	i.Description = ygot.String(me.Desc)
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	//Create subinterface
	s := i.GetOrCreateSubinterface(subintfindex)

	if vlan != 0 {
		//Add VLANs
		singletag := s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged()
		singletag.VlanId = ygot.Uint16(vlan)
	}
	//Add IPv4 stack
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	a := s4.GetOrCreateAddress(me.IPv4)
	a.PrefixLength = ygot.Uint8(plen4)

	//Add IPv6 stack
	s6 := s.GetOrCreateIpv6()
	if *deviations.InterfaceEnabled {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(me.IPv6).PrefixLength = ygot.Uint8(plen6)

	return i
}

// Configure Network instance on the DUT
func configNetworkInstance(name string, peer *attrs.Attributes) *telemetry.NetworkInstance {

	d := &telemetry.Device{}
	ni := d.GetOrCreateNetworkInstance(name)

	ni.Type = telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L2L3
	static := ni.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "STATIC")
	ipv4Nh := static.GetOrCreateStatic("0.0.0.0/0").GetOrCreateNextHop(peer.IPv4)
	ipv6Nh := static.GetOrCreateStatic("::/0").GetOrCreateNextHop(peer.IPv6)
	ipv4Nh.Recurse = ygot.Bool(true)
	ipv6Nh.Recurse = ygot.Bool(true)

	return ni
}

func configForwardingPolicy(name string, matchType string, ate *attrs.Attributes) *telemetry.NetworkInstance_PolicyForwarding {

	d := &telemetry.Device{}
	ni := d.GetOrCreateNetworkInstance(name)
	ipv4Address := "0.0.0.0/0"
	ipv6Address := "::/0"
	//TODO dscpset := []uint8{42, 46}

	//Match policy
	policyFwding := ni.GetOrCreatePolicyForwarding()

	fwdPolicy1 := policyFwding.GetOrCreatePolicy("Match-IpipDscp4246")
	fwdPolicy1.GetOrCreateRule(1).GetOrCreateIpv4().Dscp = ygot.Uint8(46)
	//TODO fwdPolicy1.GetOrCreateRule(1).GetOrCreateIpv4().DscpSet = dscpset
	fwdPolicy1.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	fwdPolicy1.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy1.GetOrCreateRule(2).GetOrCreateIpv4().Dscp = ygot.Uint8(42)
	fwdPolicy1.GetOrCreateRule(2).GetOrCreateIpv4().Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	fwdPolicy1.GetOrCreateRule(2).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy1.GetOrCreateRule(3).GetOrCreateIpv4().SourceAddress = ygot.String(ipv4Address)
	fwdPolicy1.GetOrCreateRule(3).GetOrCreateAction().Discard = ygot.Bool(true)
	fwdPolicy1.GetOrCreateRule(1).GetOrCreateIpv6().Dscp = ygot.Uint8(46)
	//TODO fwdPolicy1.GetOrCreateRule(1).GetOrCreateIpv6().DscpSet = dscpset
	fwdPolicy1.GetOrCreateRule(1).GetOrCreateIpv6().Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	fwdPolicy1.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy1.GetOrCreateRule(2).GetOrCreateIpv6().Dscp = ygot.Uint8(42)
	fwdPolicy1.GetOrCreateRule(2).GetOrCreateIpv6().Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	fwdPolicy1.GetOrCreateRule(2).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy1.GetOrCreateRule(3).GetOrCreateIpv6().SourceAddress = ygot.String(ipv6Address)
	fwdPolicy1.GetOrCreateRule(3).GetOrCreateAction().Discard = ygot.Bool(true)

	fwdPolicy2 := policyFwding.GetOrCreatePolicy("Match-IpipDscp46")
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateIpv4().Dscp = ygot.Uint8(46)
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy2.GetOrCreateRule(2).GetOrCreateIpv4().SourceAddress = ygot.String(ipv4Address)
	fwdPolicy2.GetOrCreateRule(2).GetOrCreateAction().Discard = ygot.Bool(true)
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateIpv6().Dscp = ygot.Uint8(46)
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateIpv6().Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy2.GetOrCreateRule(2).GetOrCreateIpv6().SourceAddress = ygot.String(ipv6Address)
	fwdPolicy2.GetOrCreateRule(2).GetOrCreateAction().Discard = ygot.Bool(true)

	fwdPolicy3 := policyFwding.GetOrCreatePolicy("Match-IpipNoDscp")
	fwdPolicy3.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	fwdPolicy3.GetOrCreateRule(1).GetOrCreateIpv4().Dscp = ygot.Uint8(0)
	fwdPolicy3.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy3.GetOrCreateRule(2).GetOrCreateIpv4().SourceAddress = ygot.String(ipv4Address)
	fwdPolicy3.GetOrCreateRule(2).GetOrCreateAction().Discard = ygot.Bool(true)
	fwdPolicy3.GetOrCreateRule(1).GetOrCreateIpv6().Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	fwdPolicy3.GetOrCreateRule(1).GetOrCreateIpv6().Dscp = ygot.Uint8(0)
	fwdPolicy3.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy3.GetOrCreateRule(2).GetOrCreateIpv6().SourceAddress = ygot.String(ipv6Address)
	fwdPolicy3.GetOrCreateRule(2).GetOrCreateAction().Discard = ygot.Bool(true)

	fwdPolicy4 := policyFwding.GetOrCreatePolicy("Match-Ipv4")
	fwdPolicy4.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	fwdPolicy4.GetOrCreateRule(1).GetOrCreateIpv6().SourceAddress = ygot.String(ipv6Address)
	fwdPolicy4.GetOrCreateRule(1).GetOrCreateAction().Discard = ygot.Bool(true)
	fwdPolicy4.GetOrCreateRule(2).GetOrCreateIpv4().SourceAddress = ygot.String(ipv4Address)
	fwdPolicy4.GetOrCreateRule(2).GetOrCreateAction().NetworkInstance = ygot.String("10")

	fwdPolicy5 := policyFwding.GetOrCreatePolicy("Match-Ipv4v6")
	fwdPolicy5.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	fwdPolicy5.GetOrCreateRule(1).GetOrCreateAction().Discard = ygot.Bool(true)
	fwdPolicy5.GetOrCreateRule(2).GetOrCreateIpv4().SourceAddress = ygot.String(ipv4Address)
	fwdPolicy5.GetOrCreateRule(2).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy5.GetOrCreateRule(1).GetOrCreateIpv6().Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	fwdPolicy5.GetOrCreateRule(1).GetOrCreateAction().Discard = ygot.Bool(true)
	fwdPolicy5.GetOrCreateRule(2).GetOrCreateIpv6().Protocol = telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP
	fwdPolicy5.GetOrCreateRule(2).GetOrCreateAction().Discard = ygot.Bool(true)
	fwdPolicy5.GetOrCreateRule(3).GetOrCreateIpv6().SourceAddress = ygot.String(ipv6Address)
	fwdPolicy5.GetOrCreateRule(3).GetOrCreateAction().NetworkInstance = ygot.String("20")

	return policyFwding
}

func applyForwardingPolicy(t *testing.T, ate *ondatra.ATEDevice, ingressPort string, matchType string) {

	fmt.Println("*** Applying forwarding policy ", matchType, " on interface", ingressPort, " ...")

	d := &telemetry.Device{}
	dut := ondatra.DUT(t, "dut")

	intf := d.GetOrCreateNetworkInstance("default").GetOrCreatePolicyForwarding().GetOrCreateInterface(ingressPort)

	intf.ApplyForwardingPolicy = ygot.String(matchType)
	intf.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)

	//Configure default NI and forwarding policy
	intfConfPath := dut.Config().NetworkInstance("default").PolicyForwarding().Interface(ingressPort)
	intfConfPath.Replace(t, intf)

	//Restart Protocols after policy change
	ate.Topology().New().StopProtocols(t)
	time.Sleep(sleepOnChange)
	ate.Topology().New().StartProtocols(t)
	time.Sleep(sleepOnChange)

}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) []*ondatra.Flow {

	fmt.Println("*** Configuring ATE interfaces ...")
	topo := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	i1 := topo.AddInterface(ateSrc.Name).WithPort(p1)
	i1.IPv4().WithAddress(ateSrc.IPv4CIDR()).WithDefaultGateway(dutSrc.IPv4)
	i1.IPv6().WithAddress(ateSrc.IPv6CIDR()).WithDefaultGateway(dutSrc.IPv6)

	i2 := topo.AddInterface(ateDst.Name).WithPort(p2)
	i2.Ethernet().WithVLANID(vlan1)
	i2.IPv4().WithAddress(ateDst.IPv4CIDR()).WithDefaultGateway(dutDst.IPv4)
	i2.IPv6().WithAddress(ateDst.IPv6CIDR()).WithDefaultGateway(dutDst.IPv6)

	i3 := topo.AddInterface(ateDst2.Name).WithPort(p2)
	i3.Ethernet().WithVLANID(vlan2)
	i3.IPv4().WithAddress(ateDst2.IPv4CIDR()).WithDefaultGateway(dutDst2.IPv4)
	i3.IPv6().WithAddress(ateDst2.IPv6CIDR()).WithDefaultGateway(dutDst2.IPv6)

	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipipHeader := ondatra.NewIPv4Header()
	ipv6Header := ondatra.NewIPv6Header()
	ipipDscp46Header := ondatra.NewIPv4Header().WithDSCP(46)
	ipipDscp42Header := ondatra.NewIPv4Header().WithDSCP(42)

	//Create traffic flows
	fmt.Println("*** Configuring ATE flows ...")
	ipv4FlowVlan10 := ate.Traffic().NewFlow("Ipv4Vlan10").WithSrcEndpoints(i1).WithDstEndpoints(i2).WithHeaders(ethHeader, ipv4Header).WithFrameSize(512).WithFrameRatePct(5)
	ipv6FlowVlan10 := ate.Traffic().NewFlow("Ipv6Vlan10").WithSrcEndpoints(i1).WithDstEndpoints(i2).WithHeaders(ethHeader, ipv6Header).WithFrameSize(512).WithFrameRatePct(5)
	ipipFlowVlan10 := ate.Traffic().NewFlow("IpipVlan10").WithSrcEndpoints(i1).WithDstEndpoints(i2).WithHeaders(ethHeader, ipipHeader, ipv4Header).WithFrameSize(512).WithFrameRatePct(5)
	ipipDscp46FlowVlan10 := ate.Traffic().NewFlow("IpipDscp46Vlan10").WithSrcEndpoints(i1).WithDstEndpoints(i2).WithHeaders(ethHeader, ipipDscp46Header, ipv4Header).WithFrameSize(512).WithFrameRatePct(5)
	ipipDscp42FlowVlan10 := ate.Traffic().NewFlow("IpipDscp42Vlan10").WithSrcEndpoints(i1).WithDstEndpoints(i2).WithHeaders(ethHeader, ipipDscp42Header, ipv4Header).WithFrameSize(512).WithFrameRatePct(5)
	ipv4FlowVlan20 := ate.Traffic().NewFlow("Ipv4Vlan20").WithSrcEndpoints(i1).WithDstEndpoints(i3).WithHeaders(ethHeader, ipv4Header).WithFrameSize(512).WithFrameRatePct(5)
	ipv6FlowVlan20 := ate.Traffic().NewFlow("Ipv6Vlan20").WithSrcEndpoints(i1).WithDstEndpoints(i3).WithHeaders(ethHeader, ipv6Header).WithFrameSize(512).WithFrameRatePct(5)
	ipipFlowVlan20 := ate.Traffic().NewFlow("IpipVlan20").WithSrcEndpoints(i1).WithDstEndpoints(i3).WithHeaders(ethHeader, ipipHeader, ipv4Header).WithFrameSize(512).WithFrameRatePct(5)
	ipipDscp46FlowVlan20 := ate.Traffic().NewFlow("IpipDscp46Vlan20").WithSrcEndpoints(i1).WithDstEndpoints(i3).WithHeaders(ethHeader, ipipDscp46Header, ipv4Header).WithFrameSize(512).WithFrameRatePct(5)
	ipipDscp42FlowVlan20 := ate.Traffic().NewFlow("IpipDscp42Vlan20").WithSrcEndpoints(i1).WithDstEndpoints(i3).WithHeaders(ethHeader, ipipDscp42Header, ipv4Header).WithFrameSize(512).WithFrameRatePct(5)

	t.Logf("Pushing config to ATE and starting protocols...")
	topo.Push(t)
	topo.StartProtocols(t)
	return []*ondatra.Flow{ipv4FlowVlan10, ipv6FlowVlan10, ipipFlowVlan10, ipipDscp46FlowVlan10, ipipDscp42FlowVlan10,
		ipv4FlowVlan20, ipv6FlowVlan20, ipipFlowVlan20, ipipDscp46FlowVlan20, ipipDscp42FlowVlan20}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, allFlows []*ondatra.Flow) {

	fmt.Println("*** Sending traffic from ATE ...")
	t.Logf("*** Starting traffic ...")
	ate.Traffic().Start(t, allFlows...)
	time.Sleep(trafficDuration)
	t.Logf("*** Stop traffic ...")
	ate.Traffic().Stop(t)
}

func captureTrafficStats(t *testing.T, ate *ondatra.ATEDevice, flowName string, wantLoss bool) {
	afc := ate.Telemetry().Flow(flowName).Counters()
	outPkts := afc.OutPkts().Get(t)
	fmt.Println("ate:Flow out counters", flowName, outPkts)
	fptest.LogYgot(t, "ate:Flow counters", afc, afc.Get(t))

	inPkts := afc.InPkts().Get(t)
	fmt.Println("ate:Flow in counters", flowName, inPkts)

	lostPkts := inPkts - outPkts
	t.Logf("Sent Packets: %d, received Packets: %d", outPkts, inPkts)
	if !wantLoss {
		if lostPkts > tolerance {
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

// verifyTraffic confirms that every traffic flow has the expected amount of loss (0% or 100%
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, flows []*ondatra.Flow, passFlows []string) {

	for _, flow := range flows {
		fmt.Println("*** Verifying ", flow.Name(), "traffic on ATE ...")
		captureTrafficStats(t, ate, flow.Name(), false)
		lossPct := ate.Telemetry().Flow(flow.Name()).LossPct().Get(t)
		if contains(flow.Name(), passFlows) {
			if lossPct > tolerancePct {
				fmt.Println("Traffic Loss Pct for Flow:", flow.Name(), "got ", lossPct, " want 0")
				t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want 0", flow.Name(), lossPct)
			} else {
				fmt.Println("Traffic Test Passed!")
				t.Logf("Traffic Test Passed!")
			}
		} else {
			if lossPct < 100-tolerancePct {
				fmt.Println("Traffic is expected to fail for Flow:", flow.Name(), "got ", lossPct, " want 100%% failure")
				t.Errorf("Traffic is expected to fail %s\n got %v, want 100%% failure", flow.Name(), lossPct)
			} else {
				fmt.Println("Traffic Test Passed!")
				t.Logf("Traffic Loss Test Passed!")
			}
		}
	}
}

func TestVrfPolicy(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	//Configure DUT interfaces and forwarding policy
	configureDUT(t, dut, p1, p2)

	//Configure ATE
	ate := ondatra.ATE(t, "ate")
	allFlows := configureATE(t, ate)

	// Verify ATE Traffic
	t.Run("VRF_Selection", func(t *testing.T) {
		t.Run("MatchIPv4", func(t *testing.T) {
			applyForwardingPolicy(t, ate, p1.Name(), "Match-Ipv4")
			sendTraffic(t, ate, allFlows)
			passFlows := []string{"Ipv4Vlan10"}
			verifyTraffic(t, ate, allFlows, passFlows)
		})
		t.Run("MatchIPv4v6", func(t *testing.T) {
			applyForwardingPolicy(t, ate, p1.Name(), "Match-Ipv4v6")
			sendTraffic(t, ate, allFlows)
			passFlows := []string{"Ipv4Vlan10", "Ipv6Vlan20"}
			verifyTraffic(t, ate, allFlows, passFlows)
		})
		t.Run("MatchIPIP", func(t *testing.T) {
			applyForwardingPolicy(t, ate, p1.Name(), "Match-IpipNoDscp")
			sendTraffic(t, ate, allFlows)
			passFlows := []string{"IpipVlan10"}
			verifyTraffic(t, ate, allFlows, passFlows)
		})
		t.Run("MatchIPIPDscp46", func(t *testing.T) {
			applyForwardingPolicy(t, ate, p1.Name(), "Match-IpipDscp46")
			sendTraffic(t, ate, allFlows)
			passFlows := []string{"IpipDscp46Vlan10"}
			verifyTraffic(t, ate, allFlows, passFlows)
		})
		t.Run("MatchIPIPDscp4246", func(t *testing.T) {
			applyForwardingPolicy(t, ate, p1.Name(), "Match-IpipDscp42")
			sendTraffic(t, ate, allFlows)
			passFlows := []string{"IpipDscp46Vlan10", "IpipDscp42Vlan10"}
			verifyTraffic(t, ate, allFlows, passFlows)
		})
	})

}
