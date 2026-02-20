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

// Package setupservice is scoped only to be used for scripts in path
// feature/security/gnsi/certz/tests/client_certificates
// Do not use elsewhere.
package setupservice

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"go.mozilla.org/pkcs7"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	certzpb "github.com/openconfig/gnsi/certz"
	gribipb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	ognmi "github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/knebind/creds"
	p4rtpb "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

var (
	serverName = "role001.pop55.net.example.com"
	servers    []string
	retries    int = 12
	wg         sync.WaitGroup
	success    bool
)

type rpcCredentials struct {
	*creds.UserPass
}

func (r *rpcCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"username": r.UserPass.Username,
		"password": r.UserPass.Password,
	}, nil
}

func (r *rpcCredentials) RequireTransportSecurity() bool {
	return true
}

// DUTCredentialer is an interface for getting credentials from a DUT binding.
type DUTCredentialer interface {
	RPCUsername() string
	RPCPassword() string
}

type entityType int8

const (
	// EntityTypeCertificateChain is type of entity of the certificate chain.
	EntityTypeCertificateChain entityType = 1
	// EntityTypeTrustBundle is type of entity of the trust bundle.
	EntityTypeTrustBundle entityType = 2
	// EntityTypeCRL is type of entity of the CRL.
	EntityTypeCRL entityType = 3
	// EntityTypeAuthPolicy is type of entity of the auth policy.
	EntityTypeAuthPolicy entityType = 4
)

// CertificateChainRequest is an input argument for the  type definition for the CreateCertzChain.
type CertificateChainRequest struct {
	RequestType     entityType
	ServerCertFile  string
	ServerKeyFile   string
	TrustBundleFile string
}

// CreateDialOptions function to create the gRPC dial options for certz client and retruns connection handle.
func CreateNewDialOption(t *testing.T, newClientCert tls.Certificate, newCaCert *x509.CertPool, san, username, password, serverAddr string) (conn *grpc.ClientConn) {
	credOpts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{
			Certificates: []tls.Certificate{newClientCert},
			RootCAs:      newCaCert,
			ServerName:   san,
		},
	))}
	creds := &rpcCredentials{&creds.UserPass{Username: username, Password: password}}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))
	target := fmt.Sprintf("%s:%d", serverAddr, 9339)
	conn, err := grpc.NewClient(target, credOpts...)
	if err != nil {
		t.Fatalf("%s STATUS: gRPC NewClient failed to %q with err %v", time.Now().String(), target, err)
	}
	return conn
}

// CreateCertzEntity function to create certificate entity of type certificate chain/trust bundle/CRL/Authpolicy.
func CreateCertzEntity(t *testing.T, typeOfEntity entityType, entityContent any, entityVersion string) certzpb.Entity {
	createdOnTime := time.Now()
	varClock := uint64(createdOnTime.Unix())

	switch typeOfEntity {
	case EntityTypeCertificateChain:

		return certzpb.Entity{
			Version:   entityVersion,
			CreatedOn: varClock,
			Entity:    &certzpb.Entity_CertificateChain{CertificateChain: entityContent.(*certzpb.CertificateChain)},
		}

	case EntityTypeTrustBundle:

		return certzpb.Entity{
			Version:   entityVersion,
			CreatedOn: varClock,
			Entity:    &certzpb.Entity_TrustBundlePkcs7{TrustBundlePkcs7: &certzpb.TrustBundle{Pkcs7Block: entityContent.(string)}},
		}

	case EntityTypeCRL:

		return certzpb.Entity{
			Version:   entityVersion,
			CreatedOn: varClock,
			Entity:    &certzpb.Entity_CertificateRevocationListBundle{CertificateRevocationListBundle: entityContent.(*certzpb.CertificateRevocationListBundle)},
		}

	case EntityTypeAuthPolicy:

		return certzpb.Entity{
			Version:   entityVersion,
			CreatedOn: varClock,
			Entity:    &certzpb.Entity_AuthenticationPolicy{AuthenticationPolicy: entityContent.(*certzpb.AuthenticationPolicy)},
		}

	default:
		t.Fatalf("Invalid entity type %v", typeOfEntity)
	}
	return certzpb.Entity{}
}

