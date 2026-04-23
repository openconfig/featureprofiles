package aigp_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	// ASNs (Table 5)
	asnATE              = 64496
	asnDUT1Default      = 64497
	asnDUT1TestInstance = 64498
	asnDUT2Default      = 64497
	asnDUT2TestInstance = 64498
	asnDUT2Originate    = 64499

	// Network-instance names
	niDefault      = "DEFAULT"
	niTestInstance = "test-instance"
	niOriginate    = "test-originate"

	// Policy names
	importPolicy150 = "test-import-policy_aigp_150"
	importPolicy20  = "test-import-policy_aigp_20"
	exportPolicy    = "test-export-policy"
	exportPolicyv6  = "test-export-policyv6"
	exportDefaultV4 = "default-export-v4"
	exportDefaultV6 = "default-export-v6"
	exportNonDefV4  = "non-default-export-v4"
	exportNonDefV6  = "non-default-export-v6"

	// AIGP values
	aigp20  = 20
	aigp150 = 150
	aigp200 = 200
	aigp201 = 201
	aigp220 = 220

	// ISIS NET entities
	isisNetDUT1Default      = "49.0001.1980.5110.0025.00"
	isisNetDUT1TestInstance = "49.0001.1980.5110.0029.00"
	isisNetDUT2Default      = "49.0001.1980.5110.0026.00"
	isisNetDUT2TestInstance = "49.0001.1980.5110.0030.00"
	isisNetDUT2Originate    = "49.0001.1980.5110.0100.00"
	isisDefaultInstance     = "DEFAULT"
	isisTestInstance        = "test-instance"
	isisOriginateInstance   = "test-originate"

	// Traffic loss threshold
	trafficLossThresholdPct = 2.0

	trafficDuration = 60 * time.Second

	plenIPv4   = 30
	plenIPv6   = 126
	plenIPv4lo = 32
	plenIPv6lo = 128

	pgUplink        = "uplink"
	pgUplink6       = "uplink6"
	pgDownlink      = "downlink"     // default NI, peer-as 64497
	pgDownlink6     = "downlink6"    // default NI, peer-as 64497
	pgDownlinkTI    = "downlink-ti"  // test-instance NI, peer-as 64498
	pgDownlink6TI   = "downlink6-ti" // test-instance NI, peer-as 64498
	pgUplinkDUT2    = "uplink-dut2"  // default NI DUT2, peer-as 64497
	pgUplink6DUT2   = "uplink6-dut2" // default NI DUT2, peer-as 64497
	pgUplinkDUT2TI  = "uplink-ti"    // test-instance NI DUT2, peer-as 64498
	pgUplink6DUT2TI = "uplink6-ti"   // test-instance NI DUT2, peer-as 64498
)

type dutData struct {
	dut                 *ondatra.DUTDevice
	lagName             []string
	lagData             []*cfgplugins.DUTAggData
	loopbackData        []*loopbackAttrs
	routePolicyDataList []*cfgplugins.AIGPRoutePolicyData
	bgpCfg              map[string]*niBGPConfig
}

type loopbackAttrs struct {
	attr             *attrs.Attributes
	networkInstance  cfgplugins.NetworkInstanceParams
	loopbackIntfName string
}

type routePolicyData struct {
	policyName    string
	aigpMetric    uint32
	acceptRoute   bool
	statementName string
	nexthop       string
}

type bgpNbr struct {
	peerGrpName    string
	nbrIp          string
	peerAddr       string
	peerAs         uint32
	localAs        uint32
	isV4           bool
	routeReflector bool
	clusterId      string
	importPolicy   []string
	exportPolicy   []string
	enableAIGP     bool
	srcIp          string
	removePolicy   []string
}

type niBGPConfig struct {
	defaultAsn uint32
	globalAsn  uint32
	bgpNbrs    []*bgpNbr
	peerGroups []*peerGroupCfg
}

type peerGroupCfg struct {
	name           string
	peerAs         uint32
	localAs        uint32
	routeReflector bool
}

// ATEBgpPeer defines the configuration for an ATE BGP peer
type ATEBgpPeer struct {
	Name    string
	EthName string
	AddrV4  string
	DutV4   string
	AddrV6  string
	DutV6   string
	PeerAS  uint32
}

