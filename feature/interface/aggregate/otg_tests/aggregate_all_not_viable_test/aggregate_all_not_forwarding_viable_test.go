// Copyright 2024 Google LLC
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

// For LACP. Make the following forwarding-viable transitions on a port within the LAG on the DUT.
// 1. Tag the forwarding-viable=true to allow all the ports to pass traffic in
//    port-channel.
// 2. Transition from forwarding-viable=true to forwarding-viable=false.
// For each condition above, ensure following two things:
// -  traffic is load-balanced across the remaining interfaces in the LAG.
// -  there is no packet tx on port with forwarding-viable=false.
// -  there is packet rx on the port and process it to destination  with forwarding-viable=false.

// What is forwarding viable ?
// If set to false, the interface is not used for forwarding traffic,
// but as long as it is up, the interface still maintains its layer-2 adjacencies and runs its configured layer-2 functions (e.g. LLDP, etc.).

package aggregate_all_not_forwarding_viable_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/ocpath"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PLen          = 30
	ipv6PLen          = 126
	isisInstance      = "DEFAULT"
	dutAreaAddress    = "49.0001"
	ateAreaAddress    = "49"
	dutSysID          = "1920.0000.2001"
	asn               = 64501
	acceptRoutePolicy = "PERMIT-ALL"
	trafficPPS        = 2500000
	srcTrafficV4      = "100.0.1.1"
	srcTrafficV6      = "2002:db8:64:64::1"
	dstTrafficV4      = "100.0.2.1"
	dstTrafficV6      = "2003:db8:64:64::1"
	v4Count           = 254
	v6Count           = 100000000
	lagTypeLACP       = oc.IfAggregate_AggregationType_LACP
	ieee8023adLag     = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	ethernetCsmacd    = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	LAG1              = "lag1"
	LAG2              = "lag2"
	LAG3              = "lag3"
)

type aggPortData struct {
	dutIPv4      string
	ateIPv4      string
	dutIPv6      string
	ateIPv6      string
	ateAggName   string
	ateAggMAC    string
	ateISISSysID string
	ateLagCount  uint32
}

type ipAddr struct {
	ip     string
	prefix uint32
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	top    gosnappi.Config
	ctx    context.Context
	client *fluent.GRIBIClient
}

