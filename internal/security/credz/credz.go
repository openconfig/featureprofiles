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

// Package credz provides helper APIs to simplify writing credentialz test cases.
package credz

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	cpb "github.com/openconfig/gnsi/credentialz"
	tpb "github.com/openconfig/kne/proto/topo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"golang.org/x/crypto/ssh"
)

const (
	lowercase         = "abcdefghijklmnopqrstuvwxyz"
	uppercase         = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits            = "0123456789"
	symbols           = "!@#$%^&*(){}[]\\|:;\"'"
	space             = " "
	userKey           = "testuser"
	dutKey            = "dut"
	caKey             = "ca"
	minPasswordLength = 24
	maxPasswordLength = 32
	defaultSSHPort    = 22
)

var (
	charClasses = []string{lowercase, uppercase, digits, symbols, space}
)

// PrettyPrint prints rpc requests/responses in a pretty format.
func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

// SetupUser setup user for credentialz tests.
func SetupUser(t *testing.T, dut *ondatra.DUTDevice, username string) {
	auth := &oc.System_Aaa_Authentication{}
	user := auth.GetOrCreateUser(username)
	user.SetRole(oc.AaaTypes_SYSTEM_DEFINED_ROLES_SYSTEM_ROLE_ADMIN)
	gnmi.Update(t, dut, gnmi.OC().System().Aaa().Authentication().Config(), auth)
}

// GeneratePassword creates a password with following restrictions:
// - Must be 24-32 characters long.
// - Must use 4 of the 5 character classes ([a-z], [A-Z], [0-9], [!@#$%^&*(){}[]\|:;'"], [ ]).
func GeneratePassword() string {
	// Create random length between 24-32 characters long.
	delta := maxPasswordLength - minPasswordLength + 1
	length := minPasswordLength + rand.Intn(delta)

	// Randomly select 4 out of 5 character classes by shuffling the list.
	rand.Shuffle(len(charClasses), func(i, j int) {
		charClasses[i], charClasses[j] = charClasses[j], charClasses[i]
	})
	selectedClasses := charClasses[:4]

	var password strings.Builder

	// Add one random character from each selected class.
	for _, class := range selectedClasses {
		password.WriteByte(class[rand.Intn(len(class))])
	}

	// Fill remaining characters for the password.
	for password.Len() < length {
		classIndex := rand.Intn(len(charClasses))
		class := charClasses[classIndex]
		password.WriteByte(class[rand.Intn(len(class))])
	}

	return password.String()
}

func sendHostParametersRequest(t *testing.T, dut *ondatra.DUTDevice, request *cpb.RotateHostParametersRequest) {
	credzClient := dut.RawAPIs().GNSI(t).Credentialz()
	credzRotateClient, err := credzClient.RotateHostParameters(context.Background())
	if err != nil {
		t.Fatalf("Failed fetching credentialz rotate host parameters client, error: %s", err)
	}
	t.Logf("Sending credentialz rotate host request: %s", PrettyPrint(request))
	err = credzRotateClient.Send(request)
	if err != nil {
		t.Fatalf("Failed sending credentialz rotate host parameters request, error: %s", err)
	}
	_, err = credzRotateClient.Recv()
	if err != nil {
		t.Fatalf("Failed receiving credentialz rotate host parameters response, error: %s", err)
	}
	err = credzRotateClient.Send(&cpb.RotateHostParametersRequest{
		Request: &cpb.RotateHostParametersRequest_Finalize{
			Finalize: request.GetFinalize(),
		},
	})
	if err != nil {
		t.Fatalf("Failed sending credentialz rotate host parameters finalize request, error: %s", err)
	}
	// Brief sleep for finalize to get processed.
	time.Sleep(time.Second)
}

