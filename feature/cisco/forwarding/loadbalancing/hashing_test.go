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
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/helper"
	"github.com/openconfig/featureprofiles/internal/cisco/verifiers"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
)

// per device gribiParamPerSite param.
var (
	encapVRFAV4SiteE = "10.240.118.32/28"
	encapVRFAV4SiteR = "10.240.119.48/28"
	encapVRFAV6SiteE = "2002:af0:7730::/44"
	encapVRFAV6SiteR = "2002:af0:7620::/44"
	gribi1R          = gribiParamPerSite{
		encapV4Prefix:     encapVRFAV4SiteE,
		encapV6Prefix:     encapVRFAV6SiteR,
		encapTunnelIP1:    "98.2.0.0",
		encapTunnelIP2:    "98.2.0.1",
		decapV4Prefix:     "98.1.0.0/20",
		nextSiteVIPs:      []string{"10.41.164.3", "10.41.164.1"},
		selfSiteVIPs:      []string{"10.41.164.0"},
		nextSiteIntfCount: 2,
		selfSiteIntfCount: 1,
		nextSite1VIPNH: []map[string]string{
			{"Bundle-Ether4": "169.254.0.8"},
			{"Bundle-Ether5": "169.254.0.10"},
		},
		nextSite2VIPNH: []map[string]string{
			{"Bundle-Ether10": "169.254.0.20"},
			{"Bundle-Ether11": "169.254.0.22"},
		},
		selfSiteVIPNH: []map[string]string{
			{"Bundle-Ether1": "169.254.0.2"},
			{"Bundle-Ether2": "169.254.0.4"},
			{"Bundle-Ether3": "169.254.0.6"},
		},
	}
	gribi2R = gribiParamPerSite{
		encapV4Prefix:     encapVRFAV4SiteE,
		encapV6Prefix:     encapVRFAV6SiteR,
		encapTunnelIP1:    "98.2.0.0",
		encapTunnelIP2:    "98.2.0.1",
		decapV4Prefix:     "98.1.0.0/20",
		nextSiteVIPs:      []string{"10.41.164.3", "10.41.164.3"},
		selfSiteVIPs:      []string{"10.41.164.0"},
		nextSiteIntfCount: 1,
		selfSiteIntfCount: 0,
		nextSite1VIPNH: []map[string]string{
			{"Bundle-Ether4": "169.254.0.8"},
			{"Bundle-Ether5": "169.254.0.10"},
		},
		nextSite2VIPNH: []map[string]string{
			{"Bundle-Ether7": "169.254.0.14"},
			{"Bundle-Ether9": "169.254.0.18"},
		},
		selfSiteVIPNH: []map[string]string{
			{"Bundle-Ether1": "169.254.0.2"},
			{"Bundle-Ether2": "169.254.0.4"},
			{"Bundle-Ether3": "169.254.0.6"},
		},
	}
	gribi3R = gribiParamPerSite{
		encapV4Prefix:     encapVRFAV4SiteE,
		encapV6Prefix:     encapVRFAV6SiteR,
		encapTunnelIP1:    "98.2.0.0",
		encapTunnelIP2:    "98.2.0.1",
		decapV4Prefix:     "98.1.0.0/20",
		nextSiteVIPs:      []string{"10.41.164.3", "10.41.164.3"},
		selfSiteVIPs:      []string{"10.41.164.0"},
		nextSiteIntfCount: 1,
		selfSiteIntfCount: 0,
		nextSite1VIPNH: []map[string]string{
			{"Bundle-Ether4": "169.254.0.8"},
			{"Bundle-Ether5": "169.254.0.10"},
		},
		nextSite2VIPNH: []map[string]string{
			{"Bundle-Ether8": "169.254.0.16"},
			{"Bundle-Ether9": "169.254.0.18"},
		},
		selfSiteVIPNH: []map[string]string{
			{"Bundle-Ether1": "169.254.0.2"},
			{"Bundle-Ether2": "169.254.0.4"},
			{"Bundle-Ether3": "169.254.0.6"},
		},
	}
	gribi4R = gribiParamPerSite{
		encapV4Prefix:     encapVRFAV4SiteE,
		encapV6Prefix:     encapVRFAV6SiteR,
		encapTunnelIP1:    "98.2.0.0",
		encapTunnelIP2:    "98.2.0.1",
		decapV4Prefix:     "98.1.0.0/20",
		nextSiteVIPs:      []string{"10.41.164.3", "10.41.164.3"},
		selfSiteVIPs:      []string{"10.41.164.0"},
		nextSiteIntfCount: 1,
		selfSiteIntfCount: 0,
		nextSite1VIPNH: []map[string]string{
			{"Bundle-Ether4": "169.254.0.8"},
			{"Bundle-Ether5": "169.254.0.10"},
		},
		nextSite2VIPNH: []map[string]string{
			{"Bundle-Ether8": "169.254.0.16"},
			{"Bundle-Ether9": "169.254.0.18"},
		},
		selfSiteVIPNH: []map[string]string{
			{"Bundle-Ether1": "169.254.0.2"},
			{"Bundle-Ether2": "169.254.0.4"},
			{"Bundle-Ether3": "169.254.0.6"},
		},
	}
	gribi1E = gribiParamPerSite{
		encapV4Prefix:     encapVRFAV4SiteR,
		encapV6Prefix:     encapVRFAV6SiteE,
		encapTunnelIP1:    "98.1.0.0",
		encapTunnelIP2:    "98.1.0.1",
		decapV4Prefix:     "98.2.0.0/20",
		nextSiteVIPs:      []string{"10.41.164.1", "10.41.164.3"},
		selfSiteVIPs:      []string{"10.41.164.0"},
		nextSiteIntfCount: 2,
		selfSiteIntfCount: 1,
		nextSite1VIPNH: []map[string]string{
			{"Bundle-Ether2": "169.254.0.4"},
			{"Bundle-Ether3": "169.254.0.6"},
			{"Bundle-Ether4": "169.254.0.8"},
			{"Bundle-Ether5": "169.254.0.10"},
		},
		nextSite2VIPNH: []map[string]string{
			{"Bundle-Ether10": "169.254.0.20"},
			{"Bundle-Ether11": "169.254.0.22"},
			{"Bundle-Ether56": "169.254.0.112"},
			{"Bundle-Ether57": "169.254.0.114"},
		},
		selfSiteVIPNH: []map[string]string{
			{"Bundle-Ether1": "169.254.0.2"},
		},
	}
	gribi2E = gribiParamPerSite{
		encapV4Prefix:     encapVRFAV4SiteR,
		encapV6Prefix:     encapVRFAV6SiteE,
		encapTunnelIP1:    "98.1.0.0",
		encapTunnelIP2:    "98.1.0.1",
		decapV4Prefix:     "98.2.0.0/20",
		nextSiteVIPs:      []string{"10.41.164.1", "10.41.164.3"},
		selfSiteVIPs:      []string{"10.41.164.0"},
		nextSiteIntfCount: 2,
		selfSiteIntfCount: 1,
		nextSite1VIPNH: []map[string]string{
			{"Bundle-Ether2": "169.254.0.4"},
			{"Bundle-Ether3": "169.254.0.6"},
			{"Bundle-Ether4": "169.254.0.8"},
			{"Bundle-Ether5": "169.254.0.10"},
		},
		nextSite2VIPNH: []map[string]string{
			{"Bundle-Ether8": "169.254.0.16"},
			{"Bundle-Ether9": "169.254.0.18"},
		},
		selfSiteVIPNH: []map[string]string{
			{"Bundle-Ether1": "169.254.0.2"},
		},
	}
)

