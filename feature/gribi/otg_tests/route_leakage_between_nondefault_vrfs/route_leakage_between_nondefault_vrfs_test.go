package route_leakage_between_nondefault_vrfs_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen         = 30
	vrf1Name              = "VRF-1"
	vrf2Name              = "VRF-2"
	dutAS1                = 65003
	dutAS2                = 65002
	ateAS1                = 65001
	ateAS2                = 65003
	globalAS              = 65000
	advertisedNet1StartIP = "50.1.1.0"
	advertisedNet2StartIP = "60.1.1.0"
	routePrefix           = 24
	routeCount            = 1
	pps                   = 10000
	frameSize             = 256
	trafficDuration       = 5 * time.Second
	trafficTimeout        = 10 * time.Second
	bgpWaitTime           = 1 * time.Minute
	vrf1RT                = "65003:100"
	vrf2RT                = "65002:200"
	IPV4                  = "IPv4"
)

var (
	totalPackets   = pps * uint64(trafficDuration.Seconds())
	advertisedNet1 = fmt.Sprintf("%s/%d", advertisedNet1StartIP, routePrefix)
	advertisedNet2 = fmt.Sprintf("%s/%d", advertisedNet2StartIP, routePrefix)

	dutPort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "Dut port 1",
		IPv4:    "192.1.1.1",
		IPv4Len: 30,
		IPv6:    "2001:DB8::1",
		IPv6Len: 126,
	}

	dutPort2 = attrs.Attributes{
		Name:    "port2",
		Desc:    "Dut port 2",
		IPv4:    "192.1.1.5",
		IPv4Len: 30,
		IPv6:    "2001:DB8::5",
		IPv6Len: 126,
	}

	otgPort1 = attrs.Attributes{
		Desc:    "Otg port 1",
		Name:    "port1",
		MAC:     "00:01:12:00:00:01",
		IPv4:    "192.1.1.2",
		IPv4Len: 30,
		IPv6:    "2001:DB8::2",
		IPv6Len: 126,
	}

	otgPort2 = attrs.Attributes{
		Desc:    "Otg port 2",
		Name:    "port2",
		MAC:     "00:01:12:00:00:02",
		IPv4:    "192.1.1.6",
		IPv4Len: 30,
		IPv6:    "2001:DB8::6",
		IPv6Len: 126,
	}
)

type testCase struct {
	name               string
	expectTrafficPass  bool
	expectLeakedRoutes bool
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestRouteLeakageBetweenNonDefaultVRFs(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	tc1 := testCase{
		name:               "TE-6.3.1",
		expectTrafficPass:  false,
		expectLeakedRoutes: false,
	}
	tc2 := testCase{
		name:               "TE-6.3.2",
		expectTrafficPass:  true,
		expectLeakedRoutes: true,
	}
	enableDefaultVRFBGP(t, dut)
	configureDUT(t, dut)
	config := configureATE(t, ate)

	t.Run(tc1.name, func(t *testing.T) {
		runTest(t, tc1, dut, ate, config)
	})
	configureRouteLeaking(t, dut)

	t.Run(tc2.name, func(t *testing.T) {
		runTest(t, tc2, dut, ate, config)
	})
}

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) {
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	_, ok := gnmi.Watch(t, otg, transmitPath, timeout, func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}).Await(t)

	if !ok {
		t.Errorf("Traffic for flow %s did not stop within the timeout of %d", flowName, timeout)
	} else {
		t.Logf("Traffic for flow %s has stopped", flowName)
	}
}

func runTest(t *testing.T, tc testCase, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config gosnappi.Config) {
	otg := ate.OTG()
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), config, IPV4)
	verifyBGP(t, dut)
	verifyRoutes(t, dut)

	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	for _, flow := range config.Flows().Items() {
		waitForTraffic(t, otg, flow.Name(), trafficTimeout)
	}

	otgutils.LogFlowMetrics(t, otg, config)

	verifyTrafficFlow(t, ate, config, tc.expectTrafficPass)
	verifyLeakedRoutes(t, dut, tc.expectLeakedRoutes)
}

