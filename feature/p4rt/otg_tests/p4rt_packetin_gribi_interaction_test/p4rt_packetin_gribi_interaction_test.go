// Copyright 2026 Google LLC
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

package p4rtpacketingribiinteraction_test

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/cisco-open/go-p4/utils"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/iputil"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/p4rtutils"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
	p4v1pb "github.com/p4lang/p4runtime/go/p4/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	ipv4PrefixLen        = 30
	ipv6PrefixLen        = 126
	deviceID             = uint64(1)
	leaderElectionID     = uint64(100)
	lldpMac              = "01:80:c2:00:00:0e"
	ingressPortID        = uint32(10)
	metadataIngressPort  = uint32(1)
	metadataEgressPort   = uint32(2)
	destinationPrefixv4  = "198.51.100.0/24"
	destinationPrefixv6  = "2001:db8:100::/64"
	nhv4Index            = uint64(10)
	nhv6Index            = uint64(11)
	nhgv4Index           = uint64(100)
	nhgv6Index           = uint64(101)
	streamNameLeader     = "p4rt-leader"
	frameSize            = 1514
	frameCount           = 50000
	framePps             = uint64(1000)
	concurrentRouteCount = 100000
	concurrentBatchSize  = 1000
	trafficTimeout       = time.Duration(frameCount/framePps)*time.Second + 40*time.Second
	packetInQueueSize    = 200000
	traceRouteTTL        = uint8(1)
	v4Route              = "198.18.0.0"
	v4RouteLen           = "/32"
	v6Route              = "2001:db8::"
	v6RouteLen           = "/128"
	tracerouteSrcMAC     = "00:01:00:02:00:03"
)

