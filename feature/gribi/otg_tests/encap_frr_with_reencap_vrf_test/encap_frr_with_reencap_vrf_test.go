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
	otgtelemetry "github.com/openconfig/ondatra/gnmi/otg"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

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
	plenIPv4               = 30
	plenIPv6               = 126
	dscpEncapA1            = 10
	dscpEncapA2            = 18
	dscpEncapB1            = 20
	dscpEncapB2            = 28
	ipv4InnerDst           = "138.0.11.8"
	ipv6InnerDst           = "2001:db8::138:0:11:8"
	ipv4OuterDst333        = "192.58.200.7"
	noMatchEncapDest       = baseScenario.NoMatchEncapDest
	niDecapTeVrf           = "DECAP_TE_VRF"
	niEncapTeVrfA          = "ENCAP_TE_VRF_A"
	niEncapTeVrfB          = "ENCAP_TE_VRF_B"
	niRepairVrf            = vrfpolicy.NiRepair
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

	otgIsisPort8LoopV4 = "203.0.113.10"
	otgIsisPort8LoopV6 = "2001:db8::203:0:113:10"

	dutAS        = 65501
	peerGrpName1 = "BGP-PEER-GROUP1"

	ateSrcPort           = "ate:port1"
	ateSrcPortMac        = "02:00:01:01:01:01"
	ateSrcNetName        = "srcnet"
	ateSrcNet            = "198.51.100.0"
	ateSrcNetCIDR        = "198.51.100.0/24"
	ateSrcNetFirstIP     = "198.51.100.1"
	ateSrcNetV6FirstIP   = "2001:db8::198:51:100:1"
	ateSrcNetCount       = 250
	ipOverIPProtocol     = 4
	ipv6OverIPv4Protocol = 41
	minTransmittedPkts   = 1000

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
// according to portsIPv4.  Returns nil if the port has no IP address
// mapping.
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
		for _, dp := range dut.Ports() {
			fptest.AssignToNetworkInstance(t, dut, dp.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}
	if deviations.ExplicitPortSpeed(dut) {
		for _, dp := range dut.Ports() {
			fptest.SetPortSpeed(t, dp)
		}
	}

	loopbackIntfName = helpers.GetOrCreateLoopback(t, dut, 0, 0, &dutlo0Attrs)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, loopbackIntfName, deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		baseScenario.StaticARPWithSpecificIP(t, dut)
	}
}

// configureGribiRoute configures Gribi route as per the requirements
func configureGribiRoute(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client *fluent.GRIBIClient) {
	t.Helper()

	baseScenario.ConfigureBaseGribiRoutes(ctx, t, dut, client)

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
		// Note: NH#1004 is used here for 203.0.113.102 in NHG#1001 so it does not collide with
		// NH#1003 (which points to the DEFAULT network-instance in NHG#1003).
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

// configStaticArp configures static arp entries
func configStaticArp(p string, ipv4addr string, macAddr string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	n4 := s4.GetOrCreateNeighbor(ipv4addr)
	n4.LinkLayerAddress = ygot.String(macAddr)
	return i
}

func staticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	sb := &gnmi.SetBatch{}
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	p6 := dut.Port(t, "port6")
	p7 := dut.Port(t, "port7")
	portList := []*ondatra.Port{p2, p3, p4, p5, p6, p7}
	for idx, p := range portList {
		s := &oc.NetworkInstance_Protocol_Static{
			Prefix: ygot.String(magicIP + "/32"),
			NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
				strconv.Itoa(idx): {
					Index: ygot.String(strconv.Itoa(idx)),
					InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
						Interface: ygot.String(p.Name()),
					},
				},
			},
		}
		sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
		gnmi.BatchUpdate(sb, sp.Static(magicIP+"/32").Config(), s)
		gnmi.BatchUpdate(sb, gnmi.OC().Interface(p.Name()).Config(), configStaticArp(p.Name(), magicIP, magicMAC))
	}
	sb.Set(t, dut)
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
	globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST).Enabled = ygot.Bool(true)
	if deviations.ISISSingleTopologyRequired(dut) {
		afv6 := globalISIS.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
		afv6.GetOrCreateMultiTopology().SetAfiName(oc.IsisTypes_AFI_TYPE_IPV4)
		afv6.GetOrCreateMultiTopology().SetSafiName(oc.IsisTypes_SAFI_TYPE_UNICAST)
	}
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
	isisIntfLevelAfi.Enabled = ygot.Bool(true)
	isisIntfLevelAfi6 := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV6, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfi6.Metric = ygot.Uint32(200)
	isisIntfLevelAfi6.Enabled = ygot.Bool(true)
	if deviations.ISISInterfaceAfiUnsupported(dut) {
		isisIntf.Af = nil
	}
	if deviations.MissingIsisInterfaceAfiSafiEnable(dut) {
		isisIntfLevelAfi.Enabled = nil
		isisIntfLevelAfi6.Enabled = nil
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

	bgpNbr := bgp.GetOrCreateNeighbor(otgIsisPort8LoopV4)
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
func configureOTG(t *testing.T, otg *otg.OTG, atePorts []*ondatra.Port) gosnappi.Config {
	t.Helper()
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

			// Advertise IPv6 destination 2001:db8::138:0:11:8/128 on port-8 for Test-10 and Test-11 dual-stack flows.
			bgpNeti1Bgp6PeerRoutes := iDutBgp4Peer.V6Routes().Add().SetName(port.Name() + ".BGP6.Route")
			bgpNeti1Bgp6PeerRoutes.SetNextHopIpv6Address(otgIsisPort8LoopV6).
				SetNextHopAddressType(gosnappi.BgpV6RouteRangeNextHopAddressType.IPV6).
				SetNextHopMode(gosnappi.BgpV6RouteRangeNextHopMode.MANUAL).
				Advanced().SetLocalPreference(100).SetIncludeLocalPreference(true)
			bgpNeti1Bgp6PeerRoutes.Addresses().Add().SetAddress(ipv6InnerDst).SetPrefix(128).
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
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, config, "IPv4")
	otgutils.WaitForARP(t, otg, config, "IPv6")
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
		SetRxNames([]string{otgPortDevices[1].Name() + ".IPv4", otgPortDevices[2].Name() + ".IPv4", otgPortDevices[3].Name() + ".IPv4",
			otgPortDevices[4].Name() + ".IPv4", otgPortDevices[5].Name() + ".IPv4", otgPortDevices[6].Name() + ".IPv4",
			otgPortDevices[7].Name() + ".IPv4",
		})
	flow1.Size().SetFixed(512)
	flow1.Rate().SetPps(100)
	flow1.Duration().Continuous()

	ethHeader1 := flow1.Packet().Add().Ethernet()
	ethHeader1.Src().SetValue(ateSrcPortMac)

	IPHeader := flow1.Packet().Add().Ipv4()
	IPHeader.Src().Increment().SetCount(ateSrcNetCount).SetStep("0.0.0.1").SetStart(ateSrcNetFirstIP)
	IPHeader.Dst().SetValue(trafficDestIP)
	IPHeader.Priority().Dscp().Phb().SetValue(dscpEncapA1)

	UDPHeader := flow1.Packet().Add().Udp()
	UDPHeader.DstPort().Increment().SetStart(1).SetCount(50000).SetStep(1)
	UDPHeader.SrcPort().Increment().SetStart(1).SetCount(50000).SetStep(1)

	config.Flows().Append(flow1)

	t.Logf("Pushing traffic flows to OTG and starting protocols...")
	otg.PushConfig(t, config)
	otg.StartProtocols(t)
	otgutils.WaitForARP(t, otg, config, "IPv4")
}

