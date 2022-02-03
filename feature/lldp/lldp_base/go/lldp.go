// Package lldp implements the Config Library for LLDP feature profile.
package lldp

import (
	"errors"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// To enable LLDP on a device:
//
// device.New().WithFeature(lldp.New())

// LLDP struct to store OC attributes.
type LLDP struct {
	oc *oc.Lldp
}

// New returns a new LLDP object with the feature enabled.
func New() *LLDP {
	oc := &oc.Lldp{
		Enabled: ygot.Bool(true),
	}
	return &LLDP{oc: oc}
}

// WithInterface enables LLDP on the specified interface.
func (l *LLDP) WithInterface(name string) *LLDP {
	if l == nil {
		return nil
	}
	l.oc.GetOrCreateInterface(name).Enabled = ygot.Bool(true)
	return l
}

// AugmentDevice augments the device OC with LLDP feature.
// Use d.WithFeature(l) instead of calling this method directly.
func (l *LLDP) AugmentDevice(d *oc.Device) error {
	if l == nil || d == nil {
		return errors.New("some args are nil")
	}
	if d.Lldp != nil {
		return errors.New("lldp OC is not nil")
	}
	d.Lldp = l.oc
	return nil
}
