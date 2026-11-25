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

// Package p4rt_supervisor_switchover_test implements P4RT-1.4: P4RT PacketIn and
// gRIBI Get Continuity across Supervisor Switchover.
package p4rt_supervisor_switchover_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	cmp "github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	spb "github.com/openconfig/gnoi/system"
	gpb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	p4pb "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test parameters defined in the README "Test parameters" section.
var (
	p4InfoFile             = flag.String("p4info_file_location", "../../data/wbb.p4info.pb.txt", "Path to the p4info file.")
	switchoverTimeout      = flag.Duration("switchover_timeout", 30*time.Minute, "Maximum time to re-establish gNMI and observe the new controller card roles after a switchover, and to wait for switchover-ready.")
	gribiScale             = flag.Int("gribi_scale", 10000, "Number of IPv4 scale routes programmed in the default network instance.")
	getDeadline            = flag.Duration("get_deadline", 10*time.Second, "gRPC deadline applied to each gRIBI Get RPC.")
	gribiReconnectTimeout  = flag.Duration("gribi_reconnect_timeout", 10*time.Minute, "Maximum time to establish a gRIBI session with the new primary controller card.")
	p4rtReconnectTimeout   = flag.Duration("p4rt_reconnect_timeout", 10*time.Minute, "Maximum time to re-establish the P4RT StreamChannel after a switchover.")
	gnmiReconnectTimeout   = flag.Duration("gnmi_reconnect_timeout", 10*time.Minute, "Maximum time to complete a gNMI Set on the new primary controller card after a switchover.")
	packetinTimeout        = flag.Duration("packetin_timeout", 60*time.Second, "Maximum time to receive all expected PacketIn messages.")
	packetinNegativeWindow = flag.Duration("packetin_negative_window", 10*time.Second, "Time during which no matching PacketIn message may be received in a negative check.")
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126

	vrf1 = "VRF-1"

	// P4RT interface IDs of the DUT ports (README test environment setup). lag1
	// has no P4RT interface ID: PacketIn messages for packets received on lag1
	// carry the member port ID, and some DUTs only accept P4RT interface IDs on
	// physical interfaces.
	port1ID = uint32(1)
	port2ID = uint32(2)
	port3ID = uint32(3)
	port4ID = uint32(4)

	// gRIBI next hops and next-hop-groups.
	nhIndexV4 = uint64(1)
	nhIndexV6 = uint64(2)
	nhgIDV4   = uint64(1)
	nhgIDV6   = uint64(2)
	nhgIDCan  = uint64(3)

	// gRIBI routes.
	dataPrefixV4    = "198.51.100.0/24"
	dataPrefixV6    = "2001:db8:2::/64"
	dataDstV4       = "198.51.100.1"
	dataDstV6       = "2001:db8:2::1"
	scaleStartV4    = "198.18.0.1"
	scaleStartV6    = "2001:db8:100::1"
	scaleCountV6    = 1000
	vrf1StartV4     = "198.19.0.1"
	vrf1CountV4     = 1000
	canaryCount     = 100
	maxGRIBIScale   = 65535 // 198.18.0.1 - 198.18.255.255
	canaryDelete    = 50
	gribiBatchSize  = 1000
	stalePrefixV4   = "198.51.100.200/32"
	gribiOpTimeout  = 10 * time.Minute
	aftAwaitTimeout = 10 * time.Minute

	// P4RT election IDs (README).
	p4rtPrimaryEID0   = uint64(10)
	p4rtSecondaryEID0 = uint64(5)
	p4rtPrimaryEID1   = uint64(20)
	p4rtSecondaryEID1 = uint64(15)
	p4rtPrimaryEID2   = uint64(30)
	p4rtSecondaryEID2 = uint64(25)
	p4rtStaleEID      = uint64(10)

	// P4RT election IDs used by the mandatory cleanup if it has to reconnect.
	p4rtCleanupPrimaryEID   = uint64(40)
	p4rtCleanupSecondaryEID = uint64(35)
	p4rtCookie              = uint64(159)

	// PacketIn metadata IDs in the WBB P4Info.
	metadataIngressPort = uint32(1)
	metadataEgressPort  = uint32(2)

	// Control packets.
	controlPktCount   = 10
	lagTracerouteCnt  = 16
	lagUDPSrcPortBase = 49152
	controlPktSize    = 128
	lldpDstMAC        = "01:80:c2:00:00:0e"
	gdpDstMAC         = "00:0a:da:f0:f0:f0"
	lldpEtherType     = 0x88cc
	gdpEtherType      = 0x6007
	markerMAC         = "02:00:aa:00:00:01"
	packetOutMAC      = "02:00:aa:00:00:02"

	// Data plane traffic.
	flowIPv4    = "Flow-IPv4"
	flowIPv6    = "Flow-IPv6"
	dataPPS     = 1000
	dataPktSize = 512

	captureName = "packetout-capture"

	// Control packet flows are sent only once each: some OTG implementations do
	// not retransmit a completed fixed-count flow when it is started again. The
	// ATE configuration therefore contains one flow set for the no-punt check
	// and maxValidations flow sets for the PacketIn validation procedure (the
	// test runs it at most 5 times: baseline, P4RT-1.4.1 Steps 2 and 3, and
	// P4RT-1.4.2 Steps 3 and 6).
	noPuntFlowSet    = "np"
	maxValidations   = 6
	reconnectBackoff = 5 * time.Second
	// awaitAttemptTimeout bounds each gNMI Await attempt while the controller
	// cards switch over, so that a gNMI stream that went stale during the
	// switchover is re-established on the next attempt.
	awaitAttemptTimeout = 2 * time.Minute
	// fibStatusRetryInterval paces repeated gRIBI Get RPCs while waiting for the
	// gRIBI fib_status to converge after a switchover.
	fibStatusRetryInterval = 15 * time.Second
	// clockSkewTolerance is the allowed clock difference between the test host
	// and the DUT when comparing last-switchover-time with the request time.
	clockSkewTolerance = time.Minute
	// switchoverRPCTimeout bounds the gNOI SwitchControlProcessor RPC.
	switchoverRPCTimeout = 5 * time.Minute
	// switchoverInfoTimeout bounds the wait for last-switchover-time and
	// last-switchover-reason to reflect a switchover once the new controller
	// card roles are reported.
	switchoverInfoTimeout = 2 * time.Minute

	controllerCardType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
)

var (
	dutPort1 = attrs.Attributes{Desc: "DUT port-1 to ATE port-1", IPv4: "192.0.2.1", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::1", IPv6Len: ipv6PrefixLen}
	atePort1 = attrs.Attributes{Name: "atePort1", MAC: "02:00:01:01:01:01", IPv4: "192.0.2.2", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::2", IPv6Len: ipv6PrefixLen}
	dutLAG   = attrs.Attributes{Desc: "LAG to ATE port-2 and ATE port-3", IPv4: "192.0.2.5", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::5", IPv6Len: ipv6PrefixLen}
	ateLAG   = attrs.Attributes{Name: "ateLAG", MAC: "02:00:02:01:01:01", IPv4: "192.0.2.6", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::6", IPv6Len: ipv6PrefixLen}
	dutPort4 = attrs.Attributes{Desc: "DUT port-4 to ATE port-4", IPv4: "192.0.2.9", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::9", IPv6Len: ipv6PrefixLen}
	atePort4 = attrs.Attributes{Name: "atePort4", MAC: "02:00:04:01:01:01", IPv4: "192.0.2.10", IPv4Len: ipv4PrefixLen, IPv6: "2001:db8::a", IPv6Len: ipv6PrefixLen}

	ateLAGSystemID   = "02:00:02:00:00:01"
	ateLAGMemberMACs = []string{"02:00:02:01:01:02", "02:00:02:01:01:03"}

	portIDs = map[string]uint32{"port1": port1ID, "port2": port2ID, "port3": port3ID, "port4": port4ID}
)

// pktKind classifies punted control packets.
type pktKind string

const (
	kindLLDP pktKind = "LLDP"
	kindGDP  pktKind = "GDP"
	kindTR4  pktKind = "IPv4-traceroute"
	kindTR6  pktKind = "IPv6-traceroute"
)

// p4rtNode is a P4RT integrated circuit hosting one or more bound DUT ports.
type p4rtNode struct {
	name     string
	deviceID uint64
}

// p4rtSession holds the primary and secondary P4RT clients for all nodes.
type p4rtSession struct {
	primary      *p4rt_client.P4RTClient
	secondary    *p4rt_client.P4RTClient
	primaryEID   uint64
	secondaryEID uint64
}

// gribiState is the expected gRIBI state per network instance.
type gribiState struct {
	v4   map[string]map[string]uint64 // network instance -> prefix -> NHG ID
	v6   map[string]map[string]uint64
	nhgs map[string]map[uint64]bool
	nhs  map[string]map[uint64]bool
}

// testState holds all objects shared by the setup, subtests and cleanup.
type testState struct {
	dut         *ondatra.DUTDevice
	ate         *ondatra.ATEDevice
	top         gosnappi.Config
	defaultNI   string
	aggID       string
	nodes       []p4rtNode
	nodeOfPort  map[string]p4rtNode // ondatra port ID -> node
	p4          *p4rtSession
	gribiLeader *gribi.Client
	gribiAEID   gribi.Uint128
	gribiMaxEID gribi.Uint128
	origNodeID  map[string]*ygnmi.Value[uint64] // integrated circuit -> original node-id config
	dataRunning bool                            // data plane flows are running
	// Setup progress, used by the mandatory cleanup to only undo what was done.
	dutConfigured bool                    // configureDUT started
	vrf1Created   bool                    // VRF-1 was configured
	gribiStarted  bool                    // gRIBI programming started
	puntWritten   bool                    // P4RT punt entries were written at least once
	validations   int                     // number of control flow sets used by the PacketIn validation procedure
	dataSeg       map[string]flowCounters // data plane counters at the start of the current counter segment
	dataAcc       map[string]flowCounters // data plane counters accumulated before counter resets
	expected      *gribiState
	supA          string
	supB          string
	extraWait     time.Duration
}

// reconnectTimeout returns a reconnect timeout extended by the platform
// switchover stabilize delay (deviation switchover_stabilize_delay_m).
func (s *testState) reconnectTimeout(d time.Duration) time.Duration {
	return d + s.extraWait
}

// streamName returns the P4RT stream name for a role and device ID.
func streamName(role string, deviceID uint64) string {
	return fmt.Sprintf("%s-%d", role, deviceID)
}

// generateIPv4Prefixes returns count consecutive /32 prefixes starting at start.
func generateIPv4Prefixes(t testing.TB, start string, count int) []string {
	t.Helper()
	ip := net.ParseIP(start).To4()
	if ip == nil {
		t.Fatalf("Invalid IPv4 address %q", start)
	}
	base := binary.BigEndian.Uint32(ip)
	prefixes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		b := make(net.IP, 4)
		binary.BigEndian.PutUint32(b, base+uint32(i))
		prefixes = append(prefixes, b.String()+"/32")
	}
	return prefixes
}

// generateIPv6Prefixes returns count consecutive /128 prefixes starting at start.
func generateIPv6Prefixes(t testing.TB, start string, count int) []string {
	t.Helper()
	ip := net.ParseIP(start).To16()
	if ip == nil {
		t.Fatalf("Invalid IPv6 address %q", start)
	}
	low := binary.BigEndian.Uint64(ip[8:])
	prefixes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		b := make(net.IP, 16)
		copy(b, ip)
		binary.BigEndian.PutUint64(b[8:], low+uint64(i))
		prefixes = append(prefixes, b.String()+"/128")
	}
	return prefixes
}

// retryUntil runs fn until it completes without a fatal error or timeout expires.
// The backoff only paces RPC reconnection attempts while the new primary
// controller card brings up its services; it is not used to wait for state.
func retryUntil(t *testing.T, timeout time.Duration, desc string, fn func(t testing.TB)) {
	t.Helper()
	start := time.Now()
	for attempt := 1; ; attempt++ {
		if errMsg := testt.CaptureFatal(t, fn); errMsg == nil {
			t.Logf("%s succeeded after %d attempt(s) in %v", desc, attempt, time.Since(start).Round(time.Second))
			return
		} else if time.Since(start) > timeout {
			t.Fatalf("%s did not succeed within %v: %s", desc, timeout, *errMsg)
		} else {
			t.Logf("%s attempt %d failed, retrying: %s", desc, attempt, *errMsg)
		}
		time.Sleep(reconnectBackoff)
	}
}

// ---------------------------------------------------------------------------
// Test environment setup: DUT configuration.
// ---------------------------------------------------------------------------

// dutInterface builds a routed interface with IPv4/IPv6 addresses. A non-zero id
// is configured as the P4RT interface ID; 0 configures no P4RT interface ID.
func dutInterface(dut *ondatra.DUTDevice, name string, id uint32, a *attrs.Attributes, ifType oc.E_IETFInterfaces_InterfaceType) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(name), Description: ygot.String(a.Desc), Type: ifType}
	if id != 0 {
		i.Id = ygot.Uint32(id)
	}
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4.GetOrCreateAddress(a.IPv4).PrefixLength = ygot.Uint8(uint8(a.IPv4Len))
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(uint8(a.IPv6Len))
	return i
}

// configureDUT configures DUT port-1, lag1 (port-2, port-3), DUT port-4,
// interface IDs, VRF-1 and disables on-box LLDP (README test environment setup).
func configureDUT(t *testing.T, s *testState) {
	t.Helper()
	s.dutConfigured = true
	dut := s.dut
	d := gnmi.OC()
	p1, p2, p3, p4 := dut.Port(t, "port1"), dut.Port(t, "port2"), dut.Port(t, "port3"), dut.Port(t, "port4")

	// DUT port-1 and DUT port-4: routed interfaces with P4RT interface IDs 1 and 4.
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutInterface(dut, p1.Name(), port1ID, &dutPort1, oc.IETFInterfaces_InterfaceType_ethernetCsmacd))
	gnmi.Replace(t, dut, d.Interface(p4.Name()).Config(), dutInterface(dut, p4.Name(), port4ID, &dutPort4, oc.IETFInterfaces_InterfaceType_ethernetCsmacd))

	// lag1: LACP ACTIVE / FAST and IPv4/IPv6 addresses, without a P4RT interface ID.
	lacp := &oc.Lacp_Interface{Name: ygot.String(s.aggID), LacpMode: oc.Lacp_LacpActivityType_ACTIVE, Interval: oc.Lacp_LacpPeriodType_FAST}
	if deviations.AggregateAtomicUpdate(dut) {
		// The DUT requires the aggregate interface to exist before the LACP
		// configuration and the member interfaces reference it, so lag1, its LACP
		// configuration and its members are created in one Update first.
		setupAggregateAtomically(t, s, lacp)
	}
	gnmi.Replace(t, dut, d.Lacp().Interface(s.aggID).Config(), lacp)
	agg := dutInterface(dut, s.aggID, 0, &dutLAG, oc.IETFInterfaces_InterfaceType_ieee8023adLag)
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	gnmi.Replace(t, dut, d.Interface(s.aggID).Config(), agg)
	if deviations.LacpConfigAfterAggregateReplace(dut) {
		gnmi.Update(t, dut, d.Lacp().Interface(s.aggID).Config(), lacp)
	}

	// DUT port-2 and DUT port-3: members of lag1 with P4RT interface IDs 2 and 3.
	for _, m := range []struct {
		port *ondatra.Port
		id   uint32
	}{{p2, port2ID}, {p3, port3ID}} {
		i := &oc.Interface{Name: ygot.String(m.port.Name()), Id: ygot.Uint32(m.id), Description: ygot.String("member of " + s.aggID), Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd}
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		i.GetOrCreateEthernet().AggregateId = ygot.String(s.aggID)
		gnmi.Replace(t, dut, d.Interface(m.port.Name()).Config(), i)
	}

	if deviations.ExplicitPortSpeed(dut) {
		for _, p := range []*ondatra.Port{p1, p2, p3, p4} {
			fptest.SetPortSpeed(t, p)
		}
	}
	// DUT port-1, lag1 and DUT port-4 (subinterface 0) are in the default network instance.
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, name := range []string{p1.Name(), s.aggID, p4.Name()} {
			fptest.AssignToNetworkInstance(t, dut, name, s.defaultNI, 0)
		}
	}

	// Non-default network instance VRF-1 of type L3VRF without interfaces.
	s.vrf1Created = true
	gnmi.Replace(t, dut, d.NetworkInstance(vrf1).Config(), &oc.NetworkInstance{
		Name: ygot.String(vrf1),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
	})

	// Disable on-box LLDP so that LLDP is only handled through P4RT punting.
	gnmi.Replace(t, dut, d.Lldp().Enabled().Config(), false)
}

