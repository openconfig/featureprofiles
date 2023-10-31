package certz_test

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"io"
	"io/ioutil"
	"net"
	"os"
	"testing"

	log "github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	cert "github.com/openconfig/featureprofiles/internal/cisco/security/cert"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/fptest"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestSimpleCertzGetProfile(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI(t)
	profiles, err := gnsiC.Certz().GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
	if err != nil {
		t.Fatalf("Unexpected Error in getting profile list: %v", err)
	}
	t.Logf("Profile list get was successful, %v", profiles)
}

func TestAddProfile(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI(t)
	SSLProfileList := [2]string{"Test123", "Abc123"}
	type args struct {
		req *certzpb.AddProfileRequest
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "Add SSL profile test",
			args:    args{req: &certzpb.AddProfileRequest{SslProfileId: SSLProfileList[0]}},
			wantErr: false,
		},
		{
			name:    "Add SSL profile Abc",
			args:    args{req: &certzpb.AddProfileRequest{SslProfileId: SSLProfileList[1]}},
			wantErr: false,
		},
		{
			name:    "Add SSL profile with invalid directory name",
			args:    args{req: &certzpb.AddProfileRequest{SslProfileId: "/abctest/\\0"}},
			wantErr: true,
		},
		{
			name:    "Add SSL profile with empty string",
			args:    args{req: &certzpb.AddProfileRequest{SslProfileId: ""}},
			wantErr: true,
		},
		{
			name:    "Add SSL profile with string > 256 characters",
			args:    args{req: &certzpb.AddProfileRequest{SslProfileId: "hsqnjmiwydgaprygrsmqjposbhvbsbgfstyzrgqjclcscbnqkzyvmswcvmyhlnjthschymgjcldncijiysfdlcpaidmboxzcetdbnrjcgqqseanukyyfetjndfotwzjgrcqafjedhofmqwguuiprkuhvvganhtfhbkwrkyaqwxllbzyineikbdflaalgrllezqcmpneslyyxfhfpwmqxykfuggrtrmlajcecdshqutjmonsxlgcfvjcsqqqufwhbjg"}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gnsiC.Certz().AddProfile(context.Background(), tt.args.req)
			if (tt.wantErr == true) && (err == nil) {
				t.Errorf("Expected Error But able to create profile with invalid values")
			} else if (err != nil) && (tt.wantErr == false) {
				t.Errorf("Server.AddProfile() error = %v, wantErr %v, %s", err, tt.wantErr, tt.args.req)
			}

		})
	}
}