var (
	p4InfoFile = flag.String("p4info_file_location", "../../data/wbb.p4info.pb.txt", "Path to the p4info file.")
	atePort1   = attrs.Attributes{
		Name:    "atePort1",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:1::2",
		IPv6Len: ipv6PrefixLen,
		MAC:     "02:00:00:00:00:01",
	}

	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
		ID:      ingressPortID,
		IPv6:    "2001:db8:1::1",
		IPv6Len: ipv6PrefixLen,
		MAC:     "00:1A:11:00:00:01",
	}

	atePort2 = attrs.Attributes{
		Name:    "atePort2",
		IPv4:    "192.0.2.6",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:2::2",
		IPv6Len: ipv6PrefixLen,
		MAC:     "02:00:00:00:00:02",
	}

	dutPort2 = attrs.Attributes{
		Desc:    "dutPort2",
		IPv4:    "192.0.2.5",
		IPv4Len: ipv4PrefixLen,
		ID:      ingressPortID + 1,
		IPv6:    "2001:db8:2::1",
		IPv6Len: ipv6PrefixLen,
		MAC:     "00:1A:11:00:00:02",
	}

	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "192.0.2.10",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:3::2",
		IPv6Len: ipv6PrefixLen,
		MAC:     "02:00:00:00:00:03",
	}

	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv4Len: ipv4PrefixLen,
		ID:      ingressPortID + 2,
		IPv6:    "2001:db8:3::1",
		IPv6Len: ipv6PrefixLen,
		MAC:     "00:1A:11:00:00:03",
	}

	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		IPv4:    "192.0.2.14",
		IPv4Len: ipv4PrefixLen,
		IPv6:    "2001:db8:4::2",
		IPv6Len: ipv6PrefixLen,
		MAC:     "02:00:00:00:00:04",
	}

	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.13",
		IPv4Len: ipv4PrefixLen,
		ID:      ingressPortID + 3,
		IPv6:    "2001:db8:4::1",
		IPv6Len: ipv6PrefixLen,
		MAC:     "00:1A:11:00:00:04",
	}

	otgPorts = map[string]*attrs.Attributes{
		"port1": &atePort1,
		"port2": &atePort2,
		"port3": &atePort3,
		"port4": &atePort4,
	}

	dutPorts = map[string]*attrs.Attributes{
		"port1": &dutPort1,
		"port2": &dutPort2,
		"port3": &dutPort3,
		"port4": &dutPort4,
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testArgs struct {
	leader     *p4rt_client.P4RTClient
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	otgConfig  gosnappi.Config
	packetIO   []packetIO
	gribiC     *gribi.Client
	ni         string
	v4Prefixes []string
	v6Prefixes []string
}

type dataplaneValidation struct {
	srcMac      string
	dstMac      string
	expectedTTL uint8
	srcIPv4     string
	dstIPv4     []string
	srcIPv6     string
	dstIPv6     []string
	capturePort string
}

type flowSpec struct {
	flowName   string
	frameSize  uint32
	framePps   uint64
	frameCount uint32
	continuous bool
	isIPv6     bool
	dstMAC     string
	dstIP      string
	srcMAC     string
	srcIP      string
	lldpFlow   bool
	traceRoute bool
}

type packetIOPacket struct {
	ttl            uint8
	srcMAC, dstMAC *string
	ethernetType   *uint32
}

type tracerouteP4RTPacketIO struct {
	packetIOPacket
	ingressPort uint32
	egressPort  uint32
}

type lldpBaselinePacketIO struct {
	packetIOPacket
	ingressPort uint32
	egressPort  uint32
}

type packetIO interface {
	GetPuntTableEntry(isDelete bool) ([]*p4rtutils.ACLWbbIngressTableEntryInfo, error)
	GetPacketTemplate() *packetIOPacket
	GetTrafficFlow(ate *ondatra.ATEDevice, flowname string, frameSize uint32, frameRate uint64, frameCount uint32, traceRoute bool) []gosnappi.Flow
	GetEgressPort() []string
	GetIngressPort() uint32
}

type puntRule struct {
	Name      string
	EtherType uint16
	IsIPv4    bool
	TTL       uint8
	IPv4Dst   string
	IPProto   uint8
	L4DstPort uint16
}

var puntRules = []puntRule{
	{Name: "LLDP", EtherType: 0x88CC},
	{Name: "IPv4Traceroute", IsIPv4: true, TTL: 1},
	// TODO: https://partnerissuetracker.corp.google.com/u/0/issues/567774034
	// {Name: "BGP", IPv4Dst: "192.0.2.1/32", IPProto: 6, L4DstPort: 179},
	// {Name: "OSPF", IsIPv4: true, IPProto: 89},
}

// getLLDPBaselinePacketIO builds the packetIO used by the baseline sub-test.
func getLLDPBaselinePacketIO() *lldpBaselinePacketIO {
	srcMAC := atePort1.MAC
	dstMAC := lldpMac
	etherType := uint32(0x88CC)
	return &lldpBaselinePacketIO{
		packetIOPacket: packetIOPacket{
			srcMAC:       &srcMAC,
			dstMAC:       &dstMAC,
			ethernetType: &etherType,
		},
		ingressPort: ingressPortID,
	}
}

// getTracerouteParameter returns Traceroute related parameters for startTraficAndTestPacketIn testcase.
func getTracerouteParameter(t *testing.T, testArgs *testArgs) *tracerouteP4RTPacketIO {
	t.Helper()
	srcMAC := tracerouteSrcMAC
	llAddress, found := gnmi.Watch(t, testArgs.ate.OTG(), gnmi.OTG().Interface(atePort1.Name+".Eth").Ipv4Neighbor(dutPort1.IPv4).LinkLayerAddress().State(), time.Minute, func(val *ygnmi.Value[string]) bool {
		return val.IsPresent()
	}).Await(t)
	if !found {
		t.Fatalf("Could not get the LinkLayerAddress %s", llAddress)
	}
	traceRouteDstMac, _ := llAddress.Val()
	return &tracerouteP4RTPacketIO{
		packetIOPacket: packetIOPacket{
			ttl:    traceRouteTTL,
			srcMAC: &srcMAC,
			dstMAC: &traceRouteDstMac,
		},
		ingressPort: ingressPortID,
		egressPort:  ingressPortID + 1,
	}
}

func (p *lldpBaselinePacketIO) GetPuntTableEntry(isDelete bool) ([]*p4rtutils.ACLWbbIngressTableEntryInfo, error) {
	return buildPuntTableEntries(isDelete)
}

func (p *tracerouteP4RTPacketIO) GetPuntTableEntry(isDelete bool) ([]*p4rtutils.ACLWbbIngressTableEntryInfo, error) {
	return buildPuntTableEntries(isDelete)
}

func (p *lldpBaselinePacketIO) GetPacketTemplate() *packetIOPacket {
	return &p.packetIOPacket
}

func (p *tracerouteP4RTPacketIO) GetPacketTemplate() *packetIOPacket {
	return &p.packetIOPacket
}

func (p *lldpBaselinePacketIO) GetTrafficFlow(ate *ondatra.ATEDevice, flowname string, frameSize uint32, frameRate uint64, frameCount uint32, traceRoute bool) []gosnappi.Flow {
	return buildTrafficFlows(flowname, &p.packetIOPacket, frameSize, frameRate, frameCount, traceRoute)
}

func (p *tracerouteP4RTPacketIO) GetTrafficFlow(ate *ondatra.ATEDevice, flowname string, frameSize uint32, frameRate uint64, frameCount uint32, traceRoute bool) []gosnappi.Flow {
	return buildTrafficFlows(flowname, &p.packetIOPacket, frameSize, frameRate, frameCount, traceRoute)
}

func (p *lldpBaselinePacketIO) GetEgressPort() []string {
	return buildEgressPorts(p.egressPort)
}

func (p *tracerouteP4RTPacketIO) GetEgressPort() []string {
	return buildEgressPorts(p.egressPort)
}

func (p *lldpBaselinePacketIO) GetIngressPort() uint32 {
	return p.ingressPort
}

func (p *tracerouteP4RTPacketIO) GetIngressPort() uint32 {
	return p.ingressPort
}

// buildPuntTableEntries is the generic punt-table generator used by all packetIO types.
func buildPuntTableEntries(isDelete bool) ([]*p4rtutils.ACLWbbIngressTableEntryInfo, error) {
	actionType := p4v1pb.Update_INSERT
	if isDelete {
		actionType = p4v1pb.Update_DELETE
	}
	var entries []*p4rtutils.ACLWbbIngressTableEntryInfo
	for _, rule := range puntRules {
		entry, err := getPuntTableEntry(rule, actionType)
		if err != nil {
			return nil, fmt.Errorf("punt rule %q: %v", rule.Name, err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func buildEgressPorts(egressPort uint32) []string {
	if egressPort == 0 {
		return []string{"0"}
	}
	return []string{fmt.Sprint(egressPort)}
}

func buildTrafficFlows(flowname string, packetParams *packetIOPacket, frameSize uint32, frameRate uint64, frameCount uint32, traceRoute bool) []gosnappi.Flow {
	flow := gosnappi.NewFlow()
	flow.SetName(flowname)

	eth := flow.Packet().Add().Ethernet()
	if packetParams != nil {
		if packetParams.srcMAC != nil {
			eth.Src().SetValue(*packetParams.srcMAC)
		}
		if packetParams.dstMAC != nil {
			eth.Dst().SetValue(*packetParams.dstMAC)
		}
		if packetParams.ethernetType != nil {
			eth.EtherType().SetValue(*packetParams.ethernetType)
		}
	}

	if traceRoute {
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(atePort1.IPv4)
		v4.Dst().SetValue(atePort2.IPv4)
		if packetParams.ttl != 0 {
			v4.TimeToLive().SetValue(uint32(packetParams.ttl))
		}
	}

	flow.Size().SetFixed(frameSize)
	flow.Rate().SetPps(frameRate)
	if frameCount == 0 {
		flow.Duration().Continuous()
	} else {
		flow.Duration().FixedPackets().SetPackets(frameCount)
	}
	return []gosnappi.Flow{flow}
}

func getPuntTableEntry(rule puntRule, actionType p4v1pb.Update_Type) (*p4rtutils.ACLWbbIngressTableEntryInfo, error) {
	entry := &p4rtutils.ACLWbbIngressTableEntryInfo{
		Type:     actionType,
		Priority: 1,
	}
	if rule.EtherType != 0 {
		entry.EtherType = rule.EtherType
		entry.EtherTypeMask = 0xFFFF
	}
	if rule.IsIPv4 {
		entry.IsIpv4 = 0x1
	}
	if rule.TTL != 0 {
		entry.TTL = rule.TTL
		entry.TTLMask = 0xFF
	}
	return entry, nil
}

func mustConfigureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := gnmi.OC()
	portOrder := []string{"port1", "port2", "port3", "port4"}

	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	for portName, port := range dutPorts {
		p := dut.Port(t, portName)
		gnmi.Replace(t, dut, d.Interface(p.Name()).Config(), port.NewOCInterface(p.Name(), dut))
	}

	if deviations.ExplicitPortSpeed(dut) {
		for _, portName := range portOrder {
			p := dut.Port(t, portName)
			fptest.SetPortSpeed(t, p)
		}
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, portName := range portOrder {
			p := dut.Port(t, portName)
			fptest.AssignToNetworkInstance(t, dut, p.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice) gosnappi.Config {
	t.Helper()
	otgConfig := gosnappi.NewConfig()

	for portName, portAttrs := range otgPorts {
		port := ate.Port(t, portName)
		dutPort := dutPorts[portName]
		portAttrs.AddToOTG(otgConfig, port, dutPort)
	}
	return otgConfig
}

func configureP4RTNode(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	nodes := p4rtutils.P4RTNodesByPort(t, dut)
	p := dut.Port(t, "port1")
	nodeName, ok := nodes[p.ID()]
	if !ok {
		t.Fatalf("could not find p4rt node for port %v", "port1")
	}
	t.Logf("P4RT node for port1 is %v", nodeName)

	gnmi.Replace(t, dut, gnmi.OC().Component(nodeName).Config(), configP4RTNode(nodeName, deviceID))
}

func configP4RTNode(nodeID string, deviceID uint64) *oc.Component {
	c := &oc.Component{}
	c.Name = ygot.String(nodeID)
	c.IntegratedCircuit = &oc.Component_IntegratedCircuit{}
	c.IntegratedCircuit.NodeId = ygot.Uint64(deviceID)
	return c
}

func setupP4RTClient(t *testing.T, client *p4rt_client.P4RTClient, streamName string, electionID uint64) error {
	t.Helper()
	streamParam := p4rt_client.P4RTStreamParameters{
		Name:        streamName,
		DeviceId:    deviceID,
		ElectionIdH: 0,
		ElectionIdL: electionID,
	}
	if err := client.StreamChannelCreate(&streamParam); err != nil {
		return fmt.Errorf("could not create p4rt stream: %v", err)
	}
	if err := client.StreamChannelSendMsg(&streamName, &p4v1pb.StreamMessageRequest{
		Update: &p4v1pb.StreamMessageRequest_Arbitration{
			Arbitration: &p4v1pb.MasterArbitrationUpdate{
				DeviceId: deviceID,
				ElectionId: &p4v1pb.Uint128{
					High: 0,
					Low:  electionID,
				},
			},
		},
	}); err != nil {
		return fmt.Errorf("could not send p4rt arbitration: %v", err)
	}
	if _, _, arbErr := client.StreamChannelGetArbitrationResp(&streamName, 1); arbErr != nil {
		if err := p4rtutils.StreamTermErr(client.StreamTermErr); err != nil {
			return err
		}
		return errors.New("errors seen in ClientArbitration response")
	}
	if stream := client.StreamChannelGet(&streamName); stream != nil {
		stream.SetPacketQSize(packetInQueueSize)
	}
	return nil
}

func setPipeline(t *testing.T, client *p4rt_client.P4RTClient, electionID uint64) error {
	t.Helper()
	p4Info, err := utils.P4InfoLoad(p4InfoFile)
	if err != nil {
		return fmt.Errorf("could not load p4info file: %v", err)
	}
	return client.SetForwardingPipelineConfig(&p4v1pb.SetForwardingPipelineConfigRequest{
		DeviceId:   deviceID,
		ElectionId: &p4v1pb.Uint128{High: 0, Low: electionID},
		Action:     p4v1pb.SetForwardingPipelineConfigRequest_VERIFY_AND_COMMIT,
		Config: &p4v1pb.ForwardingPipelineConfig{
			P4Info: p4Info,
			Cookie: &p4v1pb.ForwardingPipelineConfig_Cookie{Cookie: 159},
		},
	})
}

func setupGRIBI(t *testing.T, dut *ondatra.DUTDevice) *gribi.Client {
	t.Helper()
	c := &gribi.Client{
		DUT:         dut,
		FIBACK:      true,
		Persistence: true,
	}
	if err := c.Start(t); err != nil {
		t.Fatalf("could not initialize gribi client: %v", err)
	}
	c.BecomeLeader(t)
	return c
}

func programBaselineRoute(t *testing.T, dut *ondatra.DUTDevice, nhIndex, nhgIndex uint64, nextHopIP, prefix, ipVersion string) []fluent.GRIBIEntry {
	t.Helper()
	entries := []fluent.GRIBIEntry{}
	switch ipVersion {
	case "v6":
		entries = []fluent.GRIBIEntry{
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(nhIndex).WithIPAddress(nextHopIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(nhgIndex).AddNextHop(nhIndex, 1),
			fluent.IPv6Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithPrefix(prefix).WithNextHopGroup(nhgIndex),
		}
	case "v4":
		entries = []fluent.GRIBIEntry{
			fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(nhIndex).WithIPAddress(nextHopIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(nhgIndex).AddNextHop(nhIndex, 1),
			fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithPrefix(prefix).WithNextHopGroup(nhgIndex),
		}
	default:
		t.Fatalf("programBaselineRoute: unsupported ip version %q, want \"v4\" or \"v6\"", ipVersion)
	}

	return entries
}

func verifyAFTPrefix(t *testing.T, dut *ondatra.DUTDevice, ni, prefix string) {
	t.Helper()
	if strings.Contains(prefix, ":") {
		path := gnmi.OC().NetworkInstance(ni).Afts().Ipv6Entry(prefix).State()
		if got, ok := gnmi.Watch(t, dut, path, time.Minute*2, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			ipv6Entry, present := val.Val()
			if present && ipv6Entry.GetPrefix() == prefix {
				t.Logf("got expected v6 entry: got %+v, present: %v", ipv6Entry.GetPrefix(), present)
				return true
			}
			return false
		}).Await(t); !ok {
			t.Errorf("error: expected v6 route not found in aft, got %v, want %v", got, prefix)
		}
	} else {
		path := gnmi.OC().NetworkInstance(ni).Afts().Ipv4Entry(prefix).State()
		if got, ok := gnmi.Watch(t, dut, path, time.Minute*2, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			ipv4Entry, present := val.Val()
			if present && ipv4Entry.GetPrefix() == prefix {
				t.Logf("got expected v4 entry: got %+v, present: %v", ipv4Entry.GetPrefix(), present)
				return true
			}
			return false
		}).Await(t); !ok {
			t.Errorf("error: expected v4 route not found in aft, got %v, want %v", got, prefix)
		}
	}
}

func programPuntTableEntries(t *testing.T, client *p4rt_client.P4RTClient, packetIO packetIO, isDelete bool) error {
	t.Helper()

	entries, err := packetIO.GetPuntTableEntry(isDelete)
	if err != nil {
		return fmt.Errorf("failed to get punt table entries: %w", err)
	}

	writeErr := client.Write(&p4v1pb.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &p4v1pb.Uint128{High: 0, Low: leaderElectionID},
		Updates:    p4rtutils.ACLWbbIngressTableEntryGet(entries),
		Atomicity:  p4v1pb.WriteRequest_CONTINUE_ON_ERROR,
	})
	if writeErr != nil {
		return writeErr
	}
	return nil
}

func configureTrafficFlow(t *testing.T, testArgs *testArgs, flows []flowSpec, packetIO packetIO) {
	t.Helper()
	for _, flow := range flows {
		ingressPortName := fmt.Sprint(testArgs.dut.Port(t, "port1").ID())
		if flow.lldpFlow {
			lldpFlows := packetIO.GetTrafficFlow(testArgs.ate, flow.flowName, flow.frameSize, flow.framePps, flow.frameCount, false)
			for _, lldp := range lldpFlows {
				lldp.TxRx().Port().SetTxName(ingressPortName).SetRxName(ingressPortName)
				lldp.Metrics().SetEnable(true)
				testArgs.otgConfig.Flows().Append(lldp)
			}
		} else if flow.traceRoute {
			traceRouteFlows := packetIO.GetTrafficFlow(testArgs.ate, flow.flowName, flow.frameSize, flow.framePps, flow.frameCount, true)
			for _, traceRoute := range traceRouteFlows {
				traceRoute.TxRx().Port().SetTxName(ingressPortName).SetRxName(ingressPortName)
				traceRoute.Metrics().SetEnable(true)
				testArgs.otgConfig.Flows().Append(traceRoute)
			}
		} else {
			trafficFlow := testArgs.otgConfig.Flows().Add().SetName(flow.flowName)
			trafficFlow.Metrics().SetEnable(true)
			eth := trafficFlow.Packet().Add().Ethernet()
			eth.Src().SetValue(flow.srcMAC)
			if flow.isIPv6 {
				trafficFlow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort2.Name + ".IPv6"})
				ipv6 := trafficFlow.Packet().Add().Ipv6()
				ipv6.Src().SetValue(flow.srcIP)
				ipv6.Dst().SetValue(flow.dstIP)
			} else {
				trafficFlow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort2.Name + ".IPv4"})
				ipv4 := trafficFlow.Packet().Add().Ipv4()
				ipv4.Src().SetValue(flow.srcIP)
				ipv4.Dst().SetValue(flow.dstIP)
			}
			trafficFlow.Size().SetFixed(flow.frameSize)
			trafficFlow.Rate().SetPps(flow.framePps)
			if flow.continuous {
				trafficFlow.Duration().Continuous()
			} else {
				trafficFlow.Duration().FixedPackets().SetPackets(flow.frameCount)
			}
		}
	}
}

func waitForFlowPacketCountWatch(t *testing.T, ate *ondatra.ATEDevice, flowName []string, timeout time.Duration, want uint64) uint64 {
	t.Helper()
	if len(flowName) == 0 {
		return 0
	}
	var total uint64
	for _, flow := range flowName {
		path := gnmi.OTG().Flow(flow).Counters().OutPkts().State()
		val, ok := gnmi.Watch(t, ate.OTG(), path, timeout, func(v *ygnmi.Value[uint64]) bool {
			got, present := v.Val()
			if want == 0 {
				return present && got > 0
			}
			return present && got >= want
		}).Await(t)
		if !ok {
			if want == 0 {
				t.Errorf("flow %q did not transmit any packets within %v", flow, timeout)
			} else {
				t.Errorf("flow %q did not reach %d packets within %v", flow, want, timeout)
			}
			continue
		}
		got, _ := val.Val()
		total += got
	}
	return total
}

func validateTrafficFlow(t *testing.T, testArgs *testArgs, flowName []string, frameCount uint64, timeout time.Duration) uint64 {
	t.Helper()
	testArgs.ate.OTG().PushConfig(t, testArgs.otgConfig)
	testArgs.ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, testArgs.ate.OTG(), testArgs.otgConfig, "IPv4")
	otgutils.WaitForARP(t, testArgs.ate.OTG(), testArgs.otgConfig, "IPv6")

	cs := gosnappi.NewControlState()
	cs.Traffic().FlowTransmit().
		SetState(gosnappi.StateTrafficFlowTransmitState.START).
		SetFlowNames(flowName)
	testArgs.ate.OTG().SetControlState(t, cs)

	pktOut := waitForFlowPacketCountWatch(t, testArgs.ate, flowName, timeout, frameCount)

	otgutils.LogFlowMetrics(t, testArgs.ate.OTG(), testArgs.otgConfig)
	return pktOut
}

func validateDUTInterfaceCounters(t *testing.T, dut *ondatra.DUTDevice, port string, wantedCounter uint64, inPkts bool) {
	t.Helper()
	var path ygnmi.SingletonQuery[uint64]
	if inPkts {
		path = gnmi.OC().Interface(dut.Port(t, port).Name()).Counters().InPkts().State()
	} else {
		path = gnmi.OC().Interface(dut.Port(t, port).Name()).Counters().OutPkts().State()
	}

	val, ok := gnmi.Watch(t, dut, path, 2*time.Minute, func(v *ygnmi.Value[uint64]) bool {
		got, present := v.Val()
		return present && got >= wantedCounter
	}).Await(t)

	if !ok {
		got, _ := val.Val()
		t.Errorf("error: DUT interface %q packets got %d, want at least %d", port, got, wantedCounter)
	} else {
		got, _ := val.Val()
		t.Logf("DUT interface %q packets got %d, as expected: %d", port, got, wantedCounter)
	}
}

func validatePacketIn(t *testing.T, testArgs *testArgs, pktOut uint64, packetIO packetIO, traceroutePacketIn ...packetIO) {
	t.Helper()
	leaderStream := streamNameLeader
	_, packets, err := testArgs.leader.StreamChannelGetPackets(&leaderStream, pktOut, 5*time.Minute)
	if err != nil {
		t.Fatalf("StreamChannelGetPackets failed: %v", err)
	}

	if uint64(len(packets)) == 0 {
		t.Errorf("error: no packets received on leader p4rt stream, expected %d", pktOut)
		return
	}

	if len(packets) < int(pktOut) {
		t.Errorf("error: packets received on leader p4rt stream, got %d, expected %d", len(packets), int(pktOut))
	} else {
		t.Logf("received %d packet(s) on leader p4rt stream, as expected: %d", len(packets), pktOut)
	}

	if len(traceroutePacketIn) > 0 {
		err = errors.Join(err, checkTraceroutePacketIn(t, packets, traceroutePacketIn[0]))
	}
	err = errors.Join(err, checkLLDPPacketIn(t, packets, packetIO))

	if err == nil {
		if len(traceroutePacketIn) > 0 {
			t.Log("packet matched expected traceroute PacketIn")
		}
		t.Log("packet matched expected LLDP PacketIn")
		return
	}
	t.Errorf("error: packet did not match: %v", err)
}

func checkLLDPPacketIn(t *testing.T, packets []*p4rt_client.P4RTPacketInfo, packetIO packetIO) error {
	t.Helper()
	if len(packets) == 0 {
		return errors.New("found 0 LLDP packets")
	}
	for _, packet := range packets {
		lldpPacket := packetIO.GetPacketTemplate()
		if !checkHeaders(t, packet.Pkt.GetPayload(), lldpPacket.srcMAC, lldpPacket.dstMAC, lldpPacket.ethernetType) {
			continue
		}
		src := ""
		dst := ""
		ethType := uint32(0)
		if lldpPacket.srcMAC != nil {
			src = *lldpPacket.srcMAC
		}
		if lldpPacket.dstMAC != nil {
			dst = *lldpPacket.dstMAC
		}
		if lldpPacket.ethernetType != nil {
			ethType = *lldpPacket.ethernetType
		}
		t.Logf("found matching LLDP PacketIn srcMAC=%s dstMAC=%s ethernetType=0x%04x", src, dst, ethType)

		return checkMetadata(packet.Pkt.GetMetadata(), packetIO)
	}

	return errors.New("no matching LLDP PacketIn found")
}

func checkTraceroutePacketIn(t *testing.T, packets []*p4rt_client.P4RTPacketInfo, packetIO packetIO) error {
	t.Helper()
	if len(packets) == 0 {
		return errors.New("found 0 traceroute packets")
	}

	for _, packet := range packets {
		tracePacket := packetIO.GetPacketTemplate()
		if !checkHeaders(t, packet.Pkt.GetPayload(), tracePacket.srcMAC, tracePacket.dstMAC, tracePacket.ethernetType) {
			continue
		}
		src := ""
		dst := ""
		if tracePacket.srcMAC != nil {
			src = *tracePacket.srcMAC
		}
		if tracePacket.dstMAC != nil {
			dst = *tracePacket.dstMAC
		}
		t.Logf("found matching traceroute PacketIn srcMAC=%s dstMAC=%s", src, dst)

		return errors.Join(
			checkIPv4TTL(t, packet.Pkt.GetPayload(), tracePacket.ttl),
			checkMetadata(packet.Pkt.GetMetadata(), packetIO),
		)
	}

	return errors.New("no matching traceroute PacketIn found")
}

func checkHeaders(t *testing.T, payload []byte, wantSrc, wantDst *string, wantEthernetType *uint32) bool {
	t.Helper()

	if wantSrc == nil && wantDst == nil && wantEthernetType == nil {
		return true
	}

	srcMAC, dstMAC, etherType := decodePacket(t, payload)

	if wantDst != nil && !strings.EqualFold(dstMAC, *wantDst) {
		return false
	}
	if wantEthernetType != nil {
		if etherType != layers.EthernetType(*wantEthernetType) {
			return false
		}
	}
	if wantSrc != nil && !strings.EqualFold(srcMAC, *wantSrc) {
		return false
	}
	return true
}

func checkMetadata(metadata []*p4v1pb.PacketMetadata, packetIO packetIO) error {
	const wantMetadataCount = 2
	if len(metadata) != wantMetadataCount {
		return fmt.Errorf("error: metadata count got %d, want %d", len(metadata), wantMetadataCount)
	}

	wantIngress := fmt.Sprint(packetIO.GetIngressPort())
	wantEgress := packetIO.GetEgressPort()

	var errs []error
	for _, md := range metadata {
		got := string(md.GetValue())
		switch id := md.GetMetadataId(); id {
		case metadataIngressPort:
			if got != wantIngress {
				errs = append(errs, fmt.Errorf("error: ingress port got %q, want %q", got, wantIngress))
			}
		case metadataEgressPort:
			if !slices.Contains(wantEgress, got) {
				errs = append(errs, fmt.Errorf("error: egress port %q not in expected set %v", got, wantEgress))
			}
		default:
			errs = append(errs, fmt.Errorf("error: unrecognized metadata id %v", id))
		}
	}
	return errors.Join(errs...)
}

func checkIPv4TTL(t *testing.T, payload []byte, wantTTL uint8) error {
	t.Helper()
	if wantTTL == 0 {
		return nil
	}
	captureTTL := decodePacket4(t, payload)
	if captureTTL != wantTTL {
		return fmt.Errorf("error: Packet in message is not matching wanted packet=IPV4 TTL, got %d, want %d", captureTTL, wantTTL)
	} else {
		t.Logf("Packet in message matches wanted packet=IPV4 TTL, got %d, want %d", captureTTL, wantTTL)
	}
	return nil
}

func decodePacket4(t *testing.T, packetData []byte) uint8 {
	t.Helper()
	packet := gopacket.NewPacket(packetData[14:], layers.LayerTypeIPv4, gopacket.Default)
	if IPv4 := packet.Layer(layers.LayerTypeIPv4); IPv4 != nil {
		ipv4, _ := IPv4.(*layers.IPv4)
		ttl := ipv4.TTL
		return ttl
	}
	t.Errorf("error: failed to get IPv4 packet")
	return 0
}

func decodePacket(t *testing.T, packetData []byte) (string, string, layers.EthernetType) {
	t.Helper()
	packet := gopacket.NewPacket(packetData, layers.LayerTypeEthernet, gopacket.Default)
	etherHeader := packet.Layer(layers.LayerTypeEthernet)
	if etherHeader != nil {
		if header, ok := etherHeader.(*layers.Ethernet); ok {
			return header.SrcMAC.String(), header.DstMAC.String(), header.EthernetType
		}
	}
	return "", "", layers.EthernetType(0)
}

func generateConcurrentPrefixes(routeCount int) ([]string, []string, error) {
	v4Addrs, err := iputil.GenerateIPsWithStep(v4Route, routeCount, "0.0.0.1")
	if err != nil {
		return nil, nil, fmt.Errorf("GenerateIPsWithStep failed: %w", err)
	}
	v4Prefixes := make([]string, 0, len(v4Addrs))
	for _, addr := range v4Addrs {
		v4Prefixes = append(v4Prefixes, addr+v4RouteLen)
	}

	baseV6 := net.ParseIP(v6Route)
	v6Addrs, err := iputil.GenerateIPv6s(baseV6, routeCount)
	if err != nil {
		return nil, nil, fmt.Errorf("GenerateIPv6s failed: %w", err)
	}
	v6Prefixes := make([]string, 0, len(v6Addrs))
	for _, addr := range v6Addrs {
		v6Prefixes = append(v6Prefixes, addr+v6RouteLen)
	}

	return v4Prefixes, v6Prefixes, nil
}

func programConcurrentGRIBIRouteLifecycle(t *testing.T, testArgs *testArgs, v4Prefixes, v6Prefixes []string, batchSize int, actions []string) error {
	t.Helper()
	routeCount := len(v4Prefixes)
	const (
		v4NHPort3  = uint64(200)
		v4NHPort4  = uint64(201)
		v6NHPort3  = uint64(202)
		v6NHPort4  = uint64(203)
		v4NHGECMPA = uint64(220)
		v4NHGECMPB = uint64(221)
		v6NHGECMPA = uint64(222)
		v6NHGECMPB = uint64(223)
	)

	ecmpBaseEntries := []fluent.GRIBIEntry{
		fluent.NextHopEntry().WithNetworkInstance(testArgs.ni).WithIndex(v4NHPort3).WithIPAddress(atePort3.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(testArgs.ni).WithIndex(v4NHPort4).WithIPAddress(atePort4.IPv4),
		fluent.NextHopEntry().WithNetworkInstance(testArgs.ni).WithIndex(v6NHPort3).WithIPAddress(atePort3.IPv6),
		fluent.NextHopEntry().WithNetworkInstance(testArgs.ni).WithIndex(v6NHPort4).WithIPAddress(atePort4.IPv6),
		fluent.NextHopGroupEntry().WithNetworkInstance(testArgs.ni).WithID(v4NHGECMPA).AddNextHop(v4NHPort3, 1).AddNextHop(v4NHPort4, 1),
		fluent.NextHopGroupEntry().WithNetworkInstance(testArgs.ni).WithID(v4NHGECMPB).AddNextHop(v4NHPort4, 1).AddNextHop(v4NHPort3, 1),
		fluent.NextHopGroupEntry().WithNetworkInstance(testArgs.ni).WithID(v6NHGECMPA).AddNextHop(v6NHPort3, 1).AddNextHop(v6NHPort4, 1),
		fluent.NextHopGroupEntry().WithNetworkInstance(testArgs.ni).WithID(v6NHGECMPB).AddNextHop(v6NHPort4, 1).AddNextHop(v6NHPort3, 1),
	}
	testArgs.gribiC.AddEntries(t, ecmpBaseEntries, nil)

	for start := 0; start < routeCount; start += batchSize {
		end := start + batchSize
		if end > routeCount {
			end = routeCount
		}
		entries := make([]fluent.GRIBIEntry, 0, (end-start)*2)
		for i := start; i < end; i++ {
			v4NHG := v4NHGECMPA
			v6NHG := v6NHGECMPA
			if i%2 == 1 {
				v4NHG = v4NHGECMPB
				v6NHG = v6NHGECMPB
			}
			entries = append(entries,
				fluent.IPv4Entry().WithNetworkInstance(testArgs.ni).WithPrefix(v4Prefixes[i]).WithNextHopGroup(v4NHG),
				fluent.IPv6Entry().WithNetworkInstance(testArgs.ni).WithPrefix(v6Prefixes[i]).WithNextHopGroup(v6NHG),
			)
		}
		for _, action := range actions {
			switch action {
			case "add", "replace":
				testArgs.gribiC.AddEntries(t, entries, nil)
			case "delete":
				testArgs.gribiC.DeleteEntries(t, entries, nil)
			default:
				return fmt.Errorf("unsupported gRIBI action %q", action)
			}
		}
	}

	return nil
}

func waitForConcurrentAFTRouteCount(t *testing.T, dut *ondatra.DUTDevice, ni string, want int, expectedPrefixes []string, v4 bool) {
	t.Helper()
	seen := make(map[string]struct{}, want)
	expectedSet := make(map[string]struct{}, len(expectedPrefixes))
	for _, prefix := range expectedPrefixes {
		expectedSet[prefix] = struct{}{}
	}
	if v4 {
		path := gnmi.OC().NetworkInstance(ni).Afts().Ipv4EntryAny().State()
		watch := gnmi.WatchAll(t, dut, path, 10*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
			entry, ok := val.Val()
			if !ok || entry == nil {
				return false
			}
			prefix := entry.GetPrefix()
			if _, ok := expectedSet[prefix]; ok {
				seen[prefix] = struct{}{}
			}
			if len(seen) >= want {
				t.Logf("concurrent IPv4 AFT count reached, got %d, want %d", len(seen), want)
				return true
			}
			return false
		})
		if _, ok := watch.Await(t); !ok {
			t.Errorf("error:concurrent IPv4 AFT count not reached, got %d, want %d", len(seen), want)
		}
		if len(seen) != want {
			t.Errorf("error: concurrent IPv4 AFT exact count mismatch, got %d, want %d", len(seen), want)
		}
	} else {
		path := gnmi.OC().NetworkInstance(ni).Afts().Ipv6EntryAny().State()
		watch := gnmi.WatchAll(t, dut, path, 10*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv6Entry]) bool {
			entry, ok := val.Val()
			if !ok || entry == nil {
				return false
			}
			prefix := entry.GetPrefix()
			if _, ok := expectedSet[prefix]; ok {
				seen[prefix] = struct{}{}
			}
			if len(seen) >= want {
				t.Logf("concurrent IPv6 AFT count reached, got %d, want %d", len(seen), want)
				return true
			}
			return false
		})
		if _, ok := watch.Await(t); !ok {
			t.Errorf("error: concurrent IPv6 AFT count not reached, got %d, want %d", len(seen), want)
		}
		if len(seen) != want {
			t.Errorf("error: concurrent IPv6 AFT exact count mismatch, got %d, want %d", len(seen), want)
		}
	}
}

