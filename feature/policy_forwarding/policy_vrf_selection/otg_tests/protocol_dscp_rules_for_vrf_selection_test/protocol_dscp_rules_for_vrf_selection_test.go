/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package policy_based_vrf_selection_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
)

const (
	trafficDuration = 1 * time.Minute
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
)

// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut      *ondatra.DUTDevice
	ate      *ondatra.ATEDevice
	top      gosnappi.Config
	iptype   string
	protocol oc.E_PacketMatchTypes_IP_PROTOCOL
}

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		MAC:     "01:00:01:01:01:01",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		MAC:     "00:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2Vlan10 = attrs.Attributes{
		Desc:    "dutPort2Vlan10",
		MAC:     "01:00:01:01:01:01",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:9",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2Vlan10 = attrs.Attributes{
		Name:    "atePort2Vlan10",
		MAC:     "00:12:01:00:00:01",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:a",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2Vlan20 = attrs.Attributes{
		Desc:    "dutPort2Vlan20",
		MAC:     "01:00:01:01:01:01",
		IPv4:    "192.0.2.13",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:d",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2Vlan20 = attrs.Attributes{
		Name:    "atePort2Vlan20",
		MAC:     "00:12:01:00:00:01",
		IPv4:    "192.0.2.14",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:e",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2Vlan30 = attrs.Attributes{
		Desc:    "dutPort2Vlan30",
		MAC:     "01:00:01:01:01:01",
		IPv4:    "192.0.2.17",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:11",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2Vlan30 = attrs.Attributes{
		Name:    "atePort2Vlan30",
		MAC:     "00:12:01:00:00:01",
		IPv4:    "192.0.2.18",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:12",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		MAC:     "03:00:01:01:01:01",
		IPv4:    "192.0.2.21",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:21",
		IPv6Len: ipv6PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		MAC:     "00:12:01:00:00:03",
		IPv4:    "192.0.2.22",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:22",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		MAC:     "04:00:01:01:01:01",
		IPv4:    "192.0.2.33",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:33",
		IPv6Len: ipv6PrefixLen,
	}

	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		MAC:     "00:12:01:00:00:04",
		IPv4:    "192.0.2.34",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:34",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort4Vlan40 = attrs.Attributes{
		Desc:    "dutPort4Vlan40",
		MAC:     "04:00:01:01:01:01",
		IPv4:    "192.0.2.37",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:37",
		IPv6Len: ipv6PrefixLen,
	}

	atePort4Vlan40 = attrs.Attributes{
		Name:    "atePort4Vlan40",
		MAC:     "00:12:01:00:00:04",
		IPv4:    "192.0.2.38",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:38",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort4Vlan50 = attrs.Attributes{
		Desc:    "dutPort4Vlan50",
		MAC:     "04:00:01:01:01:01",
		IPv4:    "192.0.2.41",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:41",
		IPv6Len: ipv6PrefixLen,
	}

	atePort4Vlan50 = attrs.Attributes{
		Name:    "atePort4Vlan50",
		MAC:     "00:12:01:00:00:04",
		IPv4:    "192.0.2.42",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:42",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort4Vlan60 = attrs.Attributes{
		Desc:    "dutPort4Vlan60",
		MAC:     "04:00:01:01:01:01",
		IPv4:    "192.0.2.45",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:45",
		IPv6Len: ipv6PrefixLen,
	}

	atePort4Vlan60 = attrs.Attributes{
		Name:    "atePort4Vlan60",
		MAC:     "00:12:01:00:00:04",
		IPv4:    "192.0.2.46",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:46",
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// configureATE configures port1, port2, port3, port4 and vlans on port2 and port4 on the ATE.
func configureATE(t *testing.T, ate *ondatra.ATEDevice, dut *ondatra.DUTDevice) gosnappi.Config {
	top := gosnappi.NewConfig()

	// ATE Port 1 (Ingress)
	p1 := ate.Port(t, "port1")
	top.Ports().Add().SetName(p1.ID())
	srcDev1 := top.Devices().Add().SetName(atePort1.Name)
	ethSrc1 := srcDev1.Ethernets().Add().SetName(atePort1.Name + ".eth").SetMac(atePort1.MAC)
	ethSrc1.Connection().SetPortName(p1.ID())
	ethSrc1.Ipv4Addresses().Add().SetName(srcDev1.Name() + ".ipv4").SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	ethSrc1.Ipv6Addresses().Add().SetName(srcDev1.Name() + ".ipv6").SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	// ATE Port 2 (Egress for Port 1)
	p2 := ate.Port(t, "port2")
	top.Ports().Add().SetName(p2.ID())
	dstDev2 := top.Devices().Add().SetName(atePort2.Name)
	ethDst2 := dstDev2.Ethernets().Add().SetName(atePort2.Name + ".eth").SetMac(atePort2.MAC)
	if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
		ethDst2.Vlans().Add().SetName(atePort2.Name + "vlan").SetId(1)
	}
	ethDst2.Connection().SetPortName(p2.ID())
	ethDst2.Ipv4Addresses().Add().SetName(dstDev2.Name() + ".ipv4").SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	ethDst2.Ipv6Addresses().Add().SetName(dstDev2.Name() + ".ipv6").SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	// Configure VLANs on ATE port2
	dstDev2Vlan10 := top.Devices().Add().SetName(atePort2Vlan10.Name)
	ethDst2Vlan10 := dstDev2Vlan10.Ethernets().Add().SetName(atePort2Vlan10.Name + ".eth").SetMac(atePort2Vlan10.MAC)
	ethDst2Vlan10.Connection().SetPortName(p2.ID())
	ethDst2Vlan10.Vlans().Add().SetName(atePort2Vlan10.Name + "vlan").SetId(10)
	ethDst2Vlan10.Ipv4Addresses().Add().SetName(atePort2Vlan10.Name + ".ipv4").SetAddress(atePort2Vlan10.IPv4).SetGateway(dutPort2Vlan10.IPv4).SetPrefix(uint32(atePort2Vlan10.IPv4Len))
	ethDst2Vlan10.Ipv6Addresses().Add().SetName(atePort2Vlan10.Name + ".ipv6").SetAddress(atePort2Vlan10.IPv6).SetGateway(dutPort2Vlan10.IPv6).SetPrefix(uint32(atePort2Vlan10.IPv6Len))

	dstDev2Vlan20 := top.Devices().Add().SetName(atePort2Vlan20.Name)
	ethDst2Vlan20 := dstDev2Vlan20.Ethernets().Add().SetName(atePort2Vlan20.Name + ".eth").SetMac(atePort2Vlan20.MAC)
	ethDst2Vlan20.Connection().SetPortName(p2.ID())
	ethDst2Vlan20.Vlans().Add().SetName(atePort2Vlan20.Name + "vlan").SetId(20)
	ethDst2Vlan20.Ipv4Addresses().Add().SetName(atePort2Vlan20.Name + ".ipv4").SetAddress(atePort2Vlan20.IPv4).SetGateway(dutPort2Vlan20.IPv4).SetPrefix(uint32(atePort2Vlan20.IPv4Len))
	ethDst2Vlan20.Ipv6Addresses().Add().SetName(atePort2Vlan20.Name + ".ipv6").SetAddress(atePort2Vlan20.IPv6).SetGateway(dutPort2Vlan20.IPv6).SetPrefix(uint32(atePort2Vlan20.IPv6Len))

	dstDev2Vlan30 := top.Devices().Add().SetName(atePort2Vlan30.Name)
	ethDst2Vlan30 := dstDev2Vlan30.Ethernets().Add().SetName(atePort2Vlan30.Name + ".eth").SetMac(atePort2Vlan30.MAC)
	ethDst2Vlan30.Connection().SetPortName(p2.ID())
	ethDst2Vlan30.Vlans().Add().SetName(atePort2Vlan30.Name + "vlan").SetId(30)
	ethDst2Vlan30.Ipv4Addresses().Add().SetName(atePort2Vlan30.Name + ".ipv4").SetAddress(atePort2Vlan30.IPv4).SetGateway(dutPort2Vlan30.IPv4).SetPrefix(uint32(atePort2Vlan30.IPv4Len))
	ethDst2Vlan30.Ipv6Addresses().Add().SetName(atePort2Vlan30.Name + ".ipv6").SetAddress(atePort2Vlan30.IPv6).SetGateway(dutPort2Vlan30.IPv6).SetPrefix(uint32(atePort2Vlan30.IPv6Len))

	// ATE Port 3 (Ingress)
	p3 := ate.Port(t, "port3")
	top.Ports().Add().SetName(p3.ID())
	srcDev3 := top.Devices().Add().SetName(atePort3.Name)
	ethSrc3 := srcDev3.Ethernets().Add().SetName(atePort3.Name + ".eth").SetMac(atePort3.MAC)
	ethSrc3.Connection().SetPortName(p3.ID())
	ethSrc3.Ipv4Addresses().Add().SetName(srcDev3.Name() + ".ipv4").SetAddress(atePort3.IPv4).SetGateway(dutPort3.IPv4).SetPrefix(uint32(atePort3.IPv4Len))
	ethSrc3.Ipv6Addresses().Add().SetName(srcDev3.Name() + ".ipv6").SetAddress(atePort3.IPv6).SetGateway(dutPort3.IPv6).SetPrefix(uint32(atePort3.IPv6Len))

	// ATE Port 4 (Egress for Port 3)
	p4 := ate.Port(t, "port4")
	top.Ports().Add().SetName(p4.ID())
	dstDev4 := top.Devices().Add().SetName(atePort4.Name)
	ethDst4 := dstDev4.Ethernets().Add().SetName(atePort4.Name + ".eth").SetMac(atePort4.MAC)
	if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
		ethDst4.Vlans().Add().SetName(atePort4.Name + "vlan").SetId(1)
	}
	ethDst4.Connection().SetPortName(p4.ID())
	ethDst4.Ipv4Addresses().Add().SetName(dstDev4.Name() + ".ipv4").SetAddress(atePort4.IPv4).SetGateway(dutPort4.IPv4).SetPrefix(uint32(atePort4.IPv4Len))
	ethDst4.Ipv6Addresses().Add().SetName(dstDev4.Name() + ".ipv6").SetAddress(atePort4.IPv6).SetGateway(dutPort4.IPv6).SetPrefix(uint32(atePort4.IPv6Len))

	// Configure VLANs on ATE port4
	dstDev4Vlan40 := top.Devices().Add().SetName(atePort4Vlan40.Name)
	ethDst4Vlan40 := dstDev4Vlan40.Ethernets().Add().SetName(atePort4Vlan40.Name + ".eth").SetMac(atePort4Vlan40.MAC)
	ethDst4Vlan40.Connection().SetPortName(p4.ID())
	ethDst4Vlan40.Vlans().Add().SetName(atePort4Vlan40.Name + "vlan").SetId(40)
	ethDst4Vlan40.Ipv4Addresses().Add().SetName(atePort4Vlan40.Name + ".ipv4").SetAddress(atePort4Vlan40.IPv4).SetGateway(dutPort4Vlan40.IPv4).SetPrefix(uint32(atePort4Vlan40.IPv4Len))
	ethDst4Vlan40.Ipv6Addresses().Add().SetName(atePort4Vlan40.Name + ".ipv6").SetAddress(atePort4Vlan40.IPv6).SetGateway(dutPort4Vlan40.IPv6).SetPrefix(uint32(atePort4Vlan40.IPv6Len))

	dstDev4Vlan50 := top.Devices().Add().SetName(atePort4Vlan50.Name)
	ethDst4Vlan50 := dstDev4Vlan50.Ethernets().Add().SetName(atePort4Vlan50.Name + ".eth").SetMac(atePort4Vlan50.MAC)
	ethDst4Vlan50.Connection().SetPortName(p4.ID())
	ethDst4Vlan50.Vlans().Add().SetName(atePort4Vlan50.Name + "vlan").SetId(50)
	ethDst4Vlan50.Ipv4Addresses().Add().SetName(atePort4Vlan50.Name + ".ipv4").SetAddress(atePort4Vlan50.IPv4).SetGateway(dutPort4Vlan50.IPv4).SetPrefix(uint32(atePort4Vlan50.IPv4Len))
	ethDst4Vlan50.Ipv6Addresses().Add().SetName(atePort4Vlan50.Name + ".ipv6").SetAddress(atePort4Vlan50.IPv6).SetGateway(dutPort4Vlan50.IPv6).SetPrefix(uint32(atePort4Vlan50.IPv6Len))

	dstDev4Vlan60 := top.Devices().Add().SetName(atePort4Vlan60.Name)
	ethDst4Vlan60 := dstDev4Vlan60.Ethernets().Add().SetName(atePort4Vlan60.Name + ".eth").SetMac(atePort4Vlan60.MAC)
	ethDst4Vlan60.Connection().SetPortName(p4.ID())
	ethDst4Vlan60.Vlans().Add().SetName(atePort4Vlan60.Name + "vlan").SetId(60)
	ethDst4Vlan60.Ipv4Addresses().Add().SetName(atePort4Vlan60.Name + ".ipv4").SetAddress(atePort4Vlan60.IPv4).SetGateway(dutPort4Vlan60.IPv4).SetPrefix(uint32(atePort4Vlan60.IPv4Len))
	ethDst4Vlan60.Ipv6Addresses().Add().SetName(atePort4Vlan60.Name + ".ipv6").SetAddress(atePort4Vlan60.IPv6).SetGateway(dutPort4Vlan60.IPv6).SetPrefix(uint32(atePort4Vlan60.IPv6Len))

	return top
}

