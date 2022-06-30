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
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	spb "github.com/openconfig/gribi/v1/proto/service"
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
	ipv4PrefixLen         = 30
	ipv6PrefixLen         = 126
	instance              = "DEFAULT"
	dstPfx                = "198.51.100.1"
	dstPfxMask            = "32"
	dstPfxMin             = "198.51.100.1"
	dstPfxCount           = 100
	dstPfx1               = "11.1.1.1"
	dstPfxCount1          = 10
	innersrcPfx           = "200.1.0.1"
	innerdstPfxMin_bgp    = "202.1.0.1"
	innerdstPfxCount_bgp  = 100
	innerdstPfxMin_isis   = "201.1.0.1"
	innerdstPfxCount_isis = 100
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

// getL3PBRRule returns an IPv4 or IPv6 policy-forwarding rule configuration populated with protocol and/or DSCPset information.
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

// configurePBR creates policy in network instance default
func configurePBR(t *testing.T, dut *ondatra.DUTDevice, policyName, networkInstance, iptype string, index uint32, protocol telemetry.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8) {

	r1 := getL3PBRRule(networkInstance, iptype, index, protocol, dscpset)
	pf := telemetry.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(policyName)
	p.Type = telemetry.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(r1)
	dut.Config().NetworkInstance("default").PolicyForwarding().Replace(t, &pf)
}

// configurePbrDUT assigns policy to the source interface
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

// interfaceaction shuts/unshuts provided interface
func (a *testArgs) interfaceaction(t *testing.T, port string, action bool) {
	// ateP := a.ate.Port(t, port)
	dutP := a.dut.Port(t, port)
	if action {
		// a.ate.Operations().NewSetInterfaceState().WithPhysicalInterface(ateP).WithStateEnabled(true).Operate(t)
		a.dut.Config().Interface(dutP.Name()).Enabled().Replace(t, true)
		a.dut.Telemetry().Interface(dutP.Name()).OperStatus().Await(t, time.Minute, telemetry.Interface_OperStatus_UP)
	} else {
		// a.ate.Operations().NewSetInterfaceState().WithPhysicalInterface(ateP).WithStateEnabled(false).Operate(t)
		a.dut.Config().Interface(dutP.Name()).Enabled().Replace(t, false)
		a.dut.Telemetry().Interface(dutP.Name()).OperStatus().Await(t, time.Minute, telemetry.Interface_OperStatus_DOWN)
	}
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

// addAteISISL2 configures ISIS L2 ATE config
func addAteISISL2(t *testing.T, topo *ondatra.ATETopology, atePort, areaId, network_name string, metric uint32, v4prefix string, count uint32) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	network := intfs[atePort].AddNetwork(network_name)
	network.ISIS().WithIPReachabilityMetric(metric + 1)
	network.IPv4().WithAddress(v4prefix).WithCount(count)
	rnetwork := intfs[atePort].AddNetwork("recursive")
	rnetwork.ISIS().WithIPReachabilityMetric(metric + 1)
	rnetwork.IPv4().WithAddress("100.100.100.100/32")
	intfs[atePort].ISIS().WithAreaID(areaId).WithLevelL2().WithNetworkTypePointToPoint().WithMetric(metric).WithWideMetricEnabled(true)
}

// addAteEBGPPeer configures EBGP ATE config
func addAteEBGPPeer(t *testing.T, topo *ondatra.ATETopology, atePort, peerAddress string, localAsn uint32, network_name, nexthop, prefix string, count uint32, useLoopback bool) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	//

	network := intfs[atePort].AddNetwork(network_name)
	bgpAttribute := network.BGP()
	bgpAttribute.WithActive(true).WithNextHopAddress(nexthop)

	//Add prefixes, Add network instance
	if prefix != "" {

		network.IPv4().WithAddress(prefix).WithCount(count)
	}
	//Create BGP instance
	bgp := intfs[atePort].BGP()
	bgpPeer := bgp.AddPeer().WithPeerAddress(peerAddress).WithLocalASN(localAsn).WithTypeExternal()
	bgpPeer.WithOnLoopback(useLoopback)

	//Update bgpCapabilities
	bgpPeer.Capabilities().WithIPv4UnicastEnabled(true).WithIPv6UnicastEnabled(true).WithGracefulRestart(true)
}

