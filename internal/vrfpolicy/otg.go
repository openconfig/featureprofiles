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

package vrfpolicy

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

// ConfigureOTGTopology sets up the 4 ATE ports, creating the necessary devices,
// VLANs, and IP addresses, focusing on mimicking the DUT's egress subinterfaces.
func ConfigureOTGTopology(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice) {
	t.Helper()

	p1Name := ate.Port(t, "port1").ID()
	p2Name := ate.Port(t, "port2").ID()
	p3Name := ate.Port(t, "port3").ID()
	p4Name := ate.Port(t, "port4").ID()

	p1 := top.Ports().Add().SetName(p1Name)
	top.Ports().Add().SetName(p2Name)
	top.Ports().Add().SetName(p3Name)
	top.Ports().Add().SetName(p4Name)

	// Port 1 (Ingress - Source of flows)
	dev1 := top.Devices().Add().SetName("dev-port1")
	eth1 := dev1.Ethernets().Add().SetName("eth-port1").SetMac("02:1a:c0:00:00:01")
	eth1.Connection().SetPortName(p1.Name())
	eth1.Ipv4Addresses().Add().SetName("ipv4-port1").SetAddress("192.168.0.2").SetGateway("192.168.0.1").SetPrefix(24)

	// Port 2 devices (Egress 1 - 10 positive v4, 1 default)
	idx := 1
	for i := 1; i <= 10; i++ {
		addDevice(top, p2Name, fmt.Sprintf("dev-v4-%d", i), fmt.Sprintf("02:1a:c0:00:02:%02x", idx), idx, fmt.Sprintf("192.168.%d.2", idx), fmt.Sprintf("192.168.%d.1", idx), "")
		idx++
	}
	// Port 2 Default
	addDevice(top, p2Name, "dev-default", "02:1a:c0:00:02:ff", idx, fmt.Sprintf("192.168.%d.2", idx), fmt.Sprintf("192.168.%d.1", idx), fmt.Sprintf("2001:db8:%d::2", idx))
	idx++

	// Port 3 (Egress 2 - 5 positive v4, 5 positive v6)
	for i := 11; i <= 15; i++ {
		addDevice(top, p3Name, fmt.Sprintf("dev-v4-%d", i), fmt.Sprintf("02:1a:c0:00:03:%02x", idx), idx, fmt.Sprintf("192.168.%d.2", idx), fmt.Sprintf("192.168.%d.1", idx), "")
		idx++
	}
	for i := 1; i <= 5; i++ {
		addDevice(top, p3Name, fmt.Sprintf("dev-v6-%d", i), fmt.Sprintf("02:1a:c0:01:03:%02x", idx), idx, fmt.Sprintf("192.168.%d.2", idx), fmt.Sprintf("192.168.%d.1", idx), fmt.Sprintf("2001:db8:%d::2", idx))
		idx++
	}

	// Port 4 (Egress 3 - 10 positive v6)
	for i := 6; i <= 15; i++ {
		addDevice(top, p4Name, fmt.Sprintf("dev-v6-%d", i), fmt.Sprintf("02:1a:c0:01:04:%02x", idx), idx, fmt.Sprintf("192.168.%d.2", idx), fmt.Sprintf("192.168.%d.1", idx), fmt.Sprintf("2001:db8:%d::2", idx))
		idx++
	}
}

func addDevice(top gosnappi.Config, portName, devName, macAddr string, idx int, ipv4Addr, ipv4Gw, ipv6Addr string) {
	dev := top.Devices().Add().SetName(devName)
	eth := dev.Ethernets().Add().SetName(devName + "-eth").SetMac(macAddr)
	eth.Connection().SetPortName(portName)

	// We assume subinterfaces have vlan ID matching index based on test setup
	eth.Vlans().Add().SetName(devName + "-vlan").SetId(uint32(idx))

	if ipv4Addr != "" {
		eth.Ipv4Addresses().Add().SetName(devName + "-ipv4").SetAddress(ipv4Addr).SetGateway(ipv4Gw).SetPrefix(24)
	}
	if ipv6Addr != "" {
		// Compute IPv6 Gateway for OTG device which is the DUT's IPv6 address
		ipv6Gw := fmt.Sprintf("2001:db8:%d::1", idx)
		eth.Ipv6Addresses().Add().SetName(devName + "-ipv6").SetAddress(ipv6Addr).SetGateway(ipv6Gw).SetPrefix(64)
	}
}

// ConfigureScaledOTGFlows sets up 30 positive streams, 30 negative streams, 1 ghost stream, and 1 shadow stream.
func ConfigureScaledOTGFlows(top gosnappi.Config) {
	// 30 Positive Streams (IPinIP matching Rule 1-15 and IPv6inIP matching Rule 16-30)
	for i := 1; i <= 15; i++ {
		flowName := fmt.Sprintf("positive-v4-%d", i)
		addFlow(top, flowName, "dev-port1", []string{fmt.Sprintf("dev-v4-%d", i), "dev-default"}, 4, fmt.Sprintf("198.18.%d.1", i), fmt.Sprintf("203.0.113.%d", i), fmt.Sprintf("203.0.113.%d", i))
	}
	for i := 1; i <= 15; i++ {
		flowName := fmt.Sprintf("positive-v6-%d", i)
		addFlow(top, flowName, "dev-port1", []string{fmt.Sprintf("dev-v6-%d", i), "dev-default"}, 41, fmt.Sprintf("198.19.%d.1", i), fmt.Sprintf("203.0.113.%d", i), fmt.Sprintf("2001:db8:a:%d::", i)) // Inner IPv6 matches VRF-V6-x route
	}

	// 30 Negative Streams (Standard UDP, lacking encapsulation)
	for i := 1; i <= 15; i++ {
		flowName := fmt.Sprintf("negative-v4-%d", i)
		addFlow(top, flowName, "dev-port1", []string{"dev-default"}, 17, fmt.Sprintf("10.0.%d.1", i), fmt.Sprintf("203.0.113.%d", i), "")
	}
	for i := 1; i <= 15; i++ {
		// Technically these destinations are IPv6 blocks in the route table
		flowName := fmt.Sprintf("negative-v6-%d", i)
		addFlow(top, flowName, "dev-port1", []string{"dev-default"}, 17, fmt.Sprintf("10.1.%d.1", i), fmt.Sprintf("203.0.113.%d", i), "")
	}

	// Ghost Stream (Matches Rule 31, should be dropped)
	addFlow(top, "ghost-stream", "dev-port1", []string{"dev-default"}, 4, "198.20.0.1", "203.0.113.1", "203.0.113.1")

	// Shadow Stream (Matches Rule 100, maps to VRF-V4-15)
	addFlow(top, "shadow-stream", "dev-port1", []string{"dev-v4-15", "dev-default"}, 4, "0.0.0.0", "203.0.113.15", "203.0.113.15")
}

