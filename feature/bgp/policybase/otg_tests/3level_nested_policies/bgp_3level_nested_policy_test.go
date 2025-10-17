package bgp_3level_nested_policy_test

import (
	"fmt"
	"net"
	"strconv"
	"strings"
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

const (
	dutAS                  = uint32(65501)
	ateAS1                 = uint32(65502)
	ateAS2                 = uint32(65503)
	bgpName                = "BGP"
	asPathRepeat           = 15
	plenV4                 = 30
	plenV6                 = 126
	v41Route               = "192.168.10.0"
	v41TrafficStart        = "192.168.10.1"
	v4RoutePrefix          = uint32(24)
	v61Route               = "2024:db8:128:128::"
	v61TrafficStart        = "2024:db8:128:128::1"
	v6RoutePrefix          = uint32(64)
	v42Route               = "192.168.20.0"
	v42TrafficStart        = "192.168.20.1"
	v62Route               = "2024:db8:64:64::"
	v62TrafficStart        = "2024:db8:64:64::1"
	v4PrefixSetIp          = "10.0.0.0/8"
	v6PrefixSetIp          = "::/0"
	maskLenExact           = "exact"
	localPref              = 200
	med                    = 1000
	community              = "64512:100"
	bgpCoomunityVal        = 64512
	trafficPps             = 100
	totalPackets           = 1000
	trafficFrameSize       = 512
	tolerance              = 5
	trafficDuration        = 20
	v4Flow                 = "flow-v4"
	v4PrefixSet            = "prefix-set-v4"
	v4PrefixEportSet       = "prefix-export-set-v4"
	v6PrefixEportSet       = "prefix-export-set-v6"
	v4LPPolicy             = "lp-policy-v4"
	v4LPStatement          = "lp-statement-v4"
	v4ASPPolicy            = "asp-policy-v4"
	v4ASPStatement         = "asp-statement-v4"
	v4MedPolicy            = "med-policy-v4"
	v4MedStatement         = "med-statement-v4"
	v4MatchExportPolicy    = "match-export-policy-v4"
	v6MatchExportPolicy    = "match-export-policy-v6"
	v4MatchStatement       = "match-statement-v4"
	v4CommunityPolicy      = "community-policy-v4"
	v4CommunityStatement   = "community-statement-v4"
	v4CommunitySet         = "community-set-v4"
	v6Flow                 = "flow-v6"
	v6PrefixSet            = "prefix-set-v6"
	v6LPPolicy             = "lp-policy-v6"
	v6LPStatement          = "lp-statement-v6"
	v6ASPPolicy            = "asp-policy-v6"
	v6ASPStatement         = "asp-statement-v6"
	v6MedPolicy            = "med-policy-v6"
	v6MedStatement         = "med-statement-v6"
	v6MatchStatement       = "match-statement-v6"
	v6CommunityPolicy      = "community-policy-v6"
	v6CommunityStatement   = "community-statement-v6"
	v6CommunitySet         = "community-set-v6"
	peerGrpNamev4          = "BGP-PEER-GROUP-V4"
	peerGrpNamev6          = "BGP-PEER-GROUP-V6"
	permitAll              = "PERMIT-ALL"
	permitAllStmtName      = "20"
	v4MatchImportPolicy    = "match-import-policy-v4"
	v4MatchImportStatement = "match-statement-v4"
	v4InvertPolicy         = "invert-policy-v4"
	v4InvertStatement      = "invert-statement-v4"
	v4InvertPrefixSet      = "prefix-set-v4"
	v6MatchImportPolicy    = "match-import-policy-v6"
	v6MatchImportStatement = "match-statement-v6"
	v6InvertPolicy         = "invert-policy-v6"
	v6InvertStatement      = "invert-statement-v6"
	v6InvertPrefixSet      = "prefix-set-v6"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plenV4,
		IPv6Len: plenV6,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenV4,
		IPv6Len: plenV6,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE Port2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plenV4,
		IPv6Len: plenV6,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: plenV4,
		IPv6Len: plenV6,
	}
	advertisedIPv41 = Prefix{address: v41Route, prefix: v4RoutePrefix}
	advertisedIPv42 = Prefix{address: v42Route, prefix: v4RoutePrefix}
	advertisedIPv61 = Prefix{address: v61Route, prefix: v6RoutePrefix}
	advertisedIPv62 = Prefix{address: v62Route, prefix: v6RoutePrefix}
)

type Prefix struct {
	address string
	prefix  uint32
}

// TestMain is the entry point for the test suite.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func (ip *Prefix) cidr(t *testing.T) string {
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
	src      attrs.Attributes
	srcPort  string
	dstPort  []string
	dstMac   string
	dstIP    string
	flowType string
	flowName string
}

