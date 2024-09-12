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

package pathz

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	// "encoding/json"
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/povsister/scp"

	perf "github.com/openconfig/featureprofiles/feature/cisco/performance"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/cisco/ha/utils"
	"github.com/openconfig/featureprofiles/internal/cisco/security/pathz"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/authz"
	"github.com/openconfig/featureprofiles/internal/security/gnxi"
	bindpb "github.com/openconfig/featureprofiles/topologies/proto/binding"
	"github.com/openconfig/gnmi/errdiff"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	pathzpb "github.com/openconfig/gnsi/pathz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding/introspect"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/testing/protocmp"
)

type UsersMap map[string]pathz.Spiffe

// type policyMap map[string]pathz.AuthorizationPolicy

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type setOperation int
type targetInfo struct {
	dut     string
	sshIp   string
	sshPort int
	sshUser string
	sshPass string
}

type AuthorizationRule struct {
	Id        string
	Path      *gpb.Path
	Principal *pathzpb.AuthorizationRule_Group
	Mode      pathzpb.Mode
	Action    pathzpb.Action
}

const (
	// deletePath represents a SetRequest delete.
	deletePath setOperation = iota
	// replacePath represents a SetRequest replace.
	replacePath
	// updatePath represents a SetRequest update.
	updatePath
	isisInstance = "B4"
	processName  = "emsd"
	maxRetries   = 5
	ActionDeny   = "pathzpb.Action_ACTION_DENY"
	ActionPermit = "pathzpb.Action_ACTION_PERMIT"
	ModeRead     = "pathzpb.Mode_MODE_READ"
)

