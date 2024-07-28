// Copyright 2024 Google LLC
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

package link_bandwidth_test

import (
	"fmt"
	"strconv"
	"strings"
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
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen     = 30
	ipv6PrefixLen     = 126
	v41Route          = "203.0.113.0"
	v41TrafficStart   = "203.0.113.1"
	v42Route          = "203.0.114.0"
	v42TrafficStart   = "203.0.114.1"
	v43Route          = "203.0.115.0"
	v43TrafficStart   = "203.0.115.1"
	v4RoutePrefix     = uint32(24)
	v61Route          = "2001:db8:128:128::0"
	v61RouteOtg       = "2001:db8:128:128::"
	v61RouteAdvertise = "2001:db8:128:128::/64"
	v61TrafficStart   = "2001:db8:128:128::1"
	v62Route          = "2001:db8:128:129::0"
	v62RouteAdvertise = "2001:db8:128:129::/64"
	v62RouteOtg       = "2001:db8:128:129::"
	v62TrafficStart   = "2001:db8:128:129::1"
	v63Route          = "2001:db8:128:130::0"
	v63RouteAdvertise = "2001:db8:128:130::/64"
	v63RouteOtg       = "2001:db8:128:130::"
	v63TrafficStart   = "2001:db8:128:130::1"
	v6RoutePrefix     = uint32(64)
	dutAS             = uint32(32001)
	ateAS             = uint32(32002)
	bgpName           = "BGP"
	maskLenExact      = "exact"
	localPref         = 200
	v4Flow            = "flow-v4"
	v6Flow            = "flow-v6"
	localPerfCfg      = 5
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:01:01:01:02",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}

	advertisedIPv41 = ipAddr{address: v41Route, prefix: v4RoutePrefix}
	advertisedIPv42 = ipAddr{address: v42Route, prefix: v4RoutePrefix}
	advertisedIPv43 = ipAddr{address: v43Route, prefix: v4RoutePrefix}
	advertisedIPv61 = ipAddr{address: v61Route, prefix: v6RoutePrefix}
	advertisedIPv62 = ipAddr{address: v62Route, prefix: v6RoutePrefix}
	advertisedIPv63 = ipAddr{address: v63Route, prefix: v6RoutePrefix}

	extCommunitySet = map[string]string{
		"linkbw_1M":  "link-bandwidth:23456:1M",
		"linkbw_2G":  "link-bandwidth:23456:2G",
		"linkbw_any": "^link-bandwidth:.*:.$",
	}

	extCommunitySetCisco = map[string]string{
		"linkbw_1M":  "23456:1000000",
		"linkbw_2G":  "23456:2000000000",
		"linkbw_any": "^.*:.$",
	}

	CommunitySet = map[string]string{
		"regex_match_comm100": "^100:.*$",
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type ipAddr struct {
	address string
	prefix  uint32
}

type testData struct {
	dut   *ondatra.DUTDevice
	ate   *ondatra.ATEDevice
	top   gosnappi.Config
	otgP1 gosnappi.Device
	otgP2 gosnappi.Device
}

type extCommunity struct {
	prefixSet1Comm string
	prefixSet2Comm string
	prefixSet3Comm string
}

type flowConfig struct {
	src   attrs.Attributes
	dstNw string
	dstIP string
}

func TestBGPLinkBandwidth(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	devs := configureOTG(t, ate, top)
	td := testData{
		dut:   dut,
		ate:   ate,
		top:   top,
		otgP1: devs[0],
		otgP2: devs[1],
	}
	type testCase struct {
		name                     string
		policyName               string
		applyPolicy              func(t *testing.T, dut *ondatra.DUTDevice, policyName string)
		validate                 func(t *testing.T, dut *ondatra.DUTDevice, policyName string)
		routeCommunity           extCommunity
		localPerf                bool
		validateRouteCommunityV4 func(t *testing.T, td testData, ec extCommunity)
		validateRouteCommunityV6 func(t *testing.T, td testData, ec extCommunity)
	}
	baseSetupConfigAndVerification(t, td)
	configureExtCommunityRoutingPolicy(t, dut)
	if deviations.BgpExplicitExtendedCommunityEnable(dut) {
		enableExtCommunityCLIConfig(t, dut)
	}
	testCases := []testCase{
		{
			name:                     "Policy set not_match_100_set_linkbw_1M",
			policyName:               "not_match_100_set_linkbw_1M",
			applyPolicy:              applyPolicyDut,
			validate:                 validatPolicyDut,
			routeCommunity:           extCommunity{prefixSet1Comm: "none", prefixSet2Comm: "100:100", prefixSet3Comm: "link-bandwidth:23456:0"},
			localPerf:                false,
			validateRouteCommunityV4: validateRouteCommunityV4,
			validateRouteCommunityV6: validateRouteCommunityV6,
		},
		{
			name:                     "Policy set match_100_set_linkbw_2G",
			policyName:               "match_100_set_linkbw_2G",
			applyPolicy:              applyPolicyDut,
			validate:                 validatPolicyDut,
			routeCommunity:           extCommunity{prefixSet1Comm: "none", prefixSet2Comm: "link-bandwidth:23456:2000000000", prefixSet3Comm: "link-bandwidth:23456:0"},
			localPerf:                false,
			validateRouteCommunityV4: validateRouteCommunityV4,
			validateRouteCommunityV6: validateRouteCommunityV6,
		},
		{
			name:                     "Policy set del_linkbw",
			policyName:               "del_linkbw",
			applyPolicy:              applyPolicyDut,
			validate:                 validatPolicyDut,
			routeCommunity:           extCommunity{prefixSet1Comm: "none", prefixSet2Comm: "100:100", prefixSet3Comm: "none"},
			localPerf:                false,
			validateRouteCommunityV4: validateRouteCommunityV4,
			validateRouteCommunityV6: validateRouteCommunityV6,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.name)
			tc.applyPolicy(t, dut, tc.policyName)
			tc.validate(t, dut, tc.policyName)
			tc.validateRouteCommunityV4(t, td, tc.routeCommunity)
			tc.validateRouteCommunityV6(t, td, tc.routeCommunity)
		})
	}
}

