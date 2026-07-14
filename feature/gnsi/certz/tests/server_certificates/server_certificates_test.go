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
	"flag"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/feature/gnsi/certz/tests/internal/setup_service"
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

func buildCertPool(certs []*x509.Certificate) *x509.CertPool {
	pool := x509.NewCertPool()
	for _, c := range certs {
		pool.AddCert(c)
	}
	return pool
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// verifyServices explicitly validates connections for all required gRPC services: gNMI, gNOI, gNSI, gRIBI, and P4RT.
// For Certz-2.1 positive tests, it verifies that connections are established and certificates are trusted by both client and server.
// For Certz-2.2 negative tests (when mismatch is true), it explicitly verifies Step 4 (that the client certificate is untrusted and cannot be used)
// and Step 5 (that the connection is properly torn down by the DUT).
func verifyServices(t *testing.T, caCert *x509.CertPool, expectedResult bool, san, serverAddr, username, password string, cert tls.Certificate, mismatch bool) bool {
	t.Helper()
	if mismatch {
		t.Logf("%s: Verifying Certz-2.2 Step 4 & 5: untrusted client certificate cannot be used and connection is properly torn down by DUT across gNMI, gNOI, gNSI, gRIBI, and P4RT.", time.Now().String())
	} else {
		t.Logf("%s: Verifying Certz-2.1: trusted connection established across gNMI, gNOI, gNSI, gRIBI, and P4RT.", time.Now().String())
	}
	if result := setupService.VerifyGnmi(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Errorf("gNMI service validation failed: got %v, want %v", result, expectedResult)
		return false
	}
	if result := setupService.VerifyGnoi(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Errorf("gNOI service validation failed: got %v, want %v", result, expectedResult)
		return false
	}
	if result := setupService.VerifyGnsi(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Errorf("gNSI service validation failed: got %v, want %v", result, expectedResult)
		return false
	}
	if result := setupService.VerifyGribi(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Errorf("gRIBI service validation failed: got %v, want %v", result, expectedResult)
		return false
	}
	if result := setupService.VerifyP4rt(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Errorf("P4RT service validation failed: got %v, want %v", result, expectedResult)
		return false
	}
	if mismatch {
		t.Logf("Certz-2.2 Step 5 verified: connections properly torn down by DUT.")
	}
	return true
}

// TestServerCert tests the server certificates from a set of one CA are able to be validated and
// used for authentication to a device when used by a client connecting to each
// gRPC service.
func TestServerCert(t *testing.T) {

	var (
		serverAddr          string
		creds               DUTCredentialer                //an interface for getting credentials from a DUT binding
		testProfile         string          = "newprofile" //sslProfileId name
		prevClientCertFile  string          = ""
		prevClientKeyFile   string          = ""
		prevTrustBundleFile string          = ""
		expectedResult      bool            = true
		certsList                           = flag.String("certsList", "01,02,10,1000", "Number of Certificate Sets to generate for this test. Comma separated string")
		certsTimeout                        = flag.Duration("certsTimeout", 10*time.Minute, "Time duration for cert generation and cleanup. Increase if more certs are to be generated")
	)

	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name() //returns the device name.
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("STATUS:Failed to get DUT credentials using binding.DUTAs: %v. The binding for %s must implement the DUTCredentialer interface", err, dut.Name())
	}
	username := creds.RPCUsername()
	password := creds.RPCPassword()
	t.Logf("Validation of all services that are using gRPC before server certificate rotation.")
	gnmiClient, gnsiC := setup_service.PreInitCheck(context.Background(), t, dut)
	//Generate testdata certificates.
	t.Logf("Creation of test data.")
	// Registering the cleanup before the certificate generation call, so it runs even if certificate generation fails.
	t.Cleanup(func() {
		t.Logf("STATUS:Cleanup of test data.")
		if err := setup_service.TestdataMakeCleanup(t, dirPath, *certsTimeout, "./cleanup.sh"); err != nil {
			t.Logf("STATUS:Cleanup of testdata certificates failed!: %v", err)
		}
	})
	t.Logf("STATUS:Generation of testdata certificates begins.")
	command := fmt.Sprintf("./mk_cas.sh %v", *certsList)
	if err := setup_service.TestdataMakeCleanup(t, dirPath, *certsTimeout, command); err != nil {
		t.Fatalf("STATUS:Generation of testdata certificates failed!: %v", err)
	}
	//Create a certz client.
	ctx := context.Background()
	certzClient := gnsiC.Certz()
	t.Logf("STATUS:Precheck:checking baseline sslprofile list.")
	//Get sslprofile list.
	if getResp := setup_service.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Fatalf("STATUS:profileID %s already exists.", testProfile)
	}
	//Add a new sslprofileID.
	t.Logf("STATUS:Adding new sslprofileID %s.", testProfile)
	if addProfileResponse, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: testProfile}); err != nil {
		t.Fatalf("STATUS:Add profile request failed with %v! ", err)
	} else {
		t.Logf("STATUS:Received the AddProfileResponse %v.", addProfileResponse)
	}
	//Get sslprofile list after new sslprofile addition.
	if getResp := setup_service.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); !slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Fatalf("STATUS:newly added profileID is not seen.")
	} else {
		t.Logf("STATUS:new profileID %s is seen in sslprofile list", testProfile)
	}
	cases := []struct {
		desc            string
		serverCertFile  string
		serverKeyFile   string
		trustBundleFile string
		clientCertFile  string
		clientKeyFile   string
		cversion        string
		bversion        string
		newTLScreds     bool
		mismatch        bool
		scale           bool
	}{
		{
			desc:            "Certz2.1:Load server certificate of rsa keytype with 1 CA configuration",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:        "certz1",
			bversion:        "bundle1",
		},
		{
			desc:            "Certz2.1:Load server certificate of ecdsa keytype with 1 CA configuration",
			serverCertFile:  dirPath + "ca-01/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-ecdsa-a-key.pem",
			cversion:        "certz2",
			bversion:        "bundle2",
			newTLScreds:     true,
		},
		{
			desc:            "Certz2.1:Load server certificate of rsa keytype with 2 CA configuration",
			serverCertFile:  dirPath + "ca-02/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile:  dirPath + "ca-02/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-02/client-rsa-a-key.pem",
			cversion:        "certz3",
			bversion:        "bundle3",
			newTLScreds:     true,
		},
		{
			desc:            "Certz2.1:Load server certificate of ecdsa keytype with 2 CA configuration",
			serverCertFile:  dirPath + "ca-02/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-02/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-02/client-ecdsa-a-key.pem",
			cversion:        "certz4",
			bversion:        "bundle4",
			newTLScreds:     true,
		},
		{
			desc:            "Certz2.1:Load server certificate of rsa keytype with 10CA configuration",
			serverCertFile:  dirPath + "ca-10/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-10/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-10/trust_bundle_10_rsa.p7b",
			clientCertFile:  dirPath + "ca-10/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-10/client-rsa-a-key.pem",
			cversion:        "certz5",
			bversion:        "bundle5",
			newTLScreds:     true,
		},
		{
			desc:            "Certz2.1:Load server certificate of ecdsa keytype with 10CA configuration",
			serverCertFile:  dirPath + "ca-10/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-10/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-10/trust_bundle_10_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-10/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-10/client-ecdsa-a-key.pem",
			cversion:        "certz6",
			bversion:        "bundle6",
			newTLScreds:     true,
		},
		{
			desc:            "Certz2.1:Load server certificate of rsa keytype with 1000CA configuration",
			serverCertFile:  dirPath + "ca-1000/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-1000/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-1000/trust_bundle_1000_rsa.p7b",
			clientCertFile:  dirPath + "ca-1000/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-1000/client-rsa-a-key.pem",
			cversion:        "certz7",
			bversion:        "bundle7",
			newTLScreds:     true,
			scale:           true,
		},
		{
			desc:            "Certz2.1:Load server certificate of ecdsa keytype with 1000CA configuration",
			serverCertFile:  dirPath + "ca-1000/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-1000/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-1000/trust_bundle_1000_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-1000/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-1000/client-ecdsa-a-key.pem",
			cversion:        "certz8",
			bversion:        "bundle8",
			newTLScreds:     true,
			scale:           true,
		},
		{
			desc:            "Certz2.2:Load the rsa trust_bundle from ca-02 with mismatching key type rsa server certificate from ca-01",
			serverCertFile:  dirPath + "ca-02/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			mismatch:        true,
			cversion:        "certz9",
			bversion:        "bundle9",
			newTLScreds:     true,
		},
		{
			desc:            "Certz2.2:Load the ecdsa trust_bundle from ca-02 with mismatching key type ecdsa server certificate from ca-01",
			serverCertFile:  dirPath + "ca-02/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-ecdsa-a-key.pem",
			mismatch:        true,
			cversion:        "certz10",
			bversion:        "bundle10",
			newTLScreds:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Logf("STATUS:Starting test case: %s", tc.desc)
			//Read the serverSAN (Subject Alternative Name) from the certificate used for TLS verification.
			serverSAN := setup_service.ReadDecodeServerCertificate(t, tc.serverCertFile)
			//Build serverCertEntity for the server certificate rotation.
			serverCert := setup_service.CreateCertzChain(t, setup_service.CertificateChainRequest{
				RequestType:    setup_service.EntityTypeCertificateChain,
				ServerCertFile: tc.serverCertFile,
				ServerKeyFile:  tc.serverKeyFile})
			serverCertEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeCertificateChain, &serverCert, tc.cversion)
			//Create a new Cert Pool and add the certs from the trust bundle.
			pkcs7certs, pkcs7data, err := setup_service.Loadpkcs7TrustBundle(tc.trustBundleFile)
			if err != nil {
				t.Fatalf("STATUS:Failed to load trust bundle: %v", err)
			}
			newCaCert := buildCertPool(pkcs7certs)
			//Build trustBundleEntity for the server certificate rotation.
			trustBundleEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeTrustBundle, string(pkcs7data), tc.bversion)
			//Load Client certificate.
			newClientCert, err := tls.LoadX509KeyPair(tc.clientCertFile, tc.clientKeyFile)
			if err != nil {
				t.Fatalf("STATUS:Failed to load client cert: %v", err)
			}
			activeCertzClient := certzClient
			activeGNMIClient := gnmiClient
			if tc.newTLScreds {
				t.Logf("STATUS:%sCreating new TLS credentials for client connection.", tc.desc)
				//Load the prior client keypair for new client TLS credentials.
				prevClientCert, err := tls.LoadX509KeyPair(prevClientCertFile, prevClientKeyFile)
				if err != nil {
					t.Fatalf("STATUS:%s:Failed to load previous client cert: %v.", tc.desc, err)
				}
				oldPkcs7certs, oldPkcs7data, err := setup_service.Loadpkcs7TrustBundle(prevTrustBundleFile)
				if err != nil {
					t.Fatalf("STATUS:%s:Failed to load previous trust bundle,data %v with %v.", tc.desc, oldPkcs7data, err)
				}
				//Create a old set of Cert Pool and append the certs from previous trust bundle.
				prevCaCert := buildCertPool(oldPkcs7certs)
				//Before rotation, validation of all services with existing certificates.
				if result := setup_service.ServicesValidationCheck(t, prevCaCert, expectedResult, serverSAN, serverAddr, username, password, prevClientCert, tc.mismatch); !result {
					t.Fatalf("STATUS:%s:service validation failed before rotate- got %v, want %v.", tc.desc, result, expectedResult)
				}
				//Retrieve the connection with previous TLS credentials for certz rotation.
				conn := setup_service.CreateNewDialOption(t, prevClientCert, prevCaCert, serverSAN, username, password, serverAddr)
				defer conn.Close()
				//certz and gnmi clients for the rotation request.
				activeCertzClient = certzpb.NewCertzClient(conn)
				activeGNMIClient = gnmi.NewGNMIClient(conn)
			} else {
				t.Logf("STATUS:%s:Using existing TLS credentials for client connection in first iteration.", tc.desc)
			}
			//Initiate server certificate rotation.
			t.Logf("STATUS:%s Initiating Certz rotation with server cert: %s and trust bundle: %s.", tc.desc, tc.serverCertFile, tc.trustBundleFile)
			if success := setup_service.CertzRotate(ctx, t, newCaCert, activeCertzClient, activeGNMIClient, newClientCert, dut, username, password, serverSAN, serverAddr, testProfile, tc.newTLScreds, tc.mismatch, tc.scale, &serverCertEntity, &trustBundleEntity); !success {
				t.Fatalf("STATUS:%s: Certz rotation failed.", tc.desc)
			}
			t.Logf("STATUS:%s: Certz rotation completed!", tc.desc)
			//Post rotate validation of all services.
			t.Run("Verification of new connection after rotate", func(t *testing.T) {
				if result := setup_service.ServicesValidationCheck(t, newCaCert, expectedResult, serverSAN, serverAddr, username, password, newClientCert, tc.mismatch); !result {
					t.Fatalf("STATUS:%s:service validation failed after rotate- got %v, want %v.", tc.desc, result, expectedResult)
				}
				t.Logf("STATUS:%s:service validation done!", tc.desc)
			})
			//Archiving previous client cert/key and trustbundle.
			prevClientCertFile = tc.clientCertFile
			prevClientKeyFile = tc.clientKeyFile
			prevTrustBundleFile = tc.trustBundleFile
		})
	}
	t.Logf("STATUS:Server certificate test completed!")
}
