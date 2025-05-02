package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	
	"github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	gribipb "github.com/openconfig/gribi/v1/proto/service"
	binlogpb "google.golang.org/grpc/binarylog/grpc_binarylog_v1"
	"google.golang.org/protobuf/proto"
)


type RawTest struct {
	TestName string          `json:"test_name"`
	Requests []TestCaseEntry `json:"requests"`
}

// TestCaseEntry holds an individual test step
type TestCaseEntry struct {
	Error   any            `json:"err"`
	Request map[string]any `json:"request"`
	RPC     string         `json:"rpc"`
	Status  any            `json:"status"`
	Type    string         `json:"type"`
}

type TestResults map[string][]TestCaseEntry

// ConfigOpEntry aggregates config operations for a policy
type ConfigOpEntry struct {
	PassedTestcase map[string]map[string]any `json:"tc_pass_list"`
	FailedTestcase map[string]map[string]any `json:"tc_fail_list"`
	Result         string                    `json:"result"`
}

// AggregatedConfigOp holds all four config phases
type AggregatedConfigOp struct {
	Upload   ConfigOpEntry `json:"upload"`
	Rotate   ConfigOpEntry `json:"rotate"`
	Finalize ConfigOpEntry `json:"finalize"`
	Probe    ConfigOpEntry `json:"probe"`
}

// VerificationEntry holds RPC verification results
type VerificationEntry struct {
	PassedTestcases map[string]map[string]any `json:"tc_pass_list"`
	FailedTestcases map[string]map[string]any `json:"tc_fail_list"`
	Result          string                    `json:"result"`
}

// PolicyOutput is the final structure per policy
type PolicyOutput struct {
	ConfigOp     AggregatedConfigOp                      `json:"config_operation"`
	HardwareInfo HardwareInfo                            `json:"hardware_info"`
	ConfigMode   map[string]map[string]string            `json:"config_mode"`
	VerifyMode   map[string]map[string]string            `json:"verify_mode"`
	Inband       string                                  `json:"inband,omitempty"`
	IPv4         string                                  `json:"ipv4,omitempty"`
	LogLink      string                                  `json:"log_link"`
	Outband      string                                  `json:"outband"`
	Result       string                                  `json:"result"`
	SimHw        SimHw                                   `json:"sim_hw,omitempty"`
	Users        map[string]string                       `json:"users"`
	Verification map[string]map[string]VerificationEntry `json:"verification"`
}

type OutputJSON struct {
	SubmitterID     string                                 `json:"submitter_id"`
	Testbed         string                                 `json:"testbed"`
	Project         string                                 `json:"project"`
	Templates       map[string]map[string]map[string][]any `json:"templates"`
	SoftwareInfo    map[string]string                      `json:"software_info"`
	SlotInfo        map[string]map[string]string           `json:"slot_info"`
	UnitTest        bool                                   `json:"unit_test"`
	UpdateResultsDB bool                                   `json:"update_results_db"`
}
type ResultWrapper struct {
	Result string `json:"result,omitempty"`
}

type SimHw struct {
	Sim *ResultWrapper `json:"sim,omitempty"`
	Hw  *ResultWrapper `json:"hw,omitempty"`
}

type HardwareInfo struct {
	PlatformFamily string   `json:"platform_family"`
	NPU            []string `json:"npu"`
	PID            []string `json:"pid"`
}

// Response structure
type PostingResult struct {
	Passed  int `json:"pass"`
	Failed  int `json:"fail"`
	Errored int `json:"error"`
	Total   int `json:"total"`
} 

var allUsers = []string{"admin", "deny-all", "gribi-modify", "gnmi-set", "gnoi-ping", "gnoi-time", "gnsi-probe", "read-only"}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <log_file>\n", os.Args[0])
		os.Exit(1)
	}

	firexID := os.Args[1]
	logFileLink := firexID + "/test_logs/grpc_binarylog.txt"
	entries, err := LoadLogFile(logFileLink)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading log file: %v\n", err)
		os.Exit(1)
	}

	jsonData, err := rpcsTestCaseWise(entries)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error processing log entries: %v\n", err)
		os.Exit(1)
	}

	violetJson, err := generateJsonForVioletDB(jsonData, firexID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating JSON for VioletDB: %v\n", err)
		os.Exit(1)
	}
	// Write the JSON to a file
	err = os.WriteFile("violet.json", violetJson, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing JSON to file: %v\n", err)
		os.Exit(1)
	}
	// post to traceability service
	// res := postToTraceability(violetJson)
	// if res != nil {
	// 	log.Printf("Traceability: pass=%d, fail=%d, error=%d, total=%d", res.Passed, res.Failed, res.Errored, res.Total)
	// }

}