func TestBgp3LevelNestedPolicy(t *testing.T) {
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
	port1Mac := gnmi.Get(t, td.dut, gnmi.OC().Interface(td.dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	port2Mac := gnmi.Get(t, td.dut, gnmi.OC().Interface(td.dut.Port(t, "port2").Name()).Ethernet().MacAddress().State())

	testCases := []testCase{
		{
			name:        "IPv4BGPNestedImportPolicy",
			desc:        "RT-1.31.1: IPv4 BGP 3 levels of nested import policy with match-prefix-set conditions",
			applyPolicy: configureImportRoutingPolicyV4,
			validate:    validateImportRoutingPolicyV4,
			ipv4:        true,
			flowConfig:  flowConfig{src: atePort2, srcPort: td.top.Ports().Items()[1].Name(), dstPort: []string{td.top.Ports().Items()[0].Name()}, dstMac: port2Mac, dstIP: v41TrafficStart, flowType: "ipv4", flowName: v4Flow},
		},
		{
			name:        "IPv4BGPNestedExportPolicy",
			desc:        "RT-1.31.2: IPv4 BGP 3 levels of nested export policy with match-prefix-set conditions",
			applyPolicy: configureExportRoutingPolicyV4,
			validate:    validateExportRoutingPolicyV4,
			ipv4:        true,
			flowConfig:  flowConfig{src: atePort1, srcPort: td.top.Ports().Items()[0].Name(), dstPort: []string{td.top.Ports().Items()[1].Name()}, dstMac: port1Mac, dstIP: v42TrafficStart, flowType: "ipv4", flowName: v4Flow},
		},
		{
			name:        "IPv6BGPNestedImportPolicy",
			desc:        "RT-1.31.3: IPv6 BGP 3 levels of nested import policy with match-prefix-set conditions",
			applyPolicy: configureImportRoutingPolicyV6,
			validate:    validateImportRoutingPolicyV6,
			ipv4:        false,
			flowConfig:  flowConfig{src: atePort2, srcPort: td.top.Ports().Items()[1].Name(), dstPort: []string{td.top.Ports().Items()[0].Name()}, dstMac: port2Mac, dstIP: v61TrafficStart, flowType: "ipv6", flowName: v6Flow},
		},
		{
			name:        "IPv6BGPNestedExportPolicy",
			desc:        "RT-1.31.4: IPv6 BGP 3 levels of nested export policy with match-prefix-set conditions",
			applyPolicy: configureExportRoutingPolicyV6,
			validate:    validateExportRoutingPolicyV6,
			ipv4:        false,
			flowConfig:  flowConfig{src: atePort1, srcPort: td.top.Ports().Items()[0].Name(), dstPort: []string{td.top.Ports().Items()[1].Name()}, dstMac: port1Mac, dstIP: v62TrafficStart, flowType: "ipv6", flowName: v6Flow},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.desc)
			tc.applyPolicy(t, dut)
			tc.validate(t, dut, ate)
			if tc.ipv4 {
				createFlow(t, td, tc.flowConfig)
				td.verifyDUTBGPEstablished(t)
				td.verifyOTGBGPEstablished(t)
				checkTraffic(t, td, v4Flow)
			} else {
				createFlow(t, td, tc.flowConfig)
				td.verifyDUTBGPEstablished(t)
				td.verifyOTGBGPEstablished(t)
				checkTraffic(t, td, v6Flow)
			}
		})
	}
}

