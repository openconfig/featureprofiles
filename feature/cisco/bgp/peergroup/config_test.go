package bgp_peergroup_test

import (
	"fmt"
	"testing"
	"time"

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
	telemetryTimeout time.Duration = 10 * time.Second
	configApplyTime  time.Duration = 5 * time.Second // FIXME: Workaround
	configDeleteTime time.Duration = 5 * time.Second // FIXME: Workaround
	dutName          string        = "dut"
	bgpInstance_0    string        = "TEST_PG"
	bgpAs_0          uint32        = 60000
	peerGroup        string        = "TestPeerGroup"
)

var index uint32 = 1

// Workaround for back-to-back BGP config and deletes, which XR fails on
func getNextBgpInstance() (string, uint32) {
	i := index
	index++
	return fmt.Sprintf("%s_%d", bgpInstance_0, i), i + bgpAs_0
}

func baseBgpPeerGroupConfig(bgpAs uint32) *oc.NetworkInstance_Protocol_Bgp {
	return &oc.NetworkInstance_Protocol_Bgp{
		Global: &oc.NetworkInstance_Protocol_Bgp_Global{
			As: ygot.Uint32(bgpAs),
		},
		PeerGroup: map[string]*oc.NetworkInstance_Protocol_Bgp_PeerGroup{
			peerGroup: {PeerGroupName: ygot.String(peerGroup)},
		},
	}
}

