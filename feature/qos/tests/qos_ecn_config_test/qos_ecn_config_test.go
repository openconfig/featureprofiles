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

// For devices where minimum and maximum threshold values can't be the same,
// as well as it should be a multiple of 6,144 bytes
const (
	DeviatedMinThreshold = (uint64(8005632))
	DeviatedMaxThreshold = (uint64(8011776))
)

// wantWeight is the WRED exponential weighting factor applied to every ECN profile
// in this test.
//
// The test specification lists the weight leaf among the paths it covers but does
// not state a value for it, so this matches the value the DP-1.2 QoS policy config
// test configures on the same leaf. Zero selects the instantaneous queue depth
// rather than a moving average.
const wantWeight = uint32(0)

func TestQosEcnConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")

	// DP-1.3 Test environment setup
	t.Run("Test environment setup", func(t *testing.T) {
		qoscfg.ConfigureInterfaceSetup(t, dut, dp1)

		//  Step 1 - Generate DUT configuration
		qos := &oc.Root{}
		q := qos.GetOrCreateQos()

		// DP-1.3 Step: Create an input IPv4 classifier to match traffic intended for the QoS queue being tested
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

		// DP-1.3 Step: Apply the classifier to the input of DUT port-1
		// For Juniper, we must NOT pass the same `q` struct containing the classifiers
		// to the Interface/Input logic, or else `gnmi.Update` will push both mappings together.
		d2 := &oc.Root{}
		q2 := d2.GetOrCreateQos()
		qoscfg.SetInputClassifier(t, dut, q2, dp1.Name(), oc.Input_Classifier_Type_IPV4, className)

		// DP-1.3 Step: Validate the classifier is applied to the input of DUT port-1.
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
		qoscfg.ReplaceQueueManagementProfile(t, dut, q, dp1.Name(), queueName, profileName)

		// DP-1.3.1 Step 3 - Validation with pass/fail criteria
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

		if !deviations.SupervisorSwitchoverUnsupported(dut) {
			// DP-1.3.1 Step 5 - Trigger a supervisor switchover
			t.Log("Triggering supervisor switchover")
			switched, _, prevStandby := gnoi.SwitchControlProcessor(t, dut)
			if !switched {
				t.Log("Skipping DP-1.3.1 Steps 5-6: DUT has fewer than two controller cards")
				return
			}

			t.Log("Waiting for device to reconnect...")
			startT := time.Now()
			for {
				// Sleep is absolutely necessary here to prevent gNMI connection spam during supervisor switchover reboot, as gnmi.Await cannot catch native dial panics.
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

			// DP-1.3.1 Step 6 - Once the new supervisor is active, repeat checks
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
		} else {
			t.Log("Skipping supervisor switchover as unsupported on this platform")
		}
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
		qoscfg.ReplaceQueueManagementProfile(t, dut, q, dp1.Name(), queueName, profileName)

		// DP-1.3.2 Step 3 - Validation with pass/fail criteria
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
		if deviations.EcnThresholdPercentUnsupported(dut) {
			t.Skip("Percentage-based ECN thresholds not supported on this platform")
		}
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
		qoscfg.ReplaceQueueManagementProfile(t, dut, q, dp1.Name(), queueName, profileName)

		// DP-1.3.3 Step 3 - Validation with pass/fail criteria
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
		// DP-1.3.4 Step 1 - Generate DUT configuration
		qos := &oc.Root{}
		q := qos.GetOrCreateQos()
		profileName := "ECN_PROFILE_NEG"

		// Negative Test 1: min-threshold > max-threshold
		t.Run("Negative Test 1 (min > max threshold)", func(t *testing.T) {
			if deviations.EcnMinGreaterMaxThresholdUnsupported(dut) {
				t.Skip("Device does not support validating min-threshold > max-threshold")
			}
			profile1 := q.GetOrCreateQueueManagementProfile(profileName)
			uniform := profile1.GetOrCreateWred().GetOrCreateUniform()
			uniform.SetMinThreshold(81920)
			uniform.SetMaxThreshold(40960)

			// Many network NPUs (like Arista EOS) only enforce cross-leaf semantic constraints
			// at the moment a profile is actually attached to a hardware data-plane queue.
			queueName := "0"
			qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueName, profileName)

			errStr := testt.ExpectFatal(t, func(t testing.TB) {
				gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)
			})
			if errStr == "" {
				t.Errorf("Expected error for min-threshold > max-threshold, got nil")
			}

			// Detach from the local struct after the operation to preserve integrity for subsequent tests.
			q.GetOrCreateInterface(dp1.Name()).GetOrCreateOutput().GetOrCreateQueue(queueName).QueueManagementProfile = nil
			q.DeleteQueueManagementProfile(profileName)
		})

		// Negative Test 2: Invalid max-drop-probability-percent
		t.Run("Negative Test 2 (invalid max-drop-prob)", func(t *testing.T) {
			uniform := q.GetOrCreateQueueManagementProfile(profileName).GetOrCreateWred().GetOrCreateUniform()
			uniform.SetMaxDropProbabilityPercent(101)

			errStr := testt.ExpectFatal(t, func(t testing.TB) {
				gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)
			})
			if errStr == "" {
				t.Errorf("Expected error for max-drop-probability-percent > 100, got nil")
			}
		})

		// Negative Test 3: Non-existent Profile Assignment
		t.Run("Negative Test 3 (non-existent profile)", func(t *testing.T) {
			queueName := "0"
			qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueName, "BOGUS_PROFILE")

			errStr := testt.ExpectFatal(t, func(t testing.TB) {
				gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)
			})
			if errStr == "" {
				t.Errorf("Expected error when assigning non-existent profile, got nil")
			}
			qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueName, "ECN_PROFILE_2")
		})

		// Negative Test 4: Invalid Profile Deletion
		t.Run("Negative Test 4 (delete active profile)", func(t *testing.T) {
			// Profile ECN_PROFILE_2 is actively applied from DP-1.3.2. Attempt to delete it.
			deletedProfile := q.QueueManagementProfile["ECN_PROFILE_2"]
			delete(q.QueueManagementProfile, "ECN_PROFILE_2")

			errStr := testt.ExpectFatal(t, func(t testing.TB) {
				gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)
			})
			if errStr == "" {
				t.Errorf("Expected error when deleting active profile ECN_PROFILE_2, got nil")
			}
			if deletedProfile != nil {
				q.QueueManagementProfile["ECN_PROFILE_2"] = deletedProfile
			}
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
		deletedProfiles := []string{"ECN_PROFILE_1", "ECN_PROFILE_2"}
		if !deviations.EcnThresholdPercentUnsupported(dut) {
			deletedProfiles = append(deletedProfiles, "ECN_PROFILE_3")
		}
		for _, profileName := range deletedProfiles {
			gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile(profileName).Config())
		}

		// DP-1.3.5 Step 4: Validate profile is removed
		for _, profileName := range deletedProfiles {
			profilePath := gnmi.OC().Qos().QueueManagementProfile(profileName)
			qoscfg.VerifyLeafRemoved(t, dut, profilePath.State(), profilePath.Config(), time.Minute)
		}
	})
}
