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

// Package hierarchical_weight_resolution_test implements TE-3.3 of the Popgate vendor testplan
package hierarchical_weight_resolution_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/tcheck"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/telemetry"
	"github.com/openconfig/ygot/ygot"
)

type attributes struct {
	attrs.Attributes
	numSubIntf      uint32
	networkInstance string
	ip              func(vlan uint8) string
	gateway         func(vlan uint8) string
}

type nhInfo struct {
	index  uint64
	weight uint64
}

const (
	ipv4PrefixLen   = 30
	ipv4EntryPrefix = "203.0.113.0/24"
	ipv4FlowIPStart = "203.0.113.0"
	ipv4FlowIPEnd   = "203.0.113.255"
	nhEntryIP1      = "192.0.2.111"
	nhEntryIP2      = "192.0.2.222"
	nonDefaultVRF   = "VRF-1"
	// 'deviation' is the maximum difference that is allowed between the observed
	// traffic distribution and the required traffic distribution.
	deviation = 0.5
)

var (
	dutPort1 = attributes{
		Attributes: attrs.Attributes{
			Desc:    "dutPort1",
			Name:    "port1",
			IPv4:    dutPort1IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf:      0,
		networkInstance: nonDefaultVRF,
		ip:              dutPort1IPv4,
	}

	atePort1 = attributes{
		Attributes: attrs.Attributes{
			Name:    "port1",
			IPv4:    atePort1IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 0,
		ip:         atePort1IPv4,
		gateway:    dutPort1IPv4,
	}

	dutPort2 = attributes{
		Attributes: attrs.Attributes{
			Desc:    "dutPort2",
			Name:    "port2",
			IPv4:    dutPort2IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 18,
		ip:         dutPort2IPv4,
	}

	atePort2 = attributes{
		Attributes: attrs.Attributes{
			Name:    "port2",
			IPv4:    atePort2IPv4(0),
			IPv4Len: ipv4PrefixLen,
		},
		numSubIntf: 18,
		ip:         atePort2IPv4,
		gateway:    dutPort2IPv4,
	}

	// nhgIPv4EntryMap maps NextHopGroups to the ipv4 entries pointing to that NextHopGroup.
	nhgIPv4EntryMap = map[uint64]string{
		1: ipv4EntryPrefix,
		2: cidr(nhEntryIP1, 32),
		3: cidr(nhEntryIP2, 32),
	}
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// dutPort1IPv4 returns ip address 192.0.2.1, for every vlanID.
func dutPort1IPv4(uint8) string {
	return "192.0.2.1"
}

// atePort1IPv4 returns ip address 192.0.2.2, for every vlanID
func atePort1IPv4(uint8) string {
	return "192.0.2.2"
}

// dutPort2IPv4 returns ip addresses starting 192.0.2.5, increasing by 4
// for every vlanID.
func dutPort2IPv4(vlan uint8) string {
	return fmt.Sprintf("192.0.2.%d", vlan*4+5)
}

// atePort2IPv4 returns ip addresses starting 192.0.2.6, increasing by 4
// for every vlanID.
func atePort2IPv4(vlan uint8) string {
	return fmt.Sprintf("192.0.2.%d", vlan*4+6)
}

// cidr taks as input the IPv4 address and the Mask and returns the IP string in
// CIDR notation.
func cidr(ipv4 string, ones int) string {
	return ipv4 + "/" + strconv.Itoa(ones)
}

// filterPacketReceived uses ATE:EgressTracking bucket counters to create a map
// with bucket-label as the Key and the percentage of packets-received for that
// bucket as the Value.
func filterPacketReceived(t *testing.T, flow string, ate *ondatra.ATEDevice) map[string]float64 {
	t.Helper()

	flowPath := ate.Telemetry().Flow(flow)
	filters := flowPath.EgressTrackingAny().Get(t)

	inPkts := map[string]uint64{}
	for _, f := range filters {
		inPkts[f.GetFilter()] = f.GetCounters().GetInPkts()
	}
	inPct := map[string]float64{}
	total := flowPath.Counters().OutPkts().Get(t)
	for k, v := range inPkts {
		inPct[k] = (float64(v) / float64(total)) * 100.0
	}
	return inPct
}

// configureGRIBIClient configures a new GRIBI client with PRESERVE and FIB_ACK.
func configureGRIBIClient(t *testing.T, dut *ondatra.DUTDevice) *fluent.GRIBIClient {
	t.Helper()
	gribic := dut.RawAPIs().GRIBI().Default(t)

	// Configure the gRIBI client.
	c := fluent.NewClient()
	c.Connection().
		WithStub(gribic).
		WithRedundancyMode(fluent.ElectedPrimaryClient).
		WithInitialElectionID(1 /* low */, 0 /* hi */).
		WithPersistence().
		WithFIBACK()

	return c
}

// nextHopEntry configures a fluent.GRIBIEntry for a NextHopEntry.
func nextHopEntry(index uint64, networkInstance string, ipAddr string) fluent.GRIBIEntry {
	return fluent.NextHopEntry().
		WithNetworkInstance(networkInstance).
		WithIndex(index).
		WithIPAddress(ipAddr)
}

// nextHopGroupEntry configures a fluent.GRIBIEntry for a NextHopGroupEntry.
func nextHopGroupEntry(index uint64, networkInstance string, nhs []nhInfo) fluent.GRIBIEntry {
	x := fluent.NextHopGroupEntry().
		WithNetworkInstance(networkInstance).
		WithID(index)
	for _, nh := range nhs {
		x.AddNextHop(nh.index, nh.weight)
	}
	return x
}

// ipv4Entry configures a fluent.GRIBIEntry for an IPv4Entry.
func ipv4Entry(prefix string, networkInstance string, nhgIndex uint64, nextHopGroupNetworkInstance string) fluent.GRIBIEntry {
	return fluent.IPv4Entry().
		WithPrefix(prefix).
		WithNetworkInstance(networkInstance).
		WithNextHopGroup(nhgIndex).
		WithNextHopGroupNetworkInstance(nextHopGroupNetworkInstance)
}

// awaitTimeout calls a fluent client Await, adding a timeout to the context.
func awaitTimeout(ctx context.Context, c *fluent.GRIBIClient, t testing.TB, timeout time.Duration) error {
	t.Helper()
	subctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.Await(subctx, t)
}

// configSubinterfaceDUT configures the Sub Interfaces of an Interfaces,
// starting from Sub Interface 1. Each Subinterface is configured with a
// unique VlanID starting from 1 and an IP address. The starting IP Address
// for Subinterface(1) = dutPort.ip(1) = dutPort.ip + 4
func (a *attributes) configSubinterfaceDUT(t *testing.T, intf *telemetry.Interface) {
	t.Helper()

	for i := uint32(1); i <= a.numSubIntf; i++ {
		ip := a.ip(uint8(i))

		s := intf.GetOrCreateSubinterface(i)
		if *deviations.InterfaceEnabled {
			s.Enabled = ygot.Bool(true)
		}
		if *deviations.DeprecatedVlanID {
			s.GetOrCreateVlan().VlanId = telemetry.UnionUint16(i)
		} else {
			s.GetOrCreateVlan().GetOrCreateMatch().GetOrCreateSingleTagged().VlanId = ygot.Uint16(uint16(i))
		}
		s4 := s.GetOrCreateIpv4()
		if *deviations.InterfaceEnabled {
			s4.Enabled = ygot.Bool(true)
		}
		s4a := s4.GetOrCreateAddress(ip)
		s4a.PrefixLength = ygot.Uint8(a.IPv4Len)
		t.Logf("Adding DUT Subinterface with ID: %d, Vlan ID: %d and IPv4 address: %s", i, i, ip)
	}
}

// configInterfaceDUT configures the DUT interface with the provided IP Address.
// Sub Interfaces are also configured if numSubIntf > 0.
func (a *attributes) configInterfaceDUT(t *testing.T, d *ondatra.Config, p *ondatra.Port) {
	t.Helper()
	i := a.NewInterface(p.Name())

	a.configSubinterfaceDUT(t, i)
	intfPath := d.Interface(p.Name())
	intfPath.Replace(t, i)
	fptest.LogYgot(t, "DUT", intfPath, intfPath.Get(t))
}

// configureNetworkInstance creates new Network Instance and configures it, if provided,
// else configures the Default Network Instance.
func (a *attributes) configureNetworkInstance(t *testing.T, d *ondatra.Config, p *ondatra.Port) {
	t.Helper()
	addressFamilies := []telemetry.E_Types_ADDRESS_FAMILY{telemetry.Types_ADDRESS_FAMILY_IPV4}
	// Use default NI if not provided
	if a.networkInstance != "" {
		ni := &telemetry.NetworkInstance{
			Name:                   ygot.String(a.networkInstance),
			Enabled:                ygot.Bool(true),
			Type:                   telemetry.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF,
			EnabledAddressFamilies: addressFamilies,
		}
		i := ni.GetOrCreateInterface(p.Name())
		i.Interface = ygot.String(p.Name())
		i.Subinterface = ygot.Uint32(0)

		dni := d.NetworkInstance(a.networkInstance)
		dni.Replace(t, ni)
		fptest.LogYgot(t, "NI", dni, dni.Get(t))
	} else {
		dni := d.NetworkInstance(*deviations.DefaultNetworkInstance)
		dni.EnabledAddressFamilies().Replace(t, addressFamilies)
		dni.Interface(p.Name()).Replace(t, &telemetry.NetworkInstance_Interface{
			Id:           ygot.String(p.Name()),
			Interface:    ygot.String(p.Name()),
			Subinterface: ygot.Uint32(0),
		})
	}
}

// configureDUT configures a DUT port by configuring the NetworkInstance and the
// Interface + Sub Interfaces.
func (a *attributes) configureDUT(t *testing.T, d *ondatra.Config, dut *ondatra.DUTDevice) {
	t.Helper()
	p := dut.Port(t, a.Name)
	a.configureNetworkInstance(t, d, p)
	a.configInterfaceDUT(t, d, p)
}

// ConfigureATE configures Ethernet + IPv4 on the ATE. If the number of
// Subinterfaces(numSubIntf) > 0, we then create additional sub-interfaces
// each with a unique VlanID starting from 1. The IPv4 addresses start with
// ATE:Port.IPv4 and then nextIP(ATE:Port.IPv4, 4) for each sub interface
func (a *attributes) ConfigureATE(t *testing.T, top *ondatra.ATETopology, ate *ondatra.ATEDevice) {
	t.Helper()
	p := ate.Port(t, a.Name)

	ip := a.ip(0)
	gateway := a.gateway(0)

	intf := top.AddInterface(ip).WithPort(p)
	intf.IPv4().WithAddress(cidr(ip, 30))
	intf.IPv4().WithDefaultGateway(gateway)
	t.Logf("Adding ATE Ipv4 address: %s with gateway: %s", cidr(ip, 30), gateway)

	for i := uint32(1); i <= a.numSubIntf; i++ {
		ip = a.ip(uint8(i))
		gateway = a.gateway(uint8(i))
		intf := top.AddInterface(ip).WithPort(p)
		intf.IPv4().WithAddress(cidr(ip, 30))
		intf.IPv4().WithDefaultGateway(gateway)
		intf.Ethernet().WithVLANID(uint16(i))
		t.Logf("Adding ATE Ipv4 address: %s with gateway: %s and VlanID: %d", cidr(ip, 30), gateway, i)
	}
}

// testTraffic creates a traffic flow with ATE source & destination endpoints
// and configures a VlanID filter for output frames. The IPv4 header for the
// flow contains the DUT:Port1 address as source and the configured gRIBI-
// IndirectEntry as the destination. The function also takes as input a map of
// <VlanID::TrafficDistribution> that is wanted and compares it to the actual
// traffic test result.
func testTraffic(t *testing.T, ate *ondatra.ATEDevice, top *ondatra.ATETopology) map[string]float64 {
	allIntf := top.Interfaces()

	// ATE source endpoint.
	srcEndPoint := allIntf[atePort1.IPv4]

	// ATE destination endpoints.
	dstEndPoints := []ondatra.Endpoint{}
	for i := uint32(0); i <= atePort2.numSubIntf; i++ {
		dstIP := atePort2.ip(uint8(i))
		dstEndPoints = append(dstEndPoints, allIntf[dstIP])
	}

	// Configure Ethernet+IPv4 headers.
	ethHeader := ondatra.NewEthernetHeader()
	ipv4Header := ondatra.NewIPv4Header()
	ipv4Header.WithSrcAddress(dutPort1.IPv4)
	ipv4Header.DstAddressRange().
		WithMin(ipv4FlowIPStart).
		WithMax(ipv4FlowIPEnd).
		WithCount(256)

	// Ethernet header:
	//   - Destination MAC (6 octets)
	//   - Source MAC (6 octets)
	//   - Optional 802.1q VLAN tag (4 octets)
	//   - Frame size (2 octets)
	flow := ate.Traffic().NewFlow("flow").
		WithSrcEndpoints(srcEndPoint).
		WithDstEndpoints(dstEndPoints...).
		WithHeaders(ethHeader, ipv4Header)

	// VlanID is the last 12 bits in the 802.1q VLAN tag.
	// Offset for VlanID: ((6+6+4) * 8)-12 = 116.
	flow.EgressTracking().WithOffset(116).WithWidth(12).WithCount(18)

	// Run traffic for 2 minutes.
	ate.Traffic().Start(t, flow)
	time.Sleep(2 * time.Minute)
	ate.Traffic().Stop(t)

	// Verify total traffic loss is 0%.
	vd := tcheck.Equal(ate.Telemetry().Flow("flow").LossPct(), float32(0))
	if err := vd.Await(t, time.Minute); err != nil {
		t.Errorf("Packet loss: %v", err)
	}

	// Compare traffic distribution with the wanted results.
	results := filterPacketReceived(t, "flow", ate)
	t.Logf("Filters: %v", results)
	return results
}

// flushGRIBI deletes installed gRIBI routes for all Network Instances.
func flushGRIBI(t *testing.T, gRIBI *fluent.GRIBIClient) {
	t.Helper()
	_, err := gRIBI.Flush().
		WithElectionOverride().
		WithAllNetworkInstances().
		Send()
	if err != nil {
		t.Errorf("Cannot flush: %v", err)
	}
}

// aftNextHopWeights queries AFT telemetry using Get() and returns
// the weights. If not-found, an empty list is returned.
func aftNextHopWeights(t *testing.T, dut *ondatra.DUTDevice, nhg uint64, networkInstance string) []uint64 {
	aft := dut.Telemetry().NetworkInstance(networkInstance).Afts().Get(t)
	var nhgD *telemetry.NetworkInstance_Afts_NextHopGroup
	for _, nhgData := range aft.NextHopGroup {
		if nhgData.GetProgrammedId() == nhg {
			nhgD = nhgData
			break
		}
	}
	if nhgD == nil {
		return []uint64{}
	}

	got := []uint64{}
	for _, nhD := range nhgD.NextHop {
		got = append(got, nhD.GetWeight())
	}

	return got
}

// testBasicHierarchicalWeight tests and validates traffic through 4 Vlans.
func testBasicHierarchicalWeight(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice,
	ate *ondatra.ATEDevice, top *ondatra.ATETopology, gRIBI *fluent.GRIBIClient) {
	defaultVRF := *deviations.DefaultNetworkInstance

	// Set up NH#10, NH#11, NHG#2, IPv4Entry(192.0.2.111).
	nh10 := nextHopEntry(10, defaultVRF, atePort2.ip(1))
	nh11 := nextHopEntry(11, defaultVRF, atePort2.ip(2))
	nhg2 := nextHopGroupEntry(2, defaultVRF, []nhInfo{{index: 10, weight: 1}, {index: 11, weight: 3}})
	ipEntry2 := ipv4Entry(nhgIPv4EntryMap[2], defaultVRF, 2, defaultVRF)

	gRIBI.Modify().AddEntry(t, nh10, nh11, nhg2, ipEntry2)

	// Set up NH#100, NH#101, NHG#3, IPv4Entry(192.0.2.222).
	nh100 := nextHopEntry(100, defaultVRF, atePort2.ip(3))
	nh101 := nextHopEntry(101, defaultVRF, atePort2.ip(4))
	nhg3 := nextHopGroupEntry(3, defaultVRF, []nhInfo{{index: 100, weight: 2}, {index: 101, weight: 3}})
	ipEntry3 := ipv4Entry(nhgIPv4EntryMap[3], defaultVRF, 3, defaultVRF)

	gRIBI.Modify().AddEntry(t, nh100, nh101, nhg3, ipEntry3)

	// Set up NH#1, NH#2, NHG#1, IPv4Entry(203.0.113.0/24).
	nh1 := nextHopEntry(1, defaultVRF, nhEntryIP1)
	nh2 := nextHopEntry(2, defaultVRF, nhEntryIP2)
	nhg1 := nextHopGroupEntry(1, defaultVRF, []nhInfo{{index: 1, weight: 1}, {index: 2, weight: 3}})
	ipEntry1 := ipv4Entry(nhgIPv4EntryMap[1], nonDefaultVRF, 1, defaultVRF)

	gRIBI.Modify().AddEntry(t, nh1, nh2, nhg1, ipEntry1)

	if err := awaitTimeout(ctx, gRIBI, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via gRIBI, got err: %v", err)
	}

	// Validate entries were installed in FIB.
	for _, route := range nhgIPv4EntryMap {
		chk.HasResult(t, gRIBI.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(route).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// Test traffic flows correctly and
	wantWeights := map[string]float64{
		"1": 6.25,
		"2": 18.75,
		"3": 30,
		"4": 45,
	}
	t.Run("testTraffic", func(t *testing.T) {
		got := testTraffic(t, ate, top)
		if diff := cmp.Diff(wantWeights, got, cmpopts.EquateApprox(0, deviation)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
	})

	t.Run("validateAFTWeights", func(t *testing.T) {
		for nhg, weights := range map[uint64][]uint64{
			2: {1, 3},
			3: {2, 3},
		} {
			got := aftNextHopWeights(t, dut, nhg, defaultVRF)
			ok := cmp.Equal(weights, got, cmpopts.SortSlices(func(a, b uint64) bool { return a < b }))
			if !ok {
				t.Errorf("Valid weights not present for NI: %s, NHG: %d, got: %v, want: %v", defaultVRF, nhg, got, weights)
			}
		}
	})

	// Flush gRIBI routes after test.
	flushGRIBI(t, gRIBI)
}

// testHierarchicalWeightBoundaryScenario tests and validates traffic through all 18 Vlans.
func testHierarchicalWeightBoundaryScenario(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice,
	ate *ondatra.ATEDevice, top *ondatra.ATETopology, gRIBI *fluent.GRIBIClient) {
	defaultVRF := *deviations.DefaultNetworkInstance

	// Set up NH#10, NH#11, NHG#2, IPv4Entry(192.0.2.111).
	nh10 := nextHopEntry(10, defaultVRF, atePort2.ip(1))
	nh11 := nextHopEntry(11, defaultVRF, atePort2.ip(2))
	nhg2 := nextHopGroupEntry(2, defaultVRF, []nhInfo{{index: 10, weight: 2}, {index: 11, weight: 3}})
	ipEntry2 := ipv4Entry(nhgIPv4EntryMap[2], defaultVRF, 2, defaultVRF)

	gRIBI.Modify().AddEntry(t, nh10, nh11, nhg2, ipEntry2)

	// Set up NH#100..NH#116, NHG#3, IPv4Entry(192.0.2.222).
	nextHopWeights := []nhInfo{}
	nhIdx := uint64(100)
	gribiEntries := []fluent.GRIBIEntry{}
	for i := 0; i < 16; i++ {
		nh := nextHopEntry(nhIdx, defaultVRF, atePort2.ip(uint8(3+i)))
		gribiEntries = append(gribiEntries, nh)
		nextHopWeights = append(nextHopWeights, nhInfo{index: nhIdx, weight: 1})
		nhIdx++
	}
	nhg3 := nextHopGroupEntry(3, defaultVRF, nextHopWeights)
	ipEntry3 := ipv4Entry(nhgIPv4EntryMap[3], defaultVRF, 3, defaultVRF)
	gribiEntries = append(gribiEntries, nhg3, ipEntry3)

	gRIBI.Modify().AddEntry(t, gribiEntries...)

	// Set up NH#1, NH#2, NHG#1, IPv4Entry(203.0.113.0/24).
	nh1 := nextHopEntry(1, defaultVRF, nhEntryIP1)
	nh2 := nextHopEntry(2, defaultVRF, nhEntryIP2)
	nhg1 := nextHopGroupEntry(1, defaultVRF, []nhInfo{{index: 1, weight: 1}, {index: 2, weight: 31}})
	ipEntry1 := ipv4Entry(nhgIPv4EntryMap[1], nonDefaultVRF, 1, defaultVRF)

	gRIBI.Modify().AddEntry(t, nh1, nh2, nhg1, ipEntry1)

	if err := awaitTimeout(ctx, gRIBI, t, time.Minute); err != nil {
		t.Fatalf("Could not program entries via gRIBI, got err: %v", err)
	}

	// Validate entries were installed in FIB.
	for _, route := range nhgIPv4EntryMap {
		chk.HasResult(t, gRIBI.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(route).
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	wantWeights := map[string]float64{
		"1": 1.25,
		"2": 1.875,
	}
	// 6.05 weight for vlans 3 to 18.
	for i := 3; i <= 18; i++ {
		wantWeights[strconv.Itoa(i)] = 6.05
	}
	t.Run("testTraffic", func(t *testing.T) {
		got := testTraffic(t, ate, top)
		if diff := cmp.Diff(wantWeights, got, cmpopts.EquateApprox(0, deviation)); diff != "" {
			t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
		}
	})

	t.Run("validateAFTWeights", func(t *testing.T) {
		for nhg, weights := range map[uint64][]uint64{
			2: {2, 3},
			3: {1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		} {
			got := aftNextHopWeights(t, dut, nhg, defaultVRF)
			ok := cmp.Equal(weights, got, cmpopts.SortSlices(func(a, b uint64) bool { return a < b }))
			if !ok {
				t.Errorf("Valid weights not present for NI: %s, NHG: %d, got: %v, want: %v", defaultVRF, nhg, got, weights)
			}
		}
	})

	// Flush gRIBI routes after test.
	flushGRIBI(t, gRIBI)
}

func TestHierarchicalWeightResolution(t *testing.T) {
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	ctx := context.Background()

	// Configure DUT ports.
	dc := dut.Config()
	dutPort1.configureDUT(t, dc, dut)
	dutPort2.configureDUT(t, dc, dut)

	// Configure ATE ports and start Ethernet+IPv4.
	top := ate.Topology().New()
	atePort1.ConfigureATE(t, top, ate)
	atePort2.ConfigureATE(t, top, ate)
	top.Push(t)
	top.StartProtocols(t)

	// Configure gRIBI with FIB_ACK.
	gRIBI := configureGRIBIClient(t, dut)

	gRIBI.Start(ctx, t)
	defer gRIBI.Stop(t)
	gRIBI.StartSending(ctx, t)
	if err := awaitTimeout(ctx, gRIBI, t, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for gRIBI: %v", err)
	}

	// Flush existing gRIBI routes before test.
	flushGRIBI(t, gRIBI)

	t.Run("TestBasicHierarchicalWeight", func(t *testing.T) {
		testBasicHierarchicalWeight(ctx, t, dut, ate, top, gRIBI)
	})

	t.Run("TestHierarchicalWeightBoundaryScenario", func(t *testing.T) {
		testHierarchicalWeightBoundaryScenario(ctx, t, dut, ate, top, gRIBI)
	})

	top.StopProtocols(t)
}
