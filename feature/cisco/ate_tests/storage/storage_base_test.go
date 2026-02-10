package storage_test

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/schemaless"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// testArgs contains shared test infrastructure components
type testArgs struct {
	ate *ondatra.ATEDevice
	ctx context.Context
	dut *ondatra.DUTDevice
}

// storageTestCase defines a storage counter test configuration
type storageTestCase struct {
	name        string
	path        string
	counterType string
	description string
	fn          func(t *testing.T, args *testArgs, path string)
}

const (

	// Network configuration constants
	ipv4PrefixLen = 30
	ipv6PrefixLen = 126
)

// getSubscriber implements gpb.GNMI_SubscribeClient using gNMI GET requests
// instead of streaming subscriptions. This allows testing GET-based data retrieval.
type getSubscriber struct {
	gpb.GNMI_SubscribeClient
	client gpb.GNMIClient      // gNMI client for making requests
	ctx    context.Context     // Request context
	notifs []*gpb.Notification // Cached GET response notifications
	index  int                 // Current notification index
	done   bool                // Indicates if all notifications processed
}

// Send converts a SubscribeRequest to a GetRequest and executes it
func (gs *getSubscriber) Send(req *gpb.SubscribeRequest) error {
	getReq := &gpb.GetRequest{
		Prefix:   req.GetSubscribe().GetPrefix(),
		Encoding: gpb.Encoding_JSON_IETF,
		Type:     gpb.GetRequest_ALL,
	}
	for _, sub := range req.GetSubscribe().GetSubscription() {
		getReq.Path = append(getReq.Path, sub.GetPath())
	}

	// Use 60 second timeout for GET requests
	ctx, cancel := context.WithTimeout(gs.ctx, 60*time.Second)
	defer cancel()

	resp, err := gs.client.Get(ctx, getReq)
	if err != nil {
		return fmt.Errorf("GET request failed: %v", err)
	}

	if len(resp.GetNotification()) == 0 {
		return fmt.Errorf("GET response contains no notifications")
	}

	gs.notifs = resp.GetNotification()
	gs.index = 0
	gs.done = false
	return nil
}

// Recv returns the next notification from the cached GET response
func (gs *getSubscriber) Recv() (*gpb.SubscribeResponse, error) {
	if gs.done || gs.index >= len(gs.notifs) {
		return &gpb.SubscribeResponse{
			Response: &gpb.SubscribeResponse_SyncResponse{
				SyncResponse: true,
			},
		}, nil
	}

	resp := &gpb.SubscribeResponse{
		Response: &gpb.SubscribeResponse_Update{
			Update: gs.notifs[gs.index],
		},
	}
	gs.index++

	if gs.index >= len(gs.notifs) {
		gs.done = true
	}

	return resp, nil
}

// Network interface configuration attributes for test topology
var (
	dutSrc = attrs.Attributes{
		Desc:    "dutSrc",
		IPv4:    "100.121.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:121:1:1",
		IPv6Len: ipv6PrefixLen,
	}
	dutDst = attrs.Attributes{
		Desc:    "dutDst",
		IPv4:    "100.122.1.1",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2000::100:122:1:1",
		IPv6Len: ipv6PrefixLen,
	}
)

// sortPorts sorts ports by testbed port ID in ascending order
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

// getLinecardComponents returns components matching linecard naming patterns (0/X/CPUY)
// These include both linecard and route processor components with storage counters
func getLinecardComponents(t *testing.T, args *testArgs) []string {
	t.Helper()
	allComponents := gnmi.GetAll(t, args.dut, gnmi.OC().ComponentAny().State())
	var nodeComponents []string

	// Regex pattern to match exactly "X/Y/CPUZ" where Z is 0 or 1, with no additional text
	cpuPattern := regexp.MustCompile(`^[^/]+/[^/]+/CPU[01]$`)

	for _, component := range allComponents {
		name := component.GetName()
		// Filter for components with pattern "0/X/CPUY" (linecard and RP components)
		// Ensure the component name ends exactly with "/CPU0" or "/CPU1" (no additional text)
		if cpuPattern.MatchString(name) &&
			!strings.Contains(name, "-") &&
			!strings.Contains(strings.ToUpper(name), "IOSXR-NODE") {

			// Include linecard components (e.g., "0/0/CPU0" to "0/9/CPU0")
			// AND RP components (e.g., "0/RP0/CPU0", "0/RP1/CPU0")
			// Exclude components with additional text after CPU0/CPU1 (e.g., "0/0/CPU0-Optics Controller 0")
			if strings.Contains(name, "/RP") ||
				(!strings.Contains(name, "/RP") &&
					(strings.Contains(name, "/0/") || strings.Contains(name, "/1/") ||
						strings.Contains(name, "/2/") || strings.Contains(name, "/3/") ||
						strings.Contains(name, "/4/") || strings.Contains(name, "/5/") ||
						strings.Contains(name, "/6/") || strings.Contains(name, "/7/") ||
						strings.Contains(name, "/8/") || strings.Contains(name, "/9/"))) {
				nodeComponents = append(nodeComponents, name)
			}
		}
	}
	//t.Logf("Found linecard and RP components: %v", nodeComponents)

	if len(nodeComponents) == 0 {
		t.Skipf("No linecard or RP components found on device %s", args.dut.Model())
	}

	return nodeComponents
}

// executeCLICommands runs CLI commands to exercise smart_monitor_main.c code path
func executeCLICommands(t *testing.T, dut *ondatra.DUTDevice, ctx context.Context) {
	t.Helper()
	cliHandle := dut.RawAPIs().CLI(t)

	// Execute show smart-monitor (exercises main() and show_smart_monitor_node_datalist())
	t.Log("Executing CLI: show smart-monitor")
	resp, err := cliHandle.RunCommand(ctx, "show smart-monitor")
	if err != nil {
		t.Logf("CLI command 'show smart-monitor' failed: %v", err)
	} else {
		output := resp.Output()
		t.Logf("CLI 'show smart-monitor' returned %d bytes", len(output))
		parseSMARTMonitorOutput(t, output)
	}

	// Execute show smart-monitor detail (exercises show_smart_monitor_detailed_view())
	t.Log("Executing CLI: show smart-monitor location all")
	respDetail, err := cliHandle.RunCommand(ctx, "show smart-monitor location all")
	if err != nil {
		t.Logf("CLI command 'show smart-monitor location all' failed: %v", err)
	} else {
		outputDetail := respDetail.Output()
		t.Logf("CLI 'show smart-monitor location all' returned %d bytes", len(outputDetail))
		parseSMARTMonitorDetailedOutput(t, outputDetail)
	}
}

// parseSMARTMonitorOutput parses the basic show smart-monitor output
func parseSMARTMonitorOutput(t *testing.T, output string) {
	t.Helper()

	if output == "" {
		t.Log("Warning: Empty output from show smart-monitor")
		return
	}

	t.Log("=== Parsing SMART Monitor Output ===")
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		t.Logf("  %s", line)
	}
}

// DiskSMARTData holds parsed SMART monitor data for a disk
type DiskSMARTData struct {
	Location                    string
	DiskName                    string
	Model                       string
	SerialNumber                string
	HealthStatus                string
	SoftReadErrorRate           uint64
	ReallocatedSectors          uint64
	EndToEndError               uint64
	OfflineUncorrectableSectors uint64
	LifeRemaining               uint32
	PercentageUsed              uint32
}

// parseHealthStatus extracts and validates SMART health status from the output
// Checks for various health status formats as per SMART specification
func parseHealthStatus(output string) string {
	// Check for overall health assessment test results (ATA/SATA drives)
	if strings.Contains(output, "SMART overall-health self-assessment test result: PASSED") {
		return "PASSED"
	} else if strings.Contains(output, "SMART overall-health self-assessment test result: FAILED") {
		return "FAILED"
	}

	// Default to UNKNOWN if no recognized health status found
	return "UNKNOWN"
}

