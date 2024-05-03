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

	"github.com/openconfig/ygot/ygot"
        "github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra"
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
	dutIPv4       string
	dutAggName    string
	dutAggMAC     string
	ateIPv4       string
	dutIPv6       string
	ateIPv6       string
	ateAggName    string
	ateAggMAC     string
	ateISISSysID  string
	ateLoopbackV4 string
	ateLoopbackV6 string
	ateLagCount   uint32
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
		dutIPv4:       "192.0.2.1",
		ateIPv4:       "192.0.2.2",
		dutIPv6:       "2001:db8::1",
		ateIPv6:       "2001:db8::2",
		ateAggName:    LAG1,
		ateAggMAC:     "02:00:01:01:01:01",
		ateISISSysID:  "640000000002",
		ateLoopbackV4: "192.0.2.17",
		ateLoopbackV6: "2001:db8::17",
		ateLagCount:   1,
	}
	agg2 = &aggPortData{
		dutIPv4:       "192.0.2.5",
		ateIPv4:       "192.0.2.6",
		dutIPv6:       "2001:db8::5",
		ateIPv6:       "2001:db8::6",
		ateAggName:    LAG2,
		ateAggMAC:     "02:00:01:01:02:01",
		ateISISSysID:  "640000000003",
		ateLoopbackV4: "192.0.2.18",
		ateLoopbackV6: "2001:db8::18",
		ateLagCount:   5,
	}
	agg3 = &aggPortData{
		dutIPv4:       "192.0.2.9",
		ateIPv4:       "192.0.2.10",
		dutIPv6:       "2001:db8::9",
		ateIPv6:       "2001:db8::a",
		ateAggName:    LAG3,
		ateAggMAC:     "02:00:01:01:03:01",
		ateISISSysID:  "640000000004",
		ateLoopbackV4: "192.0.2.18",
		ateLoopbackV6: "2001:db8::18",
		ateLagCount:   2,
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
	pfx3AdvV6                = &ipAddr{ip: "2004:db8:64:64::0", prefix: 64}
	pfx4AdvV4                = &ipAddr{ip: "100.0.4.0", prefix: 24}
	pmd100GFRPorts           []string
	dutPortList              []*ondatra.Port
	atePortList              []*ondatra.Port
	rxPktsBeforeTraffic      map[*ondatra.Port]uint64
	txPktsBeforeTraffic      map[*ondatra.Port]uint64
	equalDistributionWeights = []uint64{50, 50}
	ecmpTolerance            = uint64(1)
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
	configureGRIBIClient(t, dut, ate, top)
	flows := createFlows(t, top)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	for _, aggID := range aggIDs {
		gnmi.Await(t, dut, gnmi.OC().Interface(aggID).AdminStatus().State(), 30*time.Second, oc.Interface_AdminStatus_UP)
	}

	for _, agg := range []*aggPortData{agg1, agg2, agg3} {
		bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
		gnmi.Await(t, dut, bgpPath.Neighbor(agg.ateLoopbackV4).SessionState().State(), time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	}

	t.Logf("ISIS cost of LAG_2 lower then ISIS cost of LAG_3 Test-01")
	t.Run("Running Testcase for TestID RT-5.7.1.1", func(t *testing.T) {
		t.Logf("Setting Forwarding-Viable to False on all the Member Ports of LAG2 except port2")
		forwardingViableDisable(t, dut, 3, 6)
		startTraffic(t, dut, ate, top)
		checkBidirectionalTraffic(t, dut, dutPortList[1:2])
		confirmNonViableForwardingTraffic(t, dut, ate, atePortList[2:6], dutPortList[2:6])
		// Ensure there is no traffic received/transmiited on DUT LAG_3
		if got := validateLagTraffic(t, dut, ate, dutPortList); got == true {
			t.Fatal("Packets are Received and Transmitted on LAG_3")
		}
		verifyTrafficFlow(t, ate, flows)
	})
	t.Run("Running Testcase for TestID RT-5.7.1.2", func(t *testing.T) {
		t.Logf("Setting Forwarding-Viable to False on all the Member Ports of LAG2")
		forwardingViableDisable(t, dut, 2, 3)
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
		startTraffic(t, dut, ate, top)
		checkBidirectionalTraffic(t, dut, dutPortList[5:6])
		confirmNonViableForwardingTraffic(t, dut, ate, atePortList[1:6], dutPortList[1:6])
		verifyTrafficFlow(t, ate, flows[1:2])
		// Ensure there is no traffic received/transmiited on DUT LAG_3
		if got := validateLagTraffic(t, dut, ate, dutPortList); got == false {
			t.Fatal("Packets are not Received and Transmitted on LAG_3")
		}

		verifyTrafficFlow(t, ate, flows[0:1])
	})

	t.Run("Running Testcase for TestID RT-5.7.1.3", func(t *testing.T) {
		t.Logf("Setting Forwarding-Viable to True on the Member Port Port6 of LAG2")
		forwardingViableEnable(t, dut, 6)
		startTraffic(t, dut, ate, top)
		checkBidirectionalTraffic(t, dut, dutPortList[5:6])
		confirmNonViableForwardingTraffic(t, dut, ate, atePortList[1:5], dutPortList[1:5])
		// Ensure there is no traffic received/transmiited on DUT LAG_3
		if got := validateLagTraffic(t, dut, ate, dutPortList); got == true {
			t.Fatal("Packets are Received and Transmitted on LAG_3")
		}
		verifyTrafficFlow(t, ate, flows)
	})

	// Change ISIS metric Equal for Both LAG_2 and LAG_3
	changeMetric(t, dut, aggIDs[2], 20)

	t.Logf("ISIS cost of LAG_2 equal to ISIS cost of LAG_3 Test-02")
	t.Run("Running Testcase for TestID RT-5.7.2.1", func(t *testing.T) {
		t.Logf("Setting Forwarding-Viable to False on all the Member Ports of LAG2 except port2")
		forwardingViableDisable(t, dut, 6, 6)
		forwardingViableEnable(t, dut, 2)
		flows = append(flows, configureFlows(t, top, pfx2AdvV4, pfx1AdvV4, "pfx2ToPfx1Lag3", agg3, []*aggPortData{agg1}))
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		startTraffic(t, dut, ate, top)
		checkBidirectionalTraffic(t, dut, dutPortList[1:2])
		checkBidirectionalTraffic(t, dut, dutPortList[6:8])
		confirmNonViableForwardingTraffic(t, dut, ate, atePortList[2:6], dutPortList[2:6])
		// Ensure Load Balancing 50:50 on LAG_2 and LAG_3
		weights := trafficRXWeights(t, ate, []string{agg2.ateAggName, agg3.ateAggName}, flows[0])
		for idx, weight := range equalDistributionWeights {
			if got, want := weights[idx], weight; got < want-ecmpTolerance || got > want+ecmpTolerance {
				t.Errorf("ECMP Percentage for Aggregate Index: %d: got %d, want %d", idx+1, got, want)
			}
		}
		verifyTrafficFlow(t, ate, flows)
	})

	t.Run("Running Testcase for TestID RT-5.7.2.2", func(t *testing.T) {
		t.Logf("Setting Forwarding-Viable to False on all the Member Ports of LAG2")
		forwardingViableDisable(t, dut, 2, 3)
		otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
		startTraffic(t, dut, ate, top)
		checkBidirectionalTraffic(t, dut, dutPortList[5:6])
		confirmNonViableForwardingTraffic(t, dut, ate, atePortList[1:6], dutPortList[1:6])
		verifyTrafficFlow(t, ate, flows[1:2])
		// Ensure there is no traffic received/transmiited on DUT LAG_3
		if got := validateLagTraffic(t, dut, ate, dutPortList); got == false {
			t.Fatal("Packets are not Received and Transmitted on LAG_3")
		}
		verifyTrafficFlow(t, ate, flows[0:1])
	})

	t.Run("Running Testcase RT-5.7.2.3", func(t *testing.T) {
		t.Logf("Setting Forwarding-Viable to True on the Member Port Port6 of LAG2")
		forwardingViableEnable(t, dut, 6)
		startTraffic(t, dut, ate, top)
		checkBidirectionalTraffic(t, dut, dutPortList[5:8])
		confirmNonViableForwardingTraffic(t, dut, ate, atePortList[1:5], dutPortList[1:5])
		verifyTrafficFlow(t, ate, flows)
	})
}

// configureDUT configures DUT
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) []string {

	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	configureDUTLoopback(t, dut)

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
	}
	configureRoutingPolicy(t, dut)
	configureDUTISIS(t, dut, aggIDs)
	configureDUTBGP(t, dut, aggIDs)
	return aggIDs
}

