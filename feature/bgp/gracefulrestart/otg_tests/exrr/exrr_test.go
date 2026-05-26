package bgp_exrr_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	bpb "github.com/openconfig/gnoi/bgp"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	trafficDuration          = 30 * time.Second
	triggerGrTimer           = 280
	grRestartTime            = 220
	grStaleRouteTime         = 250
	erRetentionTime          = 300
	erRetentionTimeLarge     = 15552000
	advertisedRoutesv4Prefix = 32
	advertisedRoutesv6Prefix = 128
	dutAS                    = 100
	ateAS                    = 200
	plenIPv4                 = 30
	plenIPv6                 = 126
	epeerv4GrpName           = "EBGP-PEER-GROUP-V4"
	epeerv6GrpName           = "EBGP-PEER-GROUP-V6"
	ipeerv4GrpName           = "IBGP-PEER-GROUP-V4"
	ipeerv6GrpName           = "IBGP-PEER-GROUP-V6"
	ipv4Prefix1              = "99.1.1.1"
	ipv4Prefix2              = "100.1.1.1"
	ipv4Prefix3              = "101.1.1.1"
	ipv4Prefix4              = "199.1.1.1"
	ipv4Prefix5              = "200.1.1.1"
	ipv4Prefix6              = "201.1.1.1"
	ipv6Prefix1              = "1001::1"
	ipv6Prefix2              = "2001::1"
	ipv6Prefix3              = "3001::1"
	ipv6Prefix4              = "4000::1"
	ipv6Prefix5              = "5000::1"
	ipv6Prefix6              = "6000::1"
	noerrasn                 = 100
	errnodeprefasn           = 200
	testibgpasn              = 500
	testebgpasn              = 600
	newibgpasn               = 700
	newebgpasn               = 800
	med50                    = 50
	med150                   = 150
	routeCount               = 3
)

var (
	dutIBGP = attrs.Attributes{
		Desc:    "DUT to port1 ATE iBGP peer",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateIBGP = attrs.Attributes{
		Name:    "ateIBGP",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutEBGP = attrs.Attributes{
		Desc:    "DUT to port1 ATE eBGP peer",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateEBGP = attrs.Attributes{
		Name:    "ateEBGP",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	gracefulRestartHardResetValidation = &packetvalidationhelpers.PacketValidation{
		PortName:    "port2",
		CaptureName: "capture",
		Validations: []packetvalidationhelpers.ValidationType{
			packetvalidationhelpers.ValidateBGPHeader},
		BGPLayer: &packetvalidationhelpers.BGPLayer{TYPE: 3, ErrorCode: 6, ErrorSubCode: 9},
	}
)

type prefixAttributes struct {
	prefix                string
	bgpPeer               string
	advertisedbgpPeer     string
	ipType                string
	configuredCommunities []struct {
		ASNumber uint16
		Value    uint16
	}
	expectedCommunities []struct {
		ASNumber uint16
		Value    uint16
	}
	expectedasPath []uint32
	expectedmed    *uint32
}

type communityConfig struct {
	name string
	asn  int
	val  int
}

var communityConf = []communityConfig{
	{
		name: "NO-ERR",
		asn:  noerrasn,
		val:  1,
	},
	{
		name: "ERR-NO-DEPREF",
		asn:  errnodeprefasn,
		val:  1,
	},
	{
		name: "STALE",
		asn:  65535,
		val:  6,
	},
	{
		name: "TEST-IBGP",
		asn:  testibgpasn,
		val:  1,
	},
	{
		name: "TEST-EBGP",
		asn:  testebgpasn,
		val:  1,
	},
	{
		name: "NEW-IBGP",
		asn:  newibgpasn,
		val:  1,
	},
	{
		name: "NEW-EBGP",
		asn:  newebgpasn,
		val:  1,
	},
}

var prefixAttrs = []prefixAttributes{
	{
		prefix:            ipv4Prefix1,
		ipType:            "ipv4",
		bgpPeer:           ateEBGP.Name + ".BGP4.peer", // This is to check validation on remote peer
		advertisedbgpPeer: ateIBGP.Name + ".BGP4.peer", // This is used to configure on local peer
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{noerrasn, 1},
		},
		expectedCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{noerrasn, 1},
		},
	},
	{
		prefix:            ipv4Prefix2,
		ipType:            "ipv4",
		bgpPeer:           ateEBGP.Name + ".BGP4.peer",
		advertisedbgpPeer: ateIBGP.Name + ".BGP4.peer",
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{errnodeprefasn, 1},
		},
		expectedCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{errnodeprefasn, 1},
		},
	},
	{
		prefix:            ipv4Prefix3,
		ipType:            "ipv4",
		bgpPeer:           ateEBGP.Name + ".BGP4.peer",
		advertisedbgpPeer: ateIBGP.Name + ".BGP4.peer",
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{testibgpasn, 1},
		},
		expectedCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{testibgpasn, 1}, {newebgpasn, 1},
		},
		expectedasPath: []uint32{100, 100, 100},
	},
	{
		prefix:            ipv4Prefix4,
		ipType:            "ipv4",
		bgpPeer:           ateIBGP.Name + ".BGP4.peer",
		advertisedbgpPeer: ateEBGP.Name + ".BGP4.peer",
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{noerrasn, 1},
		},
		expectedCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{noerrasn, 1},
		},
	},
	{
		prefix:            ipv4Prefix5,
		ipType:            "ipv4",
		bgpPeer:           ateIBGP.Name + ".BGP4.peer",
		advertisedbgpPeer: ateEBGP.Name + ".BGP4.peer",
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{errnodeprefasn, 1},
		},
		expectedCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{errnodeprefasn, 1},
		},
	},
	{
		prefix:            ipv4Prefix6,
		ipType:            "ipv4",
		bgpPeer:           ateIBGP.Name + ".BGP4.peer",
		advertisedbgpPeer: ateEBGP.Name + ".BGP4.peer",
		expectedmed:       ptrToUint32(50),
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{testebgpasn, 1},
		},
		expectedCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{testebgpasn, 1}, {newibgpasn, 1},
		},
	},
}

