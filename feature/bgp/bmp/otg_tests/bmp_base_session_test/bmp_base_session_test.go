package bmp_base_session_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
)

const (
	dutAS                  = 64520
	ate1AS                 = 64530
	plenIPv4               = 30
	plenIPv6               = 126
	bmpStationPort         = 7039
	prefix1v4              = "192.0.2.0"
	prefix1v4Subnet        = 24
	prefix2v4              = "198.18.0.0"
	prefix2v4Subnet        = 16
	prefix1v6              = "2001:db8:1::"
	prefix1v6Subnet        = 64
	prefix2v6              = "2001:db8::"
	prefix2v6Subnet        = 64
	routeCountV4           = 5000
	routeCountV6           = 5000
	bmpName                = "atebmp"
	prefixSetIPv4Name      = "PREFIX-SET"
	prefixSetIPv4          = "198.18.0.0/16"
	prefixSubnetRange      = "16..32"
	prefixSetIPv6Name      = "PREFIX-SET-V6"
	prefixSetIPv6          = "2001:db8::/64"
	prefixV6SubnetRange    = "64..128"
	policyName             = "BMP-POLICY"
	prePolicyV4RouteCount  = 10000
	prePolicyV6RouteCount  = 10000
	postPolicyV4RouteCount = 9999
	postPolicyV6RouteCount = 9999
	timeout                = 60 * time.Second
	peerGroupV4            = "BGP-PEER-GROUP-V4"
	peerGroupV6            = "BGP-PEER-GROUP-V6"
)

type PolicyRoute struct {
	Address      string
	PrefixLength int
}

var (
	dutP1 = attrs.Attributes{
		Desc:    "DUT to ATE Port 1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8:2::1",
		MAC:     "02:00:01:02:02:02",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	ateP1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8:2::2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	dutP2 = attrs.Attributes{
		Desc:    "DUT to ATE Port 2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8:3::1",
		MAC:     "02:00:02:02:02:02",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	ateP2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8:3::2",
		MAC:     "02:00:02:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	postPolicyRoutes = []PolicyRoute{
		{
			Address:      prefix1v4,
			PrefixLength: prefix1v4Subnet,
		},
	}

	postPolicyRoutesDenied = []PolicyRoute{
		{
			Address:      prefix2v4,
			PrefixLength: prefix2v4Subnet,
		},
	}

	prePolicyRoutes = []PolicyRoute{
		{
			Address:      prefix1v4,
			PrefixLength: prefix1v4Subnet,
		},
		{
			Address:      prefix2v4,
			PrefixLength: prefix2v4Subnet,
		},
	}

	postPolicyRoutesV6 = []PolicyRoute{
		{
			Address:      prefix1v6,
			PrefixLength: prefix1v6Subnet,
		},
	}

	postPolicyRoutesV6Denied = []PolicyRoute{
		{
			Address:      prefix2v6,
			PrefixLength: prefix2v6Subnet,
		},
	}

	prePolicyRoutesV6 = []PolicyRoute{
		{
			Address:      prefix1v6,
			PrefixLength: prefix1v6Subnet,
		},
		{
			Address:      prefix2v6,
			PrefixLength: prefix2v6Subnet,
		},
	}
)

type ateConfigParams struct {
	atePort      gosnappi.Port
	atePortAttrs attrs.Attributes
	dutPortAttrs attrs.Attributes
	ateAS        uint32
	bmpName      string
}

// TestMain is the entry point for the test suite.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureDUT configures all DUT aspects.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) *gnmi.SetBatch {
	t.Helper()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	bmpConfigParams := cfgplugins.BMPConfigParams{
		DutAS:       dutAS,
		Source:      p2.Name(),
		StationPort: bmpStationPort,
		StationAddr: ateP2.IPv4,
	}

	batch := &gnmi.SetBatch{}
	gnmi.BatchReplace(batch, gnmi.OC().Interface(p1.Name()).Config(), dutP1.NewOCInterface(p1.Name(), dut))
	gnmi.BatchReplace(batch, gnmi.OC().Interface(p2.Name()).Config(), dutP2.NewOCInterface(p2.Name(), dut))
	cfgBGP := cfgplugins.BGPConfig{DutAS: dutAS, RouterID: dutP1.IPv4, EnableMaxRoutes: true, PeerGroups: []string{peerGroupV4, peerGroupV6}}
	dutBgpConf := cfgplugins.ConfigureDUTBGP(t, dut, batch, cfgBGP)
	configureDUTBGPNeighbors(t, dut, batch, dutBgpConf.Bgp)
	cfgplugins.ConfigureBMP(t, dut, batch, bmpConfigParams)

	batch.Set(t, dut)

	cfgplugins.ConfigureBMPAccessList(t, dut, batch, bmpConfigParams)

	batch.Set(t, dut)
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	return batch
}

