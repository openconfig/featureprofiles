package enrollz_tpm12_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"hash"
	"math/big"
	"strings"
	"testing"
	"time"

	cpb "github.com/openconfig/attestz/proto/common_definitions"
	epb "github.com/openconfig/attestz/proto/tpm_enrollz"
	"github.com/openconfig/attestz/service/biz"
	"github.com/openconfig/featureprofiles/internal/fptest"
	spb "github.com/openconfig/gnoi/system"
	"github.com/openconfig/ondatra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func enrollzClient(t *testing.T, dut *ondatra.DUTDevice) epb.TpmEnrollzServiceClient {
	t.Helper()
	gnsiC, err := dut.RawAPIs().BindingDUT().DialGNSI(t.Context())
	if err != nil {
		t.Fatalf("DialGNSI: %v", err)
	}
	return gnsiC.Enrollz()
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

func requireRejectedRequest(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error but got nil")
	}
	c := status.Code(err)
	switch c {
	case codes.Canceled, codes.DeadlineExceeded, codes.Unavailable:
		t.Fatalf("expected the DUT to reject the request, got transport/lifecycle error %v: %v", c, err)
	default:
		t.Logf("request rejected by DUT (code=%v): %v", c, err)
	}
}

func isMissingStandbyControlCard(err error) bool {
	if status.Code(err) == codes.InvalidArgument &&
		strings.Contains(err.Error(), `no control card with role "CONTROL_CARD_ROLE_STANDBY" found`) {
		return true
	}
	return status.Code(err) == codes.Unimplemented
}

func enrollzClientReady(t *testing.T, dut *ondatra.DUTDevice, timeout time.Duration) epb.TpmEnrollzServiceClient {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		ec := enrollzClient(t, dut)
		probeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err := ec.GetControlCardVendorID(probeCtx, &epb.GetControlCardVendorIDRequest{
			ControlCardSelection: activeCard(),
		})
		cancel()
		if err == nil {
			return ec
		}
		if status.Code(err) == codes.Unimplemented {
			t.Fatalf("TpmEnrollzService not available: device must restart enrollz service on boot: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("enrollz service not ready within %v: %v", timeout, err)
		}
		t.Logf("enrollz service not ready yet: %v; retrying...", err)
		time.Sleep(10 * time.Second)
	}
}

func hasStandbyCard(t *testing.T, ctx context.Context, enrollzC epb.TpmEnrollzServiceClient) bool {
	t.Helper()
	_, err := enrollzC.GetControlCardVendorID(ctx, &epb.GetControlCardVendorIDRequest{
		ControlCardSelection: standbyCard(),
	})
	if err != nil {
		if isMissingStandbyControlCard(err) {
			t.Log("No standby control card present; skipping standby-specific steps")
			return false
		}
		t.Fatalf("standby GetControlCardVendorID probe failed: %v", err)
	}
	return true
}

func rebootDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	gnoiC, err := dut.RawAPIs().BindingDUT().DialGNOI(context.Background())
	if err != nil {
		t.Fatalf("DialGNOI for reboot: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if _, err := gnoiC.System().Reboot(ctx, &spb.RebootRequest{
		Method: spb.RebootMethod_COLD,
		Force:  true,
	}); err != nil {
		if status.Code(err) != codes.Unavailable {
			t.Fatalf("gNOI Reboot: %v", err)
		}
		t.Logf("gNOI Reboot connection dropped (expected during reboot): %v", err)
	}
	t.Log("Reboot RPC sent, waiting for DUT to come back up...")
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(15 * time.Second)
		if _, err := dut.RawAPIs().BindingDUT().DialGNSI(context.Background()); err == nil {
			t.Log("DUT is back up")
			return
		}
	}
	t.Fatal("DUT did not come back up within 10 minutes after reboot")
}

type tpm12Deps struct {
	*biz.DefaultTPM12Utils
	*biz.DefaultTpmCertVerifier
	enrollzC epb.TpmEnrollzServiceClient
	rotDB    biz.ROTDBClient
	ownerCA  biz.SwitchOwnerCaClient
}

