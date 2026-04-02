package trustbundlerotation_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	setupsvc "github.com/openconfig/featureprofiles/feature/gnsi/certz/tests/internal/setup_service"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnmi/proto/gnmi"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
)

const (
	dirPath        = "../../test_data/"
	scriptWaitTime = 20 * time.Minute
)

type DUTCredentialer interface {
	RPCUsername() string
	RPCPassword() string
}

type rotationTestCase struct {
	desc             string
	positiveRotation bool
	serverCertFile   string
	serverKeyFile    string
	trustBundleFile  string
	ca01PEMFile      string
	ca02PEMFile      string
	mismatchTBFile   string
	clientCertFile   string
	clientKeyFile    string
	cversion         string
	bversion         string
	newTLScreds      bool
	mismatch         bool
	scale            bool
}

type rotationRunConfig struct {
	tc               rotationTestCase
	ctx              context.Context
	dut              *ondatra.DUTDevice
	certzClient      certzpb.CertzClient
	gnmiClient       gnmi.GNMIClient
	serverSAN        string
	username         string
	password         string
	serverCertEntity *certzpb.Entity
}

var (
	serverAddr          string
	dutCreds            DUTCredentialer
	testProfile         = "rotationprofile"
	expectedResult      = true
	prevClientCertFile  = ""
	prevClientKeyFile   = ""
	prevTrustBundleFile = ""
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// mustConcatenatePEMFiles reads two PEM bundle files and concatenates their contents,
// returning the combined bytes. This is used to build a merged trust bundle for Certz-5.1.
func mustConcatenatePEMFiles(t *testing.T, fileA, fileB string) []byte {
	t.Helper()
	dataA, err := os.ReadFile(fileA)
	if err != nil {
		t.Fatalf("STATUS:Failed to read PEM file %s: %v", fileA, err)
	}
	dataA = bytes.TrimRight(dataA, "\n")
	dataB, err := os.ReadFile(fileB)
	if err != nil {
		t.Fatalf("STATUS:Failed to read PEM file %s: %v", fileB, err)
	}
	return bytes.Join([][]byte{dataA, dataB}, []byte("\n"))
}

// mustBuildCertPool constructs an x509.CertPool from a PKCS#7 trust bundle file.
func mustBuildCertPool(t *testing.T, p7bFile string) (*x509.CertPool, []byte) {
	t.Helper()
	certs, rawData, err := setupsvc.Loadpkcs7TrustBundle(p7bFile)
	if err != nil {
		t.Fatalf("STATUS:Failed to load trust bundle %s: %v", p7bFile, err)
	}
	pool := x509.NewCertPool()
	for _, c := range certs {
		pool.AddCert(c)
	}
	return pool, rawData
}

// mustBuildMergedP7b concatenates two PEM files and converts the result to a DER-encoded
// PKCS#7 trust bundle using openssl. A temporary p7b file is written to the OS temp
// directory; the caller is responsible for removing it via the returned cleanup func.
func mustBuildMergedP7b(t *testing.T, ca01PEMFile, ca02PEMFile string) (string, func()) {
	t.Helper()

	mergedPEMData := mustConcatenatePEMFiles(t, ca01PEMFile, ca02PEMFile)
	tmpPEM, err := os.CreateTemp("", "merged_trust_bundle_*.pem")
	if err != nil {
		t.Fatalf("STATUS:Failed to create temp merged PEM file: %v", err)
	}
	if _, err := tmpPEM.Write(mergedPEMData); err != nil {
		tmpPEM.Close()
		os.Remove(tmpPEM.Name())
		t.Fatalf("STATUS:Failed to write merged PEM data: %v", err)
	}
	tmpPEM.Close()

	// Convert merged PEM → DER-encoded PKCS#7 (.p7b) via openssl.
	p7bPath := tmpPEM.Name() + ".p7b"
	cmd := exec.Command("openssl", "crl2pkcs7", "-nocrl",
		"-certfile", tmpPEM.Name(),
		"-out", p7bPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(tmpPEM.Name())
		t.Fatalf("STATUS:Failed to generate merged p7b via openssl: %v\n%s", err, out)
	}

	cleanup := func() {
		os.Remove(tmpPEM.Name())
		os.Remove(p7bPath)
	}
	return p7bPath, cleanup
}

// createPatchedScript creates a patched copy of a script inside dirPath
func createPatchedScript(scriptName string, dirs []string) (string, string, error) {
	scriptPath := filepath.Join(dirPath, scriptName)

	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", "", fmt.Errorf("read script: %w", err)
	}

	content := string(data)
	dirsJoined := strings.Join(dirs, " ")

	reDirs := regexp.MustCompile(`DIRS\s*=\s*\([^\)]*\)`)
	content = reDirs.ReplaceAllString(content, fmt.Sprintf("DIRS=(%s)", dirsJoined))

	reFor := regexp.MustCompile(`for\s+d\s+in\s+[^;]*;`)
	content = reFor.ReplaceAllString(content, fmt.Sprintf("for d in %s;", dirsJoined))

	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return "", "", err
	}

	tmpFile, err := os.CreateTemp(absDir, "patched_*.sh")
	if err != nil {
		return "", "", err
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		return "", "", err
	}
	tmpFile.Close()

	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return "", "", err
	}

	scriptCmd := "./" + filepath.Base(tmpFile.Name())
	fullPath := tmpFile.Name()

	return scriptCmd, fullPath, nil
}

