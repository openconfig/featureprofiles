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

package ingress_police_nhg_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/client"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

const (
	vlanA           = 100
	vlanB           = 200
	ipv4PrefixLen   = 30
	ipv6PrefixLen   = 126
	trafficDuration = 15 * time.Second
	queue1          = "QUEUE_1"
	queue2          = "QUEUE_2"
	classifierType  = oc.Qos_Classifier_Type_IPV4
	schedulerNameA  = "group_A"
	schedulerNameB  = "group_B"
	targetClass     = "class-default"
	cirValue1       = 1000000000
	cirValue2       = 2000000000
	// MPLS-in-UDP test configuration
	mplsLabel1       = uint64(1000)
	mplsLabel2       = uint64(2000)
	outerIPv6Src     = "2001:db8::1"
	outerIPv6Dst1    = "2001:db8::100"
	outerIPv6Dst2    = "2001:db8::200"
	innerIPv6Prefix1 = "2001:db8:1::/64"
	innerIPv6Prefix2 = "2001:db8:2::/64"
	outerDstUDPPort  = uint16(6635) // RFC 7510 standard MPLS-in-UDP port
	// gRIBI entry IDs for MPLS-in-UDP
	mplsNHID1  = uint64(1001)
	mplsNHGID1 = uint64(2001)
	mplsNHID2  = uint64(1002)
	mplsNHGID2 = uint64(2002)
	// Static ARP configuration
	magicIP       = "192.168.1.1"
	magicMac      = "02:00:00:00:00:01"
	lossVariation = 0.01
)

