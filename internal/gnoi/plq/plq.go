// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plq

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc/codes"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
)

const (
	rpcTimeout             = 60 * time.Second
	pollInterval           = 10 * time.Second
	listPollInterval       = 2 * time.Second
	listPresentTimeout     = 30 * time.Second
	deletePollTimeout      = 3 * time.Minute
	minHistResultsReq      = uint32(2)
	listDeleteTestRate     = uint64(100000)
	minCapSetupDuration    = time.Second
	minCapTeardownDuration = time.Second
	minCapSampleInterval   = 10 * time.Second
)

type qualificationSpec struct {
	pair          LinkPair
	genID         string
	refID         string
	traffic       TrafficParams
	durations     Durations
	usePmdReflect bool
}

// Run executes the full suite of gNOI Packet Link Qualification tests for the configured speed and options.
func Run(t *testing.T, cfg Config) {
	t.Helper()
	duts := sortedDUTs(t)
	for _, dut := range duts {
		DeleteAllQualifications(t, dut)
	}

	pairs := DiscoverLinkPairs(t, cfg.TargetSpeed)
	t.Logf("Discovered %d link pair(s) for targetSpeed=%v", len(pairs), cfg.TargetSpeed)

	targetSpeedBPS := SpeedToBPS(cfg.TargetSpeed)
	if targetSpeedBPS == 0 {
		targetSpeedBPS = pairs[0].SpeedBPS
	}

	targetPairs := pairs[:1]

	t.Run("Capabilities", func(t *testing.T) {
		TestCapabilitiesResponse(t, duts, targetSpeedBPS)
	})
	t.Run("NonexistingID", func(t *testing.T) {
		TestNonexistingID(t, duts)
	})
	t.Run("ListDelete", func(t *testing.T) {
		ConfigureSingletonLinks(t, targetPairs)
		TestListDelete(t, targetPairs[0])
	})

	if cfg.RunMaxPPSSubtest {
		t.Run("MaxPPS", func(t *testing.T) {
			ConfigureSingletonLinks(t, targetPairs)
			TestMaxPPS(t, targetPairs[0], cfg.TestDuration)
		})
	}

	t.Run("Singleton", func(t *testing.T) {
		ConfigureSingletonLinks(t, targetPairs)
		TestLinkPairs(t, "singleton", targetPairs, cfg.TestDuration)
	})

	if cfg.RunLAGSubtest {
		t.Run("MemberLinkLAG", func(t *testing.T) {
			if !ConfigureLAGLinks(t, targetPairs) {
				t.Skip("Skipping Member-Link LAG subtest on incompatible topology")
			}
			TestLinkPairs(t, "lag", targetPairs, cfg.TestDuration)
		})
	}
}

// TestCapabilitiesResponse queries Capabilities once per DUT and validates vendor-aware generator
// and reflector requirements.
func TestCapabilitiesResponse(t *testing.T, duts []*ondatra.DUTDevice, targetSpeedBPS uint64) {
	t.Helper()
	for _, dut := range duts {
		resp := queryCapabilities(t, dut)
		t.Logf("DUT %s CapabilitiesResponse: %v", dut.Name(), resp)
		if resp.GetTime() == nil {
			t.Errorf("DUT %s CapabilitiesResponse.Time is nil", dut.Name())
		}
		if got, want := resp.GetMaxHistoricalResultsPerInterface(), uint64(minHistResultsReq); got < want {
			t.Errorf("DUT %s MaxHistoricalResultsPerInterface: got %d, want >= %d", dut.Name(), got, want)
		}
		validateGeneratorCapabilities(t, dut, resp.GetGenerator().GetPacketGenerator(), targetSpeedBPS)
		validateReflectorCapabilities(t, dut, resp.GetReflector())
	}
}

func validateGeneratorCapabilities(t *testing.T, dut *ondatra.DUTDevice, genCaps *plqpb.PacketGeneratorCapabilities, targetSpeedBPS uint64) {
	t.Helper()
	if genCaps == nil {
		t.Fatalf("DUT %s PacketGenerator capabilities are nil", dut.Name())
	}
	logs, errs := evaluateGeneratorCapabilities(dut.Name(), genCaps, ExpectedMinCapabilities(dut, targetSpeedBPS))
	for _, msg := range logs {
		t.Log(msg)
	}
	for _, msg := range errs {
		t.Error(msg)
	}
}

