// Copyright 2023 Google LLC
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
	context "context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	setupService "github.com/openconfig/featureprofiles/feature/security/gnsi/certz/tests/internal/setup_service"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

var (
	testProfile = "gNxI"
	serverAddr  string
	username    = "certzuser"
	password    = "certzpswd"
)

// createUser function to add an user in admin role.
func createUser(t *testing.T, dut *ondatra.DUTDevice, user, pswd string) bool {
	ocUser := &oc.System_Aaa_Authentication_User{
		Username: ygot.String(user),
		Role:     oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN,
		Password: ygot.String(pswd),
	}
	gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().User(user).Config(), ocUser)
	return true
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestClientCertTcOne Test that client certificates from a set of one CA are able to be validated and
// used for authentication to a device when used by a client connecting to each
// gRPC service.

func TestClientCertTcOne(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name()

	checkUser := createUser(t, dut, username, password)
	if !checkUser {
		t.Fatalf("Failed to create certz user.")
	}

	t.Logf("Validation of all services that are using gRPC before certz rotation.")
	initCheck := setupService.PreInitCheck(context.Background(), t, dut)
	if !initCheck {
		t.Fatalf("Failed in the preInit checks.")
	}

	ctx := context.Background()
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(ctx)
	if err != nil {
		t.Fatalf("Could not create gNSI Connection %v", err)
	}
	t.Logf("Precheck:gNSI connection is successful %v", gnsiC)

	cases := []struct {
		desc             string
		serverCertzFile  string
		serverKeyzFile   string
		trustBundlezFile string
		clientCertzFile  string
		clientKeyzFile   string
	}{
		{
			desc:             "Load the correct key-type rsa trustbundle with 1 CA configuration",
			serverCertzFile:  "test_data/ca-01/server-rsa-a-cert.pem",
			serverKeyzFile:   "test_data/ca-01/server-rsa-a-key.pem",
			trustBundlezFile: "test_data/ca-01/trust_bundle_01_rsa_a.pem",
			clientCertzFile:  "test_data/ca-01/client-rsa-a-cert.pem",
			clientKeyzFile:   "test_data/ca-01/client-rsa-a-key.pem",
		},
		{
			desc:             "Load the correct key-type ecdsa trustbundle with 1 CA configuration",
			serverCertzFile:  "test_data/ca-01/server-ecdsa-a-cert.pem",
			serverKeyzFile:   "test_data/ca-01/server-ecdsa-a-key.pem",
			trustBundlezFile: "test_data/ca-01/trust_bundle_01_ecdsa_a.pem",
			clientCertzFile:  "test_data/ca-01/client-ecdsa-a-cert.pem",
			clientKeyzFile:   "test_data/ca-01/client-ecdsa-a-key.pem",
		},
		{
			desc:             "Load the correct key-type rsa trustbundle with 2 CA configuration",
			serverCertzFile:  "test_data/ca-02/server-rsa-a-cert.pem",
			serverKeyzFile:   "test_data/ca-02/server-rsa-a-key.pem",
			trustBundlezFile: "test_data/ca-02/trust_bundle_02_rsa_a.pem",
			clientCertzFile:  "test_data/ca-02/client-rsa-a-cert.pem",
			clientKeyzFile:   "test_data/ca-02/client-rsa-a-key.pem",
		},
		{
			desc:             "Load the correct key-type ecdsa trustbundle with 2 CA configuration",
			serverCertzFile:  "test_data/ca-02/server-ecdsa-a-cert.pem",
			serverKeyzFile:   "test_data/ca-02/server-ecdsa-a-key.pem",
			trustBundlezFile: "test_data/ca-02/trust_bundle_02_ecdsa_a.pem",
			clientCertzFile:  "test_data/ca-02/client-ecdsa-a-cert.pem",
			clientKeyzFile:   "test_data/ca-02/client-ecdsa-a-key.pem",
		},
		{
			desc:             "Load the correct key-type rsa trustbundle with 10CA configuration",
			serverCertzFile:  "test_data/ca-10/server-rsa-a-cert.pem",
			serverKeyzFile:   "test_data/ca-10/server-rsa-a-key.pem",
			trustBundlezFile: "test_data/ca-10/trust_bundle_10_rsa_a.pem",
			clientCertzFile:  "test_data/ca-10/client-rsa-a-cert.pem",
			clientKeyzFile:   "test_data/ca-10/client-rsa-a-key.pem",
		},
		{
			desc:             "Load the correct key-type ecdsa trustbundle with 10CA configuration",
			serverCertzFile:  "test_data/ca-10/server-ecdsa-a-cert.pem",
			serverKeyzFile:   "test_data/ca-10/server-ecdsa-a-key.pem",
			trustBundlezFile: "test_data/ca-10/trust_bundle_10_ecdsa_a.pem",
			clientCertzFile:  "test_data/ca-10/client-ecdsa-a-cert.pem",
			clientKeyzFile:   "test_data/ca-10/client-ecdsa-a-key.pem",
		},
		{
			desc:             "Load the correct key-type rsa trustbundle with 1000CA configuration",
			serverCertzFile:  "test_data/ca-1000/server-rsa-a-cert.pem",
			serverKeyzFile:   "test_data/ca-1000/server-rsa-a-key.pem",
			trustBundlezFile: "test_data/ca-1000/trust_bundle_1000_rsa_a.pem",
			clientCertzFile:  "test_data/ca-1000/client-rsa-a-cert.pem",
			clientKeyzFile:   "test_data/ca-1000/client-rsa-a-key.pem",
		},
		{
			desc:             "Load the correct key-type ecdsa trustbundle with 1000CA configuration",
			serverCertzFile:  "test_data/ca-1000/server-ecdsa-a-cert.pem",
			serverKeyzFile:   "test_data/ca-1000/server-ecdsa-a-key.pem",
			trustBundlezFile: "test_data/ca-1000/trust_bundle_1000_ecdsa_a.pem",
			clientCertzFile:  "test_data/ca-1000/client-ecdsa-a-cert.pem",
			clientKeyzFile:   "test_data/ca-1000/client-ecdsa-a-key.pem",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {

			sc1, err := os.ReadFile(tc.serverCertzFile)
			if err != nil {
				t.Fatalf("Failed to read certificate: %v", err)
			}
			block, _ := pem.Decode(sc1)
			if block == nil {
				t.Fatalf("Failed to parse PEM block containing the public key.")
			}
			sCert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				t.Fatalf("Failed to parse certificate: %v", err)
			}

			san := sCert.DNSNames[0]

			serverCert := setupService.CreateCertzChain(t, setupService.CertificateChainRequest{
				RequestType:    setupService.EntityTypeCertificateChain,
				ServerCertFile: tc.serverCertzFile,
				ServerKeyFile:  tc.serverKeyzFile})

			serverCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &serverCert, "one")

			trustCertChain := setupService.CreateCertChainFromTrustBundle(tc.trustBundlezFile)
			trustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, trustCertChain, "two")

			cert, err := tls.LoadX509KeyPair(tc.clientCertzFile, tc.clientKeyzFile)
			if err != nil {
				t.Fatalf("Failed to load  client cert: %v", err)
			}

			cacert := x509.NewCertPool()
			cacertBytes, err := os.ReadFile(tc.trustBundlezFile)
			if err != nil {
				t.Fatalf("Failed to read ca bundle :%v", err)
			}
			if ok := cacert.AppendCertsFromPEM(cacertBytes); !ok {
				t.Fatalf("Failed to parse %v", tc.trustBundlezFile)
			}

			certzClient := gnsiC.Certz()
			success := setupService.CertzRotate(t, certzClient, testProfile, &serverCertEntity, &trustBundleEntity)
			if !success {
				t.Fatalf("%s:Certz/Rotate failed.", tc.desc)
			}
			t.Logf("%s:successfully completed second certz/Rotate!", tc.desc)

			// Verification check of the new connection with the new rotated certificates.
			t.Run("Verification of fresh new connection after successful rotate ", func(t *testing.T) {
				result := setupService.PostValidationCheck(t, cacert, san, serverAddr, username, password, cert)
				if !result {
					t.Fatalf("%s postTestcase validation failed.", tc.desc)
				}

				t.Logf("PASS: %s successfully completed!", tc.desc)
			})
		})
	}
}
