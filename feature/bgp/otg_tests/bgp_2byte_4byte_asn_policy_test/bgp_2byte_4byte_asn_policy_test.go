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

package bgp_2byte_4byte_asn_with_policy_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	connInternal        = "INTERNAL"
	connExternal        = "EXTERNAL"
	rejectPrefix        = "REJECT-PREFIX"
	communitySet        = "COMM-SET"
	regexAsSet          = "REGEX-AS-SET"
	rejectCommunity     = "REJECT-COMMUNITY"
	rejectAspath        = "REJECT-AS-PATH"
	aclStatement1       = "10"
	aclStatement2       = "20"
	aclStatement3       = "50"
	aclStatement4       = "60"
	aclStatement5       = "70"
	aclStatement6       = "80"
	prefixSubnetRangeV4 = "30..32"
	prefixSubnetRangeV6 = "126..128"
	globalAsNumber      = 999
)

var prefixV4 = []string{"198.51.100.0/30", "198.51.100.4/30", "198.51.100.8/30"}
var prefixV6 = []string{"2001:DB8:1::0/126", "2001:DB8:1::4/126", "2001:DB8:1::8/126"}
var community = []string{"200:1"}

var (
	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: 30,
		IPv6Len: 126,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:11:01:00:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: 30,
		IPv6Len: 126,
	}
)

