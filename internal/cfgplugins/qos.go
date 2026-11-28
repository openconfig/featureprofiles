// Copyright 2023 Google LLC
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

package cfgplugins

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// QosClassifier is a struct to hold the QoS classifier configuration parameters.
type QosClassifier struct {
	Desc        string
	Name        string
	ClassType   oc.E_Qos_Classifier_Type
	TermID      string
	TargetGroup string
	DscpSet     []uint8
	RemarkDscp  uint8
}

// SchedulerPolicy is a struct to hold the scheduler policy configuration.
type SchedulerPolicy struct {
	Desc        string
	Sequence    uint32
	SetPriority bool
	Priority    oc.E_Scheduler_Priority
	InputID     string
	InputType   oc.E_Input_InputType
	SetWeight   bool
	QueueName   string
	TargetGroup string
}

// ForwardingGroup is a struct to hold the forwarding group configuration.
type ForwardingGroup struct {
	Desc        string
	QueueName   string
	TargetGroup string
	Priority    uint8
}

// QoSSchedulerInterface is a struct to hold the QoS scheduler interface configuration.
type QoSSchedulerInterface struct {
	Desc      string
	QueueName string
	Scheduler string
}

// SchedulerParams is a struct to hold parameters for configuring a QoS scheduler.
type SchedulerParams struct {
	SchedulerName  string
	PolicerName    string
	InterfaceName  string
	ClassName      string
	CirValue       uint64
	PirValue       uint64
	BurstSize      uint32
	SequenceNumber uint32
	QueueID        uint32
	QueueName      string
}

func runCliCommand(t *testing.T, dut *ondatra.DUTDevice, cliCommand string) string {
	cliClient := dut.RawAPIs().CLI(t)
	output, err := cliClient.RunCommand(context.Background(), cliCommand)
	if err != nil {
		t.Fatalf("Failed to execute CLI command '%s': %v", cliCommand, err)
	}
	t.Logf("Received from cli: %s", output.Output())
	return output.Output()
}

// NewQosInitialize initializes the QoS on the DUT.
// This is a temporary solution to initialize the QoS on the DUT.
// This will be removed once the QoS initialization is supported in the Ondatra.
func NewQosInitialize(t *testing.T, dut *ondatra.DUTDevice) {
	if dut.Vendor() == ondatra.ARISTA {
		queues := netutil.CommonTrafficQueues(t, dut)
		qList := []string{queues.BE1, queues.AF1, queues.AF2, queues.AF3, queues.AF4, queues.NC1}
		var cliConfig strings.Builder
		cliConfig.WriteString("configure terminal\n")
		for index, queue := range qList {
			cliConfig.WriteString(fmt.Sprintf("qos tx-queue %d name %s\n!\n", index, queue))
			cliConfig.WriteString(fmt.Sprintf("qos map traffic-class %d to tx-queue %d\n!\n", index, index))
			cliConfig.WriteString(fmt.Sprintf("qos traffic-class %d name %s\n!\n", index, fmt.Sprintf("target-group-%s", queue)))
		}
		helpers.GnmiCLIConfig(t, dut, cliConfig.String())
	}
}

// NewQoSClassifierConfiguration creates a QoS classifier configuration.
func NewQoSClassifierConfiguration(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos, classifiers []QosClassifier) *oc.Qos {

	t.Logf("QoS classifiers config: %v", classifiers)
	for _, class := range classifiers {
		t.Log(class.Desc)
		classifier := q.GetOrCreateClassifier(class.Name)
		classifier.SetName(class.Name)
		classifier.SetType(class.ClassType)

		term, err := classifier.NewTerm(class.TermID)
		if err != nil {
			t.Fatalf("Failed to create classifier.NewTerm(): %v", err)
		}

		term.SetId(class.TermID)
		action := term.GetOrCreateActions()
		if class.TargetGroup != "" {
			action.SetTargetGroup(class.TargetGroup)
		}
		if len(class.DscpSet) > 0 {
			condition := term.GetOrCreateConditions()
			switch class.ClassType {
			case oc.Qos_Classifier_Type_IPV4:
				condition.GetOrCreateIpv4().SetDscpSet(class.DscpSet)
			case oc.Qos_Classifier_Type_IPV6:
				condition.GetOrCreateIpv6().SetDscpSet(class.DscpSet)
			default:
				t.Fatal("DSCP classification is supported only for IPv4/IPv6 classifier types")
			}
		}

		// DSCP remark configuration is not supported. Adding external static configuration after QoS OC configuration
		if class.RemarkDscp != 0 && !deviations.QosRemarkOCUnsupported(dut) {
			action.GetOrCreateRemark().SetDscp = ygot.Uint8(class.RemarkDscp)
		}
	}
	return q
}

// NewQoSSchedulerPolicy creates a QoS scheduler policy configuration.
func NewQoSSchedulerPolicy(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos, policies []SchedulerPolicy) *oc.Qos {

	t.Logf("QoS scheduler policy config: %v", policies)
	schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
	schedulerPolicy.SetName("scheduler")
	t.Logf("QoS scheduler policies config: %v", policies)
	for _, policy := range policies {
		s := schedulerPolicy.GetOrCreateScheduler(policy.Sequence)
		s.SetSequence(policy.Sequence)
		if policy.SetPriority {
			s.SetPriority(policy.Priority)
		}
		input := s.GetOrCreateInput(policy.InputID)
		input.SetId(policy.InputID)
		input.SetInputType(policy.InputType)
		input.SetQueue(policy.QueueName)
	}
	return q
}

// NewQoSForwardingGroup creates a QoS forwarding group configuration.
func NewQoSForwardingGroup(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos, forwardingGroups []ForwardingGroup) {
	t.Logf("QoS forwarding groups config: %v", forwardingGroups)
	for _, fg := range forwardingGroups {
		qoscfg.SetForwardingGroup(t, dut, q, fg.TargetGroup, fg.QueueName)
	}
}

