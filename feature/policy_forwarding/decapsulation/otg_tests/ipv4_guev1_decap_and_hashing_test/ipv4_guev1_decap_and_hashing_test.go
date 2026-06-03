// Copyright 2024 Google LLC
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

package ipv4_guev1_decap_and_hashing_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
)

// Constants for BGP ASNs
const (
	dutAS                = 100
	ate1AS               = 200   // EBGP with DUT Port1
	ate2AS               = dutAS // IBGP with DUT Port2
	ate3AS               = dutAS // IBGP with DUT LAG1
	ate4AS               = 200   // EBGP with DUT LAG2
	ate5AS               = 200   // EBGP with DUT Port7
	plenIPv4             = 30
	plenIPv6             = 126
	advertisedIPv4PfxLen = 24
	advertisedIPv6PfxLen = 64
	loopbackPfxLen       = 32
	isisInstance         = "DEFAULT"
	dutAreaAddress       = "49.0001"
	dutSysID             = "1920.0000.2001"
	ateSysID             = "64000000000"
	UDPSrcPort           = 1000
	UDPDstPort           = 6080
	UDPDstPortNeg        = 6085
	testSrcPort          = 14
	testDstPort          = 15
	flowCount            = 10 // Number of prefixes/routes per host group
	tolerance            = 5  // As per readme, Tolerance for delta: 5%
	fixedPackets         = 1000000
	trafficFrameSize     = 1500
	ratePercent          = 10
	lspV4Name            = "lsp-egress-v4"
	mplsLabel            = 1000
	decapType            = "udp"
	decapPort            = 6080
	ecmpMaxPath          = 4
	policyName           = "decap-policy"
	policyID             = 1
	trafficDuration      = 20
	dutLoopbackName      = "Loopback0"
	aggID1, aggID2       = "lag1", "lag2"
)

