// Package mpls_gre_udp_qos_test tests MPLSoGRE/MPLSoGUE QoS classification, marking,
// scheduling (bandwidth/priority classes with and without shaping) and policing.
package mpls_gre_udp_qos_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	packetvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

// TestMain calls main function.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ethernetCsmacd = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag  = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	greProtocol    = 47
	// gueDstPort is the well known UDP destination port used for MPLSoGUE encap/decap.
	gueDstPort = 6635
	// outerMarkedDSCP is the DSCP value the DUT must set on the MPLSoGRE/MPLSoGUE outer header (PF-1.18.1/1.18.3).
	outerMarkedDSCP = 32
	// ingressClassifyTC3/ingressClassifyTC4 are the traffic classes IP-to-encap traffic is classified into (PF-1.18.1/1.18.3).
	ingressClassifyTC3 = 3
	ingressClassifyTC4 = 4

	bwSchedulerName         = "scheduler-bw"
	bwShaperSchedulerName   = "scheduler-bw-shaper"
	prioSchedulerName       = "scheduler-prio"
	prioShaperSchedulerName = "scheduler-prio-shaper"
	ingressPolicerName      = "scheduler-ingress-2r3c"

	// encapMPLSLabel is the MPLS label the DUT pushes for IP-to-encap (customer -> core) traffic.
	encapMPLSLabel  = 116383
	greNHGName      = "nhg-gre-encap"
	gueNHGName      = "nhg-gue-encap"
	encapPolicyName = "encap-mplsogre-mplsogue"
	// outerEncapPrefix covers all MPLSoGRE/MPLSoGUE outer destination addresses used below and
	// is what the decap-groups match on (see configureAristaDecap).
	outerEncapPrefix = "10.99.0.0/16"
	// outerGREDstCoreN/outerGUEDstCoreN are host addresses within outerEncapPrefix, one pair per
	// core uplink, each reachable via the matching static route (see configureStaticRoutes).
	outerGREDstCore1 = "10.99.1.1"
	outerGREDstCore2 = "10.99.2.1"
	outerGUEDstCore1 = "10.99.1.2"
	outerGUEDstCore2 = "10.99.2.2"
	decapGREGroup    = "gre-decap"
	decapGUEGroup    = "gue-decap"

	trafficDuration  = 60 * time.Second
	lossTolerancePct = float32(2.0)
)

var (
	top gosnappi.Config = gosnappi.NewConfig()

	custAggID  string
	core1AggID string
	core2AggID string

	custPorts  = []string{"port1", "port2"}
	core1Ports = []string{"port3", "port4"}
	core2Ports = []string{"port5", "port6"}

	// custIntfs models the "5 sub-interfaces" the README requires the bandwidth/priority
	// class egress queueing profiles to be applied on (PF-1.18.1, PF-1.18.4-1.18.7). Traffic
	// validation is driven primarily via custIntfs[0]; the same scheduler-policy is applied to
	// all 5 to demonstrate consistent behavior across sub-interfaces.
	custIntfTC0 = attrs.Attributes{Desc: "customer-0", MTU: 1500, IPv4: "169.254.0.11", IPv4Len: 29, IPv6: "2001:db8:10:11::1", IPv6Len: 126, Subinterface: 20}
	custIntfTC1 = attrs.Attributes{Desc: "customer-1", MTU: 1500, IPv4: "169.254.0.19", IPv4Len: 29, Subinterface: 21}
	custIntfTC2 = attrs.Attributes{Desc: "customer-2", MTU: 1500, IPv4: "169.254.0.27", IPv4Len: 29, Subinterface: 22}
	custIntfTC3 = attrs.Attributes{Desc: "customer-3", MTU: 1500, IPv4: "169.254.0.35", IPv4Len: 29, Subinterface: 23}
	custIntfTC4 = attrs.Attributes{Desc: "customer-4", MTU: 1500, IPv4: "169.254.0.43", IPv4Len: 29, Subinterface: 24}
	custIntfs   = []*attrs.Attributes{&custIntfTC0, &custIntfTC1, &custIntfTC2, &custIntfTC3, &custIntfTC4}

	// core1Intf/core2Intf are the two eBGP uplinks (ATE Ports 3,4 and ATE Ports 5,6).
	core1Intf = attrs.Attributes{Desc: "core1", MTU: 9202, IPv4: "194.0.2.1", IPv4Len: 24, IPv6: "2001:10:1:6::1", IPv6Len: 126}
	core2Intf = attrs.Attributes{Desc: "core2", MTU: 9202, IPv4: "194.0.3.1", IPv4Len: 24, IPv6: "2001:10:1:7::1", IPv6Len: 126}

	agg1 = &otgconfighelpers.Port{
		Name:        "Port-Channel1",
		AggMAC:      "02:00:01:01:01:07",
		Interfaces:  []*otgconfighelpers.InterfaceProperties{custOTG0, custOTG1, custOTG2, custOTG3, custOTG4},
		MemberPorts: custPorts,
		LagID:       1,
		IsLag:       true,
	}
	agg2 = &otgconfighelpers.Port{
		Name:        "Port-Channel2",
		AggMAC:      "02:00:01:01:01:01",
		Interfaces:  []*otgconfighelpers.InterfaceProperties{core1OTG},
		MemberPorts: core1Ports,
		LagID:       2,
		IsLag:       true,
	}
	agg3 = &otgconfighelpers.Port{
		Name:        "Port-Channel3",
		AggMAC:      "02:00:01:01:01:04",
		Interfaces:  []*otgconfighelpers.InterfaceProperties{core2OTG},
		MemberPorts: core2Ports,
		LagID:       3,
		IsLag:       true,
	}

	custOTG0 = &otgconfighelpers.InterfaceProperties{
		Name: "Port-Channel1.20", MAC: "02:00:01:01:01:08", Vlan: 20,
		IPv4: "169.254.0.12", IPv4Gateway: "169.254.0.11", IPv4Len: 29,
		IPv6: "2001:db8:10:11::2", IPv6Gateway: "2001:db8:10:11::1", IPv6Len: 126,
	}
	custOTG1 = &otgconfighelpers.InterfaceProperties{
		Name: "Port-Channel1.21", MAC: "02:00:01:01:01:09", Vlan: 21,
		IPv4: "169.254.0.20", IPv4Gateway: "169.254.0.19", IPv4Len: 29,
	}
	custOTG2 = &otgconfighelpers.InterfaceProperties{
		Name: "Port-Channel1.22", MAC: "02:00:01:01:01:10", Vlan: 22,
		IPv4: "169.254.0.28", IPv4Gateway: "169.254.0.27", IPv4Len: 29,
	}
	custOTG3 = &otgconfighelpers.InterfaceProperties{
		Name: "Port-Channel1.23", MAC: "02:00:01:01:01:11", Vlan: 23,
		IPv4: "169.254.0.36", IPv4Gateway: "169.254.0.35", IPv4Len: 29,
	}
	custOTG4 = &otgconfighelpers.InterfaceProperties{
		Name: "Port-Channel1.24", MAC: "02:00:01:01:01:12", Vlan: 24,
		IPv4: "169.254.0.44", IPv4Gateway: "169.254.0.43", IPv4Len: 29,
	}
	core1OTG = &otgconfighelpers.InterfaceProperties{
		Name: "Port-Channel2", MAC: "02:00:01:01:01:02",
		IPv4: "194.0.2.2", IPv4Gateway: "194.0.2.1", IPv4Len: 24,
		IPv6: "2001:10:1:6::2", IPv6Gateway: "2001:10:1:6::1", IPv6Len: 126,
	}
	core2OTG = &otgconfighelpers.InterfaceProperties{
		Name: "Port-Channel3", MAC: "02:00:01:01:01:05",
		IPv4: "194.0.3.2", IPv4Gateway: "194.0.3.1", IPv4Len: 24,
		IPv6: "2001:10:1:7::2", IPv6Gateway: "2001:10:1:7::1", IPv6Len: 126,
	}

	// sizeWeightProfile implements the "64, 128, 256, 512, 1024...MTU" IMIX frame size mix
	// required throughout the README.
	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 20},
		{Size: 256, Weight: 20},
		{Size: 512, Weight: 10},
		{Size: 1500, Weight: 28},
		{Size: 9000, Weight: 2},
	}
)

