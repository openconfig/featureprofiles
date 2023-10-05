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

package oc_ppc_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/ha/monitor"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/ondatra"
)

// constant variables
const (
	vrf1 = "TE"
)

var (
	TestOC_PPC = map[string][]Testcase{
		TestOC_PPC_interface_subsystem = []Testcase{
			{
				name: "TC1 name",
				desc: "TC description",
				fn:   testhost,
			},
		},
		TestOC_PPC_queuing_subsystem = []Testcase{
			{
				name: "TC1 name",
				desc: "TC description",
				fn:   testhost,
			},
		},
		TestOC_PPC_lookup_subsystem = []Testcase{
			{
				name: "TC1 name",
				desc: "TC description",
				fn:   testhost,
			},
		},
		TestOC_PPC_host_subsystem = []Testcase{
			{
				name: "TC1 name",
				desc: "TC description",
				fn:   testhost,
			},
		},
		TestOC_PPC_fabric_subsystem = []Testcase{
			{
				name: "TC1 name",
				desc: "TC description",
				fn:   testhost,
			},
		},
	}
)

func testhost(){

}

// Testcase defines testcase structure
type Testcase struct {
	name string
	desc string
	fn   func(ctx context.Context, t *testing.T, args *testArgs)
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx     context.Context
	client  *gribi.Client
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     *ondatra.ATETopology
	events  *monitor.CachedConsumer
	ATELock sync.Mutex
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOC_PPC(t *testing.T) {
	t.Log("Name: OC PPC")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	// ctx, cancelMonitors := context.WithCancel(context.Background())

	// Configure the DUT
	var vrfs = []string{vrf1}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	// PBR config
	// configbasePBR(t, dut, "REPAIRED", "ipv4", 1, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_UNSET, []uint8{}, &PBROptions{SrcIP: "222.222.222.222/32"})
	// configbasePBR(t, dut, "TE", "ipv4", 2, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	// configbasePBRInt(t, dut, "Bundle-Ether120", "pbr")
	// RoutePolicy config
	configRP(t, dut)
	// configure ISIS on DUT
	addISISOC(t, dut, "Bundle-Ether127")
	// configure BGP on DUT
	addBGPOC(t, dut, "100.100.100.100")

	// Configure the ATE
	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	if *ciscoFlags.GRIBITrafficCheck {
		addPrototoAte(t, top)
		time.Sleep(120 * time.Second)
	}
	for subsystem_name, subsystem_tc_data := range TestOC_PPC {
		t.Logf("Executing Subsystem: %s", subsystem_name)

		//Creating gribi client for run across 1 subsystem
		clientA := gribi.Client{
			DUT:         ondatra.DUT(t, "dut"),
			FIBACK:      true,
			Persistence: true,
			InitialElectionIDLow:  1,
			InitialElectionIDHigh: 0,
		}}
		defer clientA.Close(t)
		if err := clientA.Start(t); err != nil {
			t.Logf("gRIBI Connection could not be established: %v\nRetrying...", err)
			if err = clientA.Start(t); err != nil {
				t.Fatalf("gRIBI Connection could not be established: %v", err)
			}
		}
		clientA.BecomeLeader(t)

		for _, subsystem_tc := range subsystem_tc_data{
			t.Run(subsystem_tc.name, func(t *testing.T) {
				t.Logf("Name: %s", subsystem_tc.name)
				t.Logf("Description: %s", subsystem_tc.desc)

				args := &testArgs{
					ctx:        ctx,
					clientA:    &clientA,
					dut:        dut,
					ate:        ate,
					top:        top,
					},
				}
				tt.fn(ctx, t, args)
			})
		}
	}
}
