// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package egressnodeforwarding_test

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
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen    = 30
	ipv6PrefixLen    = 126
	srgbName         = "srgb block"
	srgbStartLabel   = 400000
	srgbEndLabel     = 465000
	srgbID           = "99.99.99.99"
	flowV4Name       = "ipv4_explicit_null_flow"
	flowV6Name       = "ipv6_explicit_null_flow"
	mplsLabelV4      = 0
	mplsLabelV6      = 2
	mplsExpBit       = 0
	mplsSBit         = 1
	mplsOuterTTL     = 64
	tcpDstPort       = 443
	udpDstPort       = 443
	packetsPerFlow   = 1000
	trafficFrameSize = 512
	trafficRatePps   = 100
	trafficTimeout   = 30 * time.Second
	ate1SysID        = "640000000001"
	ate2SysID        = "640000000002"
	dutAreaAddress   = "49.0001"
	dutSysID         = "1920.0000.2001"
	isisMetric       = 10
	isisInstance     = "DEFAULT"
)

var (
	atePort1 = &attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		MAC:     "02:01:01:01:01:01",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort1 = &attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = &attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		MAC:     "02:02:02:02:02:02",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = &attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	atePorts = map[string]*attrs.Attributes{
		"port1": atePort1,
		"port2": atePort2,
	}

	dutPorts = map[string]*attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
	}

	// Traffic Source IPs (on ATE Port 1 side, but distinct from interface IPs)
	ateTrafficSrcIPv4 = "198.18.0.1"    // IPV4-SRC1
	ateTrafficSrcIPv6 = "2001:db8:1::1" // IPV6-SRC1

	// Traffic Destination IPs (on ATE Port 2 side, but distinct from interface IPs)
	ateTrafficDstIPv4 = "198.51.100.1"  // IPV4-DST1
	ateTrafficDstIPv6 = "2001:db8:2::1" // IPV6-DST1

	// Static IPs towards ATE Port2
	ateTrafficDstIPv4Net = "198.51.100.0/24" // Static IP for IPV4-DST1 for static route
	ateTrafficDstIPv6Net = "2001:db8:2::/64" // Static IP for IPV6-DST1 for static route

)

type protocolType int

const (
	protocolUnknown protocolType = iota
	protocolIPv4
	protocolIPv6
)

func (pt *protocolType) String() string {
	switch *pt {
	case protocolIPv4:
		return "ipv4"
	case protocolIPv6:
		return "ipv6"
	default:
		return "unsupported protocol type"
	}
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(p1, dutPort1, dut))
	p2 := dut.Port(t, "port2")
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(p2, dutPort2, dut))

	// Configure Network instance type on DUT
	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
	}
}

func configInterfaceDUT(p *ondatra.Port, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i := a.NewOCInterface(p.Name(), dut)
	s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.SetEnabled(true)
	}
	i.GetOrCreateSubinterface(0).GetOrCreateIpv6()

	return i
}

// configureDUTMPLSAndRouting configures MPLS, SRGB, and static routes on the DUT.
func configureDUTMPLSAndRouting(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	batch := &gnmi.SetBatch{}

	// ISIS Configuration

	cfgISIS := cfgplugins.ISISConfigBasic{
		InstanceName: isisInstance,
		AreaAddress:  dutAreaAddress,
		SystemID:     dutSysID,
		Ports:        []*ondatra.Port{dut.Port(t, "port1"), dut.Port(t, "port2")},
	}
	cfgplugins.NewISISBasic(t, batch, dut, cfgISIS)

	mplsCfg := cfgplugins.MPLSSRConfigBasic{
		InstanceName:   isisInstance,
		SrgbName:       srgbName,
		SrgbStartLabel: srgbStartLabel,
		SrgbEndLabel:   srgbEndLabel,
		SrgbID:         srgbID,
	}
	cfgplugins.NewMPLSSRBasic(t, batch, dut, mplsCfg)

	// IPv4 static route to ATE Port2
	mustConfigStaticRoute(t, dut, ateTrafficDstIPv4Net, atePort2.IPv4)

	// IPv6 static route to ATE Port2
	mustConfigStaticRoute(t, dut, ateTrafficDstIPv6Net, atePort2.IPv6)

}

