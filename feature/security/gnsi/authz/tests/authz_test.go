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

// Package authz_test performs functional tests for authz service
package authz_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/authz"
	"github.com/openconfig/featureprofiles/internal/security/gnxi"
	gnps "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
)

var (
	testInfraID = flag.String("test_infra_id", "cisco", "test Infra ID user for authz operation")
)

const (
	maxRebootTime   = 900
	maxCompWaitTime = 600
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func loadPolicyFromJsonFile(t *testing.T, file_path string) []authz.AuthorizationPolicy {

	// Open the JSON file.
	file, err := os.Open(file_path)
	if err != nil {
		t.Fatalf("Not expecting error while opening policy file %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var policies []authz.AuthorizationPolicy
	err1 := decoder.Decode(&policies)
	if err1 != nil {
		t.Fatalf("Not expecting error while decoding policy %v", err)
	}

	// dut := ondatra.DUT(t, "dut")
	// spiffe_id := deviations.AuthzSpiffeIDUnsupported(dut)
	spiffe_id := true
	if spiffe_id {
		for i, policy := range policies {
			for j, allowRule := range policy.AllowRules {
				allowRule.Source.Principals = removeSpiffeFromPrincipals(allowRule.Source.Principals)
				policies[i].AllowRules[j] = allowRule
			}
			for k, denyRule := range policy.DenyRules {
				denyRule.Source.Principals = removeSpiffeFromPrincipals(denyRule.Source.Principals)
				policies[i].DenyRules[k] = denyRule
			}
		}
	}
	return policies
}

func removeSpiffeFromPrincipals(principals []string) []string {
	// Create a slice to store the new principals.
	newPrincipals := []string{}

	// Iterate through the principals.
	for _, principal := range principals {
		// If the principal starts with "spiffe://", get the text after the last "/".
		if strings.HasPrefix(principal, "spiffe://") {
			newPrincipals = append(newPrincipals, strings.Split(principal, "/")[len(strings.Split(principal, "/"))-1])
		} else {
			// Else principal is not Changed
			newPrincipals = append(newPrincipals, principal)
		}
	}

	// Return the slice of new principals.
	return newPrincipals
}

// func createUsersOnDevice(t *testing.T, dut *ondatra.DUTDevice, users []*authz.User) {
// 	ocAuthentication := &oc.System_Aaa_Authentication{}
// 	for _, user := range users {
// 		ocUser := &oc.System_Aaa_Authentication_User{
// 			Username: ygot.String(user.Name),
// 			Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
// 			Password: ygot.String(password),
// 		}
// 		ocAuthentication.AppendUser(ocUser)
// 	}
// 	gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().Config(), ocAuthentication)
// }

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

type AuthorizationTable struct {
	Method       []string
	AllowedCerts []string
	DeniedCerts  []string
}

func appendMethods(authTable *AuthorizationTable, methods []string) {
	// Check if the methods slice is not nil.
	if methods == nil {
		return
	}
	// Iterate through the methods slice.
	for _, method := range methods {
		// Append the method to the AuthorizationTable struct.
		authTable.Method = append(authTable.Method, method)
	}
}

func iterateAndProbeAllowed(t *testing.T, authTable AuthorizationTable, rpcMap map[string]string) {
	// Iterate through the methods.
	for _, method := range authTable.Method {
		// Iterate through the allowed users.
		for _, allowedUser := range authTable.AllowedCerts {
			// Form a probe request.
			probReq := &authzpb.ProbeRequest{
				User: allowedUser,
				Rpc:  rpcMap[method],
			}
			// Send the probe request and verify the response.
			dut := ondatra.DUT(t, "dut")
			gnsiClient := dut.RawAPIs().GNSI(t)
			resp, err := gnsiClient.Authz().Probe(context.Background(), probReq)
			if err != nil {
				t.Fatalf("Not expecting error for prob request %v", err)
			}
			if resp.GetAction() != authzpb.ProbeResponse_ACTION_PERMIT {
				t.Fatalf("Expecting ProbeResponse_ACTION_Allow for user %s path %s, received %v ", allowedUser, method, resp.GetAction())
			}
		}
	}
}

func iterateAndProbeDenied(t *testing.T, authTable AuthorizationTable, rpcMap map[string]string) {
	// Iterate through the methods.
	for _, method := range authTable.Method {
		// Iterate through the allowed users.
		for _, allowedUser := range authTable.DeniedCerts {
			// Form a probe request.
			probReq := &authzpb.ProbeRequest{
				User: allowedUser,
				Rpc:  rpcMap[method],
			}
			// Send the probe request and verify the response.
			dut := ondatra.DUT(t, "dut")
			gnsiClient := dut.RawAPIs().GNSI(t)
			resp, err := gnsiClient.Authz().Probe(context.Background(), probReq)
			if err != nil {
				t.Fatalf("Not expecting error for prob request %v", err)
			}
			if resp.GetAction() != authzpb.ProbeResponse_ACTION_DENY {
				t.Fatalf("Expecting ProbeResponse_ACTION_DENY for user %s path %s, received %v ", allowedUser, method, resp.GetAction())
			}
		}
	}
}

func getPolicyByName(t *testing.T, policyName string, policies []authz.AuthorizationPolicy) authz.AuthorizationPolicy {
	/*
	   	Get the authorization policy with the specified name.

	   Args:
	   policyName: The name of the policy to get.
	   policies: A list of authorization policies.

	   Returns:
	   The authorization policy with the specified name, or nil if the policy does
	   not exist.
	*/
	var foundPolicy authz.AuthorizationPolicy
	for _, policy := range policies {
		if policy.Name == policyName {
			foundPolicy = policy
		}
	}
	return foundPolicy
}

func FetchUniqueItems(t *testing.T, s []string) []string {
	itemExisted := make(map[string]bool)
	var uniqueList []string
	for _, item := range s {
		if _, ok := itemExisted[item]; !ok {
			itemExisted[item] = true
			uniqueList = append(uniqueList, item)
		} else {
			t.Logf("Detected duplicated item: %v", item)
		}
	}
	return uniqueList
}

// Authz-1, Test policy behaviors, and probe results matches actual client results.
func TestAuthz1(t *testing.T) {
	t.Run("Authz-1.1, - Test empty source", func(t *testing.T) {
		// Pre-Test Section
		dut := ondatra.DUT(t, "dut")
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

		// Fetch the Desired Authorization Policy and Attach Default Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-everyone-can-gnmi-not-gribi", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(100), "policy-everyone-can-gnmi-not-gribi_v1")

		// Verification of Policy for cert_user_admin is allowed gNMI Get and denied gRIBI Get
		authz.Verify(t, dut, "cert_user_admin", gnxi.RPCs.GRIBI_GET, nil, true, false)
		// authz.Verify(t, dut, "cert_user_admin", gnxi.RPCs.GNMI_GET, nil, false, false)
	})

	t.Run("Authz-1.2, Test Empty Request", func(t *testing.T) {
		// Pre-Test Section
		dut := ondatra.DUT(t, "dut")
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

		// Fetch the Desired Authorization Policy and Attach Default Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-everyone-can-gnmi-not-gribi", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(100), "policy-everyone-can-gribi-not-gnmi_v1")

		// Verification of Policy for cert_user_fake to deny gRIBI Get and cert_user_admin to allow gRIBI Get
		authz.Verify(t, dut, "cert_user_fake", gnxi.RPCs.GRIBI_GET, nil, true, false)
		// authz.Verify(t, dut, "cert_user_admin", gnxi.RPCs.GRIBI_GET, nil, false, false)
	})

	t.Run("Authz-1.3, Test that there can only be One policy", func(t *testing.T) {
		// Pre-Test Section
		dut := ondatra.DUT(t, "dut")
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

		// Fetch the Desired Authorization Policy and Attach Default Admin Policy Before Rotate - 1
		policies := loadPolicyFromJsonFile(t, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-gribi-get", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(100), "policy-gribi-get_v1")

		// Verification of Policy for cert_read_only to allow gRIBI Get and to deny gNMI Get
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GRIBI_GET, nil, false, false)
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GNMI_GET, nil, true, false)

		// Fetch the Desired Authorization Policy and Attach Default Admin Policy Before Rotate - 2
		newpolicy = getPolicyByName(t, "policy-gnmi-get", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

		// Verification of Policy for cert_read_only to deny gRIBI Get and allow gNMI Get
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GRIBI_GET, nil, true, false)
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GNMI_GET, nil, false, false)
	})

	t.Run("Authz-1.4, Test Normal Policy", func(t *testing.T) {
		// Pre-Test Section
		dut := ondatra.DUT(t, "dut")
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

		// Fetch the Desired Authorization Policy and Attach Default Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-normal-1", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(100), "policy-normal-1_v1")

		// Verify all results match per the above table for policy policy-normal-1
		authTable := AuthorizationTable{
			Method:       []string{"gRIBI.Get"},
			AllowedCerts: []string{"cert_user_admin", "cert_gribi_modify", "cert_read_only"},
			DeniedCerts:  []string{"cert_user_fake", "cert_gnmi_set", "cert_gnoi_time", "cert_gnoi_ping", "cert_gnsi_probe"},
		}
		// Add more methods to the AuthorizationTable struct.
		authTable.Method = append(authTable.Method, "gRIBI.Set")
		authTable.AllowedCerts = append(authTable.AllowedCerts, "cert_user_admin")
		authTable.DeniedCerts = append(authTable.DeniedCerts, "")
		// Add more methods to the AuthorizationTable struct.
		authTable.Method = append(authTable.Method, "gRIBI.Set")
		authTable.AllowedCerts = append(authTable.AllowedCerts, "cert_user_admin")
		authTable.DeniedCerts = append(authTable.DeniedCerts, "")
		rpcMap := map[string]string{
			"gRIBI.Modify": gnxi.RPCs.GRIBI_MODIFY.Path,
			"gRIBI.Get":    gnxi.RPCs.GRIBI_GET.Path,
			"gNMI.Set":     gnxi.RPCs.GNMI_SET.Path,
			"gNMI.Get":     gnxi.RPCs.GNMI_GET.Path,
			"gNOI.Time":    gnxi.RPCs.GNOI_SYSTEM_TIME.Path,
			"gNOI.Ping":    gnxi.RPCs.GNOI_SYSTEM_PING.Path,
			// "gNSI.Rotate":  gnxi.RPCs.GNSI_AUTHZ_ROTATE.Path,
			// "gNSI.Get":     gnxi.RPCs.GNSI_AUTHZ_GET.Path,
			// "gNSI.Probe":   gnxi.RPCs.GNSI_AUTHZ_PROBE.Path,
		}
		iterateAndProbeAllowed(t, authTable, rpcMap)
	})
}

// Authz-2, Test rotation behavior
func TestAuthz2(t *testing.T) {
	t.Run("Authz-2.1, Test only one rotation request at a time", func(t *testing.T) {
		// Pre-Test Section
		dut := ondatra.DUT(t, "dut")
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

		// Fetch the Desired Authorization Policy and Attach Default Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-everyone-can-gnmi-not-gribi", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		jsonPolicy, err := newpolicy.Marshal()
		// Rotate Request 1
		if err != nil {
			t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
		}
		rotateStream, err := dut.RawAPIs().GNSI(t).Authz().Rotate(context.Background())
		if err != nil {
			t.Fatalf("Could not start rotate stream %v", err)
		}
		defer rotateStream.CloseSend()
		autzRotateReq := &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   fmt.Sprintf("v0.%v", (time.Now().UnixMilli())),
				CreatedOn: uint64(time.Now().UnixMilli()),
				Policy:    string(jsonPolicy),
			},
		}
		t.Logf("Sending Authz.Rotate request on device (client 1): \n %v", autzRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
		if err == nil {
			t.Logf("Authz.Rotate upload was successful, receiving response ...")
		}
		_, err = rotateStream.Recv()
		if err != nil {
			t.Fatalf("Error while receiving rotate request reply %v", err)
		}
		// Rotate Request 2 - Before Finalizing the Request 1
		newpolicy = getPolicyByName(t, "policy-everyone-can-gnmi-not-gribi", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		jsonPolicy, err = newpolicy.Marshal()
		if err != nil {
			t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
		}
		rotateStream, err = dut.RawAPIs().GNSI(t).Authz().Rotate(context.Background())
		if err != nil {
			t.Fatalf("Could not start rotate stream %v", err)
		}
		defer rotateStream.CloseSend()
		autzRotateReq = &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   fmt.Sprintf("v0.%v", (time.Now().UnixMilli())),
				CreatedOn: uint64(time.Now().UnixMilli()),
				Policy:    string(jsonPolicy),
			},
		}
		t.Logf("Sending Authz.Rotate request on device (client 2): \n %v", autzRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
		if err == nil {
			t.Logf("Authz.Rotate upload was successful, receiving response ...")
		}
		_, err = rotateStream.Recv()
		if err == nil {
			t.Fatalf("Second Rotate Request should be Rejected - Error while receiving rotate request reply %v", err)
		}
		// Verification of Policy for cert_user_admin to deny gRIBI Get and allow gNMI Get
		authz.Verify(t, dut, "cert_user_admin", gnxi.RPCs.GNMI_GET, nil, false, false)
		authz.Verify(t, dut, "cert_user_admin", gnxi.RPCs.GRIBI_GET, nil, true, false)
	})

	t.Run("Authz-2.2, Authz-2.2, Test Rollback When Connection Closed", func(t *testing.T) {
		// Pre-Test Section
		dut := ondatra.DUT(t, "dut")
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

		// Fetch the Desired Authorization Policy and Attach Default Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-gribi-get", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

		// Verification of Policy for cert_read_only to allow gRIBI Get and to deny gNMI Get
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GRIBI_GET, nil, false, false)
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GNMI_GET, nil, true, false)

		// Fetch the Desired Authorization Policy and Attach Default Admin Policy Before Rotate
		newpolicy = getPolicyByName(t, "policy-gnmi-get", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		jsonPolicy, err := newpolicy.Marshal()
		if err != nil {
			t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
		}
		rotateStream, err := dut.RawAPIs().GNSI(t).Authz().Rotate(context.Background())
		if err != nil {
			t.Fatalf("Could not start rotate stream %v", err)
		}
		defer rotateStream.CloseSend()
		autzRotateReq := &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   fmt.Sprintf("v0.%v", (time.Now().UnixMilli())),
				CreatedOn: uint64(time.Now().UnixMilli()),
				Policy:    string(jsonPolicy),
			},
		}
		t.Logf("Sending Authz.Rotate request on device (client 2): \n %v", autzRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
		if err == nil {
			t.Logf("Authz.Rotate upload was successful, receiving response ...")
		}
		_, err = rotateStream.Recv()
		if err != nil {
			t.Fatalf("Error while receiving rotate request reply %v", err)
		}
		// Verification of Policy for cert_read_only to allow gRIBI Get and to deny gNMI Get
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GRIBI_GET, nil, true, false)
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GNMI_GET, nil, false, false)

		// Close the Stream
		rotateStream.CloseSend()
		// Verification of Policy for cert_read_only to allow gRIBI Get and to deny gNMI Get
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GRIBI_GET, nil, false, false)
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GNMI_GET, nil, true, false)
	})

	t.Run("Authz-2.3, Test Rollback on Invalid Policy", func(t *testing.T) {
		// Pre-Test Section
		dut := ondatra.DUT(t, "dut")
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

		// Fetch the Desired Authorization Policy and Attach Default Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-gribi-get", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

		// Verification of Policy for cert_read_only to allow gRIBI Get and to deny gNMI Get
		// authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GRIBI_GET, nil, false, false)
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GNMI_GET, nil, true, false)

		// Fetch the Desired Authorization Policy and Attach Default Admin Policy Before Rotate
		newpolicy = getPolicyByName(t, "policy-invalid-no-allow-rules", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		jsonPolicy, err := newpolicy.Marshal()
		if err != nil {
			t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
		}
		rotateStream, err := dut.RawAPIs().GNSI(t).Authz().Rotate(context.Background())
		if err != nil {
			t.Fatalf("Could not start rotate stream %v", err)
		}
		defer rotateStream.CloseSend()
		autzRotateReq := &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   fmt.Sprintf("v0.%v", (time.Now().UnixMilli())),
				CreatedOn: uint64(time.Now().UnixMilli()),
				Policy:    string(jsonPolicy),
			},
		}
		t.Logf("Sending Authz.Rotate request on device (client 2): \n %v", autzRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
		if err == nil {
			t.Logf("Authz.Rotate upload was successful, receiving response ...")
		}
		_, err = rotateStream.Recv()
		if err != nil {
			t.Fatalf("Expected Error while receiving rotate request reply %v", err)
		}

		// Close the Stream
		rotateStream.CloseSend()
		// Verification of Policy for cert_read_only to allow gRIBI Get and to deny gNMI Get
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GRIBI_GET, nil, false, false)
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GNMI_GET, nil, true, false)
	})

	t.Run("Authz-2.4, Test Force_Overwrite when the Version does not change", func(t *testing.T) {
		// Pre-Test Section
		dut := ondatra.DUT(t, "dut")
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

		// Fetch the Desired Authorization Policy and Attach Default Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-gribi-get", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		version := fmt.Sprintf("v0.%v", (time.Now().UnixMilli()))
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), version)

		newpolicy = getPolicyByName(t, "policy-gnmi-get", policies)
		newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		jsonPolicy, err := newpolicy.Marshal()
		if err != nil {
			t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
		}
		rotateStream, err := dut.RawAPIs().GNSI(t).Authz().Rotate(context.Background())
		if err != nil {
			t.Fatalf("Could not start rotate stream %v", err)
		}
		defer rotateStream.CloseSend()
		autzRotateReq := &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   version,
				CreatedOn: uint64(time.Now().UnixMilli()),
				Policy:    string(jsonPolicy),
			},
		}
		t.Logf("Sending Authz.Rotate request on device (client 2): \n %v", autzRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
		if err == nil {
			t.Logf("Authz.Rotate upload was successful, receiving response ...")
		}
		_, err = rotateStream.Recv()
		if err != nil {
			t.Fatalf("Error while receiving rotate request reply %v", err)
		}
		// Verification of Policy for cert_read_only to allow gRIBI Get and to deny gNMI Get
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GRIBI_GET, nil, false, false)
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GNMI_GET, nil, true, false)

		// TODO: Add a force_overwrite Flag
		//newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), version)

		// Verification of Policy for cert_read_only to allow gRIBI Get and to deny gNMI Get
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GRIBI_GET, nil, true, false)
		authz.Verify(t, dut, "cert_read_only", gnxi.RPCs.GNMI_GET, nil, false, false)
	})
}

