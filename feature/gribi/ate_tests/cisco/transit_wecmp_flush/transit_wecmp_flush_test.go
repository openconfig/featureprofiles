// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transit_wecmp_flush

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/gribi/ocutils"
	"github.com/openconfig/featureprofiles/internal/gribi/util"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/gribigo/server"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
)

var (
	ixiaTopology = make(map[string]*ondatra.ATETopology)
)

func getIXIATopology(t *testing.T, ateName string) *ondatra.ATETopology {
	topo, ok := ixiaTopology[ateName]
	if !ok {
		ate := ondatra.ATE(t, ateName)
		topo = ate.Topology().New()
		generateBaseScenario(t, ate, topo)
		topo.Push(t)
		ixiaTopology[ateName] = topo
	}
	return topo
}

func generateBaseScenario(t *testing.T, ate *ondatra.ATEDevice, topoobj *ondatra.ATETopology) {
	for _, p := range ate.Device.Ports() {
		intf := topoobj.AddInterface(p.Name())
		intf.WithPort(ate.Port(t, p.ID()))
		for i := 0; i < 9; i++ {
			if fmt.Sprintf("1/%d", i+1) == p.Name() {
				intf.IPv4().WithAddress(fmt.Sprintf("100.%d.1.2/24", 120+i)).WithDefaultGateway(fmt.Sprintf("100.%d.1.1", 120+i))
			}
		}
	}
	addNetworkAndProtocolsToAte(t, ate, topoobj)
}

func addNetworkAndProtocolsToAte(t *testing.T, ate *ondatra.ATEDevice, topo *ondatra.ATETopology) {
	//Add prefixes/networks on ports
	scale := uint32(10)
	ocutils.AddIpv4Network(t, topo, "1/1", "network101", "101.1.1.1/32", scale)
	ocutils.AddIpv4Network(t, topo, "1/2", "network102", "102.1.1.1/32", scale)
	//Configure ISIS, BGP on TGN
	ocutils.AddAteISISL2(t, topo, "1/1", "490001", "isis_network1", 20, "120.1.1.1/32", scale)
	ocutils.AddAteISISL2(t, topo, "1/2", "490002", "isis_network2", 20, "121.1.1.1/32", scale)
	ocutils.AddAteEBGPPeer(t, topo, "1/1", "100.120.1.1", 64001, "bgp_network", "100.120.0.2", "130.1.1.1/32", scale, false)
	ocutils.AddAteEBGPPeer(t, topo, "1/2", "100.121.1.1", 64001, "bgp_network", "100.121.0.2", "131.1.1.1/32", scale, false)
	//Configure loopbacks for BGP to use as source addresses
	ocutils.AddLoopback(t, topo, "1/1", "11.11.11.1/32")
	ocutils.AddLoopback(t, topo, "1/2", "12.12.12.1/32")
	//BGP instance for traffic over gRIBI transit forwarding entries
	//BGP uses DSCP48 for control traffic. Router needs to be configured to handle DSCP48 accordingly.
	ocutils.AddAteEBGPPeer(t, topo, "1/1", "12.12.12.1", 64001, "bgp_transit_network", "100.121.0.2", "11.11.11.1/32", 1, true)
	ocutils.AddAteEBGPPeer(t, topo, "1/2", "11.11.11.1", 64002, "bgp_transit_network", "100.122.0.2", "12.12.12.1/32", 1, true)
}

func getBaseFlow(t *testing.T, atePorts map[string]*ondatra.Interface, ate *ondatra.ATEDevice, flowName string, vrf ...string) *ondatra.Flow {
	flow := ate.Traffic().NewFlow(flowName)
	t.Log("Setting up base flow...")
	srcPort := "1/1"
	dstPort := "1/2"

	flow.WithSrcEndpoints(atePorts[srcPort])
	flow.WithDstEndpoints(atePorts[dstPort])

	ethheader := ondatra.NewEthernetHeader()
	ethheader.WithSrcAddress("00:11:01:00:00:01")
	ethheader.WithDstAddress("00:01:00:02:00:00")

	ipheader1 := ondatra.NewIPv4Header()
	ipheader1.WithSrcAddress("100.1.0.2")
	ipheader1.WithDstAddress("11.11.11.11")

	ipheader2 := ondatra.NewIPv4Header()
	ipheader2.WithSrcAddress("200.1.0.2")
	ipheader2.DstAddressRange().WithMin("201.1.0.2").WithCount(1000).WithStep("0.0.0.1")

	flow.WithFrameRateFPS(1000)
	flow.WithFrameSize(300)

	if len(vrf) > 0 {
		flow.WithHeaders(ethheader, ipheader1)
	} else {
		flow.WithHeaders(ethheader, ipheader1, ipheader2)
	}

	return flow
}

func getScaleFlow(t *testing.T, atePorts map[string]*ondatra.Interface, ate *ondatra.ATEDevice, flowName string, scale int, vrf ...string) *ondatra.Flow {
	flow := ate.Traffic().NewFlow(flowName)
	t.Log("Setting up scale flow...")
	flow.WithSrcEndpoints(atePorts["1/1"])

	t.Log("Extending to multiple receiver ports...")
	rxPorts := []ondatra.Endpoint{}
	for i := 1; i < 9; i++ {
		rxPorts = append(rxPorts, atePorts[fmt.Sprintf("1/%d", i)])
	}
	flow.WithDstEndpoints(rxPorts...)

	ethheader := ondatra.NewEthernetHeader()
	ethheader.WithSrcAddress("00:11:01:00:00:01")
	ethheader.WithDstAddress("00:01:00:02:00:00")

	ipheader1 := ondatra.NewIPv4Header()
	ipheader1.WithSrcAddress("100.1.0.2")
	ipheader1.DstAddressRange().WithMin("11.11.11.0").WithCount(uint32(scale)).WithStep("0.0.0.1")

	ipheader2 := ondatra.NewIPv4Header()
	ipheader2.WithSrcAddress("200.1.0.2")
	ipheader2.DstAddressRange().WithMin("201.1.0.2").WithCount(1000).WithStep("0.0.0.1")

	flow.WithFrameRateFPS(1000)
	flow.WithFrameSize(300)

	if len(vrf) > 0 {
		flow.WithHeaders(ethheader, ipheader1)
	} else {
		flow.WithHeaders(ethheader, ipheader1, ipheader2)
	}

	return flow
}

