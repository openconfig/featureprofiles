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

package encap_frr_with_reencap_vrf_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/open-traffic-generator/snappi/gosnappi"
	"github.com/openconfig/featureprofiles/internal/attrs"
	"github.com/openconfig/featureprofiles/internal/cfgplugins"
	"github.com/openconfig/featureprofiles/internal/deviations"
	baseScenario "github.com/openconfig/featureprofiles/internal/encapfrr"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
	"github.com/openconfig/featureprofiles/internal/helpers"
	"github.com/openconfig/featureprofiles/internal/otgutils"
	"github.com/openconfig/featureprofiles/internal/vrfpolicy"
	"github.com/openconfig/gribigo/chk"
	"github.com/openconfig/gribigo/constants"
	"github.com/openconfig/gribigo/fluent"
	"github.com/openconfig/ondatra"
	"github.com/openconfig/ondatra/gnmi"
	"github.com/openconfig/ondatra/gnmi/oc"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// 4-Link Testbed Topology & Failure Triage Guide (TESTBED_DUT_ATE_4LINKS):
//
//   ATE port-1 <------> port-1 DUT  Bidirectional native L3 link (always UP):
//                                   - ATE -> DUT (Rx): test ingress (DSCP 10 -> ENCAP_TE_VRF_A)
//                                   - DUT <-> ATE (Tx/Rx): DEFAULT VRF IS-IS L2 + iBGP (203.0.113.10)
//                                   - DUT -> ATE (Tx): unencapsulated DEFAULT VRF decap fallback
//   DUT port-2 -------> port-2 ATE  Tunnel 1 Primary (203.0.113.1, 25% baseline):
//                                   - NHG#11: NH#11 (w=1, MagicMAC1), NH#12 (w=3, MagicMAC2)
//                                   - NHG#12: NH#13 (w=2, MagicMAC3) -> 1:3:12 MAC ratio
//   DUT port-3 -------> port-3 ATE  Tunnel 2 Primary (203.0.113.2, 75% baseline):
//                                   - NHG#14: NH#15 (w=1, MagicMAC1)
//   DUT port-4 -------> port-4 ATE  Shared Repair Backup (REPAIR_VRF -> TE_VRF_222):
//                                   - NHG#13: NH#14 (MagicMAC1) -> Tunnel 1 Bkp (203.0.113.100)
//                                   - NHG#15: NH#16 (MagicMAC2) -> Tunnel 2 Bkp (.101 / .102)
//
// Triage notes if a subtest fails:
//   - Baseline [0, 0.25, 0.75, 0] or port2 MAC ratio [0.0625, 0.1875, 0.75]: check hierarchical
//     weights across NHG#101 -> NHG#1 -> NHG#11/NHG#12 and static ARP/route programming on port2.
//   - Test-1 [0, 0, 0.75, 0.25]: check NHG#1 backup NHG#3000 -> REPAIR_VRF -> NHG#1000 -> port4.
//   - Test-2/Test-3 [0.25, 0, 0.75, 0] or Test-5..9 [1, 0, 0, 0]: check decap/fallback to DEFAULT
//     VRF and iBGP/IS-IS reachability out bidirectional port1.
//   - Test-4 [0, 0, 0, 1] or port4 backup ratio [0.25, 0.75]: check both NHG#1000 (MagicMAC1)
//     and NHG#1001 ECMP .101/.102 (MagicMAC2) on shared backup port4.

const (
	plenIPv4               = 30
	plenIPv6               = 126
	dscpEncapA1            = 10
	ipv4OuterSrcAddr       = "198.100.200.123"
	ipv4InnerDst           = "138.0.11.8"
	ipv4OuterDst333        = "192.58.200.7"
	noMatchEncapDest       = "20.0.0.1"
	niDecapTeVrf           = "DECAP_TE_VRF"
	niEncapTeVrfA          = "ENCAP_TE_VRF_A"
	niRepairVrf            = "REPAIR_VRF"
	tolerancePct           = 2
	tolerance              = 0.2
	encapFlow              = "encapFlow"
	correspondingHopLimit  = 64
	magicIP                = "192.168.1.1"
	magicMAC               = "02:00:00:00:00:01"
	maskLen24              = "24"
	maskLen32              = "32"
	gribiIPv4EntryDefVRF1  = "192.0.2.101"
	gribiIPv4EntryDefVRF2  = "192.0.2.102"
	gribiIPv4EntryDefVRF3  = "192.0.2.103"
	gribiIPv4EntryDefVRF4  = "192.0.2.104"
	gribiIPv4EntryDefVRF5  = "192.0.2.105"
	niTEVRF111             = "TE_VRF_111"
	niTEVRF222             = "TE_VRF_222"
	ipv4OuterSrc111Addr    = "198.51.100.111"
	ipv4OuterSrc222Addr    = "198.51.100.222"
	gribiIPv4EntryVRF1111  = "203.0.113.1"
	gribiIPv4EntryVRF1112  = "203.0.113.2"
	gribiIPv4EntryVRF1113  = "203.100.113.1"
	gribiIPv4EntryVRF1114  = "203.100.113.2"
	gribiIPv4EntryVRF2221  = "203.0.113.100"
	gribiIPv4EntryVRF2222  = "203.0.113.101"
	gribiIPv4EntryVRF2223  = "203.100.113.100"
	gribiIPv4EntryVRF2224  = "203.100.113.101"
	gribiIPv4EntryVRF2225  = "203.0.113.102"
	gribiIPv4EntryEncapVRF = "138.0.11.0"

	dutAreaAddress = "49.0001"
	dutSysID       = "1920.0000.2001"
	otgSysID1      = "640000000001"
	isisInstance   = "DEFAULT"

	otgIsisPort1LoopV4 = "203.0.113.10"
	otgIsisPort1LoopV6 = "2001:db8::203:0:113:10"

	dutAS        = 65501
	peerGrpName1 = "BGP-PEER-GROUP1"

	ateSrcPort       = "ate:port1"
	ateSrcPortMac    = "02:00:01:01:01:01"
	ateSrcNetName    = "srcnet"
	ateSrcNet        = "198.51.100.0"
	ateSrcNetCIDR    = "198.51.100.0/24"
	ateSrcNetFirstIP = "198.51.100.1"
	ateSrcNetCount   = 250
	ipOverIPProtocol = 4

	checkEncap = true
	wantLoss   = true
)

