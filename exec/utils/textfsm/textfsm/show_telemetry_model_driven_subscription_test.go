package textfsm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestShowTelemetryModelDrivenSubscription(t *testing.T) {
	// Get all test data files from testdata directory
	testFiles, err := filepath.Glob("../testdata/show_telemetry_model_driven_subscription/*_output.txt")
	if err != nil {
		t.Fatalf("Failed to glob test files: %v", err)
	}

	if len(testFiles) == 0 {
		t.Fatal("No test data files found in testdata/show_telemetry_model_driven_subscription/")
	}

	// Iterate over all test files
	for _, testFile := range testFiles {
		t.Run(filepath.Base(testFile), func(t *testing.T) {
			// Read CLI output from testdata file
			cliOutputBytes, err := os.ReadFile(testFile)
			if err != nil {
				t.Fatalf("Failed to read test data file: %v", err)
			}
			cliOutput := string(cliOutputBytes)

			parser := &ShowTelemetryModelDrivenSubscription{}
			err = parser.Parse(cliOutput)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if len(parser.Rows) == 0 {
				t.Logf("No rows parsed from %s, skipping validation", filepath.Base(testFile))
				return
			}

			// Load expected results from corresponding JSON file
			expectedFile := strings.TrimSuffix(testFile, "_output.txt") + "_output_expected.json"
			expectedData, err := os.ReadFile(expectedFile)
			if err != nil {
				t.Logf("No expected results file found for %s, skipping validation", filepath.Base(testFile))
				return
			}

			var expected []map[string]interface{}
			err = json.Unmarshal(expectedData, &expected)
			if err != nil {
				t.Fatalf("Failed to parse expected results JSON: %v", err)
			}

			// Verify number of rows matches
			t.Logf("Parsed %d rows", len(parser.Rows))
			for i, row := range parser.Rows {
				t.Logf("Row %d: %s", i, row.SubscriptionName)
			}
			if len(parser.Rows) != len(expected) {
				t.Fatalf("Expected %d rows, got %d", len(expected), len(parser.Rows))
			} // Compare each row
			for i, row := range parser.Rows {
				expectedRow := expected[i]

				// Compare each field
				if row.SubscriptionName != expectedRow["SUBSCRIPTION_NAME"].(string) {
					t.Errorf("Row %d: SubscriptionName mismatch. Expected %s, got %s",
						i, expectedRow["SUBSCRIPTION_NAME"], row.SubscriptionName)
				}

				if row.SubscriptionId != expectedRow["SUBSCRIPTION_ID"].(string) {
					t.Errorf("Row %d: SubscriptionId mismatch. Expected %s, got %s",
						i, expectedRow["SUBSCRIPTION_ID"], row.SubscriptionId)
				}

				if row.State != expectedRow["STATE"].(string) {
					t.Errorf("Row %d: State mismatch. Expected %s, got %s",
						i, expectedRow["STATE"], row.State)
				}

				// Compare list fields
				if !compareStringSlices(row.SensorGroupId, expectedRow["SENSOR_GROUP_ID"]) {
					t.Errorf("Row %d: SensorGroupId mismatch. Expected %v, got %v",
						i, expectedRow["SENSOR_GROUP_ID"], row.SensorGroupId)
				}

				if !compareStringSlices(row.SampleInterval, expectedRow["SAMPLE_INTERVAL"]) {
					t.Errorf("Row %d: SampleInterval mismatch. Expected %v, got %v",
						i, expectedRow["SAMPLE_INTERVAL"], row.SampleInterval)
				}

				if !compareStringSlices(row.HeartbeatInterval, expectedRow["HEARTBEAT_INTERVAL"]) {
					t.Errorf("Row %d: HeartbeatInterval mismatch. Expected %v, got %v",
						i, expectedRow["HEARTBEAT_INTERVAL"], row.HeartbeatInterval)
				}

				if !compareStringSlices(row.SensorPath, expectedRow["SENSOR_PATH"]) {
					t.Errorf("Row %d: SensorPath mismatch. Expected %v, got %v",
						i, expectedRow["SENSOR_PATH"], row.SensorPath)
				}

				if !compareStringSlices(row.SensorPathState, expectedRow["SENSOR_PATH_STATE"]) {
					t.Errorf("Row %d: SensorPathState mismatch. Expected %v, got %v",
						i, expectedRow["SENSOR_PATH_STATE"], row.SensorPathState)
				}

				if row.DestGroupId != expectedRow["DEST_GROUP_ID"].(string) {
					t.Errorf("Row %d: DestGroupId mismatch. Expected %s, got %s",
						i, expectedRow["DEST_GROUP_ID"], row.DestGroupId)
				}

				if row.DestIp != expectedRow["DEST_IP"].(string) {
					t.Errorf("Row %d: DestIp mismatch. Expected %s, got %s",
						i, expectedRow["DEST_IP"], row.DestIp)
				}

				if row.DestPort != expectedRow["DEST_PORT"].(string) {
					t.Errorf("Row %d: DestPort mismatch. Expected %s, got %s",
						i, expectedRow["DEST_PORT"], row.DestPort)
				}

				if row.DscpQos != expectedRow["DSCP_QOS"].(string) {
					t.Errorf("Row %d: DscpQos mismatch. Expected %s, got %s",
						i, expectedRow["DSCP_QOS"], row.DscpQos)
				}

				if row.Compression != expectedRow["COMPRESSION"].(string) {
					t.Errorf("Row %d: Compression mismatch. Expected %s, got %s",
						i, expectedRow["COMPRESSION"], row.Compression)
				}

				if row.Encoding != expectedRow["ENCODING"].(string) {
					t.Errorf("Row %d: Encoding mismatch. Expected %s, got %s",
						i, expectedRow["ENCODING"], row.Encoding)
				}

				if row.Transport != expectedRow["TRANSPORT"].(string) {
					t.Errorf("Row %d: Transport mismatch. Expected %s, got %s",
						i, expectedRow["TRANSPORT"], row.Transport)
				}

				if row.DestState != expectedRow["DEST_STATE"].(string) {
					t.Errorf("Row %d: DestState mismatch. Expected %s, got %s",
						i, expectedRow["DEST_STATE"], row.DestState)
				}

				if row.TlsMutual != expectedRow["TLS_MUTUAL"].(string) {
					t.Errorf("Row %d: TlsMutual mismatch. Expected %s, got %s",
						i, expectedRow["TLS_MUTUAL"], row.TlsMutual)
				}

				if row.TotalBytesSent != expectedRow["TOTAL_BYTES_SENT"].(string) {
					t.Errorf("Row %d: TotalBytesSent mismatch. Expected %s, got %s",
						i, expectedRow["TOTAL_BYTES_SENT"], row.TotalBytesSent)
				}

				if row.TotalPacketsSent != expectedRow["TOTAL_PACKETS_SENT"].(string) {
					t.Errorf("Row %d: TotalPacketsSent mismatch. Expected %s, got %s",
						i, expectedRow["TOTAL_PACKETS_SENT"], row.TotalPacketsSent)
				}

				if row.LastSentTime != expectedRow["LAST_SENT_TIME"].(string) {
					t.Errorf("Row %d: LastSentTime mismatch. Expected %s, got %s",
						i, expectedRow["LAST_SENT_TIME"], row.LastSentTime)
				}

				if row.DestEndpoint != expectedRow["DEST_ENDPOINT"].(string) {
					t.Errorf("Row %d: DestEndpoint mismatch. Expected %s, got %s",
						i, expectedRow["DEST_ENDPOINT"], row.DestEndpoint)
				}

				if row.InitialUpdates != expectedRow["INITIAL_UPDATES"].(string) {
					t.Errorf("Row %d: InitialUpdates mismatch. Expected %s, got %s",
						i, expectedRow["INITIAL_UPDATES"], row.InitialUpdates)
				}

				if !compareStringSlices(row.CollectionId, expectedRow["COLLECTION_ID"]) {
					t.Errorf("Row %d: CollectionId mismatch. Expected %v, got %v",
						i, expectedRow["COLLECTION_ID"], row.CollectionId)
				}

				if !compareStringSlices(row.CollectionSampleInterval, expectedRow["COLLECTION_SAMPLE_INTERVAL"]) {
					t.Errorf("Row %d: CollectionSampleInterval mismatch. Expected %v, got %v",
						i, expectedRow["COLLECTION_SAMPLE_INTERVAL"], row.CollectionSampleInterval)
				}

				if !compareStringSlices(row.CollectionHeartbeat, expectedRow["COLLECTION_HEARTBEAT"]) {
					t.Errorf("Row %d: CollectionHeartbeat mismatch. Expected %v, got %v",
						i, expectedRow["COLLECTION_HEARTBEAT"], row.CollectionHeartbeat)
				}

				if !compareStringSlices(row.NumCollection, expectedRow["NUM_COLLECTION"]) {
					t.Errorf("Row %d: NumCollection mismatch. Expected %v, got %v",
						i, expectedRow["NUM_COLLECTION"], row.NumCollection)
				}

				if !compareStringSlices(row.CollectionPath, expectedRow["COLLECTION_PATH"]) {
					t.Errorf("Row %d: CollectionPath mismatch. Expected %v, got %v",
						i, expectedRow["COLLECTION_PATH"], row.CollectionPath)
				}
			}
		})
	}
}

// compareStringSlices compares a []string with an interface{} that should be a slice
func compareStringSlices(actual []string, expected interface{}) bool {
	expectedSlice, ok := expected.([]interface{})
	if !ok {
		return false
	}

	if len(actual) != len(expectedSlice) {
		return false
	}

	expectedStrings := make([]string, len(expectedSlice))
	for i, v := range expectedSlice {
		str, ok := v.(string)
		if !ok {
			return false
		}
		expectedStrings[i] = str
	}

	return reflect.DeepEqual(actual, expectedStrings)
}
