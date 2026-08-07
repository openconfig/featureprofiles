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
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/feature/gnsi/certz/tests/internal/setup_service"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnmi/proto/gnmi"
	authzpb "github.com/openconfig/gnsi/authz"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

const (
	scriptPath               = "../../test_data/"
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
	prevClientCertFile  string = ""
	prevClientKeyFile   string = ""
	prevTrustBundleFile string = ""
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// verifyServices explicitly validates connections for all required gRPC services: gNMI, gNOI, gNSI, gRIBI, and P4RT.
func verifyServices(t *testing.T, caCert *x509.CertPool, expectedResult bool, san, serverAddr, username, password string, cert tls.Certificate, mismatch bool) bool {
	t.Helper()
	t.Logf("%s: Verifying gNMI, gNOI, gNSI, gRIBI, and P4RT connections.", time.Now().String())
	if result := setup_service.VerifyGnmi(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Errorf("gNMI service verification failed: got %v, want %v", result, expectedResult)
		return false
	}
	if result := setup_service.VerifyGnoi(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Errorf("gNOI service verification failed: got %v, want %v", result, expectedResult)
		return false
	}
	if result := setup_service.VerifyGnsi(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Errorf("gNSI service verification failed: got %v, want %v", result, expectedResult)
		return false
	}
	if result := setup_service.VerifyGribi(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Errorf("gRIBI service verification failed: got %v, want %v", result, expectedResult)
		return false
	}
	if result := setup_service.VerifyP4rt(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Errorf("P4RT service verification failed: got %v, want %v", result, expectedResult)
		return false
	}
	return true
}

func createCombinedBundle(t *testing.T, dirPath, typeStr string) string {
	t.Helper()
	pem1 := filepath.Join(dirPath, "ca-01", fmt.Sprintf("trust_bundle_01_%s.pem", typeStr))
	pem2 := filepath.Join(dirPath, "ca-02", fmt.Sprintf("trust_bundle_02_%s.pem", typeStr))
	tempDir := t.TempDir()
	combinedPem := filepath.Join(tempDir, fmt.Sprintf("combined_%s.pem", typeStr))

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

	return combinedPem
}

func verifyServedCertificate(t *testing.T, peerCert *x509.Certificate, expectedCertFile string) {
	t.Helper()
	sc, err := os.ReadFile(expectedCertFile)
	if err != nil {
		t.Fatalf("Failed to read expected certificate file: %v", err)
	}
	block, _ := pem.Decode(sc)
	if block == nil {
		t.Fatalf("Failed to parse expected PEM block")
	}
	expectedCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse expected certificate: %v", err)
	}

	if !bytes.Equal(peerCert.Raw, expectedCert.Raw) {
		t.Errorf("Served certificate does not match the expected certificate. Subject: %v, Expected Subject: %v", peerCert.Subject, expectedCert.Subject)
	} else {
		t.Logf("Verified that the same certificate is properly served by the server: %v", peerCert.Subject)
	}
}

func CertzRotateNegative(ctx context.Context, t *testing.T, certzClient certzpb.CertzClient, profileID string, targetCaCert *x509.CertPool, serverSAN, username, password string, clientCert tls.Certificate, entities ...*certzpb.Entity) {
	t.Helper()
	if len(entities) == 0 {
		t.Fatalf("At least one entity required for Rotate request.")
	}
	rotateCertRequest := &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profileID,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: entities,
			},
		},
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

	resp, err := rotateRequestClient.Recv()
	if err != nil {
		t.Logf("Recv failed as expected: %v", err)
		return
	}
	t.Logf("Received Rotate certificate response: %v", resp)

	// Certz-5.2: the ca-02 trust bundle must not work with the ca-01 server certificate.
	if result := setup_service.VerifyGnsi(t, targetCaCert, serverSAN, serverAddr, username, password, clientCert, true); !result {
		t.Fatalf("Expected gNSI verification to fail with mismatched trust bundle, but connection succeeded.")
	}
	t.Logf("Negative rotation validated: mismatched trust bundle cannot be used.")
}

