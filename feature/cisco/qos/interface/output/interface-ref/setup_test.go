package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testInterfaceInput []string = []string{
		"s:ic",
	}
	testSubinterfaceInput []uint32 = []uint32{
		4029735886,
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Interface"})
	bcInterface := setup.GetAnyValue(bc.Interface)
	setup.ResetStruct(bcInterface, []string{"Output"})
	bcInterfaceOutput := bcInterface.Output
	setup.ResetStruct(bcInterfaceOutput, []string{"InterfaceRef"})
	bcInterfaceOutputInterfaceRef := bcInterfaceOutput.InterfaceRef
	setup.ResetStruct(bcInterfaceOutputInterfaceRef, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
