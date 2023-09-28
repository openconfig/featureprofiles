// Package svid provides functions to generate and sign SVID.
// for more info related to SVID refer to:
//
//	https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md
package svid

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"time"
)

// GenSVID generates a SVID certificate for user and signs it based on given signing cert/key and public key algorithm
func GenSVID(commonName string, spiffeID string, expireInDays int, signingCert *x509.Certificate, signingKey any, keyAlgo x509.PublicKeyAlgorithm) (*tls.Certificate, error) {
	certSpec, err := populateCertTemplate(commonName, spiffeID, expireInDays)
	if err != nil {
		return nil, err
	}
	var privKey crypto.PrivateKey
	switch keyAlgo {
	case x509.RSA:
		privKey, err = rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, err
		}
	case x509.ECDSA:
		curve := elliptic.P256()
		privKey, err = ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("key algorithm %v is not supported", keyAlgo)
	}
	pubKey := privKey.(crypto.Signer).Public()
	certBytes, err := x509.CreateCertificate(rand.Reader, certSpec, signingCert, pubKey, signingKey)
	if err != nil {
		return nil, err
	}
	x509Cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return nil, err
	}
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  privKey,
		Leaf:        x509Cert,
	}
	return &tlsCert, nil
}

func populateCertTemplate(commonName, spiffeID string, expireInDays int) (*x509.Certificate, error) {
	uri, err := url.Parse(spiffeID)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(big.MaxBase))
	if err != nil {
		return nil, err
	}
	// following https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md#appendix-a-x509-field-reference
	certSpec := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   commonName,
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

// LoadKeyPair loads a pair of RSA/ECDSA private key and certificate from pem files
func LoadKeyPair(keyPath, certPath string) (any, *x509.Certificate, error) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}
	caKeyPem, _ := pem.Decode(keyPEM)
	var caPrivateKey any
	if caKeyPem == nil {
		return nil, nil, fmt.Errorf("error in loading private key")
	}
	switch caKeyPem.Type {
	case "EC PRIVATE KEY":
		caPrivateKey, err = x509.ParseECPrivateKey(caKeyPem.Bytes)
		if err != nil {
			return nil, nil, err
		}
	case "RSA PRIVATE KEY":
		caPrivateKey, err = x509.ParsePKCS1PrivateKey(caKeyPem.Bytes)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("file does not contain an ECDSA/RSA private key")

	}
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
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
