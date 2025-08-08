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

const (
	IPv4 = "IPv4"
	IPv6 = "IPv6"
)

type VrfRule struct {
	Index        uint32
	IpType       string
	SourcePrefix string
	PrefixLength uint8
	VrfName      string
}

func EnableDefaultVrfBgp(t *testing.T, dut *ondatra.DUTDevice, dutAS uint32) {
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

func ConfigureVrfWithBgp(t *testing.T, dut *ondatra.DUTDevice, vrfName string, isDefault bool, interfaceName, routerId, peerAddress string, routerAS, peerAS uint32, ipType string) {
	root := &oc.Root{}
	ni := root.GetOrCreateNetworkInstance(vrfName)
	if isDefault {
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE
	} else {
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
	}
	niIntf := ni.GetOrCreateInterface(interfaceName)
	niIntf.Interface = ygot.String(interfaceName)
	niIntf.Subinterface = ygot.Uint32(0)

	proto := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := proto.GetOrCreateBgp()
	global := bgp.GetOrCreateGlobal()
	global.As = ygot.Uint32(routerAS)
	global.RouterId = ygot.String(routerId)

	neighbor := bgp.GetOrCreateNeighbor(peerAddress)
	neighbor.PeerAs = ygot.Uint32(peerAS)
	neighbor.Enabled = ygot.Bool(true)
	neighbor.SendCommunityType = []oc.E_Bgp_CommunityType{oc.Bgp_CommunityType_NONE}

	neighbor.GetOrCreateApplyPolicy().DefaultExportPolicy = oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE
	neighbor.GetOrCreateApplyPolicy().DefaultImportPolicy = oc.RoutingPolicy_DefaultPolicyType_ACCEPT_ROUTE

	var nAfiSafi *oc.NetworkInstance_Protocol_Bgp_Neighbor_AfiSafi
	switch ipType {
	case IPv4:
		nAfiSafi = neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
		nAfiSafi.GetOrCreateIpv4Unicast().SendDefaultRoute = ygot.Bool(true)
	case IPv6:
		nAfiSafi = neighbor.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
		nAfiSafi.GetOrCreateIpv6Unicast().SendDefaultRoute = ygot.Bool(true)
	}
	nAfiSafi.Enabled = ygot.Bool(true)
	nAfiSafi.GetOrCreateAddPaths().Receive = ygot.Bool(true)
	nAfiSafi.GetOrCreateAddPaths().Send = ygot.Bool(true)

	t.Logf("Configuring VRF %s with BGP - %s", vrfName, ipType)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(vrfName).Config(), ni)
}

func ConfigureVrfSelectionPolicy(t *testing.T, dut *ondatra.DUTDevice, networkInstance, policyName, interfaceName string, vrfRules []VrfRule) {

	t.Logf("Configuring VRF Selection Policy")
	pf := &oc.Root{}
	ni := pf.GetOrCreateNetworkInstance(networkInstance)
	policy := ni.GetOrCreatePolicyForwarding().GetOrCreatePolicy(policyName)
	policy.Type = oc.Policy_Type_VRF_SELECTION_POLICY

	for _, vrfRule := range vrfRules {
		rule := policy.GetOrCreateRule(vrfRule.Index)
		switch vrfRule.IpType {
		case IPv4:
			rule.GetOrCreateIpv4().SourceAddress = ygot.String(fmt.Sprintf("%s/%d", vrfRule.SourcePrefix, vrfRule.PrefixLength))
		case IPv6:
			rule.GetOrCreateIpv6().SourceAddress = ygot.String(fmt.Sprintf("%s/%d", vrfRule.SourcePrefix, vrfRule.PrefixLength))
		default:
			t.Fatalf("Unsupported IP type %s in vrf rule", vrfRule.IpType)
		}
		rule.GetOrCreateTransport()
		ruleAction := rule.GetOrCreateAction()
		ruleAction.SetNetworkInstance(vrfRule.VrfName)
	}

	if interfaceName != "" {
		pfIntf := ni.GetOrCreatePolicyForwarding().GetOrCreateInterface(interfaceName)
		pfIntf.ApplyVrfSelectionPolicy = ygot.String(policyName)
		pfIntf.GetOrCreateInterfaceRef().Interface = ygot.String(interfaceName)
		pfIntf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	}
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(networkInstance).Config(), ni)
}
