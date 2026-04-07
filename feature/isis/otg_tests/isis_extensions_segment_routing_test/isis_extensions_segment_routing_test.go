// Copyright 2025 Google LLC
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

//
// RT-2.15 IS-IS Extensions for Segment Routing
// ============================================
//
// Readme Location:
//
// https://github.com/openconfig/featureprofiles/blob/main/feature/isis/otg_tests/isis_extensions_segment_routing_test/README.md
//
// Topology:
// =========
// 		                    	 |         |
//			[ ATE Port 1 ] ----  |   DUT   | ---- [ ATE Port 2]
//      		                 |         |

package isis_extensions_segment_routing_test

import (
	"context"
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
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/ocpath"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	PTISIS                 = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	DUTAreaAddress         = "49.0001"
	ATEAreaAddress1        = "49.0002"
	ATEAreaAddress2        = "49.0003"
	DUTSysID               = "1920.0000.2001"
	ATE1SysID              = "640000000001"
	ATE2SysID              = "640000000002"
	ISISName               = "DEFAULT"
	staticMplsLabel        = "16 16000"
	srgbMplsLabelBlockName = "17000 20000"
	srlbMplsLabelBlockName = "40000 41000"
	ipv4PrefixLen          = 30
	ipv6PrefixLen          = 126
	srgbGlobalLowerBound   = 17000
	srgbGlobalUpperBound   = 20000
	v4Route                = "203.0.113.1"
	v6Route                = "2001:db8::203:0:113:1"
	v4NetName              = "p2.d1.IsisIpv4.rr"
	v6NetName              = "p2.d1.IsisIpv6.rr"
	v4FlowNameNodeSID      = "nodeSidFlowv4"
	v6FlowNameNodeSID      = "nodeSidFlowv6"
	v4FlowNamePrefixSID    = "prefixSidFlowv4"
	v6FlowNamePrefixSID    = "prefixSidFlowv6"
	nodeSIDLabelv4         = 17100
	nodeSIDLabelv6         = 17200
	nodeSidIndexv4         = 500
	nodeSidIndexv6         = 600
	nodeSIDLabelv4_1       = 17500
	nodeSIDLabelv6_1       = 17600
	prefixSIdIndexv4       = 300
	prefixSIdIndexv6       = 400
	prefixSIdLabelv4       = 17300
	prefixSIdLabelv6       = 17400
	packetPerSecond        = 100
	packetSize             = 512
	srgbId                 = "99.99.99.99"
	srlbId                 = "88.88.88.88"
)