func configureRouteLeaking(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.NetworkInstanceImportExportPolicyOCUnsupported(dut) {
		t.Logf("Configuring route leaking through CLI")
		configureRouteLeakingFromCLI(t, dut)
	} else {
		t.Logf("Configuring route leaking through OC")
		configureRouteLeakingFromOC(t, dut)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	t.Logf("Enabling default VRF BGP")
	enableDefaultVRFBGP(t, dut)

	t.Logf("Configuring VRFs")
	configureVrf(t, dut, "VRF-1", dp1.Name(), dutPort1.IPv4, otgPort1.IPv4, dutAS1, ateAS1)
	configureVrf(t, dut, "VRF-2", dp2.Name(), dutPort2.IPv4, otgPort2.IPv4, dutAS2, ateAS2)

	t.Logf("Configuring Interfaces")
	configureDUTPort(t, dut, &dutPort1, dp1)
	configureDUTPort(t, dut, &dutPort2, dp2)
}

func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, attrs *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	d := gnmi.OC()
	i := attrs.NewOCInterface(p.Name(), dut)
	i.Description = ygot.String(attrs.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	i.GetOrCreateEthernet()
	i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	i4.Enabled = ygot.Bool(true)
	a := i4.GetOrCreateAddress(attrs.IPv4)
	a.PrefixLength = ygot.Uint8(attrs.IPv4Len)

	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	i6.Enabled = ygot.Bool(true)
	a6 := i6.GetOrCreateAddress(attrs.IPv6)
	a6.PrefixLength = ygot.Uint8(attrs.IPv6Len)

	gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), i)
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p)
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()

	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	d1 := otgPort1.AddToOTG(config, p1, &dutPort1)
	d2 := otgPort2.AddToOTG(config, p2, &dutPort2)

	ip1 := d1.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	ip2 := d2.Ethernets().Items()[0].Ipv4Addresses().Items()[0]

	bgp1 := d1.Bgp().SetRouterId(otgPort1.IPv4)
	bgp1Peer := bgp1.Ipv4Interfaces().Add().SetIpv4Name(ip1.Name()).Peers().Add().SetName(fmt.Sprintf("%s.BGP.peer", d1.Name()))
	bgp1Peer.SetPeerAddress(dutPort1.IPv4).SetAsNumber(uint32(ateAS1)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	bgp1Net := bgp1Peer.V4Routes().Add().SetName("vrf1-routes")
	bgp1Net.SetNextHopIpv4Address(ip1.Address())
	bgp1Net.Addresses().Add().SetAddress(advertisedNet1StartIP).SetPrefix(24).SetCount(routeCount)

	bgp2 := d2.Bgp().SetRouterId(otgPort2.IPv4)
	bgp2Peer := bgp2.Ipv4Interfaces().Add().SetIpv4Name(ip2.Name()).Peers().Add().SetName(fmt.Sprintf("%s.BGP.peer", d2.Name()))
	bgp2Peer.SetPeerAddress(dutPort2.IPv4).SetAsNumber(uint32(ateAS2)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)

	bgp2Net := bgp2Peer.V4Routes().Add().SetName("vrf2-routes")
	bgp2Net.SetNextHopIpv4Address(ip2.Address())
	bgp2Net.Addresses().Add().SetAddress(advertisedNet2StartIP).SetPrefix(routePrefix).SetCount(routeCount)

	flow1 := config.Flows().Add().SetName("VRF1_to_VRF2")
	flow1.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.IPv4", d1.Name())}).SetRxNames([]string{fmt.Sprintf("%s.IPv4", d2.Name())})
	flow1.Metrics().SetEnable(true)
	flow1.Rate().SetPps(pps)
	flow1.Size().SetFixed(frameSize)
	flow1.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(uint32(totalPackets)))

	flow1.Packet().Add().Ethernet()
	v4_1 := flow1.Packet().Add().Ipv4()
	v4_1.Src().SetValue(advertisedNet1StartIP)
	v4_1.Dst().SetValue(advertisedNet2StartIP)

	flow2 := config.Flows().Add().SetName("VRF2_to_VRF1")
	flow2.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.IPv4", d2.Name())}).SetRxNames([]string{fmt.Sprintf("%s.IPv4", d1.Name())})
	flow2.Metrics().SetEnable(true)
	flow2.Rate().SetPps(pps)
	flow2.Size().SetFixed(frameSize)
	flow2.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(uint32(totalPackets)))

	flow2.Packet().Add().Ethernet()
	v4_2 := flow2.Packet().Add().Ipv4()
	v4_2.Src().SetValue(advertisedNet2StartIP)
	v4_2.Dst().SetValue(advertisedNet1StartIP)

	return config
}

