// Copyright 2024 Google LLC
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

package vrf_selection_resiliency_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	cmp "github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/vrfpolicy"
	"github.com/openconfig/gnoigo/system"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnoi"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	trafficDuration = 10 * time.Second
	ipipProtocol    = 4
	ipv6ipProtocol  = 41
)

var (
	dutPort1 = attrs.Attributes{Desc: "ingress", MAC: "02:00:01:00:00:01", IPv4: "192.0.2.1", IPv6: "2001:db8::1", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	atePort1 = attrs.Attributes{Name: "ate_port1", MAC: "02:00:01:00:00:02", IPv4: "192.0.2.2", IPv6: "2001:db8::2", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
)

func getDUTTPEgressAttrs(idx int) (dut, ate attrs.Attributes) {
	dut = attrs.Attributes{Desc: fmt.Sprintf("egress_dut_%d", idx), MAC: fmt.Sprintf("02:00:02:00:%02x:%02x", idx>>8, idx&0xff), IPv4: fmt.Sprintf("192.0.2.%d", 4*idx+1), IPv4Len: ipv4PrefixLen, IPv6: fmt.Sprintf("2001:db8:1:%x::1", idx), IPv6Len: ipv6PrefixLen}
	ate = attrs.Attributes{Name: fmt.Sprintf("ate_port_egress_%d", idx), MAC: fmt.Sprintf("02:00:03:00:%02x:%02x", idx>>8, idx&0xff), IPv4: fmt.Sprintf("192.0.2.%d", 4*idx+2), IPv4Len: ipv4PrefixLen, IPv6: fmt.Sprintf("2001:db8:1:%x::2", idx), IPv6Len: ipv6PrefixLen}
	return
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	// Default instance
	t.Log("Configuring default network-instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	// Create Ghost VRF to be deleted later
	t.Log("Configuring Ghost VRF")
	ghostNi := &oc.NetworkInstance{Name: ygot.String("VRF-GHOST")}
	ghostNi.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ghostNi.Name).Config(), ghostNi)

	// Port 1 (Ingress)
	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	i1.Description = ygot.String("Ingress port")
	i1.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i1.Enabled = ygot.Bool(true)
	}
	s1 := i1.GetOrCreateSubinterface(0)
	s1.GetOrCreateIpv4().GetOrCreateAddress(dutPort1.IPv4).PrefixLength = ygot.Uint8(ipv4PrefixLen)
	s1.GetOrCreateIpv6().GetOrCreateAddress(dutPort1.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)
	if deviations.InterfaceEnabled(dut) {
		if !deviations.IPv4MissingEnabled(dut) {
			s1.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		}
		s1.GetOrCreateIpv6().Enabled = ygot.Bool(true)
	}
	gnmi.Update(t, dut, d.Interface(p1.Name()).Config(), i1)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	// Port 2
	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name()), Description: ygot.String("Egress Port-2"), Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd}
	if deviations.InterfaceEnabled(dut) {
		i2.Enabled = ygot.Bool(true)
	}
	for i := 1; i <= 10; i++ {
		sDst := i2.GetOrCreateSubinterface(uint32(i))
		if !deviations.DeprecatedVlanID(dut) {
			sDst.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(i))
		} else {
			sDst.GetOrCreateVlan().VlanId = oc.UnionUint16(uint16(i))
		}
		dutA, _ := getDUTTPEgressAttrs(i)
		sDst.GetOrCreateIpv4().GetOrCreateAddress(dutA.IPv4).PrefixLength = ygot.Uint8(ipv4PrefixLen)
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			sDst.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		}
		ni := &oc.NetworkInstance{Name: ygot.String(fmt.Sprintf("VRF-V4-%d", i))}
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ni.Name).Config(), ni)
	}
	// Subinterface 11 is default
	s11 := i2.GetOrCreateSubinterface(11)
	if !deviations.DeprecatedVlanID(dut) {
		s11.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(11)
	} else {
		vlan11 := uint16(11)
		s11.GetOrCreateVlan().VlanId = oc.UnionUint16(vlan11)
	}
	dutA, _ := getDUTTPEgressAttrs(11)
	s11.GetOrCreateIpv4().GetOrCreateAddress(dutA.IPv4).PrefixLength = ygot.Uint8(ipv4PrefixLen)
	s11.GetOrCreateIpv6().GetOrCreateAddress(dutA.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)
	if deviations.InterfaceEnabled(dut) {
		if !deviations.IPv4MissingEnabled(dut) {
			s11.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		}
		s11.GetOrCreateIpv6().Enabled = ygot.Bool(true)
	}

	gnmi.Update(t, dut, d.Interface(p2.Name()).Config(), i2)
	for i := 1; i <= 10; i++ {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), fmt.Sprintf("VRF-V4-%d", i), uint32(i))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 11)
	}
	gnmi.Update(t, dut, d.Interface(p2.Name()).Config(), i2)

	// Port 3
	p3 := dut.Port(t, "port3")
	i3 := &oc.Interface{Name: ygot.String(p3.Name()), Description: ygot.String("Egress Port-3"), Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd}
	if deviations.InterfaceEnabled(dut) {
		i3.Enabled = ygot.Bool(true)
	}
	for i := 1; i <= 5; i++ {
		idx := 11 + i
		sDst := i3.GetOrCreateSubinterface(uint32(idx))
		if !deviations.DeprecatedVlanID(dut) {
			sDst.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(idx))
		} else {
			sDst.GetOrCreateVlan().VlanId = oc.UnionUint16(uint16(idx))
		}
		dutA, _ := getDUTTPEgressAttrs(idx)
		sDst.GetOrCreateIpv4().GetOrCreateAddress(dutA.IPv4).PrefixLength = ygot.Uint8(ipv4PrefixLen)
		if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
			sDst.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		}
		ni := &oc.NetworkInstance{Name: ygot.String(fmt.Sprintf("VRF-V4-%d", 10+i))}
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ni.Name).Config(), ni)
	}
	for i := 1; i <= 5; i++ {
		idx := 16 + i
		sDst := i3.GetOrCreateSubinterface(uint32(idx))
		if !deviations.DeprecatedVlanID(dut) {
			sDst.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(idx))
		} else {
			sDst.GetOrCreateVlan().VlanId = oc.UnionUint16(uint16(idx))
		}
		dutA, _ := getDUTTPEgressAttrs(idx)
		sDst.GetOrCreateIpv6().GetOrCreateAddress(dutA.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)
		if deviations.InterfaceEnabled(dut) {
			sDst.GetOrCreateIpv6().Enabled = ygot.Bool(true)
		}
		ni := &oc.NetworkInstance{Name: ygot.String(fmt.Sprintf("VRF-V6-%d", i))}
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ni.Name).Config(), ni)
	}
	gnmi.Update(t, dut, d.Interface(p3.Name()).Config(), i3)
	for i := 1; i <= 5; i++ {
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), fmt.Sprintf("VRF-V4-%d", 10+i), uint32(11+i))
	}
	for i := 1; i <= 5; i++ {
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), fmt.Sprintf("VRF-V6-%d", i), uint32(16+i))
	}
	gnmi.Update(t, dut, d.Interface(p3.Name()).Config(), i3)

	// Port 4
	p4 := dut.Port(t, "port4")
	i4 := &oc.Interface{Name: ygot.String(p4.Name()), Description: ygot.String("Egress Port-4"), Type: oc.IETFInterfaces_InterfaceType_ethernetCsmacd}
	if deviations.InterfaceEnabled(dut) {
		i4.Enabled = ygot.Bool(true)
	}
	for i := 1; i <= 10; i++ {
		idx := 21 + i
		sDst := i4.GetOrCreateSubinterface(uint32(idx))
		if !deviations.DeprecatedVlanID(dut) {
			sDst.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(idx))
		} else {
			sDst.GetOrCreateVlan().VlanId = oc.UnionUint16(uint16(idx))
		}
		dutA, _ := getDUTTPEgressAttrs(idx)
		sDst.GetOrCreateIpv6().GetOrCreateAddress(dutA.IPv6).PrefixLength = ygot.Uint8(ipv6PrefixLen)
		if deviations.InterfaceEnabled(dut) {
			sDst.GetOrCreateIpv6().Enabled = ygot.Bool(true)
		}
		ni := &oc.NetworkInstance{Name: ygot.String(fmt.Sprintf("VRF-V6-%d", 5+i))}
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ni.Name).Config(), ni)
	}
	gnmi.Update(t, dut, d.Interface(p4.Name()).Config(), i4)
	for i := 1; i <= 10; i++ {
		fptest.AssignToNetworkInstance(t, dut, p4.Name(), fmt.Sprintf("VRF-V6-%d", 5+i), uint32(21+i))
	}
	gnmi.Update(t, dut, d.Interface(p4.Name()).Config(), i4)

	// 30 Default Route configs
	t.Log("Configuring static routes in VRFs")
	b := &gnmi.SetBatch{}
	for i := 1; i <= 15; i++ { // For 15 V4 VRFs
		niName := fmt.Sprintf("VRF-V4-%d", i)
		dstNet := fmt.Sprintf("198.18.%d.0/24", i)
		_, ateA := getDUTTPEgressAttrs(i)
		if i > 10 {
			_, ateA = getDUTTPEgressAttrs(i + 1) // offset for default subint
		}
		cfgplugins.NewStaticRouteCfg(b, &cfgplugins.StaticRouteCfg{
			NetworkInstance: niName,
			Prefix:          dstNet,
			NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(ateA.IPv4)},
		}, dut)
	}
	for i := 1; i <= 15; i++ { // For 15 V6 VRFs
		niName := fmt.Sprintf("VRF-V6-%d", i)
		dstNet := fmt.Sprintf("2001:db8:a:%x::/64", i)
		offset := 16 + i
		if i > 5 {
			offset = 21 + (i - 5)
		}
		_, ateA := getDUTTPEgressAttrs(offset)
		cfgplugins.NewStaticRouteCfg(b, &cfgplugins.StaticRouteCfg{
			NetworkInstance: niName,
			Prefix:          dstNet,
			NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(ateA.IPv6)},
		}, dut)
	}

	// Default route in Default NI
	_, ateADefault := getDUTTPEgressAttrs(11)
	cfgplugins.NewStaticRouteCfg(b, &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          "0.0.0.0/0",
		NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(ateADefault.IPv4)},
	}, dut)
	cfgplugins.NewStaticRouteCfg(b, &cfgplugins.StaticRouteCfg{
		NetworkInstance: deviations.DefaultNetworkInstance(dut),
		Prefix:          "::/0",
		NextHops:        map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{"0": oc.UnionString(ateADefault.IPv6)},
	}, dut)
	b.Set(t, dut)
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Log("Configuring OTG")
	topo := gosnappi.NewConfig()

	// Port 1 Ingress
	p1 := ate.Port(t, "port1")
	topo.Ports().Add().SetName(p1.ID())
	d1 := topo.Devices().Add().SetName(atePort1.Name)
	eth1 := d1.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	eth1.Connection().SetPortName(p1.ID())
	eth1.Ipv4Addresses().Add().SetName(d1.Name() + ".IPv4").SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	eth1.Ipv6Addresses().Add().SetName(d1.Name() + ".IPv6").SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	// Ports 2, 3, 4 Egress
	pEgressIds := []string{ate.Port(t, "port2").ID(), ate.Port(t, "port3").ID(), ate.Port(t, "port4").ID()}
	for i, pName := range pEgressIds {
		topo.Ports().Add().SetName(pName)
		portNum := i + 2
		startIdx := 1
		endIdx := 10
		if portNum == 2 {
			endIdx = 11
		} else if portNum == 3 {
			startIdx = 12
			endIdx = 21
		} else if portNum == 4 {
			startIdx = 22
			endIdx = 31
		}

		for idx := startIdx; idx <= endIdx; idx++ {
			dutA, ateA := getDUTTPEgressAttrs(idx)
			dev := topo.Devices().Add().SetName(ateA.Name)
			eth := dev.Ethernets().Add().SetName(ateA.Name + ".Eth").SetMac(ateA.MAC)
			eth.Connection().SetPortName(pName)
			eth.Vlans().Add().SetName(dev.Name() + "-VLAN").SetId(uint32(idx))
			if idx <= 16 {
				eth.Ipv4Addresses().Add().SetName(dev.Name() + ".IPv4").SetAddress(ateA.IPv4).SetGateway(dutA.IPv4).SetPrefix(uint32(ateA.IPv4Len))
			}
			if idx == 11 || idx >= 17 {
				eth.Ipv6Addresses().Add().SetName(dev.Name() + ".IPv6").SetAddress(ateA.IPv6).SetGateway(dutA.IPv6).SetPrefix(uint32(ateA.IPv6Len))
			}
		}
	}

	// Positive flows (30)
	for i := 1; i <= 15; i++ {
		flowName := fmt.Sprintf("PositiveFlow-V4-%d", i)
		flow := topo.Flows().Add().SetName(flowName)
		flow.Metrics().SetEnable(true)
		idx := i
		if i >= 11 {
			idx = i + 1
		}
		rxName := fmt.Sprintf("ate_port_egress_%d.IPv4", idx)
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{rxName})
		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(atePort1.MAC)

		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue(fmt.Sprintf("198.51.100.%d", i))
		ip.Dst().SetValue(fmt.Sprintf("198.18.%d.10", i))

		ipInIP := flow.Packet().Add().Ipv4() // Inner IP
		ipInIP.Src().SetValue("198.51.100.200")
		ipInIP.Dst().SetValue(fmt.Sprintf("198.18.%d.10", i))
		flow.Size().SetFixed(256)
		flow.Rate().SetPercentage(0.5)
	}
	for i := 1; i <= 15; i++ {
		flowName := fmt.Sprintf("PositiveFlow-V6-%d", i)
		flow := topo.Flows().Add().SetName(flowName)
		flow.Metrics().SetEnable(true)
		rxName := fmt.Sprintf("ate_port_egress_%d.IPv6", i+16)
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{rxName})
		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(atePort1.MAC)

		ip := flow.Packet().Add().Ipv6()
		ip.Src().SetValue(fmt.Sprintf("2001:db8:100::%d", i))
		ip.Dst().SetValue(fmt.Sprintf("2001:db8:a:%x::10", i))

		ipInIP := flow.Packet().Add().Ipv6() // Inner IP
		ipInIP.Src().SetValue("2001:db8:100:1::1")
		ipInIP.Dst().SetValue(fmt.Sprintf("2001:db8:a:%x::10", i))
		flow.Size().SetFixed(256)
		flow.Rate().SetPercentage(0.5)
	}

	// Negative flows (30) missing outer IPinIP
	for i := 1; i <= 15; i++ {
		flowName := fmt.Sprintf("NegativeFlow-V4-%d", i)
		flow := topo.Flows().Add().SetName(flowName)
		flow.Metrics().SetEnable(true)
		rxName := "ate_port_egress_11.IPv4" // Default NI
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{rxName})
		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(atePort1.MAC)

		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue(fmt.Sprintf("198.51.100.%d", i))
		ip.Dst().SetValue(fmt.Sprintf("198.18.%d.10", i)) // Should fall through to default NI

		udp := flow.Packet().Add().Udp()
		udp.SrcPort().SetValue(1024)
		udp.DstPort().SetValue(1024)

		flow.Size().SetFixed(256)
		flow.Rate().SetPercentage(0.5)
	}
	for i := 1; i <= 15; i++ {
		flowName := fmt.Sprintf("NegativeFlow-V6-%d", i)
		flow := topo.Flows().Add().SetName(flowName)
		flow.Metrics().SetEnable(true)
		rxName := "ate_port_egress_11.IPv6" // Default NI
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{rxName})
		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(atePort1.MAC)

		ip := flow.Packet().Add().Ipv6()
		ip.Src().SetValue(fmt.Sprintf("2001:db8:100::%d", i))
		ip.Dst().SetValue(fmt.Sprintf("2001:db8:a:%x::10", i)) // Should fall through to default NI

		udp := flow.Packet().Add().Udp()
		udp.SrcPort().SetValue(1024)
		udp.DstPort().SetValue(1024)

		flow.Size().SetFixed(256)
		flow.Rate().SetPercentage(0.5)
	}

	// Ghost flow
	{
		flowName := "GhostFlow"
		flow := topo.Flows().Add().SetName(flowName)
		flow.Metrics().SetEnable(true)
		rxName := "ate_port_egress_11.IPv4" // Expected loss, send any valid rx
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{rxName})
		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(atePort1.MAC)
		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue("198.51.100.31")
		ip.Dst().SetValue("198.18.31.10")
		ipInIP := flow.Packet().Add().Ipv4() // Inner IP
		ipInIP.Src().SetValue("198.51.100.200")
		ipInIP.Dst().SetValue("198.18.31.10")
		flow.Size().SetFixed(256)
		flow.Rate().SetPercentage(0.5)
	}

	// Shadow flow
	{
		flowName := "ShadowFlow"
		flow := topo.Flows().Add().SetName(flowName)
		flow.Metrics().SetEnable(true)
		rxName := "ate_port_egress_16.IPv4" // Expected to map to VRF-V4-15
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{rxName})
		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(atePort1.MAC)
		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue("198.51.100.100") // No match for rules 1-15, matches 100
		ip.Dst().SetValue("198.18.15.10")
		ipInIP := flow.Packet().Add().Ipv4() // Inner IP
		ipInIP.Src().SetValue("198.51.100.200")
		ipInIP.Dst().SetValue("198.18.15.10")
		flow.Size().SetFixed(256)
		flow.Rate().SetPercentage(0.5)
	}

	return topo
}

