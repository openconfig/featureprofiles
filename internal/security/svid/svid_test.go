// Package cert provides functions to generate and load  certificates.
package svid

import (
	"crypto/x509"
	
	"net/url"
	"testing"

	"github.com/h-fam/errdiff"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)




func TestGenRSASVID(t *testing.T) {
	tests:=[]struct{
		name string
		caCert string
		caKey  string
		username string
		err string
		keyAlgorithm x509.PublicKeyAlgorithm
		uris []*url.URL
	}{
		{
			name: "Successful SVID with RSA certificate",
			username: "test",
			err: "",
			caCert: "../testdata/rsa/ca-rsa-cert.pem",
			caKey: "../testdata/rsa/ca-rsa-key.pem",
			keyAlgorithm: x509.RSA,
			uris : []*url.URL{
				{
					Scheme:"" ,
					Host: "", 
					Path: "test",
				},
			},
		},
	}

	for _,test:= range tests {
		t.Run(test.name, func(t *testing.T) {
			caPrivateKey, CACert, err := loadKeyPair(test.caKey, test.caCert) ; if err!=nil {
				t.Fatalf("Unexpected Error, %v", err)	
			}
			key, cert, err:=GenRSASVID(test.username,300,CACert,caPrivateKey); if errdiff.Substring(err,test.err)!=""{
				t.Fatalf("Unexpected Error, want: %s, got %v", test.err, err)	
			}
			if key==nil || cert==nil {
				t.Fatalf("Key and CERT must not be nil")	
			}
			if cert.PublicKeyAlgorithm!=test.keyAlgorithm {
				t.Fatalf("KeyAlgorithm mismatch, got %s, wanted %s", cert.PublicKeyAlgorithm.String(), test.keyAlgorithm.String())
			}
			if cert.Subject.CommonName!=test.username {
				t.Errorf("Common name is not as expected, want: %s, got:%s",test.username, cert.Subject.CommonName)
			}
			opts := []cmp.Option{cmpopts.IgnoreUnexported(*test.uris[0])}
			if !cmp.Equal(test.uris, cert.URIs, opts...) {
				t.Errorf("URIs are not as expected, Diff: %s", cmp.Diff(test.uris, cert.URIs, opts...))
			}

		})
	}
}

func TestGenECDSASVID(t *testing.T) {
	tests:=[]struct{
		name string
		caCert string
		caKey  string
		username string
		err string
		keyAlgorithm x509.PublicKeyAlgorithm
		uris []*url.URL
	}{
		{
			name: "Successful SVID with ECDSA certificate",
			username: "spiffe://test-abc.foo.bar/xyz/admin",
			err: "",
			caCert: "../testdata/ecdsa/ca-ecdsa-cert.pem",
			caKey:  "../testdata/ecdsa/ca-ecdsa-key.pem",
			keyAlgorithm: x509.ECDSA,
			uris : []*url.URL{
				{
					Scheme:"spiffe" ,
					Host: "test-abc.foo.bar", 
					Path: "/xyz/admin",
				},
			},
		},
	}

	for _,test:= range tests {
		t.Run(test.name, func(t *testing.T) {
			caPrivateKey, CACert, err := loadKeyPair(test.caKey, test.caCert) ; if err!=nil {
				t.Fatalf("Unexpected Error, %v", err)	
			}
			key, cert, err:=GenECDSASVID(test.username,300,CACert,caPrivateKey); if errdiff.Substring(err,test.err)!=""{
				t.Fatalf("Unexpected Error, want: %s, got %v", test.err, err)	
			}
			if key==nil || cert==nil {
				t.Fatalf("Key and CERT must not be nil")	
			}
			if cert.PublicKeyAlgorithm!=test.keyAlgorithm {
				t.Fatalf("KeyAlgorithm mismatch, got %s, wanted %s", cert.PublicKeyAlgorithm.String(), test.keyAlgorithm.String())
			}
			if cert.Subject.CommonName!=test.username {
				t.Errorf("Common name is not as expected, want: %s, got:%s",test.username, cert.Subject.CommonName)
			}
			opts := []cmp.Option{cmpopts.IgnoreUnexported(*test.uris[0])}
			if !cmp.Equal(test.uris, cert.URIs, opts...) {
				t.Errorf("URIs are not as expected, Diff: %s", cmp.Diff(test.uris, cert.URIs, opts...))
			}
		})
	}
}
