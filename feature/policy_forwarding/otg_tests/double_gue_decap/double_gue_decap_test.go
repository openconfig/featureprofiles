package doubleguedecap_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
)

const (
	plenIPv4 = 30
	plenIPv6 = 126

	aggregateV4Prefix = 28
	aggregateV6Prefix = 60

	// GUE address
	decapOuterName   = "decapHeader1"
	decapOuterIp     = "2001:db8:50::1"
	decapInnerNameV4 = "decapHeader2V4"
	decapInnerv4     = "198.51.100.0"
	decapInnerNameV6 = "decapHeader2V6"
	decapInnerv6     = "2001:db8:50::"

	// Other mid/inner v4/v6 addresses
	midSrcIPv4           = "198.51.200.1"
	midSrcIPv6           = "2001:db8:2::1"
	srcHostv4            = "203.0.113.1"
	srcHostv6            = "2001:db8:3::1"
	dstHostv4            = "203.0.113.1"
	dstHostv6            = "2001:db8:3::100"
	dstHostv4Unreachable = "203.0.113.200"
	dstHostv6Unreachable = "2001:db8:3::200"

	staticDstHostIp   = "203.0.113.0/32"
	staticDstHostIpv6 = "2001:db8:3::/128"

	// Random Source Address
	frameSize          = 512
	packetCount        = 6000
	gueProtocolPort    = 6080
	gueSrcProtocolPort = 5000

	tolerancePct = 0.5
)

type trafficFlow struct {
	flows        otgconfighelpers.Flow
	middleParams otgconfighelpers.Flow
	innerParams  otgconfighelpers.Flow
}

type testCase struct {
	name            string
	flow            trafficFlow
	decapValidation *packetvalidationhelpers.PacketValidation
	testFunc        func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, tc testCase)
}

var (
	dutP1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateP1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	dutP2 = attrs.Attributes{
		Name:    "port2",
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	ateP2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:02:02:02",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	dutPort3Lag = attrs.Attributes{
		Desc:    "description Recirculation-Aggregate-PortChannel",
		IPv4:    "198.51.100.1",
		IPv6:    "2001:db8:50::1",
		IPv4Len: aggregateV4Prefix,
		IPv6Len: aggregateV6Prefix,
	}

	activity = oc.Lacp_LacpActivityType_ACTIVE
	period   = oc.Lacp_LacpPeriodType_FAST

	lacpParams = &cfgplugins.LACPParams{
		Activity: &activity,
		Period:   &period,
	}

	dutLagData = []*cfgplugins.DUTAggData{
		{
			Attributes: dutPort3Lag,
			LacpParams: lacpParams,
			AggType:    oc.IfAggregate_AggregationType_STATIC,
		},
	}

	otgPorts = map[string]*attrs.Attributes{
		"port1": &ateP1,
		"port2": &ateP2,
	}

	dutPorts = map[string]*attrs.Attributes{
		"port1": &dutP1,
		"port2": &dutP2,
	}

	flowValidation = &otgvalidationhelpers.OTGValidation{
		Flow: &otgvalidationhelpers.FlowParams{TolerancePct: tolerancePct},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func mustConfigureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	// Configure Network instance type on DUT
	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	d := gnmi.OC()

	for portName, port := range dutPorts {
		p := dut.Port(t, portName)
		gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), port.NewOCInterface(p.Name(), dut))
	}

	for _, l := range dutLagData {
		b := &gnmi.SetBatch{}
		// Create LAG interface
		l.LagName = netutil.NextAggregateInterface(t, dut)
		cfgplugins.NewAggregateInterface(t, dut, b, l)
		b.Set(t, dut)
	}

	// Configure GUE Decap
	// Header1 - outer gue decap
	ocPFParams := cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		AppliedPolicyName:   decapOuterName,
		TunnelIP:            decapOuterIp,
		GUEPort:             gueProtocolPort,
		IPType:              "ip",
		Dynamic:             true,
		InterfaceID:         dut.Port(t, "port1").Name(),
	}
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)

	// Header2 - inner v4 gue decap
	ocPFParams = cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		AppliedPolicyName:   decapInnerNameV4,
		TunnelIP:            decapInnerv4,
		GUEPort:             gueProtocolPort,
		IPType:              "ip",
		Dynamic:             true,
		InterfaceID:         dutLagData[0].LagName,
	}
	_, _, pf = cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)

	// Header2 - inner v6 gue decap
	ocPFParams = cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		AppliedPolicyName:   decapInnerNameV6,
		TunnelIP:            decapInnerv6,
		GUEPort:             gueProtocolPort,
		IPType:              "ip",
		Dynamic:             true,
		InterfaceID:         dutLagData[0].LagName,
	}
	_, _, pf = cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)

	t.Log("Configuring Static Routes")

	// Configuring Static Route
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticDstHostIp,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(ateP2.IPv4),
		},
		NexthopGroup: false,
		T:            t,
	}

	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)

	// Configuring Static Route for v6
	sV6 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticDstHostIpv6,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(ateP2.IPv6),
		},
		NexthopGroup: false,
		T:            t,
	}

	if _, err := cfgplugins.NewStaticRouteCfg(b, sV6, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)
}