// Load binary log file
func LoadLogFile(filePath string) ([]*binlogpb.GrpcLogEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}
	defer file.Close()

	var entries []*binlogpb.GrpcLogEntry
	reader := bufio.NewReader(file)

	for {
		// Read the length of the next message (4 bytes)
		hdr := make([]byte, 4)
		_, err := io.ReadFull(reader, hdr)
		if err == io.EOF {
			break // End of file
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read message length: %v", err)
		}

		msgLen := binary.BigEndian.Uint32(hdr)
		msg := make([]byte, msgLen)
		_, err = io.ReadFull(reader, msg)
		if err != nil {
			continue
		}

		var entry binlogpb.GrpcLogEntry
		if err := proto.Unmarshal(msg, &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message: %v", err)
		}

		entries = append(entries, &entry)
	}

	return entries, nil
}

func createRequestEntry(rpcType, requestType string, callID interface{}, extras map[string]interface{}) map[string]interface{} {
	entry := map[string]interface{}{
		"rpc":    rpcType,
		"type":   requestType,
		"callid": callID,
	}
	for k, v := range extras {
		entry[k] = v
	}
	return entry
}

func unmarshalGribiGetRequest(data []byte) (interface{}, error) {
	// Create an empty instance of GetRequest
	getRequest := &gribipb.GetRequest{}

	// Unmarshal the binary data into the GetRequest struct
	err := proto.Unmarshal(data, getRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %v", err)
	}

	return getRequest, nil
}
func unmarshalGribiModifyRequest(data []byte) (interface{}, error) {
	var req gribipb.ModifyRequest
	err := proto.Unmarshal(data, &req)
	return &req, err
}

func unmarshalGnmiGetRequest(data []byte) (interface{}, error) {
	var req gnmi.GetRequest
	err := proto.Unmarshal(data, &req)
	return &req, err
}

func unmarshalGnmiSetRequest(data []byte) (interface{}, error) {
	var req gnmi.SetRequest
	err := proto.Unmarshal(data, &req)
	return &req, err
}

func unmarshalGnoiTimeRequest(data []byte) (interface{}, error) {
	var req spb.TimeRequest
	err := proto.Unmarshal(data, &req)
	return &req, err
}

func unmarshalGnoiPingRequest(data []byte) (interface{}, error) {
	var req spb.PingRequest
	err := proto.Unmarshal(data, &req)
	return &req, err
}

func unmarshalGnsiAuthzGetRequest(data []byte) (interface{}, error) {
	var req authzpb.GetRequest
	err := proto.Unmarshal(data, &req)
	return &req, err
}
func unmarshalGnsiAuthzProbeRequest(data []byte) (interface{}, error) {
	var req authzpb.ProbeRequest
	err := proto.Unmarshal(data, &req)
	return &req, err
}
func unmarshalGnsiAuthzRotateRequest(data []byte) (interface{}, error) {
	var req authzpb.RotateAuthzRequest
	err := proto.Unmarshal(data, &req)
	return &req, err
}

func unmarshalGnoiRebootRequest(data []byte) (interface{}, error) {
	var req spb.RebootRequest
	err := proto.Unmarshal(data, &req)
	return &req, err
}

