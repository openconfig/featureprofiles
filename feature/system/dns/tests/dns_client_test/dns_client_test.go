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

package dns_client_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygnmi/ygnmi"
)

const stateTimeout = time.Minute

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// restoreDNS puts /system/dns back to what the DUT had before the test.
func restoreDNS(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	dnsPath := gnmi.OC().System().Dns()
	original, present := gnmi.LookupConfig(t, dut, dnsPath.Config()).Val()
	t.Cleanup(func() {
		if present {
			gnmi.Replace(t, dut, dnsPath.Config(), original)
			return
		}
		gnmi.Delete(t, dut, dnsPath.Config())
	})
}

// serverAddresses returns the DNS servers in list order. The list is
// ordered-by user because resolvers try the servers in that order.
func serverAddresses(dns *oc.System_Dns) []string {
	if dns.Server == nil {
		return []string{}
	}
	return dns.Server.Keys()
}

// awaitServerState waits until the state server list for DNS equals want, in
// order.
func awaitServerState(t *testing.T, dut *ondatra.DUTDevice, want []string) {
	t.Helper()
	_, ok := gnmi.Watch(t, dut, gnmi.OC().System().Dns().State(), stateTimeout, func(v *ygnmi.Value[*oc.System_Dns]) bool {
		dns, present := v.Val()
		if !present {
			return len(want) == 0
		}
		return cmp.Equal(serverAddresses(dns), want)
	}).Await(t)
	if !ok {
		got := []string{}
		if dns, present := gnmi.Lookup(t, dut, gnmi.OC().System().Dns().State()).Val(); present {
			got = serverAddresses(dns)
		}
		t.Errorf("DNS server state: got %v, want %v", got, want)
	}
}

// awaitServerPresence waits until address is, or is not, in the DNS state
// server list.
func awaitServerPresence(t *testing.T, dut *ondatra.DUTDevice, address string, want bool) {
	t.Helper()
	_, ok := gnmi.Watch(t, dut, gnmi.OC().System().Dns().State(), stateTimeout, func(v *ygnmi.Value[*oc.System_Dns]) bool {
		dns, present := v.Val()
		return (present && dns.GetServer(address) != nil) == want
	}).Await(t)
	if !ok {
		t.Errorf("DNS server %s in state: got %v, want %v", address, !want, want)
	}
}

// deleteServer removes one entry from the ordered DNS server list. ygnmi has
// no per-key path for ordered lists, so the delete is sent as a raw gNMI Set.
func deleteServer(t *testing.T, dut *ondatra.DUTDevice, address string) {
	t.Helper()
	p := &gpb.Path{
		Origin: "openconfig",
		Elem: []*gpb.PathElem{
			{Name: "system"},
			{Name: "dns"},
			{Name: "servers"},
			{Name: "server", Key: map[string]string{"address": address}},
		},
	}
	if _, err := dut.RawAPIs().GNMI(t).Set(context.Background(), &gpb.SetRequest{Delete: []*gpb.Path{p}}); err != nil {
		t.Fatalf("delete DNS server %s: %v", address, err)
	}
}

// TestDNSServerCreateDelete covers System-1.6.1.
func TestDNSServerCreateDelete(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	restoreDNS(t, dut)
	dnsPath := gnmi.OC().System().Dns()

	const address = "192.0.2.53"

	t.Log("Create one IPv4 DNS server")
	gnmi.Update(t, dut, dnsPath.Config(), dnsWithServers(t, nil, address))
	if got := gnmi.Get(t, dut, dnsPath.Config()).GetServer(address).GetAddress(); got != address {
		t.Errorf("DNS server config address: got %q, want %q", got, address)
	}
	awaitServerPresence(t, dut, address, true)

	t.Log("Delete the DNS server")
	deleteServer(t, dut, address)
	if dns, present := gnmi.LookupConfig(t, dut, dnsPath.Config()).Val(); present && dns.GetServer(address) != nil {
		t.Errorf("DNS server %s is still in config after delete", address)
	}
	awaitServerPresence(t, dut, address, false)
}

