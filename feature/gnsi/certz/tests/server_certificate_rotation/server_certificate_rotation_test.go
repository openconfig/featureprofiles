// Copyright 2026 Google LLC
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
	"testing"
	"time"

	setupService "github.com/openconfig/featureprofiles/feature/gnsi/certz/tests/internal/setup_service"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	ognmi "github.com/openconfig/ondatra/gnmi"
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
	serverAddr string
	creds      DUTCredentialer
	logTime    string = time.Now().String()
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testCase struct {
	desc          string
	keyType       string // "rsa" or "ecdsa"
	isNegative    bool
	initialCert   string
	initialKey    string
	initialBundle string
	targetCert    string
	targetKey     string
	targetBundle  string
	clientCert    string
	clientKey     string
	cversion      string
	bversion      string
}

func dialAndGetCert(t *testing.T, addr string) *x509.Certificate {
	t.Helper()
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}
	// Try to connect to gNSI port (9339)
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", addr, 9339), conf)
	if err != nil {
		t.Fatalf("Failed to dial %s: %v", addr, err)
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		t.Fatalf("No certificates returned from %s", addr)
	}
	return certs[0]
}
func TestServerCertificateRotation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name()
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("Failed to get DUT credentials: %v", err)
	}
	username := creds.RPCUsername()
	password := creds.RPCPassword()

	// Generate test data
	t.Log("Generating test data...")
	if err := setupService.TestdataMakeCleanup(t, dirPath, timeOutVar, "./mk_cas.sh"); err != nil {
		t.Fatalf("Failed to generate test data: %v", err)
	}
	defer func() {
		t.Log("Cleaning up test data...")
		if err := setupService.TestdataMakeCleanup(t, dirPath, timeOutVar, "./cleanup.sh"); err != nil {
			t.Errorf("Failed to cleanup test data: %v", err)
		}
	}()

	cases := []testCase{
		{
			desc:          "Certz-3.1: RSA server certificate rotation (Positive)",
			keyType:       "rsa",
			isNegative:    false,
			initialCert:   dirPath + "ca-01/server-rsa-a-cert.pem",
			initialKey:    dirPath + "ca-01/server-rsa-a-key.pem",
			initialBundle: dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			targetCert:    dirPath + "ca-01/server-rsa-b-cert.pem",
			targetKey:     dirPath + "ca-01/server-rsa-b-key.pem",
			targetBundle:  dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			clientCert:    dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKey:     dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:      "certz_rsa_31",
			bversion:      "bundle_rsa_31",
		},
		{
			desc:          "Certz-3.1: ECDSA server certificate rotation (Positive)",
			keyType:       "ecdsa",
			isNegative:    false,
			initialCert:   dirPath + "ca-01/server-ecdsa-a-cert.pem",
			initialKey:    dirPath + "ca-01/server-ecdsa-a-key.pem",
			initialBundle: dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			targetCert:    dirPath + "ca-01/server-ecdsa-b-cert.pem",
			targetKey:     dirPath + "ca-01/server-ecdsa-b-key.pem",
			targetBundle:  dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			clientCert:    dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKey:     dirPath + "ca-01/client-ecdsa-a-key.pem",
			cversion:      "certz_ecdsa_31",
			bversion:      "bundle_ecdsa_31",
		},
		{
			desc:          "Certz-3.2: RSA server certificate rotation (Negative - Untrusted)",
			keyType:       "rsa",
			isNegative:    true,
			initialCert:   dirPath + "ca-01/server-rsa-a-cert.pem",
			initialKey:    dirPath + "ca-01/server-rsa-a-key.pem",
			initialBundle: dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			targetCert:    dirPath + "ca-02/server-rsa-b-cert.pem",
			targetKey:     dirPath + "ca-02/server-rsa-b-key.pem",
			targetBundle:  dirPath + "ca-01/trust_bundle_01_rsa.p7b", // Keep ca-01 bundle
			clientCert:    dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKey:     dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:      "certz_rsa_32",
			bversion:      "bundle_rsa_32",
		},
		{
			desc:          "Certz-3.2: ECDSA server certificate rotation (Negative - Untrusted)",
			keyType:       "ecdsa",
			isNegative:    true,
			initialCert:   dirPath + "ca-01/server-ecdsa-a-cert.pem",
			initialKey:    dirPath + "ca-01/server-ecdsa-a-key.pem",
			initialBundle: dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			targetCert:    dirPath + "ca-02/server-ecdsa-b-cert.pem",
			targetKey:     dirPath + "ca-02/server-ecdsa-b-key.pem",
			targetBundle:  dirPath + "ca-01/trust_bundle_01_ecdsa.p7b", // Keep ca-01 bundle
			clientCert:    dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKey:     dirPath + "ca-01/client-ecdsa-a-key.pem",
			cversion:      "certz_ecdsa_32",
			bversion:      "bundle_ecdsa_32",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			runTestCase(t, dut, username, password, tc)
		})
	}
}