type bgpNbr struct {
	localAS, peerAS uint32
	peerIP          string
	isV4            bool
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
func TestBgpSession(t *testing.T) {
	t.Log("Clear ATE configuration")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	ate.OTG().PushConfig(t, top)

	t.Log("Configure DUT interface")
	dut := ondatra.DUT(t, "dut")
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	t.Log("Configure Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dut.Port(t, "port1").Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	defaultCount := uint32(0)

	cases := []struct {
		name    string
		nbr     *bgpNbr
		dutConf *oc.NetworkInstance_Protocol
		ateConf gosnappi.Config
		bgpType string
	}{
		{
			name:    "Establish eBGP connection between ATE (2-byte) - DUT (4-byte < 65535) for ipv4 peers",
			nbr:     &bgpNbr{localAS: 100, peerIP: ateSrc.IPv4, peerAS: 200, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 100, peerIP: ateSrc.IPv4, peerAS: 200, isV4: true}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 200, peerIP: dutSrc.IPv4, peerAS: 100, isV4: true}, connExternal, 2),
			bgpType: connExternal,
		}, {
			name:    "Establish eBGP connection between ATE (2-byte) - DUT (4-byte < 65535) for ipv6 peers",
			nbr:     &bgpNbr{localAS: 100, peerIP: ateSrc.IPv6, peerAS: 200, isV4: false},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 100, peerIP: ateSrc.IPv6, peerAS: 200, isV4: false}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 200, peerIP: dutSrc.IPv6, peerAS: 100, isV4: false}, connExternal, 2),
			bgpType: connExternal,
		}, {
			name:    "Establish eBGP connection between ATE (4-byte) - DUT (4-byte) for ipv4 peers",
			nbr:     &bgpNbr{localAS: 70000, peerIP: ateSrc.IPv4, peerAS: 80000, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 70000, peerIP: ateSrc.IPv4, peerAS: 80000, isV4: true}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 80000, peerIP: dutSrc.IPv4, peerAS: 70000, isV4: true}, connExternal, 4),
			bgpType: connExternal,
		}, {
			name:    "Establish eBGP connection between ATE (4-byte) - DUT (4-byte) for ipv6 peers",
			nbr:     &bgpNbr{localAS: 70000, peerIP: ateSrc.IPv6, peerAS: 80000, isV4: false},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 70000, peerIP: ateSrc.IPv6, peerAS: 80000, isV4: false}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 80000, peerIP: dutSrc.IPv6, peerAS: 70000, isV4: false}, connExternal, 4),
			bgpType: connExternal,
		}, {
			name:    "Establish iBGP connection between ATE (2-byte) - DUT (4-byte < 65535) for ipv4 peers",
			nbr:     &bgpNbr{localAS: 200, peerIP: ateSrc.IPv4, peerAS: 200, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 200, peerIP: ateSrc.IPv4, peerAS: 200, isV4: true}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 200, peerIP: dutSrc.IPv4, peerAS: 200, isV4: true}, connInternal, 2),
			bgpType: connInternal,
		}, {
			name:    "Establish iBGP connection between ATE (4-byte) - DUT (4-byte < 65535) for ipv6 peers",
			nbr:     &bgpNbr{localAS: 200, peerIP: ateSrc.IPv6, peerAS: 200, isV4: false},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 200, peerIP: ateSrc.IPv6, peerAS: 200, isV4: false}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 200, peerIP: dutSrc.IPv6, peerAS: 200, isV4: false}, connInternal, 4),
			bgpType: connInternal,
		}, {
			name:    "Establish iBGP connection between ATE (4-byte) - DUT (4-byte) for ipv4 peers",
			nbr:     &bgpNbr{localAS: 80000, peerIP: ateSrc.IPv4, peerAS: 80000, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 80000, peerIP: ateSrc.IPv4, peerAS: 80000, isV4: true}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 80000, peerIP: dutSrc.IPv4, peerAS: 80000, isV4: true}, connInternal, 4),
			bgpType: connInternal,
		}, {
			name:    "Establish iBGP connection between ATE (4-byte) - DUT (4-byte) for ipv6 peers",
			nbr:     &bgpNbr{localAS: 80000, peerIP: ateSrc.IPv6, peerAS: 80000, isV4: false},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 80000, peerIP: ateSrc.IPv6, peerAS: 80000, isV4: false}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 80000, peerIP: dutSrc.IPv6, peerAS: 80000, isV4: false}, connInternal, 4),
			bgpType: connInternal,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			otg := ondatra.ATE(t, "ate").OTG()
			t.Log("Clear BGP Configs on DUT")
			bgpClearConfig(t, dut)

			if tc.bgpType == connExternal {
				defaultCount = uint32(0)
			} else {
				defaultCount = uint32(3)
			}
			configureRegexPolicy(t, dut)

			d := &oc.Root{}
			rpl := configureBGPPolicy(t, d, tc.nbr.isV4, dut)
			gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rpl)

			t.Log("Configure BGP on DUT")
			gnmi.Replace(t, dut, dutConfPath.Config(), tc.dutConf)

			fptest.LogQuery(t, "DUT BGP Config ", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))
			t.Log("Configure BGP on ATE")
			otg.PushConfig(t, tc.ateConf)
			otg.StartProtocols(t)

			t.Log("Verify BGP session state : ESTABLISHED")
			nbrPath := statePath.Neighbor(tc.nbr.peerIP)
			gnmi.Await(t, dut, nbrPath.SessionState().State(), time.Second*120, oc.Bgp_Neighbor_SessionState_ESTABLISHED)

			t.Log("Verify BGP AS numbers and prefix count")
			verifyPeer(t, tc.nbr, dut)
			verifyPrefixesTelemetry(t, dut, defaultCount, tc.nbr.isV4)

			// Reject Prefix
			t.Log("Apply BGP policy for rejecting prefix")
			pol := applyBgpPolicy(rejectPrefix, dut, tc.nbr.isV4)
			gnmi.Update(t, dut, dutConfPath.Config(), pol)
			t.Logf("Policy applied, waiting for DUT to apply the policy")
			awaitBGPPolicy(t, dut, tc.nbr, rejectPrefix, 30*time.Second)
			verifyPrefixesTelemetry(t, dut, 2, tc.nbr.isV4)
			deleteBgpPolicy(t, dut, tc.nbr.isV4)
			t.Logf("Policy deleted, waiting for DUT to delete the policy ")
			awaitBGPPolicy(t, dut, tc.nbr, rejectPrefix, 30*time.Second)
			verifyPrefixesTelemetry(t, dut, defaultCount, tc.nbr.isV4)

			// Reject Community
			t.Log("Apply BGP policy for rejecting prefix with community filter")
			pol = applyBgpPolicy(rejectCommunity, dut, tc.nbr.isV4)
			gnmi.Update(t, dut, dutConfPath.Config(), pol)
			t.Logf("Policy applied, waiting for DUT to apply the policy")
			awaitBGPPolicy(t, dut, tc.nbr, rejectCommunity, 30*time.Second)
			verifyPrefixesTelemetry(t, dut, 2, tc.nbr.isV4)
			deleteBgpPolicy(t, dut, tc.nbr.isV4)
			t.Logf("Policy deleted, waiting for DUT to delete the policy ")
			awaitBGPPolicy(t, dut, tc.nbr, rejectPrefix, 30*time.Second)
			verifyPrefixesTelemetry(t, dut, defaultCount, tc.nbr.isV4)

			// Reject ASPath
			if !deviations.MatchAsPathSetUnsupported(dut) {
				t.Log("Apply BGP policy for rejecting prefix with as-path regex filter")
				pol = applyBgpPolicy(rejectAspath, dut, tc.nbr.isV4)
				gnmi.Update(t, dut, dutConfPath.Config(), pol)
				t.Logf("Policy applied, waiting for DUT to apply the policy")
				awaitBGPPolicy(t, dut, tc.nbr, rejectAspath, 30*time.Second)
				verifyPrefixesTelemetry(t, dut, 2, tc.nbr.isV4)
			}
			t.Log("Clear BGP Configs on ATE")
			otg.StopProtocols(t)
		})
	}
}

