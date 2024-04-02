// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package acctz_test performs functional tests for acctz service
package acctz_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/authz"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"

	"github.com/openconfig/gnoi/system"
	gnps "github.com/openconfig/gnoi/system"
	acctz "github.com/openconfig/gnsi/acctz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	DefaultCommandService = &acctz.CommandService{
		ServiceType: acctz.CommandService_CMD_SERVICE_TYPE_CLI,
		Cmd:         "run cat /etc/build-info.txt",
	}

	DefaultAuthentication = &acctz.AuthDetail{
		Identity:       "cafyauto",
		PrivilegeLevel: 2,
		Status:         acctz.AuthDetail_AUTHEN_STATUS_PERMIT,
		DenyCause:      "None",
	}

	DefaultSession = &acctz.SessionInfo{
		LocalAddress:  "10.85.84.159",
		LocalPort:     35000,
		RemoteAddress: "2807:f8b0:f800:c00::8",
		RemotePort:    38434,
		IpProto:       4,
		ChannelId:     "0",
	}

	DefaultGrpcService = &acctz.GrpcService{
		ServiceType: acctz.GrpcService_GRPC_SERVICE_TYPE_GNSI,
		RpcName:     "/gnsi.acctz.v1.Acctz/RecordSubscribe",
		// Add other fields as needed
	}
)

func TestPreAcctzGRPC(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	tartgetIP, tartgetPort, err := getIpAndPortFromBindingFile(true)
	if err != nil {
		t.Fatalf("Error Fetching Binding File")
	}
	DefaultSession.LocalAddress = tartgetIP
	DefaultSession.LocalPort = uint32(tartgetPort)

	t.Run("Test Accouting P4RT Events", func(t *testing.T) {
		responses := subcribe(t, dut)
		tartgetIP, tartgetPort, err := getIpAndPortFromBindingFile(false)
		if err != nil {
			t.Fatalf("Error in reading target IP and Port from Binding file: %v", err)
		}
		// Construct the Expected gNOI Event
		grpcService := &acctz.GrpcService{
			ServiceType: acctz.GrpcService_GRPC_SERVICE_TYPE_P4RT,
			RpcName:     "/p4.v1.P4Runtime/StreamChannel",
			// Add other fields as needed
		}
		ExpSession := &acctz.SessionInfo{
			LocalAddress:  tartgetIP,
			LocalPort:     uint32(tartgetPort),
			RemoteAddress: "2807:f8b0:f800:c00::8",
			RemotePort:    38434,
			IpProto:       4,
			ChannelId:     "0",
		}
		expected := ExpectedResponse{
			GrpcService: grpcService,
			SessionInfo: ExpSession,
			Authen:      DefaultAuthentication,
			Timestamp: &timestamppb.Timestamp{
				Seconds: 1707356705,
			},
		}
		VerifyResponses(t, responses, expected)
	})

	t.Run("Test Accouting gNOI Events", func(t *testing.T) {
		pName := "bgp"
		ctx := context.Background()
		proc := findProcessByName(ctx, t, dut, pName)
		pid := uint32(proc.GetPid())
		var wg1 sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg1.Add(1)
			go func() {
				defer wg1.Done()
				killResponse, err := dut.RawAPIs().GNOI(t).System().KillProcess(context.Background(), &gnps.KillProcessRequest{Name: pName, Pid: pid, Restart: true, Signal: gnps.KillProcessRequest_SIGNAL_TERM})
				t.Logf("Got kill process response: %v\n\n", killResponse)
				if err != nil {
					t.Logf("Failed to execute gNOI Kill Process, error received: %v", err)
				}
			}()
		}

		responses := subcribe(t, dut)
		// Construct the Expected gNOI Event
		grpcService := &acctz.GrpcService{
			ServiceType: acctz.GrpcService_GRPC_SERVICE_TYPE_GNOI,
			RpcName:     "/gnoi.system.System/KillProcess",
			// Add other fields as needed
		}
		expected := ExpectedResponse{
			GrpcService: grpcService,
			SessionInfo: DefaultSession,
			Authen:      DefaultAuthentication,
			Timestamp: &timestamppb.Timestamp{
				Seconds: 1707356705,
			},
		}
		VerifyResponses(t, responses, expected)
	})

	t.Run("Test Accouting gNMI Events", func(t *testing.T) {
		var wg1 sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg1.Add(1)
			go func() {
				defer wg1.Done()
				gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "GNMI")
				// gnmi.Get(t, dut, gnmi.OC().System().Hostname().State())
				// gnmi.Replace(t, dut, gnmi.OC().System().Hostname().Config(), "GNMI")
				// //gnmi.Watch(t,dut,gnmi.OC().System().Hostname().State())
				// hostname := "GNMI"
				// state := gnmi.OC().System().Hostname()
				// var ok bool
				// _, ok = gnmi.Watch(t, dut, state.State(), time.Minute, func(val *ygnmi.Value[string]) bool {
				// 	currState, ok := val.Val()
				// 	return ok && currState == hostname
				// }).Await(t)
				// if !ok {
				// 	t.Errorf("Name not correct")
				// } else {
				// 	t.Log(ok)
				// 	t.Log("Ok is here")
				// }
			}()
		}

		responses := subcribe(t, dut)
		// Construct the Expected gNOI Event
		grpcService := &acctz.GrpcService{
			ServiceType: acctz.GrpcService_GRPC_SERVICE_TYPE_GNMI,
			RpcName:     "/gnmi.gNMI/Set",
		}
		expected := ExpectedResponse{
			GrpcService: grpcService,
			SessionInfo: DefaultSession,
			Authen:      DefaultAuthentication,
			Timestamp: &timestamppb.Timestamp{
				Seconds: 1707356705,
			},
		}
		VerifyResponses(t, responses, expected)
	})
}

