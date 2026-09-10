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
	"fmt"
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
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	ipv4Decap       = "10.2.2.2"
	ipv6Decap       = "2001:db8::10:2:2:2"
	ipv4DecapMask   = 32
	greDecapGrpName = "GRE-DECAP"
	frameSize       = 128
	packetsPerFlow  = 5
	pps             = 100   // Packets per second
	mplsLabelV4     = 99910 // Static Mpls currently supported range 16 - 99999 (99910 instead of 100010)
	mplsLabelV6     = 99920 // Static Mpls currently supported range 16 - 99999 (99920 instead of 100020)
	sleepTime       = 20
	flowname        = "trafficItem"
	tolerance       = 2
	// udpDecapPortGUE is the UDP destination port used for GUE (variant 1)
	// decapsulation of plain IPv4 and IPv6 payloads, per the canonical OC in the README.
	udpDecapPortGUE = 6080
	// udpDecapPortMPLSoUDP is the IANA assigned MPLS-in-UDP destination port used
	// for every MPLS payload decapsulation rule, per the canonical OC in the README.
	udpDecapPortMPLSoUDP = 6635
	defaultNI            = "DEFAULT"
	policyName           = "GUE-DECAP"
	policyID             = 1
	lspName1             = "lsp1"
	lspName2             = "lsp2"
	// pcapHeaderSize is the size of the PCAP global header. A capture shorter than this cannot be
	// parsed and results in a "truncated dump file" error.
	pcapHeaderSize = 24
	// captureRetries and captureRetryDelay control how often an empty capture is fetched again
	// before the validation gives up.
	captureRetries    = 3
	captureRetryDelay = 2 * time.Second
	udpSrcPort        = 49152
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
)

// flowArgs holds the parameters used to build an OTG traffic flow, covering the
// outer and inner header addresses along with the TTL/hop-limit values to set.
type flowArgs struct {
	flowName                   string
	outerSrcIP, outerDstIP     string
	InnerSrcIP, InnerDstIP     string
	InnerSrcIPv6, InnerDstIPv6 string
	ipv4Flow                   bool
	outerIpv4Ttl               int
	outerIpv6Ttl               int
}

// packetValidation describes the expectations applied to the packets captured on
// an ATE port, i.e. which header to inspect and the destination IP and TTL it
// must carry.
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
	configureHardwareInit(t, dut)
	configureDUT(t, dut)
	config := configureATE(t, ate)
	otgConfig := ate.OTG()
	sfBatch := &gnmi.SetBatch{}
	// Configure Static Route: MPLS label binding
	cfgplugins.MPLSStaticLSPByPass(t, sfBatch, dut, lspName1, mplsLabelV4, atePort2.IPv4, "ipv4", true)
	cfgplugins.MPLSStaticLSPByPass(t, sfBatch, dut, lspName2, mplsLabelV6, atePort2.IPv6, "ipv6", true)
	// Policy Based Forwading Rule-0: GRE decapsulation.
	configureGREDecap(t, dut, sfBatch)
	sfBatch.Set(t, dut)
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
		{
			Name:        "Testcase-IPv4oMPLSoUDPoIPv6Test",
			Description: "PF-1.9.29: IPv4oMPLSoUDPoIPv6 traffic with inner TTL = 10, MPLS TTL = 20 and outer TTL = 30",
			InnerTTL:    10,
			MplsTTL:     20,
			OuterTTL:    30,
			TestFunc:    createIPv4oMPLSoUDPoIPv6Flow,
		},
		{
			Name:        "Testcase-IPv4oMPLSoUDPoIPv6InnerTest",
			Description: "PF-1.9.30: IPv4oMPLSoUDPoIPv6 traffic with inner TTL = 1, MPLS TTL = 20 and outer TTL = 30",
			InnerTTL:    1,
			MplsTTL:     20,
			OuterTTL:    30,
			TestFunc:    createIPv4oMPLSoUDPoIPv6Flow,
		},
		{
			Name:        "Testcase-IPv4oMPLSoUDPoIPv6MplsTest",
			Description: "PF-1.9.31: IPv4oMPLSoUDPoIPv6 traffic with inner TTL = 10, MPLS TTL = 1 and outer TTL = 30",
			InnerTTL:    10,
			MplsTTL:     1,
			OuterTTL:    30,
			TestFunc:    createIPv4oMPLSoUDPoIPv6Flow,
		},
		{
			Name:        "Testcase-IPv6oMPLSoUDPoIPv6Test",
			Description: "PF-1.9.32: IPv6oMPLSoUDPoIPv6 traffic with inner TTL = 10, MPLS TTL = 20 and outer TTL = 30",
			InnerTTL:    10,
			MplsTTL:     20,
			OuterTTL:    30,
			TestFunc:    createIPv6oMPLSoUDPoIPv6Flow,
		},
		{
			Name:        "Testcase-IPv6oMPLSoUDPoIPv6InnerTest",
			Description: "PF-1.9.33: IPv6oMPLSoUDPoIPv6 traffic with inner TTL = 1, MPLS TTL = 20 and outer TTL = 30",
			InnerTTL:    1,
			MplsTTL:     20,
			OuterTTL:    30,
			TestFunc:    createIPv6oMPLSoUDPoIPv6Flow,
		},
		{
			Name:        "Testcase-IPv6oMPLSoUDPoIPv6MplsTest",
			Description: "PF-1.9.34: IPv6oMPLSoUDPoIPv6 traffic with inner TTL = 10, MPLS TTL = 1 and outer TTL = 30",
			InnerTTL:    10,
			MplsTTL:     1,
			OuterTTL:    30,
			TestFunc:    createIPv6oMPLSoUDPoIPv6Flow,
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

// configureDUT configures both DUT ports facing the ATE and the default network
// instance.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	configureDUTPort(t, dut, &dutPort1, p1)
	configureDUTPort(t, dut, &dutPort2, p2)

	fptest.ConfigureDefaultNetworkInstance(t, dut)
}

