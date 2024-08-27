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

package te_14_1_gribi_scaling_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/open-traffic-generator/snappi/gosnappi"
	fpargs "github.com/openconfig/featureprofiles/internal/args"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/tescale"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the test
// topology.
//
// The testbed consists of ate:port1 -> dut:port1
// and dut:port2 -> ate:port2.
// There are 16 SubInterfaces between dut:port2
// and ate:port2
//
//   - ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   - ate:port2 -> dut:port2 16 Sub interfaces:
//   - ate:port2.0 -> dut:port2.0 VLAN-ID: 0 subnet 198.51.100.0/30
//   - ate:port2.1 -> dut:port2.1 VLAN-ID: 1 subnet 198.51.100.4/30
//   - ate:port2.2 -> dut:port2.2 VLAN-ID: 2 subnet 198.51.100.8/30
//   - ate:port2.i -> dut:port2.i VLAN-ID i subnet 198.51.100.(4*i)/30
//   - ate:port2.16 -> dut:port2.16 VLAN-ID 16 subnet 198.51.100.60/30
const (
	ipv4PrefixLen = 30 // ipv4PrefixLen is the ATE and DUT interface IP prefix length.
	policyName    = "redirect-to-vrf_t"
)

var (
	dutPort1 = attrs.Attributes{
		Desc:    "dutPort1",
		IPv4:    "192.0.2.1",
		IPv4Len: ipv4PrefixLen,
	}

	atePort1 = attrs.Attributes{
		Name:    "atePort1",
		MAC:     "02:00:01:01:01:01",
		IPv4:    "192.0.2.2",
		IPv4Len: ipv4PrefixLen,
	}
)

func configureDUT(t *testing.T, dut *ondatra.DUTDevice) {
	dp1 := dut.Port(t, "port1")
	dp2 := dut.Port(t, "port2")
	d := &oc.Root{}

	vrfs := []string{deviations.DefaultNetworkInstance(dut), tescale.VRFT, tescale.VRFR, tescale.VRFRD}
	createVrf(t, dut, vrfs)

	// configure Ethernet interfaces first
	configureInterfaceDUT(t, d, dut, dp1, "src")
	configureInterfaceDUT(t, d, dut, dp2, "dst")

	// configure an L3 subinterface without vlan tagging under DUT port#1
	createSubifDUT(t, d, dut, dp1, 0, 0, dutPort1.IPv4)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, dp1.Name(), deviations.DefaultNetworkInstance(dut), 0)
	}

	applyForwardingPolicy(t, dp1.Name())

	// configure 16 L3 subinterfaces under DUT port#2 and assign them to DEFAULT vrf
	configureDUTSubIfs(t, d, dut, dp2)
}

// createVrf takes in a list of VRF names and creates them on the target devices.
func createVrf(t *testing.T, dut *ondatra.DUTDevice, vrfs []string) {
	for _, vrf := range vrfs {
		if vrf != deviations.DefaultNetworkInstance(dut) {
			// configure non-default VRFs
			d := &oc.Root{}
			i := d.GetOrCreateNetworkInstance(vrf)
			i.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
			gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), i)
		} else {
			// configure DEFAULT vrf
			fptest.ConfigureDefaultNetworkInstance(t, dut)
		}
	}
	// configure PBF
	gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), configurePBF(dut))
}

// configurePBF returns a fully configured network-instance PF struct
func configurePBF(dut *ondatra.DUTDevice) *oc.NetworkInstance_PolicyForwarding {
	d := &oc.Root{}
	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	pf := ni.GetOrCreatePolicyForwarding()
	vrfPolicy := pf.GetOrCreatePolicy(policyName)
	vrfPolicy.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
	vrfPolicy.GetOrCreateRule(1).GetOrCreateIpv4().SourceAddress = ygot.String(atePort1.IPv4 + "/32")
	vrfPolicy.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String(tescale.VRFT)
	return pf
}

// applyForwardingPolicy applies the forwarding policy on the interface.
func applyForwardingPolicy(t *testing.T, ingressPort string) {
	t.Logf("Applying forwarding policy on interface %v ... ", ingressPort)
	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	interfaceID := ingressPort
	if deviations.InterfaceRefInterfaceIDFormat(dut) {
		interfaceID = ingressPort + ".0"
	}
	pfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Interface(interfaceID)
	pfCfg := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut)).GetOrCreatePolicyForwarding().GetOrCreateInterface(interfaceID)
	pfCfg.ApplyVrfSelectionPolicy = ygot.String(policyName)
	pfCfg.GetOrCreateInterfaceRef().Interface = ygot.String(ingressPort)
	pfCfg.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		pfCfg.InterfaceRef = nil
	}
	gnmi.Replace(t, dut, pfPath.Config(), pfCfg)
}