func evaluateGeneratorCapabilities(dutName string, genCaps *plqpb.PacketGeneratorCapabilities, thresholds CapabilityThresholds) ([]string, []string) {
	var logs, errs []string
	checks := []struct {
		desc string
		got  uint64
		min  uint64
	}{
		{desc: "Generator MaxMtu", got: uint64(genCaps.GetMaxMtu()), min: thresholds.MinMTU},
		{desc: "Generator MaxBps", got: uint64(genCaps.GetMaxBps()), min: thresholds.MinBPS},
		{desc: "Generator MaxPps", got: uint64(genCaps.GetMaxPps()), min: thresholds.MinPPS},
	}
	for _, c := range checks {
		logs = append(logs, fmt.Sprintf("DUT %s capability %s: got %d, want >= %d", dutName, c.desc, c.got, c.min))
		if c.got < c.min {
			errs = append(errs, fmt.Sprintf("DUT %s %s: got %d, want >= %d", dutName, c.desc, c.got, c.min))
		}
	}
	if genCaps.GetMinMtu() == 0 || genCaps.GetMinMtu() > genCaps.GetMaxMtu() {
		errs = append(errs, fmt.Sprintf("DUT %s invalid MTU bounds: min_mtu=%d, max_mtu=%d", dutName, genCaps.GetMinMtu(), genCaps.GetMaxMtu()))
	}

	durationChecks := []struct {
		desc string
		got  time.Duration
		min  time.Duration
	}{
		{desc: "Generator MinSetupDuration", got: protoDuration(genCaps.GetMinSetupDuration()), min: minCapSetupDuration},
		{desc: "Generator MinTeardownDuration", got: protoDuration(genCaps.GetMinTeardownDuration()), min: minCapTeardownDuration},
		{desc: "Generator MinSampleInterval", got: protoDuration(genCaps.GetMinSampleInterval()), min: minCapSampleInterval},
	}
	for _, c := range durationChecks {
		logs = append(logs, fmt.Sprintf("DUT %s capability %s: got %v, want >= %v", dutName, c.desc, c.got, c.min))
		if c.got < c.min {
			errs = append(errs, fmt.Sprintf("DUT %s %s: got %v, want >= %v", dutName, c.desc, c.got, c.min))
		}
	}
	return logs, errs
}

func validateReflectorCapabilities(t *testing.T, dut *ondatra.DUTDevice, refCaps *plqpb.ReflectorCapabilities) {
	t.Helper()
	if refCaps == nil {
		t.Fatalf("DUT %s Reflector capabilities are nil", dut.Name())
	}
	if !reflectorTimingSupported(refCaps) {
		t.Errorf("DUT %s Reflector MinSetupDuration or MinTeardownDuration is not >= 1s for PmdLoopback or AsicLoopback: %v", dut.Name(), refCaps)
	}
}

func reflectorTimingSupported(refCaps *plqpb.ReflectorCapabilities) bool {
	pmdOK := protoDuration(refCaps.GetPmdLoopback().GetMinSetupDuration()) >= minCapSetupDuration &&
		protoDuration(refCaps.GetPmdLoopback().GetMinTeardownDuration()) >= minCapTeardownDuration
	asicOK := protoDuration(refCaps.GetAsicLoopback().GetMinSetupDuration()) >= minCapSetupDuration &&
		protoDuration(refCaps.GetAsicLoopback().GetMinTeardownDuration()) >= minCapTeardownDuration
	return pmdOK || asicOK
}

func preferPmdReflector(refCaps *plqpb.ReflectorCapabilities) bool {
	if refCaps == nil {
		return true
	}
	return refCaps.GetPmdLoopback().GetMinSetupDuration().AsDuration() > 0 || refCaps.GetAsicLoopback().GetMinSetupDuration().AsDuration() == 0
}

// TestNonexistingID verifies Get and Delete return NOT_FOUND (code 5) for an unknown session ID.
func TestNonexistingID(t *testing.T, duts []*ondatra.DUTDevice) {
	t.Helper()
	for idx, dut := range duts {
		verifyNonexistingIDOnDUT(t, dut, idx)
	}
}

