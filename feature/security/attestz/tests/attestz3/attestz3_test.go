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

package attestz3

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	cdpb "github.com/openconfig/attestz/proto/common_definitions"
	attestzpb "github.com/openconfig/attestz/proto/tpm_attestz"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/attestz"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	cdpbActive  = cdpb.ControlCardRole_CONTROL_CARD_ROLE_ACTIVE
	cdpbStandby = cdpb.ControlCardRole_CONTROL_CARD_ROLE_STANDBY
)

type attestResponse struct {
	activeCard  *attestzpb.AttestResponse
	standbyCard *attestzpb.AttestResponse
}

var (
	vendorCaCertPem = flag.String("switch_vendor_ca_cert", "", "a pem file for vendor ca cert used for verifying iDevID/IAK Certs")
	ownerCaCertPem  = flag.String("switch_owner_ca_cert", "../testdata/owner-ca.cert.pem", "a pem file for ca cert that will be used to sign oDevID/oIAK/mTLS Certs")
	ownerCaKeyPem   = flag.String("switch_owner_ca_key", "../testdata/owner-ca.key.pem", "a pem file for ca key that will be used to sign oDevID/oIAK/mTLS Certs")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestAttestz3(t *testing.T) {
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

	// Enable mtls.
	tc.EnableMtls(t, dut, *attestzServer.SslProfileId)
	activeCard.MtlsCert = string(tc.ServerCert)
	standbyCard.MtlsCert = string(tc.ServerCert)

	t.Run("Attestz-3.1 - Re-attest with mTLS", func(t *testing.T) {
		// Re-attest for active & standby card.
		activeCard.AttestzWorkflow(t, dut, tc)
		standbyCard.AttestzWorkflow(t, dut, tc)
	})

	t.Run("Attestz-3.2 - Re-attest with device reboot", func(t *testing.T) {
		as := tc.NewSession(t)
		defer as.Conn.Close()

		// Collect attest response before reboot.
		attestRespMap := make(map[attestzpb.Tpm20HashAlgo]*attestResponse)
		for _, hashAlgo := range attestz.PcrBankHashAlgoMap[dut.Vendor()] {
			attestRespMap[hashAlgo] = new(attestResponse)
			attestRespMap[hashAlgo].activeCard = as.RequestAttestation(t, activeCard.Role, attestz.GenNonce(t), hashAlgo, attestz.PcrIndices)
			attestRespMap[hashAlgo].standbyCard = as.RequestAttestation(t, standbyCard.Role, attestz.GenNonce(t), hashAlgo, attestz.PcrIndices)
		}

		// Trigger reboot.
		attestz.RebootDut(t, dut)
		t.Logf("Wait for cards to get synchronized after reboot ...")
		attestz.SwitchoverReady(t, dut, activeCard.Name, standbyCard.Name)

		// Check active card after reboot & swap control card struct if required.
		rr := gnmi.Get[oc.E_Platform_ComponentRedundantRole](t, dut, gnmi.OC().Component(activeCard.Name).RedundantRole().State())
		if rr != oc.Platform_ComponentRedundantRole_PRIMARY {
			t.Logf("Card roles have changed. %s is the new active card.", standbyCard.Name)
			*activeCard, *standbyCard = *standbyCard, *activeCard
			activeCard.Role = cdpbActive
			standbyCard.Role = cdpbStandby
		}

		// Create new attestz session after reboot.
		as = tc.NewSession(t)
		defer as.Conn.Close()

		// Verify quote after reboot is different.
		for _, hashAlgo := range attestz.PcrBankHashAlgoMap[dut.Vendor()] {
			resp := as.RequestAttestation(t, activeCard.Role, attestz.GenNonce(t), hashAlgo, attestz.PcrIndices)
			if cmp.Equal(attestRespMap[hashAlgo].activeCard.Quoted, resp.Quoted) {
				t.Logf("Attest response for active card %s before reboot: \n%s", activeCard.Name, attestz.PrettyPrint(attestRespMap[hashAlgo].activeCard))
				t.Logf("Attest response for active card %s after reboot: \n%s", activeCard.Name, attestz.PrettyPrint(resp.Quoted))
				t.Errorf("Received similar quotes for active card %s hash algo: %v before and after reboot but expected different.", activeCard.Name, hashAlgo)
			}
			resp = as.RequestAttestation(t, standbyCard.Role, attestz.GenNonce(t), hashAlgo, attestz.PcrIndices)
			if cmp.Equal(attestRespMap[hashAlgo].standbyCard.Quoted, resp.Quoted) {
				t.Logf("Attest response for standby card %s before reboot: \n%s", standbyCard.Name, attestz.PrettyPrint(attestRespMap[hashAlgo].standbyCard))
				t.Logf("Attest response for standby card %s after reboot: \n%s", standbyCard.Name, attestz.PrettyPrint(resp.Quoted))
				t.Errorf("Received similar quotes for standby card %s hash algo: %v before and after reboot but expected different.", standbyCard.Name, hashAlgo)
			}
		}

		// Re-attest for active & standby card after reboot.
		activeCard.AttestzWorkflow(t, dut, tc)
		standbyCard.AttestzWorkflow(t, dut, tc)

	})

	t.Run("Attestz-3.3 - Re-attest with switchover", func(t *testing.T) {
		// Perform switchover without waiting for cards to sync.
		attestz.SwitchoverCards(t, dut, activeCard.Name, standbyCard.Name)

		// Swap active and standby card after switchover.
		*activeCard, *standbyCard = *standbyCard, *activeCard
		activeCard.Role = cdpbActive
		standbyCard.Role = cdpbStandby

		// Verify device passes attestation after switchover.
		activeCard.AttestzWorkflow(t, dut, tc)
	})

	t.Run("Attestz-3.4 - Re-attest with invalid trust bundle", func(t *testing.T) {
		// Server certificate can be used to simulate an invalid trust-bundle.
		attestz.RotateCerts(t, dut, attestz.CertTypeRaw, *attestzServer.SslProfileId, nil, nil, tc.ServerCert)
		as := tc.NewSession(t)
		_, err := as.AttestzClient.Attest(context.Background(), &attestzpb.AttestRequest{})
		if err == nil {
			t.Fatalf("Attest rpc succeeded but expected to fail.")
		}
		if status.Code(err) != codes.Unauthenticated {
			t.Errorf("Did not receive expected error code for Attest rpc. got error: %v, want error: Unauthenticated", err)
		}
		t.Logf("Got expected error for Attest rpc. error: %v", err)
	})

}