func sendAccountCredentialsRequest(t *testing.T, dut *ondatra.DUTDevice, request *cpb.RotateAccountCredentialsRequest) {
	credzClient := dut.RawAPIs().GNSI(t).Credentialz()
	credzRotateClient, err := credzClient.RotateAccountCredentials(context.Background())
	if err != nil {
		t.Fatalf("Failed fetching credentialz rotate account credentials client, error: %s", err)
	}
	t.Logf("Sending credentialz rotate account request: %s", PrettyPrint(request))
	err = credzRotateClient.Send(request)
	if err != nil {
		t.Fatalf("Failed sending credentialz rotate account credentials request, error: %s", err)
	}
	_, err = credzRotateClient.Recv()
	if err != nil {
		t.Fatalf("Failed receiving credentialz rotate account credentials response, error: %s", err)
	}
	err = credzRotateClient.Send(&cpb.RotateAccountCredentialsRequest{
		Request: &cpb.RotateAccountCredentialsRequest_Finalize{
			Finalize: request.GetFinalize(),
		},
	})
	if err != nil {
		t.Fatalf("Failed sending credentialz rotate account credentials finalize request, error: %s", err)
	}
	// Brief sleep for finalize to get processed.
	time.Sleep(time.Second)
}

// RotateUserPassword apply password for the specified username on the dut.
func RotateUserPassword(t *testing.T, dut *ondatra.DUTDevice, username, password, version string, createdOn uint64) {
	request := &cpb.RotateAccountCredentialsRequest{
		Request: &cpb.RotateAccountCredentialsRequest_Password{
			Password: &cpb.PasswordRequest{
				Accounts: []*cpb.PasswordRequest_Account{
					{
						Account: username,
						Password: &cpb.PasswordRequest_Password{
							Value: &cpb.PasswordRequest_Password_Plaintext{
								Plaintext: password,
							},
						},
						Version:   version,
						CreatedOn: createdOn,
					},
				},
			},
		},
	}

	sendAccountCredentialsRequest(t, dut, request)
}

// RotateAuthorizedPrincipal apply authorized principal for the specified username on the dut.
func RotateAuthorizedPrincipal(t *testing.T, dut *ondatra.DUTDevice, username, userPrincipal string, version string, createdOn uint64) {
	request := &cpb.RotateAccountCredentialsRequest{
		Request: &cpb.RotateAccountCredentialsRequest_User{
			User: &cpb.AuthorizedUsersRequest{
				Policies: []*cpb.UserPolicy{
					{
						Account: username,
						AuthorizedPrincipals: &cpb.UserPolicy_SshAuthorizedPrincipals{
							AuthorizedPrincipals: []*cpb.UserPolicy_SshAuthorizedPrincipal{
								{
									AuthorizedUser: userPrincipal,
								},
							},
						},
						Version:   version,
						CreatedOn: createdOn,
					},
				},
			},
		},
	}

	sendAccountCredentialsRequest(t, dut, request)
}

// RotateAuthorizedKey read user key contents from the specified directory & apply it as authorized key on the dut.
func RotateAuthorizedKey(t *testing.T, dut *ondatra.DUTDevice, dir, username, version string, createdOn uint64) {
	var keyContents []*cpb.AccountCredentials_AuthorizedKey

	if dir != "" {
		data, err := os.ReadFile(fmt.Sprintf("%s/%s.pub", dir, userKey))
		if err != nil {
			t.Fatalf("Failed reading private key contents, error: %s", err)
		}
		keyContents = append(keyContents, &cpb.AccountCredentials_AuthorizedKey{
			AuthorizedKey: data,
			KeyType:       cpb.KeyType_KEY_TYPE_ED25519,
		})
	}
	request := &cpb.RotateAccountCredentialsRequest{
		Request: &cpb.RotateAccountCredentialsRequest_Credential{
			Credential: &cpb.AuthorizedKeysRequest{
				Credentials: []*cpb.AccountCredentials{
					{
						Account:        username,
						AuthorizedKeys: keyContents,
						Version:        version,
						CreatedOn:      createdOn,
					},
				},
			},
		},
	}

	sendAccountCredentialsRequest(t, dut, request)
}