func verifyNonexistingIDOnDUT(t *testing.T, dut *ondatra.DUTDevice, idx int) {
	t.Helper()
	id := fmt.Sprintf("non-existent-id-%d-%d", idx, time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	client := dut.RawAPIs().GNOI(t).LinkQualification()
	getResp, err := client.Get(ctx, &plqpb.GetRequest{Ids: []string{id}})
	if err != nil {
		t.Fatalf("DUT %s Get(%s) RPC failed: %v", dut.Name(), id, err)
	}
	if got, want := getResp.GetResults()[id].GetStatus().GetCode(), int32(codes.NotFound); got != want {
		t.Errorf("DUT %s Get(%s) status code: got %d, want %d", dut.Name(), id, got, want)
	}

	delResp, err := client.Delete(ctx, &plqpb.DeleteRequest{Ids: []string{id}})
	if err != nil {
		t.Fatalf("DUT %s Delete(%s) RPC failed: %v", dut.Name(), id, err)
	}
	if got, want := delResp.GetResults()[id].GetCode(), int32(codes.NotFound); got != want {
		t.Errorf("DUT %s Delete(%s) status code: got %d, want %d", dut.Name(), id, got, want)
	}
}

// TestListDelete creates qualification sessions on the first link, verifies List returns them,
// deletes them via Delete, and confirms they are removed from List and Get.
func TestListDelete(t *testing.T, pair LinkPair) {
	t.Helper()
	capsA := queryCapabilities(t, pair.A.DUT)
	capsB := queryCapabilities(t, pair.B.DUT)
	dur, err := CalculateDurationsForVendors(pair.A.DUT.Vendor(), pair.B.DUT.Vendor(), capsA.GetGenerator().GetPacketGenerator(), capsB.GetReflector(), 30*time.Second)
	if err != nil {
		t.Fatalf("CalculateDurations failed: %v", err)
	}
	pktSize := choosePacketSize(capsA.GetGenerator().GetPacketGenerator().GetMinMtu(), capsA.GetGenerator().GetPacketGenerator().GetMaxMtu(), defaultTargetPacketSize)
	spec := qualificationSpec{
		pair:          pair,
		genID:         uniqueID("plq-listdel-gen", 0),
		refID:         uniqueID("plq-listdel-ref", 0),
		traffic:       TrafficParams{PacketRate: listDeleteTestRate, PacketSize: pktSize, Duration: dur.Test},
		durations:     dur,
		usePmdReflect: preferPmdReflector(capsB.GetReflector()),
	}

	markCleaned := registerSessionCleanup(t, []qualificationSpec{spec})
	issueCreateBatch(t, []qualificationSpec{spec})

	if pair.A.DUT == pair.B.DUT {
		ids := []string{spec.genID, spec.refID}
		waitForIDsInList(t, pair.A.DUT, ids, true, listPollInterval, listPresentTimeout)
		assertIDsInList(t, pair.A.DUT, ids, true)
	} else {
		waitForIDsInList(t, pair.A.DUT, []string{spec.genID}, true, listPollInterval, listPresentTimeout)
		waitForIDsInList(t, pair.B.DUT, []string{spec.refID}, true, listPollInterval, listPresentTimeout)
		assertIDsInList(t, pair.A.DUT, []string{spec.genID}, true)
		assertIDsInList(t, pair.B.DUT, []string{spec.refID}, true)
	}

	deleteQualificationIDs(t, pair.A.DUT, []string{spec.genID}, true)
	deleteQualificationIDs(t, pair.B.DUT, []string{spec.refID}, true)
	if deviations.LinkQualWaitAfterDeleteRequired(pair.A.DUT) || deviations.LinkQualWaitAfterDeleteRequired(pair.B.DUT) {
		waitForIDsRemoved(t, pair.A.DUT, []string{spec.genID})
		waitForIDsRemoved(t, pair.B.DUT, []string{spec.refID})
	}

	assertIDsInList(t, pair.A.DUT, []string{spec.genID}, false)
	assertIDsInList(t, pair.B.DUT, []string{spec.refID}, false)
	if !t.Failed() {
		markCleaned()
	}
}

// TestMaxPPS qualifies one link at a time at the device's MaxPps capability with dynamic packet sizing.
// On two-DUT topologies, both DUTs are validated as generators sequentially.
func TestMaxPPS(t *testing.T, pair LinkPair, requestedDuration time.Duration) {
	t.Helper()
	runSingleMaxPPS(t, "a-to-b", pair, requestedDuration)
	if pair.B.DUT != pair.A.DUT {
		reverse := LinkPair{A: pair.B, B: pair.A, SpeedBPS: pair.SpeedBPS}
		runSingleMaxPPS(t, "b-to-a", reverse, requestedDuration)
	}
}

func runSingleMaxPPS(t *testing.T, label string, pair LinkPair, requestedDuration time.Duration) {
	t.Helper()
	t.Run(label, func(t *testing.T) {
		capsGen := queryCapabilities(t, pair.A.DUT)
		capsRef := queryCapabilities(t, pair.B.DUT)

		genCaps := capsGen.GetGenerator().GetPacketGenerator()
		traffic, err := CalculateMaxPPSParams(pair.SpeedBPS, genCaps, requestedDuration)
		if err != nil {
			t.Fatalf("CalculateMaxPPSParams failed: %v", err)
		}
		dur, err := CalculateDurationsForVendors(pair.A.DUT.Vendor(), pair.B.DUT.Vendor(), genCaps, capsRef.GetReflector(), traffic.Duration)
		if err != nil {
			t.Fatalf("CalculateDurations failed: %v", err)
		}
		traffic.Duration = dur.Test
		t.Logf("MaxPPS (%s -> %s): rate=%d pps, size=%d B, targetBps=%d, duration=%v",
			pair.A.Port.Name(), pair.B.Port.Name(), traffic.PacketRate, traffic.PacketSize, traffic.TargetBPS, traffic.Duration)

		spec := qualificationSpec{
			pair:          pair,
			genID:         uniqueID("plq-maxpps-gen", 0),
			refID:         uniqueID("plq-maxpps-ref", 0),
			traffic:       traffic,
			durations:     dur,
			usePmdReflect: preferPmdReflector(capsRef.GetReflector()),
		}
		executeAndValidateSpecs(t, []qualificationSpec{spec})
	})
}

// TestLinkPairs runs packet link qualification across the provided link pairs.
func TestLinkPairs(t *testing.T, label string, pairs []LinkPair, requestedDuration time.Duration) {
	t.Helper()
	capsByDUT := make(map[*ondatra.DUTDevice]*plqpb.CapabilitiesResponse)
	genCountByDUT := make(map[*ondatra.DUTDevice]int)
	useInjectorByIdx := make([]bool, len(pairs))
	for i, pair := range pairs {
		if _, ok := capsByDUT[pair.A.DUT]; !ok {
			capsByDUT[pair.A.DUT] = queryCapabilities(t, pair.A.DUT)
		}
		if _, ok := capsByDUT[pair.B.DUT]; !ok {
			capsByDUT[pair.B.DUT] = queryCapabilities(t, pair.B.DUT)
		}
		useInjectorByIdx[i] = shouldUseInjector(pair.A, capsByDUT[pair.A.DUT])
		if !useInjectorByIdx[i] {
			genCountByDUT[pair.A.DUT]++
		}
	}

	specs := make([]qualificationSpec, 0, len(pairs))
	for i, pair := range pairs {
		capsA := capsByDUT[pair.A.DUT]
		refCaps := capsByDUT[pair.B.DUT].GetReflector()

		var (
			traffic TrafficParams
			dur     Durations
			err     error
		)
		if useInjectorByIdx[i] {
			injCaps := capsA.GetGenerator().GetPacketInjector()
			traffic, err = CalculateInjectorParams(injCaps, requestedDuration)
			if err != nil {
				t.Fatalf("CalculateInjectorParams(%s pair %d) failed: %v", label, i, err)
			}
			dur, err = CalculateInjectorDurationsForVendors(pair.A.DUT.Vendor(), pair.B.DUT.Vendor(), injCaps, refCaps, traffic.Duration)
			if err != nil {
				t.Fatalf("CalculateInjectorDurations(%s pair %d) failed: %v", label, i, err)
			}
			traffic.Duration = dur.Test
			t.Logf("%s pair[%d] %s:%s -> %s:%s (PacketInjector): count=%d, size=%d B, loopback=%v",
				label, i, pair.A.DUT.Name(), pair.A.Port.Name(), pair.B.DUT.Name(), pair.B.Port.Name(),
				traffic.PacketCount, traffic.PacketSize, traffic.InjectorLoopback)
		} else {
			genCaps := capsA.GetGenerator().GetPacketGenerator()
			concurrentOnGenDUT := genCountByDUT[pair.A.DUT]

			traffic, err = CalculateRateAndSize(pair.SpeedBPS, genCaps, concurrentOnGenDUT, requestedDuration)
			if err != nil {
				t.Fatalf("CalculateRateAndSize(%s pair %d) failed: %v", label, i, err)
			}
			dur, err = CalculateDurationsForVendors(pair.A.DUT.Vendor(), pair.B.DUT.Vendor(), genCaps, refCaps, traffic.Duration)
			if err != nil {
				t.Fatalf("CalculateDurations(%s pair %d) failed: %v", label, i, err)
			}
			traffic.Duration = dur.Test
			t.Logf("%s pair[%d] %s:%s -> %s:%s: rate=%d pps, size=%d B, targetBps=%d",
				label, i, pair.A.DUT.Name(), pair.A.Port.Name(), pair.B.DUT.Name(), pair.B.Port.Name(),
				traffic.PacketRate, traffic.PacketSize, traffic.TargetBPS)
		}

		specs = append(specs, qualificationSpec{
			pair:          pair,
			genID:         uniqueID("plq-"+label+"-gen", i),
			refID:         uniqueID("plq-"+label+"-ref", i),
			traffic:       traffic,
			durations:     dur,
			usePmdReflect: preferPmdReflector(refCaps),
		})
	}

	executeAndValidateSpecs(t, specs)
}

func shouldUseInjector(ep LinkEndpoint, capsResp *plqpb.CapabilitiesResponse) bool {
	if ep.DUT == nil || ep.Port == nil {
		return false
	}
	return isChannelizedJuniperInjector(ep.DUT.Vendor(), ep.Port.Name(), capsResp)
}

func isChannelizedJuniperInjector(vendor ondatra.Vendor, portName string, capsResp *plqpb.CapabilitiesResponse) bool {
	if capsResp == nil {
		return false
	}
	// Channelized Juniper breakout ports sharing a physical interface cannot run PacketGenerator concurrently.
	return vendor == ondatra.JUNIPER &&
		strings.Contains(portName, ":") &&
		capsResp.GetGenerator().GetPacketInjector() != nil
}

func executeAndValidateSpecs(t *testing.T, specs []qualificationSpec) {
	t.Helper()
	markCleaned := registerSessionCleanup(t, specs)
	issueCreateBatch(t, specs)
	pollUntilCompleted(t, specs)
	verifyResults(t, specs)
	deleteSpecs(t, specs)
	markCleaned()
}

func registerSessionCleanup(t *testing.T, specs []qualificationSpec) func() {
	t.Helper()
	var sessionsDeleted bool
	t.Cleanup(func() {
		if !sessionsDeleted {
			deleteSpecs(t, specs)
		}
	})
	return func() {
		sessionsDeleted = true
	}
}

func issueCreateBatch(t *testing.T, specs []qualificationSpec) {
	t.Helper()
	reqsByDUT := make(map[*ondatra.DUTDevice][]*plqpb.QualificationConfiguration)
	idsByDUT := make(map[*ondatra.DUTDevice][]string)

	for _, s := range specs {
		genCfg := buildGeneratorConfig(s)
		refCfg := buildReflectorConfig(s)

		reqsByDUT[s.pair.A.DUT] = append(reqsByDUT[s.pair.A.DUT], genCfg)
		reqsByDUT[s.pair.B.DUT] = append(reqsByDUT[s.pair.B.DUT], refCfg)
		idsByDUT[s.pair.A.DUT] = append(idsByDUT[s.pair.A.DUT], s.genID)
		idsByDUT[s.pair.B.DUT] = append(idsByDUT[s.pair.B.DUT], s.refID)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(reqsByDUT))
	for dut, cfgs := range reqsByDUT {
		wg.Add(1)
		go func(d *ondatra.DUTDevice, qCfgs []*plqpb.QualificationConfiguration, wantIDs []string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
			defer cancel()
			resp, err := d.RawAPIs().GNOI(t).LinkQualification().Create(ctx, &plqpb.CreateRequest{
				Interfaces: qCfgs,
			})
			if err != nil {
				errCh <- fmt.Errorf("DUT %s Create failed: %w", d.Name(), err)
				return
			}
			for _, id := range wantIDs {
				st, ok := resp.GetStatus()[id]
				if !ok {
					errCh <- fmt.Errorf("DUT %s CreateResponse missing status for id %q", d.Name(), id)
					return
				}
				if st.GetCode() != int32(codes.OK) {
					errCh <- fmt.Errorf("DUT %s Create(%s) error code=%d msg=%q", d.Name(), id, st.GetCode(), st.GetMessage())
					return
				}
			}
		}(dut, cfgs, idsByDUT[dut])
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatal(err)
	}
}

func pollUntilCompleted(t *testing.T, specs []qualificationSpec) {
	t.Helper()
	maxTimeout := time.Duration(0)
	pendingByDUT := make(map[*ondatra.DUTDevice]map[string]LinkEndpoint)
	for _, s := range specs {
		if s.durations.PollTimeout > maxTimeout {
			maxTimeout = s.durations.PollTimeout
		}
		if pendingByDUT[s.pair.A.DUT] == nil {
			pendingByDUT[s.pair.A.DUT] = make(map[string]LinkEndpoint)
		}
		if pendingByDUT[s.pair.B.DUT] == nil {
			pendingByDUT[s.pair.B.DUT] = make(map[string]LinkEndpoint)
		}
		pendingByDUT[s.pair.A.DUT][s.genID] = s.pair.A
		pendingByDUT[s.pair.B.DUT][s.refID] = s.pair.B
	}

	ctx, cancel := context.WithTimeout(context.Background(), maxTimeout)
	defer cancel()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	verifiedOper := make(map[string]bool)
	for {
		done := checkAllPendingStates(ctx, t, pendingByDUT, verifiedOper)
		if done {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Timed out after %v waiting for link qualifications to complete: %v", maxTimeout, ctx.Err())
		case <-ticker.C:
		}
	}
}

func checkAllPendingStates(ctx context.Context, t *testing.T, pendingByDUT map[*ondatra.DUTDevice]map[string]LinkEndpoint, verifiedOper map[string]bool) bool {
	t.Helper()
	allDone := true
	for dut, idMap := range pendingByDUT {
		if len(idMap) == 0 {
			continue
		}
		rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
		listResp, err := dut.RawAPIs().GNOI(t).LinkQualification().List(rpcCtx, &plqpb.ListRequest{})
		cancel()
		if err != nil {
			t.Fatalf("DUT %s List() failed during poll: %v", dut.Name(), err)
		}

		byID := make(map[string]*plqpb.ListResult, len(listResp.GetResults()))
		for _, res := range listResp.GetResults() {
			byID[res.GetId()] = res
		}
		if !evaluateDUTListResults(ctx, t, dut, idMap, byID, verifiedOper) {
			allDone = false
		}
	}
	return allDone
}

func evaluateDUTListResults(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, idMap map[string]LinkEndpoint, byID map[string]*plqpb.ListResult, verifiedOper map[string]bool) bool {
	t.Helper()
	dutDone := true
	for id, ep := range idMap {
		res, ok := byID[id]
		if !ok {
			t.Fatalf("DUT %s ListResponse missing active qualification id %q", dut.Name(), id)
		}
		switch res.GetState() {
		case plqpb.QualificationState_QUALIFICATION_STATE_ERROR:
			t.Fatalf("DUT %s qualification %q (%s) entered ERROR state: %s", dut.Name(), id, ep.Port.Name(), fetchErrorDetail(ctx, t, dut, id))
		case plqpb.QualificationState_QUALIFICATION_STATE_RUNNING:
			dutDone = false
			if !verifiedOper[id] && checkRunningOperStatus(t, ep) {
				verifiedOper[id] = true
			}
		case plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED:
			delete(idMap, id)
		default:
			dutDone = false
		}
	}
	return dutDone
}

func fetchErrorDetail(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, id string) string {
	t.Helper()
	rpcCtx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	resp, err := dut.RawAPIs().GNOI(t).LinkQualification().Get(rpcCtx, &plqpb.GetRequest{Ids: []string{id}})
	if err != nil {
		return fmt.Sprintf("Get(%q) failed: %v", id, err)
	}
	qr := resp.GetResults()[id]
	if qr == nil {
		return "result missing in GetResponse"
	}
	return fmt.Sprintf("state=%v code=%d msg=%q", qr.GetState(), qr.GetStatus().GetCode(), qr.GetStatus().GetMessage())
}

func checkRunningOperStatus(t *testing.T, ep LinkEndpoint) bool {
	t.Helper()
	if deviations.SkipPlqInterfaceOperStatusCheck(ep.DUT) {
		return true
	}
	got, ok := gnmi.Lookup(t, ep.DUT, gnmi.OC().Interface(ep.Port.Name()).OperStatus().State()).Val()
	if !ok {
		t.Errorf("DUT %s interface %s missing OperStatus during QUALIFICATION_STATE_RUNNING", ep.DUT.Name(), ep.Port.Name())
		return false
	}
	if want := oc.Interface_OperStatus_TESTING; got != want {
		t.Errorf("DUT %s interface %s OperStatus during RUNNING: got %v, want %v", ep.DUT.Name(), ep.Port.Name(), got, want)
		return false
	}
	return true
}

func buildGeneratorConfig(s qualificationSpec) *plqpb.QualificationConfiguration {
	return newGeneratorQualificationConfig(s.genID, s.pair.A.Port.Name(), s.traffic, s.durations)
}

func newGeneratorQualificationConfig(id, intfName string, traffic TrafficParams, durations Durations) *plqpb.QualificationConfiguration {
	cfg := &plqpb.QualificationConfiguration{
		Id:            id,
		InterfaceName: intfName,
		Timing: &plqpb.QualificationConfiguration_Rpc{
			Rpc: &plqpb.RPCSyncedTiming{
				Duration:         durationpb.New(durations.Test),
				PreSyncDuration:  durationpb.New(durations.GenPreSync),
				SetupDuration:    durationpb.New(durations.GenSetup),
				PostSyncDuration: durationpb.New(durations.GenPostSync),
				TeardownDuration: durationpb.New(durations.GenTeardown),
			},
		},
	}
	if traffic.UseInjector {
		inj := &plqpb.PacketInjectorConfiguration{
			PacketCount: traffic.PacketCount,
			PacketSize:  traffic.PacketSize,
		}
		if traffic.InjectorLoopback == plqpb.PacketInjectorLoopbackMode_PACKET_INJECTOR_LOOPBACK_MODE_PMD {
			inj.LoopbackMode = &plqpb.PacketInjectorConfiguration_PmdLoopback{
				PmdLoopback: &plqpb.PmdLoopbackConfiguration{},
			}
		} else {
			inj.LoopbackMode = &plqpb.PacketInjectorConfiguration_AsicLoopback{
				AsicLoopback: &plqpb.AsicLoopbackConfiguration{},
			}
		}
		cfg.EndpointType = &plqpb.QualificationConfiguration_PacketInjector{
			PacketInjector: inj,
		}
		return cfg
	}
	cfg.EndpointType = &plqpb.QualificationConfiguration_PacketGenerator{
		PacketGenerator: &plqpb.PacketGeneratorConfiguration{
			PacketRate: traffic.PacketRate,
			PacketSize: traffic.PacketSize,
		},
	}
	return cfg
}

func buildReflectorConfig(s qualificationSpec) *plqpb.QualificationConfiguration {
	cfg := &plqpb.QualificationConfiguration{
		Id:            s.refID,
		InterfaceName: s.pair.B.Port.Name(),
		Timing: &plqpb.QualificationConfiguration_Rpc{
			Rpc: &plqpb.RPCSyncedTiming{
				Duration:         durationpb.New(s.durations.Test),
				PreSyncDuration:  durationpb.New(s.durations.RefPreSync),
				SetupDuration:    durationpb.New(s.durations.RefSetup),
				PostSyncDuration: durationpb.New(s.durations.RefPostSync),
				TeardownDuration: durationpb.New(s.durations.RefTeardown),
			},
		},
	}
	if s.usePmdReflect {
		cfg.EndpointType = &plqpb.QualificationConfiguration_PmdLoopback{
			PmdLoopback: &plqpb.PmdLoopbackConfiguration{},
		}
	} else {
		cfg.EndpointType = &plqpb.QualificationConfiguration_AsicLoopback{
			AsicLoopback: &plqpb.AsicLoopbackConfiguration{},
		}
	}
	return cfg
}

func verifyResults(t *testing.T, specs []qualificationSpec) {
	t.Helper()
	idsByDUT := make(map[*ondatra.DUTDevice][]string)
	for _, s := range specs {
		idsByDUT[s.pair.A.DUT] = append(idsByDUT[s.pair.A.DUT], s.genID)
		idsByDUT[s.pair.B.DUT] = append(idsByDUT[s.pair.B.DUT], s.refID)
	}

	resultsByDUT := make(map[*ondatra.DUTDevice]map[string]*plqpb.QualificationResult)
	for dut, ids := range idsByDUT {
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		getResp, err := dut.RawAPIs().GNOI(t).LinkQualification().Get(ctx, &plqpb.GetRequest{Ids: ids})
		cancel()
		if err != nil {
			t.Fatalf("DUT %s Get(%v) failed: %v", dut.Name(), ids, err)
		}
		resultsByDUT[dut] = getResp.GetResults()
	}

	for _, s := range specs {
		genRes := resultsByDUT[s.pair.A.DUT][s.genID]
		refRes := resultsByDUT[s.pair.B.DUT][s.refID]
		// PMD loopback operates at the PHY before MAC/ASIC counters, so reflector packet counts may be unavailable.
		validateRef := !s.usePmdReflect && !deviations.PLQReflectorStatsUnsupported(s.pair.B.DUT)
		t.Logf("Link %s:%s -> %s:%s results: gen={sent:%d rx:%d err:%d drop:%d}, ref={sent:%d rx:%d err:%d drop:%d}",
			s.pair.A.DUT.Name(), s.pair.A.Port.Name(),
			s.pair.B.DUT.Name(), s.pair.B.Port.Name(),
			genRes.GetPacketsSent(), genRes.GetPacketsReceived(), genRes.GetPacketsError(), genRes.GetPacketsDropped(),
			refRes.GetPacketsSent(), refRes.GetPacketsReceived(), refRes.GetPacketsError(), refRes.GetPacketsDropped())

		if err := ValidateStatsPair(genRes, refRes, s.traffic, validateRef); err != nil {
			t.Errorf("Link %s:%s -> %s:%s validation failed: %v",
				s.pair.A.DUT.Name(), s.pair.A.Port.Name(),
				s.pair.B.DUT.Name(), s.pair.B.Port.Name(), err)
		}
	}
}

func deleteSpecs(t *testing.T, specs []qualificationSpec) {
	t.Helper()
	idsByDUT := make(map[*ondatra.DUTDevice][]string)
	for _, s := range specs {
		idsByDUT[s.pair.A.DUT] = append(idsByDUT[s.pair.A.DUT], s.genID)
		idsByDUT[s.pair.B.DUT] = append(idsByDUT[s.pair.B.DUT], s.refID)
	}
	for dut, ids := range idsByDUT {
		deleteQualificationIDsBestEffort(t, dut, ids)
		if deviations.LinkQualWaitAfterDeleteRequired(dut) {
			waitForIDsRemoved(t, dut, ids)
		}
	}
}

// DeleteAllQualifications removes any pre-existing qualification sessions on a DUT.
func DeleteAllQualifications(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	listResp, err := dut.RawAPIs().GNOI(t).LinkQualification().List(ctx, &plqpb.ListRequest{})
	if err != nil {
		t.Logf("DUT %s List() during pre-clean returned: %v", dut.Name(), err)
		return
	}
	var ids []string
	for _, res := range listResp.GetResults() {
		ids = append(ids, res.GetId())
	}
	if len(ids) == 0 {
		return
	}
	deleteQualificationIDsBestEffort(t, dut, ids)
	if deviations.LinkQualWaitAfterDeleteRequired(dut) {
		waitForIDsRemoved(t, dut, ids)
	}
}

func queryCapabilities(t *testing.T, dut *ondatra.DUTDevice) *plqpb.CapabilitiesResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	resp, err := dut.RawAPIs().GNOI(t).LinkQualification().Capabilities(ctx, &plqpb.CapabilitiesRequest{})
	if err != nil {
		t.Fatalf("DUT %s Capabilities() failed: %v", dut.Name(), err)
	}
	return resp
}

