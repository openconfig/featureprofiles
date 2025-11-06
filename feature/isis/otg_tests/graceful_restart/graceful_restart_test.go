package graceful_restart_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gnoi"
	"github.com/openconfig/featureprofiles/internal/helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	ipv4PrefixLen       = 30
	ipv6PrefixLen       = 126
	ipv4NetPfxLen       = 24
	ipv6NetPfxLen       = 64
	isisSysID1          = "640000000001"
	isisSysID2          = "640000000002"
	isisAreaAddr        = "49.0001"
	dutSysID            = "1920.0000.2001"
	isisMetric          = 10
	gracefulRestartTime = 30
	restartWait         = 40
	trafficPps          = 100
	trafficFrameSize    = 512
	trafficDuration     = time.Duration(trafficFrameSize / trafficPps)
	lossTolerancePct    = 2
	v4FlowName          = "ipv4_flow"
	v6FlowName          = "ipv6_flow"
	isisInstance        = "DEFAULT"
	isisPort1Device     = "dev1Isis"
	isisPort2Device     = "dev2Isis"
	isisLevel           = 2
	sleepTime           = 10 * time.Second

	controlcardType   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	activeController  = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController = oc.Platform_ComponentRedundantRole_SECONDARY
	maxSwitchoverTime = 900
)

var (
	// DUT port attributes
	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE Port1",
		IPv4:    "192.168.1.1",
		IPv6:    "2001:DB8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE Port2",
		IPv4:    "192.168.1.5",
		IPv6:    "2001:DB8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// ATE port attributes
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.168.1.2",
		IPv6:    "2001:DB8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.168.1.6",
		IPv6:    "2001:DB8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// Advertised networks from ATE port-2
	ipv4Prefix = "192.168.10.0"
	ipv6Prefix = "2024:db8:128:128::"

	port2isis gosnappi.DeviceIsisRouter
)

type isisConfig struct {
	port  string
	level oc.E_Isis_LevelType
}

// TestMain is the entry point for the test suite.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dc := gnmi.OC()

	// Configure interfaces on DUT Port 1
	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, dc.Interface(p1.Name()).Config(), configInterfaceDUT(p1, &dutPort1, dut))

	// Configure interfaces on DUT Port 2
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, dc.Interface(p2.Name()).Config(), configInterfaceDUT(p2, &dutPort2, dut))

	// Configure IS-IS Protocol
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	isisProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)

	isisProtocol.SetEnabled(true)
	isis := isisProtocol.GetOrCreateIsis()

	globalISIS := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.SetInstance(isisInstance)
	}

	// Configure Global ISIS settings
	globalISIS.SetNet([]string{fmt.Sprintf("%s.%s.00", isisAreaAddr, dutSysID)})
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
	level := isis.GetOrCreateLevel(isisLevel)
	level.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	// Configure ISIS enabled flag at level
	if deviations.ISISLevelEnabled(dut) {
		level.SetEnabled(true)
	}

	isisGracefulRestart := globalISIS.GetOrCreateGracefulRestart()
	isisGracefulRestart.SetEnabled(true)
	isisGracefulRestart.SetHelperOnly(true)
	isisGracefulRestart.SetRestartTime(gracefulRestartTime)

	isisConf := []*isisConfig{
		{port: p1.Name(), level: oc.Isis_LevelType_LEVEL_2},
		{port: p2.Name(), level: oc.Isis_LevelType_LEVEL_2},
	}

	for _, isisPort := range isisConf {
		// Configure ISIS on DUT Port 1
		intf := isis.GetOrCreateInterface(isisPort.port)
		intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
		intf.SetEnabled(true)
		// Configure ISIS level at global mode if true else at interface mode
		if deviations.ISISInterfaceLevel1DisableRequired(dut) {
			intf.GetOrCreateLevel(1).SetEnabled(false)
		} else {
			intf.GetOrCreateLevel(2).SetEnabled(true)
		}
		globalISIS.LevelCapability = isisPort.level
		// Configure ISIS enable flag at interface level
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
		intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).SetEnabled(true)
		if deviations.ISISInterfaceAfiUnsupported(dut) {
			intf.Af = nil
		}
	}

	// Push ISIS configuration to DUT
	gnmi.Replace(t, dut, dc.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Config(), isisProtocol)
}