// CreateCertzChain function to get the certificate chain of type certificate chain/trust bundle.
func CreateCertzChain(t *testing.T, certData CertificateChainRequest) certzpb.CertificateChain {
	switch certData.RequestType {
	case EntityTypeCertificateChain:
		if len(certData.ServerCertFile) == 0 {
			t.Fatalf("Missing server certificate file for creating certificate chain object.")
		}
		serverCertContent, err := os.ReadFile(certData.ServerCertFile)
		if err != nil {
			t.Fatalf("Error reading Server Certificate file at: %v with error: %v", certData.ServerCertFile, err)

		}
		if len(certData.ServerKeyFile) != 0 {
			serverKeyContent, err := os.ReadFile(certData.ServerKeyFile)
			if err != nil {
				t.Fatalf("Error reading Server Key file at: %v with error: %v", certData.ServerKeyFile, err)
			}
			return certzpb.CertificateChain{Certificate: &certzpb.Certificate{
				Type:            certzpb.CertificateType_CERTIFICATE_TYPE_X509,
				Encoding:        certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
				CertificateType: &certzpb.Certificate_RawCertificate{RawCertificate: serverCertContent},
				PrivateKeyType:  &certzpb.Certificate_RawPrivateKey{RawPrivateKey: serverKeyContent},
			}, Parent: nil}
		}
		return certzpb.CertificateChain{Certificate: &certzpb.Certificate{
			Type:            certzpb.CertificateType_CERTIFICATE_TYPE_X509,
			Encoding:        certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
			PrivateKeyType:  nil,
			CertificateType: &certzpb.Certificate_RawCertificate{RawCertificate: serverCertContent},
		}, Parent: nil}

	case EntityTypeTrustBundle:
		if len(certData.TrustBundleFile) == 0 {
			t.Fatalf("Missing trust bundle file for creating certificate chain object.")
		}
		trustBundleContent, err := os.ReadFile(certData.TrustBundleFile)
		if err != nil {
			t.Fatalf("Error reading trust bundle file at: %v with error: %v", certData.TrustBundleFile, err)
		}
		return certzpb.CertificateChain{Certificate: &certzpb.Certificate{
			Type:            certzpb.CertificateType_CERTIFICATE_TYPE_X509,
			Encoding:        certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
			Certificate:     trustBundleContent,
			CertificateType: &certzpb.Certificate_RawCertificate{RawCertificate: trustBundleContent},
		}, Parent: nil}

	default:
		t.Fatalf("Invalid request type received.")
	}
	return certzpb.CertificateChain{}
}

// LoadTrustBundle reads a file that contains a PKCS#7 trust‑bundle.
func Loadpkcs7TrustBundle(path string) ([]*x509.Certificate, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, nil, fmt.Errorf("decoding PEM block from %s: %w", path, err)
	}
	p7, err := pkcs7.Parse(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing PKCS#7: %w", err)
	}
	return p7.Certificates, data, nil
}

