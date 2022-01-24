package lldp

import (
	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// To enable LLDP on a device:
//
// device.New().WithFeature(lldp.Enabled())

type LLDP struct {
	enabled bool
	oc      *oc.Lldp
}

func Enabled() *LLDP {
	return &LLDP{enabled: true}
}

func (l *LLDP) AugmentDevice(d *oc.Device) error {
	if l.enabled {
		l.oc = d.GetOrCreateLldp()
		l.oc.Enabled = ygot.Bool(true)
	}
	return nil
}
