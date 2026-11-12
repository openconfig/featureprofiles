package bgp_vrf_l3vpn_parameters_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	port1 = "port1"
	port2 = "port2"

	subIfaceIndex   = 0
	bgpProtocolName = "BGP"

	vrf100          = "VRF_100"
	vrf200          = "VRF_200"
	vrf100RD        = "64496:100"
	vrf100RT        = "64496:100"
	vrf200RD        = "64496:200"
	vrf200RT        = "64496:200"
	vrf100RouterID  = "192.0.2.1"
	defaultRouterID = "198.51.100.1"

	dutAS     = uint32(64496)
	ateCustAS = uint32(64497)
	ateCoreAS = uint32(64496)

	plenIPv4 = uint8(30)
	plenIPv6 = uint8(64)

	prefixLimit     = uint32(5000)
	custV4Prefix    = "203.0.113.10"
	custV4PrefixLen = uint32(32)
	custV6Prefix    = "2001:db8:3::10"
	custV6PrefixLen = uint32(128)
	pgCustV4        = "PG-CUST-V4"
	pgCustV6        = "PG-CUST-V6"
	pgCoreV4        = "PG-CORE-V4"
	pgCoreV6        = "PG-CORE-V6"

	custV4Password = "customer_secret_v4"
	custV6Password = "customer_secret_v6"

	grRestartTime     = uint16(120)
	grHelperExtraWait = 30 * time.Second
	immediateTimeout  = 20 * time.Second

	rplName = "ALLOW"

	bgpTimeout = 2 * time.Minute
	aftTimeout = 1 * time.Minute
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE port1 (VRF_100)",
		Name:    "dutCust",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8:1::1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE port2 (DEFAULT)",
		Name:    "dutCore",
		IPv4:    "198.51.100.1",
		IPv6:    "2001:db8:2::1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	atePort1 = attrs.Attributes{
		Name:    "ateCust",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8:1::2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	atePort2 = attrs.Attributes{
		Name:    "ateCore",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "198.51.100.2",
		IPv6:    "2001:db8:2::2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	defaultNI = ""
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestBGPVRFL3VPNParameters(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	defaultNI = deviations.DefaultNetworkInstance(dut)
	configureDUT(t, dut)
	configureATE(t, ate)

	cfgplugins.VerifyPortsUp(t, dut.Device)
	testCases := []testCase{
		{"RT-1.102.1_eBGP_Session_Establishment_in_VRF", testEBGPSessionInVRF},
		{"RT-1.102.2_L3VPN_Attribute_Validation", testL3VPNAttributeValidation},
		{"RT-1.102.3_Maximum_Prefix_Limit_Enforcement", testMaxPrefixLimit},
		{"RT-1.102.4_Isolation_Boundary", testIsolationBoundary},
		{"RT-1.102.5_Graceful_Restart_in_VRF", testGracefulRestartInVRF},
		{"RT-1.102.6_Immediate_Withdrawal_without_GR", testImmediateWithdrawalNoGR},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer ate.OTG().StopProtocols(t)
			if err := tc.run(t, dut, ate); err != nil {
				t.Error(err)
			}
		})
	}
}

func bgpPeerName(dev, af string) string { return fmt.Sprintf("%s.BGP.%s.peer", dev, af) }

func configureDUTInterfaces(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	dc := gnmi.OC()
	p1 := dut.Port(t, port1)
	p2 := dut.Port(t, port2)
	gnmi.BatchReplace(batch, dc.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.BatchReplace(batch, dc.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))
}

func configureDUTNetworkInstances(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	p2 := dut.Port(t, port2)

	fptest.ConfigureDefaultNetworkInstance(t, dut)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		cfgplugins.AssignToNetworkInstance(t, dut, p2.Name(), defaultNI, subIfaceIndex)
	}

	for _, v := range []struct{ name, rd, rt string }{
		{vrf100, vrf100RD, vrf100RT},
		{vrf200, vrf200RD, vrf200RT},
	} {
		configureVRF(t, dut, batch, v.name, v.rd, v.rt)
	}
}

func configureVRF(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, name, rd, rt string) {
	t.Helper()
	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(name)
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	if !deviations.NetworkInstanceImportExportPolicyOCUnsupported(dut) {
		ni.RouteDistinguisher = ygot.String(rd)
		iexp := ni.GetOrCreateInterInstancePolicies().GetOrCreateImportExportPolicy()
		iexp.SetImportRouteTarget([]oc.NetworkInstance_InterInstancePolicies_ImportExportPolicy_ImportRouteTarget_Union{oc.UnionString(rt)})
		iexp.SetExportRouteTarget([]oc.NetworkInstance_InterInstancePolicies_ImportExportPolicy_ExportRouteTarget_Union{oc.UnionString(rt)})
	}
	gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(name).Config(), ni)
}

func configureRoutingPolicy(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	if _, err := cfgplugins.ConfigureBGPRoutePolicy(t, batch, cfgplugins.BGPPolicyConfig{PolicyName: rplName, StatementID: "10"}); err != nil {
		t.Fatalf("ConfigureBGPRoutePolicy failed: %v", err)
	}
}

type bgpConfigOpts struct {
	gracefulRestart bool
	v4PrefixLimit   uint32
	v6PrefixLimit   uint32
}

func applyBGPConfig(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, opts bgpConfigOpts) {
	t.Helper()
	gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Config(), buildCoreBGP(dut))
	gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(vrf100).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Config(), buildCustBGP(dut, opts))
}