// CertzRotate function to request the server certificate rotation and returns true on successful rotation.
func CertzRotate(ctx context.Context, t *testing.T, newcaCert *x509.CertPool, certzClient certzpb.CertzClient, gnmiClient gnmipb.GNMIClient, newclientCert tls.Certificate, dut *ondatra.DUTDevice, username string, password string, san, serverAddr, profileID string, newTLS bool, mismatch bool, scale bool, entities ...*certzpb.Entity) bool {

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
		t.Fatalf("Error sending rotate request: %v", err)
	}
	t.Logf("Sent Rotate certificate request.")
	//RotateRequest receive channel to receive the next response message.
	ch := make(chan struct{}, 1)
	rotateResponse := &certzpb.RotateCertificateResponse{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		rotateResponse, err = rotateRequestClient.Recv()
		if err != nil {
			t.Errorf("Error fetching rotate certificate response: %v", err)
			return
		}
		close(ch)
	}()
	for i := range retries {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				select {
				case <-ch:
					//Exiting receive channel.
					return
				default:
					//Did not receive response after sending rotate request.Sleeping 10s to retry.
					time.Sleep(10 * time.Second)
				}
			}
		}(i)
	}
	wg.Wait()
	t.Logf("Received Rotate certificate response: %v", rotateResponse)
	if !newTLS {
		// Replace config with newly added ssl profile during successful rotate.
		servers = ognmi.GetAll(t, dut.GNMIOpts().WithClient(gnmiClient), ognmi.OC().System().GrpcServerAny().Name().State())
		batch := ognmi.SetBatch{}
		for _, server := range servers {
			ognmi.BatchReplace(&batch, ognmi.OC().System().GrpcServer(server).CertificateId().Config(), profileID)
		}
		batch.Set(t, dut.GNMIOpts().WithClient(gnmiClient))
		t.Logf("gNMI config is replaced with new ssl profile %s successfully.", profileID)
		time.Sleep(30 * time.Second) //waiting 30s for gnmi config propagation//
	}
	//Verify gNSI service with new TLS credentials in loop with retries before finalize.
	if success = VerifyGnsi(t, newcaCert, san, serverAddr, username, password, newclientCert, mismatch); !success {
		t.Fatalf("gNSI service RPC  did not succeed after rotate. Certz/Rotate failed. FinalizeRequest will not be sent")
	}
	//Finalize the rotation after successful gNSI service verification.
	finalizeRequest := &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}}
	rotateCertRequest = &certzpb.RotateCertificateRequest{
		ForceOverwrite: false,
		SslProfileId:   profileID,
		RotateRequest:  finalizeRequest,
	}
	if err = rotateRequestClient.Send(rotateCertRequest); err != nil {
		t.Fatalf("Error sending rotate finalize request: %v", err)
	}
	if err = rotateRequestClient.CloseSend(); err != nil {
		t.Fatalf("Error sending rotate close send request: %v", err)
	}
	return true
}

// TestdataMakeCleanup function to create/cleanup test data for use in TLS tests.
// This function executes the certificate generate/cleanup script "mk_cas.sh" and "cleanup.sh"
// located in the specified dir at the start and end of the tests repectively.
func TestdataMakeCleanup(t *testing.T, dirPath string, timeout time.Duration, args string) error {

	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, args)
	cmd.Dir = dirPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If the context timed‑out, the error is reported specifically.
		if ctx.Err() == context.DeadlineExceeded {
			t.Errorf("STATUS:Script got timed out after %s\nCommand output:\n%s", timeout, string(out))
		}
		t.Errorf("STATUS:Script failed: %v\nCommand output:\n%s", err, string(out))
	}
	t.Logf("STATUS: Script execution succeeded (output size: %d bytes)", len(out))
	return nil
}

// ReadDecodeServerCertificate function to read and decode server certificates to extract the SubjectAltName and validate.
// ReadDecodeServerCertificate reads a PEM-encoded server certificate from the specified file,
// decodes it, parses the x509 certificate, and returns the first DNS Subject Alternative Name (SAN).
// It fails the test if the file does not exist, cannot be read, cannot be decoded, or the certificate
// cannot be parsed. It also validates the SAN against an expected value and logs the SAN.
// Parameters:
//
//	t - the testing context.
//	serverCertzFile - path to the PEM-encoded certificate file.
//
// Returns:
//
//	san - the first DNS Subject Alternative Name from the certificate.
func ReadDecodeServerCertificate(t *testing.T, serverCertzFile string) (san string) {

	if _, err := os.Stat(serverCertzFile); os.IsNotExist(err) {
		t.Fatalf("Certificate file does not exist: %v", serverCertzFile)
	}
	sc, err := os.ReadFile(serverCertzFile)
	if err != nil {
		t.Fatalf("Failed to read certificate with error: %v.", err)
	}
	block, _ := pem.Decode(sc)
	if block == nil {
		t.Fatalf("Failed to parse PEM block containing the public key.")
	}
	sCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate with error: %v.", err)
	}
	san = sCert.DNSNames[0]
	t.Logf("ServerAltName:%s.", san)
	if serverName != san {
		t.Fatalf("ServerAltName validation failed for %s.", serverCertzFile)
	}
	return san
}

