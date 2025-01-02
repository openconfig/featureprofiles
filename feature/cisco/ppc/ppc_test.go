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

func TestShowDropsNputraps(t *testing.T) {
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
			name: "drop/state/packet-processing-aggregate",
			flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true, ttl: true}),
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

				//aggregate post counters for a path across all the destination NPUs
				for path, query := range data {
					post, err := getData(t, path, query)
					if err != nil {
						t.Fatalf("failed to get data for path %s post trigger: %v", path, err)
					}
					postCounters = postCounters + post
				}

				for _, npu := range npus {
					path := fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/%s", npu, tt.name)
					query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
					data[path] = query
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

func TestShowDropsNpudrops(t *testing.T) {

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
			name: "drop/state/packet-processing-aggregate",
			flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
		},
		{
			name: "drop/state/no-route",
			flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
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

func TestShowDropsCefipv4drops(t *testing.T) {

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
			name: "drop/state/packet-processing-aggregate",
			flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
		},
		{
			name: "drop/state/adverse-aggregate",
			flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
		},
		{
			name: "drop/state/urpf-aggregate",
			flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
		},
		{
			name: "drop/state/no-route",
			flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
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

func TestShowDropsCefipv6drops(t *testing.T) {

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
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv6: true}),
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

func TestShowDropsLptsdrops(t *testing.T) {

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
			name: "drop/state/adverse-aggregate",
			flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true, udp: true}),
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

func TestShowDropsMulticast(t *testing.T) {

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
			name: "drop/state/adverse-aggregate",
			flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
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

func TestShowDropsMac(t *testing.T) {

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
			eventType: &eventInterfaceConfig{config: true, shut: false, mtu: 500, port: sortPorts(args.dut.Ports())[1:]},
		},
		{
			name:      "drop/state/packet-processing-aggregate",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: false, mtu: 500, port: sortPorts(args.dut.Ports())[1:]},
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

func TestOcPpcDropLookupBlock(t *testing.T) {
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
			name:      "drop/lookup-block/state/acl-drops",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventAclConfig{aclName: "deny_all_ipv4", config: true},
		},
		{
			name:      "drop/lookup-block/state/no-route",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
		{
			name:      "drop/lookup-block/state/no-nexthop",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventStaticRouteToNull{prefix: "202.1.0.1/32", config: true},
		},
		{
			name:      "drop/lookup-block/state/invalid-packet",
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

func TestVendorLeafs(t *testing.T) {
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
			name:      "drop/vendor/cisco/q200/packet-processing/state/mpls_ttl_is_zero",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventEnableMplsLdp{config: true},
		},
		{
			name:      "drop/vendor/cisco/q200/packet-processing/state/ethernet_acl_drop",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventAclConfig{aclName: "deny_all_ipv4", config: true},
		},
		{
			name: "drop/vendor/cisco/q200/packet-processing/state/ipv4_uc_forwarding_disabled",
			flow: args.createFlow("valid_ipv4_flow", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
		},
		{
			name:      "drop/vendor/cisco/q200/packet-processing/state/l3_tx_mtu_failure",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: false, mtu: 500, port: sortPorts(args.dut.Ports())[1:]},
		},
		{
			name:      "drop/vendor/cisco/q200/packet-processing/state/l3_acl_drop",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
			eventType: &eventAclConfig{aclName: "deny_all_ipv4", config: true},
		},
		{
			name: "drop/vendor/cisco/q200/congestion/state/exact_meter_packet_got_dropped_due_to_exact_meter",
			flow: args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &tgnOptions{ipv4: true}),
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
					gnmic, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t))
					if err != nil {
						t.Fatalf("Error creating ygnmi client: %v", err)
					}

					vals, err := ygnmi.GetAll(context.Background(), gnmic, query)
					if err != nil {
						t.Fatalf("Error subscribing to /vendor/cisco: %v", err)
					}

					if len(vals) == 0 {
						t.Fatalf("Did not receive a response for /vendor/cisco")
					}
					t.Logf("VAls: %d", vals)
					preCounters = preCounters + vals[0]
				}

				tgnData := float64(args.validateTrafficFlows(t, tt.flow, &tgnOptions{trafficTimer: 120, drop: true, event: tt.eventType}))

				for _, npu := range npus {
					path := fmt.Sprintf("/components/component[name=%s]/integrated-circuit/pipeline-counters/%s", npu, tt.name)
					query, _ := schemaless.NewWildcard[uint64](path, "openconfig")
					data[path] = query
					gnmic, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t))
					if err != nil {
						t.Fatalf("Error creating ygnmi client: %v", err)
					}

					post, err := ygnmi.GetAll(context.Background(), gnmic, query)
					if err != nil {
						t.Fatalf("Error subscribing to /vendor/cisco: %v", err)
					}

					if len(post) == 0 {
						t.Fatalf("Did not receive a response for /vendor/cisco")
					}
					t.Logf("VAls: %d", post)
					postCounters = postCounters + post[0]
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