// setupAggregateAtomically removes DUT port-2 and DUT port-3 from any aggregate
// and creates lag1, its LACP configuration and its members in a single Update
// (deviation aggregate_atomic_update).
func setupAggregateAtomically(t *testing.T, s *testState, lacp *oc.Lacp_Interface) {
	t.Helper()
	dut := s.dut
	members := []string{dut.Port(t, "port2").Name(), dut.Port(t, "port3").Name()}
	for _, m := range members {
		gnmi.Delete(t, dut, gnmi.OC().Interface(m).Ethernet().AggregateId().Config())
	}
	root := &oc.Root{}
	l := root.GetOrCreateLacp().GetOrCreateInterface(s.aggID)
	l.LacpMode, l.Interval = lacp.LacpMode, lacp.Interval
	agg := root.GetOrCreateInterface(s.aggID)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	for _, m := range members {
		i := root.GetOrCreateInterface(m)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
		i.GetOrCreateEthernet().AggregateId = ygot.String(s.aggID)
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), root)
}

// configureNodeIDs configures a unique P4RT node ID on every integrated circuit
// hosting a bound DUT port. Multi-IC coverage is best effort (README).
func configureNodeIDs(t *testing.T, s *testState) {
	t.Helper()
	byPort := p4rtutils.P4RTNodesByPort(t, s.dut)
	s.nodeOfPort = map[string]p4rtNode{}
	ids := map[string]uint64{}
	var names []string
	for _, pid := range []string{"port1", "port2", "port3", "port4"} {
		name, ok := byPort[pid]
		if !ok {
			t.Fatalf("Could not find the INTEGRATED_CIRCUIT hosting DUT %s", pid)
		}
		if _, seen := ids[name]; !seen {
			ids[name] = uint64(len(ids) + 1)
			names = append(names, name)
		}
		s.nodeOfPort[pid] = p4rtNode{name: name, deviceID: ids[name]}
	}
	s.origNodeID = map[string]*ygnmi.Value[uint64]{}
	for _, name := range names {
		node := p4rtNode{name: name, deviceID: ids[name]}
		// Record the original node-id so that the cleanup can restore it, then
		// merge the new node-id without replacing other component configuration.
		s.origNodeID[name] = gnmi.Lookup(t, s.dut, gnmi.OC().Component(name).IntegratedCircuit().NodeId().Config())
		s.nodes = append(s.nodes, node)
		c := &oc.Component{Name: ygot.String(name), IntegratedCircuit: &oc.Component_IntegratedCircuit{NodeId: ygot.Uint64(node.deviceID)}}
		gnmi.Update(t, s.dut, gnmi.OC().Component(name).Config(), c)
		gnmi.Await(t, s.dut, gnmi.OC().Component(name).IntegratedCircuit().NodeId().State(), time.Minute, node.deviceID)
	}
	t.Logf("Bound DUT ports are hosted on %d integrated circuit(s): %v (multi-IC coverage is best effort and depends on testbed cabling)", len(s.nodes), names)
}

// verifyInterfaceIDs verifies /interfaces/interface/state/id for the DUT ports.
func verifyInterfaceIDs(t *testing.T, s *testState) {
	t.Helper()
	want := map[string]uint32{}
	for pid, id := range portIDs {
		want[s.dut.Port(t, pid).Name()] = id
	}
	for name, id := range want {
		gnmi.Await(t, s.dut, gnmi.OC().Interface(name).Id().State(), time.Minute, id)
	}
}

// linkAndLAGCheck waits until all DUT ports and lag1 are UP and both LAG members
// are IN_SYNC, collecting and distributing (README "link and LAG check").
func linkAndLAGCheck(t *testing.T, s *testState) {
	t.Helper()
	timeout := 5*time.Minute + s.extraWait
	names := []string{s.aggID}
	for _, pid := range []string{"port1", "port2", "port3", "port4"} {
		names = append(names, s.dut.Port(t, pid).Name())
	}
	for _, name := range names {
		gnmi.Await(t, s.dut, gnmi.OC().Interface(name).OperStatus().State(), timeout, oc.Interface_OperStatus_UP)
	}
	for _, pid := range []string{"port2", "port3"} {
		awaitLAGMember(t, s, s.dut.Port(t, pid).Name(), timeout)
	}
}

// awaitLAGMember waits for a LAG member to be IN_SYNC, collecting and distributing.
func awaitLAGMember(t *testing.T, s *testState, member string, timeout time.Duration) {
	t.Helper()
	m := gnmi.OC().Lacp().Interface(s.aggID).Member(member)
	gnmi.Await(t, s.dut, m.Synchronization().State(), timeout, oc.Lacp_LacpSynchronizationType_IN_SYNC)
	gnmi.Await(t, s.dut, m.Collecting().State(), timeout, true)
	gnmi.Await(t, s.dut, m.Distributing().State(), timeout, true)
}

// ---------------------------------------------------------------------------
// Test environment setup: ATE configuration and traffic definitions.
// ---------------------------------------------------------------------------

// configureATE configures ATE port-1, the ATE LAG (port-2, port-3) and ATE
// port-4, the control packet flows, the data plane flows and the capture.
func configureATE(t *testing.T, s *testState) {
	t.Helper()
	ate := s.ate
	top := gosnappi.NewConfig()
	p1, p2, p3, p4 := ate.Port(t, "port1"), ate.Port(t, "port2"), ate.Port(t, "port3"), ate.Port(t, "port4")

	atePort1.AddToOTG(top, p1, &dutPort1)
	atePort4.AddToOTG(top, p4, &dutPort4)

	// ATE LAG: LACP ACTIVE / FAST with system ID 02:00:02:00:00:01.
	lag := top.Lags().Add().SetName(ateLAG.Name)
	lag.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId(ateLAGSystemID)
	for i, p := range []*ondatra.Port{p2, p3} {
		top.Ports().Add().SetName(p.ID())
		lp := lag.Ports().Add().SetPortName(p.ID())
		lp.Ethernet().SetMac(ateLAGMemberMACs[i]).SetName(fmt.Sprintf("%s.member%d", ateLAG.Name, i+1))
		// LACP ACTIVE with a FAST (1 second) periodic interval and a short (3 second) timeout.
		lp.Lacp().SetActorActivity("active").SetActorPortNumber(uint32(i + 1)).SetActorPortPriority(1).SetLacpduPeriodicTimeInterval(1).SetLacpduTimeout(3)
	}
	lagDev := top.Devices().Add().SetName(ateLAG.Name + ".dev")
	lagEth := lagDev.Ethernets().Add().SetName(ateLAG.Name + ".Eth").SetMac(ateLAG.MAC)
	lagEth.Connection().SetLagName(lag.Name())
	lagEth.Ipv4Addresses().Add().SetName(ateLAG.Name + ".IPv4").SetAddress(ateLAG.IPv4).SetGateway(dutLAG.IPv4).SetPrefix(uint32(ateLAG.IPv4Len))
	lagEth.Ipv6Addresses().Add().SetName(ateLAG.Name + ".IPv6").SetAddress(ateLAG.IPv6).SetGateway(dutLAG.IPv6).SetPrefix(uint32(ateLAG.IPv6Len))

	// Disable FEC for 100G-FR ports.
	var fr []string
	for _, p := range []*ondatra.Port{p1, p2, p3, p4} {
		if p.PMD() == ondatra.PMD100GBASEFR {
			fr = append(fr, p.ID())
		}
	}
	if len(fr) > 0 {
		l1 := top.Layer1().Add().SetName("L1").SetPortNames(fr)
		l1.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		l1.AutoNegotiation().SetRsFec(false)
	}

	// Control packet flow sets (each flow is sent only once, see maxValidations).
	// No-punt check: LLDP and GDP from ATE port-1.
	addL2ControlFlow(top, p1.ID(), kindLLDP, lldpDstMAC, lldpEtherType, noPuntFlowSet)
	addL2ControlFlow(top, p1.ID(), kindGDP, gdpDstMAC, gdpEtherType, noPuntFlowSet)
	// Traceroute from the ATE LAG: the ATE cannot send on a LAG and on the
	// physical member ports of that LAG at the same time (the LLDP and GDP flows
	// are sent from each member port), so the LAG traceroute packets are sent as
	// raw flows from each member port with the ATE LAG addresses, half of the
	// packets on each member, towards the MAC address of DUT lag1.
	dutLAGMAC := dutLAGMACAddress(t, s)
	for i := 0; i < maxValidations; i++ {
		set := validationFlowSet(i)
		// LLDP and GDP flows from every ATE port under test (each LAG member individually).
		for _, p := range []*ondatra.Port{p1, p2, p3} {
			addL2ControlFlow(top, p.ID(), kindLLDP, lldpDstMAC, lldpEtherType, set)
			addL2ControlFlow(top, p.ID(), kindGDP, gdpDstMAC, gdpEtherType, set)
		}
		// Traceroute flows from ATE port-1 and from the ATE LAG members.
		addTracerouteFlow(top, "port1", atePort1.Name, false, controlPktCount, set)
		addTracerouteFlow(top, "port1", atePort1.Name, true, controlPktCount, set)
		for _, p := range []*ondatra.Port{p2, p3} {
			addLAGTracerouteFlow(top, p.ID(), dutLAGMAC, false, set)
			addLAGTracerouteFlow(top, p.ID(), dutLAGMAC, true, set)
		}
	}

	// Data plane flows from ATE port-1 to the gRIBI data plane routes.
	for _, f := range []struct {
		name string
		v6   bool
	}{{flowIPv4, false}, {flowIPv6, true}} {
		suffix, src, dst := ".IPv4", atePort1.IPv4, dataDstV4
		if f.v6 {
			suffix, src, dst = ".IPv6", atePort1.IPv6, dataDstV6
		}
		flow := top.Flows().Add().SetName(f.name)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + suffix}).SetRxNames([]string{atePort4.Name + suffix})
		flow.Size().SetFixed(dataPktSize)
		flow.Rate().SetPps(dataPPS)
		flow.Packet().Add().Ethernet().Src().SetValue(atePort1.MAC)
		if f.v6 {
			ip := flow.Packet().Add().Ipv6()
			ip.Src().SetValue(src)
			ip.Dst().SetValue(dst)
		} else {
			ip := flow.Packet().Add().Ipv4()
			ip.Src().SetValue(src)
			ip.Dst().SetValue(dst)
		}
	}

	// Capture on ATE port-1 and ATE port-2 for PacketOut validation.
	top.Captures().Add().SetName(captureName).SetPortNames([]string{p1.ID(), p2.ID()}).SetFormat(gosnappi.CaptureFormat.PCAP)

	s.top = top
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")
}

// validationFlowSet returns the name of the i-th control flow set used by the
// PacketIn validation procedure.
func validationFlowSet(i int) string {
	return fmt.Sprintf("v%d", i)
}

// l2FlowName returns the name of an LLDP or GDP flow sent from an ATE port.
func l2FlowName(kind pktKind, portID, set string) string {
	return fmt.Sprintf("%s-%s-%s", kind, portID, set)
}

// tracerouteFlowName returns the name of a traceroute flow.
func tracerouteFlowName(src string, v6 bool, set string) string {
	if v6 {
		return fmt.Sprintf("%s-%s-%s", kindTR6, src, set)
	}
	return fmt.Sprintf("%s-%s-%s", kindTR4, src, set)
}

// addL2ControlFlow adds a 10-packet, 1 pps LLDP or GDP flow with the marker source MAC.
func addL2ControlFlow(top gosnappi.Config, portID string, kind pktKind, dstMAC string, etherType uint32, set string) {
	flow := top.Flows().Add().SetName(l2FlowName(kind, portID, set))
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName(portID).SetRxNames([]string{portID})
	flow.Size().SetFixed(controlPktSize)
	flow.Rate().SetPps(1)
	flow.Duration().FixedPackets().SetPackets(controlPktCount)
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(markerMAC)
	eth.Dst().SetValue(dstMAC)
	eth.EtherType().SetValue(etherType)
}

