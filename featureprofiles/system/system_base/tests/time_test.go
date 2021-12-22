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