func TestPreAcctz301(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// if buildInfo, err := sendCLI(t, dut, "run cat /etc/build-info.txt"); err == nil {
	// 	t.Logf("Installed image info:\n%s", buildInfo)
	// }
	//config.TextWithSSH(context.Background(), t, dut, "show version", 10*time.Second)
	t.Run("Test Accouting CLI Run Events", func(t *testing.T) {
		var wg1 sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg1.Add(1)
			go func() {
				defer wg1.Done()
				//config.CMDViaGNMI(context.Background(), t, dut, "run cat /etc/build-info.txt")
				dut.CLI().Run(t, "run cat /etc/build-info.txt")
			}()
		}
		responses := subcribe(t, dut)
		expected := ExpectedResponse{
			CmdService:  DefaultCommandService, // Assign commandService to the CommandService field
			SessionInfo: DefaultSession,
			Authen:      DefaultAuthentication,
			Timestamp: &timestamppb.Timestamp{
				Seconds: 1707356705,
			},
		}

		VerifyResponses(t, responses, expected)
	})

	t.Run("Test Accouting CLI Exec Events", func(t *testing.T) {
		var wg1 sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg1.Add(1)
			go func() {
				defer wg1.Done()
				dut.CLI().Run(t, "show environment fan")
			}()
		}
		responses := subcribe(t, dut)
		ExpCommandService := &acctz.CommandService{
			ServiceType: acctz.CommandService_CMD_SERVICE_TYPE_CLI,
			Cmd:         "show environment fan",
		}
		expected := ExpectedResponse{
			CmdService:  ExpCommandService, // Assign commandService to the CommandService field
			SessionInfo: DefaultSession,
			Authen:      DefaultAuthentication,
			Timestamp: &timestamppb.Timestamp{
				Seconds: 1707356705,
			},
		}

		VerifyResponses(t, responses, expected)
	})

	t.Run("Test Accouting CLI Bash Events", func(t *testing.T) {
		var wg1 sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg1.Add(1)
			go func() {
				defer wg1.Done()
				dut.CLI().Run(t, "run bash")
				dut.CLI().Run(t, "time")
			}()
		}
		responses := subcribe(t, dut)
		ExpCommandService := &acctz.CommandService{
			ServiceType: acctz.CommandService_CMD_SERVICE_TYPE_CLI,
			Cmd:         "run bash",
		}
		expected := ExpectedResponse{
			CmdService:  ExpCommandService, // Assign commandService to the CommandService field
			SessionInfo: DefaultSession,
			Authen:      DefaultAuthentication,
			Timestamp: &timestamppb.Timestamp{
				Seconds: 1707356705,
			},
		}

		VerifyResponses(t, responses, expected)
	})
}

func TestAcctzScaledGNMI(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	var wg1 sync.WaitGroup
	//for j := 0; j < 250; j++ {
	for i := 0; i < 5; i++ {
		wg1.Add(1)
		go func() {
			defer wg1.Done()
			gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "1K")
		}()
	}
	//}
	responses := subcribe(t, dut)
	// Construct the Expected gNMI Event
	grpcService := &acctz.GrpcService{
		ServiceType: acctz.GrpcService_GRPC_SERVICE_TYPE_GNMI,
		RpcName:     "/gnmi.gNMI/Set",
	}
	expected := ExpectedResponse{
		GrpcService: grpcService,
		SessionInfo: DefaultSession,
		Authen:      DefaultAuthentication,
		Timestamp: &timestamppb.Timestamp{
			Seconds: 1707356705,
		},
	}
	VerifyResponses(t, responses, expected)
}

func TestPreAcctzAuthz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	t.Run("Test Accouting AuthZ Rotate Events", func(t *testing.T) {
		_, policyBefore := authz.Get(t, dut)
		for i := 0; i < 5; i++ {
			policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)
		}
		responses := subcribe(t, dut)
		// Construct the Expected AuthZ Event
		grpcService := &acctz.GrpcService{
			ServiceType: acctz.GrpcService_GRPC_SERVICE_TYPE_GNSI,
			RpcName:     "/gnsi.authz.v1.Authz/Get",
		}
		expected := ExpectedResponse{
			GrpcService: grpcService,
			SessionInfo: DefaultSession,
			Authen:      DefaultAuthentication,
			Timestamp: &timestamppb.Timestamp{
				Seconds: 1707356705,
			},
		}
		VerifyResponses(t, responses, expected)
	})

	t.Run("Test Accouting AuthZ Get Events", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			authz.Get(t, dut)
		}
		responses := subcribe(t, dut)
		// Construct the Expected AuthZ Event
		grpcService := &acctz.GrpcService{
			ServiceType: acctz.GrpcService_GRPC_SERVICE_TYPE_GNSI,
			RpcName:     "/gnsi.authz.v1.Authz/Get",
		}
		expected := ExpectedResponse{
			GrpcService: grpcService,
			SessionInfo: DefaultSession,
			Authen:      DefaultAuthentication,
			Timestamp: &timestamppb.Timestamp{
				Seconds: 1707356705,
			},
		}
		VerifyResponses(t, responses, expected)
	})
}

func TestPostAcctzScaledAuthz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Test Accouting AuthZ Events", func(t *testing.T) {
		var wg1 sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg1.Add(1)
			go func() {
				defer wg1.Done()
				authz.Get(t, dut)
			}()
		}
		wg1.Wait()
		responses := subcribe(t, dut)
		// Construct the Expected AuthZ Event
		grpcService := &acctz.GrpcService{
			ServiceType: acctz.GrpcService_GRPC_SERVICE_TYPE_GNSI,
			RpcName:     "/gnsi.authz.v1.Authz/Get",
		}
		expected := ExpectedResponse{
			GrpcService: grpcService,
			SessionInfo: DefaultSession,
			Authen:      DefaultAuthentication,
			Timestamp: &timestamppb.Timestamp{
				Seconds: 1707356705,
			},
		}
		VerifyResponses(t, responses, expected)
	})
}

func TestPreContinuousSubscription(t *testing.T) {
	// Initialize the DUT (Device Under Test)
	dut := ondatra.DUT(t, "dut")

	// Channel to communicate with the goroutine handling subscription
	responseChan := make(chan *acctz.RecordResponse, 1000) // Adjust buffer size as needed

	// Start subscription goroutine
	go func() {
		// Subscribe to records
		resp, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
		if err != nil {
			t.Fatalf("Error subscribing to records: %v", err)
		}
		defer resp.CloseSend() // Close the stream when finished

		// Continuously receive and send responses
		for {
			resp1, err := resp.Recv()
			if err != nil {
				if err != io.EOF {
					t.Fatalf("Error receiving response: %v", err)
				}
				return
			}
			responseChan <- resp1 // Send response to channel
		}
	}()

	// Set up the timer for 24 hours
	timer := time.NewTimer(55 * time.Hour)
	defer timer.Stop()

	// Loop to continuously process responses until timer expires
	for {
		select {
		case <-timer.C:
			t.Logf("Subscription completed after 24 hours")
			return
		case response := <-responseChan:
			// Process the response (you can add your verification logic here)
			t.Logf("Received response: %v", response)
		}
	}
}

