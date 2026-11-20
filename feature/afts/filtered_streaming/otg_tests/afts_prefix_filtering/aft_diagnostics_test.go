// Copyright 2026 Google LLC
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

package aftsprefixfiltering_test

import (
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"
)

// aftFilterDiagnostics records the policy and prefixes whose effective state
// must be preserved before a failed subtest returns to the parent suite.
type aftFilterDiagnostics struct {
	networkInstance string
	policy          string
	prefixSets      []string
	prefixes        []string
}

// logAFTDiagnosticCLI uses a separate deadline for each command. In particular,
// a collector timeout must not cancel the commands that explain that timeout.
func logAFTDiagnosticCLI(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, command string) {
	t.Helper()
	logAFTDiagnosticCLIAtStage(ctx, t, dut, "", command)
}

// saveAFTDiagnostic preserves complete output under the runner's persistent log
// directory. Test temporary directories are deleted before FireX copies logs.
func saveAFTDiagnostic(t *testing.T, stage, command, output string) {
	t.Helper()
	var directory string
	for _, name := range []string{"log_dir", "outputs_dir"} {
		if option := flag.Lookup(name); option != nil && option.Value.String() != "" {
			directory = option.Value.String()
			break
		}
	}
	if directory == "" {
		directory = os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR")
	}
	if directory == "" {
		directory = "."
	}
	directory = filepath.Join(directory, "aft_diagnostics")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Logf("AFT diagnostic artifact directory unavailable: %v", err)
		return
	}
	label := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '_'
	}, stage+"-"+command)
	if len(label) > 120 {
		label = label[:120]
	}
	suffix := ".txt"
	if len(output) > 64*1024 {
		suffix += ".gz"
	}
	file, err := os.CreateTemp(directory, label+"-*"+suffix)
	if err != nil {
		t.Logf("AFT diagnostic artifact creation failed: %v", err)
		return
	}
	if err := file.Chmod(0644); err != nil {
		t.Logf("AFT diagnostic artifact permissions for %s: %v", file.Name(), err)
	}
	var writer io.Writer = file
	var compressed *gzip.Writer
	if strings.HasSuffix(suffix, ".gz") {
		compressed = gzip.NewWriter(file)
		writer = compressed
	}
	_, headerErr := fmt.Fprintf(writer, "UTC: %s\nTest: %s\nStage: %s\nCommand: %s\n\n", time.Now().UTC().Format(time.RFC3339Nano), t.Name(), stage, command)
	_, writeErr := io.WriteString(writer, output)
	var compressErr error
	if compressed != nil {
		compressErr = compressed.Close()
	}
	closeErr := file.Close()
	t.Logf("AFT diagnostic full artifact=%s uncompressedBytes=%d headerError=%v writeError=%v compressError=%v closeError=%v", file.Name(), len(output), headerErr, writeErr, compressErr, closeErr)
}

func logAFTDiagnosticCLIAtStage(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, stage, command string) (string, error) {
	t.Helper()
	if dut.Vendor() != ondatra.CISCO {
		return "", nil
	}
	if ctx.Err() != nil {
		t.Logf("AFT diagnostic CLI skipped command=%q: %v", command, ctx.Err())
		return "", ctx.Err()
	}
	var output string
	var commandErr error
	if failure := testt.CaptureFatal(t, func(tb testing.TB) {
		readCtx, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
		defer cancel()
		started := time.Now()
		t.Logf("AFT diagnostic CLI start UTC=%s command=%q", started.UTC().Format(time.RFC3339Nano), command)
		response, err := dut.RawAPIs().CLI(tb).RunCommand(readCtx, command)
		if err != nil {
			commandErr = err
			t.Logf("AFT diagnostic CLI command=%q elapsed=%v failed: %v", command, time.Since(started), err)
			return
		}
		output = response.Output()
		saveAFTDiagnostic(t, stage, command, output)
		sample := output
		const maxOutput = 64 * 1024
		if len(sample) > maxOutput {
			t.Logf("AFT diagnostic CLI command=%q output=%d bytes; logging last %d bytes", command, len(output), maxOutput)
			sample = sample[len(sample)-maxOutput:]
		}
		t.Logf("AFT diagnostic CLI end UTC=%s stage=%q command=%q elapsed=%v:\n%s", time.Now().UTC().Format(time.RFC3339Nano), stage, command, time.Since(started), sample)
	}); failure != nil {
		commandErr = fmt.Errorf("%s", *failure)
		t.Logf("AFT diagnostic CLI command=%q unavailable: %s", command, *failure)
	}
	return output, commandErr
}

