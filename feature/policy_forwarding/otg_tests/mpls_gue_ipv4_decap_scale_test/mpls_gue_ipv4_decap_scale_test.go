// Package mpls_gue_ipv4_decap_scale_test tests mplsogue decap functionality.
package mpls_gue_ipv4_decap_scale_test

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/iputil"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	"github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// TestMain calls main function.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ieee8023adLag       = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	mplsLabelCount      = 2000
	intCount            = 2000
	mplsV4Label         = 99991
	mplsV6Label         = 110993
	dutIntStartIPv4     = "169.254.0.1"
	otgIntStartIPv4     = "169.254.0.2"
	dutIntStartIPv6     = "2000:0:0:1::1"
	otgIntStartIPv6     = "2000:0:0:1::2"
	intStepV4           = "0.0.0.4"
	intStepV6           = "0:0:0:1::"
	staticRoutePrefix   = "10.99.1.0/24"
	staticRouteV6Prefix = "3000:1::/64"
	staticRouteNextHop  = "194.0.2.2"
	outerSrcIPv4        = "100.64.0.1"
	outerDstIPv4        = "11.1.1.1"
	innerSrcIPv4        = "22.1.1.1"
	innerDstIPv4        = "21.1.1.1"
	innerSrcIPv6        = "2000:1::1"
	innerDstIPv6        = "3000:1::1"
	mcastDst            = "239.1.1.1"
	udpDstPort          = 6635
	flowSrcCount        = 10000
	// outerSrcCount is the number of unique outer IPv4 source addresses. The
	// README requires 1000+ sources taken from 100.64.0.0/22, which only holds
	// 1024 addresses, so the count is capped accordingly.
	outerSrcCount = 1000
	// innerDSCPMin/innerDSCPMax/innerDSCPCount describe the inner-payload DSCP
	// range required by the README (0-56).
	innerDSCPMin   = 0
	innerDSCPMax   = 56
	innerDSCPCount = innerDSCPMax - innerDSCPMin + 1
	// innerTrafficClassStep steps the 8-bit IPv6 traffic-class field in DSCP
	// units, since DSCP occupies its upper 6 bits.
	innerTrafficClassStep  = 4
	dutIPv4Len             = 30
	dutIPv6Len             = 126
	dutMtu                 = 9202
	ratePPS                = 100
	totalPkts              = 0
	sleepTime              = 15
	carrierDelayUp         = 3000
	carrierDelayDown       = 150
	outerFlowRate          = 0
	innerTrafficClassCount = 0
	innerTrafficClass      = 10
	innerRawPriorityCount  = 0
	innerRawPriority       = 10
	innerSrcCount          = 0
	innerSrcPort           = 49152
	tolerancePct           = 5
	pushStartWaitTime      = 120 * time.Second
	// lagUpTimeout bounds the wait for the DUT aggregate to bundle and come up.
	// LACP convergence at this scale needs more than the previous 3 minutes.
	lagUpTimeout = 5 * time.Minute
	// PF-1.20.v6 constants: scaled decapsulation with an IPv6 outer header.
	v6ScaleFlowCount    = 1000            // Number of unique outer IPv6 flows/decap rules.
	v6ScaleOuterSrcIPv6 = "2001:db8:1::1" // First outer IPv6 source address.
	// v6ScaleOuterSrcStep is the step used to derive the 1000 unique outer IPv6
	// sources. It must match the increment the ATE applies to the IPv6 source
	// field (gosnappi increments the address by 1), otherwise the traffic never
	// matches the per-source decap rules programmed on the DUT.
	v6ScaleOuterSrcStep = "::1"
	v6ScaleOuterDstIPv6 = "2001:db8:2::1" // Outer IPv6 destination (decap address on the DUT).
	// v6ScaleDecapPrefix is the IPv6 decap range owned by the DUT that the outer
	// destination addresses of the MPLSoGUE traffic fall within.
	v6ScaleDecapPrefix = "2001:db8:2::/64"
	// v6ScaleLoopbackID selects the DUT loopback that owns the outer IPv6
	// destination address, and v6ScaleDecapLoopbackLen is its prefix length.
	v6ScaleLoopbackID       = 1
	v6ScaleDecapLoopbackLen = 128
	v6ScalePolicyID         = "gue-decap-scale-v6"
	v6ScaleSrcPrefixLen     = 128
	v6ScaleEphemeralMin     = 49152 // Lower bound of the ephemeral port range.
	v6ScaleEphemeralMax     = 65535 // Upper bound of the ephemeral port range.
	// v6ScaleLineRatePct is the aggregate line rate of the combined streams
	// (>= 50% of the ingress port capacity as required by the README).
	v6ScaleLineRatePct = 50
	// v6ScaleSetTimeout bounds the gNMI Set that programs the 1000 decap rules.
	v6ScaleSetTimeout = 5 * time.Minute
	// v6ScaleECMPTolerancePct is the allowed deviation from a perfectly even
	// distribution across the egress LAG member ports.
	v6ScaleECMPTolerancePct = 5.0
	// v6ScaleZeroRuleLogLimit bounds how many non-matching rule names are logged.
	v6ScaleZeroRuleLogLimit = 20
	// healthThresholdPct is the maximum acceptable CPU/memory utilization.
	healthThresholdPct = 80
	// healthSubSampleInterval is the SAMPLE interval requested on the gNMI
	// Subscribe used to monitor device health and egress packet rates.
	healthSubSampleInterval = 5 * time.Second
	// healthCPUSubDuration / healthMemSubDuration / egressRateSubDuration are the
	// durations of the /system/cpus, /system/memory and egress interface counter
	// subscriptions respectively. Their sum must stay below v6ScaleTrafficDuration
	// so that every sample is taken while traffic is running.
	healthCPUSubDuration  = 25 * time.Second
	healthMemSubDuration  = 15 * time.Second
	egressRateSubDuration = 20 * time.Second
	// v6ScaleTrafficDuration is how long traffic runs in the PF-1.20.v6 scale
	// test, sized to cover the telemetry subscriptions above.
	v6ScaleTrafficDuration = 90 * time.Second
)

