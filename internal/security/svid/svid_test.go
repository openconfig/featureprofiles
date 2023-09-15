// Package cert provides functions to generate and load  certificates.
package svid

import (
	"crypto/x509"

	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/h-fam/errdiff"
)

var (
	caRSACERT = `-----BEGIN CERTIFICATE-----
MIIFUDCCAzigAwIBAgIHEsH2uK4gwDANBgkqhkiG9w0BAQsFADA2MQswCQYDVQQG
EwJVUzETMBEGA1UEChMKT3BlbkNvbmZpZzESMBAGA1UEAxMJbG9jYWxob3N0MCAX
DTIzMDcxNzE4MDIwMVoYDzMwMjMwNzE3MTgwMjAxWjA2MQswCQYDVQQGEwJVUzET
MBEGA1UEChMKT3BlbkNvbmZpZzESMBAGA1UEAxMJbG9jYWxob3N0MIICIjANBgkq
hkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAw+CN6Ryym3wExln3ZDGODa0REmTm6MmV
jUL8yqdm35+i5/SIV+oI8e8LkXa1ccvKSCvN3s1ESimiYyltJ+GcgiA5b+pDndd9
e0RHhlnx2NmGau/MW+9dkwhmNniG/GBdvBxJTL43T3pR53HZafdvjNnEH6U7aBFC
4IHwXYF5Mw+B0JEldnHsYi+tGKQcBqMpCCZn5k3jyCE/dYBFmnQwr9Z6FlN53hKl
19YoT9o4NBFCZ42rf22oWieCBQMjTHSCSL+bKBxfF9abefyn+SHElgw5iJ9oiylA
Q4dpdbKHWOEFEPO5Hx4MdmUcx15o08xpg1Z2yk1dZff8GMFGO4EY2HXq0MoU4OMV
QOcl/JhGqmjxNA42hW50DY/qU2KBlS9m/AJg/e1zYXNA4MkgTTaO04m92+OnMeBO
/ZgEs8KfrmYox6TG4xAAr6WN+L4rafmo1q+Yp/dJD6kRJ32yM9b+O+ovVnFQYHDV
1Td8XDamQLRELpgsjvVLG4I4mMuCEcKrixI6PYNdslE2SES3f1xH9sv/z4YvxZ19
cOoWT1f5/4Shzv6GuyRfa1gf7vDZVM2SZmzxNzkAQ6iRAbY0ft4t0DsxmCnNTo3W
h1zXUxIHjfDiwF9LX3JK0t+LL8PKx0V3z4alj5zDE800RXdoQo4eqvCkjmge4oRg
sR06F0VnK9ECAwEAAaNhMF8wDgYDVR0PAQH/BAQDAgKEMB0GA1UdJQQWMBQGCCsG
AQUFBwMCBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBQKqJpq
IzSuypYALPMEOBTj1I13vTANBgkqhkiG9w0BAQsFAAOCAgEAY+URdI3aRzFwAvv4
xplwRKFphzunbm/HqJT43GJV2mbXsaheEscXGyxiYtKUlBJzR9BNXbrlM4w61x0c
wR1+Yw6UfSab/64a4OGf0xoJxyG28GofIe31NtX4LK90EH6sFop1exCfQ5Edybao
ebokZAn67EJy5A1G672bnPSDpxVsfILw/83OM+T231Oz0l8nr1Zde6chiVDrOHa5
x3ww+Ex0HjN8PDreAw2SYCO73KjjuwrjIbEAaLQ0+4Myd+Mb4UbP8biNYCfBkole
WaTNumBfYethCrSOOCYjcJv7PBujsO/qx1EJatY16V8u1jTQ29AqOW+lAkMt/igU
bV4SKZyRmMJ6dpMz9c5K6jGYySlPQeSHESi2pAO7O9NQ8smY/e+4v+puKL65jgHB
fzZfAxI8Ur7Bpf6mKJlJgnoH4phIr86l2lEXnI/Lb9CUsKTNQZ8rqlO3KJqT9H8y
7RBNfctB+McKZryKlc0+42SlnDIQdWfbb1fq6cAu+LjJGdgjeRG0/W636dFAsVQm
IghmGukwCDwFYKA5pdti251HpfltrlcbOe0L0pf4XltH1fXatqwNk2YIF8wKeNuR
QuwWEdjeXOOhqbT8uYtjbhvLCKqYN3tKUWzk53T2SF7eJuk71uEeVIeIQxu/P369
7qD0V39Ms16pWDivpaSGn7EKFJw=
-----END CERTIFICATE-----`

	caRSAKey = `-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----
`
	ecDSACert = `-----BEGIN CERTIFICATE-----
MIICDzCCAbWgAwIBAgIUFuc4cVr+eMKVi3kpWvxdOr++cIMwCgYIKoZIzj0EAwIw
XTEQMA4GA1UEAwwHQ0EgMDAwMTELMAkGA1UEBhMCQVExCzAJBgNVBAgMAk5aMQsw
CQYDVQQHDAJOWjEiMCAGA1UECgwZT3BlbkNvbmZpZ0ZlYXR1cmVQcm9maWxlczAe
Fw0yMzA5MDEyMDQ4MDBaFw0zMzA4MjkyMDQ4MDBaMF0xEDAOBgNVBAMMB0NBIDAw
MDExCzAJBgNVBAYTAkFRMQswCQYDVQQIDAJOWjELMAkGA1UEBwwCTloxIjAgBgNV
BAoMGU9wZW5Db25maWdGZWF0dXJlUHJvZmlsZXMwWTATBgcqhkjOPQIBBggqhkjO
PQMBBwNCAATQkV4pYqWEz2rbYUOpzqk2Bl/4yzXNcY5miUqJBepqYVuKWIqYox99
H2himPXn5hdMjJ5cHF13r5aiyzGP8Yczo1MwUTAdBgNVHQ4EFgQUmzc7ZL0h1rAI
IynH2tedIUZqP/8wHwYDVR0jBBgwFoAUmzc7ZL0h1rAIIynH2tedIUZqP/8wDwYD
VR0TAQH/BAUwAwEB/zAKBggqhkjOPQQDAgNIADBFAiB+70t0pUuEhwsCQIVK4Dky
EpyC2Y2erZ3KFTIdk6eaVQIhAPwQlW/ftf+sx+lJQopdhA5isjBVKdEsNROOb0yu
dK7N
-----END CERTIFICATE-----`

	ecDSAKey = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIC1DkU/pf5kg0crdLphKKIif2UNvqcfCRMmIH35LJNvKoAoGCCqGSM49
AwEHoUQDQgAE0JFeKWKlhM9q22FDqc6pNgZf+Ms1zXGOZolKiQXqamFbiliKmKMf
fR9oYpj15+YXTIyeXBxdd6+Wossxj/GHMw==
-----END EC PRIVATE KEY-----`
)

func TestGenRSASVID(t *testing.T) {
	tests := []struct {
		name         string
		caCert       string
		caKey        string
		username     string
		err          string
		keyAlgorithm x509.PublicKeyAlgorithm
		uris         []*url.URL
	}{
		{
			name:         "Successful SVID with RSA certificate",
			username:     "test",
			err:          "",
			keyAlgorithm: x509.RSA,
			caKey:        caRSAKey,
			caCert:       caRSACERT,
			uris: []*url.URL{
				{
					Scheme: "",
					Host:   "",
					Path:   "test",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caPrivateKey, CACert, err := loadKeyPair([]byte(test.caKey), []byte(test.caCert))
			if err != nil {
				t.Fatalf("Unexpected Error, %v", err)
			}
			key, cert, err := GenRSASVID(test.username, 300, CACert, caPrivateKey)
			if errdiff.Substring(err, test.err) != "" {
				t.Fatalf("Unexpected Error, want: %s, got %v", test.err, err)
			}
			if key == nil || cert == nil {
				t.Fatalf("Key and CERT must not be nil")
			}
			if cert.PublicKeyAlgorithm != test.keyAlgorithm {
				t.Fatalf("KeyAlgorithm mismatch, got %s, wanted %s", cert.PublicKeyAlgorithm.String(), test.keyAlgorithm.String())
			}
			if cert.Subject.CommonName != test.username {
				t.Errorf("Common name is not as expected, want: %s, got:%s", test.username, cert.Subject.CommonName)
			}
			opts := []cmp.Option{cmpopts.IgnoreUnexported(*test.uris[0])}
			if !cmp.Equal(test.uris, cert.URIs, opts...) {
				t.Errorf("URIs are not as expected, Diff: %s", cmp.Diff(test.uris, cert.URIs, opts...))
			}

		})
	}
}

func TestGenECDSASVID(t *testing.T) {
	tests := []struct {
		name         string
		caCert       string
		caKey        string
		username     string
		err          string
		keyAlgorithm x509.PublicKeyAlgorithm
		uris         []*url.URL
	}{
		{
			name:         "Successful SVID with ECDSA certificate",
			username:     "spiffe://test-abc.foo.bar/xyz/admin",
			err:          "",
			caCert:       ecDSACert,
			caKey:        ecDSAKey,
			keyAlgorithm: x509.ECDSA,
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
			caPrivateKey, CACert, err := loadKeyPair([]byte(test.caKey), []byte(test.caCert))
			if err != nil {
				t.Fatalf("Unexpected Error, %v", err)
			}
			key, cert, err := GenECDSASVID(test.username, 300, CACert, caPrivateKey)
			if errdiff.Substring(err, test.err) != "" {
				t.Fatalf("Unexpected Error, want: %s, got %v", test.err, err)
			}
			if key == nil || cert == nil {
				t.Fatalf("Key and CERT must not be nil")
			}
			if cert.PublicKeyAlgorithm != test.keyAlgorithm {
				t.Fatalf("KeyAlgorithm mismatch, got %s, wanted %s", cert.PublicKeyAlgorithm.String(), test.keyAlgorithm.String())
			}
			if cert.Subject.CommonName != test.username {
				t.Errorf("Common name is not as expected, want: %s, got:%s", test.username, cert.Subject.CommonName)
			}
			opts := []cmp.Option{cmpopts.IgnoreUnexported(*test.uris[0])}
			if !cmp.Equal(test.uris, cert.URIs, opts...) {
				t.Errorf("URIs are not as expected, Diff: %s", cmp.Diff(test.uris, cert.URIs, opts...))
			}
		})
	}
}
