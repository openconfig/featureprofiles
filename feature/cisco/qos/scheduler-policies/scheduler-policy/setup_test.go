package qos_test

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygnmi/ygnmi"
	//	"github.com/openconfig/testt"
)

func teardownQos(t *testing.T, dut *ondatra.DUTDevice, baseConfig *oc.Qos) {
	gnmi.Delete(t, dut, gnmi.OC().Qos().Config())
}
