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
package multipath_bgp_ecmp_protocol_nexthop_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/netinstbgp"
	"github.com/openconfig/ondatra/netutil"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

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
	prefixMin        = "100.1.1.1"
	prefixLen        = 24
	prefixV6Min      = "100:1:1::1"
	prefixV6Len      = 64
	prefixCount      = 1
	maxpaths         = 4
	isisAreaAddr     = "49.0000"
	isisSysID        = "1920.0000.2001"
	otgSysID2        = "640000000001"
	otgSysID3        = "640000000002"
	otgSysID4        = "640000000004"
	totalPackets     = 1000000
	trafficPps       = 10000
	packetSize       = 1500
	peerGrpName1     = "BGP-PEER-GROUP1"
	lossTolerancePct = 0  // Packet loss tolerance in percentage
	lbToleranceFms   = 20 // Load Balance Tolerance in percentage
	rplType          = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	rplName          = "ALLOW"
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
		MAC:     "02:00:01:01:01:01",
	}
	ateP2 = attrs.Attributes{
		Name:    "ateP2",
		IPv4:    ateP2IPv4,
		IPv4Len: p2IPv4Len,
		IPv6:    ateP2IPv6,
		IPv6Len: p2IPv6Len,
		MAC:     "02:00:02:01:01:01",
	}
	ateP3 = attrs.Attributes{
		Name:    "ateP3",
		IPv4:    ateP3IPv4,
		IPv4Len: p3IPv4Len,
		IPv6:    ateP3IPv6,
		IPv6Len: p3IPv6Len,
		MAC:     "02:00:03:01:01:01",
	}
	ateP4 = attrs.Attributes{
		Name:    "ateP4",
		IPv4:    ateP4IPv4,
		IPv4Len: p4IPv4Len,
		IPv6:    ateP4IPv6,
		IPv6Len: p4IPv6Len,
		MAC:     "02:00:04:01:01:01",
	}
	dutlo0Attrs = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "10.1.1.1",
		IPv6:    "10:10:10:1::1",
		IPv4Len: 32,
		IPv6Len: 128,
	}
	loopbackIntfName string
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

	// Configure DUT and ATE with ISIS and BGP.
	configureDUT(t, dut)
	configureRoutePolicy(t, dut, rplName, rplType)
	otg := ate.OTG()
	top := configureOTG(t, otg)

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
					enableMultipath(t, dut, maxpaths, true)
				} else {
					t.Logf("Multipath is disabled for IPv4 prefixes: [%s, %d]", prefixMin, prefixLen)
				}
				verifyBGPSessionTelemetry(t, dut)
				verifyBGPPrefixesTelemetry(t, dut, []string{ateP2Lo0IP, ateP3Lo0IP, ateP4Lo0IP}, 1, true)
				otg.StartTraffic(t)
				time.Sleep(30 * time.Second)
				otg.StopTraffic(t)
				otgutils.LogFlowMetrics(t, otg, top)
				otgutils.LogPortMetrics(t, otg, top)
				verifyTraffic(t, ate, "ipv4", 0)
				checkPacketLoss(t, ate, "ipv4")
				if tc.multipath {
					verifyECMPLoadBalance(t, ate, int(cfgplugins.PortCount4), 3)
				}
			}
			if tc.ipv6 {
				t.Logf("Validating traffic test for IPv6 prefixes: [%s, %d]", prefixV6Min, prefixV6Len)
				if tc.multipath {
					t.Logf("Multipath is enabled for IPv6 prefixes: [%s, %d]", prefixV6Min, prefixV6Len)
					enableMultipath(t, dut, maxpaths, false)
				} else {
					t.Logf("Multipath is disabled for IPv6 prefixes: [%s, %d]", prefixV6Min, prefixV6Len)
				}
				verifyBGPSessionTelemetry(t, dut)
				verifyBGPPrefixesTelemetry(t, dut, []string{ateP2Lo0IPv6, ateP3Lo0IPv6, ateP4Lo0IPv6}, 1, false)
				otg.StartTraffic(t)
				time.Sleep(30 * time.Second)
				otg.StopTraffic(t)
				otgutils.LogFlowMetrics(t, otg, top)
				otgutils.LogPortMetrics(t, otg, top)
				verifyTraffic(t, ate, "ipv6", 0)
				checkPacketLoss(t, ate, "ipv6")
				if tc.multipath {
					verifyECMPLoadBalance(t, ate, int(cfgplugins.PortCount4), 3)
				}
			}
		})
	}
}

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
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
	for _, p := range []*ondatra.Port{p1, p2, p3, p4} {
		gnmi.Await(t, dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), 10*time.Minute, oc.Interface_OperStatus_UP)
	}

	loopbackIntfName = netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1 := dutlo0Attrs.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, d.Interface(loopbackIntfName).Config(), loop1)
	} else {
		v4, ok := ipv4Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv4 = v4.GetIp()
		}
		v6, ok := ipv6Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutlo0Attrs.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutlo0Attrs.IPv6)
	}

	isisData := &cfgplugins.ISISGlobalParams{
		DUTArea:             isisAreaAddr,
		DUTSysID:            isisSysID,
		NetworkInstanceName: deviations.DefaultNetworkInstance(dut),
		ISISInterfaceNames:  []string{p2.Name(), p3.Name(), p4.Name()},
	}
	isisBatch := &gnmi.SetBatch{}
	cfgplugins.NewISIS(t, dut, isisData, isisBatch)
	isisBatch.Set(t, dut)
	t.Logf("ISIS config applied on DUT")

	t.Logf("Now configuring BGP config on DUT")
	// Configure BGP neighbors and peer groups on DUT.
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Delete(t, dut, dutConfPath.Config())
	dutConf := bgpCreateNbr(dutAS, ateAS, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	t.Logf("BGP config applied on DUT")
	time.Sleep(30 * time.Second)
}

