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

// Package qoscfg provides utilities for configure QoS across vendors.
package qoscfg

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// SetForwardingGroup sets a forwarding group in the specified QoS config.
func SetForwardingGroup(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, groupName, queueName string) {
	t.Helper()
	qos.GetOrCreateForwardingGroup(groupName).SetOutputQueue(queueName)
	qos.GetOrCreateQueue(queueName)
	gnmi.Update(t, dut, gnmi.OC().Qos().Config(), qos)
}

// SetInputClassifier sets an input classifier in the specified QoS config.
func SetInputClassifier(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, intfID string, classType oc.E_Input_Classifier_Type, className string) {
	t.Helper()
	qosIntfID := QosClassifierInterfaceID(dut, intfID)
	intf := qos.GetOrCreateInterface(qosIntfID)
	intf.SetInterfaceId(qosIntfID)
	intf.GetOrCreateInterfaceRef().SetInterface(intfID)
	if !deviations.QosSchedulerConfigRequired(dut) {
		intf.GetOrCreateInterfaceRef().SetSubinterface(0)
	}
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}
	intf.GetOrCreateInput().GetOrCreateClassifier(classType).SetName(className)
	gnmi.Update(t, dut, gnmi.OC().Qos().Config(), qos)
}

// SetOutputQueueManagementProfile sets an output queue management profile on the specified interface queue.
func SetOutputQueueManagementProfile(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, intfID string, queueName string, profileName string) {
	t.Helper()
	intf := qos.GetOrCreateInterface(intfID)
	intf.SetInterfaceId(intfID)
	intf.GetOrCreateInterfaceRef().Interface = ygot.String(intfID)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}
	output := intf.GetOrCreateOutput()
	queue := output.GetOrCreateQueue(queueName)
	queue.SetName(queueName)
	queue.SetQueueManagementProfile(profileName)
	if deviations.QOSBufferAllocationConfigRequired(dut) {
		bufferAllocation := qos.GetOrCreateBufferAllocationProfile("ballocprofile")
		bq := bufferAllocation.GetOrCreateQueue(queueName)
		bq.SetStaticSharedBufferLimit(uint32(268435456))
		output.SetBufferAllocationProfile("ballocprofile")
	}
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), qos)
}

// ConfigureIPv4DSCPClassifier adds IPv4 classifier className to qos with one
// term that matches dscp and, if targetGroup is non-empty, sends matching
// packets to that forwarding group. It only edits qos; nothing is pushed to
// the DUT. On DUTs with deviations.QOSQueueRequiresID the term ID is "0" and
// the match uses dscp-set instead of dscp.
func ConfigureIPv4DSCPClassifier(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, className string, targetGroup string, dscp uint8) {
	t.Helper()
	classifier := qos.GetOrCreateClassifier(className)
	classifier.SetName(className)
	classifier.SetType(oc.Qos_Classifier_Type_IPV4)
	termID := "term1"
	if deviations.QOSQueueRequiresID(dut) {
		termID = "0"
	}
	term := classifier.GetOrCreateTerm(termID)
	term.SetId(termID)
	if deviations.QOSQueueRequiresID(dut) {
		term.GetOrCreateConditions().GetOrCreateIpv4().SetDscpSet([]uint8{dscp})
	} else {
		term.GetOrCreateConditions().GetOrCreateIpv4().SetDscp(dscp)
	}
	if targetGroup != "" {
		term.GetOrCreateActions().SetTargetGroup(targetGroup)
	}
}

// QosInterfaceID returns the qos/interfaces list key for intfID: intfID
// itself, or "<intfID>.0" on DUTs with deviations.InterfaceRefInterfaceIDFormat.
func QosInterfaceID(dut *ondatra.DUTDevice, intfID string) string {
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		return intfID + ".0"
	}
	return intfID
}

// QosClassifierInterfaceID returns the qos/interfaces list key that
// SetInputClassifier uses for intfID. It matches QosInterfaceID, except that
// DUTs with deviations.QOSBufferAllocationConfigRequired also use "<intfID>.0",
// because they accept input classifiers only on the subinterface.
func QosClassifierInterfaceID(dut *ondatra.DUTDevice, intfID string) string {
	if deviations.InterfaceRefInterfaceIDFormat(dut) || deviations.QOSBufferAllocationConfigRequired(dut) {
		return intfID + ".0"
	}
	return intfID
}

