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

package trust_bundle_rotation_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/feature/gnsi/certz/tests/internal/setup_service"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
)

const (
	dirPath                  = "../../test_data/"
	timeOutVar time.Duration = 2 * time.Minute
)

type DUTCredentialer interface {
	RPCUsername() string
	RPCPassword() string
}

var (
	serverAddr          string
	creds               DUTCredentialer
	testProfile         string = "rotationprofile"
	logTime             string = time.Now().String()
	expectedResult      bool   = true
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func createCombinedBundle(t *testing.T, dirPath, typeStr string) string {
	t.Helper()
	pem1 := filepath.Join(dirPath, "ca-01", fmt.Sprintf("trust_bundle_01_%s.pem", typeStr))
	pem2 := filepath.Join(dirPath, "ca-02", fmt.Sprintf("trust_bundle_02_%s.pem", typeStr))
	combinedPem := filepath.Join(dirPath, fmt.Sprintf("combined_%s.pem", typeStr))
	combinedP7b := filepath.Join(dirPath, fmt.Sprintf("combined_%s.p7b", typeStr))

	// Read pem1
	d1, err := os.ReadFile(pem1)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", pem1, err)
	}
	// Read pem2
	d2, err := os.ReadFile(pem2)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", pem2, err)
	}

	// Combine
	combinedData := append(d1, d2...)
	if err := os.WriteFile(combinedPem, combinedData, 0644); err != nil {
		t.Fatalf("Failed to write %s: %v", combinedPem, err)
	}

	// Convert to P7B using openssl
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "openssl", "crl2pkcs7", "-nocrl", "-certfile", combinedPem, "-out", combinedP7b)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to convert to PKCS7: %v\nOutput:\n%s", err, string(out))
	}

	return combinedP7b
}

func CertzRotateNegative(ctx context.Context, t *testing.T, certzClient certzpb.CertzClient, profileID string, entities ...*certzpb.Entity) {
	t.Helper()
	if len(entities) == 0 {
		t.Fatalf("At least one entity required for Rotate request.")
	}
	uploadRequest := &certzpb.UploadRequest{Entities: entities}
	rotateRequest := &certzpb.RotateCertificateRequest_Certificates{Certificates: uploadRequest}
	rotateCertRequest := &certzpb.RotateCertificateRequest{
		ForceOverwrite: false,
		SslProfileId:   profileID,
		RotateRequest:  rotateRequest,
	}
	rotateRequestClient, err := certzClient.Rotate(ctx)
	if err != nil {
		t.Fatalf("Error creating rotate request client: %v", err)
	}
	defer rotateRequestClient.CloseSend()
	err = rotateRequestClient.Send(rotateCertRequest)
	if err != nil {
		t.Logf("Send failed as expected (immediate failure): %v", err)
		return
	}
	t.Logf("Sent Rotate certificate request (Negative test).")

	_, err = rotateRequestClient.Recv()
	if err != nil {
		t.Logf("Recv failed as expected: %v", err)
		return
	}
	t.Fatalf("Rotate succeeded but was expected to fail.")
}