// NewQoSSchedulerInterface creates a QoS scheduler interface configuration.
func NewQoSSchedulerInterface(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos, schedulerIntfs []QoSSchedulerInterface, schedulerPort string) *oc.Qos {
	t.Logf("QoS output interface config: %v", schedulerIntfs)
	schPort := dut.Port(t, schedulerPort)
	for _, intf := range schedulerIntfs {
		i := q.GetOrCreateInterface(schPort.Name())
		i.SetInterfaceId(schPort.Name())
		i.GetOrCreateInterfaceRef().Interface = ygot.String(schPort.Name())
		if deviations.InterfaceRefConfigUnsupported(dut) {
			i.InterfaceRef = nil
		}
		output := i.GetOrCreateOutput()
		schedulerPolicy := output.GetOrCreateSchedulerPolicy()
		schedulerPolicy.SetName(intf.Scheduler)
		queue := output.GetOrCreateQueue(intf.QueueName)
		queue.SetName(intf.QueueName)
	}
	return q
}

// NewQoSQueue creates a QoS queue configuration.
func NewQoSQueue(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos) {
	queues := netutil.CommonTrafficQueues(t, dut)

	if deviations.QOSQueueRequiresID(dut) {
		queueNames := []string{queues.NC1, queues.AF4, queues.AF3, queues.AF2, queues.AF1, queues.BE1}
		for i, queue := range queueNames {
			q1 := q.GetOrCreateQueue(queue)
			q1.Name = ygot.String(queue)
			queueid := len(queueNames) - i
			q1.QueueId = ygot.Uint8(uint8(queueid))
		}
		t.Logf("\nDUT %s %s %s requires QoS queue requires ID deviation \n\n", dut.Vendor(), dut.Model(), dut.Version())
	}
}

func configureQosClassifierDscpRemarkFromCli(t *testing.T, dut *ondatra.DUTDevice, classifierName string, interfaceName string, ipv4DscpValues []uint8, ipv6DscpValues []uint8) {
	gnmiClient := dut.RawAPIs().GNMI(t)
	qosConfig := enableQosRemarkDscpCliConfig(dut, classifierName, interfaceName, ipv4DscpValues, ipv6DscpValues)
	t.Logf("Push the CLI Qos config:%s", dut.Vendor())
	gpbSetRequest := buildCliSetRequest(qosConfig)
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Errorf("Failed to set qos classifier from cli: %v", err)
	}
}

// ConfigureQosClassifierDscpRemark configures the QoS classifier DSCP remark through OC or CLI based on the deviation.
func ConfigureQosClassifierDscpRemark(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, classifierName string, interfaceName string, ipv4DscpValues []uint8, ipv6DscpValues []uint8) {
	if deviations.QosRemarkOCUnsupported(dut) {
		t.Logf("Configuring qos dscp remark through CLI")
		configureQosClassifierDscpRemarkFromCli(t, dut, classifierName, interfaceName, ipv4DscpValues, ipv6DscpValues)
	} else {
		t.Logf("Configuring qos dscp remark through OC")
		configureQosClassifierDscpRemarkFromOc(t, dut, qos, classifierName, ipv4DscpValues, ipv6DscpValues)
	}
}

func configureQosClassifierDscpRemarkFromOc(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, classifierName string, ipv4DscpValues []uint8, ipv6DscpValues []uint8) {
	if qos == nil {
		t.Fatal("Qos OC config must be defined")
	}
	var classifiers []QosClassifier

	for i, dscp := range ipv4DscpValues {
		classifiers = append(classifiers, QosClassifier{
			Desc:       fmt.Sprintf("IPv4 DSCP term %d", i+1),
			Name:       classifierName,
			ClassType:  oc.Qos_Classifier_Type_IPV4,
			TermID:     fmt.Sprintf("termV4-%d", i+1),
			RemarkDscp: dscp,
		})
	}
	for i, dscp := range ipv6DscpValues {
		classifiers = append(classifiers, QosClassifier{
			Desc:       fmt.Sprintf("IPv6 DSCP term %d", i+1),
			Name:       classifierName,
			ClassType:  oc.Qos_Classifier_Type_IPV6,
			TermID:     fmt.Sprintf("termV6-%d", i+1),
			RemarkDscp: dscp,
		})
	}

	NewQoSClassifierConfiguration(t, dut, qos, classifiers)
}

func enableQosRemarkDscpCliConfig(dut *ondatra.DUTDevice, classifierName string, interfaceName string, ipv4DscpValues []uint8, ipv6DscpValues []uint8) string {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return `
        qos rewrite dscp
        !`
	default:
		return ""
	}
}

// CreateQueues configures the given queues within the provided oc.Qos object.
// It handles setting the queue name and optionally the QueueId based on deviations.
func CreateQueues(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, queues []string) {
	for index, q := range queues {
		queue := qos.GetOrCreateQueue(q)
		queue.Name = ygot.String(q)
		if deviations.QOSQueueRequiresID(dut) {
			queue.QueueId = ygot.Uint8(uint8(index))
		}
	}
}

func configureTwoRateThreeColorSchedulerFromOC(dut *ondatra.DUTDevice, batch *gnmi.SetBatch, params *SchedulerParams) {
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
	// Platforms with QosSchedulerConfigRequired (such as Cisco IOS-XR) reject bc, be, and
	// exceed-action/config/drop as unsupported YANG paths on two-rate-three-color schedulers,
	// and default to dropping exceeding packets.
	if !deviations.QosSchedulerConfigRequired(dut) {
		if params.BurstSize > 0 {
			trtc.Bc = ygot.Uint32(params.BurstSize)
			trtc.Be = ygot.Uint32(params.BurstSize)
		}
		trtc.GetOrCreateExceedAction().Drop = ygot.Bool(true)
	}
	trtc.GetOrCreateViolateAction().Drop = ygot.Bool(true)
	qosPath := gnmi.OC().Qos().Config()
	gnmi.BatchUpdate(batch, qosPath, qos)
}

func configureTwoRateThreeColorSchedulerFromCLI(t *testing.T, dut *ondatra.DUTDevice, params *SchedulerParams) {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cliConfig := fmt.Sprintf(`
        policy-map type quality-of-service %s
        class %s
        set traffic-class %d 
        police rate %d bps burst-size %d bytes rate %d bps burst-size %d bytes
        !
        `, params.SchedulerName, params.ClassName, params.QueueID, params.CirValue, params.BurstSize, params.PirValue, params.BurstSize)
		helpers.GnmiCLIConfig(t, dut, cliConfig)
	default:
		t.Errorf("Unsupported CLI command for dut %v %s", dut.Vendor(), dut.Name())
	}
}