var unmarshalFuncs = map[string]func([]byte) (interface{}, error){
	"/gribi.gRIBI/Get":            unmarshalGribiGetRequest,
	"/gribi.gRIBI/Modify":         unmarshalGribiModifyRequest,
	"/gnmi.gNMI/Get":              unmarshalGnmiGetRequest,
	"/gnmi.gNMI/Set":              unmarshalGnmiSetRequest,
	"/gnoi.system.System/Time":    unmarshalGnoiTimeRequest,
	"/gnoi.system.System/Ping":    unmarshalGnoiPingRequest,
	"/gnsi.authz.v1.Authz/Get":    unmarshalGnsiAuthzGetRequest,
	"/gnsi.authz.v1.Authz/Rotate": unmarshalGnsiAuthzRotateRequest,
	"/gnsi.authz.v1.Authz/Probe":  unmarshalGnsiAuthzProbeRequest,
	"/gnoi.system.System/Reboot":  unmarshalGnoiRebootRequest,
}

func handleRequest(testCaseRequests map[string][]map[string]interface{},
	currentTestName string, methodName string, mm []byte, callID uint64) {
	unmarshalFunc, exists := unmarshalFuncs[methodName]
	if !exists {
		return
	}
	_, rpc, method := strings.Split(strings.Split(methodName, "/")[1], ".")[0],
		strings.Split(strings.Split(methodName, "/")[1], ".")[1],
		strings.Split(methodName, "/")[2]

	// Unmarshal using the appropriate function for this method
	request, err := unmarshalFunc(mm)
	if err != nil {
		log.Printf("Error unmarshalling request for %s: %v", methodName, err)
		return
	}

	// Check for RotateRequest with UploadRequest
	if method == "Rotate" {
		var rotateReq authzpb.RotateAuthzRequest
		err = proto.Unmarshal(mm, &rotateReq)
		uploadRequest := rotateReq.GetUploadRequest()
		finalizeRequest := rotateReq.GetFinalizeRotation()
		if uploadRequest != nil {
			testCaseRequests[currentTestName] = append(testCaseRequests[currentTestName],
				createRequestEntry(rpc, "Upload", callID, map[string]interface{}{"request": uploadRequest}),
			)
		}
		if finalizeRequest != nil {
			testCaseRequests[currentTestName] = append(testCaseRequests[currentTestName],
				createRequestEntry(rpc, "Finalize", callID, map[string]interface{}{"request": finalizeRequest}),
			)
		}
	}
	// Store the unmarshalled request in the appropriate place
	testCaseRequests[currentTestName] = append(testCaseRequests[currentTestName],
		createRequestEntry(rpc, method, callID, map[string]interface{}{"request": request}))
}

func sanitizeTestName(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == unicode.ReplacementChar {
			return -1 // drop the character
		}
		return r
	}, name)
}
func rpcsTestCaseWise(entries []*binlogpb.GrpcLogEntry) ([]byte, error){
	var currentTestName string
	testCaseRequestsNew := make(map[string][]map[string]interface{})
	clientHeaderCallID := make(map[uint64]*binlogpb.ClientHeader)
	var orderedTestCaseNames []string
	for _, e := range entries {
		if strings.HasPrefix(e.GetClientHeader().GetMethodName(), "/ondatra/") {
			for _, md := range e.GetClientHeader().GetMetadata().GetEntry() {
				if md.GetKey() == "test_name" {
					currentTestName = string(md.GetValue())
					if _, exists := testCaseRequestsNew[currentTestName]; !exists {
						testCaseRequestsNew[currentTestName] = []map[string]interface{}{}
						orderedTestCaseNames = append(orderedTestCaseNames, currentTestName)
					}
				}
			}
		} else {
			if e.Type == binlogpb.GrpcLogEntry_EVENT_TYPE_CLIENT_HEADER {
				clientHeaderCallID[e.GetCallId()] = e.GetClientHeader()
			}
			if e.Type == binlogpb.GrpcLogEntry_EVENT_TYPE_CLIENT_MESSAGE {
				data := e.GetMessage().GetData()
				methodName := clientHeaderCallID[e.GetCallId()].GetMethodName()
				handleRequest(testCaseRequestsNew, currentTestName, methodName, data, e.GetCallId())

			}

		}
	}
	callStatusMap := make(map[uint64]map[string]interface{})

	for _, e := range entries {
		if e.Type == binlogpb.GrpcLogEntry_EVENT_TYPE_SERVER_TRAILER {
			callID := e.GetCallId()
			callStatusMap[callID] = map[string]interface{}{
				"status": e.GetTrailer().GetStatusCode(),
				"err":    e.GetTrailer().GetStatusMessage(),
			}
		}
	}
	type TestCaseEntry struct {
		TestName string                   `json:"test_name"`
		Requests []map[string]interface{} `json:"requests"`
	}
	var orderedOutput []TestCaseEntry
	formattedMap, err := json.MarshalIndent(callStatusMap, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling callStatusMap:", err)
		return nil, fmt.Errorf("error marshalling: %v", err)
	}
	fmt.Println("callStatusMap:")
	fmt.Println(string(formattedMap))
	for _, testName := range orderedTestCaseNames {
		cleanName := sanitizeTestName(testName)
		requests := testCaseRequestsNew[testName]
		for i, req := range requests {
			if callID, ok := req["callid"].(uint64); ok {
				if statusEntry, exists := callStatusMap[callID]; exists {
					requests[i]["status"] = statusEntry["status"]
					requests[i]["err"] = statusEntry["err"]
				}
			}
		}
		orderedOutput = append(orderedOutput, TestCaseEntry{
			TestName: cleanName,
			Requests: requests,
		})
	}
	data, err := json.MarshalIndent(orderedOutput, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling:", err)
		return nil, fmt.Errorf("error marshalling: %v", err)
	}

	err = os.WriteFile("output.json", data, 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
		return nil, fmt.Errorf("error writing file: %v", err)
	}
	fmt.Println("Data written to output.json")
	return data, nil
}
// initConfigOpEntry initializes an empty ConfigOpEntry
func initConfigOpEntry() ConfigOpEntry {
	return ConfigOpEntry{
		PassedTestcase: make(map[string]map[string]any),
		FailedTestcase: make(map[string]map[string]any),
		Result:         "skip",
	}
}

