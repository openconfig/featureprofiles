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

package transitwecmpflush_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

var (
	ixiaTopology   = make(map[string]*ondatra.ATETopology)
	sortedAtePorts []string //keep sorted ports for ate, the first port is the send and the rest are recive
	portID []string //list of PortIDs for ATE ports
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
	sortedAtePorts = []string{}
	for _, portid := range ate.Ports() {
		portID = append(portID, portid.ID())
		sort.Strings(portID)
	}
	for _,port := range portID{
		sortedAtePorts = append(sortedAtePorts,ate.Port(t,port).Name())
	}
	if len(sortedAtePorts) < 2 {
		t.Fatalf("At least two ports are required for the test")
	}
	if len(strings.Split(sortedAtePorts[0], "/")) != 2 {
		t.Fatalf("Ate port name expected to be in format int/int, e.g., 1/6")
	}
	atePorttoIPs := make(map[string][]string) // generate ip for tgen
	for i, port := range sortedAtePorts {
		atePorttoIPs[port] = []string{
			fmt.Sprintf("100.%d.1.2/24", 120+i),
			fmt.Sprintf("100.%d.1.1", 120+i),
		}
	}

	for _, p := range ate.Device.Ports() {
		intf := topoobj.AddInterface(p.Name())
		intf.WithPort(ate.Port(t, p.ID()))
		intf.IPv4().WithAddress(atePorttoIPs[p.Name()][0]).WithDefaultGateway(atePorttoIPs[p.Name()][1])
	}
	addNetworkAndProtocolsToAte(t, ate, topoobj)
}

func addNetworkAndProtocolsToAte(t *testing.T, ate *ondatra.ATEDevice, topo *ondatra.ATETopology) {
	//Add prefixes/networks on ports
	scale := uint32(10)
	util.AddIpv4Network(t, topo, sortedAtePorts[0], "network101", "101.1.1.1/32", scale)
	util.AddIpv4Network(t, topo, sortedAtePorts[1], "network102", "102.1.1.1/32", scale)
	//Configure ISIS, BGP on TGN
	util.AddAteISISL2(t, topo, sortedAtePorts[0], "490001", "transit_wecmp_isis_1", 20, "120.1.1.1/32", scale)
	util.AddAteISISL2(t, topo, sortedAtePorts[1], "490002", "transit_wecmp_isis_2", 20, "121.1.1.1/32", scale)
	// util.AddAteEBGPPeer(t, topo, sortedAtePorts[0], "100.120.1.1", 64001, "bgp_network_1", "100.120.1.2", "130.1.1.1/32", scale, false)
	util.AddAteEBGPPeer(t, topo, sortedAtePorts[1], "100.121.1.1", 64001, "bgp_network_2", "100.121.1.2", "131.1.1.1/32", scale, false)
	// //Configure loopbacks for BGP to use as source addresses
	// util.AddLoopback(t, topo, sortedAtePorts[0], "11.11.11.1/32")
	// util.AddLoopback(t, topo, sortedAtePorts[1], "12.12.12.1/32")
	// //BGP instance for traffic over gRIBI transit forwarding entries
	// //BGP uses DSCP48 for control traffic. Router needs to be configured to handle DSCP48 accordingly.
	// util.AddAteEBGPPeer(t, topo, sortedAtePorts[0], "100.120.1.1", 64001, "bgp_transit_network_1", "100.120.1.2", "11.11.11.1/32", 1, true)
	// util.AddAteEBGPPeer(t, topo, sortedAtePorts[1], "100.121.1.1", 64001, "bgp_transit_network_2", "100.121.1.2", "12.12.12.1/32", 1, true)
}

func getBaseFlow(t *testing.T, atePorts map[string]*ondatra.Interface, ate *ondatra.ATEDevice, flowName string, vrf ...string) *ondatra.Flow {
	flow := ate.Traffic().NewFlow(flowName)
	t.Log("Setting up base flow...")
	srcPort := sortedAtePorts[0]
	dstPort := sortedAtePorts[1]
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
	flow.WithSrcEndpoints(atePorts[sortedAtePorts[0]])

	t.Log("Extending to multiple receiver ports...")
	rxPorts := []ondatra.Endpoint{}
	for i, port := range sortedAtePorts {
		if i == 0 {
			continue
		}
		rxPorts = append(rxPorts, atePorts[port])
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
	stats := gnmi.GetAll(t, ate, gnmi.OC().InterfaceAny().Counters().State())
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
	stats := gnmi.GetAll(t, ate, gnmi.OC().InterfaceAny().Counters().State())
	t.Log("Packets transmitted by ports: ", gnmi.GetAll(t, ate, gnmi.OC().InterfaceAny().Counters().OutPkts().State()))
	t.Log("Packets received by ports: ", gnmi.GetAll(t, ate, gnmi.OC().InterfaceAny().Counters().InPkts().State()))
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
	flow.WithSrcEndpoints(atePorts[sortedAtePorts[0]])
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

func checkTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, flowDuration time.Duration, flow ...*ondatra.Flow) {
	ate.Traffic().Start(t, flow...)
	defer ate.Traffic().Stop(t)
	time.Sleep(flowDuration * time.Second)
	stats := gnmi.GetAll(t, ate, gnmi.OC().FlowAny().State())
	lossStream := util.CheckTrafficPassViaRate(stats)
	if len(lossStream) > 0 {
		t.Error("There is stream failing:", strings.Join(lossStream, ","))
	} else {
		t.Log("There is no traffic loss.")
	}
}

func checkTraffic(t *testing.T, protocl string, ate *ondatra.ATEDevice, expectFailure bool, vrf ...string) {
	t.Log("Start Ixia protocols to bring up dynamic arp entry and start traffic  ")

	topology := getIXIATopology(t, ate.ID())
	portMaps := topology.Interfaces()

	topology.StartProtocols(t)
	defer topology.StopProtocols(t)

	baseflow := getBaseFlow(t, portMaps, ate, protocl, vrf...)

	ate.Traffic().Start(t, baseflow)
	defer ate.Traffic().Stop(t)

	time.Sleep(45 * time.Second)
	stats := gnmi.GetAll(t, ate, gnmi.OC().FlowAny().State())
	lossStream := util.CheckTrafficPassViaRate(stats)

	if expectFailure {
		if len(lossStream) > 0 {
			t.Log("There is stream failing as expected: ", strings.Join(lossStream, ","))
		} else {
			t.Fatalf("Expected Traffic loss, but there is no traffic loss.")
		}
	} else {
		if len(lossStream) > 0 {
			t.Fatal("There is stream failing:", strings.Join(lossStream, ","))
		} else {
			t.Log("There is no traffic loss as expetd")
		}
	}

}