// configureDUTLoopback configures DUT loopback
func configureDUTLoopback(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	lb := netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(lb).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	foundV4 := false
	for _, ip := range ipv4Addrs {
		if v, ok := ip.Val(); ok {
			foundV4 = true
			dutLoopback.IPv4 = v.GetIp()
			break
		}
	}
	foundV6 := false
	for _, ip := range ipv6Addrs {
		if v, ok := ip.Val(); ok {
			foundV6 = true
			dutLoopback.IPv6 = v.GetIp()
			break
		}
	}
	if !foundV4 || !foundV6 {
		lo1 := dutLoopback.NewOCInterface(lb, dut)
		lo1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, gnmi.OC().Interface(lb).Config(), lo1)
	}
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
			portList = append(portList, dut.Port(t, fmt.Sprintf("port%d", portIdx+7)))
			dutPortList = append(dutPortList, dut.Port(t, fmt.Sprintf("port%d", portIdx+7)))
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
		bgpNbrV4 := bgp.GetOrCreateNeighbor(a.ateLoopbackV4)
		bgpNbrV4.PeerGroup = ygot.String(pgName)
		bgpNbrV4.PeerAs = ygot.Uint32(asn)
		bgpNbrV4.Enabled = ygot.Bool(true)
		bgpNbrV4T := bgpNbrV4.GetOrCreateTransport()
		bgpNbrV4T.LocalAddress = ygot.String(dutLoopback.IPv4)
		af4 := bgpNbrV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)
		af6 := bgpNbrV4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(false)

		bgpNbrV6 := bgp.GetOrCreateNeighbor(a.ateLoopbackV6)
		bgpNbrV6.PeerGroup = ygot.String(pgName)
		bgpNbrV6.PeerAs = ygot.Uint32(asn)
		bgpNbrV6.Enabled = ygot.Bool(true)
		bgpNbrV6T := bgpNbrV6.GetOrCreateTransport()
		bgpNbrV6T.LocalAddress = ygot.String(dutLoopback.IPv6)
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

		isisIntfLevel.GetOrCreateTimers().HelloInterval = ygot.Uint32(60)
		isisIntfLevel.GetOrCreateTimers().HelloMultiplier = ygot.Uint8(5)

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
	t.Logf("Changing metric to %v on interface %v", metric, intf)
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
				portList = append(portList, ate.Port(t, fmt.Sprintf("port%d", portIdx+7)))
				atePortList = append(atePortList, ate.Port(t, fmt.Sprintf("port%d", portIdx+7)))
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
	lagDev.Ipv4Loopbacks().Add().SetName(agg.Name() + ".Loopback4").SetEthName(lagEth.Name()).SetAddress(a.ateLoopbackV4)
	lagDev.Ipv6Loopbacks().Add().SetName(agg.Name() + ".Loopback6").SetEthName(lagEth.Name()).SetAddress(a.ateLoopbackV6)
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
	configureOTGISIS(t, lagDev, a)
	if a.ateAggName == LAG1 {
		configureOTGBGP(t, lagDev, a, pfx1AdvV4, pfx1AdvV6)
	} else {
		configureOTGBGP(t, lagDev, a, pfx2AdvV4, pfx2AdvV6)
	}
	return pmd100GFRPorts
}

