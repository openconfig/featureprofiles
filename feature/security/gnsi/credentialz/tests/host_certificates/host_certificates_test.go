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

package host_certificates_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/openconfig/ondatra/gnmi"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/gnsi/credentialz"
	tpb "github.com/openconfig/kne/proto/topo"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/binding"
	"golang.org/x/crypto/ssh"
)

const (
	dutPublicKeyFilename     = "dut.pub"
	dutCertFilename          = "dut-cert.pub"
	hostCertificateVersion   = "v1.0"
	hostCertificateCreatedOn = 1705962293
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func prepareCaAndHostKeys(t *testing.T, dir string) {
	caCmd := exec.Command(
		"ssh-keygen",
		"-t", "ed25519",
		"-f", "dut",
		"-C", "dut",
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
		"-f", "ca",
		"-C", "ca",
		"-q", "-N", "",
	)
	dutCmd.Dir = dir

	err = dutCmd.Run()
	if err != nil {
		t.Fatalf("failed generating dut key pair, error: %s", err)
	}
}

func rotateHostParameters(
	t *testing.T,
	dut *ondatra.DUTDevice,
	artifacts []*credentialz.ServerKeysRequest_AuthenticationArtifacts,
) {
	credzClient := dut.RawAPIs().GNSI(t).Credentialz()

	credzRotateClient, err := credzClient.RotateHostParameters(context.Background())
	if err != nil {
		t.Fatalf("failed fetching credentialz rotate host parameters client, error: %s", err)
	}

	request := &credentialz.RotateHostParametersRequest{
		Request: &credentialz.RotateHostParametersRequest_ServerKeys{
			ServerKeys: &credentialz.ServerKeysRequest{
				AuthArtifacts: artifacts,
				Version:       hostCertificateVersion,
				CreatedOn:     hostCertificateCreatedOn,
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

func prepareDUTKeys(t *testing.T, dut *ondatra.DUTDevice, dir string) {
	dutPrivateKeyContents, err := os.ReadFile(fmt.Sprintf("%s/dut", dir))
	if err != nil {
		t.Fatalf("failed reading host private key, error: %s", err)
	}

	rotateHostParameters(
		t,
		dut,
		[]*credentialz.ServerKeysRequest_AuthenticationArtifacts{
			{
				PrivateKey: dutPrivateKeyContents,
			},
		},
	)
}

func fetchDUTPublicKey(t *testing.T, dut *ondatra.DUTDevice) []byte {
	credzClient := dut.RawAPIs().GNSI(t).Credentialz()

	req := &credentialz.GetPublicKeysRequest{}

	response, err := credzClient.GetPublicKeys(context.Background(), req)
	if err != nil {
		t.Fatalf("failed fetching fetching credentialz public keys, error: %s", err)
	}

	if len(response.PublicKeys) < 1 {
		return nil
	}

	return response.PublicKeys[0].PublicKey
}

func signPublicKeyWithCAKey(t *testing.T, dir string) {
	cmd := exec.Command(
		"ssh-keygen",
		"-s", "ca", // sign using this ca key
		"-I", "dut", // key identity
		"-h",                 // create host (not user) certificate
		"-n", "dut.test.com", // principal(s)
		"-V", "+52w", // validity
		dutPublicKeyFilename,
	)
	cmd.Dir = dir

	err := cmd.Run()
	if err != nil {
		t.Fatalf("failed signing dut public key with ca, error: %s", err)
	}
}

func loadCertificate(t *testing.T, dut *ondatra.DUTDevice, dir string) {
	certificateContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, dutCertFilename))
	if err != nil {
		t.Fatalf("failed reading host signed certificate, error: %s", err)
	}

	rotateHostParameters(
		t,
		dut,
		[]*credentialz.ServerKeysRequest_AuthenticationArtifacts{
			{
				Certificate: certificateContents,
			},
		},
	)
}

func assertReturnedHostKeyIsCorrect(t *testing.T, dir, addr string) {
	certificateContents, err := os.ReadFile(fmt.Sprintf("%s/%s", dir, dutCertFilename))
	if err != nil {
		t.Fatalf("failed reading host signed certificate, error: %s", err)
	}

	hostCertificate, _, _, _, err := ssh.ParseAuthorizedKey(certificateContents)
	if err != nil {
		t.Fatalf("failed parsing host certificate authorized (cert)key: %s", err)
	}

	var failed bool

	_, err = ssh.Dial(
		"tcp",
		fmt.Sprintf("%s:22", addr),
		&ssh.ClientConfig{
			User: "admin",
			Auth: []ssh.AuthMethod{},
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				if !reflect.DeepEqual(hostCertificate, key) {
					failed = true
				}

				return nil
			},
		},
	)
	if err == nil {
		t.Fatal("dial ssh succeeded, but we expected failure")
	}

	if failed {
		t.Fatalf(
			"host presented key (cert) that does not match expected host certificate",
		)
	}
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

	prepareDUTKeys(t, dut, dir)
	addr := getDutAddr(t, dut)

	keyBytes := fetchDUTPublicKey(t, dut)

	err = os.WriteFile(fmt.Sprintf("%s/%s", dir, dutPublicKeyFilename), keyBytes, 0o777)
	if err != nil {
		t.Fatalf("failed writing dut public key to temp dir, error: %s", err)
	}
	signPublicKeyWithCAKey(t, dir)

	loadCertificate(t, dut, dir)

	// quick sleep to let things percolate
	time.Sleep(15 * time.Second)

	assertTelemetry(t, dut)

	t.Run("dut should return signed host certificate", func(t *testing.T) {
		assertReturnedHostKeyIsCorrect(t, dir, addr)
	})
}
