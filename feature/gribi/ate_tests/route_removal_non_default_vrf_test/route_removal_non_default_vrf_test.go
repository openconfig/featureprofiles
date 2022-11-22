// Copyright 2022 Google LLC
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

package route_removal_non_default_vrf_test

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry/ateflow"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	gpb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1,
// dut:port2 -> ate:port2.
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.4/30
//
//   * Destination network for traffic test: 198.51.100.0/24

const (
	ateDstNetEntryNonDefault = "198.51.100.0/24" // IP Entry to be injected in non-default network instance
	ateDstNetEntryDefault    = "203.0.113.0/24"  // IP Entry to be injected in default network instance
	nonDefaultVRF            = "VRF-1"           // Name of non-default network instance
	nhIndex                  = 1
	nhgIndex                 = 42
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
	}
)

// TestRouteRemovalNonDefaultVRFFlush test flush with the following operations
// 1. Flush request from clientA (the primary client) non default VRF should succeed.
// 2. Flush request from clientB (not a primary client) non default VRF should fail.
// 3. Failover the primary role from clientA to clientB.
// 4. Flush from clientB non default VRF should succeed.
// 5. Flush from default VRF should not delete all the gRIBI objects like NH and NHG.
func TestRouteRemovalNonDefaultVRFFlush(t *testing.T) {
	ctx := context.Background()

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	ateTop := configureATE(t, ate)

	ateTop.Push(t).StartProtocols(t)
	configureNetworkInstance(t, dut)

	// Configure the gRIBI client clientA and make it leader.
	clientA := &gribi.Client{
		DUT:         dut,
		Persistence: true,
	}

	defer clientA.Close(t)

	t.Log("Establish gRIBI clientA connection with PERSISTENCE set to TRUE")
	if err := clientA.Start(t); err != nil {
		t.Fatalf("gRIBI Connection for clientA could not be established")
	}
	// Make clientA leader and get the leader electionID
	clientAElectionID := clientA.BecomeLeader(t)

	// Configure the gRIBI client clientB with election ID of (leader_election_id - 1)
	clientB := &gribi.Client{
		DUT:         dut,
		Persistence: true,
	}

	defer clientB.Close(t)

	// Flush all entries after test. clientA or clientB doesn't matter since we use Election Override in FlushAll.
	defer clientB.FlushAll(t)

	t.Log("Establish gRIBI clientB connection with PERSISTENCE set to TRUE")
	if err := clientB.Start(t); err != nil {
		t.Fatalf("gRIBI Connection for clientB could not be established")
	}

	// clientB electionID is one less than clientA electionID
	clientBElectionID := clientAElectionID.Decrement()
	clientB.UpdateElectionID(t, clientBElectionID)

	t.Log("Inject an IPv4Entry for 198.51.100.0/24 into VRF-1, with its referenced NHG and NH in the default routing-instance pointing to ATE port-2")
	// clientA is primary client.
	injectEntries(ctx, t, dut, clientA, nonDefaultVRF, ateDstNetEntryNonDefault)

	t.Run("flushNonDefaultVrfclientA", func(t *testing.T) {
		t.Log("Flush request from clientA (the primary client) non default VRF should succeed.")
		flushNonDefaultVrfPrimary(ctx, t, dut, clientA, ate, ateTop)
	})

	t.Log("Re-inject entry for 198.51.100.0/24 in VRF-1 from gRIBI-A")
	injectEntries(ctx, t, dut, clientA, nonDefaultVRF, ateDstNetEntryNonDefault)

	t.Run("flushNonDefaultVrfclientB", func(t *testing.T) {
		t.Log("Flush request from clientB (not a primary client) non default VRF should fail.")
		flushNonDefaultVrfSecondary(ctx, t, dut, clientB, ate, ateTop)
	})

	// Make clientB the leader
	clientB.BecomeLeader(t)

	// clientB is now primary client.
	t.Run("flushNonDefaultVrfFailover", func(t *testing.T) {
		t.Log("Flush request from clientB (primary client) non default VRF should succeed.")
		flushNonDefaultVrfFailover(ctx, t, dut, clientB, ate, ateTop)
	})

	t.Log("Inject entry for 198.51.100.0/24 in VRF-1 from gRIBI-B. This function also verifies entry via telemetry.")
	injectIPEntry(ctx, t, dut, clientB, nonDefaultVRF, ateDstNetEntryNonDefault)

	t.Log("Inject entry for 203.0.113.0/24 in default VRF from gRIBI-B. This function also verifies entry via telemetry.")
	injectIPEntry(ctx, t, dut, clientB, *deviations.DefaultNetworkInstance, ateDstNetEntryDefault)

	t.Run("flushNonZeroReference", func(t *testing.T) {
		t.Log("After re-injecting entries, flush RPC from gRIBI-B for default VRF expected to return NON_ZERO_REFERENCE_REMAIN result.")
		flushNonZeroReference(ctx, t, dut, clientB, ate, ateTop)
	})
}

