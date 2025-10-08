package encapdecapgre_test

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/gopacket"
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
	nexthopGroupName    = "nexthop-gre"
	nexthopType         = "gre"
	pseudowireName      = "PSW"
	localLabel          = 100
	remoteLabel         = 100
	dutAS               = 65501
	ateAS               = 65502
	bgpPeerGroupName1   = "BGP-Peer1"
	bgpPeerGroupName2   = "BGP-Peer2"
	greProtocol         = 47
	decapDesIpv4IP      = "192.168.80.0/24"
	decapGrpName        = "Decap1"
	dscp                = 96 >> 2
)

var (
	sfBatch      *gnmi.SetBatch
	ni           *oc.NetworkInstance
	custAggID    string
	tunnelSrcIPs = []string{}
	custPort     = "port1"
	packetSource = []*gopacket.PacketSource{}

	activity = oc.Lacp_LacpActivityType_ACTIVE
	period   = oc.Lacp_LacpPeriodType_FAST

	lacpParams = &cfgplugins.LACPParams{
		Activity: &activity,
		Period:   &period,
	}

	custLagData = []*cfgplugins.DUTAggData{
		{
			Attributes:      custIntf,
			OndatraPortsIdx: []int{0},
			LacpParams:      lacpParams,
			AggType:         oc.IfAggregate_AggregationType_STATIC,
			SubInterfaces: []*cfgplugins.DUTSubInterfaceData{
				{
					VlanID:        10,
					VlanEnable:    false,
					IPv4Address:   net.ParseIP("192.168.10.2"),
					IPv6Address:   net.ParseIP("2001:db8::192:168:10:2"),
					IPv4PrefixLen: plenIPv4,
					IPv6PrefixLen: plenIPv6,
				},
			},
		},
	}

	coreLagData = []*cfgplugins.DUTAggData{
		{
			Attributes:      coreIntf1,
			OndatraPortsIdx: []int{1, 2},
			LacpParams:      lacpParams,
			AggType:         oc.IfAggregate_AggregationType_LACP,
		},
		{
			Attributes:      coreIntf2,
			OndatraPortsIdx: []int{3, 4},
			LacpParams:      lacpParams,
			AggType:         oc.IfAggregate_AggregationType_LACP,
		},
	}

	custIntf = attrs.Attributes{
		Desc:    "Customer_connect",
		MTU:     defaultMTU,
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
		MAC:     "02:00:01:01:01:07",
	}

	coreIntf1 = attrs.Attributes{
		Desc:    "Core_Interface-1",
		MTU:     defaultMTU,
		IPv4:    "192.168.20.2",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:168:20:2",
		IPv6Len: plenIPv6,
		MAC:     "02:00:01:01:01:08",
	}

	coreIntf2 = attrs.Attributes{
		Desc:    "Core_Interface-2",
		MTU:     defaultMTU,
		IPv4:    "192.168.30.2",
		IPv4Len: plenIPv4,
		IPv6:    "2001:db8::192:168:30:2",
		IPv6Len: plenIPv6,
	}

	agg1 = &otgconfighelpers.Port{
		Name:        "Port-Channel1",
		AggMAC:      "02:00:01:01:01:01",
		MemberPorts: []string{"port1"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{otgIntf1},
		LagID:       1,
		IsLag:       true,
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
		{Size: 64, Weight: 20},
		{Size: 128, Weight: 20},
		{Size: 256, Weight: 20},
		{Size: 512, Weight: 18},
		{Size: 1024, Weight: 18},
	}

	decapSizeWeightProfile = []otgconfighelpers.SizeWeightPair{
		{Size: 64},
		{Size: 128},
		{Size: 256},
		{Size: 512, Weight: 48},
		{Size: 1024, Weight: 48},
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

	encapAgg2Validation = &packetvalidationhelpers.PacketValidation{
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

	decapValidation = &packetvalidationhelpers.PacketValidation{
		PortName:    custPort,
		CaptureName: "decapCapture",
		Validations: []packetvalidationhelpers.ValidationType{packetvalidationhelpers.ValidateVlanHeader},
		VlanLayer:   &packetvalidationhelpers.VlanLayer{VlanID: uint16(custLagData[0].SubInterfaces[0].VlanID)},
	}

	decapFlowInnerIPv4 = &otgconfighelpers.Flow{
		CustomFlow: &otgconfighelpers.CustomFlowParams{Bytes: "00000000"},
		EthFlow:    &otgconfighelpers.EthFlowParams{SrcMAC: "00:00:00:00:00:00"},
		VLANFlow:   &otgconfighelpers.VLANFlowParams{VLANId: uint32(custLagData[0].SubInterfaces[0].VlanID)},
		IPv4Flow:   &otgconfighelpers.IPv4FlowParams{IPv4Src: "22.1.1.1", IPv4Dst: strings.Split(staticTunnelDst, "/")[0]},
		TCPFlow:    &otgconfighelpers.TCPFlowParams{TCPSrcPort: 49152, TCPDstPort: 80},
	}
)

type BGPNeighbor struct {
	neighborip string
	deviceName string
	peerGroup  string
}

func configureDut(t *testing.T, dut *ondatra.DUTDevice, ocPFParams cfgplugins.OcPolicyForwardingParams) {
	for _, l := range custLagData {
		b := &gnmi.SetBatch{}
		// Create LAG interface
		l.LagName = netutil.NextAggregateInterface(t, dut)
		custAggID = l.LagName
		cfgplugins.NewAggregateInterface(t, dut, b, l)
		b.Set(t, dut)
	}

	for _, l := range coreLagData {
		b := &gnmi.SetBatch{}
		// Create LAG interface
		l.LagName = netutil.NextAggregateInterface(t, dut)
		cfgplugins.NewAggregateInterface(t, dut, b, l)
		b.Set(t, dut)
	}

	// Configure 16 tunnels having single destination address
	for index := range tunnelCount {
		tunnelSrcIPs = append(tunnelSrcIPs, fmt.Sprintf(startTunnelSrcIP, index+10))
	}
	_, ni, _ := cfgplugins.SetupPolicyForwardingInfraOC(ocPFParams.NetworkInstanceName)
	greNextHopGroupCfg := cfgplugins.GreNextHopGroupParams{
		NetworkInstance:  ni,
		NexthopGroupName: nexthopGroupName,
		GroupType:        nexthopType,
		SrcAddr:          tunnelSrcIPs,
		DstAddr:          []string{tunnelDestinationIP},
		TTL:              0,
	}

	cfgplugins.NextHopGroupConfigForMultipleIP(t, sfBatch, dut, greNextHopGroupCfg)

	//Configure MPLS label ranges and qos configs
	cfgplugins.MplsConfig(t, dut)
	cfgplugins.QosClassificationConfig(t, dut)
	cfgplugins.LabelRangeConfig(t, dut)

	configureStaticRoute(t, dut)

	t.Log("Configuring BGP")
	bgpProtocol := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := bgpProtocol.GetOrCreateBgp()
	g := bgp.GetOrCreateGlobal()
	g.As = ygot.Uint32(dutAS)

	nbrs := []*BGPNeighbor{
		{neighborip: otgIntf2.IPv4, deviceName: "", peerGroup: bgpPeerGroupName1},
		{neighborip: otgIntf3.IPv4, deviceName: "", peerGroup: bgpPeerGroupName2},
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
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticTunnelDst,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(otgIntf2.IPv4),
			"1": oc.UnionString(otgIntf3.IPv4),
		},
		NexthopGroup: false,
		T:            t,
	}

	if _, err := cfgplugins.NewStaticRouteCfg(sfBatch, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	sfBatch.Set(t, dut)
}

func configureIngressVlan(t *testing.T, dut *ondatra.DUTDevice, intfName string, subinterfaces int, mode string) {
	// Configuring port/attachment mode
	pseudowireCfg := cfgplugins.MplsStaticPseudowire{
		PseudowireName:   pseudowireName,
		NexthopGroupName: nexthopGroupName,
		LocalLabel:       fmt.Sprintf("%d", localLabel),
		RemoteLabel:      fmt.Sprintf("%d", remoteLabel),
		IntfName:         intfName,
	}

	vlanClientCfg := cfgplugins.VlanClientEncapsulationParams{
		IntfName:      intfName,
		Subinterfaces: subinterfaces,
	}

	switch mode {
	case "port":
		// Accepts packets from all VLANs
		cfgplugins.ConfigureMplsStaticPseudowire(t, sfBatch, dut, pseudowireCfg)
	case "attachment":
		// Accepts packets only for the specified VLAN
		cfgplugins.RemoveMplsStaticPseudowire(t, sfBatch, dut)
		pseudowireCfg.Subinterface = subinterfaces
		cfgplugins.ConfigureMplsStaticPseudowire(t, sfBatch, dut, pseudowireCfg)
		vlanClientCfg.RemoveVlanConfig = false
		cfgplugins.VlanClientEncapsulation(t, sfBatch, dut, vlanClientCfg)
	case "remove":
		cfgplugins.RemoveMplsStaticPseudowire(t, sfBatch, dut)
		vlanClientCfg.RemoveVlanConfig = true
		cfgplugins.VlanClientEncapsulation(t, sfBatch, dut, vlanClientCfg)
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

	// Create a slice of aggPortData for easier iteration
	aggs := []*otgconfighelpers.Port{agg1, agg2, agg3}

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

	if params.GREFlow != nil {
		params.AddGREHeader()
	}

	if params.MPLSFlow != nil {
		params.AddMPLSHeader()
	}

	if paramsInner != nil {
		if paramsInner.CustomFlow != nil {
			params.CustomFlow = paramsInner.CustomFlow
			params.AddCustomHeader()
		}
		if paramsInner.EthFlow != nil {
			params.EthFlow = paramsInner.EthFlow
			params.AddEthHeader()
		}
		if paramsInner.VLANFlow != nil {
			params.VLANFlow = paramsInner.VLANFlow
			params.AddVLANHeader()
		}
		if paramsInner.IPv4Flow != nil {
			params.IPv4Flow = paramsInner.IPv4Flow
			params.AddIPv4Header()
		}
		if paramsInner.TCPFlow != nil {
			params.TCPFlow = paramsInner.TCPFlow
			params.AddTCPHeader()
		}
	}

}

func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice, otgConfig gosnappi.Config) {
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

func verifyECMPLagBalance(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, flowName string) error {
	t.Log("Validating traffic equally distributed equally across 2 egress ports")
	var totalEgressPkts uint64
	tolerance := 0.10

	for _, egressPort := range []string{"port2", "port3", "port4", "port5"} {
		port := dut.Port(t, egressPort)
		egressPktPath := gnmi.OC().Interface(port.Name()).Counters().OutUnicastPkts().State()
		egressPkt, present := gnmi.Lookup(t, dut, egressPktPath).Val()
		if !present {
			return fmt.Errorf("isPresent status for path %q: got false, want true", egressPktPath)
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
		return fmt.Errorf("packet count of traffic sent from ATE port1 is not equal to the sum of all packets on DUT egress ports")
	}

	rxPortNames := []string{"port2", "port3", "port4", "port5"}
	var totalRxPkts, lag1RxPkts, lag2RxPkts uint64
	portRxPkts := make(map[string]uint64)

	for _, portName := range rxPortNames {
		rxPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(ate.Port(t, portName).ID()).Counters().InFrames().State())
		portRxPkts[portName] = rxPkts
		totalRxPkts += rxPkts
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
		return fmt.Errorf("ecmp hashing between LAG1 and LAG2 is unbalanced. Difference: %f%%, want < %f%%", ecmpError*100, tolerance*100)
	} else {
		t.Logf("ECMP hashing between LAGs is balanced. Difference: %f%%", ecmpError*100)
	}

	return nil
}

func verifyLoadBalanceAcrossGre(t *testing.T, singlePath bool, packetSource []*gopacket.PacketSource, greTunnelCount ...int) {
	tunnelCount := 16
	if len(greTunnelCount) != 0 {
		tunnelCount = greTunnelCount[0]
	}
	t.Log("Validating traffic equally load-balanced across GRE destinations")
	srcIPs := make(map[string]int)
	for _, pkt := range packetSource {
		for packet := range pkt.Packets() {
			if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
				ipv4, _ := ipLayer.(*layers.IPv4)
				srcIPs[ipv4.SrcIP.String()]++
			}
		}
	}

	uniqueCount := len(srcIPs)
	t.Logf("Found %d unique GRE source IPs in the capture", uniqueCount)

	if singlePath {
		if uniqueCount == 1 {
			t.Logf("PASS: Single-path mode: Found %d GRE source IP(s), as expected", uniqueCount)
			return
		}
		t.Errorf("no GRE source IPs found, expected at least 1")
		return
	}

	if uniqueCount < tunnelCount {
		t.Errorf("traffic was not load-balanced. Got %d unique GRE source IP(s), expected: %v", uniqueCount, tunnelCount)
		return
	}

	t.Logf("PASS: Traffic was load-balanced across %d GRE sources", uniqueCount)
}

func verifyTrafficFlow(t *testing.T, ate *ondatra.ATEDevice, otgConfig gosnappi.Config, otg *otg.OTG, flowName string) {
	otgutils.LogFlowMetrics(t, otg, otgConfig)
	otgutils.LogPortMetrics(t, otg, otgConfig)

	FlowIPv4Validation.Flow.Name = flowName
	if err := FlowIPv4Validation.ValidateLossOnFlows(t, ate); err != nil {
		t.Errorf("validation on flows failed (): %q", err)
	}
}

func validateEncapPacket(t *testing.T, ate *ondatra.ATEDevice, validationPacket *packetvalidationhelpers.PacketValidation) error {
	err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, validationPacket)
	return err
}

func TestEncapDecapGre(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	sfBatch = &gnmi.SetBatch{}

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Get default parameters for OC Policy Forwarding
	ocPFParams := GetDefaultOcPolicyForwardingParams()

	// Pass ocPFParams to ConfigureDut
	configureDut(t, dut, ocPFParams)

	// Configure on OTG
	otgConfig := ConfigureOTG(t, ate)
	packetvalidationhelpers.ConfigurePacketCapture(t, otgConfig, encapAgg2Validation)
	packetvalidationhelpers.ConfigurePacketCapture(t, otgConfig, encapAgg3Validation)
	packetvalidationhelpers.ConfigurePacketCapture(t, otgConfig, decapValidation)

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
				TxPort:            otgConfig.Lags().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[1].Name(), otgConfig.Lags().Items()[2].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGREv4Entropy",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: agg1.AggMAC, SrcMACCount: 1000, DstMAC: "01:01:01:01:01:01", DstMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: uint32(custLagData[0].SubInterfaces[0].VlanID)},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testEthoCWoMPLSoGREEncapIPv4WithEntropy,
		},
		{
			name:        "PF1.23.2 : EthoCWoMPLSoGRE encapsulated for unencrytped IPv4 without entropy",
			description: "Verify no hashing of EthoCWoMPLSoGRE encapsulation for unencrytped IPv4 traffic without entropy",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Lags().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[1].Name(), otgConfig.Lags().Items()[2].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGREv4WOEntropy ",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, DstMAC: "01:01:01:01:01:01"},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: uint32(custLagData[0].SubInterfaces[0].VlanID)},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testEthoCWoMPLSoGREEncapIPv4WithoutEntrpy,
		},
		{
			name:        "PF1.23.3 : EthoCWoMPLSoGRE encapsulate action for MACSec",
			description: "Verify PF EthoCWoMPLSoGRE encapsulate action for MACSec encrytped IPv4, IPv6 traffic",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Lags().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[1].Name(), otgConfig.Lags().Items()[2].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGREv4MACSec",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, SrcMACCount: 1000, DstMAC: "01:01:01:01:01:01", DstMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: uint32(custLagData[0].SubInterfaces[0].VlanID)},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testEthoCWoMPLSoGREEncapMACSec,
		},
		{
			name:        "PF1.23.4 : EthoCWoMPLSoGRE encapsulate action with Jumbo MTU",
			description: "Verify PF EthoCWoMPLSoGRE encapsulate action with Jumbo MTU",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Lags().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[1].Name(), otgConfig.Lags().Items()[2].Name()},
				IsTxRxPort:        true,
				FlowName:          "EthoMPLSoGREJumboMTU",
				SizeWeightProfile: &jumboWeightProfile,
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, SrcMACCount: 1000, DstMAC: "01:01:01:01:01:01", DstMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: uint32(custLagData[0].SubInterfaces[0].VlanID)},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testEthoCWoMPLSoGREEncapJumboMTU,
		},
		{
			name:        "PF1.23.5 : Verify Control word for unencrypted traffic flow",
			description: "Verify Control word for unencrypted traffic flow",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Lags().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[1].Name(), otgConfig.Lags().Items()[2].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGRECWUnEncrypted",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, SrcMACCount: 1000, DstMAC: "01:01:01:01:01:01", DstMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: uint32(custLagData[0].SubInterfaces[0].VlanID)},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testControlWordUnencrypted,
		},
		{
			name:        "PF1.23.6 : Verify Control word for encrypted traffic flow",
			description: "Verify Control word for encrypted traffic flow",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Lags().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[1].Name(), otgConfig.Lags().Items()[2].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGRECWEncrypted",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: uint32(custLagData[0].SubInterfaces[0].VlanID)},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testControlWordEncrypted,
		},
		{
			name:        "PF1.23.7 : Verify DSCP of EthoCWoMPLSoGRE encapsulated packets",
			description: "Verify DSCP of EthoCWoMPLSoGRE encapsulated packets",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Lags().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[1].Name(), otgConfig.Lags().Items()[2].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGREDSCP",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, SrcMACCount: 1000, DstMAC: "01:01:01:01:01:01", DstMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: uint32(custLagData[0].SubInterfaces[0].VlanID)},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testDSCPEthoCWoMPLSoGRE,
		},
		{
			name:        "PF1.23.8 : Verify PF EthoCWoMPLSoGRE encapsulate after single GRE tunnel destination shutdown",
			description: "Verify PF EthoCWoMPLSoGRE encapsulate after single GRE tunnel destination shutdown",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Lags().Items()[0].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[1].Name(), otgConfig.Lags().Items()[2].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &sizeWeightProfile,
				FlowName:          "EthoMPLSoGREv4",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: otgIntf1.MAC, SrcMACCount: 1000, DstMAC: "01:01:01:01:01:01", DstMACCount: 1000},
				VLANFlow:          &otgconfighelpers.VLANFlowParams{VLANId: uint32(custLagData[0].SubInterfaces[0].VlanID)},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "1.1.1.1", IPv4Dst: tunnelDestinationIP},
			},
			testFunc: testEthoCWoMPLSoGRE,
		},
		{
			name:        "PF1.23.9 : Verify PF EthoCWoMPLSoGRE decapsulate action",
			description: "Verify PF EthoCWoMPLSoGRE decapsulate action",
			flow: otgconfighelpers.Flow{
				TxPort:            otgConfig.Lags().Items()[1].Name(),
				RxPorts:           []string{otgConfig.Lags().Items()[0].Name()},
				IsTxRxPort:        true,
				SizeWeightProfile: &decapSizeWeightProfile,
				FlowName:          "EthoMPLSoGREv4Decap",
				EthFlow:           &otgconfighelpers.EthFlowParams{SrcMAC: coreIntf1.MAC},
				IPv4Flow:          &otgconfighelpers.IPv4FlowParams{IPv4Src: "100.64.0.0", IPv4Dst: strings.Split(decapDesIpv4IP, "/")[0], IPv4SrcCount: 1000},
				MPLSFlow:          &otgconfighelpers.MPLSFlowParams{MPLSLabel: localLabel},
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
		configureIngressVlan(t, dut, custAggID, custLagData[0].SubInterfaces[0].VlanID, "remove")
		for _, cfg := range modeConfig {
			switch cfg.mode {
			case "port":
				t.Log(cfg.name)
				configureIngressVlan(t, dut, custAggID, custLagData[0].SubInterfaces[0].VlanID, "port")
			case "attachment":
				t.Log(cfg.name)
				configureIngressVlan(t, dut, custAggID, custLagData[0].SubInterfaces[0].VlanID, "attachment")
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
	sendTrafficCapture(t, ate, otgConfig)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	verifyECMPLagBalance(t, dut, ate, flow.FlowName)

	for _, encapValidation := range []*packetvalidationhelpers.PacketValidation{encapAgg2Validation, encapAgg3Validation} {
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
			t.Errorf("capture and validatePackets failed (): %q", err)
		}

		packetSource = append(packetSource, packetvalidationhelpers.SourceObj())
	}

	verifyLoadBalanceAcrossGre(t, false, packetSource)
}

func testEthoCWoMPLSoGREEncapIPv4WithoutEntrpy(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	// Flows should have NOT have entropy on any headers.
	createflow(otgConfig, &flow, true, nil)
	sendTrafficCapture(t, ate, otgConfig)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	verifyECMPLagBalance(t, dut, ate, flow.FlowName)

	err := validateEncapPacket(t, ate, encapAgg2Validation)
	if err != nil {
		// If error validating the whether encap packet received on other lag port
		if err = validateEncapPacket(t, ate, encapAgg3Validation); err != nil {
			t.Errorf("capture and validatePackets failed (): %q", err)
		}
		packetSource = append(packetSource, packetvalidationhelpers.SourceObj())
	} else {
		packetSource = append(packetSource, packetvalidationhelpers.SourceObj())
	}

	verifyLoadBalanceAcrossGre(t, true, packetSource)
}

func testEthoCWoMPLSoGREEncapMACSec(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	// Flows should have NOT have entropy on any headers.
	createflow(otgConfig, &flow, true, nil)

	if !deviations.MacsecOCUnsupported(dut) {
		// TODO: MACSec not supported in snappi
		flowObj := otgConfig.Flows().Items()[0]
		flowObj.Packet().Add().Macsec()
		sendTrafficCapture(t, ate, otgConfig)
		verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

		if err := verifyECMPLagBalance(t, dut, ate, flow.FlowName); err != nil {
			t.Logf("ecmp hashing between LAG1 and LAG2 is unbalanced as expected")
		} else {
			t.Errorf("ecmp hashing between LAG1 and LAG2 is balanced which is unexpected")
		}

		err := validateEncapPacket(t, ate, encapAgg2Validation)
		if err != nil {
			// If error validating the whether encap packet received on other lag port
			if err = validateEncapPacket(t, ate, encapAgg3Validation); err != nil {
				t.Errorf("capture and validatePackets failed (): %q", err)
			}
			packetSource = append(packetSource, packetvalidationhelpers.SourceObj())
		} else {
			packetSource = append(packetSource, packetvalidationhelpers.SourceObj())
		}

		verifyLoadBalanceAcrossGre(t, true, packetSource)
	}

}

func testEthoCWoMPLSoGREEncapJumboMTU(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	// Generate 1000 different traffic flows on ATE Port 1
	createflow(otgConfig, &flow, true, nil)
	sendTrafficCapture(t, ate, otgConfig)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	for _, encapValidation := range []*packetvalidationhelpers.PacketValidation{encapAgg2Validation, encapAgg3Validation} {
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
			t.Errorf("capture and validatePackets failed (): %q", err)
		}

		packetSource = append(packetSource, packetvalidationhelpers.SourceObj())
	}

	verifyLoadBalanceAcrossGre(t, false, packetSource)
}

func testControlWordUnencrypted(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	createflow(otgConfig, &flow, true, nil)
	sendTrafficCapture(t, ate, otgConfig)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	verifyECMPLagBalance(t, dut, ate, flow.FlowName)

	for _, encapValidation := range []*packetvalidationhelpers.PacketValidation{encapAgg2Validation, encapAgg3Validation} {
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
			t.Errorf("capture and validatePackets failed (): %q", err)
		}

		packetSource = append(packetSource, packetvalidationhelpers.SourceObj())
	}

	verifyLoadBalanceAcrossGre(t, false, packetSource)
}