// configureOTGISIS configure ISIS on ATE
func configureOTGISIS(t *testing.T, dev gosnappi.Device, agg *aggPortData) {
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
		SetAddress(agg.ateLoopbackV4).SetPrefix(32).SetCount(1).SetStep(1)

	devIsisRoutes6 := isis.V6Routes().Add().SetName(agg.ateAggName + ".isisnet6").SetLinkMetric(10)
	devIsisRoutes6.Addresses().Add().
		SetAddress(agg.ateLoopbackV6).SetPrefix(128).SetCount(1).SetStep(1)

}

// configureOTGBGP configure BGP on ATE
func configureOTGBGP(t *testing.T, dev gosnappi.Device, agg *aggPortData, advV4, advV6 *ipAddr) {
	t.Helper()
	v4 := dev.Ipv4Loopbacks().Items()[0]
	v6 := dev.Ipv6Loopbacks().Items()[0]

	iDutBgp := dev.Bgp().SetRouterId(agg.ateIPv4)
	iDutBgp4Peer := iDutBgp.Ipv4Interfaces().Add().SetIpv4Name(v4.Name()).Peers().Add().SetName(agg.ateAggName + ".BGP4.peer")
	iDutBgp4Peer.SetPeerAddress(dutLoopback.IPv4).SetAsNumber(asn).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iDutBgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(false)
	iDutBgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(false)

	iDutBgp6Peer := iDutBgp.Ipv6Interfaces().Add().SetIpv6Name(v6.Name()).Peers().Add().SetName(agg.ateAggName + ".BGP6.peer")
	iDutBgp6Peer.SetPeerAddress(dutLoopback.IPv6).SetAsNumber(asn).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iDutBgp6Peer.Capability().SetIpv4UnicastAddPath(false).SetIpv6UnicastAddPath(true)
	iDutBgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(false).SetUnicastIpv6Prefix(true)

	bgpNeti1Bgp4PeerRoutes := iDutBgp4Peer.V4Routes().Add().SetName(agg.ateAggName + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(agg.ateLoopbackV4).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(advV4.ip).SetPrefix(advV4.prefix).SetCount(1)
	bgpNeti1Bgp4PeerRoutes.AddPath().SetPathId(1)

	bgpNeti1Bgp6PeerRoutes := iDutBgp6Peer.V6Routes().Add().SetName(agg.ateAggName + ".BGP6.Route")
	bgpNeti1Bgp6PeerRoutes.Addresses().Add().SetAddress(advV6.ip).SetPrefix(advV6.prefix).SetCount(1)
	bgpNeti1Bgp6PeerRoutes.AddPath().SetPathId(1)

	if agg.ateAggName != LAG1 {
		bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(pfx3AdvV4.ip).SetPrefix(pfx3AdvV4.prefix).SetCount(1)
		bgpNeti1Bgp4PeerRoutes.AddPath().SetPathId(1)

		bgpNeti1Bgp6PeerRoutes.Addresses().Add().SetAddress(pfx3AdvV6.ip).SetPrefix(pfx3AdvV6.prefix).SetCount(1)
		bgpNeti1Bgp6PeerRoutes.AddPath().SetPathId(1)
	}
}

// forwardingViableEnable set to False on Port
func forwardingViableDisable(t *testing.T, dut *ondatra.DUTDevice, start, end int) {
	for port := start; port <= end; port++ {
		pName := dut.Port(t, fmt.Sprintf("port%d", port)).Name()
		gnmi.Update(t, dut, gnmi.OC().Interface(pName).ForwardingViable().Config(), false)
	}
}

// forwardingViableEnable set to True on Port
func forwardingViableEnable(t *testing.T, dut *ondatra.DUTDevice, port int) {
	pName := dut.Port(t, fmt.Sprintf("port%d", port)).Name()
	gnmi.Update(t, dut, gnmi.OC().Interface(pName).ForwardingViable().Config(), true)
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

func createFlows(t *testing.T, top gosnappi.Config) []gosnappi.Flow {
	// var flows []gosnappi.Flow
	f1V4 := configureFlows(t, top, pfx1AdvV4, pfx2AdvV4, "pfx1ToPfx2_3", agg1, []*aggPortData{agg2, agg3})
	f2V4 := configureFlows(t, top, pfx1AdvV4, pfx4AdvV4, "pfx1ToPfx4", agg1, []*aggPortData{agg2})
	f3V4 := configureFlows(t, top, pfx2AdvV4, pfx1AdvV4, "pfx2ToPfx1Lag2", agg2, []*aggPortData{agg1})
	return []gosnappi.Flow{f1V4, f2V4, f3V4}
}

// configureFlows configure flows for traffic on ATE
func configureFlows(t *testing.T, top gosnappi.Config, srcV4 *ipAddr, dstV4 *ipAddr, flowName string, srcAgg *aggPortData,
	dstAgg []*aggPortData) gosnappi.Flow {
	var ipRange uint32
	t.Helper()
	flowV4 := top.Flows().Add().SetName(flowName)
	flowV4.Metrics().SetEnable(true)
	flowV4.TxRx().Device().
		SetTxNames([]string{srcAgg.ateAggName + ".IPv4"})

	if flowName == "pfx1ToPfx2_3" {
		flowV4.TxRx().Device().
			SetRxNames([]string{dstAgg[0].ateAggName + ".IPv4", dstAgg[1].ateAggName + ".IPv4"})
		ipRange = 500
	} else {
		flowV4.TxRx().Device().
			SetRxNames([]string{dstAgg[0].ateAggName + ".IPv4"})
		ipRange = 254
	}
	flowV4.Size().SetFixed(1500)
	flowV4.Rate().SetPps(trafficPPS)
	eV4 := flowV4.Packet().Add().Ethernet()
	eV4.Src().SetValue(srcAgg.ateAggMAC)
	v4 := flowV4.Packet().Add().Ipv4()
	v4.Src().Increment().SetStart(srcV4.ip).SetCount(v4Count)
	v4.Dst().Increment().SetStart(dstV4.ip).SetCount(ipRange)
	return flowV4
}

// configureGRIBIClient configure route using gRIBI client
func configureGRIBIClient(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
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
	const (
		nhgID1 uint64 = 100
		nhID1  uint64 = 1001
	)

	t.Logf("An IPv4Entry for %s is pointing to ATE port-2 and port-3 via gRIBI", pfx4AdvV4.ip+"/24")
	nh := fluent.NextHopEntry().
		WithNetworkInstance(deviations.DefaultNetworkInstance(tcArgs.dut)).
		WithIndex(nhID1).
		WithIPAddress(agg2.ateIPv4)

	nhg := fluent.NextHopGroupEntry().
		WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithID(uint64(nhgID1)).
		AddNextHop(nhID1, 1)

	tcArgs.client.Modify().AddEntry(t, nh, nhg)

	tcArgs.client.Modify().AddEntry(t,
		fluent.IPv4Entry().
			WithPrefix(pfx4AdvV4.ip+"/24").
			WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithNextHopGroup(nhgID1))
}

// startTraffic start traffic on ATE
func startTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	capturePktsBeforeTraffic(t, dut, dutPortList)
	time.Sleep(30 * time.Second)
	ate.OTG().StartTraffic(t)
	time.Sleep(time.Minute)
	ate.OTG().StopTraffic(t)
	time.Sleep(time.Minute)
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
func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flows []gosnappi.Flow) {
	if flows[0].Name() == "pfx1ToPfx4" {
		rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flows[0].Name()).Counters().InPkts().State())
		if got := rxPkts / 100; got == 0 {
			t.Logf("No Packet received, LossPct for flow %s: got %d, want 0 packet", flows[0].Name(), got)
		} else {
			t.Fatalf("Packet received for flow %s: got %d, want 0 packet", flows[0].Name(), got)
		}
	} else {
		for _, flow := range flows {
			rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State())
			txPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State())
			lostPkt := txPkts - rxPkts
			if got := (lostPkt * 100 / txPkts); got > 0 {
				t.Fatalf("LossPct for flow %s: Lost_Percentage is %d, want 0", flow.Name(), got)
			}
		}
	}
}

