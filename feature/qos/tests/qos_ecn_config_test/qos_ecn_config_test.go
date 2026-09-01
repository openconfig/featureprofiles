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
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testcase struct {
	name string
	fn   func(t *testing.T, q *oc.Qos)
}

var qosEcnConfigTestcases = []testcase{
	{
		name: "DP-1.3.1_80KB_Equal_Threshold",
		fn:   testDP131EqualThreshold,
	},
	{
		name: "DP-1.3.2_MB_Threshold_Not_Equal",
		fn:   testDP132MBThreshold,
	},
	{
		name: "DP-1.3.3_Percent_Threshold",
		fn:   testDP133PercentThreshold,
	},
	{
		name: "DP-1.3.4_Negative_Test_Cases",
		fn:   testDP134NegativeTestCases,
	},
	{
		name: "DP-1.3.5_Teardown_And_Cleanup",
		fn:   testDP135TeardownAndCleanup,
	},
}

func setupEnvironment(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	// Configure basic interface IDs.
	d := &oc.Root{}
	i1 := d.GetOrCreateInterface(p1.Name())
	i1.SetEnabled(true)
	i1.SetType(oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
	i2 := d.GetOrCreateInterface(p2.Name())
	i2.SetEnabled(true)
	i2.SetType(oc.IETFInterfaces_InterfaceType_ethernetCsmacd)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), i1)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), i2)

	// Create an input IPv4 classifier to match traffic intended for the QoS queue being tested.
	q := &oc.Qos{}
	classifierName := "ECN_CLASSIFIER_IPV4"
	classifiers := []cfgplugins.QosClassifier{{
		Desc:      "IPv4 DSCP ECN classifier",
		Name:      classifierName,
		ClassType: oc.Qos_Classifier_Type_IPV4,
		TermID:    "term-1",
		DscpSet:   []uint8{0, 1, 2, 3, 4, 5, 6, 7},
	}}
	cfgplugins.NewQoSClassifierConfiguration(t, dut, q, classifiers)
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

	// Apply the classifier to the input of DUT port-1.
	qoscfg.SetInputClassifier(t, dut, q, p1.Name(), oc.Input_Classifier_Type_IPV4, classifierName)
}

func targetQueueName(t *testing.T, dut *ondatra.DUTDevice) string {
	if deviations.QOSQueueRequiresID(dut) {
		return netutil.CommonTrafficQueues(t, dut).BE0
	}
	return "0"
}

func TestQosEcnConfigTests(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	setupEnvironment(t, dut)

	t.Cleanup(func() {
		gnmi.Delete(t, dut, gnmi.OC().Qos().Config())
	})

	d := &oc.Root{}
	q := d.GetOrCreateQos()
	for _, tt := range qosEcnConfigTestcases {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn(t, q)
		})
	}
}

