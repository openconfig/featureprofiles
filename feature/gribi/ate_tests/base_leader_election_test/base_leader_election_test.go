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

package base_leader_election_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/topologies/binding/cisco/config"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of dut:port1 -> ate:port1,
// dut:port2 -> ate:port2, dut:port3 -> ate:port3, dut:port4 -> ate:port4, dut:port5 -> ate:port5,
// dut:port6 -> ate:port6, dut:port7 -> ate:port7 ,dut:port8 -> ate:port8
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30
//   * ate:port3 -> dut:port3 subnet 192.0.2.8/30
//   * ate:port4 -> dut:port3 subnet 192.0.2.12/30
//   * ate:port5 -> dut:port3 subnet 192.0.2.16/30
//   * ate:port6 -> dut:port3 subnet 192.0.2.20/30
//   * ate:port7 -> dut:port3 subnet 192.0.2.24/30
//   * ate:port8 -> dut:port3 subnet 192.0.2.28/30
//
//   * Destination network: 198.51.100.0/24

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
	instance      = "DEFAULT"
	ateDstNetCIDR = "198.51.100.1/32"
	hw            = true
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv6:    "192:0:2::1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv6:    "192:0:2::2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv6:    "192:0:2::5",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv6:    "192:0:2::6",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv6:    "192:0:2::9",
		IPv4Len: ipv4PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "192.0.2.10",
		IPv6:    "192:0:2::A",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.13",
		IPv6:    "192:0:2::D",
		IPv4Len: ipv4PrefixLen,
	}

	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		IPv4:    "192.0.2.14",
		IPv6:    "192:0:2::E",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort5 = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "192.0.2.17",
		IPv6:    "192:0:2::11",
		IPv4Len: ipv4PrefixLen,
	}

	atePort5 = attrs.Attributes{
		Name:    "atePort5",
		IPv4:    "192.0.2.18",
		IPv6:    "192:0:2::12",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort6 = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "192.0.2.21",
		IPv6:    "192:0:2::15",
		IPv4Len: ipv4PrefixLen,
	}

	atePort6 = attrs.Attributes{
		Name:    "atePort6",
		IPv4:    "192.0.2.22",
		IPv6:    "192:0:2::16",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort7 = attrs.Attributes{
		Desc:    "dutPort7",
		IPv4:    "192.0.2.25",
		IPv6:    "192:0:2::19",
		IPv4Len: ipv4PrefixLen,
	}

	atePort7 = attrs.Attributes{
		Name:    "atePort7",
		IPv4:    "192.0.2.26",
		IPv6:    "192:0:2::1A",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort8 = attrs.Attributes{
		Desc:    "dutPort8",
		IPv4:    "192.0.2.29",
		IPv6:    "192:0:2::1D",
		IPv4Len: ipv4PrefixLen,
	}

	atePort8 = attrs.Attributes{
		Name:    "atePort8",
		IPv4:    "192.0.2.30",
		IPv6:    "192:0:2::1E",
		IPv4Len: ipv4PrefixLen,
	}
)

//getL3PBRRule returns an IPv4 or IPv6 policy-forwarding rule configuration populated with protocol and/or DSCPset information.
func getL3PBRRule(networkInstance, iptype string, index uint32, protocol telemetry.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8) *telemetry.NetworkInstance_PolicyForwarding_Policy_Rule {

	r := telemetry.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	r.Action = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	if iptype == "ipv4" {
		r.Ipv4 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv4.DscpSet = dscpset
		}
	} else if iptype == "ipv6" {
		r.Ipv6 = &telemetry.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
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

func configurePBR(t *testing.T, dut *ondatra.DUTDevice, policyName, networkInstance, iptype string, index uint32, protocol telemetry.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8) {

	r1 := getL3PBRRule(networkInstance, iptype, index, protocol, dscpset)
	pf := telemetry.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(policyName)
	p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(r1)
	dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &pf)
}

func configurePbrDUT(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, source_port string) {

	port := dut.Port(t, source_port)
	pfpath := dut.Config().NetworkInstance("default").PolicyForwarding()
	//defer cleaning policy-forwarding
	defer pfpath.Delete(t)

	t.Log("Match IPinIP protocol to VRF10. Drop IPv4 and IPv6 traffic in VRF10.")
	configurePBR(t, dut, "Transit", "TE", "ipv4", 1, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	//defer pbr policy deletion
	defer pfpath.Policy("Transit").Delete(t)

	//configure PBR on ingress port
	pfpath.Interface(port.Name()).ApplyVrfSelectionPolicy().Replace(t, "Transit")
	//defer deletion of policy from interface
	defer pfpath.Interface(port.Name()).ApplyVrfSelectionPolicy().Delete(t)

	// t.Log("Remove Flowspec Config and add HW Module Config")
	// configToChange := "no flowspec \nhw-module profile pbr vrf-redirect\n"
	// util.GNMIWithText(ctx, t, dut, configToChange)
	// t.Log("Reload the router to activate hw module config")
	// util.ReloadDUT(t, dut)
}

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *telemetry.Interface, a *attrs.Attributes) *telemetry.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	s6 := s.GetOrCreateIpv6()
	if *deviations.InterfaceEnabled {
		s6.Enabled = ygot.Bool(true)
	}
	s6a := s6.GetOrCreateAddress(a.IPv6)
	s6a.PrefixLength = ygot.Uint8(ipv6PrefixLen)

	return i
}

