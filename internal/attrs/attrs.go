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

// Package attrs bundles some common interface attributes and provides
// helpers to generate the appropriate OpenConfig and ATETopology.
//
// The use of this package in new tests is discouraged.  Legacy tests using this package
// will be migrated to use testbed topology helpers.
package attrs

import (
	"fmt"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

// Attributes bundles some common attributes for devices and/or interfaces.
// It provides helpers to generate appropriate configuration for OpenConfig
// and for an ATETopology.  All fields are optional; only those that are
// non-empty will be set when configuring an interface.
type Attributes struct {
	IPv4    string
	IPv6    string
	MAC     string
	Name    string // Interface name, only applied to ATE ports.
	Desc    string // Description, only applied to DUT interfaces.
	IPv4Len uint8  // Prefix length for IPv4.
	IPv6Len uint8  // Prefix length for IPv6.
	MTU     uint16
}

// IPv4CIDR constructs the IPv4 CIDR notation with the given prefix
// length, e.g. "192.0.2.1/30".
func (a *Attributes) IPv4CIDR() string {
	return fmt.Sprintf("%s/%d", a.IPv4, a.IPv4Len)
}

// IPv6CIDR constructs the IPv6 CIDR notation with the given prefix
// length, e.g. "2001:db8::1/126".
func (a *Attributes) IPv6CIDR() string {
	return fmt.Sprintf("%s/%d", a.IPv6, a.IPv6Len)
}

// ConfigInterface configures an OpenConfig interface with these attributes.
func (a *Attributes) ConfigInterface(intf *oc.Interface) *oc.Interface {
	if a.Desc != "" {
		intf.Description = ygot.String(a.Desc)
	}
	intf.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		intf.Enabled = ygot.Bool(true)
	}
	if a.MTU > 0 && !*deviations.OmitL2MTU {
		intf.Mtu = ygot.Uint16(a.MTU + 14)
	}
	e := intf.GetOrCreateEthernet()
	if a.MAC != "" {
		e.MacAddress = ygot.String(a.MAC)
	}

	s := intf.GetOrCreateSubinterface(0)
	if a.IPv4 != "" {
		s4 := s.GetOrCreateIpv4()
		if *deviations.InterfaceEnabled && !*deviations.IPv4MissingEnabled {
			s4.Enabled = ygot.Bool(true)
		}
		if a.MTU > 0 {
			s4.Mtu = ygot.Uint16(a.MTU)
		}
		a4 := s4.GetOrCreateAddress(a.IPv4)
		if a.IPv4Len > 0 {
			a4.PrefixLength = ygot.Uint8(a.IPv4Len)
		}
	}

	if a.IPv6 != "" {
		s6 := s.GetOrCreateIpv6()
		if a.MTU > 0 {
			s6.Mtu = ygot.Uint32(uint32(a.MTU))
		}
		if *deviations.InterfaceEnabled {
			s6.Enabled = ygot.Bool(true)
		}
		a6 := s6.GetOrCreateAddress(a.IPv6)
		if a.IPv6Len > 0 {
			a6.PrefixLength = ygot.Uint8(a.IPv6Len)
		}
	}
	return intf
}

// NewInterface returns a new *oc.Interface configured with these attributes
func (a *Attributes) NewInterface(name string) *oc.Interface {
	return a.ConfigInterface(&oc.Interface{Name: ygot.String(name)})
}

// AddToATE adds a new interface to an ATETopology with these attributes.
func (a *Attributes) AddToATE(top *ondatra.ATETopology, ap *ondatra.Port, peer *Attributes) *ondatra.Interface {
	i := top.AddInterface(a.Name).WithPort(ap)
	if a.MTU > 0 {
		i.Ethernet().WithMTU(a.MTU)
	}
	if a.IPv4 != "" {
		i.IPv4().
			WithAddress(a.IPv4CIDR()).
			WithDefaultGateway(peer.IPv4)
	}
	if a.IPv6 != "" {
		i.IPv6().
			WithAddress(a.IPv6CIDR()).
			WithDefaultGateway(peer.IPv6)
	}
	return i
}

// AddToOTG adds basic elements to a gosnappi configuration
func (a *Attributes) AddToOTG(top gosnappi.Config, ap *ondatra.Port, peer *Attributes) {
	top.Ports().Add().SetName(ap.ID())
	dev := top.Devices().Add().SetName(a.Name)
	eth := dev.Ethernets().Add().SetName(a.Name + ".Eth")
	eth.SetPortName(ap.ID()).SetMac(a.MAC)

	if a.MTU > 0 {
		eth.SetMtu(int32(a.MTU))
	}
	if a.IPv4 != "" {
		ip := eth.Ipv4Addresses().Add().SetName(dev.Name() + ".IPv4")
		ip.SetAddress(a.IPv4).SetGateway(peer.IPv4).SetPrefix(int32(a.IPv4Len))
	}
	if a.IPv6 != "" {
		ip := eth.Ipv6Addresses().Add().SetName(dev.Name() + ".IPv6")
		ip.SetAddress(a.IPv6).SetGateway(peer.IPv6).SetPrefix(int32(a.IPv6Len))
	}
}