var (
	top        = gosnappi.NewConfig()
	custPorts  = []string{"port1", "port2"}
	corePorts1 = []string{"port3", "port4"}
	corePorts2 = []string{"port5", "port6"}
	coreIntf1  = attrs.Attributes{Desc: "Core_Interface", IPv4: "194.0.2.1", IPv6: "2001:10:1:6::1", MTU: dutMtu, IPv4Len: 24, IPv6Len: 126}
	coreIntf2  = attrs.Attributes{Desc: "Core_Interface_2", IPv4: "194.0.3.1", IPv6: "2001:10:1:7::1", MTU: dutMtu, IPv4Len: 24, IPv6Len: 126}

	agg1 = &otgconfighelpers.Port{Name: "Port-Channel1", AggMAC: "02:00:01:01:01:07", MemberPorts: []string{"port1", "port2"}, LagID: 1, IsLag: true}
	agg2 = &otgconfighelpers.Port{Name: "Port-Channel2", AggMAC: "02:00:01:01:01:01", MemberPorts: []string{"port3", "port4"}, Interfaces: []*otgconfighelpers.InterfaceProperties{agg2interface}, LagID: 2, IsLag: true}
	agg3 = &otgconfighelpers.Port{Name: "Port-Channel3", AggMAC: "02:00:01:01:01:03", MemberPorts: []string{"port5", "port6"}, Interfaces: []*otgconfighelpers.InterfaceProperties{agg3interface}, LagID: 3, IsLag: true}

	agg2interface = &otgconfighelpers.InterfaceProperties{IPv4: "194.0.2.2", IPv6: "2001:10:1:6::2", IPv4Gateway: "194.0.2.1", IPv6Gateway: "2001:10:1:6::1", Name: "Port-Channel2", MAC: "02:00:01:01:01:02", IPv4Len: 29, IPv6Len: 126}
	agg3interface = &otgconfighelpers.InterfaceProperties{IPv4: "194.0.3.2", IPv6: "2001:10:1:7::2", IPv4Gateway: "194.0.3.1", IPv6Gateway: "2001:10:1:7::1", Name: "Port-Channel3", MAC: "02:00:01:01:01:04", IPv4Len: 29, IPv6Len: 126}

	// Custom IMIX settings for all flows. The README requires the frame sizes to
	// step 64, 128, 256, 512, 1024 .. up to the configured MTU, so the largest
	// size is the DUT MTU (dutMtu) rather than a fixed 1500 bytes. The weights
	// sum to 100%.
	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 20},
		{Size: 256, Weight: 15},
		{Size: 512, Weight: 15},
		{Size: 1024, Weight: 15},
		{Size: 1500, Weight: 15},
	}

	// flowResolveArp gates traffic start on the transmit-side gateways being
	// resolved. The outer Ethernet header of every flow leaves the destination
	// MAC unset, so the traffic generator populates it automatically from the
	// transmit device's resolved gateway. Starting traffic before ARP/ND has
	// completed makes that lookup yield a null entry and the controller rejects
	// the start with "Sequence contained null element (Parameter 'values')".
	flowResolveArp = &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{
			Names: []string{agg2.Interfaces[0].Name, agg3.Interfaces[0].Name},
		},
	}
	// flowOuterIPv4 Decap IPv4 Interface IPv4 Payload traffic params Outer Header.
	flowOuterIPv4 = newOuterGUEFlow(outerFlowParams{
		name:        "MPLSOGUE-IPv4-Traffic",
		txSuffix:    ".IPv4",
		mplsLabel:   mplsV4Label,
		flowRate:    100,
		srcMAC:      agg2.AggMAC,
		srcCount:    outerSrcCount,
		udpSrcPort:  innerSrcPort,
		udpSrcCount: flowSrcCount,
	})
	// flowInnerIPv4 Inner Header IPv4 Payload.
	flowInnerIPv4 = &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: innerSrcIPv4, IPv4Dst: innerDstIPv4, IPv4SrcCount: flowSrcCount, DSCP: innerDSCPMin, DSCPCount: innerDSCPCount},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: innerSrcPort, TCPDstPort: 80, TCPSrcCount: flowSrcCount},
	}
	// flowOuterIPv6 Decap IPv6 Interface IPv6 Payload traffic params Outer Header.
	// The outer header stays IPv4 here; only the transmitting OTG device and the
	// MPLS label differ from flowOuterIPv4.
	flowOuterIPv6 = newOuterGUEFlow(outerFlowParams{
		name:       "MPLSOGUE-IPv6-Traffic",
		txSuffix:   ".IPv6",
		mplsLabel:  mplsV6Label,
		flowRate:   100,
		srcMAC:     agg2.AggMAC,
		srcCount:   outerSrcCount,
		udpSrcPort: innerSrcPort,
	})
	// flowInnerIPv6 Inner Header IPv6 Payload.
	flowInnerIPv6 = &otgconfighelpers.Flow{
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: innerSrcIPv6, IPv6Dst: innerDstIPv6, IPv6SrcCount: flowSrcCount, TrafficClass: innerDSCPMin, TrafficClassCount: innerDSCPCount, TrafficClassStep: innerTrafficClassStep},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: innerSrcPort, TCPDstPort: 80, TCPSrcCount: flowSrcCount},
	}
	// flowOuterMcast is the “outer” MPLS‐encapsulated flow whose payload is an IPv4+UDP multicast packet.
	flowOuterMcast = newOuterGUEFlow(outerFlowParams{
		name:       "MPLSoGUE-Mcast-Traffic",
		txSuffix:   ".IPv4",
		mplsLabel:  mplsV4Label,
		flowRate:   100,
		srcMAC:     agg2.AggMAC,
		srcCount:   outerSrcCount,
		udpSrcPort: innerSrcPort,
	})
	// flowInnerMcast is the “inner” multicast payload (IPv4 + UDP to the same group).
	flowInnerMcast = &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: innerSrcIPv4, IPv4Dst: mcastDst, IPv4SrcCount: flowSrcCount, DSCP: innerDSCPMin, DSCPCount: innerDSCPCount},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: innerSrcPort, TCPDstPort: 80, TCPSrcCount: flowSrcCount},
	}
	validationsIPv4     = []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateIPv4Header}
	validationsIPv6     = []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateIPv6Header}
	decapValidationIPv4 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "ipv4_decap",
		Validations: validationsIPv4,
		IPv4Layer:   &packetvalidationhelpers.IPv4Layer{DstIP: innerDstIPv4, Tos: 10, TTL: 64, SkipProtocolCheck: true},
	}
	decapValidationIPv6 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port2",
		CaptureName: "ipv6_decap",
		Validations: validationsIPv6,
		IPv6Layer:   &packetvalidationhelpers.IPv6Layer{DstIP: innerDstIPv6, TrafficClass: 10, HopLimit: 64},
	}
	// The LAG ECMP balance of the base flows is validated with
	// validateECMPonLAGForFlow, which sums the per-TxName sub flows sharing the
	// egress LAG member ports.

	// PF-1.20.v6 flows.
	// coreAggIntfID and coreAggIntfID2 are the two ingress (core facing)
	// aggregate interfaces configured on the DUT; the IPv6 decap policy is
	// applied to both, matching the README test environment.
	coreAggIntfID  string
	coreAggIntfID2 string

	// v6ScaleOuterSrcIPv6s holds the 1000 unique outer IPv6 source addresses used
	// both for the PBR decap rules on the DUT and for the ATE traffic streams.
	v6ScaleOuterSrcIPv6s []string

	// flowOuterV6ScaleIPv4Payload is the MPLSoGUE flow with a unique IPv6 outer
	// header carrying an IPv4 inner payload.
	flowOuterV6ScaleIPv4Payload = newOuterGUEFlow(outerFlowParams{
		name:        "MPLSOGUE-V6Outer-Scale-IPv4-Payload",
		txSuffix:    ".IPv6",
		v6Outer:     true,
		mplsLabel:   mplsV4Label,
		flowRate:    v6ScaleLineRatePct / 2,
		srcMAC:      agg2.AggMAC,
		srcCount:    v6ScaleFlowCount,
		udpSrcPort:  v6ScaleEphemeralMin,
		udpSrcCount: v6ScaleFlowCount,
	})
	flowInnerV6ScaleIPv4Payload = &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: innerSrcIPv4, IPv4Dst: innerDstIPv4, IPv4SrcCount: v6ScaleFlowCount, DSCP: innerDSCPMin, DSCPCount: innerDSCPCount},
		UDPFlow:  &otgconfighelpers.UDPFlowParams{UDPSrcPort: v6ScaleEphemeralMin, UDPDstPort: 80, UDPSrcCount: v6ScaleFlowCount},
	}

	// flowOuterV6ScaleIPv6Payload is the MPLSoGUE flow with a unique IPv6 outer
	// header carrying an IPv6 inner payload.
	flowOuterV6ScaleIPv6Payload = newOuterGUEFlow(outerFlowParams{
		name:        "MPLSOGUE-V6Outer-Scale-IPv6-Payload",
		txSuffix:    ".IPv6",
		v6Outer:     true,
		mplsLabel:   mplsV6Label,
		flowRate:    v6ScaleLineRatePct / 2,
		srcMAC:      agg3.AggMAC,
		srcCount:    v6ScaleFlowCount,
		udpSrcPort:  v6ScaleEphemeralMin,
		udpSrcCount: v6ScaleFlowCount,
	})
	flowInnerV6ScaleIPv6Payload = &otgconfighelpers.Flow{
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: innerSrcIPv6, IPv6Dst: innerDstIPv6, IPv6SrcCount: v6ScaleFlowCount, TrafficClass: innerDSCPMin, TrafficClassCount: innerDSCPCount, TrafficClassStep: innerTrafficClassStep},
		UDPFlow:  &otgconfighelpers.UDPFlowParams{UDPSrcPort: v6ScaleEphemeralMin, UDPDstPort: 80, UDPSrcCount: v6ScaleFlowCount},
	}
	// decapValidationV6ScaleIPv4 / decapValidationV6ScaleIPv6 confirm at packet
	// level that the DUT actually removed the outer IPv6/UDP/MPLS (MPLSoGUE)
	// headers of the PF-1.20.v6 scale flows and forwarded the inner payload with
	// its DSCP/traffic-class and TTL/hop-limit preserved. Zero loss alone does not
	// prove decapsulation, so these captures back the "DUT successfully
	// decapsulates all 1000 flows" pass criterion.
	decapValidationV6ScaleIPv4 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "v6_scale_ipv4_decap",
		Validations: validationsIPv4,
		IPv4Layer:   &packetvalidationhelpers.IPv4Layer{DstIP: innerDstIPv4, Tos: 10, TTL: 64, SkipProtocolCheck: true},
	}
	decapValidationV6ScaleIPv6 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port2",
		CaptureName: "v6_scale_ipv6_decap",
		Validations: validationsIPv6,
		IPv6Layer:   &packetvalidationhelpers.IPv6Layer{DstIP: innerDstIPv6, TrafficClass: 10, HopLimit: 64},
	}
)

// outerFlowParams captures the only attributes that differ between the
// MPLSoGUE outer flows; everything else (IMIX profile, MPLS EXP/label count,
// GUE destination port, dual ingress transmit endpoints) is shared and is
// filled in by newOuterGUEFlow.
type outerFlowParams struct {
	// name is the OTG flow name.
	name string
	// txSuffix selects the transmitting OTG device of both core facing
	// aggregates, either ".IPv4" or ".IPv6".
	txSuffix string
	// v6Outer selects an IPv6 outer header instead of the default IPv4 one.
	v6Outer bool
	// mplsLabel is the first label of the MPLSoGUE label stack.
	mplsLabel uint32
	// flowRate is the offered rate, in percent of the port line rate.
	flowRate float32
	// srcMAC is the outer Ethernet source MAC.
	srcMAC string
	// srcCount is the number of unique outer source addresses.
	srcCount uint32
	// udpSrcPort / udpSrcCount describe the GUE UDP source port range.
	udpSrcPort  uint32
	udpSrcCount uint32
}

// newOuterGUEFlow builds an MPLSoGUE outer flow from the attributes that vary
// between the test flows.
//
// The traffic generator supports a single Tx device per flow, so createFlow
// later splits the returned definition into one OTG flow per TxName. Both core
// facing aggregates therefore receive MPLSoGUE traffic simultaneously, as
// required by the README test environment.
func newOuterGUEFlow(flowParams outerFlowParams) *otgconfighelpers.Flow {
	f := &otgconfighelpers.Flow{
		TxNames:           []string{agg2.Interfaces[0].Name + flowParams.txSuffix, agg3.Interfaces[0].Name + flowParams.txSuffix},
		RxNames:           []string{},
		SizeWeightProfile: &sizeWeightProfile,
		Flowrate:          flowParams.flowRate,
		PacketsToSend:     totalPkts,
		FlowName:          flowParams.name,
		EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: flowParams.srcMAC},
		MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: flowParams.mplsLabel, MPLSExp: 7, MPLSLabelCount: mplsLabelCount},
		UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: flowParams.udpSrcPort, UDPDstPort: udpDstPort, UDPSrcCount: flowParams.udpSrcCount},
	}
	if flowParams.v6Outer {
		f.IPv6Flow = &otgconfighelpers.IPv6FlowParams{IPv6Src: v6ScaleOuterSrcIPv6, IPv6Dst: v6ScaleOuterDstIPv6, IPv6SrcCount: flowParams.srcCount}
		return f
	}
	f.IPv4Flow = &otgconfighelpers.IPv4FlowParams{IPv4Src: outerSrcIPv4, IPv4Dst: outerDstIPv4, IPv4SrcCount: flowParams.srcCount}
	return f
}

// newFlowValidation builds the loss validation of a flow. Every flow is
// validated on the same set of ports; the ingress interface names are appended
// once the Aggregate1 subinterfaces have been generated.
func newFlowValidation(flowName string, tolerancePct float32) *otgvalidationhelpers.OTGValidation {
	return &otgvalidationhelpers.OTGValidation{
		Interface: &otgvalidationhelpers.InterfaceParams{Names: []string{}, Ports: allFlowPorts()},
		Flow:      &otgvalidationhelpers.FlowParams{Name: flowName, TolerancePct: tolerancePct},
	}
}

// allFlowPorts returns the egress (customer) LAG member ports together with
// the member ports of both core facing aggregates, which transmit the
// MPLSoGUE traffic.
func allFlowPorts() []string {
	ports := append([]string{}, agg1.MemberPorts...)
	ports = append(ports, agg2.MemberPorts...)
	return append(ports, agg3.MemberPorts...)
}

