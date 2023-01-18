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

	keepAlive = 50
	holdTime  = keepAlive * 3 // Should be 3x keepAlive, see RFC 4271 - A Border Gateway Protocol 4, Sec. 10
)

func bgpWithNbr(as uint32, routerID string, nbr *oc.NetworkInstance_Protocol_Bgp_Neighbor) *oc.NetworkInstance_Protocol_Bgp {
	bgp := &oc.NetworkInstance_Protocol_Bgp{}
	bgp.GetOrCreateGlobal().As = ygot.Uint32(as)
	if routerID != "" {
		bgp.Global.RouterId = ygot.String(routerID)
	}
	bgp.AppendNeighbor(nbr)
	return bgp
}

func TestEstablish(t *testing.T) {
	// Configure interfaces
	dut := ondatra.DUT(t, "dut1")
	dutPortName := dut.Port(t, "port1").Name()
	intf1 := dutAttrs.NewOCInterface(dutPortName)
	gnmi.Replace(t, dut, gnmi.OC().Interface(intf1.GetName()).Config(), intf1)
	ate := ondatra.DUT(t, "dut2")
	atePortName := ate.Port(t, "port1").Name()
	intf2 := ateAttrs.NewOCInterface(atePortName)
	gnmi.Replace(t, ate, gnmi.OC().Interface(intf2.GetName()).Config(), intf2)

	if *deviations.ExplicitPortSpeed {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if *deviations.ExplicitInterfaceInDefaultVRF {
		fptest.AssignToNetworkInstance(t, dut, dutPortName, *deviations.DefaultNetworkInstance, 0)
		fptest.AssignToNetworkInstance(t, ate, atePortName, *deviations.DefaultNetworkInstance, 0)
	}

	// Get BGP paths
	dutConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	ateConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateAttrs.IPv4)
	// Remove any existing BGP config
	gnmi.Delete(t, dut, dutConfPath.Config())
	gnmi.Delete(t, ate, ateConfPath.Config())
	startTime := uint64(time.Now().Unix())
	// Start a new session
	dutConf := bgpWithNbr(dutAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(dutAS),
		NeighborAddress: ygot.String(ateAttrs.IPv4),
	})
	ateConf := bgpWithNbr(dutAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(dutAS),
		NeighborAddress: ygot.String(dutAttrs.IPv4),
	})
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	gnmi.Replace(t, ate, ateConfPath.Config(), ateConf)
	gnmi.Await(t, dut, nbrPath.SessionState().State(), time.Second*15, oc.Bgp_Neighbor_SessionState_ESTABLISHED)

	dutState := gnmi.Get(t, dut, statePath.State())
	confirm.State(t, dutConf, dutState)
	nbr := dutState.GetNeighbor(ateAttrs.IPv4)

	if !nbr.GetEnabled() {
		t.Errorf("Expected neighbor %v to be enabled", ateAttrs.IPv4)
	}
	if got, want := nbr.GetLastEstablished(), startTime*1000000000; got < want {
		t.Errorf("Bad last-established timestamp: got %v, want >= %v", got, want)
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
	dutConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	ateConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	ateIP := ateAttrs.IPv4
	dutIP := dutAttrs.IPv4
	nbrPath := statePath.Neighbor(ateIP)

	// Clear any existing config
	gnmi.Delete(t, dut, dutConfPath.Config())
	gnmi.Delete(t, ate, ateConfPath.Config())

	// Apply simple config
	dutConf := bgpWithNbr(dutAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(dutAS),
		NeighborAddress: ygot.String(ateIP),
	})
	ateConf := bgpWithNbr(dutAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
		PeerAs:          ygot.Uint32(dutAS),
		NeighborAddress: ygot.String(dutIP),
	})
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	gnmi.Replace(t, ate, ateConfPath.Config(), ateConf)
	gnmi.Await(t, dut, nbrPath.SessionState().State(), time.Second*15, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
	// ateConfPath.Delete(t)
	gnmi.Replace(t, ate, ateConfPath.Neighbor(dutIP).Enabled().Config(), false)
	gnmi.Await(t, dut, nbrPath.SessionState().State(), time.Second*15, oc.Bgp_Neighbor_SessionState_ACTIVE)
	code := gnmi.Get(t, dut, nbrPath.Messages().Received().LastNotificationErrorCode().State())
	if code != oc.BgpTypes_BGP_ERROR_CODE_CEASE {
		t.Errorf("On disconnect: expected error code %v, got %v", oc.BgpTypes_BGP_ERROR_CODE_CEASE, code)
	}
}