// VerifyGnsi function to validate the gNSI service RPC after successful rotation.
// VerifyGnsi establishes a gRPC connection to a gNSI server using TLS and user credentials,
// then performs an authorization check via the Authz service. It verifies the connection
// and response based on expected error scenarios, such as certificate mismatch or failed precondition.
//
// Parameters:
//
//	t         - The testing context.
//	caCert    - The certificate pool containing trusted CA certificates.
//	san       - The expected server name for TLS verification.
//	serverAddr- The address of the gNSI server.
//	username  - The username for authentication.
//	password  - The password for authentication.
//	cert      - The client TLS certificate.
//	mismatch  - Indicates if a certificate mismatch scenario is expected.
//
// Returns:
//
//	bool - True if the connection and authorization check succeed or expected errors are observed; false otherwise.
func VerifyGnsi(t *testing.T, caCert *x509.CertPool, san, serverAddr, username, password string, cert tls.Certificate, mismatch bool) bool {
	credOpts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCert,
			ServerName:   san,
		},
	))}
	creds := &rpcCredentials{&creds.UserPass{Username: username, Password: password}}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))
	target := fmt.Sprintf("%s:%d", serverAddr, 9339)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(target, credOpts...)
	if err != nil {
		t.Errorf("%sVerifyGnsi:gRPC NewClient failed to %q with err %v", time.Now().String(), target, err)
		return false
	}
	t.Logf("Connection state: %v.", conn.GetState().String())
	defer conn.Close()
	authzClient := authzpb.NewAuthzClient(conn)
	//AuthzGetRequest
	result := ValidateGnsiAuthzGetRequest(ctx, t, authzClient, mismatch)
	conn.Close()
	return result
}

// ValidateGnsiAuthzGetRequest function to verify get request with authz client.
func ValidateGnsiAuthzGetRequest(ctx context.Context, t *testing.T, authzClient authzpb.AuthzClient, mismatch bool) bool {

	t.Logf("Verifying gNSI Authz GetRequest.")
	rsp, err := authzClient.Get(ctx, &authzpb.GetRequest{})
	if err != nil {
		statusError, _ := status.FromError(err)
		if statusError.Code() == codes.FailedPrecondition {
			t.Logf("Expected error FAILED_PRECONDITION seen for authz Get Request with err:%v.", err)
		} else {
			t.Errorf("Unexpected error during authz Get Request with err:%v.", err)
		}
	}
	t.Logf("gNSI authz get response is %s", rsp)
	return true

}

// VerifyGnoi function to validate the gNOI service RPC after successful rotation.
// VerifyGnoi attempts to establish a gRPC connection to a gNOI server using the provided TLS certificate,
// CA certificate pool, server address, SAN, and user credentials. It sends a Ping request to verify connectivity.
// If 'mismatch' is true, it expects the connection to fail due to certificate mismatch and logs the error;
// otherwise, it fails the test on connection errors. Returns true if the connection and Ping succeed or if
// a mismatch is expected and occurs.
//
// Parameters:
//
//	t         - The testing context.
//	caCert    - The CA certificate pool for TLS verification.
//	san       - The expected server name (SAN) for TLS verification.
//	serverAddr- The address of the gNOI server.
//	username  - The username for authentication.
//	password  - The password for authentication.
//	cert      - The client TLS certificate.
//	mismatch  - Whether a certificate mismatch is expected.
//
// Returns:
//
//	bool - True if verification succeeds or expected mismatch occurs, false otherwise.
func VerifyGnoi(t *testing.T, caCert *x509.CertPool, san, serverAddr, username, password string, cert tls.Certificate, mismatch bool) bool {

	credOpts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCert,
			ServerName:   san,
		},
	))}
	creds := &rpcCredentials{&creds.UserPass{Username: username, Password: password}}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))
	target := fmt.Sprintf("%s:%d", serverAddr, 9339)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(target, credOpts...)
	if err != nil {
		t.Errorf("VerifyGnoi:gRPC NewClient failed to %q with err %v", target, err)
		return false
	}
	defer conn.Close()
	sysClient := spb.NewSystemClient(conn)
	result := ValidateGnoiPingRequest(ctx, t, sysClient, false)
	conn.Close()
	return result
}

