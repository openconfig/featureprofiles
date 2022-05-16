package testinput

import (
	"github.com/openconfig/featureprofiles/tools/input_cisco/proto"
	"github.com/openconfig/ondatra"
)

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
}
type Device interface {
	IFGroup(group_name string) IfGroup
	Features() *proto.Input_Feature
	GetInterface(name string) Intf
}

type IfGroup interface {
	Names() []string
	Ipv4Addresses() []string
	Ipv4AddressMasks() []string
	Ipv6Addresses() []string
	Ipv6AddressMasks() []string
}

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