var (
	port1DstMac = ""

	testInstanceParams = cfgplugins.NetworkInstanceParams{
		Name:    niTestInstance,
		Default: false,
	}

	testOriginateParams = cfgplugins.NetworkInstanceParams{
		Name:    niOriginate,
		Default: false,
	}

	testDefaultParams = cfgplugins.NetworkInstanceParams{
		Name:    niDefault,
		Default: true,
	}

	activity = oc.Lacp_LacpActivityType_ACTIVE
	period   = oc.Lacp_LacpPeriodType_FAST

	lacpParams1 = &cfgplugins.LACPParams{
		Activity: &activity,
		Period:   &period,
	}

	activity2   = oc.Lacp_LacpActivityType_PASSIVE
	lacpParams2 = &cfgplugins.LACPParams{
		Activity: &activity2,
		Period:   &period,
	}

	dut1Lag1 = []*cfgplugins.DUTAggData{
		{
			// Attributes:      custIntf,
			OndatraPortsIdx: []int{0},
			LacpParams:      lacpParams1,
			AggType:         oc.IfAggregate_AggregationType_LACP,
			SubInterfaces: []*cfgplugins.DUTSubInterfaceData{
				{
					VlanID:        10,
					VlanEnable:    false,
					IPv4Address:   net.ParseIP("198.51.100.1"),
					IPv6Address:   net.ParseIP("2001:db8::1"),
					IPv4PrefixLen: plenIPv4,
					IPv6PrefixLen: plenIPv6,
				},
				{
					VlanID:        20,
					VlanEnable:    false,
					IPv4Address:   net.ParseIP("198.51.100.5"),
					IPv6Address:   net.ParseIP("2001:db8::5"),
					IPv4PrefixLen: plenIPv4,
					IPv6PrefixLen: plenIPv6,
				},
				{
					VlanID:                30,
					VlanEnable:            false,
					IPv4Address:           net.ParseIP("198.51.100.9"),
					IPv6Address:           net.ParseIP("2001:db8::51"),
					IPv4PrefixLen:         plenIPv4,
					IPv6PrefixLen:         plenIPv6,
					NetworkInstanceParams: testInstanceParams,
				},
				{
					VlanID:                40,
					VlanEnable:            false,
					IPv4Address:           net.ParseIP("198.51.100.13"),
					IPv6Address:           net.ParseIP("2001:db8::55"),
					IPv4PrefixLen:         plenIPv4,
					IPv6PrefixLen:         plenIPv6,
					NetworkInstanceParams: testInstanceParams,
				},
			},
		},
	}

	dut1Lag2 = []*cfgplugins.DUTAggData{
		{
			OndatraPortsIdx: []int{1},
			LacpParams:      lacpParams1,
			AggType:         oc.IfAggregate_AggregationType_LACP,
			SubInterfaces: []*cfgplugins.DUTSubInterfaceData{
				{
					VlanID:        10,
					VlanEnable:    false,
					IPv4Address:   net.ParseIP("198.51.100.17"),
					IPv6Address:   net.ParseIP("2001:db8::35"),
					IPv4PrefixLen: plenIPv4,
					IPv6PrefixLen: plenIPv6,
				},
				{
					VlanID:                20,
					VlanEnable:            false,
					IPv4Address:           net.ParseIP("198.51.100.21"),
					IPv6Address:           net.ParseIP("2001:db8::21"),
					IPv4PrefixLen:         plenIPv4,
					IPv6PrefixLen:         plenIPv6,
					NetworkInstanceParams: testInstanceParams,
				},
				{
					VlanID:        30,
					VlanEnable:    false,
					IPv4Address:   net.ParseIP("198.51.100.25"),
					IPv6Address:   net.ParseIP("2001:db8::25"),
					IPv4PrefixLen: plenIPv4,
					IPv6PrefixLen: plenIPv6,
				},
				{
					VlanID:                40,
					VlanEnable:            false,
					IPv4Address:           net.ParseIP("198.51.100.29"),
					IPv6Address:           net.ParseIP("2001:db8::29"),
					IPv4PrefixLen:         plenIPv4,
					IPv6PrefixLen:         plenIPv6,
					NetworkInstanceParams: testInstanceParams,
				},
			},
		},
	}

	dut2Lag2 = []*cfgplugins.DUTAggData{
		{
			OndatraPortsIdx: []int{0},
			LacpParams:      lacpParams2,
			AggType:         oc.IfAggregate_AggregationType_LACP,
			SubInterfaces: []*cfgplugins.DUTSubInterfaceData{
				{
					VlanID:        10,
					VlanEnable:    false,
					IPv4Address:   net.ParseIP("198.51.100.18"),
					IPv6Address:   net.ParseIP("2001:db8::36"),
					IPv4PrefixLen: plenIPv4,
					IPv6PrefixLen: plenIPv6,
				},
				{
					VlanID:                20,
					VlanEnable:            false,
					IPv4Address:           net.ParseIP("198.51.100.22"),
					IPv6Address:           net.ParseIP("2001:db8::22"),
					IPv4PrefixLen:         plenIPv4,
					IPv6PrefixLen:         plenIPv6,
					NetworkInstanceParams: testInstanceParams,
				},
				{
					VlanID:                30,
					VlanEnable:            false,
					IPv4Address:           net.ParseIP("198.51.100.26"),
					IPv6Address:           net.ParseIP("2001:db8::26"),
					IPv4PrefixLen:         plenIPv4,
					IPv6PrefixLen:         plenIPv6,
					NetworkInstanceParams: testOriginateParams,
				},
				{
					VlanID:                40,
					VlanEnable:            false,
					IPv4Address:           net.ParseIP("198.51.100.30"),
					IPv6Address:           net.ParseIP("2001:db8::2a"),
					IPv4PrefixLen:         plenIPv4,
					IPv6PrefixLen:         plenIPv6,
					NetworkInstanceParams: testOriginateParams,
				},
			},
		},
	}

	agg1 = &otgconfighelpers.Port{
		Name:        "Lag1",
		AggMAC:      "02:00:01:01:01:02",
		MemberPorts: []string{"port1"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{otgIntf10, otgIntf20, otgIntf30, otgIntf40},
		LagID:       1,
		IsLag:       true,
	}

	otgIntf10 = &otgconfighelpers.InterfaceProperties{
		Name:        "eth1.10",
		IPv4:        "198.51.100.2",
		IPv4Gateway: "198.51.100.1",
		IPv6:        "2001:db8::2",
		IPv6Gateway: "2001:db8::1",
		IPv4Len:     plenIPv4,
		IPv6Len:     plenIPv6,
		MAC:         "02:00:03:01:01:01",
		Vlan:        10,
	}

	otgIntf20 = &otgconfighelpers.InterfaceProperties{
		Name:        "eth1.20",
		IPv4:        "198.51.100.6",
		IPv4Gateway: "198.51.100.5",
		IPv6:        "2001:db8::6",
		IPv6Gateway: "2001:db8::5",
		IPv4Len:     plenIPv4,
		IPv6Len:     plenIPv6,
		MAC:         "02:00:03:01:02:02",
		Vlan:        20,
	}

	otgIntf30 = &otgconfighelpers.InterfaceProperties{
		Name:        "eth1.30",
		IPv4:        "198.51.100.10",
		IPv4Gateway: "198.51.100.9",
		IPv6:        "2001:db8::52",
		IPv6Gateway: "2001:db8::51",
		IPv4Len:     plenIPv4,
		IPv6Len:     plenIPv6,
		MAC:         "02:00:03:01:03:03",
		Vlan:        30,
	}

	otgIntf40 = &otgconfighelpers.InterfaceProperties{
		Name:        "eth1.40",
		IPv4:        "198.51.100.14",
		IPv4Gateway: "198.51.100.13",
		IPv6:        "2001:db8::56",
		IPv6Gateway: "2001:db8::55",
		IPv4Len:     plenIPv4,
		IPv6Len:     plenIPv6,
		MAC:         "02:00:03:01:04:04",
		Vlan:        40,
	}

	// DUT1 loopback 10
	dut1loopback10 = loopbackAttrs{
		attr: &attrs.Attributes{
			Desc:    "DUT1 Loopback 10",
			IPv4:    "198.55.1.1",
			IPv6:    "2001:db8:50::1",
			IPv4Len: plenIPv4lo,
			IPv6Len: plenIPv6lo,
		},
		networkInstance: testDefaultParams,
	}

	// DUT2 loopback 20
	dut1loopback20 = loopbackAttrs{
		attr: &attrs.Attributes{
			Desc:    "DUT2 Loopback 20",
			IPv4:    "198.55.2.1",
			IPv6:    "2001:db8:60::1",
			IPv4Len: plenIPv4lo,
			IPv6Len: plenIPv6lo,
		},
		networkInstance: testInstanceParams,
	}

	// DUT2 loopback 10
	dut2loopback10 = loopbackAttrs{
		attr: &attrs.Attributes{
			Desc:    "DUT2 Loopback 10",
			IPv4:    "198.60.1.1",
			IPv6:    "2001:db8:60::1",
			IPv4Len: plenIPv4lo,
			IPv6Len: plenIPv6lo,
		},
		networkInstance: testOriginateParams,
	}

	// DUT2 loopback 20
	dut2loopback20 = loopbackAttrs{
		attr: &attrs.Attributes{
			Desc:    "DUT2 Loopback 20",
			IPv4:    "198.70.1.1",
			IPv6:    "2001:db8:70::1",
			IPv4Len: plenIPv4lo,
			IPv6Len: plenIPv6lo,
		},
		networkInstance: testOriginateParams,
	}

	ateIPv4Prefixes = []string{"198.51.210.0", "198.51.220.0"}
	ateIPv6Prefixes = []string{"2001:db8:10::", "2001:db8:20::"}

	networkInstanceList = []cfgplugins.NetworkInstanceParams{testInstanceParams, testOriginateParams}

	dutTestData = []*dutData{
		{
			lagData:      append(dut1Lag1, dut1Lag2...),
			loopbackData: []*loopbackAttrs{&dut1loopback10, &dut1loopback20},
			routePolicyDataList: []*cfgplugins.AIGPRoutePolicyData{
				{
					PolicyName:    "test-import-policy_aigp_20",
					StatementName: "test-import-statement",
					AigpMetric:    20,
					AcceptRoute:   true,
				},
				{
					PolicyName:    "test-import-policy_aigp_150",
					StatementName: "test-import-statement",
					AigpMetric:    150,
					AcceptRoute:   true,
				},
				{
					PolicyName:    "test-export-policy",
					StatementName: "test-export-statement",
					AigpMetric:    200,
					AcceptRoute:   true,
					Nexthop:       "SELF",
					NexthopType:   "ip",
				},
				{
					PolicyName:    "test-export-policyv6",
					StatementName: "test-export-statement",
					AigpMetric:    200,
					AcceptRoute:   true,
					Nexthop:       "SELF",
					NexthopType:   "ipv6",
				},
			},
			bgpCfg: map[string]*niBGPConfig{
				niDefault: {
					defaultAsn: asnDUT1Default,
					globalAsn:  asnDUT1Default,
					peerGroups: []*peerGroupCfg{
						{name: pgUplink, peerAs: asnATE},                                  // towards ATE
						{name: pgUplink6, peerAs: asnATE},                                 // towards ATE
						{name: pgDownlink, peerAs: asnDUT1Default, routeReflector: true},  // towards DUT2 default NI
						{name: pgDownlink6, peerAs: asnDUT1Default, routeReflector: true}, // towards DUT2 default NI
						{name: pgDownlinkTI, peerAs: asnDUT1TestInstance},                 // 64498 - test-instance downlink
						{name: pgDownlink6TI, peerAs: asnDUT1TestInstance},                // 64498 - test-instance downlinkv6
					},
					bgpNbrs: []*bgpNbr{
						{peerGrpName: pgUplink, nbrIp: otgIntf10.IPv4, peerAs: asnATE, isV4: true, importPolicy: []string{importPolicy150}, enableAIGP: true},
						{peerGrpName: pgUplink6, nbrIp: otgIntf10.IPv6, peerAs: asnATE, isV4: false, importPolicy: []string{importPolicy150}, enableAIGP: true},
						{peerGrpName: pgUplink, nbrIp: otgIntf20.IPv4, peerAs: asnATE, isV4: true, importPolicy: []string{importPolicy20}, enableAIGP: true},
						{peerGrpName: pgUplink6, nbrIp: otgIntf20.IPv6, peerAs: asnATE, isV4: false, importPolicy: []string{importPolicy20}, enableAIGP: true},
						// Default NI - downlink peers uses 64497 (DUT1 -- DUT2 vlan10)
						{peerGrpName: pgDownlink, nbrIp: "198.51.100.18", peerAs: asnDUT1Default, isV4: true, exportPolicy: []string{exportPolicy}, routeReflector: true, clusterId: "1.1.1.1", enableAIGP: true},
						{peerGrpName: pgDownlink6, nbrIp: "2001:db8::36", peerAs: asnDUT1Default, isV4: false, exportPolicy: []string{exportPolicyv6}, routeReflector: true, clusterId: "1.1.1.1", enableAIGP: true},
					},
				},
				niTestInstance: {
					defaultAsn: asnDUT1Default,
					globalAsn:  asnDUT1TestInstance,
					bgpNbrs: []*bgpNbr{
						// Test-instance NI - uplink peers uses 64496 (ATE -- DUT1 vlan30, vlan40)
						{peerGrpName: pgUplink, nbrIp: otgIntf30.IPv4, peerAs: asnATE, isV4: true, importPolicy: []string{importPolicy150}, enableAIGP: true},
						{peerGrpName: pgUplink6, nbrIp: otgIntf30.IPv6, peerAs: asnATE, isV4: false, importPolicy: []string{importPolicy150}, enableAIGP: true},
						{peerGrpName: pgUplink, nbrIp: otgIntf40.IPv4, peerAs: asnATE, isV4: true, importPolicy: []string{importPolicy20}, enableAIGP: true},
						{peerGrpName: pgUplink6, nbrIp: otgIntf40.IPv6, peerAs: asnATE, isV4: false, importPolicy: []string{importPolicy20}, enableAIGP: true},
						// Test-instance NI - downlink peers uses 64498 (DUT1 -- DUT2 vlan20) — use distinct pg names
						{peerGrpName: pgDownlinkTI, nbrIp: "198.51.100.22", peerAs: asnDUT1TestInstance, isV4: true, exportPolicy: []string{exportPolicy}, routeReflector: true, clusterId: "1.1.1.1", enableAIGP: true},
						{peerGrpName: pgDownlink6TI, nbrIp: "2001:db8::22", peerAs: asnDUT1TestInstance, isV4: false, exportPolicy: []string{exportPolicyv6}, routeReflector: true, clusterId: "1.1.1.1", enableAIGP: true},
					},
				},
			},
		},
		{
			lagData: dut2Lag2,
			bgpCfg: map[string]*niBGPConfig{
				niDefault: {
					defaultAsn: asnDUT2Default,
					globalAsn:  asnDUT2Default,
					peerGroups: []*peerGroupCfg{
						{name: pgUplinkDUT2, peerAs: asnDUT2Default},         // towards DUT1 default NI
						{name: pgUplink6DUT2, peerAs: asnDUT2Default},        // towards DUT1 default NI
						{name: pgUplinkDUT2TI, peerAs: asnDUT2TestInstance},  // towards DUT1 test-instance NI
						{name: pgUplink6DUT2TI, peerAs: asnDUT2TestInstance}, // towards DUT1 test-instance NI
					},
					bgpNbrs: []*bgpNbr{
						// Default NI uses uplink peers with 64497 (DUT2 -- DUT1 vlan10)
						{peerGrpName: pgUplinkDUT2, nbrIp: "198.51.100.17", peerAs: asnDUT2Default, isV4: true, enableAIGP: true},
						{peerGrpName: pgUplink6DUT2, nbrIp: "2001:db8::35", peerAs: asnDUT2Default, isV4: false, enableAIGP: true},
					},
				},
				niTestInstance: {
					defaultAsn: asnDUT2Default,
					globalAsn:  asnDUT2TestInstance,
					bgpNbrs: []*bgpNbr{
						// test-instance NI uses uplink peers with 64498 (DUT2 -- DUT1 vlan20) — use distinct pg names
						{peerGrpName: pgUplinkDUT2TI, nbrIp: "198.51.100.21", peerAs: asnDUT2TestInstance, isV4: true, enableAIGP: true},
						{peerGrpName: pgUplink6DUT2TI, nbrIp: "2001:db8::21", peerAs: asnDUT2TestInstance, isV4: false, enableAIGP: true},
					},
				},
			},
		},
	}
)

func configureNetworkInstances(t *testing.T, dutDataList []*dutData) {
	for _, dutData := range dutDataList {
		for _, ni := range networkInstanceList {
			if ni.Name == niDefault {
				cfgplugins.ConfigureNetworkInstance(t, dutData.dut, deviations.DefaultNetworkInstance(dutData.dut), false)
			}
			networkInstance := cfgplugins.ConfigureNetworkInstance(t, dutData.dut, ni.Name, false)
			cfgplugins.UpdateNetworkInstanceOnDut(t, dutData.dut, ni.Name, networkInstance)
		}
	}
}

func configureDut(t *testing.T, dutDataList []*dutData) {
	for _, dutData := range dutDataList {
		b := &gnmi.SetBatch{}
		for _, l := range dutData.lagData {
			// Create LAG interface
			l.LagName = netutil.NextAggregateInterface(t, dutData.dut)
			dutData.lagName = append(dutData.lagName, l.LagName)
			cfgplugins.NewAggregateInterface(t, dutData.dut, b, l)
			b.Set(t, dutData.dut)

			for _, subIntf := range l.SubInterfaces {
				vlanClientCfg := cfgplugins.VlanClientEncapsulationParams{
					IntfName:         l.LagName,
					Subinterfaces:    uint32(subIntf.VlanID),
					RemoveVlanConfig: false,
				}

				cfgplugins.VlanClientEncapsulation(t, b, dutData.dut, vlanClientCfg)
			}
		}

		//Configure loopback interfaces
		if len(dutData.loopbackData) > 0 {
			configureLoopback(t, b, dutData.dut, dutData.loopbackData)
			b.Set(t, dutData.dut)
		}

		// Configure BGP import policy to set AIGP metric
		if len(dutData.routePolicyDataList) > 0 {
			for _, policyData := range dutData.routePolicyDataList {
				cfgplugins.RoutingPolicyBGPAIGP(t, dutData.dut, *policyData)
			}
		}

		if len(dutData.bgpCfg) > 0 {
			configureBGP(t, dutData.dut, dutData.bgpCfg)
		}
	}
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	otgConfig := gosnappi.NewConfig()

	// Configure OTG Interfaces
	for _, agg := range []*otgconfighelpers.Port{agg1} {
		otgconfighelpers.ConfigureNetworkInterface(t, otgConfig, ate, agg)
	}

	// BGP peers using ATEBgpPeer struct
	ateBgpPeers := []ATEBgpPeer{
		{Name: "bgp10", EthName: "eth1.10.IPv4", AddrV4: otgIntf10.IPv4, DutV4: otgIntf10.IPv4Gateway, AddrV6: otgIntf10.IPv6, DutV6: otgIntf10.IPv6Gateway, PeerAS: asnATE},
		{Name: "bgp20", EthName: "eth1.20.IPv4", AddrV4: otgIntf20.IPv4, DutV4: otgIntf20.IPv4Gateway, AddrV6: otgIntf20.IPv6, DutV6: otgIntf20.IPv6Gateway, PeerAS: asnATE},
		{Name: "bgp30", EthName: "eth1.30.IPv4", AddrV4: otgIntf30.IPv4, DutV4: otgIntf30.IPv4Gateway, AddrV6: otgIntf30.IPv6, DutV6: otgIntf30.IPv6Gateway, PeerAS: asnATE},
		{Name: "bgp40", EthName: "eth1.40.IPv4", AddrV4: otgIntf40.IPv4, DutV4: otgIntf40.IPv4Gateway, AddrV6: otgIntf40.IPv6, DutV6: otgIntf40.IPv6Gateway, PeerAS: asnATE},
	}
	for i, spec := range ateBgpPeers {
		bgp := otgConfig.Devices().Items()[i].Bgp().SetRouterId(spec.AddrV4)
		peerIntf := bgp.Ipv4Interfaces().Add().SetIpv4Name(spec.EthName)
		peer4 := peerIntf.Peers().Add().SetName(spec.Name + ".v4").
			SetPeerAddress(spec.DutV4).SetAsNumber(spec.PeerAS).
			SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
		peer4.LearnedInformationFilter().SetUnicastIpv4Prefix(true)

		// IPv6 Peer
		peer6Int := bgp.Ipv6Interfaces().Add().SetIpv6Name(spec.EthName[:len(spec.EthName)-2] + "v6")
		peer6 := peer6Int.Peers().Add().SetName(spec.Name + ".v6").
			SetPeerAddress(spec.DutV6).SetAsNumber(spec.PeerAS).
			SetAsType(gosnappi.BgpV6PeerAsType.EBGP)
		peer6.LearnedInformationFilter().SetUnicastIpv6Prefix(true)

		// Advertised routes
		for _, pfx := range ateIPv4Prefixes {
			v4routes := peerIntf.Peers().Items()[0].V4Routes().Add().SetName(spec.Name + "_route_" + pfx)
			v4routes.Addresses().Add().SetAddress(pfx).SetPrefix(24).SetCount(1)
		}

		for _, pfx := range ateIPv6Prefixes {
			v6routes := peer6Int.Peers().Items()[0].V6Routes().Add().SetName(spec.Name + "_route_" + pfx)
			v6routes.Addresses().Add().SetAddress(pfx).SetPrefix(64).SetCount(1)
		}
	}

	// Traffic flows (Table 7)
	flows := []struct {
		name   string
		srcIP  string
		dstIP  string
		vlan   int
		srcMAC string
		v4     bool
	}{
		{"Flow1", ateIPv4Prefixes[0], dut1loopback10.attr.IPv4, 10, otgIntf10.MAC, true},
		{"Flow2", ateIPv4Prefixes[1], dut1loopback20.attr.IPv4, 30, otgIntf30.MAC, true},
		{"Flow3", ateIPv6Prefixes[0], dut1loopback10.attr.IPv6, 10, otgIntf10.MAC, false},
		{"Flow4", ateIPv6Prefixes[1], dut1loopback20.attr.IPv6, 30, otgIntf30.MAC, false},
	}
	for _, f := range flows {
		flow := otgConfig.Flows().Add().SetName(f.name)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Port().SetTxName(otgConfig.Lags().Items()[0].Name()).SetRxNames([]string{otgConfig.Lags().Items()[0].Name()})
		flow.Size().SetFixed(512)
		flow.Rate().SetPercentage(5)
		flow.Duration().FixedPackets().SetPackets(100)
		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(f.srcMAC)
		eth.Dst().SetValue(port1DstMac)
		if f.vlan > 0 {
			vl := flow.Packet().Add().Vlan()
			vl.Id().SetValue(uint32(f.vlan))
		}
		if f.v4 {
			ipv4 := flow.Packet().Add().Ipv4()
			ipv4.Src().SetValue(f.srcIP)
			ipv4.Dst().SetValue(f.dstIP)
			icmp := flow.Packet().Add().Icmp()
			icmp.SetEcho(gosnappi.NewFlowIcmpEcho())
		} else {
			ipv6 := flow.Packet().Add().Ipv6()
			ipv6.Src().SetValue(f.srcIP)
			ipv6.Dst().SetValue(f.dstIP)
			icmp := flow.Packet().Add().Icmpv6()
			icmp.SetEcho(gosnappi.NewFlowIcmpv6Echo())
		}

	}

	ate.OTG().PushConfig(t, otgConfig)
	return otgConfig
}

func configureLoopback(t *testing.T, batch *gnmi.SetBatch, dut *ondatra.DUTDevice, dutloopback []*loopbackAttrs) {
	// Configure interface loopback
	for i, dutloop := range dutloopback {
		loopbackIntfName := netutil.LoopbackInterface(t, dut, i)
		dutloopback[i].loopbackIntfName = loopbackIntfName
		loop1 := dutloop.attr.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.BatchUpdate(batch, gnmi.OC().Interface(loopbackIntfName).Config(), loop1)

		cfgplugins.AssignInterfaceToNetworkInstance(t, batch, dut, loopbackIntfName, &dutloop.networkInstance, uint32(0), false)
	}
}

func configureBGP(t *testing.T, dut *ondatra.DUTDevice, bgpCfg map[string]*niBGPConfig) {
	t.Helper()
	d := &oc.Root{}

	orderedNIs := []string{niDefault}

	for ni := range bgpCfg {
		if ni != niDefault {
			orderedNIs = append(orderedNIs, ni)
		}
	}

	for _, ni := range orderedNIs {
		niCfg, ok := bgpCfg[ni]
		if !ok {
			continue
		}
		resolvedNI := ni
		if ni == niDefault {
			resolvedNI = deviations.DefaultNetworkInstance(dut)
		}

		// Create BGP protocol object for this NI
		niObj := d.GetOrCreateNetworkInstance(resolvedNI)
		proto := niObj.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
		bgp := proto.GetOrCreateBgp()

		bgp.GetOrCreateGlobal().SetAs(niCfg.globalAsn)

		for _, pg := range niCfg.peerGroups {
			p := bgp.GetOrCreatePeerGroup(pg.name)
			p.SetPeerGroupName(pg.name)
			p.SetPeerAs(pg.peerAs)
			p.GetOrCreateRouteReflector().SetRouteReflectorClient(pg.routeReflector)

			if pg.localAs != 0 {
				p.SetLocalAs(pg.localAs)
			}
		}

		// Configure each neighbor into this NI's BGP object
		for _, nbr := range niCfg.bgpNbrs {
			configureBGPNeighbor(t, dut, resolvedNI, niCfg.defaultAsn, bgp, nbr)
		}

		bgpPath := gnmi.OC().NetworkInstance(resolvedNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
		gnmi.Update(t, dut, bgpPath.Config(), proto)
	}
}

func configureBGPNeighbor(t *testing.T, dut *ondatra.DUTDevice, ni string, defaultAsn uint32, bgp *oc.NetworkInstance_Protocol_Bgp, nbr *bgpNbr) {
	var importPolicy, exportPolicy string

	n := bgp.GetOrCreateNeighbor(nbr.nbrIp)
	n.SetPeerGroup(nbr.peerGrpName)
	n.SetPeerAs(nbr.peerAs)
	n.SetEnabled(true)

	if nbr.localAs != 0 {
		n.SetLocalAs(nbr.localAs)
	}

	if nbr.srcIp != "" {
		bgpNbrT := n.GetOrCreateTransport()
		localAddressLeaf := nbr.srcIp

		bgpNbrT.SetLocalAddress(localAddressLeaf)
	}

	if nbr.routeReflector {
		rr := n.GetOrCreateRouteReflector()
		rr.SetRouteReflectorClient(true)
		if nbr.clusterId != "" {
			// rr.SetRouteReflectorClusterId(oc.UnionString(nbr.clusterId))
			cliConfig := fmt.Sprintf(`router bgp %d
			neighbor %s route-reflector cluster-id %s`, defaultAsn, nbr.nbrIp, nbr.clusterId)
			helpers.GnmiCLIConfig(t, dut, cliConfig)
		}
	}

	if len(nbr.exportPolicy) > 0 {
		exportPolicy = nbr.exportPolicy[0]
	}
	if len(nbr.importPolicy) > 0 {
		importPolicy = nbr.importPolicy[0]
	}

	if nbr.isV4 {
		af := n.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af.SetEnabled(true)
		cliConfig := fmt.Sprintf("router bgp %d\n", defaultAsn)

		if nbr.enableAIGP {
			if ni != deviations.DefaultNetworkInstance(dut) {
				cliConfig += fmt.Sprintf(`vrf %s
								  address-family ipv4
								  aigp-session ibgp
								  neighbor %s aigp-session
								`, ni, nbr.nbrIp)
			} else {
				cliConfig += fmt.Sprintf(`address-family ipv4
								  aigp-session ibgp
								  neighbor %s aigp-session
								`, nbr.nbrIp)
			}
		}
		helpers.GnmiCLIConfig(t, dut, cliConfig)

		routeMapPolicyParams := cfgplugins.NeighborRouteMapAttributes{
			NetworkInstance:      ni,
			As:                   defaultAsn,
			NeighborIp:           nbr.nbrIp,
			ImportRouteMapPolicy: importPolicy,
			V4:                   true,
			RemoveRouteMapPolicy: false,
			ExportRouteMapPolicy: exportPolicy,
		}
		cfgplugins.ApplyRoutePolicyToBGPPeer(t, dut, routeMapPolicyParams)

	} else {
		af := n.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af.SetEnabled(true)
		cliConfig := fmt.Sprintf("router bgp %d\n", defaultAsn)

		if nbr.enableAIGP {
			if ni != deviations.DefaultNetworkInstance(dut) {
				cliConfig += fmt.Sprintf(`vrf %s
								  address-family ipv6
								  aigp-session ibgp
								  neighbor %s aigp-session
								`, ni, nbr.nbrIp)
			} else {
				cliConfig += fmt.Sprintf(`address-family ipv6
								  aigp-session ibgp
								  neighbor %s aigp-session
								`, nbr.nbrIp)
			}
		}
		helpers.GnmiCLIConfig(t, dut, cliConfig)

		routeMapPolicyParams := cfgplugins.NeighborRouteMapAttributes{
			NetworkInstance:      ni,
			As:                   defaultAsn,
			NeighborIp:           nbr.nbrIp,
			ImportRouteMapPolicy: importPolicy,
			V4:                   false,
			RemoveRouteMapPolicy: false,
			ExportRouteMapPolicy: exportPolicy,
		}
		cfgplugins.ApplyRoutePolicyToBGPPeer(t, dut, routeMapPolicyParams)

	}
}

func configureISIS(t *testing.T, dut *ondatra.DUTDevice, ni string, intfName []string, dutAreaAddress, loopbackIntf string, isisInstance string, metric uint32) {
	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(ni)
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	globalISIS.Net = []string{dutAreaAddress}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	if deviations.ISISLevelEnabled(dut) {
		isisLevel2.Enabled = ygot.Bool(true)
	}

	for _, intf := range intfName {
		isisIntf := isis.GetOrCreateInterface(intf)
		isisIntf.Enabled = ygot.Bool(true)
		isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		// Configure ISIS level at global mode if true else at interface mode
		if deviations.ISISInterfaceLevel1DisableRequired(dut) {
			isisIntf.GetOrCreateLevel(1).Enabled = ygot.Bool(false)
		} else {
			isisIntf.GetOrCreateLevel(2).Enabled = ygot.Bool(true)
		}
		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi.SetMetric(metric)
		isisIntfLevelAfi.SetEnabled(true)

		isisIntfLevelAfi6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi6.SetMetric(metric)
		isisIntfLevelAfi6.SetEnabled(true)

		if deviations.ISISInterfaceAfiUnsupported(dut) {
			isisIntfLevel.Af = nil
		}
	}

	if loopbackIntf != "" {
		loopIface := isis.GetOrCreateInterface(loopbackIntf)
		loopIface.Enabled = ygot.Bool(true)
	}

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Config(), prot)
}

func updateISISMetric(t *testing.T, dut *ondatra.DUTDevice, ni string, isisInstance string, intfName []string, metric uint32) {
	t.Helper()
	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(ni)
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	for _, intf := range intfName {
		isisIntf := isis.GetOrCreateInterface(intf)
		isisIntf.Enabled = ygot.Bool(true)

		isisIntfLevel := isisIntf.GetOrCreateLevel(2)
		isisIntfLevel.Enabled = ygot.Bool(true)
		isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi.Metric = ygot.Uint32(metric)
		isisIntfLevelAfi6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		isisIntfLevelAfi6.Metric = ygot.Uint32(metric)

	}
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Config(), prot)
}

