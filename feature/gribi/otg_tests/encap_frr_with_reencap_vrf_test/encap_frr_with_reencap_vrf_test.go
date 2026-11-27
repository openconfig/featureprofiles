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
	"github.com/openconfig/ondatra/netutil"
	"github.com/openconfig/ondatra/otg"
	"github.com/openconfig/ygnmi/ygnmi"
	"github.com/openconfig/ygot/ygot"
)

func TestMain(m *testing.M) {
	fptest.RunTests(m)
}

// Settings for configuring the baseline testbed with the 5-link full-duplex
// topology:
//
// ATE port-1 <------> port-1 DUT  (Rx on DUT: ingress flow; Tx on DUT: IS-IS + iBGP + decap fallback)
// DUT port-2 <------> port-2 ATE  (Tunnel 1 primary, 3 ECMP next-hops via distinct magic MACs)
// DUT port-3 <------> port-3 ATE  (Tunnel 1 FRR backup)
// DUT port-4 <------> port-4 ATE  (Tunnel 2 primary)
// DUT port-5 <------> port-5 ATE  (Tunnel 2 FRR backup)

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
	magicIP2               = "192.168.1.2"
	magicIP3               = "192.168.1.3"
	magicMAC               = "02:00:00:00:00:01"
	magicMAC2              = "02:00:00:00:00:02"
	magicMAC3              = "02:00:00:00:00:03"
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

		"dut:port5": "192.0.2.17",
		"ate:port5": "192.0.2.18",
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

	dutPort2DummyIP = attrs.Attributes{
		Desc:       "dutPort2",
		IPv4Sec:    "192.0.2.33",
		IPv4LenSec: plenIPv4,
	}
	otgPort2DummyIP = attrs.Attributes{
		Desc:    "otgPort2",
		IPv4:    "192.0.2.34",
		IPv4Len: plenIPv4,
	}
	dutPort2DummyIP2 = attrs.Attributes{
		Desc:       "dutPort2Nh2",
		IPv4Sec:    "192.0.2.49",
		IPv4LenSec: plenIPv4,
	}
	otgPort2DummyIP2 = attrs.Attributes{
		Desc:    "otgPort2Nh2",
		IPv4:    "192.0.2.50",
		IPv4Len: plenIPv4,
	}
	dutPort2DummyIP3 = attrs.Attributes{
		Desc:       "dutPort2Nh3",
		IPv4Sec:    "192.0.2.53",
		IPv4LenSec: plenIPv4,
	}
	otgPort2DummyIP3 = attrs.Attributes{
		Desc:    "otgPort2Nh3",
		IPv4:    "192.0.2.54",
		IPv4Len: plenIPv4,
	}
	dutPort3DummyIP = attrs.Attributes{
		Desc:       "dutPort3",
		IPv4Sec:    "192.0.2.37",
		IPv4LenSec: plenIPv4,
	}
	otgPort3DummyIP = attrs.Attributes{
		Desc:    "otgPort3",
		IPv4:    "192.0.2.38",
		IPv4Len: plenIPv4,
	}
	dutPort4DummyIP = attrs.Attributes{
		Desc:       "dutPort4",
		IPv4Sec:    "192.0.2.41",
		IPv4LenSec: plenIPv4,
	}
	otgPort4DummyIP = attrs.Attributes{
		Desc:    "otgPort4",
		IPv4:    "192.0.2.42",
		IPv4Len: plenIPv4,
	}
	dutPort5DummyIP = attrs.Attributes{
		Desc:       "dutPort5",
		IPv4Sec:    "192.0.2.45",
		IPv4LenSec: plenIPv4,
	}
	otgPort5DummyIP = attrs.Attributes{
		Desc:    "otgPort5",
		IPv4:    "192.0.2.46",
		IPv4Len: plenIPv4,
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
		for _, dp := range dutPortList {
			fptest.AssignToNetworkInstance(t, dut, dp.Name(), deviations.DefaultNetworkInstance(dut), 0)
		}
	}
	if deviations.ExplicitPortSpeed(dut) {
		for _, dp := range dutPortList {
			fptest.SetPortSpeed(t, dp)
		}
	}

	loopbackIntfName = resolveOrCreateLoopback(t, dut)
	if deviations.ExplicitInterfaceInDefaultVRF(dut) {
		fptest.AssignToNetworkInstance(t, dut, loopbackIntfName, deviations.DefaultNetworkInstance(dut), 0)
	}
	if deviations.GRIBIMACOverrideWithStaticARP(dut) {
		staticARPWithSpecificIP(t, dut)
	}
}