func performATEAction(t *testing.T, ateName string, scale int, expectPass bool, threshold ...float64) {
	ate := ondatra.ATE(t, ateName)
	topology := getIXIATopology(t, ateName)
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	scaleflow := getScaleFlow(t, portMaps, ate, "IPinIPWithScale", scale)
	ate.Traffic().Start(t, scaleflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats, threshold...)

	if trafficPass == expectPass {
		t.Log("Traffic works as expected")
	} else {
		t.Error("Traffic doesn't work as expected")
	}
}

func performATEActionForMultipleFlows(t *testing.T, ateName string, expectPass bool, threshold float64, flow ...*ondatra.Flow) {
	ate := ondatra.ATE(t, ateName)
	topology := getIXIATopology(t, ateName)
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	ate.Traffic().Start(t, flow...)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
	t.Log("Packets transmitted by ports: ", ate.Telemetry().InterfaceAny().Counters().OutPkts().Get(t))
	t.Log("Packets received by ports: ", ate.Telemetry().InterfaceAny().Counters().InPkts().Get(t))
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats, threshold)

	if trafficPass == expectPass {
		t.Log("Traffic works as expected")
	} else {
		t.Error("Traffic doesn't work as expected")
	}
}

func getDSCPFlow(t *testing.T, atePorts map[string]*ondatra.Interface, ate *ondatra.ATEDevice, flowName string, scale int, dscp uint8, dstAddress string, rxPort string) *ondatra.Flow {
	flow := ate.Traffic().NewFlow(flowName)
	t.Log("Setting up flow -> ", flowName)
	flow.WithSrcEndpoints(atePorts["1/1"])
	flow.WithDstEndpoints(atePorts[rxPort])

	ethheader := ondatra.NewEthernetHeader()
	ethheader.WithSrcAddress("00:11:01:00:00:01")
	ethheader.WithDstAddress("00:01:00:02:00:00")

	ipheader1 := ondatra.NewIPv4Header()
	ipheader1.WithSrcAddress("100.1.0.2").WithDSCP(dscp)
	ipheader1.DstAddressRange().WithMin(dstAddress).WithCount(uint32(scale)).WithStep("0.0.0.1")

	ipheader2 := ondatra.NewIPv4Header()
	ipheader2.WithSrcAddress("200.1.0.2")
	ipheader2.DstAddressRange().WithMin("201.1.0.2").WithCount(1000).WithStep("0.0.0.1")

	flow.WithFrameRateFPS(100)
	flow.WithFrameSize(1024)

	flow.WithHeaders(ethheader, ipheader1, ipheader2)

	return flow
}

func checkTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, flow_duration time.Duration, flow ...*ondatra.Flow) {
	ate.Traffic().Start(t, flow...)
	defer ate.Traffic().Stop(t)

	time.Sleep(flow_duration * time.Second)

	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

type testArgs struct {
	ctx      context.Context
	c        *gribi.Client
	dut      *ondatra.DUTDevice
	ate      *ondatra.ATEDevice
	topology *ondatra.ATETopology
	fluentC  *fluent.GRIBIClient
	elecLow  uint64
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func testCD2ConnectedNHIP(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	portMaps := args.topology.Interfaces()

	args.topology.StartProtocols(t)
	defer args.topology.StopProtocols(t)

	baseflow := getBaseFlow(t, portMaps, args.ate, "IPinIPConnected")
	args.ate.Traffic().Start(t, baseflow)
	defer args.ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)

	stats := args.ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {

		t.Log("There is no traffic loss.")
	}
	// dut1.Telemetry().NetworkInstance().Afts().Ipv4Entry().Get(t)
}

func testCD2RecursiveNonConnectedNHOP(t *testing.T, args *testArgs) {
	ops := []func(){

		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.129.1.2"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.11", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(20, 99))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 7; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	t.Log("going to start Ixia protocols to bring up dynamic arp entry and start traffic  ")

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	baseflow := getBaseFlow(t, portMaps, ate, "IPinIPUnConnected")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(45 * time.Second)
	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Log("There is stream failing as expected since the NHOP is non connected:", strings.Join(lossStream, ","))
	} else {
		t.Error("There is no traffic loss.")
	}
	// dut1.Telemetry().NetworkInstance().Afts().Ipv4Entry().Get(t)
}

