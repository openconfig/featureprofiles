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
	"github.com/openconfig/featureprofiles/internal/deviations"
	"testing"
	// "text/tabwriter"
	// "time"

	// "github.com/openconfig/featureprofiles/internal/attrs"

	// "github.com/openconfig/featureprofiles/internal/components"
	// "github.com/openconfig/featureprofiles/internal/cisco/verifiers"
	"github.com/openconfig/featureprofiles/internal/cisco/helper"
	"github.com/openconfig/featureprofiles/internal/fptest"

	"github.com/openconfig/ondatra"
	// "github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/ondatra/gnmi/oc"
	// "github.com/openconfig/ondatra/netutil"
)

const (
	cardTypeRp = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
)

// var (
// 	lcList = []string{}
// 	rtrID  uint32
// 	npList = []int{0, 1, 2}
// 	h      = NpuHash{npList: []int{0, 1, 2}}
// )

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

func TestLoadBalancing(t *testing.T) {
	// dut1R := ondatra.DUT(&testing.T{}, "dut1")
	dut1E := ondatra.DUT(t, "dut7")
	afttest := helper.FIB.GetPrefixAFTObjects(t, dut1E, "10.240.118.35/32", deviations.DefaultNetworkInstance(dut1E))
	t.Log("afttest", afttest)
	InputIF := helper.Loadbalancing.GetIngressTrafficInterfaces(t, dut1E, "ipv4")
	var OutputIFWeight = make(map[string]uint64)
	for _, nhObj := range afttest.NextHop {
		OutputIFWeight[nhObj.NextHopInterface] = nhObj.NextHopWeight
	}
	t.Log("InputIF", InputIF, OutputIFWeight)
	helper.Interface.ClearInterfaceCountersAll(t, dut1E)
	//Verify Traffic stats on InputIF and match with OutputIF

	// pfxNH, NHG := helper.FIB.GetPrefixAFTNH(t, dut1E, "10.240.118.35/32", deviations.DefaultNetworkInstance(dut1E))
	// t.Log("pfxNH", pfxNH)
	// t.Log("NHIP", NHIP)
	// helper.Loadbalancing.GetPrefixOutGoingInterfaces(t, dut1E, "10.240.118.0/24", deviations.DefaultNetworkInstance(dut1E))
	helper.Loadbalancing.GetIngressTrafficInterfaces(t, dut1E, "ipv4")
	// util.ClearInterfaceCountersAll(t, dut1E)
	util.CheckDUTTrafficViaInterfaceTelemetry(t, dut1E, []string{"Bundle-Ether1", "Bundle-Ether2", "Bundle-Ether3", "Bundle-Ether4", "Bundle-Ether5"}, []string{"Bundle-Ether14", "Bundle-Ether15"}, []float64{0.5, 0.5}, 2)
	t.Logf("Get list of LCs")
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
