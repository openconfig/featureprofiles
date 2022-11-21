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
	"testing"
	"time"

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
// There are 64 SubInterfaces between dut:port2
// and ate:port2
//
//   - ate:port1 -> dut:port1 subnet 192.0.2.0/30
//   - ate:port2 -> dut:port2 64 Sub interfaces:
//   - ate:port2.0 -> dut:port2.0 VLAN-ID: 0 subnet 198.51.100.0/30
//   - ate:port2.1 -> dut:port2.1 VLAN-ID: 1 subnet 198.51.100.4/30
//   - ate:port2.2 -> dut:port2.2 VLAN-ID: 2 subnet 198.51.100.8/30
//   - ate:port2.i -> dut:port2.i VLAN-ID i subnet 198.51.100.(4*i)/30
//   - ate:port2.63 -> dut:port2.63 VLAN-ID 63 subnet 198.51.100.252/30
const (
	ipv4PrefixLen = 30 // ipv4PrefixLen is the ATE and DUT interface IP prefix length.
	vrf1          = "vrf1"
	vrf2          = "vrf2"
	vrf3          = "vrf3"
	IPBlock1      = "198.18.0.1/18"   // IPBlock1 represents the ipv4 entries in VRF1
	IPBlock2      = "198.18.64.1/18"  // IPBlock2 represents the ipv4 entries in VRF2
	IPBlock3      = "198.18.128.1/18" // IPBlock3 represents the ipv4 entries in VRF3
	nhID1         = 2                 // nhID1 is the starting nh Index for entries in VRF1
	nhID2         = 1002              // nhID2 is the starting nh Index for entries in VRF2
	nhID3         = 18502             // nhID3 is the starting nh Index for entries in VRF3
	tunnelSrcIP   = "198.18.204.1"    // tunnelSrcIP represents Source IP of IPinIP Tunnel
	tunnelDstIP   = "198.18.208.1"    // tunnelDstIP represents Dest IP of IPinIP Tunnel
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

// entryIndex captures all the parameters required for specifying :
//
//			a. number of nextHops in a nextHopGroup
//	   b. number of IPEntries per nextHopGroup
type routesParam struct {
	nhgIndex   int    // nhgIndex is the starting nhg Index for each IPBlock
	maxNhCount int    // maxNhCount is the max number of nexthops per nextHopGroup
	maxIPCount int    // maxIPCount is the max numbver of IPs per nextHopGroup
	vrf        string // vrf represents the name of the vrf string
	nhID       int    // nhID is the starting nh Index for each nextHop range
}

// pushIPv4Entries pushes IP entries in a specified VRF in the target DUT.
// It uses the parameters from entryIndex and virtualVIPs for programming entries.
func pushIPv4Entries(t *testing.T, virtualVIPs []string, indices []*routesParam, args *testArgs) {

	IPBlocks := make(map[string][]string)
	IPBlocks[vrf1] = createIPv4Entries(IPBlock1)
	IPBlocks[vrf2] = createIPv4Entries(IPBlock2)
	IPBlocks[vrf3] = createIPv4Entries(IPBlock3)
	nextHops := make(map[string][]string)
	nextHops[vrf2] = buildL3NextHops(17500, virtualVIPs)
	nextHops[vrf1] = virtualVIPs
	nextHops[vrf3] = IPBlocks[vrf1][:500]

	for _, index := range indices {
		installEntries(t, IPBlocks[index.vrf], nextHops[index.vrf], *index, args)
	}
}

// buildIndexList returns all indices required for installing entries in each VRF.
func buildIndexList() []*routesParam {
	index1v4 := &routesParam{nhgIndex: 3, maxNhCount: 10, maxIPCount: 200, vrf: vrf1, nhID: nhID1}
	index2v4 := &routesParam{nhgIndex: 103, maxNhCount: 35, maxIPCount: 60, vrf: vrf2, nhID: nhID2}
	index3v4 := &routesParam{nhgIndex: 605, maxNhCount: 1, maxIPCount: 40, vrf: vrf3, nhID: nhID3}

	return []*routesParam{index1v4, index2v4, index3v4}
}

// buildL3NextHop buids N number of NHs each reference (squentially) an IP from the provided IP block.
func buildL3NextHops(n int, ips []string) []string {
	// repeatedNextHops will store the "n" times repeated ips []string
	repeatedNextHops := []string{}
	if n > len(ips) {
		repeatCount := len(ips) / n
		for min, max := 1, repeatCount; min < max; {
			repeatedNextHops = append(repeatedNextHops, ips...)
			min = min + 1
		}
		repeatCount = len(ips) % n
		if repeatCount > 0 {
			repeatedNextHops = append(repeatedNextHops, ips[:repeatCount]...)
		}
	}
	return repeatedNextHops
}

// createIPv4Entries creates IPv4 Entries given the totalCount and starting prefix
func createIPv4Entries(startIP string) []string {

	_, netCIDR, _ := net.ParseCIDR(startIP)
	netMask := binary.BigEndian.Uint32(netCIDR.Mask)
	firstIP := binary.BigEndian.Uint32(netCIDR.IP)
	lastIP := (firstIP & netMask) | (netMask ^ 0xffffffff)
	entries := []string{}
	for i := firstIP; i <= lastIP; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)

		entries = append(entries, fmt.Sprint(ip))
	}
	return entries
}

