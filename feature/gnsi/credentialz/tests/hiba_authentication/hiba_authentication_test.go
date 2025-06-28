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

package hibaauthentication_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/security/credz"
	"github.com/openconfig/ondatra/gnmi"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	cpb "github.com/openconfig/gnsi/credentialz"
	"github.com/openconfig/ondatra"
)

const (
	username               = "testuser"
	maxSSHRetryTime        = 30 // Unit is seconds.
	hostCertificateVersion = "v1.0"
)

var (
	hostCertificateCreatedOn = time.Now().Unix()
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCredentialz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	target := credz.GetDutTarget(t, dut)

	// Create temporary directory for storing ssh keys/certificates.
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("creating temp dir, err: %s", err)
	}
	defer func(dir string) {
		err = os.RemoveAll(dir)
		if err != nil {
			t.Logf("error removing temp directory, error: %s", err)
		}
	}(dir)

	credz.CreateHibaKeys(t, dir)
	credz.SetupUser(t, dut, username)

	// Set only public key authentication for our test.
	credz.RotateAuthenticationTypes(t, dut, []cpb.AuthenticationType{
		cpb.AuthenticationType_AUTHENTICATION_TYPE_PUBKEY,
	})

	// Setup hiba for authorized principals command.
	credz.RotateAuthorizedPrincipalCheck(t, dut, cpb.AuthorizedPrincipalCheckRequest_TOOL_HIBA_DEFAULT)

	t.Run("auth should fail hiba host certificate not present", func(t *testing.T) {
		var startingRejectCounter uint64
		if !deviations.SSHServerCountersUnsupported(dut) {
			startingRejectCounter, _ = credz.GetRejectTelemetry(t, dut)
		}

		// Verify ssh with hiba fails as expected.
		startTime := time.Now()
		for {
			_, err := credz.SSHWithCertificate(t, target, username, fmt.Sprintf("%s/users", dir))
			if err != nil {
				t.Logf("Dialing ssh failed as expected.")
				break
			}
			if uint64(time.Since(startTime).Seconds()) > maxSSHRetryTime {
				t.Fatalf("Exceeded maxSSHRetryTime, dialing ssh succeeded, but we expected to fail.")
			}
			t.Logf("Dialing ssh succeeded but expected to fail, retrying ...")
			time.Sleep(5 * time.Second)
		}

		if !deviations.SSHServerCountersUnsupported(dut) {
			endingRejectCounter, _ := credz.GetRejectTelemetry(t, dut)
			if endingRejectCounter <= startingRejectCounter {
				t.Fatalf("SSH server reject counter did not increment after unsuccessful login. startCounter: %v, endCounter: %v", startingRejectCounter, endingRejectCounter)
			}
		}
	})

	t.Run("auth should succeed ssh public key authorized for user with hiba granted certificate", func(t *testing.T) {
		// Push host key/certificate to the dut.
		credz.RotateAuthenticationArtifacts(t,
			dut,
			fmt.Sprintf("%s/hosts", dir),
			fmt.Sprintf("%s/hosts", dir),
			hostCertificateVersion,
			uint64(hostCertificateCreatedOn),
		)

		// Setup trusted user ca on the dut.
		credz.RotateTrustedUserCA(t, dut, dir)

		var startingAcceptCounter, startingLastAcceptTime uint64
		if !deviations.SSHServerCountersUnsupported(dut) {
			startingAcceptCounter, startingLastAcceptTime = credz.GetAcceptTelemetry(t, dut)
		}

		startTime := time.Now()
		for {
			_, err := credz.SSHWithCertificate(t, target, username, fmt.Sprintf("%s/users", dir))
			if err == nil {
				t.Logf("Dialing ssh succeeded as expected.")
				break
			}
			if uint64(time.Since(startTime).Seconds()) > maxSSHRetryTime {
				t.Fatalf("Exceeded maxSSHRetryTime, dialing ssh failed, but we expected to succeed, error: %s", err)
			}
			t.Logf("Dialing ssh failed, retrying ...")
			time.Sleep(5 * time.Second)
		}

		// Verify ssh counters.
		if !deviations.SSHServerCountersUnsupported(dut) {
			endingAcceptCounter, endingLastAcceptTime := credz.GetAcceptTelemetry(t, dut)
			if endingAcceptCounter <= startingAcceptCounter {
				t.Fatalf("SSH server accept counter did not increment after successful login. startCounter: %v, endCounter: %v", startingAcceptCounter, endingAcceptCounter)
			}
			if startingLastAcceptTime == endingLastAcceptTime {
				t.Fatalf("SSH server accept last timestamp did not update after successful login. Timestamp: %v", endingLastAcceptTime)
			}
		}

		// Verify host certificate telemetry.
		sshServer := gnmi.Get(t, dut, gnmi.OC().System().SshServer().State())
		gotHostCertificateVersion := sshServer.GetActiveHostCertificateVersion()
		if !cmp.Equal(gotHostCertificateVersion, hostCertificateVersion) {
			t.Fatalf(
				"Telemetry reports host certificate version is not correct\n\tgot: %s\n\twant: %s",
				gotHostCertificateVersion, hostCertificateVersion,
			)
		}
		gotHostCertificateCreatedOn := sshServer.GetActiveHostCertificateCreatedOn()
		if !cmp.Equal(time.Unix(0, int64(gotHostCertificateCreatedOn)), time.Unix(hostCertificateCreatedOn, 0)) {
			t.Fatalf(
				"Telemetry reports host certificate created on is not correct\n\tgot: %d\n\twant: %d",
				gotHostCertificateCreatedOn, hostCertificateCreatedOn,
			)
		}
	})

	t.Cleanup(func() {
		// Cleanup to remove previous policy which only allowed key auth to make sure we don't leave dut in a
		// state where we can't reset config for further tests.
		credz.RotateAuthenticationTypes(t, dut, []cpb.AuthenticationType{
			cpb.AuthenticationType_AUTHENTICATION_TYPE_PASSWORD,
			cpb.AuthenticationType_AUTHENTICATION_TYPE_PUBKEY,
			cpb.AuthenticationType_AUTHENTICATION_TYPE_KBDINTERACTIVE,
		})

		// Remove user ca so subsequent fail cases work.
		credz.RotateTrustedUserCA(t, dut, "")

		// Clear hiba for authorized principals command.
		credz.RotateAuthorizedPrincipalCheck(t, dut, cpb.AuthorizedPrincipalCheckRequest_TOOL_UNSPECIFIED)

		// Remove host artifacts from the dut.
		credz.RotateAuthenticationArtifacts(t, dut, "", "", "", 0)
	})
}
