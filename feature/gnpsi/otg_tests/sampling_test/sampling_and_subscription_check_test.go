package sampling_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
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
	gnpsipb "github.com/openconfig/gnpsi/proto/gnpsi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding/introspect"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	connectionAttempts        = 2
	connectionRetryInterval   = 10 * time.Second
	connectionTimeout         = 2 * time.Minute
	trafficTime               = 1 * time.Minute
	expectedSFlowSamplesCount = int(packetsToSend / samplingRate)
	flowCountTolerancePct     = 0.1
	subscriptionTolerance     = 2
	gnpsiClientsInParallel    = 2
	packetRate                = 1000000
	packetsToSend             = 50000000
	samplingRate              = 1000000
	port1                     = "port1"
	port2                     = "port2"
	profileName               = "gnpsiProf"
	vendorFallback            = ondatra.Vendor(0) // Fallback vendor key for unknown vendors
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
	adjustedFrameSizeMap = map[ondatra.Vendor]map[uint32]map[string]uint32{
		ondatra.JUNIPER: {
			64:   {cfgplugins.IPv4: 62, cfgplugins.IPv6: 82},
			512:  {cfgplugins.IPv4: 508, cfgplugins.IPv6: 508},
			1500: {cfgplugins.IPv4: 1496, cfgplugins.IPv6: 1496},
		},
		// Default/fallback for other vendors - uses common expected values
		vendorFallback: {
			64: {cfgplugins.IPv4: 66, cfgplugins.IPv6: 86},
			// This serves as a fallback for vendors not explicitly listed
		},
	}

	// ifIndexMap maps interface names to their ifIndex values
	ifIndexMap = map[string]uint32{}

	flow64IPv4 = flowConfig{
		name:      "FlowIPv4_64",
		ipType:    cfgplugins.IPv4,
		frameSize: 64,
	}
	flow64IPv6 = flowConfig{
		name:      "FlowIPv6_64",
		ipType:    cfgplugins.IPv6,
		frameSize: 64,
	}
	flow512IPv4 = flowConfig{
		name:      "FlowIPv4_512",
		ipType:    cfgplugins.IPv4,
		frameSize: 512,
	}
	flow512IPv6 = flowConfig{
		name:      "FlowIPv6_512",
		ipType:    cfgplugins.IPv6,
		frameSize: 512,
	}
	flow1500IPv4 = flowConfig{
		name:      "FlowIPv4_1500",
		ipType:    cfgplugins.IPv4,
		frameSize: 1500,
	}
	flow1500IPv6 = flowConfig{
		name:      "FlowIPv6_1500",
		ipType:    cfgplugins.IPv6,
		frameSize: 1500,
	}
)

type sFlowPacket struct {
	sequenceNum  uint32
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
	name        string
	flowConfigs []flowConfig
	run         func(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, fc flowConfig)
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSamplingAndSubscription(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()

	configureDUT(t, dut)

	ap1 := ate.Port(t, port1)
	ap2 := ate.Port(t, port2)

	otgPort1.AddToOTG(top, ap1, &dutPort1)
	otgPort2.AddToOTG(top, ap2, &dutPort2)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, cfgplugins.IPv4)
	otgutils.WaitForARP(t, ate.OTG(), top, cfgplugins.IPv6)

	testCases := []testCase{
		{
			name: "gNPSI 1.1: Validate DUT configuration of gNPSI server, connect OTG client and verify samples",
			flowConfigs: []flowConfig{
				flow64IPv4,
				flow64IPv6,
				flow512IPv4,
				flow512IPv6,
				flow1500IPv4,
				flow1500IPv6,
			},
			run: verifySingleSFlowClient,
		},
		{
			name: "gNPSI-1.2: Verify multiple clients can connect to the gNPSI Service and receive samples",
			flowConfigs: []flowConfig{
				flow512IPv4,
			},
			run: verifyMultipleSFlowClients,
		},
		{
			name: "gNPSI-1.3: Verify client reconnection to the gNPSI service",
			flowConfigs: []flowConfig{
				flow512IPv4,
			},
			run: verifySFlowReconnect,
		},
		{
			name: "gNPSI-1.4: Verify client connection after gNPSI service restart",
			flowConfigs: []flowConfig{
				flow512IPv4,
			},
			run: verifySFlowServiceRestart,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.run == nil {
				t.Fatalf("Test case %q has nothing to run", tc.name)
			}
			for _, fc := range tc.flowConfigs {
				configureFlow(t, top, fc)
				tc.run(t, ate, dut, top, fc)
			}
		})
	}
}

