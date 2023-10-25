// Copyright 2023 Google LLC
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

package tun_acl_loadbalance_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/acl"
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test topology.
//
// The testbed consists of:
//
//	ate:port1 -> dut:port1 and
//	dut:port2 -> ate:port2
//  dut:port3 -> ate:port3
//  dut:port4 -> ate:port4
//  Ports2,3 and 4 are used as LAG interface.

//	* ate:port1 -> dut:port1 subnet 192.0.2.2/30 and 2001:db8::2/126
//	* ate:port2,3 and 4 -> dut:port2,3 and 4 subnet 192.0.2.6/30 and 2001:db8::6/126
//
//	* Traffic is sent from 192.0.2.2, 2001:db8::2(ate:port1) to 192.0.2.6, 2001:db8::6(LAG iterface).
//	  Dut Interfaces are configured as mentioned below:
//	  dut:port1 -> 192.0.2.1, 2001:db8::1
//	  dut:LAG iterface -> 192.0.2.5, 2001:db8::5
//	  Verification of Gre protocol value in the packet capture.

const (
	ipv4PrefixLen    = 30
	ipv4Header       = "Ipv4"
	pps              = 100
	FrameSize        = 512
	aclIpv4Name      = "filterIpv4"
	aclIpv6Name      = "filterIpv6"
	termName         = "t1"
	encapSrcMatch    = "192.0.2.2"
	encapDstMatch    = "192.0.2.6"
	encapV6SrcMatch  = "2001:db8::2"
	encapv6DstMatch  = "2001:db8::6"
	countIpv4        = "GreFilterIpv4Count"
	countIpv6        = "GreFilterIpv6Count"
	tunnelEndpoint   = "tunnelEncapIpv4"
	tunnelV6Endpoint = "tunnelEncapIpv6"
	greSrcAddr       = "198.51.100.1"
	greV6SrcAddr     = "2002:db8::2"
	greDstAddr       = "203.0.113.1/32"
	greV6DstAddr     = "2002:db8::6"
	GreProtocol      = 47
	tunnelActionIpv4 = "tunnelEncapIpv4"
	tunnelActionIpv6 = "tunnelEncapIpv6"
	tolerance        = 50
	lossTolerance    = 2
	prefix           = "0.0.0.0/0"
	nexthop          = "192.0.2.6"
	ipv6Prefix       = "0::0/0"
	ipv6Nexthop      = "2001:db8::6"
	lagTypeLACP      = oc.IfAggregate_AggregationType_LACP
	plen4            = 30
	plen6            = 126
	opUp             = oc.Interface_OperStatus_UP
	ethernetCsmacd   = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	ieee8023adLag    = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	cnt              = 20
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "dutsrc",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::1",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	ateSrc = attrs.Attributes{
		Name:    "atesrc",
		MAC:     "02:11:01:00:00:01",
		IPv4:    "192.0.2.2",
		IPv6:    "2001:db8::2",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}

	dutDst = attrs.Attributes{
		Desc:    "dutdst",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::5",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
	ateDst = attrs.Attributes{
		Name:    "atedst",
		MAC:     "02:12:01:00:00:01",
		IPv4:    "192.0.2.6",
		IPv6:    "2001:db8::6",
		IPv4Len: plen4,
		IPv6Len: plen6,
	}
)

