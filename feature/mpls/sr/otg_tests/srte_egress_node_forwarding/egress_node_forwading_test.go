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
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
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
	srgbId           = "99.99.99.99"
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
	ate1SysId        = "640000000001"
	ate2SysId        = "640000000002"
	dutAreaAddress   = "49.0001"
	dutSysId         = "1920.0000.2001"
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
	ateTrafficDstIPv4Net = "198.51.100.0/24" //Static IP for IPV4-DST1 for static route
	ateTrafficDstIPv6Net = "2001:db8:2::/64" //Static IP for IPV6-DST1 for static route

)

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
		s4.Enabled = ygot.Bool(true)
	}
	i.GetOrCreateSubinterface(0).GetOrCreateIpv6()

	return i
}

func addISISOC(areaAddress, sysID, ifaceName1 string, ifaceName2 string, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := inst.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "DEFAULT")
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		glob.Instance = ygot.String("DEFAULT")
	}
	glob.Net = []string{fmt.Sprintf("%v.%v.00", areaAddress, sysID)}
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	// Configure ISIS enabled flag at level
	if deviations.ISISLevelEnabled(dut) {
		level.Enabled = ygot.Bool(true)
	}
	intf := isis.GetOrCreateInterface(ifaceName1)
	intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	intf.Enabled = ygot.Bool(true)
	// Configure ISIS level at global mode if true else at interface mode
	if deviations.ISISInterfaceLevel1DisableRequired(dut) {
		intf.GetOrCreateLevel(1).Enabled = ygot.Bool(false)
	} else {
		intf.GetOrCreateLevel(2).Enabled = ygot.Bool(true)
	}
	glob.LevelCapability = oc.Isis_LevelType_LEVEL_2
	// Configure ISIS enable flag at interface level
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	if deviations.ISISInterfaceAfiUnsupported(dut) {
		intf.Af = nil
	}

	intf2 := isis.GetOrCreateInterface(ifaceName2)
	intf2.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	intf2.Enabled = ygot.Bool(true)
	// Configure ISIS level at global mode if true else at interface mode
	if deviations.ISISInterfaceLevel1DisableRequired(dut) {
		intf2.GetOrCreateLevel(1).Enabled = ygot.Bool(false)
	} else {
		intf2.GetOrCreateLevel(2).Enabled = ygot.Bool(true)
	}
	glob.LevelCapability = oc.Isis_LevelType_LEVEL_1_2
	// Configure ISIS enable flag at interface level
	intf2.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf2.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	if deviations.ISISInterfaceAfiUnsupported(dut) {
		intf2.Af = nil
	}

	return prot
}

// configureDUTMPLSAndRouting configures MPLS, SRGB, and static routes on the DUT.
func configureDUTMPLSAndRouting(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	niName := deviations.DefaultNetworkInstance(dut)
	dc := gnmi.OC()

	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(niName)

	dutConfPath := dc.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "DEFAULT")
	dutConf := addISISOC(dutAreaAddress, dutSysId, dut.Port(t, "port1").Name(), dut.Port(t, "port2").Name(), dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)

	if deviations.IsisSrgbSrlbUnsupported(dut) {
		configureGlobalMPLS(t, dut)
	} else {
		mpls := ni.GetOrCreateMpls()
		mplsGlobal := mpls.GetOrCreateGlobal()

		rlb := mplsGlobal.GetOrCreateReservedLabelBlock(srgbName)

		rlb.SetLowerBound(oc.UnionUint32(srgbStartLabel))
		rlb.SetUpperBound(oc.UnionUint32(srgbEndLabel))

		sr := ni.GetOrCreateSegmentRouting()
		srgbConfig := sr.GetOrCreateSrgb(srgbName)
		srgbConfig.SetMplsLabelBlocks([]string{srgbName})
		srgbConfig.LocalId = ygot.String(srgbId)

		t.Logf("Pushing DUT MPLS & SRGB configurations...")
		gnmi.Update(t, dut, dc.NetworkInstance(niName).Mpls().Config(), mpls)
		gnmi.Update(t, dut, dc.NetworkInstance(niName).SegmentRouting().Config(), sr)
	}

	// IPv4 static route to ATE Port2
	configStaticRoute(t, dut, ateTrafficDstIPv4Net, atePort2.IPv4)

	// IPv6 static route to ATE Port2
	configStaticRoute(t, dut, ateTrafficDstIPv6Net, atePort2.IPv6)

}

func configureGlobalMPLS(t *testing.T, dut *ondatra.DUTDevice) {
	cliConfig := fmt.Sprintf(`
    mpls ip
	mpls label range isis-sr %v %v
		`, srgbStartLabel, srgbEndLabel)
	helpers.GnmiCLIConfig(t, dut, cliConfig)
}

