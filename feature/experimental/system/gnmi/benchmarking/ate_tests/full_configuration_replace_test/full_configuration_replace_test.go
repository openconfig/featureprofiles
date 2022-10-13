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

package full_configuration_replace_test

import (
	"sort"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/featureprofiles/feature/experimental/system/gnmi/benchmarking/ate_tests/internal/benchmarking_setup"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Test case:
// Configure DUT with:
//  Maximum number of interfaces to be supported.
//  Maximum number of BGP peers to be supported.
//  Maximum number of IS-IS adjacencies to be supported.
// Measure time required for Set operation to complete.
// Modify descriptions of a subset of interfaces within the system.
// Measure time for Set to complete.
//
// Topology:
//   dut:port(1..N)
//
// Test notes:
// This test does not cover entirely converged system, simply replacing
// the configuration for the initial case, and then a case where the device
// generates a diff.
//

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

// modIntfDesc builds OC config to modify description of a subset of interfaces.
func modIntfDesc(t *testing.T) *gpb.Update {
	type M map[string]interface{}
	var intfConfig []M

	dut := ondatra.DUT(t, "dut")
	dutPorts := sortPorts(dut.Ports())

	for i := 0; i < len(dutPorts); i++ {
		if i%2 == 0 {
			elem := map[string]interface{}{
				"name": dutPorts[i].Name(),
				"config": map[string]interface{}{
					"description": "modified via oc",
				},
			}

			intfConfig = append(intfConfig, elem)
		}
	}

	update := benchmarking_setup.CreateGNMIUpdate("interfaces", "interface", intfConfig)
	return update
}

func TestGnmiFullConfigReplace(t *testing.T) {

	benchmarking_setup.BuildIPPool(t)

	// Building gNMI Set request payload to configure interfaces, ISIS and BGP protocols
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			benchmarking_setup.BuildOCInterfaceUpdate(t),
			benchmarking_setup.BuildOCISISUpdate(t),
			benchmarking_setup.BuildOCBGPUpdate(t),
		},
	}

	t.Logf("Sending gNMI Set request to configure interfaces, ISIS and BGP protocols")
	//Start the timer.
	start := time.Now()
	benchmarking_setup.ConfigureGNMISetRequest(t, gpbSetRequest)

	//End the timer and calculate time.
	elapsed := time.Since(start)
	t.Logf("Time taken for the gNMI SetRequest %v", elapsed)

	t.Logf("Modify description of a subset of interfaces and send gNMI SetRequest")

	gpbSetRequest = &gpb.SetRequest{
		Update: []*gpb.Update{
			modIntfDesc(t),
		},
	}

	//Start the timer.
	start = time.Now()
	benchmarking_setup.ConfigureGNMISetRequest(t, gpbSetRequest)

	//End the timer and calculate time.
	elapsed = time.Since(start)
	t.Logf("Time taken for the gNMI SetRequest %v", elapsed)

}
