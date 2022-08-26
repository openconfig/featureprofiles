// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gribi

import (
	"context"
	"testing"

	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gribigo/server"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

type testArgs struct {
	ctx      context.Context
	c1       *Client
	c2       *Client
	dut      *ondatra.DUTDevice
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}



func testADDReplaceDeleteSingleEntries(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t) // let test to clear the entries at the begining for itself.
	//Using defer is problamtic since it may cause failuer when the gribi connection drops
	weights := map[uint64]uint64{3: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, server.DefaultNetworkInstanceName, false, ciscoFlags.GRIBIChecks)
	// replace
	args.c1.ReplaceNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, server.DefaultNetworkInstanceName, false, ciscoFlags.GRIBIChecks)
	// delete
	args.c1.DeleteIPv4(t, "11.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, server.DefaultNetworkInstanceName, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 11, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
}

func testBatchADDReplaceDeleteIPV4(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)
	// 192.0.2.42/32  Next-Site
	weights := map[uint64]uint64{41: 40}
	args.c1.AddNH(t, 41, "100.129.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks) // Not connected
	args.c1.AddNHG(t, 100, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32 Self-Site
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.11", i, "32"))
	}
	weights = map[uint64]uint64{20: 99}
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// Add
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// Replace
	args.c1.ReplaceIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	// Delete
	args.c1.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

}

// Transit TC 067 - Same forwarding entries across multiple vrfs
func testAddEntriesAcrossMultipleVrfs(t *testing.T, args *testArgs) {
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	weights1 := map[uint64]uint64{3: 15}
	weights2 := map[uint64]uint64{4: 15}
	args.c1.AddNH(t, 3, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 11, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 4, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 14, 11, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "12.11.11.11/32", 11, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "12.11.11.12/32", 14, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	args.c1.AddIPv4(t, "12.11.11.11/32", 11, "VRF1", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "12.11.11.12/32", 14, "VRF1", *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

}

// ADD/REPLACE/DELETE  with Interface
func testBatchAddReplaceDeleteEntriesWithInterface(t *testing.T, args *testArgs) {
	t.Log("Testcase: Add, Replace, Delete operations with related interface flap")
	args.c1.BecomeLeader(t)
	args.c1.FlushServer(t)

	// 192.0.2.40/32  Self-Site
	weights1 := map[uint64]uint64{
		31: 10,
		32: 20,
		33: 30,
	}
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	weights2 := map[uint64]uint64{
		41: 15,
		42: 25,
		43: 35,
		44: 45,
	}
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	prefixes := []string{}
	for i := 0; i < int(*ciscoFlags.GRIBIScale); i++ {
		prefixes = append(prefixes, util.GetIPPrefix("11.11.11.0", i, "32"))
	}
	weights3 := map[uint64]uint64{
		10: 85,
		20: 15,
	}
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	//Replace all entries
	// 192.0.2.40/32  Self-Site
	args.c1.ReplaceNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	args.c1.ReplaceNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	args.c1.ReplaceNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.ReplaceIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)

	//Delete all entries
	// 192.0.2.40/32  Self-Site
	args.c1.DeleteIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	args.c1.DeleteIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	args.c1.DeleteIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.DeleteNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.40/32  Self-Site
	args.c1.AddNH(t, 31, "100.121.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether121", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 32, "100.122.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether122", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 33, "100.123.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether123", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 40, 0, weights1, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.40/32", 40, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 192.0.2.42/32  Next-Site
	args.c1.AddNH(t, 41, "100.124.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether124", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 42, "100.125.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether125", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 43, "100.126.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether126", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 44, "100.127.1.2", *ciscoFlags.DefaultNetworkInstance, "", "Bundle-Ether127", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 100, 0, weights2, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4(t, "192.0.2.42/32", 100, *ciscoFlags.DefaultNetworkInstance, "", false, ciscoFlags.GRIBIChecks)

	// 11.11.11.0/32
	args.c1.AddNH(t, 10, "192.0.2.40", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNH(t, 20, "192.0.2.42", *ciscoFlags.DefaultNetworkInstance, "", "", false, ciscoFlags.GRIBIChecks)
	args.c1.AddNHG(t, 1, 0, weights3, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
	args.c1.AddIPv4Batch(t, prefixes, 1, *ciscoFlags.NonDefaultNetworkInstance, *ciscoFlags.DefaultNetworkInstance, false, ciscoFlags.GRIBIChecks)
}

func TestGRIBIAPI(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	d := &telemetry.Device{}
	ni := d.GetOrCreateNetworkInstance(*ciscoFlags.NonDefaultNetworkInstance)
	ni.GetOrCreateProtocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "default")
	dut.Config().NetworkInstance(*ciscoFlags.NonDefaultNetworkInstance).Replace(t, ni)
	ciscoFlags.GRIBIFIBCheck = ygot.Bool(true)
	ciscoFlags.GRIBIChecks.AFTChainCheck=false
	ciscoFlags.GRIBIChecks.AFTCheck=false
	ciscoFlags.GRIBIChecks.FIBACK = true
	ciscoFlags.GRIBIChecks.RIBACK = true

	test := []struct {
		name string
		desc string
		fn   func(t *testing.T, args *testArgs)
	}{
		{
			name: "testADDReplaceDeleteSingleEntries",
			desc: "testADDReplaceDeleteSingleEntries",
			fn:   testADDReplaceDeleteSingleEntries,
		},
		/*{
			name: "testAddEntriesAcrossMultipleVrfs",
			desc: "testAddEntriesAcrossMultipleVrfs",
			fn:   testAddEntriesAcrossMultipleVrfs,
		},*/
		/*{
			name: "testBatchADDReplaceDeleteIPV4",
			desc: "testBatchADDReplaceDeleteIPV4",
			fn:   testBatchADDReplaceDeleteIPV4,
		},*/
	}
	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Name: %s", tt.name)
			t.Logf("Description: %s", tt.desc)
			client1 := Client{
				DUT:                  dut,
				FibACK:               *ciscoFlags.GRIBIFIBCheck,
				Persistence:          true,
				InitialElectionIDLow: 100,
			}
			client2 := Client{
				DUT:                  dut,
				FibACK:               *ciscoFlags.GRIBIFIBCheck,
				Persistence:          true,
				InitialElectionIDLow: 10,
			}

			defer client1.Close(t)

			if err := client1.Start(t); err != nil {
				t.Fatalf("gRIBI Connection can not be established")
			}

			defer client2.Close(t)

			if err := client2.Start(t); err != nil {
				t.Fatalf("gRIBI Connection can not be established")
			}
			args := &testArgs{
				ctx: ctx,
				c1:  &client1,
				c2:  &client2,
				dut: dut,
			}
			tt.fn(t, args)
		})
	}
}