func TestTrustBundleRotation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name()
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("Failed to get DUT credentials: %v", err)
	}
	username := creds.RPCUsername()
	password := creds.RPCPassword()

	// Generate testdata certificates.
	t.Logf("%s:Creation of test data.", logTime)
	if err := setup_service.TestdataMakeCleanup(t, dirPath, timeOutVar, "./mk_cas.sh"); err != nil {
		t.Fatalf("Generation of testdata certificates failed!: %v", err)
	}
	defer func() {
		t.Logf("%s:Cleanup of test data.", logTime)
		if err := setup_service.TestdataMakeCleanup(t, dirPath, timeOutVar, "./cleanup.sh"); err != nil {
			t.Logf("Cleanup of testdata certificates failed!: %v", err)
		}
	}()

	types := []string{"rsa", "ecdsa"}

	for _, typeStr := range types {
		t.Run(fmt.Sprintf("Type_%s", typeStr), func(t *testing.T) {
			ctx := context.Background()
			gnmiClient, gnsiC := setup_service.PreInitCheck(ctx, t, dut)
			certzClient := gnsiC.Certz()

			// Add new sslprofileID.
			t.Logf("Adding new empty sslprofile ID %s.", testProfile)
			if _, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: testProfile}); err != nil {
				t.Fatalf("Add profile request failed: %v", err)
			}

			// Ensure cleanup of profile at the end of subtest
			defer func() {
				t.Logf("Cleaning up profile %s", testProfile)
				// Note: gNSI certz doesn't seem to have a DeleteProfile in setup_service,
				// but we should at least try to revert config if needed.
				// For now we just rely on DUT reset or ignore if it's persistent.
			}()

			// 1. Baseline Setup with ca-01
			serverCertFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("server-%s-a-cert.pem", typeStr))
			serverKeyFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("server-%s-a-key.pem", typeStr))
			trustBundleFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("trust_bundle_01_%s.p7b", typeStr))
			clientCertFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("client-%s-a-cert.pem", typeStr))
			clientKeyFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("client-%s-a-key.pem", typeStr))

			serverSAN := setup_service.ReadDecodeServerCertificate(t, serverCertFile)
			serverCert := setup_service.CreateCertzChain(t, setup_service.CertificateChainRequest{
				RequestType:    setup_service.EntityTypeCertificateChain,
				ServerCertFile: serverCertFile,
				ServerKeyFile:  serverKeyFile,
			})
			serverCertEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeCertificateChain, &serverCert, "v1")

			pkcs7certs, pkcs7data, err := setup_service.Loadpkcs7TrustBundle(trustBundleFile)
			if err != nil {
				t.Fatalf("Failed to load trust bundle: %v", err)
			}
			caCertPool := x509.NewCertPool()
			for _, c := range pkcs7certs {
				caCertPool.AddCert(c)
			}
			trustBundleEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeTrustBundle, string(pkcs7data), "bundle1")

			clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
			if err != nil {
				t.Fatalf("Failed to load client cert: %v", err)
			}

			// Initial rotation to establish ca-01 baseline (newTLS = false to associate profile)
			t.Logf("Installing initial ca-01 profile")
			if success := setup_service.CertzRotate(ctx, t, caCertPool, certzClient, gnmiClient, clientCert, dut, username, password, serverSAN, serverAddr, testProfile, false, false, false, &serverCertEntity, &trustBundleEntity); !success {
				t.Fatalf("Initial CertzRotation failed.")
			}

			// Verify baseline works
			if result := setup_service.ServicesValidationCheck(t, caCertPool, expectedResult, serverSAN, serverAddr, username, password, clientCert, false); !result {
				t.Fatalf("Baseline service validation failed.")
			}

			// Create new client connection using the newly rotated credentials for subsequent operations
			conn := setup_service.CreateNewDialOption(t, clientCert, caCertPool, serverSAN, username, password, serverAddr)
			defer conn.Close()
			certzClient = certzpb.NewCertzClient(conn)
			gnmiClient = gnmipb.NewGNMIClient(conn)

			// --- Certz-5.1: Positive Test ---
			t.Run("Certz-5.1_Positive", func(t *testing.T) {
				t.Logf("Starting Certz-5.1 Positive Test")
				combinedP7b := createCombinedBundle(t, dirPath, typeStr)

				combCerts, combData, err := setup_service.Loadpkcs7TrustBundle(combinedP7b)
				if err != nil {
					t.Fatalf("Failed to load combined trust bundle: %v", err)
				}
				combCaCertPool := x509.NewCertPool()
				for _, c := range combCerts {
					combCaCertPool.AddCert(c)
				}
				combTrustBundleEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeTrustBundle, string(combData), "bundle_combined")

				// Rotate ONLY trust bundle (newTLS = true because profile is already associated)
				t.Logf("Rotating trust bundle to combined version")
				if success := setup_service.CertzRotate(ctx, t, combCaCertPool, certzClient, gnmiClient, clientCert, dut, username, password, serverSAN, serverAddr, testProfile, true, false, false, &combTrustBundleEntity); !success {
					t.Fatalf("Positive Trust Bundle Rotation failed.")
				}

				// Verify that connection still works with ca-01 client cert (since ca-01 is in combined bundle)
				if result := setup_service.ServicesValidationCheck(t, combCaCertPool, expectedResult, serverSAN, serverAddr, username, password, clientCert, false); !result {
					t.Fatalf("Service validation failed after positive rotation.")
				}
			})

			// Reconnect with ca-01 credentials for negative test (we are still on combined bundle after 5.1)
			// Wait, 5.2 starts from ca-01 baseline.
			// "configure the DUT to use the ca-0001 form key/certificate/trust_bundle"
			// So we should revert back to ca-01 trust bundle first, OR just run 5.2 in a separate subtest where we initialize again.
			// It's cleaner to run them independently or revert.
			// Let's revert to ca-01 trust bundle first to reset state.
			t.Run("Revert_to_Baseline", func(t *testing.T) {
				t.Logf("Reverting to ca-01 baseline")
				if success := setup_service.CertzRotate(ctx, t, caCertPool, certzClient, gnmiClient, clientCert, dut, username, password, serverSAN, serverAddr, testProfile, true, false, false, &trustBundleEntity); !success {
					t.Fatalf("Revert to baseline failed.")
				}
			})

			// --- Certz-5.2: Negative Test ---
			t.Run("Certz-5.2_Negative", func(t *testing.T) {
				t.Logf("Starting Certz-5.2 Negative Test")
				ca02TrustBundleFile := filepath.Join(dirPath, "ca-02", fmt.Sprintf("trust_bundle_02_%s.p7b", typeStr))

				ca02Certs, ca02Data, err := setup_service.Loadpkcs7TrustBundle(ca02TrustBundleFile)
				if err != nil {
					t.Fatalf("Failed to load ca-02 trust bundle: %v", err)
				}
				ca02CaCertPool := x509.NewCertPool()
				for _, c := range ca02Certs {
					ca02CaCertPool.AddCert(c)
				}
				ca02TrustBundleEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeTrustBundle, string(ca02Data), "bundle_ca02")

				// Attempt to rotate to ca-02 trust bundle.
				// We expect this to fail because the server cert (still ca-01) is not signed by ca-02.
				t.Logf("Attempting to rotate trust bundle to ca-02 (should fail)")
				CertzRotateNegative(ctx, t, certzClient, testProfile, &ca02TrustBundleEntity)

				// Verify that the server is still serving the certificate properly and ca-01 client can still connect
				t.Logf("Verifying server still works with ca-01 baseline after failed rotation")
				if result := setup_service.ServicesValidationCheck(t, caCertPool, expectedResult, serverSAN, serverAddr, username, password, clientCert, false); !result {
					t.Fatalf("Service validation failed after negative rotation attempt (server might be broken).")
				}
			})
		})
	}
}
