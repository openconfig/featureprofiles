package isis_scale_test

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	linkIpv4PLen   = 30
	linkIpv6PLen   = 126
	dutAreaAddress = "49.0001"
	dutSysID       = "1920.0000.2001"
	lagType        = oc.IfAggregate_AggregationType_LACP
)

type testData struct {
	name                     string
	dutData                  *dutData
	ateData                  *otgconfighelpers.ATEData
	correctLSPCount          int
	correctAggInterfaceCount int
	correctISISAdjCount      int
	correctIPRouteCount      map[oc.E_Types_ADDRESS_FAMILY]int
}

type dutData struct {
	isisData *cfgplugins.ISISGlobalParams
	lags     []*cfgplugins.DUTAggData
}

var (
	defaultNetworkInstance = ""
	otgMACFormat           = "02:55:%d0:%d0:%d0:0%d"
	lacpActive             = oc.Lacp_LacpActivityType_ACTIVE
	lacpFast               = oc.Lacp_LacpPeriodType_FAST
	lacpParams             = &cfgplugins.LACPParams{
		Activity: &lacpActive,
		Period:   &lacpFast,
	}

	dutAgg11 = attrs.Attributes{
		Desc:    "DUT to ATE LAG1",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::1",
		IPv4Len: linkIpv4PLen,
		IPv6Len: linkIpv6PLen,
	}

	dutAgg12 = attrs.Attributes{
		Desc:    "DUT to ATE LAG2",
		IPv4:    "192.0.2.9",
		IPv6:    "2002:db8::1",
		IPv4Len: linkIpv4PLen,
		IPv6Len: linkIpv6PLen,
	}

	dutAggSubInterface211 = &cfgplugins.DUTSubInterfaceData{
		VlanID:        100,
		IPv4Address:   net.ParseIP("192.0.2.13"),
		IPv6Address:   net.ParseIP("2003:db8::1"),
		IPv4PrefixLen: linkIpv4PLen,
		IPv6PrefixLen: linkIpv6PLen,
	}
	dutAggSubInterface221 = &cfgplugins.DUTSubInterfaceData{
		VlanID:        101,
		IPv4Address:   net.ParseIP("192.0.2.17"),
		IPv6Address:   net.ParseIP("2004:db8::1"),
		IPv4PrefixLen: linkIpv4PLen,
		IPv6PrefixLen: linkIpv6PLen,
	}
	dutAggSubInterface222 = &cfgplugins.DUTSubInterfaceData{
		VlanID:        102,
		IPv4Address:   net.ParseIP("192.0.2.21"),
		IPv6Address:   net.ParseIP("2005:db8::1"),
		IPv4PrefixLen: linkIpv4PLen,
		IPv6PrefixLen: linkIpv6PLen,
	}

	r11 = &otgconfighelpers.AteEmulatedRouterData{
		Name:                   "R11",
		DUTIPv4:                dutAgg11.IPv4,
		ATEIPv4:                nextIP(net.ParseIP(dutAgg11.IPv4)).String(),
		LinkIPv4PLen:           linkIpv4PLen,
		DUTIPv6:                dutAgg11.IPv6,
		ATEIPv6:                nextIP(net.ParseIP(dutAgg11.IPv6)).String(),
		LinkIPv6PLen:           linkIpv6PLen,
		EthMAC:                 "02:55:10:10:10:02",
		ISISAreaAddress:        "490001",
		ISISSysID:              "640000000003",
		ISISLSPRefreshInterval: 65218,
		ISISSPLifetime:         65533,
	}

	r12 = &otgconfighelpers.AteEmulatedRouterData{
		Name:                   "R12",
		DUTIPv4:                dutAgg12.IPv4,
		ATEIPv4:                nextIP(net.ParseIP(dutAgg12.IPv4)).String(),
		LinkIPv4PLen:           linkIpv4PLen,
		DUTIPv6:                dutAgg12.IPv6,
		ATEIPv6:                nextIP(net.ParseIP(dutAgg12.IPv6)).String(),
		LinkIPv6PLen:           linkIpv6PLen,
		EthMAC:                 "02:55:20:20:20:02",
		ISISAreaAddress:        "490001",
		ISISSysID:              "640000000004",
		ISISLSPRefreshInterval: 65218,
		ISISSPLifetime:         65533,
	}

	r21 = &otgconfighelpers.AteEmulatedRouterData{
		Name:                   "R21",
		DUTIPv4:                dutAggSubInterface211.IPv4Address.String(),
		ATEIPv4:                nextIP(dutAggSubInterface211.IPv4Address).String(),
		LinkIPv4PLen:           linkIpv4PLen,
		DUTIPv6:                dutAggSubInterface211.IPv6Address.String(),
		ATEIPv6:                nextIP(dutAggSubInterface211.IPv6Address).String(),
		LinkIPv6PLen:           linkIpv6PLen,
		EthMAC:                 "02:55:10:10:10:02",
		ISISAreaAddress:        "490001",
		ISISSysID:              "640000000003",
		VlanID:                 100,
		ISISLSPRefreshInterval: 65218,
		ISISSPLifetime:         65533,
	}
	r22 = &otgconfighelpers.AteEmulatedRouterData{
		Name:                   "R22",
		DUTIPv4:                dutAggSubInterface221.IPv4Address.String(),
		ATEIPv4:                nextIP(dutAggSubInterface221.IPv4Address).String(),
		LinkIPv4PLen:           linkIpv4PLen,
		DUTIPv6:                dutAggSubInterface221.IPv6Address.String(),
		ATEIPv6:                nextIP(dutAggSubInterface221.IPv6Address).String(),
		LinkIPv6PLen:           linkIpv6PLen,
		EthMAC:                 "02:55:20:20:20:02",
		ISISAreaAddress:        "490001",
		ISISSysID:              "640000000004",
		VlanID:                 101,
		ISISLSPRefreshInterval: 65218,
		ISISSPLifetime:         65533,
	}
	r23 = &otgconfighelpers.AteEmulatedRouterData{
		Name:                   "R23",
		DUTIPv4:                dutAggSubInterface222.IPv4Address.String(),
		ATEIPv4:                nextIP(dutAggSubInterface222.IPv4Address).String(),
		LinkIPv4PLen:           linkIpv4PLen,
		DUTIPv6:                dutAggSubInterface222.IPv6Address.String(),
		ATEIPv6:                nextIP(dutAggSubInterface222.IPv6Address).String(),
		LinkIPv6PLen:           linkIpv6PLen,
		EthMAC:                 "02:55:30:30:30:02",
		ISISAreaAddress:        "490001",
		ISISSysID:              "640000000005",
		VlanID:                 102,
		ISISLSPRefreshInterval: 40,
		ISISSPLifetime:         60,
	}
)

func nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] > 0 {
			break
		}
	}
	return next
}

func calculateRoutesCount(isisBlocks *otgconfighelpers.ISISOTGBlock, afi string) int {
	nodeCount := isisBlocks.Col * isisBlocks.Row
	linkCount := isisBlocks.LinkMultiplier * (((isisBlocks.Col - 1) * (isisBlocks.Row)) + ((isisBlocks.Col) * (isisBlocks.Row - 1)))
	attachedIpv4Count := isisBlocks.V4Pfx.Count
	attachedIpv6Count := isisBlocks.V6Pfx.Count

	// The number of routes in the block is calculated as below:
	// 1. The number of nodes in the block x IPv4 /32
	// 2. The number of links in the block x IPv4 /31
	// 3. The number of attached IPv4 addresses per node x nodes count
	// 4. The number of attached IPv6 addresses per node x node count.
	// Note that for this test Links has only IPv4 address and nodes only have IPv4 Loopbacks.
	switch afi {
	case "ipv4":
		return nodeCount + linkCount + (attachedIpv4Count * nodeCount)
	case "ipv6":
		return attachedIpv6Count * nodeCount
	default:
		return 0
	}
}

func createATEISISBlocks() map[string]*otgconfighelpers.ISISOTGBlock {
	isisBlocks := make(map[string]*otgconfighelpers.ISISOTGBlock)
	type descriptor struct {
		name           string
		dimension      []int
		linkMultiplier int
		blockCount     int
	}
	firstOctetOctet := 20
	blocksDiscribtors := []descriptor{
		{
			name:           "RoutersTypeA",
			dimension:      []int{20, 20},
			linkMultiplier: 2,
			blockCount:     4,
		},
		{
			name:           "RoutersTypeB",
			dimension:      []int{12, 12},
			linkMultiplier: 17,
			blockCount:     4,
		},
		{
			name:           "RoutersTypeC",
			dimension:      []int{16, 16},
			linkMultiplier: 17,
			blockCount:     4,
		},
		{
			name:           "Dynamic",
			dimension:      []int{12, 12},
			linkMultiplier: 4,
			blockCount:     1,
		},
	}

	for _, b := range blocksDiscribtors {
		for i := 1; i <= b.blockCount; i++ {
			block := otgconfighelpers.ISISOTGBlock{
				Name:            b.name + "_" + strconv.Itoa(i),
				Col:             b.dimension[0],
				Row:             b.dimension[1],
				ISISIDFirstOct:  strconv.Itoa(firstOctetOctet),
				LinkIP4FirstOct: firstOctetOctet + 1,
				V6Pfx:           otgconfighelpers.Pfx{FirstOctet: strconv.Itoa(firstOctetOctet + 2), PfxLen: 64, Count: 2},
				LinkMultiplier:  b.linkMultiplier,
			}
			if !strings.Contains(b.name, "RoutersTypeA") {
				block.V4Pfx = otgconfighelpers.Pfx{FirstOctet: strconv.Itoa(firstOctetOctet + 2), PfxLen: 26, Count: 4}
			}
			isisBlocks[block.Name] = &block
			firstOctetOctet += 3
		}
	}
	return isisBlocks
}

