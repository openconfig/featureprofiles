// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package multipath_bgp_ecmp_protocol_nexthop

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygot/ygot"
)



func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func uint8Ptr(i uint8) *uint8 { return &i }

// The test topology consists of a DUT connected to a ATE with four ports.
// iBGP is configured between DUT Port2 and ATE Port2, DUT Port3 and ATE Port3,
// and DUT Port4 and ATE Port4.
// ISIS is configured on interfaces for ports 2, 3, and 4.
// ATE Port2, Port3 and Port4 advertise loopback addresses via ISIS.
// ATE Port2, Port3 and Port4 advertise prefix 100.1.1.0/24 via iBGP, with
// next-hops being their loopback addresses respectively.
// These loopback addresses are reachable via ISIS paths over DUT Port2, Port3 and Port4.
// The test verifies that DUT installs ECMP paths for prefixes 100.1.1.0/24
// and 100.1.1.1::1/128.
//
// Topology:
//
//	ATE port 1 -------- DUT port 1
//	ATE port 2 -------- DUT port 2
//	ATE port 3 -------- DUT port 3
//	ATE port 4 -------- DUT port 4
//
// Configuration:
//
//	DUT
//	  Port 1: 192.1.1.1/24, 2001:192:1:1::1/64
//	  Port 2: 192.1.2.1/24, 2001:192:1:2::1/64
//	  Port 3: 192.1.3.1/24, 2001:192:1:3::1/64
//	  Port 4: 192.1.4.1/24, 2001:192:1:4::1/64
//	  ISIS level2 on Port2, Port3, Port4 with metric 10
//	  iBGP sessions with 192.1.2.2, 192.1.3.2, 192.1.4.2 (ATE Port2,3,4)
//	  iBGP sessions with 2001:192:1:2::2, 2001:192:1:3::2, 2001:192:1:4::2 (ATE Port2,3,4)
//	  BGP router-id: 192.1.1.1
//	  AS: 65001
//    iBGP multipath enabled.
//
//	ATE
//	  Port 1: 192.1.1.2/24, 2001:192:1:1::2/64
//	  Port 2: 192.1.2.2/24, 2001:192:1:2::2/64
//	    Loopback IPv4: 193.1.1.1/32 (advertised in ISIS)
//	    Loopback IPv6: 193:1:1::1/128 (advertised in ISIS)
//	  Port 3: 192.1.3.2/24, 2001:192:1:3::2/64
//	    Loopback IPv4: 193.1.1.2/32 (advertised in ISIS)
//	    Loopback IPv6: 193:1:2::1/128 (advertised in ISIS)
//	  Port 4: 192.1.4.2/24, 2001:192:1:4::2/64
//	    Loopback IPv4: 193.1.1.3/32 (advertised in ISIS)
//	    Loopback IPv6: 193:1:3::1/128 (advertised in ISIS)
//	  ISIS level2 on Port2, Port3, Port4 with metric 10
//	  iBGP sessions with 192.1.2.1, 192.1.3.1, 192.1.4.1 (DUT Port2,3,4)
//	  iBGP sessions with 2001:192:1:2::1, 2001:192:1:3::1, 2001:192:1:4::1 (DUT Port2,3,4)
//	  AS: 65001
//	  ATE Port2 advertises 100.1.1.0/24 (next-hop 193.1.1.1) and 100:1:1::/48 (next-hop 193:1:1::1)
//	  ATE Port3 advertises 100.1.1.0/24 (next-hop 193.1.1.2) and 100:1:1::/48 (next-hop 193:1:2::1)
//	  ATE Port4 advertises 100.1.1.0/24 (next-hop 193.1.1.3) and 100:1:1::/48 (next-hop 193:1:3::1)

