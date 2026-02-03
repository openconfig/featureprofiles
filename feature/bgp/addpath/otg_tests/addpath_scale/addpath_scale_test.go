// Copyright 2025 Google LLC
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

package addpath_scale_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	plenIPv4              = uint8(30)
	plenIPv6              = uint8(126)
	v4VlanPlen            = uint8(24)
	v6VlanPlen            = uint8(64)
	dutAS                 = 64500
	ateAS2                = 64201
	ateAS3                = 64301
	ateAS4                = 64401
	peerv42GrpName        = "BGP-PEER-GROUP-V42"
	peerv63GrpName        = "BGP-PEER-GROUP-V63"
	peerv44GrpName        = "BGP-PEER-GROUP-V44"
	peerv64GrpName        = "BGP-PEER-GROUP-V64"
	ipv4Prefixes          = 14794
	ipv6Prefixes          = 2400
	lossTolerance         = float64(1)
	communitySetNameRegex = "any_my_regex_comms"
)

var (
	dutPort1 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port1",
			IPv4:    "192.0.2.1",
			IPv4Len: plenIPv4,
			IPv6:    "2001:0db8::192:0:2:1",
			IPv6Len: plenIPv6,
		},
		numSubIntf: 1,
	}
	dutPort2 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port2",
			IPv4:    "192.0.2.5",
			IPv4Len: plenIPv4,
			IPv6:    "2001:0db8::192:0:2:5",
			IPv6Len: plenIPv6,
		},
		numSubIntf: 100,
		ip4:        dutPort2IPv4,
	}
	dutPort3 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port3",
			IPv4:    "192.0.2.9",
			IPv4Len: plenIPv4,
			IPv6:    "2001:0db8::192:0:2:9",
			IPv6Len: plenIPv6,
		},
		numSubIntf: 50,
		ip6:        dutPort3IPv6,
	}
	dutPort4 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port4",
			IPv4:    "200.0.0.1", // 192.0.2.13
			IPv4Len: 24,
			IPv6:    "1000::200:0:0:1", // 2001:0db8::192:0:2:d
			IPv6Len: 126,
		},
		ip4: func(_ uint8) string {
			return "200.0.0.1"
		},
		ip6: func(_ uint8) string {
			return "1000::200:0:0:1"
		},
		numSubIntf: 1,
	}

	atePort1 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port1",
			MAC:     "02:00:01:01:01:01",
			IPv4:    "192.0.2.2",
			IPv4Len: plenIPv4,
			IPv6:    "2001:0db8::192:0:2:2",
			IPv6Len: plenIPv6,
		},
		numSubIntf: 1,
		gateway: func(_ uint8) string {
			return "192.0.2.1"
		},
		gateway6: func(_ uint8) string {
			return "2001:0db8::192:0:2:1"
		},
	}
	atePort2 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port2",
			MAC:     "02:00:02:01:01:01",
			IPv4:    "192.0.2.6",
			IPv4Len: plenIPv4,
			IPv6:    "2001:0db8::192:0:2:6",
			IPv6Len: plenIPv6,
		},
		numSubIntf: 100,
		ip4:        atePort2IPv4,
		gateway:    dutPort2IPv4,
	}
	atePort3 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port3",
			MAC:     "02:00:03:01:01:01",
			IPv4:    "192.0.2.10",
			IPv4Len: plenIPv4,
			IPv6:    "2001:0db8::192:0:2:a",
			IPv6Len: plenIPv6,
		},
		numSubIntf: 50,
		ip6:        atePort3IPv6,
		gateway:    dutPort3IPv6,
	}
	atePort4 = attributes{
		Attributes: &attrs.Attributes{
			Name:    "port4",
			MAC:     "02:00:04:01:01:01",
			IPv4:    "200.0.0.2", // 192.0.2.14
			IPv4Len: 24,
			IPv6:    "1000::200:0:0:2", // "2001:0db8::192:0:2:e"
			IPv6Len: 126,
		},
		ip4: func(_ uint8) string {
			return "200.0.0.2"
		},
		ip6: func(_ uint8) string {
			return "1000::200:0:0:2"
		},
		numSubIntf: 1,
		gateway: func(_ uint8) string {
			return "200.0.0.1"
		},
		gateway6: func(_ uint8) string {
			return "1000::200:0:0:1"
		},
	}
)

type attributes struct {
	*attrs.Attributes
	numSubIntf uint32
	ip4        func(vlan uint8) string
	ip6        func(vlan uint8) string
	gateway    func(vlan uint8) string
	gateway6   func(vlan uint8) string
}