// IP Addresses and Attributes
var (
	// DUT Loopback0 (GUE Decap Address)
	dutLo0 = attrs.Attributes{Desc: "DUT Loopback0", IPv4: "192.168.3.2", IPv4Len: loopbackPfxLen, IPv6: "2001:db8:c000::1", IPv6Len: 128}

	// DUT Port1 <> ATE Port1 (ATE1)
	dutP1 = attrs.Attributes{Desc: "DUT Port1", IPv4: "192.0.1.1", IPv6: "2001:db8:1::1", MAC: "02:00:01:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateP1 = attrs.Attributes{Name: "ateP1", IPv4: "192.0.1.2", IPv6: "2001:db8:1::2", MAC: "02:00:01:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}

	// DUT Port2 <> ATE Port2 (ATE2)
	dutP2 = attrs.Attributes{Desc: "DUT Port2", IPv4: "192.0.2.1", IPv6: "2001:db8:2::1", MAC: "02:00:02:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateP2 = attrs.Attributes{Name: "ateP2", IPv4: "192.0.2.2", IPv6: "2001:db8:2::2", MAC: "02:00:02:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}

	// DUT LAG (Port3, Port4) <> ATE LAG (Port3, Port4) (ATE3)
	dutLag1 = attrs.Attributes{Desc: "DUTLag1", IPv4: "192.0.3.1", IPv6: "2001:db8:3::1", MAC: "02:00:03:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateLag1 = attrs.Attributes{Name: "ateLag1", IPv4: "192.0.3.2", IPv6: "2001:db8:3::2", MAC: "02:00:03:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}

	dutLag2 = attrs.Attributes{Desc: "DUTLag2", IPv4: "192.0.4.1", IPv6: "2001:db8:4::1", MAC: "02:00:04:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateLag2 = attrs.Attributes{Name: "ateLag2", IPv4: "192.0.4.2", IPv6: "2001:db8:4::2", MAC: "02:00:04:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}

	// ATE3 Loopback (for ISIS passive demo)
	ate3Lo = attrs.Attributes{Name: "ate3Lo0", IPv4: "192.168.3.1", IPv6: "2001:db8:10::1", IPv4Len: loopbackPfxLen, IPv6Len: 128}

	// DUT Port7 <--> ATE P7 (Represents ATE5 in diagram)
	dutP7 = attrs.Attributes{Desc: "DUT Port7", IPv4: "192.0.7.1", IPv6: "2001:db8:7::1", MAC: "02:00:05:02:02:02", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	ateP7 = attrs.Attributes{Name: "atep7", IPv4: "192.0.7.2", IPv6: "2001:db8:7::2", MAC: "02:00:05:01:01:01", IPv4Len: plenIPv4, IPv6Len: plenIPv6}
	// Advertised Prefixes (base addresses)
	host1IPv4Start = "198.51.100.0"
	host1IPv6Start = "2001:db8:100::"
	host2IPv4Start = "198.51.110.0"
	host2IPv6Start = "2001:db8:110::"
	host3IPv4Start = "198.51.120.0"
	host3IPv6Start = "2001:db8:120::"
	host4IPv4Start = "198.51.130.0"
	host4IPv6Start = "2001:db8:130::"
	ate1LoopbackIP = "172.16.1.0"
	timeout        = 1 * time.Minute
	constH1v4      = "198.51.100.1"
	constH1v6      = "2001:db8:100::1"
	constH2v4      = "198.51.110.1"
	constH2v6      = "2001:db8:110::1"
	constH3v4      = "198.51.120.1"
	constH3v6      = "2001:db8:120::1"
	constH4v4      = "198.51.130.1"
	constH4v6      = "2001:db8:130::1"
)

type Neighbor struct {
	IPv4 string
	IPv6 string
}

var AggregateInterfaceIDs = map[ondatra.Vendor]map[string]string{
	ondatra.CISCO:  {aggID1: "Bundle-Ether1", aggID2: "Bundle-Ether2"},
	ondatra.ARISTA: {aggID1: "Port-Channel1", aggID2: "Port-Channel2"},
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestMultipathGUE(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	pushStartWaitTime := 20 * time.Second
	aggIDs := configureDUT(t, dut)
	otgConfig := configureATE(t, ate)
	sfBatch := &gnmi.SetBatch{}
	cfgplugins.MPLSStaticLSP(t, sfBatch, dut, lspV4Name, mplsLabel, ateLag1.IPv4, "", "ipv4")
	sfBatch.Set(t, dut)
	pushAndStartProtocols(t, ate, otgConfig, pushStartWaitTime)
	otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv6")
	for _, aggID := range aggIDs {
		gnmi.Await(t, dut, gnmi.OC().Interface(aggID).OperStatus().State(), 60*time.Second, oc.Interface_OperStatus_UP)
	}
	neighbors := []Neighbor{
		{IPv4: ateP1.IPv4, IPv6: ateP1.IPv6},
		{IPv4: ateP2.IPv4, IPv6: ateP2.IPv6},
		{IPv4: ateLag1.IPv4, IPv6: ateLag1.IPv6},
		{IPv4: ateLag2.IPv4, IPv6: ateLag2.IPv6},
		{IPv4: ateP7.IPv4, IPv6: ateP7.IPv6},
	}
	checkBgpStatus(t, dut, neighbors)
	t.Run("PF-1.22.1[Baseline]: GUE Decapsulation over ipv6 decap address and Load-balance test", func(t *testing.T) {
		destinations := [][]string{
			{otgConfig.Lags().Items()[0].Name()},                                      // Flow#1 to H3 via ATE3 LAG
			{otgConfig.Ports().Items()[1].Name(), otgConfig.Lags().Items()[0].Name()}, // Flow#2 to H2 via ATE2 + ATE3 LAG
			{otgConfig.Ports().Items()[1].Name(), otgConfig.Lags().Items()[0].Name()}, // Flow#3 same as Flow#2
			{otgConfig.Ports().Items()[2].Name(), otgConfig.Lags().Items()[1].Name()}, // Flow#4 to H4 via ATE4 LAG + ATE5
			{otgConfig.Ports().Items()[2].Name(), otgConfig.Lags().Items()[1].Name()}, // Flow#5 same as Flow#4
			{otgConfig.Lags().Items()[0].Name()},                                      // Flow#6 to H3 via ATE3 LAG
			{otgConfig.Ports().Items()[1].Name(), otgConfig.Lags().Items()[0].Name()}, // Flow#7 to H2 via ATE2 + ATE3 LAG
			{otgConfig.Ports().Items()[1].Name(), otgConfig.Lags().Items()[0].Name()}, // Flow#8 same as Flow#7
			{otgConfig.Ports().Items()[2].Name(), otgConfig.Lags().Items()[1].Name()}, // Flow#9 to H4 via ATE4 LAG + ATE5
			{otgConfig.Ports().Items()[2].Name(), otgConfig.Lags().Items()[1].Name()}, // Flow#10 same as Flow#9
		}
		activeFlowIndices := []int{
			2,
			1,
			3,
			4,
			5,
			6,
			7,
			8,
			9,
			10,
		}
		macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
		for _, flowIndex := range activeFlowIndices {
			otgConfig.Flows().Clear()
			flow := configureFlows(t, otgConfig, macAddress, destinations[flowIndex-1], flowIndex, false)
			pushAndStartProtocols(t, ate, otgConfig, pushStartWaitTime)
			otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv4")
			otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv6")
			t.Logf("Running test for flow index %d: %s", flowIndex, flow.Name())
			var payloadType, excludeLag, rxPort string
			var rxLags []string

			switch flowIndex {
			case 1, 6:
				payloadType = "mpls"
				rxLags = []string{otgConfig.Lags().Items()[0].Name()}
				rxPort = ""
				excludeLag = otgConfig.Lags().Items()[1].Name()
			case 2, 3:
				payloadType = "ipv4"
				rxLags = []string{otgConfig.Lags().Items()[0].Name()}
				rxPort = otgConfig.Ports().Items()[1].Name()
				excludeLag = otgConfig.Lags().Items()[1].Name()
			case 4, 5:
				payloadType = "ipv4"
				rxLags = []string{otgConfig.Lags().Items()[1].Name()}
				rxPort = otgConfig.Ports().Items()[2].Name()
				excludeLag = otgConfig.Lags().Items()[0].Name()
			case 7, 8:
				payloadType = "ipv6"
				rxLags = []string{otgConfig.Lags().Items()[0].Name()}
				rxPort = otgConfig.Ports().Items()[1].Name()
				excludeLag = otgConfig.Lags().Items()[1].Name()
			default:
				payloadType = "ipv6"
				rxLags = []string{otgConfig.Lags().Items()[1].Name()}
				rxPort = otgConfig.Ports().Items()[2].Name()
				excludeLag = otgConfig.Lags().Items()[0].Name()
			}
			// Configure decap on DUT for current payload
			configureDutWithGueDecap(t, dut, payloadType)

			// Allow counters to stabilize after PushConfig and StartProtocols.
			// Without explicit ClearCounters in OTG API, wait for any residual state to clear.
			time.Sleep(2 * time.Second)

			rxCheckPorts, expectedWeights := expectedPortWeightsForFlow(t, otgConfig, flowIndex, rxPort, rxLags)
			portFramesBefore := capturePortsInFrames(t, ate, rxCheckPorts)

			ate.OTG().StartTraffic(t)
			time.Sleep(trafficDuration * time.Second)
			ate.OTG().StopTraffic(t)
			// Loss check is performed inside testLoadBalance via physical port counter
			// totals, which are reliable for both port and LAG endpoints. We do not
			// use verifyFlowTraffic here because OTG flow.state.counters.in_pkts only
			// counts packets whose RX endpoint was resolved in Port mode; LAG names are
			// not resolvable in that mode and produce an undercount.
			weights := testLoadBalance(t, ate, flow.Name(), rxCheckPorts, portFramesBefore)
			if len(weights) != len(expectedWeights) {
				t.Fatalf("Flow %s: got %d load-balance buckets, want %d", flow.Name(), len(weights), len(expectedWeights))
			}
			for idx, want := range expectedWeights {
				if got := weights[idx]; got < (want-tolerance) || got > (want+tolerance) {
					t.Errorf("ECMP Percentage for destination index %d: got %d, want %d", idx+1, got, want)
				}
			}
			_ = excludeLag
		}
	})
	t.Run("PF-1.22.2: GUE Decapsulation over non-matching ipv6 decap address [Negative] test", func(t *testing.T) {
		t.Skip()
		var flows []gosnappi.Flow
		macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
		otgConfig.Flows().Clear()
		for flowIndex := 11; flowIndex <= 12; flowIndex++ {
			flow := configureFlows(t, otgConfig, macAddress, []string{otgConfig.Ports().Items()[1].Name()}, flowIndex, false)
			flows = append(flows, flow)
		}
		ate.OTG().PushConfig(t, otgConfig)
		ate.OTG().StartProtocols(t)
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration * time.Second)
		ate.OTG().StopTraffic(t)
		for _, flow := range flows {
			if ok := verifyFlowTraffic(t, ate, otgConfig, flow.Name(), "", nil); !ok {
				t.Fatalf("Packet loss detected in flow: %s", flow.Name())
			} else {
				t.Logf("Flow %s: Traffic validation success", flow.Name())
			}
		}
	})
	t.Run("PF-1.22.3: GUE Decapsulation over non-matching UDP decap port [Negative] test", func(t *testing.T) {
		t.Skip()
		var flows []gosnappi.Flow
		macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
		otgConfig.Flows().Clear()
		for flowIndex := 13; flowIndex <= 14; flowIndex++ {
			flow := configureFlows(t, otgConfig, macAddress, []string{otgConfig.Ports().Items()[1].Name()}, flowIndex, false)
			flows = append(flows, flow)
		}
		ate.OTG().PushConfig(t, otgConfig)
		ate.OTG().StartProtocols(t)
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration * time.Second)
		ate.OTG().StopTraffic(t)
		for _, flow := range flows {
			if ok := verifyTrafficFlowNegCase(t, ate, otgConfig, flow); !ok {
				t.Logf("Flow %s: Packets dropped, Test Passed", flow.Name())
			} else {
				t.Fatalf("Flow %s: Packets not dropped, Test Failed", flow.Name())
			}
		}
	})
	t.Run("PF-1.22.4: Verify the Immediate next header's L4 fields are not considered in Load-Balancing Algorithm test", func(t *testing.T) {
		t.Skip()
		t.Log("Starting test: Verify that immediate next header's L4 fields are NOT used in load-balancing")
		macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
		otgConfig.Flows().Clear()
		// Generate flows with randomized L4 ports immediately after outer header
		for flowIndex := 1; flowIndex <= 10; flowIndex++ {
			configureFlows(t, otgConfig, macAddress, []string{otgConfig.Ports().Items()[1].Name()}, flowIndex, true)
		}

		ate.OTG().PushConfig(t, otgConfig)
		ate.OTG().StartProtocols(t)
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration * time.Second)
		ate.OTG().StopTraffic(t)

		// Verify: Traffic should NOT be load-balanced → All traffic should go to a single port
		verifySinglePathTraffic(t, ate, otgConfig)
	})
	t.Run("PF-1.22.5: Verify the Immediate next header's L3 fields are not considered in Load-Balancing Algorithm test", func(t *testing.T) {
		t.Skip()
		t.Log("Starting test: Verify that immediate next header's L3 fields are NOT used in load-balancing")
		macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
		otgConfig.Flows().Clear()
		// Generate flows with Immediate next header's L3 fields
		for flowIndex := 1; flowIndex <= 10; flowIndex++ {
			configureFlows(t, otgConfig, macAddress, []string{otgConfig.Ports().Items()[1].Name()}, flowIndex, true)
		}
		ate.OTG().PushConfig(t, otgConfig)
		ate.OTG().StartProtocols(t)
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration * time.Second)
		ate.OTG().StopTraffic(t)

		// Verify: Traffic should NOT be load-balanced → All traffic should go to a single port
		verifySinglePathTraffic(t, ate, otgConfig)
	})

}

