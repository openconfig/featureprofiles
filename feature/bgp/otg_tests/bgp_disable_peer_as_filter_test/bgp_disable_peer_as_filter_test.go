// Copyright 2025 Google LLC
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

// Package bgpdisablepeerasfiltertest implements RT-1.71: BGP Disable Peer AS Filter
// Verifies BGP disable-peer-as-filter functionality for both IPv4 and IPv6 Unicast sessions.
package bgp_disable_peer_as_filter_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	// BGP AS numbers
	dutAS  = 64498
	ateAS1 = 64496
	ateAS2 = 64497
	ateAS3 = 64512 // Private AS for RT-1.71.4 test
	ateAS4 = 64499 // Transit AS for RT-1.71.3

	// Advertised routes
	advertisedRoutesv4Net    = "203.0.113.1"
	advertisedRoutesv4Prefix = 32
	advertisedRoutesv6Net    = "2001:db8:64:64::1"
	advertisedRoutesv6Prefix = 64
	routeCount               = 1

	// BGP configuration constants
	peerGrpName1 = "BGP-PEER-GROUP1"
	peerGrpName2 = "BGP-PEER-GROUP2"
	plenIPv4     = 30
	plenIPv6     = 126
)

// Port attributes for ATE and DUT
var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE Port2",
		IPv4:    "198.51.100.1",
		IPv6:    "2001:db8::5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "198.51.100.2",
		IPv6:    "2001:db8::6",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// configureDUT sets up basic interface configurations on the DUT
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutPort1.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutPort2.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	if deviations.BgpRibStreamingConfigRequired(dut) {
		cfgplugins.DeviationBgpRibStreamingConfigRequired(t, dut)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, i2.GetName(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureBGP configures BGP neighbors
func configureBGP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.SetRouterId(dutPort2.IPv4)
	global.SetAs(dutAS)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(true)

	// Peer Group 1 for ATE Port 1
	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1)
	pg1.SetPeerAs(ateAS1)
	pg1.SetPeerGroupName(peerGrpName1)
	pg1af4 := pg1.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	pg1af4.SetEnabled(true)
	pg1af6 := pg1.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	pg1af6.SetEnabled(true)

	// Peer Group 2 for ATE Port 2
	pg2 := bgp.GetOrCreatePeerGroup(peerGrpName2)
	pg2.SetPeerAs(ateAS2)
	pg2.SetPeerGroupName(peerGrpName2)
	pg2af4 := pg2.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	pg2af4.SetEnabled(true)
	pg2af6 := pg2.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	pg2af6.SetEnabled(true)

	// Configure neighbor for ATE Port 1
	nbr1v4 := bgp.GetOrCreateNeighbor(atePort1.IPv4)
	nbr1v4.SetPeerGroup(peerGrpName1)
	nbr1v4.SetPeerAs(ateAS1)
	nbr1v4.SetEnabled(true)
	af1v4 := nbr1v4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	af1v4.SetEnabled(true)
	af1v6 := nbr1v4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	af1v6.SetEnabled(true)

	nbr1v6 := bgp.GetOrCreateNeighbor(atePort1.IPv6)
	nbr1v6.SetPeerGroup(peerGrpName1)
	nbr1v6.SetPeerAs(ateAS1)
	nbr1v6.SetEnabled(true)
	af1v6nbr := nbr1v6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	af1v6nbr.SetEnabled(true)

	// Configure neighbor for ATE Port 2
	nbr2v4 := bgp.GetOrCreateNeighbor(atePort2.IPv4)
	nbr2v4.SetPeerGroup(peerGrpName2)
	nbr2v4.SetPeerAs(ateAS2)
	nbr2v4.SetEnabled(true)
	af2v4 := nbr2v4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	af2v4.SetEnabled(true)
	af2v6 := nbr2v4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	af2v6.SetEnabled(true)

	nbr2v6 := bgp.GetOrCreateNeighbor(atePort2.IPv6)
	nbr2v6.SetPeerGroup(peerGrpName2)
	nbr2v6.SetPeerAs(ateAS2)
	nbr2v6.SetEnabled(true)
	af2v6nbr := nbr2v6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	af2v6nbr.SetEnabled(true)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Replace(t, dut, dutConfPath.Config(), niProto)
}