// dutPort2IPv4 returns ip addresses starting 50.1.%d.1 for every vlanID.
func dutPort2IPv4(vlan uint8) string {
	return fmt.Sprintf("50.1.%d.1", vlan)
}

// dutPort2IPv6 returns ip addresses starting 1000:%d::50.1.1.1 for every vlanID.
func dutPort3IPv6(vlan uint8) string {
	return fmt.Sprintf("1000:%d::50:1:1:1", vlan)
}

// atePort2IPv4 returns ip addresses starting 50.1.%d.2, increasing by 4
// for every vlanID.
func atePort2IPv4(vlan uint8) string {
	return fmt.Sprintf("50.1.%d.2", vlan)
}

// atePort2IPv6 returns ip addresses starting 1000:%d::50.1.1.2 for every vlanID.
func atePort3IPv6(vlan uint8) string {
	return fmt.Sprintf("1000:%d::50:1:1:2", vlan)
}

// cidr takes as input the IPv4 address and the mask and returns the IP string in
// CIDR notation.
func cidr(ipv4 string, ones int) string {
	return ipv4 + "/" + strconv.Itoa(ones)
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestAddPathScale(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	top := gosnappi.NewConfig()
	for _, atePort := range []attributes{atePort1, atePort2, atePort3, atePort4} {
		atePort.configureATE(t, top, ate)
	}
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	configureDUT(t, dut)
	configureRoutePolicy(t, dut, "ALLOW", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	nbrList := buildNeighborList(atePort2, atePort3, atePort4)
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("configureBGP", func(t *testing.T) {
		dutConf := bgpWithNbr(dutAS, nbrList, dut, "ALLOW", "ALLOW")
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

		atePort2.configureBGPOnATE(t, top, 1, ateAS2)
		atePort3.configureBGPOnATE(t, top, 2, ateAS3)
		atePort4.configureBGPOnATE(t, top, 3, ateAS4)
		advertiseRoutesWithEBGP(t, top)
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
	})

	testCases := []struct {
		name        string
		applyConfig func(t *testing.T, dut *ondatra.DUTDevice)
		validate    func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config)
	}{
		{
			name:        "RT-1.15.1 - AddPath Disabled",
			applyConfig: configAddPathDisabled,
			validate:    validateAddPathDisabled,
		},
		{
			name:        "RT-1.15.2 - AddPath Receive Enabled",
			applyConfig: configAddPathReceive,
			validate:    validateAddPathReceive,
		},
		{
			name:        "RT-1.15.3 - AddPath Receive and Send Enabled",
			applyConfig: configAddPathReceiveSend,
			validate:    validateAddPathReceiveSend,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.applyConfig != nil {
				tc.applyConfig(t, dut)
			}
			time.Sleep(30 * time.Second)
			tc.validate(t, dut, ate, top)
		})
	}
}

