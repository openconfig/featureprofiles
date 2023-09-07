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

// Package authz provides helper APIs to simplify writing authz test cases.
// It also packs authz rotate and get operations with the corresponding verifications to
// prevent code duplications and increase the test code readability.

package authz

import (
	"context"

	"encoding/json"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/security/gnxi"
	"github.com/openconfig/gnsi/authz"
	"github.com/openconfig/ondatra"
)

// AuthorizationPolicy is an struct to save an authz policy
type AuthorizationPolicy struct {
	// name of policy
	Name string `json:"name"`
	// rules that specify what are allowed by users
	AllowRules []Rule `json:"allow_rules,omitempty"`
	// rules that specify what are denied for users
	DenyRules []Rule `json:"deny_rules,omitempty"`
}

type Rule struct {
	// name of the rule
	Name string `json:"name"`
	// the users that rule defined for
	Source struct {
		Principals []string `json:"principals"`
	} `json:"source"`
	// rpc for which the rule is specified
	Request struct {
		Paths []string `json:"paths"`
	} `json:"request"`
}

func createRule(users []string, rpcs []*gnxi.RPC) Rule {
	rule := Rule{}
	for _, rpc := range rpcs {
		rule.Name = rule.Name + rpc.FQN
		rule.Request.Paths = append(rule.Request.Paths, rpc.Path)
	}
	rule.Source.Principals = append(rule.Source.Principals, users...)
	return rule
}

// AddAllowRules adds an allow rule for policy p
func (p *AuthorizationPolicy) AddAllowRules(users []string, rpcs []*gnxi.RPC) {
	rule := createRule(users, rpcs)
	p.AllowRules = append(p.AllowRules, rule)
}

// AddDenyRules adds an allow rule for policy p
func (p *AuthorizationPolicy) AddDenyRules(users []string, rpcs []*gnxi.RPC) {
	rule := createRule(users, rpcs)
	p.DenyRules = append(p.DenyRules, rule)
}

// Unmarshal unmarshal policy p to json string
func (p *AuthorizationPolicy) Unmarshal(jsonString string) error {
	return json.Unmarshal([]byte(jsonString), p)
}

// Marshal marshal a policy from json string
func (p *AuthorizationPolicy) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

// Rotate apply policy p on device dut, this is test api for positive testing and it fails the test on failure.
func (p *AuthorizationPolicy) Rotate(t *testing.T, dut *ondatra.DUTDevice, createdOn uint64, version string) {
	t.Logf("Performing Authz.Rotate request on device %s", dut.Name())
	rotateStream, _ := dut.RawAPIs().GNSI().Default(t).Authz().Rotate(context.Background())
	defer rotateStream.CloseSend()
	policy, err := p.Marshal()
	if err != nil {
		t.Fatalf("Could not marshal the policy %s", prettyPrint(policy))
	}
	autzRotateReq := &authz.RotateAuthzRequest_UploadRequest{
		UploadRequest: &authz.UploadRequest{
			Version:   version,
			CreatedOn: createdOn,
			Policy:    string(policy),
		},
	}
	t.Logf("Sending Authz.Rotate request on device: \n %s", prettyPrint(autzRotateReq))
	err = rotateStream.Send(&authz.RotateAuthzRequest{RotateRequest: autzRotateReq})
	if err == nil {
		t.Logf("Authz.Rotate upload was successful, receiving response ...")
		_, err = rotateStream.Recv()
		if err != nil {
			t.Fatalf("Error while receiving rotate request reply %v", err)
		}
		// validate Result
		_, tempPolicy := Get(t, dut)
		if !cmp.Equal(p, tempPolicy) {
			t.Fatalf("Policy after upload (temporary) is not the same as the one upload, diff is: %v", cmp.Diff(p, tempPolicy))
		}
		finalizeRotateReq := &authz.RotateAuthzRequest_FinalizeRotation{FinalizeRotation: &authz.FinalizeRequest{}}
		err = rotateStream.Send(&authz.RotateAuthzRequest{RotateRequest: finalizeRotateReq})
		t.Logf("Sending Authz.Rotate FinalizeRotation request: \n%s", prettyPrint(finalizeRotateReq))
		if err != nil {
			t.Fatalf("Error while finalizing rotate request  %v", err)
		}
	} else {
		t.Fatalf("Error while uploading prob request reply %v", err)
	}
	_, finalPolicy := Get(t, dut)
	if !cmp.Equal(p, finalPolicy) {
		t.Fatalf("Policy after upload (temporary) is not the same as the one upload, diff is: %v", cmp.Diff(p, finalPolicy))
	}

}

// NewAuthorizationPolicy creates an empty policy
func NewAuthorizationPolicy() *AuthorizationPolicy {
	return &AuthorizationPolicy{}
}

// Get read the applied policy from device dut. this is test api and fails the test when it fails.
func Get(t testing.TB, dut *ondatra.DUTDevice) (*authz.GetResponse, *AuthorizationPolicy) {
	t.Logf("Performing Authz.Get request on device %s", dut.Name())
	gnsiC := dut.RawAPIs().GNSI().Default(t)
	resp, err := gnsiC.Authz().Get(context.Background(), &authz.GetRequest{})
	if err != nil {
		t.Fatalf("Authz.Get request is failed on device %s", dut.Name())
	}

	t.Logf("Authz.Get response is %s", prettyPrint(resp))
	if resp.GetVersion() == "" {
		t.Errorf("Version is not set in Authz.Get response")
	}
	if resp.GetCreatedOn() > uint64(time.Now().UnixMicro()) {
		t.Errorf("CreatedOn value can not be larger than current time")
	}
	p := &AuthorizationPolicy{}
	err = p.Unmarshal(resp.Policy)
	if err != nil {
		t.Fatalf("Authz.Get response contains invalid policy %s", resp.GetPolicy())
	}
	return resp, p
}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

// PrettyPrint prints policy p in a pretty format.
func (p *AuthorizationPolicy) PrettyPrint() string {
	prettyTex, err := json.MarshalIndent(p, "", "    ")
	if err != nil {
		glog.Warningf("PrettyPrint of an authz policy is failed due to err: %v", err)
		return ""
	}
	return string(prettyTex)
}