// flushNonDefaultVrfPrimary issues flush request from clientA (the primary client) non default VRF should succeed.
func flushNonDefaultVrfPrimary(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client *gribi.Client, ate *ondatra.ATEDevice, ateTop *ondatra.ATETopology) {

	t.Log("Test traffic between ATE port-1 and ATE port-2 for destinations within 198.51.100.0/24")
	srcEndPoint := ateTop.Interfaces()[atePort1.Name]
	dstEndPoint := ateTop.Interfaces()[atePort2.Name]

	flowPath := testTraffic(t, ate, ateTop, srcEndPoint, dstEndPoint)
	if got := flowPath.LossPct().Get(t); got > 0 {
		t.Errorf("LossPct for flow got %g, want 0", got)
	} else {
		t.Log("Traffic can be forwarded between ATE port-1 and ATE port-2")
	}
	leftEntries := checkNIHasNEntries(ctx, t, client.Fluent(t), nonDefaultVRF)
	t.Logf("Network instance has %d entry/entries, wanted: %d", leftEntries, 3)

	t.Log("Issue flush RPC from gRIBI-A")
	if _, err := gribi.Flush(client.Fluent(t), client.ElectionID(), nonDefaultVRF); err != nil {
		t.Errorf("Unexpected error from flush, got: %v", err)
	}

	t.Log("After flush, left entry should be 0, and packets can no longer be forwarded")
	flowPath = testTraffic(t, ate, ateTop, srcEndPoint, dstEndPoint)
	if got := flowPath.LossPct().Get(t); got == 0 {
		t.Error("Traffic can still be forwarded between ATE port-1 and ATE port-2")
	} else {
		t.Log("Traffic can not be forwarded between ATE port-1 and ATE port-2")
	}
	if got, want := checkNIHasNEntries(ctx, t, client.Fluent(t), nonDefaultVRF), 0; got != want {
		t.Errorf("Network instance has %d entry/entries, wanted: %d", leftEntries, 0)
	}
}

// flushNonDefaultVrfSecondary issues flush request from clientB (not a primary client) non default VRF should fail with NOT_PRIMARY error.
func flushNonDefaultVrfSecondary(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client *gribi.Client, ate *ondatra.ATEDevice, ateTop *ondatra.ATETopology) {

	t.Log("Issue Flush from gRIBI-B expected to fail with NOT_PRIMARY error")
	flushRes, flushErr := gribi.Flush(client.Fluent(t), client.ElectionID(), nonDefaultVRF)
	if flushErr == nil {
		t.Errorf("Flush should return an error, got response: %v", flushRes)
	}
	validateNotPrimaryError(t, dut, flushErr)
}

// flushNonDefaultVrfFailover updates clientB to become primary. Flush request from clientB (the primary client) non default VRF should succeed.
func flushNonDefaultVrfFailover(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, clientB *gribi.Client, ate *ondatra.ATEDevice, ateTop *ondatra.ATETopology) {

	t.Log("Test traffic between ATE port-1 and ATE port-2 for destinations within 198.51.100.0/24")
	srcEndPoint := ateTop.Interfaces()[atePort1.Name]
	dstEndPoint := ateTop.Interfaces()[atePort2.Name]

	t.Log("Flush should be successful and 0 entry left")
	if _, err := gribi.Flush(clientB.Fluent(t), clientB.ElectionID(), nonDefaultVRF); err != nil {
		t.Errorf("Unexpected error from flush, got: %v", err)
	}
	if got, want := checkNIHasNEntries(ctx, t, clientB.Fluent(t), nonDefaultVRF), 0; got != want {
		t.Errorf("Network instance has %d entry/entries, wanted: %d", got, want)
	}

	t.Log("After flush, left entry should be 0, and packets can no longer be forwarded")
	flowPath := testTraffic(t, ate, ateTop, srcEndPoint, dstEndPoint)
	if got := flowPath.LossPct().Get(t); got == 0 {
		t.Error("Traffic can still be forwarded between ATE port-1 and ATE port-2")
	} else {
		t.Log("Traffic stopped as expected after flush between ATE port-1 and ATE port-2")
	}
}

