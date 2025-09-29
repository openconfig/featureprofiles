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

package hostcertificates_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/ondatra/gnmi"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/credz"
	"github.com/openconfig/ondatra"
	"golang.org/x/crypto/ssh"
)

const (
	hostCertificateVersion = "v1.0"
)

var (
	username                 = "testuser"
	passwordVersion          = "v1.0"
	hostCertificateCreatedOn = uint64(time.Now().Unix())
	passwordCreatedOn        = uint64(time.Now().Unix())
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCredentialz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	// Create temporary directory for storing ssh keys/certificates.
	dir := t.TempDir()

	// Setup test user and password.
	credz.SetupUser(t, dut, username)
	password := credz.GeneratePassword()

	t.Logf("Rotating user password on DUT")
	credz.RotateUserPassword(t, dut, username, password, passwordVersion, uint64(passwordCreatedOn))

	// Create ssh keys/certificates for CA & host.
	credz.CreateSSHKeyPair(t, dir, "ca")
	credz.CreateSSHKeyPair(t, dir, dut.ID())
	credz.RotateAuthenticationArtifacts(t, dut, dir, "", hostCertificateVersion, hostCertificateCreatedOn)
	dutKey := credz.GetDutPublicKey(t, dut)
	credz.CreateHostCertificate(t, dut, dir, dutKey)
	credz.RotateAuthenticationArtifacts(t, dut, "", dir, hostCertificateVersion, hostCertificateCreatedOn)

	t.Run("dut should return signed host certificate", func(t *testing.T) {
		certificateContents, err := os.ReadFile(fmt.Sprintf("%s/%s-cert.pub", dir, dut.ID()))
		if err != nil {
			t.Fatalf("Failed reading host signed certificate, error: %s", err)
		}
		wantHostKey, _, _, _, err := ssh.ParseAuthorizedKey(certificateContents)
		if err != nil {
			t.Fatalf("Failed parsing host certificate authorized (cert)key: %s", err)
		}

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		client, err := credz.SSHWithPassword(ctx, dut, username, password)
		if err != nil {
			t.Fatalf("Failed dialing ssh with password: %s", err)
		}

		gotHostKey, _, _, _, err := ssh.ParseAuthorizedKey(client.HostKey())
		if err != nil {
			t.Fatalf("Failed parsing host certificate from device: %s", err)
		}

		// Verify correct host certificate is returned by the dut.
		if diff := cmp.Diff(gotHostKey, wantHostKey); diff != "" {
			t.Errorf("Host presented key (cert) that does not match expected host certificate. +got, -want: %s", diff)
		}

		// Verify host certificate telemetry values.
		sshServer := gnmi.Get(t, dut, gnmi.OC().System().SshServer().State())
		gotHostCertificateVersion := sshServer.GetActiveHostCertificateVersion()
		if !cmp.Equal(gotHostCertificateVersion, hostCertificateVersion) {
			t.Errorf(
				"Telemetry reports host certificate version is not correct\n\tgot: %s\n\twant: %s",
				gotHostCertificateVersion, hostCertificateVersion,
			)
		}
		gotHostCertificateCreatedOn := sshServer.GetActiveHostCertificateCreatedOn()
		if got, want := gotHostCertificateCreatedOn, hostCertificateCreatedOn*1e9; got != want {
			t.Errorf(
				"Telemetry reports host certificate created on is not correct\n\twant: %d\n\tgot: %d",
				want, got,
			)
		}
	})
}
