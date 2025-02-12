package utilization_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestResourceUtilization(t *testing.T) {

	t.Log("Name: OC Resource Utilization")

	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)
	configRoutePolicy(t, dut)
	configBgp(t, dut, "100.100.100.100")

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	otgV4Peer, otgPort1, otgConfig := configureOTG(t, otg)

	metrics := []string{"used", "free", "max-limit", "high-watermark", "last-high-watermark"}
	intMetrics := []string{"oor-red-threshold-percentage", "oor-yellow-threshold-percentage", "resource-oor-state", "last-resource-oor-change"}
	// Set up gNMI client
	gnmic, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t))
	if err != nil {
		t.Fatalf("Error creating ygnmi client: %v", err)
	}

	// Loop over the metrics to gather pre-test counters
	pretest := make(map[string]interface{})
	postTest := make(map[string]interface{})
	for _, metric := range metrics {
		t.Run(fmt.Sprintf("Pre-test metric: %s", metric), func(t *testing.T) {

			path := fmt.Sprintf("/components/component[name=0/7/CPU0-NPU0]/integrated-circuit/utilization/resources/resource[name=service_lp_attributes_table_0]/state/%s", metric)
			query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
			vals, err := ygnmi.GetAll(context.Background(), gnmic, query)
			if err != nil {
				t.Fatalf("Error querying metric %s: %v", metric, err)
			}
			if len(vals) == 0 {
				t.Fatalf("No data received for metric %s", metric)
			}
			pretest[metric] = vals[0]
			t.Logf("Pre-test %s: %d", metric, pretest[metric])

		})
	}

	for _, intmetrics := range intMetrics {
		t.Run(fmt.Sprintf("Pre-test metric: %s", intmetrics), func(t *testing.T) {

			path := fmt.Sprintf("/components/component[name=0/7/CPU0-NPU0]/integrated-circuit/utilization/resources/resource[name=service_lp_attributes_table_0]/state/cisco/%s", intmetrics)
			if intmetrics == "resource-oor-state" {
				query, _ := schemaless.NewWildcard[string](path, "openconfig")
				vals, err := ygnmi.GetAll(context.Background(), gnmic, query)
				if err != nil {
					t.Fatalf("Error querying metric %s: %v", intmetrics, err)
				}
				if len(vals) == 0 {
					t.Fatalf("No data received for metric %s", intmetrics)
				}
				pretest[intmetrics] = vals[0]
				t.Logf("Pre-test %s: %s", intmetrics, pretest[intmetrics])
			} else {
				query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
				vals, err := ygnmi.GetAll(context.Background(), gnmic, query)
				if err != nil {
					t.Fatalf("Error querying metric %s: %v", intmetrics, err)
				}
				if len(vals) == 0 {
					t.Fatalf("No data received for metric %s", intmetrics)
				}
				pretest[intmetrics] = vals[0]
				t.Logf("Pre-test %s: %d", intmetrics, pretest[intmetrics])
			}

		})
	}

	addRoute(t, otg, otgV4Peer, otgPort1, otgConfig)
	configureBGPWithIncrementalNetworks(t, otg, otgConfig, otgV4Peer, "100.121.1.3", 10, 1)

	// Define the Next-Hop Group
	nhgConfig := []string{
		"100.121.1.4",
		"100.121.1.5",
	}

	//Configure the Next-Hop Group on the DUT
	configureNextHopGroup(t, dut, nhgConfig)

	for _, metric := range metrics {
		t.Run(fmt.Sprintf("Post-test metric: %s", metric), func(t *testing.T) {
			postPath := fmt.Sprintf("/components/component[name=0/7/CPU0-NPU0]/integrated-circuit/utilization/resources/resource[name=service_lp_attributes_table_0]/state/%s", metric)
			postQuery, _ := schemaless.NewWildcard[uint64](postPath, "openconfig")
			postVal, err := ygnmi.GetAll(context.Background(), gnmic, postQuery)
			if err != nil {
				t.Fatalf("Error querying metric %s: %v", metric, err)
			}
			if len(postVal) == 0 {
				t.Fatalf("No data received for metric %s", metric)
			}
			postTest[metric] = postVal[0]
			t.Logf("Post-test %s: %d", metric, postTest[metric])

		})
	}

	for _, intmetrics := range intMetrics {
		t.Run(fmt.Sprintf("Post-test metric: %s", intmetrics), func(t *testing.T) {

			postpath := fmt.Sprintf("/components/component[name=0/7/CPU0-NPU0]/integrated-circuit/utilization/resources/resource[name=service_lp_attributes_table_0]/state/cisco/%s", intmetrics)
			if intmetrics == "resource-oor-state" {
				querys, _ := schemaless.NewWildcard[string](postpath, "openconfig")
				val, err := ygnmi.GetAll(context.Background(), gnmic, querys)
				if err != nil {
					t.Fatalf("Error querying metric %s: %v", intmetrics, err)
				}
				if len(val) == 0 {
					t.Fatalf("No data received for metric %s", intmetrics)
				}
				postTest[intmetrics] = val[0]
				t.Logf("Post-test %s: %s", intmetrics, postTest[intmetrics])
			} else {
				querys, _ := schemaless.NewWildcard[uint64](postpath, "openconfig")
				val, err := ygnmi.GetAll(context.Background(), gnmic, querys)
				if err != nil {
					t.Fatalf("Error querying metric %s: %v", intmetrics, err)
				}
				if len(val) == 0 {
					t.Fatalf("No data received for metric %s", intmetrics)
				}
				postTest[intmetrics] = val[0]
				t.Logf("Post-test %s: %d", intmetrics, postTest[intmetrics])
			}
		})
	}

	// Compare pre and post-test
	for _, metric := range metrics {
		if postTest[metric] != pretest[metric] {
			t.Errorf("Metric %s doesn't match FAIL: pre=%d, post=%d", metric, pretest[metric], postTest[metric])
		} else {
			t.Logf("Metric %s PASS: pre=%d, post=%d", metric, pretest[metric], postTest[metric])
		}
	}

	// Compare pre and post-test
	for _, metric := range intMetrics {
		if postTest[metric] != pretest[metric] {
			t.Errorf("Metric %s doesn't match FAIL: pre=%d, post=%d", metric, pretest[metric], postTest[metric])
		} else {
			t.Logf("Metric %s PASS: pre=%d, post=%d", metric, pretest[metric], postTest[metric])
		}
	}
	t.Run("Test for key not present", func(t *testing.T) {
		t.Log("Testing a key not present")

		// Define a path for a key not expected in GB
		path := "/components/component[name=0/7/CPU0-NPU0]/integrated-circuit/utilization/resources/resource[name=service_lp_attributes_table]/state/"
		query, _ := schemaless.NewWildcard[uint64](path, "openconfig")

		// Execute GetAll to check for an empty response
		vals, err := ygnmi.GetAll(context.Background(), gnmic, query)
		// **New Error Check**: Confirm error indicates path non-existence
		if err != nil {
			t.Log("Confirmed that the key is not present, as expected.")
			return
		}

		// Validate response is empty
		if len(vals) != 0 {
			t.Errorf("Expected no response for key not present in GB, but got: %v", vals)
		} else {
			t.Log("Received expected empty response for key not present in GB")
		}
	})
}
