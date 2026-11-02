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

// Package p4rt_device_down_test implements P4RT-1.3: P4RT behavior when a
// device/node is down. See README.md in this directory for the test plan; every
// step of the README procedure is referenced in the code below by its ID
// (e.g. "P4RT-1.3.1 Step 3").
package p4rt_device_down_test

import (
	"flag"
	"fmt"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
	p4configpb "github.com/p4lang/p4runtime/go/p4/config/v1"
	p4pb "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/testing/protocmp"
)

// TestMain is the entry point for the go test runner. Environment setup needs a
// *testing.T and a reserved testbed, so it is performed by setupEnvironment at the
// start of TestP4rtDeviceDown.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen = 30

	// P4RT openconfig node-id (device_id) values from the README "Test environment setup".
	deviceID1 = uint64(111) // device on Linecard 1 (stays up).
	deviceID2 = uint64(222) // device on Linecard 2 (brought down / up).
	// invalidDeviceID is an unconfigured device_id used in P4RT-1.3.1 Step 6.
	invalidDeviceID = uint64(999)

	// P4RT port ids (/interfaces/interface/config/id) of DUT port-1 and port-2.
	portID1 = uint32(20)
	portID2 = uint32(21)

	electionIDHigh = uint64(0)
	electionIDLow  = uint64(100)
	pipelineCookie = uint64(159)

	// lldpEtherType is the ethertype matched by the AclWbbIngressTableEntry (README Step 3).
	lldpEtherType = uint16(0x88cc)

	// linecardTimeout bounds the gnmi.Await/Watch calls for linecard power transitions.
	// Linecards of some modular DUTs take more than 15 minutes to boot after
	// being powered on.
	linecardTimeout = 25 * time.Minute
	// nodeIDTimeout bounds the gnmi.Await for integrated-circuit node-id state convergence.
	nodeIDTimeout = 2 * time.Minute
	// arbitrationTimeout bounds the wait for a MasterArbitrationUpdate outcome.
	arbitrationTimeout = 30 * time.Second
	// streamTermTimeout bounds the wait for a server-side StreamChannel termination.
	// The wait returns as soon as the termination is observed.
	streamTermTimeout = 30 * time.Second
	// notFoundTimeout bounds how long an RPC to a device that just went down is
	// retried until the P4RT server returns NOT_FOUND (README "subsequent" RPCs).
	notFoundTimeout = 60 * time.Second
	// notFoundPollInterval is the delay between retries within notFoundTimeout.
	notFoundPollInterval = 5 * time.Second
)

var (
	// p4InfoFile is the WBB P4Info sent with SetForwardingPipelineConfig.
	p4InfoFile = flag.String("p4info_file_location", "../../data/wbb.p4info.pb.txt", "Path to the p4info file.")

	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
	}
)

// p4rtNode describes one DUT port together with the P4RT integrated circuit
// (NPU) it belongs to and that circuit's parent linecard.
type p4rtNode struct {
	portID string // Ondatra port ID (e.g. "port1"), shared by the DUT and ATE.
	icName string // INTEGRATED_CIRCUIT component carrying the P4RT node-id.
	lcName string // Parent LINECARD component of icName.
}

// streamRef identifies a StreamChannel created by the test so it can be torn
// down when it is no longer needed.
type streamRef struct {
	client   *p4rt_client.P4RTClient
	name     string
	deviceID uint64
}

// testEnv holds the state shared by the P4RT-1.3.x subtests and the cleanup.
type testEnv struct {
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	node1  p4rtNode // device_id = 111 (Linecard 1).
	node2  p4rtNode // device_id = 222 (Linecard 2).
	p4Info *p4configpb.P4Info

	// primary1 is the primary P4RT client of device_id 111.
	primary1 *p4rt_client.P4RTClient
	// streams records every StreamChannel created by the test.
	streams []streamRef
}

// TestP4rtDeviceDown implements P4RT-1.3 from the README:
//   - "Test environment setup" (setupEnvironment)
//   - "P4RT-1.3.1": P4RT server handling of StreamChannel and RPCs for a down device.
//   - "P4RT-1.3.2": P4RT server behavior when the device state transitions.
func TestP4rtDeviceDown(t *testing.T) {
	env := setupEnvironment(t)

	t.Run("P4RT-1.3.1", func(t *testing.T) {
		testDownDevice(t, env)
	})

	t.Run("P4RT-1.3.2", func(t *testing.T) {
		testDeviceStateTransitions(t, env)
	})
}

