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

package single_decap_giribi_entry_test

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
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1,
// dut:port2 -> ate:port2.
//
//   * ate:port1 -> dut:port1 subnet 192.0.2.1/30
//   * ate:port2 -> dut:port2 subnet 192.0.2.5/30

const (
	plenIPv4              = 30
	plenIPv6              = 126
	dscpEncapA1           = 10
	dscpEncapA2           = 18
	dscpEncapB1           = 20
	dscpEncapB2           = 28
	ipv4OuterSrc111Addr   = "198.51.100.111/32"
	ipv4OuterSrc222Addr   = "198.51.100.222/32"
	ipv4OuterDst111       = "192.51.100.64"
	ipv4OuterSrc111       = "198.51.100.111"
	ipv4OuterDst333       = "203.0.113.1"
	prot4                 = 4
	prot41                = 41
	polName               = "pol1"
	nhIndex               = 1
	nhgIndex              = 1
	niDecapTeVrf          = "DECAP_TE_VRF"
	tolerancePct          = 2
	tolerance             = 50
	flow6in4              = "flow6in4"
	flow4in4              = "flow4in4"
	flowNegTest           = "flowNegTest"
	wantLoss              = true
	correspondingTTL      = 64
	correspondingHopLimit = 64
	checkTTL              = true
	checkDecap            = true
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv6:    "2001:db8::192:0:2:1",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		MAC:     "02:00:01:01:01:01",
		IPv6:    "2001:db8::192:0:2:2",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv6:    "2001:db8::192:0:2:5",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		MAC:     "02:00:02:01:01:01",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
)

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, t testing.TB, c *fluent.GRIBIClient, timeout time.Duration) error {
	t.Helper()
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

type testArgs struct {
	ctx        context.Context
	client     *fluent.GRIBIClient
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	otgConfig  gosnappi.Config
	top        gosnappi.Config
	electionID gribi.Uint128
	otg        *otg.OTG
}

type policyFwRule struct {
	SeqId           uint32
	protocol        oc.UnionUint8
	dscpSet         []uint8
	sourceAddr      string
	decapNi         string
	postDecapNi     string
	decapFallbackNi string
}

func configureVrfSelectionPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	dutPolFwdPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()

	pfRule1 := &policyFwRule{SeqId: 1, protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222Addr,
		decapNi: "DECAP_TE_VRF", postDecapNi: "ENCAP_TE_VRF_A", decapFallbackNi: "TE_VRF_222"}
	pfRule2 := &policyFwRule{SeqId: 2, protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222Addr,
		decapNi: "DECAP_TE_VRF", postDecapNi: "ENCAP_TE_VRF_A", decapFallbackNi: "TE_VRF_222"}
	pfRule3 := &policyFwRule{SeqId: 3, protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111Addr,
		decapNi: "DECAP_TE_VRF", postDecapNi: "ENCAP_TE_VRF_A", decapFallbackNi: "TE_VRF_111"}
	pfRule4 := &policyFwRule{SeqId: 4, protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111Addr,
		decapNi: "DECAP_TE_VRF", postDecapNi: "ENCAP_TE_VRF_A", decapFallbackNi: "TE_VRF_111"}

	pfRule5 := &policyFwRule{SeqId: 5, protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222Addr,
		decapNi: "DECAP_TE_VRF", postDecapNi: "ENCAP_TE_VRF_B", decapFallbackNi: "TE_VRF_222"}
	pfRule6 := &policyFwRule{SeqId: 6, protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222Addr,
		decapNi: "DECAP_TE_VRF", postDecapNi: "ENCAP_TE_VRF_B", decapFallbackNi: "TE_VRF_222"}
	pfRule7 := &policyFwRule{SeqId: 7, protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111Addr,
		decapNi: "DECAP_TE_VRF", postDecapNi: "ENCAP_TE_VRF_B", decapFallbackNi: "TE_VRF_111"}
	pfRule8 := &policyFwRule{SeqId: 8, protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111Addr,
		decapNi: "DECAP_TE_VRF", postDecapNi: "ENCAP_TE_VRF_B", decapFallbackNi: "TE_VRF_111"}

	pfRule9 := &policyFwRule{SeqId: 9, protocol: 4, sourceAddr: ipv4OuterSrc222Addr,
		decapNi: "DECAP_TE_VRF", postDecapNi: "DEFAULT", decapFallbackNi: "TE_VRF_222"}
	pfRule10 := &policyFwRule{SeqId: 10, protocol: 41, sourceAddr: ipv4OuterSrc222Addr,
		decapNi: "DECAP_TE_VRF", postDecapNi: "DEFAULT", decapFallbackNi: "TE_VRF_222"}
	pfRule11 := &policyFwRule{SeqId: 11, protocol: 4, sourceAddr: ipv4OuterSrc111Addr,
		decapNi: "DECAP_TE_VRF", postDecapNi: "DEFAULT", decapFallbackNi: "TE_VRF_111"}
	pfRule12 := &policyFwRule{SeqId: 12, protocol: 41, sourceAddr: ipv4OuterSrc111Addr,
		decapNi: "DECAP_TE_VRF", postDecapNi: "DEFAULT", decapFallbackNi: "TE_VRF_111"}

	pfRuleList := []*policyFwRule{pfRule1, pfRule2, pfRule3, pfRule4, pfRule5, pfRule6,
		pfRule7, pfRule8, pfRule9, pfRule10, pfRule11, pfRule12}

	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niP := ni.GetOrCreatePolicyForwarding()
	niPf := niP.GetOrCreatePolicy(polName)
	niPf.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pfRule := range pfRuleList {
		pfR := niPf.GetOrCreateRule(pfRule.SeqId)
		pfRProtoIPv4 := pfR.GetOrCreateIpv4()
		pfRProtoIPv4.Protocol = oc.UnionUint8(pfRule.protocol)
		if pfRule.dscpSet != nil {
			pfRProtoIPv4.DscpSet = pfRule.dscpSet
		}
		pfRProtoIPv4.SourceAddress = ygot.String(pfRule.sourceAddr)
		pfRAction := pfR.GetOrCreateAction()
		pfRAction.DecapNetworkInstance = ygot.String(pfRule.decapNi)
		pfRAction.PostDecapNetworkInstance = ygot.String(pfRule.postDecapNi)
		pfRAction.DecapFallbackNetworkInstance = ygot.String(pfRule.decapFallbackNi)
	}
	pfR := niPf.GetOrCreateRule(13)
	pfRAction := pfR.GetOrCreateAction()
	pfRAction.NetworkInstance = ygot.String("DEFAULT")

	p1 := dut.Port(t, "port1")
	intf := niP.GetOrCreateInterface(p1.Name())
	intf.ApplyVrfSelectionPolicy = ygot.String(polName)
	intf.GetOrCreateInterfaceRef().Interface = ygot.String(p1.Name())
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}
	gnmi.Replace(t, dut, dutPolFwdPath.Config(), niP)
}

