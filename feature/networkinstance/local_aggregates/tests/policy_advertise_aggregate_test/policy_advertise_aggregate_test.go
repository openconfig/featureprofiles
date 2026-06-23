package policy_advertise_aggregate_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	dutAS                    = 65501
	ateAS                    = 65502
	plenIPv4                 = 30
	plenIPv6                 = 126
	triggerRoute             = "192.0.0.0/8"
	defaultRoute             = "0.0.0.0/0"
	generatedRoutePolicyName = "GENERATED_ROUTE"
	triggerRoutePolicyName   = "TRIGGER_ROUTE"
	localAggregateName       = "DEFAULT-AGG"
	ateRoutePrefix           = "192.0.0.0"
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
	ni := root.GetOrCreateNetworkInstance(dni)
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
	triggerPS.GetOrCreatePrefix(triggerRoute, "exact")

	generatedPS := definedSet.GetOrCreatePrefixSet(generatedRoutePolicyName)
	generatedPS.SetMode(oc.PrefixSet_Mode_IPV4)
	generatedPS.GetOrCreatePrefix(defaultRoute, "exact")

	pdImport := routingPolicy.GetOrCreatePolicyDefinition(triggerRoutePolicyName)
	stImport, err := pdImport.AppendNewStatement("10")
	if err != nil {
		t.Fatalf("Failed to create new statement: %v", err)
	}
	stImport.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(triggerRoutePolicyName)
	stImport.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)

	gnmi.Replace(t, dut, dc.RoutingPolicy().Config(), routingPolicy)

	t.Log("Configuring BGP...")
	bgpProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := bgpProtocol.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.SetAs(dutAS)
	global.SetRouterId(dutPort1.IPv4)

	neighbor := bgp.GetOrCreateNeighbor(atePort1.IPv4)
	neighbor.SetPeerAs(ateAS)
	neighbor.SetEnabled(true)

	afiSafi := neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afiSafi.SetEnabled(true)
	afiSafi.GetOrCreateApplyPolicy().SetImportPolicy([]string{triggerRoutePolicyName})

	gnmi.Update(t, dut, dc.NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), bgpProtocol)

	cfgplugins.RoutingPolicyBGPAdvertiseAggregate(t, dut, triggerRoutePolicyName, triggerRoute, generatedRoutePolicyName, defaultRoute, dutAS, localAggregateName)
}

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	aftSummariesCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureEnableAFTSummaries)
	if aftSummariesCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, aftSummariesCfg)
}

// configureATE creates a basic OTG configuration with a BGP neighbor.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	t.Log("Configuring ATE...")
	otgCfg := gosnappi.NewConfig()
	port1 := otgCfg.Ports().Add().SetName(ate.Port(t, "port1").ID())
	dev1 := otgCfg.Devices().Add().SetName(atePort1.Name)
	eth1 := dev1.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	eth1.Connection().SetPortName(port1.Name())
	ip1 := eth1.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4").SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4)

	bgp := dev1.Bgp().SetRouterId(ip1.Address())
	bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ip1.Name()).Peers().Add().SetName(atePort1.Name + ".BGP.peer")
	bgp4Peer.SetPeerAddress(ip1.Gateway()).SetAsNumber(uint32(ateAS)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	return otgCfg
}

// verifyBGPTelemetry checks if the BGP session is established.
func verifyBGPTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP session state...")
	dni := deviations.DefaultNetworkInstance(dut)
	bgpPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	neighborPath := bgpPath.Neighbor(atePort1.IPv4)

	verifySessionState := func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := v.Val()
		return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}

	_, ok := gnmi.Watch(t, dut, neighborPath.SessionState().State(), 2*time.Minute, verifySessionState).Await(t)
	if !ok {
		t.Fatal("BGP session did not establish.")
	}
	t.Log("BGP session established.")
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

func TestDefaultRouteGeneration(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	configureHardwareInit(t, dut)
	configureDUT(t, dut)

	ateConf := configureATE(t, ate)
	otg.PushConfig(t, ateConf)
	otg.StartProtocols(t)
	verifyBGPTelemetry(t, dut)

	t.Run("precheck", func(t *testing.T) {
		t.Log("Precheck - ensure default route and 192.0.0.0/8 prefix are not present in the DUT routing table")
		verifyRIBRoute(t, dut, triggerRoute, false)
		verifyRIBRoute(t, dut, defaultRoute, false)
	})

	t.Run("advertise_trigger_route_check_generation", func(t *testing.T) {
		t.Logf("Advertising route %s from ATE", triggerRoute)
		bgpPeer := ateConf.Devices().Items()[0].Bgp().Ipv4Interfaces().Items()[0].Peers().Items()[0]
		route := bgpPeer.V4Routes().Add().SetName("triggerRoute")
		route.SetNextHopIpv4Address(atePort1.IPv4)
		route.Addresses().Add().SetAddress(ateRoutePrefix).SetPrefix(8)

		otg.PushConfig(t, ateConf)
		otg.StartProtocols(t)

		verifyRIBRoute(t, dut, triggerRoute, true)
		verifyRIBRoute(t, dut, defaultRoute, true)
	})

	t.Run("withdraw_trigger_route_check_deletion", func(t *testing.T) {
		t.Logf("Withdrawing route %s from ATE", triggerRoute)
		ateConfWithdraw := configureATE(t, ate)
		otg.PushConfig(t, ateConfWithdraw)
		otg.StartProtocols(t)

		verifyRIBRoute(t, dut, triggerRoute, false)
		verifyRIBRoute(t, dut, defaultRoute, false)
	})
}