// Authz-3 Test Get Behavior
func TestAuthz3(t *testing.T) {
	// Pre-Test Section
	dut := ondatra.DUT(t, "dut")
	_, policyBefore := authz.Get(t, dut)
	t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
	defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

	// Fetch the Desired Authorization Policy object.
	policies := loadPolicyFromJsonFile(t, "testdata/policy.json")
	newpolicy := getPolicyByName(t, "policy-gribi-get", policies)
	// Attach Default Admin Policy
	// Rotate the policy.
	newpolicy.AddAllowRules("default", []string{"cisco"}, []*gnxi.RPC{gnxi.RPCs.ALL})
	expCreatedOn := uint64(time.Now().UnixMilli())
	expVersion := fmt.Sprintf("v0.%v", (time.Now().UnixMilli()))
	newpolicy.Rotate(t, dut, expCreatedOn, expVersion)
	t.Logf("New Rotated Authz Policy is %s", newpolicy.PrettyPrint())
	// Wait for 30s, intial gNSI.Get and validate the value of version, created_on and gRPC policy content does not change.
	time.Sleep(30 * time.Second)
	_, finalPolicy := authz.Get(t, dut)
	t.Logf("Authz Policy after waiting for 30 seconds is %s", finalPolicy.PrettyPrint())

	// Version and Created On Field Verification
	t.Logf("Performing Authz.Get request on device %s", dut.Name())
	gnsiC := dut.RawAPIs().GNSI(t)
	resp, err := gnsiC.Authz().Get(context.Background(), &authzpb.GetRequest{})
	if err != nil {
		t.Fatalf("Authz.Get request is failed on device %s", dut.Name())
	}
	t.Logf("Authz.Get response is %s", resp)
	if resp.GetVersion() != expVersion {
		t.Errorf("Version has Changed in Authz.Get response")
	}
	if resp.GetCreatedOn() != expCreatedOn {
		t.Errorf("CreatedOn Value has Changed in Authz.Get response")
	}
	if !cmp.Equal(&newpolicy, finalPolicy) {
		t.Fatalf("Not Expecting Policy Mismatch before and after the Wait):\n%s", cmp.Diff(&newpolicy, finalPolicy))
	}
}

