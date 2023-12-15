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

package ipv4_entry_with_aggregate_ports_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ygot/ygot"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "DUT Port 1",
		IPv4:    "192.0.2.1",
		IPv4Len: 30,
	}
	dutDst = attrs.Attributes{
		Desc:    "DUT Bundle",
		IPv4:    "192.0.2.5",
		IPv4Len: 30,
	}
	atePort1 = attrs.Attributes{
		Name:    "port1",
		MAC:     "02:00:01:01:01:01",
		Desc:    "ATE Port 1",
		IPv4:    "192.0.2.2",
		IPv4Len: 30,
	}
	ateDst = attrs.Attributes{
		Name:    "port2",
		MAC:     "02:00:02:01:01:01",
		Desc:    "ATE Port 2",
		IPv4:    "192.0.2.6",
		IPv4Len: 30,
	}
)

const (
	// Next-hop group ID for dstPfx
	nhgID = 42
	// Next-hop 1 ID for dutDst
	nh1ID = 43
	// IP address of configured Next Hop
	nh1IpAddr = "192.0.2.22"
	// A destination MAC address set by gRIBI.
	staticDstMAC = "02:00:00:00:00:01"
	// ATE LAG name
	lagName = "LAGRx"
	// Destination prefix for DUT to ATE traffic.
	dstPfx      = "198.51.100.0/24"
	dstPfxMin   = "198.51.100.0"
	dstPfxMax   = "198.51.100.255"
	dstPfxCount = 256
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func TestIpv4EntryOnAggregatePort(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	aggID := netutil.NextAggregateInterface(t, dut)
	configureDUT(t, dut, aggID)

	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	configureATE(t, ate, top, aggID)

	configureGRIBIPrefixes(t, dut, aggID)

	f1 := createFlow(t, "aggregateFlow", ate, top)

	t.Run("Baseline flow", func(t *testing.T) {
		validateTrafficFlows(t, ate, []string{f1}, []string{})
	})

	t.Run("Aggregate Port2 disabled", func(t *testing.T) {
		aggregatePortState(t, dut, ate, []string{"port2"}, false)
		validateTrafficFlows(t, ate, []string{f1}, []string{})
	})

	t.Run("Aggregate Port2 and Port3 disabled", func(t *testing.T) {
		aggregatePortState(t, dut, ate, []string{"port3"}, false)
		validateTrafficFlows(t, ate, []string{}, []string{f1})
	})

	t.Run("Aggregate Ports re-enabled", func(t *testing.T) {
		aggregatePortState(t, dut, ate, []string{"port2", "port3"}, true)
		validateTrafficFlows(t, ate, []string{f1}, []string{})
	})
}

func aggregatePortState(t *testing.T, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, ports []string, portEnabled bool) {
	t.Helper()
	if deviations.ATEPortLinkStateOperationsUnsupported(ate) {
		// Setting admin state down on the DUT interface.
		// Setting the OTG interface down has no effect in KNE environments.
		for _, p := range ports {
			gnmi.Replace(t, dut, gnmi.OC().Interface(dut.Port(t, p).Name()).Enabled().Config(), portEnabled)
		}
	} else {
		ls := gosnappi.StatePortLinkState.DOWN
		if portEnabled {
			ls = gosnappi.StatePortLinkState.UP
		}

		var portNames []string
		for _, p := range ports {
			portNames = append(portNames, ate.Port(t, p).ID())
		}
		portStateAction := gosnappi.NewControlState()
		portStateAction.Port().Link().SetPortNames(portNames).SetState(ls)
		ate.OTG().SetControlState(t, portStateAction)
	}
}

func validateTrafficFlows(t *testing.T, ate *ondatra.ATEDevice, good, bad []string) {
	ateTop := ate.OTG().FetchConfig(t)
	if len(good) == 0 && len(bad) == 0 {
		return
	}

	ate.OTG().StartTraffic(t)
	time.Sleep(15 * time.Second)
	ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, ate.OTG(), ateTop)
	otgutils.LogPortMetrics(t, ate.OTG(), ateTop)

	for _, flow := range good {
		var txPackets, rxPackets uint64
		recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow).State())
		txPackets = recvMetric.GetCounters().GetOutPkts()
		rxPackets = recvMetric.GetCounters().GetInPkts()
		if txPackets == 0 {
			t.Fatalf("TxPkts == 0, want > 0")
		}
		lostPackets := float32(txPackets - rxPackets)
		lossPct := lostPackets * 100 / float32(txPackets)
		if got := lossPct; got > 0 {
			t.Fatalf("LossPct for flow %s: got %v, want 0", flow, got)
		}
	}

	for _, flow := range bad {
		recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow(flow).State())
		txPackets := recvMetric.GetCounters().GetOutPkts()
		rxPackets := recvMetric.GetCounters().GetInPkts()
		if txPackets == 0 {
			t.Fatalf("TxPkts == 0, want > 0")
		}
		lostPackets := float32(txPackets - rxPackets)
		lossPct := lostPackets * 100 / float32(txPackets)
		if got := lossPct; got < 100 {
			t.Fatalf("LossPct for flow %s: got %v, want 100", flow, got)
		}
	}
}

