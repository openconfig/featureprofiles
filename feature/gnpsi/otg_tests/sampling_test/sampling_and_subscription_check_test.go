package sampling_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gnoi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gnpsi/proto/gnpsi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding/introspect"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	sampleSize             = 256
	trafficTime            = 40 * time.Second
	retryConnectionWait    = 10 * time.Second
	retryLimit             = 6
	connectOnce            = 1
	ipv4                   = "IPv4"
	ipv6                   = "IPv6"
	port1                  = "port1"
	port2                  = "port2"
	samplingRate           = 1000000
	packetsToSend          = 10000000
	packetRate             = 500000
	flowCountTolerancePct  = 0.8
	subscriptionTolerance  = 2
	gnpsiClientsInParallel = 2
	profileName            = "gnpsiProf"
)

var (
	// DUT ports
	dutPort1 = attrs.Attributes{
		Name:    port1,
		Desc:    "Dut port 1",
		IPv4:    "192.168.1.1",
		IPv4Len: 30,
		IPv6:    "2001:DB8::1",
		IPv6Len: 126,
	}

	dutPort2 = attrs.Attributes{
		Name:    port2,
		Desc:    "Dut port 2",
		IPv4:    "192.168.1.5",
		IPv4Len: 30,
		IPv6:    "2001:DB8::5",
		IPv6Len: 126,
	}

	// ATE ports
	otgPort1 = attrs.Attributes{
		Name:    port1,
		Desc:    "Otg port 1",
		MAC:     "00:01:12:00:00:01",
		IPv4:    "192.168.1.2",
		IPv4Len: 30,
		IPv6:    "2001:DB8::2",
		IPv6Len: 126,
	}

	otgPort2 = attrs.Attributes{
		Name:    port2,
		Desc:    "Otg port 2",
		MAC:     "00:01:12:00:00:02",
		IPv4:    "192.168.1.6",
		IPv4Len: 30,
		IPv6:    "2001:DB8::6",
		IPv6Len: 126,
	}

	defaultLoopbackAttrs = attrs.Attributes{
		Desc:    "Default loopback interface attributes",
		IPv4:    "192.0.20.2",
		IPv4Len: 32,
	}

	dutLoopbackAttrs attrs.Attributes

	// adjustedFrameSizeMap contains the actual size that the SFlow packet is reporting for a specific frame size
	// This size depends on the ip type of the packet
	// If a value is not contained in the map it is expected that the SFlow packet reported size is equal to the sent frame size
	adjustedFrameSizeMap = map[uint32]map[string]uint32{
		64: {ipv4: 66, ipv6: 86},
	}

	expectedSFlowSamplesCount = int(packetsToSend / samplingRate)
)

type sFlowPacket struct {
	ingressIntf  uint32
	egressIntf   uint32
	samplingRate uint32
	size         uint32
	packet       gopacket.Packet
}

type flowConfig struct {
	name      string
	ipType    string
	frameSize uint32
}

type testCase struct {
	name string
	run  func(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config)
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSamplingAndSubscription(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()

	configureDUT(t, dut)

	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	otgPort1.AddToOTG(top, ap1, &dutPort1)
	otgPort2.AddToOTG(top, ap2, &dutPort2)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, ipv4)
	otgutils.WaitForARP(t, ate.OTG(), top, ipv6)

	testCases := []testCase{
		{
			name: "gNPSI 1.1: Validate DUT configuration of gNPSI server, connect OTG client and verify samples",
			run:  runTC1,
		},
		{
			name: "gNPSI-1.2: Verify multiple clients can connect to the gNPSI Service and receive samples",
			run:  runTC2,
		},
		{
			name: "gNPSI-1.3: Verify client reconnection to the gNPSI service",
			run:  runTC3,
		},
		{
			name: "gNPSI-1.4: Verify client connection after gNPSI service restart",
			run:  runTC4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.run != nil {
				tc.run(t, ate, dut, top)
			}
		})
	}
}