var prefixV6Attrs = []prefixAttributes{
	{
		prefix:            ipv6Prefix1,
		ipType:            "ipv6",
		bgpPeer:           ateEBGP.Name + ".BGP6.peer", // This is to check validation on remote peer
		advertisedbgpPeer: ateIBGP.Name + ".BGP6.peer", // This is used to configure on local peer
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{noerrasn, 1},
		},
		expectedCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{noerrasn, 1},
		},
	},
	{
		prefix:            ipv6Prefix2,
		ipType:            "ipv6",
		bgpPeer:           ateEBGP.Name + ".BGP6.peer",
		advertisedbgpPeer: ateIBGP.Name + ".BGP6.peer",
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{errnodeprefasn, 1},
		},
	},
	{
		prefix:            ipv6Prefix3,
		ipType:            "ipv6",
		bgpPeer:           ateEBGP.Name + ".BGP6.peer",
		advertisedbgpPeer: ateIBGP.Name + ".BGP6.peer",
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{testibgpasn, 1},
		},
		expectedCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{testibgpasn, 1}, {newebgpasn, 1},
		},
		expectedasPath: []uint32{100, 100, 100},
	},
	{
		prefix:            ipv6Prefix4,
		ipType:            "ipv6",
		bgpPeer:           ateIBGP.Name + ".BGP6.peer",
		advertisedbgpPeer: ateEBGP.Name + ".BGP6.peer",
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{noerrasn, 1},
		},
		expectedCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{noerrasn, 1},
		},
	},
	{
		prefix:            ipv6Prefix5,
		ipType:            "ipv6",
		bgpPeer:           ateIBGP.Name + ".BGP6.peer",
		advertisedbgpPeer: ateEBGP.Name + ".BGP6.peer",
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{errnodeprefasn, 1},
		},
		expectedCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{errnodeprefasn, 1},
		},
	},
	{
		prefix:            ipv6Prefix6,
		ipType:            "ipv6",
		bgpPeer:           ateIBGP.Name + ".BGP6.peer",
		advertisedbgpPeer: ateEBGP.Name + ".BGP6.peer",
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{testebgpasn, 1},
		},
		expectedCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{testebgpasn, 1}, {newibgpasn, 1},
		},
		expectedmed: ptrToUint32(50),
	},
}

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()

	for _, cfg := range communityConf {
		communitySet := rp.GetOrCreateDefinedSets().
			GetOrCreateBgpDefinedSets().
			GetOrCreateCommunitySet(cfg.name)

		communitySet.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{
			oc.UnionString(fmt.Sprintf("%d:%d", cfg.asn, cfg.val)),
		})
	}

	// Export-EBGP Policy
	exportEBGP := rp.GetOrCreatePolicyDefinition("EXPORT-EBGP")
	exportEBGPstmt10, err := exportEBGP.AppendNewStatement("10")
	if err != nil {
		t.Errorf("error while creating new statement %v", err)
	}
	exportEBGPstmt10.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	exportEBGPstmtCommunitySet := exportEBGPstmt10.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
	exportEBGPstmtCommunitySet.SetCommunitySet("TEST-IBGP")
	exportEBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend().SetAsn(100)
	exportEBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend().SetRepeatN(2)

	sc := exportEBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetCommunity()
	sc.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
	sc.SetMethod(oc.SetCommunity_Method_REFERENCE)
	sc.GetOrCreateReference().SetCommunitySetRefs([]string{"NEW-EBGP", "TEST-IBGP"})

	exportEBGPstmt20, err := exportEBGP.AppendNewStatement("20")
	if err != nil {
		t.Errorf("error while creating new statement %v", err)
	}
	exportEBGPstmt20.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	// Export-IBGP Policy
	exportIBGP := rp.GetOrCreatePolicyDefinition("EXPORT-IBGP")
	exportIBGPstmt10, err := exportIBGP.AppendNewStatement("10")
	if err != nil {
		t.Errorf("error while creating new statement %v", err)
	}
	exportIBGPstmt10.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	sc = exportIBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetCommunity()
	sc.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
	sc.SetMethod(oc.SetCommunity_Method_REFERENCE)
	sc.GetOrCreateReference().SetCommunitySetRefs([]string{"TEST-EBGP", "NEW-IBGP"})

	exportIBGPstmt10.GetOrCreateConditions().GetOrCreateBgpConditions().SetMedEq(50)

	exportIBGPstmt20, err := exportIBGP.AppendNewStatement("20")
	if err != nil {
		t.Errorf("error while creating new statement %v", err)
	}
	exportIBGPstmt20.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	// Import-IBGP Policy
	importIBGP := rp.GetOrCreatePolicyDefinition("IMPORT-IBGP")
	importIBGPstmt10, err := importIBGP.AppendNewStatement("10")
	if err != nil {
		t.Errorf("error while creating new statement %v", err)
	}
	importIBGPstmt10.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	importIBGPstmtCommunitySet := importIBGPstmt10.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
	importIBGPstmtCommunitySet.SetCommunitySet("TEST-IBGP")
	importIBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().SetSetLocalPref(200)

	importIBGPstmt20, err := importIBGP.AppendNewStatement("20")
	if err != nil {
		t.Errorf("error while creating new statement %v", err)
	}
	importIBGPstmt20.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	// Import-EBGP policy
	importEBGP := rp.GetOrCreatePolicyDefinition("IMPORT-EBGP")
	importEBGPstmt10, err := importEBGP.AppendNewStatement("10")
	if err != nil {
		t.Errorf("error while creating new statement %v", err)
	}
	importEBGPstmt10.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	setMEDstmtCommunitySet := importEBGPstmt10.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
	setMEDstmtCommunitySet.SetCommunitySet("TEST-EBGP")
	importEBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().SetSetMed(oc.UnionUint32(med50))
	if deviations.BGPSetMedActionUnsupported(dut) {
		importEBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().SetSetMedAction(oc.BgpPolicy_BgpSetMedAction_SET)
	}

	importEBGPstmt20, err := importEBGP.AppendNewStatement("20")
	if err != nil {
		t.Errorf("error while creating new statement %v", err)
	}
	importEBGPstmt20.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

	if deviations.ExtendedRouteRetentionOcUnsupported(dut) {
		staleRoutePolicyMap := (`
		route-map STALE-ROUTE-POLICY statement 10 deny 10
		match community NO-ERR
		!
		route-map STALE-ROUTE-POLICY statement 20 permit 20
		match community ERR-NO-DEPREF
		set community community-list STALE
		!
		route-map STALE-ROUTE-POLICY statement 30 permit 30
		set community community-list STALE
		set local-preference 0
		!
	`)
		helpers.GnmiCLIConfig(t, dut, staleRoutePolicyMap)
	} else {
		pd := rp.GetOrCreatePolicyDefinition("STALE-ROUTE-POLICY")
		stmt10, err := pd.AppendNewStatement("10")
		if err != nil {
			t.Errorf("error while creating new statement %v", err)
		}
		stmt10.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
		matchCommunitySet := stmt10.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
		matchCommunitySet.SetCommunitySet("NO-ERR")

		stmt20, err := pd.AppendNewStatement("20")
		if err != nil {
			t.Errorf("error while creating new statement %v", err)
		}
		stmt20.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
		matchCommunitySet2 := stmt20.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
		matchCommunitySet2.SetCommunitySet("ERR-NO-DEPREF")

		sc := stmt20.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetCommunity()
		sc.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
		sc.SetMethod(oc.SetCommunity_Method_REFERENCE)
		sc.GetOrCreateReference().SetCommunitySetRefs([]string{"STALE"})

		stmt30, err := pd.AppendNewStatement("30")
		if err != nil {
			t.Errorf("error while creating new statement %v", err)
		}
		stmt30.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
		matchCommunitySet3 := stmt30.GetOrCreateActions().GetOrCreateBgpActions()
		matchCommunitySet3.SetSetLocalPref(0)

		sc = stmt30.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetCommunity()
		sc.SetOptions(oc.BgpPolicy_BgpSetCommunityOptionType_ADD)
		sc.SetMethod(oc.SetCommunity_Method_REFERENCE)
		sc.GetOrCreateReference().SetCommunitySetRefs([]string{"STALE"})

		gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
	}
}

// configureDUT configures all the interfaces and network instance on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutIBGP.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutEBGP.NewOCInterface(dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i2.GetName()).Config(), i2)

	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, i1.GetName(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, i2.GetName(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

type bgpNeighbor struct {
	as         uint32
	neighborip string
	isV4       bool
}

func buildNbrListebgp() []*bgpNeighbor {
	nbr1v4 := &bgpNeighbor{as: ateAS, neighborip: ateEBGP.IPv4, isV4: true}
	nbr1v6 := &bgpNeighbor{as: ateAS, neighborip: ateEBGP.IPv6, isV4: false}
	return []*bgpNeighbor{nbr1v4, nbr1v6}
}

func buildNbrListibgp() []*bgpNeighbor {
	nbr2v4 := &bgpNeighbor{as: dutAS, neighborip: ateIBGP.IPv4, isV4: true}
	nbr2v6 := &bgpNeighbor{as: dutAS, neighborip: ateIBGP.IPv6, isV4: false}
	return []*bgpNeighbor{nbr2v4, nbr2v6}
}

func ebgpWithNbr(params cfgplugins.BGPGracefulRestartConfig, nbrs []*bgpNeighbor, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()

	g.SetAs(params.DutAS)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(true)
	g.SetRouterId(dutEBGP.IPv4)
	bgpgr := g.GetOrCreateGracefulRestart()
	bgpgr.SetEnabled(true)
	bgpgr.SetRestartTime(params.GracefulRestartTime)
	bgpgr.SetStaleRoutesTime(params.GracefulRestartStaleRouteTime)

	pg := bgp.GetOrCreatePeerGroup(epeerv4GrpName)
	pg.SetPeerGroupName(epeerv4GrpName)

	pgV6 := bgp.GetOrCreatePeerGroup(epeerv6GrpName)
	pgV6.SetPeerGroupName(epeerv6GrpName)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{"EXPORT-EBGP"})
		rpl.SetImportPolicy([]string{"IMPORT-EBGP"})
		rplv6 := pgV6.GetOrCreateApplyPolicy()
		rplv6.SetExportPolicy([]string{"EXPORT-EBGP"})
		rplv6.SetImportPolicy([]string{"IMPORT-EBGP"})

	} else {
		pg1af4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg1af4.SetEnabled(true)

		pg1rpl4 := pg1af4.GetOrCreateApplyPolicy()
		pg1rpl4.SetExportPolicy([]string{"EXPORT-EBGP"})
		pg1rpl4.SetImportPolicy([]string{"IMPORT-EBGP"})

		pg1af6 := pgV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pg1af6.SetEnabled(true)
		pg1rpl6 := pg1af6.GetOrCreateApplyPolicy()
		pg1rpl6.SetExportPolicy([]string{"EXPORT-EBGP"})
		pg1rpl6.SetImportPolicy([]string{"IMPORT-EBGP"})
	}

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.SetPeerGroup(epeerv4GrpName)
			nv4.SetPeerAs(nbr.as)
			nv4.SetEnabled(true)
			nv4.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED})
			af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.SetEnabled(true)
			af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.SetEnabled(false)
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.SetPeerGroup(epeerv6GrpName)
			nv6.SetPeerAs(nbr.as)
			nv6.SetEnabled(true)
			nv6.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED})
			af6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.SetEnabled(true)
			af4 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.SetEnabled(false)
		}
	}
	return niProto
}

