// Package pathz_test implements the gNSI Pathz path-level authorization tests (Pathz-1 to Pathz-4).
package pathz_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnsi/certz"
	"github.com/openconfig/gnsi/pathz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding/introspect"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/helpers"
)

const (
	roleAdmin               = "admin"
	roleReader              = "reader"
	roleUnauthorized        = "unauthorized"
	certAdmin               = "gnmi_admin"
	certReader              = "gnmi_reader"
	certUnauthorized        = "gnmi_unauthorized"
	policyVersionV1         = "v1"
	hostnamePath            = "/system/config/hostname"
	interfacesPath          = "/interfaces/interface"
	interfaceDescPath       = "/interfaces/interface[name=%s]/config/description"
	caCommonName            = "pathz-test-ca"
	sslProfileID            = "pathz-test-profile"
	serverCertVersion       = "server-v1"
	trustBundleVersion      = "trust-v1"
	certValidity            = 30 * 24 * time.Hour
	certzCreatedOnV1        = uint64(100)
	pathzSandboxVersionPath = "/system/gnmi-pathz-policies/policies/policy[instance=SANDBOX]/state/version"
)

// policyCreatedOn* are set once at startup to the current time (nanoseconds since epoch) so each
// test run uploads a fresh created_on; V2 is later than V1 to reflect the newer sandbox policy.
// var (
// 	policyCreatedOnV1 = uint64(time.Now().UnixNano())
// )

var (
	policyCreatedOnV1 = uint64(time.Now().UnixNano())

	// The JSON templates below are the README's enforcement policies verbatim (SPIFFE identities).
	// The DUT_PORT placeholder in the baseline policy is filled at runtime with the discovered port.
	baselinePolicyTemplate = `{
  "rules": [
    {
      "id": "allow-reader-read-system",
      "user": "%[1]s",
      "path": {"elem": [{"name": "system"}]},
      "action": "ACTION_PERMIT",
      "mode": "MODE_READ"
    },
    {
      "id": "deny-reader-write-system",
      "user": "%[1]s",
      "path": {"elem": [{"name": "system"}]},
      "action": "ACTION_DENY",
      "mode": "MODE_WRITE"
    },
    {
      "id": "allow-admin-write-interfaces",
      "group": "admin-group",
      "path": {"elem": [{"name": "interfaces"}, {"name": "interface"}]},
      "action": "ACTION_PERMIT",
      "mode": "MODE_WRITE"
    },
    {
      "id": "deny-admin-write-port1",
      "user": "%[2]s",
      "path": {"elem": [{"name": "interfaces"}, {"name": "interface", "key": {"name": "%[3]s"}}]},
      "action": "ACTION_DENY",
      "mode": "MODE_WRITE"
    }
  ],
  "groups": [
    {"name": "admin-group", "users": [{"name": "%[2]s"}]}
  ]
}`

	denyReaderSystemPolicy = `{
  "rules": [
    {
      "id": "deny-reader-all-system",
      "user": "%[1]s",
      "path": {"elem": [{"name": "system"}]},
      "action": "ACTION_DENY",
      "mode": "MODE_READ"
    }
  ]
}`
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// spiffeIDForRole returns the SPIFFE ID to use for the given role, in the format expected by
// the DUT's vendor.
func spiffeIDForRole(dut *ondatra.DUTDevice, role string) string {
	return fmt.Sprintf("spiffe://test-realm.foo.bar/role/%s", role)
}

// clientIdentities returns the SPIFFE ID to provision for each client certificate, keyed by
// certificate name.
func clientIdentities(dut *ondatra.DUTDevice) map[string]string {
	return map[string]string{
		certAdmin:        spiffeIDForRole(dut, roleAdmin),
		certReader:       spiffeIDForRole(dut, roleReader),
		certUnauthorized: spiffeIDForRole(dut, roleUnauthorized),
	}
}

// generateCA creates a self-signed ECDSA CA certificate used to sign the test's client and
// server certificates, and returns the parsed cert, its private key, and its DER encoding.
func generateCA() (*x509.Certificate, *ecdsa.PrivateKey, []byte, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate ca key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate ca serial number: %w", err)
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: caCommonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(certValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create ca certificate: %w", err)
	}
	caCert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse ca certificate: %w", err)
	}
	return caCert, caKey, der, nil
}

// generateClientCert issues a CA-signed client certificate carrying the given SPIFFE ID as a
// URI SAN, for use as a role identity (admin/reader/unauthorized/discovery) in mTLS dials.
func generateClientCert(caCert *x509.Certificate, caKey *ecdsa.PrivateKey, spiffeID string) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate client key for %s: %w", spiffeID, err)
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate client serial number for %s: %w", spiffeID, err)
	}
	spiffeURI, err := url.Parse(spiffeID)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to parse spiffe id %s: %w", spiffeID, err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: spiffeID},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(certValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		URIs:         []*url.URL{spiffeURI},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create client certificate for %s: %w", spiffeID, err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to marshal client key for %s: %w", spiffeID, err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	clientCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to build tls certificate for %s: %w", spiffeID, err)
	}
	return clientCert, nil
}

// provisionMTLSIdentities generates the test CA and a client certificate for every role identity
// returned by clientIdentities, returning the CA, its key, its PEM encoding, and the certs by name.
func provisionMTLSIdentities(dut *ondatra.DUTDevice) (*x509.Certificate, *ecdsa.PrivateKey, []byte, map[string]tls.Certificate, error) {
	caCert, caKey, caDER, err := generateCA()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	identities := make(map[string]tls.Certificate)
	for certName, spiffeID := range clientIdentities(dut) {
		clientCert, err := generateClientCert(caCert, caKey, spiffeID)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		identities[certName] = clientCert
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	return caCert, caKey, caPEM, identities, nil
}

// generateServerCert issues a CA-signed server certificate for ipAddress, for use as the DUT's
// gRPC server certificate once pushed via Certz. The IP is carried as an IP SAN (not a DNS SAN).
func generateServerCert(caCert *x509.Certificate, caKey *ecdsa.PrivateKey,
	ipAddress string) (certPEM, keyPEM []byte, err error) {
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return nil, nil, fmt.Errorf("failed to parse server ip address %q", ipAddress)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate server key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate server serial number: %w", err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: ipAddress},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(certValidity),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{ip},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create server certificate: %w", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal server key: %w", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}

// dutIPFromBinding resolves the DUT's IP address from the Ondatra binding file by reading the
// dial target the binding configures for the gNMI service (host:port) and returning the host
// portion. This avoids hardcoding an IP: whatever address the binding is configured to reach
// the DUT on is the address the server certificate is issued for.
func dutIPFromBinding(t *testing.T, dut *ondatra.DUTDevice) (string, error) {
	t.Helper()
	dialer := introspect.DUTDialer(t, dut, introspect.GNMI)
	host, _, err := net.SplitHostPort(dialer.DialTarget)
	if err != nil {
		return "", fmt.Errorf("failed to parse dut dial target %q: %w", dialer.DialTarget, err)
	}
	if net.ParseIP(host) == nil {
		return "", fmt.Errorf("dut dial target %q does not resolve to an ip address (got %q); check the binding file", dialer.DialTarget, host)
	}
	return host, nil
}

// dialCertzClient returns a gNSI Certz client bound to the DUT under the test framework's
// default (pre-mTLS) credentials, for use only before mTLS enforcement is turned on.
func dialCertzClient(dut *ondatra.DUTDevice) (certz.CertzClient, error) {
	gnsiClients, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to dial gnsi service for certz: %w", err)
	}
	return gnsiClients.Certz(), nil
}

// rotateServerProfile pushes the given server certificate and CA trust bundle to a Certz SSL
// profile (creating the profile first if needed), finalizing the rotation.
func rotateServerProfile(ctx context.Context, client certz.CertzClient, profileID string,
	serverCertPEM, serverKeyPEM, caPEM []byte) error {
	// A missing profile is fine here (Arista returns FailedPrecondition/NotFound); we recreate it.
	if _, err := client.DeleteProfile(ctx, &certz.DeleteProfileRequest{SslProfileId: profileID}); err != nil {
		if code := status.Code(err); code != codes.NotFound && code != codes.FailedPrecondition {
			return fmt.Errorf("failed to delete existing ssl profile %q: %w", profileID, err)
		}
	}
	if _, err := client.AddProfile(ctx, &certz.AddProfileRequest{SslProfileId: profileID}); err != nil {
		if code := status.Code(err); code != codes.AlreadyExists && code != codes.FailedPrecondition {
			return fmt.Errorf("failed to add ssl profile %q: %w", profileID, err)
		}
	}
	stream, err := client.Rotate(ctx)
	if err != nil {
		return fmt.Errorf("failed to open certz rotate stream: %w", err)
	}
	uploadReq := &certz.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profileID,
		RotateRequest: &certz.RotateCertificateRequest_Certificates{
			Certificates: &certz.UploadRequest{
				Entities: []*certz.Entity{
					{
						Version:   serverCertVersion,
						CreatedOn: certzCreatedOnV1,
						Entity: &certz.Entity_CertificateChain{
							CertificateChain: &certz.CertificateChain{
								Certificate: &certz.Certificate{
									Type:            certz.CertificateType_CERTIFICATE_TYPE_X509,
									Encoding:        certz.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
									CertificateType: &certz.Certificate_RawCertificate{RawCertificate: serverCertPEM},
									PrivateKeyType:  &certz.Certificate_RawPrivateKey{RawPrivateKey: serverKeyPEM},
								},
							},
						},
					},
					{
						Version:   trustBundleVersion,
						CreatedOn: certzCreatedOnV1,
						Entity: &certz.Entity_TrustBundle{
							TrustBundle: &certz.CertificateChain{
								Certificate: &certz.Certificate{
									Type:            certz.CertificateType_CERTIFICATE_TYPE_X509,
									Encoding:        certz.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
									CertificateType: &certz.Certificate_RawCertificate{RawCertificate: caPEM},
								},
							},
						},
					},
				},
			},
		},
	}
	if err := stream.Send(uploadReq); err != nil {
		return fmt.Errorf("failed to send certz upload request: %w", err)
	}
	if _, err := stream.Recv(); err != nil {
		return fmt.Errorf("failed to receive certz upload response: %w", err)
	}
	finalizeReq := &certz.RotateCertificateRequest{
		SslProfileId: profileID,
		RotateRequest: &certz.RotateCertificateRequest_FinalizeRotation{
			FinalizeRotation: &certz.FinalizeRequest{},
		},
	}
	if err := stream.Send(finalizeReq); err != nil {
		return fmt.Errorf("failed to send certz finalize request: %w", err)
	}
	if err := stream.CloseSend(); err != nil {
		return fmt.Errorf("failed to close certz rotate stream: %w", err)
	}
	return nil
}

