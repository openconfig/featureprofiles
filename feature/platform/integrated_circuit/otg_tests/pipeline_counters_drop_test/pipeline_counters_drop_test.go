// Copyright 2026 Google LLC
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

package pipeline_counters_vendor_drop_test

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/functional-translators/registrar"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
	mtu           = 1500
	ppsRate       = 1000
	// trafficPackets is the number of packets sent for each drop scenario.
	trafficPackets = uint32(5000)
	// trafficTimeout bounds how long to wait for traffic to be sent and counted.
	trafficTimeout = 2 * time.Minute
	// counterSampleInterval is the SAMPLE interval used to watch natively
	// streamed vendor drop counters.
	counterSampleInterval = 10 * time.Second
	// subscribeTimeout bounds each ONCE subscription.
	subscribeTimeout = 2 * time.Minute
	// nullRoutePrefix is routed to Null0 for the L3_NULL_ADJ scenario.
	nullRoutePrefix = "203.0.113.2/32"
	nullRouteDstIP  = "203.0.113.2"
	// noRouteDstIP has no route on the DUT for the L3_ROUTE_LOOKUP_FAILED scenario.
	noRouteDstIP = "198.51.100.1"
)

var (
	dutSrc = &attrs.Attributes{
		Name:    "dutSrc",
		MAC:     "00:11:11:11:11:11",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::1",
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	ateSrc = &attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "00:22:22:22:22:22",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::2",
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	dutDst = &attrs.Attributes{
		Name:    "dutDst",
		MAC:     "00:33:33:33:33:33",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::5",
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}

	ateDst = &attrs.Attributes{
		Name:    "ateDst",
		MAC:     "00:44:44:44:44:44",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8::6",
		IPv6Len: ipv6PrefixLen,
		MTU:     mtu,
	}
)

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")

	// configure interface 1
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp1.Name()).Config(), dutSrc.NewOCInterface(dp1.Name(), dut))
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp1)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	// configure interface 2
	gnmi.Replace(t, dut, gnmi.OC().Interface(dp2.Name()).Config(), dutDst.NewOCInterface(dp2.Name(), dut))
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dp2)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")

	top := gosnappi.NewConfig()
	ateSrc.AddToOTG(top, ap1, dutSrc)
	ateDst.AddToOTG(top, ap2, dutDst)

	return top
}

// vendorDropCounters holds vendor drop counters reported under
// /components/component/integrated-circuit/pipeline-counters/drop/vendor/<vendor>/<platform>/<category>/state/<counter>,
// keyed by category and counter name and summed across integrated circuits.
type vendorDropCounters struct {
	byCategory map[string]map[string]uint64
}

// total returns the number of distinct counters reported.
func (c *vendorDropCounters) total() int {
	n := 0
	for _, counters := range c.byCategory {
		n += len(counters)
	}
	return n
}

// counter returns the summed value of all counters, in any category, whose name
// starts with any of the given name prefixes, ignoring case (e.g.
// "L3_NULL_ADJ" matches "L3_NULL_ADJ(D*)" and "l3_null_adj"), and whether any
// such counter was reported.
func (c *vendorDropCounters) counter(prefixes ...string) (uint64, bool) {
	var sum uint64
	found := false
	for _, counters := range c.byCategory {
		for name, val := range counters {
			for _, prefix := range prefixes {
				if strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
					sum += val
					found = true
					break
				}
			}
		}
	}
	return sum, found
}

// dropCounterNames lists, for each drop scenario, the counter names it may be
// reported under: the name produced by the ciscoxr-vendordrops-ft functional
// translator (.../drop/vendor/CiscoXR/spitfire/...) and the name streamed
// natively by IOS-XR 25.4.15 and later
// (.../drop/vendor/Cisco-IOS-XR-openconfig-platform-pipeline-counters-ext:cisco/<asic>/...).
var dropCounterNames = map[string][]string{
	"L3_ROUTE_LOOKUP_FAILED":           {"L3_ROUTE_LOOKUP_FAILED", "l3_drop_adj_non_inject"},
	"L3_NULL_ADJ":                      {"L3_NULL_ADJ", "l3_null_adj"},
	"MPLS_TE_MIDPOINT_LDP_LABELS_MISS": {"MPLS_TE_MIDPOINT_LDP_LABELS_MISS", "mpls_te_midpoint_ldp_labels_miss"},
}