// finalizeConfigOpEntry sets Result based on entries
func finalizeConfigOpEntry(e *ConfigOpEntry) {
	if len(e.FailedTestcase) > 0 {
		e.Result = "fail"
	} else if len(e.PassedTestcase) > 0 {
		e.Result = "pass"
	} else {
		e.Result = "skip"
	}
}

// initVE initializes an empty VerificationEntry
func initVE() VerificationEntry {
	return VerificationEntry{
		PassedTestcases: make(map[string]map[string]any),
		FailedTestcases: make(map[string]map[string]any),
		Result:          "skip",
	}
}

// finalizeVE sets Result based on verification entries
func finalizeVE(v *VerificationEntry) {
	if len(v.FailedTestcases) > 0 {
		v.Result = "fail"
	} else if len(v.PassedTestcases) > 0 {
		v.Result = "pass"
	} else {
		v.Result = "skip"
	}
}

func ifThenElse(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// extractNPU returns the NPU ID for a given platform family and model.
func extractNPU(model string, hardwareMap map[string]map[string]map[string]any) string {
	if familyMap, ok := hardwareMap["8000"]; ok {
		if modelInfo, ok := familyMap[model]; ok {
			if id, ok := modelInfo["npu"].(string); ok {
				return id
			}
		}
	}
	return "unknown"
}

// parseBindingInfo reads the binding file and returns the hardware model.
func parseBindingInfo(filePath string) (model string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "hardware_model:") {
			model = strings.Trim(l[len("hardware_model:"):], ` "`)
		}
	}
	return strings.TrimPrefix(model, "CISCO-")
}

// usernameFromTestCase extracts the certificate username suffix from a test-case string.
func usernameFromTestCase(tcName string) string {
	if(strings.Contains(tcName, "TestAuthz4")) {
		tcName = tcName[:len(tcName)-1]
	}
	tcName = strings.TrimSuffix(tcName, "/")
	if idx := strings.LastIndex(tcName, "cert_"); idx != -1 {
		return tcName[idx+len("cert_"):]
	}
	return ""
}

