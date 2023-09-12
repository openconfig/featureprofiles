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
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	connInternal        = "INTERNAL"
	connExternal        = "EXTERNAL"
	rejectPrefix        = "REJECT-PREFIX"
	communitySet        = "COMM-SET"
	rejectCommunity     = "REJECT-COMMUNITY"
	rejectAspath        = "REJECT-AS-PATH"
	aclStatement1       = "10"
	aclStatement2       = "50"
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
	t.Log("Configure DUT interface")
	dut := ondatra.DUT(t, "dut")
	dc := gnmi.OC()
	i1 := dutSrc.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	t.Log("Configure Network Instance")
	dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	cases := []struct {
		name    string
		nbr     *bgpNbr
		dutConf *oc.NetworkInstance_Protocol
		ateConf *ondatra.ATETopology
	}{
		{
			name:    "Establish eBGP connection between ATE (2-byte) - DUT (4-byte < 65535) for ipv4 peers",
			nbr:     &bgpNbr{localAS: 100, peerIP: ateSrc.IPv4, peerAS: 200, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 100, peerIP: ateSrc.IPv4, peerAS: 200, isV4: true}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 200, peerIP: dutSrc.IPv4, peerAS: 100, isV4: true}, connExternal, prefixV4),
		}, {
			name:    "Establish eBGP connection between ATE (2-byte) - DUT (4-byte < 65535) for ipv6 peers",
			nbr:     &bgpNbr{localAS: 100, peerIP: ateSrc.IPv6, peerAS: 200, isV4: false},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 100, peerIP: ateSrc.IPv6, peerAS: 200, isV4: false}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 200, peerIP: dutSrc.IPv6, peerAS: 100, isV4: false}, connExternal, prefixV6),
		}, {
			name:    "Establish eBGP connection between ATE (4-byte) - DUT (4-byte) for ipv4 peers",
			nbr:     &bgpNbr{localAS: 70000, peerIP: ateSrc.IPv4, peerAS: 80000, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 70000, peerIP: ateSrc.IPv4, peerAS: 80000, isV4: true}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 80000, peerIP: dutSrc.IPv4, peerAS: 70000, isV4: true}, connExternal, prefixV4),
		}, {
			name:    "Establish eBGP connection between ATE (4-byte) - DUT (4-byte) for ipv6 peers",
			nbr:     &bgpNbr{localAS: 70000, peerIP: ateSrc.IPv6, peerAS: 80000, isV4: false},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 70000, peerIP: ateSrc.IPv6, peerAS: 80000, isV4: false}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 80000, peerIP: dutSrc.IPv6, peerAS: 70000, isV4: false}, connExternal, prefixV6),
		}, {
			name:    "Establish iBGP connection between ATE (2-byte) - DUT (4-byte < 65535) for ipv4 peers",
			nbr:     &bgpNbr{localAS: 200, peerIP: ateSrc.IPv4, peerAS: 200, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 200, peerIP: ateSrc.IPv4, peerAS: 200, isV4: true}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 200, peerIP: dutSrc.IPv4, peerAS: 200, isV4: true}, connInternal, prefixV4),
		}, {
			name:    "Establish iBGP connection between ATE (4-byte) - DUT (4-byte < 65535) for ipv6 peers",
			nbr:     &bgpNbr{localAS: 200, peerIP: ateSrc.IPv6, peerAS: 200, isV4: false},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 200, peerIP: ateSrc.IPv6, peerAS: 200, isV4: false}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 200, peerIP: dutSrc.IPv6, peerAS: 200, isV4: false}, connInternal, prefixV6),
		}, {
			name:    "Establish iBGP connection between ATE (4-byte) - DUT (4-byte) for ipv4 peers",
			nbr:     &bgpNbr{localAS: 80000, peerIP: ateSrc.IPv4, peerAS: 80000, isV4: true},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 80000, peerIP: ateSrc.IPv4, peerAS: 80000, isV4: true}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 80000, peerIP: dutSrc.IPv4, peerAS: 80000, isV4: true}, connInternal, prefixV4),
		}, {
			name:    "Establish iBGP connection between ATE (4-byte) - DUT (4-byte) for ipv6 peers",
			nbr:     &bgpNbr{localAS: 80000, peerIP: ateSrc.IPv6, peerAS: 80000, isV4: false},
			dutConf: createBgpNeighbor(&bgpNbr{localAS: 80000, peerIP: ateSrc.IPv6, peerAS: 80000, isV4: false}, dut),
			ateConf: configureATE(t, &bgpNbr{localAS: 80000, peerIP: dutSrc.IPv6, peerAS: 80000, isV4: false}, connInternal, prefixV6),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Log("Clear BGP Configs on DUT")
			bgpClearConfig(t, dut)

			configureRegexPolicy(t, dut)

			d := &oc.Root{}
			rpl := configureBGPPolicy(t, d, tc.nbr.isV4)
			gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rpl)

			t.Log("Configure BGP on DUT")
			gnmi.Replace(t, dut, dutConfPath.Config(), tc.dutConf)

			fptest.LogQuery(t, "DUT BGP Config ", dutConfPath.Config(), gnmi.GetConfig(t, dut, dutConfPath.Config()))
			t.Log("Configure BGP on ATE")
			tc.ateConf.Push(t)
			tc.ateConf.StartProtocols(t)

			t.Log("Verify BGP session state : ESTABLISHED")
			nbrPath := statePath.Neighbor(tc.nbr.peerIP)
			gnmi.Await(t, dut, nbrPath.SessionState().State(), time.Second*60, oc.Bgp_Neighbor_SessionState_ESTABLISHED)

			t.Log("Verify BGP AS numbers and prefix count")
			verifyPeer(t, tc.nbr, dut)

			t.Log("Apply BGP policy for rejecting prefix")
			pol := applyBgpPolicy(rejectPrefix, dut, tc.nbr.isV4)
			gnmi.Update(t, dut, dutConfPath.Config(), pol)
			verifyPrefixesTelemetry(t, dut, 2, tc.nbr.isV4)

			t.Log("Apply BGP policy for rejecting prefix with community filter")
			pol = applyBgpPolicy(rejectCommunity, dut, tc.nbr.isV4)
			gnmi.Update(t, dut, dutConfPath.Config(), pol)
			verifyPrefixesTelemetry(t, dut, 1, tc.nbr.isV4)

			t.Log("Apply BGP policy for rejecting prefix with as-path regex filter")
			pol = applyBgpPolicy(rejectAspath, dut, tc.nbr.isV4)
			gnmi.Update(t, dut, dutConfPath.Config(), pol)
			verifyPrefixesTelemetry(t, dut, 0, tc.nbr.isV4)

			t.Log("Clear BGP Configs on ATE")
			tc.ateConf.StopProtocols(t)
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
func configureBGPPolicy(t *testing.T, d *oc.Root, isV4 bool) *oc.RoutingPolicy {
	t.Helper()
	rp := d.GetOrCreateRoutingPolicy()
	pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(rejectPrefix)

	if isV4 {
		pset.GetOrCreatePrefix(prefixV4[2], prefixSubnetRangeV4)
	} else {
		pset.GetOrCreatePrefix(prefixV6[2], prefixSubnetRangeV6)
	}
	pdef := rp.GetOrCreatePolicyDefinition(rejectPrefix)

	stmt5, err := pdef.AppendNewStatement(aclStatement1)
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	stmt5.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	stmt5.GetOrCreateConditions().GetOrCreateMatchPrefixSet().PrefixSet = ygot.String(rejectPrefix)

	commSet := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(communitySet)
	commSet.CommunitySetName = ygot.String(communitySet)
	var communityMembers []oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union
	for _, member := range community {
		communityMember, _ := commSet.To_RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union(member)
		communityMembers = append(communityMembers, communityMember)
	}
	commSet.SetCommunityMember(communityMembers)
	pdefComm := rp.GetOrCreatePolicyDefinition(rejectCommunity)

	stmt50, err := pdefComm.AppendNewStatement(aclStatement2)
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	stmt50.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	stmt50.GetOrCreateConditions().GetOrCreateBgpConditions().CommunitySet = ygot.String(communitySet)

	return rp
}