// Build config with Origin set to cli and Ascii encoded config.
func buildCliConfigRequest(config string) (*gpb.SetRequest, error) {
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cli",
				Elem:   []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: config,
				},
			},
		}},
	}
	return gpbSetRequest, nil
}

// juniperCLI returns Juniper CLI config statement.
func juniperCLI() string {
	return fmt.Sprintf(`
	policy-options {
		policy-statement %s {
			term term1 {
				from as-path match-as-path;
				then reject;
			}
			term term2 {
				then accept;
			}
		}
		as-path match-as-path ".* 4400 3300";
	}`, rejectAspath)
}

// configureRegexPolicy is used to configure vendor specific config statement.
func configureRegexPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var config string
	gnmiClient := dut.RawAPIs().GNMI(t)

	switch dut.Vendor() {
	case ondatra.JUNIPER:
		config = juniperCLI()
		t.Logf("Push the CLI config:%s", dut.Vendor())
	default:
		t.Logf("Push no CLI config:%s", dut.Vendor())
		return
	}

	gpbSetRequest, err := buildCliConfigRequest(config)
	if err != nil {
		t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
	}

	if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
}

// configureBGPPolicy configures a BGP routing policy to accept or reject routes based on prefix match conditions
// Additonally, it also configures policy to match prefix based on community and regex for as path
func configureBGPPolicy(t *testing.T, d *oc.Root, isV4 bool, dut *ondatra.DUTDevice) *oc.RoutingPolicy {
	t.Helper()
	rp := d.GetOrCreateRoutingPolicy()
	pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(rejectPrefix)

	if isV4 {
		pset.GetOrCreatePrefix(prefixV4[2], prefixSubnetRangeV4)
	} else {
		pset.GetOrCreatePrefix(prefixV6[2], prefixSubnetRangeV6)
	}
	pdef := rp.GetOrCreatePolicyDefinition(rejectPrefix)

	stmt10, err := pdef.AppendNewStatement(aclStatement1)
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	stmt10.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	stmt10.GetOrCreateConditions().GetOrCreateMatchPrefixSet().PrefixSet = ygot.String(rejectPrefix)

	stmt20, err := pdef.AppendNewStatement(aclStatement2)
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	stmt20.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	commSet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySet)
	commSet.CommunitySetName = ygot.String(communitySet)
	var communityMembers []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union
	for _, member := range community {
		communityMember, _ := commSet.To_RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union(member)
		communityMembers = append(communityMembers, communityMember)
	}
	commSet.SetCommunityMember(communityMembers)
	pdefComm := rp.GetOrCreatePolicyDefinition(rejectCommunity)

	stmt50, err := pdefComm.AppendNewStatement(aclStatement3)
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	stmt50.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	if deviations.BGPConditionsMatchCommunitySetUnsupported(dut) {
		stmt50.GetOrCreateConditions().GetOrCreateBgpConditions().SetCommunitySet(communitySet)
	} else {
		stmt50.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet().SetCommunitySet(communitySet)
	}

	stmt60, err := pdefComm.AppendNewStatement(aclStatement4)
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	stmt60.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	// Deviation to omit AS-Path Set policy configuration
	if !deviations.MatchAsPathSetUnsupported(dut) {
		rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateAsPathSet(regexAsSet).SetAsPathSetMember([]string{".* 4400 3300"})
		pdefAs := rp.GetOrCreatePolicyDefinition(rejectAspath)

		stmt70, err := pdefAs.AppendNewStatement(aclStatement5)
		if err != nil {
			t.Errorf("Error while creating new statement %v", err)
		}
		stmt70.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
		stmt70.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchAsPathSet().SetAsPathSet(regexAsSet)
		stmt70.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchAsPathSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsType_ANY)

		stmt80, err := pdefAs.AppendNewStatement(aclStatement6)
		if err != nil {
			t.Errorf("Error while creating new statement %v", err)
		}
		stmt80.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	}
	return rp
}

