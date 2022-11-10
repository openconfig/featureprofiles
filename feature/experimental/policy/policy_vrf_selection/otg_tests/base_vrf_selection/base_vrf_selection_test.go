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

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	trafficDuration = 1 * time.Minute
	sleepOnChange   = 10 * time.Second
	plen4           = 30
	plen6           = 126
	vlan10          = 10
	vlan20          = 20
	ipipProtocol    = 4
)

var (
	ateSrc = attrs.Attributes{
		Name:    "ATE to Ingress Source",
		MAC:     "02:00:01:01:01:01",
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
		MAC:     "02:00:02:01:01:01",
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
		MAC:     "02:00:03:01:01:01",
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

	// Configure ingress interface
	t.Logf("*** Configuring interfaces on DUT ...")
	i1 := &telemetry.Interface{Name: ygot.String(p1.Name())}
	d.Interface(p1.Name()).Replace(t, configInterfaceDUT(i1, &dutSrc, &ateSrc, 0, 0))

	// Configure egress interface
	i2 := &telemetry.Interface{Name: ygot.String(p2.Name())}
	d.Interface(p2.Name()).Replace(t, configInterfaceDUT(i2, &dutDst, &ateDst, 1, vlan10))
	d.Interface(p2.Name()).Replace(t, configInterfaceDUT(i2, &dutDst2, &ateDst2, 2, vlan20))

	// Configure network instance
	t.Logf("*** Configuring network instance on DUT ... ")
	niConfPath := dut.Config().NetworkInstance("10")
	niConf := configNetworkInstance("10", &ateDst)
	niConfPath.Replace(t, niConf)
	niConfPath = dut.Config().NetworkInstance("20")
	niConf = configNetworkInstance("20", &ateDst2)
	niConfPath.Replace(t, niConf)

	// Configure default NI and forwarding policy
	t.Logf("*** Configuring default instance forwarding policy on DUT ...")
	dutConfPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance)
	dutConfPath.Type().Replace(t, telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	// policyDutConf := configForwardingPolicy()
	// dutConfPath.PolicyForwarding().Replace(t, policyDutConf)
}

func configInterfaceDUT(i *telemetry.Interface, me, peer *attrs.Attributes, subintfindex uint32, vlan uint16) *telemetry.Interface {
	i.Description = ygot.String(me.Desc)
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	// Create subinterface
	s := i.GetOrCreateSubinterface(subintfindex)

	if vlan != 0 {
		// Add VLANs
		singletag := s.GetOrCreateVlan()
		singletag.VlanId = telemetry.UnionUint16(vlan)
	}
	// Add IPv4 stack
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled && !*deviations.IPv4MissingEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	a := s4.GetOrCreateAddress(me.IPv4)
	a.PrefixLength = ygot.Uint8(plen4)

	// Add IPv6 stack
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

	ni.Type = telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	static := ni.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName)
	ipv4Nh := static.GetOrCreateStatic("0.0.0.0/0").GetOrCreateNextHop("0")
	ipv4Nh.NextHop, _ = ipv4Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(peer.IPv4)
	ipv6Nh := static.GetOrCreateStatic("::/0").GetOrCreateNextHop("1")
	ipv6Nh.NextHop, _ = ipv6Nh.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(peer.IPv6)
	// ipv4Nh.Recurse = ygot.Bool(true)
	// ipv6Nh.Recurse = ygot.Bool(true)

	return ni
}