func enableExtCommunityCLIConfig(t *testing.T, dut *ondatra.DUTDevice) {
	var extCommunityEnableCLIConfig string
	switch dut.Vendor() {
	case ondatra.CISCO:
		extCommunityEnableCLIConfig = fmt.Sprintf("router bgp %v instance BGP neighbor-group %v \n ebgp-recv-extcommunity-dmz \n ebgp-send-extcommunity-dmz\n", dutAS, cfgplugins.BGPPeerGroup1)
	default:
		t.Fatalf("Unsupported vendor %s for deviation 'BgpExplicitExtendedCommunityEnable'", dut.Vendor())
	}
	helpers.GnmiCLIConfig(t, dut, extCommunityEnableCLIConfig)
}

func applyPolicyDut(t *testing.T, dut *ondatra.DUTDevice, policyName string) {
	// Apply ipv4 policy to bgp neighbour.
	root := &oc.Root{}
	dni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).
		Bgp().Neighbor(atePort1.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()

	policy := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).
		GetOrCreateBgp().GetOrCreateNeighbor(atePort1.IPv4).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy()
	policy.SetImportPolicy([]string{policyName})
	gnmi.Replace(t, dut, path.Config(), policy)

	// Apply ipv6 policy to bgp neighbour.
	path = gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	policy = root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(atePort1.IPv6).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy()
	policy.SetImportPolicy([]string{policyName})
	gnmi.Replace(t, dut, path.Config(), policy)

	ni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	bgp.GetOrCreatePeerGroup(cfgplugins.BGPPeerGroup1).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	bgp.GetOrCreatePeerGroup(cfgplugins.BGPPeerGroup1).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	bgpNbrV4 := bgp.GetOrCreateNeighbor(atePort1.IPv4)
	bgpNbrV4.PeerGroup = ygot.String(cfgplugins.BGPPeerGroup1)
	bgpNbrV6 := bgp.GetOrCreateNeighbor(atePort1.IPv6)
	bgpNbrV6.PeerGroup = ygot.String(cfgplugins.BGPPeerGroup1)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), niProto)
}

func validatPolicyDut(t *testing.T, dut *ondatra.DUTDevice, policyName string) {
	dni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	policy := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy](t, dut, path.State())
	importPolicies := policy.GetImportPolicy()
	if len(importPolicies) != 1 {
		t.Fatalf("ImportPolicy Ipv4 got= %v, want %v", importPolicies, []string{policyName})
	}
}

func validateRouteCommunityV4(t *testing.T, td testData, ec extCommunity) {
	prefixes := map[string]string{
		v41Route: ec.prefixSet1Comm,
		v42Route: ec.prefixSet2Comm,
		v43Route: ec.prefixSet3Comm,
	}
	for prefix, community := range prefixes {
		validateRouteCommunityV4Prefix(t, td, community, prefix)
	}
}

func validateRouteCommunityV4Prefix(t *testing.T, td testData, community, v4Prefix string) {
	_, ok := gnmi.WatchAll(t,
		td.ate.OTG(),
		gnmi.OTG().BgpPeer(td.otgP2.Name()+".BGP4.peer").UnicastIpv4PrefixAny().State(),
		time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)
	if ok {
		bgpPrefixes := gnmi.GetAll(t, td.ate.OTG(), gnmi.OTG().BgpPeer(td.otgP2.Name()+".BGP4.peer").UnicastIpv4PrefixAny().State())
		t.Logf("bgp prefix:%v", bgpPrefixes)
		for _, bgpPrefix := range bgpPrefixes {
			if bgpPrefix.GetAddress() == v4Prefix {
				t.Logf("Prefix recevied on OTG is correct, got  Address %s, want prefix %v", bgpPrefix.GetAddress(), v4Prefix)
				switch community {
				case "none":
					if len(bgpPrefix.Community) != 0 || len(bgpPrefix.ExtendedCommunity) != 0 {
						t.Errorf("community and ext communituy are not empty it should be none")
					}
				case "100:100":
					for _, gotCommunity := range bgpPrefix.Community {
						t.Logf("community AS:%d val: %d", gotCommunity.GetCustomAsNumber(), gotCommunity.GetCustomAsValue())
						if gotCommunity.GetCustomAsNumber() != 100 || gotCommunity.GetCustomAsValue() != 100 {
							t.Errorf("community is not 100:100 got AS number:%d AS value:%d", gotCommunity.GetCustomAsNumber(), gotCommunity.GetCustomAsValue())
						}
					}
				default:
					if len(bgpPrefix.ExtendedCommunity) == 0 {
						t.Errorf("ERROR extended community is empty, expected %v", community)
						return
					}
					for _, ec := range bgpPrefix.ExtendedCommunity {
						lbSubType := ec.Structured.NonTransitive_2OctetAsType.LinkBandwidthSubtype
						listCommunity := strings.Split(community, ":")
						Bandwidth := listCommunity[2]
						if lbSubType.GetGlobal_2ByteAs() != 23456 && lbSubType.GetGlobal_2ByteAs() != 32002 && lbSubType.GetGlobal_2ByteAs() != 32001 {
							t.Errorf("ERROR AS number should be 23456 or %d got %d", ateAS, lbSubType.GetGlobal_2ByteAs())
							return
						}
						if Bandwidth == "0" {
							if ygot.BinaryToFloat32(lbSubType.GetBandwidth()) != 0 {
								t.Errorf("ERROR lb  Bandwidth want 0, got:=%v", ygot.BinaryToFloat32(lbSubType.GetBandwidth()))
							}
						} else {
							if ygot.BinaryToFloat32(lbSubType.GetBandwidth()) != 2.5e+08 && ygot.BinaryToFloat32(lbSubType.GetBandwidth()) != 2000000000 {
								t.Errorf("ERROR lb Bandwidth want :2G, got=%v", ygot.BinaryToFloat32(lbSubType.GetBandwidth()))
							}
						}
						if deviations.BgpExtendedCommunityIndexUnsupported(td.dut) {
							verifyExtCommunityIndexV4(t, td, v4Prefix)
						}
					}
				}
			}
		}
	}
}

