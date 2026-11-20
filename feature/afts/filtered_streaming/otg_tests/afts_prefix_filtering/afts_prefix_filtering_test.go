// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

// Package aftsprefixfiltering implements AFT-6.1:
// AFT Prefix Filtering.
package aftsprefixfiltering_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	aftpf "github.com/openconfig/featureprofiles/internal/aft_prefix_filtering"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	v4PfxSetA            = "PREFIX-SET-A"
	v4PfxSetB            = "PREFIX-SET-B-V4"
	v6PfxSetB            = "PREFIX-SET-B"
	vrfPfxSetA           = "PREFIX-SET-VRF-A"
	subnetPfxSetV4       = "PREFIX-SET-SUBNET"
	subnetPfxSetV6       = "PREFIX-SET-SUBNET-V6"
	policyPfxSetA        = "POLICY-PREFIX-SET-A"
	policyPfxSetB        = "POLICY-PREFIX-SET-B"
	policyPfxSetBV4      = "POLICY-PREFIX-SET-B-V4"
	policyVrfA           = "POLICY-PREFIX-SET-VRF-A"
	policySubnet         = "POLICY-SUBNET"
	policySubnetV6       = "POLICY-SUBNET-V6"
	policyMultiStmt      = "POLICY-MULTI-STMT"
	policyDenyPfxSetA    = "POLICY-DENY-PREFIX-SET-A"
	policyTagMatch       = "POLICY-TAG-MATCH"
	policyDropAll        = "POLICY-DROP-ALL"
	policyNotYetExist    = "POLICY-DOES-NOT-YET-EXIST"
	pfxSetNotYetExist    = "PREFIX-SET-DOES-NOT-YET-EXIST"
	secondStatementName  = "20"
	tagStatementName     = "100"
	notificationWaitTime = 30 * time.Second
	policyMatchAll       = aftpf.PolicyMatchAll
	defaultStatementName = aftpf.DefaultStatementName
	pfxMode              = aftpf.PfxMode
	subscriptionWait     = aftpf.AFTSubscriptionWait
	aftCheckInterval     = 45 * time.Second
	staticRouteIndex     = aftpf.StaticRouteIndex
	defaultV4Route       = "0.0.0.0/0"
	defaultV6Route       = "::/0"
)

var (
	atePort1           = aftpf.ATEPort1
	dialGNMI           = aftpf.DialGNMI
	deleteGlobalFilter = aftpf.AFTDeleteGlobalFilter
	baseIPv4Prefixes   = []string{
		"198.51.100.0/24",
		"203.0.113.0/28",
		"100.64.0.0/24",
	}
	baseIPv6Prefixes = []string{
		"2001:db8:1::/64",
		"2001:db8:2::/64",
		"2001:db8:3::/64",
	}
	// Cisco excludes testbed-provided defaults through policyPrefixSetMembers;
	// other vendors retain their original default-route coverage.
	pfxSetAMembers = []string{
		"198.51.100.0/24",
		"203.0.113.0/28",
		"198.51.100.1/32",
	}
	pfxSetBMembers = []string{
		"2001:db8:2::/64",
		"2001:db8:2::1/128",
	}
	// No route for this IPv4 prefix is installed by AFT-6.1.4. This lets
	// Cisco's IPv4 policy swap avoid the cross-family reference in the
	// previously rejected Set request.
	v4PfxSetBMembers = []string{"192.0.2.128/25"}
)

func policyPrefixSetMembers(dut *ondatra.DUTDevice, ipv4 bool) []string {
	members, defaultRoute := pfxSetBMembers, defaultV6Route
	if ipv4 {
		members, defaultRoute = pfxSetAMembers, defaultV4Route
	}
	if dut.Vendor() == ondatra.CISCO {
		// This test does not install Cisco's defaults through the ATE, so
		// their next hops must not be assumed to match the test routes.
		return members
	}
	return append(append([]string(nil), members...), defaultRoute)
}

// policyStringValue keeps the original leaf encoding on other vendors.
func policyStringValue(dut *ondatra.DUTDevice, value string) *gpb.TypedValue {
	if dut.Vendor() == ondatra.CISCO {
		// XR requires JSON_IETF for these string-valued configuration Sets.
		encoded, _ := json.Marshal(value) // A string always marshals successfully.
		return &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: encoded}}
	}
	return &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: value}}
}

// configurePolicies configures the routing policies and prefix-sets required by the AFT-6.1 test procedures
func configurePolicies(t *testing.T, dut *ondatra.DUTDevice, batch *gnmi.SetBatch) {
	t.Helper()
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
		PolicyName:     policyMatchAll,
		StatementNames: []string{defaultStatementName},
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
		PolicyName:     policyPfxSetA,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{v4PfxSetA},
		PrefixList:     policyPrefixSetMembers(dut, true),
		PrefixMode:     pfxMode,
		MatchPrefixSet: true,
		MatchSetOption: oc.E_RoutingPolicy_MatchSetOptionsRestrictedType(oc.RoutingPolicy_MatchSetOptionsType_ANY),
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
		PolicyName:     policyPfxSetB,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{v6PfxSetB},
		PrefixList:     policyPrefixSetMembers(dut, false),
		PrefixMode:     pfxMode,
		MatchPrefixSet: true,
		MatchSetOption: oc.E_RoutingPolicy_MatchSetOptionsRestrictedType(oc.RoutingPolicy_MatchSetOptionsType_ANY),
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	if dut.Vendor() == ondatra.CISCO {
		// AFT-6.1.4 swaps the prefix-set of an IPv4 policy. Use a second
		// IPv4 set with no installed AFT entry on Cisco; keep the existing
		// IPv6 PREFIX-SET-B for the independent IPv6 cases.
		aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
			PolicyName:     policyPfxSetBV4,
			StatementNames: []string{defaultStatementName},
			PrefixSetNames: []string{v4PfxSetB},
			PrefixList:     v4PfxSetBMembers,
			PrefixMode:     pfxMode,
			MatchPrefixSet: true,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
	}
	aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
		PolicyName:     policyVrfA,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{vrfPfxSetA},
		PrefixList:     []string{"100.64.1.0/24"},
		PrefixMode:     "24..32",
		MatchPrefixSet: true,
		MatchSetOption: oc.E_RoutingPolicy_MatchSetOptionsRestrictedType(oc.RoutingPolicy_MatchSetOptionsType_ANY),
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
		PolicyName:     policySubnet,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{subnetPfxSetV4},
		PrefixList:     []string{"203.0.113.0/24"},
		PrefixMode:     "25..32",
		MatchPrefixSet: true,
		MatchSetOption: oc.E_RoutingPolicy_MatchSetOptionsRestrictedType(oc.RoutingPolicy_MatchSetOptionsType_ANY),
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
		PolicyName:     policySubnetV6,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{subnetPfxSetV6},
		PrefixList:     []string{"2001:db8:3::/64"},
		PrefixMode:     "65..128",
		MatchPrefixSet: true,
		MatchSetOption: oc.E_RoutingPolicy_MatchSetOptionsRestrictedType(oc.RoutingPolicy_MatchSetOptionsType_ANY),
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
		PolicyName:     policyMultiStmt,
		StatementNames: []string{defaultStatementName, secondStatementName},
		PrefixSetNames: []string{v4PfxSetA, subnetPfxSetV4},
		MatchPrefixSet: true,
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
		PolicyName:     policyDenyPfxSetA,
		StatementNames: []string{defaultStatementName, secondStatementName},
		PrefixSetNames: []string{v4PfxSetA, ""},
		MatchPrefixSet: true,
		PrefixDeny:     true,
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	if dut.Vendor() == ondatra.CISCO {
		// Cisco's AFT filter supports an unconditional drop, while a tag
		// condition disables filtering because it is unsupported.
		aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
			PolicyName:     policyDropAll,
			StatementNames: []string{defaultStatementName},
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE,
		})
	} else {
		aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
			PolicyName:     policyTagMatch,
			StatementNames: []string{tagStatementName},
			MatchPrefixSet: true,
			SetTag:         true,
			PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
		})
	}
	gnmi.BatchReplace(batch, gnmi.OC().RoutingPolicy().Config(), rp)
	batch.Set(t, dut)
}

