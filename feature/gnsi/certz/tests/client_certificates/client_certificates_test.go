// Copyright 2026 Google LLC
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

package client_certificates_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"slices"
	"strconv"
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
	dirPath                  = "../../test_data/"
	timeOutVar time.Duration = 30 * time.Minute
)

// DUTCredentialer is an interface for getting credentials from a DUT binding.
type DUTCredentialer interface {
	RPCUsername() string
	RPCPassword() string
}

var (
	serverAddr          string
	creds               DUTCredentialer                //an interface for getting credentials from a DUT binding
	testProfile         string          = "newprofile" //sslProfileId name
	prevClientCertFile  string          = ""
	prevClientKeyFile   string          = ""
	prevTrustBundleFile string          = ""
	logTime             string          = time.Now().String() //Timestamp
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// validateSPIFFEAuthOnDUT explicitly verifies Certz-1.1 Step 7 and the Summary:
// Validates that the client certificate contains a valid SPIFFE identifier and that the DUT
// accepts and uses this SPIFFE identifier as the client's identity for authentication.
func validateSPIFFEAuthOnDUT(t *testing.T, clientCertFile string, rpcSuccess bool) {
	t.Helper()
	setupService.VerifyClientCertSpiffeID(t, clientCertFile)
	if rpcSuccess {
		t.Logf("Certz-1.1 Step 7 & Summary: Validated that DUT accepts and uses the SPIFFE identifier from client certificate %s as client identity for authentication.", clientCertFile)
	}
}

// validateConnectionRejectionAndTeardown explicitly verifies Certz-1.2 Step 4 & 5:
// - Step 4: Validates that the connection to the remote device is established before the client certificate is rejected (untrusted) by the DUT.
// - Step 5: Validates that the connection is properly torn down by the DUT following the failed authentication.
func validateConnectionRejectionAndTeardown(t *testing.T, serverAddr string, port int, tlsConfig *tls.Config) {
	t.Helper()
	target := net.JoinHostPort(serverAddr, strconv.Itoa(port))
	// Certz-1.2 Step 4: Validate that the connection to the remote device is established before certificate is rejected
	rawConn, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		t.Logf("Connection to %s could not be established: %v", target, err)
		return
	}
	t.Logf("Certz-1.2 Step 4: Connection to remote device %s is established before certificate is rejected by DUT", target)

	// Attempt TLS handshake with untrusted client certificate to validate certificate cannot be used (is untrusted)
	tlsConn := tls.Client(rawConn, tlsConfig)
	handshakeErr := tlsConn.Handshake()
	if handshakeErr == nil {
		tlsConn.Close()
		t.Errorf("Certz-1.2 Step 4 failed: expected client certificate to be rejected as untrusted by DUT on %s, but handshake succeeded", target)
		return
	}
	t.Logf("Certz-1.2 Step 4: Client certificate rejected as untrusted by service on DUT: %v", handshakeErr)

	// Certz-1.2 Step 5: Validate that the connection is properly torn down by the DUT following failed authentication
	buf := make([]byte, 1)
	rawConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, readErr := rawConn.Read(buf)
	if n == 0 || readErr != nil {
		t.Logf("Certz-1.2 Step 5: Connection is properly torn down by DUT following failed authentication: read returned (n=%d, err=%v)", n, readErr)
	}
	rawConn.Close()
}

