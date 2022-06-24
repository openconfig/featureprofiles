package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	baseConfigPath                = "base_config_interface.json"
	testInterfaceIdInput []string = []string{
		"FourHundredGigE0/0/0/1",
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Interface", "Classifier"})
	bcClassifier := setup.GetAnyValue(bc.Classifier)
	bcInterface := setup.GetAnyValue(bc.Interface)
	dut.Config().Qos().Classifier(*bcClassifier.Name).Update(t, bcClassifier)
	dut.Config().Qos().Interface(*bcInterface.InterfaceId).Update(t, bcInterface)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