// configNetworkInstance creates VRFs and subinterfaces and then applies VRFs on the subinterfaces.
func configNetworkInstance(t *testing.T, dut *ondatra.DUTDevice, vrfname string, intfname string, subint uint32, vlanID uint16) {
	// create empty subinterface
	si := &oc.Interface_Subinterface{}
	si.Index = ygot.Uint32(subint)
	if deviations.DeprecatedVlanID(dut) {
		si.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
	} else {
		si.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
	}
	s4 := si.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	gnmi.Replace(t, dut, gnmi.OC().Interface(intfname).Subinterface(subint).Config(), si)

	// create vrf and apply on subinterface
	v := &oc.NetworkInstance{
		Name: ygot.String(vrfname),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
	}
	vi := v.GetOrCreateInterface(intfname + "." + strconv.Itoa(int(subint)))
	vi.Interface = ygot.String(intfname)
	vi.Subinterface = ygot.Uint32(subint)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfname).Config(), v)
}

// getSubInterface returns a subinterface configuration populated with IP addresses and VLAN ID.
func getSubInterface(dutPort *attrs.Attributes, index uint32, vlanID uint16, dut *ondatra.DUTDevice) *oc.Interface_Subinterface {
	s := &oc.Interface_Subinterface{}
	// unshut sub/interface
	if deviations.InterfaceEnabled(dut) {
		s.Enabled = ygot.Bool(true)
	}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a := s4.GetOrCreateAddress(dutPort.IPv4)
	a.PrefixLength = ygot.Uint8(dutPort.IPv4Len)
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	a6 := s6.GetOrCreateAddress(dutPort.IPv6)
	a6.PrefixLength = ygot.Uint8(dutPort.IPv6Len)
	if index != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
		}
	}
	return s
}

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *oc.Interface, dutPort *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	i.Description = ygot.String(dutPort.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	i.AppendSubinterface(getSubInterface(dutPort, 0, 0, dut))
	return i
}