const (
	dutAS            = 64501
	ateAS            = 64501
	ateP2Lo0IP       = "193.1.1.1"
	ateP3Lo0IP       = "193.1.1.2"
	ateP4Lo0IP       = "193.1.1.3"
	p1IPv4           = "192.1.1.1"
	p1IPv4Len        = 24
	p2IPv4           = "192.1.2.1"
	p2IPv4Len        = 24
	p3IPv4           = "192.1.3.1"
	p3IPv4Len        = 24
	p4IPv4           = "192.1.4.1"
	p4IPv4Len        = 24
	ateP1IPv4        = "192.1.1.2"
	ateP2IPv4        = "192.1.2.2"
	ateP3IPv4        = "192.1.3.2"
	ateP4IPv4        = "192.1.4.2"
	p1IPv6           = "2001:192:1:1::1"
	p1IPv6Len        = 64
	p2IPv6           = "2001:192:1:2::1"
	p2IPv6Len        = 64
	p3IPv6           = "2001:192:1:3::1"
	p3IPv6Len        = 64
	p4IPv6           = "2001:192:1:4::1"
	p4IPv6Len        = 64
	ateP1IPv6        = "2001:192:1:1::2"
	ateP2IPv6        = "2001:192:1:2::2"
	ateP3IPv6        = "2001:192:1:3::2"
	ateP4IPv6        = "2001:192:1:4::2"
	ateP2Lo0IPv6     = "193:1:1::1"
	ateP3Lo0IPv6     = "193:1:2::1"
	ateP4Lo0IPv6     = "193:1:3::1"
	isisMetric       = 10
	prefixMin        = "100.1.1.0"
	prefixLen        = 24
	prefixV6Min      = "100:1:1::"
	prefixV6Len      = 48
	prefixCount      = 1
	bgpPassword      = "BGPKEY"
	PTISIS           = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	ISISName         = "DEFAULT"
	isisAreaAddr     = "49.0000"
	isisSysID        = "1920.0000.2001"
	totalPackets     = 1000
	trafficPps       = 100
	peerGrpName1     = "BGP-PEER-GROUP1"
	peerGrpName2     = "BGP-PEER-GROUP2"
	peerGrpName3     = "BGP-PEER-GROUP3"
	peerGrpName4     = "BGP-PEER-GROUP4"
	peerGrpName5     = "BGP-PEER-GROUP5"
	peerGrpName6     = "BGP-PEER-GROUP6"
	lossTolerancePct = 0  // Packet loss tolerance in percentage
	lbToleranceFms   = 20 // Load Balance Tolerance in percentage
)

var (
	dutP1 = attrs.Attributes{
		Desc:    "DUT to ATE Port 1",
		IPv4:    p1IPv4,
		IPv4Len: p1IPv4Len,
		IPv6:    p1IPv6,
		IPv6Len: p1IPv6Len,
	}
	dutP2 = attrs.Attributes{
		Desc:    "DUT to ATE Port 2",
		IPv4:    p2IPv4,
		IPv4Len: p2IPv4Len,
		IPv6:    p2IPv6,
		IPv6Len: p2IPv6Len,
	}
	dutP3 = attrs.Attributes{
		Desc:    "DUT to ATE Port 3",
		IPv4:    p3IPv4,
		IPv4Len: p3IPv4Len,
		IPv6:    p3IPv6,
		IPv6Len: p3IPv6Len,
	}
	dutP4 = attrs.Attributes{
		Desc:    "DUT to ATE Port 4",
		IPv4:    p4IPv4,
		IPv4Len: p4IPv4Len,
		IPv6:    p4IPv6,
		IPv6Len: p4IPv6Len,
	}
	ateP1 = attrs.Attributes{
		Name:    "ateP1",
		IPv4:    ateP1IPv4,
		IPv4Len: p1IPv4Len,
		IPv6:    ateP1IPv6,
		IPv6Len: p1IPv6Len,
	}
	ateP2 = attrs.Attributes{
		Name:    "ateP2",
		IPv4:    ateP2IPv4,
		IPv4Len: p2IPv4Len,
		IPv6:    ateP2IPv6,
		IPv6Len: p2IPv6Len,
	}
	ateP3 = attrs.Attributes{
		Name:    "ateP3",
		IPv4:    ateP3IPv4,
		IPv4Len: p3IPv4Len,
		IPv6:    ateP3IPv6,
		IPv6Len: p3IPv6Len,
	}
	ateP4 = attrs.Attributes{
		Name:    "ateP4",
		IPv4:    ateP4IPv4,
		IPv4Len: p4IPv4Len,
		IPv6:    ateP4IPv6,
		IPv6Len: p4IPv6Len,
	}
	dutlo0Attrs = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.0.113.1",
		IPv6:    "2001:0::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}
)