func verifyISISTelemetry(t *testing.T, dut *ondatra.DUTDevice, ni string, isisInstance string, dutIntf []string) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()
	for _, intfName := range dutIntf {
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			intfName = intfName + ".0"
		}
		nbrPath := statePath.Interface(intfName)
		query := nbrPath.LevelAny().AdjacencyAny().AdjacencyState().State()
		_, ok := gnmi.WatchAll(t, dut, query, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
			state, present := val.Val()
			if present && state == oc.Isis_IsisInterfaceAdjState_UP {
				t.Logf("IS-IS state on %v has adjacencies", intfName)
				return true
			}
			return false
		}).Await(t)
		if !ok {
			t.Logf("IS-IS state on %v has no adjacencies", intfName)
			t.Fatal("No IS-IS adjacencies reported.")
		}
	}
}

// verifyInterfaceOperStatus checks that an interface is operationally UP.
func verifyInterfaceOperStatus(t *testing.T, dut *ondatra.DUTDevice, intfName string) {
	t.Helper()
	t.Log("verifying interface operational status is UP for interface on dut ", dut.Name(), ": ", intfName)
	status := gnmi.Get(t, dut, gnmi.OC().Interface(intfName).OperStatus().State())
	if status != oc.Interface_OperStatus_UP {
		t.Errorf("interface %s on %s: want UP, got %v", intfName, dut.Name(), status)
	}
}