func shutdownInterface(i *telemetry.Interface, state bool) *telemetry.Interface {
	i.Enabled = ygot.Bool(state)
	return i
}

// configureDUT configures port1, port2 and port3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	i1 := &telemetry.Interface{Name: ygot.String(p1.Name())}
	d.Interface(p1.Name()).Replace(t, configInterfaceDUT(i1, &dutPort1))

	p2 := dut.Port(t, "port2")
	i2 := &telemetry.Interface{Name: ygot.String(p2.Name())}
	d.Interface(p2.Name()).Replace(t, configInterfaceDUT(i2, &dutPort2))

	p3 := dut.Port(t, "port3")
	i3 := &telemetry.Interface{Name: ygot.String(p3.Name())}
	d.Interface(p3.Name()).Replace(t, configInterfaceDUT(i3, &dutPort3))

	p4 := dut.Port(t, "port4")
	i4 := &telemetry.Interface{Name: ygot.String(p4.Name())}
	d.Interface(p4.Name()).Replace(t, configInterfaceDUT(i4, &dutPort4))

	p5 := dut.Port(t, "port5")
	i5 := &telemetry.Interface{Name: ygot.String(p5.Name())}
	d.Interface(p5.Name()).Replace(t, configInterfaceDUT(i5, &dutPort5))

	p6 := dut.Port(t, "port6")
	i6 := &telemetry.Interface{Name: ygot.String(p6.Name())}
	d.Interface(p6.Name()).Replace(t, configInterfaceDUT(i6, &dutPort6))

	p7 := dut.Port(t, "port7")
	i7 := &telemetry.Interface{Name: ygot.String(p7.Name())}
	d.Interface(p7.Name()).Replace(t, configInterfaceDUT(i7, &dutPort7))

	p8 := dut.Port(t, "port8")
	i8 := &telemetry.Interface{Name: ygot.String(p8.Name())}
	d.Interface(p8.Name()).Replace(t, configInterfaceDUT(i8, &dutPort8))
}

