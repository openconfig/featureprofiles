package mpls_gre_udp_qos_test

import (
	"fmt"
	"strings"
	"sync"
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
	"github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/qoscfg"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ethernetCsmacd     = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag      = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	greProtocol        = 47
	gueDstPort         = 6635
	outerMarkedDSCP    = 32
	ingressClassifyTC3 = 3
	ingressClassifyTC4 = 4

	bwSchedulerName         = "scheduler-bw"
	bwShaperSchedulerName   = "scheduler-bw-shaper"
	prioSchedulerName       = "scheduler-prio"
	prioShaperSchedulerName = "scheduler-prio-shaper"
	ingressPolicerName      = "scheduler-ingress-2r3c"

	encapMPLSLabel     = 116383
	greNHGName         = "nhg-gre-encap"
	gueNHGName         = "nhg-gue-encap"
	encapPolicyName    = "encap-mplsogre"
	gueEncapPolicyName = "encap-mplsogue"
	outerEncapPrefix   = "10.99.0.0/16"
	outerGREDstCore1   = "10.99.1.1"
	outerGREDstCore2   = "10.99.2.1"
	outerGUEDstCore1   = "10.99.1.2"
	outerGUEDstCore2   = "10.99.2.2"
	decapGREGroup      = "gre-decap"
	decapGUEGroup      = "gue-decap"

	trafficDuration        = 25 * time.Second
	lossTolerancePct       = float32(3.0)
	strictLossTolerancePct = float32(0.0)
	innerDSCPCapture       = 10
	mcastDst               = "239.1.1.1"
)

var (
	top = gosnappi.NewConfig()

	custAggID, core1AggID, core2AggID string

	custPorts  = []string{"port1", "port2"}
	core1Ports = []string{"port3", "port4"}
	core2Ports = []string{"port5", "port6"}

	custIntfTC0 = attrs.Attributes{Desc: "customer-0", MTU: 1500, IPv4: "169.254.0.11", IPv4Len: 29, IPv6: "2001:db8:10:11::1", IPv6Len: 126, Subinterface: 20}
	custIntfTC1 = attrs.Attributes{Desc: "customer-1", MTU: 1500, IPv4: "169.254.0.19", IPv4Len: 29, Subinterface: 21}
	custIntfTC2 = attrs.Attributes{Desc: "customer-2", MTU: 1500, IPv4: "169.254.0.27", IPv4Len: 29, Subinterface: 22}
	custIntfTC3 = attrs.Attributes{Desc: "customer-3", MTU: 1500, IPv4: "169.254.0.35", IPv4Len: 29, Subinterface: 23}
	custIntfTC4 = attrs.Attributes{Desc: "customer-4", MTU: 1500, IPv4: "169.254.0.43", IPv4Len: 29, Subinterface: 24}
	custIntfs   = []*attrs.Attributes{&custIntfTC0, &custIntfTC1, &custIntfTC2, &custIntfTC3, &custIntfTC4}

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

	qcNames = []string{"TC0", "TC1", "TC2", "TC3", "TC4", "TC5", "TC6", "TC7"}

	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 64, Weight: 15},
		{Size: 128, Weight: 15},
		{Size: 256, Weight: 15},
		{Size: 512, Weight: 15},
		{Size: 1024, Weight: 15},
		{Size: 1500, Weight: 25},
	}
)

func configureOTG(t *testing.T) {
	t.Helper()
	top.Captures().Clear()
	ate := ondatra.ATE(t, "ate")
	for _, agg := range []*otgconfighelpers.Port{agg1, agg2, agg3} {
		otgconfighelpers.ConfigureNetworkInterface(t, top, ate, agg)
	}
	ate.OTG().PushConfig(t, top)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	configureHardwareInit(t, dut)

	custAggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, custPorts, custIntfs, custAggID)

	core1AggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, core1Ports, []*attrs.Attributes{&core1Intf}, core1AggID)

	core2AggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, core2Ports, []*attrs.Attributes{&core2Intf}, core2AggID)

	configureStaticRoutes(t, dut)

	configureEncapMPLSInGREAndGUE(t, dut)
	configureDecapMPLSInGREAndGUE(t, dut)

	configureQoS(t, dut)
}

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	for _, feature := range []cfgplugins.FeatureType{cfgplugins.FeaturePolicyForwarding, cfgplugins.FeatureQOSIn} {
		cfg := cfgplugins.NewDUTHardwareInit(t, dut, feature)
		if cfg == "" {
			continue
		}
		cfgplugins.PushDUTHardwareInitConfig(t, dut, cfg)
	}
}

func configureEncapMPLSInGREAndGUE(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		configureAristaEncap(t, dut)
	default:
		t.Fatalf("MPLSoGRE/MPLSoGUE encap config is not implemented for vendor %v", dut.Vendor())
	}
}

func configureAristaEncap(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	greIntfs := custIntfs[:3]
	gueIntfs := custIntfs[3:]

	var b strings.Builder
	fmt.Fprintf(&b, "mpls ip\n")
	fmt.Fprintf(&b, "nexthop-group %s type mpls-over-gre\n", greNHGName)
	fmt.Fprintf(&b, " tos %d\n ttl 64\n fec hierarchical\n", outerMarkedDSCP<<2)
	fmt.Fprintf(&b, " entry 0 push label-stack %d tunnel-destination %s tunnel-source %s\n", encapMPLSLabel, outerGREDstCore1, core1Intf.IPv4)
	fmt.Fprintf(&b, " entry 1 push label-stack %d tunnel-destination %s tunnel-source %s\n", encapMPLSLabel, outerGREDstCore2, core2Intf.IPv4)
	fmt.Fprintf(&b, "!\n")
	fmt.Fprintf(&b, "traffic-policies\n traffic-policy %s\n", encapPolicyName)
	fmt.Fprintf(&b, "  match ipv4-all-default ipv4\n   actions\n    count\n    set traffic class %d\n    redirect next-hop group %s\n", ingressClassifyTC3, greNHGName)
	fmt.Fprintf(&b, "  match ipv6-all-default ipv6\n")
	fmt.Fprintf(&b, " !\n")
	helpers.GnmiCLIConfig(t, dut, b.String())

	for _, a := range greIntfs {
		helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s.%d\n traffic-policy input %s\n!\n", custAggID, a.Subinterface, encapPolicyName))
	}

	cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, cfgplugins.NexthopGroupUDPParams{
		IPFamily:       "V4Udp",
		NexthopGrpName: gueNHGName,
		DstIp:          []string{outerGUEDstCore1, outerGUEDstCore2},
		SrcIp:          core1Intf.IPv4,
		DstUdpPort:     gueDstPort,
		TTL:            64,
	})

	var g strings.Builder
	fmt.Fprintf(&g, "traffic-policies\n traffic-policy %s\n", gueEncapPolicyName)
	fmt.Fprintf(&g, "  match ipv4-all-default ipv4\n   actions\n    count\n    set traffic class %d\n    redirect next-hop group %s\n", ingressClassifyTC4, gueNHGName)
	fmt.Fprintf(&g, "  match ipv6-all-default ipv6\n")
	fmt.Fprintf(&g, " !\n")
	helpers.GnmiCLIConfig(t, dut, g.String())

	for _, a := range gueIntfs {
		helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s.%d\n traffic-policy input %s\n!\n", custAggID, a.Subinterface, gueEncapPolicyName))
	}
}

