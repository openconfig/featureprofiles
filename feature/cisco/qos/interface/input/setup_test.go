package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testBufferAllocationProfileInput []string = []string{
		"s",
	}
	testMulticastBufferAllocationProfileInput []string = []string{
		"ca:",
	}
	testUnicastBufferAllocationProfileInput []string = []string{
		"asi",
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Interface"})
	bcInterface := setup.GetAnyValue(bc.Interface)
	setup.ResetStruct(bcInterface, []string{"Input"})
	bcInterfaceInput := bcInterface.Input
	setup.ResetStruct(bcInterfaceInput, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
