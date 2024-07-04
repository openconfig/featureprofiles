// Copyright 2021 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package localbgp_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/confirm"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	dutAttrs = attrs.Attributes{
		Desc:    "To ATE",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	ateAttrs = attrs.Attributes{
		Desc:    "To DUT",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}
)

const (
	dutAS = 64500
	ateAS = 64501

	keepAlive   = 25
	holdTime    = keepAlive * 3 // Should be 3x keepAlive, see RFC 4271 - A Border Gateway Protocol 4, Sec. 10
	peerGrpName = "BGP-PEER-GROUP"
	policyName  = "ALLOW"
	dutRID      = "192.0.2.21"
	ateRID      = "192.0.2.31"
)

func bgpWithNbr(as uint32, routerID string, nbr *oc.NetworkInstance_Protocol_Bgp_Neighbor, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {

	d := &oc.Root{}
	ni1 := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()
	bgp.GetOrCreateGlobal().As = ygot.Uint32(as)
	bgp.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)

	if routerID != "" {
		bgp.Global.RouterId = ygot.String(routerID)
	}

	// Note: we have to define the peer group even if we aren't setting any policy because it's
	// invalid OC for the neighbor to be part of a peer group that doesn't exist.
	pg := bgp.GetOrCreatePeerGroup(peerGrpName)
	pg.PeerAs = ygot.Uint32(*nbr.PeerAs)
	pg.PeerGroupName = ygot.String(peerGrpName)
	if deviations.RoutePolicyUnderAFIUnsupported(dut) {
		rpl := pg.GetOrCreateApplyPolicy()
		rpl.ImportPolicy = []string{policyName}
		rpl.ExportPolicy = []string{policyName}
	} else {
		pgaf := pg.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		pgaf.Enabled = ygot.Bool(true)
		rpl := pgaf.GetOrCreateApplyPolicy()
		rpl.ImportPolicy = []string{policyName}
		rpl.ExportPolicy = []string{policyName}
	}

	bgp.AppendNeighbor(nbr)
	return niProto
}

func configureNIType(t *testing.T) {
	dut1 := ondatra.DUT(t, "dut1")
	dut2 := ondatra.DUT(t, "dut2")
	// Configure Network instance type on DUT
	fptest.ConfigureDefaultNetworkInstance(t, dut1)
	fptest.ConfigureDefaultNetworkInstance(t, dut2)
}

// configreRoutePolicy adds route-policy config
func configureRoutePolicy(t *testing.T, dut *ondatra.DUTDevice, name string, pr oc.E_RoutingPolicy_PolicyResultType) {
	d := &oc.Root{}
	rp := d.GetOrCreateRoutingPolicy()
	pd := rp.GetOrCreatePolicyDefinition(name)
	st, err := pd.AppendNewStatement("id-1")
	if err != nil {
		t.Fatal(err)
	}
	st.GetOrCreateActions().PolicyResult = pr
	gnmi.Replace(t, dut, gnmi.OC().RoutingPolicy().Config(), rp)
}

// bgpClearConfig removes all BGP configuration from the DUT.
func bgpClearConfig(t *testing.T, dut *ondatra.DUTDevice) {
	resetBatch := &gnmi.SetBatch{}
	gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config())

	if deviations.NetworkInstanceTableDeletionRequired(dut) {
		tablePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).TableAny()
		for _, table := range gnmi.LookupAll(t, dut, tablePath.Config()) {
			if val, ok := table.Val(); ok {
				if val.GetProtocol() == oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP {
					gnmi.BatchDelete(resetBatch, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Table(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, val.GetAddressFamily()).Config())
				}
			}
		}
	}
	resetBatch.Set(t, dut)
}