func verifyAIGPEnabled(t *testing.T, dut *ondatra.DUTDevice, ni, nbrAddr string, wantEnabled bool, v4 bool) {
	t.Helper()
	// bgpAIGPPath := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(nbrAddr)
	if v4 {
		// enabled = gnmi.GetAll(t, dut, bgpAIGPPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Config().EnableAigp().State())
		// if enabled != wantEnabled {
		// 	t.Errorf("BGP neighbor %s in NI %s: AIGP/enabled want %v, got %v", nbrAddr, ni, wantEnabled, enabled)
		// }
		t.Errorf("canonical OC is not supported for BGP AIGP enablement")
	} else {
		t.Errorf("canonical OC is not supported for BGP AIGP enablement")
		// enabled = gnmi.GetAll(t, dut, bgpAIGPPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Config().EnableAigp().State())
		// if enabled != wantEnabled {
		// t.Errorf("BGP neighbor %s in NI %s: AIGP/enabled want %v, got %v", nbrAddr, ni, wantEnabled, enabled)
	}
}

func verifyAIGPInRIB(t *testing.T, dut *ondatra.DUTDevice, ni, nbrAddr, prefix string, wantAIGP uint64, isV4 bool, nexthop string, validatePropagation bool) {
	t.Helper()
	var attrIndex uint64
	var found bool

	if !deviations.BGPRibOcPathUnsupported(dut) {
		bgpRIBPath := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Rib()
		if isV4 {
			locRib := gnmi.Get(t, dut, bgpRIBPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().LocRib().Route(prefix, oc.UnionString(nbrAddr), 0).State())
			if locRib != nil {
				attrIndex = locRib.GetAttrIndex()
				found = true
			}
			if validatePropagation {
				if locRib.GetValidRoute() != true {
					t.Errorf("route for prefix %s from neighbor %s in NI %s is not valid in RIB", prefix, nbrAddr, ni)
				}
			}
		} else {
			time.Sleep(30 * time.Second) // wait for route to be in RIB
			locRib := gnmi.Get(t, dut, bgpRIBPath.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().LocRib().Route(prefix, oc.UnionString(nbrAddr), 0).State())
			if locRib != nil {
				attrIndex = locRib.GetAttrIndex()
				found = true
			}
			if validatePropagation {
				if locRib.GetValidRoute() != true {
					t.Errorf("route for prefix %s from neighbor %s in NI %s is not valid in RIB", prefix, nbrAddr, ni)
				}
			}
		}
		if !found {
			t.Errorf("route for prefix %s from neighbor %s in NI %s not found", prefix, nbrAddr, ni)
		} else {
			validateAIGPMetric(t, dut, ni, nbrAddr, attrIndex, wantAIGP, nexthop)
		}
	} else {
		t.Errorf("deviation is not supported")
	}

}

func validateAIGPMetric(t *testing.T, dut *ondatra.DUTDevice, ni string, nbrAddr string, attrIndex uint64, wantAIGP uint64, nexthop string) {
	attrSet := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Rib().AttrSet(attrIndex).State())
	gotAIGP := attrSet.GetAigp()
	if gotAIGP != wantAIGP {
		t.Errorf("fail: aigp for neighbor %s in NI %s: want %d, got %d", nbrAddr, ni, wantAIGP, gotAIGP)
		t.Errorf("OC not supported for AIGP attribute in BGP RIB")
	} else {
		t.Logf("pass: aigp for neighbor %s in NI %s: want %d, got %d", nbrAddr, ni, wantAIGP, gotAIGP)
	}

	if nexthop != "" {
		gotNextHop := attrSet.GetNextHop()
		if gotNextHop != nexthop {
			t.Errorf("fail: next-hop for neighbor %s in NI %s: want %s, got %s", nbrAddr, ni, nexthop, gotNextHop)
		} else {
			t.Logf("pass: next-hop for neighbor %s in NI %s: want %s, got %s", nbrAddr, ni, nexthop, gotNextHop)
		}
	}
}

// verifyBestPath asserts that the given prefix received from nbrAddr is the best-path in adj-rib-in-post.
func verifyBestPath(t *testing.T, dut *ondatra.DUTDevice, ni, nbrAddr, prefix string, wantBest uint64, isV4 bool) {
	t.Helper()
	var ipUnicast string
	if deviations.BgpAdjRibOcUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			if isV4 {
				ipUnicast = "IPV4_UNICAST"
			} else {
				ipUnicast = "IPV6_UNICAST"
			}
			gpbGetRequest := &gpb.GetRequest{
				Path: []*gpb.Path{
					{
						Elem: []*gpb.PathElem{
							{Name: "network-instances"},
							{Name: "network-instance", Key: map[string]string{"name": "default"}},
							{Name: "protocols"},
							{Name: "protocol", Key: map[string]string{"identifier": "BGP", "name": "BGP"}},
							{Name: "bgp"},
							{Name: "neighbors"},
							{Name: "neighbor", Key: map[string]string{"neighbor-address": nbrAddr}},
							{Name: "afi-safis"},
							{Name: "afi-safi", Key: map[string]string{"afi-safi-name": ipUnicast}},
							{Name: "state"},
							{Name: "prefixes"},
							{Name: "best-paths"},
						},
					},
				},
				Type:     gpb.GetRequest_STATE,
				Encoding: gpb.Encoding_JSON_IETF,
			}

			gnmiClient := dut.RawAPIs().GNMI(t)
			if getResponse, err := gnmiClient.Get(context.Background(), gpbGetRequest); err != nil {
				t.Fatalf("Unexpected error getting counters: %v", err)
			} else {
				update := getResponse.GetNotification()[0].GetUpdate()[0]
				val := update.GetVal()
				bestPaths := val.Value.(*gnmipb.TypedValue_UintVal).UintVal
				if bestPaths != wantBest {
					t.Errorf("best-paths for prefix %s from %s in NI %s: want %v, got %d", prefix, nbrAddr, ni, wantBest, bestPaths)
				} else {
					t.Logf("best-paths got: %d expected: %d", bestPaths, wantBest)
				}
			}
		default:
			t.Errorf("deviation is not supported for vendor %s", dut.Vendor())
		}
	} else {
		var wantBestFlag bool

		if wantBest != 0 {
			wantBestFlag = true
		}
		bgpRib := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Rib()

		if isV4 {
			route := gnmi.Get(t, dut, bgpRib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().Neighbor(nbrAddr).AdjRibInPost().Route(prefix, 0).State())
			if route.GetBestPath() != wantBestFlag {
				t.Errorf("best-path for prefix %s from %s in NI %s: want %v, got %v",
					prefix, nbrAddr, ni, wantBest, route.GetBestPath())
			}
			return

		} else {
			route := gnmi.Get(t, dut, bgpRib.AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Ipv6Unicast().Neighbor(nbrAddr).AdjRibInPost().Route(prefix, 0).State())
			if route.GetBestPath() != wantBestFlag {
				t.Errorf("best-path for prefix %s from %s in NI %s: want %v, got %v",
					prefix, nbrAddr, ni, wantBest, route.GetBestPath())
			}
			return
		}
	}
}

// validateTrafficFlows verifies traffic flow behavior (pass/fail) based on expected outcome
func validateTrafficFlows(t *testing.T, otg *otg.OTG, otgConfig gosnappi.Config, flows []gosnappi.Flow) {

	otgutils.LogFlowMetrics(t, otg, otgConfig)

	for _, flow := range flows {
		outPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
		inPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))
		lossPct := ((outPkts - inPkts) * 100) / outPkts

		t.Logf("Flow %s: OutPkts=%v, InPkts=%v, LossPct=%v", flow.Name(), outPkts, inPkts, lossPct)

		if outPkts == 0 {
			t.Fatalf("OutPkts for flow %s is 0, want > 0", flow.Name())
		}

		// Expecting traffic to pass (0% loss)
		if got := lossPct; got > trafficLossThresholdPct {
			t.Errorf("traffic validation FAILED: Flow %s has %v%% packet loss, want < %v%%", flow.Name(), got, trafficLossThresholdPct)
		} else {
			t.Logf("Traffic validation PASSED: Flow %s has 0%% packet loss", flow.Name())
		}
	}
}

func verifyISISMetric(t *testing.T, dut *ondatra.DUTDevice, ni, isisInstance, intfID string, level uint8, afiName oc.E_IsisTypes_AFI_TYPE, safiName oc.E_IsisTypes_SAFI_TYPE, wantMetric uint32) {
	t.Helper()
	metric := gnmi.Get(t, dut,
		gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis().Interface(intfID).Level(uint8(level)).Af(afiName, safiName).Metric().State(),
	)
	if metric != wantMetric {
		t.Errorf("isis metric on %s in NI %s level %d AFI %v: want %d, got %d",
			intfID, ni, level, afiName, wantMetric, metric)
	}
}

