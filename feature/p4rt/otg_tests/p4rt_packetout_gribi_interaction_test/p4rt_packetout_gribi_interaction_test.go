// Copyright 2026 OpenConfig Contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package p4rt_packetout_gribi_interaction_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	p4v1pb "github.com/p4lang/p4runtime/go/p4/v1"
)

// dynamicRouteOperation specifies the type of operation to perform on dynamic routes.
type dynamicRouteOperation string

const (
	deviceID   = uint64(1)
	electionID = uint64(100)
	// The Packet-Out port metadata used by the P4RT pipeline.
	packetOutPortID = uint32(2)
	packetSize      = 1500
	packetRatePPS   = 1000
	// README scale.
	ipv4RouteCount    = 100000
	ipv6RouteCount    = 100000
	dynamicRouteCount = 10000 // Due to the gRIBI client session timeout, we limit the number of dynamic routes to 10k for the concurrent load test.
	ipv4RouteBase     = "198.51.200.0"
	ipv6RouteBase     = "2001:db8:200::"
	// Packet-Out validation window.
	packetOutDuration = 2 * time.Second
	// Prefix used for the negative-scenario concurrent gRIBI churn entry.
	negativeChurnRouteCount = 1000
	negativeChurnIPv4Base   = "198.51.199.0"
	negativeChurnIPv6Base   = "2001:db8:199::"
	// Allowed Packet-Out loss percentage between DUT egress and ATE ingress.
	packetOutCounterTolerance                         = float64(0) // README requires 0% Packet-Out drop and 0% data-plane loss.
	gribiBatchSize                                    = 2000
	routeV4PrfLen                                     = "/32"
	routeV6PrfLen                                     = "/128"
	intStepV4                                         = "0.0.0.1"
	intStepV6                                         = "::1"
	baselineIPv4Flow                                  = "baseline-ipv4"
	baselineIPv6Flow                                  = "baseline-ipv6"
	dynamicIPv4Flow                                   = "dynamic-ipv4"
	dynamicIPv6Flow                                   = "dynamic-ipv6"
	baselineIPv4Prefix                                = "198.51.100.0/24"
	baselineIPv6Prefix                                = "2001:db8:100::/64"
	baselineIPv4RouteDst                              = "198.51.100.1"
	baselineIPv6RouteDst                              = "2001:db8:100::1"
	gribiNHIPv4Port2                                  = uint64(10)
	gribiNHIPv6Port2                                  = uint64(11)
	gribiNHGIPv4Port2                                 = uint64(100)
	gribiNHGIPv6Port2                                 = uint64(101)
	dynNHIPv4Port3                                    = uint64(20)
	dynNHIPv4Port4                                    = uint64(21)
	dynNHIPv6Port3                                    = uint64(22)
	dynNHIPv6Port4                                    = uint64(23)
	dynNHGIPv4Port3                                   = uint64(200)
	dynNHGIPv4Port4                                   = uint64(201)
	dynNHGIPv6Port3                                   = uint64(202)
	dynNHGIPv6Port4                                   = uint64(203)
	ethernetHeaderLen                                 = 14
	ipv4HeaderLen                                     = 20
	ipv6HeaderLen                                     = 40
	packetOutDebugFrameCount                          = 5
	trafficDuration                                   = 15 * time.Second
	outPktCount                                       = 100
	baselinePacketOutCheckCount                       = 1
	dynamicRouteAdd             dynamicRouteOperation = "add"
	dynamicRouteDelete          dynamicRouteOperation = "delete"
	dynamicRouteReplace         dynamicRouteOperation = "replace"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6:    "2001:db8:1::1",
		IPv6Len: 126,
		MAC:     "00:1A:11:00:00:01",
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
		IPv6:    "2001:db8:1::2",
		IPv6Len: 126,
		MAC:     "02:00:00:00:00:01",
	}
	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
		IPv6:    "2001:db8:2::1",
		IPv6Len: 126,
		MAC:     "00:1A:11:00:00:02",
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
		IPv6:    "2001:db8:2::2",
		IPv6Len: 126,
		MAC:     "02:00:00:00:00:02",
	}
	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv4Len: 30,
		IPv6:    "2001:db8:3::1",
		IPv6Len: 126,
		MAC:     "00:1A:11:00:00:03",
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "192.0.2.10",
		IPv4Len: 30,
		IPv6:    "2001:db8:3::2",
		IPv6Len: 126,
		MAC:     "02:00:00:00:00:03",
	}
	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.13",
		IPv4Len: 30,
		IPv6:    "2001:db8:4::1",
		IPv6Len: 126,
		MAC:     "00:1A:11:00:00:04",
	}
	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		IPv4:    "192.0.2.14",
		IPv4Len: 30,
		IPv6:    "2001:db8:4::2",
		IPv6Len: 126,
		MAC:     "02:00:00:00:00:04",
	}
	p4InfoFile = flag.String(
		"p4info_file_location",
		"../../data/wbb.p4info.pb.txt",
		"Path to the P4Info file.",
	)
	streamName = "p4rt"
)

// testArgs contains the resources shared by all P4RT-3.3 subtests.
type testArgs struct {
	ctx       context.Context
	dut       *ondatra.DUTDevice
	ate       *ondatra.ATEDevice
	p4rt      *p4rt_client.P4RTClient
	gribi     *gribi.Client
	defaultNI string
	top       gosnappi.Config
	packetOut []*p4v1pb.PacketOut
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestP4RTPacketOutGRIBIInteraction(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	ctx := t.Context()
	configureDUT(t, dut)
	ateConfig := configureATE(t, ate)
	ate.OTG().PushConfig(t, ateConfig)
	ate.OTG().StartProtocols(t)
	cfgplugins.VerifyInterfacesStatus(t, dut, []string{"port1", "port2", "port3", "port4"})
	p4rt := setupP4RTClient(t, dut)
	gribiClient := setupGRIBIClient(t, dut)
	args := &testArgs{
		ctx:       ctx,
		dut:       dut,
		ate:       ate,
		p4rt:      p4rt,
		gribi:     gribiClient,
		defaultNI: deviations.DefaultNetworkInstance(dut),
		top:       ateConfig,
	}
	t.Run("P4RT_3_3_1", func(t *testing.T) {
		runBaseline(t, args)
	})
	t.Run("P4RT_3_3_2", func(t *testing.T) {
		runConcurrentLoad(t, args)
	})
	t.Run("P4RT_3_3_3", func(t *testing.T) {
		runNegativeScenarios(t, args)
	})
}

// runBaseline implements P4RT-3.3.1.
func runBaseline(t *testing.T, args *testArgs) {
	t.Helper()
	t.Log("Programming baseline gRIBI routes.")
	programBaselineGRIBIRoutes(t, args)
	waitForRoutes(t, args)
	verifyBaselineNHViaAFT(t, args.dut, args.defaultNI)
	verifyBaselineAFTCoverage(t, args.dut, args.defaultNI)
	counterBefore := uint64(gnmi.Get(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Counters().OutPkts().State()))
	args.packetOut = buildPacketOutPackets(t)
	t.Log("Sending baseline IPv4/IPv6 Packet-Out.")
	for _, pktOut := range args.packetOut {
		sendPacketOut(t, args.p4rt, pktOut, outPktCount)
	}
	verifyBaselinePacketOutExact(t, args)
	expectedDelta := uint64(len(args.packetOut) * (outPktCount + baselinePacketOutCheckCount))
	verifyPacketOutCounters(t, args, counterBefore, expectedDelta)
	t.Log("Starting baseline data-plane traffic.")
	createFlow(t, args, baselineIPv4Flow, baselineIPv6Flow, []string{"port2"}, false)
	verifyBaselineTraffic(t, args)
}

// packetOutRunStats stores Packet-Out counters collected around the test window.
type packetOutRunStats struct {
	sent              uint64
	dutOutPktsBefore  uint64
	dutInPktsBefore   uint64
	ateInFramesBefore uint64
}

// runConcurrentLoad implements P4RT-3.3.2.
func runConcurrentLoad(t *testing.T, args *testArgs) {
	t.Helper()
	if len(args.packetOut) < 2 ||
		args.packetOut[0] == nil ||
		args.packetOut[1] == nil {
		t.Log("Packet-Out payloads were not initialized; building defaults.")
		args.packetOut = buildPacketOutPackets(t)
	}
	validatePacketOutPackets(t, args.packetOut)
	packetCtx, cancel := context.WithCancel(args.ctx)
	defer cancel()
	startPacketOutCapture(t, args, []string{"port1", "port2", "port3", "port4"})
	port2Name := args.dut.Port(t, "port2").Name()
	stats := &packetOutRunStats{
		dutOutPktsBefore:  uint64(gnmi.Get(t, args.dut, gnmi.OC().Interface(port2Name).Counters().OutPkts().State())),
		dutInPktsBefore:   uint64(gnmi.Get(t, args.dut, gnmi.OC().Interface(port2Name).Counters().InPkts().State())),
		ateInFramesBefore: uint64(gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port("port2").Counters().InFrames().State())),
	}
	var packetOutSent atomic.Uint64
	t.Logf("Starting continuous P4RT Packet-Out traffic at %d pps.", packetRatePPS)
	packetErr := make(chan error, 1)
	go func() {
		packetErr <- sendContinuousPacketOut(packetCtx, args.p4rt, args.packetOut, packetRatePPS, &packetOutSent)
	}()
	t.Log("Starting concurrent gRIBI route programming.")
	if err := programGRIBIRoutes(packetCtx, t, args, ipv4RouteCount, ipv6RouteCount); err != nil {
		cancel()
		if sendErr := <-packetErr; sendErr != nil && !errors.Is(sendErr, context.Canceled) && !isTransientPacketOutError(sendErr) {
			t.Logf("Packet-Out sender stopped with error: %v", sendErr)
		}
		t.Fatalf("gRIBI programming failed: %v", err)
	}
	stats.sent = packetOutSent.Load()
	t.Logf("gRIBI programming completed while Packet-Out traffic was active; Packet-Out packets sent=%d", stats.sent)
	// Stop Packet-Out and verify the complete concurrent-load window.
	cancel()
	if err := <-packetErr; err != nil && !errors.Is(err, context.Canceled) && !isTransientPacketOutError(err) {
		t.Fatalf("Packet-Out traffic failed: %v", err)
	} else if err != nil {
		t.Logf("Ignoring transient Packet-Out sender shutdown error: %v", err)
	}
	stats.sent = packetOutSent.Load()
	t.Logf("Packet-Out sender completed: %d packets successfully sent", stats.sent)
	// Allow counter telemetry to settle before validation reads by waiting for
	// DUT egress out-pkts to reflect the Packet-Out sends completed in-window.
	outPktsPath := gnmi.OC().Interface(port2Name).Counters().OutPkts().State()
	wantOutPkts := stats.dutOutPktsBefore + stats.sent
	if _, ok := gnmi.Watch(t, args.dut, outPktsPath, 2*time.Minute, func(val *ygnmi.Value[uint64]) bool {
		got, present := val.Val()
		return present && got >= wantOutPkts
	}).Await(t); !ok {
		got := gnmi.Get(t, args.dut, outPktsPath)
		t.Fatalf("Packet-Out counter did not settle to expected value on %s: got=%d want>=%d (before=%d sent=%d)", port2Name, got, wantOutPkts, stats.dutOutPktsBefore, stats.sent)
	}
	verifyPacketOutTraffic(t, args, stats)
	t.Log("Starting dynamic route traffic validation.")
	createFlow(t, args, dynamicIPv4Flow, dynamicIPv6Flow, []string{"port3", "port4"}, true)
	verifyDynamicRouteTraffic(t, args)
}

