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

package attestz

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"

	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
)

type CertType int

const (
	CertTypeRaw    CertType = 0
	CertTypeIdevid CertType = 1
)

type entityType int

const (
	entityTypeCertificate entityType = 0
	entityTypeTrust       entityType = 1
)

// AddProfile adds ssl profile on the dut.
func AddProfile(t *testing.T, dut *ondatra.DUTDevice, sslProfileId string) {
	t.Logf("Performing Certz.AddProfile on device %s for profile %s", dut.Name(), sslProfileId)
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
	if err != nil {
		t.Errorf("gNSI client error: %v", err)
	}
	_, err = gnsiC.Certz().AddProfile(context.Background(), &certzpb.AddProfileRequest{
		SslProfileId: sslProfileId,
	})
	if err != nil {
		t.Fatalf("Error adding tls profile via certz. error: %v", err)
	}
}

// DeleteProfile delete ssl profile from the dut.
func DeleteProfile(t *testing.T, dut *ondatra.DUTDevice, sslProfileId string) {
	t.Logf("Performing Certz.DeleteProfile on device %s for profile %s", dut.Name(), sslProfileId)
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
	if err != nil {
		t.Errorf("gNSI client error: %v", err)
	}
	_, err = gnsiC.Certz().DeleteProfile(context.Background(), &certzpb.DeleteProfileRequest{
		SslProfileId: sslProfileId,
	})
	if err != nil {
		t.Fatalf("Error deleting tls profile via certz. error: %v", err)
	}
}

func createEntity(entityType entityType, certificate *certzpb.Certificate) *certzpb.Entity {
	entity := &certzpb.Entity{
		Version:   fmt.Sprintf("v0.%v", time.Now().Unix()),
		CreatedOn: uint64(time.Now().Unix()),
	}
	certChain := &certzpb.CertificateChain{
		Certificate: certificate,
	}

	switch entityType {
	case entityTypeCertificate:
		entity.Entity = &certzpb.Entity_CertificateChain{
			CertificateChain: certChain,
		}
	case entityTypeTrust:
		entity.Entity = &certzpb.Entity_TrustBundle{
			TrustBundle: certChain,
		}
	}

	return entity
}

func createCertificate(rotateType CertType, keyContents, certContents []byte) *certzpb.Certificate {
	cert := &certzpb.Certificate{
		Type:     certzpb.CertificateType_CERTIFICATE_TYPE_X509,
		Encoding: certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
	}

	switch rotateType {
	case CertTypeIdevid:
		cert.PrivateKeyType = &certzpb.Certificate_KeySource_{
			KeySource: certzpb.Certificate_KEY_SOURCE_IDEVID_TPM,
		}
		cert.CertificateType = &certzpb.Certificate_CertSource_{
			CertSource: certzpb.Certificate_CERT_SOURCE_IDEVID,
		}
	case CertTypeRaw:
		cert.PrivateKeyType = &certzpb.Certificate_RawPrivateKey{
			RawPrivateKey: keyContents,
		}
		cert.CertificateType = &certzpb.Certificate_RawCertificate{
			RawCertificate: certContents,
		}
	}

	return cert
}

// RotateCerts rotates certificates on the dut for a given ssl profile.
func RotateCerts(t *testing.T, dut *ondatra.DUTDevice, rotateType CertType, sslProfileId string, dutKey, dutCert, trustBundle []byte) {
	t.Logf("Performing Certz.Rotate request on device %s", dut.Name())
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
	if err != nil {
		t.Errorf("gNSI client error: %v", err)
	}
	rotateStream, err := gnsiC.Certz().Rotate(context.Background())
	if err != nil {
		t.Fatalf("Could not start a rotate stream. error: %v", err)
	}
	defer rotateStream.CloseSend()

	var entities []*certzpb.Entity
	switch rotateType {
	case CertTypeIdevid:
		certificate := createCertificate(rotateType, nil, nil)
		entities = append(entities, createEntity(entityTypeCertificate, certificate))
	case CertTypeRaw:
		if dutKey != nil && dutCert != nil {
			certificate := createCertificate(rotateType, dutKey, dutCert)
			entities = append(entities, createEntity(entityTypeCertificate, certificate))
		}
		if trustBundle != nil {
			certificate := createCertificate(rotateType, nil, trustBundle)
			entities = append(entities, createEntity(entityTypeTrust, certificate))
		}
	}

	// Create rotate request.
	certzRequest := &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   sslProfileId,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: entities,
			},
		},
	}

	// Send rotate request.
	t.Logf("Sending Certz.Rotate request on device: \n %s", PrettyPrint(certzRequest))
	err = rotateStream.Send(certzRequest)
	if err != nil {
		t.Fatalf("Error while uploading certz request. error: %v", err)
	}
	t.Logf("Certz.Rotate upload was successful, receiving response ...")
	_, err = rotateStream.Recv()
	if err != nil {
		t.Fatalf("Error while receiving certz rotate reply. error: %v", err)
	}

	// Finalize rotation.
	finalizeRotateRequest := &certzpb.RotateCertificateRequest{
		SslProfileId: sslProfileId,
		RotateRequest: &certzpb.RotateCertificateRequest_FinalizeRotation{
			FinalizeRotation: &certzpb.FinalizeRequest{},
		},
	}
	t.Logf("Sending Certz.Rotate FinalizeRotation request: \n %s", PrettyPrint(finalizeRotateRequest))
	err = rotateStream.Send(finalizeRotateRequest)
	if err != nil {
		t.Fatalf("Error while finalizing rotate request. error: %v", err)
	}

	// Brief sleep for finalize to get processed.
	time.Sleep(time.Second)
}

