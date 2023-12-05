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

package qos_ecn_config_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type Testcase struct {
	name string
	fn   func(t *testing.T, q *oc.Qos)
}

// For Cisco devices, minimum and maximum threshold values can't be the same,
// as well as it should be a multiple of 6,144 bytes
const (
	CiscoMinThreshold = (uint64(8005632))
	CiscoMaxThreshold = (uint64(8011776))
)

var (
	QoSEcnConfigTestcases = []Testcase{
		{

			name: "testECNConfig",
			fn:   testECNConfig,
		},
		{
			name: "testQoSOutputIntfConfig",
			fn:   testQoSOutputIntfConfig,
		},
	}
)

// QoS ecn OC config:
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

func TestQosEcnConfigTests(t *testing.T) {
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	for _, tt := range QoSEcnConfigTestcases {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn(t, q)
		})
	}
}
func testECNConfig(t *testing.T, q *oc.Qos) {
	dut := ondatra.DUT(t, "dut")

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
		minThreshold:              uint64(80000),
		maxThreshold:              uint64(80000),
		maxDropProbabilityPercent: uint8(100),
		weight:                    uint32(0),
	}

	queueMgmtProfile := q.GetOrCreateQueueManagementProfile("DropProfile")
	queueMgmtProfile.SetName("DropProfile")
	wred := queueMgmtProfile.GetOrCreateWred()
	uniform := wred.GetOrCreateUniform()
	uniform.SetEnableEcn(ecnConfig.ecnEnabled)
	uniform.SetDrop(ecnConfig.dropEnabled)
	wantMinThreshold := ecnConfig.minThreshold
	wantMaxThreshold := ecnConfig.maxThreshold
	if deviations.EcnSameMinMaxThresholdUnsupported(dut) {
		wantMinThreshold = CiscoMinThreshold
		wantMaxThreshold = CiscoMaxThreshold
	}
	uniform.SetMinThreshold(wantMinThreshold)
	uniform.SetMaxThreshold(wantMaxThreshold)
	uniform.SetMaxDropProbabilityPercent(ecnConfig.maxDropProbabilityPercent)
	if !deviations.QosSetWeightConfigUnsupported(dut) {
		uniform.SetWeight(ecnConfig.weight)
	}
	if deviations.QosSchedulerConfigRequired(dut) {
		schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
		schedulerPolicy.SetName("scheduler")
		dp := dut.Port(t, "port2")
		i := q.GetOrCreateInterface(dp.Name())
		i.SetInterfaceId(dp.Name())
		i.GetOrCreateInterfaceRef().Interface = ygot.String(dp.Name())

		queueName := []string{"NC1", "AF4", "AF3", "AF2", "AF1", "BE0", "BE1"}

		for i, queue := range queueName {
			q1 := q.GetOrCreateQueue(queue)
			q1.Name = ygot.String(queue)
			queueid := len(queueName) - i
			q1.QueueId = ygot.Uint8(uint8(queueid))

		}
		cases := []struct {
			desc         string
			sequence     uint32
			priority     oc.E_Scheduler_Priority
			inputID      string
			inputType    oc.E_Input_InputType
			weight       uint64
			queueName    string
			targetGrpoup string
			ecnProfile   string
			scheduler    string
		}{{
			desc:         "scheduler-policy-BE1",
			sequence:     uint32(1),
			priority:     oc.Scheduler_Priority_UNSET,
			inputType:    oc.Input_InputType_QUEUE,
			weight:       uint64(1),
			queueName:    "BE1",
			targetGrpoup: "target-group-BE1",
			ecnProfile:   "DropProfile",
			scheduler:    "scheduler",
		}, {
			desc:         "scheduler-policy-BE0",
			sequence:     uint32(1),
			priority:     oc.Scheduler_Priority_UNSET,
			inputType:    oc.Input_InputType_QUEUE,
			weight:       uint64(4),
			queueName:    "BE0",
			targetGrpoup: "target-group-BE0",
			ecnProfile:   "DropProfile",
			scheduler:    "scheduler",
		}, {
			desc:         "scheduler-policy-AF1",
			sequence:     uint32(1),
			priority:     oc.Scheduler_Priority_UNSET,
			inputType:    oc.Input_InputType_QUEUE,
			weight:       uint64(8),
			queueName:    "AF1",
			targetGrpoup: "target-group-AF1",
			ecnProfile:   "DropProfile",
			scheduler:    "scheduler",
		}, {
			desc:         "scheduler-policy-AF2",
			sequence:     uint32(1),
			priority:     oc.Scheduler_Priority_UNSET,
			inputType:    oc.Input_InputType_QUEUE,
			weight:       uint64(16),
			queueName:    "AF2",
			ecnProfile:   "DropProfile",
			scheduler:    "scheduler",
			targetGrpoup: "target-group-AF2",
		}, {
			desc:         "scheduler-policy-AF3",
			sequence:     uint32(1),
			priority:     oc.Scheduler_Priority_UNSET,
			inputType:    oc.Input_InputType_QUEUE,
			weight:       uint64(32),
			queueName:    "AF3",
			targetGrpoup: "target-group-AF3",
			ecnProfile:   "DropProfile",
			scheduler:    "scheduler",
		}, {
			desc:         "scheduler-policy-AF4",
			sequence:     uint32(0),
			priority:     oc.Scheduler_Priority_STRICT,
			inputType:    oc.Input_InputType_QUEUE,
			weight:       uint64(6),
			queueName:    "AF4",
			targetGrpoup: "target-group-AF4",
			ecnProfile:   "DropProfile",
			scheduler:    "scheduler",
		}, {
			desc:         "scheduler-policy-NC1",
			sequence:     uint32(0),
			priority:     oc.Scheduler_Priority_STRICT,
			inputType:    oc.Input_InputType_QUEUE,
			weight:       uint64(7),
			queueName:    "NC1",
			targetGrpoup: "target-group-NC1",
			ecnProfile:   "DropProfile",
			scheduler:    "scheduler",
		}}

		t.Logf("qos scheduler policies config cases: %v", cases)
		for _, tc := range cases {
			t.Run(tc.desc, func(t *testing.T) {
				s := schedulerPolicy.GetOrCreateScheduler(tc.sequence)
				s.SetSequence(tc.sequence)
				s.SetPriority(tc.priority)
				input := s.GetOrCreateInput(tc.queueName)
				input.SetId(tc.queueName)
				input.SetInputType(tc.inputType)
				input.SetQueue(tc.queueName)
				input.SetWeight(tc.weight)

				output := i.GetOrCreateOutput()
				schedulerPolicy := output.GetOrCreateSchedulerPolicy()
				schedulerPolicy.SetName(tc.scheduler)
				queue := output.GetOrCreateQueue(tc.queueName)
				queue.SetQueueManagementProfile(tc.ecnProfile)
				queue.SetName(tc.queueName)

			})
		}
	}

	t.Logf("qos ECN QueueManagementProfile config cases: %v", ecnConfig)
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

	// Verify the QueueManagementProfile is applied by checking the telemetry path state values.
	wredUniform := gnmi.OC().Qos().QueueManagementProfile("DropProfile").Wred().Uniform()
	if deviations.QosGetStatePathUnsupported(dut) {
		if got, want := gnmi.Get(t, dut, wredUniform.EnableEcn().Config()), ecnConfig.ecnEnabled; got != want {
			t.Errorf("wredUniform.EnableEcn().Config(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, wredUniform.MaxDropProbabilityPercent().Config()), ecnConfig.maxDropProbabilityPercent; got != want {
			t.Errorf("wredUniform.MaxDropProbabilityPercent().Config(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, wredUniform.MinThreshold().Config()), wantMinThreshold; got != want {
			t.Errorf("wredUniform.MinThreshold().Config(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, wredUniform.MaxThreshold().Config()), wantMaxThreshold; got != want {
			t.Errorf("wredUniform.MaxThreshold().Config(): got %v, want %v", got, want)
		}
	} else {
		if got, want := gnmi.Get(t, dut, wredUniform.EnableEcn().State()), ecnConfig.ecnEnabled; got != want {
			t.Errorf("wredUniform.EnableEcn().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, wredUniform.MaxDropProbabilityPercent().State()), ecnConfig.maxDropProbabilityPercent; got != want {
			t.Errorf("wredUniform.MaxDropProbabilityPercent().State(): got %v, want %v", got, want)
		}

	}
	if !deviations.StatePathsUnsupported(dut) {
		if got, want := gnmi.Get(t, dut, wredUniform.MinThreshold().State()), ecnConfig.minThreshold; got != want {
			t.Errorf("wredUniform.MinThreshold().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, wredUniform.MaxThreshold().State()), ecnConfig.maxThreshold; got != want {
			t.Errorf("wredUniform.MaxThreshold().State(): got %v, want %v", got, want)
		}
	}
	if !deviations.DropWeightLeavesUnsupported(dut) {
		if got, want := gnmi.Get(t, dut, wredUniform.Drop().State()), ecnConfig.dropEnabled; got != want {
			t.Errorf("wredUniform.Drop().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, wredUniform.Weight().State()), ecnConfig.weight; got != want {
			t.Errorf("wredUniform.Weight().State(): got %v, want %v", got, want)
		}
	}
}

func testQoSOutputIntfConfig(t *testing.T, q *oc.Qos) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	queues := netutil.CommonTrafficQueues(t, dut)

	cases := []struct {
		desc       string
		queueName  string
		ecnProfile string
		scheduler  string
	}{{
		desc:       "output-interface-BE1",
		queueName:  queues.BE1,
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}, {
		desc:       "output-interface-BE0",
		queueName:  queues.BE0,
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}, {
		desc:       "output-interface-AF1",
		queueName:  queues.AF1,
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}, {
		desc:       "output-interface-AF2",
		queueName:  queues.AF2,
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}, {
		desc:       "output-interface-AF3",
		queueName:  queues.AF3,
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}, {
		desc:       "output-interface-AF4",
		queueName:  queues.AF4,
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}, {
		desc:       "output-interface-NC1",
		queueName:  queues.NC1,
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}}

	i := q.GetOrCreateInterface(dp.Name())
	i.SetInterfaceId(dp.Name())
	i.GetOrCreateInterfaceRef().Interface = ygot.String(dp.Name())
	if deviations.InterfaceRefConfigUnsupported(dut) {
		i.InterfaceRef = nil
	}
	if deviations.QOSQueueRequiresID(dut) {
		queueNames := []string{queues.NC1, queues.AF4, queues.AF3, queues.AF2, queues.AF1, queues.BE0, queues.BE1}
		for i, queue := range queueNames {
			q1 := q.GetOrCreateQueue(queue)
			q1.Name = ygot.String(queue)
			queueid := len(queueNames) - i
			q1.QueueId = ygot.Uint8(uint8(queueid))
		}
	}

	schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
	schedulerPolicy.SetName("scheduler")
	t.Logf("qos output interface config cases: %v", cases)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			qoscfg.SetForwardingGroup(t, dut, q, tc.queueName, tc.queueName)
			output := i.GetOrCreateOutput()
			schedulerPolicy := output.GetOrCreateSchedulerPolicy()
			schedulerPolicy.SetName(tc.scheduler)
			queue := output.GetOrCreateQueue(tc.queueName)
			queue.SetQueueManagementProfile(tc.ecnProfile)
			queue.SetName(tc.queueName)
			if deviations.QOSBufferAllocationConfigRequired(dut) {
				bufferAllocation := q.GetOrCreateBufferAllocationProfile("ballocprofile")
				bq := bufferAllocation.GetOrCreateQueue(tc.queueName)
				bq.SetStaticSharedBufferLimit(uint32(268435456))
				output.SetBufferAllocationProfile("ballocprofile")
			}
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})

		// Verify the policy is applied by checking the telemetry path state values.
		policy := gnmi.OC().Qos().Interface(dp.Name()).Output().SchedulerPolicy()
		outQueue := gnmi.OC().Qos().Interface(dp.Name()).Output().Queue(tc.queueName)
		if !deviations.StatePathsUnsupported(dut) {
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
		if deviations.QosGetStatePathUnsupported(dut) {
			if got, want := gnmi.Get(t, dut, policy.Name().Config()), tc.scheduler; got != want {
				t.Errorf("policy.Name().Config(): got %v, want %v", got, want)
			}
			if got, want := gnmi.Get(t, dut, outQueue.Name().Config()), tc.queueName; got != want {
				t.Errorf("outQueue.Name().Config(): got %v, want %v", got, want)
			}
			if got, want := gnmi.Get(t, dut, outQueue.QueueManagementProfile().Config()), tc.ecnProfile; got != want {
				t.Errorf("outQueue.QueueManagementProfile().Config(): got %v, want %v", got, want)
			}
		}
	}
}