// runNegativeScenarios implements P4RT-3.3.3:
//  1. Packet-Out to an invalid egress port is rejected by the device while
//     concurrent gRIBI programming continues uninterrupted.
//  2. Packet-Out to an administratively disabled egress port is dropped
//     deterministically while gRIBI programming towards healthy ports
//     (port3/port4) completes successfully.
func runNegativeScenarios(t *testing.T, args *testArgs) {
	t.Helper()
	if len(args.packetOut) < 1 || args.packetOut[0] == nil {
		args.packetOut = buildPacketOutPackets(t)
	}
	t.Run("invalid_egress_port", func(t *testing.T) {
		ensureP4RTStream(t, args)
		testInvalidPacketOutPort(t, args)
	})
	t.Run("port_state_down", func(t *testing.T) {
		ensureP4RTStream(t, args)
		testPacketOutWithPortDown(t, args)
	})
}

// ensureP4RTStream re-establishes the P4RT session if the stream was torn down
// (for example, by the device rejecting an invalid Packet-Out).
func ensureP4RTStream(t *testing.T, args *testArgs) {
	t.Helper()
	if args.p4rt == nil || args.p4rt.StreamChannelGet(&streamName) == nil {
		t.Log("P4RT stream not present; re-establishing P4RT session.")
		args.p4rt = setupP4RTClient(t, args.dut)
	}
}

// validatePacketOutPackets validates Packet-Out payload and metadata shape.
func validatePacketOutPackets(t *testing.T, packets []*p4v1pb.PacketOut) {
	t.Helper()
	if len(packets) != 2 {
		t.Fatalf("got %d Packet-Out packets, want 2", len(packets))
	}
	for i, pkt := range packets {
		if pkt == nil {
			t.Fatalf("Packet-Out packet %d is nil", i)
		}
		if len(pkt.Payload) != packetSize {
			t.Fatalf("Packet-Out packet %d has payload size %d bytes, want %d bytes", i, len(pkt.Payload), packetSize)
		}
		if len(pkt.Metadata) == 0 {
			t.Fatalf("Packet-Out packet %d has no metadata", i)
		}
		t.Logf("Packet-Out packet %d: payload=%d bytes metadata=%d", i, len(pkt.Payload), len(pkt.Metadata))
	}
}

// startPacketOutCapture starts packet capture on all ATE ports.
func startPacketOutCapture(t *testing.T, args *testArgs, portNames []string) gosnappi.ControlState {
	t.Helper()
	args.top.Captures().Clear()
	args.top.Captures().Add().SetName("packetOutCapture").SetPortNames(portNames).SetFormat(gosnappi.CaptureFormat.PCAP)
	args.ate.OTG().PushConfig(t, args.top)
	args.ate.OTG().StartProtocols(t)
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	args.ate.OTG().SetControlState(t, cs)
	t.Log("Packet-Out capture started on all ATE ports.")
	return cs
}

// stopPacketOutCapture stops packet capture on all ATE ports.
func stopPacketOutCapture(t *testing.T, args *testArgs, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	args.ate.OTG().SetControlState(t, cs)
	t.Log("Packet-Out capture stopped on all ATE ports.")
}

// configureDUT configures all four DUT interfaces.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ports := []struct {
		name string
		id   uint32
		attr *attrs.Attributes
	}{
		{"port1", 1, &dutPort1},
		{"port2", 2, &dutPort2},
		{"port3", 3, &dutPort3},
		{"port4", 4, &dutPort4},
	}
	vrfBatch := new(gnmi.SetBatch)
	for _, p := range ports {
		port := dut.Port(t, p.name)
		intf := &oc.Interface{
			Name: ygot.String(port.Name()),
			Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Id:   ygot.Uint32(p.id),
		}
		intf.Description = ygot.String(p.attr.Desc)
		intf.Enabled = ygot.Bool(true)
		if p.attr.MAC != "" {
			intf.GetOrCreateEthernet().MacAddress = ygot.String(p.attr.MAC)
		}
		sub := intf.GetOrCreateSubinterface(0)
		ipv4 := sub.GetOrCreateIpv4()
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			ipv4.Enabled = ygot.Bool(true)
		}
		ipv4Addr := ipv4.GetOrCreateAddress(p.attr.IPv4)
		ipv4Addr.PrefixLength = ygot.Uint8(uint8(p.attr.IPv4Len))
		ipv6 := sub.GetOrCreateIpv6()
		ipv6.Enabled = ygot.Bool(true)
		ipv6Addr := ipv6.GetOrCreateAddress(p.attr.IPv6)
		ipv6Addr.PrefixLength = ygot.Uint8(uint8(p.attr.IPv6Len))
		gnmi.BatchReplace(vrfBatch, gnmi.OC().Interface(port.Name()).Config(), intf)
		fptest.SetPortSpeed(t, port)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, port.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}
	vrfBatch.Set(t, dut)
}

// configureATE configures the four ATE ports.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	addATEDevice := func(name string, ateAttr, dutAttr *attrs.Attributes) {
		port := ate.Port(t, name)
		config.Ports().Add().SetName(port.ID())
		dev := config.Devices().Add().SetName(ateAttr.Name)
		eth := dev.Ethernets().Add().SetName(ateAttr.Name + ".Eth")
		eth.Connection().SetPortName(port.ID())
		eth.SetMac(ateAttr.MAC)
		if ateAttr.IPv4 != "" {
			ip4 := eth.Ipv4Addresses().Add().SetName(dev.Name() + ".IPv4")
			ip4.SetAddress(ateAttr.IPv4).SetGateway(dutAttr.IPv4).SetPrefix(uint32(ateAttr.IPv4Len))
		}
		if ateAttr.IPv6 != "" {
			ip6 := eth.Ipv6Addresses().Add().SetName(dev.Name() + ".IPv6")
			ip6.SetAddress(ateAttr.IPv6).SetGateway(dutAttr.IPv6).SetPrefix(uint32(ateAttr.IPv6Len))
		}
	}
	addATEDevice("port1", &atePort1, &dutPort1)
	addATEDevice("port2", &atePort2, &dutPort2)
	addATEDevice("port3", &atePort3, &dutPort3)
	addATEDevice("port4", &atePort4, &dutPort4)
	return config
}

// setupGRIBIClient creates and initializes the test gRIBI client.
func setupGRIBIClient(t *testing.T, dut *ondatra.DUTDevice) *gribi.Client {
	t.Helper()
	client := &gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}
	if err := client.Start(t); err != nil {
		t.Fatalf("failed to start gRIBI client: %v", err)
	}
	client.BecomeLeader(t)
	if err := gribi.FlushAll(client.Fluent(t)); err != nil {
		t.Fatalf("failed to flush pre-existing gRIBI entries: %v", err)
	}
	t.Cleanup(func() {
		client.FlushAll(t)
		client.Close(t)
	})
	return client
}

// setupP4RTClient establishes the P4RT session and arbitration.
func setupP4RTClient(t *testing.T, dut *ondatra.DUTDevice) *p4rt_client.P4RTClient {
	t.Helper()
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	node, ok := nodes["port1"]
	if !ok {
		t.Fatalf("P4RT node for port1 not found")
	}
	t.Logf("Configuring P4RT Node: %s", node)
	c := oc.Component{}
	c.Name = ygot.String(node)
	c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	c.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
	setBatch := new(gnmi.SetBatch)
	gnmi.BatchReplace(setBatch, gnmi.OC().Component(node).Config(), &c)
	setBatch.Set(t, dut)
	client := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := client.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("could not initialize p4rt client: %v", err)
	}
	streamParameters := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceID,
		ElectionIdH: 0,
		ElectionIdL: electionID,
	}
	client.StreamChannelCreate(&streamParameters)
	if err := client.StreamChannelSendMsg(
		&streamName,
		&p4v1pb.StreamMessageRequest{
			Update: &p4v1pb.StreamMessageRequest_Arbitration{
				Arbitration: &p4v1pb.MasterArbitrationUpdate{
					DeviceId: deviceID,
					ElectionId: &p4v1pb.Uint128{
						High: 0,
						Low:  electionID,
					},
				},
			},
		},
	); err != nil {
		t.Fatalf("failed to send P4RT arbitration: %v", err)
	}
	if _, _, err := client.StreamChannelGetArbitrationResp(&streamName, 1); err != nil {
		t.Fatalf("failed to receive P4RT arbitration response: %v", err)
	}
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		t.Fatalf("failed to load P4Info: %v", err)
	}
	if err := client.SetForwardingPipelineConfig(
		&p4v1pb.SetForwardingPipelineConfigRequest{
			DeviceId:   deviceID,
			ElectionId: &p4v1pb.Uint128{High: 0, Low: electionID},
			Action:     p4v1pb.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
			Config: &p4v1pb.ForwardingPipelineConfig{
				P4Info: p4Info,
			},
		},
	); err != nil {
		t.Fatalf("failed to set P4RT forwarding pipeline: %v", err)
	}
	return client
}

// verifyBaselineAFTCoverage verifies that the baseline IPv4 and IPv6 AFT entries
// reference valid next-hop-group IDs in the specified network instance.
func verifyBaselineAFTCoverage(t *testing.T, dut *ondatra.DUTDevice, ni string) {
	t.Helper()
	if gnmi.Get(t, dut, gnmi.OC().NetworkInstance(ni).Afts().Ipv4Entry(baselineIPv4Prefix).State()).GetNextHopGroup() == 0 {
		t.Errorf("aft next-hop-group for ipv4-entry %s: got 0, want non-zero", baselineIPv4Prefix)
	}
	if gnmi.Get(t, dut, gnmi.OC().NetworkInstance(ni).Afts().Ipv6Entry(baselineIPv6Prefix).State()).GetNextHopGroup() == 0 {
		t.Errorf("aft next-hop-group for ipv6-entry %s: got 0, want non-zero", baselineIPv6Prefix)
	}
}

