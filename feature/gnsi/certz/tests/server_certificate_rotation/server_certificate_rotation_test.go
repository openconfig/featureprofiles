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
	"encoding/pem"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	setupService "github.com/openconfig/featureprofiles/feature/gnsi/certz/tests/internal/setup_service"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/gnoigo"
	authzpb "github.com/openconfig/gnsi/authz"
	certzpb "github.com/openconfig/gnsi/certz"
	gribipb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	ognmi "github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ygnmi/ygnmi"
	p4rtpb "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	dirPath                       = "../../test_data/"
	timeOutVar      time.Duration = 2 * time.Minute
	maxAllowedDelay               = 5 * time.Second
)

// DUTCredentialer is an interface for getting credentials from a DUT binding.
type DUTCredentialer interface {
	RPCUsername() string
	RPCPassword() string
}

var (
	serverAddr string
	creds      DUTCredentialer
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testCase struct {
	desc          string
	keyType       string // "rsa" or "ecdsa"
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

type serviceMaintainer struct {
	name string
	dial func(ctx context.Context) (any, error)
	ping func(ctx context.Context, client any) error
}

func dialAndGetCert(t *testing.T, addr string) *x509.Certificate {
	t.Helper()
	conf := &tls.Config{
		InsecureSkipVerify: true,
	}
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

func loadPemCert(t *testing.T, filePath string) *x509.Certificate {
	t.Helper()
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read cert file %s: %v", filePath, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatalf("Failed to decode PEM in %s", filePath)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate in %s: %v", filePath, err)
	}
	return cert
}

func awaitCert(t *testing.T, addr string, expectedCert *x509.Certificate, timeout time.Duration) {
	t.Helper()
	start := time.Now()
	for time.Since(start) < timeout {
		conf := &tls.Config{
			InsecureSkipVerify: true,
		}
		conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", addr, 9339), conf)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		certs := conn.ConnectionState().PeerCertificates
		conn.Close()
		if len(certs) > 0 {
			cert := certs[0]
			t.Logf("Awaiting cert: got subject: %s, SN: %s", cert.Subject, cert.SerialNumber.String())
			if len(cert.DNSNames) > 0 && cert.DNSNames[0] == expectedCert.DNSNames[0] && cert.SerialNumber.Cmp(expectedCert.SerialNumber) == 0 {
				return // Success
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("Timed out waiting for certificate SAN: %s, SN: %s", expectedCert.DNSNames[0], expectedCert.SerialNumber.String())
}

// configureAndVerifyMTLS configures and verifies that the specified services (gNMI, gNOI, gNSI, gRIBI, P4RT)
// on the DUT are up and require mTLS for authentication as specified in the Baseline Setup.
func configureAndVerifyMTLS(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, gnmiClient gnmipb.GNMIClient, profileID string, caCertPool *x509.CertPool, clientCert tls.Certificate, san, username, password string) {
	t.Helper()
	t.Log("Baseline Setup: configuring and verifying DUT services (gNMI, gNOI, gNSI, gRIBI, P4RT) require mTLS authentication...")

	servers := ognmi.GetAll(t, dut.GNMIOpts().WithClient(gnmiClient), ognmi.OC().System().GrpcServerAny().Name().State())
	if len(servers) > 0 {
		batch := ognmi.SetBatch{}
		for _, server := range servers {
			ognmi.BatchReplace(&batch, ognmi.OC().System().GrpcServer(server).CertificateId().Config(), profileID)
			ognmi.BatchReplace(&batch, ognmi.OC().System().GrpcServer(server).TransportSecurity().Config(), true)
		}
		batch.Set(t, dut.GNMIOpts().WithClient(gnmiClient))

		for _, server := range servers {
			ognmi.Watch(t, dut.GNMIOpts().WithClient(gnmiClient), ognmi.OC().System().GrpcServer(server).CertificateId().State(), 30*time.Second, func(val *ygnmi.Value[string]) bool {
				v, present := val.Val()
				return present && v == profileID
			}).Await(t)
		}
	}

	// Verify all services (gNMI, gNOI, gNSI, gRIBI, P4RT) validate with mTLS credentials.
	if ok := setupService.ServicesValidationCheck(t, caCertPool, true, san, serverAddr, username, password, clientCert, false); !ok {
		t.Fatalf("Baseline Setup: services mTLS validation check failed")
	}
	t.Log("Baseline Setup: all services verified up and enforcing mTLS authentication.")
}

func TestServerCertificateRotation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name()
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("Failed to get DUT credentials: %v", err)
	}
	username := creds.RPCUsername()
	password := creds.RPCPassword()

	// Baseline Setup: Input Args - Generate test data using mk_cas.sh
	t.Log("Baseline Setup: Generating certificate test data using mk_cas.sh...")
	if err := setupService.TestdataMakeCleanup(t, dirPath, timeOutVar, "./mk_cas.sh"); err != nil {
		t.Fatalf("Failed to generate test data: %v", err)
	}
	defer func() {
		t.Log("Cleaning up test data...")
		if err := setupService.TestdataMakeCleanup(t, dirPath, timeOutVar, "./cleanup.sh"); err != nil {
			t.Errorf("Failed to cleanup test data: %v", err)
		}
	}()

	posCases := []testCase{
		{
			desc:          "Certz-3.1: RSA server certificate rotation (Positive)",
			keyType:       "rsa",
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
	}

	negCases := []testCase{
		{
			desc:          "Certz-3.2: RSA server certificate rotation (Negative - Untrusted)",
			keyType:       "rsa",
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

	for _, tc := range posCases {
		t.Run(tc.desc, func(t *testing.T) {
			runCertz31Positive(t, dut, username, password, tc)
		})
	}

	for _, tc := range negCases {
		t.Run(tc.desc, func(t *testing.T) {
			runCertz32Negative(t, dut, username, password, tc)
		})
	}
}

// runCertz31Positive executes Certz-3.1 positive test steps.
func runCertz31Positive(t *testing.T, dut *ondatra.DUTDevice, username, password string, tc testCase) {
	ctx := context.Background()
	t.Logf("Running positive test case: %s", tc.desc)

	// Step 3.1.0: Build the test data, configure the DUT to use ca-0001 form
	// key/certificate/trust_bundle, use server-${TYPE}-a key/certificate.
	t.Log("Step 3.1.0: Configuring DUT to use ca-0001 form key/certificate/trust_bundle with server-${TYPE}-a...")
	_, gnsiC := setupService.PreInitCheck(ctx, t, dut)
	certzClient := gnsiC.Certz()

	profileID := "profile_" + tc.keyType

	if _, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: profileID}); err != nil {
		if status.Code(err) != codes.AlreadyExists {
			t.Fatalf("Failed to add profile %s: %v", profileID, err)
		}
		t.Logf("Profile %s already exists, continuing...", profileID)
	}

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

	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial gNMI: %v", err)
	}

	if err := rotateAndFinalize(ctx, t, dut, certzClient, gnmiClient, profileID, &initialServerCertEntity, &initialTrustBundleEntity); err != nil {
		t.Fatalf("Failed to setup initial certificate 'a': %v", err)
	}

	// Baseline Setup: verify DUT services require mTLS authentication
	initialSAN := setupService.ReadDecodeServerCertificate(t, tc.initialCert)
	initialClientCert, err := tls.LoadX509KeyPair(tc.clientCert, tc.clientKey)
	if err != nil {
		t.Fatalf("Failed to load initial client cert: %v", err)
	}
	configureAndVerifyMTLS(ctx, t, dut, gnmiClient, profileID, initialCaCertPool, initialClientCert, initialSAN, username, password)

	// Step 3.1.1: Connect a client to the service on the DUT. The client should maintain
	// its connection to the service throughout the rotation process being undertaken.
	t.Log("Step 3.1.1: Establishing persistent connections to services on DUT throughout rotation...")
	var connImpaired atomic.Bool
	var connDelayed atomic.Bool
	stopMaintain := make(chan struct{})
	var wg sync.WaitGroup

	maintainers := []serviceMaintainer{
		{
			name: "gNMI",
			dial: func(ctx context.Context) (any, error) {
				return dut.RawAPIs().BindingDUT().DialGNMI(ctx)
			},
			ping: func(ctx context.Context, client any) error {
				_, err := client.(gnmipb.GNMIClient).Capabilities(ctx, &gnmipb.CapabilityRequest{})
				return err
			},
		},
		{
			name: "gNOI",
			dial: func(ctx context.Context) (any, error) {
				return dut.RawAPIs().BindingDUT().DialGNOI(ctx)
			},
			ping: func(ctx context.Context, client any) error {
				_, err := client.(gnoigo.Clients).System().Ping(ctx, &spb.PingRequest{})
				return err
			},
		},
		{
			name: "gRIBI",
			dial: func(ctx context.Context) (any, error) {
				return dut.RawAPIs().BindingDUT().DialGRIBI(ctx)
			},
			ping: func(ctx context.Context, client any) error {
				_, err := client.(gribipb.GRIBIClient).Get(ctx, &gribipb.GetRequest{})
				return err
			},
		},
		{
			name: "P4RT",
			dial: func(ctx context.Context) (any, error) {
				return dut.RawAPIs().BindingDUT().DialP4RT(ctx)
			},
			ping: func(ctx context.Context, client any) error {
				_, err := client.(p4rtpb.P4RuntimeClient).Capabilities(ctx, &p4rtpb.CapabilitiesRequest{})
				return err
			},
		},
		{
			name: "gNSI",
			dial: func(ctx context.Context) (any, error) {
				return dut.RawAPIs().BindingDUT().DialGNSI(ctx)
			},
			ping: func(ctx context.Context, client any) error {
				_, err := client.(binding.GNSIClients).Authz().Get(ctx, &authzpb.GetRequest{})
				return err
			},
		},
	}

	for _, m := range maintainers {
		wg.Add(1)
		go func(m serviceMaintainer) {
			defer wg.Done()
			t.Logf("Starting maintainer for %s", m.name)
			client, err := m.dial(ctx)
			if err != nil {
				t.Logf("Maintain %s: failed to dial: %v", m.name, err)
				connImpaired.Store(true)
				return
			}
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-stopMaintain:
					return
				case <-ticker.C:
					start := time.Now()
					if err := m.ping(ctx, client); err != nil {
						t.Logf("Maintain %s: ping failed (impaired/restarted): %v", m.name, err)
						connImpaired.Store(true)
						return
					}
					elapsed := time.Since(start)
					if elapsed > maxAllowedDelay {
						t.Logf("Maintain %s: ping response delayed: took %v (max allowed %v)", m.name, elapsed, maxAllowedDelay)
						connDelayed.Store(true)
					}
				}
			}
		}(m)
	}
	defer wg.Wait()

	// Step 3.1.2: With the server running, connect and note that the certificate loaded
	// is the appropriate one, that it is the 'a' certificate in the ca-0001 set, validate SN/SAN.
	t.Log("Step 3.1.2: Connecting and validating active certificate is 'a' with correct SN and SAN...")
	expectedInitialCert := loadPemCert(t, tc.initialCert)
	initialCert := dialAndGetCert(t, serverAddr)
	t.Logf("Initial certificate subject: %s, SN: %s", initialCert.Subject, initialCert.SerialNumber.String())
	if len(initialCert.DNSNames) == 0 || initialCert.DNSNames[0] != expectedInitialCert.DNSNames[0] {
		t.Fatalf("Step 3.1.2: Initial certificate SAN mismatch: got %v, want %v", initialCert.DNSNames, expectedInitialCert.DNSNames)
	}
	if initialCert.SerialNumber.Cmp(expectedInitialCert.SerialNumber) != 0 {
		t.Fatalf("Step 3.1.2: Initial certificate SN mismatch: got %s, want %s", initialCert.SerialNumber.String(), expectedInitialCert.SerialNumber.String())
	}

	// Step 3.1.3: Use the gNSI Rotate RPC to load a server-${TYPE}-b key and certificate on to the server.
	t.Log("Step 3.1.3: Using gNSI Rotate RPC to load server-${TYPE}-b key and certificate...")
	targetServerCert := setupService.CreateCertzChain(t, setupService.CertificateChainRequest{
		RequestType:    setupService.EntityTypeCertificateChain,
		ServerCertFile: tc.targetCert,
		ServerKeyFile:  tc.targetKey,
	})
	targetServerCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &targetServerCert, tc.cversion)

	_, targetPkcs7data, err := setupService.Loadpkcs7TrustBundle(tc.targetBundle)
	if err != nil {
		t.Fatalf("Failed to load target trust bundle: %v", err)
	}
	targetTrustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, string(targetPkcs7data), tc.bversion)

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
	if err := stream.Send(req); err != nil {
		t.Fatalf("Step 3.1.3: Failed to send Rotate request: %v", err)
	}
	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("Step 3.1.3: Failed to receive Rotate response: %v", err)
	}
	t.Logf("Received Rotate response: %v", resp)

	// Step 3.1.4: Test that the certificate is properly loaded, using the Probe RPC.
	// Note that the new certificate is properly served by the server. Note that the certificate's SN/SAN has changed to 'b'.
	t.Log("Step 3.1.4: Probing new certificate using Probe RPC and validating SN/SAN has changed to 'b'...")
	expectedTargetCert := loadPemCert(t, tc.targetCert)
	targetSAN := expectedTargetCert.DNSNames[0]

	clientCert, err := tls.LoadX509KeyPair(tc.clientCert, tc.clientKey)
	if err != nil {
		t.Fatalf("Failed to load client cert for probing: %v", err)
	}
	targetCerts, _, err := setupService.Loadpkcs7TrustBundle(tc.targetBundle)
	if err != nil {
		t.Fatalf("Failed to load target trust bundle: %v", err)
	}
	targetCaCertPool := x509.NewCertPool()
	for _, c := range targetCerts {
		targetCaCertPool.AddCert(c)
	}

	conn := setupService.CreateNewDialOption(t, clientCert, targetCaCertPool, targetSAN, username, password, serverAddr)
	defer conn.Close()

	activeAuthzClient := authzpb.NewAuthzClient(conn)
	_, err = activeAuthzClient.Probe(ctx, &authzpb.ProbeRequest{
		User: username,
		Rpc:  "/gnsi.authz.v1.Authz/Probe",
	})
	if err != nil {
		t.Fatalf("Step 3.1.4: Probe RPC failed: %v", err)
	}
	t.Log("Step 3.1.4: Probe RPC successful.")

	probeCert := dialAndGetCert(t, serverAddr)
	t.Logf("Probe certificate subject: %s, SN: %s", probeCert.Subject, probeCert.SerialNumber.String())
	if len(probeCert.DNSNames) == 0 || probeCert.DNSNames[0] != expectedTargetCert.DNSNames[0] {
		t.Fatalf("Step 3.1.4: Probe certificate SAN mismatch: got %v, want %v", probeCert.DNSNames, expectedTargetCert.DNSNames)
	}
	if probeCert.SerialNumber.Cmp(expectedTargetCert.SerialNumber) != 0 {
		t.Fatalf("Step 3.1.4: Probe certificate SN mismatch: got %s, want %s", probeCert.SerialNumber.String(), expectedTargetCert.SerialNumber.String())
	}
	t.Log("Step 3.1.4: Certificate 'b' properly loaded and verified with Probe RPC.")

	// Step 3.1.5: Send the Finalize RPC to the server.
	t.Log("Step 3.1.5: Sending Finalize RPC to server...")
	finalizeReq := &certzpb.RotateCertificateRequest{
		ForceOverwrite: false,
		SslProfileId:   profileID,
		RotateRequest: &certzpb.RotateCertificateRequest_FinalizeRotation{
			FinalizeRotation: &certzpb.FinalizeRequest{},
		},
	}
	if err := stream.Send(finalizeReq); err != nil {
		t.Fatalf("Step 3.1.5: Failed to send Finalize request: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("Step 3.1.5: Failed to CloseSend: %v", err)
	}

	// Step 3.1.6: Verify that the server is now serving the certificate properly, that
	// the certificate is the 'b' certificate. Verify that the active gRPC server
	// certificate has changed by checking the following OpenConfig paths:
	// * /system/grpc-servers/grpc-server/state/certificate-created-on
	// * /system/grpc-servers/grpc-server/state/certificate-expires-on
	// * /system/grpc-servers/grpc-server/state/certificate-version
	t.Log("Step 3.1.6: Verifying server is serving 'b' and checking OpenConfig state paths...")
	awaitCert(t, serverAddr, expectedTargetCert, 30*time.Second)

	servers := ognmi.GetAll(t, dut.GNMIOpts().WithClient(gnmiClient), ognmi.OC().System().GrpcServerAny().Name().State())
	for _, server := range servers {
		createdVal, createdPresent := ognmi.Lookup(t, dut.GNMIOpts().WithClient(gnmiClient), ognmi.OC().System().GrpcServer(server).CertificateCreatedOn().State()).Val()
		expiresVal, expiresPresent := ognmi.Lookup(t, dut.GNMIOpts().WithClient(gnmiClient), ognmi.OC().System().GrpcServer(server).CertificateExpiresOn().State()).Val()
		versionVal, versionPresent := ognmi.Lookup(t, dut.GNMIOpts().WithClient(gnmiClient), ognmi.OC().System().GrpcServer(server).CertificateVersion().State()).Val()

		t.Logf("OpenConfig check for server %s: created-on=%d (present=%v), expires-on=%d (present=%v), version=%s (present=%v)",
			server, createdVal, createdPresent, expiresVal, expiresPresent, versionVal, versionPresent)
		if versionPresent && versionVal != tc.cversion {
			t.Errorf("Step 3.1.6: CertificateVersion mismatch for server %s: got %s, want %s", server, versionVal, tc.cversion)
		}
	}
	t.Log("Step 3.1.6 passed: verified server is serving 'b' certificate and checked OpenConfig paths.")

	// Step 3.1.7: Verify that at no time during the rotation process were existing
	// connections to the service impaired / restarted / delayed due to the rotation event.
	t.Log("Step 3.1.7: Verifying maintained connections were not impaired, restarted, or delayed...")
	close(stopMaintain)
	if connImpaired.Load() {
		t.Fatalf("Step 3.1.7 failed: Maintained connection was impaired or restarted during rotation")
	}
	if connDelayed.Load() {
		t.Fatalf("Step 3.1.7 failed: Maintained connection was delayed during rotation")
	}
	t.Log("Step 3.1.7 passed: Maintained connections were not impaired, restarted, or delayed during rotation.")
}

