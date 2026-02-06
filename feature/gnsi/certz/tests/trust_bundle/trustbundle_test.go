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

package trustbundle_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"slices"
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
	timeOutVar time.Duration = 2 * time.Minute
)

// DUTCredentialer is an interface for getting credentials from a DUT binding.
type DUTCredentialer interface {
	RPCUsername() string
	RPCPassword() string
}

var (
	serverAddr          string
	creds               DUTCredentialer                //an interface for getting credentials from a DUT binding.
	testProfile         string          = "newprofile" //sslProfileId name
	prevClientCertFile  string          = ""
	prevClientKeyFile   string          = ""
	prevTrustBundleFile string          = ""
	logTime             string          = time.Now().String() //Timestamp
	expectedResult      bool            = true
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestTrustBundle tests the load of server certificate and key from each of the following CA sets
// ca-01/ca-02/ca-10/ca-1000 of both rsa and ecdsa keytype.
func TestTrustBundleCert(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name() //returns the device name.
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("%s:STATUS:Failed to get DUT credentials using binding.DUTAs: %v. The binding for %s must implement the DUTCredentialer interface.", logTime, err, dut.Name())
	}
	username := creds.RPCUsername()
	password := creds.RPCPassword()
	t.Logf("%s:STATUS:Validation of all services that are using gRPC before certz rotation.", logTime)
	gnmiClient, gnsiC := setupService.PreInitCheck(context.Background(), t, dut)
	//Generate testdata certificates.
	t.Logf("%s:Creation of test data.", logTime)
	if err := setupService.TestdataMakeCleanup(t, dirPath, timeOutVar, "./mk_cas.sh"); err != nil {
		t.Logf("%s:STATUS:Generation of testdata certificates failed!: %v", logTime, err)
	}
	//Create a certz client.
	ctx := context.Background()
	certzClient := gnsiC.Certz()
	t.Logf("%s:STATUS:Precheck:checking baseline sslprofile list.", logTime)
	//Get sslprofile list.
	if getResp := setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Fatalf("%s:STATUS:profileID %s already exists.", logTime, testProfile)
	}
	//Add new sslprofileID.
	t.Logf("%s:Adding new empty sslprofile ID %s.", logTime, testProfile)
	if addProfileResponse, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: testProfile}); err != nil {
		t.Fatalf("%s:STATUS:Add profile request failed with %v! ", logTime, err)
	} else {
		t.Logf("%s:STATUS:Received the AddProfileResponse %v.", logTime, addProfileResponse)
	}
	//Get sslprofile list after new sslprofile addition.
	if getResp := setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); !slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Fatalf("%s:STATUS:newly added profileID is not seen.", logTime)
	} else {
		t.Logf("%s:STATUS:new profileID %s is seen in sslprofile list", logTime, testProfile)
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
			desc:            "Certz4.1:Load the key-type rsa trustbundle with 1 CA configuration",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:        "v1",
			bversion:        "bundle1",
		},
		{
			desc:            "Certz4.1:Load the key-type ecdsa trustbundle with 1 CA configuration",
			serverCertFile:  dirPath + "ca-01/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-ecdsa-a-key.pem",
			cversion:        "v2",
			bversion:        "bundle2",
			newTLScreds:     true,
		},
		{
			desc:            "Certz4.1:Load the key-type rsa trustbundle with 2 CA configuration",
			serverCertFile:  dirPath + "ca-02/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile:  dirPath + "ca-02/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-02/client-rsa-a-key.pem",
			cversion:        "v3",
			bversion:        "bundle3",
			newTLScreds:     true,
		},
		{
			desc:            "Certz4.1:Load the key-type ecdsa trustbundle with 2 CA configuration",
			serverCertFile:  dirPath + "ca-02/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-02/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-02/client-ecdsa-a-key.pem",
			cversion:        "v4",
			bversion:        "bundle4",
			newTLScreds:     true,
		},
		{
			desc:            "Certz4.1:Load the key-type rsa trustbundle with 10CA configuration",
			serverCertFile:  dirPath + "ca-10/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-10/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-10/trust_bundle_10_rsa.p7b",
			clientCertFile:  dirPath + "ca-10/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-10/client-rsa-a-key.pem",
			cversion:        "v5",
			bversion:        "bundle5",
			newTLScreds:     true,
		},
		{
			desc:            "Certz4.1:Load the key-type ecdsa trustbundle with 10CA configuration",
			serverCertFile:  dirPath + "ca-10/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-10/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-10/trust_bundle_10_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-10/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-10/client-ecdsa-a-key.pem",
			cversion:        "v6",
			bversion:        "bundle6",
			newTLScreds:     true,
		},
		{
			desc:            "Certz4.1:Load the key-type rsa trustbundle with 1000CA configuration",
			serverCertFile:  dirPath + "ca-1000/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-1000/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-1000/trust_bundle_1000_rsa.p7b",
			clientCertFile:  dirPath + "ca-1000/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-1000/client-rsa-a-key.pem",
			cversion:        "v7",
			bversion:        "bundle7",
			newTLScreds:     true,
			scale:           true,
		},
		{
			desc:            "Certz4.1:Load the key-type ecdsa trustbundle with 1000CA configuration",
			serverCertFile:  dirPath + "ca-1000/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-1000/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-1000/trust_bundle_1000_ecdsa.p7b",
			clientCertFile:  dirPath + "ca-1000/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-1000/client-ecdsa-a-key.pem",
			cversion:        "v8",
			bversion:        "bundle8",
			newTLScreds:     true,
			scale:           true,
		},
	}
	for _, tc := range cases {
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
				t.Fatalf("%s:STATUS:failed to load trust bundle: %v", logTime, err)
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
				t.Fatalf("%s:STATUS:Failed to load client cert:%v", logTime, err)
			}
			if tc.newTLScreds {
				t.Logf("%s:STATUS:%s:Creating new TLS credentials for client connection.", logTime, tc.desc)
				//Load the prior client keypair for new client TLS credentials.
				prevClientCert, err := tls.LoadX509KeyPair(prevClientCertFile, prevClientKeyFile)
				if err != nil {
					t.Fatalf("%s:STATUS:%s:Failed to load previous client cert: %v", logTime, tc.desc, err)
				}
				oldPkcs7certs, oldPkcs7data, err := setupService.Loadpkcs7TrustBundle(prevTrustBundleFile)
				if err != nil {
					t.Fatalf("%s:STATUS:%sFailed to load previous trust bundle,data %v with %v", logTime, tc.desc, oldPkcs7data, err)
				}
				//Create a old set of Cert Pool and append the certs from previous trust bundle.
				prevCaCert := x509.NewCertPool()
				for _, c := range oldPkcs7certs {
					prevCaCert.AddCert(c)
				}
				//Before rotation,validation of all services with existing certificates.
				if result := setupService.ServicesValidationCheck(t, prevCaCert, expectedResult, serverSAN, serverAddr, username, password, prevClientCert, tc.mismatch); !result {
					t.Fatalf("%s:STATUS:%s:service validation failed before rotate- got %v, want %v.", logTime, tc.desc, result, expectedResult)
				}
				//Retrieve the connection with previous TLS credentials for certz rotation.
				conn := setupService.CreateNewDialOption(t, prevClientCert, prevCaCert, serverSAN, username, password, serverAddr)
				defer conn.Close()
				certzClient = certzpb.NewCertzClient(conn)
				gnmiClient = gnmi.NewGNMIClient(conn)
			} else {
				t.Logf("%s:STATUS:%s:Using existing TLS credentials for client connection in first iteration.", logTime, tc.desc)
			}
			//Initiate trustbundle rotation.
			t.Logf("STATUS:%s Initiating Certz rotation with server cert: %s and trust bundle: %s.", tc.desc, tc.serverCertFile, tc.trustBundleFile)
			if success := setupService.CertzRotate(ctx, t, newCaCert, certzClient, gnmiClient, newClientCert, dut, username, password, serverSAN, serverAddr, testProfile, tc.newTLScreds, tc.mismatch, tc.scale, &serverCertEntity, &trustBundleEntity); !success {
				t.Fatalf("%s:STATUS: %s:CertzRotation failed.", logTime, tc.desc)
			}
			t.Logf("%s:STATUS:%s: TrustBundle rotation completed!", logTime, tc.desc)
			//Post rotate validation of all services.
			t.Run("Verification of new connection after successful trustBundle rotation", func(t *testing.T) {
				if result := setupService.ServicesValidationCheck(t, newCaCert, expectedResult, serverSAN, serverAddr, username, password, newClientCert, tc.mismatch); !result {
					t.Fatalf("STATUS:%s:service validation failed after rotate- got %v, want %v.", tc.desc, result, expectedResult)
				}
				t.Logf("%s:STATUS:%s:service validation done!", logTime, tc.desc)
			})
			//Archiving previous client cert/key and trustbundle.
			prevClientCertFile = tc.clientCertFile
			prevClientKeyFile = tc.clientKeyFile
			prevTrustBundleFile = tc.trustBundleFile
		})
	}
	t.Logf("%s:STATUS:Cleanup of test data.", logTime)
	//Cleanup of test data.
	if err := setupService.TestdataMakeCleanup(t, dirPath, timeOutVar, "./cleanup.sh"); err != nil {
		t.Logf("%s:STATUS:Cleanup of testdata certificates failed!: %v", logTime, err)
	}
	t.Logf("%s:STATUS:Test completed!", logTime)
}