// addPrototoAte calls ISIS/BGP api
func addPrototoAte(t *testing.T, top *ondatra.ATETopology) {

	// addAteISISL2(t, top, "atePort8", "B4", "isis_network", 20, innerdstPfxMin_isis+"/"+dstPfxMask, uint32(innerdstPfxCount_isis))
	// addAteEBGPPeer(t, top, "atePort8", dutPort8.IPv4, 64001, "bgp_network", atePort8.IPv4, innerdstPfxMin_bgp+"/"+dstPfxMask, innerdstPfxCount_bgp, false)

	//advertising 100.100.100.100/32 for bgp resolve over IGP prefix
	intfs := top.Interfaces()
	intfs["atePort8"].WithIPv4Loopback("100.100.100.100/32")
	addAteISISL2(t, top, "atePort8", "B4", "isis_network", 20, innerdstPfxMin_isis+"/"+dstPfxMask, uint32(innerdstPfxCount_isis))
	addAteEBGPPeer(t, top, "atePort8", dutPort8.IPv4, 64001, "bgp_recursive", atePort8.IPv4, innerdstPfxMin_bgp+"/"+dstPfxMask, innerdstPfxCount_bgp, true)
	top.Push(t).StartProtocols(t)
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE interface dst.
func (a *testArgs) createFlow(name string, srcEndPoint *ondatra.Interface, dstEndPoint []ondatra.Endpoint, innerdstPfxMin string, innerdstPfxCount uint32) *ondatra.Flow {
	hdr := ondatra.NewIPv4Header()
	hdr.WithSrcAddress(dutPort1.IPv4).DstAddressRange().WithMin(dstPfxMin).WithCount(dstPfxCount).WithStep("0.0.0.1")

	innerIpv4Header := ondatra.NewIPv4Header()
	innerIpv4Header.WithSrcAddress(innersrcPfx)
	innerIpv4Header.DstAddressRange().WithMin(innerdstPfxMin).WithCount(innerdstPfxCount).WithStep("0.0.0.1")
	flow := a.ate.Traffic().NewFlow(name).
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint...).
		WithHeaders(ondatra.NewEthernetHeader(), hdr, innerIpv4Header).WithFrameRateFPS(10).WithFrameSize(300)

	return flow
}

// validateTrafficFlows validates traffic loss on tgn side and DUT incoming and outgoing counters
func (a *testArgs) validateTrafficFlows(t *testing.T, flows []*ondatra.Flow, drop bool, s_port string, d_port []string) {
	src_port := a.dut.Telemetry().Interface(a.dut.Port(t, s_port).Name())
	subintf1 := src_port.Subinterface(0)
	dutOutPktsBeforeTraffic := map[string]uint64{"ipv4": subintf1.Ipv4().Counters().InPkts().Get(t)}

	dutInPktsBeforeTraffic := make(map[string][]uint64)
	for _, dp := range d_port {
		dst_port := a.dut.Telemetry().Interface(a.dut.Port(t, dp).Name())
		subintf2 := dst_port.Subinterface(0)
		dutInPktsBeforeTraffic["ipv4"] = append(dutInPktsBeforeTraffic["ipv4"], subintf2.Ipv4().Counters().OutPkts().Get(t))
	}
	//aggregriate dst_port counter
	totalInPktsBeforeTraffic := map[string]uint64{"ipv4": 0}
	for _, data := range dutInPktsBeforeTraffic["ipv4"] {
		totalInPktsBeforeTraffic["ipv4"] = totalInPktsBeforeTraffic["ipv4"] + uint64(data)
	}

	a.ate.Traffic().Start(t, flows...)
	time.Sleep(60 * time.Second)
	a.ate.Traffic().Stop(t)

	// sleeping while DUT interface counters are updated
	time.Sleep(20 * time.Second)

	for _, f := range flows {
		ateTxPkts := map[string]uint64{"ipv4": a.ate.Telemetry().Flow(f.Name()).Counters().OutPkts().Get(t)}
		ateRxPkts := map[string]uint64{"ipv4": a.ate.Telemetry().Flow(f.Name()).Counters().InPkts().Get(t)}

		flowPath := a.ate.Telemetry().Flow(f.Name())
		got := flowPath.LossPct().Get(t)
		if drop {
			if got != 100 {
				t.Errorf("Traffic passing for flow %s got %g, want 100 percent loss", f.Name(), got)
			}
		} else {
			if got > 0 {
				t.Errorf("LossPct for flow %s got %g, want 0", f.Name(), got)
			}
		}

		if !drop {
			dutOutPktsAfterTraffic := map[string]uint64{"ipv4": subintf1.Ipv4().Counters().InPkts().Get(t)}
			dutInPktsAfterTraffic := make(map[string][]uint64)
			for _, dp := range d_port {
				dst_port := a.dut.Telemetry().Interface(a.dut.Port(t, dp).Name())
				subintf2 := dst_port.Subinterface(0)
				dutInPktsAfterTraffic["ipv4"] = append(dutInPktsAfterTraffic["ipv4"], subintf2.Ipv4().Counters().OutPkts().Get(t))
			}

			//aggregriate dst_port counter
			totalInPktsAfterTraffic := map[string]uint64{"ipv4": 0}
			for _, data := range dutInPktsAfterTraffic["ipv4"] {
				totalInPktsAfterTraffic["ipv4"] = totalInPktsAfterTraffic["ipv4"] + uint64(data)
			}

			for k := range dutInPktsAfterTraffic {
				if got, want := totalInPktsAfterTraffic[k]-totalInPktsBeforeTraffic[k], ateTxPkts[k]; got <= want {
					t.Errorf("Get less inPkts from telemetry: got %v, want >= %v", got, want)
				}
				if got, want := dutOutPktsAfterTraffic[k]-dutOutPktsBeforeTraffic[k], ateRxPkts[k]; got <= want {
					t.Errorf("Get less outPkts from telemetry: got %v, want >= %v", got, want)
				}
			}
		}
	}
}