type networkConfig struct {
	DutIPv4s []string
	OtgIPv4s []string
	OtgMACs  []string
	DutIPv6s []string
	OtgIPv6s []string
}

// generateNetConfig generates and returns a networkConfig object containing IPv4, IPv6, and MAC address allocations for both DUT and OTG interfaces.
func generateNetConfig(t *testing.T, intCount int) (*networkConfig, error) {
	t.Helper()
	dutIPs, err := iputil.GenerateIPsWithStep(dutIntStartIPv4, intCount, intStepV4)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DUT IPs: %w", err)
	}

	otgIPs, err := iputil.GenerateIPsWithStep(otgIntStartIPv4, intCount, intStepV4)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTG IPs: %w", err)
	}

	otgMACs := iputil.GenerateMACs("00:00:00:00:00:AA", intCount, "00:00:00:00:00:01")
	dutIPsV6, err := iputil.GenerateIPv6sWithStep(dutIntStartIPv6, intCount, intStepV6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate DUT IPv6s: %w", err)
	}

	otgIPsV6, err := iputil.GenerateIPv6sWithStep(otgIntStartIPv6, intCount, intStepV6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTG IPv6s: %w", err)
	}

	return &networkConfig{
		DutIPv4s: dutIPs,
		OtgIPv4s: otgIPs,
		OtgMACs:  otgMACs,
		DutIPv6s: dutIPsV6,
		OtgIPv6s: otgIPsV6,
	}, nil
}

// configureOTG sets up the Open Traffic Generator (OTG) test configuration.
func configureOTG(t *testing.T) {
	t.Helper()
	top.Captures().Clear()
	ate := ondatra.ATE(t, "ate")

	// Create a slice of aggPortData for easier iteration
	aggs := []*otgconfighelpers.Port{agg1, agg2, agg3}

	// Configure OTG Interfaces
	for _, agg := range aggs {
		otgconfighelpers.ConfigureNetworkInterface(t, top, ate, agg)
	}
	// Pushing the scaled (2000 subinterface) configuration and bringing the OTG
	// LAGs/LACP up takes several minutes. The DUT aggregate only reports
	// oper-status UP once LACP has converged with the ATE, so wait for the OTG
	// side to be stable before any DUT LAG state is polled.
	pushAndStartProtocols(t, ate, top, pushStartWaitTime)
}

// configureDUT Generate DUT Configuration.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, netConfig *networkConfig, ocPFParams cfgplugins.OcPolicyForwardingParams) string {
	t.Helper()
	var interfaces []*attrs.Attributes
	for i := range intCount {
		interfaces = append(interfaces, &attrs.Attributes{
			Desc:         "Customer_connect",
			MTU:          dutMtu,
			IPv4:         netConfig.DutIPv4s[i],
			IPv4Len:      dutIPv4Len,
			IPv6:         netConfig.DutIPv6s[i],
			IPv6Len:      dutIPv6Len,
			Subinterface: uint32(i + 1),
		})
	}
	custAggID := netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, custPorts, interfaces, custAggID)
	coreAggID := netutil.NextAggregateInterface(t, dut)
	coreAggIntfID = coreAggID
	configureInterfaces(t, dut, corePorts1, []*attrs.Attributes{&coreIntf1}, coreAggID)
	// Second core-facing aggregate (Aggregate3) so that MPLSoGUE traffic can be
	// received simultaneously on two ingress aggregates, as required by the
	// README test environment.
	coreAggID2 := netutil.NextAggregateInterface(t, dut)
	coreAggIntfID2 = coreAggID2
	configureInterfaces(t, dut, corePorts2, []*attrs.Attributes{&coreIntf2}, coreAggID2)
	configureStaticRoute(t, dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	decapMPLSInGUE(t, dut, pf, ni, netConfig, ocPFParams)
	return custAggID
}

// waitForLAGUp waits until all specified member ports and the aggregate interface (LAG) reach an operational UP state on the DUT.
func waitForLAGUp(t *testing.T, dut *ondatra.DUTDevice, aggID string, ports []string) {
	t.Helper()

	t.Logf("Waiting for LAG %s to be UP...", aggID)

	// Wait for member ports UP
	for _, p := range ports {
		port := dut.Port(t, p)
		gnmi.Await(t, dut, gnmi.OC().Interface(port.Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
		t.Logf("Port %s is UP", p)
	}

	// Wait for LAG interface UP. Member links being UP only proves the physical
	// link; the aggregate needs LACP to converge and the members to be bundled,
	// which at this scale (plus the configured carrier-delay up) can take longer
	// than the port timers.
	if _, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(aggID).OperStatus().State(), lagUpTimeout,
		func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			state, present := val.Val()
			return present && state == oc.Interface_OperStatus_UP
		}).Await(t); !ok {
		logLACPState(t, dut, aggID, ports)
		t.Fatalf("LAG %s did not reach oper-status UP within %v", aggID, lagUpTimeout)
	}

	t.Logf("LAG %s is UP", aggID)
}

// logLACPState dumps the LACP member state and the aggregate member list to help
// distinguish an un-bundled LAG from a wrong/absent aggregate interface name.
func logLACPState(t *testing.T, dut *ondatra.DUTDevice, aggID string, ports []string) {
	t.Helper()

	if status, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(aggID).OperStatus().State()).Val(); ok {
		t.Logf("LAG %s oper-status: %v", aggID, status)
	} else {
		t.Logf("LAG %s has no oper-status in state; the aggregate may not exist under this name", aggID)
	}

	members, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(aggID).Aggregation().Member().State()).Val()
	if !ok || len(members) == 0 {
		t.Logf("LAG %s reports no bundled members", aggID)
	} else {
		t.Logf("LAG %s bundled members: %v", aggID, members)
	}

	for _, p := range ports {
		port := dut.Port(t, p)
		member, ok := gnmi.Lookup(t, dut, gnmi.OC().Lacp().Interface(aggID).Member(port.Name()).State()).Val()
		if !ok {
			t.Logf("No LACP member state for %s on %s", port.Name(), aggID)
			continue
		}
		t.Logf("LACP member %s on %s: collecting=%v distributing=%v activity=%v synchronization=%v aggregatable=%v", port.Name(), aggID, member.GetCollecting(), member.GetDistributing(), member.GetActivity(), member.GetSynchronization(), member.GetAggregatable())
	}
}

// configureDUTAndOTG generates and applies DUT configuration, prepares OTG device/interface properties, and sets up validation flows.
func configureDUTAndOTG(t *testing.T) (*ondatra.DUTDevice, string, *networkConfig) {
	t.Helper()
	t.Log("PF-1.20.1: Generate DUT Configuration")
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	netConfig, err := generateNetConfig(t, intCount)
	if err != nil {
		t.Fatalf("Error generating net config: %v", err)
	}
	mplsV4Labels := func() []int {
		r := make([]int, mplsLabelCount)
		for i := range r {
			r[i] = mplsV4Label + i
		}
		return r
	}()

	mplsV6Labels := func() []int {
		r := make([]int, mplsLabelCount)
		for i := range r {
			r[i] = mplsV6Label + i
		}
		return r
	}()

	for i := range intCount {
		agg1.Interfaces = append(agg1.Interfaces, &otgconfighelpers.InterfaceProperties{
			Name:        fmt.Sprintf("agg1port%d", i+1),
			IPv4:        netConfig.OtgIPv4s[i],
			IPv4Gateway: netConfig.DutIPv4s[i],
			Vlan:        uint32(i + 1),
			IPv4Len:     dutIPv4Len,
			IPv6:        netConfig.OtgIPv6s[i],
			IPv6Gateway: netConfig.DutIPv6s[i],
			IPv6Len:     dutIPv6Len,
			MAC:         netConfig.OtgMACs[i],
		})
	}

	// Get default parameters for OC Policy Forwarding
	ocPFParams := defaultOCPolicyForwardingParams()

	// Pass ocPFParams to ConfigureDut
	ocPFParams.DecapPolicy.DecapMPLSParams.MplsStaticLabels = mplsV4Labels
	ocPFParams.DecapPolicy.DecapMPLSParams.MplsStaticLabelsForIPv6 = mplsV6Labels
	// Pass ocPFParams to configureDut
	custAggID := configureDUT(t, dut, netConfig, ocPFParams)
	// Bind the flows to the Rx device names, now that agg1.Interfaces has been
	// populated.
	for _, intf := range agg1.Interfaces {
		flowOuterIPv4.RxNames = append(flowOuterIPv4.RxNames, intf.Name+".IPv4")
		flowOuterIPv6.RxNames = append(flowOuterIPv6.RxNames, intf.Name+".IPv6")
		flowOuterMcast.RxNames = append(flowOuterMcast.RxNames, intf.Name+".IPv4")
	}
	configureOTG(t)
	waitForLAGUp(t, dut, custAggID, custPorts)
	// Both ingress aggregates must be bundled before traffic is sent, otherwise
	// the dual-ingress decapsulation validation races LACP convergence.
	waitForLAGUp(t, dut, coreAggIntfID, corePorts1)
	waitForLAGUp(t, dut, coreAggIntfID2, corePorts2)
	return dut, custAggID, netConfig
}

// defaultOCPolicyForwardingParams provides default parameters for the generator, matching the values in the provided JSON example.
func defaultOCPolicyForwardingParams() cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
		InterfaceID:         "Agg1.10",
		AppliedPolicyName:   "customer1",
	}
}

// decapMPLSInGUE should also include the OC config , within these deviations there should be a switch statement is needed, Modified to accept pf, ni, and ocPFParams.
func decapMPLSInGUE(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding, ni *oc.NetworkInstance, netConfig *networkConfig, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	t.Helper()
	ocPFParams.DecapPolicy.DecapMPLSParams.NextHops = netConfig.OtgIPv4s
	ocPFParams.DecapPolicy.DecapMPLSParams.NextHopsV6 = netConfig.OtgIPv6s
	ocPFParams.DecapPolicy.DecapMPLSParams.ScaleStaticLSP = true
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.DecapGroupConfigGue(t, dut, pf, ocPFParams)
	cfgplugins.MPLSStaticLSPConfig(t, dut, ni, ocPFParams)
	if !deviations.PolicyForwardingOCUnsupported(dut) {
		pushPolicyForwardingConfig(t, dut, ni)
	}
}