// configureDUTPort configures a single DUT interface with the IPv4 and IPv6
// addresses held in attrs, applying the relevant deviations for the DUT.
func configureDUTPort(t *testing.T, dut *ondatra.DUTDevice, attrs *attrs.Attributes, p *ondatra.Port) {
	t.Helper()
	d := gnmi.OC()
	i := attrs.NewOCInterface(p.Name(), dut)
	gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), i)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p)
	}
}

// configureATE builds the OTG configuration for ATE:Port1 and ATE:Port2 and
// returns it.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	t.Log("Configure ATE interfaces")

	config := gosnappi.NewConfig()

	atePort1.AddToOTG(config, ate.Port(t, "port1"), &dutPort1)
	atePort2.AddToOTG(config, ate.Port(t, "port2"), &dutPort2)

	return config
}

// configureHardwareInit sets up the initial hardware configuration on the DUT.
func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	features := []cfgplugins.FeatureType{
		cfgplugins.FeatureICMPForwarding,
	}
	for _, feature := range features {
		hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, feature)
		if hardwareInitCfg != "" {
			cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)
		}
	}
}

// addFlow creates an OTG flow from ATE:Port1 to ATE:Port2 with the Ethernet and
// outer IPv4 or IPv6 headers described by flowValues. Callers append any
// encapsulation and inner headers to the returned flow.
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

// verifyTrafficFlow reports whether the given flow was received on the ATE with a
// loss percentage below the configured tolerance.
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

