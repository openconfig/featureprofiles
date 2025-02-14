package aftUtils

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/open-traffic-generator/snappi/gosnappi"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"io"
	"log"
	"strconv"
	"testing"
	"time"
)

type FlowDetails struct {
	OuterProtocol string   // Protocol for the outer header (e.g., IPv4, IPv6)
	InnerProtocol string   // Protocol for the inner header (e.g., IPv4, IPv6)
	OuterSrc      string   // Source address for the outer header
	OuterDst      string   // Destination address for the outer header
	InnerSrc      string   // Source address for the inner header
	InnerDst      string   // Destination address for the inner header
	DSCP          uint8    // DSCP value for the outer header
	InnerDSCP     uint8    // DSCP value for the inner header
	DestPorts     []string // Destination ports
	PacketCount   uint64   // Packet count (to be updated later)
}

// 1) The struct capturing final chain details
// NhHitDetail is your final next-hop chain record for each prefix that incremented counters.
type NhHitDetail struct {
	Prefix      string
	NhgID       uint64
	Nhi         uint64
	PrePackets  uint64
	PostPackets uint64
	Delta       uint64
	OriginVrf   string // optional if you have it
	NextHopVrf  string // optional if you have it
}

type PrefixInfo struct {
	Prefix     string
	OriginVrf  string
	NextHopVrf string
}

// GetAftCountersSample streams next-hop counters (in SAMPLE mode) for
// a specified collectTime.  It merges every partial update into a
// single map: <nhIndex> → <packetsForwarded>.  Even if the counters
// are zero, it stores them so you have all the indexes at the end.
// Once collectTime elapses, we forcibly close the subscription and
// return the final map.
func GetAftCountersSample(
	t *testing.T,
	gnmiClient gpb.GNMIClient,
	sampleInterval time.Duration,
	collectTime time.Duration,
) (map[string]uint64, error) {

	ctx, cancel := context.WithTimeout(context.Background(), collectTime)
	defer cancel()

	t.Logf("[INFO] Starting SAMPLE subscription: sampleInterval=%v, totalDuration=%v",
		sampleInterval, collectTime)

	// Build the subscription request
	subReq := &gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: &gpb.SubscriptionList{
				Mode: gpb.SubscriptionList_STREAM,
				Subscription: []*gpb.Subscription{
					{
						Path: &gpb.Path{
							Elem: []*gpb.PathElem{
								{Name: "network-instances"},
								{Name: "network-instance", Key: map[string]string{"name": "*"}},
								{Name: "afts"},
								{Name: "next-hops"},
								{Name: "next-hop", Key: map[string]string{"index": "*"}},
								{Name: "state"},
								{Name: "counters"},
							},
						},
						Mode:           gpb.SubscriptionMode_SAMPLE,
						SampleInterval: uint64(sampleInterval.Nanoseconds()),
					},
				},
				Encoding: gpb.Encoding_JSON_IETF,
			},
		},
	}

	// 1) Open the Subscribe stream
	stream, err := gnmiClient.Subscribe(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open SAMPLE subscription: %w", err)
	}

	// 2) Send the subscription request
	if sendErr := stream.Send(subReq); sendErr != nil {
		return nil, fmt.Errorf("failed to send subscription request: %w", sendErr)
	}
	cumulative := make(map[string]uint64)

	// Notif and error channels for the read goroutine
	notifChan := make(chan *gpb.Notification, 100)
	errChan := make(chan error, 1)

	// 3) Read goroutine
	go func() {
		defer close(notifChan)
		for {
			resp, rerr := stream.Recv()
			if rerr != nil {
				//t.Logf("[DEBUG] stream.Recv returned error: %v", rerr)
				errChan <- rerr
				return
			}
			switch s := resp.Response.(type) {
			case *gpb.SubscribeResponse_Update:
				notifChan <- s.Update
			case *gpb.SubscribeResponse_SyncResponse:

			case *gpb.SubscribeResponse_Error:

				errChan <- fmt.Errorf("subscription error: %v", s)
				return
			default:
				t.Logf("[INFO] Received unknown response type: %T => ignoring", s)
			}
		}
	}()