var (
	atePort1 = attrs.Attributes{
		Name:    "ateP1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePort2 = attrs.Attributes{
		Name:    "ateP2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}

	dutPort1 = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "DUT to ATE destination-2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutLoopback1 = attrs.Attributes{
		Desc:    "Loopback-ip1",
		IPv4:    "180.0.0.1",
		IPv6:    "180:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}

	dutLoopback2 = attrs.Attributes{
		Desc:    "Loopback-ip2",
		IPv4:    "179.0.0.1",
		IPv6:    "179:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}

	dutLoopback3 = attrs.Attributes{
		Desc:    "Loopback-ip3",
		IPv4:    "178.0.0.1",
		IPv6:    "178:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}

	lb string
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestISISSegmentRouting(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	// configure DUT
	configureDUT(t, dut)
	if deviations.IsisSrgbSrlbUnsupported(dut) {
		configureGlobalMPLS(t, dut)
	}
	configureDUTLoopback(t, dut, 1, dutLoopback1, true)
	configureDUTLoopback(t, dut, 2, dutLoopback2, false)
	configureDUTLoopback(t, dut, 3, dutLoopback3, false)
	configurePrefixSID(t, dut)

	// configure ATE
	topo := configureATE(t)
	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")

	verifyISIS(t, dut, dut.Port(t, "port1").Name(), "dev1Isis")
	verifyISIS(t, dut, dut.Port(t, "port2").Name(), "dev2Isis")

	t.Logf("Starting capture")
	startCapture(t, ate)
	t.Logf("Waiting for routes to be installed on DUT.")
	dni := deviations.DefaultNetworkInstance(dut)
	ipv4Path := gnmi.OC().NetworkInstance(dni).Afts().Ipv4Entry(v4Route + "/32")
	if _, ok := gnmi.Watch(t, dut, ipv4Path.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		return val.IsPresent()
	}).Await(t); !ok {
		t.Fatalf("Route %s not found in AFT", v4Route)
	}

	ipv6Path := gnmi.OC().NetworkInstance(dni).Afts().Ipv6Entry(v6Route + "/128")
	if _, ok := gnmi.Watch(t, dut, ipv6Path.State(), time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
		return val.IsPresent()
	}).Await(t); !ok {
		t.Fatalf("Route %s not found in AFT", v6Route)
	}
	t.Logf("Starting traffic")
	ate.OTG().StartTraffic(t)
	time.Sleep(time.Second * 15)
	t.Logf("Stop traffic")
	ate.OTG().StopTraffic(t)
	t.Logf("Stop Capture")
	stopCapture(t, ate)

	t.Run("Node SID Validation.", func(t *testing.T) {
		verifyPrefixSids(t, ate, dutLoopback1.IPv4, uint32(nodeSIDLabelv4))
		otgutils.LogFlowMetrics(t, ate.OTG(), topo)
		otgutils.LogPortMetrics(t, ate.OTG(), topo)
		verifyTrafficNodeSID(t, ate)
		t.Logf("Verify packet capture")
		processCapture(t, ate.OTG(), "port2")
		VerifyISISSRSIDCounters(t, dut, nodeSIDLabelv4)
	})

	t.Run("Prefix SID Validation", func(t *testing.T) {
		verifyPrefixSids(t, ate, dutLoopback2.IPv4, uint32(prefixSIdLabelv4))
		VerifyISISSRSIDCounters(t, dut, prefixSIdLabelv4)
	})
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()
	// Configure interfaces
	p1 := dut.Port(t, "port1").Name()
	i1 := dutPort1.NewOCInterface(p1, dut)
	gnmi.Replace(t, dut, d.Interface(p1).Config(), i1)

	p2 := dut.Port(t, "port2").Name()
	i2 := dutPort2.NewOCInterface(p2, dut)
	gnmi.Replace(t, dut, d.Interface(p2).Config(), i2)

	dutConfPath := d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISName)
	dutConf := addISISOC(DUTAreaAddress, DUTSysID, p1, p2, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)

}

// addISISOC configures basic IS-IS on a device.
func addISISOC(areaAddress, sysID, ifaceName1 string, ifaceName2 string, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := inst.GetOrCreateProtocol(PTISIS, ISISName)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	if deviations.ISISInstanceEnabledRequired(dut) {
		glob.Instance = ygot.String(ISISName)
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

	// ISIS Segment Routing configurations
	isissr := prot.GetOrCreateIsis().GetOrCreateGlobal().GetOrCreateSegmentRouting()
	isissr.Enabled = ygot.Bool(true)
	isissr.Srgb = ygot.String(srgbMplsLabelBlockName)
	isissr.Srlb = ygot.String(srlbMplsLabelBlockName)

	// SRGB and SRLB Configurations
	segmentrouting := inst.GetOrCreateSegmentRouting()
	srgb := segmentrouting.GetOrCreateSrgb(srgbId)
	srgb.LocalId = ygot.String(srgbId)
	srgb.SetMplsLabelBlocks([]string{srgbMplsLabelBlockName})

	srlb := segmentrouting.GetOrCreateSrlb(srlbId)
	srlb.LocalId = ygot.String(srlbId)
	srlb.SetMplsLabelBlock(srlbMplsLabelBlockName)

	return prot
}

func configurePrefixSID(t *testing.T, dut *ondatra.DUTDevice) {
	if !deviations.IsisSrPrefixSegmentConfigUnsupported(dut) {
		gnmiClient := dut.RawAPIs().GNMI(t)

		// Add no-php flag if required by the device
		noPhpFlag := ""
		if deviations.IsisSrNoPhpRequired(dut) {
			noPhpFlag = " no-php"
		}

		jsonConfig := fmt.Sprintf(`
		router isis %s
		segment-routing mpls
		prefix-segment %s/%v index %d%s
		prefix-segment %s/%v index %d%s
		`, ISISName, dutLoopback2.IPv4, dutLoopback2.IPv4Len, prefixSIdIndexv4,
			noPhpFlag, dutLoopback2.IPv6, dutLoopback2.IPv6Len, prefixSIdIndexv6, noPhpFlag)
		gpbSetRequest := buildCliConfigRequest(jsonConfig)

		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	}

}

// Enable MPLS Forwarding
func configureGlobalMPLS(t *testing.T, dut *ondatra.DUTDevice) {
	gnmiClient := dut.RawAPIs().GNMI(t)

	jsonConfig := fmt.Sprintf(`
    mpls ip
	mpls label range static %s
	mpls label range isis-sr %s
	mpls label range srlb %s
		`, staticMplsLabel, srgbMplsLabelBlockName, srlbMplsLabelBlockName)

	// ARISTA needs to enable hardware counters for mpls lfib to export in-pkts.
	if dut.Vendor() == ondatra.ARISTA {
		jsonConfig += "\thardware counter feature mpls lfib\n"
	}

	gpbSetRequest := buildCliConfigRequest(jsonConfig)

	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
}

// Support method to execute GNMIC commands
func buildCliConfigRequest(config string) *gpb.SetRequest {
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			{
				Path: &gpb.Path{
					Origin: "cli",
					Elem:   []*gpb.PathElem{},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_AsciiVal{
						AsciiVal: config,
					},
				},
			},
		},
	}
	return gpbSetRequest
}