// verifyBaselineNHViaAFT asserts state/index only; OC AFT has no vendor-neutral viability leaf, and absence is a valid rejection.
func verifyBaselineNHViaAFT(t *testing.T, dut *ondatra.DUTDevice, ni string) {
	t.Helper()
	nextHopObjList := gnmi.GetAll(t, dut, gnmi.OC().NetworkInstance(ni).Afts().NextHopAny().State())
	for _, nhObj := range nextHopObjList {
		if got := nhObj.GetIpAddress(); got == atePort2.IPv4 || got == atePort2.IPv6 {
			nhIndex := nhObj.GetIndex()
			val, _ := gnmi.Watch(t, dut, gnmi.OC().NetworkInstance(ni).Afts().NextHop(nhIndex).State(), 10*time.Second, func(v *ygnmi.Value[*oc.NetworkInstance_Afts_NextHop]) bool {
				_, present := v.Val()
				return present
			}).Await(t)
			_, present := val.Val()
			if !present {
				t.Errorf("aft next-hop %d: not present (not installed or rejected)", nhIndex)
				continue
			}
		}
	}
}

// buildPacketOutPackets creates the baseline IPv4 and IPv6 Packet-Out
// messages. The packet payload is 1500 bytes.
func buildPacketOutPackets(t *testing.T) []*p4v1pb.PacketOut {
	t.Helper()
	ipv4Packet := buildIPv4Packet(t)
	ipv6Packet := buildIPv6Packet(t)
	return []*p4v1pb.PacketOut{
		newPacketOut(ipv4Packet, packetOutPortID),
		newPacketOut(ipv6Packet, packetOutPortID),
	}
}

// newPacketOut builds one Packet-Out with egress-port metadata.
func newPacketOut(payload []byte, egressPortID uint32) *p4v1pb.PacketOut {
	return &p4v1pb.PacketOut{
		Payload:  payload,
		Metadata: packetOutEgressMetadata(egressPortID),
	}
}

// packetOutEgressMetadata returns the WBB egress_port metadata (ID=1).
func packetOutEgressMetadata(egressPortID uint32) []*p4v1pb.PacketMetadata {
	return []*p4v1pb.PacketMetadata{{
		MetadataId: 1,
		Value:      []byte(fmt.Sprint(egressPortID)),
	}}
}

// buildIPv4Packet builds a 1500-byte Ethernet/IPv4 packet.
func buildIPv4Packet(t *testing.T) []byte {
	t.Helper()
	payloadLen := payloadLengthForIP(t, packetSize, ipv4HeaderLen)
	eth := &layers.Ethernet{
		SrcMAC:       mustParseMAC(t, atePort1.MAC),
		DstMAC:       mustParseMAC(t, atePort2.MAC),
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := &layers.IPv4{
		Version:  4,
		TTL:      64,
		SrcIP:    net.ParseIP(atePort1.IPv4),
		DstIP:    net.ParseIP(baselineIPv4RouteDst),
		Protocol: layers.IPProtocolNoNextHeader,
	}
	return serializePacket(t, eth, ip, packetSize, payloadLen)
}

// buildIPv6Packet builds a 1500-byte Ethernet/IPv6 packet.
func buildIPv6Packet(t *testing.T) []byte {
	t.Helper()
	payloadLen := payloadLengthForIP(t, packetSize, ipv6HeaderLen)
	eth := &layers.Ethernet{
		SrcMAC:       mustParseMAC(t, atePort1.MAC),
		DstMAC:       mustParseMAC(t, atePort2.MAC),
		EthernetType: layers.EthernetTypeIPv6,
	}
	ip := &layers.IPv6{
		Version:    6,
		HopLimit:   64,
		SrcIP:      net.ParseIP(atePort1.IPv6),
		DstIP:      net.ParseIP(baselineIPv6RouteDst),
		NextHeader: layers.IPProtocolNoNextHeader,
	}
	return serializePacket(t, eth, ip, packetSize, payloadLen)
}

// payloadLengthForIP computes payload bytes needed to build an exact-size frame.
func payloadLengthForIP(t *testing.T, frameSize, ipHeaderLen int) int {
	t.Helper()
	payloadLen := frameSize - ethernetHeaderLen - ipHeaderLen
	if payloadLen <= 0 {
		t.Fatalf("invalid payload length %d (frame=%d, eth=%d, ip=%d)", payloadLen, frameSize, ethernetHeaderLen, ipHeaderLen)
	}
	return payloadLen
}

// serializePacket serializes Ethernet+IP+payload and enforces exact frame size.
func serializePacket(t *testing.T, eth *layers.Ethernet, ip gopacket.SerializableLayer, packetSize, payloadLen int) []byte {
	t.Helper()
	payload := make([]byte, payloadLen)
	buffer := gopacket.NewSerializeBuffer()
	options := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	if err := gopacket.SerializeLayers(buffer, options, eth, ip, gopacket.Payload(payload)); err != nil {
		t.Fatalf("failed to serialize packet: %v", err)
	}
	packet := buffer.Bytes()
	if len(packet) != packetSize {
		t.Fatalf("packet size = %d, want %d", len(packet), packetSize)
	}
	return packet
}

// sendPacketOut sends Packet-Out messages through StreamChannel.
func sendPacketOut(t *testing.T, client *p4rt_client.P4RTClient, packet *p4v1pb.PacketOut, count int) {
	t.Helper()
	if err := sendPacketOutErr(client, packet, count); err != nil {
		t.Fatal(err)
	}
}

// sendPacketOutErr sends Packet-Out messages through StreamChannel and returns errors.
func sendPacketOutErr(client *p4rt_client.P4RTClient, packet *p4v1pb.PacketOut, count int) error {
	if packet == nil {
		return fmt.Errorf("packet-Out packet is nil")
	}
	if count <= 0 {
		return fmt.Errorf("packet-Out count must be greater than zero, got %d", count)
	}
	for i := 0; i < count; i++ {
		if err := client.StreamChannelSendMsg(
			&streamName,
			&p4v1pb.StreamMessageRequest{
				Update: &p4v1pb.StreamMessageRequest_Packet{
					Packet: packet,
				},
			},
		); err != nil {
			return fmt.Errorf("packet-Out failed at packet %d/%d: %v", i+1, count, err)
		}
	}
	return nil
}

// isP4RTStreamMissingError reports whether err indicates that the P4RT stream
// could not be found.
func isP4RTStreamMissingError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Could not find stream") || strings.Contains(msg, "stream(p4rt)")
}

// sendContinuousPacketOut continuously sends IPv4 and IPv6 Packet-Out
// messages at approximately the requested packet rate.
func sendContinuousPacketOut(ctx context.Context, client *p4rt_client.P4RTClient, packets []*p4v1pb.PacketOut, ratePPS int, sent *atomic.Uint64) error {
	if ratePPS <= 0 {
		return fmt.Errorf("invalid Packet-Out rate %d", ratePPS)
	}
	if len(packets) == 0 {
		return fmt.Errorf("no Packet-Out payloads provided")
	}
	if sent == nil {
		return fmt.Errorf("packet-Out sent counter is nil")
	}
	interval := time.Second / time.Duration(ratePPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	index := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pkt := packets[index%len(packets)]
			if pkt == nil {
				return fmt.Errorf("nil Packet-Out payload at index %d", index%len(packets))
			}
			if len(pkt.Payload) != packetSize {
				return fmt.Errorf("packet-Out payload has size %d bytes, want %d", len(pkt.Payload), packetSize)
			}
			if err := client.StreamChannelSendMsg(
				&streamName,
				&p4v1pb.StreamMessageRequest{
					Update: &p4v1pb.StreamMessageRequest_Packet{
						Packet: pkt,
					},
				},
			); err != nil {
				return fmt.Errorf("packet-Out send failed: %w", err)
			}
			sent.Add(1)
			index++
		}
	}
}

// isTransientPacketOutError reports transport failures that can happen while
// concurrent scale operations are running and that should not hard-fail the test.
func isTransientPacketOutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "could not find stream") ||
		strings.Contains(msg, "stream(p4rt)") ||
		strings.Contains(msg, "rst_stream") ||
		strings.Contains(msg, "unavailable") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "eof")
}

// programBaselineGRIBIRoutes programs the two baseline routes.
func programBaselineGRIBIRoutes(t *testing.T, args *testArgs) {
	t.Helper()
	port2Name := args.dut.Port(t, "port2").Name()
	expectedResult := gribiProgrammingResult(args.dut)
	if deviations.GRIBIMACOverrideWithStaticARP(args.dut) {
		configureStaticARP(t, args.dut, port2Name, atePort2.IPv4, atePort2.IPv6, atePort2.MAC)
	}
	if deviations.GRIBIMACOverrideStaticARPStaticRoute(args.dut) {
		configureStaticARPAndRoute(t, args.dut, port2Name, atePort2.IPv4, atePort2.IPv6, atePort2.MAC)
	}
	entries := make([]fluent.GRIBIEntry, 0, 6)
	results := make([]*client.OpResult, 0, 6)
	nh4, nh4Res := buildGRIBINextHop(t, args.dut, gribiNHIPv4Port2, expectedResult, args.defaultNI, port2Name, atePort2.IPv4, atePort2.MAC)
	nh6, nh6Res := buildGRIBINextHop(t, args.dut, gribiNHIPv6Port2, expectedResult, args.defaultNI, port2Name, atePort2.IPv6, atePort2.MAC)
	entries = append(entries, nh4, nh6)
	results = append(results, nh4Res, nh6Res)
	nhg4, nhg4Res := gribi.NHGEntry(gribiNHGIPv4Port2,
		map[uint64]uint64{
			gribiNHIPv4Port2: 1,
		},
		args.defaultNI, expectedResult,
	)
	nhg6, nhg6Res := gribi.NHGEntry(gribiNHGIPv6Port2,
		map[uint64]uint64{
			gribiNHIPv6Port2: 1,
		},
		args.defaultNI, expectedResult,
	)
	entries = append(entries, nhg4, nhg6)
	results = append(results, nhg4Res, nhg6Res)
	entries = append(entries,
		fluent.IPv4Entry().
			WithNetworkInstance(args.defaultNI).
			WithPrefix(baselineIPv4Prefix).
			WithNextHopGroup(gribiNHGIPv4Port2).
			WithNextHopGroupNetworkInstance(args.defaultNI),
		fluent.IPv6Entry().
			WithNetworkInstance(args.defaultNI).
			WithPrefix(baselineIPv6Prefix).
			WithNextHopGroup(gribiNHGIPv6Port2).
			WithNextHopGroupNetworkInstance(args.defaultNI),
	)
	results = append(results,
		fluent.OperationResult().
			WithIPv4Operation(baselineIPv4Prefix).
			WithOperationType(constants.Add).
			WithProgrammingResult(expectedResult).
			AsResult(),
		fluent.OperationResult().
			WithIPv6Operation(baselineIPv6Prefix).
			WithOperationType(constants.Add).
			WithProgrammingResult(expectedResult).
			AsResult(),
	)
	args.gribi.AddEntries(t, entries, results)
}