// Transit-46 ADD same IPv4 Entry verify no traffic impact
func testAddIPv4EntryTrafficCheck(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Start Traffic
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	ate.Traffic().Start(t, baseflow)

	// Add same ipv4 entry
	args.dut.GRIBI().SynchElectionID(t, true)
	c2 := args.dut.GRIBI().MasterClient(t)
	ops2 := []func(){
		func() {
			c2.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}
	res2 := util.DoModifyOps(args.fluentC, t, ops2, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(4); i < 5; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-47 REPLACE same IPv4 Entry verify no traffic impact
func testReplaceIPv4EntryTrafficCheck(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Start Traffic
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	ate.Traffic().Start(t, baseflow)

	// Replace same ipv4 entry
	args.dut.GRIBI().SynchElectionID(t, true)
	c2 := args.dut.GRIBI().MasterClient(t)
	ops2 := []func(){
		func() {
			c2.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}
	res2 := util.DoModifyOps(args.fluentC, t, ops2, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(4); i < 5; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-48 ADD same NHG verify no traffic impact
func testAddNHGTrafficCheck(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Start Traffic
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	ate.Traffic().Start(t, baseflow)

	// Add same NHG entry
	args.dut.GRIBI().SynchElectionID(t, true)
	c2 := args.dut.GRIBI().MasterClient(t)
	ops2 := []func(){
		func() {
			c2.Modify().AddEntry(t,
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res2 := util.DoModifyOps(args.fluentC, t, ops2, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(4); i < 5; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-49 REPLACE same NHG verify no traffic impact
func testReplaceNHGTrafficCheck(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Start Traffic
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	ate.Traffic().Start(t, baseflow)

	// Replace same NHG entry
	args.dut.GRIBI().SynchElectionID(t, true)
	c2 := args.dut.GRIBI().MasterClient(t)
	ops2 := []func(){
		func() {
			c2.Modify().ReplaceEntry(t,
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res2 := util.DoModifyOps(args.fluentC, t, ops2, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(4); i < 5; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-50 ADD same NH verify no traffic impact
func testAddNHTrafficCheck(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Start Traffic
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	ate.Traffic().Start(t, baseflow)

	// Add same NH entry
	args.dut.GRIBI().SynchElectionID(t, true)
	c2 := args.dut.GRIBI().MasterClient(t)
	ops2 := []func(){
		func() {
			c2.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
			)
		},
	}
	res2 := util.DoModifyOps(args.fluentC, t, ops2, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(4); i < 5; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-51 REPLACE same NH verify no traffic impact
func testReplaceNHTrafficCheck(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Start Traffic
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	ate.Traffic().Start(t, baseflow)

	// Replace same NH entry
	args.dut.GRIBI().SynchElectionID(t, true)
	c2 := args.dut.GRIBI().MasterClient(t)
	ops2 := []func(){
		func() {
			c2.Modify().ReplaceEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
			)
		},
	}
	res2 := util.DoModifyOps(args.fluentC, t, ops2, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(4); i < 5; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

func testCD2SingleRecursion(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

	// dut1.Telemetry().NetworkInstance().Afts().Ipv4Entry().Get(t)

}

func testCD2DoubleRecursion(t *testing.T, args *testArgs) {

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	performATEAction(t, "ate", 1000, true)
}

// Transit-34 REPLACE: default VRF IPv4 Entry with single path NHG+NH in default vrf
func testReplaceDefaultIPv4EntrySinglePath(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.3"),
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Add New NHG
	args.dut.GRIBI().SynchElectionID(t, true)
	elecLow2, _ := args.dut.GRIBI().GetElectionID(t)
	c2 := args.dut.GRIBI().MasterClient(t)

	ops2 := []func(){
		func() {
			c2.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(4).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(12).AddNextHop(4, 15),
			)
		},
	}
	res2 := util.DoModifyOps(c2, t, ops2, fluent.InstalledInRIB, false, elecLow2+1)

	for i := uint64(4); i < 6; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Replace VRF IPv4 Entry Pointing to different NHG
	args.dut.GRIBI().SynchElectionID(t, true)
	elecLow3, _ := args.dut.GRIBI().GetElectionID(t)
	c3 := args.dut.GRIBI().MasterClient(t)

	ops3 := []func(){
		func() {
			c3.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("11.11.11.11/32").WithNextHopGroup(12).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}
	res3 := util.DoModifyOps(c3, t, ops3, fluent.InstalledInRIB, false, elecLow3+1)

	chk.HasResult(t, res3, fluent.OperationResult().
		WithOperationID(6).
		WithProgrammingResult(fluent.InstalledInRIB).
		AsResult(),
	)

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP", "default")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

}

// Transit-38 DELETE: VRF IPv4 Entry with single path NHG+NH in default vrf
func testDeleteVRFIPv4EntrySinglePath(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
	}
	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

	// Delete Entry
	args.dut.GRIBI().SynchElectionID(t, true)
	elecLow2, _ := args.dut.GRIBI().GetElectionID(t)
	c2 := args.dut.GRIBI().MasterClient(t)
	ops2 := []func(){
		func() {
			args.fluentC.Modify().DeleteEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}
	res2 := util.DoModifyOps(c2, t, ops2, fluent.InstalledInRIB, false, elecLow2+1)

	for i := uint64(4); i < 7; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

}

// Transit-42 DELETE: default VRF IPv4 Entry with single path NHG+NH in default vrf
func testDeleteDefaultIPv4EntrySinglePath(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(6).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("11.11.11.11/32").WithNextHopGroup(16).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(16).AddNextHop(6, 15),
			)
		},
	}
	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP", "default")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

	// Delete Entry
	args.dut.GRIBI().SynchElectionID(t, true)
	elecLow2, _ := args.dut.GRIBI().GetElectionID(t)
	c2 := args.dut.GRIBI().MasterClient(t)
	ops2 := []func(){
		func() {
			args.fluentC.Modify().DeleteEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(6),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(16),
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("11.11.11.11/32").WithNextHopGroup(16).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}
	res2 := util.DoModifyOps(c2, t, ops2, fluent.InstalledInRIB, false, elecLow2+1)

	for i := uint64(4); i < 7; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

}

//Transit TC 066 - Two prefixes with NHGs with backup pointing to the each other's NHG
func testTwoPrefixesWithSameSetOfPrimaryAndBackup(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(4).WithIPAddress("100.122.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(14).WithBackupNHG(11).AddNextHop(4, 15),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.11.11.12/32").WithNextHopGroup(14).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).WithBackupNHG(14).AddNextHop(3, 15))
		},
	}
	results := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < uint64(len(results)-2); i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	dscp16Flow := getDSCPFlow(t, portMaps, ate, "DSCP16", 1, 16, "12.11.11.12", "1/3")
	dscp10Flow := getDSCPFlow(t, portMaps, ate, "DSCP10", 1, 10, "12.11.11.11", "1/2")

	checkTrafficFlows(t, ate, 60, dscp16Flow, dscp10Flow)
}

//Transit TC 067 - Same forwarding entries across multiple vrfs
func testSameForwardingEntriesAcrossMultipleVrfs(t *testing.T, args *testArgs) {
	elecLow, _ := args.dut.GRIBI().GetElectionID(t)

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(4).WithIPAddress("100.122.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(14).WithBackupNHG(11).AddNextHop(4, 15),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.11.11.12/32").WithNextHopGroup(14).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
		func() {
			//Add previously used prefixes in a different vrf
			args.fluentC.Modify().AddEntry(t, fluent.IPv4Entry().WithNetworkInstance("VRF1").WithPrefix("12.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.IPv4Entry().WithNetworkInstance("VRF1").WithPrefix("12.11.11.12/32").WithNextHopGroup(14).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}
	results := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, elecLow+1)

	for i := uint64(1); i < uint64(len(results)-2); i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	dscp16Flow_vrf_TE := getDSCPFlow(t, portMaps, ate, "DSCP16_vrf_TE", 1, 16, "12.11.11.12", "1/3")
	dscp10Flow_vrf_TE := getDSCPFlow(t, portMaps, ate, "DSCP10_vrf_TE", 1, 10, "12.11.11.11", "1/2")
	dscp18Flow1_vrf_VRF1 := getDSCPFlow(t, portMaps, ate, "DSCP16_vrf_VRF1", 1, 18, "12.11.11.12", "1/3")
	dscp18Flow2_vrf_VRF1 := getDSCPFlow(t, portMaps, ate, "DSCP10_vrf_VRF1", 1, 18, "12.11.11.11", "1/2")

	checkTrafficFlows(t, ate, 60, dscp16Flow_vrf_TE, dscp10Flow_vrf_TE, dscp18Flow1_vrf_VRF1, dscp18Flow2_vrf_VRF1)
}

// Transit-11: Next Hop resoultion with interface in different VRF of NH_network_instance
func testNHInterfaceInDifferentVRF(t *testing.T, args *testArgs) {
	t.Log("Testcase: Next Hop resoultion with interface in different VRF of NH_network_instance")

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121").WithNextHopNetworkInstance("TE"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Correct the related NH and verify traffic
	ops = []func(){
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
			)
		},
	}

	res = util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+2)

	chk.HasResult(t, res, fluent.OperationResult().
		WithOperationID(1015).
		WithProgrammingResult(fluent.InstalledInRIB).
		AsResult(),
	)

	performATEAction(t, "ate", 1000, true)
}

// Transit-13: Next Hop resolution with interface+IP out of that interface subnet
func testNHIPOutOfInterfaceSubnet(t *testing.T, args *testArgs) {
	t.Log("Testcase: Next Hop resolution with interface+IP out of that interface subnet")

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 30).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.2.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Correct the related NH and verify traffic
	ops = []func(){
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
			)
		},
	}

	res = util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+2)

	chk.HasResult(t, res, fluent.OperationResult().
		WithOperationID(1015).
		WithProgrammingResult(fluent.InstalledInRIB).
		AsResult(),
	)

	performATEAction(t, "ate", 1000, true)
}

// Transit-16:Changing IP address on I/F making NHOP unreachable and changing it back
func testChangeNHToUnreachableAndChangeBack(t *testing.T, args *testArgs) {
	t.Log("Testcase: Changing IP address on I/F making NHOP unreachable and changing it back")

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 15).
					AddNextHop(32, 25).
					AddNextHop(33, 35),
				// Setting Index 31 IP out of the related subnet
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	// Correct the related NH and verify traffic
	ops = []func(){
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("1.2.3.4"),
			)
		},
	}

	res = util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+2)

	chk.HasResult(t, res, fluent.OperationResult().
		WithOperationID(1015).
		WithProgrammingResult(fluent.InstalledInRIB).
		AsResult(),
	)

	// Correct the related NH and verify traffic
	ops = []func(){
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
			)
		},
	}

	res = util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+3)

	chk.HasResult(t, res, fluent.OperationResult().
		WithOperationID(1016).
		WithProgrammingResult(fluent.InstalledInRIB).
		AsResult(),
	)

	performATEAction(t, "ate", 1000, true)

}