func configForwardingPolicy() *telemetry.NetworkInstance_PolicyForwarding {
	d := &telemetry.Device{}
	ni := d.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance)
	ipv4Address := "0.0.0.0/0"
	ipv6Address := "::/0"
	ipv6dest := "2001:db8::1/128"

	// Match policy
	policyFwding := ni.GetOrCreatePolicyForwarding()

	fwdPolicy1 := policyFwding.GetOrCreatePolicy("match-ipv4")
	fwdPolicy1.GetOrCreateRule(1).GetOrCreateIpv4().DestinationAddress = ygot.String(ipv4Address)
	fwdPolicy1.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy1.GetOrCreateRule(2).GetOrCreateIpv6().DestinationAddress = ygot.String(ipv6dest)
	fwdPolicy1.GetOrCreateRule(2).GetOrCreateAction().Discard = ygot.Bool(false)
	fwdPolicy1.GetOrCreateRule(3).GetOrCreateIpv4().DestinationAddress = ygot.String(ipv4Address)
	fwdPolicy1.GetOrCreateRule(3).GetOrCreateIpv6().DestinationAddress = ygot.String(ipv6Address)
	fwdPolicy1.GetOrCreateRule(3).GetOrCreateAction().Discard = ygot.Bool(true)

	fwdPolicy2 := policyFwding.GetOrCreatePolicy("match-ipip")
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = telemetry.UnionUint8(ipipProtocol)
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy2.GetOrCreateRule(2).GetOrCreateIpv4().DestinationAddress = ygot.String(ipv4Address)
	fwdPolicy2.GetOrCreateRule(2).GetOrCreateIpv6().DestinationAddress = ygot.String(ipv6Address)
	fwdPolicy2.GetOrCreateRule(2).GetOrCreateAction().Discard = ygot.Bool(true)

	fwdPolicy3 := policyFwding.GetOrCreatePolicy("match-ip4ip6")
	fwdPolicy3.GetOrCreateRule(1).GetOrCreateIpv4().DestinationAddress = ygot.String(ipv4Address)
	fwdPolicy3.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy3.GetOrCreateRule(2).GetOrCreateIpv6().DestinationAddress = ygot.String(ipv6Address)
	fwdPolicy3.GetOrCreateRule(2).GetOrCreateAction().NetworkInstance = ygot.String("20")

	fwdPolicy4 := policyFwding.GetOrCreatePolicy("match-ipip-dscp46")
	fwdPolicy4.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = telemetry.UnionUint8(ipipProtocol)
	fwdPolicy4.GetOrCreateRule(1).GetOrCreateIpv4().Dscp = ygot.Uint8(46)
	fwdPolicy4.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy4.GetOrCreateRule(2).GetOrCreateIpv4().DestinationAddress = ygot.String(ipv4Address)
	fwdPolicy4.GetOrCreateRule(2).GetOrCreateIpv6().DestinationAddress = ygot.String(ipv6Address)
	fwdPolicy4.GetOrCreateRule(2).GetOrCreateAction().Discard = ygot.Bool(true)

	fwdPolicy5 := policyFwding.GetOrCreatePolicy("match-ipip-dscp42or46")
	fwdPolicy5.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = telemetry.UnionUint8(ipipProtocol)
	fwdPolicy5.GetOrCreateRule(1).GetOrCreateIpv4().Dscp = ygot.Uint8(42)
	fwdPolicy5.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy5.GetOrCreateRule(2).GetOrCreateIpv4().Protocol = telemetry.UnionUint8(ipipProtocol)
	fwdPolicy5.GetOrCreateRule(2).GetOrCreateIpv4().Dscp = ygot.Uint8(46)
	fwdPolicy5.GetOrCreateRule(2).GetOrCreateAction().NetworkInstance = ygot.String("10")
	fwdPolicy5.GetOrCreateRule(3).GetOrCreateIpv4().DestinationAddress = ygot.String(ipv4Address)
	fwdPolicy5.GetOrCreateRule(3).GetOrCreateIpv6().DestinationAddress = ygot.String(ipv6Address)
	fwdPolicy3.GetOrCreateRule(3).GetOrCreateAction().Discard = ygot.Bool(true)

	return policyFwding
}

