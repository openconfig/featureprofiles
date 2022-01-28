package bgp

import (
	"errors"
	"fmt"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

//
// To configure BGP neighbor, follow these steps:
//
// Step 1: Create device.
// d := device.New()
//
// Step 2: Create default NI on device.
// ni := networkinstance.Enabled("default", oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
// d.WithFeature(ni)
//
// Step 3: Enable BGP on default NI.
// bgp := bgp.AddBGP()
// ni.WithFeature(bgp)
//
// Step 4: Create BGP  neighbor
// neighbor := bgp.AddNeighbor("1.2.3.4").WithPeerGroup("GLOBAL-PEER")
// bgp.WithFeature(neighbor)
//

type PrefixLimit struct {
	MaxPrefixes         uint32
	PreventTeardown     bool
	RestartTimer        float64
	WarningThresholdPct uint8
}

type Timers struct {
	MinAdvertisementIntvl float64
	HoldTime              float64
	KeepaliveIntvl        float64
	ConnectRetry          float64
}

type Transport struct {
	passiveMode  bool
	tcpMSS       uint16
	mtuDiscovery bool
	localAddr    string
}

type Neighbor struct {
	enabled         bool
	address         string
	afiSafiEnabled  bool
	afiSafiName     oc.E_BgpTypes_AFI_SAFI_TYPE
	peerGroup       string
	logStateChanges bool
	authPassword    string
	description     string
	transport       *Transport
	localAS         uint32
	peerAS          uint32
	peerType        oc.E_BgpTypes_PeerType
	removePrivateAS oc.E_BgpTypes_RemovePrivateAsOption
	sendCommunity   oc.E_BgpTypes_CommunityType
	v4PrefixLimit   *PrefixLimit
	timers          *Timers
	oc              *oc.NetworkInstance_Protocol_Bgp_Neighbor
}

func AddNeighbor(addr string) *Neighbor {
	return &Neighbor{
		enabled: true,
		address: addr,
	}
}

func (n *Neighbor) WithAFISAFI(t oc.E_BgpTypes_AFI_SAFI_TYPE) *Neighbor {
	if n == nil {
		return nil
	}
	n.afiSafiEnabled = true
	n.afiSafiName = t
	return n
}

func (n *Neighbor) WithPeerGroup(pg string) *Neighbor {
	if n == nil {
		return nil
	}
	n.peerGroup = pg
	return n
}

func (n *Neighbor) WithLogStateChanges(lsc bool) *Neighbor {
	if n == nil {
		return nil
	}
	n.logStateChanges = lsc
	return n
}

func (n *Neighbor) WithAuthPassword(pwd string) *Neighbor {
	if n == nil {
		return nil
	}
	n.authPassword = pwd
	return n
}

func (n *Neighbor) WithDescription(desc string) *Neighbor {
	if n == nil {
		return nil
	}
	n.description = desc
	return n
}

func (n *Neighbor) WithTransport(t *Transport) *Neighbor {
	if n == nil || t == nil {
		return nil
	}
	n.transport = t
	return n
}

func (n *Neighbor) WithLocalAS(as uint32) *Neighbor {
	if n == nil {
		return nil
	}
	n.localAS = as
	return n
}

func (n *Neighbor) WithPeerAS(as uint32) *Neighbor {
	if n == nil {
		return nil
	}
	n.peerAS = as
	return n
}

func (n *Neighbor) WithPeerType(pt oc.E_BgpTypes_PeerType) *Neighbor {
	if n == nil {
		return nil
	}
	n.peerType = pt
	return n
}

func (n *Neighbor) WithRemovePrivateAS(rmv oc.E_BgpTypes_RemovePrivateAsOption) *Neighbor {
	if n == nil {
		return nil
	}
	n.removePrivateAS = rmv
	return n
}

func (n *Neighbor) WithSendCommunity(sc oc.E_BgpTypes_CommunityType) *Neighbor {
	if n == nil {
		return nil
	}
	n.sendCommunity = sc
	return n
}

func (n *Neighbor) WithPrefixLimit(p *PrefixLimit) *Neighbor {
	if n == nil || p == nil {
		return nil
	}
	n.v4PrefixLimit = p
	return n
}

func (n *Neighbor) WithTimers(t *Timers) *Neighbor {
	if n == nil || t == nil {
		return nil
	}
	n.timers = t
	return n
}

func (n *Neighbor) validate() error {
	switch n.afiSafiName {
	case oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST:
	case oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST:
		if n.v4PrefixLimit != nil {
			return fmt.Errorf("ipv4 prefix-limit not allowed for ipv6 peer %s", n.address)
		}
	default:
		return fmt.Errorf("unsupported afi-safi %v for neighbor %s", n.afiSafiName, n.address)
	}
	return nil
}

func (n *Neighbor) AugmentBGP(bgp *oc.NetworkInstance_Protocol_Bgp) error {
	if n == nil || bgp == nil {
		return errors.New("neighbor or bgp is nil")
	}

	if err := n.validate(); err != nil {
		return err
	}

	if !n.enabled {
		return nil
	}

	n.oc = bgp.GetOrCreateNeighbor(n.address)
	if n.peerGroup != "" {
		n.oc.PeerGroup = ygot.String(n.peerGroup)
	}
	if n.logStateChanges {
		n.oc.GetOrCreateLoggingOptions().LogNeighborStateChanges = ygot.Bool(true)
	}
	if n.authPassword != "" {
		n.oc.AuthPassword = ygot.String(n.authPassword)
	}
	if n.description != "" {
		n.oc.Description = ygot.String(n.description)
	}
	if n.transport != nil {
		toc := n.oc.GetOrCreateTransport()
		if n.transport.passiveMode {
			toc.PassiveMode = ygot.Bool(true)
		}
		if n.transport.tcpMSS != 0 {
			toc.TcpMss = ygot.Uint16(n.transport.tcpMSS)
		}
		if n.transport.mtuDiscovery {
			toc.MtuDiscovery = ygot.Bool(true)
		}
		if n.transport.localAddr != "" {
			toc.LocalAddress = ygot.String(n.transport.localAddr)
		}
	}
	if n.localAS != 0 {
		n.oc.LocalAs = ygot.Uint32(n.localAS)
	}
	if n.peerAS != 0 {
		n.oc.PeerAs = ygot.Uint32(n.peerAS)
	}
	if n.peerType != oc.BgpTypes_PeerType_UNSET {
		n.oc.PeerType = n.peerType
	}
	if n.removePrivateAS != oc.BgpTypes_RemovePrivateAsOption_UNSET {
		n.oc.RemovePrivateAs = n.removePrivateAS
	}
	if n.sendCommunity != oc.BgpTypes_CommunityType_NONE {
		n.oc.SendCommunity = n.sendCommunity
	}
	if n.afiSafiEnabled {
		af := n.oc.GetOrCreateAfiSafi(n.afiSafiName)
		af.Enabled = ygot.Bool(true)

		pl := n.v4PrefixLimit
		if pl != nil {
			ploc := af.GetOrCreateIpv4Unicast().GetOrCreatePrefixLimit()
			if pl.MaxPrefixes != 0 {
				ploc.MaxPrefixes = ygot.Uint32(pl.MaxPrefixes)
			}
			if pl.PreventTeardown {
				ploc.PreventTeardown = ygot.Bool(pl.PreventTeardown)
			}
			if pl.RestartTimer != 0 {
				ploc.RestartTimer = ygot.Float64(pl.RestartTimer)
			}
			if pl.WarningThresholdPct != 0 {
				ploc.WarningThresholdPct = ygot.Uint8(pl.WarningThresholdPct)
			}
		}
	}
	if n.timers != nil {
		toc := n.oc.GetOrCreateTimers()
		if n.timers.MinAdvertisementIntvl != 0 {
			toc.MinimumAdvertisementInterval = ygot.Float64(n.timers.MinAdvertisementIntvl)
		}
		if n.timers.HoldTime != 0 {
			toc.HoldTime = ygot.Float64(n.timers.HoldTime)
		}
		if n.timers.KeepaliveIntvl != 0 {
			toc.KeepaliveInterval = ygot.Float64(n.timers.KeepaliveIntvl)
		}
		if n.timers.ConnectRetry != 0 {
			toc.ConnectRetry = ygot.Float64(n.timers.ConnectRetry)
		}
	}
	return nil
}

type NeighborFeature interface {
	AugmentNeighbor(oc *oc.NetworkInstance_Protocol_Bgp_Neighbor) error
}

func (n *Neighbor) WithFeature(f NeighborFeature) error {
	if n == nil || f == nil {
		return nil
	}
	return f.AugmentNeighbor(n.oc)
}