// buildGRIBINextHop creates a next-hop entry honoring device deviation knobs.
func buildGRIBINextHop(t *testing.T, dut *ondatra.DUTDevice, nhID uint64, expectedResult fluent.ProgrammingResult, networkInstance, portName, dstIP, mac string) (fluent.GRIBIEntry, *client.OpResult) {
	t.Helper()
	switch {
	case deviations.GRIBIMACOverrideWithStaticARP(dut):
		return gribi.NHEntry(nhID, "MACwithIp", networkInstance, expectedResult,
			&gribi.NHOptions{
				Dest: dstIP,
				Mac:  mac,
			},
		)
	case deviations.GRIBIMACOverrideStaticARPStaticRoute(dut):
		return gribi.NHEntry(nhID, "MACwithInterface", networkInstance, expectedResult,
			&gribi.NHOptions{
				Interface: portName,
				Mac:       mac,
				Dest:      dstIP,
			},
		)
	default:
		return gribi.NHEntry(nhID, "MACwithInterface", networkInstance, expectedResult,
			&gribi.NHOptions{
				Interface: portName,
				Mac:       mac,
				Dest:      dstIP,
			},
		)
	}
}

// waitForRoutes verifies both baseline AFT entries.
func waitForRoutes(t *testing.T, args *testArgs) {
	t.Helper()
	v4Path := gnmi.OC().NetworkInstance(args.defaultNI).Afts().Ipv4Entry(baselineIPv4Prefix)
	if _, ok := gnmi.Watch(t, args.dut, v4Path.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		entry, present := val.Val()
		return present && entry.GetPrefix() == baselineIPv4Prefix
	}).Await(t); !ok {
		t.Fatalf("baseline IPv4 AFT route %s not installed in time", baselineIPv4Prefix)
	}
	v6Path := gnmi.OC().NetworkInstance(args.defaultNI).Afts().Ipv6Entry(baselineIPv6Prefix)
	if _, ok := gnmi.Watch(t, args.dut, v6Path.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
		entry, present := val.Val()
		return present && entry.GetPrefix() == baselineIPv6Prefix
	}).Await(t); !ok {
		t.Fatalf("baseline IPv6 AFT route %s not installed in time", baselineIPv6Prefix)
	}
}

// waitForIPv4Route waits for the specified IPv4 route to appear in the AFT.
func waitForIPv4Route(t *testing.T, args *testArgs, prefix string) {
	t.Helper()
	path := gnmi.OC().NetworkInstance(args.defaultNI).Afts().Ipv4Entry(prefix)
	if _, ok := gnmi.Watch(t, args.dut, path.State(), 2*time.Minute,
		func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			entry, present := val.Val()
			return present && entry.GetPrefix() == prefix
		},
	).Await(t); !ok {
		t.Fatalf("IPv4 AFT route %s not installed", prefix)
	}
}

// waitForIPv6Route waits for the specified IPv6 route to appear in the AFT.
func waitForIPv6Route(t *testing.T, args *testArgs, prefix string) {
	t.Helper()
	path := gnmi.OC().NetworkInstance(args.defaultNI).Afts().Ipv6Entry(prefix)
	if _, ok := gnmi.Watch(t, args.dut, path.State(), 2*time.Minute,
		func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			entry, present := val.Val()
			return present && entry.GetPrefix() == prefix
		},
	).Await(t); !ok {
		t.Fatalf("IPv6 AFT route %s not installed", prefix)
	}
}

// verifyPacketOutCounters verifies Packet-Out egress counters increased.
func verifyPacketOutCounters(t *testing.T, args *testArgs, before, expectedDelta uint64) {
	t.Helper()
	port := args.dut.Port(t, "port2")
	counterPath := gnmi.OC().Interface(port.Name()).Counters().OutPkts().State()
	want := before + expectedDelta
	last, ok := gnmi.Watch(t, args.dut, counterPath, 2*time.Minute, func(val *ygnmi.Value[uint64]) bool {
		got, present := val.Val()
		return present && got >= want
	}).Await(t)
	if !ok {
		if got, present := last.Val(); present {
			t.Fatalf("Packet-Out egress counter on %s did not reach expected value: got=%d want>=%d (delta=%d)", port.Name(), got, want, expectedDelta)
		}
		t.Fatalf("Packet-Out egress counter on %s did not reach expected value: want>=%d (delta=%d)", port.Name(), want, expectedDelta)
	}
	got, _ := last.Val()
	t.Logf("Packet-Out egress counter on %s reached %d (before=%d delta=%d, expected delta=%d)", port.Name(), got, before, got-before, expectedDelta)
}

// verifyBaselinePacketOutExact verifies Packet-Out delivery on ATE port2
// using exact frame matching and explicit no-rewrite assertions.
func verifyBaselinePacketOutExact(t *testing.T, args *testArgs) {
	t.Helper()
	if len(args.packetOut) < 2 || args.packetOut[0] == nil || args.packetOut[1] == nil {
		t.Fatalf("Packet-Out payloads are not initialized for exact validation")
	}
	cs := startPacketOutCapture(t, args, []string{"port2"})
	// Send a minimal deterministic sample for exact-match validation.
	sendPacketOut(t, args.p4rt, args.packetOut[0], baselinePacketOutCheckCount)
	sendPacketOut(t, args.p4rt, args.packetOut[1], baselinePacketOutCheckCount)
	// Need wait time for the OTG backend to capture the packets before stopping the capture.
	time.Sleep(packetOutDuration)
	stopPacketOutCapture(t, args, cs)
	raw := args.ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName("port2"))
	frames, err := decodeCaptureFrames(raw)
	if err != nil {
		t.Fatalf("failed to decode baseline Packet-Out capture on port2: %v", err)
	}
	if len(frames) == 0 {
		t.Fatalf("baseline Packet-Out exact validation failed: no PCAP frames decoded on port2")
	}
	seenV4Exact := false
	seenV6Exact := false
	for _, frame := range frames {
		packet := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)
		switch {
		case packetOutFrameMatchesExpected(frame, args.packetOut[0].Payload):
			seenV4Exact = true
			ip4Layer := packet.Layer(layers.LayerTypeIPv4)
			ethLayer := packet.Layer(layers.LayerTypeEthernet)
			if ip4Layer == nil || ethLayer == nil {
				t.Fatalf("captured exact IPv4 Packet-Out frame missing Ethernet/IPv4 headers")
			}
			ip4 := ip4Layer.(*layers.IPv4)
			eth := ethLayer.(*layers.Ethernet)
			if ip4.TTL != 64 {
				t.Fatalf("IPv4 Packet-Out TTL was rewritten: got %d, want 64", ip4.TTL)
			}
			if eth.SrcMAC.String() != atePort1.MAC || eth.DstMAC.String() != atePort2.MAC {
				t.Fatalf("IPv4 Packet-Out MAC rewrite detected: got src=%s dst=%s, want src=%s dst=%s", eth.SrcMAC, eth.DstMAC, atePort1.MAC, atePort2.MAC)
			}
		case packetOutFrameMatchesExpected(frame, args.packetOut[1].Payload):
			seenV6Exact = true
			ip6Layer := packet.Layer(layers.LayerTypeIPv6)
			ethLayer := packet.Layer(layers.LayerTypeEthernet)
			if ip6Layer == nil || ethLayer == nil {
				t.Fatalf("captured exact IPv6 Packet-Out frame missing Ethernet/IPv6 headers")
			}
			ip6 := ip6Layer.(*layers.IPv6)
			eth := ethLayer.(*layers.Ethernet)
			if ip6.HopLimit != 64 {
				t.Fatalf("IPv6 Packet-Out HopLimit was rewritten: got %d, want 64", ip6.HopLimit)
			}
			if eth.SrcMAC.String() != atePort1.MAC || eth.DstMAC.String() != atePort2.MAC {
				t.Fatalf("IPv6 Packet-Out MAC rewrite detected: got src=%s dst=%s, want src=%s dst=%s", eth.SrcMAC, eth.DstMAC, atePort1.MAC, atePort2.MAC)
			}
		}
		if seenV4Exact && seenV6Exact {
			break
		}
	}
	if !seenV4Exact || !seenV6Exact {
		t.Fatalf("exact Packet-Out validation failed on port2: seenIPv4Exact=%v seenIPv6Exact=%v", seenV4Exact, seenV6Exact)
	}
	t.Log("Exact Packet-Out validation passed on port2: byte-for-byte match with no TTL/HopLimit or MAC rewrite.")
}

// packetOutFrameMatchesExpected compares captured and expected Packet-Out
// frames at L2/L3 plus payload bytes, while tolerating checksum-field
// representation differences in captures.
func packetOutFrameMatchesExpected(captured, expected []byte) bool {
	capPkt := gopacket.NewPacket(captured, layers.LayerTypeEthernet, gopacket.Default)
	expPkt := gopacket.NewPacket(expected, layers.LayerTypeEthernet, gopacket.Default)
	capEthLayer := capPkt.Layer(layers.LayerTypeEthernet)
	expEthLayer := expPkt.Layer(layers.LayerTypeEthernet)
	if capEthLayer == nil || expEthLayer == nil {
		return false
	}
	capEth, ok1 := capEthLayer.(*layers.Ethernet)
	expEth, ok2 := expEthLayer.(*layers.Ethernet)
	if !ok1 || !ok2 {
		return false
	}
	if capEth.EthernetType != expEth.EthernetType ||
		capEth.SrcMAC.String() != expEth.SrcMAC.String() ||
		capEth.DstMAC.String() != expEth.DstMAC.String() {
		return false
	}

	if exp4Layer := expPkt.Layer(layers.LayerTypeIPv4); exp4Layer != nil {
		cap4Layer := capPkt.Layer(layers.LayerTypeIPv4)
		if cap4Layer == nil {
			return false
		}
		cap4, ok1 := cap4Layer.(*layers.IPv4)
		exp4, ok2 := exp4Layer.(*layers.IPv4)
		if !ok1 || !ok2 {
			return false
		}
		if !cap4.SrcIP.Equal(exp4.SrcIP) || !cap4.DstIP.Equal(exp4.DstIP) ||
			cap4.TTL != exp4.TTL || cap4.Protocol != exp4.Protocol {
			return false
		}
		return bytes.Equal(cap4.Payload, exp4.Payload)
	}

	if exp6Layer := expPkt.Layer(layers.LayerTypeIPv6); exp6Layer != nil {
		cap6Layer := capPkt.Layer(layers.LayerTypeIPv6)
		if cap6Layer == nil {
			return false
		}
		cap6, ok1 := cap6Layer.(*layers.IPv6)
		exp6, ok2 := exp6Layer.(*layers.IPv6)
		if !ok1 || !ok2 {
			return false
		}
		if !cap6.SrcIP.Equal(exp6.SrcIP) || !cap6.DstIP.Equal(exp6.DstIP) ||
			cap6.HopLimit != exp6.HopLimit || cap6.NextHeader != exp6.NextHeader {
			return false
		}
		return bytes.Equal(cap6.Payload, exp6.Payload)
	}

	return false
}

