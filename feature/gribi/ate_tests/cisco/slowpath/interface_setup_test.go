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

package slowpath_test

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

type attributes struct {
	attrs.Attributes
	numSubIntf uint32
	ip         func(vlan uint8) string
	gateway    func(vlan uint8) string
}

var (
	dutPort1 = attributes{
		Attributes: attrs.Attributes{
			Desc:    "dutPort1",
			Name:    "port1",
			IPv4:    dutPort1IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 0,
		ip:         dutPort1IPv4,
	}

	atePort1 = attributes{
		Attributes: attrs.Attributes{
			Name:    "port1",
			IPv4:    atePort1IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 0,
		ip:         atePort1IPv4,
		gateway:    dutPort1IPv4,
	}

	dutPort2 = attributes{
		Attributes: attrs.Attributes{
			Desc:    "dutPort2",
			Name:    "port2",
			IPv4:    dutPort2IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 62,
		ip:         dutPort2IPv4,
	}

	atePort2 = attributes{
		Attributes: attrs.Attributes{
			Name:    "port2",
			IPv4:    atePort2IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 15,
		ip:         atePort2IPv4,
		gateway:    dutPort2IPv4,
	}
	dutPort3 = attributes{
		Attributes: attrs.Attributes{
			Desc:    "dutPort3",
			Name:    "port3",
			IPv4:    dutPort3IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 0,
		ip:         dutPort3IPv4,
	}

	atePort3 = attributes{
		Attributes: attrs.Attributes{
			Name:    "port3",
			IPv4:    atePort3IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 0,
		ip:         atePort3IPv4,
		gateway:    dutPort3IPv4,
	}

	dutPort4 = attributes{
		Attributes: attrs.Attributes{
			Desc:    "dutPort4",
			Name:    "port4",
			IPv4:    dutPort4IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 62,
		ip:         dutPort4IPv4,
	}

	atePort4 = attributes{
		Attributes: attrs.Attributes{
			Name:    "port4",
			IPv4:    atePort4IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 15,
		ip:         atePort4IPv4,
		gateway:    dutPort4IPv4,
	}
	dutPort5 = attributes{
		Attributes: attrs.Attributes{
			Desc:    "dutPort5",
			Name:    "port5",
			IPv4:    dutPort5IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 8,
		ip:         dutPort5IPv4,
	}

	atePort5 = attributes{
		Attributes: attrs.Attributes{
			Name:    "port5",
			IPv4:    atePort5IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 8,
		ip:         atePort5IPv4,
		gateway:    dutPort5IPv4,
	}

	dutPort6 = attributes{
		Attributes: attrs.Attributes{
			Desc:    "dutPort6",
			Name:    "port6",
			IPv4:    dutPort6IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 62,
		ip:         dutPort6IPv4,
	}

	atePort6 = attributes{
		Attributes: attrs.Attributes{
			Name:    "port6",
			IPv4:    atePort6IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 15,
		ip:         atePort6IPv4,
		gateway:    dutPort6IPv4,
	}
	dutPort7 = attributes{
		Attributes: attrs.Attributes{
			Desc:    "dutPort7",
			Name:    "port7",
			IPv4:    dutPort7IPv4(0),
			IPv4Len: ipv4PrefixLen,
			IPv6:    "192:0:2::19",
		},
		numSubIntf: 0,
		ip:         dutPort7IPv4,
	}

	atePort7 = attributes{
		Attributes: attrs.Attributes{
			Name:    "port7",
			IPv4:    atePort7IPv4(0),
			IPv4Len: ipv4PrefixLen,
			IPv6:    "192:0:2::1A",
		},
		numSubIntf: 0,
		ip:         atePort7IPv4,
		gateway:    dutPort7IPv4,
	}

	dutPort8 = attributes{
		Attributes: attrs.Attributes{
			Desc:    "dutPort8",
			Name:    "port8",
			IPv4:    dutPort8IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 62,
		ip:         dutPort8IPv4,
	}

	atePort8 = attributes{
		Attributes: attrs.Attributes{
			Name:    "port8",
			IPv4:    atePort8IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 15,
		ip:         atePort8IPv4,
		gateway:    dutPort8IPv4,
	}
)

// dutPort1IPv4 returns ip address 192.0.2.1, for every vlanID.
func dutPort1IPv4(uint8) string {
	return "192.0.2.1"
}

// atePort1IPv4 returns ip address 192.0.2.2, for every vlanID
func atePort1IPv4(uint8) string {
	return "192.0.2.2"
}

// dutPort2IPv4 returns ip addresses starting 192.0.2.5, increasing by 4
// for every vlanID.
func dutPort2IPv4(vlan uint8) string {
	return fmt.Sprintf("192.0.2.%d", vlan*4+5)
}

// atePort2IPv4 returns ip addresses starting 192.0.2.6, increasing by 4
// for every vlanID.
func atePort2IPv4(vlan uint8) string {
	return fmt.Sprintf("192.0.2.%d", vlan*4+6)
}

func dutPort3IPv4(vlan uint8) string {
	return "192.0.3.1"
}

func atePort3IPv4(vlan uint8) string {
	return "192.0.3.2"
}

func dutPort4IPv4(vlan uint8) string {
	return fmt.Sprintf("192.0.3.%d", vlan*4+5)
}

func atePort4IPv4(vlan uint8) string {
	return fmt.Sprintf("192.0.3.%d", vlan*4+6)
}

func dutPort5IPv4(vlan uint8) string {
	return fmt.Sprintf("192.0.6.%d", vlan*4+5)
}

func atePort5IPv4(vlan uint8) string {
	return fmt.Sprintf("192.0.6.%d", vlan*4+6)
}

func dutPort6IPv4(vlan uint8) string {
	return fmt.Sprintf("192.0.4.%d", vlan*4+5)
}

func atePort6IPv4(vlan uint8) string {
	return fmt.Sprintf("192.0.4.%d", vlan*4+6)
}

func dutPort7IPv4(vlan uint8) string {
	return "192.0.5.1"
}

func atePort7IPv4(vlan uint8) string {
	return "192.0.5.2"
}

func dutPort8IPv4(vlan uint8) string {
	return fmt.Sprintf("192.0.5.%d", vlan*4+5)
}

func atePort8IPv4(vlan uint8) string {
	return fmt.Sprintf("192.0.5.%d", vlan*4+6)
}

func (a *attributes) configInterfaceDUT(t *testing.T, d *ondatra.DUTDevice) {
	t.Helper()
	p := d.Port(t, a.Name)
	i := &oc.Interface{Name: ygot.String(p.Name())}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	if a.numSubIntf > 0 {
		i.Description = ygot.String(a.Desc)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	} else {
		i = a.NewOCInterface(p.Name(), d)
	}

	a.configSubInterfaceDUT(i)
	intfPath := gnmi.OC().Interface(p.Name())
	gnmi.Replace(t, d, intfPath.Config(), i)
}

// configInterfaceDUT configures the interface with the Addrs.
func (a *attributes) configSubInterfaceDUT(i *oc.Interface) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

	for j := uint32(1); j <= a.numSubIntf; j++ {
		ip := a.ip(uint8(j))

		s := i.GetOrCreateSubinterface(j)
		s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(j))
		s4 := s.GetOrCreateIpv4()
		s4a := s4.GetOrCreateAddress(ip)
		s4a.PrefixLength = ygot.Uint8(a.IPv4Len)
	}
	return i
}

func configBunInterfaceDUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {
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

func (a *testArgs) interfaceaction(t *testing.T, port string, action bool) {
	dutP := a.dut.Port(t, port)
	//n := &oc.NetworkInstance{Name: ygot.String("DEFAULT")}
	c := gnmi.OC().Interface(dutP.Name())

	if action {
		//gnmi.Update(t, a.dut, gnmi.OC().Interface(dutP.Name()).Subinterface(0).Enabled().Config(), true)
		// 	gnmi.Await(t, a.dut, gnmi.OC().Interface(a.dut.Port(t, port).Name()).OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
		gnmi.Update(t, a.dut, c.Config(), &oc.Interface{
			Name:    ygot.String(dutP.Name()),
			Enabled: ygot.Bool(true),
		})
	} else {
		// 	gnmi.Update(t, a.dut, gnmi.OC().Interface(dutP.Name()).Subinterface(0).Enabled().Config(), false)
		gnmi.Update(t, a.dut, c.Config(), &oc.Interface{
			Name:    ygot.String(dutP.Name()),
			Enabled: ygot.Bool(false),
		})
	}
}

// configureDUT configures port1, port2 and port3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()

	// Configure DUT ports.
	dutPort1.configInterfaceDUT(t, dut)
	dutPort2.configInterfaceDUT(t, dut)
	dutPort3.configInterfaceDUT(t, dut)
	dutPort4.configInterfaceDUT(t, dut)
	dutPort5.configInterfaceDUT(t, dut)
	dutPort6.configInterfaceDUT(t, dut)
	p7 := dut.Port(t, "port7")
	d := gnmi.OC()
	i7 := &oc.Interface{Name: ygot.String("Bundle-Ether126")}
	gnmi.Replace(t, dut, d.Interface(*i7.Name).Config(), configBunInterfaceDUT(i7, &dutPort7.Attributes))
	BE126 := generateBundleMemberInterfaceConfig(t, p7.Name(), *i7.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(p7.Name()).Config(), BE126)
	dutPort8.configInterfaceDUT(t, dut)
}

func generateBundleMemberInterfaceConfig(t *testing.T, name, bundleID string) *oc.Interface {
	t.Helper()
	i := &oc.Interface{Name: ygot.String(name)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)
	return i
}

// configRP, configures route_policy for BGP
func configRP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dev := &oc.Root{}
	inst := dev.GetOrCreateRoutingPolicy()
	pdef := inst.GetOrCreatePolicyDefinition("ALLOW")
	stmt1, _ := pdef.AppendNewStatement("1")
	stmt1.GetOrCreateActions().PolicyResult = oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE

	dutNode := gnmi.OC().RoutingPolicy()
	dutConf := dev.GetOrCreateRoutingPolicy()
	gnmi.Update(t, dut, dutNode.Config(), dutConf)
}
