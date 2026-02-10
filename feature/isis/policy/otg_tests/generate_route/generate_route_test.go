package generate_route_test

import (
	"fmt"
	"strings"
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
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	ipv4PrefixLen                = 30
	ipv6PrefixLen                = 126
	isisName                     = "DEFAULT"
	dutAreaAddr                  = "49.0001"
	ateAreaAddr                  = "49.0002"
	dutSysID                     = "1920.0000.2001"
	ateSysID                     = "640000000001"
	triggerRoute                 = "192.0.2.2"
	triggerRoutePrefixLen        = 32
	triggerRouteIPv6             = "fc00::2"
	triggerRouteIPv6PRefixLen    = 128
	generateRoute                = "192.0.2.0/30"
	generateIPv6Route            = "fc00::/126"
	generatedRoutePolicyName     = "GENERATED_ROUTE"
	triggerRoutePolicyName       = "TRIGGER_ROUTE"
	generatedRoutePolicyIPv6Name = "GENERATED_ROUTE_IPV6"
	triggerRoutePolicyIPv6Name   = "TRIGGER_ROUTE_IPV6"
	isisRouteName                = "v4-isisNet-dev"
	isisIPv6RouteName            = "v6-isisNet-dev"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		Name:    "port1",
		IPv4:    "198.51.100.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "198.51.100.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type ipAddr struct {
	address string
	prefix  uint32
}

type testData struct {
	dut            *ondatra.DUTDevice
	ate            *ondatra.ATEDevice
	top            gosnappi.Config
	otgPort        gosnappi.Device
	advertisedIPv4 ipAddr
	advertisedIPv6 ipAddr
}

func (td *testData) awaitISISAdjacency(t *testing.T, p *ondatra.Port, isisName string) error {
	t.Helper()
	isis := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName).Isis()
	intf := isis.Interface(p.Name())
	if deviations.ExplicitInterfaceInDefaultVRF(td.dut) {
		intf = isis.Interface(p.Name() + ".0")
	}
	query := intf.Level(2).AdjacencyAny().AdjacencyState().State()
	_, ok := gnmi.WatchAll(t, td.dut, query, time.Minute, func(v *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		state, _ := v.Val()
		return v.IsPresent() && state == oc.Isis_IsisInterfaceAdjState_UP
	}).Await(t)

	if !ok {
		return fmt.Errorf("timeout - waiting for adjacency state")
	}
	return nil
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	for _, dutPorts := range []*attrs.Attributes{&dutPort1} {
		dutPort := dut.Port(t, dutPorts.Name)
		dutInt := dutPorts.NewOCInterface(dutPort.Name(), dut)
		gnmi.Replace(t, dut, gnmi.OC().Interface(dutPort.Name()).Config(), dutInt)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, dutPort.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) []gosnappi.Device {
	t.Helper()
	p1 := ate.Port(t, "port1")

	d1 := atePort1.AddToOTG(top, p1, &dutPort1)
	return []gosnappi.Device{d1}
}

func (td *testData) configureISIS(t *testing.T) {
	t.Helper()

	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(td.dut))
	isisP := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisName)
	isisP.SetEnabled(true)
	isis := isisP.GetOrCreateIsis()

	g := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(td.dut) {
		g.SetInstance(isisName)
	}
	g.LevelCapability = oc.Isis_LevelType_LEVEL_2
	g.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddr, dutSysID)}
	g.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
	g.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)

	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	if deviations.ISISLevelEnabled(td.dut) {
		isisLevel2.SetEnabled(true)
	}

	interfaceName := td.dut.Port(t, "port1").Name()
	if deviations.ExplicitInterfaceInDefaultVRF(td.dut) {
		interfaceName += ".0"
	}

	isisIntf := isis.GetOrCreateInterface(interfaceName)
	isisIntf.GetOrCreateInterfaceRef().SetInterface(interfaceName)
	isisIntf.GetOrCreateInterfaceRef().SetSubinterface(0)
	if deviations.InterfaceRefConfigUnsupported(td.dut) {
		isisIntf.InterfaceRef = nil
	}
	isisIntf.SetEnabled(true)
	isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
	isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
	if deviations.ISISInterfaceAfiUnsupported(td.dut) {
		isisIntf.Af = nil
	}

	isisIntfLevel := isisIntf.GetOrCreateLevel(2)
	isisIntfLevel.SetEnabled(true)

	isisIntfLevelAfiv4 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfiv4.SetMetric(10)
	isisIntfLevelAfiv4.SetEnabled(true)
	isisIntfLevelAfiv6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfiv6.SetMetric(10)
	isisIntfLevelAfiv6.SetEnabled(true)
	if deviations.MissingIsisInterfaceAfiSafiEnable(td.dut) {
		isisIntfLevelAfiv4.Enabled = nil
		isisIntfLevelAfiv6.Enabled = nil
	}

	gnmi.Update(t, td.dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(td.dut)).Config(), ni)

	devISIS := td.otgPort.Isis().SetSystemId(ateSysID).SetName(td.otgPort.Name() + ".ISIS")
	devISIS.Basic().SetHostname(devISIS.Name()).SetLearnedLspFilter(true)
	devISIS.Advanced().SetAreaAddresses([]string{strings.Replace(ateAreaAddr, ".", "", -1)})
	devISISInt := devISIS.Interfaces().Add().
		SetEthName(td.otgPort.Ethernets().Items()[0].Name()).SetName("devISISInt").
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetMetric(10)
	devISISInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	netv4 := td.otgPort.Isis().V4Routes().Add().SetName(isisRouteName).SetLinkMetric(10)
	netv4.Addresses().Add().SetAddress(td.advertisedIPv4.address).SetPrefix(td.advertisedIPv4.prefix)

	netv6 := td.otgPort.Isis().V6Routes().Add().SetName(isisIPv6RouteName).SetLinkMetric(10)
	netv6.Addresses().Add().SetAddress(td.advertisedIPv6.address).SetPrefix(td.advertisedIPv6.prefix)
}

