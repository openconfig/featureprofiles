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

// Package aft_operations_ordering_test implements TE-3.9: gRIBI AFT Operations Ordering.
package aft_operations_ordering_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"

	spb "github.com/openconfig/gribi/v1/proto/service"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen = 30

	nhIndex1  = uint64(1)
	nhIndex2  = uint64(2)
	nhgIndex1 = uint64(1)
	nhgIndex2 = uint64(2)
	nhWeight  = uint64(1)

	tlPrefix = "203.0.113.0/24"

	// Scale constants
	nhOpsCount  = 10000 // 5,000 pairs of ADD + DEL
	nhgOpsCount = 9999  // 3,333 triplets of ADD + REPLACE + DEL
	tlOpsCount  = 9999  // 3,333 triplets of ADD + REPLACE + DEL
	mbbCycles   = 1000  // 1,000 cycles * 10 ops = 10,000 ops

	awaitTimeout = 2 * time.Minute
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:       "dutPort2",
		IPv4:       "192.0.2.5",
		IPv4Len:    ipv4PrefixLen,
		IPv4Sec:    "192.0.2.9",
		IPv4LenSec: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:       "atePort2",
		MAC:        "02:00:02:01:01:01",
		IPv4:       "192.0.2.6",
		IPv4Len:    ipv4PrefixLen,
		IPv4Sec:    "192.0.2.10",
		IPv4LenSec: ipv4PrefixLen,
	}

	dstIP1 = "192.0.2.6"
	dstIP2 = "192.0.2.10"
)

// configInterfaceDUT configures the DUT interfaces.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(a.IPv4Len)
	if a.IPv4Sec != "" {
		s4a2 := s4.GetOrCreateAddress(a.IPv4Sec)
		s4a2.PrefixLength = ygot.Uint8(a.IPv4LenSec)
		s4a2.Type = oc.IfIp_Ipv4AddressType_SECONDARY
	}

	return i
}

// configureDUT configures baseline interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1, dut))

	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2, dut))

	t.Cleanup(func() {
		gnmi.Delete(t, dut, gnmi.OC().Interface(p1.Name()).Config())
		gnmi.Delete(t, dut, gnmi.OC().Interface(p2.Name()).Config())
	})

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureATE configures the baseline ports on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()

	top.Ports().Add().SetName(ate.Port(t, "port1").ID())
	i1 := top.Devices().Add().SetName(ate.Port(t, "port1").ID())
	eth1 := i1.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	eth1.Connection().SetPortName(i1.Name())
	eth1.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4").
		SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).
		SetPrefix(uint32(atePort1.IPv4Len))

	top.Ports().Add().SetName(ate.Port(t, "port2").ID())
	i2 := top.Devices().Add().SetName(ate.Port(t, "port2").ID())
	eth2 := i2.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	eth2.Connection().SetPortName(i2.Name())
	eth2.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4").
		SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).
		SetPrefix(uint32(atePort2.IPv4Len))
	if atePort2.IPv4Sec != "" {
		eth2.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4Sec").
			SetAddress(atePort2.IPv4Sec).SetGateway(dutPort2.IPv4Sec).
			SetPrefix(uint32(atePort2.IPv4LenSec))
	}

	return top
}

// initGRIBIClient establishes a gRIBI connection, learns/updates the election ID to become leader,
// and ensures a clean initial state. It registers a cleanup handler to flush all entries and stop the client.
func initGRIBIClient(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) (*fluent.GRIBIClient, gribi.Uint128) {
	t.Helper()
	gribic := dut.RawAPIs().GRIBI(t)
	c := fluent.NewClient()
	c.Connection().
		WithStub(gribic).
		WithPersistence().
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(1, 0)

	c.Start(ctx, t)
	t.Cleanup(func() {
		if err := gribi.FlushAll(c); err != nil {
			t.Logf("Warning: FlushAll failed during teardown: %v", err)
		}
		c.Stop(t)
	})

	c.StartSending(ctx, t)
	subctx, cancel := context.WithTimeout(ctx, awaitTimeout)
	defer cancel()
	if err := c.Await(subctx, t); err != nil {
		t.Fatalf("Await got error during session negotiation: %v", err)
	}

	eID := gribi.BecomeLeader(t, c)
	return c, eID
}