func startCapture(t *testing.T, args *testArgs, capturePortList []string) gosnappi.ControlState {
	t.Helper()
	args.otgConfig.Captures().Clear()
	args.otgConfig.Captures().Add().SetName("packetCapture").
		SetPortNames(capturePortList).
		SetFormat(gosnappi.CaptureFormat.PCAP)
	args.otg.PushConfig(t, args.otgConfig)
	args.otg.StartProtocols(t)
	otgutils.WaitForARP(t, args.otg, args.otgConfig, "IPv4")
	otgutils.WaitForARP(t, args.otg, args.otgConfig, "IPv6")
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	args.otg.SetControlState(t, cs)
	t.Cleanup(func() {
		if len(args.otgConfig.Captures().Items()) > 0 {
			stopCS := gosnappi.NewControlState()
			stopCS.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
			args.otg.SetControlState(t, stopCS)
		}
	})
	return cs
}

func sendTraffic(t *testing.T, args *testArgs, capturePortList []string, cs gosnappi.ControlState) {
	t.Helper()
	otgutils.WaitForARP(t, args.otg, args.otgConfig, "IPv4")
	t.Logf("Starting traffic")
	args.otg.StartTraffic(t)
	// Wait via gnmi.Watch until at least minTransmittedPkts have been transmitted before stopping traffic.
	gnmi.Watch(t, args.otg, gnmi.OTG().Flow(encapFlow).Counters().OutPkts().State(), 30*time.Second, func(val *ygnmi.Value[uint64]) bool {
		outPkts, present := val.Val()
		return present && outPkts >= minTransmittedPkts
	}).Await(t)
	t.Logf("Stop traffic")
	args.otg.StopTraffic(t)
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	args.otg.SetControlState(t, cs)
}

func verifyTraffic(t *testing.T, args *testArgs, capturePortList []string, loadBalancePercent []float64, wantLoss, checkEncap bool, headerDstIP map[string][]string) {
	t.Helper()
	waitForFlowMetricsReady(t, args.otg, encapFlow, 1*time.Minute)
	if wantLoss {
		otgutils.ExpectedTrafficLoss(t, args.otg, encapFlow, float64(100-tolerancePct), 100)
	} else {
		otgutils.ExpectedTrafficLoss(t, args.otg, encapFlow, 0, float64(tolerancePct)+0.99)
	}
	t.Log("Verify packet load balancing as per the programmed weight")
	validateTrafficDistribution(t, args.ate, loadBalancePercent)
	var pcapFileList []string
	for _, capturePort := range capturePortList {
		bytes := args.otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(capturePort))
		pcapFileName, err := os.CreateTemp("", "pcap")
		if err != nil {
			t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
		}
		if _, err := pcapFileName.Write(bytes); err != nil {
			t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
		}
		pcapFileName.Close()
		defer os.Remove(pcapFileName.Name())
		pcapFileList = append(pcapFileList, pcapFileName.Name())
	}
	validatePackets(t, pcapFileList, checkEncap, headerDstIP)
	args.otgConfig.Captures().Clear()
	args.otg.PushConfig(t, args.otgConfig)
	args.otg.StartProtocols(t)
}

// waitForFlowMetricsReady waits via gnmi.Watch until a flow's TX/RX counters stabilize after stopping traffic.
func waitForFlowMetricsReady(t *testing.T, otgDev *otg.OTG, flowName string, timeout time.Duration) {
	t.Helper()
	const stableReads = 2
	type counters struct {
		tx uint64
		rx uint64
	}
	var (
		prev      counters
		havePrev  bool
		stableCnt int
		last      counters
	)
	_, ok := gnmi.Watch(t, otgDev, gnmi.OTG().Flow(flowName).State(), timeout, func(val *ygnmi.Value[*otgtelemetry.Flow]) bool {
		flowMetric, present := val.Val()
		if !present || flowMetric == nil || flowMetric.GetCounters() == nil {
			return false
		}
		cur := counters{
			tx: flowMetric.GetCounters().GetOutPkts(),
			rx: flowMetric.GetCounters().GetInPkts(),
		}
		last = cur
		if cur.tx == 0 {
			return false
		}
		if havePrev && cur == prev {
			stableCnt++
			if stableCnt >= stableReads {
				return true
			}
		} else {
			stableCnt = 0
		}
		prev = cur
		havePrev = true
		return false
	}).Await(t)
	if !ok {
		t.Fatalf("Flow %q metrics did not stabilize within %s (last tx=%d rx=%d)", flowName, timeout, last.tx, last.rx)
	}
	t.Logf("Flow %q metrics stabilized: tx=%d rx=%d", flowName, last.tx, last.rx)
}

func validatePackets(t *testing.T, filename []string, checkEncap bool, headerDstIP map[string][]string) {
	t.Helper()
	for index, file := range filename {
		fileStat, err := os.Stat(file)
		if err != nil {
			t.Errorf("Filestat for pcap file failed %s", err)
		}
		fileSize := fileStat.Size()
		if fileSize > 0 {
			handle, err := pcap.OpenOffline(file)
			if err != nil {
				t.Errorf("Unable to open the pcap file, error: %s", err)
			} else {
				packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
				if checkEncap {
					validateTrafficEncap(t, packetSource, headerDstIP, index)
				}
			}
			defer handle.Close()
		}
	}
}