// NewTwoRateThreeColorScheduler configures a two-rate three-color policer/scheduler on the DUT.
// It uses either OC or CLI based on the QosTwoRateThreeColorPolicerOCUnsupported deviation.
func NewTwoRateThreeColorScheduler(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, params *SchedulerParams) {
	if deviations.QosTwoRateThreeColorPolicerOCUnsupported(dut) {
		configureTwoRateThreeColorSchedulerFromCLI(t, dut, params)
	} else {
		configureTwoRateThreeColorSchedulerFromOC(dut, batch, params)
	}
}

// ApplyQosPolicyOnInterface applies a QoS policy on the specified interface.
// The configuration is applied using either CLI or OC based on the QosSchedulerIngressPolicer deviation.
func ApplyQosPolicyOnInterface(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, params *SchedulerParams) {
	if deviations.QosSchedulerIngressPolicer(dut) {
		applyQosPolicyOnInterfaceFromCLI(t, dut, params)
	} else {
		applyQosPolicyOnInterfaceFromOC(dut, batch, params)
	}
}

func applyQosPolicyOnInterfaceFromOC(dut *ondatra.DUTDevice, batch *gnmi.SetBatch, params *SchedulerParams) {
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

func applyQosPolicyOnInterfaceFromCLI(t *testing.T, dut *ondatra.DUTDevice, params *SchedulerParams) {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cliConfig := fmt.Sprintf(`
        interface %s
        service-policy type qos input %s
        !
        `, params.InterfaceName, params.SchedulerName)
		helpers.GnmiCLIConfig(t, dut, cliConfig)
	default:
		t.Errorf("Unsupported CLI command for dut %v %s", dut.Vendor(), dut.Name())
	}
}

func GetPolicyCLICounters(t *testing.T, dut *ondatra.DUTDevice, policyName string) string {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cliConfig := fmt.Sprintf("show policy-map type qos %s counters", policyName)
		return runCliCommand(t, dut, cliConfig)
	default:
		return ""
	}
}

// ConfigureQoSDSCPRemarkFix adds configuration that may be needed
// to rewrite DSCP packet fields.  It is intended to be called only if DSCP
// remarking is used.  It must be called after NewQoSClassifierConfiguration
// and SetInputClassifier
func ConfigureQoSDSCPRemarkFix(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.QosRemarkOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			aristaConfigureRemarkIpv4(t, dut)
			aristaConfigureRemarkIpv6(t, dut)
		}
	}
}

// aristaConfigureRemarkIpv4 configure remark for ipv4 through CLI
func aristaConfigureRemarkIpv4(t *testing.T, dut *ondatra.DUTDevice) {
	jsonConfig := `
    policy-map type quality-of-service __yang_[IPV4__dscp_based_classifier_ipv4][IPV6__dscp_based_classifier_ipv6]
	class __yang_[dscp_based_classifier_ipv4]_[0]
	set dscp 0
	class __yang_[dscp_based_classifier_ipv4]_[1]
	set dscp 0
	class __yang_[dscp_based_classifier_ipv4]_[2]
	set dscp 0
	class __yang_[dscp_based_classifier_ipv4]_[3]
	set dscp 0
	class __yang_[dscp_based_classifier_ipv4]_[4]
	set dscp 0
	class __yang_[dscp_based_classifier_ipv4]_[6]
	set dscp 6
		`
	helpers.GnmiCLIConfig(t, dut, jsonConfig)
}

// aristaConfigureRemarkIpv6 configure remark for ipv6 through CLI
func aristaConfigureRemarkIpv6(t *testing.T, dut *ondatra.DUTDevice) {
	jsonConfig := `
    policy-map type quality-of-service __yang_[IPV4__dscp_based_classifier_ipv4][IPV6__dscp_based_classifier_ipv6]
   class __yang_[dscp_based_classifier_ipv6]_[0]
      set dscp 0
   class __yang_[dscp_based_classifier_ipv6]_[1]
      set dscp 0
   class __yang_[dscp_based_classifier_ipv6]_[3]
   set dscp 0
   class __yang_[dscp_based_classifier_ipv6]_[2]
   set dscp 0
   class __yang_[dscp_based_classifier_ipv6]_[4]
   set dscp 0
   class __yang_[dscp_based_classifier_ipv6]_[6]
   set dscp 6
		`
	helpers.GnmiCLIConfig(t, dut, jsonConfig)
}

