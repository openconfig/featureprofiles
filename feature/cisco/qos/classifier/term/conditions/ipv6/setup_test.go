package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testHopLimitInput []uint8 = []uint8{
		189,
	}
	testDestinationFlowLabelInput []uint32 = []uint32{
		1035957,
	}
	testDestinationAddressInput []string = []string{
		"9:b::B/21",
	}
	testDscpInput []uint8 = []uint8{
		44,
	}
	testSourceAddressInput []string = []string{
		"dc5::cD2/3",
	}
	testSourceFlowLabelInput []uint32 = []uint32{
		974597,
	}
	testDscpSetInput [][]uint8 = [][]uint8{
		{
			25,
			7,
			13,
			59,
			19,
			26,
			46,
			63,
			49,
			63,
			62,
			46,
			60,
			47,
			61,
			38,
			58,
			46,
			36,
			20,
		},
	}
	testProtocolInput []oc.Qos_Classifier_Term_Conditions_Ipv6_Protocol_Union = []oc.Qos_Classifier_Term_Conditions_Ipv6_Protocol_Union{
		oc.UnionUint8(40),
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
	setup.ResetStruct(bcClassifierTermConditions, []string{"Ipv6"})
	bcClassifierTermConditionsIpv6 := bcClassifierTermConditions.Ipv6
	setup.ResetStruct(bcClassifierTermConditionsIpv6, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