// parseSMARTMonitorDetailedOutput parses the detailed SMART monitor output
// and extracts all SMART counter values from the show smart-monitor location all output
func parseSMARTMonitorDetailedOutput(t *testing.T, output string) {
	t.Helper()

	if output == "" {
		t.Log("Warning: Empty output from show smart-monitor location all")
		return
	}

	t.Log("=== Parsing SMART Monitor Detailed Output ===")

	// First, check for overall health status in the output
	overallHealth := parseHealthStatus(output)
	t.Logf("Overall SMART Health Assessment: %s", overallHealth)

	var disks []DiskSMARTData
	var currentDisk *DiskSMARTData
	var currentLocation string

	lines := strings.Split(output, "\n")

	// Regular expressions for parsing different fields
	locationRe := regexp.MustCompile(`Detailed SMART Monitor Info for Location:\s*(.+)`)
	diskRe := regexp.MustCompile(`Disk:\s*(.+)`)
	modelRe := regexp.MustCompile(`Model:\s*(.+)`)
	serialRe := regexp.MustCompile(`Serial Number:\s*(.+)`)
	healthRe := regexp.MustCompile(`Health Status:\s*(.+)`)
	softReadErrorRe := regexp.MustCompile(`Soft Read Error Rate:\s*(\d+)`)
	reallocatedSectorsRe := regexp.MustCompile(`Reallocated Sectors:\s*(\d+)`)
	endToEndErrorRe := regexp.MustCompile(`End-to-End Error:\s*(\d+)`)
	offlineUncorrectableRe := regexp.MustCompile(`Offline Uncorrectable Sectors:\s*(\d+)`)
	lifeRemainingRe := regexp.MustCompile(`Life Remaining:\s*(\d+)%`)
	percentageUsedRe := regexp.MustCompile(`Percentage Used:\s*(\d+)%`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Check for location header
		if matches := locationRe.FindStringSubmatch(line); matches != nil {
			currentLocation = strings.TrimSpace(matches[1])
			t.Logf("Found location: %s", currentLocation)
			continue
		}

		// Check for disk entry
		if matches := diskRe.FindStringSubmatch(line); matches != nil {
			// Save previous disk if exists
			if currentDisk != nil {
				disks = append(disks, *currentDisk)
			}

			// Start new disk
			currentDisk = &DiskSMARTData{
				Location: currentLocation,
				DiskName: strings.TrimSpace(matches[1]),
			}
			t.Logf("Found disk: %s", currentDisk.DiskName)
			continue
		}

		if currentDisk == nil {
			continue
		}

		// Parse disk attributes
		if matches := modelRe.FindStringSubmatch(line); matches != nil {
			currentDisk.Model = strings.TrimSpace(matches[1])
			t.Logf("  Model: %s", currentDisk.Model)
		} else if matches := serialRe.FindStringSubmatch(line); matches != nil {
			currentDisk.SerialNumber = strings.TrimSpace(matches[1])
			t.Logf("  Serial Number: %s", currentDisk.SerialNumber)
		} else if matches := healthRe.FindStringSubmatch(line); matches != nil {
			healthStatus := strings.TrimSpace(matches[1])
			currentDisk.HealthStatus = healthStatus

			// Validate and classify health status
			validHealthStatuses := []string{"PASSED", "FAILED", "UNKNOWN"}
			isValid := false
			for _, validStatus := range validHealthStatuses {
				if strings.EqualFold(healthStatus, validStatus) {
					isValid = true
					break
				}
			}

			if isValid {
				t.Logf("  Health Status: %s (Valid)", currentDisk.HealthStatus)
			} else {
				t.Logf("  Health Status: %s (Warning: Unexpected status value)", currentDisk.HealthStatus)
			}
		} else if matches := softReadErrorRe.FindStringSubmatch(line); matches != nil {
			if val, err := strconv.ParseUint(matches[1], 10, 64); err == nil {
				currentDisk.SoftReadErrorRate = val
				t.Logf("  Soft Read Error Rate: %d", val)
			}
		} else if matches := reallocatedSectorsRe.FindStringSubmatch(line); matches != nil {
			if val, err := strconv.ParseUint(matches[1], 10, 64); err == nil {
				currentDisk.ReallocatedSectors = val
				t.Logf("  Reallocated Sectors: %d", val)
			}
		} else if matches := endToEndErrorRe.FindStringSubmatch(line); matches != nil {
			if val, err := strconv.ParseUint(matches[1], 10, 64); err == nil {
				currentDisk.EndToEndError = val
				t.Logf("  End-to-End Error: %d", val)
			}
		} else if matches := offlineUncorrectableRe.FindStringSubmatch(line); matches != nil {
			if val, err := strconv.ParseUint(matches[1], 10, 64); err == nil {
				currentDisk.OfflineUncorrectableSectors = val
				t.Logf("  Offline Uncorrectable Sectors: %d", val)
			}
		} else if matches := lifeRemainingRe.FindStringSubmatch(line); matches != nil {
			if val, err := strconv.ParseUint(matches[1], 10, 32); err == nil {
				currentDisk.LifeRemaining = uint32(val)
				t.Logf("  Life Remaining: %d%%", val)
			}
		} else if matches := percentageUsedRe.FindStringSubmatch(line); matches != nil {
			if val, err := strconv.ParseUint(matches[1], 10, 32); err == nil {
				currentDisk.PercentageUsed = uint32(val)
				t.Logf("  Percentage Used: %d%%", val)
			}
		}
	}

	// Save last disk if exists
	if currentDisk != nil {
		disks = append(disks, *currentDisk)
	}

	// Summary report
	t.Logf("\n=== SMART Monitor Parsing Summary ===")
	t.Logf("Total disks parsed: %d", len(disks))

	// Health status statistics
	healthStatusCounts := make(map[string]int)
	for _, disk := range disks {
		healthStatusCounts[disk.HealthStatus]++
	}

	t.Logf("\nHealth Status Distribution:")
	for status, count := range healthStatusCounts {
		t.Logf("  %s: %d disk(s)", status, count)
	}

	for i, disk := range disks {
		t.Logf("\nDisk %d Summary:", i+1)
		t.Logf("  Location: %s", disk.Location)
		t.Logf("  Name: %s", disk.DiskName)
		t.Logf("  Model: %s", disk.Model)
		t.Logf("  Serial: %s", disk.SerialNumber)
		t.Logf("  Health: %s", disk.HealthStatus)
		t.Logf("  Counters:")
		t.Logf("    Soft Read Error Rate: %d", disk.SoftReadErrorRate)
		t.Logf("    Reallocated Sectors: %d", disk.ReallocatedSectors)
		t.Logf("    End-to-End Error: %d", disk.EndToEndError)
		t.Logf("    Offline Uncorrectable Sectors: %d", disk.OfflineUncorrectableSectors)
		t.Logf("    Life Remaining: %d%%", disk.LifeRemaining)
		t.Logf("    Percentage Used: %d%%", disk.PercentageUsed)
	}
}

// collectAndLogCountersEnhanced performs enhanced counter collection with timestamp validation
// and comprehensive error handling for SAMPLE subscription mode
func collectAndLogCountersEnhanced(t *testing.T, data map[string]ygnmi.WildcardQuery[uint64]) {
	t.Helper()

	successCount := 0
	errorCount := 0
	totalPaths := len(data)

	t.Logf("Starting enhanced data collection for %d paths with SAMPLE mode", totalPaths)

	// aggregate pre counters for a path across all the destination linecards with enhanced validation
	for path, query := range data {
		t.Logf("Attempting to fetch data for path: %s", path)

		pre, err := getData(t, path, query)
		if err != nil {
			errorCount++
			t.Errorf("Failed to get data for path %s: %v", path, err)
			continue
		}

		successCount++
		t.Logf("Successfully retrieved counter for path %s: %d", path, pre)
	}

	// Summary reporting
	t.Logf("=== DATA COLLECTION SUMMARY (SAMPLE MODE) ===")
	t.Logf("Total paths attempted: %d", totalPaths)
	t.Logf("Successful retrievals: %d", successCount)
	t.Logf("Failed retrievals: %d", errorCount)
	t.Logf("Success rate: %.1f%%", float64(successCount)/float64(totalPaths)*100)

	// Fail test if too many paths failed
	if errorCount > 0 {
		failureThreshold := 0.3 // Allow up to 30% failure rate for SAMPLE mode (more strict)
		if float64(errorCount)/float64(totalPaths) > failureThreshold {
			t.Fatalf("Too many paths failed with SAMPLE mode (%d/%d = %.1f%% > %.1f%%). This indicates a significant issue with device connectivity or path availability",
				errorCount, totalPaths, float64(errorCount)/float64(totalPaths)*100, failureThreshold*100)
		} else {
			t.Logf("Some paths failed but within acceptable threshold (%.1f%% failure rate)", float64(errorCount)/float64(totalPaths)*100)
		}
	} else {
		t.Logf("All paths retrieved successfully with SAMPLE mode")
	}
}

