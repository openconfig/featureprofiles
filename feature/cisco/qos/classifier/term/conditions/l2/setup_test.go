package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testSourceMacInput []string = []string{
		"1B:EB:Ba:4E:0a:13",
	}
	testSourceMacMaskInput []string = []string{
		"98:90:41:29:E3:fD",
	}
	testEthertypeInput []oc.Qos_Classifier_Term_Conditions_L2_Ethertype_Union = []oc.Qos_Classifier_Term_Conditions_L2_Ethertype_Union{
		oc.UnionUint16(5721),
	}
	testDestinationMacInput []string = []string{
		"1d:dB:79:E6:51:7A",
	}
	testDestinationMacMaskInput []string = []string{
		"CF:FA:08:BC:4C:eb",
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Classifier"})
	bcClassifier := setup.GetAnyValue(bc.Classifier)
	setup.ResetStruct(bcClassifier, []string{"Term"})
	bcClassifierTerm := setup.GetAnyValue(bcClassifier.Term)
	setup.ResetStruct(bcClassifierTerm, []string{"Conditions"})
	bcClassifierTermConditions := bcClassifierTerm.Conditions
	setup.ResetStruct(bcClassifierTermConditions, []string{"L2"})
	bcClassifierTermConditionsL2 := bcClassifierTermConditions.L2
	setup.ResetStruct(bcClassifierTermConditionsL2, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
