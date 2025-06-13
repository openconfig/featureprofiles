// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package client_certificates_test

import (
	"context"
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
	testProfile    = "newprofile"
	serverAddr     string
	username            = "certzuser"
	password            = "certzpasswd"
	expectedResult bool = false
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
		t.Fatalf("Failed to create credentials: got %v ,want notnil ", res)
	}

	return true
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestClientCert Test validates that client certificates from a set of one CA are able to be loaded successfully
// and  used for authentication to a device when used by a client connecting to each
// gRPC service.
func TestClientCert(t *testing.T) {
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
		clientCertFile  string
		clientKeyFile   string
		serverCertFile  string
		serverKeyFile   string
		trustBundleFile string
		p7btrustBundle  string
		mismatch        bool
	}{
		{
			desc:            "Certz1.1:Load the key-type rsa trustbundle with 1 CA configuration",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_rsa.pem",
			p7btrustBundle:  dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
		},
		{
			desc:            "Certz1.1:Load the key-type ecdsa trustbundle with 1 CA configuration",
			serverCertFile:  dirPath + "ca-01/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_ecdsa.pem",
			p7btrustBundle:  dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-ecdsa-a-key.pem",
		},
		{
			desc:            "Certz1.1:Load the key-type rsa trustbundle with 2 CA configuration",
			serverCertFile:  dirPath + "ca-02/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_rsa.pem",
			p7btrustBundle:  dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile:  dirPath + "ca-02/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-02/client-rsa-a-key.pem",
		},
		{
			desc:            "Certz1.1:Load the key-type ecdsa trustbundle with 2 CA configuration",
			serverCertFile:  dirPath + "ca-02/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_ecdsa.pem",
			p7btrustBundle:  dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-02/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-02/client-ecdsa-a-key.pem",
		},
		{
			desc:            "Certz1.1:Load the key-type rsa trustbundle with 10CA configuration",
			serverCertFile:  dirPath + "ca-10/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-10/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-10/trust_bundle_10_rsa.pem",
			p7btrustBundle:  dirPath + "ca-10/trust_bundle_10_rsa.p7b",
			clientCertFile:  dirPath + "ca-10/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-10/client-rsa-a-key.pem",
		},
		{
			desc:            "Certz1.1:Load the key-type ecdsa trustbundle with 10CA configuration",
			serverCertFile:  dirPath + "ca-10/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-10/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-10/trust_bundle_10_ecdsa.pem",
			p7btrustBundle:  dirPath + "ca-10/trust_bundle_10_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-10/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-10/client-ecdsa-a-key.pem",
		},
		{
			desc:            "Certz1.1:Load the key-type rsa trustbundle with 1000CA configuration",
			serverCertFile:  dirPath + "ca-1000/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-1000/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-1000/trust_bundle_1000_rsa.pem",
			p7btrustBundle:  dirPath + "ca-1000/trust_bundle_1000_rsa.p7b",
			clientCertFile:  dirPath + "ca-1000/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-1000/client-rsa-a-key.pem",
		},
		{
			desc:            "Certz1.1:Load the key-type ecdsa trustbundle with 1000CA configuration",
			serverCertFile:  dirPath + "ca-1000/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-1000/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-1000/trust_bundle_1000_ecdsa.pem",
			p7btrustBundle:  dirPath + "ca-1000/trust_bundle_1000_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-1000/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-1000/client-ecdsa-a-key.pem",
		},
		{
			desc:            "Certz1.2:Load the rsa trust_bundle from ca-02 with mismatching key type rsa client certificate from ca-01",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_rsa.pem",
			p7btrustBundle:  dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			mismatch:        true,
		},
		{
			desc:            "Certz1.2:Load the ecdsa trust_bundle from ca-02 with mismatching key type ecdsa client certificate from ca-01",
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
			serverCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &serverCert, "cert1")
			//Enable pkcs7 to true for new certz proto and
			//trustCertChain := setupService.CreateCertChainFromp7bTrustBundle(tc.p7btrustBundle)
			trustCertChain := setupService.CreateCertChainFromTrustBundle(tc.trustBundleFile)
			trustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, trustCertChain, "bundle1")
			cert, err := tls.LoadX509KeyPair(tc.clientCertFile, tc.clientKeyFile)
			if err != nil {
				t.Fatalf("Failed to load  client cert: %v", err)
			}
			cacert := x509.NewCertPool()
			cacertBytes, err := os.ReadFile(tc.trustBundleFile)
			if err != nil {
				t.Fatalf("Failed to read ca bundle :%v", err)
			}
			if ok := cacert.AppendCertsFromPEM(cacertBytes); !ok {
				t.Fatalf("Failed to parse %v", tc.trustBundleFile)
			}

			switch tc.mismatch {
			case true:
				success := setupService.CertzRotate(ctx, t, cacert, certzClient, cert, dut, san, serverAddr, testProfile, &serverCertEntity, &trustBundleEntity)
				if success {
					t.Fatalf("%s:Certz rotation failed.", tc.desc)
				}
				t.Logf("%s:Mismatch certz rotation failed as expected before finalize!", tc.desc)
				t.Run("Verification of new connection with mismatch rotate of trustbundle.", func(t *testing.T) {
					result := setupService.PostValidationCheck(t, cacert, expectedResult, san, serverAddr, username, password, cert)
					if !result {
						t.Fatalf("%s :postTestcase service validation failed after rotate- got %v, want %v", tc.desc, result, false)
					}
					t.Logf("%s postTestcase service validation done!", tc.desc)
				})
			case false:
				expectedResult = true
				success := setupService.CertzRotate(ctx, t, cacert, certzClient, cert, dut, san, serverAddr, testProfile, &serverCertEntity, &trustBundleEntity)
				if !success {
					t.Fatalf("%s:Certz rotation failed.", tc.desc)
				}
				t.Logf("%s:successfully completed certz rotation!", tc.desc)
				// Verification check of the new connection post rotation.
				t.Run("Verification of new connection after rotate ", func(t *testing.T) {
					result := setupService.PostValidationCheck(t, cacert, expectedResult, san, serverAddr, username, password, cert)
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
