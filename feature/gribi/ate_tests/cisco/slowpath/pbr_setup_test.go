// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slowpath_test

import (
	"testing"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	vrfDecap             = "DECAP_TE_VRF"
	vrfRepair            = "REPAIR"
	vrfRepaired          = "REPAIRED"
	vrfEncapA            = "ENCAP_TE_VRF_A"
	vrfEncapB            = "ENCAP_TE_VRF_B"
	vrfDecapPostRepaired = "DECAP"
	dscpEncapA1          = 10
	ipv4OuterSrc111      = "198.50.100.111"
	ipv4FlowIP           = "138.0.11.8"
	ipv4EntryPrefixLen   = 24
	ipv6EntryPrefix      = "2015:aa8::2"
	ipv6EntryPrefixLen   = "128"
)

// configbasePBR, creates class map, policy and configures under source interface
func configbasePBR(t *testing.T, dut *ondatra.DUTDevice, networkInstance, iptype string, index uint32, protocol oc.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8, pbrName string, intfName string, val bool) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)
	r := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	if !val {
		r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	} else {
		decapVrfSet := []string{vrfDecap, vrfEncapA, "REPAIR"}
		r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{DecapNetworkInstance: ygot.String(decapVrfSet[0]), PostDecapNetworkInstance: ygot.String(decapVrfSet[1]), DecapFallbackNetworkInstance: ygot.String(decapVrfSet[2])}

	}

	if iptype == "ipv4" {
		r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv4.DscpSet = dscpset
		}
	} else if iptype == "ipv6" {
		r4 := r.GetOrCreateIpv4()
		r4.Protocol = oc.UnionUint8(41)
	}
	pf := oc.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(pbrName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(&r)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Config(), &pf)

	//configure PBR on ingress port
	d := &oc.Root{}
	pfpath := d.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreatePolicyForwarding().GetOrCreateInterface(intfName + ".0")
	pfpath.ApplyVrfSelectionPolicy = ygot.String(pbrName)
	pfpath.GetOrCreateInterfaceRef().Interface = ygot.String(intfName)
	pfpath.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	intfConfPath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Interface(intfName + ".0")
	gnmi.Replace(t, dut, intfConfPath.Config(), pfpath)
}

// unconfigbasePBR, creates class map, policy and configures under source interface
func unconfigbasePBR(t *testing.T, dut *ondatra.DUTDevice, pbrName string, intfName []string) {
	t.Helper()
	pfpath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding()
	for _, dp := range intfName {
		pfintfPath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Interface(dp + ".0")
		gnmi.Delete(t, dut, pfintfPath.Config())

	}
	gnmi.Delete(t, dut, pfpath.Policy(pbrName).Config())
}

func configPBR(t *testing.T, dut *ondatra.DUTDevice, vrf string, val bool) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	pf := ni.GetOrCreatePolicyForwarding()

	p := pf.GetOrCreatePolicy("PBR")
	p.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	decapVrfSet := []string{vrfDecap, vrfEncapA, "REPAIR"}
	if val {
		r := p.GetOrCreateRule(1)
		r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{DecapNetworkInstance: ygot.String(decapVrfSet[0]), PostDecapNetworkInstance: ygot.String(decapVrfSet[1]), DecapFallbackNetworkInstance: ygot.String(decapVrfSet[2])}
		r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}

		r = p.GetOrCreateRule(2)

		r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{DecapNetworkInstance: ygot.String(decapVrfSet[0]), PostDecapNetworkInstance: ygot.String(decapVrfSet[1]), DecapFallbackNetworkInstance: ygot.String(decapVrfSet[2])}
		r4 := r.GetOrCreateIpv4()
		r4.Protocol = oc.UnionUint8(41)
		r = p.GetOrCreateRule(3)
		r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(vrfEncapA)}
		l2 := r.GetOrCreateL2()
		l2.SetEthertype(oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4)
		r = p.GetOrCreateRule(4)
		r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(vrfEncapA)}
		l2 = r.GetOrCreateL2()
		l2.SetEthertype(oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6)

	} else {
		r := p.GetOrCreateRule(1)
		r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(vrf)}
		r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
			Protocol: oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP,
		}
		r = p.GetOrCreateRule(2)

		r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(vrf)}
		r4 := r.GetOrCreateIpv4()
		r4.Protocol = oc.UnionUint8(41)
		r = p.GetOrCreateRule(3)

		r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(vrf)}
		l2 := r.GetOrCreateL2()
		l2.SetEthertype(oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV4)

		r = p.GetOrCreateRule(4)
		r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(vrf)}
		l2 = r.GetOrCreateL2()
		l2.SetEthertype(oc.PacketMatchTypes_ETHERTYPE_ETHERTYPE_IPV6)

	}
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), pf)
}
func configureIntfPBR(t *testing.T, dut *ondatra.DUTDevice, pbrName, intfName string) {
	d := &oc.Root{}
	pfpath := d.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreatePolicyForwarding().GetOrCreateInterface(intfName + ".0")
	pfpath.ApplyVrfSelectionPolicy = ygot.String(pbrName)
	pfpath.GetOrCreateInterfaceRef().Interface = ygot.String(intfName)
	pfpath.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	intfConfPath := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Interface(intfName + ".0")
	gnmi.Replace(t, dut, intfConfPath.Config(), pfpath)
}