func cleanup(t *testing.T, dut *ondatra.DUTDevice, bgpInst string) {
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInst).Bgp().Config())
	time.Sleep(configDeleteTime)
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-group-name
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/peer-group-name
func TestPeerGroupName(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []string{
		"Peer-Group-1",
		// "100",
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()

	// Base config for BGP instance. Only run once.
	gnmi.Update(t, dut, bgpConfig.Global().As().Config(), bgp_as)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-group-name using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(input).PeerGroupName()
			state := bgpState.PeerGroup(input).PeerGroupName()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/peer-group-name: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, bgpConfig.PeerGroup(input).Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[string]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-group-name fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/peer-as
func TestPeerAs(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint32{
		1000,
		// 1469169603,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()

	// Base config for BGP instance. Only run once.
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).PeerAs()
			state := bgpState.PeerGroup(peerGroup).PeerAs()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/peer-as: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint32]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/peer-as fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/local-as
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/local-as
func TestLocalAs(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint32{
		// 10000,
		// 406134744,
		2182860066,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()

	// BGP Base config
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/local-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).LocalAs()
			state := bgpState.PeerGroup(peerGroup).LocalAs()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/local-as: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint32]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/local-as fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/remove-private-as
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/remove-private-as
func TestRemovePrivateAs(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop D2")

	dut := ondatra.DUT(t, dutName)

	inputs := []oc.E_Bgp_RemovePrivateAsOption{
		oc.Bgp_RemovePrivateAsOption_PRIVATE_AS_REMOVE_ALL,
		oc.Bgp_RemovePrivateAsOption_PRIVATE_AS_REPLACE_ALL,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()

	// BGP Base config
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/remove-private-as using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).RemovePrivateAs()
			state := bgpState.PeerGroup(peerGroup).RemovePrivateAs()
			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/state/remove-private-as: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[oc.E_Bgp_RemovePrivateAsOption]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/config/remove-private-as fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/connect-retry
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/connect-retry
func TestTimersConnectRetry(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop D2")

	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		30,
		//time.Minute.Seconds(),
		// (30 * time.Second).Seconds(),
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/connect-retry using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).Timers().ConnectRetry()
			state := bgpState.PeerGroup(peerGroup).Timers().ConnectRetry()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/connect-retry: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint16]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/connect-retry fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/hold-time
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/hold-time
func TestTimersHoldTime(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		60,
		360,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/hold-time using value %v", input), func(t *testing.T) {
			// holdTime Should be 3x keepAlive, see RFC 4271 - A Border Gateway Protocol 4, Sec. 10
			holdTime := input
			keepAlive := holdTime / 3
			Timers := &oc.NetworkInstance_Protocol_Bgp_PeerGroup_Timers{
				HoldTime:          ygot.Uint16(holdTime),
				KeepaliveInterval: ygot.Uint16(keepAlive),
			}
			config := bgpConfig.PeerGroup(peerGroup).Timers()
			state := bgpState.PeerGroup(peerGroup).Timers().HoldTime()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), Timers) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/hold-time: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint16]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/hold-time fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/keepalive-interval
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/keepalive-interval
func TestTimersKeepaliveInterval(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		60,
		360,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/keepalive-interval using value %v", input), func(t *testing.T) {
			// holdTime Should be 3x keepAlive, see RFC 4271 - A Border Gateway Protocol 4, Sec. 10
			holdTime := input
			keepAlive := holdTime / 3
			Timers := &oc.NetworkInstance_Protocol_Bgp_PeerGroup_Timers{
				HoldTime:          ygot.Uint16(holdTime),
				KeepaliveInterval: ygot.Uint16(keepAlive),
			}
			config := bgpConfig.PeerGroup(peerGroup).Timers()
			state := bgpState.PeerGroup(peerGroup).Timers().KeepaliveInterval()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), Timers) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != keepAlive {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/keepalive-interval: got %v, want %v", stateGot, keepAlive)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint16]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/keepalive-interval fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/minimum-advertisement-interval
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/minimum-advertisement-interval
func TestTimersMinimumAdvertisementInterval(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		50,
		43,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/minimum-advertisement-interval using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).Timers().MinimumAdvertisementInterval()
			state := bgpState.PeerGroup(peerGroup).Timers().MinimumAdvertisementInterval()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/state/minimum-advertisement-interval: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint16]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/timers/config/minimum-advertisement-interval fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/config/local-address
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/state/local-address
func TestTransportLocalAddress(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []string{
		// "10.11.12.13",
		"Bundle-Ether100",
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/config/local-address using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).Transport().LocalAddress()
			state := bgpState.PeerGroup(peerGroup).Transport().LocalAddress()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/state/local-address: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[string]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/transport/config/local-address fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/enabled
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/state/enabled
func TestGracefulRestartEnabled(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop D2")

	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		false,
		true,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/enabled using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).GracefulRestart().Enabled()
			state := bgpState.PeerGroup(peerGroup).GracefulRestart().Enabled()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/state/enabled: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[bool]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/enabled fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/restart-time
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/state/restart-time
func TestGracefulRestartRestartTime(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		// 2807,
		312,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/restart-time using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).GracefulRestart().RestartTime()
			state := bgpState.PeerGroup(peerGroup).GracefulRestart().RestartTime()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/state/restart-time: got %v, want %v", stateGot, input)
				}
			})
			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint16]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/restart-time fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/stale-routes-time
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/state/stale-routes-time
func TestGracefulRestartStaleRoutesTime(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint16{
		// 122429.24,
		10,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/stale-routes-time using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).GracefulRestart().StaleRoutesTime()
			state := bgpState.PeerGroup(peerGroup).GracefulRestart().StaleRoutesTime()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/stale-routes-time: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[uint16]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/stale-routes-time fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/helper-only
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/state/helper-only
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
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/helper-only using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).GracefulRestart().HelperOnly()
			state := bgpState.PeerGroup(peerGroup).GracefulRestart().HelperOnly()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/state/helper-only: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[bool]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/graceful-restart/config/helper-only fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/config/enabled
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/state/enabled
func TestAfiSafiEnabled(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		// false,
		true,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	baseConfig := baseBgpPeerGroupConfig(bgp_as)
	baseConfig.PeerGroup[peerGroup].GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	gnmi.Update(t, dut, bgpConfig.Config(), baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/config/enabled using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
			state := bgpState.PeerGroup(peerGroup).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/state/enabled: got %v, want %v", stateGot, input)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/max-prefixes
func TestAfiSafiMaxPrefixes(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint32{
		// 10,
		1000,
		// 234567,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	baseConfig := baseBgpPeerGroupConfig(bgp_as)
	baseConfig.PeerGroup[peerGroup].GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	gnmi.Update(t, dut, bgpConfig.Config(), baseConfig)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().PrefixLimit().MaxPrefixes()
			state := bgpState.PeerGroup(peerGroup).AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Ipv4Unicast().PrefixLimit().MaxPrefixes()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/prefix-limit/state/max-prefixes: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs := gnmi.Lookup(t, dut, state.State()); qs.IsPresent() == true {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/afi-safis/afi-safi/ipv4-unicast/prefix-limit/config/max-prefixes fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/route-reflector/config/route-reflector-client
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/route-reflector/state/route-reflector-client
func TestRouteReflectorClient(t *testing.T) {
	t.Skip("Not supported by XR - Planned drop TBD")

	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		false,
		true,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/route-reflector/config/route-reflector-client using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).RouteReflector().RouteReflectorClient()
			state := bgpState.PeerGroup(peerGroup).RouteReflector().RouteReflectorClient()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/route-reflector/state/route-reflector-client: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				if qs, _ := gnmi.Watch(t, dut, state.State(), telemetryTimeout, func(val *ygnmi.Value[bool]) bool { return true }).Await(t); qs.IsPresent() {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/route-reflector/config/route-reflector-client fail: got %v", qs)
				}
			})
		})
	}
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/error-handling/config/treat-as-withdraw
// State: /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/error-handling/state/treat-as-withdraw
func TestErrHndlTreatasWDR(t *testing.T) {

	dut := ondatra.DUT(t, dutName)

	inputs := []bool{
		true,
		false,
	}

	bgp_instance, bgp_as := getNextBgpInstance()
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
	gnmi.Update(t, dut, bgpConfig.Config(), baseBgpPeerGroupConfig(bgp_as))
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgp_instance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/error-handling/config/treat-as-withdraw using value %v", input), func(t *testing.T) {
			config := bgpConfig.PeerGroup(peerGroup).ErrorHandling().TreatAsWithdraw()
			state := bgpState.PeerGroup(peerGroup).ErrorHandling().TreatAsWithdraw()

			t.Run("TreatasWDR_Config", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)

			t.Run("TreatasWDR_State", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				fmt.Println("Check value stateGot", stateGot)
				fmt.Println("Check value input", input)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/error-handling/state/treat-as-withdraw: got %v, want %v", stateGot, input)
				}
			})

			t.Run("TreatasWDR_Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != false {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/peer-groups/peer-group/error-handling/state/treat-as-withdraw: got %v, want %v", stateGot, false)
				}
			})
		})
	}
}
