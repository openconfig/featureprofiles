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

package traceroute_packetin_with_vrf_selection_test

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/cisco-open/go-p4/p4rt_client"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/deviations"
	baseScenario "github.com/openconfig/featureprofiles/internal/encapfrr"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/vrfpolicy"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

// Settings for configuring the baseline testbed with the test
// topology.
//
// ATE port-1 <------> port-1 DUT
// DUT port-2 <------> port-2 ATE
// DUT port-3 <------> port-3 ATE
// DUT port-4 <------> port-4 ATE
// DUT port-5 <------> port-5 ATE
// DUT port-6 <------> port-6 ATE
// DUT port-7 <------> port-7 ATE
// DUT port-8 <------> port-8 ATE

const (
	plenIPv4                = 30
	plenIPv6                = 126
	dscpEncapA1             = 10
	dscpEncapB1             = 20
	dscpEncapNoMatch        = 30
	ipv4OuterSrc111Addr     = "198.51.100.111"
	ipv4OuterSrc222Addr     = "198.51.100.222"
	ipv4OuterSrcAddr        = "198.100.200.123"
	ipv4OuterDst111         = "192.51.100.64"
	ipv4OuterDst111WithMask = "192.51.100.0/24"
	ipv4InnerDst            = "138.0.11.8"
	noMatchEncapDest        = "20.0.0.1"
	maskLen24               = "24"
	maskLen126              = "124"
	maskLen32               = "32"
	niDecapTeVrf            = "DECAP_TE_VRF"
	niEncapTeVrfA           = "ENCAP_TE_VRF_A"
	niEncapTeVrfB           = "ENCAP_TE_VRF_B"
	niTeVrf111              = "TE_VRF_111"
	niTeVrf222              = "TE_VRF_222"
	niRepairVrf             = "REPAIR_VRF"
	niTransitVRF            = "TRANSIT_VRF"
	tolerance               = 0.02
	encapFlow               = "encapFlow"
	gribiIPv4EntryVRF1111   = "203.0.113.1"
	gribiIPv4EntryVRF1112   = "203.0.113.2"
	gribiIPv4EntryEncapVRF  = "138.0.11.0"
	gribiIPv6EntryEncapVRF  = "2001:db8::138:0:11:0"
	ipv6InnerDst            = "2001:db8::138:0:11:8"
	ipv6InnerDstNoEncap     = "2001:db8::20:0:0:1"

	dutAreaAddress     = "49.0001"
	dutSysID           = "1920.0000.2001"
	otgSysID1          = "640000000001"
	isisInstance       = "DEFAULT"
	otgIsisPort8LoopV4 = "203.0.113.10"
	otgIsisPort8LoopV6 = "2001:db8::203:0:113:10"
	dutAS              = 65501
	peerGrpName1       = "BGP-PEER-GROUP1"
	ipOverIPProtocol   = 4
)