// QosClassificationOCConfig builds an OpenConfig QoS classification configuration.
func QosClassificationOCConfig(t *testing.T) {
	t.Helper()
	// TODO: OC commands for QOS are not in the place. Need to fix the below commented code once implemented the OC commands.
	d := &oc.Root{}
	qos := d.GetOrCreateQos()

	// DSCP → traffic-class 0
	classifier0 := qos.GetOrCreateClassifier("dscp-to-tc0")
	classifier0.Type = oc.Qos_Classifier_Type_IPV4
	// classifier0.Target = []oc.E_Qos_TargetType{oc.Qos_TargetType_FORWARDING_GROUP}
	term0 := classifier0.GetOrCreateTerm("tc0")
	term0.GetOrCreateConditions().GetOrCreateIpv4().DscpSet = []uint8{0, 1, 2, 3, 4, 5, 6, 7}
	// term0.GetOrCreateActions().Config = &oc.Qos_Classifier_Term_Actions{
	// 	TargetGroup: ygot.String("forwarding-group-tc0"),
	// }

	// DSCP → traffic-class 1
	classifier1 := qos.GetOrCreateClassifier("dscp-to-tc1")
	classifier1.Type = oc.Qos_Classifier_Type_IPV4
	// classifier1.Target = []oc.E_Qos_TargetType{oc.Qos_TargetType_FORWARDING_GROUP}
	term1 := classifier1.GetOrCreateTerm("tc1")
	term1.GetOrCreateConditions().GetOrCreateIpv4().DscpSet = []uint8{8, 9, 10, 11, 12, 13, 14, 15}
	// term1.GetOrCreateActions().Config = &oc.Qos_Classifier_Term_Actions{
	// 	TargetGroup: ygot.String("forwarding-group-tc1"),
	// }

	// DSCP → traffic-class 4
	classifier4 := qos.GetOrCreateClassifier("dscp-to-tc4")
	classifier4.Type = oc.Qos_Classifier_Type_IPV4
	// classifier4.Target = []oc.E_Qos_TargetType{oc.Qos_TargetType_FORWARDING_GROUP}
	term4 := classifier4.GetOrCreateTerm("tc4")
	term4.GetOrCreateConditions().GetOrCreateIpv4().DscpSet = []uint8{40, 41, 42, 43, 44, 45, 46, 47}
	// term4.GetOrCreateActions().Config = &oc.Qos_Classifier_Term_Actions{
	// 	TargetGroup: ygot.String("forwarding-group-tc4"),
	// }

	// DSCP → traffic-class 7
	classifier7 := qos.GetOrCreateClassifier("dscp-to-tc7")
	classifier7.Type = oc.Qos_Classifier_Type_IPV4
	// classifier7.Target = []oc.E_Qos_TargetType{oc.Qos_TargetType_FORWARDING_GROUP}
	term7 := classifier7.GetOrCreateTerm("tc7")
	term7.GetOrCreateConditions().GetOrCreateIpv4().DscpSet = []uint8{48, 49, 50, 51, 52, 53, 54, 55}
	// term7.GetOrCreateActions().Config = &oc.Qos_Classifier_Term_Actions{
	// 	TargetGroup: ygot.String("forwarding-group-tc7"),
	// }

	// fg0 := qos.GetOrCreateForwardingGroup("forwarding-group-tc0")
	// fg0.ForwardingClass = ygot.Uint8(0)

	// fg1 := qos.GetOrCreateForwardingGroup("forwarding-group-tc1")
	// fg1.ForwardingClass = ygot.Uint8(1)

	// fg4 := qos.GetOrCreateForwardingGroup("forwarding-group-tc4")
	// fg4.ForwardingClass = ygot.Uint8(4)

	// fg7 := qos.GetOrCreateForwardingGroup("forwarding-group-tc7")
	// fg7.ForwardingClass = ygot.Uint8(7)

	// // Forwarding group for policy-map af3
	// fg3 := qos.GetOrCreateForwardingGroup("forwarding-group-tc3")
	// fg3.ForwardingClass = ygot.Uint8(3)

	// policy := qos.GetOrCreatePolicy("af3")
	// stmt := policy.GetOrCreateStatement("class-default")
	// stmt.GetOrCreateActions().SetForwardingGroup = ygot.String("forwarding-group-tc3")
}

// configureOneRateTwoColorSchedulerFromOC programs a One-Rate Two-Color scheduler using OpenConfig. It builds the scheduler-policy, scheduler, queue input, CIR/Burst values, and exceed-action under /qos/scheduler-policy. The resulting OC subtree is added to the provided gNMI SetBatch.
func configureOneRateTwoColorSchedulerFromOC(batch *gnmi.SetBatch, params *SchedulerParams) {
	qos := &oc.Qos{}
	sp := qos.GetOrCreateSchedulerPolicy(params.SchedulerName)
	sp.Name = ygot.String(params.SchedulerName)
	sched := sp.GetOrCreateScheduler(params.SequenceNumber)
	sched.Sequence = ygot.Uint32(params.SequenceNumber)
	sched.Type = oc.QosTypes_QOS_SCHEDULER_TYPE_ONE_RATE_TWO_COLOR
	input := sched.GetOrCreateInput(params.PolicerName)
	input.InputType = oc.Input_InputType_QUEUE
	input.Queue = ygot.String(params.QueueName)
	trtc := sched.GetOrCreateOneRateTwoColor()
	trtc.Cir = ygot.Uint64(params.CirValue)
	trtc.Bc = ygot.Uint32(params.BurstSize)
	trtc.GetOrCreateExceedAction().Drop = ygot.Bool(false)
	qosPath := gnmi.OC().Qos().SchedulerPolicy(params.SchedulerName).Config()
	gnmi.BatchUpdate(batch, qosPath, sp)
}

// configureOneRateTwoColorSchedulerFromCLI configures a One-Rate Two-Color scheduler using CLI commands.
func configureOneRateTwoColorSchedulerFromCLI(t *testing.T, dut *ondatra.DUTDevice, params *SchedulerParams) {
	if deviations.QosTwoRateThreeColorPolicerOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			cliConfig := fmt.Sprintf(`
			policy-map type quality-of-service %s
			class %s
			set traffic-class %d 
			police cir %d bps bc %d bytes
			!
			`, params.SchedulerName, params.ClassName, params.QueueID, params.CirValue, params.BurstSize)
			helpers.GnmiCLIConfig(t, dut, cliConfig)
		default:
			t.Errorf("Unsupported CLI command for dut %v %s", dut.Vendor(), dut.Name())
		}
	}
}

// NewOneRateTwoColorScheduler is the top-level API used by callers to configure an ORTC scheduler.
func NewOneRateTwoColorScheduler(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, params *SchedulerParams) {
	if deviations.QosTwoRateThreeColorPolicerOCUnsupported(dut) {
		configureOneRateTwoColorSchedulerFromCLI(t, dut, params)
	} else {
		configureOneRateTwoColorSchedulerFromOC(batch, params)
	}
}

const (
	ciscoECNMinThreshold    = uint64(8005632)
	ciscoECNMaxThreshold    = uint64(8011776)
	fallbackECNMaxThreshold = uint64(3000000)
)

// QoSSchedulerPolicyEntry holds the scheduler policy entry configuration including optional
// scheduler type and WRR/Strict input weight without modifying the existing SchedulerPolicy struct.
type QoSSchedulerPolicyEntry struct {
	Desc          string
	Sequence      uint32
	SetPriority   bool
	Priority      oc.E_Scheduler_Priority
	SchedulerType oc.E_QosTypes_QOS_SCHEDULER_TYPE
	InputID       string
	InputType     oc.E_Input_InputType
	SetWeight     bool
	Weight        uint64
	QueueName     string
	TargetGroup   string
}

// QoSOutputQueueBinding holds the egress scheduler-policy and queue-management-profile binding
// for a single output queue without modifying the existing QoSSchedulerInterface struct.
type QoSOutputQueueBinding struct {
	Desc                   string
	QueueName              string
	Scheduler              string
	QueueManagementProfile string
}

