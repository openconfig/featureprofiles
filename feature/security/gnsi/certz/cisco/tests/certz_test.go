package certz_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"io"
	"net"
	"os"
	"testing"
	"time"

	log "github.com/golang/glog"
	"github.com/google/go-cmp/cmp"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/openconfig/featureprofiles/internal/args"
	cert "github.com/openconfig/featureprofiles/internal/cisco/security/cert"
	"github.com/openconfig/featureprofiles/internal/cisco/util"
	"github.com/openconfig/featureprofiles/internal/components"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/testt"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	spb "github.com/openconfig/gnoi/system"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
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
	data, err := os.ReadFile(filename)
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

var (
	serverIP = flag.String("serverip", "", "Server IP address")
	clientIP = flag.String("clientip", "1.1.1.1", "Client IP address")
)

func TestRotateReqWithFinalizeTestRsa(t *testing.T) {
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
	certTemp, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, []net.IP{net.ParseIP(*serverIP)}, "test", 100)
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
	//Generating Client Cert & Signed from CA
	certTemp1, err := cert.PopulateCertTemplate("client", []string{"client.cisco.com"}, []net.IP{net.ParseIP(*clientIP)}, "test", 100)
	if err != nil {
		t.Fatalf("Could not generate the cert template: %v", err)
	}
	clientCert, err := cert.GenerateCert(certTemp1, caCert, caKey, x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}
	err = cert.SaveTLSCertInPems(clientCert, "testdata/client_key.pem", "testdata/client_cert.pem", x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificates: %v", err)
	}

	//Rotate with Server cert & key

	certPEMtoload, err := os.ReadFile("testdata/server_cert.pem")
	if err != nil {
		log.Exit("Failed to read cert file", err)
	}
	privKeyPEMtoload, err := os.ReadFile("testdata/server_key.pem")
	if err != nil {
		log.Exit("Failed to read key file", err)
	}

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
	//log.V(1).Info("RotateCertificateRequest:\n", prettyPrint(request))
	log.V(1).Info("RotateCertificateRequest:\n", prettyPrint(request))
	if err = stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if response, err = stream.Recv(); err != nil {
		log.Exit("failed to receive RotateCertificateResponse:", err)
	}
	log.V(1).Info("RotateCertificateResponse:\n", prettyPrint(response))
	t.Logf("Rotate successful %v", request)

	//FINALIZE ROTATE REQUEST
	request = &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest:  &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}},
	}
	log.V(1).Info("RotateFinalizeReq:\n", prettyPrint(request))
	if err := stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if _, err = stream.Recv(); err != nil {
		if err != io.EOF {
			log.Exit("Failed, finalize Rotation is cancelled", err)
		}
	}
	log.V(1).Info("RotateCertificateFinalize: Success, stream has ended")
	stream.CloseSend()

	//CONFIG NEW gNSI CLI
	t.Log("Config new gNSI CLI")
	configToChange := "grpc gnsi service certz ssl-profile-id rotatecertzrsa \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	//Creating New gNSI Connection with rotated certs
	tlsCerts := tls.Certificate{Certificate: clientCert.Certificate,
		PrivateKey: clientCert.PrivateKey}
	var rootca *x509.Certificate
	roots := x509.NewCertPool()

	for _, cert := range certificates {
		rootca = cert
	}
	roots.AddCert(rootca)

	tlsConf := tls.Config{
		Certificates: []tls.Certificate{tlsCerts},
		RootCAs:      roots,
	}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tlsConf))}
	gnsiCNew, err := dut.RawAPIs().BindingDUT().DialGNSI(ctx, opts[len(opts)-1])
	if err != nil {
		t.Errorf("ERR %v", err)
	}

	//Get-profile with rotated CERTS
	profilelist, err := gnsiCNew.Certz().GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
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