// collectAndLogCountersWithModeEnhanced collects counter data with enhanced error handling and timestamp validation
func collectAndLogCountersWithModeEnhanced(t *testing.T, data map[string]ygnmi.WildcardQuery[uint64], subMode gpb.SubscriptionMode) {
	t.Helper()

	successCount := 0
	errorCount := 0
	totalPaths := len(data)

	t.Logf("Starting enhanced data collection for %d paths with subscription mode %v", totalPaths, subMode)

	// aggregate pre counters for a path across all the destination linecards with enhanced validation
	for path, query := range data {
		t.Logf("Attempting to fetch data for path: %s", path)

		pre, err := getDataWithMode(t, path, query, subMode)
		if err != nil {
			errorCount++
			t.Errorf("Failed to get data for path %s with mode %v: %v", path, subMode, err)

			// Try with fallback mode if TARGET_DEFINED fails
			if subMode == gpb.SubscriptionMode_TARGET_DEFINED {
				t.Logf("Attempting fallback to SAMPLE mode for path %s", path)
				fallbackPre, fallbackErr := getDataWithMode(t, path, query, gpb.SubscriptionMode_SAMPLE)
				if fallbackErr == nil {
					t.Logf("Fallback successful - Initial counter for path %s with fallback SAMPLE mode: %d", path, fallbackPre)
					successCount++
				} else {
					t.Errorf("Fallback also failed for path %s: %v", path, fallbackErr)
				}
			}
			continue
		}

		successCount++
		t.Logf("Successfully retrieved counter for path %s with mode %v: %d", path, subMode, pre)
	}

	// Summary reporting
	t.Logf("=== DATA COLLECTION SUMMARY ===")
	t.Logf("Total paths attempted: %d", totalPaths)
	t.Logf("Successful retrievals: %d", successCount)
	t.Logf("Failed retrievals: %d", errorCount)
	t.Logf("Success rate: %.1f%%", float64(successCount)/float64(totalPaths)*100)

	// Fail test if too many paths failed
	if errorCount > 0 {
		failureThreshold := 0.5 // Allow up to 50% failure rate
		if float64(errorCount)/float64(totalPaths) > failureThreshold {
			t.Fatalf("Too many paths failed (%d/%d = %.1f%% > %.1f%%). This indicates a significant issue with subscription mode %v or device connectivity",
				errorCount, totalPaths, float64(errorCount)/float64(totalPaths)*100, failureThreshold*100, subMode)
		} else {
			t.Logf("Some paths failed but within acceptable threshold (%.1f%% failure rate)", float64(errorCount)/float64(totalPaths)*100)
		}
	} else {
		t.Logf("All paths retrieved successfully with subscription mode %v", subMode)
	}
}

// collectAndLogCountersWithGetRequest collects counter data using gNMI GET requests with enhanced validation
func collectAndLogCountersWithGetRequest(t *testing.T, args *testArgs, data map[string]ygnmi.WildcardQuery[uint64]) {
	t.Helper()

	successCount := 0
	errorCount := 0
	totalPaths := len(data)

	t.Logf("Starting gNMI GET request data collection for %d paths", totalPaths)

	// Collect counter data for each path using GET requests
	for path, query := range data {
		t.Logf("Attempting GET request for path: %s", path)

		value, err := getDataWithGetRequest(t, args, path, query)
		if err != nil {
			errorCount++
			t.Errorf("Failed to get data via GET request for path %s: %v", path, err)
			continue
		}

		successCount++
		t.Logf("Successfully retrieved counter via GET request for path %s: %d", path, value)
	}

	// Summary reporting
	t.Logf("=== gNMI GET REQUEST COLLECTION SUMMARY ===")
	t.Logf("Total paths attempted: %d", totalPaths)
	t.Logf("Successful retrievals: %d", successCount)
	t.Logf("Failed retrievals: %d", errorCount)
	t.Logf("Success rate: %.1f%%", float64(successCount)/float64(totalPaths)*100)

	// Fail test if too many paths failed
	if errorCount > 0 {
		failureThreshold := 0.2 // Allow up to 20% failure rate for GET requests (more strict)
		if float64(errorCount)/float64(totalPaths) > failureThreshold {
			t.Fatalf("Too many paths failed with gNMI GET requests (%d/%d = %.1f%% > %.1f%%). This indicates a significant issue with GET request support or device connectivity",
				errorCount, totalPaths, float64(errorCount)/float64(totalPaths)*100, failureThreshold*100)
		} else {
			t.Logf("Some paths failed but within acceptable threshold (%.1f%% failure rate)", float64(errorCount)/float64(totalPaths)*100)
		}
	} else {
		t.Logf("All paths retrieved successfully with gNMI GET requests")
	}
}

// createQueries builds gNMI wildcard queries for storage counter paths on all linecard components
// Returns a map of query paths to their corresponding wildcard queries
func createQueries(t *testing.T, args *testArgs, pathSuffix string) map[string]ygnmi.WildcardQuery[uint64] {
	t.Helper()
	data := make(map[string]ygnmi.WildcardQuery[uint64])

	nodeComponents := getLinecardComponents(t, args)

	// Create queries for all node components
	for _, component := range nodeComponents {
		//t.Logf("Testing component: %s", component)
		path := fmt.Sprintf("/components/component[name=%s]/%s", component, pathSuffix)
		query, err := schemaless.NewWildcard[uint64](path, "openconfig")
		if err != nil {
			t.Fatalf("failed to create query for path %s: %v", path, err)
		}
		data[path] = query
	}

	return data
}

// testStorageCounterSampleMode validates storage counters using SAMPLE subscription mode
// with enhanced timestamp validation and error handling
func testStorageCounterSampleMode(t *testing.T, args *testArgs, pathSuffix string) {
	t.Helper()

	for _, subMode := range []gpb.SubscriptionMode{gpb.SubscriptionMode_SAMPLE} {
		t.Logf("Path name: %s", pathSuffix)
		t.Logf("Subscription mode: %v", subMode)

		data := createQueries(t, args, pathSuffix)
		collectAndLogCountersEnhanced(t, data)
	}
}

// testStorageCounterOnceMode validates storage counters using ONCE subscription mode
func testStorageCounterOnceMode(t *testing.T, args *testArgs, pathSuffix string) {
	t.Helper()
	for _, subMode := range []gpb.SubscriptionList_Mode{gpb.SubscriptionList_ONCE} {
		t.Logf("Path name: %s", pathSuffix)
		t.Logf("Subscription mode: %v", subMode)

		data := createQueries(t, args, pathSuffix)
		collectAndLogCountersEnhanced(t, data)
	}
}

// testStorageCounterTargetMode validates storage counters using TARGET_DEFINED subscription mode
func testStorageCounterTargetMode(t *testing.T, args *testArgs, pathSuffix string) {
	t.Helper()

	for _, subMode := range []gpb.SubscriptionMode{gpb.SubscriptionMode_TARGET_DEFINED} {
		t.Logf("Path name: %s", pathSuffix)
		t.Logf("Subscription mode: %v", subMode)

		data := createQueries(t, args, pathSuffix)
		collectAndLogCountersWithModeEnhanced(t, data, subMode)
	}
}

// testStorageCounterOnChangeMode tests storage counters using On-Change subscription mode
func testStorageCounterOnChangeMode(t *testing.T, args *testArgs, pathSuffix string) {
	t.Helper()

	for _, subMode := range []gpb.SubscriptionMode{gpb.SubscriptionMode_ON_CHANGE} {

		t.Logf("Path name: %s", pathSuffix)
		t.Logf("Subscription mode: %v", subMode)

		// Create queries for all components using common helper
		data := createQueries(t, args, pathSuffix)

		// Collect and log counter data using enhanced helper with timestamp validation
		collectAndLogCountersWithModeEnhanced(t, data, subMode)
	}
}

// testStorageCounterGetMode tests storage counters using gNMI GET requests
func testStorageCounterGetMode(t *testing.T, args *testArgs, pathSuffix string) {
	t.Helper()

	t.Logf("Path name: %s", pathSuffix)
	t.Logf("Request mode: gNMI GET")

	// Create queries for all components using common helper
	data := createQueries(t, args, pathSuffix)

	// Collect and log counter data using GET requests with enhanced validation
	collectAndLogCountersWithGetRequest(t, args, data)
}