// parseTestbedInfo reads testbed_info.txt and returns:
// - OS label
// - major version
// - minor version
// - simulation flag ("true"/"false")
func parseTestbedInfo(filePath string) (label, major, minor, sim string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	var simDetected bool

	for _, line := range strings.Split(string(data), "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "XR version =") {
			parts := strings.SplitN(l, "=", 2)
			version := strings.TrimSpace(strings.SplitN(parts[1], ":", 2)[0])
			label = version
			chunks := strings.Split(version, ".")
			if len(chunks) >= 4 {
				major = strings.Join(chunks[:3], ".")
				minor = chunks[3]
			}
		}
		if strings.Contains(l, "VXR") {
			simDetected = true
		}
	}
	if simDetected {
		sim = "true"
	} else {
		sim = "false"
	}
	return
}

// parseSlotInfo scans for the first occurrence of NAME: "0/RP0" or "0/RP1"
// and returns its key and details.
func parseSlotInfo(filePath string) (string, map[string]string) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading file %s: %v", filePath, err)
		return "", nil
	}

	// Split the file content into lines
	lines := strings.Split(string(raw), "\n")

	slotInfo := make(map[string]string)
	slotKey := ""

	// Search for "0/RP0" or "0/RP1"
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, `NAME: "0/RP0"`) {
			slotKey = "0/RP0/CPU0"
		} else if strings.HasPrefix(line, `NAME: "0/RP1"`) {
			slotKey = "0/RP1/CPU0"
		} else {
			continue
		}

		// Extract descr
		descrStart := strings.Index(line, `DESCR: "`)
		if descrStart != -1 {
			descrEnd := strings.Index(line[descrStart+8:], `"`)
			if descrEnd != -1 {
				slotInfo["descr"] = line[descrStart+8 : descrStart+8+descrEnd]
			}
		}

		// Extract PID
		if i+1 < len(lines) {
			nextLine := strings.TrimSpace(lines[i+1])
			pidStart := strings.Index(nextLine, `PID:`)
			if pidStart != -1 {
				slotInfo["pid"] = strings.TrimSpace(nextLine[pidStart+4:pidStart+10])
			}

			// Extract serial number
			snStart := strings.Index(nextLine, `SN:`)
			if snStart != -1 {
				slotInfo["serial_num"] = strings.TrimSpace(nextLine[snStart+3:])
			}
		}

		// Break after finding the first match
		break
	}

	return slotKey, slotInfo
}

func extractTestDetailsURL(path string) (string, string) {
    // Read the JSON file and extract test_details_url and testbed name
	raw, err := os.ReadFile(path)
    if err != nil {
        log.Printf("Error reading file %s: %v", path, err)
        return "", ""
    }

    var data map[string]any
    if err := json.Unmarshal(raw, &data); err != nil {
        log.Printf("Error unmarshalling JSON: %v", err)
        return "", ""
    }

    // Extract test_details_url and testbed name
    testDetailsURL, _ := data["test_details_url"].(string)
    testbedName, _ := data["testbeds"].([]any)[0].(string)

    return testDetailsURL, testbedName
}

func statusString(raw any) string {
	if raw == nil {
		return "pass"
	}
	switch v := raw.(type) {
	case float64:
		if v == 0 {
			return "pass"
		}
		return "fail"
	case int:
		if v == 0 {
			return "pass"
		}
		return "fail"
	case string:
		if v == "pass" {
			return "pass"
		}
		return "fail"
	default:
		return "fail"
	}
}

func extractAuthZVersion(testName string) string {
	parts := strings.Split(testName, "/")
	if parts[0] == "TestAuthz4" {
		return "4"
	}
	if len(parts) > 1 {
		versionPart := strings.Split(parts[1], ",")[0]
		return strings.TrimPrefix(versionPart, "Authz-")
	}
	return ""
}

func extractPolicyName(request map[string]any) string {
	if policyStr, ok := request["policy"].(string); ok {
		var policy map[string]any
		if err := json.Unmarshal([]byte(policyStr), &policy); err == nil {
			if name, ok := policy["name"].(string); ok {
				if strings.HasPrefix(name, "policy-") {
					return strings.TrimPrefix(name, "policy-")
				}
				return name
			}
		}
	}
	return ""
}