func (td *testData) createPolicyToAdvertiseAggregate(t *testing.T) {
	t.Helper()
	t.Log("Configuring routing policy...")
	root := &oc.Root{}
	routingPolicy := root.GetOrCreateRoutingPolicy()

	definedSet := routingPolicy.GetOrCreateDefinedSets()

	triggerPS := definedSet.GetOrCreatePrefixSet(triggerRoutePolicyName)
	triggerPS.SetMode(oc.PrefixSet_Mode_IPV4)
	triggerPS.GetOrCreatePrefix(fmt.Sprintf("%s/%d", triggerRoute, triggerRoutePrefixLen), "exact")

	generatedPS := definedSet.GetOrCreatePrefixSet(generatedRoutePolicyName)
	generatedPS.SetMode(oc.PrefixSet_Mode_IPV4)
	generatedPS.GetOrCreatePrefix(generateRoute, "exact")

	triggerPSV6 := definedSet.GetOrCreatePrefixSet(triggerRoutePolicyIPv6Name)
	triggerPSV6.SetMode(oc.PrefixSet_Mode_IPV6)
	triggerPSV6.GetOrCreatePrefix(fmt.Sprintf("%s/%d", triggerRouteIPv6, triggerRouteIPv6PRefixLen), "exact")

	generatedPSV6 := definedSet.GetOrCreatePrefixSet(generatedRoutePolicyIPv6Name)
	generatedPSV6.SetMode(oc.PrefixSet_Mode_IPV6)
	generatedPSV6.GetOrCreatePrefix(generateIPv6Route, "exact")

	pdImport := routingPolicy.GetOrCreatePolicyDefinition(triggerRoutePolicyName)
	stImport, err := pdImport.AppendNewStatement("10")
	if err != nil {
		t.Fatalf("Failed to create new statement: %v", err)
	}
	stImport.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(triggerRoutePolicyName)
	stImport.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	gnmi.Replace(t, td.dut, gnmi.OC().RoutingPolicy().Config(), routingPolicy)

	cfgplugins.GenerateDynamicRouteWithISIS(t, td.dut, &gnmi.SetBatch{})
}

// verifyRIBRoute verifies whether a given prefix exists or not in the DUT's AFT.
func verifyRIBRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, shouldExist bool) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)
	ribQuery := gnmi.OC().NetworkInstance(dni).Afts().Ipv4Entry(prefix)

	if shouldExist {
		t.Logf("Verifying route %s is present in RIB...", prefix)
		_, ok := gnmi.Watch(t, dut, ribQuery.State(), 2*time.Minute, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			return v.IsPresent()
		}).Await(t)
		if !ok {
			t.Fatalf("Route %s was not installed in the RIB, but it should be.", prefix)
		}
		t.Logf("Route %s is present in the RIB as expected.", prefix)
	} else {
		t.Logf("Verifying route %s is absent from RIB...", prefix)
		_, ok := gnmi.Watch(t, dut, ribQuery.State(), 2*time.Minute, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			return !v.IsPresent()
		}).Await(t)
		if !ok {
			t.Fatalf("Route %s was not withdrawn from the RIB, but it should have been.", prefix)
		}
		t.Logf("Route %s is not present in the RIB as expected.", prefix)
	}
}