func verifyBGPUpdatesSentNonZero(t *testing.T, dut *ondatra.DUTDevice, ni, nbrAddr string) {
	t.Helper()
	sent := gnmi.Get(t, dut,
		gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Neighbor(nbrAddr).Messages().Sent().State(),
	)
	if sent.GetUPDATE() == 0 {
		t.Errorf("bgp neighbor %s in NI %s has sent zero UPDATE messages", nbrAddr, ni)
	}
}

// verifyRouteReflectorClientState checks peer-group route-reflector-client state.
func verifyRouteReflectorClientState(t *testing.T, dut *ondatra.DUTDevice, ni, pgName string, wantClient bool) {
	t.Helper()
	rrClient := gnmi.Get(t, dut, gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().PeerGroup(pgName).RouteReflector().RouteReflectorClient().State())
	if rrClient != wantClient {
		t.Errorf("peer-group %s in NI %s: route-reflector-client want %v, got %v",
			pgName, ni, wantClient, rrClient)
	}
}

func enableAIGPOnPeer(t *testing.T, dut *ondatra.DUTDevice, ni string, defaultAsn uint32, nbrIp string, v4 bool, enable bool) {
	d := &oc.Root{}
	niObj := d.GetOrCreateNetworkInstance(ni)
	proto := niObj.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := proto.GetOrCreateBgp()

	n := bgp.GetOrCreateNeighbor(nbrIp)
	if v4 {
		af := n.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		af.SetEnabled(true)
		cliConfig := fmt.Sprintf("router bgp %d\n", defaultAsn)
		if ni != deviations.DefaultNetworkInstance(dut) {
			cliConfig += fmt.Sprintf(`vrf %s
								  address-family ipv4
								`, ni)
		} else {
			cliConfig += fmt.Sprintf("address-family ipv4\n")
		}
		if enable {
			cliConfig += fmt.Sprintf(`
								  neighbor %s aigp-session
								`, nbrIp)
		} else {
			cliConfig += fmt.Sprintf(`
								  no neighbor %s aigp-session
								`, nbrIp)
		}
		helpers.GnmiCLIConfig(t, dut, cliConfig)
	} else {
		af := n.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		af.SetEnabled(true)
		cliConfig := fmt.Sprintf("router bgp %d\n", defaultAsn)
		if ni != deviations.DefaultNetworkInstance(dut) {
			cliConfig += fmt.Sprintf(`vrf %s
								  address-family ipv6
								`, ni)
		} else {
			cliConfig += fmt.Sprintf("address-family ipv6\n")
		}
		if enable {
			cliConfig += fmt.Sprintf(`neighbor %s aigp-session
								`, nbrIp)
		} else {
			cliConfig += fmt.Sprintf(`no neighbor %s aigp-session
								`, nbrIp)
		}
		helpers.GnmiCLIConfig(t, dut, cliConfig)
	}
}

func TestAigp(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dutTestData[0].dut = dut1

	dut2 := ondatra.DUT(t, "dut2")
	dutTestData[1].dut = dut2

	ate := ondatra.ATE(t, "ate")

	// Configure network instances on DUT1 and DUT2
	configureNetworkInstances(t, dutTestData)

	// Configure DUT1 and DUT2
	configureDut(t, dutTestData)
	port1DstMac = gnmi.Get(t, dut1, gnmi.OC().Interface(dutTestData[0].lagName[0]).Ethernet().MacAddress().State())

	// Configure on OTG
	otgConfig := configureOTG(t, ate)
	type testCase struct {
		name        string
		description string
		testFunc    func(t *testing.T, dutTestData []*dutData, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow)
	}

	testCases := []testCase{
		{
			name:        "AIGP Attribute Validation",
			description: "Validate the AIGP modification with BGP policy, propagation control on BGP peers and next-hop self feature",
			testFunc:    testAIGPAttributeValidation,
		},
		{
			name:        "AS-PATH Attribute Tie-Breaker Validation",
			description: "Validate other Attribute(AS-PATH) as tie-breaker when AIGP is the same",
			testFunc:    testASPathAttributeTieBreaker,
		},
		{
			name:        "Validate AIGP Propagation using Next-Hop IP",
			description: "Validate AIGP propagation, propagation enabled by default on IBGP peers and next-hop IP feature",
			testFunc:    testAIGPPropagationNexthop,
		},
		{
			name:        "Validate AIGP Increment Plus one",
			description: "Validate the Plus 1 incremental feature of AIGP when the IGP metric to original destination is zero",
			testFunc:    testAIGPIncrementPlusOne,
		},
		{
			name:        "Validate AIGP attribute drop",
			description: "Validate AIGP attribute is dropped when AIGP propagation is disabled",
			testFunc:    testAIGPDisable,
		},
		{
			name:        "Validate AAIGP propagation in BGP peer-group",
			description: "Validate AIGP propagation in BGP peer-group",
			testFunc:    testAIGPPropagationPeerGroup,
		},
	}
	// Run the test cases.
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s - %s", tc.name, tc.name)
			tc.testFunc(t, dutTestData, ate, ate.OTG(), otgConfig, otgconfighelpers.Flow{})
		})
	}
}

func testAIGPAttributeValidation(t *testing.T, dutTestData []*dutData, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	dut1 := dutTestData[0].dut
	dut2 := dutTestData[1].dut

	// Start Protocols
	otg.StartProtocols(t)

	// Pass Criteria: 1 & 2- Validate interfaces are up
	for _, dutData := range dutTestData {
		t.Logf("Validating interfaces on %s", dutData.dut.Name())
		for _, l := range dutData.lagData {
			verifyInterfaceOperStatus(t, dutData.dut, l.LagName)
		}
	}

	// Pass Criteria: 4(DUT1) ; 1(DUT2)- Validate BGP sessions are established on otg and DUTs
	t.Logf("Verify OTG BGP sessions up")
	cfgplugins.VerifyOTGBGPEstablished(t, ate)

	t.Logf("Verify DUT BGP sessions are up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut1)
	cfgplugins.VerifyDUTBGPEstablished(t, dut2)

	t.Logf("Verify DUT vrf test-instance BGP sessions are up")
	cfgplugins.VerifyDUTBGPEstablishedForVRF(t, dut1, niTestInstance)
	cfgplugins.VerifyDUTBGPEstablishedForVRF(t, dut2, niTestInstance)

	// Pass Criteria: 5 - Validate AIGP propagation status

	// DUT1 vlan10 and vlan20 peers in default NI
	dut1NIPeers := []string{otgIntf10.IPv4, otgIntf10.IPv6, otgIntf20.IPv4, otgIntf20.IPv6}
	for _, p := range dut1NIPeers {
		verifyAIGPEnabled(t, dut1, deviations.DefaultNetworkInstance(dut1), p, true, strings.Contains(p, "IPv4"))
	}

	// DUT1 vlan30 and vlan40 peers in test-instance NI
	dut1TIPeers := []string{otgIntf30.IPv4, otgIntf30.IPv6, otgIntf40.IPv4, otgIntf40.IPv6}
	for _, p := range dut1TIPeers {
		verifyAIGPEnabled(t, dut1, niTestInstance, p, true, strings.Contains(p, "IPv4"))
	}

	// DUT2 vlan10 lag2 default NI peers
	dut2NIPeers := []string{"198.51.100.17", "2001:db8::35"}
	for _, p := range dut2NIPeers {
		verifyAIGPEnabled(t, dut2, deviations.DefaultNetworkInstance(dut2), p, true, strings.Contains(p, "."))
	}

	// DUT2 vlan10 lag2 test-instance NI peers
	dut2TIPeers := []string{"198.51.100.21", "2001:db8::21"}
	for _, p := range dut2TIPeers {
		verifyAIGPEnabled(t, dut2, niTestInstance, p, true, strings.Contains(p, "."))
	}

	// Fetch BGP rib attributes of the routes received from the peers
	for _, pfx := range []struct {
		prefix        string
		isV4          bool
		nbrIp         string
		aigpMetric    uint64
		ni            string
		validateRoute bool
	}{
		{prefix: ateIPv4Prefixes[0] + "/24", isV4: true, nbrIp: otgIntf10.IPv4, aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv4Prefixes[1] + "/24", isV4: true, nbrIp: otgIntf10.IPv4, aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv4Prefixes[0] + "/24", isV4: true, nbrIp: otgIntf20.IPv4, aigpMetric: aigp20, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv4Prefixes[1] + "/24", isV4: true, nbrIp: otgIntf20.IPv4, aigpMetric: aigp20, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv6Prefixes[0] + "/64", isV4: false, nbrIp: otgIntf10.IPv6, aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv6Prefixes[1] + "/64", isV4: false, nbrIp: otgIntf10.IPv6, aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv6Prefixes[0] + "/64", isV4: false, nbrIp: otgIntf20.IPv6, aigpMetric: aigp20, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv6Prefixes[1] + "/64", isV4: false, nbrIp: otgIntf20.IPv6, aigpMetric: aigp20, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv4Prefixes[0] + "/24", isV4: true, nbrIp: otgIntf30.IPv4, aigpMetric: aigp150, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv4Prefixes[1] + "/24", isV4: true, nbrIp: otgIntf30.IPv4, aigpMetric: aigp150, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv4Prefixes[0] + "/24", isV4: true, nbrIp: otgIntf40.IPv4, aigpMetric: aigp20, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv4Prefixes[1] + "/24", isV4: true, nbrIp: otgIntf40.IPv4, aigpMetric: aigp20, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv6Prefixes[0] + "/64", isV4: false, nbrIp: otgIntf30.IPv6, aigpMetric: aigp150, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv6Prefixes[1] + "/64", isV4: false, nbrIp: otgIntf30.IPv6, aigpMetric: aigp150, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv6Prefixes[0] + "/64", isV4: false, nbrIp: otgIntf40.IPv6, aigpMetric: aigp20, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv6Prefixes[1] + "/64", isV4: false, nbrIp: otgIntf40.IPv6, aigpMetric: aigp20, ni: niTestInstance, validateRoute: true},
	} {
		if pfx.ni != niTestInstance {
			verifyAIGPInRIB(t, dut1, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, "", pfx.validateRoute)
		} else {
			verifyAIGPInRIB(t, dut1, niTestInstance, pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, "", pfx.validateRoute)
		}
	}

	for _, pfx := range []struct {
		prefix        string
		isV4          bool
		nbrIp         string
		aigpMetric    uint64
		ni            string
		nexthop       string
		validateRoute bool
	}{
		{prefix: ateIPv4Prefixes[0] + "/24", isV4: true, nbrIp: "198.51.100.17", aigpMetric: aigp200, ni: deviations.DefaultNetworkInstance(dut1), nexthop: "198.51.100.17", validateRoute: true},
		{prefix: ateIPv4Prefixes[1] + "/24", isV4: true, nbrIp: "198.51.100.17", aigpMetric: aigp200, ni: deviations.DefaultNetworkInstance(dut1), nexthop: "198.51.100.17", validateRoute: true},
		{prefix: ateIPv4Prefixes[0] + "/24", isV4: true, nbrIp: "198.51.100.21", aigpMetric: aigp200, ni: niTestInstance, nexthop: "198.51.100.21", validateRoute: true},
		{prefix: ateIPv4Prefixes[1] + "/24", isV4: true, nbrIp: "198.51.100.21", aigpMetric: aigp200, ni: niTestInstance, nexthop: "198.51.100.21", validateRoute: true},
		{prefix: ateIPv6Prefixes[0] + "/64", isV4: false, nbrIp: "2001:db8::35", aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), nexthop: "2001:db8::35", validateRoute: true},
		{prefix: ateIPv6Prefixes[1] + "/64", isV4: false, nbrIp: "2001:db8::35", aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), nexthop: "2001:db8::35", validateRoute: true},
		{prefix: ateIPv6Prefixes[0] + "/64", isV4: false, nbrIp: "2001:db8::21", aigpMetric: aigp20, ni: niTestInstance, nexthop: "2001:db8::21", validateRoute: true},
		{prefix: ateIPv6Prefixes[1] + "/64", isV4: false, nbrIp: "2001:db8::21", aigpMetric: aigp20, ni: niTestInstance, nexthop: "2001:db8::21", validateRoute: true},
	} {
		if pfx.ni != niTestInstance {
			verifyAIGPInRIB(t, dut2, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, pfx.nexthop, pfx.validateRoute)
		} else {
			verifyAIGPInRIB(t, dut2, niTestInstance, pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, pfx.nexthop, pfx.validateRoute)
		}
	}

	// verify routes from are now best-path due to higher AIGP metric
	for _, pfx := range []struct {
		prefix   string
		isV4     bool
		nbrIp    string
		bestPath uint64
	}{
		{prefix: ateIPv4Prefixes[0] + "/24", isV4: true, nbrIp: otgIntf20.IPv4, bestPath: 2},
		{prefix: ateIPv4Prefixes[1] + "/24", isV4: true, nbrIp: otgIntf20.IPv4, bestPath: 2},
		{prefix: ateIPv6Prefixes[0] + "/64", isV4: false, nbrIp: otgIntf20.IPv6, bestPath: 2},
		{prefix: ateIPv6Prefixes[1] + "/64", isV4: false, nbrIp: otgIntf20.IPv6, bestPath: 2},
	} {
		verifyBestPath(t, dut1, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.bestPath, pfx.isV4)
	}

	// Traffic test
	otg.StartTraffic(t)
	validateTrafficFlows(t, otg, otgConfig, otgConfig.Flows().Items())

}