// addSingleStaticRoute adds one static route to the given network instance.
// batch.Set fails the test directly on error, so there is no error to
// propagate to the caller.
func addSingleStaticRoute(t *testing.T, dut *ondatra.DUTDevice, niName, prefix, index, nextHop string) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	cfgplugins.ConfigureStaticRoute(t, dut, batch, cfgplugins.ConfigureStaticRouteParams{
		NetworkInstance: niName,
		Prefix:          prefix,
		Index:           index,
		NextHop:         nextHop,
	})
	batch.Set(t, dut)
}

// removeStaticRoute deletes a static route (keyed by prefix) from the given
// network instance. gnmi.Delete fails the test directly on error, so there
// is no error to propagate to the caller.
func removeStaticRoute(t *testing.T, dut *ondatra.DUTDevice, niName, prefix string) {
	t.Helper()
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(niName).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).
		Static(prefix).Config())
}

// policyDefinitionPath builds the gNMI path for a policy-definition list entry.
// TODO: replace this raw gNMI path with the ygot-generated OC path once the
// OC path is supported.
func policyDefinitionPath(policyName string) *gpb.Path {
	return &gpb.Path{
		Elem: []*gpb.PathElem{
			{Name: "routing-policy"},
			{Name: "policy-definitions"},
			{Name: "policy-definition", Key: map[string]string{"name": policyName}},
		},
	}
}

// globalFilterPolicyPath builds the gNMI path for an afts/global-filter policy
// config leaf (ipv4-policy or ipv6-policy) under a network-instance.
// TODO: replace this raw gNMI path with the ygot-generated OC path once the
// afts/global-filter schema is supported in featureprofiles.
func globalFilterPolicyPath(niName, leaf string) *gpb.Path {
	return &gpb.Path{
		Elem: []*gpb.PathElem{
			{Name: "network-instances"},
			{Name: "network-instance", Key: map[string]string{"name": niName}},
			{Name: "afts"},
			{Name: "global-filter"},
			{Name: "config"},
			{Name: leaf},
		},
	}
}

// statementPrefixSetConfigPath builds the gNMI path for the match-prefix-set
// prefix-set leaf of a policy statement.
// TODO: replace this raw gNMI path with the ygot-generated OC path once the
// OC path is supported.
func statementPrefixSetConfigPath(policyName, statementName string) *gpb.Path {
	return &gpb.Path{
		Elem: []*gpb.PathElem{
			{Name: "routing-policy"},
			{Name: "policy-definitions"},
			{Name: "policy-definition", Key: map[string]string{"name": policyName}},
			{Name: "statements"},
			{Name: "statement", Key: map[string]string{"name": statementName}},
			{Name: "conditions"},
			{Name: "match-prefix-set"},
			{Name: "config"},
			{Name: "prefix-set"},
		},
	}
}

func policyStatementPath(policyName, statementName string) *gpb.Path {
	path := statementPrefixSetConfigPath(policyName, statementName)
	path.Elem = path.Elem[:5]
	return path
}

// setGlobalFilterExpectCode sets the global-filter policy leaves and returns an
// error unless the gNMI Set fails with exactly the given status code.
func setGlobalFilterExpectCode(t *testing.T, dut *ondatra.DUTDevice, niName, v4Policy, v6Policy string,
	wantCode codes.Code) error {
	t.Helper()
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Log("Skipping AFT global-filter negative check: unsupported on EOS")
			return nil
		}
	}

	client, err := dialGNMI(t, dut)
	if err != nil {
		return err
	}
	var updates []*gpb.Update
	if v4Policy != "" {
		updates = append(updates, &gpb.Update{
			Path: globalFilterPolicyPath(niName, "ipv4-policy"),
			Val:  policyStringValue(dut, v4Policy),
		})
	}
	if v6Policy != "" {
		updates = append(updates, &gpb.Update{
			Path: globalFilterPolicyPath(niName, "ipv6-policy"),
			Val:  policyStringValue(dut, v6Policy),
		})
	}
	req := &gpb.SetRequest{Update: updates}
	t.Logf("Sending AFT global-filter negative-check gNMI SetRequest: %v", req)
	_, setErr := client.Set(context.Background(), req)
	if setErr == nil {
		if wantCode == codes.OK {
			return nil
		}
		return fmt.Errorf("gNMI Set succeeded, want error with code %v", wantCode)
	}
	if got := status.Code(setErr); got != wantCode {
		return fmt.Errorf("unexpected gNMI Set error code: got %v, want %v (err: %v)", got, wantCode, setErr)
	}
	return nil
}

// deletePolicyExpectCode attempts to delete a policy-definition and returns
// an error unless the gNMI Set RPC returns the given status code.
func deletePolicyExpectCode(t *testing.T, dut *ondatra.DUTDevice, policyName string, wantCode codes.Code) error {
	t.Helper()
	client, err := dialGNMI(t, dut)
	if err != nil {
		return err
	}
	req := &gpb.SetRequest{Delete: []*gpb.Path{policyDefinitionPath(policyName)}}
	t.Logf("Sending policy-definition delete gNMI SetRequest: %v", req)
	_, setErr := client.Set(context.Background(), req)
	if setErr == nil {
		if wantCode == codes.OK {
			t.Logf("Deleted policy %s while it was referenced by the global filter", policyName)
			return nil
		}
		return fmt.Errorf("gNMI Set deleting policy %s succeeded, want error with code %v", policyName, wantCode)
	}
	if got := status.Code(setErr); got != wantCode {
		return fmt.Errorf("unexpected gNMI Set error code: got %v, want %v (err: %v)", got, wantCode, setErr)
	}
	t.Logf("Received expected %v error deleting policy %s: %v", wantCode, policyName, setErr)
	return nil
}

// readGlobalFilterPolicy checks the configured policy, not the streamed state.
func readGlobalFilterPolicy(t *testing.T, dut *ondatra.DUTDevice, niName, leaf string) (string, error) {
	t.Helper()
	client, err := dialGNMI(t, dut)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	resp, err := client.Get(ctx, &gpb.GetRequest{
		Prefix:   &gpb.Path{Origin: "openconfig", Target: dut.Name()},
		Path:     []*gpb.Path{globalFilterPolicyPath(niName, leaf)},
		Type:     gpb.GetRequest_CONFIG,
		Encoding: aftpf.ConfigEncoding(dut),
	})
	if err != nil {
		return "", fmt.Errorf("read global-filter %s: %w", leaf, err)
	}
	for _, notification := range resp.GetNotification() {
		for _, update := range notification.GetUpdate() {
			if update.GetVal() == nil {
				return "", fmt.Errorf("global-filter %s Get returned an empty update: %v", leaf, update)
			}
			var value any
			switch typed := update.GetVal().GetValue().(type) {
			case *gpb.TypedValue_JsonIetfVal:
				if err := json.Unmarshal(typed.JsonIetfVal, &value); err != nil {
					return "", fmt.Errorf("decode global-filter %s: %w", leaf, err)
				}
			case *gpb.TypedValue_JsonVal:
				if err := json.Unmarshal(typed.JsonVal, &value); err != nil {
					return "", fmt.Errorf("decode global-filter %s: %w", leaf, err)
				}
			case *gpb.TypedValue_StringVal:
				value = typed.StringVal
			default:
				return "", fmt.Errorf("unexpected global-filter %s encoding %T", leaf, typed)
			}
			if policy, ok := globalFilterPolicyValue(value, leaf); ok {
				return policy, nil
			}
		}
	}
	return "", fmt.Errorf("global-filter %s not found in Get response: %v", leaf, resp)
}