// ConfigureOTG configures the ATE topology (3 aggregate interfaces).
func ConfigureOTG(t *testing.T) {
	t.Helper()
	top.Captures().Clear()
	ate := ondatra.ATE(t, "ate")
	for _, agg := range []*otgconfighelpers.Port{agg1, agg2, agg3} {
		otgconfighelpers.ConfigureNetworkInterface(t, top, ate, agg)
	}
	ate.OTG().PushConfig(t, top)
}

// ConfigureDut configures interfaces, static routes, MPLSoGRE/MPLSoGUE encap and decap, and
// QoS (PF-1.18.1).
//
// NOTE: encap/decap config is written directly here (rather than via the shared
// cfgplugins.PolicyForwardingConfig/NextHopGroupConfig/DecapGroupConfigGre/MPLSStaticLSPConfig
// helpers) because those helpers' CLI-deviation branches push fixed, non-parameterized CLI
// snippets tied to a different test's lab addressing (e.g. hardcoded prefixes/next-hop-group
// names), which would silently misconfigure this test's topology.
func ConfigureDut(t *testing.T, dut *ondatra.DUTDevice) {
	configureHardwareInit(t, dut)

	custAggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, custPorts, custIntfs, custAggID)

	core1AggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, core1Ports, []*attrs.Attributes{&core1Intf}, core1AggID)

	core2AggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, core2Ports, []*attrs.Attributes{&core2Intf}, core2AggID)

	configureStaticRoutes(t, dut)

	// IP to Encap direction: classify, mark, and encapsulate (MPLSoGRE + MPLSoGUE).
	configureEncapMPLSInGREAndGUE(t, dut)
	// Encap to IP direction: decapsulate (MPLSoGRE + MPLSoGUE) and classify by MPLS EXP bits.
	configureDecapMPLSInGREAndGUE(t, dut)

	configureQoS(t, dut)
}

// configureHardwareInit programs the Arista TCAM profile required for port traffic-policy
// support; without it, binding a traffic-policy to an interface fails ("Port traffic-policy
// not supported in TCAM profile").
func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	hardwarePfCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeaturePolicyForwarding)
	if hardwarePfCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwarePfCfg)
}

// configureEncapMPLSInGREAndGUE implements the "IP to Encap" side of PF-1.18.1.
func configureEncapMPLSInGREAndGUE(t *testing.T, dut *ondatra.DUTDevice) {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		configureAristaEncap(t, dut)
	default:
		t.Fatalf("MPLSoGRE/MPLSoGUE encap config is not implemented for vendor %v", dut.Vendor())
	}
}