func ptrToString(val string) *string {
	return &val
}

// configureDUTBGPNeighbors appends multiple BGP neighbor configurations to an existing BGP protocol on the DUT. Instead of calling AppendBGPNeighbor repeatedly in the test, this helper iterates over a slice of BGPNeighborConfig and applies each neighbor configuration into the given gnmi.SetBatch.
func configureDUTBGPNeighbors(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, bgp *oc.NetworkInstance_Protocol_Bgp) {
	t.Helper()
	// Add BGP neighbors
	neighbors := []cfgplugins.BGPNeighborConfig{
		{
			AteAS:            ate1AS,
			PortName:         dutP1.Name,
			NeighborIPv4:     ateP1.IPv4,
			NeighborIPv6:     ateP1.IPv6,
			IsLag:            false,
			PolicyName:       ptrToString(policyName),
			MultiPathEnabled: false,
		},
	}
	for _, n := range neighbors {
		cfgplugins.AppendBGPNeighbor(t, dut, batch, bgp, n)
	}

	rpl, err := configureBGPPolicy()
	if err != nil {
		t.Fatalf("Failed to configure BGP Policy: %v", err)
	}
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rpl)

}

func configureBGPPolicy() (*oc.RoutingPolicy, error) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pdef := rp.GetOrCreatePolicyDefinition(policyName)

	pset := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSetIPv4Name)
	pset.GetOrCreatePrefix(prefixSetIPv4, prefixSubnetRange)
	pset.SetMode(oc.PrefixSet_Mode_IPV4)

	stmt, err := pdef.AppendNewStatement("10")
	if err != nil {
		return nil, err
	}
	stmt.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	stmt.GetOrCreateConditions().GetOrCreateMatchPrefixSet().PrefixSet = ygot.String(prefixSetIPv4Name)

	pset2 := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSetIPv6Name)
	pset2.SetMode(oc.PrefixSet_Mode_IPV6)
	pset2.GetOrCreatePrefix(prefixSetIPv6, prefixV6SubnetRange)

	stmt2, err := pdef.AppendNewStatement("20")
	if err != nil {
		return nil, err
	}
	stmt2.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE
	stmt2.GetOrCreateConditions().GetOrCreateMatchPrefixSet().PrefixSet = ygot.String(prefixSetIPv6Name)

	stmt3, err := pdef.AppendNewStatement("30")
	if err != nil {
		return nil, err
	}
	stmt3.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	return rp, nil

}

// configureATE builds and returns the OTG configuration for the ATE topology.
func configureATE(t *testing.T, ate *ondatra.ATEDevice, bmpName string) gosnappi.Config {
	t.Helper()
	ateConfig := gosnappi.NewConfig()

	// Create ATE Ports
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	// First, define OTG ports
	atePort1 := ateConfig.Ports().Add().SetName(p1.ID())
	atePort2 := ateConfig.Ports().Add().SetName(p2.ID())

	ateP1ConfigParams := ateConfigParams{
		atePort:      atePort1,
		atePortAttrs: ateP1,
		dutPortAttrs: dutP1,
		ateAS:        ate1AS,
	}

	ateP2ConfigParams := ateConfigParams{
		atePort:      atePort2,
		atePortAttrs: ateP2,
		dutPortAttrs: dutP2,
		bmpName:      bmpName,
	}

	// ATE Device 1 (EBGP)
	configureBGPOnATEDevice(t, ateConfig, ateP1ConfigParams)
	// ATE Device 2 (BMP)
	configureBMPOnATEDevice(t, ateConfig, ateP2ConfigParams)
	return ateConfig
}