// testDP131EqualThreshold implements DP-1.3.1 - 80KB min-threshold equal max-threshold
func testDP131EqualThreshold(t *testing.T, q *oc.Qos) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	queueName := targetQueueName(t, dut)

	profile := cfgplugins.QoSQueueManagementProfile{
		Desc:                      "DP-1.3.1 80KB min-threshold equal max-threshold",
		Name:                      "ECN_PROFILE_1",
		MinThreshold:              81920,
		MaxThreshold:              81920,
		EnableEcn:                 true,
		Drop:                      false,
		MaxDropProbabilityPercent: 100,
	}

	// Step 1: Generate DUT configuration with queue-management-profile and attach to target queue on output of DUT port-1/2.
	t.Log("Step 1 - Generate DUT configuration: Configure queue-management-profile ECN_PROFILE_1")
	cfgplugins.NewQoSQueueManagementProfile(t, dut, q, []cfgplugins.QoSQueueManagementProfile{profile})

	// Step 2: Push configuration to DUT using gNMI Set with REPLACE option.
	t.Log("Step 2 - Push configuration to DUT using gNMI Set with REPLACE option")
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

	// Step 3: Validate that profile is created and values are set as expected.
	t.Log("Step 3 - Validate queue-management-profile ECN_PROFILE_1 state values")
	cfgplugins.ValidateQueueManagementProfile(t, dut, profile)

	// Step 4: Validate ECN profile application on output interface queue.
	t.Log("Step 4 - Validate ECN profile ECN_PROFILE_1 application on output queue")
	qoscfg.SetOutputQueueManagementProfile(t, dut, q, dp.Name(), queueName, profile.Name)
	outQueue := gnmi.OC().Qos().Interface(dp.Name()).Output().Queue(queueName)
	if deviations.QosGetStatePathUnsupported(dut) {
		if got, want := gnmi.Get(t, dut, outQueue.QueueManagementProfile().Config()), profile.Name; got != want {
			t.Errorf("outQueue.QueueManagementProfile().Config(): got %v, want %v", got, want)
		}
	} else {
		if got, want := gnmi.Get(t, dut, outQueue.QueueManagementProfile().State()), profile.Name; got != want {
			t.Errorf("outQueue.QueueManagementProfile().State(): got %v, want %v", got, want)
		}
	}

	// Step 5: Trigger a supervisor switchover using gNOI SwitchControlProcessor.
	t.Log("Step 5 - Trigger supervisor switchover using gNOI SwitchControlProcessor")
	gnoiClient := dut.RawAPIs().GNOI(t)
	switchReq := &spb.SwitchControlProcessorRequest{}
	if _, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchReq); err != nil {
		t.Logf("SwitchControlProcessor response err (can be expected during switchover or unsupported): %v", err)
	}
	// Wait for the device to become reachable and configuration to persist after switchover.
	t.Log("Waiting for device to be reachable and configuration to persist after switchover")
	wredUniform := gnmi.OC().Qos().QueueManagementProfile(profile.Name).Wred().Uniform()
	if !deviations.QosGetStatePathUnsupported(dut) {
		gnmi.Await(t, dut, wredUniform.EnableEcn().State(), 10*time.Minute, profile.EnableEcn)
	} else {
		gnmi.Await(t, dut, wredUniform.EnableEcn().Config(), 10*time.Minute, profile.EnableEcn)
	}

	// Step 6: Once new supervisor is active, repeat gNMI Get checks to verify configuration persisted.
	t.Log("Step 6 - Verify configuration persisted after supervisor switchover")
	cfgplugins.ValidateQueueManagementProfile(t, dut, profile)
}

// testDP132MBThreshold implements DP-1.3.2 - Threshold in MB, min-threshold not-equal max-threshold
func testDP132MBThreshold(t *testing.T, q *oc.Qos) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	queueName := targetQueueName(t, dut)

	profile := cfgplugins.QoSQueueManagementProfile{
		Desc:                      "DP-1.3.2 Threshold in MB, min-threshold not-equal max-threshold",
		Name:                      "ECN_PROFILE_2",
		MinThreshold:              3276800,
		MaxThreshold:              6553600,
		EnableEcn:                 true,
		Drop:                      false,
		MaxDropProbabilityPercent: 100,
	}

	// Step 1: Generate DUT configuration for ECN_PROFILE_2.
	t.Log("Step 1 - Generate DUT configuration: Configure queue-management-profile ECN_PROFILE_2")
	cfgplugins.NewQoSQueueManagementProfile(t, dut, q, []cfgplugins.QoSQueueManagementProfile{profile})

	// Step 2: Push configuration to DUT using gNMI Set with REPLACE option.
	t.Log("Step 2 - Push configuration to DUT using gNMI Set with REPLACE option")
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

	// Step 3: Validation with pass/fail criteria.
	t.Log("Step 3 - Validate queue-management-profile ECN_PROFILE_2 state values")
	cfgplugins.ValidateQueueManagementProfile(t, dut, profile)

	// Step 4: Validate ECN profile application.
	t.Log("Step 4 - Validate ECN profile ECN_PROFILE_2 application on output queue")
	qoscfg.SetOutputQueueManagementProfile(t, dut, q, dp.Name(), queueName, profile.Name)
	outQueue := gnmi.OC().Qos().Interface(dp.Name()).Output().Queue(queueName)
	if deviations.QosGetStatePathUnsupported(dut) {
		if got, want := gnmi.Get(t, dut, outQueue.QueueManagementProfile().Config()), profile.Name; got != want {
			t.Errorf("outQueue.QueueManagementProfile().Config(): got %v, want %v", got, want)
		}
	} else {
		if got, want := gnmi.Get(t, dut, outQueue.QueueManagementProfile().State()), profile.Name; got != want {
			t.Errorf("outQueue.QueueManagementProfile().State(): got %v, want %v", got, want)
		}
	}
}