var (
	portsIPv4 = map[string]string{
		"dut:port1": "192.0.2.1",
		"ate:port1": "192.0.2.2",

		"dut:port2": "192.0.2.5",
		"ate:port2": "192.0.2.6",

		"dut:port3": "192.0.2.9",
		"ate:port3": "192.0.2.10",

		"dut:port4": "192.0.2.13",
		"ate:port4": "192.0.2.14",
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
	client     *fluent.GRIBIClient
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	otgConfig  gosnappi.Config
	top        gosnappi.Config
	electionID gribi.Uint128
	otg        *otg.OTG
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
// according to portsIPv4. Returns nil if the port has no IP address mapping.
func dutInterface(p *ondatra.Port, dut *ondatra.DUTDevice) *oc.Interface {
	id := fmt.Sprintf("%s:%s", p.Device().ID(), p.ID())
	i := &oc.Interface{
		Name:        ygot.String(p.Name()),
		Description: ygot.String(p.String()),
		Type:        oc.IETFInterfaces_InterfaceType_ethernetCsmacd,
	}
	if deviations.InterfaceEnabled(dut) {
		i.Enabled = ygot.Bool(true)
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
	if deviations.VendorPreConfigRequired(dut) {
		cfgplugins.ConfigureDecapVrfSelection(t, dut)
	}

	dc := gnmi.OC()
	for _, dp := range dutPortList {
		if i := dutInterface(dp, dut); i != nil {
			gnmi.Replace(t, dut, dc.Interface(dp.Name()).Config(), i)
		} else {
			t.Fatalf("No address found for port %v", dp)
		}
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		for _, dp := range dutPortList {
			fptest.AssignToNetworkInstance(t, dut, dp.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}
	if deviations.ExplicitPortSpeed(dut) {
		for _, dp := range dutPortList {
			fptest.SetPortSpeed(t, dp)
		}
	}

	loopbackIntfName = helpers.GetOrCreateLoopback(t, dut, 0, 0, &dutlo0Attrs)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, loopbackIntfName, deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		baseScenario.StaticARPWithSpecificIP4Links(t, dut)
	}
}

// configureGribiRoute configures Gribi route as per the requirements
func configureGribiRoute(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client *fluent.GRIBIClient) {
	t.Helper()

	baseScenario.ConfigureBaseGribiRoutes4Links(ctx, t, dut, client)

	// Programming AFT entries for backup NHG
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(2000).WithDecapsulateHeader(fluent.IPinIP).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(2000).AddNextHop(2000, 1),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	// Programming AFT entries for prefixes in TE_VRF_222
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(3).WithIPAddress(gribiIPv4EntryDefVRF3),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(2).AddNextHop(3, 1).WithBackupNHG(2000),
		fluent.IPv4Entry().WithNetworkInstance(niTEVRF222).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF2221+"/"+maskLen32).WithNextHopGroup(2),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(5).WithIPAddress(gribiIPv4EntryDefVRF5),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(4).AddNextHop(5, 1).WithBackupNHG(2000),
		fluent.IPv4Entry().WithNetworkInstance(niTEVRF222).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF2222+"/"+maskLen32).WithNextHopGroup(4),
		fluent.IPv4Entry().WithNetworkInstance(niTEVRF222).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF2225+"/"+maskLen32).WithNextHopGroup(4),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	teVRF222IPList := []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF2222, gribiIPv4EntryVRF2225}
	for ip := range teVRF222IPList {
		chk.HasResult(t, client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(teVRF222IPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// Programming AFT entries for backup NHG
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1000).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc222Addr, gribiIPv4EntryVRF2221).
			WithNextHopNetworkInstance(niTEVRF222),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1000).AddNextHop(1000, 1).WithBackupNHG(2000),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1001).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc222Addr, gribiIPv4EntryVRF2222).
			WithNextHopNetworkInstance(niTEVRF222),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1004).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc222Addr, gribiIPv4EntryVRF2225).
			WithNextHopNetworkInstance(niTEVRF222),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1001).AddNextHop(1001, 1).AddNextHop(1004, 1).WithBackupNHG(2000),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(3000).WithNextHopNetworkInstance(niRepairVrf),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(3000).AddNextHop(3000, 1),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	// Programming AFT entries for prefixes in TE_VRF_111
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1).WithIPAddress(gribiIPv4EntryDefVRF1),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(2).WithIPAddress(gribiIPv4EntryDefVRF2),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1).AddNextHop(1, 1).AddNextHop(2, 3).WithBackupNHG(3000),
		fluent.IPv4Entry().WithNetworkInstance(niTEVRF111).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF1111+"/"+maskLen32).WithNextHopGroup(1),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(4).WithIPAddress(gribiIPv4EntryDefVRF4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(3).AddNextHop(4, 1).WithBackupNHG(3000),
		fluent.IPv4Entry().WithNetworkInstance(niTEVRF111).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF1112+"/"+maskLen32).WithNextHopGroup(3),

		fluent.IPv4Entry().WithNetworkInstance(niRepairVrf).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF1111+"/"+maskLen32).WithNextHopGroup(1000),
		fluent.IPv4Entry().WithNetworkInstance(niRepairVrf).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryVRF1112+"/"+maskLen32).WithNextHopGroup(1001),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	teVRF111IPList := []string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112}
	for ip := range teVRF111IPList {
		chk.HasResult(t, client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(teVRF111IPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// Programming AFT entries for backup NHG
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(2001).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(2001).AddNextHop(2001, 1),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	// Programming AFT entries for prefixes in ENCAP_TE_VRF_A
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(101).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc111Addr, gribiIPv4EntryVRF1111).
			WithNextHopNetworkInstance(niTEVRF111),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(102).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc111Addr, gribiIPv4EntryVRF1112).
			WithNextHopNetworkInstance(niTEVRF111),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(101).AddNextHop(101, 1).AddNextHop(102, 3).WithBackupNHG(2001),
		fluent.IPv4Entry().WithNetworkInstance(niEncapTeVrfA).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryEncapVRF+"/"+maskLen24).WithNextHopGroup(101),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	chk.HasResult(t, client.Results(t),
		fluent.OperationResult().
			WithIPv4Operation(gribiIPv4EntryEncapVRF+"/24").
			WithOperationType(constants.Add).
			WithProgrammingResult(fluent.InstalledInFIB).
			AsResult(),
		chk.IgnoreOperationID(),
	)
	// Add Fallback route in ENCAP_TE_VRF_A as precondition
	client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1003).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1003).AddNextHop(1003, 1),
		fluent.IPv4Entry().WithNetworkInstance(niEncapTeVrfA).
			WithPrefix("0.0.0.0/0").WithNextHopGroup(1003).WithNextHopGroupNetworkInstance(deviations.DefaultNetworkInstance(dut)),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}
}

