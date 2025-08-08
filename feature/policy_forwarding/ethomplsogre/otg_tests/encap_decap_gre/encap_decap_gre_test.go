package encap_decap_gre_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	otgvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_validation_helpers"
	packetvalidationhelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/packetvalidationhelpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

// TestMain calls main function.
func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	defaultMTU          = 9216
	plenIPv4            = 30
	plenIPv6            = 126
	tunnelCount         = 16
	tunnelDestinationIP = "203.0.113.10"
	staticTunnelDst     = "203.0.113.0/24"
	startTunnelSrcIP    = "192.168.80.%d"
	trafficPolicyName   = "GRE-Policy"
	nexthopGroupName    = "nexthop-gre"
	nexthopType         = "gre"
	pseudowireName      = "PSW"
	localLabel          = 100
	remoteLabel         = 300
	dutAS               = 65501
	ateAS               = 65502
	bgpPeerGroupName1   = "BGP-Peer1"
	bgpPeerGroupName2   = "BGP-Peer2"
	greProtocol         = 47
	decapDesIpv4IP      = "100.64.0.0/22"
	decapGrpName        = "Decap1"
	loadInterval        = 30

	ieee8023adLag = oc.IETFInterfaces_InterfaceType_ieee8023adLag
)

var (
	// ingressAggID string
	aggID        string
	tunnelSrcIPs = []string{}
	custPort     = []string{"port1"}
	core1Ports   = []string{"port2", "port3"}
	core2Ports   = []string{"port4", "port5"}

	custIntf = attrs.Attributes{
		Desc:         "Customer_connect",
		MTU:          defaultMTU,
		IPv4:         "192.168.10.2",
		IPv4Len:      plenIPv4,
		IPv6:         "2001:db8::192:168:10:2",
		IPv6Len:      plenIPv6,
		Subinterface: 10,
	}

	coreIntf1 = attrs.Attributes{
		Desc:    "Core_Interface-1",
		MTU:     defaultMTU,
		IPv4:    "192.168.20.2",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:168:20:2",
		IPv6Len: plenIPv6,
	}

	coreIntf2 = attrs.Attributes{
		Desc:    "Core_Interface-2",
		MTU:     defaultMTU,
		IPv4:    "192.168.30.2",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:168:30:2",
		IPv6Len: plenIPv6,
	}

	agg2 = &otgconfighelpers.Port{
		Name:        "Port-Channel2",
		AggMAC:      "02:00:01:01:01:02",
		MemberPorts: []string{"port2", "port3"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{otgIntf2},
		LagID:       2,
		IsLag:       true,
	}

	agg3 = &otgconfighelpers.Port{
		Name:        "Port-Channel3",
		AggMAC:      "02:00:01:01:01:03",
		MemberPorts: []string{"port4", "port5"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{otgIntf3},
		LagID:       3,
		IsLag:       true,
	}

	otgIntf1 = &otgconfighelpers.InterfaceProperties{
		Name: "otgPort1",
		MAC:  "02:00:01:01:01:04",
		Vlan: 10,
	}

	otgIntf2 = &otgconfighelpers.InterfaceProperties{
		Name:        "ateLag2",
		IPv4:        "192.168.20.1",
		IPv4Gateway: "192.168.20.2",
		IPv4Len:     plenIPv4,
		IPv6:        "2001:db8::192:168:20:1",
		IPv6Gateway: "2001:db8::192:168:20:2",
		IPv6Len:     plenIPv6,
		MAC:         "02:00:01:01:01:05",
	}

	otgIntf3 = &otgconfighelpers.InterfaceProperties{
		Name:        "ateLag3",
		IPv4:        "192.168.30.1",
		IPv4Gateway: "192.168.30.2",
		IPv4Len:     plenIPv4,
		IPv6:        "2001:db8::192:168:30:1",
		IPv6Gateway: "2001:db8::192:168:30:2",
		IPv6Len:     plenIPv6,
		MAC:         "02:00:01:01:01:06",
	}

	sizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 64},
		{Size: 128},
		{Size: 256},
		{Size: 512},
		{Size: 1024},
	}

	jumboWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 9000, Weight: 2},
	}

	FlowIPv4Validation = &otgvalidationhelpers.OTGValidation{
		Flow: &otgvalidationhelpers.FlowParams{TolerancePct: 0.5},
	}

	Validations = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateMPLSLayer,
	}

	OuterGREIPLayerIPv4 = &packetvalidationhelpers.IPv4Layer{
		Protocol: greProtocol,
		DstIP:    tunnelDestinationIP,
		TTL:      64,
	}

	MPLSLayer = &packetvalidationhelpers.MPLSLayer{
		Label: uint32(remoteLabel),
		Tc:    1,
	}

	controlWordMPLS = &packetvalidationhelpers.MPLSLayer{
		Label:               uint32(remoteLabel),
		Tc:                  1,
		ControlWordHeader:   true,
		ControlWordSequence: 0,
	}

	encapValidation = &packetvalidationhelpers.PacketValidation{
		PortName:    agg2.MemberPorts[0],
		Validations: Validations,
		IPv4Layer:   OuterGREIPLayerIPv4,
		MPLSLayer:   MPLSLayer,
	}

	encapAgg3Validation = &packetvalidationhelpers.PacketValidation{
		PortName:    agg3.MemberPorts[0],
		Validations: Validations,
		IPv4Layer:   OuterGREIPLayerIPv4,
		MPLSLayer:   MPLSLayer,
	}

	validationsIPv4 = []packetvalidationhelpers.ValidationType{
		packetvalidationhelpers.ValidateIPv4Header,
		packetvalidationhelpers.ValidateTCPHeader,
	}

	decapValidationIPv4 = &packetvalidationhelpers.PacketValidation{
		PortName:    "port1",
		CaptureName: "ipv4_decap",
		Validations: validationsIPv4,
		IPv4Layer:   &packetvalidationhelpers.IPv4Layer{DstIP: "21.1.1.1", Tos: 10, TTL: 64, Protocol: packetvalidationhelpers.TCP},
		TCPLayer:    &packetvalidationhelpers.TCPLayer{SrcPort: 49152, DstPort: 80},
	}

	decapFlowInnerIPv4 = &otgconfighelpers.Flow{
		IPv4Flow: &otgconfighelpers.IPv4FlowParams{IPv4Src: "22.1.1.1", IPv4Dst: "21.1.1.1", IPv4SrcCount: 1000},
		TCPFlow:  &otgconfighelpers.TCPFlowParams{TCPSrcPort: 49152, TCPDstPort: 80, TCPSrcCount: 10000},
	}
)