func createATEData(lagToErouterMap map[int][]*otgconfighelpers.AteEmulatedRouterData) *otgconfighelpers.ATEData {
	var ateData otgconfighelpers.ATEData
	for li, er := range lagToErouterMap {
		ln := li + 1
		ateData.Lags = append(ateData.Lags, &otgconfighelpers.ATELagData{
			Name: "lag" + strconv.Itoa(ln),
			Mac:  fmt.Sprintf(otgMACFormat, ln, ln, ln, 1),
			Ports: []otgconfighelpers.ATEPortData{{
				Name:           "port" + strconv.Itoa(ln),
				Mac:            fmt.Sprintf(otgMACFormat, ln, ln, ln, 3),
				OndatraPortIdx: li}},
			Erouters: er,
		})
	}
	return &ateData
}

func createDUTData(aggAttributes []attrs.Attributes) *dutData {
	dutData := &dutData{
		isisData: &cfgplugins.ISISGlobalParams{
			DUTArea:  "49.0001",
			DUTSysID: "1920.0000.2001",
		},
	}
	for i, l := range aggAttributes {
		dutData.lags = append(dutData.lags, &cfgplugins.DUTAggData{
			Attributes:      l,
			LacpParams:      lacpParams,
			AggType:         oc.IfAggregate_AggregationType_LACP,
			OndatraPortsIdx: []int{i},
		})
	}
	return dutData
}

func initializeStaticTestData(t *testing.T) *testData {
	t.Helper()
	isisOTGBlocks := createATEISISBlocks()
	r11.ISISBlocks = []*otgconfighelpers.ISISOTGBlock{
		isisOTGBlocks["RoutersTypeA_2"], isisOTGBlocks["RoutersTypeA_4"],
		isisOTGBlocks["RoutersTypeB_2"], isisOTGBlocks["RoutersTypeB_4"],
		isisOTGBlocks["RoutersTypeC_2"], isisOTGBlocks["RoutersTypeC_4"],
	}
	r12.ISISBlocks = []*otgconfighelpers.ISISOTGBlock{
		isisOTGBlocks["RoutersTypeA_1"], isisOTGBlocks["RoutersTypeA_3"],
		isisOTGBlocks["RoutersTypeB_1"], isisOTGBlocks["RoutersTypeB_3"],
		isisOTGBlocks["RoutersTypeC_1"], isisOTGBlocks["RoutersTypeC_3"],
	}
	lagToErouterMap := map[int][]*otgconfighelpers.AteEmulatedRouterData{
		0: {r11},
		1: {r12},
	}
	ateData := createATEData(lagToErouterMap)
	ateData.ConfigureISIS = true
	ateData.TrafficFlowsMap = map[*otgconfighelpers.AteEmulatedRouterData][]*otgconfighelpers.AteEmulatedRouterData{
		r11: {r12},
		r12: {r11},
	}

	// Count of LSP generated by RouterTypeA = 400
	// Count of LSP generated by RouterTypeB = 672
	// Count of LSP generated by RouterTypeC = 1216
	// Total Count of LSPs = 4*(400 + 672 + 1216)  = 9152
	testData := &testData{
		name:                     "StaticLSPs",
		dutData:                  createDUTData([]attrs.Attributes{dutAgg11, dutAgg12}),
		ateData:                  ateData,
		correctLSPCount:          9152,
		correctAggInterfaceCount: 2,
		correctISISAdjCount:      2,
		correctIPRouteCount:      map[oc.E_Types_ADDRESS_FAMILY]int{oc.Types_ADDRESS_FAMILY_IPV4: 66284, oc.Types_ADDRESS_FAMILY_IPV6: 6390},
	}

	ipv4PrefixTotal := 0
	ipv6PrefixTotal := 0
	for _, l := range ateData.Lags {
		for _, r := range l.Erouters {
			for _, b := range r.ISISBlocks {
				ipv4PrefixTotal += calculateRoutesCount(b, "ipv4")
				ipv6PrefixTotal += calculateRoutesCount(b, "ipv6")
			}
		}
	}
	t.Logf("The IXIA toplogy should create %v IPv4 routes and %v IPv6 routes \ntest will fail if the IPv4 routes was less than %v or IPv6 routes was less than %v", ipv4PrefixTotal, ipv6PrefixTotal, testData.correctIPRouteCount[oc.Types_ADDRESS_FAMILY_IPV4], testData.correctIPRouteCount[oc.Types_ADDRESS_FAMILY_IPV6])

	return testData
}

