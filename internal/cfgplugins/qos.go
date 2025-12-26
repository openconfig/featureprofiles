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
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
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

func configureTwoRateThreeColorSchedulerFromOC(batch *gnmi.SetBatch, params *SchedulerParams) {
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
	trtc.Bc = ygot.Uint32(params.BurstSize)
	trtc.Be = ygot.Uint32(params.BurstSize)
	trtc.GetOrCreateExceedAction().Drop = ygot.Bool(true)
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
		configureTwoRateThreeColorSchedulerFromOC(batch, params)
	}
}

// ApplyQosPolicyOnInterface applies a QoS policy on the specified interface.
// The configuration is applied using either CLI or OC based on the QosSchedulerIngressPolicer deviation.
func ApplyQosPolicyOnInterface(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, params *SchedulerParams) {
	if deviations.QosSchedulerIngressPolicer(dut) {
		applyQosPolicyOnInterfaceFromCLI(t, dut, params)
	} else {
		applyQosPolicyOnInterfaceFromOC(batch, params)
	}
}

func applyQosPolicyOnInterfaceFromOC(batch *gnmi.SetBatch, params *SchedulerParams) {
	qos := &oc.Qos{}
	intf := qos.GetOrCreateInterface(params.InterfaceName)
	intf.InterfaceId = ygot.String(params.InterfaceName)
	in := intf.GetOrCreateInput()
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
