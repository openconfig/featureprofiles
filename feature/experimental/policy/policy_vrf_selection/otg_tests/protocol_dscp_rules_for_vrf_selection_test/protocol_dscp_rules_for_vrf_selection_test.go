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

package policy_based_vrf_selection_test

import (
	"strconv"
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

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut         *ondatra.DUTDevice
	ate         *ondatra.ATEDevice
	top         *ondatra.ATETopology
	srcEndPoint *ondatra.Interface
	policyName  string
	iptype      string
	protocol    oc.E_PacketMatchTypes_IP_PROTOCOL
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

	// configure vlans on ATE port2
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

// configNetworkInstance creates VRFs and subinterfaces and then applies VRFs on the subinterfaces.
func configNetworkInstance(t *testing.T, dut *ondatra.DUTDevice, vrfname string, intfname string, subint uint32) {
	// create empty subinterface
	si := &oc.Interface_Subinterface{}
	si.Index = ygot.Uint32(subint)
	gnmi.Replace(t, dut, gnmi.OC().Interface(intfname).Subinterface(subint).Config(), si)

	// create vrf and apply on subinterface
	v := &oc.NetworkInstance{
		Name: ygot.String(vrfname),
	}
	vi := v.GetOrCreateInterface(intfname + "." + strconv.Itoa(int(subint)))
	vi.Subinterface = ygot.Uint32(subint)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfname).Config(), v)
}

// getSubInterface returns a subinterface configuration populated with IP addresses and VLAN ID.
func getSubInterface(dutPort *attrs.Attributes, index uint32, vlanID uint16) *oc.Interface_Subinterface {
	s := &oc.Interface_Subinterface{}
	// unshut sub/interface
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
	if index != 0 {
		if *deviations.DeprecatedVlanID {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
		}
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

// configureDUT configures the base configuration on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1))

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2))

	outpath := d.Interface(p2.Name())
	// create VRFs and VRF enabled subinterfaces
	configNetworkInstance(t, dut, "VRF10", p2.Name(), uint32(1))

	// configure IP addresses on subinterfaces
	gnmi.Update(t, dut, outpath.Subinterface(1).Config(), getSubInterface(&dutPort2Vlan10, 1, 10))

	configNetworkInstance(t, dut, "VRF20", p2.Name(), uint32(2))
	gnmi.Update(t, dut, outpath.Subinterface(2).Config(), getSubInterface(&dutPort2Vlan20, 2, 20))

	configNetworkInstance(t, dut, "VRF30", p2.Name(), uint32(3))
	gnmi.Update(t, dut, outpath.Subinterface(3).Config(), getSubInterface(&dutPort2Vlan30, 3, 30))
}

// getIPinIPFlow returns an IPv4inIPv4 *ondatra.Flow with provided DSCP value for a given set of endpoints.
func getIPinIPFlow(args *testArgs, dstEndPoint ondatra.Endpoint, flowName string, dscp uint8) *ondatra.Flow {

	ethHeader := ondatra.NewEthernetHeader()
	outerIPHeader := ondatra.NewIPv4Header().WithDSCP(dscp)
	innerIPHeader := ondatra.NewIPv4Header()
	innerIPHeader.WithSrcAddress("198.51.100.1")
	innerIPHeader.DstAddressRange().WithMin("203.0.113.1").WithCount(10000).WithStep("0.0.0.1")
	flow := args.ate.Traffic().NewFlow(flowName)
	flow.WithHeaders(ethHeader, outerIPHeader, innerIPHeader)
	flow.WithSrcEndpoints(args.srcEndPoint)
	flow.WithDstEndpoints(dstEndPoint)
	flow.WithFrameRateFPS(100)
	flow.WithFrameSize(1024)
	return flow
}