// BuildOutputQueueManagementProfile edits qos so that output queue queueName
// on interface intfID uses queue management profile profileName. If
// profileName is empty, the queue is created without a profile. It only edits
// qos; push it with ReplaceQueueManagementProfile.
//
// It also adds the config some DUTs require before a profile can be attached:
//   - deviations.QOSQueueRequiresID: adds queueName to the top-level queue list
//     with queue ID 0.
//   - deviations.QOSBufferAllocationConfigRequired: creates buffer allocation
//     profile "ballocprofile" and attaches it to the interface output.
//   - deviations.QosSchedulerConfigRequired: adds queue "7" and scheduler
//     policy "scheduler" (queue "7" strict priority, queueName weight 1) and
//     attaches the policy to the interface output.
func BuildOutputQueueManagementProfile(dut *ondatra.DUTDevice, qos *oc.Qos, intfID, queueName, profileName string) {
	qosIntfID := QosInterfaceID(dut, intfID)
	intf := qos.GetOrCreateInterface(qosIntfID)
	intf.SetInterfaceId(qosIntfID)
	intf.GetOrCreateInterfaceRef().SetInterface(intfID)
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		intf.GetOrCreateInterfaceRef().SetSubinterface(0)
	}
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}

	if deviations.QOSQueueRequiresID(dut) {
		q1 := qos.GetOrCreateQueue(queueName)
		q1.Name = ygot.String(queueName)
		q1.QueueId = ygot.Uint8(0)
	}

	queue := intf.GetOrCreateOutput().GetOrCreateQueue(queueName)
	queue.SetName(queueName)
	queue.Name = ygot.String(queueName)
	if profileName != "" {
		queue.SetQueueManagementProfile(profileName)
	}
	if deviations.QOSBufferAllocationConfigRequired(dut) {
		bufferAllocationProfile := qos.GetOrCreateBufferAllocationProfile("ballocprofile")
		bufferAllocationQueue := bufferAllocationProfile.GetOrCreateQueue(queueName)
		bufferAllocationQueue.SetStaticSharedBufferLimit(uint32(268435456))
		intf.GetOrCreateOutput().SetBufferAllocationProfile("ballocprofile")
	}
	if deviations.QosSchedulerConfigRequired(dut) {
		q7 := qos.GetOrCreateQueue("7")
		q7.Name = ygot.String("7")
		q7.QueueId = ygot.Uint8(7)

		outQ7 := intf.GetOrCreateOutput().GetOrCreateQueue("7")
		outQ7.SetName("7")

		schedulerPolicy := qos.GetOrCreateSchedulerPolicy("scheduler")
		schedulerPolicy.SetName("scheduler")

		s0 := schedulerPolicy.GetOrCreateScheduler(0)
		s0.SetSequence(0)
		s0.SetPriority(oc.Scheduler_Priority_STRICT)
		in0 := s0.GetOrCreateInput("7")
		in0.SetId("7")
		in0.SetInputType(oc.Input_InputType_QUEUE)
		in0.SetQueue("7")
		in0.SetWeight(7)

		s1 := schedulerPolicy.GetOrCreateScheduler(1)
		s1.SetSequence(1)
		s1.SetPriority(oc.Scheduler_Priority_UNSET)
		in1 := s1.GetOrCreateInput(queueName)
		in1.SetId(queueName)
		in1.SetInputType(oc.Input_InputType_QUEUE)
		in1.SetQueue(queueName)
		in1.SetWeight(1)

		intf.GetOrCreateOutput().GetOrCreateSchedulerPolicy().SetName("scheduler")
	}
}

// ReplaceQueueManagementProfile pushes qos to the DUT in one gNMI SetRequest:
//   - REPLACE queue management profile profileName.
//   - REPLACE output queue queueName on interface intfID.
//   - UPDATE (merge) the rest of qos.
//
// Only these two subtrees are replaced, so each test case gets a clean profile
// and queue without deleting the classifier, forwarding group and queues
// created during test setup. gNMI applies all replaces before updates, so the
// supporting config (e.g. buffer allocation and scheduler policies) is merged
// after the two subtrees are rewritten.
//
// Call BuildOutputQueueManagementProfile with the same arguments first. The
// test fails if qos does not contain the profile or the output queue.
func ReplaceQueueManagementProfile(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, intfID, queueName, profileName string) {
	t.Helper()
	qosIntfID := QosInterfaceID(dut, intfID)

	// Fail on missing subtrees: replacing a path with an empty value would
	// delete the existing device config at that path.
	profile := qos.QueueManagementProfile[profileName]
	if profile == nil {
		t.Fatalf("ReplaceQueueManagementProfile: qos does not describe queue management profile %q", profileName)
	}
	intf := qos.Interface[qosIntfID]
	if intf == nil || intf.Output == nil || intf.Output.Queue[queueName] == nil {
		t.Fatalf("ReplaceQueueManagementProfile: qos does not describe output queue %q on interface %q", queueName, qosIntfID)
	}
	outQueue := intf.Output.Queue[queueName]

	batch := &gnmi.SetBatch{}
	gnmi.BatchReplace(batch, gnmi.OC().Qos().QueueManagementProfile(profileName).Config(), profile)
	gnmi.BatchReplace(batch, gnmi.OC().Qos().Interface(qosIntfID).Output().Queue(queueName).Config(), outQueue)
	gnmi.BatchUpdate(batch, gnmi.OC().Qos().Config(), qos)
	batch.Set(t, dut)
}