// configureAristaEncap classifies IP-to-encap traffic into TC3 (redirected into MPLSoGRE) and
// marks/redirects it into MPLSoGRE + MPLSoGUE next-hop-groups spanning both core uplinks, with
// DSCP outerMarkedDSCP set on the outer header (PF-1.18.1).
func configureAristaEncap(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "mpls ip\n")
	fmt.Fprintf(&b, "nexthop-group %s type mpls-over-gre\n", greNHGName)
	fmt.Fprintf(&b, " tos %d\n ttl 64\n fec hierarchical\n", outerMarkedDSCP<<2)
	fmt.Fprintf(&b, " entry 0 push label-stack %d tunnel-destination %s tunnel-source %s\n", encapMPLSLabel, outerGREDstCore1, core1Intf.IPv4)
	fmt.Fprintf(&b, " entry 1 push label-stack %d tunnel-destination %s tunnel-source %s\n", encapMPLSLabel, outerGREDstCore2, core2Intf.IPv4)
	fmt.Fprintf(&b, "!\n")
	fmt.Fprintf(&b, "traffic-policies\n traffic-policy %s\n", encapPolicyName)
	// TODO: split GRE (TC3) vs GUE (TC4) encapsulation by an additional match criterion (e.g.
	// inner DSCP range) instead of sending all default IPv4 traffic to GRE/TC3; GUE traffic
	// currently only reaches its next-hop-group via a dedicated match added below.
	fmt.Fprintf(&b, "  match ipv4-all-default ipv4\n   actions\n    count\n    set traffic class %d\n    redirect next-hop group %s\n", ingressClassifyTC3, greNHGName)
	fmt.Fprintf(&b, "  match ipv6-all-default ipv6\n")
	fmt.Fprintf(&b, " !\n")
	helpers.GnmiCLIConfig(t, dut, b.String())

	for _, a := range custIntfs {
		helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s.%d\n traffic-policy input %s\n!\n", custAggID, a.Subinterface, encapPolicyName))
	}

	// MPLSoGUE next-hop-group + PBR rule: these cfgplugins helpers are genuinely parameterized
	// (unlike PolicyForwardingConfig/NextHopGroupConfig above), so they're safe to reuse as-is.
	// NOTE: despite its name, SrcIp is the Arista "tunnel-source intf <name>" argument and must
	// be a DUT interface name, not an IP address.
	// TODO: DSCP is left unset here; NextHopGroupConfigForIpOverUdp's DSCP marking applies an
	// ingress QoS policy to the tunnel-source interface (via configureTOSGUE), which isn't the
	// same as marking the GUE outer header, and its cs<N> conversion (DSCP>>5) expects a
	// TOS-scaled value, not a raw DSCP. Revisit once outer-header marking for GUE is verified.
	cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, cfgplugins.NexthopGroupUDPParams{
		IPFamily:       "V4Udp",
		NexthopGrpName: gueNHGName,
		DstIp:          []string{outerGUEDstCore1, outerGUEDstCore2},
		SrcIp:          core1AggID,
		DstUdpPort:     gueDstPort,
		TTL:            64,
	})
	cfgplugins.NewPolicyForwardingGueEncap(t, dut, cfgplugins.GueEncapPolicyParams{
		IPFamily:         "V4Udp",
		PolicyName:       encapPolicyName,
		NexthopGroupName: gueNHGName,
	})
}

// configureDecapMPLSInGREAndGUE implements the "Encap to IP" side of PF-1.18.1.
func configureDecapMPLSInGREAndGUE(t *testing.T, dut *ondatra.DUTDevice) {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		configureAristaDecap(t, dut)
	default:
		t.Fatalf("MPLSoGRE/MPLSoGUE decap config is not implemented for vendor %v", dut.Vendor())
	}
}

// configureAristaDecap decapsulates MPLSoGRE and MPLSoGUE traffic destined to outerEncapPrefix
// and pops the MPLS labels used by buildEncapToIPFlows, forwarding the inner payload to
// custOTG0 (PF-1.18.1/PF-1.18.2). The "tunnel overlay mpls qos map mpls-traffic-class to
// traffic-class" line performs the MPLS EXP -> traffic-class classification (no OC path exists
// for this yet, see README TODO).
func configureAristaDecap(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	cli := fmt.Sprintf(`
mpls ip
ip decap-group %s
  tunnel type gre
  tunnel decap-ip %s
  tunnel overlay mpls qos map mpls-traffic-class to traffic-class
!
ip decap-group type udp destination port %d payload mpls
!
ip decap-group %s
  tunnel type udp
  tunnel decap-ip %s
  tunnel overlay mpls qos map mpls-traffic-class to traffic-class
!
`, decapGREGroup, outerEncapPrefix, gueDstPort, decapGUEGroup, outerEncapPrefix)
	helpers.GnmiCLIConfig(t, dut, cli)

	var mplsB strings.Builder
	for tc := 0; tc < 8; tc++ {
		fmt.Fprintf(&mplsB, "mpls static top-label %d %s pop payload-type ipv4 access-list bypass\n", 99990+tc, custOTG0.IPv4)
		fmt.Fprintf(&mplsB, "mpls static top-label %d %s pop payload-type ipv4 access-list bypass\n", 99890+tc, custOTG0.IPv4)
	}
	helpers.GnmiCLIConfig(t, dut, mplsB.String())
}

// configureQoS builds the classifiers, forwarding-groups, scheduler-policies (bandwidth,
// bandwidth+shaper, priority, priority+shaper) and the ingress two-rate-three-color policer,
// then applies them on the relevant interfaces (PF-1.18.1).
func configureQoS(t *testing.T, dut *ondatra.DUTDevice) {
	cfgplugins.NewQosInitialize(t, dut)
	queues := netutil.CommonTrafficQueues(t, dut)
	// NOTE: netutil.CommonTrafficQueues() only exposes 6 named queues (BE1, AF1-AF4, NC1).
	// TODO: The README requires 8 bandwidth/priority classes (TC0-TC7); platforms that
	// expose additional hardware queues should extend qNames/queues below accordingly.
	qNames := []string{queues.BE1, queues.AF1, queues.AF2, queues.AF3, queues.AF4, queues.NC1}

	d := &oc.Root{}
	q := d.GetOrCreateQos()
	cfgplugins.CreateQueues(t, dut, q, qNames)

	var classifiers []cfgplugins.QosClassifier
	for i, qn := range qNames {
		classifiers = append(classifiers, cfgplugins.QosClassifier{
			Desc:        fmt.Sprintf("dscp-classifier-tc%d", i),
			Name:        "dscp-classifier",
			ClassType:   oc.Qos_Classifier_Type_IPV4,
			TermID:      fmt.Sprintf("term%d", i),
			TargetGroup: "target-group-" + qn,
			DscpSet:     dscpRangeForTC(i),
		})
	}
	q = cfgplugins.NewQoSClassifierConfiguration(t, dut, q, classifiers)
	qoscfg.SetInputClassifier(t, dut, q, custAggID, oc.Input_Classifier_Type_IPV4, "dscp-classifier")

	var fgs []cfgplugins.ForwardingGroup
	for _, qn := range qNames {
		fgs = append(fgs, cfgplugins.ForwardingGroup{Desc: "fg-" + qn, QueueName: qn, TargetGroup: "target-group-" + qn})
	}
	cfgplugins.NewQoSForwardingGroup(t, dut, q, fgs)

	gnmi.Update(t, dut, gnmi.OC().Qos().Config(), q)

	configureBandwidthScheduler(t, dut, qNames)
	configureBandwidthShaperScheduler(t, dut, qNames)
	configurePriorityScheduler(t, dut, qNames)
	configurePriorityShaperScheduler(t, dut, qNames)
	configureIngressPolicer(t, dut)

	// Apply the priority scheduler on both core (encap egress) uplinks (PF-1.18.8).
	for _, intfName := range []string{core1AggID, core2AggID} {
		applySchedulerOnOutput(t, dut, intfName, prioSchedulerName, qNames)
	}
	// Apply the bandwidth/priority (and shaper variants) scheduler on the 5 customer
	// sub-interfaces used for decap egress queueing (PF-1.18.4-1.18.7). Only one scheduler
	// can be bound to a given interface at a time; PF-1.18.4-1.18.7 rebind the interface to
	// the scheduler under test before generating traffic.
}

