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
	testNetworkInstanceInput []string = []string{
		"VRF1",
	}
	testPortInput []uint16 = []uint16{
		9062,
	}
	testAddressInput []string = []string{
		"2.2.2.2",
		"2000::1",
	}
	testSourceAddressInput []string = []string{
		"2.2.2.2",
		"2000::1",
	}
)

func setupSampling(t *testing.T, dut *ondatra.DUTDevice) *oc.Sampling {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Sflow"})

	bcSflow := bc.Sflow
	setup.ResetStruct(bcSflow, []string{"Collector"})
	bcSflowCollector := setup.GetAnyValue(bcSflow.Collector)
	setup.ResetStruct(bcSflowCollector, []string{})

	gnmi.Replace(t, dut, gnmi.OC().Sampling().Config(), bc)
	return bc
}

func teardownSampling(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Sampling) {
	gnmi.Delete(t, dut, gnmi.OC().Sampling().Config())
}
