// Package networkinstance implements the Config Library for NetworkInstance
// feature.
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

// NetworkInstance struct stores the OC attributes.
type NetworkInstance struct {
	oc *oc.NetworkInstance
}

// New returns the new NetworkInstance object with specified name and type.
func New(name string, niType oc.E_NetworkInstanceTypes_NETWORK_INSTANCE_TYPE) *NetworkInstance {
	oc := &oc.NetworkInstance{
		Name:    ygot.String(name),
		Type:    niType,
		Enabled: ygot.Bool(true),
	}
	return &NetworkInstance{oc: oc}
}

// AugmentDevice method augments the provided device OC with NetworkInstance
// feature.
// Use d.WithFeature(ni) instead of calling this method directly.
func (ni *NetworkInstance) AugmentDevice(d *oc.Device) error {
	if ni == nil || d == nil {
		return errors.New("either ni or device is nil")
	}
	return d.AppendNetworkInstance(ni.oc)
}

// NetworkInstanceFeature provides interface to augment additional features to NI.
type NetworkInstanceFeature interface {
	// AugmentNetworkInstancd augments NI with provided feature.
	AugmentNetworkInstance(ni *oc.NetworkInstance) error
}

// WithFeature augments the provided feature to network-instance.
func (ni *NetworkInstance) WithFeature(f NetworkInstanceFeature) error {
	if ni == nil || f == nil {
		return errors.New("some args are nil")
	}
	return f.AugmentNetworkInstance(ni.oc)
}
