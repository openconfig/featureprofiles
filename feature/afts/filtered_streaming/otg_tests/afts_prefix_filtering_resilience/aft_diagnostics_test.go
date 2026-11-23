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

package afts_prefix_filtering_resilience_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	aftpf "github.com/openconfig/featureprofiles/internal/aft_prefix_filtering"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/telemetry/aftcache"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/testt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// This client belongs to this test run, not aftpf's package-wide sync.Once.
// The parent context outlives the reboot subtest and later policy subtests.
type resilienceFilterClient struct {
	ctx       context.Context
	client    gpb.GNMIClient
	createdAt time.Time
	// Dependent cases require both gNMI recovery and BGP route readiness.
	readyForDependentTests bool
}

func (c *resilienceFilterClient) connect(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if c.client != nil {
		t.Fatal("AFT filter client must be retired before reconnecting")
	}
	ctx, cancel := context.WithDeadline(c.ctx, time.Now().Add(20*time.Second))
	defer cancel()
	client, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx, grpc.WithBlock())
	if err != nil {
		t.Fatalf("Failed to connect test-owned AFT filter client: %v", err)
	}
	c.client, c.createdAt = client, time.Now()
	t.Logf("AFT test-owned filter client connected UTC=%s client=%p", c.createdAt.UTC().Format(time.RFC3339Nano), c.client)
}

func (c *resilienceFilterClient) close(t *testing.T) {
	t.Helper()
	c.readyForDependentTests = false
	if c.client == nil {
		return
	}
	if closer, ok := c.client.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			t.Logf("AFT filter client close failed: %v", err)
		}
	} else {
		// Binding clients commonly hide their grpc.ClientConn. Do not claim
		// to close that transport, or reset shared state used by other callers.
		t.Logf("AFT filter client %p exposes no io.Closer; retiring it from all subsequent filter Sets", c.client)
	}
	c.client = nil
	c.createdAt = time.Time{}
}

// Diagnostic commands and reads never replace the original failure, retry a
// failed Set, or supply values to the subscription cache. Independent deadlines
// allow evidence to be collected even after a collector's context expires.
func logAFTDiagnosticCLI(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, command string) {
	t.Helper()
	if dut.Vendor() != ondatra.CISCO {
		return
	}
	if failure := testt.CaptureFatal(t, func(tb testing.TB) {
		readCtx, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
		defer cancel()
		started := time.Now()
		t.Logf("AFT diagnostic CLI start UTC=%s command=%q", started.UTC().Format(time.RFC3339Nano), command)
		response, err := dut.RawAPIs().CLI(tb).RunCommand(readCtx, command)
		if err != nil {
			t.Logf("AFT diagnostic CLI command=%q elapsed=%v failed: %v", command, time.Since(started), err)
			return
		}
		output := response.Output()
		// Traces can be large. Preserve the newest evidence and report the
		// truncation rather than flooding the console with entire trace rings.
		const maxCLIBytes = 64 * 1024
		if len(output) > maxCLIBytes {
			t.Logf("AFT diagnostic CLI command=%q output=%d bytes; logging last %d bytes", command, len(output), maxCLIBytes)
			output = output[len(output)-maxCLIBytes:]
		}
		t.Logf("AFT diagnostic CLI end UTC=%s command=%q elapsed=%v:\n%s", time.Now().UTC().Format(time.RFC3339Nano), command, time.Since(started), output)
	}); failure != nil {
		t.Logf("AFT diagnostic CLI command=%q unavailable: %s", command, *failure)
	}
}

func newAFTDiagnosticClient(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) (gpb.GNMIClient, func()) {
	t.Helper()
	var client gpb.GNMIClient
	if failure := testt.CaptureFatal(t, func(tb testing.TB) {
		dialCtx, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
		defer cancel()
		var err error
		client, err = dut.RawAPIs().BindingDUT().DialGNMI(dialCtx)
		t.Logf("AFT diagnostic new DialGNMI UTC=%s client=%p error=%v (bypasses the shared filter client)", time.Now().UTC().Format(time.RFC3339Nano), client, err)
		if err != nil {
			client = nil
		}
	}); failure != nil {
		t.Logf("AFT diagnostic DialGNMI unavailable: %s", *failure)
	}
	return client, func() {
		if closer, ok := client.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				t.Logf("AFT diagnostic client close failed: %v", err)
			}
		} else if client != nil {
			t.Log("AFT diagnostic gNMI client does not expose io.Closer")
		}
	}
}