// configureDUT configures all DUT aspects.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) []string {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	p7 := dut.Port(t, "port7")
	var aggIDsList []string

	// Interface configurations
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutP1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutP2.NewOCInterface(p2.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p7.Name()).Config(), dutP7.NewOCInterface(p7.Name(), dut))

	// Loopback0 for GUE Decap and Router ID
	loopbackIntfName := netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1 := dutLo0.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, d.Interface(loopbackIntfName).Config(), loop1)
	} else {
		v4, ok := ipv4Addrs[0].Val()
		if ok {
			dutLo0.IPv4 = v4.GetIp()
		}
		v6, ok := ipv6Addrs[0].Val()
		if ok {
			dutLo0.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutLo0.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutLo0.IPv6)
	}
	// Ports 3 and 4 will be part of LAG
	dutAggPorts1 := []*ondatra.Port{
		dut.Port(t, "port3"),
		dut.Port(t, "port4"),
	}
	aggIDsList = append(aggIDsList, AggregateInterfaceIDs[dut.Vendor()][(aggID1)])
	//cfgplugins.DeleteAggregate(t, dut, AggregateInterfaceIDs[dut.Vendor()][(aggID1)], dutAggPorts1)
	aggrBatch := &gnmi.SetBatch{}
	cfgplugins.SetupStaticAggregateAtomically(t, dut, aggrBatch, cfgplugins.StaticAggregateConfig{AggID: AggregateInterfaceIDs[dut.Vendor()][(aggID1)], DutLag: dutLag1, AggPorts: dutAggPorts1})
	// Ports 5 and 6 will be part of LAG
	dutAggPorts2 := []*ondatra.Port{
		dut.Port(t, "port5"),
		dut.Port(t, "port6"),
	}
	aggIDsList = append(aggIDsList, AggregateInterfaceIDs[dut.Vendor()][(aggID2)])
	//cfgplugins.DeleteAggregate(t, dut, AggregateInterfaceIDs[dut.Vendor()][(aggID2)], dutAggPorts2)
	cfgplugins.SetupStaticAggregateAtomically(t, dut, aggrBatch, cfgplugins.StaticAggregateConfig{AggID: AggregateInterfaceIDs[dut.Vendor()][(aggID2)], DutLag: dutLag2, AggPorts: dutAggPorts2})
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	cfgplugins.ConfigureLoadbalance(t, dut)
	// Routing Policy (ALLOW) must be defined before BGP references it
	rpPolicy := cfgplugins.ConfigureCommonBGPPolicies(t, dut)
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpPolicy)
	// ISIS Configuration
	cfgISIS := cfgplugins.ISISConfigBasic{
		InstanceName: isisInstance,
		AreaAddress:  dutAreaAddress,
		SystemID:     dutSysID,
		AggID:        AggregateInterfaceIDs[dut.Vendor()][(aggID1)],
		Ports:        []*ondatra.Port{p1, p2},
		LoopbackIntf: dutLoopbackName,
	}
	cfgplugins.NewISISBasic(t, aggrBatch, dut, cfgISIS)
	cfgBGP := cfgplugins.BGPConfig{DutAS: dutAS, RouterID: dutP1.IPv4, ECMPMaxPath: ecmpMaxPath}
	dutBgpConf := cfgplugins.ConfigureDUTBGP(t, dut, aggrBatch, cfgBGP)
	configureDUTBGPNeighbors(t, dut, aggrBatch, dutBgpConf.Bgp)
	aggrBatch.Set(t, dut)

	return aggIDsList
}

// configureDutWithGueDecap configure DUT with decapsulation UDP port
func configureDutWithGueDecap(t *testing.T, dut *ondatra.DUTDevice, ipType string) {
	t.Logf("Configure DUT with decapsulation UDP port %v", decapPort)
	ocPFParams := defaultOcPolicyForwardingParams(t, dut, ipType)
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
}

// configureDUTBGPNeighbors appends multiple BGP neighbor configurations to an existing BGP protocol on the DUT. Instead of calling AppendBGPNeighbor repeatedly in the test, this helper iterates over a slice of BGPNeighborConfig and applies each neighbor configuration into the given gnmi.SetBatch.
func configureDUTBGPNeighbors(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, bgp *oc.NetworkInstance_Protocol_Bgp) {
	t.Helper()
	// Add BGP neighbors
	neighbors := []cfgplugins.BGPNeighborConfig{
		{
			AteAS:        ate1AS,
			PortName:     dutP1.Name,
			NeighborIPv4: ateP1.IPv4,
			NeighborIPv6: ateP1.IPv6,
			IsLag:        false,
		},
		{
			AteAS:        ate5AS,
			PortName:     dutP7.Name,
			NeighborIPv4: ateP7.IPv4,
			NeighborIPv6: ateP7.IPv6,
			IsLag:        false,
		},
		{
			AteAS:        ate2AS,
			PortName:     dutP2.Name,
			NeighborIPv4: ateP2.IPv4,
			NeighborIPv6: ateP2.IPv6,
			IsLag:        false,
		},
		{
			AteAS:        ate3AS,
			PortName:     dutLag1.Name,
			NeighborIPv4: ateLag1.IPv4,
			NeighborIPv6: ateLag1.IPv6,
			IsLag:        true,
		},
		{
			AteAS:        ate4AS,
			PortName:     dutLag2.Name,
			NeighborIPv4: ateLag2.IPv4,
			NeighborIPv6: ateLag2.IPv6,
			IsLag:        true,
		},
	}
	for _, n := range neighbors {
		cfgplugins.AppendBGPNeighbor(t, dut, batch, bgp, n)
	}
}

