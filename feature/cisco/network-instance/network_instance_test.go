package basetest

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestNetworkInstance(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	for _, instance := range instances {
		request := &oc.NetworkInstance{
			Name: &instance.name,
		}
		path := gnmi.OC().NetworkInstance(instance.name)
		if instance.description != "" {
			request.Description = &instance.description
		}

		defer t.Run("Delete//network-instances/network-instance/config/name", func(t *testing.T) {
			deleteNetworkInstance(t, dut)
		})
		t.Run("Update//network-instances/network-instance/config/name", func(t *testing.T) {
			defer observer.RecordYgot(t, "UPDATE", path.Name())
			gnmi.Update(t, dut, path.Config(), request)
		})
	}
	t.Run("Update//network-instances/network-instance/config/description", func(t *testing.T) {
		verifyUpdateDescription(t, dut)
	})
	t.Run("Replace//network-instances/network-instance/config/description", func(t *testing.T) {
		verifyReplaceDescription(t, dut)
	})
	t.Run("Delete//network-instances/network-instance/config/description", func(t *testing.T) {
		verifyDeleteDescription(t, dut)
	})
	t.Run("Update config//network-instances/network-instance/config/description", func(t *testing.T) {
		verifyUpdateDescription(t, dut)
	})
}