// capture records native configuration and the OpenConfig filter binding.
// The PoC oper_cache_ctl tool is unavailable on the image under investigation.
func (d aftFilterDiagnostics) capture(t *testing.T, dut *ondatra.DUTDevice, stage string) {
	t.Helper()
	if dut.Vendor() != ondatra.CISCO {
		return
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(90*time.Second))
	defer cancel()
	t.Logf("AFT filter diagnostics start UTC=%s stage=%q NI=%s policy=%s", time.Now().UTC().Format(time.RFC3339Nano), stage, d.networkInstance, d.policy)
	defer func() {
		t.Logf("AFT filter diagnostics end UTC=%s stage=%q contextError=%v", time.Now().UTC().Format(time.RFC3339Nano), stage, ctx.Err())
	}()
	logAFTDiagnosticCLIAtStage(ctx, t, dut, stage, "show running-config route-policy "+d.policy)
	for _, name := range d.prefixSets {
		logAFTDiagnosticCLIAtStage(ctx, t, dut, stage, "show running-config prefix-set "+name)
	}
	client, err := dialGNMI(t, dut)
	if err != nil {
		t.Logf("AFT diagnostic global-filter Get unavailable: %v", err)
		return
	}
	path := globalFilterPolicyPath(t, d.networkInstance, "ipv4-policy")
	path.Elem = path.Elem[:len(path.Elem)-2] // Read config and state together.
	path.Origin = ""                         // The request prefix supplies the OpenConfig origin.
	readCtx, readCancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
	defer readCancel()
	request := &gpb.GetRequest{
		Prefix: &gpb.Path{Origin: "openconfig", Target: dut.Name()},
		Path:   []*gpb.Path{path}, Type: gpb.GetRequest_ALL, Encoding: gpb.Encoding_JSON_IETF,
	}
	response, err := client.Get(readCtx, request)
	t.Logf("AFT diagnostic global-filter Get UTC=%s stage=%q request=%v response=%v error=%v", time.Now().UTC().Format(time.RFC3339Nano), stage, request, response, err)
	if d.policy == policyPfxSetA {
		diagnosticPolicyGet(ctx, t, dut, client, stage+" OpenConfig policy views",
			statementPrefixSetConfigPath(d.policy, defaultStatementName),
			policyStatementPath(d.policy, defaultStatementName), policyDefinitionPath(d.policy))
	}
}

// captureFailure runs from a defer, including after t.Fatalf on a collector
// timeout. It observes the failing configuration without retrying a Set or
// reattaching a policy. A fresh stream is opened only after the old one stops.
func (d aftFilterDiagnostics) captureFailure(t *testing.T, dut *ondatra.DUTDevice, stage string, cancel context.CancelFunc, collector *aftcache.AFTStreamSession) {
	t.Helper()
	if dut.Vendor() != ondatra.CISCO || !t.Failed() {
		return
	}
	t.Logf("AFT failure diagnostics UTC=%s stage=%q clientStreamProgress=%+v (buffer is on the test client)", time.Now().UTC().Format(time.RFC3339Nano), stage, collector.Progress())
	d.capture(t, dut, stage+" failure before collector cancellation")
	ctx, diagnosticCancel := context.WithDeadline(context.Background(), time.Now().Add(45*time.Second))
	defer diagnosticCancel()
	for _, command := range []string{"show logging", "show grpc trace all", "show telemetry model-driven trace all"} {
		logAFTDiagnosticCLI(ctx, t, dut, command)
	}
	cancel()
	waitCtx, waitCancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer waitCancel()
	if err := collector.WaitStopped(waitCtx); err != nil {
		t.Logf("Raw filtered-prefix probe skipped: main AFT collector did not stop: %v", err)
		return
	}
	got, err := readGlobalFilterPolicy(t, dut, d.networkInstance, "ipv4-policy")
	t.Logf("AFT failure diagnostic active-policy check UTC=%s stage=%q got=%q expected=%q error=%v", time.Now().UTC().Format(time.RFC3339Nano), stage, got, d.policy, err)
	if err != nil || got != d.policy {
		t.Log("Raw filtered-prefix probe skipped: expected filter is no longer confirmed active")
		return
	}
	probeRawFilteredPrefixes(t, dut, d.networkInstance, d.prefixes)
	if d.policy == policyPfxSetA && strings.Contains(stage, "prefix deletions after swap") {
		// The original failure and raw stream are preserved before any probe
		// configuration is created. These policies are never attached to AFT.
		probePolicySetForms(t, dut)
	}
}