// lagTracerouteFlowName returns the name of a traceroute flow sent from an ATE
// LAG member port.
func lagTracerouteFlowName(portID string, v6 bool, set string) string {
	return tracerouteFlowName("lag-"+portID, v6, set)
}

// dutLAGMACAddress returns the MAC address of DUT lag1.
func dutLAGMACAddress(t *testing.T, s *testState) string {
	t.Helper()
	v, ok := gnmi.Watch(t, s.dut, gnmi.OC().Interface(s.aggID).Ethernet().MacAddress().State(), 2*time.Minute, func(v *ygnmi.Value[string]) bool {
		mac, present := v.Val()
		return present && mac != ""
	}).Await(t)
	if !ok {
		t.Fatalf("DUT %s does not report /interfaces/interface/ethernet/state/mac-address", s.aggID)
	}
	mac, _ := v.Val()
	return mac
}

// addLAGTracerouteFlow adds a raw IPv4 TTL=1 or IPv6 hop-limit=1 flow sent from
// an ATE LAG member port with the marker source MAC, the ATE LAG source address
// and the DUT lag1 destination MAC. Each member port sends half of the LAG
// traceroute packets, with distinct UDP source ports.
func addLAGTracerouteFlow(top gosnappi.Config, portID, dutLAGMAC string, v6 bool, set string) {
	count := uint32(lagTracerouteCnt / 2)
	flow := top.Flows().Add().SetName(lagTracerouteFlowName(portID, v6, set))
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName(portID)
	flow.Size().SetFixed(controlPktSize)
	flow.Rate().SetPps(1)
	flow.Duration().FixedPackets().SetPackets(count)
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(markerMAC)
	eth.Dst().SetValue(dutLAGMAC)
	if v6 {
		ip := flow.Packet().Add().Ipv6()
		ip.Src().SetValue(ateLAG.IPv6)
		ip.Dst().SetValue(dataDstV6)
		ip.HopLimit().SetValue(1)
	} else {
		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue(ateLAG.IPv4)
		ip.Dst().SetValue(dataDstV4)
		ip.TimeToLive().SetValue(1)
	}
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().Increment().SetStart(lagUDPSrcPortBase).SetStep(1).SetCount(count)
	udp.DstPort().SetValue(33434)
}

// addTracerouteFlow adds an IPv4 TTL=1 or IPv6 hop-limit=1 flow with the marker
// source MAC from an ATE device.
func addTracerouteFlow(top gosnappi.Config, src, devName string, v6 bool, count uint32, set string) {
	flow := top.Flows().Add().SetName(tracerouteFlowName(src, v6, set))
	flow.Metrics().SetEnable(true)
	suffix := ".IPv4"
	if v6 {
		suffix = ".IPv6"
	}
	flow.TxRx().Device().SetTxNames([]string{devName + suffix}).SetRxNames([]string{atePort4.Name + suffix})
	flow.Size().SetFixed(controlPktSize)
	flow.Rate().SetPps(1)
	flow.Duration().FixedPackets().SetPackets(count)
	flow.Packet().Add().Ethernet().Src().SetValue(markerMAC)
	srcV4, srcV6 := atePort1.IPv4, atePort1.IPv6
	if v6 {
		ip := flow.Packet().Add().Ipv6()
		ip.Src().SetValue(srcV6)
		ip.Dst().SetValue(dataDstV6)
		ip.HopLimit().SetValue(1)
	} else {
		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue(srcV4)
		ip.Dst().SetValue(dataDstV4)
		ip.TimeToLive().SetValue(1)
	}
	udp := flow.Packet().Add().Udp()
	udp.SrcPort().SetValue(lagUDPSrcPortBase)
	udp.DstPort().SetValue(33434)
}

// setFlowState starts or stops the named flows.
func setFlowState(t *testing.T, s *testState, names []string, state gosnappi.StateTrafficFlowTransmitStateEnum) {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Traffic().FlowTransmit().SetState(state).SetFlowNames(names)
	s.ate.OTG().SetControlState(t, cs)
}

// flowOutPkts returns the cumulative transmitted packet count of a flow, or 0.
func flowOutPkts(t *testing.T, s *testState, name string) uint64 {
	t.Helper()
	v, _ := gnmi.Lookup(t, s.ate.OTG(), gnmi.OTG().Flow(name).Counters().OutPkts().State()).Val()
	return v
}

// dataPlaneCounters returns the current counters of the data plane flows.
func dataPlaneCounters(t *testing.T, s *testState) map[string]flowCounters {
	t.Helper()
	got := map[string]flowCounters{}
	for _, f := range []string{flowIPv4, flowIPv6} {
		c, _ := gnmi.Lookup(t, s.ate.OTG(), gnmi.OTG().Flow(f).Counters().State()).Val()
		got[f] = flowCounters{tx: c.GetOutPkts(), rx: c.GetInPkts()}
	}
	return got
}

// sendFixedFlows starts the given single-use fixed-count flows (flow name ->
// configured packet count), waits until each flow has transmitted at least its
// configured count, stops the flows and returns the number of packets each
// flow actually transmitted. If the data plane flows are running and starting
// the control flows resets their counters, the data plane counters accumulated
// so far are preserved for the loss calculation.
func sendFixedFlows(t *testing.T, s *testState, flows map[string]uint64) map[string]uint64 {
	t.Helper()
	otg := s.ate.OTG()
	var names []string
	for name := range flows {
		names = append(names, name)
	}
	sort.Strings(names)
	before := map[string]uint64{}
	for _, name := range names {
		before[name] = flowOutPkts(t, s, name)
	}
	var dataBefore, dataStarted map[string]flowCounters
	if s.dataRunning {
		dataBefore = dataPlaneCounters(t, s)
	}
	burstStart := time.Now()
	setFlowState(t, s, names, gosnappi.StateTrafficFlowTransmitState.START)
	if s.dataRunning {
		dataStarted = dataPlaneCounters(t, s)
	}
	// The flows send at 1 pps; wait until each flow reaches its configured count.
	for _, name := range names {
		want := before[name] + flows[name]
		if _, ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(name).Counters().OutPkts().State(), 2*time.Minute, func(v *ygnmi.Value[uint64]) bool {
			c, present := v.Val()
			return present && c >= want
		}).Await(t); !ok {
			t.Fatalf("Flow %s did not transmit %d packets within 2m (out-pkts before start %d, now %d); single-use flows must not have been started before", name, flows[name], before[name], flowOutPkts(t, s, name))
		}
	}
	setFlowState(t, s, names, gosnappi.StateTrafficFlowTransmitState.STOP)
	sent := flowsSent(t, s, before)
	for _, name := range names {
		if sent[name] != flows[name] {
			t.Logf("Flow %s: OTG out-pkts counter reports %d packets (configured %d)", name, sent[name], flows[name])
		}
	}
	if s.dataRunning {
		after := dataPlaneCounters(t, s)
		// Without a reset the data plane tx counter grows by about dataPPS per
		// second during the burst. A reset is detected if a counter read right
		// after START or after STOP is below the value read before START, or if
		// the counter grew by less than half of the expected packets.
		minGrowth := uint64(time.Since(burstStart).Seconds() * dataPPS / 2)
		for _, f := range []string{flowIPv4, flowIPv6} {
			if dataStarted[f].tx < dataBefore[f].tx || after[f].tx < dataBefore[f].tx || after[f].tx < dataBefore[f].tx+minGrowth {
				seg := s.dataSeg[f]
				if dataBefore[f].tx < seg.tx || dataBefore[f].rx < seg.rx {
					// The counters were reset when the data plane flows were started.
					seg = flowCounters{}
				}
				t.Logf("Data plane flow %s counters were reset when the control flows started; preserving %d tx / %d rx packets", f, dataBefore[f].tx-seg.tx, dataBefore[f].rx-seg.rx)
				s.dataAcc[f] = flowCounters{tx: s.dataAcc[f].tx + dataBefore[f].tx - seg.tx, rx: s.dataAcc[f].rx + dataBefore[f].rx - seg.rx}
				s.dataSeg[f] = flowCounters{}
			}
		}
	}
	return sent
}

// flowsSent returns the number of packets each flow transmitted since the
// given out-pkts values (or since a counter reset).
func flowsSent(t *testing.T, s *testState, before map[string]uint64) map[string]uint64 {
	t.Helper()
	sent := map[string]uint64{}
	for name, b := range before {
		c := flowOutPkts(t, s, name)
		if c >= b {
			sent[name] = c - b
		} else {
			sent[name] = c
		}
	}
	return sent
}

// flowCounters holds the transmitted and received packet counts of a flow.
type flowCounters struct{ tx, rx uint64 }

// startDataPlane starts the continuous data plane flows and records their
// counters at start time.
func startDataPlane(t *testing.T, s *testState) {
	t.Helper()
	s.dataSeg = dataPlaneCounters(t, s)
	s.dataAcc = map[string]flowCounters{}
	setFlowState(t, s, []string{flowIPv4, flowIPv6}, gosnappi.StateTrafficFlowTransmitState.START)
	s.dataRunning = true
}

// stopDataPlane stops the continuous data plane flows if they are running.
func stopDataPlane(t *testing.T, s *testState) {
	t.Helper()
	if !s.dataRunning {
		return
	}
	setFlowState(t, s, []string{flowIPv4, flowIPv6}, gosnappi.StateTrafficFlowTransmitState.STOP)
	s.dataRunning = false
}

// stopAndVerifyDataPlane stops the data plane flows and verifies 0 packets lost
// since startDataPlane, including packets counted before any counter reset.
func stopAndVerifyDataPlane(t *testing.T, s *testState) {
	t.Helper()
	stopDataPlane(t, s)
	otgutils.LogFlowMetrics(t, s.ate.OTG(), s.top)
	for _, f := range []string{flowIPv4, flowIPv6} {
		tx, rx := otgutils.GetFlowStats(t, s.ate.OTG(), f, time.Minute)
		seg := s.dataSeg[f]
		if tx < seg.tx || rx < seg.rx {
			// The counters were reset when the data plane flows were started.
			seg = flowCounters{}
		}
		tx = s.dataAcc[f].tx + tx - seg.tx
		rx = s.dataAcc[f].rx + rx - seg.rx
		if tx == 0 {
			t.Errorf("Flow %s transmitted no packets", f)
			continue
		}
		if rx != tx {
			t.Errorf("Flow %s lost %d of %d packets across the switchover; want 0 packets lost", f, int64(tx)-int64(rx), tx)
		}
	}
}

// ---------------------------------------------------------------------------
// gRIBI programming and validation.
// ---------------------------------------------------------------------------

// newGRIBIState returns an empty expected gRIBI state.
func newGRIBIState(nis ...string) *gribiState {
	g := &gribiState{v4: map[string]map[string]uint64{}, v6: map[string]map[string]uint64{}, nhgs: map[string]map[uint64]bool{}, nhs: map[string]map[uint64]bool{}}
	for _, ni := range nis {
		g.v4[ni], g.v6[ni], g.nhgs[ni], g.nhs[ni] = map[string]uint64{}, map[string]uint64{}, map[uint64]bool{}, map[uint64]bool{}
	}
	return g
}

// startGRIBIClient connects a PRESERVE / SINGLE_PRIMARY / FIB_ACK gRIBI client,
// retrying until the gRIBI server accepts the connection, and makes it leader.
func startGRIBIClient(t *testing.T, s *testState, timeout time.Duration) *gribi.Client {
	t.Helper()
	c := &gribi.Client{DUT: s.dut, FIBACK: true, Persistence: true}
	retryUntil(t, timeout, "gRIBI client connection", func(tb testing.TB) {
		if err := c.Start(tb); err != nil {
			tb.Fatalf("gRIBI Start failed: %v", err)
		}
	})
	eid := c.BecomeLeader(t)
	// If the new primary controller card lost the gRIBI election state, the
	// learned election ID may not exceed the previous leader's. Use a strictly
	// higher election ID so that an older client is always stale.
	if !eidGreater(eid, s.gribiMaxEID) {
		eid = s.gribiMaxEID.Increment()
		c.UpdateElectionID(t, eid)
	}
	s.gribiMaxEID = eid
	t.Logf("gRIBI client is leader with election ID {high: %d, low: %d}", eid.High, eid.Low)
	return c
}

// eidGreater reports whether election ID a is greater than b.
func eidGreater(a, b gribi.Uint128) bool {
	return a.High > b.High || (a.High == b.High && a.Low > b.Low)
}

// opKey identifies a gRIBI operation result.
func opKey(op constants.OpType, kind string, key string) string {
	return fmt.Sprintf("%v/%s/%s", op, kind, key)
}

