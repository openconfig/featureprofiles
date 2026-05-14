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
	flowDuration  = 5 * time.Second
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

func readVendorDropCounters(t *testing.T, dut *ondatra.DUTDevice) map[string]uint64 {
	ctx := context.Background()
	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial gNMI: %v", err)
	}

	ftName := deviations.CiscoxrVendordropFt(dut)
	if ftName == "" {
		t.Fatalf("CiscoxrVendordropFt deviation is not set. Cannot run vendor-specific drop counter test without it.")
	}

	ft, ok := registrar.FunctionalTranslatorRegistry[ftName]
	if !ok {
		t.Fatalf("Functional translator %q not found in registry.", ftName)
	}

	var nativePaths []*gnmipb.Path
	for _, paths := range ft.OutputToInputMap() {
		for _, p := range paths {
			isAdversePath := false
			for _, e := range p.GetElem() {
				if e.GetName() == "asic-statistics-detail-for-npu-ids" {
					isAdversePath = true
					break
				}
			}

			if isAdversePath {
				continue // Skip adverse paths
			}

			nativePaths = append(nativePaths, p)
		}
	}

	if len(nativePaths) == 0 {
		t.Fatalf("No native paths found for functional translator %q", ftName)
	}

	var allUpdates []*gnmipb.Update

	for _, nativePath := range nativePaths {
		t.Logf("Initiating SubscribeRequest (ONCE) for native path: %v", nativePath)
		subReq := &gnmipb.SubscribeRequest{
			Request: &gnmipb.SubscribeRequest_Subscribe{
				Subscribe: &gnmipb.SubscriptionList{
					Prefix:   &gnmipb.Path{Target: dut.Name()},
					Mode:     gnmipb.SubscriptionList_ONCE,
					Encoding: gnmipb.Encoding_PROTO,
					Subscription: []*gnmipb.Subscription{
						{Path: nativePath},
					},
				},
			},
		}

		subCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		subClient, err := gnmiClient.Subscribe(subCtx)
		if err != nil {
			t.Logf("Failed to create subscribe client for path %v: %v", nativePath, err)
			cancel()
			continue
		}
		if err := subClient.Send(subReq); err != nil {
			t.Logf("Failed to send subscribe request for path %v: %v", nativePath, err)
			cancel()
			continue
		}

		t.Logf("Entering Recv() loop for path: %v", nativePath)
		updateCount := 0
		for {
			resp, err := subClient.Recv()
			if err != nil {
				t.Logf("Recv() terminated with error (expected EOF for ONCE): %v", err)
				break
			}
			if update := resp.GetUpdate(); update != nil {
				prefix := update.GetPrefix()
				for _, u := range update.GetUpdate() {
					updateCount++
					fullPath := &gnmipb.Path{
						Origin: "Cisco-IOS-XR-platforms-ofa-oper",
					}
					if prefix != nil {
						fullPath.Elem = append(fullPath.Elem, prefix.GetElem()...)
					}
					if u.GetPath() != nil {
						fullPath.Elem = append(fullPath.Elem, u.GetPath().GetElem()...)
					}
					u.Path = fullPath
					allUpdates = append(allUpdates, u)
				}
			}
			if resp.GetSyncResponse() {
				t.Log("Received SyncResponse. Exiting Recv() loop.")
				break
			}
		}
		cancel()
		t.Logf("Completed Recv() loop for path: %v. Collected %d leaf updates this round.", nativePath, updateCount)
	}

	counters := make(map[string]uint64)
	if len(allUpdates) == 0 {
		t.Log("No updates received for any native path.")
		return counters
	}

	t.Logf("Total leaf updates collected across all paths: %d. Calling translator...", len(allUpdates))

	dummySR := &gnmipb.SubscribeResponse{
		Response: &gnmipb.SubscribeResponse_Update{
			Update: &gnmipb.Notification{
				Timestamp: time.Now().UnixNano(),
				Prefix:    &gnmipb.Path{}, // Prefix is already joined into the Update paths
				Update:    allUpdates,
			},
		},
	}

	translatedSR, err := ft.Translate(dummySR)
	if err != nil {
		t.Errorf("Translation Failed: %v", err)
		return counters
	}
	if translatedSR == nil {
		t.Log("Translator returned nil SubscribeResponse")
		return counters
	}
	translatedUpdate := translatedSR.GetUpdate()
	if translatedUpdate == nil {
		t.Log("Translator returned SubscribeResponse with nil Update")
		return counters
	}

	t.Log("Successfully translated native paths. Parsing translated paths...")
	for _, update := range translatedUpdate.GetUpdate() {
		path := update.GetPath()
		elems := path.GetElem()
		// Path expected: /components/component[name=X]/integrated-circuit/pipeline-counters/drop/vendor/CiscoXR/spitfire/(packet-processing|adverse)/state/<drop-reason>
		if len(elems) >= 11 &&
			elems[0].GetName() == "components" &&
			elems[2].GetName() == "integrated-circuit" &&
			elems[5].GetName() == "vendor" &&
			elems[6].GetName() == "CiscoXR" {

			dropReason := elems[10].GetName()
			val := update.GetVal().GetUintVal()
			t.Logf("Found Translated Counter: %s = %d", dropReason, val)
			counters[dropReason] += val
		}
	}
	return counters
}

