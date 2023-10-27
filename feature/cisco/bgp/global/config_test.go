package bgp_global_test

import (
	"fmt"
	"testing"
	"time"
        "context"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
        "github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
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
	bgpInstance      string        = "TEST"
	bgpAs            uint32        = 40000
)

func baseBgpGlobalConfig(bgpAs uint32) *oc.NetworkInstance_Protocol_Bgp {
	return &oc.NetworkInstance_Protocol_Bgp{
		Global: &oc.NetworkInstance_Protocol_Bgp_Global{
			As: ygot.Uint32(bgpAs),
		},
		/*Neighbor: map[string]*oc.NetworkInstance_Protocol_Bgp_Neighbor{
			neighbor_address: {
				NeighborAddress: ygot.String(neighbor_address),
				PeerAs:          ygot.Uint32(bgpAs + 100),
			},
		},*/
	}
}

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

func Test_Default_Metric(t *testing.T) {
        dut := ondatra.DUT(t, "dut")

        t.Log("Remove Flowspec Config")
        configToChange := "no flowspec \n"
        ctx := context.Background()
        util.GNMIWithText(ctx, t, dut, configToChange)
        t.Run("Testing openconfig-network-instance:network-instances/network-instance/protocols/protocol/config/default-metric", func(t *testing.T) {

                proto := oc.NetworkInstance_Protocol{}
                proto.DefaultMetric = ygot.Uint32(121)
                proto.Name = ygot.String("default")
                proto.Identifier =  oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP

                bgp_global := oc.NetworkInstance_Protocol_Bgp_Global{}
                bgp_global.As = ygot.Uint32(65000)

                bgp := oc.NetworkInstance_Protocol_Bgp{}
                bgp.Global = &bgp_global

                proto.Bgp = &bgp

                //ni := oc.NetworkInstance{}

                //key := oc.NetworkInstance_Protocol_Key{Identifier:oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, Name: "default"}

                t.Logf("TC: Configuring Default Metric for default vrf")
                t.Run("Update", func(t *testing.T) {
                        gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Config(), &proto)
                })

                proto.DefaultMetric = ygot.Uint32(144)
                t.Logf("TC: Configuring Default Metric for custom vrf - CISCO")
                t.Run("Update", func(t *testing.T) {
                        gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("CISCO").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Config(), &proto)
                })

// Get DEFAULT-METRIC

        t.Log("TC: Retrieve default-metric for DEFAULT vrf")
        t.Run("Get", func(t *testing.T) {
            config := gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").DefaultMetric()
            configGot := gnmi.GetConfig(t, dut, config.Config())
            t.Logf("Rcvd val - %v",configGot)


            expected_metric := ygot.Uint32(121)

            if configGot == *expected_metric {
                t.Logf("Passed expected metric")
            } else {
                t.Errorf("TestFAIL, Received %v Expected %v",configGot, *expected_metric)
            }

        })

        t.Log("TC: Retrieve default-metric for custom vrf - CISCO")
        t.Run("Get", func(t *testing.T) {
            config := gnmi.OC().NetworkInstance("CISCO").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").DefaultMetric()
            configGot := gnmi.GetConfig(t, dut, config.Config())
            t.Logf("Rcvd val - %v",configGot)


            expected_metric := ygot.Uint32(144)

            if configGot == *expected_metric {
                t.Logf("Passed expected metric")
            } else {
                t.Errorf("TestFAIL, Received %v Expected %v",configGot, *expected_metric)
            }

        })

        t.Run("Delete", func(t *testing.T) {
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("CISCO").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").DefaultMetric().Config())
		})

        t.Run("Delete", func(t *testing.T) {
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").DefaultMetric().Config())
		})


        })
}