// ValidateGnoiPingRequest function verifies ping request with the gnoiclient.
func ValidateGnoiPingRequest(ctx context.Context, t *testing.T, sysClient spb.SystemClient, mismatch bool) bool {

	t.Logf("Verifying gNOI Ping Request.")
	if _, err := sysClient.Ping(ctx, &spb.PingRequest{}); err != nil {
		t.Fatalf("Unable to connect gnoiClient %v", err)
	}
	return true
}

// VerifyGnmi function to validate the gNMI service RPC after successful rotation.
// VerifyGnmi establishes a gNMI client connection to a target server using TLS and user credentials,
// sends a Capabilities request, and verifies the response. It supports testing certificate mismatches.
// Returns true if the connection and request succeed, or false if the connection fails as expected when
// mismatch is true. Logs detailed information about the connection and request process.
//
// Parameters:
//
//	t         - The testing context for logging and error reporting.
//	caCert    - The certificate pool containing trusted CA certificates.
//	san       - The expected server name for TLS verification.
//	serverAddr- The address of the gNMI server.
//	username  - The username for authentication.
//	password  - The password for authentication.
//	cert      - The client TLS certificate.
//	mismatch  - If true, expects the connection to fail due to certificate mismatch.
//
// Returns:
//
//	bool - True if the connection and Capabilities request succeed, false otherwise.
func VerifyGnmi(t *testing.T, caCert *x509.CertPool, san, serverAddr, username, password string, cert tls.Certificate, mismatch bool) bool {

	credOpts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCert,
			ServerName:   san,
		},
	))}
	creds := &rpcCredentials{&creds.UserPass{Username: username, Password: password}}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))
	target := fmt.Sprintf("%s:%d", serverAddr, 9339)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(target, credOpts...)
	if err != nil {
		t.Errorf("VerifyGnmi:gRPC NewClient failed to %q with err %v", target, err)
		return false
	}
	defer conn.Close()
	gnmiClient := gnmipb.NewGNMIClient(conn)
	//CapabilityRequest with GnmiClient.
	result := ValidateGnmiCapabilityRequest(ctx, t, gnmiClient, mismatch)
	conn.Close()
	return result
}

// ValidateGnmiCapabilityRequest function validates the gNMI RPC request.
func ValidateGnmiCapabilityRequest(ctx context.Context, t *testing.T, gnmiClient gnmipb.GNMIClient, mismatch bool) bool {

	t.Logf("Verifying gNMI Capability Request.")
	response, err := gnmiClient.Capabilities(ctx, &gnmipb.CapabilityRequest{})
	if err != nil {
		t.Errorf("gNMI Capability request failed with err: %v", err)
	}
	t.Logf("VerifyGnmi:gNMI response: %s", response.GNMIVersion)
	return true
}

// VerifyGribi function to validate the gRIBI service RPC after successful rotation.
// VerifyGribi attempts to establish a gRPC connection to a gRIBI server using the provided
// TLS certificate, CA certificate pool, and user credentials. It verifies the connection
// by sending a GetRequest to the server. If the 'mismatch' flag is true, the function expects
// the connection to fail due to certificate mismatch and logs the error; otherwise, it fails
// the test on connection errors. Returns true if the connection and request succeed, false otherwise.
// Parameters:
//
//	t         - The testing context.
//	caCert    - The CA certificate pool for verifying the server's certificate.
//	san       - The expected server name for TLS verification.
//	serverAddr- The address of the gRIBI server.
//	username  - The username for authentication.
//	password  - The password for authentication.
//	cert      - The client TLS certificate.
//	mismatch  - Whether to expect a certificate mismatch (connection failure).
//
// Returns:
//
//	bool      - True if connection and request succeed, false otherwise.
func VerifyGribi(t *testing.T, caCert *x509.CertPool, san, serverAddr, username, password string, cert tls.Certificate, mismatch bool) bool {

	credOpts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCert,
			ServerName:   san,
		},
	))}
	creds := &rpcCredentials{&creds.UserPass{Username: username, Password: password}}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))
	target := fmt.Sprintf("%s:%d", serverAddr, 9340)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(target, credOpts...)
	if err != nil {
		t.Errorf("VerifyGribi: gRPC NewClient failed to %q with error:%v", target, err)
		return false
	}
	defer conn.Close()
	gRibiClient := gribipb.NewGRIBIClient(conn)
	//gRibi get request validation.
	result := ValidateGribiGetRequest(ctx, t, gRibiClient, mismatch)
	conn.Close()
	return result
}

