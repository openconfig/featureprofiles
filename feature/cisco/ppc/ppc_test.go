package ppc_test

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
)

var (
	chassisType                 string            // check if its distributed or fixed chassis
	tolerance                   uint64            // traffic loss tolerance percentage
	rpfoCount                   = 1               // if more than 10 then reset to 0 and reload the HW
	subscriptionCount           = 1               // number of parallel subscriptions to be tested
	multipleSubscriptionRuntime = 1 * time.Minute // duration for which parallel subscriptions will run
)

const (
	withRpfo  = true
	activeRp  = "0/RP0/CPU0"
	standbyRp = "0/RP1/CPU0"
	vrf1      = "TE"
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
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &TgnOptions{ipv4: true}),
			eventType: &eventAclConfig{aclName: "deny_all_ipv4", config: true},
		},
		{
			name:      "drop/lookup-block/state/no-route",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &TgnOptions{ipv4: true}),
			eventType: &eventInterfaceConfig{config: true, shut: true, port: sortPorts(args.dut.Ports())[1:]},
		},
		{
			name:      "drop/lookup-block/state/no-nexthop",
			flow:      args.createFlow("valid_stream", []ondatra.Endpoint{args.top.Interfaces()["ateDst"]}, &TgnOptions{ipv4: true}),
			eventType: &eventStaticRouteToNull{prefix: "202.1.0.1/32", config: true},
		},
	}

	npus := args.interfaceToNPU(t)                       // collect all the destination NPUs
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
					pre, _ := getData(t, path, query) // improve error handling
					preCounters = preCounters + pre
				}

				tgnData := float64(args.validateTrafficFlows(t, tt.flow, &TgnOptions{trafficTimer: 120, drop: true, event: tt.eventType}))

				// aggregate post counters for a path across all the destination NPUs
				for path, query := range data {
					post, _ := getData(t, path, query)
					postCounters = postCounters + post
				}

				// following reload, we can have pre data bigger than post data. So use absolute value
				want := math.Abs(float64(postCounters - preCounters)) // from DUT

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
