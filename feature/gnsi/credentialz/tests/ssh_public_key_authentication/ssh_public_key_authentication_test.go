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
	"context"
	"os"
	"testing"
	"time"

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

var authorizedKeysListCreatedOn int64

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCredentialz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	target := credz.GetDutTarget(t, dut)
	authorizedKeysListCreatedOn = time.Now().Unix()

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
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		_, err = credz.SSHWithKey(ctx, t, dut, target, username, dir)
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
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		sshClient, err := credz.SSHWithKey(ctx, t, dut, target, username, dir)
		if err != nil {
			t.Fatalf("Dialing ssh failed, but we expected to succeed. error: %v", err)
		}
		defer sshClient.Close()
		t.Logf("SSH session established")

		res, err := sshClient.RunCommand(ctx, "show version")
		if err != nil {
			t.Fatalf("CombinedOutput failed, err: %v", err)
		}
		t.Logf("SSH session output: %s", res)

		sshClient.Close()
		time.Sleep(2 * time.Second)
		// Verify ssh counters.
		if !deviations.SSHServerCountersUnsupported(dut) {
			endingAcceptCounter, endingLastAcceptTime := credz.GetAcceptTelemetry(t, dut)
			if endingAcceptCounter <= startingAcceptCounter {
				t.Errorf("SSH server accept counter did not increment after successful login. startCounter: %v, endCounter: %v", startingAcceptCounter, endingAcceptCounter)
			}
			if startingLastAcceptTime == endingLastAcceptTime {
				t.Errorf("SSH server accept last timestamp did not update after successful login. Timestamp: %v", endingLastAcceptTime)
			}
		}

		// Verify authorized keys telemetry.
		userState := gnmi.Get(t, dut, gnmi.OC().System().Aaa().Authentication().User(username).State())
		gotAuthorizedKeysListVersion := userState.GetAuthorizedKeysListVersion()
		if got, want := gotAuthorizedKeysListVersion, authorizedKeysListVersion; got != want {
			t.Errorf("Telemetry reports authorized keys list version is not correct, got: %s, want: %s", got, want)
		}

		gotAuthorizedKeysListCreatedOn := int64(userState.GetAuthorizedKeysListCreatedOn())
		if got, want := gotAuthorizedKeysListCreatedOn, authorizedKeysListCreatedOn; got != want {
			t.Errorf("Telemetry reports authorized keys list created on is not correct, got: %d, want: %d", got, want)
		}

	})

	t.Cleanup(func() {
		// Cleanup user authorized key after test.
		credz.RotateAuthorizedKey(t, dut, "", username, "", uint64(authorizedKeysListCreatedOn))
	})
}