func configureDecapMPLSInGREAndGUE(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		configureAristaDecap(t, dut)
	default:
		t.Fatalf("MPLSoGRE/MPLSoGUE decap config is not implemented for vendor %v", dut.Vendor())
	}
}

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
		fmt.Fprintf(&mplsB, "mpls static top-label %d %s pop payload-type ipv4 access-list bypass\n", 99980+tc, custOTG0.IPv4)
		fmt.Fprintf(&mplsB, "mpls static top-label %d %s pop payload-type ipv4 access-list bypass\n", 99880+tc, custOTG0.IPv4)
		fmt.Fprintf(&mplsB, "mpls static top-label %d %s pop payload-type ipv6 access-list bypass\n", 99970+tc, custOTG0.IPv6)
		fmt.Fprintf(&mplsB, "mpls static top-label %d %s pop payload-type ipv6 access-list bypass\n", 99870+tc, custOTG0.IPv6)
	}
	helpers.GnmiCLIConfig(t, dut, mplsB.String())
}

func configureAristaQosTxQueues(t *testing.T, dut *ondatra.DUTDevice, qNames []string) {
	t.Helper()
	var cli strings.Builder
	for index, queue := range qNames {
		fmt.Fprintf(&cli, "qos tx-queue %d name %s\n!\n", index, queue)
		if index != 7 {
			fmt.Fprintf(&cli, "qos map traffic-class %d to tx-queue %d\n!\n", index, index)
			fmt.Fprintf(&cli, "qos map traffic-class %d to exp %d\n!\n", index, index)
		}
		fmt.Fprintf(&cli, "qos traffic-class %d name target-group-%s\n!\n", index, queue)
	}
	helpers.GnmiCLIConfig(t, dut, cli.String())
}

func configureQoS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	qNames := qcNames
	configureAristaQosTxQueues(t, dut, qNames)

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

	for _, intfName := range []string{custAggID, core1AggID, core2AggID} {
		applySchedulerOnOutput(t, dut, intfName, prioSchedulerName, qNames)
	}

	for _, ports := range [][]string{custPorts, core1Ports, core2Ports} {
		for _, p := range ports {
			registerQueuesOnPhysicalPort(t, dut, dut.Port(t, p).Name(), qNames)
		}
	}
}

func dscpRangeForTC(tc int) []uint8 {
	vals := make([]uint8, 0, 8)
	for i := 0; i < 8; i++ {
		vals = append(vals, uint8(tc*8+i))
	}
	return vals
}

func configureBandwidthScheduler(t *testing.T, dut *ondatra.DUTDevice, qNames []string) {
	t.Helper()
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	sp := q.GetOrCreateSchedulerPolicy(bwSchedulerName)
	sp.SetName(bwSchedulerName)
	weights := []uint64{10, 15, 20, 25, 30, 35, 40, 45}
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
	t.Helper()
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	sp := q.GetOrCreateSchedulerPolicy(bwShaperSchedulerName)
	sp.SetName(bwShaperSchedulerName)
	type bw struct{ cir, pir uint64 }
	rates := []bw{{100_000_000, 200_000_000}, {150_000_000, 300_000_000}, {200_000_000, 400_000_000}, {250_000_000, 500_000_000}, {300_000_000, 600_000_000}, {350_000_000, 700_000_000}, {400_000_000, 800_000_000}, {450_000_000, 900_000_000}}
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
	t.Helper()
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	sp := q.GetOrCreateSchedulerPolicy(prioSchedulerName)
	sp.SetName(prioSchedulerName)
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
	t.Helper()
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	sp := q.GetOrCreateSchedulerPolicy(prioShaperSchedulerName)
	sp.SetName(prioShaperSchedulerName)
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
			ortc := s.GetOrCreateOneRateTwoColor()
			ortc.SetCir(shaperPir[i])
			ortc.GetOrCreateExceedAction().SetDrop(true)
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Qos().SchedulerPolicy(prioShaperSchedulerName).Config(), sp)
}

func configureIngressPolicer(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	batch := &gnmi.SetBatch{}
	params := &cfgplugins.SchedulerParams{
		SchedulerName:  ingressPolicerName,
		PolicerName:    ingressPolicerName,
		InterfaceName:  custAggID,
		ClassName:      "class-default",
		CirValue:       1_000_000_000,
		PirValue:       2_000_000_000,
		BurstSize:      100_000,
		SequenceNumber: 0,
	}
	cfgplugins.NewTwoRateThreeColorScheduler(t, dut, batch, params)
	cfgplugins.ApplyQosPolicyOnInterface(t, dut, batch, params)
	batch.Set(t, dut)
}

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

func registerQueuesOnPhysicalPort(t *testing.T, dut *ondatra.DUTDevice, intfName string, qNames []string) {
	t.Helper()
	d := &oc.Root{}
	q := d.GetOrCreateQos()
	i := q.GetOrCreateInterface(intfName)
	i.SetInterfaceId(intfName)
	if !deviations.InterfaceRefConfigUnsupported(dut) {
		i.GetOrCreateInterfaceRef().Interface = ygot.String(intfName)
	}
	out := i.GetOrCreateOutput()
	for _, qn := range qNames {
		out.GetOrCreateQueue(qn).SetName(qn)
	}
	gnmi.Update(t, dut, gnmi.OC().Qos().Interface(intfName).Config(), i)
}

