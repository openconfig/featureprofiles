// Copyright 2024 Google LLC
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

package gnoi

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/deviations"
	plqpb "github.com/openconfig/gnoi/packet_link_qualification"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"google.golang.org/protobuf/types/known/durationpb"
)

// PLQTestParams defines the parameters for a hairpinned or point-to-point gNOI Packet Link
// Qualification session between a near-end generator port and a far-end reflector port.
type PLQTestParams struct {
	GeneratorPort    *ondatra.Port
	ReflectorPort    *ondatra.Port
	GeneratorQualID  string
	ReflectorQualID  string
	PacketRate       uint64
	PacketSize       uint32
	SetupDuration    time.Duration
	PreSyncDuration  time.Duration
	TestDuration     time.Duration
	PostSyncDuration time.Duration
	TeardownDuration time.Duration
}

// VerifyPLQCapabilities queries gNOI LinkQualification.Capabilities on the DUT and validates that
// packet generator and loopback reflector capabilities meet the minimum requirements.
func VerifyPLQCapabilities(t *testing.T, dut *ondatra.DUTDevice, ports []*ondatra.Port, minBps uint64, minMTU uint32, minPPS uint64) *plqpb.CapabilitiesResponse {
	t.Helper()
	plqClient := dut.RawAPIs().GNOI(t).LinkQualification()

	if devMTU := deviations.PLQGeneratorCapabilitiesMaxMTU(dut); devMTU != 0 {
		minMTU = devMTU
	}
	if devPPS := deviations.PLQGeneratorCapabilitiesMaxPPS(dut); devPPS != 0 {
		minPPS = devPPS
	}

	resp, err := plqClient.Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	if err != nil {
		t.Fatalf("LinkQualification.Capabilities() failed: %v", err)
	}
	t.Logf("LinkQualification.Capabilities() for ports %v: %v", ports, resp)

	genCap := resp.GetGenerator().GetPacketGenerator()
	if genCap == nil {
		t.Fatalf("DUT %s does not report PacketGenerator capabilities", dut.Name())
	}
	if got := genCap.GetMaxBps(); got < minBps {
		t.Errorf("PacketGenerator MaxBps: got %d, want >= %d", got, minBps)
	}
	if got := genCap.GetMaxMtu(); got < minMTU {
		t.Errorf("PacketGenerator MaxMtu: got %d, want >= %d", got, minMTU)
	}
	if got := genCap.GetMaxPps(); got < minPPS {
		t.Errorf("PacketGenerator MaxPps: got %d, want >= %d", got, minPPS)
	}

	ref := resp.GetReflector()
	asicOK := ref.GetAsicLoopback().GetMinSetupDuration().GetSeconds() >= 1 && ref.GetAsicLoopback().GetMinTeardownDuration().GetSeconds() >= 1
	pmdOK := ref.GetPmdLoopback().GetMinSetupDuration().GetSeconds() >= 1 && ref.GetPmdLoopback().GetMinTeardownDuration().GetSeconds() >= 1
	if !asicOK && !pmdOK {
		t.Errorf("Reflector neither AsicLoopback nor PmdLoopback reports valid setup/teardown durations: %v", ref)
	}
	return resp
}

