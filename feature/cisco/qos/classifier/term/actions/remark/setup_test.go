package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testSetDscpInput []uint8 = []uint8{
		54,
	}
	testSetMplsTcInput []uint8 = []uint8{
		101,
	}
	testSetDot1pInput []uint8 = []uint8{
		106,
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"Classifier"})
	bcClassifier := setup.GetAnyValue(bc.Classifier)
	setup.ResetStruct(bcClassifier, []string{"Term"})
	bcClassifierTerm := setup.GetAnyValue(bcClassifier.Term)
	setup.ResetStruct(bcClassifierTerm, []string{"Actions"})
	bcClassifierTermActions := bcClassifierTerm.Actions
	setup.ResetStruct(bcClassifierTermActions, []string{"Remark"})
	bcClassifierTermActionsRemark := bcClassifierTermActions.Remark
	setup.ResetStruct(bcClassifierTermActionsRemark, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
