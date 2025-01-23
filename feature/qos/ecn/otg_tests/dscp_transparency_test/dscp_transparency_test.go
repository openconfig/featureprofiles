// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dscp_transparency_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/entity-naming/entname"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4                           = "IPv4"
	ipv6                           = "IPv6"
	ipv4PrefixLen                  = 30
	ipv6PrefixLen                  = 126
	subInterfaceIndex              = 0
	flowFrameSize           uint32 = 1_000
	trafficRunDuration             = 1 * time.Minute
	trafficStopWaitDuration        = 30 * time.Second
	dutEgressPort                  = "port1"
)

var (
	dutPort1 = &attrs.Attributes{
		Name:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = &attrs.Attributes{
		Name:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort3 = &attrs.Attributes{
		Name:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::9",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = &attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = &attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	atePort3 = &attrs.Attributes{
		Name:    "atePort3",
		MAC:     "02:00:03:01:01:01",
		IPv4:    "192.0.2.10",
		IPv6:    "2001:db8::a",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPorts = map[string]*attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
		"port3": dutPort3,
	}

	atePorts = map[string]*attrs.Attributes{
		"port1": atePort1,
		"port2": atePort2,
		"port3": atePort3,
	}

	allQueueNames = []entname.QoSQueue{
		entname.QoSNC1,
		entname.QoSAF4,
		entname.QoSAF3,
		entname.QoSAF2,
		entname.QoSAF1,
		entname.QoSBE0,
		entname.QoSBE1,
	}

	testCases = []struct {
		name           string
		createFlowsF   func(otgConfig gosnappi.Config, protocol string, atePortSpeed int)
		validateFlowsF func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, atePortSpeed int, startingCounters map[entname.QoSQueue]*queueCounters)
	}{
		{
			name:           "TestNoCongestion",
			createFlowsF:   testNoCongestionCreateFlows,
			validateFlowsF: testNoCongestionValidateFlows,
		},
		{
			name:           "TestCongestion",
			createFlowsF:   testCongestionCreateFlows,
			validateFlowsF: testCongestionValidateFlows,
		},
		{
			name:           "TestNC1Congestion",
			createFlowsF:   testNC1CongestionCreateFlows,
			validateFlowsF: testNC1CongestionValidateFlows,
		},
	}
)

type queueCounters struct {
	droppedPackets  uint64
	transmitPackets uint64
	transmitOctets  uint64
}

func prettyPrint(i any) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func getZeroIshThresholds(dutPortSpeed int) (uint64, uint64) {
	// Max allowed "zero" counters -- counters that are supposed to be zero per the test but
	// can have a few packets trickling about for random things; basically: a fudge factor,
	// proportional to the port speed.
	maxAllowedZeroPackets := uint64(5 * dutPortSpeed)
	maxAllowedZeroOctets := uint64(40 * dutPortSpeed)

	return maxAllowedZeroPackets, maxAllowedZeroOctets
}

func configureDUTQoS(
	t *testing.T,
	dut *ondatra.DUTDevice,
) {
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	dp3 := dut.Port(t, "port3")

	qosConfig := &oc.Qos{}

	if deviations.QOSQueueRequiresID(dut) {
		for i, queueName := range allQueueNames {
			q1 := qosConfig.GetOrCreateQueue(string(queueName))
			q1.Name = ygot.String(string(queueName))
			queueID := len(allQueueNames) - i
			q1.QueueId = ygot.Uint8(uint8(queueID))
		}
	}

	// Forwarding group :: queue config.
	for _, queueName := range allQueueNames {
		qoscfg.SetForwardingGroup(
			t,
			dut,
			qosConfig,
			fmt.Sprintf("target-group-%s", string(queueName)),
			string(queueName),
		)
	}

	// Queue management profile.
	queueManagementProfile := qosConfig.GetOrCreateQueueManagementProfile("queueManagementProfile")
	wredUniformProfile := queueManagementProfile.GetOrCreateWred().GetOrCreateUniform()
	wredUniformProfile.SetEnableEcn(true)
	wredUniformProfile.SetMinThreshold(uint64(80_000))
	wredUniformProfile.SetMaxThreshold(uint64(3_000_000))
	wredUniformProfile.SetMaxDropProbabilityPercent(uint8(100))

	// Classifier config.
	classifiers := []struct {
		name        string
		termID      string
		targetGroup string
		dscpSet     []uint8
	}{
		{
			name:        "dscp_based_classifier_",
			termID:      "0",
			targetGroup: "target-group-BE1",
			dscpSet:     []uint8{0, 1, 2, 3},
		},
		{
			name:        "dscp_based_classifier_",
			termID:      "1",
			targetGroup: "target-group-BE0",
			dscpSet:     []uint8{4, 5, 6, 7},
		},
		{
			name:        "dscp_based_classifier_",
			termID:      "2",
			targetGroup: "target-group-AF1",
			dscpSet:     []uint8{8, 9, 10, 11, 12, 13, 14, 15},
		},
		{
			name:        "dscp_based_classifier_",
			termID:      "3",
			targetGroup: "target-group-AF2",
			dscpSet:     []uint8{16, 17, 18, 19, 20, 21, 22, 23},
		},
		{
			name:        "dscp_based_classifier_",
			termID:      "4",
			targetGroup: "target-group-AF3",
			dscpSet:     []uint8{24, 25, 26, 27, 28, 29, 30, 31},
		},
		{
			name:        "dscp_based_classifier_",
			termID:      "5",
			targetGroup: "target-group-AF4",
			dscpSet:     []uint8{32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47},
		},
		{
			name:        "dscp_based_classifier_",
			termID:      "6",
			targetGroup: "target-group-NC1",
			dscpSet:     []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63},
		},
	}

	for _, tc := range classifiers {
		for _, protocol := range []oc.E_Qos_Classifier_Type{
			oc.Qos_Classifier_Type_IPV4,
			oc.Qos_Classifier_Type_IPV6,
		} {
			protocolString := "ipv4"
			if protocol == oc.Qos_Classifier_Type_IPV6 {
				protocolString = "ipv6"
			}

			name := fmt.Sprintf("%s%s", tc.name, protocolString)
			classifier := qosConfig.GetOrCreateClassifier(name)
			classifier.SetName(name)
			classifier.SetType(protocol)

			term, err := classifier.NewTerm(tc.termID)
			if err != nil {
				t.Fatalf("Failed to create classifier.NewTerm(): %v", err)
			}
			term.SetId(tc.termID)
			action := term.GetOrCreateActions()
			action.SetTargetGroup(tc.targetGroup)
			condition := term.GetOrCreateConditions()

			switch protocol {
			case oc.Qos_Classifier_Type_IPV4:
				condition.GetOrCreateIpv4().SetDscpSet(tc.dscpSet)
			case oc.Qos_Classifier_Type_IPV6:
				condition.GetOrCreateIpv6().SetDscpSet(tc.dscpSet)
			}
		}
	}

	// Ingress classifier config.
	for _, inputInterfaceName := range []string{dp2.Name(), dp3.Name()} {
		for _, protocol := range []oc.E_Input_Classifier_Type{
			oc.Input_Classifier_Type_IPV4,
			oc.Input_Classifier_Type_IPV6,
		} {
			protocolString := "ipv4"
			if protocol == oc.Input_Classifier_Type_IPV6 {
				protocolString = "ipv6"
			}

			qoscfg.SetInputClassifier(
				t,
				dut,
				qosConfig,
				inputInterfaceName,
				protocol,
				fmt.Sprintf("dscp_based_classifier_%s", protocolString),
			)
		}
	}

	// Egress scheduler config.
	schedulerPolicy := qosConfig.GetOrCreateSchedulerPolicy("schedulerPolicy")
	strictScheduler := schedulerPolicy.GetOrCreateScheduler(uint32(0))
	strictScheduler.SetPriority(oc.Scheduler_Priority_STRICT)
	strictInput := strictScheduler.GetOrCreateInput(string(entname.QoSNC1))
	strictInput.SetInputType(oc.Input_InputType_QUEUE)
	strictInput.SetQueue(string(entname.QoSNC1))

	wrrScheduler := schedulerPolicy.GetOrCreateScheduler(uint32(1))

	// WRR queues, equally weighted.
	for _, queueName := range allQueueNames {
		if queueName == entname.QoSNC1 {
			// Skipping NC1 since it's in its own strict scheduler.
			continue
		}
		input := wrrScheduler.GetOrCreateInput(string(queueName))
		input.SetInputType(oc.Input_InputType_QUEUE)
		input.SetQueue(string(queueName))
		input.SetWeight(uint64(10))
	}

	// Egress policy config.
	for _, queueName := range allQueueNames {
		qosInterface := qosConfig.GetOrCreateInterface(dp1.Name())
		qosInterface.GetOrCreateInterfaceRef().Interface = ygot.String(dp1.Name())
		output := qosInterface.GetOrCreateOutput()
		outputSchedulerPolicy := output.GetOrCreateSchedulerPolicy()
		outputSchedulerPolicy.SetName("schedulerPolicy")
		queue := output.GetOrCreateQueue(string(queueName))
		queue.SetQueueManagementProfile("queueManagementProfile")
		if deviations.QOSBufferAllocationConfigRequired(dut) {
			bufferAllocationProfile := qosConfig.GetOrCreateBufferAllocationProfile("bufferAllocationProfile")
			bufferAllocationQueue := bufferAllocationProfile.GetOrCreateQueue(string(queueName))
			bufferAllocationQueue.SetStaticSharedBufferLimit(uint32(268435456))
			output.SetBufferAllocationProfile("bufferAllocationProfile")
		}
	}

	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), qosConfig)
}