func TestRotateReqWithFinalizeTestEcdsa(t *testing.T) {
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
	certTemp, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, []net.IP{net.ParseIP(*serverIP)}, "test", 100)
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

	//Generating Client Cert & Signed from CA
	certTemp1, err := cert.PopulateCertTemplate("client", []string{"client.cisco.com"}, []net.IP{net.ParseIP(*clientIP)}, "test", 100)
	if err != nil {
		t.Fatalf("Could not generate the cert template: %v", err)
	}
	clientCert, err := cert.GenerateCert(certTemp1, caCert, caKey, x509.ECDSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}
	err = cert.SaveTLSCertInPems(clientCert, "testdata/client_key.pem", "testdata/client_cert.pem", x509.ECDSA)
	if err != nil {
		t.Fatalf("Could not generate certificates: %v", err)
	}

	//Rotate with Server cert & Server key

	certPEMtoload, err := os.ReadFile("testdata/server_cert.pem")
	if err != nil {
		log.Exit("Failed to read cert file", err)
	}
	privKeyPEMtoload, err := os.ReadFile("testdata/server_key.pem")
	if err != nil {
		log.Exit("Failed to read key file", err)
	}

	stream, err := gnsiC.Certz().Rotate(context.Background())
	if err != nil {
		log.Exit("failed to get stream:", err)
	}
	var response *certzpb.RotateCertificateResponse

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
	log.V(1).Info("RotateTBRequest:\n", prettyPrint(request))
	if err = stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if response, err = stream.Recv(); err != nil {
		log.Exit("failed to receive RotateCertificateTBResponse:", err)
	}
	log.V(1).Info("RotateTBCertificateResponse:\n", prettyPrint(response))
	t.Logf("Rotate successful %v", request)

	//FINALIZE ROTATE REQUEST
	request = &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest:  &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}},
	}
	log.V(1).Info("RotateFinalizeReq:\n", prettyPrint(request))
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

	//Creating New gNSI Connection with rotated certs
	tlsCerts := tls.Certificate{Certificate: clientCert.Certificate,
		PrivateKey: clientCert.PrivateKey}
	var rootca *x509.Certificate
	roots := x509.NewCertPool()

	for _, cert := range certificates {
		rootca = cert
	}
	roots.AddCert(rootca)

	tlsConf := tls.Config{
		Certificates: []tls.Certificate{tlsCerts},
		RootCAs:      roots,
	}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tlsConf))}
	gnsiCNew, err := dut.RawAPIs().BindingDUT().DialGNSI(ctx, opts[len(opts)-1])
	if err != nil {
		t.Errorf("ERR %v", err)
	}

	//Get-profile with rotated CERTS
	profilelist, err := gnsiCNew.Certz().GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
	if err != nil {
		t.Fatalf("Unexpected Error in getting profile list: %v", err)
	}
	t.Logf("Profile list get was successful, %v", profilelist)

	//UnCONFIG NEW gNSI CLI
	t.Log("UnConfig new gNSI CLI")
	configToremove := "no grpc gnsi service certz ssl-profile-id rotatecertzecdsa \n"
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
	certTemp, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, []net.IP{net.ParseIP(*serverIP)}, "test", 100)
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

	certPEMtoload, err := os.ReadFile("testdata/server_cert_rsa.pem")
	if err != nil {
		log.Exit("Failed to read cert file", err)
	}
	privKeyPEMtoload, err := os.ReadFile("testdata/server_key_rsa.pem")
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