func configInterfaceDUT(p *ondatra.Port, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i := a.NewOCInterface(p.Name(), dut)
	s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.SetEnabled(true)
	}
	i.GetOrCreateSubinterface(0).GetOrCreateIpv6()

	return i
}

func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otgConfig := gosnappi.NewConfig()

	port1 := otgConfig.Ports().Add().SetName("port1")
	port2 := otgConfig.Ports().Add().SetName("port2")

	port1Dev := otgConfig.Devices().Add().SetName(atePort1.Name + ".dev")
	port1Eth := port1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	port1Eth.Connection().SetPortName(port1.Name())
	port1Ipv4 := port1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	port1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	port1Ipv6 := port1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	port1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	// Add IS-IS in ATE port1
	port1isis := port1Dev.Isis().SetSystemId(isisSysID1).SetName(isisPort1Device)

	port1isis.Basic().SetIpv4TeRouterId(atePort1.IPv4)
	port1isis.Basic().SetHostname(port1isis.Name())
	port1isis.Basic().SetEnableWideMetric(true)
	port1isis.Basic().SetLearnedLspFilter(true)

	devIsisport1 := port1isis.Interfaces().Add().SetEthName(port1Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisPort1").SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_1_2).SetMetric(10)

	devIsisport1.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	port2Dev := otgConfig.Devices().Add().SetName(atePort2.Name + ".dev")
	port2Eth := port2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	port2Eth.Connection().SetPortName(port2.Name())
	port2Ipv4 := port2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	port2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	port2Ipv6 := port2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	port2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	// Add IS-IS in ATE port2
	port2isis = port2Dev.Isis().SetSystemId(isisSysID2).SetName(isisPort2Device)

	port2isis.Basic().SetIpv4TeRouterId(atePort2.IPv4)
	port2isis.Basic().SetHostname(port2isis.Name())
	port2isis.Basic().SetEnableWideMetric(true)
	port2isis.Basic().SetLearnedLspFilter(true)
	port2isis.GracefulRestart().SetHelperMode(false)

	devIsisport2 := port2isis.Interfaces().Add().SetEthName(port2Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisPort2").SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_1_2).SetMetric(10)

	devIsisport2.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	// Advertise IPv4 network from ATE Port 2's ISIS router
	isis2Net4 := port2isis.V4Routes().Add().SetName("ipv4-network")
	isis2Net4.SetLinkMetric(isisMetric)
	isis2Net4.Addresses().Add().SetAddress(ipv4Prefix).SetPrefix(ipv4NetPfxLen)

	// Advertise IPv6 network from ATE Port 2's ISIS router
	isis2Net6 := port2isis.V6Routes().Add().SetName("ipv6-network")
	isis2Net6.SetLinkMetric(isisMetric)
	isis2Net6.Addresses().Add().SetAddress(ipv6Prefix).SetPrefix(ipv6NetPfxLen)

	return otgConfig
}

func createTrafficFlows(t *testing.T, otgConfig gosnappi.Config, flowNameV4, flowNameV6 string) {
	t.Helper()
	// IPv4 flow from ATE port-1 to the advertised network on ATE port-2
	v4Flow := otgConfig.Flows().Add().SetName(flowNameV4)
	v4Flow.Metrics().SetEnable(true)
	v4Flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{"ipv4-network"})
	v4Flow.Rate().SetPps(trafficPps)
	v4Flow.Size().SetFixed(trafficFrameSize)
	e1 := v4Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	e1.Dst().Auto()
	v4 := v4Flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().SetValue(ipv4Prefix)

	// IPv6 flow from ATE port-1 to the advertised network on ATE port-2
	v6Flow := otgConfig.Flows().Add().SetName(flowNameV6)
	v6Flow.Metrics().SetEnable(true)
	v6Flow.TxRx().Device().
		SetTxNames([]string{atePort1.Name + ".IPv6"}).
		SetRxNames([]string{"ipv6-network"})
	v6Flow.Rate().SetPps(trafficPps)
	v6Flow.Size().SetFixed(trafficFrameSize)
	e2 := v6Flow.Packet().Add().Ethernet()
	e2.Src().SetValue(atePort1.MAC)
	e2.Dst().Auto()
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(atePort1.IPv6)
	v6.Dst().SetValue(ipv6Prefix)
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
		_, ok := gnmi.WatchAll(t, dut, query, 5*time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
			state, present := val.Val()
			if present && state == oc.Isis_IsisInterfaceAdjState_UP {
				t.Logf("IS-IS state on %v has adjacencies", intfName)
			}
			return true
		}).Await(t)
		if !ok {
			t.Logf("IS-IS state on %v has no adjacencies", intfName)
			t.Fatal("No IS-IS adjacencies reported.")
		}
	}
}