func runTestCase(t *testing.T, dut *ondatra.DUTDevice, username, password string, tc testCase) {
	ctx := context.Background()
	t.Logf("Running test case: %s", tc.desc)

	// 1. Pre-check and initial setup
	// We want to ensure we are starting from the "initial" state (cert 'a').
	// We will use Ondatra's default client to rotate to 'a' first.
	_, gnsiC := setupService.PreInitCheck(ctx, t, dut)
	certzClient := gnsiC.Certz()

	profileID := "profile_" + tc.keyType

	// Add profile if it doesn't exist (ignore error if it already exists, or check list)
	t.Logf("Adding sslprofileID %s", profileID)
	_, _ = certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: profileID})

	// Rotate to INITIAL cert 'a'
	t.Logf("Rotating to INITIAL certificate: %s", tc.initialCert)
	initialServerCert := setupService.CreateCertzChain(t, setupService.CertificateChainRequest{
		RequestType:    setupService.EntityTypeCertificateChain,
		ServerCertFile: tc.initialCert,
		ServerKeyFile:  tc.initialKey,
	})
	initialServerCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &initialServerCert, tc.cversion+"_init")

	pkcs7certs, pkcs7data, err := setupService.Loadpkcs7TrustBundle(tc.initialBundle)
	if err != nil {
		t.Fatalf("Failed to load initial trust bundle: %v", err)
	}
	initialCaCertPool := x509.NewCertPool()
	for _, c := range pkcs7certs {
		initialCaCertPool.AddCert(c)
	}
	initialTrustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, string(pkcs7data), tc.bversion+"_init")

	// We need a gnmi client to apply the config.
	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial gNMI: %v", err)
	}

	// Perform initial rotation to 'a' and finalize it.
	// We use a simplified version of CertzRotate here or just call it.
	// Since we want this to succeed, we can use a local helper or setupService.CertzRotate if it works.
	// Actually, setupService.CertzRotate might fail if it expects some specific state, but it should work for initial load.
	// Let's implement a local helper `rotateAndFinalize` for this to be safe and keep control.
	err = rotateAndFinalize(ctx, t, dut, certzClient, gnmiClient, profileID, &initialServerCertEntity, &initialTrustBundleEntity)
	if err != nil {
		t.Fatalf("Failed to setup initial certificate 'a': %v", err)
	}

	// Verify initial cert is loaded and has correct SAN
	serverSAN := setupService.ReadDecodeServerCertificate(t, tc.initialCert)
	initialCert := dialAndGetCert(t, serverAddr)
	t.Logf("Initial certificate subject: %s", initialCert.Subject)
	// Validate SAN
	if len(initialCert.DNSNames) == 0 || initialCert.DNSNames[0] != serverSAN {
		t.Fatalf("Initial certificate SAN mismatch: got %v, want %v", initialCert.DNSNames, serverSAN)
	}

	// NOW WE ARE IN THE INITIAL STATE (Cert 'a' is active).
	// Proceed with the actual test.

	// 2. ACTUAL TEST ROTATION
	t.Logf("Starting actual rotation to target certificate: %s", tc.targetCert)
	targetServerCert := setupService.CreateCertzChain(t, setupService.CertificateChainRequest{
		RequestType:    setupService.EntityTypeCertificateChain,
		ServerCertFile: tc.targetCert,
		ServerKeyFile:  tc.targetKey,
	})
	targetServerCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &targetServerCert, tc.cversion)

	// Target trust bundle (might be same as initial for positive, or same initial for negative)
	_, targetPkcs7data, err := setupService.Loadpkcs7TrustBundle(tc.targetBundle)
	if err != nil {
		t.Fatalf("Failed to load target trust bundle: %v", err)
	}
	targetTrustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, string(targetPkcs7data), tc.bversion)

	// Maintain connection goroutine (for positive test)
	var connImpaired bool
	stopMaintain := make(chan struct{})
	if !tc.isNegative {
		go func() {
			// Maintain connection by doing periodic gNMI Capabilities requests
			maintainConn, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
			if err != nil {
				t.Logf("Maintain conn: failed to dial: %v", err)
				connImpaired = true
				return
			}
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopMaintain:
					return
				case <-ticker.C:
					// Simple RPC to keep connection active
					_, err := maintainConn.Capabilities(ctx, &gnmipb.CapabilityRequest{})
					if err != nil {
						t.Logf("Maintain conn: Capabilities failed: %v", err)
						connImpaired = true
						return
					}
				}
			}
		}()
	}

	// Open Rotate stream
	stream, err := certzClient.Rotate(ctx)
	if err != nil {
		t.Fatalf("Failed to open Rotate stream: %v", err)
	}
	defer stream.CloseSend()

	req := &certzpb.RotateCertificateRequest{
		ForceOverwrite: false,
		SslProfileId:   profileID,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: []*certzpb.Entity{&targetServerCertEntity, &targetTrustBundleEntity},
			},
		},
	}

	// Send Rotate request
	err = stream.Send(req)
	if err != nil {
		if tc.isNegative {
			t.Logf("Negative test: Send failed as expected: %v", err)
			return // Success for negative test if it fails here
		}
		t.Fatalf("Failed to send Rotate request: %v", err)
	}

	// Recv response
	resp, err := stream.Recv()
	if err != nil {
		if tc.isNegative {
			t.Logf("Negative test: Recv failed as expected: %v", err)
			return // Success for negative test if it fails here
		}
		t.Fatalf("Failed to receive Rotate response: %v", err)
	}
	t.Logf("Received Rotate response: %v", resp)

	if tc.isNegative {
		// If it didn't fail yet, maybe it will fail during probe or we force rollback.
		t.Log("Negative test: probing certificate...")
		currentCert := dialAndGetCert(t, serverAddr)
		t.Logf("Negative test: current certificate subject: %s", currentCert.Subject)

		// If the device rejected it, it should still serve 'a'.
		initialSAN := setupService.ReadDecodeServerCertificate(t, tc.initialCert)
		if len(currentCert.DNSNames) > 0 && currentCert.DNSNames[0] == initialSAN {
			t.Log("Negative test: Device is still serving initial cert 'a' as expected (rejected 'b').")
		} else {
			t.Log("Negative test: Device is serving new cert 'b', which means it did not reject it immediately. Tearing down stream to rollback.")
		}

		// Tear down stream (rollback) by closing it without finalize
		stream.CloseSend()
		time.Sleep(5 * time.Second) // Wait for rollback

		// Verify it reverted to 'a'
		postRollbackCert := dialAndGetCert(t, serverAddr)
		if len(postRollbackCert.DNSNames) == 0 || postRollbackCert.DNSNames[0] != initialSAN {
			t.Fatalf("Failed to rollback to initial cert 'a': got %v, want %v", postRollbackCert.DNSNames, initialSAN)
		}
		t.Log("Negative test passed: successfully rolled back to initial cert.")
		return
	}

	// POSITIVE TEST CONTINUATION
	// 3. Probe
	t.Log("Positive test: probing new certificate...")
	targetSAN := setupService.ReadDecodeServerCertificate(t, tc.targetCert)
	probeCert := dialAndGetCert(t, serverAddr)
	t.Logf("Probe certificate subject: %s", probeCert.Subject)
	if len(probeCert.DNSNames) == 0 || probeCert.DNSNames[0] != targetSAN {
		t.Fatalf("Probe certificate SAN mismatch: got %v, want %v", probeCert.DNSNames, targetSAN)
	}
	t.Log("Probe successful: new certificate is being served.")

	// 4. Finalize
	t.Log("Finalizing rotation...")
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

	// 5. Verify post-finalize
	postFinalizeCert := dialAndGetCert(t, serverAddr)
	if len(postFinalizeCert.DNSNames) == 0 || postFinalizeCert.DNSNames[0] != targetSAN {
		t.Fatalf("Post-finalize certificate SAN mismatch: got %v, want %v", postFinalizeCert.DNSNames, targetSAN)
	}
	t.Log("Post-finalize verification successful: new certificate is permanently served.")

	// 6. Verify maintained connection
	if stopMaintain != nil {
		close(stopMaintain)
	}
	if connImpaired {
		t.Fatalf("Maintained connection was impaired during rotation")
	}
	t.Log("Maintained connection was NOT impaired during rotation.")
}

