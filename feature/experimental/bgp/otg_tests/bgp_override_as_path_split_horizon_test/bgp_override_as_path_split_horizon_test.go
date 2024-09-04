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

	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
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
	advertisedRoutesv4CIDR   = "203.0.113.1/24"
	advertisedRoutesv4Net    = "203.0.113.1"
	advertisedRoutesv4Prefix = 24
	peerGrpName1             = "BGP-PEER-GROUP1"
	peerGrpName2             = "BGP-PEER-GROUP2"
	dutAS                    = 65501
	ateAS1                   = 65502
	ateAS2                   = 65502
	plenIPv4                 = 30
	plenIPv6                 = 126
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

	nbr1 = &bgpNeighbor{as: ateAS1, neighborip: atePort1.IPv4, isV4: true, peerGrp: peerGrpName1}
	nbr2 = &bgpNeighbor{as: ateAS2, neighborip: atePort2.IPv4, isV4: true, peerGrp: peerGrpName2}

	otgPort1V4Peer = "atePort1.BGP4.peer"
	otgPort2V4Peer = "atePort2.BGP4.peer"
)

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutPort1.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutPort2.NewOCInterface(dut.Port(t, "port2").Name(), dut)
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

// bgpCreateNbr creates a BGP object with neighbors pointing to atePort1 and atePort2
func bgpCreateNbr(t *testing.T, dut *ondatra.DUTDevice, localAs uint32) *oc.NetworkInstance_Protocol {
	t.Helper()
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutPort2.IPv4)
	global.As = ygot.Uint32(localAs)

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.

	for _, nbr := range []*bgpNeighbor{nbr1, nbr2} {

		pg := bgp.GetOrCreatePeerGroup(nbr.peerGrp)
		pg.PeerAs = ygot.Uint32(nbr.as)
		pg.LocalAs = ygot.Uint32(localAs)
		pg.PeerGroupName = ygot.String(nbr.peerGrp)

		nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
		nv4.PeerGroup = ygot.String(nbr.peerGrp)
		nv4.PeerAs = ygot.Uint32(nbr.as)
		nv4.Enabled = ygot.Bool(true)
		af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af4.GetOrCreateApplyPolicy().SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE)
		af4.Enabled = ygot.Bool(true)
	}
	return niProto
}

// verifyBgpTelemetry checks that the dut has an established BGP session with reasonable settings.
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice, nbrsList []*bgpNeighbor) {
	t.Helper()
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrsList {
		nbrPath := bgpPath.Neighbor(nbr.neighborip)
		t.Logf("Waiting for BGP neighbor to establish...")
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		t.Logf("BGP adjacency for %s: %v", nbr.neighborip, state)
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbr.neighborip, state, want)
		}
	}
}

// configureOTG configures the interfaces and BGP protocols on an ATE.
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
	iDut1Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2Ipv4.Name()).Peers().Add().SetName(otgPort2V4Peer)
	iDut2Bgp4Peer.SetPeerAddress(iDut2Ipv4.Gateway()).SetAsNumber(ateAS2).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut2Bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	t.Logf("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)

	return iDut1Bgp4Peer, iDut1Ipv4, config
}

// bgpClearConfig removes all BGP configuration from the DUT.
func bgpClearConfig(t *testing.T, dut *ondatra.DUTDevice) {
	resetBatch := &gnmi.SetBatch{}
	gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config())

	if deviations.NetworkInstanceTableDeletionRequired(dut) {
		tablePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableAny()
		for _, table := range gnmi.LookupAll[*oc.NetworkInstance_Table](t, dut, tablePath.Config()) {
			if val, ok := table.Val(); ok {
				if val.GetProtocol() == oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP {
					gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Table(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, val.GetAddressFamily()).Config())
				}
			}
		}
	}
	resetBatch.Set(t, dut)
}

