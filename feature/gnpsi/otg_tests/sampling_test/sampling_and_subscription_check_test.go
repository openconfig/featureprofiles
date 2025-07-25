package sampling_test

import (
	"context"
	"fmt"
	"os"
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
	sampleSize                        = 256
	grpcPort                          = 6070
	trafficTime                       = 30 * time.Second
	IPv4                              = "IPv4"
	IPv6                              = "IPv6"
	samplingRate                      = 1000000
	packetRate                        = 500000
	defaultPacketsToSend       uint32 = 6000000
	flowCountTolerancePct             = 0.5
	samplesThresholdForRestart        = 5
	subscriptionTolerance             = 2
	gnpsiClientsInParallel            = 2
	reconnectRetries                  = 5
	reconnectWaitTime                 = 3 * time.Second
	profileName                       = "gnpsiProf"
	certFile                          = "gnpsi.crt"
	keyFile                           = "gnpsi.key"
)

var (
	// DUT ports
	dutPort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "Dut port 1",
		IPv4:    "192.168.1.1",
		IPv4Len: 30,
		IPv6:    "2001:DB8::1",
		IPv6Len: 126,
	}

	dutPort2 = attrs.Attributes{
		Name:    "port2",
		Desc:    "Dut port 2",
		IPv4:    "192.168.1.5",
		IPv4Len: 30,
		IPv6:    "2001:DB8::5",
		IPv6Len: 126,
	}

	dutlo0Attrs = attrs.Attributes{
		Name:    "Loopback0",
		IPv4:    "192.0.20.2",
		IPv6:    "2001:DB8:0::10",
		IPv4Len: 32,
		IPv6Len: 128,
	}

	// ATE ports
	otgPort1 = attrs.Attributes{
		Desc:    "Otg port 1",
		Name:    "port1",
		MAC:     "00:01:12:00:00:01",
		IPv4:    "192.168.1.2",
		IPv4Len: 30,
		IPv6:    "2001:DB8::2",
		IPv6Len: 126,
	}

	otgPort2 = attrs.Attributes{
		Desc:    "Otg port 2",
		Name:    "port2",
		MAC:     "00:01:12:00:00:02",
		IPv4:    "192.168.1.6",
		IPv4Len: 30,
		IPv6:    "2001:DB8::6",
		IPv6Len: 126,
	}

	adjustedFrameSizeMap = map[uint32]map[string]uint32{
		64: {"IPv4": 66, "IPv6": 86},
	}

	atePortPair = []attrs.Attributes{otgPort1, otgPort2}
)

