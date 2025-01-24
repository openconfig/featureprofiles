// Copyright 2024 Google LLC
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

package mpls_in_udp_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

const (
	ipv6PrefixLen      = 126
	ipv6FlowIP         = "2015:aa8::1"
	trafficDuration    = 15 * time.Second
	nhg10ID            = 10
	vrfEncapA          = "ENCAP_TE_VRF_A"
	ipv6EntryPrefix    = "2015:aa8::"
	ipv6EntryPrefixLen = 128
	nh201ID            = 201
	nhgName            = "nh-group-1"
	outerIpv6Src       = "2001:f:a:1::0"
	outerIpv6DstA      = "2001:f:c:e::1"
	outerDstUDPPort    = "6635"
	outerDscp          = "26"
	outerIPTTL         = "64"
)

var (
	otgDstPorts = []string{"port2"}
	otgSrcPort  = "port1"
	
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		MAC:     "02:01:00:00:00:01",
		IPv6:    "2001:f:d:e::1",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort1 = attrs.Attributes{
		Name:    "otgPort1",
		MAC:     "02:00:01:01:01:01",
		IPv6:    "2001:f:d:e::2",
		IPv6Len: ipv6PrefixLen,
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		MAC:     "02:01:00:00:00:02",
		IPv6:    "2001:f:d:e::5",
		IPv6Len: ipv6PrefixLen,
	}

	otgPort2 = attrs.Attributes{
		Name:    "otgPort2",
		MAC:     "02:00:02:01:01:01",
		IPv6:    "2001:f:d:e::6",
		IPv6Len: ipv6PrefixLen,
	}

	fa6 = flowAttr{
		src:        otgPort1.IPv6,
		dst:        outerIpv6DstA,
		defaultDst: ipv6FlowIP,
		srcMac:     otgPort1.MAC,
		dstMac:     dutPort1.MAC,
		srcPort:    otgSrcPort,
		dstPorts:   otgDstPorts,
		topo:       gosnappi.NewConfig(),
	}
)

type flowAttr struct {
	src        string   // source IP address
	dst        string   // destination IP address
	defaultDst string   // default destination IP address
	srcPort    string   // source OTG port
	dstPorts   []string // destination OTG ports
	srcMac     string   // source MAC address
	dstMac     string   // destination MAC address
	topo       gosnappi.Config
}


// testArgs holds the objects needed by a test case.
type testArgs struct {
	dut    *ondatra.DUTDevice
	ate    *ondatra.ATEDevice
	topo   gosnappi.Config
	client *gribi.Client
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	portList := []*ondatra.Port{p1, p2}

	// configure interfaces
	for idx, a := range []attrs.Attributes{dutPort1, dutPort2} {
		p := portList[idx]
		intf := a.NewOCInterface(p.Name(), dut)
		if p.PMD() == ondatra.PMD100GBASEFR && dut.Vendor() != ondatra.CISCO && dut.Vendor() != ondatra.JUNIPER {
			e := intf.GetOrCreateEthernet()
			e.AutoNegotiate = ygot.Bool(false)
			e.DuplexMode = oc.Ethernet_DuplexMode_FULL
			e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
		}
		gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), intf)
	}
}

// configureOTG configures port1 on the OTG.
func configureOTG(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	otg := ate.OTG()
	topo := gosnappi.NewConfig()
	t.Logf("Configuring OTG port1")
	p1 := ate.Port(t, "port1")
	p2 := ate.Port(t, "port2")

	otgPort1.AddToOTG(topo, p1, &dutPort1)
	otgPort2.AddToOTG(topo, p2, &dutPort2)

	pmd100GFRPorts := []string{}
	for _, p := range topo.Ports().Items() {
		port := ate.Port(t, p.Name())
		if port.PMD() == ondatra.PMD100GBASEFR {
			pmd100GFRPorts = append(pmd100GFRPorts, port.ID())
		}
	}
	// Disable FEC for 100G-FR ports because Novus does not support it.
	if len(pmd100GFRPorts) > 0 {
		l1Settings := topo.Layer1().Add().SetName("L1").SetPortNames(pmd100GFRPorts)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, topo)
	t.Logf("starting protocols...")
	otg.StartProtocols(t)
	time.Sleep(50 * time.Second)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")
	return topo
}