// Verify BGP capabilities like route refresh as32 and mpbgp.
func verifyBGPCapabilities(t *testing.T, dut *ondatra.DUTDevice, nbrs []*bgpNeighbor) {
	t.Log("Verifying BGP capabilities")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, nbr := range nbrs {
		nbrPath := statePath.Neighbor(nbr.neighborip)

		capabilities := map[oc.E_BgpTypes_BGP_CAPABILITY]bool{
			oc.BgpTypes_BGP_CAPABILITY_ROUTE_REFRESH: false,
			oc.BgpTypes_BGP_CAPABILITY_ASN32:         false,
			oc.BgpTypes_BGP_CAPABILITY_MPBGP:         false,
		}
		for _, cap := range gnmi.Get(t, dut, nbrPath.SupportedCapabilities().State()) {
			capabilities[cap] = true
		}
		for cap, present := range capabilities {
			if !present {
				t.Errorf("Capability not reported: %v", cap)
			}
		}
	}
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

	bgpNeti1AsPath := bgpNeti1Bgp4PeerRoutes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SET)
	bgpNeti1AsPath.Segments().Add().SetAsNumbers(asSeg).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)

	t.Logf("Pushing config to OTG and starting protocols...")
	args.otg.PushConfig(t, args.otgConfig)
	time.Sleep(30 * time.Second)
	args.otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
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

// verifyOTGPrefixTelemetry is to Validate prefix received on OTG por2.
func verifyOTGPrefixTelemetry(t *testing.T, otg *otg.OTG, wantPrefixCount int, wantASSeg []uint32) {
	t.Helper()
	_, ok := gnmi.WatchAll(t, otg, gnmi.OTG().BgpPeer(atePort2.Name+".BGP4.peer").UnicastIpv4PrefixAny().State(),
		time.Minute, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			return v.IsPresent()
		}).Await(t)

	if ok {
		bgpPrefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(atePort2.Name+".BGP4.peer").UnicastIpv4PrefixAny().State())
		gotPrefixCount := len(bgpPrefixes)
		if gotPrefixCount != wantPrefixCount {
			t.Errorf("Received prefixes on otg are not as expected got prefixes %v, want prefixes %v", gotPrefixCount, wantPrefixCount)
		}
		if wantPrefixCount != 0 {
			for _, prefix := range bgpPrefixes {
				for _, gotASSeg := range prefix.AsPath {
					if ok := cmp.Diff(gotASSeg.AsNumbers, wantASSeg); ok != "" {
						t.Errorf("Prefix not found: gotAsSeg %v wantAsSeg %v for Prefix %v", gotASSeg, wantASSeg, prefix.GetAddress())
					}
				}
			}
		}
	}
}

func testSplitHorizonNoAllowOwnIn(t *testing.T, args *otgTestArgs) {
	t.Log("Baseline Test No allow-own-in")

	t.Log("Advertise a prefix from the ATE (e.g., 192.168.1.0/24) with an AS-path that includes AS 65501 (DUT's AS) in the middle (e.g., AS-path: 65502 65500 65501 65499")
	advBGPRouteFromOTG(t, args, []uint32{65502, 65500, dutAS, 65499})

	t.Log("Validate session state and capabilities received on DUT using telemetry.")
	verifyBgpTelemetry(t, args.dut, []*bgpNeighbor{nbr1, nbr2})
	verifyBGPCapabilities(t, args.dut, []*bgpNeighbor{nbr1, nbr2})

	t.Log("Verify that the ATE Port2 doesn't receive the route. due to the presence of its own AS in the path.")
	verifyPrefixesTelemetry(t, args.dut, nbr1.neighborip, 0, 0)
	verifyOTGPrefixTelemetry(t, args.otg, 0, []uint32{65502, 65500, dutAS, 65499})
}

func testSplitHorizonAllowOwnAs1(t *testing.T, args *otgTestArgs) {
	t.Log("Test allow-own-as 1, Enable allow-own-as 1 on the DUT.")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Update(t, args.dut, dutConfPath.Bgp().PeerGroup(peerGrpName1).AsPathOptions().AllowOwnAs().Config(), 1)

	t.Log("Re-advertise the prefix from the ATE with the same AS-path.")
	advBGPRouteFromOTG(t, args, []uint32{65502, 65500, dutAS, 65499})

	t.Log("Validate session state and capabilities received on DUT using telemetry.")
	verifyBgpTelemetry(t, args.dut, []*bgpNeighbor{nbr1, nbr2})
	verifyBGPCapabilities(t, args.dut, []*bgpNeighbor{nbr1, nbr2})

	t.Log("Verify that the DUT accepts the route.")
	verifyPrefixesTelemetry(t, args.dut, nbr1.neighborip, 1, 1)

	t.Log("Verify that the ATE Port2 receives the route.")
	verifyOTGPrefixTelemetry(t, args.otg, 1, []uint32{dutAS, 65502, 65500, dutAS, 65499})

}

