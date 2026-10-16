package ingress_police_two_rate_three_color_with_classifier_test

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
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/functional-translators/registrar"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	trafficRateLowMbps  = 1500
	trafficRateHighMbps = 4000
	burstSize           = 100000
	cirValue            = 1000000000
	pirValue            = 2000000000
	trafficFrameSize    = 512
	trafficDuration     = 20 * time.Second
	lossVariation       = 0.05
	schedulerName       = "group_A_2Gb"
	inputPolicerName    = "input-policer-2Gb"
	queue1              = "QUEUE_1"
	queue2              = "QUEUE_2"
	queue3              = "QUEUE_3"
	targetQueue         = queue3
	sequenceNumber      = 1
	targetQueueID       = 2
	classifierType      = oc.Qos_Classifier_Type_IPV4
	inputClassifierType = oc.Input_Classifier_Type_IPV4
	targetGroup         = "TRAFFIC_CLASS_3"
	targetClass         = "class-default"
	port1               = "port1"
	port2               = "port2"
	ipv4                = "IPv4"
	ipv6                = "IPv6"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "Dut port 1",
		Name:    port1,
		IPv4:    "200.0.0.1",
		IPv4Len: 24,
		IPv6:    "2001:f:d:e::1",
		IPv6Len: 126,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "Dut port 2",
		Name:    port2,
		IPv4:    "100.0.0.1",
		IPv4Len: 24,
		IPv6:    "2001:c:d:e::1",
		IPv6Len: 126,
	}
	atePort1 = attrs.Attributes{
		Desc:    "Ate port 1",
		Name:    port1,
		MAC:     "00:01:12:00:00:01",
		IPv4:    "200.0.0.2",
		IPv4Len: 24,
		IPv6:    "2001:f:d:e::2",
		IPv6Len: 126,
	}
	atePort2 = attrs.Attributes{
		Desc:    "Ate port 2",
		Name:    port2,
		MAC:     "00:01:12:00:00:02",
		IPv4:    "100.0.0.2",
		IPv4Len: 24,
		IPv6:    "2001:c:d:e::2",
		IPv6Len: 126,
	}

	inputInterfaceName  string
	totalSentPackets    uint64
	prevExceedingPkts   uint64
	prevExceedingOctets uint64
	prevViolatingPkts   uint64
	prevViolatingOctets uint64
)