ReceiveLoop:
	for {
		select {
		case singleNotif := <-notifChan:
			if singleNotif == nil {

				break ReceiveLoop
			}

			partialMap := parseOneNotification(t, singleNotif)
			// Merge partial => final
			for nhIndex, pkts := range partialMap {
				oldVal := cumulative[nhIndex]
				if pkts > oldVal {
					cumulative[nhIndex] = pkts
				} else {
					// Even if the new pkts is not higher, we still store something
					// if that index isn't present.  But we do want the max for "packets-forwarded."
					if _, exist := cumulative[nhIndex]; !exist {
						cumulative[nhIndex] = pkts
					} else {
						t.Logf("[INFO] Keeping nhIndex=%s at %d (partial update says %d)", nhIndex, oldVal, pkts)
					}
				}
			}

		case subErr := <-errChan:
			if subErr != nil && subErr != io.EOF {

				return cumulative, subErr
			}

			break ReceiveLoop

		case <-ctx.Done():
			// Attempt graceful close
			if cerr := stream.CloseSend(); cerr != nil {
				t.Logf("[INFO] stream.CloseSend returned err=%v", cerr)
			}
			// Some devices might never send an EOF => we inject one ourselves
			go func() {
				time.Sleep(time.Second)
				errChan <- io.EOF
			}()
			break ReceiveLoop
		}
	}

	t.Logf("[INFO] Done receiving. Final map => %v", cumulative)
	return cumulative, nil
}

// parseOneNotification parses the Val JSON inside a single *gpb.Notification
// in SAMPLE mode, extracting each next-hop index’s "packets-forwarded."
// If that field is missing, or zero, or parse fails, we store 0.  But we
// *always* store that index in the result map.
func parseOneNotification(
	t *testing.T,
	notif *gpb.Notification,
) map[string]uint64 {

	result := make(map[string]uint64)
	if notif == nil {
		t.Log("[DEBUG] parseOneNotification => got nil notif => returning empty map")
		return result
	}

	for _, upd := range notif.Update {
		//t.Logf("[DEBUG] parseOneNotification => update #%d => path=%v", i, upd.Path)
		nhIndex := ""
		for _, pe := range upd.Path.Elem {
			//t.Logf("[DEBUG]   path elem => Name=%s, Key=%v", pe.Name, pe.Key)
			if pe.Name == "next-hop" {
				nhIndex = pe.Key["index"]
			}
		}
		if nhIndex == "" {
			//t.Log("[DEBUG]   no next-hop index found => skip this update")
			continue
		}

		raw := upd.Val.GetJsonIetfVal()
		if raw == nil {
			// No JSON => store 0 but keep index
			//t.Logf("[DEBUG]   no JSON => store zero for nhIndex=%s", nhIndex)
			result[nhIndex] = 0
			continue
		}
		//t.Logf("[DEBUG]   Raw JSON for nhIndex=%s => %s", nhIndex, string(raw))

		// Unmarshal
		var top map[string]interface{}
		if err := json.Unmarshal(raw, &top); err != nil {
			//t.Logf("[DEBUG]   unmarshal error => store 0 for nhIndex=%s. err=%v", nhIndex, err)
			result[nhIndex] = 0
			continue
		}

		// Navigate top["state"]["counters"]["packets-forwarded"]
		stObj, _ := top["state"].(map[string]interface{})
		if stObj == nil {
			//t.Logf("[DEBUG]   'state' missing => store 0 for nhIndex=%s", nhIndex)
			result[nhIndex] = 0
			continue
		}
		ctrObj, _ := stObj["counters"].(map[string]interface{})
		if ctrObj == nil {
			//t.Logf("[DEBUG]   'counters' missing => store 0 for nhIndex=%s", nhIndex)
			result[nhIndex] = 0
			continue
		}
		pktsStr, _ := ctrObj["packets-forwarded"].(string)
		if pktsStr == "" {
			//t.Logf("[DEBUG]   'packets-forwarded' missing => store 0 for nhIndex=%s", nhIndex)
			result[nhIndex] = 0
			continue
		}
		pktsVal, parseErr := strconv.ParseUint(pktsStr, 10, 64)
		if parseErr != nil {
			//t.Logf("[DEBUG]   parseErr => store 0 for nhIndex=%s. err=%v", nhIndex, parseErr)
			result[nhIndex] = 0
			continue
		}
		// We got a valid parse => store it
		//t.Logf("[DEBUG]   packets-forwarded => nhIndex=%s => %d", nhIndex, pktsVal)
		result[nhIndex] = pktsVal
	}

	//t.Logf("[DEBUG] parseOneNotification => final result => %v", result)
	return result
}

