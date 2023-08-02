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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	authzpb "github.com/openconfig/gnsi/authz"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"

	//"github.com/openconfig/gnsi/authz"
	authz "github.com/openconfig/featureprofiles/internal/cisco/security/authz"
	"github.com/openconfig/featureprofiles/internal/cisco/security/gnxi"
	"github.com/openconfig/ondatra"
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

func generateUsersCerts(t *testing.T, dut *ondatra.DUTDevice, users ...authz.User) {
	// TODO generate certificate for all users and save them in testdata folder
	// use cert lib that is created internal/security/cert
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
	t.Skip()
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
	t.Skip()
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
	t.Skip()
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
	t.Skip()
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
	t.Skip()
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
	t.Skip()
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
	t.Skip()
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

func TestRotateIsSingleton(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Performing Authz.Rotate request on device %s", dut.Name())
	policy := authz.NewAuthorizationPolicy()
	policy.Get(t, dut)
	jsonPolicy, err := policy.Marshal()
	if err != nil {
		t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
	}
	mainProbSignal := make(chan string)
	go func(t *testing.T) {
		<-mainProbSignal
		for {
			select {
			case <-mainProbSignal:
				return
			default:
				t.Logf("Starting second rotate stream (Client2)")
				rotateStream2, err := dut.RawAPIs().GNSI().Default(t).Authz().Rotate(context.Background())
				if err == nil {
					autzRotateReq := &authzpb.RotateAuthzRequest_UploadRequest{
						UploadRequest: &authzpb.UploadRequest{
							Version:   fmt.Sprintf("v0.%v", (time.Now().UnixMilli() + 1)), // same version makes things worse, will need to a seperate test for this
							CreatedOn: uint64(time.Now().UnixMicro()),
							Policy:    string(jsonPolicy),
						},
					}
					t.Logf("Sending Second Authz.Rotate Upload request on device (Client2): \n %v", autzRotateReq)
					err = rotateStream2.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
					if err == nil {
						t.Log("A second upload rotate request  is sent successfully (Client2)")
						_, err = rotateStream2.Recv()
						if err == nil {
							t.Error("The second rotate was successful, which is not expected (Client2)", err)
						}
					}
				}
				return
			}
		}
	}(t)

	rotateStream, err := dut.RawAPIs().GNSI().Default(t).Authz().Rotate(context.Background())
	if err != nil {
		t.Fatalf("Could not start rotate stream %v", err)
	}
	defer rotateStream.CloseSend()
	//mainProbSignal <- "start"
	//time.Sleep(2 * time.Second)

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
		mainProbSignal <- "start"
		time.Sleep(2 * time.Second)
		t.Logf("Authz.Rotate upload was successful, receiving response ...")
		_, err = rotateStream.Recv()
		if err != nil {
			t.Fatalf("Error while receiving rotate request reply %v", err)
		}
		time.Sleep(2 * time.Second)
		// validate Result
		tempPolicy := authz.NewAuthorizationPolicy()
		tempPolicy.Get(t, dut)
		if !cmp.Equal(policy, tempPolicy) {
			t.Fatalf("Policy after upload (temporary) is not the same as the one upload, diff is: %v", cmp.Diff(policy, tempPolicy))
		}
		//p.Verify(t,dut, false)
		finalizeRotateReq := &authzpb.RotateAuthzRequest_FinalizeRotation{FinalizeRotation: &authzpb.FinalizeRequest{}}
		t.Logf("Sending Authz.Rotate FinalizeRotation request: \n%v", finalizeRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: finalizeRotateReq})
		if err != nil {
			t.Fatalf("Error while finalizing rotate request  %v", err)
		}
		t.Logf("Authz.Rotate FinalizeRotation is successful (First Client)")
		time.Sleep(2 * time.Second)
	} else {
		t.Fatalf("Error while uploading prob request reply %v", err)
	}
	//validate Result
	finalPolicy := authz.NewAuthorizationPolicy()
	finalPolicy.Get(t, dut)
	if !cmp.Equal(policy, finalPolicy) {
		t.Fatalf("Policy after upload (temporary) is not the same as the one upload, diff is: %v", cmp.Diff(policy, finalPolicy))
	}
	close(mainProbSignal)
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

