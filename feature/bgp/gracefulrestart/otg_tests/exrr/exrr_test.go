package bgp_exrr_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	bpb "github.com/openconfig/gnoi/bgp"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	trafficDuration          = 30 * time.Second
	triggerGrTimer           = 280
	grRestartTime            = 220
	grStaleRouteTime         = 250
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
)

var (
	dutIBGP = attrs.Attributes{
		Desc:    "DUT to port2 ATE iBGP peer",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateIBGP = attrs.Attributes{
		Name:    "ateIBGP",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutEBGP = attrs.Attributes{
		Desc:    "DUT to port1 ATE eBGP peer",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateEBGP = attrs.Attributes{
		Name:    "ateEBGP",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
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

var communityconf = []communityConfig{
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
}

var prefixattrs = []prefixAttributes{
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
			{65535, 6},
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
	},
}

var prefixv6attrs = []prefixAttributes{
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
			{65535, 6},
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
		advertisedbgpPeer: ateEBGP.Name + ".BGP6.peer",
		configuredCommunities: []struct {
			ASNumber uint16
			Value    uint16
		}{
			{testebgpasn, 1},
		},
		bgpPeer:     ateIBGP.Name + ".BGP6.peer",
		expectedmed: ptrToUint32(50),
	},
}

func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()

	for _, cfg := range communityconf {
		communitySet := rp.GetOrCreateDefinedSets().
			GetOrCreateBgpDefinedSets().
			GetOrCreateCommunitySet(cfg.name)

		communitySet.SetCommunityMember([]oc.RoutingPolicy_DefinedSets_BgpDefinedSets_CommunitySet_CommunityMember_Union{
			oc.UnionString(fmt.Sprintf("%d:%d", cfg.asn, cfg.val)),
		})
	}

	pd := rp.GetOrCreatePolicyDefinition("STALE-ROUTE-POLICY")
	stmt10, err := pd.AppendNewStatement("10")
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	stmt10.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	matchCommunitySet := stmt10.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
	matchCommunitySet.SetCommunitySet("NO-ERR")

	stmt20, err := pd.AppendNewStatement("20")
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	stmt20.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	matchCommunitySet2 := stmt20.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
	matchCommunitySet2.SetCommunitySet("ERR-NO-DEPREF")

	stmt30, err := pd.AppendNewStatement("30")
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	stmt30.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	matchCommunitySet3 := stmt30.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
	matchCommunitySet3.SetCommunitySet("STALE")

	stmt40, err := pd.AppendNewStatement("40")
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	stmt40.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	appendAS := rp.GetOrCreatePolicyDefinition("APPENDAS")
	appendASstmt1, err := appendAS.AppendNewStatement("10")
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	appendASstmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	appendASstmtCommunitySet := appendASstmt1.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
	appendASstmtCommunitySet.SetCommunitySet("TEST-IBGP")
	appendASstmt1.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend().SetAsn(100)
	appendASstmt1.GetOrCreateActions().GetOrCreateBgpActions().GetOrCreateSetAsPathPrepend().SetRepeatN(2)

	appendASstmt2, err := appendAS.AppendNewStatement("20")
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	appendASstmt2.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	newIBGP := rp.GetOrCreatePolicyDefinition("NEW-IBGP")
	newIBGPstmt, err := newIBGP.AppendNewStatement("10")
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	newIBGPstmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	setLP := rp.GetOrCreatePolicyDefinition("SET-LP")
	setLPstmt, err := setLP.AppendNewStatement("10")
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	setLPstmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	setLPstmtCommunitySet := setLPstmt.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
	setLPstmtCommunitySet.SetCommunitySet("TEST-IBGP")
	setLPstmt.GetOrCreateActions().GetOrCreateBgpActions().SetSetLocalPref(200)

	setLPstmt1, err := setLP.AppendNewStatement("20")
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	setLPstmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	setMED := rp.GetOrCreatePolicyDefinition("SET-MED")
	setMEDstmt, err := setMED.AppendNewStatement("10")
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	setMEDstmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
	setMEDstmtCommunitySet := setMEDstmt.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
	setMEDstmtCommunitySet.SetCommunitySet("TEST-EBGP")
	setMEDstmt.GetOrCreateActions().GetOrCreateBgpActions().SetSetMed(oc.UnionUint32(50))
	if !deviations.BGPSetMedActionUnsupported(dut) {
		setMEDstmt.GetOrCreateActions().GetOrCreateBgpActions().SetSetMedAction(oc.BgpPolicy_BgpSetMedAction_SET)
	}

	setMEDstmt1, err := setMED.AppendNewStatement("20")
	if err != nil {
		t.Errorf("Error while creating new statement %v", err)
	}
	setMEDstmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// configureDUT configures all the interfaces and network instance on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	i1 := dutEBGP.NewOCInterface(dut.Port(t, "port1").Name(), dut)
	gnmi.Replace(t, dut, dc.Interface(i1.GetName()).Config(), i1)

	i2 := dutIBGP.NewOCInterface(dut.Port(t, "port2").Name(), dut)
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

func ebgpWithNbr(as uint32, nbrs []*bgpNeighbor, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(as)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	g.RouterId = ygot.String(dutEBGP.IPv4)
	bgpgr := g.GetOrCreateGracefulRestart()
	bgpgr.Enabled = ygot.Bool(true)
	bgpgr.SetRestartTime(grRestartTime)
	bgpgr.SetStaleRoutesTime(grStaleRouteTime)

	pg := bgp.GetOrCreatePeerGroup(epeerv4GrpName)
	pg.PeerGroupName = ygot.String(epeerv4GrpName)

	pgV6 := bgp.GetOrCreatePeerGroup(epeerv6GrpName)
	pgV6.PeerGroupName = ygot.String(epeerv6GrpName)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{"APPENDAS"})
		rpl.SetImportPolicy([]string{"SET-MED"})
		rplv6 := pgV6.GetOrCreateApplyPolicy()
		rplv6.SetExportPolicy([]string{"APPENDAS"})
		rplv6.SetImportPolicy([]string{"SET-MED"})

	} else {
		pg1af4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg1af4.Enabled = ygot.Bool(true)

		pg1rpl4 := pg1af4.GetOrCreateApplyPolicy()
		pg1rpl4.SetExportPolicy([]string{"APPENDAS"})
		pg1rpl4.SetImportPolicy([]string{"SET-MED"})

		pg1af6 := pgV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pg1af6.Enabled = ygot.Bool(true)
		pg1rpl6 := pg1af6.GetOrCreateApplyPolicy()
		pg1rpl6.SetExportPolicy([]string{"APPENDAS"})
		pg1rpl6.SetImportPolicy([]string{"SET-MED"})
	}

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerGroup = ygot.String(epeerv4GrpName)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			nv4.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED})
			af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.PeerGroup = ygot.String(epeerv6GrpName)
			nv6.PeerAs = ygot.Uint32(nbr.as)
			nv6.Enabled = ygot.Bool(true)
			nv6.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED})
			nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			af4 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
		}
	}
	return niProto
}