var (
	agg1 = &aggPortData{
		dutIPv4:      "192.0.2.1",
		ateIPv4:      "192.0.2.2",
		dutIPv6:      "2001:db8::1",
		ateIPv6:      "2001:db8::2",
		ateAggName:   LAG1,
		ateAggMAC:    "02:00:01:01:01:01",
		ateISISSysID: "640000000002",
		ateLagCount:  1,
	}
	agg2 = &aggPortData{
		dutIPv4:      "192.0.2.5",
		ateIPv4:      "192.0.2.6",
		dutIPv6:      "2001:db8::5",
		ateIPv6:      "2001:db8::6",
		ateAggName:   LAG2,
		ateAggMAC:    "02:00:01:01:02:01",
		ateISISSysID: "640000000003",
		ateLagCount:  2,
	}
	agg3 = &aggPortData{
		dutIPv4:      "192.0.2.9",
		ateIPv4:      "192.0.2.10",
		dutIPv6:      "2001:db8::9",
		ateIPv6:      "2001:db8::a",
		ateAggName:   LAG3,
		ateAggMAC:    "02:00:01:01:03:01",
		ateISISSysID: "640000000004",
		ateLagCount:  1,
	}

	dutLoopback = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "192.0.2.21",
		IPv6:    "2001:db8::21",
		IPv4Len: 32,
		IPv6Len: 128,
	}

	pfx1AdvV4                = &ipAddr{ip: "100.0.1.0", prefix: 24}
	pfx1AdvV6                = &ipAddr{ip: "2002:db8:64:64::0", prefix: 64}
	pfx2AdvV4                = &ipAddr{ip: "100.0.2.0", prefix: 24}
	pfx2AdvV6                = &ipAddr{ip: "2003:db8:64:64::0", prefix: 64}
	pfx3AdvV4                = &ipAddr{ip: "100.0.3.0", prefix: 24}
	pfx4AdvV4                = &ipAddr{ip: "100.0.4.0", prefix: 24}
	pmd100GFRPorts           []string
	dutPortList              []*ondatra.Port
	atePortList              []*ondatra.Port
	rxPktsBeforeTraffic      map[*ondatra.Port]uint64
	txPktsBeforeTraffic      map[*ondatra.Port]uint64
	equalDistributionWeights = []uint64{50, 50}
	ecmpTolerance            = uint64(1)
	ipRange                  = []uint32{254, 500}

	dutAggMac []string
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestAggregateAllNotForwardingViable Test forwarding-viable with LAG and routing
func TestAggregateAllNotForwardingViable(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	aggIDs := configureDUT(t, dut)
	changeMetric(t, dut, aggIDs[2], 30)
	top := configureATE(t, ate)
	installGRIBIRoutes(t, dut, ate, top)
	flows := createFlows(t, dut, top, aggIDs)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	for _, aggID := range aggIDs {
		gnmi.Await(t, dut, gnmi.OC().Interface(aggID).OperStatus().State(), 30*time.Second, oc.Interface_OperStatus_UP)
	}

	for _, agg := range []*aggPortData{agg1, agg2, agg3} {
		bgpPath := ocpath.Root().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
		gnmi.Await(t, dut, bgpPath.Neighbor(agg.ateIPv4).SessionState().State(), time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	}

	t.Logf("ISIS cost of LAG_2 lower then ISIS cost of LAG_3 Test-01")
	t.Run("RT-5.7.1.1: Setting Forwarding-Viable to False on Lag2 all ports except port 2", func(t *testing.T) {
		configForwardingViable(t, dut, dutPortList[2:agg2.ateLagCount+1], false)
		startTraffic(t, dut, ate, top)
		if err := checkBidirectionalTraffic(t, dut, dutPortList[1:2]); err != nil {
			t.Fatal(err)
		}
		if err := confirmNonViableForwardingTraffic(t, dut, ate, atePortList[2:agg2.ateLagCount+1], dutPortList[2:agg2.ateLagCount+1]); err != nil {
			t.Fatal(err)
		}
		// Ensure there is no traffic received/transmiited on DUT LAG_3
		if got := validateLag3Traffic(t, dut, ate, dutPortList[(agg2.ateLagCount+1):]); got == true {
			t.Fatal("Packets are Received and Transmitted on LAG_3")
		}
		if ok := verifyTrafficFlow(t, ate, flows, false); !ok {
			t.Fatal("Packet Dropped, LossPct for flow ")
		}
	})
	t.Run("RT-5.7.1.2: Setting Forwarding-Viable to False for Lag2 all ports", func(t *testing.T) {
		// Ensure ISIS Adjacency is up on LAG_2
		if ok := awaitAdjacency(t, dut, aggIDs[1], oc.Isis_IsisInterfaceAdjState_UP); !ok {
			t.Fatal("ISIS Adjacency is Down on LAG_2")
		}
		configForwardingViable(t, dut, dutPortList[1:2], false)
		// Ensure ISIS Adjacency is Down on LAG_2

		if ok := awaitAdjacency(t, dut, aggIDs[1], oc.Isis_IsisInterfaceAdjState_DOWN); !ok {
			t.Fatal("ISIS Adjacency is Established on LAG_2")
		}
		startTraffic(t, dut, ate, top)
		if err := confirmNonViableForwardingTraffic(t, dut, ate, atePortList[1:agg2.ateLagCount+1], dutPortList[1:agg2.ateLagCount+1]); err != nil {
			t.Fatal(err)
		}
		// Ensure that traffic from ATE port1 to pfx4 transmitted out using LAG3
		if ok := verifyTrafficFlow(t, ate, flows[1:2], true); !ok {
			t.Fatal("Packet Dropped, LossPct for flow ", flows[1].Name())
		}
		// Ensure there is no traffic received on DUT LAG_3
		if got := validateLag3Traffic(t, dut, ate, dutPortList[(agg2.ateLagCount+1):]); got == true {
			t.Fatal("Packets are Received on DUT LAG_3")
		}
		if ok := verifyTrafficFlow(t, ate, flows[0:1], true); !ok {
			t.Fatal("Packet Dropped, LossPct for flow ", flows[0].Name())
		}
	})

	t.Run("RT-5.7.1.3: Setting Forwarding-Viable to True for Lag2 one of the port", func(t *testing.T) {
		configForwardingViable(t, dut, dutPortList[agg2.ateLagCount:agg2.ateLagCount+1], true)
		startTraffic(t, dut, ate, top)
		if err := checkBidirectionalTraffic(t, dut, dutPortList[agg2.ateLagCount:agg2.ateLagCount+1]); err != nil {
			t.Fatal(err)
		}
		if err := confirmNonViableForwardingTraffic(t, dut, ate, atePortList[1:agg2.ateLagCount], dutPortList[1:agg2.ateLagCount]); err != nil {
			t.Fatal(err)
		}
		// Ensure there is no traffic received/transmiited on DUT LAG_3
		if got := validateLag3Traffic(t, dut, ate, dutPortList[(agg2.ateLagCount+1):]); got == true {
			t.Fatal("Packets are Received and Transmitted on LAG_3")
		}
		if ok := verifyTrafficFlow(t, ate, flows, false); !ok {
			t.Fatal("Packet Dropped, LossPct for flow ")
		}
	})
	// Reset Forwarding-Viable to True for all the ports of LAG_2
	configForwardingViable(t, dut, dutPortList[1:6], true)
	// Change ISIS metric Equal for Both LAG_2 and LAG_3
	changeMetric(t, dut, aggIDs[2], 20)

	t.Logf("ISIS cost of LAG_2 equal to ISIS cost of LAG_3 Test-02")
	t.Run("RT-5.7.2.1: Setting Forwarding-Viable to False for Lag2 ports except port 2", func(t *testing.T) {
		configForwardingViable(t, dut, dutPortList[2:agg2.ateLagCount+1], false)
		flows = append(flows, configureFlows(t, top, pfx2AdvV4, pfx1AdvV4, "pfx2ToPfx1Lag3", agg3, []*aggPortData{agg1}, dutAggMac[2], ipRange[0]))
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		startTraffic(t, dut, ate, top)
		if err := checkBidirectionalTraffic(t, dut, dutPortList[1:2]); err != nil {
			t.Fatal(err)
		}
		if err := checkBidirectionalTraffic(t, dut, dutPortList[(agg2.ateLagCount+1):]); err != nil {
			t.Fatal(err)
		}
		if err := confirmNonViableForwardingTraffic(t, dut, ate, atePortList[2:(agg2.ateLagCount+1)], dutPortList[2:(agg2.ateLagCount+1)]); err != nil {
			t.Fatal(err)
		}
		// Ensure Load Balancing 50:50 on LAG_2 and LAG_3 for prefix's pfx2, pfx3 and pfx4
		weights := trafficRXWeights(t, ate, []string{agg2.ateAggName, agg3.ateAggName}, flows[0])
		for idx, weight := range equalDistributionWeights {
			if got, want := weights[idx], weight; got < want-ecmpTolerance || got > want+ecmpTolerance {
				t.Errorf("ECMP Percentage for Aggregate Index: %d: got %d, want %d", idx+1, got, want)
			}
		}
		if ok := verifyTrafficFlow(t, ate, flows, false); !ok {
			t.Fatal("Packet Dropped, LossPct for flow ")
		}
	})

	t.Run("RT-5.7.2.2: Setting Forwarding-Viable to False for Lag2 all ports", func(t *testing.T) {
		// Ensure ISIS Adjacency is up on LAG_2
		if ok := awaitAdjacency(t, dut, aggIDs[1], oc.Isis_IsisInterfaceAdjState_UP); !ok {
			t.Fatal("ISIS Adjacency is Down on LAG_2")
		}
		configForwardingViable(t, dut, dutPortList[1:2], false)
		// Ensure ISIS Adjacency is Down on LAG_2
		if ok := awaitAdjacency(t, dut, aggIDs[1], oc.Isis_IsisInterfaceAdjState_DOWN); !ok {
			t.Fatal("ISIS Adjacency is Established on LAG_2")
		}
		startTraffic(t, dut, ate, top)
		if err := confirmNonViableForwardingTraffic(t, dut, ate, atePortList[1:(agg2.ateLagCount+1)], dutPortList[1:(agg2.ateLagCount+1)]); err != nil {
			t.Fatal(err)
		}
		// Ensure that traffic from ATE port1 to pfx4 are discarded on DUT
		if ok := verifyTrafficFlow(t, ate, flows[1:2], true); !ok {
			t.Fatal("Packet Dropped, LossPct for flow ", flows[1].Name())
		}
		// Ensure there is traffic received on DUT LAG_3
		if got := validateLag3Traffic(t, dut, ate, dutPortList[(agg2.ateLagCount+1):]); got == false {
			t.Fatal("Packets are not Received on LAG_3")
		}
		if ok := verifyTrafficFlow(t, ate, flows[0:1], true); !ok {
			t.Fatal("Packet Dropped, LossPct for flow ", flows[0].Name())
		}
	})

	t.Run("RT-5.7.2.3: Setting Forwarding-Viable to True for Lag2 one of the port", func(t *testing.T) {
		configForwardingViable(t, dut, dutPortList[agg2.ateLagCount:(agg2.ateLagCount+1)], true)
		startTraffic(t, dut, ate, top)
		if err := checkBidirectionalTraffic(t, dut, dutPortList[agg2.ateLagCount:]); err != nil {
			t.Fatal(err)
		}
		if err := confirmNonViableForwardingTraffic(t, dut, ate, atePortList[1:agg2.ateLagCount], dutPortList[1:agg2.ateLagCount]); err != nil {
			t.Fatal(err)
		}
		if ok := verifyTrafficFlow(t, ate, flows, false); !ok {
			t.Fatal("Packet Dropped, LossPct for flow ")
		}
	})
}

// configureDUT configures DUT
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) []string {

	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	if len(dut.Ports()) < 4 {
		t.Fatalf("Testbed requires at least 4 ports, got %d", len(dut.Ports()))
	}
	if len(dut.Ports()) > 4 {
		agg2.ateLagCount = uint32(len(dut.Ports()) - 3)
		agg3.ateLagCount = 2
	}
	var aggIDs []string
	for _, a := range []*aggPortData{agg1, agg2, agg3} {
		d := gnmi.OC()
		aggID := netutil.NextAggregateInterface(t, dut)
		aggIDs = append(aggIDs, aggID)
		portList := initializePort(t, dut, a)

		if deviations.AggregateAtomicUpdate(dut) {
			clearAggregate(t, dut, aggID, a, portList)
			setupAggregateAtomically(t, dut, aggID, a, portList)
		}
		lacp := &oc.Lacp_Interface{Name: ygot.String(aggID)}
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
		lacpPath := d.Lacp().Interface(aggID)
		fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
		gnmi.Replace(t, dut, lacpPath.Config(), lacp)

		aggInt := &oc.Interface{Name: ygot.String(aggID)}
		configAggregateDUT(dut, aggInt, a)

		aggPath := d.Interface(aggID)
		fptest.LogQuery(t, aggID, aggPath.Config(), aggInt)
		gnmi.Replace(t, dut, aggPath.Config(), aggInt)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, aggID, deviations.DefaultNetworkInstance(dut), 0)
		}
		for _, port := range portList {
			i := &oc.Interface{Name: ygot.String(port.Name())}
			i.Type = ethernetCsmacd
			e := i.GetOrCreateEthernet()
			e.AggregateId = ygot.String(aggID)
			if deviations.InterfaceEnabled(dut) {
				i.Enabled = ygot.Bool(true)
			}
			if port.PMD() == ondatra.PMD100GBASEFR {
				e.AutoNegotiate = ygot.Bool(false)
				e.DuplexMode = oc.Ethernet_DuplexMode_FULL
				e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
			}

			configMemberDUT(dut, i, port, aggID)
			iPath := d.Interface(port.Name())
			fptest.LogQuery(t, port.String(), iPath.Config(), i)
			gnmi.Replace(t, dut, iPath.Config(), i)
		}

		if deviations.ExplicitPortSpeed(dut) {
			for _, dp := range portList {
				fptest.SetPortSpeed(t, dp)
			}
		}
	}

	configureRoutingPolicy(t, dut)
	configureDUTISIS(t, dut, aggIDs)

	if deviations.MaxEcmpPaths(dut) {
		isisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()
		gnmi.Update(t, dut, isisPath.Global().MaxEcmpPaths().Config(), 2)
	}
	configureDUTBGP(t, dut, aggIDs)
	return aggIDs
}

