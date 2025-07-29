// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package attestz2

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"testing"

	enrollzpb "github.com/openconfig/attestz/proto/tpm_enrollz"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/attestz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	vendorCaCertPem = flag.String("switch_vendor_ca_cert", "", "a pem file for vendor ca cert used for verifying iDevID/IAK Certs")
	ownerCaCertPem  = flag.String("switch_owner_ca_cert", "../testdata/owner-ca.cert.pem", "a pem file for ca cert that will be used to sign oDevID/oIAK/mTLS Certs")
	ownerCaKeyPem   = flag.String("switch_owner_ca_key", "../testdata/owner-ca.key.pem", "a pem file for ca key that will be used to sign oDevID/oIAK/mTLS Certs")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestAttestz2(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	// Retrieve vendor ca certificate from testdata if not provided in test args.
	if *vendorCaCertPem == "" {
		*vendorCaCertPem = fmt.Sprintf("../testdata/%s-ca.cert.pem", strings.ToLower(dut.Vendor().String()))
	}

	attestzTarget, attestzServer := attestz.SetupBaseline(t, dut)
	t.Cleanup(func() {
		gnmi.Delete(t, dut, gnmi.OC().System().GrpcServer(*attestzServer.Name).Config())
		attestz.DeleteProfile(t, dut, *attestzServer.SslProfileId)
	})
	tc := &attestz.TLSConf{
		Target:     attestzTarget,
		CaKeyFile:  *ownerCaKeyPem,
		CaCertFile: *ownerCaCertPem,
	}

	// Find active & standby card.
	activeCard, standbyCard := attestz.FindControlCards(t, dut)

	// Execute initial install workflow.
	activeCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)
	standbyCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)
	activeCard.AttestzWorkflow(t, dut, tc)
	standbyCard.AttestzWorkflow(t, dut, tc)

	t.Run("Attestz-2.1 - Successful enrollz w/o mTLS", func(t *testing.T) {
		// Re-enroll for active & standby card.
		activeCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)
		standbyCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)

		// Re-attest for active & standby card.
		activeCard.AttestzWorkflow(t, dut, tc)
		standbyCard.AttestzWorkflow(t, dut, tc)
	})

	t.Run("Attestz-2.2 - Successful enrollz with mTLS", func(t *testing.T) {
		// Enable mtls.
		tc.EnableMtls(t, dut, *attestzServer.SslProfileId)
		activeCard.MtlsCert = string(tc.ServerCert)
		standbyCard.MtlsCert = string(tc.ServerCert)

		// Re-enroll for active & standby card with mtls.
		activeCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)
		activeCard.MtlsCert = ""
		standbyCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)

		// Re-attest for active & standby card.
		activeCard.AttestzWorkflow(t, dut, tc)
		standbyCard.AttestzWorkflow(t, dut, tc)
	})

	t.Run("Attestz-2.3 - enrollz with invalid trust bundle", func(t *testing.T) {
		// Server certificate can be used to simulate an invalid trust-bundle.
		attestz.RotateCerts(t, dut, attestz.CertTypeRaw, *attestzServer.SslProfileId, nil, nil, tc.ServerCert)

		as := tc.NewSession(t)
		_, err := as.EnrollzClient.GetIakCert(context.Background(), &enrollzpb.GetIakCertRequest{})
		if err == nil {
			t.Fatalf("GetIakCert rpc succeeded but expected to fail.")
		}
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("Did not receive expected error code for GetIakCert rpc. got error: %v, want error: Unauthenticated", err)
		}
		t.Logf("Got expected error for GetIakCert rpc. error: %v", err)

		_, err = as.EnrollzClient.RotateOIakCert(context.Background(), &enrollzpb.RotateOIakCertRequest{})
		if err == nil {
			t.Fatalf("RotateOIakCert rpc succeeded but expected to fail.")
		}
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("Did not receive expected error code for RotateOIakCert rpc. got error: %v, want error: Unauthenticated", err)
		}
		t.Logf("Got expected error for RotateOIakCert rpc. error: %v", err)
	})
}