func configwithprefix(t *testing.T, dut *ondatra.DUTDevice, op setOperation, origin string, config string) {
	jsonConfig, _ := json.Marshal(config)
	r := &gpb.SetRequest{
		Prefix: &gpb.Path{
			Origin: origin,
		},
	}

	switch op {
	case updatePath:
		r.Update = []*gpb.Update{
			{
				Path: &gpb.Path{
					Elem: []*gpb.PathElem{
						{Name: "hw-module"},
						{Name: "local-mac"},
						{Name: "address"},
					},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_JsonIetfVal{
						JsonIetfVal: jsonConfig,
					},
				},
			},
		}

	case replacePath:
		r.Replace = []*gpb.Update{
			{
				Path: &gpb.Path{
					Elem: []*gpb.PathElem{
						{Name: "hw-module"},
						{Name: "local-mac"},
						{Name: "address"},
					},
				},
				Val: &gpb.TypedValue{
					Value: &gpb.TypedValue_JsonIetfVal{
						JsonIetfVal: jsonConfig,
					},
				},
			},
		}

	case deletePath:
		r.Delete = []*gpb.Path{
			{
				Elem: []*gpb.PathElem{
					{Name: "hw-module"},
					{Name: "local-mac"},
					{Name: "address"},
				},
			},
		}
	}

	_, err := dut.RawAPIs().GNMI(t).Set(context.Background(), r)
	t.Logf("Rec Err %v", err)
	if err == nil {
		t.Error("This gNMI SET Operation should have failed: ", err)

	}
}
func configwithoutprefix(t *testing.T, dut *ondatra.DUTDevice, op setOperation, config string) {
	json_config, _ := json.Marshal(config)
	path := &gpb.Path{Origin: "Cisco-IOS-XR-um-hostname-cfg", Elem: []*gpb.PathElem{
		{Name: "hostname"},
		{Name: "system-network-name"}}}
	val := &gpb.TypedValue{Value: &gpb.TypedValue_JsonIetfVal{JsonIetfVal: json_config}}
	r := &gpb.SetRequest{}

	switch op {
	case updatePath:
		r = &gpb.SetRequest{
			Update: []*gpb.Update{{Path: path, Val: val}},
		}

	case replacePath:
		r = &gpb.SetRequest{
			Replace: []*gpb.Update{{Path: path, Val: val}},
		}

	case deletePath:
		r = &gpb.SetRequest{
			Delete: []*gpb.Path{path},
		}

	}

	_, err := dut.RawAPIs().GNMI(t).Set(context.Background(), r)
	t.Logf("Rec Err %v", err)
	if err == nil {
		t.Error("This gNMI SET Operation should have failed : ", err)

	}
}
func getSandboxResponse(t *testing.T, want *pathzpb.GetResponse) {
	client := start(t)
	getReq := &pathzpb.GetRequest{
		PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
	}
	for i := 0; i < 3; i++ {
		got, _ := client.Get(context.Background(), getReq)
		t.Logf("Got sandbox Response : %v", got)
		t.Logf("Required Sanbox Response: %v", want)
		if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
			t.Logf("diff : %v", d)
			if i < 2 {
				continue
			}
			t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
		} else {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}
func configISIS(t *testing.T, dut *ondatra.DUTDevice) {
	model := oc.NetworkInstance_Protocol{
		Name:       ygot.String("B4"),
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS,
		Isis: &oc.NetworkInstance_Protocol_Isis{
			Global: &oc.NetworkInstance_Protocol_Isis_Global{
				LspBit: &oc.NetworkInstance_Protocol_Isis_Global_LspBit{
					OverloadBit: &oc.NetworkInstance_Protocol_Isis_Global_LspBit_OverloadBit{
						SetBit: ygot.Bool(true),
					},
				},
			},
		},
	}

	request := &oc.NetworkInstance{
		Name:     ygot.String("DEFAULT"),
		Protocol: make(map[oc.NetworkInstance_Protocol_Key]*oc.NetworkInstance_Protocol),
	}
	request.Protocol[oc.NetworkInstance_Protocol_Key{Name: "B4", Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS}] = &model

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), request)
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Config())
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), request)
}
func gnmiwithcli(t *testing.T, dut *ondatra.DUTDevice, op setOperation, cfg string) *gpb.SetResponse {
	t.Helper()
	gnmiC := dut.RawAPIs().GNMI(t)
	textReq := &gpb.Path{Origin: "cli"}
	val := &gpb.TypedValue{
		Value: &gpb.TypedValue_AsciiVal{
			AsciiVal: cfg,
		}}

	r := &gpb.SetRequest{}

	switch op {
	case updatePath:
		r = &gpb.SetRequest{
			Update: []*gpb.Update{{Path: textReq, Val: val}},
		}
	case deletePath:
		r = &gpb.SetRequest{
			Update: []*gpb.Update{{Path: textReq, Val: val}},
		}
	}

	t.Logf("Request Sent: %s", r)
	if _, deadlineSet := context.Background().Deadline(); !deadlineSet {
		_, cncl := context.WithTimeout(context.Background(), time.Second*120)
		defer cncl()
	}
	resp, err := gnmiC.Set(context.Background(), r)
	if err != nil {
		t.Fatalf("GNMI replace is failed; %v", err)
	}
	return resp
}
func isPermissionDeniedError(t *testing.T, dut *ondatra.DUTDevice, oper string) {
	config := gnmi.OC().System().Hostname()
	operations := []struct {
		name      string
		operation func(t testing.TB)
	}{
		{"Update", func(t testing.TB) { gnmi.Update(t, dut, config.Config(), "SF2") }},
		{"Delete", func(t testing.TB) { gnmi.Delete(t, dut, config.Config()) }},
		{"Replace", func(t testing.TB) { gnmi.Replace(t, dut, config.Config(), "MTB_SF2") }},
		// {"Get", func(t testing.TB) { gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State()) }},
	}

	for _, op := range operations {
		t.Run(op.name+oper, func(t *testing.T) {
			if errMsg := testt.CaptureFatal(t, op.operation); errMsg != nil {
				t.Logf("Expected failure for %s and got testt.CaptureFatal errMsg: %s", op.name, *errMsg)
			} else {
				t.Errorf("This gNMI operation (%s) should have failed", op.name)
			}
		})
	}
}
func performOperations(t *testing.T, dut *ondatra.DUTDevice) {
	config := gnmi.OC().System().Hostname()

	gnmi.Update(t, dut, config.Config(), "SF2")

	gnmi.Delete(t, dut, config.Config())

	gnmi.Replace(t, dut, config.Config(), "MTB_SF2")

	// Get and store the result in portNum
	portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

	if portNum == uint16(0) || portNum > uint16(0) {
		t.Logf("Got the expected port number")
	} else {
		t.Fatalf("Unexpected value for port number: %v", portNum)
	}
}
func TestPathz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Run("Test invalid policy without finalize", func(t *testing.T) {
		// Define the expected error string
		wantErrs := "invalid policy"

		reqs := []*pathzpb.RotateRequest{{
			RotateRequest: &pathzpb.RotateRequest_UploadRequest{
				UploadRequest: &pathzpb.UploadRequest{
					Policy: &pathzpb.AuthorizationPolicy{
						Rules: []*pathzpb.AuthorizationRule{{}},
					},
				},
			},
		}}
		client := start(t)
		rot, err := client.Rotate(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for _, req := range reqs {
			t.Logf("Request Sent: %s", req)
			if err := rot.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}
			received, err := rot.Recv()
			t.Logf("Received Request: %s", received)
			t.Logf("Rec Err %v", err)

			// Check if the error string matches the expected error string
			if d := errdiff.Check(err, wantErrs); d != "" {
				t.Errorf("Rotate() unexpected err: %s", d)
			}
		}

		performOperations(t, dut)
		pathz.VerifyPolicyInfo(t, dut, 0, "", true)

		// Verify the pathz policy statistics.
		expectedStats := map[string]int{
			"PolicyRotations":      1,
			"PolicyRotationErrors": 1,
			"PolicyUploadRequests": 1,
			"PolicyUploadErrors":   1,
		}

		pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

	})
	t.Run("Test Multiple Invalid Policy Rotate", func(t *testing.T) {
		// Define the expected error string
		wantErrs := []string{"", "single upload request"}

		reqs := []*pathzpb.RotateRequest{{
			RotateRequest: &pathzpb.RotateRequest_UploadRequest{
				UploadRequest: &pathzpb.UploadRequest{
					Policy: &pathzpb.AuthorizationPolicy{
						Rules: []*pathzpb.AuthorizationRule{},
					},
				},
			},
		}, {
			RotateRequest: &pathzpb.RotateRequest_UploadRequest{
				UploadRequest: &pathzpb.UploadRequest{
					Policy: &pathzpb.AuthorizationPolicy{
						Rules: []*pathzpb.AuthorizationRule{},
					},
				},
			},
		}}
		client := start(t)
		rot, err := client.Rotate(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for i, req := range reqs {
			t.Logf("Request Sent: %s", req)
			if err := rot.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}
			received, err := rot.Recv()
			t.Logf("Received Request: %s", received)
			t.Logf("Rec Err %v", err)

			// Check if the error string matches the expected error string
			if d := errdiff.Check(err, wantErrs[i]); d != "" {
				t.Errorf("Rotate() unexpected err: %s", d)
			}
		}

		performOperations(t, dut)
		pathz.VerifyPolicyInfo(t, dut, 0, "", true)

		// Verify the pathz policy statistics.
		expectedStats := map[string]int{
			"PolicyRotations":      2,
			"PolicyRotationErrors": 2,
			"PolicyUploadRequests": 3,
			"PolicyUploadErrors":   2,
		}

		pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
	})
	t.Run("Test Finalize Without Policy Rotate", func(t *testing.T) {
		// Define the expected error string
		wantErrs := []string{"Finalize rotation called before upload request"}

		reqs :=
			[]*pathzpb.RotateRequest{{
				RotateRequest: &pathzpb.RotateRequest_FinalizeRotation{},
			}}

		client := start(t)
		rot, err := client.Rotate(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for i, req := range reqs {
			t.Logf("Request Sent: %s", req)
			if err := rot.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)

			}
			received, err := rot.Recv()
			t.Logf("Received Request: %s", received)
			t.Logf("Rec Err %v", err)

			// Check if the error string matches the expected error string
			if d := errdiff.Check(err, wantErrs[i]); d != "" {
				t.Errorf("Rotate() unexpected err: %s", d)
			}
		}

		performOperations(t, dut)
		pathz.VerifyPolicyInfo(t, dut, 0, "", true)

		// Verify the pathz policy statistics.
		expectedStats := map[string]int{
			"PolicyFinalize":       1,
			"PolicyFinalizeErrors": 1,
			"PolicyRotations":      3,
			"PolicyRotationErrors": 3,
			"PolicyUploadRequests": 3,
			"PolicyUploadErrors":   2,
		}

		pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
	})
	t.Run("Test Invalid Probe - User", func(t *testing.T) {
		// Define the expected error string
		wantErr := "user not specified"
		probeReq := &pathzpb.ProbeRequest{
			Mode:           pathzpb.Mode_MODE_READ,
			Path:           &gpb.Path{},
			PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
		}

		client := start(t)
		_, err := client.Probe(context.Background(), probeReq)
		t.Logf("Rec Err %v", err)

		// Check if the error string matches the expected error string
		if d := errdiff.Check(err, wantErr); d != "" {
			t.Fatalf("Probe() unexpected err: %s", d)
		}
		//Perform gNMI Operations.
		performOperations(t, dut)

		// Verify the policy info
		pathz.VerifyPolicyInfo(t, dut, 0, "", true)

		// Verify the pathz policy statistics.
		expectedStats := map[string]int{
			"ProbeRequests": 1,
			"ProbeErrors":   1,
		}

		pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

	})
	t.Run("Test Invalid Probe - Path", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			// Define the expected error string
			wantErr := "Nil Probe Request or Path"
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_READ,
				User:           d.sshUser,
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			client := start(t)
			_, err := client.Probe(context.Background(), probeReq)
			t.Logf("Rec Err %v", err)

			// Check if the error string matches the expected error string
			if d := errdiff.Check(err, wantErr); d != "" {
				t.Fatalf("Probe() unexpected err: %s", d)
			}
			//PerformgNMIOperations.
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Verify the pathz policy statistics.
			expectedStats := map[string]int{
				"ProbeRequests": 2,
				"ProbeErrors":   2,
			}

			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})
	t.Run("Test Invalid Probe - Instance", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			// Define the expected error string
			wantErr := "Unknown instance type"
			probeReq := &pathzpb.ProbeRequest{
				Mode: pathzpb.Mode_MODE_READ,
				User: d.sshUser,
				Path: &gpb.Path{},
			}

			client := start(t)
			_, err := client.Probe(context.Background(), probeReq)
			t.Logf("Rec Err %v", err)

			// Check if the error string matches the expected error string
			if d := errdiff.Check(err, wantErr); d != "" {
				t.Fatalf("Probe() unexpected err: %s", d)
			}
			// PerformgNMIOperations.
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Verify the pathz policy statistics.
			expectedStats := map[string]int{
				"ProbeRequests": 3,
				"ProbeErrors":   3,
			}

			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})
	t.Run("Test Probe Instance Without Policy Request", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {

			// Define the expected error string
			wantErr := "requested policy instance is nil"
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_READ,
				User:           d.sshUser,
				Path:           &gpb.Path{},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			client := start(t)
			_, err := client.Probe(context.Background(), probeReq)
			t.Logf("Rec Err %v", err)

			// Check if the error string matches the expected error string
			if d := errdiff.Check(err, wantErr); d != "" {
				t.Fatalf("Probe() unexpected err: %s", d)
			}
			//PerformgNMIOperations.
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Verify the pathz policy statistics.
			expectedStats := map[string]int{
				"ProbeRequests": 4,
				"ProbeErrors":   4,
			}

			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})
	t.Run("Test Invalid Probe Mode", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			// Define the expected error string
			wantErr := "mode not specified"

			probeReq := &pathzpb.ProbeRequest{
				User:           d.sshUser,
				Path:           &gpb.Path{},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			client := start(t)
			_, err := client.Probe(context.Background(), probeReq)
			t.Logf("Rec Err %v", err)

			// Check if the error string matches the expected error string
			if d := errdiff.Check(err, wantErr); d != "" {
				t.Fatalf("Probe() unexpected err: %s", d)
			}
			//PerformgNMIOperations.
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			expectedStats := map[string]int{
				"ProbeRequests": 5,
				"ProbeErrors":   5,
			}

			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})
	t.Run("Test Pathz Rule Path With Wildcard Entry", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Define expected response
			wantErr := "invalid policy: wildcard path names are not permitted"

			// Start gRPC client
			client := start(t)

			// Define rotate request
			req := &pathzpb.RotateRequest{
				RotateRequest: &pathzpb.RotateRequest_UploadRequest{
					UploadRequest: &pathzpb.UploadRequest{
						Version:   "1",
						CreatedOn: createdtime,
						Policy: &pathzpb.AuthorizationPolicy{
							Rules: []*pathzpb.AuthorizationRule{{
								Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "*"}, {Name: "hostname"}}},
								Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
								Mode:      pathzpb.Mode_MODE_WRITE,
								Action:    pathzpb.Action_ACTION_DENY,
							}},
						},
					},
				},
			}

			rot, err := client.Rotate(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			t.Logf("Rotate Req : %v", req)
			if err := rot.Send(req); err != nil {
				t.Fatalf("failed to send: %v", err)
			}

			resp, err := rot.Recv()
			t.Logf("Response Sent: %s", resp)
			t.Logf("Err: %s", err)

			if d := errdiff.Check(err, wantErr); d != "" {
				t.Fatalf("Rotate() unexpected err: %s", d)
			}

			// Perform gNMI operations
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Verify the pathz policy statistics.
			expectedStats := map[string]int{
				"PolicyFinalize":       1,
				"PolicyFinalizeErrors": 1,
				"PolicyRotations":      4,
				"PolicyRotationErrors": 4,
				"PolicyUploadRequests": 4,
				"PolicyUploadErrors":   3,
			}

			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})
	t.Run("Test Invalid Policy With Finalize", func(t *testing.T) {
		want := &pathzpb.GetResponse{
			Policy: &pathzpb.AuthorizationPolicy{
				Rules: []*pathzpb.AuthorizationRule{},
			},
		}

		req := &pathzpb.RotateRequest{
			RotateRequest: &pathzpb.RotateRequest_UploadRequest{
				UploadRequest: &pathzpb.UploadRequest{
					Policy: &pathzpb.AuthorizationPolicy{
						Rules: []*pathzpb.AuthorizationRule{},
					},
				},
			},
		}
		client := start(t)
		rot, err := client.Rotate(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("Request Sent: %s", req)
		if err := rot.Send(req); err != nil {
			t.Logf("Rec Err %v", err)
			t.Fatal(err)
		}

		// Perform GET operations for sandbox policy instance
		getSandboxResponse(t, want)

		// Finalize
		req = &pathzpb.RotateRequest{
			RotateRequest: &pathzpb.RotateRequest_FinalizeRotation{},
		}

		t.Logf("Request Sent: %s", req)
		if err := rot.Send(req); err != nil {
			t.Logf("Rec Err %v", err)
			t.Fatal(err)
		}

		// Perform GET operations for active policy instance.
		getReq := &pathzpb.GetRequest{
			PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
		}

		// Perform GET operations for active policy instance
		got, err := client.Get(context.Background(), getReq)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
			t.Fatalf("Pathz Get unexpected diff after finalize: %s", d)
		}

		// Perform gNMI operations
		isPermissionDeniedError(t, dut, "InvalidWithFinalize")

		// Verify the policy info
		pathz.VerifyPolicyInfo(t, dut, 0, "", false)

		// Verify the policy counters.
		pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
		pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

		// Verify the pathz policy statistics.
		expectedStats := map[string]int{
			"PolicyFinalize":       2,
			"PolicyFinalizeErrors": 1,
			"PolicyRotations":      5,
			"PolicyRotationErrors": 4,
			"PolicyUploadRequests": 5,
			"PolicyUploadErrors":   3,
			"GetRequests":          2,
			"GnmiPathLeaves":       1,
			"GnmiSetPathDeny":      3,
		}

		pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

		// Perform eMSD process restart
		t.Logf("Restarting emsd at %s", time.Now())
		perf.RestartProcess(t, dut, "emsd")
		t.Logf("Restart emsd finished at %s", time.Now())

		got, err = client.Get(context.Background(), getReq)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
			t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
		}

		// Perfrom gNMI Operations after process restart.
		isPermissionDeniedError(t, dut, "AfterProcessRestart")

		// Verify the policy info
		pathz.VerifyPolicyInfo(t, dut, 0, "", false)

		// Verify the policy counters.
		pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
		pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

		// Verify the pathz policy statistics after process restart.
		expectedStats = map[string]int{
			"NoPolicyAuthRequests": 0,
			"GnmiAuthorizations":   3,
			"GetRequests":          1,
			"GnmiPathLeaves":       1,
			"GnmiSetPathDeny":      3,
		}

		pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

		//Reload router
		perf.ReloadRouter(t, dut)

		client = start(t)
		got, err = client.Get(context.Background(), getReq)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
			t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
		}

		// Perfrom gNMI Operations after router reload.
		isPermissionDeniedError(t, dut, "AfterRouterReload")

		// // Verify the policy info
		pathz.VerifyPolicyInfo(t, dut, 0, "", false)

		// Verify the policy counters.
		pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
		pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

		// Verify the pathz policy statistics after router reload.
		expectedStats = map[string]int{
			"NoPolicyAuthRequests": 0,
			"GnmiAuthorizations":   3,
			"GetRequests":          1,
			"GnmiPathLeaves":       1,
			"GnmiSetPathDeny":      3,
		}
		pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
	})
	t.Run("Test Invalid Xpath With Finalize", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			want := &pathzpb.GetResponse{
				Version: "1",
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			req := &pathzpb.RotateRequest{
				RotateRequest: &pathzpb.RotateRequest_UploadRequest{
					UploadRequest: &pathzpb.UploadRequest{
						Version: "1",
						Policy: &pathzpb.AuthorizationPolicy{
							Rules: []*pathzpb.AuthorizationRule{{
								Path:      &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "hostname"}}},
								Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
								Mode:      pathzpb.Mode_MODE_WRITE,
								Action:    pathzpb.Action_ACTION_PERMIT,
							}},
						},
					},
				},
			}

			client := start(t)
			rot, err := client.Rotate(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			t.Logf("Request Sent: %s", req)
			if err := rot.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			// Perform GET operations for sandbox policy instance
			getSandboxResponse(t, want)

			// Finalize
			req = &pathzpb.RotateRequest{
				RotateRequest: &pathzpb.RotateRequest_FinalizeRotation{},
			}

			t.Logf("Request Sent: %s", req)
			if err := rot.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second)

			// Perform GET operations for active policy instance
			getReq := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			got, err := client.Get(context.Background(), getReq)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			t.Logf("GET Active Response : %v", got)
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after finalize: %s", d)
			}

			// Perfrom gNMI Operations.
			isPermissionDeniedError(t, dut, "InvalidXpathFinalize")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Verify the pathz policy statistics.
			expectedStats := map[string]int{
				"PolicyFinalize":       1,
				"PolicyFinalizeErrors": 0,
				"PolicyRotations":      1,
				"PolicyRotationErrors": 0,
				"PolicyUploadRequests": 1,
				"PolicyUploadErrors":   0,
				"NoPolicyAuthRequests": 0,
				"GnmiAuthorizations":   6,
				"GetRequests":          3,
				"GnmiPathLeaves":       1,
				"GnmiSetPathDeny":      6,
			}

			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Pathz GET after process restart
			got, err = client.Get(context.Background(), getReq)
			t.Logf("GET Active Response after Process Restart : %v", got)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed after process restart: %s", dut.Name())
			}

			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz.Get request is failed after process restart: %s", d)
			}

			// Perfrom gNMI Operations after process restart.
			isPermissionDeniedError(t, dut, "AfterProcessRestart")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Verify the pathz policy statistics after process restart.
			expectedStats = map[string]int{
				"PolicyFinalize":       0,
				"PolicyFinalizeErrors": 0,
				"PolicyRotations":      0,
				"PolicyRotationErrors": 0,
				"PolicyUploadRequests": 0,
				"PolicyUploadErrors":   0,
				"NoPolicyAuthRequests": 0,
				"GnmiAuthorizations":   3,
				"GetRequests":          1,
				"GnmiPathLeaves":       1,
				"GnmiSetPathDeny":      3,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})
	t.Run("Test Probe Request", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {

			// Define probe request
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}
			createdtime := uint64(time.Now().UnixMicro())

			// Define expected response
			want := &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_PERMIT,
			}

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_PERMIT,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err := client.Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform gNMI operations
			performOperations(t, dut)

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 4, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics.
			expectedStats := map[string]int{
				"PolicyFinalize":       1,
				"ProbeRequests":        1,
				"PolicyRotations":      1,
				"PolicyUploadRequests": 1,
				"GnmiAuthorizations":   7,
				"GetRequests":          3,
				"GnmiPathLeaves":       2,
				"GnmiSetPathDeny":      4,
				"GnmiSetPathPermit":    3,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart
			sand_res_after_process_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_process_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_process_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_process_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform gNMI operations after Process Restart.
			performOperations(t, dut)

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics.
			expectedStats = map[string]int{
				"PolicyFinalize":       0,
				"ProbeRequests":        0,
				"PolicyRotations":      0,
				"PolicyUploadRequests": 0,
				"GnmiAuthorizations":   4,
				"GetRequests":          2,
				"GnmiPathLeaves":       2,
				"GnmiSetPathDeny":      1,
				"GnmiSetPathPermit":    3,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform GET operations for sandbox policy instance after router reload
			client = start(t)
			sand_res_after_router_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_router_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Perform GET operations for active policy instance after router reload
			actv_res_after_router_reload, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_router_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Verify gNMI Operations after Router Reload
			performOperations(t, dut)

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_reload := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got_after_reload)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics.
			expectedStats = map[string]int{
				"PolicyFinalize":       0,
				"ProbeRequests":        0,
				"PolicyRotations":      0,
				"PolicyUploadRequests": 0,
				"GnmiAuthorizations":   4,
				"GetRequests":          2,
				"GnmiPathLeaves":       2,
				"GnmiSetPathDeny":      1,
				"GnmiSetPathPermit":    3,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})
	t.Run("Test Conflict Between Definite keys over wildcards keys With Triggers", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule3",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule4",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule5",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "config"},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule6",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule7",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule8",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule9",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule10",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule11",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule12",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "config"},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule13",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule14",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule3",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule4",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule5",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "config"},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule6",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule7",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule8",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule9",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule10",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule11",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule12",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "config"},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule13",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule14",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure Network Instance using gNMI.Update
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure Protcol ISIS using gNMI.Update
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Config(), &oc.NetworkInstance_Protocol{Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, Name: ygot.String("B4")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure ISIS overload bit using gNMI.Update
			config := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Delete ISIS overload bit using gNMI.Delete
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure ISIS overload bit using gNMI.Replace
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_process_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_process_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_process_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_process_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Configure Network Instance using gNMI.Update after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure Protcol ISIS using gNMI.Update after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Config(), &oc.NetworkInstance_Protocol{Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, Name: ygot.String("B4")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure ISIS overload bit using gNMI.Update after emsd process restart.

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Delete ISIS overload bit using gNMI.Delete after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure ISIS overload bit using gNMI.Replace after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform GET operations for sandbox policy instance after router reload.
			client = start(t)
			sand_res_after_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Perform GET operations for active policy instance after router reload.
			actv_res_after_reload, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Configure Network Instance using gNMI.Update after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure Protcol ISIS using gNMI.Update after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Config(), &oc.NetworkInstance_Protocol{Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, Name: ygot.String("B4")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure ISIS overload bit using gNMI.Update after router reload.

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Delete ISIS overload bit using gNMI.Delete after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure ISIS overload bit using gNMI.Replace after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
		}
	})
	t.Run("Test Conflict Between Definite keys over Wildcards Keys", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule3",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "openconfig-policy-types:ISIS", "name": "B4"}},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule4",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "openconfig-policy-types:ISIS", "name": "B4"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule5",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "openconfig-policy-types:ISIS", "name": "B4"}},
												{Name: "config"},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule6",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "openconfig-policy-types:ISIS", "name": "B4"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule7",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "openconfig-policy-types:ISIS", "name": "B4"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule8",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule9",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule10",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule11",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule12",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule13",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "config"},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule14",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule15",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule3",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "openconfig-policy-types:ISIS", "name": "B4"}},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule4",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "openconfig-policy-types:ISIS", "name": "B4"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule5",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "openconfig-policy-types:ISIS", "name": "B4"}},
									{Name: "config"},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule6",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "openconfig-policy-types:ISIS", "name": "B4"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule7",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "openconfig-policy-types:ISIS", "name": "B4"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule8",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule9",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule10",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule11",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule12",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule13",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "config"},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule14",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule15",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS Overwrite using gNMI.Update/Replace and Delete ISIS
			configISIS(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/config/name", true, true, 1, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, true, 3, 1)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, false, 0, 0)
		}
	})
	t.Run("Test Conflict Between Definite keys over Wildcards Keys - JSON", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule3",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule4",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule5",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "config"},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule6",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule7",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule8",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule9",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule10",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule11",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule12",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
												{Name: "config"},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule13",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule14",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule3",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule4",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule5",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "config"},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule6",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule7",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule8",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule9",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule10",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule11",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule12",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
									{Name: "config"},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule13",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule14",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure Network Instance using gNMI.Update
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})

			// Configure Protcol ISIS using gNMI.Update
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Config(), &oc.NetworkInstance_Protocol{Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, Name: ygot.String("B4")})

			// Configure ISIS overload bit using gNMI.Update
			config := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()
			gnmi.Update(t, dut, config.Config(), true)
			gnmi.Delete(t, dut, config.Config())
			gnmi.Replace(t, dut, config.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, true, 3, 4)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, false, 0, 0)
		}
	})
	t.Run("Test Conflict Group Over User - Invalid username in Group", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,

							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: "cafyauto1",
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: "cafyauto1",
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS using gNMI.Update
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Perform Rotate request
			rc, err = client.Rotate(context.Background())
			if err == nil {
				// Perform Rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "2",
							CreatedOn: createdtime,

							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}

				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_response := &pathzpb.GetResponse{
				Version:   "2",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sandbox := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_response, _ := client.Get(context.Background(), getReq_Sandbox)
			if d := cmp.Diff(get_response, sand_response, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Active := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_response, err := client.Get(context.Background(), getReq_Active)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_response, actv_response, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS using gNMI.Update
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "2", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, true, 3, 4)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, false, 0, 0)
		}
	})
	t.Run("Test Conflict Group Over User - Definite Keys Over Wildcard Keys", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,

							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule3",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule4",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule3",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule4",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS using gNMI.Update
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, true, 3, 4)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, false, 0, 0)
		}
	})
	t.Run("Test Conflict Group Over User - Deny/Permit", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			dut := ondatra.DUT(t, "dut")
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,

							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule3",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule4",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule3",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule4",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Configure ISIS using gNMI.Update
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, true, 3, 4)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, false, 0, 0)
		}
	})
	t.Run("Test Conflict Group Over User - Wildcard Keys", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,

							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule3",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule4",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule3",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule4",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Configure ISIS using gNMI.Update
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=*]/config/name", false, true, 0, 1)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=*]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=*]/name", false, true, 0, 1)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=*]/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, true, 3, 4)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, false, 0, 0)
		}
	})
	t.Run("Test Conflict Group Over User With Triggers", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,

							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule3",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule4",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule5",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule6",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule3",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule4",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule5",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule6",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Configure ISIS Overwrite using gNMI.Update/Replace and Delete ISIS
			configISIS(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=*]/config/name", false, true, 0, 1)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=*]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=*]/name", false, true, 0, 1)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=*]/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols", false, true, 0, 11)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, true, 3, 4)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=openconfig-policy-types:ISIS][name=B4]/name", false, false, 0, 0)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_emsd_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Configure ISIS using gNMI.Update after emsd restart.
			configISIS(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/config/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols", false, true, 0, 11)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols", false, false, 0, 0)

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform GET operations for sandbox policy instance after router reload.
			client = start(t)
			sand_res_after_router_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_router_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Perform GET operations for active policy instance after router reload.
			actv_res_after_router_reload, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_router_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Configure ISIS using gNMI.Update after router reload
			configISIS(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/config/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/name", false, true, 0, 2)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols", false, true, 0, 11)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols", false, false, 0, 0)
		}
	})
	t.Run("Test Pathz Policy with gNMI operation origin as Cli", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_DENY,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform gNMI operations
			isPermissionDeniedError(t, dut, "AfterFinalize")

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			gnmiwithcli(t, dut, updatePath, "hostname Origin-CLI-SF")
			gnmiwithcli(t, dut, deletePath, "no hostname")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_process_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_process_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_process_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_process_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Verify gNMI Operations after process restart.
			isPermissionDeniedError(t, dut, "AfterProcessRestart")

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_reload := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got_after_reload)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart ")
			}

			gnmiwithcli(t, dut, updatePath, "hostname Origin-CLI-SF")
			gnmiwithcli(t, dut, deletePath, "no hostname")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform GET operations for sandbox policy instance after router reload.
			client = start(t)
			sand_res_after_router_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_router_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance after router reload.
			actv_res_after_router_reload, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_router_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Verify gNMI Operations after router reload.
			isPermissionDeniedError(t, dut, "AfterRouterReload")

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_reload := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got_after_reload)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after router reload")
			}

			gnmiwithcli(t, dut, updatePath, "hostname Origin-CLI-SF")
			gnmiwithcli(t, dut, deletePath, "no hostname")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)
		}
	})
	t.Run("Test Pathz Policy Conflict Between Users", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,

							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto1"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto1"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS using gNMI.Update
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Perform Rotate request-2
			rc, err = client.Rotate(context.Background())
			if err == nil {
				// Perform Rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,

							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}

				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_response := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sandbox := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_response, _ := client.Get(context.Background(), getReq_Sandbox)
			if d := cmp.Diff(get_response, sand_response, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Active := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_response, err := client.Get(context.Background(), getReq_Active)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_response, actv_response, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS using gNMI.Update
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)
		}
	})
	t.Run("Test Pathz Policy Conflict Between Users with triggers", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS overload bit using gNMI.Update
			config := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Delete ISIS overload bit using gNMI.Delete
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure ISIS overload bit using gNMI.Replace
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_process_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_process_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_process_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_process_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Configure ISIS overload bit using gNMI.Update after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart ")
			}

			// Delete ISIS overload bit using gNMI.Delete after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart")
			}

			// Configure ISIS overload bit using gNMI.Replace after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", false, false, 0, 0)

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform GET operations for sandbox policy instance after router reload.
			client = start(t)
			sand_res_after_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			actv_res_after_reload, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Configure ISIS overload bit using gNMI.Update after router reload.

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after router reload")
			}

			// Delete ISIS overload bit using gNMI.Delete after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after router reload")
			}

			// Configure ISIS overload bit using gNMI.Replace after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after router reload")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", false, false, 0, 0)
		}
	})
	t.Run("Test Pathz Policy Longest Prefix Match Among Users", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS overload bit using gNMI.Update
			config := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()
			gnmi.Update(t, dut, config.Config(), true)
			gnmi.Delete(t, dut, config.Config())
			gnmi.Replace(t, dut, config.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
		}
	})
	t.Run("Test Pathz Policy Longest Prefix Match B/W Group & User", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS overload bit using gNMI.Update
			config := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()
			gnmi.Update(t, dut, config.Config(), true)
			gnmi.Delete(t, dut, config.Config())
			gnmi.Replace(t, dut, config.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, true, 0, 6)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
		}
	})
	t.Run("Test Pathz Policy Longest Prefix Among Groups", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}, {
									Name: "admin",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "admin"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}, {
						Name: "admin",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "admin"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS overload bit using gNMI.Update
			config := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed")
			}

			// Delete ISIS overload bit using gNMI.Delete
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed")
			}

			// Configure ISIS overload bit using gNMI.Replace
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, true, 3, 6)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
		}
	})
	t.Run("Test Pathz Policy Conflict Among Groups", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,

							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}, {
									Name: "admin",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "admin"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}, {
						Name: "admin",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "admin"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			t.Logf("Response : %v", sand_res)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS using gNMI.Update
			config := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed")
			}

			// Delete ISIS using gNMI.Delete
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed")
			}

			// Replace ISIS using gNMI.Replace
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)
			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, true, 3, 6)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_emsd_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Configure ISIS using gNMI.Update after process restart
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_emsd_restart := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got_after_emsd_restart)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart")
			}

			// Delete ISIS using gNMI.Delete after process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_emsd_restart := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got_after_emsd_restart)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart")
			}

			// Replace ISIS using gNMI.Replace after process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_emsd_restart := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got_after_emsd_restart)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]", false, false, 0, 0)

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform GET operations for sandbox policy instance after router reload.
			client = start(t)
			sand_res_after_router_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_router_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Perform GET operations for active policy instance after router reload.
			actv_res_after_router_reload, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_router_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Configure ISIS using gNMI.Update after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_emsd_restart := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got_after_emsd_restart)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after reouter reload")
			}

			// Delete ISIS using gNMI.Delete after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_emsd_restart := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got_after_emsd_restart)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after router reload")
			}

			// Replace ISIS using gNMI.Replace after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_emsd_restart := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got_after_emsd_restart)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after router reload")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]", false, false, 0, 0)
		}
	})
	t.Run("Test Pathz Policy File System Behaviour - Process Restart Emsd", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Define probe request
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			// Define expected response
			want := &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_DENY,
			}

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_PERMIT,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Rotate request 2
			rc, err = client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_DENY,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err := client.Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			t.Logf("Response : %v", sand_res)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform gNMI operations
			isPermissionDeniedError(t, dut, "Pathz_txt_bak")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz backup policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.bak")

			// Perform GET operations for sandbox policy instance after deleting pathz Backup file.
			sand_res_after_pathz_del, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz Backup file: %s", d)
			}

			// Perform GET operations for active policy instance after deleting pathz Backup file.
			actv_res_after_pathz_del, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz Backup file: %s", d)
			}

			// Verify gNMI SET Operations after deleting pathz backup policy.
			isPermissionDeniedError(t, dut, "AfterPathzPolicyDelete")

			// Get and store the result in portNum after deleting pathz Backup file.
			portNum_after_pathz_del := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_pathz_del == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after after deleting pathz Backup file: %v", portNum)
			}

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting bakup policy file.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform eMSD process restart after deleting Pathz backup file.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_emsd_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Verify gNMI SET Operations after process restart.
			isPermissionDeniedError(t, dut, "AfterProcessRestart")

			// Get and store the result in portNum after process restart.
			portNum_after_del_Pathzpolicy := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_del_Pathzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after process restart: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// Perform GET operations for sandbox policy instance after deleting pathz policy.
			sand_res_after_pathz_del, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Perform GET operations for active policy instance after deleting pathz policy.
			actv_res_after_pathz_del, err = client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Verify gNMI SET Operations after deleting pathz policy.
			isPermissionDeniedError(t, dut, "AfterPathzPolicyDelete")

			// Get and store the result in portNum after deleting pathz policy.
			portNum_after_del_Pathzpolicy = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_del_Pathzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting pathz policy: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz policy.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting pathz policy:")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting pathz policy.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform eMSD process restart after deleting Pathz file.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Actv)
			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Verify gNMI SET Operations after process restart.
			performOperations(t, dut)

			// Get and store the result in portNum after process restart.
			portNum_after_emsd_restart := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_emsd_restart == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after process restart: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after process restart.
			gnmi.Update(t, dut, path.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)
		}
	})
	t.Run("Test Pathz Policy with gNMI.SET Operation using XR Model", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			dut := ondatra.DUT(t, "dut")
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_PERMIT,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform gNMI operations
			performOperations(t, dut)

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// gNMI.SET Operation using XR Model
			stationMAC := "00:ba:ba:ba:ba:ba"
			configwithprefix(t, dut, replacePath, "native", stationMAC)
			configwithprefix(t, dut, updatePath, "native", stationMAC)
			configwithprefix(t, dut, deletePath, "native", stationMAC)

			hostname := "XR-Native"
			configwithoutprefix(t, dut, updatePath, hostname)
			configwithoutprefix(t, dut, replacePath, hostname)
			configwithoutprefix(t, dut, deletePath, hostname)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform GET operations for sandbox policy instance after router reload
			client = start(t)
			sand_res_after_router_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_router_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Perform GET operations for active policy instance after router reload
			actv_res_after_router_reload, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_router_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Verify gNMI Operations after Router Reload.
			performOperations(t, dut)

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_reload := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got_after_reload)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after reouter reload")
			}

			// gNMI.SET Operation using XR Model after router reload.
			configwithprefix(t, dut, replacePath, "native", stationMAC)
			configwithprefix(t, dut, updatePath, "native", stationMAC)
			configwithprefix(t, dut, deletePath, "native", stationMAC)

			configwithoutprefix(t, dut, updatePath, hostname)
			configwithoutprefix(t, dut, replacePath, hostname)
			configwithoutprefix(t, dut, deletePath, hostname)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after router reload
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart
			sand_res_after_process_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_process_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart
			actv_res_after_process_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_process_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Verify gNMI Operations after process restart.
			performOperations(t, dut)

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_process_restart := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got_after_process_restart)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart ")
			}

			// gNMI.SET Operation using XR Model after process restart.
			configwithprefix(t, dut, replacePath, "native", stationMAC)
			configwithprefix(t, dut, updatePath, "native", stationMAC)
			configwithprefix(t, dut, deletePath, "native", stationMAC)

			configwithoutprefix(t, dut, updatePath, hostname)
			configwithoutprefix(t, dut, replacePath, hostname)
			configwithoutprefix(t, dut, deletePath, hostname)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)
		}
	})
	t.Run("Test Pathz Policy File System Behaviour - Reload Router", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Define probe request
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			// Define expected response
			want := &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_DENY,
			}

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_PERMIT,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Rotate request 2
			rc, err = client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_DENY,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err := client.Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			t.Logf("Response : %v", sand_res)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform gNMI operations
			isPermissionDeniedError(t, dut, "Pathz_txt_bak")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, true, 3, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz backup policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// Perform GET operations for sandbox policy instance after deleting pathz backup policy.
			sand_res_after_pathz_del, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz backup policy: %s", d)
			}

			// Perform GET operations for active policy instance after deleting pathz backup policy.
			actv_res_after_pathz_del, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz backup policy: %s", d)
			}

			// Verify gNMI SET Operations after deleting pathz backup policy.
			isPermissionDeniedError(t, dut, "AfterPathzPolicyDelete")

			// Get and store the result in portNum after deleting pathz Backup file.
			portNum_after_pathz_del := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_pathz_del == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting pathz backup policy: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz Backup file.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting pathz backup policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting pathz policy.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, true, 6, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			get_res = &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			// Perform eMSD process restart after deleting Pathz backup file.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_emsd_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Verify gNMI SET Operations after process restart.
			performOperations(t, dut)

			// Get and store the result in portNum after process restart.
			portNum_after_del_Pathzpolicy := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_del_Pathzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after process restart: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz policy.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting pathz policy.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.bak")

			// Perform GET operations for sandbox policy instance after deleting pathz policy.
			sand_res_after_pathz_del, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance after deleting pathz policy.
			actv_res_after_pathz_del, err = client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Verify gNMI SET Operations after deleting pathz policy.
			performOperations(t, dut)

			// Get and store the result in portNum after deleting pathz policy.
			portNum_after_del_Pathzpolicy = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_del_Pathzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting pathz policy: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz policy.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting pathz policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting backup pathz policy.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 6)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			//Reload router after deleting Authz Policy & verify the behaviour.
			perf.ReloadRouter(t, dut)

			// Perform GET operations for sandbox policy instance after router reload.
			client = start(t)
			sand_res_after_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff bafter router reload: %s", d)
			}

			actv_res_after_reload, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Verify gNMI SET Operations after router reload.
			performOperations(t, dut)

			// Get and store the result in portNum after router reload.
			portNum_after_reload := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_reload == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after router reload: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after router reload")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// Perform GET operations for sandbox policy instance after deleting pathz policy.
			sand_res_after_pathz_del, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Perform GET operations for active policy instance after deleting pathz policy.
			actv_res_after_pathz_del, err = client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Verify gNMI SET Operations after deleting pathz policy.
			performOperations(t, dut)

			// Get and store the result in portNum after deleting pathz file.
			portNum_after_pathz_del = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_pathz_del == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting pathz policy: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz file.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting pathz policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting pathz policy.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 6)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform eMSD process restart after deleting Pathz file.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart
			sand_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", actv_res_after_reload)

			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Logf("gNMI Update : %v", d)
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Verify gNMI SET Operations after process restart.
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)
		}
	})
	t.Run("Test Pathz Policy File System Behaviour - Reload/ProcessRestart ", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Define probe request
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			// Define expected response
			want := &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_PERMIT,
			}

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}, {
									Name: "admin",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_DENY,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Rotate request 2
			rc, err = client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_PERMIT,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err := client.Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			t.Logf("Response : %v", sand_res)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Verify gNMI SET Operations.
			performOperations(t, dut)

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz backup policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// Perform GET operations for sandbox policy instance after deleting pathz backup policy.
			sand_res_after_pathz_del, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Errorf("Pathz Get unexpected diff deleting pathz backup policy: %s", d)
			}

			// Perform GET operations for active policy instance after deleting pathz backup policy.
			actv_res_after_pathz_del, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff deleting pathz backup policy: %s", d)
			}

			// Verify gNMI SET Operations after deleting pathz backup policy.
			performOperations(t, dut)

			// Get and store the result in portNum after deleting pathz backup policy.
			portNum_after_pathz_del := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_pathz_del == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting pathz backup policy: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz backup policy.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting pathz backup policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting pathz policy.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 6)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			get_res = &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}, {
						Name: "admin",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			// Perform eMSD process restart after deleting Pathz backup file.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_emsd_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform gNMI operations after process restart.
			isPermissionDeniedError(t, dut, "AfterProcessRestart")

			// Get and store the result in portNum after process restart.
			portNum_after_del_Pathzpolicy := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_del_Pathzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after process restart: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Backup Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.bak")

			// Perform GET operations for sandbox policy instance after deleting backup policy.
			sand_res_after_pathz_del, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting backup policy: %s", d)
			}

			// Perform GET operations for active policy instance after deleting backup policy.
			actv_res_after_pathz_del, err = client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting backup policy: %s", d)
			}

			// Perform gNMI operations
			isPermissionDeniedError(t, dut, "bak_pathz_delete")

			// Get and store the result in portNum after deleting backup pathz policy.
			portNum_after_del_Pathzpolicy = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_del_Pathzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting backup pathz policy: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after deleting backup pathz policy.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed deleting backup pathz policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting pathz backup policy.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform eMSD process restart after deleting Pathz backup file.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_emsd_restart, err = client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform gNMI operations
			isPermissionDeniedError(t, dut, "AfterProcessRestart")

			// Get and store the result in portNum deleting Authz Backup file.
			portNum_after_reload := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_reload == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after process restart: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath deleting Authz Backup file.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting pathz backup policy.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// Perform GET operations for sandbox policy instance after deleting pathz policy.
			sand_res_after_pathz_del, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Perform GET operations for active policy instance after deleting pathz policy.
			actv_res_after_pathz_del, err = client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Perform gNMI operations
			isPermissionDeniedError(t, dut, "pathz_delete")

			// Get and store the result in portNum after deleting pathz file.
			portNum_after_pathz_del = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_pathz_del == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting pathz policy: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz file.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting pathz policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting pathz backup policy.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform eMSD process restart after deleting Pathz file.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Got GET Response : %s", actv_res_after_emsd_restart)

			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Logf("GET Difference : %v", d)
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Verify gNMI SET Operations after process restart.
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)
		}
	})
	t.Run("Test Pathz Policy File System Behaviour - Pathz Rules Among Groups ", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {

			createdtime := uint64(time.Now().UnixMicro())

			// Define probe request
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			// Define expected response
			want := &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_DENY,
			}

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}, {
									Name: "admin",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_PERMIT,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Rotate request 2
			rc, err = client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}, {
									Name: "admin",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_Group{Group: "admin"},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_DENY,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err := client.Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}, {
						Name: "admin",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_Group{Group: "admin"},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			t.Logf("Response : %v", sand_res)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform gNMI operations.
			isPermissionDeniedError(t, dut, "Pathz_txt_bak")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz backup policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.bak")

			// Perform GET operations for sandbox policy instance after deleting pathz backup policy.
			sand_res_after_pathz_del, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz backup policy: %s", d)
			}

			// Perform GET operations for active policy instance after deleting pathz backup policy.
			actv_res_after_pathz_del, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz backup policy: %s", d)
			}

			// Verify gNMI SET Operations after deleting pathz backup policy.
			isPermissionDeniedError(t, dut, "AfterBakupPolicyDelete")

			// Get and store the result in portNum after deleting pathz Backup file.
			portNum_after_pathz_del := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_pathz_del == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting pathz Backup file: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz Backup file.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting pathz Backup file")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting backup pathz policy file.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			//Reload router after deleting Authz Policy & verify the behaviour.
			perf.ReloadRouter(t, dut)

			// Perform GET operations for sandbox policy instance after router reload.
			client = start(t)
			sand_res_after_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Perform GET operations for active policy instance after router reload.
			actv_res_after_reload, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Verify gNMI SET Operations after after router reload.
			isPermissionDeniedError(t, dut, "AfterRouterReload")

			// Get and store the result in portNum after router reload.
			portNum_after_del_Pathzpolicy := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_del_Pathzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after router reload: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after router reload")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// Perform GET operations for sandbox policy instance after deleting Pathz policy.
			sand_res_after_pathz_del, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting Pathz policy: %s", d)
			}

			// Perform GET operations for active policy instance after deleting Pathz policy.
			actv_res_after_pathz_del, err = client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting Pathz policy: %s", d)
			}

			// Verify gNMI SET Operations after deleting Authz policy.
			isPermissionDeniedError(t, dut, "AfterPathzDelete")

			// Get and store the result in portNum after process restart.
			portNum_after_del_Pathzpolicy = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_del_Pathzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting Pathz policy: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz policy.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting Pathz policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting pathz policy.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform eMSD process restart after deleting Pathz file.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after router_reload.
			actv_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Actv)
			t.Logf("Got GET Response : %s", actv_res_after_emsd_restart)
			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Logf("GET Difference : %v", d)
				t.Fatalf("Pathz Get unexpected diff after router_reload: %s", d)
			}

			// Verify gNMI SET Operations after router_reload.
			performOperations(t, dut)

			// Get and store the result in portNum deleting Authz Backup file.
			portNum_after_emsd_restart := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_emsd_restart == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after router_reload: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath deleting Authz Backup file.
			gnmi.Update(t, dut, path.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)
		}
	})
	t.Run("Test Corrupt Pathz Policy File Behaviour - ProcessRestart/Reload", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Start gRPC client
			client := start(t)

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_PERMIT,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Rotate request 2
			rc, err = client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_DENY,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "Deny_Rule")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			pathzRulesPath := "testdata/invalid_policy.txt"
			copyPathzRules := "/mnt/rdsfs/ems/gnsi"

			target := fmt.Sprintf("%s:%v", d.sshIp, d.sshPort)
			t.Logf("Copying Pathz rules file to %s (%s) over scp", d.dut, target)
			sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
			scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
			if err != nil {
				t.Fatalf("Error initializing scp client: %v", err)
			}
			defer scpClient.Close()

			resp := scpClient.CopyFileToRemote(pathzRulesPath, copyPathzRules, &scp.FileTransferOption{})
			t.Logf("copying file got %v", resp)
			if resp == nil || strings.Contains(resp.Error(), "Function not implemented") {
				t.Logf("SCP successful: File copied successfully")
			} else {
				t.Fatalf("SCP attempt failed: %s", resp.Error())
			}

			time.Sleep(10 * time.Second)

			// Move the invalid_policy.txt to pathz_policy.txt
			cliHandle := dut.RawAPIs().CLI(t)
			_, err = cliHandle.RunCommand(context.Background(), "run mv /mnt/rdsfs/ems/gnsi/invalid_policy.txt /mnt/rdsfs/ems/gnsi/pathz_policy.txt")
			time.Sleep(10 * time.Second)
			if err != nil {
				t.Error(err)
			}

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "Expecting_deny")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after corrupting policy file.
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Reload router
			perf.ReloadRouter(t, dut)

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			// Perform GET operations for active policy instance
			client = start(t)
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after corrupting pathz file: %s", d)
			}

			// Verify gNMI Operations after router reload.
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after corrupting policy file.
			pathz.VerifyWritePolicyCounters(t, dut, "/", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Copy invalid policy file to DUT
			scpClient, err = scp.NewClient(target, sshConf, &scp.ClientOption{})
			if err != nil {
				t.Fatalf("Error initializing scp client: %v", err)
			}
			resp = scpClient.CopyFileToRemote(pathzRulesPath, copyPathzRules, &scp.FileTransferOption{})
			t.Logf("copying file got %v", resp)
			if resp == nil || strings.Contains(resp.Error(), "Function not implemented") {
				t.Logf("SCP successful: File copied successfully")
			} else {
				t.Fatalf("SCP attempt failed: %s", resp.Error())
			}

			time.Sleep(10 * time.Second)

			// Move the invalid_policy.txt to pathz_policy.txt
			cliHandle = dut.RawAPIs().CLI(t)
			_, err = cliHandle.RunCommand(context.Background(), "run mv /mnt/rdsfs/ems/gnsi/invalid_policy.txt /mnt/rdsfs/ems/gnsi/pathz_policy.txt")
			time.Sleep(10 * time.Second)
			if err != nil {
				t.Error(err)
			}

			actv_res, err = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after corrupting pathz file: %s", d)
			}

			time.Sleep(10 * time.Second)

			// Perform eMSD process restart after router reload.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after corrupting pathz file: %s", d)
			}

			// Verify gNMI Operations after corrupting pathz policy file.
			isPermissionDeniedError(t, dut, "corrupt_files]")

			// Get and store the result in portNum after corrupting pathz policy file.
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			timestamp := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyCreatedOn().State())
			t.Logf("Got the expected Policy timestamp: %v", timestamp)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, timestamp, "Cisco-Deny-All-Bad-File-Encoding", false)

			// Verify the policy counters after corrupting policy file.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for active policy instance after deleting pathz policy.
			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz file: %s", d)
			}

			// Verify gNMI Operations after corrupting pathz policy file.
			isPermissionDeniedError(t, dut, "after_del_pathz_txt")

			// Get and store the result in portNum after corrupting pathz policy file.
			portNum = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			timestamp = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyCreatedOn().State())
			t.Logf("Got the expected Policy timestamp: %v", timestamp)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, timestamp, "Cisco-Deny-All-Bad-File-Encoding", false)

			// Verify the policy counters after corrupting policy file.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.bak")

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform GET operations for active policy instance after deleting backup pathz policy.
			client = start(t)
			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting backup pathz file: %s", d)
			}

			// Verify gNMI Operations after deleting backup pathz file.
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)
		}
	})
	t.Run("Scale Pathz Policy Rules with Router Reload", func(t *testing.T) {
		// Pathz Rules Scale Test (5800 Pathz Rules) with router reload.
		for _, d := range parseBindingFile(t) {

			// Perform eMSD process restart before capturing intial emsd process memory.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Get the initial emsd memory usage
			intial_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Initial emsd memory usage: %v", intial_emsd_memory)

			// Initialize the verifier
			verifier := pathz.NewVerifier()

			// Sample memory usage before the operation
			verifier.SampleBefore(t, dut)

			pathzRulesPath := "testdata/pathz_policy.txt"
			copyPathzRules := "/mnt/rdsfs/ems/gnsi"

			target := fmt.Sprintf("%s:%v", d.sshIp, d.sshPort)
			t.Logf("Copying Pathz rules file to %s (%s) over scp", d.dut, target)
			sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
			scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
			if err != nil {
				t.Fatalf("Error initializing scp client: %v", err)
			}
			defer scpClient.Close()

			// Copy Pathz policy file to DUT
			resp := scpClient.CopyFileToRemote(pathzRulesPath, copyPathzRules, &scp.FileTransferOption{})
			t.Logf("copying file got %v", resp)
			if resp == nil || strings.Contains(resp.Error(), "Function not implemented") {
				t.Logf("SCP successful: File copied successfully")
			} else {
				t.Fatalf("SCP attempt failed: %s", resp.Error())
			}

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform GET operations for active policy instance after process restart.
			client := start(t)
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_rule")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			time.Sleep(10 * time.Second)

			// Sample memory usage after the operation
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// guarantee a few timestamps before reload.
			time.Sleep(10 * time.Second)

			// Reload router after deleting pathz policy file.
			perf.ReloadRouter(t, dut)

			// guarantee a few timestamps to settle memory.
			time.Sleep(300 * time.Second)

			// Perform GET operations for active policy instance after process restart.
			client = start(t)
			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)

			if actv_res != nil {
				t.Fatalf("Pathz Get request is failed on device %s", dut.Name())
			}

			// Sample memory usage after Deleting Pathz policy file.
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Verify gNMI Operations after Deleting Pathz policy file.
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Get the final emsd memory usage
			final_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Final emsd memory usage: %v", final_emsd_memory)
		}
	})
	t.Run("Scale Pathz Policy Rules with ProcessRestart", func(t *testing.T) {
		// Pathz Rules Scale Test (5800 Pathz Rules) with process restart.
		for _, d := range parseBindingFile(t) {

			// Perform eMSD process restart before capturing intial emsd process memory.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Get the initial emsd memory usage
			initial_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Initial emsd memory usage: %v", initial_emsd_memory)

			// Initialize the verifier
			verifier := pathz.NewVerifier()

			// Sample memory usage before the operation
			verifier.SampleBefore(t, dut)

			// Start gRPC client
			client := start(t)

			pathzRulesPath := "testdata/pathz_policy.txt"
			copyPathzRules := "/mnt/rdsfs/ems/gnsi"

			target := fmt.Sprintf("%s:%v", d.sshIp, d.sshPort)
			t.Logf("Copying Pathz rules file to %s (%s) over scp", d.dut, target)
			sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
			scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
			if err != nil {
				t.Fatalf("Error initializing scp client: %v", err)
			}
			defer scpClient.Close()

			// Copy Pathz policy file to DUT
			resp := scpClient.CopyFileToRemote(pathzRulesPath, copyPathzRules, &scp.FileTransferOption{})
			t.Logf("copying file got %v", resp)
			if resp == nil || strings.Contains(resp.Error(), "Function not implemented") {
				t.Logf("SCP successful: File copied successfully")
			} else {
				t.Fatalf("SCP attempt failed: %s", resp.Error())
			}

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Perform eMSD process restart.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for active policy instance after process restart.
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_rule")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 1714456775238852, "5800-Rules", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			time.Sleep(10 * time.Second)

			// Sample memory usage after the operation
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Perform eMSD process restart.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			time.Sleep(30 * time.Second)

			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)

			if actv_res != nil {
				t.Fatalf("Pathz Get request is failed on device %s", dut.Name())
			}

			// Sample memory usage after Deleting Pathz policy file.
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Verify gNMI Operations after Deleting Pathz policy file.
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Get the final emsd memory usage
			final_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Final emsd memory usage: %v", final_emsd_memory)
		}
	})
	t.Run("Scale Pathz Policy Rules file & gNMI SET Request with Emsd Restart", func(t *testing.T) {
		// Pathz Rules Scale Test (5800 Pathz Rules) with gNMI SET Scale operations and eMSD Restart.
		for _, d := range parseBindingFile(t) {
			dut := ondatra.DUT(t, "dut")

			// Function to check the platform status
			Resp := pathz.CheckPlatformStatus(t, dut)
			if Resp != nil {
				fmt.Printf("Error: %v\n", Resp)
			} else {
				fmt.Println("All CPU0 entries are in 'IOS XR RUN' state.")
			}

			// Perform eMSD process restart before capturing intial emsd process memory.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			batchSet, leavesCnt := pathz.GenerateSubInterfaceConfig(t, dut)
			t.Logf("configuration %v :", batchSet)
			t.Logf("Leaves count %v :", leavesCnt)

			// Initialize the verifier
			verifier := pathz.NewVerifier()

			// Sample memory usage before the operation
			verifier.SampleBefore(t, dut)

			// Get the initial emsd memory usage
			intial_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Initial emsd memory usage: %v", intial_emsd_memory)

			pathzRulesPath := "testdata/pathz_policy.txt"
			copyPathzRules := "/mnt/rdsfs/ems/gnsi"

			target := fmt.Sprintf("%s:%v", d.sshIp, d.sshPort)
			t.Logf("Copying Pathz rules file to %s (%s) over scp", d.dut, target)
			sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
			scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
			if err != nil {
				t.Fatalf("Error initializing scp client: %v", err)
			}
			defer scpClient.Close()

			// Copy Pathz policy file to DUT
			resp := scpClient.CopyFileToRemote(pathzRulesPath, copyPathzRules, &scp.FileTransferOption{})
			t.Logf("copying file got %v", resp)
			if resp == nil || strings.Contains(resp.Error(), "Function not implemented") {
				t.Logf("SCP successful: File copied successfully")
			} else {
				t.Fatalf("SCP attempt failed: %s", resp.Error())
			}

			// Perform a gNMI Set Request with 5 MB of Data
			set := perf.CreateInterfaceSetFromOCRoot(util.LoadJsonFileToOC(t, "testdata/set_config.json"), true)

			t.Logf("Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for active policy instance after process restart.
			client := start(t)
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_xpath")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform a gNMI Set Request with 5 MB of Data
			set = perf.CreateInterfaceSetFromOCRoot(util.LoadJsonFileToOC(t, "testdata/set_config.json"), true)

			t.Logf("After process restart:Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("After process restart:Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 1714456775238852, "5800-Rules", false)

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			time.Sleep(10 * time.Second)

			// Sample memory usage after the operation
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Perform GET operations for active policy instance after process restart.
			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)

			if actv_res != nil {
				t.Fatalf("Pathz Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations after emsd process restart.
			performOperations(t, dut)

			// Get and store the result in portNum
			portNum = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Sample memory usage after Deleting Pathz policy file.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after d
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed after deleting pathz_policy.txt")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// cleanup subinterfaces configs
			pathz.CleanUPInterface(t, dut)

			// Sample memory usage after removing gNMI Set Request with 19 MB.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after removing gNMI Set Request with 19 MB.
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed removing gNMI Set Request with 19 MB.")
			}
			// Check top CPU utilization after removing gNMI Set Request with 19 MB.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Get the final emsd memory usage
			final_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Final emsd memory usage: %v", final_emsd_memory)
		}
	})
	t.Run("Scale Pathz Policy Rules file & gNMI SET Request with Router Reload", func(t *testing.T) {
		// Pathz Rules Scale Test (5800 Pathz Rules) with gNMI SET Scale operations and router reload.
		for _, d := range parseBindingFile(t) {

			// Perform eMSD process restart before capturing intial emsd process memory.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			batchSet, leavesCnt := pathz.GenerateSubInterfaceConfig(t, dut)
			t.Logf("configuration %v :", batchSet)
			t.Logf("Leaves count %v :", leavesCnt)

			// Initialize the verifier
			verifier := pathz.NewVerifier()

			// Sample memory usage before the operation
			verifier.SampleBefore(t, dut)

			// Get the initial emsd memory usage
			intial_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Initial emsd memory usage: %v", intial_emsd_memory)

			pathzRulesPath := "testdata/pathz_policy.txt"
			copyPathzRules := "/mnt/rdsfs/ems/gnsi"

			target := fmt.Sprintf("%s:%v", d.sshIp, d.sshPort)
			t.Logf("Copying Pathz rules file to %s (%s) over scp", d.dut, target)
			sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
			scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
			if err != nil {
				t.Fatalf("Error initializing scp client: %v", err)
			}
			defer scpClient.Close()

			// Copy Pathz policy file to DUT
			resp := scpClient.CopyFileToRemote(pathzRulesPath, copyPathzRules, &scp.FileTransferOption{})
			t.Logf("copying file got %v", resp)
			if resp == nil || strings.Contains(resp.Error(), "Function not implemented") {
				t.Logf("SCP successful: File copied successfully")
			} else {
				t.Fatalf("SCP attempt failed: %s", resp.Error())
			}

			// Perform a gNMI Set Request with 5 MB of Data
			set := perf.CreateInterfaceSetFromOCRoot(util.LoadJsonFileToOC(t, "testdata/set_config.json"), true)

			t.Logf("Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			//Reload router
			perf.ReloadRouter(t, dut)

			// Function to check the platform status
			Resp := pathz.CheckPlatformStatus(t, dut)
			if Resp != nil {
				fmt.Printf("Error: %v\n", Resp)
			} else {
				fmt.Println("All CPU0 entries are in 'IOS XR RUN' state.")
			}

			// Perform GET operations for active policy instance after router reload.
			client := start(t)

			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			time.Sleep(10 * time.Second)

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			t.Logf("Error Received : %v", err)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_xpath")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform a gNMI Set Request with 5 MB of Data
			set = perf.CreateInterfaceSetFromOCRoot(util.LoadJsonFileToOC(t, "testdata/set_config.json"), true)

			t.Logf("After router reload:Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("After router reload:Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 1714456775238852, "5800-Rules", false)

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			time.Sleep(10 * time.Second)

			// Sample memory usage after the operation
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			//Reload router
			perf.ReloadRouter(t, dut)

			// Function to check the platform status
			Resp = pathz.CheckPlatformStatus(t, dut)
			if Resp != nil {
				fmt.Printf("Error: %v\n", Resp)
			} else {
				fmt.Println("All CPU0 entries are in 'IOS XR RUN' state.")
			}

			// Perform GET operations for active policy instance after router reload.
			client = start(t)
			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)

			if actv_res != nil {
				t.Fatalf("Pathz Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations after router reload.
			performOperations(t, dut)

			// Get and store the result in portNum
			portNum = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Sample memory usage after Deleting Pathz policy file.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after d
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed after deleting pathz_policy.txt")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// cleanup subinterfaces configs
			pathz.CleanUPInterface(t, dut)

			// Sample memory usage after removing gNMI Set Request with 19 MB.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after removing gNMI Set Request with 19 MB.
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed removing gNMI Set Request with 19 MB.")
			}
			// Check top CPU utilization after removing gNMI Set Request with 19 MB.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Get the final emsd memory usage
			final_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Final emsd memory usage: %v", final_emsd_memory)
		}
	})
	t.Run("Scale Pathz Policy Rules Request & gNMI SET Request with Emsd Restart", func(t *testing.T) {
		// Authz Pathz Test with Router Reload.
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			fileName := "testdata/pathz_path.txt"

			// Start gRPC client
			client := start(t)

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Perform eMSD process restart before capturing intial emsd process memory.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			batchSet, leavesCnt := pathz.GenerateSubInterfaceConfig(t, dut)
			t.Logf("configuration %v :", batchSet)
			t.Logf("Leaves count %v :", leavesCnt)

			// Initialize the verifier
			verifier := pathz.NewVerifier()

			// Sample memory usage before the operation
			verifier.SampleBefore(t, dut)

			// Get the initial emsd memory usage
			intial_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Initial emsd memory usage: %v", intial_emsd_memory)

			// Rotate Request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				req := pathz.GenerateRules(fileName, "openconfig", d.sshUser, createdtime)
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform GET operations for active policy instance after process restart.
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_rule")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform a gNMI Set Request with 5 MB of Data
			set := perf.CreateInterfaceSetFromOCRoot(util.LoadJsonFileToOC(t, "testdata/set_config.json"), true)

			t.Logf("After process restart:Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("After process restart:Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "5800-Rules", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			time.Sleep(10 * time.Second)

			// Sample memory usage after the operation
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			time.Sleep(10 * time.Second)

			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_xpath")

			// Get and store the result in portNum
			portNum = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform a gNMI Set Request with 5 MB of Data
			set = perf.CreateInterfaceSetFromOCRoot(util.LoadJsonFileToOC(t, "testdata/set_config.json"), true)

			t.Logf("After process restart:Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("After process restart:Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "5800-Rules", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Sample memory usage after the operation
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Perform GET operations for active policy instance after process restart.
			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			t.Logf("Error Received : %v", err)

			if actv_res != nil {
				t.Fatalf("Pathz Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations after emsd process restart.
			performOperations(t, dut)

			// Get and store the result in portNum
			portNum = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Sample memory usage after Deleting Pathz policy file.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after d
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed after deleting pathz_policy.txt")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// cleanup subinterfaces configs
			pathz.CleanUPInterface(t, dut)

			// Sample memory usage after removing gNMI Set Request with 19 MB.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after removing gNMI Set Request with 19 MB.
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed removing gNMI Set Request with 19 MB.")
			}
			// Check top CPU utilization after removing gNMI Set Request with 19 MB.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Get the final emsd memory usage
			final_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Final emsd memory usage: %v", final_emsd_memory)
		}
	})
	t.Run("Scale Pathz Policy Rules Request & gNMI SET Request with Router Reload", func(t *testing.T) {
		// Pathz Scale Test with Router Reload.
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			fileName := "testdata/pathz_path.txt"

			// Start gRPC client
			client := start(t)

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Perform eMSD process restart before capturing intial emsd process memory.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			batchSet, leavesCnt := pathz.GenerateSubInterfaceConfig(t, dut)
			t.Logf("configuration %v :", batchSet)
			t.Logf("Leaves count %v :", leavesCnt)

			// Initialize the verifier
			verifier := pathz.NewVerifier()

			// Sample memory usage before the operation
			verifier.SampleBefore(t, dut)

			// Get the initial emsd memory usage
			intial_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Initial emsd memory usage: %v", intial_emsd_memory)

			// Rotate Request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				req := pathz.GenerateRules(fileName, "openconfig", d.sshUser, createdtime)
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform GET operations for active policy instance after process restart.
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_rule")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform a gNMI Set Request with 5 MB of Data
			set := perf.CreateInterfaceSetFromOCRoot(util.LoadJsonFileToOC(t, "testdata/set_config.json"), true)

			t.Logf("After process restart:Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("After process restart:Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "5800-Rules", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			time.Sleep(10 * time.Second)

			// Sample memory usage after the operation
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			time.Sleep(10 * time.Second)

			// Reload router & verify the behaviour.
			perf.ReloadRouter(t, dut)

			// Function to check the platform status
			Resp := pathz.CheckPlatformStatus(t, dut)
			if Resp != nil {
				fmt.Printf("Error: %v\n", Resp)
			} else {
				fmt.Println("All CPU0 entries are in 'IOS XR RUN' state.")
			}

			client = start(t)
			actv_res, err = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_xpath")

			// Get and store the result in portNum
			portNum = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform a gNMI Set Request with 5 MB of Data
			set = perf.CreateInterfaceSetFromOCRoot(util.LoadJsonFileToOC(t, "testdata/set_config.json"), true)

			t.Logf("After process restart:Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("After process restart:Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "5800-Rules", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Sample memory usage after the operation
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Reload router after deleting Pathz Policy & verify the behaviour.
			perf.ReloadRouter(t, dut)
			client = start(t)

			// Function to check the platform status
			Resp = pathz.CheckPlatformStatus(t, dut)
			if Resp != nil {
				fmt.Printf("Error: %v\n", Resp)
			} else {
				fmt.Println("All CPU0 entries are in 'IOS XR RUN' state.")
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)

			if actv_res != nil {
				t.Fatalf("Pathz Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations after emsd process restart.
			performOperations(t, dut)

			// Get and store the result in portNum
			portNum = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Sample memory usage after Deleting Pathz policy file.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after d
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed after deleting pathz_policy.txt")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// cleanup subinterfaces configs
			pathz.CleanUPInterface(t, dut)

			// Sample memory usage after removing gNMI Set Request with 19 MB.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after removing gNMI Set Request with 19 MB.
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed removing gNMI Set Request with 5 MB.")
			}
			// Check top CPU utilization after removing gNMI Set Request with 19 MB.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Get the final emsd memory usage
			final_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Final emsd memory usage: %v", final_emsd_memory)
		}
	})
	t.Run("Authz Pathz Test with Router Reload", func(t *testing.T) {
		// Authz Pathz Test with Router Reload.
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			policyMap := authz.LoadPolicyFromJSONFile(t, "testdata/policy.json")

			// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
			newpolicy, ok := policyMap["policy-gNMI-set"]
			if !ok {
				t.Fatal("policy-gNMI-set is not loaded from policy json file")
			}
			newpolicy.AddAllowRules("base", []string{d.sshUser}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
			// Rotate the policy.
			newpolicy.Rotate(t, dut, createdtime, "policy-gNMI-set", false)

			// Define probe request
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			// Define expected response
			want := &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_DENY,
			}

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_DENY,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err := client.Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			t.Logf("Response : %v", sand_res)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform gNMI operations
			isPermissionDeniedError(t, dut, "AuthzPathz")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Delete Authz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "authz_policy.txt")

			// Perform GET operations for active policy instance after deleting authz policy.
			sand_res_after_del_Authzpolicy, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_del_Authzpolicy, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting authz policy: %s", d)
			}

			actv_res_after_del_Authzpolicy, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_del_Authzpolicy, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting authz policy: %s", d)
			}

			// Verify gNMI SET Operations after deleting Authz policy.
			isPermissionDeniedError(t, dut, "AfterAuthzPolicyDelete")

			// Get and store the result in portNum after deleting authz policy.
			portNum_after_del_Authzpolicy := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_del_Authzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting authz policy: %v", portNum_after_del_Authzpolicy)
			}

			// Verify gNMI SET Operation for different xpath deleting Authz policy.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting authz policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Perform eMSD process restart after deleting Authz Backup file.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_emsd_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Verify gNMI SET Operations after after process restart.
			isPermissionDeniedError(t, dut, "AfterProcessRestart")

			// Get and store the result in portNum deleting Authz Backup file.
			portNum_after_emsd_restart := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_emsd_restart == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after process restart: %v", portNum_after_emsd_restart)
			}

			// Verify gNMI SET Operation for different xpath deleting Authz Backup file.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.bak")

			// Perform GET operations for sandbox policy instance after deleting pathz policy.
			sand_res_after_pathz_del, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Perform GET operations for active policy instance after deleting pathz policy.
			actv_res_after_pathz_del, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Verify gNMI SET Operations after deleting backup pathz policy.
			isPermissionDeniedError(t, dut, "AfterBakupPolicyDelete")

			// Get and store the result in portNum after deleting pathz policy.
			portNum_after_pathz_del := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_pathz_del == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting pathz policy: %v", portNum_after_pathz_del)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz policy.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting pathz policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting backup pathz policy.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform eMSD process restart after deleting Pathz file.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for active policy instance after process restart.
			sand_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("GOT Response: %v", actv_res_after_emsd_restart)
			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d != "" {
				t.Logf("GET Difference : %v", d)
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Verify gNMI SET Operations after proces restart.
			isPermissionDeniedError(t, dut, "AfterProcessRestart")

			// Get and store the result in portNum deleting after process restart.
			portNum_after_router_reload := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_router_reload == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after process restart: %v", portNum_after_router_reload)
			}

			// Verify gNMI SET Operation for different xpath after process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after Router reload")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// Perform GET operations for sandbox policy instance after deleting pathz policy.
			sand_res_after_pathz_del, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Perform GET operations for active policy instance after deleting pathz policy.
			actv_res_after_pathz_del, err = client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Verify gNMI SET Operations after deleting pathz policy.
			isPermissionDeniedError(t, dut, "AfterPathzPolicyDelete")

			// Get and store the result in portNum after deleting pathz policy.
			portNum_after_del_Pathzpolicy := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_del_Pathzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting pathz policy: %v", portNum_after_del_Pathzpolicy)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz policy.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting pathz policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting backup pathz policy.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Reload router after deleting Authz Policy & verify the behaviour.
			perf.ReloadRouter(t, dut)
			client = start(t)

			// Function to check the platform status
			Resp := pathz.CheckPlatformStatus(t, dut)
			if Resp != nil {
				fmt.Printf("Error: %v\n", Resp)
			} else {
				fmt.Println("All CPU0 entries are in 'IOS XR RUN' state.")
			}

			// Perform GET operations for sandbox policy instance after router reload.
			sand_res_after_router_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_router_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Perform GET operations for active policy instance after router reload.
			actv_res_after_router_reload, _ := client.Get(context.Background(), getReq_Actv)
			t.Logf("GOT Response: %v", actv_res_after_router_reload)
			if d := cmp.Diff(get_res, actv_res_after_router_reload, protocmp.Transform()); d == "" {
				t.Logf("GET Difference : %v", d)
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Verify gNMI SET Operations after router reload.
			performOperations(t, dut)

			// Get and store the result in portNum after router reload.
			portNum_after_router_reload = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_router_reload == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after router reload: %v", portNum_after_router_reload)
			}

			// Verify gNMI SET Operation for different xpath after router reload
			gnmi.Update(t, dut, path.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)
		}
	})
}
func TestRPSO_Pathz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Perform eMSD process restart
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartProcess(t, dut, "emsd")
	t.Logf("Restart emsd finished at %s", time.Now())

	t.Run("RPSO: Test Pathz Probe Request with RPSO", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {

			// Define probe request
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}
			createdtime := uint64(time.Now().UnixMicro())

			// Define expected response
			want := &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_DENY,
			}

			// Declare probeBeforeFinalize
			probeBeforeFinalize := true

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_DENY,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err := client.Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)
			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}
			t.Logf("GET sandbox Request : %v", getReq_Sand)

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}
			t.Logf("GET Sandbox Response : %v", sand_res)

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}
			t.Logf("GET Active Request : %v", getReq_Actv)

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}
			t.Logf("GET Active Response : %v", actv_res)
			t.Logf("GET Active Response Error: %v", err)

			// Perform gNMI operations
			performOperations(t, dut)

			// // Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Verify the pathz policy statistics.
			expectedStats := map[string]int{
				"PolicyRotations":      1,
				"PolicyUploadRequests": 1,
				"GetRequests":          2,
				"ProbeRequests":        1,
				"GetErrors":            1,
				"ProbeErrors":          0,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err = client.Probe(context.Background(), probeReq)
			t.Logf("Probe Error : %v", err)
			if got != nil {
				t.Fatalf("Probe() unexpected response: %v", got)
			}

			// Perform GET operations for sandbox policy instance after process restart
			sand_res_after_process_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_process_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}
			t.Logf("GET Sandbox Response after process restart: %v", sand_res_after_process_restart)

			// Perform GET operations for active policy instance after process restart
			actv_res_after_process_restart, _ := client.Get(context.Background(), getReq_Actv)
			if d := cmp.Diff(get_res, actv_res_after_process_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}
			t.Logf("GET Active Response after process restart: %v", sand_res_after_process_restart)

			// Perform gNMI operations after process restart
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Verify the pathz policy statistics after process restart.
			expectedStats = map[string]int{
				"PolicyRotations":      0,
				"PolicyUploadRequests": 0,
				"GetRequests":          2,
				"ProbeRequests":        1,
				"GetErrors":            2,
				"ProbeErrors":          1,
			}

			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			client = start(t)
			got, err = client.Probe(context.Background(), probeReq)
			t.Logf("Probe Error : %v", err)
			if got != nil {
				t.Fatalf("Probe() unexpected response: %v", got)
			}

			// Perform GET operations for sandbox policy instance after RP Switchover.
			sand_res_after_RP_Switchover, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_RP_Switchover, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP Switchover: %s", d)
			}
			t.Logf("GET Sandbox Response after RP Switchover: %v", sand_res_after_RP_Switchover)

			// Perform GET operations for active policy instance after after_RP_Switchover
			actv_res_after_after_RP_Switchover, _ := client.Get(context.Background(), getReq_Actv)
			if d := cmp.Diff(get_res, actv_res_after_after_RP_Switchover, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after after_RP_Switchover: %s", d)
			}
			t.Logf("GET Active Response after after_RP_Switchover: %v", sand_res_after_RP_Switchover)

			// Perform gNMI operations after after RP Switchover
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Verify the pathz policy statistics after RP Switchover
			expectedStats = map[string]int{
				"PolicyRotations":      0,
				"PolicyUploadRequests": 0,
				"GetRequests":          2,
				"ProbeRequests":        1,
				"GetErrors":            2,
				"ProbeErrors":          1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})
	t.Run("RPSO: Test Pathz Invalid Policy with RPSO", func(t *testing.T) {
		want := &pathzpb.GetResponse{
			Policy: &pathzpb.AuthorizationPolicy{
				Rules: []*pathzpb.AuthorizationRule{},
			},
		}

		req := &pathzpb.RotateRequest{
			RotateRequest: &pathzpb.RotateRequest_UploadRequest{
				UploadRequest: &pathzpb.UploadRequest{
					Policy: &pathzpb.AuthorizationPolicy{
						Rules: []*pathzpb.AuthorizationRule{},
					},
				},
			},
		}

		client := start(t)
		rot, err := client.Rotate(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("Request Sent: %s", req)
		if err := rot.Send(req); err != nil {
			t.Logf("Rec Err %v", err)
			t.Fatal(err)
		}

		// Perform GET operations for sandbox policy instance
		getSandboxResponse(t, want)

		// Finalize
		req = &pathzpb.RotateRequest{
			RotateRequest: &pathzpb.RotateRequest_FinalizeRotation{},
		}

		t.Logf("Request Sent: %s", req)
		if err := rot.Send(req); err != nil {
			t.Logf("Rec Err %v", err)
			t.Fatal(err)
		}

		time.Sleep(5 * time.Second)

		// Perform GET operations for active policy instance.
		getReq := &pathzpb.GetRequest{
			PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
		}

		// Perform GET operations for active policy instance
		got, err := client.Get(context.Background(), getReq)
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
			t.Fatalf("Pathz Get unexpected diff after finalize: %s", d)
		}

		// Perform gNMI operations
		isPermissionDeniedError(t, dut, "InvalidWithFinalize")

		// Verify the policy info
		pathz.VerifyPolicyInfo(t, dut, 0, "", false)

		// Verify the policy counters.
		pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
		pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

		// Verify the pathz policy statistics.
		expectedStats := map[string]int{
			"PolicyRotations":      1,
			"PolicyFinalize":       1,
			"PolicyUploadRequests": 1,
			"NoPolicyAuthRequests": 3,
			"GnmiAuthorizations":   6,
			"GetRequests":          4,
		}
		pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

		// Perform RP Switchover
		utils.Dorpfo(context.Background(), t, true)

		// Perform GET operations for active policy instance.
		client = start(t)
		got, err = client.Get(context.Background(), getReq)
		t.Logf("Rec Err %v", err)
		t.Logf("got response %v", got)

		if err != nil {
			t.Fatal(err)
		}

		if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
			t.Fatalf("Pathz Get unexpected diff after RP Switchover: %s", d)
		}

		// Perfrom gNMI Operations after process restart.
		isPermissionDeniedError(t, dut, "AfterRPSwitchover")

		// Verify the policy info
		pathz.VerifyPolicyInfo(t, dut, 0, "", false)

		// Verify the policy counters after RP Switchover.
		pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
		pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

		// Verify the pathz policy statistics after RP Switchover
		expectedStats = map[string]int{
			"PolicyRotations":      0,
			"PolicyFinalize":       0,
			"PolicyUploadRequests": 0,
			"NoPolicyAuthRequests": 0,
			"GnmiAuthorizations":   3,
			"GetRequests":          1,
			"GnmiSetPathDeny":      3,
			"GnmiPathLeaves":       1,
		}
		pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
	})
	t.Run("RPSO: Test Pathz Invalid Xpath with RPSO", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			want := &pathzpb.GetResponse{
				Version: "1",
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			req := &pathzpb.RotateRequest{
				RotateRequest: &pathzpb.RotateRequest_UploadRequest{
					UploadRequest: &pathzpb.UploadRequest{
						Version: "1",
						Policy: &pathzpb.AuthorizationPolicy{
							Rules: []*pathzpb.AuthorizationRule{{
								Path:      &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "hostname"}}},
								Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
								Mode:      pathzpb.Mode_MODE_WRITE,
								Action:    pathzpb.Action_ACTION_PERMIT,
							}},
						},
					},
				},
			}

			client := start(t)
			rot, err := client.Rotate(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			t.Logf("Request Sent: %s", req)
			if err := rot.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			// Perform GET operations for sandbox policy instance
			getSandboxResponse(t, want)

			// Finalize
			req = &pathzpb.RotateRequest{
				RotateRequest: &pathzpb.RotateRequest_FinalizeRotation{},
			}

			t.Logf("Request Sent: %s", req)
			if err := rot.Send(req); err != nil {
				t.Logf("Rec Err %v", err)
				t.Fatal(err)
			}

			time.Sleep(5 * time.Second)

			// Perform GET operations for active policy instance
			getReq := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			got, err := client.Get(context.Background(), getReq)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			t.Logf("GET Active Response : %v", got)
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after finalize: %s", d)
			}

			// Perfrom gNMI Operations.
			isPermissionDeniedError(t, dut, "InvalidXpathFinalize")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Verify the pathz policy statistics.
			expectedStats := map[string]int{
				"PolicyRotations":      1,
				"PolicyFinalize":       1,
				"PolicyUploadRequests": 1,
				"GnmiAuthorizations":   6,
				"GetRequests":          3,
				"GnmiSetPathDeny":      6,
				"GnmiPathLeaves":       1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Pathz GET after RP Switchover
			client = start(t)
			got, err = client.Get(context.Background(), getReq)
			t.Logf("GET Active Response after RP Switchover : %v", got)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed after RP Switchover: %s", dut.Name())
			}

			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz.Get request is failed after RP Switchover: %s", d)
			}

			// Perfrom gNMI Operations after RP Switchover.
			isPermissionDeniedError(t, dut, "AfterProcessRestart")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "1", false)

			// Verify the policy counters after RP Switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Verify the pathz policy statistics after RP Switchover.
			expectedStats = map[string]int{
				"PolicyRotations":      0,
				"PolicyFinalize":       0,
				"PolicyUploadRequests": 0,
				"GnmiAuthorizations":   3,
				"GetRequests":          1,
				"GnmiSetPathDeny":      3,
				"GnmiPathLeaves":       1,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})
	t.Run("RPSO: Test Pathz Policy Finalize", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {

			// Define probe request
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}
			createdtime := uint64(time.Now().UnixMicro())

			// Define expected response
			want := &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_PERMIT,
			}

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_PERMIT,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err := client.Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform gNMI operations
			performOperations(t, dut)

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 4, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics.
			expectedStats := map[string]int{
				"PolicyRotations":      1,
				"PolicyFinalize":       1,
				"PolicyUploadRequests": 1,
				"GnmiAuthorizations":   7,
				"GetRequests":          3,
				"GnmiSetPathDeny":      4,
				"GnmiPathLeaves":       2,
				"ProbeRequests":        1,
				"GnmiSetPathPermit":    3,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for sandbox policy instance after RP Switchover
			client = start(t)
			sand_res_after_RP_Switchover, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_RP_Switchover, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP Switchover: %s", d)
			}

			// Perform GET operations for active policy instance after RP Switchover.
			actv_res_after_RP_Switchover, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_RP_Switchover, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after RP Switchover: %s", d)
			}

			// Perform gNMI operations after RP Switchover.
			performOperations(t, dut)

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after RP Switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics after RP Switchover.
			expectedStats = map[string]int{
				"PolicyRotations":      0,
				"PolicyFinalize":       0,
				"PolicyUploadRequests": 0,
				"GnmiAuthorizations":   4,
				"GetRequests":          2,
				"GnmiSetPathDeny":      1,
				"GnmiPathLeaves":       2,
				"ProbeRequests":        0,
				"GnmiSetPathPermit":    3,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform GET operations for sandbox policy instance after router reload
			client = start(t)
			sand_res_after_router_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_router_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Perform GET operations for active policy instance after router reload
			actv_res_after_router_reload, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_router_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Verify gNMI Operations after Router Reload
			performOperations(t, dut)

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_reload := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got_after_reload)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Verify the pathz policy statistics after Router reload.
			expectedStats = map[string]int{
				"PolicyRotations":      0,
				"PolicyFinalize":       0,
				"PolicyUploadRequests": 0,
				"GnmiAuthorizations":   4,
				"GetRequests":          2,
				"GnmiSetPathDeny":      1,
				"GnmiPathLeaves":       2,
				"ProbeRequests":        0,
				"GnmiSetPathPermit":    3,
			}
			pathz.ValidateGnsiPathAuthStats(t, dut, expectedStats)
		}
	})
	t.Run("RPSO: Test Pathz Policy Conflict B/W Group & User", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule3",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule4",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule5",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "config"},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule6",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule7",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule8",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule9",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule10",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule11",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule12",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "config"},
												{Name: "identifier"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule13",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule14",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule3",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule4",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule5",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "config"},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule6",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule7",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule8",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule9",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule10",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule11",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule12",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "config"},
									{Name: "identifier"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule13",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule14",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "*", "name": "*"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure Network Instance using gNMI.Update
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure Protcol ISIS using gNMI.Update
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Config(), &oc.NetworkInstance_Protocol{Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, Name: ygot.String("B4")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure ISIS overload bit using gNMI.Update
			config := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Delete ISIS overload bit using gNMI.Delete
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure ISIS overload bit using gNMI.Replace
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/config/identifier", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/config/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for sandbox policy instance after RP_Switchover.
			client = start(t)
			sand_res_after_RP_Switchover, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_RP_Switchover, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Perform GET operations for active policy instance after RP_Switchover.
			actv_res_after_RP_Switchover, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_RP_Switchover, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Configure Network Instance using gNMI.Update after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed RP_Switchover")
			}

			// Configure Protcol ISIS using gNMI.Update after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Config(), &oc.NetworkInstance_Protocol{Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, Name: ygot.String("B4")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed RP_Switchover")
			}

			// Configure ISIS overload bit using gNMI.Update after router reload.

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed RP_Switchover")
			}

			// Delete ISIS overload bit using gNMI.Delete after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed RP_Switchover")
			}

			// Configure ISIS overload bit using gNMI.Replace after router reload.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed RP_Switchover")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after RP Switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/config/identifier", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/config/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_process_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_process_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_process_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_process_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Configure Network Instance using gNMI.Update after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure Protcol ISIS using gNMI.Update after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Config(), &oc.NetworkInstance_Protocol{Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, Name: ygot.String("B4")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure ISIS overload bit using gNMI.Update after emsd process restart.

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Delete ISIS overload bit using gNMI.Delete after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Configure ISIS overload bit using gNMI.Replace after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after RP Switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/config/identifier", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/config/identifier", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
		}
	})
	t.Run("RPSO: Test Pathz Policy with Invalid User", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,

							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: "cafyauto1",
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: "cafyauto1",
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS using gNMI.Update
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform Rotate request after RP_Switchover
			client = start(t)
			rc, err = client.Rotate(context.Background())
			if err == nil {
				// Perform Rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,

							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "config"},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig-legacy",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "name"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}

				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_response := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "config"},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig-legacy",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "name"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance after RP_Switchover.
			getReq_Sandbox := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_response, _ := client.Get(context.Background(), getReq_Sandbox)
			if d := cmp.Diff(get_response, sand_response, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance after RP_Switchover.
			getReq_Active := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_response, err := client.Get(context.Background(), getReq_Active)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_response, actv_response, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Configure ISIS using gNMI.Update after RP_Switchover.
			gnmi.Update(t, dut, gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Config(), &oc.NetworkInstance{Name: ygot.String("DEFAULT")})

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after RP Switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/config/name", false, true, 0, 1)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/name", false, true, 0, 1)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/name", false, false, 0, 0)
		}
	})
	t.Run("RPSO: Test Pathz Policy with Wildcard Keys", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "*"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "*"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "*"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS overload bit using gNMI.Update
			config := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Delete ISIS overload bit using gNMI.Delete
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after RP_Switchover ")
			}

			// Configure ISIS overload bit using gNMI.Replace
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/config/name", false, true, 0, 1)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/config/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/name", false, true, 0, 1)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/name", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", false, false, 0, 0)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for sandbox policy instance after RP_Switchover.
			client = start(t)
			sand_res_after_RPFO, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_RPFO, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			actv_res_after_RPFO, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_RPFO, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Configure ISIS overload bit using gNMI.Update after RP_Switchover.

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after RP_Switchover")
			}

			// Delete ISIS overload bit using gNMI.Delete after RP_Switchover.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after RP_Switchover")
			}

			// Configure ISIS overload bit using gNMI.Replace after RP_Switchover.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after RP_Switchover")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after RP Switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", false, false, 0, 0)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_process_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_process_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_process_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_process_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Configure ISIS overload bit using gNMI.Update after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart ")
			}

			// Delete ISIS overload bit using gNMI.Delete after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart")
			}

			// Configure ISIS overload bit using gNMI.Replace after emsd process restart.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", false, false, 0, 0)
		}
	})
	t.Run("RPSO: Test Pathz Policy with gNMI operation origin as Cli", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_DENY,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform gNMI operations
			isPermissionDeniedError(t, dut, "AfterFinalize")

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			gnmiwithcli(t, dut, updatePath, "hostname Origin-CLI-SF")
			gnmiwithcli(t, dut, deletePath, "no hostname")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=*]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for sandbox policy instance after RP_Switchover.
			client = start(t)
			sand_res_after_RP_Switchover, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_RP_Switchover, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Perform GET operations for active policy instance after RP_Switchover.
			actv_res_after_RP_Switchover, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_RP_Switchover, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Verify gNMI Operations after process restart.
			isPermissionDeniedError(t, dut, "AfterRP_Switchover")

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_RP_Switchover := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got_after_RP_Switchover)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after RP_Switchover")
			}

			gnmiwithcli(t, dut, updatePath, "hostname Origin-CLI-SF")
			gnmiwithcli(t, dut, deletePath, "no hostname")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after RP switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform GET operations for sandbox policy instance after router reload.
			client = start(t)
			sand_res_after_router_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_router_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance after router reload.
			actv_res_after_router_reload, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_router_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Verify gNMI Operations after router reload.
			isPermissionDeniedError(t, dut, "AfterRouterReload")

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_reload := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got_after_reload)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after router reload")
			}

			gnmiwithcli(t, dut, updatePath, "hostname Origin-CLI-SF")
			gnmiwithcli(t, dut, deletePath, "no hostname")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)
		}
	})
	t.Run("RPSO: Test Pathz Policy Longest Prefix", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Groups: []*pathzpb.Group{{
									Name: "pathz",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}, {
									Name: "admin",
									Users: []*pathzpb.User{
										{
											Name: d.sshUser,
										},
									},
								}},
								Rules: []*pathzpb.AuthorizationRule{
									{
										Id: "Rule1",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
												{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
												{Name: "isis"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_DENY,
									},
									{
										Id: "Rule2",
										Path: &gpb.Path{
											Origin: "openconfig",
											Elem: []*gpb.PathElem{
												{Name: "network-instances"},
												{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
												{Name: "protocols"},
											},
										},
										Principal: &pathzpb.AuthorizationRule_Group{Group: "admin"},
										Mode:      pathzpb.Mode_MODE_WRITE,
										Action:    pathzpb.Action_ACTION_PERMIT,
									},
								},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Groups: []*pathzpb.Group{{
						Name: "pathz",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}, {
						Name: "admin",
						Users: []*pathzpb.User{
							{
								Name: d.sshUser,
							},
						},
					}},
					Rules: []*pathzpb.AuthorizationRule{
						{
							Id: "Rule1",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
									{Name: "protocol", Key: map[string]string{"identifier": "ISIS", "name": "B4"}},
									{Name: "isis"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "pathz"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_DENY,
						},
						{
							Id: "Rule2",
							Path: &gpb.Path{
								Origin: "openconfig",
								Elem: []*gpb.PathElem{
									{Name: "network-instances"},
									{Name: "network-instance", Key: map[string]string{"name": "DEFAULT"}},
									{Name: "protocols"},
								},
							},
							Principal: &pathzpb.AuthorizationRule_Group{Group: "admin"},
							Mode:      pathzpb.Mode_MODE_WRITE,
							Action:    pathzpb.Action_ACTION_PERMIT,
						},
					},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Configure ISIS overload bit using gNMI.Update
			config := gnmi.OC().NetworkInstance(*ciscoFlags.DefaultNetworkInstance).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Global().LspBit().OverloadBit().SetBit()

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed")
			}

			// Delete ISIS overload bit using gNMI.Delete
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed")
			}

			// Configure ISIS overload bit using gNMI.Replace
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for sandbox policy instance after RP_Switchover
			client = start(t)
			getReq_Sand = &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Perform GET operations for active policy instance after RP_Switchover
			getReq_Actv = &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err = client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Configure ISIS overload bit using gNMI.Update after RP_Switchover
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after RP_Switchover")
			}

			// Delete ISIS overload bit using gNMI.Delete after RP_Switchover
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Delete(t, dut, config.Config())
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after RP_Switchover")
			}

			// Configure ISIS overload bit using gNMI.Replace after RP_Switchover
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Replace(t, dut, config.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after RP_Switchover")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after RP Switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
		}
	})
	t.Run("RPSO: Test Pathz Policy with gNMI.SET Operation using XR Model", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_PERMIT,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff: %s", d)
			}

			// Perform gNMI operations
			performOperations(t, dut)

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// gNMI.SET Operation using XR Model
			stationMAC := "00:ba:ba:ba:ba:ba"
			configwithprefix(t, dut, replacePath, "native", stationMAC)
			configwithprefix(t, dut, updatePath, "native", stationMAC)
			configwithprefix(t, dut, deletePath, "native", stationMAC)

			hostname := "XR-Native"
			configwithoutprefix(t, dut, updatePath, hostname)
			configwithoutprefix(t, dut, replacePath, hostname)
			configwithoutprefix(t, dut, deletePath, hostname)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/network-instances/network-instance[name=DEFAULT]/protocols/protocol[identifier=ISIS][name=B4]/isis", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for sandbox policy instance after RP Switchover.
			client = start(t)
			sand_res_after_RP_Switchover, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_RP_Switchover, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Perform GET operations for active policy instance after RP_Switchover.
			actv_res_after_RP_Switchover, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_RP_Switchover, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Verify gNMI Operations after RP_Switchover.
			performOperations(t, dut)

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_RPFO := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got_after_RPFO)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after RP_Switchover")
			}

			// gNMI.SET Operation using XR Model after RP_Switchover.
			configwithprefix(t, dut, replacePath, "native", stationMAC)
			configwithprefix(t, dut, updatePath, "native", stationMAC)
			configwithprefix(t, dut, deletePath, "native", stationMAC)

			configwithoutprefix(t, dut, updatePath, hostname)
			configwithoutprefix(t, dut, replacePath, hostname)
			configwithoutprefix(t, dut, deletePath, hostname)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after RP Switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform eMSD process restart
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart
			sand_res_after_process_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_process_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart
			actv_res_after_process_restart, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_process_restart, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Verify gNMI Operations after process restart.
			performOperations(t, dut)

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got_after_process_restart := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got_after_process_restart)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after process restart ")
			}

			// gNMI.SET Operation using XR Model after process restart.
			configwithprefix(t, dut, replacePath, "native", stationMAC)
			configwithprefix(t, dut, updatePath, "native", stationMAC)
			configwithprefix(t, dut, deletePath, "native", stationMAC)

			configwithoutprefix(t, dut, updatePath, hostname)
			configwithoutprefix(t, dut, replacePath, hostname)
			configwithoutprefix(t, dut, deletePath, hostname)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)
		}
	})
	t.Run("RPSO: Test Deleting Pathz Policy File", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {

			// Define probe request
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}
			createdtime := uint64(time.Now().UnixMicro())

			// Define expected response
			want := &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_DENY,
			}

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_DENY,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err := client.Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
			}

			// Perform gNMI operations
			isPermissionDeniedError(t, dut, "Deny")

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, true, 3, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.bak")

			// Perform gNMI operations
			isPermissionDeniedError(t, dut, "AfterPathzBakDelete")

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, true, 6, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for sandbox policy instance after RP Switchover
			client = start(t)
			sand_res_after_RP_Switchover, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_RP_Switchover, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP Switchover: %s", d)
			}

			// Perform GET operations for active policy instance after RP Switchover.
			actv_res_after_RP_Switchover, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_RP_Switchover, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after RP Switchover: %s", d)
			}

			// Perform gNMI operations after RP Switchover.
			isPermissionDeniedError(t, dut, "AfterRPFO")

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after RP Switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// Perform gNMI operations after RP Switchover.
			isPermissionDeniedError(t, dut, "AfterPathzDelete")

			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed ")
			}

			// 	// Verify the policy counters after RP Switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for sandbox policy instance after router reload
			client = start(t)
			sand_res_after_RPSwitch, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_RPSwitch, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Perform GET operations for active policy instance after router reload
			actv_res_after_RPSwitch, _ := client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %s", actv_res_after_RPSwitch)
			if d := cmp.Diff(get_res, actv_res_after_RPSwitch, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Verify gNMI Operations after Router Reload
			performOperations(t, dut)

			resp := gnmi.Update(t, dut, path.Config(), true)
			t.Logf("gNMI Update : %v", resp)
			if resp == nil {
				t.Fatalf("gNMI Update failed")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)
		}
	})
	t.Run("RPSO: Test Corrupt Pathz Policy File", func(t *testing.T) {
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			pathzRulesPath := "testdata/invalid_policy.txt"
			copyPathzRules := "/mnt/rdsfs/ems/gnsi"

			// Start gRPC client
			client := start(t)

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_PERMIT,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Rotate request 2
			rc, err = client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_DENY,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "Deny_Rule")

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters.
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// SCP Client
			target := fmt.Sprintf("%s:%v", d.sshIp, d.sshPort)
			t.Logf("Copying Pathz rules file to %s (%s) over scp", d.dut, target)
			sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
			scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
			if err != nil {
				t.Fatalf("Error initializing scp client: %v", err)
			}
			defer scpClient.Close()

			time.Sleep(10 * time.Second)

			// Copy invalid policy file to DUT
			resp := scpClient.CopyFileToRemote(pathzRulesPath, copyPathzRules, &scp.FileTransferOption{})
			t.Logf("copying file got %v", resp)
			if resp == nil || strings.Contains(resp.Error(), "Function not implemented") {
				t.Logf("SCP successful: File copied successfully")
			} else {
				t.Fatalf("SCP attempt failed: %s", resp.Error())
			}

			time.Sleep(10 * time.Second)

			// Move the invalid_policy.txt to pathz_policy.txt
			cliHandle := dut.RawAPIs().CLI(t)
			_, err = cliHandle.RunCommand(context.Background(), "run mv /mnt/rdsfs/ems/gnsi/invalid_policy.txt /mnt/rdsfs/ems/gnsi/pathz_policy.txt")
			time.Sleep(30 * time.Second)
			if err != nil {
				t.Error(err)
			}

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "Expecting_deny")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after RP switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			// Perform GET operations for active policy instance
			client = start(t)
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after corrupting pathz file: %s", d)
			}

			// Verify gNMI Operations after corrupting file.
			performOperations(t, dut)

			// Copy invalid policy file to DUT
			scpClient, err = scp.NewClient(target, sshConf, &scp.ClientOption{})
			if err != nil {
				t.Fatalf("Error initializing scp client: %v", err)
			}

			resp = scpClient.CopyFileToRemote(pathzRulesPath, copyPathzRules, &scp.FileTransferOption{})
			t.Logf("copying file got %v", resp)
			if resp == nil || strings.Contains(resp.Error(), "Function not implemented") {
				t.Logf("SCP successful: File copied successfully")
			} else {
				t.Fatalf("SCP attempt failed: %s", resp.Error())
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after RP switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			time.Sleep(10 * time.Second)

			// Move the invalid_policy.txt to pathz_policy.txt
			cliHandle = dut.RawAPIs().CLI(t)
			_, err = cliHandle.RunCommand(context.Background(), "run mv /mnt/rdsfs/ems/gnsi/invalid_policy.txt /mnt/rdsfs/ems/gnsi/pathz_policy.txt")
			time.Sleep(10 * time.Second)
			if err != nil {
				t.Error(err)
			}

			actv_res, err = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after corrupting pathz file: %s", d)
			}

			time.Sleep(10 * time.Second)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after corrupting pathz file: %s", d)
			}

			// Verify gNMI Operations after corrupting pathz policy file.
			isPermissionDeniedError(t, dut, "corrupt_files")

			// Get and store the result in portNum after corrupting pathz policy file.
			portNum = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			timestamp := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyCreatedOn().State())
			t.Logf("Got the expected Policy timestamp: %v", timestamp)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, timestamp, "Cisco-Deny-All-Bad-File-Encoding", false)

			// Verify the policy counters after RP switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for active policy instance after deleting pathz policy.
			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz file: %s", d)
			}

			// Verify gNMI Operations after corrupting pathz policy file.
			isPermissionDeniedError(t, dut, "after_del_pathz_txt")

			// Get and store the result in portNum after corrupting pathz policy file.
			portNum = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			timestamp = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyCreatedOn().State())
			t.Logf("Got the expected Policy timestamp: %v", timestamp)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, timestamp, "Cisco-Deny-All-Bad-File-Encoding", false)

			// Verify the policy counters after RP switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.bak")

			// Reload router
			perf.ReloadRouter(t, dut)

			// Perform GET operations for active policy instance after deleting backup pathz policy.
			client = start(t)
			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting backup pathz file: %s", d)
			}

			// Verify gNMI Operations after deleting backup pathz file.
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)
		}
	})
	t.Run("RPSO: Test Pathz Policy File with 5800 Pathz Rules", func(t *testing.T) {
		// Pathz Rules Scale Test (5800 Pathz Rules) with RP Failover.
		for _, d := range parseBindingFile(t) {

			// Perform eMSD process restart before capturing intial emsd process memory.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Get the initial emsd memory usage
			intial_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Initial emsd memory usage: %v", intial_emsd_memory)

			// Initialize the verifier
			verifier := pathz.NewVerifier()

			// Sample memory usage before the operation
			verifier.SampleBefore(t, dut)

			pathzRulesPath := "testdata/pathz_policy.txt"
			copyPathzRules := "/mnt/rdsfs/ems/gnsi"

			target := fmt.Sprintf("%s:%v", d.sshIp, d.sshPort)
			t.Logf("Copying Pathz rules file to %s (%s) over scp", d.dut, target)
			sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
			scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
			if err != nil {
				t.Fatalf("Error initializing scp client: %v", err)
			}
			defer scpClient.Close()

			// Copy invalid policy file to DUT
			resp := scpClient.CopyFileToRemote(pathzRulesPath, copyPathzRules, &scp.FileTransferOption{})
			t.Logf("copying file got %v", resp)
			if resp == nil || strings.Contains(resp.Error(), "Function not implemented") {
				t.Logf("SCP successful: File copied successfully")
			} else {
				t.Fatalf("SCP attempt failed: %s", resp.Error())
			}

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for active policy instance after process restart.
			client := start(t)
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_rule")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 1714456775238852, "5800-Rules", false)

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			time.Sleep(10 * time.Second)

			// Sample memory usage after the operation
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Perform RP Switchover after deleting pathz policy file.
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for active policy instance after process restart.
			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)

			if actv_res != nil {
				t.Fatalf("Pathz Get request is failed on device %s", dut.Name())
			}

			// Sample memory usage after after process restart.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after process restart.
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization after process restart.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Perform eMSD process restart .
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Verify gNMI Operations after after process restart.
			performOperations(t, dut)

			// Verify the policy info after process restart.
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Get the final emsd memory usage
			final_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Final emsd memory usage: %v", final_emsd_memory)
		}
	})
	t.Run("RPSO: Test Pathz Policy File with 5800 Pathz Rules file and gNMI Scale Operations", func(t *testing.T) {
		// Pathz Rules Scale Test (5800 Pathz Rules) with gNMI SET Scale operations and RP Failover.
		for _, d := range parseBindingFile(t) {

			// Function to check the platform status
			Resp := pathz.CheckPlatformStatus(t, dut)
			if Resp != nil {
				fmt.Printf("Error: %v\n", Resp)
			} else {
				fmt.Println("All CPU0 entries are in 'IOS XR RUN' state.")
			}

			// Perform eMSD process restart before capturing intial emsd process memory.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			batchSet, leavesCnt := pathz.GenerateSubInterfaceConfig(t, dut)
			t.Logf("configuration %v :", batchSet)
			t.Logf("Leaves count %v :", leavesCnt)

			// Initialize the verifier
			verifier := pathz.NewVerifier()

			// Sample memory usage before the operation
			verifier.SampleBefore(t, dut)

			// Get the initial emsd memory usage
			intial_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Initial emsd memory usage: %v", intial_emsd_memory)

			pathzRulesPath := "testdata/pathz_policy.txt"
			copyPathzRules := "/mnt/rdsfs/ems/gnsi"

			target := fmt.Sprintf("%s:%v", d.sshIp, d.sshPort)
			t.Logf("Copying Pathz rules file to %s (%s) over scp", d.dut, target)
			sshConf := scp.NewSSHConfigFromPassword(d.sshUser, d.sshPass)
			scpClient, err := scp.NewClient(target, sshConf, &scp.ClientOption{})
			if err != nil {
				t.Fatalf("Error initializing scp client: %v", err)
			}
			defer scpClient.Close()

			// Copy Pathz policy file to DUT
			resp := scpClient.CopyFileToRemote(pathzRulesPath, copyPathzRules, &scp.FileTransferOption{})
			t.Logf("copying file got %v", resp)
			if resp == nil || strings.Contains(resp.Error(), "Function not implemented") {
				t.Logf("SCP successful: File copied successfully")
			} else {
				t.Fatalf("SCP attempt failed: %s", resp.Error())
			}

			// guarantee a few timestamps before set operation.
			time.Sleep(10 * time.Second)

			// Perform a gNMI Set Request with 13 MB of Data
			set := perf.CreateInterfaceSetFromOCRoot(util.LoadJsonFileToOC(t, "testdata/set_config.json"), true)

			t.Logf("Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Verify gNMI Operations.
			performOperations(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for active policy instance after process restart.
			client := start(t)
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_xpath")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform a gNMI Set Request with 5 MB of Data
			t.Logf("AfterRPSO:Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("AfterRPSO:Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			timestamp := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").GnmiPathzPolicyCreatedOn().State())
			t.Logf("Got the expected Policy timestamp: %v", timestamp)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, timestamp, "5800-Rules", false)

			// Verify the policy counters after RP Switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			time.Sleep(10 * time.Second)

			// Sample memory usage after the operation
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Perform RP Switchover after deleting pathz policy file.
			utils.Dorpfo(context.Background(), t, true)

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Perform GET operations for active policy instance after process restart.
			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)

			if actv_res != nil {
				t.Fatalf("Pathz Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_xpath")

			// Get and store the result in portNum
			portNum = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform a gNMI Set Request with 5 MB of Data
			t.Logf("AfterRPSO:Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("AfterRPSO:Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// Sample memory usage after Deleting Pathz policy file.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after d
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed after deleting pathz_policy.txt")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Verify gNMI Operations after emsd process restart.
			performOperations(t, dut)

			// Sample memory usage after Deleting Pathz policy file.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after d
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed after deleting pathz_policy.txt")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// cleanup subinterfaces configs
			pathz.CleanUPInterface(t, dut)

			// Sample memory usage after removing gNMI Set Request with 19 MB.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after removing gNMI Set Request with 19 MB.
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed removing gNMI Set Request with 5 MB.")
			}
			// Check top CPU utilization after removing gNMI Set Request with 19 MB.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Get the final emsd memory usage
			final_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Final emsd memory usage: %v", final_emsd_memory)
		}
	})
	t.Run("RPSO: Test Pathz Policy File with 5800 Pathz Rules Request and gNMI Scale Operations", func(t *testing.T) {
		// Test Pathz Policy File with 5800 Pathz Rules Request and gNMI Scale Operations
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			fileName := "testdata/pathz_path.txt"

			// Start gRPC client
			client := start(t)

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Function to check the platform status
			Resp := pathz.CheckPlatformStatus(t, dut)
			if Resp != nil {
				fmt.Printf("Error: %v\n", Resp)
			} else {
				fmt.Println("All CPU0 entries are in 'IOS XR RUN' state.")
			}

			// Perform eMSD process restart before capturing intial emsd process memory.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			batchSet, leavesCnt := pathz.GenerateSubInterfaceConfig(t, dut)
			t.Logf("configuration %v :", batchSet)
			t.Logf("Leaves count %v :", leavesCnt)

			// Initialize the verifier
			verifier := pathz.NewVerifier()

			// Sample memory usage before the operation
			verifier.SampleBefore(t, dut)

			// Get the initial emsd memory usage
			intial_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Initial emsd memory usage: %v", intial_emsd_memory)

			// Rotate Request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				req := pathz.GenerateRules(fileName, "openconfig", d.sshUser, createdtime)
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform GET operations for active policy instance after process restart.
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_rule")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// guarantee a few timestamps before set operation.
			time.Sleep(10 * time.Second)

			// Perform a gNMI Set Request with 5 MB of Data
			set := perf.CreateInterfaceSetFromOCRoot(util.LoadJsonFileToOC(t, "testdata/set_config.json"), true)

			t.Logf("After process restart:Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("After process restart:Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "5800-Rules", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			time.Sleep(10 * time.Second)

			// Sample memory usage after the operation
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for active policy instance after process restart.
			client = start(t)
			getReq_Actv = &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_xpath")

			// Get and store the result in portNum
			portNum = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			time.Sleep(10 * time.Second)

			// Perform a gNMI Set Request with 5 MB of Data

			t.Logf("After process restart:Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("After process restart:Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "5800-Rules", false)

			// Verify the policy counters after process restart.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)

			// Sample memory usage after the operation
			verifier.SampleAfter(t, dut)

			// Verify memory usage
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Perform RP Switchover after deleting pathz policy file.
			utils.Dorpfo(context.Background(), t, true)

			// guarantee a few timestamps before emsd restart occurs
			time.Sleep(10 * time.Second)

			// Perform GET operations for active policy instance after process restart.
			actv_res, _ = client.Get(context.Background(), getReq_Actv)
			t.Logf("Active Response : %v", actv_res)

			if actv_res != nil {
				t.Fatalf("Pathz Get request is failed on device %s", dut.Name())
			}

			// Verify gNMI Operations.
			isPermissionDeniedError(t, dut, "undefined_xpath")

			// Get and store the result in portNum
			portNum = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number: %v", portNum)
			}

			// Perform a gNMI Set Request with 5 MB of Data
			t.Logf("AfterRPSO:Starting batch programming of %d leaves at %s", leavesCnt, time.Now())
			perf.BatchSet(t, dut, set, leavesCnt)
			t.Logf("AfterRPSO:Finished batch programming of %d leaves at %s", leavesCnt, time.Now())

			// Sample memory usage after Deleting Pathz policy file.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after d
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed after deleting pathz_policy.txt")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Verify gNMI Operations after emsd process restart.
			performOperations(t, dut)

			// Sample memory usage after Deleting Pathz policy file.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after d
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed after deleting pathz_policy.txt")
			}

			// Check top CPU utilization.
			pathz.TopCpuMemoryUtilization(t, dut)

			// cleanup subinterfaces configs
			pathz.CleanUPInterface(t, dut)

			// Sample memory usage after removing gNMI Set Request with 19 MB.
			verifier.SampleAfter(t, dut)

			// Verify memory usage after removing gNMI Set Request with 19 MB.
			if !verifier.Verify(t) {
				t.Errorf("Memory usage verification failed removing gNMI Set Request with 19 MB.")
			}
			// Check top CPU utilization after removing gNMI Set Request with 19 MB.
			pathz.TopCpuMemoryUtilization(t, dut)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Get the final emsd memory usage
			final_emsd_memory := pathz.EmsdMemoryCheck(t, dut)
			t.Logf("Final emsd memory usage: %v", final_emsd_memory)
		}
	})
	t.Run("RPSO: Test Authz Over Pathz", func(t *testing.T) {
		// Authz Pathz Test with RP switchover.
		for _, d := range parseBindingFile(t) {
			createdtime := uint64(time.Now().UnixMicro())

			policyMap := authz.LoadPolicyFromJSONFile(t, "testdata/policy.json")

			// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
			newpolicy, ok := policyMap["policy-gNMI-set"]
			if !ok {
				t.Fatal("policy-gNMI-set is not loaded from policy json file")
			}
			newpolicy.AddAllowRules("base", []string{d.sshUser}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
			// Rotate the policy.
			newpolicy.Rotate(t, dut, createdtime, "policy-gNMI-set", false)

			// Define probe request
			probeReq := &pathzpb.ProbeRequest{
				Mode:           pathzpb.Mode_MODE_WRITE,
				User:           d.sshUser,
				Path:           &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			// Define expected response
			want := &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_PERMIT,
			}

			// Declare probeBeforeFinalize
			probeBeforeFinalize := false

			// Start gRPC client
			client := start(t)

			// Perform Rotate request
			rc, err := client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_PERMIT,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err := client.Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			get_res := &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_PERMIT,
					}},
				},
			}

			// Perform GET operations for sandbox policy instance after RP_Switchover.
			getReq_Sand := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_SANDBOX,
			}

			sand_res, _ := client.Get(context.Background(), getReq_Sand)
			t.Logf("Response : %v", sand_res)
			if d := cmp.Diff(get_res, sand_res, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Perform GET operations for active policy instance
			getReq_Actv := &pathzpb.GetRequest{
				PolicyInstance: pathzpb.PolicyInstance_POLICY_INSTANCE_ACTIVE,
			}

			actv_res, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Perform gNMI operations
			isPermissionDeniedError(t, dut, "AuthzPathz")

			// Get and store the result in portNum
			portNum := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())

			if portNum == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after RP_Switchover: %v", portNum)
			}

			path := gnmi.OC().Lldp().Enabled()
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after RP_Switchover")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for active policy instance after deleting authz policy.
			sand_res_Authzpolicy, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_Authzpolicy, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting authz policy: %s", d)
			}

			actv_res_Authzpolicy, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_Authzpolicy, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting authz policy: %s", d)
			}

			// Verify gNMI SET Operations after RP Switchover.
			isPermissionDeniedError(t, dut, "AuthzDeny")

			// Get and store the result in portNum after RP Switchover.
			portNum_Authzpolicy := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_Authzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting authz policy: %v", portNum_Authzpolicy)
			}

			// Verify gNMI SET Operation for different xpath after RP Switchover.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting authz policy")
			}

			// Delete Authz policy file and verify the behaviour after RP_Switchover
			pathz.DeletePolicyData(t, dut, "authz_policy.txt")

			// Verify gNMI SET Operations after deleting Authz policy.
			isPermissionDeniedError(t, dut, "AfterAuthzPolicyDelete")

			// Get and store the result in portNum after deleting authz policy.
			portNum_after_AuthzDel := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_AuthzDel == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting authz policy: %v", portNum_after_AuthzDel)
			}

			// Verify gNMI SET Operation for different xpath after deleting Authz policy.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting authz policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for active policy instance after RP Switchover.
			client = start(t)
			sand_res_after_RP_Switchover, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_RP_Switchover, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			actv_res_after_RP_Switchover, err := client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_RP_Switchover, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after RP_Switchover: %s", d)
			}

			// Verify gNMI SET Operations after RP_Switchover.
			performOperations(t, dut)

			// Get and store the result in portNum after RP_Switchover.
			portNum_after_RP_Switchover := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_RP_Switchover == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after RP_Switchover: %v", portNum_after_RP_Switchover)
			}

			// Verify gNMI SET Operation for different xpath after RP_Switchover.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after RP_Switchover")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting Authz policy file.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", false, true, 0, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform Rotate request 2
			rc, err = client.Rotate(context.Background())
			if err == nil {
				// Define rotate request
				req := &pathzpb.RotateRequest{
					RotateRequest: &pathzpb.RotateRequest_UploadRequest{
						UploadRequest: &pathzpb.UploadRequest{
							Version:   "1",
							CreatedOn: createdtime,
							Policy: &pathzpb.AuthorizationPolicy{
								Rules: []*pathzpb.AuthorizationRule{{
									Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
									Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
									Mode:      pathzpb.Mode_MODE_WRITE,
									Action:    pathzpb.Action_ACTION_DENY,
								}},
							},
						},
					},
				}
				mustSendAndRecv(t, rc, req)
				if !probeBeforeFinalize {
					mustFinalize(t, rc)
				}
			}

			time.Sleep(5 * time.Second)

			// Define expected response
			want = &pathzpb.ProbeResponse{
				Version: "1",
				Action:  pathzpb.Action_ACTION_DENY,
			}

			// Perform Probe request
			t.Logf("Probe Request : %v", probeReq)
			got, err = client.Probe(context.Background(), probeReq)
			t.Logf("Probe Response : %v", got)

			if err != nil {
				t.Fatalf("Probe() unexpected error: %v", err)
			}

			// Check for differences between expected and actual responses
			if d := cmp.Diff(want, got, protocmp.Transform()); d != "" {
				t.Fatalf("Probe() unexpected diff: %s", d)
			}

			get_res = &pathzpb.GetResponse{
				Version:   "1",
				CreatedOn: createdtime,
				Policy: &pathzpb.AuthorizationPolicy{
					Rules: []*pathzpb.AuthorizationRule{{
						Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
						Principal: &pathzpb.AuthorizationRule_User{User: d.sshUser},
						Mode:      pathzpb.Mode_MODE_WRITE,
						Action:    pathzpb.Action_ACTION_DENY,
					}},
				},
			}

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.bak")

			// Perform GET operations for sandbox policy instance after deleting pathz policy.
			sand_res_after_pathz_del, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Perform GET operations for active policy instance after deleting pathz policy.
			actv_res_after_pathz_del, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Required Response : %v", get_res)
			t.Logf("Got Response : %v", actv_res_after_pathz_del)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Verify gNMI SET Operations after deleting pathz policy.
			isPermissionDeniedError(t, dut, "AfterPathzPolicyDelete")

			// Get and store the result in portNum after deleting pathz policy.
			portNum_after_pathz_del := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_pathz_del == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting pathz policy: %v", portNum_after_pathz_del)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz policy.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting pathz policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting backup pathz policy file.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, true, 3, 3)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Reload router after deleting pathz policy & verify the behaviour.
			perf.ReloadRouter(t, dut)

			// Perform GET operations for sandbox policy instance after router reload.
			client = start(t)
			sand_res_after_router_reload, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_router_reload, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Perform GET operations for active policy instance after router reload.
			actv_res_after_router_reload, err := client.Get(context.Background(), getReq_Actv)
			t.Logf("Required Response : %v", get_res)
			t.Logf("Got Response : %v", actv_res_after_router_reload)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_router_reload, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
			}

			// Verify gNMI SET Operations after router reload.
			isPermissionDeniedError(t, dut, "AfterRouterReload")

			// Get and store the result in portNum after router reload.
			portNum_after_del_Pathzpolicy := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_del_Pathzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after router reload: %v", portNum_after_del_Pathzpolicy)
			}

			// Verify gNMI SET Operation for different xpath after router reload
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after router reload")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after router reload.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Delete Pathz policy file and verify the behaviour
			pathz.DeletePolicyData(t, dut, "pathz_policy.txt")

			// Perform GET operations for sandbox policy instance after deleting pathz policy.
			sand_res_after_pathz_del, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_pathz_del, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Perform GET operations for active policy instance after deleting pathz policy.
			actv_res_after_pathz_del, err = client.Get(context.Background(), getReq_Actv)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_pathz_del, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after deleting pathz policy: %s", d)
			}

			// Verify gNMI SET Operations after deleting pathz policy.
			isPermissionDeniedError(t, dut, "AfterPathzPolicyDelete")

			// Get and store the result in portNum after deleting pathz policy.
			portNum_after_del_Pathzpolicy = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_del_Pathzpolicy == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after deleting pathz policy: %v", portNum_after_del_Pathzpolicy)
			}

			// Verify gNMI SET Operation for different xpath after deleting pathz policy.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after deleting pathz policy")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after deleting pathz policy file.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 2, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 6, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform RP Switchover after deleting Pathz file
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for sandbox policy instance after RP Switchover.
			client = start(t)
			sand_res_after_RP_Switchover, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_RP_Switchover, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP Switchover: %s", d)
			}

			// Perform GET operations for active policy instance after RP Switchover.
			actv_res_after_RP_Switchover, err = client.Get(context.Background(), getReq_Actv)
			t.Logf("GOT Response: %v", actv_res_after_RP_Switchover)
			if err != nil {
				t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
			}
			if d := cmp.Diff(get_res, actv_res_after_RP_Switchover, protocmp.Transform()); d != "" {
				t.Fatalf("Pathz Get unexpected diff after RP Switchover: %s", d)
			}

			// Verify gNMI SET Operations after RP Switchover.
			isPermissionDeniedError(t, dut, "AfterRPSwitchover")

			// Get and store the result in portNum deleting after RP Switchover..
			portNum_after_emsd_restart := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_emsd_restart == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after RP Switchover.: %v", portNum)
			}

			// Verify gNMI SET Operation for different xpath after RP Switchover.
			if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
				got := gnmi.Update(t, dut, path.Config(), true)
				t.Logf("gNMI Update : %v", got)
			}); errMsg != nil {
				t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
			} else {
				t.Errorf("This gNMI Update should have failed after RP Switchover")
			}

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, createdtime, "1", false)

			// Verify the policy counters after RP Switchover.
			pathz.VerifyWritePolicyCounters(t, dut, "/", true, false, 1, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/", false, false, 0, 0)
			pathz.VerifyWritePolicyCounters(t, dut, "/system/config/hostname", true, false, 3, 0)
			pathz.VerifyReadPolicyCounters(t, dut, "/system/config/hostname", false, false, 0, 0)

			// Perform eMSD process restart after RP_Switchover.
			t.Logf("Restarting emsd at %s", time.Now())
			perf.RestartProcess(t, dut, "emsd")
			t.Logf("Restart emsd finished at %s", time.Now())

			// Perform GET operations for sandbox policy instance after process restart.
			sand_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Perform GET operations for active policy instance after process restart.
			actv_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Actv)
			if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
			}

			// Verify gNMI SET Operations after after process restart.
			performOperations(t, dut)

			// Get and store the result in portNum after process restart.
			portNum_after_emsd_restart = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_emsd_restart == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after process restart: %v", portNum_after_emsd_restart)
			}

			// Verify gNMI SET Operation for different xpath after process restart.
			gnmi.Update(t, dut, path.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)

			// Perform RP Switchover
			utils.Dorpfo(context.Background(), t, true)

			// Perform GET operations for sandbox policy instance after RP Switchover.
			client = start(t)
			sand_res_after_RP_Switchover, _ = client.Get(context.Background(), getReq_Sand)
			if d := cmp.Diff(get_res, sand_res_after_RP_Switchover, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP Switchover: %s", d)
			}

			// Perform GET operations for active policy instance after RP Switchover.
			actv_res_after_RP_Switchover, _ = client.Get(context.Background(), getReq_Actv)
			if d := cmp.Diff(get_res, actv_res_after_RP_Switchover, protocmp.Transform()); d == "" {
				t.Fatalf("Pathz Get unexpected diff after RP Switchover: %s", d)
			}

			// Verify gNMI SET Operations after RP Switchover.
			performOperations(t, dut)

			// Get and store the result in portNum after RP Switchover.
			portNum_after_RP_Switchover = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
			if portNum_after_RP_Switchover == uint16(0) || portNum > uint16(0) {
				t.Logf("Got the expected port number")
			} else {
				t.Fatalf("Unexpected value for port number after RP Switchover: %v", portNum_after_RP_Switchover)
			}

			// Verify gNMI SET Operation for different xpath after RP Switchover.
			gnmi.Update(t, dut, path.Config(), true)

			// Verify the policy info
			pathz.VerifyPolicyInfo(t, dut, 0, "", true)
		}
	})
}

