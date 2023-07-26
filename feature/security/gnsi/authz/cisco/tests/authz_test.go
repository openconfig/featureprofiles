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

// Package mixed_oc_cli_origin_support_test implements GNMI 1.12 from go/wbb:vendor-testplan
package authz_test

import (
	//"context"
	"testing"
	//"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"

	//"github.com/openconfig/gnsi/authz"
	authz "github.com/openconfig/featureprofiles/internal/cisco/security/authz"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}
var (
	sampleUser="user1"
)

func TestSimpleAuthzGet(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	authzPolicy:= authz.NewAuthorizationPolicy()
	authzPolicy.Get(t,dut)
	t.Logf("Authz Policy of the device %s is %s", dut.Name(),authzPolicy.PrettyPrint())
}

func createUsers(t *testing.T, dut *ondatra.DUTDevice,users ...authz.User) {
	for _,user := range users {
		ocUser := &oc.System_Aaa_Authentication_User{
			Username: ygot.String(user.Name),
			Role: oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
			Password:ygot.String("123456"),
		}
		gnmi.Replace(t,dut, gnmi.OC().System().Aaa().Authentication().User(user.Name).Config(),ocUser)
		gnmi.Get(t,dut,gnmi.OC().System().Aaa().Authentication().User(user.Name).State())
	}
}

func TestSimpleRotate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	authzPolicy:= authz.NewAuthorizationPolicy()
	authzPolicy.Get(t,dut)
	authzPolicy.Get(t,dut)
	users:=[]authz.User{}
	users = append(users, authz.User{Name: "user1"})
	createUsers(t,dut,users...)
	authzPolicy.AddAllowRules(users,*authz.RPCs.GNMI_GET)
	authzPolicy.Rotate(t,dut)

}

func TestAllowRuleAll(t *testing.T) {

}



func TestDenyRuleAll(t *testing.T) {

}


func TestDenyAllForService(t *testing.T) {

}

func TestAllowAllForService(t *testing.T) {

}

func TestAllowAallRPCs(t *testing.T) {

}

func TestAllowAllRPCs(t *testing.T) {
	
}
func TestDenyOverWriteAllow(t *testing.T) {

}


func TestRotateIsSingleton(t *testing.T) {

}

func TestFailOverInSteadyState(t *testing.T) {

}

func TestFailOverDuringProb(t *testing.T) {

}

func TestSaclePolicy(t *testing.T) {

}


func TestSaclePolicyWithFailOver(t *testing.T) {

}







/*func authzRotate(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dut.ServiceAddress("GNMI")
	routateStream, _ := dut.RawAPIs().GNSI().New(t).Authz().Rotate(context.Background())
	policy := "{ \n    \"name\": \"authz\", \n    \"allow_rules\": [ \n        { \n            \"name\": \"Admin rules\", \n            \"source\": { \"principals\": [\"cafyauto\"]}, \n            \"request\": { \"paths\": [ \"*\" ] } \n        }\n    ] \n}\n"
	err := routateStream.Send(&authz.RotateAuthzRequest{RotateRequest: &authz.RotateAuthzRequest_UploadRequest{UploadRequest: &authz.UploadRequest{Policy: policy, Version: "1.0", CreatedOn: uint64(time.Now().UnixMicro())}}})
	if err == nil {
		_, err2 := routateStream.Recv()
		if err2 != nil {
			t.Fatalf("Error while uploading prob request %v", err2)
		}
		//upresp:=&authz.RotateAuthzResponse_UploadResponse{}
		//if resp.RotateResponse== upresp {
		err3 := routateStream.Send(&authz.RotateAuthzRequest{RotateRequest: &authz.RotateAuthzRequest_FinalizeRotation{FinalizeRotation: &authz.FinalizeRequest{}}})
		if err3 != nil {
			t.Fatalf("Error while finalizing rotate request  %v", err2)
		}
		//}
	}
}
func TestAuthz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	address, _:= dut.ServiceAddress("GNMI")
	t.Logf("gnmi address is: %s", address)
	probReq := &authz.ProbeRequest{User: "cafyauto", Rpc: "/gnmi.gNMI/Set"}
	resp, err := dut.RawAPIs().GNSI().New(t).Authz().Get(context.Background(), &authz.GetRequest{})
	if err != nil {
		t.Fatalf("Get request %v is failed with error  %v ", probReq, err)
	}
	t.Logf("resp is %v", resp)

	probresp, err := dut.RawAPIs().GNSI().New(t).Authz().Probe(context.Background(), probReq)
	if err != nil {
		t.Fatalf("Prob request %v is failed with error  %v ", probReq, err)
	}
	t.Logf("resp is %v", probresp)
	authzRotate(t)
}*/

// TestOpenConfigBeforeCLI pushes overlapping mixed SetRequest specifying OpenConfig before CLI for DUT port-1.