// resolveGRPCServerName returns the name of the gRPC server instance already configured on the
// DUT (e.g. "default" on Arista, per "transport grpc default" in its native config), so that mTLS
// configuration targets the same instance the gNMI API uses.
func resolveGRPCServerName(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	sys := gnmi.Get(t, dut, gnmi.OC().System().State())
	names := make([]string, 0, len(sys.GrpcServer))
	for name := range sys.GrpcServer {
		names = append(names, name)
	}
	if len(names) == 0 {
		t.Fatalf("no grpc-server instance found on the DUT; cannot determine which instance to configure mTLS on")
	}
	sort.Strings(names)
	if len(names) > 1 {
		t.Logf("multiple grpc-server instances found on the DUT; using the first one reported: %s (all: %v)", names[0], names)
	}
	return names[0]
}

// configureGRPCServerMTLS binds the certz-provisioned SSL profile (server certificate + test-CA
// trust bundle) to the gNMI API's grpc-server instances via OpenConfig CertificateId (mirroring the
// certz trust_bundle_rotation test), then enables SPIFFE-SAN principal extraction over the DUT's
// native management CLI so pathz identities are authenticated from the client-cert SPIFFE URI. It
// must run BEFORE `service pathz` is enabled.
func configureGRPCServerMTLS(t *testing.T, dut *ondatra.DUTDevice, grpcServerName, profileID string, adminCert tls.Certificate, caCert *x509.Certificate) {
	t.Helper()
	servers := gnmi.GetAll(t, dut, gnmi.OC().System().GrpcServerAny().Name().State())
	yc, err := ygnmi.NewClient(dut.RawAPIs().GNMI(t), ygnmi.WithTarget(dut.ID()))
	if err != nil {
		t.Fatalf("failed to create ygnmi client for grpc-server mTLS config: %v", err)
	}
	// Capture each server's original certificate-id so teardown can restore it.
	origCertIDs := make(map[string]string, len(servers))
	for _, server := range servers {
		if v, err := ygnmi.Lookup(t.Context(), yc, gnmi.OC().System().GrpcServer(server).CertificateId().Config()); err == nil {
			if val, ok := v.Val(); ok {
				origCertIDs[server] = val
			}
		}
	}
	// Register SPIFFE/AAA revert cleanup first so it runs last in LIFO order.
	t.Cleanup(func() {
		revertConfigViaCLI(t, dut, fmt.Sprintf(
			"management api gnmi\ntransport grpc %s\nno authentication username priority x509-spiffe-full\naaa config-commands",
			grpcServerName))
	})
	// Register certificate-id restore cleanup second so it runs first in LIFO order.
	// The binding's gNMI client is locked out by the now-mTLS transport, so restore
	// certificate-id over an admin-cert connection (service pathz is already disabled
	// by the LIFO-earlier cleanup, and AAA command-authz is still disabled here, so
	// the write is accepted), then delete the test SSL profile over the CLI.
	t.Cleanup(func() {
		adminGNMI, err := dialGNMIAs(t, dut, adminCert, caCert)
		if err != nil {
			t.Logf("pathz teardown: failed to dial admin gnmi to restore certificate-id: %v", err)
		} else if adminYC, err := ygnmi.NewClient(adminGNMI, ygnmi.WithTarget(dut.ID())); err != nil {
			t.Logf("pathz teardown: failed to build admin ygnmi client to restore certificate-id: %v", err)
		} else {
			for _, server := range servers {
				path := gnmi.OC().System().GrpcServer(server).CertificateId().Config()
				var rerr error
				if orig, ok := origCertIDs[server]; ok {
					_, rerr = ygnmi.Replace(context.Background(), adminYC, path, orig)
				} else {
					_, rerr = ygnmi.Delete(context.Background(), adminYC, path)
				}
				if rerr != nil && !isConnectionReset(rerr) {
					t.Logf("pathz teardown: failed to restore certificate-id of grpc-server %q: %v", server, rerr)
				}
			}
		}
		revertConfigViaCLI(t, dut, fmt.Sprintf("management security\nno ssl profile %s", profileID))
	})
	// Binding the SSL profile via certificate-id makes the DUT reload TLS on the gRPC transport,
	// resetting the in-flight gNMI session (Unavailable/EOF) even though the config is applied.
	for _, server := range servers {
		if _, err := ygnmi.Replace(t.Context(), yc, gnmi.OC().System().GrpcServer(server).CertificateId().Config(), profileID); err != nil && !isConnectionReset(err) {
			t.Fatalf("failed to bind ssl profile %q to grpc-server %q: %v", profileID, server, err)
		}
	}
	// authentication username priority x509-spiffe-full is Arista-native and not expressible in
	// OpenConfig. The "-full" variant uses the ENTIRE client-cert SPIFFE URI (e.g.
	// spiffe://.../role/admin) as the pathz principal, so it matches the "user" in the upload policy.
	// aaa config-commands disabled stops config-applies from being gated by AAA command
	// authorization, leaving pathz as the sole authority (else group-permitted writes are rejected
	// with InvalidArgument "failed to apply: authorization denied" before pathz is consulted).
	runConfigViaCLI(t, dut, fmt.Sprintf(
		"management api gnmi\ntransport grpc %s\naaa config-commands disabled\nauthentication username priority x509-spiffe-full",
		grpcServerName))

}