// installEntries installs IPv4 Entries in the VRF with the given nextHops and nextHopGroups using gRIBI.
func installEntries(t *testing.T, ips []string, nexthops []string, index routesParam, args *testArgs) {
	nextCount := 0
	localIndex := index.nhgIndex
	for i, ateAddr := range nexthops {
		ind := uint64(index.nhID + i)
		if index.vrf == "vrf3" {
			nh := fluent.NextHopEntry().
				WithNetworkInstance(*deviations.DefaultNetworkInstance).
				WithIndex(ind).
				WithIPinIP(tunnelSrcIP, tunnelDstIP).
				WithDecapsulateHeader(fluent.IPinIP).
				WithEncapsulateHeader(fluent.IPinIP).
				WithNextHopNetworkInstance(vrf1).
				WithElectionID(args.electionID.Low, args.electionID.High)
			args.client.Modify().AddEntry(t, nh)
		} else {
			nh := fluent.NextHopEntry().
				WithNetworkInstance(*deviations.DefaultNetworkInstance).
				WithIndex(ind).
				WithIPAddress(ateAddr).
				WithElectionID(args.electionID.Low, args.electionID.High)
			args.client.Modify().AddEntry(t, nh)
		}

		nhg := fluent.NextHopGroupEntry().
			WithNetworkInstance(*deviations.DefaultNetworkInstance).
			WithID(uint64(localIndex)).
			AddNextHop(ind, uint64(index.maxNhCount)).
			WithElectionID(args.electionID.Low, args.electionID.High)
		args.client.Modify().AddEntry(t, nhg)
		nextCount = nextCount + 1
		if nextCount == index.maxNhCount {
			localIndex = localIndex + 1
			nextCount = 0
		}
	}
	nextCount = 0
	localIndex = index.nhgIndex
	for ip := range ips {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(ips[ip]+"/32").
				WithNetworkInstance(index.vrf).
				WithNextHopGroup(uint64(localIndex)).
				WithNextHopGroupNetworkInstance(*deviations.DefaultNetworkInstance))
		nextCount = nextCount + 1
		if nextCount == index.maxIPCount {
			localIndex = localIndex + 1
			nextCount = 0
		}
	}

	time.Sleep(1 * time.Minute)
	if err := awaitTimeout(args.ctx, args.client, t, 2*time.Minute); err != nil {
		t.Fatalf("Could not program entries via clientA, got err: %v", err)
	}
	gr, err := args.client.Get().
		WithNetworkInstance(index.vrf).
		WithAFT(fluent.IPv4).
		Send()
	if err != nil {
		t.Fatalf("got unexpected error from get, got: %v", err)
	}
	nextCount = 0
	for ip := range ips {
		chk.GetResponseHasEntries(t, gr,
			fluent.IPv4Entry().
				WithNetworkInstance(index.vrf).
				WithNextHopGroup(uint64(index.nhgIndex)).
				WithPrefix(ips[ip]+"/32"),
		)
		nextCount = nextCount + 1
		if nextCount == index.maxIPCount {
			index.nhgIndex = index.nhgIndex + 1
			nextCount = 0
		}
	}
}