func deleteQualificationIDs(t *testing.T, dut *ondatra.DUTDevice, ids []string, allowCanceled bool) {
	t.Helper()
	if len(ids) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	delResp, err := dut.RawAPIs().GNOI(t).LinkQualification().Delete(ctx, &plqpb.DeleteRequest{Ids: ids})
	if err != nil {
		t.Fatalf("DUT %s Delete(%v) failed: %v", dut.Name(), ids, err)
	}
	canceled, errs := validateDeleteResults(dut.Name(), ids, delResp, allowCanceled)
	for _, id := range canceled {
		t.Logf("DUT %s Delete(%s) canceled active qualification (code=%s)", dut.Name(), id, codes.Canceled)
	}
	for _, verr := range errs {
		t.Error(verr)
	}
}

func validateDeleteResults(dutName string, ids []string, delResp *plqpb.DeleteResponse, allowCanceled bool) ([]string, []error) {
	var canceled []string
	var errs []error
	results := delResp.GetResults()
	for _, id := range ids {
		st, ok := results[id]
		if !ok {
			errs = append(errs, fmt.Errorf("DUT %s DeleteResponse missing status for id %q", dutName, id))
			continue
		}
		code := codes.Code(st.GetCode())
		switch {
		case code == codes.OK:
		case allowCanceled && code == codes.Canceled:
			canceled = append(canceled, id)
		default:
			errs = append(errs, fmt.Errorf("DUT %s Delete(%s) code: got %d (%s), want OK", dutName, id, st.GetCode(), code))
		}
	}
	return canceled, errs
}

