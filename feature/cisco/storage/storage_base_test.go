package storage_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// Common types and constants
type testArgs struct {
	ate *ondatra.ATEDevice
	ctx context.Context
	dut *ondatra.DUTDevice
	top *ondatra.ATETopology
}

type storageTestCase struct {
	name        string
	path        string
	counterType string
	description string
	fn          func(ctx context.Context, t *testing.T, args *testArgs, path string)
}

const (
	// Storage component type for filtering
	storageType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_STORAGE

	// Network configuration constants
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
)

var lcList = []string{}

// Common attribute variables
var (
	dutSrc = attrs.Attributes{
		Desc:    "dutSrc",
		IPv4:    "100.121.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:1",
		IPv6Len: ipv6PrefixLen,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "100.121.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:2",
		IPv6Len: ipv6PrefixLen,
	}
	dutDst = attrs.Attributes{
		Desc:    "dutDst",
		IPv4:    "100.122.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:122:1:1",
		IPv6Len: ipv6PrefixLen,
	}
	ateDst = attrs.Attributes{
		Name:    "ateDst",
		IPv4:    "100.122.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:122:1:2",
		IPv6Len: ipv6PrefixLen,
	}
)

// sortPorts sorts the given slice of ports by the testbed port ID in ascending order.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

// getLinecardComponents returns linecard components with pattern "X/Y/CPUZ"
func getLinecardComponents(t *testing.T, args *testArgs) []string {
	t.Helper()
	// Get Node components using component API
	allComponents := gnmi.GetAll(t, args.dut, gnmi.OC().ComponentAny().State())
	var nodeComponents []string
	for _, component := range allComponents {
		name := component.GetName()
		// Filter for Linecard components only with exact pattern like "0/X/CPUY"
		// This excludes RP nodes, sub-components, and focuses only on main linecard CPUs
		if strings.Count(name, "/") == 2 &&
			(strings.HasSuffix(name, "/CPU0") || strings.HasSuffix(name, "/CPU1")) &&
			!strings.Contains(name, "RP0") &&
			!strings.Contains(name, "RP1") &&
			!strings.Contains(name, "/RP") &&
			!strings.Contains(name, "-") &&
			!strings.Contains(strings.ToUpper(name), "IOSXR-NODE") {
			nodeComponents = append(nodeComponents, name)
		}
	}
	//t.Logf("Found linecard components: %v", nodeComponents)

	if len(nodeComponents) == 0 {
		t.Skipf("No linecard components found on device %s", args.dut.Model())
	}

	return nodeComponents
}

// collectAndLogCounters collects counter data for all paths and logs initial values
func collectAndLogCounters(t *testing.T, data map[string]ygnmi.WildcardQuery[uint64]) {
	t.Helper()
	// aggregate pre counters for a path across all the destination linecards
	for path, query := range data {
		pre, err := getData(t, path, query)
		if err != nil {
			t.Fatalf("failed to get data for path %s pre trigger: %v", path, err)
		}
		t.Logf("Initial counter for path %s : %d", path, pre)
	}
}

// createQueries creates wildcard queries for all linecard components
func createQueries(t *testing.T, args *testArgs, pathSuffix string) map[string]ygnmi.WildcardQuery[uint64] {
	t.Helper()
	data := make(map[string]ygnmi.WildcardQuery[uint64])

	// Get linecard components using common helper
	nodeComponents := getLinecardComponents(t, args)

	// Create queries for all node components
	for _, component := range nodeComponents {
		//t.Logf("Testing component: %s", component)
		path := fmt.Sprintf("/components/component[name=%s]/%s", component, pathSuffix)
		query, err := schemaless.NewWildcard[uint64](path, "openconfig")
		if err != nil {
			t.Fatalf("failed to create query for path %s: %v", path, err)
		}
		data[path] = query
	}

	return data
}

// testStorageCounterSampleMode tests storage counters using SAMPLE subscription mode
func testStorageCounterSampleMode(t *testing.T, args *testArgs, pathSuffix string) {
	t.Helper()
	for _, subMode := range []gpb.SubscriptionMode{gpb.SubscriptionMode_SAMPLE} {
		t.Logf("Path name: %s", pathSuffix)
		t.Logf("Subscription mode: %v", subMode)

		// Create queries for all components using common helper
		data := createQueries(t, args, pathSuffix)

		// Collect and log counter data using common helper
		collectAndLogCounters(t, data)
	}
}

// testStorageCounterOnceMode tests storage counters using ONCE subscription mode
func testStorageCounterOnceMode(t *testing.T, args *testArgs, pathSuffix string) {
	t.Helper()
	for _, subMode := range []gpb.SubscriptionList_Mode{gpb.SubscriptionList_ONCE} {
		t.Logf("Path name: %s", pathSuffix)
		t.Logf("Subscription mode: %v", subMode)

		// Create queries for all components using common helper
		data := createQueries(t, args, pathSuffix)

		// Collect and log counter data using common helper
		collectAndLogCounters(t, data)
	}
}