func testASPathAttributeTieBreaker(t *testing.T, dutTestData []*dutData, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	dut1 := dutTestData[0].dut
	dut2 := dutTestData[1].dut

	// Update import policy: set AIGP=150 AND prepend ASN 64496 x5
	updatedPolicy := cfgplugins.AIGPRoutePolicyData{
		PolicyName:    importPolicy20,
		StatementName: "test-import-statement",
		AigpMetric:    aigp150,
		AcceptRoute:   true,
		PrependASN:    asnATE,
		PrependRepeat: 5,
	}
	cfgplugins.RoutingPolicyBGPAIGP(t, dut1, updatedPolicy)

	otg.StartProtocols(t)

	t.Logf("Verify DUT BGP sessions up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut1)
	cfgplugins.VerifyDUTBGPEstablished(t, dut2)

	// verify aigp propagation with updated metric
	dut1NIPeers := []string{otgIntf10.IPv4, otgIntf10.IPv6, otgIntf20.IPv4, otgIntf20.IPv6}
	for _, p := range dut1NIPeers {
		verifyAIGPEnabled(t, dut1, deviations.DefaultNetworkInstance(dut1), p, true, strings.Contains(p, "IPv4"))
	}

	// DUT1 vlan30 and vlan40 peers in test-instance NI
	dut1TIPeers := []string{otgIntf30.IPv4, otgIntf30.IPv6, otgIntf40.IPv4, otgIntf40.IPv6}
	for _, p := range dut1TIPeers {
		verifyAIGPEnabled(t, dut1, niTestInstance, p, true, strings.Contains(p, "IPv4"))
	}

	// verify routes have aigp metric set
	for _, pfx := range []struct {
		prefix        string
		isV4          bool
		nbrIp         string
		aigpMetric    uint64
		ni            string
		validateRoute bool
	}{
		{prefix: ateIPv4Prefixes[0] + "/24", isV4: true, nbrIp: otgIntf10.IPv4, aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv4Prefixes[1] + "/24", isV4: true, nbrIp: otgIntf10.IPv4, aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv6Prefixes[0] + "/64", isV4: false, nbrIp: otgIntf10.IPv6, aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv6Prefixes[1] + "/64", isV4: false, nbrIp: otgIntf10.IPv6, aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv4Prefixes[0] + "/24", isV4: true, nbrIp: otgIntf20.IPv4, aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv4Prefixes[1] + "/24", isV4: true, nbrIp: otgIntf20.IPv4, aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv6Prefixes[0] + "/64", isV4: false, nbrIp: otgIntf20.IPv6, aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv6Prefixes[1] + "/64", isV4: false, nbrIp: otgIntf20.IPv6, aigpMetric: aigp150, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: ateIPv4Prefixes[0] + "/24", isV4: true, nbrIp: otgIntf30.IPv4, aigpMetric: aigp150, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv4Prefixes[1] + "/24", isV4: true, nbrIp: otgIntf30.IPv4, aigpMetric: aigp150, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv6Prefixes[0] + "/64", isV4: false, nbrIp: otgIntf30.IPv6, aigpMetric: aigp150, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv6Prefixes[1] + "/64", isV4: false, nbrIp: otgIntf30.IPv6, aigpMetric: aigp150, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv4Prefixes[0] + "/24", isV4: true, nbrIp: otgIntf40.IPv4, aigpMetric: aigp150, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv4Prefixes[1] + "/24", isV4: true, nbrIp: otgIntf40.IPv4, aigpMetric: aigp150, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv6Prefixes[0] + "/64", isV4: false, nbrIp: otgIntf40.IPv6, aigpMetric: aigp150, ni: niTestInstance, validateRoute: true},
		{prefix: ateIPv6Prefixes[1] + "/64", isV4: false, nbrIp: otgIntf40.IPv6, aigpMetric: aigp150, ni: niTestInstance, validateRoute: true},
	} {
		if pfx.ni != niTestInstance {
			verifyAIGPInRIB(t, dut1, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, "", pfx.validateRoute)
		} else {
			verifyAIGPInRIB(t, dut1, niTestInstance, pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, "", pfx.validateRoute)
		}
	}

	// verify routes from are now best-path due to higher AIGP metric
	for _, pfx := range []struct {
		prefix   string
		isV4     bool
		nbrIp    string
		bestPath uint64
	}{
		{prefix: ateIPv4Prefixes[0] + "/24", isV4: true, nbrIp: otgIntf10.IPv4, bestPath: 2},
		{prefix: ateIPv4Prefixes[1] + "/24", isV4: true, nbrIp: otgIntf10.IPv4, bestPath: 2},
		{prefix: ateIPv6Prefixes[0] + "/64", isV4: false, nbrIp: otgIntf10.IPv6, bestPath: 2},
		{prefix: ateIPv6Prefixes[1] + "/64", isV4: false, nbrIp: otgIntf10.IPv6, bestPath: 2},
	} {
		verifyBestPath(t, dut1, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.bestPath, pfx.isV4)
	}
}

func testAIGPPropagationNexthop(t *testing.T, dutTestData []*dutData, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	dut1 := dutTestData[0].dut
	dut2 := dutTestData[1].dut

	// Configure loopback on DUT2
	b := &gnmi.SetBatch{}
	configureLoopback(t, b, dut2, []*loopbackAttrs{&dut2loopback10, &dut2loopback20})
	b.Set(t, dut2)

	t.Log(dut1loopback10.loopbackIntfName)
	t.Log(dut2loopback10.loopbackIntfName)

	// Create BGP export policy
	routingPolicies := []cfgplugins.AIGPRoutePolicyData{
		{
			PolicyName:    exportDefaultV4,
			StatementName: "test-statement",
			AcceptRoute:   true,
			Nexthop:       dut1loopback10.attr.IPv4,
			NexthopType:   "ip",
		},
		{
			PolicyName:    exportDefaultV6,
			StatementName: "test-statement",
			AcceptRoute:   true,
			Nexthop:       dut1loopback10.attr.IPv6,
			NexthopType:   "ipv6",
		},
		{
			PolicyName:    exportNonDefV4,
			StatementName: "test-statement",
			AcceptRoute:   true,
			Nexthop:       dut1loopback20.attr.IPv4,
			NexthopType:   "ip",
		},
		{
			PolicyName:    exportNonDefV6,
			StatementName: "test-statement",
			AcceptRoute:   true,
			Nexthop:       dut1loopback20.attr.IPv6,
			NexthopType:   "ipv6",
		},
	}

	for _, rp := range routingPolicies {
		cfgplugins.RoutingPolicyBGPAIGP(t, dut1, rp)
	}

	// Configure ISIS on DUT1 for default and test instances with different interfaces and metrics
	configureISIS(t, dut1, deviations.DefaultNetworkInstance(dut1),
		[]string{dutTestData[0].lagName[1] + ".10", dutTestData[0].lagName[1] + ".30"}, isisNetDUT1Default, dut1loopback10.loopbackIntfName, isisDefaultInstance, 20)

	configureISIS(t, dut1, niTestInstance,
		[]string{dutTestData[0].lagName[1] + ".20", dutTestData[0].lagName[1] + ".40"}, isisNetDUT1TestInstance, dut1loopback20.loopbackIntfName, isisTestInstance, 20)

	// Configure ISIS on DUT2 for default and test instances with different interfaces and metrics
	configureISIS(t, dut2, deviations.DefaultNetworkInstance(dut2), []string{dutTestData[1].lagName[0] + ".10"}, isisNetDUT2Default, "", isisDefaultInstance, 20)
	configureISIS(t, dut2, niTestInstance, []string{dutTestData[1].lagName[0] + ".20"}, isisNetDUT2TestInstance, "", isisTestInstance, 20)
	configureISIS(t, dut2, niOriginate, []string{dutTestData[1].lagName[0] + ".30", dutTestData[1].lagName[0] + ".40"}, isisNetDUT2Originate, "", isisOriginateInstance, 20)

	// remove existing route policies on DUT1
	for _, peerPolicy := range []struct {
		ni           string
		isV4         bool
		nbrIp        string
		as           uint32
		importPolicy string
		exportPolicy string
		removePolicy bool
	}{
		{ni: niDefault, as: asnDUT1Default, nbrIp: "198.51.100.18", importPolicy: "", exportPolicy: exportPolicy, removePolicy: true, isV4: true},
		{ni: niDefault, as: asnDUT1Default, nbrIp: "2001:db8::36", importPolicy: "", exportPolicy: exportPolicyv6, removePolicy: true, isV4: false},
		{ni: niTestInstance, as: asnDUT1Default, nbrIp: "198.51.100.22", importPolicy: "", exportPolicy: exportPolicy, removePolicy: true, isV4: true},
		{ni: niTestInstance, as: asnDUT1Default, nbrIp: "2001:db8::22", importPolicy: "", exportPolicy: exportPolicyv6, removePolicy: true, isV4: false},
		{ni: niDefault, as: asnDUT1Default, nbrIp: "198.51.100.18", importPolicy: "", exportPolicy: exportDefaultV4, removePolicy: false, isV4: true},
		{ni: niDefault, as: asnDUT1Default, nbrIp: "2001:db8::36", importPolicy: "", exportPolicy: exportDefaultV6, removePolicy: false, isV4: false},
		{ni: niTestInstance, as: asnDUT1Default, nbrIp: "198.51.100.22", importPolicy: "", exportPolicy: exportNonDefV4, removePolicy: false, isV4: true},
		{ni: niTestInstance, as: asnDUT1Default, nbrIp: "2001:db8::22", importPolicy: "", exportPolicy: exportNonDefV6, removePolicy: false, isV4: false},
	} {
		routeMapPolicyParams := cfgplugins.NeighborRouteMapAttributes{
			NetworkInstance:      peerPolicy.ni,
			As:                   peerPolicy.as,
			NeighborIp:           peerPolicy.nbrIp,
			ImportRouteMapPolicy: peerPolicy.importPolicy,
			V4:                   peerPolicy.isV4,
			RemoveRouteMapPolicy: peerPolicy.removePolicy,
			ExportRouteMapPolicy: peerPolicy.exportPolicy,
		}
		cfgplugins.ApplyRoutePolicyToBGPPeer(t, dut1, routeMapPolicyParams)
	}

	dutLag2BGPNeighbors := map[string]*niBGPConfig{
		niDefault: {
			defaultAsn: asnDUT1Default,
			globalAsn:  asnDUT1Default,
			bgpNbrs: []*bgpNbr{
				// Default NI - downlink peers uses 64497 (DUT2-- DUT1 vlan30 peers)
				{peerGrpName: pgDownlink, nbrIp: "198.51.100.26", peerAs: asnDUT1Default, isV4: true, enableAIGP: true},
				{peerGrpName: pgDownlink, nbrIp: "2001:db8::26", peerAs: asnDUT1Default, isV4: false, enableAIGP: true},
			},
		},
		niTestInstance: {
			defaultAsn: asnDUT1Default,
			globalAsn:  asnDUT1TestInstance,
			bgpNbrs: []*bgpNbr{
				// Test instance NI - downlink peers uses 64498 (DUT2-- DUT1 vlan40 peers)
				{peerGrpName: pgDownlink, nbrIp: "198.51.100.30", peerAs: asnDUT1TestInstance, isV4: true, enableAIGP: true},
				{peerGrpName: pgDownlink, nbrIp: "2001:db8::2a", peerAs: asnDUT1TestInstance, isV4: false, enableAIGP: true},
			},
		},
	}

	configureBGP(t, dut1, dutLag2BGPNeighbors)

	// Create export policy with AIGP metric set and nexthop as loopback on DUT1
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	// --- Prefix-set: Loopback-prefix-v4 ---
	psV4 := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet("Loopback-prefix-v4")
	psV4.GetOrCreatePrefix(dut2loopback10.attr.IPv4+"/32", "exact")
	psV4.GetOrCreatePrefix(dut2loopback20.attr.IPv4+"/32", "exact")

	// --- Prefix-set: Loopback-prefix-v6 ---
	psV6 := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet("Loopback-prefix-v6")
	psV6.GetOrCreatePrefix(dut2loopback10.attr.IPv6+"/128", "exact")
	psV6.GetOrCreatePrefix(dut2loopback20.attr.IPv6+"/128", "exact")

	gnmi.Update(t, dut2, gnmi.OC().RoutingPolicy().Config(), rp)

	// --- Route-policy: test-export-policy ---
	routingDUT2Policies := []cfgplugins.AIGPRoutePolicyData{
		{
			PolicyName:    "test-export-policy",
			StatementName: "match-export-statement-v4",
			AcceptRoute:   true,
			AigpMetric:    200,
			PrefixSetName: "Loopback-prefix-v4",
			NexthopType:   "ip",
		},
		{
			PolicyName:    "test-export-policyv6",
			StatementName: "match-export-statement-v6",
			AcceptRoute:   true,
			AigpMetric:    200,
			PrefixSetName: "Loopback-prefix-v6",
			NexthopType:   "ipv6",
		},
	}

	for _, rp := range routingDUT2Policies {
		cfgplugins.RoutingPolicyBGPAIGP(t, dut2, rp)
	}

	dut2TestOriginatePeer := map[string]*niBGPConfig{
		niDefault: {
			defaultAsn: asnDUT2Default,
			globalAsn:  asnDUT2Default,
			peerGroups: []*peerGroupCfg{
				{name: "default-peer-group", peerAs: asnDUT2Default, localAs: asnDUT2Default},
				{name: "default-peer-group6", peerAs: asnDUT2Default, localAs: asnDUT2Default},
				{name: "test-instance-peer-group", peerAs: asnDUT1TestInstance, localAs: asnDUT2TestInstance},
				{name: "test-instance-peer-group6", peerAs: asnDUT1TestInstance, localAs: asnDUT2TestInstance},
			},
		},
		niOriginate: {
			defaultAsn: asnDUT2Default,
			globalAsn:  asnDUT2Originate,
			bgpNbrs: []*bgpNbr{
				// test-originate NI - default-peer-group (IPv4) towards DUT1 default NI (vlan30)
				{peerGrpName: "default-peer-group", nbrIp: "198.51.100.25", peerAs: asnDUT1Default, isV4: true, exportPolicy: []string{"test-export-policy"}, enableAIGP: true},

				// test-originate NI - default-peer-group6 (IPv6) towards DUT1 default NI (vlan30)
				{peerGrpName: "default-peer-group6", nbrIp: "2001:db8::25", peerAs: asnDUT1Default, isV4: false, exportPolicy: []string{"test-export-policyv6"}, enableAIGP: true},

				// test-originate NI - test-instance-peer-group (IPv4) towards DUT1 test-instance NI (vlan40)
				{peerGrpName: "test-instance-peer-group", nbrIp: "198.51.100.29", peerAs: asnDUT1TestInstance, isV4: true, exportPolicy: []string{"test-export-policy"}, enableAIGP: true},

				// test-originate NI - test-instance-peer-group6 (IPv6) towards DUT1 test-instance NI (vlan40)
				{peerGrpName: "test-instance-peer-group6", nbrIp: "2001:db8::29", peerAs: asnDUT1TestInstance, isV4: false, exportPolicy: []string{"test-export-policyv6"}, enableAIGP: true},
			},
		},
	}

	configureBGP(t, dut2, dut2TestOriginatePeer)

	root := &oc.Root{}
	tableConn := root.GetOrCreateNetworkInstance(niOriginate).GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.Types_ADDRESS_FAMILY_IPV6)
	if !deviations.SkipSettingDisableMetricPropagation(dut2) {
		tableConn.SetDisableMetricPropagation(false)
	}
	gnmi.BatchUpdate(b, gnmi.OC().NetworkInstance(niOriginate).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.Types_ADDRESS_FAMILY_IPV6).Config(), tableConn)

	tableConn1 := root.GetOrCreateNetworkInstance(niOriginate).GetOrCreateTableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.Types_ADDRESS_FAMILY_IPV4)
	gnmi.BatchUpdate(b, gnmi.OC().NetworkInstance(niOriginate).TableConnection(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_DIRECTLY_CONNECTED, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, oc.Types_ADDRESS_FAMILY_IPV4).Config(), tableConn1)
	b.Set(t, dut2)

	// --- ISIS adjacency checks DUT1 ---

	verifyISISTelemetry(t, dut1, deviations.DefaultNetworkInstance(dut1), isisDefaultInstance, []string{dutTestData[0].lagName[1] + ".10"})
	verifyISISTelemetry(t, dut1, deviations.DefaultNetworkInstance(dut1), isisDefaultInstance, []string{dutTestData[0].lagName[1] + ".30"})
	verifyISISTelemetry(t, dut1, niTestInstance, isisTestInstance, []string{dutTestData[0].lagName[1] + ".20"})
	verifyISISTelemetry(t, dut1, niTestInstance, isisTestInstance, []string{dutTestData[0].lagName[1] + ".40"})

	verifyISISTelemetry(t, dut2, deviations.DefaultNetworkInstance(dut2), isisDefaultInstance, []string{dutTestData[1].lagName[0] + ".10"})
	verifyISISTelemetry(t, dut2, niTestInstance, isisTestInstance, []string{dutTestData[1].lagName[0] + ".20"})
	verifyISISTelemetry(t, dut2, niOriginate, isisOriginateInstance, []string{dutTestData[1].lagName[0] + ".30"})
	verifyISISTelemetry(t, dut2, niOriginate, isisOriginateInstance, []string{dutTestData[1].lagName[0] + ".40"})

	t.Logf("Verify DUT BGP sessions are up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut1)
	cfgplugins.VerifyDUTBGPEstablished(t, dut2)

	t.Logf("Verify DUT vrf %s BGP sessions are up", niTestInstance)
	cfgplugins.VerifyDUTBGPEstablishedForVRF(t, dut1, niTestInstance)
	cfgplugins.VerifyDUTBGPEstablishedForVRF(t, dut2, niTestInstance)

	t.Logf("Verify DUT vrf %s BGP sessions are up", niOriginate)
	cfgplugins.VerifyDUTBGPEstablishedForVRF(t, dut2, niOriginate)

	// DUT1 vlan10 and vlan20 peers in default NI
	dut1NIPeers := []string{"198.51.100.18", "2001:db8::36", "198.51.100.26", "2001:db8::26"}
	for _, p := range dut1NIPeers {
		verifyAIGPEnabled(t, dut1, deviations.DefaultNetworkInstance(dut1), p, true, strings.Contains(p, "."))
	}

	// DUT1 vlan30 and vlan40 peers in test-instance NI
	dut1TIPeers := []string{"198.51.100.22", "2001:db8::22", "198.51.100.30", "2001:db8::2a"}
	for _, p := range dut1TIPeers {
		verifyAIGPEnabled(t, dut1, niTestInstance, p, true, strings.Contains(p, "."))
	}

	// verify routes have aigp metric set
	for _, pfx := range []struct {
		prefix        string
		isV4          bool
		nbrIp         string
		aigpMetric    uint64
		ni            string
		validateRoute bool
	}{
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.26", aigpMetric: aigp200, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::26", aigpMetric: aigp200, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.30", aigpMetric: aigp200, ni: niTestInstance, validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::2a", aigpMetric: aigp200, ni: niTestInstance, validateRoute: true},
	} {
		if pfx.ni != niTestInstance {
			verifyAIGPInRIB(t, dut1, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, "", pfx.validateRoute)
		} else {
			verifyAIGPInRIB(t, dut1, niTestInstance, pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, "", pfx.validateRoute)
		}
	}

	for _, peerGroup := range []string{"downlink", "downlink6"} {
		verifyRouteReflectorClientState(t, dut1, deviations.DefaultNetworkInstance(dut1), peerGroup, true)
	}

	dut2NIPeers := []string{"198.51.100.17", "2001:db8::35"}
	for _, p := range dut2NIPeers {
		verifyAIGPEnabled(t, dut2, deviations.DefaultNetworkInstance(dut2), p, true, strings.Contains(p, "."))
	}

	// DUT1 vlan30 and vlan40 peers in test-instance NI
	dut2TIPeers := []string{"198.51.100.21", "2001:db8::21"}
	for _, p := range dut2TIPeers {
		verifyAIGPEnabled(t, dut2, niTestInstance, p, true, strings.Contains(p, "."))
	}
	// verify routes have aigp metric set
	for _, pfx := range []struct {
		prefix        string
		isV4          bool
		nbrIp         string
		aigpMetric    uint64
		ni            string
		nexthop       string
		validateRoute bool
	}{
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.17", aigpMetric: aigp220, ni: deviations.DefaultNetworkInstance(dut2), nexthop: dut1loopback10.attr.IPv4, validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::35", aigpMetric: aigp220, ni: deviations.DefaultNetworkInstance(dut2), nexthop: dut1loopback10.attr.IPv6, validateRoute: true},
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.21", aigpMetric: aigp220, ni: niTestInstance, nexthop: dut1loopback20.attr.IPv4, validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::21", aigpMetric: aigp220, ni: niTestInstance, nexthop: dut1loopback20.attr.IPv6, validateRoute: true},
	} {
		if pfx.ni != niTestInstance {
			verifyAIGPInRIB(t, dut2, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, pfx.nexthop, pfx.validateRoute)
		} else {
			verifyAIGPInRIB(t, dut2, niTestInstance, pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, pfx.nexthop, pfx.validateRoute)
		}
	}

	for _, nbr := range []string{"198.51.100.25", "2001:db8::25", "198.51.100.29", "2001:db8::29"} {
		verifyBGPUpdatesSentNonZero(t, dut2, niOriginate, nbr)
	}
}