func validateTrafficEncap(t *testing.T, packetSource *gopacket.PacketSource, headerDstIP map[string][]string, index int) {
	t.Helper()
	for packet := range packetSource.Packets() {
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
			if ipPacket.DstIP.String() == otgIsisPort8LoopV4 {
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
		},
		)
		if got, ok := operStatus.Await(t); !ok {
			t.Errorf("Get(DUT %v oper status): got %v, want %v", port, got, wantStatus)
		}
	}
}

// setDUTInterfaceState sets the admin state on the dut interface
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

func validateTrafficDistribution(t *testing.T, ate *ondatra.ATEDevice, wantWeights []float64) {
	inFramesAllPorts := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().PortAny().Counters().InFrames().State())
	// Skip source port, Port1.
	gotWeights, sum := normalize(inFramesAllPorts[1:])
	if sum == 0 {
		t.Errorf("Total received frames across egress ports is 0, want > 0")
	}

	t.Log("got ratio:", gotWeights)
	t.Log("want ratio:", wantWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, tolerance)); diff != "" {
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}
}

// validateEncapNHTelemetry verifies AFT telemetry for the encap next-hops used by
// the 138.0.11.0/24 entry in ENCAP_TE_VRF_A. Each next-hop (NH#101 and NH#102)
// is programmed via gRIBI with encapsulate_header IPV4, so telemetry is expected
// to report next-hop/state/encapsulate-header as IPV4 and
// next-hop/state/origin-protocol as GRIBI.
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

// TestEncapFrr is to test Test FRR behaviors with encapsulation scenarios
func TestEncapFrr(t *testing.T) {
	ctx := context.Background()
	dut := ondatra.DUT(t, "dut")

	gribic := dut.RawAPIs().GRIBI(t)
	ate := ondatra.ATE(t, "ate")
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

	configureDUT(t, dut, dutPorts)

	t.Log("Apply vrf selection policy to DUT port-1")
	vrfpolicy.ConfigureVRFSelectionPolicy(t, dut, vrfpolicy.VRFPolicyC)

	if deviations.GRIBIMACOverrideStaticARPStaticRoute(dut) {
		staticARPWithMagicUniversalIP(t, dut)
	}

	t.Log("Install BGP route resolved by ISIS.")
	t.Log("Configure ISIS on DUT")
	configureISIS(t, dut, dut.Port(t, "port8").Name(), dutAreaAddress, dutSysID)

	dutConfPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_BGP, "BGP")
	gnmi.Delete(t, dut, dutConfPath.Config())
	dutConf := bgpCreateNbr(dutAS, dut)
	gnmi.Replace(t, dut, dutConfPath.Config(), dutConf)
	fptest.LogQuery(t, "DUT BGP Config", dutConfPath.Config(), gnmi.Get(t, dut, dutConfPath.Config()))

	otg := ate.OTG()
	otgConfig := configureOTG(t, otg, atePorts)

	verifyISISTelemetry(t, dut, dutPorts[7].Name())
	verifyBgpTelemetry(t, dut)

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
			DownPortList:           []string{"port2", "port3", "port4", "port6"},
			CapturePortList:        []string{atePortNamelist[7]},
			EncapHeaderOuterIPList: []string{},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{0, 0, 0, 0, 0, 0, 1},
			TestID:                 "teVrf222NoMatch",
		},
		&baseScenario.TestCase{
			// README Test-9: Update NHG#101 to point to unresolvable next-hops in TE_VRF_111
			// without shutting down physical ports; traffic falls back to DEFAULT VRF on port-8.
			Desc:                   "Test-9: no match in TE_VRF_111",
			DownPortList:           []string{},
			CapturePortList:        []string{atePortNamelist[7]},
			EncapHeaderOuterIPList: []string{},
			EncapHeaderInnerIPList: []string{ipv4InnerDst},
			TrafficDestIP:          ipv4InnerDst,
			LoadBalancePercent:     []float64{0, 0, 0, 0, 0, 0, 1},
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
			portList := []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"}
			verifyPortStatus(t, args, portList, true)

			t.Log("Flush existing gRIBI routes before test.")
			if err := gribi.FlushAll(client); err != nil {
				t.Fatal(err)
			}
			baseCapturePortList := []string{atePortNamelist[1], atePortNamelist[5]}
			configureGribiRoute(ctx, t, dut, args.client)

			// For Test-3 (primaryBackupRoutingSingle), Test-6 (primaryBackupRoutingAll), and
			// Test-8 (teVrf222NoMatch), README Step 1 updates the gRIBI NHGs first and Step 2
			// validates baseline hierarchical traffic distribution before shutting down ports.
			if tc.TestID == "primaryBackupRoutingSingle" {
				// Test-3 Step 1: Update NHG#1000 to decapsulate IPv4 and route in DEFAULT VRF.
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
				// Test-6 Step 1: Update NHG#1000 and NHG#1001 to decapsulate IPv4 and route in DEFAULT VRF.
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
			if tc.TestID == "teVrf222NoMatch" {
				// Test-8 Step 1: Update NHG#1000 and NHG#1001 to re-encapsulate to unresolvable
				// destinations 203.100.113.100 and 203.100.113.101 in TE_VRF_222.
				args.client.Modify().AddEntry(t,
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(1300).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
						WithIPinIP(ipv4OuterSrc222Addr, gribiIPv4EntryVRF2223).WithNextHopNetworkInstance(niTEVRF222),
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(1301).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
						WithIPinIP(ipv4OuterSrc222Addr, gribiIPv4EntryVRF2224).WithNextHopNetworkInstance(niTEVRF222),
					fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithID(1000).AddNextHop(1300, 1).WithBackupNHG(2000),
					fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithID(1001).AddNextHop(1301, 1).WithBackupNHG(2000),
				)
				if err := awaitTimeout(ctx, t, args.client, 2*time.Minute); err != nil {
					t.Logf("Could not program entries via client, got err, check error codes: %v", err)
				}
				res := args.client.Results(t)
				for _, nhIdx := range []uint64{1300, 1301} {
					chk.HasResult(t, res,
						fluent.OperationResult().
							WithOperationType(constants.Add).
							WithNextHopOperation(nhIdx).
							WithProgrammingResult(fluent.InstalledInFIB).
							AsResult(),
						chk.IgnoreOperationID(),
					)
				}
				for _, nhgIdx := range []uint64{1000, 1001} {
					chk.HasResult(t, res,
						fluent.OperationResult().
							WithOperationType(constants.Add).
							WithNextHopGroupOperation(nhgIdx).
							WithProgrammingResult(fluent.InstalledInFIB).
							AsResult(),
						chk.IgnoreOperationID(),
					)
				}
			}

			// Validate baseline traffic distribution per hierarchical weights (README Test-1..Test-9 baseline / Step 2).
			createFlow(t, otgConfig, otg, ipv4InnerDst)
			captureState := startCapture(t, args, baseCapturePortList)
			sendTraffic(t, args, baseCapturePortList, captureState)
			baseHeaderDstIP := map[string][]string{"outerIP": {gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112}, "innerIP": {ipv4InnerDst, ipv4InnerDst}}
			baseLoadBalancePercent := []float64{0.0156, 0.0468, 0.1875, 0, 0.75, 0, 0}
			verifyTraffic(t, args, baseCapturePortList, baseLoadBalancePercent, !wantLoss, checkEncap, baseHeaderDstIP)

			if tc.TestID == "encapNoMatch" {
				// Test-7 Step 1: In ENCAP_TE_VRF_A, add a 0.0.0.0/0 static route pointing to the DEFAULT VRF using gNMI.
				staticDefaultRoute := []*cfgplugins.StaticRouteCfg{
					{
						NetworkInstance:     niEncapTeVrfA,
						Prefix:              "0.0.0.0/0",
						NextNetworkInstance: deviations.DefaultNetworkInstance(dut),
					},
				}
				cfgplugins.ConfigureStaticRoutesInVRF(t, dut, staticDefaultRoute)
				t.Cleanup(func() {
					cfgplugins.RemoveStaticRoutesInVRF(t, dut, staticDefaultRoute)
				})
				createFlow(t, otgConfig, otg, noMatchEncapDest)
			}
			if tc.TestID == "teVrf111NoMatch" {
				// Test-9 Step 1: Update NHG#101 to point to next-hops 203.100.113.1 and 203.100.113.2
				// which have no matching routes in TE_VRF_111.
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

			if deviations.ReducedEcmpSetOnMixedEncapDecapNh(dut) {
				if tc.TestID == "primaryBackupSingle" || tc.TestID == "primaryBackupRoutingSingle" {
					tc.CapturePortList = []string{atePortNamelist[5]}
					tc.LoadBalancePercent = []float64{0, 0, 0, 0, 1, 0, 0}
				}
			}

			captureState = startCapture(t, args, tc.CapturePortList)
			if len(tc.DownPortList) > 0 {
				t.Logf("Bring down ports %s", tc.DownPortList)
				portState(t, args, tc.DownPortList, false)
				t.Cleanup(func() {
					portState(t, args, tc.DownPortList, true)
				})
				t.Log("Verify the port status after bringing down the ports")
				verifyPortStatus(t, args, tc.DownPortList, false)
			}
			sendTraffic(t, args, tc.CapturePortList, captureState)
			headerDstIP := map[string][]string{"outerIP": tc.EncapHeaderOuterIPList, "innerIP": tc.EncapHeaderInnerIPList}

			if deviations.EncapTunnelShutBackupNhgZeroTraffic(dut) {
				if tc.TestID == "primarySingle" || tc.TestID == "primaryBackupSingle" || tc.TestID == "primaryBackupRoutingSingle" {
					tc.LoadBalancePercent = []float64{0, 0, 0, 0, 1, 0, 0}
				} else if tc.TestID == "primaryAll" {
					tc.LoadBalancePercent = []float64{0, 0, 0, 0, 0, 0, 1}
				}
			}
			verifyTraffic(t, args, tc.CapturePortList, tc.LoadBalancePercent, !wantLoss, checkEncap, headerDstIP)
		})
	}

	// Execute Test-10: Multi-VRF (ENCAP_TE_VRF_A and ENCAP_TE_VRF_B) dual-stack (IPv4 and IPv6)
	// encap path convergence into REPAIR VRF on primary tunnel edge cut.
	t.Run("Test-10: multi-VRF (ENCAP_TE_VRF_A and ENCAP_TE_VRF_B) dual-stack (IPv4 and IPv6) encap path convergence into REPAIR VRF on primary tunnel edge cut", func(t *testing.T) {
		runTest10MultiVRFDualStackRepairConvergence(ctx, t, args)
	})

	// Execute Test-11: Transit IP-in-IP encapsulated (4in4 and 6in4) traffic convergence and
	// re-encapsulation via REPAIR VRF on primary tunnel edge cut.
	t.Run("Test-11: transit IP-in-IP encapsulated (4in4 and 6in4) traffic convergence and re-encapsulation via REPAIR VRF on primary tunnel edge cut", func(t *testing.T) {
		runTest11TransitIPinIPReencapRepairConvergence(ctx, t, args)
	})
}