func TestRotatePolicySingletonSameVersion(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	t.Logf("Performing Authz.Rotate request on device %s", dut.Name())
	policy := authz.NewAuthorizationPolicy()
	policy.Get(t, dut)
	jsonPolicy, err := policy.Marshal()
	if err != nil {
		t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
	}
	version := fmt.Sprintf("v0.%v", (time.Now().UnixMilli()))
	mainProbSignal := make(chan string)
	go func(t *testing.T) {
		<-mainProbSignal
		for {
			select {
			case <-mainProbSignal:
				return
			default:
				t.Logf("Starting second rotate stream (Client2)")
				rotateStream2, err := dut.RawAPIs().GNSI().Default(t).Authz().Rotate(context.Background())
				defer rotateStream2.CloseSend()
				if err == nil {
					autzRotateReq := &authzpb.RotateAuthzRequest_UploadRequest{
						UploadRequest: &authzpb.UploadRequest{
							Version:   version, // same version makes things worse, will need to a seperate test for this
							CreatedOn: uint64(time.Now().UnixMicro()),
							Policy:    string(jsonPolicy),
						},
					}
					t.Logf("Sending Second Authz.Rotate Upload request on device (Client2): \n %v", autzRotateReq)
					err = rotateStream2.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
					if err == nil {
						t.Log("A second upload rotate request  is sent successfully (Client2)")
						_, err = rotateStream2.Recv()
						if err == nil {
							t.Error("The second rotate was successful, which is not expected (Client2)", err)
						}
					}
				}
				return
			}
		}
	}(t)

	rotateStream, err := dut.RawAPIs().GNSI().Default(t).Authz().Rotate(context.Background())
	if err != nil {
		t.Fatalf("Could not start rotate stream %v", err)
	}
	defer rotateStream.CloseSend()
	//mainProbSignal <- "start"
	//time.Sleep(2 * time.Second)

	autzRotateReq := &authzpb.RotateAuthzRequest_UploadRequest{
		UploadRequest: &authzpb.UploadRequest{
			Version:   version,
			CreatedOn: uint64(time.Now().UnixMicro()),
			Policy:    string(jsonPolicy),
		},
	}
	t.Logf("Sending Authz.Rotate request on device (client 1): \n %v", autzRotateReq)
	err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
	if err == nil {
		mainProbSignal <- "start"
		time.Sleep(2 * time.Second)
		t.Logf("Authz.Rotate upload was successful, receiving response ...")
		_, err = rotateStream.Recv()
		if err != nil {
			t.Fatalf("Error while receiving rotate request reply %v", err)
		}
		time.Sleep(2 * time.Second)
		// validate Result
		tempPolicy := authz.NewAuthorizationPolicy()
		tempPolicy.Get(t, dut)
		if !cmp.Equal(policy, tempPolicy) {
			t.Fatalf("Policy after upload (temporary) is not the same as the one upload, diff is: %v", cmp.Diff(policy, tempPolicy))
		}
		//p.Verify(t,dut, false)
		finalizeRotateReq := &authzpb.RotateAuthzRequest_FinalizeRotation{FinalizeRotation: &authzpb.FinalizeRequest{}}
		t.Logf("Sending Authz.Rotate FinalizeRotation request: \n%v", finalizeRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: finalizeRotateReq})
		if err != nil {
			t.Fatalf("Error while finalizing rotate request (First Client) %v", err)
		}
		t.Logf("Authz.Rotate FinalizeRotation is successful (First Client)")
		time.Sleep(2 * time.Second)
	} else {
		t.Fatalf("Error while uploading prob request reply %v", err)
	}
	//validate Result
	finalPolicy := authz.NewAuthorizationPolicy()
	finalPolicy.Get(t, dut)
	if !cmp.Equal(policy, finalPolicy) {
		t.Fatalf("Policy after upload (temporary) is not the same as the one upload, diff is: %v", cmp.Diff(policy, finalPolicy))
	}
	close(mainProbSignal)
}