type BGPNeighbor struct {
	neighborip string
	deviceName string
	peerGroup  string
}

func ConfigureDut(t *testing.T, dut *ondatra.DUTDevice, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	p1 := dut.Port(t, "port1")
	intf := &oc.Interface{Name: ygot.String(p1.Name())}
	configDUTInterface(intf, []*attrs.Attributes{&custIntf}, dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), intf)

	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, core1Ports, []*attrs.Attributes{&coreIntf1}, aggID)

	aggID = netutil.NextAggregateInterface(t, dut)
	configureInterfaces(t, dut, core2Ports, []*attrs.Attributes{&coreIntf2}, aggID)

	// Configure 16 tunnels having single destination address
	for index := range tunnelCount {
		tunnelSrcIPs = append(tunnelSrcIPs, fmt.Sprintf(startTunnelSrcIP, index+10))
	}
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	cfgplugins.NextHopGroupConfigForMultipleIP(t, dut, ni, nexthopGroupName, nexthopType, tunnelSrcIPs, tunnelDestinationIP, 0, 0)
	configurePolicyForwarding(t, dut, pf)

	//Configure MPLS label ranges and qos configs
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)

	// Apply traffic policy and service policy on interface
	interfacePolicyParams := cfgplugins.OcPolicyForwardingParams{
		InterfaceID:       dut.Port(t, "port1").Name(),
		AppliedPolicyName: trafficPolicyName,
	}
	cfgplugins.InterfacePolicyForwardingApply(t, dut, dut.Port(t, "port1").Name(), trafficPolicyName, ni, interfacePolicyParams)
	cfgplugins.InterfaceQosClassificationConfigApply(t, dut, dut.Port(t, "port1").Name())

	configureStaticRoute(t, dut)

	// Configure Static Route: MPLS label binding
	sfBatch := &gnmi.SetBatch{}
	cfgplugins.MPLSStaticLSP(t, sfBatch, dut, "lsp1", remoteLabel, otgIntf2.IPv4, "", "ipv4")
	cfgplugins.MPLSStaticLSP(t, sfBatch, dut, "lsp2", remoteLabel, otgIntf3.IPv4, "", "ipv4")

	cfgplugins.MplsStaticPseudowire(t, sfBatch, dut, pseudowireName, nexthopGroupName, fmt.Sprintf("%d", localLabel), fmt.Sprintf("%d", remoteLabel), dut.Port(t, custPort[0]).Name(), custIntf.Subinterface)

	t.Log("Configuring BGP")
	bgpProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := bgpProtocol.GetOrCreateBgp()
	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAS)

	nbrs := []*BGPNeighbor{
		{neighborip: otgIntf2.IPv4, deviceName: "", peerGroup: bgpPeerGroupName1},
		{neighborip: otgIntf3.IPv4, deviceName: "", peerGroup: bgpPeerGroupName1},
	}
	for _, coreInterface := range nbrs {
		pg := bgp.GetOrCreatePeerGroup(coreInterface.peerGroup)
		pg.PeerAs = ygot.Uint32(ateAS)
		ipv4Nbr := bgp.GetOrCreateNeighbor(coreInterface.neighborip)
		ipv4Nbr.PeerGroup = ygot.String(coreInterface.peerGroup)
		ipv4Nbr.PeerAs = ygot.Uint32(ateAS)
		ipv4Nbr.Enabled = ygot.Bool(true)
		ipv4Nbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	}

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), bgpProtocol)
}

