package networkinstance

import (
	"errors"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// To enable default VRF on a device, follow these steps:
//
// Step 1: Create device.
// d := device.New()
//
// Step 2: Create default NI on device.
// ni := networkinstance.Enabled("default", oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
// d.WithFeature(ni)
//

type NetworkInstance struct {
	enabled bool
	name    string
	niType  oc.E_NetworkInstanceTypes_NETWORK_INSTANCE_TYPE
	oc      *oc.NetworkInstance
}

func Enabled(name string, niType oc.E_NetworkInstanceTypes_NETWORK_INSTANCE_TYPE) *NetworkInstance {
	return &NetworkInstance{enabled: true, name: name, niType: niType}
}

func (ni *NetworkInstance) AugmentDevice(d *oc.Device) error {
	if ni == nil || d == nil {
		return errors.New("either network-instance or device is nil")
	}
	ni.oc = d.GetOrCreateNetworkInstance(ni.name)
	if ni.enabled {
		ni.oc.Enabled = ygot.Bool(true)
	}
	if ni.niType != oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_UNSET {
		ni.oc.Type = ni.niType
	}
	return nil
}

type NIFeature interface {
	AugmentNetworkInstance(ni *oc.NetworkInstance) error
}

func (ni *NetworkInstance) WithFeature(f NIFeature) error {
	if ni == nil || f == nil {
		return nil
	}
	return f.AugmentNetworkInstance(ni.oc)
}