func testControlWordEncrypted(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	createflow(otgConfig, &flow, true, nil)
	sendTrafficCapture(t, ate, otgConfig)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	encapAgg2Validation.MPLSLayer = controlWordMPLS
	err := validateEncapPacket(t, ate, encapAgg2Validation)
	if err != nil {
		// If error validating the whether encap packet received on other lag port
		encapAgg3Validation.MPLSLayer = controlWordMPLS
		if err = validateEncapPacket(t, ate, encapAgg3Validation); err != nil {
			t.Errorf("capture and validatePackets failed (): %q", err)
		}
		packetSource = append(packetSource, packetvalidationhelpers.SourceObj())
	} else {
		packetSource = append(packetSource, packetvalidationhelpers.SourceObj())
	}

	verifyLoadBalanceAcrossGre(t, true, packetSource)
}

func testDSCPEthoCWoMPLSoGRE(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	greNextHopGroupCfg := cfgplugins.GreNextHopGroupParams{
		NetworkInstance:  ni,
		NexthopGroupName: nexthopGroupName,
		GroupType:        nexthopType,
		Dscp:             dscp,
	}

	cfgplugins.NextHopGroupConfigForMultipleIP(t, sfBatch, dut, greNextHopGroupCfg)

	createflow(otgConfig, &flow, true, nil)
	sendTrafficCapture(t, ate, otgConfig)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	InnerIPLayerIPv4 := &packetvalidationhelpers.IPv4Layer{
		Tos: dscp,
	}

	for _, encapValidation := range []*packetvalidationhelpers.PacketValidation{encapAgg2Validation, encapAgg3Validation} {
		encapValidation.InnerIPLayerIPv4 = InnerIPLayerIPv4
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
			t.Errorf("capture and validatePackets failed (): %q", err)
		}

		packetSource = append(packetSource, packetvalidationhelpers.SourceObj())
	}

	verifyLoadBalanceAcrossGre(t, false, packetSource)

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
	sendTrafficCapture(t, ate, otgConfig)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	verifyECMPLagBalance(t, dut, ate, flow.FlowName)

	for _, encapValidation := range []*packetvalidationhelpers.PacketValidation{encapAgg2Validation, encapAgg3Validation} {
		if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, encapValidation); err != nil {
			t.Errorf("capture and validatePackets failed (): %q", err)
		}

		packetSource = append(packetSource, packetvalidationhelpers.SourceObj())
	}

	verifyLoadBalanceAcrossGre(t, false, packetSource, 15)
}