func TestGetProfileList(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI(t)
	type args struct {
		req *certzpb.GetProfileListRequest
	}
	tests := []struct {
		name    string
		args    args
		want    *certzpb.GetProfileListResponse
		wantErr bool
	}{
		{
			name:    "Get SSL profile Request",
			args:    args{req: &certzpb.GetProfileListRequest{}},
			want:    &certzpb.GetProfileListResponse{SslProfileIds: []string{"Abc123", "Test123", "gNxI"}},
			wantErr: false,
		},
		{
			name:    "NIL Get SSL profile request",
			args:    args{},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gnsiC.Certz().GetProfileList(context.Background(), tt.args.req)
			if got != nil {
				if diff := cmp.Diff(got.SslProfileIds, tt.want.SslProfileIds); diff != "" {
					t.Errorf("Server.GetProfileList() error: did not got the expected list (-want +got):\n%v", diff)
				}
			}
			if (tt.wantErr == true) && (err == nil) {
				t.Errorf("Expected Error But able to get profile with invalid values")
			} else if (err != nil) && (tt.wantErr == false) {
				t.Errorf("Server.GetProfileList() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteProfile(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI(t)
	SSLProfileList := [2]string{"Test123", "Abc123"}
	type args struct {
		req *certzpb.DeleteProfileRequest
	}
	tests := []struct {
		name    string
		args    args
		want    *certzpb.DeleteProfileResponse
		wantErr bool
	}{
		{
			name:    "Delete SSL profile test123",
			args:    args{req: &certzpb.DeleteProfileRequest{SslProfileId: SSLProfileList[0]}},
			wantErr: false,
		},
		{
			name:    "Delete SSL profile Abc123",
			args:    args{req: &certzpb.DeleteProfileRequest{SslProfileId: SSLProfileList[1]}},
			wantErr: false,
		},
		{
			name:    "Delete profile with invalid directory name",
			args:    args{req: &certzpb.DeleteProfileRequest{SslProfileId: "/abctest/\\0"}},
			wantErr: true,
		},
		{
			name:    "Delete SSL profile with empty string",
			args:    args{req: &certzpb.DeleteProfileRequest{SslProfileId: ""}},
			wantErr: true,
		},
		{
			name:    "Send NULL request",
			args:    args{},
			wantErr: true,
		},
		{
			name:    "Delete SSL profile with string > 256 characters",
			args:    args{req: &certzpb.DeleteProfileRequest{SslProfileId: "hsqnjmiwydgaprygrsmqjposbhvbsbgfstyzrgqjclcscbnqkzyvmswcvmyhlnjthschymgjcldncijiysfdlcpaidmboxzcetdbnrjcgqqseanukyyfetjndfotwzjgrcqafjedhofmqwguuiprkuhvvganhtfhbkwrkyaqwxllbzyineikbdflaalgrllezqcmpneslyyxfhfpwmqxykfuggrtrmlajcecdshqutjmonsxlgcfvjcsqqqufwhbjg"}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gnsiC.Certz().DeleteProfile(context.Background(), tt.args.req)
			if (tt.wantErr == true) && (err == nil) {
				t.Errorf("Expected Error But able to delete profile with invalid values")
			} else if (err != nil) && (tt.wantErr == false) {
				t.Errorf("Server.DeleteProfile() error = %v, wantErr %v, %s", err, tt.wantErr, tt.args.req)
			}
		})
	}
}

func populateCSRParams(csrSuite certzpb.CSRSuite) *certzpb.CSRParams {
	return &certzpb.CSRParams{
		CsrSuite:           csrSuite,
		CommonName:         "testcertz.com",
		Country:            "US",
		State:              "California",
		City:               "San Francisco",
		Organization:       "Example Inc",
		OrganizationalUnit: "IT",
		San: &certzpb.V3ExtensionSAN{
			Dns:    []string{"testcertz.com", "www.testcertz.com"},
			Emails: []string{"admin@testcertz.com"},
			Ips:    []string{"127.0.0.1"},
			Uris:   []string{"https://testcertz.com"},
		},
	}
}
func TestCanGenerateCSR(t *testing.T) {
	rsa2048testvalidparamssha256 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_256)
	rsa2048testvalidparamssha384 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_384)
	rsa2048testvalidparamssha512 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_512)
	rsa3072testvalidparamssha256 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_3072_SIGNATURE_ALGORITHM_SHA_2_256)
	rsa3072testvalidparamssha384 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_3072_SIGNATURE_ALGORITHM_SHA_2_384)
	rsa3072testvalidparamssha512 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_3072_SIGNATURE_ALGORITHM_SHA_2_512)
	rsa4096testvalidparamssha256 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_4096_SIGNATURE_ALGORITHM_SHA_2_256)
	rsa4096testvalidparamssha384 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_4096_SIGNATURE_ALGORITHM_SHA_2_384)
	rsa4096testvalidparamssha512 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_4096_SIGNATURE_ALGORITHM_SHA_2_512)
	ecdsa256testvalidparamssha256 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_ECDSA_PRIME256V1_SIGNATURE_ALGORITHM_SHA_2_256)
	ecdsa384testvalidparamssha256 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_ECDSA_SECP384R1_SIGNATURE_ALGORITHM_SHA_2_256)
	ecdsa256testvalidparamssha384 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_ECDSA_SECP384R1_SIGNATURE_ALGORITHM_SHA_2_384)
	ecdsa256testvalidparamssha512 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_ECDSA_SECP384R1_SIGNATURE_ALGORITHM_SHA_2_512)
	ecdsa384testvalidparamssha384 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_ECDSA_SECP384R1_SIGNATURE_ALGORITHM_SHA_2_384)
	ecdsa384testvalidparamssha512 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_ECDSA_SECP384R1_SIGNATURE_ALGORITHM_SHA_2_512)
	ecdsa521testvalidparamssha256 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_ECDSA_SECP521R1_SIGNATURE_ALGORITHM_SHA_2_256)
	ecdsa521testvalidparamssha384 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_ECDSA_SECP521R1_SIGNATURE_ALGORITHM_SHA_2_384)
	ecdsa521testvalidparamssha512 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_ECDSA_SECP521R1_SIGNATURE_ALGORITHM_SHA_2_512)
	ed25519testvalidparamssha256 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_EDDSA_ED25519)
	invalidparams1 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_CIPHER_UNSPECIFIED)
	invalidparams2 := certzpb.CSRParams{
		CsrSuite: certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_256,
	}
	invalidparams3 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_256)
	invalidparams3.San.Dns = []string{"testcertz.com", "www.testcertz.com", "invalid_dns.-com"}

	invalidparams4 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_256)
	invalidparams4.San.Emails = []string{"invalid.email.com"}

	invalidparams5 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_256)
	invalidparams5.San.Ips = []string{"invalid_ip"}

	invalidparams6 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_256)
	invalidparams6.San.Uris = []string{"invalid_uri"}

	randomparams1 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_256)

	randomparams2 := populateCSRParams(certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_256)
	randomparams2.San.Ips = []string{}

	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI(t)
	type args struct {
		req *certzpb.CanGenerateCSRRequest
	}
	tests := []struct {
		name    string
		args    args
		want    *certzpb.CanGenerateCSRResponse
		wantErr bool
	}{
		{
			name:    "CSR with RSA 2048 SHA256 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: rsa2048testvalidparamssha256}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with RSA 2048 SHA384 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: rsa2048testvalidparamssha384}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with RSA 2048 SHA512 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: rsa2048testvalidparamssha512}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with RSA 3072 SHA256 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: rsa3072testvalidparamssha256}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with RSA 3072 SHA384 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: rsa3072testvalidparamssha384}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with RSA 3072 SHA512 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: rsa3072testvalidparamssha512}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with RSA 4096 SHA256 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: rsa4096testvalidparamssha256}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with RSA 4096 SHA384 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: rsa4096testvalidparamssha384}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with RSA 4096 SHA512 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: rsa4096testvalidparamssha512}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with ECDSA256 SHA256 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: ecdsa256testvalidparamssha256}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with ECDSA256 SHA384 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: ecdsa256testvalidparamssha384}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with ECDSA256 SHA512 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: ecdsa256testvalidparamssha512}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with ECDSA384 SHA256 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: ecdsa384testvalidparamssha256}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with ECDSA384 SHA384 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: ecdsa384testvalidparamssha384}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with ECDSA384 SHA512 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: ecdsa384testvalidparamssha512}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with ECDSA521 SHA256 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: ecdsa521testvalidparamssha256}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with ECDSA521 SHA384 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: ecdsa521testvalidparamssha384}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with ECDSA521 SHA512 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: ecdsa521testvalidparamssha512}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with ED25519 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: ed25519testvalidparamssha256}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with invalid unspecified suite",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: invalidparams1}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: false},
			wantErr: true,
		},
		{
			name:    "CSR with no common name",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: &invalidparams2}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: false},
			wantErr: true,
		},
		{
			name:    "CSR with invalid DNS",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: invalidparams3}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: false},
			wantErr: true,
		},
		{
			name:    "CSR with invalid email",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: invalidparams4}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: false},
			wantErr: true,
		},
		{
			name:    "CSR with invalid IP",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: invalidparams5}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: false},
			wantErr: true,
		},
		{
			name:    "CSR with invalid URI",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: invalidparams6}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: false},
			wantErr: true,
		},
		{
			name:    "Req is nil",
			args:    args{req: nil},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: false},
			wantErr: true,
		},
		{
			name:    "CSR with fewer parameters 1",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: randomparams1}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with fewer parameters 2",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: randomparams2}},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gnsiC.Certz().CanGenerateCSR(context.Background(), tt.args.req)

			if (tt.wantErr == true) && (err == nil) {
				t.Errorf("Expected Error But able to generate CSR profile with invalid values")
			} else if (err != nil) && (tt.wantErr == false) {
				t.Errorf("Server.CanGenerateCSR() error = %v, wantErr %v, %s", err, tt.wantErr, tt.args.req)
			}
			if got != nil {
				if got.CanGenerate != tt.want.CanGenerate {
					t.Errorf("Error CanGenerateCSR() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func readCertificatesFromFile(filename string) ([]*x509.Certificate, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var certificates []*x509.Certificate
	block, rest := pem.Decode(data)
	for block != nil {
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			certificates = append(certificates, cert)
		}
		if len(rest) == 0 {
			break
		}
		block, rest = pem.Decode(rest)
	}

	return certificates, nil
}
func TestRotateReqWithFinalizeRsa(t *testing.T) {
	os.Mkdir("testdata/", 0755)
	defer os.RemoveAll("testdata/")

	// Adding New SSL Profile

	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI(t)
	profile_id := "rotatecertzrsa"
	profiles, err := gnsiC.Certz().AddProfile(context.Background(), &certzpb.AddProfileRequest{SslProfileId: profile_id})

	if err != nil {
		t.Fatalf("Unexpected Error in adding profile list: %v", err)
	}
	t.Logf("Profile add successful %v", profiles)

	//Generating Self-signed Cert

	_, _, err = cert.GenRootCA("ROOTCA", x509.RSA, 100, "testdata/")

	if err != nil {
		t.Fatalf("Generation of root ca using rsa is failed: %v", err)
	}
	caKey, caCert, err := cert.LoadKeyPair("testdata/cakey.rsa.pem", "testdata/cacert.rsa.pem")
	if err != nil {
		t.Fatalf("Could not load the generated key and cer: %v", err)
	}
	//Generating Server Cert & Signed from CA
	certTemp, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, []net.IP{net.IPv4(10, 105, 237, 37)}, "test", 100)
	if err != nil {
		t.Fatalf("Could not generate the cert template: %v", err)
	}
	tlscert, err := cert.GenerateCert(certTemp, caCert, caKey, x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}
	err = cert.SaveTLSCertInPems(tlscert, "testdata/server_key.pem", "testdata/server_cert.pem", x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificates: %v", err)
	}
	//Rotate with Server cert & key

	certPEMtoload, err := ioutil.ReadFile("testdata/server_cert.pem")
	if err != nil {
		log.Exit("Failed to read cert file", err)
	}
	privKeyPEMtoload, err := ioutil.ReadFile("testdata/server_key.pem")
	if err != nil {
		log.Exit("Failed to read key file", err)
	}

	stream, err := gnsiC.Certz().Rotate(context.Background())
	if err != nil {
		log.Exit("failed to get stream:", err)
	}
	var response *certzpb.RotateCertificateResponse

	request := &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: []*certzpb.Entity{
					{
						Version:   "1.0",
						CreatedOn: 123456789,
						Entity: &certzpb.Entity_CertificateChain{
							CertificateChain: &certzpb.CertificateChain{
								Certificate: &certzpb.Certificate{
									Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
									Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
									Certificate: certPEMtoload,
									PrivateKey:  privKeyPEMtoload,
								},
							},
						},
					},
				},
			},
		},
	}
	log.V(1).Info("RotateCertificateRequest:\n", proto.MarshalTextString(request))
	if err = stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if response, err = stream.Recv(); err != nil {
		log.Exit("failed to receive RotateCertificateResponse:", err)
	}
	log.V(1).Info("RotateCertificateResponse:\n", proto.MarshalTextString(response))
	t.Logf("Rotate successful %v", request)

	//ROTATE with TRUSTBUNDLE
	certificates, err := readCertificatesFromFile("testdata/cacert.rsa.pem")
	if err != nil {
		log.Exit("failed to read bundle", err)
	}
	var certChainMessage certzpb.CertificateChain
	var x509toPEM = func(cert *x509.Certificate) []byte {
		return pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
	}
	for i, cert := range certificates {
		certMessage := &certzpb.Certificate{
			Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
			Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
			Certificate: x509toPEM(cert),
		}
		if i > 0 {
			certChainMessage.Parent = &certzpb.CertificateChain{
				Certificate: certMessage,
				Parent:      certChainMessage.Parent,
			}
		} else {
			certChainMessage = certzpb.CertificateChain{
				Certificate: certMessage,
			}
		}
	}
	request = &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: []*certzpb.Entity{
					{
						Version:   "1.0",
						CreatedOn: 123456789,
						Entity: &certzpb.Entity_TrustBundle{
							TrustBundle: &certChainMessage,
						},
					},
				},
			},
		},
	}
	log.V(1).Info("RotateTBRequest:\n", proto.MarshalTextString(request))
	if err = stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if response, err = stream.Recv(); err != nil {
		log.Exit("failed to receive RotateCertificateTBResponse:", err)
	}
	log.V(1).Info("RotateTBCertificateResponse:\n", proto.MarshalTextString(response))

	//FINALIZE ROTATE REQUEST
	request = &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest:  &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}},
	}
	log.V(1).Info("RotateFinalizeReq:\n", proto.MarshalTextString(request))
	if err := stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if _, err = stream.Recv(); err != nil {
		if err != io.EOF {
			log.Exit("Failed, finalize Rotation is cancelled", err)
		}
	}
	log.V(1).Info("RotateCertificateFinalize: Success, stream has ended")

	//CONFIG NEW gNSI CLI
	t.Log("Config new gNSI CLI")
	configToChange := "grpc gnsi service certz ssl-profile-id rotatecertzrsa \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	//Get-profile with rotated CERTS
	profilelist, err := gnsiC.Certz().GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
	if err != nil {
		t.Fatalf("Unexpected Error in getting profile list: %v", err)
	}
	t.Logf("Profile list get was successful, %v", profilelist)

	//UnCONFIG NEW gNSI CLI
	t.Log("UnConfig new gNSI CLI")
	configToremove := "no grpc gnsi service certz ssl-profile-id rotatecertzrsa \n"
	ctx = context.Background()
	util.GNMIWithText(ctx, t, dut, configToremove)

	//Delete rotated profile-id
	delprofile, err := gnsiC.Certz().DeleteProfile(context.Background(), &certzpb.DeleteProfileRequest{SslProfileId: profile_id})
	if err != nil {
		t.Fatalf("Unexpected Error in deleting profile list: %v", err)
	}
	t.Logf("Delete Profile was successful, %v", delprofile)

}