// Config: /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/config/enabled
// State: /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/state/enabled
func TestGlobalAfiSafiUseMultiplePathsEnabled(t *testing.T) {
    dut := ondatra.DUT(t, dutName)
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, "\n")

    inputs := []bool{
        true,
        //false,
    }

    bgp_instance := bgpInstance
    bgp_as := bgpAs
    //getNextBgpInstance()
    bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
    bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
    gnmi.Update(t, dut, bgpConfig.Global().As().Config(), bgp_as)
    time.Sleep(configApplyTime)

    global_addr_family_config := bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
    t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, global_addr_family_config.Config(), true) })
    time.Sleep(configApplyTime)

    gnmi.Update(t, dut, bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).UseMultiplePaths().Ebgp().MaximumPaths().Config(), 5)
    gnmi.Update(t, dut, bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).UseMultiplePaths().Ibgp().MaximumPaths().Config(), 6)
    time.Sleep(configApplyTime)

    /*
    baseConfig := baseBgpGlobalConfig(bgp_as)
    afiSafiConfig := baseConfig.Global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
    umpConfig := afiSafiConfig.GetOrCreateUseMultiplePaths()
    umpConfig.GetOrCreateIbgp().SetMaximumPaths(5)
    umpConfig.GetOrCreateEbgp().SetMaximumPaths(6)
    */

    //defer cleanup(t, dut, bgp_instance)

    for _, input := range inputs {
        t.Run(fmt.Sprintf("Testing /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/config/enabled using value %v", input), func(t *testing.T) {

            fmt.Println("********************\n**********************")
            fmt.Println("                         AJ                 ")
            fmt.Println("********************\n**********************")
            t.Run("UseMultiplePathsEnabled_Config", func(t *testing.T) { gnmi.Update(t, dut, bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).UseMultiplePaths().Enabled().Config(), input) })

            time.Sleep(configApplyTime)

            t.Run("UseMultiplePathsEnabled_State", func(t *testing.T) {
                //stateGot := gnmi.Get(t, dut, bgpState.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).UseMultiplePaths().Enabled().State())
				stateGot := gnmi.Await(t, dut, bgpState.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).UseMultiplePaths().Enabled().State(), telemetryTimeout, false)
                fmt.Println("Check value stateGot", stateGot)
                fmt.Println("Check value input", input)
				value, _ := stateGot.Val()
                if value != input {
                    t.Errorf("State /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/config/enabled: got %v, want %v", stateGot, input)
                }
            })

            /*if input == false {
                t.Run("UseMultiplePathsEnabled_Delete", func(t *testing.T) {
                    gnmi.Delete(t, dut, config.Config())
                    gnmi.Delete(t, dut, bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).UseMultiplePaths().Enabled().Config())
                    time.Sleep(configDeleteTime)
                    stateGot1 := gnmi.Get(t, dut, state.State())
                    stateGot1 := gnmi.Get(t, dut, bgpState.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).UseMultiplePaths().Enabled().State())
                    if stateGot1 != false {
                        t.Errorf("Delete /network-instances/network-instance/protocols/protocol/bgp/global/afi-safis/afi-safi/use-multiple-paths/config/enabled: got %v, want %v", stateGot1, false)
                    }
                })
            }*/
        })
    }
}


