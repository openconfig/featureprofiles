package qos_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/testt"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

func TestInterfaceIdAtContainer(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQosIngress(t, dut, "base_config_interface_ingress.json")
	defer teardownQos(t, dut, baseConfig)

	for _, input := range testInterfaceIdInput {
		t.Run(fmt.Sprintf("Testing /qos/interfaces/interface/config/interface-id using value %v", input), func(t *testing.T) {
			baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
			*baseConfigInterface.InterfaceId = input

			config := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId)

			t.Run("Replace container", func(t *testing.T) {
				config.Update(t, baseConfigInterface)
				// config.Replace(t, baseConfigInterface)
			})
			t.Run("Get container", func(t *testing.T) {
				configGot := config.Get(t)
				if diff := cmp.Diff(*configGot, *baseConfigInterface); diff != "" {
					t.Errorf("Config /qos/interfaces/interface/config/interface-id: got %v", diff)
				}
			})
		})
	}
}

func TestDeleteClassifier(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQosIngress(t, dut, "base_config_interface_ingress.json")
	defer teardownQos(t, dut, baseConfig)

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	t.Run("Delete a Classifier Policy attached to an interface", func(t *testing.T) {
		if got := testt.ExpectFatal(t, func(t testing.TB) {
			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name)
			config.Delete(t)
		}); got == "" {
			t.Errorf("Delete did not fail fatally as expected")
		}
	})
}

func TestDeleteLastClassMap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQosIngress(t, dut, "base_config_interface_ingress.json")
	defer teardownQos(t, dut, baseConfig)

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	termIds := func() []string {
		var keys []string
		for k := range baseConfigClassifier.Term {
			keys = append(keys, k)
		}
		return keys
	}()
	for _, termId := range termIds[1:] {
		config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(termId)
		config.Delete(t)
	}
	t.Run("Delete the last class-map from pmap attached to interface", func(t *testing.T) {
		if got := testt.ExpectFatal(t, func(t testing.TB) {
			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name).Term(termIds[0])
			config.Delete(t)
		}); got == "" {
			t.Errorf("Delete did not fail fatally as expected")
		}
	})
}

func TestDeleteOneQueue(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQosEgress(t, dut, "base_config_interface_egress.json")
	defer teardownQos(t, dut, baseConfig)

	queuNameInput := "tc1" // low priority queue
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
	baseConfigSchedulerPolicyScheduler := setup.GetAnyValue(baseConfigSchedulerPolicy.Scheduler)
	baseConfigSchedulerPolicySchedulerInput := baseConfigSchedulerPolicyScheduler.Input[queuNameInput]
	config := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Scheduler(*baseConfigSchedulerPolicyScheduler.Sequence).Input(*baseConfigSchedulerPolicySchedulerInput.Id)

	t.Run(fmt.Sprintf("Delete Queue %s", queuNameInput), func(t *testing.T) {
		config.Delete(t)
		// Lookup is not working after Delete - guess Nishant opened a bug for this
		// if configGot := config.Lookup(t); configGot != nil {
		// 	t.Errorf("Delete fail: got %+v", configGot)
		// }
	})
	t.Run(fmt.Sprintf("Add back Queue %s", queuNameInput), func(t *testing.T) {
		config.Update(t, baseConfigSchedulerPolicySchedulerInput)
		configGot := config.Get(t)
		if diff := cmp.Diff(configGot, baseConfigSchedulerPolicySchedulerInput); diff != "" {
			t.Errorf("Get Config BaseConfig SchedulerPolicy Scheduler Input: %+v", diff)
		}
	})

	// pull stats and verify
	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	t.Run(fmt.Sprintf("Get Interface Output Queue Telemetry %s %s", *baseConfigInterface.InterfaceId, queuNameInput), func(t *testing.T) {
		got := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Queue(queuNameInput).Get(t)
		t.Run("Verify Transmit-Octets", func(t *testing.T) {
			if !(*got.TransmitOctets == 0) {
				t.Errorf("Get Interface Output Queue Telemetry fail: got %+v", *got)
			}
		})
		t.Run("Verify Transmit-Packets", func(t *testing.T) {
			if !(*got.TransmitPkts == 0) {
				t.Errorf("Get Interface Output Queue Telemetry fail: got %+v", *got)
			}
		})
		t.Run("Verify Dropped-Packets", func(t *testing.T) {
			if !(*got.DroppedPkts == 0) {
				t.Errorf("Get Interface Output Queue Telemetry fail: got %+v", *got)
			}
		})
	})
}