// Transit-19: Next Hop Group resolution change NH from recursive and non-recursive
func testChangeNHFromRecursiveToNonRecursive(t *testing.T, args *testArgs) {
	t.Log("Testcase: Next Hop Group resolution change NH from recursive and non-recursive")

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				// Setting Index 31 IP out of the related subnet
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 15).
					AddNextHop(42, 25).
					AddNextHop(43, 35).
					AddNextHop(44, 45),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	// Correct the related NH and verify traffic
	ops = []func(){
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res = util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(1017); i < 1015+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	performATEAction(t, "ate", 1000, true)

}

// Transit TC 073 - ADD/REPLACE/DELETE during related interface flap
func testAddReplaceDeleteWithRelatedInterfaceFLap(t *testing.T, args *testArgs) {
	t.Log("Testcase: Add, Replace, Delete operations with related interface flap")

	scale := 100

	ops := []func(){
		//Add all entries
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 15).
					AddNextHop(42, 25).
					AddNextHop(43, 35).
					AddNextHop(44, 45),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
		//Replace all entries
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().ReplaceEntry(t, entries...)
		},
		//Delete all entries
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().DeleteEntry(t, entries...)
		},
	}
	results := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := 1; i <= 3*(scale+14); i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	//Flap interfaces
	interface_names := []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"}
	for _, interface_name := range interface_names {
		ocutils.SetInterfaceState(t, args.dut, interface_name, false)
	}
	time.Sleep(30 * time.Second)
	for _, interface_name := range interface_names {
		ocutils.SetInterfaceState(t, args.dut, interface_name, true)
	}

	args.dut.GRIBI().SynchElectionID(t, true)
	args.fluentC = args.dut.GRIBI().MasterClient(t)
	ops = []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 15).
					AddNextHop(32, 25).
					AddNextHop(33, 35),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	results = util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := 3*(scale+14) + 1; i <= 4*(scale+14); i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Configure ATE and Verify traffic
	performATEAction(t, "ate", scale, true)
}

//Transit-40	DELETE: VRF IPv4 Entry with ECMP path NHG+NH in default vrf
func testDeleteVRFIPv4EntryECMPPath(t *testing.T, args *testArgs) {

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 15).
					AddNextHop(42, 25).
					AddNextHop(43, 35).
					AddNextHop(44, 45),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	performATEAction(t, "ate", 1000, true)

	// Delete
	ops2 := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().DeleteEntry(t, entries...)
		},
	}

	res2 := util.DoModifyOps(args.fluentC, t, ops2, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(1015); i < 1015+1000; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	// Expect traffic to fail
	performATEAction(t, "ate", 1000, false)

}

//Transit-45	DELETE: default VRF IPv4 Entry with ECMP+backup path NHG+NH in default vrf
func testDeleteDefaultIPv4EntryECMPPath(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(4).WithIPAddress("100.122.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(14).WithBackupNHG(11).AddNextHop(4, 15),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.0/32").WithNextHopGroup(14).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 7; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	performATEAction(t, "ate", 1, true)

	// Delete
	ops2 := []func(){
		func() {
			args.fluentC.Modify().DeleteEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(4).WithIPAddress("100.122.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(14).WithBackupNHG(11).AddNextHop(4, 15),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.0/32").WithNextHopGroup(14).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
			)
		},
	}
	res2 := util.DoModifyOps(args.fluentC, t, ops2, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(7); i < 13; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Expect traffic to fail
	performATEAction(t, "ate", 1, false)

}