// configACLInterface configures the ACL attachment on interface
func configACLInterface(t *testing.T, iFace *oc.Acl_Interface, ifName string) *acl.Acl_InterfacePath {
	aclConf := gnmi.OC().Acl().Interface(ifName)
	if ifName != "" {
		iFace.GetOrCreateIngressAclSet(aclIpv4Name, oc.Acl_ACL_TYPE_ACL_IPV4)
		iFace.GetOrCreateInterfaceRef().Interface = ygot.String(ifName)
		iFace.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	} else {
		iFace.GetOrCreateIngressAclSet(aclIpv4Name, oc.Acl_ACL_TYPE_ACL_IPV4)
		iFace.DeleteIngressAclSet(aclIpv4Name, oc.Acl_ACL_TYPE_ACL_IPV4)
	}
	return aclConf
}
func configACLIpv6Interface(t *testing.T, iFace *oc.Acl_Interface, ifName string) *acl.Acl_InterfacePath {
	aclConf := gnmi.OC().Acl().Interface(ifName)
	if ifName != "" {
		iFace.GetOrCreateIngressAclSet(aclIpv6Name, oc.Acl_ACL_TYPE_ACL_IPV6)
		iFace.GetOrCreateInterfaceRef().Interface = ygot.String(ifName)
		iFace.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	} else {
		iFace.GetOrCreateIngressAclSet(aclIpv6Name, oc.Acl_ACL_TYPE_ACL_IPV6)
		iFace.DeleteIngressAclSet(aclIpv6Name, oc.Acl_ACL_TYPE_ACL_IPV6)
	}
	return aclConf
}

// configureNetworkInstance Configure default network instance
func configureNetworkInstance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
	configureStaticRoute(t, dut, prefix, nexthop)
	configureStaticRoute(t, dut, ipv6Prefix, ipv6Nexthop)
}

// configStaticRoute configures a static route.
func configureStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string) {
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(nexthop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

func validatePackets(t *testing.T, filename string) {
	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	cntNonGrePackets := 0
	ipLayerNillcount := 0
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		ethLayer := packet.Layer(layers.LayerTypeEthernet)
		ethPacket, _ := ethLayer.(*layers.Ethernet)

		if ethPacket == nil {
			fmt.Println("EthernetType:", ethPacket.EthernetType)
		}
		resetcounter := 0
		if ipLayer == nil {
			resetcounter++
			ipLayerNillcount++

		}
		fmt.Println("cntNillIpLayer:", resetcounter)
		if resetcounter == 0 {
			ipPacket, _ := ipLayer.(*layers.IPv4)
			fmt.Printf("IpLayer is : %d ", ipPacket)
			//	ipPacket, _ := ipLayer.(*layers.IPv4)
			if ipPacket.Protocol != GreProtocol {
				cntNonGrePackets++
			}
		}
		fmt.Println("cntNonGrePackets:", cntNonGrePackets)
		if ipLayerNillcount > cnt {
			t.Errorf("%d ipLayer Packets are nil", ipLayerNillcount)
		}
		if cntNonGrePackets > cnt {
			t.Errorf("%d Packets are not encapslated properly.", cntNonGrePackets)
		}

	}
}

func validateV6Packets(t *testing.T, filename string) {
	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	cntNonGreIpv6Packets := 0
	ipv6LayerNillcount := 0
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv6)
		//	gre := packet.Layer(layers.LayerTypeGRE).(*layers.GRE)
		resetcounter := 0
		if ipLayer == nil {
			resetcounter++
			ipv6LayerNillcount++

		}
		if resetcounter == 0 {
			ipV6Packet, _ := ipLayer.(*layers.IPv6)
			if ipV6Packet.NextHeader != GreProtocol {
				cntNonGreIpv6Packets++
			}
		}
		if ipv6LayerNillcount > cnt {
			t.Errorf("%d ipLayer Packets are nil", ipv6LayerNillcount)
		}
		if cntNonGreIpv6Packets > cnt {
			t.Errorf("%d Packets are not encapslated properly.", cntNonGreIpv6Packets)
		}
	}
}

func juniperEncapCLI(family string, aclName string, encapSrcMatch string, encapDstMatch string, count string, encapAction string, endPoint string, greFamily string, greSrcAddr string, greDstAddr string) string {
	return fmt.Sprintf(`
	firewall {
		family %s {
			filter %s {
				term t1 {
					from {
						source-address {
							%s;
						}
						destination-address {
							%s;
						}
					}
					then count %s;
					then encapsulate %s
				}
				term default {
					then count default;
				}
		
			}
		}
	    tunnel-end-point %s {
			%s {
				source-address %s;
				destination-address %s;
			}
			gre;
		}
	}
  `, family, aclName, encapSrcMatch, encapDstMatch, count, encapAction, endPoint, greFamily, greSrcAddr, greDstAddr)
}

