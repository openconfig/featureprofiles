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
// TODO: Streaming Validation will be done in the subsequent PR
package authz_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/authz"
	"github.com/openconfig/featureprofiles/internal/security/gnxi"
	"github.com/openconfig/featureprofiles/internal/security/svid"
	gnps "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/testt"
)

const (
	maxRebootTime   = 900 // Unit is Seconds
	maxCompWaitTime = 600
)

type UsersMap map[string]authz.Spiffe

var (
	testInfraID = flag.String("test_infra_id", "cafyauto", "SPIFFE-ID used by test Infra ID user for authz operation")
	caCertPem   = flag.String("ca_cert_pem", "testdata/ca.cert.pem", "a pem file for ca cert that will be used to generate svid")
	caKeyPem    = flag.String("ca_key_pem", "testdata/ca.key.pem", "a pem file for ca key that will be used to generate svid")
	policyMap   map[string]authz.AuthorizationPolicy

	usersMap = UsersMap{
		"cert_user_admin": {
			ID: "spiffe://test-abc.foo.bar/xyz/admin",
		},
		"cert_deny_all": {
			ID: "spiffe://test-abc.foo.bar/xyz/deny-all", // a user with valid svid but no permission (has a deny rule for target *)
		},
		"cert_gribi_modify": {
			ID: "spiffe://test-abc.foo.bar/xyz/gribi-modify",
		},
		"cert_gnmi_set": {
			ID: "spiffe://test-abc.foo.bar/xyz/gnmi-set",
		},
		"cert_gnoi_time": {
			ID: "spiffe://test-abc.foo.bar/xyz/gnoi-time",
		},
		"cert_gnoi_ping": {
			ID: "spiffe://test-abc.foo.bar/xyz/gnoi-ping",
		},
		"cert_gnsi_probe": {
			ID: "spiffe://test-abc.foo.bar/xyz/gnsi-probe",
		},
		"cert_read_only": {
			ID: "spiffe://test-abc.foo.bar/xyz/read-only",
		},
	}
)

type access struct {
	allowed []*gnxi.RPC
	denied  []*gnxi.RPC
}

type authorizationTable map[string]access

var authTable = authorizationTable{
	//table: map[string]access{
	"cert_user_admin": struct {
		allowed []*gnxi.RPC
		denied  []*gnxi.RPC
	}{
		allowed: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify, gnxi.RPCs.GnmiGet,
			gnxi.RPCs.GnoiSystemTime, gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnsiAuthzRotate,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnsiAuthzProbe, gnxi.RPCs.GnmiSet},
	},
	"cert_deny_all": {
		denied: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify, gnxi.RPCs.GnmiSet, gnxi.RPCs.GnmiGet,
			gnxi.RPCs.GnoiSystemTime, gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnsiAuthzRotate,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnsiAuthzProbe},
	},
	"cert_gribi_modify": {
		denied: []*gnxi.RPC{gnxi.RPCs.GnmiSet, gnxi.RPCs.GnmiGet,
			gnxi.RPCs.GnoiSystemTime, gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnsiAuthzRotate,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnsiAuthzProbe},
		allowed: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify},
	},
	"cert_gnmi_set": {
		denied: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify,
			gnxi.RPCs.GnoiSystemTime, gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnsiAuthzRotate,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnsiAuthzProbe},
		allowed: []*gnxi.RPC{gnxi.RPCs.GnmiGet, gnxi.RPCs.GnmiSet},
	},
	"cert_gnoi_time": {
		denied: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify, gnxi.RPCs.GnmiSet,
			gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnsiAuthzRotate, gnxi.RPCs.GnmiGet,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnsiAuthzProbe},
		allowed: []*gnxi.RPC{gnxi.RPCs.GnoiSystemTime},
	},
	"cert_gnoi_ping": {
		denied: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify, gnxi.RPCs.GnmiSet,
			gnxi.RPCs.GnsiAuthzRotate, gnxi.RPCs.GnmiGet,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnsiAuthzProbe, gnxi.RPCs.GnoiSystemTime},
		allowed: []*gnxi.RPC{gnxi.RPCs.GnoiSystemPing},
	},
	"cert_gnsi_probe": {
		denied: []*gnxi.RPC{gnxi.RPCs.GribiGet, gnxi.RPCs.GribiModify, gnxi.RPCs.GnmiSet,
			gnxi.RPCs.GnsiAuthzRotate, gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnmiGet,
			gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GnoiSystemTime},
		allowed: []*gnxi.RPC{gnxi.RPCs.GnsiAuthzProbe},
	},
	"cert_read_only": {
		denied: []*gnxi.RPC{gnxi.RPCs.GribiModify, gnxi.RPCs.GnmiSet,
			gnxi.RPCs.GnsiAuthzRotate, gnxi.RPCs.GnoiSystemPing, gnxi.RPCs.GnoiSystemTime,
			gnxi.RPCs.GnsiAuthzProbe},
		allowed: []*gnxi.RPC{gnxi.RPCs.GnsiAuthzGet, gnxi.RPCs.GribiGet, gnxi.RPCs.GnmiGet},
	},
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func setUpBaseline(t *testing.T, dut *ondatra.DUTDevice) {

	policyMap = authz.LoadPolicyFromJSONFile(t, "testdata/policy.json")

	caKey, trustBundle, err := svid.LoadKeyPair(*caKeyPem, *caCertPem)
	if err != nil {
		t.Fatalf("Could not load ca key/cert: %v", err)
	}
	caCertBytes, err := os.ReadFile(*caCertPem)
	if err != nil {
		t.Fatalf("Could not load the ca cert: %v", err)
	}
	trusBundle := x509.NewCertPool()
	if !trusBundle.AppendCertsFromPEM(caCertBytes) {
		t.Fatalf("Could not create the trust bundle: %v", err)
	}
	for user, v := range usersMap {
		userSvid, err := svid.GenSVID("", v.ID, 300, trustBundle, caKey, x509.RSA)
		if err != nil {
			t.Fatalf("Could not generate svid for user %s: %v", user, err)
		}
		tlsConf := tls.Config{
			Certificates: []tls.Certificate{*userSvid},
			RootCAs:      trusBundle,
		}
		usersMap[user] = authz.Spiffe{
			ID:      v.ID,
			TLSConf: &tlsConf,
		}
	}
}