// configureDUT configures the base configuration on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	// DUT Port 1 (Ingress)
	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutPort1, dut))

	// DUT Port 2 (Egress for Port 1)
	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutPort2, dut))
	if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
		i3 := &oc.Interface{Name: ygot.String(p2.Name())}
		s := i3.GetOrCreateSubinterface(0)
		s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(1)
		gnmi.Update(t, dut, d.Interface(p2.Name()).Config(), i3)
	}

	outpath2 := d.Interface(p2.Name())
	// create VRFs and VRF enabled subinterfaces for port2
	configNetworkInstance(t, dut, "VRF10", p2.Name(), uint32(1), 10)
	gnmi.Update(t, dut, outpath2.Subinterface(1).Config(), getSubInterface(&dutPort2Vlan10, 1, 10, dut))

	configNetworkInstance(t, dut, "VRF20", p2.Name(), uint32(2), 20)
	gnmi.Update(t, dut, outpath2.Subinterface(2).Config(), getSubInterface(&dutPort2Vlan20, 2, 20, dut))

	configNetworkInstance(t, dut, "VRF30", p2.Name(), uint32(3), 30)
	gnmi.Update(t, dut, outpath2.Subinterface(3).Config(), getSubInterface(&dutPort2Vlan30, 3, 30, dut))

	// DUT Port 3 (Ingress)
	p3 := dut.Port(t, "port3")
	i4 := &oc.Interface{Name: ygot.String(p3.Name())}
	gnmi.Replace(t, dut, d.Interface(p3.Name()).Config(), configInterfaceDUT(i4, &dutPort3, dut))

	// DUT Port 4 (Egress for Port 3)
	p4 := dut.Port(t, "port4")
	i5 := &oc.Interface{Name: ygot.String(p4.Name())}
	gnmi.Replace(t, dut, d.Interface(p4.Name()).Config(), configInterfaceDUT(i5, &dutPort4, dut))
	if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
		i6 := &oc.Interface{Name: ygot.String(p4.Name())}
		s := i6.GetOrCreateSubinterface(0)
		s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(1)
		gnmi.Update(t, dut, d.Interface(p4.Name()).Config(), i6)
	}

	outpath4 := d.Interface(p4.Name())
	// create VRFs and VRF enabled subinterfaces for port4
	configNetworkInstance(t, dut, "VRF40", p4.Name(), uint32(4), 40)
	gnmi.Update(t, dut, outpath4.Subinterface(4).Config(), getSubInterface(&dutPort4Vlan40, 4, 40, dut))

	configNetworkInstance(t, dut, "VRF50", p4.Name(), uint32(5), 50)
	gnmi.Update(t, dut, outpath4.Subinterface(5).Config(), getSubInterface(&dutPort4Vlan50, 5, 50, dut))

	configNetworkInstance(t, dut, "VRF60", p4.Name(), uint32(6), 60)
	gnmi.Update(t, dut, outpath4.Subinterface(6).Config(), getSubInterface(&dutPort4Vlan60, 6, 60, dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		fptest.SetPortSpeed(t, p2)
		fptest.SetPortSpeed(t, p3)
		fptest.SetPortSpeed(t, p4)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p3.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p4.Name(), deviations.DefaultNetworkInstance(dut), 0) // Assign port4 to default VRF
	}
}