// configDstMemberDUT enables destination ports, add other details like description,
// port and aggregate ID.
func configMemberDUT(dut *ondatra.DUTDevice, i *oc.Interface, p *ondatra.Port, aggID string) {
	i.Description = ygot.String(p.String())
	i.Type = ethernetCsmacd

	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(aggID)
}

// initializePort initializes ports for aggregate on DUT
func initializePort(t *testing.T, dut *ondatra.DUTDevice, a *aggPortData) []*ondatra.Port {
	var portList []*ondatra.Port
	var portIdx uint32
	switch a.ateAggName {
	case LAG1:
		portList = append(portList, dut.Port(t, fmt.Sprintf("port%d", portIdx+1)))
		dutPortList = append(dutPortList, dut.Port(t, fmt.Sprintf("port%d", portIdx+1)))
	case LAG2:
		for portIdx < a.ateLagCount {
			portList = append(portList, dut.Port(t, fmt.Sprintf("port%d", portIdx+2)))
			dutPortList = append(dutPortList, dut.Port(t, fmt.Sprintf("port%d", portIdx+2)))
			portIdx++
		}
	case LAG3:
		for portIdx < a.ateLagCount {
			portList = append(portList, dut.Port(t, fmt.Sprintf("port%d", portIdx+agg2.ateLagCount+2)))
			dutPortList = append(dutPortList, dut.Port(t, fmt.Sprintf("port%d", portIdx+agg2.ateLagCount+2)))
			portIdx++
		}
	}
	return portList
}