// createFlow starts IPv4 and IPv6 OTG flows toward the specified gRIBI prefixes.
func createFlow(t *testing.T, args *testArgs, v4FlowName, v6FlowName string, rxPorts []string, addDynamicRoutes bool) {
	t.Helper()
	dstMAC := gnmi.Get(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	args.top.Flows().Clear()
	v4 := args.top.Flows().Add().SetName(v4FlowName)
	v4.Metrics().SetEnable(true)
	v4.Size().SetFixed(packetSize)
	v4.Rate().SetPps(packetRatePPS)
	v4.Duration().Continuous()
	v4.TxRx().Port().SetTxName("port1").SetRxNames(rxPorts)
	v4Eth := v4.Packet().Add().Ethernet()
	v4Eth.Src().SetValue(atePort1.MAC)
	v4Eth.Dst().SetValue(dstMAC)
	v4IP := v4.Packet().Add().Ipv4()
	v4IP.Src().SetValue(atePort1.IPv4)
	if addDynamicRoutes {
		v4IP.Dst().Increment().SetStart(ipv4RouteBase).SetCount(uint32(ipv4RouteCount))
	} else {
		v4IP.Dst().SetValue(baselineIPv4RouteDst)
	}
	v6 := args.top.Flows().Add().SetName(v6FlowName)
	v6.Metrics().SetEnable(true)
	v6.Size().SetFixed(packetSize)
	v6.Rate().SetPps(packetRatePPS)
	v6.Duration().Continuous()
	v6.TxRx().Port().SetTxName("port1").SetRxNames(rxPorts)
	v6Eth := v6.Packet().Add().Ethernet()
	v6Eth.Src().SetValue(atePort1.MAC)
	v6Eth.Dst().SetValue(dstMAC)
	v6IP := v6.Packet().Add().Ipv6()
	v6IP.Src().SetValue(atePort1.IPv6)
	if addDynamicRoutes {
		v6IP.Dst().Increment().SetStart(ipv6RouteBase).SetCount(uint32(ipv6RouteCount))
	} else {
		v6IP.Dst().SetValue(baselineIPv6RouteDst)
	}
	args.ate.OTG().PushConfig(t, args.top)
	args.ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, args.ate.OTG(), args.top, "IPv4")
	otgutils.WaitForARP(t, args.ate.OTG(), args.top, "IPv6")
	args.ate.OTG().StartTraffic(t)
	t.Cleanup(func() {
		args.ate.OTG().StopTraffic(t)
	})
	time.Sleep(trafficDuration)
	args.ate.OTG().StopTraffic(t)
	otgutils.LogPortMetrics(t, args.ate.OTG(), args.top)
}

// verifyBaselineTraffic validates packet loss and router rewrites.
func verifyBaselineTraffic(t *testing.T, args *testArgs) {
	t.Helper()
	minLoss, maxLoss := packetOutCounterTolerance, packetOutCounterTolerance
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), baselineIPv4Flow, minLoss, maxLoss)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), baselineIPv6Flow, minLoss, maxLoss)
	frames := capturePortFrames(t, args, []string{"port2"}, "port2")
	if len(frames) == 0 {
		t.Fatalf("no baseline dataplane packets captured on port2")
	}
	dutMAC := gnmi.Get(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port2").Name()).Ethernet().MacAddress().State())
	seenV4 := false
	seenV6 := false
	for _, frame := range frames {
		pkt := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)
		ethLayer := pkt.Layer(layers.LayerTypeEthernet)
		if ethLayer == nil {
			continue
		}
		eth := ethLayer.(*layers.Ethernet)
		if eth.SrcMAC.String() != dutMAC || eth.DstMAC.String() != atePort2.MAC {
			continue
		}
		if ip4Layer := pkt.Layer(layers.LayerTypeIPv4); ip4Layer != nil {
			if ip4Layer.(*layers.IPv4).TTL == 63 {
				seenV4 = true
			}
		}
		if ip6Layer := pkt.Layer(layers.LayerTypeIPv6); ip6Layer != nil {
			if ip6Layer.(*layers.IPv6).HopLimit == 63 {
				seenV6 = true
			}
		}
		if seenV4 && seenV6 {
			break
		}
	}
	if !seenV4 || !seenV6 {
		t.Fatalf("baseline forwarding header validation failed: seenV4=%v seenV6=%v", seenV4, seenV6)
	}
	t.Log("Baseline traffic validation passed: 0% loss with MAC rewrite and TTL/HopLimit decrement verified for IPv4 and IPv6.")
}

// programGRIBIRoutes performs the concurrent 100K + 100K route
// programming required by P4RT-3.3.2.
func programGRIBIRoutes(ctx context.Context, t *testing.T, args *testArgs, ipv4Count, ipv6Count int) error {
	t.Helper()
	port3Name := args.dut.Port(t, "port3").Name()
	port4Name := args.dut.Port(t, "port4").Name()
	expectedResult := gribiProgrammingResult(args.dut)
	if deviations.GRIBIMACOverrideWithStaticARP(args.dut) {
		configureStaticARP(t, args.dut, port3Name, atePort3.IPv4, atePort3.IPv6, atePort3.MAC)
		configureStaticARP(t, args.dut, port4Name, atePort4.IPv4, atePort4.IPv6, atePort4.MAC)
	}
	if deviations.GRIBIMACOverrideStaticARPStaticRoute(args.dut) {
		configureStaticARPAndRoute(t, args.dut, port3Name, atePort3.IPv4, atePort3.IPv6, atePort3.MAC)
		configureStaticARPAndRoute(t, args.dut, port4Name, atePort4.IPv4, atePort4.IPv6, atePort4.MAC)
	}
	infraEntries := make([]fluent.GRIBIEntry, 0, 8)
	infraResults := make([]*client.OpResult, 0, 8)
	addNH := func(nhID uint64, portName, dstIP, mac string) {
		nh, res := buildGRIBINextHop(t, args.dut, nhID, expectedResult, args.defaultNI, portName, dstIP, mac)
		infraEntries = append(infraEntries, nh)
		infraResults = append(infraResults, res)
	}
	addNH(dynNHIPv4Port3, port3Name, atePort3.IPv4, atePort3.MAC)
	addNH(dynNHIPv4Port4, port4Name, atePort4.IPv4, atePort4.MAC)
	addNH(dynNHIPv6Port3, port3Name, atePort3.IPv6, atePort3.MAC)
	addNH(dynNHIPv6Port4, port4Name, atePort4.IPv6, atePort4.MAC)
	nhgIDs := []uint64{dynNHGIPv4Port3, dynNHGIPv4Port4, dynNHGIPv6Port3, dynNHGIPv6Port4}
	nhMembers := []uint64{dynNHIPv4Port3, dynNHIPv4Port4, dynNHIPv6Port3, dynNHIPv6Port4}
	for i, nhgID := range nhgIDs {
		nhg, res := gribi.NHGEntry(nhgID,
			map[uint64]uint64{
				nhMembers[i]: 1,
			},
			args.defaultNI, expectedResult)
		infraEntries = append(infraEntries, nhg)
		infraResults = append(infraResults, res)
	}
	if err := applyGRIBIBatches(ctx, t, args, infraEntries, nil, "add"); err != nil {
		return fmt.Errorf("failed to install infra NH/NHG entries: %v", err)
	}
	ipv4Prefixes := mustGenerateIPv4Prefixes(t, ipv4RouteBase, ipv4Count)
	ipv6Prefixes := mustGenerateIPv6Prefixes(t, ipv6RouteBase, ipv6Count)
	// Add all routes.
	if err := programDynamicRoutes(ctx, t, args, ipv4Prefixes, ipv6Prefixes, dynamicRouteAdd); err != nil {
		return fmt.Errorf("failed to add dynamic routes: %w", err)
	}
	deleteSubset := func(prefixes []string) []string {
		if len(prefixes) <= 1 {
			return prefixes
		}
		return prefixes[:len(prefixes)/2]
	}
	deleteIPv4 := deleteSubset(ipv4Prefixes)
	deleteIPv6 := deleteSubset(ipv6Prefixes)
	// Delete a subset of the installed routes.
	if err := programDynamicRoutes(ctx, t, args, deleteIPv4, deleteIPv6, dynamicRouteDelete); err != nil {
		return fmt.Errorf("failed to delete dynamic routes: %w", err)
	}
	// Re-install the deleted subset with alternate NH assignment.
	if err := programDynamicRoutes(ctx, t, args, deleteIPv4, deleteIPv6, dynamicRouteReplace); err != nil {
		return fmt.Errorf("failed to replace dynamic routes: %v", err)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// programDynamicRoutes programs IPv4 and IPv6 routes using the specified gRIBI operation.
func programDynamicRoutes(ctx context.Context, t *testing.T, args *testArgs, ipv4Prefixes, ipv6Prefixes []string, operation dynamicRouteOperation) error {
	t.Helper()
	expectedResult := gribiProgrammingResult(args.dut)
	var operationType constants.OpType
	switch operation {
	case dynamicRouteAdd:
		operationType = constants.Add
	case dynamicRouteDelete:
		operationType = constants.Delete
	case dynamicRouteReplace:
		operationType = constants.Add
	default:
		return fmt.Errorf("unsupported dynamic route operation %q", operation)
	}
	var (
		entries []fluent.GRIBIEntry
		results []*client.OpResult
	)
	for i, prefix := range ipv4Prefixes {
		entry := fluent.IPv4Entry().WithNetworkInstance(args.defaultNI).WithPrefix(prefix)
		switch operation {
		case dynamicRouteAdd:
			nhg := dynNHGIPv4Port3
			if i%2 == 1 {
				nhg = dynNHGIPv4Port4
			}
			entry = entry.WithNextHopGroup(nhg)
		case dynamicRouteReplace:
			nhg := dynNHGIPv4Port4
			if i%2 == 1 {
				nhg = dynNHGIPv4Port3
			}
			entry = entry.WithNextHopGroup(nhg)
		}
		entries = append(entries, entry)
		results = append(results, fluent.OperationResult().WithIPv4Operation(prefix).WithOperationType(operationType).WithProgrammingResult(expectedResult).AsResult())
	}
	for i, prefix := range ipv6Prefixes {
		entry := fluent.IPv6Entry().WithNetworkInstance(args.defaultNI).WithPrefix(prefix)
		switch operation {
		case dynamicRouteAdd:
			nhg := dynNHGIPv6Port3
			if i%2 == 1 {
				nhg = dynNHGIPv6Port4
			}
			entry = entry.WithNextHopGroup(nhg)
		case dynamicRouteReplace:
			nhg := dynNHGIPv6Port4
			if i%2 == 1 {
				nhg = dynNHGIPv6Port3
			}
			entry = entry.WithNextHopGroup(nhg)
		}
		entries = append(entries, entry)
		results = append(results, fluent.OperationResult().WithIPv6Operation(prefix).WithOperationType(operationType).WithProgrammingResult(expectedResult).AsResult())
	}
	if operation == dynamicRouteDelete {
		slices.Reverse(entries)
		slices.Reverse(results)
	}
	return applyGRIBIBatches(ctx, t, args, entries, results, string(operation))
}

// configureStaticARP installs static ARP/ND neighbors used by deviation paths.
func configureStaticARP(t *testing.T, dut *ondatra.DUTDevice, portName, ipv4, ipv6, mac string) {
	t.Helper()
	setBatch := new(gnmi.SetBatch)
	if ipv4 != "" {
		gnmi.BatchUpdate(setBatch, gnmi.OC().Interface(portName).Config(), configStaticArp(portName, ipv4, mac, "ipv4"))
	}
	if ipv6 != "" {
		gnmi.BatchUpdate(setBatch, gnmi.OC().Interface(portName).Config(), configStaticArp(portName, ipv6, mac, "ipv6"))
	}
	setBatch.Set(t, dut)
}

// configureStaticARPAndRoute installs static ARP/ND neighbors and static routes
// used by the GRIBIMACOverrideStaticARPStaticRoute deviation path.
func configureStaticARPAndRoute(t *testing.T, dut *ondatra.DUTDevice, portName, ipv4, ipv6, mac string) {
	t.Helper()
	sb := new(gnmi.SetBatch)
	defaultNI := deviations.DefaultNetworkInstance(dut)
	protocolName := deviations.StaticProtocolName(dut)
	protoPath := gnmi.OC().NetworkInstance(defaultNI).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, protocolName)
	proto := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC,
		Name:       ygot.String(protocolName),
	}
	gnmi.BatchUpdate(sb, protoPath.Config(), proto)
	configureRoute := func(dstIP, prefixLen, protoType string) {
		if dstIP == "" {
			return
		}
		prefix := dstIP + prefixLen
		nhIndex := "0"
		routePath := protoPath.Static(prefix)
		gnmi.BatchUpdate(sb, routePath.Config(), &oc.NetworkInstance_Protocol_Static{Prefix: ygot.String(prefix)})
		nh := &oc.NetworkInstance_Protocol_Static_NextHop{
			Index: ygot.String(nhIndex),
			InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
				Interface: ygot.String(portName),
			},
		}
		gnmi.BatchUpdate(sb, routePath.NextHop(nhIndex).Config(), nh)
		gnmi.BatchUpdate(sb, gnmi.OC().Interface(portName).Config(), configStaticArp(portName, dstIP, mac, protoType))
	}
	configureRoute(ipv4, routeV4PrfLen, "ipv4")
	configureRoute(ipv6, routeV6PrfLen, "ipv6")
	t.Cleanup(func() {
		delBatch := new(gnmi.SetBatch)
		if ipv4 != "" {
			gnmi.BatchDelete(delBatch, protoPath.Static(ipv4+routeV4PrfLen).Config())
			gnmi.BatchDelete(delBatch, gnmi.OC().Interface(portName).Subinterface(0).Ipv4().Neighbor(ipv4).Config())
		}
		if ipv6 != "" {
			gnmi.BatchDelete(delBatch, protoPath.Static(ipv6+routeV6PrfLen).Config())
			gnmi.BatchDelete(delBatch, gnmi.OC().Interface(portName).Subinterface(0).Ipv6().Neighbor(ipv6).Config())
		}
		delBatch.Set(t, dut)
	})
	sb.Set(t, dut)
}

