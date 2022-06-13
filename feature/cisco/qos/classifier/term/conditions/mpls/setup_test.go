package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testTrafficClassInput []uint8 = []uint8{
		7,
	}
	testEndLabelValueInput []oc.Qos_Classifier_Term_Conditions_Mpls_EndLabelValue_Union = []oc.Qos_Classifier_Term_Conditions_Mpls_EndLabelValue_Union{
		oc.UnionUint32(242328),
	}
	testStartLabelValueInput []oc.Qos_Classifier_Term_Conditions_Mpls_StartLabelValue_Union = []oc.Qos_Classifier_Term_Conditions_Mpls_StartLabelValue_Union{
		oc.UnionUint32(580628),
	}
	testTtlValueInput []uint8 = []uint8{
		90,
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
	setup.ResetStruct(bcClassifierTermConditions, []string{"Mpls"})
	bcClassifierTermConditionsMpls := bcClassifierTermConditions.Mpls
	setup.ResetStruct(bcClassifierTermConditionsMpls, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