func TestTrustBundleRotation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name()
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("Failed to get DUT credentials: %v", err)
	}
	username := creds.RPCUsername()
	password := creds.RPCPassword()

	dirPath := t.TempDir()
	// Generate testdata certificates.
	t.Logf("%s:Creation of test data in %s.", logTime, dirPath)
	if err := setup_service.TestdataMakeCleanup(t, scriptPath, timeOutVar, "./mk_cas.sh", dirPath, "01,02"); err != nil {
		t.Fatalf("Generation of testdata certificates failed!: %v", err)
	}
	defer func() {
		t.Logf("%s:Cleanup of test data.", logTime)
		if err := setup_service.TestdataMakeCleanup(t, scriptPath, timeOutVar, "./cleanup.sh", dirPath); err != nil {
			t.Logf("Cleanup of testdata certificates failed!: %v", err)
		}
	}()

	ctx := context.Background()
	gnmiClient, gnsiC := setup_service.PreInitCheck(ctx, t, dut)
	certzClient := gnsiC.Certz()

	types := []string{"rsa", "ecdsa"}

	for _, typeStr := range types {
		t.Run(fmt.Sprintf("Type_%s", typeStr), func(t *testing.T) {
			// Define baseline files (ca-01)
			serverCertFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("server-%s-a-cert.pem", typeStr))
			serverKeyFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("server-%s-a-key.pem", typeStr))
			trustBundleFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("trust_bundle_01_%s.pem", typeStr))
			clientCertFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("client-%s-a-cert.pem", typeStr))
			clientKeyFile := filepath.Join(dirPath, "ca-01", fmt.Sprintf("client-%s-a-key.pem", typeStr))

			serverSAN := setup_service.ReadDecodeServerCertificate(t, serverCertFile)

			// Load client cert
			clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
			if err != nil {
				t.Fatalf("Failed to load client cert: %v", err)
			}

			// Load trust bundle
			pemData, err := os.ReadFile(trustBundleFile)
			if err != nil {
				t.Fatalf("Failed to read trust bundle: %v", err)
			}
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(pemData) {
				t.Fatalf("Failed to append certs from PEM: %s", trustBundleFile)
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
						return filepath.Join(dirPath, "ca-02", fmt.Sprintf("trust_bundle_02_%s.pem", typeStr))
					},
				},
			}

			for _, tc := range cases {
				t.Run(tc.desc, func(t *testing.T) {
					profileID := fmt.Sprintf("%s_%s_%s", testProfile, typeStr, tc.desc)

					if prevClientCertFile != "" {
						t.Logf("Creating new TLS credentials for client connection.")
						// Load the prior client keypair for new client TLS credentials.
						prevClientCert, err := tls.LoadX509KeyPair(prevClientCertFile, prevClientKeyFile)
						if err != nil {
							t.Fatalf("Failed to load previous client cert: %v", err)
						}
						prevPemData, err := os.ReadFile(prevTrustBundleFile)
						if err != nil {
							t.Fatalf("Failed to read previous trust bundle: %v", err)
						}
						// Create a old set of Cert Pool and append the certs from previous trust bundle.
						prevCaCert := x509.NewCertPool()
						if !prevCaCert.AppendCertsFromPEM(prevPemData) {
							t.Fatalf("Failed to append certs from previous trust bundle PEM: %s", prevTrustBundleFile)
						}
						// Retrieve the connection with previous TLS credentials for certz rotation.
						conn := setup_service.CreateNewDialOption(t, prevClientCert, prevCaCert, serverSAN, username, password, serverAddr)
						defer conn.Close()
						certzClient = certzpb.NewCertzClient(conn)
						gnmiClient = gnmi.NewGNMIClient(conn)
					}

					if getResp := setup_service.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); slices.Contains(getResp.SslProfileIds, profileID) {
						t.Logf("profileID %s already exists, skipping AddProfile.", profileID)
					} else {
						// Add new sslprofileID.
						t.Logf("Adding new empty sslprofile ID %s.", profileID)
						if _, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: profileID}); err != nil {
							t.Fatalf("Add profile request failed: %v", err)
						}
					}

					// 1. Setup Baseline (ca-01)
					serverCert := setup_service.CreateCertzChain(t, setup_service.CertificateChainRequest{
						RequestType:    setup_service.EntityTypeCertificateChain,
						ServerCertFile: serverCertFile,
						ServerKeyFile:  serverKeyFile,
					})
					serverCertEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeCertificateChain, &serverCert, "v1")
					trustBundleChain := setup_service.CreateCertzChain(t, setup_service.CertificateChainRequest{
						RequestType:     setup_service.EntityTypeTrustBundle,
						TrustBundleFile: trustBundleFile,
					})
					trustBundleEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeTrustBundlePEM, &trustBundleChain, "bundle1")

					t.Logf("Installing baseline profile %s with ca-01", profileID)
					if success := setup_service.CertzRotate(ctx, t, caCertPool, certzClient, gnmiClient, clientCert, dut, username, password, serverSAN, serverAddr, profileID, false, false, false, &serverCertEntity, &trustBundleEntity); !success {
						t.Fatalf("Baseline CertzRotation failed.")
					}

					// Verify baseline works
					if result := verifyServices(t, caCertPool, expectedResult, serverSAN, serverAddr, username, password, clientCert, false); !result {
						t.Fatalf("Baseline service validation failed.")
					}

					// Create new client connection using the baseline credentials
					conn := setup_service.CreateNewDialOption(t, clientCert, caCertPool, serverSAN, username, password, serverAddr)
					defer conn.Close()
					activeCertzClient := certzpb.NewCertzClient(conn)

					// Prepare target trust bundle
					targetBundlePath := tc.getBundlePath(t)
					targetPemData, err := os.ReadFile(targetBundlePath)
					if err != nil {
						t.Fatalf("Failed to read target trust bundle: %v", err)
					}
					targetCaCertPool := x509.NewCertPool()
					if !targetCaCertPool.AppendCertsFromPEM(targetPemData) {
						t.Fatalf("Failed to append target certs from PEM: %s", targetBundlePath)
					}
					targetTrustBundleChain := setup_service.CreateCertzChain(t, setup_service.CertificateChainRequest{
						RequestType:     setup_service.EntityTypeTrustBundle,
						TrustBundleFile: targetBundlePath,
					})
					targetTrustBundleEntity := setup_service.CreateCertzEntity(t, setup_service.EntityTypeTrustBundlePEM, &targetTrustBundleChain, tc.version)

					if !tc.isNegative {
						// Positive Rotation - Explicit Steps
						t.Logf("Rotating trust bundle to target: %s", targetBundlePath)

						stream, err := activeCertzClient.Rotate(ctx)
						if err != nil {
							t.Fatalf("Failed to open Rotate stream: %v", err)
						}
						defer stream.CloseSend()

						req := &certzpb.RotateCertificateRequest{
							ForceOverwrite: true,
							SslProfileId:   profileID,
							RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
								Certificates: &certzpb.UploadRequest{
									Entities: []*certzpb.Entity{&targetTrustBundleEntity},
								},
							},
						}
						if err := stream.Send(req); err != nil {
							t.Fatalf("Failed to send Rotate request: %v", err)
						}

						resp, err := stream.Recv()
						if err != nil {
							t.Fatalf("Failed to receive Rotate response: %v", err)
						}
						t.Logf("Received Rotate response: %v", resp)

						// Step 4: Probe (Verify connection works with new trust pool before finalize)
						t.Log("Step 4: Probing new trust bundle using Probe RPC...")
						activeAuthzClient := authzpb.NewAuthzClient(conn)
						var p peer.Peer
						_, err = activeAuthzClient.Probe(ctx, &authzpb.ProbeRequest{
							User: username,
							Rpc:  "/gnsi.authz.v1.Authz/Probe",
						}, grpc.Peer(&p))
						if err != nil {
							t.Fatalf("Probe failed: %v", err)
						}
						t.Log("Probe successful.")

						// Verify that the same certificate is properly served by the server
						tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
						if !ok {
							t.Fatalf("Failed to get TLSInfo from peer connection info")
						}
						if len(tlsInfo.State.PeerCertificates) == 0 {
							t.Fatalf("No peer certificates served by the server")
						}
						verifyServedCertificate(t, tlsInfo.State.PeerCertificates[0], serverCertFile)

						// Step 5: Finalize
						t.Log("Step 5: Finalizing rotation...")
						finalizeReq := &certzpb.RotateCertificateRequest{
							ForceOverwrite: false,
							SslProfileId:   profileID,
							RotateRequest: &certzpb.RotateCertificateRequest_FinalizeRotation{
								FinalizeRotation: &certzpb.FinalizeRequest{},
							},
						}
						if err := stream.Send(finalizeReq); err != nil {
							t.Fatalf("Failed to send Finalize request: %v", err)
						}
						if err := stream.CloseSend(); err != nil {
							t.Fatalf("Failed to CloseSend: %v", err)
						}

						time.Sleep(5 * time.Second) // Wait for finalization to settle

						// Step 6: Post-finalization verification
						t.Log("Step 6: Verifying services after finalization...")
						if success := verifyServices(t, targetCaCertPool, expectedResult, serverSAN, serverAddr, username, password, clientCert, false); !success {
							t.Fatalf("Post-finalization service verification failed.")
						}
						t.Log("Post-finalization verification successful.")
					} else {
						// Negative Rotation
						t.Logf("Attempting negative trust bundle rotation to target: %s", targetBundlePath)
						CertzRotateNegative(ctx, t, activeCertzClient, profileID, targetCaCertPool, serverSAN, username, password, clientCert, &targetTrustBundleEntity)

						time.Sleep(5 * time.Second) // Wait for rollback

						// Verify rollback (should still work with baseline ca-01)
						t.Logf("Verifying server still works with baseline ca-01 after failed rotation")
						if result := verifyServices(t, caCertPool, expectedResult, serverSAN, serverAddr, username, password, clientCert, false); !result {
							t.Fatalf("Service validation failed after negative rotation attempt.")
						}
					}

					// Archiving previous client cert/key and trustbundle.
					prevClientCertFile = clientCertFile
					prevClientKeyFile = clientKeyFile
					prevTrustBundleFile = trustBundleFile
				})
			}
		})
	}
}
