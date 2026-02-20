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

package decap_gre_ipv4_test

import (
	"os"
	"strings"
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
	"github.com/openconfig/ygot/ygot"
)

const (
	decapDesIpv4IP   = "203.0.113.1/24"
	decapGrpName     = "Decap1"
	nullRoute        = "Null0"
	IPv4Dst1         = "192.0.4.0/30"
	IPv6Dst1         = "2001:db8:1:1:198:18:1:0/126"
	LBL1             = 3400
	IPv4Dst2         = "192.0.5.0/30"
	IPv6Dst2         = "2001:db8:1:1:198:18:2:0/126"
	plen6            = 126
	ttlBeforeDecap   = 64
	trafficFrameSize = 1000
	trafficPps       = 100
	sleepTime        = time.Duration(trafficFrameSize / trafficPps)
)

var (
	dutPort1 = &attrs.Attributes{
		Desc:    "dutPort1",
		MAC:     "00:00:a1:a1:a1:a1",
		IPv6:    "2001:db8::1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
		IPv6Len: plen6,
	}

	dutPort2 = &attrs.Attributes{
		Desc:    "dutPort2",
		MAC:     "00:00:b1:b1:b1:b1",
		IPv6:    "2001:db8::5",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
		IPv6Len: plen6,
	}

	otgPort1 = &attrs.Attributes{
		Name:    "otgPort1",
		MAC:     "00:00:01:01:01:01",
		IPv6:    "2001:db8::2",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
		IPv6Len: plen6,
	}

	otgPort2 = &attrs.Attributes{
		Name:    "otgPort2",
		MAC:     "00:00:02:02:02:02",
		IPv6:    "2001:db8::6",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
		IPv6Len: plen6,
	}

	otgPorts = map[string]*attrs.Attributes{
		"port1": otgPort1,
		"port2": otgPort2,
	}

	dutPorts = map[string]*attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
	}

	decapDestIp = strings.Split(decapDesIpv4IP, "/")[0]
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Topology:
// 	```
//                              |         |
//         [ ATE Port 1 ] ----  |   DUT   | ---- [ ATE Port 2 ]
//                              |         |
//     ```
//
// README FILE:
// https://github.com/openconfig/featureprofiles/blob/main/feature/policy_forwarding/encapsulation/otg_tests/decap_gre_ipv4/README.md

func TestDecapGre(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// configure interfaces on DUT
	configureDUT(t, dut)

	ate := ondatra.ATE(t, "ate")
	otgConfig := ate.OTG()
	config := configureOTG(t, ate)
	sfBatch := &gnmi.SetBatch{}

	// Configuring Static Route: DECAP-DST --> Null0
	configStaticRoute(t, dut, "203.0.113.0/24", nullRoute)

	// Configuring Static Route: IPV4-DST1 --> ATE Port 2
	configStaticRoute(t, dut, IPv4Dst1, otgPort2.IPv4)

	// Configuring Static Route: IPV6-DST1 --> ATE Port 2
	configStaticRoute(t, dut, IPv6Dst1, otgPort2.IPv6)

	// Configure Static Route: MPLS label binding
	cfgplugins.MPLSStaticLSP(t, sfBatch, dut, "lsp1", LBL1, otgPort2.IPv4, dut.Port(t, "port2").Name(), "ipv4")

	// Configure Static Route: IPV4-DST2 --> ATE Port 2
	configStaticRoute(t, dut, IPv4Dst2, otgPort2.IPv4)

	// Configuring Static Route: IPV6-DST2 --> ATE Port 2
	configStaticRoute(t, dut, IPv6Dst2, otgPort2.IPv6)

	// Policy Based Forwading Rule-1
	cfgplugins.PolicyForwardingGreDecapsulation(t, sfBatch, dut, strings.Split(decapDesIpv4IP, "/")[0], "PBR-Policy", "port1", decapGrpName)

	// Test cases.
	type testCase struct {
		Name        string
		Description string
		testFunc    func(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config)
	}

	testCases := []testCase{
		{
			Name:        "Testcase-GreDecapIPv4Traffic",
			Description: "GRE Decapsulation of IPv4 traffic",
			testFunc:    testGreDecapIPv4,
		},
		{
			Name:        "Testcase-GreDecapIPv6Traffic",
			Description: "GRE Decapsulation of IPv6 traffic",
			testFunc:    testGreDecapIPv6,
		},
		{
			Name:        "Testcase-GreDecapIPv4MPLSTraffic",
			Description: "GRE Decapsulation of IPv4-over-MPLS traffic",
			testFunc:    testGreDecapIPv4MPLS,
		},
		{
			Name:        "Testcase-GreDecapIPv6MPLSTraffic",
			Description: "GRE Decapsulation of IPv6-over-MPLS traffic",
			testFunc:    testGreDecapIPv6MPLS,
		},
		{
			Name:        "Testcase-GreDecapMultiLabelMPLSTraffic",
			Description: "GRE Decapsulation of multi-label MPLS traffic",
			testFunc:    testGreDecapMultiLabelMPLS,
		},
		{
			Name:        "Testcase-GrePassthroughTraffic",
			Description: "GRE Pass-through (Negative)",
			testFunc:    testGrePassthrough,
		},
	}

	// Run the test cases.
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Logf("Description: %s", tc.Description)
			tc.testFunc(t, dut, otgConfig, config)
		})
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

