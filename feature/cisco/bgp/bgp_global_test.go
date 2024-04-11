package bgp_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	bgpInstanceGlobal string = "TEST"
	bgpAsGlobal       uint32 = 40000
)

func baseBgpGlobalConfig(bgpAs uint32) *oc.NetworkInstance_Protocol_Bgp {
	return &oc.NetworkInstance_Protocol_Bgp{
		Global: &oc.NetworkInstance_Protocol_Bgp_Global{
			As: ygot.Uint32(bgpAs),
		},
	}
}

// TestAs tests the configuration of the BGP global AS leaf
//
// Config: /network-instances/network-instance/protocols/protocol/bgp/global/config/as
// State: /network-instances/network-instance/protocols/protocol/bgp/global/state/as
func TestAs(t *testing.T) {
	dut := ondatra.DUT(t, dutName)
	inputs := []uint32{
		10,
		65534,
		12345678,
	}

	for _, input := range inputs {
		t.Logf("Testing /network-instances/network-instance/protocols/protocol/bgp/global/config/as using value %v", input)
		bgpInstance := fmt.Sprint(input)
		bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
		bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
		config := bgpConfig.Global().As()
		state := bgpState.Global().As()
		fixBgpLeafRefConstraints(t, dut, "DEFAULT")
		t.Run(fmt.Sprintf("Update AS number using value %v", input), func(t *testing.T) {
			gnmi.Update(t, dut, config.Config(), input)
			time.Sleep(configApplyTime)
			defer cleanup(t, dut, bgpInstance)

			t.Run(fmt.Sprintf("Subscribe to AS number %v", input), func(t *testing.T) {
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
		"195.3.253.50",
	}

	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstanceGlobal).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstanceGlobal).Bgp()
	fixBgpLeafRefConstraints(t, dut, bgpInstanceGlobal)
	gnmi.Update(t, dut, bgpConfig.Global().As().Config(), bgpAsGlobal)
	time.Sleep(configApplyTime)
	defer cleanup(t, dut, bgpInstanceGlobal)

	config := bgpConfig.Global().RouterId()
	state := bgpState.Global().RouterId()

	for _, input := range inputs {
		t.Logf("Testing /network-instances/network-instance/protocols/protocol/bgp/global/config/router-id using value %v", input)

		t.Run(fmt.Sprintf("Update router-id using value %v", input), func(t *testing.T) {
			gnmi.Update(t, dut, config.Config(), input)
			//time.Sleep(configApplyTime)
			defer cleanup(t, dut, bgpInstanceGlobal)

			t.Run(fmt.Sprintf("Subscribe to router-id of value %v", input), func(t *testing.T) {
				stateGot := gnmi.Get(t, dut, state.State())
				if stateGot != input {
					t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/global/state/router-id: got %v, want %v", stateGot, input)
				}
			})
		})
	}
}

func TestDefaultMetric(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Log("Remove Flowspec Config")
	util.GNMIWithText(context.Background(), t, dut, "no flowspec \n")
	proto := oc.NetworkInstance_Protocol{}
	proto.DefaultMetric = ygot.Uint32(121)
	proto.Name = ygot.String("default")
	proto.Identifier = oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP

	bgpGlobal := oc.NetworkInstance_Protocol_Bgp_Global{}
	bgpGlobal.As = ygot.Uint32(65000)

	bgp := oc.NetworkInstance_Protocol_Bgp{}
	bgp.Global = &bgpGlobal

	proto.Bgp = &bgp
	t.Logf("TC: Configuring Default Metric for default vrf")
	t.Run("Update default metric for default vrf", func(t *testing.T) {
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Config(), &proto)

		t.Log("TC: Retrieve default-metric for default vrf")
		t.Run("Get default-metric for default vrf", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").DefaultMetric()
			configGot, pres := gnmi.LookupConfig(t, dut, config.Config()).Val()
			expectedMetric := ygot.Uint32(121)
			if !pres {
				t.Errorf("default-metric value not present for default vrf")
			} else if configGot != *expectedMetric {
				t.Errorf("Received default metric value: %v, Expected: %v", configGot, *expectedMetric)
			}
		})
	})

	t.Logf("TC: Configuring Default Metric for custom vrf - CISCO")
	t.Run("Update default metric for custom vrf", func(t *testing.T) {
		proto.DefaultMetric = ygot.Uint32(144)
		// update name inside network-instance Config container to satisfy leafref constraint on the list key
		gnmi.Update(t, dut, gnmi.OC().NetworkInstance("CISCO").Name().Config(), "CISCO")
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("CISCO").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Config(), &proto)

		t.Log("TC: Retrieve default-metric for custom vrf - CISCO")
		t.Run("Get default-metric for custom vrf", func(t *testing.T) {
			config := gnmi.OC().NetworkInstance("CISCO").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").DefaultMetric()
			configGot, pres := gnmi.LookupConfig(t, dut, config.Config()).Val()
			expectedMetric := ygot.Uint32(144)
			if !pres {
				t.Errorf("default-metric value not present for default vrf")
			} else if configGot != *expectedMetric {
				t.Errorf("Received default metric value: %v, Expected: %v", configGot, *expectedMetric)
			}
		})
	})
	t.Logf("TC: Deleting Default Metric for default vrf")
	t.Run("Delete default metric for default vrf", func(t *testing.T) {
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").DefaultMetric().Config())
	})

	t.Logf("TC: Deleting Default Metric for custom vrf - CISCO")
	t.Run("Delete default metric for custom vrf", func(t *testing.T) {
		gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("CISCO").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").DefaultMetric().Config())
	})
}