func logAFTDiagnosticGet(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client gpb.GNMIClient, path string) *gpb.GetResponse {
	t.Helper()
	return logAFTDiagnosticGetType(ctx, t, dut, client, path, gpb.GetRequest_ALL)
}

func logAFTDiagnosticGetType(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client gpb.GNMIClient, path string, dataType gpb.GetRequest_DataType) *gpb.GetResponse {
	t.Helper()
	if client == nil {
		t.Logf("AFT diagnostic Get skipped: no client for %s", path)
		return nil
	}
	readCtx, cancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
	defer cancel()
	request := &gpb.GetRequest{
		Prefix: &gpb.Path{Origin: "openconfig", Target: dut.Name()},
		Path:   []*gpb.Path{gnmiPath(t, path)}, Type: dataType, Encoding: aftpf.ConfigEncoding(dut),
	}
	started := time.Now()
	response, err := client.Get(readCtx, request)
	t.Logf("AFT diagnostic JSON_IETF Get UTC=%s path=%s elapsed=%v status=%s error=%v request=%v response=%v",
		time.Now().UTC().Format(time.RFC3339Nano), path, time.Since(started), status.Code(err), err, request, response)
	return response
}

func configureGlobalFilterPoliciesWithDiagnostics(t *testing.T, dut *ondatra.DUTDevice, filterClient *resilienceFilterClient, cfg aftpf.ConfigureGlobalFilterPoliciesParams) {
	t.Helper()
	if filterClient.client == nil {
		t.Fatal("No test-owned AFT filter client: DUT recovery/reconnection did not complete; refusing the cached pre-reboot client")
	}
	cfg.Client, cfg.ClientCreatedAt = filterClient.client, filterClient.createdAt
	cfg.OnSetFailure = func(setErr error) {
		ctx, cancel := context.WithDeadline(context.WithoutCancel(t.Context()), time.Now().Add(45*time.Second))
		defer cancel()
		t.Logf("AFT filter Set failure diagnostics UTC=%s NI=%s expected IPv4=%q IPv6=%q original error=%v; no Set retry",
			time.Now().UTC().Format(time.RFC3339Nano), cfg.VRFName, cfg.V4Policy, cfg.V6Policy, setErr)
		logAFTDiagnosticCLI(ctx, t, dut, "show logging")
		client, closeClient := newAFTDiagnosticClient(ctx, t, dut)
		defer closeClient()
		logAFTDiagnosticGet(ctx, t, dut, client, "/system/state/current-datetime")
		// A lost response does not establish whether the Set was committed.
		// Log config and state through the new client without modifying either.
		logAFTDiagnosticGet(ctx, t, dut, client, fmt.Sprintf("/network-instances/network-instance[name=%s]/afts/global-filter", cfg.VRFName))
	}
	aftpf.ConfigureGlobalFilterPolicies(t, dut, cfg)
}