var (
	p4InfoFile          = flag.String("p4info_file_location", "../../wbb.p4info.pb.txt", "Path to the p4info file.")
	streamName          = "p4rt"
	tracerouteSrcMAC    = "00:01:00:02:00:03"
	deviceID            = uint64(1)
	portID              = uint32(10)
	electionID          = uint64(100)
	MetadataIngressPort = uint32(1)
	MetadataEgressPort  = uint32(2)
	TTL1                = uint8(1)
	HopLimit1           = uint8(1)

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
		MAC:     "02:00:01:01:01:02",
		IPv6:    "2001:db8::192:0:2:6",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort3 = attrs.Attributes{
		Desc:    "dutPort3",
		IPv4:    "192.0.2.9",
		IPv6:    "2001:db8::192:0:2:9",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort3 = attrs.Attributes{
		Name:    "atePort3",
		IPv4:    "192.0.2.10",
		MAC:     "02:00:01:01:01:03",
		IPv6:    "2001:db8::192:0:2:a",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort4 = attrs.Attributes{
		Desc:    "dutPort4",
		IPv4:    "192.0.2.13",
		IPv6:    "2001:db8::192:0:2:d",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort4 = attrs.Attributes{
		Name:    "atePort4",
		IPv4:    "192.0.2.14",
		MAC:     "02:00:01:01:01:04",
		IPv6:    "2001:db8::192:0:2:e",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort5 = attrs.Attributes{
		Desc:    "dutPort5",
		IPv4:    "192.0.2.17",
		IPv6:    "2001:db8::192:0:2:11",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort5 = attrs.Attributes{
		Name:    "atePort5",
		IPv4:    "192.0.2.18",
		MAC:     "02:00:01:01:01:05",
		IPv6:    "2001:db8::192:0:2:12",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort6 = attrs.Attributes{
		Desc:    "dutPort6",
		IPv4:    "192.0.2.21",
		IPv6:    "2001:db8::192:0:2:15",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort6 = attrs.Attributes{
		Name:    "atePort6",
		IPv4:    "192.0.2.22",
		MAC:     "02:00:01:01:01:06",
		IPv6:    "2001:db8::192:0:2:16",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort7 = attrs.Attributes{
		Desc:    "dutPort7",
		IPv4:    "192.0.2.25",
		IPv6:    "2001:db8::192:0:2:19",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort7 = attrs.Attributes{
		Name:    "atePort7",
		IPv4:    "192.0.2.26",
		MAC:     "02:00:01:01:01:07",
		IPv6:    "2001:db8::192:0:2:1a",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	dutPort8 = attrs.Attributes{
		Desc:    "dutPort8",
		IPv4:    "192.0.2.29",
		IPv6:    "2001:db8::192:0:2:1d",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}
	atePort8 = attrs.Attributes{
		Name:    "atePort8",
		IPv4:    "192.0.2.30",
		MAC:     "02:00:01:01:01:08",
		IPv6:    "2001:db8::192:0:2:1e",
		IPv4Len: plenIPv4,
		IPv6Len: plenIPv6,
	}

	// loopbackIntfName string
	// TODO : https://github.com/open-traffic-generator/fp-testbed-juniper/issues/42
	// Below code will be uncommented once ixia issue is fixed.
	// tolerance        = 0.2

	portsMap = map[string]attrs.Attributes{
		"dutPort1": dutPort1,
		"atePort1": atePort1,
		"dutPort2": dutPort2,
		"atePort2": atePort2,
		"dutPort3": dutPort3,
		"atePort3": atePort3,
		"dutPort4": dutPort4,
		"atePort4": atePort4,
		"dutPort5": dutPort5,
		"atePort5": atePort5,
		"dutPort6": dutPort6,
		"atePort6": atePort6,
		"dutPort7": dutPort7,
		"atePort7": atePort7,
		"dutPort8": dutPort8,
		"atePort8": atePort8,
	}
	portsIPv4 = map[string]string{
		"dut:port1": "192.0.2.1",
		"ate:port1": "192.0.2.2",

		"dut:port2": "192.0.2.5",
		"ate:port2": "192.0.2.6",

		"dut:port3": "192.0.2.9",
		"ate:port3": "192.0.2.10",

		"dut:port4": "192.0.2.13",
		"ate:port4": "192.0.2.14",

		"dut:port5": "192.0.2.17",
		"ate:port5": "192.0.2.18",

		"dut:port6": "192.0.2.21",
		"ate:port6": "192.0.2.22",

		"dut:port7": "192.0.2.25",
		"ate:port7": "192.0.2.26",

		"dut:port8": "192.0.2.29",
		"ate:port8": "192.0.2.30",
	}
	portsIPv6 = map[string]string{
		"dut:port1": "2001:db8::192:0:2:1",
		"ate:port1": "2001:db8::192:0:2:2",

		"dut:port2": "2001:db8::192:0:2:5",
		"ate:port2": "2001:db8::192:0:2:6",

		"dut:port3": "2001:db8::192:0:2:9",
		"ate:port3": "2001:db8::192:0:2:a",

		"dut:port4": "2001:db8::192:0:2:d",
		"ate:port4": "2001:db8::192:0:2:e",

		"dut:port5": "2001:db8::192:0:2:11",
		"ate:port5": "2001:db8::192:0:2:12",

		"dut:port6": "2001:db8::192:0:2:15",
		"ate:port6": "2001:db8::192:0:2:16",

		"dut:port7": "2001:db8::192:0:2:19",
		"ate:port7": "2001:db8::192:0:2:1a",

		"dut:port8": "2001:db8::192:0:2:1d",
		"ate:port8": "2001:db8::192:0:2:1e",
	}
	otgPortDevices []gosnappi.Device
	dutlo0Attrs    = attrs.Attributes{
		Desc:    "Loopback ip",
		IPv4:    "203.0.113.11",
		IPv6:    "2001:db8::203:0:113:1",
		IPv4Len: 32,
		IPv6Len: 128,
	}
	loopbackIntfName string
	atePortNamelist  []string
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
	leader     *p4rt_client.P4RTClient
	follower   *p4rt_client.P4RTClient
	packetIO   PacketIO
}

type flowArgs struct {
	flowName                     string
	outHdrSrcIP, outHdrDstIP     string
	InnHdrSrcIP, InnHdrDstIP     string
	InnHdrSrcIPv6, InnHdrDstIPv6 string
	udp, isInnHdrV4              bool
	outHdrDscp                   []uint32
	proto                        uint32
}

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

func sortPorts(ports []*ondatra.Port) []*ondatra.Port {
	sort.Slice(ports, func(i, j int) bool {
		idi, idj := ports[i].ID(), ports[j].ID()
		li, lj := len(idi), len(idj)
		if li == lj {
			return idi < idj
		}
		return li < lj // "port2" < "port10"
	})
	return ports
}

// dutInterface builds a DUT interface ygot struct for a given port
// according to portsIPv4.  Returns nil if the port has no IP address
// mapping.
func dutInterface(p *ondatra.Port, dut *ondatra.DUTDevice, portIDx uint32) *oc.Interface {
	id := fmt.Sprintf("%s:%s", p.Device().ID(), p.ID())
	i := &oc.Interface{
		Name:        ygot.String(p.Name()),
		Description: ygot.String(p.String()),
		Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
		Id:          ygot.Uint32(portIDx),
	}
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
	}

	if p.PMD() == ondatra.PMD100GBASEFR {
		e := i.GetOrCreateEthernet()
		e.AutoNegotiate = ygot.Bool(false)
		e.DuplexMode = oc.Ethernet_DuplexMode_FULL
		e.PortSpeed = oc.IfEthernet_ETHERNET_SPEED_SPEED_100GB
	}

	ipv4, ok := portsIPv4[id]
	if !ok {
		return nil
	}
	ipv6, ok := portsIPv6[id]
	if !ok {
		return nil
	}
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	if deviations.InterfaceEnabled(dut) && !deviations.IPv4MissingEnabled(dut) {
		s4.Enabled = ygot.Bool(true)
	}

	a := s4.GetOrCreateAddress(ipv4)
	a.PrefixLength = ygot.Uint8(plenIPv4)
	s6 := s.GetOrCreateIpv6()
	if deviations.InterfaceEnabled(dut) {
		s6.Enabled = ygot.Bool(true)
	}
	a6 := s6.GetOrCreateAddress(ipv6)
	a6.PrefixLength = ygot.Uint8(plenIPv6)

	return i
}

// configureDUT configures all the interfaces on the DUT.
func configureDUT(t *testing.T, dut *ondatra.DUTDevice, dutPortList []*ondatra.Port) {
	dc := gnmi.OC()
	for idx, dp := range dutPortList {
		portIDx := portID + uint32(idx)
		if i := dutInterface(dp, dut, portIDx); i != nil {
			gnmi.Replace(t, dut, dc.Interface(dp.Name()).Config(), i)
		} else {
			t.Fatalf("No address found for port %v", dp)
		}
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, dp := range dut.Ports() {
			fptest.AssignToNetworkInstance(t, dut, dp.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}
	if deviations.ExplicitPortSpeed(dut) {
		for _, dp := range dut.Ports() {
			fptest.SetPortSpeed(t, dp)
		}
	}

	loopbackIntfName = netutil.LoopbackInterface(t, dut, 0)
	lo0 := gnmi.OC().Interface(loopbackIntfName).Subinterface(0)
	ipv4Addrs := gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())
	ipv6Addrs := gnmi.LookupAll(t, dut, lo0.Ipv6().AddressAny().State())
	if len(ipv4Addrs) == 0 && len(ipv6Addrs) == 0 {
		loop1 := dutlo0Attrs.NewOCInterface(loopbackIntfName, dut)
		loop1.Type = oc.IETFInterfaces_InterfaceType_softwareLoopback
		gnmi.Update(t, dut, dc.Interface(loopbackIntfName).Config(), loop1)
	} else {
		v4, ok := ipv4Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv4 = v4.GetIp()
		}
		v6, ok := ipv6Addrs[0].Val()
		if ok {
			dutlo0Attrs.IPv6 = v6.GetIp()
		}
		t.Logf("Got DUT IPv4 loopback address: %v", dutlo0Attrs.IPv4)
		t.Logf("Got DUT IPv6 loopback address: %v", dutlo0Attrs.IPv6)
	}
}

// configureAdditionalGribiAft configures additional AFT entries for Gribi.
func configureAdditionalGribiAft(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(3000).WithNextHopNetworkInstance(niRepairVrf),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(3000).AddNextHop(3000, 1),

		fluent.IPv4Entry().WithNetworkInstance(niRepairVrf).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF1111+"/"+maskLen32).WithNextHopGroup(1000),

		fluent.IPv4Entry().WithNetworkInstance(niRepairVrf).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF1112+"/"+maskLen32).WithNextHopGroup(1001),

		fluent.IPv6Entry().WithNetworkInstance(niEncapTeVrfA).
			WithPrefix(gribiIPv6EntryEncapVRF+"/"+maskLen126).WithNextHopGroup(101).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	defaultVRFIPList := []string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112}
	for ip := range defaultVRFIPList {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(defaultVRFIPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
	// Programming AFT entries for prefixes in ENCAP_TE_VRF_A
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(200).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(200).AddNextHop(200, 1),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	args.client.Modify().AddEntry(t,
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(102).AddNextHop(101, 3).AddNextHop(102, 1).WithBackupNHG(200),
		fluent.IPv4Entry().WithNetworkInstance(niEncapTeVrfB).
			WithPrefix(gribiIPv4EntryEncapVRF+"/"+maskLen24).WithNextHopGroup(102).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.IPv6Entry().WithNetworkInstance(niEncapTeVrfB).
			WithPrefix(gribiIPv6EntryEncapVRF+"/"+maskLen126).WithNextHopGroup(102).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)

	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(gribiIPv4EntryEncapVRF+"/24").
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().
			WithIPv6Operation(gribiIPv6EntryEncapVRF+"/124").
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)

}

// configureGribiRoute configures Gribi route for prefix
func configureGribiRoute(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs, prefWithMask string) {
	t.Helper()
	// Using gRIBI, install an  IPv4Entry for the prefix 192.51.100.1/24 that points to a
	// NextHopGroup that contains a single NextHop that specifies decapsulating the IPv4
	// header and specifies the DEFAULT network instance.This IPv4Entry should be installed
	// into the DECAP_TE_VRF.

	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(3001).WithDecapsulateHeader(fluent.IPinIP),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(3001).AddNextHop(3001, 1),
		fluent.IPv4Entry().WithNetworkInstance(niDecapTeVrf).
			WithPrefix(prefWithMask).WithNextHopGroup(3001).
			WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	chk.HasResult(t, args.client.Results(t),
		fluent.OperationResult().WithIPv4Operation(prefWithMask).WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).AsResult(),
		chk.IgnoreOperationID(),
	)
}

// configureISIS configures ISIS on the DUT.
func configureISIS(t *testing.T, dut *ondatra.DUTDevice, intfName, dutAreaAddress, dutSysID string) {
	t.Helper()
	d := &oc.Root{}
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()

	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)

	lspBit := globalISIS.GetOrCreateLspBit().GetOrCreateOverloadBit()
	lspBit.SetBit = ygot.Bool(false)
	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC

	isisIntf := isis.GetOrCreateInterface(intfName)
	isisIntf.GetOrCreateInterfaceRef().Interface = ygot.String(intfName)
	isisIntf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)

	if deviations.InterfaceRefConfigUnsupported(dut) {
		isisIntf.InterfaceRef = nil
	}

	isisIntf.Enabled = ygot.Bool(true)
	isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	isisIntf.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	if deviations.ISISInterfaceAfiUnsupported(dut) {
		isisIntf.Af = nil
	}
	isisIntfLevel := isisIntf.GetOrCreateLevel(2)
	isisIntfLevel.Enabled = ygot.Bool(true)

	isisIntfLevelAfiv4 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)

	isisIntfLevelAfiv4.Enabled = ygot.Bool(true)
	isisIntfLevelAfiv6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)

	isisIntfLevelAfiv4.Metric = ygot.Uint32(20)
	isisIntfLevelAfiv6.Metric = ygot.Uint32(20)

	isisIntfLevelAfiv6.Enabled = ygot.Bool(true)
	if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
		isisIntfLevelAfiv4.Enabled = nil
		isisIntfLevelAfiv6.Enabled = nil
	}
	gnmi.Update(t, dut, gnmi.OC().Config(), d)
}

// bgpCreateNbr creates BGP neighbor configuration
func bgpCreateNbr(localAs uint32, dut *ondatra.DUTDevice) *oc.NetworkInstance_Protocol {
	dutOcRoot := &oc.Root{}
	ni1 := dutOcRoot.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niProto := ni1.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	bgp := niProto.GetOrCreateBgp()

	global := bgp.GetOrCreateGlobal()
	global.RouterId = ygot.String(dutlo0Attrs.IPv4)
	global.As = ygot.Uint32(localAs)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST).Enabled = ygot.Bool(true)
	global.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST).Enabled = ygot.Bool(true)
	pg1 := bgp.GetOrCreatePeerGroup(peerGrpName1)
	pg1.PeerAs = ygot.Uint32(localAs)

	bgpNbr := bgp.GetOrCreateNeighbor(otgIsisPort8LoopV4)
	bgpNbr.PeerGroup = ygot.String(peerGrpName1)
	bgpNbr.PeerAs = ygot.Uint32(localAs)
	bgpNbr.Enabled = ygot.Bool(true)
	bgpNbrT := bgpNbr.GetOrCreateTransport()
	bgpNbrT.LocalAddress = ygot.String(dutlo0Attrs.IPv4)
	af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	af4.Enabled = ygot.Bool(true)
	af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	af6.Enabled = ygot.Bool(true)

	return niProto
}