func testAIGPIncrementPlusOne(t *testing.T, dutTestData []*dutData, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	// Configure ISIS on DUT1 for default and test instances with 0 metrics
	updateISISMetric(t, dut1, deviations.DefaultNetworkInstance(dut1), isisDefaultInstance, []string{dutTestData[0].lagName[1] + ".10", dutTestData[0].lagName[1] + ".30"}, 1)
	updateISISMetric(t, dut1, niTestInstance, isisTestInstance, []string{dutTestData[0].lagName[1] + ".20", dutTestData[0].lagName[1] + ".40"}, 1)

	// Configure ISIS on DUT2 for default and test instances with different interfaces and metrics
	updateISISMetric(t, dut2, deviations.DefaultNetworkInstance(dut2), isisDefaultInstance, []string{dutTestData[1].lagName[0] + ".10"}, 1)
	updateISISMetric(t, dut2, niTestInstance, isisTestInstance, []string{dutTestData[1].lagName[0] + ".20"}, 1)
	updateISISMetric(t, dut2, niOriginate, isisOriginateInstance, []string{dutTestData[1].lagName[0] + ".30", dutTestData[1].lagName[0] + ".40"}, 1)

	verifyISISMetric(t, dut1, deviations.DefaultNetworkInstance(dut1), isisDefaultInstance, dutTestData[0].lagName[1]+".10", 2, oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST, 1)
	verifyISISMetric(t, dut1, deviations.DefaultNetworkInstance(dut1), isisDefaultInstance, dutTestData[0].lagName[1]+".30", 2, oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST, 1)
	verifyISISMetric(t, dut1, niTestInstance, isisTestInstance, dutTestData[0].lagName[1]+".20", 2, oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST, 1)
	verifyISISMetric(t, dut1, niTestInstance, isisTestInstance, dutTestData[0].lagName[1]+".40", 2, oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST, 1)

	verifyISISMetric(t, dut2, deviations.DefaultNetworkInstance(dut2), isisDefaultInstance, dutTestData[1].lagName[0]+".10", 2, oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST, 1)
	verifyISISMetric(t, dut2, niOriginate, isisOriginateInstance, dutTestData[1].lagName[0]+".30", 2, oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST, 1)

	// verify routes have aigp metric set
	for _, pfx := range []struct {
		prefix        string
		isV4          bool
		nbrIp         string
		aigpMetric    uint64
		ni            string
		validateRoute bool
	}{
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.26", aigpMetric: aigp200, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::26", aigpMetric: aigp200, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.30", aigpMetric: aigp200, ni: niTestInstance, validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::2a", aigpMetric: aigp200, ni: niTestInstance, validateRoute: true},
	} {
		if pfx.ni != niTestInstance {
			verifyAIGPInRIB(t, dut1, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, "", pfx.validateRoute)
		} else {
			verifyAIGPInRIB(t, dut1, niTestInstance, pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, "", pfx.validateRoute)
		}
	}

	// DUT1 vlan30 and vlan40 peers in test-instance NI
	dut2TIPeers := []string{"198.51.100.21", "2001:db8::21"}
	for _, p := range dut2TIPeers {
		verifyAIGPEnabled(t, dut2, niTestInstance, p, true, strings.Contains(p, "."))
	}
	// verify routes have aigp metric set
	for _, pfx := range []struct {
		prefix        string
		isV4          bool
		nbrIp         string
		aigpMetric    uint64
		ni            string
		nexthop       string
		validateRoute bool
	}{
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.17", aigpMetric: aigp201, ni: deviations.DefaultNetworkInstance(dut2), validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::35", aigpMetric: aigp201, ni: deviations.DefaultNetworkInstance(dut2), validateRoute: true},
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.21", aigpMetric: aigp201, ni: niTestInstance, validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::21", aigpMetric: aigp201, ni: niTestInstance, validateRoute: true},
	} {
		if pfx.ni != niTestInstance {
			verifyAIGPInRIB(t, dut2, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, pfx.nexthop, pfx.validateRoute)
		} else {
			verifyAIGPInRIB(t, dut2, niTestInstance, pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, pfx.nexthop, pfx.validateRoute)
		}
	}
}

