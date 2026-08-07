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

package trustbundlerotatation_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"os/exec"
	"path/filepath"
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

func buildCombinedTrustBundle(dst string, dirs []string, keyType string) error {
	var certFiles []string
	for _, d := range dirs {
		pattern := filepath.Join(dirPath, "ca-"+d, fmt.Sprintf("ca-*-%s-cert.pem", keyType))
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		if len(matches) == 0 {
			return fmt.Errorf("no cert files found for pattern %q", pattern)
		}
		certFiles = append(certFiles, matches...)
	}

	args := []string{"crl2pkcs7", "-nocrl"}
	for _, cert := range certFiles {
		args = append(args, "-certfile", cert)
	}
	args = append(args, "-out", dst)

	cmd := exec.Command("openssl", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("openssl failed: %v, output: %s", err, string(out))
	}
	return nil
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestTrustBundleRotation tests the rotation of trust bundles on both clients and servers as new CA information is added to the trust_bundles over time.
// ca-01/ca-02 of both rsa and ecdsa keytype.
func TestTrustBundleRotation(t *testing.T) {

	var (
		serverAddr          string
		creds               DUTCredentialer                //an interface for getting credentials from a DUT binding.
		testProfile         string          = "newprofile" //sslProfileId name
		prevClientCertFile  string          = ""
		prevClientKeyFile   string          = ""
		prevTrustBundleFile string          = ""
		expectedResult      bool            = true
		certsList                           = flag.String("certsList", "01,02", "Number of Certificate Sets to generate for this test. Comma separated string")
		certsTimeout                        = flag.Duration("certsTimeout", 10*time.Minute, "Time duration for cert generation and cleanup. Increase if more certs are to be generated")
	)
	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name() //returns the device name.
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("STATUS:Failed to get DUT credentials using binding.DUTAs: %v. The binding for %s must implement the DUTCredentialer interface.", err, dut.Name())
	}
	username := creds.RPCUsername()
	password := creds.RPCPassword()
	t.Logf("STATUS:Validation of all services that are using gRPC before certz rotation.")
	gnmiClient, gnsiC := setup_service.PreInitCheck(context.Background(), t, dut)
	// Generate testdata certificates.
	t.Logf("Creation of test data.")
	//Registering the cleanup before the certificate generation call, so it runs even if certificate generation fails.
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
	//Add new sslprofileID.
	t.Logf("Adding new empty sslprofile ID %s.", testProfile)
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
		desc             string
		serverCertFile   string
		serverKeyFile    string
		trustBundleFile  string
		trustBundleFileA string
		trustBundleFileB string
		clientCertFile   string
		clientKeyFile    string
		cversion         string
		bversion         string
		newTLScreds      bool
		mismatch         bool
		scale            bool
		concatenate      bool
	}{
		{
			desc:            "Certz5.1:Load the key-type rsa trustbundle with 1 CA configuration from ca-01",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:        "v1",
			bversion:        "bundle1",
			concatenate:     false,
		},
		{
			desc:             "Certz5.1:Load the key-type rsa trustbundle with concatenated CA configuration from ca-01 and ca-02",
			serverCertFile:   dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:    dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFileA: dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			trustBundleFileB: dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			trustBundleFile:  dirPath + "ca-01/trust_bundle_new_rsa.p7b",
			clientCertFile:   dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:    dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:         "v2",
			bversion:         "bundle2",
			newTLScreds:      true,
			concatenate:      true,
		},
		{
			desc:            "Certz5.2:Negative Test: Load the key-type rsa trustbundle with 1 CA configuration from ca-01",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:        "v3",
			bversion:        "bundle3",
			newTLScreds:     true,
		},
		{
			desc:            "Certz5.2:Negative Test: Load the key-type rsa trustbundle with 1 CA configuration from ca-02",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:        "v4",
			bversion:        "bundle4",
			newTLScreds:     true,
		},
	}
	var failed bool
	for _, tc := range cases {
		if failed {
			t.Skip("Skipping remaining cases due to previous failure")
		}
		t.Run(tc.desc, func(t *testing.T) {
			defer func() {
				if t.Failed() {
					failed = true
				}
			}()
			t.Logf("STATUS:Starting test case: %s", tc.desc)
			//Read the serverSAN (Subject Alternative Name) from the certificate used for TLS verification.
			serverSAN := setup_service.ReadDecodeServerCertificate(t, tc.serverCertFile)
			//Build serverCertEntity for the server certificate rotation.
			serverCert := setup_service.CreateCertzChain(t, setup_service.CertificateChainRequest{
				RequestType:    setup_service.EntityTypeCertificateChain,
				ServerCertFile: tc.serverCertFile,
				ServerKeyFile:  tc.serverKeyFile})
			serverCertEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeCertificateChain, &serverCert, tc.cversion)
			if tc.concatenate {
				t.Logf("STATUS:%s:Building combined trust bundle for rotation.", tc.desc)
				if err := buildCombinedTrustBundle(tc.trustBundleFile, []string{"01", "02"}, "rsa"); err != nil {
					t.Fatalf("STATUS:%s:failed to build combined trust bundle: %v", tc.desc, err)
				}
			}
			//Create a new Cert Pool and add the certs from the trust bundle A .
			pkcs7certs, pkcs7data, err := setup_service.Loadpkcs7TrustBundle(tc.trustBundleFile)
			if err != nil {
				t.Fatalf("STATUS:%s:failed to load trust bundle: %v", tc.desc, err)
			}
			newCaCert := x509.NewCertPool()
			for _, c := range pkcs7certs {
				newCaCert.AddCert(c)
			}
			//Build trustBundleEntity for the server certificate rotation.
			trustBundleEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeTrustBundle, string(pkcs7data), tc.bversion)
			//Load Client certificate.
			newClientCert, err := tls.LoadX509KeyPair(tc.clientCertFile, tc.clientKeyFile)
			if err != nil {
				t.Fatalf("STATUS:%s:Failed to load client cert:%v", tc.desc, err)
			}
			activeCertzClient := certzClient
			activeGNMIClient := gnmiClient
			if tc.newTLScreds {
				t.Logf("STATUS:%s:Creating new TLS credentials for client connection", tc.desc)
				prevClientCert, err := tls.LoadX509KeyPair(prevClientCertFile, prevClientKeyFile)
				if err != nil {
					t.Fatalf("STATUS:%s:Failed to load previous client cert: %v", tc.desc, err)
				}
				oldPkcs7certs, _, err := setup_service.Loadpkcs7TrustBundle(prevTrustBundleFile)
				if err != nil {
					t.Fatalf("STATUS:%s failed to load previous trust bundle: %v", tc.desc, err)
				}
				prevCaCert := x509.NewCertPool()
				for _, c := range oldPkcs7certs {
					prevCaCert.AddCert(c)
				}
				if result := setup_service.ServicesValidationCheck(t, prevCaCert, expectedResult, serverSAN, serverAddr, username, password, prevClientCert, tc.mismatch); !result {
					t.Fatalf("STATUS:%s:service validation failed before rotate- got %v, want %v.", tc.desc, result, expectedResult)
				}
				conn := setup_service.CreateNewDialOption(t, prevClientCert, prevCaCert, serverSAN, username, password, serverAddr)
				defer conn.Close()
				activeCertzClient = certzpb.NewCertzClient(conn)
				activeGNMIClient = gnmi.NewGNMIClient(conn)
			} else {
				t.Logf("STATUS:%s:Using existing TLS credentials for client connection in first iteration.", tc.desc)
			}
			//Initiating trustbundle rotation.
			t.Logf("STATUS:%s Initiating Certz rotation with server cert: %s and trust bundle: %s.", tc.desc, tc.serverCertFile, tc.trustBundleFile)
			if success := setup_service.CertzRotate(ctx, t, newCaCert, activeCertzClient, activeGNMIClient, newClientCert, dut, username, password, serverSAN, serverAddr, testProfile, tc.newTLScreds, tc.mismatch, tc.scale, &serverCertEntity, &trustBundleEntity); !success {
				t.Fatalf("STATUS:%s:CertzRotation failed.", tc.desc)
			}
			t.Logf("STATUS:%s: TrustBundle rotation completed!", tc.desc)
			//Post rotate validation of all services.
			t.Run("Verification of new connection after successful trustBundle rotation", func(t *testing.T) {
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
	t.Logf("STATUS:Trustbundle rotation test completed!")
}
