package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"os"
	"testing"

	"github.com/openconfig/featureprofiles/internal/security/authz"
	"github.com/openconfig/featureprofiles/internal/security/svid"
	"github.com/openconfig/ondatra"
)

type UsersMap map[string]authz.Spiffe

var (
	testInfraID = flag.String("test_infra_id", "cafyauto", "SPIFFE-ID used by test Infra ID user for authz operation")
	caCertPem   = flag.String("ca_cert_pem", "testdata/ca.cert.pem", "a pem file for ca cert that will be used to generate svid")
	caKeyPem    = flag.String("ca_key_pem", "testdata/ca.key.pem", "a pem file for ca key that will be used to generate svid")
	policyMap   map[string]authz.AuthorizationPolicy

	usersMap = UsersMap{
		"cert_user_admin": {
			ID: "spiffe://test-abc.foo.bar/xyz/admin",
		},
		"cert_deny_all": {
			ID: "spiffe://test-abc.foo.bar/xyz/deny-all", // a user with valid svid but no permission (has a deny rule for target *)
		},
		"cert_gribi_modify": {
			ID: "spiffe://test-abc.foo.bar/xyz/gribi-modify",
		},
		"cert_gnmi_set": {
			ID: "spiffe://test-abc.foo.bar/xyz/gnmi-set",
		},
		"cert_gnoi_time": {
			ID: "spiffe://test-abc.foo.bar/xyz/gnoi-time",
		},
		"cert_gnoi_ping": {
			ID: "spiffe://test-abc.foo.bar/xyz/gnoi-ping",
		},
		"cert_gnsi_probe": {
			ID: "spiffe://test-abc.foo.bar/xyz/gnsi-probe",
		},
		"cert_read_only": {
			ID: "spiffe://test-abc.foo.bar/xyz/read-only",
		},
	}
)

func setUpBaseline(t *testing.T, dut *ondatra.DUTDevice) {

	policyMap = authz.LoadPolicyFromJSONFile(t, "testdata/policy.json")

	caKey, trustBundle, err := svid.LoadKeyPair(*caKeyPem, *caCertPem)
	if err != nil {
		t.Fatalf("Could not load ca key/cert: %v", err)
	}
	caCertBytes, err := os.ReadFile(*caCertPem)
	if err != nil {
		t.Fatalf("Could not load the ca cert: %v", err)
	}
	trusBundle := x509.NewCertPool()
	if !trusBundle.AppendCertsFromPEM(caCertBytes) {
		t.Fatalf("Could not create the trust bundle: %v", err)
	}
	for user, v := range usersMap {
		userSvid, err := svid.GenSVID("", v.ID, 300, trustBundle, caKey, x509.RSA)
		if err != nil {
			t.Fatalf("Could not generate svid for user %s: %v", user, err)
		}
		tlsConf := tls.Config{
			Certificates: []tls.Certificate{*userSvid},
			RootCAs:      trusBundle,
		}
		usersMap[user] = authz.Spiffe{
			ID:      v.ID,
			TLSConf: &tlsConf,
		}
	}
}

func getSpiffe(t *testing.T, dut *ondatra.DUTDevice, certName string) *authz.Spiffe {
	spiffe, ok := usersMap[certName]
	if !ok {
		t.Fatalf("Could not find Spiffe ID for user %s", certName)
	}
	return &spiffe
}

// func TestAuthzScaled(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	setUpBaseline(t, dut)
// 	ControlPlaneVerification(ygnmiCli)
// }
//
// func TestAuthz(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	setUpBaseline(t, dut)
// 	ControlPlaneVerification(ygnmiCli)
// }
//
// func TestAuthzEmsdRestart(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	setUpBaseline(t, dut)
// 	RestartEmsd(t, dut)
// 	ControlPlaneVerification(ygnmiCli)
// }
//
// func TestAuthzRouterReload(t *testing.T) {
// 	dut := ondatra.DUT(t, "dut")
// 	setUpBaseline(t, dut)
// 	RouterReload(t, dut)
// 	ControlPlaneVerification(ygnmiCli)
// }
