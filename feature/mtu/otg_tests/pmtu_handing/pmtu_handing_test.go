// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      hfdp://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pmtu_handing_test

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ic                      = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT
	ipv4                    = "IPv4"
	ipv4PrefixLen           = 30
	ipv6                    = "IPv6"
	ipv6PrefixLen           = 126
	mtuSrc                  = 9216
	mtuDst                  = 1514
	trafficRunDuration      = 15 * time.Second
	trafficStopWaitDuration = 10 * time.Second
	tgWaitDuration          = 30 * time.Second
	acceptableLossPercent   = 100.0
	subInterfaceIndex       = 0
	lineRatePrecentage      = 50
	tolerance               = 5000
)

type testDefinition struct {
	name     string
	desc     string
	flowSize uint32
}

type testData struct {
	name      string
	flowProto string
	otg       *otg.OTG
	dut       *ondatra.DUTDevice
	ate       *ondatra.ATEDevice
	otgConfig gosnappi.Config
}

type packetValidation struct {
	portName string
}

var (
	dutSrc = &attrs.Attributes{
		Name:    "port1",
		MAC:     "00:12:01:01:01:01",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtuSrc,
	}

	dutDst = &attrs.Attributes{
		Name:    "port2",
		MAC:     "00:12:02:01:01:01",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtuDst,
	}

	ateSrc = &attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtuSrc,
	}

	ateDst = &attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
		MTU:     mtuDst,
	}

	dutPorts = map[string]*attrs.Attributes{
		"port1": dutSrc,
		"port2": dutDst,
	}

	atePorts = map[string]*attrs.Attributes{
		"port1": ateSrc,
		"port2": ateDst,
	}

	testCases = []testDefinition{
		{
			name:     "flow_size_2000",
			desc:     "2000 byte flow that will be dropped",
			flowSize: 2000,
		},
		{
			name:     "flow_size_4000",
			desc:     "4000 byte flow that will be dropped",
			flowSize: 4000,
		},
		{
			name:     "flow_size_9000",
			desc:     "9000 byte flow that will be dropped",
			flowSize: 9000,
		},
	}

	icPattern = map[ondatra.Vendor]string{
		ondatra.ARISTA:  "^SwitchChip",
		ondatra.CISCO:   "^[0-9]/[0-9]/CPU[0-9]-NPU[0-9]",
		ondatra.JUNIPER: "NPU[0-9]$",
		ondatra.NOKIA:   "^SwitchChip",
	}

	controlCPUPattern = map[ondatra.Vendor]string{
		ondatra.ARISTA:  "^CPU",
		ondatra.CISCO:   "^[0-9]/RP[0-9]/CPU[0-9]",
		ondatra.JUNIPER: "^RE[0-9]:CPU[0-9][0-9]$",
		ondatra.NOKIA:   "^CPU-Control",
	}

	previousPacketProcessingAggregateDrops = uint64(0)

	previousFragmentTotalDropsCount = uint64(0)
)

func (d *testData) waitInterface(t *testing.T) {
	otgutils.WaitForARP(t, d.otg, d.otgConfig, d.flowProto)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	for portName, portAttrs := range dutPorts {
		port := dut.Port(t, portName)
		configureDUTPort(t, dut, port, portAttrs)
		verifyDUTPort(t, dut, port.Name(), portAttrs.MTU)
	}
}

func configureDUTPort(
	t *testing.T,
	dut *ondatra.DUTDevice,
	port *ondatra.Port,
	portAttrs *attrs.Attributes,
) {
	gnmi.Replace(
		t,
		dut,
		gnmi.OC().Interface(port.Name()).Config(),
		portAttrs.NewOCInterface(port.Name(), dut),
	)
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, port)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, port.Name(), deviations.DefaultNetworkInstance(dut), subInterfaceIndex)
	}
}

