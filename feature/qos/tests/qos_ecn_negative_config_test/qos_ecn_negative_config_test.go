// Copyright 2026 Google LLC
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

package qos_ecn_negative_config_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
)

const (
	validProfileName    = "ECN_VALID_PROFILE"
	negativeProfileName = "ECN_PROFILE_NEG"
	bogusProfileName    = "BOGUS_PROFILE"

	classifierIPv4Name = "dscp_based_classifier_ipv4"
	classifierIPv6Name = "dscp_based_classifier_ipv6"

	targetGroupBE1 = "target-group-BE1"
	targetGroupAF1 = "target-group-AF1"

	baselineMinThreshold              = uint64(81920)
	baselineMaxThreshold              = uint64(163840)
	invalidLowerMaxThreshold          = uint64(40960)
	baselineWeight                    = uint32(0)
	baselineMaxDropProbabilityPercent = uint8(100)
	invalidMaxDropProbabilityPercent  = uint8(101)
)

var (
	// DP-1.20 Topology: DUT Port 1 (Ethernet1) IPv4 and IPv6 addresses.
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:db8::1",
		IPv6Len: 126,
	}

	// DP-1.20 Topology: DUT Port 2 (Ethernet2) IPv4 and IPv6 addresses.
	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
		IPv6:    "2001:db8::5",
		IPv6Len: 126,
	}

	// ecnProfiles lists every queue management profile this test may create.
	ecnProfiles = []string{validProfileName, negativeProfileName}

	// setupClassifiers lists the dual-stack input classifiers created in setup.
	setupClassifiers = []string{classifierIPv4Name, classifierIPv6Name}

	// setupForwardingGroups lists the forwarding groups created in setup.
	setupForwardingGroups = []string{targetGroupBE1, targetGroupAF1}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// baselineECNProfileConfig returns the Baseline Profile Defaults defined in the
// DP-1.20 Topology table for the given profile name.
func baselineECNProfileConfig(name string) qoscfg.ECNProfileConfig {
	return qoscfg.ECNProfileConfig{
		Name:                      name,
		MinThreshold:              baselineMinThreshold,
		MaxThreshold:              baselineMaxThreshold,
		EnableECN:                 true,
		Drop:                      false,
		Weight:                    baselineWeight,
		MaxDropProbabilityPercent: baselineMaxDropProbabilityPercent,
	}
}