// dualStackFlowSpec defines parameters for a single OTG flow in Test-10 or Test-11.
type dualStackFlowSpec struct {
	name       string
	isIPv6     bool
	isTransit  bool
	dscpValues []uint32
	outerSrcIP string
	outerDsts  []string
	innerDstIP string
}

// createMultiFlows configures multiple simultaneous OTG flows on ATE port-1 as required by Test-10 and Test-11.
func createMultiFlows(t *testing.T, config gosnappi.Config, otgDev *otg.OTG, specs []dualStackFlowSpec) []string {
	t.Helper()
	config.Flows().Clear()
	var flowNames []string

	v4RxNames := []string{
		otgPortDevices[1].Name() + ".IPv4", otgPortDevices[2].Name() + ".IPv4",
		otgPortDevices[3].Name() + ".IPv4", otgPortDevices[4].Name() + ".IPv4",
		otgPortDevices[5].Name() + ".IPv4", otgPortDevices[6].Name() + ".IPv4",
		otgPortDevices[7].Name() + ".IPv4",
	}
	v6RxNames := []string{
		otgPortDevices[1].Name() + ".IPv6", otgPortDevices[2].Name() + ".IPv6",
		otgPortDevices[3].Name() + ".IPv6", otgPortDevices[4].Name() + ".IPv6",
		otgPortDevices[5].Name() + ".IPv6", otgPortDevices[6].Name() + ".IPv6",
		otgPortDevices[7].Name() + ".IPv6",
	}

	for _, spec := range specs {
		f := gosnappi.NewFlow().SetName(spec.name)
		f.Metrics().SetEnable(true)
		if spec.isIPv6 && !spec.isTransit {
			f.TxRx().Device().SetTxNames([]string{otgPortDevices[0].Name() + ".IPv6"}).SetRxNames(v6RxNames)
		} else {
			f.TxRx().Device().SetTxNames([]string{otgPortDevices[0].Name() + ".IPv4"}).SetRxNames(v4RxNames)
		}
		f.Size().SetFixed(512)
		f.Rate().SetPps(100)
		f.Duration().Continuous()

		eth := f.Packet().Add().Ethernet()
		eth.Src().SetValue(ateSrcPortMac)

		if spec.isTransit {
			// Pre-encapsulated outer IPv4 header for Test-11 transit flows.
			outerIPv4 := f.Packet().Add().Ipv4()
			outerIPv4.Src().SetValue(spec.outerSrcIP)
			outerIPv4.Dst().SetValues(spec.outerDsts)
			outerIPv4.Priority().Dscp().Phb().SetValues(spec.dscpValues)
			if spec.isIPv6 {
				outerIPv4.Protocol().SetValue(ipv6OverIPv4Protocol)
			} else {
				outerIPv4.Protocol().SetValue(ipOverIPProtocol)
			}
		}

		if spec.isIPv6 {
			ip6 := f.Packet().Add().Ipv6()
			ip6.Src().Increment().SetStart(ateSrcNetV6FirstIP).SetStep("::1").SetCount(ateSrcNetCount)
			ip6.Dst().SetValue(spec.innerDstIP)
			ip6.TrafficClass().SetValues(dscpToTrafficClass(spec.dscpValues))
		} else {
			ip4 := f.Packet().Add().Ipv4()
			ip4.Src().Increment().SetStart(ateSrcNetFirstIP).SetStep("0.0.0.1").SetCount(ateSrcNetCount)
			ip4.Dst().SetValue(spec.innerDstIP)
			ip4.Priority().Dscp().Phb().SetValues(spec.dscpValues)
		}

		udp := f.Packet().Add().Udp()
		udp.DstPort().Increment().SetStart(1).SetCount(50000).SetStep(1)
		udp.SrcPort().Increment().SetStart(1).SetCount(50000).SetStep(1)

		config.Flows().Append(f)
		flowNames = append(flowNames, spec.name)
	}

	otgDev.PushConfig(t, config)
	otgDev.StartProtocols(t)
	otgutils.WaitForARP(t, otgDev, config, "IPv4")
	otgutils.WaitForARP(t, otgDev, config, "IPv6")
	return flowNames
}