func buildCliConfigRequest(config string) *gpb.SetRequest {
	gpbSetRequest := &gpb.SetRequest{
		Update: []*gpb.Update{{
			Path: &gpb.Path{
				Origin: "cli",
			},
			Val: &gpb.TypedValue{
				Value: &gpb.TypedValue_AsciiVal{
					AsciiVal: config,
				},
			},
		}},
	}
	return gpbSetRequest
}

// sortPorts sorts the ports by the testbed port ID.
func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.SliceStable(ports, func(i, j int) bool {
		return ports[i].ID() < ports[j].ID()
	})
	return ports
}

type testCase struct {
	desc    string
	dut     *ondatra.DUTDevice
	ate     *ondatra.ATEDevice
	top     gosnappi.Config
	lagType oc.E_IfAggregate_AggregationType

	dutPorts []*ondatra.Port
	atePorts []*ondatra.Port
	aggID    string
	l3header string
}

func (tc *testCase) configureATE(t *testing.T) gosnappi.Config {
	if len(tc.atePorts) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got: %v", tc.atePorts)
	}

	p0 := tc.atePorts[0]
	tc.top.Ports().Add().SetName(p0.ID())
	d0 := tc.top.Devices().Add().SetName(ateSrc.Name)
	srcEth := d0.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	srcEth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(p0.ID())
	srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4").SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	srcEth.Ipv6Addresses().Add().SetName(ateSrc.Name + ".IPv6").SetAddress(ateSrc.IPv6).SetGateway(dutSrc.IPv6).SetPrefix(uint32(ateSrc.IPv6Len))

	agg := tc.top.Lags().Add().SetName("LAG")
	//	var port string
	for i, p := range tc.atePorts[1:] {
		port := tc.top.Ports().Add().SetName(p.ID())
		lagPort := agg.Ports().Add()
		newMac, err := incrementMAC(ateDst.MAC, i+1)
		if err != nil {
			t.Fatal(err)
		}
		lagPort.SetPortName(port.Name()).
			Ethernet().SetMac(newMac).
			SetName("LAGRx-" + strconv.Itoa(i))
		lagPort.Lacp().SetActorPortNumber(uint32(i + 1)).SetActorPortPriority(1).SetActorActivity("active")
		if i == 0 {
			tc.top.Captures().Add().SetName("capture").SetPortNames([]string{p.ID()}).SetFormat(gosnappi.CaptureFormat.PCAP)
		}
	}

	agg.Protocol().Lacp().SetActorKey(1).SetActorSystemPriority(1).SetActorSystemId("01:01:01:01:01:01")
	// Disable FEC for 100G-FR ports because Novus does not support it.
	p100gbasefr := []string{}
	for _, p := range tc.atePorts {
		if p.PMD() == ondatra.PMD100GBASEFR {
			p100gbasefr = append(p100gbasefr, p.ID())
		}
	}

	if len(p100gbasefr) > 0 {
		l1Settings := tc.top.Layer1().Add().SetName("L1").SetPortNames(p100gbasefr)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}

	dstDev := tc.top.Devices().Add().SetName(agg.Name() + ".dev")
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	dstEth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.LAG_NAME).SetLagName(agg.Name())
	dstEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	dstEth.Ipv6Addresses().Add().SetName(ateDst.Name + ".IPv6").SetAddress(ateDst.IPv6).SetGateway(dutDst.IPv6).SetPrefix(uint32(ateDst.IPv6Len))

	tc.ate.OTG().PushConfig(t, tc.top)
	tc.ate.OTG().StartProtocols(t)
	return tc.top
}

