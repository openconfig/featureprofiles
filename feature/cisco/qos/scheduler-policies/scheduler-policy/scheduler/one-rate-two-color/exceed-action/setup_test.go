package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testSetDscpInput []uint8 = []uint8{
		114,
	}
	testSetDot1pInput []uint8 = []uint8{
		176,
	}
	testSetMplsTcInput []uint8 = []uint8{
		132,
	}
	testDropInput []bool = []bool{
		false,
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"SchedulerPolicy"})
	bcSchedulerPolicy := setup.GetAnyValue(bc.SchedulerPolicy)
	setup.ResetStruct(bcSchedulerPolicy, []string{"Scheduler"})
	bcSchedulerPolicyScheduler := setup.GetAnyValue(bcSchedulerPolicy.Scheduler)
	setup.ResetStruct(bcSchedulerPolicyScheduler, []string{"OneRateTwoColor"})
	bcSchedulerPolicySchedulerOneRateTwoColor := bcSchedulerPolicyScheduler.OneRateTwoColor
	setup.ResetStruct(bcSchedulerPolicySchedulerOneRateTwoColor, []string{"ExceedAction"})
	bcSchedulerPolicySchedulerOneRateTwoColorExceedAction := bcSchedulerPolicySchedulerOneRateTwoColor.ExceedAction
	setup.ResetStruct(bcSchedulerPolicySchedulerOneRateTwoColorExceedAction, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