// defaultOcPolicyForwardingParams provides default parameters for the generator, matching the values in the provided JSON example.
func defaultOcPolicyForwardingParams(t *testing.T, dut *ondatra.DUTDevice, ipType string) cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         dut.Port(t, "port1").Name(),
		AppliedPolicyName:   policyName,
		TunnelIP:            dutLo0.IPv6,
		GUEPort:             uint32(decapPort),
		IPType:              ipType,
		Dynamic:             true,
	}
}

// configureATE builds and returns the OTG configuration for the ATE topology.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	ateConfig := gosnappi.NewConfig()

	// Create ATE Ports
	ate1p1 := ate.Port(t, "port1")
	ate2p1 := ate.Port(t, "port2")
	ate5p7 := ate.Port(t, "port7")

	// First, define OTG ports
	ate1Port := ateConfig.Ports().Add().SetName(ate1p1.ID())
	ate2Port := ateConfig.Ports().Add().SetName(ate2p1.ID())
	ate5Port := ateConfig.Ports().Add().SetName(ate5p7.ID())
	// ATE Device 1 (EBGP)
	configureATEDevice(t, ateConfig, ate1Port, ateP1, dutP1, ate1AS, loopbackPfxLen, true, true, true, host1IPv4Start, host1IPv6Start, ate1LoopbackIP, ateSysID+"1")
	// ATE Device 2 (IBGP)
	configureATEDevice(t, ateConfig, ate2Port, ateP2, dutP2, ate2AS, loopbackPfxLen, false, false, true, host2IPv4Start, host2IPv6Start, ate1LoopbackIP, ateSysID+"2")
	// ATE LAG1 (IBGP)
	ateAggPorts1 := []*ondatra.Port{
		ate.Port(t, "port3"),
		ate.Port(t, "port4"),
	}
	configureLAGDevice(t, ateConfig, ateLag1, dutLag1, ateAggPorts1, 1, ate3AS, false, true, "lag1", host2IPv4Start, host2IPv6Start, host3IPv4Start, host3IPv6Start)
	// ATE LAG2 (EBGP)
	ateAggPorts2 := []*ondatra.Port{
		ate.Port(t, "port5"),
		ate.Port(t, "port6"),
	}
	configureLAGDevice(t, ateConfig, ateLag2, dutLag2, ateAggPorts2, 2, ate4AS, true, false, "lag2", host4IPv4Start, host4IPv6Start, "", "")
	configureATEDevice(t, ateConfig, ate5Port, ateP7, dutP7, ate5AS, loopbackPfxLen, true, false, false, host4IPv4Start, host4IPv6Start, ate1LoopbackIP, ateSysID+"2")
	return ateConfig
}

// configureATEDevice configures the ports along with the associated protocols.
func configureATEDevice(t *testing.T, cfg gosnappi.Config, port gosnappi.Port, atePort, dutPort attrs.Attributes, asn, loopbackPrefixLen uint32, isEBGP, loopbacks, isisConfig bool, hostPrefixV4, hostPrefixV6, loopbackPrefix, sysID string) {
	t.Helper()
	var peerTypeV4 gosnappi.BgpV4PeerAsTypeEnum
	var peerTypeV6 gosnappi.BgpV6PeerAsTypeEnum

	dev := cfg.Devices().Add().SetName(atePort.Name)
	eth := dev.Ethernets().Add().SetName(atePort.Name + "Eth").SetMac(atePort.MAC)
	eth.Connection().SetPortName(port.Name())

	ip4 := eth.Ipv4Addresses().Add().SetName(atePort.Name + ".IPv4")
	ip4.SetAddress(atePort.IPv4).SetGateway(dutPort.IPv4).SetPrefix(uint32(atePort.IPv4Len))

	ip6 := eth.Ipv6Addresses().Add().SetName(atePort.Name + ".IPv6")
	ip6.SetAddress(atePort.IPv6).SetGateway(dutPort.IPv6).SetPrefix(uint32(atePort.IPv6Len))

	bgp := dev.Bgp().SetRouterId(atePort.IPv4)
	if isEBGP {
		peerTypeV4 = gosnappi.BgpV4PeerAsType.EBGP
		peerTypeV6 = gosnappi.BgpV6PeerAsType.EBGP
	} else {
		peerTypeV4 = gosnappi.BgpV4PeerAsType.IBGP
		peerTypeV6 = gosnappi.BgpV6PeerAsType.IBGP
	}

	bgpV4 := bgp.Ipv4Interfaces().Add().SetIpv4Name(ip4.Name())
	v4Peer := bgpV4.Peers().Add().SetName(atePort.Name + ".BGPv4.Peer").SetPeerAddress(dutPort.IPv4).SetAsNumber(asn).SetAsType(peerTypeV4)

	bgpV6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ip6.Name())
	v6Peer := bgpV6.Peers().Add().SetName(atePort.Name + ".BGPv6.Peer").SetPeerAddress(dutPort.IPv6).SetAsNumber(asn).SetAsType(peerTypeV6)

	// Advertise host routes
	addBGPRoutes(v4Peer.V4Routes().Add(), atePort.Name+".Host.v4", hostPrefixV4, advertisedIPv4PfxLen, flowCount, ip4.Address())
	addBGPRoutes(v6Peer.V6Routes().Add(), atePort.Name+".Host.v6", hostPrefixV6, advertisedIPv6PfxLen, flowCount, ip6.Address())

	if loopbacks {
		addBGPRoutes(v4Peer.V4Routes().Add(), atePort.Name+".Loopbacks.v4", loopbackPrefix, loopbackPrefixLen, flowCount, ip4.Address())
	}
	if isisConfig {
		configureISIS(dev, ip4.Address(), eth.Name(), []string{atePort.IPv4 + "/" + strconv.Itoa(plenIPv4)}, []string{atePort.IPv6 + "/" + strconv.Itoa(plenIPv6)}, sysID)
	}
}

