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

package attestz1

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"testing"

	cdpb "github.com/openconfig/attestz/proto/common_definitions"
	attestzpb "github.com/openconfig/attestz/proto/tpm_attestz"
	enrollzpb "github.com/openconfig/attestz/proto/tpm_enrollz"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/attestz"
	"github.com/openconfig/featureprofiles/internal/security/svid"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	cdpbActive  = cdpb.ControlCardRole_CONTROL_CARD_ROLE_ACTIVE
	cdpbStandby = cdpb.ControlCardRole_CONTROL_CARD_ROLE_STANDBY
)

var (
	vendorCaCertPem = flag.String("switch_vendor_ca_cert", "", "a pem file for vendor ca cert used for verifying iDevID/IAK Certs")
	ownerCaCertPem  = flag.String("switch_owner_ca_cert", "../testdata/owner-ca.cert.pem", "a pem file for ca cert that will be used to sign oDevID/oIAK/mTLS Certs")
	ownerCaKeyPem   = flag.String("switch_owner_ca_key", "../testdata/owner-ca.key.pem", "a pem file for ca key that will be used to sign oDevID/oIAK/mTLS Certs")
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestAttestz1(t *testing.T) {
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
	tc := &attestz.TlsConf{
		Target:     attestzTarget,
		CaKeyFile:  *ownerCaKeyPem,
		CaCertFile: *ownerCaCertPem,
	}

	// Find active and standby card.
	activeCard, standbyCard := attestz.FindControlCards(t, dut)

	t.Run("Attestz-1.1 - Successful enrollment and attestation", func(t *testing.T) {
		// Enroll for active & standby card.
		activeCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)
		standbyCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)

		// Attest for active & standby card.
		activeCard.AttestzWorkflow(t, dut, tc)
		standbyCard.AttestzWorkflow(t, dut, tc)
	})

	t.Run("Attestz-1.3 - Bad request", func(t *testing.T) {
		var as *attestz.AttestzSession
		as = tc.NewAttestzSession(t)
		defer as.Conn.Close()
		invalidSerial := attestz.ParseSerialSelection("000")

		_, err := as.EnrollzClient.GetIakCert(context.Background(), &enrollzpb.GetIakCertRequest{
			ControlCardSelection: invalidSerial,
		})
		if err != nil {
			t.Logf("Got expected error for GetIakCert bad request %v", err)
		} else {
			t.Fatal("GetIakCert rpc succeeded but expected to fail.")
		}

		attestzRequest := &enrollzpb.RotateOIakCertRequest{
			ControlCardSelection: invalidSerial,
		}
		_, err = as.EnrollzClient.RotateOIakCert(context.Background(), attestzRequest)
		if err != nil {
			t.Logf("Got expected error for RotateOIakCert bad request %v", err)
		} else {
			t.Fatal("RotateOIakCert rpc succeeded but expected to fail.")
		}

		_, err = as.AttestzClient.Attest(context.Background(), &attestzpb.AttestRequest{
			ControlCardSelection: invalidSerial,
		})
		if err != nil {
			t.Logf("Got expected error for Attest bad request %v", err)
		} else {
			t.Fatal("Attest rpc succeeded but expected to fail.")
		}

	})

	t.Run("Attestz-1.4 - Incorrect Public Key", func(t *testing.T) {
		roleA := attestz.ParseRoleSelection(cdpbActive)
		roleB := attestz.ParseRoleSelection(cdpbStandby)

		// Get vendor certs.
		var as *attestz.AttestzSession
		as = tc.NewAttestzSession(t)
		defer as.Conn.Close()
		resp := as.GetVendorCerts(t, roleA)
		activeCard.IAKCert, activeCard.IDevIDCert = resp.IakCert, resp.IdevidCert
		resp = as.GetVendorCerts(t, roleB)
		standbyCard.IAKCert, standbyCard.IDevIDCert = resp.IakCert, resp.IdevidCert

		caKey, caCert, err := svid.LoadKeyPair(tc.CaKeyFile, tc.CaCertFile)
		if err != nil {
			t.Fatalf("Could not load ca key/cert. error: %v", err)
		}

		// Generate active card's oIAK/oIDevId certs with standby card's public key (to simulate incorrect public key).
		standbyIAKCert, err := attestz.LoadCertificate(standbyCard.IAKCert)
		if err != nil {
			t.Fatalf("Error loading IAK cert for standby card. error: %v", err)
		}
		t.Logf("Generating oIAK cert for card %v with incorrect public key", activeCard.Name)
		oIAKCert := attestz.GenOwnerCert(t, caKey, caCert, activeCard.IAKCert, standbyIAKCert.PublicKey, tc.Target)

		standbyIDevIDCert, err := attestz.LoadCertificate(standbyCard.IDevIDCert)
		if err != nil {
			t.Fatalf("Error loading IDevID Cert for standby card. error: %v", err)
		}
		t.Logf("Generating oDevID cert for card %v with incorrect public key", activeCard.Name)
		oDevIDCert := attestz.GenOwnerCert(t, caKey, caCert, activeCard.IDevIDCert, standbyIDevIDCert.PublicKey, tc.Target)

		// Verify rotate rpc fails.
		attestzRequest := &enrollzpb.RotateOIakCertRequest{
			ControlCardSelection: roleA,
			OiakCert:             oIAKCert,
			OidevidCert:          oDevIDCert,
			SslProfileId:         *attestzServer.SslProfileId,
		}
		_, err = as.EnrollzClient.RotateOIakCert(context.Background(), attestzRequest)
		if err != nil {
			t.Logf("Got expected error for RotateOIakCert bad request %v", err)
		} else {
			t.Fatal("RotateOIakCert rpc succeeded but expected to fail.")
		}
	})

	t.Run("Attestz-1.5 - Device Reboot", func(t *testing.T) {
		activeCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)
		standbyCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)

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

		// Verify attest workflow after reboot.
		activeCard.AttestzWorkflow(t, dut, tc)
		standbyCard.AttestzWorkflow(t, dut, tc)
	})

	t.Run("Attestz-1.6 - Factory Reset", func(t *testing.T) {
		activeCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)
		standbyCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)

		// Trigger factory reset.
		attestz.FactoryResetDut(t, dut)
		t.Logf("Wait for cards to get synchronized after factory reset ...")
		attestz.SwitchoverReady(t, dut, activeCard.Name, standbyCard.Name)

		// Setup baseline configs again after factory reset (ensure bootz pushes relevant configs used by ondatra bindings).
		attestzTarget, attestzServer = attestz.SetupBaseline(t, dut)
		activeCard, standbyCard = attestz.FindControlCards(t, dut)

		var as *attestz.AttestzSession
		as = tc.NewAttestzSession(t)
		defer as.Conn.Close()

		_, err := as.AttestzClient.Attest(context.Background(), &attestzpb.AttestRequest{
			ControlCardSelection: attestz.ParseRoleSelection(cdpbActive),
			Nonce:                attestz.GenNonce(t),
			HashAlgo:             attestzpb.Tpm20HashAlgo_TPM20HASH_ALGO_SHA256,
			PcrIndices:           attestz.PcrIndices,
		})
		if err != nil {
			t.Logf("Got expected error for Attest rpc of active card %s. error: %s", activeCard.Name, err)
		} else {
			t.Fatalf("Attest rpc for active card %s succeeded but expected to fail.", activeCard.Name)
		}
	})

	t.Run("Attestz-1.7 - Invalid PCR indices", func(t *testing.T) {
		activeCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)
		var as *attestz.AttestzSession
		as = tc.NewAttestzSession(t)
		defer as.Conn.Close()
		_, err := as.AttestzClient.Attest(context.Background(), &attestzpb.AttestRequest{
			ControlCardSelection: attestz.ParseRoleSelection(activeCard.Role),
			Nonce:                attestz.GenNonce(t),
			HashAlgo:             attestzpb.Tpm20HashAlgo_TPM20HASH_ALGO_SHA256,
			PcrIndices:           []int32{25, -25},
		})
		if err != nil {
			t.Logf("Got expected error for Attest bad request %v", err)
		} else {
			t.Fatal("Expected error in Attest with invalid pcr indices")
		}
	})

	// Ensure factory reset test ran before running this test to simulate rma scenario.
	t.Run("Attestz-1.8 - Attest failure on standby card", func(t *testing.T) {
		// Enroll & attest active card.
		activeCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)
		activeCard.AttestzWorkflow(t, dut, tc)

		var as *attestz.AttestzSession
		as = tc.NewAttestzSession(t)
		defer as.Conn.Close()
		_, err := as.AttestzClient.Attest(context.Background(), &attestzpb.AttestRequest{
			ControlCardSelection: attestz.ParseRoleSelection(cdpbStandby),
			Nonce:                attestz.GenNonce(t),
			HashAlgo:             attestzpb.Tpm20HashAlgo_TPM20HASH_ALGO_SHA256,
			PcrIndices:           attestz.PcrIndices,
		})
		if err != nil {
			t.Logf("Got expected error for Attest rpc of standby card %s. error: %s", standbyCard.Name, err)
		} else {
			t.Fatalf("Attest rpc for standby card %s succeeded but expected to fail.", standbyCard.Name)
		}
	})

	t.Run("Attestz-1.9 - Control Card Switchover", func(t *testing.T) {
		activeCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)
		standbyCard.EnrollzWorkflow(t, dut, tc, *vendorCaCertPem)

		// Trigger control card switchover.
		attestz.SwitchoverCards(t, dut, activeCard.Name, standbyCard.Name)
		t.Logf("Wait for cards to get synchronized after switchover ...")
		attestz.SwitchoverReady(t, dut, activeCard.Name, standbyCard.Name)

		// Swap control card struct after switchover.
		*activeCard, *standbyCard = *standbyCard, *activeCard
		activeCard.Role = cdpbActive
		standbyCard.Role = cdpbStandby

		// Verify attest workflow after switchover.
		activeCard.AttestzWorkflow(t, dut, tc)
		standbyCard.AttestzWorkflow(t, dut, tc)
	})
}
