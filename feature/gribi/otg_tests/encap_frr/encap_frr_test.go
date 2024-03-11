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

package encap_frr_test

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
	"github.com/openconfig/featureprofiles/internal/deviations"
	"github.com/openconfig/featureprofiles/internal/fptest"
	"github.com/openconfig/featureprofiles/internal/gribi"
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
	numPorts               = 8
	dscpEncapA1            = 10
	dscpEncapA2            = 18
	dscpEncapB1            = 20
	dscpEncapB2            = 28
	dscpEncapNoMatch       = 30
	ipv4OuterSrc111Addr    = "198.51.100.111"
	ipv4OuterSrc222Addr    = "198.51.100.222"
	ipv4OuterSrcAddr       = "198.100.200.123"
	ipv4InnerDst           = "138.0.11.8"
	ipv4OuterDst333        = "192.58.200.7"
	noMatchEncapDest       = "20.0.0.1"
	prot4                  = 4
	prot41                 = 41
	polName                = "pol1"
	gribiIPv4entry         = "192.51.100.0"
	maskLen24              = "24"
	maskLen32              = "32"
	niDecapTeVrf           = "DECAP_TE_VRF"
	niEncapTeVrfA          = "ENCAP_TE_VRF_A"
	niEncapTeVrfB          = "ENCAP_TE_VRF_B"
	niTeVrf111             = "TE_VRF_111"
	niTeVrf222             = "TE_VRF_222"
	niDefault              = "DEFAULT"
	tolerancePct           = 2
	tolerance              = 0.2
	encapFlow              = "encapFlow"
	correspondingHopLimit  = 64
	magicMac               = "02:00:00:00:00:01"
	gribiIPv4EntryDefVRF1  = "192.0.2.101"
	gribiIPv4EntryDefVRF2  = "192.0.2.102"
	gribiIPv4EntryDefVRF3  = "192.0.2.103"
	gribiIPv4EntryDefVRF4  = "192.0.2.104"
	gribiIPv4EntryDefVRF5  = "192.0.2.105"
	gribiIPv4EntryVRF1111  = "203.0.113.1"
	gribiIPv4EntryVRF1112  = "203.0.113.2"
	gribiIPv4EntryVRF2221  = "203.0.113.100"
	gribiIPv4EntryVRF2222  = "203.0.113.101"
	gribiIPv4EntryEncapVRF = "138.0.11.0"

	dutAreaAddress = "49.0001"
	dutSysID       = "1920.0000.2001"
	otgSysID1      = "640000000001"
	isisInstance   = "DEFAULT"

	otgIsisPort8LoopV4 = "203.0.113.10"
	otgIsisPort8LoopV6 = "2001:db8::203:0:113:10"

	dutAS        = 65501
	peerGrpName1 = "BGP-PEER-GROUP1"

	ateSrcPort       = "ate:port1"
	ateSrcPortMac    = "02:00:01:01:01:01"
	ateSrcNetName    = "srcnet"
	ateSrcNet        = "198.51.100.0"
	ateSrcNetCIDR    = "198.51.100.0/24"
	ateSrcNetFirstIP = "198.51.100.1"
	ateSrcNetCount   = 250

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
	ctx        context.Context
	client     *fluent.GRIBIClient
	dut        *ondatra.DUTDevice
	ate        *ondatra.ATEDevice
	otgConfig  gosnappi.Config
	top        gosnappi.Config
	electionID gribi.Uint128
	otg        *otg.OTG
}