type testCase struct {
	desc      string
	ipv4      bool
	ipv6      bool
	multipath bool
}

func TestMultipathBGPEcmpProtocolNexthop(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// configureDUT(t, dut)
	t.Run("Configure DUT", func(t *testing.T) {
		configureDUT(t, dut)
	})

	// configureATE(t, ate)
	t.Run("Configure OTG", func(t *testing.T) {
		configureATE(t, ate)
	})

	// ate.OTG().StartTraffic(t)
	// time.Sleep(10 * time.Second)
	// ate.OTG().StopTraffic(t)

	testCases := []testCase{
		{
			desc:      "Testing with multipath disabled for ipv4 ",
			ipv4:      true,
			ipv6:      true,
			multipath: false,
		},
		{
			desc:      "Testing with multipath enabled for ipv4",
			ipv4:      true,
			ipv6:      false,
			multipath: true,
		},
		{
			desc:      "Testing with multipath disabled for ipv6",
			ipv4:      false,
			ipv6:      true,
			multipath: false,
		},
		{
			desc:      "Testing with multipath enabled for ipv6",
			ipv4:      false,
			ipv6:      true,
			multipath: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.ipv4 {
				t.Logf("Validating traffic test for IPv4 prefixes: [%s, %d]", prefixMin, prefixLen)
				if tc.multipath {
					t.Logf("Multipath is enabled for IPv4 prefixes: [%s, %d]", prefixMin, prefixLen)
				} else {
					t.Logf("Multipath is disabled for IPv4 prefixes: [%s, %d]", prefixMin, prefixLen)
				}
				verifyDUT(t, dut)
				verifyTraffic(t, ate, "ipv4", 0)
				sleepTime := time.Duration(totalPackets/trafficPps) + 5
				ate.OTG().StartTraffic(t)
				time.Sleep(sleepTime * time.Second)
				ate.OTG().StopTraffic(t)
				// otgutils.LogFlowMetrics(t, ate.OTG(), otgCfg)
				checkPacketLoss(t, ate)
				verifyECMPLoadBalance(t, ate, int(cfgplugins.PortCount4), 3)
			}
			if tc.ipv6 {
				t.Logf("Validating traffic test for IPv6 prefixes: [%s, %d]", prefixV6Min, prefixV6Len)
				if tc.multipath {
					t.Logf("Multipath is enabled for IPv6 prefixes: [%s, %d]", prefixV6Min, prefixV6Len)
				} else {
					t.Logf("Multipath is disabled for IPv6 prefixes: [%s, %d]", prefixV6Min, prefixV6Len)
				}
				verifyDUT(t, dut)
				verifyTraffic(t, ate, "ipv6", 0)
				sleepTime := time.Duration(totalPackets/trafficPps) + 5
				ate.OTG().StartTraffic(t)
				time.Sleep(sleepTime * time.Second)
				ate.OTG().StopTraffic(t)
				checkPacketLoss(t, ate)
				verifyECMPLoadBalance(t, ate, int(cfgplugins.PortCount4), 3)
			}
		})
	}
}