func ibgpWithNbr(as uint32, nbrs []*bgpNeighbor, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(as)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	g.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	g.RouterId = ygot.String(dutEBGP.IPv4)
	bgpgr := g.GetOrCreateGracefulRestart()
	bgpgr.Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup(ipeerv4GrpName)
	pg.PeerGroupName = ygot.String(ipeerv4GrpName)

	pgV6 := bgp.GetOrCreatePeerGroup(ipeerv6GrpName)
	pgV6.PeerGroupName = ygot.String(ipeerv6GrpName)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{"NEW-IBGP"})
		rpl.SetImportPolicy([]string{"SET-LP"})
		rplv6 := pgV6.GetOrCreateApplyPolicy()
		rplv6.SetExportPolicy([]string{"NEW-IBGP"})
		rplv6.SetImportPolicy([]string{"SET-LP"})

	} else {
		pg1af4 := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pg1af4.Enabled = ygot.Bool(true)

		pg1rpl4 := pg1af4.GetOrCreateApplyPolicy()
		pg1rpl4.SetExportPolicy([]string{"NEW-IBGP"})
		pg1rpl4.SetImportPolicy([]string{"SET-LP"})

		pg1af6 := pgV6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		pg1af6.Enabled = ygot.Bool(true)
		pg1rpl6 := pg1af6.GetOrCreateApplyPolicy()
		pg1rpl6.SetExportPolicy([]string{"NEW-IBGP"})
		pg1rpl6.SetImportPolicy([]string{"SET-LP"})
	}

	for _, nbr := range nbrs {
		if nbr.isV4 {
			nv4 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv4.PeerGroup = ygot.String(ipeerv4GrpName)
			nv4.PeerAs = ygot.Uint32(nbr.as)
			nv4.Enabled = ygot.Bool(true)
			nv4.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED})
			af4 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(true)
			af6 := nv4.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(false)
		} else {
			nv6 := bgp.GetOrCreateNeighbor(nbr.neighborip)
			nv6.PeerGroup = ygot.String(ipeerv6GrpName)
			nv6.PeerAs = ygot.Uint32(nbr.as)
			nv6.Enabled = ygot.Bool(true)
			nv6.SetSendCommunityType([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED})
			nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
			af6.Enabled = ygot.Bool(true)
			af4 := nv6.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			af4.Enabled = ygot.Bool(false)
		}
	}
	return niProto
}

