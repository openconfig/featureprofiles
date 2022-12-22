package basetest

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestSystemContainerUpdate(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	for _, system := range systemContainers {
		container := &oc.System{}
		container.Hostname = system.hostname
		path := gnmi.OC().System()
		defer observer.RecordYgot(t, "UPDATE", path)
		gnmi.Update(t, dut, path.Config(), container)
	}
}
