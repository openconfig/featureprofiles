package bgp

import (
	"errors"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

//
// To enable BGP on NI, follow these steps:
//
// Step 1: Create device.
// d := device.New()
//
// Step 2: Create default NI on device.
// ni := networkinstance.Enabled("default", oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
// d.WithFeature(ni)
//
// Step 3: Enable BGP on default NI (with some params)
// bgp := bgp.AddBGP()
//              .WithAS(1234)
//              .WithRouterID("1.2.3.4")
// ni.WithFeature(bgp)
//

type BGP struct {
	as       uint32
	routerID string
	oc       *oc.NetworkInstance_Protocol_Bgp
}

func AddBGP() *BGP {
	return &BGP{}
}

func (b *BGP) WithAS(as uint32) *BGP {
	b.as = as
	return b
}

func (b *BGP) WithRouterID(rID string) *BGP {
	b.routerID = rID
	return b
}

func (b *BGP) AugmentNetworkInstance(ni *oc.NetworkInstance) error {
	if b == nil || ni == nil {
		return errors.New("either bgp or network-instance is nil")
	}
	b.oc = ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "bgp").GetOrCreateBgp()
	goc := b.oc.GetOrCreateGlobal()
	if b.as != 0 {
		goc.As = ygot.Uint32(b.as)
	}
	if b.routerID != "" {
		goc.RouterId = ygot.String(b.routerID)
	}
	return nil
}

type BGPFeature interface {
	AugmentBGP(oc *oc.NetworkInstance_Protocol_Bgp) error
}

func (b *BGP) WithFeature(f BGPFeature) error {
	if b == nil || f == nil {
		return nil
	}
	return f.AugmentBGP(b.oc)
}
