package sampling_test

import (
	"fmt"
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
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gnpsi/proto/gnpsi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding/introspect"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	sampleSize                    = 256
	trafficTime                   = 30 * time.Second
	ipv4                          = "IPv4"
	ipv6                          = "IPv6"
	port1                         = "port1"
	port2                         = "port2"
	samplingRate                  = 1000000
	packetRate                    = 500000
	defaultPacketsToSend   uint32 = 6000000
	flowCountTolerancePct         = 0.5
	subscriptionTolerance         = 2
	gnpsiClientsInParallel        = 2
	reconnectRetries              = 5
	reconnectWaitTime             = 3 * time.Second
	profileName                   = "gnpsiProf"
	certFile                      = "gnpsi.crt"
	keyFile                       = "gnpsi.key"
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

	//adjustedFrameSizeMap contains the actual size that the SFlow packet is reporting for a specific frame size
	//This size depends on the ip type of the packet
	//If a value is not contained in the map it is expected that the SFlow packet reported size is equal to the sent frame size
	adjustedFrameSizeMap = map[uint32]map[string]uint32{
		64: {ipv4: 66, ipv6: 86},
	}
)

type sFlowPacket struct {
	ingressIntf  uint32
	egressIntf   uint32
	samplingRate uint32
	size         uint32
	packet       gopacket.Packet
}

type flowConfig struct {
	name          string
	ipType        string
	frameSize     uint32
	packetsToSend uint32
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

	ap1 := ate.Port(t, port1)
	ap2 := ate.Port(t, port2)

	otgPort1.AddToOTG(top, ap1, &dutPort1)
	otgPort2.AddToOTG(top, ap2, &dutPort2)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, ipv4)
	otgutils.WaitForARP(t, ate.OTG(), top, ipv6)

	testCases := []testCase{{
		name: "gNPSI 1.1: Validate DUT configuration of gNPSI server, connect OTG client and verify samples",
		run:  verifySFlowSamplesMultipleFlows,
	}, {
		name: "gNPSI-1.2: Verify multiple clients can connect to the gNPSI Service and receive samples",
		run:  verifyMultipleSFlowClients,
	}, {
		name: "gNPSI-1.3: Verify client reconnection to the gNPSI service",
		run:  verifySFlowReconnect,
	}, {
		name: "gNPSI-1.4: Verify client connection after gNPSI service restart",
		run:  verifySFlowServiceRestart,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.run != nil {
				tc.run(t, ate, dut, top)
			}
		})
	}
}