func testEthoCWoMPLSoGREDecap(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	sfBatch := &gnmi.SetBatch{}
	cfgplugins.PolicyForwardingGreDecapsulation(t, sfBatch, dut, strings.Split(decapDesIpv4IP, "/")[0], "trafficPolicyName", custPort, decapGrpName)

	linklayerAddress := gnmi.GetAll(t, dut, gnmi.OC().Interface(custAggID).Subinterface(0).Ipv6().AddressAny().Ip().State())[0]

	ip := net.ParseIP(linklayerAddress).To16()
	mac := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", ip[8]^2, ip[9], ip[10], ip[13], ip[14], ip[15])
	flow.EthFlow.DstMAC = mac

	createflow(otgConfig, &flow, true, decapFlowInnerIPv4)
	sendTrafficCapture(t, ate, otgConfig)

	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)
}

func testEthoCWoMPLSoGREDecapVLAN(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, otg *otg.OTG, otgConfig gosnappi.Config, flow otgconfighelpers.Flow) {
	sendTrafficCapture(t, ate, otgConfig)
	verifyTrafficFlow(t, ate, otgConfig, otg, flow.FlowName)

	if err := packetvalidationhelpers.CaptureAndValidatePackets(t, ate, decapValidation); err != nil {
		t.Errorf("capture and validatePackets failed (): %q", err)
	}
}