// setupEnvironment implements README "Test environment setup":
//   - Connect ATE port-1 and port-2 to DUT port-1 and port-2 respectively.
//   - Configure P4RT id and node-id (device_id) with two interfaces on different
//     integrated circuits: 1st Linecard device_id = 111, 2nd Linecard device_id = 222.
//   - Verify via gNMI that the oper-status of both components is ACTIVE.
//
// It also registers a t.Cleanup that restores the DUT state modified by the test.
func setupEnvironment(t *testing.T) *testEnv {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// README "Required DUT platform": MFF. Disabling Linecard 2 while device_id 111
	// stays up requires at least two linecards.
	if lcs := components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_LINECARD); len(lcs) < 2 {
		t.Skipf("Test requires a modular (MFF) DUT with at least two linecards, found %d: %v", len(lcs), lcs)
	}

	// Test environment setup: select two interfaces on different integrated
	// circuits whose parent linecards are also different, so that disabling
	// Linecard 2 does not affect device_id 111.
	node1, node2 := selectPortsOnDistinctLinecards(t, dut)

	env := &testEnv{
		dut:    dut,
		ate:    ate,
		node1:  node1,
		node2:  node2,
		p4Info: loadP4Info(t),
	}

	// Test environment setup: configure P4RT node-id (device_id) on both ICs.
	configureDeviceIDs(t, dut, node1, node2)

	// Test environment setup: configure DUT port-1/port-2 with P4RT port ids and
	// connect ATE port-1/port-2 to them. env.cleanup is registered after
	// configureDeviceIDs and configureDUT so that Linecard 2 is powered back on
	// before the interface configs and integrated-circuit node-ids are restored
	// (cleanups run in reverse order).
	configureDUT(t, dut, node1.portID, node2.portID)
	t.Cleanup(func() { env.cleanup(t) })
	ate.OTG().PushConfig(t, configureATE(t, ate, node1.portID, node2.portID))

	// Test environment setup: verify via gNMI that the oper-status of both
	// components is ACTIVE. The components of device_id 111 and 222 are their
	// parent linecards and, where the DUT reports it, their integrated circuits
	// (see operStatusComponents). P4RT-1.3.1 Step 1 starts from this state.
	t.Log("Test environment setup: verifying oper-status ACTIVE for both components")
	for _, name := range slices.Concat(operStatusComponents(dut, node1), operStatusComponents(dut, node2)) {
		if got, ok := components.AwaitOperStatus(t, dut, name, linecardTimeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE); !ok {
			t.Fatalf("Test environment setup: component %s oper-status got %v, want %v", name, got, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
		}
	}
	return env
}

// testDownDevice implements README "P4RT-1.3.1 - Verify that the P4RT server
// handles StreamChannel and RPCs for a down device".
func testDownDevice(t *testing.T, env *testEnv) {
	dut := env.dut
	// The StreamChannels opened in P4RT-1.3.1 for the down device_id 222 and the
	// unconfigured device_id 999 are torn down when this subtest ends, so that a
	// session accepted by the server cannot hold the election id used by the
	// P4RT-1.3.2 arbitration for device_id 222.
	t.Cleanup(func() { env.destroyStreams(deviceID2, invalidDeviceID) })

	// P4RT-1.3.1 Step 1 - Disable Linecard 2 and verify status.
	//   * Disable Linecard 2 (device_id = 222) by setting power-admin-state to POWER_DISABLED.
	//   * Verify oper-status of the component transitions to DISABLED or INACTIVE.
	t.Logf("P4RT-1.3.1 Step 1 - Disable Linecard 2 (%s, device_id=%d) and verify status", env.node2.lcName, deviceID2)
	disableLinecard(t, env, env.node2)

	// P4RT-1.3.1 Step 2 - Send MasterArbitrationUpdate and Pipeline Config.
	t.Log("P4RT-1.3.1 Step 2 - Send MasterArbitrationUpdate and Pipeline Config")
	env.primary1 = newP4RTClient(t, dut)
	client2 := newP4RTClient(t, dut)

	// P4RT-1.3.1 Step 2: establish a StreamChannel and send MasterArbitrationUpdate
	// for device_id = 111 (needed to become primary before pushing the pipeline).
	if err := env.arbitrate(env.primary1, "p4rt_1_3_1_dev111", deviceID1); err != nil {
		t.Fatalf("P4RT-1.3.1 Step 2: MasterArbitrationUpdate for available device_id %d failed: %v", deviceID1, err)
	}
	// P4RT-1.3.1 Step 2: attempt to establish a StreamChannel and send a
	// MasterArbitrationUpdate for device_id = 222; verify it is rejected or
	// returns NOT_FOUND.
	if err := env.arbitrate(client2, "p4rt_1_3_1_dev222", deviceID2); err == nil {
		t.Errorf("P4RT-1.3.1 Step 2: MasterArbitrationUpdate for down device_id %d was accepted, want rejected or NOT_FOUND", deviceID2)
	}

	// P4RT-1.3.1 Step 2: send the WBB P4Info via SetForwardingPipelineConfig for both
	// device_ids; verify success for 111 and NOT_FOUND for 222.
	sfpcCases := []struct {
		desc     string
		client   *p4rt_client.P4RTClient
		deviceID uint64
		wantOK   bool
	}{
		{desc: "available device_id 111", client: env.primary1, deviceID: deviceID1, wantOK: true},
		{desc: "down device_id 222", client: client2, deviceID: deviceID2, wantOK: false},
	}
	for _, tc := range sfpcCases {
		err := setForwardingPipeline(tc.client, tc.deviceID, env.p4Info)
		if tc.wantOK {
			if err != nil {
				t.Fatalf("P4RT-1.3.1 Step 2: SetForwardingPipelineConfig for %s failed: %v", tc.desc, err)
			}
			verifyForwardingPipeline(t, tc.client, tc.deviceID, env.p4Info)
			continue
		}
		if err := p4rtutils.CheckRPCErrorNotFound(err); err != nil {
			t.Errorf("P4RT-1.3.1 Step 2: SetForwardingPipelineConfig for %s: %v", tc.desc, err)
		}
	}

	// P4RT-1.3.1 Step 3 - Send Write RPC.
	//   * Install the AclWbbIngressTableEntry for LLDP (ethertype 0x88CC) for both device_ids.
	//   * Verify success for device_id 111 and NOT_FOUND for device_id 222.
	t.Log("P4RT-1.3.1 Step 3 - Send Write RPC")
	writeCases := []struct {
		desc     string
		client   *p4rt_client.P4RTClient
		deviceID uint64
		wantOK   bool
	}{
		{desc: "available device_id 111", client: env.primary1, deviceID: deviceID1, wantOK: true},
		{desc: "down device_id 222", client: client2, deviceID: deviceID2, wantOK: false},
	}
	for _, tc := range writeCases {
		// The deletion is registered before the write so that it runs even if the
		// test fails early. For the down device_id the write is expected to fail, so
		// the deletion is best effort.
		t.Cleanup(func() {
			if err := writeLLDPEntry(tc.client, tc.deviceID, p4pb.Update_DELETE); err != nil && tc.wantOK {
				t.Errorf("Cleanup: deleting LLDP entry on device_id %d failed: %v", tc.deviceID, err)
			}
		})
		err := writeLLDPEntry(tc.client, tc.deviceID, p4pb.Update_INSERT)
		if tc.wantOK {
			if err != nil {
				t.Fatalf("P4RT-1.3.1 Step 3: Write for %s failed: %v", tc.desc, err)
			}
			continue
		}
		if err := p4rtutils.CheckRPCErrorNotFound(err); err != nil {
			t.Errorf("P4RT-1.3.1 Step 3: Write for %s: %v", tc.desc, err)
		}
	}

	// P4RT-1.3.1 Step 4 - Send Read RPC.
	//   * Read back the installed table entries for both device_ids.
	//   * Verify success for device_id 111 and NOT_FOUND for device_id 222.
	t.Log("P4RT-1.3.1 Step 4 - Send Read RPC")
	readCases := []struct {
		desc     string
		client   *p4rt_client.P4RTClient
		deviceID uint64
		wantOK   bool
	}{
		{desc: "available device_id 111", client: env.primary1, deviceID: deviceID1, wantOK: true},
		{desc: "down device_id 222", client: client2, deviceID: deviceID2, wantOK: false},
	}
	for _, tc := range readCases {
		entities, err := readTableEntries(tc.client, tc.deviceID)
		if tc.wantOK {
			if err != nil {
				t.Errorf("P4RT-1.3.1 Step 4: Read for %s failed: %v", tc.desc, err)
			} else if err := verifyLLDPEntryPresent(entities); err != nil {
				t.Errorf("P4RT-1.3.1 Step 4: Read for %s: %v", tc.desc, err)
			}
			continue
		}
		if err := p4rtutils.CheckRPCErrorNotFound(err); err != nil {
			t.Errorf("P4RT-1.3.1 Step 4: Read for %s: %v", tc.desc, err)
		}
	}

	// P4RT-1.3.1 Step 5 - Verify PacketOut to Down Device.
	//   * Send a PacketOut over the StreamChannel destined for device_id = 222.
	//   * Verify the server drops it or returns an error.
	t.Log("P4RT-1.3.1 Step 5 - Verify PacketOut to Down Device")
	verifyPacketOutToDownDevice(t, env, client2)

	// P4RT-1.3.1 Step 6 - Invalid Device ID.
	//   * Send MasterArbitrationUpdate, SetForwardingPipelineConfig, Write and Read
	//     RPCs to the unconfigured device_id 999.
	//   * Verify they all return NOT_FOUND.
	t.Logf("P4RT-1.3.1 Step 6 - Invalid Device ID (device_id=%d)", invalidDeviceID)
	client3 := newP4RTClient(t, dut)
	invalidCases := []struct {
		rpc  string
		call func() error
	}{
		{
			rpc:  "MasterArbitrationUpdate",
			call: func() error { return env.arbitrate(client3, "p4rt_1_3_1_dev999", invalidDeviceID) },
		},
		{
			rpc:  "SetForwardingPipelineConfig",
			call: func() error { return setForwardingPipeline(client3, invalidDeviceID, env.p4Info) },
		},
		{
			rpc:  "Write",
			call: func() error { return writeLLDPEntry(client3, invalidDeviceID, p4pb.Update_INSERT) },
		},
		{
			rpc: "Read",
			call: func() error {
				_, err := readTableEntries(client3, invalidDeviceID)
				return err
			},
		},
	}
	for _, tc := range invalidCases {
		if err := p4rtutils.CheckRPCErrorNotFound(tc.call()); err != nil {
			t.Errorf("P4RT-1.3.1 Step 6: %s for invalid device_id %d: %v", tc.rpc, invalidDeviceID, err)
		}
	}
}

