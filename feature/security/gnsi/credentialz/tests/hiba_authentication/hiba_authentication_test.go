package hiba_authentication

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
	username                 = "testuser"
	userPrivateKeyFilename   = "users/testuser"
	userCertFilename         = "users/testuser-cert.pub"
	dutPrivateKeyFilename    = "hosts/dut"
	dutCertFilename          = "hosts/dut-cert.pub"
	caPublicKeyFilename      = "ca.pub"
	hostCertificateVersion   = "v1.0"
	hostCertificateCreatedOn = 1705962293
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func setupUser(t *testing.T, dut *ondatra.DUTDevice) {
	auth := &oc.System_Aaa_Authentication{}
	auth.GetOrCreateUser(username)
	gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().Config(), auth)
}

func setupDUT(t *testing.T, dut *ondatra.DUTDevice) {
	// set only pub key auth for our test
	setupAuthenticationTypes(t, dut)

	// setup hiba for authorized principals command
	setupAuthorizedPrincipalsCommand(t, dut)
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

	// brief sleep for finalize to get processed
	time.Sleep(time.Second)
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

func setupAuthorizedPrincipalsCommand(t *testing.T, dut *ondatra.DUTDevice) {
	request := &credentialz.RotateHostParametersRequest{
		Request: &credentialz.RotateHostParametersRequest_AuthorizedPrincipalCheck{
			AuthorizedPrincipalCheck: &credentialz.AuthorizedPrincipalCheckRequest{
				Tool: credentialz.AuthorizedPrincipalCheckRequest_TOOL_HIBA_DEFAULT,
			},
		},
	}

	sendHostParametersRequest(t, dut, request)
}

func loadCertificate(t *testing.T, dut *ondatra.DUTDevice, dir string) {
	certificateContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, dutCertFilename))
	if err != nil {
		t.Fatalf("failed reading host signed certificate, error: %s", err)
	}

	privateKeyContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, dutPrivateKeyFilename))
	if err != nil {
		t.Fatalf("failed reading host signed certificate, error: %s", err)
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

	sendHostParametersRequest(t, dut, request)
}

func setupTrustedUserCA(t *testing.T, dut *ondatra.DUTDevice, dir string) {
	keyContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, caPublicKeyFilename))
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

func sshWithKey(t *testing.T, addr, dir string) error {
	// ensure files are 0600 so ssh doesn't complain
	err := os.Chmod(fmt.Sprintf("%s/%s", dir, userPrivateKeyFilename), 0o600)
	if err != nil {
		t.Fatalf("failed ensuring user private key file permissions, error: %s", err)
	}

	err = os.Chmod(fmt.Sprintf("%s/%s", dir, userCertFilename), 0o600)
	if err != nil {
		t.Fatalf("failed ensuring user cert file permissions, error: %s", err)
	}

	privateKeyContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, userPrivateKeyFilename))
	if err != nil {
		t.Fatalf("failed loading user private key, error: %s", err)
	}

	signer, err := ssh.ParsePrivateKey(privateKeyContents)
	if err != nil {
		t.Fatalf("failed parsing user private key, error: %s", err)
	}

	certificateContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, userCertFilename))
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

func assertSSHAuthFails(t *testing.T, dut *ondatra.DUTDevice, addr, dir string) {
	var startingRejectCounter uint64

	if !deviations.SSHServerCountersUnsupported(dut) {
		startingRejectCounter = getRejectTelemetry(t, dut)
	}

	err := sshWithKey(t, addr, dir)
	if err == nil {
		t.Fatalf("dialing ssh succeeded, but we expected to fail")
	}

	if !deviations.SSHServerCountersUnsupported(dut) {
		endingRejectCounter := getRejectTelemetry(t, dut)
		if endingRejectCounter <= startingRejectCounter {
			t.Fatalf("ssh server reject counter did not increment after unsuccessful login")
		}
	}
}