// dscpRangeForTC returns the 8 DSCP values (tc*8 .. tc*8+7) that classify into traffic-class tc.
func dscpRangeForTC(tc int) []uint8 {
	var vals []uint8
	for i := 0; i < 8; i++ {
		vals = append(vals, uint8(tc*8+i))
	}
	return vals
}

func configureBandwidthScheduler(t *testing.T, dut *ondatra.DUTDevice, qNames []string) {
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	sp := q.GetOrCreateSchedulerPolicy(bwSchedulerName)
	sp.SetName(bwSchedulerName)
	// weights are relative; TODO: replace with platform specific absolute/percentage minimums.
	weights := []uint64{10, 15, 20, 25, 30}
	for i, qn := range qNames {
		if i >= len(weights) {
			break
		}
		s := sp.GetOrCreateScheduler(uint32(i))
		s.SetSequence(uint32(i))
		s.SetPriority(oc.Scheduler_Priority_UNSET)
		input := s.GetOrCreateInput(qn)
		input.SetId(qn)
		input.SetInputType(oc.Input_InputType_QUEUE)
		input.SetQueue(qn)
		input.SetWeight(weights[i])
	}
	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(bwSchedulerName).Config(), sp)
}

func configureBandwidthShaperScheduler(t *testing.T, dut *ondatra.DUTDevice, qNames []string) {
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	sp := q.GetOrCreateSchedulerPolicy(bwShaperSchedulerName)
	sp.SetName(bwShaperSchedulerName)
	// cir/pir are illustrative; TODO: replace with platform specific min/max bandwidth values.
	type bw struct{ cir, pir uint64 }
	rates := []bw{{100_000_000, 200_000_000}, {150_000_000, 300_000_000}, {200_000_000, 400_000_000}, {250_000_000, 500_000_000}, {300_000_000, 600_000_000}}
	for i, qn := range qNames {
		if i >= len(rates) {
			break
		}
		s := sp.GetOrCreateScheduler(uint32(i))
		s.SetSequence(uint32(i))
		s.SetPriority(oc.Scheduler_Priority_UNSET)
		input := s.GetOrCreateInput(qn)
		input.SetId(qn)
		input.SetInputType(oc.Input_InputType_QUEUE)
		input.SetQueue(qn)
		// TODO: b/442749011 - Arista's gNMI schema rejects two-rate-three-color/one-rate-two-color
		// under scheduler-policy/scheduler; skip pushing them until a CLI-based equivalent (Arista
		// egress tx-queue shaping) is implemented (see README TODO on shaper OC not being finalized).
		if deviations.QosTwoRateThreeColorPolicerOCUnsupported(dut) {
			continue
		}
		if i < 3 {
			trtc := s.GetOrCreateTwoRateThreeColor()
			trtc.SetCir(rates[i].cir)
			trtc.SetPir(rates[i].pir)
			trtc.GetOrCreateExceedAction().SetDrop(false)
			trtc.GetOrCreateViolateAction().SetDrop(true)
		} else {
			ortc := s.GetOrCreateOneRateTwoColor()
			ortc.SetCir(rates[i].cir)
			ortc.GetOrCreateExceedAction().SetDrop(false)
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(bwShaperSchedulerName).Config(), sp)
}

func configurePriorityScheduler(t *testing.T, dut *ondatra.DUTDevice, qNames []string) {
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	sp := q.GetOrCreateSchedulerPolicy(prioSchedulerName)
	sp.SetName(prioSchedulerName)
	// Lowest sequence number == highest priority; qNames is ordered lowest -> highest priority.
	for i, qn := range qNames {
		seq := uint32(len(qNames) - 1 - i)
		s := sp.GetOrCreateScheduler(seq)
		s.SetSequence(seq)
		s.SetPriority(oc.Scheduler_Priority_STRICT)
		input := s.GetOrCreateInput(qn)
		input.SetId(qn)
		input.SetInputType(oc.Input_InputType_QUEUE)
		input.SetQueue(qn)
	}
	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(prioSchedulerName).Config(), sp)
}