func TestSubscriptionLoop(t *testing.T) {
	// Initialize the DUT (Device Under Test)
	dut := ondatra.DUT(t, "dut")

	// Number of times to subscribe and close
	numIterations := 50

	// Loop for subscribing and closing subscriptions
	for i := 0; i < numIterations; i++ {
		// Subscribe to records
		resp, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
		if err != nil {
			t.Fatalf("Error subscribing to records: %v", err)
		}

		// Close the stream when finished
		defer func() {
			if err := resp.CloseSend(); err != nil {
				t.Fatalf("Error closing subscription: %v", err)
			}
		}()

		// Log subscription started
		t.Logf("Subscription %d started", i+1)

		// Wait for 1 minute to receive records
		time.Sleep(1 * time.Minute)

		// Log subscription closed
		t.Logf("Subscription %d closed", i+1)
	}
}

func subcribe(t *testing.T, dut *ondatra.DUTDevice) []*acctz.RecordResponse {
	beforeTimestamp := timestamppb.Now()
	responseChan := make(chan *acctz.RecordResponse, 1000) // Adjust buffer size as needed
	// Slice to store responses
	var responses []*acctz.RecordResponse
	// Subscribe thread
	go func() {
		resp, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
		if err != nil {
			t.Logf("Error subscribing to records: %v", err)
			close(responseChan) // Close the channel to signal termination
			return
		}
		defer resp.CloseSend() // Close the stream when finished

		// Send the initial request in the subscription thread
		resp.Send(&acctz.RecordRequest{
			Timestamp: beforeTimestamp,
		})
		t.Logf("Time Used for Subscribe %v", timestamppb.Now())
		t.Logf("Time Sent %v", timestamppb.Now())

		for {
			resp1, err := resp.Recv()
			if err != nil {
				if err != io.EOF {
					t.Logf("Error receiving response: %v", err)
				}
				close(responseChan) // Close the channel to signal termination
				return
			}
			responseChan <- resp1 // Send response to channel
		}
	}()

	// Process responses outside of the goroutine
	timer := time.NewTimer(30 * time.Second) // Set up the 30-second timer
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			t.Logf("Timer expired, stopping subscription %s", timestamppb.Now())
			return responses // Return the responses received so far
		case response, ok := <-responseChan:
			if !ok {
				t.Logf("Channel closed, stopping subscription %s", timestamppb.Now())
				return responses // Return the responses received so far
			}
			t.Logf("Received response: %v", response)
			responses = append(responses, response)
		}
	}
}

func subscribeAndForwardResponses(dut *ondatra.DUTDevice, t *testing.T, responseChan chan<- *acctz.RecordResponse, beforeTimestamp *timestamppb.Timestamp, wg *sync.WaitGroup) {
	defer wg.Done()
	resp, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
	if err != nil {
		t.Logf("Error subscribing to records: %v", err)
		close(responseChan) // Close the channel to signal termination
		return
	}
	defer resp.CloseSend() // Close the stream when finished

	// Send the initial request in the subscription thread
	resp.Send(&acctz.RecordRequest{
		Timestamp: beforeTimestamp,
	})
	t.Logf("Time Used for Subscribe %v", timestamppb.Now())
	t.Logf("Time Sent %v", timestamppb.Now())
	// Receive and forward responses
	for {
		resp1, err := resp.Recv()
		if err != nil {
			if err != io.EOF {
				t.Logf("Error receiving response: %v", err)
			}
			close(responseChan) // Close the channel to signal termination
			break               // Exit loop if there's an error or end of file
		}
		responseChan <- resp1 // Send response to channel
	}
}

func processResponses(responseChan <-chan *acctz.RecordResponse, done chan<- struct{}, t *testing.T) {
	var responses []*acctz.RecordResponse
	defer func() {
		// Print responses outside of the goroutine
		for _, response := range responses {
			t.Logf("Response outside of goroutine: %v", response)
		}
		close(done) // Signal that processing is done
	}()
	timer := time.NewTimer(30 * time.Second) // Adjust duration as needed
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			// Stop receiving responses after 30 seconds
			return
		case response, ok := <-responseChan:
			if !ok {
				return // Channel closed, exit the goroutine
			}
			// Process each response here
			t.Logf("Acctz.Get response: %v", response)
			responses = append(responses, response)
		}
	}
}

// ACCTZ-1.1 - gNSI.acctz.v1 (Accounting) Test Record Subscribe Full
func AcctzBase(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	//b := dut.Console()
	dut.RawAPIs().BindingDUT().Ports()
	//gnsiC := dut.RawAPIs().GNSI(t)

	// Replace timestamppb.Now() with a static timestamp value
	// gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test1")
	// gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test2")
	// gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test3")

	//staticTimestamp := timestamppb.New(time.Date(2023, 12, 12, 00, 15, 34, 0, time.UTC))
	beforeTimestamp := timestamppb.Now()
	resp, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
	resp.Send(&acctz.RecordRequest{
		Timestamp: beforeTimestamp,
	})
	t.Logf("Time Sent %v", timestamppb.Now())
	//resp, err := gnsiC.Acctz().RecordSubscribe(context.Background())
	//t.Log("Sleeping for 10 seconds after sending Subsribe")
	//time.Sleep(10 * time.Second)
	if err != nil {
		t.Fatalf("Error while receiving Response %v", err)
	}
	resp1, err := resp.Recv()
	if err != nil {
		t.Fatalf("Error while receiving Response %v", err)
	}
	t.Logf("Acctz.Get Authen response is %v", resp1.GetAuthen())
	t.Logf("Acctz.Get Authen2 response is %v", resp1.Authen)
	t.Logf("Acctz.Get Session response is %v", resp1.GetSessionInfo())
	t.Logf("Acctz.Get Session response is %v", resp1.SessionInfo)
	t.Logf("Acctz.Get Service response is %v", resp1.GetServiceRequest())
	t.Logf("Acctz.Get Service response is %v", resp1.ServiceRequest)
	t.Logf("Acctz.Get TimeStamp response is %v", resp1.GetTimestamp())
	t.Logf("Acctz.Get TimeStamp response is %v", resp1.Timestamp)

	t.Logf("Acctz.Get response1 is %v", resp1)
	// err = verifyAcctzSubscribeResponse(t, resp1, beforeTimestamp)
	// if err != nil {
	// 	t.Fatalf("Failed to verify accounting response: %v", err)
	// }
}