type policyFwRule struct {
	SeqId           uint32
	family          string
	protocol        oc.UnionUint8
	dscpSet         []uint8
	sourceAddr      string
	decapNi         string
	postDecapNi     string
	decapFallbackNi string
	networkInstance string
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

func configureVrfSelectionPolicy(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	d := &oc.Root{}
	dutPolFwdPath := gnmi.OC().NetworkInstance(deviations.DefaultNetworkInstance(dut)).PolicyForwarding()

	pfRule1 := &policyFwRule{SeqId: 1, family: "ipv4", protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222Addr + "/32",
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf222}
	pfRule2 := &policyFwRule{SeqId: 2, family: "ipv4", protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc222Addr + "/32",
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf222}
	pfRule3 := &policyFwRule{SeqId: 3, family: "ipv4", protocol: 4, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111Addr + "/32",
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf111}
	pfRule4 := &policyFwRule{SeqId: 4, family: "ipv4", protocol: 41, dscpSet: []uint8{dscpEncapA1, dscpEncapA2}, sourceAddr: ipv4OuterSrc111Addr + "/32",
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfA, decapFallbackNi: niTeVrf111}

	pfRule5 := &policyFwRule{SeqId: 5, family: "ipv4", protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222Addr + "/32",
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf222}
	pfRule6 := &policyFwRule{SeqId: 6, family: "ipv4", protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc222Addr + "/32",
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf222}
	pfRule7 := &policyFwRule{SeqId: 7, family: "ipv4", protocol: 4, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111Addr + "/32",
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf111}
	pfRule8 := &policyFwRule{SeqId: 8, family: "ipv4", protocol: 41, dscpSet: []uint8{dscpEncapB1, dscpEncapB2}, sourceAddr: ipv4OuterSrc111Addr + "/32",
		decapNi: niDecapTeVrf, postDecapNi: niEncapTeVrfB, decapFallbackNi: niTeVrf111}

	pfRule9 := &policyFwRule{SeqId: 9, family: "ipv4", protocol: 4, sourceAddr: ipv4OuterSrc222Addr + "/32",
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf222}
	pfRule10 := &policyFwRule{SeqId: 10, family: "ipv4", protocol: 41, sourceAddr: ipv4OuterSrc222Addr + "/32",
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf222}
	pfRule11 := &policyFwRule{SeqId: 11, family: "ipv4", protocol: 4, sourceAddr: ipv4OuterSrc111Addr + "/32",
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf111}
	pfRule12 := &policyFwRule{SeqId: 12, family: "ipv4", protocol: 41, sourceAddr: ipv4OuterSrc111Addr + "/32",
		decapNi: niDecapTeVrf, postDecapNi: niDefault, decapFallbackNi: niTeVrf111}

	pfRule13 := &policyFwRule{SeqId: 13, family: "ipv4", dscpSet: []uint8{dscpEncapA1, dscpEncapA2},
		networkInstance: niEncapTeVrfA}
	pfRule14 := &policyFwRule{SeqId: 14, family: "ipv6", dscpSet: []uint8{dscpEncapA1, dscpEncapA2},
		networkInstance: niEncapTeVrfA}
	pfRule15 := &policyFwRule{SeqId: 15, family: "ipv4", dscpSet: []uint8{dscpEncapA1, dscpEncapA2},
		networkInstance: niEncapTeVrfB}
	pfRule16 := &policyFwRule{SeqId: 16, family: "ipv6", dscpSet: []uint8{dscpEncapA1, dscpEncapA2},
		networkInstance: niEncapTeVrfB}
	pfRule17 := &policyFwRule{SeqId: 17, networkInstance: niDefault}

	pfRuleList := []*policyFwRule{pfRule1, pfRule2, pfRule3, pfRule4, pfRule5, pfRule6,
		pfRule7, pfRule8, pfRule9, pfRule10, pfRule11, pfRule12, pfRule13, pfRule14,
		pfRule15, pfRule16, pfRule17}

	ni := d.GetOrCreateNetworkInstance(deviations.DefaultNetworkInstance(dut))
	niP := ni.GetOrCreatePolicyForwarding()
	niPf := niP.GetOrCreatePolicy(polName)
	niPf.SetType(oc.Policy_Type_VRF_SELECTION_POLICY)

	for _, pfRule := range pfRuleList {
		pfR := niPf.GetOrCreateRule(pfRule.SeqId)

		if pfRule.family == "ipv4" {
			pfRProtoIP := pfR.GetOrCreateIpv4()
			if pfRule.protocol != 0 {
				pfRProtoIP.Protocol = oc.UnionUint8(pfRule.protocol)
			}
			if pfRule.sourceAddr != "" {
				pfRProtoIP.SourceAddress = ygot.String(pfRule.sourceAddr)
			}
			if pfRule.dscpSet != nil {
				pfRProtoIP.DscpSet = pfRule.dscpSet
			}
		} else if pfRule.family == "ipv6" {
			pfRProtoIP := pfR.GetOrCreateIpv6()
			if pfRule.dscpSet != nil {
				pfRProtoIP.DscpSet = pfRule.dscpSet
			}
		}

		pfRAction := pfR.GetOrCreateAction()
		if pfRule.decapNi != "" {
			pfRAction.DecapNetworkInstance = ygot.String(pfRule.decapNi)
		}
		if pfRule.postDecapNi != "" {
			pfRAction.PostDecapNetworkInstance = ygot.String(pfRule.postDecapNi)
		}
		if pfRule.decapFallbackNi != "" {
			pfRAction.DecapFallbackNetworkInstance = ygot.String(pfRule.decapFallbackNi)
		}
		if pfRule.networkInstance != "" {
			pfRAction.NetworkInstance = ygot.String(pfRule.networkInstance)
		}
	}

	p1 := dut.Port(t, "port1")
	intf := niP.GetOrCreateInterface(p1.Name())
	intf.ApplyVrfSelectionPolicy = ygot.String(polName)
	intf.GetOrCreateInterfaceRef().Interface = ygot.String(p1.Name())
	intf.GetOrCreateInterfaceRef().Subinterface = ygot.Uint32(0)
	if deviations.InterfaceRefConfigUnsupported(dut) {
		intf.InterfaceRef = nil
	}
	gnmi.Replace(t, dut, dutPolFwdPath.Config(), niP)
}

// configureNetworkInstance configures vrfs DECAP_TE_VRF,ENCAP_TE_VRF_A,ENCAP_TE_VRF_B,
// TE_VRF_222, TE_VRF_111.
func configNonDefaultNetworkInstance(t *testing.T, dut *ondatra.DUTDevice) {
	t.Helper()
	c := &oc.Root{}
	vrfs := []string{"DECAP_TE_VRF", niEncapTeVrfA, niEncapTeVrfB, niTeVrf222, niTeVrf111}
	for _, vrf := range vrfs {
		ni := c.GetOrCreateNetworkInstance(vrf)
		ni.Type = oc.NetworkInstanceTypes_NETWORK_INSTANCE_TYPE_L3VRF
		gnmi.Replace(t, dut, gnmi.OC().NetworkInstance(vrf).Config(), ni)
	}
}

func configureGribiRoute(ctx context.Context, t *testing.T, dut *ondatra.DUTDevice, args *testArgs) {
	t.Helper()

	// Programming AFT entries for prefixes in DEFAULT VRF
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(11).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port2").Name()),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(12).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port3").Name()),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(11).AddNextHop(11, 1).AddNextHop(12, 3),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF1+"/"+maskLen32).WithNextHopGroup(11),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(13).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port4").Name()),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(12).AddNextHop(13, 2),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF2+"/"+maskLen32).WithNextHopGroup(12),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(14).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port5").Name()),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(13).AddNextHop(14, 1),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF3+"/"+maskLen32).WithNextHopGroup(13),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(15).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port6").Name()),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(14).AddNextHop(15, 1),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF4+"/"+maskLen32).WithNextHopGroup(14),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(16).WithMacAddress(magicMac).WithInterfaceRef(dut.Port(t, "port7").Name()),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(15).AddNextHop(16, 1),
		fluent.IPv4Entry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithPrefix(gribiIPv4EntryDefVRF5+"/"+maskLen32).WithNextHopGroup(15),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	defaultVRFIPList := []string{gribiIPv4EntryDefVRF1, gribiIPv4EntryDefVRF2, gribiIPv4EntryDefVRF3, gribiIPv4EntryDefVRF4, gribiIPv4EntryDefVRF5}
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

	// Programming AFT entries for backup NHG
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(2000).WithDecapsulateHeader(fluent.IPinIP).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(2000).AddNextHop(2000, 1),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	// Programming AFT entries for prefixes in TE_VRF_222
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(3).WithIPAddress(gribiIPv4EntryDefVRF3),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(2).AddNextHop(3, 1).WithBackupNHG(2000),
		fluent.IPv4Entry().WithNetworkInstance(niTeVrf222).
			WithPrefix(gribiIPv4EntryVRF2221+"/"+maskLen32).WithNextHopGroup(2),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(5).WithIPAddress(gribiIPv4EntryDefVRF5),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(4).AddNextHop(5, 1).WithBackupNHG(2000),
		fluent.IPv4Entry().WithNetworkInstance(niTeVrf222).
			WithPrefix(gribiIPv4EntryVRF2222+"/"+maskLen32).WithNextHopGroup(4),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	teVRF222IPList := []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF2222}
	for ip := range teVRF222IPList {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(teVRF222IPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// Programming AFT entries for backup NHG
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1000).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc222Addr, gribiIPv4EntryVRF2221).
			WithNextHopNetworkInstance(niTeVrf222),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1000).AddNextHop(1000, 1),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1001).WithDecapsulateHeader(fluent.IPinIP).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc222Addr, gribiIPv4EntryVRF2222).
			WithNextHopNetworkInstance(niTeVrf222),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1001).AddNextHop(1001, 1),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	// Programming AFT entries for prefixes in TE_VRF_111
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(1).WithIPAddress(gribiIPv4EntryDefVRF1),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(2).WithIPAddress(gribiIPv4EntryDefVRF2),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(1).AddNextHop(1, 1).AddNextHop(2, 3).WithBackupNHG(1000),
		fluent.IPv4Entry().WithNetworkInstance(niTeVrf111).
			WithPrefix(gribiIPv4EntryVRF1111+"/"+maskLen32).WithNextHopGroup(1),

		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(4).WithIPAddress(gribiIPv4EntryDefVRF4),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(3).AddNextHop(4, 1).WithBackupNHG(1001),
		fluent.IPv4Entry().WithNetworkInstance(niTeVrf111).
			WithPrefix(gribiIPv4EntryVRF1112+"/"+maskLen32).WithNextHopGroup(3),
	)
	if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
		t.Logf("Could not program entries via client, got err, check error codes: %v", err)
	}

	teVRF111IPList := []string{gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112}
	for ip := range teVRF111IPList {
		chk.HasResult(t, args.client.Results(t),
			fluent.OperationResult().
				WithIPv4Operation(teVRF111IPList[ip]+"/32").
				WithOperationType(constants.Add).
				WithProgrammingResult(fluent.InstalledInFIB).
				AsResult(),
			chk.IgnoreOperationID(),
		)
	}

	// Programming AFT entries for prefixes in ENCAP_TE_VRF_A
	args.client.Modify().AddEntry(t,
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(101).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc111Addr, gribiIPv4EntryVRF1111).
			WithNextHopNetworkInstance(niTeVrf111),
		fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithIndex(102).WithEncapsulateHeader(fluent.IPinIP).
			WithIPinIP(ipv4OuterSrc111Addr, gribiIPv4EntryVRF1112).
			WithNextHopNetworkInstance(niTeVrf111),
		fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
			WithID(101).AddNextHop(101, 1).AddNextHop(102, 3).WithBackupNHG(18),
		fluent.IPv4Entry().WithNetworkInstance(niEncapTeVrfA).
			WithPrefix(gribiIPv4EntryEncapVRF+"/"+maskLen24).WithNextHopGroup(101),
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

	isisIntf := isis.GetOrCreateInterface(intfName)
	isisIntf.Enabled = ygot.Bool(true)
	isisIntf.CircuitType = oc.Isis_CircuitType_POINT_TO_POINT
	isisIntfLevel := isisIntf.GetOrCreateLevel(2)
	isisIntfLevel.Enabled = ygot.Bool(true)
	isisIntfLevelAfi := isisIntfLevel.GetOrCreateAf(oc.IsisTypes_AFI_TYPE_IPV4, oc.IsisTypes_SAFI_TYPE_UNICAST)
	isisIntfLevelAfi.Metric = ygot.Uint32(200)
	isisIntfLevelAfi.Enabled = ygot.Bool(true)

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
	bgpNbrT.LocalAddress = ygot.String(dutlo0Attrs.IPv4)
	af4 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV4_UNICAST)
	af4.Enabled = ygot.Bool(true)
	af6 := bgpNbr.GetOrCreateAfiSafi(oc.BgpTypes_AFI_SAFI_TYPE_IPV6_UNICAST)
	af6.Enabled = ygot.Bool(true)

	return niProto
}

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
	config := gosnappi.NewConfig()
	// ate := ondatra.ATE(t, "ate")
	for i, ap := range atePorts {
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
		}

	}
	config.Captures().Add().SetName("packetCapture").
		SetPortNames([]string{atePortNamelist[1], atePortNamelist[2], atePortNamelist[3], atePortNamelist[4],
			atePortNamelist[5], atePortNamelist[6], atePortNamelist[7]}).
		SetFormat(gosnappi.CaptureFormat.PCAP)
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
	cs := gosnappi.NewControlState()
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.START)
	args.otg.SetControlState(t, cs)
	return cs
}