// verifyISISTelemetry verifies ISIS telemetry.
func verifyISISTelemetry(t *testing.T, dut *ondatra.DUTDevice, dutIntf string) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()

	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		dutIntf = dutIntf + ".0"
	}
	nbrPath := statePath.Interface(dutIntf)
	query := nbrPath.LevelAny().AdjacencyAny().AdjacencyState().State()
	_, ok := gnmi.WatchAll(t, dut, query, time.Minute, func(val *ygnmi.Value[oc.E_Isis_IsisInterfaceAdjState]) bool {
		state, present := val.Val()
		return present && state == oc.Isis_IsisInterfaceAdjState_UP
	}).Await(t)
	if !ok {
		t.Logf("IS-IS state on %v has no adjacencies", dutIntf)
		t.Fatal("No IS-IS adjacencies reported.")
	}
}

// verifyBgpTelemetry verifies BGP telemetry.
func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	nbrPath := bgpPath.Neighbor(otgIsisPort8LoopV4)
	// Get BGP adjacency state.
	t.Logf("Waiting for BGP neighbor to establish...")
	var status *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]
	status, ok := gnmi.Watch(t, dut, nbrPath.SessionState().State(), time.Minute, func(val *ygnmi.Value[oc.E_Bgp_Neighbor_SessionState]) bool {
		state, ok := val.Val()
		return ok && state == oc.Bgp_Neighbor_SessionState_ESTABLISHED
	}).Await(t)
	if !ok {
		fptest.LogQuery(t, "BGP reported state", nbrPath.State(), gnmi.Get(t, dut, nbrPath.State()))
		t.Fatal("No BGP neighbor formed")
	}
	state, _ := status.Val()
	t.Logf("BGP adjacency for %s: %v", otgIsisPort8LoopV4, state)
	if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
		t.Errorf("BGP peer %s status got %d, want %d", otgIsisPort8LoopV4, state, want)
	}
}

