package mpls_gre_udp_decap_test

import (
	"fmt"
	"net"
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
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	trafficTimeout    = 10 * time.Second
	IPv4              = "IPv4"
	IPv6              = "IPv6"
	Gre               = "GRE"
	Gue               = "GUE"
	port1             = "port1"
	port2             = "port2"
	mplsLabel         = 100
	trafficFrameSize  = 256
	trafficRatePps    = 500
	noOfPackets       = 1000
	captureFilePath   = "/tmp/capture.pcap"
	capturePort       = port2
	trafficPolicyName = "decap-policy"

	inner_ipv6_dst_A = "2001:aa:bb::1"
	inner_ipv6_dst_B = "2001:aa:bb::2"
	//inner_ipv6_default = "::/0"
	ipv4_inner_dst_A = "10.5.1.1"
	ipv4_inner_dst_B = "10.5.1.2"
	//ipv4_inner_default = "0.0.0.0/0"
	outer_ipv4_src = "20.4.1.1"
	outer_ipv6_src = "2001:f:a:1::0"
	//outer_ipv6_dst_A = "2001:f:c:e::1"
	outer_ipv4_dst_A = "20.5.1.1"
	outer_ipv6_dst_B = "2001:f:c:e::2"
	//outer_ipv6_dst_def = "2001:1:1:1::0"
	outer_dst_udp_port = 6635
	outer_dscp         = 26
	outer_ip_ttl       = 64
)

var (
	// DUT ports
	dutPort1 = attrs.Attributes{
		Name:    port1,
		Desc:    "Dut port 1",
		IPv4:    "192.168.1.1",
		IPv4Len: 30,
		IPv6:    "2001:DB8::1",
		IPv6Len: 126,
	}

	dutPort2 = attrs.Attributes{
		Name:    port2,
		Desc:    "Dut port 2",
		IPv4:    "192.168.1.5",
		IPv4Len: 30,
		IPv6:    "2001:DB8::5",
		IPv6Len: 126,
	}

	// ATE ports
	otgPort1 = attrs.Attributes{
		Name:    port1,
		Desc:    "Otg port 1",
		MAC:     "00:01:12:00:00:01",
		IPv4:    "192.168.1.2",
		IPv4Len: 30,
		IPv6:    "2001:DB8::2",
		IPv6Len: 126,
	}

	otgPort2 = attrs.Attributes{
		Name:    port2,
		Desc:    "Otg port 2",
		MAC:     "00:01:12:00:00:02",
		IPv4:    "192.168.1.6",
		IPv4Len: 30,
		IPv6:    "2001:DB8::6",
		IPv6Len: 126,
	}

	grePolicyName = fmt.Sprintf("%s-%s-%s", trafficPolicyName, IPv6, Gre)
	guePolicyName = fmt.Sprintf("%s-%s-%s", trafficPolicyName, IPv6, Gue)
)

type flowConfig struct {
	innerIpType string
	outerIpType string
	innerIPSrc  string
	innerIPDest string
	outerIPSrc  string
	outerIPDest string
}

