package bgp

import (
	"errors"
	"time"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

//
// To configure BGP neighbor:
//
// device.New()
//    .WithFeature(networkinstance.New("default", oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
//         .WithFeature(bgp.New()
//              .WithAS(1234)
//              .WithFeature(bgp.NewNeighbor("1.2.3.4"))))
//

// PrefixLimit struct to hold prefix limit attributes.
type PrefixLimit struct {
	MaxPrefixes         uint32
	PreventTeardown     bool
	RestartTimer        time.Duration
	WarningThresholdPct uint8
}

// Timer struct to hold timer attributes.
type Timers struct {
	MinAdvertisementIntvl time.Duration
	HoldTime              time.Duration
	KeepaliveIntvl        time.Duration
	ConnectRetry          time.Duration
}

// Transport struct to hold transport attributes.
type Transport struct {
	PassiveMode  bool
	TCPMSS       uint16
	MTUDiscovery bool
	LocalAddr    string
}

// Neighbor struct to hold BGP neighbor OC attributes.
type Neighbor struct {
	oc *oc.NetworkInstance_Protocol_Bgp_Neighbor
}

// NewNeighbor returns a new Neighbor object.
func NewNeighbor(addr string) *Neighbor {
	oc := &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		NeighborAddress: ygot.String(addr),
	}
	return &Neighbor{
		oc: oc,
	}
}

// Address returns the neighbor address.
func (n *Neighbor) Address() string {
	return n.oc.GetNeighborAddress()
}

// WithAfiSafi adds specified AFI-SAFI type to neighbor.
func (n *Neighbor) WithAFISAFI(name oc.E_BgpTypes_AFI_SAFI_TYPE) *Neighbor {
	if n == nil {
		return nil
	}
	n.oc.GetOrCreateAfiSafi(name).Enabled = ygot.Bool(true)
	return n
}

// WithPeerGroup sets the peer-group for neighbor.
func (n *Neighbor) WithPeerGroup(pg string) *Neighbor {
	if n == nil {
		return nil
	}
	n.oc.PeerGroup = ygot.String(pg)
	return n
}

// WithLogStateChanges enables neighbor state changes logging.
func (n *Neighbor) WithLogStateChanges(val bool) *Neighbor {
	if n == nil {
		return nil
	}
	n.oc.GetOrCreateLoggingOptions().LogNeighborStateChanges = ygot.Bool(val)
	return n
}

// WithAuthPassword sets auth password on neighbor.
func (n *Neighbor) WithAuthPassword(pwd string) *Neighbor {
	if n == nil {
		return nil
	}
	n.oc.AuthPassword = ygot.String(pwd)
	return n
}

// WithDescription sets the neighbor description.
func (n *Neighbor) WithDescription(desc string) *Neighbor {
	if n == nil {
		return nil
	}
	n.oc.Description = ygot.String(desc)
	return n
}

// WithTransport sets the transport attributes on neighbor.
func (n *Neighbor) WithTransport(t *Transport) *Neighbor {
	if n == nil || t == nil {
		return nil
	}
	toc := n.oc.GetOrCreateTransport()
	toc.PassiveMode = ygot.Bool(t.PassiveMode)
	if t.TCPMSS > 0 {
		toc.TcpMss = ygot.Uint16(t.TCPMSS)
	}
	toc.MtuDiscovery = ygot.Bool(t.MTUDiscovery)
	if t.LocalAddr != "" {
		toc.LocalAddress = ygot.String(t.LocalAddr)
	}
	return n
}

// WithLocalAS sets the local AS on the neighbor.
func (n *Neighbor) WithLocalAS(as uint32) *Neighbor {
	if n == nil {
		return nil
	}
	n.oc.LocalAs = ygot.Uint32(as)
	return n
}

// WithPeerAS sets the peer AS on the neighbor.
func (n *Neighbor) WithPeerAS(as uint32) *Neighbor {
	if n == nil {
		return nil
	}
	n.oc.PeerAs = ygot.Uint32(as)
	return n
}

// WithPeerType sets the peer type on the neighbor.
func (n *Neighbor) WithPeerType(pt oc.E_BgpTypes_PeerType) *Neighbor {
	if n == nil {
		return nil
	}
	n.oc.PeerType = pt
	return n
}

// WithRemovePrivateAS specifies that private AS should be removed.
func (n *Neighbor) WithRemovePrivateAS(val oc.E_BgpTypes_RemovePrivateAsOption) *Neighbor {
	if n == nil {
		return nil
	}
	n.oc.RemovePrivateAs = val
	return n
}

// WithSendCommunity sets the send-community on the neighbor.
func (n *Neighbor) WithSendCommunity(sc oc.E_BgpTypes_CommunityType) *Neighbor {
	if n == nil {
		return nil
	}
	n.oc.SendCommunity = sc
	return n
}

// WithV4PrefixLimit sets the IPv4 prefix limits on the neighbor.
func (n *Neighbor) WithV4PrefixLimit(pl *PrefixLimit) *Neighbor {
	if n == nil || pl == nil {
		return nil
	}
	ploc := n.oc.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
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
	return n
}

// WithTimers sets the timers on the neighbor.
func (n *Neighbor) WithTimers(t *Timers) *Neighbor {
	if n == nil || t == nil {
		return nil
	}
	toc := n.oc.GetOrCreateTimers()
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
	return n
}

// AugmentBGP augments the BGP with neighbor configuration.
// Use bgp.WithFeature(n) instead of calling this method directly.
func (n *Neighbor) AugmentBGP(bgp *oc.NetworkInstance_Protocol_Bgp) error {
	if n == nil || bgp == nil {
		return errors.New("some args are nil")
	}
	return bgp.AppendNeighbor(n.oc)
}

// NeighborFeature provides interface to augment the neighbor OC with
// additional features.
type NeighborFeature interface {
	// AugmentNeighbor augments the neighbor OC with additional feature.
	AugmentNeighbor(oc *oc.NetworkInstance_Protocol_Bgp_Neighbor) error
}

// WithFeature augments the neighbor OC with additional feature.
func (n *Neighbor) WithFeature(f NeighborFeature) error {
	if n == nil || f == nil {
		return nil
	}
	return f.AugmentNeighbor(n.oc)
}