// Config: /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/config/enabled
// State: /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/state/enabled
func TestGlobalAfiSafiUseMultiplePathsEnabled(t *testing.T) {
	dut := ondatra.DUT(t, dutName)
	inputs := []bool{
		true,
		false,
	}

	bgpInstance := bgpInstanceGlobal
	bgpAs := bgpAsGlobal
	bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgpInstance).Bgp()
	fixBgpLeafRefConstraints(t, dut, bgpInstance)
	gnmi.Update(t, dut, bgpConfig.Global().As().Config(), bgpAs)

	globalAddrFamilyConfig := bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
	// satisfy leaf-ref validation constraints
	gnmi.Update(t, dut, bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).AfiSafiName().Config(), oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	gnmi.Update(t, dut, globalAddrFamilyConfig.Config(), true)

	bgp := baseBgpGlobalConfig(bgpAs)
	afiSafiConfig := bgp.Global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	umpConfig := afiSafiConfig.GetOrCreateUseMultiplePaths()
	umpConfig.GetOrCreateIbgp().SetMaximumPaths(5)
	umpConfig.GetOrCreateEbgp().SetMaximumPaths(6)
	gnmi.Update(t, dut, bgpConfig.Config(), bgp)
	defer cleanup(t, dut, bgpInstance)

	for _, input := range inputs {
		t.Run(fmt.Sprintf("Update enabled leaf under multiple-paths container with value %v", input), func(t *testing.T) {
			umpConfig.SetEnabled(input)
		})

		t.Run(fmt.Sprintf("Get enabled leaf under multiple-paths container of value %v", input), func(t *testing.T) {
			// https://wwwin-opengrok.cisco.com/xr-dev/xref/manageability/yang/pyang/modules/cisco-xr-openconfig-network-instance-deviations.yang
			t.Skipf("Not supported as of 11 April 2024")
			stateGot := gnmi.Get(t, dut, bgpState.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).UseMultiplePaths().Enabled().State())
			if stateGot != input {
				t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/config/enabled: got %v, want %v", stateGot, input)
			}
		})
	}
}

// Test_Bgp_Global_RouteSelectionOptions_IgnoreNextHopIgpMetric tests the configuration of the BGP global ignore-next-hop-igp-metric leaf
//
// Config: /network-instances/network-instance/protocols/protocol/bgp/global/route-selection-options/config/ignore-next-hop-igp-metric
// State: /network-instances/network-instance/protocols/protocol/bgp/global/route-selection-options/state/ignore-next-hop-igp-metric

