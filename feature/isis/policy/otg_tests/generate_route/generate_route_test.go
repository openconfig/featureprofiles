package generate_route

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	dutAS                     = 65501
	ateAS                     = 65502
	plenIPv4                  = 30
	plenIPv6                  = 126
	triggerRoute              = "192.168.2.2/32"
	defaultRoute              = "192.168.2.0/30"
	generateDefaultPolicyName = "GENERATE_DEFAULT_ROUTE"
	triggerRoutePolicyName    = "TRIGGER_ROUTE"
	localAggregateName        = "DEFAULT-AGG"
	ateRoutePrefix            = "192.168.2.2"
	isisInstance             = "DEFAULT"
	dutAreaAddress           = "49.0001"
	dutSysID                 = "1920.0000.2001"
	otgSysID2            = "640000000001"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port 1",
		IPv4:    "198.51.100.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "198.51.100.2",
		IPv6:    "2001:db8::2",
		MAC:     "02:00:01:01:01:01",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureDUT configures the DUT with a BGP session and a policy to accept the trigger route.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Start DUT configuration")

	dc := gnmi.OC()
	root := &oc.Root{}
	dni := deviations.DefaultNetworkInstance(dut)
	p1 := dut.Port(t, "port1").Name()
	i1 := dutPort1.NewOCInterface(p1, dut)
	gnmi.Replace(t, dut, dc.Interface(p1).Config(), i1)

	t.Log("Configuring default network instance...")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1, dni, 0)
	}

	t.Log("Configuring routing policy...")
	routingPolicy := root.GetOrCreateRoutingPolicy()

	definedSet := routingPolicy.GetOrCreateDefinedSets()
	triggerPS := definedSet.GetOrCreatePrefixSet(triggerRoutePolicyName)
	triggerPS.SetMode(oc.PrefixSet_Mode_IPV4)
	triggerPS.GetOrCreatePrefix("192.0.0.0/8", "exact")

	pdImport := routingPolicy.GetOrCreatePolicyDefinition(triggerRoutePolicyName)
	stImport, err := pdImport.AppendNewStatement("10")
	if err != nil {
		t.Fatalf("Failed to create new statement: %v", err)
	}
	stImport.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(triggerRoutePolicyName)
	stImport.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	gnmi.Replace(t, dut, dc.RoutingPolicy().Config(), routingPolicy)

	t.Log("Configuring local aggregate for 0.0.0.0/0...")
	ni := root.GetOrCreateNetworkInstance(dni)

	// aggProto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_LOCAL_AGGREGATE, localAggregateName)
	// aggProto.SetIdentifier(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_LOCAL_AGGREGATE)
	// aggProto.SetName(localAggregateName)

	// aggProto.GetOrCreateAggregate(defaultRoute)
	// aggProto.GetOrCreateAggregate(defaultRoute).SetPrefix("0.0.0.0/0")
	// aggProto.SetEnabled(true)

	// gnmi.Replace(t, dut, dc.NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_LOCAL_AGGREGATE, localAggregateName).Config(), aggProto)

	t.Log("Configuring ISIS...")
	dutConfIsisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.SetEnabled(true)
	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	if deviations.ISISSingleTopologyRequired(dut) {
		afv6 := globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		afv6.GetOrCreateMultiTopology().SetAfiName(oc.IsisTypes_AFI_TYPE_IPV4)
		afv6.GetOrCreateMultiTopology().SetSafiName(oc.IsisTypes_SAFI_TYPE_UNICAST)
	}
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	if deviations.ISISLevelEnabled(dut) {
		isisLevel2.Enabled = ygot.Bool(true)
	}
	isisIntf := isis.GetOrCreateInterface(i1.GetName())
	isisIntf.Enabled = ygot.Bool(true)
	isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	isisIntfLevel := isisIntf.GetOrCreateLevel(2)
	isisIntfLevel.Enabled = ygot.Bool(true)
	isisIntfLevelAfiv4 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfiv4.Enabled = ygot.Bool(true)
	isisIntfLevelAfiv6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfiv6.Enabled = ygot.Bool(true)
	if deviations.ISISInterfaceAfiUnsupported(dut) {
		isisIntf.Af = nil
	}
	if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
		isisIntfLevelAfiv4.Enabled = nil
		isisIntfLevelAfiv6.Enabled = nil
	}
	gnmi.Replace(t, dut, dutConfIsisPath.Config(), prot)
}

