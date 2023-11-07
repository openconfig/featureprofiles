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

package tun_acl_dscp_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/gnmi/oc/acl"
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
//
//	* ate:port1 -> dut:port1 subnet 192.0.2.2/30
//	* ate:port2 -> dut:port2 subnet 192.0.2.6/30
//
//	* Traffic is sent from 192.0.2.2(ate:port1) to 192.0.2.6(ate:port2).
//	  Dut Interfaces are configured as mentioned below:
//	  dut:port1 -> 192.0.2.1
//	  dut:port2 -> 192.0.2.5
//	  Verify the DSCP value in the packet capture.
//	  TOS Caluculation:
//	  	    For example, DSCP Value configured with 8 (001000).
//	  		In the Capture as the TOS value will have 8 bits(2 appended at LSB) corresponding binary will be 001000 00(dscp + 00)
//          which is equivalet to 32 in decimal.

const (
	ipv4PrefixLen     = 30
	pps               = 100
	FrameSize         = 512
	aclName           = "f1"
	termName          = "t1"
	EncapSrcMatch     = "192.0.2.2"
	EncapDstMatch     = "192.0.2.6"
	count             = "GreFilterCount"
	greTunnelEndpoint = "TunnelEncapIpv4"
	greSrcAddr        = "198.51.100.1"
	greDstAddr        = "203.0.113.1/32"
	dscp              = 8
	CorrespondingTOS  = 32
	GreProtocol       = 47
	Tunnelaction      = "TunnelEncapIpv4"
	plenIPv4          = 30
	tolerance         = 50
	lossTolerance     = 2
	prefix            = "0.0.0.0/0"
	nexthop           = "192.0.2.6"
)

var (
	dutSrc = attrs.Attributes{
		Desc:    "DUT to ATE source",
		IPv4:    "192.0.2.1",
		IPv4Len: plenIPv4,
	}
	ateSrc = attrs.Attributes{
		Name:    "ateSrc",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: plenIPv4,
	}
	dutDst = attrs.Attributes{
		Desc:    "DUT to ATE destination",
		IPv4:    "192.0.2.5",
		IPv4Len: plenIPv4,
	}
	ateDst = attrs.Attributes{
		Name:    "atedst",
		MAC:     "02:00:02:01:01:01",
		IPv4:    "192.0.2.6",
		IPv4Len: plenIPv4,
	}
)

// configInterfaceDUT configures the DUT interfaces.
func configInterfaceDUT(i *oc.Interface, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i.Description = ygot.String(a.Desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s4a := s4.GetOrCreateAddress(a.IPv4)
	s4a.PrefixLength = ygot.Uint8(ipv4PrefixLen)
	return i
}

// configureDUT configures port1 and port2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	i1 := &oc.Interface{Name: ygot.String(p1.Name())}
	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), configInterfaceDUT(i1, &dutSrc, dut))
	p2 := dut.Port(t, "port2")
	i2 := &oc.Interface{Name: ygot.String(p2.Name())}
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(i2, &dutDst, dut))
	configureNetworkInstance(t)
	t.Logf("Configure the DUT with static route ...")
	configStaticRoute(t, dut, prefix, nexthop)
	gnmiClient := dut.RawAPIs().GNMI(t)
	var config string
	t.Logf("Push the CLI config:\n%s", dut.Vendor())
	switch dut.Vendor() {
	case ondatra.JUNIPER:
		config = juniperEncapCLI(aclName, EncapSrcMatch, EncapDstMatch, count, greSrcAddr, greDstAddr)
		t.Logf(" Push CLI config of:\n%s", dut.Vendor())
	default:
		t.Errorf("Invalid Filter configuration")
	}
	gpbSetRequest := buildCliConfigRequest(config)
	t.Log("gnmiClient Set CLI config")
	if _, err := gnmiClient.Set(context.Background(), gpbSetRequest); err != nil {
		t.Fatalf("gnmiClient.Set() with unexpected error: %v", err)
	}
	d1 := &oc.Root{}
	ifName := dut.Port(t, "port1").Name()
	t.Log("Attach the filter to the ingress interface")
	iFace := d1.GetOrCreateAcl().GetOrCreateInterface(ifName)
	aclConf := configACLInterface(t, iFace, ifName)
	gnmi.Replace(t, dut, aclConf.Config(), iFace)
	fptest.LogQuery(t, "ACL config:\n", aclConf.Config(), gnmi.GetConfig(t, dut, aclConf.Config()))
}