func configureISIS(t *testing.T, dut *ondatra.DUTDevice, intfName, dutAreaAddress, dutSysID string) {
	t.Helper()
	d := &oc.Root{}
	dutConfIsisPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	netInstance := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	prot := netInstance.GetOrCreateProtocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance)
	prot.Enabled = ygot.Bool(true)
	isis := prot.GetOrCreateIsis()
	globalISIS := isis.GetOrCreateGlobal()
	globalISIS.LevelCapability = oc.Isis_LevelType_LEVEL_2
	globalISIS.Net = []string{fmt.Sprintf("%v.%v.00", dutAreaAddress, dutSysID)}
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	if deviations.ISISInstanceEnabledRequired(dut) {
		globalISIS.Instance = ygot.String(isisInstance)
	}
	isisLevel2 := isis.GetOrCreateLevel(2)
	isisLevel2.MetricStyle = oc.Isis_MetricStyle_WIDE_METRIC
	if deviations.ISISLevelEnabled(dut) {
		isisLevel2.Enabled = ygot.Bool(true)
	}
	if deviations.ExplicitInterfaceInDefaultVRF(dut) || deviations.InterfaceRefInterfaceIDFormat(dut) {
		intfName = intfName + ".0"
	}

	isisIntf := isis.GetOrCreateInterface(intfName)
	isisIntf.Enabled = ygot.Bool(true)
	isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	isisIntfLevel := isisIntf.GetOrCreateLevel(2)
	isisIntfLevel.Enabled = ygot.Bool(true)
	isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfi.Metric = ygot.Uint32(200)
	if deviations.ISISInterfaceAfiUnsupported(dut) {
		isisIntf.Af = nil
	}
	if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
		isisIntfLevelAfi.Enabled = nil
	}

	gnmi.Replace(t, dut, dutConfIsisPath.Config(), prot)
}

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

	bgpNbr := bgp.GetOrCreateNeighbor(otgIsisPort1LoopV4)
	bgpNbr.PeerGroup = ygot.String(peerGrpName1)
	bgpNbr.PeerAs = ygot.Uint32(localAs)
	bgpNbr.Enabled = ygot.Bool(true)
	bgpNbrT := bgpNbr.GetOrCreateTransport()
	localAddressLeaf := dutlo0Attrs.IPv4
	if deviations.UseInterfaceNameForIBGPNeighborTransportIpv4LocalAddress(dut) {
		localAddressLeaf = loopbackIntfName
	}
	bgpNbrT.LocalAddress = ygot.String(localAddressLeaf)
	af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	af4.Enabled = ygot.Bool(true)
	af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	af6.Enabled = ygot.Bool(true)

	return niProto
}

func verifyISISTelemetry(t *testing.T, dut *ondatra.DUTDevice, dutIntf string) {
	t.Helper()
	statePath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_ISIS, isisInstance).Isis()

	if deviations.ExplicitInterfaceInDefaultVRF(dut) || deviations.InterfaceRefInterfaceIDFormat(dut) {
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

func verifyBgpTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	t.Logf("Verifying BGP state.")
	bgpPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP").Bgp()

	nbrPath := bgpPath.Neighbor(otgIsisPort1LoopV4)
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
	t.Logf("BGP adjacency for %s: %v", otgIsisPort1LoopV4, state)
	if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
		t.Errorf("BGP peer %s status got %d, want %d", otgIsisPort1LoopV4, state, want)
	}
}