// configureATE configures port1, port2 and port3 on the ATE.
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

	p3 := ate.Port(t, "port3")
	i3 := top.AddInterface(atePort3.Name).WithPort(p3)
	i3.IPv4().
		WithAddress(atePort3.IPv4CIDR()).
		WithDefaultGateway(dutPort3.IPv4)
	i3.IPv6().
		WithAddress(atePort3.IPv6CIDR()).
		WithDefaultGateway(dutPort3.IPv6)

	p4 := ate.Port(t, "port4")
	i4 := top.AddInterface(atePort4.Name).WithPort(p4)
	i4.IPv4().
		WithAddress(atePort4.IPv4CIDR()).
		WithDefaultGateway(dutPort4.IPv4)
	i4.IPv6().
		WithAddress(atePort4.IPv6CIDR()).
		WithDefaultGateway(dutPort4.IPv6)

	p5 := ate.Port(t, "port5")
	i5 := top.AddInterface(atePort5.Name).WithPort(p5)
	i5.IPv4().
		WithAddress(atePort5.IPv4CIDR()).
		WithDefaultGateway(dutPort5.IPv4)
	i5.IPv6().
		WithAddress(atePort5.IPv6CIDR()).
		WithDefaultGateway(dutPort5.IPv6)

	p6 := ate.Port(t, "port6")
	i6 := top.AddInterface(atePort6.Name).WithPort(p6)
	i6.IPv4().
		WithAddress(atePort6.IPv4CIDR()).
		WithDefaultGateway(dutPort6.IPv4)
	i6.IPv6().
		WithAddress(atePort6.IPv6CIDR()).
		WithDefaultGateway(dutPort6.IPv6)

	p7 := ate.Port(t, "port7")
	i7 := top.AddInterface(atePort7.Name).WithPort(p7)
	i7.IPv4().
		WithAddress(atePort7.IPv4CIDR()).
		WithDefaultGateway(dutPort7.IPv4)
	i7.IPv6().
		WithAddress(atePort7.IPv6CIDR()).
		WithDefaultGateway(dutPort7.IPv6)

	p8 := ate.Port(t, "port8")
	i8 := top.AddInterface(atePort8.Name).WithPort(p8)
	i8.IPv4().
		WithAddress(atePort8.IPv4CIDR()).
		WithDefaultGateway(dutPort8.IPv4)
	i8.IPv6().
		WithAddress(atePort8.IPv6CIDR()).
		WithDefaultGateway(dutPort8.IPv6)
	return top
}

func addAteISISL2(t *testing.T, topo *ondatra.ATETopology, atePort, areaId, network_name string, metric uint32, v4prefix string, v6prefix string, count uint32) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	network := intfs[atePort].AddNetwork(network_name)
	//IPReachabilityConfig :=
	network.ISIS().WithIPReachabilityMetric(metric + 1)
	if len(v4prefix) != 0 {
		network.IPv4().WithAddress(v4prefix).WithCount(count)
	}
	if len(v6prefix) != 0 {
		network.IPv6().WithAddress(v6prefix).WithCount(count)
	}
	intfs[atePort].ISIS().WithAreaID(areaId).WithLevelL2().WithNetworkTypePointToPoint().WithMetric(metric).WithWideMetricEnabled(true)
}

func addAteEBGPPeer(t *testing.T, topo *ondatra.ATETopology, atePort, peerAddress string, localAsn uint32, network_name, nexthop, prefix string, count uint32, useLoopback bool) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	//

	//Add prefixes, Add network instance
	if prefix != "" {
		network := intfs[atePort].AddNetwork(network_name)
		bgpAttribute := network.BGP()
		bgpAttribute.WithActive(true).WithNextHopAddress(nexthop)
		network.IPv4().WithAddress(prefix).WithCount(count)
	}
	//Create BGP instance
	bgp := intfs[atePort].BGP()
	bgpPeer := bgp.AddPeer().WithPeerAddress(peerAddress).WithLocalASN(localAsn).WithTypeExternal()
	bgpPeer.WithOnLoopback(useLoopback)

	//Update bgpCapabilities
	bgpPeer.Capabilities().WithIPv4UnicastEnabled(true).WithIPv6UnicastEnabled(true).WithGracefulRestart(true)
}

// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint *ondatra.Interface, dstEndPoint []ondatra.Endpoint) {
	ethHeader := ondatra.NewEthernetHeader()
	// ethHeader.WithSrcAddress("00:11:01:00:00:01")
	// ethHeader.WithDstAddress("00:01:00:02:00:00")

	flow := []*ondatra.Flow{}
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.DstAddressRange().
		WithMin("198.51.100.1").
		WithMax("198.51.100.254").
		WithCount(1)

	innerIpv4Header := ondatra.NewIPv4Header()
	innerIpv4Header.WithSrcAddress("200.1.0.2")
	innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(1000).WithStep("0.0.0.1")

	flow = append(flow, ate.Traffic().NewFlow(fmt.Sprintf("Flow")).
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...).
		WithHeaders(ethHeader, ipv4Header, innerIpv4Header).WithFrameRateFPS(10).WithFrameSize(300))

	ate.Traffic().Start(t, flow...)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	time.Sleep(time.Minute)

	for _, f := range flow {
		flowPath := ate.Telemetry().Flow(f.Name())
		if got := flowPath.LossPct().Get(t); got > 0 {
			t.Errorf("LossPct for flow %s got %g, want 0", f.Name(), got)
		}
	}
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx     context.Context
	clientA *gribi.GRIBIHandler
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     *ondatra.ATETopology
}