// testDP133PercentThreshold implements DP-1.3.3 - Threshold in percentage, min-threshold not-equal max-threshold
func testDP133PercentThreshold(t *testing.T, q *oc.Qos) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	queueName := targetQueueName(t, dut)

	profile := cfgplugins.QoSQueueManagementProfile{
		Desc:                      "DP-1.3.3 Threshold in percentage, min-threshold not-equal max-threshold",
		Name:                      "ECN_PROFILE_3",
		MinThresholdPercent:       1,
		MaxThresholdPercent:       2,
		EnableEcn:                 true,
		Drop:                      false,
		MaxDropProbabilityPercent: 100,
	}

	// Step 1: Generate DUT configuration for ECN_PROFILE_3.
	t.Log("Step 1 - Generate DUT configuration: Configure queue-management-profile ECN_PROFILE_3")
	cfgplugins.NewQoSQueueManagementProfile(t, dut, q, []cfgplugins.QoSQueueManagementProfile{profile})

	// Step 2: Push configuration to DUT using gNMI Set with REPLACE option.
	t.Log("Step 2 - Push configuration to DUT using gNMI Set with REPLACE option")
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), q)

	// Step 3: Validation with pass/fail criteria.
	t.Log("Step 3 - Validate queue-management-profile ECN_PROFILE_3 state values")
	cfgplugins.ValidateQueueManagementProfile(t, dut, profile)

	// Step 4: Validate ECN profile application.
	t.Log("Step 4 - Validate ECN profile ECN_PROFILE_3 application on output queue")
	qoscfg.SetOutputQueueManagementProfile(t, dut, q, dp.Name(), queueName, profile.Name)
	outQueue := gnmi.OC().Qos().Interface(dp.Name()).Output().Queue(queueName)
	if deviations.QosGetStatePathUnsupported(dut) {
		if got, want := gnmi.Get(t, dut, outQueue.QueueManagementProfile().Config()), profile.Name; got != want {
			t.Errorf("outQueue.QueueManagementProfile().Config(): got %v, want %v", got, want)
		}
	} else {
		if got, want := gnmi.Get(t, dut, outQueue.QueueManagementProfile().State()), profile.Name; got != want {
			t.Errorf("outQueue.QueueManagementProfile().State(): got %v, want %v", got, want)
		}
	}
}