// testTraffic generates traffic flow from source network to destination network via srcEndPoint to dstEndPoint
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint *ondatra.Interface, dstEndPoint []ondatra.Endpoint, drop bool) {
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
	innerIpv4Header.DstAddressRange().WithMin("201.1.0.2").WithCount(1).WithStep("0.0.0.1")

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
		got := flowPath.LossPct().Get(t)
		if drop {
			if got != 0 {
				t.Errorf("Traffic passing for flow %s got %g, want 100 percent loss", f.Name(), got)
			}
		} else {
			if got > 0 {
				t.Errorf("LossPct for flow %s got %g, want 0", f.Name(), got)
			}
		}
	}
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx     context.Context
	clientA *gribi.Client
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     *ondatra.ATETopology
}

func testBackupNHOPCase1(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a drop address
	args.clientA.NH(t, 10, "192.0.2.100", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", 1, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", 1, fluent.InstalledInRIB)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)

	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port8"})

	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)

	// validate traffic dropping on backup
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})

	// unshut all the links and validate traffic
	args.interfaceaction(t, "port2", true)
	args.interfaceaction(t, "port3", true)
	args.interfaceaction(t, "port4", true)
	args.interfaceaction(t, "port5", true)
	args.interfaceaction(t, "port6", true)
	args.interfaceaction(t, "port7", true)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	// removing default route pointing to Valid Path
	t.Log("remvoing default route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
}

func testBackupNHOPCase2(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a drop address
	args.clientA.NH(t, 10, "192.0.2.100", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", 1, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", 1, fluent.InstalledInRIB)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//delete backup path and validate no traffic loss
	args.clientA.NHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, instance, "replace", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "delete", fluent.InstalledInRIB)
	args.clientA.NH(t, 10, "192.0.2.100", instance, "", "delete", fluent.InstalledInRIB)

	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//add back backup path and validate no traffic loss
	args.clientA.NH(t, 10, "192.0.2.100", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "replace", fluent.InstalledInRIB)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	// removing default route pointing to Valid Path
	t.Log("remvoing default route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
}

func testBackupNHOPCase3(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a drop address
	args.clientA.NH(t, 10, "192.0.2.100", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", 1, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", 1, fluent.InstalledInRIB)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//delete backup path and shut primary interfaces and validate traffic drops
	args.clientA.NHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, instance, "replace", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "delete", fluent.InstalledInRIB)
	args.clientA.NH(t, 10, "192.0.2.100", instance, "", "delete", fluent.InstalledInRIB)
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})

	//add back backup path and validate traffic drops
	args.clientA.NH(t, 10, "192.0.2.100", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "replace", fluent.InstalledInRIB)
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})

	//restore the links
	args.interfaceaction(t, "port7", true)
	args.interfaceaction(t, "port6", true)
	args.interfaceaction(t, "port5", true)
	args.interfaceaction(t, "port4", true)
	args.interfaceaction(t, "port3", true)
	args.interfaceaction(t, "port2", true)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	// removing default route and bringing up the links
	t.Log("remvoing default route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
}

func testBackupNHOPCase4(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a drop address
	args.clientA.NH(t, 10, "192.0.2.100", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", 1, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", 1, fluent.InstalledInRIB)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//Modify Backup pointing to Different ID which is pointing to a different static rooute pointitng to DROP
	args.clientA.NH(t, 999, "220.220.220.220", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{999: 100}, instance, "replace", fluent.InstalledInRIB)

	//shut down primary path and validate traffic dropping
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})

	// bringing up the links
	args.interfaceaction(t, "port7", true)
	args.interfaceaction(t, "port6", true)
	args.interfaceaction(t, "port5", true)
	args.interfaceaction(t, "port4", true)
	args.interfaceaction(t, "port3", true)
	args.interfaceaction(t, "port2", true)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	// removing default route
	t.Log("remvoing default route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
}

