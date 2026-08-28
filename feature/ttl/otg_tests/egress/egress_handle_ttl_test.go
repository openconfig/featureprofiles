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

package egress_handle_ttl_test

import (
	"os"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
)

const (
	ipv4PrefixLen    = 30
	ipv6PrefixLen    = 126
	ipv4Decap        = "10.2.2.2"
	greDecapGrpName  = "GRE-DECAP"
	frameSize        = 128
	packetsPerFlow   = 5
	pps              = 100   // Packets per second
	mplsLabelV4      = 99910 // Static Mpls currently supported range 16 - 99999 (99910 instead of 100010)
	mplsLabelV6      = 99920 // Static Mpls currently supported range 16 - 99999 (99920 instead of 100020)
	sleepTime        = 20
	flowname         = "trafficItem"
	tolerance        = 2
	udpDecapPort     = 6080 // Default UDP destination port for GUE-like decapsulation
	udpDecapPortIPv4 = 6080 // UDP destination port for IPv4 payload GUE decapsulation
	udpDecapPortIPv6 = 6081 // UDP destination port for IPv6 payload GUE decapsulation
	udpDecapPortMPLS = 6082 // UDP destination port for MPLS payload GUE decapsulation
	defaultNI        = "DEFAULT"
	policyName       = "decap-policy"
	policyId         = 1
	lspName1         = "lsp1"
	lspName2         = "lsp2"
)

var (
	// DUT port attributes
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    "192.168.10.2",
		IPv6:    "2001:db8::192:168:10:2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT Port 2",
		IPv4:    "192.168.20.2",
		IPv6:    "2001:db8::192:168:20:2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	// ATE port attributes
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.168.10.1",
		IPv6:    "2001:db8::192:168:10:1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.168.20.1",
		IPv6:    "2001:db8::192:168:20:1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	expectedTTL1 = 9
)

type flowArgs struct {
	flowName                   string
	outerSrcIP, outerDstIP     string
	InnerSrcIP, InnerDstIP     string
	InnerSrcIPv6, InnerDstIPv6 string
	ipv4Flow                   bool
	outerIpv4Ttl               int
	outerIpv6Ttl               int
}