func buildCoreBGP(dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	proto := &oc.NetworkInstance_Protocol{Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, Name: ygot.String(bgpProtocolName)}
	bgp := proto.GetOrCreateBgp()
	cfgplugins.ConfigureGlobal(bgp, dut, cfgplugins.WithAS(dutAS), cfgplugins.WithRouterID(defaultRouterID), cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV4_UNICAST, true), cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV6_UNICAST, true))

	for _, spec := range []struct {
		name string
		afi  oc.E_BgpTypes_AFI_SAFI_TYPE
	}{
		{pgCoreV4, oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV4_UNICAST},
		{pgCoreV6, oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV6_UNICAST},
	} {
		pg := bgp.GetOrCreatePeerGroup(spec.name)
		pg.PeerGroupName = ygot.String(spec.name)
		cfgplugins.ConfigurePeerGroup(pg, dut, cfgplugins.WithPeerAS(ateCoreAS), cfgplugins.WithPGSendCommunity([]oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_STANDARD, oc.Bgp_CommunityType_EXTENDED}), cfgplugins.WithPGAfiSafiEnabled(spec.afi, true, false))

		if deviations.RoutePolicyUnderAFIUnsupported(dut) {
			rpl := pg.GetOrCreateApplyPolicy()
			rpl.ImportPolicy = []string{rplName}
			rpl.ExportPolicy = []string{rplName}
		} else {
			pgafi := pg.GetOrCreateAfiSafi(spec.afi)
			pgafi.Enabled = ygot.Bool(true)
			rpl := pgafi.GetOrCreateApplyPolicy()
			rpl.ImportPolicy = []string{rplName}
			rpl.ExportPolicy = []string{rplName}
		}
	}

	addCoreNeighbor(bgp, atePort2.IPv4, pgCoreV4, oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV4_UNICAST)
	addCoreNeighbor(bgp, atePort2.IPv6, pgCoreV6, oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV6_UNICAST)
	return proto
}

func addCoreNeighbor(bgp *oc.NetworkInstance_Protocol_Bgp, addr, pgName string, afi oc.E_BgpTypes_AFI_SAFI_TYPE) {
	nbr := bgp.GetOrCreateNeighbor(addr)
	nbr.PeerAs = ygot.Uint32(ateCoreAS)
	nbr.Enabled = ygot.Bool(true)
	nbr.PeerGroup = ygot.String(pgName)
	nbr.GetOrCreateAfiSafi(afi).Enabled = ygot.Bool(true)
}

func buildCustBGP(dut *ondatra.DUTDevice, opts bgpConfigOpts) *oc.NetworkInstance_Protocol {
	proto := &oc.NetworkInstance_Protocol{Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, Name: ygot.String(bgpProtocolName)}
	bgp := proto.GetOrCreateBgp()
	globalOpts := []cfgplugins.GlobalOption{
		cfgplugins.WithAS(dutAS),
		cfgplugins.WithRouterID(vrf100RouterID),
		cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, true),
		cfgplugins.WithGlobalAfiSafiEnabled(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, true),
	}
	if opts.gracefulRestart {
		globalOpts = append(globalOpts, cfgplugins.WithGlobalGracefulRestart(true, grRestartTime, 0))
	}
	cfgplugins.ConfigureGlobal(bgp, dut, globalOpts...)

	noPeerGroup := deviations.SetNoPeerGroup(dut) || deviations.PeerGroupDefEbgpVrfUnsupported(dut)

	var pgV4Name, pgV6Name string
	if !noPeerGroup {
		for _, spec := range []struct {
			name string
			afi  oc.E_BgpTypes_AFI_SAFI_TYPE
		}{
			{pgCustV4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST},
			{pgCustV6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST},
		} {
			pg := bgp.GetOrCreatePeerGroup(spec.name)
			pg.PeerGroupName = ygot.String(spec.name)
			cfgplugins.ConfigurePeerGroup(pg, dut, cfgplugins.WithPeerAS(ateCustAS), cfgplugins.WithPGAfiSafiEnabled(spec.afi, true, false), cfgplugins.ApplyPGRoutingPolicy(rplName, rplName, false))
		}
		pgV4Name, pgV6Name = pgCustV4, pgCustV6
	}

	addCustNeighbor(dut, bgp, atePort1.IPv4, pgV4Name, custV4Password, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST, opts.gracefulRestart, opts.v4PrefixLimit, noPeerGroup)
	addCustNeighbor(dut, bgp, atePort1.IPv6, pgV6Name, custV6Password, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST, opts.gracefulRestart, opts.v6PrefixLimit, noPeerGroup)
	return proto
}

func addCustNeighbor(dut *ondatra.DUTDevice, bgp *oc.NetworkInstance_Protocol_Bgp, addr, pgName, password string, afi oc.E_BgpTypes_AFI_SAFI_TYPE, gracefulRestart bool, limit uint32, noPeerGroup bool) {
	nbr := bgp.GetOrCreateNeighbor(addr)
	nbr.PeerAs = ygot.Uint32(ateCustAS)
	nbr.Enabled = ygot.Bool(true)
	if pgName != "" {
		nbr.PeerGroup = ygot.String(pgName)
	}
	nbr.AuthPassword = ygot.String(password)
	if gracefulRestart {
		nbr.GetOrCreateGracefulRestart().Enabled = ygot.Bool(true)
	}
	af := nbr.GetOrCreateAfiSafi(afi)
	af.Enabled = ygot.Bool(true)
	if limit > 0 {
		setPrefixLimit(dut, af, afi, limit)
	}
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		ap := nbr.GetOrCreateApplyPolicy()
		ap.SetImportPolicy([]string{rplName})
		ap.SetExportPolicy([]string{rplName})
	} else {
		ap := af.GetOrCreateApplyPolicy()
		ap.SetImportPolicy([]string{rplName})
		ap.SetExportPolicy([]string{rplName})
	}
}

func setPrefixLimit(dut *ondatra.DUTDevice, afisafi *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi, afi oc.E_BgpTypes_AFI_SAFI_TYPE, limit uint32) {
	explicit := deviations.BGPExplicitPrefixLimitReceived(dut)
	switch afi {
	case oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST:
		if explicit {
			afisafi.GetOrCreateIpv4Unicast().GetOrCreatePrefixLimitReceived().MaxPrefixes = ygot.Uint32(limit)
		} else {
			afisafi.GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit().MaxPrefixes = ygot.Uint32(limit)
		}
	case oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST:
		if explicit {
			afisafi.GetOrCreateIpv6Unicast().GetOrCreatePrefixLimitReceived().MaxPrefixes = ygot.Uint32(limit)
		} else {
			afisafi.GetOrCreateIpv6Unicast().GetOrCreatePrefixLimit().MaxPrefixes = ygot.Uint32(limit)
		}
	}
}

type ateConfigOpts struct {
	enableCustGR bool
	v4RouteCount uint32
	v6RouteCount uint32
}

