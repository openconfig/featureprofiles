package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testNameInput []string = []string{
		"acii",
	}
	testStaticSharedBufferLimitInput []uint32 = []uint32{
		2327206852,
	}
	testSharedBufferLimitTypeInput []oc.E_Qos_SHARED_BUFFER_LIMIT_TYPE = []oc.E_Qos_SHARED_BUFFER_LIMIT_TYPE{
		oc.E_Qos_SHARED_BUFFER_LIMIT_TYPE(1), //DYNAMIC_BASED_ON_SCALING_FACTOR
	}
	testDedicatedBufferInput []uint64 = []uint64{
		17695640880842010619,
	}
	testDynamicLimitScalingFactorInput []int32 = []int32{
		2084197017,
	}
	testUseSharedBufferInput []bool = []bool{
		true,
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"BufferAllocationProfile"})
	bcBufferAllocationProfile := setup.GetAnyValue(bc.BufferAllocationProfile)
	setup.ResetStruct(bcBufferAllocationProfile, []string{"Queue"})
	bcBufferAllocationProfileQueue := setup.GetAnyValue(bcBufferAllocationProfile.Queue)
	setup.ResetStruct(bcBufferAllocationProfileQueue, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