func recordVerification(e TestCaseEntry, po *PolicyOutput, policy string, testCaseName string) {
	if testCaseName == "TestAuthz2/Authz-2.2,_Test_Rollback_When_Connection_Closed/Verification_of_Policy_for_read_only_to_allow_gRIBI_Get_and_to_deny_gNMI_Get_after_closing_stream"{
		policy = "gribi-get"
	}
	st := statusString(e.Status)
	rec := func(rpc, op string, pass bool) {
		ve := po.Verification[rpc][op]
		entry := map[string]any{
			"status":  st,
			"request": e.Request,
			"rpc":     e.RPC,
		}
		if e.Error != nil {
			entry["error"] = e.Error
		}
		if !pass && e.Error != nil && !strings.Contains(e.Error.(string), "unauthorized RPC request") {
			pass = !pass
		}
		if pass {
			ve.PassedTestcases[testCaseName] = entry
		} else {
			ve.FailedTestcases[testCaseName] = entry
		}
		po.Verification[rpc][op] = ve
	}

	switch policy {
	case "everyone-can-gnmi-not-gribi":
		// gNMI Get must pass, gRIBI Get must fail
		if e.RPC == "gNMI" && e.Type == "Get" {
			rec("gnmi", "get", st == "pass")
		}
		if e.RPC == "gRIBI" && e.Type == "Get" {
			rec("gribi", "get", st != "pass")
		}

	case "everyone-can-gribi-not-gnmi":
		// gRIBI Get must pass, gNMI Get must fail
		if e.RPC == "gRIBI" && e.Type == "Get" {
			rec("gribi", "get", st == "pass")
		}
		if e.RPC == "gNMI" && e.Type == "Get" {
			rec("gnmi", "get", st != "pass")
		}

	case "gribi-get":
		// only gRIBI Get should pass
		if e.RPC == "gRIBI" && e.Type == "Get" {
			rec("gribi", "get", st == "pass")
		}

		if e.RPC == "gNMI" && e.Type == "Get" {
			rec("gnmi", "get", st != "pass")
		}

	case "gnmi-get":
		// only gNMI Get should pass
		if e.RPC == "gNMI" && e.Type == "Get" {
			rec("gnmi", "get", st == "pass")
		}

		if e.RPC == "gRIBI" && e.Type == "Get"{
			rec("gribi", "get", st != "pass")
		}

	case "normal-1":
		user := usernameFromTestCase(testCaseName)
		switch {
		case e.RPC == "gRIBI" && e.Type == "Get":
			allowed := user == "gribi_modify" || user == "read_only" || user == "user_admin"
			rec("gribi", "get", allowed == (st == "pass"))
		case e.RPC == "gRIBI" && e.Type == "Modify":
			allowed := user == "gribi_modify" || user == "user_admin"
			rec("gribi", "modify", allowed == (st == "pass"))
		case e.RPC == "gNMI" && e.Type == "Get":
			allowed := user == "gnmi_set" || user == "read_only" || user == "user_admin"
			rec("gnmi", "get", allowed == (st == "pass"))
		case e.RPC == "gNMI" && e.Type == "Set":
			allowed := user == "gnmi_set" || user == "user_admin"
			rec("gnmi", "set", allowed == (st == "pass"))
		case e.RPC == "system" && e.Type == "Time":
			allowed := user == "gnoi_time" || user == "user_admin"
			rec("gnoi", "time", allowed == (st == "pass"))
		case e.RPC == "system" && e.Type == "Ping":
			allowed := user == "gnoi_ping" || user == "user_admin"
			rec("gnoi", "ping", allowed == (st == "pass"))
		case e.RPC == "authz" && e.Type == "Get":
			allowed := user == "read_only" || user == "user_admin"
			rec("gnsi", "get", allowed == (st == "pass"))
		case e.RPC == "system" && e.Type == "Reboot":
			rec("gnoi", "reboot", st == "pass")
		}
	}
}

