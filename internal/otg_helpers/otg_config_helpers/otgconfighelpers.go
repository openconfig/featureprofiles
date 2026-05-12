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
	IsLag       bool
	IsLo0Needed bool
	IsMTU       bool
	MTU         uint32
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
	MAC         string
	LoopbackV4  string
	LoopbackV6  string
}

// ConfigureNetworkInterface configures the network interface.
func ConfigureNetworkInterface(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice, a *Port) {
	t.Helper()
	if a.IsLag {
		ConfigureLag(t, top, ate, a)
	} else {
		top.Ports().Add().SetName(a.Name)
	}
	for _, intf := range a.Interfaces {
		ConfigureInterface(top, intf, a)
	}
}

// ConfigureLag configures the aggregate port.
func ConfigureLag(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice, a *Port) {
	t.Helper()
	agg := top.Lags().Add().SetName(a.Name)
	agg.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId(a.AggMAC)
	for index, portName := range a.MemberPorts {
		p := ate.Port(t, portName)
		top.Ports().Add().SetName(p.ID())
		ConfigureLagMemberPort(agg, p.ID(), a, index)
	}
}

// ConfigureLagMemberPort configures the member port in the LAG.
func ConfigureLagMemberPort(agg gosnappi.Lag, portID string, a *Port, index int) {
	lagPort := agg.Ports().Add().SetPortName(portID)
	lagPort.Ethernet().SetMac(a.AggMAC).SetName(a.Name + "-" + portID)
	lagPort.Lacp().SetActorActivity("active").SetActorPortNumber(uint32(index) + 1).SetActorPortPriority(1).SetLacpduTimeout(0)
	if a.IsMTU {
		lagPort.Ethernet().SetMtu(a.MTU)
	}
}

// ConfigureInterface configures the Ethernet for the LAG or subinterface.
func ConfigureInterface(top gosnappi.Config, intf *InterfaceProperties, a *Port) {
	dev := top.Devices().Add().SetName(intf.Name + ".Dev")
	eth := dev.Ethernets().Add().SetName(intf.Name + ".Eth").SetMac(intf.MAC)
	if a.IsLag {
		eth.Connection().SetLagName(a.Name)
	} else {
		eth.Connection().SetPortName(a.Name)
	}
	// MTU configuration
	if a.IsMTU && !a.IsLag {
		eth.SetMtu(a.MTU)
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
	// Loopback configuration
	if a.IsLo0Needed {
		if intf.LoopbackV4 != "" {
			iDut4LoopV4 := dev.Ipv4Loopbacks().Add().SetName(intf.Name + ".Loopback.IPv4").SetEthName(intf.Name + ".Eth")
			iDut4LoopV4.SetAddress(intf.LoopbackV4)
		}
		if intf.LoopbackV6 != "" {
			iDut4LoopV6 := dev.Ipv6Loopbacks().Add().SetName(intf.Name + ".Loopback.IPv6").SetEthName(intf.Name + ".Eth")
			iDut4LoopV6.SetAddress(intf.LoopbackV6)
		}
	}
}