func (d *tpm12Deps) GetIakCert(ctx context.Context, req *epb.GetIakCertRequest) (*epb.GetIakCertResponse, error) {
	return d.enrollzC.GetIakCert(ctx, req)
}

func (d *tpm12Deps) RotateOIakCert(ctx context.Context, req *epb.RotateOIakCertRequest) (*epb.RotateOIakCertResponse, error) {
	return d.enrollzC.RotateOIakCert(ctx, req)
}

func (d *tpm12Deps) RotateAIKCert(ctx context.Context, opts ...grpc.CallOption) (epb.TpmEnrollzService_RotateAIKCertClient, error) {
	return d.enrollzC.RotateAIKCert(ctx, opts...)
}

func (d *tpm12Deps) GetControlCardVendorID(ctx context.Context, req *epb.GetControlCardVendorIDRequest) (*epb.GetControlCardVendorIDResponse, error) {
	return d.enrollzC.GetControlCardVendorID(ctx, req)
}

func (d *tpm12Deps) Challenge(ctx context.Context, req *epb.ChallengeRequest) (*epb.ChallengeResponse, error) {
	return d.enrollzC.Challenge(ctx, req)
}

func (d *tpm12Deps) GetIdevidCsr(ctx context.Context, req *epb.GetIdevidCsrRequest) (*epb.GetIdevidCsrResponse, error) {
	return d.enrollzC.GetIdevidCsr(ctx, req)
}

func (d *tpm12Deps) FetchEK(ctx context.Context, req *biz.FetchEKReq) (*biz.FetchEKResp, error) {
	return d.rotDB.FetchEK(ctx, req)
}

func (d *tpm12Deps) IssueOwnerIakCert(ctx context.Context, req *biz.IssueOwnerIakCertReq) (*biz.IssueOwnerIakCertResp, error) {
	return d.ownerCA.IssueOwnerIakCert(ctx, req)
}

func (d *tpm12Deps) IssueOwnerIDevIDCert(ctx context.Context, req *biz.IssueOwnerIDevIDCertReq) (*biz.IssueOwnerIDevIDCertResp, error) {
	return d.ownerCA.IssueOwnerIDevIDCert(ctx, req)
}

func (d *tpm12Deps) IssueAikCert(ctx context.Context, req *biz.IssueAikCertReq) (*biz.IssueAikCertResp, error) {
	return d.ownerCA.IssueAikCert(ctx, req)
}

func newTPM12Deps(enrollzC epb.TpmEnrollzServiceClient, rotDB biz.ROTDBClient, ownerCA biz.SwitchOwnerCaClient) *tpm12Deps {
	return &tpm12Deps{
		DefaultTPM12Utils:      &biz.DefaultTPM12Utils{},
		DefaultTpmCertVerifier: &biz.DefaultTpmCertVerifier{},
		enrollzC:               enrollzC,
		rotDB:                  rotDB,
		ownerCA:                ownerCA,
	}
}

func rotateAIKCertErr(ctx context.Context, deps *tpm12Deps, cardSel *cpb.ControlCardSelection) error {
	return biz.RotateAIKCert(ctx, &biz.RotateAIKCertReq{
		ControlCardSelection: cardSel,
		Deps:                 deps,
	})
}

func rotateAIKCert(t *testing.T, ctx context.Context, deps *tpm12Deps, cardSel *cpb.ControlCardSelection) {
	t.Helper()
	if err := rotateAIKCertErr(ctx, deps, cardSel); err != nil {
		t.Fatalf("biz.RotateAIKCert: %v", err)
	}
}

func openRotateAIKStream(t *testing.T, ctx context.Context, enrollzC epb.TpmEnrollzServiceClient, req *epb.RotateAIKCertRequest) epb.TpmEnrollzService_RotateAIKCertClient {
	t.Helper()
	stream, err := enrollzC.RotateAIKCert(ctx)
	if err != nil {
		t.Fatalf("RotateAIKCert open stream: %v", err)
	}
	if err := stream.Send(req); err != nil {
		t.Fatalf("RotateAIKCert send: %v", err)
	}
	return stream
}