// configureOTG configures the topology of the ATE.
func configureOTG(t testing.TB, otg *otg.OTG, atePorts []*ondatra.Port) gosnappi.Config {
	t.Helper()
	t.Logf("configureOTG")
	config := gosnappi.NewConfig()
	pmd100GFRPorts := []string{}
	for i, ap := range atePorts {
		if ap.PMD() == ondatra.PMD100GBASEFR {
			pmd100GFRPorts = append(pmd100GFRPorts, ap.ID())
		}
		// DUT and ATE ports are connected by the same names.
		dutid := fmt.Sprintf("dut:%s", ap.ID())
		ateid := fmt.Sprintf("ate:%s", ap.ID())

		port := config.Ports().Add().SetName(ap.ID())
		atePortNamelist = append(atePortNamelist, port.Name())
		portName := fmt.Sprintf("atePort%s", strconv.Itoa(i+1))
		dev := config.Devices().Add().SetName(portName)
		macAddress := portsMap[portName].MAC
		eth := dev.Ethernets().Add().SetName(portName + ".Eth").SetMac(macAddress)
		eth.Connection().SetPortName(port.Name())
		eth.Ipv4Addresses().Add().SetName(portName + ".IPv4").
			SetAddress(portsIPv4[ateid]).SetGateway(portsIPv4[dutid]).
			SetPrefix(plenIPv4)
		eth.Ipv6Addresses().Add().SetName(portName + ".IPv6").
			SetAddress(portsIPv6[ateid]).SetGateway(portsIPv6[dutid]).
			SetPrefix(plenIPv6)

		otgPortDevices = append(otgPortDevices, dev)
		if i == 7 {
			iDut8LoopV4 := dev.Ipv4Loopbacks().Add().SetName("Port8LoopV4").SetEthName(eth.Name())
			iDut8LoopV4.SetAddress(otgIsisPort8LoopV4)
			iDut8LoopV6 := dev.Ipv6Loopbacks().Add().SetName("Port8LoopV6").SetEthName(eth.Name())
			iDut8LoopV6.SetAddress(otgIsisPort8LoopV6)
			isisDut := dev.Isis().SetName("ISIS1").SetSystemId(otgSysID1)
			isisDut.Basic().SetIpv4TeRouterId(portsIPv4[ateid]).SetHostname(isisDut.Name()).SetLearnedLspFilter(true)
			isisDut.Interfaces().Add().SetEthName(dev.Ethernets().Items()[0].Name()).
				SetName("devIsisInt1").
				SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
				SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

			// Advertise OTG Port8 loopback address via ISIS.
			isisPort2V4 := dev.Isis().V4Routes().Add().SetName("ISISPort8V4").SetLinkMetric(10)
			isisPort2V4.Addresses().Add().SetAddress(otgIsisPort8LoopV4).SetPrefix(32)
			isisPort2V6 := dev.Isis().V6Routes().Add().SetName("ISISPort8V6").SetLinkMetric(10)
			isisPort2V6.Addresses().Add().SetAddress(otgIsisPort8LoopV6).SetPrefix(uint32(128))
			iDutBgp := dev.Bgp().SetRouterId(otgIsisPort8LoopV4)
			iDutBgp4Peer := iDutBgp.Ipv4Interfaces().Add().SetIpv4Name(iDut8LoopV4.Name()).Peers().Add().SetName(ap.ID() + ".BGP4.peer")
			iDutBgp4Peer.SetPeerAddress(dutlo0Attrs.IPv4).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
			iDutBgp4Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
			iDutBgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

			bgpNeti1Bgp4PeerRoutes := iDutBgp4Peer.V4Routes().Add().SetName(port.Name() + ".BGP4.Route")
			bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(otgIsisPort8LoopV4).
				SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
				SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).
				Advanced().SetLocalPreference(100).SetIncludeLocalPreference(true)
			bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(ipv4InnerDst).SetPrefix(32).
				SetCount(1).SetStep(1)
			bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(noMatchEncapDest).SetPrefix(32).
				SetCount(1).SetStep(1)

			bgpNeti1Bgp6PeerRoutes := iDutBgp4Peer.V6Routes().Add().SetName(atePort8.Name + ".BGP6.Route")
			bgpNeti1Bgp6PeerRoutes.SetNextHopIpv6Address(otgIsisPort8LoopV6).
				SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
				SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).
				Advanced().SetLocalPreference(100).SetIncludeLocalPreference(true)
			bgpNeti1Bgp6PeerRoutes.Addresses().Add().SetAddress(ipv6InnerDst).SetPrefix(128).
				SetCount(1).SetStep(1)
			bgpNeti1Bgp6PeerRoutes.Addresses().Add().SetAddress(ipv6InnerDstNoEncap).SetPrefix(128).
				SetCount(1).SetStep(1)

		}

	}
	config.Captures().Add().SetName("packetCapture").
		SetPortNames([]string{atePortNamelist[1], atePortNamelist[2], atePortNamelist[3], atePortNamelist[4],
			atePortNamelist[5], atePortNamelist[6], atePortNamelist[7]}).
		SetFormat(gosnappi.CaptureFormat.PCAP)

	// Disable FEC for 100G-FR ports because Novus does not support it.
	if len(pmd100GFRPorts) > 0 {
		l1Settings := config.Layer1().Add().SetName("L1").SetPortNames(pmd100GFRPorts)
		l1Settings.SetAutoNegotiate(true).SetIeeeMediaDefaults(false).SetSpeed("speed_100_gbps")
		autoNegotiate := l1Settings.AutoNegotiation()
		autoNegotiate.SetRsFec(false)
	}

	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
	pb, _ := config.Marshal().ToProto()
	t.Log(pb.GetCaptures())
	return config
}