func logVRFNextHopDiagnostics(t *testing.T, dut *ondatra.DUTDevice, collector *aftcache.AFTStreamSession, prefix string) {
	t.Helper()
	ctx, cancel := context.WithDeadline(context.WithoutCancel(t.Context()), time.Now().Add(90*time.Second))
	defer cancel()
	t.Logf("AFT VRF chain diagnostics UTC=%s NI=%s prefix=%s stream=%+v (CLIENT buffer, not DUT queue)",
		time.Now().UTC().Format(time.RFC3339Nano), vrfName, prefix, collector.Progress())
	nextHopNI := vrfName
	if dut.Vendor() == ondatra.CISCO {
		// Match the validation collector: Cisco's VRF-A prefix refers to
		// forwarding objects held globally under DEFAULT.
		nextHopNI = deviations.DefaultNetworkInstance(dut)
	}
	t.Logf("AFT diagnostic collector scope: prefixNI=%s nextHopGroupNI=%s nextHopNI=%s", vrfName, nextHopNI, nextHopNI)
	groupIDs := map[uint64]bool{}
	nextHopIDs := map[uint64]bool{}
	if snapshot, err := collector.ToAFT(t, dut); err != nil {
		t.Logf("AFT diagnostic subscription cache conversion failed: %v", err)
	} else {
		id, present := snapshot.Prefixes[prefix]
		t.Logf("AFT diagnostic SUBSCRIPTION cache NI=%s prefix=%s present=%t NHG=%d prefixes=%d groups=%d nextHops=%d",
			vrfName, prefix, present, id, len(snapshot.Prefixes), len(snapshot.NextHopGroups), len(snapshot.NextHops))
		if present {
			groupIDs[id] = true
			t.Logf("AFT diagnostic SUBSCRIPTION cache referenced NHG %d: %+v", id, snapshot.NextHopGroups[id])
			if group := snapshot.NextHopGroups[id]; group != nil {
				for _, index := range group.NHIDs {
					nextHopIDs[index] = true
				}
				for _, index := range diagnosticIDs(nextHopIDs, 8) {
					t.Logf("AFT diagnostic SUBSCRIPTION cache referenced NH %d: %+v", index, snapshot.NextHops[index])
				}
			}
		}
	}
	logAFTDiagnosticCLI(ctx, t, dut, "show logging")
	logAFTDiagnosticCLI(ctx, t, dut, fmt.Sprintf("show route vrf %s %s detail", vrfName, prefix))
	logAFTDiagnosticCLI(ctx, t, dut, fmt.Sprintf("show cef vrf %s %s detail", vrfName, prefix))
	client, closeClient := newAFTDiagnosticClient(ctx, t, dut)
	defer closeClient()
	base := fmt.Sprintf("/network-instances/network-instance[name=%s]/afts", vrfName)
	entryPath := fmt.Sprintf("%s/ipv4-unicast/ipv4-entry[prefix=%s]", base, prefix)
	// Explicit STATE requests avoid conflating an ALL/config mapper failure
	// with absent operational data. Compare the whole entry with its state.
	for _, path := range []string{entryPath, entryPath + "/state"} {
		response := logAFTDiagnosticGetType(ctx, t, dut, client, path, gpb.GetRequest_STATE)
		for _, id := range diagnosticUintLeaves(t, response, "next-hop-group") {
			groupIDs[id] = true
		}
	}
	// Inspect only this route's references: at most four NHGs and eight NHs.
	// Do not dump the scale tables or feed fresh reads into the stream cache.
	for _, ni := range []string{vrfName, deviations.DefaultNetworkInstance(dut)} {
		niBase := fmt.Sprintf("/network-instances/network-instance[name=%s]/afts", ni)
		ids := map[uint64]bool{}
		if ni == nextHopNI {
			for index := range nextHopIDs {
				ids[index] = true
			}
		}
		// Compare the referenced group in both NIs. These diagnostic reads
		// never supply values to validation; on Cisco, validation itself
		// follows the DEFAULT NHG/NH subscriptions while retaining VRF-A
		// prefix isolation.
		for _, id := range diagnosticIDs(groupIDs, 4) {
			t.Logf("AFT diagnostic referenced NHG comparison: routeNI=%s lookupNI=%s id=%d; not used for validation", vrfName, ni, id)
			response := logAFTDiagnosticGetType(ctx, t, dut, client, fmt.Sprintf("%s/next-hop-groups/next-hop-group[id=%d]", niBase, id), gpb.GetRequest_STATE)
			for _, index := range diagnosticUintLeaves(t, response, "index") {
				ids[index] = true
			}
		}
		for _, index := range diagnosticIDs(ids, 8) {
			logAFTDiagnosticGetType(ctx, t, dut, client, fmt.Sprintf("%s/next-hops/next-hop[index=%d]", niBase, index), gpb.GetRequest_STATE)
		}
	}
	// Each path gets a separate, short-lived stream to identify which subtree
	// emits updates, returns SyncResponse, or terminates. Do not merge any of
	// these responses into the validation collector.
	nextHopBase := fmt.Sprintf("/network-instances/network-instance[name=%s]/afts", nextHopNI)
	for _, path := range []string{entryPath, nextHopBase + "/next-hop-groups/next-hop-group", nextHopBase + "/next-hops/next-hop"} {
		logAFTDiagnosticSubscription(ctx, t, dut, client, path)
	}
	logAFTDiagnosticCLI(ctx, t, dut, "show cef misc")
	logAFTDiagnosticCLI(ctx, t, dut, "show telemetry model-driven trace all")
	logAFTDiagnosticCLI(ctx, t, dut, "show grpc trace all")
}

