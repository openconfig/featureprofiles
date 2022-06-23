package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testBeInput []uint32 = []uint32{
		2606530334,
	}
	testPirInput []uint64 = []uint64{
		2751897996307395216,
	}
	testPirPctInput []uint8 = []uint8{
		72,
	}
	testCirInput []uint64 = []uint64{
		12235732223147839691,
	}
	testCirPctInput []uint8 = []uint8{
		63,
	}
	testBcInput []uint32 = []uint32{
		1180383655,
	}
	testCirPctRemainingInput []uint8 = []uint8{
		39,
	}
	testPirPctRemainingInput []uint8 = []uint8{
		63,
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
	setup.ResetStruct(bcSchedulerPolicySchedulerTwoRateThreeColor, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