type packetValidation struct {
	portName         string
	outerDstIP       string
	innerDstIP       string
	validateDecap    bool
	validateNonEncap bool
	outerTtl         int
	innerTtl         int
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestEgressHandleTTL(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	config := configureATE(t, ate)
	otgConfig := ate.OTG()
	sfBatch := &gnmi.SetBatch{}
	// Configure Static Route: MPLS label binding
	cfgplugins.MPLSStaticLSPByPass(t, sfBatch, dut, lspName1, mplsLabelV4, atePort2.IPv4, "ipv4", true)
	cfgplugins.MPLSStaticLSPByPass(t, sfBatch, dut, lspName2, mplsLabelV6, atePort2.IPv6, "ipv6", true)
	sfBatch.Set(t, dut)
	// Policy Based Forwading Rule-1
	cfgplugins.NewConfigureGRETunnel(t, dut, ipv4Decap, greDecapGrpName)

	// Test cases.
	type testCase struct {
		Name        string
		Description string
		InnerTTL    int
		OuterTTL    int
		MplsTTL     int
		TestFunc    func(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int)
	}

	testCases := []testCase{
		{
			Name:        "Testcase-IPv4NonEncapsulatedTraffic",
			Description: "IPv4 non-encapsulated traffic with TTL = 10",
			InnerTTL:    10,
			TestFunc:    createIPv4Flow,
		},
		{
			Name:        "Testcase-IPv6NonEncapsulatedTraffic",
			Description: "PF-1.9.2: IPv6 non-encapsulated traffic with TTL = 10",
			InnerTTL:    10,
			TestFunc:    createIPv6Flow,
		},
		{
			Name:        "Testcase-NegIPv4NonEncapsulatedTrafficTTL",
			Description: "PF-1.9.3: IPv4 non-encapsulated traffic with TTL = 1",
			InnerTTL:    1,
			TestFunc:    createIPv4Flow,
		},
		{
			Name:        "Testcase-NegIPv6NonEncapsulatedTrafficTTL",
			Description: "PF-1.9.4: IPv6 non-encapsulated traffic with TTL = 1",
			InnerTTL:    1,
			TestFunc:    createIPv6Flow,
		},
		{
			Name:        "Testcase-IPv4oGreDecapsulatedTrafficTTL",
			Description: "PF-1.9.5: IPv4oGRE traffic with inner TTL = 10 and outer TTL = 30",
			InnerTTL:    10,
			OuterTTL:    30,
			TestFunc:    createIPv4oGREFlow,
		},
		{
			Name:        "Testcase-IPv6oGreDecapsulatedTrafficTTL",
			Description: "PF-1.9.6: IPv6oGRE traffic with inner TTL = 10 and outer TTL = 30",
			InnerTTL:    10,
			OuterTTL:    30,
			TestFunc:    createIPv6oGREFlow,
		},
		{
			Name:        "Testcase-InnerIPv4oGreDecapsulatedTrafficTTL",
			Description: "PF-1.9.7: IPv4oGRE traffic with inner TTL = 1 and outer TTL = 30",
			InnerTTL:    1,
			OuterTTL:    30,
			TestFunc:    createIPv4oGREFlow,
		},
		{
			Name:        "Testcase-InnerIPv6oGreDecapsulatedTrafficTTL",
			Description: "PF-1.9.8: IPv6oGRE traffic with inner TTL = 1 and outer TTL = 30",
			InnerTTL:    1,
			OuterTTL:    30,
			TestFunc:    createIPv6oGREFlow,
		},
		{
			Name:        "Testcase-OuterIPv4oGreDecapsulatedTrafficTTL",
			Description: "PF-1.9.9: IPv4oGRE traffic with inner TTL = 10 and outer TTL = 1",
			InnerTTL:    10,
			OuterTTL:    1,
			TestFunc:    createIPv4oGREFlow,
		},
		{
			Name:        "Testcase-OuterIPv6oGreDecapsulatedTrafficTTL",
			Description: "PF-1.9.10: IPv6oGRE traffic with inner TTL = 10 and outer TTL = 1",
			InnerTTL:    10,
			OuterTTL:    1,
			TestFunc:    createIPv6oGREFlow,
		},
		{
			Name:        "Testcase-IPv4oMPLSoGRETest",
			Description: "PF-1.9.11: IPv4oMPLSoGRE traffic with inner TTL = 10, MPLS TTL = 20 and outer TTL = 30",
			InnerTTL:    10,
			MplsTTL:     20,
			OuterTTL:    30,
			TestFunc:    createIPv4oMPLSoGREFlow,
		},
		{
			Name:        "Testcase-IPv6oMPLSoGRETest",
			Description: "PF-1.9.12: IPv6oMPLSoGRE traffic with inner TTL = 10, MPLS TTL = 20 and outer TTL = 30",
			InnerTTL:    10,
			MplsTTL:     20,
			OuterTTL:    30,
			TestFunc:    createIPv6oMPLSoGREFlow,
		},
		{
			Name:        "Testcase-IPv4oMPLSoGREInnerTest",
			Description: "PF-1.9.13: IPv4oMPLSoGRE traffic with inner TTL = 1, MPLS TTL = 20 and outer TTL = 30",
			InnerTTL:    1,
			MplsTTL:     20,
			OuterTTL:    30,
			TestFunc:    createIPv4oMPLSoGREFlow,
		},
		{
			Name:        "Testcase-IPv6oMPLSoGREInnerTest",
			Description: "PF-1.9.14: IPv6oMPLSoGRE traffic with inner TTL = 1, MPLS TTL = 20 and outer TTL = 30",
			InnerTTL:    1,
			MplsTTL:     20,
			OuterTTL:    30,
			TestFunc:    createIPv6oMPLSoGREFlow,
		},
		{
			Name:        "Testcase-IPv4oMPLSoGREMplsTest",
			Description: "PF-1.9.15: IPv4oMPLSoGRE traffic with inner TTL = 10, MPLS TTL = 1 and outer TTL = 30",
			InnerTTL:    10,
			MplsTTL:     1,
			OuterTTL:    30,
			TestFunc:    createIPv4oMPLSoGREFlow,
		},
		{
			Name:        "Testcase-IPv6oMPLSoGREMplsTest",
			Description: "PF-1.9.16: IPv6oMPLSoGRE traffic with inner TTL = 10, MPLS TTL = 1 and outer TTL = 30",
			InnerTTL:    10,
			MplsTTL:     1,
			OuterTTL:    30,
			TestFunc:    createIPv6oMPLSoGREFlow,
		},
		{
			Name:        "Testcase-IPv4oUDPTest",
			Description: "PF-1.9.17: IPv4oUDP traffic with inner TTL = 10 and outer TTL = 30",
			InnerTTL:    10,
			OuterTTL:    30,
			TestFunc:    createIPv4oUDPFlow,
		},
		{
			Name:        "Testcase-IPv6oUDPTest",
			Description: "PF-1.9.18: IPv6oUDP traffic with inner TTL = 10 and outer TTL = 30",
			InnerTTL:    10,
			OuterTTL:    30,
			TestFunc:    createIPv6oUDPFlow,
		},
		{
			Name:        "Testcase-IPv4oUDPInnerTest",
			Description: "PF-1.9.19: IPv4oUDP traffic with inner TTL = 1 and outer TTL = 30",
			InnerTTL:    1,
			OuterTTL:    30,
			TestFunc:    createIPv4oUDPFlow,
		},
		{
			Name:        "Testcase-IPv6oUDPInnerTest",
			Description: "PF-1.9.20: IPv6oUDP traffic with inner TTL = 1 and outer TTL = 30",
			InnerTTL:    1,
			OuterTTL:    30,
			TestFunc:    createIPv6oUDPFlow,
		},
		{
			Name:        "Testcase-IPv4oUDPOuterTest",
			Description: "PF-1.9.21: IPv4oUDP traffic with inner TTL = 10 and outer TTL = 1",
			InnerTTL:    10,
			OuterTTL:    1,
			TestFunc:    createIPv4oUDPFlow,
		},
		{
			Name:        "Testcase-IPv6oUDPOuterTest",
			Description: "PF-1.9.22: IPv6oUDP traffic with inner TTL = 10 and outer TTL = 1",
			InnerTTL:    10,
			OuterTTL:    1,
			TestFunc:    createIPv6oUDPFlow,
		},
		{
			Name:        "Testcase-IPv4oMPLSoUDPTest",
			Description: "PF-1.9.23: IPv4oMPLSoUDP traffic with inner TTL = 10, MPLS TTL = 20 and outer TTL = 30",
			InnerTTL:    10,
			MplsTTL:     20,
			OuterTTL:    30,
			TestFunc:    createIPv4oMPLSoUDPFlow,
		},
		{
			Name:        "Testcase-IPv6oMPLSoUDPTest",
			Description: "PF-1.9.24: IPv6oMPLSoUDP traffic with inner TTL = 10, MPLS TTL = 20 and outer TTL = 30",
			InnerTTL:    10,
			MplsTTL:     20,
			OuterTTL:    30,
			TestFunc:    createIPv6oMPLSoUDPFlow,
		},
		{
			Name:        "Testcase-IPv4oMPLSoUDPInnerTest",
			Description: "PF-1.9.25: IPv4oMPLSoUDP traffic with inner TTL = 1, MPLS TTL = 20 and outer TTL = 30",
			InnerTTL:    1,
			MplsTTL:     20,
			OuterTTL:    30,
			TestFunc:    createIPv4oMPLSoUDPFlow,
		},
		{
			Name:        "Testcase-IPv6oMPLSoUDPInnerTest",
			Description: "PF-1.9.26: IPv6oMPLSoUDP traffic with inner TTL = 1, MPLS TTL = 20 and outer TTL = 30",
			InnerTTL:    1,
			MplsTTL:     20,
			OuterTTL:    30,
			TestFunc:    createIPv6oMPLSoUDPFlow,
		},
		{
			Name:        "Testcase-IPv4oMPLSoUDPMplsTest",
			Description: "PF-1.9.27: IPv4oMPLSoUDP traffic with inner TTL = 10, MPLS TTL = 1 and outer TTL = 30",
			InnerTTL:    10,
			MplsTTL:     1,
			OuterTTL:    30,
			TestFunc:    createIPv4oMPLSoUDPFlow,
		},
		{
			Name:        "Testcase-IPv6oMPLSoUDPMplsTest",
			Description: "PF-1.9.28: IPv6oMPLSoUDP traffic with inner TTL = 10, MPLS TTL = 1 and outer TTL = 30",
			InnerTTL:    10,
			MplsTTL:     1,
			OuterTTL:    30,
			TestFunc:    createIPv6oMPLSoUDPFlow,
		},
	}

	// Run the test cases.
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("Description: %s", tc.Description)
			tc.TestFunc(t, dut, otgConfig, config, tc.InnerTTL, tc.OuterTTL, tc.MplsTTL)
		})
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	configureDUTPort(t, dut, &dutPort1, p1)
	configureDUTPort(t, dut, &dutPort2, p2)

	fptest.ConfigureDefaultNetworkInstance(t, dut)
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
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		i4.Enabled = ygot.Bool(true)
	}
	a := i4.GetOrCreateAddress(attrs.IPv4)
	a.PrefixLength = ygot.Uint8(attrs.IPv4Len)

	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		i6.Enabled = ygot.Bool(true)
	}
	a6 := i6.GetOrCreateAddress(attrs.IPv6)
	a6.PrefixLength = ygot.Uint8(attrs.IPv6Len)

	gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), i)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p)
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	t.Log("Configure ATE interfaces")

	config := gosnappi.NewConfig()

	atePort1.AddToOTG(config, ate.Port(t, "port1"), &dutPort1)
	atePort2.AddToOTG(config, ate.Port(t, "port2"), &dutPort2)

	return config
}