func runTC1(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config) {
	flowConfigs := []flowConfig{
		{
			name:      "FlowIPv4_64",
			ipType:    ipv4,
			frameSize: 64,
		},
		{
			name:      "FlowIPv6_64",
			ipType:    ipv6,
			frameSize: 64,
		},
		{
			name:      "FlowIPv4_512",
			ipType:    ipv4,
			frameSize: 512,
		},
		{
			name:      "FlowIPv6_512",
			ipType:    ipv6,
			frameSize: 512,
		},
		{
			name:      "FlowIPv4_1500",
			ipType:    ipv4,
			frameSize: 1500,
		},
		{
			name:      "FlowIPv6_1500",
			ipType:    ipv6,
			frameSize: 1500,
		},
	}

	for _, fc := range flowConfigs {
		configureFlow(t, &top, &fc)
		verifySingleSFlowClient(t, ate, dut, top, fc)
	}
}

func runTC2(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config) {
	flow := flowConfig{
		name:      "Flow_TC2_IPv4_256",
		ipType:    ipv4,
		frameSize: 256,
	}

	configureFlow(t, &top, &flow)
	verifyMultipleSFlowClients(t, ate, dut, top, flow)
}

func runTC3(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config) {
	flow := flowConfig{
		name:      "Flow_TC3_IPv4_256",
		ipType:    ipv4,
		frameSize: 256,
	}

	configureFlow(t, &top, &flow)
	verifySFlowReconnect(t, ate, dut, top, flow)
}

func runTC4(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config) {
	flow := flowConfig{
		name:      "Flow_TC4_IPv4_256",
		ipType:    ipv4,
		frameSize: 256,
	}

	configureFlow(t, &top, &flow)
	verifySFlowServiceRestart(t, ate, dut, top, flow)
}

func subscribeGNPSIClient(t *testing.T, ctx context.Context, gnpsiclient gnpsi.GNPSIClient, connectionRetries uint8) (gnpsi.GNPSI_SubscribeClient, error) {
	var err error
	var stream gnpsi.GNPSI_SubscribeClient
	for connectionRetries > 0 {
		stream, err = gnpsiclient.Subscribe(ctx, &gnpsi.Request{})
		if err != nil {
			t.Logf("Error connecting to gNPSI server: %v, retries left %d", err, connectionRetries)
			connectionRetries--
			time.Sleep(retryConnectionWait)
			continue
		}
		return stream, nil
	}
	return nil, fmt.Errorf("failed to connect to gNPSI server: %v", err)
}

func receiveSamples(t *testing.T, stream gnpsi.GNPSI_SubscribeClient, sflowPacketsToValidateChannel chan sFlowPacket) {
	defer close(sflowPacketsToValidateChannel)
	t.Log("Waiting for gNPSI samples")
	resp, err := stream.Recv()
	for err == nil {
		if resp == nil {
			t.Logf("Received empty response from gNPSI stream")
			return
		}

		if len(resp.Packet) > 0 {
			sflow := new(layers.SFlowDatagram)
			err := sflow.DecodeFromBytes(resp.Packet, gopacket.NilDecodeFeedback)
			if err != nil {
				t.Errorf("Failed to decode SFlow packet: %v", err)
				continue
			}
			if len(sflow.FlowSamples) > 0 {
				for _, flow := range sflow.FlowSamples {
					for _, record := range flow.Records {
						switch r := record.(type) {
						case layers.SFlowRawPacketFlowRecord:
							sflowPacketsToValidateChannel <- wrapSFlowPacket(r, flow)
							t.Logf("Received gNPSI flow sample no %d:", len(sflowPacketsToValidateChannel))
						case layers.SFlowExtendedSwitchFlowRecord:
							continue
						case layers.SFlowExtendedRouterFlowRecord:
							continue
						default:
							t.Logf("Unknown record type: %T, value: %+v", r, r)
						}
					}
				}
			}
		}
		resp, err = stream.Recv()
	}

	if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Log("GNPSI client disconnected")
		return
	}
	if errors.Is(err, io.EOF) || strings.Contains(err.Error(), io.EOF.Error()) {
		t.Log("GNPSI connection closed by server")
		return
	}
	t.Errorf("Error receiving gNPSI sample: %v", err)
}

