package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

var (
	testUnicastOutputQueueInput []string = []string{
		"s",
	}
	testOutputQueueInput []string = []string{
		"c",
	}
	testNameInput []string = []string{
		"cca",
	}
	testFabricPriorityInput []uint8 = []uint8{
		132,
	}
	testMulticastOutputQueueInput []string = []string{
		"i",
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice) *oc.Qos {
	bc := setup.BaseConfig()
	setup.ResetStruct(bc, []string{"ForwardingGroup"})
	bcForwardingGroup := setup.GetAnyValue(bc.ForwardingGroup)
	setup.ResetStruct(bcForwardingGroup, []string{})
	dut.Config().Qos().Replace(t, bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	dut.Config().Qos().Delete(t)
}