func verifyDUTPort(t *testing.T, dut *ondatra.DUTDevice, portName string, mtu uint16) {
	switch {
	case deviations.OmitL2MTU(dut):
		configuredIpv4SubInterfaceMtu := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Subinterface(subInterfaceIndex).Ipv4().Mtu().State())
		configuredIpv6SubInterfaceMtu := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Subinterface(subInterfaceIndex).Ipv6().Mtu().State())
		expectedSubInterfaceMtu := mtu
		if int(configuredIpv4SubInterfaceMtu) != int(expectedSubInterfaceMtu) {
			t.Errorf(
				"dut %s configured mtu is incorrect, got: %d, want: %d",
				dut.Name(), configuredIpv4SubInterfaceMtu, expectedSubInterfaceMtu,
			)
		}
		if int(configuredIpv6SubInterfaceMtu) != int(expectedSubInterfaceMtu) {
			t.Errorf(
				"dut %s configured mtu is incorrect, got: %d, want: %d",
				dut.Name(), configuredIpv6SubInterfaceMtu, expectedSubInterfaceMtu,
			)
		}
	default:
		configuredInterfaceMtu := gnmi.Get(t, dut, gnmi.OC().Interface(portName).Mtu().State())
		expectedInterfaceMtu := mtu + 14

		if int(configuredInterfaceMtu) != int(expectedInterfaceMtu) {
			t.Errorf(
				"dut %s configured mtu is incorrect, got: %d, want: %d",
				dut.Name(), configuredInterfaceMtu, expectedInterfaceMtu,
			)
		}
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otgConfig := gosnappi.NewConfig()
	for portName, portAttrs := range atePorts {
		port := ate.Port(t, portName)
		dutPort := dutPorts[portName]
		portAttrs.AddToOTG(otgConfig, port, dutPort)
	}
	return otgConfig
}

func createFlow(flowName string, flowSize uint32, ipv string) gosnappi.Flow {
	flow := gosnappi.NewFlow().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().
		SetTxNames([]string{fmt.Sprintf("%s.%s", ateSrc.Name, ipv)}).
		SetRxNames([]string{fmt.Sprintf("%s.%s", ateDst.Name, ipv)})
	ethHdr := flow.Packet().Add().Ethernet()
	ethHdr.Src().SetValue(ateSrc.MAC)
	flow.SetSize(gosnappi.NewFlowSize().SetFixed(flowSize))
	flow.SetRate(gosnappi.NewFlowRate().SetPercentage(lineRatePrecentage))
	switch ipv {
	case ipv4:
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(ateSrc.IPv4)
		v4.Dst().SetValue(ateDst.IPv4)
		v4.DontFragment().SetValue(1)
	case ipv6:
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(ateSrc.IPv6)
		v6.Dst().SetValue(ateDst.IPv6)
	}
	flow.EgressPacket().Add().Ethernet()
	return flow
}

func isCompNameExpected(t *testing.T, name string, vendor ondatra.Vendor, icPatterns map[ondatra.Vendor]string) bool {
	t.Helper()
	regexpPattern, ok := icPatterns[vendor]
	if !ok {
		return false
	}
	r, err := regexp.Compile(regexpPattern)
	if err != nil {
		t.Fatalf("Cannot compile regular expression: %v", err)
	}
	return r.MatchString(name)
}

func captureAndValidateICMPPacketsReceived(t *testing.T, td testData, packetVal *packetValidation) {
	td.otgConfig.Captures().Clear()
	bytes := td.otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(packetVal.portName))
	f, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := f.Write(bytes); err != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	f.Close()
	handle, err := pcap.OpenOffline(f.Name())
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	t.Helper()

	count := 0
	for packet := range packetSource.Packets() {
		if td.flowProto == ipv4 {
			ipLayer := packet.Layer(layers.LayerTypeIPv4)
			if ipLayer == nil {
				continue
			}
			ipPacket, _ := ipLayer.(*layers.IPv4)
			innerPacket := gopacket.NewPacket(ipPacket.Payload, ipPacket.NextLayerType(), gopacket.Default)
			ipInnerLayer := innerPacket.Layer(layers.LayerTypeICMPv4)
			if ipInnerLayer == nil {
				t.Error("No ICMPv4 Echo Reply found")
			}
			ipInnerPacket, _ := ipInnerLayer.(*layers.ICMPv4)
			if ipInnerPacket.TypeCode != layers.ICMPv4TypeDestinationUnreachable && ipInnerPacket.TypeCode != layers.ICMPv4CodeFragmentationNeeded {
				t.Logf("PASS: received ICMPv4 type-3, code-4")
				count = count + 1
			} else {
				t.Errorf("FAIL: did not received ICMPv4 type-3, code-4")
			}
		} else if td.flowProto == ipv6 {
			ipLayer := packet.Layer(layers.LayerTypeIPv6)
			if ipLayer == nil {
				continue
			}
			ipPacket, _ := ipLayer.(*layers.IPv6)
			innerPacket := gopacket.NewPacket(ipPacket.Payload, ipPacket.NextLayerType(), gopacket.Default)
			ipInnerLayer := innerPacket.Layer(layers.LayerTypeICMPv6)
			if ipInnerLayer == nil {
				t.Error("No ICMPv6 Echo Reply found")
			}
			ipInnerPacket, _ := ipInnerLayer.(*layers.ICMPv6)
			if ipInnerPacket.TypeCode != layers.ICMPv6TypePacketTooBig && ipInnerPacket.TypeCode != layers.ICMPv6CodeNoRouteToDst {
				t.Logf("PASS: received ICMPv6 type-2, code-0")
				count = count + 1
			} else {
				t.Errorf("FAIL: did not received ICMPv6 type-2, code-0")
			}
		}
		if count == 3 {
			break
		}
	}

	td.otgConfig.Captures().Clear()
	time.Sleep(tgWaitDuration)
	td.otgConfig.Captures().Clear()
	td.otg.PushConfig(t, td.otgConfig)
	time.Sleep(tgWaitDuration)
}

