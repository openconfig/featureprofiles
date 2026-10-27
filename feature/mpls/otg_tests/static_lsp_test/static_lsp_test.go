package static_lsp_test

import (
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	lspNextHopIndex = 0
	tolerance       = 0.01 // 1% Traffic Tolerance

	// MPLSoGUE tunnel (outer) parameters.
	gueUDPPort    = 6635         // Well-known UDP port for MPLSoGUE.
	guePolicyName = "customer10" // Policy forwarding name for MPLSoGUE decap.
	outerSrcIP    = "100.64.0.1" // Outer IPv4 source of the encapsulated traffic (from 100.64.0.0/22).
	outerDstIP    = "11.1.1.1"   // Outer IPv4 destination, within the DUT decap prefix (11.0.0.0/8).
	outerUDPSrc   = 49152

	// Base Static LSP labels bound to default untagged egress next-hop.
	mplsLabelIPv4 = 99991
	mplsLabelIPv6 = 99992

	// PF-1.25.v6 push (ingress) LSP parameters: match an IPv6 FEC subnet and push a
	// single label towards the ATE IPv6 next-hop on the egress port.
	v6LSPName       = "v6-static-lsp"
	mplsPushLabelV6 = 100100
	pushFECDstIP    = "2001:db8:cafe::1"
	pushFECDstCIDR  = "2001:db8:cafe::/64"

	// Inner payload addressing.
	innerIPv4Src = "198.51.100.1"
	innerIPv4Dst = "203.0.113.1"
	innerIPv6Src = "2001:db8:2::1"
	innerIPv6Dst = "2001:db8:2::2"

	// Inner payload DSCP/TTL used to validate preservation across decap.
	innerTTL = 64

	trafficPPS     = 1000
	trafficPackets = 10000
)

var (
	// IMIX frame size profile (64, 128, 256, 512, 1024, 1500, 9000 MTU bytes).
	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 20},
		{Size: 256, Weight: 20},
		{Size: 512, Weight: 10},
		{Size: 1024, Weight: 10},
		{Size: 1500, Weight: 18},
		{Size: 9000, Weight: 2},
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// verifyIPv6NeighborTelemetry validates that the DUT has resolved the IPv6 neighbor LinkLayerAddress.
func verifyIPv6NeighborTelemetry(t *testing.T, dut *ondatra.DUTDevice, aggID string, expectedMAC string) {
	t.Helper()
	llAddr, ok := gnmi.Await(t, dut, gnmi.OC().Interface(aggID).Subinterface(0).Ipv6().Neighbor(ateDst.IPv6).LinkLayerAddress().State(), time.Minute, expectedMAC).Val()
	if !ok || llAddr == "" {
		t.Errorf("IPv6 neighbor link-layer-address is empty for next-hop %s", ateDst.IPv6)
	}
}

// verifyLagLoadBalancing verifies that packets are distributed across all member links of the Egress LAG.
func verifyLagLoadBalancing(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	var total uint64
	counts := make([]uint64, len(egressLagPorts))

	for i, portName := range egressLagPorts {
		p := ate.Port(t, portName)
		counts[i] = gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(p.ID()).Counters().InFrames().State())
		total += counts[i]
		if counts[i] == 0 {
			t.Errorf("Egress LAG member port %s did not receive any packets", p.ID())
		}
	}
	if total == 0 {
		t.Fatal("No packets were received on the Egress LAG")
	}

	expected := float64(total) / float64(len(egressLagPorts))
	const tolerancePct = 0.10
	for i, portName := range egressLagPorts {
		p := ate.Port(t, portName)
		low := expected * (1 - tolerancePct)
		high := expected * (1 + tolerancePct)
		if float64(counts[i]) < low || float64(counts[i]) > high {
			t.Errorf("Egress LAG member port %s received %d packets; expected range [%.0f, %.0f], total=%d", p.ID(), counts[i], low, high, total)
		}
		t.Logf("Egress LAG member port %s received %d packets; expected range [%.0f, %.0f]", p.ID(), counts[i], low, high)
	}
}

// innerIPv4Flow returns inner IPv4 payload with random/varying source/dest IPs, DSCP range 0-56,
// and TCP/UDP source and destination port variation.
func innerIPv4Flow() *otgconfighelpers.Flow {
	return &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{
			IPv4Src:      innerIPv4Src,
			IPv4SrcCount: 1000,
			IPv4Dst:      innerIPv4Dst,
			IPv4DstCount: 1000,
			TTL:          innerTTL,
			DSCP:         0,
			DSCPCount:    57,
		},
		TCPFlow: &otgconfighelpers.TCPFlowParams{
			TCPSrcPort:  49152,
			TCPSrcCount: 1000,
			TCPDstPort:  80,
			TCPDstCount: 1000,
		},
	}
}

