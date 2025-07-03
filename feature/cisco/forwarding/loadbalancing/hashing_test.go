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

	"github.com/openconfig/featureprofiles/internal/deviations"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"

	// "github.com/openconfig/featureprofiles/internal/components"
	// "github.com/openconfig/featureprofiles/internal/cisco/verifiers"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/helper"
	"github.com/openconfig/featureprofiles/internal/cisco/verifiers"
	"github.com/openconfig/featureprofiles/internal/fptest"

	"github.com/openconfig/ondatra"
	// "github.com/openconfig/ondatra/gnmi"
	// "github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra/gnmi/oc"
	// "github.com/openconfig/ondatra/netutil"
)

const (
	cardTypeRp    = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	ipv4PrefixLen = 30
	v4PrefixLen32 = 32
	ipv6PrefixLen = 126
)

// Traffic flow attributes
var (
	trafficTolerance = 0.03
	rSiteV4Prefix    = "10.240.118.50"
	eSiteV4Prefix    = "10.240.118.35"
	rSiteV6Prefix    = "2001:0db8::10:240:118:50"
	eSiteV6Prefix    = "2001:0db8::10:240:118:35"
	v4TrafficType    = "ipv4"
	v6TrafficType    = "ipv6"
	noTrafficType    = ""
	srcIPFlowCount   = 50000
	L4FlowCount      = 50000
)

// DUT and TGEN port attributes
var (
	dutPort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "dutPort1",
		MAC:     "00:aa:00:bb:00:cc",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:1",
		IPv6Len: ipv6PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "port1",
		Desc:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		Name:    "port2",
		MAC:     "00:bb:00:11:00:dd",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "04:00:02:02:02:02",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	//Traffic flows
	v4R2E = helper.TrafficFlowAttr{
		FlowName:          "IPv4",
		DstMacAddress:     dutPort1.MAC,
		OuterProtocolType: "IPv4",
		OuterSrcStart:     "192.1.1.1",
		OuterDstStart:     eSiteV4Prefix,
		OuterSrcStep:      "0.0.0.1",
		OuterSrcFlowCount: uint32(srcIPFlowCount),
		OuterDstFlowCount: 1,
		OuterDstStep:      "0.0.0.1",
		OuterDSCP:         10,
		OuterTTL:          55,
		OuterECN:          1,
		TgenSrcPort:       atePort1,
		TgenDstPorts:      []string{atePort2.Name},
		L4TCP:             true,
		L4PortRandom:      true,
		L4SrcPortStart:    1000,
		L4DstPortStart:    2000,
		L4FlowStep:        1,
		L4FlowCount:       uint32(srcIPFlowCount),
		TrafficPPS:        200,
		PacketSize:        128,
	}
	v4E2R = helper.TrafficFlowAttr{
		FlowName:          "IPv4",
		DstMacAddress:     dutPort2.MAC,
		OuterProtocolType: "IPv4",
		OuterSrcStart:     "192.0.2.6",
		OuterDstStart:     rSiteV4Prefix,
		OuterSrcStep:      "0.0.0.1",
		OuterSrcFlowCount: uint32(srcIPFlowCount),
		OuterDstFlowCount: 1,
		OuterDstStep:      "0.0.0.1",
		OuterDSCP:         10,
		OuterTTL:          55,
		OuterECN:          1,
		TgenSrcPort:       atePort2,
		TgenDstPorts:      []string{atePort1.Name},
		L4TCP:             true,
		L4PortRandom:      true,
		L4SrcPortStart:    1000,
		L4DstPortStart:    2000,
		L4FlowStep:        1,
		L4FlowCount:       uint32(srcIPFlowCount),
		TrafficPPS:        200,
		PacketSize:        128,
	}
)

type extendedEntropyCLIOptions struct {
	perChassis  bool
	perNPU      bool
	specificVal uint32
}

type algorithmAdjustCLIOptions struct {
	perChassis  bool
	perNPU      bool
	specificVal uint32
}

type testCase struct {
	name                  string
	desc                  string
	extendedEntropyOption *extendedEntropyCLIOptions
	algorithmAdjustOption *algorithmAdjustCLIOptions
	confHashCLIdutList    []*ondatra.DUTDevice
}

type BundleInterface struct {
	BundleInterfaceName string
	BundleNHWeight      uint64
	BundleMembers       []string
	BundleMembersWeight []uint64
}

