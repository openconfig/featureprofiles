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
		action.SetTargetGroup(class.TargetGroup)
		if len(class.DscpSet) > 0 {
			condition := term.GetOrCreateConditions()
			condition.GetOrCreateIpv4().SetDscpSet(class.DscpSet)
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

func CreateQueues(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, queues []string) {
	for index, q := range queues {
		queue := qos.GetOrCreateQueue(q)
		queue.Name = ygot.String(q)
		if deviations.QOSQueueRequiresID(dut) {
			queue.QueueId = ygot.Uint8(uint8(index))
		}
	}
}

func CreateQueuesFromCli(t *testing.T, dut *ondatra.DUTDevice, queues []string) {
	cliConfig := ""
	for index, queue := range queues {
		cliConfig += fmt.Sprintf(`
		qos tx-queue %d name %s
		!`, index, queue)
	}
	helpers.GnmiCLIConfig(t, dut, cliConfig)
}

func configureTwoRateThreeColorSchedulerFromOC(qos *oc.Qos, schedulerName, policerName string, cirValue, pirValue uint64, burstSize, sequenceNumber uint32, queueName string) {
	sp := qos.GetOrCreateSchedulerPolicy(schedulerName)
	sp.Name = ygot.String(schedulerName)
	sched := sp.GetOrCreateScheduler(sequenceNumber)
	sched.Sequence = ygot.Uint32(1)
	sched.Type = oc.QosTypes_QOS_SCHEDULER_TYPE_TWO_RATE_THREE_COLOR
	input := sched.GetOrCreateInput(policerName)
	input.InputType = oc.Input_InputType_QUEUE
	input.Queue = ygot.String(queueName)
	trtc := sched.GetOrCreateTwoRateThreeColor()
	trtc.Cir = ygot.Uint64(cirValue)
	trtc.Pir = ygot.Uint64(pirValue)
	trtc.Bc = ygot.Uint32(burstSize)
	trtc.Be = ygot.Uint32(burstSize)
	trtc.GetOrCreateExceedAction().Drop = ygot.Bool(false)
	trtc.GetOrCreateViolateAction().Drop = ygot.Bool(true)
}

func configureTwoRateThreeColorSchedulerFromCLI(t *testing.T, dut *ondatra.DUTDevice, schedulerName, className string, cirValue, pirValue uint64, burstSize, queueId uint32) {
	cliConfig := fmt.Sprintf(`
policy-map type quality-of-service %s
   class %s
   set traffic-class %d 
   police rate %d bps burst-size %d bytes rate %d bps burst-size %d bytes
!
`, schedulerName, className, queueId, cirValue, burstSize, pirValue, burstSize)

	helpers.GnmiCLIConfig(t, dut, cliConfig)
}

func NewTwoRateThreeColorScheduler(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, schedulerName, policerName, className string, cirValue, pirValue uint64, burstSize, sequenceNumber, queueId uint32, queueName string) {
	if deviations.QosTwoRateThreeColorPolicerOCUnsupported(dut) {
		configureTwoRateThreeColorSchedulerFromCLI(t, dut, schedulerName, className, cirValue, pirValue, burstSize, queueId)
	} else {
		configureTwoRateThreeColorSchedulerFromOC(qos, schedulerName, policerName, cirValue, pirValue, burstSize, sequenceNumber, queueName)
	}
}

func ApplyQosPolicyOnInterface(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, interfaceName, policyName string) {
	if deviations.QosSchedulerIngressPolicer(dut) {
		applyQosPolicyOnInterfaceFromCLI(t, dut, interfaceName, policyName)
	} else {
		applyQosPolicyOnInterfaceFromOC(qos, interfaceName, policyName)
	}
}

func applyQosPolicyOnInterfaceFromOC(qos *oc.Qos, intfName, policyName string) {
	intf := qos.GetOrCreateInterface(intfName)
	intf.InterfaceId = ygot.String(intfName)
	in := intf.GetOrCreateInput()
	in.GetOrCreateSchedulerPolicy().Name = ygot.String(policyName)
}

func applyQosPolicyOnInterfaceFromCLI(t *testing.T, dut *ondatra.DUTDevice, intfName, policyName string) {
	cliConfig := fmt.Sprintf(`
interface %s
   service-policy type qos input %s
!
`, intfName, policyName)
	helpers.GnmiCLIConfig(t, dut, cliConfig)
}

func GetPolicyCLICounters(t *testing.T, dut *ondatra.DUTDevice, policyName string) string {
	cliConfig := fmt.Sprintf("show policy-map type qos %s counters", policyName)
	return runCliCommand(t, dut, cliConfig)
}