// isConnectionReset reports whether err is the transport reset the DUT returns when committing the
// SSL profile reloads TLS on the gRPC server (Unavailable, or an EOF from the closed connection).
func isConnectionReset(err error) bool {
	if err == nil {
		return false
	}
	if status.Code(err) == codes.Unavailable {
		return true
	}
	return errors.Is(err, io.EOF)
}

// dialGNMIAs opens a raw gNMI client connection to the DUT authenticated ONLY with the given client
// identity certificate. It bypasses the ondatra binding's default dial options, which attach the
// framework's default client credentials, so the DUT sees only this identity.
func dialGNMIAs(t *testing.T, dut *ondatra.DUTDevice, clientCert tls.Certificate, caCert *x509.Certificate) (gpb.GNMIClient, error) {
	t.Helper()
	dialer := introspect.DUTDialer(t, dut, introspect.GNMI)
	// The DUT serves the certz-pushed server certificate, so the client verifies it against the
	// test CA and presents its own SPIFFE client cert for pathz identity.
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
	}
	creds := grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
	conn, err := dialer.DialFunc(context.Background(), dialer.DialTarget, creds)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gnmi with client identity: %w", err)
	}
	t.Cleanup(func() { conn.Close() })
	return gpb.NewGNMIClient(conn), nil
}

// dialYGNMIClientAs returns a ygnmi client bound to a gNMI connection authenticated with the given
// client identity certificate. It is used to read OpenConfig telemetry (e.g. the Pathz policy
// state) as that identity.
func dialYGNMIClientAs(t *testing.T, dut *ondatra.DUTDevice, clientCert tls.Certificate, caCert *x509.Certificate) *ygnmi.Client {
	t.Helper()
	raw, err := dialGNMIAs(t, dut, clientCert, caCert)
	if err != nil {
		t.Fatalf("failed to dial gnmi client identity for telemetry: %v", err)
	}
	c, err := ygnmi.NewClient(raw, ygnmi.WithTarget(dut.ID()))
	if err != nil {
		t.Fatalf("failed to create ygnmi client for telemetry: %v", err)
	}
	return c
}

// dialPathzClient returns a gNSI Pathz client authenticated with the given client identity
// certificate, required once mTLS is enforced on the DUT's grpc-server.
func dialPathzClient(t *testing.T, dut *ondatra.DUTDevice, clientCert tls.Certificate) (pathz.PathzClient, error) {
	t.Helper()
	dialer := introspect.DUTDialer(t, dut, introspect.GNSI)
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{clientCert},
		InsecureSkipVerify: true,
	}
	creds := grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
	conn, err := dialer.DialFunc(context.Background(), dialer.DialTarget, creds)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gnsi with client identity: %w", err)
	}
	t.Cleanup(func() { conn.Close() })
	return pathz.NewPathzClient(conn), nil
}

// buildAuthorizationPolicy unmarshals a JSON policy document into an AuthorizationPolicy proto.
func buildAuthorizationPolicy(policyJSON string) (*pathz.AuthorizationPolicy, error) {
	policy := &pathz.AuthorizationPolicy{}
	err := protojson.Unmarshal([]byte(policyJSON), policy)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal authorization policy: %w", err)
	}
	return policy, nil
}

// uploadPolicy sends a single UploadRequest on an open Rotate stream and waits for its response.
func uploadPolicy(stream pathz.Pathz_RotateClient, policy *pathz.AuthorizationPolicy, version string,
	createdOn uint64, forceOverwrite bool) error {
	req := &pathz.RotateRequest{
		ForceOverwrite: forceOverwrite,
		RotateRequest: &pathz.RotateRequest_UploadRequest{
			UploadRequest: &pathz.UploadRequest{
				Version:   version,
				CreatedOn: createdOn,
				Policy:    policy,
			},
		},
	}
	err := stream.Send(req)
	if err != nil {
		return fmt.Errorf("failed to send upload request for version %s: %w", version, err)
	}
	_, err = stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive upload response for version %s: %w", version, err)
	}
	return nil
}

// finalizeRotation sends a FinalizeRotation request on an open Rotate stream and waits for its
// response, tolerating io.EOF since the server may close the stream immediately after finalizing.
func finalizeRotation(stream pathz.Pathz_RotateClient) error {
	req := &pathz.RotateRequest{
		RotateRequest: &pathz.RotateRequest_FinalizeRotation{
			FinalizeRotation: &pathz.FinalizeRequest{},
		},
	}
	err := stream.Send(req)
	if err != nil {
		return fmt.Errorf("failed to send finalize request: %w", err)
	}
	_, err = stream.Recv()
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to receive finalize response: %w", err)
	}
	return nil
}

// rotateAndFinalize opens a Rotate stream, uploads the given policy, finalizes it, and closes
// the stream, combining uploadPolicy and finalizeRotation into a single call.
func rotateAndFinalize(ctx context.Context, client pathz.PathzClient, policy *pathz.AuthorizationPolicy,
	version string, createdOn uint64, forceOverwrite bool) error {
	stream, err := client.Rotate(ctx)
	if err != nil {
		return fmt.Errorf("failed to open rotate stream: %w", err)
	}
	err = uploadPolicy(stream, policy, version, createdOn, forceOverwrite)
	if err != nil {
		_ = stream.CloseSend()
		return err
	}
	err = finalizeRotation(stream)
	if err != nil {
		_ = stream.CloseSend()
		return err
	}
	err = stream.CloseSend()
	if err != nil {
		return fmt.Errorf("failed to close rotate stream: %w", err)
	}
	return nil
}

type mtlsEnvironment struct {
	identities     map[string]tls.Certificate
	spiffeIDs      map[string]string
	ports          []string
	origDescs      map[string]string
	grpcServerName string
	caCert         *x509.Certificate
}

// setupMTLSEnvironment provisions the test CA and role certs, pushes them to the DUT via Certz,
// resolves the live grpc-server instance, enables mTLS + the Pathz service on it, and registers
// t.Cleanup reverts for the CLI it enables. It must be called once from the parent test so the
// reverts run only after every subtest completes.
func setupMTLSEnvironment(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice) (*mtlsEnvironment, error) {
	t.Helper()

	caCert, caKey, caPEM, identities, err := provisionMTLSIdentities(dut)
	if err != nil {
		return nil, fmt.Errorf("failed provisioning mtls identities: %w", err)
	}
	serverIP, err := dutIPFromBinding(t, dut)
	if err != nil {
		return nil, fmt.Errorf("failed resolving dut ip from binding: %w", err)
	}
	serverCertPEM, serverKeyPEM, err := generateServerCert(caCert, caKey, serverIP)
	if err != nil {
		return nil, fmt.Errorf("failed generating server cert: %w", err)
	}

	certzClient, err := dialCertzClient(dut)
	if err != nil {
		return nil, fmt.Errorf("failed dialing certz client: %w", err)
	}
	if err := rotateServerProfile(ctx, certzClient, sslProfileID, serverCertPEM, serverKeyPEM, caPEM); err != nil {
		return nil, fmt.Errorf("failed rotating certz server profile: %w", err)
	}

	grpcServerName := resolveGRPCServerName(t, dut)
	ports := fetchInterfaceNames(t, dut, 2, dut.RawAPIs().GNMI(t))

	// Capture the discovered ports' descriptions over the pre-mTLS binding client so enforcement
	// subtests that write them can restore them (no cert identity can read /interfaces once pathz is on).
	origDescs := make(map[string]string, len(ports))
	for _, p := range ports {
		if d, ok := gnmi.Lookup(t, dut, gnmi.OC().Interface(p).Description().Config()).Val(); ok {
			origDescs[p] = d
		}
	}

	disableRequestAuthorizationCLI(t, dut, grpcServerName)

	// mTLS (ssl profile + SPIFFE principal extraction) must be applied BEFORE `service pathz`,
	// so that pathz is enabled on an already-mTLS transport and the DUT resolves the pathz
	// principal from the client-cert SPIFFE SAN rather than a username.
	configureGRPCServerMTLS(t, dut, grpcServerName, sslProfileID, identities[certAdmin], caCert)

	enablePathzServiceCLI(t, dut, grpcServerName)

	// Enabling `service pathz` reloads the gRPC transport, which is what activates the Pathz
	// service on the DUT's gNSI agent. Wait for the agent to start serving Pathz before
	// returning, otherwise the first Rotate/Get races ahead and fails Unimplemented.
	if err := awaitPathzServing(ctx, t, dut, identities[certAdmin]); err != nil {
		return nil, err
	}

	return &mtlsEnvironment{
		identities:     identities,
		spiffeIDs:      clientIdentities(dut),
		grpcServerName: grpcServerName,
		ports:          ports,
		origDescs:      origDescs,
		caCert:         caCert,
	}, nil
}