// configureOTG configures the 4-link topology of the ATE (bidirectional IS-IS + iBGP on port1).
func configureOTG(t testing.TB, otg *otg.OTG, atePorts []*ondatra.Port) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	atePortNamelist = nil
	otgPortDevices = nil
	pmd100GFRPorts := []string{}
	for i, ap := range atePorts {
		if ap.PMD() == ondatra.PMD100GBASEFR {
			pmd100GFRPorts = append(pmd100GFRPorts, ap.ID())
		}
		dutid := fmt.Sprintf("dut:%s", ap.ID())
		ateid := fmt.Sprintf("ate:%s", ap.ID())

		port := config.Ports().Add().SetName(ap.ID())
		atePortNamelist = append(atePortNamelist, port.Name())
		portName := fmt.Sprintf("atePort%s", strconv.Itoa(i))
		dev := config.Devices().Add().SetName(portName)
		macAddress, _ := incrementMAC(ateSrcPortMac, i)
		eth := dev.Ethernets().Add().SetName(portName + ".Eth").SetMac(macAddress)
		eth.Connection().SetPortName(port.Name())
		eth.Ipv4Addresses().Add().SetName(portName + ".IPv4").
			SetAddress(portsIPv4[ateid]).SetGateway(portsIPv4[dutid]).
			SetPrefix(plenIPv4)
		eth.Ipv6Addresses().Add().SetName(portName + ".IPv6").
			SetAddress(portsIPv6[ateid]).SetGateway(portsIPv6[dutid]).
			SetPrefix(plenIPv6)

		otgPortDevices = append(otgPortDevices, dev)
		if i == 0 {
			iDut1LoopV4 := dev.Ipv4Loopbacks().Add().SetName("Port1LoopV4").SetEthName(eth.Name())
			iDut1LoopV4.SetAddress(otgIsisPort1LoopV4)
			iDut1LoopV6 := dev.Ipv6Loopbacks().Add().SetName("Port1LoopV6").SetEthName(eth.Name())
			iDut1LoopV6.SetAddress(otgIsisPort1LoopV6)
			isisDut := dev.Isis().SetName("ISIS1").SetSystemId(otgSysID1)
			isisDut.Basic().SetIpv4TeRouterId(portsIPv4[ateid]).SetHostname(isisDut.Name()).SetLearnedLspFilter(true)
			isisDut.Interfaces().Add().SetEthName(dev.Ethernets().Items()[0].Name()).
				SetName("devIsisInt1").
				SetLevelType(gosnappi.IsisInterfaceLevelType.LEVEL_2).
				SetNetworkType(gosnappi.IsisInterfaceNetworkType.POINT_TO_POINT)

			// Advertise OTG Port1 loopback address via ISIS on bidirectional port1.
			isisPort1V4 := dev.Isis().V4Routes().Add().SetName("ISISPort1V4").SetLinkMetric(10)
			isisPort1V4.Addresses().Add().SetAddress(otgIsisPort1LoopV4).SetPrefix(32)
			isisPort1V6 := dev.Isis().V6Routes().Add().SetName("ISISPort1V6").SetLinkMetric(10)
			isisPort1V6.Addresses().Add().SetAddress(otgIsisPort1LoopV6).SetPrefix(uint32(128))
			iDutBgp := dev.Bgp().SetRouterId(otgIsisPort1LoopV4)
			iDutBgp4Peer := iDutBgp.Ipv4Interfaces().Add().SetIpv4Name(iDut1LoopV4.Name()).Peers().Add().SetName(ap.ID() + ".BGP4.peer")
			iDutBgp4Peer.SetPeerAddress(dutlo0Attrs.IPv4).SetAsNumber(dutAS).SetAsType(gosnappi.BgpV4PeerAsType.IBGP)
			iDutBgp4Peer.Capability().SetIpv4Unicast(true).SetIpv6Unicast(true)
			iDutBgp4Peer.LearnedInformationFilter().SetUnicastIpv4Prefix(true).SetUnicastIpv6Prefix(true)

			bgpNeti1Bgp4PeerRoutes := iDutBgp4Peer.V4Routes().Add().SetName(port.Name() + ".BGP4.Route")
			bgpNeti1Bgp4PeerRoutes.SetNextHopIpv4Address(otgIsisPort1LoopV4).
				SetNextHopAddressType(gosnappi.BgpV4RouteRangeNextHopAddressType.IPV4).
				SetNextHopMode(gosnappi.BgpV4RouteRangeNextHopMode.MANUAL).
				Advanced().SetLocalPreference(100).SetIncludeLocalPreference(true)
			bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(ipv4InnerDst).SetPrefix(32).
				SetCount(1).SetStep(1)
			bgpNeti1Bgp4PeerRoutes.Addresses().Add().SetAddress(noMatchEncapDest).SetPrefix(32).
				SetCount(1).SetStep(1)
		}
	}
	config.Captures().Add().SetName("packetCapture").
		SetPortNames(atePortNamelist).
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

func createFlow(t *testing.T, config gosnappi.Config, otg *otg.OTG, trafficDestIP string) {
	t.Helper()

	config.Flows().Clear()

	flow1 := gosnappi.NewFlow().SetName(encapFlow)
	flow1.Metrics().SetEnable(true)
	flow1.TxRx().Device().
		SetTxNames([]string{otgPortDevices[0].Name() + ".IPv4"}).
		SetRxNames([]string{
			otgPortDevices[1].Name() + ".IPv4",
			otgPortDevices[2].Name() + ".IPv4",
			otgPortDevices[3].Name() + ".IPv4",
		})
	flow1.Size().SetFixed(512)
	flow1.Rate().SetPps(100)
	flow1.Duration().Continuous()

	ethHeader1 := flow1.Packet().Add().Ethernet()
	ethHeader1.Src().SetValue(ateSrcPortMac)

	IPHeader := flow1.Packet().Add().Ipv4()
	IPHeader.Src().Increment().SetCount(1000).SetStep("0.0.0.1").SetStart(ipv4OuterSrcAddr)
	IPHeader.Dst().SetValue(trafficDestIP)
	IPHeader.Priority().Dscp().Phb().SetValue(dscpEncapA1)

	UDPHeader := flow1.Packet().Add().Udp()
	UDPHeader.DstPort().Increment().SetStart(1).SetCount(50000).SetStep(1)
	UDPHeader.SrcPort().Increment().SetStart(1).SetCount(50000).SetStep(1)

	config.Flows().Append(flow1)

	t.Logf("Pushing traffic flows to OTG and starting protocols...")
	otg.PushConfig(t, config)
	time.Sleep(30 * time.Second)
	otg.StartProtocols(t)
	time.Sleep(30 * time.Second)
}

func sendTraffic(t *testing.T, args *testArgs, cs gosnappi.ControlState) {
	t.Helper()
	defer otgutils.StopCapture(t, args.otg, cs)
	t.Logf("Starting traffic")
	args.otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	args.otg.StopTraffic(t)
}

func readPortInFrames(t *testing.T, otgDev *otg.OTG) []uint64 {
	t.Helper()
	counts := make([]uint64, len(atePortNamelist))
	for i, p := range atePortNamelist {
		counts[i] = gnmi.Get(t, otgDev, gnmi.OTG().Port(p).Counters().InFrames().State())
	}
	return counts
}