// RotateTrustedUserCA read CA key contents from the specified directory & apply it on the dut.
func RotateTrustedUserCA(t *testing.T, dut *ondatra.DUTDevice, dir string, version string, createdOn uint64) {
	var keyContents []*cpb.PublicKey

	if dir != "" {
		data, err := os.ReadFile(fmt.Sprintf("%s/%s.pub", dir, caKey))
		if err != nil {
			t.Fatalf("Failed reading ca public key contents, error: %s", err)
		}
		keyContents = append(keyContents, &cpb.PublicKey{
			PublicKey: data,
			KeyType:   cpb.KeyType_KEY_TYPE_ED25519,
		})
	}
	request := &cpb.RotateHostParametersRequest{
		Request: &cpb.RotateHostParametersRequest_SshCaPublicKey{
			SshCaPublicKey: &cpb.CaPublicKeyRequest{
				SshCaPublicKeys: keyContents,
				Version:   version,
				CreatedOn: createdOn,
			},
		},
	}

	sendHostParametersRequest(t, dut, request)
}

// RotateAuthenticationTypes apply specified host authentication types on the dut.
func RotateAuthenticationTypes(t *testing.T, dut *ondatra.DUTDevice, authTypes []cpb.AuthenticationType) {
	request := &cpb.RotateHostParametersRequest{
		Request: &cpb.RotateHostParametersRequest_AuthenticationAllowed{
			AuthenticationAllowed: &cpb.AllowedAuthenticationRequest{
				AuthenticationTypes: authTypes,
			},
		},
	}

	sendHostParametersRequest(t, dut, request)
}

// RotateAuthenticationArtifacts read dut key/certificate contents from the specified directory & apply it as host authentication artifacts on the dut.
func RotateAuthenticationArtifacts(t *testing.T, dut *ondatra.DUTDevice, keyDir, certDir, version string, createdOn uint64) {
	var artifactContents []*cpb.ServerKeysRequest_AuthenticationArtifacts

	if keyDir != "" {
		data, err := os.ReadFile(fmt.Sprintf("%s/%s", keyDir, dutKey))
		if err != nil {
			t.Fatalf("Failed reading host private key, error: %s", err)
		}
		artifactContents = append(artifactContents, &cpb.ServerKeysRequest_AuthenticationArtifacts{
			PrivateKey: data,
		})
	}

	if certDir != "" {
		data, err := os.ReadFile(fmt.Sprintf("%s/%s-cert.pub", certDir, dutKey))
		if err != nil {
			t.Fatalf("Failed reading host signed certificate, error: %s", err)
		}
		artifactContents = append(artifactContents, &cpb.ServerKeysRequest_AuthenticationArtifacts{
			Certificate: data,
		})
	}

	request := &cpb.RotateHostParametersRequest{
		Request: &cpb.RotateHostParametersRequest_ServerKeys{
			ServerKeys: &cpb.ServerKeysRequest{
				AuthArtifacts: artifactContents,
				Version:       version,
				CreatedOn:     createdOn,
			},
		},
	}

	sendHostParametersRequest(t, dut, request)
}

// RotateAuthorizedPrincipalCheck apply specified authorized principal tool on the dut.
func RotateAuthorizedPrincipalCheck(t *testing.T, dut *ondatra.DUTDevice, tool cpb.AuthorizedPrincipalCheckRequest_Tool) {
	request := &cpb.RotateHostParametersRequest{
		Request: &cpb.RotateHostParametersRequest_AuthorizedPrincipalCheck{
			AuthorizedPrincipalCheck: &cpb.AuthorizedPrincipalCheckRequest{
				Tool: tool,
			},
		},
	}

	sendHostParametersRequest(t, dut, request)
}

