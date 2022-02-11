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

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

// PeerGroup struct to store OC attributes.
type PeerGroup struct {
	oc oc.NetworkInstance_Protocol_Bgp_PeerGroup
}

// NewPeerGroup returns a new peer-group object.
func NewPeerGroup(name string) *PeerGroup {
	return &PeerGroup{
		oc: oc.NetworkInstance_Protocol_Bgp_PeerGroup{
			PeerGroupName: ygot.String(name),
		},
	}
}

// Name returns the name of the peer-group.
func (pg *PeerGroup) Name() string {
	return pg.oc.GetPeerGroupName()
}

// WithAFISAFI adds specified AFI-SAFI type to peer-group.
func (pg *PeerGroup) WithAFISAFI(name oc.E_BgpTypes_AFI_SAFI_TYPE) *PeerGroup {
	pg.oc.GetOrCreateAfiSafi(name).Enabled = ygot.Bool(true)
	return pg
}

// WithAuthPassword sets auth password on peer-group.
func (pg *PeerGroup) WithAuthPassword(pwd string) *PeerGroup {
	pg.oc.AuthPassword = ygot.String(pwd)
	return pg
}

// WithDescription sets the peer-group descriptiopg.
func (pg *PeerGroup) WithDescription(desc string) *PeerGroup {
	pg.oc.Description = ygot.String(desc)
	return pg
}

// WithTransportPassiveMode sets the transport passive-mode on neighbor.
func (pg *PeerGroup) WithTransportPassiveMode(pm bool) *PeerGroup {
	pg.oc.GetOrCreateTransport().PassiveMode = ygot.Bool(pm)
	return pg
}

// WithTransportTCPMSS sets the transport tcp-mss on neighbor.
func (pg *PeerGroup) WithTransportTCPMSS(mss uint16) *PeerGroup {
	pg.oc.GetOrCreateTransport().TcpMss = ygot.Uint16(mss)
	return pg
}

// WithTransportMTUDiscovery sets the transport mtu-discovery on neighbor.
func (pg *PeerGroup) WithTransportMTUDiscovery(md bool) *PeerGroup {
	pg.oc.GetOrCreateTransport().MtuDiscovery = ygot.Bool(md)
	return pg
}

// WithTransportLocalAddress sets the transport local-address on neighbor.
func (pg *PeerGroup) WithTransportLocalAddress(la string) *PeerGroup {
	pg.oc.GetOrCreateTransport().LocalAddress = ygot.String(la)
	return pg
}

// WithLocalAS sets the local AS on the peer-group.
func (pg *PeerGroup) WithLocalAS(as uint32) *PeerGroup {
	pg.oc.LocalAs = ygot.Uint32(as)
	return pg
}

// WithPeerAS sets the peer AS on the peer-group.
func (pg *PeerGroup) WithPeerAS(as uint32) *PeerGroup {
	pg.oc.PeerAs = ygot.Uint32(as)
	return pg
}

// WithPeerType sets the peer type on the peer-group.
func (pg *PeerGroup) WithPeerType(pt oc.E_BgpTypes_PeerType) *PeerGroup {
	pg.oc.PeerType = pt
	return pg
}

// WithRemovePrivateAS specifies that private AS should be removed.
func (pg *PeerGroup) WithRemovePrivateAS(val oc.E_BgpTypes_RemovePrivateAsOption) *PeerGroup {
	pg.oc.RemovePrivateAs = val
	return pg
}

// WithSendCommunity sets the send-community on the peer-group.
func (pg *PeerGroup) WithSendCommunity(sc oc.E_BgpTypes_CommunityType) *PeerGroup {
	pg.oc.SendCommunity = sc
	return pg
}

// WithV4PrefixLimit sets the IPv4 prefix limits on the neighbor.
func (pg *PeerGroup) WithV4PrefixLimit(maxPrefixes uint32, opts PrefixLimitOptions) *PeerGroup {
	ploc := pg.oc.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
	ploc.MaxPrefixes = ygot.Uint32(maxPrefixes)
	ploc.PreventTeardown = ygot.Bool(opts.PreventTeardown)
	if opts.RestartTimer != 0 {
		ploc.RestartTimer = ygot.Float64(opts.RestartTimer.Seconds())
	}
	if opts.WarningThresholdPct > 0 {
		ploc.WarningThresholdPct = ygot.Uint8(opts.WarningThresholdPct)
	}
	return pg
}

// WithKeepAliveInterval sets the keep-alive and hold timers on the neighbor.
func (pg *PeerGroup) WithKeepAliveInterval(keepalive, hold time.Duration) *PeerGroup {
	toc := pg.oc.GetOrCreateTimers()
	toc.HoldTime = ygot.Float64(hold.Seconds())
	toc.KeepaliveInterval = ygot.Float64(keepalive.Seconds())
	return pg
}

// WithMinimumAdvertisementInterval sets the minimum advertisement interval timer on the neighbor.
func (pg *PeerGroup) WithMinimumAdvertisementInterval(mdi time.Duration) *PeerGroup {
	pg.oc.GetOrCreateTimers().MinimumAdvertisementInterval = ygot.Float64(mdi.Seconds())
	return pg
}

// WithConnectRetry sets the connect-retry timer on the neighbor.
func (pg *PeerGroup) WithConnectRetry(cr time.Duration) *PeerGroup {
	pg.oc.GetOrCreateTimers().ConnectRetry = ygot.Float64(cr.Seconds())
	return pg
}

// AugmentGlobal implements the bgp.GlobalFeature interface.
// This method augments the BGP with peer-group configuration.
// Use bgp.WithFeature(pg) instead of calling this method directly.
func (pg *PeerGroup) AugmentGlobal(bgp *oc.NetworkInstance_Protocol_Bgp) error {
	return bgp.AppendPeerGroup(&pg.oc)
}

// PeerGroupFeature provides interface to augment peer-group with
// additional features.
type PeerGroupFeature interface {
	// AugmentPeerGroup augments peer-group with additional feature.
	AugmentPeerGroup(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup) error
}

// WithFeature augments peer-group with provided feature.
func (pg *PeerGroup) WithFeature(f PeerGroupFeature) error {
	return f.AugmentPeerGroup(&pg.oc)
}