// configureLAGDevice configures the Lags along with the associated protocols.
func configureLAGDevice(t *testing.T, ateConfig gosnappi.Config, lagAttrs attrs.Attributes, dutAttrs attrs.Attributes, atePorts []*ondatra.Port, lagID, asn uint32, isEBGP, isISIS bool, lagName, hostPrefixV4, hostPrefixV6, host3PrefixV4, host3PrefixV6 string) {
	t.Helper()
	lag := ateConfig.Lags().Add().SetName(lagName)
	lag.Protocol().Static().SetLagId(lagID)

	for i, p := range atePorts {
		port := ateConfig.Ports().Add().SetName(p.ID())
		mac, err := incrementMAC(lagAttrs.MAC, i+1)
		if err != nil {
			t.Fatal(err)
		}
		lag.Ports().Add().SetPortName(port.Name()).Ethernet().SetMac(mac).SetName("LAGMember" + strconv.Itoa(i+1))
	}

	dev := ateConfig.Devices().Add().SetName(lagName + ".Dev")
	eth := dev.Ethernets().Add().SetName(lagAttrs.Name + "Eth-" + lagName).SetMac(lagAttrs.MAC)
	eth.Connection().SetLagName(lagName)

	ipv4 := eth.Ipv4Addresses().Add().SetName(lagAttrs.Name + ".IPv4")
	ipv4.SetAddress(lagAttrs.IPv4).SetGateway(dutAttrs.IPv4).SetPrefix(uint32(lagAttrs.IPv4Len))

	ipv6 := eth.Ipv6Addresses().Add().SetName(lagAttrs.Name + ".IPv6")
	ipv6.SetAddress(lagAttrs.IPv6).SetGateway(dutAttrs.IPv6).SetPrefix(uint32(lagAttrs.IPv6Len))

	bgp := dev.Bgp().SetRouterId(lagAttrs.IPv4)
	peerTypeV4 := gosnappi.BgpV4PeerAsType.IBGP
	peerTypeV6 := gosnappi.BgpV6PeerAsType.IBGP
	if isEBGP {
		peerTypeV4 = gosnappi.BgpV4PeerAsType.EBGP
		peerTypeV6 = gosnappi.BgpV6PeerAsType.EBGP
	}

	bgpV4 := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name())
	v4Peer := bgpV4.Peers().Add().SetName(lagAttrs.Name + ".BGPv4.Peer").SetPeerAddress(dutAttrs.IPv4).SetAsNumber(asn).SetAsType(peerTypeV4)

	bgpV6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name())
	v6Peer := bgpV6.Peers().Add().SetName(lagAttrs.Name + ".BGPv6.Peer").SetPeerAddress(dutAttrs.IPv6).SetAsNumber(asn).SetAsType(peerTypeV6)

	if host3PrefixV4 != "" {
		addBGPRoutes(v4Peer.V4Routes().Add(), ateLag1.Name+".Host2.v4", hostPrefixV4, advertisedIPv4PfxLen, flowCount, ipv4.Address())
		addBGPRoutes(v6Peer.V6Routes().Add(), ateLag1.Name+".Host2.v6", hostPrefixV6, advertisedIPv6PfxLen, flowCount, ipv6.Address())
		addBGPRoutes(v4Peer.V4Routes().Add(), ateLag1.Name+".Host3.v4", host3PrefixV4, advertisedIPv4PfxLen, flowCount, ipv4.Address())
		addBGPRoutes(v6Peer.V6Routes().Add(), ateLag1.Name+".Host3.v6", host3PrefixV6, advertisedIPv6PfxLen, flowCount, ipv6.Address())
	} else {
		addBGPRoutes(v4Peer.V4Routes().Add(), ateLag2.Name+".Host4.v4", hostPrefixV4, advertisedIPv4PfxLen, flowCount, ipv4.Address())
		addBGPRoutes(v6Peer.V6Routes().Add(), ateLag2.Name+".Host4.v6", hostPrefixV6, advertisedIPv6PfxLen, flowCount, ipv6.Address())
	}
	if isISIS {
		isis3LoopbackV4Net := []string{ate3Lo.IPv4 + "/" + strconv.Itoa(int(ate3Lo.IPv4Len))}
		isis3LoopbackV6Net := []string{ate3Lo.IPv6 + "/" + strconv.Itoa(int(ate3Lo.IPv6Len))}
		configureISIS(dev, ipv4.Address(), eth.Name(), append(isis3LoopbackV4Net, ateLag1.IPv4+"/"+strconv.Itoa(int(plenIPv4))), append(isis3LoopbackV6Net, ateLag1.IPv6+"/"+strconv.Itoa(int(plenIPv6))), ateSysID+"3")
	}
}

// addBGPRoutes adds BGP route advertisements to an ATE device.
func addBGPRoutes[R any](routes R, name, startAddress string, prefixLen, count uint32, nextHop string) {
	switch r := any(routes).(type) {
	case gosnappi.BgpV4RouteRange:
		r.SetName(name).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).SetNextHopIpv4Address(nextHop)
		r.Addresses().Add().SetAddress(startAddress).SetPrefix(prefixLen).SetCount(count)
	case gosnappi.BgpV6RouteRange:
		r.SetName(name).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).SetNextHopIpv6Address(nextHop)
		r.Addresses().Add().SetAddress(startAddress).SetPrefix(prefixLen).SetCount(count)
	}
}

// configureISIS adds ISIS configuration to an ATE device.
func configureISIS(dev gosnappi.Device, routerID, ifName string, ipv4Nets, ipv6Nets []string, dutsysID string) {
	isis := dev.Isis().SetName(dev.Name() + ".ISIS").SetSystemId(dutsysID)
	isis.Basic().SetIpv4TeRouterId(routerID).SetHostname(dev.Name())

	// ISIS Interface Config
	isis.Interfaces().Add().SetEthName(ifName).SetName(dev.Name() + "IsisInt").SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	// Advertise IPv4 routes
	for i, net := range ipv4Nets {
		parts := strings.Split(net, "/")
		addr := parts[0]
		prefix, _ := strconv.Atoi(parts[1])
		isis.V4Routes().Add().SetName(fmt.Sprintf("%s.isis.v4net%d", dev.Name(), i)).Addresses().Add().SetAddress(addr).SetPrefix(uint32(prefix))
	}

	// Advertise IPv6 routes
	for i, net := range ipv6Nets {
		parts := strings.Split(net, "/")
		addr := parts[0]
		prefix, _ := strconv.Atoi(parts[1])
		isis.V6Routes().Add().SetName(fmt.Sprintf("%s.isis.v6net%d", dev.Name(), i)).Addresses().Add().SetAddress(addr).SetPrefix(uint32(prefix))
	}
}

// Validate all BGP neighbors are in ESTABLISHED state
func checkBgpStatus(t *testing.T, dut *ondatra.DUTDevice, neighbors []Neighbor) {
	t.Helper()
	t.Log("Verifying BGP neighbor sessions (IPv4 and IPv6)")

	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for idx, nbr := range neighbors {
		t.Logf("Checking BGP IPv4 neighbor %s (Neighbor %d)", nbr.IPv4, idx+1)
		nbrPath := bgpPath.Neighbor(nbr.IPv4)

		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute,
			func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
				currState, present := val.Val()
				return present && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP IPv4 state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatalf("BGP IPv4 session to neighbor %s not ESTABLISHED as expected", nbr.IPv4)
		}
		t.Logf("BGP IPv4 neighbor %s ESTABLISHED", nbr.IPv4)

		t.Logf("Checking BGP IPv6 neighbor %s (Neighbor %d)", nbr.IPv6, idx+1)
		nbrPathv6 := bgpPath.Neighbor(nbr.IPv6)

		_, ok = gnmi.Watch(t, dut, nbrPathv6.SessionState().State(), 2*time.Minute,
			func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
				currState, present := val.Val()
				return present && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
			}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP IPv6 state", nbrPathv6.State(), gnmi.Get(t, dut, nbrPathv6.State()))
			t.Fatalf("BGP IPv6 session to neighbor %s not ESTABLISHED as expected", nbr.IPv6)
		}
		t.Logf("BGP IPv6 neighbor %s ESTABLISHED", nbr.IPv6)
	}

	t.Log("All BGP IPv4 and IPv6 neighbors are ESTABLISHED.")
}

// incrementMAC takes a MAC address in string form (e.g., "02:42:ac:11:00:02") and increments it by the given integer offset `i`.
func incrementMAC(mac string, i int) (string, error) {
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
}

