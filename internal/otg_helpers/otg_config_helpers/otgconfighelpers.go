// Package otgconfighelpers provides helper functions to setup Protocol configurations on traffic generators.
package otgconfighelpers

import (
	"testing"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/ondatra"
)

/*
Port is a struct to hold aggregate/physical port data.

Usage for Aggregate port:

	  agg1 = &otgconfighelpers.Port{
		Name:        "Port-Channel1",
		AggMAC:      "02:00:01:01:01:07",
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface1, interface2, interface3, interface4, interface5, interface6},
		MemberPorts: []string{"port1", "port2"},
		LagID:       1,
		Islag:       true,
	}

Usage for Physical port:

	phy1 = &otgconfighelpers.Port{
		Name:        "port1",
		Interfaces:  []*otgconfighelpers.InterfaceProperties{interface1, interface2, interface3, interface4, interface5, interface6},
	}

Interface properties has attributes required for creating interfaces/subinterfaces on the port..

	interface1 = &otgconfighelpers.InterfaceProperties{
		IPv4:        "169.254.0.12",
		IPv4Gateway: "169.254.0.11",
		Name:        "Port-Channel1.20",
		Vlan:        20,
		IPv4Len:     29,
		Mac:         "02:00:01:01:01:08",
	}
*/
type Port struct {
	Name        string
	AggMAC      string
	Interfaces  []*InterfaceProperties
	MemberPorts []string
	LagID       uint32
	Islag       bool
}

// InterfaceProperties is a struct to hold interface data.
type InterfaceProperties struct {
	IPv4        string
	IPv4Gateway string
	IPv6        string
	IPv6Gateway string
	Name        string
	Vlan        uint32
	IPv4Len     uint32
	IPv6Len     uint32
	Mac         string
}

// ConfigureOtgNetworkInterface configures the network interface.
func ConfigureOtgNetworkInterface(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice, a *Port) {
	if a.Islag {
		ConfigureOtgLag(t, top, ate, a)
	} else {
		top.Ports().Add().SetName(a.Name)
	}
	for _, intf := range a.Interfaces {
		ConfigureOtgInterface(t, top, intf, a)
	}
}

// ConfigureOtgLag configures the aggregate port.
func ConfigureOtgLag(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice, a *Port) {
	agg := top.Lags().Add().SetName(a.Name)
	agg.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId(a.AggMAC)
	for index, portName := range a.MemberPorts {
		p := ate.Port(t, portName)
		top.Ports().Add().SetName(p.ID())
		ConfigureOtgLagMemberPort(t, top, agg, p.ID(), a, index)
	}
}

// ConfigureOtgLagMemberPort configures the member port in the LAG.
func ConfigureOtgLagMemberPort(t *testing.T, top gosnappi.Config, agg gosnappi.Lag, portID string, a *Port, index int) {
	lagPort := agg.Ports().Add().SetPortName(portID)
	lagPort.Ethernet().SetMac(a.AggMAC).SetName(a.Name + "-" + portID)
	lagPort.Lacp().SetActorActivity("active").SetActorPortNumber(uint32(index) + 1).SetActorPortPriority(1).SetLacpduTimeout(0)
}

// ConfigureOtgInterface configures the Ethernet for the LAG or subinterface.
func ConfigureOtgInterface(t *testing.T, top gosnappi.Config, intf *InterfaceProperties, a *Port) {
	dev := top.Devices().Add().SetName(intf.Name + ".Dev")
	eth := dev.Ethernets().Add().SetName(intf.Name + ".Eth").SetMac(intf.Mac)
	if a.Islag {
		eth.Connection().SetLagName(a.Name)
  } else {
		eth.Connection().SetPortName(a.Name)
	}
	// VLAN configuration
	if intf.Vlan != 0 {
		eth.Vlans().Add().SetName(intf.Name + ".vlan").SetId(intf.Vlan)
	}
	// IP address configuration
	if intf.IPv4 != "" {
		eth.Ipv4Addresses().Add().SetName(intf.Name + ".IPv4").SetAddress(intf.IPv4).SetGateway(intf.IPv4Gateway).SetPrefix(intf.IPv4Len)
	}
	if intf.IPv6 != "" {
		eth.Ipv6Addresses().Add().SetName(intf.Name + ".IPv6").SetAddress(intf.IPv6).SetGateway(intf.IPv6Gateway).SetPrefix(intf.IPv6Len)
	}
}
