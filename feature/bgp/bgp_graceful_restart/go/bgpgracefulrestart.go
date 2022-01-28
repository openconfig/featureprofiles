package bgpgracefulrestart

import (
	"errors"

	"github.com/openconfig/featureprofiles/yang/oc"
	"github.com/openconfig/ygot/ygot"
)

//
// To enable GracefulRestart on BGP peer, follow these steps:
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
// Step 4: Create BGP peer-grpup or neighbor
// pg := bgp.AddPeerGroup("GLOBAL-PEER")
// bgp.WithFeature(pg)
//
// (or)
//
// neighbor := bgp.AddNeighbor("1.2.3.4").WithPeerGroup("GLOBAL-PEER")
// bgp.WithFeature(neighbor)
//
// Step 5: Enable Graceful Restart on peer-group or neighbor.
// gr := bgpgracefulrestart.Enabled()
// neighbor.WithFeature(gr)
// pg.WithFeature(gr)
//

type GracefulRestart struct {
	enabled         bool
	restartTime     uint16
	staleRoutesTime float64
	helperOnly      bool
}

func Enabled() *GracefulRestart {
	return &GracefulRestart{enabled: true}
}

func (gr *GracefulRestart) WithRestartTime(val uint16) *GracefulRestart {
	if gr == nil {
		return nil
	}
	gr.restartTime = val
	return gr
}

func (gr *GracefulRestart) WithStaleRoutesTime(val float64) *GracefulRestart {
	if gr == nil {
		return nil
	}
	gr.staleRoutesTime = val
	return gr
}

func (gr *GracefulRestart) WithHelperOnly(val bool) *GracefulRestart {
	if gr == nil {
		return nil
	}
	gr.helperOnly = val
	return gr
}

func (gr *GracefulRestart) AugmentNeighbor(neighbor *oc.NetworkInstance_Protocol_Bgp_Neighbor) error {
	if gr == nil || neighbor == nil {
		return errors.New("graceful-restart or neighbor is nil")
	}
	if !gr.enabled {
		return nil
	}

	groc := neighbor.GetOrCreateGracefulRestart()
	if gr.restartTime != 0 {
		groc.RestartTime = ygot.Uint16(gr.restartTime)
	}
	if gr.staleRoutesTime != 0 {
		groc.StaleRoutesTime = ygot.Float64(gr.staleRoutesTime)
	}
	if gr.helperOnly {
		groc.HelperOnly = ygot.Bool(gr.helperOnly)
	}
	return nil
}

func (gr *GracefulRestart) AugmentPeerGroup(pg *oc.NetworkInstance_Protocol_Bgp_PeerGroup) error {
	if gr == nil || pg == nil {
		return errors.New("graceful-restart or peer-group is nil")
	}
	if !gr.enabled {
		return nil
	}

	groc := pg.GetOrCreateGracefulRestart()
	if gr.restartTime != 0 {
		groc.RestartTime = ygot.Uint16(gr.restartTime)
	}
	if gr.staleRoutesTime != 0 {
		groc.StaleRoutesTime = ygot.Float64(gr.staleRoutesTime)
	}
	if gr.helperOnly {
		groc.HelperOnly = ygot.Bool(gr.helperOnly)
	}
	return nil
}
