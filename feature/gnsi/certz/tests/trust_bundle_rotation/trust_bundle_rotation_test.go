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
	serverAddr     string
	creds          DUTCredentialer
	testProfile    string = "rotationprofile"
	logTime        string = time.Now().String()
	expectedResult bool   = true
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func createCombinedBundle(t *testing.T, dirPath, typeStr string) string {
	t.Helper()
	pem1 := filepath.Join(dirPath, "ca-01", fmt.Sprintf("trust_bundle_01_%s.pem", typeStr))
	pem2 := filepath.Join(dirPath, "ca-02", fmt.Sprintf("trust_bundle_02_%s.pem", typeStr))
	tempDir := t.TempDir()
	combinedPem := filepath.Join(tempDir, fmt.Sprintf("combined_%s.pem", typeStr))
	combinedP7b := filepath.Join(tempDir, fmt.Sprintf("combined_%s.p7b", typeStr))

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
			ctx, cancel := context.WithTimeout(context.Background(), timeOutVar)
			defer cancel()
			gnmiClient, gnsiC := setup_service.PreInitCheck(ctx, t, dut)
			certzClient := gnsiC.Certz()

			// Define baseline files (ca-01)
			serverCertFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("server-%s-a-cert.pem", typeStr))
			serverKeyFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("server-%s-a-key.pem", typeStr))
			trustBundleFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("trust_bundle_01_%s.p7b", typeStr))
			clientCertFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("client-%s-a-cert.pem", typeStr))
			clientKeyFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("client-%s-a-key.pem", typeStr))

			serverSAN := setup_service.ReadDecodeServerCertificate(t, serverCertFile)

			// Load client cert
			clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
			if err != nil {
				t.Fatalf("Failed to load client cert: %v", err)
			}

			// Load trust bundle
			pkcs7certs, pkcs7data, err := setup_service.Loadpkcs7TrustBundle(trustBundleFile)
			if err != nil {
				t.Fatalf("Failed to load trust bundle: %v", err)
			}
			caCertPool := x509.NewCertPool()
			for _, c := range pkcs7certs {
				caCertPool.AddCert(c)
			}

			cases := []struct {
				desc          string
				isNegative    bool
				version       string
				getBundlePath func(t *testing.T) string
			}{
				{
					desc:       "Certz-5.1_Positive",
					isNegative: false,
					version:    "bundle_combined",
					getBundlePath: func(t *testing.T) string {
						return createCombinedBundle(t, dirPath, typeStr)
					},
				},
				{
					desc:       "Certz-5.2_Negative",
					isNegative: true,
					version:    "bundle_ca02",
					getBundlePath: func(t *testing.T) string {
						return filepath.Join(dirPath, "ca-02", fmt.Sprintf("trust_bundle_02_%s.p7b", typeStr))
					},
				},
			}

			for _, tc := range cases {
				t.Run(tc.desc, func(t *testing.T) {
					profileID := fmt.Sprintf("%s_%s_%s", testProfile, typeStr, tc.desc)

					// Add new sslprofileID.
					t.Logf("Adding new empty sslprofile ID %s.", profileID)
					if _, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: profileID}); err != nil {
						t.Fatalf("Add profile request failed: %v", err)
					}

					// 1. Setup Baseline (ca-01)
					serverCert := setup_service.CreateCertzChain(t, setup_service.CertificateChainRequest{
						RequestType:    setup_service.EntityTypeCertificateChain,
						ServerCertFile: serverCertFile,
						ServerKeyFile:  serverKeyFile,
					})
					serverCertEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeCertificateChain, &serverCert, "v1")
					trustBundleEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeTrustBundle, string(pkcs7data), "bundle1")

					t.Logf("Installing baseline profile %s with ca-01", profileID)
					if success := setup_service.CertzRotate(ctx, t, caCertPool, certzClient, gnmiClient, clientCert, dut, username, password, serverSAN, serverAddr, profileID, false, false, false, &serverCertEntity, &trustBundleEntity); !success {
						t.Fatalf("Baseline CertzRotation failed.")
					}

					// Verify baseline works
					if result := setup_service.ServicesValidationCheck(t, caCertPool, expectedResult, serverSAN, serverAddr, username, password, clientCert, false); !result {
						t.Fatalf("Baseline service validation failed.")
					}

					// Create new client connection using the baseline credentials
					conn := setup_service.CreateNewDialOption(t, clientCert, caCertPool, serverSAN, username, password, serverAddr)
					defer conn.Close()
					activeCertzClient := certzpb.NewCertzClient(conn)
					activeGnmiClient := gnmipb.NewGNMIClient(conn)

					// Prepare target trust bundle
					targetBundlePath := tc.getBundlePath(t)
					targetCerts, targetData, err := setup_service.Loadpkcs7TrustBundle(targetBundlePath)
					if err != nil {
						t.Fatalf("Failed to load target trust bundle: %v", err)
					}
					targetCaCertPool := x509.NewCertPool()
					for _, c := range targetCerts {
						targetCaCertPool.AddCert(c)
					}
					targetTrustBundleEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeTrustBundle, string(targetData), tc.version)

					if !tc.isNegative {
						// Positive Rotation
						t.Logf("Rotating trust bundle to target: %s", targetBundlePath)
						if success := setup_service.CertzRotate(ctx, t, targetCaCertPool, activeCertzClient, activeGnmiClient, clientCert, dut, username, password, serverSAN, serverAddr, profileID, true, false, false, &targetTrustBundleEntity); !success {
							t.Fatalf("Trust Bundle Rotation failed.")
						}

						// Verify connection still works (with target trust pool)
						if result := setup_service.ServicesValidationCheck(t, targetCaCertPool, expectedResult, serverSAN, serverAddr, username, password, clientCert, false); !result {
							t.Fatalf("Service validation failed after rotation.")
						}
					} else {
						// Negative Rotation
						t.Logf("Attempting negative trust bundle rotation to target: %s", targetBundlePath)
						CertzRotateNegative(ctx, t, activeCertzClient, profileID, &targetTrustBundleEntity)

						// Verify rollback (should still work with baseline ca-01)
						t.Logf("Verifying server still works with baseline ca-01 after failed rotation")
						if result := setup_service.ServicesValidationCheck(t, caCertPool, expectedResult, serverSAN, serverAddr, username, password, clientCert, false); !result {
							t.Fatalf("Service validation failed after negative rotation attempt.")
						}
					}
				})
			}
		})
	}
}