// globalFilterPolicyValue accepts either a leaf- or container-valued JSON_IETF Get.
func globalFilterPolicyValue(value any, leaf string) (string, bool) {
	switch value := value.(type) {
	case string:
		return value, true
	case map[string]any:
		for key, child := range value {
			if i := strings.LastIndexByte(key, ':'); i >= 0 {
				key = key[i+1:]
			}
			if key == leaf {
				policy, ok := child.(string)
				return policy, ok
			}
		}
		for _, child := range value {
			if nested, ok := child.(map[string]any); ok {
				if policy, ok := globalFilterPolicyValue(nested, leaf); ok {
					return policy, true
				}
			}
		}
	}
	return "", false
}

// deletePolicy deletes a policy-definition using the ygot-generated OC path.
func deletePolicy(t *testing.T, dut *ondatra.DUTDevice, policyName string) error {
	t.Helper()
	gnmi.Delete(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(policyName).Config())
	return nil
}

// deleteGlobalFilterAndPolicyAtomic deletes the global-filter and a
// policy-definition in a single atomic gNMI Set request.
func deleteGlobalFilterAndPolicyAtomic(t *testing.T, dut *ondatra.DUTDevice, niName, policyName string) error {
	t.Helper()
	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Log("Skipping atomic AFT global-filter + policy delete: unsupported on EOS")
			return nil
		}
	}

	client, err := dialGNMI(t, dut)
	if err != nil {
		return err
	}
	req := &gpb.SetRequest{
		Delete: []*gpb.Path{
			globalFilterPolicyPath(niName, "ipv4-policy"),
			globalFilterPolicyPath(niName, "ipv6-policy"),
			policyDefinitionPath(policyName),
		},
	}
	if _, err := client.Set(context.Background(), req); err != nil {
		return fmt.Errorf("atomic delete failed: %w", err)
	}
	return nil
}

// updatePolicyStatementPrefixSet replaces the prefix-set referenced by a
// policy statement's match-prefix-set condition.
func updatePolicyStatementPrefixSet(t *testing.T, dut *ondatra.DUTDevice, policyName, statementName,
	newPrefixSet string) error {
	t.Helper()
	client, err := dialGNMI(t, dut)
	if err != nil {
		return err
	}
	req := &gpb.SetRequest{Update: []*gpb.Update{
		{
			Path: statementPrefixSetConfigPath(policyName, statementName),
			Val:  policyStringValue(dut, newPrefixSet),
		},
	}}
	started := time.Now()
	t.Logf("Sending prefix-set reference gNMI SetRequest UTC=%s: %v", started.UTC().Format(time.RFC3339Nano), req)
	response, setErr := client.Set(context.Background(), req)
	t.Logf("Prefix-set reference Set end UTC=%s elapsed=%v status=%s error=%v response=%v", time.Now().UTC().Format(time.RFC3339Nano), time.Since(started), status.Code(setErr), setErr, response)
	if dut.Vendor() == ondatra.CISCO {
		saveAFTDiagnostic(t, "prefix-set leaf Update "+policyName, "gNMI Set", fmt.Sprintf("UTC: %s\nRequest: %v\nResponse: %v\nError: %v\n", started.UTC().Format(time.RFC3339Nano), req, response, setErr))
	}
	if setErr != nil {
		return fmt.Errorf("failed to update policy %s statement %s prefix-set to %s: %w",
			policyName, statementName, newPrefixSet, setErr)
	}
	return nil
}

// replacePolicyPrefixSet replaces AFT-6.1.4's complete single-statement policy.
// The policy remains attached, so the same AFT subscription can verify the
// resulting streamed deletions. This helper reconstructs the entire policy
// and must not be used for policies containing additional statements.
func replacePolicyPrefixSet(t *testing.T, dut *ondatra.DUTDevice, policyName, statementName,
	newPrefixSet string) error {
	t.Helper()
	client, err := dialGNMI(t, dut)
	if err != nil {
		return err
	}
	statement := map[string]any{
		"name":   statementName,
		"config": map[string]any{"name": statementName},
		"conditions": map[string]any{"match-prefix-set": map[string]any{
			"config": map[string]any{"prefix-set": newPrefixSet, "match-set-options": "ANY"},
		}},
		"actions": map[string]any{"config": map[string]any{"policy-result": "ACCEPT_ROUTE"}},
	}
	policyJSON, err := json.Marshal(map[string]any{
		"name":       policyName,
		"config":     map[string]any{"name": policyName},
		"statements": map[string]any{"statement": []any{statement}},
	})
	if err != nil {
		return fmt.Errorf("marshal replacement policy: %w", err)
	}
	req := &gpb.SetRequest{
		Prefix: &gpb.Path{Origin: "openconfig"},
		Replace: []*gpb.Update{{
			Path: policyDefinitionPath(policyName),
			Val:  &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: policyJSON}},
		}},
	}
	started := time.Now()
	t.Logf("Sending full-policy prefix-set replacement gNMI SetRequest UTC=%s: %v", started.UTC().Format(time.RFC3339Nano), req)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	response, setErr := client.Set(ctx, req)
	t.Logf("Full-policy prefix-set replacement Set end UTC=%s elapsed=%v status=%s error=%v response=%v", time.Now().UTC().Format(time.RFC3339Nano), time.Since(started), status.Code(setErr), setErr, response)
	if dut.Vendor() == ondatra.CISCO {
		saveAFTDiagnostic(t, "prefix-set policy Replace "+policyName, "gNMI Set", fmt.Sprintf("UTC: %s\nRequest: %v\nResponse: %v\nError: %v\n", started.UTC().Format(time.RFC3339Nano), req, response, setErr))
	}
	if setErr != nil {
		return fmt.Errorf("replace complete policy %s (statement %s) with prefix-set %s: %w",
			policyName, statementName, newPrefixSet, setErr)
	}
	return nil
}