type testCase struct {
	name     string
	flowRate uint64
	lossPct  float32
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestIngressPolicerTwoRateThreeColor(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	configureDUT(t, dut)

	top := gosnappi.NewConfig()
	ap1 := ate.Port(t, port1)
	ap2 := ate.Port(t, port2)

	atePort1.AddToOTG(top, ap1, &dutPort1)
	atePort2.AddToOTG(top, ap2, &dutPort2)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, ipv4)

	lowLossPct := float32(0)
	highLossPct := float32(0.5)
	if !deviations.QosTwoRateThreeColorPolicerOCUnsupported(dut) {
		lowLossPct = float32(trafficRateLowMbps-cirValue/1000000) / float32(trafficRateLowMbps)
		highLossPct = float32(trafficRateHighMbps-cirValue/1000000) / float32(trafficRateHighMbps)
	}

	testCases := []testCase{
		{name: "DP-2.6.1 Low Traffic", flowRate: trafficRateLowMbps, lossPct: lowLossPct},
		{name: "DP-2.6.2 High Traffic", flowRate: trafficRateHighMbps, lossPct: highLossPct},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runTest(t, dut, ate, top, tc)
		})
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dp1 := dut.Port(t, port1)
	dp2 := dut.Port(t, port2)
	queues := []string{queue1, queue2, queue3}
	inputInterfaceName = dp1.Name()
	qosBatch := &gnmi.SetBatch{}

	t.Logf("Configuring Hardware Init")
	configureHardwareInit(t, dut)

	t.Logf("Configuring Interfaces")
	configureDUTPort(t, dut, &dutPort1, dp1)
	configureDUTPort(t, dut, &dutPort2, dp2)

	t.Logf("Configuring QOS with Classifier")
	qos := new(oc.Qos)
	cfgplugins.CreateQueues(t, dut, qos, queues)

	fg := cfgplugins.ForwardingGroup{
		Desc:        "Traffic Class 3",
		QueueName:   queue3,
		TargetGroup: targetGroup,
	}
	cfgplugins.NewQoSForwardingGroup(t, dut, qos, []cfgplugins.ForwardingGroup{fg})
	classifier := cfgplugins.QosClassifier{
		Desc:        "Match all IPv4 packets",
		Name:        schedulerName,
		ClassType:   classifierType,
		TermID:      targetClass,
		TargetGroup: targetGroup,
	}

	schedulerParams := &cfgplugins.SchedulerParams{
		SchedulerName:  schedulerName,
		PolicerName:    inputPolicerName,
		InterfaceName:  inputInterfaceName,
		ClassName:      targetClass,
		CirValue:       cirValue,
		PirValue:       pirValue,
		BurstSize:      burstSize,
		QueueName:      targetQueue,
		QueueID:        targetQueueID,
		SequenceNumber: sequenceNumber,
	}

	qosPath := gnmi.OC().Qos().Config()
	cfgplugins.NewQoSClassifierConfiguration(t, dut, qos, []cfgplugins.QosClassifier{classifier})
	qoscfg.SetInputClassifier(t, dut, qos, inputInterfaceName, inputClassifierType, schedulerName)
	if !deviations.QosTwoRateThreeColorPolicerOCUnsupported(dut) {
		gnmi.BatchUpdate(qosBatch, qosPath, qos)
		configureTwoRateThreeColorSchedulerOC(dut, qosBatch, schedulerParams)
		applyQosPolicyOnInterfaceOC(dut, qosBatch, schedulerParams)
		qosBatch.Set(t, dut)
	} else {
		gnmi.BatchUpdate(qosBatch, qosPath, qos)
		qosBatch.Set(t, dut)
		cfgplugins.NewTwoRateThreeColorScheduler(t, dut, qosBatch, schedulerParams)
		cfgplugins.ApplyQosPolicyOnInterface(t, dut, qosBatch, schedulerParams)
	}
}

func configureTwoRateThreeColorSchedulerOC(dut *ondatra.DUTDevice, batch *gnmi.SetBatch, params *cfgplugins.SchedulerParams) {
	qos := &oc.Qos{}
	sp := qos.GetOrCreateSchedulerPolicy(params.SchedulerName)
	sp.Name = ygot.String(params.SchedulerName)
	sched := sp.GetOrCreateScheduler(params.SequenceNumber)
	sched.Sequence = ygot.Uint32(1)
	sched.Type = oc.QosTypes_QOS_SCHEDULER_TYPE_TWO_RATE_THREE_COLOR
	input := sched.GetOrCreateInput(params.PolicerName)
	input.InputType = oc.Input_InputType_QUEUE
	input.Queue = ygot.String(params.QueueName)
	trtc := sched.GetOrCreateTwoRateThreeColor()
	trtc.Cir = ygot.Uint64(params.CirValue)
	trtc.Pir = ygot.Uint64(params.PirValue)
	if dut.Vendor() != ondatra.CISCO {
		trtc.Bc = ygot.Uint32(params.BurstSize)
		trtc.Be = ygot.Uint32(params.BurstSize)
		trtc.GetOrCreateExceedAction().Drop = ygot.Bool(true)
	}
	trtc.GetOrCreateViolateAction().Drop = ygot.Bool(true)
	qosPath := gnmi.OC().Qos().Config()
	gnmi.BatchUpdate(batch, qosPath, qos)
}