func TestGlobalAfiSafiUseMultiplePathsEnabled1(t *testing.T) {
    dut := ondatra.DUT(t, dutName)
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, "\n")

    inputs := []bool{
        true,
        false,
    }

    bgp_instance := bgpInstance
    bgp_as := bgpAs

    bgpConfig := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()
    bgpState := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, bgp_instance).Bgp()

    gnmi.Update(t, dut, bgpConfig.Global().As().Config(), bgp_as)
    time.Sleep(configApplyTime)

    global_addr_family_config := bgpConfig.Global().AfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled()
    t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, global_addr_family_config.Config(), true) })
    time.Sleep(configApplyTime)

    bgp := baseBgpGlobalConfig(bgp_as)
    afiSafiConfig := bgp.Global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
    umpConfig := afiSafiConfig.GetOrCreateUseMultiplePaths()
    umpConfig.GetOrCreateIbgp().SetMaximumPaths(5)
    umpConfig.GetOrCreateEbgp().SetMaximumPaths(6)

    defer cleanup(t, dut, bgp_instance)

    for _, input := range inputs {
        umpConfig.SetEnabled(input)

        t.Run("Update", func(t *testing.T) { gnmi.Update(t, dut, bgpConfig.Config(), bgp) })
        time.Sleep(configApplyTime)

		t.Run("UseMultiplePathsEnabled_State", func(t *testing.T) {
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
func Test_Bgp_Global_RouteSelectionOptions_IgnoreNextHopIgpMetric(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, "\n")

	t.Log("Testing openconfig-network-instance:network-instances/network-instance/protocols/protocol/bgp/global/route-selection-options/config/ignore-next-hop-igp-metric \n")

	t.Run("Test", func(t *testing.T) {

		booleanVal := false
		routeselopt := oc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions {
			IgnoreNextHopIgpMetric: &booleanVal,
		}
		bgp := oc.NetworkInstance_Protocol_Bgp {
			Global: &oc.NetworkInstance_Protocol_Bgp_Global {
				As: ygot.Uint32(1),
			},
		}
		bgp.Global.RouteSelectionOptions = &routeselopt

		/*
		 * default VRF
		 */
		path := gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().RouteSelectionOptions()//.IgnoreNextHopIgpMetric()

		/*
		 * Add ignore-next-hop-igp-metric config
		 */
		t.Log("Update the ignore-next-hop-igp-metric = true")
		t.Run("Update", func(t *testing.T) {
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Config(), &bgp)
		})

		/*
		 * Replace ignore-next-hop-igp-metric value to true and verify
		 */
		t.Log("Replace ignore-next-hop-igp-metric value to true")
		t.Run("Replace", func(t *testing.T) {
			booleanVal = true
			routeselopt = oc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions {
				IgnoreNextHopIgpMetric: &booleanVal,
			}
			gnmi.Replace(t, dut, path.Config(), &routeselopt)
		})

		t.Log("Verify after replace")
		t.Run("Get", func(t *testing.T) {
			configGot := gnmi.GetConfig(t, dut, path.Config())

			if *configGot.IgnoreNextHopIgpMetric != *routeselopt.IgnoreNextHopIgpMetric {
				t.Errorf("Failed: Fetching leaf for ignore-next-hop-igp-metric got %v, want %v", *configGot.IgnoreNextHopIgpMetric, *routeselopt.IgnoreNextHopIgpMetric)
			} else {
				t.Logf("Passed: Configured ignore-next-hop-igp-metric = Obtained ignore-next-hop-igp-metric = %v", *configGot.IgnoreNextHopIgpMetric)
			}
		})

		/*
		 * Verify state for ignore-next-hop-igp-metric
		 */ 
		t.Log("Get-State for ignore-next-hop-igp-metric")
		t.Run("Get-State", func(t *testing.T) {
			stateGot := gnmi.Get(t, dut, path.State())
 
			if *stateGot.IgnoreNextHopIgpMetric != *routeselopt.IgnoreNextHopIgpMetric {
				t.Errorf("Failed: Fetching leaf for ignore-next-hop-igp-metric got %v, want %v", *stateGot.IgnoreNextHopIgpMetric, *routeselopt.IgnoreNextHopIgpMetric)
			} else {
				t.Logf("Passed: Configured ignore-next-hop-igp-metric = Obtained ignore-next-hop-igp-metric = %v", *stateGot.IgnoreNextHopIgpMetric)
			}
		})

		/*
		 * Delete Configuration
		 */
		t.Log("Delete Configuration")
		t.Run("Delete", func(t *testing.T) {
			gnmi.Delete(t, dut, path.Config())
		})

		/*
		 * VRF1 VRF
		 */
		 path = gnmi.OC().NetworkInstance("VRF1").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Global().RouteSelectionOptions()//.IgnoreNextHopIgpMetric()

		 /*
		  * Add ignore-next-hop-igp-metric config
		  */
		 t.Log("Update the ignore-next-hop-igp-metric = true")
		 t.Run("Update", func(t *testing.T) {
			 gnmi.Update(t, dut, gnmi.OC().NetworkInstance("VRF1").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "default").Bgp().Config(), &bgp)
		 })

		 /*
		  * Replace ignore-next-hop-igp-metric value to true and verify
		  */
		 t.Log("Replace ignore-next-hop-igp-metric value to true")
		 t.Run("Replace", func(t *testing.T) {
			 booleanVal = true
			 routeselopt = oc.NetworkInstance_Protocol_Bgp_Global_RouteSelectionOptions {
				 IgnoreNextHopIgpMetric: &booleanVal,
			 }
			 gnmi.Replace(t, dut, path.Config(), &routeselopt)
		 })

		 t.Log("Verify after replace")
		 t.Run("Get", func(t *testing.T) {
			 configGot := gnmi.GetConfig(t, dut, path.Config())

			 if *configGot.IgnoreNextHopIgpMetric != *routeselopt.IgnoreNextHopIgpMetric {
				 t.Errorf("Failed: Fetching leaf for ignore-next-hop-igp-metric got %v, want %v", *configGot.IgnoreNextHopIgpMetric, *routeselopt.IgnoreNextHopIgpMetric)
			 } else {
				 t.Logf("Passed: Configured ignore-next-hop-igp-metric = Obtained ignore-next-hop-igp-metric = %v", *configGot.IgnoreNextHopIgpMetric)
			 }
		 })

		 /*
		  * Verify state for ignore-next-hop-igp-metric
		  */ 
		 t.Log("Get-State for ignore-next-hop-igp-metric")
		 t.Run("Get-State", func(t *testing.T) {
			 stateGot := gnmi.Get(t, dut, path.State())
  
			 if *stateGot.IgnoreNextHopIgpMetric != *routeselopt.IgnoreNextHopIgpMetric {
				 t.Errorf("Failed: Fetching leaf for ignore-next-hop-igp-metric got %v, want %v", *stateGot.IgnoreNextHopIgpMetric, *routeselopt.IgnoreNextHopIgpMetric)
			 } else {
				 t.Logf("Passed: Configured ignore-next-hop-igp-metric = Obtained ignore-next-hop-igp-metric = %v", *stateGot.IgnoreNextHopIgpMetric)
			 }
		 })

		 /*
		  * Delete Configuration
		  */
		 t.Log("Delete Configuration")
		 t.Run("Delete", func(t *testing.T) {
			 gnmi.Delete(t, dut, path.Config())
		 })
	})
}