// logConfigPath logs a time-bounded CONFIG readback without replacing the test's
// Set or streamed-AFT assertions. A failed Get is diagnostic, not a test result.
func logConfigPath(t *testing.T, dut *ondatra.DUTDevice, label string, path *gpb.Path) {
	t.Helper()
	client, err := dialGNMI(t, dut)
	if err != nil {
		t.Logf("%s CONFIG readback: dial failed: %v", label, err)
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	resp, err := client.Get(ctx, &gpb.GetRequest{
		Prefix:   &gpb.Path{Origin: "openconfig", Target: dut.Name()},
		Path:     []*gpb.Path{path},
		Type:     gpb.GetRequest_CONFIG,
		Encoding: aftpf.ConfigEncoding(dut),
	})
	if err != nil {
		t.Logf("%s CONFIG readback for %v failed: %v", label, path, err)
		return
	}
	t.Logf("%s CONFIG readback for %v: %v", label, path, resp)
}

func prefixSetConfigPath(name string) *gpb.Path {
	return &gpb.Path{Elem: []*gpb.PathElem{
		{Name: "routing-policy"},
		{Name: "defined-sets"},
		{Name: "prefix-sets"},
		{Name: "prefix-set", Key: map[string]string{"name": name}},
		{Name: "config"},
	}}
}

func logCiscoCommand(t *testing.T, dut *ondatra.DUTDevice, command string) {
	t.Helper()
	logAFTDiagnosticCLI(context.Background(), t, dut, command)
}

// probeRawFilteredPrefixes observes exact AFT prefixes directly from the DUT.
// It does not feed the test's validation cache, so it can distinguish a
// server-side filtering failure from a cache/decoder problem.
func probeRawFilteredPrefixes(t *testing.T, dut *ondatra.DUTDevice, niName string, prefixes []string) {
	t.Helper()
	client, err := dialGNMI(t, dut)
	if err != nil {
		t.Logf("Raw filtered-prefix probe dial failed: %v", err)
		return
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(75*time.Second))
	defer cancel()
	stream, err := client.Subscribe(ctx)
	if err != nil {
		t.Logf("Raw filtered-prefix probe Subscribe failed: %v", err)
		return
	}
	var subscriptions []*gpb.Subscription
	for _, prefix := range prefixes {
		path := &gpb.Path{Elem: []*gpb.PathElem{
			{Name: "ipv4-unicast"},
			{Name: "ipv4-entry", Key: map[string]string{"prefix": prefix}},
		}}
		subscriptions = append(subscriptions, &gpb.Subscription{Path: path, Mode: gpb.SubscriptionMode_ON_CHANGE})
	}
	req := &gpb.SubscribeRequest{Request: &gpb.SubscribeRequest_Subscribe{Subscribe: &gpb.SubscriptionList{
		Mode:     gpb.SubscriptionList_STREAM,
		Encoding: gpb.Encoding_PROTO,
		Prefix: &gpb.Path{Origin: "openconfig", Target: dut.Name(), Elem: []*gpb.PathElem{
			{Name: "network-instances"},
			{Name: "network-instance", Key: map[string]string{"name": niName}},
			{Name: "afts"},
		}},
		Subscription: subscriptions,
	}}}
	t.Logf("Raw filtered-prefix probe sending SubscribeRequest UTC=%s: %v", time.Now().UTC().Format(time.RFC3339Nano), req)
	if err := stream.Send(req); err != nil {
		t.Logf("Raw filtered-prefix probe Send failed: %v", err)
		return
	}
	updates, deletes, notifications := 0, 0, 0
	for {
		resp, err := stream.Recv()
		if err != nil {
			t.Logf("Raw filtered-prefix probe ended before SyncResponse: prefixes=%v notifications=%d updates=%d deletes=%d error=%v",
				prefixes, notifications, updates, deletes, err)
			return
		}
		if n := resp.GetUpdate(); n != nil {
			notifications++
			updates += len(n.GetUpdate())
			deletes += len(n.GetDelete())
			if notifications <= 12 {
				t.Logf("Raw filtered-prefix probe notification %d UTC=%s: %v",
					notifications, time.Now().UTC().Format(time.RFC3339Nano), n)
			}
		}
		if resp.GetSyncResponse() {
			t.Logf("Raw filtered-prefix probe SyncResponse UTC=%s: prefixes=%v notifications=%d updates=%d deletes=%d",
				time.Now().UTC().Format(time.RFC3339Nano), prefixes, notifications, updates, deletes)
			return
		}
	}
}

// collectAndVerify uses a shorter check interval for Cisco's multi-stage suite.
// The default two-minute interval spends much of the suite's timeout waiting
// between successive stages; 45 seconds also leaves room for the full-cache
// conversions after a filter is removed.
func collectAndVerify(t *testing.T, dut *ondatra.DUTDevice, cfg aftpf.RunCollectorParams,
	present, absent []string) *aftcache.AFTData {
	t.Helper()
	if dut.Vendor() == ondatra.CISCO {
		cfg.CheckInterval = aftCheckInterval
	}
	return aftpf.CollectAndVerify(t, dut, cfg, present, absent)
}

// rawAFTStream is a raw gNMI STREAM subscription to afts/global-filter/state,
// used to observe the delete notification emitted when the filter is removed.
type rawAFTStream struct {
	mu            sync.Mutex
	notifs        []*gpb.Notification
	syncResponses int
	receiveErr    error
}

// newRawAFTStream opens a raw STREAM ON_CHANGE subscription on
// afts/global-filter/state, reading notifications in the background.
// TODO: replace this raw gNMI subscription with the ygot-generated OC path
// once the afts/global-filter schema is supported in featureprofiles.
func newRawAFTStream(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, niName string) (*rawAFTStream, error) {
	t.Helper()
	client, err := dialGNMI(t, dut)
	if err != nil {
		return nil, err
	}
	sub, err := client.Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("subscribe() failed: %w", err)
	}
	req := &gpb.SubscribeRequest{Request: &gpb.SubscribeRequest_Subscribe{Subscribe: &gpb.SubscriptionList{
		Mode:     gpb.SubscriptionList_STREAM,
		Encoding: gpb.Encoding_PROTO,
		Prefix: &gpb.Path{
			Origin: "openconfig",
			Target: dut.Name(),
			Elem: []*gpb.PathElem{
				{Name: "network-instances"},
				{Name: "network-instance", Key: map[string]string{"name": niName}},
				{Name: "afts"},
			},
		},
		Subscription: []*gpb.Subscription{{
			Path: &gpb.Path{Elem: []*gpb.PathElem{{Name: "global-filter"}, {Name: "state"}}},
			Mode: gpb.SubscriptionMode_ON_CHANGE,
		}},
	}}}
	if err := sub.Send(req); err != nil {
		return nil, fmt.Errorf("sending subscribe request failed: %w", err)
	}
	rs := &rawAFTStream{}
	go func() {
		for {
			resp, err := sub.Recv()
			if err != nil {
				rs.mu.Lock()
				rs.receiveErr = err
				rs.mu.Unlock()
				return
			}
			rs.mu.Lock()
			if resp.GetSyncResponse() {
				rs.syncResponses++
			}
			if n := resp.GetUpdate(); n != nil {
				rs.notifs = append(rs.notifs, n)
			}
			rs.mu.Unlock()
		}
	}()
	return rs, nil
}

// logStateDeleteDiagnostics reports the notification paths seen by the raw
// filter-state stream. A parent-path delete may remove the policy leaf even
// when the exact leaf name is absent from the notification's delete path.
func (rs *rawAFTStream) logStateDeleteDiagnostics(t *testing.T, policyLeaf string) {
	t.Helper()
	rs.mu.Lock()
	notifs := append([]*gpb.Notification(nil), rs.notifs...)
	syncResponses, receiveErr := rs.syncResponses, rs.receiveErr
	rs.mu.Unlock()
	t.Logf("global-filter/state %s delete diagnostic: syncResponses=%d notifications=%d receiveErr=%v",
		policyLeaf, syncResponses, len(notifs), receiveErr)
	const maxUpdatePaths = 10
	updatePaths := 0
	deletePaths := 0
	for _, notification := range notifs {
		for _, path := range notification.GetDelete() {
			deletePaths++
			t.Logf("global-filter/state delete notification: timestamp=%d prefix=%v delete=%v",
				notification.GetTimestamp(), notification.GetPrefix(), path)
		}
		for _, update := range notification.GetUpdate() {
			updatePaths++
			if updatePaths <= maxUpdatePaths {
				t.Logf("global-filter/state update notification: timestamp=%d prefix=%v path=%v value=%v",
					notification.GetTimestamp(), notification.GetPrefix(), update.GetPath(), update.GetVal())
			}
		}
	}
	t.Logf("global-filter/state %s delete diagnostic: deletePaths=%d updatePaths=%d (logged first %d updates)",
		policyLeaf, deletePaths, updatePaths, maxUpdatePaths)
}

// awaitStateDelete blocks until a delete notification is observed for the
// given global-filter/state policy leaf, or until timeout.
func (rs *rawAFTStream) awaitStateDelete(timeout time.Duration, policyLeaf string) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if rs.hasStateDelete(policyLeaf) {
			return true
		}
		time.Sleep(time.Second)
	}
	return false
}

// hasStateDelete accepts either a leaf delete or a delete of its containing
// global-filter/state subtree. A delete elsewhere in global-filter does not
// establish that this state leaf was removed.
func (rs *rawAFTStream) hasStateDelete(policyLeaf string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	for _, n := range rs.notifs {
		for _, d := range n.GetDelete() {
			path := append(append([]*gpb.PathElem{}, n.GetPrefix().GetElem()...), d.GetElem()...)
			if len(path) < 2 {
				continue
			}
			for i := 0; i+1 < len(path); i++ {
				if path[i].GetName() != "global-filter" || path[i+1].GetName() != "state" {
					continue
				}
				if len(path) == i+2 || (len(path) == i+3 && path[i+2].GetName() == policyLeaf) {
					return true
				}
			}
		}
	}
	return false
}

