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

	"crypto/tls"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/security/gnxi"
	"github.com/openconfig/gnsi/authz"
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

// Spiffe is an struct to save an Spiffe id and its svid.
type Spiffe struct {
	// ID store Spiffe id.
	ID string
	// TlsConf stores the svid of Spiffe id.
	TLSConf *tls.Config
}

// AuthorizationPolicy is an struct to save an authz policy.
type AuthorizationPolicy struct {
	// name of policy.
	Name string `json:"name"`
	// rules that specify what are allowed by users.
	AllowRules []Rule `json:"allow_rules,omitempty"`
	// rules that specify what are denied for users.
	DenyRules []Rule `json:"deny_rules,omitempty"`
}

// Rule represent the structure for an authz rule.
type Rule struct {
	// name of the rule.
	Name string `json:"name"`
	// the users that rule defined for.
	Source struct {
		Principals []string `json:"principals"`
	} `json:"source"`
	// rpc for which the rule is specified.
	Request struct {
		Paths []string `json:"paths"`
	} `json:"request"`
}

func createRule(name string, users []string, rpcs []*gnxi.RPC) Rule {
	rule := Rule{Name: name}
	for _, rpc := range rpcs {
		rule.Request.Paths = append(rule.Request.Paths, rpc.Path)
	}
	rule.Source.Principals = users
	return rule
}

// AddAllowRules adds an allow rule for policy p.
func (p *AuthorizationPolicy) AddAllowRules(name string, users []string, rpcs []*gnxi.RPC) {
	rule := createRule(name, users, rpcs)
	p.AllowRules = append(p.AllowRules, rule)
}

// AddDenyRules adds an allow rule for policy p.
func (p *AuthorizationPolicy) AddDenyRules(name string, users []string, rpcs []*gnxi.RPC) {
	rule := createRule(name, users, rpcs)
	p.DenyRules = append(p.DenyRules, rule)
}

// Unmarshal unmarshal policy p to json string.
func (p *AuthorizationPolicy) Unmarshal(jsonString string) error {
	return json.Unmarshal([]byte(jsonString), p)
}

// Marshal marshal a policy from json string.
func (p *AuthorizationPolicy) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

// Rotate apply policy p on device dut, this is test api for positive testing and it fails the test on failure.
func (p *AuthorizationPolicy) Rotate(t *testing.T, dut *ondatra.DUTDevice, createdOn uint64, version string, forcOverwrite bool) {
	t.Logf("Performing Authz.Rotate request on device %s", dut.Name())
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
	if err != nil {
		t.Fatalf("Could not connect gnsi %v", err)
	}
	rotateStream, err := gnsiC.Authz().Rotate(context.Background())
	if err != nil {
		t.Fatalf("Could not start a rotate stream %v", err)
	}
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
	err = rotateStream.Send(&authz.RotateAuthzRequest{RotateRequest: autzRotateReq, ForceOverwrite: forcOverwrite})
	if err != nil {
		t.Fatalf("Error while uploading prob request reply %v", err)
	}
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
	_, finalPolicy := Get(t, dut)
	if !cmp.Equal(p, finalPolicy) {
		t.Fatalf("Policy after upload (temporary) is not the same as the one upload, diff is: %v", cmp.Diff(p, finalPolicy))
	}

}

// NewAuthorizationPolicy creates an empty policy.
func NewAuthorizationPolicy(name string) *AuthorizationPolicy {
	return &AuthorizationPolicy{
		Name: name,
	}
}

// Get read the applied policy from device dut. this is test api and fails the test when it fails.
func Get(t testing.TB, dut *ondatra.DUTDevice) (*authz.GetResponse, *AuthorizationPolicy) {
	t.Logf("Performing Authz.Get request on device %s", dut.Name())
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
	if err != nil {
		t.Fatalf("Could not connect gnsi %v", err)
	}
	resp, err := gnsiC.Authz().Get(context.Background(), &authz.GetRequest{})
	if err != nil {
		t.Fatalf("Authz.Get request is failed on device %s: %v", dut.Name(), err)
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
func (p *AuthorizationPolicy) PrettyPrint(t *testing.T) string {
	prettyTex, err := json.MarshalIndent(p, "", "    ")
	if err != nil {
		t.Logf("PrettyPrint of an authz policy is failed due to err: %v", err)
		return ""
	}
	return string(prettyTex)
}

type verifyOpt interface {
	isVerifyOpt()
}

// ExceptDeny is passed to verify function when failure is expected.
type ExceptDeny struct {
}

// HardVerify is passed to verify function when verification
// is carried out via execution on the RPC using the user svid.
type HardVerify struct {
}

func (o *ExceptDeny) isVerifyOpt() {}
func (o *HardVerify) isVerifyOpt() {}

// Verify uses prob to validate if the user access for a certain rpc is expected.
// It also execute the rpc when HardVerif is passed and verifies if it matches the expectation.
func Verify(t testing.TB, dut *ondatra.DUTDevice, spiffe *Spiffe, rpc *gnxi.RPC, opts ...verifyOpt) {
	expectedRes := authz.ProbeResponse_ACTION_PERMIT
	expectedExecErr := codes.OK
	hardVerify := false
	for _, opt := range opts {
		switch opt.(type) {
		case *ExceptDeny:
			expectedRes = authz.ProbeResponse_ACTION_DENY
			expectedExecErr = codes.PermissionDenied
		case *HardVerify:
			hardVerify = true
		default:
			t.Errorf("Invalid option is passed to Verify function: %T", opt)
		}
	}
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
	if err != nil {
		t.Fatalf("Could not connect gnsi %v", err)
	}
	resp, err := gnsiC.Authz().Probe(context.Background(), &authz.ProbeRequest{User: spiffe.ID, Rpc: rpc.Path})
	if err != nil {
		t.Fatalf("Prob Request %s failed on dut %s", prettyPrint(&authz.ProbeRequest{User: spiffe.ID, Rpc: rpc.Path}), dut.Name())
	}

	if resp.GetAction() != expectedRes {
		t.Fatalf("Prob response is not expected for user %s and path %s on dut %s, want %v, got %v", spiffe.ID, rpc.Path, dut.Name(), expectedRes, resp.GetAction())
	}
	if hardVerify {
		opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(spiffe.TLSConf))}
		err := rpc.Exec(context.Background(), dut, opts)
		if status.Code(err) != expectedExecErr {
			if status.Code(err) == codes.Unimplemented {
				t.Fatalf("The execution of rpc %s is failed due to error %v, please add implementation for the rpc", rpc.Path, err)
			}
			t.Fatalf("The execution result of of rpc %s for user %s on dut %s is unexpected, want %v, got %v", rpc.Path, spiffe.ID, dut.Name(), expectedExecErr, err)
		}
		t.Logf("The execution of rpc %s for user %s on dut %v is finished as expected, want error: %v, got error: %v ", rpc.Path, spiffe.ID, dut.Name(), expectedExecErr, err)
	}
}

// LoadPolicyFromJSONFile Loads Policy from a JSON File.
func LoadPolicyFromJSONFile(t *testing.T, filePath string) map[string]AuthorizationPolicy {
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Not expecting error while opening policy file %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var policies []AuthorizationPolicy
	err1 := decoder.Decode(&policies)
	if err1 != nil {
		t.Fatalf("Not expecting error while decoding policy %v", err)
	}
	policyMap := map[string]AuthorizationPolicy{}
	for _, policy := range policies {
		policyMap[policy.Name] = policy
	}
	return policyMap
}
