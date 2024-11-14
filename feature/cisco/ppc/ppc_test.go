package ppc_test

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
)

var (
	tolerance uint64 // traffic loss tolerance percentage
)

const (
	vrf1 = "TE"
)

type testArgs struct {
	ate *ondatra.ATEDevice
	ctx context.Context
	dut *ondatra.DUTDevice
	top *ondatra.ATETopology
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestValidateShowDrops_nputraps(t *testing.T) {
	t.Log("Name: OC PPC")

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	var vrfs = []string{vrf1}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	configBasePBR(t, dut, "TE", "ipv4", 1, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	configRoutePolicy(t, dut)
	configIsis(t, dut, []string{"Bundle-Ether121", "Bundle-Ether122"})
	configBgp(t, dut, "100.100.100.100")

	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	configAteRoutingProtocols(t, top)
	time.Sleep(120 * time.Second) // sleep is for protocols to start and stabilize on ATE

	args := &testArgs{
		dut: dut,
		ate: ate,
		top: top,
		ctx: ctx,
	}
	testcases := []Testcase{
		{
			name:      "drop/state/packet-processing-aggregate",
			flow:      args.createFlow("valid_ipv4_flow", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventZeroTtl{zeroTtlTrafficFlow: true},
		},
	}
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	npus := util.UniqueValues(t, nodes)                  // a list of unique NPU ID strings
	data := make(map[string]ygnmi.WildcardQuery[uint64]) // hold a path and its query information
	for _, tt := range testcases {
		// loop over different streaming modes
		for _, subMode := range []gpb.SubscriptionMode{gpb.SubscriptionMode_SAMPLE} {
			t.Run(fmt.Sprintf("Test path %v in subscription mode %v", tt.name, subMode), func(t *testing.T) {
				t.Logf("Path name: %s", tt.name)
				var preCounters, postCounters = uint64(0), uint64(0)
				tolerance = 2.0 // 2% change tolerance is allowed between want and got value

				// collecting each path, query per destination NPU
				for _, npu := range npus {
					path := fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/%s", npu, tt.name)
					query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
					data[path] = query
				}

				// aggregate pre counters for a path across all the destination NPUs
				for path, query := range data {
					pre, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s pre trigger: %v", path, err)
					}
					preCounters = preCounters + pre
				}

				tgnData := float64(args.validateTrafficFlows(t, tt.flow, &tgnOptions{trafficTimer: 120, drop: true, event: tt.eventType}))

				// aggregate post counters for a path across all the destination NPUs
				for path, query := range data {
					post, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s post trigger: %v", path, err)
					}
					postCounters = postCounters + post
				}

				// following reload, we can have pre data bigger than post data on the DUT. So use absolute value
				want := math.Abs(float64(postCounters - preCounters))

				t.Logf("Initial counters for path %s : %d", tt.name, preCounters)
				t.Logf("Final counters for path %s: %d", tt.name, postCounters)
				t.Logf("Expected counters for path %s: %d", tt.name, uint64(want))

				if (math.Abs(tgnData-want)/(tgnData))*100 > float64(tolerance) {
					t.Errorf("Data doesn't match for path %s, got: %f, want: %f", tt.name, tgnData, want)
				} else {
					t.Logf("PASS: Data for path %s, got: %f, want: %f", tt.name, tgnData, want)
				}
			})
		}
	}
}

func TestValidateShowDrops_cefipv6drops(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	var vrfs = []string{vrf1}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	configBasePBR(t, dut, "TE", "ipv4", 1, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	configRoutePolicy(t, dut)
	configIsis(t, dut, []string{"Bundle-Ether121", "Bundle-Ether122"})
	configBgp(t, dut, "100.100.100.100")

	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	configAteRoutingProtocols(t, top)
	time.Sleep(120 * time.Second) // sleep is for protocols to start and stabilize on ATE

	args := &testArgs{
		dut: dut,
		ate: ate,
		top: top,
		ctx: ctx,
	}
	testcases := []Testcase{
		{
			name:      "drop/state/adverse-aggregate",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
		{
			name:      "drop/state/packet-processing-aggregate",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
		{
			name:      "drop/state/no-route",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
		{
			name:      "drop/state/vendor/packet-processing/state/ipv4_uc_forwarding_disabled",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
	}
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	npus := util.UniqueValues(t, nodes)                  // a list of unique NPU ID strings
	data := make(map[string]ygnmi.WildcardQuery[uint64]) // hold a path and its query information
	for _, tt := range testcases {
		// loop over different streaming modes
		for _, subMode := range []gpb.SubscriptionMode{gpb.SubscriptionMode_SAMPLE} {
			t.Run(fmt.Sprintf("Test path %v in subscription mode %v", tt.name, subMode), func(t *testing.T) {
				t.Logf("Path name: %s", tt.name)
				var preCounters, postCounters = uint64(0), uint64(0)
				tolerance = 2.0 // 2% change tolerance is allowed between want and got value

				// collecting each path, query per destination NPU
				for _, npu := range npus {
					path := fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/%s", npu, tt.name)
					query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
					data[path] = query
				}

				// aggregate pre counters for a path across all the destination NPUs
				for path, query := range data {
					pre, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s pre trigger: %v", path, err)
					}
					preCounters = preCounters + pre
				}

				tgnData := float64(args.validateTrafficFlows(t, tt.flow, &tgnOptions{trafficTimer: 120, drop: true, event: tt.eventType}))

				// aggregate post counters for a path across all the destination NPUs
				for path, query := range data {
					post, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s post trigger: %v", path, err)
					}
					postCounters = postCounters + post
				}

				// following reload, we can have pre data bigger than post data on the DUT. So use absolute value
				want := math.Abs(float64(postCounters - preCounters))

				t.Logf("Initial counters for path %s : %d", tt.name, preCounters)
				t.Logf("Final counters for path %s: %d", tt.name, postCounters)
				t.Logf("Expected counters for path %s: %d", tt.name, uint64(want))

				if (math.Abs(tgnData-want)/(tgnData))*100 > float64(tolerance) {
					t.Errorf("Data doesn't match for path %s, got: %f, want: %f", tt.name, tgnData, want)
				} else {
					t.Logf("PASS: Data for path %s, got: %f, want: %f", tt.name, tgnData, want)
				}
			})
		}
	}
}

func TestValidateShowDrops_mac(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	var vrfs = []string{vrf1}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	configBasePBR(t, dut, "TE", "ipv4", 1, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	configRoutePolicy(t, dut)
	configIsis(t, dut, []string{"Bundle-Ether121", "Bundle-Ether122"})
	configBgp(t, dut, "100.100.100.100")

	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	configAteRoutingProtocols(t, top)
	time.Sleep(120 * time.Second) // sleep is for protocols to start and stabilize on ATE

	args := &testArgs{
		dut: dut,
		ate: ate,
		top: top,
		ctx: ctx,
	}
	testcases := []Testcase{
		{
			name:      "drop/state/congestion-aggregate",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
		{
			name:      "drop/state/packet-processing-aggregate",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
	}
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	npus := util.UniqueValues(t, nodes)                  // a list of unique NPU ID strings
	data := make(map[string]ygnmi.WildcardQuery[uint64]) // hold a path and its query information
	for _, tt := range testcases {
		// loop over different streaming modes
		for _, subMode := range []gpb.SubscriptionMode{gpb.SubscriptionMode_SAMPLE} {
			t.Run(fmt.Sprintf("Test path %v in subscription mode %v", tt.name, subMode), func(t *testing.T) {
				t.Logf("Path name: %s", tt.name)
				var preCounters, postCounters = uint64(0), uint64(0)
				tolerance = 2.0 // 2% change tolerance is allowed between want and got value

				// collecting each path, query per destination NPU
				for _, npu := range npus {
					path := fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/%s", npu, tt.name)
					query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
					data[path] = query
				}

				// aggregate pre counters for a path across all the destination NPUs
				for path, query := range data {
					pre, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s pre trigger: %v", path, err)
					}
					preCounters = preCounters + pre
				}

				tgnData := float64(args.validateTrafficFlows(t, tt.flow, &tgnOptions{trafficTimer: 120, drop: true, event: tt.eventType}))

				// aggregate post counters for a path across all the destination NPUs
				for path, query := range data {
					post, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s post trigger: %v", path, err)
					}
					postCounters = postCounters + post
				}

				// following reload, we can have pre data bigger than post data on the DUT. So use absolute value
				want := math.Abs(float64(postCounters - preCounters))

				t.Logf("Initial counters for path %s : %d", tt.name, preCounters)
				t.Logf("Final counters for path %s: %d", tt.name, postCounters)
				t.Logf("Expected counters for path %s: %d", tt.name, uint64(want))

				if (math.Abs(tgnData-want)/(tgnData))*100 > float64(tolerance) {
					t.Errorf("Data doesn't match for path %s, got: %f, want: %f", tt.name, tgnData, want)
				} else {
					t.Logf("PASS: Data for path %s, got: %f, want: %f", tt.name, tgnData, want)
				}
			})
		}
	}
}

func TestValidateShowDrops_npudrops(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	var vrfs = []string{vrf1}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	configBasePBR(t, dut, "TE", "ipv4", 1, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	configRoutePolicy(t, dut)
	configIsis(t, dut, []string{"Bundle-Ether121", "Bundle-Ether122"})
	configBgp(t, dut, "100.100.100.100")

	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	configAteRoutingProtocols(t, top)
	time.Sleep(120 * time.Second) // sleep is for protocols to start and stabilize on ATE

	args := &testArgs{
		dut: dut,
		ate: ate,
		top: top,
		ctx: ctx,
	}
	testcases := []Testcase{
		{
			name:      "drop/state/packet-processing-aggregate",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
		{
			name:      "drop/state/no-route",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
	}
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	npus := util.UniqueValues(t, nodes)                  // a list of unique NPU ID strings
	data := make(map[string]ygnmi.WildcardQuery[uint64]) // hold a path and its query information
	for _, tt := range testcases {
		// loop over different streaming modes
		for _, subMode := range []gpb.SubscriptionMode{gpb.SubscriptionMode_SAMPLE} {
			t.Run(fmt.Sprintf("Test path %v in subscription mode %v", tt.name, subMode), func(t *testing.T) {
				t.Logf("Path name: %s", tt.name)
				var preCounters, postCounters = uint64(0), uint64(0)
				tolerance = 2.0 // 2% change tolerance is allowed between want and got value

				// collecting each path, query per destination NPU
				for _, npu := range npus {
					path := fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/%s", npu, tt.name)
					query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
					data[path] = query
				}

				// aggregate pre counters for a path across all the destination NPUs
				for path, query := range data {
					pre, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s pre trigger: %v", path, err)
					}
					preCounters = preCounters + pre
				}

				tgnData := float64(args.validateTrafficFlows(t, tt.flow, &tgnOptions{trafficTimer: 120, drop: true, event: tt.eventType}))

				// aggregate post counters for a path across all the destination NPUs
				for path, query := range data {
					post, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s post trigger: %v", path, err)
					}
					postCounters = postCounters + post
				}

				// following reload, we can have pre data bigger than post data on the DUT. So use absolute value
				want := math.Abs(float64(postCounters - preCounters))

				t.Logf("Initial counters for path %s : %d", tt.name, preCounters)
				t.Logf("Final counters for path %s: %d", tt.name, postCounters)
				t.Logf("Expected counters for path %s: %d", tt.name, uint64(want))

				if (math.Abs(tgnData-want)/(tgnData))*100 > float64(tolerance) {
					t.Errorf("Data doesn't match for path %s, got: %f, want: %f", tt.name, tgnData, want)
				} else {
					t.Logf("PASS: Data for path %s, got: %f, want: %f", tt.name, tgnData, want)
				}
			})
		}
	}
}

func TestValidateShowDrops_cefipv4drops(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	var vrfs = []string{vrf1}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	configBasePBR(t, dut, "TE", "ipv4", 1, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	configRoutePolicy(t, dut)
	configIsis(t, dut, []string{"Bundle-Ether121", "Bundle-Ether122"})
	configBgp(t, dut, "100.100.100.100")

	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	configAteRoutingProtocols(t, top)
	time.Sleep(120 * time.Second) // sleep is for protocols to start and stabilize on ATE

	args := &testArgs{
		dut: dut,
		ate: ate,
		top: top,
		ctx: ctx,
	}
	testcases := []Testcase{
		{
			name:      "drop/state/packet-processing-aggregate",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
	}
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	npus := util.UniqueValues(t, nodes)                  // a list of unique NPU ID strings
	data := make(map[string]ygnmi.WildcardQuery[uint64]) // hold a path and its query information
	for _, tt := range testcases {
		// loop over different streaming modes
		for _, subMode := range []gpb.SubscriptionMode{gpb.SubscriptionMode_SAMPLE} {
			t.Run(fmt.Sprintf("Test path %v in subscription mode %v", tt.name, subMode), func(t *testing.T) {
				t.Logf("Path name: %s", tt.name)
				var preCounters, postCounters = uint64(0), uint64(0)
				tolerance = 2.0 // 2% change tolerance is allowed between want and got value

				// collecting each path, query per destination NPU
				for _, npu := range npus {
					path := fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/%s", npu, tt.name)
					query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
					data[path] = query
				}

				// aggregate pre counters for a path across all the destination NPUs
				for path, query := range data {
					pre, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s pre trigger: %v", path, err)
					}
					preCounters = preCounters + pre
				}

				tgnData := float64(args.validateTrafficFlows(t, tt.flow, &tgnOptions{trafficTimer: 120, drop: true, event: tt.eventType}))

				// aggregate post counters for a path across all the destination NPUs
				for path, query := range data {
					post, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s post trigger: %v", path, err)
					}
					postCounters = postCounters + post
				}

				// following reload, we can have pre data bigger than post data on the DUT. So use absolute value
				want := math.Abs(float64(postCounters - preCounters))

				t.Logf("Initial counters for path %s : %d", tt.name, preCounters)
				t.Logf("Final counters for path %s: %d", tt.name, postCounters)
				t.Logf("Expected counters for path %s: %d", tt.name, uint64(want))

				if (math.Abs(tgnData-want)/(tgnData))*100 > float64(tolerance) {
					t.Errorf("Data doesn't match for path %s, got: %f, want: %f", tt.name, tgnData, want)
				} else {
					t.Logf("PASS: Data for path %s, got: %f, want: %f", tt.name, tgnData, want)
				}
			})
		}
	}
}

func TestValidateShowDrops_lptsdrops(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	ctx := context.Background()
	var vrfs = []string{vrf1}
	configVRF(t, dut, vrfs)
	configureDUT(t, dut)
	configBasePBR(t, dut, "TE", "ipv4", 1, "pbr", oc.PacketMatchTypes_IP_PROTOCOL_IP_IN_IP, []uint8{})
	configRoutePolicy(t, dut)
	configIsis(t, dut, []string{"Bundle-Ether121", "Bundle-Ether122"})
	configBgp(t, dut, "100.100.100.100")

	ate := ondatra.ATE(t, "ate")
	top := configureATE(t, ate)
	configAteRoutingProtocols(t, top)
	time.Sleep(120 * time.Second) // sleep is for protocols to start and stabilize on ATE

	args := &testArgs{
		dut: dut,
		ate: ate,
		top: top,
		ctx: ctx,
	}
	testcases := []Testcase{
		{
			name:      "drop/state/adverse-aggregate",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
	}
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	npus := util.UniqueValues(t, nodes)                  // a list of unique NPU ID strings
	data := make(map[string]ygnmi.WildcardQuery[uint64]) // hold a path and its query information
	for _, tt := range testcases {
		// loop over different streaming modes
		for _, subMode := range []gpb.SubscriptionMode{gpb.SubscriptionMode_SAMPLE} {
			t.Run(fmt.Sprintf("Test path %v in subscription mode %v", tt.name, subMode), func(t *testing.T) {
				t.Logf("Path name: %s", tt.name)
				var preCounters, postCounters = uint64(0), uint64(0)
				tolerance = 2.0 // 2% change tolerance is allowed between want and got value

				// collecting each path, query per destination NPU
				for _, npu := range npus {
					path := fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/%s", npu, tt.name)
					query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
					data[path] = query
				}

				// aggregate pre counters for a path across all the destination NPUs
				for path, query := range data {
					pre, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s pre trigger: %v", path, err)
					}
					preCounters = preCounters + pre
				}

				tgnData := float64(args.validateTrafficFlows(t, tt.flow, &tgnOptions{trafficTimer: 120, drop: true, event: tt.eventType}))

				// aggregate post counters for a path across all the destination NPUs
				for path, query := range data {
					post, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s post trigger: %v", path, err)
					}
					postCounters = postCounters + post
				}

				// following reload, we can have pre data bigger than post data on the DUT. So use absolute value
				want := math.Abs(float64(postCounters - preCounters))

				t.Logf("Initial counters for path %s : %d", tt.name, preCounters)
				t.Logf("Final counters for path %s: %d", tt.name, postCounters)
				t.Logf("Expected counters for path %s: %d", tt.name, uint64(want))

				if (math.Abs(tgnData-want)/(tgnData))*100 > float64(tolerance) {
					t.Errorf("Data doesn't match for path %s, got: %f, want: %f", tt.name, tgnData, want)
				} else {
					t.Logf("PASS: Data for path %s, got: %f, want: %f", tt.name, tgnData, want)
				}
			})
		}
	}
}
