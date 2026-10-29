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
	"fmt"
	"time"
)

const (
	flowMetricsTimeout        = 3 * time.Minute
	flowCounterSampleInterval = 500 * time.Millisecond
	flowCounterStablePeriod   = 2 * time.Second
)

type flowCounterTracker struct {
	seen           bool
	last           uint64
	unchangedSince time.Time
}

// observe only reports stability after fresh, present observations span the
// stable period. Missing telemetry cannot establish or preserve stability.
func (c *flowCounterTracker) observe(pkts uint64, present bool, now time.Time) bool {
	if !present {
		c.unchangedSince = time.Time{}
		return false
	}
	if !c.seen || c.last != pkts || c.unchangedSince.IsZero() {
		c.unchangedSince = now
	}
	c.seen, c.last = true, pkts
	return now.Sub(c.unchangedSince) >= flowCounterStablePeriod
}

func (c *flowCounterTracker) targetReached(baseline, target uint64) bool {
	return c.seen && c.last >= baseline && c.last-baseline >= target
}

func (c *flowCounterTracker) sentPackets(baseline, target uint64) (uint64, error) {
	if !c.seen {
		return 0, fmt.Errorf("no transmit-counter telemetry received")
	}
	if c.last < baseline {
		return 0, fmt.Errorf("transmit counter reset: last=%d, baseline=%d", c.last, baseline)
	}
	sent := c.last - baseline
	if sent < target {
		return 0, fmt.Errorf("insufficient transmit count: last=%d, baseline=%d, sent=%d, target=%d", c.last, baseline, sent, target)
	}
	return sent, nil
}

func (c *flowCounterTracker) timeoutError(flow, phase string, baseline, target uint64) error {
	if !c.seen {
		return fmt.Errorf("OTG flow %q %s timed out after %v: no transmit-counter telemetry received; baseline=%d, target=%d; traffic completion cannot be verified", flow, phase, flowMetricsTimeout, baseline, target)
	}
	if _, err := c.sentPackets(baseline, target); err != nil {
		return fmt.Errorf("OTG flow %q %s timed out after %v: %w", flow, phase, flowMetricsTimeout, err)
	}
	return fmt.Errorf("OTG flow %q %s timed out after %v despite reaching target: last=%d, baseline=%d, target=%d", flow, phase, flowMetricsTimeout, c.last, baseline, target)
}