func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice) {
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticTunnelDst,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(otgIntf2.IPv4),
			"1": oc.UnionString(otgIntf3.IPv4),
		},
	}

	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)
}

func configurePolicyForwarding(t *testing.T, dut *ondatra.DUTDevice, pf *oc.NetworkInstance_PolicyForwarding) {
	if deviations.PolicyForwardingToNextHopUnsupported(dut) || deviations.PolicyForwardingGreEncapsulationOcUnsupported(dut) {
		cfgplugins.CreatePolicyForwardingNexthopConfig(t, dut, trafficPolicyName, "rule1", "ipv4", nexthopGroupName)
	} else {
		cfgplugins.ConfigurePolicyForwardingNextHopFromOC(t, dut, pf, trafficPolicyName, 1, []string{tunnelDestinationIP}, tunnelSrcIPs, nexthopGroupName)
	}
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

func configureIngressVlan(t *testing.T, dut *ondatra.DUTDevice, i *oc.Interface, intfName string, subinterfaces uint32, mode string) {
	// Configuring port/attachment mode
	s := i.GetOrCreateSubinterface(subinterfaces)
	switch mode {
	case "port":
		// Accepts packets from all VLANs
		i.GetOrCreateEthernet().GetOrCreateSwitchedVlan().SetInterfaceMode(oc.Vlan_VlanModeType_TRUNK)
		gnmi.Update(t, dut, gnmi.OC().Interface(intfName).Config(), i)

		if deviations.VlanClientEncapsulationOcUnsupported(dut) {
			cli := fmt.Sprintf(`
				interface %v.%v
					encapsulation vlan
					client unmatched`, intfName, subinterfaces)
			helpers.GnmiCLIConfig(t, dut, cli)
		} else {
			// OC is not available
		}

	case "attachment":
		// Accepts packets only for the specified VLAN
		if deviations.VlanClientEncapsulationOcUnsupported(dut) {
			cli := fmt.Sprintf(`
				interface %v.%v
					no encapsulation vlan
					`, intfName, subinterfaces)
			helpers.GnmiCLIConfig(t, dut, cli)
		} else {
			// OC is not available
		}
		s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(uint16(subinterfaces))
		gnmi.Update(t, dut, gnmi.OC().Interface(intfName).Config(), i)
	case "remove":
		gnmi.Delete(t, dut, gnmi.OC().Interface(intfName).Subinterface(subinterfaces).Vlan().Match().SingleTagged().Config())
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
		s6.GetOrCreateAddress(a.IPv6).SetPrefixLength(a.IPv6Len)
	}
}

func GetDefaultOcPolicyForwardingParams() cfgplugins.OcPolicyForwardingParams {
	return cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: "DEFAULT",
	}
}

func ConfigureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	otgConfig := gosnappi.NewConfig()

	port1 := otgConfig.Ports().Add().SetName("port1")

	iDut1Dev := otgConfig.Devices().Add().SetName(otgIntf1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(otgIntf1.Name + ".Eth").SetMac(otgIntf1.MAC)
	iDut1Eth.Vlans().Add().SetName(otgIntf1.Name + ".vlan").SetId(otgIntf1.Vlan)
	iDut1Eth.Connection().SetPortName(port1.Name())

	// Create a slice of aggPortData for easier iteration
	aggs := []*otgconfighelpers.Port{agg2, agg3}

	// Configure OTG Interfaces
	for _, agg := range aggs {
		otgconfighelpers.ConfigureNetworkInterface(t, otgConfig, ate, agg)
	}

	nbrs := []*BGPNeighbor{
		{neighborip: coreIntf1.IPv4, deviceName: "ateLag2.Dev", peerGroup: ""},
		{neighborip: coreIntf2.IPv4, deviceName: "ateLag3.Dev", peerGroup: ""},
	}
	// Configure BGP on lag2 and lag3
	for _, device := range otgConfig.Devices().Items() {
		for _, nbr := range nbrs {
			if device.Name() == nbr.deviceName {
				bgpD := device.Bgp().SetRouterId(nbr.neighborip)

				iDut1Ipv4 := device.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
				bgp4Nbr := bgpD.Ipv4Interfaces().Add().SetIpv4Name(iDut1Ipv4.Name()).Peers().Add().SetName(nbr.deviceName + ".BGP.peer")
				bgp4Nbr.SetPeerAddress(nbr.neighborip).SetAsNumber(uint32(ateAS)).SetAsType(gosnappi.BgpV4PeerAsType.EBGP)
			}
		}

	}

	return otgConfig
}

func createflow(top gosnappi.Config, params *otgconfighelpers.Flow, clearFlows bool, paramsInner *otgconfighelpers.Flow) {
	if clearFlows {
		top.Flows().Clear()
	}

	params.CreateFlow(top)

	params.AddEthHeader()

	if params.VLANFlow != nil {
		params.AddVLANHeader()
	}

	if params.IPv4Flow != nil {
		params.AddIPv4Header()
	}
	if paramsInner != nil {
		if paramsInner.IPv4Flow != nil {
			*params.IPv4Flow = *paramsInner.IPv4Flow
			params.AddIPv4Header()
		}
		if paramsInner.TCPFlow != nil {
			params.TCPFlow = paramsInner.TCPFlow
			params.AddTCPHeader()
		}
	}

}

func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice, otgConfig gosnappi.Config, OTG *otg.OTG) {
	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)
	time.Sleep(60 * time.Second)
	cs := packetvalidationhelpers.StartCapture(t, ate)
	ate.OTG().StartTraffic(t)
	time.Sleep(60 * time.Second)
	ate.OTG().StopTraffic(t)
	time.Sleep(60 * time.Second)
	packetvalidationhelpers.StopCapture(t, ate, cs)
}