func configureOTG(t *testing.T, otg *ondatra.ATEDevice) gosnappi.Config {
	otgConfig := gosnappi.NewConfig()

	for portName, portAttrs := range otgPorts {
		port := otg.Port(t, portName)
		dutPort := dutPorts[portName]
		portAttrs.AddToOTG(otgConfig, port, dutPort)
	}

	return otgConfig
}

func createflow(top gosnappi.Config, params *otgconfighelpers.Flow, paramsMiddle *otgconfighelpers.Flow, paramsInner *otgconfighelpers.Flow) {
	top.Flows().Clear()
	params.CreateFlow(top)

	params.AddEthHeader()

	if params.VLANFlow != nil {
		params.AddVLANHeader()
	}

	if params.IPv4Flow != nil {
		params.AddIPv4Header()
	}

	if params.IPv6Flow != nil {
		params.AddIPv6Header()
	}

	if params.UDPFlow != nil {
		params.AddUDPHeader()
	}

	if paramsMiddle != nil {
		if paramsMiddle.IPv4Flow != nil {
			params.IPv4Flow = paramsMiddle.IPv4Flow
			params.AddIPv4Header()
		}

		if paramsMiddle.IPv6Flow != nil {
			params.IPv6Flow = paramsMiddle.IPv6Flow
			params.AddIPv6Header()
		}

		if paramsMiddle.UDPFlow != nil {
			params.UDPFlow = paramsMiddle.UDPFlow
			params.AddUDPHeader()
		}
	}

	if paramsInner != nil {
		if paramsInner.IPv4Flow != nil {
			params.IPv4Flow = paramsInner.IPv4Flow
			params.AddIPv4Header()
		}

		if paramsInner.IPv6Flow != nil {
			params.IPv6Flow = paramsInner.IPv6Flow
			params.AddIPv6Header()
		}
	}
}

func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice) {
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(60 * time.Second)
	ate.OTG().StopTraffic(t)
	time.Sleep(60 * time.Second)
	packetvalidationhelpers.StopCapture(t, ate, cs)
}

func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, flowName string, trafficloss bool) {
	flowValidation.Flow.Name = flowName
	if err := flowValidation.ValidateLossOnFlows(t, ate); err != nil {
		if trafficloss {
			t.Logf("traffic loss seen as expected")
		}
		t.Errorf("validation on flows failed: %q", err)
	}
}

func validatePacket(t *testing.T, ate *ondatra.ATEDevice, validationPacket *packetvalidationhelpers.PacketValidation) error {
	err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, validationPacket)
	return err
}