func captureAndVerifyTraffic(t *testing.T, args *testArgs, capturePortList []string, loadBalancePercent []float64, wantLoss, checkEncap bool, headerDstIP map[string][]string) []string {
	t.Helper()
	var pcapFileList []string
	for idx, capturePort := range capturePortList {
		cs := gosnappi.NewControlState()
		cs.Port().Capture().SetPortNames([]string{capturePort}).SetState(gosnappi.StatePortCaptureState.START)
		args.otg.SetControlState(t, cs)
		t.Cleanup(func() {
			otgutils.StopCapture(t, args.otg, cs)
		})

		sendTraffic(t, args, cs)
		waitForFlowMetricsReady(t, args.otg, encapFlow, 1*time.Minute)

		if idx == 0 {
			// OTG resets both flow counters and port InFrames on every StartTraffic().
			txPkts := gnmi.Get(t, args.otg, gnmi.OTG().Flow(encapFlow).Counters().OutPkts().State())
			portInFrames := readPortInFrames(t, args.otg)
			t.Logf("Port InFrames [port1..port4]: %v (flow %s tx=%d)", portInFrames, encapFlow, txPkts)
			verifyTrafficMetrics(t, args, txPkts, portInFrames, loadBalancePercent, wantLoss)
		}

		pcapBytes := args.otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(capturePort))
		pcapFile, err := os.CreateTemp("", "pcap")
		if err != nil {
			t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
			continue
		}
		pcapName := pcapFile.Name()
		t.Cleanup(func() {
			os.Remove(pcapName)
		})
		if _, err := pcapFile.Write(pcapBytes); err != nil {
			t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
		}
		pcapFile.Close()
		pcapFileList = append(pcapFileList, pcapName)
		validatePacketFile(t, pcapName, idx, checkEncap, headerDstIP)
	}
	return pcapFileList
}

func verifyTrafficMetrics(t *testing.T, args *testArgs, txPkts uint64, portInFrames []uint64, loadBalancePercent []float64, wantLoss bool) {
	t.Helper()
	if wantLoss {
		otgutils.ExpectedTrafficLoss(t, args.otg, encapFlow, float64(100-tolerancePct), 100)
	} else if loadBalancePercent[0] > 0 {
		// port1 is excluded from OTG flow RxNames because it is also the Tx port;
		// account fallback packets via the per-run 4-port InFrames sum against txPkts.
		if txPkts == 0 {
			t.Errorf("Fallback traffic validation failed: txPkts=0 for flow %s (portInFrames=%v)", encapFlow, portInFrames)
		} else {
			_, totalRx := normalize(portInFrames)
			var lossPct float64
			if totalRx < txPkts {
				lossPct = float64(txPkts-totalRx) * 100.0 / float64(txPkts)
			}
			t.Logf("Fallback traffic accounting for flow %s: tx=%d totalRx=%d portInFrames=%v lossPct=%.2f%%", encapFlow, txPkts, totalRx, portInFrames, lossPct)
			if lossPct > float64(tolerancePct)+0.99 {
				t.Errorf("Fallback traffic loss pct got %.2f%%, want <= %.2f%% (txPkts=%d, portInFrames=%v)", lossPct, float64(tolerancePct)+0.99, txPkts, portInFrames)
			}
		}
	} else {
		otgutils.ExpectedTrafficLoss(t, args.otg, encapFlow, 0, float64(tolerancePct)+0.99)
	}
	t.Log("Verify packet load balancing as per the programmed weight")
	validateTrafficDistribution(t, portInFrames, loadBalancePercent)
}

// waitForFlowMetricsReady waits until a flow's TX/RX counters stop changing, or fails after timeout.
func waitForFlowMetricsReady(t *testing.T, otgDev *otg.OTG, flowName string, timeout time.Duration) {
	const pollInterval = time.Second
	const stableReads = 2 // Require N consecutive identical reads to reduce jitter.

	type counters struct {
		tx uint64
		rx uint64
	}

	deadline := time.Now().Add(timeout)
	var (
		prev      counters
		havePrev  bool
		stableCnt int
		last      counters
	)

	for time.Now().Before(deadline) {
		flowMetric := gnmi.Get(t, otgDev, gnmi.OTG().Flow(flowName).State())
		cur := counters{
			tx: flowMetric.GetCounters().GetOutPkts(),
			rx: flowMetric.GetCounters().GetInPkts(),
		}
		last = cur

		if havePrev && cur == prev {
			stableCnt++
			if stableCnt >= stableReads {
				t.Logf("Flow %q metrics stabilized: tx=%d rx=%d", flowName, cur.tx, cur.rx)
				return
			}
		} else {
			stableCnt = 0
		}

		prev = cur
		havePrev = true
		time.Sleep(pollInterval)
	}

	t.Fatalf("Flow %q metrics did not stabilize within %s (last tx=%d rx=%d)", flowName, timeout, last.tx, last.rx)
}

func validatePacketFile(t *testing.T, file string, index int, checkEncap bool, headerDstIP map[string][]string) {
	t.Helper()
	fileStat, err := os.Stat(file)
	if err != nil {
		t.Errorf("Filestat for pcap file failed %s", err)
		return
	}
	if fileStat.Size() > 0 {
		handle, err := pcap.OpenOffline(file)
		if err != nil {
			t.Errorf("Unable to open the pcap file, error: %s", err)
			return
		}
		defer handle.Close()
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		if checkEncap {
			validateTrafficEncap(t, packetSource, headerDstIP, index)
		}
	}
}

func validateTrafficEncap(t *testing.T, packetSource *gopacket.PacketSource, headerDstIP map[string][]string, index int) {
	t.Helper()
	for packet := range packetSource.Packets() {
		if ethLayer := packet.Layer(layers.LayerTypeEthernet); ethLayer != nil {
			if eth, ok := ethLayer.(*layers.Ethernet); ok && eth.SrcMAC.String() == ateSrcPortMac {
				// Ignore ATE port1's own transmitted packets on bidirectional port1 capture.
				continue
			}
		}
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			continue
		}
		ipPacket, _ := ipLayer.(*layers.IPv4)
		encapHeaderListLength := len(headerDstIP["outerIP"])
		if index <= encapHeaderListLength-1 {
			innerPacket := gopacket.NewPacket(ipPacket.Payload, ipPacket.NextLayerType(), gopacket.Default)
			ipInnerLayer := innerPacket.Layer(layers.LayerTypeIPv4)
			if ipInnerLayer != nil {
				destIP := ipPacket.DstIP.String()
				t.Logf("Outer dest ip in received packet %s", destIP)
				expectedDstIP := headerDstIP["outerIP"][index]
				outerDstIPAllowed := destIP == expectedDstIP || (expectedDstIP == gribiIPv4EntryVRF2222 && destIP == gribiIPv4EntryVRF2225)
				if !outerDstIPAllowed {
					t.Errorf("Packets have incorrect outer destination IP, got %s, want %s", destIP, expectedDstIP)
				}
				ipInnerPacket, _ := ipInnerLayer.(*layers.IPv4)
				if ipInnerPacket.DstIP.String() != headerDstIP["innerIP"][index] {
					t.Errorf("Packets are not encapsulated")
				}
				t.Logf("Traffic for encap routes passed.")
				break
			} else {
				t.Errorf("Packet is not encapsulated")
			}
		} else if index >= encapHeaderListLength || encapHeaderListLength == 0 {
			if ipPacket.DstIP.String() == otgIsisPort1LoopV4 || ipPacket.SrcIP.String() == otgIsisPort1LoopV4 {
				continue
			} else if ipPacket.DstIP.String() != headerDstIP["innerIP"][0] {
				destIP := ipPacket.DstIP.String()
				t.Logf("Dest ip in received packet %s", destIP)
				t.Errorf("Packets are encapsulated which is not expected")
			} else {
				t.Logf("Traffic for non-encap routes passed.")
				break
			}
		}
	}
}

