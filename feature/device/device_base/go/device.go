package device

import "github.com/openconfig/featureprofiles/yang/oc"

//
// Create device:
// d := device.New()
//
// Add feature:
// d.WithFeature(somefeature)
//

type Device struct {
	oc *oc.Device
}

type DeviceFeature interface {
	AugmentDevice(d *oc.Device) error
}

func New() *Device {
	d := &Device{}
	d.oc = &oc.Device{}
	return d
}

func (d *Device) Root() *oc.Device {
	return d.oc
}

func (d *Device) WithFeature(f DeviceFeature) error {
	return f.AugmentDevice(d.oc)
}