// Congigure Static Routes on DUT
func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string) {
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
	devices := otgConfig.Devices().Items()
	port1isis := devices[0].Isis().SetSystemId(ate1SysId).SetName("dev1Isis")

	// port 1 device 1 isis basic
	port1isis.Basic().SetIpv4TeRouterId(atePort1.IPv4)
	port1isis.Basic().SetHostname(port1isis.Name())
	port1isis.Basic().SetEnableWideMetric(true)
	port1isis.Basic().SetLearnedLspFilter(true)

	// configure Segment Routing in ATEport1

	sr := port1isis.SegmentRouting()
	d1rtrCap1 := sr.RouterCapability()
	d1rtrCap1.SetCustomRouterCapId(atePort1.IPv4)
	d1rtrCap1.SetAlgorithms([]uint32{0})
	d1rtrCap1.SetSBit(gosnappi.IsisRouterCapabilitySBit.FLOOD)
	d1rtrCap1.SetDBit(gosnappi.IsisRouterCapabilityDBit.DOWN)
	srCap := d1rtrCap1.SrCapability()
	srCap.Flags().SetIpv4Mpls(true).SetIpv6Mpls(true)
	srCap.SrgbRanges().Add().SetStartingSid(uint32(srgbStartLabel)).SetRange(uint32(srgbEndLabel))

	devIsisport1 := port1isis.Interfaces().Add().SetEthName(devices[0].Ethernets().Items()[0].Name()).
		SetName("devIsisPort1").SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_1_2).SetMetric(10)

	devIsisport1.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	// Add IS-IS in ATE port2
	port2isis := otgConfig.Devices().Items()[1].Isis().SetSystemId(ate2SysId).SetName("dev2Isis")

	port2isis.Basic().SetIpv4TeRouterId(atePort2.IPv4)
	port2isis.Basic().SetHostname(port2isis.Name())
	port2isis.Basic().SetEnableWideMetric(true)
	port2isis.Basic().SetLearnedLspFilter(true)

	// configure Segment Routing in ATEport2

	sr1 := port2isis.SegmentRouting()
	d2rtrCap1 := sr1.RouterCapability()
	d2rtrCap1.SetCustomRouterCapId(atePort2.IPv4)
	d2rtrCap1.SetAlgorithms([]uint32{0})
	d2rtrCap1.SetSBit(gosnappi.IsisRouterCapabilitySBit.FLOOD)
	d2rtrCap1.SetDBit(gosnappi.IsisRouterCapabilityDBit.DOWN)
	srCap1 := d2rtrCap1.SrCapability()
	srCap1.Flags().SetIpv4Mpls(true).SetIpv6Mpls(true)
	srCap1.SrgbRanges().Add().SetStartingSid(uint32(srgbStartLabel)).SetRange(uint32(srgbEndLabel))

	devIsisport2 := port2isis.Interfaces().Add().SetEthName(devices[1].Ethernets().Items()[0].Name()).
		SetName("devIsisPort2").SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_1_2).SetMetric(10)

	devIsisport2.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

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
	ethHdrV4.Dst().Auto() // MAC of DUT's ingress port

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

func processCapture(t *testing.T, otg *otg.OTG, port string) string {
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	time.Sleep(30 * time.Second)
	capture, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := capture.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	defer capture.Close()

	return capture.Name()
}

func validatePackets(t *testing.T, filename string, protocolType string) error {

	mplsPacketCount := int32(0)

	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		return err
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	for packet := range packetSource.Packets() {
		if protocolType == "ipv4" {
			if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
				ip, _ := ipLayer.(*layers.IPv4)
				if ip.DstIP.Equal(net.ParseIP(ateTrafficDstIPv4)) {
					if mplsLayer := packet.Layer(layers.LayerTypeMPLS); mplsLayer != nil {
						mplsPacketCount += 1
					}
				}
				if ip.TTL == mplsOuterTTL-1 {
					t.Logf("TTL decremented by 1")
					break
				} else {
					return fmt.Errorf("Didin't get TTL as expected. Expected: %v, Got: %v", mplsOuterTTL-1, ip.TTL)
				}
			}
		} else {
			if ipLayer := packet.Layer(layers.LayerTypeIPv6); ipLayer != nil {
				ip, _ := ipLayer.(*layers.IPv6)
				if ip.DstIP.Equal(net.ParseIP(ateTrafficDstIPv6)) {
					if mplsLayer := packet.Layer(layers.LayerTypeMPLS); mplsLayer != nil {
						mplsPacketCount += 1
					}
				}
				if ip.HopLimit == mplsOuterTTL-1 {
					t.Logf("TTL decremented by 1")
					break
				} else {
					return fmt.Errorf("Didin't get TTL as expected. Expected: %v, Got: %v", mplsOuterTTL-1, ip.HopLimit)
				}
			}
		}
	}

	if mplsPacketCount != 0 {
		return fmt.Errorf("MPLS label is not popped by the DUT")
	}

	return nil
}