// mustRunScript patches a script to limit CA dirs, executes it, and returns the temp script path.
func mustRunScript(t *testing.T, script string) string {
	t.Helper()

	scriptCmd, fullPath, err := createPatchedScript(script, []string{"01", "02"})
	if err != nil {
		t.Fatalf("STATUS:Failed to create patched script: %v", err)
	}

	if err := setupsvc.TestdataMakeCleanup(t, dirPath, scriptWaitTime, scriptCmd); err != nil {
		t.Fatalf("STATUS:Failed to execute patched script: %v", err)
	}

	return fullPath
}

// mustLoadTLSMaterial loads a client certificate and corresponding CA pool from given files.
func mustLoadTLSMaterial(t *testing.T, certFile, keyFile, bundleFile string) (*tls.Certificate, *x509.CertPool) {
	t.Helper()

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("STATUS:Failed to load cert/key: %v", err)
	}

	pool, _ := mustBuildCertPool(t, bundleFile)
	return &cert, pool
}

// mustValidateServices verifies service connectivity and TLS validation using provided cert and CA pool.
func mustValidateServices(t *testing.T, pool *x509.CertPool, cert tls.Certificate, cfg rotationRunConfig) {
	t.Helper()

	if result := setupsvc.ServicesValidationCheck(
		t, pool, expectedResult,
		cfg.serverSAN, serverAddr, cfg.username, cfg.password,
		cert, cfg.tc.mismatch,
	); !result {
		t.Fatalf("STATUS:%s:Failed to validate services", cfg.tc.desc)
	}
}

// mustRunPositiveRotation performs a positive trust bundle rotation test:
// load certs, optionally reconnect, rotate with merged bundle, validate services, and archive artifacts.
func mustRunPositiveRotation(t *testing.T, cfg rotationRunConfig) {
	t.Helper()
	tc := cfg.tc

	t.Logf("Starting positive test case: %s", tc.desc)

	newCert, _ := mustLoadTLSMaterial(t, tc.clientCertFile, tc.clientKeyFile, tc.trustBundleFile)

	// Reconnect if needed
	if tc.newTLScreds {
		prevCert, prevPool := mustLoadTLSMaterial(t, prevClientCertFile, prevClientKeyFile, prevTrustBundleFile)
		mustValidateServices(t, prevPool, *prevCert, cfg)

		conn := setupsvc.CreateNewDialOption(
			t, *prevCert, prevPool,
			cfg.serverSAN, cfg.username, cfg.password, serverAddr,
		)
		defer conn.Close()

		cfg.certzClient = certzpb.NewCertzClient(conn)
		cfg.gnmiClient = gnmi.NewGNMIClient(conn)
	}

	// Build merged bundle
	mergedP7b, cleanup := mustBuildMergedP7b(t, tc.ca01PEMFile, tc.ca02PEMFile)
	defer cleanup()

	mergedPool, mergedData := mustBuildCertPool(t, mergedP7b)

	tbEntity := setupsvc.CreateCertzEntity(
		t, setupsvc.EntityTypeTrustBundle, string(mergedData), tc.bversion,
	)

	// Rotate
	if !setupsvc.CertzRotate(
		cfg.ctx, t, mergedPool,
		cfg.certzClient, cfg.gnmiClient,
		*newCert, cfg.dut,
		cfg.username, cfg.password,
		cfg.serverSAN, serverAddr, testProfile,
		tc.newTLScreds, tc.mismatch, tc.scale,
		cfg.serverCertEntity, &tbEntity,
	) {
		t.Fatalf("STATUS:%s:Failed to rotate trust bundle", tc.desc)
	}

	// Validate
	t.Run("PostRotationValidation", func(t *testing.T) {
		mustValidateServices(t, mergedPool, *newCert, cfg)
	})

	// Save state
	prevClientCertFile = tc.clientCertFile
	prevClientKeyFile = tc.clientKeyFile
	prevTrustBundleFile = tc.trustBundleFile
}