func initializeDynamicTestData(t *testing.T) *testData {
	t.Helper()
	isisOTGBlocks := createATEISISBlocks()

	r21.ISISBlocks = []*otgconfighelpers.ISISOTGBlock{
		isisOTGBlocks["RoutersTypeA_2"], isisOTGBlocks["RoutersTypeA_4"],
		isisOTGBlocks["RoutersTypeB_2"], isisOTGBlocks["RoutersTypeB_4"],
		isisOTGBlocks["RoutersTypeC_2"], isisOTGBlocks["RoutersTypeC_4"],
	}
	r22.ISISBlocks = []*otgconfighelpers.ISISOTGBlock{
		isisOTGBlocks["RoutersTypeA_1"], isisOTGBlocks["RoutersTypeA_3"],
		isisOTGBlocks["RoutersTypeB_1"], isisOTGBlocks["RoutersTypeB_3"],
		isisOTGBlocks["RoutersTypeC_1"], isisOTGBlocks["RoutersTypeC_3"],
	}

	r23.ISISBlocks = []*otgconfighelpers.ISISOTGBlock{
		isisOTGBlocks["Dynamic_1"],
	}

	lagToErouterMap := map[int][]*otgconfighelpers.AteEmulatedRouterData{
		0: {r21},
		1: {r22, r23},
	}
	ateData := createATEData(lagToErouterMap)
	ateData.ConfigureISIS = true
	ateData.TrafficFlowsMap = map[*otgconfighelpers.AteEmulatedRouterData][]*otgconfighelpers.AteEmulatedRouterData{
		r21: {r22, r23},
		r22: {r21},
		r23: {r21},
	}
	dutAgg11.IPv4 = ""
	dutAgg11.IPv6 = ""
	dutAgg12.IPv4 = ""
	dutAgg12.IPv6 = ""

	dutData := createDUTData([]attrs.Attributes{dutAgg11, dutAgg11})
	dutData.lags[0].SubInterfaces = []*cfgplugins.DUTSubInterfaceData{dutAggSubInterface211}
	dutData.lags[1].SubInterfaces = []*cfgplugins.DUTSubInterfaceData{dutAggSubInterface221, dutAggSubInterface222}

	// Count of LSP geenrated by RouterTypeA = 400
	// Count of LSP generated by RouterTypeB = 672
	// Count of LSP generated by RouterTypeC = 1216
	// Count of LSP generated by Dynamic = 244
	// Total Count of LSPs = 4*(400 + 672 + 1216) + 244 = 9396

	testData := &testData{
		name:                     "DynamicLSPs",
		dutData:                  dutData,
		ateData:                  ateData,
		correctLSPCount:          9396,
		correctAggInterfaceCount: 2,
		correctISISAdjCount:      3,
		correctIPRouteCount:      map[oc.E_Types_ADDRESS_FAMILY]int{oc.Types_ADDRESS_FAMILY_IPV4: 68048, oc.Types_ADDRESS_FAMILY_IPV6: 6680},
	}

	ipv4PrefixTotal := 0
	ipv6PrefixTotal := 0
	for _, l := range ateData.Lags {
		for _, r := range l.Erouters {
			for _, b := range r.ISISBlocks {
				ipv4PrefixTotal += calculateRoutesCount(b, "ipv4")
				ipv6PrefixTotal += calculateRoutesCount(b, "ipv6")
			}
		}
	}
	t.Logf("The IXIA toplogy should create %v IPv4 routes and %v IPv6 routes \ntest will fail if the IPv4 routes was less than %v or IPv6 routes was less than %v", ipv4PrefixTotal, ipv6PrefixTotal, testData.correctIPRouteCount[oc.Types_ADDRESS_FAMILY_IPV4], testData.correctIPRouteCount[oc.Types_ADDRESS_FAMILY_IPV6])

	return testData
}

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureEnableAFTSummaries)
	if hardwareInitCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, dutData *dutData) {
	t.Helper()
	configureHardwareInit(t, dut)
	t.Logf("===========Configuring DUT===========")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	for _, l := range dutData.lags {
		b := &gnmi.SetBatch{}
		// Create LAG interface
		l.LagName = netutil.NextAggregateInterface(t, dut)
		agg := cfgplugins.NewAggregateInterface(t, dut, b, l)
		b.Set(t, dut)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			for k := range agg.GetOrCreateSubinterfaceMap() {
				fptest.AssignToNetworkInstance(t, dut, l.LagName, defaultNetworkInstance, k)
			}
		}
	}
	// Wait for LAG interfaces to be AdminStatus UP
	for _, l := range dutData.lags {
		gnmi.Await(t, dut, gnmi.OC().Interface(l.LagName).AdminStatus().State(), 30*time.Second, oc.Interface_AdminStatus_UP)
	}
	dutData.isisData.ISISInterfaceNames = createISISInterfaceNames(t, dut, dutData)
	b := &gnmi.SetBatch{}
	cfgplugins.NewISIS(t, dut, dutData.isisData, b)
	b.Set(t, dut)

}

