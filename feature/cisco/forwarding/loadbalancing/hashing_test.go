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
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"

	// "text/tabwriter"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"

	// "github.com/openconfig/featureprofiles/internal/components"
	// "github.com/openconfig/featureprofiles/internal/cisco/verifiers"
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
	lcList           = []string{}
	rtrID            uint32
	npList           = []int{0, 1, 2}
	h                = NpuHash{npList: []int{0, 1, 2}}
	trafficTolerance = 0.02
	dutPort1         = attrs.Attributes{
		Name:    "port1",
		Desc:    "dutPort1",
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
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:5",
		IPv6Len: ipv6PrefixLen,
	}

	atePort2 = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:0db8::192:0:2:6",
		IPv6Len: ipv6PrefixLen,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// type testCase struct {
// 	name    string
// 	desc    string
// 	npval   []int
// 	hashval int
// 	lcloc   []string
// }

var (
	//Traffic flows
	v4R2E = helper.TrafficFlowAttr{
		FlowName:          "IPv4",
		DstMacAddress:     "00:aa:00:bb:00:cc",
		OuterProtocolType: "IPv4",
		OuterSrcStart:     "192.1.1.1",
		OuterDstStart:     "10.240.118.35",
		OuterSrcStep:      "0.0.0.1",
		OuterSrcFlowCount: 1,
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
		L4FlowCount:       10,
		TrafficPPS:        100,
		PacketSize:        128,
	}
	v4E2R = helper.TrafficFlowAttr{
		FlowName:          "IPv4",
		DstMacAddress:     "00:aa:00:11:00:dd",
		OuterProtocolType: "IPv4",
		OuterSrcStart:     "192.0.2.6",
		OuterDstStart:     "10.240.118.50",
		OuterSrcStep:      "0.0.0.1",
		OuterSrcFlowCount: 1,
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
		L4FlowCount:       10,
		TrafficPPS:        100,
		PacketSize:        128,
	}
)

func TestLoadBalancing(t *testing.T) {
	// dut1R := ondatra.DUT(&testing.T{}, "dut1")
	tgenParam := helper.TgenConfigParam{
		DutIntfAttr:      []attrs.Attributes{dutPort1, dutPort2},
		TgenIntfAttr:     []attrs.Attributes{atePort1, atePort2},
		TgenPortList:     []*ondatra.Port{ondatra.ATE(t, "ate").Port(t, "port1"), ondatra.ATE(t, "ate").Port(t, "port2")},
		TrafficFlowParam: []*helper.TrafficFlowAttr{&v4R2E, &v4E2R},
	}
	topo := helper.TGEN.ConfigureTGEN(false, &tgenParam).ConfigureTgenInterface(t)
	flows := helper.TGEN.ConfigureTGEN(false, &tgenParam).ConfigureTGENFlows(t)
	helper.TGEN.StartTraffic(t, false, flows, 10*time.Second, topo)
	ate := topo.ATE
	t.Log("ate", ate.String())
	dut1E := ondatra.DUT(t, "B4E1")
	afttest := helper.FIB.GetPrefixAFTObjects(t, dut1E, "10.240.118.35/32", deviations.DefaultNetworkInstance(dut1E))
	memberList := helper.Interface.GetBundleMembers(t, dut1E, afttest.NextHop[0].NextHopInterface)
	var bundleMembers []string
	for _, intfList := range memberList {
		bundleMembers = append(bundleMembers, intfList...)
	}
	t.Log("memberL", memberList)
	t.Log("afttest", afttest)
	helper.Interface.ClearInterfaceCountersAll(t, dut1E)
	time.Sleep(30 * time.Second)
	InputIF := helper.Loadbalancing.GetIngressTrafficInterfaces(t, dut1E, "ipv4")
	var totalInPackets uint64
	for _, val := range InputIF {
		totalInPackets += val
	}
	t.Log("totalInPackets", totalInPackets)
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
	t.Log("InputIF", InputIF, OutputIFWeight)

	//Verify Traffic stats on InputIF and match with OutputIF

	// pfxNH, NHG := helper.FIB.GetPrefixAFTNH(t, dut1E, "10.240.118.35/32", deviations.DefaultNetworkInstance(dut1E))
	// t.Log("pfxNH", pfxNH)
	// t.Log("NHIP", NHIP)
	// helper.Loadbalancing.GetPrefixOutGoingInterfaces(t, dut1E, "10.240.118.0/24", deviations.DefaultNetworkInstance(dut1E))
	// util.ClearInterfaceCountersAll(t, dut1E)
	// util.CheckDUTTrafficViaInterfaceTelemetry(t, dut1E, []string{"Bundle-Ether1", "Bundle-Ether2", "Bundle-Ether3", "Bundle-Ether4", "Bundle-Ether5"}, []string{"Bundle-Ether14", "Bundle-Ether15"}, []float64{0.5, 0.5}, 2)
	// t.Logf("Get list of LCs")
	// lcList = util.GetLCList(t, dut)
	// cases := []testCase{
	// 	{
	// 		name:    "Test perNP CLI config for 1 NP",
	// 		desc:    "Configure per-NP hash value on single NP0 for all LCs",
	// 		npval:   npList[:1],
	// 		hashval: rand.Intn(34) + 1, // valid range (1-35]
	// 		lcloc:   lcList,
	// 	},
	// 	{
	// 		name:    "Test perNP CLI config for All NPs",
	// 		desc:    "Configure per-NP hash value on All the NPs for all LCs",
	// 		npval:   npList,
	// 		hashval: rand.Intn(34) + 1,
	// 		lcloc:   lcList,
	// 	},
	// }
	// for _, tc := range cases {
	// 	tc := tc
	// 	t.Run(tc.name, func(t *testing.T) {
	// 		for _, lc := range tc.lcloc {
	// 			for _, np := range tc.npval {
	// 				setPerNPHashConfig(t, dut, tc.hashval, np, lc, true)
	// 				defer setPerNPHashConfig(t, dut, tc.hashval, np, lc, false)
	// 				if got, want := getPerLCPerNPHashVal(t, dut, np, lc), verifyPerNPHashCLIVal(tc.hashval); got != want {
	// 					t.Errorf("per-NP hash rotate value for LC %v NP%v is not per calculation got %v, want %v", lc, tc.npval, got, want)
	// 				}
	// 			}
	// 		}
	// 	})
	// }
}
