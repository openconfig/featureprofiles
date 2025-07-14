package static_route_test

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

func DelAddMemberPort(t *testing.T, dut *ondatra.DUTDevice,
	dutPorts []string, bundlePort ...[]string) {

	batchConfig := &gnmi.SetBatch{}

	if len(bundlePort) > 0 {
		for i := 0; i < len(bundlePort); i++ {
			BE := generateBundleMemberInterfaceConfig(dutPorts[i], bundlePort[i][0])
			pathb1m1 := gnmi.OC().Interface(dutPorts[i])
			gnmi.BatchReplace(batchConfig, pathb1m1.Config(), BE)
		}
	} else {
		for i := 0; i < len(dutPorts); i++ {
			pathb1m1 := gnmi.OC().Interface(dutPorts[i])
			gnmi.BatchDelete(batchConfig, pathb1m1.Config())
		}
	}
	batchConfig.Set(t, dut)
}