type sflowPacket struct {
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

	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	grpcAddress := fmt.Sprintf("%s:%d", dut.Name(), grpcPort)
	t.Logf("gNPSI server address: %s", grpcAddress)

	otgPort1.AddToOTG(top, ap1, &dutPort1)
	otgPort2.AddToOTG(top, ap2, &dutPort2)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv6")

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
			name:      "FlowIPv4_64",
			ipType:    IPv4,
			frameSize: 64,
		}, {
			name:      "FlowIPv6_64",
			ipType:    IPv6,
			frameSize: 64,
		}, {
			name:      "FlowIPv4_512",
			ipType:    IPv4,
			frameSize: 512,
		}, {
			name:      "FlowIPv6_512",
			ipType:    IPv6,
			frameSize: 512,
		}, {
			name:      "FlowIPv4_1500",
			ipType:    IPv4,
			frameSize: 1500,
		}, {
			name:      "FlowIPv6_1500",
			ipType:    IPv6,
			frameSize: 1500,
		},
	}

	gnpsiclient := dut.RawAPIs().GNPSI(t)
	stream, err := gnpsiclient.Subscribe(context.Background(), &gnpsi.Request{})
	if err != nil {
		t.Fatalf("Failed to connect to gNPSI server: %v", err)
	}

	wrapSFlowPacket := func(record layers.SFlowRawPacketFlowRecord, flow layers.SFlowFlowSample) sflowPacket {
		return sflowPacket{
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
		sflowPacketsToValidate := []sflowPacket{}
		t.Logf("Starting traffic for %s", fc.name)
		go func() {
			otg.StartTraffic(t)
			waitForTraffic(t, otg, fc.name, trafficTime)
		}()
		time.Sleep(2 * time.Second)
		timeout := time.After(trafficTime)
		continueLoop := true
		for continueLoop {
			select {
			case <-timeout:
				t.Logf("Received %d flow samples", samplesWithFlows)
				if sampleCount == 0 {
					t.Errorf("[ERROR] No samples received from gNPSI")
				} else {
					t.Logf("Total SFlow packets: %d", len(sflowPacketsToValidate))
					checkSFlowPackets(t, dut, sflowPacketsToValidate, fc)
				}
				continueLoop = false
			default:
				resp, err := stream.Recv()
				if err != nil {
					t.Errorf("[ERROR] Error receiving gNPSI sample: %v", err)
					continue
				}
				sampleCount++
				t.Logf("Received gNPSI sample no %d:", sampleCount)
				if len(resp.Packet) > 0 {
					sflow := new(layers.SFlowDatagram)
					err := sflow.DecodeFromBytes(resp.Packet, gopacket.NilDecodeFeedback)
					if err != nil {
						t.Errorf("[ERROR] Failed to decode SFlow packet: %v", err)
						continue
					}
					if len(sflow.FlowSamples) > 0 {
						t.Logf("Found flow samples in this SFlow packet")
						samplesWithFlows++
						for _, flow := range sflow.FlowSamples {
							for _, record := range flow.Records {
								switch r := record.(type) {
								case layers.SFlowRawPacketFlowRecord:
									sflowPacketsToValidate = append(sflowPacketsToValidate, wrapSFlowPacket(r, flow))
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
		otgutils.LogFlowMetrics(t, otg, top)
	}
}

func verifyMultipleSFlowClients(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, top gosnappi.Config) {
	otg := ate.OTG()
	flow := flowConfig{
		name:          "Flow_TC2_IPv4_256",
		ipType:        IPv4,
		frameSize:     256,
		packetsToSend: 10000000,
	}

	configureFlow(t, &top, &flow)
	ate.OTG().PushConfig(t, top)
	otg.StartProtocols(t)

	gnpsiClients := []gnpsi.GNPSI_SubscribeClient{}

	for range gnpsiClientsInParallel {
		gnpsiclient := dut.RawAPIs().GNPSI(t)
		stream, err := gnpsiclient.Subscribe(context.Background(), &gnpsi.Request{})
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
				sflow := new(layers.SFlowDatagram)
				err := sflow.DecodeFromBytes(resp.Packet, gopacket.NilDecodeFeedback)
				if err != nil {
					t.Errorf("[ERROR] Failed to decode SFlow packet: %v", err)
					continue
				}
				if len(sflow.FlowSamples) > 0 {
					t.Logf("Found flow samples in this SFlow packet")
					for _, flow := range sflow.FlowSamples {
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
		ipType:        IPv4,
		frameSize:     256,
		packetsToSend: 20000000,
	}

	configureFlow(t, &top, &flow)
	ate.OTG().PushConfig(t, top)
	otg.StartProtocols(t)

	gnpsiclient := dut.RawAPIs().GNPSI(t)
	stream, err := gnpsiclient.Subscribe(context.Background(), &gnpsi.Request{})
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
					stream, err = gnpsiclient.Subscribe(context.Background(), &gnpsi.Request{})
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
		ipType:        IPv4,
		frameSize:     256,
		packetsToSend: 20000000,
	}

	configureFlow(t, &top, &flow)
	ate.OTG().PushConfig(t, top)
	otg.StartProtocols(t)

	gnpsiclient := dut.RawAPIs().GNPSI(t)
	stream, err := gnpsiclient.Subscribe(context.Background(), &gnpsi.Request{})
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
							stream, err = gnpsiclient.Subscribe(context.Background(), &gnpsi.Request{})
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

func checkSFlowPackets(t *testing.T, dut *ondatra.DUTDevice, sflowPackets []sflowPacket, flowConfig flowConfig) {
	expectedSFlowCount := int(flowConfig.packetsToSend / samplingRate)
	flowCountTolerance := int(float32(expectedSFlowCount)*flowCountTolerancePct) + 1
	if len(sflowPackets) < expectedSFlowCount-flowCountTolerance || len(sflowPackets) > expectedSFlowCount+flowCountTolerance {
		t.Errorf("[ERROR] Unexpected number of sFlow packets: got %d, want %d ± %d",
			len(sflowPackets), expectedSFlowCount, flowCountTolerance)
	} else {
		t.Logf("Received sFlow packets: %d, within expected range %d ± %d ", len(sflowPackets), expectedSFlowCount, flowCountTolerance)
	}

	processInterfaceNumber := func(dut *ondatra.DUTDevice, intfName string) uint32 {
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

	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	ingressIntf := processInterfaceNumber(dut, dp1.Name())
	egressIntf := processInterfaceNumber(dut, dp2.Name())

	adjustedSize := flowConfig.frameSize
	if adjustedValues, found := adjustedFrameSizeMap[flowConfig.frameSize]; found {
		adjustedSize = adjustedValues[flowConfig.ipType]
	}

	for index, sflowPacket := range sflowPackets {
		if sflowPacket.size != adjustedSize {
			t.Errorf("[ERROR] SFlow packet size %d does not match expected frame size %d", sflowPacket.size, flowConfig.frameSize)
		} else {
			t.Logf("SFlow Packet %d: Size matches expected frame size %d", index+1, flowConfig.frameSize)
		}
		if sflowPacket.samplingRate != samplingRate {
			t.Errorf("[ERROR] SFlow packet %d: Sampling rate %d does not match expected rate %d", index+1, sflowPacket.samplingRate, samplingRate)
		} else {
			t.Logf("SFlow Packet %d: Sampling rate matches expected rate %d", index+1, samplingRate)
		}

		if sflowPacket.ingressIntf != ingressIntf {
			t.Errorf("[ERROR] SFlow packet %d: Ingress interface %d does not match expected interface %d", index+1, sflowPacket.ingressIntf, ingressIntf)
		} else {
			t.Logf("SFlow Packet %d: Ingress interface matches expected interface %d", index+1, ingressIntf)
		}

		if sflowPacket.egressIntf != egressIntf {
			t.Errorf("[ERROR] SFlow packet %d: Egress interface %d does not match expected interface %d", index+1, sflowPacket.egressIntf, egressIntf)
		} else {
			t.Logf("SFlow Packet %d: Egress interface matches expected interface %d", index+1, egressIntf)
		}

		switch flowConfig.ipType {
		case IPv4:
			ipLayer := sflowPacket.packet.Layer(layers.LayerTypeIPv4)
			if ipLayer != nil {
				ipv4 := ipLayer.(*layers.IPv4)
				t.Logf("Source IP: %s, Destination IP: %s", ipv4.SrcIP, ipv4.DstIP)
				if ipv4.SrcIP.String() != atePortPair[0].IPv4 || ipv4.DstIP.String() != atePortPair[1].IPv4 {
					t.Errorf("[ERROR] IPv4 source or destination IP does not match expected values: got %s -> %s, want %s -> %s",
						ipv4.SrcIP, ipv4.DstIP, atePortPair[0].IPv4, atePortPair[1].IPv4)
				} else {
					t.Logf("Sflow Packet %d: IPv4 source and destination IP match expected values", index+1)
				}

			} else {
				t.Errorf("[ERROR] No IPv4 layer found in packet")
			}
		case IPv6:
			ipLayer := sflowPacket.packet.Layer(layers.LayerTypeIPv6)
			if ipLayer != nil {
				ipv6 := ipLayer.(*layers.IPv6)
				t.Logf("Source IP: %s, Destination IP: %s", ipv6.SrcIP, ipv6.DstIP)
				if !strings.EqualFold(ipv6.SrcIP.String(), atePortPair[0].IPv6) || !strings.EqualFold(ipv6.DstIP.String(), atePortPair[1].IPv6) {
					t.Errorf("[ERROR] IPv6 source or destination IP does not match expected values: got %s -> %s, want %s -> %s",
						ipv6.SrcIP, ipv6.DstIP, atePortPair[0].IPv6, atePortPair[1].IPv6)
				} else {
					t.Logf("Sflow Packet %d: IPv6 source and destination IP match expected values", index+1)
				}
			} else {
				t.Errorf("[ERROR] No IPv6 layer found in packet")
			}
		}
	}
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

	t.Log("Configuring SSL Profile")
	configureSslProfile(t, dut, false)

	t.Log("Configuring sFlow")
	configureSFlow(t, dut)

	t.Log("Configuring gNPSI")
	configureGnpsi(t, dut)

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
	dutlo0Attrs.Name = loopbackIntfName
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1 := dutlo0Attrs.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, dc.Interface(loopbackIntfName).Config(), loop1)
	} else {
		v4, ok := ipv4Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv4 = v4.GetIp()
		}
		v6, ok := ipv6Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutlo0Attrs.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutlo0Attrs.IPv6)
	}
}

func configureSslProfile(t *testing.T, dut *ondatra.DUTDevice, getCertificates bool) {
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

		if getCertificates {
			certPath := "/persist/secure/ssl/certs/"
			keyPath := "/persist/secure/ssl/keys/"

			getCertDataCommand := fmt.Sprintf("bash cat %s%s", certPath, certFile)
			data := runCliCommand(t, dut, getCertDataCommand)
			if data != "" {
				os.WriteFile(certFile, []byte(data), 0644)
			}

			getKeyDataCommand := fmt.Sprintf("bash cat %s%s", keyPath, keyFile)
			data = runCliCommand(t, dut, getKeyDataCommand)
			if data != "" {
				os.WriteFile(keyFile, []byte(data), 0644)
			}
		}
	default:
		t.Errorf("SSL profile configuration not implemented for vendor %s", dut.Vendor())
	}
}

func configureSFlow(t *testing.T, dut *ondatra.DUTDevice) {
	sfBatch := &gnmi.SetBatch{}

	sflowConfig := new(oc.Sampling_Sflow)

	sflowConfig.Enabled = ygot.Bool(true)
	sflowConfig.SampleSize = ygot.Uint16(256)
	sflowConfig.IngressSamplingRate = ygot.Uint32(samplingRate)

	sflowConfig.GetOrCreateInterface(dut.Port(t, "port1").Name()).Enabled = ygot.Bool(true)
	collectors := cfgplugins.NewSFlowCollector(t, sfBatch, nil, dut, deviations.DefaultNetworkInstance(dut), dutlo0Attrs.Name, dutlo0Attrs.IPv4, "", IPv4)
	for _, c := range collectors {
		sflowConfig.AppendCollector(c)
	}
	cfgplugins.NewSFlowGlobalCfg(t, sfBatch, sflowConfig, dut, deviations.DefaultNetworkInstance(dut), dutlo0Attrs.Name, dutlo0Attrs.IPv4, "", IPv4)

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

func configureGnpsi(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.GnpsiOcUnsupported(dut) {
		configureGnpsiFromCLI(t, dut)
	} else {
		configureGnpsiFromOC(t, dut)
	}
}

func configureGnpsiFromOC(t *testing.T, dut *ondatra.DUTDevice) {
	//ToDo : Implement gNPSI OC configuration when suported in OC
	t.Fatalf("gNPSI OC configuration is not supported: %s - %s", dut.Version(), dut.Model())
}

func configureGnpsiFromCLI(t *testing.T, dut *ondatra.DUTDevice) {
	t.Log("Configuring gNPSI from CLI")
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli := fmt.Sprintf(`
management api gnpsi
transport grpc test
ssl profile %s
port %d
source sflow
no disabled
`, profileName, grpcPort)
		helpers.GnmiCLIConfig(t, dut, cli)
	default:
		t.Errorf("gNPSI configuration not implemented for vendor %s", dut.Vendor())
	}
}

func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, attrs *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	d := gnmi.OC()
	i := attrs.NewOCInterface(p.Name(), dut)
	i.Description = ygot.String(attrs.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	i.GetOrCreateEthernet()
	i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	i4.Enabled = ygot.Bool(true)
	a := i4.GetOrCreateAddress(attrs.IPv4)
	a.PrefixLength = ygot.Uint8(attrs.IPv4Len)

	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	i6.Enabled = ygot.Bool(true)
	a6 := i6.GetOrCreateAddress(attrs.IPv6)
	a6.PrefixLength = ygot.Uint8(attrs.IPv6Len)

	gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), i)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
		t.Logf("DUT %s %s %s requires explicit interface in default VRF deviation ", dut.Vendor(), dut.Model(), dut.Version())
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p)
	}
}

func configureFlow(t *testing.T, config *gosnappi.Config, fc *flowConfig) {
	(*config).Flows().Clear()
	flow := (*config).Flows().Add().SetName(fc.name)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.%s", atePortPair[0].Name, fc.ipType)}).SetRxNames([]string{fmt.Sprintf("%s.%s", atePortPair[1].Name, fc.ipType)})
	flow.Size().SetFixed(fc.frameSize)
	flow.Rate().SetPps(packetRate)
	if fc.packetsToSend == 0 {
		fc.packetsToSend = defaultPacketsToSend
	}
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(fc.packetsToSend))

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(atePortPair[0].MAC)

	switch fc.ipType {
	case IPv4:
		ipv4 := flow.Packet().Add().Ipv4()
		ipv4.Src().SetValue(atePortPair[0].IPv4)
		ipv4.Dst().SetValue(atePortPair[1].IPv4)
	case IPv6:
		ipv6 := flow.Packet().Add().Ipv6()
		ipv6.Src().SetValue(atePortPair[0].IPv6)
		ipv6.Dst().SetValue(atePortPair[1].IPv6)
	default:
		t.Errorf("[ERROR] Invalid traffic type %s", fc.ipType)
	}
}

func runCliCommand(t *testing.T, dut *ondatra.DUTDevice, cliCommand string) string {
	cliClient := dut.RawAPIs().CLI(t)
	output, err := cliClient.RunCommand(context.Background(), cliCommand)
	if err != nil {
		t.Errorf("[ERROR] Failed to execute CLI command '%s': %v", cliCommand, err)
		return ""
	}
	t.Logf("Received from cli: %s", output.Output())
	return output.Output()
}
