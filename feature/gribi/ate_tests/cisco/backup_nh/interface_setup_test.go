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

package backup_nh_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
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
func configInterfaceDUT(i *telemetry.Interface, a *attrs.Attributes) *telemetry.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = telemetry.IETFInterfaces_InterfaceType_ieee8023adLag
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if *deviations.InterfaceEnabled {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	s6 := s.GetOrCreateIpv6()
	if *deviations.InterfaceEnabled {
		s6.Enabled = ygot.Bool(true)
	}
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
		a.dut.Config().Interface(dutP.Name()).Enabled().Replace(t, true)
		// a.dut.Telemetry().Interface(dutP.Name()).OperStatus().Await(t, time.Minute, telemetry.Interface_OperStatus_UP)
	} else {
		// a.ate.Operations().NewSetInterfaceState().WithPhysicalInterface(ateP).WithStateEnabled(false).Operate(t)
		a.dut.Config().Interface(dutP.Name()).Enabled().Replace(t, false)
		// a.dut.Telemetry().Interface(dutP.Name()).OperStatus().Await(t, time.Minute, telemetry.Interface_OperStatus_DOWN)
	}
}

// configureDUT configures port1, port2 and port3 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := dut.Config()

	p1 := dut.Port(t, "port1")
	i1 := &telemetry.Interface{Name: ygot.String("Bundle-Ether120")}
	d.Interface(*i1.Name).Replace(t, configInterfaceDUT(i1, &dutPort1))
	BE120 := generateBundleMemberInterfaceConfig(t, p1.Name(), *i1.Name)
	dut.Config().Interface(p1.Name()).Replace(t, BE120)

	p2 := dut.Port(t, "port2")
	i2 := &telemetry.Interface{Name: ygot.String("Bundle-Ether121")}
	d.Interface(*i2.Name).Replace(t, configInterfaceDUT(i2, &dutPort2))
	BE121 := generateBundleMemberInterfaceConfig(t, p2.Name(), *i2.Name)
	dut.Config().Interface(p2.Name()).Replace(t, BE121)

	p3 := dut.Port(t, "port3")
	i3 := &telemetry.Interface{Name: ygot.String("Bundle-Ether122")}
	d.Interface(*i3.Name).Replace(t, configInterfaceDUT(i3, &dutPort3))
	BE122 := generateBundleMemberInterfaceConfig(t, p3.Name(), *i3.Name)
	dut.Config().Interface(p3.Name()).Replace(t, BE122)

	p4 := dut.Port(t, "port4")
	i4 := &telemetry.Interface{Name: ygot.String("Bundle-Ether123")}
	d.Interface(*i4.Name).Replace(t, configInterfaceDUT(i4, &dutPort4))
	BE123 := generateBundleMemberInterfaceConfig(t, p4.Name(), *i4.Name)
	dut.Config().Interface(p4.Name()).Replace(t, BE123)

	p5 := dut.Port(t, "port5")
	i5 := &telemetry.Interface{Name: ygot.String("Bundle-Ether124")}
	d.Interface(*i5.Name).Replace(t, configInterfaceDUT(i5, &dutPort5))
	BE124 := generateBundleMemberInterfaceConfig(t, p5.Name(), *i5.Name)
	dut.Config().Interface(p5.Name()).Replace(t, BE124)

	p6 := dut.Port(t, "port6")
	i6 := &telemetry.Interface{Name: ygot.String("Bundle-Ether125")}
	d.Interface(*i6.Name).Replace(t, configInterfaceDUT(i6, &dutPort6))
	BE125 := generateBundleMemberInterfaceConfig(t, p6.Name(), *i6.Name)
	dut.Config().Interface(p6.Name()).Replace(t, BE125)

	p7 := dut.Port(t, "port7")
	i7 := &telemetry.Interface{Name: ygot.String("Bundle-Ether126")}
	d.Interface(*i7.Name).Replace(t, configInterfaceDUT(i7, &dutPort7))
	BE126 := generateBundleMemberInterfaceConfig(t, p7.Name(), *i7.Name)
	dut.Config().Interface(p7.Name()).Replace(t, BE126)

	p8 := dut.Port(t, "port8")
	i8 := &telemetry.Interface{Name: ygot.String("Bundle-Ether127")}
	d.Interface(*i8.Name).Replace(t, configInterfaceDUT(i8, &dutPort8))
	BE127 := generateBundleMemberInterfaceConfig(t, p8.Name(), *i8.Name)
	dut.Config().Interface(p8.Name()).Replace(t, BE127)
}

func generateBundleMemberInterfaceConfig(t *testing.T, name, bundleID string) *telemetry.Interface {
	i := &telemetry.Interface{Name: ygot.String(name)}
	i.Type = telemetry.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)
	return i
}
