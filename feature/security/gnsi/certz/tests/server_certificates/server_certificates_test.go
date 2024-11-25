// Copyright 2024 Google LLC
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

package server_certificates_test

import (
	context "context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"testing"
	"time"

	setupService "github.com/openconfig/featureprofiles/feature/security/gnsi/certz/tests/internal/setup_service"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"

	certzpb "github.com/openconfig/gnsi/certz"
)

const (
	dirPath = "../../test_data/"
)

var (
	testProfile     = "newprofile"
	serverAddr      string
	username        = "certzuser"
	password        = "certzpasswd"
	expected_result bool
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

// TestServerCertTcTwo tests the server certificates from a set of one CA are able to be validated and
// used for authentication to a device when used by a client connecting to each
// gRPC service.
func TestServerCert(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name()
	if !createUser(t, dut, username, password) {
		t.Fatalf("%s: Failed to create certz user.", time.Now().String())
	}
	t.Logf("Validation of all services that are using gRPC before certz rotation.")
	if !setupService.PreInitCheck(context.Background(), t, dut) {
		t.Fatalf("%s: Failed in the preInit checks.", time.Now().String())
	}
	ctx := context.Background()
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(ctx)
	if err != nil {
		t.Fatalf("%s: Failed to create gNSI Connection %v", time.Now().String(), err)
	}
	t.Logf("%s Precheck:gNSI connection is successful %v", time.Now().String(), gnsiC)
	t.Logf("%s:Creation of test data.", time.Now().String())
	if setupService.CertGeneration(t, dirPath) != nil {
		t.Fatalf("%s:Failed to generate the testdata certificates.", time.Now().String())
	}
	certzClient := gnsiC.Certz()
	t.Logf("%s Precheck:checking baseline ssl profile list.", time.Now().String())
	setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{})
	t.Logf("%s:Adding new empty ssl profile ID.", time.Now().String())
	addProfileResponse, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: testProfile})
	if err != nil {
		t.Fatalf("%s:Add profile request failed with %v! ", time.Now().String(), err)
	}
	t.Logf("%s AddProfileResponse: %v", time.Now().String(), addProfileResponse)
	t.Logf("%s: Getting the ssl profile list after new ssl profile addition.", time.Now().String())
	setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{})
	cases := []struct {
		desc            string
		serverCertFile  string
		serverKeyFile   string
		trustBundleFile string
		clientCertFile  string
		clientKeyFile   string
		p7btrustBundle  string
		mismatch        bool
	}{
		{
			desc:            "Certz2.1:Load server certificate of rsa keytype with 1 CA configuration",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_rsa.pem",
			p7btrustBundle:  dirPath + "ca-01/ca-01/trust_bundle_01_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of ecdsa keytype with 1 CA configuration",
			serverCertFile:  dirPath + "ca-01/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_ecdsa.pem",
			p7btrustBundle:  dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-ecdsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of rsa keytype with 2 CA configuration",
			serverCertFile:  dirPath + "ca-02/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_rsa.pem",
			p7btrustBundle:  dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile:  dirPath + "ca-02/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-02/client-rsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of ecdsa keytype with 2 CA configuration",
			serverCertFile:  dirPath + "ca-02/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_ecdsa.pem",
			p7btrustBundle:  dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-02/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-02/client-ecdsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of rsa keytype with 10CA configuration",
			serverCertFile:  dirPath + "ca-10/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-10/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-10/trust_bundle_10_rsa.pem",
			p7btrustBundle:  dirPath + "ca-10/trust_bundle_10_rsa.p7b",
			clientCertFile:  dirPath + "ca-10/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-10/client-rsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of ecdsa keytype with 10CA configuration",
			serverCertFile:  dirPath + "ca-10/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-10/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-10/trust_bundle_10_ecdsa.pem",
			p7btrustBundle:  dirPath + "ca-10/trust_bundle_10_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-10/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-10/client-ecdsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of rsa keytype with 1000CA configuration",
			serverCertFile:  dirPath + "ca-1000/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-1000/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-1000/trust_bundle_1000_rsa.pem",
			p7btrustBundle:  dirPath + "ca-1000/trust_bundle_1000_rsa.p7b",
			clientCertFile:  dirPath + "ca-1000/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-1000/client-rsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of ecdsa keytype with 1000CA configuration",
			serverCertFile:  dirPath + "ca-1000/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-1000/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-1000/trust_bundle_1000_ecdsa.pem",
			p7btrustBundle:  dirPath + "ca-1000/trust_bundle_1000_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-1000/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-1000/client-ecdsa-a-key.pem",
		},
		{
			desc:            "Certz2.2:Load the rsa trust_bundle from ca-02 with mismatching key type rsa server certificate from ca-01",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_rsa.pem",
			p7btrustBundle:  dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			mismatch:        true,
		},
		{
			desc:            "Certz2.2:Load the ecdsa trust_bundle from ca-02 with mismatching key type ecdsa server certificate from ca-01",
			serverCertFile:  dirPath + "ca-01/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_ecdsa.pem",
			p7btrustBundle:  dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-ecdsa-a-key.pem",
			mismatch:        true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {

			san := setupService.ReadDecodeServerCertificate(t, tc.serverCertFile)
			serverCert := setupService.CreateCertzChain(t, setupService.CertificateChainRequest{
				RequestType:    setupService.EntityTypeCertificateChain,
				ServerCertFile: tc.serverCertFile,
				ServerKeyFile:  tc.serverKeyFile})
			serverCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &serverCert, "servercert")

			trustCertChain := setupService.CreateCertChainFromTrustBundle(tc.trustBundleFile)
			trustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, trustCertChain, "cabundle")
			cert, err := tls.LoadX509KeyPair(tc.clientCertFile, tc.clientKeyFile)
			if err != nil {
				t.Fatalf("%s Failed to load  client cert: %v", time.Now().String(), err)
			}
			cacert := x509.NewCertPool()
			cacertBytes, err := os.ReadFile(tc.trustBundleFile)
			if err != nil {
				t.Fatalf("%s Failed to read ca bundle :%v", time.Now().String(), err)
			}
			if ok := cacert.AppendCertsFromPEM(cacertBytes); !ok {
				t.Fatalf("%s Failed to parse %v", time.Now().String(), tc.trustBundleFile)
			}
			switch tc.mismatch {
			case true:
				expected_result = false
				success := setupService.CertzRotate(t, cacert, certzClient, cert, ctx, dut, san, serverAddr, testProfile, &serverCertEntity, &trustBundleEntity)
				if success {
					t.Fatalf("%s:Server Certificate rotation failed.", tc.desc)
				}
				t.Logf("%s:Mismatch server certificate rotation failed as expected before finalize!", tc.desc)
				t.Run("Verification of new connections with mismatch rotate of trustbundle.", func(t *testing.T) {
					result := setupService.PostValidationCheck(t, cacert, expected_result, san, serverAddr, username, password, cert)
					if !result {
						t.Fatalf("%s :postTestcase service validation failed after rotate- got %v, want %v", tc.desc, result, false)
					}
					t.Logf("%s postTestcase service validation done!", tc.desc)
				})
			case false:
				expected_result = true
				success := setupService.CertzRotate(t, cacert, certzClient, cert, ctx, dut, san, serverAddr, testProfile, &serverCertEntity, &trustBundleEntity)
				if !success {
					t.Fatalf("%s:Server Certificate rotation failed.", tc.desc)
				}
				t.Logf("%s:successfully completed server certificate rotation!", tc.desc)
				// Verification check of the new connection post rotation.
				t.Run("Verification of new connections after rotate ", func(t *testing.T) {
					result := setupService.PostValidationCheck(t, cacert, expected_result, san, serverAddr, username, password, cert)
					if !result {
						t.Fatalf("%s :postTestcase service validation failed after rotate- got %v, want %v", tc.desc, result, true)
					}
					t.Logf("%s postTestcase service validation done!", tc.desc)
				})
			}
		})
		t.Logf("PASS: %s successfully completed!", tc.desc)
	}
	t.Logf("Cleanup of test data.")
	if setupService.CertCleanup(t, dirPath) != nil {
		t.Fatalf("could not run testdata cleanup command.")
	}
}
