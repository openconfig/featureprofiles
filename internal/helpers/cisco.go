// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package helpers provides Cisco-specific helper utilities for test cases.
package helpers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"

	spb "github.com/openconfig/gnoi/system"
)

const (
	maxRebootTime = 30 // minutes
)

// ConfigureHwProfile configures the Cisco hardware profile required for AFT scale on the DUT.
func ConfigureHwProfile(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	ciscoConfig := `
		hw-module profile cef iptunnel scale
		lpts pifib hardware police flow tpa rate 20000 
		`
	GnmiCLIConfig(t, dut, ciscoConfig)
	RebootDUT(t, dut)
	return nil
}

// ConfigureDefaultHwProfile resets Cisco hardware profile to default on the DUT.
func ConfigureDefaultHwProfile(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	ciscoConfig := `
		no hw-module profile cef iptunnel scale
		`
	GnmiCLIConfig(t, dut, ciscoConfig)
	RebootDUT(t, dut)
	return nil
}

// RebootDUT performs a cold reboot on the DUT and waits for it to come back up.
func RebootDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	rebootRequest := &spb.RebootRequest{
		Method:  spb.RebootMethod_COLD,
		Delay:   0,
		Message: "Reboot chassis without delay",
		Force:   true,
	}
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(t.Context())
	if err != nil {
		t.Fatalf("Error dialing gNOI: %v", err)
	}
	bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v %v", bootTimeBeforeReboot, time.Now())
	t.Log("Sending reboot request to DUT")

	ctxWithTimeout, cancel := context.WithTimeout(t.Context(), 8*time.Minute)
	defer cancel()
	_, err = gnoiClient.System().Reboot(ctxWithTimeout, rebootRequest)
	defer gnoiClient.System().CancelReboot(t.Context(), &spb.CancelRebootRequest{})
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}

	// Wait for the device to become reachable again.
	deviceBootStatus(t, dut)
	t.Logf("Device is reachable, waiting for boot time to update.")

	bootTimeAfterReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time after reboot: %v", bootTimeAfterReboot)
}

// deviceBootStatus waits for the DUT to boot up by polling telemetry.
func deviceBootStatus(t *testing.T, dut *ondatra.DUTDevice) {
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f minutes since reboot started.", time.Since(startReboot).Minutes())

		time.Sleep(3 * time.Minute)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}

		if uint64(time.Since(startReboot).Minutes()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	t.Logf("Device boot time: %.2f minutes", time.Since(startReboot).Minutes())
}

const aftCollectorPolicyCommand = "show running-config telemetry model-driven | include collector"

// A zero value preserves the absence of an explicit override, including when
// the platform default happens to equal the requested test setting.
type aftCollectorPolicy struct {
	minimumMiB   int
	minimumItems int
	stallSeconds int
}

var aftScaleCollectorPolicy = aftCollectorPolicy{minimumMiB: 1, minimumItems: 1000, stallSeconds: 10}

func (p aftCollectorPolicy) config() string {
	rate := "no collector minimum-dequeue-rate"
	if p.minimumMiB != 0 {
		rate = fmt.Sprintf("collector minimum-dequeue-rate %d %d", p.minimumMiB, p.minimumItems)
	}
	stall := "no collector stall-timeout"
	if p.stallSeconds != 0 {
		stall = fmt.Sprintf("collector stall-timeout %d", p.stallSeconds)
	}
	return fmt.Sprintf("telemetry model-driven\n %s\n %s\n!\n", rate, stall)
}

