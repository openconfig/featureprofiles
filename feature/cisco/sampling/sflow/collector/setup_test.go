package sampling_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/sampling/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
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
)

func setupSampling(t *testing.T, dut *ondatra.DUTDevice) *oc.Sampling {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Sflow"})

	bcSflow := bc.Sflow
	setup.ResetStruct(bcSflow, []string{"Collector"})
	bcSflowCollector := setup.GetAnyValue(bcSflow.Collector)
	setup.ResetStruct(bcSflowCollector, []string{})

	dut.Config().Sampling().Replace(t, bc)
	return bc
}

func teardownSampling(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Sampling) {
	dut.Config().Sampling().Delete(t)
}
