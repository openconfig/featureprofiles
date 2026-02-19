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

package bgp_override_as_path_split_horizon_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	advertisedRoutesv4CIDR      = "203.0.113.1/32"
	advertisedRoutesv4Net       = "203.0.113.1"
	advertisedRoutesv4Prefix    = 32
	advertisedRoutesv4PrefixLen = "32..32"
	peerGrpName1                = "BGP-PEER-GROUP1"
	peerGrpName2                = "BGP-PEER-GROUP2"
	dutGlobalAS                 = 64512
	dutLocalAS1                 = 65501
	dutLocalAS2                 = 64513
	ateAS1                      = 65502
	ateAS2                      = 65503
	plenIPv4                    = 30
	plenIPv6                    = 126
	policyName                  = "ALLOW"
	prefixSetName               = "prefSet"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE Port2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	nbr1 = &cfgplugins.BgpNeighbor{LocalAS: dutLocalAS1, PeerAS: ateAS1, Neighborip: atePort1.IPv4, IsV4: true, PeerGrp: peerGrpName1}
	nbr2 = &cfgplugins.BgpNeighbor{LocalAS: dutLocalAS2, PeerAS: ateAS2, Neighborip: atePort2.IPv4, IsV4: true, PeerGrp: peerGrpName2}

	otgPort1V4Peer = "atePort1.BGP4.peer"
	otgPort2V4Peer = "atePort2.BGP4.peer"
)

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutPort1.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutPort2.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, i2.GetName(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func bgpCreateNbr(t *testing.T, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	t.Helper()
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutPort2.IPv4)
	global.As = ygot.Uint32(dutGlobalAS)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	for _, nbr := range []*cfgplugins.BgpNeighbor{nbr1, nbr2} {
		pg := bgp.GetOrCreatePeerGroup(nbr.PeerGrp)
		pg.PeerAs = ygot.Uint32(nbr.PeerAS)
		pg.LocalAs = ygot.Uint32(nbr.LocalAS)
		pg.PeerGroupName = ygot.String(nbr.PeerGrp)
		pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

		nv4 := bgp.GetOrCreateNeighbor(nbr.Neighborip)
		nv4.PeerGroup = ygot.String(nbr.PeerGrp)
		nv4.PeerAs = ygot.Uint32(nbr.PeerAS)
		nv4.Enabled = ygot.Bool(true)
		af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)

		if deviations.RoutePolicyUnderAFIUnsupported(dut) {
			rpl := pg.GetOrCreateApplyPolicy()
			rpl.ImportPolicy = []string{policyName}
			rpl.ExportPolicy = []string{policyName}
		} else {
			pgaf := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			pgaf.Enabled = ygot.Bool(true)
			rpl := pgaf.GetOrCreateApplyPolicy()
			rpl.ImportPolicy = []string{policyName}
			rpl.ExportPolicy = []string{policyName}
		}
	}
	return niProto
}

func configureOTG(t *testing.T, otg *otg.OTG) (gosnappi.BgpV4Peer, gosnappi.DeviceIpv4, gosnappi.Config) {
	t.Helper()
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	iDut1Dev := config.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	iDut2Dev := config.Devices().Add().SetName(atePort2.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	iDut2Eth.Connection().SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	iDut2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	iDut2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut2Bgp := iDut2Dev.Bgp().SetRouterId(iDut2Ipv4.Address())

	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(otgPort1V4Peer)
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(ateAS1).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2Ipv4.Name()).Peers().Add().SetName(otgPort2V4Peer)
	iDut2Bgp4Peer.SetPeerAddress(iDut2Ipv4.Gateway()).SetAsNumber(ateAS2).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)

	return iDut1Bgp4Peer, iDut1Ipv4, config
}

