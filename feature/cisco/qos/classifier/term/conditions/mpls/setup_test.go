package qos_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/cisco/qos/setup"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
)

var (
	testTrafficClassInput []uint8 = []uint8{
		7,
	}
)

func setupQos(t *testing.T, dut *ondatra.DUTDevice, baseConfigFile string) *oc.Qos {
	bc := setup.BaseConfig(baseConfigFile)
	setup.ResetStruct(bc, []string{"Classifier"})
	gnmi.Replace(t, dut, gnmi.OC().Qos().Config(), bc)
	return bc
}

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	var err *string
	for attempt := 1; attempt <= 2; attempt++ {
		err = testt.CaptureFatal(t, func(t testing.TB) {
			gnmi.Delete(t, dut, gnmi.OC().Qos().Config())
		})
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Errorf(*err)
	}
}
