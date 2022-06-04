/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package policy_based_vrf_selection_rt_3dot1_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
	instance      = "default"
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut *ondatra.DUTDevice
	ate *ondatra.ATEDevice
	top *ondatra.ATETopology
}

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2Vlan10 = attrs.Attributes{
		Desc:    "dutPort2Vlan10",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:9",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2Vlan10 = attrs.Attributes{
		Name:    "atePort2Vlan10",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:a",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2Vlan20 = attrs.Attributes{
		Desc:    "dutPort2Vlan20",
		IPv4:    "192.0.2.13",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:d",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2Vlan20 = attrs.Attributes{
		Name:    "atePort2Vlan20",
		IPv4:    "192.0.2.14",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:e",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2Vlan30 = attrs.Attributes{
		Desc:    "dutPort2Vlan30",
		IPv4:    "192.0.2.17",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:11",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2Vlan30 = attrs.Attributes{
		Name:    "atePort2Vlan20",
		IPv4:    "192.0.2.18",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:12",
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureATE configures port1, port2 and vlans on port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)
	i1.IPv6().
		WithAddress(atePort1.IPv6CIDR()).
		WithDefaultGateway(dutPort1.IPv6)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)
	i2.IPv6().
		WithAddress(atePort2.IPv6CIDR()).
		WithDefaultGateway(dutPort2.IPv6)

	//Configure vlans on ATE port2
	i2v10 := top.AddInterface("atePort2Vlan10").WithPort(p2)
	i2v10.Ethernet().WithVLANID(10)
	i2v10.IPv4().
		WithAddress(atePort2Vlan10.IPv4CIDR()).
		WithDefaultGateway(dutPort2Vlan10.IPv4)
	i2v10.IPv6().
		WithAddress(atePort2Vlan10.IPv6CIDR()).
		WithDefaultGateway(dutPort2Vlan10.IPv6)

	i2v20 := top.AddInterface("atePort2Vlan20").WithPort(p2)
	i2v20.Ethernet().WithVLANID(20)
	i2v20.IPv4().
		WithAddress(atePort2Vlan20.IPv4CIDR()).
		WithDefaultGateway(dutPort2Vlan20.IPv4)
	i2v20.IPv6().
		WithAddress(atePort2Vlan20.IPv6CIDR()).
		WithDefaultGateway(dutPort2Vlan20.IPv6)

	i2v30 := top.AddInterface("atePort2Vlan30").WithPort(p2)
	i2v30.Ethernet().WithVLANID(30)
	i2v30.IPv4().
		WithAddress(atePort2Vlan30.IPv4CIDR()).
		WithDefaultGateway(dutPort2Vlan30.IPv4)
	i2v30.IPv6().
		WithAddress(atePort2Vlan30.IPv6CIDR()).
		WithDefaultGateway(dutPort2Vlan30.IPv6)

	return top
}

//configNetworkInstance creates VRFs and subinterfaces and then applies VRFs on the subinterfaces.
func configNetworkInstance(t *testing.T, dut *ondatra.DUTDevice, vrfname, intfname string, subint uint32) {
	//create empty subinterface
	si := &oc.Interface_Subinterface{}
	si.Index = ygot.Uint32(subint)
	dut.Config().Interface(intfname).Subinterface(subint).Replace(t, si)

	//create vrf and apply on subinterface
	v := &oc.NetworkInstance{
		Name: ygot.String(vrfname),
	}
	vi := v.GetOrCreateInterface(intfname + "." + strconv.Itoa(int(subint)))
	vi.Subinterface = ygot.Uint32(subint)
	dut.Config().NetworkInstance(vrfname).Replace(t, v)
}