// configureImportRoutingPolicyV4 configures the dut for IPv4 BGP nested import policy test.
func configureImportRoutingPolicyV4(t *testing.T, dut *ondatra.DUTDevice) {
	batch := &gnmi.SetBatch{}
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()

	// Configure PERMIT-ALL policy
	pdef := rp.GetOrCreatePolicyDefinition(permitAll)
	stmt, err := pdef.AppendNewStatement(permitAllStmtName)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", permitAllStmtName, err)
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	// match-import-policy-v4 (ANY)
	pdefMatch := rp.GetOrCreatePolicyDefinition(v4MatchImportPolicy)
	stmtMatch, err := pdefMatch.AppendNewStatement(v4MatchImportStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v4MatchImportStatement, err)
	}
	stmtMatch.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	stmtMatch.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(v4PrefixSet)
	stmtMatch.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT

	// lp-policy-v4 (set local-pref)
	pdefLP := rp.GetOrCreatePolicyDefinition(v4LPPolicy)
	stmtLP, err := pdefLP.AppendNewStatement(v4LPStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v4LPStatement, err)
	}
	stmtLP.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmtLP.GetOrCreateActions().GetOrCreateBgpActions().SetSetLocalPref(localPref)

	// community-policy-v4 (set community)
	pdefComm := rp.GetOrCreatePolicyDefinition(v4CommunityPolicy)
	stmtComm, err := pdefComm.AppendNewStatement(v4CommunityStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v4CommunityStatement, err)
	}
	stmtComm.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	commActions := stmtComm.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetCommunity()
	commActions.SetMethod(oc.SetCommunity_Method_REFERENCE)
	commActions.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
	commActions.GetOrCreateReference().SetCommunitySetRefs([]string{v4CommunitySet})

	pdInvert := rp.GetOrCreatePolicyDefinition(v4InvertPolicy)
	stmtInvert, _ := pdInvert.AppendNewStatement(v4InvertStatement)
	stmtInvert.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	stmtInvert.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(v4InvertPrefixSet)
	stmtInvert.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmtInvert.GetOrCreateConditions().SetCallPolicy(v4MatchImportPolicy)

	stmtMatch.GetOrCreateConditions().SetCallPolicy(v4LPPolicy)
	stmtLP.GetOrCreateConditions().SetCallPolicy(v4CommunityPolicy)

	// Install prefix-set-v4 (IPv4 /24)
	ps := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v4PrefixSet)
	ps.SetMode(oc.PrefixSet_Mode_IPV4)
	ps.GetOrCreatePrefix(advertisedIPv41.cidr(t), maskLenExact)

	// Install invert prefix-set
	psInv := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v4InvertPrefixSet)
	psInv.SetMode(oc.PrefixSet_Mode_IPV4)
	psInv.GetOrCreatePrefix(v4PrefixSetIp, maskLenExact)

	// Install community-set-v4
	cs := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(v4CommunitySet)
	cs.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString(community)})

	// Push the entire routing-policy subtree
	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)

	// Attach invert-policy-v4 to neighbor import
	dni := deviations.DefaultNetworkInstance(dut)
	importPath := gnmi.OC().NetworkInstance(dni).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).
		Bgp().Neighbor(atePort1.IPv4).
		AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).
		ApplyPolicy().Config()
	gnmi.BatchDelete(batch, importPath)
	apply := root.GetOrCreateNetworkInstance(dni).
		GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).
		GetOrCreateBgp().GetOrCreateNeighbor(atePort1.IPv4).
		GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).
		GetOrCreateApplyPolicy()
	apply.SetImportPolicy([]string{v4InvertPolicy})
	// Set default import policy to reject
	apply.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	gnmi.BatchReplace(batch, importPath, apply)

	batch.Set(t, dut)
}

func validateImportRoutingPolicyV4(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	validateCommunityValue(t, dut, v4CommunitySet)
	if !deviations.BGPRibOcPathUnsupported(dut) {
		dni := deviations.DefaultNetworkInstance(dut)
		bgpRIBPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Rib()
		locRib := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv4Unicast_LocRib](t, dut, bgpRIBPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().LocRib().State())
		found := false
		for k, lr := range locRib.Route {
			prefixAddr := strings.Split(lr.GetPrefix(), "/")
			t.Logf("Route: %v, lr.GetPrefix() -> %v, advertisedIPv41.address: %s, prefixAddr[0]: %s", k, lr.GetPrefix(), advertisedIPv41.address, prefixAddr[0])
			if prefixAddr[0] == advertisedIPv41.address {
				found = true
				t.Logf("Found Route(prefix %s, origin: %v, pathid: %d) => %s", k.Prefix, k.Origin, k.PathId, lr.GetPrefix())
				if !deviations.SkipCheckingAttributeIndex(dut) {
					attrSet := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet](t, dut, bgpRIBPath.AttrSet(lr.GetAttrIndex()).State())
					if attrSet == nil || attrSet.GetLocalPref() != localPref {
						t.Errorf("No local pref found for prefix %s", advertisedIPv41.address)
					}
					break
				} else {
					attrSetList := gnmi.GetAll[*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet](t, dut, bgpRIBPath.AttrSetAny().State())
					foundLP := false
					for _, attrSet := range attrSetList {
						if attrSet.GetLocalPref() == localPref {
							foundLP = true
							t.Logf("Found local pref %d for prefix %s", attrSet.GetLocalPref(), advertisedIPv41.address)
							break
						}
					}
					if !foundLP {
						t.Errorf("No local pref found for prefix %s", advertisedIPv41.address)
					}
				}
			}
		}
		if !found {
			t.Errorf("No Route found for prefix %s", advertisedIPv41.address)
		}
	} else {
		//TODO: Else will remove once fixed deviation.
		t.Logf("Currently does not has RIB support, not able to validate ImportRoutingPolicy")
	}
}

