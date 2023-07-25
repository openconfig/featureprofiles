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

package qos_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	PTISIS         = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS
	DUTAreaAddress = "47.0001"
	DUTSysID       = "0000.0000.0001"
	ISISName       = "osiris"
	pLen4          = 30
	pLen6          = 126
)

const (
	PTBGP         = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP
	BGPAS         = 65000
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv6:    "192:0:2::1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv6:    "192:0:2::2",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv6:    "192:0:2::5",
		IPv4Len: ipv4PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv6:    "192:0:2::6",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv6:    "192:0:2::9",
		IPv4Len: ipv4PrefixLen,
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "192.0.2.10",
		IPv6:    "192:0:2::A",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.13",
		IPv6:    "192:0:2::D",
		IPv4Len: ipv4PrefixLen,
	}

	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		IPv4:    "192.0.2.14",
		IPv6:    "192:0:2::E",
		IPv4Len: ipv4PrefixLen,
	}
	dutPort5 = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "192.0.2.17",
		IPv6:    "192:0:2::11",
		IPv4Len: ipv4PrefixLen,
	}

	atePort5 = attrs.Attributes{
		Name:    "atePort5",
		IPv4:    "192.0.2.18",
		IPv6:    "192:0:2::12",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort6 = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "192.0.2.21",
		IPv6:    "192:0:2::15",
		IPv4Len: ipv4PrefixLen,
	}

	atePort6 = attrs.Attributes{
		Name:    "atePort6",
		IPv4:    "192.0.2.22",
		IPv6:    "192:0:2::16",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort7 = attrs.Attributes{
		Desc:    "dutPort7",
		IPv4:    "192.0.2.25",
		IPv6:    "192:0:2::19",
		IPv4Len: ipv4PrefixLen,
	}

	atePort7 = attrs.Attributes{
		Name:    "atePort7",
		IPv4:    "192.0.2.26",
		IPv6:    "192:0:2::1A",
		IPv4Len: ipv4PrefixLen,
	}

	dutPort8 = attrs.Attributes{
		Desc:    "dutPort8",
		IPv4:    "192.0.2.29",
		IPv6:    "192:0:2::1D",
		IPv4Len: ipv4PrefixLen,
	}

	atePort8 = attrs.Attributes{
		Name:    "atePort8",
		IPv4:    "192.0.2.30",
		IPv6:    "192:0:2::1E",
		IPv4Len: ipv4PrefixLen,
	}
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

// interfaceaction shuts/unshuts provided interface
func (a *testArgs) interfaceaction(t *testing.T, port string, action bool) {
	// ateP := a.ate.Port(t, port)
	dutP := a.dut.Port(t, port)
	if action {
		// a.ate.Operations().NewSetInterfaceState().WithPhysicalInterface(ateP).WithStateEnabled(true).Operate(t)
		gnmi.Replace(t, a.dut, gnmi.OC().Interface(dutP.Name()).Enabled().Config(), true)
		// a.dut.Telemetry().Interface(dutP.Name()).OperStatus().Await(t, time.Minute, telemetry.Interface_OperStatus_UP)
	} else {
		// a.ate.Operations().NewSetInterfaceState().WithPhysicalInterface(ateP).WithStateEnabled(false).Operate(t)
		gnmi.Replace(t, a.dut, gnmi.OC().Interface(dutP.Name()).Enabled().Config(), false)
		// a.dut.Telemetry().Interface(dutP.Name()).OperStatus().Await(t, time.Minute, telemetry.Interface_OperStatus_DOWN)
	}
}

// configureDUT configures port1, port2 and port3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String("Bundle-Ether120")}
	gnmi.Replace(t, dut, d.Interface(*i1.Name).Config(), configInterfaceDUT(i1, &dutPort1))
	BE120 := generateBundleMemberInterfaceConfig(t, p1.Name(), *i1.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), BE120)

	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String("Bundle-Ether121")}
	gnmi.Replace(t, dut, d.Interface(*i2.Name).Config(), configInterfaceDUT(i2, &dutPort2))
	BE121 := generateBundleMemberInterfaceConfig(t, p2.Name(), *i2.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p2.Name()).Config(), BE121)

	p3 := dut.Port(t, "port3")
	i3 := &oc.Interface{Name: ygot.String("Bundle-Ether122")}
	gnmi.Replace(t, dut, d.Interface(*i3.Name).Config(), configInterfaceDUT(i3, &dutPort3))
	BE122 := generateBundleMemberInterfaceConfig(t, p3.Name(), *i3.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p3.Name()).Config(), BE122)

	p4 := dut.Port(t, "port4")
	i4 := &oc.Interface{Name: ygot.String("Bundle-Ether123")}
	gnmi.Replace(t, dut, d.Interface(*i4.Name).Config(), configInterfaceDUT(i4, &dutPort4))
	BE123 := generateBundleMemberInterfaceConfig(t, p4.Name(), *i4.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p4.Name()).Config(), BE123)

	p5 := dut.Port(t, "port5")
	i5 := &oc.Interface{Name: ygot.String("Bundle-Ether124")}
	gnmi.Replace(t, dut, d.Interface(*i5.Name).Config(), configInterfaceDUT(i5, &dutPort5))
	BE124 := generateBundleMemberInterfaceConfig(t, p5.Name(), *i5.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p5.Name()).Config(), BE124)

	p6 := dut.Port(t, "port6")
	i6 := &oc.Interface{Name: ygot.String("Bundle-Ether125")}
	gnmi.Replace(t, dut, d.Interface(*i6.Name).Config(), configInterfaceDUT(i6, &dutPort6))
	BE125 := generateBundleMemberInterfaceConfig(t, p6.Name(), *i6.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p6.Name()).Config(), BE125)

	p7 := dut.Port(t, "port7")
	i7 := &oc.Interface{Name: ygot.String("Bundle-Ether126")}
	gnmi.Replace(t, dut, d.Interface(*i7.Name).Config(), configInterfaceDUT(i7, &dutPort7))
	BE126 := generateBundleMemberInterfaceConfig(t, p7.Name(), *i7.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p7.Name()).Config(), BE126)

	p8 := dut.Port(t, "port8")
	i8 := &oc.Interface{Name: ygot.String("Bundle-Ether127")}
	gnmi.Replace(t, dut, d.Interface(*i8.Name).Config(), configInterfaceDUT(i8, &dutPort8))
	BE127 := generateBundleMemberInterfaceConfig(t, p8.Name(), *i8.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p8.Name()).Config(), BE127)
}

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
	stmt1 := pdef.GetOrCreateStatement("1")
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