func subscribeGNPSIClient(t *testing.T, ctx context.Context, gnpsiClient gnpsipb.GNPSIClient) (gnpsipb.GNPSI_SubscribeClient, error) {
	ticker := time.NewTicker(connectionRetryInterval)
	defer ticker.Stop()
	timeout := time.After(connectionTimeout)

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("failed to connect to GNPSI server within %v seconds", connectionTimeout)
		case <-ticker.C:
			stream, err := gnpsiClient.Subscribe(ctx, &gnpsipb.Request{})
			if err != nil {
				t.Logf("Unable to connect to GNPSI server: %v, retrying", err)
				continue
			}
			return stream, nil
		}
	}
}

func receiveSamples(t *testing.T, stream gnpsipb.GNPSI_SubscribeClient, sflowPacketsToValidateChannel chan sFlowPacket) {
	defer close(sflowPacketsToValidateChannel)
	sampleCount := 0
	t.Log("Waiting for GNPSI samples")
	for {
		resp, err := stream.Recv()
		if err != nil {
			switch {
			case errors.Is(err, context.Canceled) || strings.Contains(err.Error(), context.Canceled.Error()):
				t.Log("GNPSI client disconnected")
			case errors.Is(err, io.EOF) || strings.Contains(err.Error(), io.EOF.Error()):
				t.Log("GNPSI connection closed by server")
			default:
				t.Errorf("error receiving GNPSI sample: %v", err)
			}
			return
		}

		if resp == nil {
			t.Logf("Received empty response from GNPSI stream")
			return
		}

		if len(resp.Packet) == 0 {
			continue
		}
		sflow := new(layers.SFlowDatagram)
		if decodeErr := sflow.DecodeFromBytes(resp.Packet, gopacket.NilDecodeFeedback); decodeErr != nil {
			t.Errorf("failed to decode SFlow packet: %v", decodeErr)
			continue
		}

		for _, flow := range sflow.FlowSamples {
			for _, record := range flow.Records {
				switch r := record.(type) {
				case layers.SFlowRawPacketFlowRecord:
					sampleCount++
					sflowPacketsToValidateChannel <- wrapSFlowPacket(r, flow)
					t.Logf("Received GNPSI flow sample no %d:", sampleCount)
				default:
					continue
				}
			}
		}
	}
}

func verifySingleSFlowClient(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, fc flowConfig) {
	ctx, closeContext := context.WithCancel(t.Context())
	defer closeContext()
	gnpsiClient := dut.RawAPIs().GNPSI(t)
	stream, err := subscribeGNPSIClient(t, ctx, gnpsiClient)
	if err != nil {
		t.Fatalf("Failed to connect to GNPSI server: %v", err)
	}
	otg := ate.OTG()
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	sflowPacketsToValidateChannel := make(chan sFlowPacket, 2*expectedSFlowSamplesCount)
	t.Logf("Starting traffic for %s", fc.name)
	otg.StartTraffic(t)
	go receiveSamples(t, stream, sflowPacketsToValidateChannel)
	waitForTraffic(t, otg, fc.name, trafficTime)
	stream.CloseSend()
	closeContext()
	otgutils.LogFlowMetrics(t, otg, top)

	sampleCount := len(sflowPacketsToValidateChannel)
	if sampleCount == 0 {
		t.Errorf("no samples received from GNPSI")
		return
	}

	t.Logf("Total SFlow packets: %d", len(sflowPacketsToValidateChannel))
	checkSFlowPackets(t, dut, sflowPacketsToValidateChannel, fc)
}