// dscpToTrafficClass converts 6-bit DSCP values to 8-bit IPv6 Traffic Class values (DSCP << 2).
func dscpToTrafficClass(dscps []uint32) []uint32 {
	out := make([]uint32, len(dscps))
	for i, d := range dscps {
		out[i] = d << 2
	}
	return out
}

// sendAndVerifyMultiFlows starts packet capture while all ports are UP, optionally brings down
// downPorts and invokes afterPortDown, transmits the given OTG flows while capturing packets on
// capturePorts, verifies packet loss is <= tolerancePct on every flow, validates aggregate port
// distribution, inspects captured packets for 4in4/6in4 encapsulation or native decapsulation,
// optionally invokes afterTraffic, and restores downPorts to UP.
func sendAndVerifyMultiFlows(t *testing.T, args *testArgs, flowNames []string, downPorts []string, capturePorts []string, wantWeights []float64, expectEncap bool, wantOuterSrc string, wantOuterDsts []string, afterPortDown func(), afterTraffic func()) {
	t.Helper()
	cs := startCapture(t, args, capturePorts)
	if len(downPorts) > 0 {
		t.Logf("Bringing down ports %v", downPorts)
		portState(t, args, downPorts, false)
		t.Cleanup(func() {
			portState(t, args, downPorts, true)
		})
		verifyPortStatus(t, args, downPorts, false)
	}
	if afterPortDown != nil {
		afterPortDown()
	}
	if !expectEncap {
		verifyISISTelemetry(t, args.dut, args.dut.Port(t, "port8").Name())
		verifyBgpTelemetry(t, args.dut)
	}

	args.otg.StartTraffic(t)
	for _, fn := range flowNames {
		gnmi.Watch(t, args.otg, gnmi.OTG().Flow(fn).Counters().OutPkts().State(), 30*time.Second, func(val *ygnmi.Value[uint64]) bool {
			outPkts, present := val.Val()
			return present && outPkts >= minTransmittedPkts
		}).Await(t)
	}
	args.otg.StopTraffic(t)
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	args.otg.SetControlState(t, cs)

	for _, fn := range flowNames {
		waitForFlowMetricsReady(t, args.otg, fn, time.Minute)
		otgutils.ExpectedTrafficLoss(t, args.otg, fn, 0, float64(tolerancePct)+0.99)
	}
	validateTrafficDistribution(t, args.ate, wantWeights)

	for _, capturePort := range capturePorts {
		rawBytes := args.otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(capturePort))
		tmpFile, err := os.CreateTemp("", "pcap")
		if err != nil {
			t.Fatalf("Could not create temporary pcap file: %v", err)
		}
		if _, err := tmpFile.Write(rawBytes); err != nil {
			t.Fatalf("Could not write bytes to pcap file: %v", err)
		}
		tmpFile.Close()
		defer os.Remove(tmpFile.Name())

		validateDualStackCaptureFile(t, tmpFile.Name(), capturePort, expectEncap, wantOuterSrc, wantOuterDsts)
	}
	if afterTraffic != nil {
		afterTraffic()
	}
	if len(downPorts) > 0 {
		t.Logf("Restoring ports %v to UP", downPorts)
		portState(t, args, downPorts, true)
		verifyPortStatus(t, args, downPorts, true)
	}
	args.otgConfig.Captures().Clear()
	args.otg.PushConfig(t, args.otgConfig)
	args.otg.StartProtocols(t)
}

