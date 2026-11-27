// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package plq provides helpers for tests of the gNOI Packet Link Qualification
// service (gnoi.packet_link_qualification.LinkQualification).
package plq

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
)

const (
	// DefaultGeneratorMTU is the generator packet size, and the minimum
	// generator max_mtu capability, for platforms without the
	// plq_generator_capabilities_max_mtu deviation.
	DefaultGeneratorMTU = uint64(8184)
	// DefaultGeneratorPPS is the minimum generator max_pps capability for
	// platforms without the plq_generator_capabilities_max_pps deviation.
	DefaultGeneratorPPS = uint64(1e8)
	// PollInterval is the interval between LinkQualification.List calls while
	// waiting for qualifications to reach a state.
	PollInterval = 5 * time.Second

	// deleteWait bounds how long List may keep reporting deleted records on
	// platforms with the link_qual_wait_after_delete_required deviation.
	deleteWait = 15 * time.Second
	// deletePollInterval is the interval between List calls during deleteWait.
	deletePollInterval = time.Second
)

// Minimum RPCSyncedTiming durations. The device's minimum setup and teardown
// durations from Capabilities are used instead when they are larger.
const (
	generatorPreSync     = 10 * time.Second
	generatorMinSetup    = 30 * time.Second
	generatorPostSync    = 0 * time.Second
	generatorMinTeardown = 30 * time.Second
	reflectorPreSync     = 0 * time.Second
	reflectorMinSetup    = 40 * time.Second
	reflectorPostSync    = 30 * time.Second
	reflectorMinTeardown = 10 * time.Second
	qualificationTime    = 180 * time.Second
)

// ReflectorMode is the loopback mode of a FAR_END reflector endpoint.
type ReflectorMode int

const (
	// AsicLoopback loops packets back in the ASIC.
	AsicLoopback ReflectorMode = iota
	// PmdLoopback loops packets back in the PMD.
	PmdLoopback
)

// String returns the name of the reflector mode.
func (m ReflectorMode) String() string {
	switch m {
	case AsicLoopback:
		return "ASIC loopback"
	case PmdLoopback:
		return "PMD loopback"
	default:
		return fmt.Sprintf("ReflectorMode(%d)", int(m))
	}
}

// LinkPair is a connected pair of a NEAR_END generator port and a FAR_END
// reflector port, with the qualification ID used for each endpoint.
type LinkPair struct {
	GeneratorPort string
	ReflectorPort string
	GeneratorID   string
	ReflectorID   string
	// PacketRate is the generator packet rate in packets per second.
	PacketRate uint64
	// PacketSize is the generator packet size in bytes.
	PacketSize uint32
}

// IDs returns the generator and reflector qualification IDs of pairs, in order.
func IDs(pairs []LinkPair) []string {
	var ids []string
	for _, p := range pairs {
		ids = append(ids, p.GeneratorID, p.ReflectorID)
	}
	return ids
}

// Ports returns the generator and reflector interface names of pairs, in
// order.
func Ports(pairs []LinkPair) []string {
	var ports []string
	for _, p := range pairs {
		ports = append(ports, p.GeneratorPort, p.ReflectorPort)
	}
	return ports
}

// Timing holds the RPCSyncedTiming durations of the generator and reflector
// endpoints of a qualification.
type Timing struct {
	GeneratorPreSyncDuration  time.Duration
	GeneratorSetupDuration    time.Duration
	GeneratorPostSyncDuration time.Duration
	GeneratorTeardownDuration time.Duration
	ReflectorPreSyncDuration  time.Duration
	ReflectorSetupDuration    time.Duration
	ReflectorPostSyncDuration time.Duration
	ReflectorTeardownDuration time.Duration
	// TestDuration is the qualification duration of both endpoints.
	TestDuration time.Duration
}

// SetupTime returns an upper bound for the time from Create until both
// endpoints have finished their pre-sync and setup phases.
func (tm *Timing) SetupTime() time.Duration {
	return max(tm.GeneratorPreSyncDuration, tm.ReflectorPreSyncDuration) + max(tm.GeneratorSetupDuration, tm.ReflectorSetupDuration)
}