// resolveOrCreateLoopback reuses an existing DUT loopback address on any subinterface
// when lo0.0 is absent (e.g., Juniper with --use_mgmt_vrf=false where another lo0 unit
// already resides in master), and otherwise delegates to helpers.GetOrCreateLoopback.
func resolveOrCreateLoopback(t *testing.T, dut *ondatra.DUTDevice) string {
	t.Helper()
	if dut.Vendor() == ondatra.JUNIPER {
		lb := netutil.LoopbackInterface(t, dut, 0)
		lo0 := gnmi.OC().Interface(lb).Subinterface(0)
		if len(gnmi.LookupAll(t, dut, lo0.Ipv4().AddressAny().State())) == 0 {
			for _, v4Val := range gnmi.LookupAll(t, dut, gnmi.OC().Interface(lb).SubinterfaceAny().Ipv4().AddressAny().State()) {
				if v4, ok := v4Val.Val(); ok && v4.GetIp() != "" {
					dutlo0Attrs.IPv4 = v4.GetIp()
					for _, v6Val := range gnmi.LookupAll(t, dut, gnmi.OC().Interface(lb).SubinterfaceAny().Ipv6().AddressAny().State()) {
						if v6, ok := v6Val.Val(); ok && v6.GetIp() != "" {
							dutlo0Attrs.IPv6 = v6.GetIp()
							break
						}
					}
					t.Logf("Reusing existing DUT loopback %s IPv4=%s IPv6=%s", lb, dutlo0Attrs.IPv4, dutlo0Attrs.IPv6)
					return lb
				}
			}
		}
	}
	return helpers.GetOrCreateLoopback(t, dut, 0, 0, &dutlo0Attrs)
}