func TestRotateReqWithFinalizeValidate(t *testing.T) {
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
	certTemprsa, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, []net.IP{net.ParseIP(*serverIP)}, "test", 100)
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

	//Generating RSA Client Cert & Signed from CA
	certTemp1rsa, err := cert.PopulateCertTemplate("client", []string{"client.cisco.com"}, []net.IP{net.ParseIP(*clientIP)}, "test", 100)
	if err != nil {
		t.Fatalf("Could not generate the cert template: %v", err)
	}
	clientCert, err := cert.GenerateCert(certTemp1rsa, caCertrsa, caKeyrsa, x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}
	err = cert.SaveTLSCertInPems(clientCert, "testdatarsa/client_key.pem", "testdatarsa/client_cert.pem", x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificates: %v", err)
	}

	//Rotate with RSA Server cert & key

	certPEMtoloadRSA, err := os.ReadFile("testdatarsa/server_cert_rsa.pem")
	if err != nil {
		log.Exit("Failed to read cert file", err)
	}
	privKeyPEMtoloadRSA, err := os.ReadFile("testdatarsa/server_key_rsa.pem")
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
	certTempecdsa, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, []net.IP{net.ParseIP(*serverIP)}, "test", 100)
	if err != nil {
		t.Fatalf("Could not generate the cert template: %v", err)
	}
	tlscertecdsa, err := cert.GenerateCert(certTempecdsa, caCertecdsa, caKeyecdsa, x509.ECDSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}
	err = cert.SaveTLSCertInPems(tlscertecdsa, "testdataecdsa/server_key_ecdsa.pem", "testdataecdsa/server_cert_ecdsa.pem", x509.ECDSA)
	if err != nil {
		t.Fatalf("Could not generate certificates: %v", err)
	}
	stream, err := gnsiC.Certz().Rotate(context.Background())
	if err != nil {
		log.Exit("failed to get stream:", err)
	}
	var response *certzpb.RotateCertificateResponse

	//Generating ECDSA Client Cert & Signed from CA
	certTemp1ecdsa, err := cert.PopulateCertTemplate("client", []string{"client.cisco.com"}, []net.IP{net.ParseIP(*clientIP)}, "test", 100)
	if err != nil {
		t.Fatalf("Could not generate the cert template: %v", err)
	}
	tlscert2, err := cert.GenerateCert(certTemp1ecdsa, caCertecdsa, caKeyecdsa, x509.ECDSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}
	err = cert.SaveTLSCertInPems(tlscert2, "testdataecdsa/client_key.pem", "testdataecdsa/client_cert.pem", x509.ECDSA)
	if err != nil {
		t.Fatalf("Could not generate certificates: %v", err)
	}

	//RSA TRUSTBUNDLE
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

	//ECDSA TRUSTBUNDLE
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

	log.V(1).Info("RotateTBRequest:\n", prettyPrint(request))
	if err = stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if response, err = stream.Recv(); err != nil {
		log.Exit("failed to receive RotateCertificateTBResponse:", err)
	}
	log.V(1).Info("RotateTBCertificateResponse:\n", prettyPrint(response))

	//FINALIZE ROTATE REQUEST
	request = &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest:  &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}},
	}
	log.V(1).Info("RotateFinalizeReq:\n", prettyPrint(request))
	if err := stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if _, err = stream.Recv(); err != nil {
		if err != io.EOF {
			log.Exit("Failed, finalize Rotation is cancelled", err)
		}
	}
	log.V(1).Info("RotateCertificateFinalize: Success, stream has ended")
	stream.CloseSend()

	//CONFIG NEW gNSI CLI
	t.Log("Config new gNSI CLI")
	configToChange := "grpc gnsi service certz ssl-profile-id rotatecertzvalidate \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	//Creating New gNSI Connection with RSA rotated certs
	tlsCerts := tls.Certificate{Certificate: clientCert.Certificate,
		PrivateKey: clientCert.PrivateKey}
	var rootca *x509.Certificate
	roots := x509.NewCertPool()

	for _, cert := range certificatesrsa {
		rootca = cert
	}
	roots.AddCert(rootca)

	tlsConf := tls.Config{
		Certificates: []tls.Certificate{tlsCerts},
		RootCAs:      roots,
	}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tlsConf))}
	gnsiCNew, _ := dut.RawAPIs().BindingDUT().DialGNSI(ctx, opts[len(opts)-1])

	//Get-profile with rotated CERTS
	profilelist, err := gnsiCNew.Certz().GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
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

	log.V(1).Info("RotateTBRequest:\n", prettyPrint(request))
	if err = stream1.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if response1, err = stream1.Recv(); err != nil {
		log.Exit("failed to receive RotateCertificateTBResponse:", err)
	}
	log.V(1).Info("RotateTBCertificateResponse:\n", prettyPrint(response1))

	//Creating New gNSI Connection with RSA Server Cert &  ECDSA  Trustbundle with out Finalise
	var rootcaecdsa *x509.Certificate
	rootecdsa := x509.NewCertPool()

	for _, cert := range certificatesecdsa {
		rootcaecdsa = cert
	}
	rootecdsa.AddCert(rootcaecdsa)

	tlsConf = tls.Config{
		Certificates: []tls.Certificate{tlsCerts},
		RootCAs:      rootecdsa,
	}

	opts1 := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tlsConf))}

	retryOpt := grpc_retry.WithPerRetryTimeout(time.Duration(2) * time.Second)
	opts = append(opts1,
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(retryOpt)),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(retryOpt)),
	)
	var cancelFunc context.CancelFunc
	ctx, cancelFunc = context.WithTimeout(ctx, time.Duration(2)*time.Second)
	defer cancelFunc()

	_, err = dut.RawAPIs().BindingDUT().DialGNSI(ctx, opts[len(opts)-1])
	if err != nil {
		t.Log("Expected Failure, Error:", err)
	} else {
		t.Errorf("Connection established with wrong certs, which is not expected")
	}
	stream1.CloseSend()

	//UnCONFIG NEW gNSI CLI
	t.Log("UnConfig new gNSI CLI")
	configToremove := "no grpc gnsi service certz ssl-profile-id rotatecertzvalidate \n"
	ctx = context.Background()
	util.GNMIWithText(ctx, t, dut, configToremove)
	t.Log("Removed gNSI Profile config Successfully")

	//Delete rotated profile-id
	delprofile, err := gnsiC.Certz().DeleteProfile(context.Background(), &certzpb.DeleteProfileRequest{SslProfileId: profile_id})
	if err != nil {
		t.Fatalf("Unexpected Error in deleting profile list: %v", err)
	}
	t.Logf("Delete Profile was successful, %v", delprofile)

}

