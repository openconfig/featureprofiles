// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package traceroute_packetin_with_vrf_selection_test

import (
	"strings"
	"testing"
	"time"
)

func TestFlowCounterStability(t *testing.T) {
	start := time.Unix(1, 0)
	var counter flowCounterTracker
	for _, tc := range []struct {
		name    string
		pkts    uint64
		present bool
		elapsed time.Duration
		stable  bool
	}{
		{"absent", 0, false, 0, false},
		{"first_sample", 1000, true, 0, false},
		{"too_early", 1000, true, time.Second, false},
		{"stable", 1000, true, 2 * time.Second, true},
		{"counter_changed", 1001, true, 3 * time.Second, false},
		{"changed_stable", 1001, true, 5 * time.Second, true},
		{"telemetry_missing", 0, false, 6 * time.Second, false},
		{"telemetry_resumed", 1001, true, 7 * time.Second, false},
		{"resumed_stable", 1001, true, 9 * time.Second, true},
		{"counter_reset", 0, true, 10 * time.Second, false},
	} {
		if got := counter.observe(tc.pkts, tc.present, start.Add(tc.elapsed)); got != tc.stable {
			t.Errorf("%s: stable=%v, want %v", tc.name, got, tc.stable)
		}
	}
}

func TestFlowCounterValidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		seen     bool
		last     uint64
		baseline uint64
		want     uint64
		error    string
	}{
		{"no_telemetry", false, 0, 0, 0, "no transmit-counter telemetry"},
		{"zero", true, 0, 0, 0, "insufficient transmit count"},
		{"short", true, 999, 0, 0, "sent=999"},
		{"target", true, 1000, 0, 1000, ""},
		{"nonzero_baseline", true, 1500, 500, 1000, ""},
		{"short_delta", true, 1000, 500, 0, "sent=500"},
		{"reset", true, 1, 500, 0, "counter reset"},
		{"excess_preserves_actual_count", true, 1001, 0, 1001, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			counter := flowCounterTracker{seen: tc.seen, last: tc.last}
			got, err := counter.sentPackets(tc.baseline, 1000)
			if got != tc.want {
				t.Errorf("sentPackets=%d, want %d", got, tc.want)
			}
			if tc.error == "" {
				if err != nil {
					t.Fatalf("sentPackets: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.error) {
				t.Fatalf("sentPackets error=%v, want %q", err, tc.error)
			}
			if got := counter.targetReached(tc.baseline, 1000); got != (tc.error == "") {
				t.Errorf("targetReached=%v, want %v", got, tc.error == "")
			}
		})
	}
}

func TestDelayedFlowCounterTelemetry(t *testing.T) {
	// A delayed OTG metrics response must still provide an observed target
	// count, not merely expire a longer timer.
	const delay = 122 * time.Second
	if flowMetricsTimeout <= delay {
		t.Fatalf("Metrics timeout %v does not cover observed delay %v", flowMetricsTimeout, delay)
	}
	start := time.Unix(1, 0)
	var counter flowCounterTracker
	counter.observe(0, false, start)
	if counter.targetReached(0, 1000) {
		t.Fatal("Missing telemetry incorrectly satisfied the transmit target")
	}
	if counter.observe(1000, true, start.Add(delay)) {
		t.Fatal("First delayed sample incorrectly established stability")
	}
	if !counter.targetReached(0, 1000) {
		t.Fatal("Observed delayed target count was rejected")
	}
	if !counter.observe(1000, true, start.Add(delay+flowCounterStablePeriod)) {
		t.Fatal("Fresh unchanged samples did not establish stability")
	}
}

func TestFlowCounterTimeoutDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		counter flowCounterTracker
		want    string
	}{
		{flowCounterTracker{}, "no transmit-counter telemetry received"},
		{flowCounterTracker{seen: true, last: 499}, "counter reset"},
		{flowCounterTracker{seen: true, last: 1499}, "sent=999"},
		{flowCounterTracker{seen: true, last: 1500}, "despite reaching target"},
	} {
		got := tc.counter.timeoutError("flow4in4-8", "counter stabilization", 500, 1000).Error()
		for _, want := range []string{tc.want, "flow4in4-8", "counter stabilization", "3m0s"} {
			if !strings.Contains(got, want) {
				t.Errorf("Diagnostic %q does not contain %q", got, want)
			}
		}
	}
}