func verifyMultipleSFlowClients(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, flow flowConfig) {
	ctx, closeContext := context.WithCancel(t.Context())
	defer closeContext()
	gnpsiClient := dut.RawAPIs().GNPSI(t)
	otg := ate.OTG()

	ate.OTG().PushConfig(t, top)
	otg.StartProtocols(t)

	gnpsiClients := []gnpsipb.GNPSI_SubscribeClient{}

	for range gnpsiClientsInParallel {
		stream, err := subscribeGNPSIClient(t, ctx, gnpsiClient)
		if err != nil {
			t.Fatalf("Failed to connect to GNPSI server: %v", err)
		}
		gnpsiClients = append(gnpsiClients, stream)
	}

	maxDifference := func(values []int) int {
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
	}
	closeContext()

	otgutils.LogFlowMetrics(t, otg, top)
	sampleCountPerClient := make([]int, gnpsiClientsInParallel)
	for index, sflowPacketsChannel := range sflowPacketsToValidatePerClient {
		sampleCountPerClient[index] = len(sflowPacketsChannel)
	}
	sampleDiff := maxDifference(sampleCountPerClient)
	if sampleDiff > subscriptionTolerance {
		t.Errorf("flow sample count difference between clients is too high: %d, max tolerance %d", sampleDiff, subscriptionTolerance)
	}
}

