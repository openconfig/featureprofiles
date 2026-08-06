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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/helpers"
	cpb "github.com/openconfig/gnsi/credentialz"
	tpb "github.com/openconfig/kne/proto/topo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const (
	lowercase         = "abcdefghijklmnopqrstuvwxyz"
	uppercase         = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits            = "0123456789"
	symbols           = "!@#$%^&*(){}[]\\|:;\"'"
	space             = " "
	dutKey            = "dut"
	userKey           = "testuser"
	caKey             = "ca"
	minPasswordLength = 24
	maxPasswordLength = 32
	defaultSSHPort    = 22

	// rotateStreamTimeout bounds a single end-to-end credentialz Rotate stream
	// (open -> Send -> Recv -> Send(Finalize) -> drain to io.EOF). It must be
	// generous enough for slow control-plane commits yet short enough that a
	// stuck stream fails fast instead of hanging until the outer `go test`
	// -timeout fires.
	rotateStreamTimeout = 60 * time.Second
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
	// charClasses is a function-local slice (not a package-level var) so that
	// concurrent callers of GeneratePassword do not race on a shared slice:
	// rand.Shuffle below reorders it in place, which would otherwise mutate
	// global state and introduce a data race.
	charClasses := []string{lowercase, uppercase, digits, symbols, space}

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

// rotateStream is the minimal streaming surface shared by the credentialz
// RotateHostParameters and RotateAccountCredentials client streams. Both
// generated clients already satisfy it, which lets a single helper drive the
// full end-to-end rotate handshake for either RPC.
type rotateStream[Req any, Resp any] interface {
	Send(Req) error
	Recv() (Resp, error)
}

// runRotate performs the complete, end-to-end credentialz Rotate handshake and
// guarantees the device is left in a committed (clean) state for every call:
//
//  1. Send the caller's request.
//  2. Recv the server's acknowledgement.
//  3. Send Finalize.
//  4. Drain Recv until io.EOF so the commit is fully applied and the stream is
//     closed cleanly before returning.
//
// Context handling (why this is not simply context.Background()):
//
// The rotate stream is derived from t.Context() while it is still live. This is
// exactly what we want from the test body: if the test is canceled or hits its
// deadline, the in-flight rotate stream is torn down with it, so the Recv-until-
// EOF drain below can never outlive the test.
//
// However, per the Go testing contract, t.Context() is canceled *just before* a
// test's t.Cleanup callbacks run. These credentialz helpers are intentionally
// invoked from BOTH the test body and from t.Cleanup / deferred teardown (that
// is the whole point of the cleanup-every-run contract). If we used t.Context()
// unconditionally, every teardown rotation would fail with
// "rpc error: code = Canceled desc = context canceled" and leave the device
// dirty. So once t.Context() is already canceled (i.e. we are running from
// cleanup) we transparently fall back to context.Background() for that call.
//
// In both cases we layer a WithTimeout(rotateStreamTimeout) on top, so a stuck
// stream fails fast in CI instead of hanging until the outer `go test` -timeout.
func runRotate[Req any, Resp any](
	t *testing.T,
	rpcName string,
	open func(context.Context) (rotateStream[Req, Resp], error),
	request Req,
	finalize Req,
) {
	t.Helper()

	// Prefer the live test context; fall back to Background only when t.Context()
	// is already canceled (cleanup/teardown path). Always bounded by a timeout.
	base := t.Context()
	if base.Err() != nil {
		t.Logf("t.Context() already canceled (%v); using background context for credentialz %s (cleanup path).", base.Err(), rpcName)
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, rotateStreamTimeout)
	defer cancel()

	stream, err := open(ctx)
	if err != nil {
		t.Fatalf("Failed fetching credentialz %s client, error: %s", rpcName, err)
	}

	t.Logf("Sending credentialz %s request: %s", rpcName, PrettyPrint(request))
	if err := stream.Send(request); err != nil {
		t.Fatalf("Failed sending credentialz %s request, error: %s", rpcName, err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("Failed receiving credentialz %s response, error: %s", rpcName, err)
	}

	if err := stream.Send(finalize); err != nil {
		t.Fatalf("Failed sending credentialz %s finalize request, error: %s", rpcName, err)
	}
	// Read response to Finalize until EOF so the commit is fully applied.
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed during %s finalize Recv, error: %s", rpcName, err)
		}
	}
}