func buildATEConfig(t *testing.T, ate *ondatra.ATEDevice, opts ateConfigOpts) gosnappi.Config {
	t.Helper()
	cfg := gosnappi.NewConfig()

	d1 := atePort1.AddToOTG(cfg, ate.Port(t, port1), &dutPort1)
	d2 := atePort2.AddToOTG(cfg, ate.Port(t, port2), &dutPort2)

	ip1v4 := d1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	ip1v6 := d1.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	custBgp := d1.Bgp().SetRouterId(atePort1.IPv4)

	custV4 := custBgp.Ipv4Interfaces().Add().SetIpv4Name(ip1v4.Name()).Peers().Add().SetName(bgpPeerName(atePort1.Name, cfgplugins.IPv4)).SetPeerAddress(dutPort1.IPv4).SetAsNumber(ateCustAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
	custV4.Advanced().SetMd5Key(custV4Password)
	if opts.enableCustGR {
		custV4.GracefulRestart().SetEnableGr(true).SetRestartTime(uint32(grRestartTime))
	}
	if opts.v4RouteCount > 0 {
		v4Routes := custV4.V4Routes().Add().SetName(atePort1.Name + ".v4routes").SetNextHopIpv4Address(atePort1.IPv4).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)
		v4Routes.Addresses().Add().SetAddress(custV4Prefix).SetPrefix(custV4PrefixLen).SetCount(opts.v4RouteCount)
	}

	custV6 := custBgp.Ipv6Interfaces().Add().SetIpv6Name(ip1v6.Name()).Peers().Add().SetName(bgpPeerName(atePort1.Name, cfgplugins.IPv6)).SetPeerAddress(dutPort1.IPv6).SetAsNumber(ateCustAS).SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
	custV6.Advanced().SetMd5Key(custV6Password)
	if opts.enableCustGR {
		custV6.GracefulRestart().SetEnableGr(true).SetRestartTime(uint32(grRestartTime))
	}
	if opts.v6RouteCount > 0 {
		v6Routes := custV6.V6Routes().Add().SetName(atePort1.Name + ".v6routes").SetNextHopIpv6Address(atePort1.IPv6).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)
		v6Routes.Addresses().Add().SetAddress(custV6Prefix).SetPrefix(custV6PrefixLen).SetCount(opts.v6RouteCount)
	}

	ip2v4 := d2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	ip2v6 := d2.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	coreBgp := d2.Bgp().SetRouterId(atePort2.IPv4)
	coreV4 := coreBgp.Ipv4Interfaces().Add().SetIpv4Name(ip2v4.Name()).Peers().Add().SetName(bgpPeerName(atePort2.Name, cfgplugins.IPv4)).SetPeerAddress(dutPort2.IPv4).SetAsNumber(ateCoreAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
	coreV4.Capability().SetIpv4MplsVpn(true)
	coreV4.Capability().SetIpv4Unicast(true)
	coreV6 := coreBgp.Ipv6Interfaces().Add().SetIpv6Name(ip2v6.Name()).Peers().Add().SetName(bgpPeerName(atePort2.Name, cfgplugins.IPv6)).SetPeerAddress(dutPort2.IPv6).SetAsNumber(ateCoreAS).SetAsType(gosnappi.BgpV6PeerAsType.IBGP)
	coreV6.Capability().SetIpv6MplsVpn(true)
	coreV6.Capability().SetIpv6Unicast(true)

	return cfg
}

func pushATEConfig(t *testing.T, ate *ondatra.ATEDevice, cfg gosnappi.Config) {
	t.Helper()
	otg := ate.OTG()
	otg.PushConfig(t, cfg)
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, cfg, cfgplugins.IPv4)
	otgutils.WaitForARP(t, otg, cfg, cfgplugins.IPv6)
}

func awaitCustBGPState(t *testing.T, dut *ondatra.DUTDevice, want oc.E_Bgp_Neighbor_SessionState, timeout time.Duration) error {
	t.Helper()
	var errs []error
	for _, nbr := range []string{atePort1.IPv4, atePort1.IPv6} {
		errs = append(errs, awaitCustNeighborState(t, dut, nbr, want, timeout))
	}
	return errors.Join(errs...)
}

func awaitCustNeighborState(t *testing.T, dut *ondatra.DUTDevice, nbr string, want oc.E_Bgp_Neighbor_SessionState, timeout time.Duration) error {
	t.Helper()
	nbrPath := gnmi.OC().NetworkInstance(vrf100).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp().Neighbor(nbr)
	_, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), timeout, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		st, present := val.Val()
		return present && st == want
	}).Await(t)
	if !ok {
		return fmt.Errorf("customer eBGP neighbor %s did not reach %v in %s", nbr, want, timeout)
	}
	return nil
}

func custV4RouteCIDR() string { return fmt.Sprintf("%s/%d", custV4Prefix, custV4PrefixLen) }
func custV6RouteCIDR() string { return fmt.Sprintf("%s/%d", custV6Prefix, custV6PrefixLen) }

func verifyCustRoutesInAFT(t *testing.T, dut *ondatra.DUTDevice, ni string, want bool) error {
	return verifyCustRoutesInAFTWithin(t, dut, ni, want, aftTimeout)
}

func verifyCustRoutesInAFTWithin(t *testing.T, dut *ondatra.DUTDevice, ni string, want bool, timeout time.Duration) error {
	t.Helper()
	v4 := gnmi.OC().NetworkInstance(ni).Afts().Ipv4Entry(custV4RouteCIDR())
	v6 := gnmi.OC().NetworkInstance(ni).Afts().Ipv6Entry(custV6RouteCIDR())
	var errs []error
	if _, ok := gnmi.Watch(t, dut, v4.State(), timeout, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool { return val.IsPresent() == want }).Await(t); !ok {
		errs = append(errs, fmt.Errorf("prefix %s presence in AFT of %s did not become %t", custV4RouteCIDR(), ni, want))
	}
	if _, ok := gnmi.Watch(t, dut, v6.State(), timeout, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool { return val.IsPresent() == want }).Await(t); !ok {
		errs = append(errs, fmt.Errorf("prefix %s presence in AFT of %s did not become %t", custV6RouteCIDR(), ni, want))
	}
	return errors.Join(errs...)
}