// configureATE sets up the ATE interfaces and BGP configurations.
func configureATE(t *testing.T) gosnappi.Config {
	topo := gosnappi.NewConfig()
	t.Log("Configure ATE interface")
	port1 := topo.Ports().Add().SetName("port1")
	port2 := topo.Ports().Add().SetName("port2")

	c1 := topo.Captures().Add().SetName("Capture")
	c1.SetPortNames([]string{"port2"})

	port1Dev := topo.Devices().Add().SetName(atePort1.Name + ".dev")
	port1Eth := port1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	port1Eth.Connection().SetPortName(port1.Name())
	port1Ipv4 := port1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	port1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	port1Ipv6 := port1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	port1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	port2Dev := topo.Devices().Add().SetName(atePort2.Name + ".dev")
	port2Eth := port2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	port2Eth.Connection().SetPortName(port2.Name())
	port2Ipv4 := port2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	port2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	port2Ipv6 := port2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	port2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	// Add IS-IS in ATE port1
	port1isis := port1Dev.Isis().SetSystemId(ATE1SysID).SetName("dev1Isis")

	// port 1 device 1 isis basic
	port1isis.Basic().SetIpv4TeRouterId(port1Ipv4.Address())
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
	srCap.SrgbRanges().Add().SetStartingSid(uint32(srgbGlobalLowerBound)).SetRange(uint32(srgbGlobalUpperBound))

	devIsisport1 := port1isis.Interfaces().Add().SetEthName(port1Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisPort1").SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_1_2).SetMetric(10)

	devIsisport1.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	// Add IS-IS in ATE port2
	port2isis := port2Dev.Isis().SetSystemId(ATE2SysID).SetName("dev2Isis")

	port2isis.Basic().SetIpv4TeRouterId(port2Ipv4.Address())
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
	srCap1.SrgbRanges().Add().SetStartingSid(uint32(srgbGlobalLowerBound)).SetRange(uint32(srgbGlobalUpperBound))

	devIsisport2 := port2isis.Interfaces().Add().SetEthName(port2Dev.Ethernets().Items()[0].Name()).
		SetName("devIsisPort2").SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT).
		SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_1_2).SetMetric(10)

	devIsisport2.Advanced().SetAutoAdjustMtu(true).SetAutoAdjustArea(true).SetAutoAdjustSupportedProtocols(true)

	// port 1 device 1 isis v4 routes
	p2d1Isisv4routes := port2isis.V4Routes().Add().SetName(v4NetName).SetLinkMetric(10).
		SetOriginType(gosnappi.IsisV4RouteRangeOriginType.INTERNAL)
	p2d1Isisv4routes.Addresses().Add().SetAddress(v4Route).SetPrefix(32).SetCount(1).SetStep(1)

	p2d1Isisv4routes.SetPrefixAttrEnabled(true).SetRFlag(true).SetNFlag(true)
	p2d1Isisv4routes.PrefixSids().Add().SetSidIndices([]uint32{nodeSidIndexv4}).SetRFlag(false).SetNFlag(true).SetPFlag(false).SetAlgorithm(0)

	p2d1Isisv6routes := port2isis.V6Routes().Add().SetName(v6NetName).SetLinkMetric(10).
		SetOriginType(gosnappi.IsisV6RouteRangeOriginType.INTERNAL)
	p2d1Isisv6routes.Addresses().Add().SetAddress(v6Route).SetPrefix(128).SetCount(1).SetStep(1)

	p2d1Isisv6routes.SetPrefixAttrEnabled(true).SetRFlag(true).SetNFlag(true)
	p2d1Isisv6routes.PrefixSids().Add().SetSidIndices([]uint32{nodeSidIndexv6}).SetRFlag(false).SetNFlag(true).SetPFlag(false).SetAlgorithm(0)

	//	We generate traffic entering along port1 and destined for port2
	srcIpv4 := port1Dev.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	srcIpv6 := port1Dev.Ethernets().Items()[0].Ipv6Addresses().Items()[0]
	dstIPv4 := port2Dev.Ethernets().Items()[0].Ipv4Addresses().Items()[0]
	dstIPv6 := port2Dev.Ethernets().Items()[0].Ipv6Addresses().Items()[0]

	// configure v4, v6 traffic sending via node SID
	t.Log("Configuring v4 traffic flow sending via node SID")
	v4Flow := topo.Flows().Add().SetName(v4FlowNameNodeSID)
	v4Flow.Metrics().SetEnable(true)
	v4Flow.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{v4NetName})
	v4Flow.Size().SetFixed(packetSize)
	v4Flow.Rate().SetPps(packetPerSecond)
	v4Flow.Duration().Continuous()
	e1 := v4Flow.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	e1.Dst().Auto()
	mpls := v4Flow.Packet().Add().Mpls()
	mpls.Label().SetValue(nodeSIDLabelv4)

	v4 := v4Flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().SetValue(v4Route)

	//egress tracking
	v4Flow.EgressPacket().Add().Ethernet()
	mplsTcTracking := v4Flow.EgressPacket().Add().Mpls()
	tr1 := mplsTcTracking.Label().MetricTags().Add()
	tr1.SetName("popped-MplsLabel1")
	tr1.SetOffset(17)
	tr1.SetLength(3)

	t.Log("Configuring v6 traffic flow sending via node SID")
	v6Flow := topo.Flows().Add().SetName(v6FlowNameNodeSID)
	v6Flow.Metrics().SetEnable(true)
	v6Flow.TxRx().Device().
		SetTxNames([]string{srcIpv6.Name()}).
		SetRxNames([]string{v6NetName})
	v6Flow.Size().SetFixed(packetSize)
	v6Flow.Rate().SetPps(packetPerSecond)
	v6Flow.Duration().Continuous()
	e2 := v6Flow.Packet().Add().Ethernet()
	e2.Src().SetValue(atePort1.MAC)
	e2.Dst().Auto()
	mplsv6 := v6Flow.Packet().Add().Mpls()
	mplsv6.Label().SetValue(nodeSIDLabelv6)
	v6 := v6Flow.Packet().Add().Ipv6()
	v6.Src().SetValue(atePort1.IPv6)
	v6.Dst().SetValue(v6Route)

	//Egress tracking
	v6Flow.EgressPacket().Add().Ethernet()
	mplsTcTracking1 := v6Flow.EgressPacket().Add().Mpls()
	tr2 := mplsTcTracking1.Label().MetricTags().Add()
	tr2.SetName("Popped-MplsLabel1v6")
	tr2.SetOffset(17)
	tr2.SetLength(3)

	// configure v4, v6 traffic pointing to prefix SID
	t.Log("Configuring v4 traffic flow pointing to Prefix SID")
	v4Flow1 := topo.Flows().Add().SetName(v4FlowNamePrefixSID)
	v4Flow1.Metrics().SetEnable(true)
	v4Flow1.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{dstIPv4.Name()})
	v4Flow1.Size().SetFixed(packetSize)
	v4Flow1.Rate().SetPps(packetPerSecond)
	v4Flow1.Duration().Continuous()
	e3 := v4Flow1.Packet().Add().Ethernet()
	e3.Src().SetValue(atePort1.MAC)
	e3.Dst().Auto()
	mpls3 := v4Flow1.Packet().Add().Mpls()
	mpls3.Label().SetValue(prefixSIdLabelv4)

	v41 := v4Flow1.Packet().Add().Ipv4()
	v41.Src().SetValue(atePort1.IPv4)
	v41.Dst().SetValue(dutLoopback2.IPv4)

	//egress tracking
	v4Flow1.EgressPacket().Add().Ethernet()
	mplsTcTracking3 := v4Flow1.EgressPacket().Add().Mpls()
	tr3 := mplsTcTracking3.Label().MetricTags().Add()
	tr3.SetName("prefix-sidv4-check")
	tr3.SetOffset(17)
	tr3.SetLength(3)

	t.Log("Configuring v6 traffic flow pointing to Prefix SID")
	v6Flow1 := topo.Flows().Add().SetName(v6FlowNamePrefixSID)
	v6Flow1.Metrics().SetEnable(true)
	v6Flow1.TxRx().Device().
		SetTxNames([]string{srcIpv6.Name()}).
		SetRxNames([]string{dstIPv6.Name()})
	v6Flow1.Size().SetFixed(packetSize)
	v6Flow1.Rate().SetPps(packetPerSecond)
	v6Flow1.Duration().Continuous()
	e4 := v6Flow1.Packet().Add().Ethernet()
	e4.Src().SetValue(atePort1.MAC)
	e4.Dst().Auto()
	mplsv61 := v6Flow1.Packet().Add().Mpls()
	mplsv61.Label().SetValue(prefixSIdLabelv6)
	v61 := v6Flow1.Packet().Add().Ipv6()
	v61.Src().SetValue(atePort1.IPv6)
	v61.Dst().SetValue(dutLoopback2.IPv6)

	//Egress tracking
	v6Flow1.EgressPacket().Add().Ethernet()
	mplsTcTracking4 := v6Flow1.EgressPacket().Add().Mpls()
	tr4 := mplsTcTracking4.Label().MetricTags().Add()
	tr4.SetName("prefixv6-label")
	tr4.SetOffset(17)
	tr4.SetLength(3)

	return topo
}