func verifyECMPLagBalance(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, flowName string) {
	t.Log("Validating traffic equally distributed equally across 2 egress ports")
	var totalEgressPkts uint64
	tolerance := 0.10

	for _, egressPort := range []string{"port2", "port3", "port4", "port5"} {
		port := dut.Port(t, egressPort)
		egressPktPath := gnmi.OC().Interface(port.Name()).Counters().OutUnicastPkts().State()
		egressPkt, present := gnmi.Lookup(t, dut, egressPktPath).Val()
		if !present {
			t.Errorf("Get IsPresent status for path %q: got false, want true", egressPktPath)
		}
		totalEgressPkts += egressPkt
	}

	txPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).Counters().OutPkts().State())
	if txPkts == 0 {
		t.Fatal("ATE did not send any packets")
	}

	dutlossPct := (float64(txPkts) - float64(totalEgressPkts)) * 100 / float64(txPkts)
	if dutlossPct < tolerance*100 {
		t.Log("The packet count of traffic sent from ATE port1 equal to the sum of all packets on DUT egress ports")
	} else {
		t.Errorf("The packet count of traffic sent from ATE port1 is not equal to the sum of all packets on DUT egress ports")
	}

	rxPortNames := []string{"port2", "port3", "port4", "port5"}
	var totalRxPkts, lag1RxPkts, lag2RxPkts uint64
	portRxPkts := make(map[string]uint64)

	for _, portName := range rxPortNames {
		rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, portName).ID()).Counters().InFrames().State())
		portRxPkts[portName] = rxPkts
		totalRxPkts += rxPkts
	}

	if totalRxPkts == 0 {
		t.Skip("Skipping load balancing checks as no packets were received.")
	}

	lag1RxPkts = portRxPkts["port2"] + portRxPkts["port3"]
	lag2RxPkts = portRxPkts["port4"] + portRxPkts["port5"]
	t.Logf("LAG1 received %d packets, LAG2 received %d packets", lag1RxPkts, lag2RxPkts)
	ecmpDiff := float64(lag1RxPkts) - float64(lag2RxPkts)
	if ecmpDiff < 0 {
		ecmpDiff = -ecmpDiff
	}
	ecmpError := ecmpDiff / float64(totalRxPkts)
	if ecmpError > tolerance {
		t.Errorf("ECMP hashing between LAG1 and LAG2 is unbalanced. Difference: %f%%, want < %f%%", ecmpError*100, tolerance*100)
	} else {
		t.Logf("ECMP hashing between LAGs is balanced. Difference: %f%%", ecmpError*100)
	}
}

func verifyLoadBalanceAcrossGre(t *testing.T, singlePath bool) {
	t.Log("Validating traffic equally load-balanced across GRE destinations")
	srcIPs := make(map[string]int)
	packetSource := packetvalidationhelpers.GetPacketSource()
	for packet := range packetSource.Packets() {
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			ipv4, _ := ipLayer.(*layers.IPv4)
			srcIPs[ipv4.SrcIP.String()]++
		}
	}
	uniqueCount := len(srcIPs)
	t.Logf("Found %d unique GRE source IPs in the capture.", uniqueCount)

	if singlePath {
		if uniqueCount > 1 || uniqueCount == 0 {
			t.Errorf("FAIL: No GRE source IPs found, expected at least 1")
			return
		}
		t.Logf("PASS: Single-path mode: Found %d GRE source IP(s), as expected.", uniqueCount)
		return
	}

	if uniqueCount <= 1 {
		t.Errorf("FAIL: Traffic was not load-balanced. Got %d unique GRE source IP(s), expected > 1.", uniqueCount)
		return
	}

	t.Logf("PASS: Traffic was load-balanced across %d GRE sources.", uniqueCount)
}

func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, otgConfig gosnappi.Config, otg *otg.OTG, flowName string) {
	otgutils.LogFlowMetrics(t, otg, otgConfig)

	FlowIPv4Validation.Flow.Name = flowName
	if err := FlowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("Validation on flows failed (): %q", err)
	}
}

func validateEncapPacket(t *testing.T, ate *ondatra.ATEDevice, validationPacket *packetvalidationhelpers.PacketValidation) error {
	err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, validationPacket)
	return err
}