type bgpNeighbor struct {
	as           uint32
	neighborip   string
	isV4         bool
	peerGrp      string
	localAddress string
}

// enableMultipath enables multipath for the given DUT device with the given maximum paths.
func enableMultipath(t *testing.T, dut *ondatra.DUTDevice, maxpaths uint32, ipv4 bool) {
	dni := deviations.DefaultNetworkInstance(dut)
	bgpPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgpProto := gnmi.Get(t, dut, bgpPath.Config())
	bgp := bgpProto.GetOrCreateBgp()
	cliConfig := fmt.Sprintf("router bgp %v\nmaximum-paths %v\n", dutAS, maxpaths)
	if !deviations.IbgpMultipathPathUnsupported(dut) {
		if deviations.EnableMultipathUnderAfiSafi(dut) {
			if ipv4 {
				bgp.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateUseMultiplePaths().GetOrCreateIbgp().MaximumPaths = ygot.Uint32(maxpaths)
			} else {
				bgp.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateUseMultiplePaths().GetOrCreateIbgp().MaximumPaths = ygot.Uint32(maxpaths)
			}
		} else {
			bgp.GetOrCreateGlobal().GetOrCreateUseMultiplePaths().GetOrCreateIbgp().MaximumPaths = ygot.Uint32(maxpaths)
		}
	} else {
		t.Logf("CLI config: \n%v", cliConfig)
		t.Logf("Now applying CLI config on DUT, sleep for 30 seconds")
		helpers.GnmiCLIConfig(t, dut, cliConfig)
		time.Sleep(30 * time.Second)
	}
}