// captureAndValidatePackets retrieves the capture taken on ATE:Port2 and validates
// the captured packets against the expectations in packetVal for the given
// protocol type ("ipv4" or "ipv6").
func captureAndValidatePackets(t *testing.T, otgConfig *otg.OTG, packetVal *packetValidation, protocolType string) {
	t.Helper()
	packetCaptureGRE, ok := processCapture(t, otgConfig, "port2")
	if !ok {
		return
	}
	defer os.Remove(packetCaptureGRE)
	handle, err := pcap.OpenOffline(packetCaptureGRE)
	if err != nil {
		t.Errorf("Could not open the capture taken on ATE:Port2: %v", err)
		return
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

// validateTrafficNonEncap verifies that a non-encapsulated packet destined to
// expectedIP was captured and that its TTL or hop limit equals expectedTTL.
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

// validateTrafficDecap verifies that the captured packets were decapsulated by the
// DUT: the outer header must be gone, the destination must equal expectedIP and
// the TTL or hop limit must equal expectedTTL.
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

// validateDUTPkts compares the DUT in-unicast-pkts and out-unicast-pkts deltas
// against the packets transmitted and received by the ATE, and checks the
// policy-forwarding matched-pkts and matched-octets counters when the DUT
// supports them.
func validateDUTPkts(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, flow gosnappi.Flow, initialInUnicastPkts, initialOutUnicastPkts, finalInUnicastPkts, finalOutUnicastPkts uint64, initialPFMatchedPkts, initialPFMatchedOctets uint64, expectPFMatch bool, pfPolicyName string, pfRuleID uint32) {
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
		if !expectPFMatch {
			t.Log("Policy-forwarding matched-pkts validation is not expected for this flow.")
			return
		}
		if pfPolicyName == "" {
			t.Errorf("Policy-forwarding validation requested but policy name is empty")
			return
		}
		pf := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()
		pfMatchedPkts := gnmi.Get(t, dut, pf.Policy(pfPolicyName).Rule(pfRuleID).MatchedPkts().State())
		pfMatchedOctets := gnmi.Get(t, dut, pf.Policy(pfPolicyName).Rule(pfRuleID).MatchedOctets().State())

		if pfMatchedPkts < initialPFMatchedPkts || pfMatchedOctets < initialPFMatchedOctets {
			t.Errorf("Policy-forwarding counters decreased unexpectedly for policy %s rule %d. Initial(pkts=%d,octets=%d), Final(pkts=%d,octets=%d)",
				pfPolicyName, pfRuleID, initialPFMatchedPkts, initialPFMatchedOctets, pfMatchedPkts, pfMatchedOctets)
			return
		}

		pfMatchedPktsDelta := pfMatchedPkts - initialPFMatchedPkts
		pfMatchedOctetsDelta := pfMatchedOctets - initialPFMatchedOctets
		t.Logf("Policy-forwarding counter deltas for policy %s rule %d: matched-pkts=%d matched-octets=%d", pfPolicyName, pfRuleID, pfMatchedPktsDelta, pfMatchedOctetsDelta)
		if pfMatchedPktsDelta != packetsPerFlow {
			t.Errorf("Policy %s rule %d matched-pkts delta is %d, want %d", pfPolicyName, pfRuleID, pfMatchedPktsDelta, packetsPerFlow)
		}
		if pfMatchedOctetsDelta == 0 {
			t.Errorf("Policy %s rule %d matched-octets delta is 0 while matched-pkts delta is %d", pfPolicyName, pfRuleID, pfMatchedPktsDelta)
		}
	}
}

// otgOperation runs the flow and performs the positive-path verifications from the README.
// ARP/ND for the ATE:Port2 next hop is resolved before the traffic starts (see waitForNeighbor), so
// no packet loss is expected on the first burst.
func otgOperation(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, flow gosnappi.Flow, expectPFMatch bool, pfPolicyName string, pfRuleID uint32) {
	t.Helper()
	enableCapture(t, config, "port2")
	otgConfig.PushConfig(t, config)
	otgConfig.StartProtocols(t)

	verifyPortsUp(t, dut.Device)
	otgutils.WaitForARP(t, otgConfig, config, "IPv4")
	otgutils.WaitForARP(t, otgConfig, config, "IPv6")
	initialInUnicastPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Counters().InUnicastPkts().State())
	initialOutUnicastPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Counters().OutUnicastPkts().State())
	var initialPFMatchedPkts, initialPFMatchedOctets uint64
	if !deviations.GreDecapsulationOCUnsupported(dut) && expectPFMatch {
		pf := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()
		initialPFMatchedPkts = gnmi.Get(t, dut, pf.Policy(pfPolicyName).Rule(pfRuleID).MatchedPkts().State())
		initialPFMatchedOctets = gnmi.Get(t, dut, pf.Policy(pfPolicyName).Rule(pfRuleID).MatchedOctets().State())
	}

	cs := startCapture(t, otgConfig)
	otgConfig.StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	otgConfig.StopTraffic(t)

	stopCapture(t, otgConfig, cs)
	finalInUnicastPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Counters().InUnicastPkts().State())
	finalOutUnicastPkts := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port2").Name()).Counters().OutUnicastPkts().State())
	validateDUTPkts(t, dut, otgConfig, flow, initialInUnicastPkts, initialOutUnicastPkts, finalInUnicastPkts, finalOutUnicastPkts,
		initialPFMatchedPkts, initialPFMatchedOctets, expectPFMatch, pfPolicyName, pfRuleID)
	if ok := verifyTrafficFlow(t, otgConfig, config, flow); !ok {
		t.Fatal("Packets Dropped, LossPct for flow ")
	} else {
		t.Log("Packets Received")
	}
}

// skipICMPValidation reports whether the ICMP/ICMPv6 Time Exceeded validation must be skipped
// for the current test case because the DUT does not generate the message after decapsulation.
// This applies to PF-1.9.7, PF-1.9.8, PF-1.9.15, PF-1.9.16, PF-1.9.27, PF-1.9.28, PF-1.9.31 and
// PF-1.9.34 when the DecapIcmpTtlExceededUnsupported deviation is set.
func skipICMPValidation(t *testing.T, dut *ondatra.DUTDevice, decapCase bool) bool {
	t.Helper()
	if decapCase && deviations.DecapICMPTTLExceededUnsupported(dut) {
		t.Log("Skipping ICMP/ICMPv6 Time Exceeded validation: deviation DecapIcmpTtlExceededUnsupported is set.")
		return true
	}
	return false
}