// validateDualStackCaptureFile validates captured packets on capturePort for either 4in4/6in4
// IP-in-IP encapsulation (expectEncap=true) or native IPv4/IPv6 forwarding (expectEncap=false).
func validateDualStackCaptureFile(t *testing.T, pcapPath, capturePort string, expectEncap bool, wantOuterSrc string, wantOuterDsts []string) {
	t.Helper()
	info, err := os.Stat(pcapPath)
	if err != nil || info.Size() == 0 {
		t.Errorf("Capture file on port %s is empty or unreadable: %v", capturePort, err)
		return
	}
	handle, err := pcap.OpenOffline(pcapPath)
	if err != nil {
		t.Errorf("Unable to open pcap file on port %s: %v", capturePort, err)
		return
	}
	defer handle.Close()

	allowedOuterDst := make(map[string]bool, len(wantOuterDsts))
	for _, d := range wantOuterDsts {
		allowedOuterDst[d] = true
	}

	var matched int
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if expectEncap {
			outerLayer := packet.Layer(layers.LayerTypeIPv4)
			if outerLayer == nil {
				continue
			}
			outerIPv4, _ := outerLayer.(*layers.IPv4)
			if outerIPv4.Protocol != layers.IPProtocolIPv4 && outerIPv4.Protocol != layers.IPProtocolIPv6 {
				continue
			}
			if wantOuterSrc != "" && outerIPv4.SrcIP.String() != wantOuterSrc {
				t.Errorf("Port %s captured encap packet with outer SrcIP %s, want %s", capturePort, outerIPv4.SrcIP, wantOuterSrc)
			}
			if len(allowedOuterDst) > 0 && !allowedOuterDst[outerIPv4.DstIP.String()] {
				t.Errorf("Port %s captured encap packet with outer DstIP %s, want one of %v", capturePort, outerIPv4.DstIP, wantOuterDsts)
			}
			innerPkt := gopacket.NewPacket(outerIPv4.Payload, outerIPv4.NextLayerType(), gopacket.Default)
			if outerIPv4.Protocol == layers.IPProtocolIPv4 {
				if innerV4Layer := innerPkt.Layer(layers.LayerTypeIPv4); innerV4Layer != nil {
					innerV4, _ := innerV4Layer.(*layers.IPv4)
					if innerV4.DstIP.String() == ipv4InnerDst {
						matched++
					}
				}
			} else if outerIPv4.Protocol == layers.IPProtocolIPv6 {
				if innerV6Layer := innerPkt.Layer(layers.LayerTypeIPv6); innerV6Layer != nil {
					innerV6, _ := innerV6Layer.(*layers.IPv6)
					if innerV6.DstIP.String() == ipv6InnerDst {
						matched++
					}
				}
			}
		} else {
			if v4Layer := packet.Layer(layers.LayerTypeIPv4); v4Layer != nil {
				v4, _ := v4Layer.(*layers.IPv4)
				if v4.DstIP.String() == ipv4InnerDst && v4.Protocol != layers.IPProtocolIPv4 && v4.Protocol != layers.IPProtocolIPv6 {
					matched++
				}
			} else if v6Layer := packet.Layer(layers.LayerTypeIPv6); v6Layer != nil {
				v6, _ := v6Layer.(*layers.IPv6)
				if v6.DstIP.String() == ipv6InnerDst {
					matched++
				}
			}
		}
		if matched >= 5 {
			break
		}
	}
	if matched == 0 {
		t.Errorf("Port %s did not capture expected packets (expectEncap=%t, wantOuterSrc=%s, wantOuterDsts=%v)", capturePort, expectEncap, wantOuterSrc, wantOuterDsts)
	}
}