// testDP134NegativeTestCases implements DP-1.3.4 - Negative Test Cases
func testDP134NegativeTestCases(t *testing.T, q *oc.Qos) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	queueName := targetQueueName(t, dut)

	// Negative Test 1 (min-threshold > max-threshold)
	t.Log("Negative Test 1 - Attempt configuring min-threshold strictly greater than max-threshold")
	invalidQos1 := &oc.Qos{}
	profile1 := invalidQos1.GetOrCreateQueueManagementProfile("NEG_PROFILE_1")
	profile1.SetName("NEG_PROFILE_1")
	wred1 := profile1.GetOrCreateWred().GetOrCreateUniform()
	wred1.SetMinThreshold(81920)
	wred1.SetMaxThreshold(40960)
	if got := testt.ExpectFatal(t, func(t testing.TB) {
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), invalidQos1)
	}); got == "" {
		t.Errorf("Negative Test 1: expected fatal error when min-threshold > max-threshold, got nil")
	}

	// Negative Test 2 (Invalid max-drop-probability-percent)
	t.Log("Negative Test 2 - Attempt configuring out-of-range max-drop-probability-percent (101)")
	invalidQos2 := &oc.Qos{}
	profile2 := invalidQos2.GetOrCreateQueueManagementProfile("NEG_PROFILE_2")
	profile2.SetName("NEG_PROFILE_2")
	wred2 := profile2.GetOrCreateWred().GetOrCreateUniform()
	wred2.SetMaxDropProbabilityPercent(101)
	if got := testt.ExpectFatal(t, func(t testing.TB) {
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), invalidQos2)
	}); got == "" {
		t.Errorf("Negative Test 2: expected fatal error when max-drop-probability-percent > 100, got nil")
	}

	// Negative Test 3 (Non-existent Profile Assignment)
	t.Log("Negative Test 3 - Attempt assigning non-existent queue-management-profile BOGUS_PROFILE")
	invalidQos3 := &oc.Qos{}
	intf3 := invalidQos3.GetOrCreateInterface(dp.Name())
	intf3.SetInterfaceId(dp.Name())
	intf3.GetOrCreateInterfaceRef().Interface = ygot.String(dp.Name())
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf3.InterfaceRef = nil
	}
	queue3 := intf3.GetOrCreateOutput().GetOrCreateQueue(queueName)
	queue3.SetName(queueName)
	queue3.SetQueueManagementProfile("BOGUS_PROFILE")
	if got := testt.ExpectFatal(t, func(t testing.TB) {
		gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), invalidQos3)
	}); got == "" {
		t.Errorf("Negative Test 3: expected fatal error when assigning non-existent profile BOGUS_PROFILE, got nil")
	}

	// Negative Test 4 (Invalid Profile Deletion while actively applied)
	t.Log("Negative Test 4 - Attempt deleting queue-management-profile while actively applied")
	if got := testt.ExpectFatal(t, func(t testing.TB) {
		gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile("ECN_PROFILE_3").Config())
	}); got == "" {
		t.Errorf("Negative Test 4: expected fatal error when deleting active profile ECN_PROFILE_3, got nil")
	}
}

// testDP135TeardownAndCleanup implements DP-1.3.5 - Teardown and Cleanup Verification
func testDP135TeardownAndCleanup(t *testing.T, q *oc.Qos) {
	dut := ondatra.DUT(t, "dut")
	dp := dut.Port(t, "port2")
	queueName := targetQueueName(t, dut)

	// Step 1: Detach the queue-management-profile from output queue.
	t.Log("Step 1 - Detach queue-management-profile from output queue")
	gnmi.Delete(t, dut, gnmi.OC().Qos().Interface(dp.Name()).Output().Queue(queueName).QueueManagementProfile().Config())

	// Step 2: Validate profile is detached.
	t.Log("Step 2 - Validate profile is detached from output queue")
	outQueue := gnmi.OC().Qos().Interface(dp.Name()).Output().Queue(queueName)
	if !deviations.QosGetStatePathUnsupported(dut) {
		if got := gnmi.Lookup(t, dut, outQueue.QueueManagementProfile().State()); got.IsPresent() {
			val, _ := got.Val()
			if val != "" {
				t.Errorf("Expected empty queue-management-profile after detach, got %v", val)
			}
		}
	}

	// Step 3: Delete queue-management-profile ECN_PROFILE_1 entirely.
	t.Log("Step 3 - Delete queue-management-profile ECN_PROFILE_1")
	gnmi.Delete(t, dut, gnmi.OC().Qos().QueueManagementProfile("ECN_PROFILE_1").Config())

	// Step 4: Validate profile is removed.
	t.Log("Step 4 - Validate profile ECN_PROFILE_1 is removed")
	if !deviations.QosGetStatePathUnsupported(dut) {
		if got := gnmi.Lookup(t, dut, gnmi.OC().Qos().QueueManagementProfile("ECN_PROFILE_1").State()); got.IsPresent() {
			t.Errorf("Expected ECN_PROFILE_1 state to be removed, but lookup reported present")
		}
	}
}
