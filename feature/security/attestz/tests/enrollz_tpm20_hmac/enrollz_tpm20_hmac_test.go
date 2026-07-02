package enrollz_tpm20_hmac

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gpb "github.com/openconfig/gnmi/proto/gnmi"
	fpb "github.com/openconfig/gnoi/file"
	"github.com/openconfig/ondatra/binding/introspect"
	"github.com/openconfig/ondatra/gnmi"

	cpb "github.com/openconfig/attestz/proto/common_definitions"
	epb "github.com/openconfig/attestz/proto/tpm_enrollz"
	"github.com/openconfig/attestz/service/biz"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/topologies/binding"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	defaultSSLProfile = "ENROLLZ_PROFILE"
	dutPPKFilePath    = "/tmp/ppk_pub.pem"

	enrollzCertFilename = "enrollz.crt"
	enrollzKeyFilename  = "enrollz.key"

	rsaKeyBits = 2048

	probeTimeout          = 30 * time.Second
	retrySleepTime        = 10 * time.Second
	rebootProbeSleep      = 15 * time.Second
	rebootTimeout         = 5 * time.Minute
	enrollzRestartTimeout = 2 * time.Minute
	configBackupFile      = "enrollz_config.cfg"
)

var (
	dutTime     time.Time
	enrollzCert string
	enrollzKey  string
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type hmacEnrollDeps struct {
	*biz.DefaultTPM20Utils
	*biz.DefaultTpmCertVerifier
	enrollzC epb.TpmEnrollzServiceClient
	biz.ROTDBClient
	biz.SwitchOwnerCaClient
}

func (d *hmacEnrollDeps) GetIakCert(ctx context.Context, req *epb.GetIakCertRequest) (*epb.GetIakCertResponse, error) {
	return d.enrollzC.GetIakCert(ctx, req)
}

func (d *hmacEnrollDeps) RotateOIakCert(ctx context.Context, req *epb.RotateOIakCertRequest) (*epb.RotateOIakCertResponse, error) {
	return d.enrollzC.RotateOIakCert(ctx, req)
}

func (d *hmacEnrollDeps) RotateAIKCert(ctx context.Context, opts ...grpc.CallOption) (epb.TpmEnrollzService_RotateAIKCertClient, error) {
	return d.enrollzC.RotateAIKCert(ctx, opts...)
}

func (d *hmacEnrollDeps) GetControlCardVendorID(ctx context.Context, req *epb.GetControlCardVendorIDRequest) (*epb.GetControlCardVendorIDResponse, error) {
	return d.enrollzC.GetControlCardVendorID(ctx, req)
}

func (d *hmacEnrollDeps) Challenge(ctx context.Context, req *epb.ChallengeRequest) (*epb.ChallengeResponse, error) {
	return d.enrollzC.Challenge(ctx, req)
}

func (d *hmacEnrollDeps) GetIdevidCsr(ctx context.Context, req *epb.GetIdevidCsrRequest) (*epb.GetIdevidCsrResponse, error) {
	return d.enrollzC.GetIdevidCsr(ctx, req)
}

func newHMACEnrollDeps(enrollzC epb.TpmEnrollzServiceClient, rotDB biz.ROTDBClient, ownerCA biz.SwitchOwnerCaClient) *hmacEnrollDeps {
	return &hmacEnrollDeps{
		DefaultTPM20Utils:      &biz.DefaultTPM20Utils{},
		DefaultTpmCertVerifier: &biz.DefaultTpmCertVerifier{},
		enrollzC:               enrollzC,
		ROTDBClient:            rotDB,
		SwitchOwnerCaClient:    ownerCA,
	}
}

type iakCertBaseline struct {
	activeSHA256  [32]byte
	standbySHA256 [32]byte
	activeTLSCert [32]byte
	useTLSCert    bool
	hasStandby    bool
}

type oIAKOnlyForStandbyCAClient struct {
	biz.SwitchOwnerCaClient
}

func (c *oIAKOnlyForStandbyCAClient) IssueOwnerIDevIDCert(ctx context.Context, req *biz.IssueOwnerIDevIDCertReq) (*biz.IssueOwnerIDevIDCertResp, error) {
	if req.CardID.GetControlCardRole() == cpb.ControlCardRole_CONTROL_CARD_ROLE_STANDBY {
		return &biz.IssueOwnerIDevIDCertResp{OwnerIDevIDCertPem: ""}, nil
	}
	return c.SwitchOwnerCaClient.IssueOwnerIDevIDCert(ctx, req)
}

type ROTDBClient struct {
	t       *testing.T
	dut     *ondatra.DUTDevice
	keyType epb.Key
}

func newROTDBClient(t *testing.T, dut *ondatra.DUTDevice, keyType epb.Key) *ROTDBClient {
	return &ROTDBClient{t: t, dut: dut, keyType: keyType}
}

func (c *ROTDBClient) FetchEK(ctx context.Context, req *biz.FetchEKReq) (*biz.FetchEKResp, error) {
	switch c.keyType {
	case epb.Key_KEY_PPK:
		return fetchPPKPubFromDut(c, ctx)
	case epb.Key_KEY_EK:
		return fetchEKFromDUT(c, ctx, req)
	default:
		return nil, fmt.Errorf("unsupported key type: %v", c.keyType)
	}
}

type ownerCAClient struct {
	ca     *x509.Certificate
	caKey  *rsa.PrivateKey
	ipSANs []net.IP
}

func newOwnerCAClient(t *testing.T, dut *ondatra.DUTDevice) *ownerCAClient {
	t.Helper()
	ca, caKey := generateTestCA(t)
	addr := introspect.DUTDialer(t, dut, introspect.GNSI).DialTarget
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	var ipSANs []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ipSANs = []net.IP{ip}
	}
	return &ownerCAClient{ca: ca, caKey: caKey, ipSANs: ipSANs}
}

func (c *ownerCAClient) IssueOwnerIakCert(ctx context.Context, req *biz.IssueOwnerIakCertReq) (*biz.IssueOwnerIakCertResp, error) {
	certPEM, err := signPubKeyPEM(req.IakPubPem, c.ca, c.caKey, nil)
	if err != nil {
		return nil, fmt.Errorf("sign oIAK cert: %w", err)
	}
	return &biz.IssueOwnerIakCertResp{OwnerIakCertPem: certPEM}, nil
}

func (c *ownerCAClient) IssueOwnerIDevIDCert(ctx context.Context, req *biz.IssueOwnerIDevIDCertReq) (*biz.IssueOwnerIDevIDCertResp, error) {
	certPEM, err := signPubKeyPEM(req.IDevIDPubPem, c.ca, c.caKey, c.ipSANs)
	if err != nil {
		return nil, fmt.Errorf("sign oIDevID cert: %w", err)
	}
	return &biz.IssueOwnerIDevIDCertResp{OwnerIDevIDCertPem: certPEM}, nil
}