func ibgpWithNbr(params cfgplugins.BGPGracefulRestartConfig, nbrs []*bgpNeighbor, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.SetAs(params.DutAS)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).SetEnabled(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).SetEnabled(true)
	g.SetRouterId(dutEBGP.IPv4)
	bgpgr := g.GetOrCreateGracefulRestart()
	bgpgr.SetEnabled(true)

	pg := bgp.GetOrCreatePeerGroup(ipeerv4GrpName)
	pg.SetPeerGroupName(ipeerv4GrpName)

	pgV6 := bgp.GetOrCreatePeerGroup(ipeerv6GrpName)
	pgV6.SetPeerGroupName(ipeerv6GrpName)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{"EXPORT-IBGP"})
		rpl.SetImportPolicy([]string{"IMPORT-IBGP"})
		rplv6 := pgV6.GetOrCreateApplyPolicy()
		rplv6.SetExportPolicy([]string{"EXPORT-IBGP"})
		rplv6.SetImportPolicy([]string{"IMPORT-IBGP"})

	} else {
		pg1af4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg1af4.SetEnabled(true)

		pg1rpl4 := pg1af4.GetOrCreateApplyPolicy()
		pg1rpl4.SetExportPolicy([]string{"EXPORT-IBGP"})
		pg1rpl4.SetImportPolicy([]string{"IMPORT-IBGP"})

		pg1af6 := pgV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pg1af6.SetEnabled(true)
		pg1rpl6 := pg1af6.GetOrCreateApplyPolicy()
		pg1rpl6.SetExportPolicy([]string{"EXPORT-IBGP"})
		pg1rpl6.SetImportPolicy([]string{"IMPORT-IBGP"})
	}

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.SetPeerGroup(ipeerv4GrpName)
			nv4.SetPeerAs(nbr.as)
			nv4.SetEnabled(true)
			nv4.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED})
			af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.SetEnabled(true)
			af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.SetEnabled(false)
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.SetPeerGroup(ipeerv6GrpName)
			nv6.SetPeerAs(nbr.as)
			nv6.SetEnabled(true)
			nv6.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED})
			af6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.SetEnabled(true)
			af4 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.SetEnabled(false)
		}
	}
	return niProto
}

func mustCheckBgpGRConfig(t *testing.T, params cfgplugins.BGPGracefulRestartConfig, dut *ondatra.DUTDevice) error {
	t.Helper()
	t.Log("Verifying BGP configuration")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	isGrEnabled := gnmi.Get(t, dut, statePath.Global().GracefulRestart().Enabled().State())
	t.Logf("isGrEnabled %v", isGrEnabled)
	if isGrEnabled {
		t.Logf("Graceful restart on neighbor %v enabled as Expected", ateEBGP.IPv4)
	} else {
		return fmt.Errorf("expected Graceful restart status on neighbor: got %v, want Enabled", isGrEnabled)
	}

	grTimerVal := gnmi.Get(t, dut, statePath.Global().GracefulRestart().RestartTime().State())
	t.Logf("grTimerVal %v", grTimerVal)
	if grTimerVal == uint16(params.GracefulRestartTime) {
		t.Logf("Graceful restart timer enabled as expected to be %v", params.GracefulRestartTime)
	} else {
		return fmt.Errorf("expected Graceful restart timer: got %v, want %v", grTimerVal, params.GracefulRestartTime)
	}

	grStaleRouteTimeVal := gnmi.Get(t, dut, statePath.Global().GracefulRestart().StaleRoutesTime().State())
	t.Logf("grStaleRouteTimeVal %v", grStaleRouteTimeVal)
	if grStaleRouteTimeVal == uint16(params.GracefulRestartStaleRouteTime) {
		t.Logf("Graceful restart Stale Route timer enabled as expected to be %v", params.GracefulRestartStaleRouteTime)
	} else {
		return fmt.Errorf("expected Graceful restart timer: got %v, want %v", grStaleRouteTimeVal, params.GracefulRestartStaleRouteTime)
	}
	return nil
}

func mustCheckBgpStatus(t *testing.T, dut *ondatra.DUTDevice, routeCount uint32) {
	t.Helper()
	t.Log("Verifying BGP state")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, attr := range []attrs.Attributes{ateEBGP, ateIBGP} {

		nbrPath := statePath.Neighbor(attr.IPv4)
		nbrPathv6 := statePath.Neighbor(attr.IPv6)

		// Get BGP adjacency state
		t.Logf("Waiting for BGP neighbor %s to establish", attr.IPv4)
		sessionStatePath := nbrPath.SessionState().State()

		watchFN := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			t.Logf("current state is %s", currState)
			return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}

		_, ok := gnmi.Watch(t, dut, sessionStatePath, 2*time.Minute, watchFN).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatal("No BGP neighbor formed...")
		}

		// Get BGPv6 adjacency state
		t.Logf("Waiting for BGPv6 neighbor %s to establish", attr.IPv6)
		sessionStatePathV6 := nbrPathv6.SessionState().State()

		watchFNv6 := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			t.Logf("current state is %s", currState)
			return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}

		_, ok = gnmi.Watch(t, dut, sessionStatePathV6, 2*time.Minute, watchFNv6).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, dut, nbrPathv6.State()))
			t.Fatal("No BGPv6 neighbor formed...")
		}

		t.Log("Waiting for BGP v4 prefixes to be installed")
		installedPrefixesPathV4 := nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes().Installed().State()

		watchInstalledPrefixesV4 := func(val *ygnmi.Value[uint32]) bool {
			prefixCount, ok := val.Val()
			return ok && prefixCount == routeCount
		}

		got, found := gnmi.Watch(t, dut, installedPrefixesPathV4, 180*time.Second, watchInstalledPrefixesV4).Await(t)
		if !found {
			t.Errorf("installed prefixes v4 mismatch: got %v, want %v", got, routeCount)
		}

		t.Log("Waiting for BGP v6 prefixes to be installed")
		installedPrefixesPathV6 := nbrPathv6.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes().Installed().State()

		watchInstalledPrefixesV6 := func(val *ygnmi.Value[uint32]) bool {
			prefixCount, ok := val.Val()
			return ok && prefixCount == routeCount
		}

		got, found = gnmi.Watch(t, dut, installedPrefixesPathV6, 180*time.Second, watchInstalledPrefixesV6).Await(t)
		if !found {
			t.Errorf("installed prefixes v6 mismatch: got %v, want %v", got, routeCount)
		}
	}
}

func mustCheckBgpStatusDown(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Verifying BGP state down")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, attr := range []attrs.Attributes{ateEBGP, ateIBGP} {

		nbrPath := statePath.Neighbor(attr.IPv4)
		nbrPathv6 := statePath.Neighbor(attr.IPv6)

		// Get BGP adjacency state
		t.Logf("Waiting for BGP neighbor %s to establish", attr.IPv4)
		sessionStatePath := nbrPath.SessionState().State()

		watchNeighborDown := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			t.Logf("current state is %s", currState)
			return ok && currState != oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}

		_, ok := gnmi.Watch(t, dut, sessionStatePath, 2*time.Minute, watchNeighborDown).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatal("BGP neighbor not down")
		}

		// Get BGPv6 adjacency state
		t.Logf("Waiting for BGPv6 neighbor %s to establish", attr.IPv6)
		sessionStatePathV6 := nbrPathv6.SessionState().State()

		watchNeighborDownV6 := func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			t.Logf("current state is %s", currState)
			return ok && currState != oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}

		_, ok = gnmi.Watch(t, dut, sessionStatePathV6, 2*time.Minute, watchNeighborDownV6).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, dut, nbrPathv6.State()))
			t.Fatal("BGP neighbor not down")
		}

	}
}