// configureBaseGribiRoutes programs the DEFAULT VRF next-hop hierarchy across the 5-link topology.
func configureBaseGribiRoutes(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client *fluent.GRIBIClient) {
	t.Helper()

	defaultNI := deviations.DefaultNetworkInstance(dut)
	p2Name := dut.Port(t, "port2").Name()
	p3Name := dut.Port(t, "port3").Name()
	p4Name := dut.Port(t, "port4").Name()
	p5Name := dut.Port(t, "port5").Name()

	switch {
	case deviations.GRIBIMACOverrideStaticARPStaticRoute(dut):
		client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(11).WithMacAddress(magicMAC).WithInterfaceRef(p2Name).WithIPAddress(magicIP),
			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(12).WithMacAddress(magicMAC2).WithInterfaceRef(p2Name).WithIPAddress(magicIP2),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),

			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(13).WithMacAddress(magicMAC3).WithInterfaceRef(p2Name).WithIPAddress(magicIP3),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(12).AddNextHop(13, 2),

			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(14).WithMacAddress(magicMAC).WithInterfaceRef(p3Name).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(13).AddNextHop(14, 1),

			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(15).WithMacAddress(magicMAC).WithInterfaceRef(p4Name).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(14).AddNextHop(15, 1),

			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(16).WithMacAddress(magicMAC).WithInterfaceRef(p5Name).WithIPAddress(magicIP),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(15).AddNextHop(16, 1),
		)
	case deviations.GRIBIMACOverrideWithStaticARP(dut):
		client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(11).WithMacAddress(magicMAC).WithInterfaceRef(p2Name).WithIPAddress(otgPort2DummyIP.IPv4),
			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(12).WithMacAddress(magicMAC2).WithInterfaceRef(p2Name).WithIPAddress(otgPort2DummyIP2.IPv4),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),

			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(13).WithMacAddress(magicMAC3).WithInterfaceRef(p2Name).WithIPAddress(otgPort2DummyIP3.IPv4),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(12).AddNextHop(13, 2),

			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(14).WithMacAddress(magicMAC).WithInterfaceRef(p3Name).WithIPAddress(otgPort3DummyIP.IPv4),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(13).AddNextHop(14, 1),

			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(15).WithMacAddress(magicMAC).WithInterfaceRef(p4Name).WithIPAddress(otgPort4DummyIP.IPv4),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(14).AddNextHop(15, 1),

			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(16).WithMacAddress(magicMAC).WithInterfaceRef(p5Name).WithIPAddress(otgPort5DummyIP.IPv4),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(15).AddNextHop(16, 1),
		)
	default:
		client.Modify().AddEntry(t,
			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(11).WithMacAddress(magicMAC).WithInterfaceRef(p2Name),
			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(12).WithMacAddress(magicMAC2).WithInterfaceRef(p2Name),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),

			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(13).WithMacAddress(magicMAC3).WithInterfaceRef(p2Name),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(12).AddNextHop(13, 2),

			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(14).WithMacAddress(magicMAC).WithInterfaceRef(p3Name),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(13).AddNextHop(14, 1),

			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(15).WithMacAddress(magicMAC).WithInterfaceRef(p4Name),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(14).AddNextHop(15, 1),

			fluent.NextHopEntry().WithNetworkInstance(defaultNI).
				WithIndex(16).WithMacAddress(magicMAC).WithInterfaceRef(p5Name),
			fluent.NextHopGroupEntry().WithNetworkInstance(defaultNI).
				WithID(15).AddNextHop(16, 1),
		)
	}
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program base NH/NHG entries via client, got err, check error codes: %v", err)
	}

	client.Modify().AddEntry(t,
		fluent.IPv4Entry().WithNetworkInstance(defaultNI).
			WithPrefix(gribiIPv4EntryDefVRF1+"/"+maskLen32).WithNextHopGroup(11),
		fluent.IPv4Entry().WithNetworkInstance(defaultNI).
			WithPrefix(gribiIPv4EntryDefVRF2+"/"+maskLen32).WithNextHopGroup(12),
		fluent.IPv4Entry().WithNetworkInstance(defaultNI).
			WithPrefix(gribiIPv4EntryDefVRF3+"/"+maskLen32).WithNextHopGroup(13),
		fluent.IPv4Entry().WithNetworkInstance(defaultNI).
			WithPrefix(gribiIPv4EntryDefVRF4+"/"+maskLen32).WithNextHopGroup(14),
		fluent.IPv4Entry().WithNetworkInstance(defaultNI).
			WithPrefix(gribiIPv4EntryDefVRF5+"/"+maskLen32).WithNextHopGroup(15),
	)
	if err := awaitTimeout(ctx, t, client, time.Minute); err != nil {
		t.Logf("Could not program base IPv4 entries via client, got err, check error codes: %v", err)
	}

	defaultVRFIPList := []string{gribiIPv4EntryDefVRF1, gribiIPv4EntryDefVRF2, gribiIPv4EntryDefVRF3, gribiIPv4EntryDefVRF4, gribiIPv4EntryDefVRF5}
	for _, ip := range defaultVRFIPList {
		chk.HasResult(t, client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(ip+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}
}

// configureGribiRoute configures Gribi route as per the requirements
func configureGribiRoute(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, client *fluent.GRIBIClient) {
	t.Helper()

	configureBaseGribiRoutes(ctx, t, dut, client)

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

// configStaticArps configures static ARP entries on an interface's subinterface 0.
func configStaticArps(p string, ipToMAC map[string]string) *oc.Interface {
	i := &oc.Interface{Name: ygot.String(p)}
	i.Type = oc.IETFInterfaces_InterfaceType_ethernetCsmacd
	s := i.GetOrCreateSubinterface(0)
	s4 := s.GetOrCreateIpv4()
	for ipv4addr, macAddr := range ipToMAC {
		n4 := s4.GetOrCreateNeighbor(ipv4addr)
		n4.LinkLayerAddress = ygot.String(macAddr)
	}
	return i
}

// configStaticArp configures a single static ARP entry.
func configStaticArp(p string, ipv4addr string, macAddr string) *oc.Interface {
	return configStaticArps(p, map[string]string{ipv4addr: macAddr})
}

func staticARPWithSpecificIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2DummyIP.NewOCInterface(p2.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2DummyIP2.NewOCInterface(p2.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), dutPort2DummyIP3.NewOCInterface(p2.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), dutPort3DummyIP.NewOCInterface(p3.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), dutPort4DummyIP.NewOCInterface(p4.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), dutPort5DummyIP.NewOCInterface(p5.Name(), dut))
	gnmi.Update(t, dut, gnmi.OC().Interface(p2.Name()).Config(), configStaticArps(p2.Name(), map[string]string{
		otgPort2DummyIP.IPv4:  magicMAC,
		otgPort2DummyIP2.IPv4: magicMAC2,
		otgPort2DummyIP3.IPv4: magicMAC3,
	}))
	gnmi.Update(t, dut, gnmi.OC().Interface(p3.Name()).Config(), configStaticArp(p3.Name(), otgPort3DummyIP.IPv4, magicMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p4.Name()).Config(), configStaticArp(p4.Name(), otgPort4DummyIP.IPv4, magicMAC))
	gnmi.Update(t, dut, gnmi.OC().Interface(p5.Name()).Config(), configStaticArp(p5.Name(), otgPort5DummyIP.IPv4, magicMAC))
}

func staticARPWithMagicUniversalIP(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	sb := &gnmi.SetBatch{}
	p2 := dut.Port(t, "port2")
	p3 := dut.Port(t, "port3")
	p4 := dut.Port(t, "port4")
	p5 := dut.Port(t, "port5")
	sp := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).Protocol(oc.PolicyTypes_INSTALL_PROTOCOL_TYPE_STATIC, deviations.StaticProtocolName(dut))
	for idx, p := range []*ondatra.Port{p2, p3, p4, p5} {
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
		gnmi.BatchUpdate(sb, sp.Static(magicIP+"/32").Config(), s)
		if p != p2 {
			gnmi.BatchUpdate(sb, gnmi.OC().Interface(p.Name()).Config(), configStaticArp(p.Name(), magicIP, magicMAC))
		}
	}
	for _, extraIP := range []string{magicIP2, magicIP3} {
		s := &oc.NetworkInstance_Protocol_Static{
			Prefix: ygot.String(extraIP + "/32"),
			NextHop: map[string]*oc.NetworkInstance_Protocol_Static_NextHop{
				"0": {
					Index: ygot.String("0"),
					InterfaceRef: &oc.NetworkInstance_Protocol_Static_NextHop_InterfaceRef{
						Interface: ygot.String(p2.Name()),
					},
				},
			},
		}
		gnmi.BatchUpdate(sb, sp.Static(extraIP+"/32").Config(), s)
	}
	gnmi.BatchUpdate(sb, gnmi.OC().Interface(p2.Name()).Config(), configStaticArps(p2.Name(), map[string]string{
		magicIP:  magicMAC,
		magicIP2: magicMAC2,
		magicIP3: magicMAC3,
	}))
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
	t.Logf("BGP adjacency for %s: %v", otgIsisPort1LoopV4, state)
	if want := oc.Bgp_Neighbor_SessionState_ESTABLISHED; state != want {
		t.Errorf("BGP peer %s status got %d, want %d", otgIsisPort1LoopV4, state, want)
	}
}