// configureNetworkInstance configures vrfs DECAP_TE_VRF,ENCAP_TE_VRF_A,ENCAP_TE_VRF_B,
// TE_VRF_222, TE_VRF_111.
func configNonDefaultNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	c := &oc.Root{}
	vrfs := []string{"DECAP_TE_VRF", "ENCAP_TE_VRF_A", "ENCAP_TE_VRF_B", "TE_VRF_222", "TE_VRF_111"}
	for _, vrf := range vrfs {
		ni := c.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}
}

// configureDUT configures port1-2 on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()

	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")

	gnmi.Replace(t, dut, d.Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), dutPort2.NewOCInterface(p2.Name(), dut))

	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, dut.Port(t, "port1"))
		fptest.SetPortSpeed(t, dut.Port(t, "port2"))
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p2.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
}

func configureGribiRoute(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs, prefWithMask string) {
	t.Helper()
	// Using gRIBI, install an  IPv4Entry for the prefix 192.51.100.1/24 that points to a
	// NextHopGroup that contains a single NextHop that specifies decapsulating the IPv4
	// header and specifies the DEFAULT network instance.This IPv4Entry should be installed
	// into the DECAP_TE_VRF.

	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(nhIndex).WithDecapsulateHeader(fluent.IPinIP).
			WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(nhgIndex).AddNextHop(nhIndex, 1),
		fluent.IPv4Entry().WithNetworkInstance("DECAP_TE_VRF").
			WithPrefix(prefWithMask).WithNextHopGroup(nhgIndex),
	)
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(2).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(2).AddNextHop(2, 1),
		fluent.IPv4Entry().WithNetworkInstance("TE_VRF_111").
			WithPrefix("0.0.0.0/0").WithNextHopGroup(2),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().WithNextHopOperation(nhIndex).WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
		chk.IgnoreOperationID(),
	)
	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().WithNextHopGroupOperation(nhIndex).WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
		chk.IgnoreOperationID(),
	)
	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().WithIPv4Operation(prefWithMask).WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
		chk.IgnoreOperationID(),
	)
	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().WithIPv4Operation("0.0.0.0/0").WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
		chk.IgnoreOperationID(),
	)
}