// bgpCreateNbr creates a BGP neighbor configuration for the DUT with multiple paths.
// TODO: Add support for multiple paths and local address.
func bgpCreateNbr(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nbr1v4 := &bgpNeighbor{as: ateAS, neighborip: ateP2Lo0IP, isV4: true, peerGrp: peerGrpName1, localAddress: dutlo0Attrs.IPv4}
	nbr2v4 := &bgpNeighbor{as: ateAS, neighborip: ateP3Lo0IP, isV4: true, peerGrp: peerGrpName1, localAddress: dutlo0Attrs.IPv4}
	nbr3v4 := &bgpNeighbor{as: ateAS, neighborip: ateP4Lo0IP, isV4: true, peerGrp: peerGrpName1, localAddress: dutlo0Attrs.IPv4}
	nbr1v6 := &bgpNeighbor{as: ateAS, neighborip: ateP2Lo0IPv6, isV4: false, peerGrp: peerGrpName1, localAddress: dutlo0Attrs.IPv6}
	nbr2v6 := &bgpNeighbor{as: ateAS, neighborip: ateP3Lo0IPv6, isV4: false, peerGrp: peerGrpName1, localAddress: dutlo0Attrs.IPv6}
	nbr3v6 := &bgpNeighbor{as: ateAS, neighborip: ateP4Lo0IPv6, isV4: false, peerGrp: peerGrpName1, localAddress: dutlo0Attrs.IPv6}
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
	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1)
	pg1.PeerAs = ygot.Uint32(ateAS)
	pg1.PeerGroupName = ygot.String(peerGrpName1)
	pgV4AFI := pg1.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	pgV4AFI.SetEnabled(true)
	applyPolicyV4 := pgV4AFI.GetOrCreateApplyPolicy()
	applyPolicyV4.SetImportPolicy([]string{rplName})
	applyPolicyV4.SetExportPolicy([]string{rplName})
	pgV6AFI := pg1.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	pgV6AFI.SetEnabled(true)
	applyPolicyV6 := pgV6AFI.GetOrCreateApplyPolicy()
	applyPolicyV6.SetImportPolicy([]string{rplName})
	applyPolicyV6.SetExportPolicy([]string{rplName})
	for _, nbr := range nbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.neighborip)
		bgpNbr.PeerGroup = ygot.String(peerGrpName1)
		bgpNbr.Enabled = ygot.Bool(true)
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

