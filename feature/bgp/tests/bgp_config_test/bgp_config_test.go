// Copyright 2026 Google LLC
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

package bgp_config_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const neighbor = "192.0.2.99"

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestBGPGlobalLeaves rewrites the configured AS and router-id, then restores them.
func TestBGPGlobalLeaves(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ni := deviations.DefaultNetworkInstance(dut)
	global := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp().Global()
	as := gnmi.Get(t, dut, global.As().Config())
	routerID, hadRouterID := gnmi.LookupConfig(t, dut, global.RouterId().Config()).Val()
	t.Cleanup(func() {
		gnmi.Replace(t, dut, global.As().Config(), as)
		if hadRouterID {
			gnmi.Replace(t, dut, global.RouterId().Config(), routerID)
			return
		}
		gnmi.Delete(t, dut, global.RouterId().Config())
	})

	wantID := "192.0.2.1"
	if hadRouterID && routerID != wantID {
		wantID = routerID
	}
	t.Log("Rewrite the global AS and router-id")
	gnmi.Replace(t, dut, global.As().Config(), as)
	gnmi.Replace(t, dut, global.RouterId().Config(), wantID)
	if got := gnmi.Get(t, dut, global.As().Config()); got != as {
		t.Errorf("as config = %d, want %d", got, as)
	}
	if got := gnmi.Get(t, dut, global.RouterId().Config()); got != wantID {
		t.Errorf("router-id config = %q, want %q", got, wantID)
	}
	gnmi.Await(t, dut, global.As().State(), time.Minute, as)
}

// TestBGPNeighborAndIPv6 covers a neighbor create/delete and the IPv6 unicast AFI.
func TestBGPNeighborAndIPv6(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ni := deviations.DefaultNetworkInstance(dut)
	bgpPath := gnmi.OC().NetworkInstance(ni).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := bgpPath.Neighbor(neighbor)
	afiPath := bgpPath.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled()
	as := gnmi.Get(t, dut, bgpPath.Global().As().Config())
	afi, hadAFI := gnmi.LookupConfig(t, dut, afiPath.Config()).Val()
	t.Cleanup(func() {
		gnmi.Delete(t, dut, nbrPath.Config())
		if hadAFI {
			gnmi.Replace(t, dut, afiPath.Config(), afi)
			return
		}
		gnmi.Delete(t, dut, afiPath.Config())
	})

	t.Log("Create a BGP neighbor and enable IPv6 unicast")
	nbr := &oc.NetworkInstance_Protocol_Bgp_Neighbor{NeighborAddress: ygot.String(neighbor)}
	nbr.SetPeerAs(as)
	gnmi.Replace(t, dut, nbrPath.Config(), nbr)
	gnmi.Replace(t, dut, afiPath.Config(), true)

	if got := gnmi.Get(t, dut, nbrPath.PeerAs().Config()); got != as {
		t.Errorf("peer-as config = %d, want %d", got, as)
	}
	if got := gnmi.Get(t, dut, afiPath.Config()); !got {
		t.Error("IPV6_UNICAST enabled config = false, want true")
	}
	gnmi.Await(t, dut, nbrPath.NeighborAddress().State(), time.Minute, neighbor)

	t.Log("Delete the neighbor")
	gnmi.Delete(t, dut, nbrPath.Config())
	if gnmi.LookupConfig(t, dut, nbrPath.Config()).IsPresent() {
		t.Errorf("neighbor %s still in config", neighbor)
	}
}