func verifySFlowSamplesMultipleFlows(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config) {
	otg := ate.OTG()

	flowConfigs := []flowConfig{
		{
			name:          "FlowIPv4_64",
			ipType:        ipv4,
			frameSize:     64,
			packetsToSend: defaultPacketsToSend,
		}, {
			name:          "FlowIPv6_64",
			ipType:        ipv6,
			frameSize:     64,
			packetsToSend: defaultPacketsToSend,
		}, {
			name:          "FlowIPv4_512",
			ipType:        ipv4,
			frameSize:     512,
			packetsToSend: defaultPacketsToSend,
		}, {
			name:          "FlowIPv6_512",
			ipType:        ipv6,
			frameSize:     512,
			packetsToSend: defaultPacketsToSend,
		}, {
			name:          "FlowIPv4_1500",
			ipType:        ipv4,
			frameSize:     1500,
			packetsToSend: defaultPacketsToSend,
		}, {
			name:          "FlowIPv6_1500",
			ipType:        ipv6,
			frameSize:     1500,
			packetsToSend: defaultPacketsToSend,
		},
	}

	gnpsiclient := dut.RawAPIs().GNPSI(t)
	stream, err := gnpsiclient.Subscribe(t.Context(), &gnpsi.Request{})
	if err != nil {
		t.Fatalf("Failed to connect to gNPSI server: %v", err)
	}

	wrapSFlowPacket := func(record layers.SFlowRawPacketFlowRecord, flow layers.SFlowFlowSample) sFlowPacket {
		return sFlowPacket{
			ingressIntf:  flow.InputInterface,
			egressIntf:   flow.OutputInterface,
			samplingRate: flow.SamplingRate,
			size:         record.FrameLength,
			packet:       record.Header,
		}
	}

	for _, fc := range flowConfigs {
		configureFlow(t, &top, &fc)
		ate.OTG().PushConfig(t, top)
		otg.StartProtocols(t)

		sampleCount := 0
		samplesWithFlows := 0
		sFlowPacketsToValidate := []sFlowPacket{}
		t.Logf("Starting traffic for %s", fc.name)
		go func() {
			otg.StartTraffic(t)
			waitForTraffic(t, otg, fc.name, trafficTime)
		}()
		timeout := time.After(trafficTime)
		continueLoop := true
		for continueLoop {
			select {
			case <-timeout:
				t.Logf("Received %d flow samples", samplesWithFlows)
				if sampleCount == 0 {
					t.Errorf("[ERROR] No samples received from gNPSI")
				} else {
					t.Logf("Total SFlow packets: %d", len(sFlowPacketsToValidate))
					checkSFlowPackets(t, dut, sFlowPacketsToValidate, fc)
				}
				continueLoop = false
			default:
				resp, err := stream.Recv()
				if err != nil {
					t.Fatalf("[ERROR] Error receiving gNPSI sample: %v", err)
				}
				sampleCount++
				t.Logf("Received gNPSI sample no %d:", sampleCount)
				if len(resp.Packet) == 0 {
					t.Logf("[ERROR] No packet data in gNPSI sample")
					continue
				}
				sFlow := new(layers.SFlowDatagram)
				err = sFlow.DecodeFromBytes(resp.Packet, gopacket.NilDecodeFeedback)
				if err != nil {
					t.Errorf("[ERROR] Failed to decode SFlow packet: %v", err)
					continue
				}
				if len(sFlow.FlowSamples) == 0 {
					t.Logf("No flow samples in this SFlow packet")
					continue
				}

				t.Logf("Found flow samples in this SFlow packet")
				samplesWithFlows++
				for _, flow := range sFlow.FlowSamples {
					for _, record := range flow.Records {
						switch r := record.(type) {
						case layers.SFlowRawPacketFlowRecord:
							sFlowPacketsToValidate = append(sFlowPacketsToValidate, wrapSFlowPacket(r, flow))
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
		otgutils.LogFlowMetrics(t, otg, top)
	}
}

func verifyMultipleSFlowClients(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config) {
	otg := ate.OTG()
	flow := flowConfig{
		name:          "Flow_TC2_IPv4_256",
		ipType:        ipv4,
		frameSize:     256,
		packetsToSend: 10000000,
	}

	configureFlow(t, &top, &flow)
	ate.OTG().PushConfig(t, top)
	otg.StartProtocols(t)

	gnpsiClients := []gnpsi.GNPSI_SubscribeClient{}

	for range gnpsiClientsInParallel {
		gnpsiclient := dut.RawAPIs().GNPSI(t)
		stream, err := gnpsiclient.Subscribe(t.Context(), &gnpsi.Request{})
		if err != nil {
			t.Fatalf("Failed to connect to gNPSI server: %v", err)
		}
		gnpsiClients = append(gnpsiClients, stream)
	}

	sampleCountPerClient := []int{0, 0}
	flowSampleCountPerClient := []int{0, 0}

	t.Logf("Starting traffic for %s", flow.name)
	otg.StartTraffic(t)

	defer otgutils.LogFlowMetrics(t, otg, top)
	defer otg.StopTraffic(t)

	timeout := time.After(trafficTime)
	index := -1
	for {
		select {
		case <-timeout:
			for i, count := range sampleCountPerClient {
				if count == 0 {
					t.Errorf("[ERROR] No samples received from client %d", i+1)
				} else {
					t.Logf("Client %d received %d samples with %d flow samples", i+1, count, flowSampleCountPerClient[i])
				}
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

			sampleDiff := getMaxDifference(sampleCountPerClient)
			if sampleDiff > subscriptionTolerance {
				t.Errorf("[ERROR] Sample count difference between clients is too high: %d, max tolerance %d", sampleDiff, subscriptionTolerance)
			}
			return

		default:
			index = (index + 1) % len(gnpsiClients)
			stream := gnpsiClients[index]
			t.Logf("Receiving samples from client %d", index+1)
			resp, err := stream.Recv()
			if err != nil {
				t.Errorf("[ERROR] Error receiving gNPSI sample: %v", err)
				continue
			}
			t.Logf("Received gNPSI sample no %d from client %d", sampleCountPerClient[index], index+1)
			sampleCountPerClient[index]++
			if len(resp.Packet) > 0 {
				sFlow := new(layers.SFlowDatagram)
				err := sFlow.DecodeFromBytes(resp.Packet, gopacket.NilDecodeFeedback)
				if err != nil {
					t.Errorf("[ERROR] Failed to decode SFlow packet: %v", err)
					continue
				}
				if len(sFlow.FlowSamples) > 0 {
					t.Logf("Found flow samples in this SFlow packet")
					for _, flow := range sFlow.FlowSamples {
						for _, record := range flow.Records {
							switch r := record.(type) {
							case layers.SFlowRawPacketFlowRecord:
								flowSampleCountPerClient[index]++
							case layers.SFlowExtendedSwitchFlowRecord:
								continue
							case layers.SFlowExtendedRouterFlowRecord:
								continue
							default:
								t.Logf("Unknown record type: %T, value: %+v", r, r)
							}
						}
					}
				} else {
					t.Logf("No flow samples in this SFlow packet")
				}
			}
		}
	}

}

func verifySFlowReconnect(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config) {
	otg := ate.OTG()
	flow := flowConfig{
		name:          "Flow_TC3_IPv4_256",
		ipType:        ipv4,
		frameSize:     256,
		packetsToSend: 20000000,
	}

	configureFlow(t, &top, &flow)
	ate.OTG().PushConfig(t, top)
	otg.StartProtocols(t)

	gnpsiclient := dut.RawAPIs().GNPSI(t)
	stream, err := gnpsiclient.Subscribe(t.Context(), &gnpsi.Request{})
	if err != nil {
		t.Fatalf("Failed to connect to gNPSI server: %v", err)
	}

	t.Logf("Starting traffic for %s", flow.name)
	otg.StartTraffic(t)

	defer otgutils.LogFlowMetrics(t, otg, top)
	defer otg.StopTraffic(t)

	halfTime := trafficTime / 2
	for _, reconnect := range []bool{true, false} {
		t.Logf("Running test with reconnect: %v", reconnect)
		timeout := time.After(halfTime)
		sampleCount := 0
		continueLoop := true
		for continueLoop {
			select {
			case <-timeout:
				if sampleCount == 0 {
					t.Errorf("[ERROR] No samples received from gNPSI")
					return
				}
				if reconnect {
					t.Logf("Received %d samples before reconnect", sampleCount)
					stream.CloseSend()
					stream, err = gnpsiclient.Subscribe(t.Context(), &gnpsi.Request{})
					if err != nil {
						t.Fatalf("Failed to reconnect to gNPSI server: %v", err)
						return
					} else {
						t.Logf("Reconnected to gNPSI server successfully. Resetting sample count.")
						sampleCount = 0
					}
					continueLoop = false
				} else {
					if sampleCount == 0 {
						t.Error("Did not receive any samples after gNPSI reconnect")
					} else {
						t.Logf("Received %d samples after gNPSI reconnect", sampleCount)
					}
					return
				}
			default:
				_, err := stream.Recv()
				if err != nil {
					t.Errorf("[ERROR] Error receiving gNPSI sample: %v", err)
					continue
				}
				sampleCount++
				t.Logf("Received gNPSI sample no %d:", sampleCount)
			}
		}
	}
}

func verifySFlowServiceRestart(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config) {
	otg := ate.OTG()
	flow := flowConfig{
		name:          "Flow_TC4_IPv4_256",
		ipType:        ipv4,
		frameSize:     256,
		packetsToSend: 20000000,
	}

	configureFlow(t, &top, &flow)
	ate.OTG().PushConfig(t, top)
	otg.StartProtocols(t)

	gnpsiclient := dut.RawAPIs().GNPSI(t)
	stream, err := gnpsiclient.Subscribe(t.Context(), &gnpsi.Request{})
	if err != nil {
		t.Fatalf("Failed to connect to gNPSI server: %v", err)
	}

	t.Logf("Starting traffic for %s", flow.name)
	otg.StartTraffic(t)

	defer otgutils.LogFlowMetrics(t, otg, top)
	defer otg.StopTraffic(t)

	sleepTime := 1 * time.Second

	doubleTime := trafficTime * 2
	restartTime := trafficTime / 3
	for _, restarted := range []bool{false, true} {
		t.Logf("Restarted gNPSI: %v", restarted)
		var timeoutTime time.Duration
		if !restarted {
			timeoutTime = restartTime
		} else {
			timeoutTime = doubleTime
		}
		timeout := time.After(timeoutTime)
		sampleCount := 0
		continueLoop := true
		for continueLoop {
			select {
			case <-timeout:
				if !restarted {
					if sampleCount == 0 {
						t.Errorf("[ERROR] No samples received from gNPSI")
						return
					} else {
						t.Logf("Received %d samples, restarting DUT!", sampleCount)
						go restartGnpsiService(t, dut)
						continueLoop = false
					}
				} else if sampleCount == 0 {
					t.Error("Did not receive any samples after gNPSI service restart")
				} else {
					t.Logf("Received %d samples after gNPSI service restart", sampleCount)
				}
				return
			default:
				time.Sleep(sleepTime)
				_, err := stream.Recv()
				if err != nil {
					statusErr, ok := status.FromError(err)
					if ok && statusErr.Code() == codes.Unavailable && strings.Contains(strings.ToUpper(statusErr.Message()), "EOF") {
						t.Logf("gNPSI service is unavailable, trying to reconnect")
						reconnected := false
						for range reconnectRetries {
							stream, err = gnpsiclient.Subscribe(t.Context(), &gnpsi.Request{})
							if err == nil {
								t.Logf("Reconnected to gNPSI server successfully. Resetting sample count.")
								sampleCount = 0
								reconnected = true
								break
							}
							t.Logf("Reconnect failed: %v, retrying in %v...", err, reconnectWaitTime)
							time.Sleep(reconnectWaitTime)
						}
						if !reconnected {
							t.Errorf("[ERROR] Failed to reconnect to gNPSI server after %d attempts", reconnectRetries)
							return
						}
					} else {
						t.Errorf("[ERROR] Error receiving gNPSI sample: %v", err)
						continue
					}
				} else {
					sampleCount++
					t.Logf("Received gNPSI sample no %d:", sampleCount)
				}
			}
		}
	}
}

func restartGnpsiService(t *testing.T, dut *ondatra.DUTDevice) {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		t.Log("Restarting gNPSI service on DUT")
		cliTemplate := `
management api gnpsi
transport grpc test
%s disabled
!
`
		helpers.GnmiCLIConfig(t, dut, fmt.Sprintf(cliTemplate, ""))
		t.Log("gNPSI service disabled")
		time.Sleep(20 * time.Second)
		helpers.GnmiCLIConfig(t, dut, fmt.Sprintf(cliTemplate, "no"))
		t.Log("gNPSI service enabled")

	default:
		t.Errorf("gNPSI service restart not implemented for vendor %s", dut.Vendor())
	}
}

func checkSFlowPackets(t *testing.T, dut *ondatra.DUTDevice, sFlowPackets []sFlowPacket, flowConfig flowConfig) {
	expectedSFlowCount := int(flowConfig.packetsToSend / samplingRate)
	flowCountTolerance := int(float32(expectedSFlowCount)*flowCountTolerancePct) + 1
	if len(sFlowPackets) < expectedSFlowCount-flowCountTolerance || len(sFlowPackets) > expectedSFlowCount+flowCountTolerance {
		t.Errorf("[ERROR] Unexpected number of sFlow packets: got %d, want %d ± %d",
			len(sFlowPackets), expectedSFlowCount, flowCountTolerance)
	} else {
		t.Logf("Received sFlow packets: %d, within expected range %d ± %d ", len(sFlowPackets), expectedSFlowCount, flowCountTolerance)
	}

	for index, sFlowPkt := range sFlowPackets {
		verifySFlowPacket(t, dut, sFlowPkt, flowConfig, index+1)
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
		t.Errorf("[ERROR] SFlow packet size %d does not match expected frame size %d", sFlowPkt.size, flowConfig.frameSize)
	}
	if sFlowPkt.samplingRate != samplingRate {
		t.Errorf("[ERROR] SFlow packet %d: Sampling rate %d does not match expected rate %d", pktIndex, sFlowPkt.samplingRate, samplingRate)
	}
	if sFlowPkt.ingressIntf != ingressIntf {
		t.Errorf("[ERROR] SFlow packet %d: Ingress interface %d does not match expected interface %d", pktIndex, sFlowPkt.ingressIntf, ingressIntf)
	}
	if sFlowPkt.egressIntf != egressIntf {
		t.Errorf("[ERROR] SFlow packet %d: Egress interface %d does not match expected interface %d", pktIndex, sFlowPkt.egressIntf, egressIntf)
	}

	t.Logf("SFlow Packet %d: Size %d, Sampling rate %d, Ingress interface %d, Egress interface %d", pktIndex, flowConfig.frameSize, samplingRate, ingressIntf, egressIntf)
}

func verifyIpv4SFlowSample(t *testing.T, sFlowPkt sFlowPacket) {
	ipLayer := sFlowPkt.packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		t.Errorf("[ERROR] No IPv4 layer found in packet")
		return
	}
	ipv4, ok := ipLayer.(*layers.IPv4)
	if !ok {
		t.Errorf("[ERROR] Failed to extract IPv4 layer")
		return
	}
	initialSrc := net.ParseIP(otgPort1.IPv4)
	initialDst := net.ParseIP(otgPort2.IPv4)
	t.Logf("Source IP: %s, Destination IP: %s", ipv4.SrcIP, ipv4.DstIP)
	if !ipv4.SrcIP.Equal(initialSrc) || !ipv4.DstIP.Equal(initialDst) {
		t.Errorf("[ERROR] IPv4 source or destination IP does not match expected values: got %s -> %s, want %s -> %s",
			ipv4.SrcIP, ipv4.DstIP, otgPort1.IPv4, otgPort2.IPv4)
	}
}

func verifyIpv6SFlowSample(t *testing.T, sFlowPkt sFlowPacket) {
	ipLayer := sFlowPkt.packet.Layer(layers.LayerTypeIPv6)
	if ipLayer == nil {
		t.Errorf("[ERROR] No IPv6 layer found in packet")
		return
	}
	ipv6, ok := ipLayer.(*layers.IPv6)
	if !ok {
		t.Errorf("[ERROR] Failed to extract IPv6 layer")
		return
	}
	initialSrc := net.ParseIP(otgPort1.IPv6)
	initialDst := net.ParseIP(otgPort2.IPv6)
	t.Logf("Source IP: %s, Destination IP: %s", ipv6.SrcIP, ipv6.DstIP)
	if !ipv6.SrcIP.Equal(initialSrc) || !ipv6.DstIP.Equal(initialDst) {
		t.Errorf("[ERROR] IPv6 source or destination IP does not match expected values: got %s -> %s, want %s -> %s",
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
		}
	}
	return 0
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

	t.Log("Configuring SSL Profile")
	configureSSLProfile(t, dut)

	t.Log("Configuring sFlow")
	configureSFlow(t, dut)

	t.Log("Configuring gNPSI")
	configureGNPSI(t, dut)
}

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) {
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	_, ok := gnmi.Watch(t, otg, transmitPath, timeout, func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}).Await(t)

	if !ok {
		t.Errorf("[ERROR] Traffic for flow %s did not stop within the timeout of %d", flowName, timeout)
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

func configureSSLProfile(t *testing.T, dut *ondatra.DUTDevice) {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli := fmt.Sprintf(`
security pki key generate rsa 2048 %s
!
security pki certificate generate self-signed %s key %s parameters common-name localhost
!
		
management security
ssl profile %s
certificate %s key %s
trust certificate %s
tls versions 1.2 1.3
!
`, keyFile, certFile, keyFile, profileName, certFile, keyFile, certFile)
		helpers.GnmiCLIConfig(t, dut, cli)
	default:
		t.Errorf("SSL profile configuration not implemented for vendor %s", dut.Vendor())
	}
}

func configureSFlow(t *testing.T, dut *ondatra.DUTDevice) {
	sfBatch := &gnmi.SetBatch{}
	sFlowConfig := &oc.Sampling_Sflow{
		Enabled:             ygot.Bool(true),
		SampleSize:          ygot.Uint16(sampleSize),
		IngressSamplingRate: ygot.Uint32(samplingRate),
	}
	sFlowConfig.GetOrCreateInterface(dut.Port(t, port1).Name()).Enabled = ygot.Bool(true)
	collectors := cfgplugins.NewSFlowCollector(t, sfBatch, nil, dut, deviations.DefaultNetworkInstance(dut), dutLoopbackAttrs.Name, dutLoopbackAttrs.IPv4, "", ipv4)
	for _, c := range collectors {
		sFlowConfig.AppendCollector(c)
	}
	cfgplugins.NewSFlowGlobalCfg(t, sfBatch, sFlowConfig, dut, deviations.DefaultNetworkInstance(dut), dutLoopbackAttrs.Name, dutLoopbackAttrs.IPv4, "", ipv4)

	sfBatch.Set(t, dut)

	gotSamplingConfig := gnmi.Get(t, dut, gnmi.OC().Sampling().Sflow().Config())
	json, err := ygot.EmitJSON(gotSamplingConfig, &ygot.EmitJSONConfig{
		Format: ygot.RFC7951,
		Indent: "  ",
		RFC7951Config: &ygot.RFC7951JSONConfig{
			AppendModuleName: true,
		},
	})
	if err != nil {
		t.Errorf("[ERROR] Error decoding sampling config: %v", err)
	}
	t.Logf("Got sampling config: %v", json)
}

func configureGNPSI(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.GnpsiOcUnsupported(dut) {
		configureGNPSIFromCLI(t, dut)
	} else {
		configureGNPSIFromOC(t, dut)
	}
}

func configureGNPSIFromOC(t *testing.T, dut *ondatra.DUTDevice) {
	//TODO : Implement gNPSI OC configuration when supported in OC
	t.Fatalf("gNPSI OC configuration is not supported: %s - %s", dut.Version(), dut.Model())
}

func configureGNPSIFromCLI(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Configuring gNPSI from CLI")
	svc := introspect.GNPSI
	dialer := introspect.DUTDialer(t, dut, svc)
	t.Logf("Using gNPSI port: %d", dialer.DevicePort)
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli := fmt.Sprintf(`
management api gnpsi
transport grpc test
ssl profile %s
port %d
source sFlow
no disabled
`, profileName, dialer.DevicePort)
		helpers.GnmiCLIConfig(t, dut, cli)
	default:
		t.Errorf("gNPSI configuration not implemented for vendor %s", dut.Vendor())
	}
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
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(fc.packetsToSend))

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
		t.Errorf("[ERROR] Invalid traffic type %s", fc.ipType)
	}
}