// verifyTraffic checks traffic flow metrics against expected loss.
func verifyTraffic(t *testing.T, otg *otg.OTG, otgConfig gosnappi.Config, expectLoss bool) bool {
	t.Helper()

	fail := false

	otgutils.LogFlowMetrics(t, otg, otgConfig)
	otgutils.LogPortMetrics(t, otg, otgConfig)

	for _, flowName := range []string{v4FlowName, v6FlowName} {
		metrics := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).State())
		txPackets := metrics.GetCounters().GetOutPkts()
		rxPackets := metrics.GetCounters().GetInPkts()

		if txPackets == 0 {
			t.Fatalf("Transmit packets for flow %s was 0, expected > 0", flowName)
		}

		lossPct := (float64(txPackets-rxPackets) / float64(txPackets)) * 100

		switch {
		case expectLoss && lossPct < (100-lossTolerancePct):
			t.Errorf("traffic loss for flow %s was less than expected: got %v, want > %v", flowName, lossPct, 100-lossTolerancePct)
			fail = true

		case expectLoss:
			t.Logf("Traffic loss for flow %s was as expected: %v", flowName, lossPct)

		case lossPct > lossTolerancePct:
			t.Errorf("traffic loss for flow %s was higher than expected: got %v, want < %v", flowName, lossPct, lossTolerancePct)
			fail = true

		default:
			t.Logf("No loss seen for flow %s as expected", flowName)
		}
	}

	return fail
}

func startStopISISRouter(t *testing.T, otg *otg.OTG, routeNames []string, state string) {
	cs := gosnappi.NewControlState()
	route := cs.Protocol().Isis().Routers().SetRouterNames(routeNames)
	switch state {
	case "DOWN":
		route.SetState(gosnappi.StateProtocolIsisRoutersState.DOWN)
	case "UP":
		route.SetState(gosnappi.StateProtocolIsisRoutersState.UP)
	default:
		t.Error("invalid state for action to be performed on ISIS router")
	}

	otg.SetControlState(t, cs)
}

func TestGracefulRestart(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	configureDUT(t, dut)

	otgConfig := configureOTG(t, ate)
	createTrafficFlows(t, otgConfig, v4FlowName, v6FlowName)
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)
	time.Sleep(20 * time.Second)
	verifyISISTelemetry(t, dut, []string{dut.Port(t, "port1").Name(), dut.Port(t, "port2").Name()})

	type testCase struct {
		Name        string
		Description string
		testFunc    func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config)
	}

	testCases := []testCase{
		{
			Name:        "Testcase: ISIS with GR Helper",
			Description: "Validate traffic with GR enabled",
			testFunc:    testGrHelper,
		},
		{
			Name:        "Testcase: ISIS with Controller Card Switchover",
			Description: "Validate traffic with controller card switchover",
			testFunc:    testISISWithControllerCardSwitchOver,
		},
		{
			Name:        "Testcase: Verify traffic with DUT ISIS Restart",
			Description: "Validate traffic with DUT ISIS restart",
			testFunc:    testISISWithDUTRestart,
		},
	}

	// Run the test cases.
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("Description: %s", tc.Description)
			tc.testFunc(t, dut, ate, otgConfig)
		})
	}
}

