package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testPriorityInput []oc.E_Scheduler_Priority = []oc.E_Scheduler_Priority{
		oc.E_Scheduler_Priority(1), //STRICT
	}
	testSequenceInput []uint32 = []uint32{
		2311126647,
	}
	testTypeInput []oc.E_QosTypes_QOS_SCHEDULER_TYPE = []oc.E_QosTypes_QOS_SCHEDULER_TYPE{
		oc.E_QosTypes_QOS_SCHEDULER_TYPE(2), //TWO_RATE_THREE_COLOR
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"SchedulerPolicy"})
	bcSchedulerPolicy := setup.GetAnyValue(bc.SchedulerPolicy)
	setup.ResetStruct(bcSchedulerPolicy, []string{"Scheduler"})
	bcSchedulerPolicyScheduler := setup.GetAnyValue(bcSchedulerPolicy.Scheduler)
	setup.ResetStruct(bcSchedulerPolicyScheduler, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
