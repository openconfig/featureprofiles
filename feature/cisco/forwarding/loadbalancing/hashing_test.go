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
	// "fmt"

	// "os"
	// "sort"
	// "strings"
	"context"
	"fmt"
	// "math/rand"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"

	// "text/tabwriter"
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
	ipv6PrefixLen = 126
)

var (
	trafficTolerance = 0.02
	rSiteV4Prefix    = "10.240.118.50"
	eSiteV4Prefix    = "10.240.118.35"
	rSiteV6Prefix    = "2001:0db8::10:240:118:50"
	eSiteV6Prefix    = "2001:0db8::10:240:118:35"

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

// type extendedEntropyCLIOptions struct {
// 	perChasiss  string
// 	perNPU      string
// 	specificVal string
// }

// type algorithmAdjustCLIOptions struct {
// 	perChasiss  string
// 	perNPU      string
// 	specificVal string
// }

var (
	//Traffic flows
	v4R2E = helper.TrafficFlowAttr{
		FlowName:          "IPv4",
		DstMacAddress:     dutPort1.MAC,
		OuterProtocolType: "IPv4",
		OuterSrcStart:     "192.1.1.1",
		OuterDstStart:     eSiteV4Prefix,
		OuterSrcStep:      "0.0.0.1",
		OuterSrcFlowCount: 50000,
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
		L4FlowCount:       50000,
		TrafficPPS:        100,
		PacketSize:        128,
	}
	v4E2R = helper.TrafficFlowAttr{
		FlowName:          "IPv4",
		DstMacAddress:     dutPort2.MAC,
		OuterProtocolType: "IPv4",
		OuterSrcStart:     "192.0.2.6",
		OuterDstStart:     rSiteV4Prefix,
		OuterSrcStep:      "0.0.0.1",
		OuterSrcFlowCount: 50000,
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
		L4FlowCount:       50000,
		TrafficPPS:        100,
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
	dutList               []*ondatra.DUTDevice
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
	t.Run("Verify Traffic after init Bringup", func(t *testing.T) {
		helper.TGEN.StartTraffic(t, false, ateFlows, 10*time.Second, topo)
		verifiers.Tgen.ValidateTGEN(false, &tgenVerifyParam).ValidateTrafficLoss(t)
	})

	//Verify Traffic stats on InputIF and match with OutputIF
	cases := []testCase{
		{
			name: "Default",
			desc: "Default Hash parameters",
		},
		{
			name:                  "Both auto-global",
			desc:                  "auto-global Hash parameters for both Extended Entropy and Algorithm Adjust",
			extendedEntropyOption: &extendedEntropyCLIOptions{perChassis: true},
			algorithmAdjustOption: &algorithmAdjustCLIOptions{perChassis: true},
			dutList:               dvtCiscoDUTList,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Log("Configuring hashing parameters")
			if tt.extendedEntropyOption != nil || tt.algorithmAdjustOption != nil {
				configureOptions(t, tt.extendedEntropyOption, tt.algorithmAdjustOption, tt.dutList)
			}
			t.Logf("Get AFT Prefix objects for %s", eSiteV4Prefix)
			afttest := helper.FIB.GetPrefixAFTObjects(t, dut1E, eSiteV4Prefix+"/32", deviations.DefaultNetworkInstance(dut1E))
			memberList := helper.Interface.GetBundleMembers(t, dut1E, afttest.NextHop[0].NextHopInterface)
			var bundleMembers []string
			for _, intfList := range memberList {
				bundleMembers = append(bundleMembers, intfList...)
			}
			helper.Interface.ClearInterfaceCountersAll(t, dut1E)
			time.Sleep(30 * time.Second) // Wait for 30 seconds for XR statsd interface cache to update
			helper.TGEN.StartTraffic(t, false, ateFlows, 10*time.Second, topo)
			InputIF := helper.Loadbalancing.GetIngressTrafficInterfaces(t, dut1E, "ipv4")
			var totalInPackets uint64
			for _, val := range InputIF {
				totalInPackets += val
			}
			t.Log("TotalInPackets is", totalInPackets)
			var OutputIFWeight = make(map[string]uint64)
			for _, nhObj := range afttest.NextHop {
				OutputIFWeight[nhObj.NextHopInterface] = nhObj.NextHopWeight
			}
			var memberListWeight = make(map[string]uint64)
			for _, member := range bundleMembers {
				memberListWeight[member] = 1
			}
			t.Log("Verify Bundle non-recursive level loadbalancing")
			verifiers.Loadbalancing.VerifyEgressDistributionPerWeight(t, dut1E, OutputIFWeight, totalInPackets, trafficTolerance)
			t.Log("Verify Bundle member LAG level loadbalancing")
			verifiers.Loadbalancing.VerifyEgressDistributionPerWeight(t, dut1E, memberListWeight, totalInPackets, trafficTolerance)
		})
	}
}

func configureOptions(t *testing.T, extendHash *extendedEntropyCLIOptions, hashRotate *algorithmAdjustCLIOptions, dutList []*ondatra.DUTDevice) {
	for _, dut := range dutList {
		if extendHash != nil {
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
		if hashRotate != nil {
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

// func configureOptions(t *testing.T, extendHash *extendedEntropyOption , optionType string, dutList []*ondatra.DUTDevice) {
// 	for _, dut := range dutList {
// 		switch optionType {
// 		case "extendedEntropyOption":
// 			switch optionType {
// 			case "perChassis":
// 				// Configure for per chassis
// 				config.TextWithGNMI(context.Background(), t, dut, "cef platform load-balancing extended-entropy auto-global")
// 			case "perNPU":
// 				// Configure for per NPU
// 				config.TextWithGNMI(context.Background(), t, dut, "cef platform load-balancing extended-entropy auto-instance")
// 			case "specificValue":
// 				// Configure for specific value
// 				config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("cef platform load-balancing extended-entropy profile-index %d", rand.Intn(215)))
// 			default:
// 				t.Log("Default Hashing parameters set")
// 			}
// 		case "algorithmAdjustOption":
// 			switch optionType {
// 			case "perChassis":
// 				// Configure for per chassis
// 				config.TextWithGNMI(context.Background(), t, dut, "cef platform load-balancing algorithm adjust auto-global")
// 			case "perNPU":
// 				// Configure for per NPU
// 				config.TextWithGNMI(context.Background(), t, dut, "cef platform load-balancing algorithm adjust auto-instance")
// 			case "specificValue":
// 				// Configure for specific value
// 				config.TextWithGNMI(context.Background(), t, dut, fmt.Sprintf("cef platform load-balancing algorithm adjust %d", rand.Intn(215)))
// 			default:
// 				t.Log("Default Hashing parameters set")
// 			}
// 		default:

// 		}
// 	}

// }