func validateRouteCommunityV6(t *testing.T, td testData, ec extCommunity) {
	prefixes := map[string]string{
		v61RouteOtg: ec.prefixSet1Comm,
		v62RouteOtg: ec.prefixSet2Comm,
		v63RouteOtg: ec.prefixSet3Comm,
	}
	for prefix, community := range prefixes {
		validateRouteCommunityV6Prefix(t, td, community, prefix)
	}
}

func validateRouteCommunityV6Prefix(t *testing.T, td testData, community, v6Prefix string) {

	// This function to verify received route communities on ATE ports.
	_, ok := gnmi.WatchAll(t,
		td.ate.OTG(),
		gnmi.OTG().BgpPeer(td.otgP2.Name()+".BGP6.peer").UnicastIpv6PrefixAny().State(),
		time.Minute,
		func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)
	if ok {
		bgpPrefixes := gnmi.GetAll(t, td.ate.OTG(), gnmi.OTG().BgpPeer(td.otgP2.Name()+".BGP6.peer").UnicastIpv6PrefixAny().State())
		for _, bgpPrefix := range bgpPrefixes {
			if bgpPrefix.GetAddress() == v6Prefix {
				t.Logf("Prefix recevied on OTG is correct, got prefix:%v , want prefix %v", bgpPrefix.GetAddress(), v6Prefix)
				switch community {
				case "none":
					if len(bgpPrefix.Community) != 0 || len(bgpPrefix.ExtendedCommunity) != 0 {
						t.Errorf("community and ext community are not empty it should be none")
					}
				case "100:100":
					for _, gotCommunity := range bgpPrefix.Community {
						t.Logf("community AS:%d val: %d", gotCommunity.GetCustomAsNumber(), gotCommunity.GetCustomAsValue())
						if gotCommunity.GetCustomAsNumber() != 100 || gotCommunity.GetCustomAsValue() != 100 {
							t.Errorf("community is not 100:100 got AS number:%d AS value:%d", gotCommunity.GetCustomAsNumber(), gotCommunity.GetCustomAsValue())
						}
					}
				default:
					if len(bgpPrefix.ExtendedCommunity) == 0 {
						t.Errorf("ERROR extended community is empty, expected %v", community)
						return
					}
					for _, ec := range bgpPrefix.ExtendedCommunity {
						lbSubType := ec.Structured.NonTransitive_2OctetAsType.LinkBandwidthSubtype
						listCommunity := strings.Split(community, ":")
						Bandwidth := listCommunity[2]
						if lbSubType.GetGlobal_2ByteAs() != 23456 && lbSubType.GetGlobal_2ByteAs() != 32002 && lbSubType.GetGlobal_2ByteAs() != 32001 {
							t.Errorf("ERROR AS number should be 23456 or %d got %d", ateAS, lbSubType.GetGlobal_2ByteAs())
							return
						}
						if Bandwidth == "0" {
							if ygot.BinaryToFloat32(lbSubType.GetBandwidth()) != 0 {
								t.Errorf("ERROR lb  Bandwidth want 0, got:=%v", ygot.BinaryToFloat32(lbSubType.GetBandwidth()))
							}
						} else {
							if ygot.BinaryToFloat32(lbSubType.GetBandwidth()) != 2.5e+08 && ygot.BinaryToFloat32(lbSubType.GetBandwidth()) != 2000000000 {
								t.Errorf("ERROR lb Bandwidth want :2G, got=%v", ygot.BinaryToFloat32(lbSubType.GetBandwidth()))
							}
						}
						if deviations.BgpExtendedCommunityIndexUnsupported(td.dut) {
							verifyExtCommunityIndexV6(t, td, v6Prefix)
						}
					}
				}
			}
		}
	}
}
func configureImportRoutingPolicyAllowAll(t *testing.T, dut *ondatra.DUTDevice) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef1 := rp.GetOrCreatePolicyDefinition("allow-all")
	stmt1, err := pdef1.AppendNewStatement("allow-all")
	if err != nil {
		t.Fatalf("AppendNewStatement failed: %v", err)
	}
	stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	// Apply ipv4 policy to bgp neighbour.
	dni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv4).
		AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()

	policy := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).
		GetOrCreateBgp().GetOrCreateNeighbor(atePort1.IPv4).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy()
	policy.SetImportPolicy([]string{"allow-all"})
	gnmi.Replace(t, dut, path.Config(), policy)

	// Apply ipv6 policy to bgp neighbour.
	path = gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().
		Neighbor(atePort1.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()

	policy = root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).
		GetOrCreateBgp().GetOrCreateNeighbor(atePort1.IPv6).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy()
	policy.SetImportPolicy([]string{"allow-all"})
	gnmi.Replace(t, dut, path.Config(), policy)
}