// vendorDropOCPath returns the OpenConfig path
// /components/component[name=*]/integrated-circuit/pipeline-counters/drop/vendor.
func vendorDropOCPath() *gnmipb.Path {
	return &gnmipb.Path{
		Origin: "openconfig",
		Elem: []*gnmipb.PathElem{
			{Name: "components"},
			{Name: "component", Key: map[string]string{"name": "*"}},
			{Name: "integrated-circuit"},
			{Name: "pipeline-counters"},
			{Name: "drop"},
			{Name: "vendor"},
		},
	}
}

// readVendorDropCounters returns the vendor drop counters reported under
// /components/component/integrated-circuit/pipeline-counters/drop/vendor. By
// default the OC path is subscribed to directly on the DUT. If the
// ciscoxr_vendordrop_ft deviation is set, the counters are instead read from the
// vendor native paths and translated to the OC path by the functional
// translator.
func readVendorDropCounters(t *testing.T, dut *ondatra.DUTDevice) *vendorDropCounters {
	t.Helper()
	if ftName := deviations.CiscoxrVendordropFt(dut); ftName != "" {
		return readVendorDropCountersViaFT(t, dut, ftName)
	}
	return readVendorDropCountersNative(t, dut)
}

// subscribeOnce issues a ONCE subscription for path on the DUT and returns all
// updates received before the SyncResponse, with the notification prefix joined
// into each update path. Any subscription error fails the test.
func subscribeOnce(t *testing.T, dut *ondatra.DUTDevice, path *gnmipb.Path) []*gnmipb.Update {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), subscribeTimeout)
	defer cancel()
	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial gNMI: %v", err)
	}
	subClient, err := gnmiClient.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Failed to create subscribe client for path %v: %v", path, err)
	}
	subReq := &gnmipb.SubscribeRequest{
		Request: &gnmipb.SubscribeRequest_Subscribe{
			Subscribe: &gnmipb.SubscriptionList{
				Prefix:       &gnmipb.Path{Target: dut.Name()},
				Mode:         gnmipb.SubscriptionList_ONCE,
				Encoding:     gnmipb.Encoding_PROTO,
				Subscription: []*gnmipb.Subscription{{Path: path}},
			},
		},
	}
	t.Logf("Initiating SubscribeRequest (ONCE) for path: %v", path)
	if err := subClient.Send(subReq); err != nil {
		t.Fatalf("Failed to send subscribe request for path %v: %v", path, err)
	}

	var updates []*gnmipb.Update
	for {
		resp, err := subClient.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Subscribe to path %v failed before SyncResponse: %v", path, err)
		}
		if resp.GetSyncResponse() {
			break
		}
		notification := resp.GetUpdate()
		for _, u := range notification.GetUpdate() {
			fullPath := &gnmipb.Path{Origin: path.GetOrigin()}
			fullPath.Elem = append(fullPath.Elem, notification.GetPrefix().GetElem()...)
			fullPath.Elem = append(fullPath.Elem, u.GetPath().GetElem()...)
			u.Path = fullPath
			updates = append(updates, u)
		}
	}
	t.Logf("Collected %d leaf updates for path: %v", len(updates), path)
	return updates
}

// readVendorDropCountersNative subscribes (ONCE) to the OC vendor drop path on
// the DUT and fails the test if the DUT does not stream any leaf under it.
func readVendorDropCountersNative(t *testing.T, dut *ondatra.DUTDevice) *vendorDropCounters {
	t.Helper()
	ocPath := vendorDropOCPath()
	updates := subscribeOnce(t, dut, ocPath)
	if len(updates) == 0 {
		t.Fatalf("DUT did not stream any leaf under OC path %v", ocPath)
	}
	return parseVendorDropUpdates(t, updates)
}