func verifySFlowReconnect(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, flow flowConfig) {
	gnpsiClient := dut.RawAPIs().GNPSI(t)
	var ctx context.Context
	var closeContext context.CancelFunc
	otg := ate.OTG()
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	sampleCount := 0
	sampleCountAtDisconnect := 0

	for range connectionAttempts {
		t.Log("Connecting GNPSI client")
		sflowPacketsToValidate := make(chan sFlowPacket, 2*expectedSFlowSamplesCount)
		ctx, closeContext = context.WithCancel(t.Context())
		defer closeContext()
		stream, err := subscribeGNPSIClient(t, ctx, gnpsiClient)
		if err != nil {
			t.Fatalf("Failed to connect to GNPSI server: %v", err)
		}
		go receiveSamples(t, stream, sflowPacketsToValidate)
		t.Logf("Starting traffic for %s", flow.name)
		otg.StartTraffic(t)
		waitForTraffic(t, otg, flow.name, trafficTime)
		sampleCount += len(sflowPacketsToValidate)
		if sampleCount == 0 {
			t.Errorf("no samples received from GNPSI")
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
		t.Errorf("no SFlow packets received after GNPSI client reconnect")
		return
	}
	t.Logf("Received %d samples after GNPSI client reconnect", sampleCount-sampleCountAtDisconnect)

}

func verifySFlowServiceRestart(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config, flow flowConfig) {
	ctx, closeContext := context.WithCancel(t.Context())
	defer closeContext()
	var stream gnpsipb.GNPSI_SubscribeClient
	var err error
	gnpsiClient := dut.RawAPIs().GNPSI(t)

	otg := ate.OTG()
	otg.PushConfig(t, top)
	otg.StartProtocols(t)
	sampleCount := 0
	sampleCountAtRestart := 0

	for range connectionAttempts {
		sflowPacketsToValidate := make(chan sFlowPacket, 2*expectedSFlowSamplesCount)
		stream, err = subscribeGNPSIClient(t, ctx, gnpsiClient)
		if err != nil {
			t.Fatalf("Failed to connect to GNPSI server: %v", err)
		}
		go receiveSamples(t, stream, sflowPacketsToValidate)
		t.Logf("Starting traffic for %s", flow.name)
		otg.StartTraffic(t)
		waitForTraffic(t, otg, flow.name, trafficTime)
		sampleCount += len(sflowPacketsToValidate)
		if sampleCount == 0 {
			t.Errorf("no samples received from GNPSI")
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
		t.Errorf("no SFlow packets received after GNPSI service restart")
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
	receivedSamples := 0
	for sFlowPkt := range sFlowPackets {
		receivedSamples++
		verifySFlowPacket(t, dut, sFlowPkt, flowConfig, receivedSamples)
		switch flowConfig.ipType {
		case cfgplugins.IPv4:
			verifyIpv4SFlowSample(t, sFlowPkt)
		case cfgplugins.IPv6:
			verifyIpv6SFlowSample(t, sFlowPkt)
		}
	}

	flowCountTolerance := int(math.Round(float64(expectedSFlowSamplesCount) * flowCountTolerancePct))
	if receivedSamples < expectedSFlowSamplesCount-flowCountTolerance || receivedSamples > expectedSFlowSamplesCount+flowCountTolerance {
		t.Errorf("unexpected number of SFlow packets: got %d, want %d ± %d", receivedSamples, expectedSFlowSamplesCount, flowCountTolerance)
		return
	}
	t.Logf("Received SFlow packets: %d, within expected range %d ± %d ", receivedSamples, expectedSFlowSamplesCount, flowCountTolerance)
}

func verifySFlowPacket(t *testing.T, dut *ondatra.DUTDevice, sFlowPkt sFlowPacket, flowConfig flowConfig, pktIndex int) {
	dp1 := dut.Port(t, port1)
	dp2 := dut.Port(t, port2)
	ingressIntf := ifIndexMap[dp1.Name()]
	egressIntf := ifIndexMap[dp2.Name()]

	adjustedSize := flowConfig.frameSize
	vendor := dut.Vendor()
	
	// Try vendor-specific lookup first
	if vendorMap, found := adjustedFrameSizeMap[vendor]; found {
		if adjustedValues, found := vendorMap[flowConfig.frameSize]; found {
			if size, found := adjustedValues[flowConfig.ipType]; found {
				adjustedSize = size
			}
		}
	} else if fallbackMap, found := adjustedFrameSizeMap[vendorFallback]; found {
		if adjustedValues, found := fallbackMap[flowConfig.frameSize]; found {
			if size, found := adjustedValues[flowConfig.ipType]; found {
				adjustedSize = size
			}
		}
	}

	if sFlowPkt.size != adjustedSize {
		t.Errorf("SFlow packet size %d does not match expected frame size %d", sFlowPkt.size, adjustedSize)
	}
	if sFlowPkt.samplingRate != samplingRate {
		t.Errorf("SFlow packet %d: Sampling rate %d does not match expected rate %d", pktIndex, sFlowPkt.samplingRate, samplingRate)
	}
	if sFlowPkt.ingressIntf != ingressIntf {
		t.Errorf("SFlow packet %d: Ingress interface ifindex %d does not match expected interface ifindex %d", pktIndex, sFlowPkt.ingressIntf, ingressIntf)
	}
	if sFlowPkt.egressIntf != egressIntf {
		t.Errorf("SFlow packet %d: Egress interface ifindex %d does not match expected interface ifindex %d", pktIndex, sFlowPkt.egressIntf, egressIntf)
	}

	t.Logf("SFlow Packet %d: Sequence Number %d, Size %d, Sampling rate %d, Ingress interface %d, Egress interface %d", pktIndex, sFlowPkt.sequenceNum, flowConfig.frameSize, sFlowPkt.samplingRate, sFlowPkt.ingressIntf, sFlowPkt.egressIntf)
}

func verifyIpv4SFlowSample(t *testing.T, sFlowPkt sFlowPacket) {
	ipLayer := sFlowPkt.packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		t.Errorf("no IPv4 layer found in packet")
		return
	}
	ipv4, ok := ipLayer.(*layers.IPv4)
	if !ok {
		t.Errorf("failed to extract IPv4 layer")
		return
	}
	initialSrc := net.ParseIP(otgPort1.IPv4)
	initialDst := net.ParseIP(otgPort2.IPv4)
	t.Logf("Source IP: %s, Destination IP: %s", ipv4.SrcIP, ipv4.DstIP)
	if !ipv4.SrcIP.Equal(initialSrc) || !ipv4.DstIP.Equal(initialDst) {
		t.Errorf("IPv4 source or destination IP does not match expected values: got %s -> %s, want %s -> %s", ipv4.SrcIP, ipv4.DstIP, otgPort1.IPv4, otgPort2.IPv4)
	}
}

func verifyIpv6SFlowSample(t *testing.T, sFlowPkt sFlowPacket) {
	ipLayer := sFlowPkt.packet.Layer(layers.LayerTypeIPv6)
	if ipLayer == nil {
		t.Errorf("no IPv6 layer found in packet")
		return
	}
	ipv6, ok := ipLayer.(*layers.IPv6)
	if !ok {
		t.Errorf("failed to extract IPv6 layer")
		return
	}
	initialSrc := net.ParseIP(otgPort1.IPv6)
	initialDst := net.ParseIP(otgPort2.IPv6)
	t.Logf("Source IP: %s, Destination IP: %s", ipv6.SrcIP, ipv6.DstIP)
	if !ipv6.SrcIP.Equal(initialSrc) || !ipv6.DstIP.Equal(initialDst) {
		t.Errorf("IPv6 source or destination IP does not match expected values: got %s -> %s, want %s -> %s", ipv6.SrcIP.String(), ipv6.DstIP.String(), otgPort1.IPv6, otgPort2.IPv6)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, port1)
	dp2 := dut.Port(t, port2)

	t.Logf("Configuring Loopback Interface")
	configureLoopbackInterface(t, dut)

	t.Logf("Configuring Interfaces")
	configureDUTPort(t, dut, &dutPort1, dp1)
	configureDUTPort(t, dut, &dutPort2, dp2)
	retrieveIfIndexValues(t, dut, []string{dp1.Name(), dp2.Name()})

	t.Log("Configuring SFlow")
	configureSFlow(t, dut)

	t.Log("Configuring GNPSI")
	svc := introspect.GNPSI
	dialer := introspect.DUTDialer(t, dut, svc)
	gnpsiPort := dialer.DevicePort
	params := &cfgplugins.GNPSIParams{
		Port:       gnpsiPort,
		SSLProfile: profileName,
	}
	cfgplugins.ConfigureGNPSI(t, dut, params)
}

func retrieveIfIndexValues(t *testing.T, dut *ondatra.DUTDevice, interfaceNames []string) {
	t.Helper()
	for _, intfName := range interfaceNames {
		ifPath := gnmi.OC().Interface(intfName).Ifindex().State()
		ifIndex := gnmi.Get(t, dut, ifPath)
		ifIndexMap[intfName] = ifIndex
		t.Logf("Got interface %s ifIndex: %d", intfName, ifIndex)
	}
}

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) {
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	checkState := func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}
	if _, ok := gnmi.Watch(t, otg, transmitPath, timeout, checkState).Await(t); !ok {
		t.Errorf("traffic for flow %s did not stop within the timeout of %v", flowName, timeout)
		return
	}
	t.Logf("Traffic for flow %s has stopped", flowName)
}