// configureATE creates a basic OTG configuration with a ISIS neighbor.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	t.Log("Configuring ATE...")
	otgCfg := gosnappi.NewConfig()
	port1 := otgCfg.Ports().Add().SetName(ate.Port(t, "port1").ID())
	dev1 := otgCfg.Devices().Add().SetName(atePort1.Name)
	eth1 := dev1.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	eth1.Connection().SetPortName(port1.Name())
	eth1.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4").SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(30)

	isisDut := dev1.Isis().SetName("ISIS")
	isisDut.SetSystemId(otgSysID2)
	isisDut.Basic().SetHostname(isisDut.Name())
	isisDut.Basic().SetEnableWideMetric(true)
	isisDut.Basic().SetLearnedLspFilter(true)

	isisDut.Advanced().SetAreaAddresses([]string{"490002"})

	isisInt := isisDut.Interfaces().Add().SetEthName(dev1.Ethernets().Items()[0].Name()).
		SetName("devIsisInt2").
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
		SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

	isisInt.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)


	return otgCfg
}

func verifyISISTelemetry(t *testing.T, dut *ondatra.DUTDevice, dutIntf []string) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()
	for _, intfName := range dutIntf {
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			intfName = intfName + ".0"
		}
		nbrPath := statePath.Interface(intfName)
		query := nbrPath.LevelAny().AdjacencyAny().AdjacencyState().State()
		_, ok := gnmi.WatchAll(t, dut, query, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
			state, present := val.Val()
			return present && state == oc.Isis_IsisInterfaceAdjState_UP
		}).Await(t)
		if !ok {
			t.Logf("IS-IS state on %v has no adjacencies", intfName)
			t.Fatal("No IS-IS adjacencies reported.")
		}
	}
}

// verifyRIBRoute verifies whether a given prefix exists or not in the DUT's AFT.
func verifyRIBRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, shouldExist bool) {
	t.Helper()
	dni := deviations.DefaultNetworkInstance(dut)
	ribQuery := gnmi.OC().NetworkInstance(dni).Afts().Ipv4Entry(prefix)

	if shouldExist {
		t.Logf("Verifying route %s is present in RIB...", prefix)
		_, ok := gnmi.Watch(t, dut, ribQuery.State(), 1*time.Minute, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			return v.IsPresent()
		}).Await(t)
		if !ok {
			t.Fatalf("Route %s was not installed in the RIB, but it should be.", prefix)
		}
		t.Logf("Route %s is present in the RIB as expected.", prefix)
	} else {
		t.Logf("Verifying route %s is absent from RIB...", prefix)
		_, ok := gnmi.Watch(t, dut, ribQuery.State(), 1*time.Minute, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			return !v.IsPresent()
		}).Await(t)
		if !ok {
			t.Fatalf("Route %s was not withdrawn from the RIB, but it should have been.", prefix)
		}
		t.Logf("Route %s is not present in the RIB as expected.", prefix)
	}
}

func TestDefaultRouteGeneration(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	configureDUT(t, dut)

	ateConf := configureATE(t, ate)
	otg.PushConfig(t, ateConf)
	otg.StartProtocols(t)
	// verifyISISTelemetry(t, dut)

	// t.Run("precheck", func(t *testing.T) {
	// 	t.Log("Precheck - ensure default route and 192.0.0.0/8 prefix are not present in the DUT routing table")
	// 	verifyRIBRoute(t, dut, triggerRoute, false)
	// 	verifyRIBRoute(t, dut, defaultRoute, false)
	// })

	// t.Run("advertise_trigger_route_check_generation", func(t *testing.T) {
	// 	t.Logf("Advertising route %s from ATE", triggerRoute)
	// 	bgpPeer := ateConf.Devices().Items()[0].Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
	// 	route := bgpPeer.V4Routes().Add().SetName("triggerRoute")
	// 	route.SetNextHopIpv4Address(atePort1.IPv4)
	// 	route.Addresses().Add().SetAddress(ateRoutePrefix).SetPrefix(8)

	// 	otg.PushConfig(t, ateConf)
	// 	otg.StartProtocols(t)

	// 	verifyRIBRoute(t, dut, triggerRoute, true)
	// 	verifyRIBRoute(t, dut, defaultRoute, true)
	// })

	// t.Run("withdraw_trigger_route_check_deletion", func(t *testing.T) {
	// 	t.Logf("Withdrawing route %s from ATE", triggerRoute)
	// 	ateConfWithdraw := configureATE(t, ate)
	// 	otg.PushConfig(t, ateConfWithdraw)
	// 	otg.StartProtocols(t)

	// 	verifyRIBRoute(t, dut, triggerRoute, false)
	// 	verifyRIBRoute(t, dut, defaultRoute, false)
	// })
}
