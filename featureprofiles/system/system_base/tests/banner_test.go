package system_base_test

import (
	"testing"

	"github.com/openconfig/ondatra"
)

// TestMotdBanner verifies that the MOTD configuration paths can be read,
// updated, and deleted.
//
// config_path:/system/config/motd-banner
// telemetry_path:/system/state/motd-banner
func TestMotdBanner(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	var bannerValues = []string{
		"",
		"x",
		"Warning Text",
		"WARNING : Unauthorized access to this system is forbidden and will be prosecuted by law. By accessing this system, you agree that your actions may be monitored if unauthorized usage is suspected.",
	}
	dut := ondatra.DUT(t, "dut1")
	configB := dut.Config().System().MotdBanner()
	stateB := dut.Config().System().MotdBanner()

	for _, banner := range bannerValues {
		configB.Replace(t, banner)

		configGot := configB.Get(t)
		if configGot != banner {
			t.Errorf("Config MOTD Banner got %s want %s", configGot, banner)
		}

		stateGot := stateB.Get(t)
		if stateGot != banner {
			t.Errorf("Telemetry MOTD Banner got %v want %s", stateGot, banner)
		}
	}

	configB.Delete(t)
	if qs := configB.GetFull(t); qs.IsPresent() == true {
		t.Errorf("Delete MOTD Banner fail; got %v", qs)
	}
}

// TestLoginBanner verifies that the Login Banner configuration paths can be
// read, updated, and deleted.
//
// config_path:/system/config/login-banner
// telemetry_path:/system/state/login-banner
func TestLoginBanner(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	var bannerValues = []string{
		"",
		"x",
		"Warning Text",
		"WARNING : Unauthorized access to this system is forbidden and will be prosecuted by law. By accessing this system, you agree that your actions may be monitored if unauthorized usage is suspected.",
	}
	dut := ondatra.DUT(t, "dut1")
	configB := dut.Config().System().LoginBanner()
	stateB := dut.Config().System().LoginBanner()

	for _, banner := range bannerValues {
		configB.Replace(t, banner)

		configGot := configB.Get(t)
		if configGot != banner {
			t.Errorf("Config Login Banner got %s want %s", configGot, banner)
		}

		stateGot := stateB.Get(t)
		if stateGot != banner {
			t.Errorf("Telemetry Login Banner got %v want %s", stateGot, banner)
		}
	}

	configB.Delete(t)
	if qs := configB.GetFull(t); qs.IsPresent() == true {
		t.Errorf("Delete Login Banner fail; got %v", qs)
	}
}