func configureDUTLoopback(t *testing.T, dut *ondatra.DUTDevice, id int, dutLoopback attrs.Attributes, enablenodeSID bool) {
	dutConfIsisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISName)

	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := inst.GetOrCreateProtocol(PTISIS, ISISName)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	t.Helper()
	lb = netutil.LoopbackInterface(t, dut, id)
	lo0 := gnmi.OC().Interface(lb).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	foundV4 := false
	for _, ip := range ipv4Addrs {
		if v, ok := ip.Val(); ok {
			foundV4 = true
			dutLoopback.IPv4 = v.GetIp()
			break
		}
	}
	foundV6 := false
	for _, ip := range ipv6Addrs {
		if v, ok := ip.Val(); ok {
			foundV6 = true
			dutLoopback.IPv6 = v.GetIp()
			break
		}
	}
	if !foundV4 || !foundV6 {
		lo1 := dutLoopback.NewOCInterface(lb, dut)
		lo1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, gnmi.OC().Interface(lb).Config(), lo1)
		isisIntf := isis.GetOrCreateInterface(lo1.GetName())
		isisIntf.GetOrCreateInterfaceRef().Interface = ygot.String(lo1.GetName())
		isisIntf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
		if deviations.InterfaceRefConfigUnsupported(dut) {
			isisIntf.InterfaceRef = nil
		}
		isisIntf.Enabled = ygot.Bool(true)
		gnmi.Update(t, dut, dutConfIsisPath.Config(), prot)

		// enable node segment
		if enablenodeSID {
			if deviations.IsisSrNodeSegmentConfigUnsupported(dut) {
				gnmiClient := dut.RawAPIs().GNMI(t)

				// Add no-php flag if required by the device
				noPhpFlag := ""
				if deviations.IsisSrNoPhpRequired(dut) {
					noPhpFlag = " no-php"
				}

				jsonConfig := fmt.Sprintf(`
				interface %s
				node-segment ipv4 label %v%s
				node-segment ipv6 label %v%s
				`, lo1.GetName(), nodeSIDLabelv4, noPhpFlag, nodeSIDLabelv6, noPhpFlag)

				gpbSetRequest := buildCliConfigRequest(jsonConfig)
				if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
					t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
				}
			}
		}

	}

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, lb, deviations.DefaultNetworkInstance(dut), 0)
	}

}