func (c *ownerCAClient) IssueAikCert(ctx context.Context, req *biz.IssueAikCertReq) (*biz.IssueAikCertResp, error) {
	certPEM, err := signPubKeyPEM(req.AikPubPem, c.ca, c.caKey, nil)
	if err != nil {
		return nil, fmt.Errorf("sign AIK cert: %w", err)
	}
	return &biz.IssueAikCertResp{AikCertPem: certPEM}, nil
}

func TestEnrollzTPM20HMAC(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := t.Context()
	dutTime = systemTime(t, dut)
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		t.Fatalf("os.Getwd() failed: %v", cwdErr)
	}
	enrollzCert = filepath.Join(cwd, enrollzCertFilename)
	enrollzKey = filepath.Join(cwd, enrollzKeyFilename)
	checkFilesExist(t, enrollzCert, enrollzKey)

	ownerCA := newOwnerCAClient(t, dut)
	clientCert, err := tls.LoadX509KeyPair(enrollzCert, enrollzKey)
	if err != nil {
		t.Fatalf("load enrollz cert/key for TLS config: %v", err)
	}
	caPool := x509.NewCertPool()
	caPool.AddCert(ownerCA.ca)

	enrollzTLSCfg := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
	}

	initialTLSCfg := &tls.Config{
		Certificates:       []tls.Certificate{clientCert},
		InsecureSkipVerify: true,
	}

	enrollzC, err := enrollzClientWithTLS(t, dut, initialTLSCfg)
	if err != nil {
		t.Fatalf("enrollzClientWithTLS initial connection: %v", err)
	}
	enrollSels, hasStandby := enrollmentSelections(t, ctx, enrollzC)

	var baseline iakCertBaseline

	unspecifiedCard := func() *cpb.ControlCardSelection {
		return &cpb.ControlCardSelection{
			ControlCardId: &cpb.ControlCardSelection_Role{Role: cpb.ControlCardRole_CONTROL_CARD_ROLE_UNSPECIFIED},
		}
	}
	minimalChallenge := func() *epb.HMACChallenge {
		return &epb.HMACChallenge{HmacPubKey: []byte("k"), Duplicate: []byte("d"), InSymSeed: []byte("s")}
	}

	for _, tc := range []struct {
		id   string
		desc string
		fn   func(t *testing.T) error
	}{
		{
			"enrollz-1.1-SuccessfulEnrollmentWithEK",
			"Successful enrollment using EK stored in the RoT database",
			func(t *testing.T) error {
				edRotDB := newROTDBClient(t, dut, epb.Key_KEY_EK)
				deps := newHMACEnrollDeps(enrollzC, edRotDB, ownerCA)
				if err := enrollControlCards(ctx, deps, defaultSSLProfile, enrollSels...); err != nil {
					if !isEnrollmentRestart(err) {
						return fmt.Errorf("EK-based enrollment failed : %w", err)
					}
					t.Logf("EK-based enrollment triggered service restart (expected): %v", err)
				}
				waitForDUTEnrollzEnrollment(t, dut)
				enrollzC, err := enrollzClientWithTLS(t, dut, enrollzTLSCfg)
				if err != nil {
					return err
				}
				if err := verifyCerts(t, ctx, dut, enrollzC, ownerCA, hasStandby, nil); err != nil {
					return err
				}
				baseline = captureIAKBaseline(t, ctx, dut, enrollzC, hasStandby)
				t.Log("EK-based enrollment succeeded – oIAK and oIDevID installed on all control cards")
				return nil
			},
		},
		{
			"enrollz-1.2-SuccessfulEnrollmentWithPPK",
			"Successful enrollment using PPK public key stored in the RoT database",
			func(t *testing.T) error {
				ppkRotDB := newROTDBClient(t, dut, epb.Key_KEY_PPK)
				deps := newHMACEnrollDeps(enrollzC, ppkRotDB, ownerCA)
				if err := enrollControlCards(ctx, deps, defaultSSLProfile, enrollSels...); err != nil {
					if !isEnrollmentRestart(err) {
						return fmt.Errorf("PPK-based enrollment failed : %w", err)
					}
					t.Logf("PPK-based enrollment triggered service restart (expected): %v", err)
				}
				waitForDUTEnrollzEnrollment(t, dut)
				enrollzC, err := enrollzClientWithTLS(t, dut, enrollzTLSCfg)
				if err != nil {
					return err
				}
				if err := verifyCerts(t, ctx, dut, enrollzC, ownerCA, hasStandby, nil); err != nil {
					return err
				}
				baseline = captureIAKBaseline(t, ctx, dut, enrollzC, hasStandby)
				t.Log("PPK-based enrollment succeeded – oIAK and oIDevID installed on all control cards")
				return nil
			},
		},
		{
			"enrollz-1.3-InvalidClientCertificate",
			"Enrollz service presents an invalid client certificate",
			func(t *testing.T) error {
				badClient, err := enrollzClientWithTLS(t, dut, invalidTLSConfig(t))
				if err != nil {
					t.Logf("connection rejected at dial time (expected): %v", err)
					return nil
				}
				_, err = badClient.GetControlCardVendorID(ctx, &epb.GetControlCardVendorIDRequest{
					ControlCardSelection: activeCard(),
				})
				return requireConnectionFailure(t, err)
			},
		},
		{
			"enrollz-1.4-MissingControlCardSelection",
			"GetControlCardVendorIDRequest with missing control_card_selection",
			func(t *testing.T) error {
				_, err := enrollzC.GetControlCardVendorID(ctx, &epb.GetControlCardVendorIDRequest{})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.5-InvalidControlCardSelection",
			"GetControlCardVendorIDRequest with invalid control_card_selection",
			func(t *testing.T) error {
				_, err := enrollzC.GetControlCardVendorID(ctx, &epb.GetControlCardVendorIDRequest{
					ControlCardSelection: unspecifiedCard(),
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.6-MissingControlCardSelection",
			"ChallengeRequest with missing control_card_selection",
			func(t *testing.T) error {
				_, err := enrollzC.Challenge(ctx, &epb.ChallengeRequest{
					Challenge: minimalChallenge(),
					Key:       epb.Key_KEY_EK,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.7-InvalidControlCardSelection",
			"ChallengeRequest with invalid control_card_selection",
			func(t *testing.T) error {
				_, err := enrollzC.Challenge(ctx, &epb.ChallengeRequest{
					ControlCardSelection: unspecifiedCard(),
					Challenge:            minimalChallenge(),
					Key:                  epb.Key_KEY_EK,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.8-InvalidKeyType",
			"ChallengeRequest with invalid key type KEY_UNSPECIFIED",
			func(t *testing.T) error {
				_, err := enrollzC.Challenge(ctx, &epb.ChallengeRequest{
					ControlCardSelection: activeCard(),
					Challenge:            minimalChallenge(),
					Key:                  epb.Key_KEY_UNSPECIFIED,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.9-EmptyHmacPubKey",
			"ChallengeRequest with empty hmac_pub_key in HMACChallenge",
			func(t *testing.T) error {
				_, err := enrollzC.Challenge(ctx, &epb.ChallengeRequest{
					ControlCardSelection: activeCard(),
					Challenge:            &epb.HMACChallenge{Duplicate: []byte("d"), InSymSeed: []byte("s")},
					Key:                  epb.Key_KEY_EK,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.10-EmptyDuplicate",
			"ChallengeRequest with empty duplicate field in HMACChallenge",
			func(t *testing.T) error {
				_, err := enrollzC.Challenge(ctx, &epb.ChallengeRequest{
					ControlCardSelection: activeCard(),
					Challenge:            &epb.HMACChallenge{HmacPubKey: []byte("k"), InSymSeed: []byte("s")},
					Key:                  epb.Key_KEY_EK,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.11-EmptyInSymSeed",
			"ChallengeRequest with empty in_sym_seed field in HMACChallenge",
			func(t *testing.T) error {
				_, err := enrollzC.Challenge(ctx, &epb.ChallengeRequest{
					ControlCardSelection: activeCard(),
					Challenge:            &epb.HMACChallenge{HmacPubKey: []byte("k"), Duplicate: []byte("d")},
					Key:                  epb.Key_KEY_EK,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.12-MalformedHmacPubKey",
			"ChallengeRequest with malformed hmac_pub_key in HMACChallenge",
			func(t *testing.T) error {
				_, err := enrollzC.Challenge(ctx, &epb.ChallengeRequest{
					ControlCardSelection: activeCard(),
					Challenge:            &epb.HMACChallenge{HmacPubKey: []byte("not-valid-tpm-blob"), Duplicate: []byte("d"), InSymSeed: []byte("s")},
					Key:                  epb.Key_KEY_EK,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.13-MalformedDuplicate",
			"ChallengeRequest with malformed duplicate in HMACChallenge",
			func(t *testing.T) error {
				_, err := enrollzC.Challenge(ctx, &epb.ChallengeRequest{
					ControlCardSelection: activeCard(),
					Challenge:            &epb.HMACChallenge{HmacPubKey: []byte("k"), Duplicate: []byte("not-valid-duplicate"), InSymSeed: []byte("s")},
					Key:                  epb.Key_KEY_EK,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.14-MalformedInSymSeed",
			"ChallengeRequest with malformed in_sym_seed in HMACChallenge",
			func(t *testing.T) error {
				_, err := enrollzC.Challenge(ctx, &epb.ChallengeRequest{
					ControlCardSelection: activeCard(),
					Challenge:            &epb.HMACChallenge{HmacPubKey: []byte("k"), Duplicate: []byte("d"), InSymSeed: []byte("not-valid-seed")},
					Key:                  epb.Key_KEY_EK,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.15-InvalidEKWrapping",
			"ChallengeRequest with correct key type KEY_EK but challenge wrapped with invalid EK",
			func(t *testing.T) error {
				wrongKey, keyErr := rsa.GenerateKey(rand.Reader, rsaKeyBits)
				if keyErr != nil {
					return fmt.Errorf("failed to generate RSA key: %w", keyErr)
				}
				wrongPub, keyErr := x509.MarshalPKIXPublicKey(&wrongKey.PublicKey)
				if keyErr != nil {
					return fmt.Errorf("failed to marshal public key: %w", keyErr)
				}
				_, err := enrollzC.Challenge(ctx, &epb.ChallengeRequest{
					ControlCardSelection: activeCard(),
					Challenge:            &epb.HMACChallenge{HmacPubKey: wrongPub, Duplicate: []byte("d-wrong-ek"), InSymSeed: []byte("s-wrong-ek")},
					Key:                  epb.Key_KEY_EK,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.16-InvalidPPKWrapping",
			"ChallengeRequest with correct key type KEY_PPK but challenge wrapped with invalid PPK",
			func(t *testing.T) error {
				wrongKey, keyErr := rsa.GenerateKey(rand.Reader, rsaKeyBits)
				if keyErr != nil {
					return fmt.Errorf("failed to generate RSA key: %w", keyErr)
				}
				wrongPub, keyErr := x509.MarshalPKIXPublicKey(&wrongKey.PublicKey)
				if keyErr != nil {
					return fmt.Errorf("failed to marshal public key: %w", keyErr)
				}
				_, err := enrollzC.Challenge(ctx, &epb.ChallengeRequest{
					ControlCardSelection: activeCard(),
					Challenge:            &epb.HMACChallenge{HmacPubKey: wrongPub, Duplicate: []byte("d-wrong-ppk"), InSymSeed: []byte("s-wrong-ppk")},
					Key:                  epb.Key_KEY_PPK,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.17-MissingControlCardSelection",
			"GetIdevidCsrRequest with missing control_card_selection",
			func(t *testing.T) error {
				_, err := enrollzC.GetIdevidCsr(ctx, &epb.GetIdevidCsrRequest{
					KeyTemplate: epb.KeyTemplate_KEY_TEMPLATE_ECC_NIST_P384,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.18-InvalidControlCardSelection",
			"GetIdevidCsrRequest with invalid control_card_selection",
			func(t *testing.T) error {
				_, err := enrollzC.GetIdevidCsr(ctx, &epb.GetIdevidCsrRequest{
					ControlCardSelection: unspecifiedCard(),
					KeyTemplate:          epb.KeyTemplate_KEY_TEMPLATE_ECC_NIST_P384,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.19-InvalidKeyType",
			"GetIdevidCsrRequest with invalid key type KEY_UNSPECIFIED",
			func(t *testing.T) error {
				_, err := enrollzC.GetIdevidCsr(ctx, &epb.GetIdevidCsrRequest{
					ControlCardSelection: activeCard(),
					Key:                  epb.Key_KEY_UNSPECIFIED,
					KeyTemplate:          epb.KeyTemplate_KEY_TEMPLATE_ECC_NIST_P384,
				})
				return requireRejectedRequest(t, err)
			},
		},
		{
			"enrollz-1.20-UnsupportedKeyTemplate",
			"GetIdevidCsrRequest with unsupported KeyTemplate → STATUS_UNSUPPORTED",
			func(t *testing.T) error {
				resp, err := enrollzC.GetIdevidCsr(ctx, &epb.GetIdevidCsrRequest{
					ControlCardSelection: activeCard(),
					KeyTemplate:          epb.KeyTemplate_KEY_TEMPLATE_UNSPECIFIED,
				})
				if err != nil {
					if status.Code(err) == codes.InvalidArgument &&
						strings.Contains(strings.ToLower(err.Error()), "unsupported") {
						t.Logf("enrollz-1.20: DUT returned RPC InvalidArgument for unsupported key template (acceptable): %v", err)
						return nil
					}
					return fmt.Errorf("enrollz-1.20: unexpected RPC error (want STATUS_UNSUPPORTED response or InvalidArgument): %w", err)
				}
				if resp.GetStatus() != epb.Status_STATUS_UNSUPPORTED {
					return fmt.Errorf("expected STATUS_UNSUPPORTED, got %v", resp.GetStatus())
				}
				return nil
			},
		},
		{
			"enrollz-1.21-InvalidControlCard",
			"RotateOIakCert with invalid control card → rollback",
			func(t *testing.T) error {
				_, err := enrollzC.RotateOIakCert(ctx, &epb.RotateOIakCertRequest{
					Updates: []*epb.ControlCardCertUpdate{{
						ControlCardSelection: &cpb.ControlCardSelection{ControlCardId: &cpb.ControlCardSelection_Slot{Slot: "invalid-slot-xyz"}},
						OiakCert:             "fake", OidevidCert: "fake",
					}},
					SslProfileId: defaultSSLProfile,
				})
				if err := requireRejectedRequest(t, err); err != nil {
					return err
				}
				return verifyCerts(t, ctx, dut, enrollzC, nil, baseline.hasStandby, &baseline)
			},
		},
		{
			"enrollz-1.22-MissingSslProfileId",
			"RotateOIakCert with oidevid_cert but missing ssl_profile_id",
			func(t *testing.T) error {
				_, err := enrollzC.RotateOIakCert(ctx, &epb.RotateOIakCertRequest{
					Updates: []*epb.ControlCardCertUpdate{{
						ControlCardSelection: activeCard(),
						OiakCert:             "fake", OidevidCert: "fake",
					}},
				})
				if err := requireRejectedRequest(t, err); err != nil {
					return err
				}
				return verifyCerts(t, ctx, dut, enrollzC, nil, baseline.hasStandby, &baseline)
			},
		},
		{
			"enrollz-1.23-MalformedOIakCert",
			"RotateOIakCert with malformed oIAK certificate",
			func(t *testing.T) error {
				_, err := enrollzC.RotateOIakCert(ctx, &epb.RotateOIakCertRequest{
					Updates: []*epb.ControlCardCertUpdate{{
						ControlCardSelection: activeCard(),
						OiakCert:             "not-a-pem-cert",
						OidevidCert:          generateSelfSignedCertPEM(t),
					}},
					SslProfileId: defaultSSLProfile,
				})
				if err := requireRejectedRequest(t, err); err != nil {
					return err
				}
				return verifyCerts(t, ctx, dut, enrollzC, nil, baseline.hasStandby, &baseline)
			},
		},
		{
			"enrollz-1.24-MismatchedIAKKey",
			"RotateOIakCert with oIAK having mismatched IAK public key",
			func(t *testing.T) error {
				oiak := generateSelfSignedCertPEM(t)
				oidevid := generateSelfSignedCertPEM(t)
				_, err := enrollzC.RotateOIakCert(ctx, &epb.RotateOIakCertRequest{
					Updates:      []*epb.ControlCardCertUpdate{{ControlCardSelection: activeCard(), OiakCert: oiak, OidevidCert: oidevid}},
					SslProfileId: defaultSSLProfile,
				})
				if err := requireRejectedRequest(t, err); err != nil {
					return err
				}
				return verifyCerts(t, ctx, dut, enrollzC, nil, baseline.hasStandby, &baseline)
			},
		},
		{
			"enrollz-1.25-InvalidOIakSignature",
			"RotateOIakCert with valid oIAK chain but invalid signature",
			func(t *testing.T) error {
				oiak := certWithBadSignature(t, ownerCA)
				oidevid := generateSelfSignedCertPEM(t)
				_, err := enrollzC.RotateOIakCert(ctx, &epb.RotateOIakCertRequest{
					Updates:      []*epb.ControlCardCertUpdate{{ControlCardSelection: activeCard(), OiakCert: oiak, OidevidCert: oidevid}},
					SslProfileId: defaultSSLProfile,
				})
				if err := requireRejectedRequest(t, err); err != nil {
					return err
				}
				return verifyCerts(t, ctx, dut, enrollzC, nil, baseline.hasStandby, &baseline)
			},
		},
		{
			"enrollz-1.26-MalformedOIDevIDCert",
			"RotateOIakCert with malformed oIDevID certificate",
			func(t *testing.T) error {
				_, err := enrollzC.RotateOIakCert(ctx, &epb.RotateOIakCertRequest{
					Updates: []*epb.ControlCardCertUpdate{{
						ControlCardSelection: activeCard(),
						OiakCert:             generateSelfSignedCertPEM(t),
						OidevidCert:          "malformed-oidevid",
					}},
					SslProfileId: defaultSSLProfile,
				})
				if err := requireRejectedRequest(t, err); err != nil {
					return err
				}
				return verifyCerts(t, ctx, dut, enrollzC, nil, baseline.hasStandby, &baseline)
			},
		},
		{
			"enrollz-1.27-MismatchedIDevIDKey",
			"RotateOIakCert with oIDevID having mismatched IDevID public key",
			func(t *testing.T) error {
				oiak := generateSelfSignedCertPEM(t)
				oidevid := generateSelfSignedCertPEM(t)
				_, err := enrollzC.RotateOIakCert(ctx, &epb.RotateOIakCertRequest{
					Updates:      []*epb.ControlCardCertUpdate{{ControlCardSelection: activeCard(), OiakCert: oiak, OidevidCert: oidevid}},
					SslProfileId: defaultSSLProfile,
				})
				if err := requireRejectedRequest(t, err); err != nil {
					return err
				}
				return verifyCerts(t, ctx, dut, enrollzC, nil, baseline.hasStandby, &baseline)
			},
		},
		{
			"enrollz-1.28-InvalidOIDevIDSignature",
			"RotateOIakCert with valid oIDevID chain but invalid signature",
			func(t *testing.T) error {
				oiak := generateSelfSignedCertPEM(t)
				oidevid := certWithBadSignature(t, ownerCA)
				_, err := enrollzC.RotateOIakCert(ctx, &epb.RotateOIakCertRequest{
					Updates:      []*epb.ControlCardCertUpdate{{ControlCardSelection: activeCard(), OiakCert: oiak, OidevidCert: oidevid}},
					SslProfileId: defaultSSLProfile,
				})
				if err := requireRejectedRequest(t, err); err != nil {
					return err
				}
				return verifyCerts(t, ctx, dut, enrollzC, nil, baseline.hasStandby, &baseline)
			},
		},
		{
			"enrollz-1.29-MixedCardEnrollment",
			"RotateOIakCert: active gets oIAK+oIDevID, standby gets only oIAK",
			func(t *testing.T) error {
				if !hasStandby {
					return fmt.Errorf("no standby control card available")
				}
				mixedOwnerCA := &oIAKOnlyForStandbyCAClient{ownerCA}
				rotDB := newROTDBClient(t, dut, epb.Key_KEY_EK)
				deps := newHMACEnrollDeps(enrollzC, rotDB, mixedOwnerCA)
				if err := enrollControlCards(ctx, deps, defaultSSLProfile, activeCard(), standbyCard()); err != nil {
					if !isEnrollmentRestart(err) {
						return fmt.Errorf("mixed card enrollment failed: %w", err)
					}
					t.Logf("Mixed card enrollment triggered service restart (expected): %v", err)
				}
				waitForDUTEnrollzEnrollment(t, dut)
				enrollzC, err := enrollzClientWithTLS(t, dut, enrollzTLSCfg)
				if err != nil {
					return err
				}
				if err := verifyCerts(t, ctx, dut, enrollzC, ownerCA, hasStandby, nil); err != nil {
					return err
				}
				t.Log("RotateOIakCert with mixed oIAK/oIDevID succeeded")
				return nil
			},
		},
		{
			"enrollz-1.30-RebootAfterEnrollment",
			"Reboot after successful enrollment; certs must persist",
			func(t *testing.T) error {
				rotDB := newROTDBClient(t, dut, epb.Key_KEY_EK)
				deps := newHMACEnrollDeps(enrollzC, rotDB, ownerCA)
				if err := enrollControlCards(ctx, deps, defaultSSLProfile, enrollSels...); err != nil {
					if !isEnrollmentRestart(err) {
						return fmt.Errorf("EnrollSwitchWithHMACChallenge: %w", err)
					}
					t.Logf("Enrollment triggered service restart (expected): %v", err)
				}
				waitForDUTEnrollzEnrollment(t, dut)
				t.Log("Rebooting DUT after successful enrollment...")
				rebootDUT(t, dut)
				enrollzC, err := enrollzClientWithTLS(t, dut, enrollzTLSCfg)
				if err != nil {
					return err
				}
				if err := verifyCerts(t, ctx, dut, enrollzC, ownerCA, hasStandby, nil); err != nil {
					return err
				}
				t.Log("Post-reboot certificates verified")
				return nil
			},
		},
		{
			"enrollz-1.31-RebootAfterGetIdevidCsrAndReEnroll",
			"Reboot after GetIdevidCsr (mid-enrollment); re-enroll successfully",
			func(t *testing.T) error {
				probeCtx, cancel := context.WithTimeout(context.Background(), probeTimeout)
				defer cancel()
				if _, err := enrollzC.GetIdevidCsr(probeCtx, &epb.GetIdevidCsrRequest{
					ControlCardSelection: activeCard(),
					KeyTemplate:          epb.KeyTemplate_KEY_TEMPLATE_ECC_NIST_P384,
					Key:                  epb.Key_KEY_EK,
				}); err != nil {
					return fmt.Errorf("GetIdevidCsr failed before reboot: %w", err)
				}
				rebootDUT(t, dut)
				freshRotDB := newROTDBClient(t, dut, epb.Key_KEY_EK)
				deps := newHMACEnrollDeps(enrollzC, freshRotDB, ownerCA)
				if err := enrollControlCards(ctx, deps, defaultSSLProfile, enrollSels...); err != nil {
					if !isEnrollmentRestart(err) {
						return fmt.Errorf("re-enrollment after reboot: %w", err)
					}
					t.Logf("Re-enrollment triggered service restart (expected): %v", err)
				}
				waitForDUTEnrollzEnrollment(t, dut)
				enrollzC, err := enrollzClientWithTLS(t, dut, enrollzTLSCfg)
				if err != nil {
					return err
				}
				if err := verifyCerts(t, ctx, dut, enrollzC, ownerCA, hasStandby, nil); err != nil {
					return err
				}
				t.Log("Re-enrollment after reboot succeeded")
				return nil
			},
		},
	} {
		t.Run(tc.id, func(t *testing.T) {
			t.Logf("Test description: %s", tc.desc)
			if err := tc.fn(t); err != nil {
				t.Errorf("%s: %v", tc.id, err)
			}
		})
	}
}

func checkFilesExist(t *testing.T, files ...string) {
	for _, file := range files {
		t.Logf("Checking file: %s", file)
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Fatalf("file does not exist: %s", file)
		} else if err != nil {
			t.Fatalf("error checking file %s: %v", file, err)
		}
	}
}

func enrollzClientWithTLS(t *testing.T, dut *ondatra.DUTDevice, tlsCfg *tls.Config) (epb.TpmEnrollzServiceClient, error) {
	t.Helper()
	gnsiDialer := introspect.DUTDialer(t, dut, introspect.GNSI)
	opts := append(gnsiDialer.DialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	conn, err := grpc.NewClient(gnsiDialer.DialTarget, opts...)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { conn.Close() })
	return epb.NewTpmEnrollzServiceClient(conn), nil
}

func dutMTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	clientCert, err := tls.LoadX509KeyPair(enrollzCert, enrollzKey)
	if err != nil {
		t.Fatalf("dutMTLSConfig: load enrollz cert/key: %v", err)
	}
	return &tls.Config{
		Certificates:       []tls.Certificate{clientCert},
		InsecureSkipVerify: true,
	}
}

func fetchPPKPubFromDut(c *ROTDBClient, ctx context.Context) (*biz.FetchEKResp, error) {
	gnoiDialer := introspect.DUTDialer(c.t, c.dut, introspect.GNOI)
	opts := append(gnoiDialer.DialOpts, grpc.WithTransportCredentials(credentials.NewTLS(dutMTLSConfig(c.t))))
	conn, err := grpc.NewClient(gnoiDialer.DialTarget, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial for PPK fetch: %w", err)
	}
	defer conn.Close()
	stream, err := fpb.NewFileClient(conn).Get(ctx, &fpb.GetRequest{RemoteFile: dutPPKFilePath})
	if err != nil {
		return nil, fmt.Errorf("File.Get(%s): %w\nEnsure DUT ppk retrieval sequence has run: /usr/bin/tpm2_readPubPpk > %s", dutPPKFilePath, err, dutPPKFilePath)
	}
	var content []byte
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("File.Get stream: %w", err)
		}
		content = append(content, resp.GetContents()...)
	}
	block, _ := pem.Decode(content)
	if block == nil {
		return nil, fmt.Errorf("%s on DUT: no PEM block in content: %q", dutPPKFilePath, strings.TrimSpace(string(content)))
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%s: parse PUBLIC KEY: %w", dutPPKFilePath, err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%s: expected RSA public key, got %T", dutPPKFilePath, pub)
	}
	return &biz.FetchEKResp{EkPublicKey: rsaPub, KeyType: epb.Key_KEY_PPK}, nil
}

func fetchEKFromDUT(c *ROTDBClient, ctx context.Context, req *biz.FetchEKReq) (*biz.FetchEKResp, error) {
	gnsiDialer := introspect.DUTDialer(c.t, c.dut, introspect.GNSI)
	opts := append(gnsiDialer.DialOpts, grpc.WithTransportCredentials(credentials.NewTLS(dutMTLSConfig(c.t))))
	conn, err := grpc.NewClient(gnsiDialer.DialTarget, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial for EK fetch: %w", err)
	}
	defer conn.Close()
	enrollzC := epb.NewTpmEnrollzServiceClient(conn)
	cardSel := &cpb.ControlCardSelection{
		ControlCardId: &cpb.ControlCardSelection_Serial{Serial: req.Serial},
	}
	idevidResp, err := enrollzC.GetIdevidCsr(ctx, &epb.GetIdevidCsrRequest{
		ControlCardSelection: cardSel,
		Key:                  c.keyType,
		KeyTemplate:          epb.KeyTemplate_KEY_TEMPLATE_ECC_NIST_P384,
	})
	if err != nil {
		return nil, fmt.Errorf("GetIdevidCsr for serial %q: %w", req.Serial, err)
	}
	csrBytes := idevidResp.GetCsrResponse().GetCsrContents()
	if len(csrBytes) == 0 {
		return nil, fmt.Errorf("GetIdevidCsr returned empty csr_contents for serial %q (status=%v)", req.Serial, idevidResp.GetStatus())
	}
	csrContent, err := (&biz.DefaultTPM20Utils{}).ParseTCGCSRIDevIDContent(csrBytes)
	if err != nil {
		return nil, fmt.Errorf("parse TCG CSR IDevID content for serial %q: %w", req.Serial, err)
	}
	return ekRSAPubFromCSRContent(csrContent, req.Serial, c.keyType)
}

func gnsiAddr(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	return introspect.DUTDialer(t, dut, introspect.GNSI).DialTarget
}

func invalidTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "invalid-test-client"},
		NotBefore:    dutTime.Add(10 * time.Minute),
		NotAfter:     dutTime.Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
	}
}

func activeCard() *cpb.ControlCardSelection {
	return &cpb.ControlCardSelection{
		ControlCardId: &cpb.ControlCardSelection_Role{
			Role: cpb.ControlCardRole_CONTROL_CARD_ROLE_ACTIVE,
		},
	}
}

func standbyCard() *cpb.ControlCardSelection {
	return &cpb.ControlCardSelection{
		ControlCardId: &cpb.ControlCardSelection_Role{
			Role: cpb.ControlCardRole_CONTROL_CARD_ROLE_STANDBY,
		},
	}
}

func requireRejectedRequest(t *testing.T, err error) error {
	if err == nil {
		return fmt.Errorf("expected an error but got nil")
	}
	switch status.Code(err) {
	case codes.InvalidArgument, codes.FailedPrecondition, codes.Unknown:
		t.Logf("DUT rejected request with expected error: %v", err)
		return nil
	default:
		unwrapped := errors.Unwrap(err)
		if status.Code(unwrapped) == codes.InvalidArgument || status.Code(unwrapped) == codes.FailedPrecondition {
			t.Logf("DUT rejected request with expected inner error: %v", unwrapped)
			return nil
		}
		return fmt.Errorf("expected DUT to reject request with an invalid request error, got %v: %v", status.Code(err), err)
	}
}

func requireConnectionFailure(t *testing.T, err error) error {
	if err == nil {
		return fmt.Errorf("expected gRPC connection failure, got nil")
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.Unauthenticated, codes.PermissionDenied, codes.InvalidArgument:
		t.Logf("connection failed with expected error code %v: %v", status.Code(err), err)
		return nil
	default:
		return fmt.Errorf("expected gRPC connection failure, got %v", err)
	}
}

func enrollmentSelections(t *testing.T, ctx context.Context, enrollzC epb.TpmEnrollzServiceClient) ([]*cpb.ControlCardSelection, bool) {
	t.Helper()
	sels := []*cpb.ControlCardSelection{activeCard()}
	if _, err := enrollzC.GetControlCardVendorID(ctx, &epb.GetControlCardVendorIDRequest{
		ControlCardSelection: standbyCard(),
	}); err != nil {
		if status.Code(err) == codes.InvalidArgument {
			t.Logf("No standby control card present %v; running enrollment tests against the active control card only", err)
			return sels, false
		}
		t.Fatalf("standby GetControlCardVendorID probe failed: %v", err)
	}
	return append(sels, standbyCard()), true
}

func isEnrollmentRestart(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "eof") || status.Code(err) == codes.Unavailable
}

func needsReconfigure(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return status.Code(err) == codes.Unimplemented ||
		strings.Contains(msg, "tls handshake") ||
		strings.Contains(msg, "first record does not look like a tls handshake") ||
		strings.Contains(msg, "connection refused")
}

func waitForDUTEnrollzEnrollment(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Log("Waiting for DUT enrollz service to recover after enrollment...")
	time.Sleep(rebootProbeSleep)
	deadline := time.Now().Add(enrollzRestartTimeout)
	for time.Now().Before(deadline) {
		err := probeEnrollz(t, dut)
		if err == nil {
			t.Log("DUT enrollz service recovered after enrollment")
			return
		}
		t.Logf("enrollz probe failed: %v; retrying...", err)
		time.Sleep(retrySleepTime)
	}
	t.Fatal("DUT enrollz service did not recover within timeout after enrollment")
}

func probeGNMI(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	conn, err := insecureGNMIClient(t, dut)
	if err != nil {
		return fmt.Errorf("gnmi dial: %w", err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	_, err = gpb.NewGNMIClient(conn).Capabilities(ctx, &gpb.CapabilityRequest{})
	if err == nil || status.Code(err) == codes.InvalidArgument || status.Code(err) == codes.Unauthenticated {
		return nil
	}
	return err
}

func probeEnrollz(t *testing.T, dut *ondatra.DUTDevice) error {
	t.Helper()
	gnsiDialer := introspect.DUTDialer(t, dut, introspect.GNSI)
	opts := append(gnsiDialer.DialOpts, grpc.WithTransportCredentials(credentials.NewTLS(dutMTLSConfig(t))))
	conn, err := grpc.NewClient(gnsiDialer.DialTarget, opts...)
	if err != nil {
		return fmt.Errorf("probe dial: %w", err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	_, enrollzErr := epb.NewTpmEnrollzServiceClient(conn).GetControlCardVendorID(ctx, &epb.GetControlCardVendorIDRequest{
		ControlCardSelection: activeCard(),
	})
	return enrollzErr
}

func rebootDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	cfgplugins.BackUpConfig(t, dut, configBackupFile)

	gnoiDialer := introspect.DUTDialer(t, dut, introspect.GNOI)
	gnoiOpts := append(gnoiDialer.DialOpts, grpc.WithTransportCredentials(credentials.NewTLS(dutMTLSConfig(t))))
	conn, err := grpc.NewClient(gnoiDialer.DialTarget, gnoiOpts...)
	if err != nil {
		t.Fatalf("DialGNOI for reboot: %v", err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), rebootTimeout)
	defer cancel()
	if _, err := spb.NewSystemClient(conn).Reboot(ctx, &spb.RebootRequest{
		Method: spb.RebootMethod_COLD,
		Force:  true,
	}); err != nil {
		if status.Code(err) != codes.Unavailable {
			t.Fatalf("gNOI Reboot: %v", err)
		}
		t.Logf("gNOI Reboot connection dropped (expected): %v", err)
	}

	t.Log("Reboot RPC sent; waiting for DUT to go down...")
	time.Sleep(rebootProbeSleep)
	downDeadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(downDeadline) {
		if err := probeEnrollz(t, dut); err != nil {
			t.Logf("DUT is going down: %v", err)
			break
		}
		t.Log("DUT still reachable; waiting for reboot...")
		time.Sleep(rebootProbeSleep)
	}

	t.Log("Waiting for DUT to come back up...")
	deadline := time.Now().Add(rebootTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(rebootProbeSleep)
		if err := probeGNMI(t, dut); err != nil {
			t.Logf("DUT not yet reachable via gNMI: %v", err)
			continue
		}
		t.Log("DUT is back up")
		restoreRunningConfig(t, dut, configBackupFile)
		waitForDUTEnrollzEnrollment(t, dut)

		return
	}
	t.Fatal("DUT did not come back up within timeout after reboot")
}

func enrollControlCards(ctx context.Context, deps *hmacEnrollDeps, sslProfileID string, sels ...*cpb.ControlCardSelection) error {
	return biz.EnrollSwitchWithHMACChallenge(ctx, &biz.EnrollSwitchWithHMACChallengeReq{
		ControlCardSelections: sels,
		Deps:                  deps,
		SSLProfileID:          sslProfileID,
	})
}

func generateSelfSignedCertPEM(t *testing.T) string {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		t.Fatalf("generateSelfSignedCertPEM: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-wrong-cert"},
		NotBefore:    dutTime,
		NotAfter:     dutTime.Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("generateSelfSignedCertPEM create: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
}

func verifyCerts(t *testing.T, ctx context.Context, dut *ondatra.DUTDevice, enrollzC epb.TpmEnrollzServiceClient, ownerCA *ownerCAClient, hasStandby bool, baseline *iakCertBaseline) error {
	t.Helper()
	if baseline != nil {
		t.Log("Performing rollback check to ensure IAK certs were not changed by rejected RotateOIakCert...")
		if baseline.useTLSCert {
			raw, err := tlsRawServerCert(gnsiAddr(t, dut))
			if err != nil {
				return fmt.Errorf("rollback check: TLS server cert unavailable: %v", err)
			}
			if got := sha256.Sum256(raw); got != baseline.activeTLSCert {
				return fmt.Errorf("rollback check: oIDevID cert changed after rejected RotateOIakCert (TLS fingerprint mismatch)")
			}
			return nil
		}
		resp, err := enrollzC.GetIakCert(ctx, &epb.GetIakCertRequest{ControlCardSelection: activeCard()})
		if err != nil {
			return fmt.Errorf("rollback check: GetIakCert(active): %v", err)
		}
		var errs []error
		if got := sha256.Sum256([]byte(resp.GetIakCert())); got != baseline.activeSHA256 {
			errs = append(errs, fmt.Errorf("rollback check: active IAK cert changed after rejected RotateOIakCert (baseline=%x got=%x)", baseline.activeSHA256, got))
		}
		if baseline.hasStandby {
			resp, err := enrollzC.GetIakCert(ctx, &epb.GetIakCertRequest{ControlCardSelection: standbyCard()})
			if err != nil {
				errs = append(errs, fmt.Errorf("rollback check: GetIakCert(standby): %v", err))
			} else if got := sha256.Sum256([]byte(resp.GetIakCert())); got != baseline.standbySHA256 {
				errs = append(errs, fmt.Errorf("rollback check: standby IAK cert changed after rejected RotateOIakCert (baseline=%x got=%x)", baseline.standbySHA256, got))
			}
		}
		return errors.Join(errs...)
	}
	ownerCAPool := x509.NewCertPool()
	ownerCAPool.AddCert(ownerCA.ca)

	var errs []error

	raw, err := tlsRawServerCert(gnsiAddr(t, dut))
	if err != nil {
		t.Logf("verifyCerts: TLS server cert unavailable (plaintext mode or unreachable): %v", err)
	} else {
		cert, err := x509.ParseCertificate(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("verifyCerts: parse TLS server cert: %v", err))
		} else if cert.Issuer.CommonName != ownerCA.ca.Subject.CommonName {
			errs = append(errs, fmt.Errorf("verifyCerts: TLS server cert issuer CN = %q, want %q (oIDevID not in use)",
				cert.Issuer.CommonName, ownerCA.ca.Subject.CommonName))
		} else {
			t.Logf("verifyCerts: TLS server cert subject=%q issuer=%q (oIDevID confirmed)",
				cert.Subject.CommonName, cert.Issuer.CommonName)
		}
	}

	sels := []*cpb.ControlCardSelection{activeCard()}
	if hasStandby {
		sels = append(sels, standbyCard())
	}
	for _, sel := range sels {
		resp, err := enrollzC.GetIakCert(ctx, &epb.GetIakCertRequest{ControlCardSelection: sel})
		if err != nil {
			t.Logf("verifyCerts: GetIakCert unavailable for %v: %v", sel, err)
			continue
		}
		block, _ := pem.Decode([]byte(resp.GetIakCert()))
		if block == nil {
			errs = append(errs, fmt.Errorf("verifyCerts: oIAK cert for %v is not valid PEM", sel))
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			errs = append(errs, fmt.Errorf("verifyCerts: parse oIAK cert for %v: %v", sel, err))
			continue
		}
		if _, err := cert.Verify(x509.VerifyOptions{Roots: ownerCAPool}); err != nil {
			errs = append(errs, fmt.Errorf("verifyCerts: oIAK cert for %v not signed by ownerCA: %v", sel, err))
		} else {
			t.Logf("verifyCerts: oIAK cert for %v signed by %q – enrollment verified", sel, cert.Issuer.CommonName)
		}
	}
	return errors.Join(errs...)
}

func systemTime(t *testing.T, dut *ondatra.DUTDevice) time.Time {
	t.Helper()
	s := gnmi.Get(t, dut, gnmi.OC().System().CurrentDatetime().State())
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("Failed to parse DUT system time %q: %v", s, err)
	}
	return ts
}

func captureIAKBaseline(t *testing.T, ctx context.Context, dut *ondatra.DUTDevice, enrollzC epb.TpmEnrollzServiceClient, hasStandby bool) iakCertBaseline {
	t.Helper()
	b := iakCertBaseline{hasStandby: hasStandby}
	resp, err := enrollzC.GetIakCert(ctx, &epb.GetIakCertRequest{ControlCardSelection: activeCard()})
	if err != nil {
		t.Logf("GetIakCert(active) unavailable; trying TLS cert fingerprint as rollback baseline: %v", err)
		raw, tlsErr := tlsRawServerCert(gnsiAddr(t, dut))
		if tlsErr != nil {
			t.Fatalf("TLS fingerprint(active) unavailable, unable to check certificate rollback: %v", tlsErr)
			return b
		}
		b.activeTLSCert = sha256.Sum256(raw)
		b.useTLSCert = true
		return b
	}
	b.activeSHA256 = sha256.Sum256([]byte(resp.GetIakCert()))
	if hasStandby {
		resp, err := enrollzC.GetIakCert(ctx, &epb.GetIakCertRequest{ControlCardSelection: standbyCard()})
		if err != nil {
			t.Logf("GetIakCert(standby) unavailable: %v", err)
			return b
		}
		b.standbySHA256 = sha256.Sum256([]byte(resp.GetIakCert()))
	}
	return b
}

func generateTestCA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	caKey, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		t.Fatalf("generateTestCA: generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-owner-ca"},
		NotBefore:             dutTime,
		NotAfter:              dutTime.Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("generateTestCA: create cert: %v", err)
	}
	ca, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("generateTestCA: parse cert: %v", err)
	}
	return ca, caKey
}

func signPubKeyPEM(pubKeyPEM string, ca *x509.Certificate, caKey *rsa.PrivateKey, ipSANs []net.IP) (string, error) {
	if pubKeyPEM == "" {
		return "", fmt.Errorf("empty public key PEM")
	}
	block, _ := pem.Decode([]byte(pubKeyPEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block from public key")
	}

	var pub any
	var err error
	switch block.Type {
	case "PUBLIC KEY":
		pub, err = x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("parse PKIX public key: %w", err)
		}
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("parse certificate for re-signing: %w", err)
		}
		pub = cert.PublicKey
	default:
		return "", fmt.Errorf("unsupported PEM block type %q", block.Type)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "test-owner-cert"},
		NotBefore:    dutTime,
		NotAfter:     dutTime.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		IPAddresses:  ipSANs,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, ca, pub, caKey)
	if err != nil {
		return "", fmt.Errorf("create signed certificate: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})), nil
}

func ekRSAPubFromCSRContent(csr *biz.TCGCSRIDevIDContents, serial string, keyType epb.Key) (*biz.FetchEKResp, error) {
	if csr.EKCert == "" {
		return nil, fmt.Errorf("TCG CSR IDevID content has empty EK cert for serial %q", serial)
	}
	block, _ := pem.Decode([]byte(csr.EKCert))
	if block == nil {
		return nil, fmt.Errorf("EK cert for serial %q: failed to decode PEM block", serial)
	}
	var rsaPub *rsa.PublicKey
	switch block.Type {
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("EK cert for serial %q: parse x509: %w", serial, err)
		}
		pub, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("EK cert for serial %q: public key is not RSA (got %T)", serial, cert.PublicKey)
		}
		rsaPub = pub
	case "PUBLIC KEY":
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("EK pub for serial %q: parse PKIX: %w", serial, err)
		}
		pub2, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("EK pub for serial %q: not RSA (got %T)", serial, pub)
		}
		rsaPub = pub2
	default:
		return nil, fmt.Errorf("EK cert for serial %q: unsupported PEM block type %q", serial, block.Type)
	}
	return &biz.FetchEKResp{EkPublicKey: rsaPub, KeyType: keyType}, nil
}

func certWithBadSignature(t *testing.T, ownerCA *ownerCAClient) string {
	t.Helper()
	subjectKey, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		t.Fatalf("certWithBadSignature: generate subject key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "test-bad-signature"},
		NotBefore:    dutTime,
		NotAfter:     dutTime.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, ownerCA.ca, &subjectKey.PublicKey, ownerCA.caKey)
	if err != nil {
		t.Fatalf("certWithBadSignature: create cert: %v", err)
	}
	certDER[len(certDER)-1] ^= 0xFF
	certDER[len(certDER)-2] ^= 0xFF
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
}

func tlsRawServerCert(addr string) ([]byte, error) {
	var raw []byte
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) > 0 {
				raw = rawCerts[0]
			}
			return nil
		},
	})
	if err == nil {
		conn.Close()
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("no TLS certificate presented by %s", addr)
	}
	return raw, nil
}

func saveRunningConfigThroughGNMI(t *testing.T, dut *ondatra.DUTDevice) (string, error) {
	t.Helper()
	showCmd := cliShowRunningConfigCommand(t, dut)
	req := &gpb.GetRequest{
		Path: []*gpb.Path{{
			Origin: "cli",
			Elem:   []*gpb.PathElem{{Name: showCmd}},
		}},
		Encoding: gpb.Encoding_ASCII,
	}
	resp, err := dut.RawAPIs().GNMI(t).Get(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("baselineCLIConfig: gNMI Get CLI config: %w", err)
	}
	for _, notif := range resp.Notification {
		for _, update := range notif.Update {
			if s := update.Val.GetAsciiVal(); s != "" {
				return s, nil
			}
		}
	}
	return "", fmt.Errorf("baselineCLIConfig: no ASCII config in gNMI Get response")
}

func cliShowRunningConfigCommand(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.ARISTA, ondatra.CISCO:
		return "show running-config"
	default:
		t.Fatalf("unsupported vendor %v for CLI show running-config command", dut.Vendor())
		return ""
	}
}

type insecureRPCCreds struct{ username, password string }

func (c insecureRPCCreds) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{"username": c.username, "password": c.password}, nil
}
func (c insecureRPCCreds) RequireTransportSecurity() bool { return false }

func insecureGNMIClient(t *testing.T, dut *ondatra.DUTDevice) (*grpc.ClientConn, error) {
	t.Helper()
	username, password, err := binding.RPCCredentials(dut.RawAPIs().BindingDUT())
	if err != nil {
		return nil, fmt.Errorf("get binding credentials: %w", err)
	}
	gnmiTarget := introspect.DUTDialer(t, dut, introspect.GNMI).DialTarget
	return grpc.NewClient(gnmiTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(insecureRPCCreds{username: username, password: password}),
	)
}

func restoreRunningConfig(t *testing.T, dut *ondatra.DUTDevice, fileName string) {
	t.Helper()
	conn, err := insecureGNMIClient(t, dut)
	if err != nil {
		t.Fatalf("restoreRunningConfig: dial insecure gNMI: %v", err)
	}
	defer conn.Close()
	cmd := fmt.Sprintf("configure replace flash:%s", fileName)
	deadline := time.Now().Add(2 * time.Minute)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, setErr := gpb.NewGNMIClient(conn).Set(ctx, &gpb.SetRequest{
			Update: []*gpb.Update{{
				Path: &gpb.Path{Origin: "cli"},
				Val:  &gpb.TypedValue{Value: &gpb.TypedValue_AsciiVal{AsciiVal: cmd}},
			}},
		})
		cancel()
		if setErr == nil {
			t.Logf("Successfully restored DUT config from flash:%s", fileName)
			return
		}
		errStr := setErr.Error()
		if strings.Contains(errStr, "system not yet initialized") {
			t.Logf("DUT not fully initialized yet, retrying config restore...")
			if time.Now().After(deadline) {
				t.Fatalf("Timed out waiting for DUT initialization: %v", errStr)
			}
			time.Sleep(15 * time.Second)
			continue
		}
		if strings.Contains(errStr, "Octa agent") {
			t.Logf("Ignoring Octa informational warning during config restore: %v", errStr)
			return
		}
		t.Fatalf("Failed to restore DUT config via insecure gNMI: %v", errStr)
	}
}
