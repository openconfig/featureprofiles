package qos_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	telemetry "github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func createNameSpace(t *testing.T, dut *ondatra.DUTDevice, name, intfname string, subint uint32) {
	//create empty subinterface
	si := &telemetry.Interface_Subinterface{}
	si.Index = ygot.Uint32(subint)
	gnmi.Replace(t, dut, gnmi.OC().Interface(intfname).Subinterface(subint).Config(), si)

	//create vrf and apply on subinterface
	v := &telemetry.NetworkInstance{
		Name: ygot.String(name),
	}
	vi := v.GetOrCreateInterface(intfname + "." + strconv.Itoa(int(subint)))
	vi.Subinterface = ygot.Uint32(subint)
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(name).Config(), v)
}

func getSubInterface(ipv4 string, prefixlen4 uint8, ipv6 string, prefixlen6 uint8, vlanID uint16, index uint32) *telemetry.Interface_Subinterface {
	s := &telemetry.Interface_Subinterface{}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen4)
	s6 := s.GetOrCreateIpv6()
	a6 := s6.GetOrCreateAddress(ipv6)
	a6.PrefixLength = ygot.Uint8(prefixlen6)
	v := s.GetOrCreateVlan()
	m := v.GetOrCreateMatch()
	if index != 0 {
		m.GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
	}
	return s
}

func addIpv6Address(i *telemetry.Interface, ipv6 string, prefixlen uint8, index uint32) *telemetry.Interface {
	i.Type = telemetry.IETFInterfaces_InterfaceType_ieee8023adLag
	//Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	s := i.GetOrCreateSubinterface(index)
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv6()
	a := s4.GetOrCreateAddress(ipv6)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return i
}

func configureIpv6AndVlans(t *testing.T, dut *ondatra.DUTDevice) {
	//Configure IPv6 address on Bundle-Ether120, Bundle-Ether121
	i1 := &telemetry.Interface{Name: ygot.String("Bundle-Ether120")}
	i2 := &telemetry.Interface{Name: ygot.String("Bundle-Ether121")}
	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether120").Config(), addIpv6Address(i1, dutPort1.IPv6, dutPort1.IPv6Len, 0))
	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether121").Config(), addIpv6Address(i2, dutPort2.IPv6, dutPort2.IPv6Len, 0))

	//Configure VLANs on Bundle-Ether121
	for i := 1; i <= 3; i++ {
		//Create VRFs and VRF enabled subinterfaces
		createNameSpace(t, dut, fmt.Sprintf("VRF%d", i*10), "Bundle-Ether121", uint32(i))
		//Add IPv4/IPv6 address on VLANs
		subint := getSubInterface(fmt.Sprintf("100.121.%d.1", i*10), 24, fmt.Sprintf("2000::100:121:%d:1", i*10), 126, uint16(i*10), uint32(i))
		gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether121").Subinterface(uint32(i)).Config(), subint)
	}

}