func testIPv4BackUpSwitchDrop(ctx context.Context, t *testing.T, args *testArgs) {

	// Add an IPv4Entry for 198.51.100.0/24 pointing to ATE port-3 via gRIBI-B,
	// ensure that the entry is active through AFT telemetry and traffic.

	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", ateDstNetCIDR)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// LEVEL 2

	// Creating a backup NHG with ID 101 (bkhgIndex_2), dropping
	// NH ID 10 (nhbIndex_2_1)

	args.clientA.AddNH(t, 10, "192.0.2.100", instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.AddNH(t, 100, "192.0.2.40", instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 200, "192.0.2.42", instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, 100, "TE", instance, fluent.InstalledInRIB)

	// LEVEL 1
	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.clientA.AddNH(t, 1000, atePort2.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 1100, atePort3.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 1200, atePort4.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 1300, atePort5.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, "192.0.2.40/32", 1000, instance, "", fluent.InstalledInRIB)

	args.clientA.AddNH(t, 2000, atePort6.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 2100, atePort7.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, "192.0.2.42/32", 2000, instance, "", fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf && "atePort8" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	d := args.dut.Config()
	for _, intf := range interface_names {
		p := args.dut.Port(t, intf)
		i := &telemetry.Interface{Name: ygot.String(p.Name())}
		d.Interface(p.Name()).Replace(t, shutdownInterface(i, false))
		testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint)
	}
	// checking traffic on backup
	time.Sleep(time.Minute)
	testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint)

	//adding back interface configurations
	configureDUT(t, args.dut)
}

func testIPv4BackUpSwitchDecap(ctx context.Context, t *testing.T, args *testArgs) {

	// Add an IPv4Entry for 198.51.100.0/24 pointing to ATE port-3 via gRIBI-B,
	// ensure that the entry is active through AFT telemetry and traffic.

	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", ateDstNetCIDR)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// LEVEL 2

	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.clientA.AddNH(t, 10, "decap", instance, instance, fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.AddNH(t, 100, "192.0.2.40", instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 200, "192.0.2.42", instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, 100, "TE", instance, fluent.InstalledInRIB)

	// LEVEL 1

	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.clientA.AddNH(t, 1000, atePort2.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 1100, atePort3.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 1200, atePort4.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 1300, atePort5.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, "192.0.2.40/32", 1000, instance, "", fluent.InstalledInRIB)

	args.clientA.AddNH(t, 2000, atePort6.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 2100, atePort7.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, "192.0.2.42/32", 2000, instance, "", fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf && "atePort8" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	d := args.dut.Config()
	for _, intf := range interface_names {
		p := args.dut.Port(t, intf)
		i := &telemetry.Interface{Name: ygot.String(p.Name())}
		d.Interface(p.Name()).Replace(t, shutdownInterface(i, false))
		testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint)
	}
	// checking traffic on backup
	time.Sleep(time.Minute)
	testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint)

	//adding back interface configurations
	configureDUT(t, args.dut)
}