// StartPLQ initiates a gNOI Packet Link Qualification session on the generator and reflector ports
// and waits for both interfaces to transition to oper-status TESTING (unless skipped by deviation).
func StartPLQ(t *testing.T, dut *ondatra.DUTDevice, params PLQTestParams) {
	t.Helper()
	plqClient := dut.RawAPIs().GNOI(t).LinkQualification()

	capResp, err := plqClient.Capabilities(context.Background(), &plqpb.CapabilitiesRequest{})
	if err != nil {
		t.Fatalf("LinkQualification.Capabilities() failed: %v", err)
	}

	packetSize := params.PacketSize
	if devMTU := deviations.PLQGeneratorCapabilitiesMaxMTU(dut); devMTU != 0 && packetSize > devMTU {
		packetSize = devMTU
	}

	nearSync := &plqpb.RPCSyncedTiming{
		PreSyncDuration:  durationpb.New(params.PreSyncDuration),
		SetupDuration:    durationpb.New(params.SetupDuration),
		Duration:         durationpb.New(params.TestDuration),
		PostSyncDuration: durationpb.New(params.PostSyncDuration),
		TeardownDuration: durationpb.New(params.TeardownDuration),
	}
	farSync := &plqpb.RPCSyncedTiming{
		PreSyncDuration:  durationpb.New(0),
		SetupDuration:    durationpb.New(params.SetupDuration + params.PreSyncDuration),
		Duration:         durationpb.New(params.TestDuration),
		PostSyncDuration: durationpb.New(params.PostSyncDuration),
		TeardownDuration: durationpb.New(params.TeardownDuration),
	}

	reflectorCfg := &plqpb.QualificationConfiguration{
		Id:            params.ReflectorQualID,
		InterfaceName: params.ReflectorPort.Name(),
		Timing: &plqpb.QualificationConfiguration_Rpc{
			Rpc: farSync,
		},
	}
	switch {
	case capResp.GetReflector().GetAsicLoopback().GetMinSetupDuration().AsDuration() > 0:
		reflectorCfg.EndpointType = &plqpb.QualificationConfiguration_AsicLoopback{
			AsicLoopback: &plqpb.AsicLoopbackConfiguration{},
		}
	case capResp.GetReflector().GetPmdLoopback().GetMinSetupDuration().AsDuration() > 0:
		reflectorCfg.EndpointType = &plqpb.QualificationConfiguration_PmdLoopback{
			PmdLoopback: &plqpb.PmdLoopbackConfiguration{},
		}
	default:
		reflectorCfg.EndpointType = &plqpb.QualificationConfiguration_AsicLoopback{
			AsicLoopback: &plqpb.AsicLoopbackConfiguration{},
		}
	}

	createReq := &plqpb.CreateRequest{
		Interfaces: []*plqpb.QualificationConfiguration{
			{
				Id:            params.GeneratorQualID,
				InterfaceName: params.GeneratorPort.Name(),
				Timing: &plqpb.QualificationConfiguration_Rpc{
					Rpc: nearSync,
				},
				EndpointType: &plqpb.QualificationConfiguration_PacketGenerator{
					PacketGenerator: &plqpb.PacketGeneratorConfiguration{
						PacketRate: params.PacketRate,
						PacketSize: packetSize,
					},
				},
			},
			reflectorCfg,
		},
	}

	createResp, err := plqClient.Create(context.Background(), createReq)
	if err != nil {
		t.Fatalf("LinkQualification.Create failed: %v", err)
	}
	t.Logf("LinkQualification.Create response: %v", createResp)
	for _, id := range []string{params.GeneratorQualID, params.ReflectorQualID} {
		status, ok := createResp.GetStatus()[id]
		if !ok {
			t.Fatalf("LinkQualification.Create response missing status for ID %s", id)
		}
		if got := status.GetCode(); got != 0 {
			t.Errorf("LinkQualification.Create status[%s] code: got %d, want 0 (message: %s)", id, got, status.GetMessage())
		}
	}

	if !deviations.SkipPlqInterfaceOperStatusCheck(dut) {
		for _, dp := range []*ondatra.Port{params.GeneratorPort, params.ReflectorPort} {
			gnmi.Await(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_TESTING)
		}
	}
}