func addFlow(t *testing.T, config gosnappi.Config, flowValues *flowArgs) gosnappi.Flow {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	flow := gosnappi.NewFlow().SetName(flowValues.flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName(config.Ports().Items()[0].Name()).SetRxNames([]string{config.Ports().Items()[1].Name()})
	flow.Size().SetFixed(frameSize)
	flow.Duration().FixedPackets().SetPackets(packetsPerFlow)
	flow.Rate().SetPps(pps)
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	ethHeader.Dst().SetValue(macAddress)
	if flowValues.ipv4Flow {
		ipv4Header := flow.Packet().Add().Ipv4()
		ipv4Header.Src().SetValue(flowValues.outerSrcIP)
		ipv4Header.Dst().SetValue(flowValues.outerDstIP)
		ipv4Header.TimeToLive().SetValue(uint32(flowValues.outerIpv4Ttl))
	} else {
		ipv6Header := flow.Packet().Add().Ipv6()
		ipv6Header.Src().SetValue(flowValues.outerSrcIP)
		ipv6Header.Dst().SetValue(flowValues.outerDstIP)
		ipv6Header.HopLimit().SetValue(uint32(flowValues.outerIpv6Ttl))
	}

	return flow
}

// verifyTrafficFlow verify the each flow on ATE
func verifyTrafficFlow(t *testing.T, otgConfig *otg.OTG, config gosnappi.Config, flow gosnappi.Flow) bool {
	t.Helper()
	otgutils.LogFlowMetrics(t, otgConfig, config)
	rxPkts := gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State())
	txPkts := gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State())
	if txPkts == 0 {
		t.Fatalf("txPkts == %d, want > 0.", txPkts)
	}
	lostPkt := txPkts - rxPkts
	if got := (lostPkt * 100 / txPkts); got >= tolerance {
		return false
	}
	return true
}

