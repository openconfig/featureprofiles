package qos_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/featureprofiles/topologies/binding"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	ondatra.RunTests(m, binding.New)
}

// TestQueueSchedule will verifies that the Queue + Scheduler-config paths can be read , updated and deleted.

func TestQueueSchedule(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	baseConfig := setup.BaseConfig("scheduler_base.json") // this is not setting anything in Router just return config.
	defer teardownQos(t, dut, baseConfig)

	baseConfigQueue := setup.GetAnyValue(baseConfig.Queue)
	config := gnmi.OC().Qos().Queue(*baseConfigQueue.Name)
	t.Run("Create Queue ", func(t *testing.T) {
		gnmi.Update(t, dut, config.Config(), baseConfigQueue)
	})
	if !setup.SkipGet() {
		t.Run("Get queue Config", func(t *testing.T) {
			configGot := gnmi.GetConfig(t, dut, config.Config())
			t.Logf("got this config: \n%v", configGot)
			if diff := cmp.Diff(*configGot, *baseConfigQueue); diff != "" {
				t.Errorf("Config queue fail: \n%v", diff)
			}
		})
	}

	baseConfigSchedulerPolicy := setup.GetAnyValue(baseConfig.SchedulerPolicy)
	config1 := gnmi.OC().Qos().SchedulerPolicy(*baseConfigSchedulerPolicy.Name)
	t.Run("Create Policy ", func(t *testing.T) {
		gnmi.Update(t, dut, config1.Config(), baseConfigSchedulerPolicy)
	})
	if !setup.SkipGet() {
		t.Run("Get Policy Config", func(t *testing.T) {
			configGot := gnmi.GetConfig(t, dut, config1.Config())
			if diff := cmp.Diff(*configGot, *baseConfigSchedulerPolicy); diff != "" {
				t.Errorf("Config Schedule fail: \n%v", diff)
			}
		})
	}

	baseConfigInterface := setup.GetAnyValue(baseConfig.Interface)
	baseConfigInterfaceOutput := baseConfigInterface.Output
	baseConfigInterfaceOutputQueue := setup.GetAnyValue(baseConfigInterfaceOutput.Queue)
	config2 := gnmi.OC().Qos().Interface(*baseConfigInterface.InterfaceId)
	state2 := gnmi.OC().Qos().Interface(*baseConfigInterface.InterfaceId).Output().Queue(*baseConfigQueue.Name)
	t.Run("Update with interface config ", func(t *testing.T) {
		gnmi.Update(t, dut, config2.Config(), baseConfigInterface)
	})

	if !setup.SkipSubscribe() {
		t.Run("Get interface queue Telemetry", func(t *testing.T) {
			stateGot := gnmi.Lookup(t, dut, state2.State())
			if diff := cmp.Diff(*stateGot.Val(t), *baseConfigInterfaceOutputQueue); diff == "" {
				t.Errorf("Telemetry interface subscribe  fail: \n%v", diff)
			}
		})
	}

	//delete interface
	//configure root level
	//defer which gonna delete root level.

}