// parseVendorDropUpdates extracts vendor drop counters from updates under
// /components/component[name=X]/integrated-circuit/pipeline-counters/drop/vendor/<vendor>/<platform>/<category>/state/<counter>,
// following https://github.com/openconfig/public/blob/master/doc/vendor_counter_guide.md,
// and logs the vendor, platform, categories and counter names found.
func parseVendorDropUpdates(t *testing.T, updates []*gnmipb.Update) *vendorDropCounters {
	t.Helper()
	counters, vendors, unmatched := collectVendorDropCounters(updates)
	for _, p := range unmatched[:min(len(unmatched), 5)] {
		t.Logf("Skipping update with unexpected vendor drop path: %v", p)
	}
	if len(unmatched) > 0 {
		t.Logf("Skipped %d updates with unexpected vendor drop paths", len(unmatched))
	}
	t.Logf("Vendor drop counters reported for <vendor>/<platform>: %v", sortedKeys(vendors))
	for category, cs := range counters.byCategory {
		names := make(map[string]bool)
		for name := range cs {
			names[name] = true
		}
		t.Logf("Vendor drop category %q: %d counters: %v", category, len(cs), sortedKeys(names))
	}
	return counters
}

// collectVendorDropCounters sums vendor drop counters from updates under
// .../drop/vendor/<vendor>/<platform>/<category>/state/<counter>. It also
// returns the set of <vendor>/<platform> names found and the paths of updates
// that do not match this layout.
func collectVendorDropCounters(updates []*gnmipb.Update) (*vendorDropCounters, map[string]bool, []*gnmipb.Path) {
	counters := &vendorDropCounters{byCategory: make(map[string]map[string]uint64)}
	vendors := make(map[string]bool)
	var unmatched []*gnmipb.Path
	for _, update := range updates {
		elems := update.GetPath().GetElem()
		if len(elems) != 11 ||
			elems[0].GetName() != "components" ||
			elems[2].GetName() != "integrated-circuit" ||
			elems[5].GetName() != "vendor" ||
			elems[9].GetName() != "state" {
			unmatched = append(unmatched, update.GetPath())
			continue
		}
		vendors[elems[6].GetName()+"/"+elems[7].GetName()] = true
		category := elems[8].GetName()
		if counters.byCategory[category] == nil {
			counters.byCategory[category] = make(map[string]uint64)
		}
		counters.byCategory[category][elems[10].GetName()] += update.GetVal().GetUintVal()
	}
	return counters, vendors, unmatched
}

// pathKey returns a string key uniquely identifying the given gNMI path.
func pathKey(p *gnmipb.Path) string {
	var b strings.Builder
	for _, e := range p.GetElem() {
		b.WriteString("/")
		b.WriteString(e.GetName())
		keys := make(map[string]bool)
		for k := range e.GetKey() {
			keys[k] = true
		}
		for _, k := range sortedKeys(keys) {
			fmt.Fprintf(&b, "[%s=%s]", k, e.GetKey()[k])
		}
	}
	return b.String()
}

// awaitNativeDropCounter streams (SAMPLE) the OC vendor drop path from the DUT
// and waits until the summed value of the counters matching names reaches want.
// The DUT may refresh vendor drop counters only periodically, so the counters
// are watched rather than read once. It returns the last summed value seen and
// whether want was reached within timeout. A raw gNMI subscription is used
// instead of gnmi.Watch because the vendor counter subtree under drop/vendor is
// not part of the generated OpenConfig schema.
func awaitNativeDropCounter(t *testing.T, dut *ondatra.DUTDevice, names []string, want uint64, timeout time.Duration) (uint64, bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial gNMI: %v", err)
	}
	subClient, err := gnmiClient.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Failed to create subscribe client: %v", err)
	}
	ocPath := vendorDropOCPath()
	if err := subClient.Send(&gnmipb.SubscribeRequest{
		Request: &gnmipb.SubscribeRequest_Subscribe{
			Subscribe: &gnmipb.SubscriptionList{
				Prefix:   &gnmipb.Path{Target: dut.Name()},
				Mode:     gnmipb.SubscriptionList_STREAM,
				Encoding: gnmipb.Encoding_PROTO,
				Subscription: []*gnmipb.Subscription{{
					Path:           ocPath,
					Mode:           gnmipb.SubscriptionMode_SAMPLE,
					SampleInterval: uint64(counterSampleInterval.Nanoseconds()),
				}},
			},
		},
	}); err != nil {
		t.Fatalf("Failed to send subscribe request for OC path %v: %v", ocPath, err)
	}
	t.Logf("Watching %v for counters %v to reach %d (timeout %v)", ocPath, names, want, timeout)

	latest := make(map[string]*gnmipb.Update)
	var last uint64
	for {
		resp, err := subClient.Recv()
		if err != nil {
			if ctx.Err() != nil {
				return last, false
			}
			t.Fatalf("Subscription to OC path %v failed: %v", ocPath, err)
		}
		notification := resp.GetUpdate()
		if notification == nil {
			continue
		}
		for _, u := range notification.GetUpdate() {
			fullPath := &gnmipb.Path{Origin: ocPath.GetOrigin()}
			fullPath.Elem = append(fullPath.Elem, notification.GetPrefix().GetElem()...)
			fullPath.Elem = append(fullPath.Elem, u.GetPath().GetElem()...)
			u.Path = fullPath
			latest[pathKey(fullPath)] = u
		}
		updates := make([]*gnmipb.Update, 0, len(latest))
		for _, u := range latest {
			updates = append(updates, u)
		}
		counters, _, _ := collectVendorDropCounters(updates)
		if val, ok := counters.counter(names...); ok {
			last = val
			if val >= want {
				return val, true
			}
		}
	}
}

