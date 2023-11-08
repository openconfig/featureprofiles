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
	"encoding/json"
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
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/testt"
	"github.com/openconfig/ygot/ygot"

	gnps "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
)

type spiffe struct {
	spiffeID string
	// tls config that is created using ca bundle and svid of the user
	tslConf *tls.Config
}

var (
	testInfraID = flag.String("test_infra_id", "cafyauto", "test Infra ID user for authz operation")
	caCertPem   = flag.String("ca_cert_pem", "testdata/ca.cert.pem", "a pem file for ca cert that will be used to generate svid")
	commonName  = flag.Bool("common_name", true, "a pem file for ca cert that will be used to generate svid")
	caKeyPem    = flag.String("ca_key_pem", "testdata/ca.key.pem", "a pem file for ca key that will be used to generate svid")
	usersMap    = map[string]spiffe{
		"cert_user_admin": {
			spiffeID: "spiffe://test-abc.foo.bar/xyz/admin",
		},
		"cert_user_fake": {
			spiffeID: "spiffe://test-abc.foo.bar/xyz/fake",
		},
		"cert_gribi_modify": {
			spiffeID: "spiffe://test-abc.foo.bar/xyz/gribi-modify",
		},
		"cert_gnmi_set": {
			spiffeID: "spiffe://test-abc.foo.bar/xyz/gnmi-set",
		},
		"cert_gnoi_time": {
			spiffeID: "spiffe://test-abc.foo.bar/xyz/gnoi-time",
		},
		"cert_gnoi_ping": {
			spiffeID: "spiffe://test-abc.foo.bar/xyz/gnoi-ping",
		},
		"cert_gnsi_probe": {
			spiffeID: "spiffe://test-abc.foo.bar/xyz/gnsi-probe",
		},
		"cert_read_only": {
			spiffeID: "spiffe://test-abc.foo.bar/xyz/read-only",
		},
	}
)

const (
	maxRebootTime   = 900
	maxCompWaitTime = 600
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func loadPolicyFromJsonFile(t *testing.T, dut *ondatra.DUTDevice, file_path string) []authz.AuthorizationPolicy {

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
	return policies
}

func createUser(t *testing.T, dut *ondatra.DUTDevice, user string) {
	password := "1234"
	ocUser := &oc.System_Aaa_Authentication_User{
		Username: ygot.String(user),
		Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
		Password: ygot.String(password),
	}
	gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().User(user).Config(), ocUser)
}

func setUpUsers(t *testing.T, dut *ondatra.DUTDevice) {
	createUser(t, dut, "cisco")
	caKey, tructBundle, err := svid.LoadKeyPair(*caKeyPem, *caCertPem)
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
		userSvid, err := svid.GenSVID("", v.spiffeID, 300, tructBundle, caKey, x509.RSA)
		if *commonName {
			userSvid, err = svid.GenSVID(v.spiffeID, v.spiffeID, 300, tructBundle, caKey, x509.RSA)
		}
		if err != nil {
			t.Fatalf("Could not generate svid for user %s: %v", user, err)
		}
		tlsConf := tls.Config{
			Certificates: []tls.Certificate{*userSvid},
			RootCAs:      trusBundle,
		}
		usersMap[user] = spiffe{
			spiffeID: v.spiffeID,
			tslConf:  &tlsConf,
		}
	}
}

type access struct {
	allowed []*gnxi.RPC
	denied  []*gnxi.RPC
}
type authorizationTable struct {
	table map[string]access
}

// verifyPerm takes an authorizationTable and verify the expected rpc/acess
func (a *authorizationTable) verifyAuthorization(t *testing.T, dut *ondatra.DUTDevice) {
	for certName, access := range a.table {
		t.Run(fmt.Sprintf("Validating access for user %s", certName), func(t *testing.T) {
			for _, allowedRPC := range access.allowed {
				authz.Verify(t, dut, getSpiffeID(t, dut, certName), allowedRPC, getTlsConfig(t, dut, certName), false, true)
			}
			for _, deniedRPC := range access.denied {
				authz.Verify(t, dut, getSpiffeID(t, dut, certName), deniedRPC, getTlsConfig(t, dut, certName), true, true)
			}
		})
	}
}

