// Copyright 2022 Google LLC
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

// Package authz_test performs functional tests for authz service
package authz_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/cisco/config"
	"github.com/openconfig/featureprofiles/internal/cisco/security/authz"
	"github.com/openconfig/featureprofiles/internal/cisco/security/gnxi"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gnps "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

var (
	sampleUser = "user1"
	usersCount = 10
	password   = "123456"
)

func createUsersOnDevice(t *testing.T, dut *ondatra.DUTDevice, users []*authz.User) {
	ocAuthentication := &oc.System_Aaa_Authentication{}
	for _, user := range users {
		ocUser := &oc.System_Aaa_Authentication_User{
			Username: ygot.String(user.Name),
			Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
			Password: ygot.String(password),
		}
		ocAuthentication.AppendUser(ocUser)
	}
	gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().Config(), ocAuthentication)
}

func TestSimpleAuthzGet(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test")
	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	t.Logf("Authz Policy of the device %s is %s", dut.Name(), authzPolicy.PrettyPrint())
}

func TestSimpleRotate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	policyBefore := authz.NewAuthorizationPolicy()
	policyBefore.Get(t, dut)
	defer policyBefore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	authzPolicy.AddAllowRules(users, []*gnxi.RPC{gnxi.RPCs.GNMI_SET})
	authzPolicy.Rotate(t, dut)

}

func TestAllowRuleAll(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	policyBefore := authz.NewAuthorizationPolicy()
	policyBefore.Get(t, dut)
	defer policyBefore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	authzPolicy.AddAllowRules(users, []*gnxi.RPC{gnxi.RPCs.ALL})
	authzPolicy.Rotate(t, dut)
	gnsiClient := dut.RawAPIs().GNSI(t)
	for path := range gnxi.RPCMAP {
		probReq := &authzpb.ProbeRequest{
			User: "user1",
			Rpc:  path,
		}
		resp, err := gnsiClient.Authz().Probe(context.Background(), probReq)
		if err != nil {
			t.Fatalf("Not expecting error for prob request %v", err)
		}
		if resp.GetAction() != authzpb.ProbeResponse_ACTION_PERMIT {
			t.Logf("Response on the Device %s after the Trigger is %s", dut.Name(), resp)
			t.Fatalf("Expecting ProbeResponse_ACTION_Permit for user %s path %s, received %v ", "user1", path, resp.GetAction())
		}
	}
}

func TestDenyRuleAll(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	policyBefore := authz.NewAuthorizationPolicy()
	policyBefore.Get(t, dut)
	defer policyBefore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	authzPolicy.AddDenyRules(users, []*gnxi.RPC{gnxi.RPCs.ALL})
	authzPolicy.Rotate(t, dut)
	gnsiClient := dut.RawAPIs().GNSI(t)
	for path := range gnxi.RPCMAP {
		probReq := &authzpb.ProbeRequest{
			User: "user1",
			Rpc:  path,
		}
		resp, err := gnsiClient.Authz().Probe(context.Background(), probReq)
		if err != nil {
			t.Fatalf("Not expecting error for prob request %v", err)
		}
		if resp.GetAction() != authzpb.ProbeResponse_ACTION_DENY {
			t.Fatalf("Expecting ProbeResponse_ACTION_DENY for user %s path %s, received %v ", "user1", path, resp.GetAction())
		}
	}

}

func TestAllowAllForService(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	policyBefore := authz.NewAuthorizationPolicy()
	policyBefore.Get(t, dut)
	defer policyBefore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	gnsiClient := dut.RawAPIs().GNSI(t)

	for path, service := range gnxi.RPCMAP {
		if strings.HasSuffix(path, "/*") {
			authzPolicy.AddAllowRules(users, []*gnxi.RPC{service})
		}
	}
	authzPolicy.Rotate(t, dut)
	for path := range gnxi.RPCMAP {
		if path == "*" {
			continue
		}
		probReq := &authzpb.ProbeRequest{
			User: "user1",
			Rpc:  path,
		}
		resp, err := gnsiClient.Authz().Probe(context.Background(), probReq)
		if err != nil {
			t.Fatalf("Not expecting error for prob request %v", err)
		}
		if resp.GetAction() != authzpb.ProbeResponse_ACTION_PERMIT {
			t.Fatalf("Expecting ProbeResponse_ACTION_Permit for user %s path %s, received %v ", "user1", path, resp.GetAction())
		}
	}
}