// configureFlows configure traffic streams as per the provided input.
func configureFlows(t *testing.T, otgConfig gosnappi.Config, macAddress string, dstPorts []string, incr int, immediateHeader bool) gosnappi.Flow {
	t.Helper()
	t.Logf("Adding Traffic Stream: %s", "Flow-"+strconv.Itoa(incr))
	flow := otgConfig.Flows().Add().SetName("Flow-" + strconv.Itoa(incr))
	flow.Metrics().SetEnable(true)

	// Build lookup maps for port and lag names
	portNames := map[string]bool{}
	for _, p := range otgConfig.Ports().Items() {
		portNames[p.Name()] = true
	}
	lagNames := map[string]bool{}
	for _, l := range otgConfig.Lags().Items() {
		lagNames[l.Name()] = true
	}

	// Separate destination ports into port-only and lag-only lists
	var portDests, lagDests []string
	for _, dst := range dstPorts {
		if portNames[dst] {
			portDests = append(portDests, dst)
		} else if lagNames[dst] {
			lagDests = append(lagDests, dst)
		} else {
			t.Fatalf("Flow %s: destination %q is neither a port nor a LAG", flow.Name(), dst)
		}
	}

	// Configure TxRx based on endpoint types
	// In gosnappi, TxRx().Port() only accepts port names.
	// TxRx().Device() can handle both port and LAG names.
	txName := otgConfig.Ports().Items()[0].Name()

	if len(portDests) > 0 && len(lagDests) == 0 {
		// All destinations are ports: use Port mode for efficiency
		flow.TxRx().Port().SetTxName(txName).SetRxNames(portDests)
	} else if len(lagDests) > 0 && len(portDests) == 0 {
		// All destinations are LAGs: use Port mode with LAG names
		// In gosnappi, SetRxNames accepts LAG object names at the flow level
		flow.TxRx().Port().SetTxName(txName).SetRxNames(lagDests)
	} else if len(portDests) > 0 && len(lagDests) > 0 {
		// Mixed: combine all destinations and pass to Port mode
		// gosnappi Port.SetRxNames can accept a mix of port and LAG names
		allDests := append(portDests, lagDests...)
		flow.TxRx().Port().SetTxName(txName).SetRxNames(allDests)
	}
	flow.Size().SetFixed(trafficFrameSize)
	flow.Duration().FixedPackets().SetPackets(fixedPackets)
	flow.Rate().SetPercentage(ratePercent)
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(ateP1.MAC)
	eth.Dst().SetValue(macAddress)

	ipOuter := flow.Packet().Add().Ipv6()
	ipOuter.Src().SetValue(ateP1.IPv6)
	if incr == 11 || incr == 12 {
		ipOuter.Dst().SetValue(ateP2.IPv6)
	} else {
		ipOuter.Dst().SetValue(dutLo0.IPv6)
	}
	udpOuter := flow.Packet().Add().Udp()
	if immediateHeader {
		udpOuter.SrcPort().SetValue(UDPSrcPort)
	} else {
		udpOuter.SrcPort().Increment().SetStart(UDPSrcPort).SetStep(1).SetCount(1000)
	}
	if incr == 13 || incr == 14 {
		udpOuter.DstPort().SetValue(UDPDstPortNeg)
	} else {
		udpOuter.DstPort().SetValue(UDPDstPort)
	}
	udpOuter.Checksum().SetCustom(0)

	// Flow-specific configuration from image table
	switch incr {
	case 1, 6:
		// For flow types 1 and 6, the README requires the middle IPv4/UDP
		// header to immediately follow the outer IPv6/UDP header, with MPLS
		// carried inside that middle header.
		ipMiddle := flow.Packet().Add().Ipv4()
		ipMiddle.Src().SetValue(ate1LoopbackIP)
		ipMiddle.Dst().SetValue(ateLag1.IPv4)

		udpMiddle := flow.Packet().Add().Udp()
		if immediateHeader {
			udpMiddle.SrcPort().SetValue(UDPSrcPort - 1)
		} else {
			udpMiddle.SrcPort().Increment().SetStart(UDPSrcPort - 1).SetStep(1).SetCount(1000)
		}
		udpMiddle.DstPort().SetValue(UDPDstPort)

		mpls := flow.Packet().Add().Mpls()
		mpls.Label().SetValue(mplsLabel)

		if incr == 1 {
			ipInner := flow.Packet().Add().Ipv4()
			ipInner.Src().SetValue(constH1v4)
			ipInner.Dst().SetValue(constH3v4)
		} else {
			ipInner := flow.Packet().Add().Ipv6()
			ipInner.Src().SetValue(constH1v6)
			ipInner.Dst().SetValue(constH3v6)
		}
		tcp := flow.Packet().Add().Tcp()
		tcp.SrcPort().SetValue(testSrcPort)
		tcp.DstPort().SetValue(testDstPort)
	case 2, 4:
		ipInner := flow.Packet().Add().Ipv4()
		ipInner.Src().SetValue(constH1v4)
		if incr == 2 {
			ipInner.Dst().SetValue(constH2v4)
		} else {
			ipInner.Dst().SetValue(constH4v4)
		}
		udp := flow.Packet().Add().Udp()
		udp.SrcPort().Increment().SetStart(testSrcPort).SetStep(1).SetCount(10)
		udp.DstPort().SetValue(testDstPort)
	case 3, 5:
		ipInner := flow.Packet().Add().Ipv4()
		ipInner.Src().SetValue(constH1v4)
		if incr == 3 {
			ipInner.Dst().SetValue(constH2v4)
		} else {
			ipInner.Dst().SetValue(constH4v4)
		}
		tcp := flow.Packet().Add().Tcp()
		tcp.SrcPort().Increment().SetStart(testSrcPort).SetStep(1).SetCount(10)
		tcp.DstPort().SetValue(testDstPort)
	case 7, 9:
		ipInner := flow.Packet().Add().Ipv6()
		ipInner.Src().SetValue(constH1v6)
		if incr == 7 {
			ipInner.Dst().SetValue(constH2v6)
		} else {
			ipInner.Dst().SetValue(constH4v6)
		}
		udp := flow.Packet().Add().Udp()
		udp.SrcPort().Increment().SetStart(UDPSrcPort - 1).SetStep(1).SetCount(1000)
		udp.DstPort().SetValue(UDPSrcPort - 2)
	case 8, 10:
		ipInner := flow.Packet().Add().Ipv6()
		ipInner.Src().SetValue(constH1v6)
		if incr == 8 {
			ipInner.Dst().SetValue(constH2v6)
		} else {
			ipInner.Dst().SetValue(constH4v6)
		}
		tcp := flow.Packet().Add().Tcp()
		tcp.SrcPort().Increment().SetStart(testSrcPort).SetStep(1).SetCount(10)
		tcp.DstPort().SetValue(testDstPort)
	case 11, 12, 13, 14:
		ipInner := flow.Packet().Add().Ipv6()
		ipInner.Src().SetValue(constH1v6)
		ipInner.Dst().SetValue(constH4v6)
		tcp := flow.Packet().Add().Tcp()
		tcp.SrcPort().SetValue(testSrcPort)
		tcp.DstPort().SetValue(testDstPort)
	}
	return flow
}