func testBackupNHOPCase5(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to decap
	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", 1, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", 1, fluent.InstalledInRIB)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port8"})

	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)

	// validate traffic decap over backup path
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	// unshut all the links and validate traffic
	args.interfaceaction(t, "port2", true)
	args.interfaceaction(t, "port3", true)
	args.interfaceaction(t, "port4", true)
	args.interfaceaction(t, "port5", true)
	args.interfaceaction(t, "port6", true)
	args.interfaceaction(t, "port7", true)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	// removing default route and bringing up the links
	t.Log("remvoing default route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
}

func testBackupNHOPCase6(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a decap
	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 101, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	//shutdown primary path
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)

	//Validate traffic over backup is passing
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//flush all the entries
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	//Validate traffic dropping after deleting forwarding
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7"})

	//unshut links
	args.interfaceaction(t, "port2", true)
	args.interfaceaction(t, "port3", true)
	args.interfaceaction(t, "port4", true)
	args.interfaceaction(t, "port5", true)
	args.interfaceaction(t, "port6", true)
	args.interfaceaction(t, "port7", true)

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a decap
	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", 1, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", 1, fluent.InstalledInRIB)

	// validate traffic passing over primary path
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7"})

	//shutdown primary path
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)

	//Validate traffic over backup
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//flush the entries
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	//Validate traffic failing
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	args.interfaceaction(t, "port2", true)
	args.interfaceaction(t, "port3", true)
	args.interfaceaction(t, "port4", true)
	args.interfaceaction(t, "port5", true)
	args.interfaceaction(t, "port6", true)
	args.interfaceaction(t, "port7", true)
}

func testBackupNHOPCase7(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a drop address
	args.clientA.NH(t, 10, "192.0.2.100", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", 1, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", 1, fluent.InstalledInRIB)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//shutdown primary path
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)

	//Validate traffic over backup is failing
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})

	// Modify backup from pointing to a static route to a DECAP chain
	args.clientA.NH(t, 999, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{999: 100}, instance, "replace", fluent.InstalledInRIB)
	// validate traffic decap over backup path
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	// unshut all the links and validate traffic
	args.interfaceaction(t, "port2", true)
	args.interfaceaction(t, "port3", true)
	args.interfaceaction(t, "port4", true)
	args.interfaceaction(t, "port5", true)
	args.interfaceaction(t, "port6", true)
	args.interfaceaction(t, "port7", true)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	// removing default route and bringing up the links
	t.Log("remvoing default route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
}

func testBackupNHOPCase8(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a decap
	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 201, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Create REPAIRED INSTANCE POINTING TO THE SAME LEVEL1 VIPS
	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 200, 201, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 200, "REPAIRED", instance, "add", dstPfxCount, fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 35, 200: 65}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", 1, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", 1, fluent.InstalledInRIB)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	// removing default route and bringing up the links
	t.Log("remvoing default route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
}

func testBackupNHOPCase9(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// LEVEL 2
	// Creating a backup NHG with ID 101 and NH ID 10 pointing to a decap
	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 using backup NHG ID 101
	// PATH 1 NH ID 100, weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200, weight 15, VIP2 : 192.0.2.42
	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	// LEVEL 1
	// VIP1: NHG ID 1000
	//		- PATH1 NH ID 1000, weight 50, outgoing Port2
	//		- PATH2 NH ID 1100, weight 30, outgoing Port3
	//		- PATH3 NH ID 1200, weight 15, outgoing Port4
	//		- PATH4 NH ID 1300, weight  5, outgoing Port5
	// VIP2: NHG ID 2000
	//		- PATH1 NH ID 2000, weight 60, outgoing Port6
	//		- PATH2 NH ID 2100, weight 35, outgoing Port7
	//		- PATH3 NH ID 2200, weight  5, outgoing Port8
	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", 1, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", 1, fluent.InstalledInRIB)

	// get gribi contents
	getResponse1, err1 := args.clientA.Fluent(t).Get().AllNetworkInstances().WithAFT(fluent.IPv4).Send()
	getResponse2, err2 := args.clientA.Fluent(t).Get().AllNetworkInstances().WithAFT(fluent.AllAFTs).Send()
	getResponse3, err3 := args.clientA.Fluent(t).Get().AllNetworkInstances().WithAFT(fluent.NextHopGroup).Send()
	getResponse4, err4 := args.clientA.Fluent(t).Get().AllNetworkInstances().WithAFT(fluent.NextHop).Send()

	var prefixes []string
	if err1 != nil && err2 != nil && err3 != nil && err4 != nil {
		t.Errorf("Cannot Get")
	}

	entries1 := getResponse1.GetEntry()
	entries2 := getResponse2.GetEntry()
	entries3 := getResponse3.GetEntry()
	entries4 := getResponse4.GetEntry()

	fmt.Print(entries2, entries3, entries4)

	for _, entry := range entries1 {
		v := entry.Entry.(*spb.AFTEntry_Ipv4)
		if prefix := v.Ipv4.GetPrefix(); prefix != "" {
			prefixes = append(prefixes, prefix)
		}
	}
	var data []string
	data = append(data, "192.0.2.40/32", "192.0.2.42/32")
	ip := net.ParseIP(dstPfx)
	for i := 0; i < dstPfxCount; i++ {
		ip_v4 := ip.To4()
		data = append(data, ip_v4.String()+"/32")
		ip_v4[3]++
	}

	if diff := cmp.Diff(data, prefixes); diff != "" {
		t.Errorf("Prefixes differed (-want +got):\n%v", diff)
	}
}

