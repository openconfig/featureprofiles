// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ocutils

import (
	"net"
	"testing"
	"time"

	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

func CreateBundleInterface(t *testing.T, dut *ondatra.DUTDevice, interface_name string, bundle_name string) {

	member := &telemetry.Interface{
		Ethernet: &telemetry.Interface_Ethernet{
			AggregateId: ygot.String(bundle_name),
		},
	}
	update_response := dut.Config().Interface(interface_name).Update(t, member)
	t.Logf("Update response : %v", update_response)
	SetInterfaceState(t, dut, bundle_name, true)
}

func SetInterfaceState(t *testing.T, dut *ondatra.DUTDevice, interface_name string, admin_state bool) {

	i := &telemetry.Interface{
		Enabled: ygot.Bool(admin_state),
		Name:    ygot.String(interface_name),
	}
	update_response := dut.Config().Interface(interface_name).Update(t, i)
	t.Logf("Update response : %v", update_response)
	currEnabledState := dut.Telemetry().Interface(interface_name).Get(t).GetEnabled()
	if currEnabledState != admin_state {
		t.Fatalf("Failed to set interface admin_state to :%v", admin_state)
	} else {
		t.Logf("Interface admin_state set to :%v", admin_state)
	}
}

func FlapInterface(t *testing.T, dut *ondatra.DUTDevice, interface_name string, flap_duration time.Duration) {

	initialState := dut.Telemetry().Interface(interface_name).Get(t).GetEnabled()
	transientState := !initialState
	SetInterfaceState(t, dut, interface_name, transientState)
	time.Sleep(flap_duration * time.Second)
	SetInterfaceState(t, dut, interface_name, initialState)
}

func GetSubInterface(ipv4 string, prefixlen uint8, index uint32) *telemetry.Interface_Subinterface {
	s := &telemetry.Interface_Subinterface{}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return s
}

func GetCopyOfIpv4SubInterfaces(t *testing.T, dut *ondatra.DUTDevice, interface_names []string, index uint32) map[string]*telemetry.Interface_Subinterface {
	copiedSubInterfaces := make(map[string]*telemetry.Interface_Subinterface)
	for _, interface_name := range interface_names {
		a := dut.Telemetry().Interface(interface_name).Subinterface(index).Ipv4().Get(t)
		copiedSubInterfaces[interface_name] = &telemetry.Interface_Subinterface{}
		ipv4 := copiedSubInterfaces[interface_name].GetOrCreateIpv4()
		for _, ipval := range a.Address {
			t.Logf("*** Copying address: %v/%v for interface %s", ipval.GetIp(), ipval.GetPrefixLength(), interface_name)
			ipv4addr := ipv4.GetOrCreateAddress(ipval.GetIp())
			ipv4addr.PrefixLength = ygot.Uint8(ipval.GetPrefixLength())
		}

	}
	return copiedSubInterfaces
}

func AddAteISISL2(t *testing.T, topo *ondatra.ATETopology, atePort, areaId, network_name string, metric uint32, prefix string, count uint32) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	network := intfs[atePort].AddNetwork(network_name)
	//IPReachabilityConfig :=
	network.ISIS().WithIPReachabilityMetric(metric + 1)
	network.IPv4().WithAddress(prefix).WithCount(count)
	intfs[atePort].ISIS().WithAreaID(areaId).WithLevelL2().WithNetworkTypePointToPoint().WithMetric(metric).WithWideMetricEnabled(true)
}

func AddAteEBGPPeer(t *testing.T, topo *ondatra.ATETopology, atePort, peerAddress string, localAsn uint32, network_name, nexthop, prefix string, count uint32, useLoopback bool) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	//Add network instance
	network := intfs[atePort].AddNetwork(network_name)
	bgpAttribute := network.BGP()
	bgpAttribute.WithActive(true).WithNextHopAddress(nexthop)
	//Add prefixes
	network.IPv4().WithAddress(prefix).WithCount(count)
	//Create BGP instance
	bgp := intfs[atePort].BGP()
	bgpPeer := bgp.AddPeer().WithPeerAddress(peerAddress).WithLocalASN(localAsn).WithTypeExternal()
	bgpPeer.WithOnLoopback(useLoopback)

	//Update bgpCapabilities
	bgpPeer.Capabilities().WithIPv4UnicastEnabled(true).WithIPv6UnicastEnabled(true).WithGracefulRestart(true)
}

func AddLoopback(t *testing.T, topo *ondatra.ATETopology, port, loopback_prefix string) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].WithIPv4Loopback(loopback_prefix)
}
func AddIpv4Network(t *testing.T, topo *ondatra.ATETopology, port, network_name, address_CIDR string, count uint32) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].AddNetwork(network_name).IPv4().WithAddress(address_CIDR).WithCount(count)
}
func AddIpv6Network(t *testing.T, topo *ondatra.ATETopology, port, network_name, address_CIDR string, count uint32) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].AddNetwork(network_name).IPv6().WithAddress(address_CIDR).WithCount(count)
}

func GetBoundedFlow(t *testing.T, ate *ondatra.ATEDevice, topo *ondatra.ATETopology, srcPort, dstPort, srcNetwork, dstNetwork, flowName string, dscp uint8, ttl ...uint8) *ondatra.Flow {

	intfs := topo.Interfaces()
	flow := ate.Traffic().NewFlow(flowName)
	t.Logf("Setting up flow -> %s", flowName)
	networks1 := intfs[srcPort].Networks()
	networks2 := intfs[dstPort].Networks()
	ethheader := ondatra.NewEthernetHeader()
	ipheader1 := ondatra.NewIPv4Header().WithDSCP(dscp)
	if len(ttl) > 0 {
		ipheader1.WithTTL(ttl[0])
	}
	flow.WithHeaders(ethheader, ipheader1)
	flow.WithSrcEndpoints(networks1[srcNetwork])
	flow.WithDstEndpoints(networks2[dstNetwork])
	flow.WithFrameRateFPS(100)
	flow.WithFrameSize(1024)
	return flow
}

func GetIpv4Acl(name string, sequenceId uint32, dscp, hopLimit uint8, action telemetry.E_Acl_FORWARDING_ACTION) *telemetry.Acl {

	acl := (&telemetry.Device{}).GetOrCreateAcl()
	aclSet := acl.GetOrCreateAclSet(name, telemetry.Acl_ACL_TYPE_ACL_IPV4)
	aclEntry := aclSet.GetOrCreateAclEntry(sequenceId)
	aclEntryIpv4 := aclEntry.GetOrCreateIpv4()
	aclEntryIpv4.Dscp = ygot.Uint8(dscp)
	aclEntryIpv4.HopLimit = ygot.Uint8(hopLimit)
	aclEntryAction := aclEntry.GetOrCreateActions()
	aclEntryAction.ForwardingAction = action
	return acl
}

func AddIpv6Address(ipv6 string, prefixlen uint8, index uint32) *telemetry.Interface_Subinterface {
	s := &telemetry.Interface_Subinterface{}
	s.Index = ygot.Uint32(index)
	s4 := s.GetOrCreateIpv6()
	a := s4.GetOrCreateAddress(ipv6)
	a.PrefixLength = ygot.Uint8(prefixlen)
	return s
}

func GetIPPrefix(IPAddr string, i int, prefixLen string) string {
	ip := net.ParseIP(IPAddr)
	ip = ip.To4()
	ip[3] = ip[3] + byte(i%256)
	ip[2] = ip[2] + byte(i/256)
	ip[1] = ip[1] + byte(i/(256*256))
	return ip.String() + "/" + prefixLen
}
