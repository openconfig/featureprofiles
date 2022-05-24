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
	"time"

	"github.com/openconfig/featureprofiles/yang/fpoc"
	"github.com/openconfig/ygot/ygot"
)

// Neighbor struct to hold BGP neighbor OC attributes.
type Neighbor struct {
	oc fpoc.NetworkInstance_Protocol_Bgp_Neighbor
}

// NewNeighbor returns a new Neighbor object.
func NewNeighbor(addr string) *Neighbor {
	return &Neighbor{
		oc: fpoc.NetworkInstance_Protocol_Bgp_Neighbor{
			NeighborAddress: ygot.String(addr),
		},
	}
}

// Address returns the neighbor address.
func (n *Neighbor) Address() string {
	return n.oc.GetNeighborAddress()
}

// WithAFISAFI adds specified AFI-SAFI type to neighbor.
func (n *Neighbor) WithAFISAFI(name fpoc.E_BgpTypes_AFI_SAFI_TYPE) *Neighbor {
	n.oc.GetOrCreateAfiSafi(name).Enabled = ygot.Bool(true)
	return n
}

// WithPeerGroup sets the peer-group for neighbor.
func (n *Neighbor) WithPeerGroup(pg string) *Neighbor {
	n.oc.PeerGroup = ygot.String(pg)
	return n
}

// WithLogStateChanges enables neighbor state changes logging.
func (n *Neighbor) WithLogStateChanges(val bool) *Neighbor {
	n.oc.GetOrCreateLoggingOptions().LogNeighborStateChanges = ygot.Bool(val)
	return n
}

// WithAuthPassword sets auth password on neighbor.
func (n *Neighbor) WithAuthPassword(pwd string) *Neighbor {
	n.oc.AuthPassword = ygot.String(pwd)
	return n
}

// WithDescription sets the neighbor description.
func (n *Neighbor) WithDescription(desc string) *Neighbor {
	n.oc.Description = ygot.String(desc)
	return n
}

// WithPassiveMode sets the transport passive-mode on neighbor.
func (n *Neighbor) WithPassiveMode(pm bool) *Neighbor {
	n.oc.GetOrCreateTransport().PassiveMode = ygot.Bool(pm)
	return n
}

// WithTCPMSS sets the transport tcp-mss on neighbor.
func (n *Neighbor) WithTCPMSS(mss uint16) *Neighbor {
	n.oc.GetOrCreateTransport().TcpMss = ygot.Uint16(mss)
	return n
}

// WithMTUDiscovery sets the transport mtu-discovery on neighbor.
func (n *Neighbor) WithMTUDiscovery(md bool) *Neighbor {
	n.oc.GetOrCreateTransport().MtuDiscovery = ygot.Bool(md)
	return n
}

// WithLocalAddress sets the transport local-address on neighbor.
func (n *Neighbor) WithLocalAddress(la string) *Neighbor {
	n.oc.GetOrCreateTransport().LocalAddress = ygot.String(la)
	return n
}

// WithLocalAS sets the local AS on the neighbor.
func (n *Neighbor) WithLocalAS(as uint32) *Neighbor {
	n.oc.LocalAs = ygot.Uint32(as)
	return n
}

// WithPeerAS sets the peer AS on the neighbor.
func (n *Neighbor) WithPeerAS(as uint32) *Neighbor {
	n.oc.PeerAs = ygot.Uint32(as)
	return n
}

// WithPeerType sets the peer type on the neighbor.
func (n *Neighbor) WithPeerType(pt fpoc.E_BgpTypes_PeerType) *Neighbor {
	n.oc.PeerType = pt
	return n
}

// WithRemovePrivateAS specifies that private AS should be removed.
func (n *Neighbor) WithRemovePrivateAS(val fpoc.E_BgpTypes_RemovePrivateAsOption) *Neighbor {
	n.oc.RemovePrivateAs = val
	return n
}

// WithSendCommunity sets the send-community on the neighbor.
func (n *Neighbor) WithSendCommunity(sc fpoc.E_BgpTypes_CommunityType) *Neighbor {
	n.oc.SendCommunity = sc
	return n
}

// PrefixLimitOptions struct to hold prefix limit options.
type PrefixLimitOptions struct {
	PreventTeardown     bool
	RestartTime         time.Duration
	WarningThresholdPct uint8
}

// WithV4PrefixLimit sets the IPv4 prefix limits on the neighbor.
func (n *Neighbor) WithV4PrefixLimit(maxPrefixes uint32, opts PrefixLimitOptions) *Neighbor {
	ploc := n.oc.GetOrCreateAfiSafi(fpoc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
	ploc.MaxPrefixes = ygot.Uint32(maxPrefixes)
	ploc.PreventTeardown = ygot.Bool(opts.PreventTeardown)
	if opts.RestartTime != 0 {
		n.oc.GetOrCreateTimers().RestartTime = ygot.Uint16(uint16(opts.RestartTime.Round(time.Second).Seconds()))
	}
	if opts.WarningThresholdPct > 0 {
		ploc.WarningThresholdPct = ygot.Uint8(opts.WarningThresholdPct)
	}
	return n
}

// WithKeepaliveInterval sets the keep-alive and hold timers on the neighbor.
func (n *Neighbor) WithKeepaliveInterval(keepalive, hold time.Duration) *Neighbor {
	toc := n.oc.GetOrCreateTimers()
	toc.HoldTime = ygot.Float64(hold.Seconds())
	toc.KeepaliveInterval = ygot.Float64(keepalive.Seconds())
	return n
}

// WithMRAI sets the minimum route advertisement interval timer on the neighbor.
func (n *Neighbor) WithMRAI(mrai time.Duration) *Neighbor {
	n.oc.GetOrCreateTimers().MinimumAdvertisementInterval = ygot.Float64(mrai.Seconds())
	return n
}

// WithConnectRetry sets the connect-retry timer on the neighbor.
func (n *Neighbor) WithConnectRetry(cr time.Duration) *Neighbor {
	n.oc.GetOrCreateTimers().ConnectRetry = ygot.Float64(cr.Seconds())
	return n
}

// AugmentGlobal implements the bgp.GlobalFeature interface.
// This method augments the BGP OC with neighbor configuration.
// Use bgp.WithFeature(n) instead of calling this method directly.
func (n *Neighbor) AugmentGlobal(bgp *fpoc.NetworkInstance_Protocol_Bgp) error {
	if err := n.oc.Validate(); err != nil {
		return err
	}
	bgpnoc := bgp.GetNeighbor(n.oc.GetNeighborAddress())
	if bgpnoc == nil {
		return bgp.AppendNeighbor(&n.oc)
	}
	return ygot.MergeStructInto(bgpnoc, &n.oc)
}

// NeighborFeature provides interface to augment the neighbor OC with
// additional features.
type NeighborFeature interface {
	// AugmentNeighbor augments the neighbor OC with additional feature.
	AugmentNeighbor(n *fpoc.NetworkInstance_Protocol_Bgp_Neighbor) error
}

// WithFeature augments the neighbor OC with additional feature.
func (n *Neighbor) WithFeature(f NeighborFeature) error {
	return f.AugmentNeighbor(&n.oc)
}