func TestEncapDecapGre(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Get default parameters for OC Policy Forwarding
	ocPFParams := GetDefaultOcPolicyForwardingParams()

	// Pass ocPFParams to ConfigureDut
	ConfigureDut(t, dut, ocPFParams)

	// Configure on OTG
	otgConfig := ConfigureOTG(t, ate)
	packetvalidationhelpers.ConfigurePacketCapture(t, otgConfig, encapValidation)
	packetvalidationhelpers.ConfigurePacketCapture(t, otgConfig, encapAgg3Validation)

	type customerIntfModeCOnfig struct {
		name string
		mode string
	}
	modeConfig := []customerIntfModeCOnfig{
		{
			name: "Validate traffic in port mode",
			mode: "port",
		},
		{
			name: "Validate traffic in attachment mode",
			mode: "attachment",
		},
	}

	type testCase struct {
		name        string
		description string
		flow        otgconfighelpers.Flow
		testFunc    func(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow)
	}
	testCases := []testCase{
		{
			name:        "PF1.23.1 : EthoCWoMPLSoGRE encapsulated for unencrytped IPv4 with entropy",
			description: "Verify PF EthoCWoMPLSoGRE encapsulate action for unencrytped IPv4, IPv6 traffic with entropy on ethernet headers",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Ports().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[0].Name(), otgConfig.Lags().Items()[1].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGREv4Entropy",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, SrcMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: custIntf.Subinterface},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testEthoCWoMPLSoGREEncapIPv4WithEntropy,
		},
		{
			name:        "PF1.23.2 : EthoCWoMPLSoGRE encapsulated for unencrytped IPv4 without entropy",
			description: "Verify no hashing of EthoCWoMPLSoGRE encapsulation for unencrytped IPv4 traffic without entropy",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Ports().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[0].Name(), otgConfig.Lags().Items()[1].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGREv4WOEntropy ",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: custIntf.Subinterface},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testEthoCWoMPLSoGREEncapIPv4WithoutEntrpy,
		},
		{
			name:        "PF1.23.3 : EthoCWoMPLSoGRE encapsulate action for MACSec",
			description: "Verify PF EthoCWoMPLSoGRE encapsulate action for MACSec encrytped IPv4, IPv6 traffic",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Ports().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[0].Name(), otgConfig.Lags().Items()[1].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGREv4MACSec",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, SrcMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: custIntf.Subinterface},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testEthoCWoMPLSoGREEncapMACSec,
		},
		{
			name:        "PF1.23.4 : EthoCWoMPLSoGRE encapsulate action with Jumbo MTU",
			description: "Verify PF EthoCWoMPLSoGRE encapsulate action with Jumbo MTU",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Ports().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[0].Name(), otgConfig.Lags().Items()[1].Name()},
				IsTxRxPort:        true,
				FlowName:          "EthoMPLSoGREJumboMTU",
				SizeWeightProfile: &jumboWeightProfile,
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, SrcMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: custIntf.Subinterface},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testEthoCWoMPLSoGREEncapJumboMTU,
		},
		{
			name:        "PF1.23.5 : Verify Control word for unencrypted traffic flow",
			description: "Verify Control word for unencrypted traffic flow",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Ports().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[0].Name(), otgConfig.Lags().Items()[1].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGRECWUnEncrypted",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, SrcMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: custIntf.Subinterface},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testControlWordUnencrypted,
		},
		{
			name:        "PF1.23.6 : Verify Control word for encrypted traffic flow",
			description: "Verify Control word for encrypted traffic flow",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Ports().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[0].Name(), otgConfig.Lags().Items()[1].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGRECWEncrypted",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: custIntf.Subinterface},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testControlWordEncrypted,
		},
		{
			name:        "PF1.23.7 : Verify DSCP of EthoCWoMPLSoGRE encapsulated packets",
			description: "Verify DSCP of EthoCWoMPLSoGRE encapsulated packets",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Ports().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[0].Name(), otgConfig.Lags().Items()[1].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGREDSCP",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, SrcMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: custIntf.Subinterface},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP, DSCP: 20},
			},
			testFunc: testDSCPEthoCWoMPLSoGRE,
		},
		{
			name:        "PF1.23.8 : Verify PF EthoCWoMPLSoGRE encapsulate after single GRE tunnel destination shutdown",
			description: "Verify PF EthoCWoMPLSoGRE encapsulate after single GRE tunnel destination shutdown",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Ports().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[0].Name(), otgConfig.Lags().Items()[1].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGREv4",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, SrcMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: custIntf.Subinterface},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testEthoCWoMPLSoGRE,
		},
		{
			name:        "PF1.23.9 : Verify PF EthoCWoMPLSoGRE decapsulate action",
			description: "Verify PF EthoCWoMPLSoGRE decapsulate action",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Ports().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[0].Name(), otgConfig.Lags().Items()[1].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGREv4Decap",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, SrcMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: custIntf.Subinterface},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: strings.Split(decapDesIpv4IP, "/")[0]},
				MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: remoteLabel},
				GREFlow:           &otgconfighelpers.GREFlowParams{Protocol: otgconfighelpers.IanaMPLSEthertype},
			},
			testFunc: testEthoCWoMPLSoGREDecap,
		},
		{
			name:        "PF1.23.10 : Verify VLAN tag after PF EthoCWoMPLSoGRE decapsulate action",
			description: "Verify VLAN tag after PF EthoCWoMPLSoGRE decapsulate action",
			flow: otgconfighelpers.Flow{
				FlowName: "EthoMPLSoGREv4Decap",
			},
			testFunc: testEthoCWoMPLSoGREDecapVLAN,
		},
	}

	// Run the test cases.
	for _, tc := range testCases {
		configureIngressVlan(t, dut, &oc.Interface{Name: ygot.String(dut.Port(t, custPort[0]).Name())}, dut.Port(t, custPort[0]).Name(), custIntf.Subinterface, "remove")
		for _, cfg := range modeConfig {
			switch cfg.mode {
			case "port":
				t.Log(cfg.name)
				configureIngressVlan(t, dut, &oc.Interface{Name: ygot.String(dut.Port(t, custPort[0]).Name())}, dut.Port(t, custPort[0]).Name(), custIntf.Subinterface, "port")
			case "attachment":
				t.Log(cfg.name)
				configureIngressVlan(t, dut, &oc.Interface{Name: ygot.String(dut.Port(t, custPort[0]).Name())}, dut.Port(t, custPort[0]).Name(), custIntf.Subinterface, "attachment")
			}
			t.Run(tc.name, func(t *testing.T) {
				t.Logf("Description: %s - %s", tc.name, cfg.mode)
				tc.testFunc(t, dut, ate, ate.OTG(), otgConfig, tc.flow)
			})
		}
	}
}

