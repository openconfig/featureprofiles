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

// ConfigureIPv4DSCPClassifier creates an IPv4 classifier that matches the specified DSCP value.
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

// QosInterfaceID returns the key of the OpenConfig qos/interfaces list entry that
// corresponds to intfID.
//
// The list is keyed by the interface name. Devices that declare
// deviations.InterfaceRefInterfaceIDFormat key it by the subinterface name instead.
func QosInterfaceID(dut *ondatra.DUTDevice, intfID string) string {
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		return intfID + ".0"
	}
	return intfID
}

// QosClassifierInterfaceID returns the key of the OpenConfig qos/interfaces list
// entry that SetInputClassifier uses for intfID.
//
// This is not always the same key as QosInterfaceID. Devices that declare
// deviations.QOSBufferAllocationConfigRequired reject an input classifier attached
// to the interface itself and require the subinterface, but still key the output
// queue list by the interface name.
func QosClassifierInterfaceID(dut *ondatra.DUTDevice, intfID string) string {
	if deviations.InterfaceRefInterfaceIDFormat(dut) || deviations.QOSBufferAllocationConfigRequired(dut) {
		return intfID + ".0"
	}
	return intfID
}

// BuildOutputQueueManagementProfile attaches a queue management profile to a specified interface queue without applying it.
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

// ReplaceQueueManagementProfile pushes qos to the device in a single gNMI Set
// transaction, applying the queue management profile and the interface output
// queue it is attached to with the REPLACE option, and the rest of qos with the
// UPDATE option.
//
// BuildOutputQueueManagementProfile must have been called on qos beforehand with
// the same intfID, queueName and profileName, so that qos already describes both
// of the subtrees being replaced.
//
// A REPLACE of the whole /qos subtree cannot be used here. That subtree also
// holds the input classifier, the forwarding group and the queue installed
// while setting up the test environment, none of which are rebuilt by the
// individual test cases, so replacing it would tear down the very configuration
// the queue management profile is being attached to.
//
// Replacing only the profile and the output queue keeps the scope to the two
// subtrees each test case fully defines, so neither inherits leaves left behind
// by a preceding case. The remaining supporting configuration in qos, such as
// the top level queue list and the buffer allocation and scheduler policies
// that some devices require before a queue management profile may be attached,
// is merged with UPDATE. gNMI groups a SetRequest by operation and applies every
// replace before every update, regardless of the order in which the operations
// were added to the batch, so the two subtrees are rewritten from scratch and
// the supporting configuration is then merged around them.
func ReplaceQueueManagementProfile(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, intfID, queueName, profileName string) {
	t.Helper()
	qosIntfID := QosInterfaceID(dut, intfID)

	// Look the subtrees up rather than creating them. Replacing a subtree that
	// the caller never populated would erase whatever the device currently holds
	// at that path instead of configuring it.
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

// ConfigureInterfaceSetup sets up the test interface and subinterface configuration vendor-neutrally.
func ConfigureInterfaceSetup(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port) {
	t.Helper()
	intfConfig := &oc.Interface{Name: ygot.String(dp.Name())}
	intfConfig.SetType(oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
	if !deviations.QosSchedulerConfigRequired(dut) {
		intfConfig.GetOrCreateSubinterface(0).SetIndex(0)
	}
	gnmi.Update(t, dut, gnmi.OC().Interface(dp.Name()).Config(), intfConfig)
}

// qosStateUnsupported reports whether the QoS OpenConfig state paths must be
// substituted by the corresponding config paths on this device.
func qosStateUnsupported(dut *ondatra.DUTDevice) bool {
	return deviations.QosGetStatePathUnsupported(dut) || deviations.StatePathsUnsupported(dut)
}

// configPollInterval is how often the configuration datastore is re-read while
// waiting for a leaf to converge. State paths are event driven and do not use it.
const configPollInterval = 2 * time.Second

// verifyLeaf waits until the value at the QoS leaf satisfies pred, reporting an
// error describing want if it does not converge within timeout.
//
// Validation is performed against the OpenConfig state path. Devices that declare
// deviations.QosGetStatePathUnsupported or deviations.StatePathsUnsupported do not
// expose the QoS state subtree, so the equivalent config path is read back instead.
//
// The config read-back intentionally uses gNMI Get rather than a Subscribe-based
// Watch, because some gNMI agents resolve SUBSCRIBE against the operational
// datastore only, which has no backing for config-only containers. Ondatra maps
// config queries issued through Get and Lookup onto the gNMI Get RPC for the
// devices that require it, whereas Await and Watch always use Subscribe.
//
// Get has no streaming equivalent of the Watch timeout, so the config branch polls
// until the same deadline. Most call sites follow an acknowledged SetRequest and
// converge on the first read, but a read that follows a control processor
// switchover needs the device to finish repopulating its configuration first.
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

// VerifyLeaf validates that a QoS leaf reports want.
func VerifyLeaf[T comparable](t testing.TB, dut *ondatra.DUTDevice, state ygnmi.SingletonQuery[T], config ygnmi.ConfigQuery[T], want T, timeout time.Duration) {
	t.Helper()
	verifyLeaf(t, dut, state, config, timeout, fmt.Sprintf("%v", want), func(v *ygnmi.Value[T]) bool {
		got, present := v.Val()
		return present && got == want
	})
}

// VerifyLeafRemoved validates that a QoS leaf reports no value, using the same
// state versus config path selection as VerifyLeaf.
func VerifyLeafRemoved[T comparable](t testing.TB, dut *ondatra.DUTDevice, state ygnmi.SingletonQuery[T], config ygnmi.ConfigQuery[T], timeout time.Duration) {
	t.Helper()
	verifyLeaf(t, dut, state, config, timeout, "no value", func(v *ygnmi.Value[T]) bool {
		return !v.IsPresent()
	})
}

// VerifyLeafDetached validates that a QoS leaf reports no value, or reports a value
// other than any of unwanted, using the same state versus config path selection as
// VerifyLeaf.
//
// This is weaker than VerifyLeafRemoved on purpose: a leaf that has been detached
// may legitimately fall back to a default rather than disappear.
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