// getPathzPolicy calls gNSI Pathz.Get and returns the active policy metadata and contents.
func getPathzPolicy(ctx context.Context, client pathz.PathzClient) (*pathz.GetResponse, error) {
	resp, err := client.Get(ctx, &pathz.GetRequest{PolicyInstance: pathz.PolicyInstance_POLICY_INSTANCE_ACTIVE})
	if err != nil {
		return nil, fmt.Errorf("failed to get pathz policy: %w", err)
	}
	return resp, nil
}

// probeAccess issues a gNSI Pathz.Probe RPC and returns the resulting authorization action for
// the given user, path, and mode against the given policy instance.
func probeAccess(ctx context.Context, client pathz.PathzClient, user string, pathElems []*gpb.PathElem,
	mode pathz.Mode, instance pathz.PolicyInstance) (pathz.Action, error) {
	req := &pathz.ProbeRequest{
		User:           user,
		Path:           &gpb.Path{Elem: pathElems},
		Mode:           mode,
		PolicyInstance: instance,
	}
	resp, err := client.Probe(ctx, req)
	if err != nil {
		return pathz.Action_ACTION_UNSPECIFIED, fmt.Errorf("failed to probe access for user %s: %w", user, err)
	}
	return resp.GetAction(), nil
}

// expectPermissionDenied returns nil if err carries the PermissionDenied gRPC code, and an
// error describing the mismatch otherwise.
func expectPermissionDenied(err error) error {
	if err == nil {
		return fmt.Errorf("expected permission denied error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("expected a grpc status error, got: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		return fmt.Errorf("expected permission denied code, got: %v", st.Code())
	}
	return nil
}

// expectUnauthorizedError validates the DUT's response to an unauthorized read. By default the DUT
// must reject it with PermissionDenied. On devices where PathzUnauthorizedAccessErrorUnsupported is
// set the DUT prunes the denied subtree and returns no error instead, so the deviation asserts that
// alternate behavior rather than skipping the check.
func expectUnauthorizedError(dut *ondatra.DUTDevice, err error) error {
	if deviations.PathzUnauthorizedAccessErrorUnsupported(dut) {
		if err != nil {
			return fmt.Errorf("device does not support unauthorized access error; expected no error, got: %v", err)
		}
		return nil
	}
	return expectPermissionDenied(err)
}

// getHostnameConfig reads /system/config/hostname via a raw gNMI Get on the given client.
func getHostnameConfig(ctx context.Context, client gpb.GNMIClient) (string, error) {
	path, err := ygot.StringToStructuredPath(hostnamePath)
	if err != nil {
		return "", fmt.Errorf("failed to build path for %s: %w", hostnamePath, err)
	}
	resp, err := client.Get(ctx, &gpb.GetRequest{Path: []*gpb.Path{path}, Type: gpb.GetRequest_CONFIG})
	if err != nil {
		return "", fmt.Errorf("failed to get hostname config: %w", err)
	}
	notifications := resp.GetNotification()
	if len(notifications) == 0 || len(notifications[0].GetUpdate()) == 0 {
		return "", nil
	}
	return notifications[0].GetUpdate()[0].GetVal().GetStringVal(), nil
}

// setHostnameConfig writes /system/config/hostname via a raw gNMI Set on the given client.
func setHostnameConfig(ctx context.Context, client gpb.GNMIClient, hostname string) error {
	path, err := ygot.StringToStructuredPath(hostnamePath)
	if err != nil {
		return fmt.Errorf("failed to build path for %s: %w", hostnamePath, err)
	}
	update := &gpb.Update{Path: path, Val: &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: hostname}}}
	_, err = client.Set(ctx, &gpb.SetRequest{Update: []*gpb.Update{update}})
	if err != nil {
		return fmt.Errorf("failed to set hostname config: %w", err)
	}
	return nil
}

// setInterfaceDescription writes an interface's config/description via a raw gNMI Set on the
// given client.
func setInterfaceDescription(ctx context.Context, client gpb.GNMIClient, portName, description string) error {
	pathStr := fmt.Sprintf(interfaceDescPath, portName)
	path, err := ygot.StringToStructuredPath(pathStr)
	if err != nil {
		return fmt.Errorf("failed to build path for %s: %w", pathStr, err)
	}
	update := &gpb.Update{Path: path, Val: &gpb.TypedValue{Value: &gpb.TypedValue_StringVal{StringVal: description}}}
	_, err = client.Set(ctx, &gpb.SetRequest{Update: []*gpb.Update{update}})
	if err != nil {
		return fmt.Errorf("failed to set interface description on %s: %w", portName, err)
	}
	return nil
}

// pathzRemovalCLICommandForVendor returns the vendor-native CLI command used to remove the
// configured Pathz policy, and whether one is defined for the given vendor.
func pathzRemovalCLICommandForVendor(vendor ondatra.Vendor, grpcTransportName string) (string, bool) {
	switch vendor {
	case ondatra.CISCO:
		return "no grpc pathz-policy", true
	case ondatra.JUNIPER:
		return "delete system services extension-service request-response grpc pathz", true
	case ondatra.NOKIA:
		return "/system grpc-server pathz admin-state disable", true
	default:
		return "", false
	}
}

// disableRequestAuthorizationCLI turns off the platform's classic per-RPC AAA authorization gate
// on the gNMI/gNSI transport, if the platform has one and this test knows how to reach it. This is
// needed so that pathz is the sole authority for per-RPC authorization.
func disableRequestAuthorizationCLI(t *testing.T, dut *ondatra.DUTDevice, grpcTransportName string) {
	t.Helper()
	if dut.Vendor() != ondatra.ARISTA {
		t.Logf("no known native command to disable per-RPC AAA authorization for vendor %v; skipping", dut.Vendor())
		return
	}
	cmd := strings.Builder{}
	cmd.WriteString("management api gnmi\n")
	cmd.WriteString(fmt.Sprintf("transport grpc %s\n", grpcTransportName))
	cmd.WriteString("no authorization requests\n")
	t.Cleanup(func() {
		revertConfigViaCLI(t, dut, fmt.Sprintf("management api gnmi\ntransport grpc %s\nauthorization requests", grpcTransportName))
	})
	helpers.GnmiCLIConfig(t, dut, cmd.String())
}

// enablePathzServiceCLI enables the gNSI Pathz service on the DUT via vendor-native CLI, run over
// the DUT's out-of-band management CLI (console/SSH). It is enabled by the test -- rather than
// relying on the image default, so the test controls its state.
func enablePathzServiceCLI(t *testing.T, dut *ondatra.DUTDevice, grpcTransportName string) {
	t.Helper()
	if dut.Vendor() != ondatra.ARISTA {
		t.Logf("no known native command to enable the gNSI Pathz service for vendor %v; skipping (assuming it is pre-enabled)", dut.Vendor())
		return
	}
	t.Cleanup(func() {
		revertConfigViaCLI(t, dut, fmt.Sprintf("management api gnsi\ntransport gnmi %s\nno service pathz", grpcTransportName))
	})
	runConfigViaCLI(t, dut, fmt.Sprintf("management api gnsi\ntransport gnmi %s\nservice pathz", grpcTransportName))
}