func applyQosPolicyOnInterfaceOC(dut *ondatra.DUTDevice, batch *gnmi.SetBatch, params *cfgplugins.SchedulerParams) {
	qos := &oc.Qos{}
	intf := qos.GetOrCreateInterface(params.InterfaceName)
	intf.InterfaceId = ygot.String(params.InterfaceName)
	if !deviations.InterfaceRefConfigUnsupported(dut) {
		intf.GetOrCreateInterfaceRef().Interface = ygot.String(params.InterfaceName)
	}
	in := intf.GetOrCreateInput()
	if params.QueueName != "" {
		in.GetOrCreateQueue(params.QueueName).SetName(params.QueueName)
	}
	in.GetOrCreateSchedulerPolicy().Name = ygot.String(params.SchedulerName)
	qosPath := gnmi.OC().Qos().Config()
	gnmi.BatchUpdate(batch, qosPath, qos)
}

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	features := []cfgplugins.FeatureType{
		cfgplugins.FeatureQOSIn,
	}
	if !deviations.QosGetStatePathUnsupported(dut) {
		features = append([]cfgplugins.FeatureType{cfgplugins.FeatureNGPR}, features...)
	}
	for _, feature := range features {
		hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, feature)
		if hardwareInitCfg == "" {
			continue
		}
		cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
	}
}

func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, attrs *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	d := gnmi.OC()
	i := attrs.NewOCInterface(p.Name(), dut)
	gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), i)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		cfgplugins.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func configureFlows(config *gosnappi.Config, tc testCase, flowName string) {
	(*config).Flows().Clear()
	flow := (*config).Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.%s", port1, ipv4)}).SetRxNames([]string{fmt.Sprintf("%s.%s", port2, ipv4)})
	flow.Size().SetFixed(trafficFrameSize)
	flow.Rate().SetMbps(tc.flowRate)
	flow.Duration().SetFixedSeconds(gosnappi.NewFlowFixedSeconds().SetSeconds(float32(trafficDuration.Seconds())))

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(atePort1.MAC)

	ipv4 := flow.Packet().Add().Ipv4()
	ipv4.Src().SetValue(atePort1.IPv4)
	ipv4.Dst().SetValue(atePort2.IPv4)
}

func runTest(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config gosnappi.Config, tc testCase) {
	abs := func(num int) int {
		if num < 0 {
			return -num
		}
		return num
	}

	otg := ate.OTG()
	flowName := strings.ReplaceAll(tc.name, " ", "_")
	configureFlows(&config, tc, flowName)
	otg.PushConfig(t, config)

	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, config, ipv4)
	otg.StartTraffic(t)
	time.Sleep(trafficDuration + 2*time.Second)
	otg.StopTraffic(t)
	waitForTraffic(t, otg, flowName, trafficDuration*2)
	if !deviations.QosGetStatePathUnsupported(dut) {
		time.Sleep(20 * time.Second)
	}

	otgutils.LogFlowMetrics(t, otg, config)
	otgutils.LogPortMetrics(t, otg, config)

	flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
	sentPackets := *flowMetrics.Counters.OutPkts
	receivedPackets := *flowMetrics.Counters.InPkts

	if sentPackets == 0 {
		t.Errorf("No packets transmitted")
	}

	if receivedPackets == 0 {
		t.Errorf("No packets received")
	}
	lostPackets := abs(int(receivedPackets - sentPackets))
	switch tc.lossPct {
	case 0:
		if lostPackets != 0 {
			t.Errorf("Expected 0 lost packets, but got %d out of %d lost packets", lostPackets, sentPackets)
		}
	default:
		expectedLostPackets := int(float32(sentPackets) * tc.lossPct)
		lostPacketsVariation := int(float64(expectedLostPackets) * lossVariation)
		if lostPackets < expectedLostPackets-lostPacketsVariation || lostPackets > expectedLostPackets+lostPacketsVariation {
			t.Errorf("Expected lost packets to be within [%d, %d], but got %d", expectedLostPackets-lostPacketsVariation, expectedLostPackets+lostPacketsVariation, lostPackets)
		}
	}

	totalSentPackets += sentPackets
	if !deviations.QosGetStatePathUnsupported(dut) {
		validateSchedulerCounters(t, dut, inputInterfaceName, tc, uint64(lostPackets))
		validateClassifierCounters(t, dut, inputInterfaceName, tc, sentPackets, totalSentPackets)
	} else {
		t.Logf("Skipping scheduler and classifier counters validation as QosGetStatePathUnsupported is true")
	}
}

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) {
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	checkState := func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}
	_, ok := gnmi.Watch(t, otg, transmitPath, timeout, checkState).Await(t)

	if !ok {
		t.Errorf("Traffic for flow %s did not stop within the timeout of %d", flowName, timeout)
	} else {
		t.Logf("Traffic for flow %s has stopped", flowName)
	}
}

