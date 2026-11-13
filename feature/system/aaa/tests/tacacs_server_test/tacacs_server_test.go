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

package tacacs_server_test

import (
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
)

const group = "FP-TACACS"

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// TestTACACSServers covers two TACACS servers. No secret key is written.
func TestTACACSServers(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	groupPath := gnmi.OC().System().Aaa().ServerGroup(group)
	t.Cleanup(func() {
		gnmi.Delete(t, dut, groupPath.Config())
	})

	servers := []struct {
		address string
		port    uint16
		timeout uint16
	}{
		{"192.0.2.49", 49, 4},
		{"2001:db8::49", 49, 5},
	}
	t.Log("Create a TACACS server group with two servers")
	sg := &oc.System_Aaa_ServerGroup{}
	sg.SetName(group)
	sg.SetType(oc.AaaTypes_AAA_SERVER_TYPE_TACACS)
	for _, s := range servers {
		srv := sg.GetOrCreateServer(s.address)
		srv.SetAddress(s.address)
		srv.SetTimeout(s.timeout)
		srv.GetOrCreateTacacs().SetPort(s.port)
	}
	gnmi.Replace(t, dut, groupPath.Config(), sg)

	got := gnmi.Get(t, dut, groupPath.Config())
	if got.GetType() != oc.AaaTypes_AAA_SERVER_TYPE_TACACS {
		t.Errorf("server-group type = %v, want TACACS", got.GetType())
	}
	for _, s := range servers {
		srv := got.GetServer(s.address)
		if srv.GetAddress() != s.address {
			t.Errorf("address = %q, want %q", srv.GetAddress(), s.address)
		}
		if srv.GetTimeout() != s.timeout {
			t.Errorf("%s timeout = %d, want %d", s.address, srv.GetTimeout(), s.timeout)
		}
		if srv.GetTacacs().GetPort() != s.port {
			t.Errorf("%s port = %d, want %d", s.address, srv.GetTacacs().GetPort(), s.port)
		}
		gnmi.Await(t, dut, groupPath.Server(s.address).Tacacs().Port().State(), time.Minute, s.port)
	}

	t.Log("Delete the server group")
	gnmi.Delete(t, dut, groupPath.Config())
	if gnmi.LookupConfig(t, dut, groupPath.Config()).IsPresent() {
		t.Errorf("server group %s still in config", group)
	}
}