// verifyBGPTelemetry checks that the DUT has an established BGP session
func verifyBGPTelemetry(t *testing.T, dut *ondatra.DUTDevice, nbrs []string) error {
	t.Helper()
	t.Logf("Verifying BGP state for neighbors: %v", nbrs)
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	for _, nbr := range nbrs {
		nbrPath := statePath.Neighbor(nbr)
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			return fmt.Errorf("bgp session not established for neighbor %s", nbr)
		}
	}
	return nil
}

// configureOTG sets up ATE interfaces and BGP protocols
func configureOTG(t *testing.T, otg *otg.OTG, asSeg []uint32) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	// ATE Port 1 configuration (AS 64496)
	atePort1Dev := config.Devices().Add().SetName(atePort1.Name)
	atePort1Eth := atePort1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	atePort1Eth.Connection().SetPortName(port1.Name())
	atePort1IPv4 := atePort1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	atePort1IPv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	atePort1IPv6 := atePort1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	atePort1IPv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	// ATE Port 2 configuration (AS 64497)
	atePort2Dev := config.Devices().Add().SetName(atePort2.Name)
	atePort2Eth := atePort2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	atePort2Eth.Connection().SetPortName(port2.Name())
	atePort2IPv4 := atePort2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	atePort2IPv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	atePort2IPv6 := atePort2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	atePort2IPv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	// BGP on ATE Port 1
	atePort1Bgp := atePort1Dev.Bgp().SetRouterId(atePort1IPv4.Address())
	atePort1Bgp4Peer := atePort1Bgp.Ipv4Interfaces().Add().SetIpv4Name(atePort1IPv4.Name()).Peers().Add().SetName(atePort1.Name + ".BGP4.peer")
	atePort1Bgp4Peer.SetPeerAddress(dutPort1.IPv4).SetAsNumber(ateAS1).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	atePort1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	atePort1Bgp6Peer := atePort1Bgp.Ipv6Interfaces().Add().SetIpv6Name(atePort1IPv6.Name()).Peers().Add().SetName(atePort1.Name + ".BGP6.peer")
	atePort1Bgp6Peer.SetPeerAddress(dutPort1.IPv6).SetAsNumber(ateAS1).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	atePort1Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// BGP on ATE Port 2
	atePort2Bgp := atePort2Dev.Bgp().SetRouterId(atePort2IPv4.Address())
	atePort2Bgp4Peer := atePort2Bgp.Ipv4Interfaces().Add().SetIpv4Name(atePort2IPv4.Name()).Peers().Add().SetName(atePort2.Name + ".BGP4.peer")
	atePort2Bgp4Peer.SetPeerAddress(dutPort2.IPv4).SetAsNumber(ateAS2).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	atePort2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	atePort2Bgp6Peer := atePort2Bgp.Ipv6Interfaces().Add().SetIpv6Name(atePort2IPv6.Name()).Peers().Add().SetName(atePort2.Name + ".BGP6.peer")
	atePort2Bgp6Peer.SetPeerAddress(dutPort2.IPv6).SetAsNumber(ateAS2).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	atePort2Bgp6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// Configure IPv4 routes on ATE Port 1 with specified AS path
	atePort1Bgp4Routes := atePort1Bgp4Peer.V4Routes().Add().SetName(atePort1.Name + ".BGP4.Route")
	atePort1Bgp4Routes.SetNextHopIpv4Address(atePort1IPv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
	atePort1Bgp4Routes.Addresses().Add().
		SetAddress(advertisedRoutesv4Net).
		SetPrefix(uint32(advertisedRoutesv4Prefix)).
		SetCount(uint32(routeCount))

	BGP4AsPath := atePort1Bgp4Routes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SEQ)
	BGP4AsPath.Segments().Add().SetAsNumbers(asSeg).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)

	// Configure IPv6 routes on ATE Port 1 with specified AS path
	atePort1Bgp6Routes := atePort1Bgp6Peer.V6Routes().Add().SetName(atePort1.Name + ".BGP6.Route")
	atePort1Bgp6Routes.SetNextHopIpv6Address(atePort1IPv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
	atePort1Bgp6Routes.Addresses().Add().
		SetAddress(advertisedRoutesv6Net).
		SetPrefix(uint32(advertisedRoutesv6Prefix)).
		SetCount(uint32(routeCount))

	BGP6AsPath := atePort1Bgp6Routes.AsPath().SetAsSetMode(gosnappi.BgpAsPathAsSetMode.INCLUDE_AS_SEQ)
	BGP6AsPath.Segments().Add().SetAsNumbers(asSeg).SetType(gosnappi.BgpAsPathSegmentType.AS_SEQ)

	t.Log("Pushing config to OTG and starting protocols...")
	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	return config
}

