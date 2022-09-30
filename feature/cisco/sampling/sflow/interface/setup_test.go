// Package includes funtions to load base config
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
	testNameInput []string = []string{
		"Bundle-Ether1.1",
		"Bundle-Ether1",
		"FourHundredGigE0/0/0/0",
		"FourHundredGigE0/0/0/0.1",
	}
)

func setupSampling(t *testing.T, dut *ondatra.DUTDevice) *oc.Sampling {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Sflow"})

	bcSflow := bc.Sflow
	setup.ResetStruct(bcSflow, []string{"Interface"})
	bcSflowInterface := setup.GetAnyValue(bcSflow.Interface)
	setup.ResetStruct(bcSflowInterface, []string{})

	dut.Config().Sampling().Replace(t, bc)
	return bc
}

func teardownSampling(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Sampling) {
	dut.Config().Sampling().Delete(t)
}