func configureOTG(t *testing.T, otg *otg.OTG) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	port1 := config.Ports().Add().SetName("port1")
	port2 := config.Ports().Add().SetName("port2")

	iDut1Dev := config.Devices().Add().SetName(atePort1.Name)
	iDut1Eth := iDut1Dev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	iDut1Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port1.Name())
	iDut1Ipv4 := iDut1Eth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4")
	iDut1Ipv4.SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))
	iDut1Ipv6 := iDut1Eth.Ipv6Addresses().Add().SetName(atePort1.Name + ".IPv6")
	iDut1Ipv6.SetAddress(atePort1.IPv6).SetGateway(dutPort1.IPv6).SetPrefix(uint32(atePort1.IPv6Len))

	iDut2Dev := config.Devices().Add().SetName(atePort2.Name)
	iDut2Eth := iDut2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	iDut2Eth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(port2.Name())
	iDut2Ipv4 := iDut2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	iDut2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	iDut2Ipv6 := iDut2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	iDut2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	config.Captures().Add().SetName("packetCapture").SetPortNames([]string{port2.Name()}).SetFormat(gosnappi.CaptureFormat.PCAP)

	t.Logf("Pushing config to ATE and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
	t.Log(config.Msg().GetCaptures())
	return config
}

func createGoodFlows(t *testing.T, config gosnappi.Config, otg *otg.OTG) {
	t.Helper()

	config.Flows().Clear()

	flow1 := gosnappi.NewFlow().SetName(flow4in4)
	flow1.Metrics().SetEnable(true)
	flow1.TxRx().Device().
		SetTxNames([]string{atePort1.Name + ".IPv4"}).
		SetRxNames([]string{atePort2.Name + ".IPv4"})
	flow1.Size().SetFixed(512)
	flow1.Rate().SetPps(100)
	flow1.Duration().SetChoice("continuous")
	ethHeader1 := flow1.Packet().Add().Ethernet()
	ethHeader1.Src().SetValue(atePort1.MAC)
	outerIPHeader1 := flow1.Packet().Add().Ipv4()
	outerIPHeader1.Src().SetValue(ipv4OuterSrc111)
	outerIPHeader1.Dst().SetValue(ipv4OuterDst111)
	innerIPHeader1 := flow1.Packet().Add().Ipv4()
	innerIPHeader1.Src().SetValue(atePort1.IPv4)
	innerIPHeader1.Dst().SetValue(atePort2.IPv4)

	flow2 := gosnappi.NewFlow().SetName(flow6in4)
	flow2.Metrics().SetEnable(true)
	flow2.TxRx().Device().
		SetTxNames([]string{atePort1.Name + ".IPv4"}).
		SetRxNames([]string{atePort2.Name + ".IPv4"})
	flow2.Size().SetFixed(512)
	flow2.Rate().SetPps(100)
	flow2.Duration().SetChoice("continuous")
	ethHeader2 := flow2.Packet().Add().Ethernet()
	ethHeader2.Src().SetValue(atePort1.MAC)
	outerIPHeader2 := flow2.Packet().Add().Ipv4()
	outerIPHeader2.Src().SetValue(ipv4OuterSrc111)
	outerIPHeader2.Dst().SetValue(ipv4OuterDst111)
	innerIPv6Header2 := flow2.Packet().Add().Ipv6()
	innerIPv6Header2.Src().SetValue(atePort1.IPv6)
	innerIPv6Header2.Dst().SetValue(atePort2.IPv6)

	flowList := []gosnappi.Flow{flow1, flow2}
	for _, flow := range flowList {
		config.Flows().Append(flow)
	}

	t.Logf("Pushing traffic flows to OTG and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
}

func createNegTestFlow(t *testing.T, config gosnappi.Config, otg *otg.OTG) {
	t.Helper()

	config.Flows().Clear()

	flow3 := gosnappi.NewFlow().SetName(flowNegTest)
	flow3.Metrics().SetEnable(true)
	flow3.TxRx().Device().
		SetTxNames([]string{atePort1.Name + ".IPv4"}).
		SetRxNames([]string{atePort2.Name + ".IPv4"})
	flow3.Size().SetFixed(512)
	flow3.Rate().SetPps(100)
	flow3.Duration().SetChoice("continuous")
	ethHeader3 := flow3.Packet().Add().Ethernet()
	ethHeader3.Src().SetValue(atePort1.MAC)
	outerIPHeader3 := flow3.Packet().Add().Ipv4()
	outerIPHeader3.Src().SetValue(ipv4OuterSrc111)
	outerIPHeader3.Dst().SetValue(ipv4OuterDst333)
	innerIPHeader3 := flow3.Packet().Add().Ipv4()
	innerIPHeader3.Src().SetValue(atePort1.IPv4)
	innerIPHeader3.Dst().SetValue(atePort2.IPv4)

	config.Flows().Append(flow3)

	t.Logf("Pushing traffic flows to OTG and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
}

func sendTraffic(t *testing.T, args *testArgs) {
	t.Helper()

	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	args.otg.SetControlState(t, cs)

	t.Logf("Starting traffic")
	args.otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	args.otg.StopTraffic(t)
}

func verifyTraffic(t *testing.T, args *testArgs, flowList []string, wantLoss, validateTTL, checkDecap bool) {
	t.Helper()
	for _, flowName := range flowList {
		t.Logf("Verifying flow metrics for the flow %s\n", flowName)
		recvMetric := gnmi.Get(t, args.otg, gnmi.OTG().Flow(flowName).State())
		txPackets := recvMetric.GetCounters().GetOutPkts()
		rxPackets := recvMetric.GetCounters().GetInPkts()
		lostPackets := txPackets - rxPackets
		var lossPct uint64
		if txPackets != 0 {
			lossPct = lostPackets * 100 / txPackets
		} else {
			t.Errorf("Traffic stats are not correct %v", recvMetric)
		}
		if wantLoss {
			if lossPct < 100-tolerancePct {
				t.Errorf("Traffic is expected to fail %s\n got %v, want 100%% failure", flowName, lossPct)
			} else {
				t.Logf("Traffic Loss Test Passed!")
			}
		} else {
			if lossPct > tolerancePct {
				t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want 0", flowName, lossPct)
			} else {
				t.Logf("Traffic Test Passed!")
			}
		}
	}
	bytes := args.otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(args.otgConfig.Ports().Items()[1].Name()))
	f, err := os.CreateTemp("", "pcap")
	if err != nil {
		t.Fatalf("ERROR: Could not create temporary pcap file: %v\n", err)
	}
	if _, err := f.Write(bytes); err != nil {
		t.Fatalf("ERROR: Could not write bytes to pcap file: %v\n", err)
	}
	f.Close()
	ValidatePackets(t, f.Name(), validateTTL, checkDecap)
}

func ValidatePackets(t *testing.T, filename string, validateTTL, checkDecap bool) {
	t.Helper()
	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	if validateTTL {
		validateTrafficTTL(t, packetSource)
	}
	if checkDecap {
		validateTrafficDecap(t, packetSource)
	}
}

func validateTrafficTTL(t *testing.T, packetSource *gopacket.PacketSource) {
	t.Helper()
	dut := ondatra.DUT(t, "dut")
	var v4PacketCheckCount, v6PacketCheckCount uint32 = 0, 0
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer != nil && v4PacketCheckCount <= 3 {
			v4PacketCheckCount++
			ipPacket, _ := ipLayer.(*layers.IPv4)
			if !deviations.TTLCopyUnsupported(dut) {
				if ipPacket.TTL != correspondingTTL {
					t.Errorf("IP TTL value is altered to: %d", ipPacket.TTL)
				}
			}
			innerPacket := gopacket.NewPacket(ipPacket.Payload, ipPacket.NextLayerType(), gopacket.Default)
			ipInnerLayer := innerPacket.Layer(layers.LayerTypeIPv4)
			ipv6InnerLayer := innerPacket.Layer(layers.LayerTypeIPv6)
			if ipInnerLayer != nil {
				t.Errorf("Packets are not decapped, Inner IP/IPv6 header is not removed.")
			}
			if ipv6InnerLayer != nil {
				t.Errorf("Packets are not decapped, Inner IPv6 header is not removed.")
			}
		}
		ipv6Layer := packet.Layer(layers.LayerTypeIPv6)
		if ipv6Layer != nil && v6PacketCheckCount <= 3 {
			v6PacketCheckCount++
			ipv6Packet, _ := ipv6Layer.(*layers.IPv6)
			if !deviations.TTLCopyUnsupported(dut) {
				if ipv6Packet.HopLimit != correspondingHopLimit {
					t.Errorf("IPv6 hoplimit value is altered to %d", ipv6Packet.HopLimit)
				}
			}
			innerPacket := gopacket.NewPacket(ipv6Packet.Payload, ipv6Packet.NextLayerType(), gopacket.Default)
			ipv6InnerLayer := innerPacket.Layer(layers.LayerTypeIPv6)
			if ipv6InnerLayer != nil {
				t.Errorf("Packets are not decapped, Inner IP/IPv6 header is not removed.")
			}
		}
	}
}