func Acctz1101(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	//beforeTimestamp := timestamppb.Now()
	pName := "bgp"
	ctx := context.Background()
	proc := findProcessByName(ctx, t, dut, pName)
	pid := uint32(proc.GetPid())
	killResponse, err := dut.RawAPIs().GNOI(t).System().KillProcess(context.Background(), &gnps.KillProcessRequest{Name: pName, Pid: pid, Restart: true, Signal: gnps.KillProcessRequest_SIGNAL_TERM})
	t.Logf("Got kill process response: %v\n\n", killResponse)
	if err != nil {
		t.Fatalf("Failed to execute gNOI Kill Process, error received: %v", err)
	}
	// for i := 0; i < 3; i++ {
	// 	// Perform the update
	// 	gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test800")
	// }
	var wg1 sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg1.Add(1)
		go func() {
			defer wg1.Done()
			// Perform the update
			gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test11")
			// policyBefore := authz.NewAuthorizationPolicy()
			// policyBefore.Get(t, dut)
		}()
	}
	wg1.Wait()
	_, policyBefore := authz.Get(t, dut)
	t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
	gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test850")
	//var wg sync.WaitGroup

	// Channel to receive responses
	//responseChan := make(chan *acctz.RecordResponse, 1000) // Adjust buffer size as needed

	// Slice to store responses
	//var responses []*acctz.RecordResponse

	// Subscribe thread
	// wg.Add(1)
	// go func() {
	// 	//defer wg.Done()
	// 	resp, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
	// 	if err != nil {
	// 		t.Logf("Error subscribing to records: %v", err)
	// 		//close(responseChan) // Close the channel to signal termination
	// 		return
	// 	}
	// 	defer resp.CloseSend() // Close the stream when finished

	// 	// Send the initial request in the subscription thread
	// 	resp.Send(&acctz.RecordRequest{
	// 		Timestamp: beforeTimestamp,
	// 	})
	// 	t.Logf("Time Used for Subscribe %v", timestamppb.Now())
	// 	t.Logf("Time Sent %v", timestamppb.Now())
	// 	// Receive and forward responses
	// 	for {
	// 		resp1, err := resp.Recv()
	// 		if err != nil {
	// 			if err != io.EOF {
	// 				t.Logf("Error receiving response: %v", err)
	// 			}
	// 			//close(responseChan) // Close the channel to signal termination
	// 			break // Exit loop if there's an error or end of file
	// 		}
	// 		responseChan <- resp1 // Send response to channel
	// 	}
	// }()

	// //wg.Add(2)
	// // Process responses outside of the goroutine
	// go func() {
	// 	defer wg.Done()
	// 	timer := time.NewTimer(30 * time.Second) // Adjust duration as needed
	// 	defer timer.Stop()
	// 	for {
	// 		select {
	// 		case <-timer.C:
	// 			// Stop receiving responses after 30 seconds
	// 			//close(responseChan)
	// 			return
	// 		case response, ok := <-responseChan:
	// 			if !ok {
	// 				return // Channel closed, exit the goroutine
	// 			}
	// 			// Process each response here
	// 			//t.Logf("Acctz.Get response: %v", response)
	// 			responses = append(responses, response)
	// 		}
	// 	}
	// }()

	// // Wait for all goroutines to finish
	// wg.Wait()

	// Trial
	t.Logf("Loop entering %s", timestamppb.Now())
	responses := subcribe(t, dut)
	t.Logf("After Subscribe %s", timestamppb.Now())
	// Subscribe thread
	// go func() {
	// 	resp, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
	// 	if err != nil {
	// 		t.Logf("Error subscribing to records: %v", err)
	// 		return
	// 	}
	// 	defer resp.CloseSend() // Close the stream when finished

	// 	// Send the initial request in the subscription thread
	// 	resp.Send(&acctz.RecordRequest{
	// 		Timestamp: beforeTimestamp,
	// 	})
	// 	t.Logf("Time Used for Subscribe %v", timestamppb.Now())
	// 	t.Logf("Time Sent %v", timestamppb.Now())

	// 	timer := time.NewTimer(30 * time.Second) // Adjust duration as needed
	// 	defer timer.Stop()
	// 	defer close(responseChan)
	// 	for {
	// 		select {
	// 		case <-timer.C:
	// 			// Stop receiving responses after 30 seconds
	// 			return
	// 		default:
	// 			resp1, err := resp.Recv()
	// 			if err != nil {
	// 				if err != io.EOF {
	// 					t.Logf("Error receiving response: %v", err)
	// 				}
	// 				return
	// 			}
	// 			responseChan <- resp1 // Send response to channel
	// 		}
	// 	}
	// }()

	// // Print responses outside of the goroutine
	// for response := range responseChan {
	// 	t.Logf("Response outside of goroutine: %v", response)
	// 	responses = append(responses, response)
	// }
	// t.Logf("Loop exited")
	// data, err := json.Marshal(responses)
	// if err != nil {
	// 	fmt.Println("Error marshaling responses:", err)
	// 	return
	// }

	// Write the JSON data to a file
	// err := os.WriteFile("responses.json", data, 0644)
	// if err != nil {
	// 	fmt.Println("Error writing responses to file:", err)
	// 	return
	// }

	// data, err := os.ReadFile("responses.json")
	// if err != nil {
	// 	fmt.Println("Error reading data from file:", err)
	// 	return
	// }

	// fmt.Println("Content of responses.json:", string(data))

	// jsonData := `[
	// 	{
	// 		"session_info": {
	// 		  "local_address": "10.85.84.159",
	// 		  "local_port": 35000,
	// 		  "remote_address": "2807:f8b0:f800:c00::4e",
	// 		  "remote_port": 39904,
	// 		  "ip_proto": 4,
	// 		  "channel_id": "0"
	// 		},
	// 		"timestamp": {
	// 		  "seconds": 1708640105
	// 		},
	// 		"grpc_service": {
	// 		  "service_type": "GRPC_SERVICE_TYPE_GNMI",
	// 		  "rpc_name": "Subscribe"
	// 		},
	// 		"authen": {
	// 		  "identity": "cafyauto",
	// 		  "deny_cause": "None"
	// 		}
	// 	  },
	// 	  {
	// 		"session_info": {
	// 		  "local_address": "10.85.84.159",
	// 		  "local_port": 35000,
	// 		  "remote_address": "2807:f8b0:f800:c00::4e",
	// 		  "remote_port": 39992,
	// 		  "ip_proto": 4,
	// 		  "channel_id": "0"
	// 		},
	// 		"timestamp": {
	// 		  "seconds": 1708976619
	// 		},
	// 		"grpc_service": {
	// 		  "service_type": "GRPC_SERVICE_TYPE_GNSI",
	// 		  "rpc_name": "RecordSubscribe"
	// 		},
	// 		"authen": {
	// 		  "identity": "cafyauto",
	// 		  "deny_cause": "None"
	// 		}
	// 	  }

	// 			  ]`

	// // Define a slice to store the unmarshaled responses
	// err := json.Unmarshal([]byte(jsonData), &responses)
	// if err != nil {
	// 	fmt.Println("Error unmarshaling responses:", err)
	// 	return
	// }

	// // Now you can use the responses slice as needed
	// fmt.Println("Retrieved responses:")
	// for _, response := range responses {
	// 	// Process each response here
	// 	fmt.Println(response)
	// }

	// Construct the gRPC service
	grpcService := &acctz.GrpcService{
		ServiceType: acctz.GrpcService_GRPC_SERVICE_TYPE_GNSI,
		RpcName:     "RecordSubscribe",
		// Add other fields as needed
	}
	authentication := &acctz.AuthDetail{
		Identity:       "cafyauto",
		PrivilegeLevel: 2,
		Status:         acctz.AuthDetail_AUTHEN_STATUS_PERMIT,
		DenyCause:      "None",
	}
	expected := ExpectedResponse{
		GrpcService: grpcService,
		SessionInfo: &acctz.SessionInfo{
			LocalAddress:  "10.85.84.159",
			LocalPort:     35000,
			RemoteAddress: "2807:f8b0:f800:c00::8",
			RemotePort:    38434,
			IpProto:       4,
			ChannelId:     "0",
		},
		Timestamp: &timestamppb.Timestamp{
			Seconds: 1707356705,
		},
		Authen: authentication,
	}
	VerifyResponses(t, responses, expected)
}

