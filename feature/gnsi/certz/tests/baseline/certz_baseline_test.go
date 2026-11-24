package certzbaseline_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	setupService "github.com/openconfig/featureprofiles/feature/gnsi/certz/tests/internal/setup_service"
	"github.com/openconfig/featureprofiles/internal/fptest"
	certzpb "github.com/openconfig/gnsi/certz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

const (
	dirPath                   = "../../test_data/"
	timeOutVar  time.Duration = 2 * time.Minute
	testProfile               = "test-profile"
	rpcTimeout                = 2 * time.Minute
)

var (
	creds DUTCredentialer
)

// DUTCredentialer is an interface for getting credentials from a DUT binding.
type DUTCredentialer interface {
	RPCUsername() string
	RPCPassword() string
}

func addProfile(t *testing.T, certzClient certzpb.CertzClient, ctx context.Context) (error string) {
	resp, err := certzClient.AddProfile(ctx, &certzpb.AddProfileRequest{
		SslProfileId: testProfile,
	})

	if err != nil {
		// If the profile already exists from a prior run, that is also acceptable.
		if st, ok := status.FromError(err); ok {
			if st.Code() == codes.AlreadyExists {
				t.Logf("profile %q already exists on DUT", testProfile)
				return
			} else {
				return fmt.Sprintf("adding profile %s failed due to error: %s", testProfile, st.Code())
			}

		}
		return fmt.Sprintf("add profile %s failed: %v", testProfile, err)
	}
	if resp == nil {
		t.Fatalf("AddProfile %s returned nil response", testProfile)
	}
	return
}

func TestCertzBaseline(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	if err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &creds); err != nil {
		t.Fatalf("STATUS:Failed to get DUT credentials using binding.DUTs: %v. The binding for %s must implement the DUTCredentialer interface.", err, dut.Name())
	}

	//Generate testdata certificates
	t.Logf("STATUS:Generation of test data certificates.")
	if err := setupService.TestdataMakeCleanup(t, dirPath, timeOutVar, "./mk_cas.sh"); err != nil {
		t.Fatalf("STATUS:Generation of testdata certificates failed!: %v", err)
	}

	type testCase struct {
		Name        string
		Description string
		testFunc    func(t *testing.T, dut *ondatra.DUTDevice)
	}

	testCases := []testCase{
		{
			Name:        "Testcase: Add Profile",
			Description: "Add a new TLS service profile to the DUT",
			testFunc:    addTLSProfileTest,
		},
		{
			Name:        "Testcase: Delete Profile",
			Description: "Delete an aged TLS service profile from the DUT",
			testFunc:    deleteTLSProfileTest,
		},
		{
			Name:        "Testcase: Get Profile List",
			Description: "Validate profile exists in the DUT",
			testFunc:    getTLSProfile,
		},
		{
			Name:        "Testcase: Validate Metrics",
			Description: "Validate that appropriate metrics are returned from streaming telemetry",
			testFunc:    validateMetricsTest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			tc.testFunc(t, dut)
		})
	}

	t.Logf("STATUS:Cleanup of test data.")
	//Cleanup of test data
	if err := setupService.TestdataMakeCleanup(t, dirPath, timeOutVar, "./mk_cas.sh"); err != nil {
		t.Logf("STATUS:Generation of testdata certificates failed!: %v", err)
	}
	t.Logf("STATUS:Test completed!")
}

// Implementation for adding a TLS service profile to the DUT
func addTLSProfileTest(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	certzClient := dut.RawAPIs().GNSI(t).Certz()

	err := addProfile(t, certzClient, ctx)
	if err != "" {
		t.Errorf("failed: %s", err)
		return
	}

	//Get ssl profile list.
	if getResp := setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Logf("profile: %s is added.", testProfile)
	} else {
		t.Errorf("profile: %s not found in profile list after add profile", testProfile)
	}
}

// Implementation for deleting an aged TLS service profile from the DUT
func deleteTLSProfileTest(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	certzClient := dut.RawAPIs().GNSI(t).Certz()

	//Get ssl profile list.
	if getResp := setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Logf("profile: %s already added.", testProfile)
	} else {
		t.Fatalf("profile: %s not found in profile list", testProfile)
	}

	_, err := certzClient.DeleteProfile(ctx, &certzpb.DeleteProfileRequest{SslProfileId: testProfile})
	if err != nil {
		t.Errorf("delete profile: %s failed: %v", testProfile, err)
	}

	//Get ssl profile list.
	if getResp := setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Errorf("profile: %s still found in profile list after delete profile", testProfile)
	} else {
		t.Logf("profile: %s successfully deleted.", testProfile)
	}
}

// Implementation for getting the current details of a TLS service profile from the DUT
func getTLSProfile(t *testing.T, dut *ondatra.DUTDevice) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	certzClient := dut.RawAPIs().GNSI(t).Certz()

	err := addProfile(t, certzClient, ctx)
	if err != "" {
		t.Errorf("failed: %s", err)
		return
	}

	//Get ssl profile list.
	if getResp := setupService.GetSslProfilelist(ctx, t, certzClient, &certzpb.GetProfileListRequest{}); slices.Contains(getResp.SslProfileIds, testProfile) {
		t.Logf("profile: %s already added.", testProfile)
	} else {
		t.Errorf("profile: %s not found in profile list", testProfile)
	}
}

// Implementation for validating that appropriate metrics are returned from streaming telemetry
func validateMetricsTest(t *testing.T, dut *ondatra.DUTDevice) {
	t.Errorf("raised issue 489348277")
}
