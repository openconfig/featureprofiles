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

// pushAndResolve pushes the OTG config, starts protocols, and waits for neighbor resolution.
func pushAndResolve(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	otgObj := ate.OTG()
	otgObj.PushConfig(t, top)
	otgObj.StartProtocols(t)
	otgutils.WaitForARP(t, otgObj, top, "IPv4")
	otgutils.WaitForARP(t, otgObj, top, "IPv6")
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
			cs26 := custSubinterfaces[6] // VLAN 26
			for _, tc := range []struct {
				name, family, address, flow, txName, txMAC, rxName string
				prefixLen                                          uint8
				label                                              uint32
				inner                                              *otgconfighelpers.Flow
			}{
				{
					name: "IPv4 address with IPv6 traffic", family: "IPv4",
					address: cs26.dutIPv4, prefixLen: cs26.ipv4Len,
					flow: "mplsogue-cust-v6-vlan26", label: cs26.mplsV6Label,
					txName: ateIngress2.Name + ".IPv4", txMAC: ateIngress2.MAC,
					rxName: "cust-vlan26.IPv6", inner: innerIPv6Flow(),
				},
				{
					name: "IPv6 address with IPv4 traffic", family: "IPv6",
					address: cs26.dutIPv6, prefixLen: cs26.ipv6Len,
					flow: "mplsogue-cust-v4-vlan26", label: cs26.mplsV4Label,
					txName: ateIngress1.Name + ".IPv4", txMAC: ateIngress1.MAC,
					rxName: "cust-vlan26.IPv4", inner: innerIPv4Flow(),
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					top.Flows().Clear()
					createMPLSoGUEFlow(top, tc.flow, tc.txName, tc.txMAC, tc.label, tc.inner, tc.rxName)
					top.Flows().Items()[0].Duration().Continuous()
					pushAndResolve(t, ate, top)
					runFlowsDuringChange(t, ate, []string{tc.flow}, func() {
						logCustomerForwardingState(t, dut, aggID, cs26, "before address deletion")
						// Register restoration before deleting so a failed change does
						// not leak the missing address into the following scenario.
						restored := false
						defer func() {
							if !restored {
								addSubinterfaceAddress(t, dut, aggID, cs26.vlanID, tc.address, tc.prefixLen, tc.family)
							}
						}()
						removeSubinterfaceAddress(t, dut, aggID, cs26.vlanID, tc.address, tc.family)
						logCustomerForwardingState(t, dut, aggID, cs26, "address deleted")
						addSubinterfaceAddress(t, dut, aggID, cs26.vlanID, tc.address, tc.prefixLen, tc.family)
						restored = true
						logCustomerForwardingState(t, dut, aggID, cs26, "address restored")
					})
					otgutils.LogFlowMetrics(t, ate.OTG(), top)
					val := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: tc.flow, TolerancePct: tolerance * 100}}
					if err := val.ValidateLossOnFlows(t, ate); err != nil {
						t.Errorf("Flow %s during %s address churn: %v", tc.flow, tc.family, err)
					}
				})
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
			top.Flows().Items()[0].Duration().Continuous()
			pushAndResolve(t, ate, top)
			runFlowsDuringChange(t, ate, []string{"mplsogue-cust-v6-vlan26"}, func() {
				logCustomerForwardingState(t, dut, aggID, custSubinterfaces[6], "before LSP deletion")
				restored := false
				defer func() {
					if !restored {
						setStaticLSP(t, dut, v4LSP)
					}
				}()
				deleteStaticLSP(t, dut, v4LSP)
				logCustomerForwardingState(t, dut, aggID, custSubinterfaces[6], "LSP deleted")
				setStaticLSP(t, dut, v4LSP)
				restored = true
				logCustomerForwardingState(t, dut, aggID, custSubinterfaces[6], "LSP restored")
			})
			otgutils.LogFlowMetrics(t, ate.OTG(), top)
			v6Val26 := &otgvalidationhelpers.OTGValidation{Flow: &otgvalidationhelpers.FlowParams{Name: "mplsogue-cust-v6-vlan26", TolerancePct: tolerance * 100}}
			if err := v6Val26.ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("IPv6 decap flow during IPv4 decap config churn: %v", err)
			}

			// Remove and re-add IPv6 static LSP decap config; verify IPv4 traffic unaffected.
			top.Flows().Clear()
			createMPLSoGUEFlow(top, "mplsogue-cust-v4-vlan26", ateIngress1.Name+".IPv4", ateIngress1.MAC, 99926, innerIPv4Flow(), "cust-vlan26.IPv4")
			top.Flows().Items()[0].Duration().Continuous()
			pushAndResolve(t, ate, top)
			runFlowsDuringChange(t, ate, []string{"mplsogue-cust-v4-vlan26"}, func() {
				logCustomerForwardingState(t, dut, aggID, custSubinterfaces[6], "before LSP deletion")
				restored := false
				defer func() {
					if !restored {
						setStaticLSP(t, dut, v6LSP)
					}
				}()
				deleteStaticLSP(t, dut, v6LSP)
				logCustomerForwardingState(t, dut, aggID, custSubinterfaces[6], "LSP deleted")
				setStaticLSP(t, dut, v6LSP)
				restored = true
				logCustomerForwardingState(t, dut, aggID, custSubinterfaces[6], "LSP restored")
			})
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
			t.Cleanup(func() {
				if value, present := previous.Val(); present {
					gnmi.Update(t, dut, ttlPath.Config(), value)
				} else {
					gnmi.Delete(t, dut, ttlPath.Config())
				}
			})
			gnmi.Update(t, dut, ttlPath.Config(), false)
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