func mustSendAndRecv(t testing.TB, rc pathzpb.Pathz_RotateClient, req *pathzpb.RotateRequest) {
	t.Helper()
	t.Logf("Rotate Req : %v", req)
	if err := rc.Send(req); err != nil {
		t.Fatalf("failed to send: %v", err)
	}

	if _, err := rc.Recv(); err != nil {
		t.Fatalf("failed to recv: %v", err)
	}
}

func mustFinalize(t testing.TB, rc pathzpb.Pathz_RotateClient) {
	t.Helper()
	mustSendAndRecv(t, rc, &pathzpb.RotateRequest{RotateRequest: &pathzpb.RotateRequest_FinalizeRotation{}})
}

func dialConn(t *testing.T, dut *ondatra.DUTDevice, svc introspect.Service, wantPort uint32) *grpc.ClientConn {
	t.Helper()
	if svc == introspect.GNOI || svc == introspect.GNSI {
		// Renaming service name due to gnoi and gnsi always residing on same port as gnmi.
		svc = introspect.GNMI
	}
	dialer := introspect.DUTDialer(t, dut, introspect.GNMI)
	if dialer.DevicePort != int(wantPort) {
		t.Fatalf("DUT is not listening on correct port for %q: got %d, want %d", svc, dialer.DevicePort, wantPort)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := dialer.Dial(ctx)
	if err != nil {
		t.Fatalf("grpc.Dial failed to: %q", dialer.DialTarget)
	}
	return conn
}

func parseBindingFile(t *testing.T) []targetInfo {
	t.Helper()

	bindingFile := flag.Lookup("binding").Value.String()
	in, err := os.ReadFile(bindingFile)
	if err != nil {
		t.Fatalf("unable to read binding file")
	}

	b := &bindpb.Binding{}
	if err := prototext.Unmarshal(in, b); err != nil {
		t.Fatalf("unable to parse binding file")
	}

	targets := []targetInfo{}
	for _, dut := range b.Duts {

		sshUser := dut.Ssh.Username
		if sshUser == "" {
			sshUser = dut.Options.Username
		}
		if sshUser == "" {
			sshUser = b.Options.Username
		}

		sshPass := dut.Ssh.Password
		if sshPass == "" {
			sshPass = dut.Options.Password
		}
		if sshPass == "" {
			sshPass = b.Options.Password
		}

		sshTarget := strings.Split(dut.Ssh.Target, ":")
		sshIp := sshTarget[0]
		sshPort, _ := strconv.Atoi(strings.Split(dut.Ssh.Target, ":")[1])
		// sshPort := "22"
		// if len(sshTarget) > 1 {
		// 	sshPort = sshTarget[1]
		// }

		targets = append(targets, targetInfo{
			dut:     dut.Id,
			sshIp:   sshIp,
			sshPort: sshPort,
			sshUser: sshUser,
			sshPass: sshPass,
		})
	}

	return targets
}

func start(t *testing.T) pathzpb.PathzClient {
	dut := ondatra.DUT(t, "dut")
	conn := dialConn(t, dut, introspect.GNSI, 9339)
	c := pathzpb.NewPathzClient(conn)

	return c
}