func assertAuthSucceeds(t *testing.T, dut *ondatra.DUTDevice, addr, dir string) {
	// set the hiba host cert and also private key so it's the hiba one we setup before the test
	loadCertificate(t, dut, dir)

	// set the trusted user ca
	setupTrustedUserCA(t, dut, dir)

	// small sleep to let changes percolate
	time.Sleep(15 * time.Second)

	assertTelemetry(t, dut)

	var startingAcceptCounter, startingLastAcceptTime uint64

	if !deviations.SSHServerCountersUnsupported(dut) {
		startingAcceptCounter, startingLastAcceptTime = getAcceptTelemetry(t, dut)
	}

	err := sshWithKey(t, addr, dir)
	if err != nil {
		t.Fatalf("dialing ssh failed, but we expected to succeed, errror: %s", err)
	}

	if !deviations.SSHServerCountersUnsupported(dut) {
		endingAcceptCounter, endingLastAcceptTime := getAcceptTelemetry(t, dut)

		if endingAcceptCounter <= startingAcceptCounter {
			t.Fatalf("ssh server accept counter did not increment after successful login")
		}

		if startingLastAcceptTime == endingLastAcceptTime {
			t.Fatalf("ssh server accept last timestamp did not update after successful login")
		}
	}
}

func getRejectTelemetry(t *testing.T, dut *ondatra.DUTDevice) uint64 {
	sshCounters := gnmi.Get(t, dut, gnmi.OC().System().SshServer().Counters().State())
	return sshCounters.GetAccessRejects()
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

func prepareHibaKeysCopy(t *testing.T, dir string) {
	keyFiles := []string{
		"ca",
		caPublicKeyFilename,
		dutPrivateKeyFilename,
		"hosts/dut.pub",
		dutCertFilename,
		userPrivateKeyFilename,
		"users/testuser.pub",
		userCertFilename,
	}

	err := os.Mkdir(fmt.Sprintf("%s/hosts", dir), 0o700)
	if err != nil {
		t.Fatalf("failed ensuring hosts dir in temp dir, error: %s", err)
	}

	err = os.Mkdir(fmt.Sprintf("%s/users", dir), 0o700)
	if err != nil {
		t.Fatalf("failed ensuring users dir in temp dir, error: %s", err)
	}

	for _, keyFile := range keyFiles {
		var input []byte

		input, err = os.ReadFile(keyFile)
		if err != nil {
			fmt.Println(err)
			return
		}

		err = os.WriteFile(fmt.Sprintf("%s/%s", dir, keyFile), input, 0o600)
		if err != nil {
			t.Fatalf("failed copying key file %s to temp test dir, error: %s", keyFile, err)
		}
	}
}

func prepareHibaKeysGen(t *testing.T, dir string) {
	caCmd := exec.Command(
		"hiba-ca.sh",
		"-c",
		"-d", dir, //output to the tempdir
		"--",           // pass the rest to ssh-keygen
		"-q", "-N", "", // quiet, empty passphrase

	)
	caCmd.Dir = dir

	err := caCmd.Run()
	if err != nil {
		t.Fatalf("failed generating ca key pair, error: %s", err)
	}

	userKeyCmd := exec.Command(
		"hiba-ca.sh",
		"-c",
		"-d", dir, //output to the tempdir
		"-u", "-I", "testuser",
		"--",           // pass the rest to ssh-keygen
		"-q", "-N", "", // quiet, empty passphrase

	)
	userKeyCmd.Dir = dir

	err = userKeyCmd.Run()
	if err != nil {
		t.Fatalf("failed generating user key pair, error: %s", err)
	}

	dutKeyCmd := exec.Command(
		"hiba-ca.sh",
		"-c",
		"-d", dir, //output to the tempdir
		"-h", "-I", "dut",
		"--",           // pass the rest to ssh-keygen
		"-q", "-N", "", // quiet, empty passphrase

	)
	dutKeyCmd.Dir = dir

	err = dutKeyCmd.Run()
	if err != nil {
		t.Fatalf("failed generating dut key pair, error: %s", err)
	}

	prodIdentityCmd := exec.Command(
		"hiba-gen",
		"-i",
		"-f", fmt.Sprintf("%s/policy/identities/prod", dir),
		"domain", "example.com",
	)
	prodIdentityCmd.Dir = dir

	err = prodIdentityCmd.Run()
	if err != nil {
		t.Fatalf("failed creating prod identity, error: %s", err)
	}

	shellGrantCmd := exec.Command(
		"hiba-gen",
		"-f", fmt.Sprintf("%s/policy/grants/shell", dir),
		"domain", "example.com",
	)
	shellGrantCmd.Dir = dir

	err = shellGrantCmd.Run()
	if err != nil {
		t.Fatalf("failed creating shell grant, error: %s", err)
	}

	grantShellToUserCmd := exec.Command(
		"hiba-ca.sh",
		"-d", dir, //output to the tempdir
		"-p",
		"-I", "testuser",
		"-H", "shell",
	)
	grantShellToUserCmd.Dir = dir

	err = grantShellToUserCmd.Run()
	if err != nil {
		t.Fatalf("failed granting shell grant to testuser, error: %s", err)
	}

	createHostCertCmd := exec.Command(
		"hiba-ca.sh",
		"-d", dir, //output to the tempdir
		"-s",
		"-h",
		"-I", "dut",
		"-H", "prod",
		"-V", "+52w",
	)
	createHostCertCmd.Dir = dir

	err = createHostCertCmd.Run()
	if err != nil {
		t.Fatalf("failed creating host certificate, error: %s", err)
	}

	createUserCertCmd := exec.Command(
		"hiba-ca.sh",
		"-d", dir, //output to the tempdir
		"-s",
		"-u",
		"-I", "testuser",
		"-H", "shell",
	)
	createUserCertCmd.Dir = dir

	err = createUserCertCmd.Run()
	if err != nil {
		t.Fatalf("failed creating user certificate, error: %s", err)
	}
}

func prepareHibaKeys(t *testing.T, dir string) {
	hibaCa, _ := exec.LookPath("hiba-ca.sh")
	hibaGen, _ := exec.LookPath("hiba-gen")

	if hibaCa == "" || hibaGen == "" {
		t.Log("hiba-ca and/or hiba-gen not found on path, will try to use certs in local test dir if present")

		prepareHibaKeysCopy(t, dir)
	} else {
		prepareHibaKeysGen(t, dir)
	}
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

	prepareHibaKeys(t, dir)

	addr := getDutAddr(t, dut)

	setupUser(t, dut)

	setupDUT(t, dut)

	testCases := []struct {
		name  string
		testF func(t *testing.T, dut *ondatra.DUTDevice, addr, dir string)
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
				},
			},
		}
		sendHostParametersRequest(t, dut, request)

		// clear hiba for authorized principals command
		request = &credentialz.RotateHostParametersRequest{
			Request: &credentialz.RotateHostParametersRequest_AuthorizedPrincipalCheck{
				AuthorizedPrincipalCheck: &credentialz.AuthorizedPrincipalCheckRequest{
					Tool: credentialz.AuthorizedPrincipalCheckRequest_TOOL_UNSPECIFIED,
				},
			},
		}
		sendHostParametersRequest(t, dut, request)

		// and the host key just to clean up all the things
		request = &credentialz.RotateHostParametersRequest{
			Request: &credentialz.RotateHostParametersRequest_ServerKeys{
				ServerKeys: &credentialz.ServerKeysRequest{
					AuthArtifacts: []*credentialz.ServerKeysRequest_AuthenticationArtifacts{},
				},
			},
		}
		sendHostParametersRequest(t, dut, request)
	})

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			tt.testF(t, dut, addr, dir)
		})
	}
}
