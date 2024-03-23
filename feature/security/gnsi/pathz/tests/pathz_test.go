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
	"flag"
	"testing"
	"time"

	log "github.com/golang/glog"
	"github.com/google/go-cmp/cmp"

	perf "github.com/openconfig/featureprofiles/feature/cisco/performance"
	ciscoFlags "github.com/openconfig/featureprofiles/internal/cisco/flags"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/authz"
	"github.com/openconfig/featureprofiles/internal/security/gnxi"
	"github.com/openconfig/featureprofiles/internal/security/pathz"
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

var (
	testInfraID = flag.String("test_infra_id", "cafyauto", "SPIFFE-ID used by test Infra ID user for authz operation")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type setOperation int

const (
	// deletePath represents a SetRequest delete.
	deletePath setOperation = iota
	// replacePath represents a SetRequest replace.
	updatePath
	isisInstance = "B4"
)

func getsandboxresponse(t *testing.T, want *pathzpb.GetResponse) {
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

	gnmi.Update(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Config(), request)
	gnmi.Delete(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, "B4").Isis().Config())
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance("DEFAULT").Config(), request)
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

	log.V(1).Info("Request Sent:\n%s", prototext.Format(r))
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

func TestInvalidWithoutFinalize(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
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
		log.V(1).Infof("Request Sent:\n%s", prototext.Format(req))
		if err := rot.Send(req); err != nil {
			t.Logf("Rec Err %v", err)
			t.Fatal(err)
		}
		received, err := rot.Recv()
		log.V(1).Infof("Received Request:\n%s", prototext.Format(received))
		t.Logf("Rec Err %v", err)

		// Check if the error string matches the expected error string
		if d := errdiff.Check(err, wantErrs); d != "" {
			t.Errorf("Rotate() unexpected err: %s", d)
		}
	}
	performOperations(t, dut)
}

func TestMultipleInvalidRotate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
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
		log.V(1).Infof("Request Sent:\n%s", prototext.Format(req))
		if err := rot.Send(req); err != nil {
			t.Logf("Rec Err %v", err)
			t.Fatal(err)
		}
		received, err := rot.Recv()
		log.V(1).Infof("Received Request:\n%s", prototext.Format(received))
		t.Logf("Rec Err %v", err)

		// Check if the error string matches the expected error string
		if d := errdiff.Check(err, wantErrs[i]); d != "" {
			t.Errorf("Rotate() unexpected err: %s", d)
		}
	}
	performOperations(t, dut)
}

func TestFinalizeWithoutRotate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
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
		log.V(1).Infof("Request Sent:\n%s", prototext.Format(req))
		if err := rot.Send(req); err != nil {
			t.Logf("Rec Err %v", err)
			t.Fatal(err)

		}
		received, err := rot.Recv()
		log.V(1).Infof("Received Request:\n%s", prototext.Format(received))
		t.Logf("Rec Err %v", err)

		// Check if the error string matches the expected error string
		if d := errdiff.Check(err, wantErrs[i]); d != "" {
			t.Errorf("Rotate() unexpected err: %s", d)
		}
	}
	performOperations(t, dut)
}

func TestInvalidProbeUser(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
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
}