// testTrafficFlows verifies traffic for one or more flows.
func testTrafficFlows(t *testing.T, args *testArgs, expectPass bool, flow ...*ondatra.Flow) {

	args.ate.Traffic().Start(t, flow...)
	time.Sleep(60 * time.Second)
	args.ate.Traffic().Stop(t)
	if expectPass {
		t.Log("Expecting traffic to pass for the flows")
	} else {
		t.Log("Expecting traffic to fail for the flows")
	}

	// log stats
	t.Log("All flow LossPct: ", gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().LossPct().State()))
	t.Log("FlowAny InPkts  : ", gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().Counters().InPkts().State()))
	t.Log("FlowAny OutPkts : ", gnmi.GetAll(t, args.ate, gnmi.OC().FlowAny().Counters().OutPkts().State()))

	flowPath := gnmi.OC().FlowAny()
	if got := gnmi.GetAll(t, args.ate, flowPath.LossPct().State()); len(got) == 0 {
		t.Fatalf("Flow stats count not correct, wanted > 0, got 0")
	} else {
		for i, lossPct := range got {
			if (expectPass == true) && (lossPct == 0) {
				t.Logf("Traffic for %v flow is passing as expected", flow[i].Name())
			} else if (expectPass == false) && (lossPct == 100) {
				t.Logf("Traffic for %v flow is failing as expected", flow[i].Name())

			} else {
				t.Fatalf("Traffic is not working as expected for flow: %v.", flow[i].Name())
			}

		}
	}
}

// getL3PBRRule returns an IPv4 or IPv6 policy-forwarding rule configuration populated with protocol and/or DSCPset information.
func getL3PBRRule(args *testArgs, networkInstance string, index uint32, dscpset []uint8) *oc.NetworkInstance_PolicyForwarding_Policy_Rule {
	r := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	if args.iptype == "ipv4" {
		r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: args.protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv4.DscpSet = dscpset
		}
	} else if args.iptype == "ipv6" {
		r.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: args.protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv6.DscpSet = dscpset
		}
	} else {
		return nil
	}
	return &r

}