func verifySingleSFlowClient(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, fc flowConfig) {
	ctx, closeContext := context.WithCancel(t.Context())
	defer closeContext()
	gnpsiclient := dut.RawAPIs().GNPSI(t)
	stream, err := subscribeGNPSIClient(t, ctx, gnpsiclient, connectOnce)
	if err != nil {
		t.Fatalf("Failed to connect to gNPSI server: %v", err)
	}
	otg := ate.OTG()
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	sflowPacketsToValidateChannel := make(chan sFlowPacket, 2*expectedSFlowSamplesCount)
	go receiveSamples(t, stream, sflowPacketsToValidateChannel)
	t.Logf("Starting traffic for %s", fc.name)
	otg.StartTraffic(t)
	waitForTraffic(t, otg, fc.name, trafficTime)
	stream.CloseSend()
	closeContext()
	otgutils.LogFlowMetrics(t, otg, top)

	sampleCount := len(sflowPacketsToValidateChannel)
	if sampleCount == 0 {
		t.Errorf("No samples received from gNPSI")
		return
	}
	if len(sflowPacketsToValidateChannel) == 0 {
		t.Errorf("No SFlow packets with flow samples received from gNPSI")
		return
	}
	t.Logf("Total SFlow packets: %d", len(sflowPacketsToValidateChannel))
	checkSFlowPackets(t, dut, sflowPacketsToValidateChannel, fc)
}

func verifyMultipleSFlowClients(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, flow flowConfig) {
	ctx, closeContext := context.WithCancel(t.Context())
	defer closeContext()
	gnpsiclient := dut.RawAPIs().GNPSI(t)
	otg := ate.OTG()

	ate.OTG().PushConfig(t, top)
	otg.StartProtocols(t)

	gnpsiClients := []gnpsi.GNPSI_SubscribeClient{}

	for range gnpsiClientsInParallel {
		stream, err := subscribeGNPSIClient(t, ctx, gnpsiclient, connectOnce)
		if err != nil {
			t.Fatalf("Failed to connect to gNPSI server: %v", err)
		}
		gnpsiClients = append(gnpsiClients, stream)
	}

	getMaxDifference := func(values []int) int {
		if len(values) == 0 {
			return 0
		}
		maxValue := values[0]
		minValue := values[0]
		for _, value := range values {
			if value > maxValue {
				maxValue = value
			} else if value < minValue {
				minValue = value
			}
		}
		return maxValue - minValue
	}

	sflowPacketsToValidatePerClient := make([]chan sFlowPacket, gnpsiClientsInParallel)
	for index, client := range gnpsiClients {
		sflowPacketsToValidatePerClient[index] = make(chan sFlowPacket, 2*expectedSFlowSamplesCount)
		go receiveSamples(t, client, sflowPacketsToValidatePerClient[index])
	}

	t.Logf("Starting traffic for %s", flow.name)
	otg.StartTraffic(t)
	waitForTraffic(t, otg, flow.name, trafficTime)

	for _, client := range gnpsiClients {
		client.CloseSend()
		closeContext()
	}
	otgutils.LogFlowMetrics(t, otg, top)

	sampleCountPerClient := make([]int, gnpsiClientsInParallel)
	for index, sflowPacketsChannel := range sflowPacketsToValidatePerClient {
		sampleCountPerClient[index] = len(sflowPacketsChannel)
	}
	sampleDiff := getMaxDifference(sampleCountPerClient)
	if sampleDiff > subscriptionTolerance {
		t.Errorf("Flow sample count difference between clients is too high: %d, max tolerance %d", sampleDiff, subscriptionTolerance)
	}
}

func verifySFlowReconnect(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, flow flowConfig) {
	gnpsiclient := dut.RawAPIs().GNPSI(t)
	var ctx context.Context
	var closeContext context.CancelFunc
	otg := ate.OTG()
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	sampleCount := 0
	sampleCountAtDisconnect := 0

	for range 2 {
		t.Log("Connecting GNPSI client")
		sflowPacketsToValidate := make(chan sFlowPacket, 2*expectedSFlowSamplesCount)
		ctx, closeContext = context.WithCancel(t.Context())
		defer closeContext()
		stream, err := subscribeGNPSIClient(t, ctx, gnpsiclient, connectOnce)
		if err != nil {
			t.Fatalf("Failed to connect to gNPSI server: %v", err)
		}
		go receiveSamples(t, stream, sflowPacketsToValidate)
		t.Logf("Starting traffic for %s", flow.name)
		otg.StartTraffic(t)
		waitForTraffic(t, otg, flow.name, trafficTime)
		sampleCount += len(sflowPacketsToValidate)
		if sampleCount == 0 {
			t.Errorf("No samples received from gNPSI")
			return
		}
		if sampleCountAtDisconnect == 0 {
			sampleCountAtDisconnect = sampleCount
			t.Logf("Received %d samples before GNPSI client reconnect", sampleCountAtDisconnect)
		}
		stream.CloseSend()
		closeContext()
		otgutils.LogFlowMetrics(t, otg, top)
	}

	if sampleCountAtDisconnect == sampleCount {
		t.Errorf("No SFlow packets received after GNPSI client reconnect")
		return
	}
	t.Logf("Received %d samples after GNPSI client reconnect", sampleCount-sampleCountAtDisconnect)

}