func testBackupNHOPCase10(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// Verify the entry for 198.51.100.1/32 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()

	t.Log("going to program Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 198.51.100.1  0012.0100.0001 arpa")

	// LEVEL 2
	// Creating NHG ID 100 using backup NHG ID 101
	args.clientA.NH(t, 10, atePort8.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	args.clientA.NH(t, 100, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 100}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2"})

	//shutdown primary path port2 and switch to backup port8
	args.interfaceaction(t, "port2", false)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	t.Log("going to remove Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "no arp 198.51.100.1 0012.0100.0001 arpa")
	config.TextWithGNMI(args.ctx, t, args.dut, "interface HundredGigE0/0/0/1 arp learning disable")

	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//adding back port2 configurations
	args.interfaceaction(t, "port2", true)

	t.Logf("deleting all the IPV4 entries added")
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	args.clientA.NH(t, 3000, atePort8.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 300, 0, map[uint64]uint64{3000: 100}, instance, "add", fluent.InstalledInRIB)

	args.clientA.NH(t, 100, "193.0.2.1", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 300, map[uint64]uint64{100: 100}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 200, 0, map[uint64]uint64{2000: 100}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "193.0.2.1", "32", 200, instance, "", "add", 1, fluent.InstalledInRIB)

	args.clientA.IPv4(t, "193.0.2.1", "32", 200, instance, "", "delete", 1, fluent.InstalledInRIB)
	args.clientA.NHG(t, 200, 0, map[uint64]uint64{2000: 100}, instance, "delete", fluent.InstalledInRIB)
	args.clientA.NH(t, 2000, atePort2.IPv4, instance, "", "delete", fluent.InstalledInRIB)

	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})
}

func testBackupNHOPCase11(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
	args.clientA.BecomeLeader(t)
	if _, err := args.clientA.Fluent(t).Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send(); err != nil {
		t.Fatalf("could not remove all entries from server, got: %v", err)
	}

	// Verify the entry for 198.51.100.1/32 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()

	t.Log("going to program Static ARP different from Ixia ")
	config.TextWithGNMI(args.ctx, t, args.dut, "arp 198.51.100.1  0012.0100.0001 arpa")

	// LEVEL 2
	// Creating NHG ID 100 using backup NHG ID 101 (bkhgIndex_2)
	args.clientA.NH(t, 10, atePort8.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	args.clientA.NH(t, 100, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 50, 200: 50}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3"})

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	args.interfaceaction(t, "port2", false)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port3"})

	args.interfaceaction(t, "port3", false)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//bring up the shutlinks
	args.interfaceaction(t, "port2", true)
	args.interfaceaction(t, "port3", true)
}

/*
 * Removing the Backup Path for a prefix when primary
 * links are Up. The traffic shouldnt be impacted
 */
func testIPv4BackUpRemoveBackup(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
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

	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40/", "32", 1000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}
	args.clientA.NHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)

	// validate traffic passing via primary links
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7"})
}

/* Add a backup path when primary links are
 * down. Traffic should start taking the backup path
 */
func testIPv4BackUpAddBkNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
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

	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
	}
	// validate traffic passing successfulling after decap via backup link
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//unshut interfaces
	interface_names = []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, true)
	}
}

/*
 * Remove and re-add Backup when Primary is down
 * Traffic should be impacted during remove and
 * should be recovered during Re-Add
 */
func testIPv4BackUpToggleBkNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
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

	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
	}
	// validate traffic passing successfulling after decap via ISIS route
	args.clientA.NHG(t, 100, 0, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)

	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})

	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)

	time.Sleep(time.Minute)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//unshut all the interface
	interface_names = []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, true)
	}
}

func testIPv4BackUpShutSite1(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
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

	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
	}
	// validate traffic passing successfulling via primary Site 2
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port6", "port7"})

	//unshut interfaces
	interface_names = []string{"port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, true)
	}
}

/* Change the Backup NHG index from a Decap NHG to
 * Drop NHG. The primary links should be shut.
 * Packets should be dropped
 */
func testIPv4BackUpDecapToDrop(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
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

	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint, false)
		args.interfaceaction(t, intf, false)
	}
	// validate traffic passing successfulling after decap via backup path
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	args.clientA.NH(t, 11, "192.0.2.100", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 102, 0, map[uint64]uint64{11: 100}, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 102, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)

	// validate traffic dropping completely on the backup path
	time.Sleep(time.Minute)
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})

	//unshut interfaces
	interface_names = []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint, false)
		args.interfaceaction(t, intf, true)
	}
}