func TestEstablish(t *testing.T) {
	// Configure interfaces
	dut := ondatra.DUT(t, "dut1")
	dutPortName := dut.Port(t, "port1").Name()
	intf1 := dutAttrs.NewOCInterface(dutPortName, dut)
	gnmi.Replace(t, dut, gnmi.OC().Interface(intf1.GetName()).Config(), intf1)
	configureRoutePolicy(t, dut, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	ate := ondatra.DUT(t, "dut2")
	atePortName := ate.Port(t, "port1").Name()
	intf2 := ateAttrs.NewOCInterface(atePortName, dut)
	gnmi.Replace(t, ate, gnmi.OC().Interface(intf2.GetName()).Config(), intf2)
	configureRoutePolicy(t, ate, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	// Configure Network instance type, it has to be configured explicitly by user.
	configureNIType(t)

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dutPortName, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, ate, atePortName, deviations.DefaultNetworkInstance(dut), 0)
	}

	// Get BGP paths
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	ateConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateAttrs.IPv4)
	// Remove any existing BGP config
	bgpClearConfig(t, dut)
	bgpClearConfig(t, ate)

	// Start a new session
	dutConf := bgpWithNbr(dutAS, dutRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(dutAS),
		NeighborAddress: ygot.String(ateAttrs.IPv4),
		PeerGroup:       ygot.String(peerGrpName),
	}, dut)
	ateConf := bgpWithNbr(dutAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(dutAS),
		NeighborAddress: ygot.String(dutAttrs.IPv4),
		PeerGroup:       ygot.String(peerGrpName),
	}, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	gnmi.Replace(t, ate, ateConfPath.Config(), ateConf)
	gnmi.Await(t, dut, nbrPath.SessionState().State(), time.Second*180, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	wantState := dutConf.Bgp
	dutState := gnmi.Get(t, dut, statePath.State())
	if deviations.MissingValueForDefaults(dut) {
		wantState.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).AfiSafiName = 0
		wantState.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = nil
		wantState.GetOrCreateNeighbor(ateAttrs.IPv4).Enabled = nil
	}
	confirm.State(t, wantState, dutState)
	nbr := dutState.GetNeighbor(ateAttrs.IPv4)

	if !nbr.GetEnabled() {
		t.Errorf("Expected neighbor %v to be enabled", ateAttrs.IPv4)
	}

	lastFlapTime := gnmi.Get(t, dut, gnmi.OC().Interface(dutPortName).LastChange().State())
	lastEstTime := gnmi.Get(t, dut, nbrPath.State()).GetLastEstablished()
	if lastEstTime < lastFlapTime {
		t.Errorf("Bad last-established timestamp: got %v, want >= %v", lastEstTime, lastFlapTime)
	}

	if got := nbr.GetEstablishedTransitions(); got != 1 {
		t.Errorf("Wrong established-transitions: got %v, want 1", got)
	}

	capabilities := map[oc.E_BgpTypes_BGP_CAPABILITY]bool{
		oc.BgpTypes_BGP_CAPABILITY_ROUTE_REFRESH: false,
		oc.BgpTypes_BGP_CAPABILITY_ASN32:         false,
		oc.BgpTypes_BGP_CAPABILITY_MPBGP:         false,
	}
	for _, cap := range gnmi.Get(t, dut, nbrPath.SupportedCapabilities().State()) {
		capabilities[cap] = true
	}
	for cap, present := range capabilities {
		if !present {
			t.Errorf("Capability not reported: %v", cap)
		}
	}
}