func TestWANLinksLoadBalancing(t *testing.T) {
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
	topo := helper.TGEN.ConfigureTGEN(false, &tgenParam).ConfigureTgenInterface(t)
	ateFlows := helper.TGEN.ConfigureTGEN(false, &tgenParam).ConfigureTGENFlows(t)
	tgenVerifyParam := verifiers.TgenValidationParam{
		// Use an exported field or method to set tolerance
		Tolerance: 0.02, // Assuming Tolerance is the correct exported field
		WantLoss:  false,
		Flows:     ateFlows,
	}
	t.Run("Verify Traffic passes after init Bringup", func(t *testing.T) {
		helper.TGEN.StartTraffic(t, false, ateFlows, 10*time.Second, topo, false)
		time.Sleep(5 * time.Second) // Wait fortgen traffic to completely stop.
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
			helper.Interface.ClearInterfaceCountersAll(t, dvtCiscoDUTList)

			t.Log("Start Bidirectional Traffic flows")
			helper.TGEN.StartTraffic(t, false, ateFlows, 2*time.Minute, topo, true)
			time.Sleep(30 * time.Second) // Wait for 30 seconds for XR statsd interface cache to update

			t.Log("Measure Traffic distribution from Site R-to-E on SiteE node going to Jupiter , & other way around")
			var v4DstPrefix string
			for _, device := range tt.confHashCLIdutList {
				fmt.Printf(tt.name+" for device: %s", device.Name())
				if strings.Contains(device.Name(), "E") {
					v4DstPrefix = eSiteV4Prefix
				} else {
					v4DstPrefix = rSiteV4Prefix
				}
				t.Logf("Get AFT Prefix objects for %s", v4DstPrefix)
				aftPfxObj := helper.FIB.GetPrefixAFTObjects(t, device, v4DstPrefix+"/32", deviations.DefaultNetworkInstance(device))
				bundleObjList := []BundleInterface{}

				for _, nhObj := range aftPfxObj.NextHop {
					bundleObj := BundleInterface{}
					bundleObj.BundleInterfaceName = nhObj.NextHopInterface
					bundleObj.BundleNHWeight = nhObj.NextHopWeight
					memberMap := helper.Interface.GetBundleMembers(t, device, nhObj.NextHopInterface)
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

				inputTrafficIF := helper.Loadbalancing.GetIngressTrafficInterfaces(t, device, "ipv4", true)
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
					verifiers.Loadbalancing.VerifyEgressDistributionPerWeight(t, device, OutputIFWeight, trafficTolerance, true, v4TrafficType)
				})
				for _, bunIntf := range bundleObjList {
					var memberListWeight = make(map[string]uint64)
					for _, member := range bunIntf.BundleMembers {
						memberListWeight[member] = 1
					}
					t.Run(fmt.Sprintf("Verify Bundle member LAG level loadbalancing on device %s on Bundle %s", device.Name(), bunIntf.BundleInterfaceName), func(t *testing.T) {
						verifiers.Loadbalancing.VerifyEgressDistributionPerWeight(t, device, memberListWeight, trafficTolerance, false, noTrafficType)
					})
				}
			}
		})
	}
}

func configureHashCLIOptions(t *testing.T, extendHash *extendedEntropyCLIOptions, hashRotate *algorithmAdjustCLIOptions, dutList []*ondatra.DUTDevice, delete bool) {
	for _, dut := range dutList {
		if extendHash != nil {
			if delete {
				// Delete extended entropy configuration for all options
				t.Log("Deleting extended entropy configuration")
				config.TextWithGNMI(context.Background(), t, dut, "no cef platform load-balancing extended-entropy auto-global\n no cef platform load-balancing extended-entropy auto-instance\n no cef platform load-balancing extended-entropy profile-index")
				// config.TextWithGNMI(context.Background(), t, dut, "no cef platform load-balancing extended-entropy auto-instance")
				// config.TextWithGNMI(context.Background(), t, dut, "no cef platform load-balancing extended-entropy profile-index")
			} else {
				t.Log("Configuring extended entropy configuration")
				if extendHash.perChassis {
					// Configure for per chassis
					config.TextWithGNMI(context.Background(), t, dut, "cef platform load-balancing extended-entropy auto-global")
				}
				if extendHash.perNPU {
					// Configure for per NPU
					config.TextWithGNMI(context.Background(), t, dut, "cef platform load-balancing extended-entropy auto-instance")
				}
				if extendHash.specificVal != 0 {
					// Configure for specific value
					config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("cef platform load-balancing extended-entropy profile-index %d", extendHash.specificVal))
				}
			}
		}
		if hashRotate != nil {
			if delete {
				// Delete hash rotation configuration for all options
				t.Log("Deleting hash rotate configuration")
				config.TextWithGNMI(context.Background(), t, dut, "no cef platform load-balancing algorithm adjust auto-global\n no cef platform load-balancing algorithm adjust auto-instance\n no cef platform load-balancing algorithm adjust")
				// config.TextWithGNMI(context.Background(), t, dut, "no cef platform load-balancing algorithm adjust auto-instance")
				// config.TextWithGNMI(context.Background(), t, dut, "no cef platform load-balancing algorithm adjust")
			} else {
				t.Log("Configuring hash rotate configuration")
				if hashRotate.perChassis {
					// Configure for per chassis
					config.TextWithGNMI(context.Background(), t, dut, "cef platform load-balancing algorithm adjust auto-global")
				}
				if hashRotate.perNPU {
					// Configure for per NPU
					config.TextWithGNMI(context.Background(), t, dut, "cef platform load-balancing algorithm adjust auto-instance")
				}
				if hashRotate.specificVal != 0 {
					// Configure for specific value
					config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("cef platform load-balancing algorithm adjust %d", hashRotate.specificVal))
				}
			}
		}
	}
}
