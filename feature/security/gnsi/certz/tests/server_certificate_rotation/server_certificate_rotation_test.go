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

package server_certificate_rotation

import (
	context "context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"testing"
	"time"

	setupService "github.com/openconfig/featureprofiles/feature/security/gnsi/certz/tests/internal/setup_service"
	"github.com/openconfig/featureprofiles/internal/fptest"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	dirPath = "../../test_data/"
)

var (
	testProfile     = "newprofile"
	serverAddr      string
	username        = "certzuser"
	password        = "certzpasswd"
	servers         []string
	expected_result bool
	timeNow         = time.Now().String()
)

// createUser function to add an user in admin role.

func createUser(t *testing.T, dut *ondatra.DUTDevice, user, pswd string) bool {
	ocUser := &oc.System_Aaa_Authentication_User{
		Username: ygot.String(user),
		Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
		Password: ygot.String(pswd),
	}
	res := gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().User(user).Config(), ocUser)
	t.Logf("Update the user configuration:%v", res)
	if res == nil {
		t.Fatalf("Failed to create credentials.")
	}
	return true
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestServerCertRotation tests a server certificate can be rotated by using the gNSI certz Rotate() rpc,
// if the certificate is requested without the device generated CSR.
func TestServerCertRotation(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name()
	if !createUser(t, dut, username, password) {
		t.Fatalf("%s: Failed to create certz user.", timeNow)
	}
	t.Logf("Validation of all services that are using gRPC before certz rotation.")
	if !setupService.PreInitCheck(context.Background(), t, dut) {
		t.Fatalf("%s: Failed in the preInit checks.", timeNow)
	}

	ctx := context.Background()
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(ctx)
	if err != nil {
		t.Fatalf("%s: Failed to create gNSI Connection %v", timeNow, err)
	}
	t.Logf("%s Precheck:gNSI connection is successful %v", timeNow, gnsiC)

	t.Logf("%s:Creation of test data.", timeNow)
	if setupService.CertGeneration(t, dirPath) != nil {
		t.Fatalf("%s:Failed to generate the testdata certificates.", timeNow)
	}
	certzClient := gnsiC.Certz()
	t.Logf("%s Precheck:checking baseline ssl profile list.", timeNow)
	setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{})
	t.Logf("%s:Adding new empty ssl profile ID.", timeNow)
	addProfileResponse, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: testProfile})
	if err != nil {
		t.Fatalf("%s:Add profile request failed with %v! ", timeNow, err)
	}
	t.Logf("%s AddProfileResponse: %v", timeNow, addProfileResponse)
	t.Logf("%s: Getting the ssl profile list after new ssl profile addition.", timeNow)
	setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{})

	cases := []struct {
		desc        string
		serverCert  string
		serverKey   string
		trustBundle string
		clientCert  string
		clientKey   string
		mismatch    bool
		loop        uint16
	}{
		{
			desc:        "Certz3.1:Rotate server-rsa-a certificate/key/trustbundle from ca-01",
			serverCert:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKey:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundle: dirPath + "ca-01/trust_bundle_01_rsa.pem",
			clientCert:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKey:   dirPath + "ca-01/client-rsa-a-key.pem",
			loop:        1,
		},
		{
			desc:        "Certz3.1:Rotate server-rsa-b key and certificate/key/trustbundle from ca-01",
			serverCert:  dirPath + "ca-01/server-rsa-b-cert.pem",
			serverKey:   dirPath + "ca-01/server-rsa-b-key.pem",
			trustBundle: dirPath + "ca-01/trust_bundle_01_rsa.pem",
			clientCert:  dirPath + "ca-01/client-rsa-b-cert.pem",
			clientKey:   dirPath + "ca-01/client-rsa-b-key.pem",
			loop:        2,
		},
		{
			desc:        "Certz3.1:Rotate server-ecdsa-a key and certificate/key/trustbundle from ca-01",
			serverCert:  dirPath + "ca-01/server-ecdsa-a-cert.pem",
			serverKey:   dirPath + "ca-01/server-ecdsa-a-key.pem",
			trustBundle: dirPath + "ca-01/trust_bundle_01_ecdsa.pem",
			clientCert:  dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKey:   dirPath + "ca-01/client-ecdsa-a-key.pem",
			loop:        3,
		},
		{
			desc:        "Certz3.1:Rotate server-ecdsa-b key and certificate/key/trustbundle from ca-01",
			serverCert:  dirPath + "ca-01/server-ecdsa-b-cert.pem",
			serverKey:   dirPath + "ca-01/server-ecdsa-b-key.pem",
			trustBundle: dirPath + "ca-01/trust_bundle_01_ecdsa.pem",
			clientCert:  dirPath + "ca-01/client-ecdsa-b-cert.pem",
			clientKey:   dirPath + "ca-01/client-ecdsa-b-key.pem",
			loop:        4,
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Logf("in loop %v", tc.loop)
			san := setupService.ReadDecodeServerCertificate(t, tc.serverCert)
			serverCert := setupService.CreateCertzChain(t, setupService.CertificateChainRequest{
				RequestType:    setupService.EntityTypeCertificateChain,
				ServerCertFile: tc.serverCert,
				ServerKeyFile:  tc.serverKey})
			serverCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &serverCert, "servercert")

			trustCertChain := setupService.CreateCertChainFromTrustBundle(tc.trustBundle)
			trustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, trustCertChain, "cabundle")
			cert, err := tls.LoadX509KeyPair(tc.clientCert, tc.clientKey)
			if err != nil {
				t.Fatalf("%s Failed to load  client cert: %v", timeNow, err)
			}
			cacert := x509.NewCertPool()
			cacertBytes, err := os.ReadFile(tc.trustBundle)
			if err != nil {
				t.Fatalf("%s Failed to read ca bundle :%v", timeNow, err)
			}
			if ok := cacert.AppendCertsFromPEM(cacertBytes); !ok {
				t.Fatalf("%s Failed to parse %v", timeNow, tc.trustBundle)
			}
			certzClient := gnsiC.Certz()
			success := setupService.CertzRotate(t, cacert, certzClient, cert, san, serverAddr, testProfile, &serverCertEntity, &trustBundleEntity)
			expected_result = true
			if !success {
				t.Fatalf("%s %s:Certz rotation failed.", timeNow, tc.desc)
			}
			t.Logf("%s %s:Certz rotation completed!", timeNow, tc.desc)

			// Replace config with newly added ssl profile after successful rotate.
			servers = gnmi.GetAll(t, dut, gnmi.OC().System().GrpcServerAny().Name().State())
			batch := gnmi.SetBatch{}
			for _, server := range servers {
				gnmi.BatchReplace(&batch, gnmi.OC().System().GrpcServer(server).CertificateId().Config(), testProfile)
			}
			batch.Set(t, dut)
			t.Logf("%s %s:replaced gNMI config with new ssl profile successfully.", timeNow, tc.desc)
			time.Sleep(5 * time.Second) //waiting 5s for gnmi config propagation
			t.Run("Verification of new connection after successful server certificate rotation", func(t *testing.T) {
				result := setupService.PostValidationCheck(t, cacert, expected_result, san, serverAddr, username, password, cert)
				if !result {
					t.Fatalf("%s postTestcase service validation failed after successful rotate -got %v, want %v .", tc.desc, result, expected_result)
				}
				t.Logf("%s postTestcase service validation done after server certificate rotation!", tc.desc)
				t.Logf("PASS: %s successfully completed!", tc.desc)
			})
		})
	}
	t.Logf("Cleanup of test data.")
	if setupService.CertCleanup(t, dirPath) != nil {
		t.Fatalf("could not run cert cleanup command.")
	}
}