func awaitBGPPolicy(t *testing.T, dut *ondatra.DUTDevice, nbr *bgpNbr, policyName string, timeout time.Duration) {
	t.Helper()
	afiSafiType := oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
	if nbr.isV4 {
		afiSafiType = oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
	}
	// importPolicyPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(nbr.peerIP).AfiSafi(afiSafiType).ApplyPolicy().ImportPolicy().State()
	importPolicyPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup("ATE").AfiSafi(afiSafiType).ApplyPolicy().ImportPolicy().State()
	_, ok := gnmi.Watch(t, dut, importPolicyPath, timeout, func(val *ygnmi.Value[[]string]) bool {
		policies, present := val.Val()
		return present && len(policies) > 0 && policies[0] == policyName
	}).Await(t)
	if !ok {
		t.Logf("Policy %s not installed on peer-group ATE", policyName)
	} else {
		t.Logf("Policy %s installed on peer-group ATE", policyName)
	}
}

func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, wantInstalled uint32, isV4 bool) {
	t.Helper()
	t.Logf("Verify BGP prefix count")
	if isV4 {
		verifyPrefixesTelemetryV4(t, dut, wantInstalled)
	} else {
		verifyPrefixesTelemetryV6(t, dut, wantInstalled)
	}
}

// verifyPrefixesTelemetry confirms that the dut shows the correct numbers of installed, sent and
// received IPv4 prefixes
func verifyPrefixesTelemetryV4(t *testing.T, dut *ondatra.DUTDevice, wantInstalled uint32) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv4 := statePath.Neighbor(ateSrc.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()

	gotInstalled, ok := gnmi.Watch(t, dut, prefixesv4.Installed().State(), 120*time.Second, func(val *ygnmi.Value[uint32]) bool {
		gotInstalled, _ := val.Val()
		return gotInstalled == wantInstalled
	}).Await(t)

	if !ok {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}
}

// verifyPrefixesTelemetryV6 confirms that the dut shows the correct numbers of installed, sent and
// received IPv6 prefixes
func verifyPrefixesTelemetryV6(t *testing.T, dut *ondatra.DUTDevice, wantInstalledv6 uint32) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv6 := statePath.Neighbor(ateSrc.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()

	gotInstalledv6, ok := gnmi.Watch(t, dut, prefixesv6.Installed().State(), 120*time.Second, func(val *ygnmi.Value[uint32]) bool {
		gotInstalledv6, _ := val.Val()
		return gotInstalledv6 == wantInstalledv6
	}).Await(t)

	if !ok {
		t.Errorf("IPV6 Installed prefixes mismatch: got %v, want %v", gotInstalledv6, wantInstalledv6)
	}
}

// bgpClearConfig removes all BGP configuration from the DUT.
func bgpClearConfig(t *testing.T, dut *ondatra.DUTDevice) {
	resetBatch := &gnmi.SetBatch{}
	gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config())

	if deviations.NetworkInstanceTableDeletionRequired(dut) {
		tablePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableAny()
		for _, table := range gnmi.LookupAll(t, dut, tablePath.Config()) {
			if val, ok := table.Val(); ok {
				if val.GetProtocol() == oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP {
					gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Table(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, val.GetAddressFamily()).Config())
				}
			}
		}
	}
	resetBatch.Set(t, dut)
}