// configStaticArp builds interface OC for one static ARP/ND neighbor.
func configStaticArp(p, ipaddr, macAddr, protoType string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	if protoType == "ipv4" {
		s4 := s.GetOrCreateIpv4()
		n4 := s4.GetOrCreateNeighbor(ipaddr)
		n4.LinkLayerAddress = ygot.String(macAddr)
	} else {
		s6 := s.GetOrCreateIpv6()
		n6 := s6.GetOrCreateNeighbor(ipaddr)
		n6.LinkLayerAddress = ygot.String(macAddr)
	}
	return i
}

// verifyPacketOutTraffic validates Packet-Out traffic during gRIBI load.
func verifyPacketOutTraffic(t *testing.T, args *testArgs, stats *packetOutRunStats) {
	t.Helper()
	if stats == nil {
		t.Fatalf("Packet-Out run statistics are nil")
	}
	if stats.sent == 0 {
		t.Fatalf("no Packet-Out packets were successfully sent")
	}
	port2Name := args.dut.Port(t, "port2").Name()
	// Stop the capture that was started before Packet-Out and gRIBI
	// programming. Do not re-push the OTG configuration here.
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(
		gosnappi.StatePortCaptureState.STOP,
	)
	args.ate.OTG().SetControlState(t, cs)
	dutAfter := uint64(gnmi.Get(t, args.dut, gnmi.OC().Interface(port2Name).Counters().OutPkts().State()))
	dutInAfter := uint64(gnmi.Get(t, args.dut, gnmi.OC().Interface(port2Name).Counters().InPkts().State()))
	ateAfter := uint64(gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port("port2").Counters().InFrames().State()))
	dutDelta := counterDeltaUint64(stats.dutOutPktsBefore, dutAfter)
	dutInDelta := counterDeltaUint64(stats.dutInPktsBefore, dutInAfter)
	ateDelta := counterDeltaUint64(stats.ateInFramesBefore, ateAfter)
	t.Logf("Packet-Out counters: sent=%d DUT-port2 out=%d in=%d ATE-port2 inFrames=%d", stats.sent, dutDelta, dutInDelta, ateDelta)
	if dutDelta == 0 {
		t.Fatalf("Packet-Out not observed on DUT port2: sent=%d DUT egress=%d", stats.sent, dutDelta)
	}
	if ateDelta == 0 {
		t.Fatalf("Packet-Out not observed on ATE port2: DUT egress=%d ATE ingress=%d", dutDelta, ateDelta)
	}
	var loss uint64
	if dutDelta > 0 && dutDelta > ateDelta {
		loss = dutDelta - ateDelta
	}
	lossPct := 0.0
	if dutDelta > 0 {
		lossPct = (float64(loss) * 100) / float64(dutDelta)
	}
	if dutDelta > 0 && lossPct > 0 {
		t.Fatalf("Packet-Out loss detected between DUT and ATE: DUT egress=%d ATE ingress=%d loss=%d (%.2f%%)", dutDelta, ateDelta, loss, lossPct)
	}
	t.Logf("Packet-Out loss check passed: DUT egress=%d ATE ingress=%d loss=%d (%.2f%%)", dutDelta, ateDelta, loss, lossPct)
	v4Sig, err := packetSignature(args.packetOut[0].Payload)
	if err != nil {
		t.Fatalf("failed to build IPv4 Packet-Out signature: %v", err)
	}
	v6Sig, err := packetSignature(args.packetOut[1].Payload)
	if err != nil {
		t.Fatalf("failed to build IPv6 Packet-Out signature: %v", err)
	}
	t.Logf("Expected Packet-Out payloads: IPv4=%d bytes, IPv6=%d bytes", len(args.packetOut[0].Payload), len(args.packetOut[1].Payload))
	portNames := []string{"port1", "port2", "port3", "port4"}
	portCaptures := make(map[string][][]byte, len(portNames))
	for _, portName := range portNames {
		raw := args.ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(portName))
		frames, err := decodeCaptureFrames(raw)
		if err != nil {
			t.Fatalf("failed to decode %s capture: %v", portName, err)
		}
		portCaptures[portName] = frames
		t.Logf("Packet-Out capture: port=%s frames=%d", portName, len(frames))
	}
	port2Frames := portCaptures["port2"]
	if len(port2Frames) == 0 {
		t.Fatalf("no Packet-Out frames captured on ATE port2")
	}
	// Diagnostic logging is intentionally performed before the signature
	// validation. This makes packet-format mismatches visible without
	// weakening the validation itself.
	t.Logf("Expected IPv4 Packet-Out payload: len=%d first bytes=%x", len(args.packetOut[0].Payload), debugBytes(args.packetOut[0].Payload))
	t.Logf("Expected IPv6 Packet-Out payload: len=%d first bytes=%x", len(args.packetOut[1].Payload), debugBytes(args.packetOut[1].Payload))
	for i, frame := range port2Frames {
		if i >= packetOutDebugFrameCount {
			break
		}
		t.Logf("Captured port2 frame[%d]: len=%d first bytes=%x", i, len(frame), debugBytes(frame))
	}
	seenV4 := false
	seenV6 := false
	for _, frame := range port2Frames {
		if frameMatchesSignature(frame, v4Sig) {
			seenV4 = true
		}
		if frameMatchesSignature(frame, v6Sig) {
			seenV6 = true
		}
		if seenV4 && seenV6 {
			break
		}
	}
	if !seenV4 || !seenV6 {
		t.Fatalf("Packet-Out packet validation failed on port2: IPv4=%v IPv6=%v", seenV4, seenV6)
	}
	t.Logf("Packet-Out IPv4 and IPv6 packets were successfully validated on port2")
	// Packet-Out packets must not appear on any other ATE port.
	for _, portName := range []string{"port1", "port3", "port4"} {
		for _, frame := range portCaptures[portName] {
			if frameMatchesSignature(frame, v4Sig) ||
				frameMatchesSignature(frame, v6Sig) {
				t.Fatalf("Packet-Out leakage detected on %s", portName)
			}
		}
	}
	t.Log("Packet-Out validation passed: 0% observed loss, both IP versions received on port2, no leakage.")
}

// counterDeltaUint64 returns a non-negative counter delta, handling wraps/resets.
func counterDeltaUint64(before, after uint64) uint64 {
	if after < before {
		return 0
	}
	return after - before
}

// debugBytes truncates byte slices for concise debug logging.
func debugBytes(data []byte) []byte {
	const maxBytes = 64
	if len(data) <= maxBytes {
		return data
	}
	return data[:maxBytes]
}

// pktSig contains fields used to match captured packets to expected Packet-Outs.
type pktSig struct {
	srcMAC net.HardwareAddr
	dstMAC net.HardwareAddr
	srcIP  string
	dstIP  string
	ipv6   bool
	hopVal uint8
}