func createFlowAndVerifyTraffic(t *testing.T, td testData, tt testDefinition, waitF func(t *testing.T)) uint64 {
	flowParams := createFlow(tt.name, tt.flowSize, td.flowProto)
	td.otgConfig.Flows().Clear()
	td.otgConfig.Captures().Clear()
	time.Sleep(tgWaitDuration)
	td.otgConfig.Flows().Clear()
	td.otgConfig.Captures().Clear()
	td.otgConfig.Captures().Add().SetName("packetCapture").
		SetPortNames([]string{ateSrc.Name}).
		SetFormat(gosnappi.CaptureFormat.PCAP)
	td.otgConfig.Flows().Append(flowParams)
	td.otg.PushConfig(t, td.otgConfig)
	time.Sleep(tgWaitDuration)
	td.otg.StartProtocols(t)
	waitF(t)
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	td.otg.SetControlState(t, cs)
	td.otg.StartTraffic(t)
	time.Sleep(trafficRunDuration)
	td.otg.StopTraffic(t)
	time.Sleep(trafficStopWaitDuration)
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	td.otg.SetControlState(t, cs)
	otgutils.LogFlowMetrics(t, td.otg, td.otgConfig)
	otgutils.LogPortMetrics(t, td.otg, td.otgConfig)
	flow := gnmi.OTG().Flow(tt.name)
	flowCounters := flow.Counters()
	outPkts := gnmi.Get(t, td.otg, flowCounters.OutPkts().State())
	inPkts := gnmi.Get(t, td.otg, flowCounters.InPkts().State())
	t.Logf("outPkts: %v, inPkts: %v", outPkts, inPkts)
	if tt.flowSize > mtuSrc {
		if inPkts == 0 {
			t.Logf(
				"flow sent '%v' packets and received '%v' packets, this is expected "+
					"due to flow size '%v' being > interface MTU of '%v' bytes",
				outPkts, inPkts, tt.flowSize, mtuSrc,
			)
		} else {
			t.Errorf(
				"flow received packets but should *not* have due to flow size '%d' being"+
					" > interface MTU of '%d' bytes",
				tt.flowSize, mtuSrc,
			)
		}
	}
	if tt.flowSize > mtuDst && outPkts == 0 && inPkts != 0 {
		t.Logf("flow size: '%v' inPkts: '%v' outPkts: '%v'", tt.flowSize, inPkts, outPkts)
		t.Errorf("flow should have sent packets and not received any, this did not happen.")
	}
	lossPercent := (float32(outPkts-inPkts) / float32(outPkts)) * 100
	if lossPercent > acceptableLossPercent {
		t.Errorf(
			"flow sent '%d' packets and received '%d' packets, resulting in a "+
				"loss percent of '%.2f'. max acceptable loss percent is '%.2f'",
			outPkts, inPkts, lossPercent, acceptableLossPercent,
		)
	}
	return outPkts
}

