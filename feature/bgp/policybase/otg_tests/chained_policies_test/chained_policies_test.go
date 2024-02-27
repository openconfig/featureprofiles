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

package chained_policies_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// go through OKRs

const (
	ipv4PrefixLen     = 30
	ipv6PrefixLen     = 126
	v41Route          = "203.0.113.0"
	v41TrafficStart   = "203.0.113.1"
	v42Route          = "198.51.100.0"
	v42TrafficStart   = "198.51.100.1"
	v4RoutePrefix     = uint32(24)
	v61Route          = "2001:db8:128:128::0"
	v61TrafficStart   = "2001:db8:128:128::1"
	v62Route          = "2001:db8:128:129::0"
	v62TrafficStart   = "2001:db8:128:129::1"
	v6RoutePrefix     = uint32(64)
	dutAS             = uint32(65656)
	ateAS             = uint32(65657)
	bgpName           = "BGP"
	maskLenExact      = "exact"
	localPref         = 200
	med               = 1000
	v4Flow            = "flow-v4"
	v4PrefixPolicy    = "prefix-policy-v4"
	v4PrefixStatement = "prefix-statement-v4"
	v4PrefixSet       = "prefix-set-v4"
	v4LPPolicy        = "lp-policy-v4"
	v4LPStatement     = "lp-statement-v4"
	v4ASPPolicy       = "asp-policy-v4"
	v4ASPStatement    = "asp-statement-v4"
	v4MedPolicy       = "med-policy-v4"
	v4MedStatement    = "med-statement-v4"
	v6Flow            = "flow-v6"
	v6PrefixPolicy    = "prefix-policy-v6"
	v6PrefixStatement = "prefix-statement-v6"
	v6PrefixSet       = "prefix-set-v6"
	v6LPPolicy        = "lp-policy-v6"
	v6LPStatement     = "lp-statement-v6"
	v6ASPPolicy       = "asp-policy-v6"
	v6ASPStatement    = "asp-statement-v6"
	v6MedPolicy       = "med-policy-v6"
	v6MedStatement    = "med-statement-v6"
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
	advertisedIPv61 = ipAddr{address: v61Route, prefix: v6RoutePrefix}
	advertisedIPv62 = ipAddr{address: v62Route, prefix: v6RoutePrefix}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type ipAddr struct {
	address string
	prefix  uint32
}

func (ip *ipAddr) cidr(t *testing.T) string {
	_, net, err := net.ParseCIDR(fmt.Sprintf("%s/%d", ip.address, ip.prefix))
	if err != nil {
		t.Fatal(err)
	}
	return net.String()
}

type testData struct {
	dut   *ondatra.DUTDevice
	ate   *ondatra.ATEDevice
	top   gosnappi.Config
	otgP1 gosnappi.Device
	otgP2 gosnappi.Device
}

type testCase struct {
	name        string
	desc        string
	applyPolicy func(t *testing.T, dut *ondatra.DUTDevice)
	validate    func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice)
	ipv4        bool
	flowConfig  flowConfig
}

type flowConfig struct {
	src   attrs.Attributes
	dstNw string
	dstIP string
}