// MaxCompletionTime returns the time from Create until both endpoints are
// expected to have finished, which is the longer of the two endpoint
// timelines.
func (tm *Timing) MaxCompletionTime() time.Duration {
	generator := tm.GeneratorPreSyncDuration + tm.GeneratorSetupDuration + tm.TestDuration + tm.GeneratorPostSyncDuration + tm.GeneratorTeardownDuration
	reflector := tm.ReflectorPreSyncDuration + tm.ReflectorSetupDuration + tm.TestDuration + tm.ReflectorPostSyncDuration + tm.ReflectorTeardownDuration
	return max(generator, reflector)
}

// MinGeneratorMTU returns the generator packet size, and the minimum generator
// max_mtu capability, for dut: DefaultGeneratorMTU unless the
// plq_generator_capabilities_max_mtu deviation sets the platform maximum.
func MinGeneratorMTU(dut *ondatra.DUTDevice) uint64 {
	if mtu := deviations.PLQGeneratorCapabilitiesMaxMTU(dut); mtu != 0 {
		return uint64(mtu)
	}
	return DefaultGeneratorMTU
}

// MinGeneratorPPS returns the minimum generator max_pps capability for dut:
// DefaultGeneratorPPS unless the plq_generator_capabilities_max_pps deviation
// sets the platform maximum.
func MinGeneratorPPS(dut *ondatra.DUTDevice) uint64 {
	if pps := deviations.PLQGeneratorCapabilitiesMaxPPS(dut); pps != 0 {
		return pps
	}
	return DefaultGeneratorPPS
}

// rpcFatal fails the test for an error returned by a LinkQualification RPC and
// calls out DEADLINE_EXCEEDED explicitly.
func rpcFatal(t testing.TB, rpc string, rpcTimeout time.Duration, err error) {
	t.Helper()
	if status.Code(err) == codes.DeadlineExceeded {
		t.Fatalf("LinkQualification.%s did not complete within its %v RPC deadline (DEADLINE_EXCEEDED): %v", rpc, rpcTimeout, err)
	}
	t.Fatalf("LinkQualification.%s failed: %v", rpc, err)
}

// Capabilities returns the LinkQualification.Capabilities response. The RPC is
// bounded by rpcTimeout.
func Capabilities(t *testing.T, dut *ondatra.DUTDevice, rpcTimeout time.Duration) *plqpb.CapabilitiesResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	resp, err := dut.RawAPIs().GNOI(t).LinkQualification().Capabilities(ctx, &plqpb.CapabilitiesRequest{})
	if err != nil {
		rpcFatal(t, "Capabilities", rpcTimeout, err)
	}
	return resp
}

// ComputeTiming returns the RPCSyncedTiming of the generator and reflector
// endpoints, and the reflector mode, for a Capabilities response. Setup and
// teardown durations are the larger of the gNOI-2.1 minimums (generator setup
// and teardown 30s, reflector setup 40s and teardown 10s) and the device's
// minimums. ASIC loopback is preferred over PMD loopback.
func ComputeTiming(t *testing.T, caps *plqpb.CapabilitiesResponse) (*Timing, ReflectorMode) {
	t.Helper()
	tm, mode, err := computeTiming(caps)
	if err != nil {
		t.Fatalf("ComputeTiming(): %v", err)
	}
	return tm, mode
}

