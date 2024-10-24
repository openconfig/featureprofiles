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

// Package setupservice is scoped only to be used for scripts in path
// feature/security/gnsi/certz/tests/client_certificates
// Do not use elsewhere.
package setupservice

import (
	context "context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	//"io"
	"os"
	"os/exec"
	"testing"
	"time"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	authzpb "github.com/openconfig/gnsi/authz"
	certzpb "github.com/openconfig/gnsi/certz"
	gribipb "github.com/openconfig/gribi/v1/proto/service"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/knebind/creds"
	p4rtpb "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

var (
	username = "certzuser"
	password = "certzpasswd"
	sn       = "role001.pop55.net.example.com"
	servers  []string
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

type entityType int8

const (
	// EntityTypeCertificateChain is type of entity of the certificate chain.
	EntityTypeCertificateChain entityType = 0
	// EntityTypeTrustBundle is type of entity of the trust bundle.
	EntityTypeTrustBundle entityType = 1
	// EntityTypeCRL is type of entity of the CRL.
	EntityTypeCRL entityType = 2
	// EntityTypeAuthPolicy is type of entity of the auth policy.
	EntityTypeAuthPolicy entityType = 3
)

// CertificateChainRequest is an input argument for the  type definition for the  CreateCertzChain.
type CertificateChainRequest struct {
	RequestType     entityType
	ServerCertFile  string
	ServerKeyFile   string
	TrustBundleFile string
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
			Entity:    &certzpb.Entity_CertificateChain{CertificateChain: entityContent.(*certzpb.CertificateChain)}}

	case EntityTypeTrustBundle:

		return certzpb.Entity{
			Version:   entityVersion,
			CreatedOn: varClock,
			Entity:    &certzpb.Entity_TrustBundle{TrustBundle: entityContent.(*certzpb.CertificateChain)}}

	case EntityTypeCRL:

		return certzpb.Entity{
			Version:   entityVersion,
			CreatedOn: varClock,
			Entity:    &certzpb.Entity_CertificateRevocationListBundle{CertificateRevocationListBundle: entityContent.(*certzpb.CertificateRevocationListBundle)}}

	case EntityTypeAuthPolicy:

		return certzpb.Entity{
			Version:   entityVersion,
			CreatedOn: varClock,
			Entity:    &certzpb.Entity_AuthenticationPolicy{AuthenticationPolicy: entityContent.(*certzpb.AuthenticationPolicy)}}

	default:
		t.Fatalf("Invalid entity type")
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
				Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
				Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
				Certificate: serverCertContent,
				PrivateKey:  serverKeyContent}, Parent: nil}
		}
		return certzpb.CertificateChain{Certificate: &certzpb.Certificate{
			Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
			Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
			Certificate: serverCertContent,
			PrivateKey:  nil}, Parent: nil}

	case EntityTypeTrustBundle:
		if len(certData.TrustBundleFile) == 0 {
			t.Fatalf("Missing trust bundle file for creating certificate chain object.")
		}
		trustBundleContent, err := os.ReadFile(certData.TrustBundleFile)
		if err != nil {
			t.Fatalf("Error reading trust bundle file at: %v with error: %v", certData.TrustBundleFile, err)
		}
		return certzpb.CertificateChain{Certificate: &certzpb.Certificate{
			Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
			Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
			Certificate: trustBundleContent,
		}, Parent: nil}

	default:
		t.Fatalf("Invalid request type received.")
	}
	return certzpb.CertificateChain{}
}