// configureOTG configures the topology of the ATE.
func configureOTG(t testing.TB, otg *otg.OTG, atePorts []*ondatra.Port) gosnappi.Config {
	t.Helper()
	config := gosnappi.NewConfig()
	var pmd100GFRPorts []string
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
		// Port 1 is bidirectional: it injects the test flow and runs IS-IS + iBGP for DEFAULT VRF fallback.
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

			// Advertise OTG Port1 loopback address via ISIS.
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
			otgPortDevices[4].Name() + ".IPv4",
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
	return otgutils.StartCapture(t, args.otg)
}

func sendTraffic(t *testing.T, args *testArgs, capturePortList []string, cs gosnappi.ControlState) {
	t.Helper()
	defer otgutils.StopCapture(t, args.otg, cs)
	otgutils.WaitForARP(t, args.otg, args.top, "IPv4")
	t.Logf("Starting traffic")
	args.otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	args.otg.StopTraffic(t)
}

func verifyTrafficMetrics(t *testing.T, args *testArgs, loadBalancePercent []float64, wantLoss bool) {
	t.Helper()
	waitForFlowMetricsReady(t, args.otg, encapFlow, 1*time.Minute)
	if wantLoss {
		otgutils.ExpectedTrafficLoss(t, args.otg, encapFlow, float64(100-tolerancePct), 100)
	} else if loadBalancePercent[0] > 0 {
		// port1 is excluded from flow RxNames because OTG forbids using the Tx port
		// in RxNames; combine flow Rx on port2..port5 with port1 InFrames.
		verifyFallbackTrafficLoss(t, args.otg, encapFlow, atePortNamelist[0], float64(tolerancePct)+0.99)
	} else {
		otgutils.ExpectedTrafficLoss(t, args.otg, encapFlow, 0, float64(tolerancePct)+0.99)
	}
	t.Log("Verify packet load balancing as per the programmed weight")
	validateTrafficDistribution(t, args.ate, loadBalancePercent)
}