func testGrHelper(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
	otg := ate.OTG()

	var expectedISISAdj = map[string]interface{}{
		"IsisRouterName":                isisPort2Device,
		"LocalStateTypeExp":             "LEVEL_1_2",
		"LocalStateHoldTimeExp":         uint32(30),
		"LocalStateRestartStatusExp":    "RUNNING",
		"LocalStateLastAttemptExp":      "SUCCEEDED",
		"NeighborStateTypeExp":          "LEVEL_2",
		"NeighborStateHoldTimeExp":      uint32(30),
		"NeighborStateRestartStatusExp": "RUNNING",
		"NeighborStateLastAttemptExp":   "UNAVAILABLE",
	}

	// Validating subtest 1: Ipv4/Ipv6 traffic from ATE port1 to port2
	t.Logf("Subtest-1: Verify traffic from ATE port1 to \"target IPv4\" and \"target IPv6\" networks on ATE port-2")
	otg.StartTraffic(t)
	time.Sleep(sleepTime)
	otg.StopTraffic(t)
	if verifyTraffic(t, otg, otgConfig, false) {
		t.Error("traffic loss for flow is more than expected")
	}

	// Validating subtest 2: Restarting IS-IS on ATE port-2 and verifying traffic is not lost due to GR
	t.Logf("Subtest-2: Restarting IS-IS on ATE port-2 and verifying traffic is not lost due to GR.")
	cs := gosnappi.NewControlAction()
	isisRestart := cs.Protocol().Isis().InitiateGracefulRestart().SetRouterNames([]string{isisPort2Device})
	isisRestart.Unplanned().SetHoldingTime(gracefulRestartTime).SetRestartAfter(uint32(restartWait))
	startTime := time.Now()

	// Initiating graceful restart, waiting for ISIS to come up after GR time expiry and validating traffic
	otg.StartTraffic(t)
	replaceDuration := time.Since(startTime)
	t.Log("Send traffic while GR timer is counting down. Traffic should pass as ISIS GR is enabled!")
	otg.SetControlAction(t, cs)
	sleepTime := gracefulRestartTime*time.Second - replaceDuration
	time.Sleep(sleepTime)
	otg.StopTraffic(t)
	if verifyTraffic(t, otg, otgConfig, false) {
		t.Error("traffic loss for flow is more than expected")
	}

	otgvalidationhelpers.ValidateOTGISISTelemetry(t, ate, expectedISISAdj)

	time.Sleep(sleepTime)
	t.Log("Verify ISIS is up again after GR timeout expiry")
	verifyISISTelemetry(t, dut, []string{dut.Port(t, "port1").Name(), dut.Port(t, "port2").Name()})

	// Initiating graceful restart, validating traffic loss after graceful restart expires before restart time
	t.Logf("Validating traffic loss after after Restart Time expiry")
	otg.SetControlAction(t, cs)

	//The graceful restart timer is set to 30, validating traffic as soon as it expires before it initiate restart
	time.Sleep(29 * time.Second)
	otg.StartTraffic(t)
	time.Sleep(5 * time.Second)
	otg.StopTraffic(t)
	if verifyTraffic(t, otg, otgConfig, true) {
		t.Error("traffic loss is not seen for flow as expected")
	}

	// Validating subtest 3: Disable IS-IS on ATE port-2 and verifying traffic is lost due to GR.
	t.Logf("Subtest-3: Disable IS-IS on ATE port-2 and verifying traffic is lost due to GR.")
	otg.SetControlAction(t, cs)
	startStopISISRouter(t, otg, []string{isisPort2Device}, "DOWN")
	time.Sleep(restartWait * time.Second)
	otg.StartTraffic(t)
	time.Sleep(sleepTime)
	otg.StopTraffic(t)
	if verifyTraffic(t, otg, otgConfig, true) {
		t.Error("traffic loss is not seen for flow as expected")
	}
}