//func Test_Bgp_Global_RouteSelectionOptions_IgnoreNextHopIgpMetric(t *testing.T) {
//	dut := ondatra.DUT(t, "dut")
//
//	t.Log("Testing openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/global/route-selection-options/config/ignore-next-hop-igp-metric \n")
//	t.Run("Test", func(t *testing.T) {
//
//		booleanVal := true
//		routeselopt := oc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions{
//			IgnoreNextHopIgpMetric: &booleanVal,
//		}
//		bgp := oc.NetworkInstance_Protocol_Bgp{
//			Global: &oc.NetworkInstance_Protocol_Bgp_Global{
//				As: ygot.Uint32(bgpAsGlobal),
//			},
//		}
//		bgp.Global.RouteSelectionOptions = &routeselopt
//
//		/*
//		 * default VRF
//		 */
//		path := gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().RouteSelectionOptions()
//
//		/*
//		 * Add ignore-next-hop-igp-metric config
//		 */
//		t.Log("Update the ignore-next-hop-igp-metric = true")
//		t.Run("Update", func(t *testing.T) {
//			gnmi.Update(t, dut, path.Config(), &routeselopt)
//		})
//
//		t.Log("Verify after update")
//		t.Run("Get", func(t *testing.T) {
//			configGot := gnmi.Get(t, dut, path.Config())
//
//			if *configGot.IgnoreNextHopIgpMetric != *routeselopt.IgnoreNextHopIgpMetric {
//				t.Errorf("Failed: Fetching leaf for ignore-next-hop-igp-metric got %v, want %v", *configGot.IgnoreNextHopIgpMetric, *routeselopt.IgnoreNextHopIgpMetric)
//			} else {
//				t.Logf("Passed: Configured ignore-next-hop-igp-metric = Obtained ignore-next-hop-igp-metric = %v", *configGot.IgnoreNextHopIgpMetric)
//			}
//		})
//
//		/*
//		 * Replace ignore-next-hop-igp-metric value to true and verify
//		 */
//		t.Log("Replace ignore-next-hop-igp-metric value to false")
//		t.Run("Replace", func(t *testing.T) {
//			booleanVal = false
//			routeselopt = oc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions{
//				IgnoreNextHopIgpMetric: &booleanVal,
//			}
//			gnmi.Replace(t, dut, path.Config(), &routeselopt)
//		})
//
//		t.Log("Verify after replace")
//		t.Run("Get", func(t *testing.T) {
//			configGot := gnmi.Get(t, dut, path.Config())
//
//			if *configGot.IgnoreNextHopIgpMetric != *routeselopt.IgnoreNextHopIgpMetric {
//				t.Errorf("Failed: Fetching leaf for ignore-next-hop-igp-metric got %v, want %v", *configGot.IgnoreNextHopIgpMetric, *routeselopt.IgnoreNextHopIgpMetric)
//			} else {
//				t.Logf("Passed: Configured ignore-next-hop-igp-metric = Obtained ignore-next-hop-igp-metric = %v", *configGot.IgnoreNextHopIgpMetric)
//			}
//		})
//
//		/*
//		 * Verify state for ignore-next-hop-igp-metric
//		 */
//		t.Log("Get-State for ignore-next-hop-igp-metric")
//		t.Run("Get-State", func(t *testing.T) {
//			stateGot := gnmi.Get(t, dut, path.State())
//
//			if *stateGot.IgnoreNextHopIgpMetric != *routeselopt.IgnoreNextHopIgpMetric {
//				t.Errorf("Failed: Fetching leaf for ignore-next-hop-igp-metric got %v, want %v", *stateGot.IgnoreNextHopIgpMetric, *routeselopt.IgnoreNextHopIgpMetric)
//			} else {
//				t.Logf("Passed: Configured ignore-next-hop-igp-metric = Obtained ignore-next-hop-igp-metric = %v", *stateGot.IgnoreNextHopIgpMetric)
//			}
//		})
//
//		/*
//		 * Delete Configuration
//		 */
//		t.Log("Delete Configuration")
//		t.Run("Delete", func(t *testing.T) {
//			gnmi.Delete(t, dut, path.Config())
//		})
//
//		/*
//		 * VRF
//		 */
//		booleanVal = true
//		routeselopt = oc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions{
//			IgnoreNextHopIgpMetric: &booleanVal,
//		}
//		path = gnmi.OC().NetworkInstance("CISCO").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().RouteSelectionOptions()
//
//		/*
//		 * Add ignore-next-hop-igp-metric config
//		 */
//		t.Log("Update the ignore-next-hop-igp-metric = true")
//		t.Run("Update", func(t *testing.T) {
//			gnmi.Update(t, dut, path.Config(), &routeselopt)
//		})
//
//		t.Log("Verify after update")
//		t.Run("Get", func(t *testing.T) {
//			configGot := gnmi.Get(t, dut, path.Config())
//
//			if *configGot.IgnoreNextHopIgpMetric != *routeselopt.IgnoreNextHopIgpMetric {
//				t.Errorf("Failed: Fetching leaf for ignore-next-hop-igp-metric got %v, want %v", *configGot.IgnoreNextHopIgpMetric, *routeselopt.IgnoreNextHopIgpMetric)
//			} else {
//				t.Logf("Passed: Configured ignore-next-hop-igp-metric = Obtained ignore-next-hop-igp-metric = %v", *configGot.IgnoreNextHopIgpMetric)
//			}
//		})
//
//		/*
//		 * Replace ignore-next-hop-igp-metric value to true and verify
//		 */
//		t.Log("Replace ignore-next-hop-igp-metric value to false")
//		t.Run("Replace", func(t *testing.T) {
//			booleanVal = false
//			routeselopt = oc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions{
//				IgnoreNextHopIgpMetric: &booleanVal,
//			}
//			gnmi.Replace(t, dut, path.Config(), &routeselopt)
//		})
//
//		t.Log("Verify after replace")
//		t.Run("Get", func(t *testing.T) {
//			configGot := gnmi.Get(t, dut, path.Config())
//
//			if *configGot.IgnoreNextHopIgpMetric != *routeselopt.IgnoreNextHopIgpMetric {
//				t.Errorf("Failed: Fetching leaf for ignore-next-hop-igp-metric got %v, want %v", *configGot.IgnoreNextHopIgpMetric, *routeselopt.IgnoreNextHopIgpMetric)
//			} else {
//				t.Logf("Passed: Configured ignore-next-hop-igp-metric = Obtained ignore-next-hop-igp-metric = %v", *configGot.IgnoreNextHopIgpMetric)
//			}
//		})
//
//		/*
//		 * Verify state for ignore-next-hop-igp-metric
//		 */
//		t.Log("Get-State for ignore-next-hop-igp-metric")
//		t.Run("Get-State", func(t *testing.T) {
//			stateGot := gnmi.Get(t, dut, path.State())
//
//			if *stateGot.IgnoreNextHopIgpMetric != *routeselopt.IgnoreNextHopIgpMetric {
//				t.Errorf("Failed: Fetching leaf for ignore-next-hop-igp-metric got %v, want %v", *stateGot.IgnoreNextHopIgpMetric, *routeselopt.IgnoreNextHopIgpMetric)
//			} else {
//				t.Logf("Passed: Configured ignore-next-hop-igp-metric = Obtained ignore-next-hop-igp-metric = %v", *stateGot.IgnoreNextHopIgpMetric)
//			}
//		})
//
//		/*
//		 * Delete Configuration
//		 */
//		t.Log("Delete Configuration")
//		t.Run("Delete", func(t *testing.T) {
//			gnmi.Delete(t, dut, path.Config())
//		})
//	})
//}
