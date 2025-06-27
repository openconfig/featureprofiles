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

package sshpublickeyauthentication_test

import (
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/security/credz"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
)

const (
	username                  = "testuser"
	authorizedKeysListVersion = "v1.0"
)

var (
	authorizedKeysListCreatedOn = time.Now().Unix()
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
		t.Fatalf("Creating temp dir, err: %s", err)
	}
	defer func(dir string) {
		err = os.RemoveAll(dir)
		if err != nil {
			t.Logf("Error removing temp directory, error: %s", err)
		}
	}(dir)

	credz.CreateSSHKeyPair(t, dir, username)
	credz.SetupUser(t, dut, username)

	t.Run("auth should fail ssh public key not authorized for user", func(t *testing.T) {
		_, err := credz.SSHWithKey(t, target, username, dir)
		if err == nil {
			t.Fatalf("Dialing ssh succeeded, but we expected to fail.")
		}
	})

	t.Run("auth should succeed ssh public key authorized for user", func(t *testing.T) {
		// Push authorized key to the dut.
		credz.RotateAuthorizedKey(t,
			dut,
			dir,
			username,
			authorizedKeysListVersion,
			uint64(authorizedKeysListCreatedOn))

		var startingAcceptCounter, startingLastAcceptTime uint64
		if !deviations.SSHServerCountersUnsupported(dut) {
			startingAcceptCounter, startingLastAcceptTime = credz.GetAcceptTelemetry(t, dut)
		}

		// Verify ssh with key succeeds.
		_, err := credz.SSHWithKey(t, target, username, dir)
		if err != nil {
			t.Fatalf("Dialing ssh failed, but we expected to succeed. error: %v", err)
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

		// Verify authorized keys telemetry.
		userState := gnmi.Get(t, dut, gnmi.OC().System().Aaa().Authentication().User(username).State())
		gotAuthorizedKeysListVersion := userState.GetAuthorizedKeysListVersion()
		if !cmp.Equal(gotAuthorizedKeysListVersion, authorizedKeysListVersion) {
			t.Fatalf(
				"Telemetry reports authorized keys list version is not correct\n\tgot: %s\n\twant: %s",
				gotAuthorizedKeysListVersion, authorizedKeysListVersion,
			)
		}
		gotAuthorizedKeysListCreatedOn := userState.GetAuthorizedKeysListCreatedOn()
		if !cmp.Equal(time.Unix(0, int64(gotAuthorizedKeysListCreatedOn)), time.Unix(authorizedKeysListCreatedOn, 0)) {
			t.Fatalf(
				"Telemetry reports authorized keys list version on is not correct\n\tgot: %d\n\twant: %d",
				gotAuthorizedKeysListCreatedOn, authorizedKeysListCreatedOn,
			)
		}
	})

	t.Cleanup(func() {
		// Cleanup user authorized key after test.
		credz.RotateAuthorizedKey(t, dut, "", username, "", 0)
	})
}