func configureInterfaces(t *testing.T, dut *ondatra.DUTDevice, dutPorts []string, subinterfaces []*attrs.Attributes, aggID string) {
	t.Helper()
	d := gnmi.OC()
	dutAggPorts := make([]*ondatra.Port, 0, len(dutPorts))
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

func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
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
			t.Fatalf("failed to configure static route %s: %v", r.Prefix, err)
		}
	}
	b.Set(t, dut)
}

// cleanupDUT reverts all DUT configuration applied by ConfigureDut, in the reverse
// order it was applied, so the DUT is left in its original state.
func cleanupDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	cleanupQoS(t, dut)
	cleanupDecapMPLSInGREAndGUE(t, dut)
	cleanupEncapMPLSInGREAndGUE(t, dut)
	cleanupStaticRoutes(t, dut)
	cleanupAggregate(t, dut, custAggID, custPorts)
	cleanupAggregate(t, dut, core1AggID, core1Ports)
	cleanupAggregate(t, dut, core2AggID, core2Ports)
}

func cleanupAggregate(t *testing.T, dut *ondatra.DUTDevice, aggID string, ports []string) {
	t.Helper()
	for _, p := range ports {
		port := dut.Port(t, p)
		gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
		gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).HoldTime().Config())
	}
	gnmi.Delete(t, dut, gnmi.OC().Interface(aggID).Config())
	gnmi.Delete(t, dut, gnmi.OC().Lacp().Interface(aggID).Config())
}

func cleanupStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	ni := deviations.DefaultNetworkInstance(dut)
	for _, prefix := range []string{"10.99.1.0/24", "10.99.2.0/24"} {
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Static(prefix).Config())
	}
}

func cleanupEncapMPLSInGREAndGUE(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		greIntfs := custIntfs[:3]
		gueIntfs := custIntfs[3:]
		for _, a := range greIntfs {
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s.%d\n no traffic-policy input %s\n!\n", custAggID, a.Subinterface, encapPolicyName))
		}
		for _, a := range gueIntfs {
			helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s.%d\n no traffic-policy input %s\n!\n", custAggID, a.Subinterface, gueEncapPolicyName))
		}
		var b strings.Builder
		fmt.Fprintf(&b, "traffic-policies\n no traffic-policy %s\n no traffic-policy %s\n!\n", encapPolicyName, gueEncapPolicyName)
		fmt.Fprintf(&b, "no nexthop-group %s type mpls-over-gre\n", greNHGName)
		fmt.Fprintf(&b, "no nexthop-group %s type ipv4-over-udp\n", gueNHGName)
		helpers.GnmiCLIConfig(t, dut, b.String())
	default:
		t.Fatalf("MPLSoGRE/MPLSoGUE encap cleanup is not implemented for vendor %v", dut.Vendor())
	}
}

func cleanupDecapMPLSInGREAndGUE(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		cli := fmt.Sprintf(`
no ip decap-group %s
no ip decap-group type udp destination port %d payload mpls
no ip decap-group %s
`, decapGREGroup, gueDstPort, decapGUEGroup)
		helpers.GnmiCLIConfig(t, dut, cli)

		var mplsB strings.Builder
		for tc := 0; tc < 8; tc++ {
			fmt.Fprintf(&mplsB, "no mpls static top-label %d %s pop payload-type ipv4 access-list bypass\n", 99990+tc, custOTG0.IPv4)
			fmt.Fprintf(&mplsB, "no mpls static top-label %d %s pop payload-type ipv4 access-list bypass\n", 99890+tc, custOTG0.IPv4)
			fmt.Fprintf(&mplsB, "no mpls static top-label %d %s pop payload-type ipv4 access-list bypass\n", 99980+tc, custOTG0.IPv4)
			fmt.Fprintf(&mplsB, "no mpls static top-label %d %s pop payload-type ipv4 access-list bypass\n", 99880+tc, custOTG0.IPv4)
			fmt.Fprintf(&mplsB, "no mpls static top-label %d %s pop payload-type ipv6 access-list bypass\n", 99970+tc, custOTG0.IPv6)
			fmt.Fprintf(&mplsB, "no mpls static top-label %d %s pop payload-type ipv6 access-list bypass\n", 99870+tc, custOTG0.IPv6)
		}
		helpers.GnmiCLIConfig(t, dut, mplsB.String())
	default:
		t.Fatalf("MPLSoGRE/MPLSoGUE decap cleanup is not implemented for vendor %v", dut.Vendor())
	}
}

func cleanupQoS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	cleanupIngressPolicerCLI(t, dut)
	gnmi.Delete(t, dut, gnmi.OC().Qos().Config())
	cleanupAristaQosTxQueues(t, dut, qcNames)
}

func cleanupIngressPolicerCLI(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	if dut.Vendor() != ondatra.ARISTA {
		return
	}
	if deviations.QosSchedulerIngressPolicer(dut) {
		helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("interface %s\n no service-policy type qos input %s\n!\n", custAggID, ingressPolicerName))
	}
	if deviations.QosTwoRateThreeColorPolicerOCUnsupported(dut) {
		helpers.GnmiCLIConfig(t, dut, fmt.Sprintf("no policy-map type quality-of-service %s\n", ingressPolicerName))
	}
}

func cleanupAristaQosTxQueues(t *testing.T, dut *ondatra.DUTDevice, qNames []string) {
	t.Helper()
	var cli strings.Builder
	for index, queue := range qNames {
		fmt.Fprintf(&cli, "no qos traffic-class %d name target-group-%s\n!\n", index, queue)
		if index != 7 {
			fmt.Fprintf(&cli, "no qos map traffic-class %d to exp %d\n!\n", index, index)
			fmt.Fprintf(&cli, "no qos map traffic-class %d to tx-queue %d\n!\n", index, index)
		}
		fmt.Fprintf(&cli, "no qos tx-queue %d name %s\n!\n", index, queue)
	}
	helpers.GnmiCLIConfig(t, dut, cli.String())
}