func TestBGPChainedPolicies(t *testing.T) {
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
	td.advertiseRoutesWithEBGP(t)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	defer ate.OTG().StopProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
	td.verifyDUTBGPEstablished(t)
	td.verifyOTGBGPEstablished(t)

	testCases := []testCase{
		{
			name:        "IPv4BGPChainedImportPolicy",
			desc:        "IPv4 BGP chained import policy test",
			applyPolicy: configureImportRoutingPolicy,
			validate:    validateImportRoutingPolicy,
			ipv4:        true,
			flowConfig:  flowConfig{src: atePort2, dstNw: "v4-bgpNet-dev1", dstIP: v41TrafficStart},
		},
		{
			name:        "IPv4BGPChainedExportPolicy",
			desc:        "IPv4 BGP chained export policy test",
			applyPolicy: configureExportRoutingPolicy,
			validate:    validateExportRoutingPolicy,
			ipv4:        true,
			flowConfig:  flowConfig{src: atePort1, dstNw: "v4-bgpNet-dev2", dstIP: v42TrafficStart},
		},
		{
			name:        "IPv6BGPChainedImportPolicy",
			desc:        "IPv6 BGP chained import policy test",
			applyPolicy: configureImportRoutingPolicyV6,
			validate:    validateImportRoutingPolicyV6,
			ipv4:        false,
			flowConfig:  flowConfig{src: atePort2, dstNw: "v6-bgpNet-dev1", dstIP: v61TrafficStart},
		},
		{
			name:        "IPv6BGPChainedExportPolicy",
			desc:        "IPv6 BGP chained export policy test",
			applyPolicy: configureExportRoutingPolicyV6,
			validate:    validateExportRoutingPolicyV6,
			ipv4:        false,
			flowConfig:  flowConfig{src: atePort1, dstNw: "v6-bgpNet-dev2", dstIP: v62TrafficStart},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.desc)
			tc.applyPolicy(t, dut)
			tc.validate(t, dut, ate)
			if tc.ipv4 {
				createFlow(t, td, tc.flowConfig)
				checkTraffic(t, td, v4Flow)
			} else {
				createFlowV6(t, td, tc.flowConfig)
				checkTraffic(t, td, v6Flow)
			}
		})
	}
}

func configureImportRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef1 := rp.GetOrCreatePolicyDefinition(v4PrefixPolicy)
	stmt1, err := pdef1.AppendNewStatement(v4PrefixStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v4PrefixStatement, err)
	}
	stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	prefixSet := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v4PrefixSet)
	prefixSet.SetMode(oc.PrefixSet_Mode_IPV4)
	prefixSet.GetOrCreatePrefix(advertisedIPv41.cidr(t), maskLenExact)

	if !deviations.SkipSetRpMatchSetOptions(dut) {
		stmt1.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	}
	stmt1.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(v4PrefixSet)

	pdef2 := rp.GetOrCreatePolicyDefinition(v4LPPolicy)
	stmt2, err := pdef2.AppendNewStatement(v4LPStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v4LPStatement, err)
	}
	stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	stmt2.GetOrCreateActions().GetOrCreateBgpActions().SetSetLocalPref(localPref)
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	dni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	policy := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(atePort1.IPv4).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy()
	policy.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	policy.SetImportPolicy([]string{v4PrefixPolicy, v4LPPolicy})
	gnmi.Replace(t, dut, path.Config(), policy)
}

func validateImportRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	dni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	policy := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy](t, dut, path.State())
	importPolicies := policy.GetImportPolicy()
	if len(importPolicies) != 2 {
		t.Errorf("ImportPolicy = %v, want %v", importPolicies, []string{v4PrefixPolicy, v4LPPolicy})
	}

	bgpRIBPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Rib()
	locRib := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv4Unicast_LocRib](t, dut, bgpRIBPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().LocRib().State())
	found := false
	for k, lr := range locRib.Route {
		if lr.GetPrefix() == advertisedIPv41.address {
			found = true
			t.Logf("Found Route(prefix %s, origin: %v, pathid: %d) => %s", k.Prefix, k.Origin, k.PathId, lr.GetPrefix())
			attrSet := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet](t, dut, bgpRIBPath.AttrSet(lr.GetAttrIndex()).State())
			if attrSet == nil || attrSet.GetLocalPref() != localPref {
				t.Errorf("No local pref found for prefix %s", advertisedIPv41.address)
			}
			break
		}
	}

	if !found {
		t.Errorf("No Route found for prefix %s", advertisedIPv41.address)
	}
}

func configureExportRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef1 := rp.GetOrCreatePolicyDefinition(v4ASPPolicy)
	stmt1, err := pdef1.AppendNewStatement(v4ASPStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v4ASPStatement, err)
	}
	stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	stmt1.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend().SetAsn(dutAS)

	pdef2 := rp.GetOrCreatePolicyDefinition(v4MedPolicy)
	stmt2, err := pdef2.AppendNewStatement(v4MedStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v4MedStatement, err)
	}
	stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	stmt2.GetOrCreateActions().GetOrCreateBgpActions().SetSetMed(oc.UnionUint32(med))
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	dni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	policy := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(atePort1.IPv4).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy()
	policy.SetDefaultExportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	policy.SetExportPolicy([]string{v4ASPPolicy, v4MedPolicy})
	gnmi.Replace(t, dut, path.Config(), policy)
}

func validateExportRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	dni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
	policy := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy](t, dut, path.State())
	exportPolicies := policy.GetExportPolicy()
	if len(exportPolicies) != 2 {
		t.Errorf("ExportPolicy = %v, want %v", exportPolicies, []string{v4PrefixPolicy, v4LPPolicy})
	}

	bgpPrefixes := gnmi.GetAll[*otgtelemetry.BgpPeer_UnicastIpv4Prefix](t, ate.OTG(), gnmi.OTG().BgpPeer("v4-bgpNet-dev2").UnicastIpv4PrefixAny().State())
	found := false
	for _, bgpPrefix := range bgpPrefixes {
		if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == v42Route &&
			bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == v4RoutePrefix {
			found = true
			t.Logf("Prefix recevied on OTG is correct, got prefix %v, want prefix %v", bgpPrefix, v42Route)
			if bgpPrefix.GetMultiExitDiscriminator() != med {
				t.Errorf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), med)
			}
			asPaths := bgpPrefix.AsPath
			for _, ap := range asPaths {
				count := 0
				for _, an := range ap.AsNumbers {
					if an == dutAS {
						count++
					}
				}
				if count == 2 {
					t.Logf("ASP for prefix %v is correct, got ASP %v", bgpPrefix.GetAddress(), ap.AsNumbers)
				}
			}
			break
		}
	}

	if !found {
		t.Errorf("No Route found for prefix %s", v42Route)
	}
}

func configureImportRoutingPolicyV6(t *testing.T, dut *ondatra.DUTDevice) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef1 := rp.GetOrCreatePolicyDefinition(v6PrefixPolicy)
	stmt1, err := pdef1.AppendNewStatement(v6PrefixStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v6PrefixStatement, err)
	}
	stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	prefixSet := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v6PrefixSet)
	prefixSet.SetMode(oc.PrefixSet_Mode_IPV6)
	prefixSet.GetOrCreatePrefix(advertisedIPv61.cidr(t), maskLenExact)

	if !deviations.SkipSetRpMatchSetOptions(dut) {
		stmt1.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	}
	stmt1.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(v6PrefixSet)

	pdef2 := rp.GetOrCreatePolicyDefinition(v6LPPolicy)
	stmt2, err := pdef2.AppendNewStatement(v6LPStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v6LPStatement, err)
	}
	stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	stmt2.GetOrCreateActions().GetOrCreateBgpActions().SetSetLocalPref(localPref)
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	dni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	policy := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(atePort1.IPv6).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy()
	policy.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	policy.SetImportPolicy([]string{v6PrefixPolicy, v6LPPolicy})
	gnmi.Replace(t, dut, path.Config(), policy)
}