// verifyFlowTraffic validate the traffic stream counts.
func verifyFlowTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flowName, rxPort string, rxLags []string) bool {
	t.Helper()
	expectedTx, ok := flowExpectedPackets(config, flowName)
	if !ok {
		t.Errorf("Flow %q: expected packet count not found in config", flowName)
		return false
	}

	transmitStopped, txPkts, rxPkts := getFinalFlowCounters(t, ate, flowName)
	tolerancePkts := expectedTx * tolerance / 100
	txInRange := uint64WithinTolerance(txPkts, expectedTx, tolerancePkts)
	rxInRange := uint64WithinTolerance(rxPkts, expectedTx, tolerancePkts)
	rxMatchesTx := uint64WithinTolerance(rxPkts, txPkts, tolerancePkts)
	if !txInRange || !rxInRange || !rxMatchesTx {
		logFlowFailureDiagnostics(t, ate, flowName, rxPort, rxLags)
		t.Errorf("Flow %q validation failed: expectedTX=%d actualTX=%d actualRX=%d transmitStopped=%t", flowName, expectedTx, txPkts, rxPkts, transmitStopped)
		if !transmitStopped {
			t.Errorf("Flow %q never reached a stable stopped state before validation", flowName)
		}
		if !txInRange {
			t.Errorf("Flow %q TX mismatch: got %d, want %d +/- %d", flowName, txPkts, expectedTx, tolerancePkts)
		}
		if txInRange && !rxInRange {
			t.Errorf("Flow %q delivery mismatch: RX got %d, want %d +/- %d", flowName, rxPkts, expectedTx, tolerancePkts)
		}
		if !rxMatchesTx {
			t.Errorf("Flow %q RX/TX mismatch: RX got %d, TX got %d, tolerance %d", flowName, rxPkts, txPkts, tolerancePkts)
		}
		return false
	}

	t.Logf("Flow %q: expectedTX=%d TX=%d RX=%d transmitStopped=%t", flowName, expectedTx, txPkts, rxPkts, transmitStopped)
	return true
}

// testLoadBalance computes destination percentages across explicit RX ports.
// It also verifies no packet loss by comparing the total physical-port delta
// against the expected fixedPackets count. This avoids relying on OTG flow
// rx counters, which are unreliable when LAG names are used in Port-mode TxRx.
func testLoadBalance(t *testing.T, ate *ondatra.ATEDevice, flowName string, rxPorts []string, portFramesBefore map[string]uint64) []uint64 {
	t.Helper()

	deltas := make([]uint64, len(rxPorts))
	var total uint64
	for idx, portName := range rxPorts {
		before, ok := portFramesBefore[portName]
		if !ok {
			t.Fatalf("Flow %s: missing baseline counter for port %s", flowName, portName)
		}
		after := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(portName).Counters().InFrames().State())

		// If counter goes backward, it likely means OTG counters were reset between
		// baseline capture and measurement. Treat this as 0 delta for this bucket.
		if after < before {
			t.Logf("Flow %s: port %s counter went backward (before=%d after=%d); treating as counter reset, delta=0", flowName, portName, before, after)
			deltas[idx] = 0
		} else {
			delta := after - before
			deltas[idx] = delta
		}
		total += deltas[idx]
		t.Logf("Load-balance raw counters for port %s: before=%d after=%d delta=%d", portName, before, after, deltas[idx])
	}
	if total == 0 {
		t.Errorf("Flow %s: load-balance destination deltas were zero — no traffic received on any port", flowName)
		return make([]uint64, len(rxPorts))
	}
	// Use fixedPackets constant as the expected total — this is reliable regardless
	// of OTG flow rx tracking limitations with LAG endpoints in Port mode.
	expectedTotal := uint64(fixedPackets)
	if tol := expectedTotal * tolerance / 100; !uint64WithinTolerance(total, expectedTotal, tol) {
		t.Errorf("Flow %s: packet loss detected — physical port delta total=%d, expected=%d, tolerance=%d", flowName, total, expectedTotal, tol)
	}
	weights := make([]uint64, len(rxPorts))
	for idx, delta := range deltas {
		weights[idx] = (delta * 100) / total
		t.Logf("Load-balance percentage for port %s on flow %s: %d%%", rxPorts[idx], flowName, weights[idx])
	}
	return weights
}

// expectedPortWeightsForFlow maps README flow distribution requirements to concrete OTG receive ports.
func expectedPortWeightsForFlow(t *testing.T, otgConfig gosnappi.Config, flowIndex int, rxPort string, rxLags []string) ([]string, []uint64) {
	t.Helper()
	if len(rxLags) != 1 {
		t.Fatalf("Flow %d: expected exactly one receive LAG, got %d", flowIndex, len(rxLags))
	}
	lagName := rxLags[0]

	var lagMemberPorts []string
	for _, lag := range otgConfig.Lags().Items() {
		if lag.Name() != lagName {
			continue
		}
		for _, member := range lag.Ports().Items() {
			lagMemberPorts = append(lagMemberPorts, member.PortName())
		}
		break
	}
	if len(lagMemberPorts) != 2 {
		t.Fatalf("Flow %d: expected 2 member ports for LAG %s, got %d", flowIndex, lagName, len(lagMemberPorts))
	}

	switch flowIndex {
	case 1, 6:
		// README PF-1.22.1: H3 traffic is via ATE3 and must balance across LAG1 members.
		return lagMemberPorts, []uint64{50, 50}
	case 2, 3, 7, 8:
		// README PF-1.22.1: H2 traffic balances across ATE2 and ATE3; ATE3 share balances across LAG1 members.
		if rxPort == "" {
			t.Fatalf("Flow %d: expected non-empty RX port for H2 distribution check", flowIndex)
		}
		return append([]string{rxPort}, lagMemberPorts...), []uint64{50, 25, 25}
	case 4, 5, 9, 10:
		// README PF-1.22.1: H4 traffic balances across ATE5 and ATE4; ATE4 share balances across LAG2 members.
		if rxPort == "" {
			t.Fatalf("Flow %d: expected non-empty RX port for H4 distribution check", flowIndex)
		}
		return append([]string{rxPort}, lagMemberPorts...), []uint64{50, 25, 25}
	default:
		t.Fatalf("Unsupported flow index %d for baseline load-balance expectation", flowIndex)
		return nil, nil
	}
}

func flowExpectedPackets(config gosnappi.Config, flowName string) (uint64, bool) {
	for _, flow := range config.Flows().Items() {
		if flow.Name() == flowName {
			return uint64(flow.Duration().FixedPackets().Packets()), true
		}
	}
	return 0, false
}

func getFinalFlowCounters(t *testing.T, ate *ondatra.ATEDevice, flowName string) (bool, uint64, uint64) {
	t.Helper()
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	_, transmitStopped := gnmi.Watch(t, ate.OTG(), transmitPath, timeout, func(val *ygnmi.Value[bool]) bool {
		transmit, ok := val.Val()
		return ok && !transmit
	}).Await(t)
	flowState := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
	txPkts := flowState.GetCounters().GetOutPkts()
	rxPkts := flowState.GetCounters().GetInPkts()
	return transmitStopped, txPkts, rxPkts
}

func uint64WithinTolerance(got, want, tolerance uint64) bool {
	if got > want {
		return got-want <= tolerance
	}
	return want-got <= tolerance
}