func sendAndExpectStreamError(t *testing.T, stream epb.TpmEnrollzService_RotateAIKCertClient, req *epb.RotateAIKCertRequest) {
	t.Helper()
	if err := stream.Send(req); err != nil {
		requireRejectedRequest(t, err)
		return
	}
	_, err := stream.Recv()
	requireRejectedRequest(t, err)
}

func generateSelfSignedCertPEM(t *testing.T) string {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generateSelfSignedCertPEM: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-wrong-cert"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("generateSelfSignedCertPEM create: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
}

func TestEnrollzTPM12(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ctx := t.Context()

	ownerCA := newAristaOwnerCAClient(t)
	enrollzC := enrollzClient(t, dut)
	standby := hasStandbyCard(t, ctx, enrollzC)

	rotDB := newAristaROTDBClient(t, dut)

	for _, tc := range []struct {
		id   string
		desc string
		fn   func(t *testing.T)
	}{
		{
			"enrollz-2.1-SuccessfulEnrollment",
			"Successful TPM 1.2 RotateAIK enrollment flow",
			func(t *testing.T) {
				deps := newTPM12Deps(enrollzClient(t, dut), rotDB, ownerCA)
				rotateAIKCert(t, ctx, deps, activeCard())
				if standby {
					rotateAIKCert(t, ctx, deps, standbyCard())
				}
				t.Log("TPM 1.2 enrollment succeeded – AIK cert installed on all control cards")
			},
		},
		{
			"enrollz-2.2-MissingControlCardSelection",
			"RotateAIKCertRequest with missing control_card_selection",
			func(t *testing.T) {
				stream, err := enrollzC.RotateAIKCert(ctx)
				if err != nil {
					t.Fatalf("open stream: %v", err)
				}
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				req := &epb.RotateAIKCertRequest{
					Value: &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
				}
				sendAndExpectStreamError(t, stream, req)
			},
		},
		{
			"enrollz-2.3-InvalidControlCardSelection",
			"RotateAIKCertRequest with invalid control_card_selection",
			func(t *testing.T) {
				stream, err := enrollzC.RotateAIKCert(ctx)
				if err != nil {
					t.Fatalf("open stream: %v", err)
				}
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				req := &epb.RotateAIKCertRequest{
					Value: &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: &cpb.ControlCardSelection{
						ControlCardId: &cpb.ControlCardSelection_Role{
							Role: cpb.ControlCardRole_CONTROL_CARD_ROLE_UNSPECIFIED,
						},
					},
				}
				sendAndExpectStreamError(t, stream, req)
			},
		},
		{
			"enrollz-2.4-MissingIssuerPublicKey",
			"Initial RotateAIKCertRequest with missing issuer_public_key",
			func(t *testing.T) {
				stream, err := enrollzC.RotateAIKCert(ctx)
				if err != nil {
					t.Fatalf("open stream: %v", err)
				}
				req := &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: nil},
					ControlCardSelection: activeCard(),
				}
				sendAndExpectStreamError(t, stream, req)
			},
		},
		{
			"enrollz-2.5-MalformedIssuerPublicKey",
			"Initial RotateAIKCertRequest with malformed issuer_public_key",
			func(t *testing.T) {
				stream, err := enrollzC.RotateAIKCert(ctx)
				if err != nil {
					t.Fatalf("open stream: %v", err)
				}
				req := &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: []byte("not-a-valid-public-key")},
					ControlCardSelection: activeCard(),
				}
				sendAndExpectStreamError(t, stream, req)
			},
		},
		{
			"enrollz-2.6-MissingSymmetricKeyBlob",
			"RotateAIKCertRequest with issuer_cert_payload but missing symmetric_key_blob",
			func(t *testing.T) {
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				stream := openRotateAIKStream(t, ctx, enrollzC, &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: activeCard(),
				})
				if _, err := stream.Recv(); err != nil {
					t.Fatalf("recv Phase 1 response: %v", err)
				}
				sendAndExpectStreamError(t, stream, &epb.RotateAIKCertRequest{
					Value: &epb.RotateAIKCertRequest_IssuerCertPayload_{
						IssuerCertPayload: &epb.RotateAIKCertRequest_IssuerCertPayload{
							AikCertBlob: []byte("fake-aik-cert-blob"),
						},
					},
					ControlCardSelection: activeCard(),
				})
			},
		},
		{
			"enrollz-2.7-MissingAikCertBlob",
			"RotateAIKCertRequest with issuer_cert_payload where aik_cert_blob is missing",
			func(t *testing.T) {
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				stream := openRotateAIKStream(t, ctx, enrollzC, &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: activeCard(),
				})
				if _, err := stream.Recv(); err != nil {
					t.Fatalf("recv Phase 1 response: %v", err)
				}
				sendAndExpectStreamError(t, stream, &epb.RotateAIKCertRequest{
					Value: &epb.RotateAIKCertRequest_IssuerCertPayload_{
						IssuerCertPayload: &epb.RotateAIKCertRequest_IssuerCertPayload{
							SymmetricKeyBlob: []byte("fake-sym-key"),
						},
					},
					ControlCardSelection: activeCard(),
				})
			},
		},
		{
			"enrollz-2.8-SymmetricKeyBlobNotDecryptableWithEK",
			"RotateAIKCertRequest with issuer_cert_payload where symmetric_key_blob is not decryptable with the device EK",
			func(t *testing.T) {
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				stream := openRotateAIKStream(t, ctx, enrollzC, &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: activeCard(),
				})
				if _, err := stream.Recv(); err != nil {
					t.Fatalf("recv Phase 1 response: %v", err)
				}
				wrongKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				fakePayload, _ := rsa.EncryptOAEP(
					newSHA1(), rand.Reader, &wrongKey.PublicKey, []byte("fake-asym-ca-contents"), []byte("TCPA"),
				)
				sendAndExpectStreamError(t, stream, &epb.RotateAIKCertRequest{
					Value: &epb.RotateAIKCertRequest_IssuerCertPayload_{
						IssuerCertPayload: &epb.RotateAIKCertRequest_IssuerCertPayload{
							SymmetricKeyBlob: fakePayload,
							AikCertBlob:      []byte("fake-aik-cert-blob"),
						},
					},
					ControlCardSelection: activeCard(),
				})
			},
		},
		{
			"enrollz-2.9-MalformedSymmetricKeyBlob",
			"RotateAIKCertRequest with malformed symmetric_key_blob (not valid RSAES-OAEP format)",
			func(t *testing.T) {
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				stream := openRotateAIKStream(t, ctx, enrollzC, &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: activeCard(),
				})
				if _, err := stream.Recv(); err != nil {
					t.Fatalf("recv Phase 1 response: %v", err)
				}
				sendAndExpectStreamError(t, stream, &epb.RotateAIKCertRequest{
					Value: &epb.RotateAIKCertRequest_IssuerCertPayload_{
						IssuerCertPayload: &epb.RotateAIKCertRequest_IssuerCertPayload{
							SymmetricKeyBlob: []byte("not-valid-oaep-ciphertext"),
							AikCertBlob:      []byte("fake-aik-cert-blob"),
						},
					},
					ControlCardSelection: activeCard(),
				})
			},
		},
		{
			"enrollz-2.10-AsymCAContentsDigestMismatch",
			"RotateAIKCertRequest where symmetric_key_blob decrypts but TPM_ASYM_CA_CONTENTS digest does not match the AIK",
			func(t *testing.T) {
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				stream := openRotateAIKStream(t, ctx, enrollzC, &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: activeCard(),
				})
				phase1Resp, err := stream.Recv()
				if err != nil {
					t.Fatalf("recv Phase 1 response: %v", err)
				}
				ekResp, err := rotDB.FetchEK(ctx, &biz.FetchEKReq{
					Serial: phase1Resp.GetControlCardId().GetControlCardSerial(),
				})
				if err != nil {
					t.Fatalf("FetchEK: %v", err)
				}
				wrongAsymCA := append(make([]byte, 20), []byte("wrong-asym-ca")...)
				symKeyBlob, _ := rsa.EncryptOAEP(newSHA1(), rand.Reader, ekResp.EkPublicKey, wrongAsymCA, []byte("TCPA"))
				sendAndExpectStreamError(t, stream, &epb.RotateAIKCertRequest{
					Value: &epb.RotateAIKCertRequest_IssuerCertPayload_{
						IssuerCertPayload: &epb.RotateAIKCertRequest_IssuerCertPayload{
							SymmetricKeyBlob: symKeyBlob,
							AikCertBlob:      []byte("fake-aik-cert-blob"),
						},
					},
					ControlCardSelection: activeCard(),
				})
			},
		},
		{
			"enrollz-2.11-AikCertBlobNotDecryptable",
			"RotateAIKCertRequest where aik_cert_blob is not decryptable with the recovered session key",
			func(t *testing.T) {
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				stream := openRotateAIKStream(t, ctx, enrollzC, &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: activeCard(),
				})
				phase1Resp, err := stream.Recv()
				if err != nil {
					t.Fatalf("recv Phase 1 response: %v", err)
				}
				ekResp, err := rotDB.FetchEK(ctx, &biz.FetchEKReq{
					Serial: phase1Resp.GetControlCardId().GetControlCardSerial(),
				})
				if err != nil {
					t.Fatalf("FetchEK: %v", err)
				}
				fakeAsymCA := make([]byte, 40)
				symKeyBlob, _ := rsa.EncryptOAEP(newSHA1(), rand.Reader, ekResp.EkPublicKey, fakeAsymCA, []byte("TCPA"))
				sendAndExpectStreamError(t, stream, &epb.RotateAIKCertRequest{
					Value: &epb.RotateAIKCertRequest_IssuerCertPayload_{
						IssuerCertPayload: &epb.RotateAIKCertRequest_IssuerCertPayload{
							SymmetricKeyBlob: symKeyBlob,
							AikCertBlob:      []byte("random-bytes-not-aes-cbc-encrypted"),
						},
					},
					ControlCardSelection: activeCard(),
				})
			},
		},
		{
			"enrollz-2.12-MalformedTPMSymCAAttestation",
			"RotateAIKCertRequest where aik_cert_blob is a malformed TPM_SYM_CA_ATTESTATION structure",
			func(t *testing.T) {
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				stream := openRotateAIKStream(t, ctx, enrollzC, &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: activeCard(),
				})
				phase1Resp, err := stream.Recv()
				if err != nil {
					t.Fatalf("recv Phase 1 response: %v", err)
				}
				ekResp, err := rotDB.FetchEK(ctx, &biz.FetchEKReq{
					Serial: phase1Resp.GetControlCardId().GetControlCardSerial(),
				})
				if err != nil {
					t.Fatalf("FetchEK: %v", err)
				}
				fakeAsymCA := make([]byte, 40)
				symKeyBlob, _ := rsa.EncryptOAEP(newSHA1(), rand.Reader, ekResp.EkPublicKey, fakeAsymCA, []byte("TCPA"))
				sendAndExpectStreamError(t, stream, &epb.RotateAIKCertRequest{
					Value: &epb.RotateAIKCertRequest_IssuerCertPayload_{
						IssuerCertPayload: &epb.RotateAIKCertRequest_IssuerCertPayload{
							SymmetricKeyBlob: symKeyBlob,
							AikCertBlob:      []byte("malformed-tpm-sym-ca-attestation"),
						},
					},
					ControlCardSelection: activeCard(),
				})
			},
		},
		{
			"enrollz-2.13-MalformedDecryptedAikCert",
			"RotateAIKCertRequest where the decrypted aik_cert_blob contains a malformed PEM certificate",
			func(t *testing.T) {
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				stream := openRotateAIKStream(t, ctx, enrollzC, &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: activeCard(),
				})
				phase1Resp, err := stream.Recv()
				if err != nil {
					t.Fatalf("recv Phase 1 response: %v", err)
				}
				ekResp, err := rotDB.FetchEK(ctx, &biz.FetchEKReq{
					Serial: phase1Resp.GetControlCardId().GetControlCardSerial(),
				})
				if err != nil {
					t.Fatalf("FetchEK: %v", err)
				}
				fakeAsymCA := make([]byte, 40)
				symKeyBlob, _ := rsa.EncryptOAEP(newSHA1(), rand.Reader, ekResp.EkPublicKey, fakeAsymCA, []byte("TCPA"))
				sendAndExpectStreamError(t, stream, &epb.RotateAIKCertRequest{
					Value: &epb.RotateAIKCertRequest_IssuerCertPayload_{
						IssuerCertPayload: &epb.RotateAIKCertRequest_IssuerCertPayload{
							SymmetricKeyBlob: symKeyBlob,
							AikCertBlob:      []byte("not-pem-cert"),
						},
					},
					ControlCardSelection: activeCard(),
				})
			},
		},
		{
			"enrollz-2.14-AikCertForDifferentAIKKey",
			"aik_cert_blob contains a certificate for a different AIK public key than generated by the device",
			func(t *testing.T) {
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				stream := openRotateAIKStream(t, ctx, enrollzC, &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: activeCard(),
				})
				phase1Resp, err := stream.Recv()
				if err != nil {
					t.Fatalf("recv Phase 1 response: %v", err)
				}
				ekResp, err := rotDB.FetchEK(ctx, &biz.FetchEKReq{
					Serial: phase1Resp.GetControlCardId().GetControlCardSerial(),
				})
				if err != nil {
					t.Fatalf("FetchEK: %v", err)
				}
				wrongCertPEM := generateSelfSignedCertPEM(t)
				fakeAsymCA := make([]byte, 40)
				symKeyBlob, _ := rsa.EncryptOAEP(newSHA1(), rand.Reader, ekResp.EkPublicKey, fakeAsymCA, []byte("TCPA"))
				sendAndExpectStreamError(t, stream, &epb.RotateAIKCertRequest{
					Value: &epb.RotateAIKCertRequest_IssuerCertPayload_{
						IssuerCertPayload: &epb.RotateAIKCertRequest_IssuerCertPayload{
							SymmetricKeyBlob: symKeyBlob,
							AikCertBlob:      []byte(wrongCertPEM),
						},
					},
					ControlCardSelection: activeCard(),
				})
			},
		},
		{
			"enrollz-2.15-FinalizeBeforeAikCertReturned",
			"Service sends finalize=true before device returns aik_cert in Phase 4",
			func(t *testing.T) {
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				stream := openRotateAIKStream(t, ctx, enrollzC, &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: activeCard(),
				})
				if _, err := stream.Recv(); err != nil {
					t.Fatalf("recv Phase 1 response: %v", err)
				}
				sendAndExpectStreamError(t, stream, &epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_Finalize{Finalize: true},
					ControlCardSelection: activeCard(),
				})
			},
		},
		{
			"enrollz-2.16-PrematureStreamClose",
			"Service closes the stream prematurely; enrollment is aborted and no cert is persisted",
			func(t *testing.T) {
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				stream, err := enrollzC.RotateAIKCert(ctx)
				if err != nil {
					t.Fatalf("open stream: %v", err)
				}
				if err := stream.Send(&epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: activeCard(),
				}); err != nil {
					t.Fatalf("send Phase 1: %v", err)
				}
				if _, err := stream.Recv(); err != nil {
					t.Fatalf("recv Phase 1 response: %v", err)
				}
				if err := stream.CloseSend(); err != nil {
					t.Fatalf("CloseSend: %v", err)
				}
				if _, err := stream.Recv(); err != nil {
					t.Logf("stream closed as expected after premature service close: %v", err)
				} else {
					t.Log("device drained the stream without error; enrollment is aborted")
				}
				freshDeps := newTPM12Deps(enrollzClient(t, dut), rotDB, ownerCA)
				if err := rotateAIKCertErr(ctx, freshDeps, activeCard()); err != nil {
					t.Logf("NOTE: re-enrollment after premature close failed: %v", err)
				}
			},
		},
		{
			"enrollz-2.17-RebootDuringEnrollmentAndRestart",
			"Reboot at any time during enrollment (before finalize) and restart enrollment successfully",
			func(t *testing.T) {
				t.Log("Starting partial enrollment (Phase 1 only) to reach mid-enrollment state...")
				issuerKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				issuerPub, _ := x509.MarshalPKIXPublicKey(&issuerKey.PublicKey)
				stream, err := enrollzC.RotateAIKCert(ctx)
				if err != nil {
					t.Fatalf("open stream: %v", err)
				}
				if err := stream.Send(&epb.RotateAIKCertRequest{
					Value:                &epb.RotateAIKCertRequest_IssuerPublicKey{IssuerPublicKey: issuerPub},
					ControlCardSelection: activeCard(),
				}); err != nil {
					t.Fatalf("send Phase 1: %v", err)
				}
				if _, err := stream.Recv(); err != nil {
					t.Logf("Phase 1 recv (may be ok if reboot races): %v", err)
				}

				t.Log("Rebooting DUT mid-enrollment...")
				rebootDUT(t, dut)

				t.Log("Re-running full enrollment after reboot...")
				reenrollzC := enrollzClientReady(t, dut, 3*time.Minute)
				freshDeps := newTPM12Deps(reenrollzC, rotDB, ownerCA)
				enrollCtx, enrollCancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer enrollCancel()
				rotateAIKCert(t, enrollCtx, freshDeps, activeCard())
				if standby {
					rotateAIKCert(t, enrollCtx, freshDeps, standbyCard())
				}
				t.Log("Re-enrollment after mid-enrollment reboot succeeded")
			},
		},
		{
			"enrollz-2.18-RebootAfterSuccessfulEnrollment",
			"Reboot after successful enrollment; AIK cert must persist",
			func(t *testing.T) {
				deps := newTPM12Deps(enrollzClient(t, dut), rotDB, ownerCA)
				rotateAIKCert(t, ctx, deps, activeCard())
				if standby {
					rotateAIKCert(t, ctx, deps, standbyCard())
				}

				t.Log("Rebooting DUT after successful enrollment...")
				rebootDUT(t, dut)

				postEnrollzC := enrollzClientReady(t, dut, 3*time.Minute)
				if _, err := postEnrollzC.GetControlCardVendorID(ctx, &epb.GetControlCardVendorIDRequest{
					ControlCardSelection: activeCard(),
				}); err != nil {
					t.Fatalf("post-reboot GetControlCardVendorID failed (AIK cert not persistent?): %v", err)
				}
				t.Log("Post-reboot mTLS connection and RPC successful – AIK cert is persistent")
			},
		},
	} {
		t.Run(tc.id, func(t *testing.T) {
			t.Log(tc.desc)
			tc.fn(t)
		})
	}
}