//Transit-32 REPLACE: VRF IPv4 Entry with ECMP path NHG+NH in default vrf
func testReplaceVRFIPv4EntryECMPPath(t *testing.T, args *testArgs) {

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Traffic start
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	scaleflow := getScaleFlow(t, portMaps, ate, "IPinIPWithScale", 1000)
	ate.Traffic().Start(t, scaleflow)

	// Replace same ipv4 entry
	ops2 := []func(){
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}

			args.fluentC.Modify().ReplaceEntry(t, entries...)
		},
	}

	res2 := util.DoModifyOps(args.fluentC, t, ops2, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(1015); i < 1015+1000; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats)
	if trafficPass == true {
		t.Log("Traffic works as expected")
	} else {
		t.Error("Traffic doesn't work as expected")
	}

}

//Transit-36 REPLACE: default VRF IPv4 Entry with ECMP path NHG+NH in default vrf
func testReplaceDefaultIPv4EntryECMPPath(t *testing.T, args *testArgs) {

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)

	}

	// Replace same ipv4 entry
	ops2 := []func(){
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}

			args.fluentC.Modify().ReplaceEntry(t, entries...)
		},
	}

	res2 := util.DoModifyOps(args.fluentC, t, ops2, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(1015); i < 1015+1000; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	// Traffic start
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	scaleflow := getScaleFlow(t, portMaps, ate, "IPinIPWithScale", 1000, "default")
	ate.Traffic().Start(t, scaleflow)

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats)
	if trafficPass == true {
		t.Log("Traffic works as expected")
	} else {
		t.Error("Traffic doesn't work as expected")
	}

}

// Transit-52	ADD/REPLACE change NH from single path to ECMP
func testReplaceSinglePathtoECMP(t *testing.T, args *testArgs) {

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.11/32").WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 30),
			)
		},
	}
	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 4; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Start Traffic
	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()
	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	ate.Traffic().Start(t, baseflow)

	// Add New NHG
	ops2 := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(4).WithIPAddress("100.122.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(4, 30).AddNextHop(3, 30),
			)
		},
	}
	res2 := util.DoModifyOps(args.fluentC, t, ops2, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(4); i < 6; i++ {
		chk.HasResult(t, res2, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// traffic verification
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().InterfaceAny().Counters().Get(t)
	trafficPass := util.CheckTrafficPassViaPortPktCounter(stats)
	if trafficPass == true {
		t.Log("Traffic works as expected")
	} else {
		t.Error("Traffic doesn't work as expected")
	}

}

// Transit TC 068 - Verify ISIS/BGP control plane doesn’t  affect gRIBI related traffic with connected NHOP
func testIsisBgpControlPlaneInteractionWithGribi(t *testing.T, args *testArgs) {
	t.Log("Testcase: Verify ISIS/BGP control plane doesn’t  affect gRIBI related traffic with connected NHOP")
	scale := 100

	ops := []func(){
		//Add all entries
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 20).
					AddNextHop(33, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}
	results := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := 1; i <= scale+14; i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	//Generate flows over ISIS and BGP sessions.
	ate := ondatra.ATE(t, "ate")
	topo := getIXIATopology(t, "ate")
	isisFlow := ocutils.GetBoundedFlow(t, ate, topo, "1/1", "1/2", "isis_network1", "isis_network2", "isis", 16)
	bgpFlow := ocutils.GetBoundedFlow(t, ate, topo, "1/1", "1/2", "bgp_network", "bgp_network", "bgp", 16)
	scaleFlow := getScaleFlow(t, topo.Interfaces(), ate, "IPinIPWithScale", scale)
	// Configure ATE and Verify traffic
	performATEActionForMultipleFlows(t, "ate", true, 0.90, isisFlow, bgpFlow, scaleFlow)
}

// Transit TC 071 - Verify protocol (BGP) over gribi transit fwding entry
func testBgpProtocolOverGribiTransitEntry(t *testing.T, args *testArgs) {
	t.Log("Testcase: Verify protocol (BGP) over gribi transit fwding entry")

	ops := []func(){
		// 192.0.2.40/32  for east-to-west flow
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).AddNextHop(31, 100),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.120.1.2").WithInterfaceRef("Bundle-Ether120"),
			)
		},
		// 192.0.2.140/32  for west-to-east flow
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.140/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).AddNextHop(41, 100),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
			)
		},
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("11.11.11.1/32").WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 100),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"),
				fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix("12.12.12.1/32").WithNextHopGroup(2).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(2).AddNextHop(20, 100),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.140"),
			)
		},
	}
	results := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := 1; i <= 12; i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	//Configure BGP on TGN
	ate := ondatra.ATE(t, "ate")
	topo := getIXIATopology(t, "ate")
	//Generate DSCP48 flow
	bgpFlow := ocutils.GetBoundedFlow(t, ate, topo, "1/1", "1/2", "bgp_transit_network", "bgp_transit_network", "bgp", 48)

	// Configure ATE and Verify traffic
	performATEActionForMultipleFlows(t, "ate", true, 0.99, bgpFlow)

}

// Transit TC 075 - ADD/REPLACE/DELETE with same Prefix with varying prefix lengths
func testAddReplaceDeleteWithSamePrefixWithVaryingPrefixLength(t *testing.T, args *testArgs) {
	t.Log("Testcase: Add, Replace, Delete operations with same prefix with varying prefix lengths and traffic verification")

	//create ipv4Entry for subnet 11.0.0.0/8 through 11.11.11.1/32
	start := 8
	end := 32
	prefix := "11.11.11.1"

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := start; i <= end; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIpv4Net(prefix, i)).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := start; i <= end; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIpv4Net(prefix, i)).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().ReplaceEntry(t, entries...)
		},
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))
			for i := start; i <= end; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIpv4Net(prefix, i)).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			args.fluentC.Modify().DeleteEntry(t, entries...)
		},
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
	}

	results := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := 1; i <= (end-start+15)*3; i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Add back all entries
	args.dut.GRIBI().SynchElectionID(t, true)

	ops = []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 10).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := start; i <= end; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(util.GetIpv4Net(prefix, i)).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	results = util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := (end-start+15)*3 + 1; i <= (end-start+15)*4; i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	ate := ondatra.ATE(t, "ate")
	topo := getIXIATopology(t, "ate")
	scaleFlow := getScaleFlow(t, topo.Interfaces(), ate, "IPinIPWithScale", 1000)
	performATEActionForMultipleFlows(t, "ate", true, 0.99, scaleFlow)

}

