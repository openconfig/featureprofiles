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
	for _, tc := range classifiers {
		t.Log(tc.Desc)
		classifier := q.GetOrCreateClassifier(tc.Name)
		classifier.SetName(tc.Name)
		classifier.SetType(tc.ClassType)

		term, err := classifier.NewTerm(tc.TermID)
		if err != nil {
			t.Fatalf("Failed to create classifier.NewTerm(): %v", err)
		}

		term.SetId(tc.TermID)
		action := term.GetOrCreateActions()
		if tc.TargetGroup != "" {
			action.SetTargetGroup(tc.TargetGroup)
		}
		if len(tc.DscpSet) > 0 {
			condition := term.GetOrCreateConditions()
			condition.GetOrCreateIpv4().SetDscpSet(tc.DscpSet)
		}
		if tc.RemarkDscp != 0 {
			action.GetOrCreateRemark().SetDscp = ygot.Uint8(tc.RemarkDscp)
		}
	}
	return q
}

func NewQoSSchedulerPolicy(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos, policies []SchedulerPolicy) *oc.Qos {

	t.Logf("QoS scheduler policy config: %v", policies)
	schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
	schedulerPolicy.SetName("scheduler")
	t.Logf("QoS scheduler policies config: %v", policies)
	for _, tc := range policies {
		s := schedulerPolicy.GetOrCreateScheduler(tc.Sequence)
		s.SetSequence(tc.Sequence)
		if tc.SetPriority {
			s.SetPriority(tc.Priority)
		}
		input := s.GetOrCreateInput(tc.InputID)
		input.SetId(tc.InputID)
		input.SetInputType(tc.InputType)
		input.SetQueue(tc.QueueName)
	}
	return q
}

func NewQoSForwardingGroup(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos, forwardingGroups []ForwardingGroup) {
	t.Logf("QoS forwarding groups config: %v", forwardingGroups)
	for _, tc := range forwardingGroups {
		qoscfg.SetForwardingGroup(t, dut, q, tc.TargetGroup, tc.QueueName)
	}
}

func NewQoSSchedulerInterface(t *testing.T, dut *ondatra.DUTDevice, q *oc.Qos, schedulerIntfs []QoSSchedulerInterface, schedulerPort string) *oc.Qos {
	t.Logf("QoS output interface config: %v", schedulerIntfs)
	schPort := dut.Port(t, schedulerPort)
	for _, tc := range schedulerIntfs {
		i := q.GetOrCreateInterface(schPort.Name())
		i.SetInterfaceId(schPort.Name())
		i.GetOrCreateInterfaceRef().Interface = ygot.String(schPort.Name())
		if deviations.InterfaceRefConfigUnsupported(dut) {
			i.InterfaceRef = nil
		}
		output := i.GetOrCreateOutput()
		schedulerPolicy := output.GetOrCreateSchedulerPolicy()
		schedulerPolicy.SetName(tc.Scheduler)
		queue := output.GetOrCreateQueue(tc.QueueName)
		queue.SetName(tc.QueueName)
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
	qosConfig := qosClassifierRemarkDscpCliConfig(dut, classifierName, interfaceName, ipv4DscpValues, ipv6DscpValues)
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
func PushQosClassifierToDUT(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, interfaceName, classifierName string, isInputClassifier bool) {
	if !deviations.QosRemarkOCUnsupported(dut) {
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), qos)
		if isInputClassifier {
			qoscfg.SetInputClassifier(t, dut, qos, interfaceName, oc.Input_Classifier_Type_IPV4, classifierName)
		}
	}
}

func qosClassifierRemarkDscpCliConfig(dut *ondatra.DUTDevice, classifierName string, interfaceName string, ipv4DscpValues []uint8, ipv6DscpValues []uint8) string {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		return `
        qos rewrite dscp
        !
        `
		// cliConfig := fmt.Sprintf("qos map dscp-classifier %s\n", classifierName)
		// for i, dscp := range ipv4DscpValues {
		//  cliConfig += fmt.Sprintf(`
		//  dscp %d class termV4-%d
		//  qos map dscp-remark remarkV4-%d
		//  match dscp %d
		//  set dscp %d
		//  !
		//  `, dscp, i+1, i+1, dscp, dscp)
		// }

		// // Add terms for IPv6 DSCP values
		// for i, dscp := range ipv6DscpValues {
		//  cliConfig += fmt.Sprintf(`
		//  dscp %d class termV6-%d
		//  qos map dscp-remark remarkV6-%d
		//  match dscp %d
		//  set dscp %d
		//  !
		//  `, dscp, i+1, i+1, dscp, dscp)
		// }

		// // Apply the classifier to the interface
		// cliConfig += fmt.Sprintf(`
		// interface %s
		// service-policy input %s
		// !
		// `, interfaceName, classifierName)

		//return cliConfig
	default:
		return ""
	}
}