func sendTraffic(t *testing.T, args *testArgs, capturePortList []string, cs gosnappi.ControlState) {
	t.Helper()
	t.Logf("Starting traffic")
	args.otg.StartTraffic(t)
	time.Sleep(15 * time.Second)
	t.Logf("Stop traffic")
	args.otg.StopTraffic(t)
	cs.Port().Capture().SetState(gosnappi.StatePortCaptureState.STOP)
	args.otg.SetControlState(t, cs)
}

func verifyTraffic(t *testing.T, args *testArgs, capturePortList []string, loadBalancePercent []float64, wantLoss, checkEncap bool, headerDstIP map[string][]string) {
	t.Helper()
	t.Logf("Verifying flow metrics for the flow: encapFlow\n")
	recvMetric := gnmi.Get(t, args.otg, gnmi.OTG().Flow(encapFlow).State())
	txPackets := recvMetric.GetCounters().GetOutPkts()
	rxPackets := recvMetric.GetCounters().GetInPkts()
	lostPackets := txPackets - rxPackets
	var lossPct uint64
	if txPackets != 0 {
		lossPct = lostPackets * 100 / txPackets
	} else {
		t.Errorf("Traffic stats are not correct %v", recvMetric)
	}
	if wantLoss {
		if lossPct < 100-tolerancePct {
			t.Errorf("Traffic is expected to fail %s\n got %v, want 100%% failure", encapFlow, lossPct)
		} else {
			t.Logf("Traffic Loss Test Passed!")
		}
	} else {
		if lossPct > tolerancePct {
			t.Errorf("Traffic Loss Pct for Flow: %s\n got %v, want 0", encapFlow, lossPct)
		} else {
			t.Logf("Traffic Test Passed!")
		}
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
		pcapFileList = append(pcapFileList, pcapFileName.Name())
	}
	validatePackets(t, pcapFileList, checkEncap, headerDstIP)
	args.otgConfig.Captures().Clear()
	args.otg.PushConfig(t, args.otgConfig)
	time.Sleep(30 * time.Second)
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
				if ipPacket.DstIP.String() != headerDstIP["outerIP"][index] {
					t.Errorf("Packets are not encapsulated")
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
		t.Log("Check for port status")
		gnmi.Await(t, args.dut, gnmi.OC().Interface(p.Name()).OperStatus().State(), 1*time.Minute, wantStatus)
		operStatus := gnmi.Get(t, args.dut, gnmi.OC().Interface(p.Name()).OperStatus().State())
		if operStatus != wantStatus {
			t.Errorf("Get(DUT %v oper status): got %v, want %v", port, operStatus, wantStatus)
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
	for i, x := range xs {
		ys[i] = float64(x) / float64(sum)
	}
	return ys, sum
}

func validateTrafficDistribution(t *testing.T, ate *ondatra.ATEDevice, wantWeights []float64) {
	inFramesAllPorts := gnmi.GetAll(t, ate.OTG(), gnmi.OTG().PortAny().Counters().InFrames().State())
	// Skip source port, Port1.
	gotWeights, _ := normalize(inFramesAllPorts[1:])

	t.Log("got ratio:", gotWeights)
	t.Log("want ratio:", wantWeights)
	if diff := cmp.Diff(wantWeights, gotWeights, cmpopts.EquateApprox(0, tolerance)); diff != "" {
		t.Errorf("Packet distribution ratios -want,+got:\n%s", diff)
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

	t.Log("Configure Non-Default Network Instances")
	configNonDefaultNetworkInstance(t, dut)

	configureDUT(t, dut, dutPorts)

	t.Log("Apply vrf selection policy to DUT port-1")
	configureVrfSelectionPolicy(t, dut)

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
		ctx:        ctx,
		client:     client,
		dut:        dut,
		ate:        ate,
		otgConfig:  otgConfig,
		top:        top,
		electionID: eID,
		otg:        otg,
	}

	cases := []struct {
		desc                   string
		downPortList           []string
		capturePortList        []string
		encapHeaderOuterIPList []string
		encapHeaderInnerIPList []string
		trafficDestIP          string
		loadBalancePercent     []float64
		encapUnviable          string
	}{{
		desc:                   "Test-1: primary encap unviable but backup encap viable for single tunnel",
		downPortList:           []string{"port2", "port3", "port4"},
		capturePortList:        []string{atePortNamelist[4], atePortNamelist[5]},
		encapHeaderOuterIPList: []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF1112},
		encapHeaderInnerIPList: []string{ipv4InnerDst, ipv4InnerDst},
		trafficDestIP:          ipv4InnerDst,
		loadBalancePercent:     []float64{0, 0, 0, 0.25, 0.75, 0, 0},
		encapUnviable:          "primarySingle",
	}, {
		desc:                   "Test-2: primary and backup encap unviable for single tunnel",
		downPortList:           []string{"port2", "port3", "port4", "port5"},
		capturePortList:        []string{atePortNamelist[5], atePortNamelist[7]},
		encapHeaderOuterIPList: []string{gribiIPv4EntryVRF1112},
		encapHeaderInnerIPList: []string{ipv4InnerDst},
		trafficDestIP:          ipv4InnerDst,
		loadBalancePercent:     []float64{0, 0, 0, 0, 0.75, 0, 0.25},
		encapUnviable:          "primaryBackupSingle",
	}, {
		desc:                   "Test-3: primary encap unviable with backup to routing for single tunnel",
		downPortList:           []string{"port2", "port3", "port4"},
		capturePortList:        []string{atePortNamelist[5], atePortNamelist[7]},
		encapHeaderOuterIPList: []string{gribiIPv4EntryVRF1112},
		encapHeaderInnerIPList: []string{ipv4InnerDst},
		trafficDestIP:          ipv4InnerDst,
		loadBalancePercent:     []float64{0, 0, 0, 0, 0.75, 0, 0.25},
		encapUnviable:          "primaryBackupRoutingSingle",
	}, {
		desc:                   "Test-4: primary encap unviable but backup encap viable for all tunnels",
		downPortList:           []string{"port2", "port3", "port4", "port6"},
		capturePortList:        []string{atePortNamelist[4], atePortNamelist[6]},
		encapHeaderOuterIPList: []string{gribiIPv4EntryVRF2221, gribiIPv4EntryVRF2222},
		encapHeaderInnerIPList: []string{ipv4InnerDst, ipv4InnerDst},
		trafficDestIP:          ipv4InnerDst,
		loadBalancePercent:     []float64{0, 0, 0, 0.25, 0, 0.75, 0},
		encapUnviable:          "primaryAll",
	}, {
		desc:                   "Test-5: primary and backup encap unviable for all tunnels",
		downPortList:           []string{"port2", "port3", "port4", "port5", "port6", "port7"},
		capturePortList:        []string{atePortNamelist[7]},
		encapHeaderOuterIPList: []string{},
		encapHeaderInnerIPList: []string{ipv4InnerDst},
		trafficDestIP:          ipv4InnerDst,
		loadBalancePercent:     []float64{0, 0, 0, 0, 0, 0, 1},
		encapUnviable:          "primaryBackupAll",
	}, {
		desc:                   "Test-6: primary encap unviable with backup to routing for all tunnels",
		downPortList:           []string{"port2", "port3", "port4", "port6"},
		capturePortList:        []string{atePortNamelist[7]},
		encapHeaderOuterIPList: []string{},
		encapHeaderInnerIPList: []string{ipv4InnerDst},
		trafficDestIP:          ipv4InnerDst,
		loadBalancePercent:     []float64{0, 0, 0, 0, 0, 0, 1},
		encapUnviable:          "primaryBackupRoutingAll",
	}, {
		desc:                   "Test-7: no match in encap VRF",
		downPortList:           []string{},
		capturePortList:        []string{atePortNamelist[7]},
		encapHeaderOuterIPList: []string{},
		encapHeaderInnerIPList: []string{noMatchEncapDest},
		trafficDestIP:          noMatchEncapDest,
		loadBalancePercent:     []float64{0, 0, 0, 0, 0, 0, 1},
		encapUnviable:          "encapNoMatch",
	}}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Log("Verify whether the ports are in up state")
			portList := []string{"port2", "port3", "port4", "port5", "port6", "port7", "port8"}
			verifyPortStatus(t, args, portList, true)

			t.Log("Flush existing gRIBI routes before test.")
			if err := gribi.FlushAll(client); err != nil {
				t.Fatal(err)
			}
			baseCapturePortList := []string{atePortNamelist[1], atePortNamelist[5]}
			configureGribiRoute(ctx, t, dut, args)
			createFlow(t, otgConfig, otg, ipv4InnerDst)
			captureState := startCapture(t, args, baseCapturePortList)
			sendTraffic(t, args, baseCapturePortList, captureState)
			baseHeaderDstIP := map[string][]string{"outerIP": {gribiIPv4EntryVRF1111, gribiIPv4EntryVRF1112}, "innerIP": {ipv4InnerDst, ipv4InnerDst}}
			baseLoadBalancePercent := []float64{0.0156, 0.0468, 0.1875, 0, 0.75, 0, 0}
			verifyTraffic(t, args, baseCapturePortList, baseLoadBalancePercent, !wantLoss, checkEncap, baseHeaderDstIP)

			if tc.encapUnviable == "primaryBackupRoutingSingle" {
				args.client.Modify().AddEntry(t,
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(1000).WithDecapsulateHeader(fluent.IPinIP).
						WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
				)
				if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
					t.Logf("Could not program entries via client, got err, check error codes: %v", err)
				}
			}
			if tc.encapUnviable == "primaryBackupRoutingAll" {
				args.client.Modify().AddEntry(t,
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(1000).WithDecapsulateHeader(fluent.IPinIP).
						WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(1001).WithDecapsulateHeader(fluent.IPinIP).
						WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
				)
				if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
					t.Logf("Could not program entries via client, got err, check error codes: %v", err)
				}
			}
			if tc.encapUnviable == "encapNoMatch" {
				args.client.Modify().AddEntry(t,
					fluent.NextHopEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithIndex(1003).WithNextHopNetworkInstance(deviations.DefaultNetworkInstance(dut)),
					fluent.NextHopGroupEntry().WithNetworkInstance(deviations.DefaultNetworkInstance(dut)).
						WithID(1003).AddNextHop(1003, 1),
					fluent.IPv4Entry().WithNetworkInstance(niEncapTeVrfA).
						WithPrefix("0.0.0.0/0").WithNextHopGroup(1003),
				)
				if err := awaitTimeout(args.ctx, t, args.client, time.Minute); err != nil {
					t.Logf("Could not program entries via client, got err, check error codes: %v", err)
				}
				createFlow(t, otgConfig, otg, noMatchEncapDest)
			}

			captureState = startCapture(t, args, tc.capturePortList)
			if len(tc.downPortList) > 0 {
				t.Logf("Bring down ports %s", tc.downPortList)
				portState(t, args, tc.downPortList, false)
				defer portState(t, args, tc.downPortList, true)
				t.Log("Verify the port status after bringing down the ports")
				verifyPortStatus(t, args, tc.downPortList, false)
			}
			sendTraffic(t, args, tc.capturePortList, captureState)
			headerDstIP := map[string][]string{"outerIP": tc.encapHeaderOuterIPList, "innerIP": tc.encapHeaderInnerIPList}
			verifyTraffic(t, args, tc.capturePortList, tc.loadBalancePercent, !wantLoss, checkEncap, headerDstIP)
		})
	}
}
