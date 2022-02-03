package bgp

import (
	"errors"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

//
// To configure BGP peer group:
//
// device.New()
//    .WithFeature(networkinstance.New("default", oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
//         .WithFeature(bgp.New()
//              .WithAS(1234)
//              .WithFeature(bgp.NewPeerGroup("GLOBAL-PEER"))))
//

// PeerGroup struct to store OC attributes.
type PeerGroup struct {
	oc *oc.NetworkInstance_Protocol_Bgp_PeerGroup
}

// NewPeerGroup returns a new peer-group object.
func NewPeerGroup(name string) *PeerGroup {
	oc := &oc.NetworkInstance_Protocol_Bgp_PeerGroup{
		PeerGroupName: ygot.String(name),
	}
	return &PeerGroup{oc: oc}
}

// Name returns the name of the peer-group.
func (pg *PeerGroup) Name() string {
	return pg.oc.GetPeerGroupName()
}

// WithAfiSafi adds specified AFI-SAFI type to peer-group.
func (pg *PeerGroup) WithAFISAFI(name oc.E_BgpTypes_AFI_SAFI_TYPE) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.oc.GetOrCreateAfiSafi(name).Enabled = ygot.Bool(true)
	return pg
}

// WithAuthPassword sets auth password on peer-group.
func (pg *PeerGroup) WithAuthPassword(pwd string) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.oc.AuthPassword = ygot.String(pwd)
	return pg
}

// WithDescription sets the peer-group descriptiopg.
func (pg *PeerGroup) WithDescription(desc string) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.oc.Description = ygot.String(desc)
	return pg
}

// WithTransport sets the transport attributes on peer-group.
func (pg *PeerGroup) WithTransport(t *Transport) *PeerGroup {
	if pg == nil || t == nil {
		return nil
	}
	toc := pg.oc.GetOrCreateTransport()
	toc.PassiveMode = ygot.Bool(t.PassiveMode)
	if t.TCPMSS > 0 {
		toc.TcpMss = ygot.Uint16(t.TCPMSS)
	}
	toc.MtuDiscovery = ygot.Bool(t.MTUDiscovery)
	if t.LocalAddr != "" {
		toc.LocalAddress = ygot.String(t.LocalAddr)
	}
	return pg
}

// WithLocalAS sets the local AS on the peer-group.
func (pg *PeerGroup) WithLocalAS(as uint32) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.oc.LocalAs = ygot.Uint32(as)
	return pg
}

// WithPeerAS sets the peer AS on the peer-group.
func (pg *PeerGroup) WithPeerAS(as uint32) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.oc.PeerAs = ygot.Uint32(as)
	return pg
}

// WithPeerType sets the peer type on the peer-group.
func (pg *PeerGroup) WithPeerType(pt oc.E_BgpTypes_PeerType) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.oc.PeerType = pt
	return pg
}

// WithRemovePrivateAS specifies that private AS should be removed.
func (pg *PeerGroup) WithRemovePrivateAS(val oc.E_BgpTypes_RemovePrivateAsOption) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.oc.RemovePrivateAs = val
	return pg
}

// WithSendCommunity sets the send-community on the peer-group.
func (pg *PeerGroup) WithSendCommunity(sc oc.E_BgpTypes_CommunityType) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.oc.SendCommunity = sc
	return pg
}

// WithV4PrefixLimit sets the IPv4 prefix limits on the peer-group.
func (pg *PeerGroup) WithV4PrefixLimit(pl *PrefixLimit) *PeerGroup {
	if pg == nil || pl == nil {
		return nil
	}
	ploc := pg.oc.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
	if pl.MaxPrefixes > 0 {
		ploc.MaxPrefixes = ygot.Uint32(pl.MaxPrefixes)
	}
	ploc.PreventTeardown = ygot.Bool(pl.PreventTeardown)
	if pl.RestartTimer != 0 {
		ploc.RestartTimer = ygot.Float64(pl.RestartTimer.Seconds())
	}
	if pl.WarningThresholdPct > 0 {
		ploc.WarningThresholdPct = ygot.Uint8(pl.WarningThresholdPct)
	}
	return pg
}

// WithTimers sets the timers on the peer-group.
func (pg *PeerGroup) WithTimers(t *Timers) *PeerGroup {
	if pg == nil || t == nil {
		return nil
	}
	toc := pg.oc.GetOrCreateTimers()
	if t.MinAdvertisementIntvl != 0 {
		toc.MinimumAdvertisementInterval = ygot.Float64(t.MinAdvertisementIntvl.Seconds())
	}
	if t.HoldTime != 0 {
		toc.HoldTime = ygot.Float64(t.HoldTime.Seconds())
	}
	if t.KeepaliveIntvl != 0 {
		toc.KeepaliveInterval = ygot.Float64(t.KeepaliveIntvl.Seconds())
	}
	if t.ConnectRetry != 0 {
		toc.ConnectRetry = ygot.Float64(t.ConnectRetry.Seconds())
	}
	return pg
}

// AugmentBGP augments the BGP with peer-group configuration.
// Use bgp.WithFeature(pg) instead of calling this method directly.
func (pg *PeerGroup) AugmentBGP(bgp *oc.NetworkInstance_Protocol_Bgp) error {
	if pg == nil || bgp == nil {
		return errors.New("some args are nil")
	}
	return bgp.AppendPeerGroup(pg.oc)
}

// PeerGroupFeature provides interface to augment peer-group with
// additional features.
type PeerGroupFeature interface {
	// AugmentPeerGroup augments peer-group with additional feature.
	AugmentPeerGroup(oc *oc.NetworkInstance_Protocol_Bgp_PeerGroup) error
}

// WithFeature augments peer-group with provided feature.
func (pg *PeerGroup) WithFeature(f PeerGroupFeature) error {
	if pg == nil || f == nil {
		return nil
	}
	return f.AugmentPeerGroup(pg.oc)
}