func configureDUTPort(
	t *testing.T,
	dut *ondatra.DUTDevice,
	port *ondatra.Port,
	portAttrs *attrs.Attributes,
) {
	gnmiOCRoot := gnmi.OC()

	gnmi.Replace(
		t,
		dut,
		gnmiOCRoot.Interface(port.Name()).Config(),
		portAttrs.NewOCInterface(port.Name(), dut),
	)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(
			t, dut, port.Name(), deviations.DefaultNetworkInstance(dut), subInterfaceIndex,
		)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	for portName, portAttrs := range dutPorts {
		port := dut.Port(t, portName)
		configureDUTPort(t, dut, port, portAttrs)
	}
	configureDUTQoS(t, dut)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otgConfig := gosnappi.NewConfig()
	for portName, portAttrs := range atePorts {
		port := ate.Port(t, portName)
		dutPort := dutPorts[portName]
		portAttrs.AddToOTG(otgConfig, port, dutPort)
	}
	return otgConfig
}

func trafficClassFieldsToDecimal(dscpValue, ecnValue int) uint32 {
	dscpByte := byte(dscpValue)
	ecnByte := byte(ecnValue)
	tosStr := fmt.Sprintf("%06b%02b", dscpByte, ecnByte)
	tosDec, _ := strconv.ParseInt(tosStr, 2, 64)
	return uint32(tosDec)
}

