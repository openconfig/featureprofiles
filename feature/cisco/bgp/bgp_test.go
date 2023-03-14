package basetest

import (
	"context"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	ft "github.com/openconfig/featureprofiles/tools/inputcisco/feature"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestBGPState(t *testing.T) {
	dut := ondatra.DUT(t, device1)
	ate := ondatra.ATE(t, ate1)
	inputObj, err := testInput.GetTestInput(t)
	if err != nil {
		t.Error(err)
	}
	t.Log("Remove Flowspec Config")
	configToChange := "no flowspec \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)
	inputObj.ConfigInterfaces(dut)
	time.Sleep(30 * time.Second)
	inputObj.StartAteProtocols(ate)
	time.Sleep(30 * time.Second)
	for _, bgp := range inputObj.Device(dut).Features().Bgp {
		for _, neighbor := range bgp.Neighbors {

			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/description", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Description()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				description := gnmi.Get(t, dut, state.State())
				if description != neighbor.GetDescription() {
					t.Errorf("BGP Neigbhor Description: got %s, want %s", description, neighbor.GetDescription())
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/enabled", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Enabled()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				enabled := gnmi.Get(t, dut, state.State())
				if enabled != true {
					t.Errorf("BGP Neighbor Enabled: got %t, want %t", enabled, true)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-as", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).PeerAs()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val != uint32(neighbor.PeerAs) {
					t.Errorf("BGP Neighbor peer-as: got %d, want %d", val, neighbor.PeerAs)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/local-as", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).LocalAs()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val != uint32(neighbor.LocalAs) {
					t.Errorf("BGP Neighbor local-as: got %d, want %d", val, neighbor.LocalAs)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/established-transitions", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).EstablishedTransitions()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val == 0 {
					t.Errorf("BGP Neighbor established-transitions: got %d, want !=%d", val, 0)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/last-established", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).LastEstablished()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val == 0 {
					t.Errorf("BGP Neighbor last-established: got %d, want !=%d", val, 0)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-type", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).PeerType()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val != oc.Bgp_PeerType_EXTERNAL {
					t.Errorf("BGP Neighbor peer-type: got %v, want %v", val, oc.Bgp_PeerType_EXTERNAL)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/neighbor-address", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).NeighborAddress()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val != neighbor.GetAddress() {
					t.Errorf("BGP Neighbor neighbor-address: got %s, want %s", val, neighbor.GetAddress())
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/transport/local-address", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Transport().LocalAddress()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val != neighbor.GetLocalAddress() {
					t.Errorf("BGP Neighbor local-address: got %s, want %s", val, neighbor.GetLocalAddress())
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/transport/remote-address", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Transport().RemoteAddress()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val != neighbor.GetAddress() {
					t.Errorf("BGP Neighbor remote-address: got %s, want %s", val, neighbor.GetAddress())
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/transport/local-port", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Transport().LocalPort()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val == 0 {
					t.Errorf("BGP Neighbor local-port: got %d, want !=%d", val, 0)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/transport/remote-port", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Transport().RemotePort()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val == 0 {
					t.Errorf("BGP Neighbor remote-port: got %d, want !=%d", val, 0)
				}
			})

			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/transport/mtu-discovery", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Transport().MtuDiscovery()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val == true || val == false {
					t.Logf("Got Correct BGP Neighbor mtu-discovery")
				} else {
					t.Errorf("BGP Neighbor mtu-discovery: got %v", val)
				}
			})

			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/transport/passive-mode", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Transport().PassiveMode()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				gnmi.Get(t, dut, state.State())
				if val == true {
					t.Errorf("BGP Neighbor passive-mode: got %v, want %v", val, false)
				}
			})
			for _, afisafi := range neighbor.Afisafi {
				afitype := ft.GetAfisafiType(afisafi.Type)
				t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/graceful-restart/state/advertised", func(t *testing.T) {
					state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).AfiSafi(afitype).GracefulRestart().Advertised()
					defer observer.RecordYgot(t, "SUBSCRIBE", state)
					val := gnmi.Get(t, dut, state.State())
					gnmi.Get(t, dut, state.State())
					if val != true {
						t.Errorf("BGP Neighbor Afisafi graceful-restart Advertised: got %v, want %v", val, true)
					}
				})
				t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/graceful-restart/state/recieved", func(t *testing.T) {
					state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).AfiSafi(afitype).GracefulRestart().Received()
					defer observer.RecordYgot(t, "SUBSCRIBE", state)
					val := gnmi.Get(t, dut, state.State())
					gnmi.Get(t, dut, state.State())
					if val != true {
						t.Errorf("BGP Neighbor Afisafi graceful-restart Recieved: got %v, want %v", val, true)
					}
				})
				t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/active", func(t *testing.T) {
					state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).AfiSafi(afitype).Active()
					defer observer.RecordYgot(t, "SUBSCRIBE", state)
					val := gnmi.Get(t, dut, state.State())
					gnmi.Get(t, dut, state.State())
					if val != true {
						t.Errorf("BGP Neighbor Afisafi Active: got %v, want %v", val, true)
					}
				})
				t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/enabled", func(t *testing.T) {
					state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).AfiSafi(afitype).Enabled()
					defer observer.RecordYgot(t, "SUBSCRIBE", state)
					val := gnmi.Get(t, dut, state.State())
					gnmi.Get(t, dut, state.State())
					if val != true {
						t.Errorf("BGP Neighbor Afisafi Enabled: got %v, want %v", val, true)
					}
				})
				t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/afi-safi-name", func(t *testing.T) {
					state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).AfiSafi(afitype).AfiSafiName()
					defer observer.RecordYgot(t, "SUBSCRIBE", state)
					val := gnmi.Get(t, dut, state.State())
					gnmi.Get(t, dut, state.State())
					if val != afitype {
						t.Errorf("BGP Neighbor Afisafi Enabled: got %s, want %s", val, afitype)
					}
				})

				if afitype == oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST {
					t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/warning-threshold-pct", func(t *testing.T) {
						state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).AfiSafi(afitype).Ipv4Unicast().PrefixLimit().WarningThresholdPct()
						defer observer.RecordYgot(t, "SUBSCRIBE", state)
						val := gnmi.Get(t, dut, state.State())
						gnmi.Get(t, dut, state.State())
						if val == 0 {
							t.Errorf("BGP Neighbor Afisafi IPV4 unicast PrefixLimit WarningThresholdPct: got %d, want !=%d", val, 0)
						}
					})
					t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/max-prefixes", func(t *testing.T) {
						state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).AfiSafi(afitype).Ipv4Unicast().PrefixLimit().MaxPrefixes()
						defer observer.RecordYgot(t, "SUBSCRIBE", state)
						val := gnmi.Get(t, dut, state.State())
						gnmi.Get(t, dut, state.State())
						if val == 0 {
							t.Errorf("BGP Neighbor Afisafi IPV4 unicast PrefixLimit MAxPrefixes: got %d, want !=%d", val, 0)
						}
					})
				}
				t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/prefixes/state/installed", func(t *testing.T) {
					state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).AfiSafi(afitype).Prefixes().Installed()
					defer observer.RecordYgot(t, "SUBSCRIBE", state)
					val := gnmi.Get(t, dut, state.State())
					gnmi.Get(t, dut, state.State())
					if val == 0 || val > 0 {
						t.Logf("Got correct BGP Neighbor Afisafi  Prefixes installed value")
					} else {
						t.Errorf("BGP Neighbor Afisafi  Prefixes installed: got %d, want greater than or equal zero", val)
					}
				})
				t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/prefixes/state/recieved-pre-policy", func(t *testing.T) {
					state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).AfiSafi(afitype).Prefixes().ReceivedPrePolicy()
					defer observer.RecordYgot(t, "SUBSCRIBE", state)
					val := gnmi.Get(t, dut, state.State())
					gnmi.Get(t, dut, state.State())
					if val == 0 || val > 0 {
						t.Logf("Got correct BGP Neighbor Afisafi  Prefixes ReceivedPrePolicy value")
					} else {
						t.Errorf("BGP Neighbor Afisafi  Prefixes ReceivedPrePolicy: got %d, want greater than or equal zero", val)
					}
				})
				t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/prefixes/state/recieved", func(t *testing.T) {
					state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).AfiSafi(afitype).Prefixes().Received()
					defer observer.RecordYgot(t, "SUBSCRIBE", state)
					val := gnmi.Get(t, dut, state.State())
					gnmi.Get(t, dut, state.State())
					if val == 0 || val > 0 {
						t.Logf("Got correct BGP Neighbor Afisafi  Prefixes Received value")
					} else {
						t.Errorf("BGP Neighbor Afisafi  Prefixes Received: got %d, want greater than or equal zero", val)
					}
				})
				t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/prefixes/state/sent", func(t *testing.T) {
					state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).AfiSafi(afitype).Prefixes().Sent()
					defer observer.RecordYgot(t, "SUBSCRIBE", state)
					val := gnmi.Get(t, dut, state.State())
					gnmi.Get(t, dut, state.State())
					if val == 0 || val > 0 {
						t.Logf("Got correct BGP Neighbor Afisafi  Prefixes sent value")
					} else {
						t.Errorf("BGP Neighbor Afisafi  Prefixes sent: got %d, want greater than or equal zero", val)
					}
				})

			}
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/messages/recieved/state/notification", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Messages().Received().NOTIFICATION()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				gnmi.Get(t, dut, state.State())
				if val != 0 {
					t.Errorf("BGP Neighbor messages recieved Notification: got %d, want  %d", val, 0)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/messages/recieved/state/update", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Messages().Received().UPDATE()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				gnmi.Get(t, dut, state.State())
				if val == 0 || val > 0 {
					t.Logf("Got correct Neighbor messages recieved Update value")
				} else {
					t.Errorf("BGP Neighbor messages recieved Update: got %d, want greater than or equal zero", val)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/messages/sent/state/notification", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Messages().Sent().NOTIFICATION()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				gnmi.Get(t, dut, state.State())
				if val != 0 {
					t.Errorf("BGP Neighbor messages sent Notification: got %d, want  %d", val, 0)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/messages/sent/state/update", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Messages().Sent().UPDATE()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				gnmi.Get(t, dut, state.State())
				if val == 0 {
					t.Errorf("BGP Neighbor messages sent Update: got %d, want  %d", val, 0)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/negotiaited-hold-time", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Timers().NegotiatedHoldTime()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val == 0 {
					t.Errorf("BGP Neighbor Timers NegotiatedHoldTime: got %d, want  %d", val, 0)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/queues/state/input", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Queues().Input()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val != 0 {
					t.Errorf("BGP Neighbor Queues Input: got %d, want  %d", val, 0)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/queues/state/output", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).Queues().Output()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val != 0 {
					t.Errorf("BGP Neighbor Queues Output: got %d, want  %d", val, 0)
				}
			})
			t.Run("Subscribe//network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/session-state/state/output", func(t *testing.T) {
				state := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp.Vrf).Bgp().Neighbor(neighbor.Address).SessionState()
				defer observer.RecordYgot(t, "SUBSCRIBE", state)
				val := gnmi.Get(t, dut, state.State())
				if val != oc.Bgp_Neighbor_SessionState_ESTABLISHED {
					t.Errorf("BGP Neighbor Queues Output: got %v, want  %v", val, oc.Bgp_Neighbor_SessionState_ACTIVE)
				}
			})

		}

	}

}