// CreateCertChainFromTrustBundle function to create the certificate chain from trust bundle.
func CreateCertChainFromTrustBundle(fileName string) *certzpb.CertificateChain {
	pemData, err := os.ReadFile(fileName)
	if err != nil {
		return &certzpb.CertificateChain{}
	}
	var trust [][]byte
	for {
		var block *pem.Block
		block, pemData = pem.Decode(pemData)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		p := pem.EncodeToMemory(block)
		if p == nil {
			return &certzpb.CertificateChain{}
		}
		trust = append(trust, p)
	}
	//a valid check for trust not empty
	if len(trust) == 0 {
		return &certzpb.CertificateChain{}
	} else {
		var prevCert *certzpb.CertificateChain
		var bundleToReturn *certzpb.CertificateChain
		for i := len(trust) - 1; i >= 0; i-- {
			if i == len(trust)-1 {
				bundleToReturn = &certzpb.CertificateChain{Certificate: &certzpb.Certificate{
					Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
					Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
					Certificate: trust[i],
				}, Parent: nil}
				prevCert = bundleToReturn
			} else {
				prevCert = bundleToReturn
				bundleToReturn = &certzpb.CertificateChain{Certificate: &certzpb.Certificate{
					Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
					Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
					Certificate: trust[i],
				}, Parent: prevCert}
			}
		}
		return bundleToReturn
	}
}

// CertzRotate function to request the client certificate rotation and returns true on successful rotation.
func CertzRotate(t *testing.T, caCert *x509.CertPool, certzClient certzpb.CertzClient, cert tls.Certificate, san, serverAddr, profileID string, entities ...*certzpb.Entity) bool {
	if len(entities) == 0 {
		t.Logf("At least one entity required for Rotate request.")
		return false
	}
	uploadRequest := &certzpb.UploadRequest{Entities: entities}
	rotateRequest := &certzpb.RotateCertificateRequest_Certificates{Certificates: uploadRequest}
	rotateCertRequest := &certzpb.RotateCertificateRequest{
		ForceOverwrite: false,
		SslProfileId:   profileID,
		RotateRequest:  rotateRequest}
	rotateRequestClient, err := certzClient.Rotate(context.Background())
	defer rotateRequestClient.CloseSend()
	if err != nil {
		t.Fatalf("Error creating rotate request client: %v", err)
	}
	err = rotateRequestClient.Send(rotateCertRequest)
	if err != nil {
		t.Fatalf("Error sending rotate request: %v", err)
	}
	rotateResponse := &certzpb.RotateCertificateResponse{}
	for i := 0; i < 20; i++ {
		rotateResponse, err = rotateRequestClient.Recv()
		if err == nil {
			break
		}
		t.Logf("Did not receive response ~ %vs after sending rotate request. Sleeping 10s to retry...", i*10)
		time.Sleep(10 * time.Second)
	}
	if err != nil {
		t.Logf("Error fetching rotate certificate response: %v", err)
		return false
	}
	t.Logf("Received Rotate certificate response: %v", rotateResponse)

	finalizeRequest := &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}}
	rotateCertRequest = &certzpb.RotateCertificateRequest{
		ForceOverwrite: false,
		SslProfileId:   profileID,
		RotateRequest:  finalizeRequest}

	err = rotateRequestClient.Send(rotateCertRequest)
	if err != nil {
		t.Fatalf("Error sending rotate finalize request: %v", err)
	}
	err = rotateRequestClient.CloseSend()
	if err != nil {
		t.Fatalf("Error sending rotate close send request: %v", err)
	}
	return true
}