func validateImportRoutingPolicyV6(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	dni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	policy := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy](t, dut, path.State())
	importPolicies := policy.GetImportPolicy()
	if len(importPolicies) != 2 {
		t.Errorf("ImportPolicy = %v, want %v", importPolicies, []string{v6PrefixPolicy, v6LPPolicy})
	}

	bgpRIBPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Rib()
	locRib := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv6Unicast_LocRib](t, dut, bgpRIBPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().LocRib().State())
	found := false
	for k, lr := range locRib.Route {
		if lr.GetPrefix() == advertisedIPv61.address {
			found = true
			t.Logf("Found Route(prefix %s, origin: %v, pathid: %d) => %s", k.Prefix, k.Origin, k.PathId, lr.GetPrefix())
			attrSet := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet](t, dut, bgpRIBPath.AttrSet(lr.GetAttrIndex()).State())
			if attrSet == nil || attrSet.GetLocalPref() != localPref {
				t.Errorf("No local pref found for prefix %s", advertisedIPv61.address)
			}
			break
		}
	}

	if !found {
		t.Errorf("No Route found for prefix %s", advertisedIPv61.address)
	}
}

func configureExportRoutingPolicyV6(t *testing.T, dut *ondatra.DUTDevice) {
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	pdef1 := rp.GetOrCreatePolicyDefinition(v6ASPPolicy)
	stmt1, err := pdef1.AppendNewStatement(v6ASPStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v6ASPStatement, err)
	}
	stmt1.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	stmt1.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend().SetAsn(dutAS)

	pdef2 := rp.GetOrCreatePolicyDefinition(v6MedPolicy)
	stmt2, err := pdef2.AppendNewStatement(v6MedStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v6MedStatement, err)
	}
	stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	stmt2.GetOrCreateActions().GetOrCreateBgpActions().SetSetMed(oc.UnionUint32(med))
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	dni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	policy := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(atePort1.IPv6).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy()
	policy.SetDefaultExportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	policy.SetExportPolicy([]string{v6ASPPolicy, v6MedPolicy})
	gnmi.Replace(t, dut, path.Config(), policy)
}

func validateExportRoutingPolicyV6(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	dni := deviations.DefaultNetworkInstance(dut)
	path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
	policy := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi_ApplyPolicy](t, dut, path.State())
	exportPolicies := policy.GetExportPolicy()
	if len(exportPolicies) != 2 {
		t.Errorf("ExportPolicy = %v, want %v", exportPolicies, []string{v6PrefixPolicy, v6LPPolicy})
	}

	bgpPrefixes := gnmi.GetAll[*otgtelemetry.BgpPeer_UnicastIpv6Prefix](t, ate.OTG(), gnmi.OTG().BgpPeer("v6-bgpNet-dev2").UnicastIpv6PrefixAny().State())
	found := false
	for _, bgpPrefix := range bgpPrefixes {
		if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == v62Route &&
			bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == v6RoutePrefix {
			found = true
			t.Logf("Prefix recevied on OTG is correct, got prefix %v, want prefix %v", bgpPrefix, v62Route)
			if bgpPrefix.GetMultiExitDiscriminator() != med {
				t.Errorf("For Prefix %v, got MED %d want MED %d", bgpPrefix.GetAddress(), bgpPrefix.GetMultiExitDiscriminator(), med)
			}
			asPaths := bgpPrefix.AsPath
			for _, ap := range asPaths {
				count := 0
				for _, an := range ap.AsNumbers {
					if an == dutAS {
						count++
					}
				}
				if count == 2 {
					t.Logf("ASP for prefix %v is correct, got ASP %v", bgpPrefix.GetAddress(), ap.AsNumbers)
				}
			}
			break
		}
	}

	if !found {
		t.Errorf("No Route found for prefix %s", v62Route)
	}
}

