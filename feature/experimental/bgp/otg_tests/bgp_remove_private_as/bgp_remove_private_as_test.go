// Copyright 2023 Google LLC
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

package bgp_remove_private_as_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// The testbed consists of ate:port1 -> dut:port1 and
// dut:port2 -> ate:port2.  The first pair is called the "source"
// pair, and the second the "destination" pair.
//
// Source: ate:port1 -> dut:port1 subnet 192.0.2.0/30
// Destination: dut:port2 -> ate:port2 subnet 192.0.2.4/30
//
// Note that the first (.0, .3) and last (.4, .7) IPv4 addresses are
// reserved from the subnet for broadcast, so a /30 leaves exactly 2
// usable addresses. This does not apply to IPv6 which allows /127
// for point to point links, but we use /126 so the numbering is
// consistent with IPv4.

const (
	advertisedRoutesv4Net    = "203.0.113.1"
	advertisedRoutesv4Prefix = 32
	peerGrpName1             = "BGP-PEER-GROUP1"
	peerGrpName2             = "BGP-PEER-GROUP2"
	routeCount               = 254
	dutAS                    = 500
	ateAS1                   = 100
	ateAS2                   = 200
	plenIPv4                 = 30
	plenIPv6                 = 126
	removeASPath             = true
	policyName               = "ALLOW"
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateDst = attrs.Attributes{
		Name:    "ateDst",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// configreRoutePolicy adds route-policy config.
func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	stc := st.GetOrCreateConditions()
	stc.InstallProtocolEq = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutDst.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)
}

// verifyPortsUp asserts that each port on the device is operating.
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

// bgpCreateNbr creates a BGP object with neighbors pointing to ateSrc and ateDst.
func bgpCreateNbr(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	nbr1v4 := &bgpNeighbor{as: ateAS1, neighborip: ateSrc.IPv4, isV4: true, peerGrp: peerGrpName1}
	nbr2v4 := &bgpNeighbor{as: ateAS2, neighborip: ateDst.IPv4, isV4: true, peerGrp: peerGrpName2}
	nbrs := []*bgpNeighbor{nbr1v4, nbr2v4}

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutDst.IPv4)
	global.As = ygot.Uint32(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1)
	pg1.PeerAs = ygot.Uint32(ateAS1)
	pg1.PeerGroupName = ygot.String(peerGrpName1)

	pg2 := bgp.GetOrCreatePeerGroup(peerGrpName2)
	pg2.PeerAs = ygot.Uint32(ateAS2)
	pg2.PeerGroupName = ygot.String(peerGrpName2)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg1.GetOrCreateApplyPolicy()
		rpl.ImportPolicy = []string{policyName}
		rpl.ExportPolicy = []string{policyName}

		rp2 := pg2.GetOrCreateApplyPolicy()
		rp2.ImportPolicy = []string{policyName}
		rp2.ExportPolicy = []string{policyName}
	} else {
		pgaf := pg1.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pgaf.Enabled = ygot.Bool(true)
		rpl := pgaf.GetOrCreateApplyPolicy()
		rpl.ImportPolicy = []string{policyName}
		rpl.ExportPolicy = []string{policyName}

		pgaf2 := pg2.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pgaf2.Enabled = ygot.Bool(true)
		rp2 := pgaf2.GetOrCreateApplyPolicy()
		rp2.ImportPolicy = []string{policyName}
		rp2.ExportPolicy = []string{policyName}
	}

	for _, nbr := range nbrs {
		nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
		nv4.PeerGroup = ygot.String(nbr.peerGrp)
		nv4.PeerAs = ygot.Uint32(nbr.as)
		nv4.Enabled = ygot.Bool(true)
		af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.Enabled = ygot.Bool(true)
		af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af6.Enabled = ygot.Bool(false)
	}
	return niProto
}

// verifyBGPTelemetry checks that the dut has an established BGP session with reasonable settings.
func verifyBGPTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var nbrIP = []string{ateSrc.IPv4, ateDst.IPv4}
	t.Logf("Verifying BGP state.")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrIP {
		var status *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]
		nbrPath := statePath.Neighbor(nbr)
		t.Logf("Waiting for BGP neighbor to establish...")
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		t.Logf("BGP adjacency for %s: %s", nbr, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %v, want %d", nbr, status, want)
		}
	}
}

