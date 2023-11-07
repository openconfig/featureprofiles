// Package includes funtions to load base config
package sampling_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/sampling/setup"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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
	testInterfaceIngressSamplingRate []uint32 = []uint32{
		60,
	}
	//lint:ignore U1000 Ignore unused function temporarily for debugging
	testInterfaceEgressSamplingRate []uint32 = []uint32{
		70,
	}
)

func setupSampling(t *testing.T, dut *ondatra.DUTDevice) *oc.Sampling {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Sflow"})

	bcSflow := bc.Sflow
	setup.ResetStruct(bcSflow, []string{"Interface"})
	bcSflowInterface := setup.GetAnyValue(bcSflow.Interface)
	setup.ResetStruct(bcSflowInterface, []string{})

	gnmi.Replace(t, dut, gnmi.OC().Sampling().Config(), bc)
	return bc
}

func teardownSampling(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Sampling) {
	gnmi.Delete(t, dut, gnmi.OC().Sampling().Config())
}
