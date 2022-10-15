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
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
)

var (
	isisMed = 100
)

// setISISOverloadBit is used to configure isis overload bit to true
// using gnmi setrequest.
func setISISOverloadBit(t *testing.T) *gpb.Update {
	setBitConfig := []setup.M{
		{
			"name": "DEFAULT",
			"protocols": map[string]interface{}{
				"protocol": []setup.M{
					{
						"identifier": "ISIS",
						"name":       setup.IsisInstance,
						"isis": map[string]interface{}{
							"global": map[string]interface{}{
								"lsp-bit": map[string]interface{}{
									"overload-bit": map[string]interface{}{
										"config": map[string]interface{}{
											"set-bit": true,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	update := setup.CreateGNMIUpdate("network-instances", "network-instance", setBitConfig)
	return update
}

// setISISMetric is used to configure metric on isis interfaces using
// gnmi set request.
func setISISMetric(t *testing.T) *gpb.Update {
	dut := ondatra.DUT(t, "dut")
	var isisIntfConfig []setup.M
	for _, dp := range dut.Ports() {
		elem1 := map[string]interface{}{
			"interface-id": dp.Name(),
			"levels": map[string]interface{}{
				"level": []setup.M{
					{
						"level-number": 2,
						"afi-safi": map[string]interface{}{
							"af": []setup.M{
								{
									"afi-name":  "IPV4",
									"safi-name": "UNICAST",
									"config": map[string]interface{}{
										"afi-name":  "IPV4",
										"safi-name": "UNICAST",
										"metric":    isisMed,
										"enabled":   true,
									},
								},
							},
						},
					},
				},
			},
		}
		isisIntfConfig = append(isisIntfConfig, elem1)
	}

	setMetricConfig := []setup.M{
		{
			"name": "DEFAULT",
			"config": map[string]interface{}{
				"type": "DEFAULT_INSTANCE",
			},
			"protocols": map[string]interface{}{
				"protocol": []setup.M{
					{
						"identifier": "ISIS",
						"name":       setup.IsisInstance,
						"isis": map[string]interface{}{
							"interfaces": map[string]interface{}{
								"interface": isisIntfConfig,
							},
						},
					},
				},
			},
		},
	}

	update := setup.CreateGNMIUpdate("network-instances", "network-instance", setMetricConfig)
	return update
}

// TODO: verifyISISOverloadBit is used to verify on ATE to see how much time it
// has taken to apply overload bit on isis adjacencies.
func verifyISISOverloadBit(t *testing.T) {
	// TODO: Verify the link state database on the ATE once API support on ATE is available.
}

// TODO: verifyISISMetric is used to verify on ATE to see how much time it
// has taken to apply changes in metric
func verifyISISMetric(t *testing.T) {
	// TODO: Verify the Metric on the ATE once API support on ATE is available.
}

// TestISISBenchmarking is to test ISIS overload bit and metric change
// applied on all isis sessions.
func TestISISBenchmarking(t *testing.T) {

	// Configure Overload bit to true
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{
			setISISOverloadBit(t),
		},
	}

	// start timer
	start := time.Now()
	setup.ConfigureGNMISetRequest(t, gpbSetRequest)
	verifyISISOverloadBit(t)
	//End the timer and calculate time
	elapsed := time.Since(start)
	t.Logf("Duration taken to apply routing policy is  %v", elapsed)

	// ISIS Metric change.
	gpbSetRequest = &gpb.SetRequest{
		Update: []*gpb.Update{
			setISISMetric(t),
		},
	}

	// start timer
	start = time.Now()
	setup.ConfigureGNMISetRequest(t, gpbSetRequest)
	verifyISISMetric(t)
	//End the timer and calculate time
	elapsed = time.Since(start)
	t.Logf("Duration taken to apply routing policy is  %v", elapsed)
}
