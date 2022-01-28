package bgp

import (
	"errors"
	"fmt"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

//
// To configure BGP peer group, follow these steps:
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
// Step 4: Create BGP peer-grpup
// pg := bgp.AddPeerGroup("GLOBAL-PEER")
// bgp.WithFeature(pg)
//

type PeerGroup struct {
	name            string
	afiSafiEnabled  bool
	afiSafiName     oc.E_BgpTypes_AFI_SAFI_TYPE
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
	oc              *oc.NetworkInstance_Protocol_Bgp_PeerGroup
}

func AddPeerGroup(name string) *PeerGroup {
	return &PeerGroup{name: name}
}

func (pg *PeerGroup) WithAFISAFI(t oc.E_BgpTypes_AFI_SAFI_TYPE) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.afiSafiEnabled = true
	pg.afiSafiName = t
	return pg
}

func (pg *PeerGroup) WithAuthPassword(pwd string) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.authPassword = pwd
	return pg
}

func (pg *PeerGroup) WithDescription(desc string) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.description = desc
	return pg
}

func (pg *PeerGroup) WithTransport(t *Transport) *PeerGroup {
	if pg == nil || t == nil {
		return nil
	}
	pg.transport = t
	return pg
}

func (pg *PeerGroup) WithLocalAS(as uint32) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.localAS = as
	return pg
}

func (pg *PeerGroup) WithPeerAS(as uint32) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.peerAS = as
	return pg
}

func (pg *PeerGroup) WithPeerType(pt oc.E_BgpTypes_PeerType) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.peerType = pt
	return pg
}

func (pg *PeerGroup) WithRemovePrivateAS(rmv oc.E_BgpTypes_RemovePrivateAsOption) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.removePrivateAS = rmv
	return pg
}

func (pg *PeerGroup) WithSendCommunity(sc oc.E_BgpTypes_CommunityType) *PeerGroup {
	if pg == nil {
		return nil
	}
	pg.sendCommunity = sc
	return pg
}

func (pg *PeerGroup) WithPrefixLimit(p *PrefixLimit) *PeerGroup {
	if pg == nil || p == nil {
		return nil
	}
	pg.v4PrefixLimit = p
	return pg
}

func (pg *PeerGroup) WithTimers(t *Timers) *PeerGroup {
	if pg == nil || t == nil {
		return nil
	}
	pg.timers = t
	return pg
}

func (pg *PeerGroup) validate() error {
	switch pg.afiSafiName {
	case oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST:
	case oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST:
		if pg.v4PrefixLimit != nil {
			return fmt.Errorf("ipv4 prefix-limit not allowed for ipv6 peer-group %s", pg.name)
		}
	default:
		return fmt.Errorf("unsupported afi-safi %v for peer-group %s", pg.afiSafiName, pg.name)
	}
	return nil
}

func (pg *PeerGroup) AugmentBGP(bgp *oc.NetworkInstance_Protocol_Bgp) error {
	if pg == nil || bgp == nil {
		return errors.New("peer-group or bgp is nil")
	}
	if err := pg.validate(); err != nil {
		return err
	}

	if pg.name == "" {
		return nil
	}

	pg.oc = bgp.GetOrCreatePeerGroup(pg.name)
	if pg.authPassword != "" {
		pg.oc.AuthPassword = ygot.String(pg.authPassword)
	}
	if pg.description != "" {
		pg.oc.Description = ygot.String(pg.description)
	}
	if pg.transport != nil {
		toc := pg.oc.GetOrCreateTransport()
		if pg.transport.passiveMode {
			toc.PassiveMode = ygot.Bool(true)
		}
		if pg.transport.tcpMSS != 0 {
			toc.TcpMss = ygot.Uint16(pg.transport.tcpMSS)
		}
		if pg.transport.mtuDiscovery {
			toc.MtuDiscovery = ygot.Bool(true)
		}
		if pg.transport.localAddr != "" {
			toc.LocalAddress = ygot.String(pg.transport.localAddr)
		}
	}
	if pg.localAS != 0 {
		pg.oc.LocalAs = ygot.Uint32(pg.localAS)
	}
	if pg.peerAS != 0 {
		pg.oc.PeerAs = ygot.Uint32(pg.peerAS)
	}
	if pg.peerType != oc.BgpTypes_PeerType_UNSET {
		pg.oc.PeerType = pg.peerType
	}
	if pg.removePrivateAS != oc.BgpTypes_RemovePrivateAsOption_UNSET {
		pg.oc.RemovePrivateAs = pg.removePrivateAS
	}
	if pg.sendCommunity != oc.BgpTypes_CommunityType_NONE {
		pg.oc.SendCommunity = pg.sendCommunity
	}
	if pg.afiSafiEnabled {
		af := pg.oc.GetOrCreateAfiSafi(pg.afiSafiName)
		af.Enabled = ygot.Bool(true)

		pl := pg.v4PrefixLimit
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
	if pg.timers != nil {
		toc := pg.oc.GetOrCreateTimers()
		if pg.timers.MinAdvertisementIntvl != 0 {
			toc.MinimumAdvertisementInterval = ygot.Float64(pg.timers.MinAdvertisementIntvl)
		}
		if pg.timers.HoldTime != 0 {
			toc.HoldTime = ygot.Float64(pg.timers.HoldTime)
		}
		if pg.timers.KeepaliveIntvl != 0 {
			toc.KeepaliveInterval = ygot.Float64(pg.timers.KeepaliveIntvl)
		}
		if pg.timers.ConnectRetry != 0 {
			toc.ConnectRetry = ygot.Float64(pg.timers.ConnectRetry)
		}
	}
	return nil
}

type PeerGroupFeature interface {
	AugmentPeerGroup(oc *oc.NetworkInstance_Protocol_Bgp_PeerGroup) error
}

func (pg *PeerGroup) WithFeature(f PeerGroupFeature) error {
	if pg == nil || f == nil {
		return nil
	}
	return f.AugmentPeerGroup(pg.oc)
}