func (tc *testCase) setupAggregateAtomically(t *testing.T) {
	d := &oc.Root{}

	if tc.lagType == lagTypeLACP {
		d.GetOrCreateLacp().GetOrCreateInterface(tc.aggID)
	}

	agg := d.GetOrCreateInterface(tc.aggID)
	agg.GetOrCreateAggregation().LagType = tc.lagType
	agg.Type = ieee8023adLag

	for n, port := range tc.dutPorts {
		if n < 1 {
			// We designate port 0 as the source link, not part of LAG.
			continue
		}
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(tc.aggID)
		i.Type = ethernetCsmacd
		i.Enabled = ygot.Bool(true)

	}
	p := gnmi.OC()
	fptest.LogQuery(t, fmt.Sprintf("%s to Update()", tc.dut), p.Config(), d)
	gnmi.Update(t, tc.dut, p.Config(), d)
}

func (tc *testCase) clearAggregateMembers(t *testing.T) {
	for n, port := range tc.dutPorts {
		if n < 1 {
			// We designate port 0 as the source link, not part of LAG.
			continue
		}
		gnmi.Delete(t, tc.dut, gnmi.OC().Interface(port.Name()).Ethernet().AggregateId().Config())
	}
}

// Wait for LAG ports to be collecting and distributing and the LAG groups to be up on DUT and OTG
func (tc *testCase) verifyLAG(t *testing.T) {
	t.Logf("Waiting for LAG group on DUT to be up")
	_, ok := gnmi.Watch(t, tc.dut, gnmi.OC().Interface(tc.aggID).OperStatus().State(), time.Minute, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, present := val.Val()
		return present && status == opUp
	}).Await(t)
	if !ok {
		t.Fatalf("DUT LAG is not ready. Expected %s got %s", opUp.String(), gnmi.Get(t, tc.dut, gnmi.OC().Interface(tc.aggID).OperStatus().State()).String())
	}
	t.Logf("Waiting for LAG group on OTG to be up")
	_, ok = gnmi.Watch(t, tc.ate.OTG(), gnmi.OTG().Lag("LAG").OperStatus().State(), time.Minute, func(val *ygnmi.Value[otgtelemetry.E_Lag_OperStatus]) bool {
		status, present := val.Val()
		return present && status.String() == "UP"
	}).Await(t)
	if !ok {
		otgutils.LogLAGMetrics(t, tc.ate.OTG(), tc.top)
		t.Fatalf("OTG LAG is not ready. Expected UP got %s", gnmi.Get(t, tc.ate.OTG(), gnmi.OTG().Lag("LAG").OperStatus().State()).String())
	}
	if tc.lagType == oc.IfAggregate_AggregationType_LACP {
		t.Logf("Waiting LAG DUT ports to start collecting and distributing")
		for _, dp := range tc.dutPorts[1:] {
			_, ok := gnmi.WatchAll(t, tc.dut, gnmi.OC().Lacp().InterfaceAny().Member(dp.Name()).Collecting().State(), time.Minute, func(val *ygnmi.Value[bool]) bool {
				col, present := val.Val()
				return present && col
			}).Await(t)
			if !ok {
				t.Fatalf("DUT LAG port %v is not collecting", dp)
			}
			_, ok = gnmi.WatchAll(t, tc.dut, gnmi.OC().Lacp().InterfaceAny().Member(dp.Name()).Distributing().State(), time.Minute, func(val *ygnmi.Value[bool]) bool {
				dist, present := val.Val()
				return present && dist
			}).Await(t)
			if !ok {
				t.Fatalf("DUT LAG port %v is not distributing", dp)
			}
		}
		t.Logf("Waiting LAG OTG ports to start collecting and distributing")
		for _, p := range tc.atePorts[1:] {
			_, ok := gnmi.Watch(t, tc.ate.OTG(), gnmi.OTG().Lacp().LagMember(p.ID()).Collecting().State(), time.Minute, func(val *ygnmi.Value[bool]) bool {
				col, present := val.Val()
				t.Logf("collecting for port %v is %v and present is %v", p.ID(), col, present)
				return present && col
			}).Await(t)
			if !ok {
				t.Fatalf("OTG LAG port %v is not collecting", p)
			}
			_, ok = gnmi.Watch(t, tc.ate.OTG(), gnmi.OTG().Lacp().LagMember(p.ID()).Distributing().State(), time.Minute, func(val *ygnmi.Value[bool]) bool {
				dist, present := val.Val()
				t.Logf("distributing for port %v is %v and present is %v", p.ID(), dist, present)
				return present && dist
			}).Await(t)
			if !ok {
				t.Fatalf("OTG LAG port %v is not distributing", p)
			}
		}
		otgutils.LogLACPMetrics(t, tc.ate.OTG(), tc.top)
	}
	otgutils.LogLAGMetrics(t, tc.ate.OTG(), tc.top)

}