func sendHostParametersRequest(t *testing.T, dut *ondatra.DUTDevice, request *cpb.RotateHostParametersRequest) {
	t.Helper()
	credzClient := dut.RawAPIs().GNSI(t).Credentialz()
	runRotate(
		t,
		"rotate host parameters",
		func(ctx context.Context) (rotateStream[*cpb.RotateHostParametersRequest, *cpb.RotateHostParametersResponse], error) {
			return credzClient.RotateHostParameters(ctx)
		},
		request,
		&cpb.RotateHostParametersRequest{
			Request: &cpb.RotateHostParametersRequest_Finalize{
				Finalize: request.GetFinalize(),
			},
		},
	)
}

func sendAccountCredentialsRequest(t *testing.T, dut *ondatra.DUTDevice, request *cpb.RotateAccountCredentialsRequest) {
	t.Helper()
	credzClient := dut.RawAPIs().GNSI(t).Credentialz()
	runRotate(
		t,
		"rotate account credentials",
		func(ctx context.Context) (rotateStream[*cpb.RotateAccountCredentialsRequest, *cpb.RotateAccountCredentialsResponse], error) {
			return credzClient.RotateAccountCredentials(ctx)
		},
		request,
		&cpb.RotateAccountCredentialsRequest{
			Request: &cpb.RotateAccountCredentialsRequest_Finalize{
				Finalize: request.GetFinalize(),
			},
		},
	)
}

// GenerateVersion returns a unique version string for gNSI rotations.
func GenerateVersion() string {
	return fmt.Sprintf("v%d", time.Now().UnixNano())
}

// RotateUserPassword applies or deletes the password for the specified username on the DUT.
// To add/update a password, provide non-empty password, version, and createdOn.
// To delete a password, provide empty strings for password and version, and 0 for createdOn.
//
// The request is always constructed as a single, well-formed message. The plaintext
// password value is only attached when a password is supplied, so no combination of
// arguments can leave the request nil (which previously risked a nil pointer
// dereference / gRPC panic in sendAccountCredentialsRequest).
func RotateUserPassword(t *testing.T, dut *ondatra.DUTDevice, username, password, version string, createdOn uint64) {
	pw := &cpb.PasswordRequest_Password{}
	if password != "" {
		pw.Value = &cpb.PasswordRequest_Password_Plaintext{
			Plaintext: password,
		}
	}

	request := &cpb.RotateAccountCredentialsRequest{
		Request: &cpb.RotateAccountCredentialsRequest_Password{
			Password: &cpb.PasswordRequest{
				Accounts: []*cpb.PasswordRequest_Account{
					{
						Account:   username,
						Password:  pw,
						Version:   version,
						CreatedOn: createdOn,
					},
				},
			},
		},
	}

	sendAccountCredentialsRequest(t, dut, request)
}

// RotateAuthorizedPrincipal applies or deletes authorized principal for the specified username on the dut.
// To add/update authorized principal, provide non-empty authorized principal, version, and createdOn.
// To delete authorized principal, provide empty strings for authorized principal and version, and 0 for createdOn.
//
// The request is always constructed as a single, well-formed message. Authorized
// principals are only attached when a principal is supplied, so no combination of
// arguments can leave the request nil (which previously risked a nil pointer
// dereference / gRPC panic in sendAccountCredentialsRequest).
func RotateAuthorizedPrincipal(t *testing.T, dut *ondatra.DUTDevice, username, userPrincipal, version string, createdOn uint64) {
	var authPrincipals *cpb.UserPolicy_SshAuthorizedPrincipals
	if userPrincipal != "" {
		authPrincipals = &cpb.UserPolicy_SshAuthorizedPrincipals{
			AuthorizedPrincipals: []*cpb.UserPolicy_SshAuthorizedPrincipal{
				{
					AuthorizedUser: userPrincipal,
				},
			},
		}
	}

	request := &cpb.RotateAccountCredentialsRequest{
		Request: &cpb.RotateAccountCredentialsRequest_User{
			User: &cpb.AuthorizedUsersRequest{
				Policies: []*cpb.UserPolicy{
					{
						Account:              username,
						AuthorizedPrincipals: authPrincipals,
						Version:              version,
						CreatedOn:            createdOn,
					},
				},
			},
		},
	}

	sendAccountCredentialsRequest(t, dut, request)
}