// getIPinIPFlow returns an IPv4inIPv4 *ondatra.Flow with provided DSCP value for a given set of endpoints.
func getIPinIPFlow(args *testArgs, src attrs.Attributes, dst attrs.Attributes, flowName string, dscp uint32) gosnappi.Flow {

	flow := gosnappi.NewFlow().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.TxRx().Device().SetTxNames([]string{src.Name + "." + args.iptype}).SetRxNames([]string{dst.Name + "." + args.iptype})
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(src.MAC)
	outerIPHeader := flow.Packet().Add().Ipv4()
	outerIPHeader.Src().SetValue(src.IPv4)
	outerIPHeader.Dst().SetValue(dst.IPv4)
	outerIPHeader.Priority().Dscp().Phb().SetValue(dscp)
	innerIPHeader := flow.Packet().Add().Ipv4()
	innerIPHeader.Src().SetValue("198.51.100.1")
	innerIPHeader.Dst().Increment().SetStart("203.0.113.1").SetStep("0.0.0.1").SetCount(10000)

	flow.Size().SetFixed(1024)
	flow.Rate().SetPps(100)
	flow.Duration().FixedPackets().SetPackets(100)

	return flow
}

// testTrafficFlows verifies traffic for one or more flows.
func testTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, expectPass bool, flows ...gosnappi.Flow) {

	top.Flows().Clear()
	for _, flow := range flows {
		top.Flows().Append(flow)
	}
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "IPv4")

	t.Logf("*** Starting traffic ...")
	ate.OTG().StartTraffic(t)
	time.Sleep(trafficDuration)
	t.Logf("*** Stop traffic ...")
	ate.OTG().StopTraffic(t)

	if expectPass {
		t.Log("Expecting traffic to pass for the flows")
	} else {
		t.Log("Expecting traffic to fail for the flows")
	}

	otgTop := ate.OTG().GetConfig(t)
	otgutils.LogFlowMetrics(t, ate.OTG(), otgTop)
	for _, flow := range flows {
		t.Run(flow.Name(), func(t *testing.T) {
			t.Logf("*** Verifying %v traffic on OTG ... ", flow.Name())
			outPkts := float32(gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
			inPkts := float32(gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow.Name()).Counters().InPkts().State()))

			if outPkts == 0 {
				t.Fatalf("OutPkts == 0, want >0.")
			}

			lossPct := (outPkts - inPkts) * 100 / outPkts

			// log stats
			t.Log("Flow LossPct: ", lossPct)
			t.Log("Flow InPkts  : ", inPkts)
			t.Log("Flow OutPkts : ", outPkts)

			if (expectPass == true) && (lossPct == 0) {
				t.Logf("Traffic for %v flow is passing as expected", flow.Name())
			} else if (expectPass == false) && (lossPct == 100) {
				t.Logf("Traffic for %v flow is failing as expected", flow.Name())
			} else {
				t.Fatalf("Traffic is not working as expected for flow: %v. LossPct: %f", flow.Name(), lossPct)
			}
		})
	}
}

