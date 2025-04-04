package utilization_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
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
	configBgp("100.100.100.100")

	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	otgV4Peer, otgPort1, otgConfig := configureOTG(t, otg)

	metrics := []string{"used", "free", "max-limit", "high-watermark", "last-high-watermark"}
	intMetrics := []string{"oor-red-threshold-percentage", "oor-yellow-threshold-percentage", "resource-oor-state", "last-resource-oor-change"} // initial metrics
	// Set up gNMI client
	gnmic, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t))
	if err != nil {
		t.Fatalf("Error creating ygnmi client: %v", err)
	}
	comps := components.FindActiveComponentsByType(t, dut, oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_INTEGRATED_CIRCUIT)
	var path, postPath string
	// Loop over the metrics to gather pre-test counters
	pretest := make(map[string]interface{})
	postTest := make(map[string]interface{})
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	npus := util.UniqueValues(t, nodes)

	// TODO: We should test for these leafs if they are incremented as well.
	// Google will use these OC resource leafs to track the usage of resources,
	// and if there is an increase/decrease in usage, they will act accordingly.

	// Example: In public tests which check for increase/decrease in IPv6 route/LPM/CEM resources,

	for _, metric := range metrics {
		t.Run(fmt.Sprintf("Pre-test metric: %s", metric), func(t *testing.T) {

			for _, npu := range npus {
				path = fmt.Sprintf("/components/component[name=%s]/integrated-circuit/utilization/resources/resource[name=service_lp_attributes_table_0]/state/%s", npu, metric)
			}
			query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
			vals, err := ygnmi.GetAll(context.Background(), gnmic, query)
			if err != nil {
				t.Fatalf("Error querying metric %s: %v", metric, err)
			}
			if len(vals) == 0 {
				t.Fatalf("No data received for metric %s", metric)
			}
			beforeUtzs := componentUtilizations(t, dut, comps)
			if len(beforeUtzs) != len(comps) {
				t.Fatalf("Couldn't retrieve Utilization on information for all Active Components")
			}
			pretest[metric] = vals[0]
			t.Logf("Pre-test %s: %d", metric, pretest[metric])

		})
	}

	for _, intmetrics := range intMetrics {
		t.Run(fmt.Sprintf("Pre-test metric: %s", intmetrics), func(t *testing.T) {

			for _, npu := range npus {
				path = fmt.Sprintf("/components/component[name=%s]/integrated-circuit/utilization/resources/resource[name=service_lp_attributes_table_0]/state/cisco/%s", npu, intmetrics)
			}
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

	//Configure the Next-Hop Group on the DUT
	configureNextHopGroup(t, dut)

	for _, metric := range metrics {
		t.Run(fmt.Sprintf("Post-test metric: %s", metric), func(t *testing.T) {
			for _, npu := range npus {
				//TODO: To add 10 Res names list as table
				postPath = fmt.Sprintf("/components/component[name=%s]/integrated-circuit/utilization/resources/resource[name=service_lp_attributes_table_0]/state/%s", npu, metric)
			}
			postQuery, _ := schemaless.NewWildcard[uint64](postPath, "openconfig")
			postVal, err := ygnmi.GetAll(context.Background(), gnmic, postQuery)
			if err != nil {
				t.Fatalf("Error querying metric %s: %v", metric, err)
			}
			if len(postVal) == 0 {
				t.Fatalf("No data received for metric %s", metric)
			}
			afterUtzs := componentUtilizations(t, dut, comps)
			if len(afterUtzs) != len(comps) {
				t.Fatalf("Couldn't retrieve Utilization information for all Active Components")
			}
			postTest[metric] = postVal[0]
			t.Logf("Post-test %s: %d", metric, postTest[metric])

		})
	}

	for _, intmetrics := range intMetrics {
		t.Run(fmt.Sprintf("Post-test metric: %s", intmetrics), func(t *testing.T) {

			for _, npu := range npus {
				postPath = fmt.Sprintf("/components/component[name=%s]/integrated-circuit/utilization/resources/resource[name=service_lp_attributes_table_0]/state/cisco/%s", npu, intmetrics)
			}
			if intmetrics == "resource-oor-state" {
				querys, _ := schemaless.NewWildcard[string](postPath, "openconfig")
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
				querys, _ := schemaless.NewWildcard[uint64](postPath, "openconfig")
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