// Configures the given DUT interface.
func configInterfaceDUT(p *ondatra.Port, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i := a.NewOCInterface(p.Name(), dut)
	s4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	i.GetOrCreateSubinterface(0).GetOrCreateIpv6()

	return i
}

// Congigure Static Routes on DUT
func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string) {
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

// Configuration on OTG
func configureOTG(t *testing.T, otg *ondatra.ATEDevice) gosnappi.Config {
	otgConfig := gosnappi.NewConfig()

	for portName, portAttrs := range otgPorts {
		port := otg.Port(t, portName)
		dutPort := dutPorts[portName]
		portAttrs.AddToOTG(otgConfig, port, dutPort)
	}

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

func createFlow(flowName string, destMac string) gosnappi.Flow {
	flow := gosnappi.NewFlow().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{otgPort1.Name + ".IPv4"}).SetRxNames([]string{otgPort2.Name + ".IPv4"})
	flow.Size().SetFixed(trafficFrameSize)
	flow.Rate().SetPps(trafficPps)
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(destMac)
	outerIPHeader := flow.Packet().Add().Ipv4()
	outerIPHeader.Src().SetValue(otgPort1.IPv4)
	outerIPHeader.Dst().SetValue(decapDestIp)
	outerIPHeader.TimeToLive().SetValue(ttlBeforeDecap)

	return flow
}

func testGreDecapIPv4(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config) {
	config.Flows().Clear()
	flow := createFlow("DecapGREIpv4", otgPort1.MAC)

	flow.Packet().Add().Gre()

	grev4 := flow.Packet().Add().Ipv4()
	grev4.Src().SetValue(otgPort1.IPv4)
	staticIp := strings.Split(IPv4Dst1, "/")[0]
	grev4.Src().SetValue(otgPort1.IPv4)
	grev4.Dst().SetValue(staticIp)
	grev4.Priority().Dscp().Phb().SetValue(32)

	config.Flows().Append(flow)

	enableCapture(t, config, "port2")
	otgConfig.PushConfig(t, config)
	otgConfig.StartProtocols(t)

	otgutils.WaitForARP(t, otgConfig, config, "IPv4")
	otgutils.WaitForARP(t, otgConfig, config, "IPv6")

	cs := startCapture(t, otgConfig)

	otgConfig.StartTraffic(t)
	time.Sleep(sleepTime * time.Second)

	otgConfig.StopTraffic(t)

	stopCapture(t, otgConfig, cs)

	validateDUTPkts(t, dut)
	validateTrafficLoss(t, otgConfig, config, "DecapGREIpv4")
	validateGREDecapIpv4(t, otgConfig, false)
}

func testGreDecapIPv6(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config) {
	config.Flows().Clear()
	flow := createFlow("DecapIpv6GRE", otgPort1.MAC)

	flow.Packet().Add().Gre()

	grev6 := flow.Packet().Add().Ipv6()
	grev6.Src().SetValue(otgPort1.IPv4)

	staticIp := strings.Split(IPv6Dst1, "/")[0]
	grev6.Src().SetValue(otgPort1.IPv6)
	grev6.Dst().SetValue(staticIp)
	grev6.TrafficClass().SetValue(128)

	config.Flows().Append(flow)
	otgConfig.PushConfig(t, config)

	enableCapture(t, config, "port2")
	otgConfig.StartProtocols(t)

	otgutils.WaitForARP(t, otgConfig, config, "IPv4")
	otgutils.WaitForARP(t, otgConfig, config, "IPv6")

	cs := startCapture(t, otgConfig)

	otgConfig.StartTraffic(t)
	time.Sleep(sleepTime * time.Second)

	otgConfig.StopTraffic(t)

	stopCapture(t, otgConfig, cs)
	validateTrafficLoss(t, otgConfig, config, "DecapIpv6GRE")
	validateGREDecapIpv6(t, otgConfig, false)

}

func testGreDecapIPv4MPLS(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config) {
	config.Flows().Clear()
	flow := createFlow("Decap-IPv4-over-MPLS", otgPort1.MAC)

	flow.Packet().Add().Gre()

	mpls := flow.Packet().Add().Mpls()
	mpls.Label().SetValue(LBL1)

	grev4 := flow.Packet().Add().Ipv4()
	grev4.Src().SetValue(otgPort1.IPv4)

	staticIp := strings.Split(IPv4Dst1, "/")[0]
	grev4.Src().SetValue(otgPort1.IPv4)
	grev4.Dst().SetValue(staticIp)
	grev4.Priority().Dscp().Phb().SetValue(32)

	config.Flows().Append(flow)
	otgConfig.PushConfig(t, config)

	enableCapture(t, config, "port2")

	otgConfig.StartProtocols(t)
	otgutils.WaitForARP(t, otgConfig, config, "IPv4")
	otgutils.WaitForARP(t, otgConfig, config, "IPv6")

	cs := startCapture(t, otgConfig)

	otgConfig.StartTraffic(t)
	time.Sleep(sleepTime * time.Second)

	otgConfig.StopTraffic(t)

	stopCapture(t, otgConfig, cs)
	validateTrafficLoss(t, otgConfig, config, "Decap-IPv4-over-MPLS")
	validateGREDecapIpv4(t, otgConfig, true)

}

func testGreDecapIPv6MPLS(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config) {
	config.Flows().Clear()
	flow := createFlow("Decap-IPv6-over-MPLS", otgPort1.MAC)

	flow.Packet().Add().Gre()

	mpls := flow.Packet().Add().Mpls()
	mpls.Label().SetValue(LBL1)

	grev6 := flow.Packet().Add().Ipv6()
	grev6.Src().SetValue(otgPort1.IPv4)

	staticIp := strings.Split(IPv6Dst1, "/")[0]
	grev6.Src().SetValue(otgPort1.IPv6)
	grev6.Dst().SetValue(staticIp)
	grev6.TrafficClass().SetValue(128)

	config.Flows().Append(flow)
	otgConfig.PushConfig(t, config)

	enableCapture(t, config, "port2")

	otgConfig.StartProtocols(t)
	otgutils.WaitForARP(t, otgConfig, config, "IPv4")
	otgutils.WaitForARP(t, otgConfig, config, "IPv6")

	cs := startCapture(t, otgConfig)

	otgConfig.StartTraffic(t)
	time.Sleep(sleepTime * time.Second)

	otgConfig.StopTraffic(t)

	stopCapture(t, otgConfig, cs)
	validateTrafficLoss(t, otgConfig, config, "Decap-IPv6-over-MPLS")
	validateGREDecapIpv6(t, otgConfig, true)

}

func testGreDecapMultiLabelMPLS(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config) {
	config.Flows().Clear()
	flow := createFlow("Decap-Multilabel-MPLS", otgPort1.MAC)

	flow.Packet().Add().Gre()

	mplsL1 := flow.Packet().Add().Mpls()
	mplsL1.Label().SetValue(LBL1)
	mplsL1.TrafficClass().SetValue(4)

	mplsL2 := flow.Packet().Add().Mpls()
	mplsL2.Label().SetValue(3500)
	mplsL2.TrafficClass().SetValue(4)

	grev4 := flow.Packet().Add().Ipv4()
	grev4.Src().SetValue(otgPort1.IPv4)

	staticIp := strings.Split(IPv4Dst1, "/")[0]
	grev4.Src().SetValue(otgPort1.IPv4)
	grev4.Dst().SetValue(staticIp)
	grev4.Priority().Dscp().Phb().SetValue(32)

	config.Flows().Append(flow)
	otgConfig.PushConfig(t, config)

	enableCapture(t, config, "port2")

	otgConfig.StartProtocols(t)
	otgutils.WaitForARP(t, otgConfig, config, "IPv4")
	otgutils.WaitForARP(t, otgConfig, config, "IPv6")

	cs := startCapture(t, otgConfig)

	otgConfig.StartTraffic(t)
	time.Sleep(sleepTime * time.Second)

	otgConfig.StopTraffic(t)

	stopCapture(t, otgConfig, cs)
	validateTrafficLoss(t, otgConfig, config, "Decap-Multilabel-MPLS")
	validateMPLSDecapTraffic(t, otgConfig)

}

func testGrePassthrough(t *testing.T, dut *ondatra.DUTDevice, otgConfig *otg.OTG, config gosnappi.Config) {
	config.Flows().Clear()

	flow := gosnappi.NewFlow().SetName("DecapGREPassthrough")
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{otgPort1.Name + ".IPv4"}).SetRxNames([]string{otgPort2.Name + ".IPv4"})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(otgPort1.MAC)

	outerIPHeader := flow.Packet().Add().Ipv4()
	outerIPHeader.Src().SetValue(otgPort1.IPv4)
	staticIp := strings.Split(IPv4Dst1, "/")[0]
	outerIPHeader.Dst().SetValue(staticIp)

	v6 := flow.Packet().Add().Ipv6()
	v6.Src().SetValue(otgPort1.IPv6)
	ipv6Ip := strings.Split(IPv6Dst2, "/")[0]
	v6.Dst().SetValue(ipv6Ip)

	flow.Packet().Add().Gre()

	config.Flows().Append(flow)
	otgConfig.PushConfig(t, config)

	otgConfig.StartProtocols(t)
	otgutils.WaitForARP(t, otgConfig, config, "IPv4")
	otgutils.WaitForARP(t, otgConfig, config, "IPv6")

	otgConfig.StartTraffic(t)
	time.Sleep(sleepTime * time.Second)

	otgConfig.StopTraffic(t)

	validateTrafficLoss(t, otgConfig, config, "DecapGREPassthrough")
}