type bgpNeighbor struct {
	as           uint32
	neighborip   string
	isV4         bool
	peerGrp      string
	localAddress string
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	// Configure DUT port 1 with IPv4 and IPv6 addresses.
	// This is the DUT side of ATE port 1 used for sending traffic. Source traffic
	// is sent from ATE port 1.
	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutP1.NewOCInterface(p1.Name(), dut))
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutP2.NewOCInterface(p2.Name(), dut))
	p3 := dut.Port(t, "port3")
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), dutP3.NewOCInterface(p3.Name(), dut))
	p4 := dut.Port(t, "port4")
	gnmi.Replace(t, dut, d.Interface(p4.Name()).Config(), dutP4.NewOCInterface(p4.Name(), dut))
	t.Logf("Now configuring ISIS config on DUT")
	time.Sleep(60 * time.Second)

	dutConfPath := d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISName)
	dutConf := addISISOC(isisAreaAddr, isisSysID, []string{p2.Name(), p3.Name(), p4.Name()}, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	t.Logf("ISIS config applied on DUT")

	t.Logf("Now configuring BGP config on DUT")
	// Configure BGP neighbors and peer groups on DUT.
	dutConfPath = gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Delete(t, dut, dutConfPath.Config())
	dutConf = bgpCreateNbr(dutAS, ateAS, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	t.Logf("BGP config applied on DUT")
	time.Sleep(30 * time.Second)
}

func addISISOC(areaAddress, sysID string, ifaceNames []string, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := inst.GetOrCreateProtocol(PTISIS, ISISName)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		glob.Instance = ygot.String(ISISName)
	}
	glob.Net = []string{fmt.Sprintf("%v.%v.00", isisAreaAddr, isisSysID)}
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	// Configure ISIS enabled flag at level
	if deviations.ISISLevelEnabled(dut) {
		level.Enabled = ygot.Bool(true)
	}

	for _, ifaceName := range ifaceNames {
		intf := isis.GetOrCreateInterface(ifaceName)
		intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		intf.Enabled = ygot.Bool(true)
		// Configure ISIS level at global mode if true else at interface mode
		if deviations.ISISInterfaceLevel1DisableRequired(dut) {
			intf.GetOrCreateLevel(1).Enabled = ygot.Bool(false)
		} else {
			intf.GetOrCreateLevel(2).Enabled = ygot.Bool(true)
		}
		glob.LevelCapability = oc.Isis_LevelType_LEVEL_2
		// Configure ISIS enable flag at interface level
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
		if deviations.ISISInterfaceAfiUnsupported(dut) {
			intf.Af = nil
		}
	}

	return prot
}