func checkBgpGRConfig(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP configuration")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	isGrEnabled := gnmi.Get(t, dut, statePath.Global().GracefulRestart().Enabled().State())
	t.Logf("isGrEnabled %v", isGrEnabled)
	if isGrEnabled {
		t.Logf("Graceful restart on neighbor %v enabled as Expected", ateEBGP.IPv4)
	} else {
		t.Errorf("Expected Graceful restart status on neighbor: got %v, want Enabled", isGrEnabled)
	}

	grTimerVal := gnmi.Get(t, dut, statePath.Global().GracefulRestart().RestartTime().State())
	t.Logf("grTimerVal %v", grTimerVal)
	if grTimerVal == uint16(grRestartTime) {
		t.Logf("Graceful restart timer enabled as expected to be %v", grRestartTime)
	} else {
		t.Errorf("Expected Graceful restart timer: got %v, want %v", grTimerVal, grRestartTime)
	}

	grStaleRouteTimeVal := gnmi.Get(t, dut, statePath.Global().GracefulRestart().StaleRoutesTime().State())
	t.Logf("grStaleRouteTimeVal %v", grStaleRouteTimeVal)
	if grStaleRouteTimeVal == uint16(grStaleRouteTime) {
		t.Logf("Graceful restart Stale Route timer enabled as expected to be %v", grStaleRouteTime)
	} else {
		t.Errorf("Expected Graceful restart timer: got %v, want %v", grStaleRouteTimeVal, grStaleRouteTime)
	}
}

func checkBgpStatus(t *testing.T, dut *ondatra.DUTDevice, routeCount uint32) {
	t.Log("Verifying BGP state")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, attr := range []attrs.Attributes{ateEBGP, ateIBGP} {

		nbrPath := statePath.Neighbor(attr.IPv4)
		nbrPathv6 := statePath.Neighbor(attr.IPv6)

		// Get BGP adjacency state
		t.Logf("Waiting for BGP neighbor %s to establish", attr.IPv4)
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			t.Logf("current state is %s", currState)
			return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatal("No BGP neighbor formed...")
		}

		// Get BGPv6 adjacency state
		t.Logf("Waiting for BGPv6 neighbor %s to establish", attr.IPv6)
		_, ok = gnmi.Watch(t, dut, nbrPathv6.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			t.Logf("current state is %s", currState)
			return ok && currState == oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, dut, nbrPathv6.State()))
			t.Fatal("No BGPv6 neighbor formed...")
		}

		t.Log("Waiting for BGP v4 prefixes to be installed")
		got, found := gnmi.Watch(t, dut, nbrPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes().Installed().State(), 180*time.Second, func(val *ygnmi.Value[uint32]) bool {
			prefixCount, ok := val.Val()
			return ok && prefixCount == routeCount
		}).Await(t)
		if !found {
			t.Errorf("Installed prefixes v4 mismatch: got %v, want %v", got, routeCount)
		}
		t.Log("Waiting for BGP v6 prefixes to be installed")
		got, found = gnmi.Watch(t, dut, nbrPathv6.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes().Installed().State(), 180*time.Second, func(val *ygnmi.Value[uint32]) bool {
			prefixCount, ok := val.Val()
			return ok && prefixCount == routeCount
		}).Await(t)
		if !found {
			t.Errorf("Installed prefixes v6 mismatch: got %v, want %v", got, routeCount)
		}
	}
}

