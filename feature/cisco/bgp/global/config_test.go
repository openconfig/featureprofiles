package bgp_global_test

import (
	"fmt"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/featureprofiles/tools/cisco/yang_coverage"
	"github.com/openconfig/ondatra/eventlis"
)
var event = eventlis.EventListener{}

func TestMain(m *testing.M) {
	ws := "/nobackup/sanshety/ws/iosxr"
	models := []string{
		fmt.Sprintf("%s/manageability/yang/pyang/modules/openconfig-network-instance.yang", ws),
		fmt.Sprintf("%s/manageability/yang/pyang/modules/cisco-xr-openconfig-network-instance-deviations.yang", ws),
	}
	prefixPaths := []string{"/network-instances/network-instance/protocols/protocol/bgp/global",
	"/network-instances/network-instance/inter-instance-policies"}

	err := yang_coverage.CreateInstance("oc-sanity", models, prefixPaths, ws, event)
	fptest.RunTests(m)
	fmt.Println("end of main ", err)
}

const (
	telemetryTimeout time.Duration = 10 * time.Second
	configApplyTime  time.Duration = 5 * time.Second // FIXME: Workaround
	configDeleteTime time.Duration = 5 * time.Second // FIXME: Workaround
	dutName          string        = "dut"
	bgpInstance      string        = "TEST"
	bgpAs            uint32        = 40000
)

func cleanup(t *testing.T, dut *ondatra.DUTDevice, bgpInst string) {
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInst).Bgp().Config())
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
			bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInst).Bgp()
			bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInst).Bgp()
			config := bgpConfig.Global().As()
			state := bgpState.Global().As()

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)
			defer cleanup(t, dut, bgpInst)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
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

	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	config := bgpConfig.Global().RouterId()
	state := bgpState.Global().RouterId()

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id using value %v", input), func(t *testing.T) {
			gnmi.Update(t, dut, bgpConfig.Global().As().Config(), bgpAs)
			time.Sleep(configApplyTime)
			defer cleanup(t, dut, bgpInstance)

			t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, config.Config(), input) })
			time.Sleep(configApplyTime)
			defer cleanup(t, dut, bgpInstance)

			t.Run("Subscribe", func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/global/state/router-id: got %v, want %v", stateGot, input)
				}
			})

			t.Run("Delete", func(t *testing.T) {
				gnmi.Delete(t, dut, config.Config())
				time.Sleep(configDeleteTime)
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != "0.0.0.0" {
					t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id fail: got %v, want 0.0.0.0", stateGot)
				}
			})
		})
	}
}