// otgTrafficValidation verifies that traffic is not forwarded to ATE:Port2 and that
// ATE:Port1 receives ICMP/ICMPv6 Time Exceeded messages for the packets sent.
// decapCase indicates that the TTL expires on the inner or MPLS header of an encapsulated
// packet, in which case the ICMP validation may be skipped through skipICMPValidation.
func otgTrafficValidation(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, flow gosnappi.Flow, protocolType string, decapCase bool) {
	t.Helper()
	enableCapture(t, config, "port1", "port2")
	otgConfig.PushConfig(t, config)
	otgConfig.StartProtocols(t)

	otgutils.WaitForARP(t, otgConfig, config, "IPv4")
	otgutils.WaitForARP(t, otgConfig, config, "IPv6")

	cs := startCapture(t, otgConfig)
	otgConfig.StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	otgConfig.StopTraffic(t)
	stopCapture(t, otgConfig, cs)

	rxPkts := gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State())
	if rxPkts != 0 {
		t.Fatalf("Packet not dropped, got %d packets on ATE:Port2, want 0", rxPkts)
	}
	t.Log("Packets dropped, Test Passed")
	if skipICMPValidation(t, dut, decapCase) {
		return
	}
	validateICMPTTLExceeded(t, otgConfig, protocolType)
}

// validateICMPTTLExceeded verifies ATE:Port1 received ICMP/ICMPv6 Time Exceeded packets.
func validateICMPTTLExceeded(t *testing.T, otgConfig *otg.OTG, protocolType string) {
	t.Helper()
	capturePath, ok := processCapture(t, otgConfig, "port1")
	if !ok {
		return
	}
	defer os.Remove(capturePath)
	handle, err := pcap.OpenOffline(capturePath)
	if err != nil {
		t.Errorf("Could not open the capture taken on ATE:Port1: %v", err)
		return
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	count := 0
	for packet := range packetSource.Packets() {
		switch protocolType {
		case "ipv4":
			if l := packet.Layer(layers.LayerTypeICMPv4); l != nil {
				if l.(*layers.ICMPv4).TypeCode.Type() == layers.ICMPv4TypeTimeExceeded {
					count++
				}
			}
		case "ipv6":
			if l := packet.Layer(layers.LayerTypeICMPv6); l != nil {
				if l.(*layers.ICMPv6).TypeCode.Type() == layers.ICMPv6TypeTimeExceeded {
					count++
				}
			}
		default:
			t.Fatalf("Unsupported protocol type: %s. Must be 'ipv4' or 'ipv6'", protocolType)
		}
	}
	if count == 0 {
		t.Errorf("ATE:Port1 did not receive any ICMP Time Exceeded packets, want %d", packetsPerFlow)
		return
	}
	if count < packetsPerFlow {
		t.Errorf("ATE:Port1 received %d ICMP Time Exceeded packets, want %d", count, packetsPerFlow)
		return
	}
	t.Logf("ATE:Port1 received %d ICMP Time Exceeded packets", count)
}

// createIPv4Flow covers PF-1.9.1 and PF-1.9.3: plain IPv4 traffic towards
// ATE:Port2. With innerTTL of 1 the DUT must drop the packets and reply with ICMP
// Time Exceeded, otherwise the TTL must be decremented by one on egress.
func createIPv4Flow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv4",
		outerSrcIP: atePort1.IPv4, outerDstIP: atePort2.IPv4, outerIpv4Ttl: innerTTL, ipv4Flow: true})
	config.Flows().Append(flow)
	if innerTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow, false, "", 0)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			outerDstIP: atePort2.IPv4, outerTtl: innerTTL - 1, validateNonEncap: true}, "ipv4")
	} else {
		otgTrafficValidation(t, dut, otgConfig, config, flow, "ipv4", false)
	}
}

// createIPv6Flow covers PF-1.9.2 and PF-1.9.4: plain IPv6 traffic towards
// ATE:Port2, verifying hop-limit decrement or ICMPv6 Time Exceeded generation.
func createIPv6Flow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv6",
		outerSrcIP: atePort1.IPv6, outerDstIP: atePort2.IPv6, outerIpv6Ttl: innerTTL})
	config.Flows().Append(flow)
	if innerTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow, false, "", 0)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			outerDstIP: atePort2.IPv6, outerTtl: innerTTL - 1, validateNonEncap: true}, "ipv6")
	} else {
		otgTrafficValidation(t, dut, otgConfig, config, flow, "ipv6", false)
	}
}