// programGRIBI sends entries in batches and verifies that every operation
// listed in wantOps is acknowledged with FIB_PROGRAMMED.
func programGRIBI(t *testing.T, c *gribi.Client, add, del []fluent.GRIBIEntry, wantOps []string) {
	t.Helper()
	before := len(c.Fluent(t).Results(t))
	for _, batch := range []struct {
		entries []fluent.GRIBIEntry
		del     bool
	}{{add, false}, {del, true}} {
		for i := 0; i < len(batch.entries); i += gribiBatchSize {
			end := min(i+gribiBatchSize, len(batch.entries))
			if batch.del {
				c.Fluent(t).Modify().DeleteEntry(t, batch.entries[i:end]...)
			} else {
				c.Fluent(t).Modify().AddEntry(t, batch.entries[i:end]...)
			}
			if err := c.AwaitTimeout(context.Background(), t, gribiOpTimeout); err != nil {
				t.Fatalf("Error waiting for gRIBI operations: %v", err)
			}
		}
	}
	got := map[string]bool{}
	for _, r := range c.Fluent(t).Results(t)[before:] {
		if r.Details == nil || r.ProgrammingResult != gpb.AFTResult_FIB_PROGRAMMED {
			continue
		}
		d := r.Details
		switch {
		case d.IPv4Prefix != "":
			got[opKey(d.Type, "v4", d.IPv4Prefix)] = true
		case d.IPv6Prefix != "":
			got[opKey(d.Type, "v6", d.IPv6Prefix)] = true
		case d.NextHopGroupID != 0:
			got[opKey(d.Type, "nhg", fmt.Sprint(d.NextHopGroupID))] = true
		case d.NextHopIndex != 0:
			got[opKey(d.Type, "nh", fmt.Sprint(d.NextHopIndex))] = true
		}
	}
	var missing []string
	for _, k := range wantOps {
		if !got[k] {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("%d gRIBI operation(s) not acknowledged with FIB_PROGRAMMED, first: %v", len(missing), missing[:min(5, len(missing))])
	}
	t.Logf("All %d gRIBI operations acknowledged with FIB_PROGRAMMED", len(wantOps))
}

// programInitialGRIBI programs the next hops, next-hop-groups, data plane
// routes, scale routes and VRF-1 routes (README "gRIBI programming").
func programInitialGRIBI(t *testing.T, s *testState) {
	t.Helper()
	ni := s.defaultNI
	s.expected = newGRIBIState(ni, vrf1)
	var entries []fluent.GRIBIEntry
	var ops []string

	nhV4, _ := gribi.NHEntry(nhIndexV4, atePort4.IPv4, ni, fluent.InstalledInFIB)
	nhV6, _ := gribi.NHEntry(nhIndexV6, atePort4.IPv6, ni, fluent.InstalledInFIB)
	nhgV4, _ := gribi.NHGEntry(nhgIDV4, map[uint64]uint64{nhIndexV4: 1}, ni, fluent.InstalledInFIB)
	nhgV6, _ := gribi.NHGEntry(nhgIDV6, map[uint64]uint64{nhIndexV6: 1}, ni, fluent.InstalledInFIB)
	entries = append(entries, nhV4, nhV6, nhgV4, nhgV6)
	ops = append(ops, opKey(constants.Add, "nh", fmt.Sprint(nhIndexV4)), opKey(constants.Add, "nh", fmt.Sprint(nhIndexV6)),
		opKey(constants.Add, "nhg", fmt.Sprint(nhgIDV4)), opKey(constants.Add, "nhg", fmt.Sprint(nhgIDV6)))
	s.expected.nhs[ni][nhIndexV4], s.expected.nhs[ni][nhIndexV6] = true, true
	s.expected.nhgs[ni][nhgIDV4], s.expected.nhgs[ni][nhgIDV6] = true, true

	addV4 := func(niName, prefix string, nhg uint64) {
		e := fluent.IPv4Entry().WithPrefix(prefix).WithNetworkInstance(niName).WithNextHopGroup(nhg)
		if niName != ni {
			e.WithNextHopGroupNetworkInstance(ni)
		}
		entries = append(entries, e)
		ops = append(ops, opKey(constants.Add, "v4", prefix))
		s.expected.v4[niName][prefix] = nhg
	}
	addV6 := func(prefix string, nhg uint64) {
		entries = append(entries, fluent.IPv6Entry().WithPrefix(prefix).WithNetworkInstance(ni).WithNextHopGroup(nhg))
		ops = append(ops, opKey(constants.Add, "v6", prefix))
		s.expected.v6[ni][prefix] = nhg
	}

	// Data plane routes.
	addV4(ni, dataPrefixV4, nhgIDV4)
	addV6(dataPrefixV6, nhgIDV6)
	// Scale routes: gribi_scale IPv4 /32 and 1,000 IPv6 /128 in the default NI.
	for _, p := range generateIPv4Prefixes(t, scaleStartV4, *gribiScale) {
		addV4(ni, p, nhgIDV4)
	}
	for _, p := range generateIPv6Prefixes(t, scaleStartV6, scaleCountV6) {
		addV6(p, nhgIDV6)
	}
	// 1,000 IPv4 /32 routes in VRF-1 resolving through NHG 1 in the default NI.
	for _, p := range generateIPv4Prefixes(t, vrf1StartV4, vrf1CountV4) {
		addV4(vrf1, p, nhgIDV4)
	}

	t.Logf("Programming %d gRIBI entries (gribi_scale=%d)", len(entries), *gribiScale)
	programGRIBI(t, s.gribiLeader, entries, nil, ops)
}

// changeCanaryRoutes adds NHG 3, deletes canary routes 1-50 and modifies canary
// routes 51-100 to point to NHG 3 (README P4RT-1.4.1 Step 4).
func changeCanaryRoutes(t *testing.T, s *testState) {
	t.Helper()
	ni := s.defaultNI
	canary := generateIPv4Prefixes(t, scaleStartV4, canaryCount)
	nhg3, _ := gribi.NHGEntry(nhgIDCan, map[uint64]uint64{nhIndexV4: 1}, ni, fluent.InstalledInFIB)
	adds := []fluent.GRIBIEntry{nhg3}
	ops := []string{opKey(constants.Add, "nhg", fmt.Sprint(nhgIDCan))}
	for _, p := range canary[canaryDelete:] {
		adds = append(adds, fluent.IPv4Entry().WithPrefix(p).WithNetworkInstance(ni).WithNextHopGroup(nhgIDCan))
		ops = append(ops, opKey(constants.Add, "v4", p))
		s.expected.v4[ni][p] = nhgIDCan
	}
	var dels []fluent.GRIBIEntry
	for _, p := range canary[:canaryDelete] {
		dels = append(dels, fluent.IPv4Entry().WithPrefix(p).WithNetworkInstance(ni))
		ops = append(ops, opKey(constants.Delete, "v4", p))
		delete(s.expected.v4[ni], p)
	}
	s.expected.nhgs[ni][nhgIDCan] = true
	programGRIBI(t, s.gribiLeader, adds, dels, ops)
}

// testPrefix reports whether an AFT prefix belongs to the ranges used by this test.
func testPrefix(p string) bool {
	_, ipnet, err := net.ParseCIDR(p)
	if err != nil {
		return false
	}
	for _, r := range []string{"198.18.0.0/15", "198.51.100.0/24", "2001:db8:100::/64", "2001:db8:2::/64"} {
		_, rn, _ := net.ParseCIDR(r)
		if rn.Contains(ipnet.IP) {
			return true
		}
	}
	return false
}

// awaitAFT waits until AFT telemetry reflects the expected gRIBI state: every
// expected test prefix is present and references a next-hop-group, no other
// test prefix (for example a deleted canary route) is present, the default
// network instance reports at least the expected number of next-hop-groups and
// the gRIBI next hops are installed. AFT next-hop-group IDs are assigned by the
// DUT and do not have to match the gRIBI NHG IDs, so the exact prefix to NHG
// mapping is validated with gRIBI Get.
func awaitAFT(t *testing.T, s *testState, timeout time.Duration) {
	t.Helper()
	nhIPs := []string{atePort4.IPv4, atePort4.IPv6}
	retryUntil(t, timeout, "AFT telemetry convergence", func(tb testing.TB) {
		for _, ni := range []string{s.defaultNI, vrf1} {
			afts := gnmi.OC().NetworkInstance(ni).Afts()
			if err := compareAFTPrefixes(ni, "IPv4", s.expected.v4[ni], aftIPv4Entries(tb, s.dut, ni)); err != nil {
				tb.Fatal(err)
			}
			if len(s.expected.v6[ni]) > 0 {
				if err := compareAFTPrefixes(ni, "IPv6", s.expected.v6[ni], aftIPv6Entries(tb, s.dut, ni)); err != nil {
					tb.Fatal(err)
				}
			}
			if ni != s.defaultNI {
				continue
			}
			if got, want := len(gnmi.GetAll(tb, s.dut, afts.NextHopGroupAny().State())), len(s.expected.nhgs[ni]); got < want {
				tb.Fatalf("AFT reports %d next-hop-groups in %s, want at least %d", got, ni, want)
			}
			gotNH := map[string]bool{}
			for _, nh := range gnmi.GetAll(tb, s.dut, afts.NextHopAny().State()) {
				gotNH[nh.GetIpAddress()] = true
			}
			for _, ip := range nhIPs {
				if !gotNH[ip] {
					tb.Fatalf("AFT next hop %s missing in %s", ip, ni)
				}
			}
		}
	})
}

// aftIPv4Entries returns the IPv4 test prefixes reported by AFT telemetry in ni
// and the next-hop-group each of them references.
func aftIPv4Entries(t testing.TB, dut *ondatra.DUTDevice, ni string) map[string]uint64 {
	t.Helper()
	got := map[string]uint64{}
	// LookupAll is used because the AFT may be empty (for example after a flush).
	for _, v := range gnmi.LookupAll(t, dut, gnmi.OC().NetworkInstance(ni).Afts().Ipv4EntryAny().State()) {
		if e, ok := v.Val(); ok && testPrefix(e.GetPrefix()) {
			got[e.GetPrefix()] = e.GetNextHopGroup()
		}
	}
	return got
}

// aftIPv6Entries returns the IPv6 test prefixes reported by AFT telemetry in ni
// and the next-hop-group each of them references.
func aftIPv6Entries(t testing.TB, dut *ondatra.DUTDevice, ni string) map[string]uint64 {
	t.Helper()
	got := map[string]uint64{}
	for _, v := range gnmi.LookupAll(t, dut, gnmi.OC().NetworkInstance(ni).Afts().Ipv6EntryAny().State()) {
		if e, ok := v.Val(); ok && testPrefix(e.GetPrefix()) {
			got[e.GetPrefix()] = e.GetNextHopGroup()
		}
	}
	return got
}

// compareAFTPrefixes compares the expected test prefixes with the test prefixes
// reported by AFT telemetry and verifies that each references a next-hop-group.
func compareAFTPrefixes(ni, family string, want, got map[string]uint64) error {
	for p := range want {
		nhg, ok := got[p]
		if !ok {
			return fmt.Errorf("%s %s AFT prefix %s missing (got %d test prefixes, want %d)", ni, family, p, len(got), len(want))
		}
		if nhg == 0 {
			return fmt.Errorf("%s %s AFT prefix %s does not reference a next-hop-group", ni, family, p)
		}
	}
	for p := range got {
		if _, ok := want[p]; !ok {
			return fmt.Errorf("%s %s AFT prefix %s present but not expected (stale or deleted)", ni, family, p)
		}
	}
	return nil
}

// gribiGetValidation issues a gRIBI Get for all network instances and all AFTs
// with get_deadline and verifies the README "gRIBI Get validation" criteria.
func gribiGetValidation(t *testing.T, s *testState) {
	t.Helper()
	// Each Get must complete within get_deadline and return exactly the
	// expected entries. If the only errors are entries reporting fib_status
	// NOT_PROGRAMMED (while RIB content is correct), the Get is repeated for up
	// to gribi_reconnect_timeout so that a slow FIB status update after a
	// switchover can be told apart from a persistent FIB status error. The
	// interval only paces repeated Get RPCs; there is no telemetry to watch for
	// the gRIBI FIB status.
	timeout := s.reconnectTimeout(*gribiReconnectTimeout)
	start := time.Now()
	for attempt := 1; ; attempt++ {
		errs, fibErrs := gribiGetOnce(t, s)
		if len(errs) == 0 && fibErrs == 0 {
			if attempt > 1 {
				t.Logf("gRIBI Get: all entries report fib_status PROGRAMMED after %d attempt(s) in %v", attempt, time.Since(start).Round(time.Second))
			}
			return
		}
		if len(errs) > 0 || time.Since(start) > timeout {
			if fibErrs > 0 {
				errs = append(errs, fmt.Sprintf("%d entries report fib_status NOT_PROGRAMMED after %d attempt(s) in %v (fib_status convergence window %v)", fibErrs, attempt, time.Since(start).Round(time.Second), timeout))
			}
			t.Errorf("gRIBI Get validation found %d error(s); first errors:\n%s", len(errs), strings.Join(errs[:min(20, len(errs))], "\n"))
			return
		}
		t.Logf("gRIBI Get attempt %d: RIB content is correct but %d entries report fib_status NOT_PROGRAMMED; repeating the Get (fib_status convergence window %v)", attempt, fibErrs, timeout)
		time.Sleep(fibStatusRetryInterval)
	}
}

// gribiGetOnce issues one gRIBI Get for all network instances and all AFTs
// with get_deadline. It returns the content errors (missing, extra, duplicate
// or mis-pointed entries) and the number of entries reporting a fib_status
// other than PROGRAMMED. A Get that does not complete within get_deadline is a
// fatal error.
func gribiGetOnce(t *testing.T, s *testState) ([]string, int) {
	t.Helper()
	stub := s.dut.RawAPIs().GRIBI(t)
	ctx, cancel := context.WithTimeout(context.Background(), *getDeadline)
	defer cancel()

	start := time.Now()
	stream, err := stub.Get(ctx, &gpb.GetRequest{
		NetworkInstance: &gpb.GetRequest_All{All: &gpb.Empty{}},
		Aft:             gpb.AFTType_ALL,
	})
	if err != nil {
		t.Fatalf("gRIBI Get RPC could not be started: %v", err)
	}
	got := newGRIBIState(s.defaultNI, vrf1)
	var errs, fibErrs []string
	total := 0
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if status.Code(err) == codes.DeadlineExceeded {
				t.Fatalf("gRIBI Get did not complete within get_deadline %v (DEADLINE_EXCEEDED) after receiving %d entries", *getDeadline, total)
			}
			t.Fatalf("gRIBI Get failed with %v after receiving %d entries", err, total)
		}
		for _, e := range resp.GetEntry() {
			total++
			ni := e.GetNetworkInstance()
			if _, ok := got.v4[ni]; !ok {
				errs = append(errs, fmt.Sprintf("unexpected entry in network instance %q: %v", ni, e))
				continue
			}
			if e.GetFibStatus() != gpb.AFTEntry_PROGRAMMED {
				fibErrs = append(fibErrs, fmt.Sprintf("entry %v reports fib_status %v, want PROGRAMMED", e, e.GetFibStatus()))
			}
			var dup bool
			switch {
			case e.GetIpv4() != nil:
				p := e.GetIpv4().GetPrefix()
				_, dup = got.v4[ni][p]
				got.v4[ni][p] = e.GetIpv4().GetIpv4Entry().GetNextHopGroup().GetValue()
			case e.GetIpv6() != nil:
				p := e.GetIpv6().GetPrefix()
				_, dup = got.v6[ni][p]
				got.v6[ni][p] = e.GetIpv6().GetIpv6Entry().GetNextHopGroup().GetValue()
			case e.GetNextHopGroup() != nil:
				id := e.GetNextHopGroup().GetId()
				dup = got.nhgs[ni][id]
				got.nhgs[ni][id] = true
			case e.GetNextHop() != nil:
				idx := e.GetNextHop().GetIndex()
				dup = got.nhs[ni][idx]
				got.nhs[ni][idx] = true
			default:
				errs = append(errs, fmt.Sprintf("unexpected entry type: %v", e))
			}
			if dup {
				errs = append(errs, fmt.Sprintf("duplicate entry returned: %v", e))
			}
		}
	}
	latency := time.Since(start)
	t.Logf("gRIBI Get returned %d entries in %v (get_deadline %v)", total, latency, *getDeadline)

	for _, ni := range []string{s.defaultNI, vrf1} {
		errs = append(errs, diffPrefixes(ni, "IPv4", s.expected.v4[ni], got.v4[ni])...)
		errs = append(errs, diffPrefixes(ni, "IPv6", s.expected.v6[ni], got.v6[ni])...)
		errs = append(errs, diffIDs(ni, "next-hop-group", s.expected.nhgs[ni], got.nhgs[ni])...)
		errs = append(errs, diffIDs(ni, "next hop", s.expected.nhs[ni], got.nhs[ni])...)
	}
	if len(fibErrs) > 0 {
		t.Logf("gRIBI Get: %d of %d entries report a fib_status other than PROGRAMMED; first: %s", len(fibErrs), total, strings.Join(fibErrs[:min(3, len(fibErrs))], "; "))
	}
	return errs, len(fibErrs)
}