// mustRunNegativeRotation performs a negative rotation test:
// connect with prior creds, attempt rotation with mismatched bundle, expect rejection, and verify original cert remains active.
func mustRunNegativeRotation(t *testing.T, cfg rotationRunConfig) {
	t.Helper()
	tc := cfg.tc

	t.Logf("Starting negative test case: %s", tc.desc)

	prevCert, prevPool := mustLoadTLSMaterial(t, prevClientCertFile, prevClientKeyFile, prevTrustBundleFile)

	mustValidateServices(t, prevPool, *prevCert, cfg)

	// Build mismatch bundle
	_, raw, err := setupsvc.Loadpkcs7TrustBundle(tc.mismatchTBFile)
	if err != nil {
		t.Fatalf("STATUS:Failed to load mismatch bundle: %v", err)
	}

	tbEntity := setupsvc.CreateCertzEntity(
		t, setupsvc.EntityTypeTrustBundle, string(raw), tc.bversion,
	)

	mismatchPool, _ := mustBuildCertPool(t, tc.mismatchTBFile)

	conn := setupsvc.CreateNewDialOption(
		t, *prevCert, prevPool,
		cfg.serverSAN, cfg.username, cfg.password, serverAddr,
	)
	defer conn.Close()

	if setupsvc.CertzRotate(
		cfg.ctx, t, mismatchPool,
		certzpb.NewCertzClient(conn),
		gnmi.NewGNMIClient(conn),
		*prevCert, cfg.dut,
		cfg.username, cfg.password,
		cfg.serverSAN, serverAddr, testProfile,
		tc.newTLScreds, tc.mismatch, tc.scale,
		cfg.serverCertEntity, &tbEntity,
	) {
		t.Fatalf("STATUS:%s:Rotation unexpectedly succeeded", tc.desc)
	}

	t.Run("PostMismatchValidation", func(t *testing.T) {
		recoveredPool, _ := mustBuildCertPool(t, tc.trustBundleFile)
		mustValidateServices(t, recoveredPool, *prevCert, cfg)
	})
}

