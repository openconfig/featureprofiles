package main

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	certUtil "github.com/openconfig/featureprofiles/internal/cisco/security/cert"
)

func main() {
	username := flag.String("username", "cafyauto", "Username to embed in the certificate CN/DNS fields and SPIFFE ID")
	ipsFlag := flag.String("ips", "", "Comma-separated IP addresses to include in the certificate SAN")
	outDir := flag.String("out", ".", "Output directory for generated artifacts")
	certExpireDays := flag.Int("cert-days", 365, "Number of days the client certificate remains valid")
	caExpireYears := flag.Int("ca-years", 1, "Number of years the generated CA certificate remains valid")
	flag.Parse()

	if strings.TrimSpace(*username) == "" {
		log.Fatal("--username cannot be empty")
	}
	if *ipsFlag == "" {
		log.Fatal("--ips is required")
	}

	ips, err := parseIPs(*ipsFlag)
	if err != nil {
		log.Fatalf("invalid --ips value: %v", err)
	}
	if len(ips) == 0 {
		log.Fatal("no valid IPs provided")
	}

	trimmedUsername := strings.TrimSpace(*username)
	dnsNames := []string{trimmedUsername}
	spiffeID := trimmedUsername

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	caPrivKey, caCert, err := generateRootCA(trimmedUsername, *caExpireYears)
	if err != nil {
		log.Fatalf("failed to generate CA: %v", err)
	}

	certTemplate, err := certUtil.PopulateCertTemplate(trimmedUsername, dnsNames, ips, spiffeID, *certExpireDays)
	if err != nil {
		log.Fatalf("failed to populate certificate template: %v", err)
	}

	tlsCert, err := certUtil.GenerateCert(certTemplate, caCert, caPrivKey, x509.RSA)
	if err != nil {
		log.Fatalf("failed to generate client certificate: %v", err)
	}

	certPath := filepath.Join(*outDir, "certificate.pem")
	keyPath := filepath.Join(*outDir, "key.pem")
	if err := certUtil.SaveTLSCertInPems(tlsCert, keyPath, certPath, x509.RSA); err != nil {
		log.Fatalf("failed to save client key/cert: %v", err)
	}

	caCertPath := filepath.Join(*outDir, "CABundle.pem")
	if err := writeCertificate(caCertPath, caCert.Raw); err != nil {
		log.Fatalf("failed to write CA certificate: %v", err)
	}

	if err := writeCAPrivateKey(filepath.Join(*outDir, "ca.key"), caPrivKey); err != nil {
		log.Fatalf("failed to write CA key: %v", err)
	}

	combinedPath := filepath.Join(*outDir, "tls_bundle.pem")
	if err := writeCombinedPEM(combinedPath, tlsCert.Leaf, tlsCert.PrivateKey, caCert); err != nil {
		log.Fatalf("failed to write combined PEM file: %v", err)
	}

	fmt.Printf("Generated files:\n  %s\n  %s\n  %s\n  %s\n  %s\n", certPath, keyPath, caCertPath, filepath.Join(*outDir, "ca.key"), combinedPath)
}

func parseIPs(raw string) ([]net.IP, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	pieces := strings.Split(trimmed, ",")
	ips := make([]net.IP, 0, len(pieces))
	for _, piece := range pieces {
		p := strings.TrimSpace(piece)
		if p == "" {
			continue
		}
		ip := net.ParseIP(p)
		if ip == nil {
			return nil, fmt.Errorf("%q is not a valid IP address", p)
		}
		ips = append(ips, ip)
	}
	return ips, nil
}

func generateRootCA(username string, years int) (crypto.PrivateKey, *x509.Certificate, error) {
	if years <= 0 {
		years = 1
	}
	tmpDir, err := os.MkdirTemp("", "certgen-ca")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(tmpDir)

	trimmedUsername := strings.TrimSpace(username)
	cn := fmt.Sprintf("%s Root CA", trimmedUsername)
	if trimmedUsername == "" {
		cn = "FeatureProfiles Root CA"
	}
	priv, cert, err := certUtil.GenRootCA(cn, x509.RSA, years, tmpDir)
	if err != nil {
		return nil, nil, err
	}
	return priv, cert, nil
}

func writeCertificate(path string, derBytes []byte) error {
	block := &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}
	encoded := pem.EncodeToMemory(block)
	if encoded == nil {
		return errors.New("failed to encode certificate to PEM")
	}
	return os.WriteFile(path, encoded, 0o644)
}

func writeCAPrivateKey(path string, key crypto.PrivateKey) error {
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return errors.New("unexpected key type for RSA private key")
	}
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)}
	encoded := pem.EncodeToMemory(block)
	if encoded == nil {
		return errors.New("failed to encode key to PEM")
	}
	return os.WriteFile(path, encoded, 0o600)
}

func writeCombinedPEM(path string, clientCert *x509.Certificate, clientKey crypto.PrivateKey, caCert *x509.Certificate) error {
	var pemData []byte

	// Add client certificate
	certBlock := &pem.Block{Type: "CERTIFICATE", Bytes: clientCert.Raw}
	pemData = append(pemData, pem.EncodeToMemory(certBlock)...)

	// Add private key (PKCS#8 format for "PRIVATE KEY" header)
	privKeyBytes, err := x509.MarshalPKCS8PrivateKey(clientKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: privKeyBytes}
	pemData = append(pemData, pem.EncodeToMemory(keyBlock)...)

	// Add root CA certificate
	caBlock := &pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw}
	pemData = append(pemData, pem.EncodeToMemory(caBlock)...)

	return os.WriteFile(path, pemData, 0o644)
}