func configureConcurrentDataPlaneFlows(t *testing.T, testArgs *testArgs, prefixes []string, flowName string, v4 bool) {
	t.Helper()
	routeCount := len(prefixes)
	if routeCount > concurrentRouteCount {
		routeCount = concurrentRouteCount
	}

	flow := testArgs.otgConfig.Flows().Add().SetName(flowName)
	flow.Metrics().SetEnable(true)
	e := flow.Packet().Add().Ethernet()
	e.Src().SetValue(atePort1.MAC)
	if v4 {
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{atePort3.Name + ".IPv4", atePort4.Name + ".IPv4"})
		v4 := flow.Packet().Add().Ipv4()
		v4.Src().SetValue(atePort1.IPv4)
		v4.Dst().Increment().SetStart(strings.TrimSuffix(prefixes[0], "/32")).SetStep("0.0.0.1").SetCount(uint32(routeCount))
		v4.TimeToLive().SetValue(64)
	} else {
		flow.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv6"}).SetRxNames([]string{atePort3.Name + ".IPv6", atePort4.Name + ".IPv6"})
		v6 := flow.Packet().Add().Ipv6()
		v6.Src().SetValue(atePort1.IPv6)
		v6.Dst().Increment().SetStart(strings.TrimSuffix(prefixes[0], "/128")).SetStep("0:0:0:0:0:0:0:1").SetCount(uint32(routeCount))
		v6.HopLimit().SetValue(64)
	}
	flow.Size().SetFixed(frameSize)
	flow.Rate().SetPps(framePps)
	flow.Duration().FixedPackets().SetPackets(uint32(routeCount))
}