func configureOTG(t *testing.T, otg *otg.OTG) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")
	port3 := config.Ports().Add().SetName("port3")
	port4 := config.Ports().Add().SetName("port4")

	// Port1 Configuration.
	iDut1Dev := config.Devices().Add().SetName(ateP1.Name)

	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(ateP1.Name + ".Eth").SetMac(ateP1.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(ateP1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(ateP1.IPv4).SetGateway(dutP1.IPv4).SetPrefix(uint32(ateP1.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(ateP1.Name + ".IPv6")
	iDut1Ipv6.SetAddress(ateP1.IPv6).SetGateway(dutP1.IPv6).SetPrefix(uint32(ateP1.IPv6Len))

	portsInfo := []struct {
		id             int
		p              gosnappi.Port
		ate            attrs.Attributes
		dut            attrs.Attributes
		loopbackAddrV4 string
		loopbackAddrV6 string
		isisSystemID   string
	}{
		{2, port2, ateP2, dutP2, ateP2Lo0IP, ateP2Lo0IPv6, otgSysID2},
		{3, port3, ateP3, dutP3, ateP3Lo0IP, ateP3Lo0IPv6, otgSysID3},
		{4, port4, ateP4, dutP4, ateP4Lo0IP, ateP4Lo0IPv6, otgSysID4},
	}

	var rxIpv4IntfNames []string
	var rxIpv6IntfNames []string
	for _, p := range portsInfo {
		devName := p.ate.Name
		iDev := config.Devices().Add().SetName(devName)
		iEth := iDev.Ethernets().Add().SetName(devName + ".Eth").SetMac(p.ate.MAC)
		iEth.Connection().SetPortName(p.p.Name())
		iIpv4 := iEth.Ipv4Addresses().Add().SetName(devName + ".IPv4")
		iIpv4.SetAddress(p.ate.IPv4).SetGateway(p.dut.IPv4).SetPrefix(uint32(p.ate.IPv4Len))
		iIpv6 := iEth.Ipv6Addresses().Add().SetName(devName + ".IPv6")
		iIpv6.SetAddress(p.ate.IPv6).SetGateway(p.dut.IPv6).SetPrefix(uint32(p.ate.IPv6Len))

		rxIpv4IntfNames = append(rxIpv4IntfNames, iIpv4.Name())
		rxIpv6IntfNames = append(rxIpv6IntfNames, iIpv6.Name())

		// Loopback Configuration.
		iLoopV4 := iDev.Ipv4Loopbacks().Add().SetName(fmt.Sprintf("Port%dLoopV4", p.id)).SetEthName(iEth.Name())
		iLoopV4.SetAddress(p.loopbackAddrV4)
		iLoopV6 := iDev.Ipv6Loopbacks().Add().SetName(fmt.Sprintf("Port%dLoopV6", p.id)).SetEthName(iEth.Name())
		iLoopV6.SetAddress(p.loopbackAddrV6)

		// ISIS configuration for iBGP session establishment.
		isis := iDev.Isis().SetName(fmt.Sprintf("ISIS%d", p.id)).SetSystemId(p.isisSystemID)
		isis.Basic().SetIpv4TeRouterId(p.ate.IPv4).SetHostname(isis.Name()).SetLearnedLspFilter(true)
		isis.Basic().SetEnableWideMetric(true)
		isis.Interfaces().Add().SetEthName(iDev.Ethernets().Items()[0].Name()).
			SetName(fmt.Sprintf("devIsisInt%d", p.id)).
			SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
			SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

		// Advertise OTG loopback address via ISIS.
		isisV4 := iDev.Isis().V4Routes().Add().SetName(fmt.Sprintf("ISISPort%dV4", p.id)).SetLinkMetric(10)
		isisV4.Addresses().Add().SetAddress(p.loopbackAddrV4).SetPrefix(32)
		isisV6 := iDev.Isis().V6Routes().Add().SetName(fmt.Sprintf("ISISPort%dV6", p.id)).SetLinkMetric(10)
		isisV6.Addresses().Add().SetAddress(p.loopbackAddrV6).SetPrefix(uint32(128))

		iBgp := iDev.Bgp().SetRouterId(p.loopbackAddrV4)
		iBgp4Peer := iBgp.Ipv4Interfaces().Add().SetIpv4Name(iLoopV4.Name()).Peers().Add().SetName(devName + ".BGP4.peer")
		iBgp4Peer.SetPeerAddress(dutlo0Attrs.IPv4).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
		iBgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
		iBgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
		// iBGP - v6 session.
		iBgp6Peer := iBgp.Ipv6Interfaces().Add().SetIpv6Name(iLoopV6.Name()).Peers().Add().SetName(devName + ".BGP6.peer")
		iBgp6Peer.SetPeerAddress(dutlo0Attrs.IPv6).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
		iBgp6Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
		iBgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

		// iBGP V4 routes.
		bgpNetiBgp4PeerRoutes := iBgp4Peer.V4Routes().Add().SetName(devName + ".BGP4.Route")
		bgpNetiBgp4PeerRoutes.SetNextHopIpv4Address(p.loopbackAddrV4).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		bgpNetiBgp4PeerRoutes.Addresses().Add().
			SetAddress(prefixMin).SetPrefix(24)
		bgpNetiBgp4PeerRoutes.AddPath().SetPathId(1)

		// iBGP V6 routes.
		bgpNetiBgp6PeerRoutes := iBgp6Peer.V6Routes().Add().SetName(devName + ".BGP6.Route")
		bgpNetiBgp6PeerRoutes.SetNextHopIpv6Address(p.loopbackAddrV6).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
			SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		bgpNetiBgp6PeerRoutes.Addresses().Add().
			SetAddress(prefixV6Min).SetPrefix(64)
		bgpNetiBgp6PeerRoutes.AddPath().SetPathId(1)
	}

	// Add traffic flows.
	t.Logf("Adding traffic flows...")
	config.Flows().Clear()
	trafficFlows := []struct {
		desc     string
		dscpSet  []uint32
		ipType   string
		srcIP    string
		dstIP    string
		priority int
	}{{desc: "Traffic IPv4", dscpSet: []uint32{0}, ipType: "ipv4", srcIP: ateP1IPv4, dstIP: prefixMin, priority: 0},
		{desc: "Traffic IPv6", dscpSet: []uint32{0}, ipType: "ipv6", srcIP: ateP1IPv6, dstIP: prefixV6Min, priority: 0},
	}
	t.Logf("Traffic config: %v", trafficFlows)
	for _, tc := range trafficFlows {
		trafficID := tc.desc
		flow := config.Flows().Add().SetName(trafficID)
		flow.Metrics().SetEnable(true)
		ethHeader := flow.Packet().Add().Ethernet()
		ethHeader.Src().SetValue(ateP1.MAC)
		ethHeader.Dst().Auto()
		switch tc.ipType {
		case "ipv4":
			flow.TxRx().Device().SetTxNames([]string{iDut1Ipv4.Name()}).SetRxNames(rxIpv4IntfNames)
			ipHeader := flow.Packet().Add().Ipv4()
			ipHeader.Src().SetValue(tc.srcIP)
			ipHeader.Dst().Increment().SetStart(tc.dstIP).SetCount(100)
			udpHeader := flow.Packet().Add().Udp()
			udpHeader.SrcPort().SetValue(14)
			udpHeader.DstPort().Increment().SetStart(1024).SetCount(10000)
		case "ipv6":
			flow.TxRx().Device().SetTxNames([]string{iDut1Ipv6.Name()}).SetRxNames(rxIpv6IntfNames)
			ipHeader := flow.Packet().Add().Ipv6()
			ipHeader.Src().SetValue(tc.srcIP)
			ipHeader.Dst().Increment().SetStart(tc.dstIP).SetCount(100)
			udpHeader := flow.Packet().Add().Udp()
			udpHeader.SrcPort().SetValue(14)
			udpHeader.DstPort().Increment().SetStart(1024).SetCount(10000)
		}
		flow.Size().SetFixed(uint32(packetSize))
		flow.Rate().SetPps(trafficPps)
		flow.Duration().FixedPackets().SetPackets(totalPackets)
	}

	t.Logf("Pushing config to OTG")
	otg.PushConfig(t, config)
	time.Sleep(1 * time.Minute)

	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, config, "IPv4")
	otgutils.WaitForARP(t, otg, config, "IPv6")

	return config
}

func verifyBGPPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, nbrs []string, wantRx uint32, isV4 bool) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrs {
		t.Logf("Prefix telemetry on DUT for peer %v", nbr)
		var prefixPath *netinstbgp.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_PrefixesPath
		if isV4 {
			prefixPath = statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
		} else {
			prefixPath = statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()
		}
		if gotRx, ok := gnmi.Watch(t, dut, prefixPath.ReceivedPrePolicy().State(), 30*time.Second, func(val *ygnmi.Value[uint32]) bool {
			gotRx, ok := val.Val()
			return ok && gotRx == wantRx
		}).Await(t); !ok {
			t.Errorf("Received prefixes mismatch for neighbor %s: got %v, want %v", nbr, gotRx, wantRx)
		}
	}
}

func verifyBGPSessionTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	t.Logf("Verifying DUT BGP sessions up for IPv4")
	for _, neighborIP := range []string{ateP2Lo0IP, ateP3Lo0IP, ateP4Lo0IP} {
		gnmi.Await(t, dut, bgpPath.Neighbor(neighborIP).SessionState().State(), 2*time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	}
	t.Logf("Verifying DUT BGP sessions up for IPv6")
	for _, neighborIP := range []string{ateP2Lo0IPv6, ateP3Lo0IPv6, ateP4Lo0IPv6} {
		gnmi.Await(t, dut, bgpPath.Neighbor(neighborIP).SessionState().State(), 2*time.Minute, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	}
}

func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, prefixType string, index int) {
	flowName := "Traffic IPv4"
	if prefixType == "ipv6" {
		flowName = "Traffic IPv6"
	}
	recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
	framesTx := recvMetric.GetCounters().GetOutPkts()
	framesRx := recvMetric.GetCounters().GetInPkts()

	if framesTx == 0 {
		t.Error("No traffic was generated and frames transmitted were 0")
	} else if framesRx == framesTx {
		t.Logf("Traffic validation successful FramesTx: %d FramesRx: %d", framesTx, framesRx)
	}
}

func checkPacketLoss(t *testing.T, ate *ondatra.ATEDevice, ipType string) {
	flowName := "Traffic IPv4"
	if ipType == "ipv6" {
		flowName = "Traffic IPv6"
	}
	countersPath := gnmi.OTG().Flow(flowName).Counters()
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
	for i := 2; i <= pc; i++ {
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