func createFlow(otgConfig gosnappi.Config, protocol string, targetTotalFlowRate uint64, dscpValue int, sourceAtePort *attrs.Attributes) gosnappi.Flow {
	flow := otgConfig.Flows().Add().SetName(fmt.Sprintf("dscp-%d-%s", dscpValue, sourceAtePort.Name))
	flow.Metrics().SetEnable(true)

	// Flows go from ate port 2 -> dut -> ate port 1 and
	// from ate port 3 -> dut -> ate port 1 to be consistent with the previous test which
	// can be run with only two ports instead of three.
	flow.TxRx().Device().
		SetTxNames([]string{fmt.Sprintf("%s.%s", sourceAtePort.Name, protocol)}).
		SetRxNames([]string{fmt.Sprintf("%s.%s", atePort1.Name, protocol)})
	flow.EgressPacket().Add().Ethernet()

	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(sourceAtePort.MAC)

	switch protocol {
	case ipv4:
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(sourceAtePort.IPv4)
		v4.Dst().SetValue(atePort1.IPv4)
		v4.Priority().Raw().SetValue(trafficClassFieldsToDecimal(dscpValue, 2))

		tracking := flow.EgressPacket().Add().Ipv4()
		tracking.Priority().Raw().MetricTags().Add().SetName(fmt.Sprintf("dst-dscp-%d-%s", dscpValue, sourceAtePort.Name)).SetOffset(0).SetLength(6)
		tracking.Priority().Raw().MetricTags().Add().SetName(fmt.Sprintf("dst-ecn-%d-%s", dscpValue, sourceAtePort.Name)).SetOffset(6).SetLength(2)
	case ipv6:
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(sourceAtePort.IPv6)
		v6.Dst().SetValue(atePort1.IPv6)
		v6.TrafficClass().SetValue(trafficClassFieldsToDecimal(dscpValue, 2))

		tracking := flow.EgressPacket().Add().Ipv6()
		tracking.TrafficClass().MetricTags().Add().SetName(fmt.Sprintf("dst-dscp-%d-%s", dscpValue, sourceAtePort.Name)).SetOffset(0).SetLength(6)
		tracking.TrafficClass().MetricTags().Add().SetName(fmt.Sprintf("dst-ecn-%d-%s", dscpValue, sourceAtePort.Name)).SetOffset(6).SetLength(2)
	}

	flow.Size().SetFixed(flowFrameSize)
	flow.Rate().SetKbps(targetTotalFlowRate)
	return flow
}