func TestDenyAllForService(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	policyBefore := authz.NewAuthorizationPolicy()
	policyBefore.Get(t, dut)
	defer policyBefore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	gnsiClient := dut.RawAPIs().GNSI(t)

	for path, service := range gnxi.RPCMAP {
		if strings.HasSuffix(path, "/*") {
			authzPolicy.AddDenyRules(users, []*gnxi.RPC{service})
		}
	}
	authzPolicy.Rotate(t, dut)
	for path := range gnxi.RPCMAP {
		probReq := &authzpb.ProbeRequest{
			User: "user1",
			Rpc:  path,
		}
		resp, err := gnsiClient.Authz().Probe(context.Background(), probReq)
		if err != nil {
			t.Fatalf("Not expecting error for prob request %v", err)
		}
		if resp.GetAction() != authzpb.ProbeResponse_ACTION_DENY {
			t.Fatalf("Expecting ProbeResponse_ACTION_Deny for user %s path %s, received %v ", "user1", path, resp.GetAction())
		}
	}
}

func TestAllowAllRPCs(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	policyBefore := authz.NewAuthorizationPolicy()
	policyBefore.Get(t, dut)
	defer policyBefore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	gnsiClient := dut.RawAPIs().GNSI(t)

	for path, service := range gnxi.RPCMAP {
		if strings.Contains(path, "*") {
			continue
		}
		authzPolicy.AddAllowRules(users, []*gnxi.RPC{service})
	}
	authzPolicy.Rotate(t, dut)
	for path := range gnxi.RPCMAP {
		probReq := &authzpb.ProbeRequest{
			User: "user1",
			Rpc:  path,
		}
		resp, err := gnsiClient.Authz().Probe(context.Background(), probReq)
		if err != nil {
			t.Fatalf("Not expecting error %s for prob request %v", dut.Name(), err)
		}
		if resp.GetAction() != authzpb.ProbeResponse_ACTION_PERMIT {
			t.Logf("Response on the Device %s after the Trigger is %s", dut.Name(), resp)
			t.Fatalf("Expecting ProbeResponse_ACTION_Permit for user %s path %s, received %v ", "user1", path, resp.GetAction())
		}
	}

}

func TestDenyAllRPCs(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	policyBefore := authz.NewAuthorizationPolicy()
	policyBefore.Get(t, dut)
	defer policyBefore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	gnsiClient := dut.RawAPIs().GNSI(t)

	for path, service := range gnxi.RPCMAP {
		if strings.Contains(path, "*") {
			continue
		}
		authzPolicy.AddDenyRules(users, []*gnxi.RPC{service})
	}
	authzPolicy.Rotate(t, dut)
	for path := range gnxi.RPCMAP {
		probReq := &authzpb.ProbeRequest{
			User: "user1",
			Rpc:  path,
		}
		resp, err := gnsiClient.Authz().Probe(context.Background(), probReq)
		if err != nil {
			t.Fatalf("Not expecting error for prob request %v", err)
		}
		if resp.GetAction() != authzpb.ProbeResponse_ACTION_DENY {
			t.Fatalf("Expecting ProbeResponse_ACTION_Deny for user %s path %s, received %v ", "user1", path, resp.GetAction())
		}
	}
}
func TestDenyOverWriteAllow(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	policyBefore := authz.NewAuthorizationPolicy()
	policyBefore.Get(t, dut)
	defer policyBefore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	gnsiClient := dut.RawAPIs().GNSI(t)

	for path, service := range gnxi.RPCMAP {
		if strings.Contains(path, "*") {
			continue
		}
		authzPolicy.AddDenyRules(users, []*gnxi.RPC{service})
		authzPolicy.AddAllowRules(users, []*gnxi.RPC{service})
	}
	authzPolicy.Rotate(t, dut)
	for path := range gnxi.RPCMAP {
		probReq := &authzpb.ProbeRequest{
			User: "user1",
			Rpc:  path,
		}
		resp, err := gnsiClient.Authz().Probe(context.Background(), probReq)
		if err != nil {
			t.Fatalf("Not expecting error for prob request %v", err)
		}
		if resp.GetAction() != authzpb.ProbeResponse_ACTION_DENY {
			t.Fatalf("Expecting ProbeResponse_ACTION_Deny for user %s path %s, received %v ", "user1", path, resp.GetAction())
		}
	}

}