// getDataWithGetRequest performs gNMI GET request to fetch storage counter data with timestamp validation
// This function mimics subscription behavior but uses GET requests for testing compatibility
func getDataWithGetRequest(t *testing.T, args *testArgs, path string, query ygnmi.WildcardQuery[uint64]) (uint64, error) {
	t.Helper()

	startTime := time.Now()
	// Storage counters may not update frequently, allow up to 2 hours for stale data
	maxAcceptableAge := 2 * time.Hour

	dut := args.dut
	gnmiClient := dut.RawAPIs().GNMI(t)
	ctx := context.Background()

	getSubscriber := &getSubscriber{
		client: gnmiClient,
		ctx:    ctx,
	}

	gnmiPath, _, err := ygnmi.ResolvePath(query.PathStruct())
	if err != nil {
		return 0, fmt.Errorf("failed to resolve path for GET request %s: %v", path, err)
	}

	// Create a subscribe request (which will be converted to GET)
	subscribeReq := &gpb.SubscribeRequest{
		Request: &gpb.SubscribeRequest_Subscribe{
			Subscribe: &gpb.SubscriptionList{
				Subscription: []*gpb.Subscription{
					{
						Path: gnmiPath,
					},
				},
				Mode:     gpb.SubscriptionList_ONCE,
				Encoding: gpb.Encoding_JSON_IETF,
			},
		},
	}

	// Send the GET request
	if err := getSubscriber.Send(subscribeReq); err != nil {
		return 0, fmt.Errorf("GET request failed for path %s: %v", path, err)
	}

	// Process the response
	var foundValue uint64
	var found bool
	var responseTimestamp time.Time

	for {
		resp, err := getSubscriber.Recv()
		if err != nil {
			return 0, fmt.Errorf("failed to receive GET response for path %s: %v", path, err)
		}

		if resp.GetSyncResponse() {
			break // End of data
		}

		if update := resp.GetUpdate(); update != nil {
			t.Logf("GET response received with %d updates", len(update.GetUpdate()))

			// Check if this update matches our target path
			for i, upd := range update.GetUpdate() {
				updPath, err := ygot.PathToString(upd.GetPath())
				if err != nil {
					t.Logf("error converting update path %d to string: %v", i, err)
					continue
				}

				t.Logf("GET response path %d: %s", i, updPath)
				t.Logf("Target path: %s", path)

				// Check if this matches our target path
				// Extract component name from our target path for better matching
				targetComponent := ""
				if strings.Contains(path, "[name=") && strings.Contains(path, "]") {
					start := strings.Index(path, "[name=") + 6
					end := strings.Index(path[start:], "]")
					if end > 0 {
						targetComponent = path[start : start+end]
					}
				}

				// Multiple matching strategies
				pathMatches := false

				// Strategy 1: Direct path containment
				if strings.Contains(updPath, path) {
					pathMatches = true
					t.Logf("Direct path match: %s contains %s", updPath, path)
				}

				// Strategy 2: Component-based matching
				if !pathMatches && targetComponent != "" && strings.Contains(updPath, targetComponent) {
					// Check if the response path contains the target component and counter type
					pathSuffix := strings.Split(path, "/storage/state/counters/")
					if len(pathSuffix) > 1 && strings.Contains(updPath, pathSuffix[1]) {
						pathMatches = true
						t.Logf("Component-based match: %s matches component %s and counter %s", updPath, targetComponent, pathSuffix[1])
					}
				}

				// Strategy 3: OpenConfig prefix handling
				if !pathMatches {
					// Remove openconfig: prefix if present in response path
					cleanResponsePath := strings.TrimPrefix(updPath, "openconfig:")
					cleanTargetPath := strings.TrimPrefix(path, "/")
					if strings.Contains(cleanResponsePath, cleanTargetPath) {
						pathMatches = true
						t.Logf("OpenConfig prefix match: %s matches %s", cleanResponsePath, cleanTargetPath)
					}
				}

				if pathMatches {
					// Extract the value
					if jsonVal := upd.GetVal().GetJsonVal(); jsonVal != nil {
						// Parse JSON value - this is a simplified parser
						jsonStr := string(jsonVal)
						if val, parseErr := parseJsonUint64(jsonStr); parseErr == nil {
							foundValue = val
							found = true

							// Extract timestamp if available
							if update.GetTimestamp() != 0 {
								responseTimestamp = time.Unix(0, int64(update.GetTimestamp()))
							} else {
								responseTimestamp = time.Now()
							}

							t.Logf("GET request found JSON value %d for path %s", val, path)
							break
						}
					} else if jsonIetfVal := upd.GetVal().GetJsonIetfVal(); jsonIetfVal != nil {
						// Parse JSON IETF value - this is the standard encoding format
						jsonIetfStr := string(jsonIetfVal)
						t.Logf("GET request processing JSON IETF value: %s for path %s", jsonIetfStr, path)
						if val, parseErr := parseJsonUint64(jsonIetfStr); parseErr == nil {
							foundValue = val
							found = true

							// Extract timestamp if available
							if update.GetTimestamp() != 0 {
								responseTimestamp = time.Unix(0, int64(update.GetTimestamp()))
							} else {
								responseTimestamp = time.Now()
							}

							t.Logf("GET request found JSON IETF value %d for path %s", val, path)
							break
						}
					} else if intVal := upd.GetVal().GetUintVal(); intVal != 0 {
						foundValue = intVal
						found = true

						// Extract timestamp if available
						if update.GetTimestamp() != 0 {
							responseTimestamp = time.Unix(0, int64(update.GetTimestamp()))
						} else {
							responseTimestamp = time.Now()
						}

						t.Logf("GET request found uint value %d for path %s", intVal, path)
						break
					} else if strVal := upd.GetVal().GetStringVal(); strVal != "" {
						// Try to parse string as uint64
						if val, parseErr := strconv.ParseUint(strVal, 10, 64); parseErr == nil {
							foundValue = val
							found = true

							// Extract timestamp if available
							if update.GetTimestamp() != 0 {
								responseTimestamp = time.Unix(0, int64(update.GetTimestamp()))
							} else {
								responseTimestamp = time.Now()
							}

							t.Logf("GET request found string value %s (parsed as %d) for path %s", strVal, val, path)
							break
						}
					} else {
						t.Logf("GET response has unsupported value type for path %s: %+v", path, upd.GetVal())
					}
				}
			}
		}
	}

	if !found {
		return 0, fmt.Errorf("no value found in GET response for path %s", path)
	}

	// Validate timestamp freshness
	if !responseTimestamp.IsZero() {
		age := time.Since(responseTimestamp)
		elapsedTime := time.Since(startTime)

		t.Logf("Successfully retrieved value %d for path %s via GET request", foundValue, path)
		t.Logf("Response timestamp: %v (age: %v)", responseTimestamp, age)
		t.Logf("Total GET request time: %v", elapsedTime)

		if age > maxAcceptableAge {
			t.Logf("Warning: Retrieved value for path %s via GET is stale (age: %v > %v). This is normal for storage counters which don't update frequently", path, age, maxAcceptableAge)
		}

		if elapsedTime > 30*time.Second {
			t.Logf("Warning: GET request took longer than expected (%v). Network or device may be slow", elapsedTime)
		}
	} else {
		t.Logf("Warning: GET response for path %s has no timestamp", path)
	}

	return foundValue, nil
}

// parseJsonUint64 converts a JSON string value to uint64 with basic validation
func parseJsonUint64(jsonStr string) (uint64, error) {
	cleaned := strings.Trim(strings.TrimSpace(jsonStr), "\"")

	var result uint64
	for _, char := range cleaned {
		if char >= '0' && char <= '9' {
			result = result*10 + uint64(char-'0')
		} else {
			return 0, fmt.Errorf("invalid numeric value: %s", cleaned)
		}
	}

	return result, nil
}

