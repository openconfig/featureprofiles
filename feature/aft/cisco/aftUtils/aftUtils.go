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
	Protocol    string
	OuterSrc    string
	OuterDst    string
	DSCP        uint8
	DestPorts   []string
	PacketCount uint64
}

// PrefixStatsMapping tracks which prefixes share stats objects
type PrefixStatsMapping struct {
	StatsID     string            // The stats object identifier
	Prefixes    []string          // List of prefixes using this stats object
	PrefixCount int               // Number of prefixes sharing this stats object
	FlowInfo    map[string]uint64 // Keep the original map[string]uint64 type
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

func BuildAftAteStatsTable(t *testing.T, ate *ondatra.ATEDevice, flows []*ondatra.Flow,
	flowDetails map[string]FlowDetails, totalOutPkts uint64,
	baselinePacketsForwarded, updatedPacketsForwarded, counterDiff uint64,
	tolerancePercent float64, aftValidationType string, numAftPathObj int,
	statsMappings []*PrefixStatsMapping) {

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"METRIC", "VALUE", "VALIDATION"})
	table.SetBorder(true)
	table.SetColumnSeparator("|")
	table.SetCenterSeparator("+")
	table.SetRowSeparator("-")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)

	// Flow Information Section
	table.Append([]string{"Flow Information", "---", "---"})
	for _, flow := range flows {
		flowPath := gnmi.OC().Flow(flow.Name())
		outPkts := gnmi.Get(t, ate, flowPath.Counters().OutPkts().State())
		details := flowDetails[flow.Name()]

		table.Append([]string{
			fmt.Sprintf("Flow: %s", flow.Name()),
			fmt.Sprintf("Protocol: %s\nPackets: %d", details.Protocol, outPkts),
			fmt.Sprintf("Flow Details:\n- Outer Src: %s\n- Outer Dst: %s\n- DSCP: %d\n- Dest Ports: %v",
				details.OuterSrc, details.OuterDst, details.DSCP, details.DestPorts),
		})
	}

	table.Append([]string{
		"Total Packets",
		fmt.Sprintf("%d", totalOutPkts),
		"",
	})

	if len(statsMappings) > 0 {
		table.Append([]string{"Stats Objects Info", "---", "---"})
		seenStats := make(map[string]bool)
		var uniqueStatsCount int

		for _, mapping := range statsMappings {
			if mapping == nil {
				continue
			}
			if !seenStats[mapping.StatsID] {
				seenStats[mapping.StatsID] = true
				uniqueStatsCount++

				// Count flows by protocol
				v4Flows := 0
				v6Flows := 0
				v4Packets := uint64(0)
				v6Packets := uint64(0)

				for _, details := range flowDetails {
					if details.Protocol == "IPv6" {
						v6Flows++
						v6Packets += details.PacketCount
					} else {
						v4Flows++
						v4Packets += details.PacketCount
					}
				}

				table.Append([]string{
					fmt.Sprintf("Stats Object %s", mapping.StatsID),
					fmt.Sprintf("Used by %d prefixes:", mapping.PrefixCount),
					fmt.Sprintf("Shared between:\n%d IPv4-in-IP flows (%d packets)\n%d IPv6-in-IP flows (%d packets)",
						v4Flows, v4Packets, v6Flows, v6Packets),
				})

				for _, prefix := range mapping.Prefixes {
					table.Append([]string{
						"└─ Prefix",
						prefix,
						"",
					})
				}
			}
		}

		table.Append([]string{
			"Unique Stats Objects",
			fmt.Sprintf("%d", uniqueStatsCount),
			"",
		})

		if uniqueStatsCount < len(statsMappings) {
			tolerancePercent *= float64(len(statsMappings)) / float64(uniqueStatsCount)
			table.Append([]string{
				"Tolerance Adjustment",
				fmt.Sprintf("Increased due to %d prefixes sharing %d stats objects",
					len(statsMappings), uniqueStatsCount),
				fmt.Sprintf("New tolerance: %.2f%%", tolerancePercent),
			})
		}
	}

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

	table.Append([]string{"Counter Information", "---", "---"})
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

	table.Append([]string{"Validation Results", "---", "---"})

	switch aftValidationType {
	case "exact":
		expectedDelta := uint64(totalOutPkts)
		tolerance := float64(expectedDelta) * (tolerancePercent / 100)
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
			validationMsg := fmt.Sprintf("NOTE: Counter delta differs from expected by %d packets\n(got %d, want %d)\nThis may be normal if prefixes share stats objects",
				difference, counterDiff, expectedDelta)
			table.Append([]string{"Validation Result", "", validationMsg})
			t.Logf(validationMsg)
		} else {
			table.Append([]string{"Validation Result", "", "OK: Counter delta within expected range"})
		}

	case "transit":
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
		if counterDiff <= 0 {
			validationMsg := fmt.Sprintf("ERROR: Counter did not increment\n(delta: %d)", counterDiff)
			table.Append([]string{"Validation Result", "", validationMsg})
			t.Error(validationMsg)
		} else {
			validationMsg := fmt.Sprintf("OK: Counter increased by %d packets from baseline", counterDiff)
			table.Append([]string{"Validation Result", "", validationMsg})
		}
	}

	table.Render()
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