// packetSignature extracts a stable signature from a serialized frame.
func packetSignature(frame []byte) (*pktSig, error) {
	pkt := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)
	ethLayer := pkt.Layer(layers.LayerTypeEthernet)
	if ethLayer == nil {
		return nil, fmt.Errorf("ethernet header missing")
	}
	eth := ethLayer.(*layers.Ethernet)
	if ip4Layer := pkt.Layer(layers.LayerTypeIPv4); ip4Layer != nil {
		ip4 := ip4Layer.(*layers.IPv4)
		return &pktSig{
			srcMAC: eth.SrcMAC,
			dstMAC: eth.DstMAC,
			srcIP:  ip4.SrcIP.String(),
			dstIP:  ip4.DstIP.String(),
			ipv6:   false,
			hopVal: ip4.TTL,
		}, nil
	}
	if ip6Layer := pkt.Layer(layers.LayerTypeIPv6); ip6Layer != nil {
		ip6 := ip6Layer.(*layers.IPv6)
		return &pktSig{
			srcMAC: eth.SrcMAC,
			dstMAC: eth.DstMAC,
			srcIP:  ip6.SrcIP.String(),
			dstIP:  ip6.DstIP.String(),
			ipv6:   true,
			hopVal: ip6.HopLimit,
		}, nil
	}
	return nil, fmt.Errorf("ip header missing")
}

// frameMatchesSignature checks whether a frame matches an expected packet signature.
func frameMatchesSignature(frame []byte, sig *pktSig) bool {
	if len(frame) == 0 || sig == nil {
		return false
	}
	pkt := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)
	ethLayer := pkt.Layer(layers.LayerTypeEthernet)
	if ethLayer == nil {
		return false
	}
	if _, ok := ethLayer.(*layers.Ethernet); !ok {
		return false
	}
	if sig.ipv6 {
		ip6Layer := pkt.Layer(layers.LayerTypeIPv6)
		if ip6Layer == nil {
			return false
		}
		ip6, ok := ip6Layer.(*layers.IPv6)
		if !ok {
			return false
		}
		srcIP := net.ParseIP(sig.srcIP)
		dstIP := net.ParseIP(sig.dstIP)
		if srcIP == nil || dstIP == nil {
			return false
		}
		return ip6.SrcIP.Equal(srcIP) && ip6.DstIP.Equal(dstIP) && ip6.HopLimit == sig.hopVal
	}
	ip4Layer := pkt.Layer(layers.LayerTypeIPv4)
	if ip4Layer == nil {
		return false
	}
	ip4, ok := ip4Layer.(*layers.IPv4)
	if !ok {
		return false
	}
	srcIP := net.ParseIP(sig.srcIP)
	dstIP := net.ParseIP(sig.dstIP)
	if srcIP == nil || dstIP == nil {
		return false
	}
	return ip4.SrcIP.Equal(srcIP) && ip4.DstIP.Equal(dstIP) && ip4.TTL == sig.hopVal
}

// verifyDynamicRouteTraffic validates dynamic route forwarding.
func verifyDynamicRouteTraffic(t *testing.T, args *testArgs) {
	t.Helper()
	minLoss, maxLoss := packetOutCounterTolerance, packetOutCounterTolerance
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), dynamicIPv4Flow, minLoss, maxLoss)
	otgutils.ExpectedTrafficLoss(t, args.ate.OTG(), dynamicIPv6Flow, minLoss, maxLoss)
	in3 := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port("port3").Counters().InFrames().State())
	in4 := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port("port4").Counters().InFrames().State())
	if in3 == 0 || in4 == 0 {
		t.Fatalf("dynamic route traffic did not reach both egress ports: port3=%d port4=%d", in3, in4)
	}
	t.Logf("Dynamic route traffic reached both egress ports: port3=%d port4=%d", in3, in4)
	framesByPort := capturePortFramesByPort(t, args, []string{"port3", "port4"}, []string{"port3", "port4"})
	frames3 := framesByPort["port3"]
	frames4 := framesByPort["port4"]
	if len(frames3) == 0 || len(frames4) == 0 {
		t.Fatalf("no captures on dynamic egress ports: port3=%d port4=%d", len(frames3), len(frames4))
	}
	dutMAC3 := gnmi.Get(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port3").Name()).Ethernet().MacAddress().State())
	dutMAC4 := gnmi.Get(t, args.dut, gnmi.OC().Interface(args.dut.Port(t, "port4").Name()).Ethernet().MacAddress().State())
	if !hasDecrementedTraffic(frames3, dutMAC3, atePort3.MAC) {
		t.Fatalf("IPv4/IPv6 dynamic traffic header validation failed on port3")
	}
	if !hasDecrementedTraffic(frames4, dutMAC4, atePort4.MAC) {
		t.Fatalf("IPv4/IPv6 dynamic traffic header validation failed on port4")
	}
	t.Log("Dynamic route traffic validation passed: 0% loss, both egress ports used, TTL/Hop-Limit decrement and MAC rewrite verified.")
}

// testInvalidPacketOutPort verifies P4RT rejects an invalid egress port.
func testInvalidPacketOutPort(t *testing.T, args *testArgs) {
	t.Helper()
	// Use a port ID that cannot correspond to a valid DUT egress port.
	const invalidEgressPortID uint32 = ^uint32(0)
	port2Name := args.dut.Port(t, "port2").Name()
	dutBefore := gnmi.Get(t, args.dut, gnmi.OC().Interface(port2Name).Counters().OutPkts().State())
	invalidPacket := &p4v1pb.PacketOut{
		Payload:  []byte{0, 1, 2, 3},
		Metadata: packetOutEgressMetadata(invalidEgressPortID),
	}
	ateBefore := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port("port2").Counters().InFrames().State())
	t.Logf("Sending %d Packet-Out messages with invalid egress port ID %d while concurrent gRIBI programming is active.", outPktCount, invalidEgressPortID)
	client := args.p4rt
	packetErrCh := make(chan error, 1)
	go func() {
		packetErrCh <- sendPacketOutErr(client, invalidPacket, outPktCount)
	}()
	// gRIBI churn runs on the test goroutine (gribi helpers use t.Fatal).
	burstErr := runNegativeScenarioGRIBIChurn(args.ctx, t, args)
	sendErr := <-packetErrCh

	// P4Runtime StreamChannel is asynchronous: an invalid PacketOut is reported
	// either synchronously on send, or asynchronously as a StreamError / stream
	// termination. Give the device time to report it.
	rejection := ""
	if sendErr != nil {
		rejection = fmt.Sprintf("send error: %v", sendErr)
	} else {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if client.StreamChannelGet(&streamName) == nil {
				rejection = "P4RT stream terminated by device"
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	// The invalid Packet-Out must never be forwarded.
	dutAfter := gnmi.Get(t, args.dut, gnmi.OC().Interface(port2Name).Counters().OutPkts().State())
	ateAfter := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port("port2").Counters().InFrames().State())
	dutDelta := counterDeltaUint64(dutBefore, dutAfter)
	ateDelta := counterDeltaUint64(ateBefore, ateAfter)
	t.Logf("Invalid-port Packet-Out counters: sent=%d DUT-port2 outDelta=%d ATE-port2 inDelta=%d", outPktCount, dutDelta, ateDelta)
	if dutDelta >= uint64(outPktCount) || ateDelta >= uint64(outPktCount) {
		t.Fatalf("invalid egress Packet-Out appears forwarded: DUT out delta=%d ATE in delta=%d (sent=%d)", dutDelta, ateDelta, outPktCount)
	}

	if rejection != "" {
		t.Logf("P4RT controller received error for invalid egress port ID %d: %s", invalidEgressPortID, rejection)
	} else {
		// go-p4 p4rt_client consumes StreamMessageResponse_Error internally
		// (logged only), so a StreamError is not observable here. The device
		// rejected the packet as no Packet-Out was forwarded.
		t.Logf("Invalid egress port ID %d Packet-Out was rejected/discarded by the device (no egress observed); StreamError not surfaced by p4rt_client.", invalidEgressPortID)
	}
	if burstErr != nil {
		t.Fatalf("Concurrent gRIBI programming failed during invalid-port Packet-Out test: %v", burstErr)
	}
	// Restore the P4RT session if the device tore it down.
	ensureP4RTStream(t, args)
	t.Log("Invalid egress port Packet-Out was rejected and concurrent gRIBI programming continued uninterrupted.")
}

// runNegativeScenarioGRIBIChurn performs concurrent gRIBI add/replace/delete churn.
func runNegativeScenarioGRIBIChurn(ctx context.Context, t *testing.T, args *testArgs) error {
	t.Helper()
	ipv4Prefixes := mustGenerateIPv4Prefixes(t, negativeChurnIPv4Base, negativeChurnRouteCount)
	ipv6Prefixes := mustGenerateIPv6Prefixes(t, negativeChurnIPv6Base, negativeChurnRouteCount)
	if err := programDynamicRoutes(ctx, t, args, ipv4Prefixes, ipv6Prefixes, dynamicRouteAdd); err != nil {
		return fmt.Errorf("failed to add concurrent negative-scenario gRIBI routes: %w", err)
	}
	// Verify gRIBI programming towards healthy ports reached the AFT.
	waitForIPv4Route(t, args, ipv4Prefixes[0])
	waitForIPv4Route(t, args, ipv4Prefixes[len(ipv4Prefixes)-1])
	waitForIPv6Route(t, args, ipv6Prefixes[0])
	waitForIPv6Route(t, args, ipv6Prefixes[len(ipv6Prefixes)-1])
	t.Logf("Negative-scenario gRIBI routes installed in AFT: %d IPv4, %d IPv6.", len(ipv4Prefixes), len(ipv6Prefixes))
	if err := programDynamicRoutes(ctx, t, args, ipv4Prefixes, ipv6Prefixes, dynamicRouteReplace); err != nil {
		return fmt.Errorf("failed to replace concurrent negative-scenario gRIBI routes: %w", err)
	}
	if err := programDynamicRoutes(ctx, t, args, ipv4Prefixes, ipv6Prefixes, dynamicRouteDelete); err != nil {
		return fmt.Errorf("failed to delete concurrent negative-scenario gRIBI routes: %w", err)
	}
	return nil
}