// awaitPathzServing polls the DUT's gNSI Pathz service until it stops reporting Unimplemented,
// which on Arista only clears once the gRPC transport reload (triggered by binding the mTLS ssl
// profile) has fully activated `service pathz`. It returns nil as soon as any non-Unimplemented
// response is observed (including an expected error), or an error if the service never comes up.
func awaitPathzServing(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, clientCert tls.Certificate) error {
	t.Helper()
	const (
		timeout  = 60 * time.Second
		interval = 2 * time.Second
	)
	client, err := dialPathzClient(t, dut, clientCert)
	if err != nil {
		return fmt.Errorf("failed to dial pathz client while awaiting service: %w", err)
	}
	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var lastErr error
	for {
		_, err := client.Get(ctx, &pathz.GetRequest{PolicyInstance: pathz.PolicyInstance_POLICY_INSTANCE_ACTIVE})
		if status.Code(err) != codes.Unimplemented {
			// Any response other than Unimplemented means the Pathz service is serving.
			return nil
		}
		lastErr = err
		select {
		case <-timeoutCh:
			return fmt.Errorf("pathz service still Unimplemented after %v (transport reload did not activate it): %w", timeout, lastErr)
		case <-ticker.C:
		}
	}
}

// runConfigViaCLI applies vendor-native configuration over the DUT's out-of-band management CLI
// (console/SSH). This management-plane channel is independent of the gRPC/gNSI transport, so it is
// unaffected by transport reloads.
func runConfigViaCLI(t *testing.T, dut *ondatra.DUTDevice, commands string) {
	t.Helper()
	config := commands
	if dut.Vendor() == ondatra.ARISTA || dut.Vendor() == ondatra.CISCO {
		config = fmt.Sprintf("configure terminal\n%s\nend", commands)
	}
	if _, err := dut.RawAPIs().CLI(t).RunCommand(t.Context(), config); err != nil {
		t.Fatalf("failed to apply cli config over the management plane: %v", err)
	}
}

// revertConfigViaCLI applies vendor-native config over the out-of-band CLI as best-effort teardown,
// logging (not failing) on error so a passed test is not failed by a cleanup hiccup.
func revertConfigViaCLI(t *testing.T, dut *ondatra.DUTDevice, commands string) {
	t.Helper()
	config := commands
	if dut.Vendor() == ondatra.ARISTA || dut.Vendor() == ondatra.CISCO {
		config = fmt.Sprintf("configure terminal\n%s\nend", commands)
	}
	if _, err := dut.RawAPIs().CLI(t).RunCommand(context.Background(), config); err != nil {
		t.Logf("pathz teardown: failed to apply cli revert %q: %v", commands, err)
	}
}

// removePathzPolicyViaCLI removes the active Pathz policy using the DUT's out-of-band management CLI
// (console/SSH), as Pathz-4 specifies. This management-plane channel is independent of the
// gRPC/gNSI transport, so it survives a transport reload.
func removePathzPolicyViaCLI(t *testing.T, dut *ondatra.DUTDevice, command string) {
	t.Helper()
	runConfigViaCLI(t, dut, command)
}

// pathzRemovalCLICommand returns the vendor-native Pathz removal command, skipping the test if
// the DUT vendor has no such command defined.
func pathzRemovalCLICommand(t *testing.T, dut *ondatra.DUTDevice, grpcTransportName string) string {
	t.Helper()
	cmd, ok := pathzRemovalCLICommandForVendor(dut.Vendor(), grpcTransportName)
	if !ok {
		t.Skipf("no vendor-native Pathz removal CLI command defined for vendor %v", dut.Vendor())
	}
	return cmd
}

// fetchInterfaceNames returns count physical interface names discovered from the DUT, trying
// structured gNMI telemetry first and falling back to vendor CLI.
func fetchInterfaceNames(t *testing.T, dut *ondatra.DUTDevice, count int, gnmiClient gpb.GNMIClient) []string {
	t.Helper()
	names := interfaceNamesViaGNMI(t, gnmiClient)
	if len(names) == 0 {
		t.Logf("gNMI interface discovery returned no interfaces; falling back to vendor CLI")
		names = interfaceNamesViaCLI(t, dut)
	}
	interfaceNames := make([]string, 0, len(names))
	for name := range names {
		if strings.HasPrefix(name, "Management") || strings.HasPrefix(name, "mgmt") {
			continue
		}
		interfaceNames = append(interfaceNames, name)
	}
	sort.Strings(interfaceNames)
	if len(interfaceNames) < count {
		t.Fatalf("dut reported %d usable interfaces, want at least %d", len(interfaceNames), count)
	}
	return interfaceNames[:count]
}

// interfaceNamesViaGNMI reads the interface list from the DUT via a raw gNMI Get on the given,
// policy-permitted client.
func interfaceNamesViaGNMI(t *testing.T, client gpb.GNMIClient) map[string]bool {
	t.Helper()
	if client == nil {
		t.Logf("no policy-permitted gnmi client available for interface discovery")
		return nil
	}
	path, err := ygot.StringToStructuredPath(interfacesPath)
	if err != nil {
		t.Fatalf("failed to build path for %s: %v", interfacesPath, err)
	}
	resp, err := client.Get(t.Context(), &gpb.GetRequest{
		Path:     []*gpb.Path{path},
		Type:     gpb.GetRequest_CONFIG,
		Encoding: gpb.Encoding_JSON_IETF,
	})
	if err != nil {
		t.Logf("gnmi get for interface discovery failed: %v", err)
		return nil
	}
	return discoverInterfaceNames(t, resp)
}

// interfaceNamesViaCLI discovers interface names using a vendor-native CLI show command.
func interfaceNamesViaCLI(t *testing.T, dut *ondatra.DUTDevice) map[string]bool {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA:
		res, err := dut.RawAPIs().CLI(t).RunCommand(t.Context(), "show interfaces status | json")
		if err != nil {
			t.Fatalf("failed to run interface discovery cli command: %v", err)
		}
		return aristaInterfaceStatusNames(t, []byte(res.Output()))
	default:
		t.Fatalf("no vendor-native interface discovery command defined for vendor %v", dut.Vendor())
		return nil
	}
}

// aristaInterfaceStatusNames parses the JSON output of Arista's "show interfaces status | json"
// command and returns the set of interface names.
func aristaInterfaceStatusNames(t *testing.T, jsonOut []byte) map[string]bool {
	t.Helper()
	payload := struct {
		InterfaceStatuses map[string]json.RawMessage `json:"interfaceStatuses"`
	}{}
	if err := json.Unmarshal(jsonOut, &payload); err != nil {
		t.Fatalf("failed to parse arista interface status json: %v", err)
	}
	names := make(map[string]bool, len(payload.InterfaceStatuses))
	for name := range payload.InterfaceStatuses {
		names[name] = true
	}
	return names
}

// discoverInterfaceNames extracts interface names from every notification/update in a gNMI
// GetResponse for the interfaces subtree.
func discoverInterfaceNames(t *testing.T, resp *gpb.GetResponse) map[string]bool {
	t.Helper()
	names := make(map[string]bool)
	for _, notification := range resp.GetNotification() {
		prefixElems := notification.GetPrefix().GetElem()
		for _, update := range notification.GetUpdate() {
			elems := append(append([]*gpb.PathElem{}, prefixElems...), update.GetPath().GetElem()...)
			for _, elem := range elems {
				if elem.GetName() == "interface" {
					if name, ok := elem.GetKey()["name"]; ok {
						names[name] = true
					}
				}
			}
			if jsonVal := update.GetVal().GetJsonIetfVal(); len(jsonVal) > 0 {
				extractInterfaceNamesFromJSON(jsonVal, names)
			}
		}
	}
	return names
}

// extractInterfaceNamesFromJSON parses a JSON_IETF blob and collects openconfig-interface names
// into names, handling interface-list, single-interface, and wrapper-object shapes.
func extractInterfaceNamesFromJSON(jsonVal []byte, names map[string]bool) {
	var decoded any
	if err := json.Unmarshal(jsonVal, &decoded); err != nil {
		return
	}
	var list []any
	switch v := decoded.(type) {
	case []any:
		list = v
	case map[string]any:
		if ifs, ok := v["openconfig-interfaces:interface"].([]any); ok {
			list = ifs
		} else if ifs, ok := v["interface"].([]any); ok {
			list = ifs
		} else {
			list = []any{v}
		}
	}
	for _, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if name, ok := obj["name"].(string); ok {
			names[name] = true
		}
	}
}