// getL3PBRRule returns an IPv4 or IPv6 policy-forwarding rule configuration populated with protocol and/or DSCPset information.
func getL3PBRRule(args *testArgs, networkInstance string, index uint32, dscpset []uint8) *oc.NetworkInstance_PolicyForwarding_Policy_Rule {
	r := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	if args.iptype == "ipv4" {
		r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: args.protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv4.DscpSet = dscpset
		}
	} else if args.iptype == "ipv6" {
		r.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: args.protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv6.DscpSet = dscpset
		}
	} else {
		return nil
	}
	return &r

}

// getPBRPolicyForwarding returns pointer to policy-forwarding populated with pbr policy and rules
func getPBRPolicyForwarding(policyName string, rules ...*oc.NetworkInstance_PolicyForwarding_Policy_Rule) *oc.NetworkInstance_PolicyForwarding {
	pf := oc.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(policyName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	for _, rule := range rules {
		p.AppendRule(rule)
	}
	return &pf
}

func TestPBR(t *testing.T) {
	t.Logf("Description: Test RT3.2 with multiple DSCP, IPinIP protocol rule based VRF selection with two ingress/egress pairs.")
	dut := ondatra.DUT(t, "dut")

	// configure DUT
	configureDUT(t, dut)

	// Configure ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate, dut)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// Ingress interface for policies
	port1 := dut.Port(t, "port1")
	port3 := dut.Port(t, "port3")

	cases := []struct {
		name         string
		desc         string
		policy       *oc.NetworkInstance_PolicyForwarding
		policyName   string
		ingressPort  *ondatra.Port
		testArgs     *testArgs
		passingFlows []gosnappi.Flow
		failingFlows []gosnappi.Flow
		rejectable   bool
	}{
		{
			name:        "RT3.2 Case1 - Port 1 Ingress to Port 2 Egress",
			desc:        "Ensure matching IPinIP with DSCP (10 - VRF10, 20- VRF20, 30-VRF30) traffic reaches appropriate VLAN on Port 2 (Egress for Port 1).",
			policyName:  "L3_Port1",
			ingressPort: port1,
			testArgs: &testArgs{
				dut:      dut,
				ate:      ate,
				top:      top,
				iptype:   "ipv4",
				protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
			},
			policy: getPBRPolicyForwarding("L3_Port1",
				getL3PBRRule(&testArgs{iptype: "ipv4", protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP}, "VRF10", 1, []uint8{10}),
				getL3PBRRule(&testArgs{iptype: "ipv4", protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP}, "VRF20", 2, []uint8{20}),
				getL3PBRRule(&testArgs{iptype: "ipv4", protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP}, "VRF30", 3, []uint8{30})),
			passingFlows: []gosnappi.Flow{
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan10, "ipinipd10_p1_p2", 10),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan20, "ipinipd20_p1_p2", 20),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan30, "ipinipd30_p1_p2", 30)},
		},
		{
			name:        "RT3.2 Case2 - Port 1 Ingress to Port 2 Egress (Range DSCP)",
			desc:        "Ensure matching IPinIP with DSCP (10-12 - VRF10, 20-22- VRF20, 30-32-VRF30) traffic reaches appropriate VLAN on Port 2 (Egress for Port 1).",
			policyName:  "L3_Port1",
			ingressPort: port1,
			testArgs: &testArgs{
				dut:      dut,
				ate:      ate,
				top:      top,
				iptype:   "ipv4",
				protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
			},
			policy: getPBRPolicyForwarding("L3_Port1",
				getL3PBRRule(&testArgs{iptype: "ipv4", protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP}, "VRF10", 1, []uint8{10, 11, 12}),
				getL3PBRRule(&testArgs{iptype: "ipv4", protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP}, "VRF20", 2, []uint8{20, 21, 22}),
				getL3PBRRule(&testArgs{iptype: "ipv4", protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP}, "VRF30", 3, []uint8{30, 31, 32})),
			passingFlows: []gosnappi.Flow{
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan10, "ipinipd10_p1_p2", 10),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan10, "ipinipd11_p1_p2", 11),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan10, "ipinipd12_p1_p2", 12),

				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan20, "ipinipd20_p1_p2", 20),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan20, "ipinipd21_p1_p2", 21),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan20, "ipinipd22_p1_p2", 22),

				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan30, "ipinipd30_p1_p2", 30),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan30, "ipinipd31_p1_p2", 31),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan30, "ipinipd32_p1_p2", 32)},
		},
		{
			name:        "RT3.2 Case3 - Port 1 Ingress (Precedence)",
			desc:        "Ensure first matching of IPinIP with DSCP (10-12 - VRF10, 10-12 - VRF20) rule takes precedence on Port 1.",
			policyName:  "L3_Port1",
			ingressPort: port1,
			testArgs: &testArgs{
				dut:      dut,
				ate:      ate,
				top:      top,
				iptype:   "ipv4",
				protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
			},
			policy: getPBRPolicyForwarding("L3_Port1",
				getL3PBRRule(&testArgs{iptype: "ipv4", protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP}, "VRF10", 1, []uint8{10, 11, 12}),
				getL3PBRRule(&testArgs{iptype: "ipv4", protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP}, "VRF20", 2, []uint8{10, 11, 12})),
			passingFlows: []gosnappi.Flow{
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan10, "ipinipd10_p1_p2", 10),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan10, "ipinipd11_p1_p2", 11),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan10, "ipinipd12_p1_p2", 12)},
			failingFlows: []gosnappi.Flow{
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan20, "ipinipd10v20_p1_p2", 10),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan20, "ipinipd11v20_p1_p2", 11),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan20, "ipinipd12v20_p1_p2", 12)},
			rejectable: true,
		},
		{
			name:        "RT3.2 Case4 - Port 1 Ingress (Default Match)",
			desc:        "Ensure matching IPinIP to VRF10, IPinIP with DSCP20 to VRF20 causes unspecified DSCP IPinIP traffic to match VRF10 on Port 1.",
			policyName:  "L3_Port1",
			ingressPort: port1,
			testArgs: &testArgs{
				dut:      dut,
				ate:      ate,
				top:      top,
				iptype:   "ipv4",
				protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
			},
			policy: getPBRPolicyForwarding("L3_Port1",
				getL3PBRRule(&testArgs{iptype: "ipv4", protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP}, "VRF10", 1, []uint8{}),
				getL3PBRRule(&testArgs{iptype: "ipv4", protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP}, "VRF20", 2, []uint8{20})),
			passingFlows: []gosnappi.Flow{
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan10, "ipinipd10_p1_p2", 10),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan10, "ipinipd11_p1_p2", 11),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan10, "ipinipd12_p1_p2", 12)},
			failingFlows: []gosnappi.Flow{
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan20, "ipinipd10v20_p1_p2", 10),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan20, "ipinipd11v20_p1_p2", 11),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan20, "ipinipd12v20_p1_p2", 12),
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan20, "ipinipd20_p1_p2", 20)},
		},
		{
			name:        "RT3.2 Case5 - Multiple Policies (Port 1 and Port 3)",
			desc:        "Verify traffic for both policies simultaneously: Port 1 to Port 2 (VLAN 10) and Port 3 to Port 4 (VLAN 40).",
			policyName:  "L3_Port1", // Second policy (Port 3) will be applied within the test run
			ingressPort: port1,
			testArgs: &testArgs{ // Args for Port 1's flow
				dut:      dut,
				ate:      ate,
				top:      top,
				iptype:   "ipv4",
				protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
			},
			policy: getPBRPolicyForwarding("L3_Port1",
				getL3PBRRule(&testArgs{iptype: "ipv4", protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP}, "VRF10", 1, []uint8{10})),
			passingFlows: []gosnappi.Flow{
				// Initial flow for Port 1's policy
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan10, "ipinipd10-multi_p1_p2", 10),
			},
			failingFlows: []gosnappi.Flow{
				// Initial flow for Port 1's policy
				getIPinIPFlow(&testArgs{iptype: "ipv4"}, atePort1, atePort2Vlan20, "ipinipd21_p1_p2", 21),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Log(tc.desc)
			pfpath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()
			// Configure pbr policy-forwarding
			fptest.ConfigureDefaultNetworkInstance(t, dut)
			errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), tc.policy)
			})
			if errMsg != nil {
				if tc.rejectable {
					t.Skipf("Skipping test case %q, PolicyForwarding config was rejected with an error: %s", tc.name, *errMsg)
				}
				t.Fatalf("PolicyForwarding config update failed: %v", *errMsg)
			}
			// Defer cleaning policy-forwarding
			defer gnmi.Delete(t, dut, pfpath.Config())

			// Apply PBR policy on the ingress interface
			ingressPortName := tc.ingressPort.Name()
			d := &oc.Root{}
			interfaceID := ingressPortName
			if deviations.InterfaceRefInterfaceIDFormat(dut) {
				interfaceID = ingressPortName + ".0"
			}
			pfIntf := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(interfaceID)
			pfIntfConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(interfaceID)
			pfIntf.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPortName)
			pfIntf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
			if deviations.InterfaceRefConfigUnsupported(dut) {
				pfIntf.InterfaceRef = nil
			}
			pfIntf.SetApplyVrfSelectionPolicy(tc.policyName)
			gnmi.Update(t, dut, pfIntfConfPath.Config(), pfIntf)

			// Defer deletion of policy from interface
			defer gnmi.Delete(t, dut, pfIntfConfPath.Config())

			// "Multiple Policies" test case where a second policy is applied to Port 3
			if tc.name == "RT3.2 Case5 - Multiple Policies (Port 1 and Port 3)" {
				t.Log("Applying second policy (L3_Port3) to Port 3 for multiple policy scenario.")
				// Define testArgs for the second policy's flows and rules
				argsForPort3Policy := &testArgs{
					dut:      dut,
					ate:      ate,
					top:      top,
					iptype:   "ipv4",
					protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
				}
				multiPolicy := getPBRPolicyForwarding("L3_Port3", getL3PBRRule(argsForPort3Policy, "VRF40", 1, []uint8{40}))
				gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), multiPolicy)
				defer gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Policy("L3_Port3").Config())
				interface3ID := port3.Name()
				if deviations.InterfaceRefInterfaceIDFormat(dut) {
					interface3ID = port3.Name() + ".0"
				}
				pfIntfPort3 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(interface3ID)
				pfIntfConfPathPort3 := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(interface3ID)
				pfIntfPort3.GetOrCreateInterfaceRef().Interface = ygot.String(port3.Name())
				pfIntfPort3.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
				if deviations.InterfaceRefConfigUnsupported(dut) {
					pfIntfPort3.InterfaceRef = nil
				}
				pfIntfPort3.SetApplyVrfSelectionPolicy("L3_Port3")
				gnmi.Update(t, dut, pfIntfConfPathPort3.Config(), pfIntfPort3)
				defer gnmi.Delete(t, dut, pfIntfConfPathPort3.Config())

				// Add flows for Port 3 to the passing flows
				tc.passingFlows = append(tc.passingFlows, getIPinIPFlow(argsForPort3Policy, atePort3, atePort4Vlan40, "ipinipd40-multi_p3_p4", 40))
				tc.failingFlows = append(tc.failingFlows, getIPinIPFlow(argsForPort3Policy, atePort3, atePort4Vlan50, "ipinipd50-multi_p3_p4", 51))
			}

			// traffic should pass
			testTrafficFlows(t, ate, top, true, tc.passingFlows...)

			if len(tc.failingFlows) > 0 {
				// traffic should fail
				testTrafficFlows(t, ate, top, false, tc.failingFlows...)
			}
		})
	}
}
