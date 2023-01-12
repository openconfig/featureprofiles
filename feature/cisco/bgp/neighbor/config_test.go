package bgp_neighbor_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/config"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	telemetryTimeout time.Duration = 60 * time.Second
	configApplyTime  time.Duration = 5 * time.Second // FIXME: Workaround
	configDeleteTime time.Duration = 5 * time.Second // FIXME: Workaround
	dutName          string        = "dut"
	bgpInstance_0    string        = "TEST_N"
	bgpAs_0          uint32        = 50000
	neighbor_address string        = "1.2.3.4"
)

var index uint32 = 1

// NOTE: Using separate BGP instances due to XR errors when back-to-back
// delete and re-add hits failure on BGP backend cleanup.
// FIXME: May need to be triaged in XR BGP implementation or XR config backend.
func getNextBgpInstance() (string, uint32) {
	i := index
	index++
	return fmt.Sprintf("%s_%d", bgpInstance_0, i), i + bgpAs_0
}

func baseBgpNeighborConfig(bgpAs uint32) *oc.NetworkInstance_Protocol_Bgp {
	return &oc.NetworkInstance_Protocol_Bgp{
		Global: &oc.NetworkInstance_Protocol_Bgp_Global{
			As: ygot.Uint32(bgpAs),
		},
		Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
			neighbor_address: {
				NeighborAddress: ygot.String(neighbor_address),
				PeerAs:          ygot.Uint32(bgpAs + 100),
			},
		},
	}
}