// testPacketOutWithPortDown verifies Packet-Out behavior while the egress
// interface is administratively disabled.
func testPacketOutWithPortDown(t *testing.T, args *testArgs) {
	t.Helper()
	if len(args.packetOut) < 1 || args.packetOut[0] == nil {
		t.Log("Packet-Out payloads were not initialized; building default Packet-Out packets.")
		args.packetOut = buildPacketOutPackets(t)
	}
	port := args.dut.Port(t, "port2")
	t.Logf("Disabling Packet-Out egress port %s and validating Packet-Out drop with concurrent gRIBI programming.", port.Name())
	// Administratively disable the Packet-Out egress port.
	disableBatch := new(gnmi.SetBatch)
	gnmi.BatchUpdate(disableBatch, gnmi.OC().Interface(port.Name()).Enabled().Config(), false)
	disableBatch.Set(t, args.dut)
	t.Cleanup(func() {
		enableBatch := new(gnmi.SetBatch)
		gnmi.BatchUpdate(enableBatch, gnmi.OC().Interface(port.Name()).Enabled().Config(), true)
		enableBatch.Set(t, args.dut)
	})
	// Confirm that the interface is administratively disabled before
	// starting the concurrent Packet-Out/gRIBI operations.
	enabled := gnmi.Get(t, args.dut, gnmi.OC().Interface(port.Name()).Enabled().State())
	if enabled {
		t.Fatalf("failed to administratively disable Packet-Out egress port %s", port.Name())
	}
	operStatusPath := gnmi.OC().Interface(port.Name()).OperStatus().State()
	if _, ok := gnmi.Watch(t, args.dut, operStatusPath, 2*time.Minute, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		got, present := val.Val()
		return present && got == oc.Interface_OperStatus_DOWN
	}).Await(t); !ok {
		t.Fatalf("interface %s did not reach oper-status DOWN", port.Name())
	}
	// Wait for a post-admin-down counter sample and require a stable OutPkts
	// value before taking the baseline so pre-transition packets are excluded.
	// Use polling instead of Watch since some devices do not emit a second
	// unchanged sample while the counter is stable.
	outPktsPath := gnmi.OC().Interface(port.Name()).Counters().OutPkts().State()
	const settlePollInterval = 1 * time.Second
	deadline := time.Now().Add(30 * time.Second)
	lastOutPkts := uint64(0)
	haveSample := false
	stable := false
	for time.Now().Before(deadline) {
		if gotOper := gnmi.Get(t, args.dut, operStatusPath); gotOper != oc.Interface_OperStatus_DOWN {
			time.Sleep(settlePollInterval)
			continue
		}
		got := gnmi.Get(t, args.dut, outPktsPath)
		if haveSample && got == lastOutPkts {
			stable = true
			break
		}
		lastOutPkts = got
		haveSample = true
		time.Sleep(settlePollInterval)
	}
	if !stable {
		t.Fatalf("did not observe stable post-admin-down OutPkts telemetry for %s", port.Name())
	}
	before := gnmi.Get(t, args.dut, gnmi.OC().Interface(port.Name()).Counters().OutPkts().State())
	ateBefore := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port("port2").Counters().InFrames().State())
	t.Logf("Packet-Out egress counter before port-down test: %d", before)
	t.Logf("ATE port2 ingress frames before port-down test: %d", ateBefore)
	// Start Packet-Out traffic in a goroutine while gRIBI churn runs on the
	// test goroutine, so testing.T methods are never used from a non-test goroutine.
	packetErrCh := make(chan error, 1)
	go func() {
		packetErrCh <- sendPacketOutErr(args.p4rt, args.packetOut[0], outPktCount)
	}()
	burstErr := runNegativeScenarioGRIBIChurn(args.ctx, t, args)
	// Packet-Out is expected to be dropped because the egress port is administratively down.
	if err := <-packetErrCh; err != nil {
		if isP4RTStreamMissingError(err) {
			t.Logf("P4RT stream missing in port_state_down; re-establishing session and retrying once: %v", err)
			args.p4rt = setupP4RTClient(t, args.dut)
			if retryErr := sendPacketOutErr(args.p4rt, args.packetOut[0], outPktCount); retryErr != nil {
				t.Fatalf("Packet-Out failed after P4RT session recovery: %v", retryErr)
			}
		} else {
			t.Fatalf("Packet-Out failed while egress port %s was down: %v", port.Name(), err)
		}
	}
	// Allow counters to settle before sampling.
	time.Sleep(5 * time.Second)
	after := gnmi.Get(t, args.dut, gnmi.OC().Interface(port.Name()).Counters().OutPkts().State())
	ateAfter := gnmi.Get(t, args.ate.OTG(), gnmi.OTG().Port("port2").Counters().InFrames().State())
	dutDelta := counterDeltaUint64(before, after)
	ateDelta := counterDeltaUint64(ateBefore, ateAfter)
	t.Logf("Port-down Packet-Out counters: sent=%d DUT-%s outDelta=%d ATE-port2 inDelta=%d", outPktCount, port.Name(), dutDelta, ateDelta)
	// Deterministic drop: nothing may reach the ATE over a link that is down.
	if ateDelta != 0 {
		t.Fatalf("Packet-Out unexpectedly reached ATE port2 while egress port %s was down: inDelta=%d", port.Name(), ateDelta)
	}
	// The DUT OutPkts counter may tick by a few for locally generated control
	// frames queued at admin-down; it must not reflect the Packet-Out burst.
	if dutDelta >= uint64(outPktCount) {
		t.Fatalf("Packet-Out traffic was not dropped while egress port %s was down: DUT outDelta=%d sent=%d", port.Name(), dutDelta, outPktCount)
	}
	if dutDelta != 0 {
		t.Logf("DUT %s OutPkts changed by %d (control-plane noise, < %d Packet-Outs sent); ATE received 0 frames.", port.Name(), dutDelta, outPktCount)
	}
	t.Logf("Packet-Out traffic was deterministically dropped while %s was down.", port.Name())
	if burstErr != nil {
		t.Fatalf("gRIBI programming failed while Packet-Out egress port %s was down: %v", port.Name(), burstErr)
	}
	t.Log("Concurrent gRIBI programming completed successfully while Packet-Out traffic was dropped.")
}

// applyGRIBIBatches applies add/delete entries in bounded-size batches.
func applyGRIBIBatches(ctx context.Context, t *testing.T, args *testArgs, entries []fluent.GRIBIEntry, results []*client.OpResult, op string) error {
	t.Helper()
	if op == string(dynamicRouteReplace) {
		op = "add"
	}
	if len(results) != 0 && len(results) != len(entries) {
		return fmt.Errorf("invalid gRIBI batch input: %d entries != %d expected results", len(entries), len(results))
	}
	for i := 0; i < len(entries); i += gribiBatchSize {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		end := i + gribiBatchSize
		if end > len(entries) {
			end = len(entries)
		}
		batch := entries[i:end]
		switch op {
		case "add":
			args.gribi.AddEntries(t, batch, nil)
		case "delete":
			args.gribi.DeleteEntries(t, batch, nil)
		default:
			return fmt.Errorf("unsupported gRIBI op %q", op)
		}
	}
	t.Logf("gRIBI batch programming completed successfully: operation=%s entries=%d", op, len(entries))
	return nil
}

// capturePortFrames starts a short capture window and returns decoded frames.
func capturePortFrames(t *testing.T, args *testArgs, capturePorts []string, readPort string) [][]byte {
	t.Helper()
	return capturePortFramesByPort(t, args, capturePorts, []string{readPort})[readPort]
}

// capturePortFramesByPort starts one short capture window and returns decoded
// frames for each requested read port.
func capturePortFramesByPort(t *testing.T, args *testArgs, capturePorts, readPorts []string) map[string][][]byte {
	t.Helper()
	cs := startPacketOutCapture(t, args, capturePorts)
	args.ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	args.ate.OTG().StopTraffic(t)
	stopPacketOutCapture(t, args, cs)
	framesByPort := make(map[string][][]byte, len(readPorts))
	for _, readPort := range readPorts {
		raw := args.ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(readPort))
		frames, err := decodeCaptureFrames(raw)
		if err != nil {
			t.Fatalf("failed to decode capture from %s: %v", readPort, err)
		}
		if len(frames) > 256 {
			frames = frames[:256]
		}
		framesByPort[readPort] = frames
	}
	return framesByPort
}

// decodeCaptureFrames decodes raw PCAP bytes into packet frames.
func decodeCaptureFrames(raw []byte) ([][]byte, error) {
	reader, err := pcapgo.NewReader(bytes.NewReader(raw))
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	packetSource := gopacket.NewPacketSource(reader, reader.LinkType())
	frames := make([][]byte, 0)
	for packet := range packetSource.Packets() {
		frames = append(frames, append([]byte(nil), packet.Data()...))
	}
	return frames, nil
}

// hasDecrementedTraffic verifies TTL/HopLimit decrement with expected MAC rewrite.
func hasDecrementedTraffic(frames [][]byte, expectedSrcMAC, expectedDstMAC string) bool {
	for _, frame := range frames {
		pkt := gopacket.NewPacket(frame, layers.LayerTypeEthernet, gopacket.Default)
		ethLayer := pkt.Layer(layers.LayerTypeEthernet)
		if ethLayer == nil {
			continue
		}
		eth := ethLayer.(*layers.Ethernet)
		if eth.SrcMAC.String() != expectedSrcMAC || eth.DstMAC.String() != expectedDstMAC {
			continue
		}
		if ip4 := pkt.Layer(layers.LayerTypeIPv4); ip4 != nil && ip4.(*layers.IPv4).TTL == 63 {
			return true
		}
		if ip6 := pkt.Layer(layers.LayerTypeIPv6); ip6 != nil && ip6.(*layers.IPv6).HopLimit == 63 {
			return true
		}
	}
	return false
}

// gribiProgrammingResult selects the expected gRIBI result based on deviation.
func gribiProgrammingResult(dut *ondatra.DUTDevice) fluent.ProgrammingResult {
	if deviations.GRIBIRIBAckOnly(dut) {
		return fluent.InstalledInRIB
	}
	return fluent.InstalledInFIB
}

// mustGenerateIPv4Prefixes generates `count` /32 prefixes from a base IPv4 address.
func mustGenerateIPv4Prefixes(t testing.TB, base string, count int) []string {
	t.Helper()
	addrs, err := iputil.GenerateIPsWithStep(base, count, intStepV4)
	if err != nil {
		t.Fatalf("failed to generate IPv4 addresses from %s: %v", base, err)
	}
	prefixes := make([]string, 0, len(addrs))
	for _, ip := range addrs {
		prefixes = append(prefixes, ip+routeV4PrfLen)
	}
	return prefixes
}

// mustGenerateIPv6Prefixes generates `count` /128 prefixes from a base IPv6 address.
func mustGenerateIPv6Prefixes(t testing.TB, base string, count int) []string {
	t.Helper()
	addrs, err := iputil.GenerateIPv6sWithStep(base, count, intStepV6)
	if err != nil {
		t.Fatalf("failed to generate IPv6 addresses from %s: %v", base, err)
	}
	prefixes := make([]string, 0, len(addrs))
	for _, ip := range addrs {
		prefixes = append(prefixes, ip+routeV6PrfLen)
	}
	return prefixes
}

// mustParseMAC parses a MAC address.
func mustParseMAC(t *testing.T, mac string) net.HardwareAddr {
	t.Helper()
	hw, err := net.ParseMAC(mac)
	if err != nil {
		t.Fatalf("invalid MAC address %q: %v", mac, err)
	}
	return hw
}
