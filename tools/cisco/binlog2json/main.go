package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"unicode"

	"github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	gribipb "github.com/openconfig/gribi/v1/proto/service"
	binlogpb "google.golang.org/grpc/binarylog/grpc_binarylog_v1"
	"google.golang.org/protobuf/proto"
)

// TestCaseEntry holds an individual test step
type TestCaseEntry struct {
	Error   any            `json:"error"`
	Request map[string]any `json:"request"`
	RPC     string         `json:"rpc"`
	Status  string         `json:"status"`
	Type    string         `json:"type"`
}

type TestResults map[string][]TestCaseEntry

// ConfigOpEntry aggregates config operations for a policy
type ConfigOpEntry struct {
	PassedTestcase map[string]map[string]any `json:"passed_testcase"`
	FailedTestcase map[string]map[string]any `json:"failed_testcase"`
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
	PassedTestcases map[string]map[string]any `json:"passed_testcase"`
	FailedTestcases map[string]map[string]any `json:"failed_testcase"`
	Result          string                    `json:"result"`
}

// PolicyOutput is the final structure per policy
type PolicyOutput struct {
	ConfigOp     AggregatedConfigOp                      `json:"config_operation"`
	HardwareInfo HardwareInfo                            `json:"hardware_info"`
	ConfigMode   map[string]map[string]string            `json:"config_mode"`
	VerifyMode   map[string]map[string]string            `json:"verify_mode"`
	Inband       string                                  `json:"inband"`
	IPv4         string                                  `json:"ipv4"`
	IPv6         string                                  `json:"ipv6"`
	LogLink      string                                  `json:"log_link"`
	Outband      string                                  `json:"outband"`
	Result       string                                  `json:"result"`
	SimHw        SimHw                                   `json:"sim_hw"`
	Users        map[string]string                       `json:"users"`
	Verification map[string]map[string]VerificationEntry `json:"verification"`
	VRF          string                                  `json:"vrf"`
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

type SimHw struct {
	Sim struct {
		Result string `json:"result"`
	} `json:"sim"`
	Hw struct {
		Result string `json:"result"`
	} `json:"hw"`
}

type HardwareInfo struct {
	PlatformFamily string   `json:"platform_family"`
	NPU            []string `json:"npu"`
	PID            []string `json:"pid"`
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

	_, err = generateJsonForVioletDB(jsonData, firexID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating JSON for VioletDB: %v\n", err)
		os.Exit(1)
	}
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
func rpcsTestCaseWise(entries []*binlogpb.GrpcLogEntry) ([]byte, error) {
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
		return
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
		return
	}

	err = os.WriteFile("output.json", data, 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
		return
	}

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

// Get NPU from hardware_model and mapping
func getNPU(hwModel string, hwMapping map[string]map[string]map[string]any) string {
	if npuVal, ok := hwMapping["8000"][hwModel]["npu"]; ok {
		if npuStr, ok := npuVal.(string); ok {
			return npuStr
		}
	}
	return "unknown"
}

// Parse ondatra_binding.txt -> returns hardware model and whether IP is IPv6
func getHardwareModelAndIP(path string) (string, bool) {
	file, err := os.Open(path)

	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		return "", false
	}
	defer file.Close()

	// Read the file content
	raw, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Failed to read log file: %v", err)
		return "", false
	}
	lines := strings.Split(string(raw), "\n")

	hwModel := ""
	hasIPv6 := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "hardware_model:") {
			hwModel = strings.Trim(strings.TrimPrefix(line, "hardware_model:"), `" `)
		}
		if strings.Contains(line, "gnmi:") {
			if strings.Contains(line, "[") { // crude ipv6 check
				hasIPv6 = true
			}
		}
	}
	hwModel = strings.TrimPrefix(hwModel, "CISCO-")
	return hwModel, hasIPv6
}