// Congigure Static Routes on DUT
func mustConfigStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string) {
	t.Helper()

	b := &gnmi.SetBatch{}
	if nexthop == "Null0" {
		nexthop = "DROP"
	}
	sV4 := &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          prefix,
		NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
			"0": oc.UnionString(nexthop),
		},
	}
	if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
		t.Fatalf("Failed to configure IPv4 static route: %v", err)
	}
	b.Set(t, dut)
}

// configureATE configures ATE interfaces and traffic flows.
func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {

	otgConfig := gosnappi.NewConfig()

	for portName, portAttrs := range atePorts {
		port := ate.Port(t, portName)
		dutPort := dutPorts[portName]
		portAttrs.AddToOTG(otgConfig, port, dutPort)
	}

	// Configure ISIS

	// Add IS-IS in ATE port1
	isisConfig := []struct {
		deviceName     string
		routerIP       string
		sysID          string
		isisDeviceName string
	}{
		{deviceName: atePort1.Name, routerIP: atePort1.IPv4, sysID: ate1SysID, isisDeviceName: "dev1Isis"},
		{deviceName: atePort2.Name, routerIP: atePort2.IPv4, sysID: ate2SysID, isisDeviceName: "dev2Isis"},
	}

	for _, device := range otgConfig.Devices().Items() {
		for i, isisDevice := range isisConfig {
			if device.Name() == isisDevice.deviceName {
				portisis := device.Isis().SetSystemId(isisDevice.sysID).SetName(isisDevice.isisDeviceName)
				portisis.Basic().SetIpv4TeRouterId(isisDevice.routerIP)
				portisis.Basic().SetHostname(portisis.Name())
				portisis.Basic().SetEnableWideMetric(true)
				portisis.Basic().SetLearnedLspFilter(true)

				// configure Segment Routing
				sr := portisis.SegmentRouting()
				d1rtrCap1 := sr.RouterCapability()
				d1rtrCap1.SetCustomRouterCapId(isisDevice.routerIP)
				d1rtrCap1.SetAlgorithms([]uint32{0})
				d1rtrCap1.SetSBit(gosnappi.IsisRouterCapabilitySBit.FLOOD)
				d1rtrCap1.SetDBit(gosnappi.IsisRouterCapabilityDBit.DOWN)
				srCap := d1rtrCap1.SrCapability()
				srCap.Flags().SetIpv4Mpls(true).SetIpv6Mpls(true)
				srCap.SrgbRanges().Add().SetStartingSid(uint32(srgbStartLabel)).SetRange(uint32(srgbEndLabel))

				devIsisport := portisis.Interfaces().Add().SetEthName(device.Ethernets().Items()[0].Name()).SetName(fmt.Sprintf("devIsisPort%d", i+1)).SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_1_2).SetMetric(isisMetric)
				devIsisport.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)
			}
		}
	}

	// Traffic Flow 1: Ethernet+MPLS+IPv4+Payload
	t.Logf("Configuring ATE Flow: %s", flowV4Name)
	flowV4 := otgConfig.Flows().Add().SetName(flowV4Name)
	flowV4.Metrics().SetEnable(true)
	flowV4.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
	flowV4.Size().SetFixed(trafficFrameSize)
	flowV4.Rate().SetPps(trafficRatePps)
	flowV4.Duration().FixedPackets().SetPackets(packetsPerFlow)

	ethHdrV4 := flowV4.Packet().Add().Ethernet()
	ethHdrV4.Src().SetValue(atePort1.MAC)
	ethHdrV4.Dst().Auto()

	mplsHdrV4 := flowV4.Packet().Add().Mpls()
	mplsHdrV4.Label().SetValue(mplsLabelV4)
	mplsHdrV4.TrafficClass().SetValue(mplsExpBit)
	mplsHdrV4.BottomOfStack().SetValue(mplsSBit)
	mplsHdrV4.TimeToLive().SetValue(mplsOuterTTL)

	ipHdrV4 := flowV4.Packet().Add().Ipv4()
	ipHdrV4.Src().SetValue(ateTrafficSrcIPv4)
	ipHdrV4.Dst().SetValue(ateTrafficDstIPv4)

	tcpHdrV4 := flowV4.Packet().Add().Tcp()
	tcpHdrV4.SrcPort().SetValue(10000 + (packetsPerFlow % 50000))
	tcpHdrV4.DstPort().SetValue(tcpDstPort)

	udpHdrV4 := flowV4.Packet().Add().Udp()
	udpHdrV4.SrcPort().SetValue(10000 + (packetsPerFlow % 50000))
	udpHdrV4.DstPort().SetValue(udpDstPort)

	// Traffic Flow 2: Ethernet+MPLS+IPv6+Payload

	t.Logf("Configuring ATE Flow: %s", flowV6Name)
	flowV6 := otgConfig.Flows().Add().SetName(flowV6Name)
	flowV6.Metrics().SetEnable(true)
	flowV6.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort2.Name + ".IPv6"})
	flowV6.Size().SetFixed(trafficFrameSize)
	flowV6.Rate().SetPps(trafficRatePps)
	flowV6.Duration().FixedPackets().SetPackets(packetsPerFlow)

	ethHdrV6 := flowV6.Packet().Add().Ethernet()
	ethHdrV6.Src().SetValue(atePort1.MAC)
	ethHdrV6.Dst().Auto()

	mplsHdrV6 := flowV6.Packet().Add().Mpls()
	mplsHdrV6.Label().SetValue(mplsLabelV6)
	mplsHdrV6.TrafficClass().SetValue(mplsExpBit)
	mplsHdrV6.BottomOfStack().SetValue(mplsSBit)
	mplsHdrV6.TimeToLive().SetValue(mplsOuterTTL)

	ipHdrV6 := flowV6.Packet().Add().Ipv6()
	ipHdrV6.Src().SetValue(ateTrafficSrcIPv6)
	ipHdrV6.Dst().SetValue(ateTrafficDstIPv6)

	tcpHdrV6 := flowV4.Packet().Add().Tcp()
	tcpHdrV6.SrcPort().SetValue(10000 + (packetsPerFlow % 50000)) // pseudo-random src port > 1024
	tcpHdrV6.DstPort().SetValue(tcpDstPort)

	udpHdrV6 := flowV6.Packet().Add().Udp()
	udpHdrV6.SrcPort().SetValue(20000 + (packetsPerFlow % 40000)) // pseudo-random src port > 1024
	udpHdrV6.DstPort().SetValue(udpDstPort)

	return otgConfig
}