func verifyTrafficNodeSID(t *testing.T, ate *ondatra.ATEDevice) {
	recvMetricV4 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(v4FlowNameNodeSID).State())
	recvMetricV6 := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(v6FlowNameNodeSID).State())

	framesTxV4 := recvMetricV4.GetCounters().GetOutPkts()
	framesRxV4 := recvMetricV4.GetCounters().GetInPkts()
	framesTxV6 := recvMetricV6.GetCounters().GetOutPkts()
	framesRxV6 := recvMetricV6.GetCounters().GetInPkts()

	t.Logf("Starting V4 traffic validation")
	if framesTxV4 == 0 {
		t.Error("No traffic was generated and frames transmitted were 0")
	} else if framesRxV4 == framesTxV4 {
		t.Logf("Traffic validation successful for [%s] FramesTx: %d FramesRx: %d", v4FlowNameNodeSID, framesTxV4, framesRxV4)
	} else {
		t.Errorf("Traffic validation failed for [%s] FramesTx: %d FramesRx: %d", v4FlowNameNodeSID, framesTxV4, framesRxV4)
	}
	t.Logf("Starting V6 traffic validation")
	if framesTxV6 == 0 {
		t.Error("No traffic was generated and frames transmitted were 0")
	} else if framesRxV6 == framesTxV6 {
		t.Logf("Traffic validation successful for [%s] FramesTx: %d FramesRx: %d", v6FlowNameNodeSID, framesTxV6, framesRxV6)
	} else {
		t.Errorf("Traffic validation failed for [%s] FramesTx: %d FramesRx: %d", v6FlowNameNodeSID, framesTxV6, framesRxV6)
	}
}