func (tc *testCase) getCounters(t *testing.T, when string) map[string]*oc.Interface_Counters {
	results := make(map[string]*oc.Interface_Counters)
	b := &strings.Builder{}
	w := tabwriter.NewWriter(b, 0, 0, 1, ' ', 0)

	fmt.Fprint(w, "Raw Interface Counters\n\n")
	fmt.Fprint(w, "Name\tInUnicastPkts\tInOctets\tOutUnicastPkts\tOutOctets\n")
	for _, port := range tc.dutPorts[1:] {
		counters := gnmi.Get(t, tc.dut, gnmi.OC().Interface(port.Name()).Counters().State())
		results[port.Name()] = counters
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\n",
			port.Name(),
			counters.GetInUnicastPkts(), counters.GetInOctets(),
			counters.GetOutUnicastPkts(), counters.GetOutOctets())
	}
	w.Flush()

	t.Log(b)

	return results
}

// incrementMAC uses a mac string and increments it by the given i
func incrementMAC(mac string, i int) (string, error) {
	macAddr, err := net.ParseMAC(mac)
	if err != nil {
		return "", err
	}
	convMac := binary.BigEndian.Uint64(append([]byte{0, 0}, macAddr...))
	convMac = convMac + uint64(i)
	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, convMac)
	if err != nil {
		return "", err
	}
	newMac := net.HardwareAddr(buf.Bytes()[2:8])
	return newMac.String(), nil
}
func (tc *testCase) testFlow(t *testing.T, l3header string, config gosnappi.Config) {
	i1 := ateSrc.Name
	i2 := ateDst.Name

	tc.top.Flows().Clear().Items()
	flow := tc.top.Flows().Add().SetName(l3header)
	flow.Metrics().SetEnable(true)
	flow.Size().SetFixed(128)
	flow.Packet().Add().Ethernet().Src().SetValue(ateSrc.MAC)

	if l3header == "ipv4" {
		flow.TxRx().Device().SetTxNames([]string{i1 + ".IPv4"}).SetRxNames([]string{i2 + ".IPv4"})
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(ateSrc.IPv4)
		v4.Dst().SetValue(ateDst.IPv4)
	}
	if l3header == "ipv6" {
		flow.TxRx().Device().SetTxNames([]string{i1 + ".IPv6"}).SetRxNames([]string{i2 + ".IPv6"})
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(ateSrc.IPv6)
		v6.Dst().SetValue(ateDst.IPv6)
	}

	tcp := flow.Packet().Add().Tcp()
	tcp.SrcPort().SetValues(generateRandomPortList(1000))
	tcp.DstPort().SetValues(generateRandomPortList(1000))
	tc.ate.OTG().PushConfig(t, tc.top)
	tc.ate.OTG().StartProtocols(t)

	tc.verifyLAG(t)

	beforeTrafficCounters := tc.getCounters(t, "before")

	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	tc.ate.OTG().SetControlState(t, cs)
	tc.ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	tc.ate.OTG().StopTraffic(t)

	otgutils.LogPortMetrics(t, tc.ate.OTG(), tc.top)
	otgutils.LogFlowMetrics(t, tc.ate.OTG(), tc.top)
	otgutils.LogLAGMetrics(t, tc.ate.OTG(), tc.top)
	ap := tc.ate.Port(t, "port1")
	t.Log("get sent packets from port1 Traffic statistics")
	aic1 := gnmi.OTG().Port(ap.ID()).Counters()
	sentPkts := gnmi.Get(t, tc.ate.OTG(), aic1.OutFrames().State())
	if sentPkts == 0 {
		t.Errorf("Flow sent packets: got %v, want non zero", sentPkts)
	}
	afterTrafficCounters := tc.getCounters(t, "after")
	tc.verifyCounterDiff(t, beforeTrafficCounters, afterTrafficCounters)
	// recieved stats
	rxPkts1 := tc.ate.Port(t, "port2")
	t.Log("get recieved packets from port2 Traffic statistics")
	aic2 := gnmi.OTG().Port(rxPkts1.ID()).Counters()
	recievedPackets1 := gnmi.Get(t, tc.ate.OTG(), aic2.InFrames().State())
	fmt.Println(rxPkts1.ID())
	fmt.Println(recievedPackets1)

	rxPkts2 := tc.ate.Port(t, "port3")
	t.Log("get recieved packets from port3 Traffic statistics")
	aic3 := gnmi.OTG().Port(rxPkts2.ID()).Counters()
	recievedPackets2 := gnmi.Get(t, tc.ate.OTG(), aic3.InFrames().State())
	fmt.Println(recievedPackets2)

	bytes := tc.ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(config.Ports().Items()[1].Name()))
	//fmt.Println(bytes)
	f, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := f.Write(bytes); err != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	if l3header == "ipv4" {
		validatePackets(t, f.Name())
	}
	if l3header == "ipv6" {
		validateV6Packets(t, f.Name())
	}
}

