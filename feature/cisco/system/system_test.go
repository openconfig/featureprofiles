package system_base_test

import (
	"testing"

	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
)

func TestSystemContainerUpdate(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	for _, system := range systemContainers {
		container := &oc.System{}
		container.Hostname = system.hostname
		path := dut.Config().System()
		defer observer.RecordYgot(t, "UPDATE", path)
		path.Update(t, container)
	}
}