type testCase struct {
	name        string
	encapType   string
	flowConfigs []flowConfig
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// PF-1.7.1: MPLS in GRE decapsulation set by gNMI
func TestMPLSGREUDPDecap(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	configureDUT(t, dut)

	ap1 := ate.Port(t, port1)
	ap2 := ate.Port(t, port2)

	otgPort1.AddToOTG(top, ap1, &dutPort1)
	otgPort2.AddToOTG(top, ap2, &dutPort2)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, IPv4)
	otgutils.WaitForARP(t, ate.OTG(), top, IPv6)
	testCases := []testCase{
		{
			name:      "PF-1.7.1 - MPLS in GRE decapsulation set by gNMI",
			encapType: Gre,
			flowConfigs: []flowConfig{
				{
					innerIpType: IPv4,
					outerIpType: IPv4,
					innerIPSrc:  otgPort1.IPv4,
					innerIPDest: ipv4_inner_dst_A,
					outerIPSrc:  outer_ipv4_src,
					outerIPDest: outer_ipv4_dst_A,
				},
				{
					innerIpType: IPv6,
					outerIpType: IPv4,
					innerIPSrc:  otgPort1.IPv6,
					innerIPDest: inner_ipv6_dst_A,
					outerIPSrc:  outer_ipv4_src,
					outerIPDest: outer_ipv4_dst_A,
				},
			},
		},
		{
			name:      "PF-1.7.2 - MPLS in UDP decapsulation set by gNMI",
			encapType: Gue,
			flowConfigs: []flowConfig{
				{
					innerIpType: IPv4,
					outerIpType: IPv6,
					innerIPSrc:  otgPort1.IPv4,
					innerIPDest: ipv4_inner_dst_B,
					outerIPSrc:  outer_ipv6_src,
					outerIPDest: outer_ipv6_dst_B,
				},
				{
					innerIpType: IPv6,
					outerIpType: IPv6,
					innerIPSrc:  otgPort1.IPv6,
					innerIPDest: inner_ipv6_dst_B,
					outerIPSrc:  outer_ipv6_src,
					outerIPDest: outer_ipv6_dst_B,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runTest(t, dut, top, tc, ate)
		})
	}
}

func runTest(t *testing.T, dut *ondatra.DUTDevice, config gosnappi.Config, tc testCase, ate *ondatra.ATEDevice) {
	t.Log("Configuring input policy")
	configureInputPolicy(t, dut, tc)
	for _, flowConfig := range tc.flowConfigs {
		sendAndValidateTraffic(t, ate, config, tc, flowConfig)
	}
}

func sendAndValidateTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, tc testCase, flowConfig flowConfig) {
	otg := ate.OTG()
	var captureState gosnappi.ControlState
	flowName := fmt.Sprintf("flow-%s-%s-%s", flowConfig.outerIpType, tc.encapType, flowConfig.innerIpType)
	configureFlow(t, &config, tc, flowConfig, flowName)
	enableCapture(t, ate, config, []string{capturePort})
	defer clearCapture(t, ate, config)

	otg.PushConfig(t, config)
	otg.StartProtocols(t)

	captureState = startCapture(t, ate)
	otg.StartTraffic(t)
	waitForTraffic(t, otg, flowName, trafficTimeout)
	otg.StopProtocols(t)

	stopCapture(t, ate, captureState)
	otgutils.LogFlowMetrics(t, otg, config)
	otgutils.LogPortMetrics(t, otg, config)

	flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
	if *flowMetrics.Counters.OutPkts == 0 {
		t.Errorf("No packets transmitted")
		return
	}

	if *flowMetrics.Counters.InPkts != noOfPackets {
		t.Errorf("Unexpected number of packets received: got %d, want %d", *flowMetrics.Counters.InPkts, noOfPackets)
	}

	getCapture(t, ate, capturePort, flowName)
	verifyReceivedInnerPacket(t, captureFilePath, tc, flowConfig)
}

func enableCapture(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, otgPortNames []string) {
	t.Helper()
	for _, port := range otgPortNames {
		t.Log("Enabling capture on ", port)
		topo.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}
	ate.OTG().PushConfig(t, topo)
}

func clearCapture(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config) {
	t.Helper()
	topo.Captures().Clear()
	ate.OTG().PushConfig(t, topo)
}

func startCapture(t *testing.T, ate *ondatra.ATEDevice) gosnappi.ControlState {
	t.Helper()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	ate.OTG().SetControlState(t, cs)
	return cs
}

func getCapture(t *testing.T, ate *ondatra.ATEDevice, capturePort string, flowName string) {
	otg := ate.OTG()
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(capturePort))
	if len(bytes) == 0 {
		t.Fatalf("Empty capture received for flow %s on port %s", flowName, capturePort)
	}
	f, err := os.Create(captureFilePath)
	if err != nil {
		t.Fatalf("Could not create temporary pcap file: %v\n", err)
	}
	defer f.Close()
	if _, err := f.Write(bytes); err != nil {
		t.Fatalf("Could not write bytes to pcap file: %v\n", err)
	}
}

func stopCapture(t *testing.T, ate *ondatra.ATEDevice, cs gosnappi.ControlState) {
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	ate.OTG().SetControlState(t, cs)
	time.Sleep(5 * time.Second)
}

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) {
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	_, ok := gnmi.Watch(t, otg, transmitPath, timeout, func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}).Await(t)

	if !ok {
		t.Errorf("Traffic for flow %s did not stop within the timeout of %d", flowName, timeout)
	} else {
		t.Logf("Traffic for flow %s has stopped", flowName)
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dp1 := dut.Port(t, port1)
	dp2 := dut.Port(t, port2)

	t.Log("Configuring Interfaces")
	configureDUTPort(t, dut, &dutPort1, dp1)
	configureDUTPort(t, dut, &dutPort2, dp2)

	t.Log("Configuring routes")
	configureStaticRoutes(t, dut)

	t.Log("Configuring decap policy forwarding")
	configureDecapPolicyForwarding(t, dut, dp1.Name())
}

func configureInputPolicy(t *testing.T, dut *ondatra.DUTDevice, tc testCase) {
	defaultNIName := deviations.DefaultNetworkInstance(dut)
	interfaceName := dut.Port(t, port1).Name()
	policyName := ""
	switch tc.encapType {
	case Gre:
		policyName = grePolicyName
	case Gue:
		policyName = guePolicyName
	}
	_, _, pf := cfgplugins.SetupPolicyForwardingInfraOC(defaultNIName)
	ocPFParams := cfgplugins.OcPolicyForwardingParams{
		Dynamic:           true,
		InterfaceID:       interfaceName,
		AppliedPolicyName: policyName,
	}
	cfgplugins.InterfacePolicyForwardingConfig(t, dut, nil, "", pf, ocPFParams)
}

