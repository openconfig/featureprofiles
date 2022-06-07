package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testDscpInput []uint8 = []uint8{
		63,
	}
	testDscpSetInput [][]uint8 = [][]uint8{
		[]uint8{
			30,
			56,
			26,
			21,
			15,
			22,
			29,
			2,
			56,
			33,
			10,
			58,
			28,
			38,
			48,
			62,
			12,
			59,
			22,
			40,
			61,
			60,
			47,
			8,
			54,
			9,
			27,
			24,
			37,
			27,
			40,
			42,
			42,
			49,
			44,
		},
	}
	testHopLimitInput []uint8 = []uint8{
		59,
	}
	testSourceAddressInput []string = []string{
		"37.154.42.169/32",
	}
	testDestinationAddressInput []string = []string{
		"1.2.8.197/29",
	}
	testProtocolInput []oc.Qos_Classifier_Term_Conditions_Ipv4_Protocol_Union = []oc.Qos_Classifier_Term_Conditions_Ipv4_Protocol_Union{
		oc.UnionUint8(87),
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
	setup.ResetStruct(bcClassifierTermConditions, []string{"Ipv4"})
	bcClassifierTermConditionsIpv4 := bcClassifierTermConditions.Ipv4
	setup.ResetStruct(bcClassifierTermConditionsIpv4, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