// configACLInterface configures the ACL attachment on interface
func configACLInterface(t *testing.T, iFace *oc.Acl_Interface, ifName string) *acl.Acl_InterfacePath {
	aclConf := gnmi.OC().Acl().Interface(ifName)
	if ifName != "" {
		iFace.GetOrCreateIngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
		iFace.GetOrCreateInterfaceRef().Interface = ygot.String(ifName)
		iFace.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	} else {
		iFace.GetOrCreateIngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
		iFace.DeleteIngressAclSet(aclName, oc.Acl_ACL_TYPE_ACL_IPV4)
	}
	return aclConf
}

// configureNetworkInstance Configure default network instance
func configureNetworkInstance(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut))
	gnmi.Replace(t, dut, dutConfPath.Type().Config(), oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_DEFAULT_INSTANCE)
}

// configStaticRoute configures a static route.
func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string) {
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(nexthop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// configureOTG configures the traffic interfaces
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	topo := gosnappi.NewConfig()
	t.Logf("Configuring OTG port1")
	srcPort := topo.Ports().Add().SetName("port1")
	srcDev := topo.Devices().Add().SetName(ateSrc.Name)
	srcEth := srcDev.Ethernets().Add().SetName(ateSrc.Name + ".Eth").SetMac(ateSrc.MAC)
	srcEth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(srcPort.Name())
	srcIpv4 := srcEth.Ipv4Addresses().Add().SetName(ateSrc.Name + ".IPv4")
	srcIpv4.SetAddress(ateSrc.IPv4).SetGateway(dutSrc.IPv4).SetPrefix(uint32(ateSrc.IPv4Len))
	t.Logf("Configuring OTG port2")
	dstPort := topo.Ports().Add().SetName("port2")
	dstDev := topo.Devices().Add().SetName(ateDst.Name)
	dstEth := dstDev.Ethernets().Add().SetName(ateDst.Name + ".Eth").SetMac(ateDst.MAC)
	dstEth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(dstPort.Name())
	dstIpv4 := dstEth.Ipv4Addresses().Add().SetName(ateDst.Name + ".IPv4")
	dstIpv4.SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
	topo.Captures().Add().SetName("grecapture").SetPortNames([]string{dstPort.Name()}).SetFormat(gosnappi.CaptureFormat.PCAP)
	t.Logf("Testtraffic:start ate Traffic config")
	flowipv4 := topo.Flows().Add().SetName("IPv4")
	flowipv4.Metrics().SetEnable(true)
	flowipv4.TxRx().Device().
		SetTxNames([]string{srcIpv4.Name()}).
		SetRxNames([]string{dstIpv4.Name()})
	flowipv4.Size().SetFixed(FrameSize)
	flowipv4.Rate().SetPps(pps)
	flowipv4.Duration().SetChoice("continuous")
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(srcEth.Mac())
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(srcIpv4.Address())
	v4.Dst().SetValue(dstIpv4.Address())
	v4.Priority().Dscp().Phb().SetValue(uint32(dscp))
	t.Logf("Pushing config to ATE and starting protocols...")
	ate.OTG().PushConfig(t, topo)
	t.Logf("starting protocols...")
	ate.OTG().StartProtocols(t)
	time.Sleep(30 * time.Second)
	//	otgutils.WaitForARP(t, otg, topo, "IPv4")
	t.Log(topo.Msg().GetCaptures())
	return topo
}

// sendTraffic will send the traffic for a fixed duration
func sendTraffic(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
	t.Log("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	otg.StopTraffic(t)
	t.Log("Traffic stopped")
}

// captureTrafficStats Captures traffic statistics and verifies for the loss
func captureTrafficStats(t *testing.T, ate *ondatra.ATEDevice, config gosnappi.Config) {
	otg := ate.OTG()
	ap := ate.Port(t, "port1")
	t.Log("get sent packets from port1 Traffic statistics")
	aic1 := gnmi.OTG().Port(ap.ID()).Counters()
	sentPkts := gnmi.Get(t, otg, aic1.OutFrames().State())
	fptest.LogQuery(t, "ate:port1 counters", aic1.State(), gnmi.Get(t, otg, aic1.State()))
	op := ate.Port(t, "port2")
	aic2 := gnmi.OTG().Port(op.ID()).Counters()
	t.Log("get recieved packets from port2 Traffic statistics")
	rxPkts := gnmi.Get(t, otg, aic2.InFrames().State())
	fptest.LogQuery(t, "ate:port2 counters", aic2.State(), gnmi.Get(t, otg, aic2.State()))
	var lostPkts uint64
	t.Log("Verify Traffic statistics")
	if rxPkts > sentPkts {
		lostPkts = rxPkts - sentPkts
	} else {
		lostPkts = sentPkts - rxPkts
	}
	t.Logf("Packets: %d sent, %d received, %d lost", sentPkts, rxPkts, lostPkts)
	if lostPkts > tolerance {
		t.Errorf("Lost Packets are more than tolerance: %d", lostPkts)
	} else {
		t.Log("Traffic Test Passed!")
	}
	bytes := otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(config.Ports().Items()[1].Name()))
	f, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := f.Write(bytes); err != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	f.Close()
	ValidatePackets(t, f.Name())
}
func ValidatePackets(t *testing.T, filename string) {
	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		gre := packet.Layer(layers.LayerTypeGRE).(*layers.GRE)
		if ipLayer == nil {
			t.Errorf("IpLayer is null: %d", ipLayer)
		}
		ipPacket, _ := ipLayer.(*layers.IPv4)
		if ipPacket.Protocol != GreProtocol {
			t.Errorf("Packet is not encapslated properly. Encapsulated protocol is: %d", ipPacket.Protocol)
		}
		InnerPacket := gopacket.NewPacket(gre.Payload, gre.NextLayerType(), gopacket.Default)
		IpInnerLayer := InnerPacket.Layer(layers.LayerTypeIPv4)
		if IpInnerLayer != nil {
			IpInnerPacket, _ := IpInnerLayer.(*layers.IPv4)
			if IpInnerPacket.TOS != CorrespondingTOS {
				t.Logf("IP TOS bit: %d", IpInnerPacket.TOS)
				t.Errorf("DSCP(TOS) value is altered to: %d", IpInnerPacket.TOS)
			}
		}
	}
	t.Log("get dscp of inner header and verify whether orginal value is retained.")
}

func juniperEncapCLI(aclName string, EncapSrcMatch string, EncapDstMatch string, count string, greSrcAddr string, greDstAddr string) string {
	return fmt.Sprintf(`
	firewall {
		family inet {
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
					then encapsulate greTunnelEndpoint
				}
			}
		}
	    tunnel-end-point greTunnelEndpoint {
			ipv4 {
				source-address %s;
				destination-address %s;
			}
			gre;
		}
	}
  `, aclName, EncapSrcMatch, EncapDstMatch, count, greSrcAddr, greDstAddr)
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

func TestDscpInGreEncapPacket(t *testing.T) {
	start := time.Now()
	dut := ondatra.DUT(t, "dut")
	t.Logf("Configure DUT")
	configureDUT(t, dut)
	t.Logf("Configure OTG")
	otg := ondatra.ATE(t, "ate")
	config := configureOTG(t, otg)
	t.Log("send Traffic statistics")
	sendTraffic(t, otg)
	captureTrafficStats(t, otg, config)
	t.Logf("Time check: %s", time.Since(start))
	t.Logf("Test run time: %s", time.Since(start))
}