// Authz-4 Reboot Persistent
func TestAuthz4(t *testing.T) {
	// Pre-Test Section
	dut := ondatra.DUT(t, "dut")
	_, policyBefore := authz.Get(t, dut)
	t.Logf("Authz Policy of the Device %s before the Reboot Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
	defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixMilli())))

	// Fetch the Desired Authorization Policy and Attach Default Admin Policy Before Rotate
	policies := loadPolicyFromJsonFile(t, "testdata/policy.json")
	newpolicy := getPolicyByName(t, "policy-normal-1", policies)
	newpolicy.AddAllowRules("default", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
	expCreatedOn := uint64(time.Now().UnixMilli())
	expVersion := fmt.Sprintf("v0.%v", (time.Now().UnixMilli()))
	t.Logf("New Authz Policy is %s", newpolicy.PrettyPrint())
	newpolicy.Rotate(t, dut, expCreatedOn, expVersion)

	// Trigger Section - Reboot
	gnoiClient := dut.RawAPIs().GNOI().New(t)
	rebootRequest := &gnps.RebootRequest{
		Method: gnps.RebootMethod_COLD,
		Force:  true,
	}
	bootTimeBeforeReboot := gnmi.Get(t, dut, gnmi.OC().System().BootTime().State())
	t.Logf("DUT boot time before reboot: %v", bootTimeBeforeReboot)
	var currentTime string
	currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
	t.Logf("Time Before Reboot : %v", currentTime)
	rebootResponse, err := gnoiClient.System().Reboot(context.Background(), rebootRequest)
	t.Logf("Got Reboot response: %v, err: %v", rebootResponse, err)
	if err != nil {
		t.Fatalf("Failed to reboot chassis with unexpected err: %v", err)
	}
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {

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
		if uint64(time.Since(startReboot).Seconds()) > maxRebootTime {
			t.Errorf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	// Verification Section
	// Version and Created On Field Verification
	t.Logf("Performing Authz.Get request on device %s", dut.Name())
	gnsiC, err := dut.RawAPI().DialGNSI(context.Background())
	if err != nil {
		t.Fatal("Could not create GNSI Connection %v", err)
	}
	resp, err := gnsiC.Authz().Get(context.Background(), &authzpb.GetRequest{})
	if err != nil {
		t.Fatalf("Authz.Get request is failed with Error %v", err)
	}
	t.Logf("Authz.Get response is %s", resp)
	if resp.GetVersion() != expVersion {
		t.Errorf("Version has Changed to %v from Expected Version %v after Reboot Trigger", resp.GetVersion(), expVersion)
	}
	if resp.GetCreatedOn() != expCreatedOn {
		t.Errorf("Created On has Changed to %v from Expected Created On %v after Reboot Trigger", resp.GetCreatedOn(), expCreatedOn)
	}
}
