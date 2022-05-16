package networkinstance_base

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
)

var (
	device1   = "dut1"
	observer  = fptest.NewObserver("NetworkInstance").AddCsvRecorder()
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
		path := dut.Config().NetworkInstance(instance.name).Description()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, instance.description+"Updated")
	}

}
func verifyReplaceDescription(t *testing.T, dut *ondatra.DUTDevice) {
	for _, instance := range instances {
		path := dut.Config().NetworkInstance(instance.name).Description()
		defer observer.RecordYgot(t, "REPLACE", path)
		path.Update(t, instance.description+"REPLACE")
	}

}
func verifyDeleteDescription(t *testing.T, dut *ondatra.DUTDevice) {
	for _, instance := range instances {
		path := dut.Config().NetworkInstance(instance.name).Description()
		defer observer.RecordYgot(t, "DELETE", path)
		path.Delete(t)
	}

}
func deleteNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	for _, instance := range instances {
		path := dut.Config().NetworkInstance(instance.name)
		defer observer.RecordYgot(t, "DELETE", path)
		path.Delete(t)
	}

}