func mustCheckEndOfRibReceived(t *testing.T, ate *ondatra.ATEDevice, params cfgplugins.BGPGracefulRestartConfig) {
	t.Helper()
	for _, peer := range params.BgpPeers {
		nbrPath := gnmi.OTG().BgpPeer(peer)
		_, ok := gnmi.Watch(t, ate.OTG(), nbrPath.Counters().InEndOfRib().State(), 2*time.Minute, func(val *ygnmi.Value[uint64]) bool {
			currState, ok := val.Val()
			return ok && currState >= 1
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP End of state", nbrPath.State(), gnmi.Get(t, ate.OTG(), nbrPath.State()))
			t.Errorf("No BGP End of RIB received %s", peer)
		}
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice, params cfgplugins.BGPGracefulRestartConfig) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	ateIBGP.AddToOTG(config, p1, &dutIBGP)
	ateEBGP.AddToOTG(config, p2, &dutEBGP)

	iBGPDev := config.Devices().Items()[0]
	iBGPEth := iBGPDev.Ethernets().Items()[0]
	iBGPIPv4 := iBGPEth.Ipv4Addresses().Items()[0]
	iBGPIPv6 := iBGPEth.Ipv6Addresses().Items()[0]
	eBGPDev := config.Devices().Items()[1]
	eBGPEth := eBGPDev.Ethernets().Items()[0]
	eBGPIPv4 := eBGPEth.Ipv4Addresses().Items()[0]
	eBGPIPv6 := eBGPEth.Ipv6Addresses().Items()[0]

	cap := config.Captures().Add().SetName("capture").SetPortNames([]string{p2.ID()}).SetFormat(gosnappi.CaptureFormat.PCAP)
	filter := cap.Filters().Add()
	ipToHex, err := iputil.IPv4ToHex(ateEBGP.IPv4)
	if err != nil {
		t.Errorf("failed to convert ip to hex: %v", err)
	}
	filter.Ipv4().Dst().SetValue(ipToHex)

	iBGP := iBGPDev.Bgp().SetRouterId(iBGPIPv4.Address())
	iBGP4Peer := iBGP.Ipv4Interfaces().Add().SetIpv4Name(iBGPIPv4.Name()).Peers().Add().SetName(ateIBGP.Name + ".BGP4.peer")
	iBGP4Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(uint32(params.GracefulRestartTime))
	iBGP4Peer.SetPeerAddress(iBGPIPv4.Gateway()).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iBGP4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	iBGP6Peer := iBGP.Ipv6Interfaces().Add().SetIpv6Name(iBGPIPv6.Name()).Peers().Add().SetName(ateIBGP.Name + ".BGP6.peer")
	iBGP6Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(uint32(params.GracefulRestartTime))
	iBGP6Peer.SetPeerAddress(iBGPIPv6.Gateway()).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iBGP6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	eBGP := eBGPDev.Bgp().SetRouterId(eBGPIPv4.Address())
	eBGP4Peer := eBGP.Ipv4Interfaces().Add().SetIpv4Name(eBGPIPv4.Name()).Peers().Add().SetName(ateEBGP.Name + ".BGP4.peer")
	eBGP4Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(uint32(params.GracefulRestartTime))
	eBGP4Peer.SetPeerAddress(eBGPIPv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	eBGP4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	eBGP6Peer := eBGP.Ipv6Interfaces().Add().SetIpv6Name(eBGPIPv6.Name()).Peers().Add().SetName(ateEBGP.Name + ".BGP6.peer")
	eBGP6Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(uint32(params.GracefulRestartTime))
	eBGP6Peer.SetPeerAddress(eBGPIPv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	eBGP6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// Loop through prefixAttrs and configure routes
	for i, attr := range prefixAttrs {
		var peer gosnappi.BgpV4Peer
		var nexthop gosnappi.DeviceIpv4
		if attr.advertisedbgpPeer == ateIBGP.Name+".BGP4.peer" {
			peer = iBGP4Peer
			nexthop = iBGPIPv4
		} else {
			peer = eBGP4Peer
			nexthop = eBGPIPv4
		}

		route := peer.V4Routes().Add().SetName(fmt.Sprintf("IPv4Prefix%d", i+1))
		route.SetNextHopIpv4Address(nexthop.Address()).
			SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
			SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		route.Addresses().Add().SetAddress(attr.prefix).SetPrefix(advertisedRoutesv4Prefix)

		// Set communities if configured
		for _, comm := range attr.configuredCommunities {
			route.Communities().Add().
				SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER).
				SetAsNumber(uint32(comm.ASNumber)).
				SetAsCustom(uint32(comm.Value))
		}

		if attr.expectedmed != nil {
			route.Advanced().SetIncludeMultiExitDiscriminator(true).SetMultiExitDiscriminator(*attr.expectedmed)
		}
	}

	// Loop through prefixAttrs and configure routes
	for i, attr := range prefixV6Attrs {
		var peerv6 gosnappi.BgpV6Peer
		var nexthop gosnappi.DeviceIpv6
		if attr.advertisedbgpPeer == ateIBGP.Name+".BGP6.peer" {
			peerv6 = iBGP6Peer
			nexthop = iBGPIPv6
		} else {
			peerv6 = eBGP6Peer
			nexthop = eBGPIPv6
		}

		route := peerv6.V6Routes().Add().SetName(fmt.Sprintf("IPv6Prefix%d", i+1))
		route.SetNextHopIpv6Address(nexthop.Address()).
			SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
			SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		route.Addresses().Add().SetAddress(attr.prefix).SetPrefix(advertisedRoutesv6Prefix)

		// Set communities if configured
		for _, comm := range attr.configuredCommunities {
			route.Communities().Add().
				SetType(gosnappi.BgpCommunityType.MANUAL_AS_NUMBER).
				SetAsNumber(uint32(comm.ASNumber)).
				SetAsCustom(uint32(comm.Value))
		}

		if attr.expectedmed != nil {
			route.Advanced().SetIncludeMultiExitDiscriminator(true).SetMultiExitDiscriminator(*attr.expectedmed)
		}

	}

	return config
}

func configureFlow(t *testing.T, src, dst attrs.Attributes, srcip string, dstip string, config gosnappi.Config) {
	t.Helper()
	// ATE Traffic Configuration
	t.Log("start ate Traffic config")
	t.Logf("Creating the traffic flow with source %s and destination %s", src.IPv4, dstip)
	flowipv4 := config.Flows().Add().SetName(srcip + "-" + dstip + "-Ipv4")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{src.Name + ".IPv4"}).
		SetRxNames([]string{dst.Name + ".IPv4"})
	flowipv4.Size().SetFixed(512)
	flowipv4.Rate().SetPps(100)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(src.MAC)
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(srcip)
	v4.Dst().Increment().SetStart(dstip)
}

func configureFlowV6(t *testing.T, src, dst attrs.Attributes, srcip string, dstip string, config gosnappi.Config) {
	t.Helper()
	// ATE Traffic Configuration
	t.Log("start ate Traffic config")
	t.Logf("Creating the traffic flow with source %s and destination %s", src.IPv6, dstip)
	flowipv6 := config.Flows().Add().SetName(srcip + "-" + dstip + "-Ipv6")
	flowipv6.Metrics().SetEnable(true)
	flowipv6.TxRx().Device().
		SetTxNames([]string{src.Name + ".IPv6"}).
		SetRxNames([]string{dst.Name + ".IPv6"})
	flowipv6.Size().SetFixed(512)
	flowipv6.Rate().SetPps(100)
	e1 := flowipv6.Packet().Add().Ethernet()
	e1.Src().SetValue(src.MAC)
	v6 := flowipv6.Packet().Add().Ipv6()
	v6.Src().SetValue(srcip)
	v6.Dst().Increment().SetStart(dstip)
}

func verifyNoPacketLoss(t *testing.T, ate *ondatra.ATEDevice, flows []string) {
	t.Helper()
	otg := ate.OTG()
	c := otg.FetchConfig(t)
	otgutils.LogFlowMetrics(t, otg, c)

	for _, f := range flows {
		t.Logf("Verifying flow metrics for flow %s\n", f)
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(f).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		lostPackets := txPackets - rxPackets
		if txPackets == 0 {
			t.Fatalf("Tx packets should be higher than 0 for flow %s", f)
		}
		if lossPct := lostPackets * 100 / txPackets; lossPct < 5.0 {
			t.Logf("Traffic received as expected! Got %v loss", lossPct)
		} else {
			t.Errorf("traffic verification failed, Loss Pct for Flow %s: got %f", f, lossPct)
		}
	}
}

func confirmPacketLoss(t *testing.T, ate *ondatra.ATEDevice, flows []string) {
	t.Helper()
	otg := ate.OTG()
	c := otg.FetchConfig(t)
	otgutils.LogFlowMetrics(t, otg, c)
	for _, f := range flows {
		t.Logf("Verifying flow metrics for flow %s\n", f)
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(f).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		lostPackets := txPackets - rxPackets
		if txPackets == 0 {
			t.Fatalf("Tx packets should be higher than 0 for flow %s", f)
		}
		if lossPct := lostPackets * 100 / txPackets; lossPct > 99.0 {
			t.Logf("Traffic received as expected! Loss seen as expected: got %v, want 100%% ", lossPct)
		} else {
			t.Errorf("traffic %s is expected to fail: got %f, want 100%% failure", f, lossPct)
		}
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	t.Log("Starting traffic")
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	t.Log("Stop traffic")
	ate.OTG().StopTraffic(t)
}

// createGracefulRestartAction create a bgp control action for initiating the graceful restart process
func createGracefulRestartAction(t *testing.T, peerNames []string, restartDelay uint32, notification string) gosnappi.ControlAction {
	t.Helper()
	grAction := gosnappi.NewControlAction()
	switch notification {
	case "soft":
		grAction.Protocol().Bgp().InitiateGracefulRestart().
			SetPeerNames(peerNames).SetRestartDelay(restartDelay).Notification().Cease().SetSubcode(gosnappi.DeviceBgpCeaseErrorSubcode.ADMIN_RESET_CODE6_SUBCODE4)
	case "hard":
		grAction.Protocol().Bgp().InitiateGracefulRestart().
			SetPeerNames(peerNames).SetRestartDelay(restartDelay).Notification().Cease().SetSubcode(gosnappi.DeviceBgpCeaseErrorSubcode.HARD_RESET_CODE6_SUBCODE9)
	default:
		grAction.Protocol().Bgp().InitiateGracefulRestart().
			SetPeerNames(peerNames).SetRestartDelay(restartDelay)
	}
	return grAction
}

func validatePrefixesWithAttributes(t *testing.T, ate *ondatra.ATEDevice, prefixAttrs []prefixAttributes) {
	t.Helper()
	for _, ep := range prefixAttrs {
		bgpPrefix := gnmi.Get(t, ate.OTG(), gnmi.OTG().BgpPeer(ep.bgpPeer).UnicastIpv4Prefix(ep.prefix, 32, otgtelemetry.UnicastIpv4Prefix_Origin_IGP, 0).State())

		// Validate Communities
		if len(ep.expectedCommunities) > 0 {
			for _, expectedComm := range ep.expectedCommunities {
				found := false
				for _, actualComm := range bgpPrefix.Community {
					if actualComm.GetCustomAsNumber() == expectedComm.ASNumber &&
						actualComm.GetCustomAsValue() == expectedComm.Value {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("prefix %s: Expected community (%d,%d) not found", ep.prefix, expectedComm.ASNumber, expectedComm.Value)
				}
			}
		}

		// Validate AS Path
		if len(ep.expectedasPath) > 0 {
			actualAS := make([]uint32, 0)
			for _, seg := range bgpPrefix.AsPath {
				actualAS = append(actualAS, seg.AsNumbers...)
			}
			if len(actualAS) != len(ep.expectedasPath) {
				t.Errorf("prefix %s: AS Path length mismatch. Got %d, want %d", ep.prefix, len(actualAS), len(ep.expectedasPath))
			} else {
				for i := range actualAS {
					if actualAS[i] != ep.expectedasPath[i] {
						t.Errorf("prefix %s: ASPath[%d] mismatch. Got %d, want %d", ep.prefix, i, actualAS[i], ep.expectedasPath[i])
					}
				}
			}
		}

		// Validate MED
		if ep.expectedmed != nil {
			actualMED := bgpPrefix.GetMultiExitDiscriminator()
			if actualMED != *ep.expectedmed {
				t.Errorf("prefix %s: MED mismatch. Got %d, want %d", ep.prefix, actualMED, *ep.expectedmed)
			}
		}
	}

}

func validateV4PrefixesWithAftEntries(t *testing.T, dut *ondatra.DUTDevice, prefixAttrs []prefixAttributes) {
	for _, ep := range prefixAttrs {
		ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Afts().Ipv4Entry(fmt.Sprintf("%s/32", ep.prefix))

		watchFN := func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			entry, present := val.Val()
			t.Log(entry.GetPrefix())
			return present && entry.GetPrefix() == fmt.Sprintf("%s/32", ep.prefix) && entry.GetOriginProtocol() == oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
		}

		if got, ok := gnmi.Watch(t, dut, ipv4Path.State(), time.Minute, watchFN).Await(t); !ok {
			t.Errorf("Prefix not learnt: got %v, want %s", got, ep.prefix)
		}
		t.Logf("Prefix %s learnt by DUT...", ep.prefix)
	}
}

func validateV6PrefixesWithAftEntries(t *testing.T, dut *ondatra.DUTDevice, prefixAttrs []prefixAttributes) {
	for _, ep := range prefixAttrs {
		ipv4Path := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).
			Afts().Ipv6Entry(fmt.Sprintf("%s/128", ep.prefix))

		watchFN := func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			entry, present := val.Val()
			t.Log(entry.GetPrefix())
			return present && entry.GetPrefix() == fmt.Sprintf("%s/128", ep.prefix) && entry.GetOriginProtocol() == oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
		}

		if got, ok := gnmi.Watch(t, dut, ipv4Path.State(), time.Minute, watchFN).Await(t); !ok {
			t.Errorf("Prefix not learnt: got %v, want %s", got, ep.prefix)
		}
		t.Logf("Prefix %s learnt by DUT...", ep.prefix)
	}
}