func configureLoopbackInterface(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	loopbackIntfName := netutil.LoopbackInterface(t, dut, 0)
	dutLoopbackAttrs.Name = loopbackIntfName
	loopbackIntf := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, loopbackIntf.Ipv4().AddressAny().State())
	if len(ipv4Addrs) == 0 {
		loopIntf := defaultLoopbackAttrs.NewOCInterface(loopbackIntfName, dut)
		loopIntf.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, gnmi.OC().Interface(loopbackIntfName).Config(), loopIntf)
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
		IP:              cfgplugins.IPv4,
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

func configureFlow(t *testing.T, config gosnappi.Config, fc flowConfig) {
	config.Flows().Clear()
	flow := config.Flows().Add().SetName(fc.name)
	flow.Metrics().SetEnable(true)
	txName := fmt.Sprintf("%s.%s", port1, fc.ipType)
	rxName := fmt.Sprintf("%s.%s", port2, fc.ipType)
	flow.TxRx().Device().SetTxNames([]string{txName}).SetRxNames([]string{rxName})
	flow.Size().SetFixed(fc.frameSize)
	flow.Rate().SetPps(packetRate)
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(packetsToSend))

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(otgPort1.MAC)

	switch fc.ipType {
	case cfgplugins.IPv4:
		ipv4 := flow.Packet().Add().Ipv4()
		ipv4.Src().SetValue(otgPort1.IPv4)
		ipv4.Dst().SetValue(otgPort2.IPv4)
	case cfgplugins.IPv6:
		ipv6 := flow.Packet().Add().Ipv6()
		ipv6.Src().SetValue(otgPort1.IPv6)
		ipv6.Dst().SetValue(otgPort2.IPv6)
	default:
		t.Errorf("invalid traffic type %s", fc.ipType)
	}
}

func wrapSFlowPacket(record layers.SFlowRawPacketFlowRecord, flow layers.SFlowFlowSample) sFlowPacket {
	return sFlowPacket{
		sequenceNum:  flow.SequenceNumber,
		ingressIntf:  flow.InputInterface,
		egressIntf:   flow.OutputInterface,
		samplingRate: flow.SamplingRate,
		size:         record.FrameLength,
		packet:       record.Header,
	}
}