// configureInterfaceDUT configures a single DUT port.
func configureInterfaceDUT(t *testing.T, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port, desc string) {
	ifName := dutPort.Name()
	i := d.GetOrCreateInterface(ifName)
	i.Description = ygot.String(desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}
	if deviations.ExplicitPortSpeed(dut) {
		i.GetOrCreateEthernet().PortSpeed = fptest.GetIfSpeed(t, dutPort)
	}
	gnmi.Replace(t, dut, gnmi.OC().Interface(ifName).Config(), i)
	t.Logf("DUT port %s configured", dutPort)
}

// createSubifDUT creates a single L3 subinterface
func createSubifDUT(t *testing.T, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port, index uint32, vlanID uint16, ipv4Addr string) {
	ifName := dutPort.Name()
	i := d.GetOrCreateInterface(dutPort.Name())
	s := i.GetOrCreateSubinterface(index)
	if vlanID != 0 {
		if deviations.DeprecatedVlanID(dut) {
			s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(vlanID)
		}
	}
	s4 := s.GetOrCreateIpv4()
	a := s4.GetOrCreateAddress(ipv4Addr)
	a.PrefixLength = ygot.Uint8(uint8(ipv4PrefixLen))
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}
	gnmi.Replace(t, dut, gnmi.OC().Interface(ifName).Subinterface(index).Config(), s)
}

// configureDUTSubIfs configures 16 DUT subinterfaces on the target device
func configureDUTSubIfs(t *testing.T, d *oc.Root, dut *ondatra.DUTDevice, dutPort *ondatra.Port) {
	for i := 0; i < 16; i++ {
		index := uint32(i)
		vlanID := uint16(i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID = uint16(i) + 1
		}
		dutIPv4 := fmt.Sprintf(`198.51.100.%d`, (4*i)+2)
		createSubifDUT(t, d, dut, dutPort, index, vlanID, dutIPv4)
		if deviations.ExplicitInterfaceInDefaultVRF(dut) {
			fptest.AssignToNetworkInstance(t, dut, dutPort.Name(), deviations.DefaultNetworkInstance(dut), index)
		}
	}
}

// configureATESubIfs configures 16 ATE subinterfaces on the target device
// It returns a slice of the corresponding ATE IPAddresses.
func configureATESubIfs(t *testing.T, top gosnappi.Config, atePort *ondatra.Port, dut *ondatra.DUTDevice) []string {
	nextHops := []string{}
	for i := 0; i < 16; i++ {
		vlanID := uint16(i)
		if deviations.NoMixOfTaggedAndUntaggedSubinterfaces(dut) {
			vlanID = uint16(i) + 1
		}
		dutIPv4 := fmt.Sprintf(`198.51.100.%d`, (4*i)+2)
		ateIPv4 := fmt.Sprintf(`198.51.100.%d`, (4*i)+1)
		name := fmt.Sprintf(`dst%d`, i)
		mac, _ := incrementMAC(atePort1.MAC, i+1)
		configureATE(t, top, atePort, vlanID, name, mac, dutIPv4, ateIPv4)
		nextHops = append(nextHops, ateIPv4)
	}
	return nextHops
}