// getFlow returns a flow of ipv6.
func (fa *flowAttr) getFlow(flowType string, name string) gosnappi.Flow {
	flow := fa.topo.Flows().Add().SetName(name)
	flow.Metrics().SetEnable(true)

	flow.TxRx().Port().SetTxName(fa.srcPort).SetRxNames(fa.dstPorts)
	e1 := flow.Packet().Add().Ethernet()
	e1.Src().SetValue(fa.srcMac)
	e1.Dst().SetValue(fa.dstMac)
	if flowType == "ipv6" {
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(fa.src)
		switch name {
		case "ip6a1":
			v6.Dst().SetValue(fa.dst)
		case "ip6a2":
			v6.Dst().SetValue(fa.defaultDst)
		default:
			v6.Dst().SetValue(fa.dst)
		}
	}
	return flow
}

// programEntries pushes RIB entries on the DUT required for Encap functionality
func programEntries(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client) {
	t.Log("Programming RIB entries")
	// TODO: vvardhanreddy revisit when functionality is added.
	// nh7, op9 := gribi.NHEntry(nh201ID, "EncapUDP", vrfEncapA, fluent.InstalledInFIB,
	//	&gribi.NHOptions{Src: outerIpv6Src, Dest: outerIpv6DstA, VrfName: vrfEncapA})
	// nhg4, op11 := gribi.NHGEntry(nhg10ID, map[uint64]uint64{nh201ID: 1},
	//	vrfEncapA, fluent.InstalledInFIB)
	// c.AddEntries(t, []fluent.GRIBIEntry{nh7, nhg4}, []*client.OpResult{op9, op11})
	// c.AddIPv6(t, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), nhg10ID, vrfEncapA, deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
}

