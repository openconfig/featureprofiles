/*
 Copyright 2022 Google LLC

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

      https://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package bgp

import (
	"errors"
	"time"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// Neighbor struct to hold BGP neighbor OC attributes.
type Neighbor struct {
	oc oc.NetworkInstance_Protocol_Bgp_Neighbor
}

// NewNeighbor returns a new Neighbor object.
func NewNeighbor(addr string) *Neighbor {
	return &Neighbor{
		oc: oc.NetworkInstance_Protocol_Bgp_Neighbor{
			NeighborAddress: ygot.String(addr),
		},
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

// Transport struct to hold transport attributes.
type Transport struct {
	PassiveMode  bool
	TCPMSS       uint16
	MTUDiscovery bool
	LocalAddress string
}

// WithTransport sets the transport attributes on neighbor.
func (n *Neighbor) WithTransport(t Transport) *Neighbor {
	if n == nil {
		return nil
	}
	toc := n.oc.GetOrCreateTransport()
	toc.PassiveMode = ygot.Bool(t.PassiveMode)
	if t.TCPMSS > 0 {
		toc.TcpMss = ygot.Uint16(t.TCPMSS)
	}
	toc.MtuDiscovery = ygot.Bool(t.MTUDiscovery)
	if t.LocalAddress != "" {
		toc.LocalAddress = ygot.String(t.LocalAddress)
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

// PrefixLimit struct to hold prefix limit attributes.
type PrefixLimit struct {
	MaxPrefixes         uint32
	PreventTeardown     bool
	RestartTimer        time.Duration
	WarningThresholdPct uint8
}

// WithV4PrefixLimit sets the IPv4 prefix limits on the neighbor.
func (n *Neighbor) WithV4PrefixLimit(pl PrefixLimit) *Neighbor {
	if n == nil {
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

// Timer struct to hold timer attributes.
type Timers struct {
	MinimumAdvertisementInterval time.Duration
	HoldTime                     time.Duration
	KeepaliveInterval            time.Duration
	ConnectRetry                 time.Duration
}

// WithTimers sets the timers on the neighbor.
func (n *Neighbor) WithTimers(t Timers) *Neighbor {
	if n == nil {
		return nil
	}
	toc := n.oc.GetOrCreateTimers()
	if t.MinimumAdvertisementInterval != 0 {
		toc.MinimumAdvertisementInterval = ygot.Float64(t.MinimumAdvertisementInterval.Seconds())
	}
	if t.HoldTime != 0 {
		toc.HoldTime = ygot.Float64(t.HoldTime.Seconds())
	}
	if t.KeepaliveInterval != 0 {
		toc.KeepaliveInterval = ygot.Float64(t.KeepaliveInterval.Seconds())
	}
	if t.ConnectRetry != 0 {
		toc.ConnectRetry = ygot.Float64(t.ConnectRetry.Seconds())
	}
	return n
}

// AugmentGlobal implements the bgp.GlobalFeature interface.
// This method augments the BGP OC with neighbor configuration.
// Use bgp.WithFeature(n) instead of calling this method directly.
func (n *Neighbor) AugmentGlobal(bgp *oc.NetworkInstance_Protocol_Bgp) error {
	if n == nil || bgp == nil {
		return errors.New("some args are nil")
	}
	return bgp.AppendNeighbor(&n.oc)
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
	return f.AugmentNeighbor(&n.oc)
}