// createIPv4oGREFlow covers PF-1.9.5, PF-1.9.7 and PF-1.9.9: IPv4oGRE traffic that
// the DUT decapsulates before forwarding the inner IPv4 packet to ATE:Port2.
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
		otgOperation(t, dut, otgConfig, config, flow, true, greDecapGrpName, 0)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv4, innerTtl: innerTTL - 1, validateDecap: true}, "ipv4")
	} else {
		otgTrafficValidation(t, dut, otgConfig, config, flow, "ipv4", true)
	}
}

// createIPv6oGREFlow covers PF-1.9.6, PF-1.9.8 and PF-1.9.10: IPv6oGRE traffic that
// the DUT decapsulates before forwarding the inner IPv6 packet to ATE:Port2.
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
		otgOperation(t, dut, otgConfig, config, flow, true, greDecapGrpName, 0)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv6, innerTtl: innerTTL - 1, validateDecap: true}, "ipv6")
	} else {
		otgTrafficValidation(t, dut, otgConfig, config, flow, "ipv6", true)
	}
}

// createIPv4oMPLSoGREFlow covers PF-1.9.11, PF-1.9.13 and PF-1.9.15: IPv4oMPLSoGRE
// traffic that the DUT decapsulates and forwards using the static LSP for the
// IPv4 label.
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
	if mplsTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow, true, greDecapGrpName, 0)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv4, innerTtl: innerTTL, validateDecap: true}, "ipv4")
	} else {
		otgTrafficValidation(t, dut, otgConfig, config, flow, "ipv4", true)
	}
}

// createIPv6oMPLSoGREFlow covers PF-1.9.12, PF-1.9.14 and PF-1.9.16: IPv6oMPLSoGRE
// traffic that the DUT decapsulates and forwards using the static LSP for the
// IPv6 label.
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
	if mplsTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow, true, greDecapGrpName, 0)
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv6, innerTtl: innerTTL, validateDecap: true}, "ipv6")
	} else {
		otgTrafficValidation(t, dut, otgConfig, config, flow, "ipv6", true)
	}
}

// createIPv4oUDPFlow covers PF-1.9.17, PF-1.9.19 and PF-1.9.21: IPv4oUDP (GUE
// variant 1) traffic decapsulated by the DUT.
func createIPv4oUDPFlow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	configureGUEDecap(t, dut, cfgplugins.GUEDecapParams{
		GUEPort:    udpDecapPortGUE,
		IPType:     "ipv4",
		TunnelIP:   ipv4Decap,
		DecapInt:   dp1.Name(),
		PolicyName: policyName,
		PolicyID:   policyID,
	})
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv4",
		outerSrcIP: atePort1.IPv4, outerDstIP: ipv4Decap, outerIpv4Ttl: outerTTL, ipv4Flow: true})
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.DstPort().SetValue(udpDecapPortGUE)
	innerv4Header := flow.Packet().Add().Ipv4()
	innerv4Header.Src().SetValue(atePort1.IPv4)
	innerv4Header.Dst().SetValue(atePort2.IPv4)
	innerv4Header.TimeToLive().SetValue(uint32(innerTTL))

	config.Flows().Append(flow)
	if innerTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow, true, policyName, uint32(policyID))
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv4, innerTtl: innerTTL - 1, validateDecap: true}, "ipv4")
	} else {
		otgTrafficValidation(t, dut, otgConfig, config, flow, "ipv4", false)
	}
}

// createIPv6oUDPFlow covers PF-1.9.18, PF-1.9.20 and PF-1.9.22: IPv6oUDP (GUE
// variant 1) traffic decapsulated by the DUT.
func createIPv6oUDPFlow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	configureGUEDecap(t, dut, cfgplugins.GUEDecapParams{
		GUEPort:    udpDecapPortGUE,
		IPType:     "ipv6",
		TunnelIP:   ipv4Decap,
		DecapInt:   dp1.Name(),
		PolicyName: policyName,
		PolicyID:   policyID,
	})
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv6",
		outerSrcIP: atePort1.IPv4, outerDstIP: ipv4Decap, outerIpv4Ttl: outerTTL, ipv4Flow: true})
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.DstPort().SetValue(udpDecapPortGUE)
	innerv6Header := flow.Packet().Add().Ipv6()
	innerv6Header.Src().SetValue(atePort1.IPv6)
	innerv6Header.Dst().SetValue(atePort2.IPv6)
	innerv6Header.HopLimit().SetValue(uint32(innerTTL))
	config.Flows().Append(flow)
	if innerTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow, true, policyName, uint32(policyID))
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv6, innerTtl: innerTTL - 1, validateDecap: true}, "ipv6")
	} else {
		otgTrafficValidation(t, dut, otgConfig, config, flow, "ipv6", false)
	}
}

