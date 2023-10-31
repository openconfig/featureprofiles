package certz_test

import (
	"context"
	"testing"

	"github.com/openconfig/featureprofiles/internal/fptest"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCanGenerateCSR(t *testing.T) {

	// common_param := certzpb.CSRParams{
	// 	CommonName: "testcertz.com",

	// 	Country: "US",

	// 	State: "California",

	// 	City: "San Francisco",

	// 	Organization: "Example Inc",

	// 	OrganizationalUnit: "IT",

	// 	San: &certzpb.V3ExtensionSAN{

	// 		Dns: []string{"testcertz.com", "www.testcertz.com"},

	// 		Emails: []string{"admin@testcertz.com"},

	// 		Ips: []string{"127.0.0.1"},

	// 		Uris: []string{"https://testcertz.com"},
	// 	},
	// }
	// type CanGenerateV3ExtensionSAN struct {
	// 	San certzpb.V3ExtensionSAN{Dns: }
	// }
	// type CanGenerateCSR struct {
	// 	CsrSuite   certzpb.CSRSuite
	// 	CommonName string

	// 	Country string

	// 	State string

	// 	City string

	// 	Organization string

	// 	OrganizationalUnit string
	// 	San                certzpb.V3ExtensionSAN
	// }

	// func generateCSR(commonName string, country string, state string, city string, org string, orgUnit string) *certzpb.CSRParams{
	// 	return *certzpb.CSRParams{
	// 		CsrSuite: certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_256,
	// 		CommonName: "testcertz.com",
	// 		Country: "US",
	// 		State: "California",
	// 		City: "San Francisco",
	// 		Organization: "Example Inc",
	// 		OrganizationalUnit: "IT",
	// 		San: &certzpb.V3ExtensionSAN{
	// 			Dns: []string{"testcertz.com", "www.testcertz.com"},
	// 			Emails: []string{"admin@testcertz.com"},
	// 			Ips: []string{"127.0.0.1"},
	// 			Uris: []string{"https://testcertz.com"},		
	// 		},
	// 	}

	// }

	// rsa2048testvalidparamssha256 := CanGenerateCSR{

	// 	CsrSuite:   certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_256,
	// 	CommonName: "testcertz.com",

	// 	Country: "US",

	// 	State: "California",

	// 	City: "San Francisco",

	// 	Organization: "Example Inc",

	// 	OrganizationalUnit: "IT",

	// 	San: certzpb.V3ExtensionSAN{

	// 		Dns: []string{"testcertz.com", "www.testcertz.com"},

	// 		Emails: []string{"admin@testcertz.com"},

	// 		Ips: []string{"127.0.0.1"},

	// 		Uris: []string{"https://testcertz.com"},
	// 	},
	// }
	// rsa2048testvalidparamssha384 := CanGenerateCSR{

	// 	CsrSuite:     certzpb.CSRSuite_CSRSUITE_X509_KEY_TYPE_RSA_2048_SIGNATURE_ALGORITHM_SHA_2_384,
	// 	CommonParams: common_param,
	// }


	dut := ondatra.DUT(t, "dut")
	gnsiC := dut.RawAPIs().GNSI().Default(t)
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
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: generateCSR},
			want:    &certzpb.CanGenerateCSRResponse{CanGenerate: true},
			wantErr: false,
		},
		{
			name:    "CSR with RSA 2048 SHA384 test",
			args:    args{req: &certzpb.CanGenerateCSRRequest{Params: generateCSR}},
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

/*rsa2048testvalidparamssha256.CommonParams.CommonName*/