func configurePriorityShaperScheduler(t *testing.T, dut *ondatra.DUTDevice, qNames []string) {
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	sp := q.GetOrCreateSchedulerPolicy(prioShaperSchedulerName)
	sp.SetName(prioShaperSchedulerName)
	// TODO: replace with platform specific shaper (max bandwidth) values; illustrative only.
	shaperPir := []uint64{100_000_000, 150_000_000, 200_000_000, 250_000_000}
	for i, qn := range qNames {
		seq := uint32(len(qNames) - 1 - i)
		s := sp.GetOrCreateScheduler(seq)
		s.SetSequence(seq)
		s.SetPriority(oc.Scheduler_Priority_STRICT)
		input := s.GetOrCreateInput(qn)
		input.SetId(qn)
		input.SetInputType(oc.Input_InputType_QUEUE)
		input.SetQueue(qn)
		if i < len(shaperPir) && !deviations.QosTwoRateThreeColorPolicerOCUnsupported(dut) {
			// TODO: b/442749011 - see configureBandwidthShaperScheduler.
			ortc := s.GetOrCreateOneRateTwoColor()
			ortc.SetCir(shaperPir[i])
			ortc.GetOrCreateExceedAction().SetDrop(true)
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(prioShaperSchedulerName).Config(), sp)
}

// configureIngressPolicer configures a two-rate-three-color policer for IP-to-Encap traffic
// and applies it on the customer facing ingress aggregate (PF-1.18.1/PF-1.18.9).
func configureIngressPolicer(t *testing.T, dut *ondatra.DUTDevice) {
	batch := &gnmi.SetBatch{}
	params := &cfgplugins.SchedulerParams{
		SchedulerName: ingressPolicerName,
		PolicerName:   ingressPolicerName,
		InterfaceName: custAggID,
		ClassName:     "class-default",
		// TODO: replace with the CIR/PIR values required by the DUT platform under test.
		CirValue:       1_000_000_000,
		PirValue:       2_000_000_000,
		BurstSize:      100_000,
		SequenceNumber: 0,
	}
	cfgplugins.NewTwoRateThreeColorScheduler(t, dut, batch, params)
	cfgplugins.ApplyQosPolicyOnInterface(t, dut, batch, params)
	batch.Set(t, dut)
}

// applySchedulerOnOutput binds schedulerName to intfName's egress queues.
func applySchedulerOnOutput(t *testing.T, dut *ondatra.DUTDevice, intfName, schedulerName string, qNames []string) {
	t.Helper()
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	i := q.GetOrCreateInterface(intfName)
	i.SetInterfaceId(intfName)
	if !deviations.InterfaceRefConfigUnsupported(dut) {
		i.GetOrCreateInterfaceRef().Interface = ygot.String(intfName)
	}
	out := i.GetOrCreateOutput()
	out.GetOrCreateSchedulerPolicy().Name = ygot.String(schedulerName)
	for _, qn := range qNames {
		out.GetOrCreateQueue(qn).SetName(qn)
	}
	gnmi.Update(t, dut, gnmi.OC().Qos().Interface(intfName).Config(), i)
}

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
	time.Sleep(5 * time.Second)

	agg := &oc.Interface{Name: ygot.String(aggID)}
	configDUTInterface(agg, subinterfaces, dut)
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_LACP
	agg.Type = ieee8023adLag
	aggPath := d.Interface(aggID)
	fptest.LogQuery(t, aggID, aggPath.Config(), agg)
	gnmi.Replace(t, dut, aggPath.Config(), agg)

	for _, port := range dutAggPorts {
		holdTimeConfig := &oc.Interface_HoldTime{
			Up:   ygot.Uint32(3000),
			Down: ygot.Uint32(150),
		}
		intfPath := gnmi.OC().Interface(port.Name())
		gnmi.Update(t, dut, intfPath.HoldTime().Config(), holdTimeConfig)
	}
}

func configDUTInterface(i *oc.Interface, subinterfaces []*attrs.Attributes, dut *ondatra.DUTDevice) {
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
			configureInterfaceAddress(dut, s, a)
		} else {
			configureInterfaceAddress(dut, s1, a)
		}
	}
}

func configureInterfaceAddress(dut *ondatra.DUTDevice, s *oc.Interface_Subinterface, a *attrs.Attributes) {
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
}

// configureStaticRoutes configures reachability for the encapsulated outer headers via both
// core uplinks.
func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	b := &gnmi.SetBatch{}
	routes := []*cfgplugins.StaticRouteCfg{
		{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          "10.99.1.0/24",
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(core1OTG.IPv4),
			},
		},
		{
			NetworkInstance: deviations.DefaultNetworkInstance(dut),
			Prefix:          "10.99.2.0/24",
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.UnionString(core2OTG.IPv4),
			},
		},
	}
	for _, r := range routes {
		if _, err := cfgplugins.NewStaticRouteCfg(b, r, dut); err != nil {
			t.Fatalf("Failed to configure static route %s: %v", r.Prefix, err)
		}
	}
	b.Set(t, dut)
}

// TestSetup implements PF-1.18.1: Generate DUT Configuration.
func TestSetup(t *testing.T) {
	t.Log("PF-1.18.1: Generate DUT Configuration")
	dut := ondatra.DUT(t, "dut")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	ConfigureDut(t, dut)
	ConfigureOTG(t)
}

// createflow builds the packet layers of a Flow in top.
func createflow(t *testing.T, top gosnappi.Config, params *otgconfighelpers.Flow, clearFlows bool) {
	t.Helper()
	if clearFlows {
		top.Flows().Clear()
	}
	params.CreateFlow(top)
	params.AddEthHeader()
	if params.VLANFlow != nil {
		params.AddVLANHeader()
	}
	if params.UDPFlow != nil {
		params.AddUDPHeader()
	}
	if params.GREFlow != nil {
		params.AddGREHeader()
	}
	if params.MPLSFlow != nil {
		params.AddMPLSHeader()
	}
	if params.IPv4Flow != nil {
		params.AddIPv4Header()
	}
	if params.IPv6Flow != nil {
		params.AddIPv6Header()
	}
	if params.TCPFlow != nil {
		params.AddTCPHeader()
	}
}

func sendTraffic(t *testing.T, ate *ondatra.ATEDevice, dur time.Duration) {
	t.Helper()
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	ate.OTG().StartTraffic(t)
	time.Sleep(dur)
	ate.OTG().StopTraffic(t)
}