func verifyPortStatus(t *testing.T, args *testArgs, portList []string, portStatus bool) {
	wantStatus := oc.Interface_OperStatus_UP
	if !portStatus {
		wantStatus = oc.Interface_OperStatus_DOWN
	}
	for _, port := range portList {
		p := args.dut.Port(t, port)
		t.Logf("Checking OperStatus for port %s", port)
		operStatus := gnmi.Watch(t, args.dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), 2*time.Minute, func(val *ygnmi.Value[oc.E_Interface_OperStatus]) bool {
			if v, ok := val.Val(); ok {
				return v == wantStatus
			}
			return false
		})
		if got, ok := operStatus.Await(t); !ok {
			t.Errorf("Get(DUT %v oper status): got %v, want %v", port, got, wantStatus)
		}
	}
}

// setDUTInterfaceWithState sets the admin state on the dut interface
func setDUTInterfaceWithState(t testing.TB, dut *ondatra.DUTDevice, p *ondatra.Port, state bool) {
	dc := gnmi.OC()
	i := &oc.Interface{}
	i.Enabled = ygot.Bool(state)
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	i.Name = ygot.String(p.Name())
	gnmi.Update(t, dut, dc.Interface(p.Name()).Config(), i)
}

func portState(t *testing.T, args *testArgs, portList []string, portEnabled bool) {
	t.Logf("Change port enable state to %t", portEnabled)
	if deviations.ATEPortLinkStateOperationsUnsupported(args.ate) {
		for _, port := range portList {
			p := args.dut.Port(t, port)
			if portEnabled {
				setDUTInterfaceWithState(t, args.dut, p, true)
			} else {
				setDUTInterfaceWithState(t, args.dut, p, false)
			}
		}
	} else {
		var portNames []string
		for _, p := range portList {
			portNames = append(portNames, args.ate.Port(t, p).ID())
		}
		portStateAction := gosnappi.NewControlState()
		if portEnabled {
			portStateAction.Port().Link().SetPortNames(portNames).SetState(gosnappi.StatePortLinkState.UP)
		} else {
			portStateAction.Port().Link().SetPortNames(portNames).SetState(gosnappi.StatePortLinkState.DOWN)
		}
		args.ate.OTG().SetControlState(t, portStateAction)
	}
}

func normalize(xs []uint64) (ys []float64, sum uint64) {
	for _, x := range xs {
		sum += x
	}
	ys = make([]float64, len(xs))
	if sum == 0 {
		return ys, 0
	}
	for i, x := range xs {
		ys[i] = float64(x) / float64(sum)
	}
	return ys, sum
}

func validateTrafficDistribution(t *testing.T, inFramesDeltas []uint64, wantWeights []float64) {
	gotWeights, _ := normalize(inFramesDeltas)

	t.Log("got ratio:", gotWeights)
	t.Log("want ratio:", wantWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, tolerance)); diff != "" {
		t.Errorf("Packet distribution ratios [port1..port4] -want,+got:\n%s", diff)
	}
}

// validateEncapNHTelemetry verifies AFT telemetry for the encap next-hops used by
// the 138.0.11.0/24 entry in ENCAP_TE_VRF_A.
func validateEncapNHTelemetry(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	pfx := gribiIPv4EntryEncapVRF + "/" + maskLen24
	ipv4Path := gnmi.OC().NetworkInstance(niEncapTeVrfA).Afts().Ipv4Entry(pfx)
	ipv4Val, ok := gnmi.Watch(t, dut, ipv4Path.State(), 2*time.Minute, func(val *ygnmi.Value[*oc.NetworkInstance_Afts_Ipv4Entry]) bool {
		v, present := val.Val()
		return present && v.GetNextHopGroup() != 0
	}).Await(t)
	if !ok {
		t.Fatalf("Could not find prefix %s in %s telemetry AFT", pfx, niEncapTeVrfA)
	}
	ipv4Entry, _ := ipv4Val.Val()
	nhgNI := ipv4Entry.GetNextHopGroupNetworkInstance()
	if nhgNI == "" {
		nhgNI = deviations.DefaultNetworkInstance(dut)
	}
	nhgID := ipv4Entry.GetNextHopGroup()

	nhgVal, ok := gnmi.Watch(t, dut, gnmi.OC().NetworkInstance(nhgNI).Afts().NextHopGroup(nhgID).State(), 2*time.Minute,
		func(val *ygnmi.Value[*oc.NetworkInstance_Afts_NextHopGroup]) bool {
			v, present := val.Val()
			return present && len(v.NextHop) > 0
		}).Await(t)
	if !ok {
		t.Fatalf("Next-hop-group %d for prefix %s is not present in telemetry AFT", nhgID, pfx)
	}
	nhg, _ := nhgVal.Val()

	for nhIndex := range nhg.NextHop {
		nh, present := gnmi.Lookup(t, dut, gnmi.OC().NetworkInstance(nhgNI).Afts().NextHop(nhIndex).State()).Val()
		if !present {
			t.Errorf("Next-hop %d of next-hop-group %d is not present in telemetry AFT", nhIndex, nhgID)
			continue
		}
		t.Logf("Prefix %s next-hop %d: encapsulate-header = %v, origin-protocol = %v", pfx, nhIndex, nh.GetEncapsulateHeader(), nh.GetOriginProtocol())
		if !deviations.AFTNextHopEncapsulateHeaderUnsupported(dut) {
			if got, want := nh.GetEncapsulateHeader(), oc.Aft_EncapsulationHeaderType_IPV4; got != want {
				t.Errorf("Prefix %s next-hop %d: next-hop/state/encapsulate-header = %v, want %v", pfx, nhIndex, got, want)
			}
		}
		if !deviations.AFTNextHopOriginProtocolUnsupported(dut) {
			if got, want := nh.GetOriginProtocol(), oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_GRIBI; got != want {
				t.Errorf("Prefix %s next-hop %d: next-hop/state/origin-protocol = %v, want %v", pfx, nhIndex, got, want)
			}
		}
	}
}

