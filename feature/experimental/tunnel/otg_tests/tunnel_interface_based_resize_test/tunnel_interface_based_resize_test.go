// Copyright 2022 Google LLC
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

// Package topology_test configures just the ports on DUT and ATE,
// assuming that DUT port i is connected to ATE i.  It detects the
// number of ports in the testbed and can be used with the 2, 4, 12
// port variants of the atedut testbed.
package tunnel_interface_based_resize_test

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	otg "github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
	"github.com/openconfig/featureprofiles/internal/otgutils"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

//TUN-1.6: Tunnel End Point Resize - Interface Based GRE Tunnel
// ## Summary
// Validate of interface based GRE tunnel end point reduction and increment test with load balaning.
//## Procedure
//
// *   Configure DUT with 8 GRE encapsulation tunnels and configure another router with 8 GRE Decapsulation tunnel interfaces.
// *   Configure 4 tunnel as IPv4 tunnel source and destination address , 4 as IPv6 tunnel source and destination address
// *   Configure static router to point original destination to 8 tunnel interface to do overlay loadbalance
// *   Keep topology tunnel destination will be reachable via 2 underlay interface on both routers
// *   Send IPv4 flow and IPv6 flow and validate tunnel load balance and physical interface load balance
// *   resize the tunnel fro 8 to 4 and verify the load balance and traffic drop by removing static route to point tunnel interface.
// *   Again resize the tunnel fro 4 to 8 and verify the load balance and traffic drop
//
// ## Config Parameter coverage
//
// *   openconfig-interfaces:interfaces/interface[name='fti0']
// *   openconfig-interfaces:interfaces/interface[name='fti0']/tunnel/
// *   openconfig-interfaces:interfaces/interface[name='fti0']/tunnel/gre
//
// ## Telemetry Parameter coverage
//
// *   /interfaces/interface[name='fti0']/state/counters/operstatus
// *   /interfaces/interface[name='fti0']/state/counters/in-pkts 
// *   /interfaces/interface[name='fti0']/state/counters/in-octets 
// *   /interfaces/interface[name='fti0']/state/counters/out-pkts 
// *   /interfaces/interface[name='fti0']/state/counters/out-octets 
//
//
//  ## Topology:
// *   otg:port1 <--> port1:dut1:port3 <--> port3:dut2:port5<--->otg:port5
// *   otg:port2 <--> port2:dut1:port4 <--> port4:dut2:port6<--->otg:port6
type parameters struct {
	rtIntf1Ipv4Add   string
	rtIntf2Ipv4Add   string
	rtIntf5Ipv4Add   string
	rtIntf6Ipv4Add   string
	rtIntf1MacAdd    string
	rtIntf2MacAdd    string
	rtIntf5MacAdd    string
	rtIntf6MacAdd    string
	r0Intf1Ipv4Add   string
	r0Intf2Ipv4Add   string
	r0Intf3Ipv4Add   string
	r0Intf4Ipv4Add   string
	r0Fti0Ipv4Add    string
	r0Fti1Ipv4Add    string
	r0Fti2Ipv4Add    string
	r0Fti3Ipv4Add    string
	r0Fti4Ipv4Add    string
	r0Fti5Ipv4Add    string
	r0Fti6Ipv4Add    string
	r0Fti7Ipv4Add    string
	r0Lo0Ut0Ipv4Add  string
	r0Lo0Ut1Ipv4Add  string
	r0Lo0Ut2Ipv4Add  string
	r0Lo0Ut3Ipv4Add  string
	ipv4Mask         uint8
	ipv4FullMask     uint8
	r1Intf5Ipv4Add   string
	r1Intf6Ipv4Add   string
	r1Intf3Ipv4Add   string
	r1Intf4Ipv4Add   string
	r1Fti0Ipv4Add    string
	r1Fti1Ipv4Add    string
	r1Fti2Ipv4Add    string
	r1Fti3Ipv4Add    string
	r1Fti4Ipv4Add    string
	r1Fti5Ipv4Add    string
	r1Fti6Ipv4Add    string
	r1Fti7Ipv4Add    string
	r1Lo0Ut0Ipv4Add  string
	r1Lo0Ut1Ipv4Add  string
	r1Lo0Ut2Ipv4Add  string
	r1Lo0Ut3Ipv4Add  string
	rtIntf1Ipv6Add   string
	rtIntf2Ipv6Add   string
	rtIntf5Ipv6Add   string
	rtIntf6Ipv6Add   string
	r0Intf1Ipv6Add   string
	r0Intf2Ipv6Add   string
	r0Intf3Ipv6Add   string
	r0Intf4Ipv6Add   string
	r0Fti0Ipv6Add    string
	r0Fti1Ipv6Add    string
	r0Fti2Ipv6Add    string
	r0Fti3Ipv6Add    string
	r0Fti4Ipv6Add    string
	r0Fti5Ipv6Add    string
	r0Fti6Ipv6Add    string
	r0Fti7Ipv6Add    string
	r0Lo0Ut0Ipv6Add  string
	r0Lo0Ut1Ipv6Add  string
	r0Lo0Ut2Ipv6Add  string
	r0Lo0Ut3Ipv6Add  string
	ipv6Mask         uint8
	ipv6FullMask     uint8
	r1Intf5Ipv6Add   string
	r1Intf6Ipv6Add   string
	r1Intf3Ipv6Add   string
	r1Intf4Ipv6Add   string
	r1Fti0Ipv6Add    string
	r1Fti1Ipv6Add    string
	r1Fti2Ipv6Add    string
	r1Fti3Ipv6Add    string
	r1Fti4Ipv6Add    string
	r1Fti5Ipv6Add    string
	r1Fti6Ipv6Add    string
	r1Fti7Ipv6Add    string
	r1Lo0Ut0Ipv6Add  string
	r1Lo0Ut1Ipv6Add  string
	r1Lo0Ut2Ipv6Add  string
	r1Lo0Ut3Ipv6Add  string
	flow1            string
	flow2            string
	flow3            string
	flow4            string
	trafficDuration  int64
	trafficRate      int64
}

func GetNetworkAddress(t *testing.T, address string, mask int) string {

	Addr := net.ParseIP(address)
	var network net.IP
	_ = network
	IsIPv4 := Addr.To4()
	if IsIPv4 != nil {
		// This mask corresponds to a /24 subnet for IPv4.

		ipv4Mask := net.CIDRMask(mask, 32)
		//t.Logf("%s in %T\n",ipv4Mask,ipv4Mask)
		network := Addr.Mask(ipv4Mask)
		net := fmt.Sprintf("%s/%d", network, mask)
		t.Logf("network address : %s", net)
		return net
	} else {

		// This mask corresponds to a /32 subnet for IPv6.
		ipv6Mask := net.CIDRMask(mask, 128)
		network := Addr.Mask(ipv6Mask)
		//t.Logf("IPv6 network: %s",network)
		net := fmt.Sprintf("%s/%d", network, mask)
		t.Logf("Network address : %s", net)
		return net
	}

}

func ConfigureTunnelEncapDUT(t *testing.T, p *parameters, dut *ondatra.DUTDevice, dp1 *ondatra.Port, dp2 *ondatra.Port, dp3 *ondatra.Port, dp4 *ondatra.Port) {

	dutIntfs := []struct {
		desc     string
		intfName string
		ipAddr   string
		ipv6Addr string
		ipv4mask uint8
		ipv6mask uint8
	}{
		{
			desc:     "R0_ATE1",
			intfName: dp1.Name(),
			ipAddr:   p.r0Intf1Ipv4Add ,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r0Intf1Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		}, {
			desc:     "R0_ATE2",
			intfName: dp2.Name(),
			ipAddr:   p.r0Intf2Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r0Intf2Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		}, {
			desc:     "R0_R1_1",
			intfName: dp3.Name(),
			ipAddr:   p.r0Intf3Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r0Intf3Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},
		{
			desc:     "R0_R1_2",
			intfName: dp4.Name(),
			ipAddr:   p.r0Intf4Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r0Intf4Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},
		{
			desc:     "tunnel0",
			intfName: "lo0",
			ipAddr:   p.r0Lo0Ut0Ipv4Add,
			ipv4mask: p.ipv4FullMask,
			ipv6Addr: p.r0Lo0Ut0Ipv6Add,
			ipv6mask: p.ipv6FullMask,
		},

		{
			desc:     "tunnel-1",
			intfName: "fti0",
			ipAddr:   p.r0Fti0Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r0Fti0Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},

		{
			desc:     "tunnel-2",
			intfName: "fti1",
			ipAddr:   p.r0Fti1Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r0Fti1Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},

		{
			desc:     "tunnel-3",
			intfName: "fti2",
			ipAddr:   p.r0Fti2Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r0Fti2Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},
		{
			desc:     "tunnel-4",
			intfName: "fti3",
			ipAddr:   p.r0Fti3Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r0Fti3Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},

		{
			desc:     "tunnel-5",
			intfName: "fti4",
			ipAddr:   p.r0Fti4Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r0Fti4Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},

		{
			desc:     "tunnel-6",
			intfName: "fti5",
			ipAddr:   p.r0Fti5Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r0Fti5Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},
		{
			desc:     "tunnel-7",
			intfName: "fti6",
			ipAddr:   p.r0Fti6Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r0Fti6Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},
		{
			desc:     "tunnel-8",
			intfName: "fti7",
			ipAddr:   p.r0Fti7Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r0Fti7Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},
	}

	// Configure the interfaces.
	for _, intf := range dutIntfs {
		t.Logf("Configure DUT interface %s with attributes %v", intf.intfName, intf)
		i := &oc.Interface{
			Name:        ygot.String(intf.intfName),
			Description: ygot.String(intf.desc),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled:     ygot.Bool(true),
		}
		// configure ipv4 address
		i.GetOrCreateEthernet()
		i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		a := i4.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.ipv4mask)

		// configure ipv6 address
		i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
		b := i6.GetOrCreateAddress(intf.ipv6Addr)
		b.PrefixLength = ygot.Uint8(intf.ipv6mask)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
	}
}