// getPolicyByName Gets the authorization policy with the specified name.
func getPolicyByName(t *testing.T, policyName string, policies []authz.AuthorizationPolicy) authz.AuthorizationPolicy {
	for _, policy := range policies {
		if policy.Name == policyName {
			return policy
		}
	}
	t.Fatalf("Requested policy %s is not found", policyName)
	return authz.AuthorizationPolicy{}
}

func getSpiffeID(t *testing.T, dut *ondatra.DUTDevice, certName string) string {
	user, ok := usersMap[certName]
	if !ok {
		t.Fatalf("Could not find Spiffe ID for user %s", certName)
	}
	return user.spiffeID
}

func getTlsConfig(t *testing.T, dut *ondatra.DUTDevice, certName string) *tls.Config {
	user, ok := usersMap[certName]
	if !ok {
		t.Fatalf("Could not find Spiffe ID for user %s", certName)
	}
	return user.tslConf
}

// Authz-1, Test policy behaviors, and probe results matches actual client results.
func TestAuthz1(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	setUpUsers(t, dut)
	tlsCertAdmin := getTlsConfig(t, dut, "cert_user_admin")
	spiffeCertAdmin := getSpiffeID(t, dut, "cert_user_admin")
	t.Run("Authz-1.1, - Test empty source", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, dut, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-everyone-can-gnmi-not-gribi", policies)
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(100), "policy-everyone-can-gnmi-not-gribi_v1", false)

		// Verification of Policy for cert_user_admin is allowed gNMI Get and denied gRIBI Get
		t.Run("Verification of Policy for cert_user_admin is allowed gNMI Get and denied gRIBI Get", func(t *testing.T) {
			authz.Verify(t, dut, spiffeCertAdmin, gnxi.RPCs.GRIBI_GET, tlsCertAdmin, true, true)
			authz.Verify(t, dut, spiffeCertAdmin, gnxi.RPCs.GNMI_GET, tlsCertAdmin, false, true)
		})

	})

	t.Run("Authz-1.2, Test Empty Request", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, dut, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-everyone-can-gribi-not-gnmi", policies)
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(100), "policy-everyone-can-gribi-not-gnmi_v1", false)

		// ensure   `cert_user_fake` is denied to issue `gNMI.Get` method.
		// ensure   `cert_user_fake` is denied to issue `gRIBI.Get` method.
		// ensure  `cert_user_admin` is allowed to issue `gRIBI.Get` method.
		// ensure `cert_user_admin` is denied to issue `gNMI.Get` method.
		t.Run("Verification of cert_user_fake and cert_user_admin premissions", func(t *testing.T) {
			if false {
				// TODO: Clarification
				// fake user will be rejected due to wrong svid during hard verification,
				// but the prob will return true due to allow all permission for gribi get
				authz.Verify(t, dut, getSpiffeID(t, dut, "cert_user_fake"), gnxi.RPCs.GRIBI_GET, nil, true, false)
				authz.Verify(t, dut, getSpiffeID(t, dut, "cert_user_fake"), gnxi.RPCs.GNMI_GET, nil, true, false)
			}
			authz.Verify(t, dut, spiffeCertAdmin, gnxi.RPCs.GRIBI_GET, tlsCertAdmin, false, true)
			authz.Verify(t, dut, spiffeCertAdmin, gnxi.RPCs.GNMI_GET, tlsCertAdmin, true, true)
		})
	})

	tlsCertReadOnly := getTlsConfig(t, dut, "cert_read_only")
	spiffeCertReadOnly := getSpiffeID(t, dut, "cert_read_only")
	t.Run("Authz-1.3, Test that there can only be One policy", func(t *testing.T) {
		// Pre-Test Section
		dut := ondatra.DUT(t, "dut")
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate - 1
		policies := loadPolicyFromJsonFile(t, dut, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-gribi-get", policies)
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(100), "policy-gribi-get_v1", false)

		// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
		authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GRIBI_GET, tlsCertReadOnly, false, true)
		authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GNMI_GET, tlsCertReadOnly, true, true)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate - 2
		newpolicy = getPolicyByName(t, "policy-gnmi-get", policies)
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Verification of Policy for read-only to deny gRIBI Get and allow gNMI Get
		t.Run("Verification of Policy for read-only to deny gRIBI Get and allow gNMI Get", func(t *testing.T) {
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GRIBI_GET, tlsCertReadOnly, true, true)
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GNMI_GET, tlsCertReadOnly, false, true)
		})
	})

	t.Run("Authz-1.4, Test Normal Policy", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, dut, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-normal-1", policies)
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(100), "policy-normal-1_v1", false)

		// Verify all results match per the above table for policy policy-normal-1
		authTable := authorizationTable{
			table: map[string]access{
				"cert_user_admin": struct {
					allowed []*gnxi.RPC
					denied  []*gnxi.RPC
				}{
					allowed: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY, gnxi.RPCs.GNMI_GET,
						gnxi.RPCs.GNOI_SYSTEM_TIME, gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE,
						gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE, gnxi.RPCs.GNMI_SET},
				},
				"cert_user_fake": {
					denied: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY, gnxi.RPCs.GNMI_SET, gnxi.RPCs.GNMI_GET,
						gnxi.RPCs.GNOI_SYSTEM_TIME, gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE,
						gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE},
				},
				"cert_gribi_modify": {
					denied: []*gnxi.RPC{gnxi.RPCs.GNMI_SET, gnxi.RPCs.GNMI_GET,
						gnxi.RPCs.GNOI_SYSTEM_TIME, gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE,
						gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE},
					allowed: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY},
				},
				"cert_gnmi_set": {
					denied: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY,
						gnxi.RPCs.GNOI_SYSTEM_TIME, gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE,
						gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE},
					allowed: []*gnxi.RPC{gnxi.RPCs.GNMI_GET, gnxi.RPCs.GNMI_SET},
				},
				"cert_gnoi_time": {
					denied: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY, gnxi.RPCs.GNMI_SET,
						gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE, gnxi.RPCs.GNMI_GET,
						gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE},
					allowed: []*gnxi.RPC{gnxi.RPCs.GNOI_SYSTEM_TIME},
				},
				"cert_gnoi_ping": {
					denied: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY, gnxi.RPCs.GNMI_SET,
						gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE, gnxi.RPCs.GNMI_GET,
						gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE, gnxi.RPCs.GNOI_SYSTEM_TIME},
					allowed: []*gnxi.RPC{gnxi.RPCs.GNOI_SYSTEM_PING},
				},
				"cert_gnsi_probe": {
					denied: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY, gnxi.RPCs.GNMI_SET,
						gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE, gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNMI_GET,
						gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNOI_SYSTEM_TIME},
					allowed: []*gnxi.RPC{gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE},
				},
				"cert_read_only": {
					denied: []*gnxi.RPC{gnxi.RPCs.GRIBI_MODIFY, gnxi.RPCs.GNMI_SET,
						gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE, gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNOI_SYSTEM_TIME,
						gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE},
					allowed: []*gnxi.RPC{gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GNMI_GET},
				},
			},
		}
		authTable.verifyAuthorization(t, dut)
	})
}