func TestAddPathScaleWithRoutePolicy(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	top := gosnappi.NewConfig()
	for _, atePort := range []attributes{atePort1, atePort2, atePort3, atePort4} {
		atePort.configureATE(t, top, ate)
	}
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	configureDUT(t, dut)
	configureRoutePolicy(t, dut, "ALLOW", oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	configureImportBGPPolicy(t, dut)

	nbrList := buildNeighborList(atePort2, atePort3, atePort4)
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	t.Run("configureBGP", func(t *testing.T) {
		dutConf := bgpWithNbr(dutAS, nbrList, dut, "community-match", "ALLOW")
		gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
		fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

		atePort2.configureBGPOnATE(t, top, 1, ateAS2)
		atePort3.configureBGPOnATE(t, top, 2, ateAS3)
		atePort4.configureBGPOnATE(t, top, 3, ateAS4)
		advertiseRoutesWithEBGPWithCommunities(t, top)
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
	})

	createTrafficFlow(t, top, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(30 * time.Second)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	loss := otgutils.GetFlowLossPct(t, ate.OTG(), "flow1", 10*time.Second)
	if loss > lossTolerance {
		t.Errorf("Loss percent for IPv4 Traffic: got: %f, want %f", loss, lossTolerance)
	}
}

type communitySet struct {
	name            string
	communityMatch  []string
	matchSetOptions oc.E_BgpPolicy_MatchSetOptionsType
}

type policyStatement struct {
	name         string
	cs           communitySet
	policyResult oc.E_RoutingPolicy_PolicyResultType
}

func configureImportBGPPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition("community-match")
	statements := []policyStatement{
		{
			name: "accept_any_3_comms",
			cs: communitySet{
				name:            "any_3_comms",
				communityMatch:  []string{"100:1", "200:1", "201:1"},
				matchSetOptions: oc.BgpPolicy_MatchSetOptionsType_ANY,
			},
			policyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		},
		{
			name: "accept_any_my_regex_comms",
			cs: communitySet{
				name:            "any_my_regex_comms",
				communityMatch:  []string{"10[0-9]:1"},
				matchSetOptions: oc.BgpPolicy_MatchSetOptionsType_ANY,
			},
			policyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		},
	}
	if !deviations.MatchCommunitySetMatchSetOptionsAllUnsupported(dut) {
		allStatement := policyStatement{
			name: "accept_all_3_comms",
			cs: communitySet{
				name:            "all_3_comms",
				communityMatch:  []string{"100:1", "104:1", "201:1"},
				matchSetOptions: oc.BgpPolicy_MatchSetOptionsType_ALL,
			},
			policyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		}
		statements = append(statements, allStatement)
	}
	for _, s := range statements {
		stmt, err := pd.AppendNewStatement(s.name)
		if err != nil {
			t.Fatalf("AppendNewStatement(%s) failed: %v", "routePolicyStatement", err)
		}
		createCommunitySet(t, dut, s.cs, rp)
		if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
			stmt.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(s.cs.name)
		} else {
			matchCommunitySet := stmt.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
			matchCommunitySet.SetCommunitySet(s.cs.name)
			matchCommunitySet.SetMatchSetOptions(oc.E_RoutingPolicy_MatchSetOptionsType(s.cs.matchSetOptions))
		}
		stmt.GetOrCreateActions().PolicyResult = s.policyResult
	}

	pdAllow := rp.GetOrCreatePolicyDefinition("ALLOW")
	st, err := pdAllow.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

func createCommunitySet(t *testing.T, dut *ondatra.DUTDevice, cs communitySet, rp *oc.RoutingPolicy) {
	if !(deviations.CommunityMemberRegexUnsupported(dut) && cs.name == communitySetNameRegex) {
		communitySet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(cs.name)
		var cm []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union
		for _, commMatch := range cs.communityMatch {
			if commMatch != "" {
				cm = append(cm, oc.UnionString(commMatch))
			}
		}
		communitySet.SetCommunityMember(cm)
		communitySet.SetMatchSetOptions(cs.matchSetOptions)
	}
	var communitySetCLIConfig string
	if deviations.CommunityMemberRegexUnsupported(dut) && cs.name == communitySetNameRegex {
		switch dut.Vendor() {
		case ondatra.CISCO:
			communitySetCLIConfig = fmt.Sprintf("community-set %v\n ios-regex '10[0-9]:1'\n end-set", cs.name)
		default:
			t.Fatalf("Unsupported vendor %s for deviation 'CommunityMemberRegexUnsupported'", dut.Vendor())
		}
		helpers.GnmiCLIConfig(t, dut, communitySetCLIConfig)
	}
}

func buildNeighborList(atePort2, atePort3, atePort4 attributes) []*bgpNeighbor {
	nbrList2 := atePort2.buildIPv4NbrList(ateAS2, peerv42GrpName, peerv42GrpName)
	nbrList3 := atePort3.buildIPv4NbrList(ateAS3, peerv63GrpName, peerv63GrpName)
	nbrList4 := atePort4.buildIPv4NbrList(ateAS4, peerv44GrpName, peerv64GrpName)
	nbrList := append(nbrList2, nbrList3...)
	nbrList = append(nbrList, nbrList4...)
	return nbrList
}

func validateAddPathDisabled(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
	validatePrefixes(t, dut, "50.1.1.2", "1000:1::50:1:1:2")
}

func validateAddPathReceive(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
	validatePrefixes(t, dut, "200.0.0.2", "1000::200:0:0:2")

	createTrafficFlow(t, top, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(30 * time.Second)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	// otgutils.LogPortMetrics(t, ate.OTG(), top)
	loss := otgutils.GetFlowLossPct(t, ate.OTG(), "flow1", 10*time.Second)
	if loss > lossTolerance {
		t.Errorf("Loss percent for IPv4 Traffic: got: %f, want %f", loss, lossTolerance)
	}
}

func validateAddPathReceiveSend(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config) {
	validatePrefixes(t, dut, "200.0.0.2", "1000::200:0:0:2")
}

func configAddPathDisabled(t *testing.T, dut *ondatra.DUTDevice) {
	ocRoot := &oc.Root{}
	nbrList := buildNeighborList(atePort2, atePort3, atePort4)
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	bgp := ocRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	for _, nbr := range nbrList {
		nbrD := bgp.GetOrCreateNeighbor(nbr.neighborip)
		afiSafiType := oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
		if nbr.isV4 {
			afiSafiType = oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
		}
		nbrD.GetOrCreateAfiSafi(afiSafiType).GetOrCreateAddPaths().SetSend(false)
		nbrD.GetOrCreateAfiSafi(afiSafiType).GetOrCreateAddPaths().SetReceive(false)
	}
	gnmi.Update(t, dut, bgpPath.Config(), bgp)
}

func configAddPathReceive(t *testing.T, dut *ondatra.DUTDevice) {
	ocRoot := &oc.Root{}
	nbrList := buildNeighborList(atePort2, atePort3, atePort4)
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	bgp := ocRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	for _, nbr := range nbrList {
		nbrD := bgp.GetOrCreateNeighbor(nbr.neighborip)
		afiSafiType := oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
		if nbr.isV4 {
			afiSafiType = oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
		}
		nbrD.GetOrCreateAfiSafi(afiSafiType).GetOrCreateAddPaths().SetSend(false)
		nbrD.GetOrCreateAfiSafi(afiSafiType).GetOrCreateAddPaths().SetReceive(true)
	}
	gnmi.Update(t, dut, bgpPath.Config(), bgp)
	// ate.OTG().StopProtocols(t)
	// ate.OTG().StartProtocols(t)
}

func configAddPathReceiveSend(t *testing.T, dut *ondatra.DUTDevice) {
	ocRoot := &oc.Root{}
	nbrList := buildNeighborList(atePort2, atePort3, atePort4)
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	bgp := ocRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").GetOrCreateBgp()
	for _, nbr := range nbrList {
		nbrD := bgp.GetOrCreateNeighbor(nbr.neighborip)
		afiSafiType := oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
		if nbr.isV4 {
			afiSafiType = oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
		}
		nbrD.GetOrCreateAfiSafi(afiSafiType).GetOrCreateAddPaths().SetSend(true)
		nbrD.GetOrCreateAfiSafi(afiSafiType).GetOrCreateAddPaths().SetReceive(true)
	}
	gnmi.Update(t, dut, bgpPath.Config(), bgp)
	// ate.OTG().StopProtocols(t)
	// ate.OTG().StartProtocols(t)
}

func validatePrefixes(t *testing.T, dut *ondatra.DUTDevice, neighborIPv4, neighborIPv6 string) {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	ipv4Pfx := bgpPath.Neighbor(neighborIPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	if gotReceived, ok := gnmi.Await(t, dut, ipv4Pfx.Received().State(), 15*time.Minute, ipv4Prefixes).Val(); !ok {
		t.Errorf("Received IPv4 Prefixes - got: %v, want: %v", gotReceived, ipv4Prefixes)
	}

	if gotInstalled, ok := gnmi.Await(t, dut, ipv4Pfx.Received().State(), 15*time.Minute, ipv4Prefixes).Val(); !ok {
		t.Errorf("Received IPv4 Prefixes - got: %v, want: %v", gotInstalled, ipv4Prefixes)
	}

	ipv6Pfx := bgpPath.Neighbor(neighborIPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()
	if gotReceived, ok := gnmi.Await(t, dut, ipv6Pfx.Received().State(), 15*time.Minute, ipv6Prefixes).Val(); !ok {
		t.Errorf("Received IPv6 Prefixes - got: %v, want: %v", gotReceived, ipv6Prefixes)
	}

	if gotInstalled, ok := gnmi.Await(t, dut, ipv6Pfx.Received().State(), 15*time.Minute, ipv6Prefixes).Val(); !ok {
		t.Errorf("Received IPv6 Prefixes - got: %v, want: %v", gotInstalled, ipv6Prefixes)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	for _, dutPort := range []attributes{dutPort1, dutPort2, dutPort3, dutPort4} {
		dutPort.configInterfaceDUT(t, dut)
		dutPort.assignSubifsToDefaultNetworkInstance(t, dut)
	}
}

func (a *attributes) configInterfaceDUT(t *testing.T, d *ondatra.DUTDevice) {
	t.Helper()
	p := d.Port(t, a.Name)
	i := &oc.Interface{Name: ygot.String(p.Name())}

	if a.numSubIntf > 1 {
		i.Description = ygot.String(a.Desc)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		if deviations.InterfaceEnabled(d) {
			i.Enabled = ygot.Bool(true)
		}
	} else {
		i = a.NewOCInterface(p.Name(), d)
	}

	if deviations.ExplicitPortSpeed(d) {
		i.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, p)
	}

	if deviations.RequireRoutedSubinterface0(d) && a.numSubIntf == 1 {
		s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		s4.Enabled = ygot.Bool(true)
		s6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
		s6.Enabled = ygot.Bool(true)
	}

	a.configSubinterfaceDUT(t, i, d)
	intfPath := gnmi.OC().Interface(p.Name())
	gnmi.Update(t, d, intfPath.Config(), i)
	fptest.LogQuery(t, "DUT", intfPath.Config(), gnmi.Get(t, d, intfPath.Config()))
}

func (a *attributes) configSubinterfaceDUT(t *testing.T, intf *oc.Interface, dut *ondatra.DUTDevice) {
	t.Helper()

	if a.numSubIntf == 1 {
		return
	}

	for i := uint32(1); i <= a.numSubIntf; i++ {
		s := intf.GetOrCreateSubinterface(i)
		if deviations.InterfaceEnabled(dut) {
			s.Enabled = ygot.Bool(true)
		}
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(i)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(i))
		}

		s4 := s.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			s4.Enabled = ygot.Bool(true)
		}

		if a.Name == "port2" {
			ip := a.ip4(uint8(i))
			s4a := s4.GetOrCreateAddress(ip)
			s4a.PrefixLength = ygot.Uint8(v4VlanPlen)
			t.Logf("Adding DUT Subinterface with ID: %d, Vlan ID: %d and IPv4 address: %s", i, i, ip)
		} else if a.Name == "port3" {
			ip := a.ip6(uint8(i))
			s6 := s.GetOrCreateIpv6()
			if deviations.InterfaceEnabled(dut) {
				s6.Enabled = ygot.Bool(true)
			}
			s6a := s6.GetOrCreateAddress(ip)
			s6a.PrefixLength = ygot.Uint8(v6VlanPlen)
			t.Logf("Adding DUT Subinterface with ID: %d, Vlan ID: %d and IPv6 address: %s", i, i, ip)
		}
	}
}

func (a *attributes) assignSubifsToDefaultNetworkInstance(t *testing.T, d *ondatra.DUTDevice) {
	p := d.Port(t, a.Name)
	if deviations.ExplicitInterfaceInDefaultVRF(d) {
		if a.numSubIntf == 1 {
			fptest.AssignToNetworkInstance(t, d, p.Name(), deviations.DefaultNetworkInstance(d), 0)
		} else {
			for i := uint32(1); i <= a.numSubIntf; i++ {
				fptest.AssignToNetworkInstance(t, d, p.Name(), deviations.DefaultNetworkInstance(d), i)
			}
		}
	}
}

func (a *attributes) configureATE(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice) {
	t.Helper()
	p := ate.Port(t, a.Name)

	top.Ports().Add().SetName(p.ID())
	if a.numSubIntf == 1 {
		gateway := a.gateway(1)
		gateway6 := a.gateway6(1)
		dev := top.Devices().Add().SetName(a.Name)
		eth := dev.Ethernets().Add().SetName(a.Name + ".Eth").SetMac(a.MAC)
		eth.Connection().SetPortName(p.ID())
		ipObj4 := eth.Ipv4Addresses().Add().SetName(dev.Name() + ".IPv4")
		ipObj4.SetAddress(a.IPv4).SetGateway(gateway).SetPrefix(uint32(a.IPv4Len))
		t.Logf("Adding ATE Ipv4 address: %s with gateway: %s", cidr(a.IPv4, int(a.IPv4Len)), gateway)
		ipObj6 := eth.Ipv6Addresses().Add().SetName(dev.Name() + ".IPv6")
		ipObj6.SetAddress(a.IPv6).SetGateway(gateway6).SetPrefix(uint32(a.IPv6Len))
		t.Logf("Adding ATE Ipv6 address: %s with gateway: %s", cidr(a.IPv6, int(a.IPv6Len)), gateway)
		return
	}

	for i := uint32(1); i <= a.numSubIntf; i++ {
		name := fmt.Sprintf(`%sdst%d`, a.Name, i)

		gateway := a.gateway(uint8(i))
		mac, err := incrementMAC(a.MAC, int(i)+1)
		if err != nil {
			t.Fatalf("Failed to generate mac address with error %s", err)
		}

		dev := top.Devices().Add().SetName(name + ".Dev")
		eth := dev.Ethernets().Add().SetName(name + ".Eth").SetMac(mac)
		eth.Connection().SetPortName(p.ID())
		eth.Vlans().Add().SetName(name).SetId(uint32(i))
		if a.Name == "port2" {
			ip := a.ip4(uint8(i))
			eth.Ipv4Addresses().Add().SetName(name + ".IPv4").SetAddress(ip).SetGateway(gateway).SetPrefix(uint32(v4VlanPlen))
			t.Logf("Adding ATE Ipv4 address: %s with gateway: %s and VlanID: %d", cidr(ip, int(v4VlanPlen)), gateway, i)
		} else if a.Name == "port3" {
			ip := a.ip6(uint8(i))
			eth.Ipv6Addresses().Add().SetName(name + ".IPv6").SetAddress(ip).SetGateway(gateway).SetPrefix(uint32(v6VlanPlen))
			t.Logf("Adding ATE Ipv6 address: %s with gateway: %s and VlanID: %d", cidr(ip, int(v6VlanPlen)), gateway, i)
		}
	}
}

func (a *attributes) buildIPv4NbrList(asn uint32, v4pg, v6pg string) []*bgpNeighbor {
	var nbrList []*bgpNeighbor
	for i := uint32(1); i <= a.numSubIntf; i++ {
		if a.ip4 != nil {
			ip := a.ip4(uint8(i))
			bgpNbr := &bgpNeighbor{
				as:         asn,
				neighborip: ip,
				isV4:       true,
				pg:         v4pg,
			}
			nbrList = append(nbrList, bgpNbr)
		}
		if a.ip6 != nil {
			ip := a.ip6(uint8(i))
			bgpNbr := &bgpNeighbor{
				as:         asn,
				neighborip: ip,
				isV4:       false,
				pg:         v6pg,
			}
			nbrList = append(nbrList, bgpNbr)
		}
	}
	return nbrList
}

func (a *attributes) configureBGPOnATE(t *testing.T, top gosnappi.Config, pi int, asn uint32) {
	t.Helper()

	devices := top.Devices().Items()
	// byName := func(i, j int) bool { return devices[i].Name() < devices[j].Name() }
	// sort.Slice(devices, byName)
	devMap := make(map[string]gosnappi.Device)
	for _, dev := range devices {
		devMap[dev.Name()] = dev
	}

	for i := uint32(1); i <= a.numSubIntf; i++ {
		di := a.Name
		if a.Name == "port2" || a.Name == "port3" {
			di = fmt.Sprintf("%sdst%d.Dev", a.Name, i)
		}
		device := devMap[di]
		if a.ip4 != nil {
			bgp := device.Bgp().SetRouterId(a.ip4(uint8(i)))
			ipv4 := device.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
			bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ipv4.Name()).Peers().Add().SetName(device.Name() + ".BGP4.peer")
			bgp4Peer.SetPeerAddress(ipv4.Gateway())
			bgp4Peer.SetAsNumber(asn)
			bgp4Peer.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
			bgp4Peer.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
			bgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true)
		}
		if a.ip6 != nil {
			bgp := device.Bgp().SetRouterId(a.IPv4)
			ipv6 := device.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
			bgp6Peer := bgp.Ipv6Interfaces().Add().SetIpv6Name(ipv6.Name()).Peers().Add().SetName(device.Name() + ".BGP6.peer")
			bgp6Peer.SetPeerAddress(ipv6.Gateway())
			bgp6Peer.SetAsNumber(asn)
			bgp6Peer.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
			bgp6Peer.Capability().SetIpv6UnicastAddPath(true)
			bgp6Peer.LearnedInformationFilter().SetUnicastIpv6Prefix(true)
		}
	}
}