func TestRotateReqWithFinalizeEcdsa(t *testing.T) {
	os.Mkdir("testdata/", 0755)
	defer os.RemoveAll("testdata/")

	// Adding New SSL Profile
	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI(t)
	profile_id := "rotatecertzecdsa"
	profiles, err := gnsiC.Certz().AddProfile(context.Background(), &certzpb.AddProfileRequest{SslProfileId: profile_id})

	if err != nil {
		t.Fatalf("Unexpected Error in adding profile list: %v", err)
	}
	t.Logf("Profile add successful %v", profiles)

	//Generating Self-signed Cert
	_, _, err = cert.GenRootCA("ROOTCA", x509.ECDSA, 100, "testdata/")

	if err != nil {
		t.Fatalf("Generation of root ca using ecdsa is failed: %v", err)
	}
	caKey, caCert, err := cert.LoadKeyPair("testdata/cakey.ecdsa.pem", "testdata/cacert.ecdsa.pem")
	if err != nil {
		t.Fatalf("Could not load the generated key and cer: %v", err)
	}
	//Generating Server Cert & Signed from CA
	certTemp, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, []net.IP{net.IPv4(10, 105, 237, 37)}, "test", 100)
	if err != nil {
		t.Fatalf("Could not generate the cert template: %v", err)
	}
	tlscert, err := cert.GenerateCert(certTemp, caCert, caKey, x509.ECDSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}
	err = cert.SaveTLSCertInPems(tlscert, "testdata/server_key.pem", "testdata/server_cert.pem", x509.ECDSA)
	if err != nil {
		t.Fatalf("Could not generate certificates: %v", err)
	}
	//Rotate with Server cert & Server key

	certPEMtoload, err := ioutil.ReadFile("testdata/server_cert.pem")
	if err != nil {
		log.Exit("Failed to read cert file", err)
	}
	privKeyPEMtoload, err := ioutil.ReadFile("testdata/server_key.pem")
	if err != nil {
		log.Exit("Failed to read key file", err)
	}

	stream, err := gnsiC.Certz().Rotate(context.Background())
	if err != nil {
		log.Exit("failed to get stream:", err)
	}
	var response *certzpb.RotateCertificateResponse

	request := &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: []*certzpb.Entity{
					{
						Version:   "1.0",
						CreatedOn: 123456789,
						Entity: &certzpb.Entity_CertificateChain{
							CertificateChain: &certzpb.CertificateChain{
								Certificate: &certzpb.Certificate{
									Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
									Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
									Certificate: certPEMtoload,
									PrivateKey:  privKeyPEMtoload,
								},
							},
						},
					},
				},
			},
		},
	}
	log.V(1).Info("RotateCertificateRequest:\n", proto.MarshalTextString(request))
	if err = stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if response, err = stream.Recv(); err != nil {
		log.Exit("failed to receive RotateCertificateResponse:", err)
	}
	log.V(1).Info("RotateCertificateResponse:\n", proto.MarshalTextString(response))
	t.Logf("Rotate successful %v", request)

	//ROTATE with TRUSTBUNDLE
	certificates, err := readCertificatesFromFile("testdata/cacert.ecdsa.pem")
	if err != nil {
		log.Exit("failed to read bundle", err)
	}
	var certChainMessage certzpb.CertificateChain
	var x509toPEM = func(cert *x509.Certificate) []byte {
		return pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
	}
	for i, cert := range certificates {
		certMessage := &certzpb.Certificate{
			Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
			Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
			Certificate: x509toPEM(cert),
		}
		if i > 0 {
			certChainMessage.Parent = &certzpb.CertificateChain{
				Certificate: certMessage,
				Parent:      certChainMessage.Parent,
			}
		} else {
			certChainMessage = certzpb.CertificateChain{
				Certificate: certMessage,
			}
		}
	}
	request = &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: []*certzpb.Entity{
					{
						Version:   "1.0",
						CreatedOn: 123456789,
						Entity: &certzpb.Entity_TrustBundle{
							TrustBundle: &certChainMessage,
						},
					},
				},
			},
		},
	}
	log.V(1).Info("RotateTBRequest:\n", proto.MarshalTextString(request))
	if err = stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if response, err = stream.Recv(); err != nil {
		log.Exit("failed to receive RotateCertificateTBResponse:", err)
	}
	log.V(1).Info("RotateTBCertificateResponse:\n", proto.MarshalTextString(response))

	//FINALIZE ROTATE REQUEST
	request = &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest:  &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}},
	}
	log.V(1).Info("RotateFinalizeReq:\n", proto.MarshalTextString(request))
	if err := stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if _, err = stream.Recv(); err != nil {
		if err != io.EOF {
			log.Exit("Failed, finalize Rotation is cancelled", err)
		}
	}
	log.V(1).Info("RotateCertificateFinalize: Success, stream has ended")

	//CONFIG NEW gNSI CLI
	t.Log("Config new gNSI CLI")
	configToChange := "grpc gnsi service certz ssl-profile-id rotatecertzecdsa \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	//Get-profile with rotated CERTS
	profilelist, err := gnsiC.Certz().GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
	if err != nil {
		t.Fatalf("Unexpected Error in getting profile list: %v", err)
	}
	t.Logf("Profile list get was successful, %v", profilelist)

	//UnCONFIG NEW gNSI CLI
	t.Log("UnConfig new gNSI CLI")
	configToremove := "no grpc gnsi service certz ssl-profile-id rotatecertz \n"
	ctx = context.Background()
	util.GNMIWithText(ctx, t, dut, configToremove)

	//Delete rotated profile-id
	delprofile, err := gnsiC.Certz().DeleteProfile(context.Background(), &certzpb.DeleteProfileRequest{SslProfileId: profile_id})
	if err != nil {
		t.Fatalf("Unexpected Error in deleting profile list: %v", err)
	}
	t.Logf("Delete Profile was successful, %v", delprofile)

}