func Acctz201(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	beforeTimestamp := timestamppb.Now()
	var wg sync.WaitGroup

	// Channel to receive responses
	responseChan := make(chan *acctz.RecordResponse, 100) // Adjust buffer size as needed

	// Slice to store responses
	var responses []*acctz.RecordResponse

	// Subscribe thread
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
		if err != nil {
			t.Logf("Error subscribing to records: %v", err)
			close(responseChan) // Close the channel to signal termination
			return
		}
		defer resp.CloseSend() // Close the stream when finished

		// Send the initial request in the subscription thread
		resp.Send(&acctz.RecordRequest{
			Timestamp: beforeTimestamp,
		})
		t.Logf("Time Used for Subscribe %v", timestamppb.Now())
		t.Logf("Time Sent %v", timestamppb.Now())
		// Receive and forward responses
		for {
			resp1, err := resp.Recv()
			if err != nil {
				if err != io.EOF {
					t.Logf("Error receiving response: %v", err)
				}
				close(responseChan) // Close the channel to signal termination
				break               // Exit loop if there's an error or end of file
			}
			responseChan <- resp1 // Send response to channel
		}
	}()

	// Process responses outside of the goroutine
	go func() {
		defer wg.Done()
		timer := time.NewTimer(30 * time.Second) // Adjust duration as needed
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				// Stop receiving responses after 30 seconds
				close(responseChan)
				return
			case response, ok := <-responseChan:
				if !ok {
					return // Channel closed, exit the goroutine
				}
				// Process each response here
				t.Logf("Acctz.Get response: %v", response)
				responses = append(responses, response)
			}
		}
	}()

	// Wait for all goroutines to finish
	wg.Wait()

	// Print responses outside of the goroutine
	for _, response := range responses {
		t.Logf("Response outside of goroutine: %v", response)
	}

	// Construct the gRPC service
	grpcService := &acctz.GrpcService{
		ServiceType: acctz.GrpcService_GRPC_SERVICE_TYPE_GNMI,
		RpcName:     "RecordSubscribe",
		// Add other fields as needed
	}
	authentication := &acctz.AuthDetail{
		Identity:       "cafyauto",
		PrivilegeLevel: 2,
		Status:         acctz.AuthDetail_AUTHEN_STATUS_PERMIT,
		DenyCause:      "None",
	}
	expected := ExpectedResponse{
		GrpcService: grpcService,
		SessionInfo: &acctz.SessionInfo{
			LocalAddress:  "10.85.84.159",
			LocalPort:     35000,
			RemoteAddress: "2807:f8b0:f800:c00::4e",
			RemotePort:    38434,
			IpProto:       4,
			ChannelId:     "0",
		},
		Timestamp: &timestamppb.Timestamp{
			Seconds: 1707356705,
		},
		Authen: authentication,
	}
	VerifyResponses(t, responses, expected)
}