// ServerCertzRotate function to request the server certificate rotation and returns true on successful rotation.
func ServerCertzRotate(t *testing.T, caCert *x509.CertPool, certzClient certzpb.CertzClient, cert tls.Certificate, ctx context.Context, dut *ondatra.DUTDevice, san, serverAddr, profileID string, entities ...*certzpb.Entity) bool {
	if len(entities) == 0 {
		t.Logf("At least one entity required for Rotate request.")
		return false
	}
	uploadRequest := &certzpb.UploadRequest{Entities: entities}
	rotateRequest := &certzpb.RotateCertificateRequest_Certificates{Certificates: uploadRequest}
	rotateCertRequest := &certzpb.RotateCertificateRequest{
		ForceOverwrite: false,
		SslProfileId:   profileID,
		RotateRequest:  rotateRequest}
	rotateRequestClient, err := certzClient.Rotate(context.Background())
	defer rotateRequestClient.CloseSend()
	if err != nil {
		t.Fatalf("Error creating rotate request client: %v", err)
	}
	err = rotateRequestClient.Send(rotateCertRequest)
	if err != nil {
		t.Fatalf("Error sending rotate request: %v", err)
	}
	rotateResponse := &certzpb.RotateCertificateResponse{}
	for i := 0; i < 6; i++ {
		rotateResponse, err = rotateRequestClient.Recv()
		if err == nil {
			break
		}
		t.Logf("Did not receive response ~ %vs after sending rotate request. Sleeping 10s to retry...", i*10)
		time.Sleep(10 * time.Second)
	}
	if err != nil {
		t.Logf("Error fetching rotate certificate response: %v", err)
		return false
	}
	t.Logf("Received Rotate certificate response: %v", rotateResponse)

	// Replace config with newly added ssl profile after successful rotate.
	servers = gnmi.GetAll(t, dut, gnmi.OC().System().GrpcServerAny().Name().State())
	batch := gnmi.SetBatch{}
	for _, server := range servers {
		gnmi.BatchReplace(&batch, gnmi.OC().System().GrpcServer(server).CertificateId().Config(), profileID)
	}
	batch.Set(t, dut)
	t.Logf("gNMI config is replaced with new ssl profile successfully.")
	success := false
	//Trying for 60s for the connection to succeed.
	for i := 0; i < 6; i++ {
		success = VerifyGnsi(t, caCert, san, serverAddr, username, password, cert)
		if success {
			break
		}
		if i != 10 {
			t.Logf("gNSI service RPC did not succeed ~ %vs after rotate. Sleeping 10s to retry...", i*10)
		}
		time.Sleep(10 * time.Second)
	}
	if success {
		finalizeRequest := &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}}
		rotateCertRequest = &certzpb.RotateCertificateRequest{
			ForceOverwrite: false,
			SslProfileId:   profileID,
			RotateRequest:  finalizeRequest}

		err = rotateRequestClient.Send(rotateCertRequest)
		if err != nil {
			t.Fatalf("Error sending rotate finalize request: %v", err)
		}
		err = rotateRequestClient.CloseSend()
		if err != nil {
			t.Fatalf("Error sending rotate close send request: %v", err)
		}
		return true
	} else {
		t.Logf("gNSI service RPC  did not succeed ~60s after rotate. Certz/Rotate failed. FinalizeRequest will not be sent")
		return false
	}
}

// CertGeneration function to create test data for use in TLS tests.
func CertGeneration(t *testing.T, dirPath string) error {
	cmd := exec.Cmd{
		Path:   "./mk_cas.sh",
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	cmd.Dir = dirPath
	t.Logf("Executing cert generation command %v.", cmd)
	err := cmd.Start()
	if err != nil {
		t.Logf("Cert generation command failed with error:%v.", err)
		return err
	}
	err = cmd.Wait()
	if err != nil {
		t.Logf("Failed to run cert generation command during wait with error:%v.", err)
		return err
	}
	return err
}

// CertCleanup function to  clean out the CA content under test_data.
func CertCleanup(t *testing.T, dirPath string) error {
	cmd := exec.Cmd{
		Path:   "./cleanup.sh",
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	cmd.Dir = dirPath
	t.Logf("Executing cleanup command")
	err := cmd.Start()
	if err != nil {
		t.Logf("Testdata cleanup command failed with error:%v.", err)
		return err
	}
	err = cmd.Wait()
	if err != nil {
		t.Logf("Testdata cleanup command failed during wait with the error:%v.", err)
		return err
	}
	return err
}

// ReadDecodeServerCertificate function to read and decode server certificates to extract the SubjectAltName.
func ReadDecodeServerCertificate(t *testing.T, serverCertzFile string) (san string) {
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
	if sn != san {
		t.Fatalf("ServerAltName validation failed for %s.", serverCertzFile)
	}
	return san
}

// VerifyGnsi function to validate the gNSI service RPC after successful rotation.
func VerifyGnsi(t *testing.T, caCert *x509.CertPool, san, serverAddr, username, password string, cert tls.Certificate) bool {
	credOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCert,
			ServerName:   san,
		}))}
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
	rsp, err := authzClient.Get(ctx, &authzpb.GetRequest{})
	if err != nil {
		statusError, _ := status.FromError(err)
		if statusError.Code() == codes.FailedPrecondition {
			t.Logf("Expected error FAILED_PRECONDITION seen for authz Get Request with err:%v.", err)
		} else {
			t.Logf("Unexpected error during authz Get Request with err:%v.", err)
			return false
		}
	}
	t.Logf("gNSI authz get response is %s", rsp)
	conn.Close()
	return true
}

