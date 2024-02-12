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

	"github.com/openconfig/featureprofiles/feature/experimental/system/gnmi/benchmarking/internal/setup"
	"github.com/openconfig/featureprofiles/internal/args"
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

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

// modifyIntfDescription builds config to modify description of a subset of interfaces.
func modifyIntfDescription(dut *ondatra.DUTDevice) *oc.Root {
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

// TestGnmiFullConfigReplace measures performance of gNMI configuration replace.
func TestGnmiFullConfigReplace(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Log("Configure network instance on DUT")
	dutConfNIPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	t.Log("Cleanup exisitng BGP and ISIS configs on DUT before configuring test configs")
	dutBGPPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	gnmi.Delete(t, dut, dutBGPPath.Config())
	dutISISPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, setup.ISISInstance).Isis()
	gnmi.Delete(t, dut, dutISISPath.Config())

	confP := gnmi.OC()

	t.Logf("Build interfaces, ISIS and BGP protocols configuration for benchmarking")
	d1 := setup.BuildBenchmarkingConfig(t)

	t.Run("Benchmark full configuration replace", func(t *testing.T) {
		// Start the timer.
		start := time.Now()
		gnmi.Update(t, dut, confP.Config(), d1)

		// End the timer and calculate time requied to apply the config on DUT.
		elapsed := time.Since(start)

		if *args.FullConfigReplaceTime > 0 && elapsed > *args.FullConfigReplaceTime {
			t.Errorf("Time taken for full configuration replace is more than the expected benchmark value: want %v got %v", *args.FullConfigReplaceTime, elapsed)
		} else {
			t.Logf("Time taken for full configuration replace: %v", elapsed)
		}
	})

	t.Run("Benchmark modifying a subset of configuration", func(t *testing.T) {
		d2 := modifyIntfDescription(dut)
		fptest.LogQuery(t, "DUT", confP.Config(), d2)

		// Start the timer.
		start := time.Now()
		gnmi.Update(t, dut, confP.Config(), d2)

		// End the timer and calculate time.
		elapsed := time.Since(start)

		if *args.SubsetConfigReplaceTime > 0 && elapsed > *args.SubsetConfigReplaceTime {
			t.Errorf("Time taken to modify a subset of configuration is more than the expected benchmark value: want %v got %v", *args.SubsetConfigReplaceTime, elapsed)
		} else {
			t.Logf("Time taken to modify a subset of configuration: %v", elapsed)
		}
	})
}