func verifyBGP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	bgpPath1 := gnmi.OC().NetworkInstance(vrf1Name).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	bgpPath2 := gnmi.OC().NetworkInstance(vrf2Name).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	t.Logf("Waiting for BGP session to be ESTABLISHED in %s", vrf1Name)
	status1, ok := gnmi.Watch(t, dut, bgpPath1.Neighbor(otgPort1.IPv4).SessionState().State(), bgpWaitTime, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := val.Val()
		return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP VRF-1 state", bgpPath1.Neighbor(otgPort1.IPv4).State(), gnmi.Get(t, dut, bgpPath1.Neighbor(otgPort1.IPv4).State()))
		t.Fatalf("BGP in %s did not establish: %v", vrf1Name, status1)
	}

	t.Logf("Waiting for BGP session to be ESTABLISHED in %s", vrf2Name)
	status2, ok := gnmi.Watch(t, dut, bgpPath2.Neighbor(otgPort2.IPv4).SessionState().State(), bgpWaitTime, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, present := val.Val()
		return present && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP VRF-2 state", bgpPath2.Neighbor(otgPort2.IPv4).State(), gnmi.Get(t, dut, bgpPath2.Neighbor(otgPort2.IPv4).State()))
		t.Fatalf("BGP in %s did not establish: %v", vrf2Name, status2)
	}
}

// verifyRoutes checks that the advertised routes are installed in the correct VRF AFT.
func verifyRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	aftVrf1 := gnmi.OC().NetworkInstance(vrf1Name).Afts().Ipv4Entry(advertisedNet1)
	_, ok := gnmi.Watch(t, dut, aftVrf1.State(), 15*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		return val.IsPresent()
	}).Await(t)
	if !ok {
		t.Errorf("Route %s is not installed in AFT for %s", advertisedNet1, vrf1Name)
	} else {
		t.Logf("Route %s successfully installed in AFT for %s", advertisedNet1, vrf1Name)
	}

	aftVrf2 := gnmi.OC().NetworkInstance(vrf2Name).Afts().Ipv4Entry(advertisedNet2)
	_, ok = gnmi.Watch(t, dut, aftVrf2.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		return val.IsPresent()
	}).Await(t)
	if !ok {
		t.Errorf("Route %s is not installed in AFT for %s", advertisedNet2, vrf2Name)
	} else {
		t.Logf("Route %s successfully installed in AFT for %s", advertisedNet2, vrf2Name)
	}
}

func configureRouteLeakingFromCLI(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	cli := `
	route-map RM-ALL-ROUTES permit 10
	router general
	   vrf VRF-1
	      leak routes source-vrf VRF-2 subscribe-policy RM-ALL-ROUTES
	   vrf VRF-2
	      leak routes source-vrf VRF-1 subscribe-policy RM-ALL-ROUTES
	`
	helpers.GnmiCLIConfig(t, dut, cli)
}

func configureRouteLeakingFromOC(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	root := &oc.Root{}

	ni1 := root.GetOrCreateNetworkInstance(vrf1Name)
	ni1Pol := ni1.GetOrCreateInterInstancePolicies()
	iexp1 := ni1Pol.GetOrCreateImportExportPolicy()
	iexp1.SetImportRouteTarget([]oc.NetworkInstance_InterInstancePolicies_ImportExportPolicy_ImportRouteTarget_Union{oc.UnionString(vrf2RT)})
	iexp1.SetExportRouteTarget([]oc.NetworkInstance_InterInstancePolicies_ImportExportPolicy_ExportRouteTarget_Union{oc.UnionString(vrf1RT)})
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf1Name).InterInstancePolicies().Config(), ni1Pol)

	ni2 := root.GetOrCreateNetworkInstance(vrf2Name)
	ni2Pol := ni2.GetOrCreateInterInstancePolicies()
	iexp2 := ni2Pol.GetOrCreateImportExportPolicy()
	iexp2.SetImportRouteTarget([]oc.NetworkInstance_InterInstancePolicies_ImportExportPolicy_ImportRouteTarget_Union{oc.UnionString(vrf1RT)})
	iexp2.SetExportRouteTarget([]oc.NetworkInstance_InterInstancePolicies_ImportExportPolicy_ExportRouteTarget_Union{oc.UnionString(vrf2RT)})
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf2Name).InterInstancePolicies().Config(), ni2Pol)
}