func FindChangedIndices(
	preMap, postMap map[string]uint64,
) []string {
	allKeys := make(map[string]struct{})
	for k := range preMap {
		allKeys[k] = struct{}{}
	}
	for k := range postMap {
		allKeys[k] = struct{}{}
	}

	var changed []string
	for nhIndex := range allKeys {
		preVal := preMap[nhIndex]   // 0 if missing
		postVal := postMap[nhIndex] // 0 if missing
		if postVal > preVal {
			changed = append(changed, nhIndex)
		}
	}
	return changed
}

func BuildAftPrefixChain(
	t *testing.T,
	dut *ondatra.DUTDevice,
	preCounters, postCounters map[string]uint64,
) []NhHitDetail {

	changedIndices := FindChangedIndices(preCounters, postCounters)
	t.Logf("Incremented next-hops: %v", changedIndices)
	if len(changedIndices) == 0 {
		return nil
	}

	// Gather all AFT data
	nhgAll := gnmi.GetAll(t, dut, gnmi.OC().NetworkInstance("*").Afts().NextHopGroupAny().State())
	v4Prefixes := gnmi.GetAll(t, dut, gnmi.OC().NetworkInstance("*").Afts().Ipv4EntryAny().State())
	v6Prefixes := gnmi.GetAll(t, dut, gnmi.OC().NetworkInstance("*").Afts().Ipv6EntryAny().State())

	nhi2NhgMap := BuildNhiToNhgMap(t, nhgAll)
	nhg2PrefixMap := BuildNhgToPrefixMap(t, v4Prefixes, v6Prefixes) // now returns map[uint64][]PrefixInfo

	var results []NhHitDetail

	for _, nhiStr := range changedIndices {
		nhiVal, err := strconv.ParseUint(nhiStr, 10, 64)
		if err != nil {
			t.Errorf("Cannot parse next-hop index %q: %v", nhiStr, err)
			continue
		}

		preVal := preCounters[nhiStr]
		postVal := postCounters[nhiStr]
		delta := postVal - preVal

		nhgIDs := nhi2NhgMap[nhiVal] // all NHG IDs referencing this NHI
		if len(nhgIDs) == 0 {
			t.Logf("NHI %d incremented but wasn't found in any NHG?", nhiVal)
			continue
		}

		// For each NHG referencing this NHI
		for _, nhgID := range nhgIDs {
			// Get the associated prefix info
			prefixInfos := nhg2PrefixMap[nhgID]
			if len(prefixInfos) == 0 {
				// Possibly no route references that NHG
				continue
			}

			// Build final NhHitDetail
			for _, pInfo := range prefixInfos {
				results = append(results, NhHitDetail{
					Prefix:      pInfo.Prefix,
					Nhi:         nhiVal,
					NhgID:       nhgID,
					PrePackets:  preVal,
					PostPackets: postVal,
					Delta:       delta,
					OriginVrf:   pInfo.OriginVrf,
					NextHopVrf:  pInfo.NextHopVrf,
				})
			}
		}
	}

	return results
}

func BuildNhiToNhgMap(
	t *testing.T,
	nhgAll []*oc.NetworkInstance_Afts_NextHopGroup,
) map[uint64][]uint64 {
	nhi2Nhg := make(map[uint64][]uint64)
	for _, nhg := range nhgAll {
		if nhg == nil {
			continue
		}
		nhgID := nhg.GetId()
		for nextHopIndex, nhObj := range nhg.NextHop {
			if nhObj == nil {
				continue
			}
			nhi2Nhg[nextHopIndex] = append(nhi2Nhg[nextHopIndex], nhgID)
		}
	}
	return nhi2Nhg
}