func validateConcurrentDataPlaneFlowMetrics(t *testing.T, ate *ondatra.ATEDevice, flowNames []string) {
	t.Helper()
	for _, flowName := range flowNames {
		otgutils.ExpectedTrafficLoss(t, ate.OTG(), flowName, 0, 1)
	}
}

func validateTTLDecrementAndMACRewrite(t *testing.T, ate *ondatra.ATEDevice, dpValidations []dataplaneValidation) {
	t.Helper()
	for _, dpVal := range dpValidations {
		v4DstSet := make(map[string]struct{}, len(dpVal.dstIPv4))
		v6DstSet := make(map[string]struct{}, len(dpVal.dstIPv6))

		wantSrcIPv4 := net.ParseIP(dpVal.srcIPv4)
		wantSrcIPv6 := net.ParseIP(dpVal.srcIPv6)
		if dpVal.srcIPv4 != "" && wantSrcIPv4 == nil {
			t.Fatalf("failed to parse expected IPv4 source %q", dpVal.srcIPv4)
		}
		if dpVal.srcIPv6 != "" && wantSrcIPv6 == nil {
			t.Fatalf("failed to parse expected IPv6 source %q", dpVal.srcIPv6)
		}

		for _, pfx := range dpVal.dstIPv4 {
			ip, _, err := net.ParseCIDR(pfx)
			if err != nil {
				t.Fatalf("failed to parse IPv4 prefix %q: %v", pfx, err)
			}
			ip4 := ip.To4()
			if ip4 == nil {
				t.Fatalf("prefix %q is not IPv4", pfx)
			}
			v4DstSet[ip4.String()] = struct{}{}
		}
		for _, pfx := range dpVal.dstIPv6 {
			ip, _, err := net.ParseCIDR(pfx)
			if err != nil {
				t.Fatalf("failed to parse IPv6 prefix %q: %v", pfx, err)
			}
			ip6 := ip.To16()
			if ip6 == nil || ip.To4() != nil {
				t.Fatalf("prefix %q is not IPv6", pfx)
			}
			v6DstSet[ip6.String()] = struct{}{}
		}

		wantV4 := len(v4DstSet) > 0
		wantV6 := len(v6DstSet) > 0

		packetBytes := ate.OTG().GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(dpVal.capturePort))
		if len(packetBytes) == 0 {
			t.Errorf("error: no packet data captured on port %s", dpVal.capturePort)
			continue
		}

		// Write capture to temporary pcap file for analysis
		f, err := os.CreateTemp("", ".pcap")
		if err != nil {
			t.Fatalf("Could not create temporary pcap file: %v", err)
		}
		defer os.Remove(f.Name())
		if _, err := f.Write(packetBytes); err != nil {
			t.Fatalf("Could not write packetBytes to pcap file: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("Could not close temporary pcap file: %v", err)
		}

		handle, err := pcap.OpenOffline(f.Name())
		if err != nil {
			t.Fatalf("Could not open pcap file: %v", err)
		}
		defer handle.Close()

		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		gotV4 := !wantV4
		gotV6 := !wantV6

		for packet := range packetSource.Packets() {
			eth, ok := packet.Layer(layers.LayerTypeEthernet).(*layers.Ethernet)
			if !ok {
				continue
			}
			if !strings.EqualFold(eth.SrcMAC.String(), dpVal.srcMac) || !strings.EqualFold(eth.DstMAC.String(), dpVal.dstMac) {
				continue
			}

			if wantV4 && !gotV4 {
				if ipL := packet.Layer(layers.LayerTypeIPv4); ipL != nil {
					if ip, ok := ipL.(*layers.IPv4); ok {
						if wantSrcIPv4 != nil && ip.SrcIP.Equal(wantSrcIPv4) {
							if _, found := v4DstSet[ip.DstIP.String()]; found && ip.TTL == dpVal.expectedTTL {
								gotV4 = true
							}
						}
					}
				}
			}

			if wantV6 && !gotV6 {
				if ipL := packet.Layer(layers.LayerTypeIPv6); ipL != nil {
					if ip, ok := ipL.(*layers.IPv6); ok {
						if wantSrcIPv6 != nil && ip.SrcIP.Equal(wantSrcIPv6) {
							if _, found := v6DstSet[ip.DstIP.String()]; found && ip.HopLimit == dpVal.expectedTTL {
								gotV6 = true
							}
						}
					}
				}
			}

			if gotV4 && gotV6 {
				break
			}
		}

		if wantV4 && !gotV4 {
			t.Errorf("error: no IPv4 test packet with TTL=%d and expected MACs on port %s", dpVal.expectedTTL, dpVal.capturePort)
		}
		if wantV6 && !gotV6 {
			t.Errorf("error: no IPv6 test packet with HopLimit=%d and expected MACs on port %s", dpVal.expectedTTL, dpVal.capturePort)
		}
		if gotV4 && gotV6 {
			t.Logf("%s: ttl is decremented by 1 and Source/Destination MAC addresses are rewritten correctly by the router", dpVal.capturePort)
		}
	}
}