func createISISInterfaceNames(t *testing.T, dut *ondatra.DUTDevice, dt *dutData) []string {
	t.Helper()
	loopback0 := netutil.LoopbackInterface(t, dut, 0)
	interfaceNames := []string{loopback0}
	for _, l := range dt.lags {
		if l.Attributes.IPv4 != "" {
			interfaceNames = append(interfaceNames, l.LagName)
		} else {
			for _, s := range l.SubInterfaces {
				interfaceNames = append(interfaceNames, fmt.Sprintf("%s.%d", l.LagName, s.VlanID))
			}
		}
	}
	return interfaceNames
}

func findISISInterfaces(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	allISISInterfacesPath := gnmi.OC().NetworkInstance(defaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, defaultNetworkInstance).Isis().InterfaceAny().State()
	allISISIntf := gnmi.LookupAll(t, dut, allISISInterfacesPath)
	var isisInterfaces []string
	for _, v := range allISISIntf {
		isisIntf, ok := v.Val()
		if !ok {
			t.Fatalf("could not get isis interface on dut: %v", dut.Name())
		}
		if strings.ToLower(isisIntf.GetInterfaceId()[0:2]) == "lo" {
			continue
		}
		isisInterfaces = append(isisInterfaces, isisIntf.GetInterfaceId())
	}
	return isisInterfaces
}

func findAggregatesFromInterfaces(t *testing.T, dut *ondatra.DUTDevice) []string {
	t.Helper()
	var aggsInISIS []string
	var aggName string
	for _, isisIntf := range findISISInterfaces(t, dut) {
		if strings.Contains(isisIntf, ".") {
			aggName = strings.Split(isisIntf, ".")[0]
		} else {
			aggName = isisIntf
		}
		if !slices.Contains(aggsInISIS, aggName) {
			aggsInISIS = append(aggsInISIS, aggName)
		}
	}
	return aggsInISIS
}

func checkIntsOpState(t *testing.T, dut *ondatra.DUTDevice, waitTime time.Duration) (int, bool) {
	t.Helper()
	batch := gnmi.OCBatch()
	isisAggs := findAggregatesFromInterfaces(t, dut)
	for _, a := range isisAggs {
		batch.AddPaths(gnmi.OC().Interface(a))
	}
	intUpCount := 0
	watch := gnmi.Watch(t, dut, batch.State(), waitTime, func(val *ygnmi.Value[*oc.Root]) bool {
		root, _ := val.Val()
		intUpCount = 0
		for _, a := range isisAggs {
			if root.GetInterface(a).GetOperStatus() == oc.Interface_OperStatus_UP {
				intUpCount++
			}
		}
		return intUpCount == len(isisAggs)
	})
	_, ok := watch.Await(t)

	return intUpCount, ok
}

func findISISAdjCount(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration, nominalCount int) (int, bool) {
	t.Helper()
	var isisAdjIDs []string
	isisIntPath := gnmi.OC().NetworkInstance(defaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, defaultNetworkInstance).Isis().InterfaceAny().State()
	watchAll := gnmi.WatchAll(t, dut, isisIntPath, timeout, func(val *ygnmi.Value[*oc.NetworkInstance_Protocol_Isis_Interface]) bool {
		s, ok := val.Val()
		if !ok {
			t.Logf("Could not get ISIS interface on DUT: %v", dut.Name())
			return false
		}
		adjMap := s.GetLevel(2).Adjacency
		for nei, adj := range adjMap {
			if adj.GetAdjacencyState() == oc.Isis_IsisInterfaceAdjState_UP {
				if !slices.Contains(isisAdjIDs, nei) {
					isisAdjIDs = append(isisAdjIDs, nei)
				}
			}
		}
		return len(isisAdjIDs) >= nominalCount
	})
	_, ok := watchAll.Await(t)
	return len(isisAdjIDs), ok
}

func findISISAdjCountNonStream(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration, nominalCount int) (int, bool) {
	t.Helper()
	const trialCount = 10
	var isisAdjIDs []string
	isisIntPath := gnmi.OC().NetworkInstance(defaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, defaultNetworkInstance).Isis().InterfaceAny().State()
	for i := 0; i < trialCount; i++ {
		isisIntfsMap := gnmi.LookupAll(t, dut, isisIntPath)
		for _, v := range isisIntfsMap {
			isisIntf, ok := v.Val()
			if !ok {
				t.Logf("Could not get ISIS interface on DUT: %v", dut.Name())
				continue
			}
			adjMap := isisIntf.GetLevel(2).GetOrCreateAdjacencyMap()
			for nei, adj := range adjMap {
				if adj.GetAdjacencyState() == oc.Isis_IsisInterfaceAdjState_UP {
					if !slices.Contains(isisAdjIDs, nei) {
						isisAdjIDs = append(isisAdjIDs, nei)
					}
				}
			}
		}
		if len(isisAdjIDs) >= nominalCount {
			break
		}
		time.Sleep(timeout / trialCount)
	}

	return len(isisAdjIDs), true
}