func captureAndValidatePackets(t *testing.T, otgConfig *otg.OTG, packetVal *packetValidation, protocolType string) {
	t.Helper()
	packetCaptureGRE := processCapture(t, otgConfig, "port2")
	defer os.Remove(packetCaptureGRE)
	handle, err := pcap.OpenOffline(packetCaptureGRE)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	if packetVal.validateDecap {
		validateTrafficDecap(t, packetSource, packetVal.innerDstIP, packetVal.innerTtl, protocolType)
	}
	if packetVal.validateNonEncap {
		validateTrafficNonEncap(t, packetSource, packetVal.outerDstIP, packetVal.outerTtl, protocolType)
	}
}

func validateTrafficNonEncap(t *testing.T, packetSource *gopacket.PacketSource, expectedIP string, expectedTTL int, protocol string) {
	t.Helper()
	t.Logf("Validate non-encapsulated traffic for protocol: %s", protocol)
	matched := false
outer:
	for packet := range packetSource.Packets() {
		switch protocol {
		case "ipv4":
			ipLayer := packet.Layer(layers.LayerTypeIPv4)
			if ipLayer == nil {
				continue
			}
			ipPacket := ipLayer.(*layers.IPv4)
			gotTTL := ipPacket.TTL
			gotDstIP := ipPacket.DstIP.String()

			if gotDstIP == expectedIP {
				if int(gotTTL) == expectedTTL {
					t.Logf("Matched IPv4 packet: DstIP = %s, TTL = %d", gotDstIP, gotTTL)
					matched = true
					break outer
				}
				t.Errorf("Failed to match TTL: GotIP = %s, Expected IP = %s, GotTTL = %d, ExpectedTTL = %d", gotDstIP, expectedIP, gotTTL, expectedTTL)
				break outer
			}

		case "ipv6":
			ipLayer := packet.Layer(layers.LayerTypeIPv6)
			if ipLayer == nil {
				continue
			}
			ipPacket := ipLayer.(*layers.IPv6)
			gotHopLimit := ipPacket.HopLimit
			gotDstIP := ipPacket.DstIP.String()
			if gotDstIP == expectedIP {
				if int(gotHopLimit) == expectedTTL {
					t.Logf("Matched IPv6 packet: DstIP = %s, HopLimit = %d", gotDstIP, gotHopLimit)
					matched = true
					break outer
				}
				t.Errorf("Failed to match HopLimit: GotIP = %s, Expected IP = %s, GotHopLimit = %d, ExpectedHopLimit = %d", gotDstIP, expectedIP, gotHopLimit, expectedTTL)
				break outer
			}

		default:
			t.Fatalf("Unsupported protocol type: %s. Must be 'ipv4' or 'ipv6'", protocol)
		}
	}
	if !matched {
		t.Errorf("Did not find expected non-encapsulated packet with DstIP = %s, ExpectedTTL = %d", expectedIP, expectedTTL)
	}
}

