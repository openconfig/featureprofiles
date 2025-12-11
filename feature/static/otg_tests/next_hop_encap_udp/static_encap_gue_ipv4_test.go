package staticencapgueipv4_test

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
	otgconfighelpers "github.com/openconfig/featureprofiles/internal/otg_helpers/otg_config_helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
)

const (
	plenIPv4 = 30

	// Static and GUE address
	nexthopGroupName   = "GUE-NHG"
	nexthopGroupNameV6 = "GUE-NHGv6"
	GuePolicyName      = "GUE-Policy"
	pnhIPv6            = "fc00:10::1"
	pnhIPv6NGv6        = "fc00:10::2"
	staticpnhIPv6      = "fc00:10::1/128"
	staticpngv6IPv6    = "fc00:10::2/128"
	gueDstIPv4         = "10.50.50.1"
	staticgueDstIPv4   = "10.50.50.1/32"
	trafficDstNetIPv4  = "10.1.1.1"
	trafficDstNetIPv6  = "2001:DB8::10:1:1:1"

	// Random Source Address
	trafficSrcNetIPv4 = "1.1.1.1"
	trafficSrcNetIPv6 = "2001:db8::aaaa:0"

	frameSize          = 512
	packetCount        = 12000000
	gueProtocolPort    = 6080
	gueSrcProtocolPort = 5000

	tolerancePct = 6.0
	tolerance    = 0.06
)

var (
	dutLAG1 = attrs.Attributes{
		Desc:    "DUT to ATE LAG1",
		IPv4:    "192.0.2.5",
		IPv4Len: plenIPv4,
	}

	dutLAG2 = attrs.Attributes{
		Desc:    "DUT to ATE LAG2",
		IPv4:    "192.0.2.9",
		IPv4Len: plenIPv4,
	}

	activity = oc.Lacp_LacpActivityType_ACTIVE
	period   = oc.Lacp_LacpPeriodType_FAST

	lacpParams = &cfgplugins.LACPParams{
		Activity: &activity,
		Period:   &period,
	}

	dutLagData = []*cfgplugins.DUTAggData{
		{
			Attributes:      dutLAG1,
			OndatraPortsIdx: []int{1, 2},
			LacpParams:      lacpParams,
			AggType:         oc.IfAggregate_AggregationType_LACP,
		},
		{
			Attributes:      dutLAG2,
			OndatraPortsIdx: []int{3, 4},
			LacpParams:      lacpParams,
			AggType:         oc.IfAggregate_AggregationType_LACP,
		},
	}

	agg2 = &otgconfighelpers.Port{
		Name:        "Port-Channel2",
		AggMAC:      "02:00:01:01:01:03",
		MemberPorts: []string{"port2", "port3"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{otgIntf2},
		LagID:       2,
		IsLag:       true,
	}

	agg3 = &otgconfighelpers.Port{
		Name:        "Port-Channel3",
		AggMAC:      "02:00:01:01:01:04",
		MemberPorts: []string{"port4", "port5"},
		Interfaces:  []*otgconfighelpers.InterfaceProperties{otgIntf3},
		LagID:       3,
		IsLag:       true,
	}

	otgIntf2 = &otgconfighelpers.InterfaceProperties{
		Name:        "ateLag1",
		IPv4:        "192.0.2.6",
		IPv4Gateway: "192.0.2.5",
		IPv4Len:     plenIPv4,
		MAC:         "02:00:00:00:01:01",
	}

	otgIntf3 = &otgconfighelpers.InterfaceProperties{
		Name:        "ateLag2",
		IPv4:        "192.0.2.10",
		IPv4Gateway: "192.0.2.9",
		IPv4Len:     plenIPv4,
		MAC:         "02:00:00:00:02:02",
	}

	dutPort1Mac string
)

var capturePorts = []string{"port2", "port3", "port4", "port5"}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func mustConfigureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	for _, l := range dutLagData {
		b := &gnmi.SetBatch{}
		// Create LAG interface
		l.LagName = netutil.NextAggregateInterface(t, dut)
		cfgplugins.NewAggregateInterface(t, dut, b, l)
		b.Set(t, dut)
	}
	// Configure GUE Encap
	configureGueEncap(t, dut, []string{gueDstIPv4}, 0)

	t.Log("Configuring Static Routes")

	// Configuring Static Route: IPv4-DST-GUE --> ATE:LAG1:IPv4 & ATE:LAG2:IPv4.
	b := &gnmi.SetBatch{}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          staticgueDstIPv4,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(otgIntf2.IPv4),
			"1": oc.UnionString(otgIntf3.IPv4),
		},
		NexthopGroup: false,
		T:            t,
	}

	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)

	// Configuring Static Route: PNH-IPv6 --> IPv4 GUE tunnel.
	sV4 = &cfgplugins.StaticRouteCfg{
		NetworkInstance:  deviations.DefaultNetworkInstance(dut),
		Prefix:           staticpnhIPv6,
		NexthopGroup:     true,
		NexthopGroupName: nexthopGroupName,
		T:                t,
		TrafficType:      oc.Aft_EncapsulationHeaderType_UDPV4,
		PolicyName:       GuePolicyName,
		Rule:             "rule1",
	}

	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv6 static route: %v", err)
	}
	b.Set(t, dut)

	sV4 = &cfgplugins.StaticRouteCfg{
		NetworkInstance:  deviations.DefaultNetworkInstance(dut),
		Prefix:           staticpngv6IPv6,
		NexthopGroup:     true,
		NexthopGroupName: nexthopGroupNameV6,
		T:                t,
		TrafficType:      oc.Aft_EncapsulationHeaderType_UDPV6,
		PolicyName:       GuePolicyName,
		Rule:             "rule2",
	}

	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)

	cfgplugins.ConfigureLoadbalance(t, dut)
}

