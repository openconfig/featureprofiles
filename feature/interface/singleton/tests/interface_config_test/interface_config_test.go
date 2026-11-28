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

package interface_config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func ethernetInterface(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	for _, intf := range gnmi.GetAll(t, dut, gnmi.OC().InterfaceAny().State()) {
		name := intf.GetName()
		if strings.HasPrefix(name, "Ethernet") {
			return name
		}
	}
	t.Fatal("no Ethernet interface in state")
	return ""
}

// TestInterfaceLeaves covers enabled, description, MTU, and type.
func TestInterfaceLeaves(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	name := ethernetInterface(t, dut)
	iface := gnmi.OC().Interface(name)

	enabled := gnmi.Get(t, dut, iface.Enabled().Config())
	mtu := gnmi.Get(t, dut, iface.Mtu().Config())
	ifType := gnmi.Get(t, dut, iface.Type().Config())
	desc, hadDesc := gnmi.LookupConfig(t, dut, iface.Description().Config()).Val()
	t.Cleanup(func() {
		gnmi.Replace(t, dut, iface.Enabled().Config(), enabled)
		gnmi.Replace(t, dut, iface.Mtu().Config(), mtu)
		gnmi.Replace(t, dut, iface.Type().Config(), ifType)
		if hadDesc {
			gnmi.Replace(t, dut, iface.Description().Config(), desc)
			return
		}
		gnmi.Delete(t, dut, iface.Description().Config())
	})

	wantEnabled := !enabled
	wantMTU := uint16(1500)
	if mtu == wantMTU {
		wantMTU = 9216
	}
	t.Log("Replace enabled, description, MTU, and type")
	gnmi.Replace(t, dut, iface.Enabled().Config(), wantEnabled)
	gnmi.Replace(t, dut, iface.Description().Config(), "featureprofiles interface config")
	gnmi.Replace(t, dut, iface.Mtu().Config(), wantMTU)
	gnmi.Replace(t, dut, iface.Type().Config(), ifType)

	if got := gnmi.Get(t, dut, iface.Enabled().Config()); got != wantEnabled {
		t.Errorf("enabled config = %v, want %v", got, wantEnabled)
	}
	if got := gnmi.Get(t, dut, iface.Description().Config()); got != "featureprofiles interface config" {
		t.Errorf("description config = %q", got)
	}
	if got := gnmi.Get(t, dut, iface.Mtu().Config()); got != wantMTU {
		t.Errorf("mtu config = %d, want %d", got, wantMTU)
	}
	if got := gnmi.Get(t, dut, iface.Type().Config()); got != ifType {
		t.Errorf("type config = %v, want %v", got, ifType)
	}
	gnmi.Await(t, dut, iface.Enabled().State(), time.Minute, wantEnabled)
	gnmi.Await(t, dut, iface.Mtu().State(), time.Minute, wantMTU)
}

// TestInterfaceAddresses covers one IPv6 address and two IPv4 addresses.
func TestInterfaceAddresses(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	name := ethernetInterface(t, dut)
	sub := gnmi.OC().Interface(name).Subinterface(0)

	v6 := "2001:db8:8:1::1"
	v4 := []string{"192.0.2.10", "192.0.2.11"}
	t.Cleanup(func() {
		gnmi.Delete(t, dut, sub.Ipv6().Address(v6).Config())
		for _, addr := range v4 {
			gnmi.Delete(t, dut, sub.Ipv4().Address(addr).Config())
		}
	})

	t.Log("Create an IPv6 address and two IPv4 addresses on subinterface 0")
	ip6 := &oc.Interface_Subinterface_Ipv6_Address{Ip: ygot.String(v6), PrefixLength: ygot.Uint8(64)}
	gnmi.Replace(t, dut, sub.Ipv6().Address(v6).Config(), ip6)
	for _, addr := range v4 {
		ip4 := &oc.Interface_Subinterface_Ipv4_Address{Ip: ygot.String(addr), PrefixLength: ygot.Uint8(24)}
		gnmi.Replace(t, dut, sub.Ipv4().Address(addr).Config(), ip4)
	}

	if got := gnmi.Get(t, dut, sub.Ipv6().Address(v6).Ip().Config()); got != v6 {
		t.Errorf("ipv6 address config = %q, want %q", got, v6)
	}
	if got := gnmi.Get(t, dut, sub.Ipv6().Address(v6).PrefixLength().Config()); got != 64 {
		t.Errorf("ipv6 prefix-length config = %d, want 64", got)
	}
	for _, addr := range v4 {
		if got := gnmi.Get(t, dut, sub.Ipv4().Address(addr).Ip().Config()); got != addr {
			t.Errorf("ipv4 address config = %q, want %q", got, addr)
		}
	}
	gnmi.Await(t, dut, sub.Ipv6().Address(v6).PrefixLength().State(), time.Minute, uint8(64))

	t.Log("Delete the addresses")
	gnmi.Delete(t, dut, sub.Ipv6().Address(v6).Config())
	if gnmi.LookupConfig(t, dut, sub.Ipv6().Address(v6).Config()).IsPresent() {
		t.Errorf("ipv6 address %s still in config", v6)
	}
}
