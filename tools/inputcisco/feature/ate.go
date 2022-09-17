//Package feature conatins util function for each feature
package feature

import (
	"fmt"
	"strings"
	"testing"

	"github.com/openconfig/featureprofiles/tools/inputcisco/proto"
	"github.com/openconfig/featureprofiles/tools/inputcisco/solver"
	"github.com/openconfig/ondatra"
)

var (
	ixiaTopology = make(map[string]*ondatra.ATETopology)
)

// ConfigIxiaTopology pushes the network and protocol configuration to ixa
func ConfigIxiaTopology(dev *ondatra.ATEDevice, t *testing.T, feature *proto.Input_Feature) {
	configIXIATopology(t, dev, feature)
}

// StartIxiaProtocols starts the configured protocols on ixia
func StartIxiaProtocols(dev *ondatra.ATEDevice, t *testing.T, feature *proto.Input_Feature) {
	topoobj := configIXIATopology(t, dev, feature)
	topoobj.StartProtocols(t)
}
func addAteISISL2(t *testing.T, topo *ondatra.ATETopology, atePort, areaID, networkName string, metric uint32, iprmetric uint32, prefix string, count uint32) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	network := intfs[atePort].AddNetwork(networkName)
	//IPReachabilityConfig :=
	network.ISIS().WithIPReachabilityMetric(iprmetric)
	network.IPv4().WithAddress(prefix).WithCount(count)
	intfs[atePort].ISIS().WithAreaID(areaID).WithLevelL2().WithNetworkTypePointToPoint().WithMetric(metric).WithWideMetricEnabled(true)
}

func addAteEBGPPeer(t *testing.T, topo *ondatra.ATETopology, atePort, peerAddress string, localAsn uint32, networkName, nexthop, prefix string, count uint32, useLoopback bool) {

	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	//Add network instance
	network := intfs[atePort].AddNetwork(networkName)
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

func addLoopback(t *testing.T, topo *ondatra.ATETopology, port, loopbackPrefix string) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].WithIPv4Loopback(loopbackPrefix)
}

func addIpv4Network(t *testing.T, topo *ondatra.ATETopology, port, networkName, addressCIDR string, count uint32) {
	intfs := topo.Interfaces()
	if len(intfs) == 0 {
		t.Fatal("There are no interfaces in the Topology")
	}
	intfs[port].AddNetwork(networkName).IPv4().WithAddress(addressCIDR).WithCount(count)
}

func configIXIATopology(t *testing.T, dev *ondatra.ATEDevice, feature *proto.Input_Feature) *ondatra.ATETopology {
	topo, ok := ixiaTopology[dev.ID()]
	if !ok {
		topo = dev.Topology().New()
		generateBaseScenario(t, dev, topo, feature)
		topo.Push(t)
		ixiaTopology[dev.ID()] = topo
	}
	return topo
}

func generateBaseScenario(t *testing.T, ate *ondatra.ATEDevice, topoObj *ondatra.ATETopology, feature *proto.Input_Feature) {
	for _, p := range ate.Device.Ports() {
		intfObj := topoObj.AddInterface(p.Name())
		intfObj.WithPort(ate.Port(t, p.ID()))
		for _, intf := range feature.Interface {
			id := solver.SolveAte(t, ate, intf.Id)
			if id == p.ID() {
				if strings.HasPrefix(strings.ToLower(intf.Name), "loopback") {
					addLoopback(t, topoObj, p.Name(), fmt.Sprintf(intf.Ipv4Address+"/%d", intf.Ipv4PrefixLength))
				} else {
					if intf.Ipv4Address != "" && intf.Ipv4PrefixLength != 0 && intf.Ipv4Gateway != "" {
						intfObj.IPv4().WithAddress(fmt.Sprintf(intf.Ipv4Address+"/%d", intf.Ipv4PrefixLength)).WithDefaultGateway(intf.Ipv4Gateway)
					}
					if intf.Ipv6Address != "" && intf.Ipv6PrefixLength != 0 && intf.Ipv6Gateway != "" {
						intfObj.IPv6().WithAddress(fmt.Sprintf(intf.Ipv6Address+"/%d", intf.Ipv6PrefixLength)).WithDefaultGateway(intf.Ipv6Gateway)
					}
				}
			}
		}
	}
	addNetworkAndProtocolsToAte(t, ate, topoObj, feature)
}

func addNetworkAndProtocolsToAte(t *testing.T, ate *ondatra.ATEDevice, topo *ondatra.ATETopology, feature *proto.Input_Feature) {
	//Add prefixes/networks on ports
	for _, v4n := range feature.Ipv4Network {
		portID := solver.SolveAte(t, ate, v4n.Id)
		port := ate.Port(t, portID)

		addIpv4Network(t, topo, port.Name(), v4n.Name, v4n.Cidr, uint32(v4n.Scale))

	}
	//Configure ISIS, BGP on TGN
	for _, isis := range feature.Isis {
		portID := solver.SolveAte(t, ate, isis.Port)
		port := ate.Port(t, portID)

		addAteISISL2(t, topo, port.Name(), fmt.Sprintf("%d", isis.Areaid), isis.Networkname, uint32(isis.Prefixmetric), uint32(isis.Iprmetric), isis.V4Prefix, uint32(isis.Scale))
	}
	for _, bgp := range feature.Bgp {
		for _, neighbor := range bgp.Neighbors {
			portID := solver.SolveAte(t, ate, neighbor.Port)
			port := ate.Port(t, portID)

			addAteEBGPPeer(t, topo, port.Name(), neighbor.Address, uint32(neighbor.LocalAs), bgp.Networkname, neighbor.Nexthop, neighbor.Prefix, uint32(bgp.Scale), bgp.Useloopback)
		}
	}

}