func logFlowFailureDiagnostics(t *testing.T, ate *ondatra.ATEDevice, flowName, rxPort string, rxLags []string) {
	t.Helper()
	flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
	t.Logf("Flow %q failure metrics: transmit=%t outPkts=%d inPkts=%d", flowName, flowMetrics.GetTransmit(), flowMetrics.GetCounters().GetOutPkts(), flowMetrics.GetCounters().GetInPkts())
	if rxPort != "" {
		portMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(rxPort).State())
		t.Logf("Port %q failure metrics: inFrames=%d outFrames=%d", rxPort, portMetrics.GetCounters().GetInFrames(), portMetrics.GetCounters().GetOutFrames())
	}
	for _, lagName := range rxLags {
		lagMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Lag(lagName).State())
		t.Logf("LAG %q failure metrics: inFrames=%d outFrames=%d operStatus=%s", lagName, lagMetrics.GetCounters().GetInFrames(), lagMetrics.GetCounters().GetOutFrames(), lagMetrics.GetOperStatus())
	}
}

// verifyTrafficFlowNegCase checks whether the observed packet loss for a flow is within acceptable tolerance.
func verifyTrafficFlowNegCase(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flow gosnappi.Flow) bool {
	t.Helper()
	otgutils.LogFlowMetrics(t, ate.OTG(), config)
	flowState := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).State())
	rxPkts := flowState.GetCounters().GetInPkts()
	txPkts := flowState.GetCounters().GetOutPkts()
	lostPkt := txPkts - rxPkts
	if got := (lostPkt * 100 / txPkts); got >= tolerance {
		return false
	}
	return true
}

// verifySinglePathTraffic validates that traffic follows a single expected path without load balancing across multiple ports.
func verifySinglePathTraffic(t *testing.T, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	t.Helper()
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)
	portList := []string{
		otgConfig.Ports().Items()[1].Name(), // primary destination port
		otgConfig.Ports().Items()[2].Name(), // alternative path
		otgConfig.Ports().Items()[3].Name(),
	}
	aggNames := []string{otgConfig.Lags().Items()[0].Name(), otgConfig.Lags().Items()[1].Name()}
	totalRx := uint64(0)
	nonZeroPorts := 0
	for _, port := range portList {
		rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(port).Counters().InFrames().State())
		t.Logf("Port %s received %d packets", port, rxPkts)
		if rxPkts > tolerance {
			nonZeroPorts++
		}
		totalRx += rxPkts
	}
	for _, aggName := range aggNames {
		metrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Lag(aggName).State())
		inFrames := metrics.GetCounters().GetInFrames()
		t.Logf("Lag %s received %d packets", aggName, inFrames)
		if inFrames > tolerance {
			nonZeroPorts++
		}
	}
	if nonZeroPorts > tolerance {
		t.Fatalf("Expected traffic to follow a single path, but received on %d ports", nonZeroPorts)
	} else {
		t.Logf("PASS: All traffic followed a single path as expected")
	}
}

// waitForOTGProtocolsUpWithRetry waits for all OTG ports and LAGs to reach an operational UP state within the given timeout.
func waitForOTGProtocolsUpWithRetry(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, pushStartWaitTime time.Duration, strict bool) error {
	t.Helper()
	attempt := "initial"
	if strict {
		attempt = "restart"
	}

	t.Log("Waiting for OTG BGP peers to establish...")
	for _, dev := range config.Devices().Items() {
		for _, ip := range dev.Bgp().Ipv4Interfaces().Items() {
			for _, peer := range ip.Peers().Items() {
				peerName := peer.Name()
				path := gnmi.OTG().BgpPeer(peerName)
				_, ok := gnmi.Watch(t, ate.OTG(), path.SessionState().State(), pushStartWaitTime, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
					state, present := val.Val()
					return present && state == otgtelemetry.BgpPeer_SessionState_ESTABLISHED
				}).Await(t)
				if !ok {
					return fmt.Errorf("%s attempt: OTG BGPv4 peer %q did not reach ESTABLISHED, got %v", attempt, peerName, gnmi.Get(t, ate.OTG(), path.SessionState().State()))
				}
			}
		}
		for _, ip := range dev.Bgp().Ipv6Interfaces().Items() {
			for _, peer := range ip.Peers().Items() {
				peerName := peer.Name()
				path := gnmi.OTG().BgpPeer(peerName)
				_, ok := gnmi.Watch(t, ate.OTG(), path.SessionState().State(), pushStartWaitTime, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
					state, present := val.Val()
					return present && state == otgtelemetry.BgpPeer_SessionState_ESTABLISHED
				}).Await(t)
				if !ok {
					return fmt.Errorf("%s attempt: OTG BGPv6 peer %q did not reach ESTABLISHED, got %v", attempt, peerName, gnmi.Get(t, ate.OTG(), path.SessionState().State()))
				}
			}
		}
	}

	t.Log("Waiting for OTG ports to be UP...")
	for _, p := range config.Ports().Items() {
		_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Port(p.Name()).Link().State(), pushStartWaitTime,
			func(val *ygnmi.Value[otgtelemetry.E_Port_Link]) bool {
				state, present := val.Val()
				return present && state == otgtelemetry.Port_Link_UP
			}).Await(t)

		if !ok {
			if strict {
				return fmt.Errorf("port %s not UP", p.Name())
			}
			return fmt.Errorf("retry needed: port %s not UP", p.Name())
		}
		t.Logf("Port %s is UP", p.Name())
	}

	t.Log("Waiting for LAGs to be UP...")
	for _, lag := range config.Lags().Items() {
		_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Lag(lag.Name()).OperStatus().State(), pushStartWaitTime,
			func(val *ygnmi.Value[otgtelemetry.E_Lag_OperStatus]) bool {
				state, present := val.Val()
				return present && state == otgtelemetry.Lag_OperStatus_UP
			}).Await(t)

		if !ok {
			if strict {
				return fmt.Errorf("LAG %s not UP", lag.Name())
			}
			return fmt.Errorf("retry needed: LAG %s not UP", lag.Name())
		}
		t.Logf("LAG %s is UP", lag.Name())
	}

	return nil
}

func pushAndStartProtocols(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, pushStartWaitTime time.Duration) {
	t.Helper()
	t.Log("Pushing OTG config...")
	ate.OTG().PushConfig(t, top)
	time.Sleep(5 * time.Second)
	t.Log("Starting protocols...")
	ate.OTG().StartProtocols(t)

	if err := waitForOTGProtocolsUpWithRetry(t, ate, top, pushStartWaitTime, false); err != nil {
		t.Log("Protocols not UP on first attempt, restarting once...")
		// Restart once
		ate.OTG().StopProtocols(t)
		ate.OTG().StartProtocols(t)

		if err := waitForOTGProtocolsUpWithRetry(t, ate, top, pushStartWaitTime, true); err != nil {
			t.Fatalf("Protocols failed to come UP even after restart: %v", err)
		}
	}
	t.Log("Protocols are stable and ready")
}

func capturePortsInFrames(t *testing.T, ate *ondatra.ATEDevice, portNames []string) map[string]uint64 {
	t.Helper()
	frames := make(map[string]uint64, len(portNames))
	for _, portName := range portNames {
		frames[portName] = gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(portName).Counters().InFrames().State())
	}
	return frames
}