func advertiseRoutesWithEBGP(t *testing.T, top gosnappi.Config) {
	devices := top.Devices().Items()
	prefixesV4 := createPrefixesV4(t)
	for _, device := range devices {
		if strings.Contains(device.Name(), "port2") {
			bgp4Peer := device.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
			netv4 := bgp4Peer.V4Routes().Add().SetName(fmt.Sprintf("v4-bgpNet-%s", device.Name()))
			for _, prefix := range prefixesV4 {
				netv4.Addresses().Add().SetAddress(prefix.Addr().String()).SetPrefix(uint32(prefix.Bits()))
				netv4.AddPath().SetPathId(uint32(1))
			}
		}
	}

	prefixesV6 := createPrefixesV6(t)
	for _, device := range devices {
		if strings.Contains(device.Name(), "port3") {
			bgp6Peer := device.Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]
			netv6 := bgp6Peer.V6Routes().Add().SetName(fmt.Sprintf("v6-bgpNet-%s", device.Name()))
			for _, prefix := range prefixesV6 {
				netv6.Addresses().Add().SetAddress(prefix.Addr().String()).SetPrefix(uint32(prefix.Bits()))
				netv6.AddPath().SetPathId(uint32(1))
			}
		}
	}
}

func advertiseRoutesWithEBGPWithCommunities(t *testing.T, top gosnappi.Config) {
	t.Helper()

	devices := top.Devices().Items()

	prefixesV4 := createPrefixesV4(t)
	pfxLenMapV4 := make(map[int][]netip.Prefix)
	for _, prefix := range prefixesV4 {
		pfxLenMapV4[int(prefix.Bits())] = append(pfxLenMapV4[int(prefix.Bits())], prefix)
	}
	for _, device := range devices {
		if strings.Contains(device.Name(), "port2") {
			bgp4Peer := device.Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
			for pfxLen, pfxs := range pfxLenMapV4 {
				netv4 := bgp4Peer.V4Routes().Add().SetName(fmt.Sprintf("v4-bgpNet-%s-%d", device.Name(), pfxLen))
				for _, prefix := range pfxs {
					netv4.Addresses().Add().SetAddress(prefix.Addr().String()).SetPrefix(uint32(prefix.Bits()))
					netv4.AddPath().SetPathId(uint32(1))
				}
				switch pfxLen {
				case 22:
					commv41 := netv4.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
					commv41.SetAsNumber(uint32(100)).SetAsCustom(uint32(1))
					commv42 := netv4.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
					commv42.SetAsNumber(uint32(200)).SetAsCustom(uint32(1))
				case 24:
					commv41 := netv4.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
					commv41.SetAsNumber(uint32(101)).SetAsCustom(uint32(1))
					commv42 := netv4.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
					commv42.SetAsNumber(uint32(201)).SetAsCustom(uint32(1))
				case 30:
					commv41 := netv4.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
					commv41.SetAsNumber(uint32(104)).SetAsCustom(uint32(1))
					commv42 := netv4.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
					commv42.SetAsNumber(uint32(109)).SetAsCustom(uint32(3))
				}
			}
		}
	}

	prefixesV6 := createPrefixesV6(t)
	pfxLenMapV6 := make(map[int][]netip.Prefix)
	for _, prefix := range prefixesV6 {
		pfxLenMapV6[int(prefix.Bits())] = append(pfxLenMapV6[int(prefix.Bits())], prefix)
	}
	for _, device := range devices {
		if strings.Contains(device.Name(), "port3") {
			bgp6Peer := device.Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]
			for pfxLen, pfxs := range pfxLenMapV6 {
				netv6 := bgp6Peer.V6Routes().Add().SetName(fmt.Sprintf("v6-bgpNet-%s-%d", device.Name(), pfxLen))
				for _, prefix := range pfxs {
					netv6.Addresses().Add().SetAddress(prefix.Addr().String()).SetPrefix(uint32(prefix.Bits()))
					netv6.AddPath().SetPathId(uint32(1))
				}
				switch pfxLen {
				case 22:
					commv61 := netv6.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
					commv61.SetAsNumber(uint32(100)).SetAsCustom(uint32(1))
					commv62 := netv6.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
					commv62.SetAsNumber(uint32(200)).SetAsCustom(uint32(1))
				case 24:
					commv61 := netv6.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
					commv61.SetAsNumber(uint32(101)).SetAsCustom(uint32(1))
					commv62 := netv6.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
					commv62.SetAsNumber(uint32(201)).SetAsCustom(uint32(1))
				case 30:
					commv61 := netv6.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
					commv61.SetAsNumber(uint32(104)).SetAsCustom(uint32(1))
					commv62 := netv6.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
					commv62.SetAsNumber(uint32(109)).SetAsCustom(uint32(3))
				}
			}
		}
	}
}