// startCapture starts the capture on the otg ports
func startCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
}

// stopCapture starts the capture on the otg ports
func stopCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

func processCapture(t *testing.T, otg *otg.OTG, port string) {
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	time.Sleep(30 * time.Second)
	pcapFile, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := pcapFile.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	pcapFile.Close()
	validatePackets(t, pcapFile.Name())
}

func validatePackets(t *testing.T, filename string) {
	packetCount := int32(0)
	mplsPacketCount := int32(0)

	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	for packet := range packetSource.Packets() {
		if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
			ip, _ := ipLayer.(*layers.IPv4)
			if ip.SrcIP.Equal(net.ParseIP(atePort1.IPv4)) {
				if mplsLayer := packet.Layer(layers.LayerTypeMPLS); mplsLayer != nil {
					mpls, _ := mplsLayer.(*layers.MPLS)
					if mpls.Label == nodeSIDLabelv4 {
						mplsPacketCount += 1
					}

				}
			}
			packetCount += 1
		}
	}
	if mplsPacketCount != 0 {
		t.Errorf("NodeSID is not popped up by the DUT")
	}
}

func verifyISIS(t *testing.T, dut *ondatra.DUTDevice, intfName string, atesysId string) {
	if ok := awaitAdjacency(t, dut, intfName, []oc.E_Isis_IsisInterfaceAdjState{oc.Isis_IsisInterfaceAdjState_UP}); !ok {
		t.Fatal("ISIS Adjacency is Down for interface")
	}
}

