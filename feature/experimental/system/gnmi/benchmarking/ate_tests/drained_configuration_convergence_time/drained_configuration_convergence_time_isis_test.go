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

// drained_configuration_convergence_time_test is used to verify isis test scenarios
// as given in gnmi 1.3 testcase
package drained_configuration_convergence_time_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/feature/experimental/system/gnmi/benchmarking/ate_tests/internal/setup"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

// setISISMetric is used to configure metric on isis interfaces
func setISISMetric(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dutISISPath := gnmi.OC().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, setup.IsisInstance).Isis()

	t.Logf("Configure ISIS metric to %v", setup.ISISMetric)
	for _, dp := range dut.Ports() {
		dutISISPathIntfAF := dutISISPath.Interface(dp.Name()).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		gnmi.Replace(t, dut, dutISISPathIntfAF.Metric().Config(), setup.ISISMetric)
	}
}

// verifyISISMetric is used to verify on ATE to see how much time it
// has taken to apply changes in metric
func verifyISISMetric(t *testing.T) {
	ate := ondatra.ATE(t, "ate")

	t.Run("ISIS Metric verification", func(t *testing.T) {
		at := gnmi.OC()
		for _, ap := range ate.Ports() {
			if ap.ID() == "port1" {
				// Port1 is ingress, skip verification on ingress port
				continue
			}

			const want = oc.Interface_OperStatus_UP

			if got := gnmi.Get(t, ate, at.Interface(ap.Name()).OperStatus().State()); got != want {
				t.Errorf("%s oper-status got %v, want %v", ap, got, want)
			}
			is := at.NetworkInstance(ap.Name()).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "0").Isis()
			lsps := is.LevelAny().LspAny()
			metric := gnmi.GetAll(t, ate, lsps.Tlv(oc.IsisLsdbTypes_ISIS_TLV_TYPE_EXTENDED_IPV4_REACHABILITY).ExtendedIpv4Reachability().PrefixAny().Metric().State())

			if diff := cmp.Diff(setup.ISISMetricArray, metric); diff != "" {
				t.Errorf("obtained Metric on ATE is not as expected, got %v, want %v", metric, setup.ISISMetricArray)
			}

		}
	})
}

// TestISISBenchmarking is to test ISIS overload bit and metric change
// applied on all isis sessions.
func TestISISBenchmarking(t *testing.T) {

	t.Log("Start timer")
	start := time.Now()
	t.Log("Configure ISIS Metric on DUT")
	setISISMetric(t)
	t.Log("Verify on ATE if ISIS Metric changes are reflected")
	verifyISISMetric(t)
	t.Log("End the timer and calculate time taken to apply ISIS Metric")
	elapsed := time.Since(start)
	t.Logf("Duration taken to apply isis metric  %v", elapsed)
}