func ifThenElse(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// Parse testbed_info.txt -> versioning, vrf and sim/hw detection
func getSoftwareInfo(path string) (string, string, string, string, string) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("Failed to open log file: %v", err)
		return "", "", "", "", ""
	}
	defer file.Close()

	// Read the file content
	raw, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Failed to read log file: %v", err)
		return "", "", "", "", ""
	}

	// Split the content into lines
	lines := strings.Split(string(raw), "\n")

	version := ""
	vrf := "pass"
	sim := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "XR version =") {
			version = strings.TrimSpace(strings.Split(line, "=")[1])
			if strings.Contains(version, ":") {
				version = strings.TrimSpace(strings.Split(version, ":")[0])
			}
		}
		if strings.Contains(line, "ssh server vrf default") {
			vrf = "fail"
		}
		if strings.Contains(line, "VXR") {
			sim = true
		}
	}
	osLabel := version
	parts := strings.Split(version, ".")
	osMajor := strings.Join(parts[:3], ".")
	osMinor := parts[3]

	return osLabel, osMajor, osMinor, vrf, ifThenElse(sim, "true", "false")
}

func generateJsonForVioletDB(jsonData []byte, firexID string) ([]byte, error) {
	var input TestResults
	json.Unmarshal(jsonData, &input)

	if err := json.Unmarshal(jsonData, &input); err != nil {
		panic(err)
	}

	osLabel, osVersionMajor, osVersionMinor, vrfStatus, sim := getSoftwareInfo(firexID + "/testbed_info.txt")

	hardwareModel, hasIPv6 := getHardwareModelAndIP(firexID + "/ondatra_binding.txt") // returns e.g., "8808"
	hwMappingData, _ := os.ReadFile("/auto/ops-tool/violet/hw_mapping_v2.json")
	var hwMap map[string]map[string]map[string]any
	_ = json.Unmarshal(hwMappingData, &hwMap)

	npu := getNPU(hardwareModel, hwMap)

	// Map AuthZ1.4 suffix -> user under test
	testUser := map[string]string{
		"Validating_access_for_user_cert_deny_all":     "deny-all",
		"Validating_access_for_user_cert_gnmi_set":     "gnmi-set",
		"Validating_access_for_user_cert_gnoi_ping":    "gnoi-ping",
		"Validating_access_for_user_cert_gnoi_time":    "gnoi-time",
		"Validating_access_for_user_cert_gnsi_probe":   "gnsi-probe",
		"Validating_access_for_user_cert_gribi_modify": "gribi-modify",
		"Validating_access_for_user_cert_read_only":    "read-only",
		"Validating_access_for_user_cert_user_admin":   "admin",
	}

	// Map each testcase to its policy
	policyMap := map[string]string{
		// AuthZ1.1,1.2,1.3 base entries
		"TestAuthz1/Authz-1.1,_-_Test_empty_source/Verification_of_Policy_for_cert_user_admin_is_allowed_gNMI_Get_and_denied_gRIBI_Get\ufffd\u0001": "everyone-can-gnmi-not-gribi",
		"TestAuthz1/Authz-1.1,_-_Test_empty_sourceR": "everyone-can-gnmi-not-gribi",
		"TestAuthz1/Authz-1.2,_Test_Empty_Request/Verification_of_cert_deny_all_is_denied_to_issue_gRIBI.Get_and_cert_user_admin_is_allowed_to_issue_`gRIBI.Get`\ufffd\u0002": "everyone-can-gribi-not-gnmi",
		"TestAuthz1/Authz-1.2,_Test_Empty_RequestP": "everyone-can-gribi-not-gnmi",
		"TestAuthz1/Authz-1.3,_Test_that_there_can_only_be_One_policy/Verification_of_Policy_for_read-only_to_deny_gRIBI_Get_and_allow_gNMI_Get\ufffd\u0002": "gribi-get",
		"TestAuthz1/Authz-1.3,_Test_that_there_can_only_be_One_policyx":                                                                                      "gribi-get",
		// AuthZ1.4 config
		"TestAuthz1/Authz-1.4,_Test_Normal_PolicyP":                                                         "normal-1",
		"TestAuthz1/Authz-1.4,_Test_Normal_Policy/Validating_access_for_user_cert_deny_all\ufffd\u0001":     "normal-1",
		"TestAuthz1/Authz-1.4,_Test_Normal_Policy/Validating_access_for_user_cert_gnmi_set\ufffd\u0001":     "normal-1",
		"TestAuthz1/Authz-1.4,_Test_Normal_Policy/Validating_access_for_user_cert_gnoi_ping\ufffd\u0001":    "normal-1",
		"TestAuthz1/Authz-1.4,_Test_Normal_Policy/Validating_access_for_user_cert_gnoi_time\ufffd\u0001":    "normal-1",
		"TestAuthz1/Authz-1.4,_Test_Normal_Policy/Validating_access_for_user_cert_gnsi_probe\ufffd\u0001":   "normal-1",
		"TestAuthz1/Authz-1.4,_Test_Normal_Policy/Validating_access_for_user_cert_gribi_modify\ufffd\u0001": "normal-1",
		"TestAuthz1/Authz-1.4,_Test_Normal_Policy/Validating_access_for_user_cert_read_only\ufffd\u0001":    "normal-1",
		"TestAuthz1/Authz-1.4,_Test_Normal_Policy/Validating_access_for_user_cert_user_admin\ufffd\u0001":   "normal-1",
		// AuthZ2: verification-only probes
		"TestAuthz2/Authz-2.1,_Test_only_one_rotation_request_at_a_time/Verification_of_Policy_for_user_admin_to_deny_gRIBI_Get_and_allow_gNMI_Get\ufffd\u0002":                                                      "everyone-can-gnmi-not-gribi",
		"TestAuthz2/Authz-2.1,_Test_only_one_rotation_request_at_a_time|":                                                                                                                                            "everyone-can-gnmi-not-gribi",
		"TestAuthz2/Authz-2.2,_Test_Rollback_When_Connection_Closed/Verification_of_Policy_for_read_only_to_allow_gRIBI_Get_and_to_deny_gNMI_Get_after_closing_stream\ufffd\u0002":                                   "gribi-get",
		"TestAuthz2/Authz-2.2,_Test_Rollback_When_Connection_Closed/Verification_of_Policy_for_read_only_to_allow_gRIBI_Get_and_to_deny_gNMI_Get_after_rotate_that_is_not_finalized\ufffd\u0002":                     "gnmi-get",
		"TestAuthz2/Authz-2.2,_Test_Rollback_When_Connection_Closed/Verification_of_Policy_for_read_only_to_allow_gRIBI_Get_and_to_deny_gNMI_Get\ufffd\u0002":                                                        "gribi-get",
		"TestAuthz2/Authz-2.2,_Test_Rollback_When_Connection_Closedt":                                                                                                                                                "gribi-get",
		"TestAuthz2/Authz-2.3,_Test_Rollback_on_Invalid_Policy/Applying_policy-invalid-no-allow-rules/Verification_of_Policy_for_read_only_to_allow_gRIBI_Get_and_to_deny_gNMI_Get_after_closing_stream\ufffd\u0002": "gribi-get",
		// (skip the “ignore” ones by not listing them)
		"TestAuthz2/Authz-2.3,_Test_Rollback_on_Invalid_Policy/Verification_of_Policy_for_read_only_to_allow_gRIBI_Get_and_to_deny_gNMI_Get\ufffd\u0002": "gribi-get",

		// AuthZ4: just probes under 'normal-1', same rules as before for each user
		"TestAuthz4/Validating_access_for_user_cert_deny_allf":     "normal-1",
		"TestAuthz4/Validating_access_for_user_cert_gnmi_setf":     "normal-1",
		"TestAuthz4/Validating_access_for_user_cert_gnoi_pingh":    "normal-1",
		"TestAuthz4/Validating_access_for_user_cert_gnoi_timeh":    "normal-1",
		"TestAuthz4/Validating_access_for_user_cert_gnsi_probej":   "normal-1",
		"TestAuthz4/Validating_access_for_user_cert_gribi_modifyn": "normal-1",
		"TestAuthz4/Validating_access_for_user_cert_read_onlyh":    "normal-1",
		"TestAuthz4/Validating_access_for_user_cert_user_adminj":   "normal-1",
		"TestAuthz4\u0014": "normal-1",
	}

	// Prepare outputs per policy
	output := make(map[string]*PolicyOutput)
	for tc, entries := range input {
		policy, ok := policyMap[tc]
		if !ok {
			continue
		}

		// init policy block once
		if _, exists := output[policy]; !exists {
			output[policy] = &PolicyOutput{
				ConfigOp: AggregatedConfigOp{
					Upload:   initConfigOpEntry(),
					Rotate:   initConfigOpEntry(),
					Finalize: initConfigOpEntry(),
					Probe:    initConfigOpEntry(),
				},
				ConfigMode: map[string]map[string]string{"cli": {"result": "pass"}, "non_cli": {"result": "pass"}},
				VerifyMode: map[string]map[string]string{"cli": {"result": "pass"}, "non_cli": {"result": "pass"}},
				Inband:     "pass",
				LogLink:    "http://example.com/log",
				Outband:    "pass",
				Result:     "pass",
				Users:      make(map[string]string),
				IPv4:       ifThenElse(hasIPv6, "fail", "pass"),
				IPv6:       ifThenElse(hasIPv6, "pass", "fail"),
				VRF:        vrfStatus,
				Verification: map[string]map[string]VerificationEntry{
					"gribi": {"get": initVE(), "modify": initVE()},
					"gnmi":  {"get": initVE(), "set": initVE(), "subscribe": initVE()},
					"gnoi":  {"time": initVE(), "ping": initVE(), "reboot": initVE()},
					"gnsi":  {"get": initVE()},
				},
			}
			output[policy].SimHw.Sim.Result = ifThenElse(sim == "true", "pass", "fail")
			output[policy].SimHw.Hw.Result = ifThenElse(sim == "true", "fail", "pass")

			// hardware_info block
			output[policy].HardwareInfo = HardwareInfo{
				PlatformFamily: "8000",
				NPU:            []string{npu},
				PID:            []string{hardwareModel},
			}
		}
		agg := output[policy]

		// extract suffix for AuthZ1.4 tests
	entryLoop:
		for _, e := range entries {
			suffix := tc[strings.LastIndex(tc, "/")+1:]
			// config-only processing
			if e.Type == "Upload" {
				if polStr, ok := e.Request["policy"].(string); ok {
					if strings.Contains(polStr, "Allow all policy") {
						// drop out of this testcase immediately
						break entryLoop
					}
				}
			}
			switch tc {
			case "TestAuthz1/Authz-1.3,_Test_that_there_can_only_be_One_policy/Verification_of_Policy_for_read-only_to_deny_gRIBI_Get_and_allow_gNMI_Get\ufffd\u0002":
			case "TestAuthz1/Authz-1.1,_-_Test_empty_sourceR",
				"TestAuthz1/Authz-1.2,_Test_Empty_RequestP",
				"TestAuthz1/Authz-1.3,_Test_that_there_can_only_be_One_policyx",
				"TestAuthz1/Authz-1.4,_Test_Normal_PolicyP",
				"TestAuthz2/Authz-2.1,_Test_only_one_rotation_request_at_a_time|",
				"TestAuthz2/Authz-2.2,_Test_Rollback_When_Connection_Closedt",
				"TestAuthz4\u0014":
				if e.Type == "Upload" {
					agg.ConfigOp.Upload.PassedTestcase[tc] = e.Request
				} else if e.Type == "Rotate" {
					if rr, ok := e.Request["RotateRequest"].(map[string]any); ok {
						if ur, ok2 := rr["UploadRequest"]; ok2 {
							agg.ConfigOp.Rotate.PassedTestcase[tc] = map[string]any{"RotateRequest": map[string]any{"UploadRequest": ur}}
						}
					}
				} else if e.Type == "Finalize" {
					agg.ConfigOp.Finalize.PassedTestcase[tc] = e.Request
				}
				continue
			}

			// verification-only entries
			if e.Type == "Probe" {
				// record probe and user
				if e.Request["user"].(string) != "dummy" {
					agg.ConfigOp.Probe.PassedTestcase[tc] = e.Request
					if u, ok := e.Request["user"].(string); ok {
						user := u[strings.LastIndex(u, "/")+1:]
						agg.Users[user] = "pass"
					}
				}
				continue
			}

			// determine user for AuthZ1.4
			user := ""
			if policy == "normal-1" {

				if strings.HasPrefix(tc, "TestAuthz4") {
					// AuthZ-4: names end with a single letter (f, h, j). Drop it.
					if len(suffix) > 0 {
						suffix = suffix[:len(suffix)-1]
					}
				} else {
					// AuthZ-1.4: names end with the two-byte marker '\ufffd\u0001'. Drop it.
					suffix = strings.TrimSuffix(suffix, "\ufffd\u0001")
				}
				user = testUser[suffix]
			}

			// helper to record verification
			record := func(rpc, op string, pass bool) {
				ve := agg.Verification[rpc][op]
				ent := make(map[string]any)
				// remove request for gnoi time
				if rpc == "gnoi" && op == "time" || rpc == "gnsi" && op == "get" {
					ent["status"] = e.Status
					ent["rpc"] = e.RPC
				} else {
					ent["status"] = e.Status
					ent["request"] = e.Request
					if rpc == "gnoi" && op == "ping" || rpc == "gribi" && op == "modify" {
						if e.Status == "" {
							ent["status"] = "pass"
						}
						ent["rpc"] = e.RPC
					}
					if rpc == "gnsi" {
						ent["rpc"] = e.RPC
					}
				}
				if pass {
					ve.PassedTestcases[tc] = ent
				} else {
					ve.FailedTestcases[tc] = ent
				}
				agg.Verification[rpc][op] = ve
			}

			// now apply explicit rules per RPC
			switch {
			case tc == "TestAuthz1/Authz-1.1,_-_Test_empty_source/Verification_of_Policy_for_cert_user_admin_is_allowed_gNMI_Get_and_denied_gRIBI_Get\ufffd\u0001" && e.RPC == "gRIBI" && e.Type == "Get":
				record("gribi", "get", e.Status != "pass") // fail status => correct denial
			case tc == "TestAuthz1/Authz-1.1,_-_Test_empty_source/Verification_of_Policy_for_cert_user_admin_is_allowed_gNMI_Get_and_denied_gRIBI_Get\ufffd\u0001" && e.RPC == "gNMI" && e.Type == "Get":
				record("gnmi", "get", e.Status == "pass")

			case tc == "TestAuthz1/Authz-1.2,_Test_Empty_Request/Verification_of_cert_deny_all_is_denied_to_issue_gRIBI.Get_and_cert_user_admin_is_allowed_to_issue_`gRIBI.Get`\ufffd\u0002" && e.RPC == "gNMI" && e.Type == "Get":
				record("gnmi", "get", e.Status != "pass")
			case tc == "TestAuthz1/Authz-1.2,_Test_Empty_Request/Verification_of_cert_deny_all_is_denied_to_issue_gRIBI.Get_and_cert_user_admin_is_allowed_to_issue_`gRIBI.Get`\ufffd\u0002" && e.RPC == "gRIBI" && e.Type == "Get":
				record("gribi", "get", e.Status == "pass")

			case tc == "TestAuthz2/Authz-2.1,_Test_only_one_rotation_request_at_a_time/Verification_of_Policy_for_user_admin_to_deny_gRIBI_Get_and_allow_gNMI_Get\ufffd\u0002":
				if e.RPC == "gRIBI" && e.Type == "Get" {
					record("gribi", "get", e.Status != "pass")
				}
				if e.RPC == "gNMI" && e.Type == "Get" {
					record("gnmi", "get", e.Status == "pass")
				}
				continue entryLoop

			// 2) AuthZ2.2a: rollback on close (read-only, gribi-get)
			case tc == "TestAuthz2/Authz-2.2,_Test_Rollback_When_Connection_Closed/Verification_of_Policy_for_read_only_to_allow_gRIBI_Get_and_to_deny_gNMI_Get_after_closing_stream\ufffd\u0002":
				if e.RPC == "gRIBI" && e.Type == "Get" {
					record("gribi", "get", e.Status == "pass")
				}
				if e.RPC == "gNMI" && e.Type == "Get" {
					record("gnmi", "get", e.Status != "pass")
				}
				continue entryLoop

			// 3) AuthZ2.2b: rollback on rotate (read-only, gnmi-get)
			case tc == "TestAuthz2/Authz-2.2,_Test_Rollback_When_Connection_Closed/Verification_of_Policy_for_read_only_to_allow_gRIBI_Get_and_to_deny_gNMI_Get_after_rotate_that_is_not_finalized\ufffd\u0002":
				if e.RPC == "gRIBI" && e.Type == "Get" {
					record("gribi", "get", e.Status != "pass")
				}
				if e.RPC == "gNMI" && e.Type == "Get" {
					record("gnmi", "get", e.Status == "pass")
				}
				continue entryLoop

			// 4) AuthZ2.2c: final rollback (identical to 2.2a)
			case tc == "TestAuthz2/Authz-2.2,_Test_Rollback_When_Connection_Closed/Verification_of_Policy_for_read_only_to_allow_gRIBI_Get_and_to_deny_gNMI_Get\ufffd\u0002":
				if e.RPC == "gRIBI" && e.Type == "Get" {
					record("gribi", "get", e.Status == "pass")
				}
				if e.RPC == "gNMI" && e.Type == "Get" {
					record("gnmi", "get", e.Status != "pass")
				}
				continue entryLoop

			// 5) AuthZ2.3a: invalid policy rollback (after close)
			case tc == "TestAuthz2/Authz-2.3,_Test_Rollback_on_Invalid_Policy/Applying_policy-invalid-no-allow-rules/Verification_of_Policy_for_read_only_to_allow_gRIBI_Get_and_to_deny_gNMI_Get_after_closing_stream\ufffd\u0002":
				if e.RPC == "gRIBI" && e.Type == "Get" {
					record("gribi", "get", e.Status == "pass")
				}
				if e.RPC == "gNMI" && e.Type == "Get" {
					record("gnmi", "get", e.Status != "pass")
				}
				continue entryLoop

			// 6) AuthZ2.3b: ignore “before closing stream” case by omitting from policyMap

			// 7) AuthZ2.3c: invalid policy final (same as 5a)
			case tc == "TestAuthz2/Authz-2.3,_Test_Rollback_on_Invalid_Policy/Verification_of_Policy_for_read_only_to_allow_gRIBI_Get_and_to_deny_gNMI_Get\ufffd\u0002":
				if e.RPC == "gRIBI" && e.Type == "Get" {
					record("gribi", "get", e.Status == "pass")
				}
				if e.RPC == "gNMI" && e.Type == "Get" {
					record("gnmi", "get", e.Status != "pass")
				}
				continue entryLoop

			case policy == "normal-1" && e.RPC == "gRIBI" && e.Type == "Get":
				if user == "gribi-modify" || user == "read-only" || user == "admin" {
					record("gribi", "get", e.Status == "pass")
				} else {
					record("gribi", "get", e.Status != "pass")
				}
			case policy == "normal-1" && e.RPC == "gRIBI" && e.Type == "Modify":
				if user == "gribi-modify" || user == "admin" {
					record("gribi", "modify", e.Status == "")
				} else {
					record("gribi", "modify", e.Status != "pass")
				}

			case policy == "normal-1" && e.RPC == "gNMI" && e.Type == "Get":
				if user == "gnmi-set" || user == "read-only" || user == "admin" {
					record("gnmi", "get", e.Status == "pass")
				} else {
					record("gnmi", "get", e.Status != "pass")
				}
			case policy == "normal-1" && e.RPC == "gNMI" && e.Type == "Set":
				if user == "gnmi-set" || user == "admin" {
					record("gnmi", "set", e.Status == "pass")
				} else {
					record("gnmi", "set", e.Status != "pass")
				}

			case policy == "normal-1" && e.RPC == "system" && e.Type == "Time":
				if user == "gnoi-time" || user == "admin" {
					record("gnoi", "time", e.Status == "pass")
				} else {
					record("gnoi", "time", e.Status != "pass")
				}
			case policy == "normal-1" && e.RPC == "system" && e.Type == "Ping":
				if user == "gnoi-ping" || user == "admin" {
					if e.Status == "" {
						record("gnoi", "ping", true)
					} else {
						record("gnoi", "ping", false)
					}
				} else {
					record("gnoi", "ping", e.Status != "pass")
				}

			case policy == "normal-1" && e.RPC == "authz" && e.Type == "Get":
				if user == "read-only" || user == "admin" {
					record("gnsi", "get", e.Status == "pass")
				} else {
					record("gnsi", "get", e.Status != "pass")
				}
			}
		}

		// finalize verification and config
		for _, k := range []string{"gribi", "gnmi", "gnoi", "gnsi"} {
			v := agg.Verification[k]
			for key, ve := range v {
				finalizeVE(&ve)
				v[key] = ve
			}
			agg.Verification[k] = v
		}
		finalizeConfigOpEntry(&agg.ConfigOp.Upload)
		finalizeConfigOpEntry(&agg.ConfigOp.Rotate)
		finalizeConfigOpEntry(&agg.ConfigOp.Finalize)
		finalizeConfigOpEntry(&agg.ConfigOp.Probe)

		// Normalize user names and preserve statuses
		userRes := make(map[string]string)
		for user, status := range agg.Users {
			// Replace hyphens with underscores in the user name
			normalizedUser := strings.ReplaceAll(user, "-", "_")
			userRes[normalizedUser] = status // Preserve the original status
		}

		// Ensure all users from `allUsers` are included in the output
		for _, u := range allUsers {
			normalizedUser := strings.ReplaceAll(u, "-", "_")
			if _, exists := userRes[normalizedUser]; !exists {
				// Default to "skip" if the user is not already in `userRes`
				userRes[normalizedUser] = "skip"
			}
		}
		agg.Users = userRes
	}

	// Final JSON
	final := OutputJSON{
		SubmitterID: "sahilsi3",
		Testbed:     "PP_SFSIM_Fixed_virtual_topo",
		Project:     "Manual",
		Templates:   make(map[string]map[string]map[string][]any),
		SoftwareInfo: map[string]string{
			"device_os":        "XR",
			"os_version_major": osVersionMajor,
			"os_version_minor": osVersionMinor,
			"os_label":         osLabel,
			"os_type":          "XR7",
		},
		SlotInfo: map[string]map[string]string{
			"0/RP0/CPU0": {
				"pid":        "8201-SYS",
				"serial_num": "PCBEVTQ6JDD",
				"descr":      "Cisco 8201 1RU Chassis",
			},
		},
		UnitTest:        false,
		UpdateResultsDB: true,
	}

	for policy, agg := range output {
		if _, exists := final.Templates[policy]; !exists {
			final.Templates[policy] = make(map[string]map[string][]any)
		}
		if _, exists := final.Templates[policy]["capabilities"]; !exists {
			final.Templates[policy]["capabilities"] = make(map[string][]any)
		}
		final.Templates[policy]["capabilities"]["AUTHZ"] = append(final.Templates[policy]["capabilities"]["AUTHZ"], agg)
	}

	data, _ := json.MarshalIndent(final, "", "  ")
	err := os.WriteFile("violetdb.json", data, 0644)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return nil, fmt.Errorf("error writing to file: %v", err)
	}
	fmt.Println("JSON data successfully written to violetdb.json")
	return data, nil
}