func validateImportRoutingPolicyAllowAll(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	dni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().
		Neighbor(atePort1.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()

	policy := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy](t, dut, path.State())
	importPolicies := policy.GetImportPolicy()
	if len(importPolicies) != 1 {
		t.Fatalf("ImportPolicy Ipv4 = %v, want %v", importPolicies, []string{"allow-all"})
	}
	bgpRIBPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Rib()
	locRib := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv4Unicast_LocRib](t, dut, bgpRIBPath.
		AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().LocRib().State())
	found := 0
	expected := map[string]bool{
		advertisedIPv41.address + "/" + strconv.Itoa(int(advertisedIPv41.prefix)): true,
		advertisedIPv42.address + "/" + strconv.Itoa(int(advertisedIPv42.prefix)): true,
		advertisedIPv43.address + "/" + strconv.Itoa(int(advertisedIPv43.prefix)): true,
	}
	for route, prefix := range locRib.Route {
		if expected[prefix.GetPrefix()] {
			found++
			t.Logf("Found Route(prefix %s, origin: %v, pathid: %d) => %s", route.Prefix, route.Origin, route.PathId, prefix.GetPrefix())
		}
	}
	if found != len(expected) {
		t.Fatalf("Not all V4 routes found. expected:%d got:%d", len(expected), found)
	}

	// Verify ipv6 policy.
	pathV6 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	policyV6 := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy](t, dut, pathV6.State())
	importPolicies = policyV6.GetImportPolicy()
	if len(importPolicies) != 1 {
		t.Errorf("ImportPolicy Ipv6 got= %v, want= %v", importPolicies, []string{"allow-all"})
	}
	bgpRIBPathV6 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Rib()
	locRibv6 := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv6Unicast_LocRib](t, dut, bgpRIBPathV6.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().LocRib().State())
	found = 0
	expectedV6 := map[string]bool{
		v61RouteAdvertise: true,
		v62RouteAdvertise: true,
		v63RouteAdvertise: true,
	}
	for route, prefix := range locRibv6.Route {
		if expectedV6[prefix.GetPrefix()] {
			found++
			t.Logf("Found Route(prefix %s, origin: %v, pathid: %d) => %s", route.Prefix, route.Origin, route.PathId, prefix.GetPrefix())
		}
	}
	if found != len(expectedV6) {
		t.Fatalf("Not all v6 Routes found expected: %d got: %d", len(expectedV6), found)
	}
}

func configureExtCommunityRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	root := &oc.Root{}
	var communitySetCLIConfig string
	var extCommunitySetCLIConfig string
	switch dut.Vendor() {
	case ondatra.CISCO:
		extCommunitySet = extCommunitySetCisco
	default:
		t.Logf("extCommunitySet = %v", extCommunitySet)
	}
	if !deviations.BgpExtendedCommunitySetUnsupported(dut) {
		for name, community := range extCommunitySet {
			rp := root.GetOrCreateRoutingPolicy()
			pdef := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets()
			stmt, err := pdef.NewExtCommunitySet(name)
			if err != nil {
				t.Fatalf("NewExtCommunitySet failed: %v", err)
			}
			stmt.SetExtCommunityMember([]string{community})
			gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
		}
	} else {
		switch dut.Vendor() {
		case ondatra.CISCO:
			for name, community := range extCommunitySet {
				if name == "linkbw_any" && deviations.CommunityMemberRegexUnsupported(dut) {
					communitySetCLIConfig = fmt.Sprintf("community-set %v \n dfa-regex '%v' \n end-set", name, community)
					helpers.GnmiCLIConfig(t, dut, communitySetCLIConfig)
				} else {
					extCommunitySetCLIConfig = fmt.Sprintf("extcommunity-set bandwidth %v \n %v \n end-set", name, community)
					helpers.GnmiCLIConfig(t, dut, extCommunitySetCLIConfig)
				}
			}
		default:
			t.Fatalf("Unsupported vendor %s for native command support for deviation 'BgpExtendedCommunitySetUnsupported'", dut.Vendor())
		}
	}

	if !(deviations.CommunityMemberRegexUnsupported(dut)) {
		for name, community := range CommunitySet {
			rp := root.GetOrCreateRoutingPolicy()
			pdef := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets()
			stmt, err := pdef.NewCommunitySet(name)
			if err != nil {
				t.Fatalf("NewCommunitySet failed: %v", err)
			}
			cs := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
			cs = append(cs, oc.UnionString(community))
			stmt.SetCommunityMember(cs)
			gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
		}
	} else {
		switch dut.Vendor() {
		case ondatra.CISCO:
			for name, community := range CommunitySet {
				communitySetCLIConfig = fmt.Sprintf("community-set %v\n dfa-regex '%v' \n end-set", name, community)
				helpers.GnmiCLIConfig(t, dut, communitySetCLIConfig)
			}
		default:
			t.Fatalf("Unsupported vendor %s for native cmd support for deviation 'CommunityMemberRegexUnsupported'", dut.Vendor())
		}
	}

	// Configure routing Policy not_match_100_set_linkbw_1M.
	rpNotMatch := root.GetOrCreateRoutingPolicy()
	pdef2 := rpNotMatch.GetOrCreatePolicyDefinition("not_match_100_set_linkbw_1M")
	pdef2Stmt1, err := pdef2.AppendNewStatement("1-megabit-match")
	if err != nil {
		t.Fatalf("AppendNewStatement 1-megabit-match failed: %v", err)
	}

	if !deviations.BgpSetExtCommunitySetRefsUnsupported(dut) {
		ref := pdef2Stmt1.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetExtCommunity()
		ref.GetOrCreateReference().SetExtCommunitySetRefs([]string{"linkbw_1M"})
		ref.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
		ref.SetMethod(oc.SetCommunity_Method_REFERENCE)
	}
	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			name1, community1 := "regex_match_comm100_deviation1", "^100:.*$"
			rpDev1 := root.GetOrCreateRoutingPolicy()
			pdefDev1 := rpDev1.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets()
			stmtDev1, err := pdefDev1.NewCommunitySet(name1)
			if err != nil {
				t.Fatalf("NewCommunitySet failed: %v", err)
			}
			cs := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
			cs = append(cs, oc.UnionString(community1))
			stmtDev1.SetCommunityMember(cs)
			stmtDev1.SetMatchSetOptions(oc.BgpPolicy_MatchSetOptionsType_INVERT)
			gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpDev1)
			ref1 := pdef2Stmt1.GetOrCreateConditions().GetOrCreateBgpConditions()
			ref1.SetCommunitySet("regex_match_comm100_deviation1")
		}
	} else {
		ref1 := pdef2Stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
		ref1.SetCommunitySet("regex_match_comm100")
		ref1.SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_INVERT)
	}
	if !deviations.SkipSettingStatementForPolicy(dut) {
		pdef2Stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT)
	}
	pdef2Stmt2, err := pdef2.AppendNewStatement("accept_all_routes")
	if err != nil {
		t.Fatalf("AppendNewStatement accept_all_routes failed: %v", err)
	}
	pdef2Stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	if !deviations.BgpSetExtCommunitySetRefsUnsupported(dut) {
		gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpNotMatch)
	} else {
		switch dut.Vendor() {
		case ondatra.CISCO:
			var communityCLIConfig string
			communityCLIConfig = fmt.Sprintf("community-set %v\n dfa-regex '%v', \n match invert \n end-set", "regex_match_comm100", CommunitySet["regex_match_comm100"])
			policySetCLIConfig := fmt.Sprintf("route-policy %v \n #statement-1 1-megabit-match \n if community in %v then \n set extcommunity bandwidth %v \n endif \n pass \n #statement-2 accept_all_routes \n done \n  end-policy", "not_match_100_set_linkbw_1M", "regex_match_comm100", "linkbw_1M")
			helpers.GnmiCLIConfig(t, dut, communityCLIConfig)
			helpers.GnmiCLIConfig(t, dut, policySetCLIConfig)
		default:
			t.Fatalf("Unsupported vendor %s for native cmd support for deviation 'BgpSetExtCommunitySetRefsUnsupported'", dut.Vendor())
		}
	}

	// Configure routing policy match_100_set_linkbw_2G.
	rpMatch := root.GetOrCreateRoutingPolicy()
	pdef3 := rpMatch.GetOrCreatePolicyDefinition("match_100_set_linkbw_2G")
	pdef3Stmt1, err := pdef3.AppendNewStatement("2-gigabit-match")
	if err != nil {
		t.Fatalf("AppendNewStatement match_100_set_linkbw_2G failed: %v", err)
	}
	if !deviations.BgpSetExtCommunitySetRefsUnsupported(dut) {
		ref := pdef3Stmt1.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetExtCommunity()
		ref.GetOrCreateReference().SetExtCommunitySetRefs([]string{"linkbw_2G"})
		ref.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
		ref.SetMethod(oc.SetCommunity_Method_REFERENCE)
	}
	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			name2, community2 := "regex_match_comm100_deviation2", "^100:.*$"
			rpDev2 := root.GetOrCreateRoutingPolicy()
			pdefDev2 := rpDev2.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets()
			stmtDev2, err := pdefDev2.NewCommunitySet(name2)
			if err != nil {
				t.Fatalf("NewCommunitySet failed: %v", err)
			}
			cs := []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{}
			cs = append(cs, oc.UnionString(community2))
			stmtDev2.SetCommunityMember(cs)
			stmtDev2.SetMatchSetOptions(oc.BgpPolicy_MatchSetOptionsType_ANY)
			gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpDev2)
			ref1 := pdef3Stmt1.GetOrCreateConditions().GetOrCreateBgpConditions()
			ref1.SetCommunitySet("regex_match_comm100_deviation2")
		}
	} else {
		ref1 := pdef3Stmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetMatchCommunitySet()
		ref1.SetCommunitySet("regex_match_comm100")
		ref1.SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)
	}
	if !deviations.SkipSettingStatementForPolicy(dut) {
		pdef3Stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT)
	}
	pdef3Stmt2, err := pdef3.AppendNewStatement("accept_all_routes")
	if err != nil {
		t.Fatalf("AppendNewStatement accept_all_routes failed: %v", err)
	}
	pdef3Stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	if !deviations.BgpSetExtCommunitySetRefsUnsupported(dut) {
		gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpMatch)
	} else {
		switch dut.Vendor() {
		case ondatra.CISCO:
			communitySetCLIConfig = fmt.Sprintf("community-set %v\n dfa-regex '%v', \n match any \n end-set", "regex_match_any_comm100", CommunitySet["regex_match_comm100"])
			helpers.GnmiCLIConfig(t, dut, communitySetCLIConfig)
			communitySetCLIConfig = fmt.Sprintf("route-policy %v \n #statement-1 2-gigabit-match \n if community in %v then \n set extcommunity bandwidth %v \n endif \n pass \n #statement-2 accept_all_routes \n done \n end-policy", "match_100_set_linkbw_2G", "regex_match_any_comm100", "linkbw_2G")
			helpers.GnmiCLIConfig(t, dut, communitySetCLIConfig)
		default:
			t.Fatalf("Unsupported vendor %s for native cmd support for deviation 'BgpSetExtCommunitySetRefsUnsupported' and 'BGPConditionsMatchCommunitySetUnsupported' and 'SkipSettingStatementForPolicy'", dut.Vendor())
		}
	}

	// Configure routing policy del_linkbw.
	rpDelLinkbw := root.GetOrCreateRoutingPolicy()
	pdef4 := rpDelLinkbw.GetOrCreatePolicyDefinition("del_linkbw")
	pdef4Stmt1, err := pdef4.AppendNewStatement("del_linkbw")
	if err != nil {
		t.Fatalf("AppendNewStatement del_linkbw failed: %v", err)
	}
	if !deviations.BgpDeleteLinkBandwidthUnsupported(dut) {
		ref := pdef4Stmt1.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetExtCommunity()
		ref.GetOrCreateReference().SetExtCommunitySetRefs([]string{"linkbw_any"})
		ref.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_REMOVE)
		ref.SetMethod(oc.SetCommunity_Method_REFERENCE)
	}
	if !deviations.SkipSettingStatementForPolicy(dut) {
		pdef4Stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT)
	}
	pdef4Stmt2, err := pdef4.AppendNewStatement("accept_all_routes")
	if err != nil {
		t.Fatalf("AppendNewStatement accept_all_routes failed: %v", err)
	}
	pdef4Stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	if !deviations.BgpDeleteLinkBandwidthUnsupported(dut) {
		gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rpDelLinkbw)
	} else {
		var delLinkbwCLIConfig string
		switch dut.Vendor() {
		case ondatra.CISCO:
			delLinkbwCLIConfig = fmt.Sprintf("route-policy %v\n delete extcommunity bandwidth all\n end-policy", "del_linkbw")
		default:
			t.Fatalf("Unsupported vendor %s for native cmd support for deviation 'BgpDeleteLinkBandwidthUnsupported'", dut.Vendor())
		}
		helpers.GnmiCLIConfig(t, dut, delLinkbwCLIConfig)
	}
}