func TestRoutedFlowsLoadBalancing(t *testing.T) {
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
		TrafficFlowParam: []*helper.TrafficFlowAttr{&v4R2E, &v4E2R},
	}

	t.Log("Configure TGEN and set traffic flows")
	topo := helper.TGENHelper().ConfigureTGEN(useOTG, &tgenParam).ConfigureTgenInterface(t)

	trafficFlows := helper.TGENHelper().ConfigureTGEN(useOTG, &tgenParam).ConfigureTGENFlows(t)
	tgenVerifyParam := verifiers.TgenValidationParam{
		Tolerance: 0.02,
		WantLoss:  false,
		Flows:     trafficFlows,
	}

	t.Run("Verify Traffic passes after init Bringup", func(t *testing.T) {
		helper.TGENHelper().StartTraffic(t, useOTG, trafficFlows, 10*time.Second, topo, false)
		time.Sleep(5 * time.Second) // Wait for tgen traffic to completely stop.
		verifiers.TGENverifier().ValidateTGEN(false, &tgenVerifyParam).ValidateTrafficLoss(t)
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

			// Traffic flow map for v4, v6, IPinIP and IPv6inIP between R to E and E to R sites.
			trafficMap := make(map[string][]*helper.TrafficFlowAttr)
			trafficMap["v4"] = []*helper.TrafficFlowAttr{&v4R2E, &v4E2R}
			trafficMap["v6"] = []*helper.TrafficFlowAttr{&v6R2E, &v6E2R}
			trafficMap["IPinIP"] = []*helper.TrafficFlowAttr{&IPinIPR2E, &IPinIPE2R}
			trafficMap["IPv6inIP"] = []*helper.TrafficFlowAttr{&IPv6inIPR2E, &IPv6inIPE2R}

			//Run tests for each of Traffic Flow types (IPv4, IPv6, IPinIP, IPv6inIP).
			t.Log("Measure Traffic distribution from Site R-to-E on SiteE node going to Jupiter , & other way around")
			for trafficType, trafficFlows := range trafficMap {
				t.Run(fmt.Sprintf("%s flow", trafficType), func(t *testing.T) {
					tgenParam.TrafficFlowParam = trafficFlows
					trafficFlow := helper.TGENHelper().ConfigureTGEN(useOTG, &tgenParam).ConfigureTGENFlows(t)
					t.Log("Start Bidirectional Traffic flows")
					helper.TGENHelper().StartTraffic(t, useOTG, trafficFlow, 2*time.Minute, topo, false)
					time.Sleep(30 * time.Second) // Wait for 30 seconds for XR statsd interface cache to update

					var aftDestPrefix, prefixLength, afiType string
					//Verify traffic distribution on each of the cisco DUTs in the R, E sites.
					prefixLength = "/32"
					afiType = "ipv4"
					for _, device := range tt.confHashCLIdutList {
						fmt.Printf(tt.name+" for device: %s", device.Name())
						switch trafficType {
						case "v4", "IPinIP", "IPv6inIP":
							if strings.Contains(device.Name(), "E") {
								aftDestPrefix = eSiteV4DSTIP
							} else {
								aftDestPrefix = rSiteV4DSTIP
							}
						case "v6":
							if strings.Contains(device.Name(), "E") {
								aftDestPrefix = eSiteV6DSTPFX
								prefixLength = "/64"
								afiType = "ipv6"
							} else {
								aftDestPrefix = rSiteV6DSTPFX
								prefixLength = "/64"
								afiType = "ipv6"
							}
						default:
							t.Fatalf("Invalid traffic type %s", trafficType)
						}
						aftPfxObj := helper.FIBHelper().GetPrefixAFTObjects(t, device, aftDestPrefix+prefixLength, deviations.DefaultNetworkInstance(device), afiType)
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
									bundleMemberWt[i] = 1 // Default weight for Bundle members is 1.
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
						inputTrafficIF := helper.LoadbalancingHelper().GetIngressTrafficInterfaces(t, device, afiType, true)
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

						// Verify traffic distribution on Bundle NH Outgoing interfaces.
						t.Run(fmt.Sprintf("Bundle NH LB device %s", device.Name()), func(t *testing.T) {
							verifiers.Loadbalancingverifier().VerifyEgressDistributionPerWeight(t, device, OutputIFWeight, loadBalancingTolerance, true, afiType)
						})
						for _, bunIntf := range bundleObjList {
							var memberListWeight = make(map[string]uint64)
							for _, member := range bunIntf.BundleMembers {
								memberListWeight[member] = 1
							}
							// Verify traffic distribution on Bundle member LAG level loadbalancing for each bundle interface.
							t.Run(fmt.Sprintf("Bundle Member LB device %s on %s", device.Name(), bunIntf.BundleInterfaceName), func(t *testing.T) {
								verifiers.Loadbalancingverifier().VerifyEgressDistributionPerWeight(t, device, memberListWeight, loadBalancingTolerance, false, noTrafficType)
							})
						}
					}
				})
			}
		})
	}
}