// configureGueTunnel configures a GUE tunnel with optional ToS and TTL.
func configureGueEncap(t *testing.T, dut *ondatra.DUTDevice, dstAddr []string, ttl uint8) {
	t.Helper()
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

	v4NexthopUDPParams := cfgplugins.NexthopGroupUDPParams{
		IPFamily:           "V4Udp",
		NexthopGrpName:     nexthopGroupName,
		Index:              "0",
		SrcIp:              dut.Port(t, "port1").Name(),
		DstIp:              dstAddr,
		TTL:                ttl,
		DstUdpPort:         gueProtocolPort,
		NetworkInstanceObj: ni,
		DeleteTtl:          false,
	}
	// Create nexthop group for v4
	cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, v4NexthopUDPParams)

	v6NexthopUDPParams := cfgplugins.NexthopGroupUDPParams{
		IPFamily:           "V6Udp",
		NexthopGrpName:     nexthopGroupNameV6,
		Index:              "1",
		SrcIp:              dut.Port(t, "port1").Name(),
		DstIp:              dstAddr,
		TTL:                ttl,
		DstUdpPort:         gueProtocolPort,
		NetworkInstanceObj: ni,
		DeleteTtl:          false,
	}
	// Create nexthop group for v4
	cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, v6NexthopUDPParams)

	// Apply traffic policy on interface
	if deviations.NextHopGroupOCUnsupported(dut) {
		interfacePolicyParams := cfgplugins.OcPolicyForwardingParams{
			InterfaceName: dut.Port(t, "port1").Name(),
			PolicyName:    GuePolicyName,
		}
		cfgplugins.InterfacePolicyForwardingApply(t, dut, interfacePolicyParams)
	}
}

type tunnelCfg struct {
	policyName       string
	protocolType     string
	nexthopGroupName string
	index            string
	tos              uint8
	ttl              uint8
	deleteTos        bool
	deleteTtl        bool
}

func configureTosTtlOnTunnel(t *testing.T, dut *ondatra.DUTDevice, cfg *tunnelCfg) {
	t.Helper()

	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))

	if cfg.tos != 0 || cfg.deleteTos {
		nexthopUDPParams := cfgplugins.NexthopGroupUDPParams{
			IPFamily:           cfg.protocolType,
			NexthopGrpName:     cfg.nexthopGroupName,
			DstUdpPort:         gueProtocolPort,
			Index:              cfg.index,
			NetworkInstanceObj: ni,
			DSCP:               cfg.tos,
			DeleteDSCP:         cfg.deleteTos,
			SrcIp:              dut.Port(t, "port1").Name(),
		}
		cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, nexthopUDPParams)
	}

	if cfg.ttl != 0 || cfg.deleteTtl {

		nexthopUDPParams := cfgplugins.NexthopGroupUDPParams{
			IPFamily:           cfg.protocolType,
			NexthopGrpName:     cfg.nexthopGroupName,
			DstUdpPort:         gueProtocolPort,
			TTL:                cfg.ttl,
			Index:              cfg.index,
			NetworkInstanceObj: ni,
			DeleteTtl:          cfg.deleteTtl,
		}

		cfgplugins.NextHopGroupConfigForIpOverUdp(t, dut, nexthopUDPParams)
	}

}

// configureATE creates the base OTG configuration.
func configureATE(t *testing.T, bs *cfgplugins.BGPSession) {
	t.Helper()
	devices := bs.ATETop.Devices().Items()
	bgp6Peer := devices[0].Bgp().Ipv6Interfaces().Items()[0].Peers().Items()[0]

	// Configure IPv4-DST-NET/32 and IPv6-DST-NET/128 routes to DUT
	v4Route := bgp6Peer.V4Routes().Add().SetName("v4Net")
	v4Route.Addresses().Add().SetAddress(trafficDstNetIPv4).SetPrefix(32).SetCount(1)
	v4Route.SetNextHopIpv6Address(pnhIPv6).SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL)

	v6Route := bgp6Peer.V6Routes().Add().SetName("v6Net")
	v6Route.Addresses().Add().SetAddress(trafficDstNetIPv6).SetPrefix(128).SetCount(1)
	v6Route.SetNextHopIpv6Address(pnhIPv6NGv6).SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL)

	// Create a slice of aggPortData for easier iteration
	aggs := []*otgconfighelpers.Port{agg2, agg3}

	// Configure OTG Interfaces
	for _, agg := range aggs {
		otgconfighelpers.ConfigureNetworkInterface(t, bs.ATETop, bs.ATE, agg)
	}

}