// runTest10MultiVRFDualStackRepairConvergence implements README Test-10:
// Multi-VRF (ENCAP_TE_VRF_A and ENCAP_TE_VRF_B) dual-stack (IPv4 and IPv6) encap path
// convergence into REPAIR VRF on primary tunnel edge cut.
func runTest10MultiVRFDualStackRepairConvergence(ctx context.Context, t *testing.T, args *testArgs) {
	t.Helper()
	allEgressPorts := []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"}
	verifyPortStatus(t, args, allEgressPorts, true)

	// Test-10 Step 1: Flush existing gRIBI routes, program baseline gRIBI routes, and program
	// IPv6 entries in ENCAP_TE_VRF_A plus IPv4/IPv6 entries in ENCAP_TE_VRF_B with inverted weights (3:1).
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}
	configureGribiRoute(ctx, t, args.dut, args.client)
	baseScenario.ConfigureMultiVRFDualStackGribiRoutes(ctx, t, args.dut, args.client, niEncapTeVrfA, niEncapTeVrfB)

	// Test-10 Step 2: Verify via gNMI AFT state telemetry that IPv4 and IPv6 entries in
	// ENCAP_TE_VRF_A and ENCAP_TE_VRF_B are installed and NHG#1/NHG#3 report backup-next-hop-group = 3000.
	defNI := deviations.DefaultNetworkInstance(args.dut)
	baseScenario.VerifyMultiVRFAFTState(t, args.dut,
		map[string]map[string]uint64{
			niEncapTeVrfA: {gribiIPv4EntryEncapVRF + "/24": 101},
			niEncapTeVrfB: {gribiIPv4EntryEncapVRF + "/24": 102},
		},
		map[string]map[string]uint64{
			niEncapTeVrfA: {baseScenario.GribiIPv6EntryEncapVRF + "/" + baseScenario.GribiIPv6EntryMaskLen: 101},
			niEncapTeVrfB: {baseScenario.GribiIPv6EntryEncapVRF + "/" + baseScenario.GribiIPv6EntryMaskLen: 102},
		},
		map[string]map[uint64]uint64{
			defNI: {1: 3000, 3: 3000},
		},
	)

	// Test-10 Step 3: Configure and run 4 simultaneous dual-stack flows on ATE port-1:
	// Flow 1 (ENCAP_TE_VRF_A IPv4 DSCP 10/18), Flow 2 (ENCAP_TE_VRF_A IPv6 DSCP 10/18),
	// Flow 3 (ENCAP_TE_VRF_B IPv4 DSCP 20/28), Flow 4 (ENCAP_TE_VRF_B IPv6 DSCP 20/28).
	// Notice that ENCAP_TE_VRF_A uses weights 1:3 (25%:75%) and ENCAP_TE_VRF_B uses weights 3:1 (75%:25%),
	// so equal-rate simultaneous flows across both VRFs combine to 50% on tunnel 203.0.113.1 and 50% on tunnel 203.0.113.2.
	flowSpecs := []dualStackFlowSpec{
		{
			name:       "Test10_Flow1_VrfA_IPv4",
			isIPv6:     false,
			dscpValues: []uint32{dscpEncapA1, dscpEncapA2},
			innerDstIP: ipv4InnerDst,
		},
		{
			name:       "Test10_Flow2_VrfA_IPv6",
			isIPv6:     true,
			dscpValues: []uint32{dscpEncapA1, dscpEncapA2},
			innerDstIP: ipv6InnerDst,
		},
		{
			name:       "Test10_Flow3_VrfB_IPv4",
			isIPv6:     false,
			dscpValues: []uint32{dscpEncapB1, dscpEncapB2},
			innerDstIP: ipv4InnerDst,
		},
		{
			name:       "Test10_Flow4_VrfB_IPv6",
			isIPv6:     true,
			dscpValues: []uint32{dscpEncapB1, dscpEncapB2},
			innerDstIP: ipv6InnerDst,
		},
	}
	flowNames := createMultiFlows(t, args.otgConfig, args.otg, flowSpecs)

	// Test-10 Step 4: Validate baseline 4in4 and 6in4 encapsulation on primary TE_VRF_111 tunnels
	// (outer src 198.51.100.111, outer dst 203.0.113.1 on port-2..4 and 203.0.113.2 on port-6) with 0% loss.
	// Combined 50%/50% tunnel split across NHG#1 (port2:port3:port4 = 1:3:12 -> 0.03125, 0.09375, 0.375) and NHG#3 (port6 = 0.50).
	baseCombinedWeights := []float64{0.03125, 0.09375, 0.375, 0, 0.50, 0, 0}
	sendAndVerifyMultiFlows(t, args, flowNames, nil,
		[]string{atePortNamelist[1], atePortNamelist[5]},
		baseCombinedWeights, true, ipv4OuterSrc111Addr,
		[]string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112},
		nil, nil,
	)

	// Test-10 Step 5 & Step 6: Shut down DUT port-2, port-3, and port-4 to simulate a primary tunnel
	// edge cut on 203.0.113.1/32, verify oper-status DOWN, and validate convergence onto REPAIR VRF
	// (port-5 outer dst 203.0.113.100 with outer src 198.51.100.222, and port-6 outer dst 203.0.113.2 with outer src 198.51.100.111).
	cutPortsSingle := []string{"port2", "port3", "port4"}
	singleCutWeights := []float64{0, 0, 0, 0.50, 0.50, 0, 0}
	singleCutCapturePorts := []string{atePortNamelist[4], atePortNamelist[5]}
	singleCutOuterSrc := ""
	singleCutOuterDsts := []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF1112}
	if deviations.EncapTunnelShutBackupNhgZeroTraffic(args.dut) || deviations.ReducedEcmpSetOnMixedEncapDecapNh(args.dut) {
		singleCutWeights = []float64{0, 0, 0, 0, 1, 0, 0}
		singleCutCapturePorts = []string{atePortNamelist[5]}
		singleCutOuterSrc = ipv4OuterSrc111Addr
		singleCutOuterDsts = []string{gribiIPv4EntryVRF1112}
	}
	sendAndVerifyMultiFlows(t, args, flowNames, cutPortsSingle,
		singleCutCapturePorts,
		singleCutWeights, true, singleCutOuterSrc,
		singleCutOuterDsts,
		nil,
		func() {
			baseScenario.VerifyNoContinuousDiscards(t, args.dut, []*ondatra.Port{
				args.dut.Port(t, "port1"),
				args.dut.Port(t, "port5"),
				args.dut.Port(t, "port6"),
			})
		},
	)

	// Re-program gRIBI routes after Step 6 port restoration so any interface-bound next-hops
	// withdrawn during Step 6 are cleanly reinstalled before Step 7.
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}
	configureGribiRoute(ctx, t, args.dut, args.client)
	baseScenario.ConfigureMultiVRFDualStackGribiRoutes(ctx, t, args.dut, args.client, niEncapTeVrfA, niEncapTeVrfB)

	// Test-10 Step 7: Shut down DUT port-2, port-3, port-4, and port-6 so all primary tunnels in TE_VRF_111
	// are unviable, and validate 100% convergence onto REPAIR VRF backup tunnels in TE_VRF_222 (port-5 and port-7)
	// with outer source IP 198.51.100.222 and <= 2% loss.
	cutPortsAllPrimary := []string{"port2", "port3", "port4", "port6"}
	allCutWeights := []float64{0, 0, 0, 0.50, 0, 0.50, 0}
	allCutCapturePorts := []string{atePortNamelist[4], atePortNamelist[6]}
	allCutExpectEncap := true
	allCutOuterSrc := ipv4OuterSrc222Addr
	allCutOuterDsts := []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF2222, gribiIPv4EntryVRF2225}
	if deviations.EncapTunnelShutBackupNhgZeroTraffic(args.dut) {
		allCutWeights = []float64{0, 0, 0, 0, 0, 0, 1}
		allCutCapturePorts = []string{atePortNamelist[7]}
		allCutExpectEncap = false
		allCutOuterSrc = ""
		allCutOuterDsts = nil
	}
	sendAndVerifyMultiFlows(t, args, flowNames, cutPortsAllPrimary,
		allCutCapturePorts,
		allCutWeights, allCutExpectEncap, allCutOuterSrc,
		allCutOuterDsts,
		nil,
		func() {
			baseScenario.VerifyNoContinuousDiscards(t, args.dut, []*ondatra.Port{
				args.dut.Port(t, "port1"),
				args.dut.Port(t, "port5"),
				args.dut.Port(t, "port7"),
			})
		},
	)

	// Re-program gRIBI routes after Step 7 port restoration so primary TE_VRF_111 next-hops
	// and REPAIR VRF next-hops are cleanly restored before Step 8.
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}
	configureGribiRoute(ctx, t, args.dut, args.client)
	baseScenario.ConfigureMultiVRFDualStackGribiRoutes(ctx, t, args.dut, args.client, niEncapTeVrfA, niEncapTeVrfB)

	// Test-10 Step 8: With DUT port-2, port-3, port-4, and port-6 re-enabled and verified oper-status UP,
	// confirm traffic restores to primary TE_VRF_111 tunnels with 0% steady-state loss.
	sendAndVerifyMultiFlows(t, args, flowNames, nil,
		[]string{atePortNamelist[1], atePortNamelist[5]},
		baseCombinedWeights, true, ipv4OuterSrc111Addr,
		[]string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112},
		nil, nil,
	)
}