func validateTrafficDecap(t *testing.T, packetSource *gopacket.PacketSource) {
	t.Helper()
	for packet := range packetSource.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		ipPacket, _ := ipLayer.(*layers.IPv4)
		innerPacket := gopacket.NewPacket(ipPacket.Payload, ipPacket.NextLayerType(), gopacket.Default)
		ipInnerLayer := innerPacket.Layer(layers.LayerTypeIPv4)
		if ipInnerLayer == nil {
			if ipPacket.DstIP.String() != ipv4OuterDst333 {
				t.Errorf("Negatice test for Decap failed. Traffic sent to route which does not match the decap route are decaped")
			}
			ipInnerPacket, _ := ipInnerLayer.(*layers.IPv4)
			if ipInnerPacket.DstIP.String() != atePort2.IPv4 {
				t.Errorf("Negatice test for Decap failed. Traffic sent to route which does not match the decap route are decaped")
			}
			t.Logf("Traffic for non decap routes passed.")
			break
		}
	}
}

func configStaticRoute(t *testing.T, dut *ondatra.DUTDevice, prefix string, nexthop string) {
	t.Helper()
	ni := oc.NetworkInstance{Name: ygot.String(deviations.DefaultNetworkInstance(dut))}
	static := ni.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	sr := static.GetOrCreateStatic(prefix)
	nh := sr.GetOrCreateNextHop("0")
	nh.NextHop = oc.UnionString(nexthop)
	gnmi.Update(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut)).Config(), static)
}