var (
	baseOutPkts = make(map[string]float32)
	baseInPkts  = make(map[string]float32)
)

func verifyTraffic(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, expectedLoss map[string]bool) {
	otgutils.LogFlowMetrics(t, ate.OTG(), topo)
	for flowName, wantLoss := range expectedLoss {
		val, ok := gnmi.Watch(t, ate.OTG(), gnmi.OTG().Flow(flowName).State(), 1*time.Minute, func(val *ygnmi.Value[*gnmiotg.Flow]) bool {
			flowMetrics, present := val.Val()
			if !present || flowMetrics.GetCounters() == nil {
				return false
			}
			inPkts := float32(flowMetrics.GetCounters().GetInPkts()) - baseInPkts[flowName]
			outPkts := float32(flowMetrics.GetCounters().GetOutPkts()) - baseOutPkts[flowName]
			if outPkts <= 0 {
				return false
			}
			lossPct := (outPkts - inPkts) / outPkts * 100
			if wantLoss {
				return lossPct >= 80
			}
			return lossPct <= 15
		}).Await(t)
		if !ok {
			t.Errorf("Flow %s failed to reach expected loss condition (wantLoss: %v)", flowName, wantLoss)
			if flowMetrics, present := val.Val(); present && (flowMetrics.GetCounters() != nil) {
				inPkts := float32(flowMetrics.GetCounters().GetInPkts()) - baseInPkts[flowName]
				outPkts := float32(flowMetrics.GetCounters().GetOutPkts()) - baseOutPkts[flowName]
				t.Errorf("Flow %s interval stats: outPkts=%v inPkts=%v", flowName, outPkts, inPkts)
			}
		}
	}

	for flowName := range expectedLoss {
		if flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flowName).State()); flowMetrics != nil && flowMetrics.GetCounters() != nil {
			baseOutPkts[flowName] = float32(flowMetrics.GetCounters().GetOutPkts())
			baseInPkts[flowName] = float32(flowMetrics.GetCounters().GetInPkts())
		}
	}
}