// configureBGPOnATEDevice configures BGP on an ATE device.
func configureBGPOnATEDevice(t *testing.T, cfg gosnappi.Config, params ateConfigParams) {
	t.Helper()
	var peerTypeV4 gosnappi.BgpV4PeerAsTypeEnum
	var peerTypeV6 gosnappi.BgpV6PeerAsTypeEnum

	dev := cfg.Devices().Add().SetName(params.atePortAttrs.Name)
	eth := dev.Ethernets().Add().SetName(params.atePortAttrs.Name + "Eth").SetMac(params.atePortAttrs.MAC)
	eth.Connection().SetPortName(params.atePort.Name())

	ip4 := eth.Ipv4Addresses().Add().SetName(params.atePortAttrs.Name + ".IPv4")
	ip4.SetAddress(params.atePortAttrs.IPv4).SetGateway(params.dutPortAttrs.IPv4).SetPrefix(uint32(params.atePortAttrs.IPv4Len))

	ip6 := eth.Ipv6Addresses().Add().SetName(params.atePortAttrs.Name + ".IPv6")
	ip6.SetAddress(params.atePortAttrs.IPv6).SetGateway(params.dutPortAttrs.IPv6).SetPrefix(uint32(params.atePortAttrs.IPv6Len))

	bgp := dev.Bgp().SetRouterId(params.atePortAttrs.IPv4)
	peerTypeV4 = gosnappi.BgpV4PeerAsType.EBGP
	peerTypeV6 = gosnappi.BgpV6PeerAsType.EBGP

	bgpV4 := bgp.Ipv4Interfaces().Add().SetIpv4Name(ip4.Name())
	v4Peer := bgpV4.Peers().Add().SetName(params.atePortAttrs.Name + ".BGPv4.Peer").SetPeerAddress(params.dutPortAttrs.IPv4).SetAsNumber(params.ateAS).SetAsType(peerTypeV4)

	bgpV6 := bgp.Ipv6Interfaces().Add().SetIpv6Name(ip6.Name())
	v6Peer := bgpV6.Peers().Add().SetName(params.atePortAttrs.Name + ".BGPv6.Peer").SetPeerAddress(params.dutPortAttrs.IPv6).SetAsNumber(params.ateAS).SetAsType(peerTypeV6)

	// Advertise host routes
	addBGPRoutes(v4Peer.V4Routes().Add(), params.atePortAttrs.Name+".Host.v4.1", prefix1v4, prefix1v4Subnet, routeCountV4, ip4.Address())
	addBGPRoutes(v6Peer.V6Routes().Add(), params.atePortAttrs.Name+".Host.v6.1", prefix1v6, prefix1v6Subnet, routeCountV6, ip6.Address())

	addBGPRoutes(v4Peer.V4Routes().Add(), params.atePortAttrs.Name+".Host.v4.2", prefix2v4, prefix2v4Subnet, routeCountV4, ip4.Address())
	addBGPRoutes(v6Peer.V6Routes().Add(), params.atePortAttrs.Name+".Host.v6.2", prefix2v6, prefix2v6Subnet, routeCountV6, ip6.Address())

}