// Transit-18: Next Hop Group resolution change NH from non-recursive and recursive
func testChangeNHFromNonRecursiveToRecursive(t *testing.T, args *testArgs) {
	t.Log("Testcase: Next Hop Group resolution change NH from recursive and non-recursive")

	ops := []func(){
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(3).WithIPAddress("100.121.1.2"),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(11).AddNextHop(3, 15),
			)
		},
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 20).
					AddNextHop(33, 10),
				// Setting Index 31 IP out of the related subnet
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(11).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	// Correct the related NH and verify traffic
	ops = []func(){
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res = util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+2)

	for i := uint64(1017); i < 1015+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	performATEAction(t, "ate", 1000, true)

}

// Transit- Set ISIS overload bit and then verify traffic
func testSetISISOverloadBit(t *testing.T, args *testArgs) {

	scale := 100

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 20).
					AddNextHop(33, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+100; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Configure ISIS overload bit
	config := args.dut.Config().NetworkInstance("default").Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()
	config.Update(t, true)
	defer config.Delete(t)

	performATEAction(t, "ate", 100, true)
}

// Transit- Change peer ip/mac address and then verify traffic
func testChangePeerAddress(t *testing.T, args *testArgs) {
	scale := 1000

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 20).
					AddNextHop(33, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+100; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Try to change peer mac or fallback to peer address
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	i := 1
	// portMaps["1/2"].IPv4().WithAddress(fmt.Sprintf("100.%d.1.3/24", 120+i)).WithDefaultGateway(fmt.Sprintf("100.%d.1.1", 120+i))
	// topology.Update(t)

	performATEAction(t, "ate", scale, true)

	// Undo
	portMaps["1/2"].IPv4().WithAddress(fmt.Sprintf("100.%d.1.2/24", 120+i)).WithDefaultGateway(fmt.Sprintf("100.%d.1.1", 120+i))
	topology.Update(t)
}

// Transit- LC OIR
func testLC_OIR(t *testing.T, args *testArgs) {

	scale := 1000

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 20).
					AddNextHop(33, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+100; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// LC OIR
	//dut1.Config().New().WithCiscoText(" do reload location 0/0/CPU0 noprompt \n").Append(t)
	//t.Log(" Reload the LC")

	performATEAction(t, "ate", scale, true)
}

// Transit TC 072 - Verify dataplane fields(TTL, DSCP) with gribi transit fwding entry
func testDataPlaneFieldsOverGribiTransitFwdingEntry(t *testing.T, args *testArgs) {
	t.Log("Testcase:  Verify dataplane fields(TTL, DSCP) with gribi transit fwding entry")

	scale := 100

	ops := []func(){
		// 192.0.2.40/32  for east-to-west flow
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).AddNextHop(31, 100),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.120.1.2").WithInterfaceRef("Bundle-Ether120"),
			)
		},
		// 192.0.2.140/32  for west-to-east flow
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.140/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).AddNextHop(41, 100),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
			)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("101.1.1.1", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 100))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			args.fluentC.Modify().AddEntry(t, entries...)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("102.1.1.1", i, "32")).WithNextHopGroup(2).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(2).AddNextHop(20, 100))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.140"))
			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}
	results := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := 1; i <= 2*scale+10; i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	//Outer header TTL decrements by 1, DSCP stays same over gRIBI forwarding entry.
	ate := ondatra.ATE(t, "ate")
	topo := getIXIATopology(t, "ate")

	//flow with dscp=48, ttl=100
	dscpTtlFlow := ocutils.GetBoundedFlow(t, ate, topo, "1/1", "1/2", "network101", "network102", "dscpTtlFlow", 48, 100)
	//add acl with dscp=48, ttl=99. Transit traffic will have ttl decremented by 1
	aclName := "ttl_dscp"
	aclConfig := ocutils.GetIpv4Acl(aclName, 10, 48, 99, telemetry.Acl_FORWARDING_ACTION_ACCEPT)
	args.dut.Config().Acl().Update(t, aclConfig)
	//apply egress acl on all interfaces of interest
	interface_names := []string{"Bundle-Ether120", "Bundle-Ether121"}
	for _, interface_name := range interface_names {
		args.dut.Config().Acl().Interface(interface_name).EgressAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4).SetName().Update(t, aclName)
	}

	// Verify traffic passes through ACL - allowing same DSCP and TTL decremented by 1
	performATEActionForMultipleFlows(t, "ate", true, 0.99, dscpTtlFlow)

	//remove acl from interfaces
	for _, interface_name := range interface_names {
		args.dut.Config().Acl().Interface(interface_name).EgressAclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4).Delete(t)
	}
	//delete acl
	args.dut.Config().Acl().AclSet(aclName, telemetry.Acl_ACL_TYPE_ACL_IPV4).Delete(t)

}

