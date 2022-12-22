package qos_test

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
)

func teardownQos(t *testing.T, dut *ondatra.DUTDevice) {
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