func enableCapture(t *testing.T, otg *otg.OTG, topo gosnappi.Config, otgPortNames []string) {
	t.Helper()
	for _, port := range otgPortNames {
		topo.Captures().Add().SetName(port).SetPortNames([]string{port}).SetFormat(gosnappi.CaptureFormat.PCAP)
	}
	otg.PushConfig(t, topo)
}

func startCapture(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	otg.SetControlState(t, cs)
}

func stopCapture(t *testing.T, ate *ondatra.ATEDevice) {
	t.Helper()
	otg := ate.OTG()
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	otg.SetControlState(t, cs)
}

func cleanupP4RT(t *testing.T, leader *p4rt_client.P4RTClient, streamName string) {
	t.Helper()
	if leader != nil {
		leader.StreamChannelDestroy(&streamName)
		leader.ServerDisconnect()
	}
}

func waitPortOperStatus(t *testing.T, dut *ondatra.DUTDevice, port string, want oc.E_Interface_OperStatus) {
	t.Helper()
	path := gnmi.OC().Interface(port).OperStatus().State()
	if got, ok := gnmi.Watch(t, dut, path, 2*time.Minute, func(v *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
		status, present := v.Val()
		return present && status == want
	}).Await(t); !ok {
		t.Fatalf("%s oper-status did not reach %v, got %v", port, want, got)
	}
}

