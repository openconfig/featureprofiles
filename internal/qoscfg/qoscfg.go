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
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// SetForwardingGroup sets a forwarding group in the specified QoS config.
func SetForwardingGroup(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, groupName, queueName string) {
	t.Helper()
	qos.GetOrCreateForwardingGroup(groupName).SetOutputQueue(queueName)
	qos.GetOrCreateQueue(queueName)
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), qos)
}

// SetInputClassifier sets an input classifier in the specified QoS config.
func SetInputClassifier(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, intfID string, classType oc.E_Input_Classifier_Type, className string) {
	t.Helper()
	qosIntfID := intfID
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		qosIntfID += ".0"
	}
	intf := qos.GetOrCreateInterface(qosIntfID)
	intf.GetOrCreateInterfaceRef().SetInterface(intfID)
	if dut.Vendor() != ondatra.CISCO {
		intf.GetOrCreateInterfaceRef().SetSubinterface(0)
	}
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}
	intf.GetOrCreateInput().GetOrCreateClassifier(classType).SetName(className)
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
// qos; add it to a SetBatch with ReplaceQueueManagementProfile.
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

// ReplaceQueueManagementProfile appends to batch the operations that push qos
// to the DUT in one gNMI SetRequest. The caller sends it with batch.Set:
//   - REPLACE queue management profile profileName.
//   - UPDATE (merge) the rest of qos, including output queue queueName on
//     interface intfID.
//
// Only the profile subtree is replaced, so each test case gets a clean profile
// without deleting the classifier, forwarding group and queues created during
// test setup, and without deleting pre-existing system queues on the interface.
// The UPDATE is built from a copy of qos without the replaced profile subtree,
// so no path is sent twice in the SetRequest. qos itself is not modified; call
// batch.Set before editing qos again.
//
// Call BuildOutputQueueManagementProfile with the same arguments first. The
// test fails if qos does not contain the profile or the output queue.
func ReplaceQueueManagementProfile(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch, qos *oc.Qos, intfID, queueName, profileName string) {
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

	// Remove the replaced subtree from a copy rather than from qos: SetBatch
	// marshals values only when Set is called, so a temporary edit of qos that
	// is restored before then would have no effect.
	c, err := ygot.DeepCopy(qos)
	if err != nil {
		t.Fatalf("ReplaceQueueManagementProfile: cannot copy qos: %v", err)
	}
	rest := c.(*oc.Qos)
	delete(rest.QueueManagementProfile, profileName)
	// Drop lists and containers left empty by the removal: some devices reject
	// empty JSON lists (e.g. "queue-management-profile": []) in a SetRequest.
	if len(rest.QueueManagementProfile) == 0 {
		rest.QueueManagementProfile = nil
	}
	ygot.PruneEmptyBranches(rest)

	// On DUTs with QosSchedulerConfigRequired, swapping one queue-management-profile
	// for another while the output scheduler-policy remains attached causes the
	// device to synthesize a second egress service-policy in the same commit
	// ("Invalid multi policy type in egress"). Remove the existing output and
	// scheduler-policy first so the subsequent batch attaches a single egress policy.
	if deviations.QosSchedulerConfigRequired(dut) {
		schedPath := gnmi.OC().Qos().Interface(qosIntfID).Output().SchedulerPolicy().Name().Config()
		if lookupConfigWithGet(t, dut, schedPath).IsPresent() {
			clearBatch := &gnmi.SetBatch{}
			gnmi.BatchDelete(clearBatch, gnmi.OC().Qos().Interface(qosIntfID).Output().Config())
			gnmi.BatchDelete(clearBatch, gnmi.OC().Qos().SchedulerPolicy("scheduler").Config())
			clearBatch.Set(t, dut)
		}
	}

	gnmi.BatchReplace(batch, gnmi.OC().Qos().QueueManagementProfile(profileName).Config(), profile)
	gnmi.BatchUpdate(batch, gnmi.OC().Qos().Config(), rest)
}

// DetachOutputQueueManagementProfile detaches the queue management profile from
// output queue queueName on interface intfID. On DUTs with
// deviations.QosSchedulerConfigRequired it also removes the interface output
// and scheduler policy added by BuildOutputQueueManagementProfile.
func DetachOutputQueueManagementProfile(t testing.TB, dut *ondatra.DUTDevice, intfID, queueName string) {
	t.Helper()
	qosIntfID := QosInterfaceID(dut, intfID)
	if deviations.QosSchedulerConfigRequired(dut) {
		batch := &gnmi.SetBatch{}
		gnmi.BatchDelete(batch, gnmi.OC().Qos().Interface(qosIntfID).Output().Config())
		gnmi.BatchDelete(batch, gnmi.OC().Qos().SchedulerPolicy("scheduler").Config())
		batch.Set(t, dut)
		return
	}
	gnmi.Delete(t, dut, gnmi.OC().Qos().Interface(qosIntfID).Output().Queue(queueName).QueueManagementProfile().Config())
}