// verifyNexthopGroupsAndNexthopsRetained confirms the next-hop-group and
// next-hops backing a prefix are still present in the filtered AFT.
func verifyNexthopGroupsAndNexthopsRetained(aft *aftcache.AFTData, prefix string) error {
	nhgID, ok := aft.Prefixes[prefix]
	if !ok {
		return fmt.Errorf("retained prefix %s missing from filtered AFT after dynamic delete", prefix)
	}
	nhg, ok := aft.NextHopGroups[nhgID]
	if !ok {
		return fmt.Errorf("next-hop-group %d for retained prefix %s was deleted, want retained", nhgID, prefix)
	}
	if len(nhg.NHIDs) == 0 {
		return fmt.Errorf("next-hop-group %d for retained prefix %s has no next-hops after dynamic delete", nhgID, prefix)
	}
	for _, nhID := range nhg.NHIDs {
		if _, ok := aft.NextHops[nhID]; !ok {
			return fmt.Errorf("next-hop %d shared by retained prefix %s was deleted, want retained", nhID, prefix)
		}
	}
	return nil
}

// runPrefixSetIteration runs a single AFT-6.1.1 iteration for one address
// family: validate the filtered view, dynamic updates, then filter removal.
func runPrefixSetIteration(t *testing.T, dut *ondatra.DUTDevice, tc prefixSetIterationCase, iterIdx int) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)

	if tc.extraRoute != "" {
		idx := fmt.Sprintf("%d", staticRouteIndex+500+iterIdx)
		addSingleStaticRoute(t, dut, ni, tc.extraRoute, idx, tc.nextHop)
	}

	v4Policy, v6Policy := "", ""
	if tc.ipv4 {
		v4Policy = tc.policyName
	} else {
		v6Policy = tc.policyName
	}
	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: v4Policy,
		V6Policy: v6Policy,
		VRFName:  ni,
	})

	rawStream, rawErr := newRawAFTStream(ctx, t, dut, ni)
	if rawErr != nil {
		t.Errorf("%s: failed to open raw AFT subscription: %v", tc.name, rawErr)
	}

	wantPrefixes := make(map[string]bool)
	for _, p := range tc.matchPrefixes {
		wantPrefixes[p] = true
	}

	t.Logf("%s - Initial Synchronization", tc.name)

	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	defer func() {
		if t.Failed() {
			t.Logf("%s AFT collector progress at failure: %+v", tc.name, collector.Progress())
		}
	}()
	hooks := []aftcache.NotificationHook{aftcache.VerifyAtomicFlagHook(t)}
	stop := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
		map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true})
	if dut.Vendor() == ondatra.CISCO {
		collector.ListenUntilPreUpdateHookWithInterval(context.Background(), t, subscriptionWait, aftCheckInterval, hooks, stop)
	} else {
		collector.ListenUntilPreUpdateHook(context.Background(), t, subscriptionWait, hooks, stop)
	}
	initialAFT, err := collector.ToAFT(t, dut)
	if err != nil {
		t.Errorf("%s: ToAFT failed: %v", tc.name, err)
	} else {
		aftpf.VerifyPrefixesPresent(t, aftpf.PrefixesParams{InfoAFT: initialAFT, Prefixes: tc.matchPrefixes})
		aftpf.VerifyPrefixesAbsent(t, aftpf.PrefixesParams{InfoAFT: initialAFT, Prefixes: []string{tc.nonMatchPrefix}})
	}

	if tc.dynamicAddPrefix != "" {
		t.Logf("%s - Validate Dynamic Updates", tc.name)

		dynV4NH, dynV6NH := map[string]bool{}, map[string]bool{}
		if tc.ipv4 {
			dynV4NH = map[string]bool{atePort1.IPv4: true}
		} else {
			dynV6NH = map[string]bool{atePort1.IPv6: true}
		}

		dynIdx := fmt.Sprintf("%d", staticRouteIndex+600+iterIdx)
		addSingleStaticRoute(t, dut, ni, tc.dynamicAddPrefix, dynIdx, tc.nextHop)

		// Keep watching the same streaming session so the dynamic add is
		// observed as an incremental notification on the existing stream.
		collectAndVerify(t, dut, aftpf.RunCollectorParams{
			Ctx:       context.Background(),
			Collector: collector,
			Stop: aftcache.InitialSyncStoppingCondition(t, dut, map[string]bool{tc.dynamicAddPrefix: true},
				dynV4NH, dynV6NH),
			Timeout: subscriptionWait,
		}, []string{tc.dynamicAddPrefix}, nil)

		removeStaticRoute(t, dut, ni, tc.dynamicAddPrefix)

		// Same session again: the delete must be seen as a streamed
		// deletion notification, not by re-subscribing after the fact.
		delAFT := collectAndVerify(t, dut, aftpf.RunCollectorParams{
			Ctx:       context.Background(),
			Collector: collector,
			Stop:      aftcache.DeletionStoppingCondition(t, dut, map[string]bool{tc.dynamicAddPrefix: true}),
			Timeout:   subscriptionWait,
		}, nil, []string{tc.dynamicAddPrefix})
		if delAFT != nil && len(tc.matchPrefixes) > 0 {
			if err := verifyNexthopGroupsAndNexthopsRetained(delAFT, tc.matchPrefixes[0]); err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
		}

		if tc.dynamicNonMatchPrefix != "" {
			nmIdx := fmt.Sprintf("%d", staticRouteIndex+650+iterIdx)
			addSingleStaticRoute(t, dut, ni, tc.dynamicNonMatchPrefix, nmIdx, tc.nextHop)

			// Drain the existing stream and confirm the non-matching prefix
			// never surfaces; it must stay absent from the filtered view.
			collectAndVerify(t, dut, aftpf.RunCollectorParams{
				Ctx:       context.Background(),
				Collector: collector,
				Stop:      aftcache.DeletionStoppingCondition(t, dut, map[string]bool{tc.dynamicNonMatchPrefix: true}),
				Timeout:   subscriptionWait,
			}, nil, []string{tc.dynamicNonMatchPrefix})

			removeStaticRoute(t, dut, ni, tc.dynamicNonMatchPrefix)
		}
	}

	t.Logf("%s - Remove the Filtered View", tc.name)

	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("%s: failed to delete global-filter: %v", tc.name, err)
		return
	}

	if rawStream != nil {
		wantDeleteLeaf := "ipv4-policy"
		if !tc.ipv4 {
			wantDeleteLeaf = "ipv6-policy"
		}
		if !rawStream.awaitStateDelete(subscriptionWait, wantDeleteLeaf) {
			rawStream.logStateDeleteDiagnostics(t, wantDeleteLeaf)
			t.Errorf("%s: did not receive global-filter/state/%s delete notification after removing the filter",
				tc.name, wantDeleteLeaf)
		}
	}

	liftWant := map[string]bool{tc.nonMatchPrefix: true}
	// The filter removal is validated on the same stream: the previously
	// filtered-out prefix must now appear as a streamed update.
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, liftWant,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{tc.nonMatchPrefix}, nil)

	if tc.extraRoute != "" {
		removeStaticRoute(t, dut, ni, tc.extraRoute)
	}
}

// prefixSetIterationCase parameterizes a single AFT-6.1.1 iteration.
type prefixSetIterationCase struct {
	name                  string
	policyName            string
	ipv4                  bool
	nextHop               string
	matchPrefixes         []string
	nonMatchPrefix        string
	extraRoute            string // installed before subscribing, if not already part of the base set
	dynamicAddPrefix      string // added mid-subscription to validate dynamic updates
	dynamicNonMatchPrefix string // added mid-subscription; must never surface
}