// configureBMPOnATEDevice configures BMP on an ATE device.
func configureBMPOnATEDevice(t *testing.T, cfg gosnappi.Config, params ateConfigParams) {
	t.Helper()

	dev := cfg.Devices().Add().SetName(params.atePortAttrs.Name)
	eth := dev.Ethernets().Add().SetName(params.atePortAttrs.Name + "Eth").SetMac(params.atePortAttrs.MAC)
	eth.Connection().SetPortName(params.atePort.Name())

	ip4 := eth.Ipv4Addresses().Add().SetName(params.atePortAttrs.Name + ".IPv4")
	ip4.SetAddress(params.atePortAttrs.IPv4).SetGateway(params.dutPortAttrs.IPv4).SetPrefix(uint32(params.atePortAttrs.IPv4Len))

	ip6 := eth.Ipv6Addresses().Add().SetName(params.atePortAttrs.Name + ".IPv6")
	ip6.SetAddress(params.atePortAttrs.IPv6).SetGateway(params.dutPortAttrs.IPv6).SetPrefix(uint32(params.atePortAttrs.IPv6Len))

	// --- BMP Configuration ---
	bmpIntf := dev.Bmp().Ipv4Interfaces().Add()
	bmpIntf.SetIpv4Name(ip4.Name())
	bmpServer := bmpIntf.Servers().Add()
	bmpServer.SetName(params.bmpName)
	bmpServer.SetClientIp(params.dutPortAttrs.IPv4)
	bmpServer.Connection().Passive().SetListenPort(bmpStationPort)

	discard := bmpServer.PrefixStorage().Ipv4Unicast().Discard()
	discard.Exceptions().Add().
		SetIpv4Prefix(prefix1v4).
		SetPrefixLength(prefix1v4Subnet)
	discard.Exceptions().Add().
		SetIpv4Prefix(prefix2v4).
		SetPrefixLength(prefix2v4Subnet)

	discardv6 := bmpServer.PrefixStorage().Ipv6Unicast().Discard()
	discardv6.Exceptions().Add().
		SetIpv6Prefix(prefix1v6).
		SetPrefixLength(prefix1v6Subnet)
	discardv6.Exceptions().Add().
		SetIpv6Prefix(prefix2v6).
		SetPrefixLength(prefix2v6Subnet)

}

// addBGPRoutes adds BGP route advertisements to an ATE device.
func addBGPRoutes[R any](routes R, name, startAddress string, prefixLen, count uint32, nextHop string) {
	switch r := any(routes).(type) {
	case gosnappi.BgpV4RouteRange:
		r.SetName(name).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).SetNextHopIpv4Address(nextHop)
		r.Addresses().Add().SetAddress(startAddress).SetPrefix(prefixLen).SetCount(count)
	case gosnappi.BgpV6RouteRange:
		r.SetName(name).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).SetNextHopIpv6Address(nextHop)
		r.Addresses().Add().SetAddress(startAddress).SetPrefix(prefixLen).SetCount(count)
	}
}

func verifyBMPSessionOnATE(t *testing.T, ate *ondatra.ATEDevice, bmpName string) error {
	t.Helper()
	otg := ate.OTG()

	bmpServer := gnmi.OTG().BmpServer(bmpName)

	_, ok := gnmi.Watch(t, otg, bmpServer.SessionState().State(), 1*time.Minute, func(val *ygnmi.Value[otgtelemetry.E_BmpServer_SessionState]) bool {
		state, ok := val.Val()
		return ok && state == otgtelemetry.BmpServer_SessionState_UP
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "ATE BMP session state", bmpServer.State(), gnmi.Get(t, otg, bmpServer.State()))
		return fmt.Errorf("bmp Session state is not UP")
	}
	return nil
}

func verifyBMPStatisticsReporting(t *testing.T, ate *ondatra.ATEDevice, bmpName string) error {
	t.Helper()
	t.Log("Checking BMP statistics reporting on ATE before and after the interval")

	bmpServer := gnmi.OTG().BmpServer(bmpName)

	initialStatCounter := gnmi.Get(t, ate.OTG(), bmpServer.Counters().StatisticsMessagesReceived().State())
	t.Logf("Initial BMP statistics counter: %v", initialStatCounter)

	time.Sleep(60 * time.Second)

	updatedStatCounter := gnmi.Get(t, ate.OTG(), bmpServer.Counters().StatisticsMessagesReceived().State())
	t.Logf("Updated BMP statistics counter: %v", updatedStatCounter)

	if updatedStatCounter <= initialStatCounter {
		return fmt.Errorf("bmp statistics counter did not increment after 60 seconds. Initial: %v, Updated: %v",
			initialStatCounter, updatedStatCounter)
	}
	return nil

}

