// Package device implements the Config Library for device feature profile.
package device

import (
	"errors"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

//
// Create device:
// d := device.New()
//
// Add feature:
// d.WithFeature(somefeature)
//

// Device struct to store OC attributes.
type Device struct {
	oc *oc.Device
}

// New returns a new Device objct.
func New() *Device {
	return &Device{oc: &oc.Device{}}
}

// DeepCopy returns a deep copy of Device OC struct.
func (d *Device) DeepCopy() (*oc.Device, error) {
	dcopy, err := ygot.DeepCopy(d.oc)
	if err != nil {
		return nil, err
	}
	return dcopy.(*oc.Device), nil
}

// DeviceFeature is a feature on the device.
type DeviceFeature interface {
	// AugmentDevice augments the device OC with this feature.
	AugmentDevice(d *oc.Device) error
}

// WithFeature augments the device with the provided feature.
func (d *Device) WithFeature(f DeviceFeature) error {
	if f == nil {
		return errors.New("feature is nil")
	}
	return f.AugmentDevice(d.oc)
}
