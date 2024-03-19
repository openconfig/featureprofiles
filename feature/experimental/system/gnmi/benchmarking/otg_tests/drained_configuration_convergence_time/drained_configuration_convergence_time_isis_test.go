// Copyright 2023 Google LLC
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

	"github.com/openconfig/featureprofiles/feature/experimental/system/gnmi/benchmarking/internal/setup"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ygnmi/ygnmi"
)

// setISISOverloadBit is used to configure isis overload bit to true.
func setISISOverloadBit(t *testing.T, dut *ondatra.DUTDevice) {

	// ISIS Configs to set Overload Bit to true.
	dutISISPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, setup.ISISInstance).Isis()
	lspBit := dutISISPath.Global().LspBit().OverloadBit()
	gnmi.Replace(t, dut, lspBit.SetBit().Config(), true)
}

// setISISMetric is used to configure metric on isis interfaces.
func setISISMetric(t *testing.T, dut *ondatra.DUTDevice) {

	dutISISPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, setup.ISISInstance).Isis()
	t.Logf("Configure ISIS metric to %v", setup.ISISMetric)
	for _, dp := range dut.Ports() {
		intfName := dp.Name()
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			intfName = dp.Name() + ".0"
		}
		dutISISPathIntfAF := dutISISPath.Interface(intfName).Level(2).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
		if deviations.ISISRequireSameL1MetricWithL2Metric(dut) {
			b := &gnmi.SetBatch{}
			gnmi.BatchReplace(b, dutISISPathIntfAF.Metric().Config(), setup.ISISMetric)
			l1AF := dutISISPath.Interface(intfName).Level(1).Af(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
			gnmi.BatchReplace(b, l1AF.Metric().Config(), setup.ISISMetric)
			b.Set(t, dut)
		} else {
			gnmi.Replace(t, dut, dutISISPathIntfAF.Metric().Config(), setup.ISISMetric)
		}
	}
}

// verifyISISMetric is used to verify on ATE to see how much time it
// has taken to apply changes in metric
func verifyISISMetric(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {

	t.Run("ISIS Metric verification", func(t *testing.T) {
		for _, ap := range ate.Ports() {
			if ap.ID() == "port1" {
				// Port1 is ingress, skip verification on ingress port
				continue
			}

			got, ok := gnmi.WatchAll(t, ate.OTG(), gnmi.OTG().IsisRouter("devIsis"+ap.Name()).LinkStateDatabase().LspsAny().Tlvs().ExtendedIpv4Reachability().PrefixAny().Metric().State(), time.Minute, func(v *ygnmi.Value[uint32]) bool {
				metric, present := v.Val()
				if present {
					if metric == setup.ISISMetric {
						return true
					}
				}
				return false
			}).Await(t)

			metricInReceivedLsp, _ := got.Val()
			if !ok {
				t.Fatalf("Metric not matched. Expected %d got %d ", setup.ISISMetric, metricInReceivedLsp)
			}
		}
	})
}

// verifyISISOverloadBit is used to verify on ATE to see how much time it
// has taken to apply overload bit on isis adjacencies.
func verifyISISOverloadBit(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice) {

	t.Run("ISIS Overload bit verification", func(t *testing.T) {
		for _, ap := range ate.Ports() {
			if ap.ID() == "port1" {
				// port1 is ingress, skip verification on ingress port
				continue
			}

			otg := ate.OTG()
			_, ok := gnmi.WatchAll(t, otg, gnmi.OTG().IsisRouter("devIsis"+ap.Name()).LinkStateDatabase().LspsAny().Flags().State(), time.Minute, func(v *ygnmi.Value[[]otgtelemetry.E_Lsps_Flags]) bool {
				flags, present := v.Val()
				if present {
					for _, flag := range flags {
						if flag == otgtelemetry.Lsps_Flags_OVERLOAD {
							return true
						}
					}
				}
				return false
			}).Await(t)

			if !ok {
				t.Fatalf("OverLoad Bit not seen on learned lsp on ATE")
			}
		}
	})
}

// TestISISBenchmarking is to test ISIS overload bit and metric change
// applied on all isis sessions.
func TestISISBenchmarking(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	t.Log("Start timer for ISIS overload bit verification test.")
	start := time.Now()
	t.Log("Configure ISIS overload bit on DUT.")
	setISISOverloadBit(t, dut)
	t.Log("Verify on ATE if ISIS overload bit is reflected on ATE.")
	verifyISISOverloadBit(t, dut, ate)
	t.Log("End the timer and calculate time taken to apply ISIS overload bit.")
	elapsed := time.Since(start)
	t.Logf("Duration taken to apply overload bit: %v", elapsed)

	t.Log("Start timer for ISIS Metric test.")
	start = time.Now()
	t.Log("Configure ISIS Metric on DUT.")
	setISISMetric(t, dut)
	t.Log("Verify on ATE if ISIS Metric changes are reflected.")
	verifyISISMetric(t, dut, ate)
	t.Log("End the timer and calculate time taken to apply ISIS Metric.")
	elapsed = time.Since(start)
	t.Logf("Duration taken to apply isis metric: %v", elapsed)
}
