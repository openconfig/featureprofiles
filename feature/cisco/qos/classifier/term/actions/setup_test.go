package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
//	"github.com/openconfig/testt"
)

var (
	testTargetGroupInput []string = []string{
		"tc5",
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice, baseConfigFile string) *oc.Qos {
	bc := setup.BaseConfig(baseConfigFile)
	setup.ResetStruct(bc, []string{"Classifier", "ForwardingGroup", "Queue"})
	dut.Config().Qos().Replace(t, bc)
	return bc
}
func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
        dut.Config().Qos().Delete(t)
}

