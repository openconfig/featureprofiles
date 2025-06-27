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

package passwordconsolelogin_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/ondatra/gnmi"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/credz"
	"github.com/openconfig/ondatra"
)

const (
	username        = "testuser"
	passwordVersion = "v1.0"
)

var (
	passwordCreatedOn = time.Now().Unix()
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCredentialz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	target := credz.GetDutTarget(t, dut)

	// Setup test user and password.
	credz.SetupUser(t, dut, username)
	password := credz.GeneratePassword()
	credz.RotateUserPassword(t, dut, username, password, passwordVersion, uint64(passwordCreatedOn))

	testCases := []struct {
		name          string
		loginUser     string
		loginPassword string
		expectFail    bool
	}{
		{
			name:          "auth should succeed",
			loginUser:     username,
			loginPassword: password,
			expectFail:    false,
		},
		{
			name:          "auth should fail bad username",
			loginUser:     "notadmin",
			loginPassword: password,
			expectFail:    true,
		},
		{
			name:          "auth should fail bad password",
			loginUser:     username,
			loginPassword: "notthepassword",
			expectFail:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify ssh succeeds/fails based on expected result.
			client, err := credz.SSHWithPassword(target, tc.loginUser, tc.loginPassword)
			if tc.expectFail {
				if err == nil {
					t.Fatalf("Dialing ssh succeeded, but we expected to fail.")
				}
				return
			}
			if err != nil {
				t.Fatalf("Failed dialing ssh, error: %s", err)
			}
			defer client.Close()

			// Verify password telemetry.
			userState := gnmi.Get(t, dut, gnmi.OC().System().Aaa().Authentication().User(username).State())
			gotPasswordVersion := userState.GetPasswordVersion()
			if !cmp.Equal(gotPasswordVersion, passwordVersion) {
				t.Fatalf(
					"Telemetry reports password version is not correctn\tgot: %s\n\twant: %s",
					gotPasswordVersion, passwordVersion,
				)
			}
			gotPasswordCreatedOn := userState.GetPasswordCreatedOn()
			if !cmp.Equal(time.Unix(0, int64(gotPasswordCreatedOn)), time.Unix(passwordCreatedOn, 0)) {
				t.Fatalf(
					"Telemetry reports password created on is not correct\n\tgot: %d\n\twant: %d",
					gotPasswordCreatedOn, passwordCreatedOn,
				)
			}
		})
	}
}