// bgpCreateNbr creates a BGP neighbor configuration for the DUT with multiple paths.
// TODO: Add support for multiple paths and local address.
func bgpCreateNbr(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nbr1v4 := &bgpNeighbor{as: ateAS, neighborip: ateP2IPv4, isV4: true, peerGrp: peerGrpName1}
	nbr2v4 := &bgpNeighbor{as: ateAS, neighborip: ateP3IPv4, isV4: true, peerGrp: peerGrpName2}
	nbr3v4 := &bgpNeighbor{as: ateAS, neighborip: ateP4IPv4, isV4: true, peerGrp: peerGrpName3}
	nbr1v6 := &bgpNeighbor{as: ateAS, neighborip: ateP2IPv6, isV4: false, peerGrp: peerGrpName4}
	nbr2v6 := &bgpNeighbor{as: ateAS, neighborip: ateP3IPv6, isV4: false, peerGrp: peerGrpName5}
	nbr3v6 := &bgpNeighbor{as: ateAS, neighborip: ateP4IPv6, isV4: false, peerGrp: peerGrpName6}
	nbrs := []*bgpNeighbor{nbr1v4, nbr2v4, nbr3v4, nbr1v6, nbr2v6, nbr3v6}
	dev := &oc.Root{}
	ni := dev.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutlo0Attrs.IPv4)
	global.As = ygot.Uint32(dutAS)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	// global.GetOrCreateUseMultiplePaths().GetOrCreateIbgp().MaximumPaths = ygot.Uint32(4)
	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1)
	pg1.PeerAs = ygot.Uint32(ateAS)
	pg1.PeerGroupName = ygot.String(peerGrpName1)
	pg2 := bgp.GetOrCreatePeerGroup(peerGrpName2)
	pg2.PeerAs = ygot.Uint32(ateAS)
	pg2.PeerGroupName = ygot.String(peerGrpName2)
	pg3 := bgp.GetOrCreatePeerGroup(peerGrpName3)
	pg3.PeerAs = ygot.Uint32(ateAS)
	pg3.PeerGroupName = ygot.String(peerGrpName3)
	pg4 := bgp.GetOrCreatePeerGroup(peerGrpName4)
	pg4.PeerAs = ygot.Uint32(ateAS)
	pg4.PeerGroupName = ygot.String(peerGrpName4)
	pg5 := bgp.GetOrCreatePeerGroup(peerGrpName5)
	pg5.PeerAs = ygot.Uint32(ateAS)
	pg5.PeerGroupName = ygot.String(peerGrpName5)
	pg6 := bgp.GetOrCreatePeerGroup(peerGrpName6)
	pg6.PeerAs = ygot.Uint32(ateAS)
	pg6.PeerGroupName = ygot.String(peerGrpName6)

	for _, nbr := range nbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.neighborip)
		bgpNbr.PeerGroup = ygot.String(nbr.peerGrp)
		bgpNbr.PeerAs = ygot.Uint32(nbr.as)
		bgpNbr.Enabled = ygot.Bool(true)
		bgpNbr.AuthPassword = ygot.String(bgpPassword)
		if nbr.localAddress != "" {
			bgpNbrT := bgpNbr.GetOrCreateTransport()
			bgpNbrT.LocalAddress = ygot.String(nbr.localAddress)
		}
		if nbr.isV4 == true {
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
		}
	}
	return niProto
}