// GetRejectTelemetry retrieve ssh reject telemetry counters from the dut.
func GetRejectTelemetry(t *testing.T, dut *ondatra.DUTDevice) (uint64, uint64) {
	sshCounters := gnmi.Get(t, dut, gnmi.OC().System().SshServer().Counters().State())
	return sshCounters.GetAccessRejects(), sshCounters.GetLastAccessReject()
}

// GetAcceptTelemetry retrieve ssh accept telemetry counters from the dut.
func GetAcceptTelemetry(t *testing.T, dut *ondatra.DUTDevice) (uint64, uint64) {
	sshCounters := gnmi.Get(t, dut, gnmi.OC().System().SshServer().Counters().State())
	return sshCounters.GetAccessAccepts(), sshCounters.GetLastAccessAccept()
}

// GetDutTarget returns ssh target for the dut to be used in credentialz tests.
func GetDutTarget(t *testing.T, dut *ondatra.DUTDevice) string {
	var serviceDUT interface {
		Service(string) (*tpb.Service, error)
	}
	err := binding.DUTAs(dut.RawAPIs().BindingDUT(), &serviceDUT)
	if err != nil {
		t.Log("DUT does not support `Service` function, will attempt to use dut name field")
		return fmt.Sprintf("%s:%d", dut.Name(), defaultSSHPort)
	}
	dutSSHService, err := serviceDUT.Service("ssh")
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%s:%d", dutSSHService.GetOutsideIp(), dutSSHService.GetOutside())
}

// GetDutPublicKey retrieve single host public key from the dut.
func GetDutPublicKey(t *testing.T, dut *ondatra.DUTDevice) []byte {
	credzClient := dut.RawAPIs().GNSI(t).Credentialz()
	req := &cpb.GetPublicKeysRequest{}
	response, err := credzClient.GetPublicKeys(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed fetching fetching credentialz public keys, error: %s", err)
	}
	if len(response.PublicKeys) < 1 {
		return nil
	}
	return response.PublicKeys[0].PublicKey
}

// CreateSSHKeyPair creates ssh keypair with a filename of keyName in the specified directory.
// Keypairs can be created for ca/dut/testuser as per individual credentialz test requirements.
func CreateSSHKeyPair(t *testing.T, dir, keyName string) {
	sshCmd := exec.Command(
		"ssh-keygen",
		"-t", "ed25519",
		"-f", keyName,
		"-C", keyName,
		"-q", "-N", "",
	)
	sshCmd.Dir = dir
	err := sshCmd.Run()
	if err != nil {
		t.Fatalf("Failed generating %s key pair, error: %s", keyName, err)
	}
}

// CreateUserCertificate creates ssh user certificate in the specified directory.
func CreateUserCertificate(t *testing.T, dir, userPrincipal string) {
	userCertCmd := exec.Command(
		"ssh-keygen",
		"-s", caKey,
		"-I", userKey,
		"-n", userPrincipal,
		"-V", "+52w",
		fmt.Sprintf("%s.pub", userKey),
	)
	userCertCmd.Dir = dir
	err := userCertCmd.Run()
	if err != nil {
		t.Fatalf("Failed generating user cert, error: %s", err)
	}
}

// CreateHostCertificate takes in dut key contents & creates ssh host certificate in the specified directory.
func CreateHostCertificate(t *testing.T, dir string, dutKeyContents []byte) {
	err := os.WriteFile(fmt.Sprintf("%s/%s.pub", dir, dutKey), dutKeyContents, 0o777)
	if err != nil {
		t.Fatalf("Failed writing dut public key to temp dir, error: %s", err)
	}
	cmd := exec.Command(
		"ssh-keygen",
		"-s", caKey, // sign using this ca key
		"-I", dutKey, // key identity
		"-h",                 // create host (not user) certificate
		"-n", "dut.test.com", // principal(s)
		"-V", "+52w", // validity
		fmt.Sprintf("%s.pub", dutKey),
	)
	cmd.Dir = dir
	err = cmd.Run()
	if err != nil {
		t.Fatalf("Failed generating dut cert, error: %s", err)
	}
}

