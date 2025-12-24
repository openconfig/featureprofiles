package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/exec/utils/textfsm/textfsm"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	gnmic "github.com/openconfig/gnmic/pkg/api/path"
	"github.com/openconfig/ondatra"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	logDir = flag.String("logDir", "", "Directory path to save subscription response logs")
)

const (
	// Telemetry subscription server IPs
	orionServerIP   = "10.85.104.103" // ott-ucs1-b4
	pictor1ServerIP = "10.85.104.103" // ott-ucs1-b4
	pictor2ServerIP = "10.85.74.137"  // ott2-ucs-vm5
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestOrionSubscriptionPaths(t *testing.T) {
	testSubscriptionPaths(t, GetOrionSubscriptions(), "orion")
}

func TestPictorSubscriptionPaths(t *testing.T) {
	testSubscriptionPaths(t, GetPictorSubscriptions(), "pictor")
}

func TestVerifySubscriptionsOnDevice(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	cliClient := dut.RawAPIs().CLI(t)

	t.Logf("Fetching all telemetry subscriptions from device")

	// Execute CLI command once to get all telemetry subscriptions
	cmd := "show telemetry model-driven subscription .*"
	ctx := context.Background()
	result, err := cliClient.RunCommand(ctx, cmd)
	if err != nil {
		t.Fatalf("Failed to execute CLI command '%s': %v", cmd, err)
	}

	cliOutput := result.Output()
	t.Logf("CLI Output length: %d bytes", len(cliOutput))

	// Parse the CLI output using TextFSM once
	parser := &textfsm.ShowTelemetryModelDrivenSubscription{}
	if err := parser.Parse(cliOutput); err != nil {
		t.Fatalf("Failed to parse telemetry subscription output: %v", err)
	}

	t.Logf("Found %d total subscription(s) on device", len(parser.Rows))

	// Define expected subscriptions
	orionSubs := GetOrionSubscriptions()
	pictorSubs := GetPictorSubscriptions()

	expectedSubscriptions := []struct {
		name              string
		serverIP          string
		expectedPathCount int
		subscriptionDefs  []Subscription
	}{
		{"orion", orionServerIP, len(orionSubs), orionSubs},
		{"pictor-1", pictor1ServerIP, len(pictorSubs), pictorSubs},
		{"pictor-2", pictor2ServerIP, len(pictorSubs), pictorSubs},
	}

	// Verify each expected subscription
	for _, expected := range expectedSubscriptions {
		t.Run(expected.name, func(t *testing.T) {
			verifySubscription(t, parser, expected.name, expected.serverIP, expected.expectedPathCount, expected.subscriptionDefs)
		})
	}
}

func testSubscriptionPaths(t *testing.T, subscriptionDefs []Subscription, testName string) {
	dut := ondatra.DUT(t, "dut")
	gnmiClient := dut.RawAPIs().GNMI(t)

	subList := getSubscriptionSliceFromDefs(t, subscriptionDefs)
	t.Logf("Testing %d %s subscription paths", len(subList), testName)
	t.Logf("Assuming all subscription paths exist on device (per gNMI spec 3.5.2.4)")

	// Create subscription client
	subClient, err := gnmiClient.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Failed to create subscription client: %v", err)
	}

	// Build subscription request with all paths
	subReq := &gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: &gpb.SubscriptionList{
				Subscription: subList,
				Mode:         gpb.SubscriptionList_STREAM, // Use STREAM for SAMPLE and ON_CHANGE
				Encoding:     gpb.Encoding_PROTO,
			},
		},
	}

	// Send subscription request
	if err := subClient.Send(subReq); err != nil {
		t.Fatalf("Failed to send subscription request: %v", err)
	}

	// Track received paths and sync status
	receivedPaths := make(map[string]bool)
	syncReceived := false
	updateCount := 0
	deleteCount := 0
	missingTimestamps := 0

	// Create log file for responses
	var logFile *os.File
	var logFilePath string
	if *logDir != "" {
		logFilePath = filepath.Join(*logDir, fmt.Sprintf("subscription_responses_%s.log", testName))
	} else {
		logFilePath = fmt.Sprintf("subscription_responses_%s.log", testName)
	}
	logFile, err = os.Create(logFilePath)
	if err != nil {
		t.Logf("Warning: Could not create response log file at %s: %v", logFilePath, err)
		logFile = nil
	} else {
		defer logFile.Close()
		t.Logf("Logging responses to: %s", logFilePath)
	}

	// Receive responses
	for {
		resp, err := subClient.Recv()
		if err == io.EOF {
			t.Log("Subscription stream closed")
			break
		} else if err != nil {
			t.Fatalf("Error receiving response: %v", err)
		}

		// Log response to file
		if logFile != nil {
			fmt.Fprintf(logFile, "\n=== Response #%d ===\n", updateCount+1)
			fmt.Fprintf(logFile, "%s\n", prototext.Format(resp))
		}

		// Check for sync response
		if resp.GetSyncResponse() {
			syncReceived = true
			t.Logf("✓ Sync response received after %d updates", updateCount)
			// Per gNMI spec 3.5.1.5.2: target MUST send initial updates before sync_response
			if updateCount == 0 {
				t.Errorf("❌ FAIL: sync_response received but no initial updates sent (spec violation)")
			}
			// After sync, we can close the stream for ONCE-like behavior
			if err := subClient.CloseSend(); err != nil {
				t.Errorf("Failed to close send: %v", err)
			}
			break
		}

		// Process update response
		if update := resp.GetUpdate(); update != nil {
			updateCount++

			// Validate timestamp per gNMI spec 2.1 and 3.5.2.3
			if update.GetTimestamp() == 0 {
				missingTimestamps++
				if missingTimestamps <= 3 {
					t.Logf("⚠️  Warning: Missing timestamp in Notification (spec requires timestamp)")
				}
			}

			// Log prefix
			prefix := update.GetPrefix()
			prefixStr := formatPath(prefix)

			// Process each update
			for _, upd := range update.GetUpdate() {
				pathStr := formatPath(upd.GetPath())
				fullPath := prefixStr + pathStr
				receivedPaths[fullPath] = true

				// Log first few updates for visibility
				if updateCount <= 5 {
					t.Logf("Update #%d: %s", updateCount, fullPath)
				}
			}

			// Process deletes per gNMI spec 3.5.2.3
			for _, delPath := range update.GetDelete() {
				deleteCount++
				delPathStr := formatPath(delPath)
				if deleteCount <= 5 {
					t.Logf("Delete: %s", delPathStr)
				}
				// Remove from received paths if it was there
				delete(receivedPaths, prefixStr+delPathStr)
			}

			// Log progress every 100 updates
			if updateCount%100 == 0 {
				t.Logf("Progress: %d updates received, %d unique paths", updateCount, len(receivedPaths))
			}
		}
	}

	// Verify results
	t.Logf("\n=== Subscription Test Summary ===")
	t.Logf("Total updates received: %d", updateCount)
	t.Logf("Total deletes received: %d", deleteCount)
	t.Logf("Unique paths received: %d", len(receivedPaths))
	t.Logf("Expected paths: %d", len(subList))
	t.Logf("Missing timestamps: %d", missingTimestamps)

	// Validate sync response (gNMI spec 3.5.1.4 and 3.5.2.3)
	if !syncReceived {
		t.Errorf("❌ FAIL: Sync response was NOT received (spec violation)")
	} else {
		t.Logf("✓ PASS: Sync response received")
	}

	// Validate initial updates were sent before sync (gNMI spec 3.5.1.5.2)
	if syncReceived && updateCount == 0 {
		t.Errorf("❌ FAIL: sync_response received but no initial updates (spec violation)")
	} else if updateCount > 0 {
		t.Logf("✓ PASS: Received %d initial updates before sync", updateCount)
	}

	// Warn about missing timestamps (gNMI spec 2.1 requires timestamps)
	if missingTimestamps > 0 {
		t.Logf("⚠️  Warning: %d notifications missing timestamps (spec violation)", missingTimestamps)
	}

	// Report deletes if any
	if deleteCount > 0 {
		t.Logf("ℹ️  Info: Received %d delete messages during initial sync", deleteCount)
	}
}