func awaitCustBGPDown(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration) error {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(vrf100).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp()
	var errs []error
	for _, nbr := range []string{atePort1.IPv4, atePort1.IPv6} {
		nbrPath := bgpPath.Neighbor(nbr)
		if _, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), timeout, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			st, present := val.Val()
			return present && st != oc.Bgp_Neighbor_SessionState_ESTABLISHED
		}).Await(t); !ok {
			errs = append(errs, fmt.Errorf("customer eBGP neighbor %s remained ESTABLISHED for %s", nbr, timeout))
		}
	}
	return errors.Join(errs...)
}

func awaitCoreBGPState(t *testing.T, dut *ondatra.DUTDevice, want oc.E_Bgp_Neighbor_SessionState, timeout time.Duration) error {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp()
	var errs []error
	for _, nbr := range []string{atePort2.IPv4, atePort2.IPv6} {
		if _, ok := gnmi.Watch(t, dut, bgpPath.Neighbor(nbr).SessionState().State(), timeout, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
			st, present := val.Val()
			return present && st == want
		}).Await(t); !ok {
			errs = append(errs, fmt.Errorf("core iBGP neighbor %s did not reach %v in %s", nbr, want, timeout))
		}
	}
	return errors.Join(errs...)
}

func verifyVRF100RouterID(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	got := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(vrf100).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp().Global().RouterId().State())
	if got != vrf100RouterID {
		return fmt.Errorf("VRF_100 BGP router-id: got %q, want %q", got, vrf100RouterID)
	}
	return nil
}