// awaitConvergence waits for the client send and pending queues to drain.
func awaitConvergence(ctx context.Context, t *testing.T, c *fluent.GRIBIClient) {
	t.Helper()
	subctx, cancel := context.WithTimeout(ctx, awaitTimeout)
	defer cancel()
	if err := c.Await(subctx, t); err != nil {
		t.Fatalf("Await got error waiting for stream convergence: %v", err)
	}
}

// verifyResults verifies that all operations received RIB_ACK (spb.AFTResult_RIB_PROGRAMMED)
// without transport errors or rejection.
func verifyResults(t *testing.T, res []*client.OpResult, startOpID, expectedCount uint64) {
	t.Helper()
	if len(res) == 0 {
		t.Fatalf("Got 0 results, want at least %d", expectedCount)
	}

	resByOpID := make(map[uint64]*client.OpResult, len(res))
	for _, r := range res {
		resByOpID[r.OperationID] = r
	}

	var failures []string
	for opID := startOpID; opID < startOpID+expectedCount; opID++ {
		r, ok := resByOpID[opID]
		if !ok {
			failures = append(failures, fmt.Sprintf("opID %d: missing from results", opID))
			if len(failures) >= 10 {
				break
			}
			continue
		}
		if r.ClientError != "" {
			failures = append(failures, fmt.Sprintf("opID %d: client error: %s", opID, r.ClientError))
		} else if r.ServerError != "" {
			failures = append(failures, fmt.Sprintf("opID %d: server error: %s", opID, r.ServerError))
		} else if r.ProgrammingResult != spb.AFTResult_RIB_PROGRAMMED {
			failures = append(failures, fmt.Sprintf("opID %d: programming result = %v, want %v (RIB_PROGRAMMED)", opID, r.ProgrammingResult, spb.AFTResult_RIB_PROGRAMMED))
		}
		if len(failures) >= 10 {
			break
		}
	}

	if len(failures) > 0 {
		t.Fatalf("Validation failed for %d operations (showing up to 10 failures):\n%s", expectedCount, fmt.Sprint(failures))
	}
	t.Logf("Successfully validated %d operations (OpIDs %d..%d) with RIB_ACK", expectedCount, startOpID, startOpID+expectedCount-1)
}

// buildOp constructs an *spb.AFTOperation with the specified entry, opcode, opID, and electionID.
func buildOp(t testing.TB, entry fluent.GRIBIEntry, op spb.AFTOperation_Operation, opID uint64, electionID gribi.Uint128) *spb.AFTOperation {
	t.Helper()
	ep, err := entry.OpProto()
	if err != nil {
		t.Fatalf("failed to build entry proto: %v", err)
	}
	ep.Op = op
	ep.Id = opID
	ep.ElectionId = &spb.Uint128{
		Low:  electionID.Low,
		High: electionID.High,
	}
	return ep
}

// Group 1: Inter-Request Pipelined Stream (TE-3.9.1 - TE-3.9.4)

// TE-3.9.1: TestInterRequestNHChurn verifies 10,000 pipelined NextHop ADD/DEL operations across ModifyRequests.
func TestInterRequestNHChurn(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)

	c, _ := initGRIBIClient(ctx, t, dut)
	defaultVRF := deviations.DefaultNetworkInstance(dut)

	nh := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex1).WithIPAddress(dstIP1)
	delNH := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex1)

	// Stream 5,000 pairs of ADD and DEL (10,000 operations total) without intermediate await barriers.
	for i := 0; i < nhOpsCount/2; i++ {
		c.Modify().AddEntry(t, nh)
		c.Modify().DeleteEntry(t, delNH)
	}

	awaitConvergence(ctx, t, c)
	verifyResults(t, c.Results(t), 1, nhOpsCount)
}

// TE-3.9.2: TestInterRequestNHGMutation verifies 9,999 pipelined NextHopGroup ADD/REPLACE/DEL operations across ModifyRequests.
func TestInterRequestNHGMutation(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)

	c, _ := initGRIBIClient(ctx, t, dut)
	defaultVRF := deviations.DefaultNetworkInstance(dut)

	// Pre-program static base NextHops NH1 and NH2 (OpIDs 1 and 2).
	nh1 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex1).WithIPAddress(dstIP1)
	nh2 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex2).WithIPAddress(dstIP2)
	c.Modify().AddEntry(t, nh1, nh2)
	awaitConvergence(ctx, t, c)

	nhgAdd := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex1).AddNextHop(nhIndex1, nhWeight)
	nhgReplace := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex1).AddNextHop(nhIndex2, nhWeight)
	nhgDel := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex1)

	// Stream 3,333 triplets of ADD, REPLACE, and DEL (9,999 operations total).
	for i := 0; i < nhgOpsCount/3; i++ {
		c.Modify().AddEntry(t, nhgAdd)
		c.Modify().ReplaceEntry(t, nhgReplace)
		c.Modify().DeleteEntry(t, nhgDel)
	}

	awaitConvergence(ctx, t, c)
	verifyResults(t, c.Results(t), 3, nhgOpsCount)
}

