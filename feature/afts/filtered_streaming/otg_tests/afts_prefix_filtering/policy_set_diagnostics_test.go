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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
)

// diagnosticPolicyGet records the complete response, including parent-valued
// responses, rather than relying on the generated OpenConfig client cache.
func diagnosticPolicyGet(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client gpb.GNMIClient, stage string, paths ...*gpb.Path) (*gpb.GetResponse, error) {
	t.Helper()
	readCtx, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
	defer cancel()
	request := &gpb.GetRequest{
		Prefix: &gpb.Path{Origin: "openconfig", Target: dut.Name()},
		Path:   paths, Type: gpb.GetRequest_CONFIG, Encoding: gpb.Encoding_JSON_IETF,
	}
	response, err := client.Get(readCtx, request)
	output := fmt.Sprintf("Request:\n%s\nResponse:\n%s\nError: %v\n", prototext.Format(request), prototext.Format(response), err)
	saveAFTDiagnostic(t, stage, "gNMI CONFIG Get", output)
	t.Logf("AFT policy Get UTC=%s stage=%q status=%s request=%v response=%v error=%v", time.Now().UTC().Format(time.RFC3339Nano), stage, status.Code(err), request, response, err)
	return response, err
}

func diagnosticPolicyReference(response *gpb.GetResponse) (string, error) {
	for _, notification := range response.GetNotification() {
		for _, update := range notification.GetUpdate() {
			var value any
			switch typed := update.GetVal().GetValue().(type) {
			case *gpb.TypedValue_JsonIetfVal:
				if err := json.Unmarshal(typed.JsonIetfVal, &value); err != nil {
					return "", err
				}
			case *gpb.TypedValue_JsonVal:
				if err := json.Unmarshal(typed.JsonVal, &value); err != nil {
					return "", err
				}
			case *gpb.TypedValue_StringVal:
				return typed.StringVal, nil
			default:
				continue
			}
			if reference, ok := globalFilterPolicyValue(value, "prefix-set"); ok {
				return reference, nil
			}
		}
	}
	return "", fmt.Errorf("no prefix-set reference in exact-leaf Get response")
}

// policySetViews prints a compact comparison of the exact OpenConfig leaf and
// the native RPL text. This is diagnostic evidence, not an AFT assertion.
func policySetViews(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client gpb.GNMIClient, stage, policy, expected string) bool {
	t.Helper()
	response, getErr := diagnosticPolicyGet(ctx, t, dut, client, stage+" exact leaf", statementPrefixSetConfigPath(policy, defaultStatementName))
	var configRef string
	if getErr == nil {
		configRef, getErr = diagnosticPolicyReference(response)
	}
	native, cliErr := logAFTDiagnosticCLIAtStage(ctx, t, dut, stage, "show running-config route-policy "+policy)
	var nativeRef string
	for _, line := range strings.Split(native, "\n") {
		fields := strings.Fields(line)
		for i := 0; i+2 < len(fields); i++ {
			if fields[i] == "destination" && fields[i+1] == "in" {
				nativeRef = fields[i+2]
			}
		}
	}
	output := fmt.Sprintf("stage=%q policy=%q expected=%q OpenConfig=%q nativeRPL=%q GetError=%v CLIError=%v", stage, policy, expected, configRef, nativeRef, getErr, cliErr)
	t.Logf("AFT policy comparison UTC=%s %s", time.Now().UTC().Format(time.RFC3339Nano), output)
	saveAFTDiagnostic(t, stage, "policy comparison", output)
	return getErr == nil && cliErr == nil && configRef == expected && nativeRef == expected
}