func validateGREDecapIpv4(t *testing.T, otgConfig *otg.OTG, checkTTL bool) {
	packetCaptureGRE := processCapture(t, otgConfig, "port2")
	handle, err := pcap.OpenOffline(packetCaptureGRE)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packet := <-packetSource.Packets()

	var ipv4Layer *layers.IPv4
	ipv4LayerCount := 0
	for _, layer := range packet.Layers() {
		if ipLayer, ok := layer.(*layers.IPv4); ok {
			ipv4LayerCount++
			ipv4Layer = ipLayer
		}
	}

	if ipv4LayerCount == 2 {
		t.Fatalf("Error: Decapsulated IPv4 traffic is not received. Expected %v, got %v", "1", ipv4LayerCount)
	}

	if ipv4Layer != nil {
		dscpValue := ipv4Layer.TOS >> 2
		if dscpValue != 32 {
			t.Fatalf("Error: Inner-packet DSCP should be preserved. Expected: 32, Got: %v", dscpValue)
		} else {
			t.Logf("Inner-packet DSCP is preserved. Expected: 32, Got: %v", dscpValue)
		}
		if checkTTL {
			expectedttl := uint32(ttlBeforeDecap - 1)
			got := uint32(ipv4Layer.TTL)
			if got != expectedttl {
				t.Fatalf("TTL mismatch, got: %d, want: %d", got, expectedttl)
			} else {
				t.Logf("Got expected TTL")
			}
		}
	}
}