// pathzSandboxVersionRaw reads the pathz SANDBOX policy version via a raw gNMI Get. ok is false
// when the leaf is unpopulated (e.g. the sandbox has been rolled back / has no pending policy).
func pathzSandboxVersionRaw(ctx context.Context, client gpb.GNMIClient) (version string, ok bool, err error) {
	path, err := ygot.StringToStructuredPath(pathzSandboxVersionPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to build path for %s: %w", pathzSandboxVersionPath, err)
	}
	resp, err := client.Get(ctx, &gpb.GetRequest{Path: []*gpb.Path{path}, Type: gpb.GetRequest_STATE})
	if err != nil {
		return "", false, fmt.Errorf("failed to get pathz sandbox version: %w", err)
	}
	notifications := resp.GetNotification()
	if len(notifications) == 0 || len(notifications[0].GetUpdate()) == 0 {
		return "", false, nil
	}
	return notifications[0].GetUpdate()[0].GetVal().GetStringVal(), true, nil
}

// awaitPathzSandboxVersionRaw polls the pathz SANDBOX policy version via pathzSandboxVersionRaw
// until it equals wantVersion (or, when wantVersion is empty, until the leaf is unpopulated),
// failing after a timeout.
func awaitPathzSandboxVersionRaw(ctx context.Context, t *testing.T, client gpb.GNMIClient, wantVersion string, timeout time.Duration) {
	t.Helper()
	const pollInterval = 500 * time.Millisecond
	ticker := time.NewTicker(pollInterval)
	for {
		version, ok, err := pathzSandboxVersionRaw(ctx, client)
		if err == nil && ((wantVersion == "" && !ok) || version == wantVersion) {
			return
		}
		select {
		case <-ctx.Done():
			t.Errorf("SANDBOX pathz policy telemetry did not reach version %q within %v", wantVersion, timeout)
			return
		case <-ticker.C:
		}
	}
}

// awaitPolicyTelemetry waits until the pathz policy telemetry for the given instance reports
// wantVersion and wantCreatedOn, failing after a timeout. It watches the OC pathz policy leaves via
// the reader's mTLS ygnmi client; ondatra's gnmi.Watch cannot be used because the binding's default
// client is locked out once mTLS is enforced. On Arista, the SANDBOX instance is read via a raw gNMI
// Get instead (see pathzSandboxVersionPath); every other vendor/instance combination uses the
// generated OC path as before.
func awaitPolicyTelemetry(t *testing.T, dut *ondatra.DUTDevice, rawClient gpb.GNMIClient, client *ygnmi.Client,
	instance oc.E_Policy_Instance, wantVersion string, wantCreatedOn uint64) {
	t.Helper()
	const timeout = 30 * time.Second
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()

	if dut.Vendor() == ondatra.ARISTA && instance == oc.Policy_Instance_SANDBOX {
		awaitPathzSandboxVersionRaw(ctx, t, rawClient, wantVersion, timeout)
		return
	}

	q := gnmi.OC().System().GnmiPathzPolicies().Policy(instance).State()
	if _, err := ygnmi.Watch(ctx, client, q, func(v *ygnmi.Value[*oc.System_GnmiPathzPolicies_Policy]) error {
		p, ok := v.Val()
		if ok && p.GetVersion() == wantVersion && p.GetCreatedOn() == wantCreatedOn {
			return nil
		}
		return ygnmi.Continue
	}).Await(); err != nil {
		t.Errorf("%v pathz policy telemetry did not reach version %q / created-on %d within %v: %v",
			instance, wantVersion, wantCreatedOn, timeout, err)
	}
}

// awaitSandboxCleared waits until the sandbox pathz policy telemetry is rolled back (absent or empty
// version/created-on), failing after a timeout. On Arista this is read via a raw gNMI Get (see
// pathzSandboxVersionPath); every other vendor uses the generated OC path as before.
func awaitSandboxCleared(t *testing.T, dut *ondatra.DUTDevice, rawClient gpb.GNMIClient, client *ygnmi.Client) {
	t.Helper()
	const timeout = 30 * time.Second
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()

	if dut.Vendor() == ondatra.ARISTA {
		awaitPathzSandboxVersionRaw(ctx, t, rawClient, "", timeout)
		return
	}

	q := gnmi.OC().System().GnmiPathzPolicies().Policy(oc.Policy_Instance_SANDBOX).State()
	if _, err := ygnmi.Watch(ctx, client, q, func(v *ygnmi.Value[*oc.System_GnmiPathzPolicies_Policy]) error {
		p, ok := v.Val()
		if !ok || (p.GetVersion() == "" && p.GetCreatedOn() == 0) {
			return nil
		}
		return ygnmi.Continue
	}).Await(); err != nil {
		t.Errorf("sandbox pathz policy was not rolled back within %v: %v", timeout, err)
	}
}

// policiesEqual reports whether two AuthorizationPolicy protos are equal, ignoring rule/group
// order.
func policiesEqual(a, b *pathz.AuthorizationPolicy) bool {
	aClone, bClone := proto.Clone(a).(*pathz.AuthorizationPolicy), proto.Clone(b).(*pathz.AuthorizationPolicy)
	sortPolicyForComparison(aClone)
	sortPolicyForComparison(bClone)
	return proto.Equal(aClone, bClone)
}

// sortPolicyForComparison sorts a policy's rules and groups by id/name in place, so two
// policies with the same content in different upload order compare as equal.
func sortPolicyForComparison(p *pathz.AuthorizationPolicy) {
	if p == nil {
		return
	}
	sort.Slice(p.GetRules(), func(i, j int) bool { return p.GetRules()[i].GetId() < p.GetRules()[j].GetId() })
	sort.Slice(p.GetGroups(), func(i, j int) bool { return p.GetGroups()[i].GetName() < p.GetGroups()[j].GetName() })
}

// verifyGetResponse checks a Pathz.Get response's version, created-on, and full policy contents
// (rule/group count and equality) against the expected values.
func verifyGetResponse(t *testing.T, resp *pathz.GetResponse, wantVersion string,
	wantCreatedOn uint64, wantPolicy *pathz.AuthorizationPolicy) {
	t.Helper()
	if resp.GetVersion() != wantVersion {
		t.Errorf("get policy version: got %s, want %s", resp.GetVersion(), wantVersion)
	}
	if resp.GetCreatedOn() != wantCreatedOn {
		t.Errorf("get policy created-on: got %d, want %d", resp.GetCreatedOn(), wantCreatedOn)
	}
	gotRules, wantRules := resp.GetPolicy().GetRules(), wantPolicy.GetRules()
	if len(gotRules) != len(wantRules) {
		t.Errorf("get policy rule count: got %d, want %d", len(gotRules), len(wantRules))
	}
	gotGroups, wantGroups := resp.GetPolicy().GetGroups(), wantPolicy.GetGroups()
	if len(gotGroups) != len(wantGroups) {
		t.Errorf("get policy group count: got %d, want %d", len(gotGroups), len(wantGroups))
	}
	if !policiesEqual(resp.GetPolicy(), wantPolicy) {
		t.Errorf("get policy contents mismatch: got %v, want %v", resp.GetPolicy(), wantPolicy)
	}
}

// pathzCounterValue reads a per-path pathz counter's current value as a pre-operation baseline,
// returning 0 if the counters are unsupported or not yet populated.
func pathzCounterValue(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice,
	client *ygnmi.Client, query ygnmi.SingletonQuery[uint64]) uint64 {
	t.Helper()
	if deviations.PathzCountersUnsupported(dut) {
		return 0
	}
	rv, err := ygnmi.Lookup(ctx, client, query)
	if err != nil {
		return 0
	}
	v, _ := rv.Val()
	return v
}

// verifyPathzCounter asserts that a per-path pathz policy counter increased past the pre-operation
// baseline (proving THIS operation incremented it), unless the device does not support these
// counters (PathzCountersUnsupported), in which case the check is gated off by the deviation.
func verifyPathzCounter(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice,
	client *ygnmi.Client, query ygnmi.SingletonQuery[uint64], before uint64, desc string) {
	t.Helper()
	if deviations.PathzCountersUnsupported(dut) {
		return
	}
	rv, err := ygnmi.Lookup(ctx, client, query)
	if err != nil {
		t.Errorf("pathz %s counter for %s could not be read: %v", desc, hostnamePath, err)
		return
	}
	after, ok := rv.Val()
	if !ok {
		t.Errorf("pathz %s counter for %s not populated by DUT", desc, hostnamePath)
		return
	}
	if after <= before {
		t.Errorf("pathz %s counter for %s did not increment (before %d, after %d)", desc, hostnamePath, before, after)
	}
}

