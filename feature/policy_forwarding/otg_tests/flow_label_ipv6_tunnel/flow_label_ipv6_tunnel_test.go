// Copyright 2025 Google LLC
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

// Package flow_label_ipv6_tunnel_test implements the flow label ipv6 tunnel test.
package flow_label_ipv6_tunnel_test

import (
	"fmt"
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

const (
	ipv6PrefixLen  = 126
	pbrPolicyName  = "flow-label-policy"
	vrfName        = "VRF1"
	flowPPS        = 500
	flowPacketSize = 512
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "dutPort1",
		IPv6:    "2001:DB2::1",
		IPv6Len: ipv6PrefixLen,
	}
	atePort1 = attrs.Attributes{
		Desc:    "atePort1",
		Name:    "atePort1",
		IPv6:    "2001:DB2::2",
		IPv6Len: ipv6PrefixLen,
		MAC:     "02:00:01:01:01:01",
	}
	dutDst = attrs.Attributes{
		Desc:    "dutPort2",
		IPv6:    "2001:DB2::5",
		IPv6Len: ipv6PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Desc:    "atePort2",
		Name:    "atePort2",
		IPv6:    "2001:DB2::6",
		IPv6Len: ipv6PrefixLen,
		MAC:     "02:00:02:01:01:01",
	}
	tolerancePct = uint64(2)
)

type pbrPolicy struct {
	ruleSequenceID uint32
	flowLabel      uint32
	greDestination string
	destinationIP  string
	destinationIPs string
}

var (
	pbrPolicies = []pbrPolicy{
		{ruleSequenceID: 10, greDestination: "3008:DB8::/126", destinationIP: "4008:DBA::2", destinationIPs: "4008:DBA::2/126", flowLabel: 49512},
		{ruleSequenceID: 20, greDestination: "3009:DB9::/126", destinationIP: "5008:DBA::2", destinationIPs: "5008:DBA::2/126", flowLabel: 50512},
	}
	staticRoutes = []string{"3008:DB8::/126", "3009:DB9::/126"}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {

	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(0)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)

	return i
}

// configureInterfaceVRF configures the VRF on the interface.
func configureInterfaceVRF(t *testing.T, dut *ondatra.DUTDevice, portName string) {
	t.Helper()
	v := &oc.NetworkInstance{
		Name: ygot.String(vrfName),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
	}
	vi := v.GetOrCreateInterface(portName)
	vi.Interface = ygot.String(portName)
	vi.Subinterface = ygot.Uint32(0)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), v)
}

// configureDUT configures port1, port2 on the DUT and enables the interfaces.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	t.Logf("Configuring VRF on interface %s", p1.Name())
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	i1.Enabled = ygot.Bool(true)
	gnmi.Update(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutSrc, dut))
	configureInterfaceVRF(t, dut, p1.Name())

	p2 := dut.Port(t, "port2")
	t.Logf("Configuring VRF on interface %s", p2.Name())
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	i2.Enabled = ygot.Bool(true)
	gnmi.Update(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst, dut))
	configureInterfaceVRF(t, dut, p2.Name())

}

// configureStaticRoutes configures the static routes on the DUT.
func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	ni := oc.NetworkInstance{Name: ygot.String(vrfName)}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))

	for _, route := range staticRoutes {
		sr := static.GetOrCreateStatic(route)
		nh := sr.GetOrCreateNextHop("0")
		nh.NextHop = oc.UnionString(atePort2.IPv6)
	}

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// configureForwardingPolicy configures the PBR policy on the DUT.
func configureForwardingPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	policyFwding := ni.GetOrCreatePolicyForwarding()
	policy := policyFwding.GetOrCreatePolicy(pbrPolicyName)
	policy.SetType(oc.Policy_Type_PBR_POLICY)

	for idx, p := range pbrPolicies {
		rule := policy.GetOrCreateRule(p.ruleSequenceID)
		ruleIPv6 := rule.GetOrCreateIpv6()
		ruleIPv6.SetDestinationAddress(p.destinationIPs)
		ruleIPv6.SetSourceFlowLabel(p.flowLabel)
		action := rule.GetOrCreateAction()
		encap := action.GetOrCreateEncapsulateGre()
		target := encap.GetOrCreateTarget(fmt.Sprintf("%d", idx))
		target.SetDestination(p.greDestination)
	}

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(pbrPolicyName).Config(), policy)

	p1 := dut.Port(t, "port1")
	intf1 := policyFwding.GetOrCreateInterface(p1.Name())
	intf1.SetApplyForwardingPolicy(pbrPolicyName)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(p1.Name()).Config(), intf1)

	p2 := dut.Port(t, "port2")
	intf2 := policyFwding.GetOrCreateInterface(p2.Name())
	intf2.SetApplyForwardingPolicy(pbrPolicyName)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(p2.Name()).Config(), intf2)
}