func deleteQualificationIDsBestEffort(t *testing.T, dut *ondatra.DUTDevice, ids []string) {
	t.Helper()
	if len(ids) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	_, err := dut.RawAPIs().GNOI(t).LinkQualification().Delete(ctx, &plqpb.DeleteRequest{Ids: ids})
	if err != nil {
		t.Logf("DUT %s Delete(%v) during cleanup returned: %v", dut.Name(), ids, err)
	}
}

func assertIDsInList(t *testing.T, dut *ondatra.DUTDevice, ids []string, wantPresent bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	listResp, err := dut.RawAPIs().GNOI(t).LinkQualification().List(ctx, &plqpb.ListRequest{})
	if err != nil {
		t.Fatalf("DUT %s List() failed: %v", dut.Name(), err)
	}
	listed := make(map[string]bool, len(listResp.GetResults()))
	for _, r := range listResp.GetResults() {
		listed[r.GetId()] = true
	}
	for _, id := range ids {
		if got := listed[id]; got != wantPresent {
			t.Errorf("DUT %s List() presence for id %q: got %v, want %v", dut.Name(), id, got, wantPresent)
		}
	}
}

func waitForIDsRemoved(t *testing.T, dut *ondatra.DUTDevice, ids []string) {
	t.Helper()
	waitForIDsInList(t, dut, ids, false, 5*time.Second, deletePollTimeout)
}