// verifyPrefixesTelemetry confirms that the dut shows the correct numbers of installed,
// sent and received IPv4 prefixes.
func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, nbr string, wantInstalled, wantSent uint32) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv4 := statePath.Neighbor(nbr).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	if gotInstalled := gnmi.Get(t, dut, prefixesv4.Installed().State()); gotInstalled != wantInstalled {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}
	if gotSent := gnmi.Get(t, dut, prefixesv4.Sent().State()); gotSent != wantSent {
		t.Errorf("Sent prefixes mismatch: got %v, want %v", gotSent, wantSent)
	}
}

// configureOTG configures the interfaces and BGP protocols on an ATE, including
// advertising some(faked) networks over BGP.
func configureOTG(t *testing.T, otg *otg.OTG, asSeg []uint32, asSEQMode bool) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	iDut1Dev := config.Devices().Add().SetName(ateSrc.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	iDut1Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	iDut1Ipv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	iDut1Ipv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	iDut2Dev := config.Devices().Add().SetName(ateDst.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	iDut2Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4")
	iDut2Ipv4.SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6")
	iDut2Ipv6.SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP4.peer")
	iDut1Bgp4Peer.SetPeerAddress(dutSrc.IPv4).SetAsNumber(ateAS1).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut1Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	iDut2Bgp := iDut2Dev.Bgp().SetRouterId(iDut2Ipv4.Address())
	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2Ipv4.Name()).Peers().Add().SetName(ateDst.Name + ".BGP4.peer")
	iDut2Bgp4Peer.SetPeerAddress(dutDst.IPv4).SetAsNumber(ateAS2).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut2Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	bgpNeti1Bgp4PeerRoutes := iDut1Bgp4Peer.V4Routes().Add().SetName(ateSrc.Name + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(iDut1Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	bgpNeti1Bgp4PeerRoutes.Addresses().Add().
		SetAddress(advertisedRoutesv4Net).
		SetPrefix(uint32(advertisedRoutesv4Prefix)).
		SetCount(routeCount)

	bgpNeti1AsPath := bgpNeti1Bgp4PeerRoutes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SET)

	if asSEQMode {
		bgpNeti1AsPath.Segments().Add().SetAsNumbers(asSeg).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)
	} else {
		bgpNeti1AsPath.Segments().Add().SetAsNumbers(asSeg).SetType(gosnappi.BgpAsPathSegmentType.AS_SET)
	}

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)

	return config
}

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
	peerGrp    string
}

func verifyOTGBGPTelemetry(t *testing.T, otg *otg.OTG, c gosnappi.Config, state string) {
	t.Helper()
	for _, d := range c.Devices().Items() {
		for _, ip := range d.Bgp().Ipv4Interfaces().Items() {
			for _, configPeer := range ip.Peers().Items() {
				nbrPath := gnmi.OTG().BgpPeer(configPeer.Name())
				_, ok := gnmi.Watch(t, otg, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
					currState, ok := val.Val()
					return ok && currState.String() == state
				}).Await(t)
				if !ok {
					t.Errorf("No BGP neighbor formed for peer %s", configPeer.Name())
				}
			}
		}
		for _, ip := range d.Bgp().Ipv6Interfaces().Items() {
			for _, configPeer := range ip.Peers().Items() {
				nbrPath := gnmi.OTG().BgpPeer(configPeer.Name())
				_, ok := gnmi.Watch(t, otg, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
					currState, ok := val.Val()
					return ok && currState.String() == state
				}).Await(t)
				if !ok {
					t.Errorf("No BGP neighbor formed for peer %s", configPeer.Name())
				}
			}
		}
	}
}