// RotateAuthorizedKey read user key contents from the specified directory & apply it as authorized key on the dut.
// To add an authorized key, provide a non-empty dir (and a version/createdOn for telemetry).
// To delete/clear the authorized key, provide an empty dir (the key list is sent empty).
func RotateAuthorizedKey(t *testing.T, dut *ondatra.DUTDevice, dir, username, version string, createdOn uint64) {
	var keyContents []*cpb.AccountCredentials_AuthorizedKey

	if dir != "" {
		data, err := os.ReadFile(fmt.Sprintf("%s/%s.pub", dir, userKey))
		if err != nil {
			t.Fatalf("Failed reading private key contents, error: %s", err)
		}
		dataTypes := bytes.Fields(data)
		keyType := keyTypeFromAlgo(string(dataTypes[0]))
		if keyType == cpb.KeyType_KEY_TYPE_UNSPECIFIED {
			keyType = cpb.KeyType_KEY_TYPE_ED25519
		}
		authKey := dataTypes[1]
		keyContents = append(keyContents, &cpb.AccountCredentials_AuthorizedKey{
			AuthorizedKey: authKey,
			KeyType:       keyType,
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

// RotateTrustedUserCA applies or deletes CA key contents on the dut.
// To add the CA, provide a non-empty dir along with a version and createdOn.
// To delete the CA, provide an empty dir, empty version, and 0 createdOn.
func RotateTrustedUserCA(t *testing.T, dut *ondatra.DUTDevice, dir, version string, createdOn uint64) {
	var keyContents []*cpb.PublicKey

	if dir != "" {
		data, err := os.ReadFile(fmt.Sprintf("%s/%s.pub", dir, caKey))
		if err != nil {
			t.Fatalf("Failed reading ca public key contents, error: %s", err)
		}
		dataTypes := bytes.Fields(data)
		keyType := keyTypeFromAlgo(string(dataTypes[0]))
		if keyType == cpb.KeyType_KEY_TYPE_UNSPECIFIED {
			t.Fatalf("Unrecognized key type: %s", dataTypes[0])
		}
		pubKey := dataTypes[1]
		keyContents = append(keyContents, &cpb.PublicKey{
			PublicKey: pubKey,
			KeyType:   keyType,
		})
	}
	request := &cpb.RotateHostParametersRequest{
		Request: &cpb.RotateHostParametersRequest_SshCaPublicKey{
			SshCaPublicKey: &cpb.CaPublicKeyRequest{
				SshCaPublicKeys: keyContents,
				Version:         version,
				CreatedOn:       createdOn,
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

// RotateAuthenticationArtifacts reads dut key/certificate contents from the specified
// directories & applies them as host authentication artifacts on the dut.
//
// To install artifacts, provide keyDir and/or certDir along with a non-empty version
// and non-zero createdOn. When both a key and a certificate are provided, they are
// bundled into a single AuthenticationArtifacts entry.
//
// To clear artifacts, pass empty keyDir and certDir. Per the gNSI spec, every
// ServerKeys rotation must carry a version/created_on (they are persisted and reported
// via telemetry), so if version is empty / createdOn is 0 they are auto-generated.
// This keeps the request well-formed and accepted across all vendors (e.g. it avoids
// vendor rejections such as "Empty version string. Need value for version.").
func RotateAuthenticationArtifacts(t *testing.T, dut *ondatra.DUTDevice, keyDir, certDir, version string, createdOn uint64) {
	var artifactContents []*cpb.ServerKeysRequest_AuthenticationArtifacts

	var keyData []byte
	var certData []byte
	var err error
	if keyDir != "" {
		keyData, err = os.ReadFile(fmt.Sprintf("%s/%s", keyDir, dut.ID()))
		if err != nil {
			t.Fatalf("Failed reading host private key, error: %s", err)
		}
	}

	if certDir != "" {
		certData, err = os.ReadFile(fmt.Sprintf("%s/%s-cert.pub", certDir, dut.ID()))
		if err != nil {
			t.Fatalf("Failed reading host signed certificate, error: %s", err)
		}
	}

	// Only add an artifact when there is actually a key and/or certificate to send.
	// This keeps the cleanup call (keyDir == "" && certDir == "") from sending an empty
	// artifact (auth_artifacts: [{}]), which some vendors reject as malformed.
	if keyData != nil || certData != nil {
		artifactContents = append(artifactContents, &cpb.ServerKeysRequest_AuthenticationArtifacts{
			PrivateKey:  keyData,
			Certificate: certData,
		})
	}

	// The gNSI spec requires version/created_on on every ServerKeys rotation, including
	// when clearing artifacts. Auto-generate them if the caller did not provide them so
	// the request is accepted across all vendors.
	if version == "" {
		version = GenerateVersion()
	}
	if createdOn == 0 {
		createdOn = uint64(time.Now().Unix())
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
func GetDutPublicKey(t *testing.T, dut *ondatra.DUTDevice, targetAlgo string) []byte {
	credzClient := dut.RawAPIs().GNSI(t).Credentialz()
	req := &cpb.GetPublicKeysRequest{}
	response, err := credzClient.GetPublicKeys(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed fetching fetching credentialz public keys, error: %s", err)
	}
	if len(response.PublicKeys) < 1 {
		return nil
	}
	t.Logf("Fetching gNSI public keys... total keys: %d keys: %+v", len(response.PublicKeys), response.PublicKeys)

	var key *cpb.PublicKey
	var algo string

	if targetAlgo != "" {
		for _, k := range response.PublicKeys {
			algo = sshAlgo(t, k)
			if algo == targetAlgo {
				key = k
				break
			}
		}
		if key == nil {
			t.Fatalf("Failed to find host key for algorithm %s on DUT. Available keys and their types can be inspected via logs.", targetAlgo)
		}
	} else {
		// Form the key bytes from the proto message.
		key = response.PublicKeys[0]
		algo = sshAlgo(t, key)
		if algo == "" {
			// Attempt to find a supported key type if the first one is unsupported.
			for _, k := range response.PublicKeys {
				algo = sshAlgo(t, k)
				if algo != "" {
					key = k
					break
				}
			}
			if algo == "" {
				t.Fatalf("No supported public keys found on DUT. Available keys and their types can be inspected via logs.")
			}
		}
	}

	keyData := sshKey(t, key)
	keyLine := algo + " " + keyData + " " + key.Description
	t.Logf("Found SSH public key on DUT: %s", keyLine)
	return []byte(keyLine)
}

// CreateSSHKeyPairAlgo creates ssh keypair with a filename of keyName in the specified directory with the specified algo.
func CreateSSHKeyPairAlgo(t *testing.T, dir, keyName, algo string) {
	args := []string{
		"-t", algo,
	}
	if algo == "rsa" {
		args = append(args, "-b", "4096")
	}
	args = append(args, "-f", keyName, "-C", keyName, "-q", "-N", "")
	sshCmd := exec.Command("ssh-keygen", args...)
	sshCmd.Dir = dir
	err := sshCmd.Run()
	if err != nil {
		t.Fatalf("Failed generating %s key pair, error: %s", keyName, err)
	}
}

// CreateSSHKeyPair creates ssh keypair with a filename of keyName in the specified directory.
// Keypairs can be created for ca/dut/testuser as per individual credentialz test requirements.
func CreateSSHKeyPair(t *testing.T, dir, keyName string) {
	CreateSSHKeyPairAlgo(t, dir, keyName, "ed25519")
}

// CreateUserCertificate creates ssh user certificate in the specified directory.
func CreateUserCertificate(t *testing.T, dir, userPrincipal string) {
	userCertCmd := exec.Command(
		"ssh-keygen",
		"-s", caKey,
		"-I", userKey,
		"-n", userPrincipal,
		"-V", "-1d:+52w",
		fmt.Sprintf("%s.pub", userKey),
	)
	userCertCmd.Dir = dir
	err := userCertCmd.Run()
	if err != nil {
		t.Fatalf("Failed generating user cert, error: %s", err)
	}
}

// CreateHostCertificate takes in dut key contents & creates ssh host certificate in the specified directory.
func CreateHostCertificate(t *testing.T, dut *ondatra.DUTDevice, dir string, dutKeyContents []byte) {
	t.Logf("DUT Public Key Contents used for cert generation: %s", string(dutKeyContents))
	err := os.WriteFile(fmt.Sprintf("%s/%s.pub", dir, dut.ID()), dutKeyContents, 0o777)
	if err != nil {
		t.Fatalf("Failed writing dut public key to temp dir, error: %s", err)
	}
	cmd := exec.Command(
		"ssh-keygen",
		"-s", caKey, // sign using this ca key
		"-I", "identity", // key identity
		"-h",                 // create host (not user) certificate
		"-n", "dut.test.com", // principal(s)
		"-V", "-1d:+52w", // validity
		fmt.Sprintf("%s.pub", dut.ID()),
	)
	t.Logf("Generating host certificate: %v", cmd)
	cmd.Dir = dir
	err = cmd.Run()
	if err != nil {
		t.Fatalf("Failed generating dut cert, error: %s", err)
	}
}

func createHibaKeysCopy(t *testing.T, keysDir string) {
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
	err := os.Mkdir(fmt.Sprintf("%s/hosts", keysDir), 0o700)
	if err != nil {
		t.Fatalf("Failed ensuring hosts dir in temp dir, error: %s", err)
	}
	err = os.Mkdir(fmt.Sprintf("%s/users", keysDir), 0o700)
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
		err = os.WriteFile(fmt.Sprintf("%s/%s", keysDir, keyFile), input, 0o600)
		if err != nil {
			t.Fatalf("Failed copying key file %s to temp test dir, error: %s", keyFile, err)
		}
	}
}

func createHibaKeysGen(t *testing.T, hibaCa, hibaGen, keysDir string) {
	caCmd := exec.Command(
		hibaCa,
		"-c",
		"-d", keysDir, // output to the temp dir
		"--",           // pass the rest to ssh-keygen
		"-q", "-N", "", // quiet, empty passphrase

	)
	err := caCmd.Run()
	if err != nil {
		t.Fatalf("Failed generating ca key pair, error: %s", err)
	}

	userKeyCmd := exec.Command(
		hibaCa,
		"-c",
		"-d", keysDir,
		"-u", "-I", userKey,
		"--",
		"-q", "-N", "",
	)
	err = userKeyCmd.Run()
	if err != nil {
		t.Fatalf("Failed generating user key pair, error: %s", err)
	}

	dutKeyCmd := exec.Command(
		hibaCa,
		"-c",
		"-d", keysDir,
		"-h", "-I", dutKey,
		"--",
		"-q", "-N", "",
	)
	err = dutKeyCmd.Run()
	if err != nil {
		t.Fatalf("Failed generating dut key pair, error: %s", err)
	}

	prodIdentityCmd := exec.Command(
		hibaGen,
		"-i",
		"-f", fmt.Sprintf("%s/policy/identities/prod", keysDir),
		"domain", "google.com",
	)
	err = prodIdentityCmd.Run()
	if err != nil {
		t.Fatalf("Failed creating prod identity, error: %s", err)
	}

	shellGrantCmd := exec.Command(
		hibaGen,
		"-f", fmt.Sprintf("%s/policy/grants/shell", keysDir),
		"domain", "google.com",
	)
	err = shellGrantCmd.Run()
	if err != nil {
		t.Fatalf("Failed creating shell grant, error: %s", err)
	}

	grantShellToUserCmd := exec.Command(
		hibaCa,
		"-d", keysDir,
		"-p",
		"-I", userKey,
		"-H", "shell",
	)
	err = grantShellToUserCmd.Run()
	if err != nil {
		t.Fatalf("Failed granting shell grant to testuser, error: %s", err)
	}

	createHostCertCmd := exec.Command(
		hibaCa,
		"-d", keysDir,
		"-s",
		"-h",
		"-I", dutKey,
		"-H", "prod",
		"-V", "+52w",
	)
	err = createHostCertCmd.Run()
	if err != nil {
		t.Fatalf("Failed creating host certificate, error: %s", err)
	}

	createUserCertCmd := exec.Command(
		hibaCa,
		"-d", keysDir,
		"-s",
		"-u",
		"-I", userKey,
		"-H", "shell",
	)
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
func CreateHibaKeys(t *testing.T, dut *ondatra.DUTDevice, keysDir string) {
	hibaCa, _ := exec.LookPath("hiba-ca.sh")
	hibaGen, _ := exec.LookPath("hiba-gen")
	if hibaCa == "" || hibaGen == "" {
		t.Log("hiba-ca and/or hiba-gen not found on path, will try to use certs in local test dir if present.")
		createHibaKeysCopy(t, keysDir)
	} else {
		createHibaKeysGen(t, hibaCa, hibaGen, keysDir)
	}
}

// SSHWithPassword dials ssh with password based authentication to be used in credentialz tests.
func SSHWithPassword(ctx context.Context, dut *ondatra.DUTDevice, username, password string) (binding.SSHClient, error) {
	return dut.RawAPIs().BindingDUT().DialSSH(ctx, binding.PasswordAuth{User: username, Password: password})
}

// SSHWithCertificate dials ssh with user certificate to be used in credentialz tests.
func SSHWithCertificate(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, username, dir string) (binding.SSHClient, error) {
	privateKeyContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, userKey))
	if err != nil {
		t.Fatalf("Failed reading private key contents, error: %s", err)
	}

	certificateContents, err := os.ReadFile(fmt.Sprintf("%s/%s-cert.pub", dir, userKey))
	if err != nil {
		t.Fatalf("Failed reading certificate contents, error: %s", err)
	}

	return dut.RawAPIs().BindingDUT().DialSSH(ctx, binding.CertificateAuth{User: username, PrivateKey: privateKeyContents, Certificate: certificateContents})
}

// SSHWithKey dials ssh with key based authentication to be used in credentialz tests.
func SSHWithKey(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, username, dir string) (binding.SSHClient, error) {
	privateKeyContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, userKey))
	if err != nil {
		t.Fatalf("Failed reading private key contents, error: %s", err)
	}
	return dut.RawAPIs().BindingDUT().DialSSH(ctx, binding.KeyAuth{User: username, Key: privateKeyContents})
}

// SSHCleanup performs required cleanup on DUT.
func SSHCleanup(t *testing.T, dut *ondatra.DUTDevice) {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		t.Logf("Arista vendor, performing SSH cleanup")
		cliConfig := `no management ssh`
		helpers.GnmiCLIConfig(t, dut, cliConfig)
	default:
		t.Logf("Need cleanup support from Vendor %s", dut.Vendor())
	}
}

// GetConfiguredHostKey returns the configured host key on the DUT for the given algorithm.
// fqdn is used for logging/diagnostic context when locating the host key.
func GetConfiguredHostKey(t *testing.T, dut *ondatra.DUTDevice, algo string, fqdn string) string {
	t.Helper()
	t.Logf("Looking up configured host key for algo %q (fqdn: %q)", algo, fqdn)
	credzClient := dut.RawAPIs().GNSI(t).Credentialz()

	// Polling is required because the host key might not be immediately available after rotation.
	var matchingKey string
	var lastErr error
	var response *cpb.GetPublicKeysResponse
	for i := 0; i < 10; i++ {
		var err error
		response, err = credzClient.GetPublicKeys(context.Background(), &cpb.GetPublicKeysRequest{})
		if err != nil {
			lastErr = err
			t.Logf("Waiting for credentialz public keys (attempt %d/10): %v", i+1, err)
			time.Sleep(5 * time.Second)
			continue
		}
		for _, pk := range response.PublicKeys {
			keyAlgo := sshAlgo(t, pk)
			if keyAlgo == algo {
				matchingKey = sshKey(t, pk)
				break
			}
		}
		if matchingKey != "" {
			break
		}
		t.Logf("Waiting for %s host key (attempt %d/10) for fqdn %q", algo, i+1, fqdn)
		time.Sleep(5 * time.Second)
	}

	if matchingKey == "" {
		if lastErr != nil {
			t.Logf("Failed to get public keys from DUT: %v", lastErr)
		}
		if response != nil {
			t.Logf("Available public keys: %+v", response.PublicKeys)
		} else {
			t.Fatalf("Failed to find host key for algorithm %s (fqdn %q) on DUT. Available keys and their types can be inspected via logs.", algo, fqdn)
		}
	}

	return algo + " " + matchingKey
}

func keyTypeFromAlgo(algo string) cpb.KeyType {
	switch algo {
	case "ssh-rsa":
		return cpb.KeyType_KEY_TYPE_RSA_4096
	case "ecdsa-sha2-nistp256":
		return cpb.KeyType_KEY_TYPE_ECDSA_P_256
	case "ecdsa-sha2-nistp384":
		return cpb.KeyType_KEY_TYPE_ECDSA_P_384
	case "ecdsa-sha2-nistp521":
		return cpb.KeyType_KEY_TYPE_ECDSA_P_521
	case "ssh-ed25519":
		return cpb.KeyType_KEY_TYPE_ED25519
	default:
		return cpb.KeyType_KEY_TYPE_UNSPECIFIED
	}
}

func sshAlgo(t *testing.T, pk *cpb.PublicKey) string {
	keyType := pk.KeyType
	switch keyType {
	case cpb.KeyType_KEY_TYPE_RSA_2048, cpb.KeyType_KEY_TYPE_RSA_4096, cpb.KeyType_KEY_TYPE_RSA_3072:
		return "ssh-rsa"
	case cpb.KeyType_KEY_TYPE_ECDSA_P_256:
		return "ecdsa-sha2-nistp256"
	case cpb.KeyType_KEY_TYPE_ECDSA_P_384:
		return "ecdsa-sha2-nistp384"
	case cpb.KeyType_KEY_TYPE_ECDSA_P_521:
		return "ecdsa-sha2-nistp521"
	case cpb.KeyType_KEY_TYPE_ED25519:
		return "ssh-ed25519"
	case cpb.KeyType_KEY_TYPE_UNSPECIFIED:
		// Attempt to infer from public key content.
		keyData := string(pk.PublicKey)
		parts := strings.Fields(keyData)
		if len(parts) >= 1 && (strings.HasPrefix(parts[0], "ssh-") || strings.HasPrefix(parts[0], "ecdsa-")) {
			return parts[0]
		}
		fallthrough
	default:
		t.Logf("unsupported key type: %v", keyType)
		return ""
	}
}

func sshKey(t *testing.T, key *cpb.PublicKey) string {
	if len(key.PublicKey) == 0 {
		return ""
	}
	keyData := strings.TrimSpace(string(key.PublicKey))
	// Heuristic to check if the key is in binary wire format or base64.
	if !strings.HasPrefix(keyData, "AAAA") && !strings.HasPrefix(keyData, "ssh-") && !strings.Contains(keyData, "ecdsa-") {
		t.Logf("Key is binary, base64 encoding it.")
		keyData = base64.StdEncoding.EncodeToString(key.PublicKey)
	}

	// If the key is already in "algo base64" format, we just want the base64 part.
	parts := strings.Fields(strings.TrimSpace(keyData))
	if len(parts) >= 2 && strings.HasPrefix(parts[0], "ssh-") {
		return parts[1]
	}
	return keyData
}