// verifyPerm takes an authorization Table and verify the expected rpc/access
func verifyAuthTable(t *testing.T, dut *ondatra.DUTDevice, authTable authorizationTable) {
	for certName, access := range authTable {
		t.Run(fmt.Sprintf("Validating access for user %s", certName), func(t *testing.T) {
			for _, allowedRPC := range access.allowed {
				authz.Verify(t, dut, getSpiffe(t, dut, certName), allowedRPC, &authz.HardVerify{})
			}
			for _, deniedRPC := range access.denied {
				authz.Verify(t, dut, getSpiffe(t, dut, certName), deniedRPC, &authz.ExceptDeny{}, &authz.HardVerify{})
			}
		})
	}
}

func getSpiffe(t *testing.T, dut *ondatra.DUTDevice, certName string) *authz.Spiffe {
	spiffe, ok := usersMap[certName]
	if !ok {
		t.Fatalf("Could not find Spiffe ID for user %s", certName)
	}
	return &spiffe
}

// Authz-1, Test policy behaviors, and probe results matches actual client results.
func TestAuthz1(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	setUpBaseline(t, dut)
	certAdminSpiffe := getSpiffe(t, dut, "cert_user_admin")
	t.Run("Authz-1.1, - Test empty source", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		newpolicy, ok := policyMap["policy-everyone-can-gnmi-not-gribi"]
		if !ok {
			t.Fatal("Policy policy-everyone-can-gnmi-not-gribi is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(100), "policy-everyone-can-gnmi-not-gribi_v1", false)

		// Verification of Policy for cert_user_admin is allowed gNMI Get and denied gRIBI Get
		t.Run("Verification of Policy for cert_user_admin is allowed gNMI Get and denied gRIBI Get", func(t *testing.T) {
			authz.Verify(t, dut, certAdminSpiffe, gnxi.RPCs.GribiGet, &authz.ExceptDeny{}, &authz.HardVerify{})
			authz.Verify(t, dut, certAdminSpiffe, gnxi.RPCs.GnmiGet, &authz.HardVerify{})
		})

	})

	t.Run("Authz-1.2, Test Empty Request", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		newpolicy, ok := policyMap["policy-everyone-can-gribi-not-gnmi"]
		if !ok {
			t.Fatal("policy-everyone-can-gribi-not-gnmi")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(100), "policy-everyone-can-gribi-not-gnmi_v1", false)

		t.Run("Verification of cert_deny_all is denied to issue gRIBI.Get and cert_user_admin is allowed to issue `gRIBI.Get`", func(t *testing.T) {
			authz.Verify(t, dut, getSpiffe(t, dut, "cert_deny_all"), gnxi.RPCs.GnmiGet, &authz.ExceptDeny{}, &authz.HardVerify{})
			authz.Verify(t, dut, certAdminSpiffe, gnxi.RPCs.GribiGet, &authz.HardVerify{})
		})
	})

	readOnlySpiffe := getSpiffe(t, dut, "cert_read_only")
	t.Run("Authz-1.3, Test that there can only be One policy", func(t *testing.T) {
		// Pre-Test Section
		dut := ondatra.DUT(t, "dut")
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate - 1
		newpolicy, ok := policyMap["policy-gribi-get"]
		if !ok {
			t.Fatal("Policy policy-gribi-get is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(100), "policy-gribi-get_v1", false)

		// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
		authz.Verify(t, dut, readOnlySpiffe, gnxi.RPCs.GribiGet, &authz.HardVerify{})
		authz.Verify(t, dut, readOnlySpiffe, gnxi.RPCs.GnmiGet, &authz.ExceptDeny{}, &authz.HardVerify{})

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate - 2
		newpolicy, ok = policyMap["policy-gnmi-get"]
		if !ok {
			t.Fatal("Policy policy-gnmi-get is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Verification of Policy for read-only to deny gRIBI Get and allow gNMI Get
		t.Run("Verification of Policy for read-only to deny gRIBI Get and allow gNMI Get", func(t *testing.T) {
			authz.Verify(t, dut, readOnlySpiffe, gnxi.RPCs.GribiGet, &authz.ExceptDeny{}, &authz.HardVerify{})
			authz.Verify(t, dut, readOnlySpiffe, gnxi.RPCs.GnmiGet, &authz.HardVerify{})
		})
	})

	t.Run("Authz-1.4, Test Normal Policy", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		newpolicy, ok := policyMap["policy-normal-1"]
		if !ok {
			t.Fatal("Policy policy-normal-1 is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(100), "policy-normal-1_v1", false)

		// Verify all results match per the above table for policy policy-normal-1
		verifyAuthTable(t, dut, authTable)
	})
}

// Authz-2, Test rotation behavior
func TestAuthz2(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	setUpBaseline(t, dut)
	spiffeUserAdmin := getSpiffe(t, dut, "cert_user_admin")
	spiffeCertReadOnly := getSpiffe(t, dut, "cert_read_only")
	t.Run("Authz-2.1, Test only one rotation request at a time", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		newpolicy, ok := policyMap["policy-everyone-can-gnmi-not-gribi"]
		if !ok {
			t.Fatal("Policy policy-everyone-can-gnmi-not-gribi is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		jsonPolicy, err := newpolicy.Marshal()
		if err != nil {
			t.Fatalf("Could not marshal the policy %s", string(jsonPolicy))
		}
		// Rotate Request 1
		rotateStream, err := dut.RawAPIs().GNSI(t).Authz().Rotate(context.Background())
		if err != nil {
			t.Fatalf("Could not start rotate stream %v", err)
		}
		defer rotateStream.CloseSend()
		autzRotateReq := &authzpb.RotateAuthzRequest_UploadRequest{
			UploadRequest: &authzpb.UploadRequest{
				Version:   fmt.Sprintf("v0.%v", (time.Now().UnixNano())),
				CreatedOn: uint64(time.Now().UnixMilli()),
				Policy:    string(jsonPolicy),
			},
		}
		t.Logf("Sending Authz.Rotate request on device (client 1): \n %v", autzRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
		if err == nil {
			t.Logf("Authz.Rotate upload (client 1) was successful, receiving response ...")
		}
		_, err = rotateStream.Recv()
		if err != nil {
			t.Fatalf("Error while receiving rotate request reply (client 1) %v", err)
		}
		// Rotate Request 2 - Before Finalizing the Request 1
		newpolicy, ok = policyMap["policy-everyone-can-gnmi-not-gribi"]
		if !ok {
			t.Fatal("Policy policy-everyone-can-gnmi-not-gribi is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
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
				Version:   fmt.Sprintf("v0.%v", (time.Now().UnixNano())),
				CreatedOn: uint64(time.Now().UnixMilli()),
				Policy:    string(jsonPolicy),
			},
		}
		t.Logf("Sending Authz.Rotate request on device (client 2): \n %v", autzRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
		if err == nil {
			t.Logf("Authz.Rotate upload was successful (client 2), receiving response ...")
		}
		_, err = rotateStream.Recv()
		if err == nil {
			t.Fatalf("Second Rotate request (client 2) should be Rejected - Error while receiving rotate request reply %v", err)
		}
		t.Run("Verification of Policy for user_admin to deny gRIBI Get and allow gNMI Get", func(t *testing.T) {
			// Verification of Policy for user_admin to deny gRIBI Get and allow gNMI Get
			authz.Verify(t, dut, spiffeUserAdmin, gnxi.RPCs.GnmiGet, &authz.HardVerify{})
			authz.Verify(t, dut, spiffeUserAdmin, gnxi.RPCs.GribiGet, &authz.ExceptDeny{}, &authz.HardVerify{})
		})

	})

	t.Run("Authz-2.2, Test Rollback When Connection Closed", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		newpolicy, ok := policyMap["policy-gribi-get"]
		if !ok {
			t.Fatal("Policy policy-gribi-get is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
		t.Run("Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get", func(t *testing.T) {
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GribiGet, &authz.HardVerify{})
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GnmiGet, &authz.ExceptDeny{}, &authz.HardVerify{})
		})

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		newpolicy, ok = policyMap["policy-gnmi-get"]
		if !ok {
			t.Fatal("Policy policy-gnmi-get is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
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
				Version:   fmt.Sprintf("v0.%v", (time.Now().UnixNano())),
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
		// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
		t.Run("Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get after rotate that is not finalized", func(t *testing.T) {
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GribiGet, &authz.ExceptDeny{}, &authz.HardVerify{})
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GnmiGet, &authz.HardVerify{})
		})

		// Close the Stream
		rotateStream.CloseSend()

		// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
		t.Run("Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get after closing stream", func(t *testing.T) {
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GribiGet, &authz.HardVerify{})
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GnmiGet, &authz.ExceptDeny{}, &authz.HardVerify{})
		})

	})

	t.Run("Authz-2.3, Test Rollback on Invalid Policy", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		newpolicy, ok := policyMap["policy-gribi-get"]
		if !ok {
			t.Fatal("Policy policy-gribi-get is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
		t.Run("Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get", func(t *testing.T) {
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GribiGet, &authz.HardVerify{})
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GnmiGet, &authz.ExceptDeny{}, &authz.HardVerify{})
		})

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		t.Run("Applying policy-invalid-no-allow-rules", func(t *testing.T) {
			newpolicy, ok = policyMap["policy-invalid-no-allow-rules"]
			if !ok {
				t.Fatal("Policy policy-invalid-no-allow-rules is not loaded from policy json file")
			}
			newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
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
					Version:   fmt.Sprintf("v0.%v", (time.Now().UnixNano())),
					CreatedOn: uint64(time.Now().UnixMilli()),
					Policy:    string(jsonPolicy),
				},
			}
			t.Logf("Sending Authz.Rotate request on device: \n %v", autzRotateReq)
			err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
			if err == nil {
				t.Logf("Authz.Rotate upload was successful, receiving response ...")
			}
			_, err = rotateStream.Recv()
			if err != nil {
				t.Fatalf("Expected Error while receiving rotate request reply %v", err)
			}

			t.Run("Verification of Policy for read_only user to deny gRIBI Get before closing stream", func(t *testing.T) {
				authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GribiGet, &authz.ExceptDeny{}, &authz.HardVerify{})
			})

			// Close the Stream
			rotateStream.CloseSend()
			// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
			t.Run("Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get after closing stream", func(t *testing.T) {
				authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GribiGet, &authz.HardVerify{})
				authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GnmiGet, &authz.ExceptDeny{}, &authz.HardVerify{})
			})
		})

	})
	t.Run("Authz-2.4, Test Force_Overwrite when the Version does not change", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		newpolicy, ok := policyMap["policy-gribi-get"]
		if !ok {
			t.Fatal("Policy policy-gribi-get is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
		// Rotate the policy.
		prevVersion := fmt.Sprintf("v0.%v", (time.Now().UnixNano()))
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), prevVersion, false)

		newpolicy, ok = policyMap["policy-gnmi-get"]
		if !ok {
			t.Fatal("Policy policy-gnmi-get is not loaded from policy json file")
		}
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
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
				Version:   prevVersion,
				CreatedOn: uint64(time.Now().UnixMilli()),
				Policy:    string(jsonPolicy),
			},
		}
		t.Logf("Sending Authz.Rotate request with the same version on device: \n %v", autzRotateReq)
		err = rotateStream.Send(&authzpb.RotateAuthzRequest{RotateRequest: autzRotateReq})
		if err == nil {
			t.Logf("Authz.Rotate upload was successful, receiving response ...")
		}
		_, err = rotateStream.Recv()
		if err == nil {
			t.Fatalf("Expected Error for uploading the policy with the same version as the previous one")
		}
		// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
		t.Run("Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get after rotate without force overwrite", func(t *testing.T) {
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GribiGet, &authz.HardVerify{})
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GnmiGet, &authz.ExceptDeny{}, &authz.HardVerify{})
		})

		t.Logf("Preforming Rotate with the same version with force overwrite\n")
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), prevVersion, true)
		// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
		t.Run("Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get after rotate wth force overwrite", func(t *testing.T) {
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GribiGet, &authz.ExceptDeny{}, &authz.HardVerify{})
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GnmiGet, &authz.HardVerify{})
		})

	})
}