func testAIGPDisable(t *testing.T, dutTestData []*dutData, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	enableAIGPOnPeer(t, dut1, deviations.DefaultNetworkInstance(dut1), asnDUT1Default, "198.51.100.18", true, false)
	enableAIGPOnPeer(t, dut1, deviations.DefaultNetworkInstance(dut1), asnDUT1Default, "2001:db8::18", false, false)
	enableAIGPOnPeer(t, dut1, niTestInstance, asnDUT1Default, "198.51.100.22", true, false)
	enableAIGPOnPeer(t, dut1, niTestInstance, asnDUT1Default, "2001:db8::22", false, false)

	t.Logf("Verify DUT BGP sessions are up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut1)
	cfgplugins.VerifyDUTBGPEstablished(t, dut2)

	t.Logf("Verify DUT vrf test-instance BGP sessions are up")
	cfgplugins.VerifyDUTBGPEstablishedForVRF(t, dut1, niTestInstance)
	cfgplugins.VerifyDUTBGPEstablishedForVRF(t, dut2, niTestInstance)

	dut1DIPeers := []string{"198.51.100.18", "2001:db8::36"}
	for _, p := range dut1DIPeers {
		verifyAIGPEnabled(t, dut1, deviations.DefaultNetworkInstance(dut1), p, false, strings.Contains(p, "."))
	}

	dut1NIPeers := []string{"198.51.100.22", "2001:db8::22"}
	for _, p := range dut1NIPeers {
		verifyAIGPEnabled(t, dut2, niTestInstance, p, false, strings.Contains(p, "."))
	}

	dut1DIPeers = []string{"198.51.100.26", "2001:db8::26"}
	for _, p := range dut1DIPeers {
		verifyAIGPEnabled(t, dut1, deviations.DefaultNetworkInstance(dut1), p, true, strings.Contains(p, "."))
	}

	dut1NIPeers = []string{"198.51.100.30", "2001:db8::2a"}
	for _, p := range dut1NIPeers {
		verifyAIGPEnabled(t, dut1, niTestInstance, p, true, strings.Contains(p, "."))
	}

	// verify routes have aigp metric set
	for _, pfx := range []struct {
		prefix        string
		isV4          bool
		nbrIp         string
		aigpMetric    uint64
		ni            string
		validateRoute bool
	}{
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.26", aigpMetric: aigp200, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::26", aigpMetric: aigp200, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.30", aigpMetric: aigp200, ni: niTestInstance, validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::2a", aigpMetric: aigp200, ni: niTestInstance, validateRoute: true},
	} {
		if pfx.ni != niTestInstance {
			verifyAIGPInRIB(t, dut1, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, "", pfx.validateRoute)
		} else {
			verifyAIGPInRIB(t, dut1, niTestInstance, pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, "", pfx.validateRoute)
		}
	}

	// verify routes have aigp metric set
	for _, pfx := range []struct {
		prefix        string
		isV4          bool
		nbrIp         string
		aigpMetric    uint64
		ni            string
		nexthop       string
		validateRoute bool
	}{
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.17", aigpMetric: aigp201, ni: deviations.DefaultNetworkInstance(dut2), validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::35", aigpMetric: aigp201, ni: deviations.DefaultNetworkInstance(dut2), validateRoute: true},
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.21", aigpMetric: aigp201, ni: niTestInstance, validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::21", aigpMetric: aigp201, ni: niTestInstance, validateRoute: true},
	} {
		if pfx.ni != niTestInstance {
			verifyAIGPInRIB(t, dut2, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, pfx.nexthop, pfx.validateRoute)
		} else {
			verifyAIGPInRIB(t, dut2, niTestInstance, pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, pfx.nexthop, pfx.validateRoute)
		}
	}
}

func testAIGPPropagationPeerGroup(t *testing.T, dutTestData []*dutData, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")

	enableAIGPOnPeer(t, dut1, deviations.DefaultNetworkInstance(dut1), asnDUT1Default, "198.51.100.18", true, true)
	enableAIGPOnPeer(t, dut1, deviations.DefaultNetworkInstance(dut1), asnDUT1Default, "2001:db8::18", false, true)
	enableAIGPOnPeer(t, dut1, niTestInstance, asnDUT1Default, "198.51.100.22", true, true)
	enableAIGPOnPeer(t, dut1, niTestInstance, asnDUT1Default, "2001:db8::22", false, true)

	t.Logf("Verify DUT BGP sessions are up")
	cfgplugins.VerifyDUTBGPEstablished(t, dut1)
	cfgplugins.VerifyDUTBGPEstablished(t, dut2)

	t.Logf("Verify DUT vrf test-instance BGP sessions are up")
	cfgplugins.VerifyDUTBGPEstablishedForVRF(t, dut1, niTestInstance)
	cfgplugins.VerifyDUTBGPEstablishedForVRF(t, dut2, niTestInstance)

	// verify routes have aigp metric set
	for _, pfx := range []struct {
		prefix        string
		isV4          bool
		nbrIp         string
		aigpMetric    uint64
		ni            string
		validateRoute bool
	}{
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.26", aigpMetric: aigp200, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::26", aigpMetric: aigp200, ni: deviations.DefaultNetworkInstance(dut1), validateRoute: true},
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.30", aigpMetric: aigp200, ni: niTestInstance, validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::2a", aigpMetric: aigp200, ni: niTestInstance, validateRoute: true},
	} {
		if pfx.ni != niTestInstance {
			verifyAIGPInRIB(t, dut1, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, "", pfx.validateRoute)
		} else {
			verifyAIGPInRIB(t, dut1, niTestInstance, pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, "", pfx.validateRoute)
		}
	}

	// verify routes have aigp metric set
	for _, pfx := range []struct {
		prefix        string
		isV4          bool
		nbrIp         string
		aigpMetric    uint64
		ni            string
		nexthop       string
		validateRoute bool
	}{
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.17", aigpMetric: aigp201, ni: deviations.DefaultNetworkInstance(dut2), validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::35", aigpMetric: aigp201, ni: deviations.DefaultNetworkInstance(dut2), validateRoute: true},
		{prefix: "198.60.1.1/32", isV4: true, nbrIp: "198.51.100.21", aigpMetric: aigp201, ni: niTestInstance, validateRoute: true},
		{prefix: "2001:db8:60::1/128", isV4: false, nbrIp: "2001:db8::21", aigpMetric: aigp201, ni: niTestInstance, validateRoute: true},
	} {
		if pfx.ni != niTestInstance {
			verifyAIGPInRIB(t, dut2, deviations.DefaultNetworkInstance(dut1), pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, pfx.nexthop, pfx.validateRoute)
		} else {
			verifyAIGPInRIB(t, dut2, niTestInstance, pfx.nbrIp, pfx.prefix, pfx.aigpMetric, pfx.isV4, pfx.nexthop, pfx.validateRoute)
		}
	}
}