func checkTraffic(t *testing.T, ate *ondatra.ATEDevice, trafficFlows []gosnappi.Flow) []error {
	t.Helper()
	var errs []error
	for _, f := range trafficFlows {
		outBytes := float32(gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(f.Name()).Counters().OutOctets().State()))
		inBytes := float32(gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(f.Name()).Counters().InOctets().State()))
		fmt.Printf("Flow: %s, OutBytes: %v, InBytes: %v\n", f.Name(), outBytes, inBytes)
		rate := f.Rate().Kbps() * 1000 / 8
		if int(outBytes) == 0 {
			t.Errorf("outbytes for flow %s is 0, want > 0", f.Name())
		} else if got := ((outBytes - inBytes) * 100) / outBytes; got > 0 {
			downTime := (outBytes - inBytes) / float32(rate)
			errs = append(errs, fmt.Errorf("\ncheck failed: losspct and downtime for flow %s:\n	got %v percent loss and %v seconds downtime, want 0 percent loss and 0 seconds downtime", f.Name(), got, downTime))
		}
	}
	return errs
}

func findISISActiveLSPCount(t *testing.T, dut *ondatra.DUTDevice, waitTime time.Duration, nominalCount int) (int, bool) {
	t.Helper()
	lspCount := 0
	anyLspLifeTimePath := gnmi.OC().NetworkInstance(defaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, defaultNetworkInstance).Isis().Level(2).LspAny().RemainingLifetime().State()
	watch := gnmi.WatchAll(t, dut, anyLspLifeTimePath, waitTime, func(val *ygnmi.Value[uint16]) bool {
		lspLifeTime, ok := val.Val()
		if !ok {
			t.Logf("Could not get LSP lifetime on DUT: %v", dut.Name())
			return false
		}
		if lspLifeTime > 1 {
			lspCount++
		}
		return lspCount >= nominalCount
	})
	_, ok := watch.Await(t)
	return lspCount, ok
}

func findISISLSPCount(t *testing.T, dut *ondatra.DUTDevice, waitTime time.Duration, nominalCount int) (int, bool) {
	t.Helper()
	lspPath := gnmi.OC().NetworkInstance(defaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, defaultNetworkInstance).Isis().Level(2).SystemLevelCounters().TotalLsps().State()
	watch := gnmi.Watch(t, dut, lspPath, waitTime, func(val *ygnmi.Value[uint32]) bool {
		s, ok := val.Val()
		if !ok {
			t.Logf("Could not get LSP count on DUT: %v", dut.Name())
			return false
		}
		return int(s) >= nominalCount
	})

	lspCount, ok := watch.Await(t)
	lspCountint, _ := lspCount.Val()

	return int(lspCountint), ok

}

func findProtocolSummaryRouteCount(t *testing.T, dut *ondatra.DUTDevice, afi oc.E_Types_ADDRESS_FAMILY, protocol oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE, waitTime time.Duration, nominalCount int) int {
	t.Helper()
	var routeCount int
	switch afi {
	case oc.Types_ADDRESS_FAMILY_IPV4:
		gnmiCall := gnmi.Watch(t, dut, gnmi.OC().NetworkInstance(defaultNetworkInstance).Afts().AftSummaries().Ipv4Unicast().Protocol(protocol).Counters().State(), waitTime, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_AftSummaries_Ipv4Unicast_Protocol_Counters]) bool {
			s, ok := val.Val()
			if !ok {
				t.Logf("Could not get IPv4 route count on DUT: %v", dut.Name())
				return false
			}
			routeCount = int(s.GetAftEntries())
			return routeCount >= nominalCount
		})
		gnmiCall.Await(t)
	case oc.Types_ADDRESS_FAMILY_IPV6:
		gnmiCall := gnmi.Watch(t, dut, gnmi.OC().NetworkInstance(defaultNetworkInstance).Afts().AftSummaries().Ipv6Unicast().Protocol(protocol).Counters().State(), waitTime, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_AftSummaries_Ipv6Unicast_Protocol_Counters]) bool {
			s, ok := val.Val()
			if !ok {
				t.Logf("Could not get IPv6 route count on DUT: %v", dut.Name())
				return false
			}
			routeCount = int(s.GetAftEntries())
			return routeCount >= nominalCount
		})
		gnmiCall.Await(t)

	default:
		t.Logf("Unsupported AFI: %v", afi)
	}

	return int(routeCount)
}