func enableCapture(t *testing.T, config gosnappi.Config, port string) {
	config.Captures().Clear()
	t.Log("Enabling capture on ", port)
	config.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)

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

func validatePackets(t *testing.T, filename string, protocolType protocolType) error {

	mplsPacketCount := uint32(0)

	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		return err
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	for packet := range packetSource.Packets() {
		if protocolType.String() == "ipv4" {
			if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
				ip, _ := ipLayer.(*layers.IPv4)
				if ip.DstIP.Equal(net.ParseIP(ateTrafficDstIPv4)) {
					if mplsLayer := packet.Layer(layers.LayerTypeMPLS); mplsLayer != nil {
						mplsPacketCount++
					}
				}
				if ip.TTL == mplsOuterTTL-1 {
					t.Logf("TTL decremented by 1")
					break
				}
				return fmt.Errorf("unexpected TTL - got: %v, want: %v", ip.TTL, mplsOuterTTL-1)
			}
		} else {
			if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
				ip, _ := ipLayer.(*layers.IPv6)
				if ip.DstIP.Equal(net.ParseIP(ateTrafficDstIPv6)) {
					if mplsLayer := packet.Layer(layers.LayerTypeMPLS); mplsLayer != nil {
						mplsPacketCount++
					}
				}
				if ip.HopLimit == mplsOuterTTL-1 {
					t.Logf("TTL decremented by 1")
					break
				}
				return fmt.Errorf("unexpected TTL - got: %v, want: %v", ip.HopLimit, mplsOuterTTL-1)
			}
		}
	}

	if mplsPacketCount != 0 {
		return fmt.Errorf("mpls label is not popped by the DUT")
	}

	return nil
}