func createHibaKeysCopy(t *testing.T, dir string) {
	keyFiles := []string{
		"ca",
		"ca.pub",
		"hosts/dut",
		"hosts/dut.pub",
		"hosts/dut-cert.pub",
		"users/testuser",
		"users/testuser.pub",
		"users/testuser-cert.pub",
	}
	err := os.Mkdir(fmt.Sprintf("%s/hosts", dir), 0o700)
	if err != nil {
		t.Fatalf("Failed ensuring hosts dir in temp dir, error: %s", err)
	}
	err = os.Mkdir(fmt.Sprintf("%s/users", dir), 0o700)
	if err != nil {
		t.Fatalf("Failed ensuring users dir in temp dir, error: %s", err)
	}

	for _, keyFile := range keyFiles {
		var input []byte
		input, err = os.ReadFile(keyFile)
		if err != nil {
			t.Errorf("Error reading file %v, error: %s", keyFile, err)
			return
		}
		err = os.WriteFile(fmt.Sprintf("%s/%s", dir, keyFile), input, 0o600)
		if err != nil {
			t.Fatalf("Failed copying key file %s to temp test dir, error: %s", keyFile, err)
		}
	}
}

func createHibaKeysGen(t *testing.T, dir string) {
	caCmd := exec.Command(
		"hiba-ca.sh",
		"-c",
		"-d", dir, // output to the temp dir
		"--",           // pass the rest to ssh-keygen
		"-q", "-N", "", // quiet, empty passphrase

	)
	caCmd.Dir = dir
	err := caCmd.Run()
	if err != nil {
		t.Fatalf("Failed generating ca key pair, error: %s", err)
	}

	userKeyCmd := exec.Command(
		"hiba-ca.sh",
		"-c",
		"-d", dir,
		"-u", "-I", userKey,
		"--",
		"-q", "-N", "",
	)
	userKeyCmd.Dir = dir
	err = userKeyCmd.Run()
	if err != nil {
		t.Fatalf("Failed generating user key pair, error: %s", err)
	}

	dutKeyCmd := exec.Command(
		"hiba-ca.sh",
		"-c",
		"-d", dir,
		"-h", "-I", dutKey,
		"--",
		"-q", "-N", "",
	)
	dutKeyCmd.Dir = dir
	err = dutKeyCmd.Run()
	if err != nil {
		t.Fatalf("Failed generating dut key pair, error: %s", err)
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
		t.Fatalf("Failed creating prod identity, error: %s", err)
	}

	shellGrantCmd := exec.Command(
		"hiba-gen",
		"-f", fmt.Sprintf("%s/policy/grants/shell", dir),
		"domain", "example.com",
	)
	shellGrantCmd.Dir = dir
	err = shellGrantCmd.Run()
	if err != nil {
		t.Fatalf("Failed creating shell grant, error: %s", err)
	}

	grantShellToUserCmd := exec.Command(
		"hiba-ca.sh",
		"-d", dir,
		"-p",
		"-I", userKey,
		"-H", "shell",
	)
	grantShellToUserCmd.Dir = dir
	err = grantShellToUserCmd.Run()
	if err != nil {
		t.Fatalf("Failed granting shell grant to testuser, error: %s", err)
	}

	createHostCertCmd := exec.Command(
		"hiba-ca.sh",
		"-d", dir,
		"-s",
		"-h",
		"-I", dutKey,
		"-H", "prod",
		"-V", "+52w",
	)
	createHostCertCmd.Dir = dir
	err = createHostCertCmd.Run()
	if err != nil {
		t.Fatalf("Failed creating host certificate, error: %s", err)
	}

	createUserCertCmd := exec.Command(
		"hiba-ca.sh",
		"-d", dir,
		"-s",
		"-u",
		"-I", userKey,
		"-H", "shell",
	)
	createUserCertCmd.Dir = dir
	err = createUserCertCmd.Run()
	if err != nil {
		t.Fatalf("Failed creating user certificate, error: %s", err)
	}
}

