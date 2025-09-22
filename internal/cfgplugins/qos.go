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
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

type QosClassifier struct {
	Desc        string
	Name        string
	ClassType   oc.E_Qos_Classifier_Type
	TermID      string
	TargetGroup string
	DscpSet     []uint8
	RemarkDscp  uint8
}

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

type ForwardingGroup struct {
	Desc        string
	QueueName   string
	TargetGroup string
	Priority    uint8
}

type QoSSchedulerInterface struct {
	Desc      string
	QueueName string
	Scheduler string
}

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

func NewQosInitialize(t *testing.T, dut *ondatra.DUTDevice) {
	if dut.Vendor() == ondatra.ARISTA {
		queues := netutil.CommonTrafficQueues(t, dut)
		qosQNameSet := `
	configure terminal
	!
	qos tx-queue %d name %s
	!
	`
		qosMapTC := `
	configure terminal
	!
	qos map traffic-class %d to tx-queue %d
	!
	`

		qosCfgTargetGroup := `
	configure terminal
	!
	qos traffic-class %d name %s
	!
	`

		runCliCommand(t, dut, "show version")

		qList := []string{queues.BE1, queues.AF1, queues.AF2, queues.AF3, queues.AF4, queues.NC1}
		for index, queue := range qList {

			runCliCommand(t, dut, fmt.Sprintf(qosQNameSet, index, queue))
			time.Sleep(time.Second)
			runCliCommand(t, dut, fmt.Sprintf(qosMapTC, index, index))
			time.Sleep(time.Second)
			runCliCommand(t, dut, fmt.Sprintf(qosCfgTargetGroup, index, fmt.Sprintf("target-group-%s", queue)))
			time.Sleep(time.Second)
		}
	}
}

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
			if class.Name == "dscp_based_classifier_ipv4" {
				condition.GetOrCreateIpv4().SetDscpSet(class.DscpSet)
			} else {
				condition.GetOrCreateIpv6().SetDscpSet(class.DscpSet)
			}
		}
		if !deviations.QosRemarkOCUnsupported(dut) {
			action.GetOrCreateRemark().SetDscp = ygot.Uint8(class.RemarkDscp)
		}
	}
	return q
}

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

func NewQoSForwardingGroup(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos, forwardingGroups []ForwardingGroup) {
	t.Logf("QoS forwarding groups config: %v", forwardingGroups)
	for _, fg := range forwardingGroups {
		qoscfg.SetForwardingGroup(t, dut, q, fg.TargetGroup, fg.QueueName)
	}
}

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
	trtc.GetOrCreateExceedAction().Drop = ygot.Bool(false)
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

func NewTwoRateThreeColorScheduler(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, params *SchedulerParams) {
	if deviations.QosTwoRateThreeColorPolicerOCUnsupported(dut) {
		configureTwoRateThreeColorSchedulerFromCLI(t, dut, params)
	} else {
		configureTwoRateThreeColorSchedulerFromOC(batch, params)
	}
}

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

func ConfigureQosDscpRemarkSpecific(t *testing.T, dut *ondatra.DUTDevice) {
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