func configDUT(t *testing.T, dut *ondatra.DUTDevice) {

	configurePort(t, dut, "Loopback22", Loopback22, Loopback226, 32, 128)

	c := &oc.Root{}

	ni := c.GetOrCreateNetworkInstance(vrfEncapA)
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	niIntf := ni.GetOrCreateInterface("Loopback22")
	niIntf.Subinterface = ygot.Uint32(0)
	niIntf.Interface = ygot.String("Loopback22")
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrfEncapA).Config(), ni)
	configurePort(t, dut, "Loopback30", Loopback0, Loopback06, 32, 128)

	p := dut.Port(t, "port1")
	configurePort(t, dut, p.Name(), "192.0.9.1", "7777::3", 30, 126)

	p = dut.Port(t, "port2")
	configurePort(t, dut, p.Name(), "192.0.10.1", "192:0:2::1D", 30, 126)

	dc := gnmi.OC()
	niProto := dc.NetworkInstance(deviations.DefaultNetworkInstance(dut)).
		Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	ni = &oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	staticRoute := static.GetOrCreateStatic("197.51.0.0/16")
	nextHop := staticRoute.GetOrCreateNextHop("0")
	nextHop.NextHop = oc.UnionString("192.0.10.2")
	gnmi.Update(t, dut, niProto.Config(), static)
	ipv6nh := static.GetOrCreateStatic(ipv6EntryPrefix + "/128").GetOrCreateNextHop("0")
	ipv6nh.NextHop, _ = nextHop.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union("192:0:2::1E")
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)

}

func configurePort(t *testing.T, dut *ondatra.DUTDevice, IntfName, ip, ipv6 string, mask, mask6 int) {

	i1 := &oc.Interface{Name: ygot.String(IntfName)}
	if IntfName == "Loopback22" || IntfName == "Loopback30" {
		i1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback

	} else {
		i1.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	}
	s := i1.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	s4a := s4.GetOrCreateAddress(ip)
	s4a.PrefixLength = ygot.Uint8(uint8(mask))
	if ipv6 != "" {
		s6 := s.GetOrCreateIpv6()
		s6a := s6.GetOrCreateAddress(ipv6)
		s6a.PrefixLength = ygot.Uint8(uint8(mask6))
	}
	intfPath := gnmi.OC().Interface(IntfName)
	gnmi.Replace(t, dut, intfPath.Config(), i1)

}

func configvrfInt(t *testing.T, dut *ondatra.DUTDevice, vrfName, IntfName string) {
	c := &oc.Root{}

	ni := c.GetOrCreateNetworkInstance(vrfName)
	ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	niIntf := ni.GetOrCreateInterface(IntfName)
	niIntf.Subinterface = ygot.Uint32(0)
	niIntf.Interface = ygot.String(IntfName)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), ni)

}

func staticvrf(t *testing.T, dut *ondatra.DUTDevice, vrfName, nh1, nh2 string) {

	d := gnmi.OC()

	n := &oc.NetworkInstance{Name: ygot.String(vrfName)}
	static := n.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT")
	staticRoute := static.GetOrCreateStatic("30.30.30.30/32")
	nextHop := staticRoute.GetOrCreateNextHop("0")
	nextHop.NextHop = oc.UnionString(nh1)
	gnmi.Update(t, dut, d.NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT").Config(), static)

	ipv6nh := static.GetOrCreateStatic("30::30/128").GetOrCreateNextHop("0")
	ipv6nh.NextHop, _ = nextHop.To_NetworkInstance_Protocol_Static_NextHop_NextHop_Union(nh2)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(vrfName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT").Config(), static)
}