func verifyPrefixLimitConfig(t *testing.T, dut *ondatra.DUTDevice, want uint32) error {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(vrf100).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp()

	v4 := bgpPath.Neighbor(atePort1.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	v6 := bgpPath.Neighbor(atePort1.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)

	var gotV4, gotV6 uint32
	if deviations.BGPExplicitPrefixLimitReceived(dut) {
		gotV4 = gnmi.Get(t, dut, v4.Ipv4Unicast().PrefixLimitReceived().MaxPrefixes().State())
		gotV6 = gnmi.Get(t, dut, v6.Ipv6Unicast().PrefixLimitReceived().MaxPrefixes().State())
	} else {
		gotV4 = gnmi.Get(t, dut, v4.Ipv4Unicast().PrefixLimit().MaxPrefixes().State())
		gotV6 = gnmi.Get(t, dut, v6.Ipv6Unicast().PrefixLimit().MaxPrefixes().State())
	}
	var errs []error
	if gotV4 != want {
		errs = append(errs, fmt.Errorf("IPv4 prefix-limit max-prefixes: got %d, want %d", gotV4, want))
	}
	if gotV6 != want {
		errs = append(errs, fmt.Errorf("IPv6 prefix-limit max-prefixes: got %d, want %d", gotV6, want))
	}
	return errors.Join(errs...)
}

func verifyCoreL3VPNAFIEnabled(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp()
	var errs []error
	for _, tc := range coreVPNAFIs() {
		p := bgpPath.Neighbor(tc.addr).AfiSafi(tc.afi).Enabled().State()
		got := gnmi.Lookup(t, dut, p)
		enabled, present := got.Val()
		if !present || !enabled {
			errs = append(errs, fmt.Errorf("AFI-SAFI %v not reported enabled on iBGP neighbor %s", tc.afi, tc.addr))
		}
	}
	return errors.Join(errs...)
}

func verifyL3VPNExportConfig(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	var errs []error
	if !deviations.NetworkInstanceImportExportPolicyOCUnsupported(dut) {
		ni := gnmi.OC().NetworkInstance(vrf100)
		if got := gnmi.Get(t, dut, ni.RouteDistinguisher().State()); got != vrf100RD {
			errs = append(errs, fmt.Errorf("VRF_100 route distinguisher: got %q, want %q", got, vrf100RD))
		}
		policy := ni.InterInstancePolicies().ImportExportPolicy()
		errs = append(errs, verifyRouteTarget("import", gnmi.Get(t, dut, policy.ImportRouteTarget().State()), vrf100RT))
		errs = append(errs, verifyRouteTarget("export", gnmi.Get(t, dut, policy.ExportRouteTarget().State()), vrf100RT))
	}

	bgpPath := gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp()
	for _, pgName := range []string{pgCoreV4, pgCoreV6} {
		communities := gnmi.Get(t, dut, bgpPath.PeerGroup(pgName).SendCommunityType().State())
		if !containsCommunity(communities, oc.Bgp_CommunityType_EXTENDED) && !containsCommunity(communities, oc.Bgp_CommunityType_BOTH) {
			errs = append(errs, fmt.Errorf("core peer-group %s does not send extended communities: got %v", pgName, communities))
		}
	}
	return errors.Join(errs...)
}

func verifyRouteTarget[T any](direction string, got []T, want string) error {
	for _, value := range got {
		if fmt.Sprint(value) == want {
			return nil
		}
	}
	return fmt.Errorf("VRF_100 %s route target: got %v, want %q", direction, got, want)
}

func containsCommunity(got []oc.E_Bgp_CommunityType, want oc.E_Bgp_CommunityType) bool {
	for _, community := range got {
		if community == want {
			return true
		}
	}
	return false
}

type coreAFI struct {
	addr string
	afi  oc.E_BgpTypes_AFI_SAFI_TYPE
}

func coreVPNAFIs() []coreAFI {
	return []coreAFI{
		{addr: atePort2.IPv4, afi: oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV4_UNICAST},
		{addr: atePort2.IPv6, afi: oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV6_UNICAST},
	}
}

func coreVPNSentPrefixes(tc coreAFI) ygnmi.SingletonQuery[uint32] {
	return gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp().Neighbor(tc.addr).AfiSafi(tc.afi).Prefixes().Sent().State()
}

func sentPrefixVal(dut *ondatra.DUTDevice, val *ygnmi.Value[uint32]) (uint32, bool) {
	count, present := val.Val()
	if !present && deviations.MissingValueForDefaults(dut) {
		return 0, true
	}
	return count, present
}

func logCoreVPNDiagnostics(t *testing.T, dut *ondatra.DUTDevice, tc coreAFI) {
	t.Helper()
	afiPath := gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp().Neighbor(tc.addr).AfiSafi(tc.afi).State()
	if state, present := gnmi.Lookup(t, dut, afiPath).Val(); present {
		jsonState, err := ygot.EmitJSON(state, &ygot.EmitJSONConfig{
			Format:         ygot.RFC7951,
			Indent:         "  ",
			SkipValidation: true,
		})
		if err == nil {
			t.Logf("core neighbor %s %v AFI-SAFI state: %v", tc.addr, tc.afi, jsonState)
		}
	} else {
		t.Logf("core neighbor %s %v AFI-SAFI state is absent from telemetry", tc.addr, tc.afi)
	}
}

func awaitCoreVPNPrefixAdvertisement(t *testing.T, dut *ondatra.DUTDevice, want bool, timeout time.Duration) error {
	t.Helper()
	var errs []error
	for _, tc := range coreVPNAFIs() {
		if _, ok := gnmi.Watch(t, dut, coreVPNSentPrefixes(tc), timeout, func(val *ygnmi.Value[uint32]) bool {
			count, present := sentPrefixVal(dut, val)
			return present && (count > 0) == want
		}).Await(t); !ok {
			logCoreVPNDiagnostics(t, dut, tc)
			errs = append(errs, fmt.Errorf("VPN prefix advertisement to core neighbor %s for %v did not become %t", tc.addr, tc.afi, want))
		}
	}
	return errors.Join(errs...)
}

func coreVPNPrefixCounts(t *testing.T, dut *ondatra.DUTDevice) (map[string]uint32, error) {
	t.Helper()
	counts := map[string]uint32{}
	var errs []error
	for _, tc := range coreVPNAFIs() {
		count, present := sentPrefixVal(dut, gnmi.Lookup(t, dut, coreVPNSentPrefixes(tc)))
		if !present {
			logCoreVPNDiagnostics(t, dut, tc)
			errs = append(errs, fmt.Errorf("core neighbor %s %v sent-prefix counter is absent from telemetry", tc.addr, tc.afi))
		}
		counts[tc.addr] = count
	}
	return counts, errors.Join(errs...)
}

func awaitCoreVPNPrefixCounts(t *testing.T, dut *ondatra.DUTDevice, want map[string]uint32, timeout time.Duration) error {
	t.Helper()
	var errs []error
	for _, tc := range coreVPNAFIs() {
		wantCount := want[tc.addr]
		var realCount uint32
		if _, ok := gnmi.Watch(t, dut, coreVPNSentPrefixes(tc), timeout, func(val *ygnmi.Value[uint32]) bool {
			count, present := sentPrefixVal(dut, val)
			realCount = count
			return present && count == wantCount
		}).Await(t); !ok {
			logCoreVPNDiagnostics(t, dut, tc)
			errs = append(errs, fmt.Errorf("VPN sent-prefix count for core neighbor %s did not remain %d, got %d", tc.addr, wantCount, realCount))
		}
	}
	return errors.Join(errs...)
}

func awaitCoreVPNPrefixWithdrawal(t *testing.T, dut *ondatra.DUTDevice, before map[string]uint32, timeout time.Duration) error {
	t.Helper()
	var errs []error
	for _, tc := range coreVPNAFIs() {
		beforeCount := before[tc.addr]
		if beforeCount == 0 {
			errs = append(errs, fmt.Errorf("core neighbor %s %v sent-prefix count is zero before withdrawal", tc.addr, tc.afi))
			continue
		}
		if _, ok := gnmi.Watch(t, dut, coreVPNSentPrefixes(tc), timeout, func(val *ygnmi.Value[uint32]) bool {
			count, present := sentPrefixVal(dut, val)
			return present && count < beforeCount
		}).Await(t); !ok {
			logCoreVPNDiagnostics(t, dut, tc)
			errs = append(errs, fmt.Errorf("VPN sent-prefix count for core neighbor %s did not decrease from %d", tc.addr, beforeCount))
		}
	}
	return errors.Join(errs...)
}

func awaitATECoreVPNPrefixPresence(t *testing.T, ate *ondatra.ATEDevice, timeout time.Duration) error {
	t.Helper()
	var errs []error

	if fatalMsg := testt.CaptureFatal(t, func(t testing.TB) {
		otg := ate.OTG()
		v4Peer := bgpPeerName(atePort2.Name, cfgplugins.IPv4)
		if _, ok := gnmi.WatchAll(t, otg, gnmi.OTG().BgpPeer(v4Peer).UnicastIpv4PrefixAny().State(), timeout,
			func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv4Prefix]) bool {
				prefix, present := v.Val()
				return present && prefix.GetAddress() == custV4Prefix && prefix.GetPrefixLength() == custV4PrefixLen && hasVRF100RouteTarget4(prefix.ExtendedCommunity)
			}).Await(t); !ok {
			errs = append(errs, fmt.Errorf("ATE core IPv4 peer did not receive VPN prefix %s with RT %s within %s", custV4RouteCIDR(), vrf100RT, timeout))
		}

		v6Peer := bgpPeerName(atePort2.Name, cfgplugins.IPv6)
		if _, ok := gnmi.WatchAll(t, otg, gnmi.OTG().BgpPeer(v6Peer).UnicastIpv6PrefixAny().State(), timeout,
			func(v *ygnmi.Value[*otgtelemetry.BgpPeer_UnicastIpv6Prefix]) bool {
				prefix, present := v.Val()
				return present && prefix.GetAddress() == custV6Prefix && prefix.GetPrefixLength() == custV6PrefixLen && hasVRF100RouteTarget6(prefix.ExtendedCommunity)
			}).Await(t); !ok {
			errs = append(errs, fmt.Errorf("ATE core IPv6 peer did not receive VPN prefix %s with RT %s within %s", custV6RouteCIDR(), vrf100RT, timeout))
		}
	}); fatalMsg != nil {
		return fmt.Errorf("ATE core VPN prefix presence verification failed: %s", *fatalMsg)
	}
	return errors.Join(errs...)
}