func TestParameters(t *testing.T) {
	ateIP := ateAttrs.IPv4
	dutIP := dutAttrs.IPv4
	dut := ondatra.DUT(t, "dut1")
	ate := ondatra.DUT(t, "dut2")
	dutConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	ateConfPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	statePath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	nbrPath := statePath.Neighbor(ateIP)

	cases := []struct {
		name      string
		dutConf   *oc.NetworkInstance_Protocol_Bgp
		ateConf   *oc.NetworkInstance_Protocol_Bgp
		wantState *oc.NetworkInstance_Protocol_Bgp
		skipMsg   string
	}{
		{
			name: "basic internal",
			dutConf: bgpWithNbr(dutAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(ateIP),
			}),
			ateConf: bgpWithNbr(dutAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
			}),
		},
		{
			name: "basic external",
			dutConf: bgpWithNbr(dutAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
			}),
			ateConf: bgpWithNbr(ateAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
			}),
		},
		{
			name: "explicit AS",
			dutConf: bgpWithNbr(dutAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				LocalAs:         ygot.Uint32(100),
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
			}),
			ateConf: bgpWithNbr(ateAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(100),
				NeighborAddress: ygot.String(dutIP),
			}),
		},
		{
			name: "explicit router id",
			dutConf: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
			}),
			ateConf: bgpWithNbr(ateAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
			}),
		},
		{
			name: "password",
			dutConf: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				AuthPassword:    ygot.String("password"),
			}),
			ateConf: bgpWithNbr(ateAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
				AuthPassword:    ygot.String("password"),
			}),
		},
		{
			name: "hold-time",
			dutConf: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
					HoldTime:          ygot.Uint16(holdTime),
					KeepaliveInterval: ygot.Uint16(keepAlive),
				},
			}),
			ateConf: bgpWithNbr(ateAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
			}),
		},
		{
			name: "connect-retry",
			dutConf: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
					ConnectRetry: ygot.Uint16(100),
				},
			}),
			ateConf: bgpWithNbr(ateAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
			}),
			skipMsg: "Not currently supported by EOS (see b/186141921)",
		},
		{
			name: "hold time negotiated",
			dutConf: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
					HoldTime:          ygot.Uint16(holdTime),
					KeepaliveInterval: ygot.Uint16(keepAlive),
				},
			}),
			ateConf: bgpWithNbr(ateAS, "", &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(dutAS),
				NeighborAddress: ygot.String(dutIP),
				Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
					HoldTime:          ygot.Uint16(135),
					KeepaliveInterval: ygot.Uint16(45),
				},
			}),
			wantState: bgpWithNbr(dutAS, dutIP, &oc.NetworkInstance_Protocol_Bgp_Neighbor{
				PeerAs:          ygot.Uint32(ateAS),
				NeighborAddress: ygot.String(ateIP),
				Timers: &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
					HoldTime:           ygot.Uint16(holdTime),
					NegotiatedHoldTime: ygot.Uint16(135),
					KeepaliveInterval:  ygot.Uint16(keepAlive),
				},
			}),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.skipMsg) > 0 {
				t.Skip(tc.skipMsg)
			}
			// Disable BGP
			gnmi.Delete(t, dut, dutConfPath.Config())
			gnmi.Delete(t, ate, ateConfPath.Config())
			// Renable and wait to establish
			gnmi.Replace(t, dut, dutConfPath.Config(), tc.dutConf)
			gnmi.Replace(t, ate, ateConfPath.Config(), tc.ateConf)
			gnmi.Await(t, dut, nbrPath.SessionState().State(), time.Second*30, oc.Bgp_Neighbor_SessionState_ESTABLISHED)
			stateDut := gnmi.Get(t, dut, statePath.State())
			wantState := tc.wantState
			if wantState == nil {
				// if you don't specify a wantState, we just validate that the state corresponding to each config value is what it should be.
				wantState = tc.dutConf
			}
			confirm.State(t, wantState, stateDut)
		})
	}
}
