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
	"time"

	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gnoi"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Closest supported min/max threshold pair used by devices with
// deviations.EcnSameMinMaxThresholdUnsupported, which cannot set equal values.
const (
	DeviatedMinThreshold = (uint64(8005632))
	DeviatedMaxThreshold = (uint64(8011776))
)

// wantWeight is the WRED weight set on every ECN profile. The README gives no
// value, so 0 (instantaneous queue depth) is used, matching the DP-1.2 test.
const wantWeight = uint32(0)

// ecnProfiles lists every queue management profile this test may create.
var ecnProfiles = []string{"ECN_PROFILE_1", "ECN_PROFILE_2", "ECN_PROFILE_3", "ECN_PROFILE_NEG"}

// removeECNConfig detaches the ECN profile from queueName on dp and deletes all
// ecnProfiles, logging and ignoring errors. Used by t.Cleanup if DP-1.3.5 did not complete.
func removeECNConfig(t *testing.T, dut *ondatra.DUTDevice, dp *ondatra.Port, queueName string) {
	t.Helper()
	bestEffort := func(desc string, fn func(t testing.TB)) {
		if msg := testt.CaptureFatal(t, fn); msg != nil {
			t.Logf("Cleanup: %s failed (ignored): %s", desc, *msg)
		}
	}
	qosIntfID := qoscfg.QosInterfaceID(dut, dp.Name())
	bestEffort("detach queue management profile", func(t testing.TB) {
		gnmi.Delete(t, dut, gnmi.OC().Qos().Interface(qosIntfID).Output().Queue(queueName).QueueManagementProfile().Config())
	})
	for _, p := range ecnProfiles {
		bestEffort("delete queue management profile "+p, func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile(p).Config())
		})
	}
}

// newValidECNProfile adds to q a valid queue management profile named name
// with the DP-1.3.2 ECN parameters, and returns its wred/uniform container.
// DP-1.3.4 negative tests start from it and change a single leaf, so a
// rejected Set can only be caused by that leaf.
func newValidECNProfile(dut *ondatra.DUTDevice, q *oc.Qos, name string) *oc.Qos_QueueManagementProfile_Wred_Uniform {
	profile := q.GetOrCreateQueueManagementProfile(name)
	profile.SetName(name)
	uniform := profile.GetOrCreateWred().GetOrCreateUniform()
	uniform.SetEnableEcn(true)
	uniform.SetDrop(false)
	uniform.SetMaxDropProbabilityPercent(100)
	uniform.SetMinThreshold(3276800)
	uniform.SetMaxThreshold(6553600)
	if !deviations.QosSetWeightConfigUnsupported(dut) {
		uniform.SetWeight(wantWeight)
	}
	return uniform
}

func TestQosEcnConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")

	// teardownVerified is set when DP-1.3.5 removes and verifies all ECN config.
	teardownVerified := false
	// Safety net for DP-1.3.5: registered before any configuration is pushed so
	// the ECN config is removed even if a subtest panics before DP-1.3.5 runs.
	// It does nothing when DP-1.3.5 already completed successfully.
	t.Cleanup(func() {
		if teardownVerified {
			return
		}
		t.Log("DP-1.3.5 did not complete; removing ECN configuration (best effort)")
		removeECNConfig(t, dut, dp1, "0")
	})

	// DP-1.3 Test environment setup
	t.Run("Test environment setup", func(t *testing.T) {
		// DP-1.3 Test environment setup: DUT port-1 connects to ATE port-1
		// (testbed). Configure the DUT port-1 interface used by all subtests.
		qoscfg.ConfigureInterfaceSetup(t, dut, dp1)

		// DP-1.3 Test environment setup: Build the base QoS config (classifier,
		// forwarding group, queue "0").
		qos := &oc.Root{}
		q := qos.GetOrCreateQos()

		// DP-1.3 Test environment setup: Create an input IPv4 classifier to match traffic intended for the QoS queue being tested
		className := "ipv4_dscp_classifier"
		targetGroupName := "target-group-0"
		qoscfg.ConfigureIPv4DSCPClassifier(t, dut, q, className, targetGroupName, 10 /* dscp 10 for example */)
		q.GetOrCreateForwardingGroup(targetGroupName).SetOutputQueue("0")
		qQueue := q.GetOrCreateQueue("0")
		qQueue.SetName("0")
		if deviations.QOSQueueRequiresID(dut) {
			qQueue.SetQueueId(0)
		}

		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

		// DP-1.3 Test environment setup: Apply the classifier to the input of DUT port-1
		// Use a separate QoS struct so the classifier definitions are not pushed
		// again together with the interface input binding.
		d2 := &oc.Root{}
		q2 := d2.GetOrCreateQos()
		qoscfg.SetInputClassifier(t, dut, q2, dp1.Name(), oc.Input_Classifier_Type_IPV4, className)

		// DP-1.3 Test environment setup: Validate the classifier is applied to the input of DUT port-1.
		inClassifierPath := gnmi.OC().Qos().Interface(qoscfg.QosClassifierInterfaceID(dut, dp1.Name())).Input().Classifier(oc.Input_Classifier_Type_IPV4)
		qoscfg.VerifyLeaf(t, dut, inClassifierPath.Name().State(), inClassifierPath.Name().Config(), className, time.Minute)
	})

	t.Run("DP-1.3.1 - 80KB min-threshold equal max-threshold", func(t *testing.T) {
		// DP-1.3.1 Step 1 - Generate DUT configuration
		qos := &oc.Root{}
		q := qos.GetOrCreateQos()

		profileName := "ECN_PROFILE_1"
		queueMgmtProfile := q.GetOrCreateQueueManagementProfile(profileName)
		queueMgmtProfile.SetName(profileName)
		uniform := queueMgmtProfile.GetOrCreateWred().GetOrCreateUniform()
		uniform.SetEnableEcn(true)
		uniform.SetDrop(false)
		uniform.SetMaxDropProbabilityPercent(100)
		if !deviations.QosSetWeightConfigUnsupported(dut) {
			uniform.SetWeight(wantWeight)
		}

		// DP-1.3.1 Step 1: min-threshold = max-threshold = 81920 (80KB). Devices with
		// deviations.EcnSameMinMaxThresholdUnsupported cannot set equal values, so the
		// closest supported pair (DeviatedMinThreshold/DeviatedMaxThreshold) is used.
		var wantMinThreshold uint64 = 81920
		var wantMaxThreshold uint64 = 81920
		if deviations.EcnSameMinMaxThresholdUnsupported(dut) {
			wantMinThreshold = DeviatedMinThreshold
			wantMaxThreshold = DeviatedMaxThreshold
		}
		uniform.SetMinThreshold(wantMinThreshold)
		uniform.SetMaxThreshold(wantMaxThreshold)

		queueName := "0"
		qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueName, profileName)

		// DP-1.3.1 Step 2 - Push configuration to DUT using gNMI Set with REPLACE option.
		batch := &gnmi.SetBatch{}
		qoscfg.ReplaceQueueManagementProfile(t, dut, batch, q, dp1.Name(), queueName, profileName)
		batch.Set(t, dut)

		// DP-1.3.1 Step 3 - Validation with pass/fail criteria: verify each
		// wred/uniform state leaf (config leaf on devices without QoS state
		// support) using gnmi.Watch/Await via qoscfg.VerifyLeaf.
		uniformPath := gnmi.OC().Qos().QueueManagementProfile(profileName).Wred().Uniform()
		qoscfg.VerifyLeaf(t, dut, uniformPath.EnableEcn().State(), uniformPath.EnableEcn().Config(), true, time.Minute)
		qoscfg.VerifyLeaf(t, dut, uniformPath.MaxDropProbabilityPercent().State(), uniformPath.MaxDropProbabilityPercent().Config(), 100, time.Minute)
		qoscfg.VerifyLeaf(t, dut, uniformPath.MinThreshold().State(), uniformPath.MinThreshold().Config(), wantMinThreshold, time.Minute)
		qoscfg.VerifyLeaf(t, dut, uniformPath.MaxThreshold().State(), uniformPath.MaxThreshold().Config(), wantMaxThreshold, time.Minute)
		if !deviations.DropWeightLeavesUnsupported(dut) {
			qoscfg.VerifyLeaf(t, dut, uniformPath.Drop().State(), uniformPath.Drop().Config(), false, time.Minute)
		}
		if !deviations.QosSetWeightConfigUnsupported(dut) {
			qoscfg.VerifyLeaf(t, dut, uniformPath.Weight().State(), uniformPath.Weight().Config(), wantWeight, time.Minute)
		}

		// DP-1.3.1 Step 4 - Validate ECN profile application
		outQueuePath := gnmi.OC().Qos().Interface(qoscfg.QosInterfaceID(dut, dp1.Name())).Output().Queue(queueName)
		qoscfg.VerifyLeaf(t, dut, outQueuePath.Name().State(), outQueuePath.Name().Config(), queueName, time.Minute)
		qoscfg.VerifyLeaf(t, dut, outQueuePath.QueueManagementProfile().State(), outQueuePath.QueueManagementProfile().Config(), profileName, time.Minute)

		// DP-1.3.1 Step 5 - Trigger a supervisor switchover via gNOI SwitchControlProcessor.
		// Mandatory on multi form factor DUTs (2+ controller cards); a switchover
		// failure fails the test. Only fixed form factor DUTs skip Steps 5-6.
		t.Log("Triggering supervisor switchover")
		switched, _, prevStandby := gnoi.SwitchControlProcessor(t, dut)
		if !switched {
			t.Log("Skipping DP-1.3.1 Steps 5-6: fixed form factor DUT has fewer than two controller cards")
			return
		}

		t.Log("Waiting for device to reconnect...")
		startT := time.Now()
		for {
			// DP-1.3.1 Step 5: Wait for the new active supervisor. gNMI is unreachable
			// during the switchover, so gnmi.Watch/Await fail on dial and cannot wait
			// across the outage. A 30s back-off between reconnect attempts is therefore
			// required (same pattern as feature/gnoi/system/tests/supervisor_switchover_test).
			time.Sleep(30 * time.Second)
			errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
			})
			if errMsg == nil {
				t.Log("Device successfully reconnected")
				break
			}
			if time.Since(startT) > 10*time.Minute {
				t.Fatalf("Device failed to reconnect within 10 minutes")
			}
		}

		// DP-1.3.1 Step 6 - "Once the new supervisor is active": wait until the new
		// active card reports switchover-ready before repeating Steps 3 and 4.
		gnoi.AwaitSwitchoverReady(t, dut, prevStandby, 30*time.Minute)

		// DP-1.3.1 Step 6 - Repeat checks from Steps 3 and 4. Also verify the
		// previous standby card is now active.
		cards := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
		if _, newActive := components.FindStandbyControllerCard(t, dut, cards); newActive != prevStandby {
			t.Errorf("Active controller card after switchover: got %q, want %q", newActive, prevStandby)
		}
		qoscfg.VerifyLeaf(t, dut, uniformPath.EnableEcn().State(), uniformPath.EnableEcn().Config(), true, time.Minute)
		qoscfg.VerifyLeaf(t, dut, uniformPath.MaxDropProbabilityPercent().State(), uniformPath.MaxDropProbabilityPercent().Config(), 100, time.Minute)
		qoscfg.VerifyLeaf(t, dut, uniformPath.MinThreshold().State(), uniformPath.MinThreshold().Config(), wantMinThreshold, time.Minute)
		qoscfg.VerifyLeaf(t, dut, uniformPath.MaxThreshold().State(), uniformPath.MaxThreshold().Config(), wantMaxThreshold, time.Minute)
		if !deviations.DropWeightLeavesUnsupported(dut) {
			qoscfg.VerifyLeaf(t, dut, uniformPath.Drop().State(), uniformPath.Drop().Config(), false, time.Minute)
		}
		if !deviations.QosSetWeightConfigUnsupported(dut) {
			qoscfg.VerifyLeaf(t, dut, uniformPath.Weight().State(), uniformPath.Weight().Config(), wantWeight, time.Minute)
		}
		qoscfg.VerifyLeaf(t, dut, outQueuePath.Name().State(), outQueuePath.Name().Config(), queueName, time.Minute)
		qoscfg.VerifyLeaf(t, dut, outQueuePath.QueueManagementProfile().State(), outQueuePath.QueueManagementProfile().Config(), profileName, time.Minute)
	})

	t.Run("DP-1.3.2 - Threshold in MB, min-threshold not-equal max-threshold", func(t *testing.T) {
		// DP-1.3.2 Step 1 - Generate DUT configuration
		qos := &oc.Root{}
		q := qos.GetOrCreateQos()

		profileName := "ECN_PROFILE_2"
		queueMgmtProfile := q.GetOrCreateQueueManagementProfile(profileName)
		queueMgmtProfile.SetName(profileName)
		uniform := queueMgmtProfile.GetOrCreateWred().GetOrCreateUniform()
		uniform.SetEnableEcn(true)
		uniform.SetDrop(false)
		uniform.SetMaxDropProbabilityPercent(100)
		uniform.SetMinThreshold(3276800)
		uniform.SetMaxThreshold(6553600)
		if !deviations.QosSetWeightConfigUnsupported(dut) {
			uniform.SetWeight(wantWeight)
		}

		queueName := "0"
		qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueName, profileName)

		// DP-1.3.2 Step 2 - Push configuration to DUT using gNMI Set with REPLACE option.
		batch := &gnmi.SetBatch{}
		qoscfg.ReplaceQueueManagementProfile(t, dut, batch, q, dp1.Name(), queueName, profileName)
		batch.Set(t, dut)

		// DP-1.3.2 Step 3 - Validation with pass/fail criteria: verify each
		// wred/uniform state leaf (config leaf on devices without QoS state
		// support) using gnmi.Watch/Await via qoscfg.VerifyLeaf.
		uniformPath := gnmi.OC().Qos().QueueManagementProfile(profileName).Wred().Uniform()
		qoscfg.VerifyLeaf(t, dut, uniformPath.EnableEcn().State(), uniformPath.EnableEcn().Config(), true, time.Minute)
		qoscfg.VerifyLeaf(t, dut, uniformPath.MinThreshold().State(), uniformPath.MinThreshold().Config(), 3276800, time.Minute)
		qoscfg.VerifyLeaf(t, dut, uniformPath.MaxThreshold().State(), uniformPath.MaxThreshold().Config(), 6553600, time.Minute)
		qoscfg.VerifyLeaf(t, dut, uniformPath.MaxDropProbabilityPercent().State(), uniformPath.MaxDropProbabilityPercent().Config(), 100, time.Minute)
		if !deviations.DropWeightLeavesUnsupported(dut) {
			qoscfg.VerifyLeaf(t, dut, uniformPath.Drop().State(), uniformPath.Drop().Config(), false, time.Minute)
		}
		if !deviations.QosSetWeightConfigUnsupported(dut) {
			qoscfg.VerifyLeaf(t, dut, uniformPath.Weight().State(), uniformPath.Weight().Config(), wantWeight, time.Minute)
		}

		// DP-1.3.2 Step 4 - Validate ECN profile application
		outQueuePath := gnmi.OC().Qos().Interface(qoscfg.QosInterfaceID(dut, dp1.Name())).Output().Queue(queueName)
		qoscfg.VerifyLeaf(t, dut, outQueuePath.Name().State(), outQueuePath.Name().Config(), queueName, time.Minute)
		qoscfg.VerifyLeaf(t, dut, outQueuePath.QueueManagementProfile().State(), outQueuePath.QueueManagementProfile().Config(), profileName, time.Minute)
	})

	t.Run("DP-1.3.3 - Threshold in percentage, min-threshold not-equal max-threshold", func(t *testing.T) {
		// DP-1.3.3 Step 1 - Generate DUT configuration
		qos := &oc.Root{}
		q := qos.GetOrCreateQos()

		profileName := "ECN_PROFILE_3"
		queueMgmtProfile := q.GetOrCreateQueueManagementProfile(profileName)
		queueMgmtProfile.SetName(profileName)
		uniform := queueMgmtProfile.GetOrCreateWred().GetOrCreateUniform()
		uniform.SetEnableEcn(true)
		uniform.SetDrop(false)
		uniform.SetMaxDropProbabilityPercent(100)
		uniform.SetMinThresholdPercent(1)
		uniform.SetMaxThresholdPercent(2)
		if !deviations.QosSetWeightConfigUnsupported(dut) {
			uniform.SetWeight(wantWeight)
		}

		queueName := "0"
		qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueName, profileName)

		// DP-1.3.3 Step 2 - Push configuration to DUT using gNMI Set with REPLACE option.
		batch := &gnmi.SetBatch{}
		qoscfg.ReplaceQueueManagementProfile(t, dut, batch, q, dp1.Name(), queueName, profileName)
		batch.Set(t, dut)

		// DP-1.3.3 Step 3 - Validation with pass/fail criteria: verify each
		// wred/uniform state leaf (config leaf on devices without QoS state
		// support) using gnmi.Watch/Await via qoscfg.VerifyLeaf.
		uniformPath := gnmi.OC().Qos().QueueManagementProfile(profileName).Wred().Uniform()
		qoscfg.VerifyLeaf(t, dut, uniformPath.EnableEcn().State(), uniformPath.EnableEcn().Config(), true, time.Minute)
		qoscfg.VerifyLeaf(t, dut, uniformPath.MinThresholdPercent().State(), uniformPath.MinThresholdPercent().Config(), 1, time.Minute)
		qoscfg.VerifyLeaf(t, dut, uniformPath.MaxThresholdPercent().State(), uniformPath.MaxThresholdPercent().Config(), 2, time.Minute)
		qoscfg.VerifyLeaf(t, dut, uniformPath.MaxDropProbabilityPercent().State(), uniformPath.MaxDropProbabilityPercent().Config(), 100, time.Minute)
		if !deviations.DropWeightLeavesUnsupported(dut) {
			qoscfg.VerifyLeaf(t, dut, uniformPath.Drop().State(), uniformPath.Drop().Config(), false, time.Minute)
		}
		if !deviations.QosSetWeightConfigUnsupported(dut) {
			qoscfg.VerifyLeaf(t, dut, uniformPath.Weight().State(), uniformPath.Weight().Config(), wantWeight, time.Minute)
		}

		// DP-1.3.3 Step 4 - Validate ECN profile application
		outQueuePath := gnmi.OC().Qos().Interface(qoscfg.QosInterfaceID(dut, dp1.Name())).Output().Queue(queueName)
		qoscfg.VerifyLeaf(t, dut, outQueuePath.Name().State(), outQueuePath.Name().Config(), queueName, time.Minute)
		qoscfg.VerifyLeaf(t, dut, outQueuePath.QueueManagementProfile().State(), outQueuePath.QueueManagementProfile().Config(), profileName, time.Minute)
	})

	t.Run("DP-1.3.4 - Negative Test Cases", func(t *testing.T) {
		// DP-1.3.4: each negative test builds a fresh QoS config. Profiles start
		// from a valid ECN profile (newValidECNProfile) with a single invalid leaf,
		// so a rejection can only be caused by the element under test.
		profileName := "ECN_PROFILE_NEG"
		queueName := "0"
		outQueuePath := gnmi.OC().Qos().Interface(qoscfg.QosInterfaceID(dut, dp1.Name())).Output().Queue(queueName)

		// DP-1.3.4 Negative Test 1: min-threshold (81920) > max-threshold (40960).
		// Verify the gNMI Set is rejected.
		t.Run("Negative Test 1 (min > max threshold)", func(t *testing.T) {
			if deviations.EcnMinGreaterMaxThresholdUnsupported(dut) {
				t.Skip("Device does not support validating min-threshold > max-threshold")
			}
			q := &oc.Qos{}
			uniform := newValidECNProfile(dut, q, profileName)
			uniform.SetMinThreshold(81920)
			uniform.SetMaxThreshold(40960)
			// Some devices validate cross-leaf constraints only when the profile is
			// attached to a queue, so attach it to queue "0".
			qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueName, profileName)

			// testt.ExpectFatal fails the test if the Set is accepted.
			errStr := testt.ExpectFatal(t, func(t testing.TB) {
				gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)
			})
			t.Logf("min-threshold > max-threshold rejected as expected: %s", errStr)
		})

		// DP-1.3.4 Negative Test 2: max-drop-probability-percent = 101 (out of
		// range). Verify the gNMI Set is rejected.
		t.Run("Negative Test 2 (invalid max-drop-prob)", func(t *testing.T) {
			q := &oc.Qos{}
			newValidECNProfile(dut, q, profileName).SetMaxDropProbabilityPercent(101)

			errStr := testt.ExpectFatal(t, func(t testing.TB) {
				gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)
			})
			t.Logf("max-drop-probability-percent 101 rejected as expected: %s", errStr)
		})

		// DP-1.3.4 Negative Test 3: assign non-existent profile "BOGUS_PROFILE" to
		// queue "0". Verify the gNMI Set is rejected.
		t.Run("Negative Test 3 (non-existent profile)", func(t *testing.T) {
			q := &oc.Qos{}
			qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueName, "BOGUS_PROFILE")

			errStr := testt.ExpectFatal(t, func(t testing.TB) {
				gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)
			})
			t.Logf("Non-existent profile assignment rejected as expected: %s", errStr)
		})

		// DP-1.3.4 Negative Test 4: apply a valid profile to queue "0", then attempt
		// to delete it while it is still applied. Verify the deletion is rejected.
		t.Run("Negative Test 4 (delete active profile)", func(t *testing.T) {
			// Created here (same values as DP-1.3.2) so this test does not depend on
			// DP-1.3.2 passing.
			activeProfile := "ECN_PROFILE_2"

			// DP-1.3.4 Negative Test 4 - "Apply a valid queue-management-profile to
			// an interface's queue".
			q := &oc.Qos{}
			newValidECNProfile(dut, q, activeProfile)
			qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueName, activeProfile)
			gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)
			qoscfg.VerifyLeaf(t, dut, outQueuePath.QueueManagementProfile().State(), outQueuePath.QueueManagementProfile().Config(), activeProfile, time.Minute)

			// DP-1.3.4 Negative Test 4 - "attempt to delete that queue-management-profile
			// while it is still actively applied ... Verify the deletion is rejected."
			errStr := testt.ExpectFatal(t, func(t testing.TB) {
				gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile(activeProfile).Config())
			})
			t.Logf("Deletion of active profile %s rejected as expected: %s", activeProfile, errStr)

			// DP-1.3.4 Negative Test 4: the rejected delete must leave the profile applied.
			qoscfg.VerifyLeaf(t, dut, outQueuePath.QueueManagementProfile().State(), outQueuePath.QueueManagementProfile().Config(), activeProfile, time.Minute)
		})
	})

	t.Run("DP-1.3.5 - Teardown and Cleanup Verification", func(t *testing.T) {
		queueName := "0"

		// DP-1.3.5 Step 1: Detach profile from interface
		qosIntfID := qoscfg.QosInterfaceID(dut, dp1.Name())
		gnmi.Delete(t, dut, gnmi.OC().Qos().Interface(qosIntfID).Output().Queue(queueName).QueueManagementProfile().Config())

		// DP-1.3.5 Step 2: Validate profile is detached
		outQueuePath := gnmi.OC().Qos().Interface(qosIntfID).Output().Queue(queueName)
		allProfiles := []string{"ECN_PROFILE_1", "ECN_PROFILE_2", "ECN_PROFILE_3"}
		qoscfg.VerifyLeafDetached(t, dut, outQueuePath.QueueManagementProfile().State(), outQueuePath.QueueManagementProfile().Config(), allProfiles, time.Minute)

		// DP-1.3.5 Step 3: Delete profile globally
		for _, profileName := range allProfiles {
			gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile(profileName).Config())
		}

		// DP-1.3.5 Step 4: Validate profile is removed
		for _, profileName := range allProfiles {
			profilePath := gnmi.OC().Qos().QueueManagementProfile(profileName)
			qoscfg.VerifyLeafRemoved(t, dut, profilePath.State(), profilePath.Config(), time.Minute)
		}

		// DP-1.3.5 complete: the t.Cleanup safety net is not needed.
		teardownVerified = !t.Failed()
	})
}