// CreateHibaKeys creates/copies hiba granted keys/certificates in the specified directory.
// If hiba tool is not installed on the testbed, ensure following files (generated after executing steps
// from https://github.com/google/hiba/blob/main/CA.md) are present in the test directory :
// feature/security/gnsi/credentialz/tests/hiba_authentication/ca,
// feature/security/gnsi/credentialz/tests/hiba_authentication/ca.pub,
// feature/security/gnsi/credentialz/tests/hiba_authentication/hosts/dut,
// feature/security/gnsi/credentialz/tests/hiba_authentication/hosts/dut.pub,
// feature/security/gnsi/credentialz/tests/hiba_authentication/hosts/dut-cert.pub,
// feature/security/gnsi/credentialz/tests/hiba_authentication/users/testuser,
// feature/security/gnsi/credentialz/tests/hiba_authentication/users/testuser.pub,
// feature/security/gnsi/credentialz/tests/hiba_authentication/users/testuser-cert.pub,
func CreateHibaKeys(t *testing.T, dir string) {
	hibaCa, _ := exec.LookPath("hiba-ca.sh")
	hibaGen, _ := exec.LookPath("hiba-gen")
	if hibaCa == "" || hibaGen == "" {
		t.Log("hiba-ca and/or hiba-gen not found on path, will try to use certs in local test dir if present.")
		createHibaKeysCopy(t, dir)
	} else {
		createHibaKeysGen(t, dir)
	}
}

// SSHWithPassword dials ssh with password based authentication to be used in credentialz tests.
func SSHWithPassword(target, username, password string) (*ssh.Client, error) {
	return ssh.Dial(
		"tcp",
		target,
		&ssh.ClientConfig{
			User: username,
			Auth: []ssh.AuthMethod{
				ssh.Password(password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), // lgtm[go/insecure-hostkeycallback]
		},
	)
}

// SSHWithCertificate dials ssh with user certificate to be used in credentialz tests.
func SSHWithCertificate(t *testing.T, target, username, dir string) (*ssh.Client, error) {
	privateKeyContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, userKey))
	if err != nil {
		t.Fatalf("Failed reading private key contents, error: %s", err)
	}
	signer, err := ssh.ParsePrivateKey(privateKeyContents)
	if err != nil {
		t.Fatalf("Failed parsing private key, error: %s", err)
	}
	certificateContents, err := os.ReadFile(fmt.Sprintf("%s/%s-cert.pub", dir, userKey))
	if err != nil {
		t.Fatalf("Failed reading certificate contents, error: %s", err)
	}
	certificate, _, _, _, err := ssh.ParseAuthorizedKey(certificateContents)
	if err != nil {
		t.Fatalf("Failed parsing certificate contents, error: %s", err)
	}
	certificateSigner, err := ssh.NewCertSigner(certificate.(*ssh.Certificate), signer)
	if err != nil {
		t.Fatalf("Failed creating certificate signer, error: %s", err)
	}

	return ssh.Dial(
		"tcp",
		target,
		sshClientConfigWithPublicKeys(username, certificateSigner),
	)
}

// SSHWithKey dials ssh with key based authentication to be used in credentialz tests.
func SSHWithKey(t *testing.T, target, username, dir string) (*ssh.Client, error) {
	privateKeyContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, userKey))
	if err != nil {
		t.Fatalf("Failed reading private key contents, error: %s", err)
	}
	signer, err := ssh.ParsePrivateKey(privateKeyContents)
	if err != nil {
		t.Fatalf("Failed parsing private key, error: %s", err)
	}

	return ssh.Dial(
		"tcp",
		target,
		sshClientConfigWithPublicKeys(username, signer),
	)
}

func sshClientConfigWithPublicKeys(username string, signer ssh.Signer) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // lgtm[go/insecure-hostkeycallback]
	}
}