// verifyBGPAsPath is to Validate AS Path attribute using bgp rib telemetry on ATE.
func verifyBGPAsPath(t *testing.T, otg *otg.OTG, config gosnappi.Config, asSeg []uint32, removeASPath bool) {
	t.Helper()
	_, ok := gnmi.WatchAll(t, otg, gnmi.OTG().BgpPeer(ateDst.Name+".BGP4.peer").UnicastIpv4PrefixAny().State(),
		time.Minute, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			return v.IsPresent()
		}).Await(t)

	var wantASSeg = []uint32{dutAS, ateAS1}
	if removeASPath {
		for _, as := range asSeg {
			if as < 64512 {
				wantASSeg = append(wantASSeg, as)
			}
		}
	} else {
		wantASSeg = append(wantASSeg, asSeg...)
	}

	if ok {
		bgpPrefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(ateDst.Name+".BGP4.peer").UnicastIpv4PrefixAny().State())
		gotPrefixCount := len(bgpPrefixes)
		if gotPrefixCount != routeCount {
			t.Errorf("Received prefixes on otg are not as expected got prefixes %v, want prefixes %v", gotPrefixCount, routeCount)
		}
		for _, prefix := range bgpPrefixes {
			for _, gotASSeg := range prefix.AsPath {
				if ok := cmp.Diff(gotASSeg.AsNumbers, wantASSeg); ok != "" {
					t.Errorf("Remove private AS is not working: gotAsSeg %v wantAsSeg %v for Prefix %v", gotASSeg, wantASSeg, prefix.GetAddress())
				}
			}
		}
	}
}

// TestRemovePrivateAS is to Validate that private AS numbers are stripped
// before advertisement to the eBGP neighbor.
func TestRemovePrivateAS(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	var otgConfig gosnappi.Config

	t.Run("Configure DUT interfaces", func(t *testing.T) {
		configureDUT(t, dut)
	})

	t.Run("Configure DEFAULT network instance", func(t *testing.T) {
		dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
		gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	})

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("Configure BGP Neighbors", func(t *testing.T) {
		gnmi.Delete(t, dut, dutConfPath.Config())
		configureRoutePolicy(t, dut, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
		dutConf := bgpCreateNbr(dutAS, ateAS1, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.GetConfig(t, dut, dutConfPath.Config()))
	})

	cases := []struct {
		desc      string
		asSeg     []uint32
		asSEQMode bool
	}{{
		desc:      "AS Path SEQ 65501",
		asSeg:     []uint32{65501},
		asSEQMode: true,
	}, {
		desc:      "AS Path SEQ 65501, 65507",
		asSeg:     []uint32{65501, 65507},
		asSEQMode: true,
	}, {
		desc:      "AS Path SEQ 800, 65501, 65508",
		asSeg:     []uint32{800, 65501, 65508},
		asSEQMode: true,
	}, {
		desc:      "AS Path SEQ 65501, 600",
		asSeg:     []uint32{65501, 600},
		asSEQMode: true,
	}, {
		desc:      "AS Path SEQ 800, 65507, 600",
		asSeg:     []uint32{800, 65507, 600},
		asSEQMode: true,
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Logf("Start OTG Config.")
			otgConfig = configureOTG(t, otg, tc.asSeg, tc.asSEQMode)

			t.Log("Verifying port status.")
			verifyPortsUp(t, dut.Device)

			t.Log("Check BGP parameters.")
			verifyBGPTelemetry(t, dut)
			verifyOTGBGPTelemetry(t, otg, otgConfig, "ESTABLISHED")
			t.Log("Verify BGP prefix telemetry.")
			verifyPrefixesTelemetry(t, dut, ateSrc.IPv4, routeCount, 0)
			verifyPrefixesTelemetry(t, dut, ateDst.IPv4, 0, routeCount)

			t.Log("Verify AS Path list received at ate Port2 including private AS number.")
			verifyBGPAsPath(t, otg, otgConfig, tc.asSeg, !removeASPath)

			t.Log("Configure remove private AS on DUT.")
			gnmi.Update(t, dut, dutConfPath.Bgp().PeerGroup(peerGrpName2).RemovePrivateAs().Config(), oc.Bgp_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL)

			t.Log("Private AS numbers should be stripped off while advertising BGP routes into public AS.")
			verifyBGPAsPath(t, otg, otgConfig, tc.asSeg, removeASPath)

			otg.StopProtocols(t)
			time.Sleep(30 * time.Second)

			t.Log("Remove remove-private-AS on DUT.")
			gnmi.Delete(t, dut, dutConfPath.Bgp().PeerGroup(peerGrpName2).RemovePrivateAs().Config())
		})
	}
}