func BuildNhgToPrefixMap(
	t *testing.T,
	v4Prefixes []*oc.NetworkInstance_Afts_Ipv4Entry,
	v6Prefixes []*oc.NetworkInstance_Afts_Ipv6Entry,
) map[uint64][]PrefixInfo {

	nhg2Prefix := make(map[uint64][]PrefixInfo)

	// (A) IPv4
	for _, v4 := range v4Prefixes {
		if v4 == nil {
			continue
		}
		prefixStr := v4.GetPrefix() // e.g. "192.51.100.0/24"

		nhgID := v4.GetNextHopGroup() // numeric ID of the NHG
		if nhgID == 0 {
			continue
		}

		originVrf := v4.GetOriginNetworkInstance()        // e.g. "DECAP_TE_VRF"
		nextHopVrf := v4.GetNextHopGroupNetworkInstance() // e.g. "DEFAULT"

		info := PrefixInfo{
			Prefix:     prefixStr,
			OriginVrf:  originVrf,
			NextHopVrf: nextHopVrf,
		}

		nhg2Prefix[nhgID] = append(nhg2Prefix[nhgID], info)
	}

	// (B) IPv6
	for _, v6 := range v6Prefixes {
		if v6 == nil {
			continue
		}
		prefixStr := v6.GetPrefix() // e.g. "0:2:6000::c9/128"

		nhgID := v6.GetNextHopGroup()
		if nhgID == 0 {
			continue
		}

		originVrf := v6.GetOriginNetworkInstance()        // e.g. "DECAP_TE_VRF"
		nextHopVrf := v6.GetNextHopGroupNetworkInstance() // e.g. "DEFAULT"

		info := PrefixInfo{
			Prefix:     prefixStr,
			OriginVrf:  originVrf,
			NextHopVrf: nextHopVrf,
		}

		nhg2Prefix[nhgID] = append(nhg2Prefix[nhgID], info)
	}

	return nhg2Prefix
}

func MergeNhgNhiChains(details []NhHitDetail) []NhHitDetail {
	type groupKey struct {
		Nhi        uint64
		NhgID      uint64
		NextHopVrf string
	}

	aggMap := make(map[groupKey]*NhHitDetail)

	for _, d := range details {
		key := groupKey{d.Nhi, d.NhgID, d.NextHopVrf}

		if aggMap[key] == nil {
			// First time we see this (NHI, NHG, NextHopVrf)
			cloned := NhHitDetail{
				Prefix:      d.Prefix, // We'll combine additional prefixes below
				NhgID:       d.NhgID,
				Nhi:         d.Nhi,
				PrePackets:  d.PrePackets,
				PostPackets: d.PostPackets,
				Delta:       d.Delta,
				OriginVrf:   d.OriginVrf,
				NextHopVrf:  d.NextHopVrf,
			}
			aggMap[key] = &cloned
		} else {
			// Merge into existing
			existing := aggMap[key]
			// Append the new prefix with "   /   " as a separator
			existing.Prefix = existing.Prefix + "   /   " + d.Prefix

			// If multiple entries differ in postCounters, pick the max or sum.
			// Here, we pick the max:
			if d.PostPackets > existing.PostPackets {
				existing.PostPackets = d.PostPackets
				existing.PrePackets = d.PrePackets
				existing.Delta = d.Delta
			}
		}
	}

	// Convert map to slice
	var merged []NhHitDetail
	for _, val := range aggMap {
		merged = append(merged, *val)
	}
	return merged
}