// pushDefaultEntries creates NextHopGroup entries using the 64 SubIntf address and creates 1000 IPV4 Entries.
func pushDefaultEntries(t *testing.T, args *testArgs, nextHops []string) []string {
	for i := range nextHops {
		index := uint64(i + 1)
		args.client.Modify().AddEntry(t,
			fluent.NextHopEntry().
				WithNetworkInstance(*deviations.DefaultNetworkInstance).
				WithIndex(index).
				WithIPAddress(nextHops[i]).
				WithElectionID(12, 0))

		args.client.Modify().AddEntry(t,
			fluent.NextHopGroupEntry().
				WithNetworkInstance(*deviations.DefaultNetworkInstance).
				WithID(uint64(2)).
				AddNextHop(index, 64).
				WithElectionID(12, 0))
	}
	time.Sleep(time.Minute)
	virtualVIPs := createIPv4Entries("198.18.196.1/22")

	for ip := range virtualVIPs {
		args.client.Modify().AddEntry(t,
			fluent.IPv4Entry().
				WithPrefix(virtualVIPs[ip]+"/32").
				WithNetworkInstance(*deviations.DefaultNetworkInstance).
				WithNextHopGroup(uint64(2)).
				WithElectionID(12, 0))
	}
	if err := awaitTimeout(args.ctx, args.client, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via clientA, got err: %v", err)
	}

	for ip := range virtualVIPs {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(virtualVIPs[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
	return virtualVIPs
}

// createVrf creates takes in a list of VRF names and creates them on the target devices.
func createVrf(t *testing.T, dut *ondatra.DUTDevice, d *oc.Root, vrfs []string) {
	for _, vrf := range vrfs {
		// For non-default VRF, we want to replace the
		// entire VRF tree so the instance is created.
		i := d.GetOrCreateNetworkInstance(vrf)
		i.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		i.Enabled = ygot.Bool(true)
		i.EnabledAddressFamilies = []oc.E_Types_ADDRESS_FAMILY{
			oc.Types_ADDRESS_FAMILY_IPV4,
			oc.Types_ADDRESS_FAMILY_IPV6,
		}
		i.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, *deviations.StaticProtocolName)
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), i)
		nip := gnmi.OC().NetworkInstance(vrf)
		fptest.LogQuery(t, "nonDefaultNI", nip.Config(), gnmi.GetConfig(t, dut, nip.Config()))
	}
}

// pushConfig pushes the configuration generated by this
// struct to the device using gNMI SetReplace.
func pushConfig(t *testing.T, dut *ondatra.DUTDevice, dutPort *ondatra.Port, d *oc.Root) {
	t.Helper()

	iname := dutPort.Name()
	i := d.GetOrCreateInterface(iname)
	gnmi.Replace(t, dut, gnmi.OC().Interface(iname).Config(), i)
}

// configureInterfaceDUT configures a single DUT layer 2 port.
func configureInterfaceDUT(t *testing.T, dutPort *ondatra.Port, d *oc.Root, desc string) {
	t.Helper()

	i := d.GetOrCreateInterface(dutPort.Name())
	i.Description = ygot.String(desc)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	if *deviations.InterfaceEnabled {
		i.Enabled = ygot.Bool(true)
	}
	t.Logf("DUT port %s configured", dutPort)
}

// generateSubIntfPair takes the number of subInterfaces, dut,ate,ports and Ixia topology.
// It configures ATE/DUT SubInterfaces on the target device
// It returns a slice of the corresponding ATE IPAddresses.
func generateSubIntfPair(t *testing.T, top gosnappi.Config, dut *ondatra.DUTDevice, ate *ondatra.ATEDevice, dutPort, atePort *ondatra.Port, d *oc.Root) []string {
	nextHops := []string{}
	nextHopCount := 63 // nextHopCount specifies number of nextHop IPs needed.
	for i := 0; i <= nextHopCount; i++ {
		vlanID := uint16(i)
		name := fmt.Sprintf(`dst%d`, i)
		Index := uint32(i)
		ateIPv4 := fmt.Sprintf(`198.51.100.%d`, ((4 * i) + 1))
		dutIPv4 := fmt.Sprintf(`198.51.100.%d`, ((4 * i) + 2))
		configureSubinterfaceDUT(t, d, dutPort, Index, vlanID, dutIPv4, *deviations.DefaultNetworkInstance)
		MAC, _ := incrementMAC(atePort1.MAC, i+1)
		configureATE(t, top, ate, atePort, vlanID, name, MAC, dutIPv4, ateIPv4)
		nextHops = append(nextHops, ateIPv4)
	}
	configureInterfaceDUT(t, dutPort, d, "dst")
	pushConfig(t, dut, dutPort, d)
	return nextHops
}

