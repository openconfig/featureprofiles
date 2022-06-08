package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testDestinationPortInput []oc.Qos_Classifier_Term_Conditions_Transport_DestinationPort_Union = []oc.Qos_Classifier_Term_Conditions_Transport_DestinationPort_Union{
		oc.UnionUint16(21706),
	}
	testSourcePortInput []oc.Qos_Classifier_Term_Conditions_Transport_SourcePort_Union = []oc.Qos_Classifier_Term_Conditions_Transport_SourcePort_Union{
		oc.UnionUint16(62616),
	}
	testTcpFlagsInput [][]oc.E_PacketMatchTypes_TCP_FLAGS = [][]oc.E_PacketMatchTypes_TCP_FLAGS{
		{
			oc.E_PacketMatchTypes_TCP_FLAGS(1), //TCP_ACK
			oc.E_PacketMatchTypes_TCP_FLAGS(3), //TCP_ECE
			oc.E_PacketMatchTypes_TCP_FLAGS(2), //TCP_CWR
			oc.E_PacketMatchTypes_TCP_FLAGS(7), //TCP_SYN
			oc.E_PacketMatchTypes_TCP_FLAGS(6), //TCP_RST
			oc.E_PacketMatchTypes_TCP_FLAGS(5), //TCP_PSH
			oc.E_PacketMatchTypes_TCP_FLAGS(4), //TCP_FIN
			oc.E_PacketMatchTypes_TCP_FLAGS(8), //TCP_URG
		},
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
	setup.ResetStruct(bcClassifierTermConditions, []string{"Transport"})
	bcClassifierTermConditionsTransport := bcClassifierTermConditions.Transport
	setup.ResetStruct(bcClassifierTermConditionsTransport, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