// ValidateGribiGetRequest function verifies get request RPC with the gNMIClient.
func ValidateGribiGetRequest(ctx context.Context, t *testing.T, gRibiClient gribipb.GRIBIClient, mismatch bool) bool {

	t.Logf("Verifying gRIBI GetRequest.")
	_, err := gRibiClient.Get(ctx, &gribipb.GetRequest{})
	if err != nil {
		t.Fatalf("Failed to connect GribiClient with error:%v.", err)
	}
	return true
}

// VerifyP4rt function to validate the P4rt service RPC after successful rotation.
// VerifyP4rt establishes a gRPC connection to a P4Runtime server using TLS and user credentials,
// verifies server capabilities, and returns true if the connection and capability check succeed.
// It reports errors and failures using the provided testing.T instance.
//
// Parameters:
//
//	t         - The testing.T instance for reporting errors and failures.
//	caCert    - The x509.CertPool containing trusted CA certificates.
//	san       - The expected server name (SAN) for TLS verification.
//	serverAddr- The address of the P4Runtime server.
//	username  - The username for authentication.
//	password  - The password for authentication.
//	cert      - The client TLS certificate.
//
// Returns:
//
//	bool - true if the connection and capability check succeed; false otherwise.
func VerifyP4rt(t *testing.T, caCert *x509.CertPool, san, serverAddr, username, password string, cert tls.Certificate, mismatch bool) bool {

	credOpts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCert,
			ServerName:   san,
		},
	))}
	creds := &rpcCredentials{&creds.UserPass{Username: username, Password: password}}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))
	target := fmt.Sprintf("%s:%d", serverAddr, 9559)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(target, credOpts...)
	if err != nil {
		t.Errorf("VerifyP4rt:gRPC NewClient failed to %q with error %v.", target, err)
	}
	defer conn.Close()
	p4RtClient := p4rtpb.NewP4RuntimeClient(conn)
	//Capability Request with p4RtClient
	result := ValidateP4RtCapabilitiesRequest(ctx, t, p4RtClient, mismatch)
	conn.Close()
	return result
}

// ValidateP4RtCapabilitiesRequest function verifies the Capabilities request with the p4RT client.
func ValidateP4RtCapabilitiesRequest(ctx context.Context, t *testing.T, p4rtClient p4rtpb.P4RuntimeClient, mismatch bool) bool {

	t.Logf("Verifying P4Rt Capability Request.")
	if _, err := p4rtClient.Capabilities(ctx, &p4rtpb.CapabilitiesRequest{}); err != nil {
		t.Errorf("Failed to connect P4rtClient with error %v.", err)
	}
	return true
}

