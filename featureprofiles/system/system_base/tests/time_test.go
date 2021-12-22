package system_base_test

import (
	"testing"
	"time"

	"github.com/openconfig/ondatra"
)

// TestCurrentDateTime verifies that the current date and time state path can
// be parsed as RFC3339 time format.
//
// TODO(bstoll): Need working implementation to validate against
//
// telemetry_path:/system/state/current-datetime
func TestCurrentDateTime(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	dut := ondatra.DUT(t, "dut1")
	now := dut.Telemetry().System().CurrentDatetime().Get(t)
	_, err := time.Parse(time.RFC3339, now)
	if err != nil {
		t.Errorf("Failed to parse current time: got %s: %s", now, err)
	}
}

// TestBootTime verifies the timestamp that the system was last restarted can
// be read and is not an unreasonable value.
//
// telemetry_path:/system/state/boot-time
func TestBootTime(t *testing.T) {
	dut := ondatra.DUT(t, "dut1")
	bt := dut.Telemetry().System().BootTime().Get(t)
	if bt == 0 {
		t.Errorf("Unexpected boot timestamp: got %d", bt)
	}

	// Boot time should be after Dec 22, 2021 00:00:00 GMT in nanoseconds
	if bt < 1640131200000000000 {
		t.Errorf("Unexpected boot timestamp: got %d; check clock", bt)
	}
}

// TestTimeZone verifies the timezone-name config values can be read and set
//
// config_path:/system/config/timezone-name
// telemetry_path:/system/state/timezone-name
func TestTimeZone(t *testing.T) {
	t.Skip("Need working implementation to validate against")

	dut := ondatra.DUT(t, "dut1")
	configTz := dut.Config().System().Clock().TimezoneName()
	stateTz := dut.Telemetry().System().Clock().TimezoneName()

	var timezones = []string{
		"Etc/UTC",
		"Etc/GMT",
		"UTC",
		"GMT",
		"America/Chicago",
		"PST8PDT",
		"Europe/London",
	}

	for _, timezone := range timezones {
		configTz.Replace(t, timezone)

		configGot := configTz.Get(t)
		if configGot != timezone {
			t.Errorf("Config timezone got %s want %s", configGot, timezone)
		}

		stateGot := stateTz.Await(t, 5*time.Second, timezone)
		success := false
		for _, v := range stateGot {
			if v.Present && v.Val(t) == timezone {
				success = true
			}
		}
		if !success {
			t.Errorf("Telemetry timezone got %v want %s", stateGot, timezone)
		}
	}

	configTz.Delete(t)
	if qs := configTz.GetFull(t); qs.IsPresent() == true {
		t.Errorf("Delete timezone fail; got %v", qs)
	}
}
