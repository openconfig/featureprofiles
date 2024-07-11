package ssh_public_key_authentication

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
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
	username                    = "testuser"
	password                    = "i$V5^6IhD*tZ#eg1G@v3xdVZrQwj"
	userPrivateKeyFilename      = "user"
	userPublicKeyFilename       = "user.pub"
	authorizedKeysListVersion   = "v1.0"
	authorizedKeysListCreatedOn = 1705962293
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func prepareUserKey(t *testing.T, dir string) {
	userCmd := exec.Command(
		"ssh-keygen",
		"-t", "ed25519",
		"-f", userPrivateKeyFilename,
		"-C", "featureprofile@openconfig",
		"-q", "-N", "", // quiet, empty passphrase
	)
	userCmd.Dir = dir

	err := userCmd.Run()
	if err != nil {
		t.Fatalf("failed generating user key pair, error: %s", err)
	}
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
		t.Fatalf("failed receiving credentialz rotate account credentials response, error: %s", err)
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

func setupUser(t *testing.T, dut *ondatra.DUTDevice) {
	auth := &oc.System_Aaa_Authentication{}
	auth.GetOrCreateUser(username)
	gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().Config(), auth)
}

func sshWithKey(t *testing.T, addr, dir string) error {
	privateKeyContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, userPrivateKeyFilename))
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

func assertSSHAuthFails(t *testing.T, _ *ondatra.DUTDevice, addr, dir string) {
	err := sshWithKey(t, addr, dir)
	if err == nil {
		t.Fatalf("dialing ssh succeeded, but we expected to fail")
	}
}

func assertAuthSucceeds(t *testing.T, dut *ondatra.DUTDevice, addr, dir string) {
	publicKeyContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, userPublicKeyFilename))
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

	var startingAcceptCounter, startingLastAcceptTime uint64

	if !deviations.SSHServerCountersUnsupported(dut) {
		startingAcceptCounter, startingLastAcceptTime = getAcceptTelemetry(t, dut)
	}

	err = sshWithKey(t, addr, dir)
	if err != nil {
		t.Fatalf("dialing ssh failed, but we expected to succeed")
	}

	if !deviations.SSHServerCountersUnsupported(dut) {
		endingAcceptCounter, endingLastAcceptTime := getAcceptTelemetry(t, dut)

		if startingAcceptCounter-endingAcceptCounter < 1 {
			t.Fatal("ssh server accept counter did not increment after successful login")
		}

		if startingLastAcceptTime == endingLastAcceptTime {
			t.Fatal("ssh server accept last timestamp did not update after successful login")
		}
	}

	assertTelemetry(t, dut)
}

func getAcceptTelemetry(t *testing.T, dut *ondatra.DUTDevice) (uint64, uint64) {
	sshCounters := gnmi.Get(t, dut, gnmi.OC().System().SshServer().Counters().State())

	return sshCounters.GetAccessAccepts(), sshCounters.GetLastAccessAccept()
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

	prepareUserKey(t, dir)

	addr := getDutAddr(t, dut)

	setupUser(t, dut)

	testCases := []struct {
		name  string
		testF func(t *testing.T, dut *ondatra.DUTDevice, addr, dir string)
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
			tt.testF(t, dut, addr, dir)
		})
	}
}
