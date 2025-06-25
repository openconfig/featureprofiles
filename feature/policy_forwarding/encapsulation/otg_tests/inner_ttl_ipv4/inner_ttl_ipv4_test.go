// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package inner_ttl_ipv4_test

import (
	"context"
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
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

const (
	ipv6PrefixLen   = 126
	ipv4PrefixLen   = 30
	trafficDuration = 30
	ipv4InnerDstA   = "10.5.1.1"
	ipv4InnerDstB   = "10.5.1.2"
	ipv6InnerDstA   = "2001:f:c:e::1"
	ipv6InnerDstB   = "2001:f:c:e::2"
	innerIpv4Prefix = 32
	innerIpv6Prefix = 128
	outerDstUDPPort = 6635
	outerDscp       = 26
	outerHopLimit   = 64
	outerIPTTL      = 64
	innerTTL        = 64
	policyName      = "RETAIN_TTL_TRAFFIC_POLICY"
	flowName1       = "mpls_in_gre_ipv4"
	flowName2       = "mpls_in_gre_ipv6"
	nextHopGName    = "mplsGre"
	frameSize       = 1500
	trafficPps      = 500
	lossTolerance   = 2
	labelStack      = 16000
	mtu             = 2000
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "port1",
		MAC:     "02:01:00:00:00:01",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	otgPort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "port2",
		MAC:     "02:01:00:00:00:02",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	otgPort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: ipv4PrefixLen,
		IPv6Len: ipv6PrefixLen,
	}
	atePorts = map[string]attrs.Attributes{
		"port1": otgPort1,
		"port2": otgPort2,
	}
	dutPorts = map[string]attrs.Attributes{
		"port1": dutPort1,
		"port2": dutPort2,
	}
	timeout  = 1 * time.Minute
	interval = 10 * time.Second
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestIngressInnerPktTTL(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	topo := configureATE(t)
	verifyPortsUp(t, dut.Device)

	t.Run("Verify IPv4 MPLS-IN-GRE Packets", func(t *testing.T) {
		mplsInGREFlowIPv4(t, topo)
		enableCapture(t, topo, topo.Ports().Items()[1].Name())
		ate.OTG().PushConfig(t, topo)
		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
		cs := startCapture(t, ate.OTG())
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration * time.Second)
		ate.OTG().StopTraffic(t)
		stopCapture(t, ate.OTG(), cs)
		if verifyFlowTraffic(t, ate, topo, flowName1) {
			t.Log("IPv4 Traffic MPLS-IN-GRE forwarding Passed")
		} else {
			t.Error("IPv4 Traffic MPLS-IN-GRE forwarding Failed")
		}
		if validateGREencapIpv4(t, ate.OTG()) {
			t.Log("Validated IPv4 destination IP's, MPLS label, Inner packet TTL")
		} else {
			t.Error("Failed to validate IPv4 destination IP's, MPLS label, Inner packet TTL")
		}

	})
	t.Run("Verify IPv6 MPLS-IN-GRE Packets", func(t *testing.T) {
		mplsInGREFlowIPv6(t, topo)
		enableCapture(t, topo, topo.Ports().Items()[1].Name())
		ate.OTG().PushConfig(t, topo)
		ate.OTG().StartProtocols(t)
		otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
		cs := startCapture(t, ate.OTG())
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration * time.Second)
		ate.OTG().StopTraffic(t)
		stopCapture(t, ate.OTG(), cs)
		if verifyFlowTraffic(t, ate, topo, flowName2) {
			t.Log("IPv6 Traffic MPLS-IN-GRE forwarding Passed")
		} else {
			t.Error("IPv6 Traffic MPLS-IN-GRE forwarding Failed")
		}
		if validateGREencapIpv6(t, ate.OTG()) {
			t.Log("Validated IPv6 destination IP's, MPLS label, Outer packet HopLimit")
		} else {
			t.Error("Failed to validate IPv6 destination IP's, MPLS label, Outer packet HopLimit")
		}
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
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	configureMTUonIntreface(t, dut, p2)
	t.Logf("Configuring Loopback and Tunnel on DUT ...")
	configureDUTTunnel(t, dut)
	t.Logf("Configuring default instance forwarding policy on DUT ...")
	cfgplugins.NewTrafficPolicy(t, dut, policyName, ipv4InnerDstA, ipv4InnerDstB, innerIpv4Prefix, ipv6InnerDstA, ipv6InnerDstB, innerIpv6Prefix, nextHopGName, p1)
}

func configureMTUonIntreface(t *testing.T, dut *ondatra.DUTDevice, intName string) {
	if deviations.OmitL2MTU(dut) {
		gnmiClient := dut.RawAPIs().GNMI(t)
		jsonConfig := fmt.Sprintf(`	
		interface %s
		mtu %d					
		`, intName, mtu)
		gpbSetRequest := buildCliConfigRequest(jsonConfig)

		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	}
}
func configureDUTTunnel(t *testing.T, dut *ondatra.DUTDevice) {
	if deviations.TunnelConfigPathUnsupported(dut) {
		gnmiClient := dut.RawAPIs().GNMI(t)
		jsonConfig := fmt.Sprintf(`	
		nexthop-group %s type mpls-over-gre
		ttl %d
		tunnel-source %s
		fec hierarchical
		entry 0 push label-stack %d tunnel-destination %s tunnel-source %s					
		`, nextHopGName, innerTTL, dutPort1.IPv4, labelStack, otgPort2.IPv4, dutPort1.IPv4)
		gpbSetRequest := buildCliConfigRequest(jsonConfig)

		if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
			t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
		}
	}
}

