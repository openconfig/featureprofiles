// Package isisscale provides helper functions for the ISIS scale tests.
package isisscale

import (
	"fmt"
	"maps"
	"net"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/iputil"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	linkIpv4PLen   = 30
	linkIpv6PLen   = 126
	dutAreaAddress = "49.0001"
	dutSysID       = "1920.0000.2001"
	lagType        = oc.IfAggregate_AggregationType_LACP
)

// TestData contains the test data for the ISIS scale test.
type TestData struct {
	Name                     string
	DUTData                  *DutData
	ATEData                  *otgconfighelpers.ATEData
	CorrectLSPCount          int
	CorrectAggInterfaceCount int
	CorrectISISAdjCount      int
	CorrectIPRouteCount      map[oc.E_Types_ADDRESS_FAMILY]int
}

// DutData contains the DUT data for the ISIS scale test.
type DutData struct {
	IsisData *cfgplugins.ISISGlobalParams
	Lags     []*cfgplugins.DUTAggData
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
)

// CalculateRoutesCount calculates the number of routes in the block.
func CalculateRoutesCount(isisBlocks *otgconfighelpers.ISISOTGBlock, afi string) int {
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

// CreateATEData creates the ATE data for the ISIS scale test.
func CreateATEData(lagToErouterMap map[int][]*otgconfighelpers.AteEmulatedRouterData) *otgconfighelpers.ATEData {
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

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureEnableAFTSummaries)
	if hardwareInitCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, dutData *DutData) {
	t.Helper()
	configureHardwareInit(t, dut)
	t.Logf("===========Configuring DUT===========")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	for _, l := range dutData.Lags {
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
	for _, l := range dutData.Lags {
		gnmi.Await(t, dut, gnmi.OC().Interface(l.LagName).AdminStatus().State(), 60*time.Second, oc.Interface_AdminStatus_UP)
	}
	dutData.IsisData.ISISInterfaceNames = createISISInterfaceNames(t, dut, dutData)
	b := &gnmi.SetBatch{}
	cfgplugins.NewISIS(t, dut, dutData.IsisData, b)
	b.Set(t, dut)

}

func createISISInterfaceNames(t *testing.T, dut *ondatra.DUTDevice, dt *DutData) []string {
	t.Helper()
	loopback0 := netutil.LoopbackInterface(t, dut, 0)
	interfaceNames := []string{loopback0}
	for _, l := range dt.Lags {
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
		if strings.HasPrefix(strings.ToLower(isisIntf.GetInterfaceId()), "lo") {
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

// CheckIntsOpState checks the operational state of the interfaces participating in ISIS.
// It returns the number of interfaces that are operationally up and a boolean indicating
// whether the check was successful.
func CheckIntsOpState(t *testing.T, dut *ondatra.DUTDevice, waitTime time.Duration) (int, bool) {
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

// FindISISAdjCount finds the number of ISIS adjacencies that are up. It does this by
// watching the ISIS interface state periodically in a STREAM telemetry call  and counting the number of adjacencies that are
// up. It returns the number of adjacencies that are up and a boolean indicating whether the count
// reached the nominal count or not.
func FindISISAdjCount(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration, nominalCount int) (int, bool) {
	t.Helper()
	var isisAdjIDs []string
	isisIntPath := gnmi.OC().NetworkInstance(defaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, defaultNetworkInstance).Isis().InterfaceAny().State()
	watchAll := gnmi.WatchAll(t, dut, isisIntPath, timeout, func(val *ygnmi.Value[*oc.NetworkInstance_Protocol_Isis_Interface]) bool {
		s, ok := val.Val()
		if !ok {
			t.Logf("Could not get ISIS interface on DUT: %v", dut.Name())
			return false
		}
		if s.GetLevel(2) == nil {
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

// FindISISAdjCountNonStream finds the number of ISIS adjacencies that are up. It does this by
// polling the ISIS interface state periodically in a ONCE telemetry call and counting the number of adjacencies that are
// up. It returns the number of adjacencies that are up and a boolean indicating whether the count
// reached the nominal count or not.
func FindISISAdjCountNonStream(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration, nominalCount int) (int, bool) {
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

	return len(isisAdjIDs), len(isisAdjIDs) >= nominalCount
}

// CheckTraffic checks the traffic loss for the given traffic flows.
// It checks the outOctets for each flow and makes sure it is not zero.
// It also checks the loss percent for each flow and makes sure it is 0.
func CheckTraffic(t *testing.T, ate *ondatra.ATEDevice, trafficFlows []gosnappi.Flow) []error {
	t.Helper()
	var errs []error
	nonCheckedFlows := make(map[string]bool, len(trafficFlows))
	for _, f := range trafficFlows {
		nonCheckedFlows[f.Name()] = false
	}
	allFlows := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().FlowAny().State())
	for _, c := range allFlows {
		flowName := c.GetName()
		delete(nonCheckedFlows, flowName)
		inOctets := c.GetCounters().GetInOctets()
		outOctets := c.GetCounters().GetOutOctets()
		fmt.Printf("Flow: %s, OutOctets: %v, InOctets: %v\n", flowName, outOctets, inOctets)
		if c.GetCounters().GetOutOctets() == 0 {
			errs = append(errs, fmt.Errorf("outbytes for flow %s is 0, want > 0", flowName))
		} else if lossPercent := ((outOctets - inOctets) * 100) / outOctets; lossPercent > 0 {
			errs = append(errs, fmt.Errorf("\ncheck failed: losspct for flow %s:\n	got %v percent loss , want 0 percent loss", flowName, lossPercent))
		}
	}
	if len(nonCheckedFlows) > 0 {
		t.Logf("Flows not found in OTG: %v", maps.Keys(nonCheckedFlows))
	}
	return errs
}

// FindISISActiveLSPCount finds the number of ISIS active LSPs that are up. It does this by
// watching the ISIS LSP lifetime periodically in a STREAM telemetry call and counting the number of LSPs that are
// up that has a lifetime greater than 1 second. It returns the number of active LSPs and a boolean
// indicating whether the count reached the nominal count or not.
func FindISISActiveLSPCount(t *testing.T, dut *ondatra.DUTDevice, waitTime time.Duration, nominalCount int) (int, bool) {
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

// FindISISLSPCount finds the number of ISIS LSPs. It does this by
// watching the ISIS LSP count periodically in a STREAM telemetry call and counting the number of LSPs.
// It returns the number of LSPs and a boolean indicating whether the count reached the nominal count or not.
func FindISISLSPCount(t *testing.T, dut *ondatra.DUTDevice, waitTime time.Duration, nominalCount int) (int, bool) {
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

// FindProtocolSummaryRouteCount finds the number of routes for the given protocol and address family.
// It does this by watching the AFT summary periodically in a STREAM telemetry call and counting the number of routes.
// It returns the number of routes and a boolean indicating whether the count reached the nominal count or not.
func FindProtocolSummaryRouteCount(t *testing.T, dut *ondatra.DUTDevice, afi oc.E_Types_ADDRESS_FAMILY, protocol oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE, waitTime time.Duration, nominalCount int) int {
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

// FindProtocolRouteCount finds the number of routes for the given protocol and address family.
// It does this by watching the AFT entry state periodically in a STREAM telemetry call and counting the number of routes.
// It returns the number of routes and a boolean indicating whether the count reached the nominal count or not.
func FindProtocolRouteCount(t *testing.T, dut *ondatra.DUTDevice, afi oc.E_Types_ADDRESS_FAMILY, protocol oc.E_PolicyTypes_INSTALL_PROTOCOL_TYPE, waitTime time.Duration, nominalCount int) (int, bool) {
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

// ClearTestingConfig clears the testing config from the DUT.
// It removes the ISIS protocol config, LACP config, interface config, and aggregate id config from the DUT.
func ClearTestingConfig(t *testing.T, dut *ondatra.DUTDevice, defaultNetworkInstance string) {
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

// SetupTest sets up the test by configuring the ATE and DUT.
func SetupTest(t *testing.T, testInfo *TestData) *ondatra.DUTDevice {
	t.Helper()
	t.Logf("===========Configuring ATE===========")
	testInfo.ATEData.ATE = ondatra.ATE(t, "ate")
	top := otgconfighelpers.ConfigureATE(t, testInfo.ATEData.ATE, testInfo.ATEData)
	testInfo.ATEData.ATE.OTG().PushConfig(t, top)
	// testInfo.ateData.ATE.
	testInfo.ATEData.AppendTrafficFlows(t, top)
	// Start protocols on ATE
	testInfo.ATEData.ATE.OTG().StartProtocols(t)
	t.Cleanup(func() { testInfo.ATEData.ATE.OTG().StopProtocols(t) })
	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	defaultNetworkInstance = deviations.DefaultNetworkInstance(dut)
	testInfo.DUTData.IsisData.NetworkInstanceName = defaultNetworkInstance
	configureDUT(t, dut, testInfo.DUTData)
	t.Cleanup(func() { ClearTestingConfig(t, dut, testInfo.DUTData.IsisData.NetworkInstanceName) })
	return dut
}

// CreateDUTAggregateInterfacesData creates the DUT aggregate interfaces data for the test.
// The function takes the following parameters:
//   - aggregatesCount: The number of aggregates to create.
//   - subInterfacesCountPerAggregate: The number of sub-interfaces to create per aggregate.
//   - initialVlanID: The initial VLAN ID to use for the sub-interfaces.
//   - initialIPv4Address: The initial IPv4 address to use for the sub-interfaces.
//   - initialIPv6Address: The initial IPv6 address to use for the sub-interfaces.
//
// The function returns a slice of DUTAggData objects, each representing a single aggregate interface.
func CreateDUTAggregateInterfacesData(t *testing.T, aggregatesCount int, subInterfacesCountPerAggregate int, initialVlanID int, initialIPv4Address net.IP, initialIPv6Address net.IP) []*cfgplugins.DUTAggData {
	t.Helper()
	if subInterfacesCountPerAggregate > 99 {
		t.Fatalf("Sub interface count per aggregate should be less than or equal to 99, got %v", subInterfacesCountPerAggregate)
		return nil
	}
	nexIPToUse := []net.IP{initialIPv4Address, initialIPv6Address}
	var allDUTAggData []*cfgplugins.DUTAggData
	for i := 0; i < aggregatesCount; i++ {
		agg := &cfgplugins.DUTAggData{
			Attributes:      attrs.Attributes{Desc: "DUT to ATE LAG" + strconv.Itoa(i+1)},
			LacpParams:      lacpParams,
			AggType:         oc.IfAggregate_AggregationType_LACP,
			OndatraPortsIdx: []int{i},
		}
		for j := 0; j < subInterfacesCountPerAggregate; j++ {
			subInterfaces := &cfgplugins.DUTSubInterfaceData{
				VlanID:        initialVlanID + i*100 + (j + 1),
				IPv4Address:   nexIPToUse[0],
				IPv6Address:   nexIPToUse[1],
				IPv4PrefixLen: linkIpv4PLen,
				IPv6PrefixLen: linkIpv6PLen,
			}
			agg.SubInterfaces = append(agg.SubInterfaces, subInterfaces)
			nexIPToUse[0] = iputil.NextIPMultiSteps(nexIPToUse[0], 4)
			nexIPToUse[1] = iputil.NextIPMultiSteps(nexIPToUse[1], 4)
		}
		allDUTAggData = append(allDUTAggData, agg)
	}
	return allDUTAggData
}

// CreateATEEmulatedRouterData creates the ATE emulated router data for the test.
// The function takes the following parameters:
//   - dutAggregateInterfacesData: The DUT aggregate interfaces data.
//
// The function returns a slice of AteEmulatedRouterData objects, each representing a single ATE emulated router.
func CreateATEEmulatedRouterData(t *testing.T, dutAggregateInterfacesData []*cfgplugins.DUTAggData) []*otgconfighelpers.AteEmulatedRouterData {
	t.Helper()
	var emulatedRouters []*otgconfighelpers.AteEmulatedRouterData
	for ai, a := range dutAggregateInterfacesData {
		for si, s := range a.SubInterfaces {
			r := &otgconfighelpers.AteEmulatedRouterData{
				Name:                   "R" + strconv.Itoa(((ai+1)*100)+(si+1)),
				DUTIPv4:                s.IPv4Address.String(),
				ATEIPv4:                iputil.NextIPMultiSteps(s.IPv4Address, 1).String(),
				LinkIPv4PLen:           s.IPv4PrefixLen,
				DUTIPv6:                s.IPv6Address.String(),
				ATEIPv6:                iputil.NextIPMultiSteps(s.IPv6Address, 1).String(),
				LinkIPv6PLen:           s.IPv6PrefixLen,
				EthMAC:                 fmt.Sprintf("02:55:10:10:%x:%x", ai, si),
				ISISAreaAddress:        "490001",
				ISISSysID:              "6400000000" + strconv.Itoa(((ai+1)*100)+(si+1)),
				ISISLSPRefreshInterval: 65218,
				ISISSPLifetime:         65533,
				VlanID:                 s.VlanID,
			}
			emulatedRouters = append(emulatedRouters, r)
		}
	}
	return emulatedRouters
}