// VerifyGnoi function to validate the gNOI service RPC after successful rotation.
func VerifyGnoi(t *testing.T, caCert *x509.CertPool, san, serverAddr, username, password string, cert tls.Certificate) bool {
	credOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCert,
			ServerName:   san,
		}))}
	creds := &rpcCredentials{&creds.UserPass{Username: username, Password: password}}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))
	target := fmt.Sprintf("%s:%d", serverAddr, 9339)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(target, credOpts...)
	if err != nil {
		t.Errorf("VerifyGnoi : gRPC NewClient failed to %q with err %v", target, err)
		return false
	}
	defer conn.Close()
	sysClient := spb.NewSystemClient(conn)
	_, err = sysClient.Ping(ctx, &spb.PingRequest{})
	if err != nil {
		t.Logf("Unable to connect gnoiClient %v", err)
		return false
	}
	conn.Close()
	return true
}

// VerifyGnmi function to validate the gNMI service RPC after successful rotation.
func VerifyGnmi(t *testing.T, caCert *x509.CertPool, san, serverAddr, username, password string, cert tls.Certificate) bool {
	credOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCert,
			ServerName:   san,
		}))}
	creds := &rpcCredentials{&creds.UserPass{Username: username, Password: password}}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))
	target := fmt.Sprintf("%s:%d", serverAddr, 9339)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(target, credOpts...)
	if err != nil {
		t.Errorf("VerifyGnmi: gRPC NewClient failed to %q with err %v", target, err)
		return false
	}
	defer conn.Close()
	gnmiClient := gnmipb.NewGNMIClient(conn)
	t.Logf("%s:Sending gNMI Capability request.", time.Now().String())
	response, err := gnmiClient.Capabilities(ctx, &gnmipb.CapabilityRequest{})
	if err != nil {
		t.Logf("gNMI Capability request failed with err: %v", err)
		return false
	}
	t.Logf("VerifyGnmi:gNMI response: %s", response)
	conn.Close()
	return true
}

// VerifyGribi function to validate the gRIBI service RPC after successful rotation.
func VerifyGribi(t *testing.T, caCert *x509.CertPool, san, serverAddr, username, password string, cert tls.Certificate) bool {
	credOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCert,
			ServerName:   san,
		}))}
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
	_, err = gRibiClient.Get(ctx, &gribipb.GetRequest{})
	if err != nil {
		t.Logf("Failed to connect GribiClient with error:%v.", err)
		return false
	}
	conn.Close()
	return true
}

// VerifyP4rt function to validate the P4rt service RPC after successful rotation.
func VerifyP4rt(t *testing.T, caCert *x509.CertPool, san, serverAddr, username, password string, cert tls.Certificate) bool {
	credOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithTransportCredentials(credentials.NewTLS(
		&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCert,
			ServerName:   san,
		}))}
	creds := &rpcCredentials{&creds.UserPass{Username: username, Password: password}}
	credOpts = append(credOpts, grpc.WithPerRPCCredentials(creds))
	target := fmt.Sprintf("%s:%d", serverAddr, 9559)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(target, credOpts...)
	if err != nil {
		t.Errorf("VerifyP4rt : gRPC NewClient failed to %q with error %v.", target, err)
	}
	defer conn.Close()
	p4RtClient := p4rtpb.NewP4RuntimeClient(conn)
	_, err = p4RtClient.Capabilities(ctx, &p4rtpb.CapabilitiesRequest{})
	if err != nil {
		t.Logf("Failed to connect P4rtClient with error %v.", err)
		return false
	}
	conn.Close()
	return true
}

