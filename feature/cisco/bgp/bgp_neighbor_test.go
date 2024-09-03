package bgp_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

const (
	bgpInstanceNeighbor string = "TEST_N"
	bgpAsNeighbor       uint32 = 50000
	neighborAddress     string = "1.2.3.4"
)

func baseBgpNeighborConfig(bgpAs uint32) *oc.NetworkInstance_Protocol_Bgp {
	return &oc.NetworkInstance_Protocol_Bgp{
		Global: &oc.NetworkInstance_Protocol_Bgp_Global{
			As: ygot.Uint32(bgpAs),
		},
		Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
			neighborAddress: {
				NeighborAddress: ygot.String(neighborAddress),
				PeerAs:          ygot.Uint32(bgpAs + 100),
			},
		},
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enable
// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/enable
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/neighbor-address
func TestNeighborAddress(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []string{
		"1.2.3.4",
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	fixBgpLeafRefConstraints(t, dut, bgpInstance)
	gnmi.Update(t, dut, bgpConfig.Global().As().Config(), bgpAs)
	time.Sleep(configApplyTime)
	bgpNeighbor := bgpConfig.Neighbor(neighborAddress)
	t.Run("Update", func(t *testing.T) {
		gnmi.Update(t, dut, bgpNeighbor.Config(),
			&oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String(neighborAddress),
				PeerAs:          ygot.Uint32(34),
			})
	})
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address using value %v", input), func(t *testing.T) {
			neighborAddressPath := bgpConfig.Neighbor(input).NeighborAddress()
			state := bgpState.Neighbor(input).NeighborAddress()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, neighborAddressPath.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/neighbor-address: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Update /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled=true", func(t *testing.T) {
				gnmi.Update(t, dut, bgpConfig.Neighbor(input).Enabled().Config(), true)
				time.Sleep(configApplyTime)
				stateGot := gnmi.Await(t, dut, bgpState.Neighbor(input).Enabled().State(), telemetryTimeout, true)
				//stateGot := gnmi.Await(t, dut, bgpState.Neighbor(input).Enabled().State(), telemetryTimeout, true)
				value, _ := stateGot.Val()
				if value == false {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/enabled: got %v, want %v", stateGot, true)
				}
			})

			t.Run("Set /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled=false", func(t *testing.T) {
				gnmi.Update(t, dut, bgpConfig.Neighbor(input).Enabled().Config(), false)
				time.Sleep(configApplyTime)
				stateGot := gnmi.Await(t, dut, bgpState.Neighbor(input).Enabled().State(), telemetryTimeout, false)
				value, _ := stateGot.Val()
				if value == true {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/enabled: got %v, want %v", stateGot, false)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, bgpConfig.Neighbor(input).Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[string]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-as
func TestNeighborPeerAs(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint32{
		34,
		// 2004091086,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	fixBgpLeafRefConstraints(t, dut, bgpInstance)
	gnmi.Update(t, dut, bgpConfig.Config(), &oc.NetworkInstance_Protocol_Bgp{
		Global: &oc.NetworkInstance_Protocol_Bgp_Global{
			As: ygot.Uint32(bgpAs),
		},
		Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
			neighborAddress: {
				NeighborAddress: ygot.String(neighborAddress),
			},
		},
	})
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).PeerAs()
			state := bgpState.Neighbor(neighborAddress).PeerAs()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-as: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint32]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/local-as
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/local-as
func TestNeighborLocalAs(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint32{
		// 770,
		1482679779,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	fixBgpLeafRefConstraints(t, dut, bgpInstance)
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/local-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).LocalAs()
			state := bgpState.Neighbor(neighborAddress).LocalAs()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Await(t, dut, state.State(), telemetryTimeout, input)
				val, ok := stateGot.Val()
				if !ok {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/local-as: got %v, want %v", stateGot, input)
				}
				if val != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/local-as: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint32]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/local-as fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/route-flap-damping
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/route-flap-damping
func TestNeighborRouteFlapDamping(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		true,
		false,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	fixBgpLeafRefConstraints(t, dut, bgpInstance)
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/route-flap-damping using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).RouteFlapDamping()
			state := bgpState.Neighbor(neighborAddress).RouteFlapDamping()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/route-flap-damping: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[bool]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/route-flap-damping fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/remove-private-as
func TestNeighborRemovePrivateAs(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []oc.E_Bgp_RemovePrivateAsOption{
		oc.Bgp_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL,  //PRIVATE_AS_REMOVE_ALL
		oc.Bgp_RemovePrivateAsOption_PRIVATE_AS_REPLACE_ALL, //PRIVATE_AS_REPLACE_ALL
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/remove-private-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).RemovePrivateAs()
			state := bgpState.Neighbor(neighborAddress).RemovePrivateAs()
			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/remove-private-as: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[oc.E_Bgp_RemovePrivateAsOption]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/remove-private-as fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/connect-retry
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/connect-retry
func TestNeighborTimersConnectRetry(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		55,
		234,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/connect-retry using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).Timers().ConnectRetry()
			state := bgpState.Neighbor(neighborAddress).Timers().ConnectRetry()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/connect-retry: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint16]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/connect-retry fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/hold-time
func TestNeighborTimersHoldTime(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		60,
		360,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	fixBgpLeafRefConstraints(t, dut, bgpInstanceGlobal)
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time using value %v", input), func(t *testing.T) {
			// holdTime Should be 3x keepAlive, see RFC 4271 - A Border Gateway Protocol 4, Sec. 10
			holdTime := input
			keepAlive := holdTime / 3
			Timers := &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
				HoldTime:          ygot.Uint16(holdTime),
				KeepaliveInterval: ygot.Uint16(keepAlive),
			}

			config := bgpConfig.Neighbor(neighborAddress).Timers()
			state := bgpState.Neighbor(neighborAddress).Timers().HoldTime()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), Timers) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/hold-time: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint16]) bool { return true }).Await(t); qs.String() == "180" {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/keepalive-interval
func TestNeighborTimersKeepaliveInterval(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		60,
		360,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	fixBgpLeafRefConstraints(t, dut, bgpInstanceGlobal)
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval using value %v", input), func(t *testing.T) {
			// holdTime Should be 3x keepAlive, see RFC 4271 - A Border Gateway Protocol 4, Sec. 10
			holdTime := input
			keepAlive := holdTime / 3
			Timers := &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
				HoldTime:          ygot.Uint16(holdTime),
				KeepaliveInterval: ygot.Uint16(keepAlive),
			}

			config := bgpConfig.Neighbor(neighborAddress).Timers()
			state := bgpState.Neighbor(neighborAddress).Timers().KeepaliveInterval()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), Timers) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != keepAlive {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/keepalive-interval: got %v, want %v", stateGot, keepAlive)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint16]) bool { return true }).Await(t); qs.String() == "60" {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/minimum-advertisement-interval
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/minimum-advertisement-interval
func TestNeighborTimersMinimumAdvertisementInterval(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		40,
		41,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	fixBgpLeafRefConstraints(t, dut, bgpInstanceGlobal)
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/minimum-advertisement-interval using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).Timers().MinimumAdvertisementInterval()
			state := bgpState.Neighbor(neighborAddress).Timers().MinimumAdvertisementInterval()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/minimum-advertisement-interval: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint16]) bool { return true }).Await(t); qs.String() == "30" {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/minimum-advertisement-interval fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/enabled
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/enabled
func TestNeighborGracefulRestartEnabled(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		true,
		false,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/enabled using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).GracefulRestart().Enabled()
			state := bgpState.Neighbor(neighborAddress).GracefulRestart().Enabled()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/enabled: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[bool]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/enabled fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/restart-time
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/restart-time
func TestNeighborGracefulRestartRestartTime(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		// 30,
		2631,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	fixBgpLeafRefConstraints(t, dut, bgpInstanceGlobal)
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/restart-time using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).GracefulRestart().RestartTime()
			state := bgpState.Neighbor(neighborAddress).GracefulRestart().RestartTime()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/restart-time: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint16]) bool { return true }).Await(t); qs.String() == "120" {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/restart-time fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/stale-routes-time
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/stale-routes-time
func TestNeighborGracefulRestartStaleRoutesTime(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		12,
		23,
		// 50.68,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	fixBgpLeafRefConstraints(t, dut, bgpInstanceGlobal)
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/stale-routes-time using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).GracefulRestart().StaleRoutesTime()
			state := bgpState.Neighbor(neighborAddress).GracefulRestart().StaleRoutesTime()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/stale-routes-time: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint16]) bool { return true }).Await(t); qs.String() == "360" {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/stale-routes-time fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/helper-only
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/helper-only
func TestNeighborGracefulRestartHelperOnly(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		false,
		true,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/helper-only using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).GracefulRestart().HelperOnly()
			state := bgpState.Neighbor(neighborAddress).GracefulRestart().HelperOnly()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/helper-only: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[bool]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/helper-only fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/import-policy
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/state/import-policy
func TestNeighborApplyPolicyImportPolicy(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := [][]string{
		{
			"TEST_ACCEPT",
		},
		{
			"TEST_REJECT",
			"TEST_ACCEPT",
		},
	}

	acceptMap := &oc.RoutingPolicy_PolicyDefinition_Statement_OrderedMap{}
	acceptAction, _ := acceptMap.AppendNew("id-1")
	acceptAction.Actions = &oc.RoutingPolicy_PolicyDefinition_Statement_Actions{
		PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	}

	rejectMap := &oc.RoutingPolicy_PolicyDefinition_Statement_OrderedMap{}
	rejectAction, _ := rejectMap.AppendNew("id-1")
	rejectAction.Actions = &oc.RoutingPolicy_PolicyDefinition_Statement_Actions{
		PolicyResult: oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), &oc.RoutingPolicy{
		PolicyDefinition: map[string]*oc.RoutingPolicy_PolicyDefinition{
			"TEST_ACCEPT": {
				Name:      ygot.String("TEST_ACCEPT"),
				Statement: acceptMap,
			},
			"TEST_REJECT": {
				Name:      ygot.String("TEST_REJECT"),
				Statement: rejectMap,
			},
		},
	})
	time.Sleep(configApplyTime)
	batchDelete := &gnmi.SetBatch{}
	gnmi.BatchDelete(batchDelete, bgpConfig.Config())
	gnmi.BatchDelete(batchDelete, gnmi.OC().RoutingPolicy().PolicyDefinition("TEST_ACCEPT").Config())
	gnmi.BatchDelete(batchDelete, gnmi.OC().RoutingPolicy().PolicyDefinition("TEST_REJECT").Config())
	defer batchDelete.Set(t, dut)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/import-policy using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).ApplyPolicy().ImportPolicy()
			state := bgpState.Neighbor(neighborAddress).ApplyPolicy().ImportPolicy()
			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				for i, sg := range stateGot {
					if sg != input[i] {
						t.Errorf("Config /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/state/import-policy: got %v, want %v", sg, input[i])
					}
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[[]string]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/import-policy fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/default-import-policy
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/state/default-import-policy
func TestNeighborApplyPolicyDefaultImportPolicy(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []oc.E_RoutingPolicy_DefaultPolicyType{
		oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE,
		oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/default-import-policy using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).ApplyPolicy().DefaultImportPolicy()
			state := bgpState.Neighbor(neighborAddress).ApplyPolicy().DefaultImportPolicy()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/state/default-import-policy: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[oc.E_RoutingPolicy_DefaultPolicyType]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/default-import-policy fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/export-policy
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/state/export-policy
func TestNeighborApplyPolicyExportPolicy(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := [][]string{
		{
			"TEST_REJECT",
			"TEST_ACCEPT",
		},
		{
			"TEST_REJECT",
		},
	}

	acceptMap := &oc.RoutingPolicy_PolicyDefinition_Statement_OrderedMap{}
	acceptAction, _ := acceptMap.AppendNew("id-1")
	acceptAction.Actions = &oc.RoutingPolicy_PolicyDefinition_Statement_Actions{
		PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
	}

	rejectMap := &oc.RoutingPolicy_PolicyDefinition_Statement_OrderedMap{}
	rejectAction, _ := rejectMap.AppendNew("id-1")
	rejectAction.Actions = &oc.RoutingPolicy_PolicyDefinition_Statement_Actions{
		PolicyResult: oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), &oc.RoutingPolicy{
		PolicyDefinition: map[string]*oc.RoutingPolicy_PolicyDefinition{
			"TEST_ACCEPT": {
				Name:      ygot.String("TEST_ACCEPT"),
				Statement: acceptMap,
			},
			"TEST_REJECT": {
				Name:      ygot.String("TEST_REJECT"),
				Statement: rejectMap,
			},
		},
	})
	time.Sleep(configApplyTime)
	batchDelete := &gnmi.SetBatch{}
	gnmi.BatchDelete(batchDelete, bgpConfig.Config())
	gnmi.BatchDelete(batchDelete, gnmi.OC().RoutingPolicy().PolicyDefinition("TEST_ACCEPT").Config())
	gnmi.BatchDelete(batchDelete, gnmi.OC().RoutingPolicy().PolicyDefinition("TEST_REJECT").Config())
	defer batchDelete.Set(t, dut)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/export-policy using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).ApplyPolicy().ExportPolicy()
			state := bgpState.Neighbor(neighborAddress).ApplyPolicy().ExportPolicy()
			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				for i, sg := range stateGot {
					if sg != input[i] {
						t.Errorf("Config /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/state/export-policy: got %v, want %v", sg, input[i])
					}
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[[]string]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/export-policy fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/default-export-policy
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/state/default-export-policy
func TestNeighborApplyPolicyDefaultExportPolicy(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []oc.E_RoutingPolicy_DefaultPolicyType{
		oc.E_RoutingPolicy_DefaultPolicyType(2), //REJECT_ROUTE
		oc.E_RoutingPolicy_DefaultPolicyType(1), //ACCEPT_ROUTE
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/default-export-policy using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).ApplyPolicy().DefaultExportPolicy()
			state := bgpState.Neighbor(neighborAddress).ApplyPolicy().DefaultExportPolicy()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/state/default-export-policy: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[oc.E_RoutingPolicy_DefaultPolicyType]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/default-export-policy fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/enabled
func TestNeighborAfiSafiEnabled(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		false,
		true,
	}

	// Remove any existing BGP config
	config.TextWithGNMI(context.Background(), t, dut, "no router bgp 65000")

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	baseConfig := baseBgpNeighborConfig(bgpAs)
	baseConfig.Neighbor[neighborAddress].GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	gnmi.Update(t, dut, bgpConfig.Config(), baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled using value %v", input), func(t *testing.T) {
			globalAddrFamilyConfig := bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
			gnmi.Update(t, dut, bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).AfiSafiName().Config(), oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, globalAddrFamilyConfig.Config(), input) })
			time.Sleep(configApplyTime)

			enabledConfigPath := bgpConfig.Neighbor(neighborAddress).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
			enabledStatePath := bgpState.Neighbor(neighborAddress).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, enabledConfigPath.Config(), input) })
			time.Sleep(configApplyTime)

			// CSCwe29261 : "enabledConfigPath/enabled” is set to FALSE means neighbor under “router bgp <AS>” doesn’t have that AFI
			if input == true {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, enabledStatePath.State())
					if stateGot != input {
						t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/enabledStatePath/enabled: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/max-prefixes
func TestNeighborAfiSafiMaxPrefixes(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint32{
		234567,
	}

	// Remove any existing BGP config
	config.TextWithGNMI(context.Background(), t, dut, "no router bgp 65000")

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	baseConfig := baseBgpNeighborConfig(bgpAs)
	fixBgpLeafRefConstraints(t, dut, bgpInstance)
	baseConfig.Neighbor[neighborAddress].GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	gnmi.Update(t, dut, bgpConfig.Config(), baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes using value %v", input), func(t *testing.T) {
			globalAddrFamilyConfig := bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
			gnmi.Update(t, dut, bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).AfiSafiName().Config(), oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, globalAddrFamilyConfig.Config(), true) })
			time.Sleep(configApplyTime)

			configNeighborAddr := bgpConfig.Neighbor(neighborAddress).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, configNeighborAddr.Config(), true) })
			time.Sleep(configApplyTime)

			maxPrefixesConfigPath := bgpConfig.Neighbor(neighborAddress).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().PrefixLimit().MaxPrefixes()
			maxPrefixesStatePath := bgpState.Neighbor(neighborAddress).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().PrefixLimit().MaxPrefixes()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, maxPrefixesConfigPath.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, maxPrefixesStatePath.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/maxPrefixesStatePath/max-prefixes: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, maxPrefixesConfigPath.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Lookup(t, dut, maxPrefixesStatePath.State()).Val(); qs != uint32(4294967295) {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/maxPrefixesConfigPath/max-prefixes fail: got %v,want %v", qs, uint32(4294967295))
				}
			})
		})
	}
}

// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/local-restarting
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/peer-restarting
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/mode
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/peer-restart-time
func TestNeighborGracefulRestartState(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	mode := []bool{
		true,
		false,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Global().As().Config(), bgpAs)
	time.Sleep(configApplyTime)
	gnmi.Update(t, dut, bgpConfig.Global().GracefulRestart().Enabled().Config(), true)
	time.Sleep(configApplyTime)
	bgpNeighbor := bgpConfig.Neighbor(neighborAddress)
	t.Run("Update", func(t *testing.T) {
		gnmi.Update(t, dut, bgpNeighbor.Config(),
			&oc.NetworkInstance_Protocol_Bgp_Neighbor{
				NeighborAddress: ygot.String(neighborAddress),
				PeerAs:          ygot.Uint32(34),
			})
	})
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range mode {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state leafs using helper-only value %v", input), func(t *testing.T) {
			if input {
				gnmi.Update(t, dut, bgpConfig.Global().GracefulRestart().HelperOnly().Config(), true)
				time.Sleep(configApplyTime)
			}
			state := bgpState.Neighbor(neighborAddress).GracefulRestart()
			t.Run("Get state: local-restarting", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.LocalRestarting().State())
				if stateGot != false {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/local-restarting: got %v, want false", stateGot)
				}
			})
			t.Run("Get state: peer-restarting", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.PeerRestarting().State())
				if stateGot != false {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/peer-restarting: got %v, want false", stateGot)
				}
			})
			t.Run("Get state: peer-restart-time", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.PeerRestartTime().State())
				if stateGot != 0 {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/peer-restart-time: got %v, want ", stateGot)
				}
			})
			t.Run("Get state: mode", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.Mode().State())
				if input && stateGot != oc.GracefulRestart_Mode_HELPER_ONLY {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/mode: got %v, want %v", stateGot, input)
				}
			})

		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/error-handling/config/treat-as-withdraw
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/error-handling/state/treat-as-withdraw
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/error-handling/state/erroneous-update-messages
func TestNeighborErrorHandlingTreatAsWithdraw(t *testing.T) {

	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		true,
		false,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	baseConfig := baseBgpNeighborConfig(bgpAs)
	fixBgpLeafRefConstraints(t, dut, bgpInstance)
	gnmi.Update(t, dut, bgpConfig.Config(), baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/error-handling/config/treat-as-withdraw using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).ErrorHandling().TreatAsWithdraw()
			state := bgpState.Neighbor(neighborAddress).ErrorHandling().TreatAsWithdraw()
			erroneous_state := bgpState.Neighbor(neighborAddress).ErrorHandling().ErroneousUpdateMessages()

			t.Run("TreatasWDR_Config", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("TreatasWDR_State", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				fmt.Println("Check value stateGot", stateGot)
				fmt.Println("Check value input", input)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/error-handling/state/treat-as-withdraw: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Erroneous_upd_msg_State", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, erroneous_state.State())
				fmt.Println("Check value stateGot", stateGot)
				if stateGot != 0 {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/error-handling/state/erroneous-update-messages: got %v, want %v", stateGot, 0)
				}
			})

			t.Run("TreatasWDR_Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				stateGot1 := gnmi.Get(t, dut, state.State())
				stateGot2 := gnmi.Get(t, dut, erroneous_state.State())
				if stateGot1 != false {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/error-handling/state/treat-as-withdraw: got %v, want %v", stateGot1, false)
				}
				if stateGot2 != 0 {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/error-handling/state/erroneous-update-messages: got %v, want %v", stateGot2, 0)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/config/disable-peer-as-filter
// State:  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/state/disable-peer-as-filter
func TestNeighborASPathOptDisablePeerAS(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		true,
		false,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	baseConfig := baseBgpNeighborConfig(bgpAs)
	fixBgpLeafRefConstraints(t, dut, bgpInstance)
	gnmi.Update(t, dut, bgpConfig.Config(), baseConfig)
	//gnmi.Update(t, dut, bgpConfig.Global().As().Config(), bgpAs)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/config/disable-peer-as-filter using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).AsPathOptions().DisablePeerAsFilter()
			state := bgpState.Neighbor(neighborAddress).AsPathOptions().DisablePeerAsFilter()

			t.Run("update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				fmt.Println("Check value stateGot", stateGot)
				fmt.Println("Check value input", input)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/state/disable-peer-as-filter: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[bool]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/config/disable-peer-as-filter: got %v, want %v", qs, "")
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/config/allow-own-as
// State:  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/state/allow-own-as
func TestNeighborASPathOptionAllowOwnAS(t *testing.T) {
	dut := ondatra.DUT(t, dutName)
	inputs := []uint8{
		4,
		7,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	baseConfig := baseBgpNeighborConfig(bgpAs)
	fixBgpLeafRefConstraints(t, dut, bgpInstance)
	gnmi.Update(t, dut, bgpConfig.Config(), baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/config/allow-own-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).AsPathOptions().AllowOwnAs()
			state := bgpState.Neighbor(neighborAddress).AsPathOptions().AllowOwnAs()

			t.Run("AllowOwnAS_Config", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("AllowOwnAS_State", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				fmt.Println("Check value stateGot", stateGot)
				fmt.Println("Check value input", input)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/state/allow-own-as: got %v, want %v", stateGot, input)
				}
			})

			t.Run("AllowOwnAS_Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint8]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/config/allow-own-as: got %v, want %v", qs, "")
				}
			})

		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/send-community
// State:  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/send-community
func TestNeighborSendCommunity(t *testing.T) {
	dut := ondatra.DUT(t, dutName)
	inputs := []oc.E_Bgp_CommunityType{
		oc.Bgp_CommunityType_STANDARD,
		oc.Bgp_CommunityType_EXTENDED,
		oc.Bgp_CommunityType_BOTH,
		oc.Bgp_CommunityType_NONE,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	baseConfig := baseBgpNeighborConfig(bgpAs)
	fixBgpLeafRefConstraints(t, dut, bgpInstance)
	gnmi.Update(t, dut, bgpConfig.Config(), baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/send-community using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).SendCommunity()
			state := bgpState.Neighbor(neighborAddress).SendCommunity()

			t.Run("SendCommunity_Config", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("SendCommunity_State", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				fmt.Println("Check value stateGot", stateGot)
				fmt.Println("Check value input", input)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/send-community: got %v, want %v", stateGot, input)
				}
			})

			t.Run("SendCommunity_Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				stateGot1 := gnmi.Get(t, dut, state.State())
				if stateGot1 != oc.Bgp_CommunityType_NONE {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/send-community: got %v, want %v", stateGot1, oc.Bgp_CommunityType_NONE)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/config/replace-peer-as
// State:  /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/state/replace-peer-as
func TestNeighborASPathOptReplacePeerAS(t *testing.T) {
	dut := ondatra.DUT(t, dutName)
	inputs := []bool{
		true,
		false,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	baseConfig := baseBgpNeighborConfig(bgpAs)
	fixBgpLeafRefConstraints(t, dut, bgpInstance)
	gnmi.Update(t, dut, bgpConfig.Config(), baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/config/replace-peer-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).AsPathOptions().ReplacePeerAs()
			state := bgpState.Neighbor(neighborAddress).AsPathOptions().ReplacePeerAs()

			t.Run("update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				fmt.Println("Check value stateGot", stateGot)
				fmt.Println("Check value input", input)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/state/replace-peer-as: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[bool]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/as-path-options/config/replace-peer-as: got %v, want %v", qs, "")
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/config/route-reflector-client
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/state/route-reflector-client
func TestNeighborRouteReflectorClient(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		true,
		false,
	}

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgpAs))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/config/route-reflector-client using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighborAddress).RouteReflector().RouteReflectorClient()
			state := bgpState.Neighbor(neighborAddress).RouteReflector().RouteReflectorClient()
			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/state/route-reflector-client: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[bool]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/config/route-reflector-client fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/use-multiple-paths/config/enabled
// State: /network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/use-multiple-paths/state/enabled
func TestNeighborAfiSafiUseMultiplePathsEnabled(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		true,
		false,
	}

	// Remove any existing BGP enabledPath
	config.TextWithGNMI(context.Background(), t, dut, "no router bgp")

	bgpInstance, bgpAs := getNextBgpInstance(bgpInstanceNeighbor, bgpAsNeighbor)
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	baseConfig := baseBgpNeighborConfig(bgpAs)
	baseConfig.Neighbor[neighborAddress].GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	fixBgpLeafRefConstraints(t, dut, bgpInstance)
	gnmi.Update(t, dut, bgpConfig.Config(), baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstance)

	globalAddrFamilyConfig := bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
	gnmi.Update(t, dut, bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).AfiSafiName().Config(), oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, globalAddrFamilyConfig.Config(), true) })
	time.Sleep(configApplyTime)
	enabledPath := bgpConfig.Neighbor(neighborAddress).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
	gnmi.Update(t, dut, bgpConfig.Neighbor(neighborAddress).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).AfiSafiName().Config(), oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, enabledPath.Config(), true) })
	time.Sleep(configApplyTime)

	for _, input := range inputs {
		umpConfig := bgpConfig.Neighbor(neighborAddress).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).UseMultiplePaths().Enabled()
		t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, umpConfig.Config(), input) })
		time.Sleep(configApplyTime)
		t.Run("UseMultiplePathsEnabled_State", func(t *testing.T) {
			stateGot := gnmi.Get(t, dut, bgpState.Neighbor(neighborAddress).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).UseMultiplePaths().Enabled().State())

			if stateGot != input {
				t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/use-multiple-paths/state/enabled: got %v, want %v", stateGot, input)
			}
		})

		t.Run("Subscribe", func(t *testing.T) {
			stateGot := gnmi.Get(t, dut, bgpState.Neighbor(neighborAddress).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).UseMultiplePaths().Enabled().State())
			if stateGot != input {
				t.Errorf("State-Telemetry /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/use-multiple-paths/state/enabled: got %v, want %v", stateGot, input)
			}
		})
	}
}
