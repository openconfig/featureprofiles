// Package cert provides functions to generate and load  certificates.
package cert

import (
	"crypto/x509"
	"net"
	"os"
	"testing"
)

func TestRootCA(t *testing.T) {
	os.Mkdir("testdata/", 0755)
	defer os.RemoveAll("testdata/")
	// test ecdsa
	_, _, err := GenRootCA("test", x509.ECDSA, 100, "testdata/")
	if err != nil {
		t.Fatalf("Generation of root ca using ECDSA is failed: %v", err)
	}
	caKey, caCert, err := LoadKeyPair("testdata/cakey.ecdsa.pem", "testdata/cacert.ecdsa.pem")
	if err != nil {
		t.Fatalf("Could not load the generated key and cer: %v", err)
	}
	certTemp, err := PopulateCertTemplate("test", []string{"test"}, []net.IP{net.IPv4(173, 39, 51, 67)}, "testspiffie", 100)
	if err != nil {
		t.Fatalf("Could not generate the cert template: %v", err)
	}
	_, err = GenerateCert(certTemp, caCert, caKey, x509.ECDSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}

	// test rsa
	_, _, err = GenRootCA("test", x509.RSA, 100, "testdata/")
	if err != nil {
		t.Fatalf("Generation of root ca using ECDSA is failed: %v", err)
	}
	caKey, caCert, err = LoadKeyPair("testdata/cakey.rsa.pem", "testdata/cacert.rsa.pem")
	if err != nil {
		t.Fatalf("Could not load the generated key and cer: %v", err)
	}
	_, err = GenerateCert(certTemp, caCert, caKey, x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}

}
