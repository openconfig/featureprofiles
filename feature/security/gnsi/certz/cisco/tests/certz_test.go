package certz_test

import (
	"context"
	"github.com/openconfig/featureprofiles/internal/fptest"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
	"reflect"
	"testing"
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
			if (err != nil) && !tt.wantErr {
				t.Errorf("Server.AddProfile() error = %v, wantErr %v, %s", err, tt.wantErr, tt.args.req)
			}
			else if (err == nil) && tt.wantErr {
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
			args:    args{req: nil},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := gnsiC.Certz().GetProfileList(context.Background(), tt.args.req)
			if got != nil {
				if !reflect.DeepEqual(got.SslProfileIds, tt.want.SslProfileIds) {
					t.Errorf("Server.GetProfileList() error: did not got the expected list")
				}
			}
			if (err != nil) != tt.wantErr {
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
			args:    args{req: nil},
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
			if (err != nil) && !tt.wantErr {
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
			if (err != nil) != tt.wantErr {
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