/* Change the Backup NHG index from a Drop NHG to
 * Decap NHG. The primary links should be shut.
 * Packets should take the backup path
 */
func testIPv4BackUpDropToDecap(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
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

	args.clientA.NH(t, 11, "192.0.2.100", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 102, 0, map[uint64]uint64{11: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 102, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint, false)
		args.interfaceaction(t, intf, false)
	}
	// validate traffic dropping on the backup path
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, true, "port1", []string{"port8"})

	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	// validate traffic passing successfulling after decap via ISIS route
	time.Sleep(time.Minute)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//unshut interfaces
	interface_names = []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		testTraffic(t, args.ate, args.top, srcEndPoint, updated_dstEndPoint, false)
		args.interfaceaction(t, intf, true)
	}
}

/* Change the Backup NHG index from a Decap NHG to
 * another Decap NHG. The primary links should be shut.
 * Packets should be forwarded after decapped
 */
func testIPv4BackUpModifyDecapNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
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

	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
	}

	args.clientA.NH(t, 11, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 102, 0, map[uint64]uint64{11: 100}, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 102, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)

	// validate traffic passing successfulling after decap via ISIS route
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//unshut interfaces
	interface_names = []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, true)
	}
}

func testIPv4BackUpMultiplePrefixes(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
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

	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 110, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 111, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 112, 100, map[uint64]uint64{110: 85, 111: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 112, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 1001, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1002, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1003, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1004, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1001: 50, 1002: 30, 1003: 15, 1004: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2001, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2002, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2001: 60, 2002: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx1)

	// LEVEL 2

	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.clientA.NH(t, 20, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 200, 0, map[uint64]uint64{20: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 210, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 211, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 212, 200, map[uint64]uint64{210: 85, 211: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx1, dstPfxMask, 212, "TE", instance, "add", dstPfxCount1, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 3001, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 3002, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 3003, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 3004, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 3000, 0, map[uint64]uint64{3001: 50, 3002: 30, 3003: 15, 3004: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 3000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 4001, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 4002, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 4000, 0, map[uint64]uint64{4001: 60, 4002: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 4000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
	}
	// validate traffic passing successfulling after decap via ISIS route
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//unshut interface
	interface_names = []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, true)
	}
}

func testIPv4BackUpMultipleVRF(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
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

	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 110, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 111, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 112, 100, map[uint64]uint64{110: 85, 111: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 112, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 1001, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1002, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1003, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1004, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1001: 50, 1002: 30, 1003: 15, 1004: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2001, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2002, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2001: 60, 2002: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx1)

	// LEVEL 2

	// Creating a backup NHG with ID 101 (bkhgIndex_2)
	// NH ID 10 (nhbIndex_2_1)

	args.clientA.NH(t, 20, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 200, 0, map[uint64]uint64{20: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 110, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 111, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 212, 200, map[uint64]uint64{110: 85, 111: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx1, dstPfxMask, 212, "VRF1", instance, "add", dstPfxCount1, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 1001, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1002, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1003, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1004, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1001: 50, 1002: 30, 1003: 15, 1004: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2001, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2002, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2001: 60, 2002: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
	}
	// validate traffic passing successfulling after decap via ISIS route
	time.Sleep(time.Minute)
	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//unshut interfaces
	interface_names = []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, true)
	}
}

func testIPv4BackUpFlapBGPISIS(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
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

	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
	}
	// BGP /ISIS peer is in port 8. So flap port 8
	args.interfaceaction(t, "port8", false)
	args.interfaceaction(t, "port8", true)

	// validate traffic passing successfulling via primary Site 2
	time.Sleep(time.Minute)

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//unshut interfaces
	interface_names = []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, true)
	}
}

func testIPv4MultipleNHG(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
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

	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)

	ip := net.ParseIP(dstPfx)
	ip = ip.To4()

	for i := 501; i < 1001; i++ {
		args.clientA.NHG(t, uint64(i), 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
		ipv4 := fluent.IPv4Entry().WithPrefix(ip.String() + "/" + dstPfxMask).WithNetworkInstance("TE").WithNextHopGroup(uint64(i)).WithNextHopGroupNetworkInstance(instance)
		ip[3]++
		args.clientA.Fluent(t).Modify().AddEntry(t, ipv4)
	}

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

	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", 1, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", 1, fluent.InstalledInRIB)

	// adding default route pointing to Valid Path
	t.Log("Adding a defult route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	args.interfaceaction(t, "port7", false)
	args.interfaceaction(t, "port6", false)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port8"})

	args.interfaceaction(t, "port5", false)
	args.interfaceaction(t, "port4", false)
	args.interfaceaction(t, "port3", false)
	args.interfaceaction(t, "port2", false)

	// validate traffic decap over backup path
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	// unshut all the links and validate traffic
	args.interfaceaction(t, "port2", true)
	args.interfaceaction(t, "port3", true)
	args.interfaceaction(t, "port4", true)
	args.interfaceaction(t, "port5", true)
	args.interfaceaction(t, "port6", true)
	args.interfaceaction(t, "port7", true)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"})

	// removing default route and bringing up the links
	t.Log("remvoing default route 0.0.0.0/0 as well pointing to a Valid NHOP ")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.40")
	config.TextWithGNMI(args.ctx, t, args.dut, "no router static address-family ipv4 unicast 0.0.0.0/0 192.0.2.42")
}

