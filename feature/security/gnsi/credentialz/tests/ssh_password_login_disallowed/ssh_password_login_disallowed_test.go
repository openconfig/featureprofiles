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

package ssh_password_login_disallowed

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnsi/acctz"
	"github.com/openconfig/gnsi/credentialz"
	tpb "github.com/openconfig/kne/proto/topo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"golang.org/x/crypto/ssh"
)

const (
	username               = "testuser"
	password               = "i$V5^6IhD*tZ#eg1G@v3xdVZrQwj"
	caPublicKeyFilename    = "ca.pub"
	userPrivateKeyFilename = "id_ed25519"
	userPublicCertFilename = "id_ed25519-cert.pub"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func sendHostParametersRequest(t *testing.T, dut *ondatra.DUTDevice, request *credentialz.RotateHostParametersRequest) {
	credzClient := dut.RawAPIs().GNSI(t).Credentialz()

	credzRotateClient, err := credzClient.RotateHostParameters(context.Background())
	if err != nil {
		t.Fatalf("failed fetching credentialz rotate host parameters client, error: %s", err)
	}

	err = credzRotateClient.Send(request)
	if err != nil {
		t.Fatalf("failed sending credentialz rotate host parameters request, error: %s", err)
	}

	_, err = credzRotateClient.Recv()
	if err != nil {
		t.Fatalf("failed receiving credentialz rotate host parameters request, error: %s", err)
	}

	err = credzRotateClient.Send(&credentialz.RotateHostParametersRequest{
		Request: &credentialz.RotateHostParametersRequest_Finalize{
			Finalize: request.GetFinalize(),
		},
	})
	if err != nil {
		t.Fatalf("failed sending credentialz rotate host parameters finalize request, error: %s", err)
	}
}

func setupTrustedUserCA(t *testing.T, dut *ondatra.DUTDevice) {
	keyContents, err := os.ReadFile(caPublicKeyFilename)
	if err != nil {
		t.Fatalf("failed reading ca public key contents, error: %s", err)
	}

	request := &credentialz.RotateHostParametersRequest{
		Request: &credentialz.RotateHostParametersRequest_SshCaPublicKey{
			SshCaPublicKey: &credentialz.CaPublicKeyRequest{
				SshCaPublicKeys: []*credentialz.PublicKey{
					{
						PublicKey:   keyContents,
						KeyType:     credentialz.KeyType_KEY_TYPE_ED25519,
						Description: "credentialz-2: ssh password login disallowed",
					},
				},
				Version:   "v1.0",
				CreatedOn: uint64(time.Now().Unix()),
			},
		},
	}

	sendHostParametersRequest(t, dut, request)
}

func createNativeRole(t testing.TB, dut *ondatra.DUTDevice, role string) {
	t.Helper()
	switch dut.Vendor() {
	case ondatra.NOKIA:
		roleData, err := json.Marshal([]any{
			map[string]any{
				"services": []string{"cli", "gnmi"},
			},
		})
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		userData, err := json.Marshal([]any{
			map[string]any{
				"password": password,
				"role":     []string{role},
			},
		})
		if err != nil {
			t.Fatalf("Error with json Marshal: %v", err)
		}

		SetRequest := &gpb.SetRequest{
			Prefix: &gpb.Path{
				Origin: "native",
			},
			Replace: []*gpb.Update{
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "aaa"},
							{Name: "authorization"},
							{Name: "role", Key: map[string]string{"rolename": role}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: roleData,
						},
					},
				},
				{
					Path: &gpb.Path{
						Elem: []*gpb.PathElem{
							{Name: "system"},
							{Name: "aaa"},
							{Name: "authentication"},
							{Name: "user", Key: map[string]string{"username": username}},
						},
					},
					Val: &gpb.TypedValue{
						Value: &gpb.TypedValue_JsonIetfVal{
							JsonIetfVal: userData,
						},
					},
				},
			},
		}
		gnmiClient := dut.RawAPIs().GNMI(t)
		if _, err := gnmiClient.Set(context.Background(), SetRequest); err != nil {
			t.Fatalf("Unexpected error configuring User: %v", err)
		}
	default:
		t.Fatalf("Unsupported vendor %s for deviation 'deviation_native_users'", dut.Vendor())
	}
}