// Authz-2, Test rotation behavior

func TestAuthz2(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	setUpUsers(t, dut)
	tlsCertUserAdmin := getTlsConfig(t, dut, "cert_user_admin")
	spiffeUserAdmin := getSpiffeID(t, dut, "cert_user_admin")
	tlsCertReadOnly := getTlsConfig(t, dut, "cert_read_only")
	spiffeCertReadOnly := getSpiffeID(t, dut, "cert_read_only")
	t.Run("Authz-2.1, Test only one rotation request at a time", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, dut, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-everyone-can-gnmi-not-gribi", policies)
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
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
		newpolicy = getPolicyByName(t, "policy-everyone-can-gnmi-not-gribi", policies)
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
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
			authz.Verify(t, dut, spiffeUserAdmin, gnxi.RPCs.GNMI_GET, tlsCertUserAdmin, false, true)
			authz.Verify(t, dut, spiffeUserAdmin, gnxi.RPCs.GRIBI_GET, tlsCertUserAdmin, true, true)
		})

	})

	t.Run("Authz-2.2, Authz-2.2, Test Rollback When Connection Closed", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, dut, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-gribi-get", policies)
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
		t.Run("Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get", func(t *testing.T) {
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GRIBI_GET, tlsCertReadOnly, false, true)
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GNMI_GET, tlsCertReadOnly, true, true)
		})

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		newpolicy = getPolicyByName(t, "policy-gnmi-get", policies)
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
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
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GRIBI_GET, tlsCertReadOnly, true, true)
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GNMI_GET, tlsCertReadOnly, false, true)
		})

		// Close the Stream
		rotateStream.CloseSend()

		// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
		t.Run("Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get after closing stream", func(t *testing.T) {
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GRIBI_GET, tlsCertReadOnly, false, true)
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GNMI_GET, tlsCertReadOnly, true, true)
		})

	})

	t.Run("Authz-2.3, Test Rollback on Invalid Policy", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, dut, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-gribi-get", policies)
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
		t.Run("Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get", func(t *testing.T) {
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GRIBI_GET, tlsCertReadOnly, false, true)
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GNMI_GET, tlsCertReadOnly, true, true)
		})

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		t.Run("Applying policy-invalid-no-allow-rules", func(t *testing.T) {
			newpolicy = getPolicyByName(t, "policy-invalid-no-allow-rules", policies)
			newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
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
				authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GRIBI_GET, tlsCertReadOnly, true, true)
			})

			// Close the Stream
			rotateStream.CloseSend()
			// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
			t.Run("Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get after closing stream", func(t *testing.T) {
				authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GRIBI_GET, tlsCertReadOnly, false, true)
				authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GNMI_GET, tlsCertReadOnly, true, true)
			})
		})

	})
	t.Run("Authz-2.4, Test Force_Overwrite when the Version does not change", func(t *testing.T) {
		// Pre-Test Section
		_, policyBefore := authz.Get(t, dut)
		t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
		defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

		// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
		policies := loadPolicyFromJsonFile(t, dut, "testdata/policy.json")
		newpolicy := getPolicyByName(t, "policy-gribi-get", policies)
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
		// Rotate the policy.
		prevVersion := fmt.Sprintf("v0.%v", (time.Now().UnixNano()))
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), prevVersion, false)

		newpolicy = getPolicyByName(t, "policy-gnmi-get", policies)
		newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
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
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GRIBI_GET, tlsCertReadOnly, false, false)
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GNMI_GET, tlsCertReadOnly, true, false)
		})

		t.Logf("Preforming Rotate with the same version with force overwrite\n")
		newpolicy.Rotate(t, dut, uint64(time.Now().UnixMilli()), prevVersion, true)
		// Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get
		t.Run("Verification of Policy for read_only to allow gRIBI Get and to deny gNMI Get after rotate wth force overwrite", func(t *testing.T) {
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GRIBI_GET, tlsCertReadOnly, true, false)
			authz.Verify(t, dut, spiffeCertReadOnly, gnxi.RPCs.GNMI_GET, tlsCertReadOnly, false, false)
		})

	})
}

