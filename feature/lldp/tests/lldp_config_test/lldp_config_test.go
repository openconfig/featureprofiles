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

package lldp_config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
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

// TestLLDPGlobalConfig covers enabled, system-name, hello-timer, and the
// management interface used for the Management Address TLV.
func TestLLDPGlobalConfig(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	lldp := gnmi.OC().Lldp()
	port := ethernetInterface(t, dut)

	enabled, hadEnabled := gnmi.LookupConfig(t, dut, lldp.Enabled().Config()).Val()
	name, hadName := gnmi.LookupConfig(t, dut, lldp.SystemName().Config()).Val()
	timer, hadTimer := gnmi.LookupConfig(t, dut, lldp.HelloTimer().Config()).Val()
	mgmt, hadMgmt := gnmi.LookupConfig(t, dut, lldp.ManagementInterface().Config()).Val()
	t.Cleanup(func() {
		restoreBool(t, dut, hadEnabled, enabled, func(v bool) { gnmi.Replace(t, dut, lldp.Enabled().Config(), v) }, func() { gnmi.Delete(t, dut, lldp.Enabled().Config()) })
		restoreString(t, dut, hadName, name, func(v string) { gnmi.Replace(t, dut, lldp.SystemName().Config(), v) }, func() { gnmi.Delete(t, dut, lldp.SystemName().Config()) })
		restoreUint(t, dut, hadTimer, timer, func(v uint64) { gnmi.Replace(t, dut, lldp.HelloTimer().Config(), v) }, func() { gnmi.Delete(t, dut, lldp.HelloTimer().Config()) })
		restoreString(t, dut, hadMgmt, mgmt, func(v string) { gnmi.Replace(t, dut, lldp.ManagementInterface().Config(), v) }, func() { gnmi.Delete(t, dut, lldp.ManagementInterface().Config()) })
	})

	t.Log("Enable LLDP, set the system name, hello timer, and management interface")
	gnmi.Replace(t, dut, lldp.Enabled().Config(), true)
	gnmi.Replace(t, dut, lldp.SystemName().Config(), "fp-lldp")
	gnmi.Replace(t, dut, lldp.HelloTimer().Config(), uint64(30))
	gnmi.Replace(t, dut, lldp.ManagementInterface().Config(), port)

	if got := gnmi.Get(t, dut, lldp.Enabled().Config()); !got {
		t.Error("LLDP enabled config = false, want true")
	}
	if got := gnmi.Get(t, dut, lldp.SystemName().Config()); got != "fp-lldp" {
		t.Errorf("system-name config = %q, want fp-lldp", got)
	}
	if got := gnmi.Get(t, dut, lldp.HelloTimer().Config()); got != 30 {
		t.Errorf("hello-timer config = %d, want 30", got)
	}
	gnmi.Await(t, dut, lldp.Enabled().State(), time.Minute, true)
	gnmi.Await(t, dut, lldp.SystemName().State(), time.Minute, "fp-lldp")
	gnmi.Await(t, dut, lldp.HelloTimer().State(), time.Minute, uint64(30))
	gnmi.Await(t, dut, lldp.ManagementInterface().State(), time.Minute, port)
}

// TestLLDPInterfaceEnabled covers the per-interface enabled leaf.
func TestLLDPInterfaceEnabled(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	port := ethernetInterface(t, dut)
	leaf := gnmi.OC().Lldp().Interface(port).Enabled()
	original, present := gnmi.LookupConfig(t, dut, leaf.Config()).Val()
	t.Cleanup(func() {
		if present {
			gnmi.Replace(t, dut, leaf.Config(), original)
			return
		}
		gnmi.Delete(t, dut, leaf.Config())
	})

	t.Log("Enable LLDP on one Ethernet interface")
	gnmi.Replace(t, dut, leaf.Config(), true)
	if got := gnmi.Get(t, dut, leaf.Config()); !got {
		t.Error("interface enabled config = false, want true")
	}
	gnmi.Await(t, dut, leaf.State(), time.Minute, true)
}

func restoreBool(t *testing.T, dut *ondatra.DUTDevice, present bool, v bool, set func(bool), del func()) {
	t.Helper()
	if present {
		set(v)
		return
	}
	del()
}

func restoreString(t *testing.T, dut *ondatra.DUTDevice, present bool, v string, set func(string), del func()) {
	t.Helper()
	if present {
		set(v)
		return
	}
	del()
}

func restoreUint(t *testing.T, dut *ondatra.DUTDevice, present bool, v uint64, set func(uint64), del func()) {
	t.Helper()
	if present {
		set(v)
		return
	}
	del()
}