func createPrefixesV4(t *testing.T) []netip.Prefix {
	t.Helper()

	var ips []netip.Prefix
	// Create /22s - 768
	for i := 20; i < 32; i++ {
		for j := 0; j < 256; j += 4 {
			ips = append(ips, netip.MustParsePrefix(fmt.Sprintf("172.%d.%d.0/22", i, j)))
		}
	}

	// Create /24s - 2250
	for i := 32; i < 41; i++ {
		for j := 0; j < 250; j++ {
			ips = append(ips, netip.MustParsePrefix(fmt.Sprintf("172.%d.%d.0/24", i, j)))
		}
	}

	// Create /30s - 11776
	for i := 0; i < 184; i++ {
		for j := 0; j < 256; j += 4 {
			ips = append(ips, netip.MustParsePrefix(fmt.Sprintf("172.41.%d.%d/30", i, j)))
		}
	}

	return ips
}

func createPrefixesV6(t *testing.T) []netip.Prefix {
	t.Helper()
	var ips []netip.Prefix
	// Create /48s - 120
	for i := 0; i < 120; i++ {
		ip := netip.MustParsePrefix(fmt.Sprintf("2001:db8:%d::/48", i))
		ips = append(ips, ip)
	}

	// Create /64s - 360
	for i := 0; i < 360; i++ {
		ip := netip.MustParsePrefix(fmt.Sprintf("fc00:abcd:1:%d::/64", i))
		ips = append(ips, ip)
	}

	// Create /126s - 1920
	for i := 0; i < 1920; i++ {
		ip := netip.MustParsePrefix(fmt.Sprintf("fc00:abcd::%d:1/126", i))
		ips = append(ips, ip)
	}

	return ips
}

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

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
	pg         string
}

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	t.Helper()
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