func createFlow(t *testing.T, td testData, fc flowConfig) {
	td.top.Flows().Clear()
	v4Flow := td.top.Flows().Add().SetName(v4Flow)
	v4Flow.Metrics().SetEnable(true)
	v4Flow.TxRx().Device().
		SetTxNames([]string{fc.src.Name + ".IPv4"}).
		SetRxNames([]string{fc.dstNw})
	v4Flow.Size().SetFixed(512)
	v4Flow.Rate().SetPps(100)
	v4Flow.Duration().Continuous()
	e1 := v4Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fc.src.MAC)
	v4 := v4Flow.Packet().Add().Ipv4()
	v4.Src().SetValue(fc.src.IPv4)
	v4.Dst().Increment().SetStart(fc.dstIP).SetCount(1)

	td.ate.OTG().PushConfig(t, td.top)
	td.ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, td.ate.OTG(), td.top, "IPv4")
}

func createFlowV6(t *testing.T, td testData, fc flowConfig) {
	td.top.Flows().Clear()
	v6Flow := td.top.Flows().Add().SetName(v6Flow)
	v6Flow.Metrics().SetEnable(true)
	v6Flow.TxRx().Device().
		SetTxNames([]string{fc.src.Name + ".IPv6"}).
		SetRxNames([]string{fc.dstNw})
	v6Flow.Size().SetFixed(512)
	v6Flow.Rate().SetPps(100)
	v6Flow.Duration().Continuous()
	e1 := v6Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fc.src.MAC)
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(fc.src.IPv6)
	v6.Dst().Increment().SetStart(fc.dstIP).SetCount(1)
	td.ate.OTG().PushConfig(t, td.top)
	td.ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, td.ate.OTG(), td.top, "IPv6")
}