func checkBgpStatusDown(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP state down")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	for _, attr := range []attrs.Attributes{ateEBGP, ateIBGP} {

		nbrPath := statePath.Neighbor(attr.IPv4)
		nbrPathv6 := statePath.Neighbor(attr.IPv6)

		// Get BGP adjacency state
		t.Logf("Waiting for BGP neighbor %s to establish", attr.IPv4)
		_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			t.Logf("current state is %s", currState)
			return ok && currState != oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
			t.Fatal("BGP neighbor not down")
		}

		// Get BGPv6 adjacency state
		t.Logf("Waiting for BGPv6 neighbor %s to establish", attr.IPv6)
		_, ok = gnmi.Watch(t, dut, nbrPathv6.SessionState().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			currState, ok := val.Val()
			t.Logf("current state is %s", currState)
			return ok && currState != oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t)
		if !ok {
			fptest.LogQuery(t, "BGPv6 reported state", nbrPathv6.State(), gnmi.Get(t, dut, nbrPathv6.State()))
			t.Fatal("BGP neighbor not down")
		}

	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")
	ateEBGP.AddToOTG(config, p1, &dutEBGP)
	ateIBGP.AddToOTG(config, p2, &dutIBGP)

	iBGPDev := config.Devices().Items()[1]
	iBGPEth := iBGPDev.Ethernets().Items()[0]
	iBGPIpv4 := iBGPEth.Ipv4Addresses().Items()[0]
	iBGPIpv6 := iBGPEth.Ipv6Addresses().Items()[0]
	eBGPDev := config.Devices().Items()[0]
	eBGPEth := eBGPDev.Ethernets().Items()[0]
	eBGPIpv4 := eBGPEth.Ipv4Addresses().Items()[0]
	eBGPIpv6 := eBGPEth.Ipv6Addresses().Items()[0]

	iBGP := iBGPDev.Bgp().SetRouterId(iBGPIpv4.Address())
	iBGP4Peer := iBGP.Ipv4Interfaces().Add().SetIpv4Name(iBGPIpv4.Name()).Peers().Add().SetName(ateIBGP.Name + ".BGP4.peer")
	iBGP4Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(grRestartTime)
	iBGP4Peer.SetPeerAddress(iBGPIpv4.Gateway()).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	iBGP4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	iBGP6Peer := iBGP.Ipv6Interfaces().Add().SetIpv6Name(iBGPIpv6.Name()).Peers().Add().SetName(ateIBGP.Name + ".BGP6.peer")
	iBGP6Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(grRestartTime)
	iBGP6Peer.SetPeerAddress(iBGPIpv6.Gateway()).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	iBGP6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	eBGP := eBGPDev.Bgp().SetRouterId(eBGPIpv4.Address())
	eBGP4Peer := eBGP.Ipv4Interfaces().Add().SetIpv4Name(eBGPIpv4.Name()).Peers().Add().SetName(ateEBGP.Name + ".BGP4.peer")
	eBGP4Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(grRestartTime)
	eBGP4Peer.SetPeerAddress(eBGPIpv4.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	eBGP4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)
	eBGP6Peer := eBGP.Ipv6Interfaces().Add().SetIpv6Name(eBGPIpv6.Name()).Peers().Add().SetName(ateEBGP.Name + ".BGP6.peer")
	eBGP6Peer.GracefulRestart().SetEnableGr(true).SetRestartTime(grRestartTime)
	eBGP6Peer.SetPeerAddress(eBGPIpv6.Gateway()).SetAsNumber(ateAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	eBGP6Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

	// Loop through prefixattrs and configure routes
	for i, attr := range prefixattrs {
		var peer gosnappi.BgpV4Peer
		var nexthop gosnappi.DeviceIpv4
		if attr.advertisedbgpPeer == ateIBGP.Name+".BGP4.peer" {
			peer = iBGP4Peer
			nexthop = iBGPIpv4
		} else {
			peer = eBGP4Peer
			nexthop = eBGPIpv4
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

	// Loop through prefixattrs and configure routes
	for i, attr := range prefixv6attrs {
		var peerv6 gosnappi.BgpV6Peer
		var nexthop gosnappi.DeviceIpv6
		if attr.advertisedbgpPeer == ateIBGP.Name+".BGP6.peer" {
			peerv6 = iBGP6Peer
			nexthop = iBGPIpv6
		} else {
			peerv6 = eBGP6Peer
			nexthop = eBGPIpv6
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

	// ATE Traffic Configuration
	t.Logf("start ate Traffic config")
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

	// ATE Traffic Configuration
	t.Logf("start ate Traffic config")
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

func verifyNoPacketLoss(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	c := otg.FetchConfig(t)
	otgutils.LogFlowMetrics(t, otg, c)
	for _, f := range c.Flows().Items() {
		t.Logf("Verifying flow metrics for flow %s\n", f.Name())
		recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(f.Name()).State())
		txPackets := float32(recvMetric.GetCounters().GetOutPkts())
		rxPackets := float32(recvMetric.GetCounters().GetInPkts())
		lostPackets := txPackets - rxPackets
		if txPackets == 0 {
			t.Fatalf("Tx packets should be higher than 0 for flow %s", f.Name())
		}
		if lossPct := lostPackets * 100 / txPackets; lossPct < 5.0 {
			t.Logf("Traffic Test Passed! Got %v loss", lossPct)
		} else {
			t.Errorf("Traffic Loss Pct for Flow %s: got %f", f.Name(), lossPct)
		}
	}
}

func confirmPacketLoss(t *testing.T, ate *ondatra.ATEDevice, flows []string) {
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
			t.Logf("Traffic Test Passed! Loss seen as expected: got %v, want 100%% ", lossPct)
		} else {
			t.Errorf("Traffic %s is expected to fail: got %f, want 100%% failure", f, lossPct)
		}
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	t.Logf("Starting traffic")
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	t.Logf("Stop traffic")
	ate.OTG().StopTraffic(t)
}

// createGracefulRestartAction create a bgp control action for initiating the graceful restart process
func createGracefulRestartAction(t *testing.T, peerNames []string, restartDelay uint32, notification string) gosnappi.ControlAction {
	t.Helper()
	grAction := gosnappi.NewControlAction()
	if notification == "soft" {
		grAction.Protocol().Bgp().InitiateGracefulRestart().
			SetPeerNames(peerNames).SetRestartDelay(restartDelay).Notification().Cease().SetSubcode(gosnappi.DeviceBgpCeaseErrorSubcode.ADMIN_RESET_CODE6_SUBCODE4)
	}
	if notification == "hard" {
		grAction.Protocol().Bgp().InitiateGracefulRestart().
			SetPeerNames(peerNames).SetRestartDelay(restartDelay).Notification().Cease().SetSubcode(gosnappi.DeviceBgpCeaseErrorSubcode.HARD_RESET_CODE6_SUBCODE9)
	}
	if notification == "none" {
		grAction.Protocol().Bgp().InitiateGracefulRestart().
			SetPeerNames(peerNames).SetRestartDelay(restartDelay)
	}
	return grAction
}

func validatePrefixesWithAttributes(t *testing.T, ate *ondatra.ATEDevice, prefixattrs []prefixAttributes) {

	for _, ep := range prefixattrs {
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
					t.Errorf("Prefix %s: Expected community (%d,%d) not found", ep.prefix, expectedComm.ASNumber, expectedComm.Value)
				}
			}
		}

		// Validate AS Path
		if len(ep.expectedasPath) > 0 {
			for i, as := range bgpPrefix.AsPath {
				if len(as.AsNumbers) != len(ep.expectedasPath) {
					t.Errorf("Prefix %s: AS Path length mismatch. Got %d, want %d", ep.prefix, len(bgpPrefix.AsPath), len(ep.expectedasPath))
				} else {
					if as.AsNumbers[i] != ep.expectedasPath[i] {
						t.Errorf("Prefix %s: ASPath[%d] mismatch. Got %d, want %d", ep.prefix, i, as.AsNumbers[i], ep.expectedasPath[i])
					}
				}
			}
		}

		// Validate MED
		if ep.expectedmed != nil {
			actualMED := bgpPrefix.GetMultiExitDiscriminator()
			if actualMED != *ep.expectedmed {
				t.Errorf("Prefix %s: MED mismatch. Got %d, want %d", ep.prefix, actualMED, *ep.expectedmed)
			}
		}
	}

}

