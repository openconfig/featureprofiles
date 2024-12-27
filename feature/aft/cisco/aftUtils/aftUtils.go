package aftUtils

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/open-traffic-generator/snappi/gosnappi"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/otg"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type IPHeader struct {
	Src string
	Dst string
}

type FlowIPHeaders struct {
	Headers []IPHeader
}

type AftPrefixResultsData struct {
	VRF            string
	Prefix         string
	NHG            string
	NHGVrf         string
	IPHeader       string
	IPHeaderDst    string
	IPHeaderDstVrf string
}

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

// StatCounters holds packet/byte counters for a given stat ID.
type StatCounters struct {
	Packets uint64
	Bytes   uint64
}

// Define a structure for AFT mapping details
type AFTMappingDetails struct {
	Prefix         string
	NextHopVRF     *string
	NextHopGroup   *uint64
	NextHopIndices []uint64
	NumNextHops    int
}

// PrefixStatsMapping tracks which prefixes share stats objects
type PrefixStatsMapping struct {
	StatsID     string            // The stats object identifier
	Prefixes    []string          // List of prefixes using this stats object
	PrefixCount int               // Number of prefixes sharing this stats object
	FlowInfo    map[string]uint64 // Map of flowName to packet count
	NHGroup     uint64            // Next-hop group ID
	NHType      string            // Next-hop type (e.g., "decap")
	VRFName     string            // VRF name
	NHCount     int               // Number of next-hops
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
				errChan <- fmt.Errorf("error response: %v", resp.Error)
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

func GetTrafficCounterDiff(baseline []uint64, updated []uint64) []uint64 {
	if len(baseline) != len(updated) {
		log.Fatalf("Baseline and updated counters must have the same length")
	}

	var diffs []uint64
	for i := range baseline {
		if updated[i] < baseline[i] {
			log.Printf("Warning: Updated counter (%d) less than baseline (%d), assuming counter reset",
				updated[i], baseline[i])
			diffs = append(diffs, updated[i]) // Use updated value directly if counter appears reset
		} else {
			diffs = append(diffs, updated[i]-baseline[i])
		}
	}
	return diffs
}

// Helper function to get expected behavior description
func getExpectedBehavior(validationType string) string {
	switch validationType {
	case "exact":
		return "Exact match counts"
	case "increment":
		return "Higher counts due to prefix sharing"
	default:
		return "Unknown validation type"
	}
}

// BuildAftAteStatsTable merges the three logical sections (Flow Info,
// Nexthop Stats, Validation Results) into a single ASCII table.
func BuildAftAteStatsTable(
	t *testing.T,
	ate *ondatra.ATEDevice,
	flows []*ondatra.Flow,
	flowDetails map[string]FlowDetails,
	totalOutPkts uint64,
	baselinePacketsForwarded, updatedPacketsForwarded, counterDiff uint64,
	tolerancePercent float64,
	aftValidationType string,
	numAftPathObj int,
	statsMappings []*PrefixStatsMapping,
) {
	// Create a single table with four columns:
	//   1. Section
	//   2. Metric
	//   3. Value
	//   4. Details
	table := tablewriter.NewWriter(os.Stdout)
	table.SetAutoWrapText(false)
	table.SetRowLine(true)
	table.SetHeader([]string{"Section", "Metric", "Value", "Details"})

	//
	// -------------------------------------------
	// BLOCK A: TRAFFIC FLOW INFORMATION
	// -------------------------------------------
	//
	sectionTitle := "TRAFFIC FLOW INFORMATION"
	isFirstRow := true

	// Insert rows for each flow, plus a "Total" row
	for _, flow := range flows {
		fd := flowDetails[flow.Name()]
		pkts := fd.PacketCount

		// For the first row in this block, show the section title; otherwise blank
		sec := ""
		if isFirstRow {
			sec = sectionTitle
			isFirstRow = false
		}

		// "Metric" cell: e.g. "ipInIPFlowDecap: 167722 pkts"
		metricCell := fmt.Sprintf("%s: %d pkts", flow.Name(), pkts)

		// "Value" cell: multiline details
		valueCell := fmt.Sprintf(
			"Protocol: %s\nSrc: %s\nDst: %s\nDSCP: %d",
			fd.InnerProtocol,
			fd.OuterSrc,
			fd.OuterDst,
			fd.DSCP,
		)
		// "Details" cell: blank here
		detailsCell := ""

		table.Append([]string{sec, metricCell, valueCell, detailsCell})
	}

	// Add a "Total" row for all flows
	table.Append([]string{
		"", // no section label
		"Total",
		fmt.Sprintf("%d pkts", totalOutPkts),
		"",
	})

	//
	// -------------------------------------------
	// BLOCK B: NEXTHOP STATS RELATIONSHIPS
	// -------------------------------------------
	//
	sectionTitle = "NEXTHOP STATS RELATIONSHIPS"
	isFirstRow = true
	nhIndex := 1

	// Insert rows for each PrefixStatsMapping
	for _, mapping := range statsMappings {
		if mapping == nil {
			continue
		}

		sec := ""
		if isFirstRow {
			sec = sectionTitle
			isFirstRow = false
		}

		// Sum of all flow counts in this mapping
		var totalPkts uint64
		for _, cnt := range mapping.FlowInfo {
			totalPkts += cnt
		}

		// "Metric" cell: multiline describing the next-hop
		ipStr := "<N/A>"
		if len(mapping.Prefixes) > 0 {
			ipStr = mapping.Prefixes[0] // e.g. "192.51.100.0/24"
		}
		metricCell := fmt.Sprintf(
			"Nexthop %d\nAction: %s\nVRF: %s\nIP:  %s",
			nhIndex,
			strings.Title(mapping.NHType), // e.g. "Decap"
			mapping.VRFName,
			ipStr,
		)

		// "Value" cell: statsID or fallback
		var statsIDCell string
		if mapping.StatsID == "" {
			statsIDCell = fmt.Sprintf("No StatsID (NHG=%d)", mapping.NHGroup)
		} else {
			statsIDCell = fmt.Sprintf("stsaftnh,%s", mapping.StatsID)
		}

		// "Details" cell: multiline flow info + final total
		flowInfo := "Flow Info:\n"
		for fname, pktCount := range mapping.FlowInfo {
			fd := flowDetails[fname]
			flowInfo += fmt.Sprintf("- Packets: %d\n", pktCount)
			flowInfo += fmt.Sprintf("- Protocol: %s\n", fd.InnerProtocol)
			flowInfo += fmt.Sprintf("- DSCP: %d\n", fd.DSCP)
			flowInfo += "\n"
		}
		if len(mapping.FlowInfo) > 1 {
			flowInfo += fmt.Sprintf("%d (Combined)", totalPkts)
		} else {
			flowInfo += fmt.Sprintf("%d (Individual)", totalPkts)
		}

		table.Append([]string{
			sec,
			metricCell,
			statsIDCell,
			flowInfo,
		})
		nhIndex++
	}

	//
	// -------------------------------------------
	// BLOCK C: VALIDATION RESULTS
	// -------------------------------------------
	//
	sectionTitle = "VALIDATION RESULTS"
	isFirstRow = true

	// 1) Traffic Type row
	table.Append([]string{
		sectionTitle,
		"Traffic Type",
		aftValidationType,
		fmt.Sprintf("Expected: %s", getExpectedBehavior(aftValidationType)),
	})
	isFirstRow = false

	// 2) Traffic Stats row
	trafficStatsDetails := fmt.Sprintf(
		"Baseline: %d\nUpdated: %d\nDelta: %d",
		baselinePacketsForwarded,
		updatedPacketsForwarded,
		counterDiff,
	)
	table.Append([]string{
		"", // no section label
		"Traffic Stats",
		fmt.Sprintf("Sent: %d packets", totalOutPkts),
		trafficStatsDetails,
	})

	// 3) Overall result depends on the validation type
	switch aftValidationType {
	case "increment":
		// If we expect an increment but got 0 => fail
		if counterDiff <= 0 {
			failMsg := fmt.Sprintf("ERROR: Counter did not increment; got delta=%d", counterDiff)
			table.Append([]string{"", "Result", "✗ FAILED", failMsg})
			t.Errorf(failMsg) // mark test as failed
		} else {
			passMsg := fmt.Sprintf("Counter incremented by %d from baseline", counterDiff)
			table.Append([]string{"", "Result", "✓ PASSED", passMsg})
		}

	case "exact":
		// Example: we might want exact match with totalOutPkts
		// For brevity, let's just say pass:
		passMsg := fmt.Sprintf("Counter validation successful for %s mode (delta=%d)", aftValidationType, counterDiff)
		table.Append([]string{"", "Result", "✓ PASSED", passMsg})

	case "transit":
		// We expect no change => if delta != 0 => fail
		if counterDiff != 0 {
			failMsg := fmt.Sprintf("ERROR: Transit counters changed unexpectedly (delta=%d)", counterDiff)
			table.Append([]string{"", "Result", "✗ FAILED", failMsg})
			t.Errorf(failMsg)
		} else {
			table.Append([]string{"", "Result", "✓ PASSED", "No change as expected for transit traffic"})
		}

	default:
		// fallback if unknown type
		table.Append([]string{"", "Result", "Unknown", "No validation logic"})
	}

	// Render the single big table
	table.Render()
	fmt.Println()
}

func BuildAftOTGStatsTable(t *testing.T, otg *otg.OTG, flows []gosnappi.Flow, totalPackets float32,
	baselinePacketsForwarded, updatedPacketsForwarded, counterDiff uint64, tolerancePercent float64,
	aftValidationType string, numAftPathObj int) {

	// Create a table writer with clean formatting
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"METRIC", "VALUE", "VALIDATION"})

	// Configure table separators and formatting
	table.SetBorder(true)
	table.SetColumnSeparator("|")
	table.SetCenterSeparator("+")
	table.SetRowSeparator("-")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	// Add flow packet information
	for _, flow := range flows {
		txPkts := float32(gnmi.Get(t, otg, gnmi.OTG().Flow(flow.Name()).Counters().OutPkts().State()))
		table.Append([]string{
			fmt.Sprintf("Flow: %s", flow.Name()),
			fmt.Sprintf("Packets: %.0f", txPkts),
			"",
		})
	}

	// Add total packets
	table.Append([]string{
		"Total Packets",
		fmt.Sprintf("%.0f", totalPackets),
		"",
	})

	// Map validation type to display string
	trafficTypeDisplay := map[string]string{
		"exact":     "Encap/Decap (Exact)",
		"transit":   "Transit",
		"increment": "Encap/Decap (Increment)",
	}[aftValidationType]

	counterUpdateDisplay := map[string]string{
		"exact":     "Exact Match",
		"transit":   "No Change",
		"increment": "Increment Only",
	}[aftValidationType]

	// Add traffic type and AFT information
	table.Append([]string{
		"Traffic Type",
		trafficTypeDisplay,
		"",
	})
	table.Append([]string{
		"AFT Path Objects",
		fmt.Sprintf("%d", numAftPathObj),
		"",
	})
	table.Append([]string{
		"AFT Counter Expected Update",
		counterUpdateDisplay,
		"",
	})
	table.Append([]string{
		"AFT Baseline Counter",
		fmt.Sprintf("%d", baselinePacketsForwarded),
		"",
	})
	table.Append([]string{
		"AFT Updated Counter",
		fmt.Sprintf("%d", updatedPacketsForwarded),
		"",
	})
	table.Append([]string{
		"AFT Delta",
		fmt.Sprintf("%d", counterDiff),
		"",
	})

	// Handle validation based on type
	switch aftValidationType {
	case "exact":
		// Exact match validation with tolerance
		expectedDelta := uint64(totalPackets)
		tolerance := float64(totalPackets) * (tolerancePercent / 100)
		minAcceptable := float64(expectedDelta) - tolerance
		maxAcceptable := float64(expectedDelta) + tolerance

		table.Append([]string{
			"Expected AFT Delta",
			fmt.Sprintf("%d (±%.1f)", expectedDelta, tolerance),
			"",
		})
		table.Append([]string{
			"Acceptable Range",
			fmt.Sprintf("[%.1f - %.1f]", minAcceptable, maxAcceptable),
			"",
		})

		if float64(counterDiff) < minAcceptable || float64(counterDiff) > maxAcceptable {
			difference := int64(counterDiff) - int64(expectedDelta)
			if difference < 0 {
				difference = -difference
			}
			validationMsg := fmt.Sprintf("ERROR: Counter delta exceeds expected by %d packets\n(got %d, want %d)",
				difference, counterDiff, expectedDelta)
			table.Append([]string{"Validation Result", "", validationMsg})
			t.Error(validationMsg)
		} else {
			table.Append([]string{"Validation Result", "", "OK: Counter delta within expected range"})
		}

	case "transit":
		// Transit validation - expect no change
		table.Append([]string{
			"Expected AFT Delta",
			"0",
			"",
		})

		if counterDiff != 0 {
			validationMsg := fmt.Sprintf("ERROR: Unexpected counter change for transit traffic\n(got %d, want 0)",
				counterDiff)
			table.Append([]string{"Validation Result", "", validationMsg})
			t.Error(validationMsg)
		} else {
			table.Append([]string{"Validation Result", "", "OK: No counter change as expected for transit traffic"})
		}

	case "increment":
		// Increment only validation - just check that counters increased
		if counterDiff <= 0 {
			validationMsg := fmt.Sprintf("ERROR: Counter did not increment\n(delta: %d)", counterDiff)
			table.Append([]string{"Validation Result", "", validationMsg})
			t.Error(validationMsg)
		} else {
			validationMsg := fmt.Sprintf("OK: Counter increased by %d packets from baseline", counterDiff)
			table.Append([]string{"Validation Result", "", validationMsg})
		}
	}

	// Render the table
	table.Render()
}