// formatPath converts a gNMI path to a readable string
func formatPath(path *gpb.Path) string {
	if path == nil {
		return ""
	}
	var pathStr strings.Builder
	if path.GetOrigin() != "" {
		pathStr.WriteString(path.GetOrigin())
		pathStr.WriteString(":")
	}
	for _, elem := range path.GetElem() {
		// Always add "/" before each element (including first one)
		pathStr.WriteString("/")
		pathStr.WriteString(elem.GetName())
		if len(elem.GetKey()) > 0 {
			pathStr.WriteString("[")
			first := true
			for k, v := range elem.GetKey() {
				if !first {
					pathStr.WriteString(",")
				}
				pathStr.WriteString(fmt.Sprintf("%s=%s", k, v))
				first = false
			}
			pathStr.WriteString("]")
		}
	}
	return pathStr.String()
}

// getSubscriptionSliceFromDefs converts Subscription definitions to gNMI Subscription protobuf messages
func getSubscriptionSliceFromDefs(t *testing.T, subscriptionDefs []Subscription) []*gpb.Subscription {
	t.Helper()

	var subPaths []*gpb.Path
	for _, line := range subscriptionDefs {
		split := strings.SplitN(line.Path, ":", 2)
		path, _ := gnmic.ParsePath(split[1])
		path.Origin = split[0]
		subPaths = append(subPaths, path)
	}

	var subs []*gpb.Subscription
	for i, path := range subPaths {
		sub := &gpb.Subscription{
			Path: &gpb.Path{
				Elem:   path.GetElem(),
				Origin: path.GetOrigin(),
			},
			Mode: subscriptionDefs[i].StreamMode,
		}

		// Add sample interval for SAMPLE mode subscriptions
		if subscriptionDefs[i].StreamMode == gpb.SubscriptionMode_SAMPLE && subscriptionDefs[i].SampleInterval != "" {
			// Parse sample interval (e.g., "10s" -> 10000000000 nanoseconds)
			var nanos uint64
			if strings.HasSuffix(subscriptionDefs[i].SampleInterval, "s") {
				var seconds uint64
				fmt.Sscanf(subscriptionDefs[i].SampleInterval, "%d", &seconds)
				nanos = seconds * 1000000000
			}
			sub.SampleInterval = nanos
		}

		subs = append(subs, sub)
	}

	return subs
}