func verifyATECoreVPNPrefixAbsence(t *testing.T, ate *ondatra.ATEDevice) error {
	t.Helper()
	var errs []error

	if fatalMsg := testt.CaptureFatal(t, func(t testing.TB) {
		otg := ate.OTG()
		v4Peer := bgpPeerName(atePort2.Name, cfgplugins.IPv4)
		for _, p := range gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(v4Peer).UnicastIpv4PrefixAny().State()) {
			if p.GetAddress() == custV4Prefix && p.GetPrefixLength() == custV4PrefixLen {
				errs = append(errs, fmt.Errorf("ATE core IPv4 peer still has VPN prefix %s after withdrawal", custV4RouteCIDR()))
				break
			}
		}

		v6Peer := bgpPeerName(atePort2.Name, cfgplugins.IPv6)
		for _, p := range gnmi.GetAll(t, otg, gnmi.OTG().BgpPeer(v6Peer).UnicastIpv6PrefixAny().State()) {
			if p.GetAddress() == custV6Prefix && p.GetPrefixLength() == custV6PrefixLen {
				errs = append(errs, fmt.Errorf("ATE core IPv6 peer still has VPN prefix %s after withdrawal", custV6RouteCIDR()))
				break
			}
		}
	}); *fatalMsg != "" {
		return fmt.Errorf("ATE core VPN prefix absence verification failed: %s", *fatalMsg)
	}
	return errors.Join(errs...)
}

func hasVRF100RouteTarget4(ecs []*otgtelemetry.BgpPeer_UnicastIpv4Prefix_ExtendedCommunity) bool {
	for _, ec := range ecs {
		if s := ec.GetStructured(); s != nil {
			if t2 := s.GetTransitive_2OctetAsType(); t2 != nil {
				if rt := t2.GetRouteTargetSubtype(); rt != nil && rt.GetGlobal_2ByteAs() == uint16(dutAS) && rt.GetLocal_4ByteAdmin() == 100 {
					return true
				}
			}
		}
	}
	return false
}

func hasVRF100RouteTarget6(ecs []*otgtelemetry.BgpPeer_UnicastIpv6Prefix_ExtendedCommunity) bool {
	for _, ec := range ecs {
		if s := ec.GetStructured(); s != nil {
			if t2 := s.GetTransitive_2OctetAsType(); t2 != nil {
				if rt := t2.GetRouteTargetSubtype(); rt != nil && rt.GetGlobal_2ByteAs() == uint16(dutAS) && rt.GetLocal_4ByteAdmin() == 100 {
					return true
				}
			}
		}
	}
	return false
}

func verifyCustomerGRNegotiation(t *testing.T, dut *ondatra.DUTDevice, want bool) error {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(vrf100).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp()
	var errs []error
	for _, tc := range []struct {
		addr string
		afi  oc.E_BgpTypes_AFI_SAFI_TYPE
	}{
		{atePort1.IPv4, oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST},
		{atePort1.IPv6, oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST},
	} {
		gr := bgpPath.Neighbor(tc.addr).AfiSafi(tc.afi).GracefulRestart()
		for name, query := range map[string]ygnmi.SingletonQuery[bool]{
			"advertised": gr.Advertised().State(),
			"received":   gr.Received().State(),
		} {
			value, present := gnmi.Lookup(t, dut, query).Val()
			if (present && value) != want {
				errs = append(errs, fmt.Errorf("GR %s for neighbor %s: got value=%t present=%t, want %t", name, tc.addr, value, present, want))
			}
		}
	}
	return errors.Join(errs...)
}

func establishedTransitions(t *testing.T, dut *ondatra.DUTDevice) map[string]uint64 {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(vrf100).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp()
	return map[string]uint64{
		atePort1.IPv4: gnmi.Get(t, dut, bgpPath.Neighbor(atePort1.IPv4).EstablishedTransitions().State()),
		atePort1.IPv6: gnmi.Get(t, dut, bgpPath.Neighbor(atePort1.IPv6).EstablishedTransitions().State()),
	}
}

func verifySingleReestablishment(t *testing.T, dut *ondatra.DUTDevice, before map[string]uint64) error {
	t.Helper()
	var errs []error
	for nbr, got := range establishedTransitions(t, dut) {
		if want := before[nbr] + 1; got != want {
			errs = append(errs, fmt.Errorf("neighbor %s established transitions: got %d, want %d", nbr, got, want))
		}
	}
	return errors.Join(errs...)
}

func receivedNotificationCounts(t *testing.T, dut *ondatra.DUTDevice) map[string]uint64 {
	t.Helper()
	bgpPath := gnmi.OC().NetworkInstance(vrf100).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp()
	return map[string]uint64{
		atePort1.IPv4: gnmi.Get(t, dut, bgpPath.Neighbor(atePort1.IPv4).Messages().Received().NOTIFICATION().State()),
		atePort1.IPv6: gnmi.Get(t, dut, bgpPath.Neighbor(atePort1.IPv6).Messages().Received().NOTIFICATION().State()),
	}
}

func verifyNoNotificationReceived(t *testing.T, dut *ondatra.DUTDevice, before map[string]uint64) error {
	t.Helper()
	var errs []error
	for nbr, got := range receivedNotificationCounts(t, dut) {
		if got != before[nbr] {
			errs = append(errs, fmt.Errorf("neighbor %s received BGP notifications: got %d, want unchanged at %d", nbr, got, before[nbr]))
		}
	}
	return errors.Join(errs...)
}

func verifyMaxPrefixNotification(t *testing.T, dut *ondatra.DUTDevice, nbr string) error {
	t.Helper()
	path := gnmi.OC().NetworkInstance(vrf100).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpProtocolName).Bgp().Neighbor(nbr).Messages().Sent()
	var errs []error
	if v := gnmi.Lookup(t, dut, path.LastNotificationErrorCode().State()); !v.IsPresent() {
		errs = append(errs, fmt.Errorf("neighbor %s: last-notification-error-code not present", nbr))
	} else if got, _ := v.Val(); got != oc.BgpTypes_BGP_ERROR_CODE_CEASE {
		errs = append(errs, fmt.Errorf("neighbor %s notification error code: got %v, want CEASE", nbr, got))
	}
	if v := gnmi.Lookup(t, dut, path.LastNotificationErrorSubcode().State()); !v.IsPresent() {
		errs = append(errs, fmt.Errorf("neighbor %s: last-notification-error-subcode not present", nbr))
	} else if got, _ := v.Val(); got != oc.BgpTypes_BGP_ERROR_SUBCODE_MAX_NUM_PREFIXES_REACHED {
		errs = append(errs, fmt.Errorf("neighbor %s notification error subcode: got %v, want MAX_NUM_PREFIXES_REACHED", nbr, got))
	}
	return errors.Join(errs...)
}