// sortedKeys returns the keys of m in sorted order.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// readVendorDropCountersViaFT reads all vendor native input paths of the given
// functional translator and translates them to the OC vendor drop path. The test
// fails if the translator does not support the DUT software version, if any
// native input path cannot be read or returns no data, or if translation fails.
func readVendorDropCountersViaFT(t *testing.T, dut *ondatra.DUTDevice, ftName string) *vendorDropCounters {
	t.Helper()
	ft, ok := registrar.FunctionalTranslatorRegistry[ftName]
	if !ok {
		t.Fatalf("Functional translator %q not found in registry.", ftName)
	}

	// Only use the translator for the DUT software versions it was built for.
	dutVersion := strings.Split(dut.Version(), "-")[0]
	// This mirrors the translator's own metadata matching: an entry with no
	// version constraint matches any version.
	versionSupported := len(ft.Metadata()) == 0
	for _, m := range ft.Metadata() {
		switch {
		case m.SoftwareVersion == "" && m.SoftwareVersionRange == nil:
			versionSupported = true
		case m.SoftwareVersion != "":
			versionSupported = versionSupported || strings.EqualFold(m.SoftwareVersion, dutVersion)
		default:
			versionSupported = versionSupported || m.SoftwareVersionRange.Contains(dutVersion)
		}
	}
	if !versionSupported {
		var supported []string
		for _, m := range ft.Metadata() {
			switch {
			case m.SoftwareVersion != "":
				supported = append(supported, m.SoftwareVersion)
			case m.SoftwareVersionRange != nil:
				supported = append(supported, fmt.Sprintf("[%q, %q)", m.SoftwareVersionRange.InclusiveMin, m.SoftwareVersionRange.ExclusiveMax))
			}
		}
		t.Fatalf("Functional translator %q does not support DUT software version %q (supported versions: %s). If the DUT streams the OC path natively in this release, remove the ciscoxr_vendordrop_ft deviation for this version.", ftName, dutVersion, strings.Join(supported, ", "))
	}
	t.Logf("Using functional translator %q for DUT software version %q", ftName, dutVersion)

	// Subscribe to every native input path of the translator.
	var allUpdates []*gnmipb.Update
	for _, paths := range ft.OutputToInputMap() {
		for _, nativePath := range paths {
			updates := subscribeOnce(t, dut, nativePath)
			if len(updates) == 0 {
				t.Errorf("DUT did not stream any leaf for native input path %v of functional translator %q", nativePath, ftName)
			}
			allUpdates = append(allUpdates, updates...)
		}
	}
	if len(allUpdates) == 0 {
		t.Fatalf("No updates received for any native input path of functional translator %q", ftName)
	}
	t.Logf("Total leaf updates collected across all paths: %d. Calling translator...", len(allUpdates))

	translatedSR, err := ft.Translate(&gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: time.Now().UnixNano(),
				Prefix:    &gnmipb.Path{}, // Prefix is already joined into the Update paths.
				Update:    allUpdates,
			},
		},
	})
	if err != nil {
		t.Fatalf("Translation with functional translator %q failed: %v", ftName, err)
	}
	if translatedSR.GetUpdate() == nil {
		t.Fatalf("Functional translator %q returned no update", ftName)
	}
	t.Log("Successfully translated native paths. Parsing translated paths...")
	return parseVendorDropUpdates(t, translatedSR.GetUpdate().GetUpdate())
}