// testPrefixSetPolicySubscription implements AFT-6.1.1, iterated across
// address families and prefix-set/subnet policies.
func testPrefixSetPolicySubscription(t *testing.T, dut *ondatra.DUTDevice) {
	cases := []prefixSetIterationCase{
		{
			name:                  "IPv4-POLICY-PREFIX-SET-A",
			policyName:            policyPfxSetA,
			ipv4:                  true,
			nextHop:               atePort1.IPv4,
			matchPrefixes:         []string{"198.51.100.0/24", "203.0.113.0/28"},
			nonMatchPrefix:        "100.64.0.0/24",
			dynamicAddPrefix:      "198.51.100.1/32",
			dynamicNonMatchPrefix: "100.64.1.0/24",
		},
		{
			name:                  "IPv6-POLICY-PREFIX-SET-B",
			policyName:            policyPfxSetB,
			ipv4:                  false,
			nextHop:               atePort1.IPv6,
			matchPrefixes:         []string{"2001:db8:2::/64"},
			nonMatchPrefix:        "2001:db8:1::/64",
			dynamicAddPrefix:      "2001:db8:2::1/128",
			dynamicNonMatchPrefix: "2001:db8:4::/64",
		},
		{
			name:           "IPv4-POLICY-SUBNET",
			policyName:     policySubnet,
			ipv4:           true,
			nextHop:        atePort1.IPv4,
			matchPrefixes:  []string{"203.0.113.0/28"},
			nonMatchPrefix: "100.64.0.0/24",
		},
		{
			name:           "IPv6-POLICY-SUBNET-V6",
			policyName:     policySubnetV6,
			ipv4:           false,
			nextHop:        atePort1.IPv6,
			matchPrefixes:  []string{"2001:db8:3::/65"},
			nonMatchPrefix: "2001:db8:1::/64",
			extraRoute:     "2001:db8:3::/65",
		},
	}

	for i, tc := range cases {
		if dut.Vendor() != ondatra.CISCO {
			switch tc.policyName {
			case policyPfxSetA:
				tc.matchPrefixes = append(tc.matchPrefixes, defaultV4Route)
			case policyPfxSetB:
				tc.matchPrefixes = append(tc.matchPrefixes, defaultV6Route)
			}
		}
		t.Run(tc.name, func(t *testing.T) {
			runPrefixSetIteration(t, dut, tc, i)
		})
	}
}

// testNonExistentPolicy implements AFT-6.1.2.
func testNonExistentPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.AftsGlobalFilterPolicyConfigReferenceValidationUnsupported(dut) {
		t.Skip("AFT global-filter policy reference validation is unsupported")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)
	matchPrefix := "198.51.100.128/25"

	wantSetCode := codes.FailedPrecondition
	if dut.Vendor() == ondatra.CISCO {
		// Cisco permits a global-filter reference before its routing-policy
		// definition exists. Verify that both references were stored, then
		// create the policy and continue testing the filtered AFT view.
		wantSetCode = codes.OK
	}
	if err := setGlobalFilterExpectCode(t, dut, ni, policyNotYetExist, policyNotYetExist,
		wantSetCode); err != nil {
		t.Errorf("Non-existent policy check: %v", err)
	}
	if dut.Vendor() == ondatra.CISCO {
		for _, leaf := range []string{"ipv4-policy", "ipv6-policy"} {
			if got, err := readGlobalFilterPolicy(t, dut, ni, leaf); err != nil || got != policyNotYetExist {
				t.Fatalf("Non-existent policy check: global-filter %s got %q, want %q (err: %v)",
					leaf, got, policyNotYetExist, err)
			}
		}
	}

	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
		PolicyName:     policyNotYetExist,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{pfxSetNotYetExist},
		PrefixList:     []string{matchPrefix},
		PrefixMode:     pfxMode,
		MatchPrefixSet: true,
		MatchSetOption: oc.E_RoutingPolicy_MatchSetOptionsRestrictedType(oc.RoutingPolicy_MatchSetOptionsType_ANY),
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})
	batch := &gnmi.SetBatch{}
	gnmi.BatchUpdate(batch, gnmi.OC().RoutingPolicy().Config(), rp)
	batch.Set(t, dut)

	mpIdx := fmt.Sprintf("%d", staticRouteIndex+700)
	addSingleStaticRoute(t, dut, ni, matchPrefix, mpIdx, atePort1.IPv4)

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyNotYetExist,
		V6Policy: "",
		VRFName:  ni,
	})

	wantPrefixes := map[string]bool{matchPrefix: true}
	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{matchPrefix}, []string{"198.51.100.0/24", "100.64.0.0/24"})

	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Cleanup: failed to delete global-filter: %v", err)
	}
	removeStaticRoute(t, dut, ni, matchPrefix)
	if err := deletePolicy(t, dut, policyNotYetExist); err != nil {
		t.Errorf("Cleanup: failed to delete policy %s: %v", policyNotYetExist, err)
	}
}

// testPolicyDeletion implements AFT-6.1.3.
func testPolicyDeletion(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.AftsGlobalFilterPolicyConfigReferenceValidationUnsupported(dut) {
		t.Skip("AFT global-filter policy reference validation is unsupported")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)
	wantPrefixes := map[string]bool{"198.51.100.0/24": true, "203.0.113.0/28": true}
	liftWant := map[string]bool{"100.64.0.0/24": true}
	filteredOut := map[string]bool{"100.64.0.0/24": true}
	root := &oc.Root{}
	rp := root.GetOrCreateRoutingPolicy()
	aftpf.AddPrefixSetPolicy(t, rp, aftpf.PrefixSetPolicyParams{
		PolicyName:     policyPfxSetA,
		StatementNames: []string{defaultStatementName},
		PrefixSetNames: []string{v4PfxSetA},
		PrefixList:     policyPrefixSetMembers(dut, true),
		PrefixMode:     pfxMode,
		MatchPrefixSet: true,
		MatchSetOption: oc.E_RoutingPolicy_MatchSetOptionsRestrictedType(oc.RoutingPolicy_MatchSetOptionsType_ANY),
		PolicyResult:   oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	})

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyPfxSetA,
		V6Policy: "",
		VRFName:  ni,
	})

	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, nil, nil)

	wantDeleteCode := codes.FailedPrecondition
	if dut.Vendor() == ondatra.CISCO {
		// Cisco's configuration policy allows deletion of a routing-policy
		// definition even while the AFT global filter references it. The
		// subsequent atomic-delete check still needs that definition and
		// reference, so restore and verify them after this successful delete.
		wantDeleteCode = codes.OK
		if got, err := readGlobalFilterPolicy(t, dut, ni, "ipv4-policy"); err != nil || got != policyPfxSetA {
			t.Fatalf("Policy-still-referenced check: IPv4 filter before delete got %q, want %q (err: %v)", got, policyPfxSetA, err)
		}
		if !gnmi.Lookup(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(policyPfxSetA).Config()).IsPresent() {
			t.Fatalf("Policy-still-referenced check: policy %q was absent before delete", policyPfxSetA)
		}
	}
	if err := deletePolicyExpectCode(t, dut, policyPfxSetA, wantDeleteCode); err != nil {
		t.Fatalf("Policy-still-referenced check: %v", err)
	}
	if dut.Vendor() == ondatra.CISCO {
		if gnmi.Lookup(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(policyPfxSetA).Config()).IsPresent() {
			t.Fatalf("Policy-still-referenced check: policy %q still exists after successful delete", policyPfxSetA)
		}
		restoreBatch := &gnmi.SetBatch{}
		gnmi.BatchUpdate(restoreBatch, gnmi.OC().RoutingPolicy().Config(), rp)
		restoreBatch.Set(t, dut)
		aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
			V4Policy: policyPfxSetA,
			VRFName:  ni,
		})
		if !gnmi.Lookup(t, dut, gnmi.OC().RoutingPolicy().PolicyDefinition(policyPfxSetA).Config()).IsPresent() {
			t.Fatalf("Policy-still-referenced check: policy %q was not restored", policyPfxSetA)
		}
		if got, err := readGlobalFilterPolicy(t, dut, ni, "ipv4-policy"); err != nil || got != policyPfxSetA {
			t.Fatalf("Policy-still-referenced check: restored IPv4 filter got %q, want %q (err: %v)", got, policyPfxSetA, err)
		}
		collectAndVerify(t, dut, aftpf.RunCollectorParams{
			Ctx:       context.Background(),
			Collector: collector,
			// The wanted routes can already be present while the collector is
			// still processing the deletion of the formerly unfiltered route.
			Stop: aftcache.InitialSyncStoppingConditionWithAbsentPrefixes(t, dut, wantPrefixes, filteredOut,
				map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
			Timeout: subscriptionWait,
		}, []string{"198.51.100.0/24", "203.0.113.0/28"}, []string{"100.64.0.0/24"})
	}

	if err := deleteGlobalFilterAndPolicyAtomic(t, dut, ni, policyPfxSetA); err != nil {
		t.Errorf("Atomic delete failed: %v", err)
		return
	}

	// Same session observes the atomic filter+policy delete as a streamed change.
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, liftWant,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{"100.64.0.0/24"}, nil)

	batch := &gnmi.SetBatch{}
	gnmi.BatchUpdate(batch, gnmi.OC().RoutingPolicy().Config(), rp)
	batch.Set(t, dut)
	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyPfxSetA,
		V6Policy: "",
		VRFName:  ni,
	})

	// Same session observes the policy re-configuration as streamed updates.
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingConditionWithAbsentPrefixes(t, dut, wantPrefixes, filteredOut,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{"198.51.100.0/24", "203.0.113.0/28"}, []string{"100.64.0.0/24"})

	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Multi-step delete: failed to delete global-filter: %v", err)
	}
	if err := deletePolicy(t, dut, policyPfxSetA); err != nil {
		t.Errorf("Multi-step delete: failed to delete policy: %v", err)
	}

	// Same session observes the multi-step filter+policy delete as streamed deletions.
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, liftWant,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{"100.64.0.0/24"}, nil)

	batch2 := &gnmi.SetBatch{}
	gnmi.BatchUpdate(batch2, gnmi.OC().RoutingPolicy().Config(), rp)
	batch2.Set(t, dut)
}