// verifyTrafficValidations runs traffic and validates ATE flow metrics and DUT counters.
func verifyTrafficValidations(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, flowName string) (totalTxFromATE, totalRxAtATE uint64, err []error) {
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
		lossPct := float32(0)
		if txPkts > 0 {
			lossPct = (float32(txPkts-rxPkts) * 100) / float32(txPkts)
		}
		err = append(err, fmt.Errorf("Flow %s: Packet loss detected. TxPkts: %d, RxPkts: %d, Loss %%: %.2f",
			flowName, txPkts, rxPkts, lossPct))
	case txPkts > 0:
		t.Logf("Flow %s: Successfully transmitted %d packets and received %d packets with no loss.",
			flowName, txPkts, rxPkts)
	case packetsPerFlow > 0: // Expected to send but didn't
		err = append(err, fmt.Errorf("Flow %s: No packets were transmitted for this flow, but %d were expected.", flowName, packetsPerFlow))
	}

	return totalTxFromATE, totalRxAtATE, err

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

	t.Logf("DUT Port1 InUnicastPkts: Before=%d, After=%d, Delta=%d",
		dutP1InCountersBefore, dutP1InCountersAfter, inPktsDelta)

	t.Logf("DUT Port2 OutUnicastPkts: Before=%d, After=%d, Delta=%d",
		dutP2OutCountersBefore, dutP2OutCountersAfter, outPktsDelta)

	t.Logf("Total packets sent by ATE (sum of flows reported by ATE): %d", totalTxFromATE)
	t.Logf("Total packets received by ATE (sum of flows reported by ATE): %d", totalRxAtATE)

	expectedTotalTraffic := uint64(packetsPerFlow * flowCount)
	switch {
	case totalTxFromATE > 0:
		// Check if DUT ingress counters reflect ATE Tx.
		// Allow a small difference (e.g., 2%) due to other control plane packets or timing of counter polling.
		if float64(inPktsDelta) < float64(totalTxFromATE)*0.98 {
			return fmt.Errorf("DUT Port1 InUnicastPkts delta (%d) is significantly less than ATE Tx (%d). Expected approx %d.", inPktsDelta, totalTxFromATE, expectedTotalTraffic)
		}
		// Check if DUT egress counters reflect ATE Rx.
		if float64(outPktsDelta) < float64(totalRxAtATE)*0.98 {
			return fmt.Errorf("DUT Port2 OutUnicastPkts delta (%d) is significantly less than ATE Rx (%d). Potential drop in DUT. Expected approx %d.", outPktsDelta, totalRxAtATE, expectedTotalTraffic)
		}
	case expectedTotalTraffic > 0:
		return fmt.Errorf("No traffic was reported as transmitted by ATE flows, but %d total packets were expected.", expectedTotalTraffic)
	}

	return nil
}

func TestMPLSExplicitNull(t *testing.T) {
	var err []error
	var ateTxCounters, ateRxCounters uint64

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Congigure DUT
	configureDUT(t, dut)

	// 1. Configure DUT MPLS, SRGB, and Static Routing
	configureDUTMPLSAndRouting(t, dut)

	// 2. Configure ATE (Interfaces and Traffic Flows)
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

	capture := processCapture(t, ate.OTG(), "port2")

	otgutils.LogFlowMetrics(t, ate.OTG(), otgConfig)
	otgutils.LogPortMetrics(t, ate.OTG(), otgConfig)

	// Capture the DUT counters after traffic
	dutP1AfterCounters, dutP2AfterCounters := captureDUTCounters(t, dut)

	flowNames := []struct {
		name     string
		flowType string
	}{
		{
			name:     flowV4Name,
			flowType: "ipv4",
		},
		{
			name:     flowV6Name,
			flowType: "ipv6",
		},
	}
	for _, flowName := range flowNames {
		t.Run(fmt.Sprintf("ValidateFlow_%s", flowName), func(t *testing.T) {
			ateTxCounters, ateRxCounters, err = verifyTrafficValidations(t, dut, ate, flowName.name)
			if len(err) > 0 {
				t.Errorf("%s", err)
			}
			err := validatePackets(t, capture, flowName.flowType)
			if err != nil {
				t.Errorf("%s", err)
			}
		})
	}

	counterErr := validateDUTCounters(t, len(flowNames), dutP1BeforeCounters, dutP1AfterCounters, dutP2BeforeCounters, dutP2AfterCounters, ateTxCounters, ateRxCounters)
	if counterErr != nil {
		t.Errorf("%s", err)
	}

	t.Log("MPLS Explicit Null Test Completed.")
}
