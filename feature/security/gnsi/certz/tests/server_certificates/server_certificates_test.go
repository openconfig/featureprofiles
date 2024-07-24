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

package server_certificates_test

import (
	context "context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"testing"
	"time"

	setupService "github.com/openconfig/featureprofiles/feature/security/gnsi/certz/tests/internal/setup_service"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"

	certzpb "github.com/openconfig/gnsi/certz"
)

const (
	dirPath = "../../test_data/"
)

var (
	testProfile = "newprofile"
	serverAddr  string
	username    = "certzuser"
	password    = "certzpasswd"
	servers     []string
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
		t.Fatalf("Failed to create credentials.")
	}
	return true
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestServerCertTcTwo tests the server certificates from a set of one CA are able to be validated and
// used for authentication to a device when used by a client connecting to each
// gRPC service.
func TestServerCert(t *testing.T) {

	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name()
	if !createUser(t, dut, username, password) {
		t.Fatalf("Failed to create certz user.")
	}

	t.Logf("Validation of all services that are using gRPC before certz rotation.")
	if !setupService.PreInitCheck(context.Background(), t, dut) {
		t.Fatalf("Failed in the preInit checks.")
	}

	ctx := context.Background()
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(ctx)
	if err != nil {
		t.Fatalf("Failed to create gNSI Connection %v", err)
	}
	t.Logf("Precheck:gNSI connection is successful %v", gnsiC)

	t.Logf("Creation of test data.")
	if setupService.CertGeneration(dirPath) != nil {
		t.Fatalf("Failed to generate the testdata certificates.")
	}

	certzClient := gnsiC.Certz()
	t.Logf("Precheck:baseline ssl profile list")
	setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{})
	t.Logf("Adding new empty ssl profile ID.")
	addProfileResponse, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: testProfile})
	if err != nil {
		t.Fatalf("Add profile request failed with %v!", err)
	}
	t.Logf("AddProfileResponse: %v", addProfileResponse)
	t.Logf("Getting the ssl profile list after new ssl profile addition.")
	setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{})

	cases := []struct {
		desc            string
		serverCertFile  string
		serverKeyFile   string
		trustBundleFile string
		clientCertFile  string
		clientKeyFile   string
	}{
		{
			desc:            "Certz2.1:Load server certificate of rsa keytype with 1 CA configuration",
			serverCertFile:  dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_rsa.pem",
			clientCertFile:  dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-rsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of ecdsa keytype with 1 CA configuration",
			serverCertFile:  dirPath + "ca-01/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-01/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-01/trust_bundle_01_ecdsa.pem",
			clientCertFile:  dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-01/client-ecdsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of rsa keytype with 2 CA configuration",
			serverCertFile:  dirPath + "ca-02/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_rsa.pem",
			clientCertFile:  dirPath + "ca-02/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-02/client-rsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of ecdsa keytype with 2 CA configuration",
			serverCertFile:  dirPath + "ca-02/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-02/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-02/trust_bundle_02_ecdsa.pem",
			clientCertFile:  dirPath + "ca-02/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-02/client-ecdsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of rsa keytype with 10CA configuration",
			serverCertFile:  dirPath + "ca-10/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-10/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-10/trust_bundle_10_rsa.pem",
			clientCertFile:  dirPath + "ca-10/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-10/client-rsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of ecdsa keytype with 10CA configuration",
			serverCertFile:  dirPath + "ca-10/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-10/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-10/trust_bundle_10_ecdsa.pem",
			clientCertFile:  dirPath + "ca-10/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-10/client-ecdsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of rsa keytype with 1000CA configuration",
			serverCertFile:  dirPath + "ca-1000/server-rsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-1000/server-rsa-a-key.pem",
			trustBundleFile: dirPath + "ca-1000/trust_bundle_1000_rsa.pem",
			clientCertFile:  dirPath + "ca-1000/client-rsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-1000/client-rsa-a-key.pem",
		},
		{
			desc:            "Certz2.1:Load server certificate of ecdsa keytype with 1000CA configuration",
			serverCertFile:  dirPath + "ca-1000/server-ecdsa-a-cert.pem",
			serverKeyFile:   dirPath + "ca-1000/server-ecdsa-a-key.pem",
			trustBundleFile: dirPath + "ca-1000/trust_bundle_1000_ecdsa.pem",
			clientCertFile:  dirPath + "ca-1000/client-ecdsa-a-cert.pem",
			clientKeyFile:   dirPath + "ca-1000/client-ecdsa-a-key.pem",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {

			san := setupService.ReadDecodeServerCertificate(t, tc.serverCertFile)
			serverCert := setupService.CreateCertzChain(t, setupService.CertificateChainRequest{
				RequestType:    setupService.EntityTypeCertificateChain,
				ServerCertFile: tc.serverCertFile,
				ServerKeyFile:  tc.serverKeyFile})
			serverCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &serverCert, "servercert")

			trustCertChain := setupService.CreateCertChainFromTrustBundle(tc.trustBundleFile)
			trustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, trustCertChain, "cabundle")

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

			certzClient := gnsiC.Certz()
			success := setupService.CertzRotate(t, cacert, certzClient, cert, san, serverAddr, testProfile, &serverCertEntity, &trustBundleEntity)
			if !success {
				t.Fatalf("%s:Certz rotation failed.", tc.desc)
			}
			t.Logf("%s:Certz rotation completed!", tc.desc)

			// Replace config with newly added ssl profile after successful rotate.
			servers = gnmi.GetAll(t, dut, gnmi.OC().System().GrpcServerAny().Name().State())
			batch := gnmi.SetBatch{}
			for _, server := range servers {
				gnmi.BatchReplace(&batch, gnmi.OC().System().GrpcServer(server).CertificateId().Config(), testProfile)
			}
			batch.Set(t, dut)
			t.Logf("%s:replaced gNMI config with new ssl profile successfully.", tc.desc)

			// Verification check of the new connection with the newly rotated certificates.
			t.Run("Verification of new connection after successful server certificate rotation", func(t *testing.T) {
				if !setupService.PostValidationCheck(t, cacert, ctx, san, serverAddr, username, password, cert) {
					t.Fatalf("%s postTestcase service validation failed after successful rotate.", tc.desc)
				}
				t.Logf("%s postTestcase service validation done after server certificate rotation!", tc.desc)
				t.Logf("PASS: %s successfully completed!", tc.desc)
			})
		})
	}
	t.Logf("Cleanup of test data.")
	if setupService.CertCleanup(dirPath) != nil {
		t.Fatalf("could not run cert cleanup command.")
	}
}