// testDeviceStateTransitions implements README "P4RT-1.3.2 - Verify P4RT server
// behavior when device state transitions".
func testDeviceStateTransitions(t *testing.T, env *testEnv) {
	// P4RT-1.3.2 Step 1 - State Transition (Down to Up).
	//   * Re-enable Linecard 2 (device_id = 222) via gNMI.
	//   * Verify oper-status of the component transitions back to ACTIVE.
	//   * Verify the P4RT server now accepts Mastership, SetForwardingPipelineConfig,
	//     Write and Read for device_id = 222.
	t.Logf("P4RT-1.3.2 Step 1 - State Transition (Down to Up): re-enabling Linecard 2 (%s)", env.node2.lcName)
	enableLinecard(t, env, env.node2)

	client2 := newP4RTClient(t, env.dut)
	// P4RT-1.3.2 Step 1: Mastership (MasterArbitrationUpdate) is accepted.
	if err := env.arbitrate(client2, "p4rt_1_3_2_dev222", deviceID2); err != nil {
		t.Fatalf("P4RT-1.3.2 Step 1: MasterArbitrationUpdate for re-enabled device_id %d failed: %v", deviceID2, err)
	}
	// P4RT-1.3.2 Step 1: SetForwardingPipelineConfig is accepted.
	if err := setForwardingPipeline(client2, deviceID2, env.p4Info); err != nil {
		t.Fatalf("P4RT-1.3.2 Step 1: SetForwardingPipelineConfig for re-enabled device_id %d failed: %v", deviceID2, err)
	}
	verifyForwardingPipeline(t, client2, deviceID2, env.p4Info)
	// P4RT-1.3.2 Step 1: Write is accepted. The deletion is registered before the
	// write so that it runs even if the test fails early. Linecard 2 is powered off
	// again in Step 2, so the entry is deleted once the linecard is back up.
	t.Cleanup(func() { env.restoreDevice2(t) })
	if err := writeLLDPEntry(client2, deviceID2, p4pb.Update_INSERT); err != nil {
		t.Fatalf("P4RT-1.3.2 Step 1: Write for re-enabled device_id %d failed: %v", deviceID2, err)
	}
	// P4RT-1.3.2 Step 1: Read is accepted and returns the installed entry.
	entities, err := readTableEntries(client2, deviceID2)
	if err != nil {
		t.Errorf("P4RT-1.3.2 Step 1: Read for re-enabled device_id %d failed: %v", deviceID2, err)
	} else if err := verifyLLDPEntryPresent(entities); err != nil {
		t.Errorf("P4RT-1.3.2 Step 1: Read for re-enabled device_id %d: %v", deviceID2, err)
	}

	// P4RT-1.3.2 Step 2 - Device goes down mid-operation (Up to Down).
	//   * While device_id = 222 is up (client2 is its primary), disable Linecard 2 via gNMI.
	//   * Verify subsequent Read and Write RPCs to device_id = 222 fail with NOT_FOUND.
	t.Logf("P4RT-1.3.2 Step 2 - Device goes down mid-operation (Up to Down): disabling Linecard 2 (%s)", env.node2.lcName)
	disableLinecard(t, env, env.node2)

	// P4RT-1.3.2 Step 2: subsequent Write fails with NOT_FOUND. The P4RT server
	// may reflect the linecard state change asynchronously, so the RPC is retried
	// until it returns NOT_FOUND or notFoundTimeout expires.
	if err := awaitNotFound(func() error { return writeLLDPEntry(client2, deviceID2, p4pb.Update_INSERT) }); err != nil {
		t.Errorf("P4RT-1.3.2 Step 2: Write for down device_id %d: %v", deviceID2, err)
	}
	// P4RT-1.3.2 Step 2: subsequent Read fails with NOT_FOUND.
	if err := awaitNotFound(func() error {
		_, err := readTableEntries(client2, deviceID2)
		return err
	}); err != nil {
		t.Errorf("P4RT-1.3.2 Step 2: Read for down device_id %d: %v", deviceID2, err)
	}
	// Linecard 2 is re-enabled by the cleanup registered in setupEnvironment.
}