func testSplitHorizonAllowOwnAs3(t *testing.T, args *otgTestArgs) {
	t.Log("Test allow-own-as 3, Enable allow-own-as 3 on the DUT.")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Update(t, args.dut, dutConfPath.Bgp().PeerGroup(peerGrpName1).AsPathOptions().AllowOwnAs().Config(), 3)

	t.Run("Re-advertise the prefix from the ATE with 1 Occurrence: 65502 65500 65501 65499", func(t *testing.T) {
		advBGPRouteFromOTG(t, args, []uint32{65502, 65500, dutAS, 65499})

		t.Log("Validate session state and capabilities received on DUT using telemetry.")
		verifyBgpTelemetry(t, args.dut, []*bgpNeighbor{nbr1, nbr2})
		verifyBGPCapabilities(t, args.dut, []*bgpNeighbor{nbr1, nbr2})

		t.Log("Verify that the DUT accepts the route.")
		verifyPrefixesTelemetry(t, args.dut, nbr1.neighborip, 1, 1)

		t.Log("Verify that the ATE Port2 receives the route.")
		verifyOTGPrefixTelemetry(t, args.otg, 1, []uint32{dutAS, 65502, 65500, dutAS, 65499})
	})

	t.Run("Re-advertise the prefix from the ATE with 3 Occurrences: 65502 65501 65501 65501 65499", func(t *testing.T) {
		advBGPRouteFromOTG(t, args, []uint32{65502, dutAS, dutAS, dutAS, 65499})

		t.Log("Validate session state and capabilities received on DUT using telemetry.")
		verifyBgpTelemetry(t, args.dut, []*bgpNeighbor{nbr1, nbr2})

		t.Log("Verify that the DUT accepts the route.")
		verifyPrefixesTelemetry(t, args.dut, nbr1.neighborip, 1, 1)

		t.Log("Verify that the ATE Port2 receives the route.")
		verifyOTGPrefixTelemetry(t, args.otg, 1, []uint32{65502, dutAS, dutAS, dutAS, 65499})
	})

	t.Run("Re-advertise the prefix from the ATE with 4 Occurrences: 65502 65501 65501 65501 65501 65499 (Should be rejected)", func(t *testing.T) {
		advBGPRouteFromOTG(t, args, []uint32{65502, dutAS, dutAS, dutAS, dutAS, 65499})

		t.Log("Validate session state and capabilities received on DUT using telemetry.")
		verifyBgpTelemetry(t, args.dut, []*bgpNeighbor{nbr1, nbr2})
		verifyBGPCapabilities(t, args.dut, []*bgpNeighbor{nbr1, nbr2})

		t.Log("Verify that the DUT accepts the route.")
		verifyPrefixesTelemetry(t, args.dut, nbr1.neighborip, 0, 0)

		t.Log("Verify that the ATE Port2 receives the route.")
		verifyOTGPrefixTelemetry(t, args.otg, 0, []uint32{65502, dutAS, dutAS, dutAS, dutAS, 65499})
	})
}

