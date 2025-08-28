// Copyright 2025 Google LLC
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

package cfgplugins

import (
	"fmt"
	"testing"

	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

// Consts
const (
	IPv4 = "IPv4"
	IPv6 = "IPv6"
)

// Vrf struct
type VrfRule struct {
	Index        uint32
	IpType       string
	SourcePrefix string
	PrefixLength uint8
	NetInstName  string
}

func EnableDefaultNetworkInstanceBgp(t *testing.T, dut *ondatra.DUTDevice, dutAS uint32) {
	d := gnmi.OC()
	bgp := &oc.NetworkInstance_Protocol{
		Identifier: oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP,
		Name:       ygot.String("BGP"),
		Enabled:    ygot.Bool(true),
		Bgp:        &oc.NetworkInstance_Protocol_Bgp{},
	}

	bgp.Bgp.Global = &oc.NetworkInstance_Protocol_Bgp_Global{
		As: ygot.Uint32(dutAS),
	}

	gnmi.Replace(t, dut, d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), bgp)
}

// ConfigureNetworkInstance configures a new network instance on the DUT not using batch update
func ConfigureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice, netInstName string, isDefault bool) *oc.NetworkInstance {
	t.Logf("Creating new Network Instance: %s", netInstName)
	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(netInstName)

	if isDefault {
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	} else {
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	}

	return ni
}

// UpdateNetworkInstanceOnDut updates the network instance on the DUT. Not using batch update
func UpdateNetworkInstanceOnDut(t *testing.T, dut *ondatra.DUTDevice, netInstName string, netInst *oc.NetworkInstance) {
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(netInstName).Config(), netInst)
}

// ConfigureDefaultNetworkInstance configures the default network instance name and type.
func NewDefaultNetworkInstance(t testing.TB, batch *gnmi.SetBatch, d *ondatra.DUTDevice) {
	defNiPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(d))
	gnmi.BatchUpdate(batch, defNiPath.Config(), &oc.NetworkInstance{
		Name: ygot.String(deviations.DefaultNetworkInstance(d)),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
	})
}

// ConfigureCustomNetworkInstance configures a non-default network instance name and type.
func NewCustomNetworkInstance(t testing.TB, batch *gnmi.SetBatch, d *ondatra.DUTDevice, ni string) {
	defNiPath := gnmi.OC().NetworkInstance(ni)
	gnmi.BatchUpdate(batch, defNiPath.Config(), &oc.NetworkInstance{
		Name: ygot.String(ni),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
	})

}

// AssignToNetworkInstance attaches a subinterface to a network instance.
func AssignToNetworkInstance(t testing.TB, d *ondatra.DUTDevice, i string, ni string, si uint32) {
	t.Helper()
	if ni == "" {
		t.Fatalf("Network instance not provided for interface assignment")
	}
	netInst := &oc.NetworkInstance{Name: ygot.String(ni)}
	intf := &oc.Interface{Name: ygot.String(i)}
	netInstIntf, err := netInst.NewInterface(intf.GetName())
	if err != nil {
		t.Errorf("Error fetching NewInterface for %s", intf.GetName())
	}
	netInstIntf.Interface = ygot.String(intf.GetName())
	netInstIntf.Subinterface = ygot.Uint32(si)
	if deviations.InterfaceRefInterfaceIDFormat(d) {
		netInstIntf.Id = ygot.String(fmt.Sprintf("%s.%d", intf.GetName(), si))
	} else {
		netInstIntf.Id = ygot.String(intf.GetName())
	}
	if intf.GetOrCreateSubinterface(si) != nil {
		gnmi.Update(t, d, gnmi.OC().NetworkInstance(ni).Config(), netInst)
	}
}

// AssignInterfaceToNetworkInstance attaches an interface to a network instance using batch update.
// This is required for vendors that do not support subinterfaces and only support interface to network instance assignment.
func AssignInterfaceToNetworkInstance(batch *gnmi.SetBatch, t testing.TB, d *ondatra.DUTDevice, i string, ni string, si uint32) {
	t.Helper()
	if ni == "" {
		t.Fatalf("Network instance not provided for interface assignment")
	}
	netInst := &oc.NetworkInstance{Name: ygot.String(ni)}
	intf := &oc.Interface{Name: ygot.String(i)}
	netInstIntf, err := netInst.NewInterface(intf.GetName())
	if err != nil {
		t.Errorf("Error fetching NewInterface for %s", intf.GetName())
	}
	netInstIntf.Interface = ygot.String(intf.GetName())
	netInstIntf.Subinterface = ygot.Uint32(si)
	switch d.Vendor() {
	case ondatra.ARISTA:
		netInstIntf.Id = ygot.String(intf.GetName())
	case ondatra.CISCO:
		netInstIntf.Id = ygot.String(intf.GetName())
	case ondatra.NOKIA:
		netInstIntf.Id = ygot.String(intf.GetName())
	case ondatra.JUNIPER:
		netInstIntf.Id = ygot.String(intf.GetName() + "." + fmt.Sprint(si))
	default:
		netInstIntf.Id = ygot.String(intf.GetName() + "." + fmt.Sprint(si))
	}
	if intf.GetOrCreateSubinterface(si) != nil {
		gnmi.BatchReplace(batch, gnmi.OC().NetworkInstance(ni).Config(), netInst)
	}
}