// TE-3.9.3: TestInterRequestTLMutation verifies 9,999 pipelined IPv4Entry ADD/REPLACE/DEL operations across ModifyRequests.
func TestInterRequestTLMutation(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)

	c, _ := initGRIBIClient(ctx, t, dut)
	defaultVRF := deviations.DefaultNetworkInstance(dut)

	// Pre-program base NextHops NH1, NH2 (OpIDs 1..2) and NextHopGroups NHG1, NHG2 (OpIDs 3..4).
	nh1 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex1).WithIPAddress(dstIP1)
	nh2 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex2).WithIPAddress(dstIP2)
	nhg1 := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex1).AddNextHop(nhIndex1, nhWeight)
	nhg2 := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex2).AddNextHop(nhIndex2, nhWeight)
	c.Modify().AddEntry(t, nh1, nh2)
	c.Modify().AddEntry(t, nhg1, nhg2)
	awaitConvergence(ctx, t, c)

	tlAdd := fluent.IPv4Entry().WithNetworkInstance(defaultVRF).WithPrefix(tlPrefix).WithNextHopGroup(nhgIndex1)
	tlReplace := fluent.IPv4Entry().WithNetworkInstance(defaultVRF).WithPrefix(tlPrefix).WithNextHopGroup(nhgIndex2)
	tlDel := fluent.IPv4Entry().WithNetworkInstance(defaultVRF).WithPrefix(tlPrefix)

	// Stream 3,333 triplets of ADD, REPLACE, and DEL (9,999 operations total).
	for i := 0; i < tlOpsCount/3; i++ {
		c.Modify().AddEntry(t, tlAdd)
		c.Modify().ReplaceEntry(t, tlReplace)
		c.Modify().DeleteEntry(t, tlDel)
	}

	awaitConvergence(ctx, t, c)
	verifyResults(t, c.Results(t), 5, tlOpsCount)
}

// TE-3.9.4: TestInterRequestMBBPingPong verifies 10,000 pipelined cross-layer MBB operations across ModifyRequests.
func TestInterRequestMBBPingPong(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)

	c, _ := initGRIBIClient(ctx, t, dut)
	defaultVRF := deviations.DefaultNetworkInstance(dut)

	// Program initial base state: NH1 (OpID 1), NHG1->NH1 (OpID 2), TL->NHG1 (OpID 3).
	nh1 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex1).WithIPAddress(dstIP1)
	nhg1 := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex1).AddNextHop(nhIndex1, nhWeight)
	tl1 := fluent.IPv4Entry().WithNetworkInstance(defaultVRF).WithPrefix(tlPrefix).WithNextHopGroup(nhgIndex1)
	c.Modify().AddEntry(t, nh1)
	c.Modify().AddEntry(t, nhg1)
	c.Modify().AddEntry(t, tl1)
	awaitConvergence(ctx, t, c)

	nh2 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex2).WithIPAddress(dstIP2)
	nhg2 := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex2).AddNextHop(nhIndex2, nhWeight)
	tl2 := fluent.IPv4Entry().WithNetworkInstance(defaultVRF).WithPrefix(tlPrefix).WithNextHopGroup(nhgIndex2)

	delNH1 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex1)
	delNHG1 := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex1)
	delNH2 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex2)
	delNHG2 := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex2)

	// Stream 1,000 cycles (10 operations per cycle = 10,000 operations total).
	for i := 0; i < mbbCycles; i++ {
		// Cycle 1->2 (5 operations)
		c.Modify().AddEntry(t, nh2)
		c.Modify().AddEntry(t, nhg2)
		c.Modify().ReplaceEntry(t, tl2)
		c.Modify().DeleteEntry(t, delNHG1)
		c.Modify().DeleteEntry(t, delNH1)

		// Cycle 2->1 (5 operations)
		c.Modify().AddEntry(t, nh1)
		c.Modify().AddEntry(t, nhg1)
		c.Modify().ReplaceEntry(t, tl1)
		c.Modify().DeleteEntry(t, delNHG2)
		c.Modify().DeleteEntry(t, delNH2)
	}

	awaitConvergence(ctx, t, c)
	verifyResults(t, c.Results(t), 4, mbbCycles*10)
}

