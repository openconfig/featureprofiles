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

package vlan_config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const vlanID uint16 = 4090

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

// TestVLANCreateDelete covers creating a VLAN and removing it.
func TestVLANCreateDelete(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ni := deviations.DefaultNetworkInstance(dut)
	vlanPath := gnmi.OC().NetworkInstance(ni).Vlan(vlanID)
	t.Cleanup(func() {
		gnmi.Delete(t, dut, vlanPath.Config())
	})

	t.Log("Create a VLAN under the default network instance")
	gnmi.Replace(t, dut, vlanPath.Config(), &oc.NetworkInstance_Vlan{
		VlanId: ygot.Uint16(vlanID),
	})
	if got := gnmi.Get(t, dut, vlanPath.VlanId().Config()); got != vlanID {
		t.Errorf("vlan-id config = %d, want %d", got, vlanID)
	}
	gnmi.Await(t, dut, vlanPath.VlanId().State(), time.Minute, vlanID)

	t.Log("Delete the VLAN")
	gnmi.Delete(t, dut, vlanPath.Config())
	if gnmi.LookupConfig(t, dut, vlanPath.Config()).IsPresent() {
		t.Errorf("VLAN %d still present in config after delete", vlanID)
	}
}

// TestSwitchedVLAN covers access mode and a trunk native VLAN.
func TestSwitchedVLAN(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ni := deviations.DefaultNetworkInstance(dut)
	port := ethernetInterface(t, dut)
	vlanPath := gnmi.OC().NetworkInstance(ni).Vlan(vlanID)
	switched := gnmi.OC().Interface(port).Ethernet().SwitchedVlan()

	gnmi.Replace(t, dut, vlanPath.Config(), &oc.NetworkInstance_Vlan{VlanId: ygot.Uint16(vlanID)})
	t.Cleanup(func() {
		gnmi.Delete(t, dut, switched.Config())
		gnmi.Delete(t, dut, vlanPath.Config())
	})

	t.Log("Put the port in access mode")
	access := &oc.Interface_Ethernet_SwitchedVlan{}
	access.SetInterfaceMode(oc.Vlan_VlanModeType_ACCESS)
	access.SetAccessVlan(vlanID)
	gnmi.Replace(t, dut, switched.Config(), access)
	if got := gnmi.Get(t, dut, switched.InterfaceMode().Config()); got != oc.Vlan_VlanModeType_ACCESS {
		t.Errorf("interface-mode config = %v, want ACCESS", got)
	}
	if got := gnmi.Get(t, dut, switched.AccessVlan().Config()); got != vlanID {
		t.Errorf("access-vlan config = %d, want %d", got, vlanID)
	}
	gnmi.Await(t, dut, switched.AccessVlan().State(), time.Minute, vlanID)

	t.Log("Move the port to trunk mode and set the native VLAN")
	trunk := &oc.Interface_Ethernet_SwitchedVlan{}
	trunk.SetInterfaceMode(oc.Vlan_VlanModeType_TRUNK)
	trunk.SetNativeVlan(vlanID)
	gnmi.Replace(t, dut, switched.Config(), trunk)
	if got := gnmi.Get(t, dut, switched.InterfaceMode().Config()); got != oc.Vlan_VlanModeType_TRUNK {
		t.Errorf("interface-mode config = %v, want TRUNK", got)
	}
	if got := gnmi.Get(t, dut, switched.NativeVlan().Config()); got != vlanID {
		t.Errorf("native-vlan config = %d, want %d", got, vlanID)
	}
	gnmi.Await(t, dut, switched.NativeVlan().State(), time.Minute, vlanID)
}