// configDstAggregateDUT configures port-channel destination ports
func configAggregateDUT(dut *ondatra.DUTDevice, i *oc.Interface, a *aggPortData) {
	i.Description = ygot.String(a.ateAggName)
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a4 := s4.GetOrCreateAddress(a.dutIPv4)
	a4.PrefixLength = ygot.Uint8(ipv4PLen)

	// n4 := s4.GetOrCreateNeighbor(a.ateIPv4)
	// n4.LinkLayerAddress = ygot.String(a.ateAggMAC)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.dutIPv6).PrefixLength = ygot.Uint8(ipv6PLen)
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = lagTypeLACP
}

// setupAggregateAtomically setup port-channel based on LAG type.
func setupAggregateAtomically(t *testing.T, dut *ondatra.DUTDevice, aggID string, agg *aggPortData, portList []*ondatra.Port) {
	d := &oc.Root{}
	d.GetOrCreateLacp().GetOrCreateInterface(aggID)

	aggr := d.GetOrCreateInterface(aggID)
	aggr.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	aggr.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	for _, port := range portList {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)

		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
	}
	p := gnmi.OC()
	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", dut), p.Config(), d)
	gnmi.Update(t, dut, p.Config(), d)
}

// clearAggregate delete any previously existing members of aggregate.
func clearAggregate(t *testing.T, dut *ondatra.DUTDevice, aggID string, agg *aggPortData, portList []*ondatra.Port) {
	// Clear the aggregate minlink.
	gnmi.Delete(t, dut, gnmi.OC().Interface(aggID).Aggregation().MinLinks().Config())
	// Clear the members of the aggregate.
	for _, port := range portList {
		resetBatch := &gnmi.SetBatch{}
		gnmi.BatchDelete(resetBatch, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
		gnmi.BatchDelete(resetBatch, gnmi.OC().Interface(port.Name()).ForwardingViable().Config())
		resetBatch.Set(t, dut)
	}
}

// configureDUTBGP configure BGP on DUT
func configureDUTBGP(t *testing.T, dut *ondatra.DUTDevice, aggIDs []string) {
	t.Helper()

	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutLoopback.IPv4)
	global.As = ygot.Uint32(asn)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	pgName := "BGP-PEER-GROUP1"
	pg := bgp.GetOrCreatePeerGroup(pgName)
	pg.PeerAs = ygot.Uint32(asn)
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{acceptRoutePolicy})
		rpl.SetImportPolicy([]string{acceptRoutePolicy})
	} else {
		af4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)
		rpl := af4.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{acceptRoutePolicy})
		rpl.SetImportPolicy([]string{acceptRoutePolicy})

		af6 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(true)
		rpl = af6.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{acceptRoutePolicy})
		rpl.SetImportPolicy([]string{acceptRoutePolicy})
	}

	for _, a := range []*aggPortData{agg1, agg2, agg3} {
		bgpNbrV4 := bgp.GetOrCreateNeighbor(a.ateIPv4)
		bgpNbrV4.PeerGroup = ygot.String(pgName)
		bgpNbrV4.PeerAs = ygot.Uint32(asn)
		bgpNbrV4.Enabled = ygot.Bool(true)
		bgpNbrV4T := bgpNbrV4.GetOrCreateTransport()

		bgpNbrV4T.LocalAddress = ygot.String(a.dutIPv4)
		af4 := bgpNbrV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)

		af6 := bgpNbrV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(false)

		bgpNbrV6 := bgp.GetOrCreateNeighbor(a.ateIPv6)
		bgpNbrV6.PeerGroup = ygot.String(pgName)
		bgpNbrV6.PeerAs = ygot.Uint32(asn)
		bgpNbrV6.Enabled = ygot.Bool(true)
		bgpNbrV6T := bgpNbrV6.GetOrCreateTransport()

		bgpNbrV6T.LocalAddress = ygot.String(a.dutIPv6)
		af4 = bgpNbrV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(false)
		af6 = bgpNbrV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(true)
	}

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), niProto)
}