func createFlow(otgConfig gosnappi.Config, flowName string, flowtype string, ipAddr string, tos int, ttl uint32, singleFlow bool, dstport uint32, srcPort uint32, icmp bool) {
	otgConfig.Flows().Clear()
	flow := otgConfig.Flows().Add().SetName(flowName)
	flow.TxRx().Device().
		SetTxNames([]string{otgConfig.Ports().Items()[0].Name() + ".IPv4"}).
		SetRxNames([]string{agg2.Interfaces[0].Name + ".IPv4", agg3.Interfaces[0].Name + ".IPv4"})

	flow.Metrics().SetEnable(true)
	flow.Size().SetFixed(frameSize)
	flow.Rate().SetPercentage(10)
	flow.Duration().FixedPackets().SetPackets(packetCount)

	e1 := flow.Packet().Add().Ethernet()
	e1.Dst().Auto()

	// Adding outer IP header
	if flowtype == "ipv4" {
		v4 := flow.Packet().Add().Ipv4()
		if singleFlow {
			v4.Src().SetValue(trafficSrcNetIPv4)
			v4.Dst().SetValue(ipAddr)
		} else {
			v4.Src().Increment().SetStart(trafficSrcNetIPv4).SetCount(256)
			v4.Dst().Increment().SetStart(ipAddr).SetCount(256)
		}
		v4.Priority().Tos().Precedence().SetValue(uint32(tos >> 5))
		v4.TimeToLive().SetValue(ttl)
		if icmp {
			flow.Packet().Add().Icmp()
		}
	} else {
		v6 := flow.Packet().Add().Ipv6()
		if singleFlow {
			v6.Src().SetValue(trafficSrcNetIPv6)
			v6.Dst().SetValue(ipAddr)
			udp := flow.Packet().Add().Udp()
			udp.DstPort().SetValue(dstport)
			udp.SrcPort().SetValue(srcPort)
		} else {
			v6.Src().Increment().SetStart(trafficSrcNetIPv6).SetCount(256)
			v6.Dst().Increment().SetStart(ipAddr).SetCount(256)
			udp := flow.Packet().Add().Udp()
			udp.DstPort().Increment().SetStart(dstport).SetCount(256)
			udp.SrcPort().Increment().SetStart(srcPort).SetCount(256)
		}
		v6.TrafficClass().SetValue(uint32(tos))
		v6.HopLimit().SetValue(ttl)
		if icmp {
			flow.Packet().Add().Icmpv6()
		}
	}
}

// verifyTraffic checks packet counts and ECMP load balancing.
func verifyTraffic(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, config gosnappi.Config, flowName string) {
	var totalEgressPkts uint64

	for _, egressPort := range []string{"port2", "port3", "port4", "port5"} {
		port := dut.Port(t, egressPort)
		egressPktPath := gnmi.OC().Interface(port.Name()).Counters().OutUnicastPkts().State()
		egressPkt, present := gnmi.Lookup(t, dut, egressPktPath).Val()
		if !present {
			t.Errorf("get IsPresent status for path %q: got false, want true", egressPktPath)
		}
		totalEgressPkts += egressPkt
	}

	otg := ate.OTG()
	otgutils.LogFlowMetrics(t, otg, config)

	txPkts := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().OutPkts().State())
	if txPkts == 0 {
		t.Fatal("ATE did not send any packets")
	}

	dutlossPct := (float64(txPkts) - float64(totalEgressPkts)) * 100 / float64(txPkts)
	if dutlossPct < tolerancePct {
		t.Log("The packet count of traffic sent from ATE port1 equal to the sum of all packets on DUT egress ports")
	} else {
		t.Errorf("the packet count of traffic sent from ATE port1 is not equal to the sum of all packets on DUT egress ports")
	}

	rxPortNames := []string{"port2", "port3", "port4", "port5"}
	var totalRxPkts, lag1RxPkts, lag2RxPkts uint64
	portRxPkts := make(map[string]uint64)

	for _, portName := range rxPortNames {
		rxPkts := gnmi.Get(t, otg, gnmi.OTG().Port(ate.Port(t, portName).ID()).Counters().InFrames().State())
		portRxPkts[portName] = rxPkts
		totalRxPkts += rxPkts
	}

	lossPct := (float64(txPkts) - float64(totalRxPkts)) * 100 / float64(txPkts)
	if lossPct > tolerancePct {
		t.Errorf("high packet loss detected: got %f%%, want < %f%%", lossPct, tolerancePct)
	} else {
		t.Logf("Packet loss is within tolerance: %f%%", lossPct)
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
		t.Errorf("ecmp hashing between LAG1 and LAG2 is unbalanced. Difference: %f%%, want < %f%%", ecmpError*100, tolerance*100)
	} else {
		t.Logf("ECMP hashing between LAGs is balanced. Difference: %f%%", ecmpError*100)
	}
}

