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

package bgp_prefix_set_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	peerGrpName   = "BGP-PEER-GROUP"
	dutAS         = 65501
	ateAS         = 65502
	plenIPv4      = 30
	plenIPv6      = 126
	v4Prefixes    = true
	acceptPolicy  = "PERMIT-ALL"
	rejectPolicy  = "REJECT-ALL"
	bgpImportIPv4 = "IPv4-IMPORT"
	bgpImportIPv6 = "IPv6-IMPORT"
	bgpExportIPv4 = "IPv4-ExPORT"
	bgpExportIPv6 = "IPv6-ExPORT"
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

	ebgp1NbrV4 = &bgpNeighbor{
		nbrAddr: atePort1.IPv4,
		isV4:    true,
		afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
		as:      ateAS}
	ebgp1NbrV6 = &bgpNeighbor{
		nbrAddr: atePort1.IPv6,
		isV4:    false,
		afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
		as:      ateAS}
	ebgp2NbrV4 = &bgpNeighbor{
		nbrAddr: atePort2.IPv4,
		isV4:    true,
		afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST,
		as:      ateAS}
	ebgp2NbrV6 = &bgpNeighbor{
		nbrAddr: atePort2.IPv6,
		isV4:    false,
		afiSafi: oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST,
		as:      ateAS}
	ebgpNbrs = []*bgpNeighbor{ebgp1NbrV4, ebgp1NbrV6, ebgp2NbrV4, ebgp2NbrV6}

	route1 = &route{prefix: "10.23.15.1", maskLen: 32, isV4: true}
	route2 = &route{prefix: "10.23.15.2", maskLen: 16, isV4: true}
	route3 = &route{prefix: "10.23.15.60", maskLen: 26, isV4: true}
	route4 = &route{prefix: "10.23.15.70", maskLen: 26, isV4: true}
	route5 = &route{prefix: "20.23.15.2", maskLen: 16, isV4: true}

	route6  = &route{prefix: "2001:4860:f804::1", maskLen: 48, isV4: false}
	route7  = &route{prefix: "2001:4860:f804::2", maskLen: 128, isV4: false}
	route8  = &route{prefix: "2001:4860:f804:1111::1", maskLen: 64, isV4: false}
	route9  = &route{prefix: "2001:4860:f804::10", maskLen: 70, isV4: false}
	route10 = &route{prefix: "2001:5555:f804::1", maskLen: 48, isV4: false}

	routes = []*route{route1, route2, route3, route4, route5, route6, route7, route8, route9, route10}

	prefixSet1V4 = &prefixSetPolicy{
		name:         "IPv4-prefix-set-1",
		ipPrefix:     "10.23.15.0/26",
		maskLenRange: "exact",
		statement:    "10",
		actionAccept: false,
		isV4:         true}
	prefixSet2V4 = &prefixSetPolicy{
		name:         "IPv4-prefix-set-2",
		ipPrefix:     "10.23.0.0/16",
		maskLenRange: "16..32",
		statement:    "20",
		actionAccept: true,
		isV4:         true}
	prefixSet1V6 = &prefixSetPolicy{
		name:         "IPv6-prefix-set-1",
		ipPrefix:     "2001:4860:f804::/48",
		maskLenRange: "exact",
		statement:    "10",
		actionAccept: true,
		isV4:         false}
	prefixSet2V6 = &prefixSetPolicy{
		name:         "IPv6-prefix-set-2",
		ipPrefix:     "::/0",
		maskLenRange: "65..128",
		statement:    "20",
		actionAccept: false,
		isV4:         false}
)

type route struct {
	prefix  string
	maskLen uint32
	isV4    bool
}

type prefixSetPolicy struct {
	name         string
	ipPrefix     string
	maskLenRange string
	statement    string
	actionAccept bool
	isV4         bool
}