func getQueueCounters(t *testing.T, dut *ondatra.DUTDevice) map[entname.QoSQueue]*queueCounters {
	t.Helper()
	ep := dut.Port(t, dutEgressPort)
	qc := map[entname.QoSQueue]*queueCounters{}

	for _, egressQueueName := range allQueueNames {
		qc[egressQueueName] = &queueCounters{
			droppedPackets:  gnmi.Get(t, dut, gnmi.OC().Qos().Interface(ep.Name()).Output().Queue(string(egressQueueName)).DroppedPkts().State()),
			transmitPackets: gnmi.Get(t, dut, gnmi.OC().Qos().Interface(ep.Name()).Output().Queue(string(egressQueueName)).TransmitPkts().State()),
			transmitOctets:  gnmi.Get(t, dut, gnmi.OC().Qos().Interface(ep.Name()).Output().Queue(string(egressQueueName)).TransmitOctets().State()),
		}
	}

	return qc
}

func logAndGetResolvedQueueCounters(t *testing.T, egressQueueName entname.QoSQueue, egressQueueStartingCounters, egressQueueEndingCounters *queueCounters) (uint64, uint64, uint64) {
	queueDroppedPackets := egressQueueEndingCounters.droppedPackets - egressQueueStartingCounters.droppedPackets
	queueTransmitPackets := egressQueueEndingCounters.transmitPackets - egressQueueStartingCounters.transmitPackets
	queueTransmitOctets := egressQueueEndingCounters.transmitOctets - egressQueueStartingCounters.transmitOctets

	t.Logf(
		"\nqueue %q pre-test telemetry data:\n\tdropped %d packets\n\ttransmit %d packets\n\ttransmit %d octets\n",
		egressQueueName,
		egressQueueStartingCounters.droppedPackets,
		egressQueueStartingCounters.transmitPackets,
		egressQueueStartingCounters.transmitOctets,
	)

	t.Logf(
		"\nqueue %q post-test telemetry data:\n\tdropped %d packets\n\ttransmit %d packets\n\ttransmit %d octets\n",
		egressQueueName,
		egressQueueEndingCounters.droppedPackets,
		egressQueueEndingCounters.transmitPackets,
		egressQueueEndingCounters.transmitOctets,
	)

	t.Logf(
		"\nqueue %q resolved telemetry data:\n\tdropped %d packets\n\ttransmit %d packets\n\ttransmit %d octets\n",
		egressQueueName,
		queueDroppedPackets,
		queueTransmitPackets,
		queueTransmitOctets,
	)

	return queueDroppedPackets, queueTransmitPackets, queueTransmitOctets
}