func TestDeleteClassifierScheduler(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQosFull(t, dut, "base_config_interface_full.json")
	defer teardownQos(t, dut, baseConfig)

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)

	t.Run("Delete a Classifier Policy attached to an interface", func(t *testing.T) {
		if got := testt.ExpectFatal(t, func(t testing.TB) {
			config := dut.Config().Qos().Classifier(*baseConfigClassifier.Name)
			config.Delete(t)
		}); got == "" {
			t.Errorf("Delete did not fail fatally as expected")
		}
	})
	t.Run("Delete a Scheduler Policy attached to an interface", func(t *testing.T) {
		if got := testt.ExpectFatal(t, func(t testing.TB) {
			config := dut.Config().Qos().Classifier(*baseConfigSchedulerPolicy.Name)
			config.Delete(t)
		}); got == "" {
			t.Errorf("Delete did not fail fatally as expected")
		}
	})
}

func TestDeleteSharedQueues(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQosEgress(t, dut, "base_config_interface_egress.json")
	defer teardownQos(t, dut, baseConfig)

	var InterfaceSchedulerPolicyInfo = []struct {
		interfaceId         string
		schedulerPolicyName string
	}{
		{
			"FourHundredGigE0/0/0/1",
			"eg_policy1111",
		},
		{
			"FourHundredGigE0/0/0/2",
			"eg_policy2222",
		},
	}
	for _, intfsch := range InterfaceSchedulerPolicyInfo {
		var baseConfigSchedulerPolicy = new(oc.Qos_SchedulerPolicy)
		baseConfigSchedulerPolicy = setup.GetAnyValue(baseConfig.SchedulerPolicy)
		*baseConfigSchedulerPolicy.Name = intfsch.schedulerPolicyName
		baseConfig.SchedulerPolicy[*baseConfigSchedulerPolicy.Name] = baseConfigSchedulerPolicy
		dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Update(t, baseConfigSchedulerPolicy)
		var baseConfigInterface = new(oc.Qos_Interface)
		baseConfigInterface = setup.GetAnyValue(baseConfig.Interface)
		*baseConfigInterface.InterfaceId = intfsch.interfaceId
		baseConfig.Interface[*baseConfigInterface.InterfaceId] = baseConfigInterface
		dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Update(t, baseConfigInterface)
	}

	testqueuNameInput := []string{"tc1", "tc2", "tc3"}
	t.Run(fmt.Sprintf("Deleting Shared Queues %v", testqueuNameInput), func(t *testing.T) {
		batchSet := config.NewBatchSetRequest()
		ctx := context.Background()
		for _, qName := range testqueuNameInput {
			for _, intfsch := range InterfaceSchedulerPolicyInfo {
				tmpSchedulerPolicy := baseConfig.SchedulerPolicy[intfsch.schedulerPolicyName]
				tmpSchedulerPolicyScheduler := setup.GetAnyValue(tmpSchedulerPolicy.Scheduler)
				tmpSchedulerPolicySchedulerInput := tmpSchedulerPolicyScheduler.Input[qName]
				queuePath := dut.Config().Qos().SchedulerPolicy(intfsch.schedulerPolicyName).Scheduler(*tmpSchedulerPolicyScheduler.Sequence).Input(*tmpSchedulerPolicySchedulerInput.Id)
				batchSet.Append(ctx, t, queuePath, nil, config.DeleteOC)
			}
		}
		batchSet.Send(ctx, t, dut)
	})
}

func TestDetachSchedulerPolicy(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQosEgress(t, dut, "base_config_interface_egress.json")
	defer teardownQos(t, dut, baseConfig)

	var InterfaceSchedulerPolicyInfo = []struct {
		interfaceId         string
		schedulerPolicyName string
	}{
		{
			"FourHundredGigE0/0/0/1",
			"eg_policy1111",
		},
		{
			"FourHundredGigE0/0/0/2",
			"eg_policy2222",
		},
	}
	for _, intfsch := range InterfaceSchedulerPolicyInfo {
		baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
		*baseConfigSchedulerPolicy.Name = intfsch.schedulerPolicyName
		dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name).Update(t, baseConfigSchedulerPolicy)
		baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
		*baseConfigInterface.InterfaceId = intfsch.interfaceId
		dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId).Update(t, baseConfigInterface)
	}

	t.Run("Detaching Scheduler Policies", func(t *testing.T) {
		batchSet := config.NewBatchSetRequest()
		ctx := context.Background()
		for _, intfsch := range InterfaceSchedulerPolicyInfo {
			intfschPath := dut.Config().Qos().Interface(intfsch.interfaceId).Output().SchedulerPolicy()
			batchSet.Append(ctx, t, intfschPath, nil, config.DeleteOC)
		}
		batchSet.Send(ctx, t, dut)
	})
}

func TestInterfaceOutputTelemetryAfterClearCounters(t *testing.T) {
	t.Skip()
}