// verifyOTGBGPTelemetry verifies BGP session state on OTG
func verifyOTGBGPTelemetry(t *testing.T, otg *otg.OTG, c gosnappi.Config, state string) error {
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
					return fmt.Errorf("bgp session not established for peer %s", configPeer.Name())
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
					return fmt.Errorf("bgp session not established for peer %s", configPeer.Name())
				}
			}
		}
	}
	return nil
}

// verifyReceivedRoutes checks if routes were received on the ATE Port 2
func verifyReceivedRoutes(t *testing.T, otg *otg.OTG, peerName string, disablePeerASFilter bool) error {
	t.Helper()

	// Check IPv4 routes
	ipv4PeerName := peerName + ".BGP4.peer"
	_, ok := gnmi.WatchAll(t, otg, gnmi.OTG().BgpPeer(ipv4PeerName).UnicastIpv4PrefixAny().State(),
		time.Minute, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)

	var gotPrefixCount int
	if ok {
		bgpPrefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(ipv4PeerName).UnicastIpv4PrefixAny().State())
		gotPrefixCount = len(bgpPrefixes)
	}

	if disablePeerASFilter {
		if gotPrefixCount == 0 {
			return fmt.Errorf("expected to receive IPv4 routes on %s but got 0", ipv4PeerName)
		}
		t.Logf("Successfully received %d IPv4 routes on %s", gotPrefixCount, ipv4PeerName)
	} else {
		if gotPrefixCount != 0 {
			return fmt.Errorf("expected to NOT receive IPv4 routes on %s but got %d", ipv4PeerName, gotPrefixCount)
		}
		t.Logf("Correctly did not receive IPv4 routes on %s", ipv4PeerName)
	}

	// Check IPv6 routes
	ipv6PeerName := peerName + ".BGP6.peer"
	_, ok6 := gnmi.WatchAll(t, otg, gnmi.OTG().BgpPeer(ipv6PeerName).UnicastIpv6PrefixAny().State(),
		time.Minute, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)

	var gotV6PrefixCount int
	if ok6 {
		bgpV6Prefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(ipv6PeerName).UnicastIpv6PrefixAny().State())
		gotV6PrefixCount = len(bgpV6Prefixes)
	}

	if disablePeerASFilter {
		if gotV6PrefixCount == 0 {
			return fmt.Errorf("expected to receive IPv6 routes on %s but got 0", ipv6PeerName)
		}
		t.Logf("Successfully received %d IPv6 routes on %s", gotV6PrefixCount, ipv6PeerName)
	} else {
		if gotV6PrefixCount != 0 {
			return fmt.Errorf("expected to NOT receive IPv6 routes on %s but got %d", ipv6PeerName, gotV6PrefixCount)
		}
		t.Logf("Correctly did not receive IPv6 routes on %s", ipv6PeerName)
	}
	return nil
}