func enableCapture(t *testing.T, config gosnappi.Config, portNames []string) {
	config.Captures().Clear()
	for _, portName := range portNames {
		t.Logf("Configuring packet capture for OTG port: %s", portName)
		cap := config.Captures().Add()
		cap.SetName(portName)
		cap.SetPortNames([]string{portName})
		cap.SetFormat(gosnappi.CaptureFormat.PCAP)
	}
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

func mustProcessCapture(t *testing.T, otg *otg.OTG, port string) string {
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	time.Sleep(30 * time.Second)
	capture, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := capture.Write(bytes); err != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	defer capture.Close()

	return capture.Name()
}

func validatePackets(t *testing.T, filename string, protocolType string, outertos, innertos string, outerttl, innerttl uint8, outerPacket bool) {
	var packetCount uint32 = 0

	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		packetCount += 1
		ipOuterLayer, ok := ipLayer.(*layers.IPv4)
		if !ok || ipOuterLayer == nil {
			t.Errorf("outer IP layer not found %d", ipLayer)
			return
		}

		udpLayer := packet.Layer(layers.LayerTypeUDP)
		udp, ok := udpLayer.(*layers.UDP)
		if !ok || udp == nil {
			t.Error("gue layer not found")
			return
		} else {
			if udp.DstPort == gueProtocolPort {
				t.Log("Got the encapsulated GUE layer")
			}

			if outerPacket {
				validateOuterPacket(t, ipOuterLayer, outertos, outerttl)
			}

			var gotInnerPacketTOS, gotInnerttl uint8
			dscp := ""

			switch protocolType {
			case "ipv4":
				innerPacket := gopacket.NewPacket(udp.Payload, layers.LayerTypeIPv4, gopacket.Default)
				ipLayer := innerPacket.Layer(layers.LayerTypeIPv4)
				if ipLayer == nil {
					t.Errorf("inner layer of type %s not found", protocolType)
					return
				}
				ip, _ := ipLayer.(*layers.IPv4)
				gotInnerPacketTOS = ip.TOS
				dscp = fmt.Sprintf("0x%X", gotInnerPacketTOS)
				gotInnerttl = ip.TTL
			case "ipv6":
				innerPacket := gopacket.NewPacket(udp.Payload, layers.LayerTypeIPv6, gopacket.Default)
				ipLayer := innerPacket.Layer(layers.LayerTypeIPv6)
				if ipLayer == nil {
					t.Errorf("inner layer of type %s not found", protocolType)
					return
				}
				ip, _ := ipLayer.(*layers.IPv6)
				gotInnerPacketTOS = ip.TrafficClass
				dscp = fmt.Sprintf("0x%X", gotInnerPacketTOS)
				gotInnerttl = ip.HopLimit
			}

			if innerttl != 0 {
				if innerttl == gotInnerttl {
					t.Logf("TTL matched: expected ttl %d, got ttl %d", innerttl, gotInnerttl)
				} else {
					t.Errorf("ttl mismatch: expected ttl %d, got ttl %d", innerttl, gotInnerttl)
				}
			}
			if innertos != "" {
				if dscp == innertos {
					t.Logf("TOS matched: expected TOS %v, got TOS %v", innertos, dscp)
				} else {
					t.Errorf("tos mismatch: expected TOS %v, got TOS %v", innertos, dscp)
				}
			}

		}
		break
	}

}

func validateOuterPacket(t *testing.T, outerPacket *layers.IPv4, tos string, ttl uint8) {
	var outerTOS, outerttl uint8
	outerDSCP := ""

	outerttl = outerPacket.TTL
	outerTOS = outerPacket.TOS
	outerDSCP = fmt.Sprintf("0x%X", outerTOS)

	if ttl != 0 {
		if outerttl == ttl {
			t.Logf("Outer TTL matched: expected ttl %d, got ttl %d", ttl, outerttl)
		} else {
			t.Errorf("outer TTL mismatch: expected ttl %d, got ttl %d", ttl, outerttl)
		}
	}
	if tos != "" {
		if outerDSCP == tos {
			t.Logf("Outer TOS matched: expected TOS %v, got TOS %v", tos, outerDSCP)
		} else {
			t.Errorf("outer TOS mismatch: expected TOS %v, got TOS %v", tos, outerDSCP)
		}
	}
}

// verifySinglePathTraffic checks that traffic is received on only one egress port.
func verifySinglePathTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flowName string) []string {
	otg := ate.OTG()

	otgutils.LogFlowMetrics(t, otg, config)

	txPkts := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().OutPkts().State())
	if txPkts == 0 {
		t.Fatal("ATE did not send any packets")
	}

	rxPortNames := []string{"port2", "port3", "port4", "port5"}
	var totalRxPkts uint64
	var activePorts []string

	for _, portName := range rxPortNames {
		rxPkts := gnmi.Get(t, otg, gnmi.OTG().Port(ate.Port(t, portName).ID()).Counters().InFrames().State())
		totalRxPkts += rxPkts
		if rxPkts >= packetCount {
			activePorts = append(activePorts, portName)
		}
	}

	lossPct := (float64(txPkts) - float64(totalRxPkts)) * 100 / float64(txPkts)
	if lossPct > tolerancePct {
		t.Errorf("high packet loss detected: got %f%%, want < %f%%", lossPct, tolerancePct)
	} else {
		t.Logf("Packet loss is within tolerance: %f%%", lossPct)
	}

	if len(activePorts) == 0 {
		t.Errorf("no traffic was received on any destination port")
	} else if len(activePorts) > 1 {
		t.Errorf("traffic was received on multiple ports %v, expected only one", activePorts)
	} else {
		t.Logf("Traffic correctly hashed to a single port: %s", activePorts[0])
	}

	return activePorts
}