// configureExportRoutingPolicy configures the dut for IPv4 BGP nested export policy test.
func configureExportRoutingPolicyV4(t *testing.T, dut *ondatra.DUTDevice) {
	batch := &gnmi.SetBatch{}
	root := &oc.Root{}
	gnmi.BatchDelete(batch, gnmi.OC().RoutingPolicy().Config())
	rp := root.GetOrCreateRoutingPolicy()

	pdef := rp.GetOrCreatePolicyDefinition(permitAll)
	stmt, err := pdef.AppendNewStatement(permitAllStmtName)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", permitAllStmtName, err)
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	//match‑export‑policy‑v4: ANY match on prefix‑set-export-v4
	psExportV4 := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v4PrefixEportSet)
	psExportV4.SetMode(oc.PrefixSet_Mode_IPV4)
	psExportV4.GetOrCreatePrefix(advertisedIPv42.cidr(t), maskLenExact)

	pdMatch := rp.GetOrCreatePolicyDefinition(v4MatchExportPolicy)
	stMatch, err := pdMatch.AppendNewStatement(v4MatchStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v4MatchStatement, err)
	}
	cond := stMatch.GetOrCreateConditions().GetOrCreateMatchPrefixSet()
	cond.SetPrefixSet(v4PrefixEportSet)
	cond.SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	stMatch.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT)

	pdAsp := rp.GetOrCreatePolicyDefinition(v4ASPPolicy)
	st, _ := pdAsp.AppendNewStatement(v4ASPStatement)
	st.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend().SetRepeatN(asPathRepeat)
	st.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT)
	// med‑policy‑v4: set MED to 1000
	pdef2 := rp.GetOrCreatePolicyDefinition(v4MedPolicy)
	stmt2, err := pdef2.AppendNewStatement(v4MedStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v4MedStatement, err)
	}

	stmt2.GetOrCreateActions().GetOrCreateBgpActions().SetSetMed(oc.UnionUint32(med))
	stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	if !deviations.BGPSetMedActionUnsupported(dut) {
		stmt2.GetOrCreateActions().GetOrCreateBgpActions().SetSetMedAction(oc.BgpPolicy_BgpSetMedAction_SET)
	}
	stMatch.GetOrCreateConditions().SetCallPolicy(v4ASPPolicy)
	statPath := rp.GetOrCreatePolicyDefinition(v4ASPPolicy).GetStatement(v4ASPStatement).GetOrCreateConditions()
	statPath.SetCallPolicy(v4MedPolicy)

	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)

	dni := deviations.DefaultNetworkInstance(dut)

	// Configure the parent BGP import policy.
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		//policy under peer group
		path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().PeerGroup(peerGrpNamev4).ApplyPolicy()
		gnmi.BatchDelete(batch, path.Config())
		policy := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreatePeerGroup(peerGrpNamev4).GetOrCreateApplyPolicy()
		policy.SetExportPolicy([]string{v4MatchExportPolicy})
		gnmi.BatchReplace(batch, path.Config(), policy)
	} else {
		path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).ApplyPolicy()
		gnmi.BatchDelete(batch, path.Config())
		policy := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(atePort1.IPv4).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy()
		policy.SetExportPolicy([]string{v4MatchExportPolicy})
		if !deviations.RoutingPolicyChainingUnsupported(dut) {
			gnmi.BatchUpdate(batch, path.Config(), policy)
		} else {
			gnmi.BatchReplace(batch, path.Config(), policy)
		}
	}
	batch.Set(t, dut)
}

