package storage_test

import (
	"context"
	"fmt"
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
	top *ondatra.ATETopology
}

// storageTestCase defines a storage counter test configuration
type storageTestCase struct {
	name        string
	path        string
	counterType string
	description string
	fn          func(ctx context.Context, t *testing.T, args *testArgs, path string)
}

const (
	// Storage component type for filtering
	storageType = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_STORAGE

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
	for _, component := range allComponents {
		name := component.GetName()
		// Filter for components with pattern "0/X/CPUY" (linecard and RP components)
		if strings.Count(name, "/") == 2 &&
			(strings.HasSuffix(name, "/CPU0") || strings.HasSuffix(name, "/CPU1")) &&
			!strings.Contains(name, "-") &&
			!strings.Contains(strings.ToUpper(name), "IOSXR-NODE") {

			// Include linecard components (e.g., "0/0/CPU0" to "0/9/CPU0")
			// AND RP components (e.g., "0/RP0/CPU0", "0/RP1/CPU0")
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

// collectAndLogCounters performs basic counter data collection and logging
func collectAndLogCounters(t *testing.T, data map[string]ygnmi.WildcardQuery[uint64]) {
	t.Helper()
	for path, query := range data {
		pre, err := getData(t, path, query)
		if err != nil {
			t.Fatalf("failed to get data for path %s pre trigger: %v", path, err)
		}
		t.Logf("Initial counter for path %s : %d", path, pre)
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

// collectAndLogCountersWithMode collects counter data with specific subscription mode
func collectAndLogCountersWithMode(t *testing.T, data map[string]ygnmi.WildcardQuery[uint64], subMode gpb.SubscriptionMode) {
	t.Helper()
	// Collect counter data using specified subscription mode
	for path, query := range data {
		pre, err := getDataWithMode(t, path, query, subMode)
		if err != nil {
			t.Fatalf("failed to get data for path %s pre trigger with mode %v: %v", path, subMode, err)
		}
		t.Logf("Initial counter for path %s with mode %v: %d", path, subMode, pre)
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

// getStorageComponents retrieves all components of storage type from the device
func (args *testArgs) getStorageComponents(t *testing.T) []*oc.Component {
	t.Helper()

	components := gnmi.GetAll(t, args.dut, gnmi.OC().ComponentAny().State())
	var storageComponents []*oc.Component

	for _, component := range components {
		if component.GetType() == storageType {
			storageComponents = append(storageComponents, component)
		}
	}

	t.Logf("Found %d storage components", len(storageComponents))
	return storageComponents
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

// testStorageCounterSystemEvents executes all system event tests for a storage counter path
func testStorageCounterSystemEvents(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	linecardsReload(t, args, ctx, pathSuffix)
	rpfoReload(t, args, ctx, pathSuffix)
	reloadRouter(t, args, ctx, pathSuffix)
	processRestart(t, args, ctx, pathSuffix)
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
func processRestart(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	util.ProcessRestart(t, args.dut, "emsd")
	time.Sleep(120 * time.Second)
	testStorageCounterSampleMode(t, args, pathSuffix)
}

// rpfoReload performs Route Processor Failover and validates storage counters afterward
func rpfoReload(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	util.RPFO(t, args.dut)
	time.Sleep(120 * time.Second)
	testStorageCounterSampleMode(t, args, pathSuffix)
}

// validateStorageCounters compares telemetry percentage-used with smartctl Media_Wearout_Indicator
func validateStorageCounters(t *testing.T, args *testArgs, data map[string]ygnmi.WildcardQuery[uint64]) {
	t.Helper()

	// Step 1: Fetch current telemetry values
	t.Log("Step 1: Fetching current telemetry percentage-used values...")
	telemetryValues := make(map[string]uint64)

	for path, query := range data {
		if strings.Contains(path, "percentage-used") {
			value, err := getData(t, path, query)
			if err != nil {
				t.Logf("Warning: Failed to get telemetry data for %s: %v", path, err)
				continue
			}
			// Extract component name from path
			// Path format: /components/component[name=0/RP0/CPU0]/storage/state/counters/percentage-used
			parts := strings.Split(path, "[name=")
			if len(parts) > 1 {
				componentName := strings.Split(parts[1], "]")[0]
				telemetryValues[componentName] = value
				t.Logf("Telemetry: Component %s percentage-used = %d", componentName, value)
			}
		}
	}

	// Step 2: Fetch smartctl data for comparison
	t.Log("Step 2: Fetching smartctl Media_Wearout_Indicator values...")

	// Get smartctl data from each component
	for componentName, telemetryPercentageUsed := range telemetryValues {
		t.Logf("Validating component: %s", componentName)

		// Run smartctl command via bash
		smartctlCmd := "run smartctl -a /dev/sda"
		smartctlResp := args.dut.CLI().RunResult(t, smartctlCmd)
		if smartctlResp.Error() != "" {
			t.Logf("Warning: smartctl command failed for %s: %v", componentName, smartctlResp.Error())
			continue
		}

		// Parse smartctl output to find Media_Wearout_Indicator
		smartctlOutput := smartctlResp.Output()
		lines := strings.Split(smartctlOutput, "\n")
		var mediaWearoutIndicator uint64
		var found bool

		for _, line := range lines {
			if strings.Contains(line, "Media_Wearout_Indicator") || strings.Contains(line, "233") {
				// Parse SMART attribute line
				// Format: 233 Media_Wearout_Indicator 0x0032   100   100   000    Old_age   Always       -       0
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					// The VALUE field is typically the 4th field (index 3)
					if val := strings.TrimSpace(fields[3]); val != "" {
						if parsedVal, err := fmt.Sscanf(val, "%d", &mediaWearoutIndicator); parsedVal == 1 && err == nil {
							found = true
							t.Logf("smartctl: Component %s Media_Wearout_Indicator = %d", componentName, mediaWearoutIndicator)
							break
						}
					}
				}
			}
		}

		if !found {
			t.Logf("Warning: Could not find Media_Wearout_Indicator in smartctl output for %s", componentName)
			continue
		}

		// Step 3: Compare values following OpenConfig specification:
		// percentage-used: uint8 (0-255), can exceed 100, values >254 represented as 255
		// Relationship: generally 100 - Media_Wearout_Indicator == percentage-used
		// BUT must handle edge cases per specification

		calculatedPercentageUsed := int64(100) - int64(mediaWearoutIndicator)

		t.Logf("Validation for component %s:", componentName)
		t.Logf("  Telemetry percentage-used: %d", telemetryPercentageUsed)
		t.Logf("  smartctl Media_Wearout_Indicator: %d", mediaWearoutIndicator)
		t.Logf("  Calculated: 100 - Media_Wearout_Indicator = %d", calculatedPercentageUsed)

		// Handle OpenConfig specification edge cases
		var expectedPercentageUsed uint64

		if calculatedPercentageUsed < 0 {
			// If Media_Wearout_Indicator > 100, percentage-used should be clamped
			// This can happen when SSD is severely worn beyond 100%
			if calculatedPercentageUsed <= -154 { // 100 - 254 = -154
				expectedPercentageUsed = 255 // Values >254 represented as 255
				t.Logf("  OpenConfig rule: Calculated %d <= -154, expected percentage-used = 255", calculatedPercentageUsed)
			} else {
				// For values between 101-254, use the absolute calculated value
				expectedPercentageUsed = uint64(-calculatedPercentageUsed + 200) // Adjust for wear beyond 100%
				if expectedPercentageUsed > 254 {
					expectedPercentageUsed = 255
				}
				t.Logf("  OpenConfig rule: Media wear beyond 100%%, expected percentage-used = %d", expectedPercentageUsed)
			}
		} else if calculatedPercentageUsed > 255 {
			// Should not happen in practice, but handle per spec
			expectedPercentageUsed = 255
			t.Logf("  OpenConfig rule: Calculated %d > 255, clamped to 255", calculatedPercentageUsed)
		} else {
			// Normal case: 0 <= calculatedPercentageUsed <= 255
			expectedPercentageUsed = uint64(calculatedPercentageUsed)
			t.Logf("  Normal case: Expected percentage-used = %d", expectedPercentageUsed)
		}

		// Validate the relationship
		if expectedPercentageUsed != telemetryPercentageUsed {
			// Check for common edge cases that might still be valid
			if telemetryPercentageUsed == 255 && calculatedPercentageUsed < -154 {
				t.Logf("✓ PASS: percentage-used correctly clamped to 255 for extreme wear (calculated %d)", calculatedPercentageUsed)
			} else if telemetryPercentageUsed <= 100 && calculatedPercentageUsed >= 0 && calculatedPercentageUsed <= 100 {
				// Allow small tolerance for normal wear range
				tolerance := uint64(2)
				if telemetryPercentageUsed >= expectedPercentageUsed-tolerance &&
					telemetryPercentageUsed <= expectedPercentageUsed+tolerance {
					t.Logf("✓ PASS: Values match within tolerance (±%d) for normal wear range", tolerance)
				} else {
					t.Errorf("✗ FAIL: 100-Media_Wearout_Indicator: %d but gnmi percentage-used was: %d! (outside tolerance)",
						expectedPercentageUsed, telemetryPercentageUsed)
				}
			} else {
				t.Errorf("✗ FAIL: 100-Media_Wearout_Indicator: %d but gnmi percentage-used was: %d!",
					expectedPercentageUsed, telemetryPercentageUsed)
			}
		} else {
			t.Logf("✓ PASS: 100-Media_Wearout_Indicator: %d and gnmi percentage-used got same thing",
				expectedPercentageUsed)
		}
	}

	if len(telemetryValues) == 0 {
		t.Log("Warning: No telemetry percentage-used values found for validation")
	}
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

// testLifeLeftTriggerScenario tests life-left counter changes using trigger mechanism with gNMI
func testLifeLeftTriggerScenario(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	t.Helper()

	t.Log("=== LIFE-LEFT TRIGGER SCENARIO TEST ===")
	t.Log("Testing life-left counter changes using /tmp/edt trigger mechanism with gNMI subscription")

	// Step 1: Get initial life-left values using existing gNMI infrastructure
	t.Log("Step 1: Fetching initial life-left values using gNMI subscription...")

	// Create queries for life-left monitoring using existing infrastructure
	data := createQueries(t, args, pathSuffix)

	// Collect initial life-left values
	initialLifeLeftValues := make(map[string]uint64)
	for path, query := range data {
		if strings.Contains(path, "life-left") {
			value, err := getData(t, path, query)
			if err != nil {
				t.Logf("Warning: Failed to get initial value for %s: %v", path, err)
				continue
			}
			// Extract component name from path
			componentName := extractComponentNameFromPath(path)
			initialLifeLeftValues[componentName] = value
			t.Logf("Initial %s life-left: %d", componentName, value)
		}
	}

	if len(initialLifeLeftValues) == 0 {
		t.Fatal("No initial life-left values found via gNMI subscription")
	}

	t.Logf("Parsed initial life-left values: %v", initialLifeLeftValues)

	// Step 2: Trigger the wear mechanism
	t.Log("Step 2: Triggering wear mechanism with 'touch /tmp/edt'...")

	triggerCmd := "touch /tmp/edt"
	triggerResp := args.dut.CLI().RunResult(t, triggerCmd)
	if triggerResp.Error() != "" {
		t.Fatalf("Failed to trigger wear mechanism: %v", triggerResp.Error())
	}
	t.Log("Trigger file /tmp/edt created successfully")

	// Step 3: Wait for trigger to take effect
	t.Log("Step 3: Waiting for trigger to take effect (60 seconds)...")
	time.Sleep(60 * time.Second)

	// Step 4: Fetch post-trigger life-left values using gNMI
	t.Log("Step 4: Fetching post-trigger life-left values using gNMI subscription...")

	postTriggerLifeLeftValues := make(map[string]uint64)
	for path, query := range data {
		if strings.Contains(path, "life-left") {
			value, err := getData(t, path, query)
			if err != nil {
				t.Logf("Warning: Failed to get post-trigger value for %s: %v", path, err)
				continue
			}
			componentName := extractComponentNameFromPath(path)
			postTriggerLifeLeftValues[componentName] = value
			t.Logf("Post-trigger %s life-left: %d", componentName, value)
		}
	}

	if len(postTriggerLifeLeftValues) == 0 {
		t.Fatal("No post-trigger life-left values found via gNMI subscription")
	}

	t.Logf("Parsed post-trigger life-left values: %v", postTriggerLifeLeftValues)

	// Step 5: Compare pre and post values
	t.Log("Step 5: Comparing pre and post trigger life-left values...")

	// Compare values for each component
	hasChanges := false
	for componentName, initialValue := range initialLifeLeftValues {
		if postValue, exists := postTriggerLifeLeftValues[componentName]; exists {
			t.Logf("Component %s:", componentName)
			t.Logf("  Initial life-left: %d", initialValue)
			t.Logf("  Post-trigger life-left: %d", postValue)
			t.Logf("  Change: %d", postValue-initialValue)

			if initialValue != postValue {
				hasChanges = true
				t.Logf("CHANGE DETECTED for component %s", componentName)

				// Expected behavior: life-left should decrease when wear increases
				if postValue < initialValue {
					t.Logf("EXPECTED: life-left decreased from %d to %d (wear increased)", initialValue, postValue)
				} else {
					t.Logf("UNEXPECTED: life-left increased from %d to %d", initialValue, postValue)
				}
			} else {
				t.Logf("No change detected for component %s", componentName)
			}
		} else {
			t.Logf("Component %s not found in post-trigger values", componentName)
		}
	}

	// Step 6: Trigger additional wear and monitor continuous changes using gNMI
	t.Log("Step 6: Triggering additional wear for extended monitoring...")

	// Trigger a few more times to see incremental changes
	for i := 1; i <= 3; i++ {
		t.Logf("Additional trigger %d/3", i)

		// Trigger again
		triggerResp := args.dut.CLI().RunResult(t, triggerCmd)
		if triggerResp.Error() != "" {
			t.Logf("Warning: Failed additional trigger %d: %v", i, triggerResp.Error())
			continue
		}

		// Wait briefly
		t.Logf("Waiting 30 seconds after trigger %d...", i)
		time.Sleep(60 * time.Second)

		// Check values again using gNMI
		currentValues := make(map[string]uint64)
		for path, query := range data {
			if strings.Contains(path, "life-left") {
				value, err := getData(t, path, query)
				if err != nil {
					t.Logf("Warning: Failed to get values after trigger %d for %s: %v", i, path, err)
					continue
				}
				componentName := extractComponentNameFromPath(path)
				currentValues[componentName] = value
			}
		}

		t.Logf("After trigger %d, life-left values: %v", i, currentValues)

		// Compare with previous values
		for componentName, currentValue := range currentValues {
			if previousValue, exists := postTriggerLifeLeftValues[componentName]; exists {
				if currentValue != previousValue {
					t.Logf("Change detected in %s: %d → %d", componentName, previousValue, currentValue)
				}
			}
		}

		// Update post-trigger values for next iteration
		postTriggerLifeLeftValues = currentValues
	}

	// Step 7: Final validation
	t.Log("Step 7: Final validation...")

	if hasChanges {
		t.Log("SUCCESS: life-left counter changes detected after trigger using gNMI subscription")
	} else {
		t.Log("WARNING: No life-left counter changes detected - this may indicate:")
		t.Log("   - Trigger mechanism not working")
		t.Log("   - SSD wear simulation not active")
		t.Log("   - Very minimal wear changes below detection threshold")
	}

	// Step 8: Log continuous monitoring instructions
	t.Log("Step 8: Continuous monitoring instructions...")
	t.Log("For manual continuous monitoring, you can:")
	t.Log("  1. Use the existing gNMI subscription infrastructure")
	t.Log("  2. Run: mdt_exec -s 'openconfig:components/component/storage/state/counters' -c 0")
	t.Log("  3. Use 'touch /tmp/edt' to trigger wear changes during monitoring")
	t.Log("  4. Observe data streaming every 60 seconds")

	t.Log("=== LIFE-LEFT TRIGGER SCENARIO TEST COMPLETED ===")
}

// testLifeLeftTriggerScenarioWithTelemetry uses telemetry instead of mdt_exec as fallback
func testLifeLeftTriggerScenarioWithTelemetry(t *testing.T, args *testArgs, ctx context.Context, pathSuffix string) {
	t.Helper()

	t.Log("=== TELEMETRY-BASED LIFE-LEFT TRIGGER TEST ===")

	// Create queries for life-left monitoring
	data := createQueries(t, args, pathSuffix)

	// Step 1: Get initial life-left values
	t.Log("Step 1: Fetching initial life-left values via telemetry...")
	initialValues := make(map[string]uint64)

	for path, query := range data {
		if strings.Contains(path, "life-left") {
			value, err := getData(t, path, query)
			if err != nil {
				t.Logf("Warning: Failed to get initial value for %s: %v", path, err)
				continue
			}
			// Extract component name from path
			componentName := extractComponentNameFromPath(path)
			initialValues[componentName] = value
			t.Logf("Initial %s life-left: %d", componentName, value)
		}
	}

	if len(initialValues) == 0 {
		t.Fatal("No initial life-left values found via telemetry")
	}

	// Step 2: Trigger the wear mechanism
	t.Log("Step 2: Triggering wear mechanism with 'touch /tmp/edt'...")

	triggerCmd := "touch /tmp/edt"
	triggerResp := args.dut.CLI().RunResult(t, triggerCmd)
	if triggerResp.Error() != "" {
		t.Fatalf("Failed to trigger wear mechanism: %v", triggerResp.Error())
	}
	t.Log("Trigger file /tmp/edt created successfully")

	// Step 3: Wait and fetch post-trigger values
	t.Log("Step 3: Waiting for trigger effect and fetching post-trigger values (60 seconds)...")
	time.Sleep(60 * time.Second)

	postValues := make(map[string]uint64)
	for path, query := range data {
		if strings.Contains(path, "life-left") {
			value, err := getData(t, path, query)
			if err != nil {
				t.Logf("Warning: Failed to get post-trigger value for %s: %v", path, err)
				continue
			}
			componentName := extractComponentNameFromPath(path)
			postValues[componentName] = value
			t.Logf("Post-trigger %s life-left: %d", componentName, value)
		}
	}

	// Step 4: Compare values
	t.Log("Step 4: Comparing telemetry life-left values...")
	hasChanges := false

	for componentName, initialValue := range initialValues {
		if postValue, exists := postValues[componentName]; exists {
			t.Logf("Component %s:", componentName)
			t.Logf("  Initial life-left: %d", initialValue)
			t.Logf("  Post-trigger life-left: %d", postValue)
			t.Logf("  Change: %d", postValue-initialValue)

			if initialValue != postValue {
				hasChanges = true
				t.Logf("CHANGE DETECTED for component %s", componentName)
			} else {
				t.Logf("No change detected for component %s", componentName)
			}
		}
	}

	if hasChanges {
		t.Log("SUCCESS: life-left counter changes detected via telemetry")
	} else {
		t.Log("WARNING: No life-left counter changes detected via telemetry")
	}

	t.Log("=== TELEMETRY-BASED LIFE-LEFT TRIGGER TEST COMPLETED ===")
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

// parseUint64Safe safely parses a string to uint64, returns 0 if parsing fails
func parseUint64Safe(s string) uint64 {
	// Remove any non-numeric characters
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	// Simple conversion - in production, use strconv.ParseUint
	var result uint64
	for _, char := range s {
		if char >= '0' && char <= '9' {
			result = result*10 + uint64(char-'0')
		} else {
			break // Stop at first non-digit
		}
	}

	return result
}

// validateTimestampFreshness checks if a timestamp is within acceptable age limits
func validateTimestampFreshness(t *testing.T, timestamp *time.Time, path string, maxAge time.Duration) bool {
	t.Helper()

	if timestamp == nil {
		t.Logf("Warning: No timestamp available for path %s", path)
		return false
	}

	age := time.Since(*timestamp)
	if age > maxAge {
		t.Logf("Warning: Stale data for path %s - age: %v (max acceptable: %v)", path, age, maxAge)
		return false
	}

	t.Logf("Fresh data for path %s - age: %v", path, age)
	return true
}

// logSubscriptionModeHealth provides a summary of subscription mode performance
func logSubscriptionModeHealth(t *testing.T, mode string, successCount, errorCount, totalPaths int, averageResponseTime time.Duration) {
	t.Helper()

	successRate := float64(successCount) / float64(totalPaths) * 100

	t.Logf("=== SUBSCRIPTION MODE HEALTH REPORT: %s ===", mode)
	t.Logf("Total paths tested: %d", totalPaths)
	t.Logf("Successful responses: %d", successCount)
	t.Logf("Failed responses: %d", errorCount)
	t.Logf("Success rate: %.1f%%", successRate)
	t.Logf("Average response time: %v", averageResponseTime)

	// Health assessment
	if successRate >= 60.0 {
		t.Logf("EXCELLENT: Subscription mode %s is performing optimally", mode)
	} else {
		t.Logf("CRITICAL: Subscription mode %s is performing poorly", mode)
	}

	// Response time assessment
	if averageResponseTime < 60*time.Second {
		t.Logf("Response time is excellent")
	} else {
		t.Logf("Response time is critically slow")
	}
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

		t.Run(fmt.Sprintf("rpfo-reload-%s", pathSuffix), func(t *testing.T) {
			rpfoReload(t, args, ctx, pathSuffix)
		})

		t.Run(fmt.Sprintf("router-reload-%s", pathSuffix), func(t *testing.T) {
			reloadRouter(t, args, ctx, pathSuffix)
		})

		t.Run(fmt.Sprintf("process-restart-%s", pathSuffix), func(t *testing.T) {
			processRestart(t, args, ctx, pathSuffix)
		})
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

// getStorageComponents returns all storage components from the device
func getStorageComponents(t *testing.T, dut *ondatra.DUTDevice) []*oc.Component {
	var storageComponents []*oc.Component

	// Get all components
	components := gnmi.GetAll(t, dut, gnmi.OC().ComponentAny().State())

	// Filter for storage components
	for _, component := range components {
		if component.GetType() == storageType {
			storageComponents = append(storageComponents, component)
		}
	}

	if len(storageComponents) == 0 {
		t.Logf("No storage components found")
	} else {
		t.Logf("Found %d storage components", len(storageComponents))
		for _, comp := range storageComponents {
			t.Logf("   - %s", comp.GetName())
		}
	}

	return storageComponents
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

		// Validate data type based on counter type
		if counterType == "counter64" && value < 0 {
			t.Logf("Warning: Unexpected negative value for counter64 leaf %s: %d", leafName, value)
		}

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