func validateV6PrefixesWithAttributes(t *testing.T, ate *ondatra.ATEDevice, prefixattrs []prefixAttributes) {

	for _, ep := range prefixattrs {
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
					t.Errorf("Prefix %s: Expected community (%d,%d) not found", ep.prefix, expectedComm.ASNumber, expectedComm.Value)
				}
			}
		}

		// Validate AS Path
		if len(ep.expectedasPath) > 0 {
			for i, as := range bgpPrefix.AsPath {
				if len(as.AsNumbers) != len(ep.expectedasPath) {
					t.Errorf("Prefix %s: AS Path length mismatch. Got %d, want %d", ep.prefix, len(bgpPrefix.AsPath), len(ep.expectedasPath))
				} else {
					if as.AsNumbers[i] != ep.expectedasPath[i] {
						t.Errorf("Prefix %s: ASPath[%d] mismatch. Got %d, want %d", ep.prefix, i, as.AsNumbers[i], ep.expectedasPath[i])
					}
				}
			}
		}

		// Validate MED
		if ep.expectedmed != nil {
			actualMED := bgpPrefix.GetMultiExitDiscriminator()
			if actualMED != *ep.expectedmed {
				t.Errorf("Prefix %s: MED mismatch. Got %d, want %d", ep.prefix, actualMED, *ep.expectedmed)
			}
		}
	}

}

func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, match bool) {
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
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow.Name())
		}

		if match {
			// Expecting traffic to pass (0% loss)
			if got := lossPct; got > 0 {
				t.Fatalf("Traffic validation FAILED: Flow %s has %v%% packet loss, want 0%%", flow.Name(), got)
			} else {
				t.Logf("Traffic validation PASSED: Flow %s has 0%% packet loss", flow.Name())
			}
		} else {
			// Expecting traffic to fail (100% loss)
			if got := lossPct; got != 100 {
				t.Fatalf("Traffic validation FAILED: Flow %s has %v%% packet loss, want 100%%", flow.Name(), got)
			} else {
				t.Logf("Traffic validation PASSED: Flow %s has 100%% packet loss", flow.Name())
			}
		}
	}
}

func ptrToUint32(val uint32) *uint32 {
	return &val
}