func verifyPeer(t *testing.T, nbr *bgpNbr, dut *ondatra.DUTDevice) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(nbr.peerIP)
	glblPath := statePath.Global()

	// Check BGP peerAS from telemetry.
	peerAS := gnmi.Get(t, dut, nbrPath.State()).GetPeerAs()
	if peerAS != nbr.peerAS {
		t.Errorf("BGP peerAs: got %v, want %v", peerAS, nbr.peerAS)
	}

	// Check BGP localAS from telemetry.
	localAS := gnmi.Get(t, dut, nbrPath.State()).GetLocalAs()
	if localAS != nbr.localAS {
		t.Errorf("BGP localAS: got %v, want %v", localAS, nbr.localAS)
	}

	// Check BGP globalAS from telemetry.
	globalAS := gnmi.Get(t, dut, glblPath.State()).GetAs()
	if globalAS != globalAsNumber {
		t.Errorf("BGP globalAS: got %v, want %v", globalAS, nbr.localAS)
	}

}

func configureATE(t *testing.T, ateParams *bgpNbr, connectionType string, asWidth int) gosnappi.Config {
	t.Helper()
	t.Log("Create otg configuration")
	ate := ondatra.ATE(t, "ate")
	port1 := ate.Port(t, "port1")
	config := gosnappi.NewConfig()

	config.Ports().Add().SetName(port1.ID())
	srcDev := config.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	srcEth.Connection().SetPortName(port1.ID())
	srcIpv4 := srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	srcIpv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	srcIpv6 := srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6")
	srcIpv6.SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	srcBgp := srcDev.Bgp().SetRouterId(srcIpv4.Address())
	if ateParams.isV4 {
		srcBgpPeer := srcBgp.Ipv4Interfaces().Add().SetIpv4Name(srcIpv4.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP4.peer")
		srcBgpPeer.SetPeerAddress(ateParams.peerIP).SetAsNumber(ateParams.localAS)
		if connectionType == connInternal {
			srcBgpPeer.SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
		} else {
			srcBgpPeer.SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
		}
		if asWidth == 2 {
			srcBgpPeer.SetAsNumberWidth(gosnappi.BgpV4PeerAsNumberWidth.TWO)
		}
		subnetAddr1, subnetLen1 := prefixAndLen(prefixV4[0])
		subnetAddr2, subnetLen2 := prefixAndLen(prefixV4[1])
		subnetAddr3, subnetLen3 := prefixAndLen(prefixV4[2])

		network1 := srcBgpPeer.V4Routes().Add().SetName("bgpNeti1")
		network1.SetNextHopIpv4Address(ateSrc.IPv4).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		network1.Addresses().Add().SetAddress(subnetAddr1).SetPrefix(subnetLen1).SetCount(1)
		network1.AsPath().Segments().Add().SetAsNumbers([]uint32{55000, 4400, 3300})

		network2 := srcBgpPeer.V4Routes().Add().SetName("bgpNeti2")
		network2.SetNextHopIpv4Address(ateSrc.IPv4).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		network2.Addresses().Add().SetAddress(subnetAddr2).SetPrefix(subnetLen2).SetCount(1)
		network2.AsPath().Segments().Add().SetAsNumbers([]uint32{55000, 7700})
		network2.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER).SetAsNumber(200).SetAsCustom(1)

		network3 := srcBgpPeer.V4Routes().Add().SetName("bgpNeti3")
		network3.SetNextHopIpv4Address(ateSrc.IPv4).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		network3.Addresses().Add().SetAddress(subnetAddr3).SetPrefix(subnetLen3).SetCount(1)

	} else {
		srcBgpPeer := srcBgp.Ipv6Interfaces().Add().SetIpv6Name(srcIpv6.Name()).Peers().Add().SetName(ateSrc.Name + ".BGP6.peer")
		srcBgpPeer.SetPeerAddress(ateParams.peerIP).SetAsNumber(ateParams.localAS)
		if connectionType == connInternal {
			srcBgpPeer.SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
		} else {
			srcBgpPeer.SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
		}
		if asWidth == 2 {
			srcBgpPeer.SetAsNumberWidth(gosnappi.BgpV6PeerAsNumberWidth.TWO)
		}
		prefixArr1 := strings.Split(prefixV6[0], "/")
		mask1, _ := strconv.Atoi(prefixArr1[1])
		prefixArr2 := strings.Split(prefixV6[1], "/")
		mask2, _ := strconv.Atoi(prefixArr2[1])
		prefixArr3 := strings.Split(prefixV6[2], "/")
		mask3, _ := strconv.Atoi(prefixArr3[1])

		network1 := srcBgpPeer.V6Routes().Add().SetName("bgpNeti1")
		network1.SetNextHopIpv6Address(ateSrc.IPv6).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
			SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		network1.Addresses().Add().SetAddress(prefixArr1[0]).SetPrefix(uint32(mask1)).SetCount(1)
		network1.AsPath().Segments().Add().SetAsNumbers([]uint32{55000, 4400, 3300})

		network2 := srcBgpPeer.V6Routes().Add().SetName("bgpNeti2")
		network2.SetNextHopIpv6Address(ateSrc.IPv6).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
			SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		network2.Addresses().Add().SetAddress(prefixArr2[0]).SetPrefix(uint32(mask2)).SetCount(1)
		network2.AsPath().Segments().Add().SetAsNumbers([]uint32{55000, 7700})
		network2.Communities().Add().SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER).SetAsNumber(200).SetAsCustom(1)

		network3 := srcBgpPeer.V6Routes().Add().SetName("bgpNeti3")
		network3.SetNextHopIpv6Address(ateSrc.IPv6).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
			SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		network3.Addresses().Add().SetAddress(prefixArr3[0]).SetPrefix(uint32(mask3)).SetCount(1)

	}

	return config
}