// TestEncapFrr is to test FRR behaviors with encapsulation scenarios on a 4-link testbed.
func TestEncapFrr(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	gribic := dut.RawAPIs().GRIBI(t)
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	dutPorts := sortPorts(dut.Ports())[0:4]
	atePorts := sortPorts(ate.Ports())[0:4]

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

	configureDUT(t, dut, dutPorts)

	t.Log("Apply vrf selection policy to DUT port-1")
	vrfpolicy.ConfigureVRFSelectionPolicy(t, dut, vrfpolicy.VRFPolicyC)

	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		baseScenario.StaticARPWithMagicUniversalIP4Links(t, dut)
	}

	t.Log("Install BGP route resolved by ISIS on bidirectional DUT port-1.")
	t.Log("Configure ISIS on DUT")
	configureISIS(t, dut, dut.Port(t, "port1").Name(), dutAreaAddress, dutSysID)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Delete(t, dut, dutConfPath.Config())
	dutConf := bgpCreateNbr(dutAS, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

	otg := ate.OTG()
	otgConfig := configureOTG(t, otg, atePorts)

	verifyISISTelemetry(t, dut, dutPorts[0].Name())
	verifyBgpTelemetry(t, dut)

	client := fluent.NewClient()
	client.Connection().WithStub(gribic).WithPersistence().WithInitialElectionID(1, 0).
		WithFIBACK().WithRedundancyMode(fluent.ElectedPrimaryClient)
	client.Start(ctx, t)
	defer client.Stop(t)

	defer func() {
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
		client:     client,
		dut:        dut,
		ate:        ate,
		otgConfig:  otgConfig,
		top:        top,
		electionID: eID,
		otg:        otg,
	}

	testCases := baseScenario.TestCases(atePortNamelist, ipv4InnerDst)
	testCases = append(testCases,
		&baseScenario.TestCase{
			Desc:                   "Test-8: no match in TE_VRF_222",
			DownPortList:           []string{"port2", "port3"},
			CapturePortList:        []string{atePortNamelist[0]},
			EncapHeaderOuterIPList: []string{},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{1, 0, 0, 0},
			TestID:                 "teVrf222NoMatch",
		},
		&baseScenario.TestCase{
			Desc:                   "Test-9: no match in TE_VRF_111",
			DownPortList:           []string{"port2", "port3"},
			CapturePortList:        []string{atePortNamelist[0]},
			EncapHeaderOuterIPList: []string{},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{1, 0, 0, 0},
			TestID:                 "teVrf111NoMatch",
		},
	)
	t.Run("Validate encap next-hop AFT telemetry", func(t *testing.T) {
		if err := gribi.FlushAll(client); err != nil {
			t.Fatal(err)
		}
		configureGribiRoute(ctx, t, dut, args.client)
		validateEncapNHTelemetry(t, dut)
	})
	for _, tc := range testCases {
		t.Run(tc.Desc, func(t *testing.T) {
			t.Log("Verify whether the ports are in up state")
			portList := []string{"port1", "port2", "port3", "port4"}
			verifyPortStatus(t, args, portList, true)

			t.Log("Flush existing gRIBI routes before test.")
			if err := gribi.FlushAll(client); err != nil {
				t.Fatal(err)
			}
			baseCapturePortList := []string{atePortNamelist[1], atePortNamelist[2]}
			configureGribiRoute(ctx, t, dut, args.client)
			createFlow(t, otgConfig, otg, ipv4InnerDst)
			baseHeaderDstIP := map[string][]string{"outerIP": {gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112}, "innerIP": {ipv4InnerDst, ipv4InnerDst}}
			baseLoadBalancePercent := []float64{0, 0.25, 0.75, 0}
			basePcaps := captureAndVerifyTraffic(t, args, baseCapturePortList, baseLoadBalancePercent, !wantLoss, checkEncap, baseHeaderDstIP)
			if len(basePcaps) > 0 {
				if err := baseScenario.ValidateTunnel1MACDistribution(basePcaps[0], tolerance); err != nil {
					t.Errorf("Baseline port2 MAC distribution check failed: %v", err)
				}
			}

			if tc.TestID == "primaryBackupRoutingSingle" {
				args.client.Modify().AddEntry(t,
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(1100).WithDecapsulateHeader(fluent.IPinIP).
						WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
					fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithID(1000).AddNextHop(1100, 1),
				)
				if err := awaitTimeout(ctx, t, args.client, 2*time.Minute); err != nil {
					t.Logf("Could not program entries via client, got err, check error codes: %v", err)
				}
			}
			if tc.TestID == "primaryBackupRoutingAll" {
				args.client.Modify().AddEntry(t,
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(1200).WithDecapsulateHeader(fluent.IPinIP).
						WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(1201).WithDecapsulateHeader(fluent.IPinIP).
						WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
					fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithID(1000).AddNextHop(1200, 1),
					fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithID(1001).AddNextHop(1201, 1),
				)
				if err := awaitTimeout(ctx, t, args.client, 2*time.Minute); err != nil {
					t.Logf("Could not program entries via client, got err, check error codes: %v", err)
				}
			}
			if tc.TestID == "encapNoMatch" {
				createFlow(t, otgConfig, otg, noMatchEncapDest)
			}
			if tc.TestID == "teVrf222NoMatch" {
				args.client.Modify().AddEntry(t,
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(1300).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
						WithIPinIP(ipv4OuterSrc222Addr, gribiIPv4EntryVRF2223).WithNextHopNetworkInstance(niTEVRF222),
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(1301).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
						WithIPinIP(ipv4OuterSrc222Addr, gribiIPv4EntryVRF2224).WithNextHopNetworkInstance(niTEVRF222),
					fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithID(1000).AddNextHop(1300, 1),
					fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithID(1001).AddNextHop(1301, 1),
				)
				if err := awaitTimeout(ctx, t, args.client, 2*time.Minute); err != nil {
					t.Logf("Could not program entries via client, got err, check error codes: %v", err)
				}
				res := args.client.Results(t)
				operIndexList := []uint64{1000, 1001}
				for _, operIndex := range operIndexList {
					chk.HasResult(t, res,
						fluent.OperationResult().
							WithOperationType(constants.Add).
							WithNextHopOperation(operIndex).
							WithProgrammingResult(fluent.InstalledInFIB).
							AsResult(),
						chk.IgnoreOperationID(),
					)
					chk.HasResult(t, res,
						fluent.OperationResult().
							WithOperationType(constants.Add).
							WithNextHopGroupOperation(operIndex).
							WithProgrammingResult(fluent.InstalledInFIB).
							AsResult(),
						chk.IgnoreOperationID(),
					)
				}
			}
			if tc.TestID == "teVrf111NoMatch" {
				args.client.Modify().AddEntry(t,
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(201).WithEncapsulateHeader(fluent.IPinIP).WithIPinIP(ipv4OuterSrc111Addr, gribiIPv4EntryVRF1113).
						WithNextHopNetworkInstance(niTEVRF111),
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(202).WithEncapsulateHeader(fluent.IPinIP).WithIPinIP(ipv4OuterSrc111Addr, gribiIPv4EntryVRF1114).
						WithNextHopNetworkInstance(niTEVRF111),
					fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithID(101).AddNextHop(201, 1).AddNextHop(202, 3).WithBackupNHG(2001),
				)
				if err := awaitTimeout(ctx, t, args.client, 2*time.Minute); err != nil {
					t.Logf("Could not program entries via client, got err, check error codes: %v", err)
				}
				res := args.client.Results(t)
				chk.HasResult(t, res,
					fluent.OperationResult().
						WithOperationType(constants.Add).
						WithNextHopOperation(201).
						WithProgrammingResult(fluent.InstalledInFIB).
						AsResult(),
					chk.IgnoreOperationID(),
				)
				chk.HasResult(t, res,
					fluent.OperationResult().
						WithOperationType(constants.Add).
						WithNextHopOperation(202).
						WithProgrammingResult(fluent.InstalledInFIB).
						AsResult(),
					chk.IgnoreOperationID(),
				)
				chk.HasResult(t, res,
					fluent.OperationResult().
						WithOperationType(constants.Add).
						WithNextHopGroupOperation(101).
						WithProgrammingResult(fluent.InstalledInFIB).
						AsResult(),
					chk.IgnoreOperationID(),
				)
			}

			capturePortList := tc.CapturePortList
			loadBalancePercent := tc.LoadBalancePercent
			outerIPList := tc.EncapHeaderOuterIPList
			innerIPList := tc.EncapHeaderInnerIPList

			if deviations.ReducedEcmpSetOnMixedEncapDecapNh(dut) {
				if tc.TestID == "primaryBackupSingle" || tc.TestID == "primaryBackupRoutingSingle" {
					capturePortList = []string{atePortNamelist[2]}
					loadBalancePercent = []float64{0, 0, 1, 0}
					outerIPList = []string{gribiIPv4EntryVRF1112}
					innerIPList = []string{ipv4InnerDst}
				}
			}
			if deviations.EncapTunnelShutBackupNhgZeroTraffic(dut) {
				if tc.TestID == "primarySingle" || tc.TestID == "primaryBackupSingle" || tc.TestID == "primaryBackupRoutingSingle" {
					capturePortList = []string{atePortNamelist[2]}
					loadBalancePercent = []float64{0, 0, 1, 0}
					outerIPList = []string{gribiIPv4EntryVRF1112}
					innerIPList = []string{ipv4InnerDst}
				} else if tc.TestID == "primaryAll" {
					capturePortList = []string{atePortNamelist[0]}
					loadBalancePercent = []float64{1, 0, 0, 0}
					outerIPList = nil
					innerIPList = []string{ipv4InnerDst}
				}
			}

			if len(tc.DownPortList) > 0 {
				downPorts := tc.DownPortList
				t.Logf("Bring down ports %s", downPorts)
				portState(t, args, downPorts, false)
				t.Cleanup(func() {
					portState(t, args, downPorts, true)
				})
				t.Log("Verify the port status after bringing down the ports")
				verifyPortStatus(t, args, downPorts, false)
			}

			if tc.TestID == "primaryAll" && !deviations.EncapTunnelShutBackupNhgZeroTraffic(dut) {
				// In Test-4, both backup tunnels share port4; verify [0, 0, 0, 1] port share
				// and the 25%:75% split across (203.0.113.100, MagicMAC1) and (.101/.102, MagicMAC2).
				testPcaps := captureAndVerifyTraffic(t, args, capturePortList, loadBalancePercent, !wantLoss, false, nil)
				if len(testPcaps) > 0 {
					if err := baseScenario.ValidateBackupTunnelDistribution(testPcaps[0], ipv4InnerDst, true, tolerance); err != nil {
						t.Errorf("Test-4 shared backup port4 distribution check failed: %v", err)
					}
				}
				return
			}

			headerDstIP := map[string][]string{"outerIP": outerIPList, "innerIP": innerIPList}
			captureAndVerifyTraffic(t, args, capturePortList, loadBalancePercent, !wantLoss, checkEncap, headerDstIP)
		})
	}
}
