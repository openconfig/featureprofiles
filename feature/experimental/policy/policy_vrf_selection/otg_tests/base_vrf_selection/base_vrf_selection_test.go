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
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	trafficDuration           = 1 * time.Minute
	sleepOnChange             = 10 * time.Second
	plen4                     = 30
	plen6                     = 126
	vlan10                    = 10
	vlan20                    = 20
	ipipProtocol              = 4
	ipv6ipProtocol            = 41
	srcIpv4Address            = "198.18.0.1"
	prefixedSrcIpv4Address    = "198.18.0.1/32"
	nonMatchingIpv4Address    = "198.51.100.1"
	ateDestIPv4VLAN10         = "203.0.113.0"
	prefixedAteDestIPv4VLAN10 = "203.0.113.0/30"
	ateDestIPv4VLAN20         = "203.0.113.4"
	prefixedAteDestIPv4VLAN20 = "203.0.113.4/30"
	ateDestIPv6               = "2001:DB8:2::/64"
	defaultNHv4               = "192.0.2.10"
	defaultNHv6               = "2001:db8::a"
	vrfNH                     = "192.0.2.6"
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
	d := gnmi.OC()

	// Configure ingress interface
	t.Logf("*** Configuring interfaces on DUT ...")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutSrc, 0, 0, dut))

	// Configure egress interface
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst, 10, vlan10, dut))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst2, 20, vlan20, dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}

	// Configure network instance
	t.Logf("*** Configuring network instance on DUT ... ")
	niConfPath := gnmi.OC().NetworkInstance("VRF-10")
	niConf := configNetworkInstance("VRF-10", i2, 10)
	gnmi.Replace(t, dut, niConfPath.Config(), niConf)
	if deviations.InterfaceConfigVRFBeforeAddress(dut) {
		gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), i2)
	}

	// Configure default NI and forwarding policy
	t.Logf("*** Configuring default instance forwarding policy on DUT ...")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, i2.GetName(), deviations.DefaultNetworkInstance(dut), 20)
	}

	policyDutConf := configForwardingPolicy(dut)
	gnmi.Replace(t, dut, dutConfPath.PolicyForwarding().Config(), policyDutConf)

}

func configInterfaceDUT(i *oc.Interface, me *attrs.Attributes, subIntfIndex uint32, vlan uint16, dut *ondatra.DUTDevice) *oc.Interface {
	i.Description = ygot.String(me.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	if deviations.RequireRoutedSubinterface0(dut) {
		s0 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		s0.Enabled = ygot.Bool(true)
	}

	// Create subinterface.
	s := i.GetOrCreateSubinterface(subIntfIndex)

	if vlan != 0 {
		// Add VLANs.
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlan)
		} else {
			singletag := s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged()
			singletag.VlanId = ygot.Uint16(vlan)
		}
	}
	// Add IPv4 stack.
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a := s4.GetOrCreateAddress(me.IPv4)
	a.PrefixLength = ygot.Uint8(plen4)

	// Add IPv6 stack.
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(me.IPv6).PrefixLength = ygot.Uint8(plen6)

	return i
}

// Configure Network instance on the DUT
func configNetworkInstance(name string, intf *oc.Interface, id uint32) *oc.NetworkInstance {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(name)

	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	niIntf := ni.GetOrCreateInterface(*intf.Name + "." + strconv.Itoa(int(id)))
	niIntf.Subinterface = ygot.Uint32(id)
	niIntf.Interface = ygot.String(*intf.Name)

	return ni
}