// createIPv4oMPLSoUDPFlow covers PF-1.9.23, PF-1.9.25 and PF-1.9.27:
// IPv4oMPLSoUDP (GUE variant 1) traffic decapsulated by the DUT and forwarded via
// the IPv4 static LSP.
func createIPv4oMPLSoUDPFlow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	configureGUEDecap(t, dut, cfgplugins.GUEDecapParams{
		GUEPort:    udpDecapPortMPLSoUDP,
		IPType:     "mpls",
		TunnelIP:   ipv4Decap,
		DecapInt:   dp1.Name(),
		PolicyName: policyName,
		PolicyID:   policyID,
	})
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv4",
		outerSrcIP: atePort1.IPv4, outerDstIP: ipv4Decap, outerIpv4Ttl: outerTTL, ipv4Flow: true})
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.SrcPort().SetValue(udpSrcPort)
	udpHeader.DstPort().SetValue(udpDecapPortMPLSoUDP)
	mplsHeader := flow.Packet().Add().Mpls()
	mplsHeader.Label().SetValue(mplsLabelV4)
	mplsHeader.TimeToLive().SetValue(uint32(mplsTTL))
	innerv4Header := flow.Packet().Add().Ipv4()
	innerv4Header.Src().SetValue(atePort1.IPv4)
	innerv4Header.Dst().SetValue(atePort2.IPv4)
	innerv4Header.TimeToLive().SetValue(uint32(innerTTL))
	config.Flows().Append(flow)
	if mplsTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow, true, policyName, uint32(policyID))
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv4, innerTtl: innerTTL, validateDecap: true}, "ipv4")
	} else {
		configureMPLSStaticLSPForTTLOne(t, dut, lspName1, mplsLabelV4, atePort2.IPv4, "ipv4")
		otgTrafficValidation(t, dut, otgConfig, config, flow, "ipv4", false)
	}
}

// createIPv6oMPLSoUDPFlow covers PF-1.9.24, PF-1.9.26 and PF-1.9.28:
// IPv6oMPLSoUDP (GUE variant 1) traffic decapsulated by the DUT and forwarded via
// the IPv6 static LSP.
func createIPv6oMPLSoUDPFlow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	configureGUEDecap(t, dut, cfgplugins.GUEDecapParams{
		GUEPort:    udpDecapPortMPLSoUDP,
		IPType:     "mpls",
		TunnelIP:   ipv4Decap,
		DecapInt:   dp1.Name(),
		PolicyName: policyName,
		PolicyID:   policyID,
	})
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv6",
		outerSrcIP: atePort1.IPv4, outerDstIP: ipv4Decap, outerIpv4Ttl: outerTTL, ipv4Flow: true})
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.SrcPort().SetValue(udpSrcPort)
	udpHeader.DstPort().SetValue(udpDecapPortMPLSoUDP)
	mplsHeader := flow.Packet().Add().Mpls()
	mplsHeader.Label().SetValue(mplsLabelV6)
	mplsHeader.TimeToLive().SetValue(uint32(mplsTTL))
	innerv6Header := flow.Packet().Add().Ipv6()
	innerv6Header.Src().SetValue(atePort1.IPv6)
	innerv6Header.Dst().SetValue(atePort2.IPv6)
	innerv6Header.HopLimit().SetValue(uint32(innerTTL))
	config.Flows().Append(flow)
	if mplsTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow, true, policyName, uint32(policyID))
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv6, innerTtl: innerTTL, validateDecap: true}, "ipv6")
	} else {
		configureMPLSStaticLSPForTTLOne(t, dut, lspName2, mplsLabelV6, atePort2.IPv6, "ipv6")
		otgTrafficValidation(t, dut, otgConfig, config, flow, "ipv6", false)
	}
}