// diffPrefixes returns errors for missing, extra or mis-pointed prefixes.
func diffPrefixes(ni, family string, want, got map[string]uint64) []string {
	var errs []string
	for p, nhg := range want {
		g, ok := got[p]
		switch {
		case !ok:
			errs = append(errs, fmt.Sprintf("%s %s prefix %s missing from Get", ni, family, p))
		case g != nhg:
			errs = append(errs, fmt.Sprintf("%s %s prefix %s references NHG %d, want %d", ni, family, p, g, nhg))
		}
	}
	for p := range got {
		if _, ok := want[p]; !ok {
			errs = append(errs, fmt.Sprintf("%s %s prefix %s returned by Get but not expected (stale or deleted)", ni, family, p))
		}
	}
	sort.Strings(errs)
	return errs
}

// diffIDs returns errors for missing or extra NHG / next hop IDs.
func diffIDs(ni, kind string, want, got map[uint64]bool) []string {
	var errs []string
	for id := range want {
		if !got[id] {
			errs = append(errs, fmt.Sprintf("%s %s %d missing from Get", ni, kind, id))
		}
	}
	for id := range got {
		if !want[id] {
			errs = append(errs, fmt.Sprintf("%s %s %d returned by Get but not expected", ni, kind, id))
		}
	}
	return errs
}

// ---------------------------------------------------------------------------
// P4RT programming and validation.
// ---------------------------------------------------------------------------

// puntEntries returns the 4 punt entries (README "P4RT programming").
func puntEntries(op p4pb.Update_Type) []*p4rtutils.ACLWbbIngressTableEntryInfo {
	return []*p4rtutils.ACLWbbIngressTableEntryInfo{
		{Type: op, EtherType: lldpEtherType, EtherTypeMask: 0xFFFF, Priority: 1},
		{Type: op, EtherType: gdpEtherType, EtherTypeMask: 0xFFFF, Priority: 1},
		{Type: op, IsIpv4: 0x1, TTL: 0x1, TTLMask: 0xFF, Priority: 1},
		{Type: op, IsIpv6: 0x1, TTL: 0x1, TTLMask: 0xFF, Priority: 1},
	}
}

// destroyP4RTStreams closes the primary and secondary streams of the current
// P4RT session, if any. Errors are ignored: after a switchover the streams are
// usually already terminated.
func destroyP4RTStreams(s *testState) {
	if s.p4 == nil {
		return
	}
	for _, n := range s.nodes {
		for _, rc := range []struct {
			c    *p4rt_client.P4RTClient
			role string
		}{{s.p4.primary, "primary"}, {s.p4.secondary, "secondary"}} {
			if rc.c == nil {
				continue
			}
			name := streamName(rc.role, n.deviceID)
			rc.c.StreamChannelDestroy(&name)
		}
	}
}

// connectP4RT connects primary and secondary P4RT clients to every node with the
// given election IDs, retrying until the P4RT server accepts the arbitration.
// Streams created by a failed attempt are destroyed before the next attempt so
// that the election IDs are not held by a stale stream.
func connectP4RT(t *testing.T, s *testState, primaryEID, secondaryEID uint64, timeout time.Duration) {
	t.Helper()
	destroyP4RTStreams(s)
	s.p4 = nil
	sess := &p4rtSession{primaryEID: primaryEID, secondaryEID: secondaryEID}
	retryUntil(t, timeout, "P4RT StreamChannel arbitration", func(tb testing.TB) {
		primary := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
		secondary := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
		type stream struct {
			c    *p4rt_client.P4RTClient
			name string
		}
		var created []stream
		success := false
		defer func() {
			if success {
				return
			}
			for _, st := range created {
				st.c.StreamChannelDestroy(&st.name)
			}
		}()
		for _, c := range []*p4rt_client.P4RTClient{primary, secondary} {
			if err := c.P4rtClientSet(s.dut.RawAPIs().P4RT(tb)); err != nil {
				tb.Fatalf("Could not initialize P4RT client: %v", err)
			}
		}
		for _, n := range s.nodes {
			pName, sName := streamName("primary", n.deviceID), streamName("secondary", n.deviceID)
			created = append(created, stream{primary, pName})
			if err := p4rtutils.StreamArbitrate(primary, &p4rt_client.P4RTStreamParameters{Name: pName, DeviceId: n.deviceID, ElectionIdL: primaryEID}, time.Minute); err != nil {
				tb.Fatalf("Primary arbitration on node %s failed: %v", n.name, err)
			}
			created = append(created, stream{secondary, sName})
			err := p4rtutils.StreamArbitrate(secondary, &p4rt_client.P4RTStreamParameters{Name: sName, DeviceId: n.deviceID, ElectionIdL: secondaryEID}, time.Minute)
			if err := p4rtutils.CheckRPCErrorCode(err, codes.AlreadyExists); err != nil {
				tb.Fatalf("Secondary client on node %s should not become primary: %v", n.name, err)
			}
		}
		success = true
		sess.primary, sess.secondary = primary, secondary
	})
	s.p4 = sess
}

// pushPipelineAndEntries pushes the forwarding pipeline and writes the 4 punt
// entries from the primary client. ALREADY_EXISTS is accepted for punt entries
// that persisted across a switchover.
func pushPipelineAndEntries(t *testing.T, s *testState) {
	t.Helper()
	pushPipeline(t, s)
	s.puntWritten = true
	writePuntEntries(t, s, p4pb.Update_INSERT, codes.AlreadyExists)
}

// pushPipeline pushes the forwarding pipeline (VERIFY_AND_COMMIT) from the
// primary client to every node.
func pushPipeline(t *testing.T, s *testState) {
	t.Helper()
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		t.Fatalf("Could not load P4Info file %q: %v", *p4InfoFile, err)
	}
	eid := &p4pb.Uint128{Low: s.p4.primaryEID}
	for _, n := range s.nodes {
		if err := s.p4.primary.SetForwardingPipelineConfig(p4rtutils.SetForwardingPipelineConfigGet(n.deviceID, eid, p4Info, p4rtCookie)); err != nil {
			t.Fatalf("SetForwardingPipelineConfig on node %s failed: %v", n.name, err)
		}
	}
}

// p4rtErrorHasCode reports whether a P4RT Write error carries code, either as
// the top-level gRPC status code or as a per-update p4.v1.Error detail.
func p4rtErrorHasCode(err error, code codes.Code) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	if st.Code() == code {
		return true
	}
	for _, d := range st.Details() {
		if pe, ok := d.(*p4pb.Error); ok && codes.Code(pe.GetCanonicalCode()) == code {
			return true
		}
	}
	return false
}

// writePuntEntries inserts or deletes the punt entries on every node. Errors
// carrying one of the tolerated codes are accepted.
func writePuntEntries(t testing.TB, s *testState, op p4pb.Update_Type, tolerated ...codes.Code) {
	t.Helper()
	for _, n := range s.nodes {
		for _, u := range p4rtutils.ACLWbbIngressTableEntryGet(puntEntries(op)) {
			err := s.p4.primary.Write(&p4pb.WriteRequest{
				DeviceId:   n.deviceID,
				ElectionId: &p4pb.Uint128{Low: s.p4.primaryEID},
				Updates:    []*p4pb.Update{u},
				Atomicity:  p4pb.WriteRequest_CONTINUE_ON_ERROR,
			})
			if err == nil {
				continue
			}
			accepted := false
			for _, c := range tolerated {
				if p4rtErrorHasCode(err, c) {
					accepted = true
				}
			}
			if !accepted {
				t.Fatalf("P4RT Write (%v) on node %s failed: %v", op, n.name, err)
			}
		}
	}
}

// tableEntryKey returns a canonical string for a table entry: table ID,
// priority, match fields (sorted, with leading zero bytes removed as allowed by
// the P4Runtime canonical bytestring representation) and action ID.
func tableEntryKey(te *p4pb.TableEntry) string {
	canon := func(b []byte) string { return fmt.Sprintf("%x", bytes.TrimLeft(b, "\x00")) }
	var fields []string
	for _, m := range te.GetMatch() {
		var v string
		switch {
		case m.GetTernary() != nil:
			v = canon(m.GetTernary().GetValue()) + "&" + canon(m.GetTernary().GetMask())
		case m.GetExact() != nil:
			v = canon(m.GetExact().GetValue())
		case m.GetOptional() != nil:
			v = canon(m.GetOptional().GetValue())
		default:
			v = m.String()
		}
		fields = append(fields, fmt.Sprintf("%d=%s", m.GetFieldId(), v))
	}
	sort.Strings(fields)
	return fmt.Sprintf("table=%d priority=%d match=[%s] action=%d", te.GetTableId(), te.GetPriority(), strings.Join(fields, ","), te.GetAction().GetAction().GetActionId())
}

// verifyPuntEntries verifies with P4RT Read that every node holds exactly the 4
// punt entries (installed == true) or no punt entry (installed == false). When
// no entry is expected, a Read rejected because no pipeline is set is accepted.
func verifyPuntEntries(t testing.TB, s *testState, installed bool) {
	t.Helper()
	want := map[string]int{}
	if installed {
		for _, u := range p4rtutils.ACLWbbIngressTableEntryGet(puntEntries(p4pb.Update_INSERT)) {
			want[tableEntryKey(u.GetEntity().GetTableEntry())]++
		}
	}
	for _, n := range s.nodes {
		entities, err := p4rtutils.ReadTableEntries(s.p4.primary, n.deviceID, p4rtutils.WbbTableMap["acl_wbb_ingress_table"])
		if err != nil {
			if !installed && status.Code(err) == codes.FailedPrecondition {
				t.Logf("P4RT Read on node %s: no pipeline is set (%v)", n.name, err)
				continue
			}
			t.Fatalf("P4RT Read on node %s failed: %v", n.name, err)
		}
		got := map[string]int{}
		for _, e := range entities {
			got[tableEntryKey(e.GetTableEntry())]++
		}
		for k, c := range want {
			if got[k] != c {
				t.Errorf("P4RT Read on node %s: punt entry %s found %d time(s), want %d", n.name, k, got[k], c)
			}
		}
		for k, c := range got {
			if _, ok := want[k]; !ok {
				t.Errorf("P4RT Read on node %s: unexpected entry %s (%d time(s))", n.name, k, c)
			}
		}
	}
}

// drainPacketIns discards all PacketIn messages buffered on a client's streams.
func drainPacketIns(c *p4rt_client.P4RTClient, role string, nodes []p4rtNode) {
	for _, n := range nodes {
		name := streamName(role, n.deviceID)
		c.StreamChannelGetPackets(&name, 0, 0)
	}
}

// packetIn is a decoded marker PacketIn message.
type packetIn struct {
	kind    pktKind
	ingress string
	egress  string
}

// classify decodes a PacketIn and returns it if it carries the marker source MAC.
func classify(pkt *p4pb.PacketIn) (packetIn, bool) {
	p := gopacket.NewPacket(pkt.GetPayload(), layers.LayerTypeEthernet, gopacket.Default)
	eth, ok := p.Layer(layers.LayerTypeEthernet).(*layers.Ethernet)
	if !ok || !strings.EqualFold(eth.SrcMAC.String(), markerMAC) {
		return packetIn{}, false
	}
	var res packetIn
	switch eth.EthernetType {
	case layers.EthernetType(lldpEtherType):
		res.kind = kindLLDP
	case layers.EthernetType(gdpEtherType):
		res.kind = kindGDP
	case layers.EthernetTypeIPv4:
		res.kind = kindTR4
	case layers.EthernetTypeIPv6:
		res.kind = kindTR6
	default:
		return packetIn{}, false
	}
	for _, m := range pkt.GetMetadata() {
		switch m.GetMetadataId() {
		case metadataIngressPort:
			res.ingress = string(m.GetValue())
		case metadataEgressPort:
			res.egress = string(m.GetValue())
		}
	}
	return res, true
}

// collectPacketIns collects marker PacketIns from a client's streams until want
// messages are received or timeout expires. Each stream read waits for at most
// pollWindow (split across nodes) and never beyond the deadline. Once the
// deadline is reached (or for a timeout of 0), the PacketIns already buffered
// on the streams are drained without waiting (minimum sequence number 0), which
// avoids blocking on a stream that receives no PacketIn. Streams that report an
// error (for example a terminated stream) are not polled again.
func collectPacketIns(c *p4rt_client.P4RTClient, role string, nodes []p4rtNode, want int, timeout time.Duration) []packetIn {
	const (
		pollWindow = 2 * time.Second
		minWait    = 50 * time.Millisecond
	)
	var got []packetIn
	dead := map[uint64]bool{}
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		for _, n := range nodes {
			if dead[n.deviceID] {
				continue
			}
			name := streamName(role, n.deviceID)
			var pkts []*p4rt_client.P4RTPacketInfo
			var err error
			if remaining <= 0 {
				_, pkts, err = c.StreamChannelGetPackets(&name, 0, 0)
			} else {
				wait := max(min(pollWindow/time.Duration(len(nodes)), remaining), minWait)
				_, pkts, err = c.StreamChannelGetPackets(&name, 1<<62, wait)
			}
			// A read that times out returns os.ErrDeadlineExceeded; any other
			// error means that the stream is no longer usable.
			if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) && len(pkts) == 0 {
				dead[n.deviceID] = true
			}
			for _, pkt := range pkts {
				if pkt == nil {
					continue
				}
				if pi, ok := classify(pkt.Pkt); ok {
					got = append(got, pi)
				}
			}
		}
		if (want > 0 && len(got) >= want) || remaining <= 0 || len(dead) == len(nodes) {
			return got
		}
	}
}