// createFlow returns a flow from atePort1 to the dstPfx, expected to arrive on ATE bundle interface.
func createFlow(t *testing.T, name string, ate *ondatra.ATEDevice, ateTop gosnappi.Config) string {
	t.Helper()
	otg := ate.OTG()
	flowipv4 := ateTop.Flows().Add().SetName(name)
	flowipv4.Metrics().SetEnable(true)
	e1 := flowipv4.Packet().Add().Ethernet()
	e1.Src().SetValue(atePort1.MAC)
	flowipv4.TxRx().Device().SetTxNames([]string{atePort1.Name + ".IPv4"}).SetRxNames([]string{lagName + ".IPv4"})
	v4 := flowipv4.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().Increment().SetStart(dstPfxMin).SetCount(dstPfxCount)
	otg.PushConfig(t, ateTop)
	otg.StartProtocols(t)
	return name
}

func configureGRIBIPrefixes(t *testing.T, dut *ondatra.DUTDevice, aggID string) {
	t.Helper()

	e1 := fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithIndex(nh1ID).WithInterfaceRef(aggID).WithMacAddress(staticDstMAC)
	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) || deviations.GRIBIMACOverrideWithStaticARP(dut) {
		// Static route to nh1IPAddr which is the ATE Lag port.
		s := &oc.NetworkInstance_Protocol_Static{
			Prefix: ygot.String(nh1IpAddr + "/32"),
			NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
				"0": {
					Index: ygot.String("0"),
					InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
						Interface: ygot.String(aggID),
					},
				},
			},
		}
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		gnmi.Replace(t, dut, sp.Static(nh1IpAddr+"/32").Config(), s)
		e1 = fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(nh1ID).WithInterfaceRef(aggID).WithIPAddress(nh1IpAddr).WithMacAddress(staticDstMAC)
	}

	e2 := fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithID(nhgID).AddNextHop(nh1ID, 1)
	e3 := fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
		WithPrefix(dstPfx).WithNextHopGroup(nhgID)

	r1 := fluent.OperationResult().
		WithNextHopOperation(nh1ID).
		WithProgrammingResult(fluent.InstalledInFIB).
		WithOperationType(constants.Add).
		AsResult()
	r2 := fluent.OperationResult().
		WithNextHopGroupOperation(nhgID).
		WithProgrammingResult(fluent.InstalledInFIB).
		WithOperationType(constants.Add).
		AsResult()
	r3 := fluent.OperationResult().
		WithIPv4Operation(dstPfx).
		WithProgrammingResult(fluent.InstalledInFIB).
		WithOperationType(constants.Add).
		AsResult()

	// Configure the gRIBI client.
	c := fluent.NewClient()
	c.Connection().
		WithStub(dut.RawAPIs().GRIBI(t)).
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(1 /* low */, 0 /* hi */). // ID must be > 0.
		WithPersistence().
		WithFIBACK()

	ctx := context.Background()
	c.Start(ctx, t)
	defer c.Stop(t)
	c.StartSending(ctx, t)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation: %v", err)
	}
	gribi.BecomeLeader(t, c)

	t.Cleanup(func() {
		if err := gribi.FlushAll(c); err != nil {
			t.Errorf("Cannot flush: %v", err)
		}
	})

	c.Modify().AddEntry(t, e1, e2, e3)
	if err := awaitTimeout(ctx, c, t, time.Minute); err != nil {
		t.Fatalf("Await got error for entries: %v", err)
	}
	t.Cleanup(func() {
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		gnmi.Delete(t, dut, sp.Static(nh1IpAddr+"/32").Config())
	})
	chk.HasResult(t, c.Results(t), r1, chk.IgnoreOperationID())
	chk.HasResult(t, c.Results(t), r2, chk.IgnoreOperationID())
	chk.HasResult(t, c.Results(t), r3, chk.IgnoreOperationID())
}