func verifySFlowServiceRestart(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, flow flowConfig) {
	ctx, closeContext := context.WithCancel(t.Context())
	defer closeContext()
	var stream gnpsi.GNPSI_SubscribeClient
	var err error
	gnpsiclient := dut.RawAPIs().GNPSI(t)

	otg := ate.OTG()
	otg.PushConfig(t, top)
	otg.StartProtocols(t)
	sampleCount := 0
	sampleCountAtRestart := 0

	for range 2 {
		sflowPacketsToValidate := make(chan sFlowPacket, 2*expectedSFlowSamplesCount)
		stream, err = subscribeGNPSIClient(t, ctx, gnpsiclient, retryLimit)
		if err != nil {
			t.Fatalf("Failed to connect to gNPSI server: %v", err)
		}
		go receiveSamples(t, stream, sflowPacketsToValidate)
		t.Logf("Starting traffic for %s", flow.name)
		otg.StartTraffic(t)
		waitForTraffic(t, otg, flow.name, trafficTime)
		sampleCount += len(sflowPacketsToValidate)
		if sampleCount == 0 {
			t.Errorf("No samples received from gNPSI")
			return
		}
		if sampleCountAtRestart == 0 {
			sampleCountAtRestart = sampleCount
			t.Logf("Received %d samples before GNPSI service restart", sampleCountAtRestart)
			restartGNPSIService(t, dut)
			continue
		}
		stream.CloseSend()
		closeContext()
		otgutils.LogFlowMetrics(t, otg, top)
	}

	if sampleCountAtRestart == sampleCount {
		t.Errorf("No SFlow packets received after GNPSI service restart")
		return
	}
	t.Logf("Received %d samples after GNPSI service restart", sampleCount-sampleCountAtRestart)
}

func restartGNPSIService(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Restarting GNPSI service")
	gnoi.KillProcess(t, dut, gnoi.GNPSI, gnoi.SigTerm, true, true)
	t.Log("GNPSI service restarted")
}

func checkSFlowPackets(t *testing.T, dut *ondatra.DUTDevice, sFlowPackets chan sFlowPacket, flowConfig flowConfig) {
	flowCountTolerance := int(float32(expectedSFlowSamplesCount)*flowCountTolerancePct) + 1
	if len(sFlowPackets) < expectedSFlowSamplesCount-flowCountTolerance || len(sFlowPackets) > expectedSFlowSamplesCount+flowCountTolerance {
		t.Errorf("Unexpected number of sFlow packets: got %d, want %d ± %d",
			len(sFlowPackets), expectedSFlowSamplesCount, flowCountTolerance)
	} else {
		t.Logf("Received sFlow packets: %d, within expected range %d ± %d ", len(sFlowPackets), expectedSFlowSamplesCount, flowCountTolerance)
	}

	index := 0
	for sFlowPkt := range sFlowPackets {
		index++
		verifySFlowPacket(t, dut, sFlowPkt, flowConfig, index)
		switch flowConfig.ipType {
		case ipv4:
			verifyIpv4SFlowSample(t, sFlowPkt)
		case ipv6:
			verifyIpv6SFlowSample(t, sFlowPkt)
		}
	}
}