func TestVrfSelectionResiliency(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	configureDUT(t, dut)
	topo := configureATE(t, ate)

	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	// Await DUT ports to be UP
	for _, p := range []string{"port1", "port2", "port3", "port4"} {
		gnmi.Await(t, dut, gnmi.OC().Interface(dut.Port(t, p).Name()).OperStatus().State(), 2*time.Minute, oc.Interface_OperStatus_UP)
	}
	time.Sleep(15 * time.Second)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")

	// Step 1: Configure Massive VRF Policy
	vrfpolicy.ConfigureHA_VRFSelectionPolicy(t, dut, "HA_VRF_SELECTION")

	// Delete Ghost VRF to test missing reference handling
	t.Log("Deleting VRF-GHOST")
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("VRF-GHOST").Config())

	expectedLoss := make(map[string]bool)
	for i := 1; i <= 15; i++ {
		expectedLoss[fmt.Sprintf("PositiveFlow-V4-%d", i)] = false
		expectedLoss[fmt.Sprintf("NegativeFlow-V4-%d", i)] = false
		if dut.Vendor() != ondatra.CISCO {
			expectedLoss[fmt.Sprintf("PositiveFlow-V6-%d", i)] = false
			expectedLoss[fmt.Sprintf("NegativeFlow-V6-%d", i)] = false
		}
	}
	expectedLoss["GhostFlow"] = true   // VRF-GHOST doesn't exist, we assume drops
	expectedLoss["ShadowFlow"] = false // Match rule 100 which routes to VRF-V4-15 which is valid

	// RT-3.4.1 Validation
	t.Run("VRF Selection Policy Programming", func(t *testing.T) {
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)
		ate.OTG().StopTraffic(t)
		verifyTraffic(t, ate, topo, expectedLoss)
	})

	// RT-3.4.2 - VRF Selection Policy Resilience Post Supervisor Switchover
	t.Run("RT-3.4.2 - VRF Selection Policy Resilience Post Supervisor Switchover", func(t *testing.T) {
		t.Log("Step 1 - Trigger Switchover")
		supervisors := cmp.FindComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD)
		t.Logf("Found controller cards: %v", supervisors)
		if len(supervisors) < 2 {
			t.Skipf("Dual controllers required on %v: got %v, want 2", dut.Model(), len(supervisors))
		}
		secondary, primary := cmp.FindStandbyControllerCard(t, dut, supervisors)
		cmp.AwaitSwitchoverReady(t, dut, primary, 5*time.Minute)

		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration) // Run traffic prior to and during switchover

		switchoverReq := system.NewSwitchControlProcessorOperation().Path(cmp.GetSubcomponentPath(secondary, deviations.GNOISubcomponentPath(dut)))
		switchoverResp := gnoi.Execute(t, dut, switchoverReq)
		t.Logf("gnoi.Execute SwitchControlProcessor response: %v", switchoverResp)

		t.Log("Waiting for new Primary controller to become active...")
		gnmi.Watch(t, dut, gnmi.OC().Component(secondary).RedundantRole().State(), 30*time.Minute, func(val *ygnmi.Value[oc.E_Platform_ComponentRedundantRole]) bool {
			role, present := val.Val()
			return present && role == oc.Platform_ComponentRedundantRole_PRIMARY
		}).Await(t)

		t.Log("Validating supervisor switchover telemetry...")
		secComp := gnmi.OC().Component(secondary)
		if !gnmi.Lookup(t, dut, secComp.LastSwitchoverTime().State()).IsPresent() {
			t.Errorf("Component %s last-switchover-time telemetry is missing", secondary)
		} else {
			t.Logf("Component %s last-switchover-time: %v", secondary, gnmi.Get(t, dut, secComp.LastSwitchoverTime().State()))
		}

		if !gnmi.Lookup(t, dut, secComp.LastSwitchoverReason().State()).IsPresent() {
			t.Errorf("Component %s last-switchover-reason telemetry is missing", secondary)
		} else {
			reason := gnmi.Get(t, dut, secComp.LastSwitchoverReason().State())
			t.Logf("Component %s last-switchover-reason trigger: %v, details: %v", secondary, reason.GetTrigger(), reason.GetDetails())
			wantTrigger := oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_USER_INITIATED
			if deviations.GNOISwitchoverReasonMissingUserInitiated(dut) {
				wantTrigger = oc.PlatformTypes_ComponentRedundantRoleSwitchoverReasonTrigger_SYSTEM_INITIATED
			}
			if got := reason.GetTrigger(); got != wantTrigger {
				t.Errorf("Component %s last-switchover-reason trigger got %v, want %v", secondary, got, wantTrigger)
			}
		}

		t.Log("Step 2 - Validation")
		time.Sleep(trafficDuration) // Run traffic post switchover completion
		ate.OTG().StopTraffic(t)
		verifyTraffic(t, ate, topo, expectedLoss)
	})

	p1 := dut.Port(t, "port1")

	// RT-3.4.3 - VRF Selection Policy Resilience Post Linecard OIR
	t.Run("RT-3.4.3 - VRF Selection Policy Resilience Post Linecard OIR", func(t *testing.T) {
		t.Log("Step 1 - Perform Linecard Soft OIR")
		// Find linecard for Port 1
		p1Component := gnmi.Get(t, dut, gnmi.OC().Interface(p1.Name()).HardwarePort().State())
		lcComponent := cfgplugins.FindLineCardParent(t, dut, p1Component)

		// Start continuous traffic prior to disabling linecard as required by README
		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)

		t.Logf("Disabling linecard %s", lcComponent)
		if deviations.PowerDisableEnableLeafRefValidation(dut) {
			gnmi.Update(t, dut, gnmi.OC().Component(lcComponent).Config(), &oc.Component{Name: ygot.String(lcComponent)})
		}
		gnmi.Replace(t, dut, gnmi.OC().Component(lcComponent).Linecard().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_DISABLED)
		gnmi.Await(t, dut, gnmi.OC().Component(lcComponent).Linecard().PowerAdminState().State(), 5*time.Minute, oc.Platform_ComponentPowerType_POWER_DISABLED)
		gnmi.Await(t, dut, gnmi.OC().Component(lcComponent).OperStatus().State(), 5*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_DISABLED)

		t.Log("Verifying traffic drops while linecard is disabled...")
		droppedLoss := make(map[string]bool)
		for flowName := range expectedLoss {
			droppedLoss[flowName] = true
		}
		verifyTraffic(t, ate, topo, droppedLoss)

		t.Logf("Enabling linecard %s", lcComponent)
		if deviations.PowerDisableEnableLeafRefValidation(dut) {
			gnmi.Update(t, dut, gnmi.OC().Component(lcComponent).Config(), &oc.Component{Name: ygot.String(lcComponent)})
		}
		gnmi.Replace(t, dut, gnmi.OC().Component(lcComponent).Linecard().PowerAdminState().Config(), oc.Platform_ComponentPowerType_POWER_ENABLED)
		gnmi.Await(t, dut, gnmi.OC().Component(lcComponent).Linecard().PowerAdminState().State(), 10*time.Minute, oc.Platform_ComponentPowerType_POWER_ENABLED)
		gnmi.Await(t, dut, gnmi.OC().Component(lcComponent).OperStatus().State(), 10*time.Minute, oc.PlatformTypes_COMPONENT_OPER_STATUS_ACTIVE)
		gnmi.Await(t, dut, gnmi.OC().Interface(p1.Name()).OperStatus().State(), 5*time.Minute, oc.Interface_OperStatus_UP)

		t.Log("Step 2 - Validation: verifying autonomous recovery")
		time.Sleep(trafficDuration)
		ate.OTG().StopTraffic(t)
		verifyTraffic(t, ate, topo, expectedLoss)
	})

	t.Run("RT-3.4.4 - Policy Deletion", func(t *testing.T) {
		t.Log("Step 1 & 2 - Delete VRF Selection Policy")
		vrfpolicy.DeletePolicyForwarding(t, dut, p1.ID())
		t.Log("Sleeping 30s to allow hardware to digest TCAM deletion before traffic validation...")
		time.Sleep(30 * time.Second)

		t.Log("Step 3 - Validation")
		// Per README: Verify that all previously matching Positive Streams are no longer
		// steered to their specific VRFs and correctly revert to using the DEFAULT VRF routing
		// table (egressing DUT Port-2's default sub-interface).
		for _, flow := range topo.Flows().Items() {
			if strings.Contains(flow.Name(), "V6") {
				flow.TxRx().Device().SetRxNames([]string{"ate_port_egress_11.IPv6"})
			} else {
				flow.TxRx().Device().SetRxNames([]string{"ate_port_egress_11.IPv4"})
			}
		}
		ate.OTG().PushConfig(t, topo)

		ate.OTG().StartTraffic(t)
		time.Sleep(trafficDuration)
		ate.OTG().StopTraffic(t)

		expectedLossAfterDelete := make(map[string]bool)
		for i := 1; i <= 15; i++ {
			expectedLossAfterDelete[fmt.Sprintf("PositiveFlow-V4-%d", i)] = false
			expectedLossAfterDelete[fmt.Sprintf("NegativeFlow-V4-%d", i)] = false
			if dut.Vendor() != ondatra.CISCO {
				expectedLossAfterDelete[fmt.Sprintf("PositiveFlow-V6-%d", i)] = false
				expectedLossAfterDelete[fmt.Sprintf("NegativeFlow-V6-%d", i)] = false
			}
		}
		expectedLossAfterDelete["GhostFlow"] = false  // Ghost flow previously dropped, now matches DEFAULT route!
		expectedLossAfterDelete["ShadowFlow"] = false // Shadow flow now routes via DEFAULT VRF to subinterface 11

		verifyTraffic(t, ate, topo, expectedLossAfterDelete)
	})
}