// configureRoutingPolicy configure routing policy on DUT
func configureRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(acceptRoutePolicy)
	stmt, _ := pdef.AppendNewStatement("20")
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(acceptRoutePolicy).Config(), pdef)
}

// configureDUTISIS configure ISIS on DUT
func configureDUTISIS(t *testing.T, dut *ondatra.DUTDevice, aggIDs []string) {
	t.Helper()
	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()

	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	lspBit := globalISIS.GetOrCreateLspBit().GetOrCreateOverloadBit()
	lspBit.SetBit = ygot.Bool(false)
	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC

	for _, aggID := range aggIDs {
		isisIntf := isis.GetOrCreateInterface(aggID)
		isisIntf.GetOrCreateInterfaceRef().Interface = ygot.String(aggID)
		isisIntf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)

		if deviations.InterfaceRefConfigUnsupported(dut) {
			isisIntf.InterfaceRef = nil
		}

		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		if deviations.ISISInterfaceAfiUnsupported(dut) {
			isisIntf.Af = nil
		}
		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)

		isisIntfLevelAfiv4 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)

		isisIntfLevelAfiv4.Enabled = ygot.Bool(true)
		isisIntfLevelAfiv6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)

		isisIntfLevelAfiv4.Metric = ygot.Uint32(20)
		isisIntfLevelAfiv6.Metric = ygot.Uint32(20)

		isisIntfLevelAfiv6.Enabled = ygot.Bool(true)
		if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
			isisIntfLevelAfiv4.Enabled = nil
			isisIntfLevelAfiv6.Enabled = nil
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), d)

}

// changeMetric change metric for ISIS on Interface
func changeMetric(t *testing.T, dut *ondatra.DUTDevice, intf string, metric uint32) {
	t.Logf("Updating ISIS metric of LAG2 equal to LAG3 ")
	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	isis := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).GetOrCreateIsis()
	isisIntfLevel := isis.GetOrCreateInterface(intf).GetOrCreateLevel(2)
	isisIntfLevelAfiv4 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfiv4.Metric = ygot.Uint32(metric)
	isisIntfLevelAfiv6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfiv6.Metric = ygot.Uint32(metric)

	if deviations.ISISRequireSameL1MetricWithL2Metric(dut) {
		l1 := isis.GetOrCreateInterface(intf).GetOrCreateLevel(1)
		l1V4 := l1.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		l1V4.Metric = ygot.Uint32(metric)
		l1V6 := l1.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		l1V6.Metric = ygot.Uint32(metric)
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}

// configureATE configure ATE
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	for _, a := range []*aggPortData{agg1, agg2, agg3} {
		var portList []*ondatra.Port
		var portIdx uint32
		switch a.ateAggName {
		case LAG1:
			portList = append(portList, ate.Port(t, fmt.Sprintf("port%d", portIdx+1)))
			atePortList = append(atePortList, ate.Port(t, fmt.Sprintf("port%d", portIdx+1)))
		case LAG2:
			for portIdx < a.ateLagCount {
				portList = append(portList, ate.Port(t, fmt.Sprintf("port%d", portIdx+2)))
				atePortList = append(atePortList, ate.Port(t, fmt.Sprintf("port%d", portIdx+2)))
				portIdx++
			}
		case LAG3:
			for portIdx < a.ateLagCount {
				portList = append(portList, ate.Port(t, fmt.Sprintf("port%d", portIdx+agg2.ateLagCount+2)))
				atePortList = append(atePortList, ate.Port(t, fmt.Sprintf("port%d", portIdx+agg2.ateLagCount+2)))
				portIdx++
			}
		}
		configureOTGPorts(t, ate, top, portList, a)
	}
	// Disable FEC for 100G-FR ports because Novus does not support it.
	if len(pmd100GFRPorts) > 0 {
		l1Settings := top.Layer1().Add().SetName("L1").SetPortNames(pmd100GFRPorts)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}
	return top
}