// TestSingleDecapGribiEntry is to test support for decap action for gRIBI route.
func TestSingleDecapGribiEntry(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	gribic := dut.RawAPIs().GRIBI(t)
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()

	t.Run("Configure Default Network Instance", func(t *testing.T) {
		fptest.ConfigureDefaultNetworkInstance(t, dut)
	})

	t.Run("Configure Non-Default Network Instances", func(t *testing.T) {
		configNonDefaultNetworkInstance(t, dut)
	})

	t.Run("Configure interfaces on DUT", func(t *testing.T) {
		configureDUT(t, dut)
	})

	t.Run("Apply vrf selectioin policy to DUT port-1", func(t *testing.T) {
		configureVrfSelectionPolicy(t, dut)
	})

	otg := ate.OTG()
	var otgConfig gosnappi.Config
	t.Run("Configure OTG", func(t *testing.T) {
		otgConfig = configureOTG(t, otg)
	})

	negTestAddr := fmt.Sprintf("%s/%d", ipv4OuterDst333, uint32(32))
	t.Run("Add static route for validating negative traffic test", func(t *testing.T) {
		configStaticRoute(t, dut, negTestAddr, atePort2.IPv4)
	})

	// Connect gRIBI client to DUT referred to as gRIBI - using PRESERVE persistence and
	// SINGLE_PRIMARY mode, with FIB ACK requested. Specify gRIBI as the leader.
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(1, 0).
		WithFIBACK().WithRedundancyMode(fluent.ElectedPrimaryClient)
	client.Start(ctx, t)
	defer client.Stop(t)

	defer func() {
		// Flush all entries after test.
		if err := gribi.FlushAll(client); err != nil {
			t.Error(err)
		}
	}()

	client.StartSending(ctx, t)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for clientA: %v", err)
	}
	eID := gribi.BecomeLeader(t, client)

	args := &testArgs{
		ctx:        ctx,
		client:     client,
		dut:        dut,
		ate:        ate,
		otgConfig:  otgConfig,
		top:        top,
		electionID: eID,
		otg:        otg,
	}

	cases := []struct {
		desc   string
		prefix string
		mask   string
	}{{
		desc:   "Mask Length 24",
		prefix: "192.51.100.0",
		mask:   "24",
	}, {
		desc:   "Mask Length 32",
		prefix: "192.51.100.64",
		mask:   "32",
	}, {
		desc:   "Mask Length 28",
		prefix: "192.51.100.64",
		mask:   "28",
	}, {
		desc:   "Mask Length 22",
		prefix: "192.51.100.0",
		mask:   "22",
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Log("Flush existing gRIBI routes before test.")
			if err := gribi.FlushAll(client); err != nil {
				t.Fatal(err)
			}

			prefixWithMask := fmt.Sprintf("%s/%s", tc.prefix, tc.mask)
			t.Run("Program gRIBi route", func(t *testing.T) {
				configureGribiRoute(ctx, t, dut, args, prefixWithMask)
			})
			// Send both 6in4 and 4in4 packets. Verify that the packets have their outer
			// v4 header stripped and are forwarded according to the route in the DEFAULT
			// VRF that matches the inner IP address.
			t.Run("Create ip-in-ip and ipv6-in-ip flows, send traffic and verify decap functionality",
				func(t *testing.T) {
					createGoodFlows(t, otgConfig, otg)
					sendTraffic(t, args)
					if deviations.SkipV6TrafficCheckPostDecap(dut) {
						verifyTraffic(t, args, []string{flow4in4}, !wantLoss, checkTTL, !checkDecap)
					} else {
						verifyTraffic(t, args, []string{flow4in4, flow6in4}, !wantLoss, checkTTL, !checkDecap)
					}
				})

			// Test with packets with a destination address 203.0.113.1 such as that does not match
			// the decap route, and verify that such packets are not decapped.
			t.Run("Send traffic to non decap route and verify the behavior",
				func(t *testing.T) {
					createNegTestFlow(t, otgConfig, otg)
					sendTraffic(t, args)
					verifyTraffic(t, args, []string{flowNegTest}, !wantLoss, !checkTTL, checkDecap)
				})
		})
	}
}