// runTest11TransitIPinIPReencapRepairConvergence implements README Test-11:
// Transit IP-in-IP encapsulated (4in4 and 6in4) traffic convergence and re-encapsulation
// via REPAIR VRF on primary tunnel edge cut.
func runTest11TransitIPinIPReencapRepairConvergence(ctx context.Context, t *testing.T, args *testArgs) {
	t.Helper()
	allEgressPorts := []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"}

	// Test-11 Step 1: Reset DUT and ATE to baseline setup with all ports UP and baseline gRIBI routes installed.
	verifyPortStatus(t, args, allEgressPorts, true)
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}
	configureGribiRoute(ctx, t, args.dut, args.client)
	baseScenario.ConfigureMultiVRFDualStackGribiRoutes(ctx, t, args.dut, args.client, niEncapTeVrfA, niEncapTeVrfB)

	// Test-11 Step 2: Verify via gNMI state telemetry that vrf_selection_policy_c rules 3, 4, 7, 8, 11, and 12
	// match outer source 198.51.100.111/32 and direct packets missing DECAP_TE_VRF to decap-fallback-network-instance TE_VRF_111.
	vrfpolicy.VerifyVRFSelectionPolicyCState(t, args.dut)

	// Test-11 Step 3: Configure 4 pre-encapsulated IP-in-IP transit flows on ATE port-1:
	// Transit Flow 1 (4in4 DSCP 10), Transit Flow 2 (6in4 DSCP 10),
	// Transit Flow 3 (4in4 DSCP 20), Transit Flow 4 (6in4 DSCP 20).
	transitSpecs := []dualStackFlowSpec{
		{
			name:       "Test11_TransitFlow1_4in4_DSCP10",
			isIPv6:     false,
			isTransit:  true,
			dscpValues: []uint32{dscpEncapA1},
			outerSrcIP: ipv4OuterSrc111Addr,
			outerDsts:  []string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112},
			innerDstIP: ipv4InnerDst,
		},
		{
			name:       "Test11_TransitFlow2_6in4_DSCP10",
			isIPv6:     true,
			isTransit:  true,
			dscpValues: []uint32{dscpEncapA1},
			outerSrcIP: ipv4OuterSrc111Addr,
			outerDsts:  []string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112},
			innerDstIP: ipv6InnerDst,
		},
		{
			name:       "Test11_TransitFlow3_4in4_DSCP20",
			isIPv6:     false,
			isTransit:  true,
			dscpValues: []uint32{dscpEncapB1},
			outerSrcIP: ipv4OuterSrc111Addr,
			outerDsts:  []string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112},
			innerDstIP: ipv4InnerDst,
		},
		{
			name:       "Test11_TransitFlow4_6in4_DSCP20",
			isIPv6:     true,
			isTransit:  true,
			dscpValues: []uint32{dscpEncapB1},
			outerSrcIP: ipv4OuterSrc111Addr,
			outerDsts:  []string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112},
			innerDstIP: ipv6InnerDst,
		},
	}
	flowNames := createMultiFlows(t, args.otgConfig, args.otg, transitSpecs)

	// Test-11 Step 4: Validate steady-state forwarding via TE_VRF_111 (outer dst 203.0.113.1 on port-2..4
	// and outer dst 203.0.113.2 on port-6) with outer source 198.51.100.111 preserved and 0% packet loss.
	steadyTransitWeights := []float64{0.03125, 0.09375, 0.375, 0, 0.50, 0, 0}
	sendAndVerifyMultiFlows(t, args, flowNames, nil,
		[]string{atePortNamelist[1], atePortNamelist[5]},
		steadyTransitWeights, true, ipv4OuterSrc111Addr,
		[]string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112},
		nil, nil,
	)

	// Test-11 Step 5 & Step 6: Shut down DUT port-2, port-3, port-4, and port-6, verify oper-status DOWN,
	// verify AFT next-hop decapsulate-header/encapsulate-header state, and validate that transit flows
	// converge via NHG#3000 -> REPAIR VRF (NHG#1000/NHG#1001) and are re-encapsulated with new outer
	// source IP 198.51.100.222 onto port-5 (203.0.113.100) and port-7 (203.0.113.101/203.0.113.102).
	primaryTransitPorts := []string{"port2", "port3", "port4", "port6"}
	reencapWeights := []float64{0, 0, 0, 0.50, 0, 0.50, 0}
	reencapCapturePorts := []string{atePortNamelist[4], atePortNamelist[6]}
	reencapExpectEncap := true
	reencapOuterSrc := ipv4OuterSrc222Addr
	reencapOuterDsts := []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF2222, gribiIPv4EntryVRF2225}
	if deviations.EncapTunnelShutBackupNhgZeroTraffic(args.dut) {
		reencapWeights = []float64{0, 0, 0, 0, 0, 0, 1}
		reencapCapturePorts = []string{atePortNamelist[7]}
		reencapExpectEncap = false
		reencapOuterSrc = ""
		reencapOuterDsts = nil
	}
	sendAndVerifyMultiFlows(t, args, flowNames, primaryTransitPorts,
		reencapCapturePorts,
		reencapWeights, reencapExpectEncap, reencapOuterSrc,
		reencapOuterDsts,
		func() {
			baseScenario.VerifyRepairReencapNextHopState(t, args.dut, []uint64{1000, 1001, 1004})
		},
		func() {
			baseScenario.VerifyNoContinuousDiscards(t, args.dut, []*ondatra.Port{
				args.dut.Port(t, "port1"),
				args.dut.Port(t, "port5"),
				args.dut.Port(t, "port7"),
			})
		},
	)

	// Re-program gRIBI routes after Step 6 port restoration so primary TE_VRF_111 next-hops
	// and REPAIR VRF next-hops are cleanly reinstalled before Step 7/8.
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}
	configureGribiRoute(ctx, t, args.dut, args.client)
	baseScenario.ConfigureMultiVRFDualStackGribiRoutes(ctx, t, args.dut, args.client, niEncapTeVrfA, niEncapTeVrfB)

	// Test-11 Step 7 & Step 8: Shut down DUT port-2, port-3, port-4, port-5, port-6, and port-7 so both
	// TE_VRF_111 primary tunnels and TE_VRF_222 backup tunnels are unviable, triggering backup_next_hop_group: 2000
	// (NH#2000 decapsulate to DEFAULT VRF), and validate that 100% of 4in4 and 6in4 transit packets are
	// decapsulated and forwarded as native IPv4/IPv6 out port-8.
	allPrimaryAndRepairTransitPorts := []string{"port2", "port3", "port4", "port5", "port6", "port7"}
	decapFallbackWeights := []float64{0, 0, 0, 0, 0, 0, 1}
	sendAndVerifyMultiFlows(t, args, flowNames, allPrimaryAndRepairTransitPorts,
		[]string{atePortNamelist[7]},
		decapFallbackWeights, false, "", nil,
		nil, nil,
	)

	// Re-program gRIBI routes after Step 8 port restoration so primary TE_VRF_111 next-hops
	// are cleanly reinstalled before Step 9 restoration check.
	if err := gribi.FlushAll(args.client); err != nil {
		t.Fatal(err)
	}
	configureGribiRoute(ctx, t, args.dut, args.client)
	baseScenario.ConfigureMultiVRFDualStackGribiRoutes(ctx, t, args.dut, args.client, niEncapTeVrfA, niEncapTeVrfB)

	// Test-11 Step 9: With DUT port-2, port-3, port-4, port-5, port-6, and port-7 re-enabled and verified
	// oper-status UP, confirm all transit flows revert to the primary TE_VRF_111 tunnels with 0% steady-state loss.
	sendAndVerifyMultiFlows(t, args, flowNames, nil,
		[]string{atePortNamelist[1], atePortNamelist[5]},
		steadyTransitWeights, true, ipv4OuterSrc111Addr,
		[]string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112},
		nil, nil,
	)
}