func startCapture(t *testing.T, args *testArgs, capturePortList []string) gosnappi.ControlState {
	t.Helper()
	args.otgConfig.Captures().Clear()
	args.otgConfig.Captures().Add().SetName("packetCapture").
		SetPortNames(capturePortList).
		SetFormat(gosnappi.CaptureFormat.PCAP)
	args.otg.PushConfig(t, args.otgConfig)
	time.Sleep(30 * time.Second)
	args.otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	args.otg.SetControlState(t, cs)
	return cs
}

func verifyPortStatus(t *testing.T, args *testArgs, portList []string, portStatus bool) {
	wantStatus := oc.Interface_OperStatus_UP
	if !portStatus {
		wantStatus = oc.Interface_OperStatus_DOWN
	}
	for _, port := range portList {
		p := args.dut.Port(t, port)
		t.Log("Check for port status")
		gnmi.Await(t, args.dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), 1*time.Minute, wantStatus)
		operStatus := gnmi.Get(t, args.dut, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if operStatus != wantStatus {
			t.Errorf("Get(DUT %v oper status): got %v, want %v", port, operStatus, wantStatus)
		}
	}
}

func validateTrafficDistribution(t *testing.T, ate *ondatra.ATEDevice, wantWeights []float64, gotWeights []float64) {
	t.Log("Verify packet load balancing as per the programmed weight")
	t.Log("got ratio:", gotWeights)
	t.Log("want ratio:", wantWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, tolerance)); diff != "" {
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}
}

