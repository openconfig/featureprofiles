// Package cert provides functions to generate and load  certificates.
package cert

import (
	"bytes"
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
	"net"
	"net/url"
	"os"
	"time"
)

func SaveTLSCertInPems(cert *tls.Certificate, keyPath, certPath string, keyAlgo x509.PublicKeyAlgorithm) error {
	keyType := "RSA PRIVATE KEY"
	var err error
	var keyBytes []byte
	switch keyAlgo {
	case x509.RSA:
		keyType = "RSA PRIVATE KEY"
		keyBytes = x509.MarshalPKCS1PrivateKey(cert.PrivateKey.(*rsa.PrivateKey))

	case x509.ECDSA:
		keyType = "EC PRIVATE KEY"
		keyBytes, err = x509.MarshalECPrivateKey(cert.PrivateKey.(*ecdsa.PrivateKey))
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("key algorithm %v is not supported", keyAlgo)
	}
	// pem encode
	caCertPEM := new(bytes.Buffer)
	pem.Encode(caCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Certificate[0],
	})

	if err := os.WriteFile(certPath, caCertPEM.Bytes(), 0444); err != nil {
		return err
	}

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  keyType,
		Bytes: keyBytes,
	})
	if err := os.WriteFile(keyPath, caPrivKeyPEM.Bytes(), 0400); err != nil {
		return err
	}
	return nil
}

// GenerateCert generate a RSA/ECDSA  key/certificate based on given ca key/certificate and cert template
func GenerateCert(certTemplate *x509.Certificate, signingCert *x509.Certificate, signingKey any, keyAlgo x509.PublicKeyAlgorithm) (*tls.Certificate, error) {
	var privKey crypto.PrivateKey
	var err error
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
	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, signingCert, pubKey, signingKey)
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

func PopulateCertTemplate(cname string, domainNames []string, ips []net.IP, spiffeID string, expireInDays int) (*x509.Certificate, error) {
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
			CommonName:   cname,
			Organization: []string{"OpenconfigFeatureProfiles"},
			Country:      []string{"US"},
		},
		IPAddresses: ips,
		URIs:        []*url.URL{uri},
		DNSNames:    domainNames,
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

// GenRootCA a self-signed pair of RSA/ECDSA private key and certificate
func GenRootCA(cn string, keyAlgo x509.PublicKeyAlgorithm, expireInDays int, dir string) (crypto.PrivateKey, *x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, big.NewInt(9999999999999999))
	if err != nil {
		return nil, nil, err
	}
	ca := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{"OpenConfig"},
			Country:      []string{"US"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(expireInDays, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create a private and public key
	var caPrivKey crypto.PrivateKey
	keyType := ""
	keyFileName := ""
	certFileName := ""
	var keyBytes []byte
	switch keyAlgo {
	case x509.RSA:
		caPrivKey, err = rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, nil, err
		}
		keyType = "RSA PRIVATE KEY"
		keyBytes = x509.MarshalPKCS1PrivateKey(caPrivKey.(*rsa.PrivateKey))
		keyFileName = "cakey.rsa.pem"
		certFileName = "cacert.rsa.pem"

	case x509.ECDSA:
		curve := elliptic.P256()
		caPrivKey, err = ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		keyType = "EC PRIVATE KEY"
		keyBytes, err = x509.MarshalECPrivateKey(caPrivKey.(*ecdsa.PrivateKey))
		if err != nil {
			return nil, nil, err
		}
		keyFileName = "cakey.ecdsa.pem"
		certFileName = "cacert.ecdsa.pem"
	default:
		return nil, nil, fmt.Errorf("key algorithm %v is not supported", keyAlgo)
	}
	// create the CA
	pubKey := caPrivKey.(crypto.Signer).Public()
	caCertBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, pubKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	// pem encode
	caCertPEM := new(bytes.Buffer)
	pem.Encode(caCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertBytes,
	})

	if err := os.WriteFile(dir+"/"+certFileName, caCertPEM.Bytes(), 0444); err != nil {
		return nil, nil, err
	}

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  keyType,
		Bytes: keyBytes,
	})
	if err := os.WriteFile(dir+"/"+keyFileName, caPrivKeyPEM.Bytes(), 0400); err != nil {
		return nil, nil, err
	}
	// parsing it from pem to ensure the saved PEM is ok
	pem, _ := pem.Decode(caCertPEM.Bytes())
	caCert, err := x509.ParseCertificate(pem.Bytes)
	if err != nil {
		return nil, nil, err
	}
	return caPrivKey, caCert, nil
}

func GenCRS(certReq *x509.CertificateRequest, keyAlgo x509.PublicKeyAlgorithm, expireInDays int, dir string) (crypto.PrivateKey, *x509.CertificateRequest, error) {
	var privKey crypto.PrivateKey
	keyType := ""
	keyFileName := ""
	var err error
	certReqFileName := ""
	var keyBytes []byte
	switch keyAlgo {
	case x509.RSA:
		privKey, err = rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, nil, err
		}
		keyType = "RSA PRIVATE KEY"
		keyBytes = x509.MarshalPKCS1PrivateKey(privKey.(*rsa.PrivateKey))
		keyFileName = "key.rsa.pem"
		certReqFileName = "cert.csr.rsa.pem"

	case x509.ECDSA:
		curve := elliptic.P256()
		privKey, err = ecdsa.GenerateKey(curve, rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		keyType = "EC PRIVATE KEY"
		keyBytes, err = x509.MarshalECPrivateKey(privKey.(*ecdsa.PrivateKey))
		if err != nil {
			return nil, nil, err
		}
		keyFileName = "key.ecdsa.pem"
		certReqFileName = "cert.csr.ecdsa.pem"
	default:
		return nil, nil, fmt.Errorf("key algorithm %v is not supported", keyAlgo)
	}
	// create the CA

	csr, err := x509.CreateCertificateRequest(rand.Reader, certReq, privKey)
	if err != nil {
		return nil, nil, err
	}

	// pem encode
	csrPEM := new(bytes.Buffer)
	pem.Encode(csrPEM, &pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csr,
	})

	if err := os.WriteFile(dir+"/"+certReqFileName, csrPEM.Bytes(), 0444); err != nil {
		return nil, nil, err
	}

	privKeyPEM := new(bytes.Buffer)
	pem.Encode(privKeyPEM, &pem.Block{
		Type:  keyType,
		Bytes: keyBytes,
	})
	if err := os.WriteFile(dir+"/"+keyFileName, privKeyPEM.Bytes(), 0400); err != nil {
		return nil, nil, err
	}
	// parsing it from pem to ensure the saved PEM is ok
	pem, _ := pem.Decode(csrPEM.Bytes())
	csrReq, err := x509.ParseCertificateRequest(pem.Bytes)
	if err != nil {
		return nil, nil, err
	}
	return privKey, csrReq, nil
}