func TestInvalidProbePath(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// Define the expected error string
	wantErr := "Nil Probe Request or Path"
	probeReq := &pathzpb.ProbeRequest{
		Mode:           pathzpb.Mode_MODE_READ,
		User:           "cafyauto",
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
}

func TestInvalidProbeInst(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// Define the expected error string
	wantErr := "Unknown instance type"
	probeReq := &pathzpb.ProbeRequest{
		Mode: pathzpb.Mode_MODE_READ,
		User: "cafyauto",
		Path: &gpb.Path{},
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
}

func TestProbeInstWithoutReq(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// Define the expected error string
	wantErr := "requested policy instance is nil"
	probeReq := &pathzpb.ProbeRequest{
		Mode:           pathzpb.Mode_MODE_READ,
		User:           "cafyauto",
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
}

func TestInvalidProbeMode(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// Define the expected error string
	wantErr := "mode not specified"

	probeReq := &pathzpb.ProbeRequest{
		User:           "cafyauto",
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
}

func TestProbeReqWithoutFinalize(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Define probe request
	probeReq := &pathzpb.ProbeRequest{
		Mode:           pathzpb.Mode_MODE_WRITE,
		User:           "cafyauto",
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
							Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
				Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	actv_res, _ := client.Get(context.Background(), getReq_Actv)
	if d := cmp.Diff(get_res, actv_res, protocmp.Transform()); d == "" {
		t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
	}
	t.Logf("GET Active Response : %v", actv_res)

	// Perform gNMI operations
	performOperations(t, dut)

	// Perform eMSD process restart
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
	t.Logf("Restart emsd finished at %s", time.Now())

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

	//Reload router
	pathz.ReloadRouter(t, dut)

	// Perform GET operations for sandbox policy instance after router reload.
	client = start(t)
	sand_res_after_router_reload, _ := client.Get(context.Background(), getReq_Sand)
	if d := cmp.Diff(get_res, sand_res_after_router_reload, protocmp.Transform()); d == "" {
		t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
	}
	t.Logf("GET Sandbox Response after Router Reload: %v", sand_res_after_router_reload)

	// Perform GET operations for active policy instance after router reload.
	actv_res_after_router_reload, _ := client.Get(context.Background(), getReq_Actv)
	if d := cmp.Diff(get_res, actv_res_after_router_reload, protocmp.Transform()); d == "" {
		t.Fatalf("Pathz Get unexpected diff before finalize: %s", d)
	}
	t.Logf("GET Active Response after Router Reload: %v", actv_res_after_router_reload)

	// Perform gNMI operations after Router Reload.
	performOperations(t, dut)
}

func TestInvalidWithFinalize(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
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

	log.V(1).Infof("Request Sent:\n%s", prototext.Format(req))
	if err := rot.Send(req); err != nil {
		t.Logf("Rec Err %v", err)
		t.Fatal(err)
	}

	// Perform GET operations for sandbox policy instance
	getsandboxresponse(t, want)

	// Finalize
	req = &pathzpb.RotateRequest{
		RotateRequest: &pathzpb.RotateRequest_FinalizeRotation{},
	}

	log.V(1).Infof("Request Sent:\n%s", prototext.Format(req))
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

	// Perform eMSD process restart
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

	//Reload router
	pathz.ReloadRouter(t, dut)

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
}

func TestInvalidXpathFinalize(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	want := &pathzpb.GetResponse{
		Version: "1",
		Policy: &pathzpb.AuthorizationPolicy{
			Rules: []*pathzpb.AuthorizationRule{{
				Path:      &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}, {Name: "hostname"}}},
				Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
						Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	log.V(1).Infof("Request Sent:\n%s", prototext.Format(req))
	if err := rot.Send(req); err != nil {
		t.Logf("Rec Err %v", err)
		t.Fatal(err)
	}

	// Perform GET operations for sandbox policy instance
	getsandboxresponse(t, want)

	// Finalize
	req = &pathzpb.RotateRequest{
		RotateRequest: &pathzpb.RotateRequest_FinalizeRotation{},
	}

	log.V(1).Infof("Request Sent:\n%s", prototext.Format(req))
	if err := rot.Send(req); err != nil {
		t.Logf("Rec Err %v", err)
		t.Fatal(err)
	}

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

	// Perform eMSD process restart
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

}
func TestProbeReqWithFinalize(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Define probe request
	probeReq := &pathzpb.ProbeRequest{
		Mode:           pathzpb.Mode_MODE_WRITE,
		User:           "cafyauto",
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
							Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
				Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	// Perform eMSD process restart
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

	// Reload router
	pathz.ReloadRouter(t, dut)

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
}

func TestConflictBwExplicitKeyAnsAsterisk_1(t *testing.T) {
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
									Name: "cafyauto",
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
						Name: "cafyauto",
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	// Perform eMSD process restart
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

	// Reload router
	pathz.ReloadRouter(t, dut)

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
}

func TestConflictBwExplicitKeyAnsAsterisk_2(t *testing.T) {
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
									Name: "cafyauto",
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
						Name: "cafyauto",
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
}

func TestConflictBwExplicitKeyAnsAsterisk_3(t *testing.T) {
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
									Name: "cafyauto",
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
						Name: "cafyauto",
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
}

func TestConflictBwGroupAndUser_1(t *testing.T) {
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	// Perform Rotate request
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
									Name: "cafyauto",
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
						Name: "cafyauto",
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

}

func TestConflictBwGroupAndUser_2(t *testing.T) {
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
									Name: "cafyauto",
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
						Name: "cafyauto",
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

}

func TestConflictBwGroupAndUser_3(t *testing.T) {
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
									Name: "cafyauto",
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
						Name: "cafyauto",
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

}

func TestConflictBwGroupAndUser_4(t *testing.T) {
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
									Name: "cafyauto",
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
						Name: "cafyauto",
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

}

func TestConflictBwGroupAndUser_5(t *testing.T) {
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
									Name: "cafyauto",
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
						Name: "cafyauto",
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	// Perform eMSD process restart
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

	// Reload router
	pathz.ReloadRouter(t, dut)

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

}

func TestGnmiOriginCli(t *testing.T) {
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
						Rules: []*pathzpb.AuthorizationRule{{
							Path:      &gpb.Path{Origin: "", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
							Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
				Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	// Perform eMSD process restart
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

	// Reload router
	pathz.ReloadRouter(t, dut)

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
}

func TestConflictBwUsers_1(t *testing.T) {
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

}
func TestConflictBwUsers_2(t *testing.T) {
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	// Perform eMSD process restart
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

	// Reload router
	pathz.ReloadRouter(t, dut)

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
}

func TestLongestPrefixMatch_1(t *testing.T) {
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
									Name: "cafyauto",
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
						Name: "cafyauto",
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
}

func TestLongestPrefixMatch_2(t *testing.T) {
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
									Name: "cafyauto",
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
								Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
						Name: "cafyauto",
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
					Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
}

func TestLongestPrefixMatch_3(t *testing.T) {
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
									Name: "cafyauto",
								},
							},
						}, {
							Name: "admin",
							Users: []*pathzpb.User{
								{
									Name: "cafyauto",
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
						Name: "cafyauto",
					},
				},
			}, {
				Name: "admin",
				Users: []*pathzpb.User{
					{
						Name: "cafyauto",
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
}

func TestConflictBwGroups(t *testing.T) {
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
									Name: "cafyauto",
								},
							},
						}, {
							Name: "admin",
							Users: []*pathzpb.User{
								{
									Name: "cafyauto",
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
						Name: "cafyauto",
					},
				},
			}, {
				Name: "admin",
				Users: []*pathzpb.User{
					{
						Name: "cafyauto",
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

	// Perform eMSD process restart
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

	// Reload router
	pathz.ReloadRouter(t, dut)

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
}

func TestPathz_txt_bak_1(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	createdtime := uint64(time.Now().UnixMicro())

	// Define probe request
	probeReq := &pathzpb.ProbeRequest{
		Mode:           pathzpb.Mode_MODE_WRITE,
		User:           "cafyauto",
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
							Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
							Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
				Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	// Verify gNMI SET Operation for different xpath after deleting pathz Backup file.
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		got := gnmi.Update(t, dut, path.Config(), true)
		t.Logf("gNMI Update : %v", got)
	}); errMsg != nil {
		t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
	} else {
		t.Errorf("This gNMI Update should have failed after deleting pathz Backup file")
	}

	// Perform eMSD process restart after deleting Pathz backup file.
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

	// Perform eMSD process restart after deleting Pathz file.
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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
}

func TestPathz_txt_bak_2(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	createdtime := uint64(time.Now().UnixMicro())

	// Define probe request
	probeReq := &pathzpb.ProbeRequest{
		Mode:           pathzpb.Mode_MODE_WRITE,
		User:           "cafyauto",
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
							Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
							Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
				Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	get_res = &pathzpb.GetResponse{
		Version:   "1",
		CreatedOn: createdtime,
		Policy: &pathzpb.AuthorizationPolicy{
			Rules: []*pathzpb.AuthorizationRule{{
				Path:      &gpb.Path{Origin: "openconfig", Elem: []*gpb.PathElem{{Name: "system"}, {Name: "config"}, {Name: "hostname"}}},
				Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
				Mode:      pathzpb.Mode_MODE_WRITE,
				Action:    pathzpb.Action_ACTION_PERMIT,
			}},
		},
	}

	// Perform eMSD process restart after deleting Pathz backup file.
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

	//Reload router after deleting Pathz Policy & verify the behaviour.
	pathz.ReloadRouter(t, dut)

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

	// Perform eMSD process restart after deleting Pathz file.
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
	t.Logf("Restart emsd finished at %s", time.Now())

	// Perform GET operations for sandbox policy instance after process restart.
	sand_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Sand)
	if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
		t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
	}

	// Perform GET operations for active policy instance after process restart.
	actv_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Actv)
	t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", actv_res_after_emsd_restart)

	if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d == "" {
		t.Logf("gNMI Update : %v", d)
		t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
	}

	// Verify gNMI SET Operations after process restart.
	performOperations(t, dut)
}

func TestPathz_txt_bak_3(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	createdtime := uint64(time.Now().UnixMicro())

	// Define probe request
	probeReq := &pathzpb.ProbeRequest{
		Mode:           pathzpb.Mode_MODE_WRITE,
		User:           "cafyauto",
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
									Name: "cafyauto",
								},
							},
						}, {
							Name: "admin",
							Users: []*pathzpb.User{
								{
									Name: "cafyauto",
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
							Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
				Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	get_res = &pathzpb.GetResponse{
		Version:   "1",
		CreatedOn: createdtime,
		Policy: &pathzpb.AuthorizationPolicy{
			Groups: []*pathzpb.Group{{
				Name: "pathz",
				Users: []*pathzpb.User{
					{
						Name: "cafyauto",
					},
				},
			}, {
				Name: "admin",
				Users: []*pathzpb.User{
					{
						Name: "cafyauto",
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
	perf.RestartEmsd(t, dut)
	t.Logf("Restart emsd finished at %s", time.Now())

	// Perform GET operations for sandbox policy instance after router reload.
	sand_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Sand)
	if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
		t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
	}

	// Perform GET operations for active policy instance after router reload.
	actv_res_after_emsd_restart, err := client.Get(context.Background(), getReq_Actv)
	if err != nil {
		t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
	}
	if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d != "" {
		t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
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

	// Perform eMSD process restart after deleting Pathz backup file.
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

	// Perform eMSD process restart after deleting Pathz file.
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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
}

func TestPathz_txt_bak_4(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	createdtime := uint64(time.Now().UnixMicro())

	// Define probe request
	probeReq := &pathzpb.ProbeRequest{
		Mode:           pathzpb.Mode_MODE_WRITE,
		User:           "cafyauto",
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
									Name: "cafyauto",
								},
							},
						}, {
							Name: "admin",
							Users: []*pathzpb.User{
								{
									Name: "cafyauto",
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
									Name: "cafyauto",
								},
							},
						}, {
							Name: "admin",
							Users: []*pathzpb.User{
								{
									Name: "cafyauto",
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
						Name: "cafyauto",
					},
				},
			}, {
				Name: "admin",
				Users: []*pathzpb.User{
					{
						Name: "cafyauto",
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
	isPermissionDeniedError(t, dut, "AfterPathzPolicyDelete")

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

	// //Reload router after deleting pathz Backup file & verify the behaviour.
	pathz.ReloadRouter(t, dut)

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

	// Perform eMSD process restart after deleting Pathz file.
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
	t.Logf("Restart emsd finished at %s", time.Now())

	// Perform GET operations for sandbox policy instance after process restart.
	sand_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Sand)
	if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
		t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
	}

	// Perform GET operations for active policy instance after process restart.
	actv_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Actv)
	t.Logf("Got GET Response : %s", actv_res_after_emsd_restart)
	if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d == "" {
		t.Logf("GET Difference : %v", d)
		t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
	}

	// Verify gNMI SET Operations after process restart.
	performOperations(t, dut)

	// Get and store the result in portNum deleting Authz Backup file.
	portNum_after_emsd_restart := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
	if portNum_after_emsd_restart == uint16(0) || portNum > uint16(0) {
		t.Logf("Got the expected port number")
	} else {
		t.Fatalf("Unexpected value for port number after process restart: %v", portNum)
	}

	// Verify gNMI SET Operation for different xpath deleting Authz Backup file.
	gnmi.Update(t, dut, path.Config(), true)
}

func TestAuthzPathz_1(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	createdtime := uint64(time.Now().UnixMicro())

	policyMap := authz.LoadPolicyFromJSONFile(t, "testdata/policy.json")

	// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
	newpolicy, ok := policyMap["policy-gNMI-set"]
	if !ok {
		t.Fatal("policy-gNMI-set is not loaded from policy json file")
	}
	newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
	// Rotate the policy.
	newpolicy.Rotate(t, dut, createdtime, "policy-gNMI-set", false)

	// Define probe request
	probeReq := &pathzpb.ProbeRequest{
		Mode:           pathzpb.Mode_MODE_WRITE,
		User:           "cafyauto",
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
							Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
				Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	// Perform eMSD process restart.
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
	t.Logf("Restart emsd finished at %s", time.Now())

	// Perform GET operations for sandbox policy instance after process restart
	sand_res_after_emsd_restart, _ := client.Get(context.Background(), getReq_Sand)
	if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
		t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
	}

	// Perform GET operations for active policy instance after process restart
	actv_res_after_emsd_restart, err := client.Get(context.Background(), getReq_Actv)
	if err != nil {
		t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
	}
	if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d != "" {
		t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
	}

	// Verify gNMI SET Operations after process restart.
	isPermissionDeniedError(t, dut, "AfterEmsdRestart")

	// Get and store the result in portNum deleting Authz Backup file.
	portNum_after_emsd_restart := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
	if portNum_after_emsd_restart == uint16(0) || portNum > uint16(0) {
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

	// Delete Authz backup policy file and verify the behaviour
	pathz.DeletePolicyData(t, dut, "authz_policy.bak")

	// Perform GET operations for sandbox policy instance after deleting Authz backup policy.
	sand_res_after_del_Authzbkup, _ := client.Get(context.Background(), getReq_Sand)
	if d := cmp.Diff(get_res, sand_res_after_del_Authzbkup, protocmp.Transform()); d == "" {
		t.Fatalf("Pathz Get unexpected diff after deleting Authz backup policy: %s", d)
	}

	// Perform GET operations for active policy instance after deleting Authz backup policy.
	actv_res_after_del_Authzbkup, err := client.Get(context.Background(), getReq_Actv)
	if err != nil {
		t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
	}
	if d := cmp.Diff(get_res, actv_res_after_del_Authzbkup, protocmp.Transform()); d != "" {
		t.Fatalf("Pathz Get unexpected diff after deleting Authz backup policy: %s", d)
	}

	// Verify gNMI SET Operations deleting Authz Backup file.
	isPermissionDeniedError(t, dut, "AfterAuthzBackupDelete")

	// Get and store the result in portNum deleting Authz Backup file.
	portNum_after_del_Authzbkup := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
	if portNum_after_del_Authzbkup == uint16(0) || portNum > uint16(0) {
		t.Logf("Got the expected port number")
	} else {
		t.Fatalf("Unexpected value for port number after deleting Authz backup policy: %v", portNum)
	}

	// Verify gNMI SET Operation for different xpath deleting Authz Backup file
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		got := gnmi.Update(t, dut, path.Config(), true)
		t.Logf("gNMI Update : %v", got)
	}); errMsg != nil {
		t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
	} else {
		t.Errorf("This gNMI Update should have failed after deleting Authz backup policy")
	}

	// Perform eMSD process restart after deleting Authz Backup file.
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

	// Verify gNMI SET Operations after process restart.
	isPermissionDeniedError(t, dut, "AfterEmsdRestart")

	// Get and store the result in portNum after process restart.
	portNum_after_emsd_restart = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
	if portNum_after_emsd_restart == uint16(0) || portNum > uint16(0) {
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

	// Delete Authz policy file and verify the behaviour
	pathz.DeletePolicyData(t, dut, "authz_policy.json")

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

	// Get and store the result in portNum after router reload.
	portNum_after_del_Authzpolicy := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
	if portNum_after_del_Authzpolicy == uint16(0) || portNum > uint16(0) {
		t.Logf("Got the expected port number")
	} else {
		t.Fatalf("Unexpected value for port number after deleting authz policy: %v", portNum)
	}

	// Verify gNMI SET Operation for different xpath after deleting authz policy.
	if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
		got := gnmi.Update(t, dut, path.Config(), true)
		t.Logf("gNMI Update : %v", got)
	}); errMsg != nil {
		t.Logf("Expected failure and got testt.CaptureFatal errMsg : %s", *errMsg)
	} else {
		t.Errorf("This gNMI Update should have failed after deleting authz policy")
	}

	//Reload router after deleting Authz Policy & verify the behaviour.
	pathz.ReloadRouter(t, dut)

	// Perform GET operations for sandbox policy instance after router reload.
	client = start(t)
	sand_res_reload_after_del_authz, _ := client.Get(context.Background(), getReq_Sand)
	if d := cmp.Diff(get_res, sand_res_reload_after_del_authz, protocmp.Transform()); d == "" {
		t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
	}

	// Perform GET operations for active policy instance after router reload.
	actv_res_reload_after_del_authz, err := client.Get(context.Background(), getReq_Actv)
	if err != nil {
		t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
	}
	if d := cmp.Diff(get_res, actv_res_reload_after_del_authz, protocmp.Transform()); d != "" {
		t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
	}

	// Verify gNMI SET Operations after router reload.
	performOperations(t, dut)

	// Get and store the result in portNum after router reload.
	portNum_reload_after_del_authz := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
	if portNum_reload_after_del_authz == uint16(0) || portNum > uint16(0) {
		t.Logf("Got the expected port number")
	} else {
		t.Fatalf("Unexpected value for port number after router reload: %v", portNum)
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
}

func TestAuthzPathz_2(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	createdtime := uint64(time.Now().UnixMicro())

	policyMap := authz.LoadPolicyFromJSONFile(t, "testdata/policy.json")

	// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
	newpolicy, ok := policyMap["policy-gNMI-set"]
	if !ok {
		t.Fatal("policy-gNMI-set is not loaded from policy json file")
	}
	newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
	// Rotate the policy.
	newpolicy.Rotate(t, dut, createdtime, "policy-gNMI-set", false)

	// Define probe request
	probeReq := &pathzpb.ProbeRequest{
		Mode:           pathzpb.Mode_MODE_WRITE,
		User:           "cafyauto",
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
							Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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
				Principal: &pathzpb.AuthorizationRule_User{User: "cafyauto"},
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

	// Delete Authz policy file and verify the behaviour
	pathz.DeletePolicyData(t, dut, "authz_policy.json")

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
		t.Fatalf("Unexpected value for port number after deleting authz policy: %v", portNum)
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

	// Perform eMSD process restart after deleting Authz Backup file.
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
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

	// Verify gNMI SET Operations after deleting pathz policy.
	isPermissionDeniedError(t, dut, "AfterPathzPolicyDelete")

	// Get and store the result in portNum after deleting pathz policy.
	portNum_after_pathz_del := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
	if portNum_after_pathz_del == uint16(0) || portNum > uint16(0) {
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

	//Reload router after deleting pathz policy & verify the behaviour.
	pathz.ReloadRouter(t, dut)

	// Perform GET operations for sandbox policy instance after router reload.
	client = start(t)
	sand_res_reload_after_del_pathz, _ := client.Get(context.Background(), getReq_Sand)
	if d := cmp.Diff(get_res, sand_res_reload_after_del_pathz, protocmp.Transform()); d == "" {
		t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
	}

	// Perform GET operations for active policy instance after router reload.
	actv_res_reload_after_del_pathz, err := client.Get(context.Background(), getReq_Actv)
	if err != nil {
		t.Fatalf("Pathz.Get request is failed on device %s", dut.Name())
	}
	if d := cmp.Diff(get_res, actv_res_reload_after_del_pathz, protocmp.Transform()); d != "" {
		t.Fatalf("Pathz Get unexpected diff after router reload: %s", d)
	}

	// Verify gNMI SET Operations after router reload.
	isPermissionDeniedError(t, dut, "AfterRouterReload")

	// Get and store the result in portNum after router reload.
	portNum_after_del_Pathzpolicy := gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
	if portNum_after_del_Pathzpolicy == uint16(0) || portNum > uint16(0) {
		t.Logf("Got the expected port number")
	} else {
		t.Fatalf("Unexpected value for port number after router reload: %v", portNum)
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
		t.Errorf("This gNMI Update should have failed after deleting pathz policy")
	}

	// Perform eMSD process restart after deleting Pathz file.
	t.Logf("Restarting emsd at %s", time.Now())
	perf.RestartEmsd(t, dut)
	t.Logf("Restart emsd finished at %s", time.Now())

	// Perform GET operations for sandbox policy instance after process restart.
	sand_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Sand)
	if d := cmp.Diff(get_res, sand_res_after_emsd_restart, protocmp.Transform()); d == "" {
		t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
	}

	// Perform GET operations for active policy instance after process restart.
	actv_res_after_emsd_restart, _ = client.Get(context.Background(), getReq_Actv)
	t.Logf("GOT Response: %v", actv_res_after_emsd_restart)
	if d := cmp.Diff(get_res, actv_res_after_emsd_restart, protocmp.Transform()); d == "" {
		t.Logf("GET Difference : %v", d)
		t.Fatalf("Pathz Get unexpected diff after process restart: %s", d)
	}

	// Verify gNMI SET Operations after process restart.
	performOperations(t, dut)

	// Get and store the result in portNum deleting after process restart.
	portNum_after_emsd_restart = gnmi.Get(t, dut, gnmi.OC().System().GrpcServer("DEFAULT").Port().State())
	if portNum_after_emsd_restart == uint16(0) || portNum > uint16(0) {
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

func start(t *testing.T) pathzpb.PathzClient {
	dut := ondatra.DUT(t, "dut")
	conn := dialConn(t, dut, introspect.GNSI, 9339)
	c := pathzpb.NewPathzClient(conn)

	return c
}