func validateSchedulerCounters(t *testing.T, dut *ondatra.DUTDevice, intf string, tc testCase, droppedPackets uint64) {
	var opts []ygnmi.Option
	if dut.Vendor() != ondatra.CISCO {
		opts = fptest.GetOptsForFunctionalTranslator(t, deviations.QosFt(dut))
	}
	gnmiOpts := dut.GNMIOpts().WithYGNMIOpts(opts...)
	schedPath := gnmi.OC().Qos().Interface(intf).Input().SchedulerPolicy().Scheduler(sequenceNumber)
	expectedDroppedOctets := droppedPackets * trafficFrameSize

	var (
		conformingPkts, exceedingPkts, violatingPkts       uint64
		conformingOctets, exceedingOctets, violatingOctets uint64
		deltaDroppedPkts, deltaDroppedOctets               uint64
	)

	deadline := time.Now().Add(45 * time.Second)
	for {
		conformingPkts, _ = gnmi.Lookup(t, gnmiOpts, schedPath.ConformingPkts().State()).Val()
		exceedingPkts, _ = gnmi.Lookup(t, gnmiOpts, schedPath.ExceedingPkts().State()).Val()
		violatingPkts, _ = gnmi.Lookup(t, gnmiOpts, schedPath.ViolatingPkts().State()).Val()
		conformingOctets, _ = gnmi.Lookup(t, gnmiOpts, schedPath.ConformingOctets().State()).Val()
		exceedingOctets, _ = gnmi.Lookup(t, gnmiOpts, schedPath.ExceedingOctets().State()).Val()
		violatingOctets, _ = gnmi.Lookup(t, gnmiOpts, schedPath.ViolatingOctets().State()).Val()

		deltaExceedingPkts := exceedingPkts - prevExceedingPkts
		deltaExceedingOctets := exceedingOctets - prevExceedingOctets
		deltaViolatingPkts := violatingPkts - prevViolatingPkts
		deltaViolatingOctets := violatingOctets - prevViolatingOctets

		deltaDroppedPkts = deltaViolatingPkts
		deltaDroppedOctets = deltaViolatingOctets
		if !deviations.QosTwoRateThreeColorPolicerOCUnsupported(dut) {
			deltaDroppedPkts += deltaExceedingPkts
			deltaDroppedOctets += deltaExceedingOctets
		}

		settled := conformingPkts > 0 && conformingOctets > 0 &&
			exceedingPkts > 0 && exceedingOctets > 0 &&
			conformingOctets == conformingPkts*trafficFrameSize &&
			exceedingOctets == exceedingPkts*trafficFrameSize &&
			violatingOctets == violatingPkts*trafficFrameSize &&
			deltaDroppedPkts == droppedPackets &&
			deltaDroppedOctets == expectedDroppedOctets
		if settled || time.Now().After(deadline) {
			break
		}
		time.Sleep(3 * time.Second)
	}

	prevExceedingPkts = exceedingPkts
	prevExceedingOctets = exceedingOctets
	prevViolatingPkts = violatingPkts
	prevViolatingOctets = violatingOctets

	t.Logf("Scheduler counters for %s at %.1fGbps: conformingPkts=%d, conformingOctets=%d, exceedingPkts=%d, exceedingOctets=%d, violatingPkts=%d, violatingOctets=%d",
		intf, float64(tc.flowRate/1000), conformingPkts, conformingOctets, exceedingPkts, exceedingOctets, violatingPkts, violatingOctets)
	if conformingPkts == 0 || conformingOctets == 0 {
		t.Errorf("Expected non-zero conforming packets and octets, got conformingPkts=%d, conformingOctets=%d", conformingPkts, conformingOctets)
	}
	if exceedingPkts == 0 || exceedingOctets == 0 {
		t.Errorf("Expected non-zero exceeding packets and octets, got exceedingPkts=%d, exceedingOctets=%d", exceedingPkts, exceedingOctets)
	}
	if conformingOctets != conformingPkts*trafficFrameSize {
		t.Errorf("Expected conformingOctets (%d) == conformingPkts*frameSize (%d)", conformingOctets, conformingPkts*trafficFrameSize)
	}
	if exceedingOctets != exceedingPkts*trafficFrameSize {
		t.Errorf("Expected exceedingOctets (%d) == exceedingPkts*frameSize (%d)", exceedingOctets, exceedingPkts*trafficFrameSize)
	}
	if violatingOctets != violatingPkts*trafficFrameSize {
		t.Errorf("Expected violatingOctets (%d) == violatingPkts*frameSize (%d)", violatingOctets, violatingPkts*trafficFrameSize)
	}
	if deltaDroppedPkts != droppedPackets {
		t.Errorf("Expected %d dropped packets, but got %d", droppedPackets, deltaDroppedPkts)
	}
	if deltaDroppedOctets != expectedDroppedOctets {
		t.Errorf("Expected %d dropped octets, but got %d", expectedDroppedOctets, deltaDroppedOctets)
	}
}