// QoSQueueManagementProfile holds the configuration parameters for a QoS WRED queue-management profile.
type QoSQueueManagementProfile struct {
	ProfileName               string
	EnableECN                 bool
	Drop                      bool
	MinThreshold              uint64
	MaxThreshold              uint64
	MaxDropProbabilityPercent uint8
	Weight                    uint32
	SetWeight                 bool
}

// QoSInputClassifierBinding holds an interface name and the IPv4/IPv6 classifier name to bind on input.
type QoSInputClassifierBinding struct {
	Desc           string
	IntfName       string
	ClassifierType oc.E_Input_Classifier_Type
	ClassifierName string
}

// NewQoSSchedulerPolicyFromEntries creates a QoS scheduler policy configuration supporting
// scheduler type and WRR/Strict input weights.
func NewQoSSchedulerPolicyFromEntries(t *testing.T, _ *ondatra.DUTDevice, q *oc.Qos, policies []QoSSchedulerPolicyEntry) *oc.Qos {
	t.Helper()
	t.Logf("QoS scheduler policy config: %v", policies)
	schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
	schedulerPolicy.SetName("scheduler")
	for _, policy := range policies {
		s := schedulerPolicy.GetOrCreateScheduler(policy.Sequence)
		s.SetSequence(policy.Sequence)
		if policy.SetPriority {
			s.SetPriority(policy.Priority)
		}
		if policy.SchedulerType != oc.QosTypes_QOS_SCHEDULER_TYPE_UNSET {
			s.SetType(policy.SchedulerType)
		}
		input := s.GetOrCreateInput(policy.InputID)
		input.SetId(policy.InputID)
		input.SetInputType(policy.InputType)
		input.SetQueue(policy.QueueName)
		if policy.SetWeight && policy.Weight > 0 {
			input.SetWeight(policy.Weight)
		}
	}
	return q
}

// NewQoSForwardingGroupsFromList populates forwarding-group and output-queue entries on the provided
// oc.Qos struct without issuing intermediate gNMI Replace calls, allowing callers to push the full
// QoS configuration atomically.
func NewQoSForwardingGroupsFromList(t *testing.T, _ *ondatra.DUTDevice, q *oc.Qos, forwardingGroups []ForwardingGroup) *oc.Qos {
	t.Helper()
	t.Logf("QoS forwarding groups config: %v", forwardingGroups)
	for _, fg := range forwardingGroups {
		fgObj := q.GetOrCreateForwardingGroup(fg.TargetGroup)
		fgObj.SetName(fg.TargetGroup)
		fgObj.SetOutputQueue(fg.QueueName)
		qObj := q.GetOrCreateQueue(fg.QueueName)
		qObj.SetName(fg.QueueName)
	}
	return q
}

// NewQoSOutputInterfaceByName configures egress scheduler-policy and per-queue queue-management-profile
// bindings on an interface identified by its OpenConfig interface name (physical port or LACP aggregate).
func NewQoSOutputInterfaceByName(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos, schedulerIntfs []QoSOutputQueueBinding, intfName string) *oc.Qos {
	t.Helper()
	t.Logf("QoS output interface config for %s: %v", intfName, schedulerIntfs)
	for _, intf := range schedulerIntfs {
		i := q.GetOrCreateInterface(intfName)
		i.SetInterfaceId(intfName)
		i.GetOrCreateInterfaceRef().Interface = ygot.String(intfName)
		if deviations.InterfaceRefConfigUnsupported(dut) {
			i.InterfaceRef = nil
		}
		output := i.GetOrCreateOutput()
		if intf.Scheduler != "" {
			schedulerPolicy := output.GetOrCreateSchedulerPolicy()
			schedulerPolicy.SetName(intf.Scheduler)
		}
		queue := output.GetOrCreateQueue(intf.QueueName)
		queue.SetName(intf.QueueName)
		if intf.QueueManagementProfile != "" {
			queue.SetQueueManagementProfile(intf.QueueManagementProfile)
		}
		if deviations.QOSBufferAllocationConfigRequired(dut) {
			bufferAllocation := q.GetOrCreateBufferAllocationProfile("ballocprofile")
			bq := bufferAllocation.GetOrCreateQueue(intf.QueueName)
			bq.SetStaticSharedBufferLimit(uint32(268435456))
			output.SetBufferAllocationProfile("ballocprofile")
		}
	}
	return q
}

// NewQoSQueueManagementProfile configures a WRED uniform queue-management profile on the provided oc.Qos struct,
// respecting platform deviations for min/max threshold constraints, drop/weight leaves, and weight configuration.
// It returns the updated oc.Qos struct along with the resolved minThreshold and maxThreshold values.
func NewQoSQueueManagementProfile(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos, cfg QoSQueueManagementProfile) (*oc.Qos, uint64, uint64) {
	t.Helper()
	minThreshold := cfg.MinThreshold
	maxThreshold := cfg.MaxThreshold
	if deviations.EcnSameMinMaxThresholdUnsupported(dut) {
		minThreshold = ciscoECNMinThreshold
		maxThreshold = ciscoECNMaxThreshold
	} else if deviations.QosSchedulerConfigRequired(dut) && maxThreshold == math.MaxUint64 {
		maxThreshold = fallbackECNMaxThreshold
	}

	t.Logf("QoS queue-management-profile config: %+v (resolved min=%d, max=%d)", cfg, minThreshold, maxThreshold)
	qmp := q.GetOrCreateQueueManagementProfile(cfg.ProfileName)
	qmp.SetName(cfg.ProfileName)
	wup := qmp.GetOrCreateWred().GetOrCreateUniform()
	wup.SetEnableEcn(cfg.EnableECN)
	wup.SetMinThreshold(minThreshold)
	wup.SetMaxThreshold(maxThreshold)
	wup.SetMaxDropProbabilityPercent(cfg.MaxDropProbabilityPercent)
	if !deviations.DropWeightLeavesUnsupported(dut) {
		wup.SetDrop(cfg.Drop)
		if cfg.SetWeight && !deviations.QosSetWeightConfigUnsupported(dut) {
			wup.SetWeight(cfg.Weight)
		}
	}
	return q, minThreshold, maxThreshold
}