// verifyPacketOutToDownDevice implements P4RT-1.3.1 Step 5. A new StreamChannel
// is opened for device_id 222 (the Step 2 stream was already terminated by the
// server), a MasterArbitrationUpdate followed by a PacketOut whose egress_port is
// DUT port-2 (on the disabled Linecard 2) is sent, and the test verifies the server
// either returns an error or drops the packet (no frame received on ATE port-2).
func verifyPacketOutToDownDevice(t *testing.T, env *testEnv, client *p4rt_client.P4RTClient) {
	t.Helper()
	const streamName = "p4rt_1_3_1_packetout_dev222"
	atePort2Name := env.ate.Port(t, env.node2.portID).ID()
	rxBefore, rxBeforeOK := gnmi.Lookup(t, env.ate.OTG(), gnmi.OTG().Port(atePort2Name).Counters().InFrames().State()).Val()

	p4rtutils.DrainStreamTermErr(client.StreamTermErr)
	params := streamParams(streamName, deviceID2)
	if err := client.StreamChannelCreate(params); err != nil {
		t.Fatalf("P4RT-1.3.1 Step 5: could not create StreamChannel %q: %v", streamName, err)
	}
	env.streams = append(env.streams, streamRef{client: client, name: streamName, deviceID: deviceID2})
	name := streamName
	sendErr := client.StreamChannelSendMsg(&name, &p4pb.StreamMessageRequest{
		Update: &p4pb.StreamMessageRequest_Arbitration{
			Arbitration: &p4pb.MasterArbitrationUpdate{
				DeviceId:   deviceID2,
				ElectionId: electionID(),
			},
		},
	})
	if sendErr == nil {
		sendErr = client.StreamChannelSendMsg(&name, p4rtutils.PacketOutWithEgressPortGet(lldpFrame(), portID2, false))
	}
	// The server may report the error by terminating the StreamChannel.
	termErr := p4rtutils.StreamTermErrForStream(client.StreamTermErr, streamName, streamTermTimeout)
	if termErr != nil || sendErr != nil {
		// The server returned an error for the PacketOut to the down device.
		return
	}
	// No error returned: verify the server dropped the packet, i.e. nothing was
	// received on ATE port-2 which is connected to DUT port-2 on Linecard 2.
	rxAfter, rxAfterOK := gnmi.Lookup(t, env.ate.OTG(), gnmi.OTG().Port(atePort2Name).Counters().InFrames().State()).Val()
	if rxBeforeOK && rxAfterOK && rxAfter > rxBefore {
		t.Errorf("P4RT-1.3.1 Step 5: PacketOut to down device_id %d was neither rejected nor dropped: ATE %s in-frames increased %d -> %d", deviceID2, atePort2Name, rxBefore, rxAfter)
	}
}

// operStatusComponents returns the components whose oper-status represents the
// state of the device_id on node, in the order they are verified: the linecard
// being power cycled, followed by the integrated circuit carrying the node-id.
// The integrated circuit is omitted on DUTs whose
// /components/component/state/oper-status for INTEGRATED_CIRCUIT does not track
// the component state (deviation OperStatusForIcUnsupported); on those DUTs the
// parent linecard is the component whose oper-status is verified, and the
// integrated-circuit node-id state (awaitNodeID) still confirms it is back.
func operStatusComponents(dut *ondatra.DUTDevice, node p4rtNode) []string {
	if deviations.OperStatusForIcUnsupported(dut) {
		return []string{node.lcName}
	}
	return []string{node.lcName, node.icName}
}