//getSubInterface returns a subinterface configuration populated with IP addresses and VLAN ID.
func getSubInterface(dutPort *attrs.Attributes, index uint32, vlanID uint16) *oc.Interface_Subinterface {
	s := &oc.Interface_Subinterface{}
	//unshut sub/interface
	if *deviations.InterfaceEnabled {
		s.Enabled = ygot.Bool(true)
	}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(dutPort.IPv4)
	a.PrefixLength = ygot.Uint8(dutPort.IPv4Len)
	s6 := s.GetOrCreateIpv6()
	a6 := s6.GetOrCreateAddress(dutPort.IPv6)
	a6.PrefixLength = ygot.Uint8(dutPort.IPv6Len)
	v := s.GetOrCreateVlan()
	m := v.GetOrCreateMatch()
	if index != 0 {
		m.GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
	}
	return s
}

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *oc.Interface, dutPort *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(dutPort.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	i.AppendSubinterface(getSubInterface(dutPort, 0, 0))
	return i
}

//configureDUT configures the base configuration on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	d.Interface(p1.Name()).Replace(t, configInterfaceDUT(i1, &dutPort1))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	d.Interface(p2.Name()).Replace(t, configInterfaceDUT(i2, &dutPort2))

	outpath := d.Interface(p2.Name())
	//Create VRFs and VRF enabled subinterfaces
	configNetworkInstance(t, dut, "VRF10", p2.Name(), uint32(1))

	//Configure IP addresses on subinterfaces
	outpath.Subinterface(1).Update(t, getSubInterface(&dutPort2Vlan10, 1, 10))

	configNetworkInstance(t, dut, "VRF20", p2.Name(), uint32(2))
	outpath.Subinterface(2).Update(t, getSubInterface(&dutPort2Vlan20, 2, 20))

	configNetworkInstance(t, dut, "VRF30", p2.Name(), uint32(3))
	outpath.Subinterface(3).Update(t, getSubInterface(&dutPort2Vlan30, 3, 30))
}

func TestPBR(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	//configure DUT
	configureDUT(t, dut)

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	t.Run("Protocol DSCP Based VRF Selection", func(t *testing.T) {
		t.Logf("Description: Test RT3.1 with DSCP, IPv4, IPv6, IPinIP based VRF selection")
		args := &testArgs{
			dut: dut,
			ate: ate,
			top: top,
		}
		testDscpProtocolBasedVRFSelection(t, args)
	})
}

//getFlow returns *ondatra.Flow based on the endpoint, flowName and header inputs.
func getFlow(t *testing.T, ate *ondatra.ATEDevice, srcEndpoint, dstEndPoint ondatra.Endpoint, flowName string, header ...ondatra.Header) *ondatra.Flow {

	flow := ate.Traffic().NewFlow(flowName)
	t.Logf("Setting up flow -> %s", flowName)
	flow.WithHeaders(header...)
	flow.WithSrcEndpoints(srcEndpoint)
	flow.WithDstEndpoints(dstEndPoint)
	flow.WithFrameRateFPS(100)
	flow.WithFrameSize(1024)
	return flow
}

//getIPv4Flow returns an IPv4 *ondatra.Flow with provided DSCP and TTL values.
func getIPv4Flow(t *testing.T, ate *ondatra.ATEDevice, srcEndpoint, dstEndPoint ondatra.Endpoint, flowName string, dscp uint8) *ondatra.Flow {

	ethHeader := ondatra.NewEthernetHeader()
	ipHeader := ondatra.NewIPv4Header().WithDSCP(dscp)

	return getFlow(t, ate, srcEndpoint, dstEndPoint, flowName, ethHeader, ipHeader)
}

//getIPv6Flow returns an IPv6 *ondatra.Flow with provided DSCP value for a given set of endpoints.
func getIPv6Flow(t *testing.T, ate *ondatra.ATEDevice, srcEndpoint, dstEndPoint ondatra.Endpoint, flowName string, dscp uint8) *ondatra.Flow {

	ethHeader := ondatra.NewEthernetHeader()
	ipHeader := ondatra.NewIPv6Header().WithDSCP(dscp)

	return getFlow(t, ate, srcEndpoint, dstEndPoint, flowName, ethHeader, ipHeader)
}