// getPBRPolicyForwarding returns pointer to policy-forwarding populated with pbr policy and rules
func getPBRPolicyForwarding(args *testArgs, rules ...*oc.NetworkInstance_PolicyForwarding_Policy_Rule) *oc.NetworkInstance_PolicyForwarding {
	pf := oc.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(args.policyName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	for _, rule := range rules {
		p.AppendRule(rule)
	}
	return &pf
}

func TestPBR(t *testing.T) {
	t.Logf("Description: Test RT3.2 with multiple DSCP, IPinIP protocol rule based VRF selection")
	dut := ondatra.DUT(t, "dut")

	// configure DUT
	configureDUT(t, dut)

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	top.Push(t).StartProtocols(t)

	args := &testArgs{
		dut:         dut,
		ate:         ate,
		top:         top,
		srcEndPoint: top.Interfaces()["atePort1"],
		policyName:  "L3",
		iptype:      "ipv4",
		protocol:    oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
	}

	// dut ingress interface
	port1 := dut.Port(t, "port1")

	// dut egress vlan interfaces
	ateInterfaces := args.top.Interfaces()
	dstEndPointVlan10 := ateInterfaces["atePort2Vlan10"]
	dstEndPointVlan20 := ateInterfaces["atePort2Vlan20"]
	dstEndPointVlan30 := ateInterfaces["atePort2Vlan30"]

	cases := []struct {
		name         string
		desc         string
		policy       *oc.NetworkInstance_PolicyForwarding
		passingFlows []*ondatra.Flow
		failingFlows []*ondatra.Flow
	}{
		{
			name: "RT3.2 Case1",
			desc: "Ensure matching IPinIP with DSCP (10 - VRF10, 20- VRF20, 30-VRF30) traffic reaches appropriate VLAN.",
			policy: getPBRPolicyForwarding(args,
				getL3PBRRule(args, "VRF10", 1, []uint8{10}),
				getL3PBRRule(args, "VRF20", 2, []uint8{20}),
				getL3PBRRule(args, "VRF30", 3, []uint8{30})),
			// use IPinIP DSCP10, DSCP20, DSCP30 flows for VLAN10, VLAN20 and VLAN30 respectively.
			passingFlows: []*ondatra.Flow{
				getIPinIPFlow(args, dstEndPointVlan10, "ipinipd10", 10),
				getIPinIPFlow(args, dstEndPointVlan20, "ipinipd20", 20),
				getIPinIPFlow(args, dstEndPointVlan30, "ipinipd30", 30)},
		},
		{
			name: "RT3.2 Case2",
			desc: "Ensure matching IPinIP with DSCP (10-12 - VRF10, 20-22- VRF20, 30-32-VRF30) traffic reaches appropriate VLAN.",
			policy: getPBRPolicyForwarding(args,
				getL3PBRRule(args, "VRF10", 1, []uint8{10, 11, 12}),
				getL3PBRRule(args, "VRF20", 2, []uint8{20, 21, 22}),
				getL3PBRRule(args, "VRF30", 3, []uint8{30, 31, 32})),
			// use IPinIP flows with DSCP10-12 for VLAN10, DSCP20-22 for VLAN20, DSCP30-32 for VLAN30.
			passingFlows: []*ondatra.Flow{
				getIPinIPFlow(args, dstEndPointVlan10, "ipinipd10", 10),
				getIPinIPFlow(args, dstEndPointVlan10, "ipinipd11", 11),
				getIPinIPFlow(args, dstEndPointVlan10, "ipinipd12", 12),

				getIPinIPFlow(args, dstEndPointVlan20, "ipinipd20", 20),
				getIPinIPFlow(args, dstEndPointVlan20, "ipinipd21", 21),
				getIPinIPFlow(args, dstEndPointVlan20, "ipinipd22", 22),

				getIPinIPFlow(args, dstEndPointVlan30, "ipinipd30", 30),
				getIPinIPFlow(args, dstEndPointVlan30, "ipinipd31", 31),
				getIPinIPFlow(args, dstEndPointVlan30, "ipinipd32", 32)},
		},
		{
			name: "RT3.2 Case3",
			desc: "Ensure first matching of IPinIP with DSCP (10-12 - VRF10, 10-12 - VRF20) rule takes precedence.",
			policy: getPBRPolicyForwarding(args,
				getL3PBRRule(args, "VRF10", 1, []uint8{10, 11, 12}),
				getL3PBRRule(args, "VRF20", 2, []uint8{10, 11, 12})),
			// use IPinIP DSCP10-12 flows for VLAN10 as well as VLAN20.
			passingFlows: []*ondatra.Flow{
				getIPinIPFlow(args, dstEndPointVlan10, "ipinipd10", 10),
				getIPinIPFlow(args, dstEndPointVlan10, "ipinipd11", 11),
				getIPinIPFlow(args, dstEndPointVlan10, "ipinipd12", 12)},
			failingFlows: []*ondatra.Flow{
				getIPinIPFlow(args, dstEndPointVlan20, "ipinipd10v20", 10),
				getIPinIPFlow(args, dstEndPointVlan20, "ipinipd11v20", 11),
				getIPinIPFlow(args, dstEndPointVlan20, "ipinipd12v20", 12)},
		},
		{
			name: "RT3.2 Case4",
			desc: "Ensure matching IPinIP to VRF10, IPinIP with DSCP20 to VRF20 causes unspecified DSCP IPinIP traffic to match VRF10.",
			policy: getPBRPolicyForwarding(args,
				getL3PBRRule(args, "VRF10", 1, []uint8{}),
				getL3PBRRule(args, "VRF20", 2, []uint8{20})),
			// use IPinIP DSCP10-12 flows to match IPinIP to VRF10
			// use IPinIP DSCP20 flow to match to VRF20
			// use IPinIP DSCP10-12 flows to match to VRF20 to show they fail for VRF20
			passingFlows: []*ondatra.Flow{
				getIPinIPFlow(args, dstEndPointVlan10, "ipinipd10", 10),
				getIPinIPFlow(args, dstEndPointVlan10, "ipinipd11", 11),
				getIPinIPFlow(args, dstEndPointVlan10, "ipinipd12", 12)},
			failingFlows: []*ondatra.Flow{
				getIPinIPFlow(args, dstEndPointVlan20, "ipinipd10v20", 10),
				getIPinIPFlow(args, dstEndPointVlan20, "ipinipd11v20", 11),
				getIPinIPFlow(args, dstEndPointVlan20, "ipinipd12v20", 12),
				getIPinIPFlow(args, dstEndPointVlan20, "ipinipd20", 20)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Log(tc.desc)
			pfpath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).PolicyForwarding()

			//configure pbr policy-forwarding
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).PolicyForwarding().Config(), tc.policy)
			// defer cleaning policy-forwarding
			defer gnmi.Delete(t, args.dut, pfpath.Config())

			// apply pbr policy on ingress interface
			gnmi.Replace(t, args.dut, pfpath.Interface(port1.Name()).ApplyVrfSelectionPolicy().Config(), args.policyName)

			// defer deletion of policy from interface
			defer gnmi.Delete(t, args.dut, pfpath.Interface(port1.Name()).ApplyVrfSelectionPolicy().Config())

			// traffic should pass
			testTrafficFlows(t, args, true, tc.passingFlows...)

			if len(tc.failingFlows) > 0 {
				// traffic should fail
				testTrafficFlows(t, args, false, tc.failingFlows...)
			}
		})
	}
}