func setupUser(t *testing.T, dut *ondatra.DUTDevice) {
	auth := &oc.System_Aaa_Authentication{}
	user := auth.GetOrCreateUser(username)

	if deviations.SetNativeUser(dut) {
		// probably all vendors need to handle this since the user should have a role attached to
		// it allowing us to login via ssh/console/whatever
		createNativeRole(t, dut, "credz-fp-test")
	} else {
		user.SetPassword(password)
	}

	gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().Config(), auth)

}

func setupAuthenticationTypes(t *testing.T, dut *ondatra.DUTDevice) {
	request := &credentialz.RotateHostParametersRequest{
		Request: &credentialz.RotateHostParametersRequest_AuthenticationAllowed{
			AuthenticationAllowed: &credentialz.AllowedAuthenticationRequest{
				AuthenticationTypes: []credentialz.AuthenticationType{
					credentialz.AuthenticationType_AUTHENTICATION_TYPE_PUBKEY,
				},
			},
		},
	}

	sendHostParametersRequest(t, dut, request)
}

func setupAuthorizedUsers(t *testing.T, dut *ondatra.DUTDevice) {
	request := &credentialz.RotateAccountCredentialsRequest{
		Request: &credentialz.RotateAccountCredentialsRequest_User{
			User: &credentialz.AuthorizedUsersRequest{
				Policies: []*credentialz.UserPolicy{
					{
						Account: username,
						AuthorizedPrincipals: &credentialz.UserPolicy_SshAuthorizedPrincipals{
							AuthorizedPrincipals: []*credentialz.UserPolicy_SshAuthorizedPrincipal{
								{
									AuthorizedUser: "my_principal",
								},
							},
						},
						Version:   "1.0",
						CreatedOn: uint64(time.Now().Unix()),
					},
				},
			},
		},
	}

	credzClient := dut.RawAPIs().GNSI(t).Credentialz()

	credzRotateClient, err := credzClient.RotateAccountCredentials(context.Background())
	if err != nil {
		t.Fatalf("failed fetching credentialz rotate account credentials client, error: %s", err)
	}

	err = credzRotateClient.Send(request)
	if err != nil {
		t.Fatalf("failed sending credentialz rotate account credentials request, error: %s", err)
	}

	_, err = credzRotateClient.Recv()
	if err != nil {
		t.Fatalf("failed receiving credentialz rotate account credentials request, error: %s", err)
	}

	err = credzRotateClient.Send(&credentialz.RotateAccountCredentialsRequest{
		Request: &credentialz.RotateAccountCredentialsRequest_Finalize{
			Finalize: request.GetFinalize(),
		},
	})
	if err != nil {
		t.Fatalf("failed sending credentialz rotate account credentials finalize request, error: %s", err)
	}
}