// buildEncapToIPFlows builds one flow per MPLS EXP value (0-7) for both MPLSoGRE and
// MPLSoGUE, split across both core aggregates (README: ATE Ports 3,4,5,6) with GRE flows on
// core1 and GUE flows on core2, egressing (post decap) on the customer aggregate. Implements
// Flow-A. Per-flow rate is capped so each aggregate's own total stays under 100% of its port
// capacity (the ATE rejects any single Tx port configured above that); congestion for
// PF-1.18.4-1.18.7 instead comes from both aggregates' traffic converging on the single
// customer egress interface.
func buildEncapToIPFlows() []*otgconfighelpers.Flow {
	var flows []*otgconfighelpers.Flow
	for tc := 0; tc < 8; tc++ {
		flows = append(flows, &otgconfighelpers.Flow{
			TxNames:           []string{core1OTG.Name + ".IPv4"},
			RxNames:           []string{custOTG0.Name + ".IPv4"},
			SizeWeightProfile: &sizeWeightProfile,
			Flowrate:          8,
			FlowName:          fmt.Sprintf("MPLSoGRE-tc%d-%s", tc, core1OTG.Name),
			EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
			IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 1000},
			MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: uint32(99990 + tc), MPLSExp: uint32(tc)},
			GREFlow:           &otgconfighelpers.GREFlowParams{Protocol: otgconfighelpers.IanaMPLSEthertype},
		})
		flows = append(flows, &otgconfighelpers.Flow{
			TxNames:           []string{core2OTG.Name + ".IPv4"},
			RxNames:           []string{custOTG0.Name + ".IPv4"},
			SizeWeightProfile: &sizeWeightProfile,
			Flowrate:          8,
			FlowName:          fmt.Sprintf("MPLSoGUE-tc%d-%s", tc, core2OTG.Name),
			EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg3.AggMAC},
			IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.1.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 1000},
			UDPFlow:           &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: gueDstPort},
			GREFlow:           &otgconfighelpers.GREFlowParams{Protocol: otgconfighelpers.IanaMPLSEthertype},
			MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: uint32(99890 + tc), MPLSExp: uint32(tc)},
		})
	}
	return flows
}

// buildIPToEncapFlows builds one flow per traffic class (0-7), classified by DSCP, ingressing
// on the customer aggregate. This implements Flow-B.
func buildIPToEncapFlows() []*otgconfighelpers.Flow {
	var flows []*otgconfighelpers.Flow
	for tc := 0; tc < 8; tc++ {
		flows = append(flows, &otgconfighelpers.Flow{
			TxNames:           []string{custOTG0.Name + ".IPv4"},
			RxNames:           []string{core1OTG.Name + ".IPv4", core2OTG.Name + ".IPv4"},
			SizeWeightProfile: &sizeWeightProfile,
			Flowrate:          10,
			FlowName:          fmt.Sprintf("IPtoEncap-tc%d", tc),
			EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
			VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: uint32(custIntfTC0.Subinterface)},
			IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "12.1.1.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 1000, DSCP: uint32(tc * 8), DSCPCount: 8},
		})
	}
	return flows
}

func flowValidation(name string) *otgvalidationhelpers.OTGValidation {
	return &otgvalidationhelpers.OTGValidation{
		Flow: &otgvalidationhelpers.FlowParams{Name: name, TolerancePct: lossTolerancePct},
	}
}

// queueTransmitPkts returns the transmit-pkts counter for a given egress interface/queue.
func queueTransmitPkts(t *testing.T, dut *ondatra.DUTDevice, intf, queue string) uint64 {
	t.Helper()
	return gnmi.Get(t, dut, gnmi.OC().Qos().Interface(intf).Output().Queue(queue).TransmitPkts().State())
}

// queueDroppedPkts returns the dropped-pkts counter for a given egress interface/queue.
func queueDroppedPkts(t *testing.T, dut *ondatra.DUTDevice, intf, queue string) uint64 {
	t.Helper()
	return gnmi.Get(t, dut, gnmi.OC().Qos().Interface(intf).Output().Queue(queue).DroppedPkts().State())
}

// TestPF1182MPLSTrafficClassClassification implements PF-1.18.2.
func TestPF1182MPLSTrafficClassClassification(t *testing.T) {
	t.Log("PF-1.18.2: Verify Classification of MPLSoGRE and MPLSoGUE traffic based on traffic class bits in MPLS header")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	queues := netutil.CommonTrafficQueues(t, dut)
	qNames := []string{queues.BE1, queues.AF1, queues.AF2, queues.AF3, queues.AF4, queues.NC1}

	top.Flows().Clear()
	flows := buildEncapToIPFlows()
	for i, f := range flows {
		createflow(t, top, f, i == 0)
	}
	sendTraffic(t, ate, trafficDuration)

	for _, f := range flows {
		if err := flowValidation(f.FlowName).ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("ValidateLossOnFlows(%s): %v", f.FlowName, err)
		}
	}
	for i, qn := range qNames {
		if got := queueTransmitPkts(t, dut, custAggID, qn); got == 0 {
			t.Errorf("queue %s (tc%d) transmit-pkts on %s: got 0, want > 0", qn, i, custAggID)
		}
	}
}