func verifyPrefixesTelemetry(t *testing.T, dut *ondatra.DUTDevice, wantInstalled uint32, isV4 bool) {
	t.Helper()
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

	if gotInstalled := gnmi.Get(t, dut, prefixesv4.Installed().State()); gotInstalled != wantInstalled {
		t.Errorf("Installed prefixes mismatch: got %v, want %v", gotInstalled, wantInstalled)
	}
}

// verifyPrefixesTelemetryV6 confirms that the dut shows the correct numbers of installed, sent and
// received IPv6 prefixes
func verifyPrefixesTelemetryV6(t *testing.T, dut *ondatra.DUTDevice, wantInstalledv6 uint32) {
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixesv6 := statePath.Neighbor(ateSrc.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()

	if gotInstalledv6 := gnmi.Get(t, dut, prefixesv6.Installed().State()); gotInstalledv6 != wantInstalledv6 {
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

	verifyPrefixesTelemetry(t, dut, 3, nbr.isV4)
}

func configureATE(t *testing.T, ateParams *bgpNbr, connectionType string, prefixes []string) *ondatra.ATETopology {
	t.Helper()
	t.Log("Configure ATE interface")
	ate := ondatra.ATE(t, "ate")
	port1 := ate.Port(t, "port1")
	topo := ate.Topology().New()

	iDut1 := topo.AddInterface(ateSrc.Name).WithPort(port1)
	iDut1.IPv4().WithAddress(ateSrc.IPv4CIDR()).WithDefaultGateway(dutSrc.IPv4)
	iDut1.IPv6().WithAddress(ateSrc.IPv6CIDR()).WithDefaultGateway(dutSrc.IPv6)

	bgpDut1 := iDut1.BGP()
	peer := bgpDut1.AddPeer().WithPeerAddress(ateParams.peerIP).WithLocalASN(ateParams.localAS)

	if connectionType == connInternal {
		peer.WithTypeInternal()
	} else {
		peer.WithTypeExternal()
	}

	network1 := iDut1.AddNetwork("bgpNeti1")
	network2 := iDut1.AddNetwork("bgpNeti2")
	network3 := iDut1.AddNetwork("bgpNeti3")

	if ateParams.isV4 {
		network1.IPv4().WithAddress(prefixes[0]).WithCount(1)
		network1.BGP().WithNextHopAddress(ateSrc.IPv4).AddASPathSegment(55000, 4400, 3300)

		network2.IPv4().WithAddress(prefixes[1]).WithCount(1)
		network2.BGP().WithNextHopAddress(ateSrc.IPv4).AddASPathSegment(55000, 7700)
		network2.BGP().Communities().WithPrivateCommunities("200:1")

		network3.IPv4().WithAddress(prefixes[2]).WithCount(1)
		network3.BGP().WithNextHopAddress(ateSrc.IPv4)
	} else {
		network1.IPv6().WithAddress(prefixes[0]).WithCount(1)
		network1.BGP().WithNextHopAddress(ateSrc.IPv6).AddASPathSegment(55000, 4400, 3300)

		network2.IPv6().WithAddress(prefixes[1]).WithCount(1)
		network2.BGP().WithNextHopAddress(ateSrc.IPv6).AddASPathSegment(55000, 7700)
		network2.BGP().Communities().WithPrivateCommunities("200:1")

		network3.IPv6().WithAddress(prefixes[2]).WithCount(1)
		network3.BGP().WithNextHopAddress(ateSrc.IPv6)
	}
	return topo
}

func applyBgpPolicy(policyName string, dut *ondatra.DUTDevice, isV4 bool) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	pg := bgp.GetOrCreatePeerGroup("ATE")
	pg.PeerGroupName = ygot.String("ATE")

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		//policy under peer group
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

func createBgpNeighbor(nbr *bgpNbr, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(globalAsNumber)
	global.RouterId = ygot.String(dutSrc.IPv4)

	pg := bgp.GetOrCreatePeerGroup("ATE")
	pg.PeerAs = ygot.Uint32(nbr.peerAS)
	pg.PeerGroupName = ygot.String("ATE")

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