func advBGPRouteFromOTG(t *testing.T, args *otgTestArgs, asSeg []uint32) {
	args.otgBgpPeer.V4Routes().Clear()
	bgpNeti1Bgp4PeerRoutes := args.otgBgpPeer.V4Routes().Add().SetName(atePort1.Name + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(args.otgIPv4Device.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv4Net).
		SetPrefix(uint32(advertisedRoutesv4Prefix)).
		SetCount(1)

	bgpNeti1AsPath := bgpNeti1Bgp4PeerRoutes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SEQ)
	bgpNeti1AsPath.Segments().Add().SetAsNumbers(asSeg).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)

	t.Logf("Pushing config to OTG and starting protocols...")
	args.otg.PushConfig(t, args.otgConfig)
	time.Sleep(30 * time.Second)
	args.otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
}

func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, nbr string, wantInstalled, wantSent uint32) {
	t.Helper()
	time.Sleep(15 * time.Second)
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv4 := statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	if gotInstalled := gnmi.Get(t, dut, prefixesv4.Installed().State()); gotInstalled != wantInstalled {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}
	if gotSent := gnmi.Get(t, dut, prefixesv4.Sent().State()); gotSent != wantSent {
		t.Errorf("Sent prefixes mismatch: got %v, want %v", gotSent, wantSent)
	}
}

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	prefixSet := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSetName)
	prefixSet.GetOrCreatePrefix(advertisedRoutesv4CIDR, advertisedRoutesv4PrefixLen)
	pdef := rp.GetOrCreatePolicyDefinition(name)
	stmt, err := pdef.AppendNewStatement(name)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", name, err)
	}
	stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(prefixSetName)
	stmt.GetOrCreateActions().PolicyResult = pr
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func verifyOTGPrefixTelemetry(t *testing.T, otg *otg.OTG, wantPrefix bool) {
	t.Helper()
	_, ok := gnmi.WatchAll(t, otg, gnmi.OTG().BgpPeer(atePort2.Name+".BGP4.peer").UnicastIpv4PrefixAny().State(),
		time.Minute, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			return v.IsPresent()
		}).Await(t)

	if ok {
		bgpPrefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(atePort2.Name+".BGP4.peer").UnicastIpv4PrefixAny().State())
		for _, prefix := range bgpPrefixes {
			if prefix.GetAddress() == advertisedRoutesv4Net {
				if wantPrefix {
					if len(prefix.AsPath) == 0 {
						t.Errorf("Prefix %v received but AS-PATH is empty", prefix.GetAddress())
						continue
					}
					gotASPath := prefix.AsPath[len(prefix.AsPath)-1].GetAsNumbers()
					t.Logf("Received prefix %v on otg as expected with AS-PATH %v", prefix.GetAddress(), gotASPath)
				} else {
					t.Errorf("Prefix %v is received on otg when it is not expected", prefix.GetAddress())
				}
			}
		}
	} else if wantPrefix {
		t.Errorf("Timed out waiting for prefix %v on OTG", advertisedRoutesv4Net)
	}
}

func testSplitHorizonNoAllowOwnIn(t *testing.T, args *otgTestArgs) {
	t.Log("Baseline Test No allow-own-in")
	advBGPRouteFromOTG(t, args, []uint32{65500, dutLocalAS1, 65499})
	cfgplugins.VerifyDUTBGPEstablished(t, args.dut)
	verifyPrefixesTelemetry(t, args.dut, nbr1.Neighborip, 0, 0)
	verifyOTGPrefixTelemetry(t, args.otg, false)
}

func testSplitHorizonAllowOwnAs1(t *testing.T, args *otgTestArgs) {
	t.Log("Test allow-own-as 1, Enable allow-own-as 1 on the DUT.")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Replace(t, args.dut, dutConfPath.Bgp().PeerGroup(peerGrpName1).AsPathOptions().AllowOwnAs().Config(), uint8(1))

	advBGPRouteFromOTG(t, args, []uint32{65500, dutLocalAS1, 65499})
	verifyPrefixesTelemetry(t, args.dut, nbr1.Neighborip, 1, 0)
	verifyOTGPrefixTelemetry(t, args.otg, true)
}