// Authz-3 Test Get Behavior
func TestAuthz3(t *testing.T) {
	// Pre-Test Section
	dut := ondatra.DUT(t, "dut")
	setUpUsers(t, dut)
	_, policyBefore := authz.Get(t, dut)
	t.Logf("Authz Policy of the Device %s before the Rotate Trigger is %s", dut.Name(), policyBefore.PrettyPrint())
	defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

	// Fetch the Desired Authorization Policy object.
	policies := loadPolicyFromJsonFile(t, dut, "testdata/policy.json")
	newpolicy := getPolicyByName(t, "policy-gribi-get", policies)
	// Attach base Admin Policy
	// Rotate the policy.
	newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
	expCreatedOn := uint64(time.Now().UnixMilli())
	expVersion := fmt.Sprintf("v0.%v", (time.Now().UnixNano()))
	newpolicy.Rotate(t, dut, expCreatedOn, expVersion, false)
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
	defer policyBefore.Rotate(t, dut, uint64(time.Now().UnixMilli()), fmt.Sprintf("v0.%v", (time.Now().UnixNano())), false)

	// Fetch the Desired Authorization Policy and Attach base Admin Policy Before Rotate
	policies := loadPolicyFromJsonFile(t, dut, "testdata/policy.json")
	newpolicy := getPolicyByName(t, "policy-normal-1", policies)
	newpolicy.AddAllowRules("base", []string{*testInfraID}, []*gnxi.RPC{gnxi.RPCs.ALL})
	expCreatedOn := uint64(time.Now().UnixMilli())
	expVersion := fmt.Sprintf("v0.%v", (time.Now().UnixNano()))
	t.Logf("New Authz Policy is %s", newpolicy.PrettyPrint())
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
	authTable := authorizationTable{
		table: map[string]access{
			"cert_user_admin": struct {
				allowed []*gnxi.RPC
				denied  []*gnxi.RPC
			}{
				allowed: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY, gnxi.RPCs.GNMI_GET,
					gnxi.RPCs.GNOI_SYSTEM_TIME, gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE,
					gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE, gnxi.RPCs.GNMI_SET},
			},
			"cert_user_fake": {
				denied: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY, gnxi.RPCs.GNMI_SET, gnxi.RPCs.GNMI_GET,
					gnxi.RPCs.GNOI_SYSTEM_TIME, gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE,
					gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE},
			},
			"cert_gribi_modify": {
				denied: []*gnxi.RPC{gnxi.RPCs.GNMI_SET, gnxi.RPCs.GNMI_GET,
					gnxi.RPCs.GNOI_SYSTEM_TIME, gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE,
					gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE},
				allowed: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY},
			},
			"cert_gnmi_set": {
				denied: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY,
					gnxi.RPCs.GNOI_SYSTEM_TIME, gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE,
					gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE},
				allowed: []*gnxi.RPC{gnxi.RPCs.GNMI_GET, gnxi.RPCs.GNMI_SET},
			},
			"cert_gnoi_time": {
				denied: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY, gnxi.RPCs.GNMI_SET,
					gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE, gnxi.RPCs.GNMI_GET,
					gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE},
				allowed: []*gnxi.RPC{gnxi.RPCs.GNOI_SYSTEM_TIME},
			},
			"cert_gnoi_ping": {
				denied: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY, gnxi.RPCs.GNMI_SET,
					gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE, gnxi.RPCs.GNMI_GET,
					gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE, gnxi.RPCs.GNOI_SYSTEM_TIME},
				allowed: []*gnxi.RPC{gnxi.RPCs.GNOI_SYSTEM_PING},
			},
			"cert_gnsi_probe": {
				denied: []*gnxi.RPC{gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GRIBI_MODIFY, gnxi.RPCs.GNMI_SET,
					gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE, gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNMI_GET,
					gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GNOI_SYSTEM_TIME},
				allowed: []*gnxi.RPC{gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE},
			},
			"cert_read_only": {
				denied: []*gnxi.RPC{gnxi.RPCs.GRIBI_MODIFY, gnxi.RPCs.GNMI_SET,
					gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_ROTATE, gnxi.RPCs.GNOI_SYSTEM_PING, gnxi.RPCs.GNOI_SYSTEM_TIME,
					gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_PROBE},
				allowed: []*gnxi.RPC{gnxi.RPCs.GNSI_AUTHZ_V1_AUTHZ_GET, gnxi.RPCs.GRIBI_GET, gnxi.RPCs.GNMI_GET},
			},
		},
	}
	authTable.verifyAuthorization(t, dut)
}
