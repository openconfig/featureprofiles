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
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/gribigo/server"
	"github.com/openconfig/ondatra"
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
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func testCD2ConnectedNHIP(t *testing.T, args *testArgs) {
	args.c.AddNH(t, 3, "100.121.1.2", server.DefaultNetworkInstanceName, fluent.InstalledInRIB)
	args.c.AddIPv4(t, "11.11.11.11/32", 11, "TE", server.DefaultNetworkInstanceName, fluent.InstalledInRIB)
	args.c.AddNHG(t, 11, map[uint64]uint64{3: 15}, server.DefaultNetworkInstanceName, fluent.InstalledInRIB)

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

func TestTransitWECMPFlush(t *testing.T) {
	ctx := context.Background()
	test := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
	}{
		{
			name: "CD2ConnectedNHIP",
			desc: "Set primary and backup path with gribi and shutdown all the primary path validating traffic switching over backup path and dropping",
			fn:   testCD2ConnectedNHIP,
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
			}
			tt.fn(t, args)
		})
	}
}