func cleanup(t *testing.T, dut *ondatra.DUTDevice, bgpInst string) {
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInst).Bgp().Config())
	time.Sleep(configDeleteTime)
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

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Global().As().Config(), bgp_as)
	time.Sleep(configApplyTime)
	config := bgpConfig.Neighbor(neighbor_address).PeerAs()
	t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), 34) })
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(input).NeighborAddress()
			state := bgpState.Neighbor(input).NeighborAddress()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
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
func TestPeerAs(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint32{
		34,
		// 2004091086,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), &oc.NetworkInstance_Protocol_Bgp{
		Global: &oc.NetworkInstance_Protocol_Bgp_Global{
			As: ygot.Uint32(bgp_as),
		},
		Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
			neighbor_address: {
				NeighborAddress: ygot.String(neighbor_address),
			},
		},
	})
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/peer-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).PeerAs()
			state := bgpState.Neighbor(neighbor_address).PeerAs()

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
func TestLocalAs(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint32{
		// 770,
		1482679779,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/local-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).LocalAs()
			state := bgpState.Neighbor(neighbor_address).LocalAs()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
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

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/remove-private-as
func TestRemovePrivateAs(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []oc.E_Bgp_RemovePrivateAsOption{
		oc.Bgp_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL,  //PRIVATE_AS_REMOVE_ALL
		oc.Bgp_RemovePrivateAsOption_PRIVATE_AS_REPLACE_ALL, //PRIVATE_AS_REPLACE_ALL
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/remove-private-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).RemovePrivateAs()
			state := bgpState.Neighbor(neighbor_address).RemovePrivateAs()
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
func TestTimersConnectRetry(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		55,
		234,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/connect-retry using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).Timers().ConnectRetry()
			state := bgpState.Neighbor(neighbor_address).Timers().ConnectRetry()

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
func TestTimersHoldTime(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		60,
		360,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time using value %v", input), func(t *testing.T) {
			// holdTime Should be 3x keepAlive, see RFC 4271 - A Border Gateway Protocol 4, Sec. 10
			holdTime := input
			keepAlive := holdTime / 3
			Timers := &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
				HoldTime:          ygot.Uint16(holdTime),
				KeepaliveInterval: ygot.Uint16(keepAlive),
			}

			config := bgpConfig.Neighbor(neighbor_address).Timers()
			state := bgpState.Neighbor(neighbor_address).Timers().HoldTime()

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
func TestTimersKeepaliveInterval(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		60,
		360,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval using value %v", input), func(t *testing.T) {
			// holdTime Should be 3x keepAlive, see RFC 4271 - A Border Gateway Protocol 4, Sec. 10
			holdTime := input
			keepAlive := holdTime / 3
			Timers := &oc.NetworkInstance_Protocol_Bgp_Neighbor_Timers{
				HoldTime:          ygot.Uint16(holdTime),
				KeepaliveInterval: ygot.Uint16(keepAlive),
			}

			config := bgpConfig.Neighbor(neighbor_address).Timers()
			state := bgpState.Neighbor(neighbor_address).Timers().KeepaliveInterval()

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
func TestTimersMinimumAdvertisementInterval(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		40,
		41,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/minimum-advertisement-interval using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).Timers().MinimumAdvertisementInterval()
			state := bgpState.Neighbor(neighbor_address).Timers().MinimumAdvertisementInterval()

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
func TestGracefulRestartEnabled(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		true,
		false,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/enabled using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).GracefulRestart().Enabled()
			state := bgpState.Neighbor(neighbor_address).GracefulRestart().Enabled()

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
func TestGracefulRestartRestartTime(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		// 30,
		2631,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/restart-time using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).GracefulRestart().RestartTime()
			state := bgpState.Neighbor(neighbor_address).GracefulRestart().RestartTime()

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
func TestGracefulRestartStaleRoutesTime(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		12,
		23,
		// 50.68,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/stale-routes-time using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).GracefulRestart().StaleRoutesTime()
			state := bgpState.Neighbor(neighbor_address).GracefulRestart().StaleRoutesTime()

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
func TestGracefulRestartHelperOnly(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		false,
		true,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/helper-only using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).GracefulRestart().HelperOnly()
			state := bgpState.Neighbor(neighbor_address).GracefulRestart().HelperOnly()

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
func TestApplyPolicyImportPolicy(t *testing.T) {
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

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), &oc.RoutingPolicy{
		PolicyDefinition: map[string]*oc.RoutingPolicy_PolicyDefinition{
			"TEST_ACCEPT": {
				Name: ygot.String("TEST_ACCEPT"),
				Statement: map[string]*oc.RoutingPolicy_PolicyDefinition_Statement{
					"id-1": {
						Name: ygot.String("id-1"),
						Actions: &oc.RoutingPolicy_PolicyDefinition_Statement_Actions{
							PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
						},
					},
				},
			},
			"TEST_REJECT": {
				Name: ygot.String("TEST_REJECT"),
				Statement: map[string]*oc.RoutingPolicy_PolicyDefinition_Statement{
					"id-1": {
						Name: ygot.String("id-1"),
						Actions: &oc.RoutingPolicy_PolicyDefinition_Statement_Actions{
							PolicyResult: oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE,
						},
					},
				},
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
			config := bgpConfig.Neighbor(neighbor_address).ApplyPolicy().ImportPolicy()
			state := bgpState.Neighbor(neighbor_address).ApplyPolicy().ImportPolicy()
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
func TestApplyPolicyDefaultImportPolicy(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []oc.E_RoutingPolicy_DefaultPolicyType{
		oc.RoutingPolicy_DefaultPolicyType_REJECT_ROUTE,
		oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/default-import-policy using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).ApplyPolicy().DefaultImportPolicy()
			state := bgpState.Neighbor(neighbor_address).ApplyPolicy().DefaultImportPolicy()

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
func TestApplyPolicyExportPolicy(t *testing.T) {
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

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	gnmi.Update(t, dut, gnmi.OC().RoutingPolicy().Config(), &oc.RoutingPolicy{
		PolicyDefinition: map[string]*oc.RoutingPolicy_PolicyDefinition{
			"TEST_ACCEPT": {
				Name: ygot.String("TEST_ACCEPT"),
				Statement: map[string]*oc.RoutingPolicy_PolicyDefinition_Statement{
					"id-1": {
						Name: ygot.String("id-1"),
						Actions: &oc.RoutingPolicy_PolicyDefinition_Statement_Actions{
							PolicyResult: oc.RoutingPolicy_PolicyResultType_ACCEPT_ROUTE,
						},
					},
				},
			},
			"TEST_REJECT": {
				Name: ygot.String("TEST_REJECT"),
				Statement: map[string]*oc.RoutingPolicy_PolicyDefinition_Statement{
					"id-1": {
						Name: ygot.String("id-1"),
						Actions: &oc.RoutingPolicy_PolicyDefinition_Statement_Actions{
							PolicyResult: oc.RoutingPolicy_PolicyResultType_REJECT_ROUTE,
						},
					},
				},
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
			config := bgpConfig.Neighbor(neighbor_address).ApplyPolicy().ExportPolicy()
			state := bgpState.Neighbor(neighbor_address).ApplyPolicy().ExportPolicy()
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
func TestApplyPolicyDefaultExportPolicy(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []oc.E_RoutingPolicy_DefaultPolicyType{
		oc.E_RoutingPolicy_DefaultPolicyType(2), //REJECT_ROUTE
		oc.E_RoutingPolicy_DefaultPolicyType(1), //ACCEPT_ROUTE
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/default-export-policy using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).ApplyPolicy().DefaultExportPolicy()
			state := bgpState.Neighbor(neighbor_address).ApplyPolicy().DefaultExportPolicy()

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
func TestAfiSafiEnabled(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		false,
		true,
	}

	// Remove any existing BGP config
	config.TextWithGNMI(context.Background(), t, dut, "no router bgp 65000")

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	baseConfig := baseBgpNeighborConfig(bgp_as)
	baseConfig.Neighbor[neighbor_address].GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	gnmi.Update(t, dut, bgpConfig.Config(), baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled using value %v", input), func(t *testing.T) {

			global_addr_family_config := bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, global_addr_family_config.Config(), input) })
			time.Sleep(configApplyTime)

			config := bgpConfig.Neighbor(neighbor_address).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
			state := bgpState.Neighbor(neighbor_address).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			// CSCwe29261 : "config/enabled” is set to FALSE means neighbor under “router bgp <AS>” doesn’t have that AFI
			if input == true {
				t.Run("Subscribe", func(t *testing.T) {
					stateGot := gnmi.Get(t, dut, state.State())
					if stateGot != input {
						t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/enabled: got %v, want %v", stateGot, input)
					}
				})
			}
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/max-prefixes
func TestAfiSafiMaxPrefixes(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint32{
		// 10,
		// 1000,
		234567,
	}

	// Remove any existing BGP config
	config.TextWithGNMI(context.Background(), t, dut, "no router bgp 65000")

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	baseConfig := baseBgpNeighborConfig(bgp_as)
	baseConfig.Neighbor[neighbor_address].GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	gnmi.Update(t, dut, bgpConfig.Config(), baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes using value %v", input), func(t *testing.T) {

			global_addr_family_config := bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, global_addr_family_config.Config(), true) })
			time.Sleep(configApplyTime)

			config_neighbor_addr := bgpConfig.Neighbor(neighbor_address).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config_neighbor_addr.Config(), true) })
			time.Sleep(configApplyTime)

			config := bgpConfig.Neighbor(neighbor_address).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().PrefixLimit().MaxPrefixes()
			state := bgpState.Neighbor(neighbor_address).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().PrefixLimit().MaxPrefixes()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/max-prefixes: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Lookup(t, dut, state.State()).Val(); qs != uint32(4294967295) {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes fail: got %v,want %v", qs, uint32(4294967295))
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/config/route-reflector-client
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/state/route-reflector-client
func TestRouteReflectorClient(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		true,
		false,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/config/route-reflector-client using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).RouteReflector().RouteReflectorClient()
			state := bgpState.Neighbor(neighbor_address).RouteReflector().RouteReflectorClient()
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

// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/local-restarting
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/peer-restarting
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/mode
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/peer-restart-time
func TestGracefulRestartState(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	mode := []bool{
		true,
		false,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Global().As().Config(), bgp_as)
	time.Sleep(configApplyTime)
	gnmi.Update(t, dut, bgpConfig.Global().GracefulRestart().Enabled().Config(), true)
	time.Sleep(configApplyTime)
	config := bgpConfig.Neighbor(neighbor_address).PeerAs()
	t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), 34) })
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range mode {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state leafs using helper-only value %v", input), func(t *testing.T) {
			if input {
				gnmi.Update(t, dut, bgpConfig.Global().GracefulRestart().HelperOnly().Config(), true)
				time.Sleep(configApplyTime)
			}
			state := bgpState.Neighbor(neighbor_address).GracefulRestart()
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
