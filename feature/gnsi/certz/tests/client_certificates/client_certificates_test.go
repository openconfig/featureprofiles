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
	creds               DUTCredentialer                //an interface for getting credentials from a DUT binding
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

// TestClientCert Test validates that client certificates from a set of one CA are able to be loaded successfully
// and  used for authentication to a device when used by a client connecting to each
// gRPC service.
func TestClientCert(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name() //returns the device name.
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("%sSTATUS:Failed to get DUT credentials using binding.DUTAs: %v. The binding for %s must implement the DUTCredentialer interface.", logTime, err, dut.Name())
	}
	username := creds.RPCUsername()
	password := creds.RPCPassword()
	t.Logf("Validation of all services that are using gRPC before certz rotation.")
	gnmiClient, gnsiC := setupService.PreInitCheck(context.Background(), t, dut)
	//Generate testdata certificates
	t.Logf("%sSTATUS:Generation of test data certificates.", logTime)
	if err := setupService.TestdataMakeCleanup(t, dirPath, timeOutVar, "./mk_cas.sh"); err != nil {
		t.Fatalf("%sSTATUS:Generation of testdata certificates failed!: %v", logTime, err)
	}
	//Create a certz client
	ctx := context.Background()
	certzClient := gnsiC.Certz()
	t.Logf("%sSTATUS:Precheck:checking baseline ssl profile list.", logTime)
	//Get ssl profile list.
	if getResp := setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Fatalf("%sSTATUS:profileID %s already exists.", logTime, testProfile)
	}
	//Add a new ssl profileID
	t.Logf("%sSTATUS:Adding new empty ssl profile ID %s.", logTime, testProfile)
	if addProfileResponse, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: testProfile}); err != nil {
		t.Fatalf("%sSTATUS:Add profile request failed with %v!", logTime, err)
	} else {
		t.Logf("%sSTATUS:Received the AddProfileResponse %v.", logTime, addProfileResponse)
	}
	//Get ssl profile list after new ssl profile addition.
	if getResp := setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); !slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Fatalf("%sSTATUS:newly added profileID is not seen.", logTime)
	} else {
		t.Logf("%sSTATUS: new profileID %s is seen in ssl profile list.", logTime, testProfile)
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
			desc:            "Certz1.1:Load the key-type rsa trustbundle with 1 CA configuration",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:        "certz1",
			bversion:        "bundle1",
		},
		{
			desc:            "Certz1.1:Load the key-type ecdsa trustbundle with 1 CA configuration",
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
			desc:            "Certz1.1:Load the key-type rsa trustbundle with 2 CA configuration",
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
			desc:            "Certz1.1:Load the key-type ecdsa trustbundle with 2 CA configuration",
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
			desc:            "Certz1.1:Load the key-type rsa trustbundle with 10CA configuration",
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
			desc:            "Certz1.1:Load the key-type ecdsa trustbundle with 10CA configuration",
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
			desc:            "Certz1.1:Load the key-type rsa trustbundle with 1000CA configuration",
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
			desc:            "Certz1.1:Load the key-type ecdsa trustbundle with 1000CA configuration",
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
			desc:            "Certz1.2:Load the rsa trust_bundle from ca-02 with mismatching key type rsa client certificate from ca-01",
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
			desc:            "Certz1.2:Load the ecdsa trust_bundle from ca-02 with mismatching key type ecdsa client certificate from ca-01",
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
			t.Logf("%s:STATUS:Starting test case: %s", logTime, tc.desc)
			//Read the serverSAN (Subject Alternative Name) from the certificate used for TLS verification.
			serverSAN := setupService.ReadDecodeServerCertificate(t, tc.serverCertFile)
			//Build serverCertEntity for the server certificate rotation.
			serverCert := setupService.CreateCertzChain(t, setupService.CertificateChainRequest{
				RequestType:    setupService.EntityTypeCertificateChain,
				ServerCertFile: tc.serverCertFile,
				ServerKeyFile:  tc.serverKeyFile})
			serverCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &serverCert, tc.cversion)
			//Create a new Cert Pool and add the certs from the trustbundle.
			pkcs7certs, pkcs7data, err := setupService.Loadpkcs7TrustBundle(tc.trustBundleFile)
			if err != nil {
				t.Fatalf("%sSTATUS:failed to load trust bundle: %v", logTime, err)
			}
			newCaCert := x509.NewCertPool()
			for _, c := range pkcs7certs {
				newCaCert.AddCert(c)
			}
			//Build trustBundleEntity for the server certificate rotation.
			trustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, string(pkcs7data), tc.bversion)
			//Load Client certificate
			newClientCert, err := tls.LoadX509KeyPair(tc.clientCertFile, tc.clientKeyFile)
			if err != nil {
				t.Fatalf("%sSTATUS:Failed to load client cert: %v", logTime, err)
			}
			if tc.newTLScreds {
				t.Logf("%s:STATUS:%s: Creating new TLS credentials for client connection.", logTime, tc.desc)
				//Load the prior client keypair for new client TLS credentials.
				prevClientCert, err := tls.LoadX509KeyPair(prevClientCertFile, prevClientKeyFile)
				if err != nil {
					t.Fatalf("%s:STATUS:%s:Failed to load previous client cert: %v", logTime, tc.desc, err)
				}
				oldPkcs7certs, oldPkcs7data, err := setupService.Loadpkcs7TrustBundle(prevTrustBundleFile)
				if err != nil {
					t.Fatalf("%s:STATUS:%s:Failed to load previous trust bundle,data %v with %v", logTime, tc.desc, oldPkcs7data, err)
				}
				//Create a old set of Cert Pool and append the certs from previous trust bundle.
				prevCaCert := x509.NewCertPool()
				for _, c := range oldPkcs7certs {
					prevCaCert.AddCert(c)
				}
				//Before rotation, validation of all services with existing certificates.
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
			//Initiate certz rotation.
			t.Logf("STATUS:%s Initiating Certz rotation with server cert: %s and trust bundle: %s", tc.desc, tc.serverCertFile, tc.trustBundleFile)
			if success := setupService.CertzRotate(ctx, t, newCaCert, certzClient, gnmiClient, newClientCert, dut, username, password, serverSAN, serverAddr, testProfile, tc.newTLScreds, tc.mismatch, tc.scale, &serverCertEntity, &trustBundleEntity); !success {
				t.Fatalf("%sSTATUS: %s:Certz rotation failed.", logTime, tc.desc)
			}
			t.Logf("%s:STATUS:%s: Certz rotation completed!", logTime, tc.desc)
			//Post rotate validation of all services.
			t.Run("Verification of new connection after rotate ", func(t *testing.T) {
				if result := setupService.ServicesValidationCheck(t, newCaCert, expectedResult, serverSAN, serverAddr, username, password, newClientCert, tc.mismatch); !result {
					t.Fatalf("%s:STATUS:%s:service validation failed after rotate- got %v, want %v.", logTime, tc.desc, result, expectedResult)
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
		t.Errorf("%s:STATUS:Cleanup of testdata certificates failed!: %v", logTime, err)
	}
	t.Logf("%s:STATUS:Test completed!", logTime)
}