func testNoCongestionCreateFlows(otgConfig gosnappi.Config, protocol string, dutPortSpeed int) {
	// Target flow rate is 60% of the ate port speed spread across 64 flows (do this in kbps so we
	// still work w/ round numbers on 1g interfaces).
	portSpeedInKbps := dutPortSpeed * 1_000_000
	portSpeedSixtyPercent := float32(portSpeedInKbps) * float32(0.6)
	targetTotalFlowRate := uint64(portSpeedSixtyPercent / 64)

	for dscpValue := 0; dscpValue < 64; dscpValue++ {
		finalTargetFlowRate := targetTotalFlowRate
		if dscpValue <= 7 {
			// There are fewer flows in the BE0/BE1 queues so increase those flows to have
			// a similar amount of traffic so wred handles things consistently.
			finalTargetFlowRate = targetTotalFlowRate * 2
		}

		createFlow(
			otgConfig,
			protocol,
			finalTargetFlowRate,
			dscpValue,
			atePort2,
		)
	}
}

func testNoCongestionValidateFlows(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, dutPortSpeed int, startingCounters map[entname.QoSQueue]*queueCounters) {
	maxAllowedZeroPackets, _ := getZeroIshThresholds(dutPortSpeed)
	endingCounters := getQueueCounters(t, dut)

	for egressQueueName, egressQueueEndingCounters := range endingCounters {
		egressQueueStartingCounters := startingCounters[egressQueueName]

		queueDroppedPackets, queueTransmitPackets, queueTransmitOctets := logAndGetResolvedQueueCounters(
			t,
			egressQueueName,
			egressQueueStartingCounters,
			egressQueueEndingCounters,
		)

		if queueDroppedPackets > maxAllowedZeroPackets {
			t.Errorf("queue %s indicates %d dropped packets but should show zero or near-zero", egressQueueName, queueDroppedPackets)
		}

		if queueTransmitPackets == 0 {
			t.Errorf("queue %s indicates 0 transmit packets but should be non-zero", egressQueueName)
		}

		if queueTransmitOctets == 0 {
			t.Errorf("queue %s indicates 0 transmit octets but should be non-zero", egressQueueName)
		}
	}

	for dscpValue := 0; dscpValue < 64; dscpValue++ {
		etPath := gnmi.OTG().Flow(fmt.Sprintf("dscp-%d-%s", dscpValue, atePort2.Name)).TaggedMetricAny()
		ets := gnmi.GetAll(t, ate.OTG(), etPath.State())

		dscpAsHex := fmt.Sprintf("0x%02x", dscpValue)

		if len(ets) != 1 {
			t.Logf("got %d flows, but expected one, this probably indicates that the flow has"+
				" some packets tagged 01 and some tagged 11 (congestion experienced) -- "+
				"this should not happen in this test case, will continue validation...", len(ets))
		}

		for _, et := range ets {
			if len(et.Tags) != 2 {
				t.Errorf("expected two metric tags (dscp/ecn) but got %d", len(et.Tags))
			}

			for _, tag := range et.Tags {
				tagName := tag.GetTagName()
				valueAsHex := tag.GetTagValue().GetValueAsHex()
				t.Logf("flow with dscp value %d, tag name %q, got value %s", dscpValue, tagName, valueAsHex)
				if strings.Contains(tagName, "dscp") {
					if valueAsHex != dscpAsHex {
						t.Errorf("expected dscp bit to be %x, but got %s", dscpAsHex, valueAsHex)
					}
				} else {
					// ECN should be 10 -- ecn capable but no congestion experienced.
					if valueAsHex != "0x2" {
						t.Errorf("expected ecn bit to be 0x2, but got %s", valueAsHex)
					}
				}
			}
		}
	}
}

