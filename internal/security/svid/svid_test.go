// Package cert provides functions to generate and load  certificates.
package svid

import (
	"crypto/x509"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/gnmi/errdiff"
)

func TestGenRSASVID(t *testing.T) {
	tests := []struct {
		name         string
		caCert       string
		caKey        string
		commonName   string
		spiffeID     string
		errStr       string
		keyAlgorithm x509.PublicKeyAlgorithm
		uris         []*url.URL
	}{
		{
			name:         "Successful SVID with RSA certificate",
			commonName:   "test",
			spiffeID:     "spiffe://test-abc.foo.bar/xyz/admin",
			errStr:       "",
			keyAlgorithm: x509.RSA,
			caKey:        "testdata/rsa/ca-rsa-key.pem",
			caCert:       "testdata/rsa/ca-rsa-cert.pem",
			uris: []*url.URL{
				{
					Scheme: "spiffe",
					Host:   "test-abc.foo.bar",
					Path:   "/xyz/admin",
				},
			},
		},
		{
			name:         "Successful SVID with ECDSA certificate",
			commonName:   "test",
			spiffeID:     "spiffe://test-abc.foo.bar/xyz/admin",
			errStr:       "",
			keyAlgorithm: x509.ECDSA,
			caKey:        "testdata/ecdsa/ca-ecdsa-key.pem",
			caCert:       "testdata/ecdsa/ca-ecdsa-cert.pem",
			uris: []*url.URL{
				{
					Scheme: "spiffe",
					Host:   "test-abc.foo.bar",
					Path:   "/xyz/admin",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caPrivateKey, CACert, err := LoadKeyPair(test.caKey, test.caCert)
			if err != nil {
				t.Fatalf("Unexpected Error, %v", err)
			}
			cert, err := GenSVID(test.commonName, test.spiffeID, 300, CACert, caPrivateKey, test.keyAlgorithm)
			if diff := errdiff.Substring(err, test.errStr); diff != "" {
				t.Fatalf("Unexpected Error, %v", diff)
			}
			if test.errStr == "" && cert == nil {
				t.Fatalf("Cert must not be nil")
			}
			if cert.Leaf.PublicKeyAlgorithm != test.keyAlgorithm {
				t.Fatalf("KeyAlgorithm mismatch, got %s, wanted %s", x509.PublicKeyAlgorithm(cert.Leaf.SignatureAlgorithm).String(), test.keyAlgorithm.String())
			}
			if cert.Leaf.Subject.CommonName != test.commonName {
				t.Errorf("Common name is not as expected, want: %s, got:%s", test.commonName, cert.Leaf.Issuer.CommonName)
			}
			opts := []cmp.Option{cmpopts.IgnoreUnexported(*test.uris[0])}
			if !cmp.Equal(test.uris, cert.Leaf.URIs, opts...) {
				t.Errorf("URIs are not as expected, Diff: %s", cmp.Diff(test.uris, cert.Leaf.URIs, opts...))
			}

		})
	}
}