// generateJsonForVioletDB transforms raw test results into the VioletDB payload.
func generateJsonForVioletDB(jsonData []byte, firexID string) ([]byte, error) {
	// Parse input JSON into slice of RawTest
	var tests []RawTest
	if err := json.Unmarshal(jsonData, &tests); err != nil {
		return nil, fmt.Errorf("failed to parse test results: %w", err)
	}

	// Extract system and hardware info
	osLabel, osMajor, osMinor, simFlag := parseTestbedInfo(firexID + "/testbed_info.txt")
	hwModel := parseBindingInfo(firexID + "/ondatra_binding.txt")
	mapBytes, _ := os.ReadFile("/auto/ops-tool/violet/hw_mapping_v2.json")
	var hwMap map[string]map[string]map[string]any
	_ = json.Unmarshal(mapBytes, &hwMap)

	npuID := extractNPU(hwModel, hwMap)
	slotKey, slotDetails := parseSlotInfo(firexID + "/testbed_info.txt")
	logURL, testBedName := extractTestDetailsURL(firexID + "/.firex_internal_data/repro_info/build_and_regress_abog.json")

	// Prepare container for per-policy outputs
	outputs := make(map[string]*PolicyOutput)
	var (
		currVersion    string
		inConfigPhase  bool
		currPolicyOut  *PolicyOutput
		currPolicyName string
	)

	// Walk through each test suite
	for _, test := range tests {
		if len(test.Requests) == 0 {
			continue
		}
		testName := test.TestName
		version := extractAuthZVersion(testName)
		if version != currVersion {
			currVersion = version
			inConfigPhase = true
			currPolicyOut = nil
			currPolicyName = ""
		}

		for _, entry := range test.Requests {
			switch entry.Type {
			case "Upload":
				if entry.Error != nil {
					continue
				}
				if !inConfigPhase {
					// restart config phase
					inConfigPhase = true
					currPolicyOut = nil
					currPolicyName = ""
				}
				// skip the 'Allow all policy'
				if p, ok := entry.Request["policy"].(string); ok &&
					strings.Contains(p, "Allow all policy") {
					inConfigPhase = false
					continue
				}
				// determine policy key
				key := extractPolicyName(entry.Request)
				if key == "" || key == "invalid-no-allow-rules" {
					continue
				}
				currPolicyName = key

				// lazy-init PolicyOutput
				po, exists := outputs[key]
				if !exists {
					po = &PolicyOutput{
						ConfigOp: AggregatedConfigOp{
							Upload:   initConfigOpEntry(),
							Rotate:   initConfigOpEntry(),
							Finalize: initConfigOpEntry(),
							Probe:    initConfigOpEntry(),
						},
						ConfigMode:   map[string]map[string]string{"non_cli": {"result": "pass"}},
						VerifyMode:   map[string]map[string]string{"non_cli": {"result": "pass"}},
						LogLink:      logURL,
						Outband:      "pass",
						Result:       "pass",
						Users:        make(map[string]string),
						IPv4:         "pass",
						SimHw:        SimHw{Sim: &ResultWrapper{}, Hw: &ResultWrapper{}},
						Verification: defaultVerificationEntries(),
					}
					// populate sim/hw and hardware_info
					po.SimHw.Sim.Result = ifThenElse(simFlag == "true", "pass", "fail")
					po.SimHw.Hw.Result = ifThenElse(simFlag == "true", "fail", "pass")
					po.HardwareInfo = HardwareInfo{
						PlatformFamily: "8000",
						NPU:            []string{npuID},
						PID:            []string{hwModel},
					}
					outputs[key] = po
				}
				currPolicyOut = po
				currPolicyOut.ConfigOp.Upload.PassedTestcase[testName] = entry.Request

			case "Rotate":
				if inConfigPhase && currPolicyOut != nil && entry.Error == nil {
					currPolicyOut.ConfigOp.Rotate.PassedTestcase[testName] = entry.Request
				}

			case "Finalize":
				if inConfigPhase && currPolicyOut != nil {
					currPolicyOut.ConfigOp.Finalize.PassedTestcase[testName] = entry.Request
				}
				inConfigPhase = false

			case "Probe":
				inConfigPhase = false
				if currPolicyOut != nil {
					if u, _ := entry.Request["user"].(string); u == "dummy" {
						continue
					}
					currPolicyOut.ConfigOp.Probe.PassedTestcase[testName] = entry.Request
					if u, _ := entry.Request["user"].(string); u != "" {
						currPolicyOut.Users[sanitizeUsername(u)] = "pass"
					}
				}

			default:
				// verification phase
				if !inConfigPhase && currPolicyOut != nil {
					recordVerification(entry, currPolicyOut, currPolicyName, testName)
				}
			}
		}
	}

	// Finalize all policy outputs
	for _, po := range outputs {
		// clean up verification entries
		for rpc, ops := range po.Verification {
			for op, ve := range ops {
				finalizeVE(&ve)
				if ve.Result == "skip" {
					delete(ops, op)
				} else {
					ops[op] = ve
				}
			}
			if len(ops) == 0 {
				delete(po.Verification, rpc)
			}
		}
		// finalize config phases
		finalizeConfigOpEntry(&po.ConfigOp.Upload)
		finalizeConfigOpEntry(&po.ConfigOp.Rotate)
		finalizeConfigOpEntry(&po.ConfigOp.Finalize)
		finalizeConfigOpEntry(&po.ConfigOp.Probe)
		// drop empty phases
		cleanupConfigPhase(&po.ConfigOp.Upload)
		cleanupConfigPhase(&po.ConfigOp.Rotate)
		cleanupConfigPhase(&po.ConfigOp.Finalize)
		cleanupConfigPhase(&po.ConfigOp.Probe)
		
		// remove failed sim/hw entries
		if po.SimHw.Sim.Result == "fail" {
			po.SimHw.Sim = nil
		}
		if po.SimHw.Hw.Result == "fail" {
			po.SimHw.Hw = nil
		}
		// drop skipped users
		for u, st := range po.Users {
			if st == "skip" {
				delete(po.Users, u)
			}
		}
		// determine overall result
		po.Result = "pass"
	outer:
		for _, ops := range po.Verification {
			for _, ve := range ops {
				if len(ve.FailedTestcases) > 0 {
					po.Result = "fail"
					break outer
				}
			}
		}
	}

	// Assemble final output JSON
	finalOut := OutputJSON{
		SubmitterID: "kjahed",
		Testbed:     testBedName,
		Project:     "Manual",
		Templates:   make(map[string]map[string]map[string][]any),
		SoftwareInfo: map[string]string{
			"device_os":        "XR",
			"os_version_major": osMajor,
			"os_version_minor": osMinor,
			"os_label":         osLabel,
			"os_type":          "XR7",
		},
		SlotInfo: map[string]map[string]string{
			slotKey: slotDetails, // Dynamically set the slot key and details
		},
		UnitTest:        false,
		UpdateResultsDB: true,
	}
	for name, po := range outputs {
		if _, ok := finalOut.Templates[name]; !ok {
			finalOut.Templates[name] = map[string]map[string][]any{"capabilities": {"AUTHZ": {}}}
		}
		finalOut.Templates[name]["capabilities"]["AUTHZ"] = append(
			finalOut.Templates[name]["capabilities"]["AUTHZ"], po,
		)
	}

	payload, err := json.MarshalIndent(finalOut, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal output: %w", err)
	}
	return payload, nil
}

// defaultVerificationEntries returns the initial verification structure.
func defaultVerificationEntries() map[string]map[string]VerificationEntry {
	return map[string]map[string]VerificationEntry{
		"gribi": {"get": initVE(), "modify": initVE()},
		"gnmi":  {"get": initVE(), "set": initVE()},
		"gnoi":  {"time": initVE(), "ping": initVE(), "reboot": initVE()},
		"gnsi":  {"get": initVE()},
	}
}

// cleanupConfigPhase clears a config entry if it was skipped.
func cleanupConfigPhase(entry *ConfigOpEntry) {
	if entry.Result == "skip" {
		*entry = ConfigOpEntry{}
	}
}

// sanitizeUsername extracts the base name and normalizes it.
func sanitizeUsername(pathStr string) string {
	base := filepath.Base(pathStr)
	return strings.ReplaceAll(base, "-", "_")
}

// postToTraceability sends raw JSON to local endpoint
func postToTraceability(data []byte) *PostingResult {
	url := "http://violet-prod-lnx:8000/traceabilityv2"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		log.Printf("request create error: %v", err)
		return nil
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("request error: %v", err)
		return nil
	}
	defer resp.Body.Close()

	var result PostingResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("decode error: %v", err)
		return nil
	}
	return &result
}