const (
	maxSwitchoverTime = 900
	controlcardType   = oc.PlatformTypes_OPENCONFIG_HARDWARE_COMPONENT_CONTROLLER_CARD
	activeController  = oc.Platform_ComponentRedundantRole_PRIMARY
	standbyController = oc.Platform_ComponentRedundantRole_SECONDARY
)

func TestHARedundancySwithOver(t *testing.T) {
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
	certTemp, err := cert.PopulateCertTemplate("server", []string{"Server.cisco.com"}, []net.IP{net.ParseIP(*serverIP)}, "test", 100)
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
	//Generating Client Cert & Signed from CA
	certTemp1, err := cert.PopulateCertTemplate("client", []string{"client.cisco.com"}, []net.IP{net.ParseIP(*clientIP)}, "test", 100)
	if err != nil {
		t.Fatalf("Could not generate the cert template: %v", err)
	}
	clientCert, err := cert.GenerateCert(certTemp1, caCert, caKey, x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificate template: %v", err)
	}
	err = cert.SaveTLSCertInPems(clientCert, "testdata/client_key.pem", "testdata/client_cert.pem", x509.RSA)
	if err != nil {
		t.Fatalf("Could not generate certificates: %v", err)
	}

	//Rotate with Server cert & key

	certPEMtoload, err := os.ReadFile("testdata/server_cert.pem")
	if err != nil {
		log.Exit("Failed to read cert file", err)
	}
	privKeyPEMtoload, err := os.ReadFile("testdata/server_key.pem")
	if err != nil {
		log.Exit("Failed to read key file", err)
	}

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
	log.V(1).Info("RotateCertificateRequest:\n", prettyPrint(request))
	if err = stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if response, err = stream.Recv(); err != nil {
		log.Exit("failed to receive RotateCertificateResponse:", err)
	}
	log.V(1).Info("RotateCertificateResponse:\n", prettyPrint(response))
	t.Logf("Rotate successful %v", request)

	//FINALIZE ROTATE REQUEST
	request = &certzpb.RotateCertificateRequest{
		ForceOverwrite: true,
		SslProfileId:   profile_id,
		RotateRequest:  &certzpb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &certzpb.FinalizeRequest{}},
	}
	log.V(1).Info("RotateFinalizeReq:\n", prettyPrint(request))
	if err := stream.Send(request); err != nil {
		log.Exit("failed to send RotateRequest:", err)
	}
	if _, err = stream.Recv(); err != nil {
		if err != io.EOF {
			log.Exit("Failed, finalize Rotation is cancelled", err)
		}
	}
	log.V(1).Info("RotateCertificateFinalize: Success, stream has ended")
	stream.CloseSend()

	//CONFIG NEW gNSI CLI
	t.Log("Config new gNSI CLI")
	configToChange := "grpc gnsi service certz ssl-profile-id rotatecertzrsa \n"
	ctx := context.Background()
	util.GNMIWithText(ctx, t, dut, configToChange)

	//Creating New gNSI Connection with rotated certs
	tlsCerts := tls.Certificate{Certificate: clientCert.Certificate,
		PrivateKey: clientCert.PrivateKey}
	var rootca *x509.Certificate
	roots := x509.NewCertPool()

	for _, cert := range certificates {
		rootca = cert
	}
	roots.AddCert(rootca)

	tlsConf := tls.Config{
		Certificates: []tls.Certificate{tlsCerts},
		RootCAs:      roots,
	}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tlsConf))}
	gnsiCNew, err := dut.RawAPIs().BindingDUT().DialGNSI(ctx, opts[len(opts)-1])
	if err != nil {
		t.Errorf("ERR %v", err)
	}

	//Get-profile with rotated CERTS
	profilelist, err := gnsiCNew.Certz().GetProfileList(context.Background(), &certzpb.GetProfileListRequest{})
	if err != nil {
		t.Fatalf("Unexpected Error in getting profile list: %v", err)
	}
	t.Logf("Profile list get was successful, %v", profilelist)

	controllerCards := components.FindComponentsByType(t, dut, controlcardType)
	t.Logf("Found controller card list: %v", controllerCards)

	if *args.NumControllerCards >= 0 && len(controllerCards) != *args.NumControllerCards {
		t.Errorf("Incorrect number of controller cards: got %v, want exactly %v (specified by flag)", len(controllerCards), *args.NumControllerCards)
	}

	if got, want := len(controllerCards), 2; got < want {
		t.Skipf("Not enough controller cards for the test on %v: got %v, want at least %v", dut.Model(), got, want)
	}

	rpStandbyBeforeSwitch, rpActiveBeforeSwitch := components.FindStandbyRP(t, dut, controllerCards)
	t.Logf("Detected rpStandby: %v, rpActive: %v", rpStandbyBeforeSwitch, rpActiveBeforeSwitch)

	switchoverReady := gnmi.OC().Component(rpActiveBeforeSwitch).SwitchoverReady()
	gnmi.Await(t, dut, switchoverReady.State(), 30*time.Minute, true)
	t.Logf("SwitchoverReady().Get(t): %v", gnmi.Get(t, dut, switchoverReady.State()))
	if got, want := gnmi.Get(t, dut, switchoverReady.State()), true; got != want {
		t.Errorf("switchoverReady.Get(t): got %v, want %v", got, want)
	}

	gnoiClient, err := dut.RawAPIs().BindingDUT().DialGNOI(ctx, opts[len(opts)-1])
	if err != nil {
		t.Fatalf("GNOI dial is failed %v", err)
	}
	useNameOnly := deviations.GNOISubcomponentPath(dut)
	switchoverRequest := &spb.SwitchControlProcessorRequest{
		ControlProcessor: components.GetSubcomponentPath(rpStandbyBeforeSwitch, useNameOnly),
	}
	t.Logf("switchoverRequest: %v", switchoverRequest)
	switchoverResponse, err := gnoiClient.System().SwitchControlProcessor(context.Background(), switchoverRequest)
	if err != nil {
		t.Fatalf("Failed to perform control processor switchover with unexpected err: %v", err)
	}
	t.Logf("gnoiClient.System().SwitchControlProcessor() response: %v, err: %v", switchoverResponse, err)

	want := rpStandbyBeforeSwitch
	got := ""
	if deviations.GNOISubcomponentPath(dut) {
		got = switchoverResponse.GetControlProcessor().GetElem()[0].GetName()
	} else {
		got = switchoverResponse.GetControlProcessor().GetElem()[1].GetKey()["name"]
	}
	if got != want {
		t.Fatalf("switchoverResponse.GetControlProcessor().GetElem()[0].GetName(): got %v, want %v", got, want)
	}
	if got, want := switchoverResponse.GetVersion(), ""; got == want {
		t.Errorf("switchoverResponse.GetVersion(): got %v, want non-empty version", got)
	}
	if got := switchoverResponse.GetUptime(); got == 0 {
		t.Errorf("switchoverResponse.GetUptime(): got %v, want > 0", got)
	}

	startSwitchover := time.Now()
	t.Logf("Wait for new active RP to boot up by polling the telemetry output.")
	getRequest := &gnmipb.GetRequest{
		Path: []*gnmipb.Path{
			{Origin: "openconfig", Elem: []*gnmipb.PathElem{
				{Name: "system"},
				{Name: "state"},
				{Name: "hostname"},
			}},
		},
		Type:     gnmipb.GetRequest_ALL,
		Encoding: gnmipb.Encoding_JSON_IETF,
	}

	gnmi, err := dut.RawAPIs().BindingDUT().DialGNMI(ctx, opts[len(opts)-1])
	if err != nil {
		t.Fatalf("GNOI dial is failed %v", err)
	}
	for {
		var currentTime *gnmipb.GetResponse
		t.Logf("Time elapsed %.2f seconds since switchover started.", time.Since(startSwitchover).Seconds())
		time.Sleep(60 * time.Second)
		if errMsg := testt.CaptureFatal(t, func(t testing.TB) {
			currentTime, _ = gnmi.Get(context.Background(), getRequest)
		}); errMsg != nil {
			t.Logf("Got testt.CaptureFatal errMsg: %s, keep polling ...", *errMsg)
		} else {
			t.Logf("RP switchover has completed successfully with received time: %v", currentTime)
			break
		}
		if got, want := uint64(time.Since(startSwitchover).Seconds()), uint64(maxSwitchoverTime); got >= want {
			t.Fatalf("time.Since(startSwitchover): got %v, want < %v", got, want)
		}
	}
	t.Logf("RP switchover time: %.2f seconds", time.Since(startSwitchover).Seconds())

	getResponse, err := gnmi.Get(context.Background(), getRequest)
	if err != nil {
		t.Fatal("Get hostname is failed after switchover")
	}
	t.Logf("VAL:  %v ", getResponse)

	//UnCONFIG NEW gNSI CLI
	t.Log("UnConfig new gNSI CLI")
	r := &gnmipb.SetRequest{
		Update: []*gnmipb.Update{
			{
				Path: &gnmipb.Path{Origin: "cli"},
				Val:  &gnmipb.TypedValue{Value: &gnmipb.TypedValue_AsciiVal{AsciiVal: "no grpc gnsi service certz ssl-profile-id rotatecertzrsa \n"}},
			},
		},
	}
	_, err = gnmi.Set(context.Background(), r)
	if err != nil {
		t.Fatalf("There is error when applying the config, %v", err)
	}

	//Delete rotated profile-id using the old cert, not passing cert does that
	gnsiC, err = dut.RawAPIs().BindingDUT().DialGNSI(ctx)
	if err != nil {
		t.Fatalf("There is error reconnecting  GNSI, %v", err)
	}
	delprofile, err := gnsiC.Certz().DeleteProfile(context.Background(), &certzpb.DeleteProfileRequest{SslProfileId: profile_id})
	if err != nil {
		t.Fatalf("Unexpected Error in deleting profile list: %v", err)
	}
	t.Logf("Delete Profile was successful, %v", delprofile)
}
