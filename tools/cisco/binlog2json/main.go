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

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <log_file>\n", os.Args[0])
		os.Exit(1)
	}

	logFile := os.Args[1]
	entries, err := LoadLogFile(logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading log file: %v\n", err)
		os.Exit(1)
	}

	rpcsTestCaseWise(entries)

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
func rpcsTestCaseWise(entries []*binlogpb.GrpcLogEntry) {
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