// testGribiMatchNoSourceNoProtocolMacthDSCP is to test based on packet which doesn't match source IP and protocol
// but match DSCP value
// Test-1
func testGribiMatchNoSourceNoProtocolMacthDSCP(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Log("Flush existing gRIBI routes before test.")
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}

	// Configure GRIBi baseline AFTs.
	baseScenario.ConfigureBaseGribiRoutes(ctx, t, dut, args.client)
	configureAdditionalGribiAft(ctx, t, dut, args)

	baseCapturePortList := []string{atePortNamelist[1], atePortNamelist[5]}
	EgressPortMap := map[string]bool{"11": true, "12": true, "13": true, "14": false, "15": true, "16": false, "17": false}
	LoadBalancePercent := []float64{0.0156, 0.0468, 0.1875, 0, 0.75, 0, 0}
	flow := []*flowArgs{{flowName: "flow4in4",
		outHdrSrcIP: ipv4OuterSrcAddr, outHdrDstIP: ipv4InnerDst, outHdrDscp: []uint32{dscpEncapA1},
		InnHdrSrcIP: ipv4OuterSrcAddr, InnHdrDstIP: ipv4InnerDst, isInnHdrV4: true, udp: true}}
	captureState := startCapture(t, args, baseCapturePortList)
	gotWeights := testPacket(t, args, captureState, flow, EgressPortMap)
	validateTrafficDistribution(t, args.ate, LoadBalancePercent, gotWeights)
}

// testTunnelTrafficMatchDefaultTerm is to test Tunnel traffic match default term
// Test-2
func testTunnelTrafficMatchDefaultTerm(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Log("Flush existing gRIBI routes before test.")
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}

	// Configure GRIBi baseline AFTs.
	baseScenario.ConfigureBaseGribiRoutes(ctx, t, dut, args.client)
	configureAdditionalGribiAft(ctx, t, dut, args)

	baseCapturePortList := []string{atePortNamelist[1], atePortNamelist[5]}
	EgressPortMap := map[string]bool{"11": false, "12": false, "13": false, "14": false, "15": false, "16": false, "17": true}
	LoadBalancePercent := []float64{0, 0, 0, 0, 0, 0, 1}
	flow := []*flowArgs{{flowName: "flow4in4",
		outHdrSrcIP: ipv4OuterSrcAddr, outHdrDstIP: noMatchEncapDest,
		InnHdrSrcIP: ipv4OuterSrcAddr, InnHdrDstIP: noMatchEncapDest, isInnHdrV4: true, udp: true}}
	captureState := startCapture(t, args, baseCapturePortList)
	gotWeights := testPacket(t, args, captureState, flow, EgressPortMap)
	validateTrafficDistribution(t, args.ate, LoadBalancePercent, gotWeights)
}

// testGribiDecapMatchSrcProtoDSCP is to test Gribi decap match src proto DSCP
// Test-3
func testGribiDecapMatchSrcProtoDSCP(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Log("Flush existing gRIBI routes before test.")
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}

	// Configure GRIBi baseline AFTs.
	baseScenario.ConfigureBaseGribiRoutes(ctx, t, dut, args.client)
	configureAdditionalGribiAft(ctx, t, dut, args)
	baseCapturePortList := []string{atePortNamelist[1], atePortNamelist[5]}
	EgressPortMap := map[string]bool{"11": true, "12": true, "13": true, "14": false, "15": false, "16": false, "17": false}
	LoadBalancePercent := []float64{0.0625, 0.1875, 0.75, 0, 0, 0, 0}
	flow := []*flowArgs{{flowName: "flow4in4",
		outHdrSrcIP: ipv4OuterSrc111Addr, outHdrDstIP: gribiIPv4EntryVRF1111, outHdrDscp: []uint32{dscpEncapA1},
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst, isInnHdrV4: true}}
	captureState := startCapture(t, args, baseCapturePortList)
	gotWeights := testPacket(t, args, captureState, flow, EgressPortMap)
	validateTrafficDistribution(t, args.ate, LoadBalancePercent, gotWeights)
}