// getData performs a subscription to the specified path using a wildcard query.
func getData(t *testing.T, path string, query ygnmi.WildcardQuery[uint64]) (uint64, error) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")

	watchOpts := dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE),
		ygnmi.WithSampleInterval(60*time.Second))
	data, pred := gnmi.WatchAll(t, watchOpts, query, 60*time.Second, func(val *ygnmi.Value[uint64]) bool {
		_, present := val.Val()
		stringPath, err := ygot.PathToString(val.Path)
		if err != nil {
			t.Logf("error converting path to string: %v", err)
			return false
		}
		if stringPath == path {
			return present
		}
		return !present
	},
	).Await(t)
	if pred == false {
		return 0, fmt.Errorf("watch failed for path %s. Predicate returned is %v", path, pred)
	}

	counter, ok := data.Val()
	if ok {
		return counter, nil
	} else {
		return 0, fmt.Errorf("failed to collect data for path %s", path)
	}
}

// configureDUT configures the DUT interfaces
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dutPorts := sortPorts(dut.Ports())
	d := gnmi.OC()

	// incoming interface is Bundle-Ether121 with only 1 member (port1)
	incoming := &oc.Interface{Name: ygot.String("Bundle-Ether121")}
	gnmi.Replace(t, dut, d.Interface(*incoming.Name).Config(), configInterfaceDUT(incoming, &dutSrc))
	srcPort := dutPorts[0]
	dutSource := generateBundleMemberInterfaceConfig(t, srcPort.Name(), *incoming.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(srcPort.Name()).Config(), dutSource)

	outgoing := &oc.Interface{Name: ygot.String("Bundle-Ether122")}
	outgoingData := configInterfaceDUT(outgoing, &dutDst)
	g := outgoingData.GetOrCreateAggregation()
	g.LagType = oc.IfAggregate_AggregationType_LACP
	gnmi.Replace(t, dut, d.Interface(*outgoing.Name).Config(), configInterfaceDUT(outgoing, &dutDst))
	for _, port := range dutPorts[1:] {
		dutDest := generateBundleMemberInterfaceConfig(t, port.Name(), *outgoing.Name)
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), dutDest)
	}
}

// getStorageComponents returns all storage type components
func (args *testArgs) getStorageComponents(t *testing.T) []*oc.Component {
	t.Helper()

	components := gnmi.GetAll(t, args.dut, gnmi.OC().ComponentAny().State())
	var storageComponents []*oc.Component

	for _, component := range components {
		if component.GetType() == storageType {
			storageComponents = append(storageComponents, component)
		}
	}

	t.Logf("Found %d storage components", len(storageComponents))
	return storageComponents
}

// configInterfaceDUT configures the interfaces with corresponding addresses
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	s6 := s.GetOrCreateIpv6()
	s6a := s6.GetOrCreateAddress(a.IPv6)
	s6a.PrefixLength = ygot.Uint8(ipv6PrefixLen)

	return i
}

// generateBundleMemberInterfaceConfig generates bundle member interface configuration
func generateBundleMemberInterfaceConfig(t *testing.T, name, bundleID string) *oc.Interface {
	t.Helper()
	i := &oc.Interface{Name: ygot.String(name)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)
	return i
}

func testStorageCounterSystemEvents(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {

	//function call to reload linecards
	linecardsReload(t, args, ctx, pathSuffix)
	rpfoReload(t, args, ctx, pathSuffix)
	reloadRouter(t, args, ctx, pathSuffix)
	processRestart(t, args, ctx, pathSuffix)
}

// function to reload linecards
func linecardsReload(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	lcList := util.GetLCList(t, args.dut)
	if len(lcList) == 0 {
		t.Skip("No linecards found")
	}
	util.ReloadLinecards(t, lcList)

	time.Sleep(120 * time.Second)

	//After linecard reload fetch the values of the counters again and check if the counters value have changed
	// aggregate pre counters for a path across all the destination linecards
	testStorageCounterSampleMode(t, args, pathSuffix)

}

func reloadRouter(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	//function call to reload router
	util.ReloadRouter(t, args.dut)
	time.Sleep(120 * time.Second)

	testStorageCounterSampleMode(t, args, pathSuffix)
}

func processRestart(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	//function call to restart emsd process
	util.ProcessRestart(t, args.dut, "emsd")
	time.Sleep(120 * time.Second)

	testStorageCounterSampleMode(t, args, pathSuffix)
}

// function to reload RPFO
func rpfoReload(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {

	util.RPFO(t, args.dut)
	time.Sleep(120 * time.Second)

	testStorageCounterSampleMode(t, args, pathSuffix)
}