func computeTiming(caps *plqpb.CapabilitiesResponse) (*Timing, ReflectorMode, error) {
	gen := caps.GetGenerator().GetPacketGenerator()
	ref := caps.GetReflector()
	asic, pmd := ref.GetAsicLoopback(), ref.GetPmdLoopback()

	var mode ReflectorMode
	var refMinSetup, refMinTeardown time.Duration
	switch {
	case wholeSeconds(asic.GetMinSetupDuration()) >= time.Second && wholeSeconds(asic.GetMinTeardownDuration()) >= time.Second:
		mode = AsicLoopback
		refMinSetup, refMinTeardown = asic.GetMinSetupDuration().AsDuration(), asic.GetMinTeardownDuration().AsDuration()
	case wholeSeconds(pmd.GetMinSetupDuration()) >= time.Second && wholeSeconds(pmd.GetMinTeardownDuration()) >= time.Second:
		mode = PmdLoopback
		refMinSetup, refMinTeardown = pmd.GetMinSetupDuration().AsDuration(), pmd.GetMinTeardownDuration().AsDuration()
	default:
		return nil, 0, fmt.Errorf("neither ASIC nor PMD loopback reports min_setup_duration >= 1s and min_teardown_duration >= 1s: %v", ref)
	}

	// Device minimums are used exactly (not truncated to whole seconds) so that
	// the configured durations are never below them.
	return &Timing{
		GeneratorPreSyncDuration:  generatorPreSync,
		GeneratorSetupDuration:    max(generatorMinSetup, gen.GetMinSetupDuration().AsDuration()),
		GeneratorPostSyncDuration: generatorPostSync,
		GeneratorTeardownDuration: max(generatorMinTeardown, gen.GetMinTeardownDuration().AsDuration()),
		ReflectorPreSyncDuration:  reflectorPreSync,
		ReflectorSetupDuration:    max(reflectorMinSetup, refMinSetup),
		ReflectorPostSyncDuration: reflectorPostSync,
		ReflectorTeardownDuration: max(reflectorMinTeardown, refMinTeardown),
		TestDuration:              qualificationTime,
	}, mode, nil
}

// wholeSeconds returns d as a time.Duration without its fractional seconds;
// the gNOI-2.1 capability thresholds are expressed in whole seconds.
func wholeSeconds(d *durationpb.Duration) time.Duration {
	return time.Duration(d.GetSeconds()) * time.Second
}

// pairConfigs returns the QualificationConfiguration of the generator and
// reflector endpoints of pair.
func pairConfigs(pair LinkPair, tm *Timing, mode ReflectorMode) []*plqpb.QualificationConfiguration {
	gen := &plqpb.QualificationConfiguration{
		Id:            pair.GeneratorID,
		InterfaceName: pair.GeneratorPort,
		EndpointType: &plqpb.QualificationConfiguration_PacketGenerator{
			PacketGenerator: &plqpb.PacketGeneratorConfiguration{
				PacketRate: pair.PacketRate,
				PacketSize: pair.PacketSize,
			},
		},
		Timing: rpcSyncedTiming(tm.GeneratorPreSyncDuration, tm.GeneratorSetupDuration, tm.TestDuration, tm.GeneratorPostSyncDuration, tm.GeneratorTeardownDuration),
	}
	ref := &plqpb.QualificationConfiguration{
		Id:            pair.ReflectorID,
		InterfaceName: pair.ReflectorPort,
		Timing:        rpcSyncedTiming(tm.ReflectorPreSyncDuration, tm.ReflectorSetupDuration, tm.TestDuration, tm.ReflectorPostSyncDuration, tm.ReflectorTeardownDuration),
	}
	if mode == PmdLoopback {
		ref.EndpointType = &plqpb.QualificationConfiguration_PmdLoopback{
			PmdLoopback: &plqpb.PmdLoopbackConfiguration{},
		}
	} else {
		ref.EndpointType = &plqpb.QualificationConfiguration_AsicLoopback{
			AsicLoopback: &plqpb.AsicLoopbackConfiguration{},
		}
	}
	return []*plqpb.QualificationConfiguration{gen, ref}
}

func rpcSyncedTiming(preSync, setup, duration, postSync, teardown time.Duration) *plqpb.QualificationConfiguration_Rpc {
	return &plqpb.QualificationConfiguration_Rpc{
		Rpc: &plqpb.RPCSyncedTiming{
			PreSyncDuration:  durationpb.New(preSync),
			SetupDuration:    durationpb.New(setup),
			Duration:         durationpb.New(duration),
			PostSyncDuration: durationpb.New(postSync),
			TeardownDuration: durationpb.New(teardown),
		},
	}
}