// configDefaultRoute configures a static route in DEFAULT network-instance.
func configDefaultRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, v4NextHop, v6Prefix, v6NextHop string) {
	t.Logf("*** Configuring static route in DEFAULT network-instance ...")
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(v4Prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v4NextHop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
	sr = static.GetOrCreateStatic(v6Prefix)
	nh = sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v6NextHop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// configVRFRoute configures a static route in VRF.
func configVRFRoute(t *testing.T, dut *ondatra.DUTDevice, v4Prefix, v4NextHop string) {
	t.Logf("*** Configuring static route in VRF-10 network-instance ...")
	ni := oc.NetworkInstance{Name: ygot.String("VRF-10")}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(v4Prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v4NextHop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance("VRF-10").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

func configForwardingPolicy(dut *ondatra.DUTDevice) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	// Match policy.
	policyFwding := ni.GetOrCreatePolicyForwarding()

	fwdPolicy1 := policyFwding.GetOrCreatePolicy("match-ipip")
	fwdPolicy1.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	fwdPolicy1.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = oc.UnionUint8(ipipProtocol)
	fwdPolicy1.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("VRF-10")

	fwdPolicy2 := policyFwding.GetOrCreatePolicy("match-ipip-src")
	fwdPolicy2.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = oc.UnionUint8(ipipProtocol)
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateIpv4().SourceAddress = ygot.String(prefixedSrcIpv4Address)
	fwdPolicy2.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("VRF-10")

	fwdPolicy3 := policyFwding.GetOrCreatePolicy("match-ipv6inipv4")
	fwdPolicy3.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	fwdPolicy3.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = oc.UnionUint8(ipv6ipProtocol)
	fwdPolicy3.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("VRF-10")

	fwdPolicy4 := policyFwding.GetOrCreatePolicy("match-ipv6inipv4-src")
	fwdPolicy4.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	fwdPolicy4.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = oc.UnionUint8(ipv6ipProtocol)
	fwdPolicy4.GetOrCreateRule(1).GetOrCreateIpv4().SourceAddress = ygot.String(prefixedSrcIpv4Address)
	fwdPolicy4.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String("VRF-10")

	return policyFwding
}

// applyForwardingPolicy applies the forwarding policy on the interface.
func applyForwardingPolicy(t *testing.T, ate *ondatra.ATEDevice, ingressPort, matchType string) {
	t.Logf("*** Applying forwarding policy %v on interface %v ... ", matchType, ingressPort)

	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	pfpath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(ingressPort)
	gnmi.Delete(t, dut, pfpath.Config())

	intf := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(ingressPort)
	intf.ApplyVrfSelectionPolicy = ygot.String(matchType)
	intf.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}

	// Configure default NI and forwarding policy.
	intfConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(ingressPort)
	gnmi.Replace(t, dut, intfConfPath.Config(), intf)

	// Restart Protocols after policy change
	ate.OTG().StopProtocols(t)
	time.Sleep(sleepOnChange)
	ate.OTG().StartProtocols(t)
	time.Sleep(sleepOnChange)
}

type trafficFlows struct {
	ipInIPFlow1, ipInIPFlow2, ipInIPFlow3, ipInIPFlow4         gosnappi.Flow
	ipv6InIPFlow5, ipv6InIPFlow6, ipv6InIPFlow7, ipv6InIPFlow8 gosnappi.Flow
	nativeIPv4, nativeIPv6                                     gosnappi.Flow
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) *trafficFlows {
	t.Helper()
	t.Logf("*** Configuring OTG interfaces ...")
	topo := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	topo.Ports().Add().SetName(p1.ID())
	srcDev := topo.Devices().Add().SetName(ateSrc.Name)
	ethSrc := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth")
	ethSrc.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(p1.ID())
	ethSrc.SetMac(ateSrc.MAC)
	ethSrc.Ipv4Addresses().Add().SetName(srcDev.Name() + ".IPv4").SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	ethSrc.Ipv6Addresses().Add().SetName(srcDev.Name() + ".IPv6").SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	topo.Ports().Add().SetName(p2.ID())
	dstDev := topo.Devices().Add().SetName(ateDst.Name)
	ethDst := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth")

	ethDst.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(p2.ID())
	ethDst.SetMac(ateDst.MAC)
	ethDst.Vlans().Add().SetName(dstDev.Name() + "-VLAN").SetId(uint32(vlan10))
	ethDst.Ipv4Addresses().Add().SetName(dstDev.Name() + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	ethDst.Ipv6Addresses().Add().SetName(dstDev.Name() + ".IPv6").SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	dstDev2 := topo.Devices().Add().SetName(ateDst2.Name)
	ethDst2 := dstDev2.Ethernets().Add().SetName(ateDst2.Name + ".Eth")

	ethDst2.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(p2.ID())
	ethDst2.SetMac(ateDst2.MAC)
	ethDst2.Vlans().Add().SetName(dstDev2.Name() + "-VLAN").SetId(uint32(vlan20))
	ethDst2.Ipv4Addresses().Add().SetName(dstDev2.Name() + ".IPv4").SetAddress(ateDst2.IPv4).SetGateway(dutDst2.IPv4).SetPrefix(uint32(ateDst2.IPv4Len))
	ethDst2.Ipv6Addresses().Add().SetName(dstDev2.Name() + ".IPv6").SetAddress(ateDst2.IPv6).SetGateway(dutDst2.IPv6).SetPrefix(uint32(ateDst2.IPv6Len))

	// Create traffic flows
	t.Logf("*** Configuring OTG flows ...")
	topo.Flows().Clear().Items()
	ipInIPFlow1 := createIPv4Flow("ipInIPFlow1", topo, ateDst, nonMatchingIpv4Address, ateDestIPv4VLAN10, "IPv4")
	ipInIPFlow2 := createIPv4Flow("ipInIPFlow2", topo, ateDst2, nonMatchingIpv4Address, ateDestIPv4VLAN20, "IPv4")
	ipInIPFlow3 := createIPv4Flow("ipInIPFlow3", topo, ateDst, srcIpv4Address, ateDestIPv4VLAN10, "IPv4")
	ipInIPFlow4 := createIPv4Flow("ipInIPFlow4", topo, ateDst2, srcIpv4Address, ateDestIPv4VLAN20, "IPv4")
	ipv6InIPFlow5 := createIPv4Flow("ipv6InIPFlow5", topo, ateDst, nonMatchingIpv4Address, ateDestIPv4VLAN10, "IPv6")
	ipv6InIPFlow6 := createIPv4Flow("ipv6InIPFlow6", topo, ateDst2, nonMatchingIpv4Address, ateDestIPv4VLAN20, "IPv6")
	ipv6InIPFlow7 := createIPv4Flow("ipv6InIPFlow7", topo, ateDst, srcIpv4Address, ateDestIPv4VLAN10, "IPv6")
	ipv6InIPFlow8 := createIPv4Flow("ipv6InIPFlow8", topo, ateDst2, srcIpv4Address, ateDestIPv4VLAN20, "IPv6")
	nativeIPv4 := createIPv4Flow("nativeIPv4", topo, ateDst2, ateSrc.IPv4, ateDestIPv4VLAN20, "")
	nativeIPv6 := topo.Flows().Add().SetName("nativeIPv6")
	nativeIPv6.Metrics().SetEnable(true)
	nativeIPv6.TxRx().Device().SetTxNames([]string{ateSrc.Name + ".IPv6"}).SetRxNames([]string{ateDst.Name + ".IPv6"})
	nativeIPv6.Packet().Add().Ethernet().Src().SetValue(ateSrc.MAC)
	v6 := nativeIPv6.Packet().Add().Ipv6()
	v6.Src().SetValue(ateSrc.IPv6)
	v6.Dst().SetValue("2001:DB8:2::")
	nativeIPv6.Size().SetFixed(512)
	nativeIPv6.Rate().SetChoice("percentage").SetPercentage(5)

	t.Logf("Pushing config to ATE and starting protocols...")
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	return &trafficFlows{ipInIPFlow1, ipInIPFlow2, ipInIPFlow3, ipInIPFlow4, ipv6InIPFlow5, ipv6InIPFlow6, ipv6InIPFlow7, ipv6InIPFlow8, nativeIPv4, nativeIPv6}
}

func createIPv4Flow(name string, top gosnappi.Config, dst attrs.Attributes, srcIP, dstIP, innerIpType string) gosnappi.Flow {
	flow := top.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{ateSrc.Name + ".IPv4"}).SetRxNames([]string{dst.Name + ".IPv4"})
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(ateSrc.MAC)
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(srcIP)
	v4.Dst().SetValue(dstIP)
	if innerIpType == "IPv4" {
		flow.Packet().Add().Ipv4()
	}
	if innerIpType == "IPv6" {
		flow.Packet().Add().Ipv6()
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

	flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
	inPkts := flowMetrics.GetCounters().GetInPkts()
	outPkts := flowMetrics.GetCounters().GetOutPkts()

	t.Logf("otg:Flow out counters %v %v", flowName, outPkts)
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

func contains(item gosnappi.Flow, items []gosnappi.Flow) bool {
	flag := false
	for _, j := range items {
		if item == j {
			return true
		}
	}
	return flag
}

// verifyTraffic confirms that every traffic flow has the expected amount of loss (0% or 100%)
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, flows, passFlows []gosnappi.Flow) {
	t.Helper()
	topo := ate.OTG().FetchConfig(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), topo)
	for _, flow := range flows {
		t.Logf("*** Verifying %v traffic on OTG ... ", flow.Name())
		captureTrafficStats(t, ate, flow.Name(), false)
		flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).State())
		inPkts := float32(flowMetrics.GetCounters().GetInPkts())
		outPkts := float32(flowMetrics.GetCounters().GetOutPkts())
		if outPkts == 0 {
			t.Errorf("tx packets is 0")
			return
		}
		lossPct := (outPkts - inPkts) / outPkts * 100
		if contains(flow, passFlows) {
			if lossPct > 0 {
				t.Errorf("Traffic Loss Pct for Flow: %s got %f, want 0", flow.Name(), lossPct)
			} else {
				t.Logf("Traffic Test Passed!")
			}
		} else {
			if lossPct < 100 {
				t.Errorf("Traffic is expected to fail %s got %f, want 100%% failure", flow.Name(), lossPct)
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
	configDefaultRoute(t, dut, prefixedAteDestIPv4VLAN20, defaultNHv4, ateDestIPv6, defaultNHv6)
	configVRFRoute(t, dut, prefixedAteDestIPv4VLAN10, vrfNH)

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	allFlows := configureATE(t, ate)

	tcs := []struct {
		desc      string
		policy    string
		passFlows []gosnappi.Flow
		flows     []gosnappi.Flow
	}{
		{
			desc:   "Match IP in IP.",
			policy: "match-ipip",
			flows: []gosnappi.Flow{allFlows.ipInIPFlow1, allFlows.ipInIPFlow3, allFlows.ipv6InIPFlow6,
				allFlows.ipv6InIPFlow8, allFlows.nativeIPv4, allFlows.nativeIPv6},
			passFlows: []gosnappi.Flow{allFlows.ipInIPFlow1, allFlows.ipInIPFlow3, allFlows.ipv6InIPFlow6,
				allFlows.ipv6InIPFlow8, allFlows.nativeIPv4, allFlows.nativeIPv6},
		},
		{
			desc:   "Match IPinIP with Source IP.",
			policy: "match-ipip-src",
			flows: []gosnappi.Flow{allFlows.ipInIPFlow2, allFlows.ipInIPFlow3, allFlows.ipv6InIPFlow6,
				allFlows.ipv6InIPFlow8, allFlows.nativeIPv4, allFlows.nativeIPv6},
			passFlows: []gosnappi.Flow{allFlows.ipInIPFlow2, allFlows.ipInIPFlow3, allFlows.ipv6InIPFlow6,
				allFlows.ipv6InIPFlow8, allFlows.nativeIPv4, allFlows.nativeIPv6},
		},
		{
			desc:   "Match IPv6 in IP.",
			policy: "match-ipv6inipv4",
			flows: []gosnappi.Flow{allFlows.ipInIPFlow2, allFlows.ipInIPFlow4, allFlows.ipv6InIPFlow5,
				allFlows.ipv6InIPFlow7, allFlows.nativeIPv4, allFlows.nativeIPv6},
			passFlows: []gosnappi.Flow{allFlows.ipInIPFlow2, allFlows.ipInIPFlow4, allFlows.ipv6InIPFlow5,
				allFlows.ipv6InIPFlow7, allFlows.nativeIPv4, allFlows.nativeIPv6},
		},
		{
			desc:   "Match IPv6 in IP with Source IP.",
			policy: "match-ipv6inipv4-src",
			flows: []gosnappi.Flow{allFlows.ipInIPFlow2, allFlows.ipInIPFlow4, allFlows.ipv6InIPFlow6,
				allFlows.ipv6InIPFlow7, allFlows.nativeIPv4, allFlows.nativeIPv6},
			passFlows: []gosnappi.Flow{allFlows.ipInIPFlow2, allFlows.ipInIPFlow4, allFlows.ipv6InIPFlow6,
				allFlows.ipv6InIPFlow7, allFlows.nativeIPv4, allFlows.nativeIPv6},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			applyForwardingPolicy(t, ate, p1.Name(), tc.policy)
			sendTraffic(t, ate)
			verifyTraffic(t, ate, tc.flows, tc.passFlows)
		})
	}
}