// disableLinecard implements P4RT-1.3.1 Step 1 and the disable action of
// P4RT-1.3.2 Step 2: it sets the linecard power-admin-state to POWER_DISABLED and
// verifies that the oper-status of the components for the device_id (see
// operStatusComponents) transition to DISABLED or INACTIVE. An integrated circuit
// that is removed from the component tree while its linecard is powered off also
// satisfies the check, since it is no longer operational. Waiting for every
// component ensures that no P4RT RPC is sent while the device is still up.
func disableLinecard(t *testing.T, env *testEnv, node p4rtNode) {
	t.Helper()
	components.SetLinecardPowerAdminState(t, env.dut, node.lcName, oc.Platform_ComponentPowerType_POWER_DISABLED, linecardTimeout)
	want := []oc.E_PlatformTypes_COMPONENT_OPER_STATUS{oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED, oc.PlatformTypes_COMPONENT_OPER_STATUS_INACTIVE}
	for _, name := range operStatusComponents(env.dut, node) {
		t.Logf("Verifying oper-status of component %s transitions to DISABLED or INACTIVE", name)
		var got oc.E_PlatformTypes_COMPONENT_OPER_STATUS
		var ok bool
		if name == node.lcName {
			got, ok = components.AwaitOperStatus(t, env.dut, name, linecardTimeout, want...)
		} else {
			got, ok = components.AwaitOperStatusOrAbsent(t, env.dut, name, linecardTimeout, want...)
		}
		if !ok {
			t.Fatalf("Component %s oper-status got %v, want DISABLED or INACTIVE", name, got)
		}
	}
}