// capturePolicyOperation captures the trace rings immediately after the Set,
// before diagnostic Gets and later AFT processing fill them with other work.
func (d aftFilterDiagnostics) capturePolicyOperation(t *testing.T, dut *ondatra.DUTDevice, stage, expected string) {
	t.Helper()
	if dut.Vendor() != ondatra.CISCO {
		return
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(90*time.Second))
	defer cancel()
	t.Logf("AFT policy operation diagnostics start UTC=%s stage=%q", time.Now().UTC().Format(time.RFC3339Nano), stage)
	for _, command := range []string{
		"show grpc trace all",
		"show telemetry model-driven trace all",
		"show configuration commit list 5",
		"show configuration commit changes last 1 diff",
	} {
		logAFTDiagnosticCLIAtStage(ctx, t, dut, stage, command)
	}
	client, err := dialGNMI(t, dut)
	if err != nil {
		t.Logf("AFT policy operation Get unavailable: %v", err)
		return
	}
	policySetViews(ctx, t, dut, client, stage+" shared client", d.policy, expected)
	diagnosticPolicyGet(ctx, t, dut, client, stage+" statement and policy", policyStatementPath(d.policy, defaultStatementName), policyDefinitionPath(d.policy))
	// A new connection checks whether the changed leaf is visible beyond the
	// connection that issued the Set. It does not populate the AFT cache.
	dialCtx, dialCancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
	defer dialCancel()
	freshClient, err := dut.RawAPIs().BindingDUT().DialGNMI(dialCtx)
	if err != nil {
		t.Logf("AFT policy independent DialGNMI failed: %v", err)
		return
	}
	if closer, ok := freshClient.(io.Closer); ok {
		defer closer.Close()
	}
	t.Logf("AFT policy independent client UTC=%s stage=%q client=%p", time.Now().UTC().Format(time.RFC3339Nano), stage, freshClient)
	policySetViews(ctx, t, dut, freshClient, stage+" independent client", d.policy, expected)
}

// diagnosticPolicyObjects builds complete objects for request-scope probes.
// Each probe policy is unique, unattached, and removed after its observation.
func diagnosticPolicyObjects(name, prefixSet string) (map[string]any, map[string]any, map[string]any) {
	statement := map[string]any{
		"name":   defaultStatementName,
		"config": map[string]any{"name": defaultStatementName},
		"conditions": map[string]any{"match-prefix-set": map[string]any{
			"config": map[string]any{"prefix-set": prefixSet, "match-set-options": "ANY"},
		}},
		"actions": map[string]any{"config": map[string]any{"policy-result": "ACCEPT_ROUTE"}},
	}
	policy := map[string]any{
		"name": name, "config": map[string]any{"name": name},
		"statements": map[string]any{"statement": []any{statement}},
	}
	root := map[string]any{"policy-definitions": map[string]any{"policy-definition": []any{policy}}}
	return statement, policy, root
}

func diagnosticPolicySet(ctx context.Context, t *testing.T, client gpb.GNMIClient, stage string, request *gpb.SetRequest) error {
	t.Helper()
	setCtx, cancel := context.WithDeadline(ctx, time.Now().Add(15*time.Second))
	defer cancel()
	started := time.Now()
	response, err := client.Set(setCtx, request)
	output := fmt.Sprintf("StartUTC: %s\nElapsed: %v\nRequest:\n%s\nResponse:\n%s\nError: %v\nStatus:\n%s", started.UTC().Format(time.RFC3339Nano), time.Since(started), prototext.Format(request), prototext.Format(response), err, prototext.Format(status.Convert(err).Proto()))
	saveAFTDiagnostic(t, stage, "gNMI Set", output)
	t.Logf("AFT policy Set probe UTC=%s stage=%q elapsed=%v status=%s request=%v response=%v error=%v", time.Now().UTC().Format(time.RFC3339Nano), stage, time.Since(started), status.Code(err), request, response, err)
	return err
}