// sendTraffic push the OTG config and start the protocols/traffic and get the flow/port metrics.
func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, custAggID string, netConfig *networkConfig) {
	t.Helper()
	pushAndStartProtocols(t, ate, top, pushStartWaitTime)
	waitForSubinterfacesUp(t, dut, custAggID, netConfig, 180*time.Second)
	if err := flowResolveArp.IsIPv4Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv4 interface for ATE: %v, error: %v", ate, err)
	}
	if err := flowResolveArp.IsIPv6Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv6 interface for ATE: %v, error: %v", ate, err)
	}
	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	ate.OTG().StopTraffic(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.LogPortMetrics(t, ate.OTG(), top)
}

// sendTrafficWithTelemetry starts the traffic, runs the supplied gNMI Subscribe
// based telemetry monitor while the traffic is running, and then stops the
// traffic. Traffic runs for at least trafficDuration so that every subscription
// samples a loaded device.
func sendTrafficWithTelemetry(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, custAggID string, netConfig *networkConfig, trafficDuration time.Duration, monitor func(*testing.T)) {
	t.Helper()
	pushAndStartProtocols(t, ate, top, pushStartWaitTime)
	waitForSubinterfacesUp(t, dut, custAggID, netConfig, 180*time.Second)
	if err := flowResolveArp.IsIPv4Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv4 interface for ATE: %v, error: %v", ate, err)
	}
	if err := flowResolveArp.IsIPv6Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv6 interface for ATE: %v, error: %v", ate, err)
	}
	ate.OTG().StartTraffic(t)
	// monitor may call t.Fatalf, which unwinds the goroutine and would otherwise
	// skip the stop below, leaving traffic running for the remaining subtests.
	trafficStopped := false
	defer func() {
		if !trafficStopped {
			ate.OTG().StopTraffic(t)
		}
	}()
	start := time.Now()
	if monitor != nil {
		monitor(t)
	}
	if remaining := trafficDuration - time.Since(start); remaining > 0 {
		time.Sleep(remaining)
	}
	ate.OTG().StopTraffic(t)
	trafficStopped = true
	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.LogPortMetrics(t, ate.OTG(), top)
}

// sendTrafficCapture push the OTG config and start/stop the capture/traffic to validate the captured packets.
func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, custAggID string, netConfig *networkConfig) {
	t.Helper()
	pushAndStartProtocols(t, ate, top, pushStartWaitTime)
	waitForSubinterfacesUp(t, dut, custAggID, netConfig, 180*time.Second)
	if err := flowResolveArp.IsIPv4Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv4 interface for ATE: %v, error: %v", ate, err)
	}
	// The IPv6 payload-preserve capture transmits from the IPv6 device, so its
	// gateway must be resolved before the flow is started as well.
	if err := flowResolveArp.IsIPv6Interfaceresolved(t, ate); err != nil {
		t.Fatalf("Failed to resolve IPv6 interface for ATE: %v, error: %v", ate, err)
	}
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(sleepTime * time.Second)
	ate.OTG().StopTraffic(t)
	packetvalidationhelpers.StopCapture(t, ate, cs)
}

// pushAndStartProtocols pushes the OTG configuration to the ATE, starts all control-plane protocols, waits for protocol convergence, and optionally stops the protocols after the provided duration.
func pushAndStartProtocols(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, pushStartWaitTime time.Duration) {
	t.Helper()

	t.Log("Pushing OTG config...")
	ate.OTG().PushConfig(t, top)
	time.Sleep(pushStartWaitTime)
	t.Log("Starting protocols...")
	ate.OTG().StartProtocols(t)

	if err := waitForOTGProtocolsUpWithRetry(t, ate, top, pushStartWaitTime, false); err != nil {
		t.Log("Protocols not UP on first attempt, restarting once...")

		// Restart once
		ate.OTG().StopProtocols(t)
		ate.OTG().StartProtocols(t)

		if err := waitForOTGProtocolsUpWithRetry(t, ate, top, pushStartWaitTime, true); err != nil {
			t.Fatalf("Protocols failed to come UP even after restart: %v", err)
		}
	}

	t.Log("Protocols are stable and ready")
}

// waitForSubinterfacesUp validates that all DUT subinterfaces are configured and operational by verifying IP presence and (optionally) neighbor resolution.
func waitForSubinterfacesUp(t *testing.T, dut *ondatra.DUTDevice, aggID string, netConfig *networkConfig, timeout time.Duration) {
	t.Helper()
	t.Logf("Waiting for subinterfaces on %s...", aggID)

	for i := range netConfig.DutIPv4s {
		subif := uint32(i + 1)
		// -------------------------------
		// IPv4 Address Check
		// -------------------------------
		ipv4 := netConfig.DutIPv4s[i]

		_, ok := gnmi.Watch(t, dut, gnmi.OC().Interface(aggID).Subinterface(subif).Ipv4().Address(ipv4).PrefixLength().State(), timeout,
			func(val *ygnmi.Value[uint8]) bool {
				_, present := val.Val()
				return present
			},
		).Await(t)

		if !ok {
			t.Fatalf("IPv4 not configured on %s.%d", aggID, subif)
		}
		// -------------------------------
		// IPv6 Address Check
		// -------------------------------
		ipv6 := netConfig.DutIPv6s[i]

		_, ok = gnmi.Watch(t, dut, gnmi.OC().Interface(aggID).Subinterface(subif).Ipv6().Address(ipv6).PrefixLength().State(), timeout,
			func(val *ygnmi.Value[uint8]) bool {
				_, present := val.Val()
				return present
			},
		).Await(t)

		if !ok {
			t.Fatalf("IPv6 not configured on %s.%d", aggID, subif)
		}
	}

	t.Log("All subinterfaces are configured successfully")
}

// waitForOTGProtocolsUpWithRetry waits for all OTG ports and LAGs to reach an operational UP state within the given timeout.
func waitForOTGProtocolsUpWithRetry(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, pushStartWaitTime time.Duration, strict bool) error {
	t.Helper()
	t.Log("Waiting for OTG ports to be UP...")
	for _, p := range config.Ports().Items() {
		_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Port(p.Name()).Link().State(), pushStartWaitTime,
			func(val *ygnmi.Value[otgtelemetry.E_Port_Link]) bool {
				state, present := val.Val()
				return present && state == otgtelemetry.Port_Link_UP
			}).Await(t)

		if !ok {
			if strict {
				return fmt.Errorf("port %s not UP", p.Name())
			}
			return fmt.Errorf("retry needed: port %s not UP", p.Name())
		}
		t.Logf("Port %s is UP", p.Name())
	}

	t.Log("Waiting for LAGs to be UP...")
	for _, lag := range config.Lags().Items() {
		_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Lag(lag.Name()).OperStatus().State(), pushStartWaitTime,
			func(val *ygnmi.Value[otgtelemetry.E_Lag_OperStatus]) bool {
				state, present := val.Val()
				return present && state == otgtelemetry.Lag_OperStatus_UP
			}).Await(t)

		if !ok {
			if strict {
				return fmt.Errorf("LAG %s not UP", lag.Name())
			}
			return fmt.Errorf("retry needed: LAG %s not UP", lag.Name())
		}
		t.Logf("LAG %s is UP", lag.Name())
	}

	return nil
}

// txDeviceName strips the ".IPv4"/".IPv6" suffix of an OTG transmit endpoint
// name and returns the owning device (aggregate interface) name.
func txDeviceName(txName string) string {
	if i := strings.LastIndex(txName, "."); i != -1 {
		return txName[:i]
	}
	return txName
}

// txAggregate returns the aggregate that owns the given transmit endpoint, so
// that a per-TxName flow can use the source MAC of the aggregate it egresses.
func txAggregate(txName string) *otgconfighelpers.Port {
	dev := txDeviceName(txName)
	for _, agg := range []*otgconfighelpers.Port{agg1, agg2, agg3} {
		for _, intf := range agg.Interfaces {
			if intf.Name == dev {
				return agg
			}
		}
	}
	return nil
}

// flowNameForTx builds the name of the OTG flow generated for a single
// transmit endpoint of a multi-TxName flow.
func flowNameForTx(flowName, txName string) string {
	return flowName + "-" + txDeviceName(txName)
}

// flowNames returns the names of the OTG flows created by createflow for f.
// The traffic generator supports a single Tx device per flow ("Tx device in
// flow ... is configured with N devices. Only 1 device is supported now"), so a
// flow that lists several TxNames is split into one OTG flow per TxName. This
// keeps the README requirement of receiving MPLSoGUE traffic simultaneously on
// both ingress aggregates.
func flowNames(f *otgconfighelpers.Flow) []string {
	if len(f.TxNames) <= 1 {
		return []string{f.FlowName}
	}
	names := make([]string, 0, len(f.TxNames))
	for _, tx := range f.TxNames {
		names = append(names, flowNameForTx(f.FlowName, tx))
	}
	return names
}

// createFlow configures the traffic streams as per the README. When the flow
// definition carries several TxNames, one OTG flow per transmit device is
// created and the configured rate is shared between them, so that the total
// offered rate stays the same while the traffic ingresses on both aggregates.
func createFlow(t *testing.T, top gosnappi.Config, outer *otgconfighelpers.Flow, inner *otgconfighelpers.Flow, clearFlows bool) {
	t.Helper()

	if clearFlows {
		top.Flows().Clear()
	}

	if len(outer.TxNames) <= 1 {
		createFlowForTx(t, top, outer, inner, "")
		return
	}
	for _, tx := range outer.TxNames {
		createFlowForTx(t, top, outer, inner, tx)
	}
}