// setupEnvironment implements the DP-1.20 Test environment setup procedure.
func setupEnvironment(t *testing.T, dut *ondatra.DUTDevice, dp1, dp2 *ondatra.Port, queueBE1, queueAF1 string) {
	t.Helper()

	// DP-1.20 Test environment setup - Step 1: Interface Configuration.
	// Connect DUT port-1 (Ethernet1) to ATE port 1 and DUT port-2 (Ethernet2)
	// to ATE port 2, and configure the IPv4 and IPv6 addresses defined in the
	// Topology table.
	t.Log("DP-1.20 Test environment setup - Step 1: Configuring IPv4 and IPv6 addresses on DUT port-1 and port-2")
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp1.Name()).Config(), dutPort1.NewOCInterface(dp1.Name(), dut))
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp2.Name()).Config(), dutPort2.NewOCInterface(dp2.Name(), dut))
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp1)
		fptest.SetPortSpeed(t, dp2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, dp2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	// DP-1.20 Test environment setup - Step 2: Queues and Forwarding Groups.
	// Configure queues 0 (BE1) and 1 (AF1) under /qos/queues/queue/config/name
	// and map forwarding groups target-group-BE1 (to queue 0) and
	// target-group-AF1 (to queue 1) under
	// /qos/forwarding-groups/forwarding-group/config/output-queue.
	t.Log("DP-1.20 Test environment setup - Step 2: Configuring QoS queues and forwarding groups")
	qos := &oc.Root{}
	q := qos.GetOrCreateQos()
	cfgplugins.CreateQueues(t, dut, q, []string{queueBE1, queueAF1})
	q.GetOrCreateForwardingGroup(targetGroupBE1).SetOutputQueue(queueBE1)
	q.GetOrCreateForwardingGroup(targetGroupAF1).SetOutputQueue(queueAF1)

	// DP-1.20 Test environment setup - Step 3: Dual-Stack Input Classifiers.
	// Configure dscp_based_classifier_ipv4 (IPV4) and dscp_based_classifier_ipv6
	// (IPV6) matching DSCP [0, 1, 2, 3] -> target-group-BE1 and
	// DSCP [8, 9, 10, 11] -> target-group-AF1.
	t.Log("DP-1.20 Test environment setup - Step 3: Configuring dual-stack IPv4 and IPv6 input classifiers")
	classifiers := []cfgplugins.QosClassifier{
		{
			Desc:        "IPv4 classifier term 0 for target-group-BE1",
			Name:        classifierIPv4Name,
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "0",
			TargetGroup: targetGroupBE1,
			DscpSet:     []uint8{0, 1, 2, 3},
		},
		{
			Desc:        "IPv4 classifier term 1 for target-group-AF1",
			Name:        classifierIPv4Name,
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      "1",
			TargetGroup: targetGroupAF1,
			DscpSet:     []uint8{8, 9, 10, 11},
		},
		{
			Desc:        "IPv6 classifier term 0 for target-group-BE1",
			Name:        classifierIPv6Name,
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "0",
			TargetGroup: targetGroupBE1,
			DscpSet:     []uint8{0, 1, 2, 3},
		},
		{
			Desc:        "IPv6 classifier term 1 for target-group-AF1",
			Name:        classifierIPv6Name,
			ClassType:   oc.Qos_Classifier_Type_IPV6,
			TermID:      "1",
			TargetGroup: targetGroupAF1,
			DscpSet:     []uint8{8, 9, 10, 11},
		},
	}
	cfgplugins.NewQoSClassifierConfiguration(t, dut, q, classifiers)
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

	// DP-1.20 Test environment setup - Step 3: Attach both classifiers to the
	// input of Ethernet1 (dp1) and Ethernet2 (dp2) using a separate QoS struct
	// so classifier definitions and interface bindings are not pushed together.
	qIntf := &oc.Qos{}
	for _, dp := range []*ondatra.Port{dp1, dp2} {
		qoscfg.BuildInputClassifierAttachment(dut, qIntf, dp.Name(), oc.Input_Classifier_Type_IPV4, classifierIPv4Name)
		qoscfg.BuildInputClassifierAttachment(dut, qIntf, dp.Name(), oc.Input_Classifier_Type_IPV6, classifierIPv6Name)
	}
	gnmi.Update(t, dut, gnmi.OC().Qos().Config(), qIntf)

	// DP-1.20 Test environment setup - Step 3: Verify classifier attachment
	// using gNMI Subscribe (Watch) on
	// /qos/interfaces/interface/input/classifiers/classifier/state/name and
	// /qos/interfaces/interface/input/classifiers/classifier/state/type.
	for _, dp := range []*ondatra.Port{dp1, dp2} {
		qoscfg.VerifyInputClassifier(t, dut, dp.Name(), oc.Input_Classifier_Type_IPV4, classifierIPv4Name, time.Minute)
		qoscfg.VerifyInputClassifier(t, dut, dp.Name(), oc.Input_Classifier_Type_IPV6, classifierIPv6Name, time.Minute)
	}
}

// removeECNAndSetupConfig detaches any active ECN profile, deletes all ECN
// profiles, and reverts the setup QoS and interface configurations. Used by
// t.Cleanup if DP-1.20.5 did not complete.
func removeECNAndSetupConfig(t *testing.T, dut *ondatra.DUTDevice, dp1, dp2 *ondatra.Port, queueBE1, queueAF1 string) {
	t.Helper()
	bestEffort := func(desc string, fn func(t testing.TB)) {
		if msg := testt.CaptureFatal(t, fn); msg != nil {
			t.Logf("Cleanup: %s failed (ignored): %s", desc, *msg)
		}
	}
	bestEffort("detach queue management profile", func(t testing.TB) {
		qoscfg.DetachOutputQueueManagementProfile(t, dut, dp1.Name(), queueBE1)
	})
	for _, p := range ecnProfiles {
		bestEffort("delete queue management profile "+p, func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile(p).Config())
		})
	}
	qoscfg.CleanupQosSetup(t, dut, []*ondatra.Port{dp1, dp2}, setupClassifiers, setupForwardingGroups, []string{queueBE1, queueAF1})
}

func TestQosEcnNegativeConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	queueBE1 := "0"
	queueAF1 := "1"
	if !deviations.QOSQueueRequiresID(dut) {
		queues := netutil.CommonTrafficQueues(t, dut)
		queueBE1 = queues.BE1
		queueAF1 = queues.AF1
	}

	// teardownVerified is set when DP-1.20.5 removes and verifies all config.
	teardownVerified := false
	// setupOK is set when DP-1.20 Test environment setup completes successfully.
	setupOK := false
	// Safety net for DP-1.20.5: registered before any configuration is pushed so
	// the DUT is restored to baseline even if a subtest fails or panics early.
	t.Cleanup(func() {
		if teardownVerified {
			return
		}
		if !setupOK {
			// Setup failures are typically caused by an unreachable gNMI server,
			// where further gNMI calls can block until the test times out.
			t.Log("DP-1.20 Test environment setup did not complete; skipping DUT cleanup")
			return
		}
		t.Log("DP-1.20.5 did not complete; removing ECN and setup configuration (best effort)")
		removeECNAndSetupConfig(t, dut, dp1, dp2, queueBE1, queueAF1)
	})

	// DP-1.20 Test environment setup
	// The negative tests (DP-1.20.1 - DP-1.20.4) pass when gNMI Set fails, so
	// they must not run against a DUT that was never configured or cannot be
	// reached; otherwise a transport error would be counted as a rejection.
	setupOK = t.Run("Test environment setup", func(t *testing.T) {
		setupEnvironment(t, dut, dp1, dp2, queueBE1, queueAF1)
	})
	if !setupOK {
		t.Fatal("DP-1.20 Test environment setup failed; skipping DP-1.20.1 - DP-1.20.5")
	}

	// DP-1.20.1 - Negative Test 1: min-threshold Strictly Greater Than max-threshold
	t.Run("DP-1.20.1 - Negative Test 1: min-threshold Strictly Greater Than max-threshold", func(t *testing.T) {
		if deviations.EcnMinGreaterMaxThresholdUnsupported(dut) {
			t.Skip("Skipping DP-1.20.1: device does not support validating min-threshold > max-threshold")
		}

		// DP-1.20.1 Step 1 - Construct ECN_PROFILE_NEG using the Baseline Profile
		// Defaults and override the threshold leaves so min-threshold (81920) >
		// max-threshold (40960), including the Ethernet1 output queue 0 binding in
		// the same gNMI Set request.
		t.Log("DP-1.20.1 Step 1: Constructing ECN_PROFILE_NEG with min-threshold (81920) > max-threshold (40960) and output queue binding")
		q := &oc.Qos{}
		cfg := baselineECNProfileConfig(negativeProfileName)
		cfg.MinThreshold = baselineMinThreshold
		cfg.MaxThreshold = invalidLowerMaxThreshold
		qoscfg.BuildECNQueueManagementProfile(dut, q, cfg)
		qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueBE1, negativeProfileName)

		// DP-1.20.1 Step 2 - Push the configuration to the DUT using gNMI Set
		// (Update / Replace).
		// DP-1.20.1 Step 3 - Validation with pass/fail criteria:
		// Pass: The gNMI Set RPC is rejected with a non-nil error.
		// Fail: The gNMI Set RPC succeeds without error.
		t.Log("DP-1.20.1 Steps 2-3: Pushing invalid min-threshold > max-threshold config via gNMI Set and verifying rejection")
		errStr := testt.ExpectFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)
		})
		if errStr == "" {
			t.Errorf("DP-1.20.1 Step 3 failed: expected gNMI Set error when min-threshold (%d) > max-threshold (%d), got nil", cfg.MinThreshold, cfg.MaxThreshold)
		}
	})

	// DP-1.20.2 - Negative Test 2: Invalid max-drop-probability-percent Out-of-Range
	t.Run("DP-1.20.2 - Negative Test 2: Invalid max-drop-probability-percent Out-of-Range", func(t *testing.T) {
		// DP-1.20.2 Step 1 - Construct ECN_PROFILE_NEG using the Baseline Profile
		// Defaults and override max-drop-probability-percent to an out-of-range
		// value (101).
		t.Log("DP-1.20.2 Step 1: Constructing ECN_PROFILE_NEG with out-of-range max-drop-probability-percent = 101")
		q := &oc.Qos{}
		cfg := baselineECNProfileConfig(negativeProfileName)
		cfg.MaxDropProbabilityPercent = invalidMaxDropProbabilityPercent
		qoscfg.BuildECNQueueManagementProfile(dut, q, cfg)

		// DP-1.20.2 Step 2 - Push the configuration to the DUT using gNMI Set
		// (Update / Replace).
		// DP-1.20.2 Step 3 - Validation with pass/fail criteria:
		// Pass: The gNMI Set RPC is rejected with a non-nil error.
		// Fail: The gNMI Set RPC succeeds without error.
		t.Log("DP-1.20.2 Steps 2-3: Pushing out-of-range max-drop-probability-percent (101) via gNMI Set and verifying rejection")
		errStr := testt.ExpectFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)
		})
		if errStr == "" {
			t.Errorf("DP-1.20.2 Step 3 failed: expected gNMI Set error when max-drop-probability-percent = %d (> 100), got nil", cfg.MaxDropProbabilityPercent)
		}
	})

	// DP-1.20.3 - Negative Test 3: Non-Existent Queue-Management-Profile Assignment
	t.Run("DP-1.20.3 - Negative Test 3: Non-Existent Queue-Management-Profile Assignment", func(t *testing.T) {
		// DP-1.20.3 Step 1 - Construct an interface output queue configuration on
		// Ethernet1 queue 0 referencing BOGUS_PROFILE (which does not exist under
		// /qos/queue-management-profiles/queue-management-profile).
		t.Log("DP-1.20.3 Step 1: Constructing output queue configuration referencing non-existent profile BOGUS_PROFILE")
		q := &oc.Qos{}
		qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueBE1, bogusProfileName)

		// DP-1.20.3 Step 2 - Push the configuration to the DUT using gNMI Set
		// (Update / Replace).
		// DP-1.20.3 Step 3 - Validation with pass/fail criteria:
		// Pass: The gNMI Set RPC is rejected with a non-nil error.
		// Fail: The gNMI Set RPC succeeds without error.
		t.Log("DP-1.20.3 Steps 2-3: Pushing non-existent profile assignment via gNMI Set and verifying rejection")
		errStr := testt.ExpectFatal(t, func(t testing.TB) {
			gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)
		})
		if errStr == "" {
			t.Errorf("DP-1.20.3 Step 3 failed: expected gNMI Set error when assigning non-existent profile %q, got nil", bogusProfileName)
		}
	})

	// DP-1.20.4 - Negative Test 4: Invalid Profile Deletion While Actively Applied
	t.Run("DP-1.20.4 - Negative Test 4: Invalid Profile Deletion While Actively Applied", func(t *testing.T) {
		// DP-1.20.4 Step 1 - Configure ECN_VALID_PROFILE using the Baseline Profile
		// Defaults and attach it to Ethernet1 output queue 0 via gNMI Set (Replace).
		t.Log("DP-1.20.4 Step 1: Configuring ECN_VALID_PROFILE and attaching to Ethernet1 output queue 0 via gNMI Set (Replace)")
		q := &oc.Qos{}
		validCfg := baselineECNProfileConfig(validProfileName)
		qoscfg.BuildECNQueueManagementProfile(dut, q, validCfg)
		qoscfg.BuildOutputQueueManagementProfile(dut, q, dp1.Name(), queueBE1, validProfileName)

		batch := &gnmi.SetBatch{}
		qoscfg.ReplaceQueueManagementProfile(t, dut, batch, q, dp1.Name(), queueBE1, validProfileName)
		batch.Set(t, dut)

		// DP-1.20.4 Step 2 - Confirm ECN_VALID_PROFILE and its Ethernet1 queue 0
		// binding are active using gNMI Subscribe (Watch) on all profile state
		// leaves and output queue state leaves.
		t.Log("DP-1.20.4 Step 2: Confirming ECN_VALID_PROFILE and Ethernet1 output queue 0 binding are active via gNMI Subscribe (Watch)")
		qoscfg.VerifyECNQueueManagementProfile(t, dut, validCfg, time.Minute)
		outQueuePath := gnmi.OC().Qos().Interface(qoscfg.QosInterfaceID(dut, dp1.Name())).Output().Queue(queueBE1)
		qoscfg.VerifyLeaf(t, dut, outQueuePath.Name().State(), outQueuePath.Name().Config(), queueBE1, time.Minute)
		qoscfg.VerifyLeaf(t, dut, outQueuePath.QueueManagementProfile().State(), outQueuePath.QueueManagementProfile().Config(), validProfileName, time.Minute)

		// DP-1.20.4 Step 3 - While ECN_VALID_PROFILE remains applied to Ethernet1
		// queue 0, attempt to delete
		// /qos/queue-management-profiles/queue-management-profile[name=ECN_VALID_PROFILE]
		// using gNMI Set (Delete).
		t.Log("DP-1.20.4 Step 3: Attempting to delete actively applied ECN_VALID_PROFILE using gNMI Set (Delete)")
		errStr := testt.ExpectFatal(t, func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile(validProfileName).Config())
		})

		// DP-1.20.4 Step 4 - Validation with pass/fail criteria:
		// Pass: The gNMI Set (Delete) RPC is rejected with a non-nil error, and
		// gNMI Subscribe (Watch) on
		// /qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/state/queue-management-profile
		// confirms ECN_VALID_PROFILE remains applied.
		t.Log("DP-1.20.4 Step 4: Verifying deletion was rejected and ECN_VALID_PROFILE remains applied on Ethernet1 queue 0")
		if errStr == "" {
			t.Errorf("DP-1.20.4 Step 4 failed: expected gNMI Delete error when deleting actively applied profile %q, got nil", validProfileName)
		}
		qoscfg.VerifyLeaf(t, dut, outQueuePath.QueueManagementProfile().State(), outQueuePath.QueueManagementProfile().Config(), validProfileName, time.Minute)
	})

	// DP-1.20.5 - Teardown and Cleanup Verification
	t.Run("DP-1.20.5 - Teardown and Cleanup Verification", func(t *testing.T) {
		qosIntfID := qoscfg.QosInterfaceID(dut, dp1.Name())
		outQueuePath := gnmi.OC().Qos().Interface(qosIntfID).Output().Queue(queueBE1)

		// DP-1.20.5 Step 1 - Detach ECN_VALID_PROFILE from Ethernet1 output queue 0
		// using gNMI Set (Delete) on
		// /qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/config/queue-management-profile.
		t.Log("DP-1.20.5 Step 1: Detaching ECN_VALID_PROFILE from Ethernet1 output queue 0")
		qoscfg.DetachOutputQueueManagementProfile(t, dut, dp1.Name(), queueBE1)

		// DP-1.20.5 Step 2 - Verify via gNMI Subscribe (Watch) on
		// /qos/interfaces/interface[interface-id=Ethernet1]/output/queues/queue[name=0]/state/queue-management-profile
		// that ECN_VALID_PROFILE is no longer attached.
		t.Log("DP-1.20.5 Step 2: Verifying ECN_VALID_PROFILE is detached from Ethernet1 output queue 0")
		qoscfg.VerifyLeafDetached(t, dut, outQueuePath.QueueManagementProfile().State(), outQueuePath.QueueManagementProfile().Config(), ecnProfiles, time.Minute)

		// DP-1.20.5 Step 3 - Delete ECN_VALID_PROFILE using gNMI Set (Delete) on
		// /qos/queue-management-profiles/queue-management-profile[name=ECN_VALID_PROFILE].
		t.Log("DP-1.20.5 Step 3: Deleting ECN_VALID_PROFILE globally")
		gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile(validProfileName).Config())

		// DP-1.20.5 Step 4 - Verify via gNMI Subscribe (Watch) on
		// /qos/queue-management-profiles/queue-management-profile[name=ECN_VALID_PROFILE]/state/name
		// that the profile is removed.
		t.Log("DP-1.20.5 Step 4: Verifying ECN_VALID_PROFILE is removed")
		profilePath := gnmi.OC().Qos().QueueManagementProfile(validProfileName)
		qoscfg.VerifyLeafRemoved(t, dut, profilePath.Name().State(), profilePath.Name().Config(), time.Minute)

		// DP-1.20.5 Step 5 - Revert the setup QoS and interface configurations
		// from Test environment setup using gNMI Set (Delete) to restore the DUT
		// completely to its baseline state.
		t.Log("DP-1.20.5 Step 5: Reverting setup QoS classifiers, forwarding groups, queues, and interface configurations")
		qoscfg.CleanupQosSetup(t, dut, []*ondatra.Port{dp1, dp2}, setupClassifiers, setupForwardingGroups, []string{queueBE1, queueAF1})

		// DP-1.20.5 complete: the t.Cleanup safety net is not needed.
		teardownVerified = !t.Failed()
	})
}