//getIPinIPFlow returns an IPv4inIPv4 *ondatra.Flow with provided DSCP and TTL values for a given set of endpoints.
func getIPinIPFlow(t *testing.T, ate *ondatra.ATEDevice, srcEndpoint, dstEndPoint ondatra.Endpoint, flowName string, dscp uint8) *ondatra.Flow {

	ethHeader := ondatra.NewEthernetHeader()
	outerIPHeader := ondatra.NewIPv4Header().WithDSCP(dscp)
	innerIPHeader := ondatra.NewIPv4Header()
	innerIPHeader.WithSrcAddress("198.51.100.1")
	innerIPHeader.DstAddressRange().WithMin("203.0.113.1").WithCount(10000).WithStep("0.0.0.1")

	return getFlow(t, ate, srcEndpoint, dstEndPoint, flowName, ethHeader, outerIPHeader, innerIPHeader)
}

//testTrafficFlows verifies traffic for one or more flows with accuracy within tolerance.
func testTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, topology *ondatra.ATETopology, expectPass bool, flow ...*ondatra.Flow) {

	ate.Traffic().Start(t, flow...)
	time.Sleep(60 * time.Second)
	ate.Traffic().Stop(t)
	if expectPass {
		t.Log("Expecting traffic to pass for the flows")
	} else {
		t.Log("Expecting traffic to drop for the flows")
	}

	//log stats
	t.Log("All flow LossPct: ", ate.Telemetry().FlowAny().LossPct().Get(t))
	t.Log("FlowAny InPkts  : ", ate.Telemetry().FlowAny().Counters().InPkts().Get(t))
	t.Log("FlowAny OutPkts : ", ate.Telemetry().FlowAny().Counters().OutPkts().Get(t))

	flowPath := ate.Telemetry().FlowAny()
	if got := flowPath.LossPct().Get(t); len(got) == 0 {
		t.Fatalf("Flow stats count not correct, wanted > 0, got 0")
	} else {
		for i, lossPct := range got {
			if (expectPass == true) && (lossPct == 0) {
				t.Logf("Traffic for %v flow is passing as expected", flow[i].Name())
			} else if (expectPass == false) && (lossPct == 100) {
				t.Logf("Traffic is not passing for %v flow as expected", flow[i].Name())

			} else {
				t.Fatalf("Traffic is not working as expected for flow: %v.", flow[i].Name())
			}

		}
	}
}

//getL3PBRRule returns an IPv4 or IPv6 policy-forwarding rule configuration populated with protocol and/or DSCPset information.
func getL3PBRRule(networkInstance, iptype string, index uint32, protocol oc.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8) *oc.NetworkInstance_PolicyForwarding_Policy_Rule {

	r := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	if iptype == "ipv4" {
		r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv4.DscpSet = dscpset
		}
	} else if iptype == "ipv6" {
		r.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv6.DscpSet = dscpset
		}
	} else {
		return nil
	}
	return &r
}

//configurePBR configures poliy-forwarding with a PBR policy having single rule.
func configurePBR(t *testing.T, dut *ondatra.DUTDevice, policyName, networkInstance, iptype string, index uint32, protocol oc.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8) {

	r1 := getL3PBRRule(networkInstance, iptype, index, protocol, dscpset)
	pf := oc.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(policyName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(r1)
	dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &pf)
}

func getL2PBRRule(networkInstance, iptype string, index uint32) *oc.NetworkInstance_PolicyForwarding_Policy_Rule {

	r := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	if iptype == "ipv4" {
		r.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4,
		}
	} else if iptype == "ipv6" {
		r.L2 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_L2{
			Ethertype: oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6,
		}
	} else {
		return nil
	}

	return &r
}