// verifyTTLDropAndICMP verifies that the TTL=1 flow is dropped and ICMP responses are received.
func verifyTTLDropAndICMP(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flowname string, captureName string) {
	t.Helper()
	otg := ate.OTG()

	otgutils.LogFlowMetrics(t, otg, config)

	outTxPkts := gnmi.Get(t, otg, gnmi.OTG().Flow(flowname).Counters().OutPkts().State())
	if outTxPkts == 0 {
		t.Fatal("ATE did not send any packets for the TTL=1 flow")
	}

	rxPortNames := []string{"port2", "port3", "port4", "port5"}
	var totalRxPkts uint64

	for _, portName := range rxPortNames {
		rxPkts := gnmi.Get(t, otg, gnmi.OTG().Port(ate.Port(t, portName).ID()).Counters().InFrames().State())
		totalRxPkts += rxPkts
	}

	if totalRxPkts >= packetCount {
		t.Errorf("received %d packets on destination ports, but expected 0 due to TTL expiry", totalRxPkts)
	} else {
		t.Logf("Successfully verified that no packets for flow %s were received at the GUE destinations", flowname)
	}

	handle, err := pcap.OpenOffline(captureName)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		ipv6Layer := packet.Layer(layers.LayerTypeIPv6)
		if ipv6Layer != nil {
			icmpv6Layer := packet.Layer(layers.LayerTypeICMPv6)
			if icmpv6Layer != nil {
				icmpPacket, _ := icmpv6Layer.(*layers.ICMPv6)
				if icmpPacket.TypeCode.Type() == layers.ICMPv6TypeRouterAdvertisement && icmpPacket.TypeCode.Code() == 0 {
					t.Log("PASS: received ICMPv6 code-0")
				} else {
					t.Errorf("did not received ICMPv6 code-0, Got Code: %v , type: %v", icmpPacket.TypeCode.Code(), icmpPacket.TypeCode.Type())
				}
			}
		}
	}
}

func configureHardwareInit(t *testing.T, dut *ondatra.DUTDevice) {
	hardwareInitCfg := cfgplugins.NewDUTHardwareInit(t, dut, cfgplugins.FeatureNGPR)
	if hardwareInitCfg == "" {
		return
	}
	cfgplugins.PushDUTHardwareInitConfig(t, dut, hardwareInitCfg)

}

