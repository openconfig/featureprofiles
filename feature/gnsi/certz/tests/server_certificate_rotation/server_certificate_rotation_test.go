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
	"fmt"
	"path/filepath"
	"sync"
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

func monitorConnection(ctx context.Context, t *testing.T, gnmiClient gnmipb.GNMIClient, errChan chan error, wg *sync.WaitGroup) {
	t.Helper()
	defer wg.Done()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := gnmiClient.Capabilities(ctx, &gnmipb.CapabilityRequest{})
			if err != nil {
				errChan <- err
				return
			}
		}
	}
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
	// Tearing down the RPC (closing stream without Finalize) should rollback.
	defer rotateRequestClient.CloseSend()

	err = rotateRequestClient.Send(rotateCertRequest)
	if err != nil {
		t.Logf("Send failed as expected: %v", err)
		return
	}
	t.Logf("Sent Rotate certificate request (Negative test).")

	rotateResponse, err := rotateRequestClient.Recv()
	if err != nil {
		t.Logf("Recv failed as expected: %v", err)
		return
	}
	t.Logf("Rotate succeeded (unexpectedly or pending verification): %v", rotateResponse)
}

func TestServerCertificateRotation(t *testing.T) {
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

			// Define baseline files (ca-01, server-a)
			serverCertAFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("server-%s-a-cert.pem", typeStr))
			serverKeyAFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("server-%s-a-key.pem", typeStr))
			trustBundleFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("trust_bundle_01_%s.p7b", typeStr))
			clientCertFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("client-%s-a-cert.pem", typeStr))
			clientKeyFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("client-%s-a-key.pem", typeStr))

			serverSAN := setup_service.ReadDecodeServerCertificate(t, serverCertAFile)

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
				desc           string
				targetCertFile string
				targetKeyFile  string
				isNegative     bool
				monitorConn    bool
				version        string
			}{
				{
					desc:           "Certz-3.1_Positive",
					targetCertFile: filepath.Join(dirPath, "ca-01", fmt.Sprintf("server-%s-b-cert.pem", typeStr)),
					targetKeyFile:  filepath.Join(dirPath, "ca-01", fmt.Sprintf("server-%s-b-key.pem", typeStr)),
					isNegative:     false,
					monitorConn:    true,
					version:        "v2",
				},
				{
					desc:           "Certz-3.2_Negative",
					targetCertFile: filepath.Join(dirPath, "ca-02", fmt.Sprintf("server-%s-b-cert.pem", typeStr)),
					targetKeyFile:  filepath.Join(dirPath, "ca-02", fmt.Sprintf("server-%s-b-key.pem", typeStr)),
					isNegative:     true,
					monitorConn:    false,
					version:        "v2_ca02",
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

					// 1. Setup Baseline (server-a)
					serverCertA := setup_service.CreateCertzChain(t, setup_service.CertificateChainRequest{
						RequestType:    setup_service.EntityTypeCertificateChain,
						ServerCertFile: serverCertAFile,
						ServerKeyFile:  serverKeyAFile,
					})
					serverCertAEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeCertificateChain, &serverCertA, "v1")
					trustBundleEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeTrustBundle, string(pkcs7data), "bundle1")

					t.Logf("Installing baseline profile %s with server-a", profileID)
					if success := setup_service.CertzRotate(ctx, t, caCertPool, certzClient, gnmiClient, clientCert, dut, username, password, serverSAN, serverAddr, profileID, false, false, false, &serverCertAEntity, &trustBundleEntity); !success {
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

					// Prepare target cert entity
					targetCert := setup_service.CreateCertzChain(t, setup_service.CertificateChainRequest{
						RequestType:    setup_service.EntityTypeCertificateChain,
						ServerCertFile: tc.targetCertFile,
						ServerKeyFile:  tc.targetKeyFile,
					})
					targetCertEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeCertificateChain, &targetCert, tc.version)

					var monitorCtx context.Context
					var monitorCancel context.CancelFunc
					errChan := make(chan error, 1)
					var wg sync.WaitGroup

					if tc.monitorConn {
						monitorCtx, monitorCancel = context.WithCancel(ctx)
						wg.Add(1)
						go monitorConnection(monitorCtx, t, activeGnmiClient, errChan, &wg)
					}

					if !tc.isNegative {
						// Positive Rotation
						t.Logf("Rotating server certificate to target: %s", tc.targetCertFile)
						if success := setup_service.CertzRotate(ctx, t, caCertPool, activeCertzClient, activeGnmiClient, clientCert, dut, username, password, serverSAN, serverAddr, profileID, true, false, false, &targetCertEntity); !success {
							t.Fatalf("Server Certificate Rotation failed.")
						}

						if tc.monitorConn {
							monitorCancel()
							wg.Wait()
							select {
							case err := <-errChan:
								t.Errorf("Connection was impaired during rotation: %v", err)
							default:
								t.Logf("Connection remained stable during rotation.")
							}
						}

						// Verify new cert is served
						if result := setup_service.ServicesValidationCheck(t, caCertPool, expectedResult, serverSAN, serverAddr, username, password, clientCert, false); !result {
							t.Fatalf("Service validation failed after rotation.")
						}
					} else {
						// Negative Rotation
						t.Logf("Attempting negative server certificate rotation to target: %s", tc.targetCertFile)
						CertzRotateNegative(ctx, t, activeCertzClient, profileID, &targetCertEntity)

						// Verify rollback (should still work with baseline server-a)
						t.Logf("Verifying server still works with baseline server-a after failed rotation")
						if result := setup_service.ServicesValidationCheck(t, caCertPool, expectedResult, serverSAN, serverAddr, username, password, clientCert, false); !result {
							t.Fatalf("Service validation failed after negative rotation attempt.")
						}
					}
				})
			}
		})
	}
}