// TestPathz implements the gNSI Pathz path-level authorization tests (Pathz-1 to Pathz-4). mTLS and
// the Pathz service are configured once for all subtests and reverted when TestPathz completes.
func TestPathz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	if dut.Vendor() != ondatra.ARISTA {
		t.Skipf("pathz mTLS/service setup is only implemented for ARISTA; skipping for vendor %v", dut.Vendor())
	}
	ctx := t.Context()
	env, err := setupMTLSEnvironment(ctx, t, dut)
	if err != nil {
		t.Fatal(err)
	}

	// Register cleanup to ensure the policy is reset to empty when this subtest completes.
	t.Cleanup(func() {
		client, err := dialPathzClient(t, dut, env.identities[certAdmin])
		if err != nil {
			t.Logf("Failed to dial pathz client in t.Cleanup: %v", err)
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Create an empty gNSI authorization policy struct pointer
		emptyPolicy := &pathz.AuthorizationPolicy{}

		if err := rotateAndFinalize(cleanupCtx, client, emptyPolicy, "", 0, true); err != nil {
			t.Logf("Failed to reset pathz policy in t.Cleanup: %v", err)
		}
	})

	t.Run("Pathz-1_PolicyRotationAndFreshness", func(t *testing.T) {
		port1 := env.ports[0]

		client, err := dialPathzClient(t, dut, env.identities[certAdmin])
		if err != nil {
			t.Fatal(err)
		}
		readerTelemetry, err := dialGNMIAs(t, dut, env.identities[certReader], env.caCert)
		if err != nil {
			t.Fatal(err)
		}
		// ygnmi client over the same reader connection, for Watch-based sandbox telemetry waits.
		readerWatch, err := ygnmi.NewClient(readerTelemetry, ygnmi.WithTarget(dut.ID()))
		if err != nil {
			t.Fatal(err)
		}

		policyJSON := fmt.Sprintf(baselinePolicyTemplate, env.spiffeIDs[certReader], env.spiffeIDs[certAdmin], port1)
		policy, err := buildAuthorizationPolicy(policyJSON)
		if err != nil {
			t.Fatal(err)
		}

		// pathz default-denies every read until an ACTIVE policy grants it, so commit the baseline first
		// (it permits the reader to read /system); only then can the reader observe sandbox telemetry
		// during a subsequent rotation.
		if err := rotateAndFinalize(ctx, client, policy, policyVersionV1, policyCreatedOnV1, true); err != nil {
			t.Fatal(err)
		}

		// Shared sandbox Rotate stream on a cancelable context: InitialPush uploads (without
		// finalizing) and RollbackOnDisconnect cancels the context to close the gRPC session (per the
		// README's "close the gRPC session without sending Finalize" step), forcing the rollback.
		sandboxCtx, cancelSandbox := context.WithCancel(ctx)
		defer cancelSandbox()
		sandboxStream, err := client.Rotate(sandboxCtx)
		if err != nil {
			t.Fatal(err)
		}

		t.Run("InitialPushAndSandboxTelemetry", func(t *testing.T) {
			if err := uploadPolicy(sandboxStream, policy, policyVersionV1, policyCreatedOnV1, false); err != nil {
				t.Fatal(err)
			}
			awaitPolicyTelemetry(t, dut, readerTelemetry, readerWatch, oc.Policy_Instance_SANDBOX, policyVersionV1, policyCreatedOnV1)
		})

		t.Run("RollbackOnDisconnect", func(t *testing.T) {
			// Cancel the Rotate context to fully close the gRPC session (CloseSend only half-closes it).
			cancelSandbox()
			awaitSandboxCleared(t, dut, readerTelemetry, readerWatch)
			// The rolled-back sandbox must leave the committed ACTIVE policy (v1) intact.
			awaitPolicyTelemetry(t, dut, readerTelemetry, readerWatch, oc.Policy_Instance_ACTIVE, policyVersionV1, policyCreatedOnV1)
		})

		t.Run("FinalizeRotation", func(t *testing.T) {
			err := rotateAndFinalize(ctx, client, policy, policyVersionV1, policyCreatedOnV1, false)
			if err != nil {
				t.Fatal(err)
			}
			awaitPolicyTelemetry(t, dut, readerTelemetry, readerWatch, oc.Policy_Instance_ACTIVE, policyVersionV1, policyCreatedOnV1)
		})

		t.Run("GetVerification", func(t *testing.T) {
			resp, err := getPathzPolicy(ctx, client)
			if err != nil {
				t.Fatal(err)
			}
			verifyGetResponse(t, resp, policyVersionV1, policyCreatedOnV1, policy)
		})

		t.Run("ForceOverwrite", func(t *testing.T) {
			modifiedPolicy := proto.Clone(policy).(*pathz.AuthorizationPolicy)
			modifiedPolicy.Rules = append(modifiedPolicy.Rules, &pathz.AuthorizationRule{
				Id:        "force-overwrite-probe-rule",
				Principal: &pathz.AuthorizationRule_User{User: env.spiffeIDs[certUnauthorized]},
				Path:      &gpb.Path{Elem: []*gpb.PathElem{{Name: "system"}}},
				Action:    pathz.Action_ACTION_DENY,
				Mode:      pathz.Mode_MODE_READ,
			})

			// Without force_overwrite, changed content under an already-committed version must be
			// rejected (pathz.proto: ALREADY_EXISTS). On devices with PathzForceOverwriteUnsupported the
			// DUT does not enforce this and accepts the rotation, so the deviation asserts that instead.
			err := rotateAndFinalize(ctx, client, modifiedPolicy, policyVersionV1, policyCreatedOnV1, false)
			if deviations.PathzForceOverwriteUnsupported(dut) {
				if err != nil {
					t.Errorf("device does not enforce force_overwrite; expected the non-forced rotation to be accepted, got: %v", err)
				}
			} else if err == nil {
				t.Errorf("rotation of changed content under an existing version without force_overwrite unexpectedly succeeded")
			}
			// force_overwrite=true must accept changed content under an already-committed version.
			if err := rotateAndFinalize(ctx, client, modifiedPolicy, policyVersionV1, policyCreatedOnV1, true); err != nil {
				t.Errorf("rotation with force_overwrite failed: %v", err)
			}
			if err := rotateAndFinalize(ctx, client, policy, policyVersionV1, policyCreatedOnV1, true); err != nil {
				t.Errorf("failed to restore original policy content after force_overwrite probe: %v", err)
			}
		})
	})

	t.Run("Pathz-2_Enforcement", func(t *testing.T) {
		port1 := env.ports[0]
		port2 := env.ports[1]
		identities := env.identities

		client, err := dialPathzClient(t, dut, env.identities[certAdmin])
		if err != nil {
			t.Fatal(err)
		}
		policyJSON := fmt.Sprintf(baselinePolicyTemplate, env.spiffeIDs[certReader], env.spiffeIDs[certAdmin], port1)
		policy, err := buildAuthorizationPolicy(policyJSON)
		if err != nil {
			t.Fatal(err)
		}
		err = rotateAndFinalize(ctx, client, policy, policyVersionV1, policyCreatedOnV1, true)
		if err != nil {
			t.Fatal(err)
		}

		readerClient, err := dialGNMIAs(t, dut, identities[certReader], env.caCert)
		if err != nil {
			t.Fatal(err)
		}
		adminClient, err := dialGNMIAs(t, dut, identities[certAdmin], env.caCert)
		if err != nil {
			t.Fatal(err)
		}
		unauthorizedClient, err := dialGNMIAs(t, dut, identities[certUnauthorized], env.caCert)
		if err != nil {
			t.Fatal(err)
		}
		readerTelemetry := dialYGNMIClientAs(t, dut, identities[certReader], env.caCert)

		t.Run("ReaderReadPermittedWriteDenied", func(t *testing.T) {
			pathzCounters := gnmi.OC().System().GrpcServer(env.grpcServerName).GnmiPathzPolicyCounters()
			readAccepts := pathzCounters.Path(hostnamePath).Reads().AccessAccepts().State()
			writeRejects := pathzCounters.Path(hostnamePath).Writes().AccessRejects().State()

			beforeReads := pathzCounterValue(ctx, t, dut, readerTelemetry, readAccepts)
			if _, err := getHostnameConfig(ctx, readerClient); err != nil {
				t.Errorf("reader get on hostname unexpectedly failed: %v", err)
			}
			verifyPathzCounter(ctx, t, dut, readerTelemetry, readAccepts, beforeReads, "read access-accepts")

			beforeRejects := pathzCounterValue(ctx, t, dut, readerTelemetry, writeRejects)
			if err := expectPermissionDenied(setHostnameConfig(ctx, readerClient, "reader-attempt")); err != nil {
				t.Errorf("reader set on hostname: %v", err)
			}
			verifyPathzCounter(ctx, t, dut, readerTelemetry, writeRejects, beforeRejects, "write access-rejects")
		})

		t.Run("AdminGroupPermitSpecificUserDeny", func(t *testing.T) {
			// Restore port2's description via the admin identity (it holds the group write permit; pathz
			// is still active during this subtest's cleanup, before the parent teardown).
			t.Cleanup(func() {
				if err := setInterfaceDescription(ctx, adminClient, port2, env.origDescs[port2]); err != nil {
					t.Logf("failed to restore description on %s: %v", port2, err)
				}
			})
			err := setInterfaceDescription(ctx, adminClient, port2, "pathz-admin-group-test")
			if err != nil {
				t.Errorf("admin set on port2 description unexpectedly failed: %v", err)
			}
			err = setInterfaceDescription(ctx, adminClient, port1, "pathz-admin-deny-test")
			err = expectPermissionDenied(err)
			if err != nil {
				t.Errorf("admin set on port1 description (best-match: user-specific deny beats group permit): %v", err)
			}
		})

		t.Run("DefaultDeny", func(t *testing.T) {
			// The unauthorized identity has no matching rule, so reads default-deny. On devices with the
			// PathzUnauthorizedAccessErrorUnsupported deviation the DUT prunes the subtree and returns no
			// error instead of PermissionDenied; the write is still expected to be denied outright.
			_, getErr := getHostnameConfig(ctx, unauthorizedClient)
			if err := expectUnauthorizedError(dut, getErr); err != nil {
				t.Errorf("unauthorized get on hostname: %v", err)
			}
			setErr := setHostnameConfig(ctx, unauthorizedClient, "unauthorized-attempt")
			if err := expectPermissionDenied(setErr); err != nil {
				t.Errorf("unauthorized set on hostname: %v", err)
			}
		})
	})

	t.Run("Pathz-3_Probe", func(t *testing.T) {
		port1 := env.ports[0]

		client, err := dialPathzClient(t, dut, env.identities[certAdmin])
		if err != nil {
			t.Fatal(err)
		}
		policyJSON := fmt.Sprintf(baselinePolicyTemplate, env.spiffeIDs[certReader], env.spiffeIDs[certAdmin], port1)
		policy, err := buildAuthorizationPolicy(policyJSON)
		if err != nil {
			t.Fatal(err)
		}
		err = rotateAndFinalize(ctx, client, policy, policyVersionV1, policyCreatedOnV1, true)
		if err != nil {
			t.Fatal(err)
		}
		systemElems := []*gpb.PathElem{{Name: "system"}}

		t.Run("ProbeActivePolicy", func(t *testing.T) {
			active := pathz.PolicyInstance_POLICY_INSTANCE_ACTIVE
			action, err := probeAccess(ctx, client, env.spiffeIDs[certReader], systemElems, pathz.Mode_MODE_READ, active)
			if err != nil {
				t.Fatal(err)
			}
			if action != pathz.Action_ACTION_PERMIT {
				t.Errorf("probe read on active policy: got %v, want ACTION_PERMIT", action)
			}
			action, err = probeAccess(ctx, client, env.spiffeIDs[certReader], systemElems, pathz.Mode_MODE_WRITE, active)
			if err != nil {
				t.Fatal(err)
			}
			if action != pathz.Action_ACTION_DENY {
				t.Errorf("probe write on active policy: got %v, want ACTION_DENY", action)
			}
		})

		t.Run("ProbeSandboxDuringRotation", func(t *testing.T) {
			denyPolicy, err := buildAuthorizationPolicy(fmt.Sprintf(denyReaderSystemPolicy, env.spiffeIDs[certReader]))
			if err != nil {
				t.Fatal(err)
			}
			stream, err := client.Rotate(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = uploadPolicy(stream, denyPolicy, policyVersionV1, policyCreatedOnV1, false)
			if err != nil {
				t.Fatal(err)
			}
			sandbox := pathz.PolicyInstance_POLICY_INSTANCE_SANDBOX
			sandboxAction, err := probeAccess(ctx, client, env.spiffeIDs[certReader], systemElems, pathz.Mode_MODE_READ, sandbox)
			if err != nil {
				t.Fatal(err)
			}
			if sandboxAction != pathz.Action_ACTION_DENY {
				t.Errorf("probe read on sandbox policy: got %v, want ACTION_DENY", sandboxAction)
			}
			active := pathz.PolicyInstance_POLICY_INSTANCE_ACTIVE
			activeAction, err := probeAccess(ctx, client, env.spiffeIDs[certReader], systemElems, pathz.Mode_MODE_READ, active)
			if err != nil {
				t.Fatal(err)
			}
			if activeAction != pathz.Action_ACTION_PERMIT {
				t.Errorf("probe read on active policy during rotation: got %v, want ACTION_PERMIT", activeAction)
			}
			err = stream.CloseSend()
			if err != nil {
				t.Fatal(err)
			}
		})
	})

	t.Run("Pathz-4_PolicyRemovalCLI", func(t *testing.T) {
		port1 := env.ports[0]

		client, err := dialPathzClient(t, dut, env.identities[certAdmin])
		if err != nil {
			t.Fatal(err)
		}
		policyJSON := fmt.Sprintf(baselinePolicyTemplate, env.spiffeIDs[certReader], env.spiffeIDs[certAdmin], port1)
		policy, err := buildAuthorizationPolicy(policyJSON)
		if err != nil {
			t.Fatal(err)
		}
		err = rotateAndFinalize(ctx, client, policy, policyVersionV1, policyCreatedOnV1, true)
		if err != nil {
			t.Fatal(err)
		}

		t.Run("VerifyPolicyActive", func(t *testing.T) {
			resp, err := getPathzPolicy(ctx, client)
			if err != nil {
				t.Fatal(err)
			}
			verifyGetResponse(t, resp, policyVersionV1, policyCreatedOnV1, policy)
		})

		t.Run("RemoveViaCLIAndVerifyCleared", func(t *testing.T) {
			if deviations.PathzPolicyRemovalViaCliUnsupported(dut) {
				t.Logf("pathz policy removal via cli is not supported on vendor %v; skipping removal verification", dut.Vendor())
				return
			}
			cliCommand := pathzRemovalCLICommand(t, dut, env.grpcServerName)
			removePathzPolicyViaCLI(t, dut, cliCommand)
			resp, err := client.Get(ctx, &pathz.GetRequest{PolicyInstance: pathz.PolicyInstance_POLICY_INSTANCE_ACTIVE})
			if err != nil {
				// Once the policy is removed via CLI, the DUT reports it is gone via NotFound /
				// Unimplemented, or an "unable to find key" style error. Any other code is a real failure.
				if code := status.Code(err); code != codes.NotFound && code != codes.Unimplemented &&
					!strings.Contains(err.Error(), "unable to find key") {
					t.Errorf("pathz Get after cli removal: got error code %v, want empty policy or policy-cleared error: %v", code, err)
				}
				return
			}
			if resp.GetVersion() != "" || len(resp.GetPolicy().GetRules()) != 0 {
				t.Errorf("pathz policy still present after cli removal: version %q, %d rules", resp.GetVersion(), len(resp.GetPolicy().GetRules()))
			}
		})
	})
}