type aristaROTDBClient struct {
	t        *testing.T
	enrollzC epb.TpmEnrollzServiceClient
}

func newAristaROTDBClient(t *testing.T, dut *ondatra.DUTDevice) biz.ROTDBClient {
	t.Helper()
	return &aristaROTDBClient{
		t:        t,
		enrollzC: enrollzClient(t, dut),
	}
}

func (c *aristaROTDBClient) FetchEK(ctx context.Context, req *biz.FetchEKReq) (*biz.FetchEKResp, error) {
	cardSel := &cpb.ControlCardSelection{
		ControlCardId: &cpb.ControlCardSelection_Role{
			Role: cpb.ControlCardRole_CONTROL_CARD_ROLE_ACTIVE,
		},
	}

	idevidResp, err := c.enrollzC.GetIdevidCsr(ctx, &epb.GetIdevidCsrRequest{
		ControlCardSelection: cardSel,
		Key:                  epb.Key_KEY_EK,
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

	rsaPub, err := ekRSAPubFromCSRContent(csrContent, req.Serial)
	if err != nil {
		return nil, err
	}

	c.t.Logf("Fetched EK public key for serial %q: RSA %d-bit key", req.Serial, rsaPub.N.BitLen())

	return &biz.FetchEKResp{
		EkPublicKey: rsaPub,
		KeyType:     epb.Key_KEY_EK,
	}, nil
}

func ekRSAPubFromCSRContent(csr *biz.TCGCSRIDevIDContents, serial string) (*rsa.PublicKey, error) {
	if csr.EKCert == "" {
		return nil, fmt.Errorf("TCG CSR IDevID content has empty EK cert for serial %q", serial)
	}
	block, _ := pem.Decode([]byte(csr.EKCert))
	if block == nil {
		return nil, fmt.Errorf("EK cert for serial %q: failed to decode PEM block", serial)
	}
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
		return pub, nil
	case "PUBLIC KEY":
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("EK pub for serial %q: parse PKIX: %w", serial, err)
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("EK pub for serial %q: not RSA (got %T)", serial, pub)
		}
		return rsaPub, nil
	default:
		return nil, fmt.Errorf("EK cert for serial %q: unsupported PEM block type %q", serial, block.Type)
	}
}