// configureATE configures port1 and port2 on the ATE.
func configureATE(t *testing.T) gosnappi.Config {
	t.Log("Configure ATE interface")
	top := gosnappi.NewConfig()
	for p, ap := range atePorts {
		dp := dutPorts[p]
		top.Ports().Add().SetName(ap.Name)
		i1 := top.Devices().Add().SetName(ap.Name)
		eth1 := i1.Ethernets().Add().SetName(ap.Name + ".Eth").SetMac(ap.MAC)
		eth1.Connection().SetPortName(i1.Name())
		eth1.Ipv4Addresses().Add().SetName(i1.Name() + ".IPv4").
			SetAddress(ap.IPv4).SetGateway(dp.IPv4).
			SetPrefix(uint32(ap.IPv4Len))
		eth1.Ipv6Addresses().Add().SetName(i1.Name() + ".IPv6").
			SetAddress(ap.IPv6).SetGateway(dp.IPv6).
			SetPrefix(uint32(ap.IPv6Len))
	}
	return top
}

func mplsInGREFlowIPv4(t *testing.T, config gosnappi.Config) {
	dut := ondatra.DUT(t, "dut")
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	flow := config.Flows().Add()
	flow.SetName(flowName1)
	flow.Size().SetFixed(frameSize)
	flow.Rate().SetPps(trafficPps)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName(config.Ports().Items()[0].Name()).SetRxNames([]string{config.Ports().Items()[1].Name()})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(otgPort1.MAC)
	ethHeader.Dst().SetValue(macAddress)

	ipv4Header := flow.Packet().Add().Ipv4()
	ipv4Header.Src().SetValue(otgPort1.IPv4)
	ipv4Header.Dst().SetValue(ipv4InnerDstB)
	ipv4Header.TimeToLive().SetValue(innerTTL)
}

func mplsInGREFlowIPv6(t *testing.T, config gosnappi.Config) {
	dut := ondatra.DUT(t, "dut")
	macAddress := gnmi.Get(t, dut, gnmi.OC().Interface(dut.Port(t, "port1").Name()).Ethernet().MacAddress().State())
	config.Flows().Clear()
	flow := config.Flows().Add()
	flow.SetName(flowName2)
	flow.Size().SetFixed(frameSize)
	flow.Rate().SetPps(trafficPps)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Port().SetTxName(config.Ports().Items()[0].Name()).SetRxNames([]string{config.Ports().Items()[1].Name()})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(otgPort1.MAC)
	ethHeader.Dst().SetValue(macAddress)

	ipv6Header := flow.Packet().Add().Ipv6()
	ipv6Header.TrafficClass().SetValue(outerDscp)
	ipv6Header.HopLimit().SetValue(outerHopLimit)
	ipv6Header.Src().SetValue(otgPort1.IPv6)
	ipv6Header.Dst().SetValue(ipv6InnerDstB)

	udpHeader := flow.Packet().Add().Udp()
	udpHeader.DstPort().SetValue(outerDstUDPPort)
}