func validateTrafficDecap(t *testing.T, packetSource *gopacket.PacketSource, expectedIP string, expectedTTL int, protocol string) {
	t.Helper()
	t.Log("Validating decapsulated traffic: Inner DstIP and TTL/HopLimit")
	matched := false
outer:
	for packet := range packetSource.Packets() {
		switch protocol {
		case "ipv4":
			ipLayer := packet.Layer(layers.LayerTypeIPv4)
			if ipLayer == nil {
				continue
			}
			ipPacket := ipLayer.(*layers.IPv4)
			payload := ipPacket.Payload
			nextLayerType := ipPacket.NextLayerType()

			gotTTL := ipPacket.TTL
			gotDstIP := ipPacket.DstIP.String()
			if gotDstIP == expectedIP {
				if int(gotTTL) == expectedTTL {
					t.Logf("Matched IPv4 packet: DstIP = %s, TTL = %d", gotDstIP, gotTTL)

					// Decode inner packet from outer payload
					innerPacket := gopacket.NewPacket(payload, nextLayerType, gopacket.Default)
					if innerPacket.Layer(layers.LayerTypeIPv4) != nil {
						t.Errorf("Packets are not decapped: inner IPv4 header still present.")
					}
					matched = true
					break outer
				}
				t.Errorf("Failed to match TTL: GotIP = %s, Expected IP = %s, GotTTL = %d, ExpectedTTL = %d", gotDstIP, expectedIP, gotTTL, expectedTTL)
				break outer
			}

		case "ipv6":
			ipLayer := packet.Layer(layers.LayerTypeIPv6)
			if ipLayer == nil {
				continue
			}
			ipPacket := ipLayer.(*layers.IPv6)
			payload := ipPacket.Payload
			nextLayerType := ipPacket.NextLayerType()

			gotHopLimit := ipPacket.HopLimit
			gotDstIP := ipPacket.DstIP.String()

			if gotDstIP == expectedIP {
				if int(gotHopLimit) == expectedTTL {
					t.Logf("Matched IPv6 packet: DstIP = %s, HopLimit = %d", gotDstIP, gotHopLimit)

					// Decode inner packet from outer payload
					innerPacket := gopacket.NewPacket(payload, nextLayerType, gopacket.Default)
					if innerPacket.Layer(layers.LayerTypeIPv6) != nil {
						t.Errorf("Packets are not decapped: inner IPv6 header still present.")
					}
					matched = true
					break outer
				}
				t.Errorf("Failed to match HopLimit: GotIP = %s, Expected IP = %s, GotHopLimit = %d, ExpectedHopLimit = %d", gotDstIP, expectedIP, gotHopLimit, expectedTTL)
				break outer
			}

		default:
			t.Fatalf("Unsupported protocol type: %s. Must be 'ipv4' or 'ipv6'", protocol)
		}
	}
	if !matched {
		t.Errorf("Did not find expected decapsulated packet with DstIP = %s, ExpectedTTL = %d", expectedIP, expectedTTL)
	}
}

