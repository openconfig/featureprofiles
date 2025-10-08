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
	"context"
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"

	setupService "github.com/openconfig/featureprofiles/feature/security/gnsi/certz/tests/internal/setup_service"
	"github.com/openconfig/featureprofiles/internal/fptest"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
)

const (
	dirPath = "../../test_data/"
)

// DUTCredentialer is an interface for getting credentials from a DUT bindin
type DUTCredentialer interface {
	RPCUsername() string
	RPCPassword() string
}

var (
	testProfile     = "newprofile"
	serverAddr      string
	expected_result bool
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestServerCertTcTwo tests the server certificates from a set of one CA are able to be validated and
// used for authentication to a device when used by a client connecting to each
// gRPC service.
func TestServerCert(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name()
	var creds DUTCredentialer
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("Failed to get DUT credentials using binding.DUTAs: %v. The binding for %s must implement the DUTCredentialer interface", err, dut.Name())
	}
	username := creds.RPCUsername()
	password := creds.RPCPassword()
	t.Logf("Username: %s, Password: %s", username, password)
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
	time.Sleep(2 * time.Second)
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
		mismatch        bool
		cversion        string
		bversion        string
	}{
		{
			desc:           "Certz2.1:Load server certificate of rsa keytype with 1 CA configuration",
			serverCertFile: dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:  dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundle:    dirPath + "ca-01/ca-01/trust_bundle_01_rsa.p7b",
			clientCertFile: dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:  dirPath + "ca-01/client-rsa-a-key.pem",
		},
		{
			desc:           "Certz2.1:Load server certificate of ecdsa keytype with 1 CA configuration",
			serverCertFile: dirPath + "ca-01/server-ecdsa-a-cert.pem",
			serverKeyFile:  dirPath + "ca-01/server-ecdsa-a-key.pem",
			trustBundle:    dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			clientCertFile: dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:  dirPath + "ca-01/client-ecdsa-a-key.pem",
		},
		{
			desc:           "Certz2.1:Load server certificate of rsa keytype with 2 CA configuration",
			serverCertFile: dirPath + "ca-02/server-rsa-a-cert.pem",
			serverKeyFile:  dirPath + "ca-02/server-rsa-a-key.pem",
			trustBundle:    dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile: dirPath + "ca-02/client-rsa-a-cert.pem",
			clientKeyFile:  dirPath + "ca-02/client-rsa-a-key.pem",
		},
		{
			desc:           "Certz2.1:Load server certificate of ecdsa keytype with 2 CA configuration",
			serverCertFile: dirPath + "ca-02/server-ecdsa-a-cert.pem",
			serverKeyFile:  dirPath + "ca-02/server-ecdsa-a-key.pem",
			trustBundle:    dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
			clientCertFile: dirPath + "ca-02/client-ecdsa-a-cert.pem",
			clientKeyFile:  dirPath + "ca-02/client-ecdsa-a-key.pem",
		},
		{
			desc:           "Certz2.1:Load server certificate of rsa keytype with 10CA configuration",
			serverCertFile: dirPath + "ca-10/server-rsa-a-cert.pem",
			serverKeyFile:  dirPath + "ca-10/server-rsa-a-key.pem",
			trustBundle:    dirPath + "ca-10/trust_bundle_10_rsa.p7b",
			clientCertFile: dirPath + "ca-10/client-rsa-a-cert.pem",
			clientKeyFile:  dirPath + "ca-10/client-rsa-a-key.pem",
		},
		{
			desc:           "Certz2.1:Load server certificate of ecdsa keytype with 10CA configuration",
			serverCertFile: dirPath + "ca-10/server-ecdsa-a-cert.pem",
			serverKeyFile:  dirPath + "ca-10/server-ecdsa-a-key.pem",
			trustBundle:    dirPath + "ca-10/trust_bundle_10_ecdsa.p7b",
			clientCertFile: dirPath + "ca-10/client-ecdsa-a-cert.pem",
			clientKeyFile:  dirPath + "ca-10/client-ecdsa-a-key.pem",
		},
		{
			desc:           "Certz2.1:Load server certificate of rsa keytype with 1000CA configuration",
			serverCertFile: dirPath + "ca-1000/server-rsa-a-cert.pem",
			serverKeyFile:  dirPath + "ca-1000/server-rsa-a-key.pem",
			trustBundle:    dirPath + "ca-1000/trust_bundle_1000_rsa.p7b",
			clientCertFile: dirPath + "ca-1000/client-rsa-a-cert.pem",
			clientKeyFile:  dirPath + "ca-1000/client-rsa-a-key.pem",
		},
		{
			desc:           "Certz2.1:Load server certificate of ecdsa keytype with 1000CA configuration",
			serverCertFile: dirPath + "ca-1000/server-ecdsa-a-cert.pem",
			serverKeyFile:  dirPath + "ca-1000/server-ecdsa-a-key.pem",
			trustBundle:    dirPath + "ca-1000/trust_bundle_1000_ecdsa.p7b",
			clientCertFile: dirPath + "ca-1000/client-ecdsa-a-cert.pem",
			clientKeyFile:  dirPath + "ca-1000/client-ecdsa-a-key.pem",
		},
		{
			desc:           "Certz2.2:Load the rsa trust_bundle from ca-02 with mismatching key type rsa server certificate from ca-01",
			serverCertFile: dirPath + "ca-02/server-rsa-a-cert.pem",
			serverKeyFile:  dirPath + "ca-02/server-rsa-a-key.pem",
			trustBundle:    dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile: dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:  dirPath + "ca-01/client-rsa-a-key.pem",
			mismatch:       true,
		},
		{
			desc:           "Certz2.2:Load the ecdsa trust_bundle from ca-02 with mismatching key type ecdsa server certificate from ca-01",
			serverCertFile: dirPath + "ca-02/server-ecdsa-a-cert.pem",
			serverKeyFile:  dirPath + "ca-02/server-ecdsa-a-key.pem",
			trustBundle:    dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
			clientCertFile: dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:  dirPath + "ca-01/client-ecdsa-a-key.pem",
			mismatch:       true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			san := setupService.ReadDecodeServerCertificate(t, tc.serverCertFile)
			serverCert := setupService.CreateCertzChain(t, setupService.CertificateChainRequest{
				RequestType:    setupService.EntityTypeCertificateChain,
				ServerCertFile: tc.serverCertFile,
				ServerKeyFile:  tc.serverKeyFile})
			serverCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &serverCert, tc.cversion)
			pkcs7certs, pkcs7data, err := setupService.Loadpkcs7TrustBundle(tc.trustBundleFile)
			if err != nil {
				t.Fatalf("failed to load trust bundle: %v", err)
			}
			newcacert := x509.NewCertPool()
			for _, c := range pkcs7certs {
				newcacert.AddCert(c)
			}
			trustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, string(pkcs7data), tc.bversion)
			//Load Client certificate
			newclientcert, err := tls.LoadX509KeyPair(tc.clientCertFile, tc.clientKeyFile)
			if err != nil {
				t.Fatalf("Failed to load  client cert: %v", err)
			}
			//Certz Rotation in progress
			success := setupService.CertzRotate(ctx, t, newcacert, certzClient, newclientcert, dut, username, password, san, serverAddr, testProfile, tc.mismatch, &serverCertEntity, &trustBundleEntity)
			if !success {
				t.Fatalf("%s:STATUS: Certz rotation failed.", tc.desc)
			}
			t.Logf("%s:STATUS: Certz rotation completed!", tc.desc)
			//Post rotate validation of all services.
			t.Run("Verification of new connection after rotate ", func(t *testing.T) {
				result := setupService.PostValidationCheck(t, newcacert, expectedResult, san, serverAddr, username, password, newclientcert, tc.mismatch)
				if !result {
					t.Fatalf("%s :postTestcase service validation failed after rotate- got %v, want %v", tc.desc, result, expectedResult)
				}
				t.Logf("%s:STATUS: postTestcase service validation done!", tc.desc)
			})
		})
	}
	t.Logf("Cleanup of test data.")
	if setupService.CertCleanup(t, dirPath) != nil {
		t.Fatalf("could not run testdata cleanup command.")
	}
	t.Logf("STATUS: Testdata cleanup completed!")
}