func TestGUEEncap(t *testing.T) {
	bs := cfgplugins.NewBGPSession(t, cfgplugins.PortCount1, nil)
	bs.WithEBGP(t, []oc.E_BgpTypes_AFI_SAFI_TYPE{oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST}, []string{"port1"}, true, true)

	configureHardwareInit(t, bs.DUT)

	mustConfigureDUT(t, bs.DUT)
	dutPort1Mac = gnmi.Get(t, bs.DUT, gnmi.OC().Interface(bs.DUT.Port(t, "port1").Name()).Ethernet().MacAddress().State())

	configureATE(t, bs)

	if err := bs.PushDUT(t); err != nil {
		t.Error(err)
	}

	t.Run("RT-3.53.1: IPv4 GUE Encap without explicit ToS/TTL", func(t *testing.T) {

		createFlow(bs.ATETop, "IPv4-GUE-Traffic", "ipv4", trafficDstNetIPv4, 0x80, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, bs.ATETop, capturePorts)
		bs.ATE.OTG().PushConfig(t, bs.ATETop)

		t.Log("Start ATE protocols and verify BGP session")
		bs.ATE.OTG().StartProtocols(t)

		t.Log("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
		t.Log("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

		t.Log("Starting capture")
		cs := startCapture(t, bs.ATE.OTG())

		t.Log("Starting traffic")
		bs.ATE.OTG().StartTraffic(t)

		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)

		t.Log("Stop Capture")
		stopCapture(t, bs.ATE.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, bs.DUT, bs.ATE, bs.ATETop, "IPv4-GUE-Traffic")

		for _, p := range capturePorts {
			capture := mustProcessCapture(t, bs.ATE.OTG(), p)
			validatePackets(t, capture, "ipv4", "0x80", "0x80", 64, 9, true)
		}

	})

	t.Run("RT-3.53.2: IPv6 traffic GUE encapsulation without explicit ToS/TTL configuration on tunnel", func(t *testing.T) {

		createFlow(bs.ATETop, "IPv6-GUE-Traffic", "ipv6", trafficDstNetIPv6, 0x80, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, bs.ATETop, capturePorts)
		bs.ATE.OTG().PushConfig(t, bs.ATETop)
		t.Log("Start ATE protocols and verify BGP session")
		bs.ATE.OTG().StartProtocols(t)

		t.Log("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
		t.Log("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

		t.Log("Starting capture")
		cs := startCapture(t, bs.ATE.OTG())

		t.Log("Starting traffic")
		bs.ATE.OTG().StartTraffic(t)

		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)

		t.Log("Stop Capture")
		stopCapture(t, bs.ATE.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, bs.DUT, bs.ATE, bs.ATETop, "IPv6-GUE-Traffic")

		for _, p := range capturePorts {
			capture := mustProcessCapture(t, bs.ATE.OTG(), p)
			validatePackets(t, capture, "ipv6", "0x80", "0x80", 64, 9, false)
		}

	})

	t.Run("RT-3.53.3: IPv4 GUE Encap with explicit ToS", func(t *testing.T) {
		t.Log("Configure DUT with explicit ToS on GUE tunnel")
		configureTosTtlOnTunnel(
			t,
			bs.DUT,
			&tunnelCfg{policyName: "policy1",
				tos:              0x60,
				ttl:              0,
				deleteTos:        false,
				deleteTtl:        false,
				protocolType:     "V4Udp",
				nexthopGroupName: nexthopGroupName,
			})

		createFlow(bs.ATETop, "IPv4-GUE-Traffic", "ipv4", trafficDstNetIPv4, 0x80, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, bs.ATETop, capturePorts)
		bs.ATE.OTG().PushConfig(t, bs.ATETop)

		t.Log("Start ATE protocols and verify BGP session")
		bs.ATE.OTG().StartProtocols(t)

		t.Log("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
		t.Log("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)
		t.Logf("Starting capture")
		cs := startCapture(t, bs.ATE.OTG())

		t.Log("Starting traffic")
		bs.ATE.OTG().StartTraffic(t)

		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)

		t.Log("Stop Capture")
		stopCapture(t, bs.ATE.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, bs.DUT, bs.ATE, bs.ATETop, "IPv4-GUE-Traffic")

		for _, p := range capturePorts {
			capture := mustProcessCapture(t, bs.ATE.OTG(), p)
			validatePackets(t, capture, "ipv4", "", "0x60", 0, 0, false)
		}
	})

	// Set the ToS on the packets from ATE and have the DUT copy it to the outer ToS field
	t.Run("RT-3.53.4: IPv6 GUE Encap with explicit ToS", func(t *testing.T) {
		// Configure TOS on ATE, and validate the TOS is copied by DUT on outer header
		createFlow(bs.ATETop, "IPv6-GUE-Traffic", "ipv6", trafficDstNetIPv6, 0x60, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, bs.ATETop, capturePorts)
		bs.ATE.OTG().PushConfig(t, bs.ATETop)
		t.Log("Start ATE protocols and verify BGP session")
		bs.ATE.OTG().StartProtocols(t)

		t.Log("Starting capture")
		cs := startCapture(t, bs.ATE.OTG())

		t.Log("Starting traffic")
		bs.ATE.OTG().StartTraffic(t)

		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)

		t.Log("Stop Capture")
		stopCapture(t, bs.ATE.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, bs.DUT, bs.ATE, bs.ATETop, "IPv6-GUE-Traffic")

		for _, p := range capturePorts {
			capture := mustProcessCapture(t, bs.ATE.OTG(), p)
			validatePackets(t, capture, "ipv6", "", "0x60", 0, 0, false)
		}
	})

	t.Run("RT-3.53.5: IPv4 GUE Encap with explicit TTL", func(t *testing.T) {
		configureTosTtlOnTunnel(t,
			bs.DUT,
			&tunnelCfg{policyName: "policy1",
				protocolType:     "V4Udp",
				nexthopGroupName: nexthopGroupName,
				tos:              0,
				ttl:              20,
				deleteTos:        true,
				deleteTtl:        false,
				index:            "0",
			})

		createFlow(bs.ATETop, "IPv4-GUE-Traffic", "ipv4", trafficDstNetIPv4, 0x80, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, bs.ATETop, capturePorts)
		bs.ATE.OTG().PushConfig(t, bs.ATETop)

		t.Log("Start ATE protocols and verify BGP session")
		bs.ATE.OTG().StartProtocols(t)

		t.Log("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
		t.Log("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

		t.Log("Starting capture")
		cs := startCapture(t, bs.ATE.OTG())

		t.Log("Starting traffic")
		bs.ATE.OTG().StartTraffic(t)

		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)

		t.Log("Stop Capture")
		stopCapture(t, bs.ATE.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, bs.DUT, bs.ATE, bs.ATETop, "IPv4-GUE-Traffic")

		for _, p := range capturePorts {
			capture := mustProcessCapture(t, bs.ATE.OTG(), p)
			validatePackets(t, capture, "ipv4", "", "", 20, 0, true)
		}
	})

	t.Run("RT-3.53.6: IPv6 GUE Encap with explicit TTL", func(t *testing.T) {
		configureTosTtlOnTunnel(t, bs.DUT,
			&tunnelCfg{policyName: "policy1",
				protocolType:     "V6Udp",
				nexthopGroupName: nexthopGroupNameV6,
				tos:              0,
				ttl:              20,
				deleteTos:        true,
				deleteTtl:        false,
				index:            "1",
			})
		createFlow(bs.ATETop, "IPv6-GUE-Traffic", "ipv6", trafficDstNetIPv6, 0x80, 10, false, gueProtocolPort, gueSrcProtocolPort, false)
		enableCapture(t, bs.ATETop, capturePorts)
		bs.ATE.OTG().PushConfig(t, bs.ATETop)
		t.Log("Start ATE protocols and verify BGP session")
		bs.ATE.OTG().StartProtocols(t)

		t.Log("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
		t.Log("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

		t.Log("Starting capture")
		cs := startCapture(t, bs.ATE.OTG())

		t.Log("Starting traffic")
		bs.ATE.OTG().StartTraffic(t)

		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)

		t.Log("Stop Capture")
		stopCapture(t, bs.ATE.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, bs.DUT, bs.ATE, bs.ATETop, "IPv6-GUE-Traffic")

		for _, p := range capturePorts {
			capture := mustProcessCapture(t, bs.ATE.OTG(), p)
			validatePackets(t, capture, "ipv6", "", "", 20, 0, true)
		}
	})

	t.Run("RT-3.53.7: IPv4 traffic GUE encapsulation with explicit ToS and TTL configuration on tunnel", func(t *testing.T) {
		configureTosTtlOnTunnel(t, bs.DUT,
			&tunnelCfg{policyName: "policy1",
				protocolType:     "V4Udp",
				nexthopGroupName: nexthopGroupName,
				tos:              0x60,
				ttl:              20,
				deleteTos:        false,
				deleteTtl:        false,
				index:            "0",
			})
		createFlow(bs.ATETop, "IPv4-GUE-Traffic", "ipv4", trafficDstNetIPv4, 0x60, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, bs.ATETop, capturePorts)
		bs.ATE.OTG().PushConfig(t, bs.ATETop)

		t.Log("Start ATE protocols and verify BGP session")
		bs.ATE.OTG().StartProtocols(t)

		t.Log("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
		t.Log("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)
		t.Log("Starting capture")
		cs := startCapture(t, bs.ATE.OTG())

		t.Log("Starting traffic")
		bs.ATE.OTG().StartTraffic(t)

		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)

		t.Log("Stop Capture")
		stopCapture(t, bs.ATE.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, bs.DUT, bs.ATE, bs.ATETop, "IPv4-GUE-Traffic")

		for _, p := range capturePorts {
			capture := mustProcessCapture(t, bs.ATE.OTG(), p)
			validatePackets(t, capture, "ipv4", "0x60", "", 20, 0, true)
		}
	})

	t.Run("RT-3.53.8: IPv6 traffic GUE encapsulation with explicit ToS and TTL configuration on tunnel", func(t *testing.T) {
		configureTosTtlOnTunnel(t, bs.DUT,
			&tunnelCfg{policyName: "policy1",
				protocolType:     "V6Udp",
				nexthopGroupName: nexthopGroupNameV6,
				tos:              0x60,
				ttl:              20,
				deleteTos:        false,
				deleteTtl:        false,
				index:            "1",
			})
		createFlow(bs.ATETop, "IPv6-GUE-Traffic", "ipv6", trafficDstNetIPv6, 0x60, 10, false, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, bs.ATETop, capturePorts)
		bs.ATE.OTG().PushConfig(t, bs.ATETop)
		t.Log("Start ATE protocols and verify BGP session")
		bs.ATE.OTG().StartProtocols(t)
		t.Log("Starting capture")
		cs := startCapture(t, bs.ATE.OTG())

		t.Log("Starting traffic")
		bs.ATE.OTG().StartTraffic(t)

		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)

		t.Log("Stop Capture")
		stopCapture(t, bs.ATE.OTG(), cs)

		t.Log("Verify traffic packets")
		verifyTraffic(t, bs.DUT, bs.ATE, bs.ATETop, "IPv6-GUE-Traffic")

		for _, p := range capturePorts {
			capture := mustProcessCapture(t, bs.ATE.OTG(), p)
			validatePackets(t, capture, "ipv6", "0x60", "", 20, 0, true)
		}
	})

	t.Run("RT-3.53.9: IPv4 traffic GUE encapsulation to a single 5-tuple tunnel", func(t *testing.T) {
		configureTosTtlOnTunnel(t,
			bs.DUT,
			&tunnelCfg{policyName: "policy1",
				protocolType:     "V4Udp",
				nexthopGroupName: nexthopGroupName,
				tos:              0x60,
				ttl:              20,
				deleteTos:        true,
				deleteTtl:        true,
				index:            "0",
			})
		createFlow(bs.ATETop, "IPv4-GUE-Single-Traffic", "ipv4", trafficDstNetIPv4, 0x80, 10, true, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, bs.ATETop, capturePorts)
		bs.ATE.OTG().PushConfig(t, bs.ATETop)
		t.Log("Start ATE protocols and verify BGP session")
		bs.ATE.OTG().StartProtocols(t)

		t.Log("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
		t.Log("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

		t.Log("Starting capture")
		cs := startCapture(t, bs.ATE.OTG())

		t.Log("Starting traffic")
		bs.ATE.OTG().StartTraffic(t)

		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)

		t.Log("Stop Capture")
		stopCapture(t, bs.ATE.OTG(), cs)

		t.Log("Verify traffic packets")
		activeport := verifySinglePathTraffic(t, bs.ATE, bs.ATETop, "IPv4-GUE-Single-Traffic")

		capture := mustProcessCapture(t, bs.ATE.OTG(), activeport[0])
		validatePackets(t, capture, "ipv4", "0x80", "0x80", 64, 9, true)

	})

	t.Run("RT-3.53.10: IPv6 traffic GUE encapsulation to a single 5-tuple tunnel", func(t *testing.T) {
		configureTosTtlOnTunnel(t, bs.DUT,
			&tunnelCfg{policyName: "policy1",
				protocolType:     "V6Udp",
				nexthopGroupName: nexthopGroupNameV6,
				tos:              0x60,
				ttl:              20,
				deleteTos:        true,
				deleteTtl:        true,
				index:            "1",
			})
		createFlow(bs.ATETop, "IPv6-GUE-Single-Traffic", "ipv6", trafficDstNetIPv6, 0x80, 10, true, gueProtocolPort, gueSrcProtocolPort, false)

		enableCapture(t, bs.ATETop, capturePorts)
		bs.ATE.OTG().PushConfig(t, bs.ATETop)
		t.Log("Start ATE protocols and verify BGP session")
		bs.ATE.OTG().StartProtocols(t)

		t.Log("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
		t.Log("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

		t.Log("Starting capture")
		cs := startCapture(t, bs.ATE.OTG())

		t.Log("Starting traffic")
		bs.ATE.OTG().StartTraffic(t)

		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)

		t.Log("Stop Capture")
		stopCapture(t, bs.ATE.OTG(), cs)

		t.Log("Verify traffic packets")
		activeport := verifySinglePathTraffic(t, bs.ATE, bs.ATETop, "IPv6-GUE-Single-Traffic")

		capture := mustProcessCapture(t, bs.ATE.OTG(), activeport[0])
		validatePackets(t, capture, "ipv6", "0x80", "0x80", 64, 9, true)

	})

	t.Run("RT-3.53.11: IPv4 traffic with TTL=1", func(t *testing.T) {
		createFlow(bs.ATETop, "IPv4-GUE-ttl", "ipv4", trafficDstNetIPv4, 0x80, 1, false, gueProtocolPort, gueSrcProtocolPort, true)
		enableCapture(t, bs.ATETop, []string{"port1"})
		bs.ATE.OTG().PushConfig(t, bs.ATETop)

		t.Log("Start ATE protocols and verify BGP session")
		bs.ATE.OTG().StartProtocols(t)
		t.Log("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
		t.Log("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

		cs := startCapture(t, bs.ATE.OTG())

		t.Log("Starting TTL=1 traffic test")
		bs.ATE.OTG().StartTraffic(t)

		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)

		time.Sleep(20 * time.Second)
		stopCapture(t, bs.ATE.OTG(), cs)
		captureName := mustProcessCapture(t, bs.ATE.OTG(), "port1")

		t.Log("Verify TTL=1 packets are dropped and ICMP is returned")
		verifyTTLDropAndICMP(t, bs.ATE, bs.ATETop, "IPv4-GUE-ttl", captureName)
	})

	t.Run("RT-3.53.12: IPv6 traffic with TTL=1", func(t *testing.T) {
		createFlow(bs.ATETop, "IPv6-GUE-ttl", "ipv6", trafficDstNetIPv6, 0x80, 1, false, gueProtocolPort, gueSrcProtocolPort, true)

		enableCapture(t, bs.ATETop, []string{"port1"})
		bs.ATE.OTG().PushConfig(t, bs.ATETop)

		t.Log("Start ATE protocols and verify BGP session")
		bs.ATE.OTG().StartProtocols(t)
		t.Log("Verify DUT BGP sessions up")
		cfgplugins.VerifyDUTBGPEstablished(t, bs.DUT)
		t.Log("Verify OTG BGP sessions up")
		cfgplugins.VerifyOTGBGPEstablished(t, bs.ATE)

		cs := startCapture(t, bs.ATE.OTG())

		t.Log("Starting TTL=1 traffic test")
		bs.ATE.OTG().StartTraffic(t)

		otgutils.LogPortMetrics(t, bs.ATE.OTG(), bs.ATETop)
		otgutils.LogFlowMetrics(t, bs.ATE.OTG(), bs.ATETop)

		time.Sleep(20 * time.Second)
		stopCapture(t, bs.ATE.OTG(), cs)
		captureName := mustProcessCapture(t, bs.ATE.OTG(), "port1")

		t.Log("Verify TTL=1 packets are dropped and ICMP is returned")
		verifyTTLDropAndICMP(t, bs.ATE, bs.ATETop, "IPv6-GUE-ttl", captureName)
	})
}