func ConfigureTunnelDecapDUT(t *testing.T, p *parameters, dut *ondatra.DUTDevice, dp1 *ondatra.Port, dp2 *ondatra.Port, dp3 *ondatra.Port, dp4 *ondatra.Port) {

	dutIntfs := []struct {
		desc     string
		intfName string
		ipAddr   string
		ipv6Addr string
		ipv4mask uint8
		ipv6mask uint8
	}{
		{
			desc:     "R1_ATE1",
			intfName: dp1.Name(),
			ipAddr:   p.r1Intf3Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r1Intf3Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		}, {
			desc:     "R1_ATE",
			intfName: dp2.Name(),
			ipAddr:   p.r1Intf4Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r1Intf4Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		}, {
			desc:     "R1_R0_1",
			intfName: dp3.Name(),
			ipAddr:   p.r1Intf5Ipv4Add ,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r1Intf5Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},
		{
			desc:     "R1_R0_2",
			intfName: dp4.Name(),
			ipAddr:   p.r1Intf6Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r1Intf6Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},
		{
			desc:     "tunnel0",
			intfName: "lo0",
			ipAddr:   p.r1Lo0Ut0Ipv4Add,
			ipv4mask: p.ipv4FullMask,
			ipv6Addr: p.r1Lo0Ut0Ipv6Add,
			ipv6mask: p.ipv6FullMask,
		},

		{
			desc:     "tunnel-1",
			intfName: "fti0",
			ipAddr:   p.r1Fti0Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r1Fti0Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},

		{
			desc:     "tunnel-2",
			intfName: "fti1",
			ipAddr:   p.r1Fti1Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r1Fti1Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},

		{
			desc:     "tunnel-3",
			intfName: "fti2",
			ipAddr:   p.r1Fti2Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r1Fti2Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},
		{
			desc:     "tunnel-4",
			intfName: "fti3",
			ipAddr:   p.r1Fti3Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r1Fti3Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},

		{
			desc:     "tunnel-5",
			intfName: "fti4",
			ipAddr:   p.r1Fti4Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r1Fti4Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},

		{
			desc:     "tunnel-6",
			intfName: "fti5",
			ipAddr:   p.r1Fti5Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r1Fti5Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},
		{
			desc:     "tunnel-7",
			intfName: "fti6",
			ipAddr:   p.r1Fti6Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r1Fti6Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},
		{
			desc:     "tunnel-8",
			intfName: "fti7",
			ipAddr:   p.r1Fti7Ipv4Add,
			ipv4mask: p.ipv4Mask,
			ipv6Addr: p.r1Fti7Ipv6Add,
			ipv6mask: p.ipv6Mask ,
		},
	}

	// Configure the interfaces.
	for _, intf := range dutIntfs {
		t.Logf("Configure DUT interface %s with attributes %v", intf.intfName, intf)
		i := &oc.Interface{
			Name:        ygot.String(intf.intfName),
			Description: ygot.String(intf.desc),
			Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
			Enabled:     ygot.Bool(true),
		}

		// configure ipv4 address
		i.GetOrCreateEthernet()
		i4 := i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		a := i4.GetOrCreateAddress(intf.ipAddr)
		a.PrefixLength = ygot.Uint8(intf.ipv4mask)

		// configure ipv6 address
		i6 := i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
		b := i6.GetOrCreateAddress(intf.ipv6Addr)
		b.PrefixLength = ygot.Uint8(intf.ipv6mask)
		gnmi.Replace(t, dut, gnmi.OC().Interface(intf.intfName).Config(), i)
	}
}


func ConfigureTunnelInterface(t *testing.T, intf string, tunnelSrc string, tunnelDst string, dut *ondatra.DUTDevice) {

	// IPv4 tunnel source and destination configuration
	t.Logf("Push the IPv4/IPv6 tunnel endpoint config:\n%s", dut.Vendor())
	var config string
	switch dut.Vendor() {
		case ondatra.JUNIPER:
	        config = ConfigureTunnelEndPoints(intf, tunnelSrc, tunnelDst)
			t.Logf("Push the CLI config:\n%s", config)

		default:
 		        t.Errorf("Invalid Tunnel endpoint configuration")
 	}
	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	gpbSetRequest, err := buildCliConfigRequest(config)
	if err != nil {
		t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
	}
	t.Log("gnmiClient Set CLI config")
	if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
}

func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.Slice(ports, func(i, j int) bool {
		idi, idj := ports[i].ID(), ports[j].ID()
		li, lj := len(idi), len(idj)
		if li == lj {
			return idi < idj
		}
		return li < lj // "port2" < "port10"
	})
	return ports
}

func ConfigureTunnelEndPoints(intf string, tunnelSrc string, tunnelDest string) string {

	return fmt.Sprintf(`
	interfaces {
	%s {
		unit 0 {
			tunnel {
				encapsulation gre {
					source {
						address %s;
					}
					destination {
						address %s;
					}
				}
			}
		}
	}
	}`, intf, tunnelSrc, tunnelDest)

}

func ConfigureAdditionalIPv4AddressonLoopback(address string) string {

	return fmt.Sprintf(`
	interfaces {

    lo0 {
        unit 0 {
            family inet {
                address %s;
            }
        }
    }
}`, address)

}

func ConfigureAdditionalIPv6AddressonLoopback(address string) string {

	return fmt.Sprintf(`
	interfaces {

    lo0 {
        unit 0 {
            family inet6 {
                address %s;
            }
        }
    }
}`, address)

}

func ConfigureTunnelTerminationOption(interf string) string {

	return fmt.Sprintf(`
	interfaces {

    %s {
        unit 0 {
            family inet {
                  tunnel-termination;
            }
            family inet6 {
                tunnel-termination;
            }
        }
    }
}`, interf)

}

func configIPv4StaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string, index string) {
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop(index)
	nh.NextHop = oc.UnionString(nexthop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)

}

func deleteStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string, index string) {
    dutProtoStatConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, "DEFAULT")
    gnmi.Delete(t, dut, dutProtoStatConfPath.Static(prefix).NextHop(index).Config())
}


func buildCliConfigRequest(config string) (*gpb.SetRequest, error) {
	// Build config with Origin set to cli and Ascii encoded config.
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cli",
				Elem:   []*gpb.PathElem{},
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: config,
				},
			},
		}},
	}
	return gpbSetRequest, nil
}

// Configure network instance
func configureNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
}

func ConfigureLoobackInterfaceWithIPv6address(t *testing.T, address string, dut *ondatra.DUTDevice) {

	// IPv6 address on lo0 interface
	t.Logf("Push the IPv4 address to lo0 interface :\n%s", dut.Vendor())
	var config string
	switch dut.Vendor() {
		case ondatra.JUNIPER:
	        config = ConfigureAdditionalIPv6AddressonLoopback(address)
	        t.Logf("Push the CLI config:\n%s", config)
	    default:
 		    t.Errorf("Invalid IPv6 Loop back address configuration")
 	}
	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	gpbSetRequest, err := buildCliConfigRequest(config)
	if err != nil {
	    t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
	}
	 t.Log("gnmiClient Set CLI config")
	if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
	    t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
}

func ConfigureLoobackInterfaceWithIPv4address(t *testing.T, address string, dut *ondatra.DUTDevice) {

	// IPv4 address on lo0 interface
	t.Logf("Push the IPv4 address to lo0 interface :\n%s", dut.Vendor())
	var config string
	switch dut.Vendor() {
		case ondatra.JUNIPER:
	        config := ConfigureAdditionalIPv4AddressonLoopback(address)
	        t.Logf("Push the CLI config:\n%s", config)
	    default:
 		    t.Errorf("Invalid IPv4 Loop back address configuration")
 	}	
	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	gpbSetRequest, err := buildCliConfigRequest(config)
	if err != nil {
	    t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
	}
	t.Log("gnmiClient Set CLI config")
	if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
	    t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}

}
func ConfigureTunnelTermination(t *testing.T, intf *ondatra.Port, dut *ondatra.DUTDevice) {

	// IPv4/IPv6 tunnel termination on underlay port
	t.Logf("IPv4/IPv6 tunnel termination on underlay port :\n%s", dut.Vendor())
	var config string
	switch dut.Vendor() {
		case ondatra.JUNIPER:
	        config = ConfigureTunnelTerminationOption(intf.Name())
	        t.Logf("Push the CLI config:\n%s", config)
	    default:
 		    t.Errorf("Invalid Tunnel termination configuration")
 	}
	gnmiClient := dut.RawAPIs().GNMI().Default(t)
	gpbSetRequest, err := buildCliConfigRequest(config)
	if err != nil {
	    t.Fatalf("Cannot build a gNMI SetRequest: %v", err)
	}

	t.Log("gnmiClient Set CLI config")
	if _, err = gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
	    t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
}