func rotate(t *testing.T, clientID string, dut *ondatra.DUTDevice, version, policy string, creationTime uint64) error {
	t.Logf("Starting second rotate stream (%v)", clientID)
	rotateStream, err := dut.RawAPIs().GNSI(t).Authz().Rotate(context.Background())
	if err == nil {
		autzRotateReq := &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   version,
				CreatedOn: creationTime,
				Policy:    policy,
			},
		}
		t.Logf("Sending Authz.Rotate Upload request on device (%s) : \n %v", clientID, autzRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
		if err == nil {
			t.Logf("The upload rotate request is sent successfully (%s)", clientID)
			_, err = rotateStream.Recv()
			return err
		}
	} else {
		return err
	}
	return nil
}

func TestTwoInterLeavingRotates(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Performing Authz.Rotate request on device %s", dut.Name())
	policy := authz.NewAuthorizationPolicy()
	policy.Get(t, dut)
	jsonPolicy, err := policy.Marshal()
	if err != nil {
		t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
	}
	version := fmt.Sprintf("v0.%v", (time.Now().UnixMilli()))

	secondRotate := func(t *testing.T) {
		err := rotate(t, "Second Client", dut, version, string(jsonPolicy), uint64(time.Now().UnixMicro()))
		if status.Code(err) != codes.Unavailable {
			t.Fatalf("Expecting failure with error code Unavailable, received error %v", err)
		}
	}

	rotateStream, err := dut.RawAPIs().GNSI(t).Authz().Rotate(context.Background())
	if err != nil {
		t.Fatalf("Could not start rotate stream %v", err)
	}
	defer rotateStream.CloseSend()

	t.Run("Perform Second rotate after starting stream", secondRotate)
	autzRotateReq := &authzpb.RotateAuthzRequest_UploadRequest{
		UploadRequest: &authzpb.UploadRequest{
			Version:   fmt.Sprintf("v0.%v", (time.Now().UnixMilli())),
			CreatedOn: uint64(time.Now().UnixMicro()),
			Policy:    string(jsonPolicy),
		},
	}
	t.Logf("Sending Authz.Rotate request on device (client 1): \n %v", autzRotateReq)
	err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
	if err == nil {
		t.Run("Perform Second rotate after uploading request, and before starting receive", secondRotate)
		t.Logf("Authz.Rotate upload was successful, receiving response ...")
		_, err = rotateStream.Recv()
		if err != nil {
			t.Fatalf("Error while receiving rotate request reply %v", err)
		}
		t.Run("Perform Second rotate after receiving upload result and before sending finalize", secondRotate)
		tempPolicy := authz.NewAuthorizationPolicy()
		tempPolicy.Get(t, dut)
		if !cmp.Equal(policy, tempPolicy) {
			t.Fatalf("Policy after upload (temporary) is not the same as the one upload, diff is: %v", cmp.Diff(policy, tempPolicy))
		}
		finalizeRotateReq := &authzpb.RotateAuthzRequest_FinalizeRotation{FinalizeRotation: &authzpb.FinalizeRequest{}}
		t.Logf("Sending Authz.Rotate FinalizeRotation request: \n%v", finalizeRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: finalizeRotateReq})
		if err != nil {
			t.Fatalf("Error while finalizing rotate request  %v", err)
		}
		t.Logf("Authz.Rotate FinalizeRotation is successful (First Client)")

		t.Run("Perform Second rotate after sending finalizing request", secondRotate)
	} else {
		t.Fatalf("Error while uploading prob request reply %v", err)
	}

	finalPolicy := authz.NewAuthorizationPolicy()
	finalPolicy.Get(t, dut)
	if !cmp.Equal(policy, finalPolicy) {
		t.Fatalf("Policy after upload (temporary) is not the same as the one upload, diff is: %v", cmp.Diff(policy, finalPolicy))
	}
}

func TestSingleRotateCompetingClients(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	threadNum := 10
	for i := 0; i <= 100; i++ {
		var wg sync.WaitGroup
		var successfulStream uint64
		wg.Add(threadNum)
		startRotateStream := func() {
			rotateStream, err := dut.RawAPIs().GNSI(t).Authz().Rotate(context.Background())
			if err == nil {
				atomic.AddUint64(&successfulStream, 1)
			}
			time.Sleep(1 * time.Second)
			wg.Done()
			defer rotateStream.CloseSend()
		}
		for j := 1; j <= threadNum; j++ {
			go startRotateStream()
		}
		wg.Wait()
		if successfulStream > 1 {
			t.Fatalf("More than one rotate stream can be started which is not expected, number of open streams is %v", successfulStream)
		}
		if successfulStream == 0 {
			t.Fatal("One stream is expected to be successful")
		}
	}
}