// probePolicySetForms runs only after AFT-6.1.4 has failed and its original
// subscription has stopped. Results cannot change that test's pass/fail status.
func probePolicySetForms(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(3*time.Minute))
	defer cancel()
	client, err := dialGNMI(t, dut)
	if err != nil {
		t.Logf("AFT policy Set probes unavailable: %v", err)
		return
	}
	rootPath := &gpb.Path{Elem: []*gpb.PathElem{{Name: "routing-policy"}}}
	forms := []string{"leaf-no-origin", "leaf-openconfig", "match-config-openconfig", "statement-no-origin", "statement-openconfig", "policy-openconfig", "root-openconfig"}
	runID := time.Now().UnixNano()
	for index, form := range forms {
		if dut.Vendor() == ondatra.CISCO && strings.HasPrefix(form, "leaf-") {
			t.Logf("AFT policy Set probe %s skipped: Cisco does not support single-leaf prefix-set updates", form)
			continue
		}
		if ctx.Err() != nil {
			t.Logf("AFT policy Set probes stopped before %s: %v", form, ctx.Err())
			return
		}
		func() {
			name := fmt.Sprintf("AFT-DIAG-%d-%d", runID, index)
			stage := "AFT-6.1.4 probe " + form
			// Keep target empty in both variants so origin is the only
			// difference between the paired requests.
			origin := &gpb.Path{Origin: "openconfig"}
			encode := func(value any) *gpb.TypedValue {
				// All values below contain only strings, slices, and maps.
				data, _ := json.Marshal(value)
				return &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: data}}
			}
			_, _, baseline := diagnosticPolicyObjects(name, v4PfxSetB)
			// Clean up even if creation partially succeeds or the budget ends.
			defer func() {
				cleanupCtx, cleanupCancel := context.WithDeadline(context.Background(), time.Now().Add(20*time.Second))
				defer cleanupCancel()
				cleanupErr := diagnosticPolicySet(cleanupCtx, t, client, stage+" cleanup", &gpb.SetRequest{Prefix: origin, Delete: []*gpb.Path{policyDefinitionPath(name)}})
				remaining, readErr := logAFTDiagnosticCLIAtStage(cleanupCtx, t, dut, stage+" cleanup verification", "show running-config route-policy "+name)
				present := false
				for _, line := range strings.Split(remaining, "\n") {
					present = present || strings.TrimSpace(line) == "route-policy "+name
				}
				t.Logf("AFT policy Set probe cleanup policy=%s nativeStillPresent=%t deleteError=%v readError=%v", name, present, cleanupErr, readErr)
			}()
			if err := diagnosticPolicySet(ctx, t, client, stage+" create baseline", &gpb.SetRequest{
				Prefix: origin, Update: []*gpb.Update{{Path: rootPath, Val: encode(baseline)}},
			}); err != nil {
				return
			}
			if !policySetViews(ctx, t, dut, client, stage+" baseline", name, v4PfxSetB) {
				t.Logf("AFT policy Set probe %s skipped: native and OpenConfig baseline not confirmed", form)
				return
			}
			statement, policy, root := diagnosticPolicyObjects(name, v4PfxSetA)
			path, value, replace := statementPrefixSetConfigPath(name, defaultStatementName), any(v4PfxSetA), false
			switch form {
			case "match-config-openconfig":
				path.Elem = path.Elem[:len(path.Elem)-1]
				value = map[string]any{"prefix-set": v4PfxSetA, "match-set-options": "ANY"}
			case "statement-no-origin", "statement-openconfig":
				path, value, replace = policyStatementPath(name, defaultStatementName), statement, true
				// Match the original statement Replace payload exactly.
				delete(statement, "name")
			case "policy-openconfig":
				path, value, replace = policyDefinitionPath(name), policy, true
			case "root-openconfig":
				path, value = rootPath, root
			}
			request := &gpb.SetRequest{Prefix: origin}
			if strings.HasSuffix(form, "no-origin") {
				request.Prefix = nil
			}
			update := &gpb.Update{Path: path, Val: encode(value)}
			if replace {
				request.Replace = []*gpb.Update{update}
			} else {
				request.Update = []*gpb.Update{update}
			}
			setErr := diagnosticPolicySet(ctx, t, client, stage+" change reference", request)
			// Preserve the mutation's trace before Gets add unrelated records.
			logAFTDiagnosticCLIAtStage(ctx, t, dut, stage, "show grpc trace all")
			consistent := policySetViews(ctx, t, dut, client, stage+" result", name, v4PfxSetA)
			t.Logf("AFT policy Set probe result form=%s policy=%s rpcStatus=%s nativeAndOpenConfigChanged=%t", form, name, status.Code(setErr), consistent)
		}()
	}
}