var (
	atePort1Vlan1 = attrs.Attributes{Name: "ateP1VLan1", MAC: "02:00:01:01:01:01", IPv4: "192.0.2.2", IPv6: "2001:db8::2", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	atePort1Vlan2 = attrs.Attributes{Name: "ateP1Vlan2", MAC: "02:00:01:01:01:02", IPv4: "192.0.2.6", IPv6: "2001:db8::6", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}

	atePort2      = attrs.Attributes{Name: "ateP2", MAC: "02:00:02:01:01:01", IPv4: "192.0.2.10", IPv6: "2001:db8::10", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	dutPort1Vlan1 = attrs.Attributes{Desc: "dutPort1Vlan1", MAC: "02:02:01:00:00:01", IPv6: "2001:db8::1", IPv4: "192.0.2.1", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	dutPort1Vlan2 = attrs.Attributes{Desc: "dutPort1Vlan2", MAC: "02:02:01:00:00:02", IPv6: "2001:db8::5", IPv4: "192.0.2.5", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}

	dutPort2        = &attrs.Attributes{Desc: "dutPort2", MAC: "02:02:02:00:00:01", IPv6: "2001:db8::9", IPv4: "192.0.2.9", IPv4Len: ipv4PrefixLen, IPv6Len: ipv6PrefixLen}
	dutPort2DummyIP = attrs.Attributes{Desc: "dutPort2", IPv4Sec: "192.0.2.21", IPv4LenSec: ipv4PrefixLen}
	otgPort2DummyIP = attrs.Attributes{Desc: "otgPort2", IPv4: "192.0.2.22", IPv4Len: ipv4PrefixLen}

	otgDstPort         = "port2"
	otgSrcPort         = "port1"
	inputInterfaceName []string
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

type testCase struct {
	name     string
	gbpsRate uint32
	queue    []string
}

func TestMPLSOverUDPTunnelHashing(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	// Configure DUT interfaces.
	ConfigureDUTIntf(t, dut)

	// configure ATE
	topo := configureATE(t)
	ate.OTG().PushConfig(t, topo)

	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv6")

	// Configure Qos Classifier
	t.Log("Configure QoS policies on DUT")
	configureQoS(t, dut)

	configureGribi(t, dut)

	t.Run("Validate that flow experiences 0 packet loss at 0.7Gbps", func(t *testing.T) {
		validateFlowRate(t, ate, topo, 1, 0)
	})
	t.Run("Validate that flow experiences ~50% packet loss at 2Gbps", func(t *testing.T) {
		validateFlowRate(t, ate, topo, 2, 0.5)
	})

	// t.Run("DP-2.2.3 IPv6 flow label validiation", func(t *testing.T) {
	// 	validateFlowRate(t, ate, topo,)
	// })

}

func ConfigureDUTIntf(t *testing.T, dut *ondatra.DUTDevice) {
	d := gnmi.OC()
	p1 := dut.Port(t, "port1")
	p2 := dut.Port(t, "port2")
	b := new(gnmi.SetBatch)

	// Configure DUT Port 1 with VLAN subinterfaces
	createDUTSubinterface(t, b, new(oc.Root), dut, p1, vlanA, vlanA, dutPort1Vlan1.IPv4, dutPort1Vlan1.IPv6)
	createDUTSubinterface(t, b, new(oc.Root), dut, p1, vlanB, vlanB, dutPort1Vlan2.IPv4, dutPort1Vlan2.IPv6)
	b.Set(t, dut)

	gnmi.Replace(t, dut, d.Interface(p2.Name()).Config(), configInterfaceDUT(p2, dutPort2, dut))

	// Configure Network instance type on DUT
	t.Log("Configure/update Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

}

func configureQoS(t *testing.T, dut *ondatra.DUTDevice) {
	queues := []string{queue1, queue2}
	qos := new(oc.Qos)
	inputInterfaceNameA := dut.Port(t, "port1").Name() + "." + strconv.Itoa(vlanA)
	inputInterfaceNameB := dut.Port(t, "port1").Name() + "." + strconv.Itoa(vlanB)
	cfgplugins.CreateQueues(t, dut, qos, queues)
	qosBatch := &gnmi.SetBatch{}

	// Define forwarding groups
	forwardingGroups := []cfgplugins.ForwardingGroup{
		{
			Desc:        "input_dest_A",
			QueueName:   queue1,
			TargetGroup: "target_input_dest_A",
		},
		{
			Desc:        "input_dest_B",
			QueueName:   queue2,
			TargetGroup: "target_input_dest_B",
		},
	}

	cfgplugins.NewQoSForwardingGroup(t, dut, qos, forwardingGroups)

	qosConfigs := []struct {
		classifier    cfgplugins.QosClassifier
		scheduler     *cfgplugins.SchedulerParams
		interfaceName string
	}{
		{
			classifier: cfgplugins.QosClassifier{
				Desc:        "match_1_dest_A1",
				Name:        schedulerNameA,
				ClassType:   classifierType,
				TermID:      targetClass,
				TargetGroup: "target_input_dest_A",
			},
			scheduler: &cfgplugins.SchedulerParams{
				SchedulerName:  schedulerNameA,
				PolicerName:    "limit_group_A_1Gb",
				InterfaceName:  inputInterfaceNameA,
				ClassName:      targetClass,
				CirValue:       cirValue1,
				BurstSize:      1000,
				QueueName:      queue1,
				QueueID:        1,
				SequenceNumber: 1,
			},
			interfaceName: inputInterfaceNameA,
		},
		{
			classifier: cfgplugins.QosClassifier{
				Desc:        "match_1_dest_B1",
				Name:        schedulerNameB,
				ClassType:   classifierType,
				TermID:      targetClass,
				TargetGroup: "target_input_dest_B",
			},
			scheduler: &cfgplugins.SchedulerParams{
				SchedulerName:  schedulerNameB,
				PolicerName:    "limit_group_B_2Gb",
				InterfaceName:  inputInterfaceNameB,
				ClassName:      targetClass,
				CirValue:       cirValue2,
				BurstSize:      2000,
				QueueName:      queue2,
				QueueID:        2,
				SequenceNumber: 2,
			},
			interfaceName: inputInterfaceNameB,
		},
	}

	qosPath := gnmi.OC().Qos().Config()

	// Loop through both A and B configurations
	for _, cfg := range qosConfigs {
		cfgplugins.NewQoSClassifierConfiguration(t, dut, qos, []cfgplugins.QosClassifier{cfg.classifier})
		cfgplugins.NewOneRateTwoColorScheduler(t, dut, qosBatch, cfg.scheduler)
		cfgplugins.ApplyQosPolicyOnInterface(t, dut, qosBatch, cfg.scheduler)
		gnmi.BatchUpdate(qosBatch, qosPath, qos)
		qosBatch.Set(t, dut)
	}
}

func createDUTSubinterface(t *testing.T, vrfBatch *gnmi.SetBatch, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port, index uint32, vlanID uint16, ipv4Addr, ipv6Addr string) {
	t.Helper()

	ifName := dutPort.Name()
	i := d.GetOrCreateInterface(dutPort.Name())
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	// Always create subif 0
	subif := i.GetOrCreateSubinterface(0)
	subif.Index = ygot.Uint32(0)
	iv4 := subif.GetOrCreateIpv4()
	iv6 := subif.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		iv4.Enabled = ygot.Bool(true)
		iv6.Enabled = ygot.Bool(true)
	}
	gnmi.BatchUpdate(vrfBatch, gnmi.OC().Interface(ifName).Subinterface(0).Config(), subif)

	s := i.GetOrCreateSubinterface(index)

	if vlanID != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
		}
	}
	s4 := s.GetOrCreateIpv4()
	a4 := s4.GetOrCreateAddress(ipv4Addr)
	a4.PrefixLength = ygot.Uint8(uint8(ipv4PrefixLen))
	if deviations.InterfaceEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	s6 := s.GetOrCreateIpv6()
	a6 := s6.GetOrCreateAddress(ipv6Addr)
	a6.PrefixLength = ygot.Uint8(uint8(ipv6PrefixLen))
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	gnmi.BatchUpdate(vrfBatch, gnmi.OC().Interface(ifName).Subinterface(index).Config(), s)
}

// Configures the given DUT interface.
func configInterfaceDUT(p *ondatra.Port, a *attrs.Attributes, dut *ondatra.DUTDevice) *oc.Interface {
	i := a.NewOCInterface(p.Name(), dut)
	i.GetOrCreateSubinterface(0).GetOrCreateIpv4()
	i.GetOrCreateSubinterface(0).GetOrCreateIpv6()
	return i
}

// configureATE sets up the ATE interfaces and BGP configurations.
func configureATE(t *testing.T) gosnappi.Config {
	topo := gosnappi.NewConfig()
	t.Log("Configure ATE interface")
	port1 := topo.Ports().Add().SetName("port1")
	port2 := topo.Ports().Add().SetName("port2")

	port1Vlan1Dev := topo.Devices().Add().SetName(atePort1Vlan1.Name + ".dev")
	port1Vlan1Eth := port1Vlan1Dev.Ethernets().Add().SetName(atePort1Vlan1.Name + ".Eth").SetMac(atePort1Vlan1.MAC)
	port1Vlan1Eth.Connection().SetPortName(port1.Name())
	port1Vlan1Eth.Vlans().Add().SetName("vlanA").SetId(uint32(vlanA))
	port1Vlan1Ipv4 := port1Vlan1Eth.Ipv4Addresses().Add().SetName(atePort1Vlan1.Name + ".IPv4")
	port1Vlan1Ipv4.SetAddress(atePort1Vlan1.IPv4).SetGateway(dutPort1Vlan1.IPv4).SetPrefix(uint32(atePort1Vlan1.IPv4Len))
	port1Vlan1Ipv6 := port1Vlan1Eth.Ipv6Addresses().Add().SetName(atePort1Vlan1.Name + ".IPv6")
	port1Vlan1Ipv6.SetAddress(atePort1Vlan1.IPv6).SetGateway(dutPort1Vlan1.IPv6).SetPrefix(uint32(atePort1Vlan1.IPv6Len))

	port1Vlan2Dev := topo.Devices().Add().SetName(atePort1Vlan2.Name + ".dev")
	port1Vlan2Eth := port1Vlan2Dev.Ethernets().Add().SetName(atePort1Vlan2.Name + ".Eth").SetMac(atePort1Vlan2.MAC)
	port1Vlan2Eth.Connection().SetPortName(port1.Name())
	port1Vlan2Eth.Vlans().Add().SetName("vlanB").SetId(uint32(vlanB))
	port1Vlan2Ipv4 := port1Vlan2Eth.Ipv4Addresses().Add().SetName(atePort1Vlan2.Name + ".IPv4")
	port1Vlan2Ipv4.SetAddress(atePort1Vlan2.IPv4).SetGateway(dutPort1Vlan2.IPv4).SetPrefix(uint32(atePort1Vlan2.IPv4Len))
	port1Vlan2Ipv6 := port1Vlan2Eth.Ipv6Addresses().Add().SetName(atePort1Vlan2.Name + ".IPv6")
	port1Vlan2Ipv6.SetAddress(atePort1Vlan2.IPv6).SetGateway(dutPort1Vlan2.IPv6).SetPrefix(uint32(atePort1Vlan2.IPv6Len))

	port2Dev := topo.Devices().Add().SetName(atePort2.Name + ".dev")
	port2Eth := port2Dev.Ethernets().Add().SetName(atePort2.Name + ".Eth").SetMac(atePort2.MAC)
	port2Eth.Connection().SetPortName(port2.Name())
	port2Ipv4 := port2Eth.Ipv4Addresses().Add().SetName(atePort2.Name + ".IPv4")
	port2Ipv4.SetAddress(atePort2.IPv4).SetGateway(dutPort2.IPv4).SetPrefix(uint32(atePort2.IPv4Len))
	port2Ipv6 := port2Eth.Ipv6Addresses().Add().SetName(atePort2.Name + ".IPv6")
	port2Ipv6.SetAddress(atePort2.IPv6).SetGateway(dutPort2.IPv6).SetPrefix(uint32(atePort2.IPv6Len))

	return topo
}

func configureGribi(t *testing.T, dut *ondatra.DUTDevice) {
	ctx := context.Background()
	// Configure gRIBI client
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

	// Flush all existing AFT entries and set up basic routing infrastructure
	c.FlushAll(t)
	programBasicEntries(t, dut, &c)

	// Verify basic infrastructure is properly installed
	if err := c.AwaitTimeout(ctx, t, 3*time.Minute); err != nil {
		t.Fatalf("Failed to install basic infrastructure entries: %v", err)
	}

	t.Log("Adding MPLS-in-UDP entries")

	mplsLabels := []uint64{mplsLabel1, mplsLabel2}
	mplsNHIDs := []uint64{mplsNHID1, mplsNHID2}
	mplsNHGIDs := []uint64{mplsNHGID1, mplsNHGID2}
	outerIPv6Dsts := []string{outerIPv6Dst1, outerIPv6Dst2}
	innerIPv6Dsts := []string{innerIPv6Prefix1, innerIPv6Prefix2}

	var entries []fluent.GRIBIEntry
	var wantAddResults []*client.OpResult

	for i := range mplsLabels {
		label := mplsLabels[i]
		nhID := mplsNHIDs[i]
		nhgID := mplsNHGIDs[i]
		outerIPv6Dst := outerIPv6Dsts[i]
		innerIPv6Dst := innerIPv6Dsts[i]
		t.Logf("Programming MPLS-in-UDP encapsulation #%d: Label=%d, NH=%d, NHG=%d", i+1, label, nhID, nhgID)

		// --- Create MPLS-in-UDP encapsulation NextHop ---
		entries = append(entries,
			fluent.NextHopEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithIndex(nhID).
				AddEncapHeader(
					fluent.MPLSEncapHeader().WithLabels(label),
					fluent.UDPV6EncapHeader().
						WithSrcIP(outerIPv6Src).
						WithDstIP(outerIPv6Dst).
						WithDstUDPPort(uint64(outerDstUDPPort)),
				),
		)

		// --- Create NHG pointing to the above NextHop ---
		entries = append(entries,
			fluent.NextHopGroupEntry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithID(nhgID).
				AddNextHop(nhID, uint64(i+1)),
		)

		// --- Create IPv6 route triggering MPLS-in-UDP encapsulation ---
		entries = append(entries,
			fluent.IPv6Entry().
				WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
				WithPrefix(innerIPv6Dst).
				WithNextHopGroup(nhgID).
				WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		)

		// --- Expected results ---
		wantAddResults = append(wantAddResults,
			fluent.OperationResult().
				WithNextHopOperation(nhID).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			fluent.OperationResult().
				WithNextHopGroupOperation(nhgID).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
			fluent.OperationResult().
				WithIPv6Operation(innerIPv6Dst).
				WithProgrammingResult(fluent.InstalledInRIB).
				WithOperationType(constants.Add).
				AsResult(),
		)

	}
	c.AddEntries(t, entries, wantAddResults)
}

func programBasicEntries(t *testing.T, dut *ondatra.DUTDevice, c *gribi.Client) {

	// Set up static ARP configuration for gRIBI NH entries
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		b := &gnmi.SetBatch{}
		cfg := cfgplugins.SecondaryIPConfig{
			Entries: []cfgplugins.SecondaryIPEntry{
				{PortName: "port2", PortDummyAttr: dutPort2DummyIP, DummyIP: otgPort2DummyIP.IPv4, MagicMAC: magicMac},
			},
		}
		sb := cfgplugins.StaticARPWithSecondaryIP(t, dut, b, cfg)
		sb.Set(t, dut)
	} else if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		b := &gnmi.SetBatch{}
		cfg := cfgplugins.StaticARPConfig{
			Entries: []cfgplugins.StaticARPEntry{
				{PortName: "port2", MagicIP: magicIP, MagicMAC: magicMac},
			},
		}
		sb := cfgplugins.StaticARPWithMagicUniversalIP(t, dut, b, cfg)
		sb.Set(t, dut)
	}

	// Allow time for configuration to be applied
	time.Sleep(10 * time.Second)

	t.Log("Setting up basic routing infrastructure for MPLS-in-UDP (looped two NH/NHG entries on same port)")

	p2 := dut.Port(t, "port2")

	// Define NH/NHG ID pairs
	type entryPair struct {
		nhID  uint64
		nhgID uint64
		route string
	}
	pairs := []entryPair{
		{nhID: 101, nhgID: 201, route: outerIPv6Dst1},
		{nhID: 102, nhgID: 202, route: outerIPv6Dst2},
	}

	for i, pair := range pairs {
		t.Logf("Programming NH %d and NHG %d for port %s", pair.nhID, pair.nhgID, p2.Name())
		i = i + 1
		switch {
		case deviations.GRIBIMACOverrideWithStaticARP(dut):
			t.Log("Using GRIBIMACOverrideWithStaticARP deviation")
			c.AddNH(t, pair.nhID, "MACwithIp", deviations.DefaultNetworkInstance(dut),
				fluent.InstalledInFIB, &gribi.NHOptions{Dest: otgPort2DummyIP.IPv4, Mac: magicMac})
			c.AddNHG(t, pair.nhgID, map[uint64]uint64{pair.nhID: uint64(i)},
				deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)

		case deviations.GRIBIMACOverrideStaticARPStaticRoute(dut):
			t.Log("Using GRIBIMACOverrideStaticARPStaticRoute deviation")
			nh, op1 := gribi.NHEntry(pair.nhID, "MACwithInterface", deviations.DefaultNetworkInstance(dut),
				fluent.InstalledInFIB, &gribi.NHOptions{Interface: p2.Name(), Mac: magicMac, Dest: magicIP})
			nhg, op2 := gribi.NHGEntry(pair.nhgID, map[uint64]uint64{pair.nhID: uint64(i)},
				deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
			c.AddEntries(t, []fluent.GRIBIEntry{nh, nhg}, []*client.OpResult{op1, op2})

		default:
			t.Log("Using default deviation")
			c.AddNH(t, pair.nhID, "MACwithInterface", deviations.DefaultNetworkInstance(dut),
				fluent.InstalledInFIB, &gribi.NHOptions{Interface: p2.Name(), Mac: magicMac})
			c.AddNHG(t, pair.nhgID, map[uint64]uint64{pair.nhID: uint64(i)},
				deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
		}

		// Add IPv6 route for each NHG
		t.Logf("Adding IPv6 route %s/128 -> NHG %d", pair.route, pair.nhgID)
		c.AddIPv6(t, pair.route+"/128", pair.nhgID, deviations.DefaultNetworkInstance(dut),
			deviations.DefaultNetworkInstance(dut), fluent.InstalledInFIB)
	}
}

func validateFlowRate(t *testing.T, ate *ondatra.ATEDevice, topo gosnappi.Config, trafficRate uint32, lossPct float32) {
	sources := []struct {
		name   string
		vlan   attrs.Attributes
		rate   uint32
		vlanId uint32
	}{
		{"ipv4Flow1", atePort1Vlan1, trafficRate, 100},
		{"ipv4Flow2", atePort1Vlan2, trafficRate * 2, 200},
	}

	abs := func(num int) int {
		if num < 0 {
			return -num
		}
		return num
	}

	topo.Flows().Clear()

	for _, src := range sources {
		flow := topo.Flows().Add().SetName(src.name)
		flow.Metrics().SetEnable(true)
		flow.TxRx().Device().
			SetTxNames([]string{src.vlan.Name + ".IPv4"}).
			SetRxNames([]string{atePort2.Name + ".IPv4"})
		// flow.Size().SetFixed(trafficFrameSize)
		flow.Rate().SetGbps(uint32(src.rate))
		flow.Duration().
			SetFixedSeconds(gosnappi.NewFlowFixedSeconds().
				SetSeconds(float32(trafficDuration.Seconds())))

		eth := flow.Packet().Add().Ethernet()
		eth.Src().SetValue(src.vlan.MAC)
		eth.Dst().Auto()

		flow.Packet().Add().Vlan().Id().SetValue(src.vlanId)

		ipv4 := flow.Packet().Add().Ipv4()
		ipv4.Src().SetValue(src.vlan.IPv4)
		ipv4.Dst().SetValue(atePort2.IPv4)
	}

	ate.OTG().PushConfig(t, topo)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), topo, "IPv4")

	t.Logf("Sending traffic flows: ")
	ate.OTG().StartTraffic(t)
	time.Sleep(10 * time.Second)
	ate.OTG().StopTraffic(t)
	time.Sleep(10 * time.Second)

	otgutils.LogFlowMetrics(t, ate.OTG(), topo)

	for _, src := range sources {
		flowMetrics := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(src.name).State())
		sentPackets := *flowMetrics.Counters.OutPkts
		receivedPackets := *flowMetrics.Counters.InPkts

		if sentPackets == 0 {
			t.Errorf("No packets transmitted")
		}

		if receivedPackets == 0 {
			t.Errorf("No packets received")
		}
		lostPackets := abs(int(receivedPackets - sentPackets))
		switch lossPct {
		case 0:
			if lostPackets != 0 {
				t.Errorf("Expected 0 lost packets, but got %d out of %d lost packets", lostPackets, sentPackets)
			}
		default:
			expectedLostPackets := int(float32(sentPackets) * lossPct)
			lostPacketsVariation := int(float64(expectedLostPackets) * lossVariation)
			if lostPackets < expectedLostPackets-lostPacketsVariation || lostPackets > expectedLostPackets+lostPacketsVariation {
				t.Errorf("Expected lost packets to be within [%d, %d], but got %d", expectedLostPackets-lostPacketsVariation, expectedLostPackets+lostPacketsVariation, lostPackets)
			}
		}

	}

}