// captureAndVerifyTraffic captures and validates each port in capturePortList
// sequentially so at most one OTG port capture is active at a time (avoiding
// per-linecard active capture limits on shared ATE cards).
func captureAndVerifyTraffic(t *testing.T, args *testArgs, downPortList, capturePortList []string, loadBalancePercent []float64, wantLoss, checkEncap bool, headerDstIP map[string][]string) []string {
	t.Helper()
	var pcapFileList []string
	for idx, capturePort := range capturePortList {
		captureState := startCapture(t, args, []string{capturePort})
		if len(downPortList) > 0 {
			if idx == 0 {
				t.Logf("Bring down ports %s", downPortList)
			}
			portState(t, args, downPortList, false)
			if idx == 0 {
				defer portState(t, args, downPortList, true)
			}
			t.Log("Verify the port status after bringing down the ports")
			verifyPortStatus(t, args, downPortList, false)
		}
		sendTraffic(t, args, []string{capturePort}, captureState)
		if idx == 0 {
			verifyTrafficMetrics(t, args, loadBalancePercent, wantLoss)
		}
		pcapBytes := args.otg.GetCapture(t, gosnappi.NewCaptureRequest().SetPortName(capturePort))
		pcapFileName, err := os.CreateTemp("", "pcap")
		if err != nil {
			t.Errorf("ERROR: Could not create temporary pcap file: %v\n", err)
		} else {
			if _, err := pcapFileName.Write(pcapBytes); err != nil {
				t.Errorf("ERROR: Could not write bytes to pcap file: %v\n", err)
			}
			pcapFileName.Close()
			pcapFileList = append(pcapFileList, pcapFileName.Name())
			validatePacketFile(t, pcapFileName.Name(), idx, checkEncap, headerDstIP)
		}
		args.otgConfig.Captures().Clear()
		args.otg.PushConfig(t, args.otgConfig)
		time.Sleep(30 * time.Second)
	}
	return pcapFileList
}

