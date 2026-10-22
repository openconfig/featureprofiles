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

type NetworkInstanceParams struct {
	Name    string
	Default bool
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
	if deviations.BgpAfiSafiInDefaultNiBeforeOtherNi(dut) {
		bgp.Bgp.Global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
		bgp.Bgp.Global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
		bgp.Bgp.Global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV4_UNICAST).Enabled = ygot.Bool(true)
		bgp.Bgp.Global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_L3VPN_IPV6_UNICAST).Enabled = ygot.Bool(true)
	}

	gnmi.Replace(t, dut, d.NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Config(), bgp)
}

// ConfigureNetworkInstance configures a new network instance on the DUT not using batch update
// TODO: To migrate to AssignInterfaceToNetworkInstance() which uses gnmi.BatchReplace
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

// ConfigureCustomNetworkInstance configures a default or non-default network instance name and type.
func NewNetworkInstance(t testing.TB, d *ondatra.DUTDevice, batch *gnmi.SetBatch, nip *NetworkInstanceParams) {
	t.Helper()
	//ni := nip.Name not required as for default we use deviations.DefaultNetworkInstance(d)
	if nip.Default {
		t.Logf("Creating default network instance: %s", deviations.DefaultNetworkInstance(d))
		defNiPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(d))
		gnmi.BatchUpdate(batch, defNiPath.Config(), &oc.NetworkInstance{
			Name: ygot.String(deviations.DefaultNetworkInstance(d)),
			Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE,
		})
	} else {
		t.Logf("Creating non-default network instance: %s", nip.Name)
		ni := nip.Name
		defNiPath := gnmi.OC().NetworkInstance(ni)
		gnmi.BatchUpdate(batch, defNiPath.Config(), &oc.NetworkInstance{
			Name: ygot.String(ni),
			Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
		})
	}
}

// AssignToNetworkInstance attaches a subinterface to a network instance.
// TODO: To migrate to AssignInterfaceToNetworkInstance() which uses gnmi.BatchReplace
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
func AssignInterfaceToNetworkInstance(t testing.TB, batch *gnmi.SetBatch, d *ondatra.DUTDevice, i string, nip *NetworkInstanceParams, si uint32, subInt ...bool) {
	var ni string
	if nip.Default {
		ni = deviations.DefaultNetworkInstance(d)
	} else {
		ni = nip.Name
	}
	if ni == "" {
		t.Fatalf("Network instance not provided for interface assignment")
	}
	t.Logf("Assigning interface %s to network instance %s", i, ni)
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
		if deviations.InterfaceConfigVRFBeforeAddress(d) && (len(subInt) > 0 && subInt[0]) {
			netInstIntf.Id = ygot.String(fmt.Sprintf("%s.%d", intf.GetName(), si))
		} else {
			netInstIntf.Id = ygot.String(intf.GetName())
		}
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
		gnmi.BatchUpdate(batch, gnmi.OC().NetworkInstance(ni).Config(), netInst)
	}
}

// L3VRFSubinterfaceBinding specifies an interface and subinterface index to bind to an L3VRF.
type L3VRFSubinterfaceBinding struct {
	InterfaceName string
	SubintfIndex  uint32
	VlanID        uint16
}

// ConfigureL3VRFWithSubinterfaces configures an L3VRF network instance and binds the specified
// subinterfaces to it using `<interface>.<subinterface>` ID format without using dut.Vendor().
func ConfigureL3VRFWithSubinterfaces(t *testing.T, dut *ondatra.DUTDevice, sb *gnmi.SetBatch, vrfName string, bindings []L3VRFSubinterfaceBinding) *gnmi.SetBatch {
	t.Helper()
	if len(bindings) == 0 {
		t.Fatalf("ConfigureL3VRFWithSubinterfaces: bindings slice cannot be empty")
	}
	if sb == nil {
		sb = &gnmi.SetBatch{}
	}

	// Ensure subinterfaces exist prior to VRF binding when required by platform deviations.
	for _, b := range bindings {
		sub := &oc.Interface_Subinterface{
			Index: ygot.Uint32(b.SubintfIndex),
		}
		if deviations.InterfaceEnabled(dut) {
			sub.Enabled = ygot.Bool(true)
		}
		if b.VlanID > 0 {
			if deviations.DeprecatedVlanID(dut) {
				sub.GetOrCreateVlan().VlanId = oc.UnionUint16(b.VlanID)
			} else {
				sub.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().SetVlanId(b.VlanID)
			}
		}
		if !deviations.IPv4MissingEnabled(dut) {
			sub.GetOrCreateIpv4().Enabled = ygot.Bool(true)
		}
		if deviations.InterfaceEnabled(dut) {
			sub.GetOrCreateIpv6().Enabled = ygot.Bool(true)
		}
		gnmi.BatchUpdate(sb, gnmi.OC().Interface(b.InterfaceName).Subinterface(b.SubintfIndex).Config(), sub)
	}

	ni := &oc.NetworkInstance{
		Name: ygot.String(vrfName),
		Type: oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
	}
	if deviations.BgpAfiSafiInDefaultNiBeforeOtherNi(dut) {
		ni.SetRouteDistinguisher("64500:100")
	}
	for _, b := range bindings {
		intfID := fmt.Sprintf("%s.%d", b.InterfaceName, b.SubintfIndex)
		if !deviations.InterfaceRefInterfaceIDFormat(dut) && b.SubintfIndex == 0 {
			intfID = b.InterfaceName
		}
		niIntf := ni.GetOrCreateInterface(intfID)
		niIntf.Id = ygot.String(intfID)
		niIntf.Interface = ygot.String(b.InterfaceName)
		niIntf.Subinterface = ygot.Uint32(b.SubintfIndex)
	}
	gnmi.BatchUpdate(sb, gnmi.OC().NetworkInstance(vrfName).Config(), ni)
	return sb
}
