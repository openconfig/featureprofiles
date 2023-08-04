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
	"github.com/openconfig/featureprofiles/internal/fptest"
	authzpb "github.com/openconfig/gnsi/authz"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"

	"github.com/openconfig/featureprofiles/internal/cisco/security/authz"
	"github.com/openconfig/featureprofiles/internal/cisco/security/gnxi"
	"github.com/openconfig/ondatra"
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
	policyBerfore := authz.NewAuthorizationPolicy()
	policyBerfore.Get(t, dut)
	defer policyBerfore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	authzPolicy.AddAllowRules(users, []*gnxi.RPC{gnxi.RPCs.GNMI_SET})
	authzPolicy.Rotate(t, dut)

}

func TestSaclePolicy(t *testing.T) {
	t.SkipNow()
	dut := ondatra.DUT(t, "dut")
	policyBerfore := authz.NewAuthorizationPolicy()
	policyBerfore.Get(t, dut)
	defer policyBerfore.Rotate(t, dut)
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

	// add a few conflicting rules per users
	// add * rules
	// user *
	// path *
	// both * *  (deny and allow)

}

func TestAllowRuleAll(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	policyBerfore := authz.NewAuthorizationPolicy()
	policyBerfore.Get(t, dut)
	defer policyBerfore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	authzPolicy.AddAllowRules(users, []*gnxi.RPC{gnxi.RPCs.ALL})
	authzPolicy.Rotate(t, dut)
	gnsiClient := dut.RawAPIs().GNSI().Default(t)
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
			t.Fatalf("Expecting ProbeResponse_ACTION_Permit for user %s path %s, received %v ", "user1", path, resp.GetAction())
		}
	}
}

func TestDenyRuleAll(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	policyBerfore := authz.NewAuthorizationPolicy()
	policyBerfore.Get(t, dut)
	defer policyBerfore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	authzPolicy.AddDenyRules(users, []*gnxi.RPC{gnxi.RPCs.ALL})
	authzPolicy.Rotate(t, dut)
	gnsiClient := dut.RawAPIs().GNSI().Default(t)
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
	policyBerfore := authz.NewAuthorizationPolicy()
	policyBerfore.Get(t, dut)
	defer policyBerfore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	gnsiClient := dut.RawAPIs().GNSI().Default(t)

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
	policyBerfore := authz.NewAuthorizationPolicy()
	policyBerfore.Get(t, dut)
	defer policyBerfore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	gnsiClient := dut.RawAPIs().GNSI().Default(t)

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
	policyBerfore := authz.NewAuthorizationPolicy()
	policyBerfore.Get(t, dut)
	defer policyBerfore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	gnsiClient := dut.RawAPIs().GNSI().Default(t)

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
			t.Fatalf("Not expecting error for prob request %v", err)
		}
		if resp.GetAction() != authzpb.ProbeResponse_ACTION_PERMIT {
			t.Fatalf("Expecting ProbeResponse_ACTION_Permit for user %s path %s, received %v ", "user1", path, resp.GetAction())
		}
	}

}

func TestDenyAllRPCs(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	policyBerfore := authz.NewAuthorizationPolicy()
	policyBerfore.Get(t, dut)
	defer policyBerfore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	gnsiClient := dut.RawAPIs().GNSI().Default(t)

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
	policyBerfore := authz.NewAuthorizationPolicy()
	policyBerfore.Get(t, dut)
	defer policyBerfore.Rotate(t, dut)

	authzPolicy := authz.NewAuthorizationPolicy()
	authzPolicy.Get(t, dut)
	users := []*authz.User{}
	users = append(users, &authz.User{Name: sampleUser})
	createUsersOnDevice(t, dut, users)
	gnsiClient := dut.RawAPIs().GNSI().Default(t)

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
	rotateStream, err := dut.RawAPIs().GNSI().Default(t).Authz().Rotate(context.Background())
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

	rotateStream, err := dut.RawAPIs().GNSI().Default(t).Authz().Rotate(context.Background())
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
			rotateStream, err := dut.RawAPIs().GNSI().Default(t).Authz().Rotate(context.Background())
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

func TestFailOverInSteadyState(t *testing.T) {
	t.Skip()
}

func TestFailOverDuringProb(t *testing.T) {
	t.Skip()
}

func TestSaclePolicyWithFailOver(t *testing.T) {
	t.Skip()
}