// configureOTG configures the OTG with the atePort1 and atePort2.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()

	top := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	atePort1.AddToOTG(top, p1, &dutSrc)
	atePort2.AddToOTG(top, p2, &dutDst)
	return top
}

// createTrafficFlows creates the traffic flows for each PBR policy.
func createTrafficFlows(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice) {
	t.Helper()

	for _, p := range pbrPolicies {
		flowName := fmt.Sprintf("%d-%d-Flow:", p.ruleSequenceID, p.flowLabel)
		flow := top.Flows().Add().SetName(flowName)
		flow.TxRx().Port().
			SetTxName(ate.Port(t, "port1").ID()).
			SetRxNames([]string{ate.Port(t, "port2").ID()})

		flow.Metrics().SetEnable(true)
		flow.Rate().SetPps(flowPPS)
		flow.Size().SetFixed(flowPacketSize)
		flow.Duration().Continuous()

		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(atePort1.MAC)
		dutDstInterface := dut.Port(t, "port1").Name()
		dstMac := gnmi.Get(t, dut, gnmi.OC().Interface(dutDstInterface).Ethernet().MacAddress().State())
		eth.Dst().SetValue(dstMac)

		ip := flow.Packet().Add().Ipv6()
		flowLabel := gosnappi.NewPatternFlowIpv6FlowLabel()
		flowLabel.SetValue(p.flowLabel)
		ip.SetFlowLabel(flowLabel)
		ip.Src().SetValue(atePort1.IPv6)
		ip.Dst().SetValue(p.destinationIP)
	}
}

// verifyTraffic verifies the traffic metrics for each flow.
func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, c gosnappi.Config) {
	t.Helper()
	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, c)

	for _, p := range pbrPolicies {
		flowName := fmt.Sprintf("%d-%d-Flow:", p.ruleSequenceID, p.flowLabel)
		t.Logf("Verifying flow metrics for flow %s\n", flowName)
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).State())
		txPackets := recvMetric.GetCounters().GetOutPkts()
		rxPackets := recvMetric.GetCounters().GetInPkts()
		lostPackets := txPackets - rxPackets
		lossPct := lostPackets * 100 / txPackets
		if lossPct > tolerancePct {
			t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want 0", flowName, lossPct)
		} else {
			t.Logf("Traffic Test Passed!")
		}
	}
}

// TestFlowLabelIPv6Tunnel tests the PBR policy based on the IPv6 flow label.
func TestFlowLabelIPv6Tunnel(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Log("Configure the DUT")
	configureDUT(t, dut)

	t.Log("Configure static routes on the DUT")
	configureStaticRoutes(t, dut)

	t.Log("Configure forwarding policy on the DUT")
	configureForwardingPolicy(t, dut)

	t.Log("Configure OTG")
	top := configureOTG(t, ate)
	createTrafficFlows(t, top, ate, dut)

	t.Log("Push config to the OTG device")
	t.Log(top.String())
	otgObj := ate.OTG()
	otgObj.PushConfig(t, top)

	t.Log("Start protocols and traffic")
	otgObj.StartProtocols(t)
	otgObj.StartTraffic(t)
	t.Log("Wait for traffic to start")
	time.Sleep(15 * time.Second)
	t.Log("Stop traffic")
	otgObj.StopTraffic(t)
	t.Log("Stop protocols")
	otgObj.StopProtocols(t)

	t.Log("Verifying GRE traffic is received by ATE")
	verifyTraffic(t, ate, top)
}