func TestFtiTunnels(t *testing.T) {

	p := &parameters{
		rtIntf1Ipv4Add:   "10.1.1.2",
		rtIntf2Ipv4Add:   "11.1.1.2",
		rtIntf5Ipv4Add:   "30.1.1.1",
		rtIntf6Ipv4Add:   "31.1.1.1",
		rtIntf1MacAdd:    "00:00:aa:aa:aa:aa",
		rtIntf2MacAdd:    "00:00:bb:bb:bb:bb",
		rtIntf5MacAdd:    "00:00:cc:cc:cc:cc",
		rtIntf6MacAdd:    "00:00:dd:dd:dd:dd",
		r0Intf1Ipv4Add:   "10.1.1.1",
		r0Intf2Ipv4Add:   "11.1.1.1",
		r0Intf3Ipv4Add:   "20.1.1.1",
		r0Intf4Ipv4Add:   "21.1.1.1",
		r0Fti0Ipv4Add:    "90.1.1.1",
		r0Fti1Ipv4Add:    "91.1.1.1",
		r0Fti2Ipv4Add:    "92.1.1.1",
		r0Fti3Ipv4Add:    "93.1.1.1",
		r0Fti4Ipv4Add:    "94.1.1.1",
		r0Fti5Ipv4Add:    "95.1.1.1",
		r0Fti6Ipv4Add:    "96.1.1.1",
		r0Fti7Ipv4Add:    "97.1.1.1",
		r0Lo0Ut0Ipv4Add:  "70.1.1.1",
		r0Lo0Ut1Ipv4Add:  "71.1.1.1",
		r0Lo0Ut2Ipv4Add:  "72.1.1.1",
		r0Lo0Ut3Ipv4Add:  "73.1.1.1",
		ipv4Mask:         24,
		ipv4FullMask:     32,
		r1Intf5Ipv4Add:   "30.1.1.2",
		r1Intf6Ipv4Add:   "31.1.1.2",
		r1Intf3Ipv4Add:   "20.1.1.2",
		r1Intf4Ipv4Add:   "21.1.1.2",
		r1Fti0Ipv4Add:    "90.1.1.2",
		r1Fti1Ipv4Add:    "91.1.1.2",
		r1Fti2Ipv4Add:    "92.1.1.2",
		r1Fti3Ipv4Add:    "93.1.1.2",
		r1Fti4Ipv4Add:    "94.1.1.2",
		r1Fti5Ipv4Add:    "95.1.1.2",
		r1Fti6Ipv4Add:    "96.1.1.2",
		r1Fti7Ipv4Add:    "97.1.1.2",
		r1Lo0Ut0Ipv4Add:  "80.1.1.1",
		r1Lo0Ut1Ipv4Add:  "81.1.1.1",
		r1Lo0Ut2Ipv4Add:  "82.1.1.1",
		r1Lo0Ut3Ipv4Add:  "83.1.1.1",
		rtIntf1Ipv6Add:   "2000:10:1:1::2",
		rtIntf2Ipv6Add:   "2000:11:1:1::2",
		rtIntf5Ipv6Add:   "2000:30:1:1::1",
		rtIntf6Ipv6Add:   "2000:31:1:1::1",
		r0Intf1Ipv6Add:   "2000:10:1:1::1",
		r0Intf2Ipv6Add:   "2000:11:1:1::1",
		r0Intf3Ipv6Add:   "2000:20:1:1::1",
		r0Intf4Ipv6Add:   "2000:21:1:1::1",
		r0Fti0Ipv6Add:    "2000:90:1:1::1",
		r0Fti1Ipv6Add:    "2000:91:1:1::1",
		r0Fti2Ipv6Add:    "2000:92:1:1::1",
		r0Fti3Ipv6Add:    "2000:93:1:1::1",
		r0Fti4Ipv6Add:    "2000:94:1:1::1",
		r0Fti5Ipv6Add:    "2000:95:1:1::1",
		r0Fti6Ipv6Add:    "2000:96:1:1::1",
		r0Fti7Ipv6Add:    "2000:97:1:1::1",
		r0Lo0Ut0Ipv6Add:  "3000:70:1:1::1",
		r0Lo0Ut1Ipv6Add:  "3000:71:1:1::1",
		r0Lo0Ut2Ipv6Add:  "3000:72:1:1::1",
		r0Lo0Ut3Ipv6Add:  "3000:73:1:1::1",
		ipv6Mask:         120,
		r1Intf5Ipv6Add:   "2000:30:1:1::2",
		r1Intf6Ipv6Add:   "2000:31:1:1::2",
		r1Intf3Ipv6Add:   "2000:20:1:1::2",
		r1Intf4Ipv6Add:   "2000:21:1:1::2",
		r1Fti0Ipv6Add:    "2000:90:1:1::2",
		r1Fti1Ipv6Add:    "2000:91:1:1::2",
		r1Fti2Ipv6Add:    "2000:92:1:1::2",
		r1Fti3Ipv6Add:    "2000:93:1:1::2",
		r1Fti4Ipv6Add:    "2000:94:1:1::2",
		r1Fti5Ipv6Add:    "2000:95:1:1::2",
		r1Fti6Ipv6Add:    "2000:96:1:1::2",
		r1Fti7Ipv6Add:    "2000:97:1:1::2",
		r1Lo0Ut0Ipv6Add:  "3000:80:1:1::1",
		r1Lo0Ut1Ipv6Add:  "3000:81:1:1::1",
		r1Lo0Ut2Ipv6Add:  "3000:82:1:1::1",
		r1Lo0Ut3Ipv6Add:  "3000:83:1:1::1",
		ipv6FullMask:     128,
		flow1:            "IPv4-flow1",
		flow2:            "IPv4-flow2",
		flow3:            "IPv6-flow3",
		flow4:            "IPv6-flow4",
		trafficDuration:  60,
		trafficRate:      1000,
	}
	t.Logf("the input variable %+v", p)
	t.Helper()

	dut1 := ondatra.DUT(t, "dut1")
	dut1Intf1 := dut1.Port(t, "port1")
	dut1Intf2 := dut1.Port(t, "port2")
	dut1Intf3 := dut1.Port(t, "port3")
	dut1Intf4 := dut1.Port(t, "port4")
	rt := ondatra.ATE(t, "ate")
	t.Logf("dut-1 %v", dut1)
	t.Logf("dut1-port1 %v", dut1Intf1)
	t.Logf("dut1-port2 %v", dut1Intf2)
	t.Logf("dut1-port3 %v", dut1Intf3)
	t.Logf("dut1-port4 %v", dut1Intf4)

	dut2 := ondatra.DUT(t, "dut2")
	dut2Intf1 := dut2.Port(t, "port3")
	dut2Intf2 := dut2.Port(t, "port4")
	dut2Intf3 := dut2.Port(t, "port5")
	dut2Intf4 := dut2.Port(t, "port6")

	t.Logf("dut-2 %v", dut2)
	t.Logf("dut2-port1 %v", dut2Intf1)
	t.Logf("dut2-port2 %v", dut2Intf2)
	t.Logf("dut2-port3 %v", dut2Intf3)
	t.Logf("dut2-port3 %v", dut2Intf4)

	
	t.Run("Configure dut1 and dut2 ", func(t *testing.T) {
		ConfigureTunnelEncapDUT(t, p, dut1, dut1Intf1, dut1Intf2, dut1Intf3, dut1Intf4)
		ConfigureTunnelDecapDUT(t, p, dut2, dut2Intf1, dut2Intf2, dut2Intf3, dut2Intf4)
	})

	t.Run("Configure loopback interface on dut1 and dut2 ", func(t *testing.T) {
		// configure addtional loop address by native cli configuration.
		ConfigureLoobackInterfaceWithIPv4address(t, p.r0Lo0Ut1Ipv4Add, dut1)
		ConfigureLoobackInterfaceWithIPv4address(t, p.r0Lo0Ut2Ipv4Add, dut1)
		ConfigureLoobackInterfaceWithIPv4address(t, p.r0Lo0Ut3Ipv4Add, dut1)
		ConfigureLoobackInterfaceWithIPv6address(t, p.r0Lo0Ut1Ipv6Add, dut1)
		ConfigureLoobackInterfaceWithIPv6address(t, p.r0Lo0Ut2Ipv6Add, dut1)
		ConfigureLoobackInterfaceWithIPv6address(t, p.r0Lo0Ut3Ipv6Add, dut1)

		// configure addtional loop address by native cli configuration.

		ConfigureLoobackInterfaceWithIPv4address(t, p.r1Lo0Ut1Ipv4Add, dut2)
		ConfigureLoobackInterfaceWithIPv4address(t, p.r1Lo0Ut2Ipv4Add, dut2)
		ConfigureLoobackInterfaceWithIPv4address(t, p.r1Lo0Ut3Ipv4Add, dut2)
		ConfigureLoobackInterfaceWithIPv6address(t, p.r1Lo0Ut1Ipv6Add, dut2)
		ConfigureLoobackInterfaceWithIPv6address(t, p.r1Lo0Ut2Ipv6Add, dut2)
		ConfigureLoobackInterfaceWithIPv6address(t, p.r1Lo0Ut3Ipv6Add, dut2)
	})

	t.Run("Configure 8 tunnel interface on dut1 and dut2 ", func(t *testing.T) {
		// configure tunnel interface on dut1 - IPv4

		if deviations.TunnelConfigPathUnsupported(dut1) {
		    ConfigureTunnelInterface(t, "fti0", p.r0Lo0Ut0Ipv4Add, p.r1Lo0Ut0Ipv4Add, dut1)
		    ConfigureTunnelInterface(t, "fti1", p.r0Lo0Ut1Ipv4Add, p.r1Lo0Ut1Ipv4Add, dut1)
		    ConfigureTunnelInterface(t, "fti2", p.r0Lo0Ut2Ipv4Add, p.r1Lo0Ut2Ipv4Add, dut1)
		    ConfigureTunnelInterface(t, "fti3", p.r0Lo0Ut3Ipv4Add, p.r1Lo0Ut3Ipv4Add, dut1)
		}
		// configure tunnel interface on dut2- IPv4
		if deviations.TunnelConfigPathUnsupported(dut2) {
		    ConfigureTunnelInterface(t, "fti0", p.r1Lo0Ut0Ipv4Add, p.r0Lo0Ut0Ipv4Add, dut2)
		    ConfigureTunnelInterface(t, "fti1", p.r1Lo0Ut1Ipv4Add, p.r0Lo0Ut1Ipv4Add, dut2)
		    ConfigureTunnelInterface(t, "fti2", p.r1Lo0Ut2Ipv4Add, p.r0Lo0Ut2Ipv4Add, dut2)
		    ConfigureTunnelInterface(t, "fti3", p.r1Lo0Ut3Ipv4Add, p.r0Lo0Ut3Ipv4Add, dut2)
		}
		//configure tunnel interface on dut2- IPv6
		if deviations.TunnelConfigPathUnsupported(dut1) {
		    ConfigureTunnelInterface(t, "fti4", p.r0Lo0Ut0Ipv6Add, p.r1Lo0Ut0Ipv6Add, dut1)
		    ConfigureTunnelInterface(t, "fti5", p.r0Lo0Ut1Ipv6Add, p.r1Lo0Ut1Ipv6Add, dut1)
		    ConfigureTunnelInterface(t, "fti6", p.r0Lo0Ut2Ipv6Add, p.r1Lo0Ut2Ipv6Add, dut1)
		    ConfigureTunnelInterface(t, "fti7", p.r0Lo0Ut3Ipv6Add, p.r1Lo0Ut3Ipv6Add, dut1)
		}
		//configure tunnel interface on dut2- IPv6
		if deviations.TunnelConfigPathUnsupported(dut2) {
		    ConfigureTunnelInterface(t, "fti4", p.r1Lo0Ut0Ipv6Add, p.r0Lo0Ut0Ipv6Add, dut2)
		    ConfigureTunnelInterface(t, "fti5", p.r1Lo0Ut1Ipv6Add, p.r0Lo0Ut1Ipv6Add, dut2)
		    ConfigureTunnelInterface(t, "fti6", p.r1Lo0Ut2Ipv6Add, p.r0Lo0Ut2Ipv6Add, dut2)
		    ConfigureTunnelInterface(t, "fti7", p.r1Lo0Ut3Ipv6Add, p.r0Lo0Ut3Ipv6Add, dut2)
		}
	})
	// configure tunnel termination on dut1
	t.Run("Configure tunnel termination at underlay interface on dut1 and dut2", func(t *testing.T) {
		ConfigureTunnelTermination(t, dut1Intf3, dut1)
		ConfigureTunnelTermination(t, dut1Intf4, dut1)
		ConfigureTunnelTermination(t, dut2Intf1, dut2)
		ConfigureTunnelTermination(t, dut2Intf2, dut2)
	})
	//configure Network Instance for both dut
	t.Run("Configure routing instance on dut1 and dut2", func(t *testing.T) {
		configureNetworkInstance(t, dut1)
		configureNetworkInstance(t, dut2)
	})


	// underylay IPv4 static route to reach tunnel-destination at dut1
	t.Run("Configure underlay IPv4 static routes on dut1", func(t *testing.T) {
	    ipv4Destination1 := GetNetworkAddress(t, p.r1Lo0Ut0Ipv4Add, int(p.ipv4Mask))
	    ipv4Destination2 := GetNetworkAddress(t, p.r1Lo0Ut1Ipv4Add, int(p.ipv4Mask))
	    ipv4Destination3 := GetNetworkAddress(t, p.r1Lo0Ut2Ipv4Add, int(p.ipv4Mask))
	    ipv4Destination4 := GetNetworkAddress(t, p.r1Lo0Ut3Ipv4Add, int(p.ipv4Mask))
		// underlay static route Nexthops
	    underlayIPv4NextHopDut1 := []string{p.r1Intf3Ipv4Add, p.r1Intf4Ipv4Add}
		for i, nextHop := range underlayIPv4NextHopDut1 {
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv4Destination1, nextHop)
			configIPv4StaticRoute(t, dut1, ipv4Destination1, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv4Destination2, nextHop)
			configIPv4StaticRoute(t, dut1, ipv4Destination2, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv4Destination3, nextHop)
			configIPv4StaticRoute(t, dut1, ipv4Destination3, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv4Destination4, nextHop)
			configIPv4StaticRoute(t, dut1, ipv4Destination4, nextHop, strconv.Itoa(i))
		}
	})

    // underylay IPv6 static route to reach tunnel-destination at dut1
	t.Run("Configure underlay IPv6 static routes on dut1", func(t *testing.T) {
	    ipv6Destination1 := GetNetworkAddress(t, p.r1Lo0Ut0Ipv6Add, int(p.ipv6Mask ))
	    ipv6Destination2 := GetNetworkAddress(t, p.r1Lo0Ut1Ipv6Add, int(p.ipv6Mask ))
	    ipv6Destination3 := GetNetworkAddress(t, p.r1Lo0Ut2Ipv6Add, int(p.ipv6Mask ))
	    ipv6Destination4 := GetNetworkAddress(t, p.r1Lo0Ut3Ipv6Add, int(p.ipv6Mask ))
		// underlay static route Nexthops
	    underlayIPv6NextHopDut1 := []string{p.r1Intf3Ipv6Add, p.r1Intf4Ipv6Add}
		for i, nextHop := range underlayIPv6NextHopDut1 {
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv6Destination1, nextHop)
			configIPv4StaticRoute(t, dut1, ipv6Destination1, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv6Destination2, nextHop)
			configIPv4StaticRoute(t, dut1, ipv6Destination2, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv6Destination3, nextHop)
			configIPv4StaticRoute(t, dut1, ipv6Destination3, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv6Destination4, nextHop)
			configIPv4StaticRoute(t, dut1, ipv6Destination4, nextHop, strconv.Itoa(i))
		}
	})

    // underylay IPv4 static route to reach tunnel-destination at dut2
	t.Run("Configure underlay IPv4 static routes on dut2", func(t *testing.T) {
	    ipv4Destination1 := GetNetworkAddress(t, p.r0Lo0Ut0Ipv4Add, int(p.ipv4Mask))
	    ipv4Destination2 := GetNetworkAddress(t, p.r0Lo0Ut1Ipv4Add, int(p.ipv4Mask))
	    ipv4Destination3 := GetNetworkAddress(t, p.r0Lo0Ut2Ipv4Add, int(p.ipv4Mask))
	    ipv4Destination4 := GetNetworkAddress(t, p.r0Lo0Ut3Ipv4Add, int(p.ipv4Mask))
		// underlay static route Nexthops
	    underlayIPv4NextHopDut2 := []string{p.r0Intf3Ipv4Add, p.r0Intf4Ipv4Add}
		for i, nextHop := range underlayIPv4NextHopDut2 {
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut2, ipv4Destination1, nextHop)
			configIPv4StaticRoute(t, dut2, ipv4Destination1, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut2, ipv4Destination2, nextHop)
			configIPv4StaticRoute(t, dut2, ipv4Destination2, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut2, ipv4Destination3, nextHop)
			configIPv4StaticRoute(t, dut2, ipv4Destination3, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut2, ipv4Destination4, nextHop)
			configIPv4StaticRoute(t, dut2, ipv4Destination4, nextHop, strconv.Itoa(i))
		}
	})

    // underylay IPv6 static route to reach tunnel-destination at dut2
	t.Run("Configure underlay IPv6 static routes on dut2", func(t *testing.T) {
	    ipv6Destination1 := GetNetworkAddress(t, p.r0Lo0Ut0Ipv6Add, int(p.ipv6Mask ))
	    ipv6Destination2 := GetNetworkAddress(t, p.r0Lo0Ut1Ipv6Add, int(p.ipv6Mask ))
	    ipv6Destination3 := GetNetworkAddress(t, p.r0Lo0Ut2Ipv6Add, int(p.ipv6Mask ))
	    ipv6Destination4 := GetNetworkAddress(t, p.r0Lo0Ut3Ipv6Add, int(p.ipv6Mask ))
		// underlay static route Nexthops
	    underlayIPv6NextHopDut2 := []string{p.r0Intf3Ipv6Add, p.r0Intf4Ipv6Add}
		for i, nextHop := range underlayIPv6NextHopDut2 {
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut2, ipv6Destination1, nextHop)
			configIPv4StaticRoute(t, dut2, ipv6Destination1, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut2, ipv6Destination2, nextHop)
			configIPv4StaticRoute(t, dut2, ipv6Destination2, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut2, ipv6Destination3, nextHop)
			configIPv4StaticRoute(t, dut2, ipv6Destination3, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut2, ipv6Destination4, nextHop)
			configIPv4StaticRoute(t, dut2, ipv6Destination4, nextHop, strconv.Itoa(i))
		}
	})

	t.Run("Telemetry: Verify all tunnel interfaces oper-state", func(t *testing.T) {
		tunnelIntf := []string{"fti0", "fti1", "fti2", "fti3", "fti4", "fti5", "fti6", "fti7"}
		const want = oc.Interface_OperStatus_UP
		for _, dp := range tunnelIntf  {

			if deviations.TunnelStatePathUnsupported(dut1) {
			   if got := gnmi.Get(t, dut1, gnmi.OC().Interface(dp).Subinterface(0).OperStatus().State()); got != want {
				    t.Errorf("device %s interface %s oper-status got %v, want %v",dut1, dp, got, want)
			    } else {
				    t.Logf("device %s interface %s oper-status got %v",dut1, dp, got)
			    }
			}
			if deviations.TunnelStatePathUnsupported(dut2) {	
			    if got := gnmi.Get(t, dut2, gnmi.OC().Interface(dp).Subinterface(0).OperStatus().State()); got != want {
				    t.Errorf("device %s interface %s oper-status got %v, want %v",dut2, dp, got, want)
			    } else {
			    	t.Logf("device %s interface %s ioper-status got %v", dut2, dp, got)
			    }
			}         
		}

	})
	

	//Configure Overlay Static routes for IPv4 at dut1
	t.Run("Configure overlay IPv4 static routes on dut1", func(t *testing.T) {
		ipv4Destination1 := GetNetworkAddress(t, p.rtIntf5Ipv4Add, int(p.ipv4Mask))
	    ipv4Destination2 := GetNetworkAddress(t, p.rtIntf6Ipv4Add, int(p.ipv4Mask))
		// overlay static route Nexthops
		overlayIPv4NextHopDut1 := []string{p.r1Fti0Ipv4Add, p.r1Fti1Ipv4Add, p.r1Fti2Ipv4Add, p.r1Fti3Ipv4Add, p.r1Fti4Ipv4Add, p.r1Fti5Ipv4Add, p.r1Fti6Ipv4Add, p.r1Fti7Ipv4Add}
		for i, nextHop := range overlayIPv4NextHopDut1 {
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv4Destination1, nextHop)
			configIPv4StaticRoute(t, dut1, ipv4Destination1, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv4Destination2, nextHop)
			configIPv4StaticRoute(t, dut1, ipv4Destination2, nextHop, strconv.Itoa(i))
		}
	})

	//Configure Overlay Static routes for IPv6 at dut1
	t.Run("Configure overlay IPv6 static routes on dut1", func(t *testing.T) {
		ipv6Destination1 := GetNetworkAddress(t, p.rtIntf5Ipv6Add, int(p.ipv6Mask ))
	    ipv6Destination2 := GetNetworkAddress(t, p.rtIntf6Ipv6Add, int(p.ipv6Mask ))
		// overlay static route Nexthops
		overlayIPv6NextHopDut1 := []string{p.r1Fti0Ipv6Add, p.r1Fti1Ipv6Add, p.r1Fti2Ipv6Add, p.r1Fti3Ipv6Add, p.r1Fti4Ipv6Add, p.r1Fti5Ipv6Add, p.r1Fti6Ipv6Add, p.r1Fti7Ipv6Add}
		for i, nextHop := range overlayIPv6NextHopDut1 {
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv6Destination1, nextHop)
			configIPv4StaticRoute(t, dut1, ipv6Destination1, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut1, ipv6Destination2, nextHop)
			configIPv4StaticRoute(t, dut1, ipv6Destination2, nextHop, strconv.Itoa(i))
		}
	})

	//Configure Overlay Static routes for IPv4 at dut2
	t.Run("Configure overlay IPv4 static routes on dut2", func(t *testing.T) {
	    ipv4Destination1 := GetNetworkAddress(t, p.rtIntf1Ipv4Add, int(p.ipv4Mask))
	    ipv4Destination2 := GetNetworkAddress(t, p.rtIntf2Ipv4Add, int(p.ipv4Mask))
		// underlay static route Nexthops
		overlayIPv4NextHopDut2 := []string{p.r0Fti0Ipv4Add, p.r0Fti1Ipv4Add, p.r0Fti2Ipv4Add, p.r0Fti3Ipv4Add, p.r0Fti4Ipv4Add, p.r0Fti5Ipv4Add, p.r0Fti6Ipv4Add, p.r0Fti7Ipv4Add}
		for i, nextHop := range overlayIPv4NextHopDut2 {
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut2, ipv4Destination1, nextHop)
			configIPv4StaticRoute(t, dut2, ipv4Destination1, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut2, ipv4Destination2, nextHop)
			configIPv4StaticRoute(t, dut2, ipv4Destination2, nextHop, strconv.Itoa(i))

		}
	})

	//Configure Overlay Static routes for IPv6 at dut2
	t.Run("Configure overlay IPv6 static routes on dut2", func(t *testing.T) {
		Ipv4destination1 := GetNetworkAddress(t, p.rtIntf1Ipv6Add, int(p.ipv6Mask ))
	    Ipv4destination2 := GetNetworkAddress(t, p.rtIntf2Ipv6Add, int(p.ipv6Mask ))
		// overlay static route Nexthops
		overlayIPv6NextHopDut2 := []string{p.r0Fti0Ipv6Add, p.r0Fti1Ipv6Add, p.r0Fti2Ipv6Add, p.r0Fti3Ipv6Add, p.r0Fti4Ipv6Add, p.r0Fti5Ipv6Add, p.r0Fti6Ipv6Add, p.r0Fti7Ipv6Add}
		for i, nextHop := range overlayIPv6NextHopDut2 {
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut2, Ipv4destination1, nextHop)
			configIPv4StaticRoute(t, dut2, Ipv4destination1, nextHop, strconv.Itoa(i))
			t.Logf("configuring static route in %s destination %s with next-hop %s", dut2, Ipv4destination2, nextHop)
			configIPv4StaticRoute(t, dut2, Ipv4destination2, nextHop, strconv.Itoa(i))

		}
	})

    // Send the traffic as mentioned in Tunnel-1.3 and Tunnel-1.4 with TP-1.1 and TP-1.2 
	otg := rt.OTG()
	var otgConfig gosnappi.Config
	t.Run("Configure ATE", func(t *testing.T) {
		t.Logf("Start ATE Config.")
		otgConfig = configureOtg(t, otg, p) 
	})
    _= otgConfig

    wantLoss := false
	t.Run("Verify load balance and traffic drops with IPv4 and IPv6 flow via 8 tunnel", func(t *testing.T) {
		t.Log("Verify load balance and traffic drops with IPv4 and IPv6 flow via 8 tunnel")
        VerifyUnderlayOverlayLoadbalanceTest(t, p, dut1, dut2, rt,  dut1Intf1, dut1Intf2, dut1Intf3, dut1Intf4, dut2Intf1, dut2Intf2, dut2Intf3, dut2Intf4, 8, wantLoss)
	})


    // CASE:2
	// Reduce number of Tunnel interfaces(e.g. From 8 to 4)
	// Delete 2 IPv4 Static route and 2 IPv6 Static router to reduce 8 to 4 tunnel interface.
	// If the static routes are used to forward traffic to tunnel, please disable or delete the static route in this test to simulate the reduction in available paths
	t.Logf("CASE:2 If the static routes are used to forward traffic to tunnel, please disable or delete the static route in this test to simulate the reduction in available paths")
	

	//delete Overlay Static routes for IPv4 at dut1
	index:=0
	t.Run("Delete overlay IPv4 static routes on dut1 and reduce the tunnel interface from 8 to 4", func(t *testing.T) {
	    ipv4Destination1 := GetNetworkAddress(t, p.rtIntf5Ipv4Add, int(p.ipv4Mask))
	    ipv4Destination2 := GetNetworkAddress(t, p.rtIntf6Ipv4Add, int(p.ipv4Mask))
		// Next hops list
		overlayIPv4NextHopDut1 := []string{ p.r1Fti2Ipv4Add, p.r1Fti3Ipv4Add,  p.r1Fti6Ipv4Add, p.r1Fti7Ipv4Add}
		for i, nextHop := range overlayIPv4NextHopDut1 {
			switch {
			    case i<2 :
					index = i+2
			    case i>=2:
					index = i+4
			}
			t.Logf("delete static route in %s destination %s with next-hop %s on index %d", dut1, ipv4Destination1, nextHop, index)
			deleteStaticRoute(t, dut1, ipv4Destination1, nextHop, strconv.Itoa(index))
			t.Logf("delete static route in %s destination %s with next-hop %s on index %d", dut1, ipv4Destination2, nextHop, index)
			deleteStaticRoute(t, dut1, ipv4Destination2, nextHop, strconv.Itoa(index))
		}
	})

	//delete Overlay Static routes for IPv6 at dut1
	t.Run("Delete overlay IPv6 static routes on dut1 and reduce the tunnel interface from 8 to 4", func(t *testing.T) {
		ipv6Destination1 := GetNetworkAddress(t, p.rtIntf5Ipv6Add, int(p.ipv6Mask ))
	    ipv6Destination2 := GetNetworkAddress(t, p.rtIntf6Ipv6Add, int(p.ipv6Mask ))
		// Next hops list
	    overlayIPv6NextHopDut1 := []string{ p.r1Fti2Ipv6Add, p.r1Fti3Ipv6Add,  p.r1Fti6Ipv6Add, p.r1Fti7Ipv6Add}
		for i, nextHop := range overlayIPv6NextHopDut1 {
			switch {
		        case i<2 :
			        index = i+2
		        case i>=2:
			        index = i+4
	        }
			t.Logf("delete static route in %s destination %s with next-hop %s on index %d", dut1, ipv6Destination1, nextHop, index)
			deleteStaticRoute(t, dut1, ipv6Destination1, nextHop, strconv.Itoa(index))
			t.Logf("delete static route in %s destination %s with next-hop %s on index %d", dut1, ipv6Destination2, nextHop, index)
			deleteStaticRoute(t, dut1, ipv6Destination2, nextHop, strconv.Itoa(index))
		}
	})

    wantLoss = true
	t.Run("Verify load balance and traffic drops with IPv4 and IPv6 flow via 4 tunnel", func(t *testing.T) {
		t.Log("Verify load balance and traffic drops with IPv4 and IPv6 flow via 4 tunnel")
        VerifyUnderlayOverlayLoadbalanceTest(t, p, dut1, dut2, rt,  dut1Intf1, dut1Intf2, dut1Intf3, dut1Intf4, dut2Intf1, dut2Intf2, dut2Intf3, dut2Intf4, 4, wantLoss)
	})

    //CASE:3
	// Increase number of Tunnel interfaces(e.g. From 4 to 8)
    // add back 4 more tunnel interface for IPv4 traffic
	t.Run("Increase the tunnel interface from 4 to 8 (Configure overlay IPv4 static routes on dut1) ", func(t *testing.T) {
	    ipv4Destination1 := GetNetworkAddress(t, p.rtIntf5Ipv4Add, int(p.ipv4Mask))
	    ipv4Destination2 := GetNetworkAddress(t, p.rtIntf6Ipv4Add, int(p.ipv4Mask))
		// Next hops list
		overlayIPv4NextHopDut1 := []string{ p.r1Fti2Ipv4Add, p.r1Fti3Ipv4Add,  p.r1Fti6Ipv4Add, p.r1Fti7Ipv4Add}
		for i, nextHop := range overlayIPv4NextHopDut1 {
            index:=0
			switch {
			    case i<2 :
					index = i+2
			    case i>=2:
					index = i+4
			}
			t.Logf("configure static route in %s destination %s with next-hop %s on index %d", dut1, ipv4Destination1, nextHop, index)
			configIPv4StaticRoute(t, dut1, ipv4Destination1, nextHop, strconv.Itoa(index))
			t.Logf("configure static route in %s destination %s with next-hop %s on index %d", dut1, ipv4Destination2, nextHop, index)
			configIPv4StaticRoute(t, dut1, ipv4Destination2, nextHop, strconv.Itoa(index))
		}
	})
	 
    // add back 4 more tunnel interface for IPv6 traffic
	t.Run("Increase the tunnel interface from 4 to 8 (Configure overlay IPv4 static routes on dut1)", func(t *testing.T) {
	    ipv6Destination1 := GetNetworkAddress(t, p.rtIntf5Ipv6Add, int(p.ipv6Mask ))
	    ipv6Destination2 := GetNetworkAddress(t, p.rtIntf6Ipv6Add, int(p.ipv6Mask ))
		// Next hops list
	    overlayIPv6NextHopDut1 := []string{ p.r1Fti2Ipv6Add, p.r1Fti3Ipv6Add,  p.r1Fti6Ipv6Add, p.r1Fti7Ipv6Add}
		for i, nextHop := range overlayIPv6NextHopDut1 {
		    switch {
		        case i<2 :
			        index = i+2
		        case i>=2:
			        index = i+4
	        }
			t.Logf("configure static route in %s destination %s with next-hop %s on index %d", dut1, ipv6Destination1, nextHop, index)
			configIPv4StaticRoute(t, dut1, ipv6Destination1, nextHop, strconv.Itoa(index))
			t.Logf("configure static route in %s destination %s with next-hop %s on index %d", dut1, ipv6Destination2, nextHop, index)
			configIPv4StaticRoute(t, dut1, ipv6Destination2, nextHop, strconv.Itoa(index))
		}
	})

    wantLoss = false
	t.Run("Verify load balance and traffic drops with IPv4 and IPv6 flow agin 8 tunnel", func(t *testing.T) {
		t.Log("Verify load balance and traffic drops with IPv4 and IPv6 flow agin 8 tunnel")
        VerifyUnderlayOverlayLoadbalanceTest(t, p, dut1, dut2, rt,  dut1Intf1, dut1Intf2, dut1Intf3, dut1Intf4, dut2Intf1, dut2Intf2, dut2Intf3, dut2Intf4, 8, wantLoss)
	})

}