func parseAFTCollectorPolicy(output string) (aftCollectorPolicy, error) {
	var policy aftCollectorPolicy
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "%") || strings.Contains(strings.ToLower(line), "syntax error") {
			return aftCollectorPolicy{}, fmt.Errorf("collector policy CLI failed: %s", line)
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "collector" {
			continue
		}
		switch fields[1] {
		case "minimum-dequeue-rate":
			if len(fields) != 4 || policy.minimumMiB != 0 {
				return aftCollectorPolicy{}, fmt.Errorf("unsupported or duplicate collector rate: %q", line)
			}
			mib, err1 := strconv.Atoi(fields[2])
			items, err2 := strconv.Atoi(fields[3])
			if err1 != nil || err2 != nil || mib < 1 || mib > 100 || items < 1000 || items > 100000 {
				return aftCollectorPolicy{}, fmt.Errorf("invalid collector rate: %q", line)
			}
			policy.minimumMiB, policy.minimumItems = mib, items
		case "stall-timeout":
			if len(fields) != 3 || policy.stallSeconds != 0 {
				return aftCollectorPolicy{}, fmt.Errorf("unsupported or duplicate collector timeout: %q", line)
			}
			seconds, err := strconv.Atoi(fields[2])
			if err != nil || seconds < 1 || seconds > 10 {
				return aftCollectorPolicy{}, fmt.Errorf("invalid collector timeout: %q", line)
			}
			policy.stallSeconds = seconds
		}
	}
	return policy, nil
}

// changeAFTCollectorPolicy registers restoration before attempting a write: a
// failed RPC can still have committed configuration on the device.
func changeAFTCollectorPolicy(read func() (aftCollectorPolicy, error), write func(aftCollectorPolicy) error, cleanup func(func() error)) error {
	previous, err := read()
	if err != nil {
		return fmt.Errorf("read original AFT collector policy: %w", err)
	}
	apply := func(want aftCollectorPolicy) error {
		if err := write(want); err != nil {
			return err
		}
		got, err := read()
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("AFT collector policy readback mismatch: got %q, want %q", got.config(), want.config())
		}
		return nil
	}
	cleanup(func() error { return apply(previous) })
	return apply(aftScaleCollectorPolicy)
}

// ConfigureAFTCollectorPolicy temporarily adjusts Cisco collector protection for
// full-scale AFT collection. Call after the initial profile reboot and before
// opening subscriptions. Cleanup restores and verifies the original overrides
// after the test's deferred hardware-profile reset and reboot, even on failure.
// These values come from dedicated XR slow-collector tests, not a qualification
// recommendation; a pass uses adjusted collector protection.
func ConfigureAFTCollectorPolicy(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	read := func() (aftCollectorPolicy, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// Some bindings ignore command contexts. Bound our wait and let the
		// worker finish without retaining or logging through a completed test.
		type result struct {
			output string
			err    error
		}
		done := make(chan result, 1)
		go func() {
			cli, err := dut.RawAPIs().BindingDUT().DialCLI(ctx)
			if err != nil {
				done <- result{err: err}
				return
			}
			if cli == nil {
				done <- result{err: fmt.Errorf("binding returned no CLI client")}
				return
			}
			if closer, ok := cli.(interface{ Close() error }); ok {
				defer closer.Close()
			}
			response, err := cli.RunCommand(ctx, aftCollectorPolicyCommand)
			if err != nil {
				done <- result{err: err}
				return
			}
			if response == nil {
				done <- result{err: fmt.Errorf("binding returned no collector policy result")}
				return
			}
			if response.Error() != "" {
				done <- result{err: fmt.Errorf("collector policy CLI: %s", response.Error())}
				return
			}
			done <- result{output: response.Output()}
		}()
		select {
		case got := <-done:
			if got.err != nil {
				return aftCollectorPolicy{}, got.err
			}
			policy, err := parseAFTCollectorPolicy(got.output)
			if err == nil {
				t.Logf("AFT_COLLECTOR_POLICY readback: %q", policy.config())
			}
			return policy, err
		case <-ctx.Done():
			return aftCollectorPolicy{}, fmt.Errorf("collector policy read timed out: %w", ctx.Err())
		}
	}
	write := func(policy aftCollectorPolicy) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		client, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
		if err != nil {
			return err
		}
		request, err := buildCliConfigRequest(policy.config())
		if err != nil {
			return err
		}
		t.Logf("AFT_COLLECTOR_POLICY applying: %q", policy.config())
		_, err = client.Set(ctx, request)
		return err
	}
	return changeAFTCollectorPolicy(read, write, func(restore func() error) {
		t.Cleanup(func() {
			if err := restore(); err != nil {
				t.Errorf("failed to restore original AFT collector policy: %v", err)
			}
		})
	})
}