// enableLinecard implements the re-enable action of P4RT-1.3.2 Step 1 (also used
// by cleanup): sets the linecard power-admin-state to POWER_ENABLED, verifies the
// components for the device_id (see operStatusComponents) return to ACTIVE, and
// verifies the integrated-circuit node-id state reconverges to device_id 222
// before any P4RT RPC is issued.
func enableLinecard(t *testing.T, env *testEnv, node p4rtNode) {
	t.Helper()
	components.SetLinecardPowerAdminState(t, env.dut, node.lcName, oc.Platform_ComponentPowerType_POWER_ENABLED, linecardTimeout)
	for _, name := range operStatusComponents(env.dut, node) {
		t.Logf("Verifying oper-status of component %s transitions to ACTIVE", name)
		if got, ok := components.AwaitOperStatus(t, env.dut, name, linecardTimeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE); !ok {
			t.Fatalf("Component %s oper-status got %v, want %v", name, got, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
		}
	}
	awaitNodeID(t, env.dut, node.icName, deviceID2)
}

// restoreDevice2 is the cleanup of the LLDP table entry installed on device_id 222
// in P4RT-1.3.2 Step 1. Linecard 2 is powered off again in P4RT-1.3.2 Step 2, so
// it is re-enabled first if it was left disabled. The power cycle may have removed
// the entry, so it is deleted only if it is still installed. Streams still open for
// device_id 222 are torn down first so that the new arbitration does not clash
// with their election id.
func (e *testEnv) restoreDevice2(t *testing.T) {
	t.Helper()
	e.enableLinecardIfDisabled(t)
	e.destroyStreams(deviceID2)
	e.deleteLLDPEntryIfPresent(t, deviceID2)
}

// enableLinecardIfDisabled re-enables Linecard 2 if it was left disabled. If it
// is already POWER_ENABLED (e.g. a re-enable earlier in the test timed out
// before the linecard came up), it waits for the linecard to become ACTIVE and
// reports an error if it does not, so that a DUT left with Linecard 2 down is
// flagged explicitly instead of being silently handed back.
func (e *testEnv) enableLinecardIfDisabled(t *testing.T) {
	t.Helper()
	lc2 := gnmi.OC().Component(e.node2.lcName)
	if v, ok := gnmi.Lookup(t, e.dut, lc2.Linecard().PowerAdminState().State()).Val(); ok && v == oc.Platform_ComponentPowerType_POWER_DISABLED {
		t.Logf("Cleanup: re-enabling Linecard %s", e.node2.lcName)
		enableLinecard(t, e, e.node2)
		return
	}
	if got, ok := gnmi.Lookup(t, e.dut, lc2.OperStatus().State()).Val(); ok && got == oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE {
		return
	}
	t.Logf("Cleanup: Linecard %s is POWER_ENABLED but not ACTIVE, waiting for it to come up", e.node2.lcName)
	if got, ok := components.AwaitOperStatus(t, e.dut, e.node2.lcName, linecardTimeout, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE); !ok {
		t.Errorf("Cleanup: Linecard %s oper-status got %v, want %v; the DUT is left with this linecard down and needs manual recovery", e.node2.lcName, got, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
	}
}

// cleanup restores the DUT state modified by the test: it re-enables Linecard 2
// if it was left disabled and tears down every StreamChannel created by the test.
// The LLDP table entries are deleted by the cleanups registered right before the
// writes that install them.
func (e *testEnv) cleanup(t *testing.T) {
	t.Log("Cleanup: restoring DUT state")
	e.enableLinecardIfDisabled(t)
	e.destroyStreams()
}

// deleteLLDPEntryIfPresent deletes the LLDP table entry from deviceID if a Read
// still returns it. Per the P4Runtime spec, a Read that fails with
// FAILED_PRECONDITION means no forwarding pipeline config is set on the device,
// so it holds no table entries.
func (e *testEnv) deleteLLDPEntryIfPresent(t *testing.T, deviceID uint64) {
	t.Helper()
	c := newP4RTClient(t, e.dut)
	if err := e.arbitrate(c, fmt.Sprintf("p4rt_cleanup_dev%d", deviceID), deviceID); err != nil {
		t.Errorf("Cleanup: MasterArbitrationUpdate for device_id %d failed: %v", deviceID, err)
		return
	}
	entities, err := readTableEntries(c, deviceID)
	if err != nil {
		if p4rtutils.CheckRPCErrorCode(err, codes.FailedPrecondition) != nil {
			t.Errorf("Cleanup: Read for device_id %d failed: %v", deviceID, err)
		}
		return
	}
	if verifyLLDPEntryPresent(entities) != nil {
		// The entry is no longer installed.
		return
	}
	if err := writeLLDPEntry(c, deviceID, p4pb.Update_DELETE); err != nil {
		t.Errorf("Cleanup: deleting LLDP entry on device_id %d failed: %v", deviceID, err)
	}
}

// destroyStreams tears down the StreamChannels created by the test that are still
// open and belong to one of deviceIDs, or all of them when no deviceIDs are given.
func (e *testEnv) destroyStreams(deviceIDs ...uint64) {
	for _, s := range e.streams {
		if len(deviceIDs) > 0 && !slices.Contains(deviceIDs, s.deviceID) {
			continue
		}
		name := s.name
		if s.client.StreamChannelGet(&name) != nil {
			s.client.StreamChannelDestroy(&name)
		}
	}
}

// selectPortsOnDistinctLinecards selects two DUT ports whose P4RT integrated
// circuits are different and whose parent linecards are also different, as
// required by "Test environment setup". Ports are evaluated in sorted order so the
// selection is deterministic.
func selectPortsOnDistinctLinecards(t *testing.T, dut *ondatra.DUTDevice) (p4rtNode, p4rtNode) {
	t.Helper()
	portToIC := p4rtutils.P4RTNodesByPort(t, dut)

	var ports []string
	for port, ic := range portToIC {
		if ic != "" {
			ports = append(ports, port)
		}
	}
	sort.Strings(ports)

	lcByIC := make(map[string]string)
	nodes := make([]p4rtNode, 0, len(ports))
	for _, port := range ports {
		ic := portToIC[port]
		if _, ok := lcByIC[ic]; !ok {
			lcByIC[ic] = cfgplugins.FindLineCardParent(t, dut, ic)
		}
		nodes = append(nodes, p4rtNode{portID: port, icName: ic, lcName: lcByIC[ic]})
	}

	for i := range nodes {
		for j := i + 1; j < len(nodes); j++ {
			if nodes[i].icName != nodes[j].icName && nodes[i].lcName != nodes[j].lcName {
				return nodes[i], nodes[j]
			}
		}
	}
	t.Fatalf("Test requires two DUT ports on integrated circuits with different parent linecards; P4RT nodes by port: %v, candidates: %+v", portToIC, nodes)
	return p4rtNode{}, p4rtNode{}
}

// configureDeviceIDs implements the "Test environment setup" step "Configure P4RT
// node-id (device_id)": it sets /components/component/integrated-circuit/config/node-id
// to 111 and 222 on the integrated circuits of node1 and node2 in one gNMI
// SetRequest and waits for the state leaves to reflect the configured values.
//
// Before the push it registers a t.Cleanup that restores the node-ids configured
// on the DUT before the test: the previous value is put back where there was one,
// otherwise the leaf is deleted.
func configureDeviceIDs(t *testing.T, dut *ondatra.DUTDevice, node1, node2 p4rtNode) {
	t.Helper()
	type nodeIDConfig struct {
		ic     string
		id     uint64
		orig   uint64
		origOK bool
	}
	var cfgs []nodeIDConfig
	for _, d := range []struct {
		node p4rtNode
		id   uint64
	}{{node: node1, id: deviceID1}, {node: node2, id: deviceID2}} {
		orig, ok := gnmi.LookupConfig(t, dut, gnmi.OC().Component(d.node.icName).IntegratedCircuit().NodeId().Config()).Val()
		cfgs = append(cfgs, nodeIDConfig{ic: d.node.icName, id: d.id, orig: orig, origOK: ok})
	}

	t.Cleanup(func() {
		t.Log("Cleanup: restoring integrated-circuit node-ids")
		batch := &gnmi.SetBatch{}
		for _, c := range cfgs {
			path := gnmi.OC().Component(c.ic).IntegratedCircuit().NodeId().Config()
			if c.origOK {
				gnmi.BatchReplace(batch, path, c.orig)
			} else {
				gnmi.BatchDelete(batch, path)
			}
		}
		batch.Set(t, dut)
	})

	batch := &gnmi.SetBatch{}
	for _, c := range cfgs {
		t.Logf("Test environment setup: configuring node-id %d on integrated circuit %s", c.id, c.ic)
		gnmi.BatchReplace(batch, gnmi.OC().Component(c.ic).Config(), &oc.Component{
			Name:              ygot.String(c.ic),
			IntegratedCircuit: &oc.Component_IntegratedCircuit{NodeId: ygot.Uint64(c.id)},
		})
	}
	batch.Set(t, dut)
	awaitNodeID(t, dut, node1.icName, deviceID1)
	awaitNodeID(t, dut, node2.icName, deviceID2)
}

// awaitNodeID waits for /components/component/integrated-circuit/state/node-id of
// ic to report id.
func awaitNodeID(t *testing.T, dut *ondatra.DUTDevice, ic string, id uint64) {
	t.Helper()
	if got, ok := gnmi.Await(t, dut, gnmi.OC().Component(ic).IntegratedCircuit().NodeId().State(), nodeIDTimeout, id).Val(); !ok {
		t.Fatalf("Component %s integrated-circuit node-id state got %v, want %d", ic, got, id)
	}
}

// configureInterfaceDUT returns the interface config for the given attributes
// with the P4RT port id set (/interfaces/interface/config/id).
func configureInterfaceDUT(dut *ondatra.DUTDevice, name string, id uint32, a *attrs.Attributes) *oc.Interface {
	i := &oc.Interface{
		Name:        ygot.String(name),
		Id:          ygot.Uint32(id),
		Description: ygot.String(a.Desc),
		Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
	}
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4.GetOrCreateAddress(a.IPv4).PrefixLength = ygot.Uint8(a.IPv4Len)
	return i
}

// configureDUT implements the "Test environment setup" DUT side: it configures
// DUT port-1 and port-2 with IPv4 addresses and P4RT port ids 20 and 21.
//
// Before the push it registers a t.Cleanup that restores the interface config
// present on the DUT before the test: the previous config is put back where there
// was one, otherwise the interface config is deleted.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, port1ID, port2ID string) {
	t.Helper()
	d := gnmi.OC()
	p1 := dut.Port(t, port1ID)
	p2 := dut.Port(t, port2ID)

	type intfConfig struct {
		name   string
		orig   *oc.Interface
		origOK bool
	}
	var cfgs []intfConfig
	for _, p := range []*ondatra.Port{p1, p2} {
		orig, ok := gnmi.LookupConfig(t, dut, d.Interface(p.Name()).Config()).Val()
		// Some DUTs report aggregation config (e.g. min-links) on non-LAG
		// interfaces. The aggregation container is only valid when the interface
		// type is ieee8023adLag, so replaying it would be rejected by the DUT.
		if ok && orig.GetType() != oc.IETFInterfaces_InterfaceType_ieee8023adLag {
			orig.Aggregation = nil
		}
		// Likewise, some DUTs report routed-vlan config on non-VLAN interfaces.
		// The routed-vlan container is only valid when the interface type is
		// l3ipvlan, so replaying it would be rejected by the DUT.
		if ok && orig.GetType() != oc.IETFInterfaces_InterfaceType_l3ipvlan {
			orig.RoutedVlan = nil
		}
		cfgs = append(cfgs, intfConfig{name: p.Name(), orig: orig, origOK: ok})
	}

	// On DUTs that need it (deviation ExplicitInterfaceInDefaultVRF), the test
	// assigns subinterface 0 of each port to the default network instance. The
	// network-instance interface entry references that subinterface, so the DUT
	// rejects restoring an interface config without it while the reference
	// exists. The entries added by the test are therefore deleted in the same
	// SetRequest as the interface restore (gNMI processes deletes first).
	ni := deviations.DefaultNetworkInstance(dut)
	var niIntfIDs []string
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, p := range []*ondatra.Port{p1, p2} {
			id := p.Name() + ".0"
			if _, ok := gnmi.LookupConfig(t, dut, d.NetworkInstance(ni).Interface(id).Config()).Val(); !ok {
				niIntfIDs = append(niIntfIDs, id)
			}
		}
	}

	t.Cleanup(func() {
		t.Log("Cleanup: restoring DUT interface configs")
		batch := &gnmi.SetBatch{}
		for _, id := range niIntfIDs {
			gnmi.BatchDelete(batch, d.NetworkInstance(ni).Interface(id).Config())
		}
		for _, c := range cfgs {
			if c.origOK {
				gnmi.BatchReplace(batch, d.Interface(c.name).Config(), c.orig)
			} else {
				gnmi.BatchDelete(batch, d.Interface(c.name).Config())
			}
		}
		batch.Set(t, dut)
	})

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configureInterfaceDUT(dut, p1.Name(), portID1, &dutPort1))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configureInterfaceDUT(dut, p2.Name(), portID2, &dutPort2))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

