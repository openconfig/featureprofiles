// Copyright 2022 Google LLC
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

package password_console_login

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnsi/credentialz"
	tpb "github.com/openconfig/kne/proto/topo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"golang.org/x/crypto/ssh"
)

const (
	username          = "testuser"
	password          = "i$V5^6IhD*tZ#eg1G@v3xdVZrQwj"
	passwordVersion   = "v1.0"
	passwordCreatedOn = 1705962293
	// think this should work for all vendors contributing to fp
	command = "show version"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func setupUser(t *testing.T, dut *ondatra.DUTDevice) {
	auth := &oc.System_Aaa_Authentication{}
	user := auth.GetOrCreateUser(username)
	user.SetRole(oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN)
	gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().Config(), auth)
}

func rotateUserCredential(t *testing.T, dut *ondatra.DUTDevice) {
	credzClient := dut.RawAPIs().GNSI(t).Credentialz()

	credzRotateClient, err := credzClient.RotateAccountCredentials(context.Background())
	if err != nil {
		t.Fatalf("failed fetching credentialz rotate account credentials client, error: %s", err)
	}

	req := &credentialz.RotateAccountCredentialsRequest{
		Request: &credentialz.RotateAccountCredentialsRequest_Password{
			Password: &credentialz.PasswordRequest{
				Accounts: []*credentialz.PasswordRequest_Account{
					{
						Account: username,
						Password: &credentialz.PasswordRequest_Password{
							Value: &credentialz.PasswordRequest_Password_Plaintext{
								Plaintext: password,
							},
						},
						Version:   passwordVersion,
						CreatedOn: passwordCreatedOn,
					},
				},
			},
		},
	}

	err = credzRotateClient.Send(req)
	if err != nil {
		t.Fatalf("failed sending credentialz rotate account credentials request, error: %s", err)
	}

	resp, err := credzRotateClient.Recv()
	if err != nil {
		t.Fatalf("failed receiving credentialz rotate request, error: %s", err)
	}

	t.Logf("got credentialz rotate request response: %s", resp)

	err = credzRotateClient.Send(&credentialz.RotateAccountCredentialsRequest{
		Request: &credentialz.RotateAccountCredentialsRequest_Finalize{
			Finalize: req.GetFinalize(),
		},
	})
	if err != nil {
		t.Fatalf("failed sending credentialz rotate account credentials finalize request, error: %s", err)
	}
}

func assertTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	userState := gnmi.Get(t, dut, gnmi.OC().System().Aaa().Authentication().User(username).State())

	actualPasswordVersion := userState.GetPasswordVersion()

	if actualPasswordVersion != passwordVersion {
		t.Fatalf(
			"telemetry reports password version is not correctn\tgot: %s\n\twant: %s",
			actualPasswordVersion, passwordVersion,
		)
	}

	actualPasswordCreatedOn := userState.GetPasswordCreatedOn()

	if time.Unix(int64(passwordCreatedOn), 0).Compare(time.Unix(0, int64(actualPasswordCreatedOn))) != 0 {
		t.Fatalf(
			"telemetry reports password created on is not correct\n\tgot: %d\n\twant: %d",
			actualPasswordCreatedOn, passwordCreatedOn,
		)
	}
}

func tryLogin(t *testing.T, addr, loginUser, loginPassword string, expectFail bool) {
	client, err := ssh.Dial(
		"tcp",
		fmt.Sprintf("%s:22", addr),
		&ssh.ClientConfig{
			User: loginUser,
			Auth: []ssh.AuthMethod{
				ssh.Password(loginPassword),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
	)
	if expectFail {
		if err == nil {
			t.Fatalf("dialing ssh succeeded, but we expected to fail")
		}

		return
	}
	if err != nil {
		t.Fatalf("failed dialing ssh, error: %s", err)
	}

	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("failed spawning ssh session, error: %s", err)
	}

	var buf bytes.Buffer

	session.Stdout = &buf

	err = session.Run(command)
	if err != nil {
		t.Fatalf("failed sending return to ssh session, error: %s", err)
	}

	if buf.Len() == 0 {
		t.Fatalf("no output received from command, were we not authenticated?")
	}
}

func getDutAddr(t *testing.T, dut *ondatra.DUTDevice) string {
	var serviceDUT interface {
		Service(string) (*tpb.Service, error)
	}

	err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &serviceDUT)
	if err != nil {
		t.Log("DUT does not support `Service` function, will attempt to use dut name field")

		return dut.Name()
	}

	dutSSHService, err := serviceDUT.Service("ssh")
	if err != nil {
		t.Fatal(err)
	}

	return dutSSHService.GetOutsideIp()
}

func TestCredentialz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	addr := getDutAddr(t, dut)

	setupUser(t, dut)
	rotateUserCredential(t, dut)

	// quick sleep to let things percolate
	time.Sleep(time.Second)

	assertTelemetry(t, dut)

	testCases := []struct {
		name       string
		loginUser  string
		loginPass  string
		expectFail bool
	}{
		{
			name:       "auth should succeed",
			loginUser:  username,
			loginPass:  password,
			expectFail: false,
		},
		{
			name:       "auth should fail bad username",
			loginUser:  "notadmin",
			loginPass:  password,
			expectFail: true,
		},
		{
			name:       "auth should fail bad password",
			loginUser:  username,
			loginPass:  "notthepassword",
			expectFail: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tryLogin(t, addr, tt.loginUser, tt.loginPass, tt.expectFail)
		})
	}
}