// func configureATE(t *testing.T, ate *ondatra.ATEDevice) {
func configureATE(t *testing.T, ate *ondatra.ATEDevice) {
	top := ate.Topology().New()
	t.Log("Configure ATE interface")
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	p3 := ate.Port(t, "port3")
	p4 := ate.Port(t, "port4")

	// Add interface for ATE port 1.
	i1 := top.AddInterface(ateP1.Name).WithPort(p1)
	i1.IPv4().WithAddress(fmt.Sprintf("%s/%d", ateP1.IPv4, ateP1.IPv4Len)).WithDefaultGateway(p1IPv4)
	i1.IPv6().WithAddress(fmt.Sprintf("%s/%d", ateP1.IPv6, ateP1.IPv6Len)).WithDefaultGateway(p1IPv6)

	ports := []struct {
		portIdx int
		port    *ondatra.Port
		attrs   attrs.Attributes
		loIPv4  string
		loIPv6  string
		dutIPv4 string
		dutIPv6 string
	}{
		{portIdx: 2, port: p2, attrs: ateP2, loIPv4: ateP2Lo0IP, loIPv6: ateP2Lo0IPv6, dutIPv4: p2IPv4, dutIPv6: p2IPv6},
		{portIdx: 3, port: p3, attrs: ateP3, loIPv4: ateP3Lo0IP, loIPv6: ateP3Lo0IPv6, dutIPv4: p3IPv4, dutIPv6: p3IPv6},
		{portIdx: 4, port: p4, attrs: ateP4, loIPv4: ateP4Lo0IP, loIPv6: ateP4Lo0IPv6, dutIPv4: p4IPv4, dutIPv6: p4IPv6},
	}
	var ecmpIntfs []ondatra.Endpoint
	for _, p := range ports {
		intf := top.AddInterface(p.attrs.Name).WithPort(p.port)
		ecmpIntfs = append(ecmpIntfs, intf)
		intf.IPv4().WithAddress(fmt.Sprintf("%s/%d", p.attrs.IPv4, p.attrs.IPv4Len)).WithDefaultGateway(p.dutIPv4)
		intf.IPv6().WithAddress(fmt.Sprintf("%s/%d", p.attrs.IPv6, p.attrs.IPv6Len)).WithDefaultGateway(p.dutIPv6)
		intf.ISIS().WithLevelL2().WithMetric(isisMetric).WithNetworkTypePointToPoint().WithHelloPaddingEnabled(true)
		net := intf.AddNetwork(fmt.Sprintf("n%d", p.portIdx))
		net.IPv4().WithAddress(p.loIPv4 + "/32")
		net.IPv6().WithAddress(p.loIPv6 + "/128")
		net.ISIS().WithIPReachabilityInternal()
		bgp := intf.BGP()
		bgp4 := bgp.AddPeer()
		bgp4.WithPeerAddress(p.dutIPv4).WithLocalASN(ateAS).WithTypeInternal().WithMD5Key(bgpPassword)
		bgp4.Capabilities().WithIPv4UnicastEnabled(true)
		bgpNet4 := intf.AddNetwork(fmt.Sprintf("bgpNet%dIpv4", p.portIdx))
		bgpNet4.IPv4().WithAddress(prefixMin + "/" + fmt.Sprint(prefixLen)).WithCount(prefixCount)
		// bgpNet4.BGP().WithNextHopAddress(p.loIPv4).WithPeer(bgp4)
		bgp6 := bgp.AddPeer()
		bgp6.WithPeerAddress(p.dutIPv6).WithLocalASN(ateAS).WithTypeInternal().WithMD5Key(bgpPassword)
		bgp6.Capabilities().WithIPv6UnicastEnabled(true)
		bgpNet6 := intf.AddNetwork(fmt.Sprintf("bgpNet%dIpv6", p.portIdx))
		bgpNet6.IPv6().WithAddress(prefixV6Min + "/" + fmt.Sprint(prefixV6Len)).WithCount(prefixCount)
		// bgpNet6.BGP().WithNextHopAddress(p.loIPv6).WithPeer(bgp6)
	}

	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header().WithDstAddress(prefixMin)
	ipv6Header := ondatra.NewIPv6Header().WithDstAddress(prefixV6Min)

	// Add flows for traffic verification.
	flow4 := ate.Traffic().NewFlow("flowipv40").
		WithSrcEndpoints(i1).
		WithDstEndpoints(ecmpIntfs...).
		WithHeaders(ethHeader, ipv4Header).
		WithFrameSize(1500).
		WithFrameRateFPS(trafficPps)
	flow4.WithFrameSize(1500)

	flow6 := ate.Traffic().NewFlow("flowipv60").
		WithSrcEndpoints(ecmpIntfs...).
		WithDstEndpoints(i1).
		WithHeaders(ethHeader, ipv6Header).
		WithFrameSize(1500).
		WithFrameRateFPS(trafficPps)
	flow6.WithFrameSize(1500)

	// return top
	t.Logf("Pushing config to OTG")
	top.Push(t)
	time.Sleep(40 * time.Second)
	t.Logf("Starting protocols on OTG")
	top.StartProtocols(t)
	time.Sleep(40 * time.Second)
}