func validateExportRoutingPolicyV4(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	t.Logf("Validating Export Routing Policy, waiting for 30 seconds")
	time.Sleep(time.Second * 30)
	bgpPrefixes := gnmi.GetAll[*otgtelemetry.BgpPeer_UnicastIpv4Prefix](t, ate.OTG(), gnmi.OTG().BgpPeer("atePort1.BGP4.peer").UnicastIpv4PrefixAny().State())
	found := false
	for _, bgpPrefix := range bgpPrefixes {
		if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == v42Route &&
			bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == v4RoutePrefix {
			found = true
			t.Logf("Prefix recevied on OTG is correct, got prefix %v, want prefix %v", bgpPrefix.GetAddress(), v42Route)
			t.Logf("Prefix MED %d", bgpPrefix.GetMultiExitDiscriminator())
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
				if (count - 1) != asPathRepeat {
					t.Errorf("Expected %d prepends, got %d", asPathRepeat, (count - 1))
				} else {
					t.Logf("AS-PATH has DUT ASN %d occurrences", count) // 1 original + 15 prepended = 16 total
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
	batch := &gnmi.SetBatch{}
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()

	// Configure PERMIT-ALL policy
	pdef := rp.GetOrCreatePolicyDefinition(permitAll)
	stmt, err := pdef.AppendNewStatement(permitAllStmtName)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", permitAllStmtName, err)
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	// match-import-policy-v6 (ANY)
	pdefMatch := rp.GetOrCreatePolicyDefinition(v6MatchImportPolicy)
	stmtMatch, err := pdefMatch.AppendNewStatement(v6MatchImportStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v6MatchImportStatement, err)
	}
	stmtMatch.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	stmtMatch.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(v6PrefixSet)
	stmtMatch.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT

	// lp-policy-v6 (set local-pref)
	pdefLP := rp.GetOrCreatePolicyDefinition(v6LPPolicy)
	stmtLP, err := pdefLP.AppendNewStatement(v6LPStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v6LPStatement, err)
	}
	stmtLP.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmtLP.GetOrCreateActions().GetOrCreateBgpActions().SetSetLocalPref(localPref)

	// community-policy-v6 (set community)
	pdefComm := rp.GetOrCreatePolicyDefinition(v6CommunityPolicy)
	stmtComm, err := pdefComm.AppendNewStatement(v6CommunityStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v6CommunityStatement, err)
	}
	stmtComm.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	commActions := stmtComm.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetCommunity()
	commActions.SetMethod(oc.SetCommunity_Method_REFERENCE)
	commActions.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
	commActions.GetOrCreateReference().SetCommunitySetRefs([]string{v6CommunitySet})

	pdInvert := rp.GetOrCreatePolicyDefinition(v6InvertPolicy)
	stmtInvert, _ := pdInvert.AppendNewStatement(v6InvertStatement)
	stmtInvert.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	stmtInvert.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(v6InvertPrefixSet)
	stmtInvert.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT
	stmtInvert.GetOrCreateConditions().SetCallPolicy(v6MatchImportPolicy)

	stmtMatch.GetOrCreateConditions().SetCallPolicy(v6LPPolicy)
	stmtLP.GetOrCreateConditions().SetCallPolicy(v6CommunityPolicy)

	// Install prefix-set-v6
	ps := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v6PrefixSet)
	ps.SetMode(oc.PrefixSet_Mode_IPV6)
	ps.GetOrCreatePrefix(advertisedIPv61.cidr(t), maskLenExact)

	// Install invert prefix-set
	psInv := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v6InvertPrefixSet)
	psInv.SetMode(oc.PrefixSet_Mode_IPV6)
	psInv.GetOrCreatePrefix(v6PrefixSetIp, maskLenExact)

	// Install community-set-v6
	cs := rp.GetOrCreateDefinedSets().GetOrCreateBgpDefinedSets().GetOrCreateCommunitySet(v6CommunitySet)
	cs.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{oc.UnionString(community)})

	// Push the entire routing-policy subtree
	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)

	// Attach invert-policy-v6 to neighbor import
	dni := deviations.DefaultNetworkInstance(dut)
	importPath := gnmi.OC().NetworkInstance(dni).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).
		Bgp().Neighbor(atePort1.IPv6).
		AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).
		ApplyPolicy().Config()
	gnmi.BatchDelete(batch, importPath)
	apply := root.GetOrCreateNetworkInstance(dni).
		GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).
		GetOrCreateBgp().GetOrCreateNeighbor(atePort1.IPv6).
		GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).
		GetOrCreateApplyPolicy()
	apply.SetImportPolicy([]string{v6InvertPolicy})
	// Set default import policy to reject
	apply.SetDefaultImportPolicy(oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE)
	gnmi.BatchReplace(batch, importPath, apply)

	batch.Set(t, dut)
}

func validateImportRoutingPolicyV6(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	validateCommunityValue(t, dut, v6CommunitySet)
	if !deviations.BGPRibOcPathUnsupported(dut) {
		dni := deviations.DefaultNetworkInstance(dut)
		bgpRIBPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Rib()
		locRib := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AfiSafi_Ipv6Unicast_LocRib](t, dut, bgpRIBPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().LocRib().State())
		found := false
		for k, lr := range locRib.Route {
			prefixAddr := strings.Split(lr.GetPrefix(), "/")
			t.Logf("Route: %v, lr.GetPrefix() -> %v, advertisedIPv61.address: %s, prefixAddr[0]: %s", k, lr.GetPrefix(), advertisedIPv61.address, prefixAddr[0])
			if prefixAddr[0] == advertisedIPv61.address {
				found = true
				t.Logf("Found Route(prefix %s, origin: %v, pathid: %d) => %s", k.Prefix, k.Origin, k.PathId, lr.GetPrefix())
				if !deviations.SkipCheckingAttributeIndex(dut) {
					attrSet := gnmi.Get[*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet](t, dut, bgpRIBPath.AttrSet(lr.GetAttrIndex()).State())
					if attrSet == nil || attrSet.GetLocalPref() != localPref {
						t.Errorf("No local pref found for prefix %s", advertisedIPv61.address)
					}
					break
				} else {
					attrSetList := gnmi.GetAll[*oc.NetworkInstance_Protocol_Bgp_Rib_AttrSet](t, dut, bgpRIBPath.AttrSetAny().State())
					foundLP := false
					for _, attrSet := range attrSetList {
						if attrSet.GetLocalPref() == localPref {
							foundLP = true
							t.Logf("Found local pref %d for prefix %s", attrSet.GetLocalPref(), advertisedIPv61.address)
							break
						}
					}
					if !foundLP {
						t.Errorf("No local pref found for prefix %s", advertisedIPv41.address)
					}
				}
			}
		}
		if !found {
			t.Errorf("No Route found for prefix %s", advertisedIPv61.address)
		}
	} else {
		//TODO: Else will remove once fixed deviation.
		t.Logf("Currently does not has RIB support, not able to validate ImportRoutingPolicy")
	}
}