func TestDoubleGueDecap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	mustConfigureDUT(t, dut)
	port1DstMac := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())

	otgConfig := configureOTG(t, ate)
	t.Log(otgConfig.Ports().Items())

	testCases := []testCase{
		{
			name: "Double GUE Decapsulation of IPv4 Traffic",
			flow: trafficFlow{
				flows: otgconfighelpers.Flow{
					TxPort:     otgConfig.Ports().Items()[0].Name(),
					RxPorts:    []string{otgConfig.Ports().Items()[1].Name()},
					IsTxRxPort: true,
					FlowName:   "flowType1",
					EthFlow:    &otgconfighelpers.EthFlowParams{SrcMAC: otgPorts["port1"].MAC, DstMAC: port1DstMac},
					IPv6Flow:   &otgconfighelpers.IPv6FlowParams{IPv6Src: ateP1.IPv6, IPv6Dst: decapOuterIp, TrafficClass: 35, HopLimit: 70},
					UDPFlow:    &otgconfighelpers.UDPFlowParams{UDPDstPort: gueProtocolPort, SetUDPCheckSum: true, UDPCheckSum: 0},
				},
				middleParams: otgconfighelpers.Flow{
					IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: midSrcIPv4, IPv4Dst: decapInnerv4, DSCP: 32, TTL: 60},
					UDPFlow:  &otgconfighelpers.UDPFlowParams{UDPDstPort: gueProtocolPort},
				},
				innerParams: otgconfighelpers.Flow{
					IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: srcHostv4, IPv4Dst: dstHostv4, DSCP: 20, TTL: 50},
				},
			},

			decapValidation: &packetvalidationhelpers.PacketValidation{
				PortName:    "port2",
				CaptureName: "decapCapture",
				Validations: []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateIPv4Header},
				IPv4Layer: &packetvalidationhelpers.IPv4Layer{
					Protocol: 17,
					TTL:      69,
					Tos:      32,
					DstIP:    srcHostv4,
				},
			},
			testFunc: testgueV4Decap,
		},
		{
			name: "Double GUE Decapsulation with IPv6 Inner Payload",
			flow: trafficFlow{
				flows: otgconfighelpers.Flow{
					TxPort:     otgConfig.Ports().Items()[0].Name(),
					RxPorts:    []string{otgConfig.Ports().Items()[1].Name()},
					IsTxRxPort: true,
					FlowName:   "flowType2",
					EthFlow:    &otgconfighelpers.EthFlowParams{SrcMAC: otgPorts["port1"].MAC, DstMAC: port1DstMac},
					IPv6Flow:   &otgconfighelpers.IPv6FlowParams{IPv6Src: ateP1.IPv6, IPv6Dst: decapOuterIp, TrafficClass: 35, HopLimit: 70},
					UDPFlow:    &otgconfighelpers.UDPFlowParams{UDPDstPort: gueProtocolPort, SetUDPCheckSum: true, UDPCheckSum: 0},
				},
				middleParams: otgconfighelpers.Flow{
					IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: midSrcIPv6, IPv6Dst: decapInnerv6, TrafficClass: 32, HopLimit: 60},
					UDPFlow:  &otgconfighelpers.UDPFlowParams{UDPDstPort: gueProtocolPort},
				},
				innerParams: otgconfighelpers.Flow{
					IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: srcHostv6, IPv6Dst: dstHostv6, TrafficClass: 20, HopLimit: 50},
				},
			},

			decapValidation: &packetvalidationhelpers.PacketValidation{
				PortName:    "port2",
				CaptureName: "decapCapture",
				Validations: []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateIPv6Header},
				IPv6Layer: &packetvalidationhelpers.IPv6Layer{
					NextHeader:   17,
					HopLimit:     69,
					TrafficClass: 32,
					DstIP:        srcHostv6,
				},
			},
			testFunc: testgueV6Decap,
		},
		{
			name: "Negative - Middle Header UDP Port Mismatch",
			flow: trafficFlow{
				flows: otgconfighelpers.Flow{
					TxPort:     otgConfig.Ports().Items()[0].Name(),
					RxPorts:    []string{otgConfig.Ports().Items()[1].Name()},
					IsTxRxPort: true,
					FlowName:   "flowType3",
					EthFlow:    &otgconfighelpers.EthFlowParams{SrcMAC: otgPorts["port1"].MAC, DstMAC: port1DstMac},
					IPv6Flow:   &otgconfighelpers.IPv6FlowParams{IPv6Src: ateP1.IPv6, IPv6Dst: decapOuterIp, TrafficClass: 35, HopLimit: 70},
					UDPFlow:    &otgconfighelpers.UDPFlowParams{UDPDstPort: gueProtocolPort, SetUDPCheckSum: true, UDPCheckSum: 0},
				},
				middleParams: otgconfighelpers.Flow{
					IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: midSrcIPv4, IPv4Dst: decapInnerv4, DSCP: 32, TTL: 60},
					UDPFlow:  &otgconfighelpers.UDPFlowParams{UDPDstPort: 6085},
				},
				innerParams: otgconfighelpers.Flow{
					IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: srcHostv4, IPv4Dst: dstHostv4, DSCP: 20, TTL: 50},
				},
			},
			testFunc: testgueUDPMismatch,
		},
		{
			name: "Negative - Middle Header no IPv4 destination available",
			flow: trafficFlow{
				flows: otgconfighelpers.Flow{
					TxPort:     otgConfig.Ports().Items()[0].Name(),
					RxPorts:    []string{otgConfig.Ports().Items()[1].Name()},
					IsTxRxPort: true,
					FlowName:   "flowType1",
					EthFlow:    &otgconfighelpers.EthFlowParams{SrcMAC: otgPorts["port1"].MAC, DstMAC: port1DstMac},
					IPv6Flow:   &otgconfighelpers.IPv6FlowParams{IPv6Src: ateP1.IPv6, IPv6Dst: decapOuterIp, TrafficClass: 35, HopLimit: 70},
					UDPFlow:    &otgconfighelpers.UDPFlowParams{UDPDstPort: gueProtocolPort, SetUDPCheckSum: true, UDPCheckSum: 0},
				},
				middleParams: otgconfighelpers.Flow{
					IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: midSrcIPv4, IPv4Dst: decapInnerv4, DSCP: 32, TTL: 60},
					UDPFlow:  &otgconfighelpers.UDPFlowParams{UDPDstPort: gueProtocolPort},
				},
				innerParams: otgconfighelpers.Flow{
					IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: srcHostv4, IPv4Dst: dstHostv4Unreachable, DSCP: 20, TTL: 50},
				},
			},
			testFunc: testgueV4DstUnreachable,
		},
		{
			name: "Negative - Middle Header no IPv6 destination available",
			flow: trafficFlow{
				flows: otgconfighelpers.Flow{
					TxPort:     otgConfig.Ports().Items()[0].Name(),
					RxPorts:    []string{otgConfig.Ports().Items()[1].Name()},
					IsTxRxPort: true,
					FlowName:   "flowType2",
					EthFlow:    &otgconfighelpers.EthFlowParams{SrcMAC: otgPorts["port1"].MAC, DstMAC: port1DstMac},
					IPv6Flow:   &otgconfighelpers.IPv6FlowParams{IPv6Src: ateP1.IPv6, IPv6Dst: decapOuterIp, TrafficClass: 35, HopLimit: 70},
					UDPFlow:    &otgconfighelpers.UDPFlowParams{UDPDstPort: gueProtocolPort, SetUDPCheckSum: true, UDPCheckSum: 0},
				},
				middleParams: otgconfighelpers.Flow{
					IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: midSrcIPv6, IPv6Dst: decapInnerv6, TrafficClass: 32, HopLimit: 60},
					UDPFlow:  &otgconfighelpers.UDPFlowParams{UDPDstPort: gueProtocolPort},
				},
				innerParams: otgconfighelpers.Flow{
					IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: srcHostv6, IPv6Dst: dstHostv6Unreachable, TrafficClass: 20, HopLimit: 50},
				},
			},
			testFunc: testgueV6DstUnreachable,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Description: %s", tc.name)
			tc.testFunc(t, dut, ate, ate.OTG(), otgConfig, tc)
		})
	}
}