func setATEPeerState(t *testing.T, ate *ondatra.ATEDevice, up bool) {
	t.Helper()
	cs := gosnappi.NewControlState()
	target := gosnappi.StateProtocolBgpPeersState.DOWN
	if up {
		target = gosnappi.StateProtocolBgpPeersState.UP
	}
	cs.Protocol().Bgp().Peers().SetPeerNames([]string{bgpPeerName(atePort1.Name, cfgplugins.IPv4), bgpPeerName(atePort1.Name, cfgplugins.IPv6)}).SetState(target)
	ate.OTG().SetControlState(t, cs)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	cfgplugins.MplsConfig(t, dut)
	niBatch := &gnmi.SetBatch{}
	configureDUTNetworkInstances(t, dut, niBatch)
	niBatch.Set(t, dut)

	p1 := dut.Port(t, port1)
	cfgplugins.AssignToNetworkInstance(t, dut, p1.Name(), vrf100, subIfaceIndex)

	batch := &gnmi.SetBatch{}
	configureDUTInterfaces(t, dut, batch)
	configureRoutingPolicy(t, dut, batch)
	applyBGPConfig(t, dut, batch, bgpConfigOpts{gracefulRestart: true, v4PrefixLimit: prefixLimit, v6PrefixLimit: prefixLimit})
	batch.Set(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		p2 := dut.Port(t, port2)
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}

	if deviations.NetworkInstanceImportExportPolicyOCUnsupported(dut) {
		cfgplugins.ConfigureRouteTargetsCLI(t, dut, cfgplugins.BGPRoutePolicyConfig{
			VrfName:    vrf100,
			RD:         vrf100RD,
			RT:         vrf100RT,
			DutAS:      dutAS,
			PolicyName: rplName,
		})
		cfgplugins.ConfigureRouteTargetsCLI(t, dut, cfgplugins.BGPRoutePolicyConfig{
			VrfName:    vrf200,
			RD:         vrf200RD,
			RT:         vrf200RT,
			DutAS:      dutAS,
			PolicyName: rplName,
		})
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	cfg := buildATEConfig(t, ate, ateConfigOpts{enableCustGR: true, v4RouteCount: 1, v6RouteCount: 1})
	pushATEConfig(t, ate, cfg)
	return cfg
}

type testCase struct {
	name string
	run  func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) error
}

func testEBGPSessionInVRF(t *testing.T, dut *ondatra.DUTDevice, _ *ondatra.ATEDevice) error {
	return errors.Join(
		awaitCustBGPState(t, dut, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout),
		verifyVRF100RouterID(t, dut),
	)
}

func testL3VPNAttributeValidation(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) error {
	pushATEConfig(t, ate, buildATEConfig(t, ate, ateConfigOpts{enableCustGR: true, v4RouteCount: 1, v6RouteCount: 1}))
	if err := awaitCustBGPState(t, dut, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout); err != nil {
		return err
	}
	if err := awaitCoreBGPState(t, dut, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout); err != nil {
		return err
	}
	return errors.Join(
		verifyCustRoutesInAFT(t, dut, vrf100, true),
		verifyL3VPNExportConfig(t, dut),
		verifyCoreL3VPNAFIEnabled(t, dut),
		awaitCoreVPNPrefixAdvertisement(t, dut, true, bgpTimeout),
		awaitATECoreVPNPrefixPresence(t, ate, bgpTimeout),
	)
}

func testMaxPrefixLimit(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) error {
	if err := verifyPrefixLimitConfig(t, dut, prefixLimit); err != nil {
		return err
	}

	atLimit := buildATEConfig(t, ate, ateConfigOpts{enableCustGR: true, v4RouteCount: prefixLimit, v6RouteCount: prefixLimit})
	pushATEConfig(t, ate, atLimit)
	if err := awaitCustBGPState(t, dut, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout); err != nil {
		return err
	}

	for _, tc := range []struct {
		name                     string
		v4Count, v6Count         uint32
		downNeighbor, upNeighbor string
	}{
		{"IPv4", prefixLimit + 1, prefixLimit, atePort1.IPv4, atePort1.IPv6},
		{"IPv6", prefixLimit, prefixLimit + 1, atePort1.IPv6, atePort1.IPv4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer restoreCustomerBaseline(t, dut, ate)
			overLimit := buildATEConfig(t, ate, ateConfigOpts{enableCustGR: true, v4RouteCount: tc.v4Count, v6RouteCount: tc.v6Count})
			pushATEConfig(t, ate, overLimit)
			if err := errors.Join(
				awaitCustNeighborState(t, dut, tc.downNeighbor, oc.Bgp_Neighbor_SessionState_IDLE, bgpTimeout),
				awaitCustNeighborState(t, dut, tc.upNeighbor, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout),
				verifyMaxPrefixNotification(t, dut, tc.downNeighbor),
			); err != nil {
				t.Error(err)
			}
		})
	}
	return verifyCustRoutesInAFT(t, dut, vrf100, true)
}

func bgpClearConfig(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	resetBatch := &gnmi.SetBatch{}
	gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(vrf100).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config())
	if deviations.NetworkInstanceTableDeletionRequired(dut) {
		gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config())
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

func restoreCustomerBaseline(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	t.Helper()
	bgpClearConfig(t, dut)
	baseline := buildATEConfig(t, ate, ateConfigOpts{enableCustGR: true, v4RouteCount: 1, v6RouteCount: 1})
	pushATEConfig(t, ate, baseline)
	b := &gnmi.SetBatch{}
	applyBGPConfig(t, dut, b, bgpConfigOpts{gracefulRestart: true})
	b.Set(t, dut)
	if err := awaitCustBGPState(t, dut, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout); err != nil {
		t.Fatal(err)
	}
	b = &gnmi.SetBatch{}
	applyBGPConfig(t, dut, b, bgpConfigOpts{gracefulRestart: true, v4PrefixLimit: prefixLimit, v6PrefixLimit: prefixLimit})
	b.Set(t, dut)
	if deviations.NetworkInstanceImportExportPolicyOCUnsupported(dut) {
		cfgplugins.ConfigureRouteTargetsCLI(t, dut, cfgplugins.BGPRoutePolicyConfig{
			VrfName: vrf100, RD: vrf100RD, RT: vrf100RT, DutAS: dutAS, PolicyName: rplName,
		})
	}
	if err := awaitCustBGPState(t, dut, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout); err != nil {
		t.Fatal(err)
	}
}

