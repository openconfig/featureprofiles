package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testQueueInput []string = []string{
		":",
	}
	testIdInput []string = []string{
		"cscaia",
	}
	testWeightInput []uint64 = []uint64{
		4628759132720709975,
	}
	testInputTypeInput []oc.E_Input_InputType = []oc.E_Input_InputType{
		oc.E_Input_InputType(2), //IN_PROFILE
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"SchedulerPolicy"})
	bcSchedulerPolicy := setup.GetAnyValue(bc.SchedulerPolicy)
	setup.ResetStruct(bcSchedulerPolicy, []string{"Scheduler"})
	bcSchedulerPolicyScheduler := setup.GetAnyValue(bcSchedulerPolicy.Scheduler)
	setup.ResetStruct(bcSchedulerPolicyScheduler, []string{"Input"})
	bcSchedulerPolicySchedulerInput := setup.GetAnyValue(bcSchedulerPolicyScheduler.Input)
	setup.ResetStruct(bcSchedulerPolicySchedulerInput, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