func configureOtg(t *testing.T, otg *otg.OTG, p *parameters) gosnappi.Config {

	//  NewConfig creates a new OTG config.
	config := otg.NewConfig(t)
	// Add ports to config.
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")
	port3 := config.Ports().Add().SetName("port5")
	port4 := config.Ports().Add().SetName("port6")

	//port1
	iDut1Dev := config.Devices().Add().SetName("port1")
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName("port1" + ".Eth").SetMac(p.rtIntf1MacAdd)
	iDut1Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName("port1" + ".IPv4")
	iDut1Ipv4.SetAddress(p.rtIntf1Ipv4Add).SetGateway(p.r0Intf1Ipv4Add ).SetPrefix(int32(p.ipv4Mask))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName("port1" + ".IPv6")
	iDut1Ipv6.SetAddress(p.rtIntf1Ipv6Add).SetGateway(p.r0Intf1Ipv6Add).SetPrefix(int32(p.ipv6Mask ))

	//port2
	iDut2Dev := config.Devices().Add().SetName("port2")
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName("port2" + ".Eth").SetMac(p.rtIntf2MacAdd)
	iDut2Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName("port2" + ".IPv4")
	iDut2Ipv4.SetAddress(p.rtIntf2Ipv4Add).SetGateway(p.r0Intf2Ipv4Add).SetPrefix(int32(p.ipv4Mask))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName("port2" + ".IPv6")
	iDut2Ipv6.SetAddress(p.rtIntf2Ipv6Add).SetGateway(p.r0Intf2Ipv6Add).SetPrefix(int32(p.ipv6Mask ))

	//port5
	iDut3Dev := config.Devices().Add().SetName("port5")
	iDut3Eth := iDut3Dev.Ethernets().Add().SetName("port5" + ".Eth").SetMac(p.rtIntf5MacAdd)
	iDut3Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port3.Name())
	iDut3Ipv4 := iDut3Eth.Ipv4Addresses().Add().SetName("port5" + ".IPv4")
	iDut3Ipv4.SetAddress(p.rtIntf5Ipv4Add).SetGateway(p.r1Intf5Ipv4Add ).SetPrefix(int32(p.ipv4Mask))
	iDut3Ipv6 := iDut3Eth.Ipv6Addresses().Add().SetName("port5" + ".IPv6")
	iDut3Ipv6.SetAddress(p.rtIntf5Ipv6Add).SetGateway(p.r1Intf5Ipv6Add).SetPrefix(int32(p.ipv6Mask ))

	//port6
	iDut4Dev := config.Devices().Add().SetName("port6")
	iDut4Eth := iDut4Dev.Ethernets().Add().SetName("port6" + ".Eth").SetMac(p.rtIntf6MacAdd)
	iDut4Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port4.Name())
	iDut4Ipv4 := iDut4Eth.Ipv4Addresses().Add().SetName("port6" + ".IPv4")
	iDut4Ipv4.SetAddress(p.rtIntf6Ipv4Add).SetGateway(p.r1Intf6Ipv4Add).SetPrefix(int32(p.ipv4Mask))
	iDut4Ipv6 := iDut4Eth.Ipv6Addresses().Add().SetName("port6" + ".IPv6")
	iDut4Ipv6.SetAddress(p.rtIntf6Ipv6Add).SetGateway(p.r1Intf6Ipv6Add).SetPrefix(int32(p.ipv6Mask ))


	t.Logf("Start Ote Traffic config")
	t.Logf("configure IPv4 flow from %s to %s ", port1.Name(), port3.Name())
	// Set config flow
	flow1ipv4 := config.Flows().Add().SetName(p.flow1)
	flow1ipv4.Metrics().SetEnable(true)
	// Set source and reciving ports.
	flow1ipv4.TxRx().Device().
		SetTxNames([]string{iDut1Ipv4.Name()}).
		SetRxNames([]string{iDut3Ipv4.Name()})
	// Flow settings.
	flow1ipv4.Size().SetFixed(512)
	flow1ipv4.Rate().SetPps(p.trafficRate)
	flow1ipv4.Duration().SetChoice("continuous")
	// Ethernet header
	f1e1 := flow1ipv4.Packet().Add().Ethernet()
	f1e1.Src().SetValue(iDut1Eth.Mac())
	// IP header
	f1v4 := flow1ipv4.Packet().Add().Ipv4()
	// V4 source
	f1v4.Src().Increment().SetStart(iDut1Ipv4.Address()).SetCount(200)
	// V4 destination
	f1v4.Dst().SetValue(iDut3Ipv4.Address())




	t.Logf("configure IPv4 flow from %s to %s ", port2.Name(), port4.Name())
	// Set config flow
	flow2ipv4 := config.Flows().Add().SetName(p.flow2)
	flow2ipv4.Metrics().SetEnable(true)
	// Set source and reciving ports.
	flow2ipv4.TxRx().Device().
		SetTxNames([]string{iDut2Ipv4.Name()}).
		SetRxNames([]string{iDut4Ipv4.Name()})
	// Flow settings.
	flow2ipv4.Size().SetFixed(512)
	flow2ipv4.Rate().SetPps(p.trafficRate)
	flow2ipv4.Duration().SetChoice("continuous")
	// Ethernet header
	f2e1 := flow2ipv4.Packet().Add().Ethernet()
	f2e1.Src().SetValue(iDut2Eth.Mac())
	// IP header
	f2v4 := flow2ipv4.Packet().Add().Ipv4()
	// V4 source
	f2v4.Src().Increment().SetStart(iDut2Ipv4.Address()).SetCount(200)
	// V4 destination
	f2v4.Dst().SetValue(iDut4Ipv4.Address())

	t.Logf("configure IPv6 flow from %s to %s ", port1.Name(), port3.Name())
	// Set config flow
	flow3ipv6 := config.Flows().Add().SetName(p.flow3)
	flow3ipv6.Metrics().SetEnable(true)
	// Set source and reciving ports.
	flow3ipv6.TxRx().Device().
		SetTxNames([]string{iDut1Ipv6.Name()}).
		SetRxNames([]string{iDut3Ipv6.Name()})
	// Flow settings.
	flow3ipv6.Size().SetFixed(512)
	flow3ipv6.Rate().SetPps(p.trafficRate)
	flow3ipv6.Duration().SetChoice("continuous")
	// Ethernet header
	f3e1 := flow3ipv6.Packet().Add().Ethernet()
	f3e1.Src().SetValue(iDut1Eth.Mac())
	// IPv6 header
	f3v6 := flow3ipv6.Packet().Add().Ipv6()
	// V6 source
	f3v6.Src().Increment().SetStart(iDut1Ipv6.Address()).SetCount(200)
	// V6 destination
	f3v6.Dst().SetValue(iDut3Ipv6.Address())

	t.Logf("configure IPv6 flow from %s to %s ", port2.Name(), port4.Name())
	// Set config flow
	flow4ipv6 := config.Flows().Add().SetName(p.flow4)
	flow4ipv6.Metrics().SetEnable(true)
	// Set source and reciving ports.
	flow4ipv6.TxRx().Device().
		SetTxNames([]string{iDut2Ipv6.Name()}).
		SetRxNames([]string{iDut4Ipv6.Name()})
	// Flow settings.
	flow4ipv6.Size().SetFixed(512)
	flow4ipv6.Rate().SetPps(p.trafficRate)
	flow4ipv6.Duration().SetChoice("continuous")
	// Ethernet header
	f4e1 := flow4ipv6.Packet().Add().Ethernet()
	f4e1.Src().SetValue(iDut2Eth.Mac())
	// IPv6 header
	f4v6 := flow4ipv6.Packet().Add().Ipv6()
	// V6 source
	f4v6.Src().Increment().SetStart(iDut2Ipv6.Address()).SetCount(200)
	// V6 destination
	f4v6.Dst().SetValue(iDut4Ipv6.Address())
   
	//t.Logf(config.ToJson())
	t.Logf("Pushing Traffic config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
	otgutils.WaitForARP(t, otg, config, "IPv4")
	otgutils.WaitForARP(t, otg, config, "IPv6")
	return config
}