// PreInitCheck function to dial gNMI/gNOI/gRIBI/p4RT/gNSI services before certz rotation.
func PreInitCheck(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) (gnmipb.GNMIClient, binding.GNSIClients) {

	mismatch := false // mismatch set to false
	//gNMI service validation in the preinit check.
	gnmiC, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("PreInitcheck:Failed to dial gNMI Connection with error: %v.", err)
	}
	if result := ValidateGnmiCapabilityRequest(ctx, t, gnmiC, mismatch); !result {
		t.Fatalf("PreInitcheck:Failed in gNMI capability request.")
	}
	t.Logf("PreInitcheck:gNMI dial %s is successful.", gnmiC)
	//gRIBI service validation in the preinit check.
	gribiC, err := dut.RawAPIs().BindingDUT().DialGRIBI(ctx)
	if err != nil {
		t.Fatalf("PreInitcheck:Failed to dial gRIBI Connection with error: %v.", err)
	}
	if result := ValidateGribiGetRequest(ctx, t, gribiC, mismatch); !result {
		t.Fatalf("PreInitcheck:Failed to validate get request with GribiClient.")
	}
	t.Logf("PreInitcheck:gRIBI dial %s is successful.", gribiC)
	//gNOI service validation in the preinit check.
	gnoiC, err := dut.RawAPIs().BindingDUT().DialGNOI(ctx)
	if err != nil {
		t.Fatalf("PreInitcheck:Failed to dial gNOI Connection with error: %v.", err)
	}
	if result := ValidateGnoiPingRequest(ctx, t, gnoiC.System(), mismatch); !result {
		t.Fatalf("PreInitcheck:Unable to verify ping request with gnoiClient.")
	}
	t.Logf("PreInitcheck:gNOI dial %s is successful.", gnoiC)
	//p4RT service validation in the preinit check.
	p4rtC, err := dut.RawAPIs().BindingDUT().DialP4RT(ctx)
	if err != nil {
		t.Fatalf("Failed to dial p4RT Connection with error: %v.", err)
	}
	if result := ValidateP4RtCapabilitiesRequest(ctx, t, p4rtC, mismatch); !result {
		t.Logf("PreInitcheck:Unable to verify p4RT capability request with p4RTclient.")
	}
	t.Logf("PreInitcheck:%s p4RT dial %s is successful.", time.Now().String(), p4rtC)
	//gNSI service validation in the preinit check.
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(ctx)
	if err != nil {
		t.Fatalf("Failed to create gNSI Connection %v", err)
	}
	if result := ValidateGnsiAuthzGetRequest(ctx, t, gnsiC.Authz(), mismatch); !result {
		t.Logf("PreInitcheck:Authz request failed.")
	}
	t.Logf("PreInitcheck:gNSI dial is successful %s", gnsiC)
	return gnmiC, gnsiC
}

// GetSslProfilelist function to fetch the existing ssl profiles on the device.
func GetSslProfilelist(ctx context.Context, t *testing.T, certzClient certzpb.CertzClient, certzGetReq *certzpb.GetProfileListRequest) *certzpb.GetProfileListResponse {

	getProfileResponse, err := certzClient.GetProfileList(ctx, certzGetReq)
	if err != nil {
		t.Fatalf("GetProfileList request failed with %v!", err)
		return nil
	}
	t.Logf("GetProfileList response: %s", getProfileResponse)
	return getProfileResponse
}

// ServicesValidationCheck function to do a validation of all services after certz rotation.
func ServicesValidationCheck(t *testing.T, caCert *x509.CertPool, expectedResult bool, san, serverAddr, username, password string, cert tls.Certificate, mismatch bool) bool {

	t.Logf("%s:Verifying New gNOI connection.", time.Now().String())
	if result := VerifyGnoi(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Fatalf("Failed with new gNOI Connection: got %v, want %v", result, expectedResult)
	}
	t.Logf("%s:Verifying New gRIBI connection.", time.Now().String())
	if result := VerifyGribi(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Fatalf("Failed with new gRIBI Connection: got %v, want %v.", result, expectedResult)
	}
	t.Logf("%s:Verifying New P4rt connection.", time.Now().String())
	if result := VerifyP4rt(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Fatalf("Failed with new P4rt Connection: got %v, want %v.", result, expectedResult)
	}
	t.Logf("%s:Verifying New gNMI connection.", time.Now().String())
	if result := VerifyGnmi(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Fatalf("Failed with new gNMI Connection: got %v, want %v.", result, expectedResult)
	}
	t.Logf("%s:Verifying New gNSI connection.", time.Now().String())
	if result := VerifyGnsi(t, caCert, san, serverAddr, username, password, cert, mismatch); !result {
		t.Fatalf("Failed with new gNSI Connection: got %v, want %v.", result, expectedResult)
	}
	return true
}