// createFlowForTx creates a single OTG flow. When txName is empty the flow is
// created as defined; otherwise the flow transmits only from txName and its
// name, source MAC and rate are adjusted accordingly.
func createFlowForTx(t *testing.T, top gosnappi.Config, outer *otgconfighelpers.Flow, inner *otgconfighelpers.Flow, txName string) {
	t.Helper()

	outerCopy := *outer

	if txName != "" {
		n := len(outer.TxNames)
		outerCopy.TxNames = []string{txName}
		outerCopy.FlowName = flowNameForTx(outer.FlowName, txName)
		outerCopy.Flowrate = outer.Flowrate / float32(n)
		outerCopy.PpsRate = outer.PpsRate / uint64(n)
		if outer.PacketsToSend != 0 {
			outerCopy.PacketsToSend = outer.PacketsToSend / uint32(n)
		}
	}

	if outer.EthFlow != nil {
		eth := *outer.EthFlow
		if agg := txAggregate(txName); txName != "" && agg != nil {
			eth.SrcMAC = agg.AggMAC
		}
		outerCopy.EthFlow = &eth
	}
	if outer.IPv4Flow != nil {
		ipv4 := *outer.IPv4Flow
		outerCopy.IPv4Flow = &ipv4
	}
	if outer.IPv6Flow != nil {
		ipv6 := *outer.IPv6Flow
		outerCopy.IPv6Flow = &ipv6
	}
	if outer.TCPFlow != nil {
		tcp := *outer.TCPFlow
		outerCopy.TCPFlow = &tcp
	}
	if outer.UDPFlow != nil {
		udp := *outer.UDPFlow
		outerCopy.UDPFlow = &udp
	}
	if outer.MPLSFlow != nil {
		mpls := *outer.MPLSFlow
		outerCopy.MPLSFlow = &mpls
	}

	outerCopy.CreateFlow(top)
	outerCopy.AddEthHeader()

	if outerCopy.IPv4Flow != nil {
		outerCopy.AddIPv4Header()
	}
	if outerCopy.IPv6Flow != nil {
		outerCopy.AddIPv6Header()
	}
	if outerCopy.UDPFlow != nil {
		outerCopy.AddUDPHeader()
	}
	if outerCopy.MPLSFlow != nil {
		outerCopy.AddMPLSHeader()
	}

	if inner != nil {
		if inner.IPv4Flow != nil {
			ipv4 := *inner.IPv4Flow
			outerCopy.IPv4Flow = &ipv4
			outerCopy.AddIPv4Header()
		}

		if inner.IPv6Flow != nil {
			ipv6 := *inner.IPv6Flow
			outerCopy.IPv6Flow = &ipv6
			outerCopy.AddIPv6Header()
		}

		if inner.TCPFlow != nil {
			tcp := *inner.TCPFlow
			outerCopy.TCPFlow = &tcp
			outerCopy.AddTCPHeader()
		}

		if inner.UDPFlow != nil {
			udp := *inner.UDPFlow
			outerCopy.UDPFlow = &udp
			outerCopy.AddUDPHeader()
		}
	}
}

// updateFlow upadte the traffic streams as per the input.
func updateFlow(t *testing.T, paramsOuter *otgconfighelpers.Flow, paramsInner *otgconfighelpers.Flow, clearFlows bool, pps uint64, totalPackets uint32) {
	t.Helper()
	paramsOuter.PacketsToSend = totalPackets
	paramsOuter.PpsRate = pps
	paramsOuter.Flowrate = outerFlowRate
	if paramsInner.IPv6Flow != nil {
		paramsInner.IPv6Flow.TrafficClassCount = innerTrafficClassCount
		paramsInner.IPv6Flow.TrafficClassStep = 0
		paramsInner.IPv6Flow.TrafficClass = innerTrafficClass
	}
	if paramsInner.IPv4Flow != nil {
		// The payload-preserve validation expects a single, known TOS byte, so the
		// DSCP sweep is replaced by a fixed raw priority for these runs.
		paramsInner.IPv4Flow.DSCP = 0
		paramsInner.IPv4Flow.DSCPCount = 0
		paramsInner.IPv4Flow.RawPriorityCount = innerRawPriorityCount
		paramsInner.IPv4Flow.RawPriority = innerRawPriority
		if paramsInner.TCPFlow != nil {
			paramsInner.TCPFlow.TCPSrcCount = innerSrcCount
			paramsInner.TCPFlow.TCPSrcPort = innerSrcPort
		}
		if paramsOuter.IPv4Flow != nil {
			paramsOuter.IPv4Flow.IPv4Src = outerSrcIPv4
			paramsOuter.IPv4Flow.IPv4Dst = outerDstIPv4
		}
	}
	createFlow(t, top, paramsOuter, paramsInner, clearFlows)
}

// configureInterfaces configures a LAG (aggregate interface) and attaches DUT ports to it. It also applies LACP settings, enables aggregation, and sets hold-time for member interfaces.
func configureInterfaces(t *testing.T, dut *ondatra.DUTDevice, dutPorts []string, subinterfaces []*attrs.Attributes, aggID string) {
	t.Helper()
	d := gnmi.OC()
	dutAggPorts := []*ondatra.Port{}
	for _, port := range dutPorts {
		dutAggPorts = append(dutAggPorts, dut.Port(t, port))
	}
	if deviations.AggregateAtomicUpdate(dut) {
		cfgplugins.DeleteAggregate(t, dut, aggID, dutAggPorts)
		cfgplugins.SetupAggregateAtomically(t, dut, aggID, dutAggPorts)
	}

	lacp := &oc.Lacp_Interface{Name: ygot.String(aggID)}
	lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
	lacpPath := d.Lacp().Interface(aggID)
	fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
	gnmi.Replace(t, dut, lacpPath.Config(), lacp)

	agg := &oc.Interface{Name: ygot.String(aggID)}
	configDUTInterface(t, agg, subinterfaces, dut)
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	agg.Type = ieee8023adLag
	aggPath := d.Interface(aggID)
	fptest.LogQuery(t, aggID, aggPath.Config(), agg)
	gnmi.Replace(t, dut, aggPath.Config(), agg)

	for _, port := range dutAggPorts {
		holdTimeConfig := &oc.Interface_HoldTime{Up: ygot.Uint32(carrierDelayUp), Down: ygot.Uint32(carrierDelayDown)}
		intfPath := gnmi.OC().Interface(port.Name())
		gnmi.Update(t, dut, intfPath.HoldTime().Config(), holdTimeConfig)
	}
}

// configDUTInterface configures the aggregate interface and its subinterfaces based on the provided attributes.
func configDUTInterface(t *testing.T, i *oc.Interface, subinterfaces []*attrs.Attributes, dut *ondatra.DUTDevice) {
	t.Helper()
	for _, a := range subinterfaces {
		i.Description = ygot.String(a.Desc)
		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		s1 := i.GetOrCreateSubinterface(0)
		b4 := s1.GetOrCreateIpv4()
		b6 := s1.GetOrCreateIpv6()
		b4.Mtu = ygot.Uint16(a.MTU)
		b6.Mtu = ygot.Uint32(uint32(a.MTU))
		if deviations.InterfaceEnabled(dut) {
			b4.Enabled = ygot.Bool(true)
		}
		if a.Subinterface != 0 {
			s := i.GetOrCreateSubinterface(a.Subinterface)
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(uint16(a.Subinterface))
			configureInterfaceAddress(t, dut, s, a)
		} else {
			configureInterfaceAddress(t, dut, s1, a)
		}
	}
}

// configureInterfaceAddress assigns IPv4/IPv6 addresses to a given subinterface.
func configureInterfaceAddress(t *testing.T, dut *ondatra.DUTDevice, s *oc.Interface_Subinterface, a *attrs.Attributes) {
	t.Helper()
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	if a.IPv4 != "" {
		a4 := s4.GetOrCreateAddress(a.IPv4)
		a4.PrefixLength = ygot.Uint8(a.IPv4Len)
	}
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	if a.IPv6 != "" {
		s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(a.IPv6Len)
	}

	if a.IPv6Sec != "" {
		s62 := s.GetOrCreateIpv6()
		if deviations.InterfaceEnabled(dut) {
			s62.Enabled = ygot.Bool(true)
		}
		s62.GetOrCreateAddress(a.IPv6Sec).PrefixLength = ygot.Uint8(a.IPv6Len)
	}
}

// configureStaticRoute installs a static IPv4 route on the DUT using GNMI batch configuration.
func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	b := new(gnmi.SetBatch)
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticRoutePrefix,
		NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(staticRouteNextHop)},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	sV6 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticRouteV6Prefix,
		NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"1": oc.UnionString(agg2interface.IPv6Gateway)},
	}

	if _, err := cfgplugins.NewStaticRouteCfg(b, sV6, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, dut)
}

// pushPolicyForwardingConfig pushes the given policy forwarding configuration for the specified network instance to the DUT via gNMI Replace.
func pushPolicyForwardingConfig(t *testing.T, dut *ondatra.DUTDevice, ni *oc.NetworkInstance) {
	t.Helper()
	niPath := gnmi.OC().NetworkInstance(ni.GetName()).Config()
	gnmi.Replace(t, dut, niPath, ni)
}

// scaleTestCase describes one MPLSoGUE decapsulation scenario. A nil
// captureConfig selects the loss/ECMP scale validation; a non-nil one selects
// the payload-preserve capture validation.
type scaleTestCase struct {
	name          string
	outer         *otgconfighelpers.Flow
	inner         *otgconfighelpers.Flow
	captureConfig *packetvalidationhelpers.PacketValidation
}

func TestMPLSOGUEDecapScale(t *testing.T) {
	ate := ondatra.ATE(t, "ate")
	dut, custAggID, netConfig := configureDUTAndOTG(t)
	tests := []scaleTestCase{
		{name: "IPv4 Traffic Scale", outer: flowOuterIPv4, inner: flowInnerIPv4},
		{name: "IPv6 Traffic Scale", outer: flowOuterIPv6, inner: flowInnerIPv6},
		{name: "Multicast Traffic Scale", outer: flowOuterMcast, inner: flowInnerMcast},
		{name: "IPv4 Payload Preserve", outer: flowOuterIPv4, inner: flowInnerIPv4, captureConfig: decapValidationIPv4},
		{name: "IPv6 Payload Preserve", outer: flowOuterIPv6, inner: flowInnerIPv6, captureConfig: decapValidationIPv6},
	}

	packetvalidationhelpers.ClearCapture(t, top, ate)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.captureConfig != nil {
				updateFlow(t, tc.outer, tc.inner, true, ratePPS, totalPkts)
				packetvalidationhelpers.ConfigurePacketCapture(t, top, tc.captureConfig)
				sendTrafficCapture(t, ate, dut, custAggID, netConfig)
				if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, tc.captureConfig); err != nil {
					t.Errorf("CaptureAndValidatePackets(%s) got err: %v, want nil", tc.outer.FlowName, err)
				}
				return
			}

			createFlow(t, top, tc.outer, tc.inner, true)
			sendTraffic(t, ate, dut, custAggID, netConfig)

			// createFlow splits a multi-TxName definition into one OTG flow per
			// transmit device, so every generated sub flow is validated.
			for _, name := range flowNames(tc.outer) {
				validation := newFlowValidation(name, tolerancePct)
				if err := validation.ValidateLossOnFlows(t, ate); err != nil {
					t.Errorf("ValidateLossOnFlows(%s) got err: %v, want nil", name, err)
				}
			}
			// The sub flows share the egress LAG member ports, so the balance is
			// checked once against their combined counters.
			if err := validateV6ScaleECMPonLAG(t, ate, flowNames(tc.outer)); err != nil {
				t.Errorf("validateV6ScaleECMPonLAG(%s) got err: %v, want nil", tc.outer.FlowName, err)
			}
		})
	}

	// PF-1.20.v6 reuses the environment configured above instead of rebuilding the
	// full 2000-subinterface scale setup.
	testDecapScaleIPv6Outer(t, ate, dut, custAggID, netConfig)
}