// checkBidirectionalTraffic verify the bidirectional traffic on DUT ports.
func checkBidirectionalTraffic(t *testing.T, dut *ondatra.DUTDevice, portList []*ondatra.Port) {

	for _, port := range portList {
		txPkts := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).Counters().InPkts().State())
		rxPkts := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).Counters().InPkts().State())

		if got := (rxPkts - rxPktsBeforeTraffic[port]) / 100; got == 0 {
			t.Fatalf("No Packet received, LossPct on Port %s: got %d", port.Name(), got)
		}
		if got := (txPkts - txPktsBeforeTraffic[port]) / 100; got == 0 {
			t.Fatalf("No Packet transmitted, LossPct on Port %s: got %d", port.Name(), got)
		}
	}
}

// confirmNonViableForwardingTraffic verify the traffic received on DUT
// interfaces and transmitted to ATE-1
func confirmNonViableForwardingTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice,
	atePort []*ondatra.Port, dutPort []*ondatra.Port) {
	// Ensure no traffic is transmitted out of DUT ports with Forwarding Viable False
	for _, port := range atePort {
		rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(port.ID()).Counters().InFrames().State())
		if got := rxPkts / 100; got > 0 {
			t.Fatalf("Packets are transmiited out of %s: got %d, want 0", port.Name(), got)
		}
	}
	// Ensure that traffic is delivered to ATE-1 port1
	for _, port := range dutPort {
		rxPkts := gnmi.Get(t, dut, gnmi.OC().Interface(port.Name()).Counters().InPkts().State()) - rxPktsBeforeTraffic[port]
		txPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dutPortList[0].Name()).Counters().OutPkts().State()) - txPktsBeforeTraffic[port]
		if got := rxPkts / 100; got == 0 {
			t.Fatalf("LossPct, No packet Received on Interface %s: got %d, want packet", port.Name(), got)
		}
		if got := txPkts / 100; got == 0 {
			t.Fatalf("LossPct, No packet transmitted from Interface %s: got %d, want packet", dutPortList[0].Name(), got)
		}
	}
}

// validateLagTraffic to ensure traffic Received/Transmitted on DUT LAG_3
func validateLagTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, dutPortList []*ondatra.Port) bool {
	result := false
	for _, port := range dutPortList[6:8] {
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
	// pfx4FlowRx get the RX counters for the Flow which have traffic destined for prefix 100.0.4.0
	pfx4FlowRx := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State())
	for aggIdx, aggName := range aggNames {
		metrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Lag(aggName).State())
		if aggIdx == 0 {
			rxs = append(rxs, (metrics.GetCounters().GetInFrames() - pfx4FlowRx))
		} else {
			rxs = append(rxs, metrics.GetCounters().GetInFrames())
		}
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
