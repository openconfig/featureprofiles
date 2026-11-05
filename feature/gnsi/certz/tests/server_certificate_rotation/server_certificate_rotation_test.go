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

package server_certificate_rotation_test

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

// DUTCredentialer is an interface for getting credentials from
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
	expectedResult      bool            = true
	success             bool
	certsList           = flag.String("certsList", "01,02,1,1000", "Number of Certificate Sets to generate for this test. Comma separated string")
	certsTimeout        = flag.Duration("certsTimeout", 10*time.Minute, "Time duration for cert generation and cleanup. Increase if more certs are to be generated")

	certsString = func() string {
		return *certsList
	}
	certsTimeOutVar = func() time.Duration {
		return *certsTimeout
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestServerCertRotation tests a server certificate can be rotated by using the gNSI certz Rotate() rpc,
// if the certificate is requested without the device generated CSR.
func TestServerCertRotation(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name() //returns the device name.
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("%s:STATUS:Failed to get DUT credentials using binding.DUTAs: %v. The binding for %s must implement the DUTCredentialer interface.", time.Now().String(), err, dut.Name())
	}
	username := creds.RPCUsername()
	password := creds.RPCPassword()
	t.Logf("%s:STATUS:Validation of all services that are using gRPC before certz rotation.", time.Now().String())
	gnmiClient, gnsiC := setup_service.PreInitCheck(context.Background(), t, dut)
	//Generate testdata certificates.
	t.Logf("%s:Creation of test data.", time.Now().String())
	command := fmt.Sprintf("./mk_cas.sh %v", certsString())
	if err := setup_service.TestdataMakeCleanup(t, dirPath, certsTimeOutVar(), command); err != nil {
		t.Fatalf("%s:STATUS:Generation of testdata certificates failed!: %v", time.Now().String(), err)
	}
	//Create a certz client.
	ctx := context.Background()
	certzClient := gnsiC.Certz()
	t.Logf("%s:STATUS:Precheck:checking baseline sslprofile list.", time.Now().String())
	//Get sslprofile list.
	if getResp := setup_service.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Fatalf("%s:STATUS:profileID %s already exists.", time.Now().String(), testProfile)
	}
	//Add new sslprofileID.
	t.Logf("%s:Adding new empty sslprofile ID %s.", time.Now().String(), testProfile)
	if addProfileResponse, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: testProfile}); err != nil {
		t.Fatalf("%s:STATUS:Add profile request failed with %v! ", time.Now().String(), err)
	} else {
		t.Logf("%s:STATUS:Received the AddProfileResponse %v.", time.Now().String(), addProfileResponse)
	}
	//Get sslprofile list after new sslprofile addition.
	if getResp := setup_service.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); !slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Fatalf("%s:STATUS:newly added profileID is not seen.", time.Now().String())
	} else {
		t.Logf("%s:STATUS:new profileID %s is seen in sslprofile list", time.Now().String(), testProfile)
	}
	cases := []struct {
		desc                 string
		serverCertFile       string
		serverKeyFile        string
		trustBundleFile      string
		clientCertFile       string
		clientKeyFile        string
		cversion             string
		bversion             string
		newTLScreds          bool
		serverCertOnlyRotate bool
		mismatch             bool
		scale                bool
	}{
		{
			desc:            "Certz3.1:Rotate server-rsa-a certificate/key/trustbundle from ca-01",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:        "v1",
			bversion:        "bundle1",
		},
		{
			desc:                 "Certz3.1:Rotate server-rsa-b certificate/key/trustbundle from ca-01",
			serverCertFile:       dirPath + "ca-01/server-rsa-b-cert.pem",
			serverKeyFile:        dirPath + "ca-01/server-rsa-b-key.pem",
			trustBundleFile:      dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			clientCertFile:       dirPath + "ca-01/client-rsa-b-cert.pem",
			clientKeyFile:        dirPath + "ca-01/client-rsa-b-key.pem",
			cversion:             "v2",
			bversion:             "bundle1",
			serverCertOnlyRotate: true,
			newTLScreds:          true,
		},
		{
			desc:            "Certz3.1:Rotate server-ecdsa-a certificate/key/trustbundle from ca-01",
			serverCertFile:  dirPath + "ca-01/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-ecdsa-a-key.pem",
			cversion:        "v3",
			bversion:        "bundle2",
			newTLScreds:     true,
		},
		{
			desc:                 "Certz3.1:Rotate server-ecdsa-b certificate/key/trustbundle from ca-01",
			serverCertFile:       dirPath + "ca-01/server-ecdsa-b-cert.pem",
			serverKeyFile:        dirPath + "ca-01/server-ecdsa-b-key.pem",
			trustBundleFile:      dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			clientCertFile:       dirPath + "ca-01/client-ecdsa-b-cert.pem",
			clientKeyFile:        dirPath + "ca-01/client-ecdsa-b-key.pem",
			cversion:             "v4",
			bversion:             "bundle2",
			serverCertOnlyRotate: true,
			newTLScreds:          true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Logf("%s:STATUS:Starting test case: %s", time.Now().String(), tc.desc)
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
				t.Fatalf("%s:STATUS:failed to load trust bundle: %v", time.Now().String(), err)
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
				t.Fatalf("%s:STATUS:Failed to load client cert:%v", time.Now().String(), err)
			}
			if tc.newTLScreds {
				t.Logf("%s:STATUS:%s:Creating new TLS credentials for client connection.", time.Now().String(), tc.desc)
				//Load the prior client keypair for new client TLS credentials.
				prevClientCert, err := tls.LoadX509KeyPair(prevClientCertFile, prevClientKeyFile)
				if err != nil {
					t.Fatalf("%s:STATUS:%s:Failed to load previous client cert: %v", time.Now().String(), tc.desc, err)
				}
				oldPkcs7certs, oldPkcs7data, err := setup_service.Loadpkcs7TrustBundle(prevTrustBundleFile)
				if err != nil {
					t.Fatalf("%s:STATUS:%sFailed to load previous trust bundle,data %v with %v", time.Now().String(), tc.desc, oldPkcs7data, err)
				}
				//Create a old set of Cert Pool and append the certs from previous trust bundle.
				prevCaCert := x509.NewCertPool()
				for _, c := range oldPkcs7certs {
					prevCaCert.AddCert(c)
				}
				//Before rotation, validation of all services with existing certificates.
				if result := setup_service.ServicesValidationCheck(t, prevCaCert, expectedResult, serverSAN, serverAddr, username, password, prevClientCert, tc.mismatch); !result {
					t.Fatalf("%s:STATUS:%s:service validation failed before rotate- got %v, want %v.", time.Now().String(), tc.desc, result, expectedResult)
				}
				//Retrieve the connection with previous TLS credentials for certz rotation.
				conn := setup_service.CreateNewDialOption(t, prevClientCert, prevCaCert, serverSAN, username, password, serverAddr)
				defer conn.Close()
				certzClient = certzpb.NewCertzClient(conn)
				gnmiClient = gnmi.NewGNMIClient(conn)
			} else {
				t.Logf("%s:STATUS:%s:Using existing TLS credentials for client connection in first iteration.", time.Now().String(), tc.desc)
			}
			//Initiate server certitificate rotation.
			if tc.serverCertOnlyRotate {
				t.Logf("%s:STATUS:%s:Initiating server certificate rotation to server-${TYPE}-b.", time.Now().String(), tc.desc)
				if success = setup_service.CertzRotate(ctx, t, newCaCert, certzClient, gnmiClient, newClientCert, dut, username, password, serverSAN, serverAddr, testProfile, tc.newTLScreds, tc.mismatch, tc.scale, &serverCertEntity); !success {
					t.Fatalf("%s STATUS %s:Server certificate rotation failed.", time.Now().String(), tc.desc)
				}
			} else {
				t.Logf("%s:STATUS:%s Initiating Certz rotation with server cert: %s and trust bundle: %s", time.Now().String(), tc.desc, tc.serverCertFile, tc.trustBundleFile)
				if success = setup_service.CertzRotate(ctx, t, newCaCert, certzClient, gnmiClient, newClientCert, dut, username, password, serverSAN, serverAddr, testProfile, tc.newTLScreds, tc.mismatch, tc.scale, &serverCertEntity, &trustBundleEntity); !success {
					t.Fatalf("%s STATUS %s:Server certificate rotation failed.", time.Now().String(), tc.desc)
				}
			}
			t.Logf("%s:STATUS:%s:Server certificate rotation completed!", time.Now().String(), tc.desc)
			t.Run("Verification of new connection after successful server certificate rotation", func(t *testing.T) {
				if result := setup_service.ServicesValidationCheck(t, newCaCert, expectedResult, serverSAN, serverAddr, username, password, newClientCert, tc.mismatch); !result {
					t.Fatalf("%s:STATUS:%s:service validation failed after rotate- got %v, want %v.", time.Now().String(), tc.desc, result, expectedResult)
				}
				t.Logf("%s:STATUS:%s:service validation done!", time.Now().String(), tc.desc)
			})
			//Archiving previous client cert/key and trustbundle.
			prevClientCertFile = tc.clientCertFile
			prevClientKeyFile = tc.clientKeyFile
			prevTrustBundleFile = tc.trustBundleFile
		})
	}
	t.Logf("%s:STATUS:Cleanup of test data.", time.Now().String())
	//Cleanup of test data.
	if err := setup_service.TestdataMakeCleanup(t, dirPath, certsTimeOutVar(), "./cleanup.sh"); err != nil {
		t.Logf("%s:STATUS:Cleanup of testdata certificates failed!: %v", time.Now().String(), err)
	}
	t.Logf("%s:STATUS: Testdata cleanup completed!", time.Now().String())
}