func configureDecapPolicyForwarding(t *testing.T, dut *ondatra.DUTDevice, interfaceName string) {
	defaultNIName := deviations.DefaultNetworkInstance(dut)
	_, ni, pf := cfgplugins.SetupPolicyForwardingInfraOC(defaultNIName)

	grePFParams := cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: defaultNIName,
		InterfaceID:         interfaceName,
		AppliedPolicyName:   grePolicyName,
		TunnelIP:            outer_ipv4_dst_A,
		Dynamic:             true,
		HasMPLS:             true,
	}
	cfgplugins.DecapGroupConfigGre(t, dut, pf, grePFParams)

	guePFParams := cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: defaultNIName,
		InterfaceID:         interfaceName,
		AppliedPolicyName:   guePolicyName,
		GUEPort:             outer_dst_udp_port,
		IPType:              IPv6,
		TunnelIP:            outer_ipv6_dst_B,
		Dynamic:             true,
		HasMPLS:             true,
	}
	cfgplugins.DecapGroupConfigGue(t, dut, pf, guePFParams)

	if !deviations.PolicyForwardingOCUnsupported(dut) {
		cfgplugins.PushPolicyForwardingConfig(t, dut, ni)
	}
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
	i4.Enabled = ygot.Bool(true)
	a := i4.GetOrCreateAddress(attrs.IPv4)
	a.PrefixLength = ygot.Uint8(attrs.IPv4Len)

	i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	i6.Enabled = ygot.Bool(true)
	a6 := i6.GetOrCreateAddress(attrs.IPv6)
	a6.PrefixLength = ygot.Uint8(attrs.IPv6Len)

	gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), i)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
		t.Logf("DUT %s %s %s requires explicit interface in default VRF deviation ", dut.Vendor(), dut.Model(), dut.Version())
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p)
	}
}

func configureFlow(t *testing.T, config *gosnappi.Config, tc testCase, fc flowConfig, flowName string) {
	(*config).Flows().Clear()
	flow := (*config).Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.%s", otgPort1.Name, fc.outerIpType)}).SetRxNames([]string{fmt.Sprintf("%s.%s", otgPort2.Name, fc.outerIpType)})
	flow.Size().SetFixed(trafficFrameSize)
	flow.Rate().SetPps(trafficRatePps)
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(noOfPackets))

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(otgPort1.MAC)

	switch fc.innerIpType {
	case IPv4:
		innerIPv4 := flow.Packet().Add().Ipv4()
		innerIPv4.Src().SetValue(fc.innerIPSrc)
		innerIPv4.Dst().SetValue(fc.innerIPDest)
	case IPv6:
		innerIPv6 := flow.Packet().Add().Ipv6()
		innerIPv6.Src().SetValue(fc.innerIPSrc)
		innerIPv6.Dst().SetValue(fc.innerIPDest)
	}

	mpls := flow.Packet().Add().Mpls()
	mpls.Label().SetValue(mplsLabel)

	switch tc.encapType {
	case Gre:
		flow.Packet().Add().Gre()
	case Gue:
		udp := flow.Packet().Add().Udp()
		udp.DstPort().SetValue(outer_dst_udp_port)
	default:
		t.Errorf("Invalid encap type %s", tc.encapType)
	}

	switch fc.outerIpType {
	case IPv4:
		outerIPv4 := flow.Packet().Add().Ipv4()
		outerIPv4.Src().SetValue(fc.outerIPSrc)
		outerIPv4.Dst().SetValue(fc.outerIPDest)
		outerIPv4.Priority().Dscp().Phb().SetValue(outer_dscp)
		outerIPv4.TimeToLive().SetValue(outer_ip_ttl)
	case IPv6:
		outerIPv6 := flow.Packet().Add().Ipv6()
		outerIPv6.Src().SetValue(fc.outerIPSrc)
		outerIPv6.Dst().SetValue(fc.outerIPDest)
		outerIPv6.TrafficClass().SetValue(outer_dscp)
		outerIPv6.HopLimit().SetValue(outer_ip_ttl)
	}
}

