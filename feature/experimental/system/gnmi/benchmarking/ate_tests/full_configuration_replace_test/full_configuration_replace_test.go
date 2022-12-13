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

	"github.com/openconfig/featureprofiles/feature/experimental/system/gnmi/benchmarking/ate_tests/internal/setup"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
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
func modIntfDesc(t *testing.T) *oc.Root {
	dut := ondatra.DUT(t, "dut")
	d := &oc.Root{}
	dutPorts := sortPorts(dut.Ports())

	for i := 0; i < len(dutPorts); i++ {
		if i%2 == 0 {
			i := d.GetOrCreateInterface(dutPorts[i].Name())
			i.Description = ygot.String("modified via oc")
		}
	}

	return d
}

func TestGnmiFullConfigReplace(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Build pool of ip addresses to configure DUT interfaces
	setup.BuildIPPool(t)

	t.Log("Configure Network Instance type to DEFAULT on DUT")
	dutConfNIPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance)
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	t.Log("Cleanup exisitng bgp and isis configs on DUT before configuring test configs")
	dutBGPPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	gnmi.Delete(t, dut, dutBGPPath.Config())
	dutISISPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, setup.IsisInstance).Isis()
	gnmi.Delete(t, dut, dutISISPath.Config())

	t.Logf("Build interfaces, ISIS and BGP protocols configuration and send gNMI Set request")
	setup.BuildOCUpdate(t)

	t.Logf("Modify description of a subset of interfaces and send gNMI Set request")
	d2 := modIntfDesc(t)
	confP := gnmi.OC()

	fptest.LogQuery(t, "DUT", confP.Config(), d2)
	//Start the timer.
	start := time.Now()

	//conf.Update(t, d2)
	gnmi.Update(t, dut, confP.Config(), d2)

	//End the timer and calculate time.
	elapsed := time.Since(start)
	t.Logf("Time taken for gNMI Set request is: %v", elapsed)

}