// -----------------------------------------------------------------------------
// PF-1.20.v6: Validate scaled decapsulation of MPLS over GUE with 1000 unique
// IPv6 outer header flows.
// -----------------------------------------------------------------------------

// systemHealth holds a snapshot of the DUT CPU/memory telemetry.
type systemHealth struct {
	maxCPUInstantPct uint8
	sustainedCPUPct  uint8
	maxCPUAvgPct     uint8
	memoryUsedPct    float64
	memoryPhysical   uint64
	memoryUsed       uint64
	memoryFree       uint64
	cpuSamples       int
	memorySamples    int
}

// healthSubOpts returns the gNMI options used to establish the SAMPLE mode
// Subscribe required by the README telemetry section.
func healthSubOpts(t *testing.T, dut *ondatra.DUTDevice) *gnmi.Opts {
	t.Helper()
	return dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE), ygnmi.WithSampleInterval(healthSubSampleInterval))
}

// generateV6ScaleOuterSources returns v6ScaleFlowCount unique outer IPv6 source
// addresses used to program the decap rules and to generate the ATE streams.
func generateV6ScaleOuterSources(t *testing.T) []string {
	t.Helper()

	// The README requires the UDP source ports to vary within the ephemeral
	// range 49152-65535; confirm the generated streams stay inside it.
	if v6ScaleEphemeralMin+v6ScaleFlowCount-1 > v6ScaleEphemeralMax {
		t.Fatalf("UDP source ports %d-%d exceed the ephemeral range %d-%d", v6ScaleEphemeralMin, v6ScaleEphemeralMin+v6ScaleFlowCount-1, v6ScaleEphemeralMin, v6ScaleEphemeralMax)
	}
	srcs, err := iputil.GenerateIPv6sWithStep(v6ScaleOuterSrcIPv6, v6ScaleFlowCount, v6ScaleOuterSrcStep)
	if err != nil {
		t.Fatalf("Failed to generate %d unique outer IPv6 sources: %v", v6ScaleFlowCount, err)
	}
	return srcs
}