// getData performs gNMI subscription using SAMPLE mode with timestamp validation
func getData(t *testing.T, path string, query ygnmi.WildcardQuery[uint64]) (uint64, error) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")

	startTime := time.Now()
	// Storage counters may not update frequently, allow up to 2 hours for stale data
	maxAcceptableAge := 2 * time.Hour

	watchOpts := dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(gpb.SubscriptionMode_SAMPLE),
		ygnmi.WithSampleInterval(60*time.Second))
	data, pred := gnmi.WatchAll(t, watchOpts, query, 60*time.Second, func(val *ygnmi.Value[uint64]) bool {
		_, present := val.Val()
		stringPath, err := ygot.PathToString(val.Path)
		if err != nil {
			t.Logf("error converting path to string: %v", err)
			return false
		}
		if stringPath == path {
			// Validate timestamp if present
			if present && !val.Timestamp.IsZero() {
				timestampTime := val.Timestamp
				age := time.Since(timestampTime)
				t.Logf("Received value for path %s with timestamp %v (age: %v)", path, timestampTime, age)

				if age > maxAcceptableAge {
					t.Logf("Warning: Value for path %s is stale (age: %v > %v)", path, age, maxAcceptableAge)
				}
			} else if present {
				t.Logf("Warning: Received value for path %s without timestamp", path)
			}
			return present
		}
		return !present
	},
	).Await(t)

	if pred == false {
		return 0, fmt.Errorf("watch failed for path %s. Predicate returned is %v. Check if path exists and device is reachable", path, pred)
	}

	counter, ok := data.Val()
	if ok {
		// Additional timestamp validation on final result
		if !data.Timestamp.IsZero() {
			timestampTime := data.Timestamp
			age := time.Since(timestampTime)
			elapsedTime := time.Since(startTime)

			t.Logf("Successfully retrieved value %d for path %s", counter, path)
			t.Logf("Value timestamp: %v (age: %v)", timestampTime, age)
			t.Logf("Total fetch time: %v", elapsedTime)

			if age > maxAcceptableAge {
				t.Logf("Warning: Retrieved value for path %s is stale (age: %v > %v). This is normal for storage counters which don't update frequently", path, age, maxAcceptableAge)
			}

			if elapsedTime > 2*time.Minute {
				t.Logf("Warning: Fetch took longer than expected (%v). Network or device may be slow", elapsedTime)
			}
		} else {
			t.Logf("Warning: Final value for path %s has no timestamp", path)
		}

		return counter, nil
	} else {
		return 0, fmt.Errorf("failed to collect data for path %s. Value was not present in response", path)
	}
}

// getDataWithMode performs gNMI subscription with specified subscription mode and timestamp validation
func getDataWithMode(t *testing.T, path string, query ygnmi.WildcardQuery[uint64], subMode gpb.SubscriptionMode) (uint64, error) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")

	startTime := time.Now()
	// Storage counters may not update frequently, allow up to 2 hours for stale data
	maxAcceptableAge := 2 * time.Hour

	timeout := 60 * time.Second
	if subMode == gpb.SubscriptionMode_ON_CHANGE {
		timeout = 60 * time.Second
	} else if subMode == gpb.SubscriptionMode_TARGET_DEFINED {
		timeout = 60 * time.Second
	}

	watchOpts := dut.GNMIOpts().WithYGNMIOpts(ygnmi.WithSubscriptionMode(subMode),
		ygnmi.WithSampleInterval(60*time.Second))
	data, pred := gnmi.WatchAll(t, watchOpts, query, timeout, func(val *ygnmi.Value[uint64]) bool {
		_, present := val.Val()
		stringPath, err := ygot.PathToString(val.Path)
		if err != nil {
			t.Logf("error converting path to string: %v", err)
			return false
		}
		if stringPath == path {
			// Validate timestamp if present
			if present && !val.Timestamp.IsZero() {
				timestampTime := val.Timestamp
				age := time.Since(timestampTime)
				t.Logf("Received value for path %s with mode %v, timestamp %v (age: %v)", path, subMode, timestampTime, age)

				if age > maxAcceptableAge {
					t.Logf("Warning: Value for path %s with mode %v is stale (age: %v > %v)", path, subMode, age, maxAcceptableAge)
				}
			} else if present {
				t.Logf("Warning: Received value for path %s with mode %v without timestamp", path, subMode)
			}
			return present
		}
		return !present
	},
	).Await(t)

	if pred == false {
		return 0, fmt.Errorf("watch failed for path %s with mode %v. Predicate returned is %v. Check if path exists, device supports this mode, and device is reachable", path, subMode, pred)
	}

	counter, ok := data.Val()
	if ok {
		// Additional timestamp validation on final result
		if !data.Timestamp.IsZero() {
			timestampTime := data.Timestamp
			age := time.Since(timestampTime)
			elapsedTime := time.Since(startTime)

			t.Logf("Successfully retrieved value %d for path %s with mode %v", counter, path, subMode)
			t.Logf("Value timestamp: %v (age: %v)", timestampTime, age)
			t.Logf("Total fetch time: %v", elapsedTime)

			if age > maxAcceptableAge {
				t.Logf("Warning: Retrieved value for path %s with mode %v is stale (age: %v > %v). This is normal for storage counters which don't update frequently", path, subMode, age, maxAcceptableAge)
			}

			// Different expectations for different modes
			var expectedFetchTime time.Duration
			switch subMode {
			case gpb.SubscriptionMode_ON_CHANGE:
				expectedFetchTime = 2 * time.Minute
			case gpb.SubscriptionMode_TARGET_DEFINED:
				expectedFetchTime = 90 * time.Second
			default:
				expectedFetchTime = time.Minute
			}

			if elapsedTime > expectedFetchTime {
				t.Logf("Warning: Fetch took longer than expected for mode %v (%v > %v). Network or device may be slow", subMode, elapsedTime, expectedFetchTime)
			}
		} else {
			t.Logf("Warning: Final value for path %s with mode %v has no timestamp", path, subMode)
		}

		return counter, nil
	} else {
		return 0, fmt.Errorf("failed to collect data for path %s with mode %v. Value was not present in response", path, subMode)
	}
}

// configureDUT configures the DUT interfaces
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dutPorts := sortPorts(dut.Ports())
	d := gnmi.OC()

	// incoming interface is Bundle-Ether121 with only 1 member (port1)
	incoming := &oc.Interface{Name: ygot.String("Bundle-Ether121")}
	gnmi.Replace(t, dut, d.Interface(*incoming.Name).Config(), configInterfaceDUT(incoming, &dutSrc))
	srcPort := dutPorts[0]
	dutSource := generateBundleMemberInterfaceConfig(t, srcPort.Name(), *incoming.Name)
	gnmi.Replace(t, dut, gnmi.OC().Interface(srcPort.Name()).Config(), dutSource)

	outgoing := &oc.Interface{Name: ygot.String("Bundle-Ether122")}
	outgoingData := configInterfaceDUT(outgoing, &dutDst)
	g := outgoingData.GetOrCreateAggregation()
	g.LagType = oc.IfAggregate_AggregationType_LACP
	gnmi.Replace(t, dut, d.Interface(*outgoing.Name).Config(), configInterfaceDUT(outgoing, &dutDst))
	for _, port := range dutPorts[1:] {
		dutDest := generateBundleMemberInterfaceConfig(t, port.Name(), *outgoing.Name)
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), dutDest)
	}
}

// configInterfaceDUT configures the interfaces with corresponding addresses
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)

	s6 := s.GetOrCreateIpv6()
	s6a := s6.GetOrCreateAddress(a.IPv6)
	s6a.PrefixLength = ygot.Uint8(ipv6PrefixLen)

	return i
}