// testTunnelTrafficNoDecap is to test Tunnel traffic no decap
// Test-6
func testTunnelTrafficNoDecap(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Helper()
	baseCapturePortList := []string{atePortNamelist[1], atePortNamelist[5]}
	EgressPortMap := map[string]bool{"11": false, "12": false, "13": false, "14": false, "15": false, "16": false, "17": true}
	LoadBalancePercent := []float64{0, 0, 0, 0, 0, 0, 1}

	cases := []struct {
		desc           string
		prefixWithMask string
	}{{
		desc:           "Mask Length 24",
		prefixWithMask: "192.51.100.0/24",
	}, {
		desc:           "Mask Length 32",
		prefixWithMask: "192.51.100.64/32",
	}, {
		desc:           "Mask Length 28",
		prefixWithMask: "192.51.100.64/28",
	}, {
		desc:           "Mask Length 22",
		prefixWithMask: "192.51.100.0/22",
	}}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Log("Flush existing gRIBI routes before test.")
			if err := gribi.FlushAll(args.client); err != nil {
				t.Fatal(err)
			}

			// Configure GRIBi baseline AFTs.
			baseScenario.ConfigureBaseGribiRoutes(ctx, t, dut, args.client)
			configureAdditionalGribiAft(ctx, t, dut, args)

			t.Run("Program gRIBi route for Prefix "+tc.prefixWithMask, func(t *testing.T) {
				configureGribiRoute(ctx, t, dut, args, tc.prefixWithMask)
			})
			t.Run("Create ip-in-ip and ipv6-in-ip flows, send traffic and verify decap functionality",
				func(t *testing.T) {
					// Send both 6in4 and 4in4 packets. Verify that the packets have their outer
					// v4 header stripped and are forwarded according to the route in the DEFAULT
					// VRF that matches the inner IP address.
					flow := []*flowArgs{{flowName: "flow4in4",
						outHdrSrcIP: ipv4OuterSrc111Addr, outHdrDstIP: ipv4OuterDst111, outHdrDscp: []uint32{dscpEncapNoMatch},
						InnHdrSrcIP: ipv4OuterSrcAddr, InnHdrDstIP: ipv4InnerDst, isInnHdrV4: true},
						{flowName: "flow6in4",
							outHdrSrcIP: ipv4OuterSrc111Addr, outHdrDstIP: ipv4OuterDst111, outHdrDscp: []uint32{dscpEncapNoMatch},
							InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: ipv6InnerDst, isInnHdrV4: false}}
					captureState := startCapture(t, args, baseCapturePortList)
					gotWeights := testPacket(t, args, captureState, flow, EgressPortMap)
					validateTrafficDistribution(t, args.ate, LoadBalancePercent, gotWeights)
				})
		})
	}
}

// testTunnelTrafficDecapEncap is to test Tunnel traffic decap encap
// Test-9
func testTunnelTrafficDecapEncap(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Log("Flush existing gRIBI routes before test.")
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}

	// Configure GRIBi baseline AFTs.
	baseScenario.ConfigureBaseGribiRoutes(ctx, t, dut, args.client)
	configureAdditionalGribiAft(ctx, t, dut, args)
	configureGribiRoute(ctx, t, dut, args, ipv4OuterDst111WithMask)

	baseCapturePortList := []string{atePortNamelist[1], atePortNamelist[5]}
	EgressPortMap := map[string]bool{"11": true, "12": true, "13": true, "14": false, "15": true, "16": false, "17": false}
	LoadBalancePercent := []float64{0.0156, 0.0468, 0.1875, 0, 0.75, 0, 0}
	flow := []*flowArgs{{flowName: "flow4in4",
		outHdrSrcIP: ipv4OuterSrc222Addr, outHdrDstIP: ipv4OuterDst111, outHdrDscp: []uint32{dscpEncapA1},
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst, isInnHdrV4: true},
		{flowName: "flow6in4",
			outHdrSrcIP: ipv4OuterSrc111Addr, outHdrDstIP: ipv4OuterDst111, outHdrDscp: []uint32{dscpEncapA1},
			InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: ipv6InnerDst, isInnHdrV4: false}}
	captureState := startCapture(t, args, baseCapturePortList)
	gotWeights := testPacket(t, args, captureState, flow, EgressPortMap)
	validateTrafficDistribution(t, args.ate, LoadBalancePercent, gotWeights)

	LoadBalancePercent = []float64{0.0468, 0.1406, 0.5625, 0, 0.25, 0, 0}
	flow = []*flowArgs{{flowName: "flow4in4",
		outHdrSrcIP: ipv4OuterSrc111Addr, outHdrDstIP: ipv4OuterDst111, outHdrDscp: []uint32{dscpEncapB1},
		InnHdrSrcIP: atePort1.IPv4, InnHdrDstIP: ipv4InnerDst, isInnHdrV4: true},
		{flowName: "flow6in4",
			outHdrSrcIP: ipv4OuterSrc222Addr, outHdrDstIP: ipv4OuterDst111, outHdrDscp: []uint32{dscpEncapB1},
			InnHdrSrcIPv6: atePort1.IPv6, InnHdrDstIPv6: ipv6InnerDst, isInnHdrV4: false}}
	captureState = startCapture(t, args, baseCapturePortList)
	gotWeights = testPacket(t, args, captureState, flow, EgressPortMap)
	validateTrafficDistribution(t, args.ate, LoadBalancePercent, gotWeights)
}