func testSplitHorizonAllowOwnAs4(t *testing.T, args *otgTestArgs) {
	t.Log("Test allow-own-as 4, Enable allow-own-as 4 on the DUT.")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(args.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Update(t, args.dut, dutConfPath.Bgp().PeerGroup(peerGrpName1).AsPathOptions().AllowOwnAs().Config(), 4)

	t.Run("Re-advertise the prefix from the ATE with 1 Occurrence: 65502 65500 65501 65499", func(t *testing.T) {
		advBGPRouteFromOTG(t, args, []uint32{65502, 65500, dutAS, 65499})

		t.Log("Validate session state and capabilities received on DUT using telemetry.")
		verifyBgpTelemetry(t, args.dut, []*bgpNeighbor{nbr1, nbr2})
		verifyBGPCapabilities(t, args.dut, []*bgpNeighbor{nbr1, nbr2})

		t.Log("Verify that the DUT accepts the route.")
		verifyPrefixesTelemetry(t, args.dut, nbr1.neighborip, 1, 1)

		t.Log("Verify that the ATE Port2 receives the route.")
		verifyOTGPrefixTelemetry(t, args.otg, 1, []uint32{dutAS, 65502, 65500, dutAS, 65499})
	})

	t.Run("Re-advertise the prefix from the ATE with 3 Occurrences: 65502 65501 65501 65501 65499", func(t *testing.T) {
		advBGPRouteFromOTG(t, args, []uint32{65502, dutAS, dutAS, dutAS, 65499})

		t.Log("Validate session state and capabilities received on DUT using telemetry.")
		verifyBgpTelemetry(t, args.dut, []*bgpNeighbor{nbr1, nbr2})

		t.Log("Verify that the DUT accepts the route.")
		verifyPrefixesTelemetry(t, args.dut, nbr1.neighborip, 1, 1)

		t.Log("Verify that the ATE Port2 receives the route.")
		verifyOTGPrefixTelemetry(t, args.otg, 1, []uint32{65502, dutAS, dutAS, dutAS, 65499})
	})

	t.Run("Re-advertise the prefix from the ATE with 4 Occurrences: 65502 65501 65501 65501 65501 65499 (Should be accepted)", func(t *testing.T) {
		advBGPRouteFromOTG(t, args, []uint32{65502, dutAS, dutAS, dutAS, dutAS, 65499})

		t.Log("Validate session state and capabilities received on DUT using telemetry.")
		verifyBgpTelemetry(t, args.dut, []*bgpNeighbor{nbr1, nbr2})
		verifyBGPCapabilities(t, args.dut, []*bgpNeighbor{nbr1, nbr2})

		t.Log("Verify that the DUT accepts the route.")
		verifyPrefixesTelemetry(t, args.dut, nbr1.neighborip, 1, 1)

		t.Log("Verify that the ATE Port2 receives the route.")
		verifyOTGPrefixTelemetry(t, args.otg, 1, []uint32{65502, dutAS, dutAS, dutAS, dutAS, 65499})
	})
}

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
	peerGrp    string
}

type otgTestArgs struct {
	dut           *ondatra.DUTDevice
	ate           *ondatra.ATEDevice
	otgBgpPeer    gosnappi.BgpV4Peer
	otgIPv4Device gosnappi.DeviceIpv4
	otgConfig     gosnappi.Config
	otg           *otg.OTG
}

// TestBGPOverrideASPathSplitHorizon validates BGP Override AS-path split-horizon.
func TestBGPOverrideASPathSplitHorizon(t *testing.T) {
	t.Logf("Start DUT config load.")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Run("Configure DUT interfaces", func(t *testing.T) {
		configureDUT(t, dut)
	})

	t.Run("Configure DEFAULT network instance", func(t *testing.T) {
		dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
		gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	})

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")

	t.Run("Configure BGP Neighbors", func(t *testing.T) {
		bgpClearConfig(t, dut)
		dutConf := bgpCreateNbr(t, dut, dutAS)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
	})

	otg := ate.OTG()
	var otgConfig gosnappi.Config
	var otgBgpPeer gosnappi.BgpV4Peer
	var otgIPv4Device gosnappi.DeviceIpv4
	otgBgpPeer, otgIPv4Device, otgConfig = configureOTG(t, otg)
	//t.Run("Configure OTG", func(t *testing.T) {
	//otgConfig = configureOTG(t, otg)
	//})
	t.Logf("otg config %v", otgConfig)

	args := &otgTestArgs{
		dut:           dut,
		ate:           ate,
		otgBgpPeer:    otgBgpPeer,
		otgIPv4Device: otgIPv4Device,
		otgConfig:     otgConfig,
		otg:           otg,
	}

	t.Run("Verify port status on DUT", func(t *testing.T) {
		verifyPortsUp(t, dut.Device)
	})

	t.Run("Verify BGP telemetry", func(t *testing.T) {
		verifyBgpTelemetry(t, dut, []*bgpNeighbor{nbr1, nbr2})
		verifyBGPCapabilities(t, dut, []*bgpNeighbor{nbr1, nbr2})
	})

	cases := []struct {
		desc     string
		funcName func()
		skipMsg  string
	}{{
		desc:     " Baseline Test No allow-own-in",
		funcName: func() { testSplitHorizonNoAllowOwnIn(t, args) },
	}, {
		desc:     " Test allow-own-as 1",
		funcName: func() { testSplitHorizonAllowOwnAs1(t, args) },
	}, {
		desc:     " Test allow-own-as 3",
		funcName: func() { testSplitHorizonAllowOwnAs3(t, args) },
	}, {
		desc:     " Test allow-own-as 4",
		funcName: func() { testSplitHorizonAllowOwnAs4(t, args) },
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			tc.funcName()
		})
	}
}
