package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	//"github.com/openconfig/testt"
)

var (
	testTypeInput []oc.E_Qos_Classifier_Type = []oc.E_Qos_Classifier_Type{
		oc.E_Qos_Classifier_Type(2),
	}
	testNameInput []string = []string{
		"pmap_new",
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice, baseConfigFile string) *oc.Qos {
	bc := setup.BaseConfig(baseConfigFile)
	setup.ResetStruct(bc, []string{"Classifier"})
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), bc)
	return bc
}
func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	gnmi.Delete(t, dut, gnmi.OC().Qos().Config())
}
