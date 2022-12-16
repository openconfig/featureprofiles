package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygnmi/ygnmi"
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
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), bc)
	return bc
}
func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	gnmi.Delete(t, dut, gnmi.OC().Qos().Config())
}