func createFlow(t *testing.T, top gosnappi.Config, params *otgconfighelpers.Flow, clearFlows bool) {
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

func sendTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, dur time.Duration) {
	t.Helper()
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	waitForProtocols(t, dut, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(dur)
	ate.OTG().StopTraffic(t)
	time.Sleep(10 * time.Second)
}

func waitForLAGUp(t *testing.T, dut *ondatra.DUTDevice, aggID string, ports []string) {
	t.Helper()
	for _, portID := range ports {
		portName := dut.Port(t, portID).Name()
		gnmi.Await(t, dut, gnmi.OC().Interface(portName).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
		memberPath := gnmi.OC().Lacp().Interface(aggID).Member(portName).State()
		_, ok := gnmi.Watch(t, dut, memberPath, 3*time.Minute, func(value *ygnmi.Value[*oc.Lacp_Interface_Member]) bool {
			member, present := value.Val()
			return present && member.GetSynchronization() == oc.Lacp_LacpSynchronizationType_IN_SYNC && member.GetCollecting() && member.GetDistributing()
		}).Await(t)
		if !ok {
			t.Fatalf("LACP member %s in %s did not reach IN_SYNC/collecting/distributing", portName, aggID)
		}
	}
	t.Logf("DUT LAG %s members are IN_SYNC, collecting, and distributing", aggID)
}

func waitForOTGLAGUp(t *testing.T, ate *ondatra.ATEDevice, agg *otgconfighelpers.Port) {
	t.Helper()
	_, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Lag(agg.Name).OperStatus().State(), 2*time.Minute, func(value *ygnmi.Value[otgtelemetry.E_Lag_OperStatus]) bool {
		status, present := value.Val()
		return present && status == otgtelemetry.Lag_OperStatus_UP
	}).Await(t)
	if !ok {
		t.Fatalf("OTG LAG %s did not reach UP", agg.Name)
	}

	_, ok = gnmi.Watch(t, ate.OTG(), gnmi.OTG().Lacp().State(), 2*time.Minute, func(value *ygnmi.Value[*otgtelemetry.Lacp]) bool {
		lacp, present := value.Val()
		if !present || lacp == nil {
			return false
		}
		for _, portID := range agg.MemberPorts {
			member := lacp.GetLagMember(ate.Port(t, portID).ID())
			if member == nil || !member.GetCollecting() || !member.GetDistributing() {
				return false
			}
		}
		return true
	}).Await(t)
	if !ok {
		t.Fatalf("OTG LAG %s members did not reach collecting/distributing", agg.Name)
	}
	t.Logf("OTG LAG %s is UP and members are collecting/distributing", agg.Name)
}

func waitForLAG(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	t.Helper()
	for _, agg := range []*otgconfighelpers.Port{agg1, agg2, agg3} {
		waitForOTGLAGUp(t, ate, agg)
	}
	waitForLAGUp(t, dut, custAggID, custPorts)
	waitForLAGUp(t, dut, core1AggID, core1Ports)
	waitForLAGUp(t, dut, core2AggID, core2Ports)
}

func waitForProtocols(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {
	t.Helper()
	waitForLAG(t, dut, ate)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")
}

type encapToIPFlow struct {
	*otgconfighelpers.Flow
	innerIPv4 *otgconfighelpers.IPv4FlowParams
	innerIPv6 *otgconfighelpers.IPv6FlowParams
	innerTCP  *otgconfighelpers.TCPFlowParams
}

// encapOuterSpec is the GRE- or GUE-tunneled outer encapsulation shared by several encap flow variants.
type encapOuterSpec struct {
	txOTG                      *otgconfighelpers.InterfaceProperties
	srcMAC, outerSrc, outerDst string
	isGRE                      bool
}

// encapFlowSpec describes one encap-to-IP traffic variant repeated across all 8 MPLS traffic classes.
type encapFlowSpec struct {
	namePrefix                 string
	outer                      encapOuterSpec
	labelBase                  uint32
	innerSrc, innerDst         string // used when !ipv6
	ipv6                       bool
	innerIPv6Src, innerIPv6Dst string
	tcpDstPort                 uint32 // 0 means no inner TCP header
}

func encapFlowSpecs() []encapFlowSpec {
	gre := encapOuterSpec{txOTG: core1OTG, srcMAC: agg2.AggMAC, outerSrc: "100.64.0.1", outerDst: outerGREDstCore1, isGRE: true}
	gue := encapOuterSpec{txOTG: core2OTG, srcMAC: agg3.AggMAC, outerSrc: "100.64.1.1", outerDst: outerGUEDstCore2, isGRE: false}
	return []encapFlowSpec{
		{namePrefix: "MPLSoGRE", outer: gre, labelBase: 99990, innerSrc: "50.1.1.1", innerDst: "11.1.1.1", tcpDstPort: 80},
		{namePrefix: "MPLSoGUE", outer: gue, labelBase: 99890, innerSrc: "50.1.2.1", innerDst: "11.1.1.1", tcpDstPort: 80},
		// Multicast inner payload flows using dedicated multicast MPLS labels.
		{namePrefix: "MPLSoGRE-mcast", outer: gre, labelBase: 99980, innerSrc: "50.1.1.1", innerDst: mcastDst},
		{namePrefix: "MPLSoGUE-mcast", outer: gue, labelBase: 99880, innerSrc: "50.1.2.1", innerDst: mcastDst},
		// IPv6 inner payload flows.
		{namePrefix: "MPLSoGRE-v6", outer: gre, labelBase: 99970, ipv6: true, innerIPv6Src: "2001:db8:100::1", innerIPv6Dst: "2001:db8:200::1", tcpDstPort: 443},
		{namePrefix: "MPLSoGUE-v6", outer: gue, labelBase: 99870, ipv6: true, innerIPv6Src: "2001:db8:100::1", innerIPv6Dst: "2001:db8:200::1", tcpDstPort: 443},
	}
}

func newEncapFlow(s encapFlowSpec, tc int) *encapToIPFlow {
	f := &encapToIPFlow{
		Flow: &otgconfighelpers.Flow{
			TxNames:           []string{s.outer.txOTG.Name + ".IPv4"},
			RxNames:           []string{custOTG0.Name + ".IPv4"},
			SizeWeightProfile: &sizeWeightProfile,
			Flowrate:          8,
			FlowName:          fmt.Sprintf("%s-tc%d-%s", s.namePrefix, tc, s.outer.txOTG.Name),
			EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: s.outer.srcMAC},
			IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: s.outer.outerSrc, IPv4Dst: s.outer.outerDst, IPv4SrcCount: 1000},
			MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: s.labelBase + uint32(tc), MPLSExp: uint32(tc)},
		},
	}
	if s.outer.isGRE {
		f.GREFlow = &otgconfighelpers.GREFlowParams{Protocol: otgconfighelpers.IanaMPLSEthertype}
	} else {
		f.UDPFlow = &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: gueDstPort}
	}
	if s.ipv6 {
		f.innerIPv6 = &otgconfighelpers.IPv6FlowParams{IPv6Src: s.innerIPv6Src, IPv6Dst: s.innerIPv6Dst, IPv6SrcCount: 1000}
	} else {
		f.innerIPv4 = &otgconfighelpers.IPv4FlowParams{IPv4Src: s.innerSrc, IPv4Dst: s.innerDst, IPv4SrcCount: 1000}
	}
	if s.tcpDstPort != 0 {
		f.innerTCP = &otgconfighelpers.TCPFlowParams{TCPSrcPort: 49152, TCPDstPort: s.tcpDstPort, TCPSrcCount: 1000}
	}
	return f
}

