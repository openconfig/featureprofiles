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
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	trafficRateLowMbps  = 1500
	trafficRateHighMbps = 4000
	burstSize           = 100000
	cirValue            = 1000000000
	pirValue            = 2000000000
	trafficFrameSize    = 512
	trafficDuration     = 20 * time.Second
	lossVariation       = 0.01
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

	inputInterfaceName string
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
	otgutils.WaitForARP(t, ate.OTG(), top, ipv6)

	testCases := []testCase{
		{name: "DP-2.6.1 Low Traffic", flowRate: trafficRateLowMbps, lossPct: 0},
		{name: "DP-2.6.2 High Traffic", flowRate: trafficRateHighMbps, lossPct: 0.5},
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
	cfgplugins.NewTwoRateThreeColorScheduler(t, dut, qosBatch, schedulerParams)
	cfgplugins.ApplyQosPolicyOnInterface(t, dut, qosBatch, schedulerParams)
	gnmi.BatchUpdate(qosBatch, qosPath, qos)
	qosBatch.Set(t, dut)
}

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureQOSCounters)
	if hardwareInitCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
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
	otg.StartTraffic(t)
	waitForTraffic(t, otg, flowName, trafficDuration*2)

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

	validateSchedulerCounters(t, dut, inputInterfaceName, tc, uint64(lostPackets))
	validateClassifierCounters(t, dut, inputInterfaceName, tc, sentPackets)
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
	scheduler := gnmi.Get(t, dut, gnmi.OC().Qos().Interface(intf).Input().SchedulerPolicy().Scheduler(sequenceNumber).State())
	t.Logf("Scheduler counters for %s at %.1fGbps: conforming=%d, exceeding=%d, violating=%d", intf, float64(tc.flowRate/1000), *scheduler.ConformingPkts, *scheduler.ExceedingPkts, *scheduler.ViolatingPkts)
	if *scheduler.ViolatingPkts != droppedPackets {
		t.Errorf("Expected %d dropped packets, but got %d", droppedPackets, *scheduler.ViolatingPkts)
	}
}

func validateClassifierCounters(t *testing.T, dut *ondatra.DUTDevice, intf string, tc testCase, expectedPackets uint64) {
	classifier := gnmi.Get(t, dut, gnmi.OC().Qos().Interface(intf).Input().Classifier(inputClassifierType).Term(targetClass).State())
	t.Logf("Classifier counters for %s at %.1fGbps: matched-packets=%d", intf, float64(tc.flowRate/1000), *classifier.MatchedPackets)
	if *classifier.MatchedPackets != expectedPackets {
		t.Errorf("Expected %d matched packets, but got %d", expectedPackets, *classifier.MatchedPackets)
	}
}