func checkTraffic(t *testing.T, td testData, flowName string) {
	td.ate.OTG().StartTraffic(t)
	time.Sleep(time.Second * 30)
	td.ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, td.ate.OTG(), td.top)
	otgutils.LogPortMetrics(t, td.ate.OTG(), td.top)
	recvMetric := gnmi.Get(t, td.ate.OTG(), gnmi.OTG().Flow(flowName).State())
	txPackets := recvMetric.GetCounters().GetOutPkts()
	rxPackets := recvMetric.GetCounters().GetInPkts()
	lostPackets := txPackets - rxPackets
	lossPct := lostPackets * 100 / txPackets

	if lossPct > 1 {
		t.Errorf("FAIL in checkTraffic - Got %v%% packet loss for %s ; expected < 1%%", lossPct, flowName)
	}
}

func (td *testData) advertiseRoutesWithEBGP(t *testing.T) {
	t.Helper()

	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(td.dut))
	bgpP := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName)
	bgpP.SetEnabled(true)
	bgp := bgpP.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.SetAs(dutAS)
	g.SetRouterId(dutPort1.IPv4)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)

	nV41 := bgp.GetOrCreateNeighbor(atePort1.IPv4)
	nV41.SetPeerAs(ateAS)
	nV41.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	nV42 := bgp.GetOrCreateNeighbor(atePort2.IPv4)
	nV42.SetPeerAs(dutAS)
	if !deviations.SkipBgpSendCommunityType(td.dut) {
		nV42.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_BOTH})
	}
	nV42.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	nV61 := bgp.GetOrCreateNeighbor(atePort1.IPv6)
	nV61.SetPeerAs(ateAS)
	nV61.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	nV62 := bgp.GetOrCreateNeighbor(atePort2.IPv6)
	nV62.SetPeerAs(dutAS)
	if !deviations.SkipBgpSendCommunityType(td.dut) {
		nV62.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_BOTH})
	}
	nV62.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	gnmi.Update(t, td.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Config(), ni)

	// Configure eBGP on OTG port1.
	ipv41 := td.otgP1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dev1BGP := td.otgP1.Bgp().SetRouterId(atePort1.IPv4)
	bgp4Peer1 := dev1BGP.Ipv4Interfaces().Add().SetIpv4Name(ipv41.Name()).Peers().Add().SetName(td.otgP1.Name() + ".BGP4.peer")
	bgp4Peer1.SetPeerAddress(dutPort1.IPv4).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	ipv61 := td.otgP1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	bgp6Peer1 := dev1BGP.Ipv6Interfaces().Add().SetIpv6Name(ipv61.Name()).Peers().Add().SetName(td.otgP1.Name() + ".BGP6.peer")
	bgp6Peer1.SetPeerAddress(dutPort1.IPv6).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	// Configure emulated network on ATE port1.
	netv41 := bgp4Peer1.V4Routes().Add().SetName("v4-bgpNet-dev1")
	netv41.Addresses().Add().SetAddress(advertisedIPv41.address).SetPrefix(advertisedIPv41.prefix)
	netv61 := bgp6Peer1.V6Routes().Add().SetName("v6-bgpNet-dev1")
	netv61.Addresses().Add().SetAddress(advertisedIPv61.address).SetPrefix(advertisedIPv61.prefix)

	// Configure routes with BGP community.
	netv42 := bgp4Peer1.V4Routes().Add().SetName("v4-bgpNet-dev2")
	netv42.Addresses().Add().SetAddress(advertisedIPv42.address).SetPrefix(advertisedIPv42.prefix)
	commv4 := netv42.Communities().Add()
	commv4.SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	commv4.SetAsNumber(100)
	commv4.SetAsCustom(100)
	netv62 := bgp6Peer1.V6Routes().Add().SetName("v6-bgpNet-dev2")
	netv62.Addresses().Add().SetAddress(advertisedIPv62.address).SetPrefix(advertisedIPv62.prefix)
	commv6 := netv62.Communities().Add()
	commv6.SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER)
	commv6.SetAsNumber(100)
	commv6.SetAsCustom(100)

	// Configure routes with Link bandwidth community.
	netv43 := bgp4Peer1.V4Routes().Add().SetName("v4-bgpNet-dev3")
	netv43.Addresses().Add().SetAddress(advertisedIPv43.address).SetPrefix(advertisedIPv43.prefix)
	extcommv4 := netv43.ExtendedCommunities().Add().NonTransitive2OctetAsType().LinkBandwidthSubtype()
	extcommv4.SetGlobal2ByteAs(23456)
	extcommv4.SetBandwidth(0)
	netv63 := bgp6Peer1.V6Routes().Add().SetName("v6-bgpNet-dev3")
	netv63.Addresses().Add().SetAddress(advertisedIPv63.address).SetPrefix(advertisedIPv63.prefix)
	extcommv6 := netv63.ExtendedCommunities().Add().NonTransitive2OctetAsType().LinkBandwidthSubtype()
	extcommv6.SetGlobal2ByteAs(23456)
	extcommv6.SetBandwidth(0)

	// Configure iBGP on OTG port2.
	ipv42 := td.otgP2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dev2BGP := td.otgP2.Bgp().SetRouterId(atePort2.IPv4)
	bgp4Peer2 := dev2BGP.Ipv4Interfaces().Add().SetIpv4Name(ipv42.Name()).Peers().Add().SetName(td.otgP2.Name() + ".BGP4.peer")
	bgp4Peer2.SetPeerAddress(dutPort2.IPv4).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	bgp4Peer2.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true)
	bgp4Peer2.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	ipv62 := td.otgP2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	bgp6Peer2 := dev2BGP.Ipv6Interfaces().Add().SetIpv6Name(ipv62.Name()).Peers().Add().SetName(td.otgP2.Name() + ".BGP6.peer")
	bgp6Peer2.SetPeerAddress(dutPort2.IPv6).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	bgp6Peer2.Capability().SetIpv4UnicastAddPath(true).SetIpv6UnicastAddPath(true).SetExtendedNextHopEncoding(true)
	bgp6Peer2.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
}

