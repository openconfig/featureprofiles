package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testMaxQueueDepthPercentInput []uint8 = []uint8{
		12,
	}
	testMaxQueueDepthPacketsInput []uint32 = []uint32{
		1474112130,
	}
	testCirPctInput []uint8 = []uint8{
		52,
	}
	testCirPctRemainingInput []uint8 = []uint8{
		33,
	}
	testMaxQueueDepthBytesInput []uint32 = []uint32{
		4073638605,
	}
	testQueuingBehaviorInput []oc.E_QosTypes_QueueBehavior = []oc.E_QosTypes_QueueBehavior{
		oc.E_QosTypes_QueueBehavior(1), //SHAPE
	}
	testBcInput []uint32 = []uint32{
		3221016771,
	}
	testCirInput []uint64 = []uint64{
		1925011161009534377,
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
	setup.ResetStruct(bcSchedulerPolicySchedulerOneRateTwoColor, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
