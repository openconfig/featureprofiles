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

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_interface_ingress.json")
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

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_interface_ingress.json")
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

	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_interface_ingress.json")
	defer teardownQos(t, dut, baseConfig)

	baseConfigClassifier := setup.GetAnyValue(baseConfig.Classifier)

	termIds := func() []string {
		var keys []string
		for k, _ := range baseConfigClassifier.Term {
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

func TestFwdingrp1(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")
	// set traffic
	var baseConfig *oc.Qos = setupQos(t, dut, "base_config_fwdingrp1.json")
	defer teardownQos(t, dut, baseConfig)
	// verify traffic
}
func TestDeleteOneQueue(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfig *oc.Qos = setupQosEgress(t, dut, "base_config_interface_egress.json")
	defer teardownQos(t, dut, baseConfig)
	// test traffic
	// verify transmit-pkts on default queue = 0
	baseConfigQueue := baseConfig.Queue["tc1"]

	t.Run("Delete the Queue with lowest priority", func(t *testing.T) {
		config := dut.Config().Qos().Queue(*baseConfigQueue.Name)
		if qs := config.Lookup(t); qs != nil {
			t.Errorf("Delete fail: got %v", qs)
		}
	})
	// test traffic
	// verify transmit-pkts on default queue > 0
}
func TestDeleteClassifierScheduler(t *testing.T) {
	t.Skip()
	dut := ondatra.DUT(t, "dut")

	var baseConfigIngress *oc.Qos = setupQosIngress(t, dut, "base_config_interface_ingress.json")
	var baseConfigEgress *oc.Qos = setupQosEgress(t, dut, "base_config_interface_egress.json")
	defer dut.Config().Qos().Delete(t)

	baseConfigClassifier := setup.GetAnyValue(baseConfigIngress.Classifier)
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfigEgress.SchedulerPolicy)

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
	// Verify that traffic through the interface is not disturbed
	// testTraffic func might work
}

func TestDeleteSharedQueues(t *testing.T) {
	t.Skip()
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

	t.Run("Deleting Queues", func(t *testing.T) {
		for qName, _ := range baseConfig.Queue {
			config := dut.Config().Qos().Queue(qName)
			config.Delete(t)
			if qs := config.Get(t); qs != nil {
				t.Errorf("Delete Queue %s fail: got %v", qName, qs)
			}
		}
	})
}
func TestDetachSchedulerPolicy(t *testing.T) {
	t.Skip()
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