// Transit TC 074 - ADD/REPLACE/DELETE during related configuration change
func testAddReplaceDeleteWithRelatedConfigChange(t *testing.T, args *testArgs) {
	t.Log("Testcase: Add, Replace, Delete operations with related configuration change")

	scale := 100

	ops := []func(){
		//Add all entries
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 20).
					AddNextHop(43, 30).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
		//Replace all entries
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 30).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().ReplaceEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 40).
					AddNextHop(43, 40).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().ReplaceEntry(t, entries...)
		},
		//Delete all entries
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 30).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().DeleteEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 40).
					AddNextHop(43, 40).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().DeleteEntry(t, entries...)
		},
	}
	results := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := 1; i <= 3*(scale+14); i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	//Change interface configurations and revert back
	interface_names := []string{"Bundle-Ether121", "Bundle-Ether122", "Bundle-Ether123", "Bundle-Ether124", "Bundle-Ether125", "Bundle-Ether126", "Bundle-Ether127"}
	//Store current config
	originalInterfaces := ocutils.GetCopyOfIpv4SubInterfaces(t, args.dut, interface_names, 0)
	//Change IP addresses for the interfaces in the slice
	initialIp := "123.123.123.123"
	counter := 1
	for _, interface_name := range interface_names {
		ipPrefix := ocutils.GetIPPrefix(initialIp, counter, "24")
		initialIp = strings.Split(ipPrefix, "/")[0]
		args.dut.Config().Interface(interface_name).Subinterface(0).Replace(t, ocutils.GetSubInterface(initialIp, 24, 0))
		t.Logf("Changed configuration of interface %s", interface_name)
		counter = counter + 256

	}
	//Revert original config
	for _, interface_name := range interface_names {
		osi := originalInterfaces[interface_name]
		osi.Index = ygot.Uint32(0)
		args.dut.Config().Interface(interface_name).Subinterface(0).Replace(t, osi)
		t.Logf("Restored configuration of interface %s", interface_name)
	}
	//Config change end

	args.dut.GRIBI().SynchElectionID(t, true)

	ops = []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 30).
					AddNextHop(32, 30).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 40).
					AddNextHop(43, 40).
					AddNextHop(44, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		func() {
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	results = util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := 3*(scale+14) + 1; i <= 4*(scale+14); i++ {
		chk.HasResult(t, results, fluent.OperationResult().
			WithOperationID(uint64(i)).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Configure ATE and Verify traffic
	performATEAction(t, "ate", scale, true, 0.99)
}

//Static Arp Resolution
func testCD2StaticMacChangeNHOP(t *testing.T, args *testArgs) {

	ops := []func(){

		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.121.1.3").WithInterfaceRef("Bundle-Ether121"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.11", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(20, 99))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 7; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	t.Log("going to program Static ARP different from Ixia ")
	args.dut.Config().New().WithCiscoText("arp 100.121.1.3  0000.0012.0011 arpa \n commit \n end \n").Append(t)

	time.Sleep(10 * time.Second)

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	t.Log("Traffic starting from Ixia should go with Next hop and Static ARP  ")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().FlowAny().Get(t)
	t.Log("slept and now need to collect stats")
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}
	t.Log("going to change Static ARP ")
	args.dut.Config().New().WithCiscoText("arp 100.121.1.3  0000.0012.0111 arpa \n commit\n").Append(t)

	time.Sleep(10 * time.Second)

	defer args.dut.Config().New().WithCiscoText("no arp 100.121.1.3  0000.0012.0111 arpa \n commit\n").Append(t)

	statsb := ate.Telemetry().FlowAny().Get(t)
	lossStreamb := util.CheckTrafficPassViaRate(statsb)

	if len(lossStreamb) > 0 {
		t.Error("There is stream failing after configuring static arp :", strings.Join(lossStreamb, ","))
	} else {
		t.Log("There is no traffic loss even after adding static arp ")
	}

	// dut1.Telemetry().NetworkInstance().Afts().Ipv4Entry().Get(t)
	time.Sleep(10 * time.Second)
}

//Initially Dynamic arp and then static arp to be resolved
func testCD2StaticDynamicMacNHOP(t *testing.T, args *testArgs) {

	ops := []func(){

		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.11", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(20, 99))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 7; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	t.Log("going to start Ixia protocols to bring up dynamic arp entry and start traffic  ")

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)

	baseflow := getBaseFlow(t, portMaps, ate, "IPinIPDynamic")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(60 * time.Second)
	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}
	t.Log("going to stop protocols to make sure static arp works ")
	topology.StopProtocols(t)

	t.Log("going to clear dynamic arp entry ")
	args.dut.Config().New().WithCiscoText("do clear arp-cache bundle-Ether 121 location 0/RP0/CPU0 \n commit\n").Append(t)

	time.Sleep(10 * time.Second)

	t.Log("going to configure static arp entry to make sure traffic is not failing after static arp is configured   ")
	args.dut.Config().New().WithCiscoText("arp 100.121.1.2  0000.0012.0011 arpa \n commit \n end \n").Append(t)

	time.Sleep(10 * time.Second)

	defer args.dut.Config().New().WithCiscoText("no arp 100.121.1.2  0000.0012.0011 arpa \n commit\n").Append(t)

	statsb := ate.Telemetry().FlowAny().Get(t)
	lossStreamb := util.CheckTrafficPassViaRate(statsb)

	if len(lossStreamb) > 0 {
		t.Error("There is stream failing after configuring static arp :", strings.Join(lossStreamb, ","))
	} else {
		t.Log("There is no traffic loss even after adding static arp ")
	}
	// dut1.Telemetry().NetworkInstance().Afts().Ipv4Entry().Get(t)
	time.Sleep(10 * time.Second)
}

// Transit-83 DELETE and RE-ADD flow spec config
func testDeleteReAddFlowSpecConfig(t *testing.T, args *testArgs) {

	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Remove the config
	t.Log("going to remove flow spec config")
	args.dut.Config().New().WithCiscoText("no flowspec \n commit\n end \n").Append(t)

	time.Sleep(10 * time.Second)

	t.Log("unconfigured flowspec")

	time.Sleep(2 * time.Second)

	// Re-add the config
	// flowspec
	//  local-install interface-all
	//  address-family ipv4
	//   local-install interface-all
	//   service-policy type pbr Transit local

	t.Log("going to re-add flow spec config")
	args.dut.Config().New().WithCiscoText("flowspec local-install interface-all  \n commit\n").Append(t)

	time.Sleep(10 * time.Second)

	args.dut.Config().New().WithCiscoText("flowspec address-family ipv4 local-install interface-all \n commit\n").Append(t)

	time.Sleep(10 * time.Second)

	args.dut.Config().New().WithCiscoText("flowspec address-family ipv4 service-policy type pbr Transit local \n commit\n end \n").Append(t)

	time.Sleep(10 * time.Second)

	t.Log("configured flowspec")

	time.Sleep(2 * time.Second)

	performATEAction(t, "ate", 1000, true)

}

// Transit- Clearing ARP and then verify traffic
func testClearingARP(t *testing.T, args *testArgs) {
	ops := []func(){
		// 192.0.2.40/32  Self-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.40/32").WithNextHopGroup(40),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(40).
					AddNextHop(31, 10).
					AddNextHop(32, 20).
					AddNextHop(33, 30),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(31).WithIPAddress("100.121.1.2").WithInterfaceRef("Bundle-Ether121"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(32).WithIPAddress("100.122.1.2").WithInterfaceRef("Bundle-Ether122"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(33).WithIPAddress("100.123.1.2").WithInterfaceRef("Bundle-Ether123"),
			)
		},
		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40).
					AddNextHop(42, 30).
					AddNextHop(43, 20).
					AddNextHop(44, 10),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.124.1.2").WithInterfaceRef("Bundle-Ether124"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(42).WithIPAddress("100.125.1.2").WithInterfaceRef("Bundle-Ether125"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(43).WithIPAddress("100.126.1.2").WithInterfaceRef("Bundle-Ether126"),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(44).WithIPAddress("100.127.1.2").WithInterfaceRef("Bundle-Ether127"),
			)
		},
		// 11.11.11.0/32
		func() {
			scale := 1000
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.0", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(10, 85).AddNextHop(20, 15))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(10).WithIPAddress("192.0.2.40"))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 13+1000; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}

	// Clear ARP
	args.dut.Config().New().WithCiscoText("do clear arp-cache location all \n").Append(t)

	time.Sleep(10 * time.Second)

	t.Log("Cleared ARP")

	time.Sleep(1 * time.Second)

	performATEAction(t, "ate", 1000, true)

}