// awaitAdjacency wait for adjacency to be up/down
func awaitAdjacency(t *testing.T, dut *ondatra.DUTDevice, intfName string, state []oc.E_Isis_IsisInterfaceAdjState) bool {
	isisPath := ocpath.Root().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, ISISName).Isis()
	intf := isisPath.Interface(intfName)
	query := intf.LevelAny().AdjacencyAny().AdjacencyState().State()
	_, ok := gnmi.WatchAll(t, dut, query, 15*time.Second, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		v, ok := val.Val()
		for _, s := range state {
			if (v == s) && ok {
				return true
			}
		}
		return false
	}).Await(t)
	return ok
}

func verifyPrefixSids(t *testing.T, ate *ondatra.ATEDevice, ipaddr string, label uint32) {

	t.Run("Verify Prefix through TLV ", func(t *testing.T) {
		_, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().IsisRouter("dev1Isis").LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().Prefix(ipaddr).State(), 30*time.Second, func(v *ygnmi.Value[*otgtelemetry.IsisRouter_LinkStateDatabase_Lsps_Tlvs_ExtendedIpv4Reachability_Prefix]) bool {
			_, present := v.Val()
			return present
		}).Await(t)

		if ok {
			t.Logf("Prefix found, want: %s", ipaddr)
		} else {
			t.Errorf("Prefix Not found. want: %s", ipaddr)
		}
	})
}

func VerifyISISSRSIDCounters(t *testing.T, dut *ondatra.DUTDevice, mplsLabel oc.UnionUint32) {

	const timeout = 10 * time.Second
	isPresent := func(val *ygnmi.Value[uint64]) bool { return val.IsPresent() }
	_, ok := gnmi.WatchAll(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().SignalingProtocols().SegmentRouting().AggregateSidCounterAny().InPkts().State(), timeout, isPresent).Await(t)
	if !ok {
		t.Errorf("Unable to find input matched packets related to MPLS label")
	}

	if !deviations.AggregateSIDCounterOutPktsUnsupported(dut) {
		_, ok1 := gnmi.WatchAll(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().SignalingProtocols().SegmentRouting().AggregateSidCounterAny().OutPkts().State(), timeout, isPresent).Await(t)
		if !ok1 {
			t.Errorf("Unable to find output matched packets related to MPLS label")
		}
	}

	// MplsLabel:= mplsLabel oc.E_AggregateSidCounter_MplsLabel]

	inpkts := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().SignalingProtocols().SegmentRouting().AggregateSidCounter(mplsLabel).InPkts().State()

	inpcktstats := gnmi.Get(t, dut, inpkts)
	t.Log(inpcktstats)

	if inpcktstats == 0 {
		t.Errorf("Unable to find input matched packets related to MPLS label")
	}

	if !deviations.AggregateSIDCounterOutPktsUnsupported(dut) {
		OutPkts := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Mpls().SignalingProtocols().SegmentRouting().AggregateSidCounter(mplsLabel).OutPkts()

		outpcktstats := gnmi.Get(t, dut, OutPkts.State())
		t.Log(outpcktstats)

		if outpcktstats == 0 {
			t.Errorf("Unable to find output matched packets related to MPLS label")
		}
	}
}
