// Package svid provides functions to generate and sign svid.
package svid

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"strings"

	"math/big"
	"net/url"

	"time"
)

// GenRSASVID Generates SVID for user and signs it based on given rsa cert/key
func GenRSASVID(id string, expireInDays int, signingCert *x509.Certificate, signingKey any) (*rsa.PrivateKey, *x509.Certificate, error) {
	certSpec, err := populateCertTemplate(id, expireInDays)
	if err != nil {
		return nil, nil, err
	}
	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, certSpec, signingCert, &privKey.PublicKey, signingKey)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, err
	}
	return privKey, cert, nil
}

// GenECDSASVID Generates SVID for user and signs it based on given ECDSA cert/key
func GenECDSASVID(id string, expireInDays int, signingCert *x509.Certificate, signingKey any) (*ecdsa.PrivateKey, *x509.Certificate, error) {
	certSpec, err := populateCertTemplate(id, expireInDays)
	if err != nil {
		return nil, nil, err
	}
	curve := elliptic.P256()
	privKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, certSpec, signingCert, &privKey.PublicKey, signingKey)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, nil, err
	}
	return privKey, cert, nil
}

func populateCertTemplate(id string, expireInDays int) (*x509.Certificate, error) {
	uri, err := url.Parse(id)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(big.MaxBase))
	if err != nil {
		return nil, err
	}
	// follows https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md#appendix-a-x509-field-reference
	certSpec := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   id,
			Organization: []string{"OpenconfigFeatureProfiles"},
			Country:      []string{"US"},
		},
		URIs:        []*url.URL{uri},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(0, 0, expireInDays),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}
	return certSpec, nil
}

// loadKeyPair load a pair of RSA/ECDSA private key and certificate
func loadKeyPair(keyPEM, certPEM []byte) (any, *x509.Certificate, error) {
	var err error
	caKeyPem, _ := pem.Decode(keyPEM)
	var caPrivateKey any
	if caKeyPem == nil {
		return nil, nil, fmt.Errorf("error in loading private key")
	}
	if strings.Contains(caKeyPem.Type, "EC") {
		caPrivateKey, err = x509.ParseECPrivateKey(caKeyPem.Bytes)
		if err != nil {
			return nil, nil, err
		}
	} else if strings.Contains(caKeyPem.Type, "RSA") {
		caPrivateKey, err = x509.ParsePKCS1PrivateKey(caKeyPem.Bytes)
		if err != nil {
			return nil, nil, err
		}
	} else {
		return nil, nil, fmt.Errorf("file does not contain an ECDSA/RSA private key")
	}

	caCertPem, _ := pem.Decode(certPEM)
	if caCertPem == nil {
		return nil, nil, fmt.Errorf("error in loading ca cert")
	}
	caCert, err := x509.ParseCertificate(caCertPem.Bytes)
	if err != nil {
		return nil, nil, err
	}
	return caPrivateKey, caCert, nil
}