func validateGREDecapIpv6(t *testing.T, otgConfig *otg.OTG, checkTTL bool) {
	capturePktFile := processCapture(t, otgConfig, "port2")
	handle, err := pcap.OpenOffline(capturePktFile)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	for packet := range packetSource.Packets() {
		if ipv4Layer := packet.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
			t.Fatalf("Error: Decapsulated IPv6 traffic is not received on ATE Port 2")
		}
		if ipv6Layer := packet.Layer(layers.LayerTypeIPv6); ipv6Layer != nil {
			ipv6, _ := ipv6Layer.(*layers.IPv6)
			t.Logf("Decapsulated IPv6 traffic is received on ATE Port 2")
			if trafficClass := ipv6.TrafficClass; trafficClass != 128 {
				t.Fatalf("Error: Inner-packet traffic-class is not preserved. Expected: 128, Got: %v", trafficClass)
			} else {
				t.Logf("Inner-packet traffic-class is preserved. Expected: 128, Got: %v", trafficClass)
			}

			if checkTTL {
				expectedttl := uint32(ttlBeforeDecap - 1)
				got := uint32(ipv6.HopLimit)
				if got != expectedttl {
					t.Fatalf("TTL mismatch, got: %d, want: %d", got, expectedttl)
				} else {
					t.Logf("Got expected TTL")
				}
			}
			break
		} else {
			t.Fatalf("Decapsulated IPv6 traffic is not received on ATE Port 2")
		}
	}

}