// innerIPv6Flow returns inner IPv6 payload with random/varying source/dest IPs, DSCP/TC range,
// and TCP/UDP source and destination port variation.
func innerIPv6Flow() *otgconfighelpers.Flow {
	return &otgconfighelpers.Flow{
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{
			IPv6Src:           innerIPv6Src,
			IPv6SrcCount:      1000,
			IPv6Dst:           innerIPv6Dst,
			IPv6DstCount:      1000,
			HopLimit:          innerTTL,
			TrafficClass:      0,
			TrafficClassCount: 255,
		},
		TCPFlow: &otgconfighelpers.TCPFlowParams{
			TCPSrcPort:  49152,
			TCPSrcCount: 1000,
			TCPDstPort:  80,
			TCPDstCount: 1000,
		},
	}
}

// allIPv4TOSValuesForDSCPRange returns the IPv4 ToS byte values that correspond to
// the DSCP range [start, end]
func allIPv4TOSValuesForDSCPRange(start, end uint32) []uint8 {
	vals := make([]uint8, 0, end-start+1)
	for dscp := start; dscp <= end; dscp++ {
		vals = append(vals, uint8(dscp<<2))
	}
	return vals
}

// innerIPv4PreserveSweepFlow sends one flow that cycles through DSCP values 0..56.
// This validates the full range without 57 independent OTG runs/captures.
func innerIPv4PreserveSweepFlow() *otgconfighelpers.Flow {
	return &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{
			IPv4Src:   innerIPv4Src,
			IPv4Dst:   innerIPv4Dst,
			TTL:       innerTTL,
			DSCP:      0,
			DSCPCount: 57,
		},
		TCPFlow: &otgconfighelpers.TCPFlowParams{
			TCPSrcPort:  49152,
			TCPSrcCount: 1000,
			TCPDstPort:  80,
			TCPDstCount: 1000,
		},
	}
}

// createMPLSoGUEFlow builds an MPLSoGUE-encapsulated flow from an Ingress ATE port towards the DUT.
func createMPLSoGUEFlow(top gosnappi.Config, name string, txName string, txMAC string, label uint32, inner *otgconfighelpers.Flow, rxName string) {
	f := &otgconfighelpers.Flow{
		FlowName:          name,
		TxNames:           []string{txName},
		RxNames:           []string{rxName},
		SizeWeightProfile: &sizeWeightProfile,
		PpsRate:           trafficPPS,
		PacketsToSend:     trafficPackets,
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: txMAC},
		IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIP, IPv4SrcCount: 1000, IPv4Dst: outerDstIP},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: outerUDPSrc, UDPSrcCount: 1000, UDPDstPort: gueUDPPort},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: label, MPLSExp: 7},
	}
	f.CreateFlow(top)
	f.AddEthHeader()
	f.AddIPv4Header()
	f.AddUDPHeader()
	f.AddMPLSHeader()
	if inner.IPv4Flow != nil {
		f.IPv4Flow = inner.IPv4Flow
		f.AddIPv4Header()
	}
	if inner.IPv6Flow != nil {
		f.IPv6Flow = inner.IPv6Flow
		f.AddIPv6Header()
	}
	// The outer GUE encapsulation uses UDP, while the inner payload TCP
	if inner.TCPFlow != nil {
		f.TCPFlow = inner.TCPFlow
		f.AddTCPHeader()
	}
}

// pushAndResolve pushes the OTG config, starts protocols, and waits for neighbor resolution.
func pushAndResolve(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	otgObj := ate.OTG()
	otgObj.PushConfig(t, top)
	otgObj.StartProtocols(t)
	otgutils.WaitForARP(t, otgObj, top, "IPv4")
	otgutils.WaitForARP(t, otgObj, top, "IPv6")
}

// runFlows starts traffic, waits for all packets to be transmitted, and stops traffic.
func runFlows(t *testing.T, ate *ondatra.ATEDevice, flowNames []string) {
	t.Helper()
	otgObj := ate.OTG()
	otgObj.StartTraffic(t)
	defer otgObj.StopTraffic(t)
	for _, name := range flowNames {
		gnmi.Watch(t, otgObj, gnmi.OTG().Flow(name).Counters().OutPkts().State(), time.Minute, func(v *ygnmi.Value[uint64]) bool {
			pkts, ok := v.Val()
			return ok && pkts >= uint64(trafficPackets)
		}).Await(t)
	}
}