// PreInitCheck function to dial gNMI/gNOI/gRIBI/p4RT services before certz rotation.
func PreInitCheck(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) bool {

	gnmiC, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx)
	if err != nil {
		t.Fatalf("%s Failed to dial gNMI Connection with error: %v.", time.Now().String(), err)
	}
	t.Logf("Precheck:gNMI dial is successful %v", gnmiC)
	gribiC, err := dut.RawAPIs().BindingDUT().DialGRIBI(ctx)
	if err != nil {
		t.Fatalf("%s Failed to dial gRIBI Connection with error: %v.", time.Now().String(), err)
	}
	t.Logf("Precheck:gRIBI dial is successful %v", gribiC)
	gnoiC, err := dut.RawAPIs().BindingDUT().DialGNOI(ctx)
	if err != nil {
		t.Fatalf("%s Failed to dial gNOI Connection with error: %v.", time.Now().String(), err)
	}
	t.Logf("Precheck:gNOI dial is successful %v", gnoiC)
	p4rtC, err := dut.RawAPIs().BindingDUT().DialP4RT(ctx)
	if err != nil {
		t.Fatalf("%s Failed to dial p4RT Connection with error: %v.", time.Now().String(), err)
	}
	t.Logf("Precheck:p4RT dial is successful %v", p4rtC)
	return true
}

// GetSslProfilelist function to fetch the existing ssl profiles on the device.
func GetSslProfilelist(ctx context.Context, t *testing.T, certzClient certzpb.CertzClient, certzGetReq *certzpb.GetProfileListRequest) {
	getProfileResponse, err := certzClient.GetProfileList(ctx, certzGetReq)
	if err != nil {
		t.Fatalf("Get profile list request failed with %v!", err)
	}
	t.Logf("GetProfileResponse: %v", getProfileResponse)
}

// PostValidationCheck function to do a validation of all services after certz rotation.
func PostValidationCheck(t *testing.T, caCert *x509.CertPool, expected_result bool, san, serverAddr, username, password string, cert tls.Certificate) bool {
	t.Logf("%s:Verifying New gNSI connection.", time.Now().String())
	result := VerifyGnsi(t, caCert, san, serverAddr, username, password, cert)
	if expected_result != result {
		t.Fatalf("Failed with new gNSI Connection: got %v, want %v.", result, expected_result)
	}
	t.Logf("%s:Verifying New gNOI connection.", time.Now().String())
	result = VerifyGnoi(t, caCert, san, serverAddr, username, password, cert)
	if expected_result != result {
		t.Fatalf("Failed with new gNOI Connection: got false, want %v", expected_result)
	}
	t.Logf("%s:Verifying New gRIBI connection.", time.Now().String())
	if expected_result != VerifyGribi(t, caCert, san, serverAddr, username, password, cert) {
		t.Fatalf("Failed with new gRIBI Connection: got false, want %v.", expected_result)
	}
	t.Logf("%s:Verifying New P4rt connection.", time.Now().String())
	if expected_result != VerifyP4rt(t, caCert, san, serverAddr, username, password, cert) {
		t.Fatalf("Failed with new P4rt Connection: got false, want %v.", expected_result)
	}
	t.Logf("%s:Verifying New gNMI connection.", time.Now().String())
	if expected_result != VerifyGnmi(t, caCert, san, serverAddr, username, password, cert) {
		t.Fatalf("Failed with new gNMI Connection: got false, want %v.", expected_result)
	}
	return true
}
