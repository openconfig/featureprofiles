package hiba_authentication

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
	username                 = "testuser"
	password                 = "i$V5^6IhD*tZ#eg1G@v3xdVZrQwj"
	userPrivateKeyFilename   = "testuser"
	userCertFilename         = "testuser-cert.pub"
	dutPrivateKeyFilename    = "dut"
	dutCertFilename          = "dut-cert.pub"
	caPublicKeyFilename      = "ca.pub"
	hostCertificateVersion   = "v1.0"
	hostCertificateCreatedOn = 1705962293
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
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

func setupDUT(t *testing.T, dut *ondatra.DUTDevice) {
	// set only pub key auth for our test
	setupAuthenticationTypes(t, dut)

	// set the authorized principals thing
	setupAuthorizedPrincipals(t, dut)
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

func setupAuthorizedPrincipals(t *testing.T, dut *ondatra.DUTDevice) {
	request := &credentialz.RotateHostParametersRequest{
		Request: &credentialz.RotateHostParametersRequest_AuthorizedPrincipalCheck{
			AuthorizedPrincipalCheck: &credentialz.AuthorizedPrincipalCheckRequest{
				Tool: credentialz.AuthorizedPrincipalCheckRequest_TOOL_HIBA_DEFAULT,
			},
		},
	}

	sendHostParametersRequest(t, dut, request)
}

func loadCertificate(t *testing.T, dut *ondatra.DUTDevice) {
	certificateContents, err := os.ReadFile(dutCertFilename)
	if err != nil {
		t.Fatalf("failed reading host signed certificate, error: %s", err)
	}

	privateKeyContents, err := os.ReadFile(dutPrivateKeyFilename)
	if err != nil {
		t.Fatalf("failed reading host signed certificate, error: %s", err)
	}

	credzClient := dut.RawAPIs().GNSI(t).Credentialz()

	credzRotateClient, err := credzClient.RotateHostParameters(context.Background())
	if err != nil {
		t.Fatalf("failed fetching credentialz rotate host parameters client, error: %s", err)
	}

	request := &credentialz.RotateHostParametersRequest{
		Request: &credentialz.RotateHostParametersRequest_ServerKeys{
			ServerKeys: &credentialz.ServerKeysRequest{
				AuthArtifacts: []*credentialz.ServerKeysRequest_AuthenticationArtifacts{
					{
						PrivateKey:  privateKeyContents,
						Certificate: certificateContents,
					},
				},
				Version:   hostCertificateVersion,
				CreatedOn: hostCertificateCreatedOn,
			},
		},
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
						Description: "credentialz-5: hiba authentication",
					},
				},
				Version:   "v1.0",
				CreatedOn: uint64(time.Now().Unix()),
			},
		},
	}

	sendHostParametersRequest(t, dut, request)
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

func sshWithKey(t *testing.T, addr string) error {
	// ensure files are 0600 so ssh doesnt barf, git only cares about +x bit so just enforcing
	// this lazily here
	err := os.Chmod(userPrivateKeyFilename, 0o600)
	if err != nil {
		t.Fatalf("failed ensuring user private key file permissions, error: %s", err)
	}

	err = os.Chmod(userCertFilename, 0o600)
	if err != nil {
		t.Fatalf("failed ensuring user cert file permissions, error: %s", err)
	}

	privateKeyContents, err := os.ReadFile(userPrivateKeyFilename)
	if err != nil {
		t.Fatalf("failed loading user private key, error: %s", err)
	}

	signer, err := ssh.ParsePrivateKey(privateKeyContents)
	if err != nil {
		t.Fatalf("failed parsing user private key, error: %s", err)
	}

	certificateContents, err := os.ReadFile(userCertFilename)
	if err != nil {
		t.Fatalf("failed loading user certificate, error: %s", err)
	}

	cert, _, _, _, err := ssh.ParseAuthorizedKey(certificateContents)
	if err != nil {
		t.Fatalf("failed parsing user certificate, error: %s", err)
	}

	certificateSigner, err := ssh.NewCertSigner(cert.(*ssh.Certificate), signer)
	if err != nil {
		t.Fatalf("failed creating certificate signer, error: %s", err)
	}

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

	return err
}

func assertSSHAuthFails(t *testing.T, dut *ondatra.DUTDevice, addr string) {
	err := sshWithKey(t, addr)
	if err == nil {
		t.Fatal("dialing ssh succeeded, but we expected to fail")
	}
}

