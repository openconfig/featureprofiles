// Copyright 2026 Google LLC
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

package remote_syslog_config_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	host    = "192.0.2.51"
	profile = "SYSLOG-TLS"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestRemoteSyslog covers host, port, and the TLS leaves on a remote server.
func TestRemoteSyslog(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	serverPath := gnmi.OC().System().Logging().RemoteServer(host)
	t.Cleanup(func() {
		gnmi.Delete(t, dut, serverPath.Config())
	})

	t.Log("Create a remote syslog server with TLS")
	server := &oc.System_Logging_RemoteServer{
		Host: ygot.String(host),
	}
	server.SetRemotePort(6514)
	server.SetTransportSecurity(true)
	server.SetTlsProfileId(profile)
	gnmi.Replace(t, dut, serverPath.Config(), server)

	got := gnmi.Get(t, dut, serverPath.Config())
	if got.GetHost() != host {
		t.Errorf("host config = %q, want %q", got.GetHost(), host)
	}
	if got.GetRemotePort() != 6514 {
		t.Errorf("remote-port config = %d, want 6514", got.GetRemotePort())
	}
	if !got.GetTransportSecurity() {
		t.Error("transport-security config = false, want true")
	}
	if got.GetTlsProfileId() != profile {
		t.Errorf("tls-profile-id config = %q, want %q", got.GetTlsProfileId(), profile)
	}
	gnmi.Await(t, dut, serverPath.TransportSecurity().State(), time.Minute, true)
	gnmi.Await(t, dut, serverPath.TlsProfileId().State(), time.Minute, profile)

	t.Log("Delete the remote syslog server")
	gnmi.Delete(t, dut, serverPath.Config())
	if gnmi.LookupConfig(t, dut, serverPath.Config()).IsPresent() {
		t.Errorf("remote server %s still in config", host)
	}
}