func verifyBMPPostPolicyRouteMonitoring(t *testing.T, ate *ondatra.ATEDevice, bmpName string) error {
	t.Helper()

	var aggErr error
	addErr := func(format string, args ...any) {
		aggErr = errors.Join(aggErr, fmt.Errorf(format, args...))
	}

	otg := ate.OTG()
	bmpServer := gnmi.OTG().BmpServer(bmpName)

	fptest.LogQuery(t, "Route Monitoring Updates", bmpServer.State(), gnmi.Get(t, otg, bmpServer.State()))

	// --- Pre-policy IPv4 ---
	_, ok := gnmi.Watch(
		t, otg, bmpServer.Counters().PrePolicyIpv4UnicastRoutesReceived().State(),
		timeout,
		func(val *ygnmi.Value[uint64]) bool {
			receiveState, present := val.Val()
			return present && receiveState == prePolicyV4RouteCount
		},
	).Await(t)
	if !ok {
		prePolicyV4Routes := gnmi.Get(t, otg, bmpServer.Counters().PrePolicyIpv4UnicastRoutesReceived().State())
		addErr("PrePolicyIpv4UnicastRoutesReceived mismatch (got=%d, expected=%d)", prePolicyV4Routes, prePolicyV4RouteCount)
	} else {
		t.Logf("PrePolicyIPv4Routes: %v", prePolicyV4RouteCount)
	}

	// --- Pre-policy IPv6 ---
	_, ok = gnmi.Watch(
		t, otg, bmpServer.Counters().PrePolicyIpv6UnicastRoutesReceived().State(),
		timeout,
		func(val *ygnmi.Value[uint64]) bool {
			receiveState, present := val.Val()
			return present && receiveState == prePolicyV6RouteCount
		},
	).Await(t)
	if !ok {
		prePolicyV6Routes := gnmi.Get(t, otg, bmpServer.Counters().PrePolicyIpv6UnicastRoutesReceived().State())
		addErr("PrePolicyIpv6UnicastRoutesReceived mismatch (got=%d, expected=%d)", prePolicyV6Routes, prePolicyV6RouteCount)
	} else {
		t.Logf("PrePolicyIPv6Routes: %v", prePolicyV6RouteCount)
	}

	// --- Post-policy IPv4 ---
	_, ok = gnmi.Watch(
		t, otg, bmpServer.Counters().PostPolicyIpv4UnicastRoutesReceived().State(),
		timeout,
		func(val *ygnmi.Value[uint64]) bool {
			receiveState, present := val.Val()
			return present && receiveState == postPolicyV4RouteCount
		},
	).Await(t)
	if !ok {
		postPolicyV4Routes := gnmi.Get(t, otg, bmpServer.Counters().PostPolicyIpv4UnicastRoutesReceived().State())
		addErr("PostPolicyIpv4UnicastRoutesReceived mismatch (got=%d, expected=%d)", postPolicyV4Routes, postPolicyV4RouteCount)
	} else {
		t.Logf("PostPolicyIPv4Routes: %v", postPolicyV4RouteCount)
	}

	// --- Post-policy IPv6 ---
	_, ok = gnmi.Watch(
		t, otg, bmpServer.Counters().PostPolicyIpv6UnicastRoutesReceived().State(),
		timeout,
		func(val *ygnmi.Value[uint64]) bool {
			receiveState, present := val.Val()
			return present && receiveState == postPolicyV6RouteCount
		},
	).Await(t)
	if !ok {
		postPolicyV6Routes := gnmi.Get(t, otg, bmpServer.Counters().PostPolicyIpv6UnicastRoutesReceived().State())
		addErr("PostPolicyIpv6UnicastRoutesReceived mismatch (got=%d, expected=%d)", postPolicyV6Routes, postPolicyV6RouteCount)
	} else {
		t.Logf("PostPolicyIPv6Routes: %v", postPolicyV6RouteCount)
	}

	return aggErr
}

