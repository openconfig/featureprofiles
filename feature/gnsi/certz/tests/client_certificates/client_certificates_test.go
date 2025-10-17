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
	"testing"
	"time"

	setupService "github.com/openconfig/featureprofiles/feature/gnsi/certz/tests/internal/setup_service"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnmi/proto/gnmi"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
)

const (
	dirPath = "../../test_data/"
)

// DUTCredentialer is an interface for getting credentials from a DUT binding.
type DUTCredentialer interface {
	RPCUsername() string
	RPCPassword() string
}

var (
	testProfile    = "newprofile"
	serverAddr     string
	expectedResult bool = true
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestClientCert Test validates that client certificates from a set of one CA are able to be loaded successfully
// and  used for authentication to a device when used by a client connecting to each
// gRPC service.
func TestClientCert(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name()
	var creds DUTCredentialer
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("Failed to get DUT credentials using binding.DUTAs: %v. The binding for %s must implement the DUTCredentialer interface.", err, dut.Name())
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
	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("%s: Failed to create gNMI Connection %v", time.Now().String(), err)
	}
	t.Logf("%s Precheck:gNMI connection is successful %v", time.Now().String(), gnmiClient)
	//Generate testdata certificates
	t.Logf("%s:Creation of test data.", time.Now().String())
	/*if setupService.CertGeneration(t, dirPath) != nil {
		t.Fatalf("%s:Failed to generate the testdata certificates.", time.Now().String())
	}*/
	//Create a certz client
	certzClient := gnsiC.Certz()
	t.Logf("%s Precheck:checking baseline ssl profile list.", time.Now().String())
	setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{})
	//Add a new ssl profile
	t.Logf("%s:Adding new empty ssl profile ID.", time.Now().String())
	addProfileResponse, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: testProfile})
	if err != nil {
		t.Fatalf("%s:Add profile request failed with %v! ", time.Now().String(), err)
	}
	t.Logf("%s AddProfileResponse: %v", time.Now().String(), addProfileResponse)
	t.Logf("%s: Getting the ssl profile list after new ssl profile addition.", time.Now().String())
	setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{})
	cases := []struct {
		desc                string
		serverCertFile      string
		serverKeyFile       string
		trustBundleFile     string
		clientCertFile      string
		clientKeyFile       string
		cversion            string
		bversion            string
		prevClientCertFile  string
		prevClientKeyFile   string
		prevTrustBundleFile string
		newTLScreds         bool
		mismatch            bool
		scale               bool
	}{
		{
			desc:                "Certz1.1:Load the key-type rsa trustbundle with 1 CA configuration",
			serverCertFile:      dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:       dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile:     dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			clientCertFile:      dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:       dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:            "certz1",
			bversion:            "bundle1",
			prevClientCertFile:  "",
			prevClientKeyFile:   "",
			prevTrustBundleFile: "",
		},
		{
			desc:                "Certz1.1:Load the key-type ecdsa trustbundle with 1 CA configuration",
			serverCertFile:      dirPath + "ca-01/server-ecdsa-a-cert.pem",
			serverKeyFile:       dirPath + "ca-01/server-ecdsa-a-key.pem",
			trustBundleFile:     dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			clientCertFile:      dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:       dirPath + "ca-01/client-ecdsa-a-key.pem",
			cversion:            "certz2",
			bversion:            "bundle2",
			newTLScreds:         true,
			prevClientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			prevClientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			prevTrustBundleFile: dirPath + "ca-01/trust_bundle_01_rsa.p7b",
		},
		{
			desc:                "Certz1.1:Load the key-type rsa trustbundle with 2 CA configuration",
			serverCertFile:      dirPath + "ca-02/server-rsa-a-cert.pem",
			serverKeyFile:       dirPath + "ca-02/server-rsa-a-key.pem",
			trustBundleFile:     dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile:      dirPath + "ca-02/client-rsa-a-cert.pem",
			clientKeyFile:       dirPath + "ca-02/client-rsa-a-key.pem",
			cversion:            "certz3",
			bversion:            "bundle3",
			newTLScreds:         true,
			prevClientCertFile:  dirPath + "ca-01/client-ecdsa-a-cert.pem",
			prevClientKeyFile:   dirPath + "ca-01/client-ecdsa-a-key.pem",
			prevTrustBundleFile: dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
		},
		{
			desc:                "Certz1.1:Load the key-type ecdsa trustbundle with 2 CA configuration",
			serverCertFile:      dirPath + "ca-02/server-ecdsa-a-cert.pem",
			serverKeyFile:       dirPath + "ca-02/server-ecdsa-a-key.pem",
			trustBundleFile:     dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
			clientCertFile:      dirPath + "ca-02/client-ecdsa-a-cert.pem",
			clientKeyFile:       dirPath + "ca-02/client-ecdsa-a-key.pem",
			cversion:            "certz4",
			bversion:            "bundle4",
			newTLScreds:         true,
			prevClientCertFile:  dirPath + "ca-02/client-rsa-a-cert.pem",
			prevClientKeyFile:   dirPath + "ca-02/client-rsa-a-key.pem",
			prevTrustBundleFile: dirPath + "ca-02/trust_bundle_02_rsa.p7b",
		},
		{
			desc:                "Certz1.1:Load the key-type rsa trustbundle with 10CA configuration",
			serverCertFile:      dirPath + "ca-10/server-rsa-a-cert.pem",
			serverKeyFile:       dirPath + "ca-10/server-rsa-a-key.pem",
			trustBundleFile:     dirPath + "ca-10/trust_bundle_10_rsa.p7b",
			clientCertFile:      dirPath + "ca-10/client-rsa-a-cert.pem",
			clientKeyFile:       dirPath + "ca-10/client-rsa-a-key.pem",
			cversion:            "certz5",
			bversion:            "bundle5",
			newTLScreds:         true,
			prevClientCertFile:  dirPath + "ca-02/client-ecdsa-a-cert.pem",
			prevClientKeyFile:   dirPath + "ca-02/client-ecdsa-a-key.pem",
			prevTrustBundleFile: dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
		},
		{
			desc:                "Certz1.1:Load the key-type ecdsa trustbundle with 10CA configuration",
			serverCertFile:      dirPath + "ca-10/server-ecdsa-a-cert.pem",
			serverKeyFile:       dirPath + "ca-10/server-ecdsa-a-key.pem",
			trustBundleFile:     dirPath + "ca-10/trust_bundle_10_ecdsa.p7b",
			clientCertFile:      dirPath + "ca-10/client-ecdsa-a-cert.pem",
			clientKeyFile:       dirPath + "ca-10/client-ecdsa-a-key.pem",
			cversion:            "certz6",
			bversion:            "bundle6",
			newTLScreds:         true,
			prevClientCertFile:  dirPath + "ca-10/client-rsa-a-cert.pem",
			prevClientKeyFile:   dirPath + "ca-10/client-rsa-a-key.pem",
			prevTrustBundleFile: dirPath + "ca-10/trust_bundle_10_rsa.p7b",
		},
		{
			desc:                "Certz1.1:Load the key-type rsa trustbundle with 1000CA configuration",
			serverCertFile:      dirPath + "ca-1000/server-rsa-a-cert.pem",
			serverKeyFile:       dirPath + "ca-1000/server-rsa-a-key.pem",
			trustBundleFile:     dirPath + "ca-1000/trust_bundle_1000_rsa.p7b",
			clientCertFile:      dirPath + "ca-1000/client-rsa-a-cert.pem",
			clientKeyFile:       dirPath + "ca-1000/client-rsa-a-key.pem",
			cversion:            "certz7",
			bversion:            "bundle7",
			newTLScreds:         true,
			prevClientCertFile:  dirPath + "ca-10/client-ecdsa-a-cert.pem",
			prevClientKeyFile:   dirPath + "ca-10/client-ecdsa-a-key.pem",
			prevTrustBundleFile: dirPath + "ca-10/trust_bundle_10_ecdsa.p7b",
			scale:               true,
		},
		{
			desc:                "Certz1.1:Load the key-type ecdsa trustbundle with 1000CA configuration",
			serverCertFile:      dirPath + "ca-1000/server-ecdsa-a-cert.pem",
			serverKeyFile:       dirPath + "ca-1000/server-ecdsa-a-key.pem",
			trustBundleFile:     dirPath + "ca-1000/trust_bundle_1000_ecdsa.p7b",
			clientCertFile:      dirPath + "ca-1000/client-ecdsa-a-cert.pem",
			clientKeyFile:       dirPath + "ca-1000/client-ecdsa-a-key.pem",
			cversion:            "certz8",
			bversion:            "bundle8",
			newTLScreds:         true,
			prevClientCertFile:  dirPath + "ca-1000/client-rsa-a-cert.pem",
			prevClientKeyFile:   dirPath + "ca-1000/client-rsa-a-key.pem",
			prevTrustBundleFile: dirPath + "ca-1000/trust_bundle_1000_rsa.p7b",
			scale:               true,
		},
		{
			desc:                "Certz1.2:Load the rsa trust_bundle from ca-02 with mismatching key type rsa client certificate from ca-01",
			serverCertFile:      dirPath + "ca-02/server-rsa-a-cert.pem",
			serverKeyFile:       dirPath + "ca-02/server-rsa-a-key.pem",
			trustBundleFile:     dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile:      dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:       dirPath + "ca-01/client-rsa-a-key.pem",
			mismatch:            true,
			cversion:            "certz9",
			bversion:            "bundle9",
			newTLScreds:         true,
			prevClientCertFile:  dirPath + "ca-1000/client-ecdsa-a-cert.pem",
			prevClientKeyFile:   dirPath + "ca-1000/client-ecdsa-a-key.pem",
			prevTrustBundleFile: dirPath + "ca-1000/trust_bundle_1000_ecdsa.p7b",
		},
		{
			desc:                "Certz1.2:Load the ecdsa trust_bundle from ca-02 with mismatching key type ecdsa client certificate from ca-01",
			serverCertFile:      dirPath + "ca-02/server-ecdsa-a-cert.pem",
			serverKeyFile:       dirPath + "ca-02/server-ecdsa-a-key.pem",
			trustBundleFile:     dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
			clientCertFile:      dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:       dirPath + "ca-01/client-ecdsa-a-key.pem",
			mismatch:            true,
			cversion:            "certz10",
			bversion:            "bundle10",
			newTLScreds:         true,
			prevClientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			prevClientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			prevTrustBundleFile: dirPath + "ca-02/trust_bundle_02_rsa.p7b",
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Logf("%s:STATUS:Starting test case: %s", tc.desc, time.Now().String())
			san := setupService.ReadDecodeServerCertificate(t, tc.serverCertFile)
			serverCert := setupService.CreateCertzChain(t, setupService.CertificateChainRequest{
				RequestType:    setupService.EntityTypeCertificateChain,
				ServerCertFile: tc.serverCertFile,
				ServerKeyFile:  tc.serverKeyFile})
			serverCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &serverCert, tc.cversion)
			t.Logf("STATUS:Loading PKCS#7 trust bundle from file: %s.", tc.trustBundleFile)
			pkcs7certs, pkcs7data, err := setupService.Loadpkcs7TrustBundle(tc.trustBundleFile)
			if err != nil {
				t.Fatalf("failed to load trust bundle: %v", err)
			}
			//Create a new Cert Pool and add the certs from the trust bundle.
			newCaCert := x509.NewCertPool()
			for _, c := range pkcs7certs {
				newCaCert.AddCert(c)
			}
			trustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, string(pkcs7data), tc.bversion)
			//Load Client certificate
			newClientCert, err := tls.LoadX509KeyPair(tc.clientCertFile, tc.clientKeyFile)
			if err != nil {
				t.Fatalf("Failed to load client cert: %v", err)
			}
			if tc.newTLScreds {
				t.Logf("%s:STATUS:%s: Creating new TLS credentials for client connection.", tc.desc, time.Now().String())
				//Load the prior client keypair for new client TLS credentials
				prevClientCert, err := tls.LoadX509KeyPair(tc.prevClientCertFile, tc.prevClientKeyFile)
				if err != nil {
					t.Fatalf("%s:STATUS:Failed to load previous client cert: %v", tc.desc, err)
				}
				oldPkcs7certs, oldPkcs7data, err := setupService.Loadpkcs7TrustBundle(tc.prevTrustBundleFile)
				if err != nil {
					t.Fatalf("%s:STATUS:Failed to load previous trust bundle,data %v with %v", tc.desc, oldPkcs7data, err)
				}
				//Create a old set of Cert Pool and append the certs from previous trust bundle.
				prevCaCert := x509.NewCertPool()
				for _, c := range oldPkcs7certs {
					prevCaCert.AddCert(c)
				}
				//Retrieve the connection with previous TLS credentials for certz rotation.
				conn := setupService.CreateNewDialOption(t, prevClientCert, prevCaCert, san, username, password, serverAddr)
				defer conn.Close()
				certzClient = certzpb.NewCertzClient(conn)
				gnmiClient = gnmi.NewGNMIClient(conn)

			} else {
				t.Logf("%s:STATUS:%s:Using existing TLS credentials for client connection in first iteration.", tc.desc, time.Now().String())
			}
			//Certz Rotation in progress.
			t.Logf("STATUS:%s Initiating Certz rotation with server cert: %s and trust bundle: %s", tc.desc, tc.serverCertFile, tc.trustBundleFile)
			success := setupService.CertzRotate(ctx, t, newCaCert, certzClient, gnmiClient, newClientCert, dut, username, password, san, serverAddr, testProfile, tc.newTLScreds, tc.mismatch, tc.scale, &serverCertEntity, &trustBundleEntity)
			if !success {
				t.Fatalf("%s:STATUS: Certz rotation failed.", tc.desc)
			}
			t.Logf("%s:STATUS:%s :Certz rotation completed!", tc.desc, time.Now().String())

			//Post rotate validation of all services.
			t.Run("Verification of new connection after rotate ", func(t *testing.T) {
				t.Logf("%s:STATUS: %s: Validation of all services after completion of certz rotate with %s and %s.", tc.desc, time.Now().String(), tc.clientCertFile, tc.trustBundleFile)
				result := setupService.PostValidationCheck(t, newCaCert, expectedResult, san, serverAddr, username, password, newClientCert, tc.mismatch)
				if !result {
					t.Fatalf("%s :postTestcase service validation failed after rotate- got %v, want %v", tc.desc, result, expectedResult)
				}
				t.Logf("%s:STATUS: postTestcase service validation done!", tc.desc)
			})

		})
		t.Logf("%s:STATUS: Test case completed: %s", time.Now().String(), tc.desc)
	}
	/*t.Logf("Cleanup of test data.")
	if setupService.CertCleanup(t, dirPath) != nil {
		t.Fatalf("could not run testdata cleanup command.")
	}*/
	t.Logf("STATUS: Testdata cleanup completed!")
}