func waitForIDsInList(t *testing.T, dut *ondatra.DUTDevice, ids []string, wantPresent bool, interval, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	target := make(map[string]bool, len(ids))
	for _, id := range ids {
		target[id] = true
	}

	var lastErr error
	for {
		rpcCtx, rpcCancel := context.WithTimeout(ctx, rpcTimeout)
		listResp, err := dut.RawAPIs().GNOI(t).LinkQualification().List(rpcCtx, &plqpb.ListRequest{})
		rpcCancel()
		if err == nil && idsMatchListState(listResp, target, wantPresent) {
			return
		}
		lastErr = err
		select {
		case <-ctx.Done():
			if lastErr != nil {
				t.Logf("DUT %s List() wait (wantPresent=%v) timed out; last error: %v", dut.Name(), wantPresent, lastErr)
			}
			return
		case <-ticker.C:
		}
	}
}

func idsMatchListState(listResp *plqpb.ListResponse, target map[string]bool, wantPresent bool) bool {
	listed := make(map[string]bool, len(listResp.GetResults()))
	for _, r := range listResp.GetResults() {
		listed[r.GetId()] = true
	}
	for id := range target {
		if listed[id] != wantPresent {
			return false
		}
	}
	return true
}

func uniqueID(prefix string, idx int) string {
	return fmt.Sprintf("%s-%d-%d", prefix, idx, time.Now().UnixNano())
}
