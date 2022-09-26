package sampling_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/sampling/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testEnabledInput []bool = []bool{
		true,
	}
	testSamplingRateInput []uint32 = []uint32{
		710,
	}
	testSampleSizeInput []uint16 = []uint16{
		256,
	}
	testSourceAddressInput []string = []string{
		"1.1.1.5",
		"2001::1",
	}
)

func setupSampling(t *testing.T, dut *ondatra.DUTDevice) *oc.Sampling {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Sflow"})
	bcSflow := bc.Sflow
	setup.ResetStruct(bcSflow, []string{})
	dut.Config().Sampling().Replace(t, bc)
	return bc
}

func teardownSampling(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Sampling) {
	t.Log("teardownSampling")
	dut.Config().Sampling().Delete(t)
}