// AftCounterResults prints a table summarizing flows, next-hop deltas, and
// then runs validation (exact/increment/transit/etc.). Now includes a chainType
// parameter, but the lines for chainType and validationType appear in the
// final "VALIDATION" block instead of the main header.
func AftCounterResults(
	t *testing.T,
	flowDetails map[string]FlowDetails,
	chainResults []NhHitDetail,
	validationType string,
	totalEvaluated int, // how many total next-hop counters we evaluated
	tolerancePercent float64,
	chainType string, // e.g. "Decap", "Regular", etc.
) {
	// 1) Print a main header
	t.Logf("+=====================================================================+")
	t.Logf("| TRAFFIC FLOW & NEXT-HOP STATS SUMMARY                               |")
	t.Logf("+=====================================================================+")

	// 2) Print traffic flow info
	var totalFlowPkts uint64
	flowIndex := 1
	for flowName, fd := range flowDetails {
		t.Logf("| TRAFFIC FLOW INFORMATION (%d) | %s | %d pkts |",
			flowIndex, flowName, fd.PacketCount)

		// Outer header line
		t.Logf("|   Outer %s: %s -> %s, DSCP=%d",
			fd.OuterProtocol, fd.OuterSrc, fd.OuterDst, fd.DSCP)

		// Inner header line (if any)
		if fd.InnerProtocol != "" {
			t.Logf("|   Inner %s: %s -> %s, DSCP=%d",
				fd.InnerProtocol, fd.InnerSrc, fd.InnerDst, fd.InnerDSCP)
		}
		t.Logf("+---------------------------------------------------------------------+")
		totalFlowPkts += fd.PacketCount
		flowIndex++
	}

	// Summation row for all flows
	t.Logf("| **Aggregate Total** | **%d Flows** | **%d pkts** |",
		len(flowDetails), totalFlowPkts)
	t.Logf("+=====================================================================+")

	// 3) Merge chainResults so multiple prefixes sharing the same (NHI, NHG)
	//    become a single row. E.g., if one NHG references multiple prefixes,
	//    they collapse if they share the same NHI.
	mergedChain := MergeNhgNhiChains(chainResults)

	// 4) Print the chain mappings heading
	t.Logf("| CHAIN MAPPINGS (Prefix → NHG → NHI)                                 |")
	t.Logf("+=====================================================================+")

	mergedCount := len(mergedChain)
	if mergedCount == 0 {
		t.Logf("| => Discovered 0 changed next-hop chains out of %d total counters", totalEvaluated)
		t.Logf("+---------------------------------------------------------------------+")
		t.Logf("| => No increments / no chain details to display.")
		t.Logf("+---------------------------------------------------------------------+")
		// Still run validation with a chain delta of 0
		runValidationBlock(t, validationType, 0, totalFlowPkts, mergedChain, tolerancePercent, chainType)
		return
	}

	// If some changed, let's show the merged details
	t.Logf("| => Discovered %d changed next-hop chain(s) out of %d total counters",
		mergedCount, totalEvaluated)
	t.Logf("+---------------------------------------------------------------------+")

	var totalChainDelta uint64
	for _, hit := range mergedChain {
		t.Logf("| Prefix=%s", hit.Prefix)
		if hit.NextHopVrf != "" {
			t.Logf("|   -> NHG=%d (VRF=%s)", hit.NhgID, hit.NextHopVrf)
		} else {
			t.Logf("|   -> NHG=%d", hit.NhgID)
		}
		t.Logf("|   -> NHI=%d", hit.Nhi)
		t.Logf("|    counters => Pre=%d, Post=%d, Delta=%d",
			hit.PrePackets, hit.PostPackets, hit.Delta)
		t.Logf("+---------------------------------------------------------------------+")
		totalChainDelta += hit.Delta
	}

	// 5) Perform validation checks
	runValidationBlock(t, validationType, totalChainDelta, totalFlowPkts, mergedChain, tolerancePercent, chainType)
}