// configureATE implements the "Test environment setup" ATE side: ATE port-1 and
// port-2 connected to DUT port-1 and port-2 respectively.
func configureATE(t *testing.T, ate *ondatra.ATEDevice, port1ID, port2ID string) gosnappi.Config {
	t.Helper()
	top := gosnappi.NewConfig()
	atePort1.AddToOTG(top, ate.Port(t, port1ID), &dutPort1)
	atePort2.AddToOTG(top, ate.Port(t, port2ID), &dutPort2)
	return top
}

// loadP4Info loads the WBB P4Info sent in P4RT-1.3.1 Step 2 and P4RT-1.3.2 Step 1.
func loadP4Info(t *testing.T) *p4configpb.P4Info {
	t.Helper()
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		t.Fatalf("Errors seen when loading p4info file: %v", err)
	}
	return p4Info
}

// newP4RTClient returns a P4RT client using the DUT's P4RT connection.
func newP4RTClient(t *testing.T, dut *ondatra.DUTDevice) *p4rt_client.P4RTClient {
	t.Helper()
	client := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := client.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize P4RT client: %v", err)
	}
	return client
}

// electionID returns the election id used by every primary client of the test.
func electionID() *p4pb.Uint128 {
	return &p4pb.Uint128{High: electionIDHigh, Low: electionIDLow}
}

// streamParams returns StreamChannel parameters for deviceID.
func streamParams(name string, deviceID uint64) *p4rt_client.P4RTStreamParameters {
	return &p4rt_client.P4RTStreamParameters{
		Name:        name,
		DeviceId:    deviceID,
		ElectionIdH: electionIDHigh,
		ElectionIdL: electionIDLow,
	}
}