// LoadCertificate decodes certificate provided as a string.
func LoadCertificate(cert string) (*x509.Certificate, error) {
	certPem, _ := pem.Decode([]byte(cert))
	if certPem == nil {
		return nil, fmt.Errorf("Error decoding certificate.")
	}
	return x509.ParseCertificate(certPem.Bytes)
}

// GenOwnerCert creates switch owner iak/idevid certs & signs it based on given ca cert/key.
func GenOwnerCert(t *testing.T, caKey any, caCert *x509.Certificate, inputCert string, pubKey any, netAddr string) string {
	cert, err := LoadCertificate(inputCert)
	if err != nil {
		t.Fatalf("Error loading vendor certificate. error: %v", err)
	}
	if pubKey == nil {
		pubKey = cert.PublicKey
	}

	// Generate Random Serial Number as per TCG Spec (between 64 and 160 bits).
	// https://trustedcomputinggroup.org/wp-content/uploads/TPM-2p0-Keys-for-Device-Identity-and-Attestation_v1_r12_pub10082021.pdf#page=55
	minBits := 64
	maxBits := 160
	minVal := new(big.Int).Lsh(big.NewInt(1), uint(minBits-1)) // minVal = 2^63
	maxVal := new(big.Int).Lsh(big.NewInt(1), uint(maxBits))   // maxVal = 2^160
	// Random number between [2^63, 2^160)
	serial, err := rand.Int(rand.Reader, maxVal.Sub(maxVal, minVal))
	if err != nil {
		t.Fatalf("Error generating serial number. error: %s", err)
	}
	serial.Add(minVal, serial)
	t.Logf("Generated new serial number for cert: %s", serial)

	// Get IP Address from gnsi target
	ip, _, err := net.SplitHostPort(netAddr)
	if err != nil {
		t.Errorf("Error parsing host:port info. error: %v", err)
	}

	// Create switch owner certificate template.
	ownerCert := &x509.Certificate{
		SerialNumber: serial,
		NotBefore:    time.Now(),
		NotAfter:     cert.NotAfter,
		Subject:      cert.Subject,
		KeyUsage:     cert.KeyUsage,
		ExtKeyUsage:  cert.ExtKeyUsage,
		IPAddresses:  []net.IP{net.ParseIP(ip)},
	}

	// Sign certificate with switch owner ca.
	certBytes, err := x509.CreateCertificate(rand.Reader, ownerCert, caCert, pubKey, caKey)
	if err != nil {
		t.Fatalf("Could not generate owner certificate. error: %v", err)
	}

	// PEM encode switch owner certificate.
	pemBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}
	certPEM := new(bytes.Buffer)
	if err := pem.Encode(certPEM, pemBlock); err != nil {
		t.Fatalf("Could not PEM encode owner certificate. error: %v", err)
	}

	return certPEM.String()
}

// GenTlsCert creates mtls client/server certificates & signs it based on given signing cert/key.
func GenTlsCert(ip string, signingCert *x509.Certificate, signingKey any, keyAlgo x509.PublicKeyAlgorithm) ([]byte, []byte, error) {
	certSpec, err := populateCertTemplate(ip)
	if err != nil {
		return nil, nil, err
	}
	var privKey crypto.PrivateKey
	switch keyAlgo {
	case x509.RSA:
		privKey, err = rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, nil, err
		}
	case x509.ECDSA:
		curve := elliptic.P256()
		privKey, err = ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("Key algorithm %v is not supported.", keyAlgo)
	}
	pubKey := privKey.(crypto.Signer).Public()
	certBytes, err := x509.CreateCertificate(rand.Reader, certSpec, signingCert, pubKey, signingKey)
	if err != nil {
		return nil, nil, err
	}
	// PEM encode certificate.
	pemBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}
	certPem := new(bytes.Buffer)
	if err = pem.Encode(certPem, pemBlock); err != nil {
		return nil, nil, err
	}
	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return nil, nil, err
	}
	// PEM encode private key.
	pemBlock = &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privKeyBytes,
	}
	privKeyPem := new(bytes.Buffer)
	if err = pem.Encode(privKeyPem, pemBlock); err != nil {
		return nil, nil, err
	}
	return certPem.Bytes(), privKeyPem.Bytes(), nil
}

func populateCertTemplate(ip string) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, big.NewInt(big.MaxBase))
	if err != nil {
		return nil, err
	}
	certSpec := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   ip,
			Organization: []string{"OpenconfigFeatureProfiles"},
			Country:      []string{"US"},
		},
		IPAddresses: []net.IP{net.ParseIP(ip)},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(0, 0, 1),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}
	return certSpec, nil
}