// Create starts qualification of all pairs with a single batched
// LinkQualification.Create call bounded by rpcTimeout, and verifies that the
// response reports status code OK (0) for every generator and reflector ID.
func Create(t *testing.T, dut *ondatra.DUTDevice, pairs []LinkPair, tm *Timing, mode ReflectorMode, rpcTimeout time.Duration) {
	t.Helper()
	req := &plqpb.CreateRequest{}
	for _, pair := range pairs {
		req.Interfaces = append(req.Interfaces, pairConfigs(pair, tm, mode)...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	resp, err := dut.RawAPIs().GNOI(t).LinkQualification().Create(ctx, req)
	if err != nil {
		rpcFatal(t, "Create", rpcTimeout, err)
	}
	for _, id := range IDs(pairs) {
		st, ok := resp.GetStatus()[id]
		if !ok {
			t.Fatalf("CreateResponse has no status for qualification ID %q: %v", id, resp)
		}
		if got, want := st.GetCode(), int32(codes.OK); got != want {
			t.Errorf("CreateResponse status code for %q: got %d (%s), want %d", id, got, st.GetMessage(), want)
		}
	}
}

// List returns the LinkQualification.List response. The RPC is bounded by
// rpcTimeout.
func List(t *testing.T, dut *ondatra.DUTDevice, rpcTimeout time.Duration) *plqpb.ListResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	resp, err := dut.RawAPIs().GNOI(t).LinkQualification().List(ctx, &plqpb.ListRequest{})
	if err != nil {
		rpcFatal(t, "List", rpcTimeout, err)
	}
	return resp
}

// Get returns the LinkQualification.Get response for ids. The RPC is bounded
// by rpcTimeout.
func Get(t *testing.T, dut *ondatra.DUTDevice, ids []string, rpcTimeout time.Duration) *plqpb.GetResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	resp, err := dut.RawAPIs().GNOI(t).LinkQualification().Get(ctx, &plqpb.GetRequest{Ids: ids})
	if err != nil {
		rpcFatal(t, "Get", rpcTimeout, err)
	}
	return resp
}

// Delete returns the LinkQualification.Delete response for ids. The RPC is
// bounded by rpcTimeout.
func Delete(t *testing.T, dut *ondatra.DUTDevice, ids []string, rpcTimeout time.Duration) *plqpb.DeleteResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	resp, err := dut.RawAPIs().GNOI(t).LinkQualification().Delete(ctx, &plqpb.DeleteRequest{Ids: ids})
	if err != nil {
		rpcFatal(t, "Delete", rpcTimeout, err)
	}
	return resp
}

// DeleteAndVerify deletes ids with a single batched LinkQualification.Delete
// call bounded by rpcTimeout, verifies status code OK (0) for every ID, and
// verifies that List then reports no qualification records.
func DeleteAndVerify(t *testing.T, dut *ondatra.DUTDevice, ids []string, rpcTimeout time.Duration) {
	t.Helper()
	if len(ids) == 0 {
		return
	}
	resp := Delete(t, dut, ids, rpcTimeout)
	for _, id := range ids {
		st, ok := resp.GetResults()[id]
		if !ok {
			t.Errorf("DeleteResponse has no status for qualification ID %q", id)
			continue
		}
		if got, want := st.GetCode(), int32(codes.OK); got != want {
			t.Errorf("DeleteResponse status code for %q: got %d (%s), want %d", id, got, st.GetMessage(), want)
		}
	}
	awaitNoRecords(t, dut, rpcTimeout)
}

// ClearAll deletes every qualification record reported by
// LinkQualification.List, if any, and verifies that none remain.
func ClearAll(t *testing.T, dut *ondatra.DUTDevice, rpcTimeout time.Duration) {
	t.Helper()
	resp := List(t, dut, rpcTimeout)
	if len(resp.GetResults()) == 0 {
		t.Logf("LinkQualification.List reports no qualification records on %s", dut.Name())
		return
	}
	var ids []string
	for _, r := range resp.GetResults() {
		ids = append(ids, r.GetId())
	}
	t.Logf("Deleting %d qualification record(s) on %s: %v", len(ids), dut.Name(), ids)
	DeleteAndVerify(t, dut, ids, rpcTimeout)
}