// runCertz32Negative executes Certz-3.2 negative test steps.
func runCertz32Negative(t *testing.T, dut *ondatra.DUTDevice, username, password string, tc testCase) {
	ctx := context.Background()
	t.Logf("Running negative test case: %s", tc.desc)

	// Step 3.2.0: Build the test data, configure the DUT to use ca-0001 form
	// key/certificate/trust_bundle, use server-${TYPE}-a key/certificate.
	t.Log("Step 3.2.0: Configuring DUT to use ca-0001 form key/certificate/trust_bundle with server-${TYPE}-a...")
	_, gnsiC := setupService.PreInitCheck(ctx, t, dut)
	certzClient := gnsiC.Certz()

	profileID := "profile_" + tc.keyType

	if _, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{SslProfileId: profileID}); err != nil {
		if status.Code(err) != codes.AlreadyExists {
			t.Fatalf("Failed to add profile %s: %v", profileID, err)
		}
		t.Logf("Profile %s already exists, continuing...", profileID)
	}

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

	gnmiClient, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("Failed to dial gNMI: %v", err)
	}

	if err := rotateAndFinalize(ctx, t, dut, certzClient, gnmiClient, profileID, &initialServerCertEntity, &initialTrustBundleEntity); err != nil {
		t.Fatalf("Failed to setup initial certificate 'a': %v", err)
	}

	// Baseline Setup: verify DUT services require mTLS authentication
	initialSAN := setupService.ReadDecodeServerCertificate(t, tc.initialCert)
	initialClientCert, err := tls.LoadX509KeyPair(tc.clientCert, tc.clientKey)
	if err != nil {
		t.Fatalf("Failed to load initial client cert: %v", err)
	}
	configureAndVerifyMTLS(ctx, t, dut, gnmiClient, profileID, initialCaCertPool, initialClientCert, initialSAN, username, password)

	// Step 3.2.1: With the server running, connect and note that the certificate loaded is the appropriate one ('a').
	t.Log("Step 3.2.1: Connecting and validating active certificate is the initial certificate 'a'...")
	expectedInitialCert := loadPemCert(t, tc.initialCert)
	initialCert := dialAndGetCert(t, serverAddr)
	t.Logf("Initial certificate subject: %s, SN: %s", initialCert.Subject, initialCert.SerialNumber.String())
	if len(initialCert.DNSNames) == 0 || initialCert.DNSNames[0] != expectedInitialCert.DNSNames[0] {
		t.Fatalf("Step 3.2.1: Initial certificate SAN mismatch: got %v, want %v", initialCert.DNSNames, expectedInitialCert.DNSNames)
	}
	if initialCert.SerialNumber.Cmp(expectedInitialCert.SerialNumber) != 0 {
		t.Fatalf("Step 3.2.1: Initial certificate SN mismatch: got %s, want %s", initialCert.SerialNumber.String(), expectedInitialCert.SerialNumber.String())
	}

	// Step 3.2.2: Use the gNSI Rotate RPC to load a ca-02/server-${TYPE}-b key and certificate on to the server.
	t.Log("Step 3.2.2: Using gNSI Rotate RPC to load ca-02/server-${TYPE}-b untrusted certificate...")
	targetServerCert := setupService.CreateCertzChain(t, setupService.CertificateChainRequest{
		RequestType:    setupService.EntityTypeCertificateChain,
		ServerCertFile: tc.targetCert,
		ServerKeyFile:  tc.targetKey,
	})
	targetServerCertEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeCertificateChain, &targetServerCert, tc.cversion)

	_, targetPkcs7data, err := setupService.Loadpkcs7TrustBundle(tc.targetBundle)
	if err != nil {
		t.Fatalf("Failed to load target trust bundle: %v", err)
	}
	targetTrustBundleEntity := setupService.CreateCertzEntity(t, setupService.EntityTypeTrustBundle, string(targetPkcs7data), tc.bversion)

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
	if err := stream.Send(req); err != nil {
		t.Logf("Step 3.2.3: Rotate Send rejected untrusted certificate as expected: %v", err)
		return
	}
	resp, err := stream.Recv()
	if err != nil {
		t.Logf("Step 3.2.3: Rotate Recv returned error for untrusted certificate as expected: %v", err)
		return
	}
	t.Logf("Received Rotate response: %v", resp)

	// Step 3.2.3: Test that the certificate load fails, because the certificate is not trusted by a known CA.
	t.Log("Step 3.2.3: Testing that untrusted certificate load fails...")
	currentCert := dialAndGetCert(t, serverAddr)
	t.Logf("Current certificate subject: %s, SN: %s", currentCert.Subject, currentCert.SerialNumber.String())

	if len(currentCert.DNSNames) > 0 && currentCert.DNSNames[0] == initialSAN {
		t.Log("Step 3.2.3 passed: Device is still serving initial cert 'a' as expected (untrusted cert 'b' rejected).")
	} else {
		// Untrusted certificate rotation unexpectedly succeeded! Assert test failure as required by Step 3.2.3.
		t.Errorf("Step 3.2.3 failed: untrusted certificate load unexpectedly succeeded; device is serving untrusted certificate: %v", currentCert.DNSNames)
	}

	// Step 3.2.4: Tear down the Rotate RPC, forcing the device to return to the previously used certificate/key material.
	t.Log("Step 3.2.4: Tearing down Rotate RPC to force rollback...")
	stream.CloseSend()

	// Step 3.2.5: Verify that the server is now serving the previous certificate properly.
	t.Log("Step 3.2.5: Verifying server is now serving previous certificate 'a' properly...")
	expectedInitialCert = loadPemCert(t, tc.initialCert)
	awaitCert(t, serverAddr, expectedInitialCert, 30*time.Second)
	t.Log("Step 3.2.5 passed: successfully verified rollback to previous certificate 'a'.")
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
	if len(servers) > 0 {
		server := servers[0]
		_, ok := ognmi.Watch(t, dut.GNMIOpts().WithClient(gnmiClient), ognmi.OC().System().GrpcServer(server).CertificateId().State(), 30*time.Second, func(val *ygnmi.Value[string]) bool {
			v, present := val.Val()
			return present && v == profileID
		}).Await(t)
		if !ok {
			return fmt.Errorf("timed out waiting for gNMI config to propagate for profile %s", profileID)
		}
	}

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