func logAFTDiagnosticSubscription(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client gpb.GNMIClient, path string) {
	t.Helper()
	if client == nil || ctx.Err() != nil {
		t.Logf("AFT diagnostic subscription skipped path=%s clientAvailable=%t contextError=%v", path, client != nil, ctx.Err())
		return
	}
	readCtx, cancel := context.WithDeadline(ctx, time.Now().Add(8*time.Second))
	defer cancel()
	request := &gpb.SubscribeRequest{Request: &gpb.SubscribeRequest_Subscribe{Subscribe: &gpb.SubscriptionList{
		Prefix: &gpb.Path{Origin: "openconfig", Target: dut.Name()},
		Mode:   gpb.SubscriptionList_STREAM, Encoding: gpb.Encoding_PROTO, UpdatesOnly: false,
		Subscription: []*gpb.Subscription{{Path: gnmiPath(t, path), Mode: gpb.SubscriptionMode_ON_CHANGE}},
	}}}
	started := time.Now()
	t.Logf("AFT diagnostic independent PROTO subscription start UTC=%s path=%s request=%v", started.UTC().Format(time.RFC3339Nano), path, request)
	stream, err := client.Subscribe(readCtx)
	responses, updates, deletes, syncs := 0, 0, 0, 0
	stopReason := "RPC error"
	defer func() {
		t.Logf("AFT diagnostic independent PROTO subscription end UTC=%s path=%s elapsed=%v responses=%d updates=%d deletes=%d SyncResponses=%d stop=%s status=%s error=%v contextError=%v",
			time.Now().UTC().Format(time.RFC3339Nano), path, time.Since(started), responses, updates, deletes, syncs, stopReason, status.Code(err), err, readCtx.Err())
	}()
	if err != nil {
		return
	}
	if err = stream.Send(request); err != nil {
		return
	}
	// Bound both duration and response count even if the server is busy. Only
	// the first three responses are sampled in the log; counts include all.
	for responses < 128 {
		var response *gpb.SubscribeResponse
		response, err = stream.Recv()
		if err != nil {
			return
		}
		responses++
		if responses <= 3 {
			sample := response.String()
			if len(sample) > 4096 {
				sample = sample[:4096] + " [truncated]"
			}
			t.Logf("AFT diagnostic PROTO sample path=%s response=%d: %s", path, responses, sample)
		}
		updates += len(response.GetUpdate().GetUpdate())
		deletes += len(response.GetUpdate().GetDelete())
		if response.GetSyncResponse() {
			syncs++
			stopReason = "initial SyncResponse (cancel diagnostic stream)"
			return
		}
		if _, ok := response.GetResponse().(*gpb.SubscribeResponse_Error); ok {
			stopReason = fmt.Sprintf("server error response: %v", response.GetError())
			return
		}
	}
	stopReason = "128-response cap reached before SyncResponse"
}

func diagnosticIDs(ids map[uint64]bool, limit int) []uint64 {
	var result []uint64
	for id := range ids {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

// Decode diagnostic IDs without float64 conversion: AFT's uint64 identifiers
// can exceed JSON's exactly representable floating-point integer range.
func diagnosticUintLeaves(t *testing.T, response *gpb.GetResponse, leaf string) []uint64 {
	ids := map[uint64]bool{}
	add := func(value any) {
		id, err := strconv.ParseUint(fmt.Sprint(value), 10, 64)
		if err != nil {
			t.Logf("AFT diagnostic cannot decode %s=%v: %v", leaf, value, err)
			return
		}
		ids[id] = true
	}
	var visit func(any)
	visit = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			for key, child := range node {
				if key[strings.LastIndex(key, ":")+1:] == leaf {
					add(child)
				} else {
					visit(child)
				}
			}
		case []any:
			for _, child := range node {
				visit(child)
			}
		}
	}
	for _, notification := range response.GetNotification() {
		for _, update := range notification.GetUpdate() {
			var decoded any
			switch value := update.GetVal().GetValue().(type) {
			case *gpb.TypedValue_JsonIetfVal:
				decoder := json.NewDecoder(strings.NewReader(string(value.JsonIetfVal)))
				decoder.UseNumber()
				if err := decoder.Decode(&decoded); err != nil {
					t.Logf("AFT diagnostic JSON decode failed: %v", err)
					continue
				}
			case *gpb.TypedValue_UintVal:
				decoded = value.UintVal
			case *gpb.TypedValue_StringVal:
				decoded = value.StringVal
			default:
				t.Logf("AFT diagnostic unsupported ID encoding: %T", value)
				continue
			}
			switch decoded.(type) {
			case map[string]any, []any:
				visit(decoded)
			default:
				path := notificationUpdatePath(t, notification.GetPrefix(), update.GetPath())
				last := path[strings.LastIndex(path, "/")+1:]
				if last[strings.LastIndex(last, ":")+1:] == leaf {
					add(decoded)
				}
			}
		}
	}
	return diagnosticIDs(ids, len(ids))
}