func testIsolationBoundary(t *testing.T, dut *ondatra.DUTDevice, _ *ondatra.ATEDevice) error {
	return errors.Join(
		verifyCustRoutesInAFT(t, dut, vrf100, true),
		verifyCustRoutesInAFT(t, dut, defaultNI, false),
		verifyCustRoutesInAFT(t, dut, vrf200, false),
	)
}

func testGracefulRestartInVRF(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) error {
	pushATEConfig(t, ate, buildATEConfig(t, ate, ateConfigOpts{enableCustGR: true, v4RouteCount: 1, v6RouteCount: 1}))
	if err := awaitCustBGPState(t, dut, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout); err != nil {
		return err
	}
	if err := awaitCoreBGPState(t, dut, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout); err != nil {
		return err
	}
	var errs []error
	errs = append(errs, verifyCustomerGRNegotiation(t, dut, true))
	errs = append(errs, verifyCustRoutesInAFT(t, dut, vrf100, true))
	errs = append(errs, awaitCoreVPNPrefixAdvertisement(t, dut, true, bgpTimeout))
	corePrefixesBefore, err := coreVPNPrefixCounts(t, dut)
	errs = append(errs, err)
	transitionsBefore := establishedTransitions(t, dut)
	notificationsBefore := receivedNotificationCounts(t, dut)

	t.Log("Stopping ATE customer BGP peer without notification")
	setATEPeerState(t, ate, false)
	if err := awaitCustBGPDown(t, dut, bgpTimeout); err != nil {
		errs = append(errs, err)
		return errors.Join(errs...)
	}

	t.Log("Verifying prefixes retained during GR helper mode")
	errs = append(errs, verifyCustRoutesInAFT(t, dut, vrf100, true))
	errs = append(errs, awaitCoreVPNPrefixCounts(t, dut, corePrefixesBefore, aftTimeout))
	errs = append(errs, verifyNoNotificationReceived(t, dut, notificationsBefore))

	t.Log("Restoring ATE customer BGP peer within GR window")
	setATEPeerState(t, ate, true)
	if err := awaitCustBGPState(t, dut, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout); err != nil {
		errs = append(errs, err)
		return errors.Join(errs...)
	}
	errs = append(errs, verifyCustRoutesInAFT(t, dut, vrf100, true))
	errs = append(errs, awaitCoreVPNPrefixCounts(t, dut, corePrefixesBefore, aftTimeout))
	errs = append(errs, verifyNoNotificationReceived(t, dut, notificationsBefore))
	errs = append(errs, verifySingleReestablishment(t, dut, transitionsBefore))

	t.Log("Timeout scenario: keep customer peer down for longer than GR restart-time")
	setATEPeerState(t, ate, false)
	if err := awaitCustBGPDown(t, dut, bgpTimeout); err != nil {
		errs = append(errs, err)
		return errors.Join(errs...)
	}
	time.Sleep(time.Duration(grRestartTime)*time.Second + grHelperExtraWait)
	errs = append(errs, verifyCustRoutesInAFT(t, dut, vrf100, false))
	errs = append(errs, awaitCoreVPNPrefixWithdrawal(t, dut, corePrefixesBefore, aftTimeout))
	errs = append(errs, verifyATECoreVPNPrefixAbsence(t, ate))

	setATEPeerState(t, ate, true)
	if err := awaitCustBGPState(t, dut, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout); err != nil {
		errs = append(errs, err)
		return errors.Join(errs...)
	}
	errs = append(errs, verifyCustRoutesInAFT(t, dut, vrf100, true))
	errs = append(errs, awaitCoreVPNPrefixAdvertisement(t, dut, true, bgpTimeout))
	return errors.Join(errs...)
}

func testImmediateWithdrawalNoGR(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) error {
	b := &gnmi.SetBatch{}
	applyBGPConfig(t, dut, b, bgpConfigOpts{gracefulRestart: false, v4PrefixLimit: prefixLimit, v6PrefixLimit: prefixLimit})
	b.Set(t, dut)
	cfg := buildATEConfig(t, ate, ateConfigOpts{enableCustGR: false, v4RouteCount: 1, v6RouteCount: 1})
	pushATEConfig(t, ate, cfg)

	if err := awaitCustBGPState(t, dut, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout); err != nil {
		return err
	}
	if err := awaitCoreBGPState(t, dut, oc.Bgp_Neighbor_SessionState_ESTABLISHED, bgpTimeout); err != nil {
		return err
	}
	var errs []error
	errs = append(errs, verifyCustomerGRNegotiation(t, dut, false))
	errs = append(errs, verifyCustRoutesInAFT(t, dut, vrf100, true))
	errs = append(errs, awaitCoreVPNPrefixAdvertisement(t, dut, true, bgpTimeout))
	corePrefixesBefore, err := coreVPNPrefixCounts(t, dut)
	errs = append(errs, err)
	notificationsBefore := receivedNotificationCounts(t, dut)

	t.Log("Dropping customer BGP session without GR: prefixes must be withdrawn immediately")
	start := time.Now()
	setATEPeerState(t, ate, false)
	if err := awaitCustBGPDown(t, dut, immediateTimeout); err != nil {
		errs = append(errs, err)
		return errors.Join(errs...)
	}
	remaining := immediateTimeout - time.Since(start)
	if remaining <= 0 {
		errs = append(errs, fmt.Errorf("customer sessions did not go down within immediate-withdrawal bound %s", immediateTimeout))
		return errors.Join(errs...)
	}
	errs = append(errs, verifyCustRoutesInAFTWithin(t, dut, vrf100, false, remaining))
	errs = append(errs, awaitCoreVPNPrefixWithdrawal(t, dut, corePrefixesBefore, remaining))
	errs = append(errs, verifyATECoreVPNPrefixAbsence(t, ate))
	errs = append(errs, verifyNoNotificationReceived(t, dut, notificationsBefore))
	if elapsed := time.Since(start); elapsed > immediateTimeout {
		errs = append(errs, fmt.Errorf("local and core withdrawal took %s, want <= %s", elapsed, immediateTimeout))
	}
	return errors.Join(errs...)
}