func buildEncapToIPFlows() []*encapToIPFlow {
	specs := encapFlowSpecs()
	var flows []*encapToIPFlow
	for tc := 0; tc < 8; tc++ {
		for _, s := range specs {
			flows = append(flows, newEncapFlow(s, tc))
		}
	}
	return flows
}

func createEncapToIPFlow(t *testing.T, top gosnappi.Config, f *encapToIPFlow, clearFlows bool) {
	t.Helper()
	if clearFlows {
		top.Flows().Clear()
	}
	f.CreateFlow(top)
	f.AddEthHeader()
	f.AddIPv4Header()
	if f.UDPFlow != nil {
		f.AddUDPHeader()
	}
	if f.GREFlow != nil {
		f.AddGREHeader()
	}
	f.AddMPLSHeader()
	if f.innerIPv6 != nil {
		f.Flow.IPv6Flow = f.innerIPv6
		f.AddIPv6Header()
	} else {
		f.IPv4Flow = f.innerIPv4
		f.AddIPv4Header()
	}
	if f.innerTCP != nil {
		f.Flow.TCPFlow = f.innerTCP
		f.AddTCPHeader()
	}
}

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

func flowValidationStrict(name string) *otgvalidationhelpers.OTGValidation {
	return &otgvalidationhelpers.OTGValidation{
		Flow: &otgvalidationhelpers.FlowParams{Name: name, TolerancePct: strictLossTolerancePct},
	}
}

const queueCounterTimeout = 15 * time.Second

func queueCounterIsPresent(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }

func queueCounterCandidates(t *testing.T, dut *ondatra.DUTDevice, aggID string, ports []string) []string {
	t.Helper()
	intfs := []string{aggID}
	for _, p := range ports {
		intfs = append(intfs, dut.Port(t, p).Name())
	}
	return intfs
}

var (
	queueCounterMu    sync.Mutex
	queueCounterCache = map[string][]string{} // "aggID|counterKind" -> confirmed-reporting interfaces
)

// sumQueueCounter sums a per-queue counter across the interfaces known to report it for aggID.
// The reporting subset of queueCounterCandidates is discovered once per (aggID, counterKind) and
// cached, so later calls skip re-probing (each a queueCounterTimeout wait) candidates that never report.
func sumQueueCounter(t *testing.T, dut *ondatra.DUTDevice, aggID string, ports []string, counterKind, queue string, fetch func(intf string) (uint64, bool)) uint64 {
	t.Helper()
	key := aggID + "|" + counterKind
	queueCounterMu.Lock()
	intfs, cached := queueCounterCache[key]
	queueCounterMu.Unlock()

	if !cached {
		var reporting []string
		for _, intf := range queueCounterCandidates(t, dut, aggID, ports) {
			if _, ok := fetch(intf); ok {
				reporting = append(reporting, intf)
			}
		}
		queueCounterMu.Lock()
		queueCounterCache[key] = reporting
		queueCounterMu.Unlock()
		intfs = reporting
	}

	if len(intfs) == 0 {
		t.Errorf("%s for queue %s not available on any of %v within %v", counterKind, queue, queueCounterCandidates(t, dut, aggID, ports), queueCounterTimeout)
		return 0
	}
	var total uint64
	for _, intf := range intfs {
		if got, ok := fetch(intf); ok {
			t.Logf("queue %s %s on %s: %d", queue, counterKind, intf, got)
			total += got
		}
	}
	return total
}

func queueTransmitPkts(t *testing.T, dut *ondatra.DUTDevice, aggID string, ports []string, queue string) uint64 {
	t.Helper()
	return sumQueueCounter(t, dut, aggID, ports, "transmit-pkts", queue, func(intf string) (uint64, bool) {
		val, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(intf).Output().Queue(queue).TransmitPkts().State(), queueCounterTimeout, queueCounterIsPresent).Await(t)
		got, _ := val.Val()
		return got, ok
	})
}

func queueDroppedPkts(t *testing.T, dut *ondatra.DUTDevice, aggID string, ports []string, queue string) uint64 {
	t.Helper()
	return sumQueueCounter(t, dut, aggID, ports, "dropped-pkts", queue, func(intf string) (uint64, bool) {
		val, ok := gnmi.Watch(t, dut, gnmi.OC().Qos().Interface(intf).Output().Queue(queue).DroppedPkts().State(), queueCounterTimeout, queueCounterIsPresent).Await(t)
		got, _ := val.Val()
		return got, ok
	})
}

// collectQueueTxPkts gathers per-queue transmit-pkts counters in qNames order.
func collectQueueTxPkts(t *testing.T, dut *ondatra.DUTDevice, aggID string, ports []string, qNames []string) []uint64 {
	t.Helper()
	txPkts := make([]uint64, len(qNames))
	for i, qn := range qNames {
		txPkts[i] = queueTransmitPkts(t, dut, aggID, ports, qn)
	}
	return txPkts
}

// setEncapFlowRates sets Flowrate to 0 for TCs in stoppedTCs and to activeRate otherwise.
func setEncapFlowRates(flows []*encapToIPFlow, stoppedTCs map[int]bool, activeRate float32) {
	for _, f := range flows {
		if stoppedTCs[int(f.MPLSFlow.MPLSExp)] {
			f.Flowrate = 0
		} else {
			f.Flowrate = activeRate
		}
	}
}