// configureOTGPorts define ATE ports
func configureOTGPorts(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, portList []*ondatra.Port, a *aggPortData) []string {
	agg := top.Lags().Add().SetName(a.ateAggName)
	agg.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId(a.ateAggMAC)
	lagDev := top.Devices().Add().SetName(agg.Name() + ".Dev")
	lagEth := lagDev.Ethernets().Add().SetName(agg.Name() + ".Eth").SetMac(a.ateAggMAC)
	lagEth.Connection().SetLagName(agg.Name())
	lagEth.Ipv4Addresses().Add().SetName(agg.Name() + ".IPv4").SetAddress(a.ateIPv4).SetGateway(a.dutIPv4).SetPrefix(ipv4PLen)
	lagEth.Ipv6Addresses().Add().SetName(agg.Name() + ".IPv6").SetAddress(a.ateIPv6).SetGateway(a.dutIPv6).SetPrefix(ipv6PLen)
	for aggIdx, pList := range portList {
		top.Ports().Add().SetName(pList.ID())
		if pList.PMD() == ondatra.PMD100GBASEFR {
			pmd100GFRPorts = append(pmd100GFRPorts, pList.ID())
		}
		newMac, err := incrementMAC(a.ateAggMAC, aggIdx+1)
		if err != nil {
			t.Fatal(err)
		}
		lagPort := agg.Ports().Add().SetPortName(pList.ID())
		lagPort.Ethernet().SetMac(newMac).SetName(a.ateAggName + "." + strconv.Itoa(aggIdx))
		lagPort.Lacp().SetActorActivity("active").SetActorPortNumber(uint32(aggIdx) + 1).SetActorPortPriority(1).SetLacpduTimeout(0)
	}

	if a.ateAggName == LAG1 {
		configureOTGISIS(t, lagDev, a, pfx1AdvV4)
		configureOTGBGP(t, lagDev, a, pfx1AdvV4, pfx1AdvV6)
	} else {
		configureOTGISIS(t, lagDev, a, pfx2AdvV4)
		configureOTGBGP(t, lagDev, a, pfx2AdvV4, pfx2AdvV6)
	}
	return pmd100GFRPorts
}

