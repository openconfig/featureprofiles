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

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

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

const (
	dropProfile               = "DropProfile"
	forwardingGroup           = "fc"
	schedulerMap              = "smap"
	sequeneId                 = 1
	schedulerPriority         = 1
	queueName                 = "0"
	minThresholdPerecent      = 10
	maxThresholdPercent       = 70
	maxDropProbabilityPercent = 1
	enableEcn                 = true
)

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
		s := schedulerPolicy.GetOrCreateScheduler(sequeneId)
		s.SetSequence(sequeneId)
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
		uniform.SetEnableEcn(enableEcn)
		uniform.SetMinThreshold(minThresholdPerecent)
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
				s := CreateSchedulerPolicy.GetOrCreateScheduler(sequeneId)
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