func bgpWithNbr(as uint32, nbrs []*bgpNeighbor, dut *ondatra.DUTDevice, impPolicy, expPolicy string) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(as)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	g.RouterId = ygot.String(dutPort2.IPv4)

	pg24 := bgp.GetOrCreatePeerGroup(peerv42GrpName)
	pg24.PeerAs = ygot.Uint32(ateAS2)

	pgv63 := bgp.GetOrCreatePeerGroup(peerv63GrpName)
	pgv63.PeerAs = ygot.Uint32(ateAS3)

	pgv44 := bgp.GetOrCreatePeerGroup(peerv44GrpName)
	pgv44.PeerAs = ygot.Uint32(ateAS4)

	pgv64 := bgp.GetOrCreatePeerGroup(peerv64GrpName)
	pgv64.PeerAs = ygot.Uint32(ateAS4)

	pgs4 := []*oc.NetworkInstance_Protocol_Bgp_PeerGroup{pg24, pgv44}
	pgs6 := []*oc.NetworkInstance_Protocol_Bgp_PeerGroup{pgv63, pgv64}
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		for _, pg := range append(pgs4, pgs6...) {
			rpl := pg.GetOrCreateApplyPolicy()
			rpl.SetExportPolicy([]string{"ALLOW"})
			rpl.SetImportPolicy([]string{"ALLOW"})
		}
	} else {
		for _, pg := range pgs4 {
			pgaf := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			pgaf.Enabled = ygot.Bool(true)
			rpl := pgaf.GetOrCreateApplyPolicy()
			rpl.SetExportPolicy([]string{"ALLOW"})
			rpl.SetImportPolicy([]string{"ALLOW"})
		}

		for _, pg := range pgs6 {
			pgaf := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			pgaf.Enabled = ygot.Bool(true)
			rpl := pgaf.GetOrCreateApplyPolicy()
			rpl.SetExportPolicy([]string{"ALLOW"})
			rpl.SetImportPolicy([]string{"ALLOW"})
		}
	}

	for _, nbr := range nbrs {
		bgpNbr := bgp.GetOrCreateNeighbor(nbr.neighborip)
		bgpNbr.GetOrCreateTimers().HoldTime = ygot.Uint16(180)
		bgpNbr.GetOrCreateTimers().KeepaliveInterval = ygot.Uint16(60)
		bgpNbr.GetOrCreateTimers().SetMinimumAdvertisementInterval(10)
		bgpNbr.PeerAs = ygot.Uint32(nbr.as)
		bgpNbr.Enabled = ygot.Bool(true)
		bgpNbr.PeerGroup = ygot.String(nbr.pg)
		if nbr.isV4 {
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
		}
	}
	return niProto
}

func createTrafficFlow(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice) {
	t.Helper()

	prefixesV4 := createPrefixesV4(t)
	var dstIps []string
	for _, prefix := range prefixesV4 {
		dstIps = append(dstIps, prefix.Addr().String())
	}

	top.Flows().Clear()
	for i := uint32(1); i <= atePort2.numSubIntf; i++ {
		rxName := fmt.Sprintf(`v4-bgpNet-%sdst%d.Dev`, atePort2.Name, i)
		rxNames := []string{}
		for _, pfxLen := range []int{22, 24, 30} {
			rxNames = append(rxNames, fmt.Sprintf(`%s-%d`, rxName, pfxLen))
		}
		flow := top.Flows().Add().SetName(fmt.Sprintf("flow%d", i))
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames(rxNames)
		flow.Size().SetFixed(512)
		flow.Rate().SetPps(100)
		e1 := flow.Packet().Add().Ethernet()
		e1.Src().SetValue(atePort1.MAC)
		e1.Dst().SetValue(atePort2.MAC)
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(atePort1.IPv4)
		v4.Dst().SetValues(dstIps)
	}

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
}
