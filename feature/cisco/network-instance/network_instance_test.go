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

// TestNetworkInstance validates that creating a new vrf and
// config_path:
//    /network-instances/network-instance[name=*]/openconfig-network-instance:config/name
//    /network-instances/network-instance[name=*]/openconfig-network-instance:config/description
// telemetry_path:/system/ntp/config/enabled
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
		defer observer.RecordYgot(t, "UPDATE", path)
		defer observer.RecordYgot(t, "UPDATE", path.Name())
		defer t.Run("deleteconfig//network-instances/network-instance/openconfig-network-instance:config/name", func(t *testing.T) {
			deleteNetworkInstance(t, dut)
		})
		t.Run("updateconfig//network-instances/network-instance/openconfig-network-instance:config/name", func(t *testing.T) {
			path.Update(t, request)
		})
	}
	t.Run("updateconfig//network-instances/network-instance/openconfig-network-instance:config/description", func(t *testing.T) {
		verifyUpdateDescription(t, dut)
	})
	t.Run("replaceconfig//network-instances/network-instance/openconfig-network-instance:config/description", func(t *testing.T) {
		verifyReplaceDescription(t, dut)
	})
	t.Run("deleteconfig//network-instances/network-instance/openconfig-network-instance:config/description", func(t *testing.T) {
		verifyDeleteDescription(t, dut)
	})
	t.Run("pdateconfig//network-instances/network-instance/openconfig-network-instance:config/description", func(t *testing.T) {
		verifyUpdateDescription(t, dut)
	})
}
