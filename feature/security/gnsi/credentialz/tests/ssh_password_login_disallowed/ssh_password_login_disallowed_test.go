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

package sshpasswordlogindisallowed

import (
	"context"
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/credz"
	acctzpb "github.com/openconfig/gnsi/acctz"
	cpb "github.com/openconfig/gnsi/credentialz"
	"github.com/openconfig/ondatra"
)

const (
	username      = "testuser"
	userPrincipal = "my_principal"
	command       = "show version"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCredentialz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	target := credz.GetDutTarget(t, dut)
	recordStartTime := timestamppb.New(time.Now())

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

	// Create ssh keys/certificates for CA & testuser.
	credz.CreateSSHKeyPair(t, dir, "ca")
	credz.CreateSSHKeyPair(t, dir, username)
	credz.CreateUserCertificate(t, dir, userPrincipal)

	// Setup user and password.
	credz.SetupUser(t, dut, username)
	password := credz.GeneratePassword()
	credz.RotateUserPassword(t, dut, username, password, "v1.0", uint64(time.Now().Unix()))

	credz.RotateTrustedUserCA(t, dut, dir)
	credz.RotateAuthenticationTypes(t, dut, []cpb.AuthenticationType{
		cpb.AuthenticationType_AUTHENTICATION_TYPE_PUBKEY,
	})
	credz.RotateAuthorizedPrincipal(t, dut, username, userPrincipal)

	t.Run("auth should fail ssh password authentication disallowed", func(t *testing.T) {
		var startingRejectCounter, startingLastRejectTime uint64
		if !deviations.SSHServerCountersUnsupported(dut) {
			startingRejectCounter, startingLastRejectTime = credz.GetRejectTelemetry(t, dut)
		}

		// Verify ssh with password fails as expected.
		_, err := credz.SSHWithPassword(target, username, password)
		if err == nil {
			t.Fatalf("Dialing ssh succeeded, but we expected to fail.")
		}

		// Verify ssh counters.
		if !deviations.SSHServerCountersUnsupported(dut) {
			endingRejectCounter, endingLastRejectTime := credz.GetRejectTelemetry(t, dut)
			if endingRejectCounter <= startingRejectCounter {
				t.Fatalf("SSH server reject counter did not increment after unsuccessful login. startCounter: %v, endCounter: %v", startingRejectCounter, endingRejectCounter)
			}
			if startingLastRejectTime == endingLastRejectTime {
				t.Fatalf("SSH server reject last timestamp did not update after unsuccessful login. Timestamp: %v", endingLastRejectTime)
			}
		}
	})

	t.Run("auth should succeed ssh certificate authentication allowed", func(t *testing.T) {
		var startingAcceptCounter, startingLastAcceptTime uint64
		if !deviations.SSHServerCountersUnsupported(dut) {
			startingAcceptCounter, startingLastAcceptTime = credz.GetAcceptTelemetry(t, dut)
		}

		// Verify ssh with certificate succeeds.
		conn, err := credz.SSHWithCertificate(t, target, username, dir)
		if err != nil {
			t.Fatalf("Dialing ssh failed, but we expected to succeed, error: %s", err)
		}
		defer conn.Close()

		// Send command for accounting.
		sess, err := conn.NewSession()
		if err != nil {
			t.Fatalf("Failed creating ssh session, error: %s", err)
		}
		defer sess.Close()
		sess.Run(command)

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

		// Verify accounting record.
		acctzClient := dut.RawAPIs().GNSI(t).AcctzStream()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		acctzSubClient, err := acctzClient.RecordSubscribe(ctx, &acctzpb.RecordRequest{Timestamp: recordStartTime})
		if err != nil {
			t.Fatalf("Failed sending accountz record request, error: %s", err)
		}
		defer acctzSubClient.CloseSend()

		var foundRecord bool
		for {
			acctzResponse, err := acctzSubClient.Recv()
			if err != nil {
				if status.Code(err) == codes.DeadlineExceeded {
					t.Log("Done receiving records...")
					break
				}
				t.Fatalf(
					"Failed receiving from accountz record subscribe client, "+
						"this could mean we didn't find the user identity and eventually timed "+
						"out with no more records to review, or another server error. Error: %s",
					err,
				)
			}

			// Skip non-ssh records.
			if acctzResponse.GetCmdService() == nil {
				continue
			}

			reportedIdentity := acctzResponse.GetSessionInfo().GetUser().GetIdentity()
			if reportedIdentity == username {
				t.Logf("Found Record: %s", credz.PrettyPrint(acctzResponse))
				foundRecord = true
				break
			}
		}
		if !foundRecord {
			t.Fatalf("Did not find accounting record for %s", username)
		}
	})

	t.Cleanup(func() {
		// Cleanup to remove previous policy which only allowed key auth to make sure we don't
		// leave dut in a state where we can't reset config for further tests.
		credz.RotateAuthenticationTypes(t, dut, []cpb.AuthenticationType{
			cpb.AuthenticationType_AUTHENTICATION_TYPE_PASSWORD,
			cpb.AuthenticationType_AUTHENTICATION_TYPE_PUBKEY,
			cpb.AuthenticationType_AUTHENTICATION_TYPE_KBDINTERACTIVE,
		})
	})
}
