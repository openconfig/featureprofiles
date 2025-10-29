package policy_advertisea_ggregate

import (
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
	dutAS          = 65501
	ateAS          = 65502
	plenIPv4       = 30
	plenIPv6       = 126
	triggerRoute   = "192.0.0.0/8"
	defaultRoute   = "0.0.0.0/0"
	policyName     = "GENERATE_DEFAULT"
	prefixSetName  = "TRIGGER_ROUTE_PS"
	ateRoutePrefix = "192.0.0.0"
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
	t.Log("Configuring DUT interfaces...")
	dc := gnmi.OC()
	p1 := dut.Port(t, "port1").Name()
	i1 := dutPort1.NewOCInterface(p1, dut)
	gnmi.Replace(t, dut, dc.Interface(p1).Config(), i1)

	t.Log("Configuring default network instance...")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1, deviations.DefaultNetworkInstance(dut), 0)
	}

	t.Log("Configuring routing policy...")
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	ps := rp.GetOrCreateDefinedSets().GetOrCreatePrefixSet(prefixSetName)
	ps.SetMode(oc.PrefixSet_Mode_IPV4)
	ps.GetOrCreatePrefix(triggerRoute, "exact")

	pd := rp.GetOrCreatePolicyDefinition(policyName)
	st, err := pd.AppendNewStatement("20")
	if err != nil {
		t.Fatalf("Failed to create new statement: %v", err)
	}
	st.GetOrCreateConditions().GetOrCreateMatchPrefixSet().SetPrefixSet(prefixSetName)
	st.GetOrCreateActions().SetPolicyResult(oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	gnmi.Replace(t, dut, dc.RoutingPolicy().Config(), rp)

	t.Log("Configuring BGP...")
	dni := deviations.DefaultNetworkInstance(dut)
	ni := root.GetOrCreateNetworkInstance(dni)
	bgpProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := bgpProtocol.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(dutAS)
	global.RouterId = ygot.String(dutPort1.IPv4)

	neighbor := bgp.GetOrCreateNeighbor(atePort1.IPv4)
	neighbor.PeerAs = ygot.Uint32(ateAS)
	neighbor.Enabled = ygot.Bool(true)

	afiSafi := neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afiSafi.Enabled = ygot.Bool(true)
	afiSafi.GetOrCreateApplyPolicy().SetImportPolicy([]string{policyName})

	gnmi.Update(t, dut, dc.NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), bgpProtocol)
}

// configureATE creates a basic OTG configuration with a BGP neighbor.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Log("Configuring ATE...")
	otgCfg := gosnappi.NewConfig()
	port1 := otgCfg.Ports().Add().SetName(ate.Port(t, "port1").ID())
	dev1 := otgCfg.Devices().Add().SetName(atePort1.Name)
	eth1 := dev1.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	eth1.Connection().SetPortName(port1.Name())
	ip1 := eth1.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4").SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))

	bgp := dev1.Bgp().SetRouterId(ip1.Address())
	bgp4Peer := bgp.Ipv4Interfaces().Add().SetIpv4Name(ip1.Name()).Peers().Add().SetName(atePort1.Name + ".BGP.peer")
	bgp4Peer.SetPeerAddress(ip1.Gateway()).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	return otgCfg
}

// verifyBGPTelemetry checks if the BGP session is established.
func verifyBGPTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Verifying BGP session state...")
	dni := deviations.DefaultNetworkInstance(dut)
	bgpPath := gnmi.OC().NetworkInstance(dni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	neighborPath := bgpPath.Neighbor(atePort1.IPv4)

	_, ok := gnmi.Watch(t, dut, neighborPath.SessionState().State(), 2*time.Minute, func(v *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := v.Val()
		return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
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

// TestDefaultRouteGeneration is the main test function.
func TestDefaultRouteGeneration(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	// Configure the DUT.
	configureDUT(t, dut)

	// Configure base ATE topology and BGP neighbor.
	ateConf := configureATE(t, ate)
	otg.PushConfig(t, ateConf)
	otg.StartProtocols(t)
	verifyBGPTelemetry(t, dut)

	// Subtest: Pre-check to ensure routes are not present initially.
	t.Run("Pre-check", func(t *testing.T) {
		verifyRIBRoute(t, dut, triggerRoute, false)
		verifyRIBRoute(t, dut, defaultRoute, false)
	})

	// Subtest: Advertise the trigger route and verify default route generation.
	t.Run("Advertise Trigger Route and Verify Generation", func(t *testing.T) {
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

	// Subtest: Withdraw the trigger route and verify default route is removed.
	t.Run("Withdraw Trigger Route and Verify Deletion", func(t *testing.T) {
		t.Logf("Withdrawing route %s from ATE", triggerRoute)
		// Re-configure ATE without the route to trigger a withdrawal.
		ateConfWithdraw := configureATE(t, ate)
		otg.PushConfig(t, ateConfWithdraw)
		otg.StartProtocols(t)

		verifyRIBRoute(t, dut, triggerRoute, false)
		verifyRIBRoute(t, dut, defaultRoute, false)
	})
}