// noPuntCheck sends LLDP and GDP from ATE port-1 before any punt entry exists
// and verifies that no matching PacketIn is received (README "No-punt check").
func noPuntCheck(t *testing.T, s *testState) {
	t.Helper()
	drainPacketIns(s.p4.primary, "primary", s.nodes)
	p1 := s.ate.Port(t, "port1").ID()
	sendFixedFlows(t, s, map[string]uint64{
		l2FlowName(kindLLDP, p1, noPuntFlowSet): controlPktCount,
		l2FlowName(kindGDP, p1, noPuntFlowSet):  controlPktCount,
	})
	if got := collectPacketIns(s.p4.primary, "primary", s.nodes, 0, *packetinNegativeWindow); len(got) != 0 {
		t.Fatalf("No-punt check: received %d matching PacketIn messages before any punt entry was installed, want 0", len(got))
	}
}

// packetInOutValidation runs the README "PacketIn and PacketOut validation
// procedure" on the given ATE/DUT ports (subset of port1, port2, port3).
func packetInOutValidation(t *testing.T, s *testState, ports []string, extraNoPacketIn ...*p4rt_client.P4RTClient) {
	t.Helper()
	dut := s.dut
	drainPacketIns(s.p4.primary, "primary", s.nodes)
	drainPacketIns(s.p4.secondary, "secondary", s.nodes)

	// Record in-pkts on the DUT ports under test.
	before := map[string]uint64{}
	for _, pid := range ports {
		before[pid] = gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, pid).Name()).Counters().InPkts().State())
	}

	// Each validation uses its own single-use control flow set.
	if s.validations >= maxValidations {
		t.Fatalf("PacketIn validation procedure run more than %d times; increase maxValidations", maxValidations)
	}
	set := validationFlowSet(s.validations)
	s.validations++

	// LLDP and GDP from each ATE port under test, traceroute from ATE port-1 and
	// (when the LAG members are under test) from the ATE LAG members. Each flow
	// maps to its PacketIn key (kind/ingress_singleton_port, or kind/lag for
	// traceroute from the ATE LAG) and to the DUT port it is sent to.
	type flowInfo struct {
		key, dutPort string
	}
	flows := map[string]uint64{}
	info := map[string]flowInfo{}
	add := func(name string, count uint64, key, dutPort string) {
		flows[name] = count
		info[name] = flowInfo{key: key, dutPort: dutPort}
	}
	for _, pid := range ports {
		atePID := s.ate.Port(t, pid).ID()
		id := fmt.Sprint(portIDs[pid])
		add(l2FlowName(kindLLDP, atePID, set), controlPktCount, string(kindLLDP)+"/"+id, pid)
		add(l2FlowName(kindGDP, atePID, set), controlPktCount, string(kindGDP)+"/"+id, pid)
	}
	if contains(ports, "port1") {
		id := fmt.Sprint(port1ID)
		add(tracerouteFlowName("port1", false, set), controlPktCount, string(kindTR4)+"/"+id, "port1")
		add(tracerouteFlowName("port1", true, set), controlPktCount, string(kindTR6)+"/"+id, "port1")
	}
	lagUnderTest := contains(ports, "port2") || contains(ports, "port3")
	if lagUnderTest {
		for _, pid := range []string{"port2", "port3"} {
			atePID := s.ate.Port(t, pid).ID()
			add(lagTracerouteFlowName(atePID, false, set), lagTracerouteCnt/2, string(kindTR4)+"/lag", pid)
			add(lagTracerouteFlowName(atePID, true, set), lagTracerouteCnt/2, string(kindTR6)+"/lag", pid)
		}
	}
	sendFixedFlows(t, s, flows)

	// Expected PacketIn counts: one PacketIn per packet sent. Each flow is
	// single use and sends exactly its configured fixed packet count; the OTG
	// out-pkts counter is only logged, because some OTG implementations report
	// inflated counters for raw flows sent from LAG member ports.
	want := map[string]int{} // kind/ingress -> count
	lagTR := map[pktKind]int{}
	sentToPort := map[string]uint64{}
	total := 0
	for name, n := range flows {
		fi := info[name]
		switch fi.key {
		case string(kindTR4) + "/lag":
			lagTR[kindTR4] += int(n)
		case string(kindTR6) + "/lag":
			lagTR[kindTR6] += int(n)
		default:
			want[fi.key] += int(n)
		}
		sentToPort[fi.dutPort] += n
		total += int(n)
	}
	got := collectPacketIns(s.p4.primary, "primary", s.nodes, total, *packetinTimeout)
	// Collect any PacketIn for packets counted after the first read, then
	// drain so that extra PacketIn messages are reported.
	got = append(got, collectPacketIns(s.p4.primary, "primary", s.nodes, 0, 0)...)

	// PacketIn counts and metadata.
	gotCount := map[string]int{}
	gotLAG := map[pktKind]int{}
	lagIDs := map[string]bool{fmt.Sprint(port2ID): true, fmt.Sprint(port3ID): true}
	for _, pi := range got {
		if (pi.kind == kindTR4 || pi.kind == kindTR6) && lagIDs[pi.ingress] {
			gotLAG[pi.kind]++
		} else {
			gotCount[string(pi.kind)+"/"+pi.ingress]++
		}
		if (pi.kind == kindTR4 || pi.kind == kindTR6) && pi.egress != fmt.Sprint(port4ID) {
			t.Errorf("Traceroute PacketIn egress_singleton_port = %q, want %q", pi.egress, fmt.Sprint(port4ID))
		}
		if pi.ingress == "" {
			t.Errorf("PacketIn %v has no ingress_singleton_port metadata", pi.kind)
		}
	}
	for k, n := range want {
		if gotCount[k] != n {
			t.Errorf("PacketIn count for %s (kind/ingress_singleton_port): got %d, want %d", k, gotCount[k], n)
		}
	}
	for k, n := range gotCount {
		if _, ok := want[k]; !ok {
			t.Errorf("Unexpected PacketIn %s (kind/ingress_singleton_port): %d message(s)", k, n)
		}
	}
	for k, n := range lagTR {
		if gotLAG[k] != n {
			t.Errorf("PacketIn count for %s received on lag1 members: got %d, want %d", k, gotLAG[k], n)
		}
	}

	// The secondary client (and any additional client, e.g. a stale client)
	// receives no matching PacketIn.
	if n := len(collectPacketIns(s.p4.secondary, "secondary", s.nodes, 0, 0)); n != 0 {
		t.Errorf("Secondary P4RT client received %d matching PacketIn messages, want 0", n)
	}
	for _, c := range extraNoPacketIn {
		if n := len(collectPacketIns(c, "stale", s.nodes, 0, 0)); n != 0 {
			t.Errorf("Stale P4RT client received %d matching PacketIn messages, want 0", n)
		}
	}

	// DUT in-pkts increased by at least the number of packets sent to each port.
	for _, pid := range ports {
		path := gnmi.OC().Interface(dut.Port(t, pid).Name()).Counters().InPkts().State()
		wantMin := before[pid] + sentToPort[pid]
		_, ok := gnmi.Watch(t, dut, path, time.Minute, func(v *ygnmi.Value[uint64]) bool {
			c, present := v.Val()
			return present && c >= wantMin
		}).Await(t)
		if !ok {
			t.Errorf("DUT %s in-pkts did not increase by at least %d", pid, wantMin-before[pid])
		}
	}

	packetOutValidation(t, s)
}

// packetOutValidation sends 10 LLDP and 10 GDP PacketOut messages to DUT port-1
// and DUT port-2 and verifies that ATE port-1 and ATE port-2 receive exactly 20
// packets each.
func packetOutValidation(t *testing.T, s *testState) {
	t.Helper()
	otg := s.ate.OTG()
	inFrames := func(pid string) *ygnmi.Value[uint64] {
		return gnmi.Lookup(t, otg, gnmi.OTG().Port(s.ate.Port(t, pid).ID()).Counters().InFrames().State())
	}
	before := map[string]uint64{}
	for _, pid := range []string{"port1", "port2"} {
		v, _ := inFrames(pid).Val()
		before[pid] = v
	}
	cs := otgutils.StartCapture(t, otg)
	for _, pid := range []string{"port1", "port2"} {
		node := s.nodeOfPort[pid]
		name := streamName("primary", node.deviceID)
		for _, f := range []struct {
			dst string
			et  layers.EthernetType
		}{{lldpDstMAC, lldpEtherType}, {gdpDstMAC, gdpEtherType}} {
			payload := l2Frame(t, packetOutMAC, f.dst, f.et)
			for i := 0; i < controlPktCount; i++ {
				if err := s.p4.primary.StreamChannelSendMsg(&name, p4rtutils.PacketOutWithEgressPortGet(payload, portIDs[pid], false)); err != nil {
					t.Fatalf("PacketOut to DUT %s failed: %v", pid, err)
				}
			}
		}
	}
	// Wait until ATE port-1 and ATE port-2 have received at least the PacketOut
	// frames before the capture is stopped.
	for _, pid := range []string{"port1", "port2"} {
		want := before[pid] + 2*controlPktCount
		if _, ok := gnmi.Watch(t, otg, gnmi.OTG().Port(s.ate.Port(t, pid).ID()).Counters().InFrames().State(), 30*time.Second, func(v *ygnmi.Value[uint64]) bool {
			c, present := v.Val()
			return present && c >= want
		}).Await(t); !ok {
			t.Logf("ATE %s in-frames did not increase by %d within 30s; validating the capture", pid, 2*controlPktCount)
		}
	}
	otgutils.StopCapture(t, otg, cs)
	for _, pid := range []string{"port1", "port2"} {
		if got := countCapturedPacketOuts(t, s, s.ate.Port(t, pid).ID()); got != 2*controlPktCount {
			t.Errorf("ATE %s received %d PacketOut frames, want %d", pid, got, 2*controlPktCount)
		}
	}
}

// l2Frame serializes an Ethernet frame with the given addresses and EtherType.
func l2Frame(t *testing.T, src, dst string, et layers.EthernetType) []byte {
	t.Helper()
	srcMAC, _ := net.ParseMAC(src)
	dstMAC, _ := net.ParseMAC(dst)
	buf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true},
		&layers.Ethernet{SrcMAC: srcMAC, DstMAC: dstMAC, EthernetType: et},
		gopacket.Payload(make([]byte, 64))); err != nil {
		t.Fatalf("Could not serialize frame: %v", err)
	}
	return buf.Bytes()
}

// countCapturedPacketOuts counts captured frames carrying the PacketOut source MAC.
func countCapturedPacketOuts(t *testing.T, s *testState, portID string) int {
	t.Helper()
	data := s.ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(portID))
	r, err := pcapgo.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Could not read the capture of ATE %s: %v", portID, err)
	}
	n := 0
	for pkt := range gopacket.NewPacketSource(r, r.LinkType()).Packets() {
		if eth, ok := pkt.Layer(layers.LayerTypeEthernet).(*layers.Ethernet); ok && strings.EqualFold(eth.SrcMAC.String(), packetOutMAC) {
			n++
		}
	}
	return n
}

// checkPipelineBeforeRepush issues GetForwardingPipelineConfig and Read on every
// node before any re-push (README P4RT-1.4.1 Step 2). A node passes if it
// reports that no pipeline is set, or a pipeline with no punt entry or with all
// 4 punt entries. If the node hosting DUT port-1 reports the pipeline and all 4
// punt entries as present, punting on DUT port-1 must work before the re-push.
func checkPipelineBeforeRepush(t *testing.T, s *testState) {
	t.Helper()
	port1Node := s.nodeOfPort["port1"].deviceID
	port1Ready := false
	for _, n := range s.nodes {
		resp, err := s.p4.primary.GetForwardingPipelineConfig(&p4pb.GetForwardingPipelineConfigRequest{
			DeviceId:     n.deviceID,
			ResponseType: p4pb.GetForwardingPipelineConfigRequest_ALL,
		})
		if err != nil {
			if c := status.Code(err); c == codes.FailedPrecondition || c == codes.NotFound {
				t.Logf("Node %s: GetForwardingPipelineConfig reports no pipeline set (%v)", n.name, err)
				continue
			}
			t.Errorf("Node %s: GetForwardingPipelineConfig failed: %v", n.name, err)
			continue
		}
		if resp.GetConfig().GetP4Info() == nil {
			t.Logf("Node %s: no forwarding pipeline is set after the switchover", n.name)
			continue
		}
		entities, err := p4rtutils.ReadTableEntries(s.p4.primary, n.deviceID, p4rtutils.WbbTableMap["acl_wbb_ingress_table"])
		if err != nil {
			t.Errorf("Node %s: pipeline is set but P4RT Read failed: %v", n.name, err)
			continue
		}
		t.Logf("Node %s: pipeline is set and P4RT Read returned %d punt entries before re-push", n.name, len(entities))
		switch len(entities) {
		case 0:
		case len(puntEntries(p4pb.Update_INSERT)):
			if n.deviceID == port1Node {
				port1Ready = true
			}
		default:
			t.Errorf("Node %s: pipeline is set with %d punt entries, want 0 or %d (partial state)", n.name, len(entities), len(puntEntries(p4pb.Update_INSERT)))
		}
	}
	if port1Ready {
		t.Log("Pipeline and punt entries reported as present on the node hosting DUT port-1 before re-push; validating PacketIn on DUT port-1")
		packetInOutValidation(t, s, []string{"port1"})
	} else {
		t.Log("The node hosting DUT port-1 does not report a complete pipeline; the pipeline and punt entries are re-pushed in the next step")
	}
}

// ---------------------------------------------------------------------------
// Supervisor switchover helpers.
// ---------------------------------------------------------------------------

// findRoles returns the primary and secondary controller cards.
func findRoles(t testing.TB, dut *ondatra.DUTDevice, controllers []string) (primary, secondary string) {
	t.Helper()
	batch := gnmi.OCBatch()
	for _, c := range controllers {
		batch.AddPaths(gnmi.OC().Component(c).RedundantRole())
	}
	root := gnmi.Get(t, dut, batch.State())
	for _, c := range controllers {
		switch root.GetComponent(c).GetRedundantRole() {
		case oc.Platform_ComponentRedundantRole_PRIMARY:
			primary = c
		case oc.Platform_ComponentRedundantRole_SECONDARY:
			secondary = c
		}
	}
	if primary == "" || secondary == "" {
		t.Fatalf("Expected exactly one PRIMARY and one SECONDARY controller card, got primary=%q secondary=%q", primary, secondary)
	}
	return primary, secondary
}