type bgpNeighbor struct {
	as      uint32
	nbrAddr string
	isV4    bool
	afiSafi oc.E_BgpTypes_AFI_SAFI_TYPE
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutPort1.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutPort2.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dut.Port(t, "port1").Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, dut.Port(t, "port2").Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func configurePrefixSet(t *testing.T, dut *ondatra.DUTDevice, prefixSet []*prefixSetPolicy) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	for _, ps := range prefixSet {
		pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(ps.name)
		pset.GetOrCreatePrefix(ps.ipPrefix, ps.maskLenRange)
		if !deviations.SkipPrefixSetMode(dut) {
			if ps.isV4 {
				pset.SetMode(oc.PrefixSet_Mode_IPV4)
			} else {
				pset.SetMode(oc.PrefixSet_Mode_IPV6)
			}
		}
		gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().DefinedSets().PrefixSet(ps.name).Config(), pset)
	}
}

func applyPrefixSetPolicy(t *testing.T, dut *ondatra.DUTDevice, prefixSet []*prefixSetPolicy, policyName string, bgpNbr bgpNeighbor, importPolicy bool) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()

	// Associate prefix-set with routing-policy
	pdef := rp.GetOrCreatePolicyDefinition(policyName)
	for _, pSet := range prefixSet {
		stmt, err := pdef.AppendNewStatement(pSet.statement)
		if err != nil {
			t.Fatal(err)
		}
		ps := stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet()
		ps.SetPrefixSet(pSet.name)
		if !deviations.SkipSetRpMatchSetOptions(dut) {
			ps.SetMatchSetOptions(oc.E_RoutingPolicy_MatchSetOptionsRestrictedType(oc.RoutingPolicy_MatchSetOptionsType_ANY))
		}
		stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
		if !pSet.actionAccept {
			stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
		}
	}

	batchConfig := &gnmi.SetBatch{}
	gnmi.BatchUpdate(batchConfig, gnmi.OC().RoutingPolicy().Config(), rp)

	// Apply routing-policy with BGP neighbor
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	if importPolicy {
		gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(bgpNbr.nbrAddr).AfiSafi(bgpNbr.afiSafi).ApplyPolicy().ImportPolicy().Config(), []string{policyName})
	} else {
		gnmi.BatchReplace(batchConfig, bgpPath.Neighbor(bgpNbr.nbrAddr).AfiSafi(bgpNbr.afiSafi).ApplyPolicy().ExportPolicy().Config(), []string{policyName})
	}
	batchConfig.Set(t, dut)
}

func bgpCreateNbr(localAs, peerAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {

	// Configure BGP on DUT
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutPort1.IPv4)
	global.As = ygot.Uint32(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup(peerGrpName)
	pg.PeerAs = ygot.Uint32(ateAS)
	pg.PeerGroupName = ygot.String(peerGrpName)

	for _, nbr := range ebgpNbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.nbrAddr)
		bgpNbr.PeerGroup = ygot.String(peerGrpName)
		bgpNbr.PeerAs = ygot.Uint32(nbr.as)
		bgpNbr.Enabled = ygot.Bool(true)

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

func verifyBgpState(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	t.Logf("Waiting for BGP neighbor to establish...")
	for _, nbr := range ebgpNbrs {
		nbrPath := bgpPath.Neighbor(nbr.nbrAddr)
		var status *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]
		status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			state, ok := val.Val()
			return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatal("No BGP neighbor formed")
		}
		state, _ := status.Val()
		if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
			t.Errorf("BGP peer %s status got %d, want %d", nbr.nbrAddr, state, want)
		}
	}
}

