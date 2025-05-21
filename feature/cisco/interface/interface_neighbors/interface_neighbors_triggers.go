package interface_neighbors_test

import (
	"sync"
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

func DelMemberPortScale(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup) {

	defer wg.Done()
	batchConfig := &gnmi.SetBatch{}

	for i := 0; i < TOTAL_BUNDLE_INTF; i++ {
		pathb1m1 := gnmi.OC().Interface(mapBundleMemberPorts[dut.ID()][i].MemberPorts[0])
		gnmi.BatchDelete(batchConfig, pathb1m1.Config())
	}
	batchConfig.Set(t, dut)
}

func AddMemberPortScale(t *testing.T, dut *ondatra.DUTDevice, wg *sync.WaitGroup) {

	defer wg.Done()
	batchConfig := &gnmi.SetBatch{}

	for i := 0; i < TOTAL_BUNDLE_INTF; i++ {
		BE := generateBundleMemberInterfaceConfig(mapBundleMemberPorts[dut.ID()][i].MemberPorts[0],
			mapBundleMemberPorts[dut.ID()][i].BundleName)
		pathb1m1 := gnmi.OC().Interface(mapBundleMemberPorts[dut.ID()][i].MemberPorts[0])
		gnmi.BatchUpdate(batchConfig, pathb1m1.Config(), BE)
	}
	batchConfig.Set(t, dut)
}