//configureL2PBR configures policy-forwarding with an L2 PBR policy.
func configureL2PBR(t *testing.T, dut *ondatra.DUTDevice, policyName, networkInstance, iptype string, index uint32) {
	r1 := getL2PBRRule(networkInstance, iptype, index)
	pf := oc.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(policyName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(r1)
	dut.Config().NetworkInstance("default").PolicyForwarding().Update(t, &pf)
}

//configureL2PBRRule applies an L2 rule to an existing PBR policy.
func configureL2PBRRule(t *testing.T, dut *ondatra.DUTDevice, policyName, networkInstance, iptype string, index uint32) {
	if r := getL2PBRRule(networkInstance, iptype, index); r == nil {
		t.Fatalf("Invalid pbr rule")
	} else {
		dut.Config().NetworkInstance("default").PolicyForwarding().Policy(policyName).Rule(index).Replace(t, r)
	}
}

//testDscpProtocolBasedVRFSelection ensures that protocol and DSCP based VRF selection is configured correctly.
func testDscpProtocolBasedVRFSelection(t *testing.T, args *testArgs) {
	t.Log("RT-3.1 :Protocol, DSCP-based VRF Selection - ensure that protocol and DSCP based VRF selection is configured correctly")
	pfpath := args.dut.Config().NetworkInstance("default").PolicyForwarding()
	//defer cleaning policy-forwarding
	defer pfpath.Delete(t)

	port1 := args.dut.Port(t, "port1")
	srcEndPoint := args.top.Interfaces()["atePort1"]
	dstEndPointVlan10 := args.top.Interfaces()["atePort2Vlan10"]
	dstEndPointVlan20 := args.top.Interfaces()["atePort2Vlan20"]

	//Case1 - Matching ipv4 protocol to VRF10. Dropping IPv6 traffic in VRF10.
	//Create IPV4 and IPv6 flows for VLAN10 with DSCP0.
	ipv4vlan10flow := getIPv4Flow(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipv4vlan10flow", 0)
	ipv6vlan10flow := getIPv6Flow(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipv6vlan10flow", 0)
	t.Run("RT-3.1 Case1", func(t *testing.T) {
		t.Log("Matching ipv4 protocol to VRF10. Dropping IPv6 traffic in VRF10.")
		configureL2PBR(t, args.dut, "L2", "VRF10", "ipv4", 1)
		//defer pbr policy deletion
		defer pfpath.Policy("L2").Delete(t)

		//configure PBR on ingress port
		pfpath.Interface(port1.Name()).ApplyVrfSelectionPolicy().Replace(t, "L2")
		//defer deletion of policy from interface
		defer pfpath.Interface(port1.Name()).ApplyVrfSelectionPolicy().Delete(t)

		//traffic should pass
		testTrafficFlows(t, args.ate, args.top, true, ipv4vlan10flow)
		//traffic should drop
		testTrafficFlows(t, args.ate, args.top, false, ipv6vlan10flow)
	})

	//Case3 - Matching IPv4 protocol to VRF10, IPv6 protocol to VRF20. Dropping IPv6 traffic in VRF10 and IPv4 in VRF20.
	//Create IPv4 and IPv6 flows for VLAN20 with DSCP0.
	ipv4vlan20flow := getIPv4Flow(t, args.ate, srcEndPoint, dstEndPointVlan20, "ipv4vlan20flow", 0)
	ipv6vlan20flow := getIPv6Flow(t, args.ate, srcEndPoint, dstEndPointVlan20, "ipv6vlan20flow", 0)

	t.Run("RT-3.1 Case3", func(t *testing.T) {
		t.Log("Matching IPv4 protocol to VRF10, IPv6 protocol to VRF20. Dropping IPv6 traffic in VRF10 and IPv4 in VRF20.")
		configureL2PBR(t, args.dut, "L2", "VRF10", "ipv4", 1)
		configureL2PBRRule(t, args.dut, "L2", "VRF20", "ipv6", 2)
		//defer pbr policy deletion
		defer pfpath.Policy("L2").Delete(t)

		//configure PBR on ingress port
		pfpath.Interface(port1.Name()).ApplyVrfSelectionPolicy().Replace(t, "L2")
		//defer deletion of policy from interface
		defer pfpath.Interface(port1.Name()).ApplyVrfSelectionPolicy().Delete(t)

		//traffic should pass
		testTrafficFlows(t, args.ate, args.top, true, ipv4vlan10flow, ipv6vlan20flow)
		//traffic should drop
		testTrafficFlows(t, args.ate, args.top, false, ipv6vlan10flow, ipv4vlan20flow)
	})

	//Case2 - Match IPinIP protocol to VRF10. Drop IPv4 and IPv6 traffic in VRF10.
	//Create IPinIP flow for VLAN10 with DSCP0.
	ipinipvlan10flow := getIPinIPFlow(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipv4inipv4v10flow0", 0)
	t.Run("RT-3.1 Case2", func(t *testing.T) {
		t.Log("Match IPinIP protocol to VRF10. Drop IPv4 and IPv6 traffic in VRF10.")
		configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
		//defer pbr policy deletion
		defer pfpath.Policy("L3").Delete(t)

		//configure PBR on ingress port
		pfpath.Interface(port1.Name()).ApplyVrfSelectionPolicy().Replace(t, "L3")
		//defer deletion of policy from interface
		defer pfpath.Interface(port1.Name()).ApplyVrfSelectionPolicy().Delete(t)

		testTrafficFlows(t, args.ate, args.top, true, ipinipvlan10flow)
		testTrafficFlows(t, args.ate, args.top, false, ipv4vlan10flow, ipv6vlan10flow)
	})

	//Case4 - Match IPinIP and single DSCP46 to VRF10. Drop DSCP0 in VRF10.
	//Create IPinIP flow with DSCP46 for VLAN10.
	ipinipvlan10flowd46 := getIPinIPFlow(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipv4inipv4v10flow46", 46)
	t.Run("RT-3.1 Case4", func(t *testing.T) {
		t.Log("Match IPinIP and single DSCP46 to VRF10. Drop DSCP0 in VRF10.")
		configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{46})
		//defer pbr policy deletion
		defer pfpath.Policy("L3").Delete(t)

		//configure PBR on ingress port
		pfpath.Interface(port1.Name()).ApplyVrfSelectionPolicy().Replace(t, "L3")
		//defer deletion of policy from interface
		defer pfpath.Interface(port1.Name()).ApplyVrfSelectionPolicy().Delete(t)

		testTrafficFlows(t, args.ate, args.top, false, ipinipvlan10flow)
		testTrafficFlows(t, args.ate, args.top, true, ipinipvlan10flowd46)
	})

	//Case5 - Match IPinIP and single DSCP46, DSCP42 to VRF10. Drop DSCP0 in VRF10.
	//Create IPinIP flow with DSCP42 for VLAN10.
	ipinipvlan10flowd42 := getIPinIPFlow(t, args.ate, srcEndPoint, dstEndPointVlan10, "ipv4inipv4v10flow42", 42)
	t.Run("RT-3.1 Case5", func(t *testing.T) {
		t.Log("Match IPinIP and single DSCP46, DSCP42 to VRF10. Drop DSCP0 in VRF10.")
		configurePBR(t, args.dut, "L3", "VRF10", "ipv4", 1, oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{42, 46})
		//defer pbr policy deletion
		defer pfpath.Policy("L3").Delete(t)

		//configure PBR on ingress port
		pfpath.Interface(port1.Name()).ApplyVrfSelectionPolicy().Replace(t, "L3")
		//defer deletion of policy from interface
		defer pfpath.Interface(port1.Name()).ApplyVrfSelectionPolicy().Delete(t)

		testTrafficFlows(t, args.ate, args.top, false, ipinipvlan10flow)
		testTrafficFlows(t, args.ate, args.top, true, ipinipvlan10flowd46, ipinipvlan10flowd42)
	})
}