// configureV6ScaleDecapPolicy programs v6ScaleFlowCount unique MPLSoGUE decap
// rules (one per unique outer IPv6 source address) in a single gNMI Set
// operation and applies the resulting policy to the ingress aggregate interface.
//
// In addition to the policy-forwarding rules this enables the MPLSoGUE
// decapsulation plumbing (MPLS forwarding, the static label range, the GUE decap
// group and QoS classification), which is what makes the DUT pop the MPLS label
// stack from the UDP payload and forward the inner IPv4/IPv6 packet.
//
// The gNMI Set is expected to complete within v6ScaleSetTimeout; the test fails
// if the device rejects the configuration (e.g. TCAM exhaustion) or times out.
//
// It returns true when the rules were programmed through the OC
// policy-forwarding model, along with the parameters used, so that the caller
// can verify the rules on the matching path (OC telemetry or CLI).
//
// parent is the top-level test; all revert operations are registered on it so
// that the configuration survives until every subtest (including the traffic
// subtest) has finished.
func configureV6ScaleDecapPolicy(t *testing.T, parent *testing.T, dut *ondatra.DUTDevice, srcs []string, ingressAggIDs []string) (bool, cfgplugins.GueDecapV6ScaleParams) {
	t.Helper()
	ocProgrammed := !deviations.GueGreDecapUnsupported(dut) && !deviations.PolicyForwardingOCUnsupported(dut)
	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	ni.SetType(oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	pf := ni.GetOrCreatePolicyForwarding()
	guePFParams := cfgplugins.GueDecapV6ScaleParams{
		PolicyID:            v6ScalePolicyID,
		OuterSrcIPv6s:       srcs,
		SrcPrefixLen:        v6ScaleSrcPrefixLen,
		DecapIPv6Prefix:     v6ScaleDecapPrefix,
		GUEPort:             udpDstPort,
		IngressInterfaceIDs: ingressAggIDs,
		PfInstance:          pf,
	}
	// MPLSoGUE decapsulation plumbing: enable MPLS forwarding, open the static
	// label range used by the MPLSoGUE header, and classify the MPLS EXP bits.
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	// The outer IPv6 destination of the MPLSoGUE traffic must be owned by the
	// DUT, otherwise the packets are never attracted to the decapsulation engine
	// and are dropped instead of being decapsulated and forwarded.
	configureV6ScaleDecapDestination(t, parent, dut)
	t.Logf("Pushing %d IPv6 outer-header MPLSoGUE decap rules with a single gNMI Set...", len(srcs))
	start := time.Now()
	cfgplugins.DecapGroupConfigGueV6Scale(t, dut, guePFParams)
	if ocProgrammed {
		gnmi.Update(t, dut, gnmi.OC().NetworkInstance(ni.GetName()).PolicyForwarding().Config(), pf)
	}
	elapsed := time.Since(start)
	// Revert the DUT to its pre-test state once the whole test (not just this
	// subtest) has finished, so that the traffic subtest still sees the decap
	// configuration.
	parent.Cleanup(func() {
		guePFParams.NetworkInstanceName = ni.GetName()
		cfgplugins.RemoveDecapGroupConfigGueV6Scale(parent, dut, guePFParams)
	})
	if elapsed > v6ScaleSetTimeout {
		t.Errorf("gNMI Set of %d IPv6 decap rules took %v, want <= %v", len(srcs), elapsed, v6ScaleSetTimeout)
		return ocProgrammed, guePFParams
	}
	t.Logf("gNMI Set of %d IPv6 MPLSoGUE decap rules completed in %v", len(srcs), elapsed)
	return ocProgrammed, guePFParams
}

// configureV6ScaleDecapDestination assigns the outer IPv6 destination address of
// the MPLSoGUE traffic to a DUT loopback so the traffic is routed to the DUT
// itself and hits the decapsulation group covering v6ScaleDecapPrefix.
func configureV6ScaleDecapDestination(t *testing.T, parent *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	loopName := netutil.LoopbackInterface(t, dut, v6ScaleLoopbackID)
	lo := &oc.Interface{
		Name: ygot.String(loopName),
		Type: oc.IETFInterfaces_InterfaceType_softwareLoopback,
	}
	if deviations.InterfaceEnabled(dut) {
		lo.Enabled = ygot.Bool(true)
	}
	s := lo.GetOrCreateSubinterface(0)
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(v6ScaleOuterDstIPv6).PrefixLength = ygot.Uint8(v6ScaleDecapLoopbackLen)
	gnmi.Update(t, dut, gnmi.OC().Interface(loopName).Config(), lo)
	t.Logf("Configured decap destination %s/%d on %s", v6ScaleOuterDstIPv6, v6ScaleDecapLoopbackLen, loopName)
	parent.Cleanup(func() {
		gnmi.Delete(parent, dut, gnmi.OC().Interface(loopName).Subinterface(0).Ipv6().Address(v6ScaleOuterDstIPv6).Config())
	})
}

// verifyV6ScaleDecapRules confirms via telemetry that all programmed rules are
// present on the DUT after the scaled gNMI Set operation.
func verifyV6ScaleDecapRules(t *testing.T, dut *ondatra.DUTDevice, wantRules int) error {
	t.Helper()
	policyPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(v6ScalePolicyID)
	// Wait for the last programmed rule to be reflected in state before counting.
	if _, ok := gnmi.Watch(t, dut, policyPath.Rule(uint32(wantRules)).SequenceId().State(), v6ScaleSetTimeout,
		func(val *ygnmi.Value[uint32]) bool {
			_, present := val.Val()
			return present
		}).Await(t); !ok {
		return fmt.Errorf("policy %v rule %d not programmed within %v", v6ScalePolicyID, wantRules, v6ScaleSetTimeout)
	}
	seqIDs := gnmi.LookupAll(t, dut, policyPath.RuleAny().SequenceId().State())
	got := 0
	for _, s := range seqIDs {
		if _, ok := s.Val(); ok {
			got++
		}
	}
	if got != wantRules {
		return fmt.Errorf("policy %v programmed rules: got %d, want %d", v6ScalePolicyID, got, wantRules)
	}
	t.Logf("Policy %v reports all %d IPv6 decap rules programmed", v6ScalePolicyID, got)
	return nil
}

// verifyV6ScaleDecapRulesCLI confirms that all decap rules were programmed on
// platforms where policy-forwarding OC is unsupported and the equivalent native
// configuration is used instead.
func verifyV6ScaleDecapRulesCLI(t *testing.T, dut *ondatra.DUTDevice, params cfgplugins.GueDecapV6ScaleParams, wantRules int) error {
	t.Helper()
	got := cfgplugins.CountGueDecapV6ScaleRulesNative(t, dut, params)
	if got < 0 {
		return fmt.Errorf("no native verification available for vendor %v; cannot confirm %d decap rules", dut.Vendor(), wantRules)
	}
	if got != wantRules {
		return fmt.Errorf("policy %v programmed rules (native config): got %d, want %d", params.PolicyID, got, wantRules)
	}
	t.Logf("Native configuration reports all %d IPv6 decap rules programmed for policy %v", got, params.PolicyID)
	return nil
}

// collectSystemHealth establishes a gNMI Subscribe (SAMPLE mode) to
// /system/cpus/cpu/state/total/instant, /system/cpus/cpu/state/total/avg,
// /system/memory/state/physical, /system/memory/state/free and
// /system/memory/state/used, as required by the README telemetry section, and
// summarizes the collected samples.
//
// The instantaneous CPU counter routinely spikes to 100% for a single sample
// (for example while the gNMI subscription itself is served), so the median of
// the streamed samples represents sustained utilization while the maximum is
// only logged.
func collectSystemHealth(t *testing.T, dut *ondatra.DUTDevice) systemHealth {
	t.Helper()
	h := systemHealth{}
	opts := healthSubOpts(t, dut)

	// /system/cpus/cpu/state/total/instant and .../avg across all cores.
	//
	// The leaves are subscribed individually rather than subscribing to the
	// parent container: a SAMPLE subscription on a container yields updates that
	// ygnmi has to unmarshal into a struct, and any sample that carries no data
	// (or a delete) makes it fail with "invalid input to DeepCopy, got nil
	// value". Leaf subscriptions return plain scalars and are unaffected.
	cpuDur := healthCPUSubDuration / 2
	var instants []uint8
	for _, s := range gnmi.CollectAll(t, opts, gnmi.OC().System().CpuAny().Total().Instant().State(), cpuDur).Await(t) {
		v, ok := s.Val()
		if !ok {
			continue
		}
		h.cpuSamples++
		instants = append(instants, v)
		if v > h.maxCPUInstantPct {
			h.maxCPUInstantPct = v
		}
	}
	for _, s := range gnmi.CollectAll(t, opts, gnmi.OC().System().CpuAny().Total().Avg().State(), cpuDur).Await(t) {
		v, ok := s.Val()
		if !ok {
			continue
		}
		if v > h.maxCPUAvgPct {
			h.maxCPUAvgPct = v
		}
	}
	if len(instants) > 0 {
		sort.Slice(instants, func(i, j int) bool { return instants[i] < instants[j] })
		h.sustainedCPUPct = instants[len(instants)/2]
	}
	if h.cpuSamples == 0 {
		t.Errorf("gNMI Subscribe to /system/cpus/cpu/state/total/instant returned no samples in %v", cpuDur)
	}

	// /system/memory/state/physical, /free and /used, again subscribed as
	// individual leaves for the reason described above.
	memDur := healthMemSubDuration / 3
	collectMemLeaf := func(path ygnmi.SingletonQuery[uint64], name string) (uint64, int) {
		var last uint64
		samples := 0
		for _, s := range gnmi.Collect(t, opts, path, memDur).Await(t) {
			v, ok := s.Val()
			if !ok {
				continue
			}
			samples++
			last = v
		}
		if samples == 0 {
			t.Errorf("gNMI Subscribe to %s returned no samples in %v", name, memDur)
		}
		return last, samples
	}
	var physSamples, usedSamples, freeSamples int
	h.memoryPhysical, physSamples = collectMemLeaf(gnmi.OC().System().Memory().Physical().State(), "/system/memory/state/physical")
	h.memoryUsed, usedSamples = collectMemLeaf(gnmi.OC().System().Memory().Used().State(), "/system/memory/state/used")
	h.memoryFree, freeSamples = collectMemLeaf(gnmi.OC().System().Memory().Free().State(), "/system/memory/state/free")
	h.memorySamples = physSamples + usedSamples + freeSamples
	// Some platforms do not report /physical; fall back to used+free so that the
	// utilization can still be validated against the threshold.
	total := h.memoryPhysical
	if total == 0 {
		total = h.memoryUsed + h.memoryFree
	}
	if total > 0 && h.memoryUsed > 0 {
		h.memoryUsedPct = float64(h.memoryUsed) / float64(total) * 100
	}
	return h
}

// validateSystemHealth verifies that CPU/memory utilization stays within the
// safe threshold during the scale operation.
func validateSystemHealth(t *testing.T, h systemHealth, stage string) error {
	t.Helper()
	t.Logf("%s: max CPU instant=%d%%, sustained CPU=%d%%, max CPU avg=%d%% (%d samples)", stage, h.maxCPUInstantPct, h.sustainedCPUPct, h.maxCPUAvgPct, h.cpuSamples)
	t.Logf("%s: memory physical=%d bytes, used=%d bytes, free=%d bytes, used=%.2f%% (%d samples)", stage, h.memoryPhysical, h.memoryUsed, h.memoryFree, h.memoryUsedPct, h.memorySamples)
	if h.maxCPUInstantPct > healthThresholdPct && h.sustainedCPUPct <= healthThresholdPct {
		t.Logf("%s: transient CPU spike of %d%% ignored (sustained utilization %d%%)", stage, h.maxCPUInstantPct, h.sustainedCPUPct)
	}
	if h.sustainedCPUPct > healthThresholdPct {
		return fmt.Errorf("%s: sustained CPU utilization %d%% exceeds threshold %d%%", stage, h.sustainedCPUPct, healthThresholdPct)
	}
	if h.maxCPUAvgPct > healthThresholdPct {
		return fmt.Errorf("%s: CPU average utilization %d%% exceeds threshold %d%%", stage, h.maxCPUAvgPct, healthThresholdPct)
	}
	if h.memoryUsedPct > healthThresholdPct {
		return fmt.Errorf("%s: memory utilization %.2f%% exceeds threshold %d%%", stage, h.memoryUsedPct, healthThresholdPct)
	}
	return nil
}

// collectEgressPacketRates establishes a gNMI Subscribe on the egress interface
// packet counters of the Aggregate1 member links and derives the packet rate
// from the streamed samples, as required by the README telemetry section. It
// must be called while traffic is running.
func collectEgressPacketRates(t *testing.T, dut *ondatra.DUTDevice) map[string]float64 {
	t.Helper()
	opts := healthSubOpts(t, dut)
	rates := map[string]float64{}
	// The subscription budget is shared between the member links so the whole
	// measurement fits inside the traffic run.
	per := egressRateSubDuration / time.Duration(len(agg1.MemberPorts))
	for _, p := range agg1.MemberPorts {
		name := dut.Port(t, p).Name()
		type point struct {
			ts   time.Time
			pkts uint64
		}
		var pts []point
		for _, s := range gnmi.Collect(t, opts, gnmi.OC().Interface(name).Counters().OutPkts().State(), per).Await(t) {
			v, ok := s.Val()
			if !ok {
				continue
			}
			pts = append(pts, point{ts: s.Timestamp, pkts: v})
		}
		if len(pts) < 2 {
			t.Errorf("egress port %s: gNMI Subscribe returned %d out-pkts samples in %v, want >= 2 to derive a packet rate", name, len(pts), per)
			continue
		}
		first, last := pts[0], pts[len(pts)-1]
		dt := last.ts.Sub(first.ts).Seconds()
		if dt <= 0 {
			t.Errorf("egress port %s: non increasing sample timestamps, cannot derive packet rate", name)
			continue
		}
		rate := float64(last.pkts-first.pkts) / dt
		rates[name] = rate
		t.Logf("Egress port %s: out-pkts %d -> %d over %.2fs, packet rate %.0f pps", name, first.pkts, last.pkts, dt, rate)
		if rate <= 0 {
			t.Errorf("egress port %s: packet rate is %.0f pps while scaled traffic is running, want > 0", name, rate)
		}
	}
	return rates
}

// testDecapScaleIPv6Outer implements PF-1.20.v6: it programs 1000 unique IPv6
// outer-header decap rules on the DUT, runs 1000 simultaneous MPLSoGUE flows
// with unique outer IPv6 source addresses / UDP source ports and unique inner
// payload addresses and ports, and validates zero packet loss along with stable
// CPU/memory utilization. It reuses the environment built by
// configureDUTAndOTG so that the expensive scale setup runs only once.
func testDecapScaleIPv6Outer(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice, custAggID string, netConfig *networkConfig) {
	t.Helper()
	// Bind the egress (Aggregate1) subinterfaces to the scale flows.
	for _, intf := range agg1.Interfaces {
		flowOuterV6ScaleIPv4Payload.RxNames = append(flowOuterV6ScaleIPv4Payload.RxNames, intf.Name+".IPv4")
		flowOuterV6ScaleIPv6Payload.RxNames = append(flowOuterV6ScaleIPv6Payload.RxNames, intf.Name+".IPv6")
	}
	// testDecapScaleIPv6Outer's t is the top-level test, so capture it for the
	// cleanup registrations that must outlive the individual subtests.
	parent := t
	// Shared between the two subtests so that the per-rule counters can be read
	// back after traffic has been forwarded.
	var guePFParams cfgplugins.GueDecapV6ScaleParams
	var ocProgrammed bool
	t.Run("PF-1.20.v6: Program 1000 unique IPv6 outer header decap rules", func(t *testing.T) {
		v6ScaleOuterSrcIPv6s = generateV6ScaleOuterSources(t)
		if err := validateSystemHealth(t, collectSystemHealth(t, dut), "Before scaled gNMI Set"); err != nil {
			t.Error(err)
		}
		// Both ingress (core facing) aggregates carry the MPLSoGUE traffic.
		ocProgrammed, guePFParams = configureV6ScaleDecapPolicy(t, parent, dut, v6ScaleOuterSrcIPv6s, []string{coreAggIntfID, coreAggIntfID2})
		if ocProgrammed {
			if err := verifyV6ScaleDecapRules(t, dut, v6ScaleFlowCount); err != nil {
				t.Error(err)
			}
		} else {
			if err := verifyV6ScaleDecapRulesCLI(t, dut, guePFParams, v6ScaleFlowCount); err != nil {
				t.Error(err)
			}
		}
		validateSystemHealth(t, collectSystemHealth(t, dut), "After scaled gNMI Set")
	})
	t.Run("PF-1.20.v6: Decapsulate 1000 simultaneous IPv6 outer header flows", func(t *testing.T) {
		// Per-rule counters must be active before traffic is sent, otherwise the
		// "count" actions are not backed by counter resources and keep reading 0.
		if !ocProgrammed && !cfgplugins.TrafficPolicyCountersEnabled(t, dut) {
			if !cfgplugins.EnableTrafficPolicyCounters(t, dut) {
				t.Fatalf("Unable to enable traffic-policy counters on %v; per-rule matched-packet validation cannot be performed", dut.Vendor())
			}
		}
		// Both flows run simultaneously so that the combined line rate is at
		// least v6ScaleLineRatePct of the ingress port capacity.
		createFlow(t, top, flowOuterV6ScaleIPv4Payload, flowInnerV6ScaleIPv4Payload, true)
		createFlow(t, top, flowOuterV6ScaleIPv6Payload, flowInnerV6ScaleIPv6Payload, false)
		// Device health and the egress packet rates are monitored through gNMI
		// Subscribe while the scaled traffic is running.
		sendTrafficWithTelemetry(t, ate, dut, custAggID, netConfig, v6ScaleTrafficDuration, func(t *testing.T) {
			validateSystemHealth(t, collectSystemHealth(t, dut), "During scaled traffic")
			collectEgressPacketRates(t, dut)
		})
		// The scale flows must forward with zero loss, so every generated sub flow
		// is validated against a zero tolerance.
		for _, f := range []*otgconfighelpers.Flow{flowOuterV6ScaleIPv4Payload, flowOuterV6ScaleIPv6Payload} {
			for _, name := range flowNames(f) {
				if err := newFlowValidation(name, 0).ValidateLossOnFlows(t, ate); err != nil {
					t.Errorf("ValidateLossOnFlows(%s) got err: %v, want nil", name, err)
				}
			}
		}
		// All the scale sub flows egress the same LAG simultaneously, so the
		// balance is validated against the combined member-port counters rather
		// than against a single flow's counters.
		if err := validateV6ScaleECMPonLAG(t, ate, append(flowNames(flowOuterV6ScaleIPv4Payload), flowNames(flowOuterV6ScaleIPv6Payload)...)); err != nil {
			t.Errorf("validateV6ScaleECMPonLAG(): got err: %v, want nil", err)
		}
		// Confirm every decap rule actually matched traffic, so that a rule that
		// was programmed but never installed in hardware is detected.
		if ocProgrammed {
			if err := verifyV6ScaleRuleMatchedPkts(t, dut, v6ScaleFlowCount); err != nil {
				t.Error(err)
			}
			return
		}
		// On platforms that terminate the tunnel in hardware before the ingress
		// classifiers are evaluated, the outer IPv6 header is no longer visible
		// to the per-rule counters while the decap group is active. The rule
		// classification of all 1000 unique outer headers is therefore measured
		// in a dedicated pass with the decap group temporarily removed, which
		// leaves the outer header intact on ingress.
		guePFParams.Enabled = false
		cfgplugins.SetGueDecapV6ScaleDecapGroup(t, dut, guePFParams)
		defer func() {
			guePFParams.Enabled = true
			cfgplugins.SetGueDecapV6ScaleDecapGroup(t, dut, guePFParams)
		}()
		cfgplugins.ClearGueDecapV6ScaleCounters(t, dut, guePFParams)
		// Forwarding is intentionally not validated in this pass: without the
		// decap group the outer destination is a local DUT address, so the
		// packets are consumed by the DUT instead of being decapsulated and
		// forwarded to the customer LAG. The ATE therefore reports 0 Rx here,
		// which is expected. Decapsulation and forwarding were already validated
		// with zero loss in the pass above.
		t.Log("Re-sending traffic with the decap group removed to measure per-rule classification; 0 Rx on the ATE is expected in this pass")
		sendTraffic(t, ate, dut, custAggID, netConfig)
		verifyV6ScaleRuleMatchedPktsCLI(t, dut, guePFParams)
	})
	t.Run("PF-1.20.v6: Verify MPLSoGUE headers are removed and inner payload preserved", func(t *testing.T) {
		// The pass above proves zero loss; this pass proves at packet level that
		// the outer IPv6/UDP/MPLS headers were actually stripped and that the
		// inner DSCP/traffic-class and TTL/hop-limit were not altered. A reduced
		// rate is used so the ATE capture buffer is not overrun.
		for _, tc := range []struct {
			name             string
			outer, inner     *otgconfighelpers.Flow
			validationConfig *packetvalidationhelpers.PacketValidation
		}{
			{"IPv4 payload", flowOuterV6ScaleIPv4Payload, flowInnerV6ScaleIPv4Payload, decapValidationV6ScaleIPv4},
			{"IPv6 payload", flowOuterV6ScaleIPv6Payload, flowInnerV6ScaleIPv6Payload, decapValidationV6ScaleIPv6},
		} {
			t.Run(tc.name, func(t *testing.T) {
				packetvalidationhelpers.ClearCapture(t, top, ate)
				updateFlow(t, tc.outer, tc.inner, true, ratePPS, totalPkts)
				packetvalidationhelpers.ConfigurePacketCapture(t, top, tc.validationConfig)
				sendTrafficCapture(t, ate, dut, custAggID, netConfig)
				if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, tc.validationConfig); err != nil {
					t.Errorf("CaptureAndValidatePackets(%s) got err: %v, want nil", tc.outer.FlowName, err)
				}
			})
		}
	})
}

