package bgp_neighbor_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	telemetryTimeout time.Duration = 10 * time.Second
	configApplyTime  time.Duration = 5 * time.Second // FIXME: Workaround
	configDeleteTime time.Duration = 5 * time.Second // FIXME: Workaround
	dutName          string        = "dut"
	networkInstance  string        = "default"
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
	dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInst).Bgp().Delete(t)
	time.Sleep(configDeleteTime)
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enable
// Config: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/enable
// State: /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/neighbor-address
func TestNeighborAddress(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []string{
		// "12.13.14.15",
		// "2008:23::1",
		"b:04:188:FaB:75:28:b:fd",
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Global().As().Update(t, bgp_as)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/neighbor-address using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(input).NeighborAddress()
			state := bgpState.Neighbor(input).NeighborAddress()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/neighbor-address: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Update /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled=true", func(t *testing.T) {
				bgpConfig.Neighbor(input).Enabled().Update(t, true)
				time.Sleep(configApplyTime)
				stateGot := bgpState.Neighbor(input).Enabled().Await(t, telemetryTimeout, true)
				if stateGot.Val(t) == false {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/enabled: got %v, want %v", stateGot, true)
				}
			})

			t.Run("Set /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/enabled=false", func(t *testing.T) {
				bgpConfig.Neighbor(input).Enabled().Update(t, false)
				time.Sleep(configApplyTime)
				stateGot := bgpState.Neighbor(input).Enabled().Await(t, telemetryTimeout, false)
				if stateGot.Val(t) == true {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/enabled: got %v, want %v", stateGot, false)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				bgpConfig.Neighbor(input).Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedString) bool { return true }).Await(t); qs.IsPresent() {
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
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, &oc.NetworkInstance_Protocol_Bgp{
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

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/peer-as: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedUint32) bool { return true }).Await(t); qs.IsPresent() {
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
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/local-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).LocalAs()
			state := bgpState.Neighbor(neighbor_address).LocalAs()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/local-as: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedUint32) bool { return true }).Await(t); qs.IsPresent() {
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

	inputs := []oc.E_BgpTypes_RemovePrivateAsOption{
		oc.E_BgpTypes_RemovePrivateAsOption(1), //PRIVATE_AS_REMOVE_ALL
		oc.E_BgpTypes_RemovePrivateAsOption(2), //PRIVATE_AS_REPLACE_ALL
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/config/remove-private-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).RemovePrivateAs()
			state := bgpState.Neighbor(neighbor_address).RemovePrivateAs()
			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/state/remove-private-as: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedE_BgpTypes_RemovePrivateAsOption) bool { return true }).Await(t); qs.IsPresent() {
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

	inputs := []float64{
		55,
		234.5,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/connect-retry using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).Timers().ConnectRetry()
			state := bgpState.Neighbor(neighbor_address).Timers().ConnectRetry()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/connect-retry: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedFloat64) bool { return true }).Await(t); qs.IsPresent() && qs.Val(t) != 30 {
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

	inputs := []float64{
		81,
		321.5,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/hold-time using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).Timers().HoldTime()
			state := bgpState.Neighbor(neighbor_address).Timers().HoldTime()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/hold-time: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedFloat64) bool { return true }).Await(t); qs.IsPresent() && qs.Val(t) != 90 {
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

	inputs := []float64{
		65,
		145.3,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/keepalive-interval using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).Timers().KeepaliveInterval()
			state := bgpState.Neighbor(neighbor_address).Timers().KeepaliveInterval()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/keepalive-interval: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedFloat64) bool { return true }).Await(t); qs.IsPresent() && qs.Val(t) != 30 {
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

	inputs := []float64{
		40,
		40.1,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/config/minimum-advertisement-interval using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).Timers().MinimumAdvertisementInterval()
			state := bgpState.Neighbor(neighbor_address).Timers().MinimumAdvertisementInterval()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/timers/state/minimum-advertisement-interval: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedFloat64) bool { return true }).Await(t); qs.IsPresent() && qs.Val(t) != 30 {
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
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/enabled using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).GracefulRestart().Enabled()
			state := bgpState.Neighbor(neighbor_address).GracefulRestart().Enabled()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/enabled: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedBool) bool { return true }).Await(t); qs.IsPresent() {
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
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/restart-time using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).GracefulRestart().RestartTime()
			state := bgpState.Neighbor(neighbor_address).GracefulRestart().RestartTime()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/restart-time: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedUint16) bool { return true }).Await(t); qs.IsPresent() {
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

	inputs := []float64{
		12,
		23.4,
		// 50.68,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/stale-routes-time using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).GracefulRestart().StaleRoutesTime()
			state := bgpState.Neighbor(neighbor_address).GracefulRestart().StaleRoutesTime()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/stale-routes-time: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedFloat64) bool { return true }).Await(t); qs.IsPresent() {
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
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/config/helper-only using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).GracefulRestart().HelperOnly()
			state := bgpState.Neighbor(neighbor_address).GracefulRestart().HelperOnly()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/graceful-restart/state/helper-only: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedBool) bool { return true }).Await(t); qs.IsPresent() {
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
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	dut.Config().RoutingPolicy().Update(t, &oc.RoutingPolicy{
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
	batchDelete := dut.Config().NewBatch()
	bgpConfig.BatchDelete(t, batchDelete)
	dut.Config().RoutingPolicy().PolicyDefinition("TEST_ACCEPT").BatchDelete(t, batchDelete)
	dut.Config().RoutingPolicy().PolicyDefinition("TEST_REJECT").BatchDelete(t, batchDelete)
	defer batchDelete.Set(t)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/import-policy using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).ApplyPolicy().ImportPolicy()
			state := bgpState.Neighbor(neighbor_address).ApplyPolicy().ImportPolicy()
			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				for i, sg := range stateGot {
					if sg != input[i] {
						t.Errorf("Config /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/state/import-policy: got %v, want %v", sg, input[i])
					}
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedStringSlice) bool { return true }).Await(t); qs.IsPresent() {
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
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/default-import-policy using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).ApplyPolicy().DefaultImportPolicy()
			state := bgpState.Neighbor(neighbor_address).ApplyPolicy().DefaultImportPolicy()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/state/default-import-policy: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedE_RoutingPolicy_DefaultPolicyType) bool { return true }).Await(t); qs.IsPresent() {
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
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	dut.Config().RoutingPolicy().Update(t, &oc.RoutingPolicy{
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
	batchDelete := dut.Config().NewBatch()
	bgpConfig.BatchDelete(t, batchDelete)
	dut.Config().RoutingPolicy().PolicyDefinition("TEST_ACCEPT").BatchDelete(t, batchDelete)
	dut.Config().RoutingPolicy().PolicyDefinition("TEST_REJECT").BatchDelete(t, batchDelete)
	defer batchDelete.Set(t)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/export-policy using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).ApplyPolicy().ExportPolicy()
			state := bgpState.Neighbor(neighbor_address).ApplyPolicy().ExportPolicy()
			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				for i, sg := range stateGot {
					if sg != input[i] {
						t.Errorf("Config /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/state/export-policy: got %v, want %v", sg, input[i])
					}
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedStringSlice) bool { return true }).Await(t); qs.IsPresent() {
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
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/config/default-export-policy using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).ApplyPolicy().DefaultExportPolicy()
			state := bgpState.Neighbor(neighbor_address).ApplyPolicy().DefaultExportPolicy()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/apply-policy/state/default-export-policy: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedE_RoutingPolicy_DefaultPolicyType) bool { return true }).Await(t); qs.IsPresent() {
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
		// false,
		true,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	baseConfig := baseBgpNeighborConfig(bgp_as)
	baseConfig.Neighbor[neighbor_address].GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	bgpConfig.Update(t, baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/config/enabled using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
			state := bgpState.Neighbor(neighbor_address).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/state/enabled: got %v, want %v", stateGot, input)
				}
			})
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

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	baseConfig := baseBgpNeighborConfig(bgp_as)
	baseConfig.Neighbor[neighbor_address].GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	bgpConfig.Update(t, baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().PrefixLimit().MaxPrefixes()
			state := bgpState.Neighbor(neighbor_address).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().PrefixLimit().MaxPrefixes()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/max-prefixes: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs := state.Lookup(t); qs.IsPresent() == true {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes fail: got %v", qs)
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
	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpConfig.Update(t, baseBgpNeighborConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/config/route-reflector-client using value %v", input), func(t *testing.T) {
			config := bgpConfig.Neighbor(neighbor_address).RouteReflector().RouteReflectorClient()
			state := bgpState.Neighbor(neighbor_address).RouteReflector().RouteReflectorClient()
			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/state/route-reflector-client: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedBool) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/neighbors/neighbor/route-reflector/config/route-reflector-client fail: got %v", qs)
				}
			})
		})
	}
}
