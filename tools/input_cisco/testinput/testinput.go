package testinput

import (
	"github.com/openconfig/featureprofiles/tools/input_cisco/proto"
	"github.com/openconfig/ondatra"
)

// TestInput is an interface that allows for config operations on an Input file
type TestInput interface {
	UnConfigProtocols(dev *ondatra.DUTDevice)
	ConfigProtocols(dev *ondatra.DUTDevice)
	Config(dev *ondatra.DUTDevice)
	ConfigInterfaces(dev *ondatra.DUTDevice)
	ConfigVrf(dev *ondatra.DUTDevice)
	ReplaceVrf(dev *ondatra.DUTDevice)
	UnConfigVrf(dev *ondatra.DUTDevice)
	ConfigRPL(dev *ondatra.DUTDevice)
	ReplaceRPL(dev *ondatra.DUTDevice)
	UnConfigRPL(dev *ondatra.DUTDevice)
	ConfigBGP(dev *ondatra.DUTDevice)
	ConfigISIS(dev *ondatra.DUTDevice)
	ConfigJson(dev *ondatra.DUTDevice)
	UnConfig(dev *ondatra.DUTDevice)
	UnConfigBGP(dev *ondatra.DUTDevice)
	UnConfigISIS(dev *ondatra.DUTDevice)
	UnConfigInterfaces(dev *ondatra.DUTDevice)
	Device(dev *ondatra.DUTDevice) Device
	ATE(dev *ondatra.ATEDevice) ATE
}

// Device interface provides methods for accessing Device properties
type Device interface {
	IFGroup(groupName string) IfGroup
	Features() *proto.Input_Feature
	GetInterface(name string) Intf
}

// ATE interface provides methods for accessing ATE properties
type ATE interface {
	IFGroup(groupName string) IfGroup
	Features() *proto.Input_Feature
	GetInterface(name string) Intf
}

// IfGroup provides an interface to get grouing data from input file
type IfGroup interface {
	Names() []string
	Ipv4Addresses() []string
	Ipv4AddressMasks() []string
	Ipv6Addresses() []string
	Ipv6AddressMasks() []string
}

// Intf acts as an interface for collecting Interface data
type Intf interface {
	Name() string
	ID() string
	Ipv4Address() string
	Ipv4AddressMask() string
	Ipv6Address() string
	Ipv6AddressMask() string
	Ipv4PrefixLength() uint8
	Ipv6PrefixLength() uint8
	Vrf() string
	Group() string
	Members() []string
}