func validateV6PrefixesWithAttributes(t *testing.T, ate *ondatra.ATEDevice, prefixAttrs []prefixAttributes) {
	t.Helper()
	for _, ep := range prefixAttrs {
		bgpPrefix := gnmi.Get(t, ate.OTG(), gnmi.OTG().BgpPeer(ep.bgpPeer).UnicastIpv6Prefix(ep.prefix, 128, otgtelemetry.UnicastIpv6Prefix_Origin_IGP, 0).State())

		// Validate Communities
		if len(ep.expectedCommunities) > 0 {
			for _, expectedComm := range ep.expectedCommunities {
				found := false
				for _, actualComm := range bgpPrefix.Community {
					if actualComm.GetCustomAsNumber() == expectedComm.ASNumber &&
						actualComm.GetCustomAsValue() == expectedComm.Value {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("prefix %s: Expected community (%d,%d) not found", ep.prefix, expectedComm.ASNumber, expectedComm.Value)
				}
			}
		}

		// Validate AS Path
		if len(ep.expectedasPath) > 0 {
			actualAS := make([]uint32, 0)
			for _, seg := range bgpPrefix.AsPath {
				actualAS = append(actualAS, seg.AsNumbers...)
			}
			if len(actualAS) != len(ep.expectedasPath) {
				t.Errorf("prefix %s: AS Path length mismatch. Got %d, want %d", ep.prefix, len(actualAS), len(ep.expectedasPath))
			} else {
				for i := range actualAS {
					if actualAS[i] != ep.expectedasPath[i] {
						t.Errorf("prefix %s: ASPath[%d] mismatch. Got %d, want %d", ep.prefix, i, actualAS[i], ep.expectedasPath[i])
					}
				}
			}
		}

		// Validate MED
		if ep.expectedmed != nil {
			actualMED := bgpPrefix.GetMultiExitDiscriminator()
			if actualMED != *ep.expectedmed {
				t.Errorf("prefix %s: MED mismatch. Got %d, want %d", ep.prefix, actualMED, *ep.expectedmed)
			}
		}
	}

}

func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, match bool) error {
	t.Helper()
	t.Logf("=== TRAFFIC FLOW VALIDATION START (expecting match=%v) ===", match)

	otg := ate.OTG()

	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	otg.StopTraffic(t)

	otgutils.LogPortMetrics(t, otg, config)
	otgutils.LogFlowMetrics(t, otg, config)

	for _, flow := range config.Flows().Items() {
		outPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))
		lossPct := ((outPkts - inPkts) * 100) / outPkts

		t.Logf("Flow %s: OutPkts=%v, InPkts=%v, LossPct=%v", flow.Name(), outPkts, inPkts, lossPct)

		if outPkts == 0 {
			return fmt.Errorf("outpkts for flow %s is 0, want > 0", flow.Name())
		}

		if match {
			// Expecting traffic to pass (0% loss)
			if got := lossPct; got > 0 {
				return fmt.Errorf("traffic validation FAILED: Flow %s has %v%% packet loss, want 0%%", flow.Name(), got)
			}
			t.Logf("Traffic validation PASSED: Flow %s has 0%% packet loss", flow.Name())
		} else {
			// Expecting traffic to fail (100% loss)
			if got := lossPct; got != 100 {
				return fmt.Errorf("traffic validation FAILED: Flow %s has %v%% packet loss, want 100%%", flow.Name(), got)
			}
			t.Logf("Traffic validation PASSED: Flow %s has 100%% packet loss", flow.Name())
		}
	}
	return nil
}

func ptrToUint32(val uint32) *uint32 {
	return &val
}

func mustValidateGracefulRestartHardResetCode6Subcode9(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, gracefulRestartHardResetValidation); err != nil {
		t.Fatalf("validation error: %v", err)
	}
}

func validateExrr(t *testing.T, flowsWithNoERR []string, flowsWithNoLoss []string, hardReset bool, params cfgplugins.BGPGracefulRestartConfig) {
	t.Helper()
	ate := ondatra.ATE(t, "ate")

	startTime := time.Now()
	t.Log("Sending packets while GR timer is counting down. Traffic should pass as BGP GR is enabled!")
	t.Logf("Time passed since graceful restart was initiated is %s", time.Since(startTime))
	waitDuration := time.Duration(params.GracefulRestartStaleRouteTime)*time.Second - time.Since(startTime) - 10*time.Second
	t.Logf("Waiting for %s short of stale route time expiration of %v", waitDuration, params.GracefulRestartStaleRouteTime)
	time.Sleep(waitDuration)
	ate.OTG().StopTraffic(t)
	dut := ondatra.DUT(t, "dut")

	if !deviations.ExrrStaleRouteTimeUnsupported(dut) {
		verifyNoPacketLoss(t, ate, append(flowsWithNoERR, flowsWithNoLoss...))
	}

	t.Logf("Time passed since graceful restart was initiated is %s", time.Since(startTime))
	if time.Since(startTime) < time.Duration(params.GracefulRestartStaleRouteTime)*time.Second {
		waitDuration = time.Duration(params.GracefulRestartStaleRouteTime)*time.Second - time.Since(startTime)
		t.Logf("Waiting another %s seconds to ensure the stale route timer of %v expired", waitDuration, params.GracefulRestartStaleRouteTime)
		time.Sleep(waitDuration)
	} else {
		t.Logf("Enough time passed to ensure the expiration of stale route timer of %v", params.GracefulRestartStaleRouteTime)
	}

	ate.OTG().StartTraffic(t)
	waitDuration = time.Duration(triggerGrTimer*time.Second - time.Duration(params.GracefulRestartStaleRouteTime)*time.Second - 5*time.Second)
	time.Sleep(waitDuration)
	ate.OTG().StopTraffic(t)

	if hardReset {
		confirmPacketLoss(t, ate, append(flowsWithNoERR, flowsWithNoLoss...))
	} else {
		confirmPacketLoss(t, ate, flowsWithNoERR)
		verifyNoPacketLoss(t, ate, flowsWithNoLoss)
	}

}

