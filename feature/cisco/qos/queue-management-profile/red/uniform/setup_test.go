package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testDropInput []bool = []bool{
		true,
	}
	testMaxThresholdInput []uint64 = []uint64{
		133980922025205922,
	}
	testEnableEcnInput []bool = []bool{
		true,
	}
	testMinThresholdInput []uint64 = []uint64{
		15864297395427335348,
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"QueueManagementProfile"})
	bcQueueManagementProfile := setup.GetAnyValue(bc.QueueManagementProfile)
	setup.ResetStruct(bcQueueManagementProfile, []string{"Red"})
	bcQueueManagementProfileRed := bcQueueManagementProfile.Red
	setup.ResetStruct(bcQueueManagementProfileRed, []string{"Uniform"})
	bcQueueManagementProfileRedUniform := bcQueueManagementProfileRed.Uniform
	setup.ResetStruct(bcQueueManagementProfileRedUniform, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
