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

	"github.com/openconfig/featureprofiles/feature/experimental/system/gnmi/benchmarking/ate_tests/internal/setup"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
)

const (
	isisMed = 100
)

// setISISOverloadBit is used to configure isis overload bit to true
func setISISOverloadBit(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// ISIS Configs to set OVerload Bit to true
	dutISISPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, setup.IsisInstance).Isis()
	lspBit := dutISISPath.Global().LspBit().OverloadBit()
	lspBit.SetBit().Replace(t, true)
}

// setISISMetric is used to configure metric on isis interfaces
func setISISMetric(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dutISISPath := dut.Config().NetworkInstance(*deviations.DefaultNetworkInstance).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, setup.IsisInstance).Isis()

	// Set ISIS metric to 100
	for _, dp := range dut.Ports() {
		dutISISPathIntfAF := dutISISPath.Interface(dp.Name()).Level(2).Af(telemetry.IsisTypes_AFI_TYPE_IPV4, telemetry.IsisTypes_SAFI_TYPE_UNICAST)
		dutISISPathIntfAF.Metric().Replace(t, isisMed)
	}
}

// TODO: verifyISISOverloadBit is used to verify on ATE to see how much time it
// has taken to apply overload bit on isis adjacencies.
// https://github.com/openconfig/ondatra/issues/51
func verifyISISOverloadBit(t *testing.T) {
	ate := ondatra.ATE(t, "ate")

	t.Run("ISIS Overload bit verification", func(t *testing.T) {
		at := ate.Telemetry()
		for _, ap := range ate.Ports() {
			if ap.ID() == "port1" {
				//port1 is ingress, skip verification on ingress port
				continue
			}

			const want = telemetry.Interface_OperStatus_UP

			if got := at.Interface(ap.Name()).OperStatus().Get(t); got != want {
				t.Errorf("%s oper-status got %v, want %v", ap, got, want)
			}
			// https://github.com/openconfig/ondatra/issues/51
			/*is := at.NetworkInstance(ap.Name()).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "0").Isis()
			seq1 := is.LevelAny().LspAny().Tlv(telemetry.IsisLsdbTypes_ISIS_TLV_TYPE_IS_NEIGHBOR_ATTRIBUTE).Get(t)
			fmt.Println(seq1)*/

		}
	})

}

// TODO: verifyISISMetric is used to verify on ATE to see how much time it
// has taken to apply changes in metric
// https://github.com/openconfig/ondatra/issues/51
func verifyISISMetric(t *testing.T) {
	ate := ondatra.ATE(t, "ate")

	t.Run("ISIS Overload bit verification", func(t *testing.T) {
		at := ate.Telemetry()
		for _, ap := range ate.Ports() {
			if ap.ID() == "port1" {
				//port1 is ingress, skip verification on ingress port
				continue
			}

			const want = telemetry.Interface_OperStatus_UP

			if got := at.Interface(ap.Name()).OperStatus().Get(t); got != want {
				t.Errorf("%s oper-status got %v, want %v", ap, got, want)
			}

			// https://github.com/openconfig/ondatra/issues/51
			/*is := at.NetworkInstance(ap.Name()).Protocol(telemetry.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "0").Isis()
			seq := is.LevelAny().LspAny().SequenceNumber().Get(t)
			fmt.Println(seq)*/

		}
	})

}

// TestISISBenchmarking is to test ISIS overload bit and metric change
// applied on all isis sessions.
func TestISISBenchmarking(t *testing.T) {

	// start timer
	start := time.Now()
	setISISOverloadBit(t)
	verifyISISOverloadBit(t)
	//End the timer and calculate time
	elapsed := time.Since(start)
	t.Logf("Duration taken to apply overload bit  %v", elapsed)

	// start timer
	start = time.Now()
	setISISMetric(t)
	verifyISISMetric(t)
	//End the timer and calculate time
	elapsed = time.Since(start)
	t.Logf("Duration taken to apply isis metric  %v", elapsed)
}