// configDefaultRoute configures a static route in DEFAULT network-instance.
func configDefaultRoute(t *testing.T, dut *ondatra.DUTDevice, v6Prefix, v6NextHop string) {
	t.Logf("TE-18.1.2: Configuring static route in DEFAULT network-instance")
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(v6Prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(v6NextHop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// cidr takes as input the IP address and the Mask and returns the IP string in
// CIDR notation.
func cidr(ipaddrs string, ones int) string {
	return ipaddrs + "/" + strconv.Itoa(ones)
}

// configureEncapHeadersCli is only used if a DUT does not support gRIBI.
func configureEncapHeaderCli(t *testing.T, dut *ondatra.DUTDevice) {
	switch dut.Vendor() {
	case ondatra.ARISTA:
		var encapHeaderCLI string
		encapHeaderCLI = fmt.Sprintf("tunnel type mpls-over-udp udp destination port %s\n", outerDstUDPPort)
		helpers.GnmiCLIConfig(t, dut, encapHeaderCLI)
		encapHeaderCLI = fmt.Sprintf(" nexthop-group %s type mpls-over-udp\n", nhgName)
		helpers.GnmiCLIConfig(t, dut, encapHeaderCLI)
		encapHeaderCLI = fmt.Sprintf(" tos %s\n", outerDscp)
		helpers.GnmiCLIConfig(t, dut, encapHeaderCLI)
		encapHeaderCLI = fmt.Sprintf(" ttl %s\n", outerIPTTL)
		helpers.GnmiCLIConfig(t, dut, encapHeaderCLI)
		encapHeaderCLI = " fec hierarchical"
		helpers.GnmiCLIConfig(t, dut, encapHeaderCLI)
		encapHeaderCLI = fmt.Sprintf(" tunnel-source %s\n", outerIpv6Src)
		helpers.GnmiCLIConfig(t, dut, encapHeaderCLI)
		encapHeaderCLI = fmt.Sprintf(" entry 0 push label-stack 899999 tunnel-destination %s\n", outerIpv6DstA)
		helpers.GnmiCLIConfig(t, dut, encapHeaderCLI)
		encapHeaderCLI = fmt.Sprintf(" ip route vrf customer %s nexthop-group nhg%d\n", outerIpv6DstA, nhg10ID)
		helpers.GnmiCLIConfig(t, dut, encapHeaderCLI)
	default:
		t.Logf("Unsupported vendor %s for native command support for deviation 'GribiEncapHeaderUnsupported'", dut.Vendor())
	}
}

func TestMPLSOUDPEncap(t *testing.T) {
	// Configure DUT
	dut := ondatra.DUT(t, "dut")
	configureDUT(t, dut)

	// Configure ATE
	otg := ondatra.ATE(t, "ate")
	topo := configureOTG(t, otg)

	// configure gRIBI client
	c := gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}

	if err := c.Start(t); err != nil {
		t.Fatalf("gRIBI Connection can not be established")
	}

	defer c.Close(t)
	c.BecomeLeader(t)
	// Flush all existing AFT entries on the router
	c.FlushAll(t)
	if deviations.GribiEncapHeaderUnsupported(dut) {
		configureEncapHeaderCli(t, dut)
	} else {
		programEntries(t, dut, &c)
	}
	configDefaultRoute(t, dut, cidr(ipv6EntryPrefix, ipv6EntryPrefixLen), otgPort1.IPv6)
	test := []struct {
		name         string
		flows        []gosnappi.Flow
		capturePorts []string
	}{
		{
			name:         "TE-18.1.1 Match and Encapsulate using gRIBI aft modify",
			flows:        []gosnappi.Flow{fa6.getFlow("ipv6", "ip6a1")},
			capturePorts: otgDstPorts,
		},
		{
			name:         "TE-18.1.2 Validate prefix match rule for MPLS in GRE encap using default route",
			flows:        []gosnappi.Flow{fa6.getFlow("ipv6", "ip6a2")},
			capturePorts: otgDstPorts,
		},
	}

	tcArgs := &testArgs{
		client: &c,
		dut:    dut,
		ate:    otg,
		topo:   topo,
	}

	for _, tc := range test {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Name: %s", tc.name)
			enableCapture(t, otg.OTG(), topo, tc.capturePorts)
			t.Log("Start capture and send traffic")
			sendTraffic(t, tcArgs, tc.flows, true)
			t.Log("Validate captured packet attributes")
			// TODO: b/364961777 upstream GUE decoder to gopacket addition is pending.
			// err := validatePacketCapture(t, tcArgs, tc.capturePorts)
			clearCapture(t, otg.OTG(), topo)
			// if err != nil {
			//	t.Fatalf("Failed to validate ATE port 2 receives MPLS-IN-UDP packets: %v", err)
			// }
		})
	}
}

// clearCapture clears capture from all ports on the OTG
func clearCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config) {
	t.Log("Clearing capture")
	topo.Captures().Clear()
	otg.PushConfig(t, topo)
}

// sendTraffic starts traffic flows and send traffic for a fixed duration
func sendTraffic(t *testing.T, args *testArgs, flows []gosnappi.Flow, capture bool) {
	otg := args.ate.OTG()
	args.topo.Flows().Clear().Items()
	args.topo.Flows().Append(flows...)

	otg.PushConfig(t, args.topo)
	otg.StartProtocols(t)

	otgutils.WaitForARP(t, args.ate.OTG(), args.topo, "IPv4")
	otgutils.WaitForARP(t, args.ate.OTG(), args.topo, "IPv6")

	if capture {
		startCapture(t, args.ate)
		defer stopCapture(t, args.ate)
	}
	t.Log("Starting traffic")
	otg.StartTraffic(t)
	time.Sleep(trafficDuration)
	otg.StopTraffic(t)
	t.Log("Traffic stopped")
}

// startCapture starts the capture on the otg ports
func startCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
}

// enableCapture enables packet capture on specified list of ports on OTG
func enableCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config, otgPortNames []string) {
	for _, port := range otgPortNames {
		t.Log("Enabling capture on ", port)
		topo.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}
	pb, _ := topo.Marshal().ToProto()
	t.Log(pb.GetCaptures())
	otg.PushConfig(t, topo)
}

// stopCapture starts the capture on the otg ports
func stopCapture(t *testing.T, ate *ondatra.ATEDevice) {
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}