func verifySFlowPacket(t *testing.T, dut *ondatra.DUTDevice, sFlowPkt sFlowPacket, flowConfig flowConfig, pktIndex int) {
	dp1 := dut.Port(t, port1)
	dp2 := dut.Port(t, port2)
	ingressIntf := processInterfaceNumber(dut, dp1.Name())
	egressIntf := processInterfaceNumber(dut, dp2.Name())

	adjustedSize := flowConfig.frameSize
	if adjustedValues, found := adjustedFrameSizeMap[flowConfig.frameSize]; found {
		adjustedSize = adjustedValues[flowConfig.ipType]
	}

	if sFlowPkt.size != adjustedSize {
		t.Errorf("SFlow packet size %d does not match expected frame size %d", sFlowPkt.size, flowConfig.frameSize)
	}
	if sFlowPkt.samplingRate != samplingRate {
		t.Errorf("SFlow packet %d: Sampling rate %d does not match expected rate %d", pktIndex, sFlowPkt.samplingRate, samplingRate)
	}
	if sFlowPkt.ingressIntf != ingressIntf {
		t.Errorf("SFlow packet %d: Ingress interface %d does not match expected interface %d", pktIndex, sFlowPkt.ingressIntf, ingressIntf)
	}
	if sFlowPkt.egressIntf != egressIntf {
		t.Errorf("SFlow packet %d: Egress interface %d does not match expected interface %d", pktIndex, sFlowPkt.egressIntf, egressIntf)
	}

	t.Logf("SFlow Packet %d: Size %d, Sampling rate %d, Ingress interface %d, Egress interface %d", pktIndex, flowConfig.frameSize, samplingRate, ingressIntf, egressIntf)
}

func verifyIpv4SFlowSample(t *testing.T, sFlowPkt sFlowPacket) {
	ipLayer := sFlowPkt.packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		t.Errorf("No IPv4 layer found in packet")
		return
	}
	ipv4, ok := ipLayer.(*layers.IPv4)
	if !ok {
		t.Errorf("Failed to extract IPv4 layer")
		return
	}
	initialSrc := net.ParseIP(otgPort1.IPv4)
	initialDst := net.ParseIP(otgPort2.IPv4)
	t.Logf("Source IP: %s, Destination IP: %s", ipv4.SrcIP, ipv4.DstIP)
	if !ipv4.SrcIP.Equal(initialSrc) || !ipv4.DstIP.Equal(initialDst) {
		t.Errorf("IPv4 source or destination IP does not match expected values: got %s -> %s, want %s -> %s",
			ipv4.SrcIP, ipv4.DstIP, otgPort1.IPv4, otgPort2.IPv4)
	}
}

func verifyIpv6SFlowSample(t *testing.T, sFlowPkt sFlowPacket) {
	ipLayer := sFlowPkt.packet.Layer(layers.LayerTypeIPv6)
	if ipLayer == nil {
		t.Errorf("No IPv6 layer found in packet")
		return
	}
	ipv6, ok := ipLayer.(*layers.IPv6)
	if !ok {
		t.Errorf("Failed to extract IPv6 layer")
		return
	}
	initialSrc := net.ParseIP(otgPort1.IPv6)
	initialDst := net.ParseIP(otgPort2.IPv6)
	t.Logf("Source IP: %s, Destination IP: %s", ipv6.SrcIP, ipv6.DstIP)
	if !ipv6.SrcIP.Equal(initialSrc) || !ipv6.DstIP.Equal(initialDst) {
		t.Errorf("IPv6 source or destination IP does not match expected values: got %s -> %s, want %s -> %s",
			ipv6.SrcIP.String(), ipv6.DstIP.String(), otgPort1.IPv6, otgPort2.IPv6)
	}
}

func processInterfaceNumber(dut *ondatra.DUTDevice, intfName string) uint32 {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		if strings.HasPrefix(intfName, "Ethernet") {
			num, _ := strings.CutPrefix(intfName, "Ethernet")
			parts := strings.Split(num, "/")
			if len(parts) == 2 {
				slot, _ := strconv.Atoi(parts[0])
				port, _ := strconv.Atoi(parts[1])
				return uint32(slot*1000 + port)
			}
			if n, err := strconv.Atoi(num); err == nil {
				return uint32(n)
			}
		}
	default:
		if n, err := strconv.Atoi(intfName); err == nil {
			return uint32(n)
		}
	}
	return 0
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	t.Logf("Configuring Loopback Interface")
	configureLoopbackInterface(t, dut)

	t.Logf("Configuring Interfaces")
	configureDUTPort(t, dut, &dutPort1, dp1)
	configureDUTPort(t, dut, &dutPort2, dp2)

	t.Log("Configuring sFlow")
	configureSFlow(t, dut)

	t.Log("Configuring gNPSI")
	svc := introspect.GNPSI
	dialer := introspect.DUTDialer(t, dut, svc)
	gnpsiPort := dialer.DevicePort
	params := &cfgplugins.GNPSIParams{
		Port:       gnpsiPort,
		SSLProfile: profileName,
	}
	cfgplugins.ConfigureGNPSI(t, dut, params)
}

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) {
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	checkState := func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}
	_, ok := gnmi.Watch(t, otg, transmitPath, timeout, checkState).Await(t)

	if !ok {
		t.Errorf("Traffic for flow %s did not stop within the timeout of %d", flowName, timeout)
	} else {
		t.Logf("Traffic for flow %s has stopped", flowName)
	}
}