// TestStaticLSP verifies MPLSoGUE decapsulation and static MPLS label to egress next-hop forwarding.
func TestStaticLSP(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	aggID := configureDUT(t, dut)

	var top gosnappi.Config
	t.Run("PF-1.25.1: Generate config for MPLS in GRE decap and push to DUT", func(t *testing.T) {
		top = configureOTG(t, ate)
	})

	v4Val := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-ipv4", TolerancePct: tolerance * 100}}
	v6Val := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-ipv6", TolerancePct: tolerance * 100}}

	t.Run("PF-1.25.2: Verify MPLSoGUE decapsulate action for IPv4 and IPv6 payload", func(t *testing.T) {
		top.Flows().Clear()
		// Base flows to default egress LAG subinterface 0
		createMPLSoGUEFlow(top, "mplsogue-ipv4", ateIngress1.Name+".IPv4", ateIngress1.MAC, mplsLabelIPv4, innerIPv4Flow(), ateDst.Name+".IPv4")
		createMPLSoGUEFlow(top, "mplsogue-ipv6", ateIngress2.Name+".IPv4", ateIngress2.MAC, mplsLabelIPv6, innerIPv6Flow(), ateDst.Name+".IPv6")

		// Customer VLAN flows to customer subinterfaces (e.g. VLAN 20 IPv4, VLAN 24 IPv6, VLAN 26 Dual-stack)
		createMPLSoGUEFlow(top, "mplsogue-cust-v4-vlan20", ateIngress1.Name+".IPv4", ateIngress1.MAC, 99920, innerIPv4Flow(), "cust-vlan20.IPv4")
		createMPLSoGUEFlow(top, "mplsogue-cust-v6-vlan24", ateIngress2.Name+".IPv4", ateIngress2.MAC, 99924, innerIPv6Flow(), "cust-vlan24.IPv6")
		createMPLSoGUEFlow(top, "mplsogue-cust-v4-vlan26", ateIngress1.Name+".IPv4", ateIngress1.MAC, 99926, innerIPv4Flow(), "cust-vlan26.IPv4")
		createMPLSoGUEFlow(top, "mplsogue-cust-v6-vlan26", ateIngress2.Name+".IPv4", ateIngress2.MAC, 99936, innerIPv6Flow(), "cust-vlan26.IPv6")

		pushAndResolve(t, ate, top)
		flowList := []string{
			"mplsogue-ipv4",
			"mplsogue-ipv6",
			"mplsogue-cust-v4-vlan20",
			"mplsogue-cust-v6-vlan24",
			"mplsogue-cust-v4-vlan26",
			"mplsogue-cust-v6-vlan26",
		}
		runFlows(t, ate, flowList)
		otgutils.LogFlowMetrics(t, ate.OTG(), top)

		for _, flowName := range flowList {
			val := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: flowName, TolerancePct: tolerance * 100}}
			if err := val.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("Flow %s decap validation failed: %v", flowName, err)
			}
		}
		verifyLagLoadBalancing(t, ate)
	})

	t.Run("PF-1.25.3: Verify decap traffic is unaffected by IPv4/IPv6 VLAN config changes", func(t *testing.T) {
		t.Run("VLAN config churn", func(t *testing.T) {
			// Test VLAN config churn on Dual-stack VLAN 26:
			// Churn IPv4 address on VLAN 26 while IPv6 decap traffic flows on VLAN 26.
			top.Flows().Clear()
			createMPLSoGUEFlow(top, "mplsogue-cust-v6-vlan26", ateIngress2.Name+".IPv4", ateIngress2.MAC, 99936, innerIPv6Flow(), "cust-vlan26.IPv6")
			createMPLSoGUEFlow(top, "mplsogue-cust-v4-vlan26", ateIngress1.Name+".IPv4", ateIngress1.MAC, 99926, innerIPv4Flow(), "cust-vlan26.IPv4")
			pushAndResolve(t, ate, top)

			cs26 := custSubinterfaces[6] // VLAN 26
			removeSubinterfaceAddress(t, dut, aggID, cs26.vlanID, cs26.dutIPv4, "IPv4")
			addSubinterfaceAddress(t, dut, aggID, cs26.vlanID, cs26.dutIPv4, cs26.ipv4Len, "IPv4")
			runFlows(t, ate, []string{"mplsogue-cust-v6-vlan26"})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			v6Val26 := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-cust-v6-vlan26", TolerancePct: tolerance * 100}}
			if err := v6Val26.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv6 decap flow during IPv4 VLAN config churn: %v", err)
			}

			// Churn IPv6 address on VLAN 26 while IPv4 decap traffic flows on VLAN 26.
			removeSubinterfaceAddress(t, dut, aggID, cs26.vlanID, cs26.dutIPv6, "IPv6")
			addSubinterfaceAddress(t, dut, aggID, cs26.vlanID, cs26.dutIPv6, cs26.ipv6Len, "IPv6")
			runFlows(t, ate, []string{"mplsogue-cust-v4-vlan26"})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			v4Val26 := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-cust-v4-vlan26", TolerancePct: tolerance * 100}}
			if err := v4Val26.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv4 decap flow during IPv6 VLAN config churn: %v", err)
			}
		})

		t.Run("Decap config churn", func(t *testing.T) {
			var v4LSP, v6LSP staticLSP
			for _, l := range staticLSPList() {
				if l.name == "lsp-v4-vlan26" {
					v4LSP = l
				}
				if l.name == "lsp-v6-vlan26" {
					v6LSP = l
				}
			}

			// Remove and re-add IPv4 static LSP decap config; verify IPv6 traffic unaffected.
			top.Flows().Clear()
			createMPLSoGUEFlow(top, "mplsogue-cust-v6-vlan26", ateIngress2.Name+".IPv4", ateIngress2.MAC, 99936, innerIPv6Flow(), "cust-vlan26.IPv6")
			pushAndResolve(t, ate, top)
			deleteStaticLSP(t, dut, v4LSP)
			setStaticLSP(t, dut, v4LSP)
			runFlows(t, ate, []string{"mplsogue-cust-v6-vlan26"})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			v6Val26 := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-cust-v6-vlan26", TolerancePct: tolerance * 100}}
			if err := v6Val26.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv6 decap flow during IPv4 decap config churn: %v", err)
			}

			// Remove and re-add IPv6 static LSP decap config; verify IPv4 traffic unaffected.
			top.Flows().Clear()
			createMPLSoGUEFlow(top, "mplsogue-cust-v4-vlan26", ateIngress1.Name+".IPv4", ateIngress1.MAC, 99926, innerIPv4Flow(), "cust-vlan26.IPv4")
			pushAndResolve(t, ate, top)
			deleteStaticLSP(t, dut, v6LSP)
			setStaticLSP(t, dut, v6LSP)
			runFlows(t, ate, []string{"mplsogue-cust-v4-vlan26"})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			v4Val26 := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-cust-v4-vlan26", TolerancePct: tolerance * 100}}
			if err := v4Val26.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv4 decap flow during IPv6 decap config churn: %v", err)
			}
		})
	})

	t.Run("PF-1.25.4: Verify MPLSoGUE DSCP/TTL preserve operation", func(t *testing.T) {
		// Preserve the inner TTL across MPLS disposition rather than copying
		// the decremented outer label TTL into the IP header.
		if !deviations.MplsOCUnsupported(dut) {
			ttlPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().Global().TtlPropagation()
			previous := gnmi.Lookup(t, dut, ttlPath.Config())
			gnmi.Update(t, dut, ttlPath.Config(), false)
			t.Cleanup(func() {
				if value, present := previous.Val(); present {
					gnmi.Update(t, dut, ttlPath.Config(), value)
				} else {
					gnmi.Delete(t, dut, ttlPath.Config())
				}
			})
		}
		packetvalidationhelpers.ClearCapture(t, top, ate)
		top.Flows().Clear()
		createMPLSoGUEFlow(top, "mplsogue-ipv4", ateIngress1.Name+".IPv4", ateIngress1.MAC, mplsLabelIPv4, innerIPv4PreserveSweepFlow(), ateDst.Name+".IPv4")
		pCapture := ate.Port(t, egressLagPorts[0])
		pv := &packetvalidationhelpers.PacketValidation{
			PortName:    pCapture.ID(),
			CaptureName: "ipv4-decap",
			Validations: []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateIPv4Header},
			// Decapsulated inner IPv4 must retain the DSCP/TOS set across the sweep. TOS = DSCP << 2.
			IPv4Layer: &packetvalidationhelpers.IPv4Layer{DstIP: innerIPv4Dst, AllowedTOSValues: allIPv4TOSValuesForDSCPRange(0, 56), TTL: innerTTL, SkipProtocolCheck: true},
		}
		packetvalidationhelpers.ConfigurePacketCapture(t, top, pv)
		pushAndResolve(t, ate, top)
		cs := packetvalidationhelpers.StartCapture(t, ate)
		defer packetvalidationhelpers.StopCapture(t, ate, cs)
		runFlows(t, ate, []string{"mplsogue-ipv4"})
		otgutils.LogFlowMetrics(t, ate.OTG(), top)
		if err := v4Val.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("IPv4 decap flow: %v", err)
		}
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, pv); err != nil {
			t.Errorf("DSCP/TTL preserve validation: %v", err)
		}
	})

	t.Run("PF-1.25.5: Verify IPv4/IPv6 nexthop resolution of decap traffic", func(t *testing.T) {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		top.Flows().Clear()
		createMPLSoGUEFlow(top, "mplsogue-ipv4", ateIngress1.Name+".IPv4", ateIngress1.MAC, mplsLabelIPv4, innerIPv4Flow(), ateDst.Name+".IPv4")
		createMPLSoGUEFlow(top, "mplsogue-ipv6", ateIngress2.Name+".IPv4", ateIngress2.MAC, mplsLabelIPv6, innerIPv6Flow(), ateDst.Name+".IPv6")
		pushAndResolve(t, ate, top)
		nhVal := &otgvalidationhelpers.InterfaceParams{Names: []string{ateDst.Name}}
		nhValObj := &otgvalidationhelpers.OTGValidation{Interface: nhVal}
		if err := nhValObj.IsIPv4Interfaceresolved(t, ate); err != nil {
			t.Errorf("IPv4 next-hop resolution: %v", err)
		}
		if err := nhValObj.IsIPv6Interfaceresolved(t, ate); err != nil {
			t.Errorf("IPv6 next-hop resolution: %v", err)
		}

		// Clear ARP and IPv6 neighbors on DUT to verify dynamic resolution during traffic.
		clearNeighbors(t, dut)

		runFlows(t, ate, []string{"mplsogue-ipv4", "mplsogue-ipv6"})
		otgutils.LogFlowMetrics(t, ate.OTG(), top)
		if err := v4Val.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("IPv4 decap flow: %v", err)
		}
		if err := v6Val.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("IPv6 decap flow: %v", err)
		}
	})

	t.Run("PF-1.25.v6: Validate static MPLS LSP push using an IPv6 next-hop", func(t *testing.T) {
		pIngressPush := dut.Port(t, ingressPorts[0])
		pCapture := ate.Port(t, egressLagPorts[0])
		top.Flows().Clear()

		// Plain IPv6 flow from Ingress Port 1 destined to the FEC subnet the DUT matches for the
		// push LSP. The DUT pushes the MPLS label and forwards to the ATE Egress LAG next-hop.
		f := &otgconfighelpers.Flow{
			FlowName:      "v6-push",
			TxNames:       []string{ateIngress1.Name + ".IPv6"},
			RxNames:       []string{ateDst.Name + ".IPv6"},
			FrameSize:     512,
			PpsRate:       trafficPPS,
			PacketsToSend: trafficPackets,
			EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: ateIngress1.MAC},
			IPv6Flow:      &otgconfighelpers.IPv6FlowParams{IPv6Src: ateIngress1.IPv6, IPv6SrcCount: 1000, IPv6Dst: pushFECDstIP},
			UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPSrcPort: 10000, UDPSrcCount: 1000, UDPDstPort: 5000},
		}
		f.CreateFlow(top)
		f.AddEthHeader()
		f.AddIPv6Header()
		f.AddUDPHeader()

		pv := &packetvalidationhelpers.PacketValidation{
			PortName:    pCapture.ID(),
			CaptureName: "v6-mpls-push",
			Validations: []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateMPLSLayer},
			MPLSLayer:   &packetvalidationhelpers.MPLSLayer{Label: mplsPushLabelV6},
		}
		packetvalidationhelpers.ConfigurePacketCapture(t, top, pv)
		pushAndResolve(t, ate, top)

		// Validate IPv6 Neighbor Discovery (ND) resolution on DUT telemetry leaf.
		verifyIPv6NeighborTelemetry(t, dut, aggID, ateDst.MAC)

		configureIPv6PushLSP(t, dut, pIngressPush.Name())

		cs := packetvalidationhelpers.StartCapture(t, ate)
		defer packetvalidationhelpers.StopCapture(t, ate, cs)
		runFlows(t, ate, []string{"v6-push"})
		otgutils.LogFlowMetrics(t, ate.OTG(), top)

		pushVal := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "v6-push", TolerancePct: tolerance * 100}}
		if err := pushVal.ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("IPv6 push flow: %v", err)
		}
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, pv); err != nil {
			t.Errorf("MPLS label push validation: %v", err)
		}
	})
}