// VerifyQoSInputClassifierState validates /qos/interfaces/interface/input/classifiers/classifier/state/name
// across the specified input bindings using gnmi.Watch (gNMI.Subscribe ON_CHANGE).
func VerifyQoSInputClassifierState(t *testing.T, dut *ondatra.DUTDevice, bindings []QoSInputClassifierBinding, timeout time.Duration) {
	t.Helper()
	if deviations.StatePathsUnsupported(dut) || deviations.QosGetStatePathUnsupported(dut) {
		return
	}
	for _, b := range bindings {
		qosIntf := b.IntfName
		if deviations.InterfaceRefInterfaceIDFormat(dut) {
			qosIntf = b.IntfName + ".0"
		}
		_, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(qosIntf).Input().Classifier(b.ClassifierType).Name().State(), timeout, func(v *ygnmi.Value[string]) bool {
			got, present := v.Val()
			return present && got == b.ClassifierName
		}).Await(t)
		if !ok {
			t.Errorf("QoS input classifier state on %s (%v): did not converge to %q", qosIntf, b.ClassifierType, b.ClassifierName)
		}
	}
}

// VerifyQoSQueueManagementProfileState validates WRED uniform queue-management profile state leaves
// using gnmi.Await (gNMI.Subscribe ON_CHANGE), respecting platform deviations.
func VerifyQoSQueueManagementProfileState(t *testing.T, dut *ondatra.DUTDevice, cfg QoSQueueManagementProfile, timeout time.Duration) {
	t.Helper()
	wantMin := cfg.MinThreshold
	wantMax := cfg.MaxThreshold
	if deviations.EcnSameMinMaxThresholdUnsupported(dut) {
		wantMin = ciscoECNMinThreshold
		wantMax = ciscoECNMaxThreshold
	} else if deviations.QosSchedulerConfigRequired(dut) && wantMax == math.MaxUint64 {
		wantMax = fallbackECNMaxThreshold
	}

	wredUniform := gnmi.OC().Qos().QueueManagementProfile(cfg.ProfileName).Wred().Uniform()
	if deviations.QosGetStatePathUnsupported(dut) || deviations.StatePathsUnsupported(dut) {
		gnmi.Await(t, dut, wredUniform.EnableEcn().Config(), timeout, cfg.EnableECN)
		gnmi.Await(t, dut, wredUniform.MinThreshold().Config(), timeout, wantMin)
		gnmi.Await(t, dut, wredUniform.MaxThreshold().Config(), timeout, wantMax)
		gnmi.Await(t, dut, wredUniform.MaxDropProbabilityPercent().Config(), timeout, cfg.MaxDropProbabilityPercent)
		return
	}

	gnmi.Await(t, dut, wredUniform.EnableEcn().State(), timeout, cfg.EnableECN)
	gnmi.Await(t, dut, wredUniform.MinThreshold().State(), timeout, wantMin)
	gnmi.Await(t, dut, wredUniform.MaxThreshold().State(), timeout, wantMax)
	gnmi.Await(t, dut, wredUniform.MaxDropProbabilityPercent().State(), timeout, cfg.MaxDropProbabilityPercent)
	if !deviations.DropWeightLeavesUnsupported(dut) {
		gnmi.Await(t, dut, wredUniform.Drop().State(), timeout, cfg.Drop)
		if cfg.SetWeight && !deviations.QosSetWeightConfigUnsupported(dut) {
			gnmi.Await(t, dut, wredUniform.Weight().State(), timeout, cfg.Weight)
		}
	}
}

// VerifyQoSOutputInterfaceState validates egress scheduler-policy state and per-queue queue-management-profile
// state on intfName using gnmi.Await (gNMI.Subscribe ON_CHANGE).
func VerifyQoSOutputInterfaceState(t *testing.T, dut *ondatra.DUTDevice, intfName, schedulerName string, schedulerIntfs []QoSOutputQueueBinding, timeout time.Duration) {
	t.Helper()
	policyPath := gnmi.OC().Qos().Interface(intfName).Output().SchedulerPolicy()
	if !deviations.StatePathsUnsupported(dut) && !deviations.QosGetStatePathUnsupported(dut) {
		gnmi.Await(t, dut, policyPath.Name().State(), timeout, schedulerName)
	} else {
		gnmi.Await(t, dut, policyPath.Name().Config(), timeout, schedulerName)
	}
	for _, sq := range schedulerIntfs {
		VerifyQoSQueueProfileBinding(t, dut, intfName, sq.QueueName, sq.QueueManagementProfile, timeout)
	}
}

// VerifyQoSQueueProfileBinding verifies that a single egress queue reports the expected queue-management-profile.
func VerifyQoSQueueProfileBinding(t *testing.T, dut *ondatra.DUTDevice, intfName, queueName, wantProfile string, timeout time.Duration) {
	t.Helper()
	outQueue := gnmi.OC().Qos().Interface(intfName).Output().Queue(queueName)
	if !deviations.StatePathsUnsupported(dut) && !deviations.QosGetStatePathUnsupported(dut) {
		gnmi.Await(t, dut, outQueue.Name().State(), timeout, queueName)
		gnmi.Await(t, dut, outQueue.QueueManagementProfile().State(), timeout, wantProfile)
	} else {
		gnmi.Await(t, dut, outQueue.Name().Config(), timeout, queueName)
		gnmi.Await(t, dut, outQueue.QueueManagementProfile().Config(), timeout, wantProfile)
	}
}

// VerifyICResourceUtilizationState validates hardware resource utilization under
// /components/component/integrated-circuit/utilization/resources/resource/state/used and free
// using gnmi.Watch (gNMI.Subscribe ON_CHANGE), respecting UseChassisAggregateUtilization.
func VerifyICResourceUtilizationState(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration) {
	t.Helper()
	compType := oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT
	if deviations.UseChassisAggregateUtilization(dut) {
		compType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CHASSIS
	}
	compNames := components.FindComponentsByType(t, dut, compType)
	if len(compNames) == 0 {
		t.Logf("No %v components reported on DUT %s; skipping IC resource utilization check", compType, dut.Name())
		return
	}
	isPresent := func(v *ygnmi.Value[uint64]) bool { return v.IsPresent() }
	for _, compName := range compNames {
		resources := gnmi.LookupAll(t, dut, gnmi.OC().Component(compName).IntegratedCircuit().Utilization().ResourceAny().State())
		for _, resVal := range resources {
			res, ok := resVal.Val()
			if !ok || res == nil || res.GetName() == "" {
				continue
			}
			rName := res.GetName()
			resPath := gnmi.OC().Component(compName).IntegratedCircuit().Utilization().Resource(rName)
			usedVal, usedOK := gnmi.Watch(t, dut, resPath.Used().State(), timeout, isPresent).Await(t)
			freeVal, freeOK := gnmi.Watch(t, dut, resPath.Free().State(), timeout, isPresent).Await(t)
			if !usedOK || !freeOK {
				t.Errorf("Component %s resource %s utilization telemetry not streaming: usedOK=%v, freeOK=%v", compName, rName, usedOK, freeOK)
				continue
			}
			used, _ := usedVal.Val()
			free, _ := freeVal.Val()
			t.Logf("Component %s resource %s utilization: used=%d, free=%d", compName, rName, used, free)
		}
	}
}