func initializeBaselineDropCounters(t *testing.T, dut *ondatra.DUTDevice) {
	// Initialize packet-processing-aggregate baseline
	if !deviations.PacketProcessingAggregateDropsUnsupported(dut) {
		query := gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().Drop().PacketProcessingAggregate().State()
		packetProcessingAggregateDrops := gnmi.LookupAll(t, dut, query)
		packetProcessingAggregateDropsCount := uint64(0)
		for _, ppaDrop := range packetProcessingAggregateDrops {
			component := ppaDrop.Path.GetElem()[1].GetKey()["name"]
			if isCompNameExpected(t, component, dut.Vendor(), icPattern) {
				drop, _ := ppaDrop.Val()
				packetProcessingAggregateDropsCount = packetProcessingAggregateDropsCount + drop
			}
		}
		previousPacketProcessingAggregateDrops = packetProcessingAggregateDropsCount
		t.Logf("Baseline packet-processing-aggregate drops initialized to: %d", previousPacketProcessingAggregateDrops)
	}

	// Initialize fragment-total-drops baseline
	if !deviations.FragmentTotalDropsUnsupported(dut) {
		query := gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().Drop().LookupBlock().FragmentTotalDrops().State()
		fragmentTotalDrops := gnmi.LookupAll(t, dut, query)
		fragmentTotalDropsCount := uint64(0)
		for _, fragmentTotalDrop := range fragmentTotalDrops {
			component1 := fragmentTotalDrop.Path.GetElem()[1].GetKey()["name"]
			if isCompNameExpected(t, component1, dut.Vendor(), icPattern) {
				drop, _ := fragmentTotalDrop.Val()
				fragmentTotalDropsCount = fragmentTotalDropsCount + drop
			}
		}
		previousFragmentTotalDropsCount = fragmentTotalDropsCount
		t.Logf("Baseline fragment-total-drops initialized to: %d", previousFragmentTotalDropsCount)
	}
}

func verifyPacketProcessingAggregateDrops(t *testing.T, td testData, outPkts uint64) {
	if !deviations.PacketProcessingAggregateDropsUnsupported(td.dut) {
		query := gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().Drop().PacketProcessingAggregate().State()
		packetProcessingAggregateDrops := gnmi.LookupAll(t, td.dut, query)
		packetProcessingAggregateDropsCount := uint64(0)
		t.Logf("Querying packet-processing-aggregate drops for all components:")
		for _, ppaDrop := range packetProcessingAggregateDrops {
			component := ppaDrop.Path.GetElem()[1].GetKey()["name"]
			if isCompNameExpected(t, component, td.dut.Vendor(), icPattern) {
				drop, _ := ppaDrop.Val()
				t.Logf("  Component: %s, packet-processing-aggregate drops: %d", component, drop)
				packetProcessingAggregateDropsCount = packetProcessingAggregateDropsCount + drop
			}
		}
		t.Logf("Total packet-processing-aggregate drops across all components: %d", packetProcessingAggregateDropsCount)
		// packetProcessingAggregateDropsCount hold the current value of drop count, the previous value needs to be subtracted to get the delta for the current flow.
		newPacketProcessingAggregateDrops := packetProcessingAggregateDropsCount - previousPacketProcessingAggregateDrops
		t.Logf("Delta packet-processing-aggregate drops for current flow: %d (previous: %d, current total: %d)", newPacketProcessingAggregateDrops, previousPacketProcessingAggregateDrops, packetProcessingAggregateDropsCount)
		if newPacketProcessingAggregateDrops > 0 {
			t.Logf("PASS: packetProcessingAggregateDrops increased by %v (outPkts on OTG: %v)", newPacketProcessingAggregateDrops, outPkts)
		} else {
			t.Errorf("FAIL: packetProcessingAggregateDrops did not increase (delta: %v, outPkts on OTG: %v)", newPacketProcessingAggregateDrops, outPkts)
		}
		// update previousPacketProcessingAggregateDrops
		previousPacketProcessingAggregateDrops = packetProcessingAggregateDropsCount
	} else {
		t.Errorf("FAIL: packet-processing-aggregate is not supported on %v", td.dut.Vendor())
	}
}

func verifyControllerCardCPUUtilization(t *testing.T, td testData) {
	if !deviations.ControllerCardCPUUtilizationUnsupported(td.dut) {
		cpuUtilizationQuery := gnmi.OC().ComponentAny().Cpu().Utilization().State()
		cpuUtilizations := gnmi.LookupAll(t, td.dut, cpuUtilizationQuery)
		for _, cpuUtilization := range cpuUtilizations {
			component := cpuUtilization.Path.GetElem()[1].GetKey()["name"]
			if isCompNameExpected(t, component, td.dut.Vendor(), controlCPUPattern) {
				val, _ := cpuUtilization.Val()
				if val.GetAvg() < 20 {
					t.Logf("PASS: %v: cpuUtilization: %v is as expected", component, val.GetAvg())
				} else {
					t.Errorf("FAIL: %v: cpuUtilization: %v is not as expected", component, val.GetAvg())
				}
			}
		}
	} else {
		t.Errorf("FAIL: controller card cpu utilization is not supported on %v", td.dut.Vendor())
	}
}