// awaitNoRecords verifies that LinkQualification.List reports no
// qualification records. Platforms with the
// link_qual_wait_after_delete_required deviation get up to deleteWait to
// finish their post-delete cleanup.
func awaitNoRecords(t *testing.T, dut *ondatra.DUTDevice, rpcTimeout time.Duration) {
	t.Helper()
	resp := List(t, dut, rpcTimeout)
	if len(resp.GetResults()) == 0 {
		return
	}
	if !deviations.LinkQualWaitAfterDeleteRequired(dut) {
		t.Errorf("LinkQualification.List after Delete: got %d records (%v), want 0", len(resp.GetResults()), resp.GetResults())
		return
	}
	resp, ok := pollList(t, dut, deletePollInterval, deleteWait, rpcTimeout, func(r *plqpb.ListResponse) bool {
		return len(r.GetResults()) == 0
	})
	if !ok {
		t.Errorf("LinkQualification.List %v after Delete: got %d records (%v), want 0", deleteWait, len(resp.GetResults()), resp.GetResults())
	}
}

// AwaitState waits until LinkQualification.List reports state want for every
// qualification ID in ids, and fails the test if any of them reports
// QUALIFICATION_STATE_ERROR or if maxWait expires. Qualification state is only
// exposed by the unary List RPC (there is no telemetry path or streaming RPC
// for it), so List is polled every PollInterval; each call is bounded by
// rpcTimeout.
func AwaitState(t *testing.T, dut *ondatra.DUTDevice, ids []string, want plqpb.QualificationState, maxWait, rpcTimeout time.Duration) {
	t.Helper()
	resp, ok := pollList(t, dut, PollInterval, maxWait, rpcTimeout, func(r *plqpb.ListResponse) bool {
		return allInState(t, r, ids, want)
	})
	if !ok {
		t.Fatalf("Qualification IDs %v did not reach %v within %v; last ListResponse: %v", ids, want, maxWait, resp)
	}
	t.Logf("Qualification IDs %v reached %v", ids, want)
}

// pollList calls LinkQualification.List every interval, each call bounded by
// rpcTimeout, until done returns true or maxWait expires. It returns the last
// response and whether done returned true.
func pollList(t *testing.T, dut *ondatra.DUTDevice, interval, maxWait, rpcTimeout time.Duration, done func(*plqpb.ListResponse) bool) (*plqpb.ListResponse, bool) {
	t.Helper()
	deadline := time.Now().Add(maxWait)
	resp := List(t, dut, rpcTimeout)
	if done(resp) {
		return resp, true
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		resp = List(t, dut, rpcTimeout)
		if done(resp) {
			return resp, true
		}
		if time.Now().After(deadline) {
			return resp, false
		}
	}
	return resp, false
}

// allInState reports whether resp lists every ID in ids with state want, and
// fails the test if any of them is in QUALIFICATION_STATE_ERROR.
func allInState(t *testing.T, resp *plqpb.ListResponse, ids []string, want plqpb.QualificationState) bool {
	t.Helper()
	states := make(map[string]plqpb.QualificationState, len(resp.GetResults()))
	for _, r := range resp.GetResults() {
		states[r.GetId()] = r.GetState()
	}
	for _, id := range ids {
		state, ok := states[id]
		if !ok {
			return false
		}
		if state == plqpb.QualificationState_QUALIFICATION_STATE_ERROR {
			t.Fatalf("Qualification ID %q is in QUALIFICATION_STATE_ERROR: %v", id, resp)
		}
		if state != want {
			return false
		}
	}
	return true
}