// testTraceRoute  is to test Test FRR behaviors with encapsulation scenarios
func TestTraceRoute(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")
	ate := ondatra.ATE(t, "ate")
	otg := ate.OTG()
	gribic := dut.RawAPIs().GRIBI(t)
	top := gosnappi.NewConfig()
	dutPorts := sortPorts(dut.Ports())[0:8]
	atePorts := sortPorts(ate.Ports())[0:8]

	t.Log("Configure Default Network Instance")
	fptest.ConfigureDefaultNetworkInstance(t, dut)

	if deviations.BackupNHGRequiresVrfWithDecap(dut) {
		d := &oc.Root{}
		ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
		pf := ni.GetOrCreatePolicyForwarding()
		fp1 := pf.GetOrCreatePolicy("match-ipip")
		fp1.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)
		fp1.GetOrCreateRule(1).GetOrCreateIpv4().Protocol = oc.UnionUint8(ipOverIPProtocol)
		fp1.GetOrCreateRule(1).GetOrCreateAction().NetworkInstance = ygot.String(deviations.DefaultNetworkInstance(dut))
		p1 := dut.Port(t, "port1")
		intf := pf.GetOrCreateInterface(p1.Name())
		intf.ApplyVrfSelectionPolicy = ygot.String("match-ipip")
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding().Config(), pf)
	}

	t.Run("Configure Interface on DUT", func(t *testing.T) {
		configureDUT(t, dut, dutPorts)
	})

	t.Log("Apply vrf selection policy_c to DUT port-1")
	vrfpolicy.ConfigureVRFSelectionPolicy(t, dut, vrfpolicy.VRFPolicyC)

	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		// staticARPWithMagicUniversalIP(t, dut)
		baseScenario.StaticARPWithMagicUniversalIP(t, dut)
	}

	t.Log("Install BGP route resolved by ISIS.")
	t.Log("Configure ISIS on DUT")
	configureISIS(t, dut, dut.Port(t, "port8").Name(), dutAreaAddress, dutSysID)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Delete(t, dut, dutConfPath.Config())
	dutConf := bgpCreateNbr(dutAS, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

	var otgConfig gosnappi.Config
	t.Run("Configure OTG", func(t *testing.T) {
		otgConfig = configureOTG(t, otg, atePorts)
	})

	// Connect gRIBI client to DUT referred to as gRIBI - using PRESERVE persistence and
	// SINGLE_PRIMARY mode, with FIB ACK requested. Specify gRIBI as the leader.
	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(1, 0).
		WithFIBACK().WithRedundancyMode(fluent.ElectedPrimaryClient)
	client.Start(ctx, t)

	client.StartSending(ctx, t)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Fatalf("Await got error during session negotiation for clientA: %v", err)
	}
	eID := gribi.BecomeLeader(t, client)

	leader := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := leader.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}

	follower := p4rt_client.NewP4RTClient(&p4rt_client.P4RTClientParameters{})
	if err := follower.P4rtClientSet(dut.RawAPIs().P4RT(t)); err != nil {
		t.Fatalf("Could not initialize p4rt client: %v", err)
	}
	args := &testArgs{
		ctx:        ctx,
		client:     client,
		dut:        dut,
		ate:        ate,
		otgConfig:  otgConfig,
		top:        top,
		electionID: eID,
		otg:        otg,
		leader:     leader,
		follower:   follower,
	}
	t.Log("Configure gRIBI routes")
	t.Log("Flush existing gRIBI routes before test.")
	if err := gribi.FlushAll(client); err != nil {
		t.Fatal(err)
	}

	t.Log("Verify whether the ports are in up state")
	portList := []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"}
	verifyPortStatus(t, args, portList, true)

	t.Log("Verify ISIS telemetry")
	verifyISISTelemetry(t, dut, dut.Port(t, "port8").Name())
	t.Log("Verify BGP telemetry")
	verifyBgpTelemetry(t, dut)

	t.Run("Test-1: Match on DSCP, no Source and no Protocol", func(t *testing.T) {
		testGribiMatchNoSourceNoProtocolMacthDSCP(ctx, t, dut, args)
	})

	t.Log("Delete vrf selection policy C and Apply vrf selectioin policy W")
	vrfpolicy.ConfigureVRFSelectionPolicy(t, dut, vrfpolicy.VRFPolicyW)

	t.Run("Test-2: Match on default term and send to default VRF", func(t *testing.T) {
		testTunnelTrafficMatchDefaultTerm(ctx, t, dut, args)
	})
	t.Run("Test-3: Match on source, protocol and DSCP, VRF_DECAP hit -> VRF_ENCAP_A miss -> DEFAULT", func(t *testing.T) {
		testGribiDecapMatchSrcProtoDSCP(ctx, t, dut, args)
	})
	// Below test case will implement later
	/*
		t.Run("Test-4: Tests that traceroute respects transit FRR", func(t *testing.T) {

		})
		t.Run("Test-5: Tests that traceroute respects transit FRR when the backup is also unviable.", func(t *testing.T) {

		})*/
	t.Run("Test-6: Tunneled traffic with no decap", func(t *testing.T) {
		testTunnelTrafficNoDecap(ctx, t, dut, args)
	})
	// Below test case will implement later
	/*
		t.Run("Test-7: Encap failure cases (TBD on confirmation)", func(t *testing.T) {

		})
		t.Run("Test-8: Tests that traceroute for a packet with a route lookup miss has an unset target_egress_port.", func(t *testing.T) {

		})*/
	t.Run("Test-9: Decap then encap", func(t *testing.T) {
		testTunnelTrafficDecapEncap(ctx, t, dut, args)
	})
}