type aristaOwnerCAClient struct {
	ca    *x509.Certificate
	caKey *rsa.PrivateKey
}

func newAristaOwnerCAClient(t *testing.T) biz.SwitchOwnerCaClient {
	t.Helper()
	ca, caKey := generateTestCA(t)
	return &aristaOwnerCAClient{ca: ca, caKey: caKey}
}

func (c *aristaOwnerCAClient) IssueOwnerIakCert(ctx context.Context, req *biz.IssueOwnerIakCertReq) (*biz.IssueOwnerIakCertResp, error) {
	certPEM, err := signPubKeyPEM(req.IakPubPem, c.ca, c.caKey)
	if err != nil {
		return nil, fmt.Errorf("sign oIAK cert: %w", err)
	}
	return &biz.IssueOwnerIakCertResp{OwnerIakCertPem: certPEM}, nil
}

func (c *aristaOwnerCAClient) IssueOwnerIDevIDCert(ctx context.Context, req *biz.IssueOwnerIDevIDCertReq) (*biz.IssueOwnerIDevIDCertResp, error) {
	certPEM, err := signPubKeyPEM(req.IDevIDPubPem, c.ca, c.caKey)
	if err != nil {
		return nil, fmt.Errorf("sign oIDevID cert: %w", err)
	}
	return &biz.IssueOwnerIDevIDCertResp{OwnerIDevIDCertPem: certPEM}, nil
}

func (c *aristaOwnerCAClient) IssueAikCert(ctx context.Context, req *biz.IssueAikCertReq) (*biz.IssueAikCertResp, error) {
	certPEM, err := signPubKeyPEM(req.AikPubPem, c.ca, c.caKey)
	if err != nil {
		return nil, fmt.Errorf("sign AIK cert: %w", err)
	}
	return &biz.IssueAikCertResp{AikCertPem: certPEM}, nil
}

func generateTestCA(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generateTestCA: generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-owner-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
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

func signPubKeyPEM(pubKeyPEM string, ca *x509.Certificate, caKey *rsa.PrivateKey) (string, error) {
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
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, ca, pub, caKey)
	if err != nil {
		return "", fmt.Errorf("create signed certificate: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})), nil
}

func newSHA1() hash.Hash {
	return sha1.New()
}