// Authz-3 Test Get Behavior
func TestAuthz3(t *testing.T) {
	// Pre-Test Section
	dut := ondatra.DUT(t, "dut")
	setUpBaseline(t, dut)
	_, policyBefore := authz.Get(t, dut)
	t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
	defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

	// Fetch the Desired Authorization Policy object.
	newpolicy, ok := policyMap["policy-gribi-get"]
	if !ok {
		t.Fatal("Policy policy-gribi-get is not loaded from policy json file")
	}
	// Attach base Admin Policy
	// Rotate the policy.
	newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
	expCreatedOn := uint64(time.Now().UnixMilli())
	expVersion := fmt.Sprintf("v0.%v", (time.Now().UnixNano()))
	newpolicy.Rotate(t, dut, expCreatedOn, expVersion, false)
	t.Logf("New Rotated Authz Policy is %s", newpolicy.PrettyPrint(t))
	// Wait for 30s, intial gNSI.Get and validate the value of version, created_on and gRPC policy content does not change.
	time.Sleep(30 * time.Second)
	_, finalPolicy := authz.Get(t, dut)
	t.Logf("Authz Policy after waiting for 30 seconds is %s", finalPolicy.PrettyPrint(t))

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
	t.Logf("Authz Policy of the Device %s before the Reboot Trigger is %s", dut.Name(), policyBefore.PrettyPrint(t))
	defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

	// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
	newpolicy, ok := policyMap["policy-normal-1"]
	if !ok {
		t.Fatal("Policy policy-normal-1 is not loaded from policy json file")
	}
	newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.AllRPC})
	expCreatedOn := uint64(time.Now().UnixMilli())
	expVersion := fmt.Sprintf("v0.%v", (time.Now().UnixNano()))
	t.Logf("New Authz Policy is %s", newpolicy.PrettyPrint(t))
	newpolicy.Rotate(t, dut, expCreatedOn, expVersion, false)

	// Trigger Section - Reboot
	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("Failed to connect to gnoi server, err: %v", err)
	}
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
	for {
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Log("Reboot is started")
			break
		}
		t.Log("Wait for reboot to be started")
		time.Sleep(30 * time.Second)
	}
	startReboot := time.Now()
	t.Logf("Wait for DUT to boot up by polling the telemetry output.")
	for {
		t.Logf("Time elapsed %.2f seconds since reboot started.", time.Since(startReboot).Seconds())
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime = gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("Device rebooted successfully with received time: %v", currentTime)
			break
		}
		if uint64(time.Since(startReboot).Seconds()) > maxRebootTime {
			t.Fatalf("Check boot time: got %v, want < %v", time.Since(startReboot), maxRebootTime)
		}
	}
	// Verification Section
	// Version and Created On Field Verification
	t.Logf("Performing Authz.Get request on device %s", dut.Name())
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
	if err != nil {
		t.Fatalf("Could not create GNSI Connection %v", err)
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
	// Verify all results match per the above table for policy policy-normal-1
	verifyAuthTable(t, dut, authTable)
}
