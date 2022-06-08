package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testDropInput []bool = []bool{
		false,
	}
	testSetMplsTcInput []uint8 = []uint8{
		196,
	}
	testSetDscpInput []uint8 = []uint8{
		173,
	}
	testSetDot1pInput []uint8 = []uint8{
		109,
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"SchedulerPolicy"})
	bcSchedulerPolicy := setup.GetAnyValue(bc.SchedulerPolicy)
	setup.ResetStruct(bcSchedulerPolicy, []string{"Scheduler"})
	bcSchedulerPolicyScheduler := setup.GetAnyValue(bcSchedulerPolicy.Scheduler)
	setup.ResetStruct(bcSchedulerPolicyScheduler, []string{"TwoRateThreeColor"})
	bcSchedulerPolicySchedulerTwoRateThreeColor := bcSchedulerPolicyScheduler.TwoRateThreeColor
	setup.ResetStruct(bcSchedulerPolicySchedulerTwoRateThreeColor, []string{"ExceedAction"})
	bcSchedulerPolicySchedulerTwoRateThreeColorExceedAction := bcSchedulerPolicySchedulerTwoRateThreeColor.ExceedAction
	setup.ResetStruct(bcSchedulerPolicySchedulerTwoRateThreeColorExceedAction, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