//Static Arp Resolution
func testCD2StaticMacNHOP(t *testing.T, args *testArgs) {

	ops := []func(){

		// 192.0.2.42/32  Next-Site
		func() {
			args.fluentC.Modify().AddEntry(t,
				fluent.IPv4Entry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithPrefix("192.0.2.42/32").WithNextHopGroup(100),
				fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(100).
					AddNextHop(41, 40),
				fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(41).WithIPAddress("100.121.1.3").WithInterfaceRef("Bundle-Ether121"),
			)
		},
		// 11.11.11.1132
		func() {
			scale := 1
			entries := []fluent.GRIBIEntry{}
			for i := 0; i < scale; i++ {
				entries = append(entries, fluent.IPv4Entry().WithNetworkInstance("TE").WithPrefix(ocutils.GetIPPrefix("11.11.11.11", i, "32")).WithNextHopGroup(1).WithNextHopGroupNetworkInstance(server.DefaultNetworkInstanceName))
			}
			entries = append(entries, fluent.NextHopGroupEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithID(1).AddNextHop(20, 99))
			entries = append(entries, fluent.NextHopEntry().WithNetworkInstance(server.DefaultNetworkInstanceName).WithIndex(20).WithIPAddress("192.0.2.42"))

			args.fluentC.Modify().AddEntry(t, entries...)
		},
	}

	res := util.DoModifyOps(args.fluentC, t, ops, fluent.InstalledInRIB, false, args.elecLow+1)

	for i := uint64(1); i < 7; i++ {
		chk.HasResult(t, res, fluent.OperationResult().
			WithOperationID(i).
			WithProgrammingResult(fluent.InstalledInRIB).
			AsResult(),
		)
	}
	t.Log("going to program Static ARP different from Ixia ")
	args.dut.Config().New().WithCiscoText("arp 100.121.1.3  0000.0012.0011 arpa \n commit \n end \n").Append(t)

	time.Sleep(10 * time.Second)

	defer args.dut.Config().New().WithCiscoText("no arp 100.121.1.3  0000.0012.0011 arpa \n commit\n").Append(t)

	ate := ondatra.ATE(t, "ate")
	topology := getIXIATopology(t, "ate")
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)
	baseflow := getBaseFlow(t, portMaps, ate, "IPinIP")
	t.Log("Traffic starting from Ixia should go with Next hop and Static ARP  ")
	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)
	time.Sleep(60 * time.Second)

	stats := ate.Telemetry().FlowAny().Get(t)
	lossStream := util.CheckTrafficPassViaRate(stats)

	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}

	// dut1.Telemetry().NetworkInstance().Afts().Ipv4Entry().Get(t)
	time.Sleep(10 * time.Second)
}

func TestTransitWECMPFlush(t *testing.T) {
	ctx := context.Background()
	test := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
	}{
		// {
		// 	name: "CD2ConnectedNHIP",
		// 	desc: "Set primary and backup path with gribi and shutdown all the primary path validating traffic switching over backup path and dropping",
		// 	fn:   testCD2ConnectedNHIP,
		// },
		// {
		// 	name: "CD2RecursiveNonConnectedNHOP",
		// 	desc: "Set primary and backup path with gribi and shutdown all the primary path validating traffic switching over backup path and dropping",
		// 	fn:   testCD2RecursiveNonConnectedNHOP,
		// },
		{
			name: "CD2RecursiveNonConnectedNHOP",
			desc: "Set primary and backup path with gribi and shutdown all the primary path validating traffic switching over backup path and dropping",
			fn:   testCD2RecursiveNonConnectedNHOP,
		},
	}
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)
			dut := ondatra.DUT(t, "dut")
			ate := ondatra.ATE(t, "ate")
			topology := getIXIATopology(t, "ate")
			client := gribi.Client{
				DUT:                  dut,
				FibACK:               false,
				Persistence:          true,
				InitialElectionIDLow: 10,
			}
			defer client.Close(t)
			if err := client.Start(t); err != nil {
				t.Fatalf("gRIBI Connection can not be established")
			}
			elecLow, _ := client.LearnElectionID(t)
			client.BecomeLeader(t)
			fluentC := client.Fluent(t)
			defer util.FlushServer(fluentC, t)
			args := &testArgs{
				ctx:      ctx,
				c:        &client,
				dut:      dut,
				ate:      ate,
				topology: topology,
				fluentC:  fluentC,
				elecLow:  elecLow,
			}
			tt.fn(t, args)
		})
	}
}
