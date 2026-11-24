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
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4                    = "IPv4"
	ipv4PrefixLen           = 30
	ipv6                    = "IPv6"
	ipv6PrefixLen           = 126
	mtuSrc                  = 9216
	mtuDst                  = 1514
	trafficRunDuration      = 15 * time.Second
	trafficStopWaitDuration = 10 * time.Second
	tgWaitDuration          = 30 * time.Second
	subInterfaceIndex       = 0
	lineRatePrecentage      = 50
)

type testDefinition struct {
	name     string
	desc     string
	flowSize uint32
}

type testData struct {
	name         string
	flowProto    string
	otg          *otg.OTG
	dut          *ondatra.DUTDevice
	ate          *ondatra.ATEDevice
	otgConfig    gosnappi.Config
	preConfigCPU map[string]uint8
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
		// Give the PMTU probe a defined upper-layer header instead of the
		// generator's default IPv6 No Next Header payload.
		flow.Packet().Add().Udp()
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
	t.Helper()
	data := td.otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(packetVal.portName))
	t.Logf("Capture %s: %d bytes", packetVal.portName, len(data))
	if dir := os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR"); dir != "" {
		path := filepath.Join(dir, td.name+".pcap")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Errorf("Save capture: %v", err)
		} else {
			t.Logf("Saved capture: %s", path)
		}
	}
	if err := validatePMTUCapture(data, td.flowProto, ateSrc.IPv4, ateSrc.IPv6); err != nil {
		t.Error(err)
	}
}

func createFlowAndVerifyTraffic(t *testing.T, td testData, tt testDefinition, waitF func(t *testing.T)) {
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
	cs := otgutils.StartCapture(t, td.otg)
	defer otgutils.StopCapture(t, td.otg, cs)
	baselineCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	baseline := readPacketProcessingAggregateDrops(baselineCtx, t, td.dut)
	cancel()
	td.otg.StartTraffic(t)
	time.Sleep(trafficRunDuration)
	td.otg.StopTraffic(t)
	time.Sleep(trafficStopWaitDuration)
	otgutils.StopCapture(t, td.otg, cs)
	otgutils.LogFlowMetrics(t, td.otg, td.otgConfig)
	otgutils.LogPortMetrics(t, td.otg, td.otgConfig)
	flow := gnmi.OTG().Flow(tt.name)
	flowCounters := flow.Counters()
	outPkts := gnmi.Get(t, td.otg, flowCounters.OutPkts().State())
	inPkts := gnmi.Get(t, td.otg, flowCounters.InPkts().State())
	t.Logf("outPkts: %v, inPkts: %v", outPkts, inPkts)
	if err := validateOversizedTraffic(outPkts, inPkts); err != nil {
		t.Error(err)
	}
	verifyPacketProcessingAggregateDrops(t, td, baseline)
}

func getCPUUtilization(t *testing.T, dut *ondatra.DUTDevice) map[string]uint8 {
	cpuUtilizationQuery := gnmi.OC().ComponentAny().Cpu().Utilization().State()
	cpuUtilizations := gnmi.LookupAll(t, dut, cpuUtilizationQuery)
	res := make(map[string]uint8)
	for _, cpuUtilization := range cpuUtilizations {
		component := cpuUtilization.Path.GetElem()[1].GetKey()["name"]
		if isCompNameExpected(t, component, dut.Vendor(), controlCPUPattern) {
			val, _ := cpuUtilization.Val()
			res[component] = uint8(val.GetAvg())
		}
	}
	return res
}

// readPacketProcessingAggregateDrops requires a present counter on every matching IC.
func readPacketProcessingAggregateDrops(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) map[string]uint64 {
	t.Helper()
	if deviations.PacketProcessingAggregateDropsUnsupported(dut) {
		t.Fatalf("Required packet-processing-aggregate telemetry is unsupported on %v", dut.Vendor())
	}
	client, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		t.Fatalf("Create aggregate-drop client: %v", err)
	}
	values := make(map[string]uint64)
	for _, name := range components.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT) {
		if !isCompNameExpected(t, name, dut.Vendor(), icPattern) {
			continue
		}
		query := gnmi.OC().Component(name).IntegratedCircuit().PipelineCounters().Drop().PacketProcessingAggregate().State()
		result, err := ygnmi.Lookup(ctx, client, query)
		if err != nil {
			t.Fatalf("Read packet-processing-aggregate for %s: %v", name, err)
		}
		value, ok := result.Val()
		if !ok {
			t.Fatalf("Missing packet-processing-aggregate for %s", name)
		}
		values[name] = value
		t.Logf("%s packet-processing-aggregate: %d", name, value)
	}
	if len(values) == 0 {
		t.Fatal("No matching integrated circuits with packet-processing-aggregate telemetry")
	}
	return values
}

func verifyPacketProcessingAggregateDrops(t *testing.T, td testData, baseline map[string]uint64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		current := readPacketProcessingAggregateDrops(ctx, t, td.dut)
		increased, err := aggregateDropsIncreased(baseline, current)
		if err != nil {
			t.Fatalf("Invalid aggregate-drop telemetry: %v", err)
		}
		if increased {
			for name, before := range baseline {
				t.Logf("%s aggregate-drop delta: %d", name, current[name]-before)
			}
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("packet-processing-aggregate did not increase within 30 seconds after traffic")
		case <-time.After(time.Second):
		}
	}
}