// configureATE configures a single ATE layer 3 interface.
func configureATE(t *testing.T, top gosnappi.Config, atePort *ondatra.Port, vlanID uint16, Name, MAC, dutIPv4, ateIPv4 string) {
	t.Helper()

	dev := top.Devices().Add().SetName(Name + ".Dev")
	eth := dev.Ethernets().Add().SetName(Name + ".Eth").SetMac(MAC)
	eth.Connection().SetPortName(atePort.ID())
	if vlanID != 0 {
		eth.Vlans().Add().SetName(Name).SetId(uint32(vlanID))
	}
	eth.Ipv4Addresses().Add().SetName(Name + ".IPv4").SetAddress(ateIPv4).SetGateway(dutIPv4).SetPrefix(uint32(atePort1.IPv4Len))
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
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

func TestScaling(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	overrideScaleParams(dut)
	ate := ondatra.ATE(t, "ate")

	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI(t)

	ap1 := ate.Port(t, "port1")
	ap2 := ate.Port(t, "port2")
	top := gosnappi.NewConfig()
	top.Ports().Add().SetName(ate.Port(t, "port1").ID())
	top.Ports().Add().SetName(ate.Port(t, "port2").ID())

	configureDUT(t, dut)

	configureATE(t, top, ap1, 0, "src", atePort1.MAC, dutPort1.IPv4, atePort1.IPv4)
	// subIntfIPs is a []string slice with ATE IPv4 addresses for all the subInterfaces
	subIntfIPs := configureATESubIfs(t, top, ap2, dut)
	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)

	// Connect gRIBI client to DUT referred to as gRIBI - using PRESERVE persistence and
	// SINGLE_PRIMARY mode, with FIB ACK requested. Specify gRIBI as the leader.
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(12, 0).
		WithRedundancyMode(fluent.ElectedPrimaryClient).WithFIBACK()

	client.Start(ctx, t)
	defer client.Stop(t)

	defer func() {
		// Flush all entries after test.
		if err := gribi.FlushAll(client); err != nil {
			t.Error(err)
		}
	}()

	client.StartSending(ctx, t)
	if err := awaitTimeout(ctx, client, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for clientA: %v", err)
	}
	gribi.BecomeLeader(t, client)

	vrfConfigs := tescale.BuildVRFConfig(dut, subIntfIPs,
		tescale.Param{
			V4TunnelCount:         *fpargs.V4TunnelCount,
			V4TunnelNHGCount:      *fpargs.V4TunnelNHGCount,
			V4TunnelNHGSplitCount: *fpargs.V4TunnelNHGSplitCount,
			EgressNHGSplitCount:   *fpargs.EgressNHGSplitCount,
			V4ReEncapNHGCount:     *fpargs.V4ReEncapNHGCount,
		},
	)
	createFlow(t, ate, top, vrfConfigs[1])

	for _, vrfConfig := range vrfConfigs {
		entries := append(vrfConfig.NHs, vrfConfig.NHGs...)
		entries = append(entries, vrfConfig.V4Entries...)
		client.Modify().AddEntry(t, entries...)
		if err := awaitTimeout(ctx, client, t, 5*time.Minute); err != nil {
			t.Fatalf("Could not program entries, got err: %v", err)
		}
		t.Logf("Created %d NHs, %d NHGs, %d IPv4Entries in %s VRF", len(vrfConfig.NHs), len(vrfConfig.NHGs), len(vrfConfig.V4Entries), vrfConfig.Name)
	}

	checkTraffic(t, ate, top)
}

func createFlow(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config, vrfTConf *tescale.VRFConfig) {
	dstIPs := []string{}
	for _, v4Entry := range vrfTConf.V4Entries {
		ep, _ := v4Entry.EntryProto()
		dstIPs = append(dstIPs, strings.Split(ep.GetIpv4().GetPrefix(), "/")[0])
	}
	rxNames := []string{}
	for i := 0; i < 16; i++ {
		rxNames = append(rxNames, fmt.Sprintf(`dst%d.IPv4`, i))
	}

	top.Flows().Clear()
	flow := top.Flows().Add().SetName("flow")
	flow.Metrics().SetEnable(true)
	flow.Size().SetFixed(512)
	flow.Rate().SetPps(100)
	flow.Duration().Continuous()
	flow.TxRx().Device().
		SetTxNames([]string{"src.IPv4"}).
		SetRxNames(rxNames)
	ethHeader := flow.Packet().Add().Ethernet()
	ethHeader.Src().SetValue(atePort1.MAC)
	v4 := flow.Packet().Add().Ipv4()
	v4.Src().SetValue(atePort1.IPv4)
	v4.Dst().SetValues(dstIPs)

	ate.OTG().PushConfig(t, top)
	ate.OTG().StartProtocols(t)
	otgutils.WaitForARP(t, ate.OTG(), top, "flow")
}

func checkTraffic(t *testing.T, ate *ondatra.ATEDevice, top gosnappi.Config) {
	ate.OTG().StartTraffic(t)
	time.Sleep(time.Second * 30)
	ate.OTG().StopTraffic(t)

	otgutils.LogFlowMetrics(t, ate.OTG(), top)
	otgutils.LogPortMetrics(t, ate.OTG(), top)

	t.Log("Checking flow telemetry...")
	recvMetric := gnmi.Get(t, ate.OTG(), gnmi.OTG().Flow("flow").State())
	txPackets := recvMetric.GetCounters().GetOutPkts()
	rxPackets := recvMetric.GetCounters().GetInPkts()
	lostPackets := txPackets - rxPackets
	lossPct := lostPackets * 100 / txPackets

	if lossPct > 1 {
		t.Errorf("FAIL- Got %v%% packet loss for %s ; expected < 1%%", lossPct, "flow")
	}
}

// overrideScaleParams allows to override the default scale parameters based on the DUT vendor.
func overrideScaleParams(dut *ondatra.DUTDevice) {
	if deviations.OverrideDefaultNhScale(dut) {
		if dut.Vendor() == ondatra.CISCO {
			*fpargs.V4TunnelCount = 3328
		}
	}
}