func configureExportRoutingPolicyV6(t *testing.T, dut *ondatra.DUTDevice) {
	batch := &gnmi.SetBatch{}
	root := &oc.Root{}
	gnmi.BatchDelete(batch, gnmi.OC().RoutingPolicy().Config())
	rp := root.GetOrCreateRoutingPolicy()

	pdef := rp.GetOrCreatePolicyDefinition(permitAll)
	stmt, err := pdef.AppendNewStatement(permitAllStmtName)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", permitAllStmtName, err)
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	//match‑export‑policy‑v6: ANY match on prefix‑set-export-v6
	psExportV6 := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(v6PrefixEportSet)
	psExportV6.SetMode(oc.PrefixSet_Mode_IPV6)
	psExportV6.GetOrCreatePrefix(advertisedIPv62.cidr(t), maskLenExact)

	pdMatch := rp.GetOrCreatePolicyDefinition(v6MatchExportPolicy)
	stMatch, err := pdMatch.AppendNewStatement(v6MatchStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v6MatchStatement, err)
	}
	cond := stMatch.GetOrCreateConditions().GetOrCreateMatchPrefixSet()
	cond.SetPrefixSet(v6PrefixEportSet)
	cond.SetMatchSetOptions(oc.RoutingPolicy_MatchSetOptionsRestrictedType_ANY)
	stMatch.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT)

	pdAsp := rp.GetOrCreatePolicyDefinition(v6ASPPolicy)
	st, _ := pdAsp.AppendNewStatement(v6ASPStatement)
	st.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend().SetRepeatN(asPathRepeat)
	st.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_NEXT_STATEMENT)
	// med‑policy‑v6: set MED to 1000
	pdef2 := rp.GetOrCreatePolicyDefinition(v6MedPolicy)
	stmt2, err := pdef2.AppendNewStatement(v6MedStatement)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", v6MedStatement, err)
	}

	stmt2.GetOrCreateActions().GetOrCreateBgpActions().SetSetMed(oc.UnionUint32(med))
	stmt2.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	if !deviations.BGPSetMedActionUnsupported(dut) {
		stmt2.GetOrCreateActions().GetOrCreateBgpActions().SetSetMedAction(oc.BgpPolicy_BgpSetMedAction_SET)
	}
	stMatch.GetOrCreateConditions().SetCallPolicy(v6ASPPolicy)
	statPath := rp.GetOrCreatePolicyDefinition(v6ASPPolicy).GetStatement(v6ASPStatement).GetOrCreateConditions()
	statPath.SetCallPolicy(v6MedPolicy)

	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)

	dni := deviations.DefaultNetworkInstance(dut)

	// Configure the parent BGP import policy.
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		//policy under peer group
		path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().PeerGroup(peerGrpNamev6).ApplyPolicy()
		gnmi.BatchDelete(batch, path.Config())
		policy := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreatePeerGroup(peerGrpNamev6).GetOrCreateApplyPolicy()
		policy.SetExportPolicy([]string{v6MatchExportPolicy})
		gnmi.BatchReplace(batch, path.Config(), policy)
	} else {
		path := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).Bgp().Neighbor(atePort1.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).ApplyPolicy()
		gnmi.BatchDelete(batch, path.Config())
		policy := root.GetOrCreateNetworkInstance(dni).GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpName).GetOrCreateBgp().GetOrCreateNeighbor(atePort1.IPv6).GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).GetOrCreateApplyPolicy()
		policy.SetExportPolicy([]string{v6MatchExportPolicy})
		if !deviations.RoutingPolicyChainingUnsupported(dut) {
			gnmi.BatchUpdate(batch, path.Config(), policy)
		} else {
			gnmi.BatchReplace(batch, path.Config(), policy)
		}
	}
	batch.Set(t, dut)
}

