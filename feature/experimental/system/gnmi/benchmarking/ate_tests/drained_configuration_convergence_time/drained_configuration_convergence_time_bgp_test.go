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

// drained_configuration_convergence_time_test is used to verify bgp test scenarios
// as given in gnmi1.3 testcase.
package drained_configuration_convergence_time_test

import (
	"testing"

	"github.com/openconfig/featureprofiles/feature/experimental/system/gnmi/benchmarking/ate_tests/internal/setup"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestEstablish is to configure Interface, BGP and ISIS configurations
// on DUT using gnmi set request. It also verifies for bgp and isis adjacencies.
func TestEstablish(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	setup.BuildIPPool(t)

	t.Log("Configure Network Instance type to DEFAULT on DUT")
	dutConfNIPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance)
	gnmi.Replace(t, dut, dutConfNIPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)

	t.Log("Cleanup exisitng bgp and isis configs on DUT before configuring test configs")
	dutBGPPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()
	gnmi.Delete(t, dut, dutBGPPath.Config())
	dutISISPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, setup.IsisInstance).Isis()
	gnmi.Delete(t, dut, dutISISPath.Config())

	t.Log("Configure BGP and ISIS test configs")
	setup.BuildOCUpdate(t)

	t.Log("Coonfigure ATE with Interfaces, BGP, ISIS configs")
	ate := ondatra.ATE(t, "ate")
	setup.ConfigureATE(t, ate)

	t.Log("Verify BGP Session state , should be in ESTABLISHED State")
	setup.VerifyBgpTelemetry(t, dut)

	t.Log("Verify ISIS adjacency state, should be UP")
	setup.VerifyISISTelemetry(t, dut)
}