// NewQoSInputClassifierConfiguration attaches input classifiers to interfaces on the provided oc.Qos struct
// without issuing intermediate gNMI Replace calls, respecting InterfaceRefInterfaceIDFormat,
// QosSetWeightConfigUnsupported, and InterfaceRefConfigUnsupported deviations.
func NewQoSInputClassifierConfiguration(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos, bindings []QoSInputClassifierBinding) *oc.Qos {
	t.Helper()
	t.Logf("QoS input classifier bindings: %v", bindings)
	for _, b := range bindings {
		qosIntfID := b.IntfName
		if deviations.InterfaceRefInterfaceIDFormat(dut) {
			qosIntfID = b.IntfName + ".0"
		}
		intf := q.GetOrCreateInterface(qosIntfID)
		intf.SetInterfaceId(qosIntfID)
		intf.GetOrCreateInterfaceRef().SetInterface(b.IntfName)
		if !deviations.QosSetWeightConfigUnsupported(dut) {
			intf.GetOrCreateInterfaceRef().SetSubinterface(0)
		}
		if deviations.InterfaceRefConfigUnsupported(dut) {
			intf.InterfaceRef = nil
		}
		intf.GetOrCreateInput().GetOrCreateClassifier(b.ClassifierType).SetName(b.ClassifierName)
	}
	return q
}

// VerifyQoSProfileRemovedFromQueues verifies via gNMI.Get (gnmi.Lookup) and gNMI.Subscribe (gnmi.Watch)
// that /qos/interfaces/interface/output/queues/queue/state/queue-management-profile no longer returns
// removedProfile across all specified queues on intfName.
func VerifyQoSProfileRemovedFromQueues(t *testing.T, dut *ondatra.DUTDevice, intfName string, queueNames []string, removedProfile string, timeout time.Duration) {
	t.Helper()
	for _, qName := range queueNames {
		outQueue := gnmi.OC().Qos().Interface(intfName).Output().Queue(qName)
		if !deviations.StatePathsUnsupported(dut) && !deviations.QosGetStatePathUnsupported(dut) {
			if gotProfile, present := gnmi.Lookup(t, dut, outQueue.QueueManagementProfile().State()).Val(); present && gotProfile == removedProfile {
				t.Errorf("Queue %s on %s still reports queue-management-profile %q via gNMI.Get after cleanup", qName, intfName, gotProfile)
			}
			_, ok := gnmi.Watch(t, dut, outQueue.QueueManagementProfile().State(), timeout, func(v *ygnmi.Value[string]) bool {
				got, present := v.Val()
				return !present || got != removedProfile
			}).Await(t)
			if !ok {
				t.Errorf("Queue %s on %s queue-management-profile state did not clear %q via gNMI.Subscribe within %v", qName, intfName, removedProfile, timeout)
			}
		} else {
			if gotProfile, present := gnmi.Lookup(t, dut, outQueue.QueueManagementProfile().Config()).Val(); present && gotProfile == removedProfile {
				t.Errorf("Queue %s on %s still reports queue-management-profile config %q via gNMI.Get after cleanup", qName, intfName, gotProfile)
			}
			_, ok := gnmi.Watch(t, dut, outQueue.QueueManagementProfile().Config(), timeout, func(v *ygnmi.Value[string]) bool {
				got, present := v.Val()
				return !present || got != removedProfile
			}).Await(t)
			if !ok {
				t.Errorf("Queue %s on %s queue-management-profile config did not clear %q via gNMI.Subscribe within %v", qName, intfName, removedProfile, timeout)
			}
		}
	}
}

// QoSQueueCounterSnapshot stores DUT QoS output queue packet and octet counters.
type QoSQueueCounterSnapshot struct {
	TxPkts     uint64
	TxOctets   uint64
	DropPkts   uint64
	DropOctets uint64
}

// QoSQueueTrafficExpectation holds expected post-traffic packet and octet counter parameters for a single QoS output queue.
type QoSQueueTrafficExpectation struct {
	QueueName         string
	AteOutPkts        uint64
	AteInPkts         uint64
	FrameSize         uint32
	ExpectTailDrops   bool
	ExpectLossless    bool
	ToleranceFraction float64
	MinTolerancePkts  uint64
}

// SnapshotQoSQueueCounters captures transmit-pkts, transmit-octets, dropped-pkts, and dropped-octets
// on egressIntf for the specified active queues using gnmi.Watch (gNMI.Subscribe ON_CHANGE).
func SnapshotQoSQueueCounters(t *testing.T, dut *ondatra.DUTDevice, egressIntf string, activeQueues map[string]bool, timeout time.Duration) map[string]QoSQueueCounterSnapshot {
	t.Helper()
	snaps := make(map[string]QoSQueueCounterSnapshot)
	if deviations.SkipSamplingQosCounters(dut) {
		return snaps
	}
	isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }
	for qName := range activeQueues {
		qPath := gnmi.OC().Qos().Interface(egressIntf).Output().Queue(qName)
		var snap QoSQueueCounterSnapshot
		if v, ok := gnmi.Watch(t, dut, qPath.TransmitPkts().State(), timeout, isPresent).Await(t); ok {
			snap.TxPkts, _ = v.Val()
		}
		if v, ok := gnmi.Watch(t, dut, qPath.DroppedPkts().State(), timeout, isPresent).Await(t); ok {
			snap.DropPkts, _ = v.Val()
		}
		if !deviations.QOSOctets(dut) {
			if v, ok := gnmi.Watch(t, dut, qPath.TransmitOctets().State(), timeout, isPresent).Await(t); ok {
				snap.TxOctets, _ = v.Val()
			}
			if v, ok := gnmi.Watch(t, dut, qPath.DroppedOctets().State(), timeout, isPresent).Await(t); ok {
				snap.DropOctets, _ = v.Val()
			}
		}
		snaps[qName] = snap
	}
	return snaps
}

