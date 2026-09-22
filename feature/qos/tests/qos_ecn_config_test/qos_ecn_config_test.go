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

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gnoi"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
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

func TestQosEcnConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")

	// DP-1.3 Test environment setup
	t.Run("Test environment setup", func(t *testing.T) {
		//  Step 1 - Generate DUT configuration
		qos := &oc.Root{}
		q := qos.GetOrCreateQos()

		// DP-1.3 Step: Create an input IPv4 classifier to match traffic intended for the QoS queue being tested
		className := "ipv4_dscp_classifier"
		targetGroupName := "target-group-0"
		qoscfg.ConfigureIPv4DSCPClassifier(t, q, className, targetGroupName, 10 /* dscp 10 for example */)
		q.GetOrCreateForwardingGroup(targetGroupName).SetOutputQueue("0")

		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

		// DP-1.3 Step: Apply the classifier to the input of DUT port-1
		// For Juniper, we must NOT pass the same `q` struct containing the classifiers
		// to the Interface/Input logic, or else `gnmi.Update` will push both mappings together.
		d2 := &oc.Root{}
		q2 := d2.GetOrCreateQos()
		qoscfg.SetInputClassifier(t, dut, q2, dp1.Name(), oc.Input_Classifier_Type_IPV4, className)
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
		gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)

		// DP-1.3.1 Step 3 - Validation with pass/fail criteria
		uniformPath := gnmi.OC().Qos().QueueManagementProfile(profileName).Wred().Uniform()
		if deviations.QosGetStatePathUnsupported(dut) {
			gnmi.Await(t, dut, uniformPath.EnableEcn().Config(), time.Minute, true)
			gnmi.Await(t, dut, uniformPath.MaxDropProbabilityPercent().Config(), time.Minute, 100)
			gnmi.Await(t, dut, uniformPath.MinThreshold().Config(), time.Minute, wantMinThreshold)
			gnmi.Await(t, dut, uniformPath.MaxThreshold().Config(), time.Minute, wantMaxThreshold)
		} else {
			gnmi.Await(t, dut, uniformPath.EnableEcn().State(), time.Minute, true)
			gnmi.Await(t, dut, uniformPath.MaxDropProbabilityPercent().State(), time.Minute, 100)
			if !deviations.StatePathsUnsupported(dut) {
				gnmi.Await(t, dut, uniformPath.MinThreshold().State(), time.Minute, wantMinThreshold)
				gnmi.Await(t, dut, uniformPath.MaxThreshold().State(), time.Minute, wantMaxThreshold)
			}
			if !deviations.DropWeightLeavesUnsupported(dut) {
				gnmi.Await(t, dut, uniformPath.Drop().State(), time.Minute, false)
			}
		}

		// DP-1.3.1 Step 4 - Validate ECN profile application
		qosIntfID := dp1.Name()
		if deviations.InterfaceRefInterfaceIDFormat(dut) {
			qosIntfID += ".0"
		}
		outQueuePath := gnmi.OC().Qos().Interface(qosIntfID).Output().Queue(queueName)
		if deviations.QosGetStatePathUnsupported(dut) {
			gnmi.Await(t, dut, outQueuePath.QueueManagementProfile().Config(), time.Minute, profileName)
		} else if !deviations.StatePathsUnsupported(dut) {
			gnmi.Await(t, dut, outQueuePath.QueueManagementProfile().State(), time.Minute, profileName)
		}

		// DP-1.3.1 Step 5 - Trigger a supervisor switchover
		t.Log("Triggering supervisor switchover")
		gnoi.SwitchControlProcessor(t, dut)

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

		// DP-1.3.1 Step 6 - Wait for device to reconnect and repeat checks
		if deviations.QosGetStatePathUnsupported(dut) {
			gnmi.Await(t, dut, uniformPath.EnableEcn().Config(), time.Minute, true)
			gnmi.Await(t, dut, uniformPath.MaxDropProbabilityPercent().Config(), time.Minute, 100)
			gnmi.Await(t, dut, outQueuePath.QueueManagementProfile().Config(), time.Minute, profileName)
		} else {
			gnmi.Await(t, dut, uniformPath.EnableEcn().State(), time.Minute, true)
			gnmi.Await(t, dut, uniformPath.MaxDropProbabilityPercent().State(), time.Minute, 100)
			gnmi.Await(t, dut, outQueuePath.QueueManagementProfile().State(), time.Minute, profileName)
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

		queueName := "0"
		qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueName, profileName)

		gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)

		uniformPath := gnmi.OC().Qos().QueueManagementProfile(profileName).Wred().Uniform()
		// Validation
		if deviations.QosGetStatePathUnsupported(dut) {
			gnmi.Await(t, dut, uniformPath.EnableEcn().Config(), time.Minute, true)
			gnmi.Await(t, dut, uniformPath.MinThreshold().Config(), time.Minute, 3276800)
			gnmi.Await(t, dut, uniformPath.MaxThreshold().Config(), time.Minute, 6553600)
		} else {
			gnmi.Await(t, dut, uniformPath.EnableEcn().State(), time.Minute, true)
			if !deviations.StatePathsUnsupported(dut) {
				gnmi.Await(t, dut, uniformPath.MinThreshold().State(), time.Minute, 3276800)
				gnmi.Await(t, dut, uniformPath.MaxThreshold().State(), time.Minute, 6553600)
			}
		}
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

		queueName := "0"
		qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueName, profileName)

		gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)

		uniformPath := gnmi.OC().Qos().QueueManagementProfile(profileName).Wred().Uniform()
		if deviations.QosGetStatePathUnsupported(dut) {
			gnmi.Await(t, dut, uniformPath.EnableEcn().Config(), time.Minute, true)
			gnmi.Await(t, dut, uniformPath.MinThresholdPercent().Config(), time.Minute, 1)
			gnmi.Await(t, dut, uniformPath.MaxThresholdPercent().Config(), time.Minute, 2)
		} else {
			gnmi.Await(t, dut, uniformPath.EnableEcn().State(), time.Minute, true)
			if !deviations.StatePathsUnsupported(dut) {
				gnmi.Await(t, dut, uniformPath.MinThresholdPercent().State(), time.Minute, 1)
				gnmi.Await(t, dut, uniformPath.MaxThresholdPercent().State(), time.Minute, 2)
			}
		}
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
			if deviations.InterfaceOutputQueueNonStandardName(dut) {
				// Some devices use different queue names.
				// Wait! If they use a non-standard name, it might be dynamically defined.
				// Since Negative Test 1 was hardcoded with "0" originally, we use "0".
				// Actually, DP-1.3 uses QosQueueRequiresID, etc. We just use "0" as the reference generic string,
				// avoiding complexity for a negative constraint test.
			}
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
			outQueuePath := gnmi.OC().Qos().Interface(dp1.Name()).Output().Queue(queueName)
			if deviations.InterfaceRefInterfaceIDFormat(dut) {
				outQueuePath = gnmi.OC().Qos().Interface(dp1.Name() + ".0").Output().Queue(queueName)
			}
			gnmi.Replace(t, dut, outQueuePath.QueueManagementProfile().Config(), "BOGUS_PROFILE")

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
		qosIntfID := dp1.Name()
		if deviations.InterfaceRefInterfaceIDFormat(dut) {
			qosIntfID += ".0"
		}
		gnmi.Delete(t, dut, gnmi.OC().Qos().Interface(qosIntfID).Output().Queue(queueName).QueueManagementProfile().Config())

		// DP-1.3.5 Step 2: Validate profile is detached
		outQueuePath := gnmi.OC().Qos().Interface(qosIntfID).Output().Queue(queueName)
		if deviations.QosGetStatePathUnsupported(dut) {
			val, pf := gnmi.Watch(t, dut, outQueuePath.QueueManagementProfile().Config(), time.Minute, func(v *ygnmi.Value[string]) bool {
				valStr, present := v.Val()
				return !present || valStr == ""
			}).Await(t)
			if !pf {
				t.Errorf("Profile was not successfully detached, last val: %v", val)
			}
		} else if !deviations.StatePathsUnsupported(dut) {
			val, pf := gnmi.Watch(t, dut, outQueuePath.QueueManagementProfile().State(), time.Minute, func(v *ygnmi.Value[string]) bool {
				valStr, present := v.Val()
				return !present || valStr == ""
			}).Await(t)
			if !pf {
				t.Errorf("Profile was not successfully detached, last val: %v", val)
			}
		}

		// DP-1.3.5 Step 3: Delete profile globally
		gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile("ECN_PROFILE_1").Config())
		gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile("ECN_PROFILE_2").Config())
		if !deviations.EcnThresholdPercentUnsupported(dut) {
			gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile("ECN_PROFILE_3").Config())
		}

		// DP-1.3.5 Step 4: Validate profile is removed
		if !deviations.EcnThresholdPercentUnsupported(dut) {
			val2, pf2 := gnmi.Watch(t, dut, gnmi.OC().Qos().QueueManagementProfile("ECN_PROFILE_3").State(), time.Minute, func(v *ygnmi.Value[*oc.Qos_QueueManagementProfile]) bool {
				return !v.IsPresent()
			}).Await(t)
			if !pf2 {
				t.Errorf("Profile was not successfully deleted from state, last val: %v", val2)
			}
		}
	})
}