// TestDNSServerList covers System-1.6.2.
func TestDNSServerList(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	restoreDNS(t, dut)

	servers := []struct {
		address string
		port    uint16
	}{
		{"192.0.2.53", 53},
		{"192.0.2.54", 53},
		{"2001:db8::53", 53},
		{"2001:db8::54", 5353},
	}

	t.Log("Replace the DNS server list with IPv4 and IPv6 servers and ports")
	dns := &oc.System_Dns{}
	var want []string
	for _, s := range servers {
		srv, err := dns.AppendNewServer(s.address)
		if err != nil {
			t.Fatalf("AppendNewServer(%s): %v", s.address, err)
		}
		srv.SetPort(s.port)
		want = append(want, s.address)
	}
	gnmi.Replace(t, dut, gnmi.OC().System().Dns().ServerMap().Config(), dns.Server)

	got := gnmi.Get(t, dut, gnmi.OC().System().Dns().Config())
	if diff := cmp.Diff(want, serverAddresses(got)); diff != "" {
		t.Errorf("DNS server config, in order (-want +got):\n%s", diff)
	}
	for _, s := range servers {
		if p := got.GetServer(s.address).GetPort(); p != s.port {
			t.Errorf("DNS server %s config port: got %d, want %d", s.address, p, s.port)
		}
	}
	awaitServerState(t, dut, want)
	state := gnmi.Get(t, dut, gnmi.OC().System().Dns().State())
	for _, s := range servers {
		if p := state.GetServer(s.address).GetPort(); p != s.port {
			t.Errorf("DNS server %s state port: got %d, want %d", s.address, p, s.port)
		}
	}
}

// TestDNSSearchList covers System-1.6.3.
func TestDNSSearchList(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	restoreDNS(t, dut)
	search := gnmi.OC().System().Dns().Search()

	for _, tc := range []struct {
		desc string
		want []string
	}{
		{"Two search domains", []string{"example.test", "lab.example.test"}},
		{"One search domain", []string{"example.test"}},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			gnmi.Replace(t, dut, search.Config(), tc.want)
			if diff := cmp.Diff(tc.want, gnmi.Get(t, dut, search.Config())); diff != "" {
				t.Errorf("DNS search config (-want +got):\n%s", diff)
			}
			_, ok := gnmi.Watch(t, dut, search.State(), stateTimeout, func(v *ygnmi.Value[[]string]) bool {
				got, present := v.Val()
				return present && cmp.Equal(got, tc.want)
			}).Await(t)
			if !ok {
				t.Errorf("DNS search state did not become %v", tc.want)
			}
		})
	}

	t.Run("Delete search list", func(t *testing.T) {
		gnmi.Delete(t, dut, search.Config())
		if got, present := gnmi.LookupConfig(t, dut, search.Config()).Val(); present && len(got) != 0 {
			t.Errorf("DNS search config after delete: got %v, want empty", got)
		}
	})
}

// TestDNSReplace covers System-1.6.4.
func TestDNSReplace(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	restoreDNS(t, dut)
	dnsPath := gnmi.OC().System().Dns()

	for _, step := range []struct {
		desc    string
		servers []string
	}{
		{"Replace /system/dns with two servers and a search domain", []string{"192.0.2.53", "2001:db8::53"}},
		{"Replace with the same servers in reverse order", []string{"2001:db8::53", "192.0.2.53"}},
		{"Replace with a different server; the earlier servers must be removed", []string{"192.0.2.55"}},
	} {
		t.Log(step.desc)
		gnmi.Replace(t, dut, dnsPath.Config(), dnsWithServers(t, []string{"example.test"}, step.servers...))
		if diff := cmp.Diff(step.servers, serverAddresses(gnmi.Get(t, dut, dnsPath.Config()))); diff != "" {
			t.Errorf("DNS server config after %q (-want +got):\n%s", step.desc, diff)
		}
		awaitServerState(t, dut, step.servers)
	}
}

func dnsWithServers(t *testing.T, search []string, addresses ...string) *oc.System_Dns {
	t.Helper()
	dns := &oc.System_Dns{Search: search}
	for _, addr := range addresses {
		if _, err := dns.AppendNewServer(addr); err != nil {
			t.Fatalf("AppendNewServer(%s): %v", addr, err)
		}
	}
	return dns
}