func Acctz101(t *testing.T) {
	dut := ondatra.DUT(t, "dut") // Call ondatra.DUT to obtain a *ondatra.DUTDevice object
	beforeTimestamp := timestamppb.Now()
	var wg sync.WaitGroup

	// Channel to receive responses
	responseChan := make(chan *acctz.RecordResponse, 100) // Adjust buffer size as needed
	// Channel to signal completion of processing
	done := make(chan struct{})

	// Slice to store responses
	var responses []*acctz.RecordResponse

	// Subscribe thread
	wg.Add(1)
	go subscribeAndForwardResponses(dut, t, responseChan, beforeTimestamp, &wg)

	// Process responses outside of the goroutine
	wg.Add(1)
	go processResponses(responseChan, done, t)

	// Wait for all goroutines to finish
	wg.Wait()

	// Close the response channel after processing is done
	close(responseChan)
	// Print responses outside of the goroutine
	for _, response := range responses {
		t.Logf("Response outside of goroutine: %v", response)
	}

	// Construct the gRPC service
	grpcService := &acctz.GrpcService{
		ServiceType: acctz.GrpcService_GRPC_SERVICE_TYPE_GNMI,
		RpcName:     "RecordSubscribe",
		// Add other fields as needed
	}
	authentication := &acctz.AuthDetail{
		Identity:       "cafyauto",
		PrivilegeLevel: 2,
		Status:         acctz.AuthDetail_AUTHEN_STATUS_PERMIT,
		DenyCause:      "None",
	}
	expected := ExpectedResponse{
		GrpcService: grpcService,
		SessionInfo: &acctz.SessionInfo{
			LocalAddress:  "10.85.84.159",
			LocalPort:     35000,
			RemoteAddress: "2807:f8b0:f800:c00::4e",
			RemotePort:    38434,
			IpProto:       4,
			ChannelId:     "0",
		},
		Timestamp: &timestamppb.Timestamp{
			Seconds: 1707356705,
		},
		Authen: authentication,
	}
	VerifyResponses(t, responses, expected)

}

func Acctz51(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	beforeTimestamp := timestamppb.Now()
	var wg sync.WaitGroup

	// Channel to receive responses
	responseChan := make(chan *acctz.RecordResponse, 100) // Adjust buffer size as needed

	// Subscribe thread
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
		if err != nil {
			t.Logf("Error subscribing to records: %v", err)
			close(responseChan) // Close the channel to signal termination
			return
		}
		defer resp.CloseSend() // Close the stream when finished

		// Send the initial request in the subscription thread
		resp.Send(&acctz.RecordRequest{
			Timestamp: beforeTimestamp,
		})

		// Receive and forward responses
		for {
			resp1, err := resp.Recv()
			if err != nil {
				if err != io.EOF {
					t.Logf("Error receiving response: %v", err)
				}
				close(responseChan) // Close the channel to signal termination
				break               // Exit loop if there's an error or end of file
			}
			responseChan <- resp1 // Send response to channel
		}
	}()

	// Process responses
	go func() {
		defer wg.Done()
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				// Stop receiving responses after 30 seconds
				close(responseChan)
				return
			case response, ok := <-responseChan:
				if !ok {
					return // Channel closed, exit the goroutine
				}
				// Process each response here
				t.Logf("Acctz.Get response: %v", response)
			}
		}
	}()

	// Wait for all goroutines to finish
	wg.Wait()
}

func Acctz11(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	beforeTimestamp := timestamppb.Now()
	var wg sync.WaitGroup

	// Channel to signal termination
	done := make(chan struct{})

	// Channel to signal completion of response processing
	responseProcessed := make(chan struct{})

	// Response channel
	responseChan := make(chan *acctz.RecordResponse)

	// Subscribe thread
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
		if err != nil {
			t.Logf("Error subscribing to records: %v", err)
			close(done) // Signal termination if there is an error
			return
		}
		defer resp.CloseSend() // Close the stream when finished

		// Send the initial request in the subscription thread
		resp.Send(&acctz.RecordRequest{
			Timestamp: beforeTimestamp,
		})
		t.Logf("Time Sent %v", timestamppb.Now())

		// Wait for termination signal or response processing completion
		select {
		case <-done:
		case <-responseProcessed:
		}
	}()

	// Response thread
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Define timer duration
		duration := time.Duration(30) * time.Second

		// Signal termination after the set timer duration
		timer := time.NewTimer(duration)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				close(done) // Signal termination after the set timer duration
				return      // Exit the loop after the timer duration
			case resp := <-responseChan:
				t.Logf("Acctz.Get response: %v", resp)
				responseProcessed <- struct{}{} // Signal that response processing is completed
			}
		}
	}()

	// Wait for both goroutines to finish
	wg.Wait()
}