func verifyBMPPostPolicyRouteMonitoringPerPrefix(t *testing.T, ate *ondatra.ATEDevice, bmpName string) error {
	t.Helper()

	var aggErr error
	addErr := func(format string, args ...any) {
		aggErr = errors.Join(aggErr, fmt.Errorf(format, args...))
	}

	path := gnmi.OTG().BmpServer(bmpName).PeerStateDatabase().PeerAny()

	for _, postPolicyRoute := range postPolicyRoutes {
		_, ok := gnmi.WatchAll(
			t, ate.OTG(),
			path.PostPolicyInRib().
				BmpUnicastIpv4Prefix(postPolicyRoute.Address, uint32(postPolicyRoute.PrefixLength), 1, 0).
				State(),
			2*time.Minute,
			func(v *ygnmi.Value[*otgtelemetry.BmpServer_PeerStateDatabase_Peer_PostPolicyInRib_BmpUnicastIpv4Prefix]) bool {
				prefix, present := v.Val()
				return present && prefix.GetAddress() == postPolicyRoute.Address
			},
		).Await(t)
		if !ok {
			addErr("IPv4 post-policy route not found in PostPolicyRib: %s/%d",
				postPolicyRoute.Address, postPolicyRoute.PrefixLength)
		}
	}

	for _, postPolicyRoutedenied := range postPolicyRoutesDenied {
		_, ok := gnmi.WatchAll(
			t, ate.OTG(),
			path.PostPolicyInRib().
				BmpUnicastIpv4Prefix(postPolicyRoutedenied.Address, uint32(postPolicyRoutedenied.PrefixLength), 1, 0).
				State(),
			10*time.Second,
			func(v *ygnmi.Value[*otgtelemetry.BmpServer_PeerStateDatabase_Peer_PostPolicyInRib_BmpUnicastIpv4Prefix]) bool {
				prefix, present := v.Val()
				return present && prefix.GetAddress() == postPolicyRoutedenied.Address
			},
		).Await(t)
		if ok {
			addErr("IPv4 post-policy denied route unexpectedly found in PostPolicyRib: %s/%d",
				postPolicyRoutedenied.Address, postPolicyRoutedenied.PrefixLength)
		}
	}

	for _, prePolicyRoute := range prePolicyRoutes {
		_, ok := gnmi.WatchAll(
			t, ate.OTG(),
			path.PrePolicyInRib().
				BmpUnicastIpv4Prefix(prePolicyRoute.Address, uint32(prePolicyRoute.PrefixLength), 1, 0).
				State(),
			2*time.Minute,
			func(v *ygnmi.Value[*otgtelemetry.BmpServer_PeerStateDatabase_Peer_PrePolicyInRib_BmpUnicastIpv4Prefix]) bool {
				prefix, present := v.Val()
				return present && prefix.GetAddress() == prePolicyRoute.Address
			},
		).Await(t)
		if !ok {
			addErr("IPv4 pre-policy route not found in PrePolicyRib: %s/%d",
				prePolicyRoute.Address, prePolicyRoute.PrefixLength)
		}
	}

	for _, postPolicyRoutev6 := range postPolicyRoutesV6 {
		_, ok := gnmi.WatchAll(
			t, ate.OTG(),
			path.PostPolicyInRib().
				BmpUnicastIpv6Prefix(postPolicyRoutev6.Address, uint32(postPolicyRoutev6.PrefixLength), 1, 0).
				State(),
			2*time.Minute,
			func(v *ygnmi.Value[*otgtelemetry.BmpServer_PeerStateDatabase_Peer_PostPolicyInRib_BmpUnicastIpv6Prefix]) bool {
				prefix, present := v.Val()
				return present && prefix.GetAddress() == postPolicyRoutev6.Address
			},
		).Await(t)
		if !ok {
			addErr("IPv6 post-policy route not found in PostPolicyRib: %s/%d",
				postPolicyRoutev6.Address, postPolicyRoutev6.PrefixLength)
		}
	}

	for _, postPolicyRouteV6Denied := range postPolicyRoutesV6Denied {
		_, ok := gnmi.WatchAll(
			t, ate.OTG(),
			path.PostPolicyInRib().
				BmpUnicastIpv6Prefix(postPolicyRouteV6Denied.Address, uint32(postPolicyRouteV6Denied.PrefixLength), 1, 0).
				State(),
			10*time.Second,
			func(v *ygnmi.Value[*otgtelemetry.BmpServer_PeerStateDatabase_Peer_PostPolicyInRib_BmpUnicastIpv6Prefix]) bool {
				prefix, present := v.Val()
				return present && prefix.GetAddress() == postPolicyRouteV6Denied.Address
			},
		).Await(t)
		if ok {
			addErr("IPv6 post-policy denied route unexpectedly found in PostPolicyRib: %s/%d",
				postPolicyRouteV6Denied.Address, postPolicyRouteV6Denied.PrefixLength)
		}
	}

	for _, prePolicyRoutev6 := range prePolicyRoutesV6 {
		_, ok := gnmi.WatchAll(
			t, ate.OTG(),
			path.PrePolicyInRib().
				BmpUnicastIpv6Prefix(prePolicyRoutev6.Address, uint32(prePolicyRoutev6.PrefixLength), 1, 0).
				State(),
			2*time.Minute,
			func(v *ygnmi.Value[*otgtelemetry.BmpServer_PeerStateDatabase_Peer_PrePolicyInRib_BmpUnicastIpv6Prefix]) bool {
				prefix, present := v.Val()
				return present && prefix.GetAddress() == prePolicyRoutev6.Address
			},
		).Await(t)
		if !ok {
			addErr("IPv6 pre-policy route not found in PrePolicyRib: %s/%d",
				prePolicyRoutev6.Address, prePolicyRoutev6.PrefixLength)
		}
	}

	return aggErr
}