// EthoCWoMPLSoGRE encapsulate for IPv4
func testEthoCWoMPLSoGREEncapIPv4WithEntropy(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	// Generate 1000 different traffic flows on ATE Port 1
	createflow(otgConfig, &flow, true, nil)
	sendTrafficCapture(t, ate, otgConfig, otg)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	verifyECMPLagBalance(t, dut, ate, flow.FlowName)

	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
		t.Errorf("Capture And ValidatePackets Failed (): %q", err)
	}

	verifyLoadBalanceAcrossGre(t, false)
}

func testEthoCWoMPLSoGREEncapIPv4WithoutEntrpy(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	// Flows should have NOT have entropy on any headers.
	createflow(otgConfig, &flow, true, nil)
	sendTrafficCapture(t, ate, otgConfig, otg)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	err := validateEncapPacket(t, ate, encapValidation)
	if err != nil {
		// If error validating the whether encap packet received on other lag port
		if err = validateEncapPacket(t, ate, encapAgg3Validation); err != nil {
			t.Errorf("Capture And ValidatePackets Failed (): %q", err)
		}
	}

	verifyLoadBalanceAcrossGre(t, true)
}

func testEthoCWoMPLSoGREEncapMACSec(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	// Flows should have NOT have entropy on any headers.
	createflow(otgConfig, &flow, true, nil)

	if !deviations.MacsecOCUnsupported(dut) {
		// TODO: Issue - 436181703
		flowObj := otgConfig.Flows().Items()[0]
		flowObj.Packet().Add().Macsec()
		sendTrafficCapture(t, ate, otgConfig, otg)
		verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
			t.Errorf("Capture And ValidatePackets Failed (): %q", err)
		}

		verifyLoadBalanceAcrossGre(t, true)
	}

}