func validateInvalidP4RTEntryRejected(t *testing.T, testArgs *testArgs) {
	t.Helper()
	err := testArgs.leader.Write(&p4v1pb.WriteRequest{
		DeviceId:   deviceID,
		ElectionId: &p4v1pb.Uint128{High: 0, Low: leaderElectionID},
		Updates: []*p4v1pb.Update{{
			Type: p4v1pb.Update_INSERT,
			Entity: &p4v1pb.Entity{Entity: &p4v1pb.Entity_TableEntry{TableEntry: &p4v1pb.TableEntry{
				TableId: 0xFFFFFFFF,
				Action:  &p4v1pb.TableAction{Type: &p4v1pb.TableAction_Action{Action: &p4v1pb.Action{ActionId: 0xFFFFFFFF}}},
			}}},
		}},
		Atomicity: p4v1pb.WriteRequest_CONTINUE_ON_ERROR,
	})
	if err == nil {
		t.Errorf("error: invalid P4RT table entry was accepted unexpectedly")
		return
	}
	t.Logf("invalid P4RT table entry was rejected as expected: %v", err)

	st, _ := status.FromError(err)
	t.Logf("invalid entry rejected: code=%v msg=%q", st.Code(), st.Message())

	switch st.Code() {
	case codes.PermissionDenied, codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		t.Fatalf("got %v, which is a session/transport problem, not an invalid-entry rejection: %v", st.Code(), err)
	}

	okCnt, notOKCnt, details := p4rt_client.P4RTWriteErrParse(err)
	t.Logf("write result: ok=%d not-ok=%d details=%v", okCnt, notOKCnt, details)
	if okCnt != 0 || notOKCnt != 1 {
		t.Errorf("error: want 0 accepted and 1 rejected update, got ok=%d not-ok=%d", okCnt, notOKCnt)
	}
}

func checkPacketInCounts(args *testArgs) uint64 {
	name := streamNameLeader
	if s := args.leader.StreamChannelGet(&name); s != nil {
		return uint64(s.GetPacketCounters().RxPktCntr)
	}
	return 0
}

func captureDataPlanePacket(t *testing.T, testArgs *testArgs, flowNames []string, totalPktCount ...uint64) {
	t.Helper()
	testArgs.ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, testArgs.ate.OTG(), testArgs.otgConfig, "IPv4")
	otgutils.WaitForARP(t, testArgs.ate.OTG(), testArgs.otgConfig, "IPv6")

	startCapture(t, testArgs.ate)
	testArgs.ate.OTG().StartTraffic(t)
	if len(totalPktCount) > 0 {
		waitForFlowPacketCountWatch(t, testArgs.ate, flowNames, 10*time.Minute, totalPktCount[0])
	} else {
		waitForFlowPacketCountWatch(t, testArgs.ate, flowNames, 10*time.Minute, 0)
	}

	testArgs.ate.OTG().StopTraffic(t)
	stopCapture(t, testArgs.ate)

	otgutils.LogFlowMetrics(t, testArgs.ate.OTG(), testArgs.otgConfig)
}