// ConfigureInterfaceSetup merges (gNMI UPDATE) an ethernetCsmacd interface
// config for port dp onto the DUT. It also creates subinterface 0, except on
// DUTs with deviations.QosSchedulerConfigRequired.
func ConfigureInterfaceSetup(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port) {
	t.Helper()
	intfConfig := &oc.Interface{Name: ygot.String(dp.Name())}
	intfConfig.SetType(oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
	if !deviations.QosSchedulerConfigRequired(dut) {
		intfConfig.GetOrCreateSubinterface(0).SetIndex(0)
	}
	gnmi.Update(t, dut, gnmi.OC().Interface(dp.Name()).Config(), intfConfig)
}

// qosStateUnsupported reports whether the DUT lacks QoS state paths, in which
// case the matching config paths are read instead.
func qosStateUnsupported(dut *ondatra.DUTDevice) bool {
	return deviations.QosGetStatePathUnsupported(dut) || deviations.StatePathsUnsupported(dut)
}

// configPollInterval is the delay between config path reads in verifyLeaf.
const configPollInterval = 2 * time.Second

// verifyLeaf waits up to timeout for a QoS leaf to satisfy pred, and reports a
// test error showing want as the expected value if it does not.
//
// By default it watches the state path. On DUTs without QoS state support (see
// qosStateUnsupported) it instead polls the config path with gNMI Get every
// configPollInterval. Get is used because some devices do not serve
// config-only paths over Subscribe, and polling allows for slow convergence,
// such as after a control processor switchover.
func verifyLeaf[T any](t testing.TB, dut *ondatra.DUTDevice, state ygnmi.SingletonQuery[T], config ygnmi.ConfigQuery[T], timeout time.Duration, want string, pred func(*ygnmi.Value[T]) bool) {
	t.Helper()
	if qosStateUnsupported(dut) {
		deadline := time.Now().Add(timeout)
		var got *ygnmi.Value[T]
		for {
			got = gnmi.LookupConfig(t, dut, config)
			if pred(got) {
				return
			}
			if !time.Now().Before(deadline) {
				break
			}
			time.Sleep(configPollInterval)
		}
		t.Errorf("verifyLeaf(%v): got %v, want %s", config, got, want)
		return
	}
	got, ok := gnmi.Watch(t, dut, state, timeout, pred).Await(t)
	if !ok {
		t.Errorf("verifyLeaf(%v): got %v, want %s", state, got, want)
	}
}

// VerifyLeaf checks that a QoS leaf equals want within timeout. It reads the
// state path, or the config path on DUTs without QoS state support.
func VerifyLeaf[T comparable](t testing.TB, dut *ondatra.DUTDevice, state ygnmi.SingletonQuery[T], config ygnmi.ConfigQuery[T], want T, timeout time.Duration) {
	t.Helper()
	verifyLeaf(t, dut, state, config, timeout, fmt.Sprintf("%v", want), func(v *ygnmi.Value[T]) bool {
		got, present := v.Val()
		return present && got == want
	})
}

// VerifyLeafRemoved checks that a QoS leaf has no value within timeout. It
// reads the same state or config path as VerifyLeaf.
func VerifyLeafRemoved[T comparable](t testing.TB, dut *ondatra.DUTDevice, state ygnmi.SingletonQuery[T], config ygnmi.ConfigQuery[T], timeout time.Duration) {
	t.Helper()
	verifyLeaf(t, dut, state, config, timeout, "no value", func(v *ygnmi.Value[T]) bool {
		return !v.IsPresent()
	})
}

// VerifyLeafDetached checks that a QoS leaf, within timeout, is either unset or
// set to a value not in unwanted. It reads the same state or config path as
// VerifyLeaf. Use it instead of VerifyLeafRemoved when a removed leaf may fall
// back to a default value rather than disappear.
func VerifyLeafDetached[T comparable](t testing.TB, dut *ondatra.DUTDevice, state ygnmi.SingletonQuery[T], config ygnmi.ConfigQuery[T], unwanted []T, timeout time.Duration) {
	t.Helper()
	verifyLeaf(t, dut, state, config, timeout, fmt.Sprintf("no value or a value other than %v", unwanted), func(v *ygnmi.Value[T]) bool {
		got, present := v.Val()
		if !present {
			return true
		}
		for _, u := range unwanted {
			if got == u {
				return false
			}
		}
		return true
	})
}