// verifyFallbackTrafficLoss validates packet loss when traffic falls back to the
// Tx port (port1), which OTG cannot list in flow RxNames. It sums flow Rx
// packets on port2..port5 with port1 InFrames and verifies loss does not exceed
// maxLossPct.
func verifyFallbackTrafficLoss(t testing.TB, otgDev *otg.OTG, flowName, fallbackPort string, maxLossPct float64) {
	t.Helper()
	flowMetric := gnmi.Get(t, otgDev, gnmi.OTG().Flow(flowName).State())
	txPkts := flowMetric.GetCounters().GetOutPkts()
	if txPkts == 0 {
		t.Fatalf("OTG traffic generation failed: TxPkts = 0 for flow %s", flowName)
	}
	flowRxPkts := flowMetric.GetCounters().GetInPkts()
	port1InFrames := gnmi.Get(t, otgDev, gnmi.OTG().Port(fallbackPort).Counters().InFrames().State())
	totalRx := flowRxPkts + port1InFrames
	var lossPct float64
	if totalRx < txPkts {
		lossPct = float64(txPkts-totalRx) * 100.0 / float64(txPkts)
	}
	t.Logf("Fallback traffic accounting for flow %s: tx=%d flowRx(port2..5)=%d port1InFrames=%d lossPct=%.2f%%", flowName, txPkts, flowRxPkts, port1InFrames, lossPct)
	if lossPct > maxLossPct+0.01 {
		t.Errorf("Flow %s loss %.2f%% exceeds maxLossPct %.2f%% (tx=%d, flowRx=%d, port1InFrames=%d)", flowName, lossPct, maxLossPct, txPkts, flowRxPkts, port1InFrames)
	}
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
	if fileStat.Size() == 0 {
		return
	}
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

func validateTrafficEncap(t *testing.T, packetSource *gopacket.PacketSource, headerDstIP map[string][]string, index int) {
	t.Helper()
	for packet := range packetSource.Packets() {
		if ethLayer := packet.Layer(layers.LayerTypeEthernet); ethLayer != nil {
			if eth, ok := ethLayer.(*layers.Ethernet); ok && eth.SrcMAC.String() == ateSrcPortMac {
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
			if ipPacket.DstIP.String() == otgIsisPort1LoopV4 {
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
	inFrames := make([]uint64, 0, len(atePortNamelist))
	for _, portName := range atePortNamelist {
		inFrames = append(inFrames, gnmi.Get(t, ate.OTG(), gnmi.OTG().Port(portName).Counters().InFrames().State()))
	}
	gotWeights, _ := normalize(inFrames)

	t.Log("got ratio:", gotWeights)
	t.Log("want ratio:", wantWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, tolerance)); diff != "" {
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
	}
}

// validateTunnel1MACDistribution verifies the 1:3:12 (0.0625:0.1875:0.75) WECMP
// distribution across the three Tunnel 1 next-hop MACs on port2.
func validateTunnel1MACDistribution(pcapPath string, wantWeights []float64) error {
	handle, err := pcap.OpenOffline(pcapPath)
	if err != nil {
		return fmt.Errorf("opening pcap %q: %v", pcapPath, err)
	}
	defer handle.Close()

	macCounts := make([]uint64, 3)
	macIndex := map[string]int{
		magicMAC:  0,
		magicMAC2: 1,
		magicMAC3: 2,
	}
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		ethLayer := packet.Layer(layers.LayerTypeEthernet)
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ethLayer == nil || ipLayer == nil {
			continue
		}
		ipPacket, _ := ipLayer.(*layers.IPv4)
		if ipPacket.DstIP.String() != gribiIPv4EntryVRF1111 {
			continue
		}
		eth, _ := ethLayer.(*layers.Ethernet)
		if idx, ok := macIndex[eth.DstMAC.String()]; ok {
			macCounts[idx]++
		}
	}
	gotWeights, total := normalize(macCounts)
	if total == 0 {
		return fmt.Errorf("received 0 encapsulated Tunnel 1 packets on port2")
	}
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, tolerance)); diff != "" {
		return fmt.Errorf("Tunnel 1 MAC distribution (-want +got):\n%s", diff)
	}
	return nil
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

type testCase struct {
	Desc                   string
	DownPortList           []string
	CapturePortList        []string
	EncapHeaderOuterIPList []string
	EncapHeaderInnerIPList []string
	TrafficDestIP          string
	LoadBalancePercent     []float64
	TestID                 string
}

func buildTestCases(atePorts []string, innerDst string) []*testCase {
	return []*testCase{
		{
			Desc:                   "Test-1: primary encap unviable but backup encap viable for single tunnel",
			DownPortList:           []string{"port2"},
			CapturePortList:        []string{atePorts[2], atePorts[3]},
			EncapHeaderOuterIPList: []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF1112},
			EncapHeaderInnerIPList: []string{innerDst, innerDst},
			TrafficDestIP:          innerDst,
			LoadBalancePercent:     []float64{0, 0, 0.25, 0.75, 0},
			TestID:                 "primarySingle",
		},
		{
			Desc:                   "Test-2: primary and backup encap unviable for single tunnel",
			DownPortList:           []string{"port2", "port3"},
			CapturePortList:        []string{atePorts[3], atePorts[0]},
			EncapHeaderOuterIPList: []string{gribiIPv4EntryVRF1112},
			EncapHeaderInnerIPList: []string{innerDst},
			TrafficDestIP:          innerDst,
			LoadBalancePercent:     []float64{0.25, 0, 0, 0.75, 0},
			TestID:                 "primaryBackupSingle",
		},
		{
			Desc:                   "Test-3: primary encap unviable with backup to routing for single tunnel",
			DownPortList:           []string{"port2"},
			CapturePortList:        []string{atePorts[3], atePorts[0]},
			EncapHeaderOuterIPList: []string{gribiIPv4EntryVRF1112},
			EncapHeaderInnerIPList: []string{innerDst},
			TrafficDestIP:          innerDst,
			LoadBalancePercent:     []float64{0.25, 0, 0, 0.75, 0},
			TestID:                 "primaryBackupRoutingSingle",
		},
		{
			Desc:                   "Test-4: primary encap unviable but backup encap viable for all tunnels",
			DownPortList:           []string{"port2", "port4"},
			CapturePortList:        []string{atePorts[2], atePorts[4]},
			EncapHeaderOuterIPList: []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF2222},
			EncapHeaderInnerIPList: []string{innerDst, innerDst},
			TrafficDestIP:          innerDst,
			LoadBalancePercent:     []float64{0, 0, 0.25, 0, 0.75},
			TestID:                 "primaryAll",
		},
		{
			Desc:                   "Test-5: primary and backup encap unviable for all tunnels",
			DownPortList:           []string{"port2", "port3", "port4", "port5"},
			CapturePortList:        []string{atePorts[0]},
			EncapHeaderOuterIPList: nil,
			EncapHeaderInnerIPList: []string{innerDst},
			TrafficDestIP:          innerDst,
			LoadBalancePercent:     []float64{1, 0, 0, 0, 0},
			TestID:                 "primaryBackupAll",
		},
		{
			Desc:                   "Test-6: primary encap unviable with backup to routing for all tunnels",
			DownPortList:           []string{"port2", "port4"},
			CapturePortList:        []string{atePorts[0]},
			EncapHeaderOuterIPList: nil,
			EncapHeaderInnerIPList: []string{innerDst},
			TrafficDestIP:          innerDst,
			LoadBalancePercent:     []float64{1, 0, 0, 0, 0},
			TestID:                 "primaryBackupRoutingAll",
		},
		{
			Desc:                   "Test-7: no match in encap VRF",
			DownPortList:           nil,
			CapturePortList:        []string{atePorts[0]},
			EncapHeaderOuterIPList: nil,
			EncapHeaderInnerIPList: []string{noMatchEncapDest},
			TrafficDestIP:          noMatchEncapDest,
			LoadBalancePercent:     []float64{1, 0, 0, 0, 0},
			TestID:                 "encapNoMatch",
		},
		{
			Desc:                   "Test-8: no match in TE_VRF_222",
			DownPortList:           []string{"port2", "port4"},
			CapturePortList:        []string{atePorts[0]},
			EncapHeaderOuterIPList: nil,
			EncapHeaderInnerIPList: []string{innerDst},
			TrafficDestIP:          innerDst,
			LoadBalancePercent:     []float64{1, 0, 0, 0, 0},
			TestID:                 "teVrf222NoMatch",
		},
		{
			Desc:                   "Test-9: no match in TE_VRF_111",
			DownPortList:           []string{"port2", "port4"},
			CapturePortList:        []string{atePorts[0]},
			EncapHeaderOuterIPList: nil,
			EncapHeaderInnerIPList: []string{innerDst},
			TrafficDestIP:          innerDst,
			LoadBalancePercent:     []float64{1, 0, 0, 0, 0},
			TestID:                 "teVrf111NoMatch",
		},
	}
}