func validateExportRoutingPolicyV6(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	t.Logf("Validating Export Routing Policy, waiting for 30 seconds")
	time.Sleep(time.Second * 30)
	bgpPrefixes := gnmi.GetAll[*otgtelemetry.BgpPeer_UnicastIpv6Prefix](t, ate.OTG(), gnmi.OTG().BgpPeer("atePort1.BGP6.peer").UnicastIpv6PrefixAny().State())

	found := false
	for _, bgpPrefix := range bgpPrefixes {
		if bgpPrefix.Address != nil && bgpPrefix.GetAddress() == v62Route &&
			bgpPrefix.PrefixLength != nil && bgpPrefix.GetPrefixLength() == v6RoutePrefix {
			found = true
			t.Logf("Prefix recevied on OTG is correct, got prefix %v, want prefix %v", bgpPrefix.GetAddress(), v62Route)
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
				if (count - 1) != asPathRepeat {
					t.Errorf("Expected %d prepends, got %d", asPathRepeat, (count - 1))
				} else {
					t.Logf("AS-PATH has DUT ASN %d occurrences", count) // 1 original + 15 prepended = 16 total
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
	t.Log("Configuring Traffic Flow")
	Flow := td.top.Flows().Add().SetName(fc.flowName)
	Flow.Metrics().SetEnable(true)
	Flow.TxRx().Port().SetTxName(fc.srcPort).SetRxNames(fc.dstPort)
	Flow.Size().SetFixed(trafficFrameSize)
	Flow.Rate().SetPps(trafficPps)
	Flow.Duration().FixedPackets().SetPackets(totalPackets)
	e1 := Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fc.src.MAC)
	e1.Dst().SetValue(fc.dstMac)
	if fc.flowType == "ipv4" {
		v4 := Flow.Packet().Add().Ipv4()
		v4.Src().SetValue(fc.src.IPv4)
		v4.Dst().Increment().SetStart(fc.dstIP).SetCount(10)
	} else {
		v6 := Flow.Packet().Add().Ipv6()
		v6.Src().SetValue(fc.src.IPv6)
		v6.Dst().Increment().SetStart(fc.dstIP).SetCount(10)
	}

	td.ate.OTG().PushConfig(t, td.top)
	td.ate.OTG().StartProtocols(t)
}

func checkTraffic(t *testing.T, td testData, flowName string) {
	td.ate.OTG().StartTraffic(t)
	time.Sleep(time.Second * trafficDuration)
	td.ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, td.ate.OTG(), td.top)
	otgutils.LogPortMetrics(t, td.ate.OTG(), td.top)

	t.Log("Checking flow telemetry...")
	recvMetric := gnmi.Get(t, td.ate.OTG(), gnmi.OTG().Flow(flowName).State())
	txPackets := recvMetric.GetCounters().GetOutPkts()
	rxPackets := recvMetric.GetCounters().GetInPkts()
	lostPackets := txPackets - rxPackets
	if txPackets == 0 {
		t.Fatalf("txPkts == %d, want > 0.", txPackets)
	}
	if got := (lostPackets * 100 / txPackets); got >= tolerance {
		t.Errorf("FAIL- Packet loss for flow %s: got %v, want %v", flowName, got, tolerance)
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

	t.Logf("Configuring route-policy for BGP on DUT")
	rp := root.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(permitAll)
	stmt, err := pdef.AppendNewStatement(permitAllStmtName)
	if err != nil {
		t.Fatalf("AppendNewStatement(%s) failed: %v", permitAllStmtName, err)
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	gnmi.Update(t, td.dut, gnmi.OC().RoutingPolicy().Config(), rp)

	pgv4 := bgp.GetOrCreatePeerGroup(peerGrpNamev4)
	pgv4.PeerGroupName = ygot.String(peerGrpNamev4)
	pgv6 := bgp.GetOrCreatePeerGroup(peerGrpNamev6)
	pgv6.PeerGroupName = ygot.String(peerGrpNamev6)
	nV41 := bgp.GetOrCreateNeighbor(atePort1.IPv4)
	nV41.SetPeerAs(ateAS1)
	nV41.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	nV41.PeerGroup = ygot.String(peerGrpNamev4)
	afisafiv41 := nV41.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afisafiv41.GetOrCreateApplyPolicy().SetImportPolicy([]string{permitAll})
	afisafiv41.GetOrCreateApplyPolicy().SetExportPolicy([]string{permitAll})

	nV42 := bgp.GetOrCreateNeighbor(atePort2.IPv4)
	nV42.SetPeerAs(ateAS2)
	nV42.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	nV42.PeerGroup = ygot.String(peerGrpNamev4)
	afisafiv42 := nV42.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afisafiv42.GetOrCreateApplyPolicy().SetImportPolicy([]string{permitAll})
	afisafiv42.GetOrCreateApplyPolicy().SetExportPolicy([]string{permitAll})

	nV61 := bgp.GetOrCreateNeighbor(atePort1.IPv6)
	nV61.SetPeerAs(ateAS1)
	nV61.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	nV61.PeerGroup = ygot.String(peerGrpNamev6)
	afisafiv61 := nV61.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	afisafiv61.GetOrCreateApplyPolicy().SetImportPolicy([]string{permitAll})
	afisafiv61.GetOrCreateApplyPolicy().SetExportPolicy([]string{permitAll})

	nV62 := bgp.GetOrCreateNeighbor(atePort2.IPv6)
	nV62.SetPeerAs(ateAS2)
	nV62.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	nV62.PeerGroup = ygot.String(peerGrpNamev6)
	afisafiv62 := nV62.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	afisafiv62.GetOrCreateApplyPolicy().SetImportPolicy([]string{permitAll})
	afisafiv62.GetOrCreateApplyPolicy().SetExportPolicy([]string{permitAll})

	gnmi.Update(t, td.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Config(), ni)

	// Configure eBGP on OTG port1.
	ipv41 := td.otgP1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dev1BGP := td.otgP1.Bgp().SetRouterId(atePort1.IPv4)
	bgp4Peer1 := dev1BGP.Ipv4Interfaces().Add().SetIpv4Name(ipv41.Name()).Peers().Add().SetName(td.otgP1.Name() + ".BGP4.peer")
	bgp4Peer1.SetPeerAddress(dutPort1.IPv4).SetAsNumber(ateAS1).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	bgp4Peer1.LearnedInformationFilter().SetUnicastIpv4Prefix(true)

	ipv61 := td.otgP1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	bgp6Peer1 := dev1BGP.Ipv6Interfaces().Add().SetIpv6Name(ipv61.Name()).Peers().Add().SetName(td.otgP1.Name() + ".BGP6.peer")
	bgp6Peer1.SetPeerAddress(dutPort1.IPv6).SetAsNumber(ateAS1).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	bgp6Peer1.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

	// Configure emulated network on ATE port1.
	netv41 := bgp4Peer1.V4Routes().Add().SetName("v4-bgpNet-dev1")
	netv41.Addresses().Add().SetAddress(advertisedIPv41.address).SetPrefix(advertisedIPv41.prefix)
	netv61 := bgp6Peer1.V6Routes().Add().SetName("v6-bgpNet-dev1")
	netv61.Addresses().Add().SetAddress(advertisedIPv61.address).SetPrefix(advertisedIPv61.prefix)

	// Configure eBGP on OTG port2.
	ipv42 := td.otgP2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dev2BGP := td.otgP2.Bgp().SetRouterId(atePort2.IPv4)
	bgp4Peer2 := dev2BGP.Ipv4Interfaces().Add().SetIpv4Name(ipv42.Name()).Peers().Add().SetName(td.otgP2.Name() + ".BGP4.peer")
	bgp4Peer2.SetPeerAddress(dutPort2.IPv4).SetAsNumber(ateAS2).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	bgp4Peer2.LearnedInformationFilter().SetUnicastIpv4Prefix(true)

	ipv62 := td.otgP2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	bgp6Peer2 := dev2BGP.Ipv6Interfaces().Add().SetIpv6Name(ipv62.Name()).Peers().Add().SetName(td.otgP2.Name() + ".BGP6.peer")
	bgp6Peer2.SetPeerAddress(dutPort2.IPv6).SetAsNumber(ateAS2).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	bgp6Peer2.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

	// Configure emulated network on ATE port2.
	netv42 := bgp4Peer2.V4Routes().Add().SetName("v4-bgpNet-dev2")
	netv42.Addresses().Add().SetAddress(advertisedIPv42.address).SetPrefix(advertisedIPv42.prefix)
	netv62 := bgp6Peer2.V6Routes().Add().SetName("v6-bgpNet-dev2")
	netv62.Addresses().Add().SetAddress(advertisedIPv62.address).SetPrefix(advertisedIPv62.prefix)
}

// verifyDUTBGPEstablished verifies on dut for BGP peer establishment.
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

// VerifyOTGBGPEstablished verifies on OTG for BGP peer establishment.
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

func validateCommunityValue(t *testing.T, dut *ondatra.DUTDevice, communitySetName string) {
	t.Helper()
	commSet := gnmi.Get[*oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet](t, dut,
		gnmi.OC().RoutingPolicy().DefinedSets().BgpDefinedSets().CommunitySet(communitySetName).State())
	if commSet == nil {
		t.Errorf("Community set is nil, want non-nil")
	}
	cm, _ := strconv.ParseInt(fmt.Sprintf("%04x%04x", bgpCoomunityVal, 100), 16, 0)
	if commSetMember := commSet.GetCommunityMember(); len(commSetMember) == 0 || !(containsValue(commSetMember, oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union(oc.UnionString(community))) || containsValue(commSetMember, oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union(oc.UnionUint32(cm)))) {
		t.Errorf("Community set member: %v, want: %s or %d", commSetMember, community, cm)
	} else {
		t.Logf("Community %s encoded as %d is FOUND in expected list %v\n", community, cm, commSet.GetCommunityMember())
	}
}

func containsValue[T comparable](slice []T, val T) bool {
	found := false
	for _, v := range slice {
		if v == val {
			found = true
			break
		}
	}
	return found
}