// createIPv4oMPLSoUDPoIPv6Flow covers PF-1.9.29 - PF-1.9.31: IPv4oMPLSoUDP with an IPv6 outer header.
func createIPv4oMPLSoUDPoIPv6Flow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	configureGUEDecap(t, dut, cfgplugins.GUEDecapParams{
		GUEPort:    udpDecapPortMPLSoUDP,
		IPType:     "mpls",
		TunnelIP:   ipv6Decap,
		DecapInt:   dp1.Name(),
		PolicyName: policyName,
		PolicyID:   policyID,
	})
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv4omplsoudpoipv6",
		outerSrcIP: atePort1.IPv6, outerDstIP: ipv6Decap, outerIpv6Ttl: outerTTL})
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.SrcPort().SetValue(udpSrcPort)
	udpHeader.DstPort().SetValue(udpDecapPortMPLSoUDP)
	mplsHeader := flow.Packet().Add().Mpls()
	mplsHeader.Label().SetValue(mplsLabelV4)
	mplsHeader.TimeToLive().SetValue(uint32(mplsTTL))
	innerv4Header := flow.Packet().Add().Ipv4()
	innerv4Header.Src().SetValue(atePort1.IPv4)
	innerv4Header.Dst().SetValue(atePort2.IPv4)
	innerv4Header.TimeToLive().SetValue(uint32(innerTTL))
	config.Flows().Append(flow)
	if mplsTTL != 1 {
		// MPLS TTL is not propagated to the inner header on decapsulation, so the
		// inner TTL is expected to be unchanged.
		otgOperation(t, dut, otgConfig, config, flow, true, policyName, uint32(policyID))
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv4, innerTtl: innerTTL, validateDecap: true}, "ipv4")
	} else {
		configureMPLSStaticLSPForTTLOne(t, dut, lspName1, mplsLabelV4, atePort2.IPv4, "ipv4")
		otgTrafficValidation(t, dut, otgConfig, config, flow, "ipv4", false)
	}
}

// createIPv6oMPLSoUDPoIPv6Flow covers PF-1.9.32 - PF-1.9.34: IPv6oMPLSoUDP with an IPv6 outer header.
func createIPv6oMPLSoUDPoIPv6Flow(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config, innerTTL, outerTTL, mplsTTL int) {
	t.Helper()
	dp1 := dut.Port(t, "port1")
	configureGUEDecap(t, dut, cfgplugins.GUEDecapParams{
		GUEPort:    udpDecapPortMPLSoUDP,
		IPType:     "mpls",
		TunnelIP:   ipv6Decap,
		DecapInt:   dp1.Name(),
		PolicyName: policyName,
		PolicyID:   policyID,
	})
	config.Flows().Clear()
	flow := addFlow(t, config, &flowArgs{flowName: flowname + "-ipv6omplsoudpoipv6",
		outerSrcIP: atePort1.IPv6, outerDstIP: ipv6Decap, outerIpv6Ttl: outerTTL})
	udpHeader := flow.Packet().Add().Udp()
	udpHeader.SrcPort().SetValue(udpSrcPort)
	udpHeader.DstPort().SetValue(udpDecapPortMPLSoUDP)
	mplsHeader := flow.Packet().Add().Mpls()
	mplsHeader.Label().SetValue(mplsLabelV6)
	mplsHeader.TimeToLive().SetValue(uint32(mplsTTL))
	innerv6Header := flow.Packet().Add().Ipv6()
	innerv6Header.Src().SetValue(atePort1.IPv6)
	innerv6Header.Dst().SetValue(atePort2.IPv6)
	innerv6Header.HopLimit().SetValue(uint32(innerTTL))
	config.Flows().Append(flow)
	if mplsTTL != 1 {
		otgOperation(t, dut, otgConfig, config, flow, true, policyName, uint32(policyID))
		captureAndValidatePackets(t, otgConfig, &packetValidation{portName: atePort2.Name,
			innerDstIP: atePort2.IPv6, innerTtl: innerTTL, validateDecap: true}, "ipv6")
	} else {
		configureMPLSStaticLSPForTTLOne(t, dut, lspName2, mplsLabelV6, atePort2.IPv6, "ipv6")
		otgTrafficValidation(t, dut, otgConfig, config, flow, "ipv6", false)
	}
}

