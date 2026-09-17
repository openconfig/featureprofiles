package fptest

import (
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestPruneUnpushableNetworkInstances(t *testing.T) {
	for _, tc := range []struct {
		name       string
		vendor     ondatra.Vendor
		wantPruned bool
	}{
		{name: "Cisco", vendor: ondatra.CISCO, wantPruned: true},
		{name: "NonCisco", vendor: ondatra.ARISTA, wantPruned: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := &oc.Root{}
			for _, name := range []string{"DEFAULT", "vrf-any", "**iid"} {
				config.GetOrCreateNetworkInstance(name)
			}

			PruneUnpushableNetworkInstances(tc.vendor, config)

			if got := config.GetNetworkInstance("DEFAULT"); got == nil {
				t.Error("PruneUnpushableNetworkInstances() removed pushable network instance DEFAULT")
			}
			for _, name := range []string{"vrf-any", "**iid"} {
				gotPruned := config.GetNetworkInstance(name) == nil
				if gotPruned != tc.wantPruned {
					t.Errorf("PruneUnpushableNetworkInstances() network instance %q pruned: %v, want %v", name, gotPruned, tc.wantPruned)
				}
			}
		})
	}

	t.Run("NilConfig", func(t *testing.T) {
		PruneUnpushableNetworkInstances(ondatra.CISCO, nil)
	})
}
