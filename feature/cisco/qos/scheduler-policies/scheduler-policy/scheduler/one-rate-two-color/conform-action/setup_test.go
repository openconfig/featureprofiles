package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testSetMplsTcInput []uint8 = []uint8{
		78,
	}
	testSetDot1pInput []uint8 = []uint8{
		100,
	}
	testSetDscpInput []uint8 = []uint8{
		154,
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
	setup.ResetStruct(bcSchedulerPolicySchedulerOneRateTwoColor, []string{"ConformAction"})
	bcSchedulerPolicySchedulerOneRateTwoColorConformAction := bcSchedulerPolicySchedulerOneRateTwoColor.ConformAction
	setup.ResetStruct(bcSchedulerPolicySchedulerOneRateTwoColorConformAction, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