// sendTraffic sends trafficPackets packets of flow from ATE port1 and waits
// until the flow has stopped transmitting and the DUT port1 has received the
// packets.
func sendTraffic(t *testing.T, otg *otg.OTG, top gosnappi.Config, flow gosnappi.Flow, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Configuring and starting OTG protocols...")
	top.Flows().Clear()
	top.Flows().Append(flow)
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	t.Log("Waiting for ARP resolution...")
	otgutils.WaitForARP(t, otg, top, "IPv4")

	// Read initial DUT ingress packets on Port 1.
	dp1 := dut.Port(t, "port1")
	inPkts := gnmi.OC().Interface(dp1.Name()).Counters().InUnicastPkts().State()
	initialRx := gnmi.Get(t, dut, inPkts)
	t.Logf("DUT %s initial Rx packets: %d", dp1.Name(), initialRx)

	t.Logf("ARP resolved. Starting traffic of %d packets...", trafficPackets)
	otg.StartTraffic(t)
	_, ok := gnmi.Watch(t, otg, gnmi.OTG().Flow(flow.Name()).Transmit().State(), trafficTimeout, func(v *ygnmi.Value[bool]) bool {
		transmitting, present := v.Val()
		return present && !transmitting
	}).Await(t)
	otg.StopTraffic(t)
	if !ok {
		t.Fatalf("Flow %s did not finish transmitting within %v", flow.Name(), trafficTimeout)
	}
	if txPkts := gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()); txPkts < uint64(trafficPackets) {
		t.Fatalf("Flow %s transmitted %d packets, want %d", flow.Name(), txPkts, trafficPackets)
	}

	// Wait for the DUT ingress counter to reflect the received packets.
	finalRx, ok := gnmi.Watch(t, dut, inPkts, trafficTimeout, func(v *ygnmi.Value[uint64]) bool {
		rx, present := v.Val()
		return present && rx >= initialRx+uint64(trafficPackets)
	}).Await(t)
	if !ok {
		t.Fatalf("DUT %s did not receive %d packets within %v: last in-unicast-pkts %v (initial %d)", dp1.Name(), trafficPackets, trafficTimeout, finalRx, initialRx)
	}
	t.Log("Traffic run complete.")
}

// verifyDropCounterIncrements sends traffic from ATE port1 to dstIP and verifies
// that the vendor drop counter with the given name prefix
// increments.
func verifyDropCounterIncrements(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, top gosnappi.Config, flowName, dstIP, dropReason string) {
	t.Helper()
	before := readVendorDropCounters(t, dut)

	dp1 := dut.Port(t, "port1")
	dutMac := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().MacAddress().State())
	flow := gosnappi.NewFlow().SetName(flowName)
	flow.Metrics().SetEnable(true)
	flow.Rate().SetPps(ppsRate)
	flow.Duration().FixedPackets().SetPackets(trafficPackets)
	flow.TxRx().Port().SetTxName(ate.Port(t, "port1").ID()).SetRxName(ate.Port(t, "port2").ID())
	eth := flow.Packet().Add().Ethernet()
	eth.Src().SetValue(ateSrc.MAC)
	eth.Dst().SetValue(dutMac)
	ip := flow.Packet().Add().Ipv4()
	ip.Src().SetValue(ateSrc.IPv4)
	ip.Dst().SetValue(dstIP)

	names := dropCounterNames[dropReason]
	initial, okBefore := before.counter(names...)
	if !okBefore {
		t.Errorf("Counter %s not found in telemetry before traffic", dropReason)
	}

	sendTraffic(t, ate.OTG(), top, flow, dut)

	// The DUT refreshes natively streamed vendor drop counters periodically, so
	// wait for the counter to reflect the traffic before reading all counters.
	if okBefore && deviations.CiscoxrVendordropFt(dut) == "" {
		if val, ok := awaitNativeDropCounter(t, dut, names, initial+uint64(trafficPackets), trafficTimeout); !ok {
			t.Logf("Counter %s did not reach %d within %v (last value %d)", dropReason, initial+uint64(trafficPackets), trafficTimeout, val)
		}
	}

	after := readVendorDropCounters(t, dut)
	logIncrementedCounters(t, before, after)
	final, okAfter := after.counter(names...)
	if !okBefore || !okAfter {
		t.Fatalf("Counter %s not found in telemetry (before traffic: %t, after traffic: %t)", dropReason, okBefore, okAfter)
	}
	if final < initial+uint64(trafficPackets) {
		t.Errorf("Counter %s did not increment by at least the %d packets sent: initial %d, final %d", dropReason, trafficPackets, initial, final)
		return
	}
	t.Logf("Counter %s successfully incremented. Initial: %d, Final: %d", dropReason, initial, final)
}