func testIPv4BackUpSwitchCase3(ctx context.Context, t *testing.T, args *testArgs) {

	// Add an IPv4Entry for 198.51.100.0/24 pointing to ATE port-3 via gRIBI-B,
	// ensure that the entry is active through AFT telemetry and traffic.

	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", ateDstNetCIDR)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// LEVEL 2

	// Create REPAIR INSTANCE
	args.clientA.AddNH(t, 3000, atePort8.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 3, 0, map[uint64]uint64{3000: 100}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, 3, "REPAIR", instance, fluent.InstalledInRIB)

	// Create REPAIRED INSTANCE POINTING TO THE SAME LEVEL1 VIPS
	args.clientA.AddNH(t, 100, "192.0.2.40", instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 200, "192.0.2.42", instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 2, 0, map[uint64]uint64{100: 85, 200: 15}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, 2, "REPAIRED", instance, fluent.InstalledInRIB)

	// Creating a backup NHG with ID 101 (bkhgIndex_2), pointing to vrf Repair
	// NH ID 10 (nhbIndex_2_1)

	args.clientA.AddNH(t, 10, "REPAIR", "DEFAULT", "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.AddNH(t, 100, "192.0.2.40", instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 200, "192.0.2.42", instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, 100, "TE", instance, fluent.InstalledInRIB)

	// LEVEL 1

	// VIP1: NHG ID 1000 (nhgIndex_1_1)
	//		- PATH1 NH ID 1000 (nhIndex_1_11), weight 50, outgoing Port2
	//		- PATH2 NH ID 1100 (nhIndex_1_12), weight 30, outgoing Port3
	//		- PATH3 NH ID 1200 (nhIndex_1_13), weight 15, outgoing Port4
	//		- PATH4 NH ID 1300 (nhIndex_1_14), weight  5, outgoing Port5
	// VIP2: NHG ID 2000 (nhgIndex_1_2)
	//		- PATH1 NH ID 2000 (nhIndex_1_21), weight 60, outgoing Port6
	//		- PATH2 NH ID 2100 (nhIndex_1_22), weight 35, outgoing Port7
	//		- PATH3 NH ID 2200 (nhIndex_1_23), weight  5, outgoing Port8

	args.clientA.AddNH(t, 1000, atePort2.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 1100, atePort3.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 1200, atePort4.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 1300, atePort5.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, "192.0.2.40/32", 1000, instance, "", fluent.InstalledInRIB)

	args.clientA.AddNH(t, 2000, atePort6.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNH(t, 2100, atePort7.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, "192.0.2.42/32", 2000, instance, "", fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf && "atePort8" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	d := args.dut.Config()
	for _, intf := range interface_names {
		p := args.dut.Port(t, intf)
		i := &telemetry.Interface{Name: ygot.String(p.Name())}
		d.Interface(p.Name()).Replace(t, shutdownInterface(i, false))
		testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint)
	}
	// checking traffic on backup
	time.Sleep(time.Minute)
	testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint)

	//adding back interface configurations
	configureDUT(t, args.dut)
}

func testIPv4BackUpSingleNH(ctx context.Context, t *testing.T, args *testArgs) {

	// Add an IPv4Entry for 198.51.100.0/24 pointing to ATE port-3 via gRIBI-B,
	// ensure that the entry is active through AFT telemetry and traffic.

	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", ateDstNetCIDR)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// Verify the entry for 1.0.0.0/8 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()

	t.Log("going to program Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 198.51.100.1  0012.0100.0001 arpa")

	// LEVEL 2
	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)

	args.clientA.AddNH(t, 10, atePort8.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, fluent.InstalledInRIB)

	args.clientA.AddNH(t, 100, atePort2.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 100, 101, map[uint64]uint64{100: 100}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, 100, "TE", instance, fluent.InstalledInRIB)

	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort2" == intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}
	testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint)

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port2"}
	d := args.dut.Config()
	for _, intf := range interface_names {
		p := args.dut.Port(t, intf)
		i := &telemetry.Interface{Name: ygot.String(p.Name())}
		d.Interface(p.Name()).Replace(t, shutdownInterface(i, false))
	}

	testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint)

	t.Log("going to remove Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "no arp 198.51.100.1 0012.0100.0001 arpa")
	config.TextWithGNMI(args.ctx, t, args.dut, "interface fourHundredGigE 0/0/0/11 arp learning disable")

	//adding back interface configurations
	configureDUT(t, args.dut)

	t.Logf("deleting all the IPV4 entries added")
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	args.clientA.AddNH(t, 3000, atePort3.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 300, 0, map[uint64]uint64{3000: 100}, instance, fluent.InstalledInRIB)

	args.clientA.AddNH(t, 100, "193.0.2.1", instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 100, 300, map[uint64]uint64{100: 100}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, ateDstNetCIDR, 100, "TE", instance, fluent.InstalledInRIB)

	args.clientA.AddNH(t, 2000, atePort2.IPv4, instance, "", fluent.InstalledInRIB)
	args.clientA.AddNHG(t, 200, 0, map[uint64]uint64{2000: 100}, instance, fluent.InstalledInRIB)
	args.clientA.AddIPv4(t, "193.0.2.1/32", 200, instance, "", fluent.InstalledInRIB)

	args.clientA.RemoveIPv4(t, "193.0.2.1/32", 200, instance, "", fluent.InstalledInRIB)
	args.clientA.RemoveNHG(t, 200, 0, map[uint64]uint64{2000: 100}, instance, fluent.InstalledInRIB)
	args.clientA.RemoveNH(t, 2000, atePort2.IPv4, instance, fluent.InstalledInRIB)

	// checking traffic on backup
	time.Sleep(time.Minute)
	updated_dstEndPoint = []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort3" == intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}
	testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint)
}