func findProtocolRouteCount(t *testing.T, dut *ondatra.DUTDevice, afi oc.E_Types_ADDRESS_FAMILY, protocol oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE, waitTime time.Duration, nominalCount int) (int, bool) {
	t.Helper()
	var routeCount int
	switch afi {
	case oc.Types_ADDRESS_FAMILY_IPV4:
		ipv4RoutePath := gnmi.OC().NetworkInstance(defaultNetworkInstance).Afts().Ipv4EntryAny().State()
		watch := gnmi.WatchAll(t, dut, ipv4RoutePath, waitTime, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			s, ok := val.Val()
			if !ok {
				t.Logf("Could not get IPv4 route on DUT: %v", dut.Name())
				return false
			}
			if s.GetOriginProtocol() == protocol {
				routeCount++
			}
			return routeCount >= nominalCount
		})
		_, ok := watch.Await(t)
		return routeCount, ok
	case oc.Types_ADDRESS_FAMILY_IPV6:
		ipv6RoutePath := gnmi.OC().NetworkInstance(defaultNetworkInstance).Afts().Ipv6EntryAny().State()
		watch := gnmi.WatchAll(t, dut, ipv6RoutePath, waitTime, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			s, ok := val.Val()
			if !ok {
				t.Logf("Could not get IPv6 route on DUT: %v", dut.Name())
				return false
			}
			if s.GetOriginProtocol() == protocol {
				routeCount++
			}
			return routeCount >= nominalCount
		})
		_, ok := watch.Await(t)
		return routeCount, ok
	default:
		t.Logf("Unsupported AFI: %v", afi)
		return 0, false
	}
}

func clearTestingConfig(t *testing.T, dut *ondatra.DUTDevice, defaultNetworkInstance string) {
	t.Helper()
	t.Logf("===========Clearing Dut config===========")
	isisIntf := findISISInterfaces(t, dut)
	aggNames := findAggregatesFromInterfaces(t, dut)

	b := &gnmi.SetBatch{}
	gnmi.BatchDelete(b, gnmi.OC().NetworkInstance(defaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, defaultNetworkInstance).Config())

	// Remove LACP for vendors with atomic update deviation.
	if deviations.AggregateAtomicUpdate(dut) {
		for _, agg := range aggNames {
			gnmi.BatchDelete(b, gnmi.OC().Lacp().Interface(agg).Config())
		}
	}
	// Remove aggregate id for all ondatra ports.
	for _, p := range dut.Ports() {
		gnmi.BatchDelete(b, gnmi.OC().Interface(p.Name()).Ethernet().AggregateId().Config())
	}
	// Remove all interfaces from default network instance.
	for _, in := range append(isisIntf, aggNames...) {
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			gnmi.BatchDelete(b, gnmi.OC().NetworkInstance(defaultNetworkInstance).Interface(in).Config())
		}
	}
	// Remove all interfaces from default network instance.
	for _, in := range aggNames {
		gnmi.BatchDelete(b, gnmi.OC().Interface(in).Config())
	}

	b.Set(t, dut)

	// Remove LACP for vendors without atomic update deviation.
	if !deviations.AggregateAtomicUpdate(dut) {
		b = &gnmi.SetBatch{}
		for _, agg := range aggNames {
			gnmi.BatchDelete(b, gnmi.OC().Lacp().Interface(agg).Config())
		}
		b.Set(t, dut)
	}
}

func setupTest(t *testing.T, testInfo *testData) *ondatra.DUTDevice {
	t.Helper()
	t.Logf("===========Configuring ATE===========")
	testInfo.ateData.ATE = ondatra.ATE(t, "ate")
	top := otgconfighelpers.ConfigureATE(t, testInfo.ateData.ATE, testInfo.ateData)
	testInfo.ateData.ATE.OTG().PushConfig(t, top)
	// testInfo.ateData.ATE.
	testInfo.ateData.AppendTrafficFlows(t, top)
	// Start protocols on ATE
	testInfo.ateData.ATE.OTG().StartProtocols(t)
	t.Cleanup(func() { testInfo.ateData.ATE.OTG().StopProtocols(t) })
	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	defaultNetworkInstance = deviations.DefaultNetworkInstance(dut)
	testInfo.dutData.isisData.NetworkInstanceName = defaultNetworkInstance
	configureDUT(t, dut, testInfo.dutData)
	t.Cleanup(func() { clearTestingConfig(t, dut, testInfo.dutData.isisData.NetworkInstanceName) })
	return dut
}