func TestGribiFlowsLoadBalancing(t *testing.T) {
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

	test := verifiers.Interfaceverifier().VerifyShowInterfaceCLI(t, context.Background(), dut1E, "Bundle-Ether1")

	t.Log(test)
	for i, entry := range test {
		fmt.Printf("=== Entry %d ===\n", i)
		for key, val := range entry {
			fmt.Printf("%s: %s\n", key, val)
		}
		fmt.Println()
	}
	//Just to use variable and compile
	t.Log("R,E,V and Jupiter sites", siteRDUTList, siteEDUTList, siteVDUTList, jupiterDUTList)

	dvtCiscoDUTList := []*ondatra.DUTDevice{dut1R, dut2R, dut3R, dut4R, dut1E, dut2E}

	tgenParam := helper.TgenConfigParam{
		DutIntfAttr:      []attrs.Attributes{dutPort1, dutPort2},
		TgenIntfAttr:     []attrs.Attributes{atePort1, atePort2},
		TgenPortList:     []*ondatra.Port{ondatra.ATE(t, "ate").Port(t, "port1"), ondatra.ATE(t, "ate").Port(t, "port2")},
		TrafficFlowParam: []*helper.TrafficFlowAttr{&v4R2E, &v4E2R},
	}

	deviceGribiMap := make(map[*ondatra.DUTDevice]gribiParamPerSite)
	deviceGribiMap[dut1E] = gribi1E
	deviceGribiMap[dut2E] = gribi2E
	deviceGribiMap[dut1R] = gribi1R
	deviceGribiMap[dut2R] = gribi2R
	deviceGribiMap[dut3R] = gribi3R
	deviceGribiMap[dut4R] = gribi4R

	for device, gribiParam := range deviceGribiMap {
		t.Log("Program gRIBI entries for device: ", device.Name())
		programGribiEntries(t, device, gribiParam)
	}
	t.Log("gRIBI entries programmed successfully for all the devices")

	t.Log("Configure TGEN and set traffic flows")
	topo := helper.TGENHelper().ConfigureTGEN(useOTG, &tgenParam).ConfigureTgenInterface(t)

	trafficFlows := helper.TGENHelper().ConfigureTGEN(useOTG, &tgenParam).ConfigureTGENFlows(t)
	tgenVerifyParam := verifiers.TgenValidationParam{
		Tolerance: 0.02,
		WantLoss:  false,
		Flows:     trafficFlows,
	}

	t.Run("Verify Traffic passes after init Bringup", func(t *testing.T) {
		helper.TGENHelper().StartTraffic(t, useOTG, trafficFlows, 10*time.Second, topo, false)
		time.Sleep(5 * time.Second) // Wait for tgen traffic to completely stop.
		verifiers.TGENverifier().ValidateTGEN(false, &tgenVerifyParam).ValidateTrafficLoss(t)
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

			// Traffic flow map for v4, v6, IPinIP and IPv6inIP between R to E and E to R sites.
			trafficMap := make(map[string][]*helper.TrafficFlowAttr)
			trafficMap["v4"] = []*helper.TrafficFlowAttr{&v4R2E, &v4E2R}
			trafficMap["v6"] = []*helper.TrafficFlowAttr{&v6R2E, &v6E2R}
			trafficMap["IPinIP"] = []*helper.TrafficFlowAttr{&IPinIPR2E, &IPinIPE2R}
			trafficMap["IPv6inIP"] = []*helper.TrafficFlowAttr{&IPv6inIPR2E, &IPv6inIPE2R}

			//Run tests for each of Traffic Flow types (IPv4, IPv6, IPinIP, IPv6inIP).
			t.Log("Measure Traffic distribution from Site R-to-E on SiteE node going to Jupiter , & other way around")
			for trafficType, trafficFlows := range trafficMap {
				t.Run(fmt.Sprintf("%s flow", trafficType), func(t *testing.T) {
					tgenParam.TrafficFlowParam = trafficFlows
					trafficFlow := helper.TGENHelper().ConfigureTGEN(useOTG, &tgenParam).ConfigureTGENFlows(t)
					t.Log("Start Bidirectional Traffic flows")
					helper.TGENHelper().StartTraffic(t, useOTG, trafficFlow, 2*time.Minute, topo, false)
					time.Sleep(30 * time.Second) // Wait for 30 seconds for XR statsd interface cache to update

					var aftDestPrefix, prefixLength, afiType string
					//Verify traffic distribution on each of the cisco DUTs in the R, E sites.
					prefixLength = "/32"
					afiType = "ipv4"
					for _, device := range tt.confHashCLIdutList {
						fmt.Printf(tt.name+" for device: %s", device.Name())
						switch trafficType {
						case "v4", "IPinIP", "IPv6inIP":
							if strings.Contains(device.Name(), "E") {
								aftDestPrefix = eSiteV4DSTIP
							} else {
								aftDestPrefix = rSiteV4DSTIP
							}
						case "v6":
							if strings.Contains(device.Name(), "E") {
								aftDestPrefix = eSiteV6DSTPFX
								prefixLength = "/64"
								afiType = "ipv6"
							} else {
								aftDestPrefix = rSiteV6DSTPFX
								prefixLength = "/64"
								afiType = "ipv6"
							}
						default:
							t.Fatalf("Invalid traffic type %s", trafficType)
						}
						aftPfxObj := helper.FIBHelper().GetPrefixAFTObjects(t, device, aftDestPrefix+prefixLength, deviations.DefaultNetworkInstance(device), afiType)
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
									bundleMemberWt[i] = 1 // Default weight for Bundle members is 1.
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
						inputTrafficIF := helper.LoadbalancingHelper().GetIngressTrafficInterfaces(t, device, afiType, true)
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

						// Verify traffic distribution on Bundle NH Outgoing interfaces.
						t.Run(fmt.Sprintf("Bundle NH LB device %s", device.Name()), func(t *testing.T) {
							verifiers.Loadbalancingverifier().VerifyEgressDistributionPerWeight(t, device, OutputIFWeight, loadBalancingTolerance, true, afiType)
						})
						for _, bunIntf := range bundleObjList {
							var memberListWeight = make(map[string]uint64)
							for _, member := range bunIntf.BundleMembers {
								memberListWeight[member] = 1
							}
							// Verify traffic distribution on Bundle member LAG level loadbalancing for each bundle interface.
							t.Run(fmt.Sprintf("Bundle Member LB device %s on %s", device.Name(), bunIntf.BundleInterfaceName), func(t *testing.T) {
								verifiers.Loadbalancingverifier().VerifyEgressDistributionPerWeight(t, device, memberListWeight, loadBalancingTolerance, false, noTrafficType)
							})
						}
					}
				})
			}
		})
	}
}
