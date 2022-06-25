package qos_test

import (
	
	"testing"
    
	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

// TestQueueSchedule will verifies that the Queue + Scheduler-config paths can be read , updated and deleted.

func TestQueueSchedule(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setup.BaseConfig()
	defer teardownQos(t, dut, baseConfig)
	
	baseConfigQueue := setup.GetAnyValue(baseConfig.Queue)
	config := dut.Config().Qos().Queue(*baseConfigQueue.Name)
	t.Run("Create Queue ", func(t *testing.T) {
		config.Update(t, baseConfigQueue)
	})
	if !setup.SkipGet() {
		t.Run("Get queue Config", func(t *testing.T) {
			configGot := config.Get(t)
			if diff := cmp.Diff(*configGot, *baseConfigQueue); diff != "" {
				t.Errorf("Config queue fail: \n%v", diff)
			}
		})
	}
	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
	config1 := dut.Config().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name)
	t.Run("Create Policy ", func(t *testing.T) {
		config1.Update(t, baseConfigSchedulerPolicy)
	})
	
	if !setup.SkipGet() {
		t.Run("Get Policy Config", func(t *testing.T) {
			configGot := config1.Get(t)
			if diff := cmp.Diff(*configGot, *baseConfigSchedulerPolicy); diff != "" {
				t.Errorf("Config Schedule fail: \n%v", diff)
			}
		})
	}

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	//baseConfigInterfaceOutput := baseConfigInterface.Output
	//baseConfigInterfaceOutputQueue := setup.GetAnyValue(baseConfigInterfaceOutput.Queue)
	config2 := dut.Config().Qos().Interface(*baseConfigInterface.InterfaceId)
	//state2 := dut.Telemetry().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Queue(*baseConfigInterfaceOutputQueue.Name)
	t.Run("Update with interface config ", func(t *testing.T) {
		config2.Update(t, baseConfigInterface)
	})
	/* Telemetry disabled
	if !setup.SkipSubscribe() {
		t.Run("Get interface queue Telemetry", func(t *testing.T) {
			stateGot := state.Lookup(t)
			if diff := cmp.Diff(*stateGot.Val(t), *baseConfigClassifier); diff != "" {
				t.Errorf("Telemetry Classifier fail: \n%v", diff)
			}
		})
	}*/
	
}