// configureOTGISIS configure ISIS on ATE
func configureOTGISIS(t *testing.T, dev gosnappi.Device, agg *aggPortData, advV4 *ipAddr) {
	t.Helper()
	isis := dev.Isis().SetSystemId(agg.ateISISSysID).SetName(agg.ateAggName + ".ISIS")
	isis.Basic().SetHostname(isis.Name())
	isis.Advanced().SetAreaAddresses([]string{ateAreaAddress})
	isisInt := isis.Interfaces().Add()

	isisInt = isisInt.SetEthName(dev.Ethernets().
		Items()[0].Name()).SetName(agg.ateAggName + ".ISISInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).SetMetric(20)
	isisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	devIsisRoutes4 := isis.V4Routes().Add().SetName(agg.ateAggName + ".isisnet4").SetLinkMetric(10)
	devIsisRoutes4.Addresses().Add().
		SetAddress(advV4.ip).SetPrefix(advV4.prefix).SetCount(1).SetStep(1)

}

// configureOTGBGP configure BGP on ATE
func configureOTGBGP(t *testing.T, dev gosnappi.Device, agg *aggPortData, advV4, advV6 *ipAddr) {
	t.Helper()

	iDutBgp := dev.Bgp().SetRouterId(agg.ateIPv4)
	iDutBgp4Peer := iDutBgp.Ipv4Interfaces().Add().SetIpv4Name(agg.ateAggName + ".IPv4").Peers().Add().SetName(agg.ateAggName + ".BGP4.peer")
	iDutBgp4Peer.SetPeerAddress(agg.dutIPv4).SetAsNumber(asn).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDutBgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(false)

	iDutBgp6Peer := iDutBgp.Ipv6Interfaces().Add().SetIpv6Name(agg.ateAggName + ".IPv6").Peers().Add().SetName(agg.ateAggName + ".BGP6.peer")
	iDutBgp6Peer.SetPeerAddress(agg.dutIPv6).SetAsNumber(asn).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iDutBgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(false).SetUnicastIpv6Prefix(true)

	bgpNeti1Bgp4PeerRoutes := iDutBgp4Peer.V4Routes().Add().SetName(agg.ateAggName + ".BGP4.Route")
	if agg.ateAggName != LAG1 {
		bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(pfx2AdvV4.ip + "1").
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(pfx3AdvV4.ip).SetPrefix(pfx3AdvV4.prefix).SetCount(1)
	} else {
		bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(agg.ateIPv4).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	}
}

// configForwardingViable is to set forwarding viable on DUT ports
func configForwardingViable(t *testing.T, dut *ondatra.DUTDevice, dutPorts []*ondatra.Port, forwardingViable bool) {
	for _, port := range dutPorts {
		if forwardingViable {
			gnmi.Update(t, dut, gnmi.OC().Interface(port.Name()).ForwardingViable().Config(), forwardingViable)
		} else {
			gnmi.Update(t, dut, gnmi.OC().Interface(port.Name()).ForwardingViable().Config(), forwardingViable)
		}
	}
}

// incrementMAC uses a mac string and increments it by the given i
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

func createFlows(t *testing.T, dut *ondatra.DUTDevice, top gosnappi.Config, aggIDs []string) []gosnappi.Flow {
	for _, aggID := range aggIDs {
		dutAggMac = append(dutAggMac, gnmi.Get(t, dut, gnmi.OC().Lacp().Interface(aggID).SystemIdMac().State()))
	}
	f1V4 := configureFlows(t, top, pfx1AdvV4, pfx2AdvV4, "pfx1ToPfx2_3", agg1, []*aggPortData{agg2, agg3}, dutAggMac[0], ipRange[1])
	f2V4 := configureFlows(t, top, pfx1AdvV4, pfx4AdvV4, "pfx1ToPfx4", agg1, []*aggPortData{agg2, agg3}, dutAggMac[0], ipRange[0])
	f3V4 := configureFlows(t, top, pfx2AdvV4, pfx1AdvV4, "pfx2ToPfx1Lag2", agg2, []*aggPortData{agg1}, dutAggMac[1], ipRange[0])
	return []gosnappi.Flow{f1V4, f2V4, f3V4}
}

// configureFlows configure flows for traffic on ATE
func configureFlows(t *testing.T, top gosnappi.Config, srcV4 *ipAddr, dstV4 *ipAddr, flowName string, srcAgg *aggPortData,
	dstAgg []*aggPortData, dutAggMac string, ipRange uint32) gosnappi.Flow {

	t.Helper()
	flowV4 := top.Flows().Add().SetName(flowName)
	flowV4.Metrics().SetEnable(true)
	flowV4.TxRx().Port().
		SetTxName(srcAgg.ateAggName)

	if flowName == "pfx2ToPfx1Lag2" || flowName == "pfx2ToPfx1Lag3" {
		flowV4.TxRx().Port().
			SetRxNames([]string{dstAgg[0].ateAggName})
	} else {
		flowV4.TxRx().Port().
			SetRxNames([]string{dstAgg[0].ateAggName, dstAgg[1].ateAggName})
	}
	flowV4.Size().SetFixed(1500)
	flowV4.Rate().SetPps(trafficPPS)
	eV4 := flowV4.Packet().Add().Ethernet()
	eV4.Src().SetValue(srcAgg.ateAggMAC)
	eV4.Dst().SetValue(dutAggMac)
	v4 := flowV4.Packet().Add().Ipv4()
	v4.Src().Increment().SetStart(srcV4.ip).SetCount(v4Count)
	v4.Dst().Increment().SetStart(dstV4.ip).SetCount(ipRange)
	return flowV4
}

// installGRIBIRoutes configure route using gRIBI client
func installGRIBIRoutes(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI(t)
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(12, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()
	client.Start(ctx, t)
	defer client.Stop(t)
	gribi.FlushAll(client)
	client.StartSending(ctx, t)
	gribi.BecomeLeader(t, client)

	tcArgs := &testArgs{
		ctx:    ctx,
		client: client,
		dut:    dut,
		ate:    ate,
		top:    top,
	}

	t.Logf("An IPv4Entry for %s is pointing to ATE LAG2 via gRIBI", pfx4AdvV4.ip+"/24")

	tcArgs.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithIndex(uint64(100)).WithIPAddress(agg2.ateIPv4),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithIndex(uint64(101)).WithIPAddress(agg3.ateIPv4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithID(uint64(100)).AddNextHop(uint64(100), uint64(1)).AddNextHop(uint64(101), uint64(1)))

	tcArgs.client.Modify().AddEntry(t,
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
			WithPrefix(pfx4AdvV4.ip+"/24").WithNextHopGroup(uint64(100)))

	if err := awaitTimeout(tcArgs.ctx, t, tcArgs.client, 5*time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}
	defaultVRFIPList := []string{pfx4AdvV4.ip}
	for ip := range defaultVRFIPList {
		chk.HasResult(t, tcArgs.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(defaultVRFIPList[ip]+"/24").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, t testing.TB, c *fluent.GRIBIClient, timeout time.Duration) error {
	t.Helper()
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

// startTraffic start traffic on ATE
func startTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	capturePktsBeforeTraffic(t, dut, dutPortList)
	time.Sleep(10 * time.Second)
	ate.OTG().StartTraffic(t)
	time.Sleep(time.Minute)
	ate.OTG().StopTraffic(t)
	time.Sleep(time.Second * 40)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.LogLAGMetrics(t, ate.OTG(), top)
	otgutils.LogPortMetrics(t, ate.OTG(), top)
}

// capturePktsBeforeTraffic capture the pkts before traffic on DUT Ports
func capturePktsBeforeTraffic(t *testing.T, dut *ondatra.DUTDevice, dutPortList []*ondatra.Port) {
	rxPktsBeforeTraffic = map[*ondatra.Port]uint64{}
	txPktsBeforeTraffic = map[*ondatra.Port]uint64{}
	for _, port := range dutPortList {
		rxPktsBeforeTraffic[port] = gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).Counters().InPkts().State())
		txPktsBeforeTraffic[port] = gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).Counters().OutPkts().State())
	}
}