func sendTraffic(t *testing.T, otg *otg.OTG, top gosnappi.Config, flow gosnappi.Flow, dut *ondatra.DUTDevice) {
	t.Log("Configuring and starting OTG protocols...")
	top.Flows().Clear()
	top.Flows().Append(flow)
	otg.PushConfig(t, top)
	otg.StartProtocols(t)

	t.Log("Waiting for ARP resolution...")
	otgutils.WaitForARP(t, otg, top, "IPv4")

	// Read initial DUT ingress packets on Port 1
	dp1 := dut.Port(t, "port1")
	initialRx := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Counters().InUnicastPkts().State())
	t.Logf("DUT %s initial Rx packets: %d", dp1.Name(), initialRx)

	t.Logf("ARP resolved. Starting traffic for %v...", flowDuration)
	otg.StartTraffic(t)
	time.Sleep(flowDuration)

	t.Log("Stopping traffic...")
	otg.StopTraffic(t)
	time.Sleep(2 * time.Second)

	// Read final DUT ingress packets on Port 1
	finalRx := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Counters().InUnicastPkts().State())
	t.Logf("DUT %s final Rx packets: %d (Delta: %d)", dp1.Name(), finalRx, finalRx-initialRx)
	if finalRx <= initialRx {
		t.Logf("WARNING: DUT interface %s did not receive any traffic according to InUnicastPkts! Counters: %d -> %d. This might be expected if packets are dropped early.", dp1.Name(), initialRx, finalRx)
	}

	t.Log("Traffic run complete.")
}

func TestCiscoVendorDrops(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	if dut.Vendor() != ondatra.CISCO {
		t.Skip("Test only runs for Cisco vendor")
	}

	t.Run("L3RouteLookupFailed", func(t *testing.T) {
		counters := readVendorDropCounters(t, dut)

		dropReason := "L3_ROUTE_LOOKUP_FAILED"
		// The README specifies to check for a non-negative value. We do not check for >= 0 explicitly because the counter value is a uint64, which is always non-negative by definition.
		if val, ok := counters[dropReason]; ok {
			t.Logf("Counter %s found with value: %d", dropReason, val)
		} else {
			found := false
			for k, v := range counters {
				if strings.Contains(k, dropReason) {
					found = true
					t.Logf("Counter %s (matched by substring %q) found with value: %d", k, dropReason, v)
					break
				}
			}
			if !found {
				t.Errorf("Counter %s not found in telemetry", dropReason)
			}
		}
	})

	t.Run("L3NullAdj", func(t *testing.T) {
		ate := ondatra.ATE(t, "ate")

		configureDUT(t, dut)
		top := configureATE(t, ate)

		niName := deviations.DefaultNetworkInstance(dut)
		if !deviations.ExplicitInterfaceInDefaultVRF(dut) {
			niName = "DEFAULT"
		}

		gnmi.Update(t, dut, gnmi.OC().NetworkInstance(niName).Config(), &oc.NetworkInstance{Name: &niName})

		b := &gnmi.SetBatch{}
		sV4 := &cfgplugins.StaticRouteCfg{
			NetworkInstance: niName,
			Prefix:          "203.0.113.2/32",
			NextHops: map[string]oc.NetworkInstance_Protocol_Static_NextHop_NextHop_Union{
				"0": oc.LocalRouting_LOCAL_DEFINED_NEXT_HOP_DROP,
			},
		}
		if _, err := cfgplugins.NewStaticRouteCfg(b, sV4, dut); err != nil {
			t.Fatalf("Failed to configure IPv4 static route: %v", err)
		}
		b.Set(t, dut)
		defer func() {
			sp := gnmi.OC().NetworkInstance(niName).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
			gnmi.Delete(t, dut, sp.Static("203.0.113.2/32").Config())
		}()

		initialCounters := readVendorDropCounters(t, dut)

		dp1 := dut.Port(t, "port1")
		dutMac := gnmi.Get(t, dut, gnmi.OC().Interface(dp1.Name()).Ethernet().MacAddress().State())

		ap1 := ate.Port(t, "port1")
		ap2 := ate.Port(t, "port2")
		flow := gosnappi.NewFlow().SetName("NullRouteFlow")
		flow.Metrics().SetEnable(true)
		flow.Rate().SetPps(ppsRate)
		flow.Duration().Continuous()
		flow.TxRx().Port().SetTxName(ap1.ID()).SetRxName(ap2.ID())
		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(ateSrc.MAC)
		eth.Dst().SetValue(dutMac)
		ip := flow.Packet().Add().Ipv4()
		ip.Src().SetValue(ateSrc.IPv4)
		ip.Dst().SetValue("203.0.113.2")

		sendTraffic(t, ate.OTG(), top, flow, dut)

		finalCounters := readVendorDropCounters(t, dut)

		found := false
		for reason, finalVal := range finalCounters {
			if strings.HasPrefix(reason, "L3_NULL_ADJ") {
				initialVal := initialCounters[reason]
				if finalVal > initialVal {
					found = true
					t.Logf("Counter %s successfully incremented. Initial: %d, Final: %d", reason, initialVal, finalVal)
				}
			}
		}

		if !found {
			t.Errorf("No L3_NULL_ADJ counter incremented.")
		}
	})

	t.Run("MPLSLabelMiss", func(t *testing.T) {
		counters := readVendorDropCounters(t, dut)

		dropReason := "MPLS_TE_MIDPOINT_LDP_LABELS_MISS"
		// The README specifies to check for a non-negative value. We do not check for >= 0 explicitly because the counter value is a uint64, which is always non-negative by definition.
		if val, ok := counters[dropReason]; ok {
			t.Logf("Counter %s found with value: %d", dropReason, val)
		} else {
			found := false
			for k, v := range counters {
				if strings.Contains(k, dropReason) {
					found = true
					t.Logf("Counter %s (matched by substring %q) found with value: %d", k, dropReason, v)
					break
				}
			}
			if !found {
				t.Errorf("Counter %s not found in telemetry", dropReason)
			}
		}
	})
}