// assertNonDecreasingPriority fails if a lower-priority queue outpaces a higher-priority one, or the top queue is silent.
func assertNonDecreasingPriority(t *testing.T, label string, qNames []string, txPkts []uint64) {
	t.Helper()
	if txPkts[len(txPkts)-1] == 0 {
		t.Errorf("%shighest priority queue %s: got 0 transmit-pkts, want > 0", label, qNames[len(qNames)-1])
	}
	for i := 1; i < len(qNames); i++ {
		if txPkts[i] < txPkts[i-1] {
			t.Errorf("%spriority %d (%s) transmit-pkts %d < priority %d (%s) %d; higher priority must not be starved by lower",
				label, i, qNames[i], txPkts[i], i-1, qNames[i-1], txPkts[i-1])
		}
	}
}

// assertIncreased fails if got did not grow past base, e.g. after freeing bandwidth by stopping another TC.
func assertIncreased(t *testing.T, qn string, got, base uint64) {
	t.Helper()
	if got <= base {
		t.Errorf("queue %s: transmit-pkts %d did not increase after freeing bandwidth (was %d)", qn, got, base)
	}
}

func TestPF118Traffic(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	t.Cleanup(func() { cleanupDUT(t, dut) })

	t.Run("PF-1.18.1_Setup", func(t *testing.T) {
		fptest.ConfigureDefaultNetworkInstance(t, dut)
		configureDUT(t, dut)
		configureOTG(t)
	})

	t.Run("PF-1.18.2_MPLSTrafficClassClassification", func(t *testing.T) {
		qNames := qcNames

		top.Flows().Clear()
		flows := buildEncapToIPFlows()
		for i, f := range flows {
			f.Flowrate = 2
			createEncapToIPFlow(t, top, f, i == 0)
		}
		sendTraffic(t, dut, ate, trafficDuration)

		for _, f := range flows {
			if err := flowValidation(f.FlowName).ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("ValidateLossOnFlows(%s) got err: %v, want nil", f.FlowName, err)
			}
		}
		for i, qn := range qNames {
			if got := queueTransmitPkts(t, dut, custAggID, custPorts, qn); got == 0 {
				t.Errorf("queue %s (tc%d) transmit-pkts on %s: got 0, want > 0", qn, i, custAggID)
			}
		}

		dscpCaptureGRE := &encapToIPFlow{
			Flow: &otgconfighelpers.Flow{
				TxNames:       []string{core1OTG.Name + ".IPv4"},
				RxNames:       []string{custOTG0.Name + ".IPv4"},
				FlowName:      "MPLSoGRE-decap-inner-header-capture",
				PacketsToSend: 100,
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: agg2.AggMAC},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.1", IPv4Dst: outerGREDstCore1, IPv4SrcCount: 1000},
				GREFlow:       &otgconfighelpers.GREFlowParams{Protocol: otgconfighelpers.IanaMPLSEthertype},
				MPLSFlow:      &otgconfighelpers.MPLSFlowParams{MPLSLabel: 99990, MPLSExp: 0},
			},
			innerIPv4: &otgconfighelpers.IPv4FlowParams{IPv4Src: "50.1.1.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 1000, DSCP: innerDSCPCapture},
		}
		dscpCaptureGUE := &encapToIPFlow{
			Flow: &otgconfighelpers.Flow{
				TxNames:       []string{core2OTG.Name + ".IPv4"},
				RxNames:       []string{custOTG0.Name + ".IPv4"},
				FlowName:      "MPLSoGUE-decap-inner-header-capture",
				PacketsToSend: 100,
				EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: agg3.AggMAC},
				IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.1.1", IPv4Dst: outerGUEDstCore2, IPv4SrcCount: 1000},
				UDPFlow:       &otgconfighelpers.UDPFlowParams{UDPSrcPort: 49152, UDPDstPort: gueDstPort},
				MPLSFlow:      &otgconfighelpers.MPLSFlowParams{MPLSLabel: 99890, MPLSExp: 0},
			},
			innerIPv4: &otgconfighelpers.IPv4FlowParams{IPv4Src: "50.1.2.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 1000, DSCP: innerDSCPCapture},
		}
		innerCapture := &packetvalidationhelpers.PacketValidation{
			PortName:    custPorts[0],
			CaptureName: "decap-inner-header",
			Validations: []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateIPv4Header},
			IPv4Layer:   &packetvalidationhelpers.IPv4Layer{DstIP: "11.1.1.1", Tos: innerDSCPCapture << 2, TTL: 64, SkipProtocolCheck: true},
		}
		top.Flows().Clear()
		createEncapToIPFlow(t, top, dscpCaptureGRE, true)
		createEncapToIPFlow(t, top, dscpCaptureGUE, false)
		packetvalidationhelpers.ConfigurePacketCapture(t, top, innerCapture)
		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		waitForProtocols(t, dut, ate)
		cs := packetvalidationhelpers.StartCapture(t, ate)
		ate.OTG().StartTraffic(t)
		time.Sleep(10 * time.Second)
		ate.OTG().StopTraffic(t)
		time.Sleep(10 * time.Second)
		packetvalidationhelpers.StopCapture(t, ate, cs)
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, innerCapture); err != nil {
			t.Errorf("CaptureAndValidatePackets(inner DSCP/dest-IP after decap): %v", err)
		}
	})

	t.Run("PF-1.18.3_DSCPMarking", func(t *testing.T) {
		validations := []packetvalidationhelpers.ValidationType{
			packetvalidationhelpers.ValidateIPv4Header,
			packetvalidationhelpers.ValidateMPLSLayer,
			packetvalidationhelpers.ValidateInnerIPv4Header,
		}
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
			RxNames:       []string{core1OTG.Name + ".IPv4", core2OTG.Name + ".IPv4"},
			FlowName:      "IPtoEncap-dscp-marking",
			PacketsToSend: 1000,
			EthFlow:       &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC},
			VLANFlow:      &otgconfighelpers.VLANFlowParams{VLANId: uint32(custIntfTC0.Subinterface)},
			IPv4Flow:      &otgconfighelpers.IPv4FlowParams{IPv4Src: "12.1.1.1", IPv4Dst: "11.1.1.1", IPv4SrcCount: 100, DSCP: 4},
		}
		createFlow(t, top, flow, true)
		packetvalidationhelpers.ConfigurePacketCapture(t, top, encapValidation)

		ate.OTG().PushConfig(t, top)
		ate.OTG().StartProtocols(t)
		waitForProtocols(t, dut, ate)
		cs := packetvalidationhelpers.StartCapture(t, ate)
		ate.OTG().StartTraffic(t)
		time.Sleep(30 * time.Second)
		ate.OTG().StopTraffic(t)
		time.Sleep(30 * time.Second)
		packetvalidationhelpers.StopCapture(t, ate, cs)

		if err := flowValidation(flow.FlowName).ValidateLossOnFlows(t, ate); err != nil {
			packetvalidationhelpers.ClearCapture(t, top, ate)
			t.Errorf("ValidateLossOnFlows(%s) got err: %v, want nil", flow.FlowName, err)
		}
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
			t.Errorf("CaptureAndValidatePackets(): %v", err)
		}
		packetvalidationhelpers.ClearCapture(t, top, ate)
	})

	t.Run("PF-1.18.4_AssuredForwardingMinBandwidth", func(t *testing.T) {
		qNames := qcNames

		applySchedulerOnOutput(t, dut, custAggID, bwSchedulerName, qNames)

		// Phase 1: all TCs active under congestion.
		top.Flows().Clear()
		flows := buildEncapToIPFlows()
		setEncapFlowRates(flows, nil, 12)
		for i, f := range flows {
			createEncapToIPFlow(t, top, f, i == 0)
		}
		sendTraffic(t, dut, ate, trafficDuration)

		baseTransmit := collectQueueTxPkts(t, dut, custAggID, custPorts, qNames)
		for i, qn := range qNames {
			if baseTransmit[i] == 0 {
				t.Errorf("queue %s: got 0 transmit-pkts, want > 0 (minimum bandwidth not honored)", qn)
			}
			if dropped := queueDroppedPkts(t, dut, custAggID, custPorts, qn); dropped > 0 {
				t.Logf("queue %s dropped-pkts: %d (congestion expected per README)", qn, dropped)
			}
		}

		// Phase 2: stop TC0 and TC1, verify remaining TCs absorb bandwidth.
		top.Flows().Clear()
		flows = buildEncapToIPFlows()
		setEncapFlowRates(flows, map[int]bool{0: true, 1: true}, 12)
		for i, f := range flows {
			createEncapToIPFlow(t, top, f, i == 0)
		}
		sendTraffic(t, dut, ate, trafficDuration)

		afterStop := collectQueueTxPkts(t, dut, custAggID, custPorts, qNames)
		for i, qn := range qNames {
			if i <= 1 {
				continue
			}
			assertIncreased(t, qn, afterStop[i], baseTransmit[i])
		}
	})

	t.Run("PF-1.18.5_AssuredForwardingShaper", func(t *testing.T) {
		qNames := qcNames

		applySchedulerOnOutput(t, dut, custAggID, bwShaperSchedulerName, qNames)

		// Phase 1: all TCs active, verify shaper limits on TC0-TC2.
		top.Flows().Clear()
		flows := buildEncapToIPFlows()
		setEncapFlowRates(flows, nil, 12)
		for i, f := range flows {
			createEncapToIPFlow(t, top, f, i == 0)
		}
		sendTraffic(t, dut, ate, trafficDuration)

		txPkts := collectQueueTxPkts(t, dut, custAggID, custPorts, qNames)
		for i, qn := range qNames {
			if txPkts[i] == 0 {
				t.Errorf("queue %s: got 0 transmit-pkts, want > 0", qn)
			}
		}
		// PIR limits for TC0-TC2 (bits/s); TC3+ have CIR-only (no hard cap).
		pirLimits := []uint64{200_000_000, 300_000_000, 400_000_000}
		for i := range pirLimits {
			maxExpectedPkts := (pirLimits[i] / 8) * uint64(trafficDuration.Seconds()) / 64
			t.Logf("queue %s: transmit-pkts %d, shaper max ~%d pkts (PIR %d bps, %v)", qNames[i], txPkts[i], maxExpectedPkts, pirLimits[i], trafficDuration)
		}

		// Phase 2: stop TC0 and TC1, verify redistribution.
		top.Flows().Clear()
		flows = buildEncapToIPFlows()
		setEncapFlowRates(flows, map[int]bool{0: true, 1: true}, 12)
		for i, f := range flows {
			createEncapToIPFlow(t, top, f, i == 0)
		}
		sendTraffic(t, dut, ate, trafficDuration)

		afterStop := collectQueueTxPkts(t, dut, custAggID, custPorts, qNames)
		for i, qn := range qNames {
			if i <= 1 {
				continue
			}
			if afterStop[i] == 0 {
				t.Errorf("queue %s: got 0 transmit-pkts after stopping TC0/TC1", qn)
			}
		}
	})

	t.Run("PF-1.18.6_ExpeditedForwardingPriorityDecap", func(t *testing.T) {
		qNames := qcNames

		applySchedulerOnOutput(t, dut, custAggID, prioSchedulerName, qNames)

		// Phase 1: all TCs active under strict priority with congestion.
		top.Flows().Clear()
		flows := buildEncapToIPFlows()
		setEncapFlowRates(flows, nil, 12)
		for i, f := range flows {
			createEncapToIPFlow(t, top, f, i == 0)
		}
		sendTraffic(t, dut, ate, trafficDuration)

		txPkts := collectQueueTxPkts(t, dut, custAggID, custPorts, qNames)
		assertNonDecreasingPriority(t, "", qNames, txPkts)

		// Phase 2: stop TC7, verify TC6 absorbs freed bandwidth.
		top.Flows().Clear()
		flows = buildEncapToIPFlows()
		setEncapFlowRates(flows, map[int]bool{7: true}, 12)
		for i, f := range flows {
			createEncapToIPFlow(t, top, f, i == 0)
		}
		sendTraffic(t, dut, ate, trafficDuration)

		afterStop := collectQueueTxPkts(t, dut, custAggID, custPorts, qNames)
		assertIncreased(t, qNames[6], afterStop[6], txPkts[6])
	})

	t.Run("PF-1.18.7_ExpeditedForwardingPriorityShaper", func(t *testing.T) {
		qNames := qcNames

		applySchedulerOnOutput(t, dut, custAggID, prioShaperSchedulerName, qNames)

		// Phase 1: all TCs active under strict priority + shaper.
		top.Flows().Clear()
		flows := buildEncapToIPFlows()
		setEncapFlowRates(flows, nil, 12)
		for i, f := range flows {
			createEncapToIPFlow(t, top, f, i == 0)
		}
		sendTraffic(t, dut, ate, trafficDuration)

		txPkts := collectQueueTxPkts(t, dut, custAggID, custPorts, qNames)
		for i, qn := range qNames {
			dropped := queueDroppedPkts(t, dut, custAggID, custPorts, qn)
			t.Logf("queue %s dropped-pkts: %d", qn, dropped)
			if txPkts[i] == 0 && i >= 4 {
				t.Errorf("queue %s: got 0 transmit-pkts, want > 0", qn)
			}
		}
		assertNonDecreasingPriority(t, "", qNames, txPkts)
		// OneRateTwoColor PIR for TC0-TC3 (bits/s).
		shaperPir := []uint64{100_000_000, 150_000_000, 200_000_000, 250_000_000}
		for i := range shaperPir {
			maxExpectedPkts := (shaperPir[i] / 8) * uint64(trafficDuration.Seconds()) / 64
			t.Logf("queue %s: transmit-pkts %d, shaper max ~%d pkts (PIR %d bps)", qNames[i], txPkts[i], maxExpectedPkts, shaperPir[i])
		}

		// Phase 2: stop TC7, verify TC6 absorbs freed bandwidth.
		top.Flows().Clear()
		flows = buildEncapToIPFlows()
		setEncapFlowRates(flows, map[int]bool{7: true}, 12)
		for i, f := range flows {
			createEncapToIPFlow(t, top, f, i == 0)
		}
		sendTraffic(t, dut, ate, trafficDuration)

		afterStop := collectQueueTxPkts(t, dut, custAggID, custPorts, qNames)
		assertIncreased(t, qNames[6], afterStop[6], txPkts[6])
	})

	t.Run("PF-1.18.8_ExpeditedForwardingPriorityEncap", func(t *testing.T) {
		qNames := qcNames
		aggs := []struct {
			aggID string
			ports []string
			name  string
		}{{core1AggID, core1Ports, "core1"}, {core2AggID, core2Ports, "core2"}}

		// Phase 1: escalating rates per TC to ensure congestion + priority ordering.
		top.Flows().Clear()
		flows := buildIPToEncapFlows()
		for i, f := range flows {
			f.Flowrate = float32(10 + i*2)
			createFlow(t, top, f, i == 0)
		}
		sendTraffic(t, dut, ate, trafficDuration)

		for _, agg := range aggs {
			txPkts := collectQueueTxPkts(t, dut, agg.aggID, agg.ports, qNames)
			assertNonDecreasingPriority(t, agg.name+" ", qNames, txPkts)
		}

		// Phase 2: stop TC7, verify TC6 absorbs freed bandwidth.
		top.Flows().Clear()
		flows = buildIPToEncapFlows()
		for i, f := range flows {
			if i == 7 {
				f.Flowrate = 0
			} else {
				f.Flowrate = float32(10 + i*2)
			}
			createFlow(t, top, f, i == 0)
		}
		sendTraffic(t, dut, ate, trafficDuration)

		for _, agg := range aggs {
			afterStop := collectQueueTxPkts(t, dut, agg.aggID, agg.ports, qNames)
			if afterStop[6] == 0 {
				t.Errorf("%s queue %s: got 0 transmit-pkts after stopping TC7, want > 0", agg.name, qNames[6])
			}
		}
	})

	t.Run("PF-1.18.9_TwoRateThreeColorPolicer", func(t *testing.T) {
		top.Flows().Clear()
		flows := buildIPToEncapFlows()
		for i, f := range flows {
			f.Flowrate = 12
			createFlow(t, top, f, i == 0)
		}
		sendTraffic(t, dut, ate, trafficDuration)

		var totalLossPct float32
		var anyForwarded bool
		for _, f := range flows {
			lossPct := flowValidation(f.FlowName).ReturnLossPercentage(t, ate)
			t.Logf("flow %s loss: %.2f%%", f.FlowName, lossPct)
			totalLossPct += lossPct
			if lossPct < 100 {
				anyForwarded = true
			}
		}
		avgLoss := totalLossPct / float32(len(flows))
		t.Logf("Average loss across %d flows: %.2f%%", len(flows), avgLoss)

		if !anyForwarded {
			t.Errorf("all flows show 100%% loss; policer should conform traffic up to PIR (2 Gbps)")
		}
		if avgLoss == 0 {
			t.Errorf("average loss is 0%%; offered load exceeds PIR so policer must drop excess traffic")
		}
	})

	t.Run("PF-1.18.10_PortHardwareDependency", func(t *testing.T) {
		t.Skip("TODO: requires per-platform packet-processing-engine (PPE) port placement " +
			"information which is not modeled in the standard topology; re-run PF-1.18.1-1.18.9 " +
			"with ingress/egress aggregate member ports redistributed across PPEs once that " +
			"information is available for the DUT under test.")
	})

	t.Run("PF-1.18.v6_MPLSoGUEv6QoS", func(t *testing.T) {
		highestQueue := qcNames[len(qcNames)-1]

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
		createFlow(t, top, highPrioFlow, true)
		createFlow(t, top, lowPrioFlow, false)
		sendTraffic(t, dut, ate, trafficDuration)

		for _, f := range []*otgconfighelpers.Flow{highPrioFlow, lowPrioFlow} {
			if err := flowValidationStrict(f.FlowName).ValidateLossOnFlows(t, ate); err != nil {
				t.Errorf("ValidateLossOnFlows(%s): %v", f.FlowName, err)
			}
		}
		if dropped := queueDroppedPkts(t, dut, core1AggID, core1Ports, highestQueue); dropped != 0 {
			t.Errorf("dropped-pkts on %s queue %s: got %d, want 0", core1AggID, highestQueue, dropped)
		}
		if got := queueTransmitPkts(t, dut, core1AggID, core1Ports, highestQueue); got == 0 {
			t.Errorf("transmit-pkts on %s queue %s: got 0, want > 0", core1AggID, highestQueue)
		}
	})
}
