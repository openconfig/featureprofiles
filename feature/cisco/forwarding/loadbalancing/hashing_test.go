// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package setup is scoped only to be used for scripts in path
// feature/experimental/system/gnmi/benchmarking/ate_tests/
// Do not use elsewhere.
package hashing

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"

	"github.com/openconfig/featureprofiles/internal/attrs"

	// "github.com/openconfig/featureprofiles/internal/cisco/verifiers"

	"github.com/openconfig/featureprofiles/internal/cisco/helper"
	"github.com/openconfig/featureprofiles/internal/cisco/verifiers"

	"github.com/openconfig/ondatra"
	// "github.com/openconfig/featureprofiles/internal/cisco/util"
)

func TestWANLinksRoutedLoadBalancing(t *testing.T) {
	// DUT var for R, E, V sites and Jupiter cluster nodes
	dut1R := ondatra.DUT(t, "dut1R")
	dut2R := ondatra.DUT(t, "dut2R")
	dut3R := ondatra.DUT(t, "dut3R")
	dut4R := ondatra.DUT(t, "dut4R")
	dut1E := ondatra.DUT(t, "dut1E")
	dut2E := ondatra.DUT(t, "dut2E")
	dut1V := ondatra.DUT(t, "dut1V")
	dut2V := ondatra.DUT(t, "dut2V")
	dutJupiterE := ondatra.DUT(t, "JupiterR")
	dutJupiterR := ondatra.DUT(t, "JupiterE")
	//DUT list for different site groupings
	siteRDUTList := []*ondatra.DUTDevice{dut1R, dut2R, dut3R, dut4R}
	siteEDUTList := []*ondatra.DUTDevice{dut1E, dut2E}
	siteVDUTList := []*ondatra.DUTDevice{dut1V, dut2V}
	jupiterDUTList := []*ondatra.DUTDevice{dutJupiterE, dutJupiterR}
	//Just to use variable and compile
	t.Log("R,E,V and Jupiter sites", siteRDUTList, siteEDUTList, siteVDUTList, jupiterDUTList)

	dvtCiscoDUTList := []*ondatra.DUTDevice{dut1R, dut2R, dut3R, dut4R, dut1E, dut2E}

	tgenParam := helper.TgenConfigParam{
		DutIntfAttr:      []attrs.Attributes{dutPort1, dutPort2},
		TgenIntfAttr:     []attrs.Attributes{atePort1, atePort2},
		TgenPortList:     []*ondatra.Port{ondatra.ATE(t, "ate").Port(t, "port1"), ondatra.ATE(t, "ate").Port(t, "port2")},
		TrafficFlowParam: []*helper.TrafficFlowAttr{&IPinIPE2R, &IPinIPR2E},
	}

	t.Log("Configure TGEN and set traffic flows")
	topo := helper.TGENHelper().ConfigureTGEN(false, &tgenParam).ConfigureTgenInterface(t)

	trafficFlows := helper.TGENHelper().ConfigureTGEN(false, &tgenParam).ConfigureTGENFlows(t)
	tgenVerifyParam := verifiers.TgenValidationParam{
		Tolerance: 0.02,
		WantLoss:  false,
		Flows:     trafficFlows,
	}
	t.Run("Verify Traffic passes after init Bringup", func(t *testing.T) {
		helper.TGENHelper().StartTraffic(t, false, trafficFlows, 10*time.Second, topo, false)
		time.Sleep(5 * time.Second) // Wait for tgen traffic to completely stop.
		verifiers.Tgen.ValidateTGEN(false, &tgenVerifyParam).ValidateTrafficLoss(t)
	})
	cases := []testCase{
		// {
		// 	name: "Default",
		// 	desc: "Default Hash parameters",
		// },
		{
			name:                  "Both auto-global",
			desc:                  "Auto-global Hash parameters for both Extended Entropy and Algorithm Adjust options",
			extendedEntropyOption: &extendedEntropyCLIOptions{perChassis: true},
			algorithmAdjustOption: &algorithmAdjustCLIOptions{perChassis: true},
			confHashCLIdutList:    dvtCiscoDUTList,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {

			// Configure extended entropy and algorithm adjust CLI options as per the test case
			if tt.extendedEntropyOption != nil || tt.algorithmAdjustOption != nil {
				configureHashCLIOptions(t, tt.extendedEntropyOption, tt.algorithmAdjustOption, tt.confHashCLIdutList, false)
			}
			defer configureHashCLIOptions(t, tt.extendedEntropyOption, tt.algorithmAdjustOption, tt.confHashCLIdutList, true)

			t.Log("Clearing interface counters on all the DUTs")
			helper.InterfaceHelper().ClearInterfaceCountersAll(t, dvtCiscoDUTList)
			v4E2R.TrafficPPS = 200
			v4R2E.TrafficPPS = 200
			trafficMap := make(map[string][]*helper.TrafficFlowAttr)
			trafficMap["v4"] = []*helper.TrafficFlowAttr{&v4R2E, &v4E2R}
			trafficMap["v6"] = []*helper.TrafficFlowAttr{&v6R2E, &v6E2R}
			for trafficType, trafficList := range trafficMap {
				t.Run(fmt.Sprintf("Test %s Traffic", trafficType), func(t *testing.T) {
				})
				tgenParam.TrafficFlowParam = trafficList
				trafficFlow := helper.TGENHelper().ConfigureTGEN(false, &tgenParam).ConfigureTGENFlows(t)
				t.Log("Start Bidirectional Traffic flows")
				helper.TGENHelper().StartTraffic(t, false, trafficFlow, 2*time.Minute, topo, false)
				time.Sleep(30 * time.Second) // Wait for 30 seconds for XR statsd interface cache to update
				t.Log(trafficType + " traffic started")
				t.Log("Measure Traffic distribution from Site R-to-E on SiteE node going to Jupiter , & other way around")
				var v4DstPrefix, v6DstPrefix string
				for _, device := range tt.confHashCLIdutList {
					fmt.Printf(tt.name+" for device: %s", device.Name())
					if strings.Contains(device.Name(), "E") {
						v4DstPrefix = eSiteV4DSTIP
						v6DstPrefix = eSiteV6DSTIP
					} else {
						v4DstPrefix = rSiteV6DSTIP
						v6DstPrefix = rSiteV4DSTIP
					}
					t.Logf("Get AFT Prefix objects for %s", v4DstPrefix)
					if trafficType == "v6" {
						t.Log("Get AFT Prefix objects for " + v6DstPrefix)
					}
					aftPfxObj := helper.FIBHelper().GetPrefixAFTObjects(t, device, v4DstPrefix, deviations.DefaultNetworkInstance(device))
					bundleObjList := []BundleInterface{}

					for _, nhObj := range aftPfxObj.NextHop {
						bundleObj := BundleInterface{}
						bundleObj.BundleInterfaceName = nhObj.NextHopInterface
						bundleObj.BundleNHWeight = nhObj.NextHopWeight
						memberMap := helper.InterfaceHelper().GetBundleMembers(t, device, nhObj.NextHopInterface)
						for _, memberList := range memberMap {
							bundleMemberWt := make([]uint64, len(memberList))
							bundleObj.BundleMembers = memberList
							for i := 0; i < len(memberList); i++ {
								bundleMemberWt[i] = 1 // Default weight for Bundle members is 1
							}
							bundleObj.BundleMembersWeight = bundleMemberWt
						}
						bundleObjList = append(bundleObjList, bundleObj)
					}
					// Create Map of Bundle NH Outgoing interfaces with their weights
					var OutputIFWeight = make(map[string]uint64)
					for _, nhObj := range aftPfxObj.NextHop {
						OutputIFWeight[nhObj.NextHopInterface] = nhObj.NextHopWeight
					}

					inputTrafficIF := helper.LoadbalancingHelper().GetIngressTrafficInterfaces(t, device, "ipv4", true)
					var bundleNHIntf []string
					for _, intf := range bundleObjList {
						bundleNHIntf = append(bundleNHIntf, intf.BundleInterfaceName)
					}

					//Remove NH Outgoing Bundle interfaces from inputTrafficIF MAP.
					for _, intf := range bundleNHIntf {
						delete(inputTrafficIF, intf)
					}
					var totalInPackets uint64
					for _, val := range inputTrafficIF {
						totalInPackets += val
					}
					t.Logf("TotalInPackets on dut %s are: %d", device, totalInPackets)

					t.Run(fmt.Sprintf("Verify Bundle NH BGP recursive level loadbalancing on device %s", device.Name()), func(t *testing.T) {
						verifiers.Loadbalancing.VerifyEgressDistributionPerWeight(t, device, OutputIFWeight, loadBalancingTolerance, true, v4TrafficType)
					})
					for _, bunIntf := range bundleObjList {
						var memberListWeight = make(map[string]uint64)
						for _, member := range bunIntf.BundleMembers {
							memberListWeight[member] = 1
						}
						t.Run(fmt.Sprintf("Verify Bundle member LAG level loadbalancing on device %s on Bundle %s", device.Name(), bunIntf.BundleInterfaceName), func(t *testing.T) {
							verifiers.Loadbalancing.VerifyEgressDistributionPerWeight(t, device, memberListWeight, loadBalancingTolerance, false, noTrafficType)
						})
					}
				}
			}
		})
	}
}
