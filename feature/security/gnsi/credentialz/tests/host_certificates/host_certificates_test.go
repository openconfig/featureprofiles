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

package hostcertificates_test

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/ondatra/gnmi"

	"reflect"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/security/credz"
	"github.com/openconfig/ondatra"
	"golang.org/x/crypto/ssh"
)

const (
	hostCertificateVersion = "v1"
)

var (
	hostCertificateCreatedOn = time.Now().Unix()
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestCredentialz(t *testing.T) {
	hostCertificateVersion1 := fmt.Sprintf("%s-%d", hostCertificateVersion, time.Now().Unix())
	time.Sleep(5 * time.Second)
	hostCertificateVersion2 := fmt.Sprintf("%s-%d", hostCertificateVersion, time.Now().Unix())
	dut := ondatra.DUT(t, "dut")
	target := credz.GetDutTarget(t, dut)

	// Create temporary directory for storing ssh keys/certificates.
	dir, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fatalf("Creating temp dir, err: %s", err)
	}
	defer func(dir string) {
		err = os.RemoveAll(dir)
		if err != nil {
			t.Logf("Error removing temp directory, error: %s", err)
		}
	}(dir)

	// Create ssh keys/certificates for CA & host.
	credz.CreateSSHKeyPair(t, dir, "ca")
	credz.CreateSSHKeyPair(t, dir, "dut")

	credz.RotateAuthenticationArtifacts(t, dut, dir, "", hostCertificateVersion1, uint64(hostCertificateCreatedOn))
	dutKey := credz.GetDutPublicKey(t, dut)
	credz.CreateHostCertificate(t, dir, dutKey)
	credz.RotateAuthenticationArtifacts(t, dut, "", dir, hostCertificateVersion2, uint64(hostCertificateCreatedOn))

	t.Run("dut should return signed host certificate", func(t *testing.T) {
		certificateContents, err := os.ReadFile(fmt.Sprintf("%s/dut-cert.pub", dir))
		if err != nil {
			t.Fatalf("Failed reading host signed certificate, error: %s", err)
		}
		wantHostKey, _, _, _, err := ssh.ParseAuthorizedKey(certificateContents)
		if err != nil {
			t.Fatalf("Failed parsing host certificate authorized (cert)key: %s", err)
		}

		// Verify correct host certificate is returned by the dut.
		_, err = ssh.Dial(
			"tcp",
			target,
			&ssh.ClientConfig{
				User: "admin",
				Auth: []ssh.AuthMethod{},
				HostKeyCallback: func(hostname string, remote net.Addr, gotHostKey ssh.PublicKey) error {
				        //if !cmp.Equal(gotHostKey, wantHostKey) {
					// ***** cmp.Equal function is crashing (panic) while comparing the keys hence used reflect to function to compare *****"
					if !reflect.DeepEqual(gotHostKey, wantHostKey) {
						t.Fatalf("Host presented key (cert) that does not match expected host certificate. got: %v, want: %v", gotHostKey, wantHostKey)
					}
					return nil
				},
			},
		)
		if err == nil {
			t.Fatal("Dial ssh succeeded, but we expected failure.")
		}

		// Verify host certificate telemetry values.
		sshServer := gnmi.Get(t, dut, gnmi.OC().System().SshServer().State())
		gotHostCertificateVersion := sshServer.GetActiveHostCertificateVersion()
		if !cmp.Equal(gotHostCertificateVersion, hostCertificateVersion2) {
			t.Fatalf(
				"Telemetry reports host certificate version is not correct\n\tgot: %s\n\twant: %s",
				gotHostCertificateVersion, hostCertificateVersion2,
			)
		}
		gotHostCertificateCreatedOn := sshServer.GetActiveHostCertificateCreatedOn()
		if !cmp.Equal(time.Unix(0, int64(gotHostCertificateCreatedOn)), time.Unix(0, int64(hostCertificateCreatedOn))) {
			t.Fatalf(
				"Telemetry reports host certificate created on is not correct\n\tgot: %d\n\twant: %d",
				gotHostCertificateCreatedOn, hostCertificateCreatedOn,
			)
		}
	})
}
