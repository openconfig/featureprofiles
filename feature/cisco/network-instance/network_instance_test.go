package basetest

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
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
		path := dut.Config().NetworkInstance(instance.name)
		if instance.description != "" {
			request.Description = &instance.description
		}

		defer t.Run("deleteconfig//network-instances/network-instance/config/name", func(t *testing.T) {
			deleteNetworkInstance(t, dut)
		})
		t.Run("updateconfig//network-instances/network-instance/config/name", func(t *testing.T) {
			defer observer.RecordYgot(t, "UPDATE", path.Name())
			path.Update(t, request)
		})
	}
	t.Run("updateconfig//network-instances/network-instance/config/description", func(t *testing.T) {
		verifyUpdateDescription(t, dut)
	})
	t.Run("replaceconfig//network-instances/network-instance/config/description", func(t *testing.T) {
		verifyReplaceDescription(t, dut)
	})
	t.Run("deleteconfig//network-instances/network-instance/config/description", func(t *testing.T) {
		verifyDeleteDescription(t, dut)
	})
	t.Run("pdateconfig//network-instances/network-instance/config/description", func(t *testing.T) {
		verifyUpdateDescription(t, dut)
	})
}