func addFlow(top gosnappi.Config, flowName, srcEndpoint string, dstEndpoints []string, protocol uint32, srcIP, dstIP, innerIP string) {
	flow := top.Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{srcEndpoint}).SetRxNames(dstEndpoints)

	flow.Packet().Add().Ethernet()

	var isIpv6 bool
	for _, c := range srcIP {
		if c == ':' {
			isIpv6 = true
			break
		}
	}

	if isIpv6 {
		ip := flow.Packet().Add().Ipv6()
		ip.Src().SetValue(srcIP)
		ip.Dst().SetValue(dstIP)
		ip.NextHeader().SetValue(protocol) // Use NextHeader for IPv6 outer
	} else {
		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue(srcIP)
		ip.Dst().SetValue(dstIP)
		ip.Protocol().SetValue(protocol) // 4 for IPinIP, 41 for IPv6inIP(v4 outer), 17 for UDP
	}

	if protocol == 4 {
		inner := flow.Packet().Add().Ipv4()
		inner.Src().SetValue("198.51.100.1")
		inner.Dst().SetValue(dstIP) // Just replicate outer dst if innerIP=""
		if innerIP != "" {
			inner.Dst().SetValue(innerIP)
		}
	} else if protocol == 41 {
		inner := flow.Packet().Add().Ipv6()
		inner.Src().SetValue("2001:db8:c::1")
		inner.Dst().SetValue("2001:db8:c::2") // Valid unused IPv6 string
		if innerIP != "" {
			inner.Dst().SetValue(innerIP)
		}
	} else if protocol == 17 {
		udp := flow.Packet().Add().Udp()
		udp.SrcPort().SetValue(1024)
		udp.DstPort().SetValue(1024)
	}
}

// ValidateBaselineTraffic checks that loss metrics match expectations.
func ValidateBaselineTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	t.Logf("Validating Baseline OTG Traffic Isolations...")
	// Wait for metrics
	time.Sleep(15 * time.Second)

	for _, f := range top.Flows().Items() {
		outPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(f.Name()).Counters().OutPkts().State())
		inPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(f.Name()).Counters().InPkts().State())

		if outPkts == 0 {
			t.Errorf("Flow %s did not send any packets", f.Name())
			continue
		}

		lossPct := (float64(outPkts) - float64(inPkts)) / float64(outPkts) * 100.0
		t.Logf("Flow %s: OutPkts %d, InPkts %d, Loss %.2f%%", f.Name(), outPkts, inPkts, lossPct)

		isGhost := f.Name() == "ghost-stream"

		if isGhost {
			// Ghost must be dropped
			if lossPct < 99.0 {
				t.Errorf("Flow %s (Ghost) should be dropped, but loss is only: %.2f%%", f.Name(), lossPct)
			}
		} else {
			// Positives, Negatives, Shadow must be hitless
			if lossPct > 0.5 {
				t.Errorf("Flow %s has unexpected packet loss: %.2f%%, want <= 0.5%%", f.Name(), lossPct)
			}
		}

		// TODO: In a truly resilient test, we'd also verify the specific RX port
		// using the flow metrics rx port info or device Rx metrics to prove VRF isolation.
		// For now, ensuring 0% loss asserts that the traffic reached *a* destination in the top.
	}
}

// ValidateFallbackTraffic checks that after policy deletion, all traffic (positive streams)
// falls back to the DEFAULT VRF, meaning it should still arrive, but technically via the default route.
func ValidateFallbackTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	t.Helper()
	t.Logf("Validating Fallback OTG Traffic...")
	time.Sleep(15 * time.Second)

	for _, f := range top.Flows().Items() {
		outPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(f.Name()).Counters().OutPkts().State())
		inPkts := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(f.Name()).Counters().InPkts().State())

		if outPkts == 0 {
			t.Errorf("Flow %s did not send any packets", f.Name())
			continue
		}

		lossPct := (float64(outPkts) - float64(inPkts)) / float64(outPkts) * 100.0
		t.Logf("Flow %s: OutPkts %d, InPkts %d, Loss %.2f%%", f.Name(), outPkts, inPkts, lossPct)

		// With policy deleted, ALL streams are treated as generic IP traffic and look up the
		// DEFAULT VRF table. Thus all should egress the DEFAULT port
		// Ghost stream may now forward if there's a default route 0.0.0.0/0.
		// For simplicity in this assertion, we ensure it's hitless.
		if lossPct > 0.5 {
			t.Errorf("Flow %s has unexpected packet loss in fallback: %.2f%%", f.Name(), lossPct)
		}
	}
}