// verifyReceivedRoutesWithAsPath checks routes and validates the AS path
func verifyReceivedRoutesWithAsPath(t *testing.T, otg *otg.OTG, peerName string, wantASSeg []uint32) error {
	t.Helper()

	ipv4PeerName := peerName + ".BGP4.peer"
	_, ok := gnmi.WatchAll(t, otg, gnmi.OTG().BgpPeer(ipv4PeerName).UnicastIpv4PrefixAny().State(),
		time.Minute, func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)

	if !ok {
		return fmt.Errorf("expected to receive IPv4 routes on %s but got 0", ipv4PeerName)
	}

	bgpPrefixes := gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(ipv4PeerName).UnicastIpv4PrefixAny().State())
	gotPrefixCount := len(bgpPrefixes)

	if gotPrefixCount == 0 {
		return fmt.Errorf("expected to receive IPv4 routes on %s but got 0", ipv4PeerName)
	}

	for _, prefix := range bgpPrefixes {
		prefixAsSegments := []uint32{}
		for _, gotASSeg := range prefix.AsPath {
			prefixAsSegments = append(prefixAsSegments, gotASSeg.AsNumbers...)
		}
		if diff := cmp.Diff(prefixAsSegments, wantASSeg); diff != "" {
			return fmt.Errorf("as path mismatch for prefix %v: got %v, want %v", prefix.Address, prefixAsSegments, wantASSeg)
		}
	}
	return nil
}

// testCase represents a single test case for the disable-peer-as-filter functionality.
type testCase struct {
	name                string
	setupFunc           func(*testing.T, *ondatra.DUTDevice, *gnmi.SetBatch, bool) *gnmi.SetBatch
	asSeg               []uint32
	disablePeerASFilter bool
	expectedASPath      []uint32
	verifyASPath        bool
	peerGroup           bool
}