func TestRotateReqWithFinalizeNegative(t *testing.T) {

	os.Mkdir("testdata/", 0755)
	defer os.RemoveAll("testdata/")

	// Adding New SSL Profile
	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI(t)
	profile_id := "rotatecertznegative"
	profiles, err := gnsiC.Certz().AddProfile(context.Background(), &certzpb.AddProfileRequest{SslProfileId: profile_id})
	if err != nil {
		t.Fatalf("Unexpected Error in getting profile list: %v", err)
	}
	t.Logf("Profile add successful, %v", profiles)

	//Generating Self-signed Cert
	_, _, err = cert.GenRootCA("ROOTCA", x509.RSA, 100, "testdata/")

	if err != nil {
		t.Fatalf("Generation of root ca using rsa is failed: %v", err)
	}
	caKey, caCert, err := cert.LoadKeyPair("testdata/cakey.rsa.pem", "testdata/cacert.rsa.pem")
	if err != nil {
		t.Fatalf("Could not load the generated key and cer: %v", err)
	}
	//Generating Server Cert & Signed from CA
	certTemp, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, []net.IP{net.IPv4(10, 105, 237, 37)}, "test", 100)
	if err != nil {
		t.Fatalf("Could not generate the cert template: %v", err)
	}
	tlscert, err := cert.GenerateCert(certTemp, caCert, caKey, x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}
	err = cert.SaveTLSCertInPems(tlscert, "testdata/server_key_rsa.pem", "testdata/server_cert_rsa.pem", x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificates: %v", err)
	}
	//Rotate with Server cert & key

	certPEMtoload, err := ioutil.ReadFile("testdata/server_cert_rsa.pem")
	if err != nil {
		log.Exit("Failed to read cert file", err)
	}
	privKeyPEMtoload, err := ioutil.ReadFile("testdata/server_key_rsa.pem")
	if err != nil {
		log.Exit("Failed to read key file", err)
	}
	var expired_cert = `-----BEGIN CERTIFICATE-----
	MIIBjTCCAROgAwIBAgIBATAKBggqhkjOPQQDAzAWMRQwEgYDVQQDEwtleGFtcGxl
	LmNvbTAeFw0yMzA5MDYwNTMxNTdaFw0yMzA5MDYwNTMyMDdaMBYxFDASBgNVBAMT
	C2V4YW1wbGUuY29tMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEcebqv4ACjUQNaJVE
	sokZTeNgRRAJzGfZoW20RAYaekIPZZSAiT7iU/JuL+GIlJvtlFVQl5Jcb5XOV4nO
	G2lk/mTKQI+tVS/jaBNeo+7ksIA5ly+82l+Id5M7Y6TNNxxGozUwMzAOBgNVHQ8B
	Af8EBAMCBaAwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDAYDVR0TAQH/BAIwADAKBggq
	hkjOPQQDAwNoADBlAjEAxmC2wcSyf9vGcrlIJ2k+9D19d9KCvB8N8TvrGePbz6kp
	fiPmdzFVJSYwTABYR2zxAjAMLKNv1F898l1eZXBUSBETIUUGrIQGXMq5p3QD6zQF
	xw+P9fXxxL5HKoCrTksitAo=
	-----END CERTIFICATE-----`

	type testRequest struct {
		req *certzpb.RotateCertificateRequest
	}
	tests := []struct {
		desc     string
		testCase testRequest
		wantErr  bool
	}{

		{
			desc: "Rotate with empty entity",
			testCase: testRequest{
				req: &certzpb.RotateCertificateRequest{
					ForceOverwrite: true,
					SslProfileId:   profile_id,
					RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
						Certificates: &certzpb.UploadRequest{
							Entities: []*certzpb.Entity{},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "Rotate Leaf certificate with garbage privatekey",
			testCase: testRequest{
				req: &certzpb.RotateCertificateRequest{
					ForceOverwrite: true,
					SslProfileId:   profile_id,
					RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
						Certificates: &certzpb.UploadRequest{
							Entities: []*certzpb.Entity{
								{
									Version:   "1.0",
									CreatedOn: 123456789,
									Entity: &certzpb.Entity_CertificateChain{
										CertificateChain: &certzpb.CertificateChain{
											Certificate: &certzpb.Certificate{
												Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
												Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
												Certificate: certPEMtoload,
												PrivateKey:  certPEMtoload,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "Rotate expired Leaf certificate",
			testCase: testRequest{
				req: &certzpb.RotateCertificateRequest{
					ForceOverwrite: true,
					SslProfileId:   profile_id,
					RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
						Certificates: &certzpb.UploadRequest{
							Entities: []*certzpb.Entity{
								{
									Version:   "1.0",
									CreatedOn: 123456789,
									Entity: &certzpb.Entity_CertificateChain{
										CertificateChain: &certzpb.CertificateChain{
											Certificate: &certzpb.Certificate{
												Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
												Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
												Certificate: []byte(expired_cert),
												PrivateKey:  privKeyPEMtoload,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "Rotate Leaf certificate for non-existent profile id",
			testCase: testRequest{
				req: &certzpb.RotateCertificateRequest{
					ForceOverwrite: true,
					SslProfileId:   "non-existent",
					RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
						Certificates: &certzpb.UploadRequest{
							Entities: []*certzpb.Entity{
								{
									Version:   "1.0",
									CreatedOn: 123456789,
									Entity: &certzpb.Entity_CertificateChain{
										CertificateChain: &certzpb.CertificateChain{
											Certificate: &certzpb.Certificate{
												Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
												Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
												Certificate: certPEMtoload,
												PrivateKey:  privKeyPEMtoload,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {

			r, err := gnsiC.Certz().Rotate(context.Background())
			if err != nil {
				log.Exit("failed to get stream:", err)
			}
			//var response *certzpb.RotateCertificateResponse

			t.Logf("request: %+v", tt.testCase.req)
			err = r.Send(tt.testCase.req)
			if err != nil {
				t.Fatalf("Send failed.%v", err)
			}
			_, err = r.Recv()
			if (tt.wantErr == true) && (err == nil) {
				t.Errorf("Expected Error But able to rotate profile with invalid values")
			}
		})
	}
	//Delete rotated profile-id
	delprofile, err := gnsiC.Certz().DeleteProfile(context.Background(), &certzpb.DeleteProfileRequest{SslProfileId: profile_id})
	if err != nil {
		t.Fatalf("Unexpected Error in deleting profile list: %v", err)
	}
	t.Logf("Delete Profile was successful, %v", delprofile)
}

func TestRotateReqWithFinalizeRsaValidate(t *testing.T) {
	os.Mkdir("testdatarsa/", 0755)
	defer os.RemoveAll("testdatarsa/")

	os.Mkdir("testdataecdsa/", 0755)
	defer os.RemoveAll("testdataecdsa/")

	// Adding New SSL Profile

	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI(t)
	profile_id := "rotatecertzvalidate"
	profiles, err := gnsiC.Certz().AddProfile(context.Background(), &certzpb.AddProfileRequest{SslProfileId: profile_id})

	if err != nil {
		t.Fatalf("Unexpected Error in adding profile list: %v", err)
	}
	t.Logf("Profile add successful %v", profiles)

	//Generating Self-signed RSA Cert

	_, _, err = cert.GenRootCA("ROOTCA", x509.RSA, 100, "testdatarsa/")

	if err != nil {
		t.Fatalf("Generation of root ca using rsa is failed: %v", err)
	}
	caKeyrsa, caCertrsa, err := cert.LoadKeyPair("testdatarsa/cakey.rsa.pem", "testdatarsa/cacert.rsa.pem")
	if err != nil {
		t.Fatalf("Could not load the generated key and cer: %v", err)
	}
	//Generating Server Cert & Signed from RSA CA
	certTemprsa, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, []net.IP{net.IPv4(10, 105, 237, 37)}, "test", 100)
	if err != nil {
		t.Fatalf("Could not generate the cert template: %v", err)
	}
	tlscertrsa, err := cert.GenerateCert(certTemprsa, caCertrsa, caKeyrsa, x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}
	err = cert.SaveTLSCertInPems(tlscertrsa, "testdatarsa/server_key_rsa.pem", "testdatarsa/server_cert_rsa.pem", x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificates: %v", err)
	}
	//Rotate with RSA Server cert & key

	certPEMtoloadRSA, err := ioutil.ReadFile("testdatarsa/server_cert_rsa.pem")
	if err != nil {
		log.Exit("Failed to read cert file", err)
	}
	privKeyPEMtoloadRSA, err := ioutil.ReadFile("testdatarsa/server_key_rsa.pem")
	if err != nil {
		log.Exit("Failed to read key file", err)
	}

	//Generating Self-signed ECDSA Cert
	_, _, err = cert.GenRootCA("ROOTCA", x509.ECDSA, 100, "testdataecdsa/")

	if err != nil {
		t.Fatalf("Generation of root ca using ecdsa is failed: %v", err)
	}
	caKeyecdsa, caCertecdsa, err := cert.LoadKeyPair("testdataecdsa/cakey.ecdsa.pem", "testdataecdsa/cacert.ecdsa.pem")
	if err != nil {
		t.Fatalf("Could not load the generated key and cer: %v", err)
	}
	//Generating Server Cert & Signed from CA
	certTempecdsa, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, []net.IP{net.IPv4(10, 105, 237, 37)}, "test", 100)
	if err != nil {
		t.Fatalf("Could not generate the cert template: %v", err)
	}
	tlscertecdsa, err := cert.GenerateCert(certTempecdsa, caCertecdsa, caKeyecdsa, x509.ECDSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}
	err = cert.SaveTLSCertInPems(tlscertecdsa, "testdataecdsa/server_key.pem", "testdataecdsa/server_cert.pem", x509.ECDSA)
	if err != nil {
		t.Fatalf("Could not generate certificates: %v", err)
	}
	stream, err := gnsiC.Certz().Rotate(context.Background())
	if err != nil {
		log.Exit("failed to get stream:", err)
	}
	var response *certzpb.RotateCertificateResponse

	//ROTATE with RSA TRUSTBUNDLE
	certificatesrsa, err := readCertificatesFromFile("testdatarsa/cacert.rsa.pem")
	if err != nil {
		log.Exit("failed to read bundle", err)
	}
	var certChainMessageRSA certzpb.CertificateChain
	var x509toPEM = func(cert *x509.Certificate) []byte {
		return pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
	}
	for i, cert := range certificatesrsa {
		certMessage := &certzpb.Certificate{
			Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
			Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
			Certificate: x509toPEM(cert),
		}
		if i > 0 {
			certChainMessageRSA.Parent = &certzpb.CertificateChain{
				Certificate: certMessage,
				Parent:      certChainMessageRSA.Parent,
			}
		} else {
			certChainMessageRSA = certzpb.CertificateChain{
				Certificate: certMessage,
			}
		}
	}

	//ROTATE with ECDSA TRUSTBUNDLE
	certificatesecdsa, err := readCertificatesFromFile("testdataecdsa/cacert.ecdsa.pem")
	if err != nil {
		log.Exit("failed to read bundle", err)
	}
	var certChainMessageECDSA certzpb.CertificateChain
	var x509toECDSAPEM = func(cert *x509.Certificate) []byte {
		return pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		})
	}
	for i, cert := range certificatesecdsa {
		certMessage := &certzpb.Certificate{
			Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
			Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
			Certificate: x509toECDSAPEM(cert),
		}
		if i > 0 {
			certChainMessageECDSA.Parent = &certzpb.CertificateChain{
				Certificate: certMessage,
				Parent:      certChainMessageECDSA.Parent,
			}
		} else {
			certChainMessageECDSA = certzpb.CertificateChain{
				Certificate: certMessage,
			}
		}
	}

	request := &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: []*certzpb.Entity{
					{
						Version:   "1.0",
						CreatedOn: 123456789,
						Entity: &certzpb.Entity_CertificateChain{
							CertificateChain: &certzpb.CertificateChain{
								Certificate: &certzpb.Certificate{
									Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
									Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
									Certificate: certPEMtoloadRSA,
									PrivateKey:  privKeyPEMtoloadRSA,
								},
							},
						},
					},
					{
						Version:   "1.0",
						CreatedOn: 123456789,
						Entity: &certzpb.Entity_TrustBundle{
							TrustBundle: &certChainMessageRSA,
						},
					},
				},
			},
		},
	}

	log.V(1).Info("RotateTBRequest:\n", proto.MarshalTextString(request))
	if err = stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if response, err = stream.Recv(); err != nil {
		log.Exit("failed to receive RotateCertificateTBResponse:", err)
	}
	log.V(1).Info("RotateTBCertificateResponse:\n", proto.MarshalTextString(response))

	//FINALIZE ROTATE REQUEST
	request = &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest:  &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}},
	}
	log.V(1).Info("RotateFinalizeReq:\n", proto.MarshalTextString(request))
	if err := stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if _, err = stream.Recv(); err != nil {
		if err != io.EOF {
			log.Exit("Failed, finalize Rotation is cancelled", err)
		}
	}
	log.V(1).Info("RotateCertificateFinalize: Success, stream has ended")

	//CONFIG NEW gNSI CLI
	t.Log("Config new gNSI CLI")
	configToChange := "grpc gnsi service certz ssl-profile-id rotatecertzvalidate \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	//Get-profile with rotated CERTS
	profilelist, err := gnsiC.Certz().GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
	if err != nil {
		t.Fatalf("Unexpected Error in getting profile list: %v", err)
	}
	t.Logf("Profile list get was successful, %v", profilelist)

	//Create a new stream for this Rotate

	stream1, err := gnsiC.Certz().Rotate(context.Background())
	if err != nil {
		log.Exit("failed to get stream:", err)
	}
	var response1 *certzpb.RotateCertificateResponse

	//Rotate with Validate ECDSA
	request = &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest: &certzpb.RotateCertificateRequest_Certificates{
			Certificates: &certzpb.UploadRequest{
				Entities: []*certzpb.Entity{
					{
						Version:   "1.0",
						CreatedOn: 123456789,
						Entity: &certzpb.Entity_CertificateChain{
							CertificateChain: &certzpb.CertificateChain{
								Certificate: &certzpb.Certificate{
									Type:        certzpb.CertificateType_CERTIFICATE_TYPE_X509,
									Encoding:    certzpb.CertificateEncoding_CERTIFICATE_ENCODING_PEM,
									Certificate: certPEMtoloadRSA,
									PrivateKey:  privKeyPEMtoloadRSA,
								},
							},
						},
					},
					{
						Version:   "1.0",
						CreatedOn: 123456789,
						Entity: &certzpb.Entity_TrustBundle{
							TrustBundle: &certChainMessageECDSA,
						},
					},
				},
			},
		},
	}
	log.V(1).Info("RotateTBRequest:\n", proto.MarshalTextString(request))
	if err = stream1.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if response1, err = stream1.Recv(); err != nil {
		log.Exit("failed to receive RotateCertificateTBResponse:", err)
	}
	log.V(1).Info("RotateTBCertificateResponse:\n", proto.MarshalTextString(response1))

	//Can-generate-csr with new rotated certs
	params := certzpb.CSRParams{

		CsrSuite: certzpb.CSRSuite(2),

		CommonName: "ems.cisco.com",

		Country: "IN",
	}
	csrrequest := &certzpb.CanGenerateCSRRequest{
		Params: &params}
	got, err := gnsiC.Certz().CanGenerateCSR(context.Background(), csrrequest)

	log.V(1).Info("sending CanGenerateCSRRequest to validate:\n", proto.MarshalTextString(csrrequest))
	//response, err := clientT.CanGenerateCSR(context.Background(), csrrequest)
	if got == nil {
		log.Exit("Able to do CanGenerateCSR finalize with invalid cert", err)
	}
	log.Info("CanGenerateCSRResponse:\n", proto.MarshalTextString(response))

	//UnCONFIG NEW gNSI CLI
	t.Log("UnConfig new gNSI CLI")
	configToremove := "no grpc gnsi service certz ssl-profile-id rotatecertzvalidate \n"
	ctx = context.Background()
	util.GNMIWithText(ctx, t, dut, configToremove)

	//Delete rotated profile-id
	delprofile, err := gnsiC.Certz().DeleteProfile(context.Background(), &certzpb.DeleteProfileRequest{SslProfileId: profile_id})
	if err != nil {
		t.Fatalf("Unexpected Error in deleting profile list: %v", err)
	}
	t.Logf("Delete Profile was successful, %v", delprofile)

}