// ResyncOutputQueueManagementProfile re-establishes the DUT's internal
// OpenConfig-to-native QoS policy mapping after a control processor switchover
// on DUTs with deviations.QosSchedulerConfigRequired, and then detaches the
// output policy so subsequent test cases can attach a new profile cleanly.
func ResyncOutputQueueManagementProfile(t *testing.T, dut *ondatra.DUTDevice, qos *oc.Qos, intfID, queueName, profileName string) {
	t.Helper()
	if !deviations.QosSchedulerConfigRequired(dut) {
		return
	}
	DetachOutputQueueManagementProfile(t, dut, intfID, queueName)
	gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile(profileName).Config())

	batch := &gnmi.SetBatch{}
	ReplaceQueueManagementProfile(t, dut, batch, qos, intfID, queueName, profileName)
	batch.Set(t, dut)

	DetachOutputQueueManagementProfile(t, dut, intfID, queueName)
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

// nilValUpdateErr is the ygnmi error reported when a DUT answers a gNMI Get
// with an update that carries a path but no value.
const nilValUpdateErr = "invalid nil Val in update"

// lookupConfigWithGet reads a config leaf using gNMI Get. Some DUTs answer a
// Get for an unset leaf with an update that carries only the parent path and
// no value, which ygnmi reports as nilValUpdateErr. That response is treated
// as the leaf being absent; any other error is fatal.
func lookupConfigWithGet[T any](t testing.TB, dut *ondatra.DUTDevice, q ygnmi.ConfigQuery[T]) *ygnmi.Value[T] {
	t.Helper()
	var v *ygnmi.Value[T]
	if msg := testt.CaptureFatal(t, func(t testing.TB) {
		v = gnmi.LookupConfig(t, dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithUseGet()), q)
	}); msg != nil {
		if !strings.Contains(*msg, nilValUpdateErr) {
			t.Fatal(*msg)
		}
		t.Logf("lookupConfigWithGet(%v): DUT returned an update with no value; treating leaf as absent", q)
		return &ygnmi.Value[T]{}
	}
	return v
}

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
			got = lookupConfigWithGet(t, dut, config)
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

// ECNProfileConfig holds parameters for an OpenConfig QoS WRED/ECN queue
// management profile.
type ECNProfileConfig struct {
	Name                      string
	MinThreshold              uint64
	MaxThreshold              uint64
	EnableECN                 bool
	Drop                      bool
	Weight                    uint32
	MaxDropProbabilityPercent uint8
}

// BuildECNQueueManagementProfile populates a WRED uniform queue management
// profile in qos from cfg, respecting vendor deviations for the weight leaf.
func BuildECNQueueManagementProfile(dut *ondatra.DUTDevice, qos *oc.Qos, cfg ECNProfileConfig) {
	profile := qos.GetOrCreateQueueManagementProfile(cfg.Name)
	profile.SetName(cfg.Name)
	uniform := profile.GetOrCreateWred().GetOrCreateUniform()
	uniform.SetEnableEcn(cfg.EnableECN)
	uniform.SetDrop(cfg.Drop)
	uniform.SetMinThreshold(cfg.MinThreshold)
	uniform.SetMaxThreshold(cfg.MaxThreshold)
	uniform.SetMaxDropProbabilityPercent(cfg.MaxDropProbabilityPercent)
	if !deviations.QosSetWeightConfigUnsupported(dut) {
		uniform.SetWeight(cfg.Weight)
	}
}

// VerifyECNQueueManagementProfile verifies the state (or config on DUTs
// without QoS state support) of a WRED uniform queue management profile.
func VerifyECNQueueManagementProfile(t testing.TB, dut *ondatra.DUTDevice, cfg ECNProfileConfig, timeout time.Duration) {
	t.Helper()
	profilePath := gnmi.OC().Qos().QueueManagementProfile(cfg.Name)
	VerifyLeaf(t, dut, profilePath.Name().State(), profilePath.Name().Config(), cfg.Name, timeout)
	uniformPath := profilePath.Wred().Uniform()
	VerifyLeaf(t, dut, uniformPath.MinThreshold().State(), uniformPath.MinThreshold().Config(), cfg.MinThreshold, timeout)
	VerifyLeaf(t, dut, uniformPath.MaxThreshold().State(), uniformPath.MaxThreshold().Config(), cfg.MaxThreshold, timeout)
	VerifyLeaf(t, dut, uniformPath.EnableEcn().State(), uniformPath.EnableEcn().Config(), cfg.EnableECN, timeout)
	if !deviations.QosSetWeightConfigUnsupported(dut) {
		VerifyLeaf(t, dut, uniformPath.Weight().State(), uniformPath.Weight().Config(), cfg.Weight, timeout)
	}
	if !deviations.DropWeightLeavesUnsupported(dut) {
		VerifyLeaf(t, dut, uniformPath.Drop().State(), uniformPath.Drop().Config(), cfg.Drop, timeout)
	}
	VerifyLeaf(t, dut, uniformPath.MaxDropProbabilityPercent().State(), uniformPath.MaxDropProbabilityPercent().Config(), cfg.MaxDropProbabilityPercent, timeout)
}