func configureOTG(t *testing.T, otg *otg.OTG) {
	t.Helper()
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	// Port1 Configuration. Sets the ATE port
	iDut1Dev := config.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	// Port2 Configuration.
	iDut2Dev := config.Devices().Add().SetName(atePort2.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	iDut2Eth.Connection().SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	iDut2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	iDut2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	// eBGP v4 seesion on Port1.
	iDut1Bgp := iDut1Dev.Bgp().SetRouterId(iDut1Ipv4.Address())
	iDut1Bgp4Peer := iDut1Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(atePort1.Name + ".BGP4.peer")
	iDut1Bgp4Peer.SetPeerAddress(iDut1Ipv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut1Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
	// eBGP v6 seesion on Port1.
	iDut1Bgp6Peer := iDut1Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut1Ipv6.Name()).Peers().Add().SetName(atePort1.Name + ".BGP6.peer")
	iDut1Bgp6Peer.SetPeerAddress(iDut1Ipv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	iDut1Bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

	// eBGP v4 seesion on Port2.
	iDut2Bgp := iDut2Dev.Bgp().SetRouterId(iDut2Ipv4.Address())
	iDut2Bgp4Peer := iDut2Bgp.Ipv4Interfaces().Add().SetIpv4Name(iDut2Ipv4.Name()).Peers().Add().SetName(atePort2.Name + ".BGP4.peer")
	iDut2Bgp4Peer.SetPeerAddress(iDut2Ipv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	iDut2Bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
	// eBGP v6 seesion on Port2.
	iDut2Bgp6Peer := iDut2Bgp.Ipv6Interfaces().Add().SetIpv6Name(iDut2Ipv6.Name()).Peers().Add().SetName(atePort2.Name + ".BGP6.peer")
	iDut2Bgp6Peer.SetPeerAddress(iDut2Ipv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	iDut2Bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

	// eBGP V4 routes from Port1.
	bgpNeti1Bgp4PeerRoutes := iDut1Bgp4Peer.V4Routes().Add().SetName(atePort1.Name + ".BGP4.Route")
	bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(iDut1Ipv4.Address()).
		SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
		SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)

	// eBGP V6 routes from Port1.
	bgpNeti1Bgp6PeerRoutes := iDut1Bgp6Peer.V6Routes().Add().SetName(atePort1.Name + ".BGP6.Route")
	bgpNeti1Bgp6PeerRoutes.SetNextHopIpv6Address(iDut1Ipv6.Address()).
		SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
		SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)

	for _, sendRoute := range routes {
		if sendRoute.isV4 {
			bgpNeti1Bgp4PeerRoutes.Addresses().Add().
				SetAddress(sendRoute.prefix).SetPrefix(sendRoute.maskLen)
		}
		if !sendRoute.isV4 {
			bgpNeti1Bgp6PeerRoutes.Addresses().Add().
				SetAddress(sendRoute.prefix).SetPrefix(sendRoute.maskLen)
		}
	}

	otg.PushConfig(t, config)
	otg.StartProtocols(t)
}

func validatePrefixCount(t *testing.T, dut *ondatra.DUTDevice, nbr bgpNeighbor, wantInstalled, wantRx, wantSent uint32) {
	t.Helper()

	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	t.Logf("Validating prefix count for peer %v", nbr.nbrAddr)
	prefixPath := statePath.Neighbor(nbr.nbrAddr).AfiSafi(nbr.afiSafi).Prefixes()

	// Waiting for Installed count to get updated after session comes up or policy is applied
	gotInstalled, ok := gnmi.Watch(t, dut, prefixPath.Installed().State(), 20*time.Second, func(val *ygnmi.Value[uint32]) bool { // increased wait time to 20s from 10s
		gotInstalled, _ := val.Val()
		t.Logf("Prefix that are installed %v and want %v", gotInstalled, wantInstalled)
		return gotInstalled == wantInstalled
	}).Await(t)
	if !ok {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}

	if !deviations.MissingPrePolicyReceivedRoutes(dut) {
		// Waiting for Received count to get updated after session comes up or policy is applied
		gotRx, ok := gnmi.Watch(t, dut, prefixPath.ReceivedPrePolicy().State(), 10*time.Second, func(val *ygnmi.Value[uint32]) bool {
			gotRx, _ := val.Val()
			t.Logf("Prefix that are received %v and want %v", gotRx, wantRx)
			return gotRx == wantRx
		}).Await(t)
		if !ok {
			t.Errorf("Received prefixes mismatch: got %v, want %v", gotRx, wantRx)
		}
	}

	// Waiting for Sent count to get updated after session comes up or policy is applied
	gotSent, ok := gnmi.Watch(t, dut, prefixPath.Sent().State(), 10*time.Second, func(val *ygnmi.Value[uint32]) bool {
		t.Logf("Prefix that are sent %v", prefixPath.Sent().State())
		gotSent, _ := val.Val()
		t.Logf("Prefix that are sent %v and want %v", gotSent, wantSent)
		return gotSent == wantSent
	}).Await(t)
	if !ok {
		t.Errorf("Sent prefixes mismatch: got %v, want %v", gotSent, wantSent)
	}
}

// testPrefixSet is to validate prefix-set policies
func testPrefixSet(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	importPolicy := true

	// Configuring all 4 reqruired prefix-sets
	t.Run("Configure prefix-set", func(t *testing.T) {
		configurePrefixSet(t, dut, []*prefixSetPolicy{prefixSet1V4, prefixSet2V4, prefixSet1V6, prefixSet2V6})
	})

	// Associating prefix-set with the required routing-policy and applying to BGP neighbors on ATE-port-1
	t.Run("Validate acceptance based on prefix-set policy - import policy on neighbor", func(t *testing.T) {
		applyPrefixSetPolicy(t, dut, []*prefixSetPolicy{prefixSet1V4, prefixSet2V4}, bgpImportIPv4, *ebgp1NbrV4, importPolicy)
		applyPrefixSetPolicy(t, dut, []*prefixSetPolicy{prefixSet1V6, prefixSet2V6}, bgpImportIPv6, *ebgp1NbrV6, importPolicy)
		if !deviations.DefaultImportExportPolicy(dut) {
			t.Logf("Validate for neighbour %v", ebgp1NbrV4)
			validatePrefixCount(t, dut, *ebgp1NbrV4, 3, 5, 0)
			validatePrefixCount(t, dut, *ebgp1NbrV6, 1, 5, 0)
			validatePrefixCount(t, dut, *ebgp2NbrV4, 0, 0, 3)
			validatePrefixCount(t, dut, *ebgp2NbrV6, 0, 0, 1)
		} else {
			t.Logf("Validate for neighbour %v", ebgp1NbrV4)
			validatePrefixCount(t, dut, *ebgp1NbrV4, 3, 5, 0)
			// only route6 is expected to accepted based on prefix-set
			validatePrefixCount(t, dut, *ebgp1NbrV6, 1, 5, 0)
			validatePrefixCount(t, dut, *ebgp2NbrV4, 0, 0, 0)
			validatePrefixCount(t, dut, *ebgp2NbrV6, 0, 0, 0)
		}
	})

	// Associating prefix-set with the required routing-policy and applying to BGP neighbors on ATE-port-2
	t.Run("Validate advertise based on prefix-set policy - export policy on neighbor", func(t *testing.T) {
		applyPrefixSetPolicy(t, dut, []*prefixSetPolicy{prefixSet2V4}, bgpExportIPv4, *ebgp2NbrV4, !importPolicy)
		applyPrefixSetPolicy(t, dut, []*prefixSetPolicy{prefixSet1V6}, bgpExportIPv6, *ebgp2NbrV6, !importPolicy)

		// route1, route2, route4 expected to be advertised based on prefix-set
		validatePrefixCount(t, dut, *ebgp2NbrV4, 0, 0, 3)
		// only route6 is expected to be advertised based on prefix-set
		validatePrefixCount(t, dut, *ebgp2NbrV6, 0, 0, 1)
	})
}

// TestBGPPrefixSet is to test prefix-set at the BGP neighbor levels.
func TestBGPPrefixSet(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Bring up 4 eBGP neighbors between DUT and ATE
	t.Run("Establish BGP sessions", func(t *testing.T) {
		configureDUT(t, dut)

		dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
		gnmi.Delete(t, dut, dutConfPath.Config())
		dutConf := bgpCreateNbr(dutAS, ateAS, dut)
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)

		otg := ate.OTG()
		configureOTG(t, otg)
		verifyBgpState(t, dut)
	})

	if !deviations.DefaultImportExportPolicy(dut) {
		t.Run("Validate initial prefix count", func(t *testing.T) {
			validatePrefixCount(t, dut, *ebgp1NbrV4, 5, 5, 0)
			validatePrefixCount(t, dut, *ebgp1NbrV6, 5, 5, 0)
			validatePrefixCount(t, dut, *ebgp2NbrV4, 0, 0, 5)
			validatePrefixCount(t, dut, *ebgp2NbrV6, 0, 0, 5)
		})
	} else {
		t.Run("Validate initial prefix count", func(t *testing.T) {
			validatePrefixCount(t, dut, *ebgp1NbrV4, 0, 1, 0)
			validatePrefixCount(t, dut, *ebgp1NbrV6, 0, 5, 0)
			validatePrefixCount(t, dut, *ebgp2NbrV4, 0, 0, 0)
			validatePrefixCount(t, dut, *ebgp2NbrV6, 0, 0, 0)
		})

	}

	testPrefixSet(t, dut)
}