func TestDisablePeerAsFilterPerBGPNeighbor(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otgClient := ate.OTG()

	t.Run("Setting up DUT and BGP on DUT", func(t *testing.T) {
		configureDUT(t, dut)
		configureBGP(t, dut)
	})

	t.Run("Configure DEFAULT network instance", func(t *testing.T) {
		fptest.ConfigureDefaultNetworkInstance(t, dut)
	})

	tests := []testCase{
		{
			name:                "RT-1.71.1: Baseline Test - Default filtering (should NOT accept routes with peer AS in path)",
			setupFunc:           cfgplugins.ConfigureBGPEnablePeerAsFilterPeer,
			asSeg:               []uint32{ateAS2, ateAS4},
			disablePeerASFilter: false,
			verifyASPath:        false,
			peerGroup:           false,
		},
		{
			name:                "RT-1.71.2: Enable disable-peer-as-filter at peer group level",
			setupFunc:           cfgplugins.ConfigureBGPDisablePeerAsFilter,
			asSeg:               []uint32{ateAS2, ateAS4},
			disablePeerASFilter: true,
			verifyASPath:        false,
			peerGroup:           false,
		},
		{
			name:                "RT-1.71.3: Test Originating Peer AS",
			setupFunc:           cfgplugins.ConfigureBGPDisablePeerAsFilter,
			asSeg:               []uint32{ateAS2, ateAS4},
			disablePeerASFilter: true,
			expectedASPath:      []uint32{dutAS, ateAS1, ateAS2, ateAS4},
			verifyASPath:        true,
			peerGroup:           false,
		},
		{
			name:                "RT-1.71.4: Private AS Number Scenario",
			setupFunc:           cfgplugins.ConfigureBGPDisablePeerAsFilter,
			asSeg:               []uint32{ateAS3, ateAS4},
			disablePeerASFilter: true,
			expectedASPath:      []uint32{dutAS, ateAS1, ateAS3, ateAS4},
			verifyASPath:        true,
			peerGroup:           false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Configure BGP with appropriate settings
			b := &gnmi.SetBatch{}
			tc.setupFunc(t, dut, b, tc.peerGroup)
			b.Set(t, dut)

			// Configure ATE and establish BGP sessions
			config := configureOTG(t, otgClient, tc.asSeg)

			// Verify BGP sessions are established
			if err := verifyBGPTelemetry(t, dut, []string{atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6}); err != nil {
				t.Error(err)
			}
			if err := verifyOTGBGPTelemetry(t, otgClient, config, "ESTABLISHED"); err != nil {
				t.Error(err)
			}

			// Verify routes received according to test case expectations
			if err := verifyReceivedRoutes(t, otgClient, atePort2.Name, tc.disablePeerASFilter); err != nil {
				t.Error(err)
			}

			// Verify AS path if required by test case
			if tc.verifyASPath {
				if err := verifyReceivedRoutesWithAsPath(t, otgClient, atePort2.Name, tc.expectedASPath); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

func TestDisablePeerAsFilterPerBGPPeerGroup(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otgClient := ate.OTG()

	t.Run("Setting up DUT and BGP", func(t *testing.T) {
		configureDUT(t, dut)
		configureBGP(t, dut)
	})

	t.Run("Configure DEFAULT network instance", func(t *testing.T) {
		fptest.ConfigureDefaultNetworkInstance(t, dut)
	})

	tests := []testCase{
		{
			name:                "RT-1.71.5.1: Baseline Test - Default filtering (should NOT accept routes with peer AS in path)",
			setupFunc:           cfgplugins.ConfigureBGPEnablePeerAsFilterPeer,
			asSeg:               []uint32{ateAS2, ateAS4},
			disablePeerASFilter: false,
			verifyASPath:        false,
			peerGroup:           true,
		},
		{
			name:                "RT-1.71.5.2: Enable disable-peer-as-filter at peer group level",
			setupFunc:           cfgplugins.ConfigureBGPDisablePeerAsFilter,
			asSeg:               []uint32{ateAS2, ateAS4},
			disablePeerASFilter: true,
			verifyASPath:        false,
			peerGroup:           true,
		},
		{
			name:                "RT-1.71.5.3: Test Originating Peer AS",
			setupFunc:           cfgplugins.ConfigureBGPDisablePeerAsFilter,
			asSeg:               []uint32{ateAS2, ateAS4},
			disablePeerASFilter: true,
			expectedASPath:      []uint32{dutAS, ateAS1, ateAS2, ateAS4},
			verifyASPath:        true,
			peerGroup:           true,
		},
		{
			name:                "RT-1.71.5.4: Private AS Number Scenario",
			setupFunc:           cfgplugins.ConfigureBGPDisablePeerAsFilter,
			asSeg:               []uint32{ateAS3, ateAS4},
			disablePeerASFilter: true,
			expectedASPath:      []uint32{dutAS, ateAS1, ateAS3, ateAS4},
			verifyASPath:        true,
			peerGroup:           true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Configure BGP with appropriate settings
			b := &gnmi.SetBatch{}
			tc.setupFunc(t, dut, b, tc.peerGroup)
			b.Set(t, dut)

			// Configure ATE and establish BGP sessions
			config := configureOTG(t, otgClient, tc.asSeg)

			// Verify BGP sessions are established
			if err := verifyBGPTelemetry(t, dut, []string{atePort1.IPv4, atePort1.IPv6, atePort2.IPv4, atePort2.IPv6}); err != nil {
				t.Error(err)
			}
			if err := verifyOTGBGPTelemetry(t, otgClient, config, "ESTABLISHED"); err != nil {
				t.Error(err)
			}

			// Verify routes received according to test case expectations
			if err := verifyReceivedRoutes(t, otgClient, atePort2.Name, tc.disablePeerASFilter); err != nil {
				t.Error(err)
			}

			// Verify AS path if required by test case
			if tc.verifyASPath {
				if err := verifyReceivedRoutesWithAsPath(t, otgClient, atePort2.Name, tc.expectedASPath); err != nil {
					t.Error(err)
				}
			}
		})
	}
}