func validateExrr(t *testing.T, flowsWithNoERR []string) {

	ate := ondatra.ATE(t, "ate")

	startTime := time.Now()
	t.Log("Sending packets while GR timer is counting down. Traffic should pass as BGP GR is enabled!")
	t.Logf("Time passed since graceful restart was initiated is %s", time.Since(startTime))
	waitDuration := grStaleRouteTime*time.Second - time.Since(startTime) - 10*time.Second
	t.Logf("Waiting for %s short of stale route time expiration of %v", waitDuration, grStaleRouteTime)
	time.Sleep(waitDuration)
	ate.OTG().StopTraffic(t)
	// Test verification fails because of https://partnerissuetracker.corp.google.com/issues/439825838
	t.Run("Verify No Packet Loss for ", func(t *testing.T) {
		verifyNoPacketLoss(t, ate)
	})

	t.Logf("Time passed since graceful restart was initiated is %s", time.Since(startTime))
	if time.Since(startTime) < time.Duration(grStaleRouteTime)*time.Second {
		waitDuration = time.Duration(grStaleRouteTime)*time.Second - time.Since(startTime) + 5*time.Second
		t.Logf("Waiting another %s seconds to ensure the stale route timer of %v expired", waitDuration, grStaleRouteTime)
		time.Sleep(waitDuration)
	} else {
		t.Logf("Enough time passed to ensure the expiration of stale route timer of %v", grStaleRouteTime)
	}

	ate.OTG().StartTraffic(t)
	waitDuration = time.Duration(triggerGrTimer*time.Second - grStaleRouteTime*time.Second - 5*time.Second)
	time.Sleep(waitDuration)
	ate.OTG().StopTraffic(t)

	t.Run("Confirm Packet Loss for ", func(t *testing.T) {
		confirmPacketLoss(t, ate, flowsWithNoERR)
	})

}