func testCongestionCreateFlows(otgConfig gosnappi.Config, protocol string, dutPortSpeed int) {
	// Target flow rate is 60% of the ate port speed spread across 64 flows (do this in kbps so we
	// still work w/ round numbers on 1g interfaces).
	portSpeedInKbps := dutPortSpeed * 1_000_000
	portSpeedSixtyPercent := float32(portSpeedInKbps) * float32(0.6)
	targetTotalFlowRate := uint64(portSpeedSixtyPercent / 64)

	for _, sourceAtePort := range []*attrs.Attributes{atePort2, atePort3} {
		for dscpValue := 0; dscpValue < 64; dscpValue++ {
			finalTargetFlowRate := targetTotalFlowRate
			if dscpValue <= 7 {
				// There are fewer flows in the be0/be1 queues so increase those flows to have
				// a similar amount of traffic so wred handles things consistently.
				finalTargetFlowRate = targetTotalFlowRate * 2
			}

			createFlow(
				otgConfig,
				protocol,
				finalTargetFlowRate,
				dscpValue,
				sourceAtePort,
			)
		}
	}
}

func testCongestionValidateFlows(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, dutPortSpeed int, startingCounters map[entname.QoSQueue]*queueCounters) {
	maxAllowedZeroPackets, _ := getZeroIshThresholds(dutPortSpeed)
	endingCounters := getQueueCounters(t, dut)

	for egressQueueName, egressQueueEndingCounters := range endingCounters {
		egressQueueStartingCounters := startingCounters[egressQueueName]

		queueDroppedPackets, queueTransmitPackets, queueTransmitOctets := logAndGetResolvedQueueCounters(
			t,
			egressQueueName,
			egressQueueStartingCounters,
			egressQueueEndingCounters,
		)

		if queueTransmitPackets == 0 {
			t.Errorf("queue %s indicates 0 transmit packets but should be non-zero", egressQueueName)
		}

		if queueTransmitOctets == 0 {
			t.Errorf("queue %s indicates 0 transmit octets but should be non-zero", egressQueueName)
		}

		if egressQueueName == entname.QoSNC1 {
			// NC1 should have no drops
			if queueDroppedPackets > maxAllowedZeroPackets {
				t.Errorf("queue %s indicates %d dropped packets but should show zero or near-zero", egressQueueName, queueDroppedPackets)
			}
		} else {
			// Any other queue should have at least some drops.
			if queueDroppedPackets == 0 {
				t.Errorf(
					"queue %s indicates %d dropped packets but should show some non-zero value as there is congestion in this case",
					egressQueueName, queueDroppedPackets)
			}
		}
	}

	var congestedFlowCount int

	// These should have the majority of flows have ecn set.
	for _, sourceAtePort := range []*attrs.Attributes{atePort2, atePort3} {
		for dscpValue := 0; dscpValue < 48; dscpValue++ {
			etPath := gnmi.OTG().Flow(fmt.Sprintf("dscp-%d-%s", dscpValue, sourceAtePort.Name)).TaggedMetricAny()
			ets := gnmi.GetAll(t, ate.OTG(), etPath.State())

			dscpAsHex := fmt.Sprintf("0x%02x", dscpValue)

			if len(ets) != 2 {
				// We should always have two sets of metric tags for flows in this test case -- the
				// initial packets will not be marked as congestion experienced of course, but all
				// the flows should eventually be marked as such. if we get a flow w/ only 1 path
				// we know this flow had no congestion.
				t.Logf("expected two sets of tags for flow but got %d\n\t%s", len(ets), prettyPrint(ets))
				continue
			}

			// We only care about checking the second set of tags as these are the ones that should
			// have been marked w/ congestion experienced.
			if len(ets[1].Tags) != 2 {
				t.Errorf("expected two metric tags (dscp/ecn) but got %d", len(ets[1].Tags))
			}

			for _, tag := range ets[1].Tags {
				tagName := tag.GetTagName()
				valueAsHex := tag.GetTagValue().GetValueAsHex()
				t.Logf("flow with dscp value %d, tag name %q, got value %s", dscpValue, tagName, valueAsHex)
				if strings.Contains(tagName, "dscp") {
					if valueAsHex != dscpAsHex {
						t.Errorf("expected dscp bit to be %x, but got %s", dscpAsHex, valueAsHex)
					}
				} else if valueAsHex != "0x2" {
					// Not dscp tag, and not 0x2, meaning ecn tag and congestion experienced.
					congestedFlowCount++
				}
			}
		}
	}

	if float32(congestedFlowCount/96) < 0.9 {
		t.Errorf("less than 90 percent of flows (not in nc1 queue) had congestion experienced")
	}

	// These flows should all have no ecn set.
	for _, sourceAtePort := range []*attrs.Attributes{atePort2, atePort3} {
		for dscpValue := 48; dscpValue < 64; dscpValue++ {
			etPath := gnmi.OTG().Flow(fmt.Sprintf("dscp-%d-%s", dscpValue, sourceAtePort.Name)).TaggedMetricAny()
			ets := gnmi.GetAll(t, ate.OTG(), etPath.State())

			dscpAsHex := fmt.Sprintf("0x%02x", dscpValue)

			for _, et := range ets {
				if len(et.Tags) != 2 {
					t.Errorf("expected two metric tags (dscp/ecn) but got %d", len(et.Tags))
				}

				for _, tag := range et.Tags {
					tagName := tag.GetTagName()
					valueAsHex := tag.GetTagValue().GetValueAsHex()
					t.Logf("flow with dscp value %d, tag name %q, got value %s", dscpValue, tagName, valueAsHex)
					if strings.Contains(tagName, "dscp") {
						if valueAsHex != dscpAsHex {
							t.Errorf("expected dscp bit to be %x, but got %s", dscpAsHex, valueAsHex)
						}
					} else {
						if valueAsHex != "0x2" {
							t.Errorf("expected ecn bit for dscp value %d to be 0x2, but got %s", dscpValue, valueAsHex)
						}
					}
				}
			}
		}
	}
}