// TestTrustBundleRotation runs Certz-5.1 (positive) and Certz-5.2 (negative) sub-tests
// for RSA and ECDSA key types in a single table-driven loop.
func TestTrustBundleRotation(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	serverAddr = dut.Name()

	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &dutCreds); err != nil {
		t.Fatalf("STATUS:Failed to get DUT credentials: %v. The binding for %s must implement the DUTCredentialer interface.", err, dut.Name())
	}
	username := dutCreds.RPCUsername()
	password := dutCreds.RPCPassword()

	t.Logf("STATUS:Pre-init service validation before certz rotation.")
	gnmiClient, gnsiC := setupsvc.PreInitCheck(context.Background(), t, dut)

	// Generate test certificate data.
	t.Logf("STATUS:Generating test data certificates.")
	patchedMkCas := mustRunScript(t, "mk_cas.sh")
	defer os.Remove(patchedMkCas)

	ctx := context.Background()
	certzClient := gnsiC.Certz()

	// Verify testProfile does not pre-exist.
	t.Logf("STATUS:Checking baseline SSL profile list.")
	if getResp := setupsvc.GetSslProfilelist(
		ctx, t, certzClient, &certzpb.GetProfileListRequest{},
	); slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Fatalf("STATUS:profileID %s already exists, cannot proceed.", testProfile)
	}

	// Add the new SSL profile.
	t.Logf("STATUS:Adding new SSL profile ID %s.", testProfile)
	if addResp, err := certzClient.AddProfile(
		ctx, &certzpb.AddProfileRequest{SslProfileId: testProfile},
	); err != nil {
		t.Fatalf("STATUS:AddProfile request failed: %v", err)
	} else {
		t.Logf("STATUS:Received AddProfileResponse: %v", addResp)
	}

	// Confirm the new profile appears in the list.
	if getResp := setupsvc.GetSslProfilelist(
		ctx, t, certzClient, &certzpb.GetProfileListRequest{},
	); !slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Fatalf("STATUS:Newly added profileID %s is not present in SSL profile list.",
			testProfile)
	}
	t.Logf("STATUS:New profileID %s confirmed in SSL profile list.", testProfile)

	testCases := []rotationTestCase{
		// ── Certz-5.1: Positive rotation ─────────────────────────────────────────
		{
			desc:             "Certz5.1:Positive rotation with RSA key type",
			positiveRotation: true,
			serverCertFile:   dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:    dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile:  dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			ca01PEMFile:      dirPath + "ca-01/trust_bundle_01_rsa.pem",
			ca02PEMFile:      dirPath + "ca-02/trust_bundle_02_rsa.pem",
			clientCertFile:   dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:    dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:         "v1",
			bversion:         "bundle1",
		},
		{
			desc:             "Certz5.1:Positive rotation with ECDSA key type",
			positiveRotation: true,
			serverCertFile:   dirPath + "ca-01/server-ecdsa-a-cert.pem",
			serverKeyFile:    dirPath + "ca-01/server-ecdsa-a-key.pem",
			trustBundleFile:  dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			ca01PEMFile:      dirPath + "ca-01/trust_bundle_01_ecdsa.pem",
			ca02PEMFile:      dirPath + "ca-02/trust_bundle_02_ecdsa.pem",
			clientCertFile:   dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:    dirPath + "ca-01/client-ecdsa-a-key.pem",
			cversion:         "v2",
			bversion:         "bundle2",
			newTLScreds:      true,
		},
		// ── Certz-5.2: Negative rotation ─────────────────────────────────────────
		// Negative cases run after all positive cases. Each one reuses
		// prevClientCertFile/prevClientKeyFile/prevTrustBundleFile archived by the
		// last positive iteration to establish a valid connection before attempting
		// the mismatched rotation.
		{
			desc:             "Certz5.2:Negative rotation with RSA key type mismatched CA trust bundle",
			positiveRotation: false,
			serverCertFile:   dirPath + "ca-01/server-rsa-a-cert.pem",
			serverKeyFile:    dirPath + "ca-01/server-rsa-a-key.pem",
			trustBundleFile:  dirPath + "ca-01/trust_bundle_01_rsa.p7b",
			mismatchTBFile:   dirPath + "ca-02/trust_bundle_02_rsa.p7b",
			clientCertFile:   dirPath + "ca-01/client-rsa-a-cert.pem",
			clientKeyFile:    dirPath + "ca-01/client-rsa-a-key.pem",
			cversion:         "v3",
			bversion:         "bundle3",
		},
		{
			desc:             "Certz5.2:Negative rotation with ECDSA key type mismatched CA trust bundle",
			positiveRotation: false,
			serverCertFile:   dirPath + "ca-01/server-ecdsa-a-cert.pem",
			serverKeyFile:    dirPath + "ca-01/server-ecdsa-a-key.pem",
			trustBundleFile:  dirPath + "ca-01/trust_bundle_01_ecdsa.p7b",
			mismatchTBFile:   dirPath + "ca-02/trust_bundle_02_ecdsa.p7b",
			clientCertFile:   dirPath + "ca-01/client-ecdsa-a-cert.pem",
			clientKeyFile:    dirPath + "ca-01/client-ecdsa-a-key.pem",
			cversion:         "v4",
			bversion:         "bundle4",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Common setup: read the server SAN and build the server cert entity,
			// which is used identically by both positive and negative rotation paths.
			serverSAN := setupsvc.ReadDecodeServerCertificate(t, tc.serverCertFile)
			serverCert := setupsvc.CreateCertzChain(t, setupsvc.CertificateChainRequest{
				RequestType:    setupsvc.EntityTypeCertificateChain,
				ServerCertFile: tc.serverCertFile,
				ServerKeyFile:  tc.serverKeyFile,
			})
			serverCertEntity := setupsvc.CreateCertzEntity(
				t, setupsvc.EntityTypeCertificateChain, &serverCert, tc.cversion,
			)

			if tc.positiveRotation {
				mustRunPositiveRotation(t, rotationRunConfig{
					tc:               tc,
					ctx:              ctx,
					dut:              dut,
					certzClient:      certzClient,
					gnmiClient:       gnmiClient,
					serverSAN:        serverSAN,
					username:         username,
					password:         password,
					serverCertEntity: &serverCertEntity,
				})
			} else {
				mustRunNegativeRotation(t, rotationRunConfig{
					tc:               tc,
					ctx:              ctx,
					dut:              dut,
					serverSAN:        serverSAN,
					username:         username,
					password:         password,
					serverCertEntity: &serverCertEntity,
				})
			}
		})
	}

	// Final cleanup of generated test data.
	t.Logf("STATUS:Cleaning up test data.")
	patchedCleanup := mustRunScript(t, "cleanup.sh")
	defer os.Remove(patchedCleanup)

	t.Logf("STATUS:TestTrustBundleRotation completed!")
}