// configureSubinterfaceDUT configures a single DUT layer 3 sub-interface.
func configureSubinterfaceDUT(t *testing.T, d *oc.Root, dutPort *ondatra.Port, index uint32, vlanID uint16, dutIPv4 string, vrf string) {
	t.Helper()
	if vrf != "" {
		t.Logf("Put port %s into vrf %s", dutPort.Name(), vrf)
		d.GetOrCreateNetworkInstance(vrf).GetOrCreateInterface(dutPort.Name())
		d.GetOrCreateNetworkInstance(vrf).EnabledAddressFamilies = []oc.E_Types_ADDRESS_FAMILY{oc.Types_ADDRESS_FAMILY_IPV4}
	}

	i := d.GetOrCreateInterface(dutPort.Name())
	s := i.GetOrCreateSubinterface(index)
	if vlanID != 0 {
		s.GetOrCreateVlan().VlanId = oc.UnionUint16(vlanID)
	}

	sipv4 := s.GetOrCreateIpv4()

	if *deviations.InterfaceEnabled {
		sipv4.Enabled = ygot.Bool(true)
	}

	a := sipv4.GetOrCreateAddress(dutIPv4)
	a.PrefixLength = ygot.Uint8(uint8(ipv4PrefixLen))

}

// configureATE configures a single ATE layer 3 interface.
func configureATE(t *testing.T, top gosnappi.Config, ate *ondatra.ATEDevice, atePort *ondatra.Port, vlanID uint16, Name, MAC, dutIPv4, ateIPv4 string) {
	t.Helper()

	dev := top.Devices().Add().SetName(Name + ".Dev")
	eth := dev.Ethernets().Add().SetName(Name + ".Eth")
	eth.Connection().SetChoice("port_name")
	eth.SetPortName(atePort.ID()).SetMac(MAC)
	if vlanID != 0 {
		eth.Vlans().Add().SetName(Name).SetId(int32(vlanID))
	}
	eth.Ipv4Addresses().Add().SetName(Name + ".IPv4").SetAddress(ateIPv4).SetGateway(dutIPv4).SetPrefix(int32(atePort1.IPv4Len))

}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

// testArgs holds the objects needed by a test case.
type testArgs struct {
	ctx        context.Context
	client     *fluent.GRIBIClient
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	top        gosnappi.Config
	electionID gribi.Uint128
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
	d := &oc.Root{}
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")

	ctx := context.Background()
	gribic := dut.RawAPIs().GRIBI().Default(t)
	dp1 := dut.Port(t, "port1")
	ap1 := ate.Port(t, "port1")
	top := ate.OTG().NewConfig(t)
	top.Ports().Add().SetName(ate.Port(t, "port1").ID())
	vrfs := []string{*deviations.DefaultNetworkInstance, vrf1, vrf2, vrf3}
	createVrf(t, dut, d, vrfs)
	// configure an L3 subinterface of no vlan tagging under DUT port#1
	configureSubinterfaceDUT(t, d, dp1, 0, 0, dutPort1.IPv4, vrf1)
	configureInterfaceDUT(t, dp1, d, "src")
	configureATE(t, top, ate, ap1, 0, "src", atePort1.MAC, dutPort1.IPv4, atePort1.IPv4)
	pushConfig(t, dut, dp1, d)
	ap2 := ate.Port(t, "port2")
	dp2 := dut.Port(t, "port2")
	top.Ports().Add().SetName(ate.Port(t, "port2").ID())
	// subIntfIPs is the ATE IPv4 addresses for all the subInterfaces
	subIntfIPs := generateSubIntfPair(t, top, dut, ate, dp2, ap2, d)
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
	eID := gribi.BecomeLeader(t, client)

	args := &testArgs{
		ctx:        ctx,
		client:     client,
		dut:        dut,
		ate:        ate,
		top:        top,
		electionID: eID,
	}
	// nextHops are ipv4 entries used for deriving nextHops for IPBlock1 and IPBlock2
	nextHops := pushDefaultEntries(t, args, subIntfIPs)
	// indexList is the metadata of number of NH/NHG/IP count/VRF for each IPBlock
	indexList := buildIndexList()
	// pushIPv4Entries builds the scaling topology.
	pushIPv4Entries(t, nextHops, indexList, args)
}
