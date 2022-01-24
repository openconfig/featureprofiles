package intf

import (
	"errors"

	"github.com/openconfig/featureprofiles/yang/oc"
)

type Interface struct {
	name string
	d    *oc.Device
	oc   *oc.Interface
}

type Feature interface {
	AugmentInterface(d *oc.Device, i *oc.Interface) error
}

func New(name string, d *oc.Device) *Interface {
	return &Interface{name: name, d: d}
}

func (i *Interface) validate() error {
	if i.name == "" {
		return errors.New("interface name is required")
	}
	return nil
}

func (i *Interface) AugmentDevice(d *oc.Device) error {
	if i == nil {
		return nil
	}
	if err := i.validate(); err != nil {
		return err
	}
	i.oc = d.GetOrCreateInterface(i.name)
	// TODO(sthesayi): Other OC for interface
	return nil
}

func (i *Interface) WithFeature(f Feature) error {
	if i == nil {
		return nil
	}
	if i.d == nil || i.oc == nil {
		return errors.New("interface is not augmented to device yet")
	}
	return f.AugmentInterface(i.d, i.oc)
}