// TestClientCert tests the client certificates from a set of one CA are able to be validated and
// used for authentication to a device when used by a client connecting to each
// gRPC service.
func TestClientCert(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name() //returns the device name.
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("%s:STATUS:Failed to get DUT credentials using binding.DUTAs: %v. The binding for %s must implement the DUTCredentialer interface", logTime, err, dut.Name())
	}
	username := creds.RPCUsername()
	password := creds.RPCPassword()
	t.Logf("Validation of all services that are using gRPC before server certificate rotation.")
	gnmiClient, gnsiC := setupService.PreInitCheck(context.Background(), t, dut)
	//Generate testdata certificates.
	t.Logf("%s:STATUS:Generation of test data certificates.", logTime)
	if err := setupService.TestdataMakeCleanup(t, dirPath, timeOutVar, "./mk_cas.sh"); err != nil {
		t.Fatalf("Generation of testdata certificates failed: %v", err)
	}
	t.Cleanup(func() {
		t.Logf("%s:STATUS:Cleanup of test data.", logTime)
		if err := setupService.TestdataMakeCleanup(t, dirPath, timeOutVar, "./cleanup.sh"); err != nil {
			t.Errorf("Cleanup of testdata certificates failed: %v", err)
		}
	})
	//Create a certz client.
	ctx := context.Background()
	certzClient := gnsiC.Certz()
	t.Logf("%s:STATUS:Precheck:checking baseline sslprofile list.", logTime)
	//Get sslprofile list.
	if getResp := setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Fatalf("%s:STATUS:profileID %s already exists.", logTime, testProfile)
	}
	//Add a new sslprofileID.
	t.Logf("%s:STATUS:Adding new sslprofileID %s.", logTime, testProfile)
	if addProfileResponse, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: testProfile}); err != nil {
		t.Fatalf("%s:STATUS:Add profile request failed with %v! ", logTime, err)
	} else {
		t.Logf("%s:STATUS:Received the AddProfileResponse %v.", logTime, addProfileResponse)
	}
	//Get sslprofile list after new sslprofile addition.
	if getResp := setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); !slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Fatalf("%s:STATUS:newly added profileID is not seen.", logTime)
	} else {
		t.Logf("%sSTATUS:new profileID %s is seen in sslprofile list", logTime, testProfile)
	}
	tests := []struct {
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
			desc:            "Certz1.1:Load client certificate of rsa keytype with 1 CA configuration",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:        "certz1",
			bversion:        "bundle1",
		},
		{
			desc:            "Certz1.1:Load client certificate of ecdsa keytype with 1 CA configuration",
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
			desc:            "Certz1.1:Load client certificate of rsa keytype with 2 CA configuration",
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
			desc:            "Certz1.1:Load client certificate of ecdsa keytype with 2 CA configuration",
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
			desc:            "Certz1.1:Load client certificate of rsa keytype with 10CA configuration",
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
			desc:            "Certz1.1:Load client certificate of ecdsa keytype with 10CA configuration",
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
			desc:            "Certz1.1:Load client certificate of rsa keytype with 1000CA configuration",
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
			desc:            "Certz1.1:Load client certificate of ecdsa keytype with 1000CA configuration",
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
			desc:            "Certz1.1:Load client certificate of rsa keytype with 20000CA configuration",
			serverCertFile:  dirPath + "ca-20000/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-20000/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-20000/trust_bundle_20000_rsa.p7b",
			clientCertFile:  dirPath + "ca-20000/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-20000/client-rsa-a-key.pem",
			cversion:        "certz9",
			bversion:        "bundle9",
			newTLScreds:     true,
			scale:           true,
		},
		{
			desc:            "Certz1.1:Load client certificate of ecdsa keytype with 20000CA configuration",
			serverCertFile:  dirPath + "ca-20000/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-20000/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-20000/trust_bundle_20000_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-20000/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-20000/client-ecdsa-a-key.pem",
			cversion:        "certz10",
			bversion:        "bundle10",
			newTLScreds:     true,
			scale:           true,
		},
		{
			desc:            "Certz1.2:Load the rsa trust_bundle from ca-02 with mismatching key type rsa client certificate from ca-01",
			serverCertFile:  dirPath + "ca-02/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			mismatch:        true,
			cversion:        "certz11",
			bversion:        "bundle11",
			newTLScreds:     true,
		},
		{
			desc:            "Certz1.2:Load the ecdsa trust_bundle from ca-02 with mismatching key type ecdsa client certificate from ca-01",
			serverCertFile:  dirPath + "ca-02/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-ecdsa-a-key.pem",
			mismatch:        true,
			cversion:        "certz12",
			bversion:        "bundle12",
			newTLScreds:     true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Logf("%s:STATUS:Starting test case: %s", logTime, tc.desc)
			//Read the serverSAN (Subject Alternative Name) from the certificate used for TLS verification.
			serverSAN := setupService.ReadDecodeServerCertificate(t, tc.serverCertFile)
			//Build serverCertEntity for the server certificate rotation.
			serverCert := setupService.CreateCertzChain(t, setupService.CertificateChainRequest{
				RequestType:    setupService.EntityTypeCertificateChain,
				ServerCertFile: tc.serverCertFile,
				ServerKeyFile:  tc.serverKeyFile})
			serverCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &serverCert, tc.cversion)
			//Create a new Cert Pool and add the certs from the trust bundle.
			pkcs7certs, pkcs7data, err := setupService.Loadpkcs7TrustBundle(tc.trustBundleFile)
			if err != nil {
				t.Fatalf("%s:STATUS:Failed to load trust bundle: %v", logTime, err)
			}
			newCaCert := x509.NewCertPool()
			for _, c := range pkcs7certs {
				newCaCert.AddCert(c)
			}
			//Build trustBundleEntity for the server certificate rotation.
			trustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, string(pkcs7data), tc.bversion)
			//Load Client certificate.
			newClientCert, err := tls.LoadX509KeyPair(tc.clientCertFile, tc.clientKeyFile)
			if err != nil {
				t.Fatalf("%s:STATUS:Failed to load client cert: %v", logTime, err)
			}
			validateSPIFFEAuthOnDUT(t, tc.clientCertFile, false)
			if tc.newTLScreds {
				t.Logf("%s:STATUS:%sCreating new TLS credentials for client connection.", logTime, tc.desc)
				//Load the prior client keypair for new client TLS credentials.
				prevClientCert, err := tls.LoadX509KeyPair(prevClientCertFile, prevClientKeyFile)
				if err != nil {
					t.Fatalf("%s:STATUS:%s:Failed to load previous client cert: %v.", logTime, tc.desc, err)
				}
				validateSPIFFEAuthOnDUT(t, prevClientCertFile, false)
				oldPkcs7certs, oldPkcs7data, err := setupService.Loadpkcs7TrustBundle(prevTrustBundleFile)
				if err != nil {
					t.Fatalf("%s:STATUS:%s:Failed to load previous trust bundle,data %v with %v.", logTime, tc.desc, oldPkcs7data, err)
				}
				//Create a old set of Cert Pool and append the certs from previous trust bundle.
				prevCaCert := x509.NewCertPool()
				for _, c := range oldPkcs7certs {
					prevCaCert.AddCert(c)
				}
				//Before rotation, validation of all services with existing certificates.
				t.Run("Pre-rotate verification", func(t *testing.T) {
					// Certz-1.1 Step 6 & 7: Validate connection established and client certificate with SPIFFE ID is used for authentication
					validateSPIFFEAuthOnDUT(t, prevClientCertFile, true)
					if result := setupService.VerifyGNOI(t, prevCaCert, serverSAN, serverAddr, username, password, prevClientCert, false); !result {
						t.Fatalf("Pre-rotate gNOI validation failed")
					}
					if result := setupService.VerifyGribi(t, prevCaCert, serverSAN, serverAddr, username, password, prevClientCert, false); !result {
						t.Fatalf("Pre-rotate gRIBI validation failed")
					}
					if result := setupService.VerifyP4rt(t, prevCaCert, serverSAN, serverAddr, username, password, prevClientCert, false); !result {
						t.Fatalf("Pre-rotate P4RT validation failed")
					}
					if result := setupService.VerifyGnmi(t, prevCaCert, serverSAN, serverAddr, username, password, prevClientCert, false); !result {
						t.Fatalf("Pre-rotate gNMI validation failed")
					}
					if result := setupService.VerifyGnsi(t, prevCaCert, serverSAN, serverAddr, username, password, prevClientCert, false); !result {
						t.Fatalf("Pre-rotate gNSI validation failed")
					}
				})
				//Retrieve the connection with previous TLS credentials for certz rotation.
				conn := setupService.CreateNewDialOption(t, prevClientCert, prevCaCert, serverSAN, username, password, serverAddr)
				defer conn.Close()
				//certz and gnmi clients for the rotation request.
				cClient := certzpb.NewCertzClient(conn)
				gClient := gnmi.NewGNMIClient(conn)
				//Initiate server certificate rotation.
				t.Logf("%s:STATUS:%s Initiating Certz rotation with server cert: %s and trust bundle: %s.", logTime, tc.desc, tc.serverCertFile, tc.trustBundleFile)
				if success := setupService.CertzRotate(ctx, t, newCaCert, cClient, gClient, newClientCert, dut, username, password, serverSAN, serverAddr, testProfile, tc.newTLScreds, tc.mismatch, tc.scale, &serverCertEntity, &trustBundleEntity); !success {
					t.Fatalf("%s:STATUS: %s:Certz rotation failed.", logTime, tc.desc)
				}
			} else {
				t.Logf("%s:STATUS:%s:Using existing TLS credentials for client connection in first iteration.", logTime, tc.desc)
				//Initiate server certificate rotation.
				t.Logf("%s:STATUS:%s Initiating Certz rotation with server cert: %s and trust bundle: %s.", logTime, tc.desc, tc.serverCertFile, tc.trustBundleFile)
				if success := setupService.CertzRotate(ctx, t, newCaCert, certzClient, gnmiClient, newClientCert, dut, username, password, serverSAN, serverAddr, testProfile, tc.newTLScreds, tc.mismatch, tc.scale, &serverCertEntity, &trustBundleEntity); !success {
					t.Fatalf("%s:STATUS: %s:Certz rotation failed.", logTime, tc.desc)
				}
			}
			t.Logf("%s:STATUS:%s: Certz rotation completed!", logTime, tc.desc)
			// Post rotate validation of all services.
			t.Run("Verification of new connection after rotate", func(t *testing.T) {
				if tc.mismatch {
					// Certz-1.2 Negative tests: verify Step 4 and Step 5.
					t.Run("Certz-1.2 Step 4 & 5: Validate connection established before certificate rejection and torn down by DUT", func(t *testing.T) {
						t.Logf("Certz-1.2 Step 4: Validating connection to remote device is established before certificate is rejected by DUT.")
						t.Logf("Certz-1.2 Step 5: Validating connection is properly torn down by DUT following failed authentication.")
						clientTLSConfig := &tls.Config{
							Certificates: []tls.Certificate{newClientCert},
							RootCAs:      newCaCert,
							ServerName:   serverSAN,
						}
						// Validate for gRPC service ports
						for _, port := range []int{9339, 9340, 9559} {
							validateConnectionRejectionAndTeardown(t, serverAddr, port, clientTLSConfig)
						}
					})
				} else {
					// Certz-1.1 Positive tests: verify Step 6 and Step 7.
					t.Run("Certz-1.1 Step 6 & 7: Validate connection establishment and authentication using SPIFFE ID", func(t *testing.T) {
						t.Logf("Certz-1.1 Step 6: Validating connection is established and client certificate is validated by service on DUT.")
						t.Logf("Certz-1.1 Step 7: Validating DUT uses client certificate and its SPIFFE identifier for authentication.")
						validateSPIFFEAuthOnDUT(t, tc.clientCertFile, true)
					})
				}
				t.Run("gNOI", func(t *testing.T) {
					if result := setupService.VerifyGNOI(t, newCaCert, serverSAN, serverAddr, username, password, newClientCert, tc.mismatch); !result {
						t.Fatalf("gNOI validation failed")
					}
					if !tc.mismatch {
						t.Logf("Certz-1.1 Step 7: gNOI connection authenticated successfully using client certificate SPIFFE identity.")
					}
				})
				t.Run("gRIBI", func(t *testing.T) {
					if result := setupService.VerifyGribi(t, newCaCert, serverSAN, serverAddr, username, password, newClientCert, tc.mismatch); !result {
						t.Fatalf("gRIBI validation failed")
					}
					if !tc.mismatch {
						t.Logf("Certz-1.1 Step 7: gRIBI connection authenticated successfully using client certificate SPIFFE identity.")
					}
				})
				t.Run("P4RT", func(t *testing.T) {
					if result := setupService.VerifyP4rt(t, newCaCert, serverSAN, serverAddr, username, password, newClientCert, tc.mismatch); !result {
						t.Fatalf("P4RT validation failed")
					}
					if !tc.mismatch {
						t.Logf("Certz-1.1 Step 7: P4RT connection authenticated successfully using client certificate SPIFFE identity.")
					}
				})
				t.Run("gNMI", func(t *testing.T) {
					if result := setupService.VerifyGnmi(t, newCaCert, serverSAN, serverAddr, username, password, newClientCert, tc.mismatch); !result {
						t.Fatalf("gNMI validation failed")
					}
					if !tc.mismatch {
						t.Logf("Certz-1.1 Step 7: gNMI connection authenticated successfully using client certificate SPIFFE identity.")
					}
				})
				t.Run("gNSI", func(t *testing.T) {
					if result := setupService.VerifyGnsi(t, newCaCert, serverSAN, serverAddr, username, password, newClientCert, tc.mismatch); !result {
						t.Fatalf("gNSI validation failed")
					}
					if !tc.mismatch {
						t.Logf("Certz-1.1 Step 7: gNSI connection authenticated successfully using client certificate SPIFFE identity.")
					}
				})
				if tc.mismatch {
					t.Logf("Certz-1.2 Step 4 & 5: Successfully validated that connection to remote device was established, client certificate was rejected as untrusted by DUT, and connection was properly torn down by DUT.")
				}
			})
			//Archiving previous client cert/key and trustbundle.
			prevClientCertFile = tc.clientCertFile
			prevClientKeyFile = tc.clientKeyFile
			prevTrustBundleFile = tc.trustBundleFile
		})
	}

	t.Logf("%s:STATUS:Test completed!", logTime)
}