// testChangeReferencedPrefixSet implements AFT-6.1.4.
func testChangeReferencedPrefixSet(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyPfxSetA,
		V6Policy: "",
		VRFName:  ni,
	})

	wantPrefixes := map[string]bool{"198.51.100.0/24": true, "203.0.113.0/28": true}
	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	diagnostics := aftFilterDiagnostics{
		networkInstance: ni,
		policy:          policyPfxSetA,
		prefixSets:      []string{v4PfxSetA, v4PfxSetB},
		prefixes:        []string{"198.51.100.0/24", "203.0.113.0/28"},
	}
	stage := "AFT-6.1.4 initial subscription"
	failureCaptured := false
	captureFailure := func() {
		if !failureCaptured && t.Failed() {
			failureCaptured = true
			diagnostics.captureFailure(t, dut, stage, cancel, collector)
		}
	}
	defer captureFailure()
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{"198.51.100.0/24", "203.0.113.0/28"}, nil)
	diagnostics.capture(t, dut, "AFT-6.1.4 before prefix-set swap")
	stage = "AFT-6.1.4 prefix-set swap"

	newPrefixSet := v6PfxSetB
	setPrefixSetReference := updatePolicyStatementPrefixSet
	if dut.Vendor() == ondatra.CISCO {
		// Cisco-specific XR limitation: individual fields within the native
		// route-policy bag cannot be set independently. Replace the complete
		// OpenConfig policy definition so the mapper can replace the whole
		// bag, for both the policy change and cleanup. This test policy has
		// only statement 10, which is included in full in each replacement.
		// An uninstalled IPv4 set keeps the active IPv4 policy in-family.
		newPrefixSet = v4PfxSetB
		setPrefixSetReference = replacePolicyPrefixSet
	}
	referencePath := statementPrefixSetConfigPath(policyPfxSetA, defaultStatementName)
	logConfigPath(t, dut, "before prefix-set swap", referencePath)
	logConfigPath(t, dut, "replacement prefix-set", prefixSetConfigPath(newPrefixSet))
	if err := setPrefixSetReference(t, dut, policyPfxSetA, defaultStatementName, newPrefixSet); err != nil {
		diagnostics.capturePolicyOperation(t, dut, "AFT-6.1.4 prefix-set swap rejected", v4PfxSetA)
		logConfigPath(t, dut, "after failed prefix-set swap", referencePath)
		logCiscoCommand(t, dut, "show running-config route-policy "+policyPfxSetA)
		logCiscoCommand(t, dut, "show logging")
		t.Errorf("Failed to swap prefix-set reference: %v", err)
		return
	}
	diagnostics.capturePolicyOperation(t, dut, "AFT-6.1.4 accepted prefix-set swap", newPrefixSet)
	logConfigPath(t, dut, "after prefix-set swap", referencePath)
	diagnostics.capture(t, dut, "AFT-6.1.4 after prefix-set swap")
	stage = "AFT-6.1.4 waiting for prefix deletions after swap"

	// Same session observes the prefix-set swap as streamed deletions.
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop:      aftcache.DeletionStoppingCondition(t, dut, map[string]bool{"198.51.100.0/24": true}),
		Timeout:   subscriptionWait,
	}, nil, []string{"198.51.100.0/24", "203.0.113.0/28"})
	stage = "AFT-6.1.4 prefix deletion stability check"

	// The swap only produces delete notifications for PREFIX-SET-A's members;
	// PREFIX-SET-B's members were never installed in the AFT, so there is no
	// further positive event to wait on. Wait out a full notification window
	// and re-check the snapshot to catch any spurious re-add that a bug might
	// otherwise introduce after the deletion settles.
	time.Sleep(notificationWaitTime)
	finalAFT, err := collector.ToAFT(t, dut)
	if err != nil {
		t.Errorf("ToAFT failed on stability check: %v", err)
	} else {
		aftpf.VerifyPrefixesAbsent(t, aftpf.PrefixesParams{
			InfoAFT: finalAFT, Prefixes: []string{"198.51.100.0/24", "203.0.113.0/28"},
		})
	}

	// Preserve a nonfatal ToAFT error before cleanup restores the policy.
	captureFailure()
	stage = "AFT-6.1.4 cleanup"
	if err := setPrefixSetReference(t, dut, policyPfxSetA, defaultStatementName, v4PfxSetA); err != nil {
		t.Errorf("Cleanup: failed to restore prefix-set reference: %v", err)
	}
	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Cleanup: failed to delete global-filter: %v", err)
	}
}

// testMultiStatementPolicy implements AFT-6.1.5.
func testMultiStatementPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)
	extraPrefix := "203.0.113.128/25"

	extraIdx := fmt.Sprintf("%d", staticRouteIndex+800)
	addSingleStaticRoute(t, dut, ni, extraPrefix, extraIdx, atePort1.IPv4)

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyMultiStmt,
		V6Policy: "",
		VRFName:  ni,
	})

	wantPrefixes := map[string]bool{"198.51.100.0/24": true, "203.0.113.0/28": true, extraPrefix: true}
	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{"198.51.100.0/24", "203.0.113.0/28", extraPrefix}, []string{"100.64.0.0/24"})

	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Cleanup: failed to delete global-filter: %v", err)
	}
	removeStaticRoute(t, dut, ni, extraPrefix)
}

// testDenyActionPolicy implements AFT-6.1.6.
func testDenyActionPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyDenyPfxSetA,
		V6Policy: "",
		VRFName:  ni,
	})

	wantPrefixes := map[string]bool{"100.64.0.0/24": true}
	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop: aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, []string{"100.64.0.0/24"}, []string{"198.51.100.0/24", "203.0.113.0/28"})

	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Cleanup: failed to delete global-filter: %v", err)
	}
}