type ciscoClassifierFT struct {
	ygnmi.FunctionalTranslator
}

func (c *ciscoClassifierFT) Translate(sr *gnmipb.SubscribeResponse) (*gnmipb.SubscribeResponse, error) {
	out, err := c.FunctionalTranslator.Translate(sr)
	if err != nil || out.GetUpdate() == nil {
		return out, err
	}
	for _, u := range out.GetUpdate().GetUpdate() {
		for _, elem := range u.GetPath().GetElem() {
			if elem.GetName() == "classifier" && elem.GetKey()["type"] == "" {
				elem.GetKey()["type"] = "IPV4"
			}
			if elem.GetName() == "term" && elem.GetKey()["id"] == "default" {
				elem.GetKey()["id"] = targetClass
			}
		}
	}
	return out, nil
}

func qosOptsForClassifier(t *testing.T, dut *ondatra.DUTDevice) []ygnmi.Option {
	if dut.Vendor() == ondatra.ARISTA {
		return nil
	}
	ftName := deviations.QosFt(dut)
	if dut.Vendor() != ondatra.CISCO || ftName == "" {
		return fptest.GetOptsForFunctionalTranslator(t, ftName)
	}
	baseFT, ok := registrar.FunctionalTranslatorRegistry[ftName]
	if !ok {
		t.Fatalf("Functional translator %q is not registered", ftName)
	}
	return []ygnmi.Option{ygnmi.WithFT(&ciscoClassifierFT{FunctionalTranslator: baseFT})}
}

func validateClassifierCounters(t *testing.T, dut *ondatra.DUTDevice, intf string, tc testCase, sentPackets, totalSentPackets uint64) {
	opts := qosOptsForClassifier(t, dut)
	gnmiOpts := dut.GNMIOpts().WithYGNMIOpts(opts...)
	var matchedPkts uint64
	deadline := time.Now().Add(30 * time.Second)
	for {
		matchedPkts, _ = gnmi.Lookup(t, gnmiOpts, gnmi.OC().Qos().Interface(intf).Input().Classifier(inputClassifierType).Term(targetClass).MatchedPackets().State()).Val()
		if matchedPkts == totalSentPackets || (dut.Vendor() == ondatra.ARISTA && matchedPkts == sentPackets) || time.Now().After(deadline) {
			break
		}
		time.Sleep(2 * time.Second)
	}
	t.Logf("Classifier counters for %s at %.1fGbps: matched-packets=%d", intf, float64(tc.flowRate/1000), matchedPkts)
	if matchedPkts != totalSentPackets && !(dut.Vendor() == ondatra.ARISTA && matchedPkts == sentPackets) {
		t.Errorf("Expected %d (or %d) matched packets, but got %d", totalSentPackets, sentPackets, matchedPkts)
	}
}