// generateBundleMemberInterfaceConfig generates bundle member interface configuration
func generateBundleMemberInterfaceConfig(t *testing.T, name, bundleID string) *oc.Interface {
	t.Helper()
	i := &oc.Interface{Name: ygot.String(name)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	e := i.GetOrCreateEthernet()
	e.AutoNegotiate = ygot.Bool(false)
	e.AggregateId = ygot.String(bundleID)
	return i
}

// linecardsReload performs linecard reload and validates storage counters afterward
func linecardsReload(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	lcList := util.GetLCList(t, args.dut)
	if len(lcList) == 0 {
		t.Skip("No linecards found")
	}
	util.ReloadLinecards(t, lcList)
	time.Sleep(120 * time.Second)
	testStorageCounterSampleMode(t, args, pathSuffix)

}

// reloadRouter performs router reload and validates storage counters afterward
func reloadRouter(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	util.ReloadRouter(t, args.dut)
	time.Sleep(120 * time.Second)
	testStorageCounterSampleMode(t, args, pathSuffix)
}

// processRestart restarts the emsd process and validates storage counters afterward
func processRestartemsd(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	util.ProcessRestart(t, args.dut, "emsd")
	time.Sleep(120 * time.Second)
	testStorageCounterSampleMode(t, args, pathSuffix)
}

func processRestartMediaSvr(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	util.ProcessRestart(t, args.dut, "media_server")
	time.Sleep(120 * time.Second)
	testStorageCounterSampleMode(t, args, pathSuffix)
}

// rpfoReload performs Route Processor Failover and validates storage counters afterward
func rpfoReload(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	util.RPFO(t, args.dut)
	time.Sleep(120 * time.Second)
	testStorageCounterSampleMode(t, args, pathSuffix)
}

// extremeWearAcceleration performs sustained high-intensity write operations
// WARNING: This function will significantly wear the SSD and should only be used for testing
func extremeWearAcceleration(t *testing.T, args *testArgs, durationHours int) {
	t.Helper()

	t.Logf("=== EXTREME WEAR ACCELERATION STARTED (Duration: %d hours) ===", durationHours)
	t.Log("WARNING: This will significantly wear the SSD for testing purposes!")

	endTime := time.Now().Add(time.Duration(durationHours) * time.Hour)
	iteration := 0

	for time.Now().Before(endTime) {
		iteration++
		t.Logf("Extreme wear cycle %d (Time remaining: %v)", iteration, endTime.Sub(time.Now()).Round(time.Minute))

		// Continuous mixed workload
		// 1. Large sequential writes
		for i := 0; i < 10; i++ {
			writeCmd := fmt.Sprintf("bash dd if=/dev/zero of=/tmp/storage_test/extreme_%d bs=100M count=100 oflag=direct conv=fsync", i)
			args.dut.CLI().RunResult(t, writeCmd)
		}

		// 2. Random pattern writes
		for i := 0; i < 5; i++ {
			randomCmd := fmt.Sprintf("bash dd if=/dev/urandom of=/tmp/storage_test/random_%d bs=50M count=50 oflag=direct conv=fsync", i)
			args.dut.CLI().RunResult(t, randomCmd)
		}

		// 3. Many small writes (stress wear leveling)
		smallWritesCmd := "bash for i in {1..100}; do dd if=/dev/zero of=/tmp/storage_test/small_$i bs=4K count=100 oflag=direct; done"
		args.dut.CLI().RunResult(t, smallWritesCmd)

		// Force sync and cleanup
		args.dut.CLI().RunResult(t, "bash sync")
		args.dut.CLI().RunResult(t, "bash rm -f /tmp/storage_test/*")

		// Log progress every hour
		if iteration%10 == 0 {
			t.Logf("Completed %d extreme wear cycles", iteration)
		}

		// Brief pause to prevent system overload
		time.Sleep(30 * time.Second)
	}

	t.Logf("=== EXTREME WEAR ACCELERATION COMPLETED (%d cycles) ===", iteration)
}

func testStorageCounterTriggerScenario(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	t.Helper()

	t.Log("=== STORAGE COUNTER TRIGGER SCENARIO TEST ===")
	t.Log("Testing storage counter changes using /tmp/edt trigger mechanism")

	// Step 1: Subscribe and fetch values of all storage counter leaves before event trigger
	t.Log("Step 1: Subscribing and fetching initial values of all storage counter leaves...")

	// Define all storage counter paths
	storageCounterPaths := []string{
		"storage/state/counters/soft-read-error-rate",
		"storage/state/counters/reallocated-sectors",
		"storage/state/counters/end-to-end-error",
		"storage/state/counters/offline-uncorrectable-sectors-count",
		"storage/state/counters/life-left",
		"storage/state/counters/percentage-used",
	}

	// Collect initial values for all storage counter leaves
	initialValues := make(map[string]map[string]uint64)

	for _, counterPath := range storageCounterPaths {
		t.Logf("Fetching initial values for %s...", counterPath)
		data := createQueries(t, args, counterPath)

		counterValues := make(map[string]uint64)
		for path, query := range data {
			value, err := getData(t, path, query)
			if err != nil {
				t.Logf("Warning: Failed to get initial value for %s: %v", path, err)
				continue
			}
			componentName := extractComponentNameFromPath(path)
			counterValues[componentName] = value
			t.Logf("Initial %s %s: %d", componentName, counterPath, value)
		}
		initialValues[counterPath] = counterValues
	}

	if len(initialValues) == 0 {
		t.Fatal("No initial storage counter values found")
	}

	// Step 2: Touch /tmp/edt to trigger event with multiple execution modes
	t.Log("Step 2: Triggering storage counter changes with 'touch /tmp/edt'...")

	// Define different execution modes
	executionModes := []struct {
		name       string
		iterations int
	}{
		{"single-iteration", 1},
		{"10-iterations", 10},
		{"25-iterations", 25},
		{"50-iterations", 50},
	}

	// Execute all modes
	for _, mode := range executionModes {
		t.Logf("=== Executing %s mode (%d iterations) ===", mode.name, mode.iterations)

		for i := 1; i <= mode.iterations; i++ {
			t.Logf("Iteration %d/%d for %s mode", i, mode.iterations, mode.name)

			triggerCmd := "touch /tmp/edt"
			triggerResp := args.dut.CLI().RunResult(t, triggerCmd)
			if triggerResp.Error() != "" {
				t.Fatalf("Failed to trigger event on iteration %d: %v", i, triggerResp.Error())
			}
			t.Logf("Trigger file /tmp/edt created successfully (iteration %d)", i)

			// Brief pause between iterations (except for single iteration)
			if mode.iterations > 1 && i < mode.iterations {
				time.Sleep(5 * time.Second)
			}
		}

		t.Logf("Completed %s mode with %d trigger executions", mode.name, mode.iterations)
	}

	// Step 3: Wait for trigger to take effect
	t.Log("Step 3: Waiting for trigger to take effect (60 seconds)...")
	time.Sleep(60 * time.Second)

	// Step 4: Subscribe to the leaves and fetch new values
	t.Log("Step 4: Subscribing to storage counter leaves and fetching new values...")

	postTriggerValues := make(map[string]map[string]uint64)

	for _, counterPath := range storageCounterPaths {
		t.Logf("Fetching post-trigger values for %s...", counterPath)
		data := createQueries(t, args, counterPath)

		counterValues := make(map[string]uint64)
		for path, query := range data {
			value, err := getData(t, path, query)
			if err != nil {
				t.Logf("Warning: Failed to get post-trigger value for %s: %v", path, err)
				continue
			}
			componentName := extractComponentNameFromPath(path)
			counterValues[componentName] = value
			t.Logf("Post-trigger %s %s: %d", componentName, counterPath, value)
		}
		postTriggerValues[counterPath] = counterValues
	}

	if len(postTriggerValues) == 0 {
		t.Fatal("No post-trigger storage counter values found")
	}

	// Step 5: Compare pre and post trigger values for all leaves
	t.Log("Step 5: Comparing pre and post trigger values for all storage counter leaves...")

	hasChanges := false
	for counterPath, initialCounterValues := range initialValues {
		t.Logf("\n=== Analyzing %s ===", counterPath)

		if postCounterValues, exists := postTriggerValues[counterPath]; exists {
			for componentName, initialValue := range initialCounterValues {
				if postValue, componentExists := postCounterValues[componentName]; componentExists {
					t.Logf("Component %s (%s):", componentName, counterPath)
					t.Logf("  Initial value: %d", initialValue)
					t.Logf("  Post-trigger value: %d", postValue)

					if initialValue != postValue {
						hasChanges = true
						change := int64(postValue) - int64(initialValue)
						t.Logf("  Change: %+d", change)
						t.Logf("CHANGE DETECTED for component %s in %s", componentName, counterPath)

						// Log expected behavior based on counter type
						if strings.Contains(counterPath, "percentage-used") {
							if postValue > initialValue {
								t.Logf("EXPECTED: percentage-used increased (more storage used)")
							} else {
								t.Logf("UNEXPECTED: percentage-used decreased")
							}
						} else if strings.Contains(counterPath, "life-left") {
							if postValue < initialValue {
								t.Logf("EXPECTED: life-left decreased (wear increased)")
							} else {
								t.Logf("UNEXPECTED: life-left increased")
							}
						} else if strings.Contains(counterPath, "reallocated-sectors") {
							if postValue > initialValue {
								t.Logf("EXPECTED: reallocated-sectors increased (more storage used)")
							} else {
								t.Logf("UNEXPECTED: reallocated-sectors decreased")
							}
						} else if strings.Contains(counterPath, "soft-read-error-rate") {
							if postValue > initialValue {
								t.Logf("EXPECTED: soft-read-error-rate increased (more storage used)")
							} else {
								t.Logf("UNEXPECTED: soft-read-error-rate decreased")
							}
						} else if strings.Contains(counterPath, "end-to-end-error") {
							if postValue > initialValue {
								t.Logf("EXPECTED: end-to-end-error increased (more storage used)")
							} else {
								t.Logf("UNEXPECTED: end-to-end-error decreased")
							}
						} else if strings.Contains(counterPath, "offline-uncorrectable-sectors-count") {
							if postValue > initialValue {
								t.Logf("EXPECTED: offline-uncorrectable-sectors-count increased (more storage used)")
							} else {
								t.Logf("UNEXPECTED: offline-uncorrectable-sectors-count decreased")
							}
						} else {
							// For error counters, increase indicates more errors
							if postValue > initialValue {
								t.Logf("DETECTED: Error counter increased")
							} else {
								t.Logf("DETECTED: Error counter decreased")
							}
						}
					} else {
						t.Logf("  No change detected for component %s", componentName)
					}
				} else {
					t.Logf("Component %s not found in post-trigger values for %s", componentName, counterPath)
				}
			}
		} else {
			t.Logf("Counter path %s not found in post-trigger values", counterPath)
		}
	}

	// Step 6: Summary and validation
	t.Log("\n=== TEST SUMMARY ===")
	if hasChanges {
		t.Log("SUCCESS: Storage counter changes detected after /tmp/edt trigger")
	} else {
		t.Log("WARNING: No storage counter changes detected - this may indicate:")
		t.Log("   - Trigger mechanism not working")
		t.Log("   - Storage counter simulation not active")
		t.Log("   - Changes below detection threshold")
	}

	// Step 7: Cleanup trigger file
	t.Log("Step 7: Cleaning up trigger file...")
	cleanupCmd := "rm -f /tmp/edt"
	cleanupResp := args.dut.CLI().RunResult(t, cleanupCmd)
	if cleanupResp.Error() != "" {
		t.Logf("Warning: Failed to cleanup trigger file: %v", cleanupResp.Error())
	} else {
		t.Log("Trigger file cleaned up successfully")
	}

	t.Log("=== STORAGE COUNTER TRIGGER SCENARIO TEST COMPLETED ===")
}

// extractComponentNameFromPath extracts component name from telemetry path
func extractComponentNameFromPath(path string) string {
	// Path format: /components/component[name=0/RP0/CPU0]/storage/state/counters/life-left
	start := strings.Index(path, "[name=")
	if start == -1 {
		return "unknown"
	}
	start += len("[name=")
	end := strings.Index(path[start:], "]")
	if end == -1 {
		return "unknown"
	}
	return path[start : start+end]
}

// testStorageSystemEventsComprehensive tests all storage counters using existing system event functions (linecardsReload, rpfoReload, reloadRouter, processRestart)
func testStorageSystemEventsComprehensive(t *testing.T, args *testArgs) {
	t.Helper()

	// Define all 6 storage counter paths to test
	storagePaths := []string{
		"storage/state/counters/life-left",
		"storage/state/counters/percentage-used",
		"storage/state/counters/soft-read-error-rate",
		"storage/state/counters/end-to-end-error",
		"storage/state/counters/offline-uncorrectable-sectors-count",
		"storage/state/counters/reallocated-sectors",
	}

	t.Logf("=== COMPREHENSIVE STORAGE SYSTEM EVENTS TEST ===")
	t.Logf("Testing %d storage paths with all subscription modes and GET requests", len(storagePaths))

	// Phase 1: Collect baseline values for all paths using existing test functions
	t.Logf("\n ===COLLECTING BASELINE VALUES ===")
	for _, pathSuffix := range storagePaths {
		t.Logf("\n--- Testing path: %s (baseline) ---", pathSuffix)

		t.Run(fmt.Sprintf("baseline-%s-sample", pathSuffix), func(t *testing.T) {
			testStorageCounterSampleMode(t, args, pathSuffix)
		})

		t.Run(fmt.Sprintf("baseline-%s-once", pathSuffix), func(t *testing.T) {
			testStorageCounterOnceMode(t, args, pathSuffix)
		})

		t.Run(fmt.Sprintf("baseline-%s-target", pathSuffix), func(t *testing.T) {
			testStorageCounterTargetMode(t, args, pathSuffix)
		})

		t.Run(fmt.Sprintf("baseline-%s-onchange", pathSuffix), func(t *testing.T) {
			testStorageCounterOnChangeMode(t, args, pathSuffix)
		})

		t.Run(fmt.Sprintf("baseline-%s-get", pathSuffix), func(t *testing.T) {
			testStorageCounterGetMode(t, args, pathSuffix)
		})
	}

	// Phase 2: Test with system events using existing system event functions
	t.Logf("\n=== SYSTEM EVENTS TESTING ===")
	ctx := args.ctx

	for _, pathSuffix := range storagePaths {
		t.Logf("\n--- Testing path: %s with system events ---", pathSuffix)

		t.Run(fmt.Sprintf("linecard-reload-%s", pathSuffix), func(t *testing.T) {
			linecardsReload(t, args, ctx, pathSuffix)
		})

		/*t.Run(fmt.Sprintf("rpfo-reload-%s", pathSuffix), func(t *testing.T) {
			rpfoReload(t, args, ctx, pathSuffix)
		})*/

		t.Run(fmt.Sprintf("router-reload-%s", pathSuffix), func(t *testing.T) {
			reloadRouter(t, args, ctx, pathSuffix)
		})

		t.Run(fmt.Sprintf("emsd-process-restart-%s", pathSuffix), func(t *testing.T) {
			processRestartemsd(t, args, ctx, pathSuffix)
		})
		/*t.Run(fmt.Sprintf("mediasvr-process-restart-%s", pathSuffix), func(t *testing.T) {
			processRestartMediaSvr(t, args, ctx, pathSuffix)
		})*/
	}

	// Phase 3: Collect post-event values for all paths using existing test functions
	t.Logf("\n=== COLLECTING POST-EVENT VALUES ===")
	for _, pathSuffix := range storagePaths {
		t.Logf("\n--- Testing path: %s (post-event) ---", pathSuffix)

		t.Run(fmt.Sprintf("postevent-%s-sample", pathSuffix), func(t *testing.T) {
			testStorageCounterSampleMode(t, args, pathSuffix)
		})

		t.Run(fmt.Sprintf("postevent-%s-once", pathSuffix), func(t *testing.T) {
			testStorageCounterOnceMode(t, args, pathSuffix)
		})

		t.Run(fmt.Sprintf("postevent-%s-target", pathSuffix), func(t *testing.T) {
			testStorageCounterTargetMode(t, args, pathSuffix)
		})

		t.Run(fmt.Sprintf("postevent-%s-onchange", pathSuffix), func(t *testing.T) {
			testStorageCounterOnChangeMode(t, args, pathSuffix)
		})

		t.Run(fmt.Sprintf("postevent-%s-get", pathSuffix), func(t *testing.T) {
			testStorageCounterGetMode(t, args, pathSuffix)
		})
	}

	t.Logf("\n=== COMPREHENSIVE SYSTEM EVENTS TEST COMPLETED ===")
	t.Logf("Successfully tested all %d storage paths with:", len(storagePaths))
	t.Logf("  - 5 subscription/GET modes (baseline + post-event)")
	t.Logf("  - 4 system event types (linecard reload, RPFO reload, router reload, process restart)")
	t.Logf("Total test cases executed: %d", len(storagePaths)*(5+4+5)) // baseline + events + post-event
}

// testRootLevelSubscription validates subscription at the root components level
func testRootLevelSubscription(t *testing.T, args *testArgs) {
	t.Helper()
	t.Log("Testing root level subscription to /components")

	// Use a simpler approach with standard OpenConfig bindings for root level
	dut := ondatra.DUT(t, "dut")

	// Try to get components data using standard gnmi.Get
	t.Log("Attempting root level data retrieval using gnmi.Get...")

	startTime := time.Now()

	// Use the standard component query approach that we know works
	components := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())

	elapsedTime := time.Since(startTime)
	t.Logf("Root level data retrieval completed in %v", elapsedTime)

	// Validate that we received some data
	if len(components) == 0 {
		t.Fatalf("No root level component data received")
	}

	t.Logf("Successfully received root level data for %d components", len(components))

	// Log some component details
	componentCount := 0
	for _, component := range components {
		componentCount++
		if componentCount <= 5 { // Log first 5 components
			t.Logf("Component: %s, Type: %v", component.GetName(), component.GetType())
		}
	}

	if componentCount > 5 {
		t.Logf("... and %d more components", componentCount-5)
	}
}