// arbitrate opens the StreamChannel name for deviceID on client, sends a
// MasterArbitrationUpdate and returns nil if the client became primary, or the
// error (gRPC status when reported by the server) otherwise.
func (e *testEnv) arbitrate(client *p4rt_client.P4RTClient, name string, deviceID uint64) error {
	p4rtutils.DrainStreamTermErr(client.StreamTermErr)
	e.streams = append(e.streams, streamRef{client: client, name: name, deviceID: deviceID})
	return p4rtutils.StreamArbitrate(client, streamParams(name, deviceID), arbitrationTimeout)
}

// setForwardingPipeline sends the WBB P4Info via SetForwardingPipelineConfig
// (VERIFY_AND_COMMIT) to deviceID.
func setForwardingPipeline(client *p4rt_client.P4RTClient, deviceID uint64, p4Info *p4configpb.P4Info) error {
	return client.SetForwardingPipelineConfig(p4rtutils.SetForwardingPipelineConfigGet(deviceID, electionID(), p4Info, pipelineCookie))
}

// verifyForwardingPipeline verifies with GetForwardingPipelineConfig that the
// P4Info pushed by a successful SetForwardingPipelineConfig is in effect.
func verifyForwardingPipeline(t *testing.T, client *p4rt_client.P4RTClient, deviceID uint64, want *p4configpb.P4Info) {
	t.Helper()
	resp, err := client.GetForwardingPipelineConfig(&p4pb.GetForwardingPipelineConfigRequest{
		DeviceId:     deviceID,
		ResponseType: p4pb.GetForwardingPipelineConfigRequest_P4INFO_AND_COOKIE,
	})
	if err != nil {
		t.Fatalf("GetForwardingPipelineConfig for device_id %d failed: %v", deviceID, err)
	}
	if diff := cmp.Diff(want, resp.GetConfig().GetP4Info(), protocmp.Transform()); diff != "" {
		t.Errorf("GetForwardingPipelineConfig for device_id %d: P4Info diff (-want +got):\n%s", deviceID, diff)
	}
}

// lldpUpdate returns the AclWbbIngressTableEntry update for LLDP
// (ethertype 0x88CC) used in P4RT-1.3.1 Step 3 and P4RT-1.3.2.
func lldpUpdate(updateType p4pb.Update_Type) *p4pb.Update {
	return p4rtutils.ACLWbbIngressTableEntryGet([]*p4rtutils.ACLWbbIngressTableEntryInfo{{
		Type:          updateType,
		EtherType:     lldpEtherType,
		EtherTypeMask: 0xFFFF,
		Priority:      1,
	}})[0]
}

// writeLLDPEntry sends a Write RPC with the LLDP AclWbbIngressTableEntry to deviceID.
func writeLLDPEntry(client *p4rt_client.P4RTClient, deviceID uint64, updateType p4pb.Update_Type) error {
	return client.Write(&p4pb.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: electionID(),
		Updates:    []*p4pb.Update{lldpUpdate(updateType)},
		Atomicity:  p4pb.WriteRequest_CONTINUE_ON_ERROR,
	})
}

// readTableEntries sends a Read RPC for the acl_wbb_ingress_table entries of deviceID.
func readTableEntries(client *p4rt_client.P4RTClient, deviceID uint64) ([]*p4pb.Entity, error) {
	return p4rtutils.ReadTableEntries(client, deviceID, p4rtutils.WbbTableMap["acl_wbb_ingress_table"])
}

// awaitNotFound calls rpc until it returns a NOT_FOUND gRPC status or
// notFoundTimeout expires, and returns nil on NOT_FOUND or the last mismatch
// otherwise. It is used for RPCs sent right after a device goes down, which the
// P4RT server may reflect asynchronously; the expected status is unchanged.
func awaitNotFound(rpc func() error) error {
	deadline := time.Now().Add(notFoundTimeout)
	for {
		err := p4rtutils.CheckRPCErrorNotFound(rpc())
		if err == nil {
			return nil
		}
		if time.Now().Add(notFoundPollInterval).After(deadline) {
			return fmt.Errorf("no NOT_FOUND within %v: %w", notFoundTimeout, err)
		}
		time.Sleep(notFoundPollInterval)
	}
}

// verifyLLDPEntryPresent verifies the LLDP AclWbbIngressTableEntry is among the
// entities returned by a Read RPC.
func verifyLLDPEntryPresent(entities []*p4pb.Entity) error {
	want := lldpUpdate(p4pb.Update_INSERT).GetEntity()
	for _, e := range entities {
		if cmp.Equal(e, want, protocmp.Transform(), protocmp.IgnoreFields(&p4pb.TableEntry{}, "meter_config", "counter_data")) {
			return nil
		}
	}
	return fmt.Errorf("LLDP table entry not found in Read response: got %v, want %v", entities, want)
}

// lldpFrame returns a minimal LLDP Ethernet frame used as PacketOut payload in
// P4RT-1.3.1 Step 5.
func lldpFrame() []byte {
	return []byte{
		0x01, 0x80, 0xc2, 0x00, 0x00, 0x0e, // Destination MAC: LLDP nearest bridge.
		0x02, 0x00, 0x00, 0x00, 0x00, 0x01, // Source MAC (locally administered).
		0x88, 0xcc, // EtherType: LLDP.
		0x02, 0x07, 0x04, 0x02, 0x00, 0x00, 0x00, 0x00, 0x01, // Chassis ID TLV (MAC).
		0x04, 0x02, 0x07, 0x31, // Port ID TLV (locally assigned "1").
		0x06, 0x02, 0x00, 0x78, // TTL TLV (120s).
		0x00, 0x00, // End of LLDPDU TLV.
	}
}