// testNonPrefixSetMatchCriteria implements AFT-6.1.7. Cisco uses an
// unconditional drop policy to cover an empty policy view and live transitions;
// this substitution does not exercise tag matching.
func testNonPrefixSetMatchCriteria(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ni := deviations.DefaultNetworkInstance(dut)
	policyName, policyDescription := policyTagMatch, "tag-match"
	if dut.Vendor() == ondatra.CISCO {
		policyName, policyDescription = policyDropAll, "drop-all"
		t.Log("Cisco AFT-6.1.7 uses an unconditional drop policy for empty-view and live-transition coverage; tag matching is unsupported")
	}

	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyName,
		V6Policy: "",
		VRFName:  ni,
	})
	diagnostics := aftFilterDiagnostics{
		networkInstance: ni,
		policy:          policyName,
		prefixes:        []string{"198.51.100.0/24", "203.0.113.0/28", "100.64.0.0/24"},
	}
	diagnostics.capture(t, dut, "AFT-6.1.7 "+policyDescription+" policy attached")
	if dut.Vendor() == ondatra.CISCO {
		// Record the applied policy and test routes if drop-all does not
		// exclude them. Required next-hop resolution routes may remain visible.
		if got, err := readGlobalFilterPolicy(t, dut, ni, "ipv4-policy"); err != nil {
			t.Logf("%s IPv4 global-filter CONFIG readback failed: %v", policyDescription, err)
		} else {
			t.Logf("%s IPv4 global-filter CONFIG readback: %q", policyDescription, got)
		}
		logConfigPath(t, dut, policyDescription+" policy definition", policyDefinitionPath(policyName))
		logCiscoCommand(t, dut, "show running-config route-policy "+policyName)
		for _, prefix := range []string{"198.51.100.0/24", "203.0.113.0/28", "100.64.0.0/24"} {
			logCiscoCommand(t, dut, "show route "+prefix+" detail")
		}
	}

	collector := aftcache.NewAFTStreamSession(ctx, t, aftpf.GnmiClientSession(t, dut, aftpf.PrefixesParams{Ctx: ctx}), dut)
	stage := "AFT-6.1.7 initial " + policyDescription + " subscription"
	defer func() {
		diagnostics.captureFailure(t, dut, stage, cancel, collector)
	}()
	absentPrefixes := map[string]bool{"198.51.100.0/24": true, "203.0.113.0/28": true, "100.64.0.0/24": true}
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		// Wait for the initial SyncResponse and for all forbidden prefixes
		// to disappear; an empty desired-prefix set alone stops too early.
		Stop: aftcache.InitialSyncStoppingConditionWithAbsentPrefixes(t, dut, nil, absentPrefixes,
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true}),
		Timeout: subscriptionWait,
	}, nil, []string{"198.51.100.0/24", "203.0.113.0/28", "100.64.0.0/24"})
	stage = "AFT-6.1.7 transition to prefix-set policy"

	if dut.Vendor() != ondatra.CISCO {
		if err := deleteGlobalFilter(t, dut, ni); err != nil {
			t.Errorf("Failed to delete global-filter: %v", err)
			return
		}
	}

	// Replace Cisco's policy reference directly, preserving the same stream
	// and avoiding an unfiltered interval between drop-all and prefix filtering.
	diagnostics.policy = policyPfxSetA
	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyPfxSetA,
		V6Policy: "",
		VRFName:  ni,
	})

	// Same session observes the transition to the prefix-set policy as streamed updates.
	wantPrefixes := map[string]bool{"198.51.100.0/24": true, "203.0.113.0/28": true}
	stop := aftcache.InitialSyncStoppingCondition(t, dut, wantPrefixes,
		map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true})
	var absent []string
	if dut.Vendor() == ondatra.CISCO {
		absent = []string{"100.64.0.0/24"}
		stop = aftcache.InitialSyncStoppingConditionWithAbsentPrefixes(t, dut, wantPrefixes,
			map[string]bool{"100.64.0.0/24": true},
			map[string]bool{atePort1.IPv4: true}, map[string]bool{atePort1.IPv6: true})
	}
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop:      stop,
		Timeout:   subscriptionWait,
	}, nil, absent)

	diagnostics.policy = policyName
	aftpf.ConfigureGlobalFilterPolicies(t, dut, aftpf.ConfigureGlobalFilterPoliciesParams{
		V4Policy: policyName,
		V6Policy: "",
		VRFName:  ni,
	})
	diagnostics.capture(t, dut, "AFT-6.1.7 transition back to "+policyDescription+" policy")
	stage = "AFT-6.1.7 waiting for deletions after " + policyDescription + " transition"

	// Same session observes the transition back as streamed deletions.
	deletePrefixes := map[string]bool{"198.51.100.0/24": true}
	absentAfterTransition := []string{"198.51.100.0/24", "203.0.113.0/28"}
	if dut.Vendor() == ondatra.CISCO {
		deletePrefixes = absentPrefixes
		absentAfterTransition = append(absentAfterTransition, "100.64.0.0/24")
	}
	collectAndVerify(t, dut, aftpf.RunCollectorParams{
		Ctx:       context.Background(),
		Collector: collector,
		Stop:      aftcache.DeletionStoppingCondition(t, dut, deletePrefixes),
		Timeout:   subscriptionWait,
	}, nil, absentAfterTransition)

	stage = "AFT-6.1.7 cleanup"
	if err := deleteGlobalFilter(t, dut, ni); err != nil {
		t.Errorf("Cleanup: failed to delete global-filter: %v", err)
	}
}

// TestMain runs featureprofile tests.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestAFTPrefixFiltering implements AFT-6.1.
func TestAFTPrefixFiltering(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	if deviations.AftsGlobalFilterPolicyOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			t.Skipf("Skipping AFT-6.1 test validation: AFT global-filter policy is not supported on %s", dut.Vendor())
		}
	}

	ate := ondatra.ATE(t, "ate")
	ni := deviations.DefaultNetworkInstance(dut)

	if _, err := aftpf.DialGNMI(t, dut); err != nil {
		t.Fatalf("%v", err)
	}

	batch := aftpf.ConfigureDUT(t, dut)
	configurePolicies(t, dut, batch)
	prefixes := aftpf.ConfigureBaseRoutesParams{V4Prefixes: baseIPv4Prefixes, V6Prefixes: baseIPv6Prefixes}
	aftpf.ConfigureBaseRoutes(t, dut, batch, prefixes)
	d := &oc.Root{}
	defNI := d.GetOrCreateNetworkInstance(ni)
	aftpf.ConfigureBGP(t, dut, batch, defNI)
	batch.Set(t, dut)
	aftpf.ApplyBGPMaxPrefixes(t, dut, aftpf.BGPPrefixParams{V4Prefix: aftpf.ATEPort1.IPv4,
		V6Prefix: aftpf.ATEPort2.IPv6, NetworkInstance: defNI})
	topo, interfaceNamesList := aftpf.ConfigureATE(t, ate)
	aftpf.ConfigureATEBGP(t, topo)
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	cfgplugins.IsIPv4InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})
	cfgplugins.IsIPv6InterfaceARPresolved(t, ate, cfgplugins.AddressFamilyParams{InterfaceNames: interfaceNamesList})

	aftpf.AwaitBGPConvergence(t, dut, ni)

	if dut.Vendor() != ondatra.CISCO {
		aftpf.CheckForDefaultRoutes(t, &aftpf.CheckForDefaultRoutesParams{
			DUT: dut, NetworkInstance: ni,
			IPv4Route: defaultV4Route, IPv6Route: defaultV6Route,
		})
	}

	tests := []struct {
		name string
		test func(t *testing.T, dut *ondatra.DUTDevice)
	}{
		{
			name: "aft-6.1.1-testPrefixSetPolicySubscription",
			test: testPrefixSetPolicySubscription,
		},
		{
			name: "aft-6.1.2-testNonExistentPolicy",
			test: testNonExistentPolicy,
		},
		{
			name: "aft-6.1.3-testPolicyDeletion",
			test: testPolicyDeletion,
		},
		{
			name: "aft-6.1.4-testChangeReferencedPrefixSet",
			test: testChangeReferencedPrefixSet,
		},
		{
			name: "aft-6.1.5-testMultiStatementPolicy",
			test: testMultiStatementPolicy,
		},
		{
			name: "aft-6.1.6-testDenyActionPolicy",
			test: testDenyActionPolicy,
		},
		{
			name: "aft-6.1.7-testNonPrefixSetMatchCriteria",
			test: testNonPrefixSetMatchCriteria,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.test(t, dut)
		})
	}
}