// Apply forwarding policy on the interface
func applyForwardingPolicy(t *testing.T, ate *ondatra.ATEDevice, ingressPort string, matchType string) {
	t.Logf("*** Applying forwarding policy %v on interface %v ... ", matchType, ingressPort)

	d := &telemetry.Device{}
	dut := ondatra.DUT(t, "dut")

	intf := d.GetOrCreateNetworkInstance(*deviations.DefaultNetworkInstance).GetOrCreatePolicyForwarding().GetOrCreateInterface(ingressPort)
	intf.ApplyForwardingPolicy = ygot.String(matchType)
	intf.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)

	// Configure default NI and forwarding policy
	intfConfPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).PolicyForwarding().Interface(ingressPort)
	intfConfPath.Replace(t, intf)

	// Restart Protocols after policy change
	ate.OTG().StopProtocols(t)
	time.Sleep(sleepOnChange)
	ate.OTG().StartProtocols(t)
	time.Sleep(sleepOnChange)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) []gosnappi.Flow {
	t.Logf("*** Configuring OTG interfaces ...")
	topo := ate.OTG().NewConfig(t)

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	topo.Ports().Add().SetName(p1.ID())
	srcDev := topo.Devices().Add().SetName(ateSrc.Name)
	ethSrc := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth")
	ethSrc.SetPortName(p2.ID()).SetMac(ateSrc.MAC)
	ethSrc.Ipv4Addresses().Add().SetName(srcDev.Name() + ".IPv4").SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(int32(ateSrc.IPv4Len))
	ethSrc.Ipv6Addresses().Add().SetName(srcDev.Name() + ".IPv6").SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(int32(ateSrc.IPv6Len))

	topo.Ports().Add().SetName(p2.ID())
	dstDev := topo.Devices().Add().SetName(ateDst.Name)
	eth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth")
	eth.SetPortName(p2.ID()).SetMac(ateDst.MAC)
	eth.Vlans().Add().SetName(dstDev.Name() + "VLAN").SetId(int32(vlan10))
	eth.Ipv4Addresses().Add().SetName(dstDev.Name() + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(int32(ateDst.IPv4Len))
	eth.Ipv6Addresses().Add().SetName(dstDev.Name() + ".IPv6").SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(int32(ateDst.IPv6Len))

	dstDev2 := topo.Devices().Add().SetName(ateDst2.Name)
	eth2 := dstDev2.Ethernets().Add().SetName(ateDst2.Name + ".Eth")
	eth2.SetPortName(p2.ID()).SetMac(ateDst2.MAC)
	eth2.Vlans().Add().SetName(dstDev2.Name() + "VLAN").SetId(int32(vlan20))
	eth2.Ipv4Addresses().Add().SetName(dstDev2.Name() + ".IPv4").SetAddress(ateDst2.IPv4).SetGateway(dutDst2.IPv4).SetPrefix(int32(ateDst2.IPv4Len))
	eth2.Ipv6Addresses().Add().SetName(dstDev2.Name() + ".IPv6").SetAddress(ateDst2.IPv6).SetGateway(dutDst2.IPv6).SetPrefix(int32(ateDst2.IPv6Len))

	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)

	// Create traffic flows
	t.Logf("*** Configuring OTG flows ...")
	topo.Flows().Clear().Items()
	ipv4FlowVlan10 := createFlow("Ipv4Vlan10", topo, "IPv4", false, 10, 0, ateDst)
	ipv6FlowVlan10 := createFlow("Ipv6Vlan10", topo, "IPv6", false, 10, 0, ateDst)
	ipipFlowVlan10 := createFlow("IpipVlan10", topo, "IPv4", true, 10, 0, ateDst)
	ipipDscp46FlowVlan10 := createFlow("IpipDscp46Vlan10", topo, "IPv4", true, 10, 46, ateDst)
	ipipDscp42FlowVlan10 := createFlow("IpipDscp42Vlan10", topo, "IPv4", true, 10, 42, ateDst)
	ipv4FlowVlan20 := createFlow("Ipv4Vlan20", topo, "IPv4", false, 20, 0, ateDst2)
	ipv6FlowVlan20 := createFlow("Ipv6Vlan20", topo, "IPv6", false, 20, 0, ateDst2)
	ipipFlowVlan20 := createFlow("IpipVlan20", topo, "IPv4", true, 20, 0, ateDst2)
	ipipDscp46FlowVlan20 := createFlow("IpipDscp46Vlan20", topo, "IPv4", true, 20, 46, ateDst2)
	ipipDscp42FlowVlan20 := createFlow("IpipDscp42Vlan20", topo, "IPv4", true, 20, 42, ateDst2)

	t.Logf("Pushing config to ATE and starting protocols...")
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	return []gosnappi.Flow{ipv4FlowVlan10, ipv6FlowVlan10, ipipFlowVlan10, ipipDscp46FlowVlan10, ipipDscp42FlowVlan10,
		ipv4FlowVlan20, ipv6FlowVlan20, ipipFlowVlan20, ipipDscp46FlowVlan20, ipipDscp42FlowVlan20}
}

