package mpls_gre_udp_decap_test

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
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
	ipv4              = "IPv4"
	ipv6              = "IPv6"
	gre               = "GRE"
	gue               = "GUE"
	port1             = "port1"
	port2             = "port2"
	mplsLabel         = 100
	trafficFrameSize  = 256
	trafficRatePPS    = 500
	packetCount       = 1000
	captureFilePath   = "/tmp/capture.pcap"
	capturePort       = port2
	trafficPolicyName = "decap-policy"

	innerIPv4DstA   = "10.5.1.1"
	innerIPv4DstB   = "10.5.1.2"
	innerIPv6DstA   = "2001:aa:bb::1"
	innerIPv6DstB   = "2001:aa:bb::2"
	outerIPv4Src    = "20.4.1.1"
	outerIPv6Src    = "2001:f:a:1::0"
	outerIPv4DstA   = "20.5.1.1"
	outerIPv6DstB   = "2001:f:c:e::2"
	outerDstUDPPort = 6635
	outerDSCP       = 26
	outerIPTTL      = 64
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

	grePolicyName = fmt.Sprintf("%s-%s-%s", trafficPolicyName, ipv6, gre)
	guePolicyName = fmt.Sprintf("%s-%s-%s", trafficPolicyName, ipv6, gue)
)

type flowConfig struct {
	innerIPType string
	outerIPType string
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
	otgutils.WaitForARP(t, ate.OTG(), top, ipv4)
	otgutils.WaitForARP(t, ate.OTG(), top, ipv6)
	testCases := []testCase{
		{
			name:      "PF-1.7.1 - MPLS in GRE decapsulation set by gNMI",
			encapType: gre,
			flowConfigs: []flowConfig{
				{
					innerIPType: ipv4,
					outerIPType: ipv4,
					innerIPSrc:  otgPort1.IPv4,
					innerIPDest: innerIPv4DstA,
					outerIPSrc:  outerIPv4Src,
					outerIPDest: outerIPv4DstA,
				},
				{
					innerIPType: ipv6,
					outerIPType: ipv4,
					innerIPSrc:  otgPort1.IPv6,
					innerIPDest: innerIPv6DstA,
					outerIPSrc:  outerIPv4Src,
					outerIPDest: outerIPv4DstA,
				},
			},
		},
		{
			name:      "PF-1.7.2 - MPLS in UDP decapsulation set by gNMI",
			encapType: gue,
			flowConfigs: []flowConfig{
				{
					innerIPType: ipv4,
					outerIPType: ipv6,
					innerIPSrc:  otgPort1.IPv4,
					innerIPDest: innerIPv4DstB,
					outerIPSrc:  outerIPv6Src,
					outerIPDest: outerIPv6DstB,
				},
				{
					innerIPType: ipv6,
					outerIPType: ipv6,
					innerIPSrc:  otgPort1.IPv6,
					innerIPDest: innerIPv6DstB,
					outerIPSrc:  outerIPv6Src,
					outerIPDest: outerIPv6DstB,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := runTest(t, dut, top, tc, ate); err != nil {
				t.Errorf("test %s failed: %v", tc.name, err)
			}
		})
	}
}

func runTest(t *testing.T, dut *ondatra.DUTDevice, config gosnappi.Config, tc testCase, ate *ondatra.ATEDevice) error {
	t.Log("Configuring input policy")
	configureInputPolicy(t, dut, tc)
	var flowErrors []error
	for _, flowConfig := range tc.flowConfigs {
		if err := sendAndValidateTraffic(t, ate, config, tc, flowConfig); err != nil {
			flowErrors = append(flowErrors, err)
		}
	}
	return errors.Join(flowErrors...)
}

func sendAndValidateTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, tc testCase, flowConfig flowConfig) error {
	otg := ate.OTG()
	flowName := fmt.Sprintf("flow-%s-%s-%s", flowConfig.outerIPType, tc.encapType, flowConfig.innerIPType)
	configureFlow(t, config, tc, flowConfig, flowName)
	enableCapture(t, ate, config, []string{capturePort})
	defer clearCapture(t, ate, config)

	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	captureState := startCapture(t, ate)
	otg.StartTraffic(t)
	err := waitForTraffic(t, otg, flowName, trafficTimeout)
	stopCapture(t, ate, captureState)
	if err != nil {
		return err
	}
	otg.StopProtocols(t)
	otgutils.LogFlowMetrics(t, otg, config)
	otgutils.LogPortMetrics(t, otg, config)

	flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State())
	if *flowMetrics.Counters.OutPkts == 0 {
		return errors.New("no packets transmitted")
	}

	if *flowMetrics.Counters.InPkts != packetCount {
		return fmt.Errorf("unexpected number of packets received: got %d, want %d", *flowMetrics.Counters.InPkts, packetCount)
	}

	mustProcessCapture(t, ate, capturePort, flowName)
	if err := verifyReceivedInnerPacket(t, captureFilePath, tc, flowConfig); err != nil {
		return fmt.Errorf("packet validation failed: %w", err)
	}
	return nil
}