func rotateAndFinalize(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, certzClient certzpb.CertzClient, gnmiClient gnmipb.GNMIClient, profileID string, entities ...*certzpb.Entity) error {
	t.Helper()
	stream, err := certzClient.Rotate(ctx)
	if err != nil {
		return fmt.Errorf("failed to open Rotate stream: %w", err)
	}
	defer stream.CloseSend()

	req := &certzpb.RotateCertificateRequest{
		ForceOverwrite: false,
		SslProfileId:   profileID,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: entities,
			},
		},
	}
	if err := stream.Send(req); err != nil {
		return fmt.Errorf("failed to send Rotate request: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive Rotate response: %w", err)
	}
	t.Logf("Received Rotate response: %v", resp)

	// Apply gNMI config to use this profile.
	servers := ognmi.GetAll(t, dut.GNMIOpts().WithClient(gnmiClient), ognmi.OC().System().GrpcServerAny().Name().State())
	batch := ognmi.SetBatch{}
	for _, server := range servers {
		ognmi.BatchReplace(&batch, ognmi.OC().System().GrpcServer(server).CertificateId().Config(), profileID)
	}
	batch.Set(t, dut.GNMIOpts().WithClient(gnmiClient))
	t.Logf("Applied gNMI config for profile %s", profileID)
	time.Sleep(10 * time.Second) // Wait for config to propagate

	// Finalize
	finalizeReq := &certzpb.RotateCertificateRequest{
		ForceOverwrite: false,
		SslProfileId:   profileID,
		RotateRequest: &certzpb.RotateCertificateRequest_FinalizeRotation{
			FinalizeRotation: &certzpb.FinalizeRequest{},
		},
	}
	if err := stream.Send(finalizeReq); err != nil {
		return fmt.Errorf("failed to send Finalize request: %w", err)
	}

	if err := stream.CloseSend(); err != nil {
		return fmt.Errorf("failed to CloseSend: %w", err)
	}

	return nil
}