func TestBGPPGracefulRestartExtendedRouteRetention(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// ATE Configuration.
	t.Log("Start ATE Config")
	config := configureATE(t, ate)

	// ate.OTG().PushConfig(t, config)

	// Configure interface on the DUT
	t.Log("Start DUT interface Config")
	configureDUT(t, dut)
	configureRoutePolicy(t, dut)

	// Configure BGP+Neighbors on the DUT
	t.Log("Configure BGP with Graceful Restart option under Global Bgp")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	nbrList := buildNbrListebgp()
	dutConf := ebgpWithNbr(dutAS, nbrList, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	nbrList = buildNbrListibgp()
	dutConf = ibgpWithNbr(dutAS, nbrList, dut)
	gnmi.Update(t, dut, dutConfPath.Config(), dutConf)
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

	var src, dst attrs.Attributes
	var srcips, dstips, srcipv6s, dstipv6s []string
	config.Flows().Clear()
	modes := []string{"IBGP", "EBGP"}
	for _, mode := range modes {
		if mode == "EBGP" {
			src = ateIBGP
			dst = ateEBGP
			srcips = []string{ipv4Prefix1, ipv4Prefix2, ipv4Prefix3}
			dstips = []string{ipv4Prefix4, ipv4Prefix5, ipv4Prefix6}
			srcipv6s = []string{ipv6Prefix1, ipv6Prefix2, ipv6Prefix3}
			dstipv6s = []string{ipv6Prefix4, ipv6Prefix5, ipv6Prefix6}
		}
		if mode == "IBGP" {
			src = ateEBGP
			dst = ateIBGP
			srcips = []string{ipv4Prefix4, ipv4Prefix5, ipv4Prefix6}
			dstips = []string{ipv4Prefix1, ipv4Prefix2, ipv4Prefix3}
			srcipv6s = []string{ipv6Prefix4, ipv6Prefix5, ipv6Prefix6}
			dstipv6s = []string{ipv6Prefix1, ipv6Prefix2, ipv6Prefix3}
		}

		// Creating flows
		for i := 0; i <= 2; i++ {
			configureFlow(t, src, dst, srcips[i], dstips[i], config)
		}
		for i := 0; i <= 2; i++ {
			configureFlowV6(t, src, dst, srcipv6s[i], dstipv6s[i], config)
		}
	}
	ate.OTG().PushConfig(t, config)
	ate.OTG().StartProtocols(t)

	flowsWithNoERR := []string{fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix1, ipv4Prefix4),
		fmt.Sprintf("%s-%s-Ipv4", ipv4Prefix4, ipv4Prefix1),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix1, ipv6Prefix4),
		fmt.Sprintf("%s-%s-Ipv6", ipv6Prefix1, ipv6Prefix4)}

	peers := []string{
		ateIBGP.Name + ".BGP4.peer",
		ateEBGP.Name + ".BGP4.peer",
		ateIBGP.Name + ".BGP6.peer",
		ateEBGP.Name + ".BGP6.peer",
	}

	t.Run("Check BGP status", func(t *testing.T) {
		t.Log("Check BGP status")
		checkBgpStatus(t, dut, 3)
	})

	t.Run("1_BaseLine_Validation", func(t *testing.T) {
		t.Log("verify BGP Graceful Restart settings")
		checkBgpGRConfig(t, dut)

		// // TODO: Add Deviation
		// exrrConfig := `router bgp 100
		// 		neighbor 192.0.2.2 graceful-restart-helper restart-time 15552000  stale-route route-map STALE-ROUTE-POLICY`
		// helpers.GnmiCLIConfig(t, dut, exrrConfig)
		// exrrConfig = `router bgp 100
		// 		neighbor 192.0.2.6 graceful-restart-helper restart-time 15552000  stale-route route-map STALE-ROUTE-POLICY`
		// helpers.GnmiCLIConfig(t, dut, exrrConfig)

		// TODO: Add ExRR validation once OC is available

		validatePrefixesWithAttributes(t, ate, prefixattrs)
		validateV6PrefixesWithAttributes(t, ate, prefixv6attrs)
		validateTrafficFlows(t, ate, config, true)
	})

	// Add deviation
	exrrConfig := `router bgp 100
				neighbor 192.0.2.2 graceful-restart-helper restart-time 300  stale-route route-map STALE-ROUTE-POLICY`
	helpers.GnmiCLIConfig(t, dut, exrrConfig)
	exrrConfig = `router bgp 100
				neighbor 192.0.2.6 graceful-restart-helper restart-time 300  stale-route route-map STALE-ROUTE-POLICY`
	helpers.GnmiCLIConfig(t, dut, exrrConfig)
	exrrConfig = `router bgp 100
				neighbor 2001:db8::192:0:2:2 graceful-restart-helper restart-time 300  stale-route route-map STALE-ROUTE-POLICY`
	helpers.GnmiCLIConfig(t, dut, exrrConfig)
	exrrConfig = `router bgp 100
				neighbor 2001:db8::192:0:2:6 graceful-restart-helper restart-time 300  stale-route route-map STALE-ROUTE-POLICY`
	helpers.GnmiCLIConfig(t, dut, exrrConfig)

	t.Run("2_DUT_as_Helper_for_a_(gracefully)_Restarting_Peer", func(t *testing.T) {

		ate.OTG().StartTraffic(t)

		t.Log("Send Graceful Restart Trigger from OTG to DUT")
		ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "none"))

		validateExrr(t, flowsWithNoERR)

		// Check Routes are re-learnt after graceful-restart completes
		checkBgpStatus(t, dut, 3)

	})

	t.Run("3_ATE_Peer_Abrupt_Termination", func(t *testing.T) {

		ate.OTG().StartTraffic(t)

		t.Logf("Stop BGP on the ATE Peer")
		stopBgp := gosnappi.NewControlState()
		stopBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
			SetState(gosnappi.StateProtocolBgpPeersState.DOWN)
		ate.OTG().SetControlState(t, stopBgp)

		validateExrr(t, flowsWithNoERR)

		t.Logf("Start BGP on the ATE Peer")
		startBgp := gosnappi.NewControlState()
		startBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
			SetState(gosnappi.StateProtocolBgpPeersState.UP)
		ate.OTG().SetControlState(t, startBgp)

		checkBgpStatus(t, dut, 3)

	})

	t.Run("4_Administrative_Reset_Notification_Sent_By_DUT_Graceful", func(t *testing.T) {
		// TODO: Remove the skip once https://partnerissuetracker.corp.google.com/issues/444181975 resolved
		t.Skip("Skipping this subtest for now")

		ate.OTG().StartTraffic(t)

		gnoiClient := dut.RawAPIs().GNOI(t)
		bgpReq := &bpb.ClearBGPNeighborRequest{
			Mode: bpb.ClearBGPNeighborRequest_SOFT,
		}
		gnoiClient.BGP().ClearBGPNeighbor(context.Background(), bgpReq)

		validateExrr(t, flowsWithNoERR)

		checkBgpStatus(t, dut, 3)
	})

	t.Run("5_Administrative_Reset_Notification_Received_By_DUT_Graceful", func(t *testing.T) {

		ate.OTG().StartTraffic(t)

		t.Log("Send Graceful Restart Trigger from OTG to DUT")
		ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "soft"))

		validateExrr(t, flowsWithNoERR)

		// Check Routes are re-learnt after graceful-restart completes
		checkBgpStatus(t, dut, 3)
	})

	t.Run("6_Administrative_Reset_Notification_Sent_By_DUT_Hard_Reset", func(t *testing.T) {
		// TODO: Remove the skip once https://partnerissuetracker.corp.google.com/issues/444181975 resolved
		t.Skip("Skipping this subtest for now")

		ate.OTG().StartTraffic(t)

		gnoiClient := dut.RawAPIs().GNOI(t)
		bgpReq := &bpb.ClearBGPNeighborRequest{
			Mode: bpb.ClearBGPNeighborRequest_HARD,
		}
		gnoiClient.BGP().ClearBGPNeighbor(context.Background(), bgpReq)

		validateExrr(t, flowsWithNoERR)

		checkBgpStatus(t, dut, 3)

	})

	t.Run("7_Administrative_Reset_Notification_Received_By_DUT_Hard_Reset", func(t *testing.T) {

		ate.OTG().StartTraffic(t)

		t.Log("Send Graceful Restart Trigger from OTG to DUT")
		ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "hard"))

		validateExrr(t, flowsWithNoERR)

		checkBgpStatus(t, dut, 3)

	})

	t.Run("8_Additive_Policy_Application", func(t *testing.T) {

		d := &oc.Root{}
		rp := d.GetOrCreateRoutingPolicy()

		setLP := rp.GetOrCreatePolicyDefinition("SET-LP")
		setLPstmt, err := setLP.AppendNewStatement("10")
		if err != nil {
			t.Errorf("Error while creating new statement %v", err)
		}
		setLPstmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE
		setLPstmtCommunitySet := setLPstmt.GetOrCreateConditions().GetOrCreateBgpConditions().GetOrCreateMatchCommunitySet()
		setLPstmtCommunitySet.SetCommunitySet("TEST-IBGP")
		setLPstmt.GetOrCreateActions().GetOrCreateBgpActions().SetSetLocalPref(200)
		setLPstmt.GetOrCreateActions().GetOrCreateBgpActions().SetSetMed(oc.UnionUint32(150))
		if !deviations.BGPSetMedActionUnsupported(dut) {
			setLPstmt.GetOrCreateActions().GetOrCreateBgpActions().SetSetMedAction(oc.BgpPolicy_BgpSetMedAction_SET)
		}

		gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)

		ate.OTG().StartTraffic(t)

		t.Log("Send Graceful Restart Trigger from OTG to DUT")
		ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "none"))

		validateExrr(t, flowsWithNoERR)

		checkBgpStatus(t, dut, 3)

	})

	t.Run("9_Default_Reject_Behavior", func(t *testing.T) {

		// TODO: Add deviation
		exrrConfig := `router bgp 100
				no neighbor 192.0.2.2 graceful-restart-helper restart-time 300  stale-route route-map STALE-ROUTE-POLICY`
		helpers.GnmiCLIConfig(t, dut, exrrConfig)
		exrrConfig = `router bgp 100
				no neighbor 192.0.2.6 graceful-restart-helper restart-time 300  stale-route route-map STALE-ROUTE-POLICY`
		helpers.GnmiCLIConfig(t, dut, exrrConfig)
		exrrConfig = `router bgp 100
				no neighbor 2001:db8::192:0:2:2 graceful-restart-helper restart-time 300  stale-route route-map STALE-ROUTE-POLICY`
		helpers.GnmiCLIConfig(t, dut, exrrConfig)
		exrrConfig = `router bgp 100
				no neighbor 2001:db8::192:0:2:6 graceful-restart-helper restart-time 300  stale-route route-map STALE-ROUTE-POLICY`
		helpers.GnmiCLIConfig(t, dut, exrrConfig)

		// TODO: Add deviation
		exrrConfig = `router bgp 100
				neighbor 192.0.2.2 graceful-restart-helper restart-time 300`
		helpers.GnmiCLIConfig(t, dut, exrrConfig)
		exrrConfig = `router bgp 100
				neighbor 192.0.2.6 graceful-restart-helper restart-time 300`
		helpers.GnmiCLIConfig(t, dut, exrrConfig)
		exrrConfig = `router bgp 100
				neighbor 2001:db8::192:0:2:2 graceful-restart-helper restart-time 300`
		helpers.GnmiCLIConfig(t, dut, exrrConfig)
		exrrConfig = `router bgp 100
				neighbor 2001:db8::192:0:2:6 graceful-restart-helper restart-time 300`
		helpers.GnmiCLIConfig(t, dut, exrrConfig)

		t.Logf("Stop BGP on the ATE Peer")
		stopBgp := gosnappi.NewControlState()
		stopBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
			SetState(gosnappi.StateProtocolBgpPeersState.DOWN)
		ate.OTG().SetControlState(t, stopBgp)

		checkBgpStatusDown(t, dut)

		time.Sleep(triggerGrTimer)

		sendTraffic(t, ate)
		confirmPacketLoss(t, ate, flowsWithNoERR)
	})

	t.Run("10_Consecutive_BGP_Restarts", func(t *testing.T) {

		// TODO: Add deviation
		exrrConfig := `router bgp 100
				neighbor 192.0.2.2 graceful-restart-helper restart-time 300  stale-route route-map STALE-ROUTE-POLICY`
		helpers.GnmiCLIConfig(t, dut, exrrConfig)
		exrrConfig = `router bgp 100
				neighbor 192.0.2.6 graceful-restart-helper restart-time 300  stale-route route-map STALE-ROUTE-POLICY`
		helpers.GnmiCLIConfig(t, dut, exrrConfig)

		ate.OTG().StartTraffic(t)

		for i := 0; i < 3; i++ {
			ate.OTG().SetControlAction(t, createGracefulRestartAction(t, peers, triggerGrTimer, "soft"))
			time.Sleep(60 * time.Second)
		}

		t.Logf("Stop BGP on the ATE Peer")
		stopBgp := gosnappi.NewControlState()
		stopBgp.Protocol().Bgp().Peers().SetPeerNames(peers).
			SetState(gosnappi.StateProtocolBgpPeersState.DOWN)

		ate.OTG().SetControlState(t, stopBgp)

		validateExrr(t, flowsWithNoERR)

	})

}