// flushNonZeroReference verified behaviour after flush operation issue for default VRF. It is expected to NOT delete all the gRIBI objects like NH and NHG.
func flushNonZeroReference(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, clientB *gribi.Client, ate *ondatra.ATEDevice, ateTop *ondatra.ATETopology) {

	t.Log("Test traffic between ATE port-1 and ATE port-2 for destinations within 198.51.100.0/24")
	srcEndPoint := ateTop.Interfaces()[atePort1.Name]
	dstEndPoint := ateTop.Interfaces()[atePort2.Name]

	flowPath := testTraffic(t, ate, ateTop, srcEndPoint, dstEndPoint)
	if got := flowPath.LossPct().Get(t); got > 0 {
		t.Errorf("LossPct for flow got %g, want 0", got)
	} else {
		t.Log("Traffic can be forwarded between ATE port-1 and ATE port-2")
	}

	t.Log("Issue Flush RPC from gRIBI-B for default VRF. It expected to return NON_ZERO_REFERENCE_REMAIN result.")
	flushRes, _ := gribi.Flush(clientB.Fluent(t), clientB.ElectionID(), *deviations.DefaultNetworkInstance)

	wantRes := &gpb.FlushResponse{
		Result: gpb.FlushResponse_NON_ZERO_REFERENCE_REMAIN,
	}
	if flushRes.Result != wantRes.Result {
		t.Errorf("Flush operation did not return NON_ZERO_REFERENCE_REMAIN output, got %s", flushRes.Result)
	} else {
		t.Log("Flush operation successfuly returned NON_ZERO_REFERENCE_REMAIN as expected after deleting an IPv4 entry from default network instance.")
	}

	t.Log("Ensure that the IPEntry 198.51.100.0/24 (ateDstNetEntryNonDefault) is not removed, by validating packet forwarding and telemetry.")
	flowPath = testTraffic(t, ate, ateTop, srcEndPoint, dstEndPoint)
	if got := flowPath.LossPct().Get(t); got > 0 {
		t.Errorf("LossPct for flow got %g, want 0", got)
	} else {
		t.Log("Traffic can be forwarded between ATE port-1 and ATE port-2")
	}

	entry := verifyEntry(t, dut, nonDefaultVRF, ateDstNetEntryNonDefault)
	if !entry {
		t.Errorf("ipv4-entry/state/prefix does not contain entry, expected: %s", ateDstNetEntryNonDefault)
	} else {
		t.Logf("IP Entry for %s has NOT been removed from network instance: %s as confirmed from telemetry.", ateDstNetEntryNonDefault, nonDefaultVRF)
	}

	t.Log("Ensure that 203.0.113.0/24 (ateDstNetEntryDefault) has been removed by validating telemetry.")
	entry = verifyEntry(t, dut, *deviations.DefaultNetworkInstance, ateDstNetEntryDefault)
	if entry {
		t.Errorf("ipv4-entry/state/prefix contains entry %s, expected no entry", ateDstNetEntryDefault)
	} else {
		t.Logf("IP Entry for %s has been successfully removed from network instance: %s as confirmed from telemetry.", ateDstNetEntryDefault, *deviations.DefaultNetworkInstance)
	}
}

// configureDUT configures port1-2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name()))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name()))

}

// configureATE configures port1, port2 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) *ondatra.ATETopology {
	t.Helper()
	top := ate.Topology().New()

	p1 := ate.Port(t, "port1")
	i1 := top.AddInterface(atePort1.Name).WithPort(p1)
	i1.IPv4().
		WithAddress(atePort1.IPv4CIDR()).
		WithDefaultGateway(dutPort1.IPv4)

	p2 := ate.Port(t, "port2")
	i2 := top.AddInterface(atePort2.Name).WithPort(p2)
	i2.IPv4().
		WithAddress(atePort2.IPv4CIDR()).
		WithDefaultGateway(dutPort2.IPv4)

	return top
}

// configureNetworkInstance configures network instance.
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {

	nonDefaultNI := networkInstance(t, nonDefaultVRF)
	p1 := dut.Port(t, "port1")
	niIntf := nonDefaultNI.GetOrCreateInterface(p1.Name())
	niIntf.Subinterface = ygot.Uint32(0)
	niIntf.Interface = ygot.String(p1.Name())

	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(nonDefaultVRF).Config(), nonDefaultNI)
}

// networkInstance creates an OpenConfig network instance with the specified name
func networkInstance(t *testing.T, name string) *oc.NetworkInstance {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(name)
	ni.Description = ygot.String("Non Default routing instance created for testing")
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	ni.Enabled = ygot.Bool(true)
	ni.EnabledAddressFamilies = []oc.E_Types_ADDRESS_FAMILY{oc.Types_ADDRESS_FAMILY_IPV4, oc.Types_ADDRESS_FAMILY_IPV6}
	return ni
}