// verifySubscription verifies that a specific telemetry subscription exists in the parsed data
func verifySubscription(t *testing.T, parser *textfsm.ShowTelemetryModelDrivenSubscription, subscriptionName, expectedServerIP string, expectedPathCount int, subscriptionDefs []Subscription) {
	t.Helper()

	t.Logf("Verifying %s subscription from server %s with %d defined paths (may expand to more)", subscriptionName, expectedServerIP, expectedPathCount)

	// Create a map of expected path patterns from subscription definitions
	expectedPathPatterns := make(map[string]bool)
	for _, sub := range subscriptionDefs {
		// Extract just the path part without origin for comparison
		pathParts := strings.SplitN(sub.Path, ":", 2)
		if len(pathParts) == 2 {
			expectedPathPatterns[pathParts[1]] = true
		}
	}

	// Find the subscription matching our criteria (server IP and matching paths)
	var matchingSubscription *textfsm.ShowTelemetryModelDrivenSubscriptionRow
	var bestMatchScore int

	for i := range parser.Rows {
		row := &parser.Rows[i]
		nested := row.ToNested()

		// Calculate total paths for this subscription
		totalPaths := len(nested.SensorGroups) + len(nested.CollectionGroups)

		t.Logf("Checking Subscription: %s, Destination IP: %s, State: %s, Total Paths: %d",
			nested.SubscriptionName,
			nested.DestinationGroup.IP,
			nested.State,
			totalPaths)

		// Only consider subscriptions with matching IP
		if nested.DestinationGroup.IP != expectedServerIP {
			continue
		}

		// Count how many expected paths match this subscription
		matchScore := 0
		for expectedPattern := range expectedPathPatterns {
			// Check in sensor paths
			for _, sg := range nested.SensorGroups {
				if matchesPathPattern(sg.SensorPath, expectedPattern) {
					matchScore++
					break
				}
			}
			// Check in collection paths if not found in sensor paths
			for _, cg := range nested.CollectionGroups {
				if matchesPathPattern(cg.Path, expectedPattern) {
					matchScore++
					break
				}
			}
		}

		// Keep track of the subscription with the best match
		if matchScore > bestMatchScore {
			bestMatchScore = matchScore
			matchingSubscription = row
		}
	}

	if matchingSubscription == nil {
		t.Fatalf("❌ FAIL: No %s subscription found with destination IP %s", subscriptionName, expectedServerIP)
	}

	nested := matchingSubscription.ToNested()
	t.Logf("✓ Found best matching subscription: %s (matched %d/%d expected path patterns)",
		nested.SubscriptionName, bestMatchScore, len(expectedPathPatterns)) // Now verify the matched subscription in detail
	// Verify subscription state
	if nested.State != "ACTIVE" {
		t.Errorf("❌ FAIL: Subscription state is '%s', expected 'ACTIVE'", nested.State)
	} else {
		t.Logf("✓ PASS: Subscription state is ACTIVE")
	}

	// Verify destination group details
	t.Logf("Destination Group Details:")
	t.Logf("  - IP: %s", nested.DestinationGroup.IP)
	t.Logf("  - Port: %s", nested.DestinationGroup.Port)
	t.Logf("  - Encoding: %s", nested.DestinationGroup.Encoding)
	t.Logf("  - Transport: %s", nested.DestinationGroup.Transport)
	t.Logf("  - State: %s", nested.DestinationGroup.State)
	t.Logf("  - Total Bytes Sent: %s", nested.DestinationGroup.TotalBytesSent)
	t.Logf("  - Total Packets Sent: %s", nested.DestinationGroup.TotalPacketsSent)

	if nested.DestinationGroup.State != "Active" {
		t.Errorf("❌ FAIL: Destination state is '%s', expected 'Active'", nested.DestinationGroup.State)
	} else {
		t.Logf("✓ PASS: Destination state is Active")
	}

	// Count total paths from both sensor groups and collection groups
	totalPaths := len(nested.SensorGroups) + len(nested.CollectionGroups)

	t.Logf("Total paths on device: %d (Sensor Groups: %d, Collection Groups: %d)",
		totalPaths, len(nested.SensorGroups), len(nested.CollectionGroups))
	t.Logf("Expected path patterns: %d (may expand to more actual paths)", expectedPathCount)

	// Verify all expected path patterns are present
	matchedPaths := 0
	missingPaths := []string{}
	additionalPaths := []string{}

	// Check each expected path pattern
	for _, sub := range subscriptionDefs {
		pathParts := strings.SplitN(sub.Path, ":", 2)
		if len(pathParts) != 2 {
			continue
		}
		pathPattern := pathParts[1]
		found := false

		// Check in sensor paths
		for _, sg := range nested.SensorGroups {
			if matchesPathPattern(sg.SensorPath, pathPattern) {
				found = true
				matchedPaths++
				break
			}
		}

		// Check in collection paths if not found in sensor paths
		if !found {
			for _, cg := range nested.CollectionGroups {
				if matchesPathPattern(cg.Path, pathPattern) {
					found = true
					matchedPaths++
					break
				}
			}
		}

		if !found {
			missingPaths = append(missingPaths, sub.Path)
		}
	}

	// Find additional paths on device that don't match expected patterns
	devicePaths := make(map[string]bool)
	for _, sg := range nested.SensorGroups {
		if sg.SensorPath != "" {
			devicePaths[sg.SensorPath] = true
		}
	}
	for _, cg := range nested.CollectionGroups {
		if cg.Path != "" {
			devicePaths[cg.Path] = true
		}
	}

	for devicePath := range devicePaths {
		matched := false
		for _, sub := range subscriptionDefs {
			pathParts := strings.SplitN(sub.Path, ":", 2)
			if len(pathParts) == 2 && matchesPathPattern(devicePath, pathParts[1]) {
				matched = true
				break
			}
		}
		if !matched {
			additionalPaths = append(additionalPaths, devicePath)
		}
	}

	// Report results
	t.Logf("Matched %d/%d expected path patterns", matchedPaths, expectedPathCount)

	if len(missingPaths) > 0 {
		t.Errorf("❌ FAIL: %d expected path patterns not found in device subscription", len(missingPaths))
		t.Logf("Missing path patterns:")
		for _, path := range missingPaths {
			t.Logf("  - %s", path)
		}
	}

	if len(additionalPaths) > 0 {
		t.Logf("ℹ️  Info: %d additional paths found on device (not in expected patterns)", len(additionalPaths))
		if len(additionalPaths) <= 10 {
			t.Logf("Additional paths:")
			for _, path := range additionalPaths {
				t.Logf("  + %s", path)
			}
		} else {
			t.Logf("Additional paths (showing first 10 of %d):", len(additionalPaths))
			for i := 0; i < 10; i++ {
				t.Logf("  + %s", additionalPaths[i])
			}
		}
	}

	// Final summary
	t.Logf("\n=== %s Subscription Verification Summary ===", strings.Title(subscriptionName))
	t.Logf("Subscription Name: %s", nested.SubscriptionName)
	t.Logf("Subscription ID: %s", nested.SubscriptionID)
	t.Logf("Destination IP: %s", nested.DestinationGroup.IP)
	t.Logf("Subscription State: %s", nested.State)
	t.Logf("Destination State: %s", nested.DestinationGroup.State)
	t.Logf("Total Paths on Device: %d", totalPaths)
	t.Logf("Expected Path Patterns: %d", expectedPathCount)
	t.Logf("Matched Path Patterns: %d/%d", matchedPaths, expectedPathCount)
	t.Logf("Additional Paths: %d", len(additionalPaths))
	t.Logf("Total Packets Sent: %s", nested.DestinationGroup.TotalPacketsSent)
	t.Logf("Total Bytes Sent: %s", nested.DestinationGroup.TotalBytesSent)

	if nested.State == "ACTIVE" && nested.DestinationGroup.State == "Active" && len(missingPaths) == 0 {
		t.Logf("✓ PASS: %s subscription is running and active on the device", strings.Title(subscriptionName))
	} else {
		t.Errorf("❌ FAIL: %s subscription verification failed", strings.Title(subscriptionName))
	}
}

// matchesPathPattern checks if a device path matches an expected path pattern
// Handles wildcards like [name=*] in the pattern
func matchesPathPattern(devicePath, pattern string) bool {
	// Simple case: exact match
	if strings.Contains(devicePath, pattern) {
		return true
	}

	// Handle wildcard patterns like /interface[name=*]
	// Convert pattern with wildcards to a regex-like check
	patternParts := strings.Split(pattern, "/")
	deviceParts := strings.Split(devicePath, "/")

	if len(patternParts) > len(deviceParts) {
		return false
	}

	for i, patternPart := range patternParts {
		if patternPart == "" {
			continue
		}

		if i >= len(deviceParts) {
			return false
		}

		devicePart := deviceParts[i]

		// Handle wildcard in keys like [name=*]
		if strings.Contains(patternPart, "[") && strings.Contains(patternPart, "*") {
			// Extract the base element name before the bracket
			basePart := strings.Split(patternPart, "[")[0]
			deviceBasePart := strings.Split(devicePart, "[")[0]

			if basePart != deviceBasePart {
				return false
			}
			// If base matches, consider it a match (wildcard matches any key value)
		} else if patternPart != devicePart {
			return false
		}
	}

	return true
}
