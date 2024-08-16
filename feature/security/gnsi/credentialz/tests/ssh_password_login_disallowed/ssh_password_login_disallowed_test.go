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
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/openconfig/gnsi/acctz"
	"google.golang.org/protobuf/types/known/timestamppb"

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
	username               = "testuser"
	password               = "i$V5^6IhD*tZ#eg1G@v3xdVZrQwj"
	userIdentity           = "my_principal"
	caPublicKeyFilename    = "ca.pub"
	userPrivateKeyFilename = "user"
	userPublicCertFilename = "user-cert.pub"
	// think this should work for all vendors contributing to fp
	command = "show version"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func prepareCaAndHostKeys(t *testing.T, dir string) {
	caCmd := exec.Command(
		"ssh-keygen",
		"-t", "ed25519",
		"-f", "ca",
		"-C", "ca",
		"-q", "-N", "", // quiet, empty passphrase
	)
	caCmd.Dir = dir

	err := caCmd.Run()
	if err != nil {
		t.Fatalf("failed generating ca key pair, error: %s", err)
	}

	dutCmd := exec.Command(
		"ssh-keygen",
		"-t", "ed25519",
		"-f", "user",
		"-C", "featureprofile@openconfig",
		"-q", "-N", "",
	)
	dutCmd.Dir = dir

	err = dutCmd.Run()
	if err != nil {
		t.Fatalf("failed generating user key pair, error: %s", err)
	}

	dutCertCmd := exec.Command(
		"ssh-keygen",
		"-s", "ca",
		"-I", "testuser",
		"-n", "my_principal",
		"-V", "+52w",
		"user.pub",
	)
	dutCertCmd.Dir = dir

	err = dutCertCmd.Run()
	if err != nil {
		t.Fatalf("failed generating user cert, error: %s", err)
	}
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
		t.Fatalf("failed receiving credentialz rotate host parameters response, error: %s", err)
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

func setupUser(t *testing.T, dut *ondatra.DUTDevice) {
	auth := &oc.System_Aaa_Authentication{}
	user := auth.GetOrCreateUser(username)
	user.SetRole(oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN)
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

func assertSSHAuthFails(t *testing.T, dut *ondatra.DUTDevice, addr, _ string) {
	var startingRejectCounter, startingLastRejectTime uint64

	if !deviations.SSHServerCountersUnsupported(dut) {
		startingRejectCounter, startingLastRejectTime = getAcceptTelemetry(t, dut)
	}

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

	if !deviations.SSHServerCountersUnsupported(dut) {
		endingRejectCounter, endingLastRejectTime := getRejectTelemetry(t, dut)

		if endingRejectCounter <= startingRejectCounter {
			t.Fatalf("ssh server reject counter did not increment after unsuccessful login")
		}

		if startingLastRejectTime == endingLastRejectTime {
			t.Fatalf("ssh server reject last timestamp did not update after unsuccessful login")
		}
	}
}

func assertConsoleAuthSucceeds(t *testing.T, _ *ondatra.DUTDevice, _, _ string) {
	t.Skip("skipping console test, partner issue: 304734163")
}

func assertSSHCertificateAuthSucceeds(t *testing.T, dut *ondatra.DUTDevice, addr, dir string) {
	privateKeyContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, userPrivateKeyFilename))
	if err != nil {
		t.Fatalf("failed reading private key contents, error: %s", err)
	}

	signer, err := ssh.ParsePrivateKey(privateKeyContents)
	if err != nil {
		t.Fatalf("failed parsing private key, error: %s", err)
	}

	certificateContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, userPublicCertFilename))
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

	var startingAcceptCounter, startingLastAcceptTime uint64

	if !deviations.SSHServerCountersUnsupported(dut) {
		startingAcceptCounter, startingLastAcceptTime = getAcceptTelemetry(t, dut)
	}

	conn, err := ssh.Dial(
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

	sess, err := conn.NewSession()
	if err != nil {
		t.Fatalf("failed creating ssh session, error: %s", err)
	}

	// we dont care if it fails really, just gotta send something so accountz has something to...
	// account
	_ = sess.Run(command)

	if !deviations.SSHServerCountersUnsupported(dut) {
		endingAcceptCounter, endingLastAcceptTime := getAcceptTelemetry(t, dut)

		if endingAcceptCounter <= startingAcceptCounter {
			t.Fatalf("ssh server accept counter did not increment after successful login")
		}

		if startingLastAcceptTime == endingLastAcceptTime {
			t.Fatalf("ssh server accept last timestamp did not update after successful login")
		}
	}

	assertSSHAuthAccounting(t, dut)
}

func getRejectTelemetry(t *testing.T, dut *ondatra.DUTDevice) (uint64, uint64) {
	sshCounters := gnmi.Get(t, dut, gnmi.OC().System().SshServer().Counters().State())

	return sshCounters.GetAccessRejects(), sshCounters.GetLastAccessReject()
}

func getAcceptTelemetry(t *testing.T, dut *ondatra.DUTDevice) (uint64, uint64) {
	sshCounters := gnmi.Get(t, dut, gnmi.OC().System().SshServer().Counters().State())

	return sshCounters.GetAccessAccepts(), sshCounters.GetLastAccessAccept()
}

func assertSSHAuthAccounting(t *testing.T, dut *ondatra.DUTDevice) {
	acctzClient := dut.RawAPIs().GNSI(t).Acctz()

	acctzSubClient, err := acctzClient.RecordSubscribe(context.Background())
	if err != nil {
		t.Fatalf("failed getting accountz record subscribe client, error: %s", err)
	}

	currentTime := time.Now()
	recordStarTime := currentTime.Add(-time.Minute * 3) // should very safely cover the last tests

	err = acctzSubClient.Send(&acctz.RecordRequest{
		Timestamp: timestamppb.New(recordStarTime),
	})
	if err != nil {
		t.Fatalf("failed sending accountz record request, error: %s", err)
	}

	for {
		var acctzResponse *acctz.RecordResponse

		acctzResponse, err = acctzSubClient.Recv()
		if err != nil {
			t.Fatalf(
				"failed receiving from accountz record subscribe client,"+
					"this could mean we didnt find the authorized principal and eventually timed "+
					"out with no more records to review, or another server error. error: %s",
				err,
			)
		}

		if acctzResponse.GetSessionInfo().LocalPort != 22 {
			// not ssh, not checking
			continue
		}

		reportedIdentity := acctzResponse.GetSessionInfo().GetUser().GetIdentity()

		if reportedIdentity == username {
			return
		}
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

	prepareCaAndHostKeys(t, dir)

	addr := getDutAddr(t, dut)

	setupUser(t, dut)
	setupTrustedUserCA(t, dut, dir)
	setupAuthenticationTypes(t, dut)
	setupAuthorizedUsers(t, dut)

	// brief sleep for ssh daemons to restart if necessary (after updating allowed auth types)
	time.Sleep(10 * time.Second)

	testCases := []struct {
		name  string
		testF func(t *testing.T, dut *ondatra.DUTDevice, addr, dir string)
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
			tt.testF(t, dut, addr, dir)
		})
	}
}