func createFlow(t *testing.T, td testData, fc flowConfig) {
	td.top.Flows().Clear()

	t.Log("Configuring v4 traffic flow")
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

	t.Log("Configuring v6 traffic flow")
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

	t.Log("Checking flow telemetry...")
	recvMetric := gnmi.Get(t, td.ate.OTG(), gnmi.OTG().Flow(flowName).State())
	txPackets := recvMetric.GetCounters().GetOutPkts()
	rxPackets := recvMetric.GetCounters().GetInPkts()
	lostPackets := txPackets - rxPackets
	lossPct := lostPackets * 100 / txPackets

	if lossPct > 1 {
		t.Errorf("FAIL- Got %v%% packet loss for %s ; expected < 1%%", lossPct, flowName)
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
	nV42.SetPeerAs(ateAS)
	nV42.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	nV61 := bgp.GetOrCreateNeighbor(atePort1.IPv6)
	nV61.SetPeerAs(ateAS)
	nV61.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	nV62 := bgp.GetOrCreateNeighbor(atePort2.IPv6)
	nV62.SetPeerAs(ateAS)
	nV62.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	gnmi.Update(t, td.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Config(), ni)

	// configure eBGP on OTG port1
	ipv41 := td.otgP1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dev1BGP := td.otgP1.Bgp().SetRouterId(atePort1.IPv4)
	bgp4Peer1 := dev1BGP.Ipv4Interfaces().Add().SetIpv4Name(ipv41.Name()).Peers().Add().SetName(td.otgP1.Name() + ".BGP4.peer")
	bgp4Peer1.SetPeerAddress(dutPort1.IPv4).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	ipv61 := td.otgP1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	bgp6Peer1 := dev1BGP.Ipv6Interfaces().Add().SetIpv6Name(ipv61.Name()).Peers().Add().SetName(td.otgP1.Name() + ".BGP6.peer")
	bgp6Peer1.SetPeerAddress(dutPort1.IPv6).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	// configure emulated network on ATE port1
	netv41 := bgp4Peer1.V4Routes().Add().SetName("v4-bgpNet-dev1")
	netv41.Addresses().Add().SetAddress(advertisedIPv41.address).SetPrefix(advertisedIPv41.prefix)
	netv61 := bgp6Peer1.V6Routes().Add().SetName("v6-bgpNet-dev1")
	netv61.Addresses().Add().SetAddress(advertisedIPv61.address).SetPrefix(advertisedIPv61.prefix)

	// configure eBGP on OTG port2
	ipv42 := td.otgP2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dev2BGP := td.otgP2.Bgp().SetRouterId(atePort2.IPv4)
	bgp4Peer2 := dev2BGP.Ipv4Interfaces().Add().SetIpv4Name(ipv42.Name()).Peers().Add().SetName(td.otgP2.Name() + ".BGP4.peer")
	bgp4Peer2.SetPeerAddress(dutPort2.IPv4).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	ipv62 := td.otgP2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	bgp6Peer2 := dev2BGP.Ipv6Interfaces().Add().SetIpv6Name(ipv62.Name()).Peers().Add().SetName(td.otgP2.Name() + ".BGP6.peer")
	bgp6Peer2.SetPeerAddress(dutPort2.IPv6).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)

	// configure emulated network on ATE port2
	netv42 := bgp4Peer2.V4Routes().Add().SetName("v4-bgpNet-dev2")
	netv42.Addresses().Add().SetAddress(advertisedIPv42.address).SetPrefix(advertisedIPv42.prefix)
	netv62 := bgp6Peer2.V6Routes().Add().SetName("v6-bgpNet-dev2")
	netv62.Addresses().Add().SetAddress(advertisedIPv62.address).SetPrefix(advertisedIPv62.prefix)
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
		t.Fatalf("BGP sessions not established: got %v", val)
	}
	t.Log("DUT BGP sessions established")
}

// VerifyOTGBGPEstablished verifies on OTG BGP peer establishment
func (td *testData) verifyOTGBGPEstablished(t *testing.T) {
	sp := gnmi.OTG().BgpPeerAny().SessionState().State()
	watch := gnmi.WatchAll(t, td.ate.OTG(), sp, 2*time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BgpPeer_SessionState]) bool {
		state, ok := val.Val()
		if !ok || state != otgtelemetry.BgpPeer_SessionState_ESTABLISHED {
			return false
		}
		return true
	})
	if val, ok := watch.Await(t); !ok {
		t.Fatalf("BGP sessions not established: got %v", val)
	}
	t.Log("OTG BGP sessions established")
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