func applyBgpPolicy(policyName string, dut *ondatra.DUTDevice, isV4 bool) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	pg := bgp.GetOrCreatePeerGroup("ATE")
	pg.PeerGroupName = ygot.String("ATE")

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		// policy under peer group
		pg.GetOrCreateApplyPolicy().ImportPolicy = []string{policyName}
		return niProto
	}

	aftType := oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
	if isV4 {
		aftType = oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
	}

	afisafi := pg.GetOrCreateAfiSafi(aftType)
	afisafi.Enabled = ygot.Bool(true)
	rpl := afisafi.GetOrCreateApplyPolicy()
	rpl.SetImportPolicy([]string{policyName})

	return niProto
}

func deleteBgpPolicy(t *testing.T, dut *ondatra.DUTDevice, isV4 bool) {
	t.Helper()
	t.Logf("Delete BGP policy on DUT")
	aftType := oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST
	if isV4 {
		aftType = oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST
	}

	policyConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup("ATE").AfiSafi(aftType).ApplyPolicy().Config()
	gnmi.Delete(t, dut, policyConfPath)
}

func createBgpNeighbor(nbr *bgpNbr, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(globalAsNumber)
	global.RouterId = ygot.String(dutSrc.IPv4)

	bgpGlobalIPv4AF := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	bgpGlobalIPv4AF.SetEnabled(true)

	bgpGlobalIPv6AF := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	bgpGlobalIPv6AF.SetEnabled(true)

	pg := bgp.GetOrCreatePeerGroup("ATE")
	pg.PeerAs = ygot.Uint32(nbr.peerAS)
	pg.PeerGroupName = ygot.String("ATE")
	if nbr.peerAS != nbr.localAS {
		if deviations.BgpDefaultPolicyBehaviorAcceptRoute(dut) {
			pg.GetOrCreateApplyPolicy().SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
		}
	}

	neighbor := bgp.GetOrCreateNeighbor(nbr.peerIP)
	neighbor.PeerAs = ygot.Uint32(nbr.peerAS)
	neighbor.LocalAs = ygot.Uint32(nbr.localAS)
	neighbor.Enabled = ygot.Bool(true)
	neighbor.PeerGroup = ygot.String("ATE")
	neighbor.GetOrCreateTimers().RestartTime = ygot.Uint16(75)

	if nbr.isV4 {
		afisafi := neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		afisafi.Enabled = ygot.Bool(true)
		neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(false)
	} else {
		afisafi6 := neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		afisafi6.Enabled = ygot.Bool(true)
		neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(false)
	}
	return niProto
}

func prefixAndLen(prefix string) (string, uint32) {
	subnetAddr := strings.Split(prefix, "/")[0]
	len, _ := strconv.Atoi(strings.Split(prefix, "/")[1])
	subnetLen := uint32(len)
	return subnetAddr, subnetLen
}