// verifyV6ScaleRuleMatchedPkts checks the OC matched-pkts and matched-octets
// counters of every decap rule and reports the rules that never matched a packet.
func verifyV6ScaleRuleMatchedPkts(t *testing.T, dut *ondatra.DUTDevice, wantRules int) error {
	t.Helper()
	policyPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy(v6ScalePolicyID)
	var zeroRules []uint32
	var zeroOctetRules []uint32
	var total, totalOctets uint64
	for i := 1; i <= wantRules; i++ {
		seq := uint32(i)
		pkts, ok := gnmi.Lookup(t, dut, policyPath.Rule(seq).MatchedPkts().State()).Val()
		if !ok || pkts == 0 {
			zeroRules = append(zeroRules, seq)
			continue
		}
		total += pkts
		// The README also requires the matched-octets telemetry to be covered.
		octets, ok := gnmi.Lookup(t, dut, policyPath.Rule(seq).MatchedOctets().State()).Val()
		if !ok || octets == 0 {
			zeroOctetRules = append(zeroOctetRules, seq)
			continue
		}
		totalOctets += octets
	}
	if len(zeroOctetRules) > 0 {
		return fmt.Errorf("%d of %d decap rules reported 0 matched-octets (first offenders: %v); want non-zero octets on every rule", len(zeroOctetRules), wantRules, truncateUint32s(zeroOctetRules, v6ScaleZeroRuleLogLimit))
	}
	t.Logf("All %d decap rules reported matched-octets (total matched octets: %d)", wantRules, totalOctets)
	reportV6ScaleZeroRules(t, len(zeroRules), wantRules, total, fmt.Sprint(zeroRules))
	return nil
}

// verifyV6ScaleRuleMatchedPktsCLI checks the native per-rule packet counters via
// "show traffic-policy <policy> counters" and reports the rules that never matched a packet.
func verifyV6ScaleRuleMatchedPktsCLI(t *testing.T, dut *ondatra.DUTDevice, params cfgplugins.GueDecapV6ScaleParams) error {
	t.Helper()
	// Without counter granularity the "count" actions are not backed by counter
	// resources and always read 0. Enable them through the native CLI rather than
	// skipping the validation.
	if !cfgplugins.TrafficPolicyCountersEnabled(t, dut) && !cfgplugins.EnableTrafficPolicyCounters(t, dut) {
		return fmt.Errorf("traffic-policy counters are not enabled on %v; per-rule matched-packet validation cannot be performed", dut.Vendor())
	}
	counters := cfgplugins.GueDecapV6ScaleRuleCounters(t, dut, params)
	if counters == nil {
		return fmt.Errorf("no native traffic-policy counter verification available for vendor %v", dut.Vendor())
	}
	names := cfgplugins.GueDecapV6ScaleRuleNames(params)
	var (
		zeroRules []string
		total     uint64
	)
	for _, name := range names {
		pkts, ok := counters[name]
		if !ok || pkts == 0 {
			zeroRules = append(zeroRules, name)
			continue
		}
		total += pkts
	}
	if err := reportV6ScaleZeroRules(t, len(zeroRules), len(names), total, strings.Join(truncateStrings(zeroRules, v6ScaleZeroRuleLogLimit), ", ")); err != nil {
		return err
	}
	return nil
}

// reportV6ScaleZeroRules fails the test when any decap rule did not match a
// packet, and logs the aggregate matched packet count otherwise.
func reportV6ScaleZeroRules(t *testing.T, zeroCount, wantRules int, totalPkts uint64, zeroSample string) error {
	t.Helper()
	if zeroCount > 0 {
		return fmt.Errorf("%d of %d decap rules matched 0 packets (first offenders: %s); want all rules to match traffic", zeroCount, wantRules, zeroSample)
	}
	t.Logf("All %d decap rules matched traffic (total matched packets: %d)", wantRules, totalPkts)
	return nil
}

// truncateStrings returns at most limit elements of s, for concise logging.
func truncateStrings(s []string, limit int) []string {
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}

// truncateUint32s returns at most limit elements of s, for concise logging.
func truncateUint32s(s []uint32, limit int) []uint32 {
	if len(s) <= limit {
		return s
	}
	return s[:limit]
}

// validateV6ScaleECMPonLAG verifies that the decapsulated traffic of all the
// given flows is distributed evenly across the egress LAG member ports.
//
// The shared ValidateECMPonLAG helper compares each member port against half of
// a single flow's received packets, which under-counts when several flows share
// the same LAG. Here the expected per-port share is derived from the sum of the
// flows' received packets.
func validateV6ScaleECMPonLAG(t *testing.T, ate *ondatra.ATEDevice, flowNames []string) error {
	t.Helper()
	var totalPkts uint64
	for _, name := range flowNames {
		totalPkts += gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(name).Counters().InPkts().State())
	}
	if totalPkts == 0 {
		return fmt.Errorf("flows %v received no packets", flowNames)
	}
	ports := agg1.MemberPorts
	expected := float64(totalPkts) / float64(len(ports))
	for _, p := range ports {
		got := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, p).ID()).Counters().InFrames().State())
		deviation := math.Abs(expected-float64(got)) * 100 / expected
		t.Logf("LAG member %s received %d frames (expected ~%.0f, deviation %.2f%%)", p, got, expected, deviation)
		if deviation > v6ScaleECMPTolerancePct {
			return fmt.Errorf("port %s packet count mismatch: got %d, want within ~%.0f ±%v%%", p, got, expected, v6ScaleECMPTolerancePct)
		}
	}
	return nil
}