// Group 2: Intra-Request Batching (TE-3.9.5 - TE-3.9.8)

// TE-3.9.5: TestIntraRequestNHChurn verifies 10,000 NextHop ADD/DEL operations batched in a single ModifyRequest.
func TestIntraRequestNHChurn(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)

	c, eID := initGRIBIClient(ctx, t, dut)
	defaultVRF := deviations.DefaultNetworkInstance(dut)

	nh := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex1).WithIPAddress(dstIP1)
	delNH := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex1)

	ops := make([]*spb.AFTOperation, 0, nhOpsCount)
	var opID uint64 = 1
	for i := 0; i < nhOpsCount/2; i++ {
		ops = append(ops, buildOp(t, nh, spb.AFTOperation_ADD, opID, eID))
		opID++
		ops = append(ops, buildOp(t, delNH, spb.AFTOperation_DELETE, opID, eID))
		opID++
	}

	c.Modify().InjectRequest(t, &spb.ModifyRequest{
		Operation: ops,
	})

	awaitConvergence(ctx, t, c)
	verifyResults(t, c.Results(t), 1, nhOpsCount)
}

// TE-3.9.6: TestIntraRequestNHGMutation verifies 9,999 NextHopGroup ADD/REPLACE/DEL operations batched in a single ModifyRequest.
func TestIntraRequestNHGMutation(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)

	c, eID := initGRIBIClient(ctx, t, dut)
	defaultVRF := deviations.DefaultNetworkInstance(dut)

	// Pre-program base NextHops NH1 and NH2 (OpIDs 1 and 2).
	nh1 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex1).WithIPAddress(dstIP1)
	nh2 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex2).WithIPAddress(dstIP2)
	c.Modify().AddEntry(t, nh1, nh2)
	awaitConvergence(ctx, t, c)

	nhgAdd := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex1).AddNextHop(nhIndex1, nhWeight)
	nhgReplace := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex1).AddNextHop(nhIndex2, nhWeight)
	nhgDel := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex1)

	ops := make([]*spb.AFTOperation, 0, nhgOpsCount)
	var opID uint64 = 3
	for i := 0; i < nhgOpsCount/3; i++ {
		ops = append(ops, buildOp(t, nhgAdd, spb.AFTOperation_ADD, opID, eID))
		opID++
		ops = append(ops, buildOp(t, nhgReplace, spb.AFTOperation_REPLACE, opID, eID))
		opID++
		ops = append(ops, buildOp(t, nhgDel, spb.AFTOperation_DELETE, opID, eID))
		opID++
	}

	c.Modify().InjectRequest(t, &spb.ModifyRequest{
		Operation: ops,
	})

	awaitConvergence(ctx, t, c)
	verifyResults(t, c.Results(t), 3, nhgOpsCount)
}

// TE-3.9.7: TestIntraRequestTLMutation verifies 9,999 IPv4Entry ADD/REPLACE/DEL operations batched in a single ModifyRequest.
func TestIntraRequestTLMutation(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)

	c, eID := initGRIBIClient(ctx, t, dut)
	defaultVRF := deviations.DefaultNetworkInstance(dut)

	// Pre-program base NextHops NH1, NH2 (OpIDs 1..2) and NextHopGroups NHG1, NHG2 (OpIDs 3..4).
	nh1 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex1).WithIPAddress(dstIP1)
	nh2 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex2).WithIPAddress(dstIP2)
	nhg1 := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex1).AddNextHop(nhIndex1, nhWeight)
	nhg2 := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex2).AddNextHop(nhIndex2, nhWeight)
	c.Modify().AddEntry(t, nh1, nh2)
	c.Modify().AddEntry(t, nhg1, nhg2)
	awaitConvergence(ctx, t, c)

	tlAdd := fluent.IPv4Entry().WithNetworkInstance(defaultVRF).WithPrefix(tlPrefix).WithNextHopGroup(nhgIndex1)
	tlReplace := fluent.IPv4Entry().WithNetworkInstance(defaultVRF).WithPrefix(tlPrefix).WithNextHopGroup(nhgIndex2)
	tlDel := fluent.IPv4Entry().WithNetworkInstance(defaultVRF).WithPrefix(tlPrefix)

	ops := make([]*spb.AFTOperation, 0, tlOpsCount)
	var opID uint64 = 5
	for i := 0; i < tlOpsCount/3; i++ {
		ops = append(ops, buildOp(t, tlAdd, spb.AFTOperation_ADD, opID, eID))
		opID++
		ops = append(ops, buildOp(t, tlReplace, spb.AFTOperation_REPLACE, opID, eID))
		opID++
		ops = append(ops, buildOp(t, tlDel, spb.AFTOperation_DELETE, opID, eID))
		opID++
	}

	c.Modify().InjectRequest(t, &spb.ModifyRequest{
		Operation: ops,
	})

	awaitConvergence(ctx, t, c)
	verifyResults(t, c.Results(t), 5, tlOpsCount)
}