// testTraffic generates traffic flow from source network to
// destination network via srcEndPoint to dstEndPoint and checks for
// packet loss.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology, srcEndPoint, dstEndPoint *ondatra.Interface) *ateflow.FlowPath {
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.DstAddressRange().
		WithMin("198.51.100.0").
		WithMax("198.51.100.254").
		WithCount(255)

	flow := ate.Traffic().NewFlow("Flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoint).
		WithHeaders(ethHeader, ipv4Header)

	ate.Traffic().Start(t, flow)
	time.Sleep(15 * time.Second)
	ate.Traffic().Stop(t)

	flowPath := ate.Telemetry().Flow(flow.Name())
	return flowPath
}

// verifyEntry checks if the entry is active through AFT Telemetry.
func verifyEntry(t *testing.T, dut *ondatra.DUTDevice, networkInstanceName string, ateDstNetCIDR string) bool {
	ipv4Entry := gnmi.OC().NetworkInstance(networkInstanceName).Afts().Ipv4Entry(ateDstNetCIDR)
	got := gnmi.Lookup(t, dut, ipv4Entry.Prefix().State())
	prefix, present := got.Val()
	return present && prefix == ateDstNetCIDR
}

// injectEntries adds a fully referenced IP Entry, NH and NHG.
func injectEntries(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client *gribi.Client, networkInstanceName string, ateDstNetCIDR string) {
	t.Logf("Add an IPv4Entry for %s pointing to ATE port-2 via gRIBI client", ateDstNetCIDR)
	client.AddNH(t, nhIndex, atePort2.IPv4, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	client.AddNHG(t, nhgIndex, map[uint64]uint64{nhIndex: 1}, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
	client.AddIPv4(t, ateDstNetCIDR, nhgIndex, networkInstanceName, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)
}

// injectIPEntry adds only IPv4 entry to the specified network instance referencing to the nhgid, to the VRF.
func injectIPEntry(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client *gribi.Client, networkInstanceName string, ateDstNetCIDR string) {
	t.Logf("Add an IPv4Entry for %s via gRIBI client's %s network instance", ateDstNetCIDR, networkInstanceName)
	client.AddIPv4(t, ateDstNetCIDR, nhgIndex, networkInstanceName, *deviations.DefaultNetworkInstance, fluent.InstalledInRIB)

	// After adding the entry, verify the entry is active through AFT Telemetry.
	ipv4Path := gnmi.OC().NetworkInstance(networkInstanceName).Afts().Ipv4Entry(ateDstNetCIDR)
	if got, ok := gnmi.Watch(t, dut, ipv4Path.Prefix().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		prefix, present := val.Val()
		return present && prefix == ateDstNetCIDR
	}).Await(t); !ok {
		t.Errorf("ipv4-entry/state/prefix got %v, want %s", got, ateDstNetCIDR)
	}
}

// validateNotPrimaryError validates the canonical and exact error details for flush operation of non-primary client.
func validateNotPrimaryError(t *testing.T, dut *ondatra.DUTDevice, flushErr error) {
	s, ok := status.FromError(flushErr)
	if !ok {
		t.Fatalf("received invalid error from server, got: %v", flushErr)
	}
	if got, want := s.Code(), codes.FailedPrecondition; got != want {
		t.Fatalf("did not get the expected canonical error code from server, got code: %s (error: %v), want: %s", got, flushErr, want)
	}
	if len(s.Details()) != 1 {
		t.Fatalf("got more than 1 error details, got: %v", flushErr)
	}
	gotD, ok := s.Details()[0].(*gpb.FlushResponseError)
	if !ok {
		t.Fatalf("did not get the expected error details type, got: %T, want: *gpb.FlushResponseError", s.Details()[0])
	}
	wantD := &gpb.FlushResponseError{
		Status: gpb.FlushResponseError_NOT_PRIMARY,
	}
	if !proto.Equal(gotD, wantD) {
		t.Fatalf("did not get the exact error details, got: %s, want: %s", prototext.Format(gotD), prototext.Format(wantD))
	}
}

// checkNIHasNEntries uses the Get RPC to validate that the network instance named ni.
func checkNIHasNEntries(ctx context.Context, t *testing.T, c *fluent.GRIBIClient, ni string) int {
	t.Helper()
	gr, err := c.Get().
		WithNetworkInstance(ni).
		WithAFT(fluent.AllAFTs).
		Send()

	if err != nil {
		t.Fatalf("Unexpected error from get, got: %v", err)
	}
	return len(gr.GetEntry())
}
