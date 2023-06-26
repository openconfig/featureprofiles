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
	"math"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
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
	fn   func(t *testing.T)
}

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
	QoSCiscoEcnConfigTestcases = []Testcase{
		{

			name: "testCiscoECNConfig",
			fn:   testCiscoECNConfig,
		},
	}
	QoSJuniperEcnConfigTestcases = []Testcase{
		{

			name: "testJuniperECNConfig",
			fn:   testJuniperECNConfig,
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
	dut := ondatra.DUT(t, "dut")
	switch dut.Vendor() {
	case ondatra.CISCO:
		for _, tt := range QoSCiscoEcnConfigTestcases {
			t.Run(tt.name, func(t *testing.T) {
				tt.fn(t)
			})
		}
	case ondatra.JUNIPER:
		for _, tt := range QoSJuniperEcnConfigTestcases {
			t.Run(tt.name, func(t *testing.T) {
				tt.fn(t)
			})
		}
	default:
		for _, tt := range QoSEcnConfigTestcases {
			t.Run(tt.name, func(t *testing.T) {
				tt.fn(t)
			})
		}
	}
}
func testECNConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	q := d.GetOrCreateQos()

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
		maxThreshold:              math.MaxUint64,
		maxDropProbabilityPercent: uint8(1),
		weight:                    uint32(0),
	}

	queueMgmtProfile := q.GetOrCreateQueueManagementProfile("DropProfile")
	queueMgmtProfile.SetName("DropProfile")
	wred := queueMgmtProfile.GetOrCreateWred()
	uniform := wred.GetOrCreateUniform()
	uniform.SetEnableEcn(ecnConfig.ecnEnabled)
	uniform.SetDrop(ecnConfig.dropEnabled)
	uniform.SetMinThreshold(ecnConfig.minThreshold)
	uniform.SetMaxThreshold(ecnConfig.maxThreshold)
	uniform.SetMaxDropProbabilityPercent(ecnConfig.maxDropProbabilityPercent)
	uniform.SetWeight(ecnConfig.weight)

	t.Logf("qos ECN QueueManagementProfile config cases: %v", ecnConfig)
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

	// TODO: Remove the following t.Skipf() after the config verification code has been tested.
	t.Skipf("Skip the QoS config verification until it is tested against a DUT.")

	// Verify the QueueManagementProfile is applied by checking the telemetry path state values.
	wredUniform := gnmi.OC().Qos().QueueManagementProfile("DropProfile").Wred().Uniform()
	if got, want := gnmi.Get(t, dut, wredUniform.EnableEcn().State()), ecnConfig.ecnEnabled; got != want {
		t.Errorf("wredUniform.EnableEcn().State(): got %v, want %v", got, want)
	}
	if got, want := gnmi.Get(t, dut, wredUniform.Drop().State()), ecnConfig.dropEnabled; got != want {
		t.Errorf("wredUniform.Drop().State(): got %v, want %v", got, want)
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
	if got, want := gnmi.Get(t, dut, wredUniform.Weight().State()), ecnConfig.weight; got != want {
		t.Errorf("wredUniform.Weight().State(): got %v, want %v", got, want)
	}
}