func configureDUT(t *testing.T, dut *ondatra.DUTDevice, aggID string) {
	t.Helper()
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	p1 := dut.Port(t, "port1")
	gnmi.Replace(t, dut, gnmi.OC().Interface(p1.Name()).Config(), dutPort1.NewOCInterface(p1.Name(), dut))
	dutAggPorts := []*ondatra.Port{
		dut.Port(t, "port2"),
		dut.Port(t, "port3"),
	}

	configureDUTBundle(t, dut, dutAggPorts, aggID)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, aggID, deviations.DefaultNetworkInstance(dut), 0)
		fptest.AssignToNetworkInstance(t, dut, p1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.ExplicitPortSpeed(dut) {
		fptest.SetPortSpeed(t, p1)
		for _, port := range dutAggPorts {
			fptest.SetPortSpeed(t, port)
		}
	}
}

func configureATE(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, aggID string) {
	t.Helper()
	p1 := ate.Port(t, "port1")
	top.Ports().Add().SetName(p1.ID())
	srcDev := top.Devices().Add().SetName(atePort1.Name)
	srcEth := srcDev.Ethernets().Add().SetName(atePort1.Name + ".Eth").SetMac(atePort1.MAC)
	srcEth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.PORT_NAME).SetPortName(p1.ID())
	srcEth.Ipv4Addresses().Add().SetName(atePort1.Name + ".IPv4").SetAddress(atePort1.IPv4).SetGateway(dutPort1.IPv4).SetPrefix(uint32(atePort1.IPv4Len))

	ateAggPorts := []*ondatra.Port{
		ate.Port(t, "port2"),
		ate.Port(t, "port3"),
	}
	configureATEBundle(t, ate, top, ateAggPorts, aggID)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
}

func configureATEBundle(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()
	agg := top.Lags().Add().SetName(lagName)
	lagID, _ := strconv.Atoi(aggID)
	agg.Protocol().SetChoice("static").Static().SetLagId(uint32(lagID))
	for i, p := range aggPorts {
		port := top.Ports().Add().SetName(p.ID())
		newMac, err := incrementMAC(ateDst.MAC, i+1)
		if err != nil {
			t.Fatal(err)
		}
		agg.Ports().Add().SetPortName(port.Name()).Ethernet().SetMac(newMac).SetName("LAGRx-" + strconv.Itoa(i))
	}

	dstDev := top.Devices().Add().SetName(agg.Name() + ".dev")
	dstEth := dstDev.Ethernets().Add().SetName(lagName + ".Eth").SetMac(ateDst.MAC)
	dstEth.Connection().SetChoice(gosnappi.EthernetConnectionChoice.LAG_NAME).SetLagName(agg.Name())
	dstEth.Ipv4Addresses().Add().SetName(lagName + ".IPv4").SetAddress(ateDst.IPv4).SetGateway(dutDst.IPv4).SetPrefix(uint32(ateDst.IPv4Len))
}

// incrementMAC increments the MAC by i. Returns error if the mac cannot be parsed or overflows the mac address space
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

func setupAggregateAtomically(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()
	d := &oc.Root{}
	agg := d.GetOrCreateInterface(aggID)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC

	for _, port := range aggPorts {
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}

func configureDUTBundle(t *testing.T, dut *ondatra.DUTDevice, aggPorts []*ondatra.Port, aggID string) {
	t.Helper()

	if deviations.AggregateAtomicUpdate(dut) {
		// Clear aggregate & ip config on ports.
		for _, port := range aggPorts {
			gnmi.Delete(t, dut, gnmi.OC().Interface(port.Name()).Ethernet().Config())
		}
		setupAggregateAtomically(t, dut, aggPorts, aggID)
	}

	agg := dutDst.NewOCInterface(aggID, dut)
	agg.Type = oc.IETFInterfaces_InterfaceType_ieee8023adLag
	agg.GetOrCreateAggregation().LagType = oc.IfAggregate_AggregationType_STATIC
	gnmi.Replace(t, dut, gnmi.OC().Interface(aggID).Config(), agg)

	// Static ARP configuration with neighbor IP as nh1IPAddr
	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) || deviations.GRIBIMACOverrideWithStaticARP(dut) {
		ipv4 := agg.GetOrCreateSubinterface(0).GetOrCreateIpv4()
		n4 := ipv4.GetOrCreateNeighbor(nh1IpAddr)
		n4.LinkLayerAddress = ygot.String(staticDstMAC)
		gnmi.Replace(t, dut, gnmi.OC().Interface(aggID).Config(), agg)
	}

	for _, port := range aggPorts {
		d := &oc.Root{}
		i := d.GetOrCreateInterface(port.Name())
		i.GetOrCreateEthernet().AggregateId = ygot.String(aggID)
		i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd

		if deviations.InterfaceEnabled(dut) {
			i.Enabled = ygot.Bool(true)
		}
		gnmi.Replace(t, dut, gnmi.OC().Interface(port.Name()).Config(), i)
	}
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}