// logIncrementedCounters logs every vendor drop counter that increased between
// before and after.
func logIncrementedCounters(t *testing.T, before, after *vendorDropCounters) {
	t.Helper()
	var incremented []string
	for category, counters := range after.byCategory {
		for name, val := range counters {
			if prev := before.byCategory[category][name]; val > prev {
				incremented = append(incremented, fmt.Sprintf("%s/%s: %d -> %d", category, name, prev, val))
			}
		}
	}
	sort.Strings(incremented)
	t.Logf("Vendor drop counters that incremented during traffic: %v", incremented)
}

func TestCiscoVendorDrops(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	if dut.Vendor() != ondatra.CISCO {
		t.Skip("Test only runs for Cisco vendor")
	}

	// gNMI-1.29.0 - Verify the DUT reports the vendor drop subtree.
	t.Run("VendorDropPathStreamed", func(t *testing.T) {
		t.Log("Verify that /components/component/integrated-circuit/pipeline-counters/drop/vendor reports vendor drop counters")
		if counters := readVendorDropCounters(t, dut); counters.total() == 0 {
			t.Errorf("No vendor drop counters reported under /components/component/integrated-circuit/pipeline-counters/drop/vendor")
		}
	})

	ate := ondatra.ATE(t, "ate")
	configureDUT(t, dut)
	top := configureATE(t, ate)

	// gNMI-1.29.1 - Send traffic to a destination with no route and verify
	// L3_ROUTE_LOOKUP_FAILED increments.
	t.Run("L3RouteLookupFailed", func(t *testing.T) {
		verifyDropCounterIncrements(t, dut, ate, top, "NoRouteFlow", noRouteDstIP, "L3_ROUTE_LOOKUP_FAILED")
	})

	// gNMI-1.29.2 - Send traffic to a Null0 route and verify L3_NULL_ADJ
	// increments.
	t.Run("L3NullAdj", func(t *testing.T) {
		niName := deviations.DefaultNetworkInstance(dut)
		if !deviations.ExplicitInterfaceInDefaultVRF(dut) {
			niName = "DEFAULT"
		}

		gnmi.Update(t, dut, gnmi.OC().NetworkInstance(niName).Config(), &oc.NetworkInstance{Name: &niName})

		b := &gnmi.SetBatch{}
		sV4 := &cfgplugins.StaticRouteCfg{
			NetworkInstance: niName,
			Prefix:          nullRoutePrefix,
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.LocalRouting_LOCAL_DEFINED_NEXT_HOP_DROP,
			},
		}
		if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
			t.Fatalf("Failed to configure IPv4 static route: %v", err)
		}
		b.Set(t, dut)
		t.Cleanup(func() {
			sp := gnmi.OC().NetworkInstance(niName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
			gnmi.Delete(t, dut, sp.Static(nullRoutePrefix).Config())
		})

		verifyDropCounterIncrements(t, dut, ate, top, "NullRouteFlow", nullRouteDstIP, "L3_NULL_ADJ")
	})

	// gNMI-1.29.3 - Verify MPLS_TE_MIDPOINT_LDP_LABELS_MISS is reported.
	t.Run("MPLSLabelMiss", func(t *testing.T) {
		// TODO: Trigger this drop and verify the counter increments. This
		// requires an MPLS-TE midpoint with LDP, which this testbed topology does
		// not configure.
		dropReason := "MPLS_TE_MIDPOINT_LDP_LABELS_MISS"
		val, ok := readVendorDropCounters(t, dut).counter(dropCounterNames[dropReason]...)
		if !ok {
			t.Fatalf("Counter %s not found in telemetry", dropReason)
		}
		t.Logf("Counter %s found with value: %d", dropReason, val)
	})
}