func runValidationBlock(
	t *testing.T,
	validationType string,
	totalChainDelta, totalFlowPkts uint64,
	mergedChain []NhHitDetail,
	tolerancePercent float64,
	chainType string, // chain type
) {
	t.Logf("| VALIDATION                                                          |")
	t.Logf("|  - CHAIN TYPE EVALUATED: %s", chainType)
	t.Logf("|  - VALIDATION TYPE: %s", validationType)

	pass := true
	var validationMsg string

	switch validationType {
	case "exact":
		// (same as your existing logic)
		if totalFlowPkts == 0 {
			if totalChainDelta == 0 {
				validationMsg = "Exact match: chainDelta=0, flowPkts=0"
			} else {
				pass = false
				validationMsg = fmt.Sprintf("Mismatch: chainDelta=%d, flowPkts=0", totalChainDelta)
			}
		} else {
			diff := absDiff(totalChainDelta, totalFlowPkts)
			tolerance := uint64(float64(totalFlowPkts) * (tolerancePercent / 100.0))

			if tolerancePercent <= 0.0 {
				// strict check, no tolerance
				if totalChainDelta == totalFlowPkts {
					validationMsg = fmt.Sprintf(
						"Exact match: chainDelta=%d, flowPkts=%d (no tolerance)",
						totalChainDelta, totalFlowPkts,
					)
				} else {
					pass = false
					validationMsg = fmt.Sprintf(
						"Mismatch: chainDelta=%d, flowPkts=%d (no tolerance)",
						totalChainDelta, totalFlowPkts,
					)
				}
			} else {
				// do +/- tolerance check
				if diff <= tolerance {
					validationMsg = fmt.Sprintf(
						"Exact match (±%.2f%%): chainDelta=%d, flowPkts=%d, diff=%d, tolerance=%d",
						tolerancePercent, totalChainDelta, totalFlowPkts, diff, tolerance,
					)
				} else {
					pass = false
					validationMsg = fmt.Sprintf(
						"Mismatch (±%.2f%%): chainDelta=%d, flowPkts=%d, diff=%d, tolerance=%d",
						tolerancePercent, totalChainDelta, totalFlowPkts, diff, tolerance,
					)
				}
			}
		}

	case "increment":
		// If we expect increments, but discovered NO changed next-hop chains,
		// it should fail. Because "no increments" = 0 changes, which is not correct
		// for the "increment" scenario.
		if len(mergedChain) == 0 {
			pass = false
			validationMsg = "Expected increments but found no changed next-hop chains."
			break
		}
		// For each chain, Delta must be > 0.
		for _, hit := range mergedChain {
			if hit.Delta == 0 {
				pass = false
				validationMsg = fmt.Sprintf("Chain (NHG=%d, NHI=%d) had zero increment", hit.NhgID, hit.Nhi)
				break
			}
		}
		if pass && validationMsg == "" {
			validationMsg = "All chain deltas > 0"
		}

	case "transit":
		// each chain must have Delta=0
		for _, hit := range mergedChain {
			if hit.Delta != 0 {
				pass = false
				validationMsg = fmt.Sprintf(
					"Chain (NHG=%d, NHI=%d) unexpectedly incremented by %d",
					hit.NhgID, hit.Nhi, hit.Delta,
				)
				break
			}
		}
		if pass && validationMsg == "" {
			validationMsg = "All chain deltas = 0 (transit scenario passed)"
		}

	default:
		// unrecognized validation => skip
		validationMsg = fmt.Sprintf("Unknown validation type=%s, skipping check", validationType)
	}

	// Print final PASS or FAIL block
	if pass {
		t.Logf("|  - Combined traffic across these flows = %d pkts                    |", totalFlowPkts)
		t.Logf("|  - Next-hop chain delta sum = %d", totalChainDelta)
		t.Logf("|  - %s", validationMsg)
		t.Logf("|  - Test PASSED!")
		t.Logf("+=====================================================================+")
	} else {
		t.Logf("|  - Combined traffic across these flows = %d pkts                    |", totalFlowPkts)
		t.Logf("|  - Next-hop chain delta sum = %d", totalChainDelta)
		t.Logf("|  - %s", validationMsg)
		t.Logf("|  - TEST FAILED!")
		t.Logf("+=====================================================================+")
		t.Fatalf("AftCounterResults => validationType=%q => FAILED: %s", validationType, validationMsg)
	}
}

// absDiff is a small helper to compute the absolute difference of two uint64.
func absDiff(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return b - a
}

func GetOnceNotifications(t *testing.T, gnmiClient gpb.GNMIClient) []*gpb.Notification {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()

	subscribeRequest := &gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: &gpb.SubscriptionList{
				Mode: gpb.SubscriptionList_ONCE,
				Subscription: []*gpb.Subscription{
					{
						Path: &gpb.Path{
							Elem: []*gpb.PathElem{
								{Name: "network-instances"},
								{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
								{Name: "afts"},
								{Name: "next-hops"},
								{Name: "next-hop"},
								{Name: "state"},
								{Name: "counters"},
							},
						},
						Mode: gpb.SubscriptionMode_ON_CHANGE,
					},
				},
				Encoding: gpb.Encoding_JSON_IETF,
			},
		},
	}

	// Start the subscription
	stream, err := gnmiClient.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Failed to start subscription: %v", err)
	}

	// Send the request
	if err := stream.Send(subscribeRequest); err != nil {
		t.Fatalf("Failed to send subscription request: %v", err)
	}

	// Create a channel for notifications and errors
	notifChan := make(chan *gpb.Notification, 100)
	errChan := make(chan error, 1)
	doneChan := make(chan struct{})

	// Process responses in a goroutine
	go func() {
		defer close(doneChan)
		for {
			response, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					t.Logf("Stream closed by server")
					return
				}
				errChan <- fmt.Errorf("receive error: %v", err)
				return
			}

			switch resp := response.Response.(type) {
			case *gpb.SubscribeResponse_Update:
				t.Logf("Received Update notification")
				notifChan <- resp.Update
			case *gpb.SubscribeResponse_SyncResponse:
				t.Logf("Received Sync response")
				return
			case *gpb.SubscribeResponse_Error:
				errChan <- fmt.Errorf("error response: %v", resp)
				return
			default:
				t.Logf("Received unknown response type: %T", resp)
			}
		}
	}()

	// Collect notifications with timeout
	var notifications []*gpb.Notification
	collectTimeout := time.NewTimer(30 * time.Second)
	defer collectTimeout.Stop()

	for {
		select {
		case notif := <-notifChan:
			notifications = append(notifications, notif)
		case err := <-errChan:
			t.Logf("Error during subscription: %v", err)
			return notifications
		case <-doneChan:
			t.Logf("Subscription completed normally")
			return notifications
		case <-collectTimeout.C:
			t.Logf("Collection timeout reached, returning %d notifications", len(notifications))
			return notifications
		case <-ctx.Done():
			t.Logf("Context timeout reached, returning %d notifications", len(notifications))
			return notifications
		}
	}
}