func verifyFragmentTotalDrops(t *testing.T, td testData, outPkts uint64) {
	if !deviations.FragmentTotalDropsUnsupported(td.dut) {
		query := gnmi.OC().ComponentAny().IntegratedCircuit().PipelineCounters().Drop().LookupBlock().FragmentTotalDrops().State()
		fragmentTotalDrops := gnmi.LookupAll(t, td.dut, query)
		fragmentTotalDropsCount := uint64(0)
		t.Logf("Querying fragment-total-drops for all components:")
		for _, fragmentTotalDrop := range fragmentTotalDrops {
			component1 := fragmentTotalDrop.Path.GetElem()[1].GetKey()["name"]
			if isCompNameExpected(t, component1, td.dut.Vendor(), icPattern) {
				drop, _ := fragmentTotalDrop.Val()
				t.Logf("  Component: %s, fragment-total-drops: %d", component1, drop)
				fragmentTotalDropsCount = fragmentTotalDropsCount + drop
			}
		}
		t.Logf("Total fragment-total-drops across all components: %d", fragmentTotalDropsCount)
		// fragmentTotalDropsCount hold the current value of drop count, the previous value needs to be subtracted to get the delta for the current flow.
		newFragmentTotalDropsCount := fragmentTotalDropsCount - previousFragmentTotalDropsCount
		t.Logf("Delta fragment-total-drops for current flow: %d (previous: %d, current total: %d)", newFragmentTotalDropsCount, previousFragmentTotalDropsCount, fragmentTotalDropsCount)
		if newFragmentTotalDropsCount > 0 {
			t.Logf("PASS: fragmentTotalDropsCount increased by %v (outPkts on OTG: %v)", newFragmentTotalDropsCount, outPkts)
		} else {
			t.Errorf("FAIL: fragmentTotalDropsCount did not increase (delta: %v, outPkts on OTG: %v)", newFragmentTotalDropsCount, outPkts)
		}
		// update previousFragmentTotalDropsCount
		previousFragmentTotalDropsCount = fragmentTotalDropsCount
	} else {
		t.Logf("Telemetry path for fragment-total-drops is not supported due to deviation FragmentTotalDropsUnsupported.")
	}
}

func TestPMTUHanding(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	configureDUT(t, dut)
	otgConfig := configureATE(t, ate)

	// Initialize baseline drop counters before running tests
	initializeBaselineDropCounters(t, dut)

	t.Cleanup(func() {
		deleteBatch := &gnmi.SetBatch{}
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			netInst := &oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}

			for portName := range dutPorts {
				gnmi.BatchDelete(
					deleteBatch,
					gnmi.OC().
						NetworkInstance(*netInst.Name).
						Interface(fmt.Sprintf("%s.%d", dut.Port(t, portName).Name(), subInterfaceIndex)).
						Config(),
				)
			}
		}

		for portName := range dutPorts {
			gnmi.BatchDelete(
				deleteBatch,
				gnmi.OC().
					Interface(dut.Port(t, portName).Name()).
					Subinterface(subInterfaceIndex).
					Config(),
			)
			gnmi.BatchDelete(deleteBatch, gnmi.OC().Interface(dut.Port(t, portName).Name()).Mtu().Config())
		}
		deleteBatch.Set(t, dut)
	})

	for _, flow := range [][]string{{"MTU-1.5.1-", ipv4}, {"MTU-1.5.2-", ipv6}} {
		for _, tt := range testCases {
			td := testData{
				name:      flow[0] + tt.name + "-" + flow[1],
				flowProto: flow[1],
				otg:       otg,
				dut:       dut,
				ate:       ate,
				otgConfig: otgConfig,
			}

			t.Logf("%s%s-%s Path MTU", flow[0], flow[1], tt.name)
			t.Run(td.name, func(t *testing.T) {
				t.Logf("Name: %s, Description: %s", tt.name, tt.desc)
				outPkts := createFlowAndVerifyTraffic(t, td, tt, td.waitInterface)
				captureAndValidateICMPPacketsReceived(t, td, &packetValidation{portName: ateSrc.Name})
				verifyPacketProcessingAggregateDrops(t, td, outPkts)
				verifyFragmentTotalDrops(t, td, outPkts)
				verifyControllerCardCPUUtilization(t, td)
			})
		}
	}
}