func verifyLeakedRoutes(t *testing.T, dut *ondatra.DUTDevice, expectLeakedRoutes bool) {
	t.Helper()
	t.Logf("Verifying leaked route %s in %s", advertisedNet1, vrf2Name)
	aftVrf2 := gnmi.OC().NetworkInstance(vrf2Name).Afts().Ipv4Entry(advertisedNet1)
	_, ok := gnmi.Watch(t, dut, aftVrf2.State(), 15*time.Second, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		return val.IsPresent()
	}).Await(t)
	if !ok {
		if expectLeakedRoutes {
			t.Errorf("Route %s was not leaked into %s unexpectedly", advertisedNet1, vrf2Name)
		} else {
			t.Logf("Route %s was not leaked into %s as expected", advertisedNet1, vrf2Name)
		}
	} else {
		if expectLeakedRoutes {
			t.Logf("Route %s was successfully leaked into %s as expected", advertisedNet1, vrf2Name)
		} else {
			t.Errorf("Route %s was leaked into %s unexpectedly", advertisedNet1, vrf2Name)
		}
	}

	t.Logf("Verifying leaked route %s in %s", advertisedNet2, vrf1Name)
	aftVrf1 := gnmi.OC().NetworkInstance(vrf1Name).Afts().Ipv4Entry(advertisedNet2)
	_, ok = gnmi.Watch(t, dut, aftVrf1.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		return val.IsPresent()
	}).Await(t)
	if !ok {
		if expectLeakedRoutes {
			t.Errorf("Route %s was not leaked into %s unexpectedly", advertisedNet2, vrf1Name)
		} else {
			t.Logf("Route %s was not leaked into %s as expected", advertisedNet2, vrf1Name)
		}
	} else {
		if expectLeakedRoutes {
			t.Logf("Route %s was successfully leaked into %s as expected", advertisedNet2, vrf1Name)
		} else {
			t.Errorf("Route %s was leaked into %s unexpectedly", advertisedNet2, vrf1Name)
		}
	}
}

func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, expectTrafficPass bool) {
	t.Helper()
	otg := ate.OTG()

	for _, flow := range config.Flows().Items() {
		flowMetrics := gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).State())
		txPackets := flowMetrics.GetCounters().GetOutPkts()
		rxPackets := flowMetrics.GetCounters().GetInPkts()

		if txPackets == 0 {
			t.Fatalf("Flow %s did not send any packets", flow.Name())
		}

		if txPackets < totalPackets {
			t.Errorf("Flow %s sent fewer packets than expected: sent %d, expected %d", flow.Name(), txPackets, totalPackets)
		}

		var expectedPackets uint64
		if expectTrafficPass {
			expectedPackets = totalPackets
		} else {
			expectedPackets = 0
		}

		t.Logf("Expecting %d packets for flow %s", expectedPackets, flow.Name())
		msg := fmt.Sprintf("Sent %d packets, expected %d packets, received %d packets.", txPackets, expectedPackets, rxPackets)
		if rxPackets == expectedPackets {
			t.Log(msg)
		} else {
			t.Error(msg)
		}
	}
}

func enableDefaultVRFBGP(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	bgp := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Name:       ygot.String("BGP"),
		Enabled:    ygot.Bool(true),
		Bgp:        &oc.NetworkInstance_Protocol_Bgp{},
	}
	bgp.Bgp.Global = &oc.NetworkInstance_Protocol_Bgp_Global{
		As:       ygot.Uint32(globalAS),
		RouterId: ygot.String("1.1.1.1"),
	}
	gnmi.Replace(t, dut, d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), bgp)
}

func configureVrf(t *testing.T, dut *ondatra.DUTDevice, vrfName, interfaceName, routerId, peerAddress string, routerAS, peerAS uint32) {
	t.Helper()
	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(vrfName)
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	niIntf := ni.GetOrCreateInterface(interfaceName)
	niIntf.Interface = ygot.String(interfaceName)
	niIntf.Subinterface = ygot.Uint32(0)

	proto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := proto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(routerAS)
	global.RouterId = ygot.String(routerId)

	neighbor := bgp.GetOrCreateNeighbor(peerAddress)
	neighbor.PeerAs = ygot.Uint32(peerAS)
	neighbor.Enabled = ygot.Bool(true)
	neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	afiSafi := global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	afiSafi.Enabled = ygot.Bool(true)

	neighbor.Enabled = ygot.Bool(true)
	neighbor.SendCommunityType = []oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_NONE}

	nAfiSafi := neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	nAfiSafi.Enabled = ygot.Bool(true)
	nAfiSafi.GetOrCreateAddPaths().Receive = ygot.Bool(true)
	nAfiSafi.GetOrCreateAddPaths().Send = ygot.Bool(true)
	nAfiSafi.GetOrCreateIpv6Unicast().SendDefaultRoute = ygot.Bool(true)

	nAfiSafi4 := neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	nAfiSafi4.Enabled = ygot.Bool(true)
	nAfiSafi4.GetOrCreateAddPaths().Receive = ygot.Bool(true)
	nAfiSafi4.GetOrCreateAddPaths().Send = ygot.Bool(true)
	nAfiSafi4.GetOrCreateIpv4Unicast().SendDefaultRoute = ygot.Bool(true)

	neighbor.GetOrCreateApplyPolicy().DefaultExportPolicy = oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE
	neighbor.GetOrCreateApplyPolicy().DefaultImportPolicy = oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), ni)
}
