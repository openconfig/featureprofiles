// Copyright 2022 Google LLC
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

package qos_policy_config_test

import (
	"math"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// QoS policy OC config:
//  - forwarding-groups:
//    - /qos/forwarding-groups/forwarding-group/config/name
//    - /qos/forwarding-groups/forwarding-group/config/output-queue
//    - /qos/queues/queue/config/name
//  - classifiers:
//    - /qos/classifiers/classifier/config/name
//    - /qos/classifiers/classifier/config/type
//    - /qos/classifiers/classifier/terms/term/actions/config/target-group
//    - /qos/classifiers/classifier/terms/term/conditions/ipv4/config/dscp-set
//    - /qos/classifiers/classifier/terms/term/conditions/ipv6/config/dscp-set
//    - /qos/classifiers/classifier/terms/term/config/id
//  - classifiers on input interface:
//    - /qos/interfaces/interface/interface-id,
//    - /qos/interfaces/interface/config/interface-id
//    - /qos/interfaces/interface/input/classifiers/classifier/type
//    - /qos/interfaces/interface/input/classifiers/classifier/config/type
//    - /qos/interfaces/interface/input/classifiers/classifier/config/name
//  - scheduler-policies:
//    - /qos/scheduler-policies/scheduler-policy/config/name
//    - /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/priority
//    - /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/config/sequence
//    - /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/id
//    - /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/input-type
//    - /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/queue
//    - /qos/scheduler-policies/scheduler-policy/schedulers/scheduler/inputs/input/config/weight
//  - ECN queue-management-profiles:
//    - /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/min-threshold
//    - /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-threshold
//    - /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/enable-ecn
//    - /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/weight
//    - /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/drop
//    - /qos/queue-management-profiles/queue-management-profile/wred/uniform/config/max-drop-probability-percent
//  - Output interface queue and scheduler-policy:
//    - /qos/interfaces/interface/output/queues/queue/config/queue-management-profile
//    - /qos/interfaces/interface/output/queues/queue/config/name
//    - /qos/interfaces/interface/output/scheduler-policy/config/name
//
//
// Topology:
//   ate:port1 <--> port1:dut:port2 <--> ate:port2
//
// Test notes:
//
//  Sample CLI command to get telemetry using gmic:
//   - gnmic -a ipaddr:10162 -u username -p password --skip-verify get \
//      --path /components/component --format flat
//   - gnmic tool info:
//     - https://github.com/karimra/gnmic/blob/main/README.md
//

const (
	dropProfile               = "DropProfile"
	forwardingGroup           = "fc"
	schedulerMap              = "smap"
	sequenceID                = 1
	schedulerPriority         = 1
	queueName                 = "0"
	minThresholdPercent       = 10
	maxThresholdPercent       = 70
	maxDropProbabilityPercent = 1
	enableECN                 = true
)

func TestQoSForwadingGroupsConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	q := d.GetOrCreateQos()

	cases := []struct {
		desc        string
		queueName   string
		targetGroup string
	}{{
		desc:        "forwarding-group-BE1",
		queueName:   "0",
		targetGroup: "target-group-BE1",
	}, {
		desc:        "forwarding-group-BE0",
		queueName:   "1",
		targetGroup: "target-group-BE0",
	}, {
		desc:        "forwarding-group-AF1",
		queueName:   "2",
		targetGroup: "target-group-AF1",
	}, {
		desc:        "forwarding-group-AF2",
		queueName:   "3",
		targetGroup: "target-group-AF2",
	}, {
		desc:        "forwarding-group-AF3",
		queueName:   "4",
		targetGroup: "target-group-AF3",
	}, {
		desc:        "forwarding-group-AF4",
		queueName:   "5",
		targetGroup: "target-group-AF4",
	}, {
		desc:        "forwarding-group-NC1",
		queueName:   "6",
		targetGroup: "target-group-NC1",
	}}

	t.Logf("qos forwarding groups config cases: %v", cases)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			fwdGroup := q.GetOrCreateForwardingGroup(tc.targetGroup)
			fwdGroup.SetName(tc.targetGroup)
			fwdGroup.SetOutputQueue(tc.queueName)
			queue := q.GetOrCreateQueue(tc.queueName)
			queue.SetName(tc.queueName)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})

		// Verify the ForwardingGroup is applied by checking the telemetry path state values.
		forwardingGroup := gnmi.OC().Qos().ForwardingGroup(tc.targetGroup)
		if got, want := gnmi.Get(t, dut, forwardingGroup.Name().State()), tc.targetGroup; got != want {
			t.Errorf("forwardingGroup.Name().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, forwardingGroup.OutputQueue().State()), tc.queueName; got != want {
			t.Errorf("forwardingGroup.OutputQueue().State(): got %v, want %v", got, want)
		}
	}
}

func TestQoSClassifierConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	q := d.GetOrCreateQos()

	cases := []struct {
		desc        string
		name        string
		classType   oc.E_Qos_Classifier_Type
		termID      string
		targetGroup string
		dscpSet     []uint8
		queueName   string
	}{{
		desc:        "classifier_ipv4_be1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "0",
		targetGroup: "target-group-BE1",
		dscpSet:     []uint8{0, 1, 2, 3},
		queueName:   "0",
	}, {
		desc:        "classifier_ipv4_be0",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "1",
		targetGroup: "target-group-BE0",
		dscpSet:     []uint8{4, 5, 6, 7},
		queueName:   "1",
	}, {
		desc:        "classifier_ipv4_af1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "2",
		targetGroup: "target-group-AF1",
		dscpSet:     []uint8{8, 9, 10, 11},
		queueName:   "2",
	}, {
		desc:        "classifier_ipv4_af2",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "3",
		targetGroup: "target-group-AF2",
		dscpSet:     []uint8{16, 17, 18, 19},
		queueName:   "3",
	}, {
		desc:        "classifier_ipv4_af3",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "4",
		targetGroup: "target-group-AF3",
		dscpSet:     []uint8{24, 25, 26, 27},
		queueName:   "4",
	}, {
		desc:        "classifier_ipv4_af4",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "5",
		targetGroup: "target-group-AF4",
		dscpSet:     []uint8{32, 33, 34, 35},
		queueName:   "5",
	}, {
		desc:        "classifier_ipv4_nc1",
		name:        "dscp_based_classifier_ipv4",
		classType:   oc.Qos_Classifier_Type_IPV4,
		termID:      "6",
		targetGroup: "target-group-NC1",
		dscpSet:     []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59},
		queueName:   "6",
	}, {
		desc:        "classifier_ipv6_be1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "0",
		targetGroup: "target-group-BE1",
		dscpSet:     []uint8{0, 1, 2, 3},
		queueName:   "0",
	}, {
		desc:        "classifier_ipv6_be0",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "1",
		targetGroup: "target-group-BE0",
		dscpSet:     []uint8{4, 5, 6, 7},
		queueName:   "1",
	}, {
		desc:        "classifier_ipv6_af1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "2",
		targetGroup: "target-group-AF1",
		dscpSet:     []uint8{8, 9, 10, 11},
		queueName:   "2",
	}, {
		desc:        "classifier_ipv6_af2",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "3",
		targetGroup: "target-group-AF2",
		dscpSet:     []uint8{16, 17, 18, 19},
		queueName:   "3",
	}, {
		desc:        "classifier_ipv6_af3",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "4",
		targetGroup: "target-group-AF3",
		dscpSet:     []uint8{24, 25, 26, 27},
		queueName:   "4",
	}, {
		desc:        "classifier_ipv6_af4",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "5",
		targetGroup: "target-group-AF4",
		dscpSet:     []uint8{32, 33, 34, 35},
		queueName:   "5",
	}, {
		desc:        "classifier_ipv6_nc1",
		name:        "dscp_based_classifier_ipv6",
		classType:   oc.Qos_Classifier_Type_IPV6,
		termID:      "6",
		targetGroup: "target-group-NC1",
		dscpSet:     []uint8{48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59},
		queueName:   "6",
	}}

	t.Logf("qos Classifiers config cases: %v", cases)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			classifier := q.GetOrCreateClassifier(tc.name)
			classifier.SetName(tc.name)
			classifier.SetType(tc.classType)
			term, err := classifier.NewTerm(tc.termID)
			if err != nil {
				t.Fatalf("Failed to create classifier.NewTerm(): %v", err)
			}

			term.SetId(tc.termID)
			action := term.GetOrCreateActions()
			action.SetTargetGroup(tc.targetGroup)
			condition := term.GetOrCreateConditions()
			if tc.name == "dscp_based_classifier_ipv4" {
				condition.GetOrCreateIpv4().SetDscpSet(tc.dscpSet)
			} else if tc.name == "dscp_based_classifier_ipv6" {
				condition.GetOrCreateIpv6().SetDscpSet(tc.dscpSet)
			}
			t.Logf("Forwarding group config required for binding to a classifier")
			fwdGroup := q.GetOrCreateForwardingGroup(tc.targetGroup)
			fwdGroup.SetName(tc.targetGroup)
			fwdGroup.SetOutputQueue(tc.queueName)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})

		// Verify the Classifier is applied by checking the telemetry path state values.
		classifier := gnmi.OC().Qos().Classifier(tc.name)
		term := classifier.Term(tc.termID)
		action := term.Actions()
		condition := term.Conditions()

		cmp.Equal([]uint8{1, 2, 3}, []uint8{1, 2, 3})
		cmp.Equal([]uint8{1, 2, 3}, []uint8{1, 3, 2})

		if got, want := gnmi.Get(t, dut, classifier.Name().State()), tc.name; got != want {
			t.Errorf("classifier.Name().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, classifier.Type().State()), tc.classType; got != want {
			t.Errorf("classifier.Name().Type(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, term.Id().State()), tc.termID; got != want {
			t.Errorf("term.Id().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, action.TargetGroup().State()), tc.targetGroup; got != want {
			t.Errorf("action.TargetGroup().State(): got %v, want %v", got, want)
		}

		// This Transformer sorts a []uint8.
		trans := cmp.Transformer("Sort", func(in []uint8) []uint8 {
			out := append([]uint8(nil), in...)
			sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
			return out
		})

		if tc.name == "dscp_based_classifier_ipv4" {
			if equal := cmp.Equal(condition.Ipv4().DscpSet().State(), tc.dscpSet, trans); !equal {
				t.Errorf("condition.Ipv4().DscpSet().State(): got %v, want %v", condition.Ipv4().DscpSet().State(), tc.dscpSet)
			}
		} else if tc.name == "dscp_based_classifier_ipv6" {
			if equal := cmp.Equal(condition.Ipv6().DscpSet().State(), tc.dscpSet, trans); !equal {
				t.Errorf("condition.Ipv4().DscpSet().State(): got %v, want %v", condition.Ipv6().DscpSet().State(), tc.dscpSet)
			}
		}
	}
}

func TestQoSInputIntfClassifierConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port1")

	cases := []struct {
		desc                string
		inputClassifierType oc.E_Input_Classifier_Type
		classifier          string
		classType           oc.E_Qos_Classifier_Type
		termID              string
		dscpSet             []uint8
		targetGroup         string
		queueName           string
	}{{
		desc:                "Input Classifier Type IPV4",
		inputClassifierType: oc.Input_Classifier_Type_IPV4,
		classifier:          "dscp_based_classifier_ipv4",
		classType:           oc.Qos_Classifier_Type_IPV4,
		dscpSet:             []uint8{0, 1, 2, 3},
		termID:              "0",
		targetGroup:         "target-group-BE1",
		queueName:           "0",
	}, {
		desc:                "Input Classifier Type IPV6",
		inputClassifierType: oc.Input_Classifier_Type_IPV6,
		classifier:          "dscp_based_classifier_ipv6",
		classType:           oc.Qos_Classifier_Type_IPV6,
		termID:              "0",
		targetGroup:         "target-group-BE1",
		dscpSet:             []uint8{0, 1, 2, 3},
		queueName:           "0",
	}}

	d := &oc.Root{}
	q := d.GetOrCreateQos()
	i := q.GetOrCreateInterface(dp.Name())
	i.SetInterfaceId(dp.Name())
	t.Logf("Interface config required to bind a classifier with an interface")
	i.GetOrCreateInterfaceRef().Interface = ygot.String(dp.Name())
	i.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	ip := &oc.Interface{}

	t.Logf("qos input classifier config cases: %v", cases)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Logf("Classifier config required to bind with an interface")
			classifier := q.GetOrCreateClassifier(tc.classifier)
			classifier.SetName(tc.classifier)
			classifier.SetType(tc.classType)
			term, err := classifier.NewTerm(tc.termID)
			if err != nil {
				t.Fatalf("Failed to create classifier.NewTerm(): %v", err)
			}
			term.SetId(tc.termID)
			action := term.GetOrCreateActions()
			action.SetTargetGroup(tc.targetGroup)
			condition := term.GetOrCreateConditions()
			if tc.classType == oc.Qos_Classifier_Type_IPV4 {
				s := ip.GetOrCreateSubinterface(0).GetOrCreateIpv4()
				s.Enabled = ygot.Bool(true)
				condition.GetOrCreateIpv4().SetDscpSet(tc.dscpSet)
			} else if tc.classType == oc.Qos_Classifier_Type_IPV6 {
				s := ip.GetOrCreateSubinterface(0).GetOrCreateIpv6()
				s.Enabled = ygot.Bool(true)
				condition.GetOrCreateIpv6().SetDscpSet(tc.dscpSet)
			}
			fwdGroup := q.GetOrCreateForwardingGroup(tc.targetGroup)
			fwdGroup.SetName(tc.targetGroup)
			fwdGroup.SetOutputQueue(tc.queueName)
			c := i.GetOrCreateInput().GetOrCreateClassifier(tc.inputClassifierType)
			c.SetType(tc.inputClassifierType)
			c.SetName(tc.classifier)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})

		// Verify the Classifier is applied on interface by checking the telemetry path state values.
		classifier := gnmi.OC().Qos().Interface(dp.Name()).Input().Classifier(tc.inputClassifierType)
		if got, want := gnmi.Get(t, dut, classifier.Name().State()), tc.classifier; got != want {
			t.Errorf("classifier.Name().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, classifier.Type().State()), tc.inputClassifierType; got != want {
			t.Errorf("classifier.Name().State(): got %v, want %v", got, want)
		}
	}
}

func TestSchedulerPoliciesConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	q := d.GetOrCreateQos()

	cases := []struct {
		desc        string
		sequence    uint32
		priority    oc.E_Scheduler_Priority
		inputID     string
		inputType   oc.E_Input_InputType
		weight      uint64
		queueName   string
		targetGroup string
	}{{
		desc:        "scheduler-policy-BE1",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "BE1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(1),
		queueName:   "BE1",
		targetGroup: "target-group-BE1",
	}, {
		desc:        "scheduler-policy-BE0",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "BE0",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(2),
		queueName:   "BE0",
		targetGroup: "target-group-BE0",
	}, {
		desc:        "scheduler-policy-AF1",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(4),
		queueName:   "AF1",
		targetGroup: "target-group-AF1",
	}, {
		desc:        "scheduler-policy-AF2",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF2",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(8),
		queueName:   "AF2",
		targetGroup: "target-group-AF2",
	}, {
		desc:        "scheduler-policy-AF3",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF3",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(16),
		queueName:   "AF3",
		targetGroup: "target-group-AF3",
	}, {
		desc:        "scheduler-policy-AF4",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF4",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(100),
		queueName:   "AF4",
		targetGroup: "target-group-AF4",
	}, {
		desc:        "scheduler-policy-NC1",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "NC1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(200),
		queueName:   "NC1",
		targetGroup: "target-group-NC1",
	}}

	schedulerPolicy := q.GetOrCreateSchedulerPolicy(schedulerMap)
	schedulerPolicy.SetName(schedulerMap)
	t.Logf("qos scheduler policies config cases: %v", cases)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			s := schedulerPolicy.GetOrCreateScheduler(tc.sequence)
			s.SetSequence(tc.sequence)
			s.SetPriority(tc.priority)
			input := s.GetOrCreateInput(tc.inputID)
			input.SetId(tc.inputID)
			input.SetInputType(tc.inputType)
			input.SetQueue(tc.queueName)
			input.SetWeight(tc.weight)
			t.Logf("Forwarding group config is required to bind with the scheduler")
			Output := s.GetOrCreateOutput()
			Output.SetOutputFwdGroup(tc.targetGroup)
			fwdGroup := q.GetOrCreateForwardingGroup(tc.targetGroup)
			fwdGroup.SetName(tc.targetGroup)
			fwdGroup.SetOutputQueue(tc.queueName)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})

		// Verify the SchedulerPolicy is applied by checking the telemetry path state values.
		scheduler := gnmi.OC().Qos().SchedulerPolicy(schedulerMap).Scheduler(tc.sequence)
		input := scheduler.Input(tc.inputID)

		if got, want := gnmi.Get(t, dut, scheduler.Sequence().State()), tc.sequence; got != want {
			t.Errorf("scheduler.Sequence().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, scheduler.Priority().State()), tc.priority; got != want {
			t.Errorf("scheduler.Priority().State(): got %v, want %v", got, want)
		}
		if dut.Vendor() != ondatra.JUNIPER {
			if got, want := gnmi.Get(t, dut, input.Id().State()), tc.inputID; got != want {
				t.Errorf("input.Id().State(): got %v, want %v", got, want)
			}
			if got, want := gnmi.Get(t, dut, input.InputType().State()), tc.inputType; got != want {
				t.Errorf("input.InputType().State(): got %v, want %v", got, want)
			}
			if got, want := gnmi.Get(t, dut, input.Weight().State()), tc.weight; got != want {
				t.Errorf("input.Weight().State(): got %v, want %v", got, want)
			}
			if got, want := gnmi.Get(t, dut, input.Queue().State()), tc.queueName; got != want {
				t.Errorf("input.Queue().State(): got %v, want %v", got, want)
			}
		}
	}
}

func TestECNConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	q := d.GetOrCreateQos()

	if dut.Vendor() == ondatra.JUNIPER {
		t.Logf("Interface config required for queue management profile")
		dp := dut.Port(t, "port2")
		i := q.GetOrCreateInterface(dp.Name())
		i.SetInterfaceId(dp.Name())
		iref := i.GetOrCreateInterfaceRef()
		iref.SetInterface(dp.Name())

		t.Logf("Forwarding group config required for queue management profile")
		fwdGroup := q.GetOrCreateForwardingGroup(forwardingGroup)
		fwdGroup.SetName(forwardingGroup)
		fwdGroup.SetOutputQueue(queueName)

		t.Logf("Scheduler config required for binding with queue management profile")
		schedulerPolicy := q.GetOrCreateSchedulerPolicy(schedulerMap)
		schedulerPolicy.SetName(schedulerMap)
		s := schedulerPolicy.GetOrCreateScheduler(sequenceID)
		s.SetSequence(sequenceID)
		s.SetPriority(schedulerPriority)
		Output := s.GetOrCreateOutput()
		Output.SetOutputFwdGroup(forwardingGroup)
		output := i.GetOrCreateOutput()
		queue := output.GetOrCreateQueue(forwardingGroup)
		queue.SetQueueManagementProfile(dropProfile)
		queue.SetName(forwardingGroup)

	}
	var minThresholdValue, maxThresholdValue uint64
	switch dut.Vendor() {
	case ondatra.JUNIPER:
		t.Logf("Minthreshold and Maxthreshold values are expressed in percentages")
		minThresholdValue = uint64(10)
		maxThresholdValue = uint64(70)
	default:
		minThresholdValue = uint64(80000)
		maxThresholdValue = math.MaxUint64
	}
	ecnConfig := struct {
		ecnEnabled                bool
		dropEnabled               bool
		minThreshold              uint64
		maxThreshold              uint64
		maxDropProbabilityPercent uint8
		weight                    uint32
	}{
		ecnEnabled:                true,
		dropEnabled:               false,
		minThreshold:              minThresholdValue,
		maxThreshold:              maxThresholdValue,
		maxDropProbabilityPercent: uint8(1),
		weight:                    uint32(0),
	}

	queueMgmtProfile := q.GetOrCreateQueueManagementProfile(dropProfile)
	queueMgmtProfile.SetName(dropProfile)
	wred := queueMgmtProfile.GetOrCreateWred()
	uniform := wred.GetOrCreateUniform()
	uniform.SetEnableEcn(ecnConfig.ecnEnabled)
	uniform.SetMinThreshold(ecnConfig.minThreshold)
	uniform.SetMaxThreshold(ecnConfig.maxThreshold)
	uniform.SetMaxDropProbabilityPercent(ecnConfig.maxDropProbabilityPercent)

	if dut.Vendor() != ondatra.JUNIPER {
		uniform.SetDrop(ecnConfig.dropEnabled)
		// TODO: uncomment the following config after it is supported.
		// uniform.SetWeight(ecnConfig.weight)
	}

	uniform.SetWeight(ecnConfig.weight)

	t.Logf("qos ECN QueueManagementProfile config cases: %v", ecnConfig)
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

	// Verify the QueueManagementProfile is applied by checking the telemetry path state values.
	wredUniform := gnmi.OC().Qos().QueueManagementProfile(dropProfile).Wred().Uniform()
	if got, want := gnmi.Get(t, dut, wredUniform.EnableEcn().State()), ecnConfig.ecnEnabled; got != want {
		t.Errorf("wredUniform.EnableEcn().State(): got %v, want %v", got, want)
	}
	if got, want := gnmi.Get(t, dut, wredUniform.MinThreshold().State()), ecnConfig.minThreshold; got != want {
		t.Errorf("wredUniform.MinThreshold().State(): got %v, want %v", got, want)
	}
	if got, want := gnmi.Get(t, dut, wredUniform.MaxThreshold().State()), ecnConfig.maxThreshold; got != want {
		t.Errorf("wredUniform.MaxThreshold().State(): got %v, want %v", got, want)
	}
	if got, want := gnmi.Get(t, dut, wredUniform.MaxDropProbabilityPercent().State()), ecnConfig.maxDropProbabilityPercent; got != want {
		t.Errorf("wredUniform.MaxDropProbabilityPercent().State(): got %v, want %v", got, want)
	}
	if dut.Vendor() != ondatra.JUNIPER {
		if got, want := gnmi.Get(t, dut, wredUniform.Drop().State()), ecnConfig.dropEnabled; got != want {
			t.Errorf("wredUniform.Drop().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, wredUniform.Weight().State()), ecnConfig.weight; got != want {
			t.Errorf("wredUniform.Weight().State(): got %v, want %v", got, want)
		}

	}
}

func TestQoSOutputIntfConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")

	cases := []struct {
		desc       string
		queueName  string
		ecnProfile string
		scheduler  string
		queue      string
		sequence   uint32
	}{{
		desc:       "output-interface-BE1",
		queueName:  "BE1",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
		queue:      "0",
		sequence:   0,
	}, {
		desc:       "output-interface-BE0",
		queueName:  "BE0",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
		queue:      "1",
		sequence:   1,
	}, {
		desc:       "output-interface-AF1",
		queueName:  "AF1",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
		queue:      "2",
		sequence:   2,
	}, {
		desc:       "output-interface-AF2",
		queueName:  "AF2",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
		queue:      "3",
		sequence:   3,
	}, {
		desc:       "output-interface-AF3",
		queueName:  "AF3",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
		queue:      "4",
		sequence:   4,
	}, {
		desc:       "output-interface-AF4",
		queueName:  "AF4",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
		queue:      "5",
		sequence:   5,
	}, {
		desc:       "output-interface-NC1",
		queueName:  "NC1",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
		queue:      "6",
		sequence:   6,
	}}

	d := &oc.Root{}
	q := d.GetOrCreateQos()
	i := q.GetOrCreateInterface(dp.Name())
	i.SetInterfaceId(dp.Name())
	if dut.Vendor() == ondatra.JUNIPER {
		t.Logf("Adding required interface-ref config")
		iref := i.GetOrCreateInterfaceRef()
		iref.SetInterface(dp.Name())
		t.Logf("Queue management profile config required for scheduler and Queue management profile binding")
		queueMgmtProfile := q.GetOrCreateQueueManagementProfile(dropProfile)
		queueMgmtProfile.SetName(dropProfile)
		wred := queueMgmtProfile.GetOrCreateWred()
		uniform := wred.GetOrCreateUniform()
		uniform.SetEnableEcn(enableECN)
		uniform.SetMinThreshold(minThresholdPercent)
		uniform.SetMaxThreshold(maxThresholdPercent)
		uniform.SetMaxDropProbabilityPercent(maxDropProbabilityPercent)
	}
	t.Logf("qos output interface config cases: %v", cases)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			output := i.GetOrCreateOutput()
			schedulerPolicy := output.GetOrCreateSchedulerPolicy()
			schedulerPolicy.SetName(tc.scheduler)
			queue := output.GetOrCreateQueue(tc.queueName)
			queue.SetQueueManagementProfile(tc.ecnProfile)
			queue.SetName(tc.queueName)
			t.Logf("Scheduler config required for binding with queue management profile")
			if dut.Vendor() == ondatra.JUNIPER {
				fwdGroup := q.GetOrCreateForwardingGroup(tc.queueName)
				fwdGroup.SetName(tc.queueName)
				fwdGroup.SetOutputQueue(tc.queue)
				CreateSchedulerPolicy := q.GetOrCreateSchedulerPolicy(tc.scheduler)
				CreateSchedulerPolicy.SetName(tc.scheduler)
				s := CreateSchedulerPolicy.GetOrCreateScheduler(sequenceID)
				s.SetSequence(tc.sequence)
				s.SetPriority(schedulerPriority)
				SchedulerOutput := s.GetOrCreateOutput()
				SchedulerOutput.SetOutputFwdGroup(tc.queueName)
			}
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})

		// Verify the policy is applied by checking the telemetry path state values.
		policy := gnmi.OC().Qos().Interface(dp.Name()).Output().SchedulerPolicy()
		outQueue := gnmi.OC().Qos().Interface(dp.Name()).Output().Queue(tc.queueName)
		if got, want := gnmi.Get(t, dut, policy.Name().State()), tc.scheduler; got != want {
			t.Errorf("policy.Name().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, outQueue.Name().State()), tc.queueName; got != want {
			t.Errorf("outQueue.Name().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, outQueue.QueueManagementProfile().State()), tc.ecnProfile; got != want {
			t.Errorf("outQueue.QueueManagementProfile().State(): got %v, want %v", got, want)
		}
	}
}
