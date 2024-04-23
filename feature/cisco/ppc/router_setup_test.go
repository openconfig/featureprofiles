// Copyright 2022 Google LLC
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

package ppc_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
	vlanMTU       = 1518
	vlans         = 2
)

const (
	PTISIS         = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	DUTAreaAddress = "47.0001"
	DUTSysID       = "0000.0000.0001"
	ISISName       = "osiris"
)

const (
	PTBGP = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	BGPAS = 65000
)

type PBROptions struct {
	// BackupNHG specifies the backup next-hop-group to be used when all next-hops are unavailable.
	SrcIP string
}

var (
	dutSrc = attrs.Attributes{
		Desc:    "dutSrc",
		IPv4:    "100.121.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:1",
		IPv6Len: ipv6PrefixLen,
	}

	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		IPv4:    "100.121.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutDst = attrs.Attributes{
		Desc:    "dutDst",
		IPv4:    "100.122.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:122:1:1",
		IPv6Len: ipv6PrefixLen,
	}

	ateDst = attrs.Attributes{
		Name:    "ateDst",
		IPv4:    "100.122.1.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:122:1:2",
		IPv6Len: ipv6PrefixLen,
	}

	// dutPort2Vlan10 = attrs.Attributes{
	// 	Desc:    "dutPort2Vlan10",
	// 	IPv4:    "100.128.10.1",
	// 	IPv4Len: ipv4PrefixLen,
	// 	IPv6:    "2000::100:128:10:1",
	// 	IPv6Len: ipv6PrefixLen,
	// 	MTU:     vlanMTU,
	// }
	// atePort2Vlan10 = attrs.Attributes{
	// 	Name:    "atePort2Vlan10",
	// 	IPv4:    "100.128.10.2",
	// 	IPv4Len: ipv4PrefixLen,
	// 	IPv6:    "2000::100:128:10:2",
	// 	IPv6Len: ipv6PrefixLen,
	// 	MTU:     vlanMTU,
	// }
)

// configInterfaceDUT configures the interface with the Addrs.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	s6 := s.GetOrCreateIpv6()
	s6a := s6.GetOrCreateAddress(a.IPv6)
	s6a.PrefixLength = ygot.Uint8(ipv6PrefixLen)

	return i
}

// configureDUT configures port1-port8 on DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dutPorts := sortPorts(dut.Ports())
	d := gnmi.OC()

	//incoming interface is bundler-120 with only 1 member (port1)
	incoming := &oc.Interface{Name: ygot.String("Bundle-Ether120")}
	gnmi.Replace(t, dut, d.Interface(*incoming.Name).Config(), configInterfaceDUT(incoming, &dutSrc))
	srcPorts := dutPorts[0]
	dutsrc := generateBundleMemberInterfaceConfig(t, srcPorts.Name(), *incoming.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(srcPorts.Name()).Config(), dutsrc)

	//outing interface is bundle-121 with 7 members (port2, port 3, port4, port5, port6, port7)
	// lacp := &oc.Lacp_Interface{Name: ygot.String("Bundle-Ether121")}
	// lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE
	// lacpPath := d.Lacp().Interface("Bundle-Ether121")
	// gnmi.Replace(t, dut, lacpPath.Config(), lacp)

	outgoing := &oc.Interface{Name: ygot.String("Bundle-Ether121")}
	outgoing_data := configInterfaceDUT(outgoing, &dutDst)
	g := outgoing_data.GetOrCreateAggregation()
	g.LagType = oc.IfAggregate_AggregationType_LACP
	gnmi.Replace(t, dut, d.Interface(*outgoing.Name).Config(), configInterfaceDUT(outgoing, &dutDst))
	for _, port := range dutPorts[1:] {
		dutdest := generateBundleMemberInterfaceConfig(t, port.Name(), *outgoing.Name)
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), dutdest)
	}
	//Configure VLANs on Bundle-Ether127
	// for i := 1; i <= vlans; i++ {
	// 	//Create VRFs and VRF enabled subinterfaces
	// 	createNameSpace(t, dut, fmt.Sprintf("VRF%d", i+9), "Bundle-Ether127", uint32(i))
	// 	//Add IPv4/IPv6 address on VLANs
	// 	subint := getSubInterface(fmt.Sprintf("100.128.%d.1", i+9), 24, fmt.Sprintf("2000::100:128:%d:1", i+10), 126, uint16(i+10), uint32(i))
	// 	gnmi.Update(t, dut, gnmi.OC().Interface("Bundle-Ether121").Subinterface(uint32(i)).Config(), subint)
	// }
}

// func createNameSpace(t *testing.T, dut *ondatra.DUTDevice, name, intfname string, subint uint32) {
// 	//create empty subinterface
// 	si := &oc.Interface_Subinterface{}
// 	si.Index = ygot.Uint32(subint)
// 	gnmi.Replace(t, dut, gnmi.OC().Interface(intfname).Subinterface(subint).Config(), si)

// 	//create vrf and apply on subinterface
// 	v := &oc.NetworkInstance{
// 		Name: ygot.String(name),
// 	}
// 	vi := v.GetOrCreateInterface(intfname + "." + strconv.Itoa(int(subint)))
// 	vi.Subinterface = ygot.Uint32(subint)
// 	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(name).Config(), v)
// }

// func getSubInterface(ipv4 string, prefixlen4 uint8, ipv6 string, prefixlen6 uint8, vlanID uint16, index uint32) *oc.Interface_Subinterface {
// 	s := &oc.Interface_Subinterface{}
// 	s.Index = ygot.Uint32(index)
// 	s4 := s.GetOrCreateIpv4()
// 	a := s4.GetOrCreateAddress(ipv4)
// 	a.PrefixLength = ygot.Uint8(prefixlen4)
// 	s6 := s.GetOrCreateIpv6()
// 	a6 := s6.GetOrCreateAddress(ipv6)
// 	a6.PrefixLength = ygot.Uint8(prefixlen6)
// 	v := s.GetOrCreateVlan()
// 	m := v.GetOrCreateMatch()
// 	if index != 0 {
// 		m.GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
// 	}
// 	return s
// }