func validateDUTPkts(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, flow gosnappi.Flow, initialInUnicastPkts, initialOutUnicastPkts, finalInUnicastPkts, finalOutUnicastPkts uint64) {
	t.Helper()
	if deviations.GreDecapsulationOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			ingressPkt := finalInUnicastPkts - initialInUnicastPkts
			ingressAtePkts := gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State())

			egressPkt := finalOutUnicastPkts - initialOutUnicastPkts
			egressAtePkts := gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State())

			if ingressPkt == 0 || egressPkt == 0 {
				t.Errorf("Got the unexpected packet count ingressPkt: %d, egressPkt: %d", ingressPkt, egressPkt)
			}

			if ingressPkt >= ingressAtePkts && egressPkt >= egressAtePkts {
				t.Logf("Interface counters reflect decapsulated packets: InUnicastPkts : %d OutUnicastPkts : %d", ingressPkt, egressPkt)
			} else {
				t.Errorf("Error: Interface counters didn't reflect decapsulated packets.")
			}
		default:
			t.Errorf("Deviation GreDecapsulationUnsupported is not handled for the dut: %v", dut.Vendor())
		}
	} else {
		// TO-DO: Once the support is added in the DUT, need to work on the validation of PF counters.
		matchedPkts := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy("PBR-MAP").Rule(10).MatchedPkts()
		pktCount := gnmi.Get(t, dut, matchedPkts.State())
		if pktCount != 0 {
			t.Logf("Interface counters received")
		} else {
			t.Errorf("Interface counters not received")
		}

		matchedOctets := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy("PBR-MAP").Rule(10).MatchedOctets()
		octetCount := gnmi.Get(t, dut, matchedOctets.State())
		if octetCount == 0 {
			t.Errorf("Octet counters not received")
		}
	}
}

func otgOperation(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, flow gosnappi.Flow) {
	t.Helper()
	enableCapture(t, config, "port2")
	otgConfig.PushConfig(t, config)
	otgConfig.StartProtocols(t)

	verifyPortsUp(t, dut.Device)
	otgutils.WaitForARP(t, otgConfig, config, "IPv4")
	otgutils.WaitForARP(t, otgConfig, config, "IPv6")
	initialInUnicastPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Counters().InUnicastPkts().State())
	initialOutUnicastPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Counters().OutUnicastPkts().State())

	cs := startCapture(t, otgConfig)
	otgConfig.StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	otgConfig.StopTraffic(t)

	stopCapture(t, otgConfig, cs)
	finalInUnicastPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Counters().InUnicastPkts().State())
	finalOutUnicastPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Counters().OutUnicastPkts().State())
	validateDUTPkts(t, dut, otgConfig, flow, initialInUnicastPkts, initialOutUnicastPkts, finalInUnicastPkts, finalOutUnicastPkts)
	if ok := verifyTrafficFlow(t, otgConfig, config, flow); !ok {
		t.Fatal("Packets Dropped, LossPct for flow ")
	} else {
		t.Log("Packets Received")
	}
}

func otgTrafficValidation(t *testing.T, otgConfig *otg.OTG, config gosnappi.Config, flow gosnappi.Flow) {
	t.Helper()
	otgConfig.PushConfig(t, config)
	otgConfig.StartProtocols(t)

	otgutils.WaitForARP(t, otgConfig, config, "IPv4")
	otgutils.WaitForARP(t, otgConfig, config, "IPv6")

	otgConfig.StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	otgConfig.StopTraffic(t)

	if ok := verifyTrafficFlow(t, otgConfig, config, flow); !ok {
		t.Log("Packets Dropped, Test Passed")
	} else {
		t.Fatal("Packet not Dropped, Test Failed")
	}
}

func createIPv4Flow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv4",
		outerSrcIP: atePort1.IPv4, outerDstIP: atePort2.IPv4, outerIpv4Ttl: innerTTL, ipv4Flow: true})
	config.Flows().Append(flow)
	if innerTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			outerDstIP: atePort2.IPv4, outerTtl: expectedTTL1, validateNonEncap: true}, "ipv4")
	} else {
		otgTrafficValidation(t, otgConfig, config, flow)
	}
}

func createIPv6Flow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv6",
		outerSrcIP: atePort1.IPv6, outerDstIP: atePort2.IPv6, outerIpv6Ttl: innerTTL})
	config.Flows().Append(flow)
	if innerTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			outerDstIP: atePort2.IPv6, outerTtl: expectedTTL1, validateNonEncap: true}, "ipv6")
	} else {
		otgTrafficValidation(t, otgConfig, config, flow)
	}
}