func testNC1CongestionCreateFlows(otgConfig gosnappi.Config, protocol string, dutPortSpeed int) {
	// Target flow rate is 60% of the ate port speed spread across 16 flows (do this in kbps so we
	// still work w/ round numbers on 1g interfaces).
	portSpeedInKbps := dutPortSpeed * 1_000_000
	portSpeedSixtyPercent := float32(portSpeedInKbps) * float32(0.6)
	targetTotalFlowRate := uint64(portSpeedSixtyPercent / 16)

	for _, sourceAtePort := range []*attrs.Attributes{atePort2, atePort3} {
		for dscpValue := 48; dscpValue < 64; dscpValue++ {
			createFlow(
				otgConfig,
				protocol,
				targetTotalFlowRate,
				dscpValue,
				sourceAtePort,
			)
		}
	}
}

func testNC1CongestionValidateFlows(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, dutPortSpeed int, startingCounters map[entname.QoSQueue]*queueCounters) {
	maxAllowedZeroPackets, maxAllowedZeroOctets := getZeroIshThresholds(dutPortSpeed)
	endingCounters := getQueueCounters(t, dut)

	for egressQueueName, egressQueueEndingCounters := range endingCounters {
		egressQueueStartingCounters := startingCounters[egressQueueName]

		queueDroppedPackets, queueTransmitPackets, queueTransmitOctets := logAndGetResolvedQueueCounters(
			t,
			egressQueueName,
			egressQueueStartingCounters,
			egressQueueEndingCounters,
		)

		if egressQueueName == entname.QoSNC1 {
			if queueTransmitPackets == 0 {
				t.Errorf("queue %s indicates 0 transmit packets but should be non-zero", egressQueueName)
			}

			if queueTransmitOctets == 0 {
				t.Errorf("queue %s indicates 0 transmit octets but should be non-zero", egressQueueName)
			}

			if queueDroppedPackets == 0 {
				t.Errorf("queue %s indicates %d dropped packets but should show non-zero", egressQueueName, queueDroppedPackets)
			}
		} else {
			if queueTransmitPackets > maxAllowedZeroPackets {
				t.Errorf("queue %s indicates non zero transmit packets but should be zero or near zero", egressQueueName)
			}

			if queueTransmitOctets > maxAllowedZeroOctets {
				t.Errorf("queue %s indicates non zero transmit octets but should be zero or near zero", egressQueueName)
			}
		}
	}

	var congestedFlowCount int

	for _, sourceAtePort := range []*attrs.Attributes{atePort2, atePort3} {
		for dscpValue := 48; dscpValue < 64; dscpValue++ {
			etPath := gnmi.OTG().Flow(fmt.Sprintf("dscp-%d-%s", dscpValue, sourceAtePort.Name)).TaggedMetricAny()
			ets := gnmi.GetAll(t, ate.OTG(), etPath.State())

			dscpAsHex := fmt.Sprintf("0x%02x", dscpValue)

			if len(ets) != 2 {
				// Similar to the congestion (non NC1) test, we expect two sets of metrics -- one for
				// the start of the flow where ecn is not yet set, and the second for when it is.
				t.Logf("expected two sets of tags for flow but got %d\n\t%s", len(ets), prettyPrint(ets))
				continue
			}

			if len(ets[1].Tags) != 2 {
				t.Errorf("expected two metric tags (dscp/ecn) but got %d", len(ets[1].Tags))
			}

			for _, tag := range ets[1].Tags {
				tagName := tag.GetTagName()
				valueAsHex := tag.GetTagValue().GetValueAsHex()
				t.Logf("flow with dscp value %d, tag name %q, got value %s", dscpValue, tagName, valueAsHex)
				if strings.Contains(tagName, "dscp") {
					if valueAsHex != dscpAsHex {
						t.Errorf("expected dscp bit to be %x, but got %s", dscpAsHex, valueAsHex)
					}
				} else if valueAsHex != "0x2" {
					// Not dscp tag, and not 0x2, meaning ecn tag and congestion experienced.
					congestedFlowCount++
				}
			}
		}
	}

	if float32(congestedFlowCount/32) < 0.9 {
		t.Errorf("less than 90 percent of flows (in nc1 queue) had congestion experienced")
	}
}

func TestDSCPTransparency(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()

	configureDUT(t, dut)

	otgConfig := configureATE(t, ate)

	dutPortSpeed := dut.Ports()[0].Speed()
	if dutPortSpeed == 0 {
		t.Log("dut port speed was unset, assuming 100G.")
		dutPortSpeed = 100
	}

	for _, testCase := range testCases {
		for _, flowProto := range []string{ipv4, ipv6} {
			t.Run(fmt.Sprintf("%s-%s", testCase.name, flowProto), func(t *testing.T) {
				otgConfig.Flows().Clear()
				testCase.createFlowsF(otgConfig, flowProto, int(dutPortSpeed))

				otg.PushConfig(t, otgConfig)
				otg.StartProtocols(t)
				otgutils.WaitForARP(t, otg, otgConfig, flowProto)

				// Get QoS egress packet counters before the traffic.
				startingCounters := getQueueCounters(t, dut)

				otg.StartTraffic(t)
				time.Sleep(trafficRunDuration)
				otg.StopTraffic(t)
				time.Sleep(trafficStopWaitDuration)

				testCase.validateFlowsF(t, dut, ate, int(dutPortSpeed), startingCounters)
				otg.StopProtocols(t)
			})
		}
	}
}