// TestEncapFrr is to test Test FRR behaviors with encapsulation scenarios
func TestEncapFrr(t *testing.T) {
	ctx := t.Context()
	dut := ondatra.DUT(t, "dut")

	gribic := dut.RawAPIs().GRIBI(t)
	ate := ondatra.ATE(t, "ate")
	top := gosnappi.NewConfig()
	dutPorts := sortPorts(dut.Ports())[:5]
	atePorts := sortPorts(ate.Ports())[:5]

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

	testCases := buildTestCases(atePortNamelist, ipv4InnerDst)
	t.Run("Validate encap next-hop AFT telemetry", func(t *testing.T) {
		if err := gribi.FlushAll(client); err != nil {
			t.Fatal(err)
		}
		configureGribiRoute(t.Context(), t, dut, args.client)
		validateEncapNHTelemetry(t, dut)
	})
	for _, tc := range testCases {
		t.Run(tc.Desc, func(t *testing.T) {
			subCtx := t.Context()
			t.Log("Verify whether the ports are in up state")
			portList := []string{"port1", "port2", "port3", "port4", "port5"}
			verifyPortStatus(t, args, portList, true)

			t.Log("Flush existing gRIBI routes before test.")
			if err := gribi.FlushAll(client); err != nil {
				t.Fatal(err)
			}
			baseCapturePortList := []string{atePortNamelist[1], atePortNamelist[3]}
			configureGribiRoute(subCtx, t, dut, args.client)
			createFlow(t, otgConfig, otg, ipv4InnerDst)
			baseHeaderDstIP := map[string][]string{"outerIP": {gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112}, "innerIP": {ipv4InnerDst, ipv4InnerDst}}
			baseLoadBalancePercent := []float64{0, 0.25, 0, 0.75, 0}
			basePcapFiles := captureAndVerifyTraffic(t, args, nil, baseCapturePortList, baseLoadBalancePercent, !wantLoss, checkEncap, baseHeaderDstIP)
			if len(basePcapFiles) > 0 {
				if err := validateTunnel1MACDistribution(basePcapFiles[0], []float64{0.0625, 0.1875, 0.75}); err != nil {
					t.Errorf("validateTunnel1MACDistribution(%q) failed: %v", basePcapFiles[0], err)
				}
			}
			for _, f := range basePcapFiles {
				os.Remove(f)
			}

			if tc.TestID == "primaryBackupRoutingSingle" {
				args.client.Modify().AddEntry(t,
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(1100).WithDecapsulateHeader(fluent.IPinIP).
						WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
					fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithID(1000).AddNextHop(1100, 1),
				)
				if err := awaitTimeout(subCtx, t, args.client, 2*time.Minute); err != nil {
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
				if err := awaitTimeout(subCtx, t, args.client, 2*time.Minute); err != nil {
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
				if err := awaitTimeout(subCtx, t, args.client, 2*time.Minute); err != nil {
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
				if err := awaitTimeout(subCtx, t, args.client, 2*time.Minute); err != nil {
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
					capturePortList = []string{atePortNamelist[3]}
					loadBalancePercent = []float64{0, 0, 0, 1, 0}
					outerIPList = []string{gribiIPv4EntryVRF1112}
					innerIPList = []string{ipv4InnerDst}
				}
			}
			if deviations.EncapTunnelShutBackupNhgZeroTraffic(dut) {
				if tc.TestID == "primarySingle" || tc.TestID == "primaryBackupSingle" || tc.TestID == "primaryBackupRoutingSingle" {
					capturePortList = []string{atePortNamelist[3]}
					loadBalancePercent = []float64{0, 0, 0, 1, 0}
					outerIPList = []string{gribiIPv4EntryVRF1112}
					innerIPList = []string{ipv4InnerDst}
				} else if tc.TestID == "primaryAll" {
					capturePortList = []string{atePortNamelist[0]}
					loadBalancePercent = []float64{1, 0, 0, 0, 0}
					outerIPList = nil
					innerIPList = []string{ipv4InnerDst}
				}
			}

			headerDstIP := map[string][]string{"outerIP": outerIPList, "innerIP": innerIPList}
			testPcapFiles := captureAndVerifyTraffic(t, args, tc.DownPortList, capturePortList, loadBalancePercent, !wantLoss, checkEncap, headerDstIP)
			for _, f := range testPcapFiles {
				os.Remove(f)
			}
		})
	}
}