// TE-3.9.8: TestIntraRequestMBBPingPong verifies 10,000 cross-layer MBB operations batched in a single ModifyRequest.
func TestIntraRequestMBBPingPong(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	top := configureATE(t, ate)
	ate.OTG().PushConfig(t, top)

	c, eID := initGRIBIClient(ctx, t, dut)
	defaultVRF := deviations.DefaultNetworkInstance(dut)

	// Program initial base state: NH1 (OpID 1), NHG1->NH1 (OpID 2), TL->NHG1 (OpID 3).
	nh1 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex1).WithIPAddress(dstIP1)
	nhg1 := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex1).AddNextHop(nhIndex1, nhWeight)
	tl1 := fluent.IPv4Entry().WithNetworkInstance(defaultVRF).WithPrefix(tlPrefix).WithNextHopGroup(nhgIndex1)
	c.Modify().AddEntry(t, nh1)
	c.Modify().AddEntry(t, nhg1)
	c.Modify().AddEntry(t, tl1)
	awaitConvergence(ctx, t, c)

	nh2 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex2).WithIPAddress(dstIP2)
	nhg2 := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex2).AddNextHop(nhIndex2, nhWeight)
	tl2 := fluent.IPv4Entry().WithNetworkInstance(defaultVRF).WithPrefix(tlPrefix).WithNextHopGroup(nhgIndex2)

	delNH1 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex1)
	delNHG1 := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex1)
	delNH2 := fluent.NextHopEntry().WithNetworkInstance(defaultVRF).WithIndex(nhIndex2)
	delNHG2 := fluent.NextHopGroupEntry().WithNetworkInstance(defaultVRF).WithID(nhgIndex2)

	ops := make([]*spb.AFTOperation, 0, mbbCycles*10)
	var opID uint64 = 4
	for i := 0; i < mbbCycles; i++ {
		// Cycle 1->2 (5 operations)
		ops = append(ops, buildOp(t, nh2, spb.AFTOperation_ADD, opID, eID))
		opID++
		ops = append(ops, buildOp(t, nhg2, spb.AFTOperation_ADD, opID, eID))
		opID++
		ops = append(ops, buildOp(t, tl2, spb.AFTOperation_REPLACE, opID, eID))
		opID++
		ops = append(ops, buildOp(t, delNHG1, spb.AFTOperation_DELETE, opID, eID))
		opID++
		ops = append(ops, buildOp(t, delNH1, spb.AFTOperation_DELETE, opID, eID))
		opID++

		// Cycle 2->1 (5 operations)
		ops = append(ops, buildOp(t, nh1, spb.AFTOperation_ADD, opID, eID))
		opID++
		ops = append(ops, buildOp(t, nhg1, spb.AFTOperation_ADD, opID, eID))
		opID++
		ops = append(ops, buildOp(t, tl1, spb.AFTOperation_REPLACE, opID, eID))
		opID++
		ops = append(ops, buildOp(t, delNHG2, spb.AFTOperation_DELETE, opID, eID))
		opID++
		ops = append(ops, buildOp(t, delNH2, spb.AFTOperation_DELETE, opID, eID))
		opID++
	}

	c.Modify().InjectRequest(t, &spb.ModifyRequest{
		Operation: ops,
	})

	awaitConvergence(ctx, t, c)
	verifyResults(t, c.Results(t), 4, mbbCycles*10)
}