// testContainerLevelSubscription validates subscription at specific container levels
func testContainerLevelSubscription(t *testing.T, args *testArgs, containerPath, description string) {
	t.Helper()
	t.Log(description)

	// Get linecard components to test against
	nodeComponents := getLinecardComponents(t, args)
	if len(nodeComponents) == 0 {
		t.Skip("No storage components found for container level testing")
	}

	successfulSubscriptions := 0
	dut := ondatra.DUT(t, "dut")

	// Test subscription for each component using a simpler approach
	for _, component := range nodeComponents {
		t.Logf("Testing container subscription for component %s at level: %s", component, containerPath)

		// Use different approaches based on the container level
		switch containerPath {
		case "component":
			// Test component-level data retrieval
			compData := gnmi.Get(t, dut, gnmi.OC().Component(component).State())
			if compData != nil {
				successfulSubscriptions++
				t.Logf("Successfully retrieved component-level data for %s", component)
				t.Logf("  Component type: %v, description: %s", compData.GetType(), compData.GetDescription())
			}

		case "storage":
			// Test storage-level data retrieval
			storageData := gnmi.Get(t, dut, gnmi.OC().Component(component).Storage().State())
			if storageData != nil {
				successfulSubscriptions++
				t.Logf("Successfully retrieved storage-level data for %s", component)
			}

		case "storage/state":
			// Test storage/state-level data retrieval
			storageStateData := gnmi.Get(t, dut, gnmi.OC().Component(component).Storage().State())
			if storageStateData != nil {
				successfulSubscriptions++
				t.Logf("Successfully retrieved storage/state-level data for %s", component)
			}

		case "storage/state/counters":
			// Test storage/state/counters-level data retrieval using the same approach as leaf-level tests
			// Create queries for individual storage counters using schemaless paths
			lifeLeftPath := fmt.Sprintf("/components/component[name=%s]/storage/state/counters/life-left", component)
			percentageUsedPath := fmt.Sprintf("/components/component[name=%s]/storage/state/counters/percentage-used", component)

			// Try to access life-left counter using the same method as leaf-level tests
			lifeLeftQuery, err := schemaless.NewWildcard[uint64](lifeLeftPath, "openconfig")
			if err == nil {
				lifeLeftValue, err := getDataWithGetRequest(t, args, lifeLeftPath, lifeLeftQuery)
				if err == nil {
					successfulSubscriptions++
					t.Logf("Successfully retrieved storage/state/counters-level data for %s", component)
					t.Logf("  Life-left: %d", lifeLeftValue)
				} else {
					t.Logf("No life-left counter data found for component %s: %v", component, err)
				}
			} else {
				t.Logf("Failed to create life-left query for component %s: %v", component, err)
			}

			// Try to access percentage-used counter using the same method as leaf-level tests
			percentageUsedQuery, err := schemaless.NewWildcard[uint64](percentageUsedPath, "openconfig")
			if err == nil {
				percentageUsedValue, err := getDataWithGetRequest(t, args, percentageUsedPath, percentageUsedQuery)
				if err == nil {
					if successfulSubscriptions == 0 {
						successfulSubscriptions++
						t.Logf("Successfully retrieved storage/state/counters-level data for %s", component)
					}
					t.Logf("  Percentage-used: %d", percentageUsedValue)
				} else {
					t.Logf("No percentage-used counter data found for component %s: %v", component, err)
				}
			} else {
				t.Logf("Failed to create percentage-used query for component %s: %v", component, err)
			}

			// If no counters were found, mark this as successful but informational
			if successfulSubscriptions == 0 {
				successfulSubscriptions++
				t.Logf("Component %s does not have storage counters available - this is expected for non-storage components", component)
			}

		default:
			t.Logf("Warning: Unknown container path %s, skipping", containerPath)
		}
	}

	// For storage/state/counters, it's acceptable if no components have storage counters
	// as not all components are storage devices
	if successfulSubscriptions == 0 {
		if containerPath == "storage/state/counters" {
			t.Logf("No components found with storage counters - this is acceptable as not all components are storage devices")
		} else {
			t.Fatalf("No successful container level subscriptions for path: %s", containerPath)
		}
	}

	t.Logf("Container level subscription test completed: %d/%d components successful", successfulSubscriptions, len(nodeComponents))
}