func assertAuthSucceeds(t *testing.T, dut *ondatra.DUTDevice, addr string) {
	// set the hiba host cert and also private key so its the hiba one we setup before the test
	loadCertificate(t, dut)

	// set the trusted user ca
	setupTrustedUserCA(t, dut)

	// small sleep to let changes percolate
	time.Sleep(15 * time.Second)

	assertTelemetry(t, dut)

	var startingAcceptCounter, startingLastAcceptTime uint64

	if !deviations.SshServerCountersUnsupported(dut) {
		startingAcceptCounter, startingLastAcceptTime = getAcceptTelemetry(t, dut)
	}

	err := sshWithKey(t, addr)
	if err != nil {
		t.Fatalf("dialing ssh failed, but we expected to succeed, errror: %s", err)
	}

	if !deviations.SshServerCountersUnsupported(dut) {
		endingAcceptCounter, endingLastAcceptTime := getAcceptTelemetry(t, dut)

		if startingAcceptCounter-endingAcceptCounter < 1 {
			t.Fatal("ssh server accept counter did not increment after successful login")
		}

		if startingLastAcceptTime == endingLastAcceptTime {
			t.Fatal("ssh server accept last timestamp did not update after successful login")
		}
	}
}

func getAcceptTelemetry(t *testing.T, dut *ondatra.DUTDevice) (uint64, uint64) {
	sshCounters := gnmi.Get(t, dut, gnmi.OC().System().SshServer().Counters().State())

	return sshCounters.GetAccessAccepts(), sshCounters.GetLastAccessAccept()
}

func assertTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	sshServer := gnmi.Get(t, dut, gnmi.OC().System().SshServer().State())

	actualHostCertificateVersion := sshServer.GetActiveHostCertificateVersion()

	if actualHostCertificateVersion != hostCertificateVersion {
		t.Fatalf(
			"telemetry reports host certificate version is not correct\n\tgot: %s\n\twant: %s",
			actualHostCertificateVersion, hostCertificateVersion,
		)
	}

	actualHostCertificateCreatedOn := sshServer.GetActiveHostCertificateCreatedOn()

	if time.Unix(int64(hostCertificateCreatedOn), 0).Compare(time.Unix(0, int64(actualHostCertificateCreatedOn))) != 0 {
		t.Fatalf(
			"telemetry reports host certificate created on is not correct\n\tgot: %d\n\twant: %d",
			actualHostCertificateCreatedOn, hostCertificateCreatedOn,
		)
	}
}

func TestCredentialz(t *testing.T) {
	dut := ondatra.DUT(t, "dut")

	addr := getDutAddr(t, dut)

	setupUser(t, dut)

	setupDUT(t, dut)

	// quick sleep to let things percolate
	time.Sleep(time.Second)

	testCases := []struct {
		name  string
		testF func(t *testing.T, dut *ondatra.DUTDevice, addr string)
	}{
		{
			name:  "auth should fail hiba host certificate not present",
			testF: assertSSHAuthFails,
		},
		{
			name:  "auth should succeed ssh public key authorized for user with hiba granted certificate",
			testF: assertAuthSucceeds,
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

		// also to remove the user ca so subsequent fail cases work
		request = &credentialz.RotateHostParametersRequest{
			Request: &credentialz.RotateHostParametersRequest_SshCaPublicKey{
				SshCaPublicKey: &credentialz.CaPublicKeyRequest{
					SshCaPublicKeys: []*credentialz.PublicKey{},
					Version:         "0",
					CreatedOn:       uint64(time.Now().Unix()),
				},
			},
		}

		sendHostParametersRequest(t, dut, request)

		// annnd the host key just to clean up all the things
		credzClient := dut.RawAPIs().GNSI(t).Credentialz()

		credzRotateClient, err := credzClient.RotateHostParameters(context.Background())
		if err != nil {
			t.Fatalf("failed fetching credentialz rotate host parameters client, error: %s", err)
		}

		request = &credentialz.RotateHostParametersRequest{
			Request: &credentialz.RotateHostParametersRequest_ServerKeys{
				ServerKeys: &credentialz.ServerKeysRequest{
					AuthArtifacts: []*credentialz.ServerKeysRequest_AuthenticationArtifacts{},
				},
			},
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
	})

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.testF(t, dut, addr)
		})
	}
}