func testIPv4BackUpLCOIR(ctx context.Context, t *testing.T, args *testArgs) {

	// Elect ClientA as leader and flush all the past entries
	t.Logf("an IPv4Entry for %s pointing via gRIBI-A", dstPfx)
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

	args.clientA.NH(t, 10, "decap", instance, instance, "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 101, 0, map[uint64]uint64{10: 100}, instance, "add", fluent.InstalledInRIB)

	// Creating NHG ID 100 (nhgIndex_2_1) using backup NHG ID 101 (bkhgIndex_2)
	// PATH 1 NH ID 100 (nhIndex_2_1), weight 85, VIP1 : 192.0.2.40
	// PATH 2 NH ID 200 (nhIndex_2_2), weight 15, VIP2 : 192.0.2.42

	args.clientA.NH(t, 100, "192.0.2.40", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 200, "192.0.2.42", instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 100, 101, map[uint64]uint64{100: 85, 200: 15}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, dstPfx, dstPfxMask, 100, "TE", instance, "add", dstPfxCount, fluent.InstalledInRIB)

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

	args.clientA.NH(t, 1000, atePort2.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1100, atePort3.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1200, atePort4.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 1300, atePort5.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 1000, 0, map[uint64]uint64{1000: 50, 1100: 30, 1200: 15, 1300: 5}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.40", "32", 1000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	args.clientA.NH(t, 2000, atePort6.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NH(t, 2100, atePort7.IPv4, instance, "", "add", fluent.InstalledInRIB)
	args.clientA.NHG(t, 2000, 0, map[uint64]uint64{2000: 60, 2100: 40}, instance, "add", fluent.InstalledInRIB)
	args.clientA.IPv4(t, "192.0.2.42", "32", 2000, instance, "", "add", dstPfxCount, fluent.InstalledInRIB)

	// Verify the entry for 198.51.100.0/24 is active through Traffic.
	srcEndPoint := args.top.Interfaces()[atePort1.Name]
	dstEndPoint := args.top.Interfaces()
	updated_dstEndPoint := []ondatra.Endpoint{}
	for intf, intf_data := range dstEndPoint {
		if "atePort1" != intf {
			updated_dstEndPoint = append(updated_dstEndPoint, intf_data)
		}
	}

	//shutdown primary path one by one (destination end) and validate traffic switching to backup (port8)
	interface_names := []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, false)
	}
	// BGP /ISIS peer is in port 8. So flap port 8
	util.ReloadDUT(t, args.dut)
	// validate traffic passing successfulling via primary Site 2
	time.Sleep(time.Minute)

	bgp_flow := args.createFlow("BaseFlow_BGP", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_bgp, innerdstPfxCount_bgp)
	isis_flow := args.createFlow("BaseFlow_ISIS", srcEndPoint, updated_dstEndPoint, innerdstPfxMin_isis, innerdstPfxCount_isis)
	flows := []*ondatra.Flow{}
	flows = append(flows, bgp_flow, isis_flow)
	args.validateTrafficFlows(t, flows, false, "port1", []string{"port8"})

	//unshut interfaces
	interface_names = []string{"port7", "port6", "port5", "port4", "port3", "port2"}
	for _, intf := range interface_names {
		args.interfaceaction(t, intf, true)
	}
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

	t.Log("Match IPinIP protocol to VRF10")
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
	addPrototoAte(t, top)

	test := []struct {
		name string
		desc string
		fn   func(ctx context.Context, t *testing.T, args *testArgs)
	}{
		{
			name: "Backup pointing to route",
			desc: "Base usecase with 2 NHOP Groups - Backup Pointing to route (drop)",
			fn:   testBackupNHOPCase1,
		},
		{
			name: "Delete add backup",
			desc: "Deleting and Adding Back the Backup has no impact on traffic",
			fn:   testBackupNHOPCase2,
		},
		{
			name: "Add backup after primary down",
			desc: "Add the backup - After Primary links are down Traffic continues ot DROP",
			fn:   testBackupNHOPCase3,
		},
		{
			name: "Update backup to another ID",
			desc: "Modify Backup pointing to Different ID which is pointing to a different static rooute pointitng to DROP",
			fn:   testBackupNHOPCase4,
		},
		{
			name: "Backup pointing to decap",
			desc: "Base usecase with 2 NHOP Groups - - Backup Pointing to Decap",
			fn:   testBackupNHOPCase5,
		},
		{
			name: "flush forwarding chain with and without backup NH",
			desc: "add testcase to flush forwarding chain with backup NHG only and forwarding chain with backup NHG",
			fn:   testBackupNHOPCase6,
		},
		{
			name: "Backup change from static to decap",
			desc: "While Primary Paths are down Modify the Backup from poiniting to a static route to a DECAP chain - Traffic resumes after Decap",
			fn:   testBackupNHOPCase7,
		},
		{
			name: "Multiple NW Instance with different NHG, same NH and different NHG backup",
			desc: "Multiple NW Instances (VRF's ) pointing to different NHG but same NH Entry but different NHG Backup",
			fn:   testBackupNHOPCase8,
		},
		{
			name: "Get function validation",
			desc: "add decap NH and related forwarding chain and validate them using GET function",
			fn:   testBackupNHOPCase9,
		},
		{
			name: "IPv4BackUpSingleNH",
			desc: "Single NH Ensure that backup NextHopGroup entries are honoured in gRIBI for NHGs containing a single NH",
			fn:   testBackupNHOPCase10,
		},
		{
			name: "IPv4BackUpMultiNH",
			desc: "Multiple NHBackup NHG: Multiple NH Ensure that backup NHGs are honoured with NextHopGroup entries containing",
			fn:   testBackupNHOPCase11,
		},
		{
			name: "IPv4BackUpRemoveBackup",
			desc: "Set primary and backup path with gribi and send traffic. Delete the backup NHG and check if impacts traffic",
			fn:   testIPv4BackUpRemoveBackup,
		},
		{
			name: "IPv4BackUpAddBkNHG",
			desc: "Set primary path with gribi and shutdown all the primary path. Now add the backup NHG and  validate traffic ",
			fn:   testIPv4BackUpAddBkNHG,
		},
		{
			name: "IPv4BackUpToggleBkNHG",
			desc: "Set primary and backup path with gribi and shutdown all the primary path. Now remove,readd the backup NHG and validate traffic ",
			fn:   testIPv4BackUpToggleBkNHG,
		},
		{
			name: "IPv4BackUpDecapToDrop",
			desc: "Shutdown all the primary path and modify Backup NHG from Drop to Decap and validate traffic ",
			fn:   testIPv4BackUpDecapToDrop,
		},
		{
			name: "IPv4BackUpDropToDecap",
			desc: "Shutdown all the primary path and modify Backup NHG from Decap to Drop and validate traffic ",
			fn:   testIPv4BackUpDropToDecap,
		},
		{
			name: "IPv4BackUpShutSite1",
			desc: "Shutdown the primary path for 1 Site  and validate traffic is going through another primary and not backup ",
			fn:   testIPv4BackUpShutSite1,
		},
		{
			name: "IPv4BackUpModifyDecapNHG",
			desc: "Shutdown all the primary path and modify Backup NHG from  Decap NHG 101 to Decap NHG 102 and validate traffic ",
			fn:   testIPv4BackUpModifyDecapNHG,
		},
		{
			name: "IPv4BackUpMultiplePrefixes",
			desc: "Have same primary and backup links for 2 prefixes with different NHG IDs and validate backup traffic ",
			fn:   testIPv4BackUpMultiplePrefixes,
		},
		{
			name: "IPv4BackUpMultipleVRF",
			desc: "Have same primary and backup links for 2 prefixes with different NHG IDs in different VRFs and validate backup traffic ",
			fn:   testIPv4BackUpMultipleVRF,
		},
		{
			name: "IPv4BackUpFlapBGPISIS",
			desc: "Have same primary and backup links for 2 prefixes with different NHG IDs in different VRFs and validate backup traffic ",
			fn:   testIPv4BackUpFlapBGPISIS,
		},
		{
			name: "IPv4BackupLCOIR",
			desc: "Have Primary and backup configured on same LC and do a shut of primary. Followed by LC reload",
			fn:   testIPv4BackUpLCOIR,
		},
		// {
		// 	name: "IPv4MultipleNHG",
		// 	desc: "Have same primary and backup decap with multiple nhg",
		// 	fn:   testIPv4MultipleNHG,
		// },
	}
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)

			// Configure the gRIBI client clientA
			clientA := gribi.Client{
				DUT:                   dut,
				FibACK:                false,
				Persistence:           true,
				InitialElectionIDLow:  10,
				InitialElectionIDHigh: 0,
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