func validateMPLSDecapTraffic(t *testing.T, otgConfig *otg.OTG) {
	capturePktFile := processCapture(t, otgConfig, "port2")
	handle, err := pcap.OpenOffline(capturePktFile)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packet := <-packetSource.Packets()

	var mplsLayer *layers.MPLS
	mplsLayerCount := 0

	for _, layer := range packet.Layers() {
		if ipLayer, ok := layer.(*layers.MPLS); ok {
			mplsLayerCount++
			mplsLayer = ipLayer
		}
	}

	if mplsLayerCount == 2 {
		t.Fatalf("Error: Decapsulated MPLS traffic is not received. Expected %v, got %v", "1", mplsLayerCount)
	}

	if mplsLayer != nil {
		expectedttl := uint32(ttlBeforeDecap - 1)
		got := uint32(mplsLayer.TTL)
		if got != expectedttl {
			t.Fatalf("TTL mismatch, got: %d, want: %d", got, expectedttl)
		} else {
			t.Logf("Got expected TTL")
		}

		mplsExp := mplsLayer.TrafficClass
		if mplsExp != 4 {
			t.Fatalf("Error: EXP is not set to original value.. Expected: 4, Got: %v", mplsExp)
		} else {
			t.Logf("Got expected EXP.Expected: 4, Got: %v", mplsExp)
		}
	}
}

func validateTrafficLoss(t *testing.T, otgConfig *otg.OTG, config gosnappi.Config, flowName string) {
	otgutils.LogFlowMetrics(t, otgConfig, config)
	otgutils.LogPortMetrics(t, otgConfig, config)

	outPkts := float32(gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flowName).Counters().OutPkts().State()))
	inPkts := float32(gnmi.Get(t, otgConfig, gnmi.OTG().Flow(flowName).Counters().InPkts().State()))
	t.Logf("outPkts: %v, inPkts: %v", outPkts, inPkts)
	if outPkts == 0 {
		t.Fatalf("OutPkts for flow %s is 0, want > 0", flowName)
	}
	if got := ((outPkts - inPkts) * 100) / outPkts; got > 0 {
		t.Fatalf("LossPct for flow %s: got %v, want 0", flowName, got)
	}
}

func processCapture(t *testing.T, otg *otg.OTG, port string) string {
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	time.Sleep(30 * time.Second)
	capturePktFile, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := capturePktFile.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	capturePktFile.Close()
	return capturePktFile.Name()
}

// TO-DO: Curently PolicyForwarding not supported in DUT. Adding deviation to check the PF counters.
// Currently added the workaround through CLI, to check the number of packets on the DUT.
func validateDUTPkts(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.GreDecapsulationOCUnsupported(dut) {
		switch dut.Vendor() {
		case ondatra.ARISTA:
			port1 := dut.Port(t, "port1")
			ingressPort := port1.Name()

			port2 := dut.Port(t, "port2")
			egressPort := port2.Name()

			ingressPktPath := gnmi.OC().Interface(ingressPort).Counters().InUnicastPkts().State()
			ingressPkt, present := gnmi.Lookup(t, dut, ingressPktPath).Val()
			if !present {
				t.Errorf("Get IsPresent status for path %q: got false, want true", ingressPktPath)
			}

			egressPktPath := gnmi.OC().Interface(egressPort).Counters().InUnicastPkts().State()
			egressPkt, present := gnmi.Lookup(t, dut, egressPktPath).Val()
			if !present {
				t.Errorf("Get IsPresent status for path %q: got false, want true", egressPktPath)
			}

			if ingressPkt == 0 || egressPkt == 0 {
				t.Errorf("Got the unexpected packet count ingressPkt: %d, egreesPkt: %d", ingressPkt, egressPkt)
			}

			if ingressPkt > egressPkt {
				t.Logf("Interface counters reflect decapsulated packets.")
			} else {
				t.Errorf("Error: Interface counters didn't reflect decapsulated packets.")
			}
		default:
			t.Errorf("Deviation GreDecapsulationUnsupported is not handled for the dut: %v", dut.Vendor())
		}
	} else {
		// TO-DO: Once the support is added in the DUT, need to work on the validation of PF counters.
		matchedPkts := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy("PBR-MAP").Rule(10).MatchedPkts()
		pktCount := gnmi.Get(t, dut, matchedPkts.State())
		if pktCount != 0 {
			t.Logf("Interface counters received")
		} else {
			t.Errorf("Counters received is 0")
		}

		matchedOctets := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy("PBR-MAP").Rule(10).MatchedOctets()
		octetCount := gnmi.Get(t, dut, matchedOctets.State())
		if octetCount == 0 {
			t.Errorf("Octet count is 0")
		}
	}

}