// verifyRIBIPv6Route verifies whether a given prefix exists or not in the DUT's AFT.
func verifyRIBIPv6Route(t *testing.T, dut *ondatra.DUTDevice, prefix string, shouldExist bool) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)
	ribQuery := gnmi.OC().NetworkInstance(dni).Afts().Ipv6Entry(prefix)

	if shouldExist {
		t.Logf("Verifying route %s is present in RIB...", prefix)
		_, ok := gnmi.Watch(t, dut, ribQuery.State(), 2*time.Minute, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			return v.IsPresent()
		}).Await(t)
		if !ok {
			t.Fatalf("Route %s was not installed in the RIB, but it should be.", prefix)
		}
		t.Logf("Route %s is present in the RIB as expected.", prefix)
	} else {
		t.Logf("Verifying route %s is absent from RIB...", prefix)
		_, ok := gnmi.Watch(t, dut, ribQuery.State(), 2*time.Minute, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			return !v.IsPresent()
		}).Await(t)
		if !ok {
			t.Fatalf("Route %s was not withdrawn from the RIB, but it should have been.", prefix)
		}
		t.Logf("Route %s is not present in the RIB as expected.", prefix)
	}
}

func advertiseISISRoutes(t *testing.T, routeNames []string) {

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(routeNames).SetState(gosnappi.StateProtocolRouteState.ADVERTISE)
	otg.SetControlState(t, cs)

}

func withdrawISISRoutes(t *testing.T, routeNames []string) {

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Protocol().Route().SetNames(routeNames).SetState(gosnappi.StateProtocolRouteState.WITHDRAW)
	otg.SetControlState(t, cs)

}

func TestGenerateRoute(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	devs := configureOTG(t, ate, top)

	td := testData{
		dut:            dut,
		ate:            ate,
		top:            top,
		otgPort:        devs[0],
		advertisedIPv4: ipAddr{address: triggerRoute, prefix: triggerRoutePrefixLen},
		advertisedIPv6: ipAddr{address: triggerRouteIPv6, prefix: triggerRouteIPv6PRefixLen},
	}
	td.configureISIS(t)
	td.createPolicyToAdvertiseAggregate(t)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

	if err := td.awaitISISAdjacency(t, dut.Port(t, "port1"), isisName); err != nil {
		t.Fatal(err)
	}

	withdrawISISRoutes(t, []string{isisRouteName, isisIPv6RouteName})

	t.Run("precheck", func(t *testing.T) {
		t.Logf("Precheck - ensure trigger route and %s prefix are not present in the DUT routing table", generateRoute)
		verifyRIBRoute(t, dut, fmt.Sprintf("%s/%d", triggerRoute, triggerRoutePrefixLen), false)
		verifyRIBRoute(t, dut, generateRoute, false)

		t.Logf("Precheck - ensure trigger route and %s prefix are not present in the DUT routing table", generateIPv6Route)
		verifyRIBIPv6Route(t, dut, fmt.Sprintf("%s/%d", triggerRouteIPv6, triggerRouteIPv6PRefixLen), false)
		verifyRIBIPv6Route(t, dut, generateIPv6Route, false)
	})

	t.Run("advertise_trigger_route_check_generation", func(t *testing.T) {
		t.Log("Advertising routes from ATE")
		td.createPolicyToAdvertiseAggregate(t)
		advertiseISISRoutes(t, []string{isisRouteName, isisIPv6RouteName})

		verifyRIBRoute(t, dut, fmt.Sprintf("%s/%d", triggerRoute, triggerRoutePrefixLen), true)
		verifyRIBRoute(t, dut, generateRoute, true)

		verifyRIBIPv6Route(t, dut, fmt.Sprintf("%s/%d", triggerRouteIPv6, triggerRouteIPv6PRefixLen), true)
		verifyRIBIPv6Route(t, dut, generateIPv6Route, true)
	})

	t.Run("withdraw_trigger_route_check_route_deletion", func(t *testing.T) {
		t.Logf("Withdraw route %s from ATE", triggerRoute)

		withdrawISISRoutes(t, []string{isisRouteName, isisIPv6RouteName})

		verifyRIBRoute(t, dut, fmt.Sprintf("%s/%d", triggerRoute, triggerRoutePrefixLen), false)
		verifyRIBRoute(t, dut, generateRoute, false)

		verifyRIBIPv6Route(t, dut, fmt.Sprintf("%s/%d", triggerRouteIPv6, triggerRouteIPv6PRefixLen), false)
		verifyRIBIPv6Route(t, dut, generateIPv6Route, false)
	})
}