func TestDisconnect(t *testing.T) {
	dut := ondatra.DUT(t, "dut1")
	ate := ondatra.DUT(t, "dut2")
	configureRoutePolicy(t, dut, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	configureRoutePolicy(t, ate, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	ateConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	ateIP := ateAttrs.IPv4
	dutIP := dutAttrs.IPv4
	nbrPath := statePath.Neighbor(ateIP)

	// Configure Network instance type, it has to be configured explicitly by user.
	configureNIType(t)

	// Clear any existing config
	bgpClearConfig(t, dut)
	bgpClearConfig(t, ate)

	// Apply simple config
	dutConf := bgpWithNbr(dutAS, dutRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(dutAS),
		NeighborAddress: ygot.String(ateIP),
		PeerGroup:       ygot.String(peerGrpName),
	}, dut)
	ateConf := bgpWithNbr(dutAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(dutAS),
		NeighborAddress: ygot.String(dutIP),
		PeerGroup:       ygot.String(peerGrpName),
	}, dut)

	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	gnmi.Replace(t, ate, ateConfPath.Config(), ateConf)
	gnmi.Await(t, dut, nbrPath.SessionState().State(), time.Second*120, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	gnmi.Replace(t, ate, ateConfPath.Bgp().Neighbor(dutIP).Enabled().Config(), false)
	gnmi.Await(t, dut, nbrPath.SessionState().State(), time.Second*120, oc.Bgp_Neighbor_SessionState_ACTIVE)
	code := gnmi.Lookup(t, dut, nbrPath.Messages().Received().LastNotificationErrorCode().State())
	if code.IsPresent() {
		value, _ := code.Val()
		if value != oc.BgpTypes_BGP_ERROR_CODE_CEASE {
			t.Errorf("On disconnect: expected error code %v, got %v", oc.BgpTypes_BGP_ERROR_CODE_CEASE, value)
		}
	} else {
		if deviations.MissingBgpLastNotificationErrorCode(dut) {
			t.Log("Last notification error code leaf not present. The validation result is ignored due to the deviation missingBgpLastNotificationErrorCode")
		} else {
			t.Error("Last notification error code leaf not present.")
		}
	}
}

func TestParameters(t *testing.T) {
	ateIP := ateAttrs.IPv4
	dutIP := dutAttrs.IPv4
	dut := ondatra.DUT(t, "dut1")
	ate := ondatra.DUT(t, "dut2")
	configureRoutePolicy(t, dut, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	configureRoutePolicy(t, ate, policyName, oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE)
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	ateConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateIP)

	// Configure Network instance type, it has to be configured explicitly by user.
	configureNIType(t)

	cases := []struct {
		name      string
		dutConf   *oc.NetworkInstance_Protocol
		ateConf   *oc.NetworkInstance_Protocol
		wantState *oc.NetworkInstance_Protocol
		skipMsg   string
	}{
		{
			name: "basic internal",
			dutConf: bgpWithNbr(dutAS, dutRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(ateIP),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
			ateConf: bgpWithNbr(dutAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
		},
		{
			name: "basic external",
			dutConf: bgpWithNbr(dutAS, dutRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
			ateConf: bgpWithNbr(ateAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
		},
		{
			name: "explicit AS",
			dutConf: bgpWithNbr(dutAS, dutRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				LocalAs:         ygot.Uint32(100),
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
			ateConf: bgpWithNbr(ateAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(100),
				NeighborAddress: ygot.String(dutIP),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
		},
		{
			name: "explicit router id",
			dutConf: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
			ateConf: bgpWithNbr(ateAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
		},
		{
			name: "password",
			dutConf: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				AuthPassword:    ygot.String("password"),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
			ateConf: bgpWithNbr(ateAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
				AuthPassword:    ygot.String("password"),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
			wantState: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
		},
		{
			name: "hold-time",
			dutConf: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				PeerGroup:       ygot.String(peerGrpName),
				Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
					HoldTime:          ygot.Uint16(holdTime),
					KeepaliveInterval: ygot.Uint16(keepAlive),
				},
			}, dut),
			ateConf: bgpWithNbr(ateAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
			wantState: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
					HoldTime:          ygot.Uint16(holdTime),
					KeepaliveInterval: ygot.Uint16(keepAlive),
				},
			}, dut),
		},
		{
			name: "connect-retry",
			dutConf: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				PeerGroup:       ygot.String(peerGrpName),
				Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
					ConnectRetry: ygot.Uint16(100),
				},
			}, dut),
			ateConf: bgpWithNbr(ateAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
				PeerGroup:       ygot.String(peerGrpName),
			}, dut),
			skipMsg: "Not currently supported by EOS (see b/186141921)",
		},
		{
			name: "hold time negotiated",
			dutConf: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				PeerGroup:       ygot.String(peerGrpName),
				NeighborAddress: ygot.String(ateIP),
				Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
					HoldTime:          ygot.Uint16(holdTime),
					KeepaliveInterval: ygot.Uint16(keepAlive),
				},
			}, dut),
			ateConf: bgpWithNbr(ateAS, ateRID, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				PeerGroup:       ygot.String(peerGrpName),
				NeighborAddress: ygot.String(dutIP),
				Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
					HoldTime:          ygot.Uint16(135),
					KeepaliveInterval: ygot.Uint16(45),
				},
			}, dut),
			wantState: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
					HoldTime:           ygot.Uint16(holdTime),
					NegotiatedHoldTime: ygot.Uint16(holdTime),
					KeepaliveInterval:  ygot.Uint16(keepAlive),
				},
			}, dut),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if deviations.ConnectRetry(dut) && tc.name == "connect-retry" || len(tc.skipMsg) > 0 {
				t.Skip(tc.skipMsg)
			}
			// Disable BGP
			bgpClearConfig(t, dut)
			bgpClearConfig(t, ate)
			// Renable and wait to establish
			gnmi.Replace(t, dut, dutConfPath.Config(), tc.dutConf)
			gnmi.Replace(t, ate, ateConfPath.Config(), tc.ateConf)
			gnmi.Await(t, dut, nbrPath.SessionState().State(), time.Second*120, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
			stateDut := gnmi.Get(t, dut, statePath.State())
			var wantState1 *oc.NetworkInstance_Protocol_Bgp
			if tc.wantState == nil {
				// if you don't specify a wantState, we just validate that the state corresponding to each config value is what it should be.
				wantState1 = tc.dutConf.Bgp
			} else {
				wantState1 = tc.wantState.Bgp
			}
			if deviations.MissingValueForDefaults(dut) {
				wantState1.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).AfiSafiName = 0
				wantState1.GetOrCreateGlobal().GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = nil
				wantState1.GetOrCreateNeighbor(ateAttrs.IPv4).Enabled = nil
			}
			confirm.State(t, wantState1, stateDut)
		})
	}
}