// Verify ports status
func verifyPortsUp(t *testing.T, dev *ondatra.Device) {
	t.Helper()
	t.Log("Verifying port status")
	for _, p := range dev.Ports() {
		status := gnmi.Get(t, dev, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if want := oc.Interface_OperStatus_UP; status != want {
			t.Errorf("%s Status: got %v, want %v", p, status, want)
		}
	}
}

func verifyFlowTraffic(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config, flowName string) bool {
	otgutils.LogFlowMetrics(t, ate.OTG(), config)
	countersPath := gnmi.OTG().Flow(flowName).Counters()
	txRate := gnmi.Get(t, ate.OTG(), countersPath.OutPkts().State())
	isWithinTolerance := func(v uint64) bool {
		return v >= txRate-lossTolerance && v <= txRate+lossTolerance
	}
	txVal, ok := gnmi.Watch(t, ate.OTG(), countersPath.OutPkts().State(), timeout,
		func(val *ygnmi.Value[uint64]) bool {
			v, present := val.Val()
			return val.IsPresent() && present && isWithinTolerance(v)
		}).Await(t)

	if !ok {
		t.Errorf("Flow %q: TX did not reach expected count (%d)", flowName, txRate)
		return false
	}

	// Wait for RX to match TX exactly
	rxVal, ok := gnmi.Watch(t, ate.OTG(), countersPath.InPkts().State(), timeout,
		func(val *ygnmi.Value[uint64]) bool {
			v, present := val.Val()
			return val.IsPresent() && present && isWithinTolerance(v)
		}).Await(t)

	if !ok {
		t.Errorf("Flow %q: RX packets did not match expected TX count (%d)", flowName, txRate)
		return false
	}

	txPkts, _ := txVal.Val()
	rxPkts, _ := rxVal.Val()
	t.Logf("Flow %q: TX=%d, RX=%d", flowName, txPkts, rxPkts)
	return true
}

func validateGREencapIpv4(t *testing.T, otgConfig *otg.OTG) bool {
	packetCaptureGRE := processCapture(t, otgConfig, "port2")
	handle, err := pcap.OpenOffline(packetCaptureGRE)
	if err != nil {
		t.Fatal(err)
		return false
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

	if ipv4LayerCount != 2 {
		t.Fatalf("Error: Encapsulated IPv4 traffic is not received. Expected %v, got %v", "2", ipv4LayerCount)
		return false
	}

	if ipv4Layer != nil {
		gotDstIp := ipv4Layer.DstIP
		if gotDstIp.String() == ipv4InnerDstB {
			t.Log("Validated IPV4 Outer Destination IP's")
		} else {
			t.Fatalf("Failed to Validate IPV4 Outer Destination IP's")
		}
		expectedttl := uint32(innerTTL - 1)
		got := uint32(ipv4Layer.TTL)
		if got != expectedttl {
			t.Fatalf("TTL mismatch, got: %d, want: %d", got, innerTTL)
			return false
		} else {
			t.Logf("Got expected TTL")
		}
	}
	for _, layer := range packet.Layers() {
		if greLayer, ok := layer.(*layers.GRE); ok {
			if greLayer == nil {
				t.Fatalf("Failed to add GRE header")
				return false
			} else {
				t.Log("Successfully Added GRE Header")
			}
		}
		if mplsLayer, ok := layer.(*layers.MPLS); ok {
			if mplsLayer.Label != 0 {
				t.Logf("Added MPLS Label")
			} else {
				t.Fatalf("MPLS Label not added")
				return false
			}
		}
	}
	return true
}

func validateGREencapIpv6(t *testing.T, otgConfig *otg.OTG) bool {
	packetCaptureGRE := processCapture(t, otgConfig, "port2")
	handle, err := pcap.OpenOffline(packetCaptureGRE)
	if err != nil {
		t.Fatal(err)
		return false
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packet := <-packetSource.Packets()

	var ipv6Layer *layers.IPv6
	ipv6LayerCount := 0
	for _, layer := range packet.Layers() {
		if ipLayer, ok := layer.(*layers.IPv6); ok {
			ipv6LayerCount++
			ipv6Layer = ipLayer
		}
	}
	if ipv6Layer != nil {
		gotDstIp := ipv6Layer.DstIP
		if gotDstIp.String() == ipv6InnerDstB {
			t.Log("Validated IPV6 Outer Destination IP's")
		} else {
			t.Fatalf("Failed to Validate IPV6 Outer Destination IP's")
		}
		expectedttl := uint32(outerHopLimit - 1)
		got := uint32(ipv6Layer.HopLimit)
		if got != expectedttl {
			t.Fatalf("HopLimit mismatch, got: %d, want: %d", got, expectedttl)
			return false
		} else {
			t.Logf("Got expected HopLimit")
		}
	}
	for _, layer := range packet.Layers() {
		if greLayer, ok := layer.(*layers.GRE); ok {
			if greLayer == nil {
				t.Fatalf("Failed to add GRE header")
				return false
			} else {
				t.Log("Successfully Added GRE Header")
			}
		}
		if mplsLayer, ok := layer.(*layers.MPLS); ok {
			if mplsLayer.Label != 0 {
				t.Logf("Added MPLS Label")
			} else {
				t.Fatalf("MPLS Label not added")
				return false
			}
		}
	}
	return true
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
	time.Sleep(interval)
	return cs
}

func stopCapture(t *testing.T, otg *otg.OTG, cs gosnappi.ControlState) {
	time.Sleep(interval)
	t.Helper()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

func processCapture(t *testing.T, otg *otg.OTG, port string) string {
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(port))
	time.Sleep(trafficDuration * time.Second)
	pcapFile, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := pcapFile.Write(bytes); err != nil {
		t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	pcapFile.Close()
	return pcapFile.Name()
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