func TestBackUp(t *testing.T) {
	deviations.InterfaceEnabled = &[]bool{false}[0]
	t.Log("Name: BackUp")
	t.Log("Description: Connect gRIBI clientA and B to DUT using SINGLE_PRIMARY client redundancy with persistance and RibACK")

	dut := ondatra.DUT(t, "dut")

	// Dial gRIBI
	ctx := context.Background()

	// Configure the DUT
	configureDUT(t, dut)

	port := dut.Port(t, "port1")
	pfpath := dut.Config().NetworkInstance("default").PolicyForwarding()
	//defer cleaning policy-forwarding
	defer pfpath.Delete(t)

	t.Log("Match IPinIP protocol to VRF10. Drop IPv4 and IPv6 traffic in VRF10.")
	configurePBR(t, dut, "Transit", "TE", "ipv4", 1, telemetry.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	//defer pbr policy deletion
	defer pfpath.Policy("Transit").Delete(t)

	//configure PBR on ingress port
	pfpath.Interface(port.Name()).ApplyVrfSelectionPolicy().Replace(t, "Transit")
	//defer deletion of policy from interface
	defer pfpath.Interface(port.Name()).ApplyVrfSelectionPolicy().Delete(t)

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	addAteISISL2(t, top, "atePort8", "B4", "testing", 20, ateDstNetCIDR, "198:51:100::1/128", uint32(1))
	addAteEBGPPeer(t, top, "atePort8", "192.0.2.29", 64001, "bgp_network", "192.0.2.30", "", 1, false)
	top.Push(t).StartProtocols(t)

	test := []struct {
		name string
		desc string
		fn   func(ctx context.Context, t *testing.T, args *testArgs)
	}{
		{
			name: "IPv4BackUpSwitchDrop",
			desc: "Set primary and backup path with gribi and shutdown all the primary path validating traffic switching over backup path and dropping",
			fn:   testIPv4BackUpSwitchDrop,
		},
		{
			name: "IPv4BackUpSwitchDecap",
			desc: "Set primary and backup path with gribi and shutdown all the primary path validating traffic switching over default backup path ",
			fn:   testIPv4BackUpSwitchDecap,
		},
		{
			name: "IPv4BackUpSwitchCase",
			desc: "Set primary and backup path with gribi and shutdown all the primary path validating traffic switching over backup path ",
			fn:   testIPv4BackUpSwitchCase3,
		},
		{
			name: "IPv4BackUpSingleNH",
			desc: "Set primary and backup path with gribi and shutdown all the primary path validating traffic switching over backup path ",
			fn:   testIPv4BackUpSingleNH,
		},
	}
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			// Configure the gRIBI client clientA
			clientA := gribi.GRIBIHandler{
				DUT:         dut,
				FibACK:      false,
				Persistence: true,
			}
			defer clientA.Close(t)
			if err := clientA.Start(t); err != nil {
				t.Fatalf("gRIBI Connection can not be established")
			}
			args := &testArgs{
				ctx:     ctx,
				clientA: &clientA,
				dut:     dut,
				ate:     ate,
				top:     top,
			}
			tt.fn(ctx, t, args)
		})
	}
}
