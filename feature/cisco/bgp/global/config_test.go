package bgp_global_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	oc "github.com/openconfig/ondatra/telemetry"
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
	bgpInstance      string        = "TEST"
	bgpAs            uint32        = 40000
)

func cleanup(t *testing.T, dut *ondatra.DUTDevice, bgpInst string) {
	dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInst).Bgp().Delete(t)
	time.Sleep(configDeleteTime)
}

// TestAs tests the configuration of the BGP global AS leaf
//
// Config: /network-instances/network-instance/protocols/protocol/bgp/global/config/as
// State: /network-instances/network-instance/protocols/protocol/bgp/global/state/as
func TestAs(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []uint32{
		// 10,
		// 65535,
		12345678,
	}

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/global/config/as using value %v", input), func(t *testing.T) {
			bgpInst := fmt.Sprint(input)
			bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInst).Bgp()
			bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInst).Bgp()
			config := bgpConfig.Global().As()
			state := bgpState.Global().As()

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)
			defer cleanup(t, dut, bgpInst)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/global/state/as: got %v, want %v", stateGot, input)
				}
			})
		})
	}
}

// TestRouterId tests the configuration of the BGP global router-id leaf
//
// Config: /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id
// State: /network-instances/network-instance/protocols/protocol/bgp/global/state/router-id
func TestRouterId(t *testing.T) {
	dut := ondatra.DUT(t, dutName)

	inputs := []string{
		// "4.134.130.98",
		"195.3.253.50",
	}

	bgpConfig := dut.Config().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := dut.Telemetry().NetworkInstance(networkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	config := bgpConfig.Global().RouterId()
	state := bgpState.Global().RouterId()

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id using value %v", input), func(t *testing.T) {
			bgpConfig.Global().As().Update(t, bgpAs)
			time.Sleep(configApplyTime)
			defer cleanup(t, dut, bgpInstance)

			t.Run("Update", func(t *testing.T) { config.Update(t, input) })
			time.Sleep(configApplyTime)
			defer cleanup(t, dut, bgpInstance)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := state.Get(t)
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/global/state/router-id: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				config.Delete(t)
				time.Sleep(configDeleteTime)
				if qs, _ := state.Watch(t, telemetryTimeout, func(val *oc.QualifiedString) bool { return true }).Await(t); qs.IsPresent() { // FIXME: qs.Val(t) != 0.0.0.0
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id fail: got %v", qs)
				}
			})
		})
	}
}