func verifyDUT(t *testing.T, dut *ondatra.DUTDevice) {
	statePathV4 := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
		Bgp().Rib().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast()
	statePathV6 := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
		Bgp().Rib().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast()

	t.Logf("Verifying DUT BGP sessions up for IPv4")
	for _, neighborIP := range []string{ateP2IPv4, ateP3IPv4, ateP4IPv4} {
		gnmi.Await(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
			Bgp().Neighbor(neighborIP).SessionState().State(), 2*time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	}
	t.Logf("Verifying DUT BGP sessions up for IPv6")
	for _, neighborIP := range []string{ateP2IPv6, ateP3IPv6, ateP4IPv6} {
		gnmi.Await(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").
			Bgp().Neighbor(neighborIP).SessionState().State(), 2*time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	}

	prefixV4 := fmt.Sprintf("%s/%d", prefixMin, prefixLen)
	for _, neighborIP := range []string{ateP2IPv4, ateP3IPv4, ateP4IPv4} {
		routes := gnmi.Lookup(t, dut, statePathV4.Neighbor(neighborIP).AdjRibInPost().Route(prefixV4, 0).State())
		if !routes.IsPresent() {
			t.Errorf("Prefix %s not found in AdjRibInPost for neighbor %s", prefixV4, neighborIP)
		}
		locRib := gnmi.Lookup(t, dut, statePathV4.LocRib().Route(prefixV4, oc.UnionString(neighborIP), 0).State())
		if !locRib.IsPresent() {
			t.Errorf("Prefix %s not found in LocRib from %s", prefixV4, neighborIP)
		}
	}

	prefixV6 := fmt.Sprintf("%s/%d", prefixV6Min, prefixV6Len)
	for _, neighborIP := range []string{ateP2IPv6, ateP3IPv6, ateP4IPv6} {
		routes := gnmi.Lookup(t, dut, statePathV6.Neighbor(neighborIP).AdjRibInPost().Route(prefixV6, 0).State())
		if !routes.IsPresent() {
			t.Errorf("Prefix %s not found in AdjRibInPost for neighbor %s", prefixV6, neighborIP)
		}
		locRib := gnmi.Lookup(t, dut, statePathV6.LocRib().Route(prefixV6, oc.UnionString(neighborIP), 0).State())
		if !locRib.IsPresent() {
			t.Errorf("Prefix %s not found in LocRib from %s", prefixV6, neighborIP)
		}
	}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	ipv4Entries := gnmi.Lookup(t, dut, statePath.Neighbor(ateP2IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes().State())

	if !ipv4Entries.IsPresent() {
		t.Fatalf("Prefix %s not found in AFT", prefixV4)
	}
	// for _, ipv4Entry := range ipv4Entries {
	// 	nhgID := ipv4Entry.GetNextHopGroup()
	// 	if nhgID == 0 {
	// 		t.Fatalf("Prefix %s doesn't have a next-hop-group", prefixV4)
	// 	}
	// 	nhs := gnmi.Lookup(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().NextHopGroup(nhgID).NextHopAny().State())
	// 	if len(nhs) != 3 {
	// 		t.Errorf("Prefix %s has %d next-hops in NHG %d, want 3 for ECMP", prefixV4, len(nhs), nhgID)
	// 	} else {
	// 		t.Logf("Prefix %s has %d next-hops in NHG %d, ECMP is active.", prefixV4, len(nhs), nhgID)
	// 	}
	// }

	ipv6Entries := gnmi.Lookup(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().Ipv6Entry(prefixV6).State())
	if !ipv6Entries.IsPresent() {
		t.Fatalf("Prefix %s not found in AFT", prefixV6)
	}
	// for _, ipv6Entry := range ipv6Entries {
	// 	nhgID := ipv6Entry.GetNextHopGroup()
	// 	if nhgID == 0 {
	// 		t.Fatalf("Prefix %s doesn't have a next-hop-group", prefixV6)
	// 	}
	// 	nhs := gnmi.Lookup(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Afts().NextHopGroup(nhgID).NextHopAny().State())
	// 	if len(nhs) != 3 {
	// 		t.Errorf("Prefix %s has %d next-hops in NHG %d, want 3 for ECMP", prefixV6, len(nhs), nhgID)
	// 	} else {
	// 		t.Logf("Prefix %s has %d next-hops in NHG %d, ECMP is active.", prefixV6, len(nhs), nhgID)
	// 	}
	// }
}

func configureFlow(t *testing.T, bs *cfgplugins.BGPSession, prefixPair []string, prefixType string, index int) {
	flow := bs.ATETop.Flows().Add().SetName("flow" + prefixType + strconv.Itoa(index))
	flow.Metrics().SetEnable(true)

	if prefixType == "ipv4" {
		flow.TxRx().Device().
			SetTxNames([]string{bs.ATEPorts[0].Name + ".IPv4"}).
			SetRxNames([]string{bs.ATEPorts[1].Name + ".BGP4.peer.dut." + strconv.Itoa(index)})
	} else {
		flow.TxRx().Device().
			SetTxNames([]string{bs.ATEPorts[0].Name + ".IPv6"}).
			SetRxNames([]string{bs.ATEPorts[1].Name + ".BGP6.peer.dut." + strconv.Itoa(index)})
	}

	flow.Duration().FixedPackets().SetPackets(totalPackets)
	flow.Size().SetFixed(1500)
	flow.Rate().SetPps(trafficPps)

	e := flow.Packet().Add().Ethernet()
	e.Src().SetValue(bs.ATEPorts[1].MAC)

	if prefixType == "ipv4" {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(bs.ATEPorts[0].IPv4)
		v4.Dst().SetValues(prefixPair)
	} else {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(bs.ATEPorts[0].IPv6)
		v6.Dst().SetValues(prefixPair)
	}
}

func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, prefixType string, index int) {
	recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow("flow"+prefixType+strconv.Itoa(index)).State())
	framesTx := recvMetric.GetCounters().GetOutPkts()
	framesRx := recvMetric.GetCounters().GetInPkts()

	if framesTx == 0 {
		t.Error("No traffic was generated and frames transmitted were 0")
	} else if framesRx == framesTx {
		t.Logf("Traffic validation successful FramesTx: %d FramesRx: %d", framesTx, framesRx)
	}
}

func checkPacketLoss(t *testing.T, ate *ondatra.ATEDevice) {
	countersPath := gnmi.OTG().Flow("flow").Counters()
	rxPackets := gnmi.Get(t, ate.OTG(), countersPath.InPkts().State())
	txPackets := gnmi.Get(t, ate.OTG(), countersPath.OutPkts().State())
	lostPackets := txPackets - rxPackets
	if txPackets < 1 {
		t.Fatalf("Tx packets should be higher than 0")
	}

	if got := lostPackets * 100 / txPackets; got != lossTolerancePct {
		t.Errorf("Packet loss percentage for flow: got %v, want %v", got, lossTolerancePct)
	}
}

func verifyECMPLoadBalance(t *testing.T, ate *ondatra.ATEDevice, pc int, expectedLinks int) {
	dut := ondatra.DUT(t, "dut")
	framesTx := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port1").ID()).Counters().OutFrames().State())
	expectedPerLinkFms := framesTx / uint64(expectedLinks)
	t.Logf("Total packets %d flow through the %d links and expected per link packets %d", framesTx, expectedLinks, expectedPerLinkFms)
	min := expectedPerLinkFms - (expectedPerLinkFms * lbToleranceFms / 100)
	max := expectedPerLinkFms + (expectedPerLinkFms * lbToleranceFms / 100)

	got := 0
	for i := 3; i <= pc; i++ {
		framesRx := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, "port"+strconv.Itoa(i)).ID()).Counters().InFrames().State())
		if framesRx <= lbToleranceFms {
			t.Logf("Skip: Traffic through port%d interface is %d", i, framesRx)
			continue
		}
		if int64(min) < int64(framesRx) && int64(framesRx) < int64(max) {
			t.Logf("Traffic %d is in expected range: %d - %d, Load balance Test Passed", framesRx, min, max)
			got++
		} else {
			if !deviations.BgpMaxMultipathPathsUnsupported(dut) {
				t.Errorf("Traffic is expected in range %d - %d but got %d. Load balance Test Failed", min, max, framesRx)
			}
		}
	}
	if !deviations.BgpMaxMultipathPathsUnsupported(dut) {
		if got != expectedLinks {
			t.Errorf("invalid number of load balancing interfaces, got: %d want %d", got, expectedLinks)
		}
	}
}