// verifyTrafficFlow verify the each flow on ATE
func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flows []gosnappi.Flow, status bool) bool {
	if flows[0].Name() == "pfx1ToPfx4" {
		rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flows[0].Name()).Counters().InPkts().State())
		txPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flows[0].Name()).Counters().OutPkts().State())
		lostPkt := txPkts - rxPkts
		if status {
			if got := (lostPkt * 100 / txPkts); got >= 51 {
				return false
			}
		} else if got := (lostPkt * 100 / txPkts); got > 0 {
			return false
		}
	} else {
		for _, flow := range flows {
			rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State())
			txPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State())
			lostPkt := txPkts - rxPkts
			if got := (lostPkt * 100 / txPkts); got > 0 {
				return false
			}
		}
	}
	return true
}

// awaitAdjacency wait for adjacency to be up/down
func awaitAdjacency(t *testing.T, dut *ondatra.DUTDevice, intfName string, state oc.E_Isis_IsisInterfaceAdjState) bool {
	isisPath := ocpath.Root().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()
	intf := isisPath.Interface(intfName)
	query := intf.LevelAny().AdjacencyAny().AdjacencyState().State()
	_, ok := gnmi.WatchAll(t, dut, query, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		v, ok := val.Val()
		return v == state && ok
	}).Await(t)

	return ok
}

// checkBidirectionalTraffic verify the bidirectional traffic on DUT ports.
func checkBidirectionalTraffic(t *testing.T, dut *ondatra.DUTDevice, portList []*ondatra.Port) error {

	for _, port := range portList {
		txPkts := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).Counters().OutPkts().State())
		rxPkts := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).Counters().InPkts().State())
		if got := (rxPkts - rxPktsBeforeTraffic[port]) / 100; got == 0 {
			return fmt.Errorf("No Packet received, LossPct on Port %s: got %d", port.Name(), got)
		}
		if got := (txPkts - txPktsBeforeTraffic[port]) / 100; got == 0 {
			return fmt.Errorf("No Packet transmitted, LossPct on Port %s: got %d", port.Name(), got)
		}
	}
	return nil
}

// confirmNonViableForwardingTraffic verify the traffic received on DUT
// interfaces and transmitted to ATE-1
func confirmNonViableForwardingTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice,
	atePort []*ondatra.Port, dutPort []*ondatra.Port) error {

	// Ensure no traffic is transmitted out of DUT ports with Forwarding Viable False
	for _, port := range atePort {
		rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(port.ID()).Counters().InFrames().State())
		if got := rxPkts / 100; got > 0 {
			return fmt.Errorf("Packets are transmiited out of %s: got %d, want 0", port.Name(), got)
		}
	}
	// Ensure that traffic is delivered to ATE-1 port1
	for _, port := range dutPort {
		rxPkts := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).Counters().InPkts().State()) - rxPktsBeforeTraffic[port]
		txPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dutPortList[0].Name()).Counters().OutPkts().State()) - txPktsBeforeTraffic[port]
		if got := rxPkts / 100; got == 0 {
			return fmt.Errorf("No Packet received on Interface %s: got %d, want packet", port.Name(), got)
		}
		if got := txPkts / 100; got == 0 {
			return fmt.Errorf("No Packet transmitted on Interface %s: got %d, want packet", port.Name(), got)
		}
	}
	return nil
}

// validateLag3Traffic to ensure traffic Received/Transmitted on DUT LAG_3
func validateLag3Traffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, dutPortList []*ondatra.Port) bool {
	result := false
	for _, port := range dutPortList {
		rxPkts := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).Counters().InPkts().State()) - rxPktsBeforeTraffic[port]
		txPkts := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).Counters().OutPkts().State()) - txPktsBeforeTraffic[port]
		if got := rxPkts / 100; got > 0 {
			if got := txPkts / 100; got > 0 {
				result = true
			}
		} else {
			result = false
		}
	}
	return result
}

// trafficRXWeights to ensure 50:50 Load Balancing
func trafficRXWeights(t *testing.T, ate *ondatra.ATEDevice, aggNames []string, flow gosnappi.Flow) []uint64 {
	t.Helper()
	var rxs []uint64
	for _, aggName := range aggNames {
		metrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Lag(aggName).State())
		rxs = append(rxs, (metrics.GetCounters().GetInFrames()))
	}
	var total uint64
	for _, rx := range rxs {
		total += rx
	}
	for idx, rx := range rxs {
		rxs[idx] = (rx * 100) / total
	}
	return rxs
}