func testISISWithControllerCardSwitchOver(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {

	otg := ate.OTG()

	otg.StartProtocols(t)
	verifyISISTelemetry(t, dut, []string{dut.Port(t, "port1").Name(), dut.Port(t, "port2").Name()})

	otg.StartTraffic(t)
	time.Sleep(sleepTime)
	otg.StopTraffic(t)
	if verifyTraffic(t, otg, otgConfig, false) {
		t.Error("traffic loss for flow is more than expected")
	}

	// TODO: Not able to verify because of HW limitation. Adding the below deviation instead creating new one
	if deviations.GNOISwitchoverReasonMissingUserInitiated(dut) {
		// TODO: Not able to verify because of HW limitation. Adding the below deviation instead creating new one
	} else {
		t.Logf("Initiating controller card switchover")

		controllerCards := components.FindComponentsByType(t, dut, controlcardType)
		t.Logf("Found controller card list: %v", controllerCards)

		if got, want := len(controllerCards), 2; got < want {
			t.Skipf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
		}

		rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyControllerCard(t, dut, controllerCards)
		t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

		switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
		gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
		t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
		if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
			t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
		}

		intfsOperStatusUPBeforeSwitch := helpers.FetchOperStatusUPIntfs(t, dut, *args.CheckInterfacesInBinding)
		t.Logf("intfsOperStatusUP interfaces before switchover: %v", intfsOperStatusUPBeforeSwitch)
		if got, want := len(intfsOperStatusUPBeforeSwitch), 0; got == want {
			t.Errorf("get the number of intfsOperStatusUP interfaces for %q: got %v, want > %v", dut.Name(), got, want)
		}

		gnoiClient := dut.RawAPIs().GNOI(t)
		useNameOnly := deviations.GNOISubcomponentPath(dut)
		switchoverRequest := &spb.SwitchControlProcessorRequest{
			ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
		}
		t.Logf("switchoverRequest: %v", switchoverRequest)
		switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
		if err != nil {
			t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
		}
		t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

		startSwitchover := time.Now()
		t.Logf("Wait for new active RP to boot up by polling the telemetry output.")
		for {
			var currentTime string
			t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
			if !verifyTraffic(t, otg, otgConfig, false) {
				break
			}
			time.Sleep(60 * time.Second)
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
			}); errMsg != nil {
				t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
			} else {
				t.Logf("RP switchover has completed successfully with received time: %v", currentTime)
				break
			}

			if got, want := uint64(time.Since(startSwitchover).Seconds()), uint64(maxSwitchoverTime); got >= want {
				t.Fatalf("time.Since(startSwitchover): got %v, want < %v", got, want)
			}
		}
	}

}

func testISISWithDUTRestart(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {

	otg := ate.OTG()

	var expectedISISAdj = map[string]interface{}{
		"IsisRouterName":                isisPort2Device,
		"LocalStateTypeExp":             "LEVEL_1_2",
		"LocalStateHoldTimeExp":         uint32(30),
		"LocalStateRestartStatusExp":    "RUNNING",
		"LocalStateLastAttemptExp":      "UNAVAILABLE",
		"NeighborStateTypeExp":          "LEVEL_2",
		"NeighborStateHoldTimeExp":      uint32(30),
		"NeighborStateRestartStatusExp": "RUNNING",
		"NeighborStateLastAttemptExp":   "SUCCEEDED",
	}

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()
	gnmi.Update(t, dut, dutConfPath.Global().GracefulRestart().HelperOnly().Config(), false)
	port2isis.GracefulRestart().SetHelperMode(true)
	otg.PushConfig(t, otgConfig)
	otg.StartProtocols(t)
	time.Sleep(20 * time.Second)

	verifyISISTelemetry(t, dut, []string{dut.Port(t, "port1").Name(), dut.Port(t, "port2").Name()})
	otg.StartTraffic(t)
	time.Sleep(sleepTime)
	otg.StopTraffic(t)
	if verifyTraffic(t, otg, otgConfig, false) {
		t.Error("traffic loss for flow is more than expected")
	}

	t.Logf("Initiating Kill Process on DUT")
	gnoi.KillProcess(t, dut, "ISIS", gnoi.SigTerm, true, false)
	startTime := time.Now()
	for {
		otg.StartTraffic(t)
		time.Sleep(5 * time.Second)
		otg.StopTraffic(t)
		if !verifyTraffic(t, otg, otgConfig, false) {
			break
		}

		if uint64(time.Since(startTime).Seconds()) > gracefulRestartTime {
			t.Fatalf("Traffic verification failed. Traffic didn't pass within the graceful restart time : %v sec", gracefulRestartTime)
		}
	}

	otgvalidationhelpers.ValidateOTGISISTelemetry(t, ate, expectedISISAdj)
}