var approxOpt = cmpopts.EquateApprox(0 /* frac */, 0.05 /* absolute */)

func (tc *testCase) verifyCounterDiff(t *testing.T, before, after map[string]*oc.Interface_Counters) {
	b := &strings.Builder{}
	w := tabwriter.NewWriter(b, 0, 0, 1, ' ', 0)

	fmt.Fprint(w, "Interface Counter Deltas\n\n")
	fmt.Fprint(w, "Name\tInPkts\tInOctets\tOutPkts\tOutOctets\n")
	allInPkts := []uint64{}
	allOutPkts := []uint64{}

	for port := range before {
		inPkts := after[port].GetInUnicastPkts() - before[port].GetInUnicastPkts()
		allInPkts = append(allInPkts, inPkts)
		inOctets := after[port].GetInOctets() - before[port].GetInOctets()
		outPkts := after[port].GetOutUnicastPkts() - before[port].GetOutUnicastPkts()
		allOutPkts = append(allOutPkts, outPkts)
		outOctets := after[port].GetOutOctets() - before[port].GetOutOctets()

		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\n",
			port,
			inPkts, inOctets,
			outPkts, outOctets)
	}
	got, outSum := normalize(allOutPkts)
	want := tc.portWants()
	t.Logf("outPkts normalized got: %v", got)
	t.Logf("want: %v", want)
	t.Run("Ratio", func(t *testing.T) {
		if diff := cmp.Diff(want, got, approxOpt); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
	})
	t.Run("Loss", func(t *testing.T) {
		if allInPkts[0] > outSum {
			t.Errorf("Traffic flow received %d packets, sent only %d",
				allOutPkts[0], outSum)
		}
	})
	w.Flush()

	t.Log(b)
}

// portWants converts the nextHop wanted weights to per-port wanted
// weights listed in the same order as atePorts.
func (tc *testCase) portWants() []float64 {
	numPorts := len(tc.dutPorts[1:])
	weights := []float64{}
	for i := 0; i < numPorts; i++ {
		weights = append(weights, 1/float64(numPorts))
	}
	return weights
}

// normalize normalizes the input values so that the output values sum
// to 1.0 but reflect the proportions of the input.  For example,
// input [1, 2, 3, 4] is normalized to [0.1, 0.2, 0.3, 0.4].
func normalize(xs []uint64) (ys []float64, sum uint64) {
	for _, x := range xs {
		sum += x
	}
	ys = make([]float64, len(xs))
	for i, x := range xs {
		ys[i] = float64(x) / float64(sum)
	}
	return ys, sum
}

