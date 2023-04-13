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
	testSampleSizeInput []uint16 = []uint16{
		256,
	}
	testAgentIdv4Input []string = []string{
		"5.5.5.1",
	}
	testAgentIdv6Input []string = []string{
		"5::1",
	}
	testDscpInput []uint8 = []uint8{
		60,
	}
	testPollingIntervalInput []uint16 = []uint16{
		60,
	}
	testIngressSamplingRate []uint32 = []uint32{
		100,
	}
)

func setupSampling(t *testing.T, dut *ondatra.DUTDevice) *oc.Sampling {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Sflow"})
	bcSflow := bc.Sflow
	setup.ResetStruct(bcSflow, []string{})
	gnmi.Replace(t, dut, gnmi.OC().Sampling().Config(), bc)
	return bc
}

func teardownSampling(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Sampling) {
	t.Log("teardownSampling")
	gnmi.Delete(t, dut, gnmi.OC().Sampling().Config())
}