// awaitSwitchoverReady waits up to switchover_timeout for a controller card to
// report switchover-ready = true, tolerating gNMI reconnections.
func awaitSwitchoverReady(t *testing.T, s *testState, controller string) {
	t.Helper()
	timeout := *switchoverTimeout + s.extraWait
	start := time.Now()
	retryUntil(t, timeout, "switchover-ready on "+controller, func(tb testing.TB) {
		remaining := min(max(timeout-time.Since(start), 30*time.Second), awaitAttemptTimeout)
		gnmi.Await(tb, s.dut, gnmi.OC().Component(controller).SwitchoverReady().State(), remaining, true)
	})
}

// issueSwitchControlProcessor issues gNOI SwitchControlProcessor to target. The
// DUT may drop the gNOI connection before it replies (UNAVAILABLE, CANCELED,
// DEADLINE_EXCEEDED or UNKNOWN); the switchover is then validated from the
// controller card roles. Any other error is fatal.
func issueSwitchControlProcessor(t *testing.T, s *testState, target string) {
	t.Helper()
	t.Logf("Issuing gNOI SwitchControlProcessor to %s", target)
	ctx, cancel := context.WithTimeout(context.Background(), switchoverRPCTimeout)
	defer cancel()
	resp, err := s.dut.RawAPIs().GNOI(t).System().SwitchControlProcessor(ctx, &spb.SwitchControlProcessorRequest{
		ControlProcessor: cmp.GetSubcomponentPath(target, deviations.GNOISubcomponentPath(s.dut)),
	})
	switch status.Code(err) {
	case codes.OK:
		t.Logf("SwitchControlProcessor response: %v", resp)
	case codes.Unavailable, codes.Canceled, codes.DeadlineExceeded, codes.Unknown:
		t.Logf("SwitchControlProcessor connection dropped (%v); validating the switchover from the controller card roles", err)
	default:
		t.Fatalf("SwitchControlProcessor to %s failed: %v", target, err)
	}
}

// awaitControllerRoles waits up to switchover_timeout for target to report
// PRIMARY and previous to report SECONDARY, re-establishing gNMI as needed.
func awaitControllerRoles(t *testing.T, s *testState, target, previous string) {
	t.Helper()
	timeout := *switchoverTimeout + s.extraWait
	start := time.Now()
	retryUntil(t, timeout, "new controller card roles", func(tb testing.TB) {
		remaining := min(max(timeout-time.Since(start), 30*time.Second), awaitAttemptTimeout)
		gnmi.Await(tb, s.dut, gnmi.OC().Component(target).RedundantRole().State(), remaining, oc.Platform_ComponentRedundantRole_PRIMARY)
		gnmi.Await(tb, s.dut, gnmi.OC().Component(previous).RedundantRole().State(), remaining, oc.Platform_ComponentRedundantRole_SECONDARY)
	})
}

// triggerSwitchover issues gNOI SwitchControlProcessor to target and validates
// the new roles, last-switchover-time and last-switchover-reason/trigger.
func triggerSwitchover(t *testing.T, s *testState, target, previous string) {
	t.Helper()
	comp := gnmi.OC().Component(target)
	prevTime, hadPrev := gnmi.Lookup(t, s.dut, comp.LastSwitchoverTime().State()).Val()
	requested := time.Now()
	issueSwitchControlProcessor(t, s, target)
	awaitControllerRoles(t, s, target, previous)

	// last-switchover-time must be later than the switchover request. A clock
	// skew tolerance between the test host and the DUT is allowed, and the value
	// must also be later than the value reported before the switchover. The
	// DUT may update it shortly after it reports the new roles, so wait for it.
	infoTimeout := switchoverInfoTimeout + s.extraWait
	earliest := uint64(requested.Add(-clockSkewTolerance).UnixNano())
	if _, ok := gnmi.Watch(t, s.dut, comp.LastSwitchoverTime().State(), infoTimeout, func(v *ygnmi.Value[uint64]) bool {
		got, present := v.Val()
		return present && got >= earliest && (!hadPrev || got > prevTime)
	}).Await(t); !ok {
		got, present := gnmi.Lookup(t, s.dut, comp.LastSwitchoverTime().State()).Val()
		switch {
		case !present:
			t.Errorf("%s last-switchover-time is not reported within %v", target, infoTimeout)
		case hadPrev && got <= prevTime:
			t.Errorf("%s last-switchover-time = %d, want later than the previous value %d", target, got, prevTime)
		default:
			t.Errorf("%s last-switchover-time = %d, want later than the switchover request time %d (clock skew tolerance %v)", target, got, requested.UnixNano(), clockSkewTolerance)
		}
	}
	wantTrigger := oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_USER_INITIATED
	if deviations.GNOISwitchoverReasonMissingUserInitiated(s.dut) {
		wantTrigger = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_SYSTEM_INITIATED
	}
	triggerPath := comp.LastSwitchoverReason().Trigger().State()
	if _, ok := gnmi.Watch(t, s.dut, triggerPath, infoTimeout, func(v *ygnmi.Value[oc.E_PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger]) bool {
		got, present := v.Val()
		return present && got == wantTrigger
	}).Await(t); !ok {
		got, _ := gnmi.Lookup(t, s.dut, triggerPath).Val()
		t.Errorf("%s last-switchover-reason/trigger = %v within %v, want %v", target, got, infoTimeout, wantTrigger)
	}
}

// ---------------------------------------------------------------------------
// Stale election ID checks (README P4RT-1.4.2 Step 6).
// ---------------------------------------------------------------------------

// staleP4RTCheck verifies that a client using the pre-switchover primary
// election ID cannot become primary, write entries or receive PacketIns.
func staleP4RTCheck(t *testing.T, s *testState) {
	t.Helper()
	stale := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := stale.P4rtClientSet(s.dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize stale P4RT client: %v", err)
	}
	for _, n := range s.nodes {
		name := streamName("stale", n.deviceID)
		defer stale.StreamChannelDestroy(&name)
		err := p4rtutils.StreamArbitrate(stale, &p4rt_client.P4RTStreamParameters{Name: name, DeviceId: n.deviceID, ElectionIdL: p4rtStaleEID}, time.Minute)
		if err := p4rtutils.CheckRPCErrorCode(err, codes.AlreadyExists); err != nil {
			t.Errorf("Stale P4RT client on node %s: %v", n.name, err)
		}
		lldpDel := p4rtutils.ACLWbbIngressTableEntryGet(puntEntries(p4pb.Update_DELETE)[:1])
		werr := stale.Write(&p4pb.WriteRequest{DeviceId: n.deviceID, ElectionId: &p4pb.Uint128{Low: p4rtStaleEID}, Updates: lldpDel, Atomicity: p4pb.WriteRequest_CONTINUE_ON_ERROR})
		if err := p4rtutils.CheckRPCErrorCode(werr, codes.PermissionDenied); err != nil {
			t.Errorf("Stale P4RT Write on node %s: %v", n.name, err)
		}
	}
	verifyPuntEntries(t, s, true)
	packetInOutValidation(t, s, []string{"port1"}, stale)
}

// staleGRIBICheck verifies that a gRIBI client using gRIBI-A's election ID is
// rejected because it is not the primary client and cannot change state. The
// rejection is accepted either as a FAILED_PRECONDITION RPC error or as a
// FAILED result for the operation, which is how the gRIBI reference
// implementation rejects AFT operations from a non-primary client.
func staleGRIBICheck(t *testing.T, s *testState) {
	t.Helper()
	if !eidGreater(s.gribiMaxEID, s.gribiAEID) {
		t.Fatalf("Current gRIBI leader election ID %+v is not higher than gRIBI-A's %+v; the stale check is not meaningful", s.gribiMaxEID, s.gribiAEID)
	}
	c := fluent.NewClient()
	c.Connection().WithStub(s.dut.RawAPIs().GRIBI(t)).
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithPersistence().WithFIBACK().
		WithInitialElectionID(s.gribiAEID.Low, s.gribiAEID.High)
	ctx := context.Background()
	c.Start(ctx, t)
	defer c.Stop(t)
	c.StartSending(ctx, t)
	c.Modify().AddEntry(t, fluent.IPv4Entry().WithPrefix(stalePrefixV4).WithNetworkInstance(s.defaultNI).WithNextHopGroup(nhgIDV4))
	subctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	err := c.Await(subctx, t)
	rejected := false
	if err != nil {
		var ce *client.ClientErr
		if errors.As(err, &ce) {
			for _, e := range ce.Recv {
				if status.Code(e) == codes.FailedPrecondition {
					rejected = true
				}
			}
		}
	}
	for _, r := range c.Results(t) {
		if r.Details == nil || r.Details.IPv4Prefix != stalePrefixV4 {
			continue
		}
		switch r.ProgrammingResult {
		case gpb.AFTResult_FAILED:
			rejected = true
		case gpb.AFTResult_RIB_PROGRAMMED, gpb.AFTResult_FIB_PROGRAMMED:
			t.Errorf("Stale gRIBI client programmed %s with result %v, want rejection", stalePrefixV4, r.ProgrammingResult)
		}
	}
	if !rejected {
		t.Errorf("Stale gRIBI client Modify for %s was not rejected within 1m (error: %v); want FAILED_PRECONDITION or a FAILED operation result", stalePrefixV4, err)
	}
	if gnmi.Lookup(t, s.dut, gnmi.OC().NetworkInstance(s.defaultNI).Afts().Ipv4Entry(stalePrefixV4).State()).IsPresent() {
		t.Errorf("AFT contains %s programmed by the stale gRIBI client", stalePrefixV4)
	}
	awaitAFT(t, s, aftAwaitTimeout)
	gribiGetValidation(t, s)
}

// ---------------------------------------------------------------------------
// Mandatory cleanup (registered before any configuration change).
// ---------------------------------------------------------------------------

// registerCleanup registers the README mandatory cleanup. t.Cleanup functions
// run in reverse registration order, so they are registered from last to first.
func registerCleanup(t *testing.T, s *testState) {
	t.Helper()
	dut := s.dut
	d := gnmi.OC()
	var ports []*ondatra.Port
	for _, pid := range []string{"port1", "port2", "port3", "port4"} {
		ports = append(ports, dut.Port(t, pid))
	}
	origIntf := map[string]*ygnmi.Value[*oc.Interface]{}
	// origUp records which DUT ports were operationally UP before the test, so
	// that the cleanup only waits for those ports to come back UP.
	origUp := map[string]bool{}
	for _, p := range ports {
		origIntf[p.Name()] = gnmi.Lookup(t, dut, d.Interface(p.Name()).Config())
		v, ok := gnmi.Lookup(t, dut, d.Interface(p.Name()).OperStatus().State()).Val()
		origUp[p.Name()] = ok && v == oc.Interface_OperStatus_UP
	}
	origLLDP := gnmi.Lookup(t, dut, d.Lldp().Enabled().Config())

	// 5. Restore the original primary controller card (SUP-A) if a failure left
	// the other controller card primary (for example a failure between the two
	// switchovers), so that the DUT ends in the same state as before the test.
	// It runs after step 4 and only switches over when the roles differ.
	t.Cleanup(func() {
		if s.supA == "" || s.supB == "" {
			return
		}
		var primary string
		if errMsg := testt.CaptureFatal(t, func(tb testing.TB) {
			primary, _ = findRoles(tb, dut, []string{s.supA, s.supB})
		}); errMsg != nil {
			t.Errorf("Cleanup: could not read the controller card roles to restore the original primary %s: %s", s.supA, *errMsg)
			return
		}
		if primary == s.supA {
			return
		}
		t.Logf("Cleanup: %s is primary; switching back to the original primary controller card %s", primary, s.supA)
		issueSwitchControlProcessor(t, s, s.supA)
		awaitControllerRoles(t, s, s.supA, s.supB)
		awaitSwitchoverReady(t, s, s.supB)
	})

	// 4. Wait for the standby controller card to report switchover-ready.
	t.Cleanup(func() {
		t.Log("Cleanup: waiting for the standby controller card to be switchover-ready")
		controllers := cmp.FindComponentsByType(t, dut, controllerCardType)
		_, standby := findRoles(t, dut, controllers)
		awaitSwitchoverReady(t, s, standby)
	})

	// 3. Restore the DUT configuration.
	t.Cleanup(func() {
		if !s.dutConfigured {
			return
		}
		t.Log("Cleanup: restoring the DUT configuration")
		// Each step reports its failure and the cleanup continues, so that one
		// rejected restore does not leave the remaining configuration in place.
		step := func(desc string, fn func(tb testing.TB)) {
			if errMsg := testt.CaptureFatal(t, fn); errMsg != nil {
				t.Errorf("Cleanup: %s failed: %s", desc, *errMsg)
			}
		}
		if s.vrf1Created {
			step("deleting VRF-1", func(tb testing.TB) { gnmi.Delete(tb, dut, d.NetworkInstance(vrf1).Config()) })
		}
		for _, pid := range []string{"port2", "port3"} {
			name := dut.Port(t, pid).Name()
			step("removing "+name+" from "+s.aggID, func(tb testing.TB) {
				gnmi.Delete(tb, dut, d.Interface(name).Ethernet().AggregateId().Config())
			})
		}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			for _, name := range []string{dut.Port(t, "port1").Name(), s.aggID, dut.Port(t, "port4").Name()} {
				step("removing "+name+" from the default network instance", func(tb testing.TB) {
					gnmi.Delete(tb, dut, d.NetworkInstance(s.defaultNI).Interface(fmt.Sprintf("%s.0", name)).Config())
				})
			}
		}
		// The LACP configuration references lag1, so it is deleted first.
		step("deleting the LACP configuration of "+s.aggID, func(tb testing.TB) { gnmi.Delete(tb, dut, d.Lacp().Interface(s.aggID).Config()) })
		step("deleting "+s.aggID, func(tb testing.TB) { gnmi.Delete(tb, dut, d.Interface(s.aggID).Config()) })
		for name, orig := range origIntf {
			step("restoring "+name, func(tb testing.TB) {
				if v, ok := orig.Val(); ok {
					gnmi.Replace(tb, dut, d.Interface(name).Config(), restorableInterface(v))
				} else {
					gnmi.Delete(tb, dut, d.Interface(name).Config())
				}
			})
		}
		for _, n := range s.nodes {
			path := d.Component(n.name).IntegratedCircuit().NodeId().Config()
			step("restoring the node-id of "+n.name, func(tb testing.TB) {
				if orig := s.origNodeID[n.name]; orig != nil && orig.IsPresent() {
					v, _ := orig.Val()
					gnmi.Replace(tb, dut, path, v)
				} else {
					gnmi.Delete(tb, dut, path)
				}
			})
		}
		step("restoring /lldp/config/enabled", func(tb testing.TB) {
			if v, ok := origLLDP.Val(); ok {
				gnmi.Replace(tb, dut, d.Lldp().Enabled().Config(), v)
			} else {
				gnmi.Delete(tb, dut, d.Lldp().Enabled().Config())
			}
		})
		for _, p := range ports {
			if !origUp[p.Name()] {
				t.Logf("Cleanup: DUT %s was not oper-status UP before the test; not waiting for it to come UP", p.Name())
				continue
			}
			step("waiting for "+p.Name()+" to be UP", func(tb testing.TB) {
				gnmi.Await(tb, dut, d.Interface(p.Name()).OperStatus().State(), 5*time.Minute+s.extraWait, oc.Interface_OperStatus_UP)
			})
		}
	})

	// 2. Flush gRIBI entries in DEFAULT and VRF-1 and close gRIBI sessions.
	t.Cleanup(func() {
		if !s.gribiStarted {
			return
		}
		t.Log("Cleanup: flushing gRIBI entries in the default network instance and VRF-1")
		if s.gribiLeader != nil {
			s.gribiLeader.Close(t)
		}
		c := startGRIBIClient(t, s, s.reconnectTimeout(*gribiReconnectTimeout))
		defer c.Close(t)
		// VRF-1 routes reference NHG 1 in the default network instance, so VRF-1
		// is flushed first.
		nis := []string{s.defaultNI}
		if s.vrf1Created {
			c.Flush(t, c.ElectionID(), vrf1)
			nis = append(nis, vrf1)
		}
		c.Flush(t, c.ElectionID(), s.defaultNI)
		retryUntil(t, aftAwaitTimeout, "AFT cleanup", func(tb testing.TB) {
			for _, ni := range nis {
				for p := range aftIPv4Entries(tb, dut, ni) {
					tb.Fatalf("gRIBI IPv4 prefix %s still present in %s", p, ni)
				}
				for p := range aftIPv6Entries(tb, dut, ni) {
					tb.Fatalf("gRIBI IPv6 prefix %s still present in %s", p, ni)
				}
			}
		})
	})

	// 1. Delete the P4RT punt entries and close the P4RT sessions. If there is
	// no P4RT session (for example a reconnect failed after a switchover) or the
	// current session cannot write, the clients are reconnected with higher
	// election IDs first. NOT_FOUND and FAILED_PRECONDITION (no pipeline) are
	// accepted.
	t.Cleanup(func() {
		if !s.puntWritten {
			return
		}
		t.Log("Cleanup: deleting P4RT punt entries and closing P4RT sessions")
		del := func(tb testing.TB) {
			writePuntEntries(tb, s, p4pb.Update_DELETE, codes.NotFound, codes.FailedPrecondition)
		}
		reconnect := func(reason string) {
			t.Logf("Cleanup: %s; reconnecting the P4RT clients", reason)
			connectP4RT(t, s, p4rtCleanupPrimaryEID, p4rtCleanupSecondaryEID, s.reconnectTimeout(*p4rtReconnectTimeout))
		}
		if s.p4 == nil || s.p4.primary == nil {
			reconnect("no P4RT session")
			del(t)
		} else if errMsg := testt.CaptureFatal(t, del); errMsg != nil {
			reconnect(fmt.Sprintf("P4RT DELETE failed (%s)", *errMsg))
			del(t)
		}
		verifyPuntEntries(t, s, false)
		destroyP4RTStreams(s)
	})
}