func verifyPrefixCountV4(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	compare := func(val *ygnmi.Value[uint32]) bool {
		c, ok := val.Val()
		return ok && c == postPolicyV4RouteCount
	}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixes := statePath.Neighbor(ateP1.IPv4).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Prefixes()
	if got, ok := gnmi.Watch(t, dut, prefixes.Received().State(), 2*time.Minute, compare).Await(t); !ok {
		return fmt.Errorf("received prefixes v4 mismatch: got %v, want %v", got, postPolicyV4RouteCount)
	}
	return nil
}

func verifyPrefixCountV6(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	compare := func(val *ygnmi.Value[uint32]) bool {
		c, ok := val.Val()
		return ok && c == postPolicyV6RouteCount
	}
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	prefixes := statePath.Neighbor(ateP1.IPv6).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Prefixes()

	if got, ok := gnmi.Watch(t, dut, prefixes.Received().State(), 2*time.Minute, compare).Await(t); !ok {
		return fmt.Errorf("received prefixes v6 mismatch: got %v, want %v", got, postPolicyV6RouteCount)
	}
	return nil
}

func TestBMPBaseSession(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	t.Log("Start DUT Configuration")
	configureDUT(t, dut)
	t.Log("Start ATE Configuration")
	otgConfig := configureATE(t, ate, bmpName)
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Log("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut)

	t.Log("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	if err := verifyPrefixCountV4(t, dut); err != nil {
		t.Error(err)
	}
	if err := verifyPrefixCountV6(t, dut); err != nil {
		t.Error(err)
	}

	type testCase struct {
		name string
		fn   func(t *testing.T)
	}

	cases := []testCase{
		{
			name: "1.1.1_Verify_BMP_Session_Establishment",
			fn: func(t *testing.T) {

				t.Log("Verify BMP session on ATE")
				if err := verifyBMPSessionOnATE(t, ate, bmpName); err != nil {
					t.Fatal(err)
				} else {
					t.Log("BMP Session Established")
				}
			},
		},
		{
			name: "1.1.2_Verify_Statisitics_Reporting",
			fn: func(t *testing.T) {
				t.Log("Verify BMP session on DUT")
				if err := verifyBMPStatisticsReporting(t, ate, bmpName); err != nil {
					t.Error(err)
				}
			},
		},
		{
			name: "1.1.3_Verify_Route_Monitoring_Post_Policy",
			fn: func(t *testing.T) {

				t.Log("Verify Route Monitoring Post Policy on DUT")
				if err := verifyBMPPostPolicyRouteMonitoring(t, ate, bmpName); err != nil {
					t.Fatalf("BMP Post-Policy Route Monitoring validation failed: %v", err)
				}
				if err := verifyBMPPostPolicyRouteMonitoringPerPrefix(t, ate, bmpName); err != nil {
					t.Fatalf("BMP Post-Policy Route Monitoring validation failed: %v", err)
				}
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