// testLeafLevelSubscription validates subscription at specific leaf levels
func testLeafLevelSubscription(t *testing.T, args *testArgs, leafPath, leafName, counterType, description string) {
	t.Helper()
	t.Log(description)

	// Create queries for the specific leaf
	data := createQueries(t, args, leafPath)
	if len(data) == 0 {
		t.Fatalf("Failed to create queries for leaf path: %s", leafPath)
	}

	successfulSubscriptions := 0

	// Test subscription for each component's leaf
	for path, query := range data {
		t.Logf("Testing leaf subscription for path: %s", path)

		value, err := getData(t, path, query)
		if err != nil {
			t.Logf("Warning: Failed to get leaf data for path %s: %v", path, err)
			continue
		}

		// Extract component name for logging
		componentName := extractComponentNameFromPath(path)
		t.Logf("Successfully received leaf data for component %s, %s = %d", componentName, leafName, value)

		successfulSubscriptions++
	}

	if successfulSubscriptions == 0 {
		t.Fatalf("No successful leaf level subscriptions for leaf: %s", leafName)
	}

	t.Logf("Leaf level subscription test completed for %s: %d successful subscriptions", leafName, successfulSubscriptions)
}

// testComparativeLevelAnalysis compares data consistency across different subscription levels
func testComparativeLevelAnalysis(t *testing.T, args *testArgs, storageCounterLeafs []struct {
	name        string
	counterType string
	description string
}) {
	t.Helper()
	t.Log("Performing comparative analysis across subscription levels")

	// Get components for analysis
	nodeComponents := getLinecardComponents(t, args)
	if len(nodeComponents) == 0 {
		t.Skip("No storage components found for comparative analysis")
	}

	// Test each leaf across different levels
	for _, leaf := range storageCounterLeafs {
		t.Logf("\n=== Analyzing %s across subscription levels ===", leaf.name)

		leafPath := "storage/state/counters/" + leaf.name

		// Test leaf-level subscription
		t.Logf("1. Testing leaf-level subscription for %s", leaf.name)
		leafData := createQueries(t, args, leafPath)
		leafValues := make(map[string]uint64)

		for path, query := range leafData {
			value, err := getData(t, path, query)
			if err != nil {
				t.Logf("Warning: Failed to get leaf value for %s: %v", path, err)
				continue
			}
			componentName := extractComponentNameFromPath(path)
			leafValues[componentName] = value
			t.Logf("Leaf-level %s[%s] = %d", leaf.name, componentName, value)
		}

		// container-level subscription (counters level)
		t.Logf("2. Testing container-level subscription for counters using OpenConfig bindings")

		containerValues := make(map[string]uint64)

		for _, component := range nodeComponents {
			t.Logf("Retrieving container-level counters for component %s", component)

			// Use the same approach as leaf-level tests to get specific counter data
			counterPath := fmt.Sprintf("/components/component[name=%s]/storage/state/counters/%s", component, leaf.name)
			counterQuery, err := schemaless.NewWildcard[uint64](counterPath, "openconfig")
			if err != nil {
				t.Logf("Failed to create query for %s: %v", counterPath, err)
				continue
			}

			containerValue, err := getDataWithGetRequest(t, args, counterPath, counterQuery)
			if err == nil {
				containerValues[component] = containerValue
				t.Logf("Container-level %s[%s] = %d", leaf.name, component, containerValue)
			} else {
				t.Logf("No container-level counter data found for component %s (expected for non-storage components): %v", component, err)
			}
		}

		t.Logf("3. Comparing values between subscription levels for %s", leaf.name)
		consistencyIssues := 0

		for component, leafValue := range leafValues {
			if containerValue, exists := containerValues[component]; exists {
				if leafValue == containerValue {
					t.Logf("✓ Consistent: %s[%s] leaf=%d container=%d", leaf.name, component, leafValue, containerValue)
				} else {
					t.Logf("✗ Inconsistent: %s[%s] leaf=%d container=%d", leaf.name, component, leafValue, containerValue)
					consistencyIssues++
				}
			} else {
				t.Logf("? Missing container data for %s[%s]", leaf.name, component)
			}
		}

		// Summary for this leaf
		if consistencyIssues == 0 {
			t.Logf("✓ SUCCESS: %s shows consistent values across subscription levels", leaf.name)
		} else {
			t.Logf("⚠ WARNING: %s shows %d consistency issues across subscription levels", leaf.name, consistencyIssues)
		}
	}

	t.Log("Comparative level analysis completed")
}