func testgueV4Decap(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgObj *otg.OTG, otgConfig gosnappi.Config, tc testCase) {
	t.Helper()
	packetvalidationhelpers.ConfigurePacketCapture(t, otgConfig, tc.decapValidation)

	createflow(otgConfig, &tc.flow.flows, &tc.flow.middleParams, &tc.flow.innerParams)
	otgObj.PushConfig(t, otgConfig)

	otgObj.StartProtocols(t)

	otgutils.WaitForARP(t, otgObj, otgConfig, "ipv4")
	otgutils.WaitForARP(t, otgObj, otgConfig, "ipv6")

	sendTrafficCapture(t, ate)

	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	verifyTrafficFlow(t, ate, tc.flow.flows.FlowName, false)

	if err := validatePacket(t, ate, tc.decapValidation); err != nil {
		t.Errorf("capture and validatePackets failed (): %q", err)
	} else {
		t.Log("GUE decapsulated packets are received")
	}
}

func testgueV6Decap(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgObj *otg.OTG, otgConfig gosnappi.Config, tc testCase) {
	t.Helper()
	createflow(otgConfig, &tc.flow.flows, &tc.flow.middleParams, &tc.flow.innerParams)
	otgObj.PushConfig(t, otgConfig)
	otgObj.StartProtocols(t)

	sendTrafficCapture(t, ate)
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	verifyTrafficFlow(t, ate, tc.flow.flows.FlowName, false)

	if err := validatePacket(t, ate, tc.decapValidation); err != nil {
		t.Errorf("capture and validatePackets failed (): %q", err)
	} else {
		t.Log("GUE decapsulated packets are received")
	}
}

func testgueUDPMismatch(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgObj *otg.OTG, otgConfig gosnappi.Config, tc testCase) {
	t.Helper()
	createflow(otgConfig, &tc.flow.flows, &tc.flow.middleParams, &tc.flow.innerParams)
	otgObj.PushConfig(t, otgConfig)
	otgObj.StartProtocols(t)

	sendTrafficCapture(t, ate)
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	verifyTrafficFlow(t, ate, tc.flow.flows.FlowName, true)
}

func testgueV4DstUnreachable(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgObj *otg.OTG, otgConfig gosnappi.Config, tc testCase) {
	t.Helper()
	createflow(otgConfig, &tc.flow.flows, &tc.flow.middleParams, &tc.flow.innerParams)
	otgObj.PushConfig(t, otgConfig)
	otgObj.StartProtocols(t)

	sendTrafficCapture(t, ate)
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	verifyTrafficFlow(t, ate, tc.flow.flows.FlowName, true)
}

func testgueV6DstUnreachable(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otgObj *otg.OTG, otgConfig gosnappi.Config, tc testCase) {
	t.Helper()
	createflow(otgConfig, &tc.flow.flows, &tc.flow.middleParams, &tc.flow.innerParams)
	otgObj.PushConfig(t, otgConfig)
	otgObj.StartProtocols(t)

	sendTrafficCapture(t, ate)
	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	verifyTrafficFlow(t, ate, tc.flow.flows.FlowName, true)
}