// restorableInterface prepares an interface configuration read back from the
// DUT for a gNMI Replace. Some DUTs report containers whose YANG "when"
// condition does not hold for the interface type (aggregation on a non-LAG
// interface, routed-vlan on a non-VLAN interface) and placeholder values (MTU 0,
// SPEED_UNKNOWN) that they reject when written back, so these are removed.
func restorableInterface(v *oc.Interface) *oc.Interface {
	if v.GetType() != oc.IETFInterfaces_InterfaceType_ieee8023adLag {
		v.Aggregation = nil
	}
	if v.GetType() != oc.IETFInterfaces_InterfaceType_l3ipvlan {
		v.RoutedVlan = nil
	}
	if v.Mtu != nil && *v.Mtu == 0 {
		v.Mtu = nil
	}
	if e := v.GetEthernet(); e != nil && e.GetPortSpeed() == oc.IfEthernet_ETHERNET_SPEED_SPEED_UNKNOWN {
		e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_UNSET
	}
	return v
}

// contains reports whether list contains v.
func contains(list []string, v string) bool {
	for _, e := range list {
		if e == v {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Test.
// ---------------------------------------------------------------------------

func TestP4RTSupervisorSwitchover(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// The canary routes are the first canaryCount scale routes, and the scale
	// routes must stay inside 198.18.0.0/16 so that they do not overlap with the
	// VRF-1 routes in 198.19.0.0/16.
	if *gribiScale < canaryCount || *gribiScale > maxGRIBIScale {
		t.Fatalf("gribi_scale = %d, want %d to %d", *gribiScale, canaryCount, maxGRIBIScale)
	}
	controllers := cmp.FindComponentsByType(t, dut, controllerCardType)
	if len(controllers) != 2 {
		t.Skipf("Dual controller cards required on %v: got %d, want 2", dut.Model(), len(controllers))
	}
	s := &testState{
		dut:       dut,
		ate:       ate,
		defaultNI: deviations.DefaultNetworkInstance(dut),
		aggID:     netutil.NextAggregateInterface(t, dut),
		extraWait: time.Duration(deviations.SwitchoverStabilizeDelayM(dut)) * time.Minute,
	}

	// Mandatory cleanup registration: before any configuration is applied.
	registerCleanup(t, s)

	// Test environment setup: DUT, node IDs, ATE, link and LAG check, roles.
	t.Log("Setup: configuring DUT interfaces, lag1, interface IDs, VRF-1 and disabling on-box LLDP")
	configureDUT(t, s)
	t.Log("Setup: configuring P4RT node IDs on the integrated circuits hosting the bound ports")
	configureNodeIDs(t, s)
	t.Log("Setup: configuring ATE ports, ATE LAG, control packet flows, data plane flows and capture")
	configureATE(t, s)
	t.Cleanup(func() { ate.OTG().StopProtocols(t) })
	verifyInterfaceIDs(t, s)
	t.Log("Setup: link and LAG check")
	linkAndLAGCheck(t, s)
	s.supA, s.supB = findRoles(t, dut, controllers)
	t.Logf("Setup: SUP-A (primary) = %s, SUP-B (secondary) = %s", s.supA, s.supB)

	// gRIBI programming.
	t.Log("Setup: connecting gRIBI-A (PRESERVE, SINGLE_PRIMARY, FIB_ACK) and programming entries")
	s.gribiStarted = true
	s.gribiLeader = startGRIBIClient(t, s, s.reconnectTimeout(*gribiReconnectTimeout))
	s.gribiAEID = s.gribiLeader.ElectionID()
	programInitialGRIBI(t, s)
	awaitAFT(t, s, aftAwaitTimeout)

	// P4RT programming.
	t.Log("Setup: connecting P4RT primary and secondary clients, pushing the pipeline")
	connectP4RT(t, s, p4rtPrimaryEID0, p4rtSecondaryEID0, s.reconnectTimeout(*p4rtReconnectTimeout))
	pushPipeline(t, s)
	t.Log("Setup: no-punt check before installing punt entries")
	noPuntCheck(t, s)
	t.Log("Setup: installing the 4 punt entries")
	s.puntWritten = true
	writePuntEntries(t, s, p4pb.Update_INSERT)
	verifyPuntEntries(t, s, true)

	// Baseline.
	t.Log("Baseline: PacketIn and PacketOut validation on DUT port-1, port-2 and port-3")
	packetInOutValidation(t, s, []string{"port1", "port2", "port3"})
	t.Log("Baseline: gRIBI Get validation from gRIBI-A")
	gribiGetValidation(t, s)

	if !t.Run("P4RT-1.4.1 Switchover 1 (SUP-A to SUP-B) with all ports and lag1 up", func(t *testing.T) {
		// Step 1 - Trigger the switchover under continuous traffic.
		t.Log("P4RT-1.4.1 Step 1: waiting for SUP-B switchover-ready and starting continuous traffic")
		awaitSwitchoverReady(t, s, s.supB)
		startDataPlane(t, s)
		t.Cleanup(func() { stopDataPlane(t, s) })
		triggerSwitchover(t, s, s.supB, s.supA)

		// Step 2 - Check the P4RT pipeline before re-pushing.
		t.Log("P4RT-1.4.1 Step 2: reconnecting P4RT clients and checking the pipeline before re-push")
		start := time.Now()
		connectP4RT(t, s, p4rtPrimaryEID1, p4rtSecondaryEID1, s.reconnectTimeout(*p4rtReconnectTimeout))
		t.Logf("P4RT clients reconnected %v after the switchover completed", time.Since(start).Round(time.Second))
		checkPipelineBeforeRepush(t, s)

		// Step 3 - Re-push the pipeline and punt entries and validate punting.
		t.Log("P4RT-1.4.1 Step 3: re-pushing the pipeline and punt entries")
		pushPipelineAndEntries(t, s)
		verifyPuntEntries(t, s, true)
		linkAndLAGCheck(t, s)
		verifyInterfaceIDs(t, s)
		packetInOutValidation(t, s, []string{"port1", "port2", "port3"})

		// Step 4 - Reconnect the gRIBI leader, change canary routes and validate Get.
		t.Log("P4RT-1.4.1 Step 4: connecting gRIBI-B, changing canary routes and validating gRIBI Get")
		s.gribiLeader.Close(t)
		s.gribiLeader = startGRIBIClient(t, s, s.reconnectTimeout(*gribiReconnectTimeout))
		changeCanaryRoutes(t, s)
		awaitAFT(t, s, aftAwaitTimeout)
		gribiGetValidation(t, s)

		// Step 5 - Validate the data plane.
		t.Log("P4RT-1.4.1 Step 5: stopping traffic and verifying 0 packets lost")
		stopAndVerifyDataPlane(t, s)
	}) {
		t.Fatal("P4RT-1.4.1 failed; P4RT-1.4.2 requires SUP-B to be primary after switchover 1")
	}

	t.Run("P4RT-1.4.2 Switchover 2 (SUP-B to SUP-A) with port-3 bring-up", func(t *testing.T) {
		port3 := dut.Port(t, "port3").Name()

		// Step 1 - Prepare and trigger the reverse switchover.
		t.Log("P4RT-1.4.2 Step 1: disabling DUT port-3 and triggering the reverse switchover")
		gnmi.Replace(t, dut, gnmi.OC().Interface(port3).Enabled().Config(), false)
		gnmi.Await(t, dut, gnmi.OC().Interface(port3).OperStatus().State(), 5*time.Minute, oc.Interface_OperStatus_DOWN)
		gnmi.Await(t, dut, gnmi.OC().Interface(s.aggID).OperStatus().State(), 5*time.Minute, oc.Interface_OperStatus_UP)
		awaitLAGMember(t, s, dut.Port(t, "port2").Name(), 5*time.Minute)
		awaitSwitchoverReady(t, s, s.supA)
		startDataPlane(t, s)
		t.Cleanup(func() { stopDataPlane(t, s) })
		triggerSwitchover(t, s, s.supA, s.supB)

		// Step 2 - Bring up port-3 during the switchover window, before P4RT reconnects.
		t.Log("P4RT-1.4.2 Step 2: re-enabling DUT port-3 before the P4RT clients reconnect")
		retryUntil(t, s.reconnectTimeout(*gnmiReconnectTimeout), "gNMI Set enabled=true on DUT port-3", func(tb testing.TB) {
			gnmi.Replace(tb, dut, gnmi.OC().Interface(port3).Enabled().Config(), true)
		})
		upBeforeReconnect := false
		if v, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(port3).OperStatus().State()).Val(); ok && v == oc.Interface_OperStatus_UP {
			upBeforeReconnect = true
		}

		// Step 3 - Reconnect P4RT and validate punting on all ports.
		t.Log("P4RT-1.4.2 Step 3: reconnecting P4RT, re-pushing and validating punting on port-1, port-2 and port-3")
		connectP4RT(t, s, p4rtPrimaryEID2, p4rtSecondaryEID2, s.reconnectTimeout(*p4rtReconnectTimeout))
		if upBeforeReconnect {
			t.Log("DUT port-3 reported oper-status UP before the P4RT clients reconnected")
		} else {
			gnmi.Await(t, dut, gnmi.OC().Interface(port3).OperStatus().State(), 5*time.Minute+s.extraWait, oc.Interface_OperStatus_UP)
			t.Log("DUT port-3 reported oper-status UP after the P4RT clients reconnected")
		}
		pushPipelineAndEntries(t, s)
		verifyPuntEntries(t, s, true)
		linkAndLAGCheck(t, s)
		packetInOutValidation(t, s, []string{"port1", "port2", "port3"})

		// Step 4 - Reconnect gRIBI and validate Get.
		t.Log("P4RT-1.4.2 Step 4: connecting gRIBI-C and validating gRIBI Get")
		s.gribiLeader.Close(t)
		s.gribiLeader = startGRIBIClient(t, s, s.reconnectTimeout(*gribiReconnectTimeout))
		awaitAFT(t, s, aftAwaitTimeout)
		gribiGetValidation(t, s)

		// Step 5 - Validate the data plane.
		t.Log("P4RT-1.4.2 Step 5: stopping traffic and verifying 0 packets lost")
		stopAndVerifyDataPlane(t, s)

		// Step 6 - Stale election ID rejection (no additional switchover).
		t.Log("P4RT-1.4.2 Step 6: stale election ID rejection for P4RT and gRIBI")
		staleP4RTCheck(t, s)
		staleGRIBICheck(t, s)
	})
}