func TestScalePolicy(t *testing.T) {
	t.SkipNow()
	dut := ondatra.DUT(t, "dut")
	policyBefore := authz.NewAuthorizationPolicy()
	policyBefore.Get(t, dut)
	defer policyBefore.Rotate(t, dut)
	// create n users
	users := []*authz.User{}
	for i := 1; i <= usersCount; i++ {
		user := &authz.User{
			Name: "user" + strconv.Itoa(i),
		}
		users = append(users, user)
	}
	createUsersOnDevice(t, dut, users)
	// TODO: create n intermediate CA per user
	// TODO: create users certificate
	// create m random rules for n users(no conflicting)
}

func TestScalePolicyWithFailOver(t *testing.T) {
	t.Skip()
}

// findProcessByName uses telemetry to collect and return the process information. It return nill if the process is not found.
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
func TestHAEMSDProcessKill(t *testing.T) {
	// Process Restart EMSD Test Case with Pre and Post Trigger Policy Verification

	// Pre-Trigger Section
	dut := ondatra.DUT(t, "dut")
	policyBefore := authz.NewAuthorizationPolicy()
	policyBefore.Get(t, dut)
	t.Logf("Authz Policy of the Device %s before the Trigger is %s", dut.Name(), policyBefore.PrettyPrint())

	// Trigger Section
	pName := "emsd"
	ctx := context.Background()
	proc := findProcessByName(ctx, t, dut, pName)
	pid := uint32(proc.GetPid())
	killResponse, err := dut.RawAPIs().GNOI(t).System().KillProcess(context.Background(), &gnps.KillProcessRequest{Name: pName, Pid: pid, Restart: true, Signal: gnps.KillProcessRequest_SIGNAL_TERM})
	t.Logf("Got kill process response: %v\n\n", killResponse)
	if err != nil {
		t.Fatalf("Failed to execute gNOI Kill Process, error received: %v", err)
	}
	time.Sleep(30 * time.Second)
	newProc := findProcessByName(ctx, t, dut, pName)
	if newProc == nil {
		t.Logf("Retry to get the process emsd info after restart")
		time.Sleep(30 * time.Second)
		if newProc = findProcessByName(ctx, t, dut, "emsd"); newProc == nil {
			t.Fatalf("Failed to start process emsd after failure")
		}
	}
	if newProc.GetPid() == proc.GetPid() {
		t.Fatalf("The process id of %s is expected to be changed after the restart", pName)
	}
	if newProc.GetStartTime() <= proc.GetStartTime() {
		t.Fatalf("The start time of process emsd is expected to be larger than %d, got %d ", proc.GetStartTime(), newProc.GetStartTime())
	}

	// Verification Section
	policyAfter := authz.NewAuthorizationPolicy()
	policyAfter.Get(t, dut)
	t.Logf("Authz Policy of the device %s after the Trigger is %s", dut.Name(), policyAfter.PrettyPrint())
	if !cmp.Equal(policyBefore, policyAfter) {
		t.Fatalf("Not Expecting Policy Mismatch before and after the Trigger):\n%s", cmp.Diff(policyBefore, policyAfter))
	}
}

func TestHAFailOverInSteadyState(t *testing.T) {
	// RPFO Test Case with Pre and Post Trigger Policy Verification

	// Pre-Trigger Section
	dut := ondatra.DUT(t, "dut")
	policyBefore := authz.NewAuthorizationPolicy()
	policyBefore.Get(t, dut)
	t.Logf("Authz Policy of the Device %s before the Trigger is %s", dut.Name(), policyBefore.PrettyPrint())

	// Trigger Section
	gnoiClient := dut.RawAPIs().GNOI(t)
	rebootRequest := &gnps.RebootRequest{
		Method: gnps.RebootMethod_COLD,
		Force:  true,
	}
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	t.Logf("Got Reboot response: %v, err: %v", rebootResponse, err)
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since reboot started.", time.Since(startReboot).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}
	}

	// Verification Section
	policyAfter := authz.NewAuthorizationPolicy()
	policyAfter.Get(t, dut)
	t.Logf("Authz Policy of the device %s after the Trigger is %s", dut.Name(), policyAfter.PrettyPrint())
	if !cmp.Equal(policyBefore, policyAfter) {
		t.Fatalf("Not Expecting Policy Mismatch before and after the Trigger):\n%s", cmp.Diff(policyBefore, policyAfter))
	}
}