func createIPv4oGREFlow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv4",
		outerSrcIP: atePort1.IPv4, outerDstIP: ipv4Decap, outerIpv4Ttl: outerTTL, ipv4Flow: true})
	flow.Packet().Add().Gre()
	innerv4Header := flow.Packet().Add().Ipv4()
	innerv4Header.Src().SetValue(atePort1.IPv4)
	innerv4Header.Dst().SetValue(atePort2.IPv4)
	innerv4Header.TimeToLive().SetValue(uint32(innerTTL))
	config.Flows().Append(flow)

	if innerTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv4, innerTtl: expectedTTL1, validateDecap: true}, "ipv4")
	} else {
		otgTrafficValidation(t, otgConfig, config, flow)
	}
}

func createIPv6oGREFlow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv6",
		outerSrcIP: atePort1.IPv4, outerDstIP: ipv4Decap, outerIpv4Ttl: outerTTL, ipv4Flow: true})
	flow.Packet().Add().Gre()
	innerv6Header := flow.Packet().Add().Ipv6()
	innerv6Header.Src().SetValue(atePort1.IPv6)
	innerv6Header.Dst().SetValue(atePort2.IPv6)
	innerv6Header.HopLimit().SetValue(uint32(innerTTL))
	config.Flows().Append(flow)
	if innerTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv6, innerTtl: expectedTTL1, validateDecap: true}, "ipv6")
	} else {
		otgTrafficValidation(t, otgConfig, config, flow)
	}
}

func createIPv4oMPLSoGREFlow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv4",
		outerSrcIP: atePort1.IPv4, outerDstIP: ipv4Decap, outerIpv4Ttl: outerTTL, ipv4Flow: true})
	flow.Packet().Add().Gre()
	mplsHeader := flow.Packet().Add().Mpls()
	mplsHeader.Label().SetValue(mplsLabelV4)
	mplsHeader.TimeToLive().SetValue(uint32(mplsTTL))
	innerv4Header := flow.Packet().Add().Ipv4()
	innerv4Header.Src().SetValue(atePort1.IPv4)
	innerv4Header.Dst().SetValue(atePort2.IPv4)
	innerv4Header.TimeToLive().SetValue(uint32(innerTTL))
	config.Flows().Append(flow)
	expectedTTL2 := 10
	if mplsTTL != 1 {
		if innerTTL == 1 {
			expectedTTL2 = 1
		}
		otgOperation(t, dut, otgConfig, config, flow)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv4, innerTtl: expectedTTL2, validateDecap: true}, "ipv4")
	} else {
		otgTrafficValidation(t, otgConfig, config, flow)
	}
}

func createIPv6oMPLSoGREFlow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv6",
		outerSrcIP: atePort1.IPv4, outerDstIP: ipv4Decap, outerIpv4Ttl: outerTTL, ipv4Flow: true})
	flow.Packet().Add().Gre()
	mplsHeader := flow.Packet().Add().Mpls()
	mplsHeader.Label().SetValue(mplsLabelV6)
	mplsHeader.TimeToLive().SetValue(uint32(mplsTTL))
	innerv6Header := flow.Packet().Add().Ipv6()
	innerv6Header.Src().SetValue(atePort1.IPv6)
	innerv6Header.Dst().SetValue(atePort2.IPv6)
	innerv6Header.HopLimit().SetValue(uint32(innerTTL))
	config.Flows().Append(flow)
	expectedTTL2 := 10
	if mplsTTL != 1 {
		if innerTTL == 1 {
			expectedTTL2 = 1
		}
		otgOperation(t, dut, otgConfig, config, flow)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv6, innerTtl: expectedTTL2, validateDecap: true}, "ipv6")
	} else {
		otgTrafficValidation(t, otgConfig, config, flow)
	}
}

func createIPv4oUDPFlow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	cfgplugins.ConfigureDutWithGueDecap(t, dut, udpDecapPortIPv4, "ip", ipv4Decap, dp1.Name(), policyName, policyId)
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv4",
		outerSrcIP: atePort1.IPv4, outerDstIP: ipv4Decap, outerIpv4Ttl: outerTTL, ipv4Flow: true})
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.DstPort().SetValue(udpDecapPortIPv4)
	innerv4Header := flow.Packet().Add().Ipv4()
	innerv4Header.Src().SetValue(atePort1.IPv4)
	innerv4Header.Dst().SetValue(atePort2.IPv4)
	innerv4Header.TimeToLive().SetValue(uint32(innerTTL))

	config.Flows().Append(flow)
	if innerTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv4, innerTtl: expectedTTL1, validateDecap: true}, "ipv4")
	} else {
		otgTrafficValidation(t, otgConfig, config, flow)
	}
}