// verifyTraffic confirms that every traffic flow has the expected amount of loss (0% or 100%
// depending on wantLoss, +- 5%).
func VerifyTraffic(t *testing.T, ate *ondatra.ATEDevice, flowName string, wantLoss bool) {
	otg := ate.OTG()
	tolerancePct := 5
	t.Logf("Verifying flow metrics for flow %s\n", flowName)
	recvMetric := gnmi.Get(t, otg, gnmi.OTG().Flow(flowName).State())
	txPackets := recvMetric.GetCounters().GetOutPkts()
	t.Logf("Flow: %s transmitted packets: %d !", flowName,txPackets)
	rxPackets := recvMetric.GetCounters().GetInPkts()
	t.Logf("Flow: %s received packets: %d !", flowName,rxPackets)
	lostPackets := txPackets - rxPackets
	t.Logf("Flow: %s lost packets: %d !", flowName,lostPackets)
	lossPct := lostPackets * 100 / txPackets
	t.Logf("Flow: %s packet loss percent : %d !", flowName,lossPct)
	t.Logf("Traffic Loss Test Validation")
	if wantLoss {
		if lossPct < 100-uint64(tolerancePct) {
			t.Errorf("Traffic is expected to fail %s\n got %v, want 100%% failure", flowName, lossPct)
		} else {
			t.Logf("Traffic Loss Test Passed!")
		}
	} else {
		if lossPct > uint64(tolerancePct) {
			t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want 0", flowName, lossPct)
		} else {
			t.Logf("Traffic No Loss Test Passed!")
		}
	}
}

