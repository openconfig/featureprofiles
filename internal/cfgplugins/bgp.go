// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cfgplugins

import (
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	// DUTAS dut AS number
	DUTAS = 65656

	// RPLPermitAll policy
	RPLPermitAll = "PERMIT-ALL"
)

// BuildBGPOCConfig builds the BGP OC config applying global, neighbors and peer-group config
func BuildBGPOCConfig(t *testing.T, dut *ondatra.DUTDevice, routerID string, aftType oc.E_BgpTypes_AFI_SAFI_TYPE, neighborAS map[string]uint32, neighborPG map[string]string) *oc.NetworkInstance_Protocol_Bgp {
	afiSafiGlobal := map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Global_AfiSafi{
		aftType: {
			AfiSafiName: aftType,
			Enabled:     ygot.Bool(true),
		},
	}
	afiSafiNeighbor := map[oc.E_BgpTypes_AFI_SAFI_TYPE]*oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi{
		aftType: {
			AfiSafiName: aftType,
			Enabled:     ygot.Bool(true),
		},
	}

	global := &oc.NetworkInstance_Protocol_Bgp_Global{
		As:       ygot.Uint32(uint32(DUTAS)),
		RouterId: ygot.String(routerID),
		AfiSafi:  afiSafiGlobal,
	}

	neighbors := make(map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor)
	for neighbor, as := range neighborAS {
		pg, ok := neighborPG[neighbor]
		if !ok {
			t.Fatalf("Neighbor peer-group %s not found in neighborPeerGroup", neighbor)
		}
		neighbors[neighbor] = &oc.NetworkInstance_Protocol_Bgp_Neighbor{
			PeerAs:          ygot.Uint32(as),
			PeerGroup:       ygot.String(pg),
			NeighborAddress: ygot.String(neighbor),
			AfiSafi:         afiSafiNeighbor,
		}
	}

	peerGroups := make(map[string]*oc.NetworkInstance_Protocol_Bgp_PeerGroup)
	for _, pg := range neighborPG {
		peerGroups[pg] = getPeerGroup(pg, dut, aftType)
	}

	return &oc.NetworkInstance_Protocol_Bgp{
		Global:    global,
		Neighbor:  neighbors,
		PeerGroup: peerGroups,
	}
}

// getPeerGroup build peer-config
func getPeerGroup(pgn string, dut *ondatra.DUTDevice, aftype oc.E_BgpTypes_AFI_SAFI_TYPE) *oc.NetworkInstance_Protocol_Bgp_PeerGroup {
	bgp := &oc.NetworkInstance_Protocol_Bgp{}
	pg := bgp.GetOrCreatePeerGroup(pgn)

	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		// policy under peer group
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.SetExportPolicy([]string{RPLPermitAll})
		rpl.SetImportPolicy([]string{RPLPermitAll})
		return pg
	}

	// policy under peer group AFI
	afisafi := pg.GetOrCreateAfiSafi(aftype)
	afisafi.Enabled = ygot.Bool(true)
	rpl := afisafi.GetOrCreateApplyPolicy()
	rpl.SetExportPolicy([]string{RPLPermitAll})
	rpl.SetImportPolicy([]string{RPLPermitAll})
	return pg
}