func createIPv6oUDPFlow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	cfgplugins.ConfigureDutWithGueDecap(t, dut, udpDecapPortIPv6, "ipv6", ipv4Decap, dp1.Name(), policyName, policyId)
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv6",
		outerSrcIP: atePort1.IPv4, outerDstIP: ipv4Decap, outerIpv4Ttl: outerTTL, ipv4Flow: true})
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.DstPort().SetValue(udpDecapPortIPv6)
	innerv6Header := flow.Packet().Add().Ipv6()
	innerv6Header.Src().SetValue(atePort1.IPv6)
	innerv6Header.Dst().SetValue(atePort2.IPv6)
	innerv6Header.HopLimit().SetValue(uint32(innerTTL))
	config.Flows().Append(flow)
	if innerTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv6, innerTtl: expectedTTL1, validateDecap: true}, "ipv6")
	} else {
		otgTrafficValidation(t, otgConfig, config, flow)
	}
}

func createIPv4oMPLSoUDPFlow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	cfgplugins.ConfigureDutWithGueDecap(t, dut, udpDecapPortMPLS, "mpls", ipv4Decap, dp1.Name(), policyName, policyId)
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv4",
		outerSrcIP: atePort1.IPv4, outerDstIP: ipv4Decap, outerIpv4Ttl: outerTTL, ipv4Flow: true})
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.DstPort().SetValue(udpDecapPortMPLS)
	mplsHeader := flow.Packet().Add().Mpls()
	mplsHeader.Label().SetValue(mplsLabelV4)
	mplsHeader.TimeToLive().SetValue(uint32(mplsTTL))
	innerv4Header := flow.Packet().Add().Ipv4()
	innerv4Header.Src().SetValue(atePort1.IPv4)
	innerv4Header.Dst().SetValue(atePort2.IPv4)
	innerv4Header.TimeToLive().SetValue(uint32(innerTTL))
	config.Flows().Append(flow)
	expectedTTL2 := 10
	if mplsTTL != 1 {
		if innerTTL == 1 {
			expectedTTL2 = 1
		}
		otgOperation(t, dut, otgConfig, config, flow)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv4, innerTtl: expectedTTL2, validateDecap: true}, "ipv4")
	} else {
		otgTrafficValidation(t, otgConfig, config, flow)
	}
}

func createIPv6oMPLSoUDPFlow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	cfgplugins.ConfigureDutWithGueDecap(t, dut, udpDecapPortMPLS, "mpls", ipv4Decap, dp1.Name(), policyName, policyId)
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv6",
		outerSrcIP: atePort1.IPv4, outerDstIP: ipv4Decap, outerIpv4Ttl: outerTTL, ipv4Flow: true})
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.DstPort().SetValue(udpDecapPortMPLS)
	mplsHeader := flow.Packet().Add().Mpls()
	mplsHeader.Label().SetValue(mplsLabelV6)
	mplsHeader.TimeToLive().SetValue(uint32(mplsTTL))
	innerv6Header := flow.Packet().Add().Ipv6()
	innerv6Header.Src().SetValue(atePort1.IPv6)
	innerv6Header.Dst().SetValue(atePort2.IPv6)
	innerv6Header.HopLimit().SetValue(uint32(innerTTL))
	config.Flows().Append(flow)
	expectedTTL2 := 10
	if mplsTTL != 1 {
		if innerTTL == 1 {
			expectedTTL2 = 1
		}
		otgOperation(t, dut, otgConfig, config, flow)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv6, innerTtl: expectedTTL2, validateDecap: true}, "ipv6")
	} else {
		otgTrafficValidation(t, otgConfig, config, flow)
	}
}

func enableCapture(t *testing.T, config gosnappi.Config, port string) {
	t.Helper()
	config.Captures().Clear()
	t.Log("Enabling capture on ", port)
	config.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
}

func startCapture(t *testing.T, otg *otg.OTG) gosnappi.ControlState {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)

	return cs
}

func stopCapture(t *testing.T, otg *otg.OTG, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

func processCapture(t *testing.T, otg *otg.OTG, port string) string {
	t.Helper()
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	capturePktFile, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := capturePktFile.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	defer capturePktFile.Close() // <- ensures the file is always closed
	return capturePktFile.Name()
}

// Verify ports status
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	t.Log("Verifying port status")
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}