func configureLoopbackInterface(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dc := gnmi.OC()
	loopbackIntfName := netutil.LoopbackInterface(t, dut, 0)
	dutLoopbackAttrs.Name = loopbackIntfName
	loopbackIntf := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, loopbackIntf.Ipv4().AddressAny().State())
	if len(ipv4Addrs) == 0 {
		loopIntf := defaultLoopbackAttrs.NewOCInterface(loopbackIntfName, dut)
		loopIntf.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, dc.Interface(loopbackIntfName).Config(), loopIntf)
		dutLoopbackAttrs.IPv4 = defaultLoopbackAttrs.IPv4
		return
	}

	v4, ok := ipv4Addrs[0].Val()
	if !ok {
		t.Fatalf("Unable to get loopback ipv4 address for %s", loopbackIntfName)
	}
	dutLoopbackAttrs.IPv4 = v4.GetIp()
	t.Logf("Got DUT IPv4 loopback address: %v", dutLoopbackAttrs.IPv4)
}

func configureSFlow(t *testing.T, dut *ondatra.DUTDevice) {
	sfBatch := &gnmi.SetBatch{}

	sflowParams := &cfgplugins.SFlowGlobalParams{
		Ni:              deviations.DefaultNetworkInstance(dut),
		IntfName:        dutLoopbackAttrs.Name,
		SrcAddrV4:       dutLoopbackAttrs.IPv4,
		IP:              ipv4,
		MinSamplingRate: samplingRate,
	}
	cfgplugins.NewSFlowGlobalCfg(t, sfBatch, nil, dut, sflowParams)
	sfBatch.Set(t, dut)
}

func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, attrs *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	d := gnmi.OC()
	intf := attrs.NewOCInterface(p.Name(), dut)
	gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), intf)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
		t.Logf("DUT %s %s %s requires explicit interface in default VRF deviation ", dut.Vendor(), dut.Model(), dut.Version())
	}
}

func configureFlow(t *testing.T, config *gosnappi.Config, fc *flowConfig) {
	(*config).Flows().Clear()
	flow := (*config).Flows().Add().SetName(fc.name)
	flow.Metrics().SetEnable(true)
	txName := fmt.Sprintf("%s.%s", port1, fc.ipType)
	rxName := fmt.Sprintf("%s.%s", port2, fc.ipType)
	flow.TxRx().Device().
		SetTxNames([]string{txName}).
		SetRxNames([]string{rxName})
	flow.Size().SetFixed(fc.frameSize)
	flow.Rate().SetPps(packetRate)
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(packetsToSend))

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(otgPort1.MAC)

	switch fc.ipType {
	case ipv4:
		ipv4 := flow.Packet().Add().Ipv4()
		ipv4.Src().SetValue(otgPort1.IPv4)
		ipv4.Dst().SetValue(otgPort2.IPv4)
	case ipv6:
		ipv6 := flow.Packet().Add().Ipv6()
		ipv6.Src().SetValue(otgPort1.IPv6)
		ipv6.Dst().SetValue(otgPort2.IPv6)
	default:
		t.Errorf("Invalid traffic type %s", fc.ipType)
	}
}

func wrapSFlowPacket(record layers.SFlowRawPacketFlowRecord, flow layers.SFlowFlowSample) sFlowPacket {
	return sFlowPacket{
		ingressIntf:  flow.InputInterface,
		egressIntf:   flow.OutputInterface,
		samplingRate: flow.SamplingRate,
		size:         record.FrameLength,
		packet:       record.Header,
	}
}
