package ssh_public_key_authentication

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnsi/credentialz"
	tpb "github.com/openconfig/kne/proto/topo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"golang.org/x/crypto/ssh"
)

const (
	username                    = "testuser"
	password                    = "i$V5^6IhD*tZ#eg1G@v3xdVZrQwj"
	userPrivateKeyFilename      = "id_ed25519"
	userPublicKeyFilename       = "id_ed25519.pub"
	authorizedKeysListVersion   = "v1.0"
	authorizedKeysListCreatedOn = 1705962293
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func sendCredentialsRequest(t *testing.T, dut *ondatra.DUTDevice, request *credentialz.RotateAccountCredentialsRequest) {
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
		if _, err = gnmiClient.Set(context.Background(), SetRequest); err != nil {
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

func sshWithKey(t *testing.T, addr string) error {
	privateKeyContents, err := os.ReadFile(userPrivateKeyFilename)
	if err != nil {
		t.Fatalf("failed reading private key contents, error: %s", err)
	}

	signer, err := ssh.ParsePrivateKey(privateKeyContents)
	if err != nil {
		t.Fatalf("failed parsing private key, error: %s", err)
	}

	_, err = ssh.Dial(
		"tcp",
		fmt.Sprintf("%s:22", addr),
		&ssh.ClientConfig{
			User: username,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(signer),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
	)

	return err
}

func assertSSHAuthFails(t *testing.T, _ *ondatra.DUTDevice, addr string) {
	err := sshWithKey(t, addr)
	if err == nil {
		t.Fatalf("dialing ssh succeeded, but we expected to fail")
	}
}

func assertAuthSucceeds(t *testing.T, dut *ondatra.DUTDevice, addr string) {
	publicKeyContents, err := os.ReadFile(userPublicKeyFilename)
	if err != nil {
		t.Fatalf("failed reading private key contents, error: %s", err)
	}

	request := &credentialz.RotateAccountCredentialsRequest{
		Request: &credentialz.RotateAccountCredentialsRequest_Credential{
			Credential: &credentialz.AuthorizedKeysRequest{
				Credentials: []*credentialz.AccountCredentials{
					{
						Account: username,
						AuthorizedKeys: []*credentialz.AccountCredentials_AuthorizedKey{
							{
								AuthorizedKey: publicKeyContents,
								KeyType:       credentialz.KeyType_KEY_TYPE_ED25519,
								Description:   "credentialz-4: ssh public key authentication",
							},
						},
						Version:   authorizedKeysListVersion,
						CreatedOn: authorizedKeysListCreatedOn,
					},
				},
			},
		},
	}

	sendCredentialsRequest(t, dut, request)

	startingAcceptCounter, startingLastAcceptTime := getAcceptTelemetry(t, dut)

	err = sshWithKey(t, addr)
	if err != nil {
		t.Fatalf("dialing ssh failed, but we expected to succeed")
	}

	endingAcceptCounter, endingLastAcceptTime := getAcceptTelemetry(t, dut)

	if startingAcceptCounter-endingAcceptCounter < 1 {
		t.Log("would check accept counter incremented after successful auth")
	}

	if startingLastAcceptTime == endingLastAcceptTime {
		t.Log("would compare last accept times to make sure timestamp has been updated after successful auth")
	}

	assertTelemetry(t, dut)
}

func getAcceptTelemetry(t *testing.T, dut *ondatra.DUTDevice) (uint64, uint64) {
	t.Log("skipping checking ssh server telemetry until ondatra is unpinned from old gnsi commit and we have model regenerated")

	// would return accept counter and last accept timestamp
	return 0, 0
}

func assertTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	userState := gnmi.Get(t, dut, gnmi.OC().System().Aaa().Authentication().User(username).State())

	actualAuthorizedKeysListVersion := userState.GetAuthorizedKeysListVersion()

	if actualAuthorizedKeysListVersion != authorizedKeysListVersion {
		t.Fatalf(
			"telemetry reports authorized keys list version is not correct\n\tgot: %s\n\twant: %s",
			actualAuthorizedKeysListVersion, authorizedKeysListVersion,
		)
	}

	actualAuthorizedKeysListCreatedOn := userState.GetAuthorizedKeysListCreatedOn()

	if time.Unix(int64(authorizedKeysListCreatedOn), 0).Compare(time.Unix(0, int64(actualAuthorizedKeysListCreatedOn))) != 0 {
		t.Fatalf(
			"telemetry reports authorized keys list version on is not correct\n\tgot: %d\n\twant: %d",
			actualAuthorizedKeysListCreatedOn, authorizedKeysListCreatedOn,
		)
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

	testCases := []struct {
		name  string
		testF func(t *testing.T, dut *ondatra.DUTDevice, addr string)
	}{
		{
			name:  "auth should fail ssh public key not authorized for user",
			testF: assertSSHAuthFails,
		},
		{
			name:  "auth should succeed ssh public key authorized for user",
			testF: assertAuthSucceeds,
		},
	}

	t.Cleanup(func() {
		// cleanup to user authorized key after test
		request := &credentialz.RotateAccountCredentialsRequest{
			Request: &credentialz.RotateAccountCredentialsRequest_Credential{
				Credential: &credentialz.AuthorizedKeysRequest{
					Credentials: []*credentialz.AccountCredentials{
						{
							Account:        username,
							AuthorizedKeys: []*credentialz.AccountCredentials_AuthorizedKey{},
							Version:        "v1",
							CreatedOn:      uint64(time.Now().Unix()),
						},
					},
				},
			},
		}

		sendCredentialsRequest(t, dut, request)
	})

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.testF(t, dut, addr)
		})
	}
}