func createFlow(name string, top gosnappi.Config, ipType string, IPinIP bool, vlanID, dscp int32, dst attrs.Attributes) gosnappi.Flow {

	flow := top.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{ateSrc.Name + "." + ipType}).SetRxNames([]string{dst.Name + "." + ipType})
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(ateSrc.MAC)
	flow.Packet().Add().Vlan().Id().SetValue(vlanID)
	if ipType == "IPv4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(ateSrc.IPv4)
		v4.Dst().SetValue(dst.IPv4)
		if dscp != 0 {
			v4.Priority().Dscp().Phb().SetValue(dscp)
		}
	}
	if ipType == "IPv6" {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(ateSrc.IPv6)
		v6.Dst().SetValue(dst.IPv6)
	}
	if IPinIP {
		flow.Packet().Add().Ipv4()
	}
	flow.Size().SetFixed(512)
	flow.Rate().SetChoice("percentage").SetPercentage(5)
	return flow
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	t.Logf("*** Starting traffic ...")
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	t.Logf("*** Stop traffic ...")
	ate.OTG().StopTraffic(t)
}

func captureTrafficStats(t *testing.T, ate *ondatra.ATEDevice, flowName string, wantLoss bool) {
	afc := ate.OTG().Telemetry().Flow(flowName).Counters()
	outPkts := ate.OTG().Telemetry().Flow(flowName).Counters().OutPkts().Get(t)
	t.Logf("otg:Flow out counters %v %v", flowName, outPkts)
	fptest.LogYgot(t, "otg:Flow counters", afc, afc.Get(t))

	inPkts := afc.InPkts().Get(t)
	t.Logf("otg:Flow in counters %v %v", flowName, inPkts)

	lostPkts := outPkts - inPkts
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

// verifyTraffic confirms that every traffic flow has the expected amount of loss (0% or 100%)
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, flows []gosnappi.Flow, passFlows []string) {
	topo := ate.OTG().FetchConfig(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), topo)
	for _, flow := range flows {
		t.Logf("*** Verifying %v traffic on OTG ... ", flow.Name())
		captureTrafficStats(t, ate, flow.Name(), false)
		outPkts := ate.OTG().Telemetry().Flow(flow.Name()).Counters().OutPkts().Get(t)
		inPkts := ate.OTG().Telemetry().Flow(flow.Name()).Counters().InPkts().Get(t)
		lossPct := (outPkts - inPkts) / outPkts * 100
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

	// Configure DUT interfaces and forwarding policy
	configureDUT(t, dut, p1, p2)

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	allFlows := configureATE(t, ate)

	tcs := []struct {
		desc      string
		policy    string
		passFlows []string
	}{
		{
			desc:      "Match IPv4",
			policy:    "match-ipv4",
			passFlows: []string{"Ipv4Vlan10", "IpipVlan10", "IpipDscp46Vlan10", "IpipDscp42Vlan10"},
		},
		{
			desc:      "Match IPinIP",
			policy:    "match-ipip",
			passFlows: []string{"IpipVlan10", "IpipDscp46Vlan10", "IpipDscp42Vlan10"},
		},
		{
			desc:      "Match IPv4 and IPv6",
			policy:    "match-ip4ip6",
			passFlows: []string{"Ipv4Vlan10", "IpipVlan10", "IpipDscp46Vlan10", "IpipDscp42Vlan10", "Ipv6Vlan20"},
		},
		{
			desc:      "Match IPinIP DSCP 46",
			policy:    "match-ipip-dscp46",
			passFlows: []string{"IpipDscp46Vlan10"},
		},
		{
			desc:      "Match IPinIP DSCP 42 or 46",
			policy:    "match-ipip-dscp42or46",
			passFlows: []string{"IpipDscp46Vlan10", "IpipDscp46Vlan10"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			// applyForwardingPolicy(t, ate, p1.Name(), tc.policy)
			sendTraffic(t, ate)
			verifyTraffic(t, ate, allFlows, tc.passFlows)
		})
	}
}
