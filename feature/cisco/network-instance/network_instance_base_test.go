package basetest

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

var (
	device1  = "dut"
	observer = fptest.NewObserver("NetworkInstance").AddCsvRecorder("ocreport").
			AddCsvRecorder("NetworkInstance")
	instances = []NetworkInstance{
		{
			name:        "vrf_test1",
			description: "description for vrf_test1",
		},
		{
			name:        "vrf_test2",
			description: "description for vrf_test2",
		},
		{
			name:        "vrf_test3",
			description: "description for vrf_test3",
		},
		{
			name:        "vrf_test4",
			description: "description for vrf_test4",
		},
	}
)

type NetworkInstance struct {
	name        string
	description string
}

func verifyUpdateDescription(t *testing.T, dut *ondatra.DUTDevice) {
	for _, instance := range instances {
		path := gnmi.OC().NetworkInstance(instance.name).Description()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), instance.description+"Updated")
	}
}

func verifyReplaceDescription(t *testing.T, dut *ondatra.DUTDevice) {
	for _, instance := range instances {
		path := gnmi.OC().NetworkInstance(instance.name).Description()
		defer observer.RecordYgot(t, "REPLACE", path)
		gnmi.Update(t, dut, path.Config(), instance.description+"REPLACE")
	}

}

func verifyDeleteDescription(t *testing.T, dut *ondatra.DUTDevice) {
	for _, instance := range instances {
		path := gnmi.OC().NetworkInstance(instance.name).Description()
		defer observer.RecordYgot(t, "DELETE", path)
		gnmi.Delete(t, dut, path.Config())
	}

}

func deleteNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	for _, instance := range instances {
		path := gnmi.OC().NetworkInstance(instance.name)
		defer observer.RecordYgot(t, "DELETE", path)
		gnmi.Delete(t, dut, path.Config())
	}

}