// toggleInterface toggles the interface.
func toggleInterface(t *testing.T, dut *ondatra.DUTDevice, intf string, isEnabled bool) {
	t.Helper()
	d := &oc.Root{}
	i := d.GetOrCreateInterface(intf)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	i.Enabled = ygot.Bool(isEnabled)
	gnmi.Update(t, dut, gnmi.OC().Interface(intf).Config(), i)
}

func packetInDelta(testArgs *testArgs, window time.Duration) uint64 {
	before := checkPacketInCounts(testArgs)
	time.Sleep(window)
	return checkPacketInCounts(testArgs) - before
}

func TestP4RTPacketInGRIBIInteraction(t *testing.T) {
	args := &testArgs{}
	args.dut = ondatra.DUT(t, "dut")
	args.ate = ondatra.ATE(t, "ate")
	args.ni = deviations.DefaultNetworkInstance(args.dut)
	args.packetIO = []packetIO{getLLDPBaselinePacketIO()}

	var err error
	args.v4Prefixes, args.v6Prefixes, err = generateConcurrentPrefixes(concurrentRouteCount)
	if err != nil {
		t.Errorf("failed to generate concurrent prefixes: %v", err)
	}

	t.Log("configure dut interfaces and network instance")
	mustConfigureDUT(t, args.dut)

	t.Log("configure otg")
	args.otgConfig = configureATE(t, args.ate)
	args.ate.OTG().PushConfig(t, args.otgConfig)

	t.Log("start otg protocols")
	args.ate.OTG().StartProtocols(t)

	otgutils.WaitForARP(t, args.ate.OTG(), args.otgConfig, "IPv4")
	otgutils.WaitForARP(t, args.ate.OTG(), args.otgConfig, "IPv6")

	t.Log("validate interface operational state")
	upIntfs := helpers.FetchOperStatusUPIntfs(t, args.dut, true)
	helpers.ValidateOperStatusUPIntfs(t, args.dut, upIntfs, 2*time.Minute)

	t.Log("configure p4rt node-id")
	configureP4RTNode(t, args.dut)

	t.Log("establish p4rt leader sessions")
	args.leader = p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := args.leader.P4rtClientSet(args.dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("could not initialize p4rt leader client: %v", err)
	}

	t.Cleanup(func() {
		cleanupP4RT(t, args.leader, streamNameLeader)
	})

	if err := setupP4RTClient(t, args.leader, streamNameLeader, leaderElectionID); err != nil {
		t.Fatalf("could not setup p4rt leader client: %v", err)
	}

	if err := setPipeline(t, args.leader, leaderElectionID); err != nil {
		t.Fatalf("could not set pipeline with leader: %v", err)
	}

	t.Log("establish gribi client")
	args.gribiC = setupGRIBI(t, args.dut)
	t.Cleanup(func() {
		if args.gribiC != nil {
			args.gribiC.FlushAll(t)
			args.gribiC.Close(t)
		}
	})

	type testCase struct {
		Name                string
		Description         string
		flowSpecs           []flowSpec
		dataplaneValidation []dataplaneValidation
		testFunc            func(t *testing.T, testArgs *testArgs, flows []flowSpec, dpVal []dataplaneValidation)
	}

	testCases := []testCase{
		{
			Name: "3.4.1_BaselineGRIBIForwardingAndPacketIn",
			flowSpecs: []flowSpec{
				{
					flowName:   "lldp-flow",
					frameSize:  frameSize,
					framePps:   framePps,
					frameCount: frameCount,
					lldpFlow:   true,
				},
				{
					flowName:   "baselineFlow",
					frameSize:  frameSize,
					framePps:   framePps,
					frameCount: frameCount,
					isIPv6:     false,
					dstIP:      strings.Split(destinationPrefixv4, "/")[0],
					srcIP:      atePort1.IPv4,
					srcMAC:     atePort1.MAC,
				},
				{
					flowName:   "baselineFlow-v6",
					frameSize:  frameSize,
					framePps:   framePps,
					frameCount: frameCount,
					isIPv6:     true,
					dstIP:      strings.Split(destinationPrefixv6, "/")[0],
					srcIP:      atePort1.IPv6,
					srcMAC:     atePort1.MAC,
				},
			},
			dataplaneValidation: []dataplaneValidation{
				{
					srcMac:      dutPort2.MAC,
					dstMac:      atePort2.MAC,
					expectedTTL: 63,
					srcIPv4:     atePort1.IPv4,
					dstIPv4:     []string{destinationPrefixv4},
					srcIPv6:     atePort1.IPv6,
					dstIPv6:     []string{destinationPrefixv6},
					capturePort: "port2",
				},
			},
			testFunc: testBaselinePacketIn,
		},
		{
			Name: "3.4.2_ConcurrentGRIBIAndPacketIn",
			flowSpecs: []flowSpec{
				{
					flowName:   "concurrent-lldp",
					frameSize:  frameSize,
					framePps:   framePps,
					frameCount: 0,
					lldpFlow:   true,
				},
				{
					flowName:   "traceRoute",
					frameSize:  frameSize,
					framePps:   framePps,
					frameCount: 0,
					traceRoute: true,
				},
			},
			dataplaneValidation: []dataplaneValidation{
				{
					srcMac:      dutPort3.MAC,
					dstMac:      atePort3.MAC,
					expectedTTL: 63,
					srcIPv4:     atePort1.IPv4,
					dstIPv4:     args.v4Prefixes,
					srcIPv6:     atePort1.IPv6,
					dstIPv6:     args.v6Prefixes,
					capturePort: "port3",
				},
				{
					srcMac:      dutPort4.MAC,
					dstMac:      atePort4.MAC,
					expectedTTL: 63,
					srcIPv4:     atePort1.IPv4,
					dstIPv4:     args.v4Prefixes,
					srcIPv6:     atePort1.IPv6,
					dstIPv6:     args.v6Prefixes,
					capturePort: "port4",
				},
			},
			testFunc: testConcurrentPacketIn,
		},
		{
			Name:     "3.4.3_NegativeScenariosUnderConcurrentLoad",
			testFunc: testNegativeConcurrentScenario,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			tc.testFunc(t, args, tc.flowSpecs, tc.dataplaneValidation)
		})
	}
}

func testBaselinePacketIn(t *testing.T, testArgs *testArgs, flows []flowSpec, dpVal []dataplaneValidation) {
	t.Log("program baseline gribi route")
	programentry := programBaselineRoute(t, testArgs.dut, nhv4Index, nhgv4Index, atePort2.IPv4, destinationPrefixv4, "v4")
	testArgs.gribiC.AddEntries(t, programentry, nil)
	programentry = programBaselineRoute(t, testArgs.dut, nhv6Index, nhgv6Index, atePort2.IPv6, destinationPrefixv6, "v6")
	testArgs.gribiC.AddEntries(t, programentry, nil)

	for _, prefix := range []string{destinationPrefixv4, destinationPrefixv6} {
		verifyAFTPrefix(t, testArgs.dut, testArgs.ni, prefix)
	}

	// Register cleanup to delete wbb acl entry on the device
	t.Cleanup(func() {
		if err := programPuntTableEntries(t, testArgs.leader, testArgs.packetIO[0], true); err != nil {
			t.Errorf("error when deleting entry: %v", err)
		}
	})

	// Insert wbb acl entry on the DUT
	if err := programPuntTableEntries(t, testArgs.leader, testArgs.packetIO[0], false); err != nil {
		t.Fatalf("error when programming entry: %v", err)
	}

	t.Log("send baseline traffic and validate p4rt packet-in")
	configureTrafficFlow(t, testArgs, []flowSpec{flows[0]}, testArgs.packetIO[0])
	pktOut := validateTrafficFlow(t, testArgs, []string{flows[0].flowName}, frameCount, trafficTimeout)

	// validate pkts received on DUT
	validateDUTInterfaceCounters(t, testArgs.dut, "port1", frameCount, true)
	validatePacketIn(t, testArgs, pktOut, testArgs.packetIO[0])

	// configure baseline data plane traffic
	configureTrafficFlow(t, testArgs, flows[1:], testArgs.packetIO[0])

	// enable packet capture on the ATE
	enableCapture(t, testArgs.ate.OTG(), testArgs.otgConfig, []string{dpVal[0].capturePort})
	captureDataPlanePacket(t, testArgs, []string{flows[1].flowName, flows[2].flowName}, frameCount)

	validateConcurrentDataPlaneFlowMetrics(t, testArgs.ate, []string{flows[1].flowName, flows[2].flowName})

	// validate pkts received on DUT on egress port
	validateDUTInterfaceCounters(t, testArgs.dut, "port2", frameCount*2, false)
	validateTTLDecrementAndMACRewrite(t, testArgs.ate, dpVal)
}

