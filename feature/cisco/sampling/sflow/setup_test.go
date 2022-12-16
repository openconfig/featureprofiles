// Package includes funtions to load base config
package sampling_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/sampling/setup"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygnmi/ygnmi"
)

var (
	testEnabledInput []bool = []bool{
		true,
	}
	testSampleSizeInput []uint16 = []uint16{
		256,
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