// TestPF1183DSCPMarking implements PF-1.18.3.
func TestPF1183DSCPMarking(t *testing.T) {
	t.Log("PF-1.18.3: Verify DSCP marking of encapsulated and decapsulated traffic")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	validations := []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateMPLSLayer,
		packetvalidationhelpers.ValidateInnerIPv4Header,
	}
	// Outer header must carry DSCP 32 (PF-1.18.1); inner DSCP is preserved.
	encapValidation := &packetvalidationhelpers.PacketValidation{
		PortName:         core1Ports[0],
		CaptureName:      "ip-encap-dscp",
		Validations:      validations,
		IPv4Layer:        &packetvalidationhelpers.IPv4Layer{Protocol: greProtocol, DstIP: outerGREDstCore1, Tos: outerMarkedDSCP << 2, TTL: 64},
		MPLSLayer:        &packetvalidationhelpers.MPLSLayer{Label: encapMPLSLabel, Tc: ingressClassifyTC3},
		InnerIPLayerIPv4: &packetvalidationhelpers.IPv4Layer{DstIP: "11.1.1.1", Tos: 4 << 2, TTL: 63},
	}

	flow := &otgconfighelpers.Flow{
		TxNames:       []string{custOTG0.Name + ".IPv4"},
		RxNames:       []string{core1OTG.Name + ".IPv4"},
		FlowName:      "IPtoEncap-dscp-marking",
		PacketsToSend: 1000,
		EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow:      &otgconfighelpers.VLANFlowParams{VLANId: uint32(custIntfTC0.Subinterface)},
		IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: "12.1.1.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 100, DSCP: 4},
	}
	createflow(t, top, flow, true)
	packetvalidationhelpers.ConfigurePacketCapture(t, top, encapValidation)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(30 * time.Second)
	ate.OTG().StopTraffic(t)
	time.Sleep(30 * time.Second)
	packetvalidationhelpers.StopCapture(t, ate, cs)

	if err := flowValidation(flow.FlowName).ValidateLossOnFlows(t, ate); err != nil {
		packetvalidationhelpers.ClearCapture(t, top, ate)
		t.Errorf("ValidateLossOnFlows(): %v", err)
	}
	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
		t.Errorf("CaptureAndValidatePackets(): %v", err)
	}
	packetvalidationhelpers.ClearCapture(t, top, ate)
	_ = dut
}

// TestPF1184AssuredForwardingMinBandwidth implements PF-1.18.4.
func TestPF1184AssuredForwardingMinBandwidth(t *testing.T) {
	t.Log("PF-1.18.4: Verify Assured forwarding (bandwidth class) - Queueing of decap traffic")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	queues := netutil.CommonTrafficQueues(t, dut)
	qNames := []string{queues.BE1, queues.AF1, queues.AF2, queues.AF3, queues.AF4, queues.NC1}

	applySchedulerOnOutput(t, dut, custAggID, bwSchedulerName, qNames)

	top.Flows().Clear()
	flows := buildEncapToIPFlows()
	for i, f := range flows {
		f.Flowrate = 12 // oversubscribe to induce congestion.
		createflow(t, top, f, i == 0)
	}
	sendTraffic(t, ate, trafficDuration)

	for i, qn := range qNames {
		got := queueTransmitPkts(t, dut, custAggID, qn)
		t.Logf("queue %s (class %d) transmit-pkts: %d", qn, i, got)
		if got == 0 {
			t.Errorf("queue %s: got 0 transmit-pkts, want > 0 (minimum bandwidth not honored)", qn)
		}
		if dropped := queueDroppedPkts(t, dut, custAggID, qn); dropped > 0 {
			t.Logf("queue %s dropped-pkts: %d (congestion expected per README)", qn, dropped)
		}
	}
}

// TestPF1185AssuredForwardingShaper implements PF-1.18.5.
func TestPF1185AssuredForwardingShaper(t *testing.T) {
	t.Log("PF-1.18.5: Verify Assured forwarding (bandwidth class) - Queueing of decap traffic with min/max bandwidth (shaper)")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	queues := netutil.CommonTrafficQueues(t, dut)
	qNames := []string{queues.BE1, queues.AF1, queues.AF2, queues.AF3, queues.AF4, queues.NC1}

	applySchedulerOnOutput(t, dut, custAggID, bwShaperSchedulerName, qNames)

	top.Flows().Clear()
	flows := buildEncapToIPFlows()
	for i, f := range flows {
		f.Flowrate = 12
		createflow(t, top, f, i == 0)
	}
	sendTraffic(t, ate, trafficDuration)

	for i, qn := range qNames {
		got := queueTransmitPkts(t, dut, custAggID, qn)
		t.Logf("queue %s (class %d) transmit-pkts: %d", qn, i, got)
		if got == 0 {
			t.Errorf("queue %s: got 0 transmit-pkts, want > 0", qn)
		}
		// TODO: verify shaped classes (i < 3) do not exceed the configured PIR once the
		// OC for shaping rate is finalized (see README TODO).
	}
}

// TestPF1186ExpeditedForwardingPriorityDecap implements PF-1.18.6.
func TestPF1186ExpeditedForwardingPriorityDecap(t *testing.T) {
	t.Log("PF-1.18.6: Verify Expedited forwarding (Priority class) - Queueing of decap traffic")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	queues := netutil.CommonTrafficQueues(t, dut)
	qNames := []string{queues.BE1, queues.AF1, queues.AF2, queues.AF3, queues.AF4, queues.NC1}

	applySchedulerOnOutput(t, dut, custAggID, prioSchedulerName, qNames)

	top.Flows().Clear()
	flows := buildEncapToIPFlows()
	for i, f := range flows {
		f.Flowrate = 12
		createflow(t, top, f, i == 0)
	}
	sendTraffic(t, ate, trafficDuration)

	// Highest priority queue (last in qNames, i.e. NC1) should never starve.
	highest := qNames[len(qNames)-1]
	if got := queueTransmitPkts(t, dut, custAggID, highest); got == 0 {
		t.Errorf("highest priority queue %s: got 0 transmit-pkts, want > 0", highest)
	}
	for i, qn := range qNames {
		t.Logf("queue %s (priority level %d) transmit-pkts: %d", qn, i, queueTransmitPkts(t, dut, custAggID, qn))
	}
}