func (td *testData) verifyDUTBGPEstablished(t *testing.T) {
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().NeighborAny().SessionState().State()
	watch := gnmi.WatchAll(t, td.dut, sp, 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		if !ok || state != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
			return false
		}
		return true
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("BGP sessions not established in verifyDUTBGPEstablished : got %v", val)
	}
}

// verifyOTGBGPEstablished verifies on OTG BGP peer establishment.
func (td *testData) verifyOTGBGPEstablished(t *testing.T) {
	sp := gnmi.OTG().BgpPeerAny().SessionState().State()
	watch := gnmi.WatchAll(t, td.ate.OTG(), sp, 2*time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
		state, ok := val.Val()
		if !ok || state != otgtelemetry.BgpPeer_SessionState_ESTABLISHED {
			return false
		}
		return true
	})
	state, ok := watch.Await(t)
	if !ok {
		t.Fatalf("BGP sessions not established : verifyOTGBGPEstablished (%v)", state)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	b := &gnmi.SetBatch{}
	gnmi.BatchReplace(b, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.BatchReplace(b, gnmi.OC().Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
	b.Set(t, dut)
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) []gosnappi.Device {
	t.Helper()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	d1 := atePort1.AddToOTG(top, p1, &dutPort1)
	d2 := atePort2.AddToOTG(top, p2, &dutPort2)
	return []gosnappi.Device{d1, d2}
}

// TODO to move base setup config in helper.
func baseSetupConfigAndVerification(t *testing.T, td testData) {
	td.advertiseRoutesWithEBGP(t)
	td.ate.OTG().PushConfig(t, td.top)
	td.ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, td.ate.OTG(), td.top, "IPv4")
	otgutils.WaitForARP(t, td.ate.OTG(), td.top, "IPv6")
	td.verifyDUTBGPEstablished(t)
	td.verifyOTGBGPEstablished(t)
	configureImportRoutingPolicyAllowAll(t, td.dut)
	validateImportRoutingPolicyAllowAll(t, td.dut, td.ate)
	createFlow(t, td, flowConfig{src: atePort2, dstNw: "v4-bgpNet-dev1", dstIP: v41TrafficStart})
	createFlow(t, td, flowConfig{src: atePort2, dstNw: "v4-bgpNet-dev2", dstIP: v42TrafficStart})
	createFlow(t, td, flowConfig{src: atePort2, dstNw: "v4-bgpNet-dev3", dstIP: v43TrafficStart})
	checkTraffic(t, td, v4Flow)
	createFlowV6(t, td, flowConfig{src: atePort2, dstNw: "v6-bgpNet-dev1", dstIP: v61TrafficStart})
	createFlowV6(t, td, flowConfig{src: atePort2, dstNw: "v6-bgpNet-dev2", dstIP: v62TrafficStart})
	createFlowV6(t, td, flowConfig{src: atePort2, dstNw: "v6-bgpNet-dev3", dstIP: v63TrafficStart})
	checkTraffic(t, td, v6Flow)
}

func verifyExtCommunityIndexV4(t *testing.T, td testData, v4Address string) {
	dni := deviations.DefaultNetworkInstance(td.dut)
	bgpRIBPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Rib()
	locRib := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv4Unicast_LocRib](t, td.dut, bgpRIBPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().LocRib().State())
	t.Logf("RIB: %v", locRib)
	for route, prefix := range locRib.Route {
		if prefix.GetPrefix() != v4Address {
			continue
		}
		t.Logf("Found Route(prefix %s, origin: %v, pathid: %d) => %s", route.Prefix, route.Origin, route.PathId, prefix.GetPrefix())
		if prefix.ExtCommunityIndex == nil {
			t.Fatalf("No V4 community index found")
		}
		extCommunity := bgpRIBPath.ExtCommunity(prefix.GetExtCommunityIndex()).ExtCommunity().State()
		if extCommunity == nil {
			t.Fatalf("No V4 community found at given index: %v", prefix.GetExtCommunityIndex())
		}
	}
}

func verifyExtCommunityIndexV6(t *testing.T, td testData, v6Address string) {
	dni := deviations.DefaultNetworkInstance(td.dut)
	bgpRIBPathV6 := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Rib()
	locRibv6 := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv6Unicast_LocRib](t, td.dut, bgpRIBPathV6.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().LocRib().State())
	t.Logf("RIB: %v", locRibv6)
	for route, prefix := range locRibv6.Route {
		if prefix.GetPrefix() != v6Address {
			continue
		}
		t.Logf("Found Route(prefix %s, origin: %v, pathid: %d) => %s", route.Prefix, route.Origin, route.PathId, prefix.GetPrefix())
		if prefix.ExtCommunityIndex == nil {
			t.Fatalf("No V6 community index found")
		}
		extCommunity := bgpRIBPathV6.ExtCommunity(prefix.GetExtCommunityIndex()).ExtCommunity().State()
		if extCommunity == nil {
			t.Fatalf("No V6 community found at given index: %v", prefix.GetExtCommunityIndex())
		}
	}
}
