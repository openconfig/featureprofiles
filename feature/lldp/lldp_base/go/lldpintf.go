package lldp

import (
	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

//
// To enable LLDP on an interface:
//
// The interface config for LLDP is under the /lldp/ prefix,
// which requires the device root for configuration.
//
// We need to add interface to the device root before we can
// configure any features on the interface.
//
// d := device.New()
// i := interface.New("Ethernet1", d.Root())
// d.WithFeature(i)
//
// i.WithFeature(lldp.IntfEnabled())
//

type LLDPIntf struct {
	enabled bool
}

func IntfEnabled() *LLDPIntf {
	return &LLDPIntf{enabled: true}
}

func (li *LLDPIntf) AugmentInterface(d *oc.Device, i *oc.Interface) error {
	if li.enabled {
		d.GetOrCreateLldp().GetOrCreateInterface(i.GetName()).Enabled = ygot.Bool(true)
	}
	return nil
}