func TestHAFailOverDuringRotate(t *testing.T) {
	// RPFO Test Case during a On-Going Rotate Request with Pre and Post Trigger Policy Verification
	// Pre-Trigger Section

	dut := ondatra.DUT(t, "dut")
	t.Logf("Performing Authz.Rotate request on device %s", dut.Name())
	policy := authz.NewAuthorizationPolicy()
	policy.Get(t, dut)
	jsonPolicy, err := policy.Marshal()
	if err != nil {
		t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
	}
	version := fmt.Sprintf("v0.%v", (time.Now().UnixMilli()))

	rotateStream, err := dut.RawAPIs().GNSI(t).Authz().Rotate(context.Background())
	if err != nil {
		t.Fatalf("Could not start rotate stream %v", err)
	}
	defer rotateStream.CloseSend()
	defer policy.Rotate(t, dut)
	autzRotateReq := &authzpb.RotateAuthzRequest_UploadRequest{
		UploadRequest: &authzpb.UploadRequest{
			Version:   version,
			CreatedOn: uint64(time.Now().UnixMicro()),
			Policy:    string(jsonPolicy),
		},
	}
	t.Logf("Sending Authz.Rotate request on device (client 1): \n %v", autzRotateReq)
	err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
	if err != nil {
		t.Fatalf("Error while uploading prob request reply %v", err)
	}
	// Get the temporary policy
	tempPolicy := authz.NewAuthorizationPolicy()
	tempPolicy.Get(t, dut)

	// Check that the temporary policy is different from the original policy
	if cmp.Equal(policy, tempPolicy) {
		t.Fatalf("Temporary policy is the same as the original policy")
	}

	// Trigger Section
	// Simulate a RP failover by closing the stream.
	rotateStream.CloseSend()
	gnoiClient := dut.RawAPIs().GNOI(t)
	rebootRequest := &gnps.RebootRequest{
		Method: gnps.RebootMethod_COLD,
		Force:  true,
	}
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	t.Logf("Got Reboot response: %v, err: %v", rebootResponse, err)
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		var currentTime string
		t.Logf("Time elapsed %.2f seconds since reboot started.", time.Since(startReboot).Seconds())
		time.Sleep(30 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}
	}

	// Send a finalize rotate message.
	finalizeRotateReq := &authzpb.RotateAuthzRequest_FinalizeRotation{FinalizeRotation: &authzpb.FinalizeRequest{}}
	t.Logf("Sending Authz.Rotate FinalizeRotation request: \n%v", finalizeRotateReq)
	err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: finalizeRotateReq})
	if err == nil {
		t.Fatalf("Finalize rotation request should have been rejected")
	}

	// Verification Section
	// Check that the policy has not been rotated.
	finalPolicy := authz.NewAuthorizationPolicy()
	finalPolicy.Get(t, dut)
	if cmp.Equal(policy, finalPolicy) {
		t.Logf("Policy has not been rotated after RP failover")
	} else {
		t.Fatalf("Policy has been rotated after RP failover")
	}
}

func TestHARedundancySwithOver(t *testing.T) {
	// RP SwitchOver Reload Test Case with Pre and Post Trigger Policy Verification

	// Pre-Trigger Section
	dut := ondatra.DUT(t, "dut")
	policyBefore := authz.NewAuthorizationPolicy()
	policyBefore.Get(t, dut)
	t.Logf("Authz Policy of the Device %s before the Trigger is %s", dut.Name(), policyBefore.PrettyPrint())

	// Trigger Section
	config.CMDViaGNMI(context.Background(), t, dut, "redundancy switchover \n")
	time.Sleep(30 * time.Second)
	gnmi.Update(t, dut, gnmi.OC().System().Hostname().Config(), "test")

	// Verification Section
	policyAfter := authz.NewAuthorizationPolicy()
	policyAfter.Get(t, dut)
	t.Logf("Authz Policy of the device %s after the Trigger is %s", dut.Name(), policyAfter.PrettyPrint())
	if !cmp.Equal(policyBefore, policyAfter) {
		t.Fatalf("Not Expecting Policy Mismatch before and after the Trigger):\n%s", cmp.Diff(policyBefore, policyAfter))
	}
}