func generateBundleMemberInterfaceConfig(t *testing.T, name, bundleID string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(name)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)
	return i
}

// configRP, configures route_policy for BGP
func configRP(t *testing.T, dut *ondatra.DUTDevice) {
	dev := &oc.Root{}
	inst := dev.GetOrCreateRoutingPolicy()
	pdef := inst.GetOrCreatePolicyDefinition("ALLOW")
	stmt1, _ := pdef.AppendNewStatement("1")
	stmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	dutNode := gnmi.OC().RoutingPolicy()
	dutConf := dev.GetOrCreateRoutingPolicy()
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}

// addISISOC, configures ISIS on DUT
func addISISOC(t *testing.T, dut *ondatra.DUTDevice, ifaceName string) {
	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(PTISIS, ISISName)
	isis := prot.GetOrCreateIsis()
	glob := isis.GetOrCreateGlobal()
	glob.Net = []string{fmt.Sprintf("%v.%v.00", DUTAreaAddress, DUTSysID)}
	glob.LevelCapability = 2
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	glob.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf := isis.GetOrCreateInterface(ifaceName)
	intf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	intf.Enabled = ygot.Bool(true)
	intf.HelloPadding = 1
	intf.Passive = ygot.Bool(false)
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	intf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	level := isis.GetOrCreateLevel(2)
	level.MetricStyle = 2

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(PTISIS, ISISName)
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(PTISIS, ISISName)
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}

// addBGPOC, configures ISIS on DUT
func addBGPOC(t *testing.T, dut *ondatra.DUTDevice, neighbor string) {
	dev := &oc.Root{}
	inst := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance)
	prot := inst.GetOrCreateProtocol(PTBGP, *ciscoFlags.DefaultNetworkInstance)
	bgp := prot.GetOrCreateBgp()
	glob := bgp.GetOrCreateGlobal()
	glob.As = ygot.Uint32(BGPAS)
	glob.RouterId = ygot.String("1.1.1.1")
	glob.GetOrCreateGracefulRestart().Enabled = ygot.Bool(true)
	glob.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	pg := bgp.GetOrCreatePeerGroup("BGP-PEER-GROUP")
	pg.PeerAs = ygot.Uint32(64001)
	pg.LocalAs = ygot.Uint32(63001)
	pg.PeerGroupName = ygot.String("BGP-PEER-GROUP")

	peer := bgp.GetOrCreateNeighbor(neighbor)
	peer.PeerGroup = ygot.String("BGP-PEER-GROUP")
	peer.GetOrCreateEbgpMultihop().Enabled = ygot.Bool(true)
	peer.GetOrCreateEbgpMultihop().MultihopTtl = ygot.Uint8(255)
	peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ImportPolicy = []string{"ALLOW"}
	peer.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateApplyPolicy().ExportPolicy = []string{"ALLOW"}

	dutNode := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(PTBGP, *ciscoFlags.DefaultNetworkInstance)
	dutConf := dev.GetOrCreateNetworkInstance(*ciscoFlags.DefaultNetworkInstance).GetOrCreateProtocol(PTBGP, *ciscoFlags.DefaultNetworkInstance)
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}

// configVRF
func configVRF(t *testing.T, dut *ondatra.DUTDevice, vrfs []string) {
	for _, vrf_name := range vrfs {
		vrf := &oc.NetworkInstance{
			Name: ygot.String(vrf_name),
			Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
		}
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf_name).Config(), vrf)
	}
}

// configbasePBR, creates class map, policy and configures under source interface
func configbasePBR(t *testing.T, dut *ondatra.DUTDevice, networkInstance, iptype string, index uint32, pbrName string, protocol oc.E_PacketMatchTypes_IP_PROTOCOL, dscpset []uint8, opts ...*PBROptions) {

	fptest.ConfigureDefaultNetworkInstance(t, dut)

	r := oc.NetworkInstance_PolicyForwarding_Policy_Rule{}
	r.SequenceId = ygot.Uint32(index)
	r.Action = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Action{NetworkInstance: ygot.String(networkInstance)}
	if iptype == "ipv4" {
		if len(opts) != 0 {
			for _, opt := range opts {
				if opt.SrcIP != "" {
					r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
						SourceAddress: &opt.SrcIP,
						Protocol:      protocol,
					}
				}
			}
		} else {
			r.Ipv4 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv4{
				Protocol: protocol,
			}
		}
		if len(dscpset) > 0 {
			r.Ipv4.DscpSet = dscpset
		}
	} else if iptype == "ipv6" {
		r.Ipv6 = &oc.NetworkInstance_PolicyForwarding_Policy_Rule_Ipv6{
			Protocol: protocol,
		}
		if len(dscpset) > 0 {
			r.Ipv6.DscpSet = dscpset
		}
	}
	pf := oc.NetworkInstance_PolicyForwarding{}
	p := pf.GetOrCreatePolicy(pbrName)
	p.Type = oc.Policy_Type_VRF_SELECTION_POLICY
	p.AppendRule(&r)
	intf := pf.GetOrCreateInterface("Bundle-Ether120.0")
	intf.GetOrCreateInterfaceRef().Interface = ygot.String("Bundle-Ether120")
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	intf.ApplyVrfSelectionPolicy = ygot.String(pbrName)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).PolicyForwarding().Config(), &pf)
}