// generates a list of random tcp ports values
func generateRandomPortList(count uint) []uint32 {
	a := make([]uint32, count)
	for index := range a {
		a[index] = uint32(rand.Intn(65536-1) + 1)
	}
	return a
}

func (tc *testCase) configDstAggregateDUT(i *oc.Interface, a *attrs.Attributes) {
	tc.configSrcDUT(i, a)
	i.Type = ieee8023adLag
	g := i.GetOrCreateAggregation()
	g.LagType = tc.lagType
}

func (tc *testCase) configSrcDUT(i *oc.Interface, a *attrs.Attributes) {
	i.Description = ygot.String(a.Desc)
	if deviations.InterfaceEnabled(tc.dut) {
		i.Enabled = ygot.Bool(true)
	}

	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(tc.dut) && !deviations.IPv4MissingEnabled(tc.dut) {
		s4.Enabled = ygot.Bool(true)
	}
	a4 := s4.GetOrCreateAddress(a.IPv4)
	a4.PrefixLength = ygot.Uint8(plen4)

	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(tc.dut) {
		s6.Enabled = ygot.Bool(true)
	}
	s6.GetOrCreateAddress(a.IPv6).PrefixLength = ygot.Uint8(plen6)
}

func (tc *testCase) configDstMemberDUT(i *oc.Interface, p *ondatra.Port) {
	i.Description = ygot.String(p.String())
	i.Type = ethernetCsmacd
	if deviations.InterfaceEnabled(tc.dut) {
		i.Enabled = ygot.Bool(true)
	}

	e := i.GetOrCreateEthernet()
	e.AggregateId = ygot.String(tc.aggID)
}