// verifyTrafficValidations runs traffic and validates ATE flow metrics and DUT counters.
func verifyTrafficValidations(t *testing.T, ate *ondatra.ATEDevice, flowName string) (totalTxFromATE, totalRxAtATE uint64, err error) {
	t.Helper()
	otg := ate.OTG()

	txPkts := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().OutPkts().State())
	rxPkts := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).Counters().InPkts().State())

	totalTxFromATE += txPkts
	totalRxAtATE += rxPkts

	if txPkts < uint64(packetsPerFlow) {
		t.Logf("Flow %s: Expected to transmit at least %d packets, but only %d were reported sent.", flowName, packetsPerFlow, txPkts)
	}
	switch {
	case rxPkts < txPkts:
		var lossPct float32
		if txPkts > 0 {
			lossPct = (float32(txPkts-rxPkts) * 100) / float32(txPkts)
		}
		return 0, 0, fmt.Errorf("flow %s: packet loss detected - TxPkts: %d, RxPkts: %d, Loss %%: %.2f", flowName, txPkts, rxPkts, lossPct)
	case txPkts > 0:
		t.Logf("Flow %s: Successfully transmitted %d packets and received %d packets with no loss.", flowName, txPkts, rxPkts)
	case packetsPerFlow > 0: // Expected to send but didn't
		return 0, 0, fmt.Errorf("flow %s: no packets were transmitted for this flow, but %d were expected", flowName, packetsPerFlow)
	}

	return totalTxFromATE, totalRxAtATE, nil

}

func sendTrafficCapture(t *testing.T, ate *ondatra.ATEDevice) {
	t.Logf("Starting capture")
	cs := startCapture(t, ate.OTG())

	t.Logf("Starting traffic on ATE...")
	ate.OTG().StartTraffic(t)
	t.Logf("Waiting for %v for traffic to complete and stats to update...", trafficTimeout)
	time.Sleep(trafficTimeout)
	ate.OTG().StopTraffic(t)
	t.Logf("Traffic stopped.")

	t.Logf("Stop Capture")
	stopCapture(t, ate.OTG(), cs)
}

func captureDUTCounters(t *testing.T, dut *ondatra.DUTDevice) (uint64, uint64) {
	dutP1 := dut.Port(t, "port1")
	dutP2 := dut.Port(t, "port2")

	// Capture initial DUT interface counters for delta calculation.
	dutP1InCounters := gnmi.Get(t, dut, gnmi.OC().Interface(dutP1.Name()).Counters().State()).GetInUnicastPkts()
	dutP2OutCounters := gnmi.Get(t, dut, gnmi.OC().Interface(dutP2.Name()).Counters().State()).GetOutUnicastPkts()

	return dutP1InCounters, dutP2OutCounters
}