// BuildInputClassifierAttachment adds an input classifier attachment for
// classType and className on intfID to qos using deviations rather than
// vendor checks.
func BuildInputClassifierAttachment(dut *ondatra.DUTDevice, qos *oc.Qos, intfID string, classType oc.E_Input_Classifier_Type, className string) {
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
	clf := intf.GetOrCreateInput().GetOrCreateClassifier(classType)
	clf.SetType(classType)
	clf.SetName(className)
}

// VerifyInputClassifier verifies that input classifier className of classType
// is attached to intfID.
func VerifyInputClassifier(t testing.TB, dut *ondatra.DUTDevice, intfID string, classType oc.E_Input_Classifier_Type, className string, timeout time.Duration) {
	t.Helper()
	qosIntfID := QosClassifierInterfaceID(dut, intfID)
	inClassifierPath := gnmi.OC().Qos().Interface(qosIntfID).Input().Classifier(classType)
	VerifyLeaf(t, dut, inClassifierPath.Name().State(), inClassifierPath.Name().Config(), className, timeout)
	VerifyLeaf(t, dut, inClassifierPath.Type().State(), inClassifierPath.Type().Config(), classType, timeout)
}

// CleanupQosSetup reverts the setup QoS classifiers, forwarding groups,
// queues, QoS interface bindings, and IPv4/IPv6 interface addresses on ports.
func CleanupQosSetup(t testing.TB, dut *ondatra.DUTDevice, ports []*ondatra.Port, classifiers, forwardingGroups, queues []string) {
	t.Helper()
	bestEffort := func(desc string, fn func(t testing.TB)) {
		if msg := testt.CaptureFatal(t, fn); msg != nil {
			t.Logf("Cleanup: %s failed (ignored): %s", desc, *msg)
		}
	}
	for _, dp := range ports {
		clfIntfID := QosClassifierInterfaceID(dut, dp.Name())
		for _, ct := range []oc.E_Input_Classifier_Type{oc.Input_Classifier_Type_IPV4, oc.Input_Classifier_Type_IPV6} {
			bestEffort(fmt.Sprintf("detach input classifier %v on %s", ct, clfIntfID), func(t testing.TB) {
				gnmi.Delete(t, dut, gnmi.OC().Qos().Interface(clfIntfID).Input().Classifier(ct).Config())
			})
		}
		bestEffort("delete QoS classifier interface "+clfIntfID, func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Qos().Interface(clfIntfID).Config())
		})
		qosIntfID := QosInterfaceID(dut, dp.Name())
		if qosIntfID != clfIntfID {
			bestEffort("delete QoS interface "+qosIntfID, func(t testing.TB) {
				gnmi.Delete(t, dut, gnmi.OC().Qos().Interface(qosIntfID).Config())
			})
		}
	}
	if deviations.QOSBufferAllocationConfigRequired(dut) {
		bestEffort("delete buffer allocation profile ballocprofile", func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Qos().BufferAllocationProfile("ballocprofile").Config())
		})
	}
	for _, className := range classifiers {
		bestEffort("delete QoS classifier "+className, func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Qos().Classifier(className).Config())
		})
	}
	for _, fg := range forwardingGroups {
		bestEffort("delete QoS forwarding group "+fg, func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Qos().ForwardingGroup(fg).Config())
		})
	}
	allQueues := append([]string(nil), queues...)
	if deviations.QosSchedulerConfigRequired(dut) {
		allQueues = append(allQueues, "7")
	}
	for _, qName := range allQueues {
		bestEffort("delete QoS queue "+qName, func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Qos().Queue(qName).Config())
		})
	}
	for _, dp := range ports {
		bestEffort("delete IPv4 subinterface config on "+dp.Name(), func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Interface(dp.Name()).Subinterface(0).Ipv4().Config())
		})
		bestEffort("delete IPv6 subinterface config on "+dp.Name(), func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Interface(dp.Name()).Subinterface(0).Ipv6().Config())
		})
	}
}