func Acctz1(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	beforeTimestamp := timestamppb.Now()

	// Wait group to synchronize threads
	wg := sync.WaitGroup{}

	resp, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
	// Subscribe thread
	wg.Add(1)
	go func() {
		if err != nil {
			t.Logf("Error subscribing to records: %v", err)
		}
		defer resp.CloseSend() // Close the stream when finished
		// Send the initial request in the subscription thread
		resp.Send(&acctz.RecordRequest{
			Timestamp: beforeTimestamp,
		})
		t.Logf("Time Sent %v", timestamppb.Now())

	}()

	// Response thread
	responseChan := make(chan *acctz.RecordResponse, 100)
	go func() {
		ticker := time.NewTicker(5 * time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		defer close(responseChan) // Close the channel when done to signal completion
		for {
			select {
			case <-ctx.Done():
				// Post process here
				//t.Logf("Acctz.Get response: %v", response)
				wg.Done()
				responseChan <- nil
				return // Break out of the loop if timeout occurs
			case <-ticker.C:
				if err := ctx.Err(); err != nil {
					wg.Done()
					return // Break out of the loop if context is canceled
				}
				resp1, err := resp.Recv()
				if err != nil {
					t.Logf("Error receiving response Retrying: %v", err)
					break
				}
				t.Logf("Loop Acctz.Get response: %v", resp1)
				responseChan <- resp1
				wg.Done()
				return
			}
		}
	}()
	wg.Wait()
	var responses []*acctz.RecordResponse
	for response := range responseChan { // Receive all responses from the channel
		responses = append(responses, response)
	}

	t.Logf("All responses received: %v", responses)

	response := <-responseChan
	t.Logf("Acctz.Get response: %v", response)
}

func Acctz10(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	beforeTimestamp := timestamppb.Now()
	resp, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
	resp.Send(&acctz.RecordRequest{
		Timestamp: beforeTimestamp,
	})
	t.Logf("Time Sent %v", timestamppb.Now())
	if err != nil {
		t.Fatalf("Error while receiving Response %v", err)
	}
	//
	go func() {
		// Wait for the defined time
		time.Sleep(time.Duration(30) * time.Second) // Adjust the duration as needed

		// Signal termination
		for {
			resp1, err := resp.Recv()
			if err != nil {
				t.Logf("Error receiving response: %v", err)
			}
			t.Logf("Acctz.Get response: %v", resp1)
		}
	}()
}

// ACCTZ-2.1 - gNSI.acctz.v1 (Accounting) Test Record Subscribe Partial
func Acctz2(t *testing.T) {
	t.Run("Acctz2.1 - gNSI.acctz.v1 (Accounting) Test Record Subscribe Partial", func(t *testing.T) {
		// Test Record Subscribe for records since a non-zero timestamp
		dut := ondatra.DUT(t, "dut")
		gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test1")
		gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test2")
		time.Sleep(1 * time.Second)
		gnsiC := dut.RawAPIs().GNSI(t)
		gnsiC.Acctz().RecordSubscribe(context.Background())
		beforeTimestamp := timestamppb.Now()
		// Create a new timestamp with only the date component
		zeroTimeTimestamp := timestamppb.Timestamp{
			Seconds: beforeTimestamp.Seconds,
			Nanos:   0, // Set nanoseconds to 0 to clear the time component
		}
		req, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
		req.Send(&acctz.RecordRequest{
			Timestamp: &zeroTimeTimestamp,
		})
		if err != nil {
			t.Fatalf("Error while receiving Response %v", err)
		}
		resp, err := req.Recv()
		if err != nil {
			t.Fatalf("Error while receiving Response %v", err)
		}
		firstrecordTime := resp.Timestamp
		req1, err := dut.RawAPIs().GNSI(t).Acctz().RecordSubscribe(context.Background())
		req1.Send(&acctz.RecordRequest{
			Timestamp: firstrecordTime,
		})
		if err != nil {
			t.Fatalf("Error while receiving Response %v", err)
		}
		// Verify Accouting Response
		err = verifyAcctzSubscribeResponse(t, resp, beforeTimestamp)
		if err != nil {
			t.Fatalf("Failed to verify accounting response: %v", err)
		}
		resp1, err := req.Recv()
		if err != nil {
			t.Fatalf("Error while receiving Response %v", err)
		}
		t.Logf("Acctz.Get response1 is %v", resp1)
	})
}

func verifyAcctzSubscribeResponse(t *testing.T, resp *acctz.RecordResponse, Timestampbefore *timestamppb.Timestamp) error {
	// Verify session_info
	if err := old_verifySessionInfo(t, resp.GetSessionInfo()); err != nil {
		t.Logf("failed to verify session_info: %v", err)
	}
	// Verify TimeStamp_info
	if err := verifyTimestampInfo(t, resp.GetTimestamp(), Timestampbefore); err != nil {
		t.Logf("failed to verify timestamp_info: %v", err)
	}

	// Verify authen_type
	if err := verifyAuthenType(resp.GetAuthen()); err != nil {
		return fmt.Errorf("failed to verify authen_type: %w", err)
	}

	// // Verify user identity
	// if err := verifyUserIdentity(resp.GetUser().GetIdentity()); err != nil {
	//   return fmt.Errorf("failed to verify user identity: %w", err)
	// }

	// // Verify rpc_name
	// if err := verifyRPCName(resp.GetRpcName()); err != nil {
	//   return fmt.Errorf("failed to verify rpc_name: %w", err)
	// }

	// // Verify payloads
	// if err := verifyPayloads(resp.ProtoMessage); err != nil {
	// 	return fmt.Errorf("failed to verify payloads: %w", err)
	// }

	return nil
}

// Implement individual verification functions for each field
func old_verifySessionInfo(t *testing.T, sessionInfo *acctz.SessionInfo) error {
	expLocalAddress := "None"
	if !cmp.Equal(expLocalAddress, sessionInfo.LocalAddress) {
		t.Logf("Acctz Session Info Local Address is %v", sessionInfo.LocalAddress)
		t.Logf("Acctz Session Info Local Address is %v", expLocalAddress)
	}
	return nil
}

func verifyTimestampInfo(t *testing.T, Timestampresponse *timestamppb.Timestamp, Timestampbefore *timestamppb.Timestamp) error {
	// if !cmp.Equal(Timestampbefore, Timestampresponse) {
	// 	t.Logf("Acctz Session Info Local Address is %v", Timestampresponse)
	// 	t.Logf("Acctz Session Info Local Address is %v", Timestampbefore)
	// }
	return nil
}

func verifyAuthenType(auth *acctz.AuthDetail) error {
	// Implement logic to verify payloads
	// ...
	return nil
}

// Define a struct for the expected response fields
type ExpectedResponse struct {
	SessionInfo        *acctz.SessionInfo
	Timestamp          *timestamppb.Timestamp
	Authen             *acctz.AuthDetail
	GrpcService        *acctz.GrpcService
	CmdService         *acctz.CommandService
	HistoryIstruncated bool
}

//lint:ignore U1000 Ignore unused function warning
func absUint32(x, y uint32) uint32 {
	if x > y {
		return x - y
	}
	return y - x
}

// Define a function to verify the responses
func VerifyResponses(t *testing.T, responses []*acctz.RecordResponse, expected ExpectedResponse) {
	// Iterate over each response
	// Initialize flag to track mismatches
	match := false
	for _, response := range responses {
		// Verify session_info field
		if expected.GrpcService != nil {
			// Check if gRPC service type matches the expected value
			actualGrpcService := response.GetGrpcService()
			if actualGrpcService != nil && actualGrpcService.GetServiceType() == expected.GrpcService.ServiceType {
				// Additional verification if the gRPC service type matches
				// For example:
				// Compare other fields or perform specific checks
				if actualGrpcService.RpcName != expected.GrpcService.RpcName {
					continue
					//t.Errorf("Session info verification failed. Expected: %v, Got: %v", expected.GrpcService.RpcName, actualGrpcService.RpcName)
				}
				t.Logf("gRPC service type match found. Service type: %v", expected.GrpcService.ServiceType)
				match = true
				sessionInfoPassed := verifySessionInfo(t, response, expected.SessionInfo)
				if !sessionInfoPassed {
					return // Return early if session information verification fails
				}
				// Verify timestamp
				timestampPassed := verifyTimestamp(t, response, expected.Timestamp)
				if !timestampPassed {
					return // Return early if timestamp verification fails
				}
				// Verify authentication details
				authPassed := verifyAuthentication(t, response, expected.Authen)
				if !authPassed {
					return // Return early if authentication verification fails
				}
				//return
			}
		} else if expected.CmdService != nil {
			actualCmdService := response.GetCmdService()
			if actualCmdService != nil && actualCmdService.GetServiceType() == expected.CmdService.ServiceType {
				if !cmp.Equal(actualCmdService.Cmd, expected.CmdService.Cmd) {
					// t.Logf("Cmd service type match NOT found. Expected Service type: %q", expected.CmdService.Cmd)
					// t.Logf("Cmd service type match NOT found. Actual Service type: %q", actualCmdService.Cmd)
					continue
				}
				t.Logf("Cmd service type match found. Service type: %v", expected.CmdService.Cmd)
				match = true
			}
		} else {
			t.Errorf("No Match Accouting Found")
		}
	}
	if !match {
		t.Fatal("No Match Accouting Found")
	}
}

// Define a function to verify session information
func verifySessionInfo(t *testing.T, response *acctz.RecordResponse, expectedSessionInfo *acctz.SessionInfo) bool {
	if expectedSessionInfo != nil {
		actualSessionInfo := response.GetSessionInfo()
		if actualSessionInfo == nil {
			t.Errorf("Expected session info is not nil, but actual session info is nil")
			return false
		}
		// Compare session info fields
		//const portTolerance = uint32(20) // Allow a difference of 2 in port values
		if actualSessionInfo.LocalAddress != expectedSessionInfo.LocalAddress ||
			actualSessionInfo.LocalPort != expectedSessionInfo.LocalPort ||
			actualSessionInfo.RemoteAddress != expectedSessionInfo.RemoteAddress ||
			//actualSessionInfo.RemotePort != expectedSessionInfo.RemotePort ||
			actualSessionInfo.IpProto != expectedSessionInfo.IpProto ||
			actualSessionInfo.ChannelId != expectedSessionInfo.ChannelId {
			t.Errorf("Session info verification failed. Expected: %v, Got: %v", expectedSessionInfo, actualSessionInfo)
			return false
		}
	}
	return true
}

// Define a function to verify the timestamp
func verifyTimestamp(t *testing.T, response *acctz.RecordResponse, expectedTimestamp *timestamppb.Timestamp) bool {
	actualTimestamp := response.GetTimestamp()
	if actualTimestamp == nil || expectedTimestamp == nil {
		t.Errorf("Nil timestamp")
		return false
	}

	actualSeconds := actualTimestamp.GetSeconds()
	expectedSeconds := expectedTimestamp.GetSeconds()

	if actualSeconds <= expectedSeconds {
		t.Errorf("Actual timestamp is not greater than expected. Actual: %d, Expected: %d", actualSeconds, expectedSeconds)
		return false
	}

	return true
}

// Define a function to verify authentication details
func verifyAuthentication(t *testing.T, response *acctz.RecordResponse, expectedAuth *acctz.AuthDetail) bool {
	actualAuth := response.GetAuthen()
	if actualAuth == nil || expectedAuth == nil {
		t.Error("Nil authentication details")
		return false
	}

	if actualAuth.GetIdentity() != expectedAuth.GetIdentity() {
		t.Errorf("Mismatched identity. Expected: %s, Got: %s", expectedAuth.GetIdentity(), actualAuth.GetIdentity())
		return false
	}

	// if actualAuth.GetPrivilegeLevel() != expectedAuth.GetPrivilegeLevel() {
	// 	t.Errorf("Mismatched privilege level. Expected: %d, Got: %d", expectedAuth.GetPrivilegeLevel(), actualAuth.GetPrivilegeLevel())
	// 	return false
	// }

	if actualAuth.GetStatus() != expectedAuth.GetStatus() {
		t.Errorf("Mismatched authentication status. Expected: %v, Got: %v", expectedAuth.GetStatus(), actualAuth.GetStatus())
		return false
	}

	if actualAuth.GetDenyCause() != expectedAuth.GetDenyCause() {
		t.Errorf("Mismatched deny cause. Expected: %s, Got: %s", expectedAuth.GetDenyCause(), actualAuth.GetDenyCause())
		return false
	}

	return true
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func GnoiSystemTime(ctx context.Context, dut *ondatra.DUTDevice, opts []grpc.DialOption, _ ...any) error {
	gnoiC, err := dut.RawAPIs().BindingDUT().DialGNOI(ctx, opts...)
	if err != nil {
		return err
	}
	_, err = gnoiC.System().Time(ctx, &system.TimeRequest{})
	return err
}

// findProcessByName uses telemetry to collect and return the process information. It return nill if the process is not found.
//
//lint:ignore U1000 Ignore unused function warning
func findProcessByName(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, pName string) *oc.System_Process {
	pList := gnmi.GetAll(t, dut, gnmi.OC().System().ProcessAny().State())
	for _, proc := range pList {
		if proc.GetName() == pName {
			t.Logf("Pid of daemon '%s' is '%d'", pName, proc.GetPid())
			return proc
		}
	}
	return nil
}

//lint:ignore U1000 Ignore unused function warning
func sendCLI(t testing.TB, dut *ondatra.DUTDevice, cmd string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30)
	defer cancel()
	sshClient := dut.RawAPIs().CLI(t)
	out, err := sshClient.RunCommand(ctx, cmd)
	return out.Output(), err
}

// Will be used when Response needs to be Marshalled as JSON
//
//lint:ignore U1000 Ignore unused function warning
func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func getIpAndPortFromBindingFile(grpc bool) (string, int, error) {
	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		return "", 0, err
	}
	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		return "", 0, err
	}

	var target string
	if grpc {
		target = b.Duts[0].Gnmi.Target
	} else {
		target = b.Duts[0].P4Rt.Target
	}

	targetIP := strings.Split(target, ":")[0]
	targetPort, _ := strconv.Atoi(strings.Split(target, ":")[1])
	return targetIP, targetPort, nil
}