// configureMPLSStaticLSPForTTLOne updates the static LSP programming for MPLS TTL=1
// validation so that the TTL=1 packet takes the expected behavior for this test, and
// restores the original programming when the subtest finishes.
func configureMPLSStaticLSPForTTLOne(t *testing.T, dut *ondatra.DUTDevice, lspName string, label uint32, nextHop, ipType string) {
	t.Helper()
	sfBatch := &gnmi.SetBatch{}
	cfgplugins.RemoveMPLSStaticLSP(t, sfBatch, dut, lspName, label, nextHop, ipType, true)
	cfgplugins.MPLSStaticLSPByPass(t, sfBatch, dut, lspName, label, nextHop, ipType, false)
	sfBatch.Set(t, dut)

	t.Cleanup(func() {
		rb := &gnmi.SetBatch{}
		cfgplugins.RemoveMPLSStaticLSP(t, rb, dut, lspName, label, nextHop, ipType, false)
		cfgplugins.MPLSStaticLSPByPass(t, rb, dut, lspName, label, nextHop, ipType, true)
		rb.Set(t, dut)
	})
}

// configureGUEDecap programs the GUE/MPLS-in-UDP decapsulation policy on the DUT and registers
// its removal with t.Cleanup, so that each subtest starts from a clean decapsulation state and the
// DUT is left in its original state once the subtest completes.
func configureGUEDecap(t *testing.T, dut *ondatra.DUTDevice, params cfgplugins.GUEDecapParams) {
	t.Helper()
	sfBatch := &gnmi.SetBatch{}
	cfgplugins.NewConfigureDutWithGueDecap(t, dut, sfBatch, params)
	sfBatch.Set(t, dut)
	t.Cleanup(func() {
		cfgplugins.NewRemoveDutGueDecap(t, dut, params)
	})
}

// configureGREDecap programs rule 0 of the decapsulation policy on the DUT and registers its
// removal with t.Cleanup, so that the DUT is left in its original state once the test completes.
func configureGREDecap(t *testing.T, dut *ondatra.DUTDevice, sfBatch *gnmi.SetBatch) {
	t.Helper()
	ingressPort := dut.Port(t, "port1").Name()
	greParams := cfgplugins.GRETunnelParams{DecapIP: fmt.Sprintf("%s/%d", ipv4Decap, ipv4DecapMask), DecapGroupName: greDecapGrpName, DecapInterface: ingressPort}
	cfgplugins.NewConfigureGRETunnel(t, dut, sfBatch, greParams)
	t.Cleanup(func() {
		cfgplugins.NewRemoveGRETunnel(t, dut, greParams)
	})
}

// enableCapture replaces the capture configuration so that packets are captured in
// PCAP format on each of the given ATE ports.
func enableCapture(t *testing.T, config gosnappi.Config, ports ...string) {
	t.Helper()
	config.Captures().Clear()
	for _, port := range ports {
		t.Log("Enabling capture on ", port)
		config.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}
}

// startCapture starts packet capture on the ATE and returns the control state that
// must be passed to stopCapture.
func startCapture(t *testing.T, otg *otg.OTG) gosnappi.ControlState {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)

	return cs
}

// stopCapture stops the packet capture previously started with startCapture.
func stopCapture(t *testing.T, otg *otg.OTG, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

// processCapture downloads the capture for the given ATE port into a temporary PCAP file and
// returns its path. The ATE occasionally reports an empty buffer right after the capture is
// stopped, which results in a "truncated dump file" error when the file is opened, so the capture
// is retrieved again a few times before giving up. The returned boolean reports whether a usable
// capture was written; the caller is responsible for removing the file.
func processCapture(t *testing.T, otg *otg.OTG, port string) (string, bool) {
	t.Helper()
	var bytes []byte
	for i := 0; i < captureRetries; i++ {
		bytes = otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
		// A PCAP file always starts with a 24 byte global header; anything shorter is a
		// truncated dump that gopacket cannot parse.
		if len(bytes) >= pcapHeaderSize {
			break
		}
		t.Logf("Capture on %s returned %d bytes, retrying in %v", port, len(bytes), captureRetryDelay)
		time.Sleep(captureRetryDelay)
	}
	if len(bytes) < pcapHeaderSize {
		t.Errorf("Capture on %s is truncated: got %d bytes, want at least %d", port, len(bytes), pcapHeaderSize)
		return "", false
	}
	capturePktFile, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
		return "", false
	}
	if _, err := capturePktFile.Write(bytes); err != nil {
		capturePktFile.Close()
		os.Remove(capturePktFile.Name())
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
		return "", false
	}
	// The file must be closed before it is opened for reading, otherwise the buffered
	// contents may not have been flushed yet and the dump appears truncated.
	if err := capturePktFile.Close(); err != nil {
		os.Remove(capturePktFile.Name())
		t.Errorf("ERROR: Could not close pcap file: %v\n", err)
		return "", false
	}
	return capturePktFile.Name(), true
}

// verifyPortsUp verifies that every port of the given device reports an
// operational status of UP.
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