func assertSSHAuthFails(t *testing.T, dut *ondatra.DUTDevice, addr string) {
	startingRejectCounter, startingLastRejectTime := getRejectTelemetry(t, dut)

	_, err := ssh.Dial(
		"tcp",
		fmt.Sprintf("%s:22", addr),
		&ssh.ClientConfig{
			User: username,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
	)
	if err == nil {
		t.Fatalf("dialing ssh succeeded, but we expected to fail")
	}

	endingRejectCounter, endingLastRejectTime := getRejectTelemetry(t, dut)

	if startingRejectCounter-endingRejectCounter < 1 {
		t.Log("would check reject counter incremented after unsuccessful auth")
	}

	if startingLastRejectTime == endingLastRejectTime {
		t.Log("would compare last reject times to make sure timestamp has been updated after unsuccessful auth")
	}
}

func assertConsoleAuthSucceeds(t *testing.T, _ *ondatra.DUTDevice, _ string) {
	t.Skip("skipping console test, partner issue: 304734163")
}

func assertSSHCertificateAuthSucceeds(t *testing.T, dut *ondatra.DUTDevice, addr string) {
	privateKeyContents, err := os.ReadFile(userPrivateKeyFilename)
	if err != nil {
		t.Fatalf("failed reading private key contents, error: %s", err)
	}

	signer, err := ssh.ParsePrivateKey(privateKeyContents)
	if err != nil {
		t.Fatalf("failed parsing private key, error: %s", err)
	}

	certificateContents, err := os.ReadFile(userPublicCertFilename)
	if err != nil {
		t.Fatalf("failed reading certificate contents, error: %s", err)
	}

	certificate, _, _, _, err := ssh.ParseAuthorizedKey(certificateContents)
	if err != nil {
		t.Fatalf("failed parsing certificate contents, error: %s", err)
	}

	certificateSigner, err := ssh.NewCertSigner(certificate.(*ssh.Certificate), signer)
	if err != nil {
		t.Fatalf("failed creating certificate signer, error: %s", err)
	}

	startingAcceptCounter, startingLastAcceptTime := getAcceptTelemetry(t, dut)

	_, err = ssh.Dial(
		"tcp",
		fmt.Sprintf("%s:22", addr),
		&ssh.ClientConfig{
			User: username,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(certificateSigner),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
	)
	if err != nil {
		t.Fatalf("dialing ssh failed, but we expected to succeed, error: %s", err)
	}

	endingAcceptCounter, endingLastAcceptTime := getAcceptTelemetry(t, dut)

	if startingAcceptCounter-endingAcceptCounter < 1 {
		t.Log("would check accept counter incremented after successful auth")
	}

	if startingLastAcceptTime == endingLastAcceptTime {
		t.Log("would compare last accept times to make sure timestamp has been updated after successful auth")
	}

	t.Log("skipping accountz telemetry check -- not yet implemented")
	// assertSSHAuthAccounting(t, dut)
}

func getRejectTelemetry(t *testing.T, dut *ondatra.DUTDevice) (uint64, uint64) {
	t.Log("skipping checking ssh server telemetry until ondatra is unpinned from old gnsi commit and we have model regenerated")

	// would return reject counter and last reject timestamp
	return 0, 0
}

func getAcceptTelemetry(t *testing.T, dut *ondatra.DUTDevice) (uint64, uint64) {
	t.Log("skipping checking ssh server telemetry until ondatra is unpinned from old gnsi commit and we have model regenerated")

	// would return accept counter and last reject timestamp
	return 0, 0
}

func assertSSHAuthAccounting(t *testing.T, dut *ondatra.DUTDevice) {
	acctzClient := dut.RawAPIs().GNSI(t).Acctz()

	acctzSubClient, err := acctzClient.RecordSubscribe(context.Background())
	if err != nil {
		t.Fatalf("failed getting accountz record subscribe client, error: %s", err)
	}

	currentTime := time.Now()
	recordStarTime := currentTime.Add(-time.Second * 10)

	err = acctzSubClient.Send(&acctz.RecordRequest{
		Timestamp: &timestamp.Timestamp{
			Seconds: recordStarTime.UnixNano(),
		},
	})
	if err != nil {
		t.Fatalf("failed sending accountz record request, error: %s", err)
	}

	acctzResponse, err := acctzSubClient.Recv()
	if err != nil {
		t.Fatalf("failed receiving from accountz record subscribe client, error: %s", err)
	}

	t.Log("check for our principal (my_principal) in here:", acctzResponse.GetAuthen())
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
	setupTrustedUserCA(t, dut)
	setupAuthenticationTypes(t, dut)
	setupAuthorizedUsers(t, dut)

	// brief sleep for ssh daemons to restart if necessary (after updating allowed auth types)
	time.Sleep(10 * time.Second)

	testCases := []struct {
		name  string
		testF func(t *testing.T, dut *ondatra.DUTDevice, addr string)
	}{
		{
			name:  "auth should fail ssh password authentication disallowed",
			testF: assertSSHAuthFails,
		},
		{
			name:  "auth should succeed console password authentication allowed",
			testF: assertConsoleAuthSucceeds,
		},
		{
			name:  "auth should succeed ssh certificate authentication allowed",
			testF: assertSSHCertificateAuthSucceeds,
		},
	}

	t.Cleanup(func() {
		// cleanup to remove policy only allowing key auth to make sure we dont leave dut in a
		// state where you cant reset config and such for further tests
		request := &credentialz.RotateHostParametersRequest{
			Request: &credentialz.RotateHostParametersRequest_AuthenticationAllowed{
				AuthenticationAllowed: &credentialz.AllowedAuthenticationRequest{
					AuthenticationTypes: []credentialz.AuthenticationType{
						credentialz.AuthenticationType_AUTHENTICATION_TYPE_PASSWORD,
						credentialz.AuthenticationType_AUTHENTICATION_TYPE_PUBKEY,
						credentialz.AuthenticationType_AUTHENTICATION_TYPE_KBDINTERACTIVE,
					},
				},
			},
		}

		sendHostParametersRequest(t, dut, request)
	})

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.testF(t, dut, addr)
		})
	}
}