func SendTraffic(t *testing.T, ate *ondatra.ATEDevice,p *parameters) {
    otg := ate.OTG()
	t.Logf("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(time.Duration(p.trafficDuration) * time.Second)
	t.Logf("Stop traffic")
	otg.StopTraffic(t)
}

func VerifyLoadbalance(t *testing.T,flowCount int64,rate int64, duration int64, sharingIntfCont int64, initialStats int64, finalStats int64) {

	tolerance := 20
	// colculate correct stats on interface
	stats:= finalStats - initialStats
	expectedTotalPkts:=  (flowCount * rate * duration )
	expectedPerLinkPkts:=expectedTotalPkts/sharingIntfCont
	t.Logf("Total packets %d flow through the %d links", expectedTotalPkts, sharingIntfCont)
	t.Logf("Expected per link packets %d ",expectedPerLinkPkts)
    min:= expectedPerLinkPkts-(expectedPerLinkPkts * int64(tolerance)/100)
	max:= expectedPerLinkPkts+(expectedPerLinkPkts * int64(tolerance)/100)

	if min < stats && stats < max {
		t.Logf("Traffic  %d is in expected range: %d - %d", stats, min, max )
		t.Logf("Traffic Load balance Test Passed!")
	} else {
		t.Errorf("Traffic is expected in range %d - %d but got %d. Load balance Test Failed\n", min, max, stats )
		
	}

}

func VerifyUnderlayOverlayLoadbalanceTest(t *testing.T,p *parameters, dut1 *ondatra.DUTDevice, dut2 *ondatra.DUTDevice, rt *ondatra.ATEDevice,  dut1Intf1 *ondatra.Port, dut1Intf2 *ondatra.Port, dut1Intf3 *ondatra.Port, dut1Intf4 *ondatra.Port,  dut2Intf1 *ondatra.Port, dut2Intf2 *ondatra.Port, dut2Intf3 *ondatra.Port, dut2Intf4 *ondatra.Port, ftiIntfCount int64, wantLoss bool){

	// dut1 interface statistics
	initialInfStats :=map[string]uint64{}
	initialInfStats["dut1InputIntf1InPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(dut1Intf1.Name()).Counters().InUnicastPkts().State())
	initialInfStats["dut1InputIntf2InPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(dut1Intf2.Name()).Counters().InUnicastPkts().State())
	initialInfStats["dut1OutputIntf3OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(dut1Intf3.Name()).Counters().OutUnicastPkts().State())
	initialInfStats["dut1OutputIntf4OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(dut1Intf4.Name()).Counters().OutUnicastPkts().State())
   
	t.Logf("Initial ingress interface: %v input pkts stats: %d at dut1\n", dut1Intf1 ,initialInfStats["dut1InputIntf1InPkts"])
	t.Logf("Initial ingress interface: %v input pkts stats: %d at dut1\n", dut1Intf2 ,initialInfStats["dut1InputIntf2InPkts"])
	t.Logf("Initial egress interface: %v output pkts stats: %d at dut1\n", dut1Intf3 ,initialInfStats["dut1OutputIntf3OutPkts"])
	t.Logf("Initial egress interface: %v output pkts stats: %d at dut1\n", dut1Intf4 ,initialInfStats["dut1OutputIntf4OutPkts"])	
	//dut2 interface statistics
	initialInfStats["dut2InputIntf1InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(dut2Intf1.Name()).Counters().InUnicastPkts().State())
	initialInfStats["dut2InputIntf2InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(dut2Intf2.Name()).Counters().InUnicastPkts().State())
	initialInfStats["dut2OutputIntf3OutPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(dut2Intf3.Name()).Counters().OutUnicastPkts().State())
	initialInfStats["dut2OutputIntf4IutPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(dut2Intf4.Name()).Counters().OutUnicastPkts().State())
   
	t.Logf("Initial ingress interface: %v input pkts stats: %d at dut2\n", dut2Intf1 ,initialInfStats["dut2InputIntf1InPkts"])
	t.Logf("Initial ingress interface: %v input pkts stats: %d at dut2\n", dut2Intf2 ,initialInfStats["dut2InputIntf2InPkts"])
	t.Logf("Initial egress interface: %v output pkts stats: %d at dut2\n", dut2Intf3 ,initialInfStats["dut2OutputIntf3OutPkts"])
	t.Logf("Initial egress interface: %v output pkts stats: %d at dut2\n", dut2Intf4 ,initialInfStats["dut2OutputIntf4IutPkts"])	
   
    
	// Verify the tunnel interfaces traffic/flow for equal distribution for optimal load balancing 
    if deviations.TunnelStatePathUnsupported(dut1) {
		dut1InitialFitStats :=map[string]uint64{}
		dut1InitialFitStats["dut1Fti0OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti0").Subinterface(0).Counters().OutPkts().State())
		dut1InitialFitStats["dut1Fti1OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti1").Subinterface(0).Counters().OutPkts().State())
		dut1InitialFitStats["dut1Fti2OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti2").Subinterface(0).Counters().OutPkts().State())
		dut1InitialFitStats["dut1Fti3OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti3").Subinterface(0).Counters().OutPkts().State())
		dut1InitialFitStats["dut1Fti4OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti4").Subinterface(0).Counters().OutPkts().State())
		dut1InitialFitStats["dut1Fti5OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti5").Subinterface(0).Counters().OutPkts().State())
		dut1InitialFitStats["dut1Fti6OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti6").Subinterface(0).Counters().OutPkts().State())
		dut1InitialFitStats["dut1Fti7OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti7").Subinterface(0).Counters().OutPkts().State())	

		t.Logf("Encapsulating router inital fti0 interface OutPkts stats: %d\n",  dut1InitialFitStats["dut1Fti0OutPkts"])
		t.Logf("Encapsulating router inital fti1 interface OutPkts stats: %d\n",  dut1InitialFitStats["dut1Fti1OutPkts"])
		t.Logf("Encapsulating router inital fti2 interface OutPkts stats: %d\n",  dut1InitialFitStats["dut1Fti2OutPkts"])
		t.Logf("Encapsulating router inital fti3 interface OutPkts stats: %d\n",  dut1InitialFitStats["dut1Fti3OutPkts"])
		t.Logf("Encapsulating router inital fti4 interface OutPkts stats: %d\n",  dut1InitialFitStats["dut1Fti4OutPkts"])
		t.Logf("Encapsulating router inital fti5 interface OutPkts stats: %d\n",  dut1InitialFitStats["dut1Fti5OutPkts"])
		t.Logf("Encapsulating router inital fti6 interface OutPkts stats: %d\n",  dut1InitialFitStats["dut1Fti6OutPkts"])
		t.Logf("Encapsulating router inital fti7 interface OutPkts stats: %d\n",  dut1InitialFitStats["dut1Fti7OutPkts"])
	}
	if deviations.TunnelStatePathUnsupported(dut2) {
		dut2InitialFitStats :=map[string]uint64{}
		dut2InitialFitStats["dut2Fti0InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti0").Subinterface(0).Counters().InPkts().State())
		dut2InitialFitStats["dut2Fti1InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti1").Subinterface(0).Counters().InPkts().State())
		dut2InitialFitStats["dut2Fti2InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti2").Subinterface(0).Counters().InPkts().State())
		dut2InitialFitStats["dut2Fti3InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti3").Subinterface(0).Counters().InPkts().State())
		dut2InitialFitStats["dut2Fti4InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti4").Subinterface(0).Counters().InPkts().State())
		dut2InitialFitStats["dut2Fti5InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti5").Subinterface(0).Counters().InPkts().State())
		dut2InitialFitStats["dut2Fti6InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti6").Subinterface(0).Counters().InPkts().State())
		dut2InitialFitStats["dut2Fti7InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti7").Subinterface(0).Counters().InPkts().State())	

		t.Logf("Decapsulating router inital fti0 interface InPkts stats: %d\n",  dut2InitialFitStats["dut2Fti0InPkts"])
		t.Logf("Decapsulating router inital fti1 interface InPkts stats: %d\n",  dut2InitialFitStats["dut2Fti1InPkts"])
		t.Logf("Decapsulating router inital fti2 interface InPkts stats: %d\n",  dut2InitialFitStats["dut2Fti2InPkts"])
		t.Logf("Decapsulating router inital fti3 interface InPkts stats: %d\n",  dut2InitialFitStats["dut2Fti3InPkts"])
		t.Logf("Decapsulating router inital fti4 interface InPkts stats: %d\n",  dut2InitialFitStats["dut2Fti4InPkts"])
		t.Logf("Decapsulating router inital fti5 interface InPkts stats: %d\n",  dut2InitialFitStats["dut2Fti5InPkts"])
		t.Logf("Decapsulating router inital fti6 interface InPkts stats: %d\n",  dut2InitialFitStats["dut2Fti6InPkts"])
		t.Logf("Decapsulating router inital fti7 interface InPkts stats: %d\n",  dut2InitialFitStats["dut2Fti7InPkts"])

	}

    // Verify GRE Traffic loss at ATE 
	wantDrops := false
	t.Log("Send and validate traffic from ATE Port1 and Port2")
	SendTraffic(t, rt, p)

    flows := []string{p.flow1, p.flow2,p.flow3,p.flow4}
	for i, flowName := range flows {
			t.Logf("Verify flow %d stats", i)
            VerifyTraffic(t, rt, flowName, wantDrops)
	}

	

	// Incoming traffic flow should be equally distributed for Encapsulation(ECMP) 
    // dut1 physical interface statistics
	finalInfStats :=map[string]uint64{}
	finalInfStats["dut1InputIntf1InPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(dut1Intf1.Name()).Counters().InUnicastPkts().State())
	finalInfStats["dut1InputIntf2InPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(dut1Intf2.Name()).Counters().InUnicastPkts().State())
	finalInfStats["dut1OutputIntf3OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(dut1Intf3.Name()).Counters().OutUnicastPkts().State())
	finalInfStats["dut1OutputIntf4OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface(dut1Intf4.Name()).Counters().OutUnicastPkts().State())

	t.Logf("After Traffic Test ingress interface: %v input pkts stats: %d at dut1\n", dut1Intf1 ,finalInfStats["dut1InputIntf1InPkts"])
	t.Logf("After Traffic Test ingress interface: %v input pkts stats: %d at dut1\n", dut1Intf2 ,finalInfStats["dut1InputIntf2InPkts"])
	t.Logf("After Traffic Test egress interface: %v output pkts stats: %d at dut1\n", dut1Intf3 ,finalInfStats["dut1OutputIntf3OutPkts"])
	t.Logf("After Traffic Test egress interface: %v output pkts stats: %d at dut1\n", dut1Intf4 ,finalInfStats["dut1OutputIntf4OutPkts"])	
	//dut2 physical interface statistics
	finalInfStats["dut2InputIntf1InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(dut2Intf1.Name()).Counters().InUnicastPkts().State())
	finalInfStats["dut2InputIntf2InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(dut2Intf2.Name()).Counters().InUnicastPkts().State())
	finalInfStats["dut2OutputIntf3OutPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(dut2Intf3.Name()).Counters().OutUnicastPkts().State())
	finalInfStats["dut2OutputIntf4IutPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface(dut2Intf4.Name()).Counters().OutUnicastPkts().State())

	t.Logf("After Traffic Test ingress interface: %v input pkts stats: %d at dut2\n", dut2Intf1 ,finalInfStats["dut2InputIntf1InPkts"])
	t.Logf("After Traffic Test ingress interface: %v input pkts stats: %d at dut2\n", dut2Intf2 ,finalInfStats["dut2InputIntf2InPkts"])
	t.Logf("After Traffic Test egress interface: %v output pkts stats: %d at dut2\n", dut2Intf3 ,finalInfStats["dut2OutputIntf3OutPkts"])
	t.Logf("After Traffic Test egress interface: %v output pkts stats: %d at dut2\n", dut2Intf4 ,finalInfStats["dut2OutputIntf4IutPkts"])	

	// Incoming traffic flow should be equally distributed for Encapsulation(ECMP) 
	t.Logf("Verify Underlay loadbalancing 2 fti tunnel interface - Incoming traffic flow should be equally distributed for Encapsulation(ECMP) ")
    for key,_:= range finalInfStats {
		VerifyLoadbalance(t , 4, p.trafficRate, p.trafficDuration, 2, int64(initialInfStats[key]) , int64(finalInfStats[key]))
    }

	// Verify the tunnel interfaces traffic/flow for equal distribution for optimal load balancing 
	if deviations.TunnelStatePathUnsupported(dut1) {
		dut1FinalFitStats :=map[string]uint64{}
		dut1FinalFitStats["dut1Fti0OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti0").Subinterface(0).Counters().OutPkts().State())
		dut1FinalFitStats["dut1Fti1OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti1").Subinterface(0).Counters().OutPkts().State())
		dut1FinalFitStats["dut1Fti2OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti2").Subinterface(0).Counters().OutPkts().State())
		dut1FinalFitStats["dut1Fti3OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti3").Subinterface(0).Counters().OutPkts().State())
		dut1FinalFitStats["dut1Fti4OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti4").Subinterface(0).Counters().OutPkts().State())
		dut1FinalFitStats["dut1Fti5OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti5").Subinterface(0).Counters().OutPkts().State())
		dut1FinalFitStats["dut1Fti6OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti6").Subinterface(0).Counters().OutPkts().State())
		dut1FinalFitStats["dut1Fti7OutPkts"] = gnmi.Get(t, dut1, gnmi.OC().Interface("fti7").Subinterface(0).Counters().OutPkts().State())	

		t.Logf("Encapsulating router Final fti0 interface OutPkts stats: %d\n",  dut1FinalFitStats["dut1Fti0OutPkts"])
		t.Logf("Encapsulating router Final fti1 interface OutPkts stats: %d\n",  dut1FinalFitStats["dut1Fti1OutPkts"])
		t.Logf("Encapsulating router Final fti2 interface OutPkts stats: %d\n",  dut1FinalFitStats["dut1Fti2OutPkts"])
		t.Logf("Encapsulating router Final fti3 interface OutPkts stats: %d\n",  dut1FinalFitStats["dut1Fti3OutPkts"])
		t.Logf("Encapsulating router Final fti4 interface OutPkts stats: %d\n",  dut1FinalFitStats["dut1Fti4OutPkts"])
		t.Logf("Encapsulating router Final fti5 interface OutPkts stats: %d\n",  dut1FinalFitStats["dut1Fti5OutPkts"])
		t.Logf("Encapsulating router Final fti6 interface OutPkts stats: %d\n",  dut1FinalFitStats["dut1Fti6OutPkts"])
		t.Logf("Encapsulating router Final fti7 interface OutPkts stats: %d\n",  dut1FinalFitStats["dut1Fti7OutPkts"])

    	t.Logf("Verify Overlay loadbalancing at encapsulation router- Verify the tunnel interfaces traffic/flow for equal distribution for optimal load balancing ) ")
    	for key,_:= range dut1FinalFitStats {
			if wantLoss {
				if key =="dut1Fti2OutPkts" || key =="dut1Fti3OutPkts" || key =="dut1Fti6OutPkts" ||key =="dut1Fti7OutPkts" {
        	       t.Logf("Due tunnel resize test. Skiping reduced fti interface stats: %s",key)
            	   continue 
            	} 
				VerifyLoadbalance(t , 4, p.trafficRate, p.trafficDuration, ftiIntfCount, int64(dut1InitialFitStats[key]) , int64(dut1FinalFitStats[key]))
			} else {
		   		VerifyLoadbalance(t , 4, p.trafficRate, p.trafficDuration, ftiIntfCount, int64(dut1InitialFitStats[key]) , int64(dut1FinalFitStats[key]))
        	}
		}
	}
	if deviations.TunnelStatePathUnsupported(dut2) {
		dut2FinalFitStats :=map[string]uint64{}
		dut2FinalFitStats["dut2Fti0InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti0").Subinterface(0).Counters().InPkts().State())
		dut2FinalFitStats["dut2Fti1InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti1").Subinterface(0).Counters().InPkts().State())
		dut2FinalFitStats["dut2Fti2InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti2").Subinterface(0).Counters().InPkts().State())
		dut2FinalFitStats["dut2Fti3InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti3").Subinterface(0).Counters().InPkts().State())
		dut2FinalFitStats["dut2Fti4InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti4").Subinterface(0).Counters().InPkts().State())
		dut2FinalFitStats["dut2Fti5InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti5").Subinterface(0).Counters().InPkts().State())
		dut2FinalFitStats["dut2Fti6InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti6").Subinterface(0).Counters().InPkts().State())
		dut2FinalFitStats["dut2Fti7InPkts"] = gnmi.Get(t, dut2, gnmi.OC().Interface("fti7").Subinterface(0).Counters().InPkts().State())	

		t.Logf("Decapsulating router Final fti0 interface InPkts stats: %d\n",  dut2FinalFitStats["dut2Fti0InPkts"])
		t.Logf("Decapsulating router Final fti1 interface InPkts stats: %d\n",  dut2FinalFitStats["dut2Fti1InPkts"])
		t.Logf("Decapsulating router Final fti2 interface InPkts stats: %d\n",  dut2FinalFitStats["dut2Fti2InPkts"])
		t.Logf("Decapsulating router Final fti3 interface InPkts stats: %d\n",  dut2FinalFitStats["dut2Fti3InPkts"])
		t.Logf("Decapsulating router Final fti4 interface InPkts stats: %d\n",  dut2FinalFitStats["dut2Fti4InPkts"])
		t.Logf("Decapsulating router Final fti5 interface InPkts stats: %d\n",  dut2FinalFitStats["dut2Fti5InPkts"])
		t.Logf("Decapsulating router Final fti6 interface InPkts stats: %d\n",  dut2FinalFitStats["dut2Fti6InPkts"])
		t.Logf("Decapsulating router Final fti7 interface InPkts stats: %d\n",  dut2FinalFitStats["dut2Fti7InPkts"])

    	t.Logf("Verify Overlay loadbalancing at decapsulation router- Verify the tunnel interfaces traffic/flow for equal distribution for optimal load balancing ) ")
    	for key,_:= range dut2FinalFitStats {
			if wantLoss {
				if key =="dut2Fti2InPkts" || key =="dut2Fti3InPkts" || key =="dut2Fti6InPkts" ||key =="dut2Fti7InPkts" {
       	            t.Logf("Due tunnel resize test Skiping reduced fti interface stats: %s",key)
               	    continue 
            	} 
            	VerifyLoadbalance(t , 4, p.trafficRate, p.trafficDuration, ftiIntfCount, int64(dut2InitialFitStats[key]) , int64(dut2FinalFitStats[key]))
			} else {
		     	VerifyLoadbalance(t , 4, p.trafficRate, p.trafficDuration, ftiIntfCount, int64(dut2InitialFitStats[key]) , int64(dut2FinalFitStats[key]))
       	    }
		}
	}
}