// VerifyResults retrieves the results of all pairs with a single batched
// LinkQualification.Get call bounded by rpcTimeout, and verifies for every
// generator and reflector ID: status code OK (0), state
// QUALIFICATION_STATE_COMPLETED, packets_error == 0 and packets_dropped == 0.
// It also verifies packets_sent > 0 and packets_received > 0 on generators,
// and on reflectors in ASIC loopback mode unless the
// plq_reflector_stats_unsupported deviation is set. Packet counts of each pair
// must differ by at most tolerancePct percent: generator sent vs. reflector
// received and reflector sent vs. generator received when reflector counters
// are available, otherwise generator sent vs. generator received.
func VerifyResults(t *testing.T, dut *ondatra.DUTDevice, pairs []LinkPair, mode ReflectorMode, tolerancePct float64, rpcTimeout time.Duration) {
	t.Helper()
	results := Get(t, dut, IDs(pairs), rpcTimeout).GetResults()
	reflectorStats := mode == AsicLoopback && !deviations.PLQReflectorStatsUnsupported(dut)
	for _, pair := range pairs {
		gen, ref := results[pair.GeneratorID], results[pair.ReflectorID]
		verifyResult(t, pair.GeneratorID, gen, true)
		verifyResult(t, pair.ReflectorID, ref, reflectorStats)
		if reflectorStats {
			verifyDiscrepancy(t, pair.GeneratorID+" packets_sent vs. "+pair.ReflectorID+" packets_received", gen.GetPacketsSent(), ref.GetPacketsReceived(), tolerancePct)
			verifyDiscrepancy(t, pair.ReflectorID+" packets_sent vs. "+pair.GeneratorID+" packets_received", ref.GetPacketsSent(), gen.GetPacketsReceived(), tolerancePct)
		} else {
			verifyDiscrepancy(t, pair.GeneratorID+" packets_sent vs. packets_received", gen.GetPacketsSent(), gen.GetPacketsReceived(), tolerancePct)
		}
	}
}

// verifyResult verifies the status, state and error and drop counters of the
// result of qualification id, and non-zero sent and received counters if
// wantPackets. QualificationResult.status is only set for
// QUALIFICATION_STATE_ERROR, so an unset status reads as code OK (0).
func verifyResult(t *testing.T, id string, res *plqpb.QualificationResult, wantPackets bool) {
	t.Helper()
	if res == nil {
		t.Fatalf("GetResponse has no result for qualification ID %q", id)
	}
	if got, want := res.GetStatus().GetCode(), int32(codes.OK); got != want {
		t.Errorf("Result %q status code: got %d (%s), want %d", id, got, res.GetStatus().GetMessage(), want)
	}
	if got, want := res.GetState(), plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED; got != want {
		t.Errorf("Result %q state: got %v, want %v", id, got, want)
	}
	if got := res.GetPacketsError(); got != 0 {
		t.Errorf("Result %q packets_error: got %d, want 0", id, got)
	}
	if got := res.GetPacketsDropped(); got != 0 {
		t.Errorf("Result %q packets_dropped: got %d, want 0", id, got)
	}
	if !wantPackets {
		return
	}
	if res.GetPacketsSent() == 0 {
		t.Errorf("Result %q packets_sent: got 0, want > 0", id)
	}
	if res.GetPacketsReceived() == 0 {
		t.Errorf("Result %q packets_received: got 0, want > 0", id)
	}
}

// verifyDiscrepancy verifies that packet counts a and b differ by at most
// tolerancePct percent of their mean.
func verifyDiscrepancy(t *testing.T, desc string, a, b uint64, tolerancePct float64) {
	t.Helper()
	pct, ok := discrepancyPct(a, b)
	if !ok {
		t.Errorf("%s: both packet counts are 0", desc)
		return
	}
	if pct > tolerancePct {
		t.Errorf("%s: discrepancy %.6f%% exceeds tolerance %.4f%% (%d vs. %d)", desc, pct, tolerancePct, a, b)
	}
}

// discrepancyPct returns the difference between a and b as a percentage of
// their mean; ok is false when both are 0.
func discrepancyPct(a, b uint64) (pct float64, ok bool) {
	sum := float64(a) + float64(b)
	if sum == 0 {
		return 0, false
	}
	return math.Abs(float64(a)-float64(b)) / (sum / 2) * 100, true
}