func testEthoCWoMPLSoGREEncapJumboMTU(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	// Generate 1000 different traffic flows on ATE Port 1
	createflow(otgConfig, &flow, true, nil)
	sendTrafficCapture(t, ate, otgConfig, otg)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
		t.Errorf("Capture And ValidatePackets Failed (): %q", err)
	}

	verifyLoadBalanceAcrossGre(t, false)
}

func testControlWordUnencrypted(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	createflow(otgConfig, &flow, true, nil)
	sendTrafficCapture(t, ate, otgConfig, otg)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	verifyECMPLagBalance(t, dut, ate, flow.FlowName)

	encapValidation.MPLSLayer = controlWordMPLS
	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
		t.Errorf("Capture And ValidatePackets Failed (): %q", err)
	}

	verifyLoadBalanceAcrossGre(t, false)
}

func testControlWordEncrypted(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	createflow(otgConfig, &flow, true, nil)
	sendTrafficCapture(t, ate, otgConfig, otg)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	encapValidation.MPLSLayer = controlWordMPLS
	err := validateEncapPacket(t, ate, encapValidation)
	if err != nil {
		// If error validating the whether encap packet received on other lag port
		encapAgg3Validation.MPLSLayer = controlWordMPLS
		if err = validateEncapPacket(t, ate, encapAgg3Validation); err != nil {
			t.Errorf("Capture And ValidatePackets Failed (): %q", err)
		}
	}

	verifyLoadBalanceAcrossGre(t, true)
}

func testDSCPEthoCWoMPLSoGRE(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	// TODO: DSCP=96 in readme out of range of DSCP values. Issue - 436181703
	// createflow(otgConfig, &flow, true, nil)
	// sendTrafficCapture(t, ate, otgConfig, otg)

	// verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	// verifyECMPLagBalance(t, dut, ate, otgConfig, flow.FlowName)

	// OuterGREIPLayerIPv4.Tos = 20
	// if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
	// 	t.Errorf("Capture And ValidatePackets Failed (): %q", err)
	// }

	// verifyLoadBalanceAcrossGre(t, false)

}

func testEthoCWoMPLSoGRE(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	// Shutdown or remove a single GRE tunnel destination on the DUT
	if deviations.NextHopGroupOCUnsupported(dut) {
		cli := fmt.Sprintf(`
				nexthop-group %s type %s
				no entry %d tunnel-destination %v`,
			nexthopGroupName, nexthopType, 0, tunnelDestinationIP)
		helpers.GnmiCLIConfig(t, dut, cli)
	}

	createflow(otgConfig, &flow, true, nil)
	sendTrafficCapture(t, ate, otgConfig, otg)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	verifyECMPLagBalance(t, dut, ate, flow.FlowName)

	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
		t.Errorf("Capture And ValidatePackets Failed (): %q", err)
	}

	verifyLoadBalanceAcrossGre(t, false)
}

func testEthoCWoMPLSoGREDecap(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	sfBatch := &gnmi.SetBatch{}
	cfgplugins.PolicyForwardingGreDecapsulation(t, sfBatch, dut, strings.Split(decapDesIpv4IP, "/")[0], "trafficPolicyName", "port1", decapGrpName)
	createflow(otgConfig, &flow, true, decapFlowInnerIPv4)

	// TODO: Test validation to be added once issue-436181703 is resolved
	// sendTrafficCapture(t, ate, otgConfig, otg)
	// verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

}

func testEthoCWoMPLSoGREDecapVLAN(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	// TODO: Test validation to be added once issue-436181703 is resolved
	// createflow(otgConfig, &flow, true, nil)
	// sendTrafficCapture(t, ate, otgConfig, otg)
	// verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)
}