// setup function to configure ATE and DUT for BGP Graceful Restart test
func setup(t *testing.T, bgpGracefulRestartConfigParams cfgplugins.BGPGracefulRestartConfig) gosnappi.Config {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// ATE Configuration.
	t.Log("Start ATE Config")
	config := configureATE(t, ate, bgpGracefulRestartConfigParams)

	// Configure interface on the DUT
	t.Log("Start DUT interface Config")
	configureDUT(t, dut)
	configureRoutePolicy(t, dut)

	// Configure BGP+Neighbors on the DUT
	t.Log("Configure BGP with Graceful Restart option under Global Bgp")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	nbrList := buildNbrListebgp()
	dutConf := ebgpWithNbr(bgpGracefulRestartConfigParams, nbrList, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	nbrList = buildNbrListibgp()
	dutConf = ibgpWithNbr(bgpGracefulRestartConfigParams, nbrList, dut)
	gnmi.Update(t, dut, dutConfPath.Config(), dutConf)
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

	var src, dst attrs.Attributes
	var srcIPs, dstIPs, srcIPv6s, dstIPv6s []string
	config.Flows().Clear()
	modes := []string{"IBGP", "EBGP"}
	for _, mode := range modes {
		if mode == "EBGP" {
			src = ateIBGP
			dst = ateEBGP
			srcIPs = []string{ipv4Prefix1, ipv4Prefix2, ipv4Prefix3}
			dstIPs = []string{ipv4Prefix4, ipv4Prefix5, ipv4Prefix6}
			srcIPv6s = []string{ipv6Prefix1, ipv6Prefix2, ipv6Prefix3}
			dstIPv6s = []string{ipv6Prefix4, ipv6Prefix5, ipv6Prefix6}
		}
		if mode == "IBGP" {
			src = ateEBGP
			dst = ateIBGP
			srcIPs = []string{ipv4Prefix4, ipv4Prefix5, ipv4Prefix6}
			dstIPs = []string{ipv4Prefix1, ipv4Prefix2, ipv4Prefix3}
			srcIPv6s = []string{ipv6Prefix4, ipv6Prefix5, ipv6Prefix6}
			dstIPv6s = []string{ipv6Prefix1, ipv6Prefix2, ipv6Prefix3}
		}

		// Creating flows
		for i := 0; i <= 2; i++ {
			configureFlow(t, src, dst, srcIPs[i], dstIPs[i], config)
		}
		for i := 0; i <= 2; i++ {
			configureFlowV6(t, src, dst, srcIPv6s[i], dstIPv6s[i], config)
		}
	}
	return config
}

func TestBGPPGracefulRestartExtendedRouteRetention(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	bgpNeighbors := []string{
		ateIBGP.IPv4,
		ateEBGP.IPv4,
		ateIBGP.IPv6,
		ateEBGP.IPv6,
	}

	bgpPeerGroups := []string{
		"EBGP-PEER-GROUP-V4",
		"EBGP-PEER-GROUP-V6",
		"IBGP-PEER-GROUP-V4",
		"IBGP-PEER-GROUP-V6",
	}

	peers := []string{
		ateIBGP.Name + ".BGP4.peer",
		ateEBGP.Name + ".BGP4.peer",
		ateIBGP.Name + ".BGP6.peer",
		ateEBGP.Name + ".BGP6.peer",
	}

	bgpGracefulRestartConfigParams := cfgplugins.BGPGracefulRestartConfig{
		GracefulRestartTime:           grRestartTime,
		GracefulRestartStaleRouteTime: grStaleRouteTime,
		ERRetentionTime:               erRetentionTime,
		DutAS:                         dutAS,
		BgpNeighbors:                  bgpNeighbors,
		BgpPeerGroups:                 bgpPeerGroups,
		BgpPeers:                      peers,
	}

	config := setup(t, bgpGracefulRestartConfigParams)

	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)

	mustCheckBgpStatus(t, dut, routeCount)

	flowsWithNoERR := []string{fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix1, ipv4Prefix4),
		fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix4, ipv4Prefix1),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix1, ipv6Prefix4),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix1, ipv6Prefix4)}

	flowsWithNoLoss := []string{fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix2, ipv4Prefix5),
		fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix5, ipv4Prefix2),
		fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix3, ipv4Prefix6),
		fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix6, ipv4Prefix3),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix2, ipv6Prefix5),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix5, ipv6Prefix2),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix3, ipv6Prefix6),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix6, ipv6Prefix3)}

	type testCase struct {
		name string
		fn   func(t *testing.T)
	}

	cases := []testCase{
		{
			name: "1_BaseLine_Validation",
			fn: func(t *testing.T) {

				t.Log("verify BGP Graceful Restart settings")
				err := mustCheckBgpGRConfig(t, bgpGracefulRestartConfigParams, dut)
				if err != nil {
					t.Fatalf("checkBgpGRConfig failed: %v", err)
				}

				validateV4PrefixesWithAftEntries(t, dut, prefixAttrs)
				validateV6PrefixesWithAftEntries(t, dut, prefixV6Attrs)
				validatePrefixesWithAttributes(t, ate, prefixAttrs)
				validateV6PrefixesWithAttributes(t, ate, prefixV6Attrs)
				if err := validateTrafficFlows(t, ate, config, true); err != nil {
					t.Fatalf("validateTrafficFlows failed: %v", err)
				}

				bgpGracefulRestartConfigParamsERRLarge := cfgplugins.BGPGracefulRestartConfig{
					GracefulRestartTime:           grRestartTime,
					GracefulRestartStaleRouteTime: grStaleRouteTime,
					ERRetentionTime:               erRetentionTimeLarge,
					DutAS:                         dutAS,
					BgpNeighbors:                  bgpNeighbors,
				}
				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, false, bgpGracefulRestartConfigParamsERRLarge)
				b.Set(t, dut)
			},
		},
		{
			name: "2_DUT_as_Helper_for_graceful_Restart",
			fn: func(t *testing.T) {
				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, false, bgpGracefulRestartConfigParams)
				b.Set(t, dut)
				ate.OTG().StartTraffic(t)

				t.Log("Send Graceful Restart Trigger from OTG to DUT")
				ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "none"))

				validateExrr(t, flowsWithNoERR, flowsWithNoLoss, false, bgpGracefulRestartConfigParams)

				// Check Routes are re-learnt after graceful-restart completes
				mustCheckBgpStatus(t, dut, routeCount)

				mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
			},
		},
		{
			name: "3_ATE_Peer_Abrupt_Termination",
			fn: func(t *testing.T) {

				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, false, bgpGracefulRestartConfigParams)
				b.Set(t, dut)
				ate.OTG().StartTraffic(t)

				t.Log("Stop BGP on the ATE Peer")
				stopBgp := gosnappi.NewControlState()
				stopBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
					SetState(gosnappi.StateProtocolBgpPeersState.DOWN)
				ate.OTG().SetControlState(t, stopBgp)

				validateExrr(t, flowsWithNoERR, flowsWithNoLoss, false, bgpGracefulRestartConfigParams)

				t.Log("Start BGP on the ATE Peer")
				startBgp := gosnappi.NewControlState()
				startBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
					SetState(gosnappi.StateProtocolBgpPeersState.UP)
				ate.OTG().SetControlState(t, startBgp)

				mustCheckBgpStatus(t, dut, routeCount)

				mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
			},
		},
		{
			name: "4_Administrative_Reset_Notification_Sent_By_DUT_Graceful",
			fn: func(t *testing.T) {

				if !deviations.GnoiBgpGracefulRestartUnsupported(dut) {
					b := new(gnmi.SetBatch)
					cfgplugins.ApplyExtendedRouteRetention(t, dut, b, false, bgpGracefulRestartConfigParams)
					b.Set(t, dut)

					ate.OTG().StartTraffic(t)

					gnoiClient := dut.RawAPIs().GNOI(t)
					bgpReq := &bpb.ClearBGPNeighborRequest{
						Mode: bpb.ClearBGPNeighborRequest_GRACEFUL_RESET,
					}

					if _, err := gnoiClient.BGP().ClearBGPNeighbor(context.Background(), bgpReq); err != nil {
						t.Fatalf("Failed to clear BGP neighbor: %v", err)
					}

					validateExrr(t, flowsWithNoERR, flowsWithNoLoss, false, bgpGracefulRestartConfigParams)

					mustCheckBgpStatus(t, dut, routeCount)

					mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
				}
			},
		},
		{
			name: "5_Administrative_Reset_Notification_Received_By_DUT_Graceful",
			fn: func(t *testing.T) {

				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, false, bgpGracefulRestartConfigParams)
				b.Set(t, dut)
				ate.OTG().StartTraffic(t)

				t.Log("Send Graceful Restart Trigger from OTG to DUT")
				ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "soft"))

				validateExrr(t, flowsWithNoERR, flowsWithNoLoss, false, bgpGracefulRestartConfigParams)

				// Check Routes are re-learnt after graceful-restart completes
				mustCheckBgpStatus(t, dut, routeCount)

				mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
			},
		},
		{
			name: "6_Administrative_Reset_Notification_Sent_By_DUT_Hard_Reset",
			fn: func(t *testing.T) {

				if !deviations.GnoiBgpGracefulRestartUnsupported(dut) {
					b := new(gnmi.SetBatch)
					cfgplugins.ApplyExtendedRouteRetention(t, dut, b, false, bgpGracefulRestartConfigParams)
					b.Set(t, dut)

					cs := packetvalidationhelpers.StartCapture(t, ate)

					ate.OTG().StartTraffic(t)

					gnoiClient := dut.RawAPIs().GNOI(t)
					bgpReq := &bpb.ClearBGPNeighborRequest{
						Mode: bpb.ClearBGPNeighborRequest_HARD_RESET,
					}

					if _, err := gnoiClient.BGP().ClearBGPNeighbor(context.Background(), bgpReq); err != nil {
						t.Fatalf("Failed to clear BGP neighbor: %v", err)
					}

					packetvalidationhelpers.StopCapture(t, ate, cs)
					mustValidateGracefulRestartHardResetCode6Subcode9(t, ate)

					validateExrr(t, flowsWithNoERR, flowsWithNoLoss, true, bgpGracefulRestartConfigParams)

					mustCheckBgpStatus(t, dut, routeCount)

					mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
				}
			},
		},
		{
			name: "7_Administrative_Reset_Notification_Received_By_DUT_Hard_Reset",
			fn: func(t *testing.T) {

				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, false, bgpGracefulRestartConfigParams)
				b.Set(t, dut)
				ate.OTG().StartTraffic(t)

				t.Log("Send Graceful Restart Trigger from OTG to DUT")
				ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "hard"))

				validateExrr(t, flowsWithNoERR, flowsWithNoLoss, true, bgpGracefulRestartConfigParams)

				mustCheckBgpStatus(t, dut, routeCount)

				mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
			},
		},
		{
			name: "8_Additive_Policy_Application",
			fn: func(t *testing.T) {

				d := &oc.Root{}
				rp := d.GetOrCreateRoutingPolicy()

				importIBGP := rp.GetOrCreatePolicyDefinition("IMPORT-IBGP")
				importIBGPstmt10, err := importIBGP.AppendNewStatement("10")
				if err != nil {
					t.Errorf("error while creating new statement %v", err)
				}
				importIBGPstmt10.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
				importIBGPstmtCommunitySet := importIBGPstmt10.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
				importIBGPstmtCommunitySet.SetCommunitySet("TEST-IBGP")
				importIBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().SetSetLocalPref(200)
				importIBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().SetSetMed(oc.UnionUint32(med150))
				if deviations.BGPSetMedActionUnsupported(dut) {
					importIBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().SetSetMedAction(oc.BgpPolicy_BgpSetMedAction_SET)
				}

				gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, false, bgpGracefulRestartConfigParams)
				b.Set(t, dut)

				ate.OTG().StartTraffic(t)

				t.Log("Send Graceful Restart Trigger from OTG to DUT")
				ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "none"))

				validateExrr(t, flowsWithNoERR, flowsWithNoLoss, false, bgpGracefulRestartConfigParams)

				mustCheckBgpStatus(t, dut, routeCount)

				mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
			},
		},
		{
			name: "9_Default_Reject_Behavior",
			fn: func(t *testing.T) {

				b := new(gnmi.SetBatch)
				cfgplugins.DeleteExtendedRouteRetention(t, dut, b, false, bgpGracefulRestartConfigParams)
				b.Set(t, dut)

				t.Log("Stop BGP on the ATE Peer")
				stopBgp := gosnappi.NewControlState()
				stopBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
					SetState(gosnappi.StateProtocolBgpPeersState.DOWN)
				ate.OTG().SetControlState(t, stopBgp)

				mustCheckBgpStatusDown(t, dut)

				time.Sleep(triggerGrTimer * time.Second)

				sendTraffic(t, ate)
				confirmPacketLoss(t, ate, append(flowsWithNoERR, flowsWithNoLoss...))

				t.Log("Stop BGP on the ATE Peer")
				startBgp := gosnappi.NewControlState()
				startBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
					SetState(gosnappi.StateProtocolBgpPeersState.UP)

				ate.OTG().SetControlState(t, startBgp)
				mustCheckBgpStatus(t, dut, routeCount)

				mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
			},
		},
		{
			name: "10_Consecutive_BGP_Restarts",
			fn: func(t *testing.T) {

				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, false, bgpGracefulRestartConfigParams)
				b.Set(t, dut)

				for i := 0; i < 3; i++ {
					ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "soft"))
					time.Sleep(60 * time.Second)
				}

				mustCheckBgpStatus(t, dut, routeCount)

				ate.OTG().StartTraffic(t)
				t.Log("Stop BGP on the ATE Peer")
				stopBgp := gosnappi.NewControlState()
				stopBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
					SetState(gosnappi.StateProtocolBgpPeersState.DOWN)

				ate.OTG().SetControlState(t, stopBgp)

				validateExrr(t, flowsWithNoERR, flowsWithNoLoss, false, bgpGracefulRestartConfigParams)
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.fn(t)
		})
	}
}