func validateDUTCounters(t *testing.T, flowCount int, dutP1InCountersBefore, dutP1InCountersAfter, dutP2OutCountersBefore, dutP2OutCountersAfter, totalTxFromATE, totalRxAtATE uint64) error {
	// DUT Counter Validation (Delta Check)
	inPktsDelta := dutP1InCountersAfter - dutP1InCountersBefore
	outPktsDelta := dutP2OutCountersAfter - dutP2OutCountersBefore

	t.Logf("DUT Port1 InUnicastPkts: Before=%d, After=%d, Delta=%d", dutP1InCountersBefore, dutP1InCountersAfter, inPktsDelta)

	t.Logf("DUT Port2 OutUnicastPkts: Before=%d, After=%d, Delta=%d", dutP2OutCountersBefore, dutP2OutCountersAfter, outPktsDelta)

	t.Logf("Total packets sent by ATE (sum of flows reported by ATE): %d", totalTxFromATE)
	t.Logf("Total packets received by ATE (sum of flows reported by ATE): %d", totalRxAtATE)

	expectedTotalTraffic := uint64(packetsPerFlow) * uint64(flowCount)
	switch {
	case totalTxFromATE > 0:
		// Check if DUT ingress counters reflect ATE Tx.
		// Allow a small difference (e.g., 2%) due to other control plane packets or timing of counter polling.
		if float64(inPktsDelta) < float64(totalTxFromATE)*0.98 {
			return fmt.Errorf("dut port1 InUnicastPkts delta (%d) is significantly less than ATE Tx (%d) - expected approx %d", inPktsDelta, totalTxFromATE, expectedTotalTraffic)
		}
		// Check if DUT egress counters reflect ATE Rx.
		if float64(outPktsDelta) < float64(totalRxAtATE)*0.98 {
			return fmt.Errorf("dut Port2 OutUnicastPkts delta (%d) is significantly less than ATE Rx (%d) - potential drop in DUT. expected approx %d", outPktsDelta, totalRxAtATE, expectedTotalTraffic)
		}
	case expectedTotalTraffic > 0:
		return fmt.Errorf("traffic was not reported as transmitted by ATE flows, but %d total packets were expected", expectedTotalTraffic)
	}

	return nil
}

func TestMPLSExplicitNull(t *testing.T) {
	var err error
	var ateTxCounters, ateRxCounters uint64

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Congigure DUT
	configureDUT(t, dut)

	// Configure DUT MPLS, SRGB, and Static Routing
	configureDUTMPLSAndRouting(t, dut)

	// Configure ATE (Interfaces and Traffic Flows)
	otgConfig := configureATE(t, ate)
	enableCapture(t, otgConfig, "port2")

	ate.OTG().PushConfig(t, otgConfig)
	ate.OTG().StartProtocols(t)

	t.Logf("Waiting for ARP/NDP resolution...")
	otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), otgConfig, "IPv6")

	// Capture the DUT counters before initialising traffic
	dutP1BeforeCounters, dutP2BeforeCounters := captureDUTCounters(t, dut)
	// Start Traffic & capture
	sendTrafficCapture(t, ate)

	capture := mustProcessCapture(t, ate.OTG(), "port2")

	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	// Capture the DUT counters after traffic
	dutP1AfterCounters, dutP2AfterCounters := captureDUTCounters(t, dut)

	flowNames := []struct {
		name     string
		flowType protocolType
	}{
		{
			name:     flowV4Name,
			flowType: protocolIPv4,
		},
		{
			name:     flowV6Name,
			flowType: protocolIPv6,
		},
	}
	for _, flowName := range flowNames {
		t.Run(fmt.Sprintf("ValidateFlow_%s", flowName.name), func(t *testing.T) {
			ateTxCounters, ateRxCounters, err = verifyTrafficValidations(t, ate, flowName.name)
			if err != nil {
				t.Error(err)
			}
			err = validatePackets(t, capture, flowName.flowType)
			if err != nil {
				t.Error(err)
			}
		})
	}

	err = validateDUTCounters(t, len(flowNames), dutP1BeforeCounters, dutP1AfterCounters, dutP2BeforeCounters, dutP2AfterCounters, ateTxCounters, ateRxCounters)
	if err != nil {
		t.Error(err)
	}

	t.Log("MPLS Explicit Null Test Completed")
}