func verifyControllerCardCPUUtilization(t *testing.T, td testData) {
	if !deviations.ControllerCardCPUUtilizationUnsupported(td.dut) {
		currentCPU := getCPUUtilization(t, td.dut)
		for component, postVal := range currentCPU {
			preVal, ok := td.preConfigCPU[component]
			if !ok {
				t.Errorf("FAIL: %v: pre-config cpuUtilization not found", component)
				continue
			}
			if postVal > preVal+10 {
				t.Errorf("FAIL: %v: cpuUtilization increased by more than 10%%, pre: %v, post: %v", component, preVal, postVal)
			} else {
				t.Logf("PASS: %v: cpuUtilization increase within 10%%, pre: %v, post: %v", component, preVal, postVal)
			}
		}
	} else {
		t.Logf("Telemetry path for controller card cpu utilization is not supported due to deviation ControllerCardCPUUtilizationUnsupported.")
	}
}

func TestPMTUHanding(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	// Capture baseline CPU utilization before configuration
	preConfigCPU := getCPUUtilization(t, dut)

	configureDUT(t, dut)
	otgConfig := configureATE(t, ate)

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
				name:         flow[0] + tt.name + "-" + flow[1],
				flowProto:    flow[1],
				otg:          otg,
				dut:          dut,
				ate:          ate,
				otgConfig:    otgConfig,
				preConfigCPU: preConfigCPU,
			}

			t.Logf("%s%s-%s Path MTU", flow[0], flow[1], tt.name)
			t.Run(td.name, func(t *testing.T) {
				t.Logf("Name: %s, Description: %s", tt.name, tt.desc)
				createFlowAndVerifyTraffic(t, td, tt, td.waitInterface)
				captureAndValidateICMPPacketsReceived(t, td, &packetValidation{portName: ateSrc.Name})
				verifyControllerCardCPUUtilization(t, td)
			})
		}
	}
}

func validateOversizedTraffic(sent, received uint64) error {
	if sent == 0 {
		return fmt.Errorf("oversized flow transmitted no packets")
	}
	if received != 0 {
		return fmt.Errorf("oversized flow forwarded %d of %d packets; want zero received", received, sent)
	}
	return nil
}

// aggregateDropsIncreased checks every component before accepting any increase.
// Comparing before subtraction avoids unsigned underflow on counter resets.
func aggregateDropsIncreased(before, after map[string]uint64) (bool, error) {
	if len(before) == 0 {
		return false, fmt.Errorf("empty aggregate-drop baseline")
	}
	if len(before) != len(after) {
		return false, fmt.Errorf("aggregate-drop component set changed")
	}
	increased := false
	for name, initial := range before {
		current, ok := after[name]
		if !ok {
			return false, fmt.Errorf("missing aggregate-drop component %s", name)
		}
		if current < initial {
			return false, fmt.Errorf("%s counter decreased from %d to %d", name, initial, current)
		}
		increased = increased || current > initial
	}
	return increased, nil
}

func matchesPMTUResponse(packet gopacket.Packet, protocol, sourceIPv4, sourceIPv6 string) bool {
	switch protocol {
	case "IPv4":
		ip, ok := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
		if !ok || !ip.DstIP.Equal(net.ParseIP(sourceIPv4)) {
			return false
		}
		icmp, ok := packet.Layer(layers.LayerTypeICMPv4).(*layers.ICMPv4)
		return ok && icmp.TypeCode.Type() == layers.ICMPv4TypeDestinationUnreachable && icmp.TypeCode.Code() == layers.ICMPv4CodeFragmentationNeeded
	case "IPv6":
		ip, ok := packet.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
		if !ok || !ip.DstIP.Equal(net.ParseIP(sourceIPv6)) {
			return false
		}
		icmp, ok := packet.Layer(layers.LayerTypeICMPv6).(*layers.ICMPv6)
		return ok && icmp.TypeCode.Type() == layers.ICMPv6TypePacketTooBig && icmp.TypeCode.Code() == 0
	}
	return false
}

// validatePMTUCapture parses PCAP in memory, avoiding temporary files and libpcap.
func validatePMTUCapture(data []byte, protocol, sourceIPv4, sourceIPv6 string) error {
	if len(data) == 0 {
		return fmt.Errorf("OTG returned an empty capture (0 bytes); no ICMP response can be validated")
	}
	reader, err := pcapgo.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("open PMTU capture: %w", err)
	}
	source := gopacket.NewPacketSource(reader, reader.LinkType())
	packets, ipv6Packets, icmpv6Packets := 0, 0, 0
	for {
		packet, err := source.NextPacket()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read PMTU capture: %w", err)
		}
		packets++
		if packet.Layer(layers.LayerTypeIPv6) != nil {
			ipv6Packets++
		}
		if packet.Layer(layers.LayerTypeICMPv6) != nil {
			icmpv6Packets++
		}
		if matchesPMTUResponse(packet, protocol, sourceIPv4, sourceIPv6) {
			return nil
		}
	}
	return fmt.Errorf("no matching %s PMTU ICMP response addressed to the traffic source (captured packets: %d, IPv6: %d, ICMPv6: %d)", protocol, packets, ipv6Packets, icmpv6Packets)
}