func TestISISScale(t *testing.T) {
	for _, f := range []func(*testing.T) *testData{
		initializeStaticTestData,
		initializeDynamicTestData,
	} {
		testInfo := f(t)
		t.Run(testInfo.name, func(t *testing.T) {
			dut := setupTest(t, testInfo)
			var count int
			var ok bool
			t.Logf("===========Conducting pre-test checks===========")
			// Check Aggregate on DUT are UP
			count, ok = checkIntsOpState(t, dut, 1*time.Minute)
			switch {
			case ok && count == testInfo.correctAggInterfaceCount:
				t.Logf("Check passed: All interfaces participating in ISIS are operationally up  need %v up interfaces got %v", testInfo.correctAggInterfaceCount, count)
			default:
				t.Fatalf("check failed: not all interfaces participating in ISIS are operationally up  need %v up interfaces got %v", testInfo.correctAggInterfaceCount, count)
			}

			// Check ISIS Adjacency
			if deviations.ISISAdjacencyStreamUnsupported(dut) {
				count, ok = findISISAdjCountNonStream(t, dut, 2*time.Minute, testInfo.correctISISAdjCount)
			} else {
				count, ok = findISISAdjCount(t, dut, 2*time.Minute, testInfo.correctISISAdjCount)
			}
			switch {
			case !ok:
				t.Fatalf("check failed: not all isis adjacencies are up need %v up adjacencies got %v", testInfo.correctISISAdjCount, count)
			case count == testInfo.correctISISAdjCount:
				t.Logf("Check passed: All ISIS adjacencies are up  need %v up adjacencies got %v", testInfo.correctISISAdjCount, count)
			case count > testInfo.correctISISAdjCount:
				t.Errorf("ISIS adjacencies are more than expected  need %v up adjacencies got %v", testInfo.correctISISAdjCount, count)
			default:
				t.Fatalf("check failed: not all ISIS adjacencies are up : need %v up adjacencies got %v", testInfo.correctISISAdjCount, count)
			}

			t.Logf("===========Sleep for 5 minutes to check DUT stabilty===========")
			// Test will not check any metrics for 5 minutes to make sure DUT is stable.
			time.Sleep(5 * 60 * time.Second)
			t.Run("LSP_Count", func(t *testing.T) {
				// Check LSP Count
				if deviations.ISISLSPTlvsOCUnsupported(dut) {
					count, ok = findISISLSPCount(t, dut, 3*time.Minute, testInfo.correctLSPCount)
				} else {
					count, ok = findISISActiveLSPCount(t, dut, 3*time.Minute, testInfo.correctLSPCount)
				}
				if ok {
					t.Logf("Check passed: correct ISIS LSP count need %v lsps got %v", testInfo.correctLSPCount, count)
				} else {
					t.Errorf("check failed: incorrect isis lsp count need %v lsps got %v", testInfo.correctLSPCount, count)
				}
			})

			t.Run("Route_Count", func(t *testing.T) {
				var wg sync.WaitGroup
				for _, f := range []oc.E_Types_ADDRESS_FAMILY{oc.Types_ADDRESS_FAMILY_IPV4, oc.Types_ADDRESS_FAMILY_IPV6} {
					wg.Add(1)
					family := f
					go func() {
						defer wg.Done()
						if deviations.AFTSummaryOCUnsupported(dut) {
							count, ok := findProtocolRouteCount(t, dut, family, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, 1*time.Minute, testInfo.correctIPRouteCount[family])
							if !ok {
								t.Errorf("check failed: incorrect %s route count need %v routes got %v", family.String(), testInfo.correctIPRouteCount[family], count)
								return
							}
							t.Logf("Check passed: correct %s route count need %v routes got %v", family.String(), testInfo.correctIPRouteCount[family], count)
						} else {
							count := findProtocolSummaryRouteCount(t, dut, family, oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, 1*time.Minute, testInfo.correctIPRouteCount[family])
							if count >= testInfo.correctIPRouteCount[family] {
								t.Logf("Check passed: correct route count for the family %s need %v routes got %v", family.String(), testInfo.correctIPRouteCount[family], count)
							} else {
								t.Errorf("check failed: incorrect route count for the family %s need %v routes got %v", family.String(), testInfo.correctIPRouteCount[family], count)
							}
						}
					}()
				}
				wg.Wait()
			})

			t.Run("Traffic_Loss", func(t *testing.T) {
				// Start and stop traffic
				testInfo.ateData.ATE.OTG().StartTraffic(t)
				time.Sleep(30 * time.Second)
				testInfo.ateData.ATE.OTG().StopTraffic(t)
				// Check Traffic Loss
				errs := checkTraffic(t, testInfo.ateData.ATE, testInfo.ateData.TrafficFlows)
				if len(errs) > 0 {
					for _, err := range errs {
						t.Errorf("%v", err.Error())
					}
				} else {
					t.Logf("Check passed: no traffic loss found for the flows")
				}
			})
		})
	}
}