func enableCapture(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, otgPortNames []string) {
	t.Helper()
	for _, port := range otgPortNames {
		t.Logf("Enabling capture on %s", port)
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

func mustProcessCapture(t *testing.T, ate *ondatra.ATEDevice, capturePort string, flowName string) {
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

func waitForTraffic(t *testing.T, otg *otg.OTG, flowName string, timeout time.Duration) error {
	transmitPath := gnmi.OTG().Flow(flowName).Transmit().State()
	checkState := func(val *ygnmi.Value[bool]) bool {
		transmitState, present := val.Val()
		return present && !transmitState
	}
	_, ok := gnmi.Watch(t, otg, transmitPath, timeout, checkState).Await(t)
	if !ok {
		return fmt.Errorf("traffic for flow %s did not stop within the timeout of %d", flowName, timeout)
	}
	t.Logf("Traffic for flow %s has stopped", flowName)
	return nil
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
	var policyName string
	switch tc.encapType {
	case gre:
		policyName = grePolicyName
	case gue:
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
		TunnelIP:            outerIPv4DstA,
		Dynamic:             true,
		HasMPLS:             true,
	}
	cfgplugins.DecapGroupConfigGre(t, dut, pf, grePFParams)

	guePFParams := cfgplugins.OcPolicyForwardingParams{
		NetworkInstanceName: defaultNIName,
		InterfaceID:         interfaceName,
		AppliedPolicyName:   guePolicyName,
		GUEPort:             outerDstUDPPort,
		IPType:              ipv6,
		TunnelIP:            outerIPv6DstB,
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

func configureFlow(t *testing.T, config gosnappi.Config, tc testCase, fc flowConfig, flowName string) {
	config.Flows().Clear()
	flow := config.Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{fmt.Sprintf("%s.%s", otgPort1.Name, fc.outerIPType)}).SetRxNames([]string{fmt.Sprintf("%s.%s", otgPort2.Name, fc.outerIPType)})
	flow.Size().SetFixed(trafficFrameSize)
	flow.Rate().SetPps(trafficRatePPS)
	flow.Duration().SetFixedPackets(gosnappi.NewFlowFixedPackets().SetPackets(packetCount))

	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(otgPort1.MAC)

	switch fc.innerIPType {
	case ipv4:
		innerIPv4 := flow.Packet().Add().Ipv4()
		innerIPv4.Src().SetValue(fc.innerIPSrc)
		innerIPv4.Dst().SetValue(fc.innerIPDest)
	case ipv6:
		innerIPv6 := flow.Packet().Add().Ipv6()
		innerIPv6.Src().SetValue(fc.innerIPSrc)
		innerIPv6.Dst().SetValue(fc.innerIPDest)
	}

	mpls := flow.Packet().Add().Mpls()
	mpls.Label().SetValue(mplsLabel)

	switch tc.encapType {
	case gre:
		flow.Packet().Add().Gre()
	case gue:
		udp := flow.Packet().Add().Udp()
		udp.DstPort().SetValue(outerDstUDPPort)
	default:
		t.Errorf("invalid encap type %s", tc.encapType)
	}

	switch fc.outerIPType {
	case ipv4:
		outerIPv4 := flow.Packet().Add().Ipv4()
		outerIPv4.Src().SetValue(fc.outerIPSrc)
		outerIPv4.Dst().SetValue(fc.outerIPDest)
		outerIPv4.Priority().Dscp().Phb().SetValue(outerDSCP)
		outerIPv4.TimeToLive().SetValue(outerIPTTL)
	case ipv6:
		outerIPv6 := flow.Packet().Add().Ipv6()
		outerIPv6.Src().SetValue(fc.outerIPSrc)
		outerIPv6.Dst().SetValue(fc.outerIPDest)
		outerIPv6.TrafficClass().SetValue(outerDSCP)
		outerIPv6.HopLimit().SetValue(outerIPTTL)
	}
}

func verifyReceivedInnerPacket(t *testing.T, captureFilename string, tc testCase, fc flowConfig) error {
	if captureFilename == "" {
		return fmt.Errorf("no capture file provided for inner packet verification for testcase %s", tc.name)
	}

	handle, err := pcap.OpenOffline(captureFilename)
	if err != nil {
		return fmt.Errorf("failed to open pcap file: %v", err)
	}
	defer handle.Close()

	var baseIPLayer, icmpLayer gopacket.LayerType
	switch fc.innerIPType {
	case ipv4:
		baseIPLayer = layers.LayerTypeIPv4
		icmpLayer = layers.LayerTypeICMPv4
	case ipv6:
		baseIPLayer = layers.LayerTypeIPv6
		icmpLayer = layers.LayerTypeICMPv6
	default:
		return fmt.Errorf("unknown IP type %s in testcase %s", fc.innerIPType, tc.name)
	}

	srcIPAddr := net.ParseIP(fc.innerIPSrc)
	destIPAddr := net.ParseIP(fc.innerIPDest)

	var capturePacketCount int
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	var validationErrs []error
	for packet := range packetSource.Packets() {
		if packet.Layer(icmpLayer) != nil {
			t.Log("Skipping ICMP packet")
			continue
		}

		if packet.Layer(baseIPLayer) == nil {
			t.Logf("Skipping non %s packet", fc.innerIPType)
			continue
		}

		capturePacketCount++

		mplsInnerLayer := packet.Layer(layers.LayerTypeMPLS)
		mplsPacket, ok := mplsInnerLayer.(*layers.MPLS)
		if !ok || mplsPacket == nil {
			validationErrs = append(validationErrs, fmt.Errorf("MPLS layer not found for packet %d", capturePacketCount))
			continue
		}
		if mplsPacket.Label != mplsLabel {
			validationErrs = append(validationErrs, fmt.Errorf("MPLS inner packet %d: got label %d, want %d", capturePacketCount, mplsPacket.Label, mplsLabel))
		}

		switch fc.innerIPType {
		case ipv4:
			ipInnerLayer := packet.Layer(layers.LayerTypeIPv4)
			if ipInnerLayer == nil {
				validationErrs = append(validationErrs, fmt.Errorf("inner IPv4 layer not found for packet %d", capturePacketCount))
				continue
			}
			ipInnerPacket, ok := ipInnerLayer.(*layers.IPv4)
			if !ok || ipInnerPacket == nil {
				validationErrs = append(validationErrs, fmt.Errorf("unable to extract inner IPv4 layer for packet %d", capturePacketCount))
				continue
			}

			if !ipInnerPacket.SrcIP.Equal(srcIPAddr) {
				validationErrs = append(validationErrs, fmt.Errorf("IPv4 inner packet %d: got SrcIP %s, want %s", capturePacketCount, ipInnerPacket.SrcIP.String(), srcIPAddr.String()))
			}
			if !ipInnerPacket.DstIP.Equal(destIPAddr) {
				validationErrs = append(validationErrs, fmt.Errorf("IPv4 inner packet %d: got DstIP %s, want %s", capturePacketCount, ipInnerPacket.DstIP.String(), destIPAddr.String()))
			}
		case ipv6:
			ipInnerLayer := packet.Layer(layers.LayerTypeIPv6)
			if ipInnerLayer == nil {
				validationErrs = append(validationErrs, fmt.Errorf("inner IPv6 layer not found for packet %d", capturePacketCount))
				continue
			}
			ipInnerPacket, ok := ipInnerLayer.(*layers.IPv6)
			if !ok || ipInnerPacket == nil {
				validationErrs = append(validationErrs, fmt.Errorf("unable to extract inner IPv6 layer for packet %d", capturePacketCount))
				continue
			}
			if !ipInnerPacket.SrcIP.Equal(srcIPAddr) {
				validationErrs = append(validationErrs, fmt.Errorf("IPv6 inner packet %d: got SrcIP %s, want %s", capturePacketCount, ipInnerPacket.SrcIP.String(), srcIPAddr.String()))
			}
			if !ipInnerPacket.DstIP.Equal(destIPAddr) {
				validationErrs = append(validationErrs, fmt.Errorf("IPv6 inner packet %d: got DstIP %s, want %s", capturePacketCount, ipInnerPacket.DstIP.String(), destIPAddr.String()))
			}
		}
	}
	if capturePacketCount != packetCount {
		validationErrs = append(validationErrs, fmt.Errorf("expected %d %s decapsulated packets with inner %s packet, got %d", packetCount, tc.encapType, fc.innerIPType, capturePacketCount))
	} else {
		t.Logf("Found %d %s decapsulated packets with inner %s packet", capturePacketCount, tc.encapType, fc.innerIPType)
	}
	return errors.Join(validationErrs...)
}

func configureStaticRoutes(t *testing.T, dut *ondatra.DUTDevice) {
	for index, destination := range []string{innerIPv4DstA, innerIPv4DstB} {
		mustConfigStaticRoute(t, dut, destination+"/32", otgPort2.IPv4, strconv.Itoa(index))
	}
	for index, destination := range []string{innerIPv6DstA, innerIPv6DstB} {
		mustConfigStaticRoute(t, dut, destination+"/128", otgPort2.IPv6, strconv.Itoa(index))
	}
}

func mustConfigStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string, index string) {
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