func TestBGPPGracefulRestartExtendedRouteRetentionOnPeerGroup(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	bgpPeerGroups := []string{
		"EBGP-PEER-GROUP-V4",
		"EBGP-PEER-GROUP-V6",
		"IBGP-PEER-GROUP-V4",
		"IBGP-PEER-GROUP-V6",
	}

	peers := []string{
		ateIBGP.Name + ".BGP4.peer",
		ateEBGP.Name + ".BGP4.peer",
		ateIBGP.Name + ".BGP6.peer",
		ateEBGP.Name + ".BGP6.peer",
	}

	bgpGracefulRestartConfigParams := cfgplugins.BGPGracefulRestartConfig{
		GracefulRestartTime:           grRestartTime,
		GracefulRestartStaleRouteTime: grStaleRouteTime,
		ERRetentionTime:               erRetentionTime,
		DutAS:                         dutAS,
		BgpPeerGroups:                 bgpPeerGroups,
		BgpPeers:                      peers,
	}

	config := setup(t, bgpGracefulRestartConfigParams)

	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)

	mustCheckBgpStatus(t, dut, routeCount)

	flowsWithNoERR := []string{fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix1, ipv4Prefix4),
		fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix4, ipv4Prefix1),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix1, ipv6Prefix4),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix1, ipv6Prefix4)}

	flowsWithNoLoss := []string{fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix2, ipv4Prefix5),
		fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix5, ipv4Prefix2),
		fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix3, ipv4Prefix6),
		fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix6, ipv4Prefix3),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix2, ipv6Prefix5),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix5, ipv6Prefix2),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix3, ipv6Prefix6),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix6, ipv6Prefix3)}

	type testCase struct {
		name string
		fn   func(t *testing.T)
	}

	cases := []testCase{
		{
			name: "3_11_1_BaseLine_Validation",
			fn: func(t *testing.T) {

				t.Log("verify BGP Graceful Restart settings")
				err := mustCheckBgpGRConfig(t, bgpGracefulRestartConfigParams, dut)
				if err != nil {
					t.Fatalf("checkBgpGRConfig failed: %v", err)
				}

				validateV4PrefixesWithAftEntries(t, dut, prefixAttrs)
				validateV6PrefixesWithAftEntries(t, dut, prefixV6Attrs)
				validatePrefixesWithAttributes(t, ate, prefixAttrs)
				validateV6PrefixesWithAttributes(t, ate, prefixV6Attrs)
				if err := validateTrafficFlows(t, ate, config, true); err != nil {
					t.Fatalf("validateTrafficFlows failed: %v", err)
				}

				bgpGracefulRestartConfigParamsERRLarge := cfgplugins.BGPGracefulRestartConfig{
					GracefulRestartTime:           grRestartTime,
					GracefulRestartStaleRouteTime: grStaleRouteTime,
					ERRetentionTime:               erRetentionTimeLarge,
					DutAS:                         dutAS,
					BgpPeerGroups:                 bgpPeerGroups,
				}
				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, true, bgpGracefulRestartConfigParamsERRLarge)
				b.Set(t, dut)
			},
		},
		{
			name: "3_11_2_DUT_as_Helper_for_graceful_Restart",
			fn: func(t *testing.T) {
				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, true, bgpGracefulRestartConfigParams)
				b.Set(t, dut)
				ate.OTG().StartTraffic(t)

				t.Log("Send Graceful Restart Trigger from OTG to DUT")
				ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "none"))

				validateExrr(t, flowsWithNoERR, flowsWithNoLoss, false, bgpGracefulRestartConfigParams)

				// Check Routes are re-learnt after graceful-restart completes
				mustCheckBgpStatus(t, dut, routeCount)

				mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
			},
		},
		{
			name: "3_11_3_ATE_Peer_Abrupt_Termination",
			fn: func(t *testing.T) {

				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, true, bgpGracefulRestartConfigParams)
				b.Set(t, dut)
				ate.OTG().StartTraffic(t)

				t.Log("Stop BGP on the ATE Peer")
				stopBgp := gosnappi.NewControlState()
				stopBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
					SetState(gosnappi.StateProtocolBgpPeersState.DOWN)
				ate.OTG().SetControlState(t, stopBgp)

				validateExrr(t, flowsWithNoERR, flowsWithNoLoss, false, bgpGracefulRestartConfigParams)

				t.Log("Start BGP on the ATE Peer")
				startBgp := gosnappi.NewControlState()
				startBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
					SetState(gosnappi.StateProtocolBgpPeersState.UP)
				ate.OTG().SetControlState(t, startBgp)

				mustCheckBgpStatus(t, dut, routeCount)

				mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
			},
		},
		{
			name: "3_11_4_Administrative_Reset_Notification_Sent_By_DUT_Graceful",
			fn: func(t *testing.T) {

				if !deviations.GnoiBgpGracefulRestartUnsupported(dut) {
					b := new(gnmi.SetBatch)
					cfgplugins.ApplyExtendedRouteRetention(t, dut, b, true, bgpGracefulRestartConfigParams)
					b.Set(t, dut)

					ate.OTG().StartTraffic(t)

					gnoiClient := dut.RawAPIs().GNOI(t)
					bgpReq := &bpb.ClearBGPNeighborRequest{
						Mode: bpb.ClearBGPNeighborRequest_GRACEFUL_RESET,
					}

					if _, err := gnoiClient.BGP().ClearBGPNeighbor(context.Background(), bgpReq); err != nil {
						t.Fatalf("Failed to clear BGP neighbor: %v", err)
					}

					validateExrr(t, flowsWithNoERR, flowsWithNoLoss, false, bgpGracefulRestartConfigParams)

					mustCheckBgpStatus(t, dut, routeCount)

					mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
				}
			},
		},
		{
			name: "3_11_5_Administrative_Reset_Notification_Received_By_DUT_Graceful",
			fn: func(t *testing.T) {

				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, true, bgpGracefulRestartConfigParams)
				b.Set(t, dut)
				ate.OTG().StartTraffic(t)

				t.Log("Send Graceful Restart Trigger from OTG to DUT")
				ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "soft"))

				validateExrr(t, flowsWithNoERR, flowsWithNoLoss, false, bgpGracefulRestartConfigParams)

				// Check Routes are re-learnt after graceful-restart completes
				mustCheckBgpStatus(t, dut, routeCount)

				mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
			},
		},
		{
			name: "3_11_6_Administrative_Reset_Notification_Sent_By_DUT_Hard_Reset",
			fn: func(t *testing.T) {

				if !deviations.GnoiBgpGracefulRestartUnsupported(dut) {
					b := new(gnmi.SetBatch)
					cfgplugins.ApplyExtendedRouteRetention(t, dut, b, true, bgpGracefulRestartConfigParams)
					b.Set(t, dut)

					cs := packetvalidationhelpers.StartCapture(t, ate)

					ate.OTG().StartTraffic(t)

					gnoiClient := dut.RawAPIs().GNOI(t)
					bgpReq := &bpb.ClearBGPNeighborRequest{
						Mode: bpb.ClearBGPNeighborRequest_HARD_RESET,
					}

					if _, err := gnoiClient.BGP().ClearBGPNeighbor(context.Background(), bgpReq); err != nil {
						t.Fatalf("Failed to clear BGP neighbor: %v", err)
					}

					packetvalidationhelpers.StopCapture(t, ate, cs)
					mustValidateGracefulRestartHardResetCode6Subcode9(t, ate)

					validateExrr(t, flowsWithNoERR, flowsWithNoLoss, true, bgpGracefulRestartConfigParams)

					mustCheckBgpStatus(t, dut, routeCount)

					mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
				}
			},
		},
		{
			name: "3_11_7_Administrative_Reset_Notification_Received_By_DUT_Hard_Reset",
			fn: func(t *testing.T) {

				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, true, bgpGracefulRestartConfigParams)
				b.Set(t, dut)
				ate.OTG().StartTraffic(t)

				t.Log("Send Graceful Restart Trigger from OTG to DUT")
				ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "hard"))

				validateExrr(t, flowsWithNoERR, flowsWithNoLoss, true, bgpGracefulRestartConfigParams)

				mustCheckBgpStatus(t, dut, routeCount)

				mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
			},
		},
		{
			name: "3_11_8_Additive_Policy_Application",
			fn: func(t *testing.T) {

				d := &oc.Root{}
				rp := d.GetOrCreateRoutingPolicy()

				importIBGP := rp.GetOrCreatePolicyDefinition("IMPORT-IBGP")
				importIBGPstmt10, err := importIBGP.AppendNewStatement("10")
				if err != nil {
					t.Errorf("error while creating new statement %v", err)
				}
				importIBGPstmt10.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
				importIBGPstmtCommunitySet := importIBGPstmt10.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
				importIBGPstmtCommunitySet.SetCommunitySet("TEST-IBGP")
				importIBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().SetSetLocalPref(200)
				importIBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().SetSetMed(oc.UnionUint32(med150))
				if deviations.BGPSetMedActionUnsupported(dut) {
					importIBGPstmt10.GetOrCreateActions().GetOrCreateBgpActions().SetSetMedAction(oc.BgpPolicy_BgpSetMedAction_SET)
				}

				gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, true, bgpGracefulRestartConfigParams)
				b.Set(t, dut)

				ate.OTG().StartTraffic(t)

				t.Log("Send Graceful Restart Trigger from OTG to DUT")
				ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "none"))

				validateExrr(t, flowsWithNoERR, flowsWithNoLoss, false, bgpGracefulRestartConfigParams)

				mustCheckBgpStatus(t, dut, routeCount)

				mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
			},
		},
		{
			name: "3_11_9_Default_Reject_Behavior",
			fn: func(t *testing.T) {

				b := new(gnmi.SetBatch)
				cfgplugins.DeleteExtendedRouteRetention(t, dut, b, true, bgpGracefulRestartConfigParams)
				b.Set(t, dut)

				t.Log("Stop BGP on the ATE Peer")
				stopBgp := gosnappi.NewControlState()
				stopBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
					SetState(gosnappi.StateProtocolBgpPeersState.DOWN)
				ate.OTG().SetControlState(t, stopBgp)

				mustCheckBgpStatusDown(t, dut)

				time.Sleep(triggerGrTimer * time.Second)

				sendTraffic(t, ate)
				confirmPacketLoss(t, ate, append(flowsWithNoERR, flowsWithNoLoss...))

				t.Log("Stop BGP on the ATE Peer")
				startBgp := gosnappi.NewControlState()
				startBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
					SetState(gosnappi.StateProtocolBgpPeersState.UP)

				ate.OTG().SetControlState(t, startBgp)
				mustCheckBgpStatus(t, dut, routeCount)

				mustCheckEndOfRibReceived(t, ate, bgpGracefulRestartConfigParams)
			},
		},
		{
			name: "3_11_10_Consecutive_BGP_Restarts",
			fn: func(t *testing.T) {

				b := new(gnmi.SetBatch)
				cfgplugins.ApplyExtendedRouteRetention(t, dut, b, true, bgpGracefulRestartConfigParams)
				b.Set(t, dut)

				for i := 0; i < 3; i++ {
					ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "soft"))
					time.Sleep(60 * time.Second)
				}

				mustCheckBgpStatus(t, dut, routeCount)

				ate.OTG().StartTraffic(t)

				t.Log("Stop BGP on the ATE Peer")
				stopBgp := gosnappi.NewControlState()
				stopBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
					SetState(gosnappi.StateProtocolBgpPeersState.DOWN)

				ate.OTG().SetControlState(t, stopBgp)

				validateExrr(t, flowsWithNoERR, flowsWithNoLoss, false, bgpGracefulRestartConfigParams)
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.fn(t)
		})
	}
}
