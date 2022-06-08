package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testNameInput []string = []string{
		":a",
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Interface"})
	bcInterface := setup.GetAnyValue(bc.Interface)
	setup.ResetStruct(bcInterface, []string{"Output"})
	bcInterfaceOutput := bcInterface.Output
	setup.ResetStruct(bcInterfaceOutput, []string{"SchedulerPolicy"})
	bcInterfaceOutputSchedulerPolicy := bcInterfaceOutput.SchedulerPolicy
	setup.ResetStruct(bcInterfaceOutputSchedulerPolicy, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
