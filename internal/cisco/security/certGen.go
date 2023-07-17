package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"

	"io/ioutil"
	"math/big"
	"net"

	//"net/http"
	//"net/http/httptest"
	//"strings"
	"time"

)

var (
	caPrivateKey *rsa.PrivateKey
	caCert *x509.Certificate
)
func main() {
	// read the CA keys from keys/ca and generate it if not found
	var err error
	caPrivateKey,caCert,err= loadRootCA(); if err!=nil {
		caPrivateKey,caCert, err=genRootCA(); if err!= nil {
			panic("Could load the CA keys")
		}
		fmt.Println(caPrivateKey,caCert)
	}
	genCERT("Moji_SFD", 100)
}

func genCERT(cn string, expireInDays int) (*rsa.PrivateKey, *x509.Certificate, error) {
	// set up our server certificate
	serial, err := rand.Int(rand.Reader,big.NewInt(9999999999999999)); if err!=nil {
		return nil,nil, err
	}
	certSpec := &x509.Certificate{
		SerialNumber: 	serial,
		Subject: pkix.Name{
			CommonName: cn,
			Organization:  []string{"OpenConfig"},
			Country:       []string{"US"},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, expireInDays),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}
	//parsedCaCert, err := x509.ParseCertificate(caCert)
	/*caTrusBundle := x509.NewCertPool()	
	if ! caTrusBundle.AppendCertsFromPEM(caCert) {
		return  nil,nil, fmt.Errorf("error in loading ca cert")
	}*/


	certBytes, err := x509.CreateCertificate(rand.Reader, certSpec, caCert, &privKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	if err:=ioutil.WriteFile("keys/nodes/"+cn+".cert.pem",certPEM.Bytes(),0444); err!=nil {
		return nil,nil,err
	}

	privKeyPEM := new(bytes.Buffer)
	pem.Encode(privKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})
	if err:=ioutil.WriteFile("keys/nodes/"+cn+".key.pem",privKeyPEM.Bytes(),0400); err!=nil {
		return nil,nil,err
	}
	// To ensure pem is correct. 
	cert, err:= x509.ParseCertificate(certPEM.Bytes()); if err!=nil {
		return nil,nil,err
	}
	return privKey,cert,nil
}

func loadRootCA() (*rsa.PrivateKey, *x509.Certificate, error) {
	caPrivateKeyBytes, err := ioutil.ReadFile("keys/CA/ca.key.pem"); if err!=nil {
		return nil,nil,err
	}
	caPrivatePem, _ := pem.Decode(caPrivateKeyBytes)	; if caPrivatePem==nil {
		return  nil,nil, fmt.Errorf("error in loading private key")
	}
	caPrivateKey, err := x509.ParsePKCS1PrivateKey(caPrivatePem.Bytes); if err!=nil {
		return nil,nil,err
	}

	caCertBytes, err := ioutil.ReadFile("keys/CA/ca.cert.pem"); if err!=nil {
		return nil,nil,err
	}
	caCertPem, _ := pem.Decode(caCertBytes)	; if caCertPem==nil {
		return  nil,nil, fmt.Errorf("error in loading ca cert")
	}
    /*trusBundle := x509.NewCertPool()	
	if ! trusBundle.AppendCertsFromPEM(caCertBytes) {
		return  nil,nil, fmt.Errorf("error in loading ca cert")
	}*/
	caCert, err:= x509.ParseCertificate(caCertPem.Bytes); if err!=nil {
		return nil,nil,err
	}
	return caPrivateKey,caCert,nil
}
func genRootCA() (*rsa.PrivateKey, *x509.Certificate, error){
	serial, err := rand.Int(rand.Reader,big.NewInt(9999999999999999)); if err!=nil {
		return nil,nil, err
	}
	ca := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:     "OpenConfig",
			Organization:  []string{"OpenConfig"},
			Country:       []string{"US"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create our private and public key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil,nil,err
	}

	// create the CA
	caCertBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil,nil, err
	}

	// pem encode
	caCertPEM := new(bytes.Buffer)
	pem.Encode(caCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCertBytes,
	})
	if err:=ioutil.WriteFile("keys/CA/ca.cert.pem",caCertPEM.Bytes(),0444); err!=nil {
		return nil,nil,err
	}


	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})
	if err:=ioutil.WriteFile("keys/CA/ca.key.pem",caPrivKeyPEM.Bytes(),0400); err!=nil {
		return nil,nil,err
	}
	// parsing it from pem to ensure the saved PEM is ok
	caCert, err:= x509.ParseCertificate(caCertPEM.Bytes()); if err!=nil {
		return nil,nil,err
	}
	return caPrivKey,caCert,nil
}

func certsetup() (serverTLSConf *tls.Config, clientTLSConf *tls.Config, err error) {
	// set up our CA certificate
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"Golden Gate Bridge"},
			PostalCode:    []string{"94016"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
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
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	// pem encode
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	// set up our server certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Company, INC."},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"San Francisco"},
			StreetAddress: []string{"Golden Gate Bridge"},
			PostalCode:    []string{"94016"},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})
 
	if err:=ioutil.WriteFile("CA/ca.key.pem",certPrivKeyPEM.Bytes(),0400); err!=nil {
		panic("Could not save the CA key")
	}

	return
}