// VerifyQoSQueueCountersAfterTraffic waits via gnmi.Watch (gNMI.Subscribe ON_CHANGE) for DUT QoS queue counters
// to update and verifies transmit-pkts, transmit-octets, dropped-pkts, and dropped-octets against ATE counters.
func VerifyQoSQueueCountersAfterTraffic(t *testing.T, dut *ondatra.DUTDevice, egressIntf string, expectations []QoSQueueTrafficExpectation, before map[string]QoSQueueCounterSnapshot, timeout time.Duration) {
	t.Helper()
	if deviations.SkipSamplingQosCounters(dut) {
		return
	}
	for _, exp := range expectations {
		qPath := gnmi.OC().Qos().Interface(egressIntf).Output().Queue(exp.QueueName)
		beforeSnap := before[exp.QueueName]
		wantMinTxPkts := beforeSnap.TxPkts + exp.AteInPkts

		val, ok := gnmi.Watch(t, dut, qPath.TransmitPkts().State(), timeout, func(v *ygnmi.Value[uint64]) bool {
			pkts, present := v.Val()
			return present && pkts >= wantMinTxPkts
		}).Await(t)
		var afterTxPkts uint64
		if ok {
			afterTxPkts, _ = val.Val()
		} else {
			afterTxPkts = gnmi.Get(t, dut, qPath.TransmitPkts().State())
		}
		afterDropPkts := gnmi.Get(t, dut, qPath.DroppedPkts().State())

		txDelta := afterTxPkts - beforeSnap.TxPkts
		dropDelta := afterDropPkts - beforeSnap.DropPkts
		ateRx := exp.AteInPkts
		ateLoss := uint64(0)
		if exp.AteOutPkts > ateRx {
			ateLoss = exp.AteOutPkts - ateRx
		}
		t.Logf("QoS queue %s on %s packets: txDelta=%d (ateRx=%d), dropDelta=%d (ateLoss=%d)", exp.QueueName, egressIntf, txDelta, ateRx, dropDelta, ateLoss)

		txTol := uint64(float64(ateRx)*exp.ToleranceFraction) + exp.MinTolerancePkts
		if txDelta < ateRx || txDelta > ateRx+txTol {
			t.Errorf("Queue %s on %s transmit-pkts delta: got %d, want %d (+0..%d)", exp.QueueName, egressIntf, txDelta, ateRx, txTol)
		}
		if exp.ExpectTailDrops {
			if dropDelta == 0 && !deviations.DequeueDeleteNotCountedAsDrops(dut) {
				t.Errorf("Queue %s on %s dropped-pkts delta: got 0, want > 0 for tail-dropped traffic", exp.QueueName, egressIntf)
			}
			dropTol := uint64(float64(ateLoss)*exp.ToleranceFraction) + exp.MinTolerancePkts
			if !deviations.DequeueDeleteNotCountedAsDrops(dut) && ateLoss > dropTol && (dropDelta+dropTol < ateLoss || dropDelta > ateLoss+dropTol) {
				t.Errorf("Queue %s on %s dropped-pkts delta: got %d, want %d +/- %d", exp.QueueName, egressIntf, dropDelta, ateLoss, dropTol)
			}
		}
		if exp.ExpectLossless && dropDelta > 0 {
			t.Errorf("Queue %s on %s dropped-pkts delta: got %d, want 0 for lossless traffic", exp.QueueName, egressIntf, dropDelta)
		}

		if !deviations.QOSOctets(dut) {
			frameBytes := uint64(exp.FrameSize)
			if frameBytes == 0 {
				frameBytes = 1000
			}
			wantMinTxOctets := beforeSnap.TxOctets + ateRx*frameBytes
			octVal, octOK := gnmi.Watch(t, dut, qPath.TransmitOctets().State(), timeout, func(v *ygnmi.Value[uint64]) bool {
				octs, present := v.Val()
				return present && octs >= wantMinTxOctets
			}).Await(t)
			var afterTxOctets uint64
			if octOK {
				afterTxOctets, _ = octVal.Val()
			} else if v, present := gnmi.Lookup(t, dut, qPath.TransmitOctets().State()).Val(); present {
				afterTxOctets = v
			}
			afterDropOctets, _ := gnmi.Lookup(t, dut, qPath.DroppedOctets().State()).Val()

			txOctetsDelta := afterTxOctets - beforeSnap.TxOctets
			dropOctetsDelta := afterDropOctets - beforeSnap.DropOctets
			t.Logf("QoS queue %s on %s octets: txOctetsDelta=%d, dropOctetsDelta=%d", exp.QueueName, egressIntf, txOctetsDelta, dropOctetsDelta)

			if ateRx > 0 && txOctetsDelta == 0 {
				t.Errorf("Queue %s on %s transmit-octets delta: got 0, want > 0 matching ATE Rx octets", exp.QueueName, egressIntf)
			}
			if exp.ExpectLossless && dropOctetsDelta > 0 {
				t.Errorf("Queue %s on %s dropped-octets delta: got %d, want 0 for lossless traffic", exp.QueueName, egressIntf, dropOctetsDelta)
			}
			if exp.ExpectTailDrops && dropOctetsDelta == 0 && !deviations.DequeueDeleteNotCountedAsDrops(dut) {
				t.Errorf("Queue %s on %s dropped-octets delta: got 0, want > 0 for tail-dropped traffic", exp.QueueName, egressIntf)
			}
		}
	}
}

// NewQoSQueuesFromList creates the given queues on the oc.Qos struct and assigns descending QueueId values
// when deviations.QOSQueueRequiresID(dut) is true.
func NewQoSQueuesFromList(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos, queueNames []string) *oc.Qos {
	t.Helper()
	for i, queue := range queueNames {
		q1 := q.GetOrCreateQueue(queue)
		q1.SetName(queue)
		if deviations.QOSQueueRequiresID(dut) {
			q1.SetQueueId(uint8(len(queueNames) - i))
		}
	}
	return q
}