// AwaitAndVerifyPLQCompleted waits for both generator and reflector qualification sessions to reach
// AwaitAndVerifyPLQCompleted polls gNOI LinkQualification.List until both sessions reach
// QUALIFICATION_STATE_COMPLETED, and validates zero errors, zero drops, and matching sent/received
// packet counts.
func AwaitAndVerifyPLQCompleted(t *testing.T, dut *ondatra.DUTDevice, params PLQTestParams, timeout time.Duration) {
	t.Helper()
	plqClient := dut.RawAPIs().GNOI(t).LinkQualification()
	deadline := time.Now().Add(timeout)

	for {
		listResp, err := plqClient.List(context.Background(), &plqpb.ListRequest{})
		if err != nil {
			t.Fatalf("LinkQualification.List failed: %v", err)
		}
		results := listResp.GetResults()
		allCompleted := len(results) >= 2
		for _, res := range results {
			t.Logf("PLQ List result: id=%s interface=%s state=%v", res.GetId(), res.GetInterfaceName(), res.GetState())
			if res.GetState() != plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED {
				allCompleted = false
			}
			if res.GetState() == plqpb.QualificationState_QUALIFICATION_STATE_ERROR {
				t.Fatalf("PLQ session %s on %s entered QUALIFICATION_STATE_ERROR", res.GetId(), res.GetInterfaceName())
			}
		}
		if allCompleted {
			break
		}
		if time.Until(deadline) <= 0 {
			t.Fatalf("PLQ sessions (%s, %s) did not reach QUALIFICATION_STATE_COMPLETED within %v", params.GeneratorQualID, params.ReflectorQualID, timeout)
		}
		time.Sleep(5 * time.Second)
	}

	getReq := &plqpb.GetRequest{
		Ids: []string{params.GeneratorQualID, params.ReflectorQualID},
	}
	getResp, err := plqClient.Get(context.Background(), getReq)
	if err != nil {
		t.Fatalf("LinkQualification.Get(%v) failed: %v", getReq.GetIds(), err)
	}
	t.Logf("LinkQualification.Get response: %v", getResp)

	for _, id := range []string{params.GeneratorQualID, params.ReflectorQualID} {
		res, ok := getResp.GetResults()[id]
		if !ok {
			t.Errorf("LinkQualification.Get missing result for ID %s", id)
			continue
		}
		if got, want := res.GetState(), plqpb.QualificationState_QUALIFICATION_STATE_COMPLETED; got != want {
			t.Errorf("Result[%s] state: got %v, want %v", id, got, want)
		}
		if got := res.GetStatus().GetCode(); got != 0 {
			t.Errorf("Result[%s] status.code: got %d, want 0 (message: %s)", id, got, res.GetStatus().GetMessage())
		}
		if got := res.GetPacketsError(); got != 0 {
			t.Errorf("Result[%s] packets_error: got %d, want 0", id, got)
		}
		if got := res.GetPacketsDropped(); got != 0 {
			t.Errorf("Result[%s] packets_dropped: got %d, want 0", id, got)
		}
		if id == params.ReflectorQualID && deviations.PLQReflectorStatsUnsupported(dut) {
			continue
		}
		if res.GetPacketsSent() == 0 {
			t.Errorf("Result[%s] packets_sent: got 0, want > 0", id)
		}
		if res.GetPacketsReceived() == 0 {
			t.Errorf("Result[%s] packets_received: got 0, want > 0", id)
		}
		if diff := cmp.Diff(res.GetPacketsSent(), res.GetPacketsReceived()); diff != "" {
			t.Errorf("Result[%s] packets_sent != packets_received (-sent +received):\n%s", id, diff)
		}
	}
}

// CleanupPLQSessions deletes the specified gNOI Packet Link Qualification sessions and verifies
// via LinkQualification.List that no qualification sessions remain.
func CleanupPLQSessions(t *testing.T, dut *ondatra.DUTDevice, qualIDs []string, ports []*ondatra.Port) {
	t.Helper()
	plqClient := dut.RawAPIs().GNOI(t).LinkQualification()
	delResp, err := plqClient.Delete(context.Background(), &plqpb.DeleteRequest{Ids: qualIDs})
	if err != nil {
		t.Logf("LinkQualification.Delete(%v) returned error: %v", qualIDs, err)
		return
	}
	for id, st := range delResp.GetResults() {
		if got := st.GetCode(); got != 0 {
			t.Logf("LinkQualification.Delete[%s] status code=%d message=%s", id, got, st.GetMessage())
		}
	}

	if deviations.LinkQualWaitAfterDeleteRequired(dut) {
		for _, dp := range ports {
			gnmi.Watch(t, dut, gnmi.OC().Interface(dp.Name()).OperStatus().State(), 30*time.Second, func(v *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
				st, present := v.Val()
				return present && st == oc.Interface_OperStatus_UP
			}).Await(t)
		}
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		listResp, err := plqClient.List(context.Background(), &plqpb.ListRequest{})
		if err != nil {
			t.Errorf("LinkQualification.List after Delete failed: %v", err)
			return
		}
		if len(listResp.GetResults()) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("LinkQualification.List after Delete still has %d results: %v", len(listResp.GetResults()), listResp.GetResults())
			return
		}
		time.Sleep(5 * time.Second)
	}
}