func (tc *testCase) configureDUT(t *testing.T) {
	t.Logf("dut ports = %v", tc.dutPorts)
	if len(tc.dutPorts) < 2 {
		t.Fatalf("Testbed requires at least 2 ports, got %d", len(tc.dutPorts))
	}
	d := gnmi.OC()
	tc.clearAggregateMembers(t)
	tc.setupAggregateAtomically(t)
	if tc.lagType == lagTypeLACP {
		lacp := &oc.Lacp_Interface{Name: ygot.String(tc.aggID)}
		lacp.LacpMode = oc.Lacp_LacpActivityType_ACTIVE

		lacpPath := d.Lacp().Interface(tc.aggID)
		fptest.LogQuery(t, "LACP", lacpPath.Config(), lacp)
		gnmi.Replace(t, tc.dut, lacpPath.Config(), lacp)
	}
	// TODO - to remove this sleep later
	time.Sleep(5 * time.Second)

	agg := &oc.Interface{Name: ygot.String(tc.aggID)}
	tc.configDstAggregateDUT(agg, &dutDst)
	aggPath := d.Interface(tc.aggID)
	fptest.LogQuery(t, tc.aggID, aggPath.Config(), agg)
	gnmi.Replace(t, tc.dut, aggPath.Config(), agg)

	srcp := tc.dutPorts[0]
	srci := &oc.Interface{Name: ygot.String(srcp.Name())}
	tc.configSrcDUT(srci, &dutSrc)
	srci.Type = ethernetCsmacd
	srciPath := d.Interface(srcp.Name())
	fptest.LogQuery(t, srcp.String(), srciPath.Config(), srci)
	gnmi.Replace(t, tc.dut, srciPath.Config(), srci)
	if deviations.ExplicitInterfaceInDefaultVRF(tc.dut) {
		fptest.AssignToNetworkInstance(t, tc.dut, srcp.Name(), deviations.DefaultNetworkInstance(tc.dut), 0)
		fptest.AssignToNetworkInstance(t, tc.dut, tc.aggID, deviations.DefaultNetworkInstance(tc.dut), 0)
	}
	for _, port := range tc.dutPorts[1:] {
		i := &oc.Interface{Name: ygot.String(port.Name())}
		tc.configDstMemberDUT(i, port)
		iPath := d.Interface(port.Name())
		fptest.LogQuery(t, port.String(), iPath.Config(), i)
		gnmi.Replace(t, tc.dut, iPath.Config(), i)
	}
	if deviations.ExplicitPortSpeed(tc.dut) {
		for _, port := range tc.dutPorts {
			fptest.SetPortSpeed(t, port)
		}
	}
	configureNetworkInstance(t)
	gnmiClient := tc.dut.RawAPIs().GNMI(t)
	var v4Config, v6Config string
	if deviations.TunnelAclEncapsulationConfigUnsupported(tc.dut) {
		switch tc.dut.Vendor() {
		case ondatra.JUNIPER:
			v4Config = juniperEncapCLI("inet", aclIpv4Name, encapSrcMatch, encapDstMatch, countIpv4, tunnelActionIpv4, tunnelEndpoint, "ipv4", greSrcAddr, greDstAddr)
			v6Config = juniperEncapCLI("inet6", aclIpv6Name, encapV6SrcMatch, encapv6DstMatch, countIpv6, tunnelActionIpv6, tunnelV6Endpoint, "ipv6", greV6SrcAddr, greV6DstAddr)
			t.Logf(" Push CLI config of:\n%s", tc.dut.Vendor())
		default:
			t.Errorf("Invalid Filter configuration")
		}
	}
	gpbSetRequest := buildCliConfigRequest(v4Config)
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}

	gpbSetRequestV6 := buildCliConfigRequest(v6Config)
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequestV6); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}

	d1 := &oc.Root{}
	ifName := tc.dut.Port(t, "port1").Name()
	t.Log("Attach the filter to the ingress interface")
	iFace := d1.GetOrCreateAcl().GetOrCreateInterface(ifName)
	aclConf := configACLInterface(t, iFace, ifName)
	gnmi.Replace(t, tc.dut, aclConf.Config(), iFace)
	fptest.LogQuery(t, "ACL config:\n", aclConf.Config(), gnmi.GetConfig(t, tc.dut, aclConf.Config()))

	aclConfV6 := configACLIpv6Interface(t, iFace, ifName)
	gnmi.Replace(t, tc.dut, aclConfV6.Config(), iFace)
	fptest.LogQuery(t, "ACL config:\n", aclConfV6.Config(), gnmi.GetConfig(t, tc.dut, aclConfV6.Config()))
}
func (tc *testCase) verifyDUT(t *testing.T) {
	// Wait for LAG negotiation and verify LAG type for the aggregate interface.
	gnmi.Await(t, tc.dut, gnmi.OC().Interface(tc.aggID).Type().State(), time.Minute, ieee8023adLag)
	for _, port := range tc.dutPorts {
		path := gnmi.OC().Interface(port.Name())
		gnmi.Await(t, tc.dut, path.OperStatus().State(), time.Minute, oc.Interface_OperStatus_UP)
	}
}

func TestTunAclLoadbalance(t *testing.T) {
	start := time.Now()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	aggID := netutil.NextAggregateInterface(t, dut)

	tests := []testCase{
		{
			desc:     "IPV4",
			l3header: "ipv4",
		},

		{
			desc:     "IPV6",
			l3header: "ipv6",
		},
	}
	tc := &testCase{
		dut:     dut,
		ate:     ate,
		lagType: lagTypeLACP,
		top:     gosnappi.NewConfig(),

		dutPorts: sortPorts(dut.Ports()),
		atePorts: sortPorts(ate.Ports()),
		aggID:    aggID,
	}
	config := tc.configureATE(t)
	tc.configureDUT(t)
	t.Run("verifyDUT", tc.verifyDUT)

	for _, tf := range tests {
		t.Run(tf.desc, func(t *testing.T) {
			tc.l3header = tf.l3header
			tc.testFlow(t, tc.l3header, config)
		})
	}

	t.Logf("Time check: %s", time.Since(start))
	t.Logf("Test run time: %s", time.Since(start))
}