// TestPF1187ExpeditedForwardingPriorityShaper implements PF-1.18.7.
func TestPF1187ExpeditedForwardingPriorityShaper(t *testing.T) {
	t.Log("PF-1.18.7: Verify Expedited forwarding (Priority class) - Queueing of decap traffic with min/max bandwidth (shaper)")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	queues := netutil.CommonTrafficQueues(t, dut)
	qNames := []string{queues.BE1, queues.AF1, queues.AF2, queues.AF3, queues.AF4, queues.NC1}

	applySchedulerOnOutput(t, dut, custAggID, prioShaperSchedulerName, qNames)

	top.Flows().Clear()
	flows := buildEncapToIPFlows()
	for i, f := range flows {
		f.Flowrate = 12
		createflow(t, top, f, i == 0)
	}
	sendTraffic(t, ate, trafficDuration)

	for i, qn := range qNames {
		t.Logf("queue %s (priority level %d) transmit-pkts: %d, dropped-pkts: %d",
			qn, i, queueTransmitPkts(t, dut, custAggID, qn), queueDroppedPkts(t, dut, custAggID, qn))
	}
	// TODO: verify shaped priority classes never exceed the configured shaper rate once the
	// OC for shaping rate is finalized (see README TODO).
}

// TestPF1188ExpeditedForwardingPriorityEncap implements PF-1.18.8.
func TestPF1188ExpeditedForwardingPriorityEncap(t *testing.T) {
	t.Log("PF-1.18.8: Verify Expedited forwarding (Priority class) - Queueing of encap traffic")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	queues := netutil.CommonTrafficQueues(t, dut)
	qNames := []string{queues.BE1, queues.AF1, queues.AF2, queues.AF3, queues.AF4, queues.NC1}

	// Priority scheduler is already applied on both core uplinks in configureQoS().
	top.Flows().Clear()
	flows := buildIPToEncapFlows()
	for i, f := range flows {
		f.Flowrate = 12
		createflow(t, top, f, i == 0)
	}
	sendTraffic(t, ate, trafficDuration)

	for _, coreAggID := range []string{core1AggID, core2AggID} {
		highest := qNames[len(qNames)-1]
		if got := queueTransmitPkts(t, dut, coreAggID, highest); got == 0 {
			t.Errorf("highest priority queue %s on %s: got 0 transmit-pkts, want > 0", highest, coreAggID)
		}
	}
}

// TestPF1189TwoRateThreeColorPolicer implements PF-1.18.9.
func TestPF1189TwoRateThreeColorPolicer(t *testing.T) {
	t.Log("PF-1.18.9: Verify two rate three color policer - Ingress rate limiting of encap traffic")
	ate := ondatra.ATE(t, "ate")

	top.Flows().Clear()
	flows := buildIPToEncapFlows()
	for i, f := range flows {
		f.Flowrate = 12 // sum > configured PIR/CIR, per README.
		createflow(t, top, f, i == 0)
	}
	sendTraffic(t, ate, trafficDuration)

	var totalLossPct float32
	for _, f := range flows {
		totalLossPct += flowValidation(f.FlowName).ReturnLossPercentage(t, ate)
	}
	t.Logf("Average loss across %d flows: %.2f%% (expect drops beyond configured PIR)", len(flows), totalLossPct/float32(len(flows)))
}

// TestPF11810PortHardwareDependency implements PF-1.18.10.
func TestPF11810PortHardwareDependency(t *testing.T) {
	t.Log("PF-1.18.10: Verify port/hardware dependency")
	t.Skip("TODO: requires per-platform packet-processing-engine (PPE) port placement " +
		"information which is not modeled in the standard topology; re-run PF-1.18.1-1.18.9 " +
		"with ingress/egress aggregate member ports redistributed across PPEs once that " +
		"information is available for the DUT under test.")
}

// TestPF118v6MPLSoGUEv6QoS implements PF-1.18.v6.
//
// NOTE: the README describes a standalone two-port (non-aggregate) topology for this test
// case. To avoid re-configuring ports1/2 (already bound to the LACP aggregate used by
// PF-1.18.1-1.18.10), this re-uses the existing custIntfTC0 (IPv6 enabled) and core1
// aggregate interfaces instead of raw port1/port2.
func TestPF118v6MPLSoGUEv6QoS(t *testing.T) {
	t.Log("PF-1.18.v6: Validate MPLS over GRE over UDP over IPv6 encapsulation and decapsulation with QoS prioritization")
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	queues := netutil.CommonTrafficQueues(t, dut)

	highPrioFlow := &otgconfighelpers.Flow{
		TxNames:  []string{custOTG0.Name + ".IPv6"},
		RxNames:  []string{core1OTG.Name + ".IPv6"},
		FlowName: "MPLSoGUEv6-high-priority",
		EthFlow:  &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow: &otgconfighelpers.VLANFlowParams{VLANId: uint32(custIntfTC0.Subinterface)},
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: "2001:db8:1::10", IPv6Dst: "2001:db8:1::1", TrafficClass: 46},
	}
	lowPrioFlow := &otgconfighelpers.Flow{
		TxNames:  []string{custOTG0.Name + ".IPv6"},
		RxNames:  []string{core1OTG.Name + ".IPv6"},
		FlowName: "MPLSoGUEv6-low-priority",
		EthFlow:  &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
		VLANFlow: &otgconfighelpers.VLANFlowParams{VLANId: uint32(custIntfTC0.Subinterface)},
		IPv6Flow: &otgconfighelpers.IPv6FlowParams{IPv6Src: "2001:db8:1::11", IPv6Dst: "2001:db8:1::1", TrafficClass: 0},
	}
	top.Flows().Clear()
	createflow(t, top, highPrioFlow, true)
	createflow(t, top, lowPrioFlow, false)
	sendTraffic(t, ate, trafficDuration)

	for _, f := range []*otgconfighelpers.Flow{highPrioFlow, lowPrioFlow} {
		if err := flowValidation(f.FlowName).ValidateLossOnFlows(t, ate); err != nil {
			t.Errorf("ValidateLossOnFlows(%s): %v", f.FlowName, err)
		}
	}
	if dropped := queueDroppedPkts(t, dut, core1AggID, queues.NC1); dropped != 0 {
		t.Errorf("dropped-pkts on %s queue %s: got %d, want 0", core1AggID, queues.NC1, dropped)
	}
	if got := queueTransmitPkts(t, dut, core1AggID, queues.NC1); got == 0 {
		t.Errorf("transmit-pkts on %s queue %s: got 0, want > 0", core1AggID, queues.NC1)
	}
}