func testSplitHorizonAllowOwnAs3(t *testing.T, args *otgTestArgs) {
	t.Log("Test allow-own-as 3, Enable allow-own-as 3 on the DUT.")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Replace(t, args.dut, dutConfPath.Bgp().PeerGroup(peerGrpName1).AsPathOptions().AllowOwnAs().Config(), uint8(3))

	t.Run("Re-advertise with 3 Occurrences", func(t *testing.T) {
		advBGPRouteFromOTG(t, args, []uint32{dutLocalAS1, dutLocalAS1, dutLocalAS1, 65499})
		verifyPrefixesTelemetry(t, args.dut, nbr1.Neighborip, 1, 0)
		verifyOTGPrefixTelemetry(t, args.otg, true)
	})
	t.Run("Re-advertise with 4 Occurrences (Reject)", func(t *testing.T) {
		advBGPRouteFromOTG(t, args, []uint32{dutLocalAS1, dutLocalAS1, dutLocalAS1, dutLocalAS1, 65499})
		verifyPrefixesTelemetry(t, args.dut, nbr1.Neighborip, 0, 0)
		verifyOTGPrefixTelemetry(t, args.otg, false)
	})
}

func testSplitHorizonAllowOwnAs4(t *testing.T, args *otgTestArgs) {
	t.Log("Test allow-own-as 4, Enable allow-own-as 4 on the DUT.")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Replace(t, args.dut, dutConfPath.Bgp().PeerGroup(peerGrpName1).AsPathOptions().AllowOwnAs().Config(), uint8(4))

	advBGPRouteFromOTG(t, args, []uint32{dutLocalAS1, dutLocalAS1, dutLocalAS1, dutLocalAS1, 65499})
	verifyPrefixesTelemetry(t, args.dut, nbr1.Neighborip, 1, 0)
	verifyOTGPrefixTelemetry(t, args.otg, true)
}

func testSplitHorizonOriginatingAS(t *testing.T, args *otgTestArgs) {
	t.Log("Test RT-1.54.5: DUT's AS as Originating AS")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Replace(t, args.dut, dutConfPath.Bgp().PeerGroup(peerGrpName1).AsPathOptions().AllowOwnAs().Config(), uint8(1))

	advBGPRouteFromOTG(t, args, []uint32{65502, 65500, dutLocalAS1})
	verifyPrefixesTelemetry(t, args.dut, nbr1.Neighborip, 1, 0)
	verifyOTGPrefixTelemetry(t, args.otg, true)
}

type otgTestArgs struct {
	dut           *ondatra.DUTDevice
	ate           *ondatra.ATEDevice
	otgBgpPeer    gosnappi.BgpV4Peer
	otgIPv4Device gosnappi.DeviceIpv4
	otgConfig     gosnappi.Config
	otg           *otg.OTG
}

func TestBGPOverrideASPathSplitHorizon(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run("Configure DUT and OTG", func(t *testing.T) {
		configureDUT(t, dut)
		fptest.ConfigureDefaultNetworkInstance(t, dut)
		configureRoutePolicy(t, dut, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
		cfgplugins.BGPClearConfig(t, dut)
		dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
		gnmi.Replace(t, dut, dutConfPath.Config(), bgpCreateNbr(t, dut))
	})

	otg := ate.OTG()
	otgBgpPeer, otgIPv4Device, otgConfig := configureOTG(t, otg)
	args := &otgTestArgs{dut: dut, ate: ate, otgBgpPeer: otgBgpPeer, otgIPv4Device: otgIPv4Device, otgConfig: otgConfig, otg: otg}

	cases := []struct {
		desc     string
		funcName func()
	}{
		{desc: "RT-1.54.1: Baseline (Reject)", funcName: func() { testSplitHorizonNoAllowOwnIn(t, args) }},
		{desc: "RT-1.54.2: Allow 1", funcName: func() { testSplitHorizonAllowOwnAs1(t, args) }},
		{desc: "RT-1.54.3: Allow 3", funcName: func() { testSplitHorizonAllowOwnAs3(t, args) }},
		{desc: "RT-1.54.4: Allow 4", funcName: func() { testSplitHorizonAllowOwnAs4(t, args) }},
		{desc: "RT-1.54.5: Originating AS", funcName: func() { testSplitHorizonOriginatingAS(t, args) }},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) { tc.funcName() })
	}
}