func testConcurrentPacketIn(t *testing.T, testArgs *testArgs, flows []flowSpec, dpVal []dataplaneValidation) {
	// LLDP and IPv4 traceroute continous traffic and validate packet-in messages
	testArgs.packetIO = append(testArgs.packetIO, getTracerouteParameter(t, testArgs))
	for i, flow := range flows {
		configureTrafficFlow(t, testArgs, []flowSpec{flow}, testArgs.packetIO[i])
	}

	pktOut := validateTrafficFlow(t, testArgs, []string{flows[0].flowName, flows[1].flowName}, 0, 20*time.Second)

	// configure IPv4 and IPv6 routes
	if err := programConcurrentGRIBIRouteLifecycle(t, testArgs, testArgs.v4Prefixes, testArgs.v6Prefixes, concurrentBatchSize, []string{"add", "delete", "replace"}); err != nil {
		t.Errorf("failed to program concurrent gRIBI route lifecycle: %v", err)
	}

	verifyAFTPrefix(t, testArgs.dut, testArgs.ni, testArgs.v4Prefixes[0])
	verifyAFTPrefix(t, testArgs.dut, testArgs.ni, testArgs.v4Prefixes[concurrentRouteCount-1])
	verifyAFTPrefix(t, testArgs.dut, testArgs.ni, testArgs.v6Prefixes[0])
	verifyAFTPrefix(t, testArgs.dut, testArgs.ni, testArgs.v6Prefixes[concurrentRouteCount-1])

	// validate total routes installed in the AFT
	waitForConcurrentAFTRouteCount(t, testArgs.dut, testArgs.ni, concurrentRouteCount, testArgs.v4Prefixes, true)
	waitForConcurrentAFTRouteCount(t, testArgs.dut, testArgs.ni, concurrentRouteCount, testArgs.v6Prefixes, false)

	testArgs.ate.OTG().StopTraffic(t)
	validatePacketIn(t, testArgs, pktOut, testArgs.packetIO[0], testArgs.packetIO[1])
	// configure concurrent data plane flows for IPv4 and IPv6 prefixes
	configureConcurrentDataPlaneFlows(t, testArgs, testArgs.v4Prefixes, "concurrent-dp-v4-scale", true)
	configureConcurrentDataPlaneFlows(t, testArgs, testArgs.v6Prefixes, "concurrent-dp-v6-scale", false)

	// enable capture on all OTG ports
	enableCapture(t, testArgs.ate.OTG(), testArgs.otgConfig, []string{dpVal[0].capturePort, dpVal[1].capturePort})
	captureDataPlanePacket(t, testArgs, []string{"concurrent-dp-v4-scale", "concurrent-dp-v6-scale"}, concurrentRouteCount)

	validateConcurrentDataPlaneFlowMetrics(t, testArgs.ate, []string{"concurrent-dp-v4-scale", "concurrent-dp-v6-scale"})
	validateTTLDecrementAndMACRewrite(t, testArgs.ate, dpVal)
}

func testNegativeConcurrentScenario(t *testing.T, testArgs *testArgs, flows []flowSpec, dpVal []dataplaneValidation) {
	leaderStream := streamNameLeader

	otgutils.WaitForARP(t, testArgs.ate.OTG(), testArgs.otgConfig, "IPv4")
	otgutils.WaitForARP(t, testArgs.ate.OTG(), testArgs.otgConfig, "IPv6")
	testArgs.ate.OTG().StartTraffic(t)

	t.Log("validating that an invalid P4RT entry is rejected while gRIBI remains operational")
	validateInvalidP4RTEntryRejected(t, testArgs)

	verifyAFTPrefix(t, testArgs.dut, testArgs.ni, testArgs.v4Prefixes[0])
	verifyAFTPrefix(t, testArgs.dut, testArgs.ni, testArgs.v4Prefixes[concurrentRouteCount-1])
	verifyAFTPrefix(t, testArgs.dut, testArgs.ni, testArgs.v6Prefixes[0])
	verifyAFTPrefix(t, testArgs.dut, testArgs.ni, testArgs.v6Prefixes[concurrentRouteCount-1])

	// validate total routes installed in the AFT
	waitForConcurrentAFTRouteCount(t, testArgs.dut, testArgs.ni, concurrentRouteCount, testArgs.v4Prefixes, true)
	waitForConcurrentAFTRouteCount(t, testArgs.dut, testArgs.ni, concurrentRouteCount, testArgs.v6Prefixes, false)

	t.Log("validating that ingress port down suppresses PacketIn while gRIBI continues")
	port1Name := testArgs.dut.Port(t, "port1").Name()
	t.Cleanup(func() {
		toggleInterface(t, testArgs.dut, port1Name, true)
		waitPortOperStatus(t, testArgs.dut, port1Name, oc.Interface_OperStatus_UP)
	})
	toggleInterface(t, testArgs.dut, port1Name, false)
	waitPortOperStatus(t, testArgs.dut, port1Name, oc.Interface_OperStatus_DOWN)

	time.Sleep(10 * time.Second)
	if d := packetInDelta(testArgs, 15*time.Second); d != 0 {
		t.Errorf("PacketIn total grew by %d while ingress port is down, want 0", d)
	} else {
		t.Log("PacketIn total unchanged while ingress port is down, as expected")
	}

	verifyAFTPrefix(t, testArgs.dut, testArgs.ni, testArgs.v4Prefixes[0])
	verifyAFTPrefix(t, testArgs.dut, testArgs.ni, testArgs.v4Prefixes[concurrentRouteCount-1])
	verifyAFTPrefix(t, testArgs.dut, testArgs.ni, testArgs.v6Prefixes[0])
	verifyAFTPrefix(t, testArgs.dut, testArgs.ni, testArgs.v6Prefixes[concurrentRouteCount-1])

	toggleInterface(t, testArgs.dut, port1Name, true)
	waitPortOperStatus(t, testArgs.dut, port1Name, oc.Interface_OperStatus_UP)

	testArgs.ate.OTG().StartTraffic(t)

	t.Log("validating that gRIBI session drop does not break PacketIn")
	testArgs.gribiC.Close(t)

	if testArgs.leader.StreamChannelGet(&leaderStream) == nil {
		t.Fatalf("leader P4RT stream was lost after the gRIBI session drop")
	}

	if got := checkPacketInCounts(testArgs); got == 0 {
		t.Errorf("error: PacketIn stopped after the gRIBI session drop")
	} else {
		t.Logf("PacketIn continued after gRIBI drop: %d packets", got)
	}

	testArgs.gribiC = setupGRIBI(t, testArgs.dut)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(2 * time.Second)
		// Drop the leader P4RT session while programming is in flight.
		testArgs.leader.StreamChannelDestroy(&leaderStream)
		testArgs.leader.ServerDisconnect()
	}()

	if err := programConcurrentGRIBIRouteLifecycle(t, testArgs, testArgs.v4Prefixes, testArgs.v6Prefixes, concurrentBatchSize, []string{"add"}); err != nil {
		t.Errorf("failed to program concurrent gRIBI routes during P4RT drop: %v", err)
	}

	// gRIBI must finish uninterrupted.
	wg.Wait()

	// gRIBI must finish uninterrupted.
	waitForConcurrentAFTRouteCount(t, testArgs.dut, testArgs.ni, concurrentRouteCount, testArgs.v4Prefixes, true)
	waitForConcurrentAFTRouteCount(t, testArgs.dut, testArgs.ni, concurrentRouteCount, testArgs.v6Prefixes, false)
}