func testQoSOutputIntfConfig(t *testing.T) {
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

	d := &oc.Root{}
	q := d.GetOrCreateQos()
	i := q.GetOrCreateInterface(dp.Name())
	i.SetInterfaceId(dp.Name())

	t.Logf("qos output interface config cases: %v", cases)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			output := i.GetOrCreateOutput()
			schedulerPolicy := output.GetOrCreateSchedulerPolicy()
			schedulerPolicy.SetName(tc.scheduler)
			queue := output.GetOrCreateQueue(tc.queueName)
			queue.SetQueueManagementProfile(tc.ecnProfile)
			queue.SetName(tc.queueName)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})

		// TODO: Remove the following t.Skipf() after the config verification code has been tested.
		t.Skipf("Skip the QoS config verification until it is tested against a DUT.")

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
func testCiscoECNConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	queueName := []string{"NC1", "AF4", "AF3", "AF2", "AF1", "BE0", "BE1"}

	for i, queue := range queueName {
		q1 := q.GetOrCreateQueue(queue)
		q1.Name = ygot.String(queue)
		queueid := len(queueName) - i
		q1.QueueId = ygot.Uint8(uint8(queueid))

	}
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

	cases := []struct {
		desc         string
		sequence     uint32
		priority     oc.E_Scheduler_Priority
		inputID      string
		inputType    oc.E_Input_InputType
		weight       uint64
		queueName    string
		targetGrpoup string
	}{{
		desc:         "scheduler-policy-BE1",
		sequence:     uint32(1),
		priority:     oc.Scheduler_Priority_UNSET,
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(1),
		queueName:    "BE1",
		targetGrpoup: "target-group-BE1",
	}, {
		desc:         "scheduler-policy-BE0",
		sequence:     uint32(1),
		priority:     oc.Scheduler_Priority_UNSET,
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(4),
		queueName:    "BE0",
		targetGrpoup: "target-group-BE0",
	}, {
		desc:         "scheduler-policy-AF1",
		sequence:     uint32(1),
		priority:     oc.Scheduler_Priority_UNSET,
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(8),
		queueName:    "AF1",
		targetGrpoup: "target-group-AF1",
	}, {
		desc:         "scheduler-policy-AF2",
		sequence:     uint32(1),
		priority:     oc.Scheduler_Priority_UNSET,
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(16),
		queueName:    "AF2",
		targetGrpoup: "target-group-AF2",
	}, {
		desc:         "scheduler-policy-AF3",
		sequence:     uint32(1),
		priority:     oc.Scheduler_Priority_UNSET,
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(32),
		queueName:    "AF3",
		targetGrpoup: "target-group-AF3",
	}, {
		desc:         "scheduler-policy-AF4",
		sequence:     uint32(0),
		priority:     oc.Scheduler_Priority_STRICT,
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(6),
		queueName:    "AF4",
		targetGrpoup: "target-group-AF4",
	}, {
		desc:         "scheduler-policy-NC1",
		sequence:     uint32(0),
		priority:     oc.Scheduler_Priority_STRICT,
		inputType:    oc.Input_InputType_QUEUE,
		weight:       uint64(7),
		queueName:    "NC1",
		targetGrpoup: "target-group-NC1",
	}}

	schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
	schedulerPolicy.SetName("scheduler")
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
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})
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
		minThreshold:              uint64(8005632),
		maxThreshold:              uint64(8011776),
		maxDropProbabilityPercent: uint8(100),
		weight:                    uint32(0),
	}
	queueMgmtProfile := q.GetOrCreateQueueManagementProfile("DropProfile")
	queueMgmtProfile.SetName("DropProfile")
	wred := queueMgmtProfile.GetOrCreateWred()
	uniform := wred.GetOrCreateUniform()
	uniform.SetEnableEcn(ecnConfig.ecnEnabled)
	uniform.SetMinThreshold(ecnConfig.minThreshold)
	uniform.SetMaxThreshold(ecnConfig.maxThreshold)
	uniform.SetDrop(ecnConfig.dropEnabled)
	uniform.SetMaxDropProbabilityPercent(ecnConfig.maxDropProbabilityPercent)
	t.Logf("qos ECN QueueManagementProfile config cases: %v", ecnConfig)
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

	dp := dut.Port(t, "port2")

	intcases := []struct {
		desc       string
		queueName  string
		ecnProfile string
		scheduler  string
	}{{
		desc:       "output-interface-NC1",
		queueName:  "NC1",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}, {
		desc:       "output-interface-AF4",
		queueName:  "AF4",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}, {
		desc:       "output-interface-AF3",
		queueName:  "AF3",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}, {
		desc:       "output-interface-AF2",
		queueName:  "AF2",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}, {
		desc:       "output-interface-AF1",
		queueName:  "AF1",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}, {
		desc:       "output-interface-BE0",
		queueName:  "BE0",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}, {
		desc:       "output-interface-BE1",
		queueName:  "BE1",
		ecnProfile: "DropProfile",
		scheduler:  "scheduler",
	}}
	i := q.GetOrCreateInterface(dp.Name())
	i.SetInterfaceId(dp.Name())
	t.Logf("qos output interface config cases: %v", cases)

	for _, tc := range intcases {
		t.Run(tc.desc, func(t *testing.T) {
			output := i.GetOrCreateOutput()
			schedulerPolicy := output.GetOrCreateSchedulerPolicy()
			schedulerPolicy.SetName(tc.scheduler)
			queue := output.GetOrCreateQueue(tc.queueName)
			queue.SetQueueManagementProfile(tc.ecnProfile)
			queue.SetName(tc.queueName)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})

	}

	for _, tc := range cases {
		// Verify the SchedulerPolicy is applied by checking the telemetry path state values.
		scheduler := gnmi.OC().Qos().SchedulerPolicy("scheduler").Scheduler(tc.sequence)
		input := scheduler.Input(tc.queueName)

		if got, want := gnmi.GetConfig(t, dut, scheduler.Sequence().Config()), tc.sequence; got != want {
			t.Errorf("scheduler.Sequence().State(): got %v, want %v", got, want)
		}
		if tc.priority == oc.Scheduler_Priority_STRICT {
			if got, want := gnmi.GetConfig(t, dut, scheduler.Priority().Config()), tc.priority; got != want {
				t.Errorf("scheduler.Priority().State(): got %v, want %v", got, want)
			}
		}
		if got, want := gnmi.GetConfig(t, dut, input.Id().Config()), tc.queueName; got != want {
			t.Errorf("input.Id().State(): got %v, want %v", got, want)
		}

		if got, want := gnmi.GetConfig(t, dut, input.Weight().Config()), tc.weight; got != want {
			t.Errorf("input.Weight().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.GetConfig(t, dut, input.Queue().Config()), tc.queueName; got != want {
			t.Errorf("input.Queue().State(): got %v, want %v", got, want)
		}
	}

	// Verify the QueueManagementProfile is applied by checking the telemetry path state values.
	wredUniform := gnmi.OC().Qos().QueueManagementProfile("DropProfile").Wred().Uniform()
	if got, want := gnmi.GetConfig(t, dut, wredUniform.EnableEcn().Config()), ecnConfig.ecnEnabled; got != want {
		t.Errorf("wredUniform.EnableEcn().State(): got %v, want %v", got, want)
	}

	if got, want := gnmi.GetConfig(t, dut, wredUniform.MinThreshold().Config()), ecnConfig.minThreshold; got != want {
		t.Errorf("wredUniform.MinThreshold().State(): got %v, want %v", got, want)
	}
	if got, want := gnmi.GetConfig(t, dut, wredUniform.MaxThreshold().Config()), ecnConfig.maxThreshold; got != want {
		t.Errorf("wredUniform.MaxThreshold().State(): got %v, want %v", got, want)
	}
	if got, want := gnmi.GetConfig(t, dut, wredUniform.MaxDropProbabilityPercent().Config()), ecnConfig.maxDropProbabilityPercent; got != want {
		t.Errorf("wredUniform.MaxDropProbabilityPercent().State(): got %v, want %v", got, want)
	}

	for _, tc := range intcases {
		policy := gnmi.OC().Qos().Interface(dp.Name()).Output().SchedulerPolicy()
		outQueue := gnmi.OC().Qos().Interface(dp.Name()).Output().Queue(tc.queueName)
		if got, want := gnmi.GetConfig(t, dut, policy.Name().Config()), tc.scheduler; got != want {
			t.Errorf("policy.Name().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.GetConfig(t, dut, outQueue.Name().Config()), tc.queueName; got != want {
			t.Errorf("outQueue.Name().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.GetConfig(t, dut, outQueue.QueueManagementProfile().Config()), tc.ecnProfile; got != want {
			t.Errorf("outQueue.QueueManagementProfile().State(): got %v, want %v", got, want)
		}
	}

}

func testJuniperECNConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	dp := dut.Port(t, "port2")
	i := q.GetOrCreateInterface(dp.Name())
	i.SetInterfaceId(dp.Name())
	i.GetOrCreateInterfaceRef().Interface = ygot.String(dp.Name())
	i.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)

	schedulers := []struct {
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
		queueName:   "6",
		targetGroup: "BE1",
	}, {
		desc:        "scheduler-policy-BE0",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "BE0",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(2),
		queueName:   "0",
		targetGroup: "BE0",
	}, {
		desc:        "scheduler-policy-AF1",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(4),
		queueName:   "4",
		targetGroup: "AF1",
	}, {
		desc:        "scheduler-policy-AF2",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF2",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(8),
		queueName:   "1",
		targetGroup: "AF2",
	}, {
		desc:        "scheduler-policy-AF3",
		sequence:    uint32(1),
		priority:    oc.Scheduler_Priority_UNSET,
		inputID:     "AF3",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(16),
		queueName:   "5",
		targetGroup: "AF3",
	}, {
		desc:        "scheduler-policy-AF4",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "AF4",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(99),
		queueName:   "2",
		targetGroup: "AF4",
	}, {
		desc:        "scheduler-policy-NC1",
		sequence:    uint32(0),
		priority:    oc.Scheduler_Priority_STRICT,
		inputID:     "NC1",
		inputType:   oc.Input_InputType_QUEUE,
		weight:      uint64(100),
		queueName:   "3",
		targetGroup: "NC1",
	}}

	schedulerPolicy := q.GetOrCreateSchedulerPolicy("scheduler")
	schedulerPolicy.SetName("scheduler")
	t.Logf("qos scheduler policies config cases: %v", schedulers)
	for _, tc := range schedulers {
		t.Run(tc.desc, func(t *testing.T) {
			fwdGroup := q.GetOrCreateForwardingGroup(tc.targetGroup)
			fwdGroup.SetName(tc.targetGroup)
			fwdGroup.SetOutputQueue(tc.queueName)
			s := schedulerPolicy.GetOrCreateScheduler(tc.sequence)
			s.SetSequence(tc.sequence)
			s.SetPriority(tc.priority)
			input := s.GetOrCreateInput(tc.inputID)
			input.SetId(tc.inputID)
			input.SetInputType(tc.inputType)
			input.SetQueue(tc.queueName)
			input.SetWeight(tc.weight)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})
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
		minThreshold:              uint64(80000),
		maxThreshold:              math.MaxUint64,
		maxDropProbabilityPercent: uint8(1),
		weight:                    uint32(0),
	}

	queueMgmtProfile := q.GetOrCreateQueueManagementProfile("DropProfile")
	queueMgmtProfile.SetName("DropProfile")
	wred := queueMgmtProfile.GetOrCreateWred()
	uniform := wred.GetOrCreateUniform()
	uniform.SetEnableEcn(ecnConfig.ecnEnabled)
	uniform.SetMinThreshold(ecnConfig.minThreshold)
	uniform.SetMaxThreshold(ecnConfig.maxThreshold)
	uniform.SetMaxDropProbabilityPercent(ecnConfig.maxDropProbabilityPercent)

	t.Logf("qos ECN QueueManagementProfile config cases: %v", ecnConfig)
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

	// Verify the QueueManagementProfile is applied by checking the telemetry path state values.
	wredUniform := gnmi.OC().Qos().QueueManagementProfile("DropProfile").Wred().Uniform()

	if got, want := gnmi.Get(t, dut, wredUniform.MaxDropProbabilityPercent().State()), ecnConfig.maxDropProbabilityPercent; got != want {
		t.Errorf("wredUniform.MaxDropProbabilityPercent().State(): got %v, want %v", got, want)
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

		uniform.SetDrop(ecnConfig.dropEnabled)
		uniform.SetWeight(ecnConfig.weight)
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

		if got, want := gnmi.Get(t, dut, wredUniform.Drop().State()), ecnConfig.dropEnabled; got != want {
			t.Errorf("wredUniform.Drop().State(): got %v, want %v", got, want)
		}
		if got, want := gnmi.Get(t, dut, wredUniform.Weight().State()), ecnConfig.weight; got != want {
			t.Errorf("wredUniform.Weight().State(): got %v, want %v", got, want)
		}
	}

	cases := []struct {
		desc        string
		targetGroup string
		ecnProfile  string
		scheduler   string
	}{{
		desc:        "output-interface-BE1",
		targetGroup: "BE1",
		ecnProfile:  "DropProfile",
		scheduler:   "scheduler",
	}, {
		desc:        "output-interface-BE0",
		targetGroup: "BE0",
		ecnProfile:  "DropProfile",
		scheduler:   "scheduler",
	}, {
		desc:        "output-interface-AF1",
		targetGroup: "AF1",
		ecnProfile:  "DropProfile",
		scheduler:   "scheduler",
	}, {
		desc:        "output-interface-AF2",
		targetGroup: "AF2",
		ecnProfile:  "DropProfile",
		scheduler:   "scheduler",
	}, {
		desc:        "output-interface-AF3",
		targetGroup: "AF3",
		ecnProfile:  "DropProfile",
		scheduler:   "scheduler",
	}, {
		desc:        "output-interface-AF4",
		targetGroup: "AF4",
		ecnProfile:  "DropProfile",
		scheduler:   "scheduler",
	}, {
		desc:        "output-interface-NC1",
		targetGroup: "NC1",
		ecnProfile:  "DropProfile",
		scheduler:   "scheduler",
	}}

	t.Logf("qos output interface config cases: %v", cases)
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			output := i.GetOrCreateOutput()
			schedulerPolicy := output.GetOrCreateSchedulerPolicy()
			schedulerPolicy.SetName(tc.scheduler)
			queue := output.GetOrCreateQueue(tc.targetGroup)
			queue.SetQueueManagementProfile(tc.ecnProfile)
			queue.SetName(tc.targetGroup)
			gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)
		})

		// Verify the policy is applied by checking the telemetry path state values.
		policy := gnmi.OC().Qos().Interface(dp.Name()).Output().SchedulerPolicy()
		outQueue := gnmi.OC().Qos().Interface(dp.Name()).Output().Queue(tc.targetGroup)
		if !deviations.StatePathsUnsupported(dut) {
			if got, want := gnmi.Get(t, dut, policy.Name().State()), tc.scheduler; got != want {
				t.Errorf("policy.Name().State(): got %v, want %v", got, want)
			}
			if got, want := gnmi.Get(t, dut, outQueue.Name().State()), tc.targetGroup; got != want {
				t.Errorf("outQueue.Name().State(): got %v, want %v", got, want)
			}
			if got, want := gnmi.Get(t, dut, outQueue.QueueManagementProfile().State()), tc.ecnProfile; got != want {
				t.Errorf("outQueue.QueueManagementProfile().State(): got %v, want %v", got, want)
			}
		}
		if got, want := gnmi.Get(t, dut, wredUniform.EnableEcn().State()), ecnConfig.ecnEnabled; got != want {
			t.Errorf("wredUniform.EnableEcn().State(): got %v, want %v", got, want)
		}
	}
}
