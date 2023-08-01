// Package cert provides functions to generate and load  certificates.
package cert

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"strings"

	"math/big"
	"net"
	"net/url"
	"os"

	"time"
)

var (
	caPrivateKey *rsa.PrivateKey
	CACert       *x509.Certificate
	caPath = flag.String("ca_cert_path", "", "path to a directory for ca cert and key located")
	clientKeysPath = flag.String("client_keys_path", "", "path to a directory where clinets keys will be saved")
	caKeyFileName = "ca.key.pem"
	caCertFileName = "ca.cert.pem"
)


func getCACertPath() (string, error){
	if *caPath!="" {
		return *caPath, nil
	}
	cwd, err := os.Getwd(); if err!=nil {
		return "",err
	}
	if strings.Contains(cwd, "/featureprofiles/"){
		rootSrc:=strings.Split(cwd, "featureprofiles")[0]
		return rootSrc + "featureprofiles/internal/cisco/security/cert/keys/CA/", nil
	}
	return "",fmt.Errorf("ca_cert_path need to be passed as arg")
}

func getClientsKeysPath() (string, error){
	if *clientKeysPath!="" {
		return *clientKeysPath, nil
	}
	cwd, err := os.Getwd(); if err!=nil {
		return "",err
	}
	if strings.Contains(cwd, "/featureprofiles/"){
		rootSrc:=strings.Split(cwd, "featureprofiles")[0]
		return rootSrc + "featureprofiles/internal/cisco/security/cert/keys/clients/", nil
	}
	return "",fmt.Errorf("ca_cert_path need to be passed as arg")
}

func init() {
	// read the CA keys from keys/ca and generate it if not found
	var err error
	caPrivateKey, CACert, err = loadRootCA()
	if err != nil {
		caPrivateKey, CACert, err = genRootCA()
		if err != nil {
			panic(fmt.Sprintf("Could load the CA keys, Error: %v", err))
		}
		fmt.Println(caPrivateKey, CACert)
	}

}

// GenCERT Generates RSA keys and sign the public key with root ca certificate
func GenCERT(cn string, expireInDays int, ips []net.IP, spiffe string, path string) (*rsa.PrivateKey, *x509.Certificate, error) {
	// set up our server certificate
	uris := []*url.URL{}
	if spiffe != "" {
		uri, err := url.Parse(spiffe)
		if err != nil {
			uris = append(uris, uri)
		}
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(big.MaxBase))
	if err != nil {
		return nil, nil, err
	}
	certSpec := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{"OpenConfig"},
			Country:      []string{"US"},
		},
		IPAddresses: ips,
		URIs:        uris,
		DNSNames:    []string{spiffe},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(0, 0, expireInDays),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, certSpec, CACert, &privKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, nil, err
	}

	keyPath := path
	if keyPath ==""{
		keyPath, _ = getClientsKeysPath()
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	if keyPath!="" {
		os.WriteFile(keyPath+"/"+cn+".cert.pem", certPEM.Bytes(), 0444)
	}

	privKeyPEM := new(bytes.Buffer)
	pem.Encode(privKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})
	if keyPath!="" {
		os.WriteFile(keyPath+"/"+cn+".key.pem", privKeyPEM.Bytes(), 0400)
	}
	// To ensure pem is correct.
	cert, err := x509.ParseCertificate(certPEM.Bytes())
	if err != nil {
		return nil, nil, err
	}
	return privKey, cert, nil
}

func loadRootCA() (*rsa.PrivateKey, *x509.Certificate, error) {
	caCertPath, err := getCACertPath(); if err!=nil {
		return nil, nil, err
	}
	caPrivateKeyBytes, err := os.ReadFile(caCertPath + "/"+caKeyFileName)
	if err != nil {
		return nil, nil, err
	}
	caPrivatePem, _ := pem.Decode(caPrivateKeyBytes)
	if caPrivatePem == nil {
		return nil, nil, fmt.Errorf("error in loading private key")
	}
	caPrivateKey, err := x509.ParsePKCS1PrivateKey(caPrivatePem.Bytes)
	if err != nil {
		return nil, nil, err
	}

	caCertBytes, err := os.ReadFile(caCertPath + "/"+caCertFileName)
	if err != nil {
		return nil, nil, err
	}
	caCertPem, _ := pem.Decode(caCertBytes)
	if caCertPem == nil {
		return nil, nil, fmt.Errorf("error in loading ca cert")
	}
	caCert, err := x509.ParseCertificate(caCertPem.Bytes)
	if err != nil {
		return nil, nil, err
	}
	return caPrivateKey, caCert, nil
}
func genRootCA() (*rsa.PrivateKey, *x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, big.NewInt(9999999999999999))
	if err != nil {
		return nil, nil, err
	}
	ca := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"OpenConfig"},
			Country:      []string{"US"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1000, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create our private and public key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	// create the CA
	caCertBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	// pem encode
	caCertPEM := new(bytes.Buffer)
	pem.Encode(caCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertBytes,
	})
	caCertPath, err := getCACertPath(); if err!=nil {
		return nil, nil, err
	}
	if err := os.WriteFile(caCertPath+"/"+caCertFileName, caCertPEM.Bytes(), 0444); err != nil {
		return nil, nil, err
	}

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})
	if err := os.WriteFile(caCertPath+"/"+caKeyFileName, caPrivKeyPEM.Bytes(), 0400); err != nil {
		return nil, nil, err
	}
	// parsing it from pem to ensure the saved PEM is ok
	caCert, err := x509.ParseCertificate(caCertPEM.Bytes())
	if err != nil {
		return nil, nil, err
	}
	return caPrivKey, caCert, nil
}