func verifyReceivedInnerPacket(t *testing.T, captureFilename string, tc testCase, fc flowConfig) {
	if captureFilename == "" {
		t.Errorf("No capture file provided for inner packet verification for testcase %s", tc.name)
		return
	}

	handle, err := pcap.OpenOffline(captureFilename)
	if err != nil {
		t.Errorf("Failed to open pcap file: %v", err)
		return
	}
	defer handle.Close()

	var baseIpLayer, icmpLayer gopacket.LayerType
	switch fc.innerIpType {
	case IPv4:
		baseIpLayer = layers.LayerTypeIPv4
		icmpLayer = layers.LayerTypeICMPv4
	case IPv6:
		baseIpLayer = layers.LayerTypeIPv6
		icmpLayer = layers.LayerTypeICMPv6
	default:
		t.Errorf("Unknown IP type %s in testcase %s", fc.innerIpType, tc.name)
		return
	}

	srcIpAddr := net.ParseIP(fc.innerIPSrc)
	destIpAddr := net.ParseIP(fc.innerIPDest)

	packetCount := 0
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if packet.Layer(icmpLayer) != nil {
			t.Logf("Skipping ICMP packet")
			continue
		}

		if packet.Layer(baseIpLayer) == nil {
			t.Logf("Skipping non %s packet", fc.innerIpType)
			continue
		}

		packetCount++

		mplsInnerLayer := packet.Layer(layers.LayerTypeMPLS)
		mplsPacket, ok := mplsInnerLayer.(*layers.MPLS)
		if !ok || mplsPacket == nil {
			t.Errorf("MPLS layer not found for packet %d", packetCount)
			continue
		}
		if mplsPacket.Label != mplsLabel {
			t.Errorf("MPLS inner packet %d: got label %d, want %d", packetCount, mplsPacket.Label, mplsLabel)
		}

		switch fc.innerIpType {
		case IPv4:
			ipInnerLayer := packet.Layer(layers.LayerTypeIPv4)
			if ipInnerLayer == nil {
				t.Errorf("Inner IPv4 layer not found for packet %d", packetCount)
				continue
			}
			ipInnerPacket, ok := ipInnerLayer.(*layers.IPv4)
			if !ok || ipInnerPacket == nil {
				t.Errorf("Unable to extract inner IPv4 layer for packet %d", packetCount)
				continue
			}

			if !ipInnerPacket.SrcIP.Equal(srcIpAddr) {
				t.Errorf("IPv4 inner packet %d: got SrcIP %s, want %s", packetCount, ipInnerPacket.SrcIP.String(), srcIpAddr.String())
			}
			if !ipInnerPacket.DstIP.Equal(destIpAddr) {
				t.Errorf("IPv4 inner packet %d: got DstIP %s, want %s", packetCount, ipInnerPacket.DstIP.String(), destIpAddr.String())
			}
		case IPv6:
			ipInnerLayer := packet.Layer(layers.LayerTypeIPv6)
			if ipInnerLayer == nil {
				t.Errorf("Inner IPv6 layer not found for packet %d", packetCount)
				continue
			}
			ipInnerPacket, ok := ipInnerLayer.(*layers.IPv6)
			if !ok || ipInnerPacket == nil {
				t.Errorf("Unable to extract inner IPv6 layer for packet %d", packetCount)
				continue
			}
			if !ipInnerPacket.SrcIP.Equal(srcIpAddr) {
				t.Errorf("IPv6 inner packet %d: got SrcIP %s, want %s", packetCount, ipInnerPacket.SrcIP.String(), srcIpAddr.String())
			}
			if !ipInnerPacket.DstIP.Equal(destIpAddr) {
				t.Errorf("IPv6 inner packet %d: got DstIP %s, want %s", packetCount, ipInnerPacket.DstIP.String(), destIpAddr.String())
			}
		}
	}
	if packetCount != noOfPackets {
		t.Errorf("Expected %d %s decapsulated packets with inner %s packet, got %d", noOfPackets, tc.encapType, fc.innerIpType, packetCount)
	} else {
		t.Logf("Found %d %s decapsulated packets with inner %s packet", packetCount, tc.encapType, fc.innerIpType)
	}
}

func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	for index, destination := range []string{ipv4_inner_dst_A, ipv4_inner_dst_B} {
		configStaticRoute(t, dut, fmt.Sprintf("%s/32", destination), otgPort2.IPv4, fmt.Sprintf("%d", index))
	}
	for index, destination := range []string{inner_ipv6_dst_A, inner_ipv6_dst_B} {
		configStaticRoute(t, dut, fmt.Sprintf("%s/128", destination), otgPort2.IPv6, fmt.Sprintf("%d", index))
	}
}

func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string, index string) {
	b := &gnmi.SetBatch{}
	if nexthop == "Null0" {
		nexthop = "DROP"
	}
	routeCfg := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          prefix,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			index: oc.UnionString(nexthop),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, routeCfg, dut); err != nil {
		t.Fatalf("Failed to configure static route: %v", err)
	}
	b.Set(t, dut)
}