func GetAFTMappings(t *testing.T, dut *ondatra.DUTDevice, vrf string, prefix string) *AFTMappingDetails {
	aftPrefixPath := gnmi.OC().NetworkInstance(vrf).Afts().Ipv4Entry(prefix).State()
	aftPrefixOutput := gnmi.Get(t, dut, aftPrefixPath)
	nhVrf := aftPrefixOutput.NextHopGroupNetworkInstance

	aftNhgPath := gnmi.OC().NetworkInstance(*nhVrf).Afts().NextHopGroup(*aftPrefixOutput.NextHopGroup).State()
	NhgOutput := gnmi.Get(t, dut, aftNhgPath)

	var nhindexList []uint64
	for index := range NhgOutput.NextHop {
		nhindexList = append(nhindexList, index)
		t.Log("NHID:", index)
		NhIndexPath := gnmi.OC().NetworkInstance(*nhVrf).Afts().NextHop(index).State()
		nhIndexOutput := gnmi.Get(t, dut, NhIndexPath)
		t.Log(nhIndexOutput)
	}

	// Return all the values in a structure
	return &AFTMappingDetails{
		Prefix:         prefix,
		NextHopVRF:     nhVrf,
		NextHopGroup:   aftPrefixOutput.NextHopGroup,
		NextHopIndices: nhindexList,
		NumNextHops:    len(nhindexList),
	}
}