func GetPacketForwardedCounts(t *testing.T, gnmiClient gpb.GNMIClient) ([]uint64, int) {
	//dut := ondatra.DUT(t, "dut")
	//output2 := gnmi.GetAll(t, dut, gnmi.OC().NetworkInstanceAny().Afts().NextHopAny().State())
	//t.Log(output2)

	notifications := GetOnceNotifications(t, gnmiClient)

	var packetCounts []uint64
	numAftPathObjects := len(notifications)

	for _, n := range notifications {
		for _, update := range n.Update {
			typedValue := update.Val

			// Assuming the data is in JSON format, you can unmarshal it into a map
			if typedValue.GetJsonIetfVal() != nil {
				jsonData := typedValue.GetJsonIetfVal()

				var state map[string]interface{}
				if err := json.Unmarshal(jsonData, &state); err != nil {
					log.Fatalf("Failed to unmarshal JSON: %v", err)
				}

				// Navigate to "state.counters.packets-forwarded"
				if counters, ok := state["state"].(map[string]interface{})["counters"].(map[string]interface{}); ok {
					if packetsForwarded, ok := counters["packets-forwarded"].(string); ok {
						// Convert the string to a uint64 value
						packetsForwardedUint, err := strconv.ParseUint(packetsForwarded, 10, 64)
						if err != nil {
							log.Fatalf("Failed to convert packets-forwarded to uint64: %v", err)
						}
						// Save the value into the slice
						packetCounts = append(packetCounts, packetsForwardedUint)
					}
				}
			} else {
				log.Printf("Unhandled TypedValue format")
			}
		}
	}

	return packetCounts, numAftPathObjects
}

func GetOtgFlowDetails(
	t *testing.T,
	flows []gosnappi.Flow,
	totalPkts float32,
) map[string]FlowDetails {

	t.Helper()
	fdMap := make(map[string]FlowDetails)

	for _, fl := range flows {
		// Start building the struct
		var fd FlowDetails
		fd.PacketCount = uint64(totalPkts)

		items := fl.Packet().Items()
		//t.Logf("[DEBUG] Flow %q has %d packet item(s).", fl.Name(), len(items))

		for i, pktItem := range items {
			hasV4 := pktItem.HasIpv4()
			hasV6 := pktItem.HasIpv6()
			//t.Logf("[DEBUG] item[%d]: hasIpv4=%v, hasIpv6=%v", i, hasV4, hasV6)

			switch {
			case hasV4:
				fd.OuterProtocol = "IPv4"
				fd.OuterSrc = pktItem.Ipv4().Src().Value()
				fd.OuterDst = pktItem.Ipv4().Dst().Value()

				if p := pktItem.Ipv4().Priority(); p != nil && p.Dscp() != nil {
					fd.DSCP = uint8(p.Dscp().Phb().Value())
				} else {
					fd.DSCP = 0
				}

			case hasV6:
				fd.OuterProtocol = "IPv6"
				fd.OuterSrc = pktItem.Ipv6().Src().Value()
				fd.OuterDst = pktItem.Ipv6().Dst().Value()

				// For IPv6, DSCP is top 6 bits of traffic-class
				if tc := pktItem.Ipv6().TrafficClass(); tc != nil {
					fd.DSCP = uint8(tc.Value() >> 2)
				} else {
					fd.DSCP = 0
				}
			default:
				// Not IPv4 or IPv6 => skip
				t.Logf("[INFO] item[%d]: not IPv4/IPv6 => skipping", i)
			}

			// If a found a recognized IP, break to avoid overwriting
			//if fd.OuterProtocol != "" {
			//	break
			//}
		}

		fdMap[fl.Name()] = fd
	}

	return fdMap
}
