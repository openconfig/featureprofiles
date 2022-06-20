package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testEnableEcnInput []bool = []bool{
		true,
	}
	testMinThresholdInput []uint64 = []uint64{
		3529592816735691873,
	}
	testWeightInput []uint32 = []uint32{
		1470766556,
	}
	testMaxThresholdInput []uint64 = []uint64{
		819501211464602070,
	}
	testDropInput []bool = []bool{
		false,
	}
	testMaxDropProbabilityPercentInput []uint8 = []uint8{
		64,
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"QueueManagementProfile"})
	bcQueueManagementProfile := setup.GetAnyValue(bc.QueueManagementProfile)
	setup.ResetStruct(bcQueueManagementProfile, []string{"Wred"})
	bcQueueManagementProfileWred := bcQueueManagementProfile.Wred
	setup.ResetStruct(bcQueueManagementProfileWred, []string{"Uniform"})
	bcQueueManagementProfileWredUniform := bcQueueManagementProfileWred.Uniform
	setup.ResetStruct(bcQueueManagementProfileWredUniform, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
